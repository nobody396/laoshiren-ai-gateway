package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	OpenAIRoutePoliciesSettingKey = "openai_route_policies"

	defaultOpenAIRoutePolicyCacheTTL      = 60 * time.Second
	defaultOpenAIRoutePolicyErrorCacheTTL = 5 * time.Second
)

var ErrOpenAIRouteEnforceDisabled = errors.New("OpenAI route enforce mode is disabled in this release")

// OpenAIRoutePolicyReader is intentionally narrower than SettingRepository so
// the request path depends only on the immutable policy read it needs.
type OpenAIRoutePolicyReader interface {
	GetValue(ctx context.Context, key string) (string, error)
}

// OpenAIRouteShadowCandidate is the scheduler-owned, hard-filtered candidate
// projection consumed by shadow routing. It contains no credentials.
type OpenAIRouteShadowCandidate struct {
	Account *Account

	Endpoint  string
	Transport string
	Priority  int

	HasReliabilitySample bool
	SuccessLowerBound    float64
	TTFTMilliseconds     float64
	LoadRatio            float64
	WaitingCount         int
}

type OpenAIRouteShadowRequest struct {
	GroupID int64
	Model   string
	Seed    uint64
	Now     time.Time

	Candidates []OpenAIRouteShadowCandidate
}

// OpenAIRouteShadowDecision is diagnostic-only in this release. The legacy
// scheduler remains authoritative even when a shadow decision is available.
type OpenAIRouteShadowDecision struct {
	Evaluated bool
	Mode      OpenAIRoutePolicyMode
	Version   int
	Reason    string

	CandidateCount          int
	ExcludedCount           int
	LegacySelectedAccountID int64
	SelectedAccountID       int64
	SelectedRate            float64
	Diverged                bool
	Emergency               bool
}

type OpenAIRouteShadowEvaluator interface {
	EvaluateShadow(ctx context.Context, req OpenAIRouteShadowRequest) (OpenAIRouteShadowDecision, error)
}

// openAIRoutePolicyConfig is the versioned control-plane representation. A
// missing setting or unmatched policy means legacy routing. No production
// policy values are compiled into the binary.
type openAIRoutePolicyConfig struct {
	GroupID int64  `json:"group_id"`
	Model   string `json:"model"`
	Enabled bool   `json:"enabled"`
	Mode    string `json:"mode"`
	Version int    `json:"policy_version"`

	TargetAverageMultiplier *float64 `json:"target_avg_multiplier"`
	HardAverageMultiplier   float64  `json:"hard_avg_multiplier"`
	EmergencyDebtLimitUSD   float64  `json:"emergency_extra_budget_usd"`
	MaxCreditUSD            float64  `json:"max_credit_usd"`
	EstimatedBaseCostUSD    float64  `json:"estimated_base_cost_usd"`

	PriceExponent   float64 `json:"price_exponent"`
	LatencyBeta     float64 `json:"latency_beta"`
	PriorityPenalty float64 `json:"priority_penalty"`
	MinHealthFactor float64 `json:"min_health_factor"`

	MaxAccountShare      float64   `json:"max_account_share"`
	MaxProviderShare     float64   `json:"max_provider_share"`
	NewAccountShare      float64   `json:"new_account_canary_share"`
	DegradedShare        float64   `json:"degraded_share"`
	RecoveryShares       []float64 `json:"recovery_steps"`
	GenericFailThreshold int       `json:"generic_fail_threshold"`
	FailureWindowSeconds int       `json:"failure_window_seconds"`
	ProbeBackoffSeconds  []int     `json:"probe_backoff_seconds"`
	HardShareCaps        bool      `json:"hard_share_caps"`
}

type openAIRoutePolicyDocument struct {
	Policies []openAIRoutePolicyConfig `json:"policies"`
}

type cachedOpenAIRoutePolicies struct {
	policies  []openAIRoutePolicyConfig
	expiresAt time.Time
	err       error
}

type OpenAIRouteController struct {
	reader      OpenAIRoutePolicyReader
	healthStore OpenAIRouteHealthStore
	budgetStore OpenAIRouteBudgetStore

	cacheMu sync.Mutex
	cache   cachedOpenAIRoutePolicies
}

func NewOpenAIRouteController(
	reader OpenAIRoutePolicyReader,
	healthStore OpenAIRouteHealthStore,
	budgetStore OpenAIRouteBudgetStore,
) *OpenAIRouteController {
	return &OpenAIRouteController{
		reader:      reader,
		healthStore: healthStore,
		budgetStore: budgetStore,
	}
}

func (c *OpenAIRouteController) InvalidatePolicyCache() {
	if c == nil {
		return
	}
	c.cacheMu.Lock()
	c.cache = cachedOpenAIRoutePolicies{}
	c.cacheMu.Unlock()
}

func (c *OpenAIRouteController) EvaluateShadow(
	ctx context.Context,
	req OpenAIRouteShadowRequest,
) (OpenAIRouteShadowDecision, error) {
	decision := OpenAIRouteShadowDecision{Mode: OpenAIRoutePolicyLegacy}
	if c == nil || c.reader == nil || c.healthStore == nil || c.budgetStore == nil {
		decision.Reason = "runtime_unavailable"
		return decision, nil
	}
	if req.GroupID <= 0 || strings.TrimSpace(req.Model) == "" || len(req.Candidates) == 0 {
		decision.Reason = "request_ineligible"
		return decision, nil
	}

	policies, err := c.loadPolicies(ctx, req.Now)
	if err != nil {
		return decision, err
	}
	config, ok := resolveOpenAIRoutePolicyConfig(policies, req.GroupID, req.Model)
	if !ok || !config.Enabled {
		decision.Reason = "policy_not_enabled"
		return decision, nil
	}
	configuredMode := OpenAIRoutePolicyMode(strings.TrimSpace(config.Mode))
	if configuredMode == "" {
		configuredMode = OpenAIRoutePolicyShadow
	}
	decision.Mode = configuredMode
	decision.Version = config.Version
	if configuredMode == OpenAIRoutePolicyLegacy {
		decision.Reason = "legacy_mode"
		return decision, nil
	}
	if configuredMode == OpenAIRoutePolicyEnforce {
		return decision, ErrOpenAIRouteEnforceDisabled
	}
	policy, err := config.normalizedPolicy()
	if err != nil {
		return decision, err
	}
	decision.Mode = policy.Mode
	decision.Version = config.Version
	if config.EstimatedBaseCostUSD <= 0 {
		return decision, fmt.Errorf("%w: estimated_base_cost_usd must be positive", ErrOpenAIRouteInvalidPolicy)
	}

	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	windows := buildOpenAIRouteBudgetWindows(req.GroupID, req.Model, config.Version, now, policy)
	candidates := make([]OpenAIRouteCandidate, 0, len(req.Candidates))
	for _, source := range req.Candidates {
		if source.Account == nil {
			continue
		}
		key, keyErr := NewOpenAIRouteKey(source.Account, req.GroupID, req.Model, source.Endpoint, source.Transport)
		if keyErr != nil {
			continue
		}
		routeHealth, getErr := c.healthStore.Get(ctx, OpenAIRouteHealthStoreKeyForRoute(key))
		if getErr != nil {
			return decision, getErr
		}
		providerHealth, getErr := c.healthStore.Get(ctx, OpenAIRouteHealthStoreKeyForProvider(key))
		if getErr != nil {
			return decision, getErr
		}
		candidates = append(candidates, OpenAIRouteCandidate{
			Key:                  key,
			RateMultiplier:       source.Account.BillingRateMultiplier(),
			Priority:             source.Priority,
			CircuitState:         routeHealth.State,
			ProviderCircuitState: providerHealth.State,
			RecoveryStep:         routeHealth.RecoveryStep,
			HasReliabilitySample: source.HasReliabilitySample,
			SuccessLowerBound:    source.SuccessLowerBound,
			P90TTFTMilliseconds:  source.TTFTMilliseconds,
			LoadRatio:            source.LoadRatio,
			WaitingCount:         source.WaitingCount,
		})
	}
	if len(candidates) == 0 {
		return decision, ErrOpenAIRouteNoCandidate
	}

	reservationID, err := newOpenAIRouteShadowReservationID()
	if err != nil {
		return decision, err
	}
	plan, reservation, err := AllocateAndReserveOpenAIRoute(ctx, c.budgetStore, OpenAIRouteAllocationRequest{
		Policy:               policy,
		Candidates:           candidates,
		EstimatedBaseCostUSD: config.EstimatedBaseCostUSD,
		Seed:                 req.Seed,
		HardShareCaps:        config.HardShareCaps,
	}, windows, reservationID, time.Minute)
	if err != nil {
		return decision, err
	}
	settlement := OpenAIRouteBudgetStoreSettlement{
		ReservationID:        reservation.ReservationID,
		RouteKey:             reservation.RouteKey,
		ActualBaseCostUSD:    config.EstimatedBaseCostUSD,
		ActualAccountCostUSD: config.EstimatedBaseCostUSD * plan.Selected.Candidate.RateMultiplier,
		Windows:              windows,
		AuditTTL:             48 * time.Hour,
	}
	if err := c.budgetStore.Settle(ctx, settlement); err != nil {
		settlement.ActualBaseCostUSD = 0
		settlement.ActualAccountCostUSD = 0
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_ = c.budgetStore.Cancel(cleanupCtx, settlement)
		cleanupCancel()
		return decision, err
	}
	decision.Evaluated = true
	decision.Reason = "shadow_selected"
	decision.CandidateCount = len(candidates)
	decision.ExcludedCount = len(plan.Excluded)
	decision.SelectedAccountID = plan.Selected.Candidate.Key.AccountID
	decision.SelectedRate = plan.Selected.Candidate.RateMultiplier
	decision.Emergency = plan.Emergency
	return decision, nil
}

func newOpenAIRouteShadowReservationID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", fmt.Errorf("create OpenAI route shadow reservation: %w", err)
	}
	return "shadow:" + hex.EncodeToString(id[:]), nil
}

func (c *OpenAIRouteController) loadPolicies(ctx context.Context, now time.Time) ([]openAIRoutePolicyConfig, error) {
	if now.IsZero() {
		now = time.Now()
	}
	c.cacheMu.Lock()
	if now.Before(c.cache.expiresAt) {
		cached := append([]openAIRoutePolicyConfig(nil), c.cache.policies...)
		err := c.cache.err
		c.cacheMu.Unlock()
		return cached, err
	}
	c.cacheMu.Unlock()

	raw, err := c.reader.GetValue(ctx, OpenAIRoutePoliciesSettingKey)
	if errors.Is(err, ErrSettingNotFound) {
		err = nil
		raw = ""
	}
	policies := []openAIRoutePolicyConfig(nil)
	if err == nil && strings.TrimSpace(raw) != "" {
		policies, err = decodeOpenAIRoutePolicies(raw)
	}
	ttl := defaultOpenAIRoutePolicyCacheTTL
	if err != nil {
		ttl = defaultOpenAIRoutePolicyErrorCacheTTL
	}
	c.cacheMu.Lock()
	c.cache = cachedOpenAIRoutePolicies{
		policies:  append([]openAIRoutePolicyConfig(nil), policies...),
		expiresAt: now.Add(ttl),
		err:       err,
	}
	c.cacheMu.Unlock()
	return policies, err
}

func decodeOpenAIRoutePolicies(raw string) ([]openAIRoutePolicyConfig, error) {
	data := []byte(strings.TrimSpace(raw))
	if len(data) == 0 {
		return nil, nil
	}
	var policies []openAIRoutePolicyConfig
	if data[0] == '[' {
		if err := json.Unmarshal(data, &policies); err != nil {
			return nil, fmt.Errorf("decode OpenAI route policies: %w", err)
		}
		return policies, nil
	}
	var document openAIRoutePolicyDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode OpenAI route policy document: %w", err)
	}
	if document.Policies == nil {
		var single openAIRoutePolicyConfig
		if err := json.Unmarshal(data, &single); err != nil {
			return nil, fmt.Errorf("decode OpenAI route policy: %w", err)
		}
		if single.GroupID > 0 || strings.TrimSpace(single.Model) != "" {
			return []openAIRoutePolicyConfig{single}, nil
		}
	}
	return document.Policies, nil
}

func resolveOpenAIRoutePolicyConfig(
	policies []openAIRoutePolicyConfig,
	groupID int64,
	model string,
) (openAIRoutePolicyConfig, bool) {
	model = strings.TrimSpace(model)
	bestScore := -1
	var best openAIRoutePolicyConfig
	for _, policy := range policies {
		if policy.GroupID != groupID {
			continue
		}
		pattern := strings.TrimSpace(policy.Model)
		score := -1
		switch {
		case pattern == model:
			score = 1_000_000 + len(pattern)
		case pattern == "*":
			score = 0
		case strings.HasSuffix(pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*")):
			score = len(strings.TrimSuffix(pattern, "*"))
		}
		if score > bestScore {
			bestScore = score
			best = policy
		}
	}
	return best, bestScore >= 0
}

func (c openAIRoutePolicyConfig) normalizedPolicy() (OpenAIRoutePolicy, error) {
	policy := DefaultOpenAIRoutePolicy()
	mode := strings.TrimSpace(c.Mode)
	if mode == "" {
		mode = string(OpenAIRoutePolicyShadow)
	}
	policy.Mode = OpenAIRoutePolicyMode(mode)
	if policy.Mode == OpenAIRoutePolicyEnforce {
		return policy, ErrOpenAIRouteEnforceDisabled
	}
	if c.Version <= 0 {
		return policy, fmt.Errorf("%w: policy_version must be positive", ErrOpenAIRouteInvalidPolicy)
	}
	if c.TargetAverageMultiplier == nil || *c.TargetAverageMultiplier < 0 {
		return policy, fmt.Errorf("%w: target_avg_multiplier must be present and non-negative", ErrOpenAIRouteInvalidPolicy)
	}
	policy.TargetAverageMultiplier = *c.TargetAverageMultiplier
	policy.HardAverageMultiplier = c.HardAverageMultiplier
	policy.EmergencyDebtLimitUSD = c.EmergencyDebtLimitUSD
	if c.MaxCreditUSD > 0 {
		policy.MaxCreditUSD = c.MaxCreditUSD
	}
	if c.PriceExponent > 0 {
		policy.PriceExponent = c.PriceExponent
	}
	if c.LatencyBeta > 0 {
		policy.LatencyBeta = c.LatencyBeta
	}
	if c.PriorityPenalty > 0 {
		policy.PriorityPenalty = c.PriorityPenalty
	}
	if c.MinHealthFactor > 0 {
		policy.MinHealthFactor = c.MinHealthFactor
	}
	if c.MaxAccountShare > 0 {
		policy.MaxAccountShare = c.MaxAccountShare
	}
	if c.MaxProviderShare > 0 {
		policy.MaxProviderShare = c.MaxProviderShare
	}
	if c.NewAccountShare > 0 {
		policy.NewAccountShare = c.NewAccountShare
	}
	if c.DegradedShare > 0 {
		policy.DegradedShare = c.DegradedShare
	}
	if len(c.RecoveryShares) > 0 {
		policy.RecoveryShares = append([]float64(nil), c.RecoveryShares...)
	}
	if c.GenericFailThreshold > 0 {
		policy.GenericFailThreshold = c.GenericFailThreshold
	}
	if c.FailureWindowSeconds > 0 {
		policy.FailureWindow = time.Duration(c.FailureWindowSeconds) * time.Second
	}
	if len(c.ProbeBackoffSeconds) > 0 {
		policy.ProbeBackoff = make([]time.Duration, 0, len(c.ProbeBackoffSeconds))
		for _, seconds := range c.ProbeBackoffSeconds {
			policy.ProbeBackoff = append(policy.ProbeBackoff, time.Duration(seconds)*time.Second)
		}
	}
	return NormalizeOpenAIRoutePolicy(policy)
}

func buildOpenAIRouteBudgetWindows(
	groupID int64,
	model string,
	version int,
	now time.Time,
	policy OpenAIRoutePolicy,
) []OpenAIRouteBudgetWindowConfig {
	type windowDefinition struct {
		name     string
		duration time.Duration
	}
	definitions := []windowDefinition{
		{name: "5m", duration: 5 * time.Minute},
		{name: "1h", duration: time.Hour},
		{name: "24h", duration: 24 * time.Hour},
	}
	now = now.UTC()
	windows := make([]OpenAIRouteBudgetWindowConfig, 0, len(definitions))
	for _, definition := range definitions {
		epoch := fmt.Sprintf("v%d:%s", version, now.Truncate(definition.duration).Format(time.RFC3339))
		windows = append(windows, OpenAIRouteBudgetWindowConfig{
			Scope: OpenAIRouteBudgetScope{
				GroupID: groupID,
				Model:   strings.TrimSpace(model),
				Window:  definition.name,
				Epoch:   epoch,
			},
			TargetAverageMultiplier: policy.TargetAverageMultiplier,
			HardAverageMultiplier:   policy.HardAverageMultiplier,
			EmergencyDebtLimitUSD:   policy.EmergencyDebtLimitUSD,
			MaxCreditUSD:            policy.MaxCreditUSD,
			TTL:                     2 * definition.duration,
		})
	}
	return windows
}

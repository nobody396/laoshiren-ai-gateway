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
	GroupID      int64
	Model        string
	RequestClass OpenAIRouteRequestClass
	Seed         uint64
	Now          time.Time

	Candidates []OpenAIRouteShadowCandidate
}

// OpenAIRouteShadowDecision is diagnostic-only in this release. The legacy
// scheduler remains authoritative even when a shadow decision is available.
type OpenAIRouteShadowDecision struct {
	DecisionID               string
	Evaluated                bool
	Mode                     OpenAIRoutePolicyMode
	Version                  int
	Reason                   string
	EvaluationDurationMicros int64

	CandidateCount          int
	ExcludedCount           int
	LegacySelectedAccountID int64
	SelectedAccountID       int64
	SelectedRate            float64
	Diverged                bool
	Emergency               bool
	Audit                   *OpenAIRouteShadowAuditSnapshot
}

type OpenAIRouteShadowEvaluator interface {
	EvaluateShadow(ctx context.Context, req OpenAIRouteShadowRequest) (OpenAIRouteShadowDecision, error)
}

// openAIRoutePolicyConfig is the versioned control-plane representation. A
// missing setting or unmatched policy means legacy routing. No production
// policy values are compiled into the binary.
type openAIRoutePolicyConfig struct {
	GroupID      int64  `json:"group_id"`
	Model        string `json:"model"`
	RequestClass string `json:"request_class"`
	Enabled      bool   `json:"enabled"`
	Mode         string `json:"mode"`
	Version      int    `json:"policy_version"`

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
	reader           OpenAIRoutePolicyReader
	healthStore      OpenAIRouteHealthStore
	budgetStore      OpenAIRouteBudgetStore
	observationStore OpenAIRouteObservationStore

	cacheMu sync.Mutex
	cache   cachedOpenAIRoutePolicies
}

func NewOpenAIRouteController(
	reader OpenAIRoutePolicyReader,
	healthStore OpenAIRouteHealthStore,
	budgetStore OpenAIRouteBudgetStore,
	observationStore OpenAIRouteObservationStore,
) *OpenAIRouteController {
	return &OpenAIRouteController{
		reader:           reader,
		healthStore:      healthStore,
		budgetStore:      budgetStore,
		observationStore: observationStore,
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

// RecordOpenAIRouteOutcome drives the route-scoped circuit from real upstream
// attempts. It deliberately does not fan one account failure out to the shared
// failure domain; correlated provider ejection requires independent evidence.
func (c *OpenAIRouteController) RecordOpenAIRouteOutcome(ctx context.Context, observation OpenAIRouteObservation) error {
	if c == nil || c.healthStore == nil {
		return ErrOpenAIRouteNoCandidate
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	policy := DefaultOpenAIRoutePolicy()
	policies, err := c.loadPolicies(ctx, observation.ObservedAt)
	if err != nil {
		return err
	}
	if config, ok := resolveOpenAIRoutePolicyConfig(policies, observation.Key.GroupID, observation.Key.Model, observation.Key.RequestClass); ok && config.Enabled {
		policy, err = config.normalizedPolicy()
		if err != nil {
			return err
		}
	}
	key := OpenAIRouteHealthStoreKeyForRoute(observation.Key)
	state, err := c.healthStore.Get(ctx, key)
	if err != nil {
		return err
	}
	if observation.Success && state.State == OpenAIRouteCircuitHealthy {
		return nil
	}
	if !observation.Success && !observation.PenalizeRoute {
		return nil
	}
	_, err = c.healthStore.ApplyEvent(ctx, key, OpenAIRouteHealthEvent{
		At:           observation.ObservedAt,
		Success:      observation.Success,
		FailureClass: observation.FailureClass,
	}, policy)
	return err
}

func (c *OpenAIRouteController) EvaluateShadow(
	ctx context.Context,
	req OpenAIRouteShadowRequest,
) (decision OpenAIRouteShadowDecision, err error) {
	startedAt := time.Now()
	decision = OpenAIRouteShadowDecision{Mode: OpenAIRoutePolicyLegacy}
	defer func() {
		decision.EvaluationDurationMicros = time.Since(startedAt).Microseconds()
	}()
	if c == nil || c.reader == nil || c.healthStore == nil || c.budgetStore == nil {
		decision.Reason = "runtime_unavailable"
		return decision, nil
	}
	if req.GroupID <= 0 || strings.TrimSpace(req.Model) == "" || !req.RequestClass.Valid() || len(req.Candidates) == 0 {
		decision.Reason = "request_ineligible"
		return decision, nil
	}

	policies, err := c.loadPolicies(ctx, req.Now)
	if err != nil {
		return decision, err
	}
	config, ok := resolveOpenAIRoutePolicyConfig(policies, req.GroupID, req.Model, req.RequestClass)
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
	decisionID, err := newOpenAIRouteShadowReservationID()
	if err != nil {
		return decision, err
	}
	decision.DecisionID = decisionID
	decision.Audit = &OpenAIRouteShadowAuditSnapshot{
		EstimatedBaseCostUSD: config.EstimatedBaseCostUSD,
		RequestClass:         req.RequestClass,
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
	decision.Audit.Policy = newOpenAIRouteShadowAuditPolicy(policy, config.HardShareCaps)

	now := req.Now
	if now.IsZero() {
		now = time.Now()
	}
	windows := buildOpenAIRouteBudgetWindows(req.GroupID, req.Model, req.RequestClass, config.Version, now, policy)
	candidates := make([]OpenAIRouteCandidate, 0, len(req.Candidates))
	routeKeys := make([]OpenAIRouteKey, 0, len(req.Candidates))
	auditIndices := make([]int, 0, len(req.Candidates))
	for _, source := range req.Candidates {
		if source.Account == nil {
			continue
		}
		key, keyErr := NewOpenAIRouteKey(source.Account, req.GroupID, req.Model, req.RequestClass, source.Endpoint, source.Transport)
		if keyErr != nil {
			decision.Audit.Candidates = append(decision.Audit.Candidates, OpenAIRouteShadowAuditCandidate{
				AccountID:      source.Account.ID,
				RateMultiplier: source.Account.BillingRateMultiplier(),
				Priority:       source.Priority,
				Transport:      source.Transport,
				ExclusionReasons: []OpenAIRouteExclusionReason{
					OpenAIRouteExcludedInvalid,
				},
			})
			decision.Audit.Exclusions = append(decision.Audit.Exclusions, OpenAIRouteExclusion{
				AccountID: source.Account.ID,
				Reason:    OpenAIRouteExcludedInvalid,
			})
			continue
		}
		candidate := OpenAIRouteCandidate{
			Key:                  key,
			RateMultiplier:       source.Account.BillingRateMultiplier(),
			Priority:             source.Priority,
			HasReliabilitySample: source.HasReliabilitySample,
			SuccessLowerBound:    source.SuccessLowerBound,
			P90TTFTMilliseconds:  source.TTFTMilliseconds,
			LoadRatio:            source.LoadRatio,
			WaitingCount:         source.WaitingCount,
		}
		decision.Audit.Candidates = append(decision.Audit.Candidates, newOpenAIRouteShadowAuditCandidate(candidate))
		auditIndex := len(decision.Audit.Candidates) - 1
		routeHealth, getErr := c.healthStore.Get(ctx, OpenAIRouteHealthStoreKeyForRoute(key))
		if getErr != nil {
			decision.CandidateCount = len(decision.Audit.Candidates)
			return decision, getErr
		}
		providerHealth, getErr := c.healthStore.Get(ctx, OpenAIRouteHealthStoreKeyForProvider(key))
		if getErr != nil {
			decision.CandidateCount = len(decision.Audit.Candidates)
			return decision, getErr
		}
		candidate.CircuitState = routeHealth.State
		candidate.ProviderCircuitState = providerHealth.State
		candidate.RecoveryStep = routeHealth.RecoveryStep
		decision.Audit.Candidates[auditIndex] = newOpenAIRouteShadowAuditCandidate(candidate)
		candidates = append(candidates, candidate)
		routeKeys = append(routeKeys, key)
		auditIndices = append(auditIndices, auditIndex)
	}
	decision.CandidateCount = len(decision.Audit.Candidates)
	if len(candidates) == 0 {
		decision.ExcludedCount = len(decision.Audit.Exclusions)
		return decision, ErrOpenAIRouteNoCandidate
	}
	profiles := make(map[string]OpenAIRouteObservationProfile)
	if c.observationStore != nil {
		var getErr error
		profiles, getErr = c.observationStore.GetBatch(ctx, routeKeys, now)
		if getErr != nil {
			return decision, getErr
		}
	}
	profileByCandidate := make([]OpenAIRouteObservationProfile, len(candidates))
	var totalShareAttempts uint64
	var totalReliabilitySamples uint64
	providerShareAttempts := make(map[string]uint64)
	for idx := range candidates {
		profile := profiles[OpenAIRouteObservationFingerprint(candidates[idx].Key)]
		profileByCandidate[idx] = profile
		shareAttempts := profile.Recent.AttemptCount
		if shareAttempts == 0 {
			shareAttempts = profile.Global.AttemptCount
		}
		totalShareAttempts += shareAttempts
		providerShareAttempts[openAIRouteProviderKey(candidates[idx])] += shareAttempts
		totalReliabilitySamples += profile.Global.ReliabilityCount
	}
	for idx := range candidates {
		profile := profileByCandidate[idx]
		blended := BlendOpenAIRouteObservationProfile(profile)
		observationSource := "process_local"
		if blended.ReliabilityCount > 0 {
			observationSource = "shared"
			candidates[idx].HasReliabilitySample = true
			candidates[idx].SuccessLowerBound = blended.SuccessLowerBound()
			candidates[idx].P90TTFTMilliseconds = blended.TTFTPercentile(0.90)
			candidates[idx].P95CompletionLatencyMilliseconds = blended.LatencyPercentile(0.95)
			candidates[idx].ObservationSampleCount = blended.ReliabilityCount
			if blended.AttemptCount > 0 {
				candidates[idx].PartialStreamRate = float64(blended.PartialStreams) / float64(blended.AttemptCount)
			}
		}
		shareAttempts := profile.Recent.AttemptCount
		if shareAttempts == 0 {
			shareAttempts = profile.Global.AttemptCount
		}
		if totalShareAttempts > 0 {
			candidates[idx].CurrentAccountShare = float64(shareAttempts) / float64(totalShareAttempts)
			candidates[idx].CurrentProviderShare = float64(providerShareAttempts[openAIRouteProviderKey(candidates[idx])]) / float64(totalShareAttempts)
		}
		candidates[idx].ExplorationBoost = openAIRouteExplorationBoost(blended.ReliabilityCount, totalReliabilitySamples)
		auditIndex := auditIndices[idx]
		decision.Audit.Candidates[auditIndex] = newOpenAIRouteShadowAuditCandidate(candidates[idx])
		decision.Audit.Candidates[auditIndex].ObservationSource = observationSource
		decision.Audit.Candidates[auditIndex].ObservationSamples = profile.Global.ReliabilityCount
		decision.Audit.Candidates[auditIndex].RecentSamples = profile.Recent.ReliabilityCount
		decision.Audit.Candidates[auditIndex].HourOfWeekSamples = profile.HourOfWeek.ReliabilityCount
	}

	plan, reservation, ledgers, err := allocateAndReserveOpenAIRouteWithLedgers(ctx, c.budgetStore, OpenAIRouteAllocationRequest{
		Policy:               policy,
		Candidates:           candidates,
		EstimatedBaseCostUSD: config.EstimatedBaseCostUSD,
		Seed:                 req.Seed,
		HardShareCaps:        config.HardShareCaps,
	}, windows, decisionID, time.Minute)
	selectedAccountID := int64(0)
	selectedRate := 0.0
	if err == nil {
		selectedAccountID = plan.Selected.Candidate.Key.AccountID
		selectedRate = plan.Selected.Candidate.RateMultiplier
	}
	decision.Audit.MinHealthyMultiplier = plan.MinHealthyMultiplier
	decision.Audit.Exclusions = append(decision.Audit.Exclusions, plan.Excluded...)
	decision.Audit.BudgetWindows = newOpenAIRouteShadowAuditBudgetWindows(
		windows,
		ledgers,
		selectedAccountID,
		selectedRate,
		config.EstimatedBaseCostUSD,
	)
	applyOpenAIRouteShadowAuditPlan(decision.Audit, plan, selectedAccountID)
	decision.ExcludedCount = len(decision.Audit.Exclusions)
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
	decision.ExcludedCount = len(decision.Audit.Exclusions)
	decision.SelectedAccountID = plan.Selected.Candidate.Key.AccountID
	decision.SelectedRate = plan.Selected.Candidate.RateMultiplier
	decision.Emergency = plan.Emergency
	return decision, nil
}

func newOpenAIRouteShadowAuditPolicy(policy OpenAIRoutePolicy, hardShareCaps bool) OpenAIRouteShadowAuditPolicy {
	backoff := make([]int64, 0, len(policy.ProbeBackoff))
	for _, value := range policy.ProbeBackoff {
		backoff = append(backoff, int64(value/time.Second))
	}
	return OpenAIRouteShadowAuditPolicy{
		TargetAverageMultiplier: policy.TargetAverageMultiplier,
		HardAverageMultiplier:   policy.HardAverageMultiplier,
		EmergencyDebtLimitUSD:   policy.EmergencyDebtLimitUSD,
		MaxCreditUSD:            policy.MaxCreditUSD,
		PriceExponent:           policy.PriceExponent,
		LatencyBeta:             policy.LatencyBeta,
		PriorityPenalty:         policy.PriorityPenalty,
		MinHealthFactor:         policy.MinHealthFactor,
		MaxAccountShare:         policy.MaxAccountShare,
		MaxProviderShare:        policy.MaxProviderShare,
		NewAccountShare:         policy.NewAccountShare,
		DegradedShare:           policy.DegradedShare,
		RecoveryShares:          append([]float64(nil), policy.RecoveryShares...),
		GenericFailThreshold:    policy.GenericFailThreshold,
		FailureWindowSeconds:    int64(policy.FailureWindow / time.Second),
		ProbeBackoffSeconds:     backoff,
		HardShareCaps:           hardShareCaps,
	}
}

func newOpenAIRouteShadowAuditCandidate(candidate OpenAIRouteCandidate) OpenAIRouteShadowAuditCandidate {
	return OpenAIRouteShadowAuditCandidate{
		AccountID:                        candidate.Key.AccountID,
		EndpointHash:                     candidate.Key.EndpointHash,
		FailureDomain:                    candidate.Key.FailureDomain,
		Transport:                        candidate.Key.Transport,
		RateMultiplier:                   candidate.RateMultiplier,
		Priority:                         candidate.Priority,
		CircuitState:                     normalizeOpenAIRouteCircuitState(candidate.CircuitState),
		ProviderCircuitState:             normalizeOpenAIRouteProviderCircuitState(candidate.ProviderCircuitState),
		RecoveryStep:                     candidate.RecoveryStep,
		HasReliabilitySample:             candidate.HasReliabilitySample,
		SuccessLowerBound:                candidate.SuccessLowerBound,
		TTFTMilliseconds:                 candidate.P90TTFTMilliseconds,
		P95CompletionLatencyMilliseconds: candidate.P95CompletionLatencyMilliseconds,
		PartialStreamRate:                candidate.PartialStreamRate,
		ObservationSamples:               candidate.ObservationSampleCount,
		LoadRatio:                        candidate.LoadRatio,
		WaitingCount:                     candidate.WaitingCount,
		CurrentAccountShare:              candidate.CurrentAccountShare,
		CurrentProviderShare:             candidate.CurrentProviderShare,
		ExplorationBoost:                 candidate.ExplorationBoost,
	}
}

func applyOpenAIRouteShadowAuditPlan(
	snapshot *OpenAIRouteShadowAuditSnapshot,
	plan OpenAIRouteAllocationPlan,
	selectedAccountID int64,
) {
	if snapshot == nil {
		return
	}
	byID := make(map[int64]*OpenAIRouteShadowAuditCandidate, len(snapshot.Candidates))
	for idx := range snapshot.Candidates {
		byID[snapshot.Candidates[idx].AccountID] = &snapshot.Candidates[idx]
	}
	for rank, item := range plan.Ranked {
		candidate := byID[item.Candidate.Key.AccountID]
		if candidate == nil {
			continue
		}
		candidate.Rank = rank + 1
		candidate.Selected = selectedAccountID > 0 && item.Candidate.Key.AccountID == selectedAccountID
		candidate.Weight = item.Weight
		candidate.HealthFactor = item.HealthFactor
		candidate.LatencyFactor = item.LatencyFactor
		candidate.TailLatencyFactor = item.TailLatencyFactor
		candidate.StreamIntegrityFactor = item.StreamIntegrityFactor
		candidate.HeadroomFactor = item.HeadroomFactor
		candidate.PriceFactor = item.PriceFactor
		candidate.PriorityFactor = item.PriorityFactor
		candidate.PredictedExtraCostUSD = item.PredictedExtraCost
		candidate.EmergencyBudgetUsed = item.EmergencyBudgetUsed
	}
	for _, exclusion := range plan.Excluded {
		candidate := byID[exclusion.AccountID]
		if candidate != nil {
			candidate.ExclusionReasons = append(candidate.ExclusionReasons, exclusion.Reason)
		}
	}
}

func newOpenAIRouteShadowAuditBudgetWindows(
	windows []OpenAIRouteBudgetWindowConfig,
	ledgers []OpenAIRouteBudgetLedger,
	selectedAccountID int64,
	rateMultiplier float64,
	estimatedBaseCostUSD float64,
) []OpenAIRouteShadowAuditBudgetWindow {
	out := make([]OpenAIRouteShadowAuditBudgetWindow, 0, len(windows))
	for idx, window := range windows {
		item := OpenAIRouteShadowAuditBudgetWindow{
			Window:     window.Scope.Window,
			Epoch:      window.Scope.Epoch,
			TTLSeconds: int64(window.TTL / time.Second),
		}
		if idx < len(ledgers) {
			item.Before = ledgers[idx]
			item.ProjectedAfter = ledgers[idx]
			if selectedAccountID > 0 {
				reservation, reserveErr := item.ProjectedAfter.Reserve(rateMultiplier, estimatedBaseCostUSD)
				if reserveErr == nil {
					accountCost := estimatedBaseCostUSD * rateMultiplier
					item.ProjectionValid = item.ProjectedAfter.Settle(reservation, estimatedBaseCostUSD, accountCost) == nil
				}
			}
		}
		out = append(out, item)
	}
	return out
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
	requestClass OpenAIRouteRequestClass,
) (openAIRoutePolicyConfig, bool) {
	model = strings.TrimSpace(model)
	bestScore := -1
	var best openAIRoutePolicyConfig
	for _, policy := range policies {
		if policy.GroupID != groupID {
			continue
		}
		classPattern := strings.TrimSpace(policy.RequestClass)
		if classPattern == "" {
			// Policies created before request classes existed governed text
			// routing. Do not silently apply them to image requests.
			classPattern = string(OpenAIRouteRequestClassText)
		}
		classScore := -1
		switch classPattern {
		case string(requestClass):
			classScore = 10_000_000
		case "*":
			classScore = 0
		default:
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
		if score < 0 {
			continue
		}
		score += classScore
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
	requestClass OpenAIRouteRequestClass,
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
				GroupID:      groupID,
				Model:        strings.TrimSpace(model),
				RequestClass: requestClass,
				Window:       definition.name,
				Epoch:        epoch,
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

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

	"golang.org/x/sync/errgroup"
)

const (
	OpenAIRoutePoliciesSettingKey = "openai_route_policies"

	defaultOpenAIRoutePolicyCacheTTL              = 60 * time.Second
	defaultOpenAIRoutePolicyErrorCacheTTL         = 5 * time.Second
	defaultOpenAIRouteProviderCorrelationAccounts = 2
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
	// RouteVariant is true only for a diagnostic endpoint synthesized from a
	// policy. It never changes the underlying Account or Legacy endpoint.
	RouteVariant bool

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
	ActivationID             string
	ExperimentID             string
	VariantID                string
	ShadowStartedAt          time.Time
	Evaluated                bool
	Mode                     OpenAIRoutePolicyMode
	Version                  int
	Reason                   string
	EvaluationDurationMicros int64

	CandidateCount           int
	ExcludedCount            int
	LegacySelectedAccountID  int64
	SelectedAccountID        int64
	SelectedEndpointHash     string
	SelectedRouteFingerprint string
	SelectedRate             float64
	Diverged                 bool
	Emergency                bool
	Audit                    *OpenAIRouteShadowAuditSnapshot
}

type OpenAIRouteShadowEvaluator interface {
	EvaluateShadow(ctx context.Context, req OpenAIRouteShadowRequest) (OpenAIRouteShadowDecision, error)
}

// OpenAIRouteShadowBatchEvaluator evaluates independent treatments against the
// same immutable scheduling opportunity. It is Shadow-only: Legacy remains the
// sole source of the account actually returned to the caller.
type OpenAIRouteShadowBatchEvaluator interface {
	EvaluateShadows(ctx context.Context, req OpenAIRouteShadowRequest) ([]OpenAIRouteShadowDecision, error)
}

// openAIRoutePolicyConfig is the versioned control-plane representation. A
// missing setting or unmatched policy means legacy routing. No production
// policy values are compiled into the binary.
type openAIRoutePolicyConfig struct {
	GroupID         int64  `json:"group_id"`
	Model           string `json:"model"`
	RequestClass    string `json:"request_class"`
	Enabled         bool   `json:"enabled"`
	Mode            string `json:"mode"`
	Version         int    `json:"policy_version"`
	ActivationID    string `json:"activation_id"`
	ShadowStartedAt string `json:"shadow_started_at"`
	ExperimentID    string `json:"experiment_id,omitempty"`
	VariantID       string `json:"variant_id,omitempty"`

	TargetAverageMultiplier *float64 `json:"target_avg_multiplier"`
	HardAverageMultiplier   float64  `json:"hard_avg_multiplier"`
	EmergencyDebtLimitUSD   float64  `json:"emergency_extra_budget_usd"`
	MaxCreditUSD            float64  `json:"max_credit_usd"`
	EstimatedBaseCostUSD    float64  `json:"estimated_base_cost_usd"`

	PriceExponent   float64 `json:"price_exponent"`
	LatencyBeta     float64 `json:"latency_beta"`
	PriorityPenalty float64 `json:"priority_penalty"`
	MinHealthFactor float64 `json:"min_health_factor"`

	MaxAccountShare       float64                         `json:"max_account_share"`
	MaxProviderShare      float64                         `json:"max_provider_share"`
	NewAccountShare       float64                         `json:"new_account_canary_share"`
	DegradedShare         float64                         `json:"degraded_share"`
	RecoveryShares        []float64                       `json:"recovery_steps"`
	GenericFailThreshold  int                             `json:"generic_fail_threshold"`
	FailureWindowSeconds  int                             `json:"failure_window_seconds"`
	ProbeBackoffSeconds   []int                           `json:"probe_backoff_seconds"`
	HardShareCaps         bool                            `json:"hard_share_caps"`
	BenchmarkPriorEnabled bool                            `json:"benchmark_prior_enabled,omitempty"`
	RouteVariants         []openAIRouteRouteVariantConfig `json:"route_variants,omitempty"`
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
	benchmarkStore   OpenAIRouteBenchmarkObservationStore
	observationCache *openAIRouteObservationProfileCache
	benchmarkCache   *openAIRouteBenchmarkObservationProfileCache

	cacheMu sync.Mutex
	cache   cachedOpenAIRoutePolicies
}

func NewOpenAIRouteController(
	reader OpenAIRoutePolicyReader,
	healthStore OpenAIRouteHealthStore,
	budgetStore OpenAIRouteBudgetStore,
	observationStore OpenAIRouteObservationStore,
) *OpenAIRouteController {
	controller := &OpenAIRouteController{
		reader:           reader,
		healthStore:      healthStore,
		budgetStore:      budgetStore,
		observationStore: observationStore,
	}
	if benchmarkStore, ok := observationStore.(OpenAIRouteBenchmarkObservationStore); ok {
		controller.benchmarkStore = benchmarkStore
		controller.benchmarkCache = newOpenAIRouteBenchmarkObservationProfileCache(
			benchmarkStore,
			defaultOpenAIRouteBenchmarkProfileCacheTTL,
			defaultOpenAIRouteBenchmarkProfileLoadTimeout,
			defaultOpenAIRouteBenchmarkProfileCacheMaxEntries,
		)
	}
	if observationStore != nil {
		controller.observationCache = newOpenAIRouteObservationProfileCache(
			observationStore,
			defaultOpenAIRouteObservationProfileCacheTTL,
			defaultOpenAIRouteObservationProfileLoadTimeout,
			defaultOpenAIRouteObservationProfileCacheMaxEntries,
		)
	}
	return controller
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
	event := OpenAIRouteHealthEvent{
		At:           observation.ObservedAt,
		Success:      observation.Success,
		FailureClass: observation.FailureClass,
	}
	applyRoute := observation.Success && state.State != OpenAIRouteCircuitHealthy
	applyRoute = applyRoute || (!observation.Success && observation.PenalizeRoute)
	if applyRoute {
		if _, err = c.healthStore.ApplyEvent(ctx, key, event, policy); err != nil {
			return err
		}
	}

	if !OpenAIRouteHasSharedFailureDomain(observation.Key) {
		return nil
	}
	providerEvidence := observation.Success || (observation.PenalizeRoute && OpenAIRouteFailureCanEscalateProvider(observation.FailureClass))
	if !providerEvidence {
		return nil
	}
	_, err = c.healthStore.RecordProviderEvidence(ctx, observation.Key, event, policy, defaultOpenAIRouteProviderCorrelationAccounts)
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
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	policies, err := c.loadPolicies(ctx, now)
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
	decision.ActivationID = strings.TrimSpace(config.ActivationID)
	decision.Audit = &OpenAIRouteShadowAuditSnapshot{
		EstimatedBaseCostUSD: config.EstimatedBaseCostUSD,
		RequestClass:         req.RequestClass,
		ActivationID:         decision.ActivationID,
	}
	activationID, shadowStartedAt, err := config.normalizedShadowActivation(now)
	if err != nil {
		return decision, err
	}
	decision.ActivationID = activationID
	decision.ShadowStartedAt = shadowStartedAt
	decision.Audit.ActivationID = activationID
	decision.Audit.ShadowStartedAt = shadowStartedAt
	experimentID, variantID, err := config.normalizedExperimentIdentity(activationID)
	if err != nil {
		return decision, err
	}
	decision.ExperimentID = experimentID
	decision.VariantID = variantID
	decision.Audit.ExperimentID = experimentID
	decision.Audit.VariantID = variantID
	policy, err := config.normalizedPolicy()
	if err != nil {
		return decision, err
	}
	decision.Mode = policy.Mode
	decision.Version = config.Version
	if config.EstimatedBaseCostUSD <= 0 {
		return decision, fmt.Errorf("%w: estimated_base_cost_usd must be positive", ErrOpenAIRouteInvalidPolicy)
	}
	if len(config.RouteVariants) > 0 && !config.BenchmarkPriorEnabled {
		return decision, fmt.Errorf("%w: route variants require benchmark_prior_enabled", ErrOpenAIRouteInvalidPolicy)
	}
	if config.BenchmarkPriorEnabled {
		if req.RequestClass != OpenAIRouteRequestClassText {
			return decision, fmt.Errorf("%w: benchmark prior supports text routes only", ErrOpenAIRouteInvalidPolicy)
		}
		if c.benchmarkStore == nil || c.benchmarkCache == nil {
			return decision, fmt.Errorf("%w: benchmark observation store unavailable", ErrOpenAIRouteInvalidPolicy)
		}
	}
	shadowSources, auditRouteVariants, err := expandOpenAIRouteShadowVariants(req.Candidates, config.RouteVariants)
	if err != nil {
		return decision, err
	}
	decision.Audit.Policy = newOpenAIRouteShadowAuditPolicy(
		policy,
		config.HardShareCaps,
		config.BenchmarkPriorEnabled,
		auditRouteVariants,
	)

	windows := buildOpenAIRouteBudgetWindows(
		req.GroupID,
		req.Model,
		req.RequestClass,
		config.Version,
		experimentID,
		variantID,
		now,
		policy,
	)
	candidates := make([]OpenAIRouteCandidate, 0, len(shadowSources))
	routeKeys := make([]OpenAIRouteKey, 0, len(shadowSources))
	healthKeys := make([]OpenAIRouteHealthStoreKey, 0, 2*len(shadowSources))
	auditIndices := make([]int, 0, len(shadowSources))
	for _, source := range shadowSources {
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
				RouteVariant:   source.RouteVariant,
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
			RouteVariant:         source.RouteVariant,
			HasReliabilitySample: source.HasReliabilitySample,
			SuccessLowerBound:    source.SuccessLowerBound,
			P90TTFTMilliseconds:  source.TTFTMilliseconds,
			LoadRatio:            source.LoadRatio,
			WaitingCount:         source.WaitingCount,
		}
		decision.Audit.Candidates = append(decision.Audit.Candidates, newOpenAIRouteShadowAuditCandidate(candidate))
		auditIndex := len(decision.Audit.Candidates) - 1
		candidates = append(candidates, candidate)
		routeKeys = append(routeKeys, key)
		healthKeys = append(healthKeys, OpenAIRouteHealthStoreKeyForRoute(key), OpenAIRouteHealthStoreKeyForProvider(key))
		auditIndices = append(auditIndices, auditIndex)
	}
	decision.CandidateCount = len(decision.Audit.Candidates)
	if len(candidates) == 0 {
		decision.ExcludedCount = len(decision.Audit.Exclusions)
		return decision, ErrOpenAIRouteNoCandidate
	}
	profiles := make(map[string]OpenAIRouteObservationProfile)
	benchmarkProfiles := make(map[string]OpenAIRouteBenchmarkObservationProfile)
	var healthStates map[string]OpenAIRouteHealthState
	readGroup, readCtx := errgroup.WithContext(ctx)
	readGroup.Go(func() error {
		var readErr error
		healthStates, readErr = c.healthStore.GetBatch(readCtx, healthKeys)
		return readErr
	})
	if c.observationStore != nil {
		readGroup.Go(func() error {
			var readErr error
			if c.observationCache != nil {
				profiles, readErr = c.observationCache.GetBatch(readCtx, routeKeys, now)
			} else {
				profiles, readErr = c.observationStore.GetBatch(readCtx, routeKeys, now)
			}
			return readErr
		})
	}
	if config.BenchmarkPriorEnabled {
		readGroup.Go(func() error {
			var readErr error
			benchmarkProfiles, readErr = c.benchmarkCache.GetBatch(readCtx, routeKeys, now)
			return readErr
		})
	}
	if getErr := readGroup.Wait(); getErr != nil {
		return decision, getErr
	}
	for idx, key := range routeKeys {
		routeStoreKey := OpenAIRouteHealthStoreKeyForRoute(key)
		providerStoreKey := OpenAIRouteHealthStoreKeyForProvider(key)
		routeHealth, ok := healthStates[routeStoreKey.Fingerprint()]
		if !ok {
			return decision, fmt.Errorf("missing OpenAI route health state %s", routeStoreKey.Fingerprint())
		}
		providerHealth, ok := healthStates[providerStoreKey.Fingerprint()]
		if !ok {
			return decision, fmt.Errorf("missing OpenAI provider health state %s", providerStoreKey.Fingerprint())
		}
		candidates[idx].CircuitState = routeHealth.State
		candidates[idx].ProviderCircuitState = providerHealth.State
		candidates[idx].RecoveryStep = routeHealth.RecoveryStep
		decision.Audit.Candidates[auditIndices[idx]] = newOpenAIRouteShadowAuditCandidate(candidates[idx])
	}
	profileByCandidate := make([]OpenAIRouteObservationProfile, len(candidates))
	benchmarkByCandidate := make([]OpenAIRouteBenchmarkObservationProfile, len(candidates))
	blendedByCandidate := make([]OpenAIRouteObservationAggregate, len(candidates))
	benchmarkAppliedByCandidate := make([]bool, len(candidates))
	var totalShareAttempts uint64
	var totalReliabilitySamples uint64
	accountShareAttempts := make(map[int64]uint64)
	providerShareAttempts := make(map[string]uint64)
	for idx := range candidates {
		profile := profiles[OpenAIRouteObservationFingerprint(candidates[idx].Key)]
		profileByCandidate[idx] = profile
		benchmarkProfile := benchmarkProfiles[OpenAIRouteObservationFingerprint(candidates[idx].Key)]
		benchmarkByCandidate[idx] = benchmarkProfile
		blended := BlendOpenAIRouteObservationProfile(profile)
		if config.BenchmarkPriorEnabled {
			blended, benchmarkAppliedByCandidate[idx] = ApplyOpenAIRouteBenchmarkPrior(blended, benchmarkProfile)
		}
		blendedByCandidate[idx] = blended
		shareAttempts := profile.Recent.AttemptCount
		if shareAttempts == 0 {
			shareAttempts = profile.Global.AttemptCount
		}
		totalShareAttempts += shareAttempts
		accountShareAttempts[candidates[idx].Key.AccountID] += shareAttempts
		providerShareAttempts[openAIRouteProviderKey(candidates[idx])] += shareAttempts
		totalReliabilitySamples += blended.ReliabilityCount
	}
	for idx := range candidates {
		profile := profileByCandidate[idx]
		benchmarkProfile := benchmarkByCandidate[idx]
		blended := blendedByCandidate[idx]
		observationSource := "process_local"
		if blended.ReliabilityCount > 0 {
			observationSource = "shared"
			if benchmarkAppliedByCandidate[idx] {
				if profile.Global.ReliabilityCount > 0 {
					observationSource = "shared+active_benchmark_prior"
				} else {
					observationSource = "active_benchmark_prior"
				}
			}
			candidates[idx].HasReliabilitySample = true
			candidates[idx].SuccessLowerBound = blended.SuccessLowerBound()
			candidates[idx].P90TTFTMilliseconds = blended.TTFTPercentile(0.90)
			candidates[idx].P95CompletionLatencyMilliseconds = blended.LatencyPercentile(0.95)
			candidates[idx].ObservationSampleCount = blended.ReliabilityCount
			candidates[idx].PartialStreamRate = blended.PartialStreamRate()
		}
		if totalShareAttempts > 0 {
			candidates[idx].CurrentAccountShare = float64(accountShareAttempts[candidates[idx].Key.AccountID]) / float64(totalShareAttempts)
			candidates[idx].CurrentProviderShare = float64(providerShareAttempts[openAIRouteProviderKey(candidates[idx])]) / float64(totalShareAttempts)
		}
		candidates[idx].ExplorationBoost = openAIRouteExplorationBoost(blended.ReliabilityCount, totalReliabilitySamples)
		costEstimate := EstimateOpenAIRouteBaseCost(req.RequestClass, config.EstimatedBaseCostUSD, profile.Global)
		candidates[idx].EstimatedBaseCostUSD = costEstimate.EstimatedBaseCostUSD
		candidates[idx].ObservedMeanCostUSD = costEstimate.ObservedMeanCostUSD
		candidates[idx].CostObservationSamples = costEstimate.Samples
		candidates[idx].CostEstimateSource = costEstimate.Source
		auditIndex := auditIndices[idx]
		decision.Audit.Candidates[auditIndex] = newOpenAIRouteShadowAuditCandidate(candidates[idx])
		decision.Audit.Candidates[auditIndex].ObservationSource = observationSource
		decision.Audit.Candidates[auditIndex].ObservationSamples = profile.Global.ReliabilityCount
		decision.Audit.Candidates[auditIndex].RecentSamples = profile.Recent.ReliabilityCount
		decision.Audit.Candidates[auditIndex].HourOfWeekSamples = profile.HourOfWeek.ReliabilityCount
		decision.Audit.Candidates[auditIndex].BenchmarkSamples = benchmarkProfile.Evidence.RawSamples
		decision.Audit.Candidates[auditIndex].BenchmarkEffectiveSamples = benchmarkProfile.Evidence.EffectiveSamples
		decision.Audit.Candidates[auditIndex].BenchmarkRecentSamples = benchmarkProfile.Evidence.RecentSamples
		decision.Audit.Candidates[auditIndex].BenchmarkHourOfWeekSamples = benchmarkProfile.Evidence.HourOfWeekSamples
		decision.Audit.Candidates[auditIndex].BenchmarkConfidence = benchmarkProfile.Evidence.Confidence
		decision.Audit.Candidates[auditIndex].BenchmarkRecencyWeight = benchmarkProfile.Evidence.RecencyWeight
		if !benchmarkProfile.Evidence.LastObservedAt.IsZero() {
			lastObservedAt := benchmarkProfile.Evidence.LastObservedAt
			decision.Audit.Candidates[auditIndex].BenchmarkLastObservedAt = &lastObservedAt
		}
	}
	allocatorCandidates := make([]OpenAIRouteCandidate, 0, len(candidates))
	for idx, candidate := range candidates {
		passiveRouteEvidence := profileByCandidate[idx].Global.ReliabilityCount >= OpenAIRouteBenchmarkMinimumSamples
		if candidate.RouteVariant && !passiveRouteEvidence && !benchmarkAppliedByCandidate[idx] {
			fingerprint := OpenAIRouteObservationFingerprint(candidate.Key)
			auditIndex := auditIndices[idx]
			decision.Audit.Candidates[auditIndex].ExclusionReasons = append(
				decision.Audit.Candidates[auditIndex].ExclusionReasons,
				OpenAIRouteExcludedBenchmark,
			)
			decision.Audit.Exclusions = append(decision.Audit.Exclusions, OpenAIRouteExclusion{
				AccountID: candidate.Key.AccountID, RouteFingerprint: fingerprint, Reason: OpenAIRouteExcludedBenchmark,
			})
			continue
		}
		allocatorCandidates = append(allocatorCandidates, candidate)
	}

	plan, reservation, ledgers, err := allocateAndReserveOpenAIRouteWithLedgers(ctx, c.budgetStore, OpenAIRouteAllocationRequest{
		Policy:               policy,
		Candidates:           allocatorCandidates,
		EstimatedBaseCostUSD: config.EstimatedBaseCostUSD,
		Seed:                 req.Seed,
		HardShareCaps:        config.HardShareCaps,
	}, windows, decisionID, time.Minute)
	selectedAccountID := int64(0)
	selectedEndpointHash := ""
	selectedRouteFingerprint := ""
	selectedRate := 0.0
	selectedBaseCostUSD := config.EstimatedBaseCostUSD
	if err == nil {
		selectedAccountID = plan.Selected.Candidate.Key.AccountID
		selectedEndpointHash = plan.Selected.Candidate.Key.EndpointHash
		selectedRouteFingerprint = OpenAIRouteObservationFingerprint(plan.Selected.Candidate.Key)
		selectedRate = plan.Selected.Candidate.RateMultiplier
		selectedBaseCostUSD = openAIRouteCandidateEstimatedBaseCost(plan.Selected.Candidate, config.EstimatedBaseCostUSD)
	}
	decision.Audit.MinHealthyMultiplier = plan.MinHealthyMultiplier
	decision.Audit.Exclusions = append(decision.Audit.Exclusions, plan.Excluded...)
	decision.Audit.BudgetWindows = newOpenAIRouteShadowAuditBudgetWindows(
		windows,
		ledgers,
		selectedAccountID,
		selectedRate,
		selectedBaseCostUSD,
	)
	decision.Audit.AdaptiveSelectedEndpointHash = selectedEndpointHash
	decision.Audit.AdaptiveSelectedRouteFingerprint = selectedRouteFingerprint
	applyOpenAIRouteShadowAuditPlan(decision.Audit, plan, selectedRouteFingerprint)
	decision.ExcludedCount = len(decision.Audit.Exclusions)
	if err != nil {
		return decision, err
	}
	settlement := OpenAIRouteBudgetStoreSettlement{
		ReservationID:        reservation.ReservationID,
		RouteKey:             reservation.RouteKey,
		ActualBaseCostUSD:    selectedBaseCostUSD,
		ActualAccountCostUSD: selectedBaseCostUSD * plan.Selected.Candidate.RateMultiplier,
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
	decision.SelectedEndpointHash = plan.Selected.Candidate.Key.EndpointHash
	decision.SelectedRouteFingerprint = OpenAIRouteObservationFingerprint(plan.Selected.Candidate.Key)
	decision.SelectedRate = plan.Selected.Candidate.RateMultiplier
	decision.Emergency = plan.Emergency
	return decision, nil
}

// EvaluateShadows resolves every treatment at the most-specific policy scope.
// Each treatment receives an isolated cost-budget namespace and a distinct
// decision row. An invalid treatment is retained as unevaluated evidence while
// healthy siblings continue; only a shared policy-read failure aborts the batch.
func (c *OpenAIRouteController) EvaluateShadows(
	ctx context.Context,
	req OpenAIRouteShadowRequest,
) ([]OpenAIRouteShadowDecision, error) {
	if c == nil || c.reader == nil {
		return []OpenAIRouteShadowDecision{{Mode: OpenAIRoutePolicyLegacy, Reason: "runtime_unavailable"}}, nil
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	policies, err := c.loadPolicies(ctx, now)
	if err != nil {
		return nil, err
	}
	configs := resolveOpenAIRoutePolicyConfigs(policies, req.GroupID, req.Model, req.RequestClass)
	if len(configs) == 0 {
		decision, evaluateErr := c.EvaluateShadow(ctx, req)
		return []OpenAIRouteShadowDecision{decision}, evaluateErr
	}

	results := make([]OpenAIRouteShadowDecision, 0, len(configs))
	seen := make(map[string]struct{}, len(configs))
	for _, config := range configs {
		if !config.Enabled {
			continue
		}
		mode := OpenAIRoutePolicyMode(strings.TrimSpace(config.Mode))
		if mode == "" {
			mode = OpenAIRoutePolicyShadow
		}
		if mode == OpenAIRoutePolicyLegacy {
			continue
		}
		experimentID, variantID, identityErr := config.normalizedExperimentIdentity(strings.TrimSpace(config.ActivationID))
		identityKey := experimentID + "\x00" + variantID
		if identityErr == nil {
			if _, duplicate := seen[identityKey]; duplicate {
				identityErr = fmt.Errorf("%w: duplicate experiment_id/variant_id %q/%q", ErrOpenAIRouteInvalidPolicy, experimentID, variantID)
			} else {
				seen[identityKey] = struct{}{}
			}
		}
		if identityErr != nil {
			results = append(results, OpenAIRouteShadowDecision{
				Mode:         mode,
				Version:      config.Version,
				ActivationID: strings.TrimSpace(config.ActivationID),
				ExperimentID: experimentID,
				VariantID:    variantID,
				Reason:       "policy_invalid",
			})
			continue
		}

		isolated := &OpenAIRouteController{
			reader:           openAIRouteFixedPolicyReader{config: config},
			healthStore:      c.healthStore,
			budgetStore:      c.budgetStore,
			observationStore: c.observationStore,
			benchmarkStore:   c.benchmarkStore,
			observationCache: c.observationCache,
			benchmarkCache:   c.benchmarkCache,
		}
		decision, evaluateErr := isolated.EvaluateShadow(ctx, req)
		if evaluateErr != nil {
			decision.Evaluated = false
			decision.Reason = openAIRouteShadowEvaluationErrorReason(evaluateErr)
		}
		results = append(results, decision)
	}
	if len(results) == 0 {
		return []OpenAIRouteShadowDecision{{Mode: OpenAIRoutePolicyLegacy, Reason: "policy_not_enabled"}}, nil
	}
	return results, nil
}

type openAIRouteFixedPolicyReader struct {
	config openAIRoutePolicyConfig
}

func (r openAIRouteFixedPolicyReader) GetValue(context.Context, string) (string, error) {
	raw, err := json.Marshal([]openAIRoutePolicyConfig{r.config})
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (c *OpenAIRouteController) SnapshotObservationProfileCache() (OpenAIRouteObservationProfileCacheStats, bool) {
	if c == nil || c.observationCache == nil {
		return OpenAIRouteObservationProfileCacheStats{}, false
	}
	return c.observationCache.Stats(), true
}

func newOpenAIRouteShadowAuditPolicy(
	policy OpenAIRoutePolicy,
	hardShareCaps bool,
	benchmarkPriorEnabled bool,
	routeVariants []OpenAIRouteShadowAuditRouteVariant,
) OpenAIRouteShadowAuditPolicy {
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
		BenchmarkPriorEnabled:   benchmarkPriorEnabled,
		RouteVariants:           append([]OpenAIRouteShadowAuditRouteVariant(nil), routeVariants...),
	}
}

func newOpenAIRouteShadowAuditCandidate(candidate OpenAIRouteCandidate) OpenAIRouteShadowAuditCandidate {
	return OpenAIRouteShadowAuditCandidate{
		AccountID:                        candidate.Key.AccountID,
		RouteFingerprint:                 OpenAIRouteObservationFingerprint(candidate.Key),
		EndpointHash:                     candidate.Key.EndpointHash,
		RouteVariant:                     candidate.RouteVariant,
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
		EstimatedBaseCostUSD:             candidate.EstimatedBaseCostUSD,
		EstimatedAccountCostUSD:          candidate.EstimatedBaseCostUSD * candidate.RateMultiplier,
		ObservedMeanCostUSD:              candidate.ObservedMeanCostUSD,
		CostObservationSamples:           candidate.CostObservationSamples,
		CostEstimateSource:               candidate.CostEstimateSource,
	}
}

func applyOpenAIRouteShadowAuditPlan(
	snapshot *OpenAIRouteShadowAuditSnapshot,
	plan OpenAIRouteAllocationPlan,
	selectedRouteFingerprint string,
) {
	if snapshot == nil {
		return
	}
	byFingerprint := make(map[string]*OpenAIRouteShadowAuditCandidate, len(snapshot.Candidates))
	for idx := range snapshot.Candidates {
		byFingerprint[snapshot.Candidates[idx].RouteFingerprint] = &snapshot.Candidates[idx]
	}
	for rank, item := range plan.Ranked {
		fingerprint := OpenAIRouteObservationFingerprint(item.Candidate.Key)
		candidate := byFingerprint[fingerprint]
		if candidate == nil {
			continue
		}
		candidate.Rank = rank + 1
		candidate.Selected = selectedRouteFingerprint != "" && fingerprint == selectedRouteFingerprint
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
		candidate := byFingerprint[exclusion.RouteFingerprint]
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
	matches := resolveOpenAIRoutePolicyConfigs(policies, groupID, model, requestClass)
	if len(matches) == 0 {
		return openAIRoutePolicyConfig{}, false
	}
	return matches[0], true
}

func resolveOpenAIRoutePolicyConfigs(
	policies []openAIRoutePolicyConfig,
	groupID int64,
	model string,
	requestClass OpenAIRouteRequestClass,
) []openAIRoutePolicyConfig {
	bestScore := -1
	best := make([]openAIRoutePolicyConfig, 0, 1)
	for _, policy := range policies {
		score, matched := openAIRoutePolicyMatchScore(policy, groupID, model, requestClass)
		if !matched || score < bestScore {
			continue
		}
		if score > bestScore {
			bestScore = score
			best = best[:0]
		}
		best = append(best, policy)
	}
	return best
}

func openAIRoutePolicyMatchScore(
	policy openAIRoutePolicyConfig,
	groupID int64,
	model string,
	requestClass OpenAIRouteRequestClass,
) (int, bool) {
	if policy.GroupID != groupID {
		return 0, false
	}
	classPattern := strings.TrimSpace(policy.RequestClass)
	if classPattern == "" {
		classPattern = string(OpenAIRouteRequestClassText)
	}
	classScore := 0
	switch classPattern {
	case string(requestClass):
		classScore = 10_000_000
	case "*":
	default:
		return 0, false
	}
	model = strings.TrimSpace(model)
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
		return 0, false
	}
	return score + classScore, true
}

func (c openAIRoutePolicyConfig) normalizedShadowActivation(now time.Time) (string, time.Time, error) {
	activationID := strings.TrimSpace(c.ActivationID)
	if activationID == "" {
		return "", time.Time{}, fmt.Errorf("%w: activation_id is required for enabled shadow policies", ErrOpenAIRouteInvalidPolicy)
	}
	if len(activationID) > 128 {
		return "", time.Time{}, fmt.Errorf("%w: activation_id exceeds 128 bytes", ErrOpenAIRouteInvalidPolicy)
	}
	for _, char := range activationID {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("-._:", char) {
			continue
		}
		return "", time.Time{}, fmt.Errorf("%w: activation_id must use only ASCII letters, digits, dash, dot, underscore or colon", ErrOpenAIRouteInvalidPolicy)
	}

	startedRaw := strings.TrimSpace(c.ShadowStartedAt)
	if startedRaw == "" {
		return "", time.Time{}, fmt.Errorf("%w: shadow_started_at is required for enabled shadow policies", ErrOpenAIRouteInvalidPolicy)
	}
	startedAt, err := time.Parse(time.RFC3339Nano, startedRaw)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("%w: shadow_started_at must be RFC3339: %v", ErrOpenAIRouteInvalidPolicy, err)
	}
	_, offset := startedAt.Zone()
	if offset != 0 {
		return "", time.Time{}, fmt.Errorf("%w: shadow_started_at must use UTC", ErrOpenAIRouteInvalidPolicy)
	}
	startedAt = startedAt.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if startedAt.After(now) {
		return "", time.Time{}, fmt.Errorf("%w: shadow_started_at cannot be in the future", ErrOpenAIRouteInvalidPolicy)
	}
	return activationID, startedAt, nil
}

func (c openAIRoutePolicyConfig) normalizedExperimentIdentity(activationID string) (string, string, error) {
	experimentID := strings.TrimSpace(c.ExperimentID)
	if experimentID == "" {
		experimentID = strings.TrimSpace(activationID)
	}
	variantID := strings.TrimSpace(c.VariantID)
	if variantID == "" {
		variantID = "default"
	}
	if err := validateOpenAIRouteExperimentToken("experiment_id", experimentID, 128); err != nil {
		return experimentID, variantID, err
	}
	if err := validateOpenAIRouteExperimentToken("variant_id", variantID, 64); err != nil {
		return experimentID, variantID, err
	}
	return experimentID, variantID, nil
}

func validateOpenAIRouteExperimentToken(name, value string, max int) error {
	if value == "" || len(value) > max {
		return fmt.Errorf("%w: %s is required and must not exceed %d bytes", ErrOpenAIRouteInvalidPolicy, name, max)
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("-._:", char) {
			continue
		}
		return fmt.Errorf("%w: %s contains unsupported characters", ErrOpenAIRouteInvalidPolicy, name)
	}
	return nil
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
	experimentID string,
	variantID string,
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
		epoch := fmt.Sprintf(
			"v%d:%s:%s:%s",
			version,
			strings.TrimSpace(experimentID),
			strings.TrimSpace(variantID),
			now.Truncate(definition.duration).Format(time.RFC3339),
		)
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

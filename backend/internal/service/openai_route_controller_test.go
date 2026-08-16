package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAIRouteBenchmarkObservationStoreStub struct {
	*openAIRouteObservationStoreStub
	profiles map[string]OpenAIRouteBenchmarkObservationProfile
	err      error
	calls    int
}

func (s *openAIRouteBenchmarkObservationStoreStub) GetBenchmarkBatch(
	_ context.Context,
	_ []OpenAIRouteKey,
	_ time.Time,
) (map[string]OpenAIRouteBenchmarkObservationProfile, error) {
	s.calls++
	return s.profiles, s.err
}

type openAIRoutePolicyReaderStub struct {
	value string
	err   error
	calls int
}

func (s *openAIRoutePolicyReaderStub) GetValue(context.Context, string) (string, error) {
	s.calls++
	return s.value, s.err
}

type openAIRouteHealthStoreStub struct {
	states         map[OpenAIRouteHealthStoreKey]OpenAIRouteHealthState
	getCalls       int
	batchCalls     int
	omitBatchKeys  map[string]struct{}
	batchStarted   chan struct{}
	batchRelease   chan struct{}
	appliedKeys    []OpenAIRouteHealthStoreKey
	appliedEvents  []OpenAIRouteHealthEvent
	providerKeys   []OpenAIRouteKey
	providerEvents []OpenAIRouteHealthEvent
}

func (s *openAIRouteHealthStoreStub) RecordProviderEvidence(_ context.Context, key OpenAIRouteKey, event OpenAIRouteHealthEvent, _ OpenAIRoutePolicy, _ int) (OpenAIRouteProviderEvidenceResult, error) {
	s.providerKeys = append(s.providerKeys, key)
	s.providerEvents = append(s.providerEvents, event)
	return OpenAIRouteProviderEvidenceResult{}, nil
}

func (s *openAIRouteHealthStoreStub) Get(_ context.Context, key OpenAIRouteHealthStoreKey) (OpenAIRouteHealthState, error) {
	s.getCalls++
	if state, ok := s.states[key]; ok {
		return state, nil
	}
	return OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}, nil
}

func (s *openAIRouteHealthStoreStub) GetBatch(ctx context.Context, keys []OpenAIRouteHealthStoreKey) (map[string]OpenAIRouteHealthState, error) {
	s.batchCalls++
	if s.batchStarted != nil {
		select {
		case s.batchStarted <- struct{}{}:
		default:
		}
	}
	if s.batchRelease != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.batchRelease:
		}
	}
	result := make(map[string]OpenAIRouteHealthState, len(keys))
	for _, key := range keys {
		if _, omitted := s.omitBatchKeys[key.Fingerprint()]; omitted {
			continue
		}
		state := OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}
		if configured, ok := s.states[key]; ok {
			state = configured
		}
		result[key.Fingerprint()] = state
	}
	return result, nil
}

func (s *openAIRouteHealthStoreStub) ApplyEvent(_ context.Context, key OpenAIRouteHealthStoreKey, event OpenAIRouteHealthEvent, _ OpenAIRoutePolicy) (OpenAIRouteHealthState, error) {
	s.appliedKeys = append(s.appliedKeys, key)
	s.appliedEvents = append(s.appliedEvents, event)
	return OpenAIRouteHealthState{}, nil
}

func (s *openAIRouteHealthStoreStub) AcquireHalfOpenPermit(context.Context, OpenAIRouteHealthStoreKey, string) (bool, error) {
	return true, nil
}

func (s *openAIRouteHealthStoreStub) RefreshHalfOpenPermit(context.Context, OpenAIRouteHealthStoreKey, string) (bool, error) {
	return true, nil
}

func (s *openAIRouteHealthStoreStub) ReleaseHalfOpenPermit(context.Context, OpenAIRouteHealthStoreKey, string) error {
	return nil
}

type openAIRouteBudgetSnapshotStoreStub struct {
	windows        []OpenAIRouteBudgetWindowConfig
	windowBatches  [][]OpenAIRouteBudgetWindowConfig
	reserveCalls   int
	settleCalls    int
	lastReserve    OpenAIRouteBudgetStoreReserveRequest
	lastSettlement OpenAIRouteBudgetStoreSettlement
}

func (s *openAIRouteBudgetSnapshotStoreStub) GetLedgers(_ context.Context, windows []OpenAIRouteBudgetWindowConfig) ([]OpenAIRouteBudgetLedger, error) {
	s.windows = append([]OpenAIRouteBudgetWindowConfig(nil), windows...)
	s.windowBatches = append(s.windowBatches, append([]OpenAIRouteBudgetWindowConfig(nil), windows...))
	ledgers := make([]OpenAIRouteBudgetLedger, len(windows))
	for i, window := range windows {
		ledgers[i] = window.EmptyLedger()
	}
	return ledgers, nil
}

func TestOpenAIRouteControllerEvaluateShadowsIsolatesParallelVariants(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `{"policies":[
		{"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow","policy_version":21,"activation_id":"activation-21","shadow_started_at":"2026-08-01T00:00:00Z","experiment_id":"scheduler-v2","variant_id":"reliability","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,"latency_beta":1.0},
		{"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow","policy_version":21,"activation_id":"activation-21","shadow_started_at":"2026-08-01T00:00:00Z","experiment_id":"scheduler-v2","variant_id":"latency","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,"latency_beta":2.0}
	]}`}
	budget := &openAIRouteBudgetSnapshotStoreStub{}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, budget, &openAIRouteObservationStoreStub{})
	now := time.Date(2026, 8, 16, 3, 0, 0, 0, time.UTC)
	decisions, err := controller.EvaluateShadows(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText, Now: now,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: testOpenAIRouteControllerAccount(1, 0.15), Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	})
	require.NoError(t, err)
	require.Len(t, decisions, 2)
	require.Equal(t, "scheduler-v2", decisions[0].ExperimentID)
	require.Equal(t, "reliability", decisions[0].VariantID)
	require.Equal(t, "latency", decisions[1].VariantID)
	require.NotEqual(t, decisions[0].DecisionID, decisions[1].DecisionID)
	require.True(t, decisions[0].Evaluated)
	require.True(t, decisions[1].Evaluated)
	require.Len(t, budget.windowBatches, 2)
	require.Contains(t, budget.windowBatches[0][0].Scope.Epoch, ":scheduler-v2:reliability:")
	require.Contains(t, budget.windowBatches[1][0].Scope.Epoch, ":scheduler-v2:latency:")
}

func (s *openAIRouteBudgetSnapshotStoreStub) Reserve(_ context.Context, req OpenAIRouteBudgetStoreReserveRequest) (OpenAIRouteBudgetStoreReservation, error) {
	s.reserveCalls++
	s.lastReserve = req
	return OpenAIRouteBudgetStoreReservation{
		ReservationID:  req.ReservationID,
		RouteKey:       req.RouteKey,
		Allowed:        true,
		RejectedWindow: -1,
	}, nil
}

func (s *openAIRouteBudgetSnapshotStoreStub) Settle(_ context.Context, settlement OpenAIRouteBudgetStoreSettlement) error {
	s.settleCalls++
	s.lastSettlement = settlement
	if settlement.ReservationID == "" || !settlement.RouteKey.Valid() {
		return errors.New("invalid shadow settlement")
	}
	return nil
}

func (s *openAIRouteBudgetSnapshotStoreStub) Cancel(context.Context, OpenAIRouteBudgetStoreSettlement) error {
	return errors.New("shadow evaluation must not cancel budget")
}

func TestOpenAIRouteController_MissingPolicyIsLegacyAndCached(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{err: ErrSettingNotFound}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, &openAIRouteObservationStoreStub{})
	req := OpenAIRouteShadowRequest{
		GroupID:      7,
		Model:        "gpt-5.6-sol",
		RequestClass: OpenAIRouteRequestClassText,
		Now:          time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC),
		Candidates: []OpenAIRouteShadowCandidate{{
			Account:   testOpenAIRouteControllerAccount(1, 0.15),
			Endpoint:  "https://example.invalid/v1/responses",
			Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	}

	first, err := controller.EvaluateShadow(context.Background(), req)
	require.NoError(t, err)
	require.False(t, first.Evaluated)
	require.Equal(t, OpenAIRoutePolicyLegacy, first.Mode)
	require.Equal(t, "policy_not_enabled", first.Reason)

	req.Now = req.Now.Add(time.Second)
	second, err := controller.EvaluateShadow(context.Background(), req)
	require.NoError(t, err)
	require.False(t, second.Evaluated)
	require.Equal(t, 1, reader.calls)

	controller.InvalidatePolicyCache()
	req.Now = req.Now.Add(time.Second)
	_, err = controller.EvaluateShadow(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, 2, reader.calls)
}

func TestOpenAIRouteController_ExactPolicyBeatsWildcardAndSelectsWithinBudget(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `{
  "policies": [
    {"group_id":7,"model":"gpt-*","enabled":true,"mode":"shadow","policy_version":1,"activation_id":"test-activation-1","shadow_started_at":"2026-08-01T00:00:00Z","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01},
    {"group_id":7,"model":"gpt-5.6-sol","enabled":true,"mode":"shadow","policy_version":9,"activation_id":"test-activation-9","shadow_started_at":"2026-08-01T00:00:00Z","target_avg_multiplier":0.155,"hard_avg_multiplier":0.18,"estimated_base_cost_usd":0.01}
  ]
}`}
	budget := &openAIRouteBudgetSnapshotStoreStub{}
	health := &openAIRouteHealthStoreStub{}
	controller := NewOpenAIRouteController(reader, health, budget, &openAIRouteObservationStoreStub{})
	now := time.Date(2026, 8, 8, 12, 3, 0, 0, time.UTC)

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID:      7,
		Model:        "gpt-5.6-sol",
		RequestClass: OpenAIRouteRequestClassText,
		Seed:         42,
		Now:          now,
		Candidates: []OpenAIRouteShadowCandidate{
			{
				Account:              testOpenAIRouteControllerAccount(1, 0.15),
				Endpoint:             "https://cheap.example.invalid/v1/responses",
				Transport:            string(OpenAIUpstreamTransportHTTPSSE),
				Priority:             10,
				HasReliabilitySample: true,
				SuccessLowerBound:    0.95,
				TTFTMilliseconds:     1000,
			},
			{
				Account:              testOpenAIRouteControllerAccount(2, 0.20),
				Endpoint:             "https://fast.example.invalid/v1/responses",
				Transport:            string(OpenAIUpstreamTransportHTTPSSE),
				Priority:             1,
				HasReliabilitySample: true,
				SuccessLowerBound:    0.99,
				TTFTMilliseconds:     100,
			},
		},
	})
	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.Equal(t, OpenAIRoutePolicyShadow, decision.Mode)
	require.Equal(t, 9, decision.Version)
	require.Equal(t, "test-activation-9", decision.ActivationID)
	require.Equal(t, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), decision.ShadowStartedAt)
	require.Equal(t, int64(1), decision.SelectedAccountID, "the 0.20 route has no earned credit and must be budget-filtered")
	require.InDelta(t, 0.15, decision.SelectedRate, 1e-12)
	require.NotEmpty(t, decision.DecisionID)
	require.GreaterOrEqual(t, decision.EvaluationDurationMicros, int64(0))
	require.NotNil(t, decision.Audit)
	require.Equal(t, OpenAIRouteRequestClassText, decision.Audit.RequestClass)
	require.Equal(t, decision.ActivationID, decision.Audit.ActivationID)
	require.Equal(t, decision.ShadowStartedAt, decision.Audit.ShadowStartedAt)
	require.InDelta(t, 0.155, decision.Audit.Policy.TargetAverageMultiplier, 1e-12)
	require.Len(t, decision.Audit.Candidates, 2)
	require.Len(t, decision.Audit.BudgetWindows, 3)
	require.True(t, decision.Audit.BudgetWindows[0].ProjectionValid)
	require.Equal(t, int64(1), decision.Audit.Candidates[0].AccountID)
	require.NotEmpty(t, decision.Audit.Candidates[0].EndpointHash)
	require.Empty(t, decision.Audit.Candidates[0].ExclusionReasons)
	require.Contains(t, decision.Audit.Candidates[1].ExclusionReasons, OpenAIRouteExcludedCost)
	require.Len(t, budget.windows, 3)
	require.Equal(t, 1, budget.reserveCalls)
	require.Equal(t, 1, budget.settleCalls)
	require.Equal(t, "5m", budget.windows[0].Scope.Window)
	require.Contains(t, budget.windows[0].Scope.Epoch, "v9:")
	require.Equal(t, 1, health.batchCalls, "route and provider state must share one batch read")
	require.Zero(t, health.getCalls, "Shadow evaluation must not issue sequential health reads")
}

func TestOpenAIRouteController_SharedObservationsOverrideProcessLocalInputs(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{"group_id":7,"model":"gpt-5.6-sol","enabled":true,"mode":"shadow","policy_version":11,"activation_id":"test-activation-11","shadow_started_at":"2026-08-01T00:00:00Z","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01}]`}
	firstAccount := testOpenAIRouteControllerAccount(1, 0.15)
	secondAccount := testOpenAIRouteControllerAccount(2, 0.15)
	firstKey, err := NewOpenAIRouteKey(firstAccount, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://slow.example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	secondKey, err := NewOpenAIRouteKey(secondAccount, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://fast.example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)

	profile := func(successes uint64, ttft int64) OpenAIRouteObservationProfile {
		aggregate := NewOpenAIRouteObservationAggregate()
		aggregate.AttemptCount = 100
		aggregate.ReliabilityCount = 100
		aggregate.SuccessCount = successes
		aggregate.FailureCount = 100 - successes
		aggregate.TTFTSampleCount = successes
		aggregate.TTFTHistogram[OpenAIRouteLatencyHistogramBucket(ttft)] = successes
		aggregate.LatencySampleCount = successes
		aggregate.LatencyHistogram[OpenAIRouteLatencyHistogramBucket(ttft*4)] = successes
		return OpenAIRouteObservationProfile{Global: aggregate, Recent: aggregate}
	}
	observations := &openAIRouteObservationStoreStub{profiles: map[string]OpenAIRouteObservationProfile{
		OpenAIRouteObservationFingerprint(firstKey):  profile(50, 5_000),
		OpenAIRouteObservationFingerprint(secondKey): profile(99, 500),
	}}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, observations)
	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText, Seed: 42,
		Candidates: []OpenAIRouteShadowCandidate{
			{Account: firstAccount, Endpoint: "https://slow.example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE), HasReliabilitySample: true, SuccessLowerBound: 0.99, TTFTMilliseconds: 100},
			{Account: secondAccount, Endpoint: "https://fast.example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE), HasReliabilitySample: true, SuccessLowerBound: 0.50, TTFTMilliseconds: 10_000},
		},
	})
	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.Len(t, decision.Audit.Candidates, 2)
	byID := map[int64]OpenAIRouteShadowAuditCandidate{}
	for _, candidate := range decision.Audit.Candidates {
		byID[candidate.AccountID] = candidate
	}
	require.Equal(t, "shared", byID[1].ObservationSource)
	require.Equal(t, uint64(100), byID[1].ObservationSamples)
	require.Less(t, byID[1].SuccessLowerBound, byID[2].SuccessLowerBound)
	require.Greater(t, byID[1].TTFTMilliseconds, byID[2].TTFTMilliseconds)
	require.InDelta(t, 0.5, byID[1].CurrentAccountShare, 1e-12)
}

func TestOpenAIRouteControllerBenchmarkPriorIsOptInAndDefaultAuditIsCompatible(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":17,"activation_id":"test-activation-17","shadow_started_at":"2026-08-01T00:00:00Z",
		"target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01
	}]`}
	store := &openAIRouteBenchmarkObservationStoreStub{openAIRouteObservationStoreStub: &openAIRouteObservationStoreStub{}}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, store)

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: testOpenAIRouteControllerAccount(1, 0.15), Endpoint: "https://hk.pomoai.xyz/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	})

	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.Zero(t, store.calls, "existing Shadow policies must not query the operational benchmark table")
	encoded, err := json.Marshal(decision.Audit)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "benchmark_", "default-disabled code must not perturb existing audit snapshots")
}

func TestOpenAIRouteControllerAppliesAuditableBoundedBenchmarkPrior(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":18,"activation_id":"test-activation-18","shadow_started_at":"2026-08-01T00:00:00Z",
		"target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,
		"benchmark_prior_enabled":true
	}]`}
	account := testOpenAIRouteControllerAccount(23, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://hk.pomoai.xyz/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	now := time.Date(2026, 8, 15, 8, 30, 0, 0, time.UTC)
	benchmark := FinalizeOpenAIRouteBenchmarkObservationProfile(testOpenAIRouteBenchmarkProfile(80, 80, now), now)
	store := &openAIRouteBenchmarkObservationStoreStub{
		openAIRouteObservationStoreStub: &openAIRouteObservationStoreStub{},
		profiles: map[string]OpenAIRouteBenchmarkObservationProfile{
			OpenAIRouteObservationFingerprint(key): benchmark,
		},
	}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, store)

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText, Now: now,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: account, Endpoint: "https://hk.pomoai.xyz/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	})

	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.Equal(t, 1, store.calls)
	require.True(t, decision.Audit.Policy.BenchmarkPriorEnabled)
	require.Len(t, decision.Audit.Candidates, 1)
	candidate := decision.Audit.Candidates[0]
	require.Equal(t, "active_benchmark_prior", candidate.ObservationSource)
	require.Zero(t, candidate.ObservationSamples, "active evidence must not masquerade as passive traffic")
	require.Equal(t, uint64(80), candidate.BenchmarkSamples)
	require.Equal(t, uint64(20), candidate.BenchmarkEffectiveSamples)
	require.InDelta(t, 0.25, candidate.BenchmarkConfidence, 1e-12)
	require.NotNil(t, candidate.BenchmarkLastObservedAt)
	require.Equal(t, now, *candidate.BenchmarkLastObservedAt)
	_, err = controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText, Now: now,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: account, Endpoint: "https://hk.pomoai.xyz/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	})
	require.NoError(t, err)
	require.Equal(t, 1, store.calls, "benchmark reads must be removed from the per-request Shadow hot path")
}

func TestOpenAIRouteControllerRouteVariantsAreShadowOnlyRouteScopedAndEvidenceGated(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":6,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":20,"activation_id":"test-activation-20","shadow_started_at":"2026-08-01T00:00:00Z",
		"target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,
		"benchmark_prior_enabled":true,
		"route_variants":[
			{"account_id":23,"base_url":"https://hk.pomoai.xyz"},
			{"account_id":23,"base_url":"https://jp.pomoai.xyz"}
		]
	}]`}
	account := testOpenAIRouteControllerAccount(23, 0.15)
	hkEndpoint := "https://hk.pomoai.xyz/v1/responses"
	jpEndpoint := "https://jp.pomoai.xyz/v1/responses"
	hkKey, err := NewOpenAIRouteKey(account, 6, "gpt-5.6-sol", OpenAIRouteRequestClassText, hkEndpoint, string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	jpKey, err := NewOpenAIRouteKey(account, 6, "gpt-5.6-sol", OpenAIRouteRequestClassText, jpEndpoint, string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	now := time.Date(2026, 8, 15, 8, 30, 0, 0, time.UTC)
	passive := NewOpenAIRouteObservationAggregate()
	passive.AttemptCount = 100
	passive.ReliabilityCount = 100
	passive.SuccessCount = 100
	passive.TTFTSampleCount = 100
	passive.TTFTHistogram[OpenAIRouteLatencyHistogramBucket(1_000)] = 100
	passive.LatencySampleCount = 100
	passive.LatencyHistogram[OpenAIRouteLatencyHistogramBucket(3_000)] = 100
	store := &openAIRouteBenchmarkObservationStoreStub{
		openAIRouteObservationStoreStub: &openAIRouteObservationStoreStub{profiles: map[string]OpenAIRouteObservationProfile{
			OpenAIRouteObservationFingerprint(hkKey): {Global: passive, Recent: passive},
		}},
		profiles: map[string]OpenAIRouteBenchmarkObservationProfile{
			OpenAIRouteObservationFingerprint(jpKey): FinalizeOpenAIRouteBenchmarkObservationProfile(
				testOpenAIRouteBenchmarkProfile(1, 1, now), now,
			),
		},
	}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, store)

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 6, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText, Now: now, Seed: 42,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: account, Endpoint: hkEndpoint, Transport: string(OpenAIUpstreamTransportHTTPSSE), Priority: 1,
		}},
	})

	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.Equal(t, int64(23), decision.SelectedAccountID)
	require.Equal(t, hkKey.EndpointHash, decision.SelectedEndpointHash)
	require.Equal(t, OpenAIRouteObservationFingerprint(hkKey), decision.SelectedRouteFingerprint)
	require.Equal(t, 2, decision.CandidateCount)
	require.Equal(t, 1, decision.ExcludedCount)
	require.Len(t, decision.Audit.Policy.RouteVariants, 2)
	require.Len(t, decision.Audit.Candidates, 2)
	byEndpoint := make(map[string]OpenAIRouteShadowAuditCandidate)
	for _, candidate := range decision.Audit.Candidates {
		byEndpoint[candidate.EndpointHash] = candidate
		require.InDelta(t, 1, candidate.CurrentAccountShare, 1e-12, "same-account endpoints must share one account concentration")
		require.InDelta(t, 1, candidate.CurrentProviderShare, 1e-12, "same credential failure domain must share provider concentration")
	}
	require.False(t, byEndpoint[hkKey.EndpointHash].RouteVariant)
	require.True(t, byEndpoint[hkKey.EndpointHash].Selected)
	jpAudit := byEndpoint[jpKey.EndpointHash]
	require.True(t, jpAudit.RouteVariant)
	require.Equal(t, uint64(1), jpAudit.BenchmarkSamples)
	require.Zero(t, jpAudit.BenchmarkEffectiveSamples)
	require.Contains(t, jpAudit.ExclusionReasons, OpenAIRouteExcludedBenchmark)
	require.False(t, jpAudit.Selected)
	require.Len(t, decision.Audit.Exclusions, 1)
	require.Equal(t, OpenAIRouteObservationFingerprint(jpKey), decision.Audit.Exclusions[0].RouteFingerprint)
	encoded, err := json.Marshal(decision.Audit)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "pomoai.xyz", "raw Base URLs must never enter durable Shadow audit")
}

func TestOpenAIRouteControllerRouteVariantPlanMapsSelectionByFingerprint(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":6,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":21,"activation_id":"test-activation-21","shadow_started_at":"2026-08-01T00:00:00Z",
		"target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,
		"benchmark_prior_enabled":true,
		"route_variants":[{"account_id":23,"base_url":"https://jp.pomoai.xyz"}]
	}]`}
	account := testOpenAIRouteControllerAccount(23, 0.15)
	now := time.Date(2026, 8, 15, 8, 30, 0, 0, time.UTC)
	hkKey, err := NewOpenAIRouteKey(account, 6, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://hk.pomoai.xyz/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	jpKey, err := NewOpenAIRouteKey(account, 6, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://jp.pomoai.xyz/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	poorPassive := NewOpenAIRouteObservationAggregate()
	poorPassive.AttemptCount = 100
	poorPassive.ReliabilityCount = 100
	poorPassive.SuccessCount = 50
	poorPassive.FailureCount = 50
	poorPassive.TTFTSampleCount = 50
	poorPassive.TTFTHistogram[OpenAIRouteLatencyHistogramBucket(8_000)] = 50
	store := &openAIRouteBenchmarkObservationStoreStub{
		openAIRouteObservationStoreStub: &openAIRouteObservationStoreStub{profiles: map[string]OpenAIRouteObservationProfile{
			OpenAIRouteObservationFingerprint(hkKey): {Global: poorPassive},
		}},
		profiles: map[string]OpenAIRouteBenchmarkObservationProfile{
			OpenAIRouteObservationFingerprint(jpKey): FinalizeOpenAIRouteBenchmarkObservationProfile(
				testOpenAIRouteBenchmarkProfile(80, 80, now), now,
			),
		},
	}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, store)

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 6, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText, Now: now, Seed: 42,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: account, Endpoint: "https://hk.pomoai.xyz/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE), Priority: 1,
		}},
	})

	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.Zero(t, decision.ExcludedCount)
	selectedCount := 0
	for _, candidate := range decision.Audit.Candidates {
		if candidate.Selected {
			selectedCount++
			require.Equal(t, decision.SelectedEndpointHash, candidate.EndpointHash)
			require.Equal(t, decision.SelectedRouteFingerprint, candidate.RouteFingerprint)
		}
	}
	require.Equal(t, 1, selectedCount, "duplicate account IDs must not mark both endpoint variants selected")
}

func TestOpenAIRouteControllerRejectsUnsupportedBenchmarkPriorPolicies(t *testing.T) {
	benchmarkStore := &openAIRouteBenchmarkObservationStoreStub{openAIRouteObservationStoreStub: &openAIRouteObservationStoreStub{}}
	tests := []struct {
		name         string
		requestClass OpenAIRouteRequestClass
		store        OpenAIRouteObservationStore
	}{
		{name: "image", requestClass: OpenAIRouteRequestClassImage, store: benchmarkStore},
		{name: "missing benchmark store", requestClass: OpenAIRouteRequestClassText, store: &openAIRouteObservationStoreStub{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reader := &openAIRoutePolicyReaderStub{value: `[{
				"group_id":7,"model":"*","request_class":"*","enabled":true,"mode":"shadow",
				"policy_version":19,"activation_id":"test-activation-19","shadow_started_at":"2026-08-01T00:00:00Z",
				"target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,
				"benchmark_prior_enabled":true
			}]`}
			controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, test.store)
			model := "gpt-5.6-sol"
			endpoint := "https://hk.pomoai.xyz/v1/responses"
			if test.requestClass == OpenAIRouteRequestClassImage {
				model = "gpt-image-2"
				endpoint = "https://hk.pomoai.xyz/v1/images/generations"
			}

			decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
				GroupID: 7, Model: model, RequestClass: test.requestClass,
				Candidates: []OpenAIRouteShadowCandidate{{
					Account: testOpenAIRouteControllerAccount(23, 0.15), Endpoint: endpoint, Transport: string(OpenAIUpstreamTransportHTTPSSE),
				}},
			})

			require.ErrorIs(t, err, ErrOpenAIRouteInvalidPolicy)
			require.False(t, decision.Evaluated)
		})
	}
}

func TestOpenAIRouteControllerRejectsRouteVariantsWithoutBenchmarkPrior(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":6,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":22,"activation_id":"test-activation-22","shadow_started_at":"2026-08-01T00:00:00Z",
		"target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,
		"route_variants":[{"account_id":23,"base_url":"https://jp.pomoai.xyz"}]
	}]`}
	health := &openAIRouteHealthStoreStub{}
	controller := NewOpenAIRouteController(reader, health, &openAIRouteBudgetSnapshotStoreStub{}, &openAIRouteObservationStoreStub{})

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 6, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: testOpenAIRouteControllerAccount(23, 0.15), Endpoint: "https://hk.pomoai.xyz/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	})

	require.ErrorIs(t, err, ErrOpenAIRouteInvalidPolicy)
	require.False(t, decision.Evaluated)
	require.Zero(t, health.batchCalls)
}

func TestOpenAIRouteControllerUsesRouteSpecificSettledTextCostInBudgetAndAudit(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":15,"activation_id":"test-activation-15","shadow_started_at":"2026-08-01T00:00:00Z","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.10
	}]`}
	account := testOpenAIRouteControllerAccount(1, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	aggregate := NewOpenAIRouteObservationAggregate()
	aggregate.AttemptCount = 80
	aggregate.ReliabilityCount = 80
	aggregate.SuccessCount = 80
	aggregate.ActualCostSamples = 80
	aggregate.ActualBaseCostUSD = 16
	store := &openAIRouteObservationStoreStub{profiles: map[string]OpenAIRouteObservationProfile{
		OpenAIRouteObservationFingerprint(key): {Global: aggregate},
	}}
	budget := &openAIRouteBudgetSnapshotStoreStub{}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, budget, store)

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText, Seed: 42,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: account, Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	})

	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.Len(t, decision.Audit.Candidates, 1)
	candidate := decision.Audit.Candidates[0]
	require.Equal(t, OpenAIRouteCostEstimateSourceSettledText, candidate.CostEstimateSource)
	require.Equal(t, uint64(80), candidate.CostObservationSamples)
	require.InDelta(t, 0.20, candidate.ObservedMeanCostUSD, 1e-12)
	require.InDelta(t, 0.18, candidate.EstimatedBaseCostUSD, 1e-12)
	require.InDelta(t, 0.027, candidate.EstimatedAccountCostUSD, 1e-12)
	require.InDelta(t, 0.18, budget.lastReserve.EstimatedBaseCostUSD, 1e-12)
	require.InDelta(t, 0.18, budget.lastSettlement.ActualBaseCostUSD, 1e-12)
	require.InDelta(t, 0.027, budget.lastSettlement.ActualAccountCostUSD, 1e-12)
	require.InDelta(t, 0.10, decision.Audit.EstimatedBaseCostUSD, 1e-12, "the configured prior remains separately auditable")
}

func TestOpenAIRouteControllerNeverLearnsImageCostFromObservations(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":7,"model":"gpt-image-2","request_class":"image","enabled":true,"mode":"shadow",
		"policy_version":16,"activation_id":"test-activation-16","shadow_started_at":"2026-08-01T00:00:00Z","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.30
	}]`}
	account := testOpenAIRouteControllerAccount(1, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-image-2", OpenAIRouteRequestClassImage, "https://example.invalid/v1/images/generations", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	aggregate := NewOpenAIRouteObservationAggregate()
	aggregate.ActualCostSamples = 1_000
	aggregate.ActualBaseCostUSD = 1
	store := &openAIRouteObservationStoreStub{profiles: map[string]OpenAIRouteObservationProfile{
		OpenAIRouteObservationFingerprint(key): {Global: aggregate},
	}}
	budget := &openAIRouteBudgetSnapshotStoreStub{}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, budget, store)

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7, Model: "gpt-image-2", RequestClass: OpenAIRouteRequestClassImage, Seed: 42,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: account, Endpoint: "https://example.invalid/v1/images/generations", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	})

	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.Len(t, decision.Audit.Candidates, 1)
	candidate := decision.Audit.Candidates[0]
	require.Equal(t, OpenAIRouteCostEstimateSourcePolicy, candidate.CostEstimateSource)
	require.Zero(t, candidate.CostObservationSamples)
	require.InDelta(t, 0.30, candidate.EstimatedBaseCostUSD, 1e-12)
	require.InDelta(t, 0.30, budget.lastReserve.EstimatedBaseCostUSD, 1e-12)
}

func TestOpenAIRouteControllerCachesSharedObservationReads(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":12,"activation_id":"test-activation-12","shadow_started_at":"2026-08-01T00:00:00Z","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01
	}]`}
	account := testOpenAIRouteControllerAccount(1, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	store := &openAIRouteProfileCacheStoreStub{profiles: testOpenAIRouteProfileCacheProfiles(key)}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, store)
	req := OpenAIRouteShadowRequest{
		GroupID: 7, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText,
		Now: time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC),
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: account, Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	}

	first, err := controller.EvaluateShadow(context.Background(), req)
	require.NoError(t, err)
	require.True(t, first.Evaluated)
	second, err := controller.EvaluateShadow(context.Background(), req)
	require.NoError(t, err)
	require.True(t, second.Evaluated)
	require.Equal(t, uint64(1), store.calls.Load())
	stats, available := controller.SnapshotObservationProfileCache()
	require.True(t, available)
	require.Equal(t, uint64(1), stats.Hits)
	require.Equal(t, uint64(1), stats.Loads)

	gateway := &OpenAIGatewayService{openAIRouteEvaluator: controller}
	gatewayStats, available := gateway.SnapshotOpenAIRouteObservationProfileCache()
	require.True(t, available)
	require.Equal(t, stats, gatewayStats)
	ops := &OpsService{
		openAIGatewayService:    gateway,
		openAIRouteAuditService: NewOpenAIRouteAuditService(&openAIRouteDecisionRepositoryStub{}),
	}
	health := ops.GetOpenAIRouteAuditHealth(context.Background())
	require.NotNil(t, health.ObservationProfileCache)
	require.Equal(t, uint64(1), health.ObservationProfileCache.Hits)
}

func TestOpenAIRouteControllerReadsHealthAndObservationsConcurrently(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":14,"activation_id":"test-activation-14","shadow_started_at":"2026-08-01T00:00:00Z","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01
	}]`}
	account := testOpenAIRouteControllerAccount(1, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	healthRelease := make(chan struct{})
	health := &openAIRouteHealthStoreStub{batchStarted: make(chan struct{}, 1), batchRelease: healthRelease}
	profileRelease := make(chan struct{})
	store := &openAIRouteProfileCacheStoreStub{
		started: make(chan struct{}, 1), release: profileRelease,
		profiles: testOpenAIRouteProfileCacheProfiles(key),
	}
	controller := NewOpenAIRouteController(reader, health, &openAIRouteBudgetSnapshotStoreStub{}, store)
	controller.observationCache = newOpenAIRouteObservationProfileCache(store, time.Second, time.Second, 8)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	type result struct {
		decision OpenAIRouteShadowDecision
		err      error
	}
	done := make(chan result, 1)
	go func() {
		decision, evaluateErr := controller.EvaluateShadow(ctx, OpenAIRouteShadowRequest{
			GroupID: 7, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText,
			Now: time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC),
			Candidates: []OpenAIRouteShadowCandidate{{
				Account: account, Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
			}},
		})
		done <- result{decision: decision, err: evaluateErr}
	}()

	for name, started := range map[string]<-chan struct{}{"health": health.batchStarted, "observations": store.started} {
		select {
		case <-started:
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("%s read did not start while the other dependency was blocked", name)
		}
	}
	close(healthRelease)
	close(profileRelease)
	resultValue := <-done
	require.NoError(t, resultValue.err)
	require.True(t, resultValue.decision.Evaluated)
}

func TestOpenAIRouteController_RealOutcomeUpdatesOnlyNarrowRouteHealth(t *testing.T) {
	health := &openAIRouteHealthStoreStub{}
	controller := NewOpenAIRouteController(&openAIRoutePolicyReaderStub{err: ErrSettingNotFound}, health, &openAIRouteBudgetSnapshotStoreStub{}, &openAIRouteObservationStoreStub{})
	key, err := NewOpenAIRouteKey(testOpenAIRouteControllerAccount(1, 0.15), 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	require.NoError(t, controller.RecordOpenAIRouteOutcome(context.Background(), OpenAIRouteObservation{
		Key: key, ObservedAt: time.Now().UTC(), FailureClass: OpenAIRouteFailureRateLimit, PenalizeRoute: true,
	}))
	require.Len(t, health.appliedKeys, 1)
	require.Equal(t, OpenAIRouteHealthScopeRoute, health.appliedKeys[0].Scope)
	require.Equal(t, OpenAIRouteFailureRateLimit, health.appliedEvents[0].FailureClass)
	require.Empty(t, health.providerEvents, "an account-isolated route must never create provider evidence")
}

func TestOpenAIRouteController_EscalatesOnlyCorrelatableSharedDomainFailures(t *testing.T) {
	health := &openAIRouteHealthStoreStub{}
	controller := NewOpenAIRouteController(&openAIRoutePolicyReaderStub{err: ErrSettingNotFound}, health, &openAIRouteBudgetSnapshotStoreStub{}, &openAIRouteObservationStoreStub{})
	account := testOpenAIRouteControllerAccount(1, 0.15)
	account.Extra[OpenAIRouteFailureDomainExtraKey] = "pomoai"
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	now := time.Now().UTC()

	require.NoError(t, controller.RecordOpenAIRouteOutcome(context.Background(), OpenAIRouteObservation{
		Key: key, ObservedAt: now, FailureClass: OpenAIRouteFailureRateLimit, PenalizeRoute: true,
	}))
	require.Empty(t, health.providerEvents, "rate limits stay on the narrow route")

	require.NoError(t, controller.RecordOpenAIRouteOutcome(context.Background(), OpenAIRouteObservation{
		Key: key, ObservedAt: now.Add(time.Millisecond), FailureClass: OpenAIRouteFailureUpstream5xx, PenalizeRoute: true,
	}))
	require.Len(t, health.providerEvents, 1)
	require.Equal(t, OpenAIRouteFailureUpstream5xx, health.providerEvents[0].FailureClass)
}

func TestResolveOpenAIRoutePolicyConfigSeparatesRequestClasses(t *testing.T) {
	policies := []openAIRoutePolicyConfig{
		{GroupID: 7, Model: "gpt-*", Version: 1},
		{GroupID: 7, Model: "gpt-*", RequestClass: "*", Version: 2},
		{GroupID: 7, Model: "gpt-5.6-sol", RequestClass: "image", Version: 3},
	}

	text, ok := resolveOpenAIRoutePolicyConfig(policies, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText)
	require.True(t, ok)
	require.Equal(t, 1, text.Version, "an exact text-class policy must beat an all-class wildcard")

	image, ok := resolveOpenAIRoutePolicyConfig(policies, 7, "gpt-5.6-sol", OpenAIRouteRequestClassImage)
	require.True(t, ok)
	require.Equal(t, 3, image.Version)

	legacyOnly, ok := resolveOpenAIRoutePolicyConfig(policies[:1], 7, "gpt-5.6-sol", OpenAIRouteRequestClassImage)
	require.False(t, ok, "a pre-request-class text policy must not govern images")
	require.Zero(t, legacyOnly.Version)
}

func TestOpenAIRouteController_EnforceModeIsHardDisabled(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{"group_id":7,"model":"*","enabled":true,"mode":"enforce","policy_version":1,"activation_id":"test-activation-1","shadow_started_at":"2026-08-01T00:00:00Z"}]`}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, &openAIRouteObservationStoreStub{})

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID:      7,
		Model:        "gpt-5.6-sol",
		RequestClass: OpenAIRouteRequestClassText,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account:   testOpenAIRouteControllerAccount(1, 0.15),
			Endpoint:  "https://example.invalid/v1/responses",
			Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	})
	require.ErrorIs(t, err, ErrOpenAIRouteEnforceDisabled)
	require.False(t, decision.Evaluated)
	require.Equal(t, OpenAIRoutePolicyEnforce, decision.Mode)
}

func TestOpenAIRouteController_EnabledShadowRequiresImmutableActivationIdentityAndUTCStart(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	tests := map[string]string{
		"missing activation": `{"group_id":7,"model":"*","enabled":true,"mode":"shadow","policy_version":1,"shadow_started_at":"2026-08-01T00:00:00Z"}`,
		"invalid activation": `{"group_id":7,"model":"*","enabled":true,"mode":"shadow","policy_version":1,"activation_id":"bad activation","shadow_started_at":"2026-08-01T00:00:00Z"}`,
		"missing start":      `{"group_id":7,"model":"*","enabled":true,"mode":"shadow","policy_version":1,"activation_id":"activation-1"}`,
		"non utc start":      `{"group_id":7,"model":"*","enabled":true,"mode":"shadow","policy_version":1,"activation_id":"activation-1","shadow_started_at":"2026-08-01T08:00:00+08:00"}`,
		"future start":       `{"group_id":7,"model":"*","enabled":true,"mode":"shadow","policy_version":1,"activation_id":"activation-1","shadow_started_at":"2026-08-09T00:00:00Z"}`,
	}
	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			health := &openAIRouteHealthStoreStub{}
			budget := &openAIRouteBudgetSnapshotStoreStub{}
			controller := NewOpenAIRouteController(&openAIRoutePolicyReaderStub{value: raw}, health, budget, &openAIRouteObservationStoreStub{})
			decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
				GroupID: 7, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText, Now: now,
				Candidates: []OpenAIRouteShadowCandidate{{
					Account: testOpenAIRouteControllerAccount(1, 0.15), Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
				}},
			})

			require.ErrorIs(t, err, ErrOpenAIRouteInvalidPolicy)
			require.False(t, decision.Evaluated)
			require.NotNil(t, decision.Audit, "matched invalid policy remains diagnosable without affecting Legacy routing")
			require.Zero(t, health.batchCalls)
			require.Zero(t, budget.reserveCalls)
		})
	}
}

func TestOpenAIRouteController_BudgetFailureDoesNotClaimASelectedCandidate(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `{
  "group_id":7,
  "model":"gpt-5.6-sol",
  "enabled":true,
  "mode":"shadow",
  "policy_version":10,"activation_id":"test-activation-10","shadow_started_at":"2026-08-01T00:00:00Z",
  "target_avg_multiplier":0.10,
  "hard_avg_multiplier":0.10,
  "estimated_base_cost_usd":0.01
}`}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, &openAIRouteObservationStoreStub{})

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID:      7,
		Model:        "gpt-5.6-sol",
		RequestClass: OpenAIRouteRequestClassText,
		Seed:         42,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account:   testOpenAIRouteControllerAccount(1, 0.15),
			Endpoint:  "https://example.invalid/v1/responses",
			Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	})
	require.ErrorIs(t, err, ErrOpenAIRouteBudgetExhausted)
	require.False(t, decision.Evaluated)
	require.Zero(t, decision.SelectedAccountID)
	require.NotNil(t, decision.Audit)
	require.Len(t, decision.Audit.Candidates, 1)
	require.False(t, decision.Audit.Candidates[0].Selected)
	require.Len(t, decision.Audit.BudgetWindows, 3)
	for _, window := range decision.Audit.BudgetWindows {
		require.False(t, window.ProjectionValid)
	}
}

func TestOpenAIRouteControllerRejectsIncompleteHealthBatch(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":13,"activation_id":"test-activation-13","shadow_started_at":"2026-08-01T00:00:00Z","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01
	}]`}
	account := testOpenAIRouteControllerAccount(1, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	provider := OpenAIRouteHealthStoreKeyForProvider(key)
	health := &openAIRouteHealthStoreStub{omitBatchKeys: map[string]struct{}{provider.Fingerprint(): {}}}
	controller := NewOpenAIRouteController(reader, health, &openAIRouteBudgetSnapshotStoreStub{}, &openAIRouteObservationStoreStub{})

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7, Model: "gpt-5.6-sol", RequestClass: OpenAIRouteRequestClassText,
		Candidates: []OpenAIRouteShadowCandidate{{
			Account: account, Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE),
		}},
	})
	require.ErrorContains(t, err, "missing OpenAI provider health state")
	require.False(t, decision.Evaluated)
	require.Equal(t, 1, health.batchCalls)
}

func TestDecodeOpenAIRoutePolicies_AcceptsSinglePolicyDocument(t *testing.T) {
	policies, err := decodeOpenAIRoutePolicies(`{"group_id":8,"model":"gpt-5.6-sol","enabled":true,"mode":"shadow","policy_version":2,"activation_id":"test-activation-2","shadow_started_at":"2026-08-01T00:00:00Z"}`)
	require.NoError(t, err)
	require.Len(t, policies, 1)
	require.Equal(t, int64(8), policies[0].GroupID)
	require.Equal(t, "gpt-5.6-sol", policies[0].Model)
}

func testOpenAIRouteControllerAccount(id int64, rate float64) *Account {
	return &Account{
		ID:             id,
		Platform:       PlatformOpenAI,
		Type:           AccountTypeAPIKey,
		RateMultiplier: &rate,
		Extra: map[string]any{
			OpenAIRouteFailureDomainExtraKey: "provider-test",
		},
	}
}

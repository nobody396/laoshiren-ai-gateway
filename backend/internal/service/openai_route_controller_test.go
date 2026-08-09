package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
	states map[OpenAIRouteHealthStoreKey]OpenAIRouteHealthState
}

func (s *openAIRouteHealthStoreStub) Get(_ context.Context, key OpenAIRouteHealthStoreKey) (OpenAIRouteHealthState, error) {
	if state, ok := s.states[key]; ok {
		return state, nil
	}
	return OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}, nil
}

func (s *openAIRouteHealthStoreStub) ApplyEvent(_ context.Context, _ OpenAIRouteHealthStoreKey, _ OpenAIRouteHealthEvent, _ OpenAIRoutePolicy) (OpenAIRouteHealthState, error) {
	return OpenAIRouteHealthState{}, nil
}

func (s *openAIRouteHealthStoreStub) AcquireHalfOpenPermit(context.Context, OpenAIRouteHealthStoreKey, string) (bool, error) {
	return true, nil
}

func (s *openAIRouteHealthStoreStub) ReleaseHalfOpenPermit(context.Context, OpenAIRouteHealthStoreKey, string) error {
	return nil
}

type openAIRouteBudgetSnapshotStoreStub struct {
	windows      []OpenAIRouteBudgetWindowConfig
	reserveCalls int
	settleCalls  int
}

func (s *openAIRouteBudgetSnapshotStoreStub) GetLedgers(_ context.Context, windows []OpenAIRouteBudgetWindowConfig) ([]OpenAIRouteBudgetLedger, error) {
	s.windows = append([]OpenAIRouteBudgetWindowConfig(nil), windows...)
	ledgers := make([]OpenAIRouteBudgetLedger, len(windows))
	for i, window := range windows {
		ledgers[i] = window.EmptyLedger()
	}
	return ledgers, nil
}

func (s *openAIRouteBudgetSnapshotStoreStub) Reserve(_ context.Context, req OpenAIRouteBudgetStoreReserveRequest) (OpenAIRouteBudgetStoreReservation, error) {
	s.reserveCalls++
	return OpenAIRouteBudgetStoreReservation{
		ReservationID:  req.ReservationID,
		RouteKey:       req.RouteKey,
		Allowed:        true,
		RejectedWindow: -1,
	}, nil
}

func (s *openAIRouteBudgetSnapshotStoreStub) Settle(_ context.Context, settlement OpenAIRouteBudgetStoreSettlement) error {
	s.settleCalls++
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
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{})
	req := OpenAIRouteShadowRequest{
		GroupID: 7,
		Model:   "gpt-5.6-sol",
		Now:     time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC),
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
    {"group_id":7,"model":"gpt-*","enabled":true,"mode":"shadow","policy_version":1,"target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01},
    {"group_id":7,"model":"gpt-5.6-sol","enabled":true,"mode":"shadow","policy_version":9,"target_avg_multiplier":0.155,"hard_avg_multiplier":0.18,"estimated_base_cost_usd":0.01}
  ]
}`}
	budget := &openAIRouteBudgetSnapshotStoreStub{}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, budget)
	now := time.Date(2026, 8, 8, 12, 3, 0, 0, time.UTC)

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7,
		Model:   "gpt-5.6-sol",
		Seed:    42,
		Now:     now,
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
	require.Equal(t, int64(1), decision.SelectedAccountID, "the 0.20 route has no earned credit and must be budget-filtered")
	require.InDelta(t, 0.15, decision.SelectedRate, 1e-12)
	require.NotEmpty(t, decision.DecisionID)
	require.GreaterOrEqual(t, decision.EvaluationDurationMicros, int64(0))
	require.NotNil(t, decision.Audit)
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
}

func TestOpenAIRouteController_EnforceModeIsHardDisabled(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{"group_id":7,"model":"*","enabled":true,"mode":"enforce","policy_version":1}]`}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{})

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7,
		Model:   "gpt-5.6-sol",
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

func TestOpenAIRouteController_BudgetFailureDoesNotClaimASelectedCandidate(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `{
  "group_id":7,
  "model":"gpt-5.6-sol",
  "enabled":true,
  "mode":"shadow",
  "policy_version":10,
  "target_avg_multiplier":0.10,
  "hard_avg_multiplier":0.10,
  "estimated_base_cost_usd":0.01
}`}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{})

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{
		GroupID: 7,
		Model:   "gpt-5.6-sol",
		Seed:    42,
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

func TestDecodeOpenAIRoutePolicies_AcceptsSinglePolicyDocument(t *testing.T) {
	policies, err := decodeOpenAIRoutePolicies(`{"group_id":8,"model":"gpt-5.6-sol","enabled":true,"mode":"shadow","policy_version":2}`)
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
			openAIRouteFailureDomainExtraKey: "provider-test",
		},
	}
}

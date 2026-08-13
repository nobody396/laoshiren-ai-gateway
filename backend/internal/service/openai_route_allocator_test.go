package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func testOpenAIRouteCandidate(id int64, provider string, rate float64) OpenAIRouteCandidate {
	return OpenAIRouteCandidate{
		Key: OpenAIRouteKey{
			GroupID:       7,
			AccountID:     id,
			Model:         "gpt-5.6-sol",
			RequestClass:  OpenAIRouteRequestClassText,
			EndpointHash:  "endpoint",
			Transport:     "sse",
			FailureDomain: provider,
		},
		RateMultiplier:       rate,
		Priority:             1,
		CircuitState:         OpenAIRouteCircuitHealthy,
		ProviderCircuitState: OpenAIRouteCircuitHealthy,
		HasReliabilitySample: true,
		SuccessLowerBound:    0.99,
		P90TTFTMilliseconds:  1000,
		LoadRatio:            0.1,
		CurrentAccountShare:  0.1,
		CurrentProviderShare: 0.1,
	}
}

func testOpenAIRouteAllocationRequest(candidates ...OpenAIRouteCandidate) OpenAIRouteAllocationRequest {
	policy := testOpenAIRouteBudgetPolicy()
	policy.MaxAccountShare = 0.80
	policy.MaxProviderShare = 0.90
	ledger, err := NewOpenAIRouteBudgetLedger(policy, 1, 0, 0)
	if err != nil {
		panic(err)
	}
	return OpenAIRouteAllocationRequest{
		Policy:               policy,
		Budget:               ledger,
		Candidates:           candidates,
		EstimatedBaseCostUSD: 1,
		Seed:                 42,
	}
}

func TestBuildOpenAIRouteAllocationPlan_UsesDynamicMultipliersWithoutHardcodedAccounts(t *testing.T) {
	request := testOpenAIRouteAllocationRequest(
		testOpenAIRouteCandidate(24, "pomo", 0.15),
		testOpenAIRouteCandidate(28, "anyroute", 0.15),
		testOpenAIRouteCandidate(31, "future-provider", 0.20),
		testOpenAIRouteCandidate(44, "another-future-provider", 0.25),
	)
	plan, err := BuildOpenAIRouteAllocationPlan(request)
	require.NoError(t, err)
	require.Len(t, plan.Ranked, 4)
	require.InDelta(t, 0.15, plan.MinHealthyMultiplier, 1e-12)

	weights := make(map[int64]OpenAIRouteWeightedCandidate)
	for _, item := range plan.Ranked {
		weights[item.Candidate.Key.AccountID] = item
	}
	require.InDelta(t, 1, weights[24].PriceFactor, 1e-12)
	require.InDelta(t, 0.5625, weights[31].PriceFactor, 1e-12)
	require.InDelta(t, 0.36, weights[44].PriceFactor, 1e-12)
}

func TestBuildOpenAIRouteAllocationPlanUsesRouteSpecificSettledCost(t *testing.T) {
	cheap := testOpenAIRouteCandidate(1, "provider-a", 0.20)
	cheap.EstimatedBaseCostUSD = 0.01
	expensive := testOpenAIRouteCandidate(2, "provider-b", 0.20)
	expensive.EstimatedBaseCostUSD = 0.10
	request := testOpenAIRouteAllocationRequest(cheap, expensive)
	request.Budget.CreditUSD = 0.004
	request.Policy.EmergencyDebtLimitUSD = 0

	plan, err := BuildOpenAIRouteAllocationPlan(request)

	require.NoError(t, err)
	require.Len(t, plan.Ranked, 1)
	require.Equal(t, int64(1), plan.Selected.Candidate.Key.AccountID)
	require.Contains(t, plan.Excluded, OpenAIRouteExclusion{AccountID: 2, Reason: OpenAIRouteExcludedCost})
	require.InDelta(t, 1, plan.Selected.PriceFactor, 1e-12)
}

func TestBuildOpenAIRouteAllocationPlanPricesEqualMultipliersByPredictedAccountCost(t *testing.T) {
	cheap := testOpenAIRouteCandidate(1, "provider-a", 0.20)
	cheap.EstimatedBaseCostUSD = 0.05
	expensive := testOpenAIRouteCandidate(2, "provider-b", 0.20)
	expensive.EstimatedBaseCostUSD = 0.20
	request := testOpenAIRouteAllocationRequest(cheap, expensive)
	request.Budget.CreditUSD = 1

	plan, err := BuildOpenAIRouteAllocationPlan(request)

	require.NoError(t, err)
	byID := make(map[int64]OpenAIRouteWeightedCandidate)
	for _, candidate := range plan.Ranked {
		byID[candidate.Candidate.Key.AccountID] = candidate
	}
	require.InDelta(t, 1, byID[1].PriceFactor, 1e-12)
	require.InDelta(t, 0.0625, byID[2].PriceFactor, 1e-12)
	require.Greater(t, byID[1].Weight, byID[2].Weight)
}

func TestBuildOpenAIRouteAllocationPlan_ExcludesUnaffordableExpensiveRoutes(t *testing.T) {
	cheap := testOpenAIRouteCandidate(1, "cheap", 0.15)
	expensive := testOpenAIRouteCandidate(2, "expensive", 0.25)
	request := testOpenAIRouteAllocationRequest(cheap, expensive)
	request.Budget.CreditUSD = 0
	request.Policy.EmergencyDebtLimitUSD = 0

	plan, err := BuildOpenAIRouteAllocationPlan(request)
	require.NoError(t, err)
	require.Len(t, plan.Ranked, 1)
	require.Equal(t, int64(1), plan.Selected.Candidate.Key.AccountID)
	require.Contains(t, plan.Excluded, OpenAIRouteExclusion{AccountID: 2, Reason: OpenAIRouteExcludedCost})

	request.Candidates = []OpenAIRouteCandidate{expensive}
	plan, err = BuildOpenAIRouteAllocationPlan(request)
	require.ErrorIs(t, err, ErrOpenAIRouteBudgetExhausted)
	require.Empty(t, plan.Ranked)
}

func TestBuildOpenAIRouteAllocationPlan_EveryBudgetWindowMustAdmitRoute(t *testing.T) {
	cheap := testOpenAIRouteCandidate(1, "cheap", 0.15)
	expensive := testOpenAIRouteCandidate(2, "expensive", 0.20)
	request := testOpenAIRouteAllocationRequest(cheap, expensive)
	shortWindow := request.Budget
	shortWindow.CreditUSD = 1
	longWindow := request.Budget
	longWindow.CreditUSD = 0
	request.Budgets = []OpenAIRouteBudgetLedger{shortWindow, longWindow}
	request.Policy.EmergencyDebtLimitUSD = 0

	plan, err := BuildOpenAIRouteAllocationPlan(request)
	require.NoError(t, err)
	require.Len(t, plan.Ranked, 1)
	require.Equal(t, int64(1), plan.Selected.Candidate.Key.AccountID)
	require.Contains(t, plan.Excluded, OpenAIRouteExclusion{AccountID: 2, Reason: OpenAIRouteExcludedCost})

	request.Candidates = []OpenAIRouteCandidate{expensive}
	_, err = BuildOpenAIRouteAllocationPlan(request)
	require.ErrorIs(t, err, ErrOpenAIRouteBudgetExhausted)
}

func TestBuildOpenAIRouteAllocationPlan_ProviderFailureAndShareCapSkipCorrelatedAccounts(t *testing.T) {
	pomo1 := testOpenAIRouteCandidate(1, "pomo", 0.15)
	pomo2 := testOpenAIRouteCandidate(2, "pomo", 0.20)
	anyroute := testOpenAIRouteCandidate(3, "anyroute", 0.15)
	pomo1.ProviderCircuitState = OpenAIRouteCircuitOpen
	pomo2.ProviderCircuitState = OpenAIRouteCircuitOpen

	plan, err := BuildOpenAIRouteAllocationPlan(testOpenAIRouteAllocationRequest(pomo1, pomo2, anyroute))
	require.NoError(t, err)
	require.Len(t, plan.Ranked, 1)
	require.Equal(t, int64(3), plan.Selected.Candidate.Key.AccountID)

	pomo1.ProviderCircuitState = OpenAIRouteCircuitHealthy
	pomo2.ProviderCircuitState = OpenAIRouteCircuitHealthy
	pomo1.CurrentProviderShare = 0.95
	pomo2.CurrentProviderShare = 0.95
	anyroute.CurrentProviderShare = 0.05
	plan, err = BuildOpenAIRouteAllocationPlan(testOpenAIRouteAllocationRequest(pomo1, pomo2, anyroute))
	require.NoError(t, err)
	require.Len(t, plan.Ranked, 1)
	require.Equal(t, int64(3), plan.Selected.Candidate.Key.AccountID)
}

func TestBuildOpenAIRouteAllocationPlan_SoftShareCapRelaxesWhenOnlyHealthyRouteRemains(t *testing.T) {
	sole := testOpenAIRouteCandidate(1, "only-provider", 0.15)
	sole.CurrentAccountShare = 1
	sole.CurrentProviderShare = 1
	request := testOpenAIRouteAllocationRequest(sole)

	plan, err := BuildOpenAIRouteAllocationPlan(request)
	require.NoError(t, err)
	require.Equal(t, int64(1), plan.Selected.Candidate.Key.AccountID)

	request.HardShareCaps = true
	_, err = BuildOpenAIRouteAllocationPlan(request)
	require.ErrorIs(t, err, ErrOpenAIRouteNoCandidate)
}

func TestBuildOpenAIRouteAllocationPlan_EnforcesHalfOpenAndRecoveryCaps(t *testing.T) {
	halfOpen := testOpenAIRouteCandidate(1, "p1", 0.15)
	halfOpen.CircuitState = OpenAIRouteCircuitHalfOpen
	halfOpen.HalfOpenPermit = false
	healthy := testOpenAIRouteCandidate(2, "p2", 0.15)

	plan, err := BuildOpenAIRouteAllocationPlan(testOpenAIRouteAllocationRequest(halfOpen, healthy))
	require.NoError(t, err)
	require.Equal(t, int64(2), plan.Selected.Candidate.Key.AccountID)
	require.Contains(t, plan.Excluded, OpenAIRouteExclusion{AccountID: 1, Reason: OpenAIRouteExcludedHalfOpen})

	recovering := testOpenAIRouteCandidate(3, "p3", 0.15)
	recovering.CircuitState = OpenAIRouteCircuitRecovering
	recovering.RecoveryStep = 0
	recovering.CurrentAccountShare = 0.01
	plan, err = BuildOpenAIRouteAllocationPlan(testOpenAIRouteAllocationRequest(recovering, healthy))
	require.NoError(t, err)
	require.Equal(t, int64(2), plan.Selected.Candidate.Key.AccountID)
	require.Contains(t, plan.Excluded, OpenAIRouteExclusion{AccountID: 3, Reason: OpenAIRouteExcludedRecoveryCap})
}

func TestBuildOpenAIRouteAllocationPlan_ReliabilityCanBeatSmallPriceDifference(t *testing.T) {
	cheapUnstable := testOpenAIRouteCandidate(1, "cheap", 0.15)
	cheapUnstable.SuccessLowerBound = 0.20
	fastReliable := testOpenAIRouteCandidate(2, "reliable", 0.20)
	fastReliable.SuccessLowerBound = 0.99
	request := testOpenAIRouteAllocationRequest(cheapUnstable, fastReliable)

	plan, err := BuildOpenAIRouteAllocationPlan(request)
	require.NoError(t, err)
	weights := map[int64]float64{}
	for _, item := range plan.Ranked {
		weights[item.Candidate.Key.AccountID] = item.Weight
	}
	require.Greater(t, weights[2], weights[1])
}

func TestBuildOpenAIRouteAllocationPlan_PenalizesTailLatencyAndPartialStreams(t *testing.T) {
	unstable := testOpenAIRouteCandidate(1, "p1", 0.15)
	unstable.P90TTFTMilliseconds = 500
	unstable.P95CompletionLatencyMilliseconds = 30_000
	unstable.PartialStreamRate = 0.20
	stable := testOpenAIRouteCandidate(2, "p2", 0.15)
	stable.P90TTFTMilliseconds = 500
	stable.P95CompletionLatencyMilliseconds = 5_000
	stable.PartialStreamRate = 0

	plan, err := BuildOpenAIRouteAllocationPlan(testOpenAIRouteAllocationRequest(unstable, stable))
	require.NoError(t, err)
	byID := make(map[int64]OpenAIRouteWeightedCandidate)
	for _, candidate := range plan.Ranked {
		byID[candidate.Candidate.Key.AccountID] = candidate
	}
	require.Less(t, byID[1].TailLatencyFactor, byID[2].TailLatencyFactor)
	require.Less(t, byID[1].StreamIntegrityFactor, byID[2].StreamIntegrityFactor)
	require.Less(t, byID[1].Weight, byID[2].Weight)
}

func TestBuildOpenAIRouteAllocationPlan_PriorityIsPriorNotAbsoluteBucket(t *testing.T) {
	priorityOneSlow := testOpenAIRouteCandidate(1, "p1", 0.15)
	priorityOneSlow.Priority = 1
	priorityOneSlow.P90TTFTMilliseconds = 5000
	priorityThreeFast := testOpenAIRouteCandidate(2, "p2", 0.15)
	priorityThreeFast.Priority = 3
	priorityThreeFast.P90TTFTMilliseconds = 500

	plan, err := BuildOpenAIRouteAllocationPlan(testOpenAIRouteAllocationRequest(priorityOneSlow, priorityThreeFast))
	require.NoError(t, err)
	weights := map[int64]float64{}
	for _, item := range plan.Ranked {
		weights[item.Candidate.Key.AccountID] = item.Weight
	}
	require.Greater(t, weights[2], weights[1])
}

func TestBuildOpenAIRouteAllocationPlan_IsDeterministicForSameSeed(t *testing.T) {
	request := testOpenAIRouteAllocationRequest(
		testOpenAIRouteCandidate(1, "p1", 0.15),
		testOpenAIRouteCandidate(2, "p2", 0.15),
		testOpenAIRouteCandidate(3, "p3", 0.15),
	)
	first, err := BuildOpenAIRouteAllocationPlan(request)
	require.NoError(t, err)
	second, err := BuildOpenAIRouteAllocationPlan(request)
	require.NoError(t, err)
	require.Equal(t, first.Ranked, second.Ranked)
}

func TestBuildOpenAIRouteAllocationPlan_DistinguishesHealthAndBudgetExhaustion(t *testing.T) {
	open := testOpenAIRouteCandidate(1, "p1", 0.15)
	open.CircuitState = OpenAIRouteCircuitOpen
	_, err := BuildOpenAIRouteAllocationPlan(testOpenAIRouteAllocationRequest(open))
	require.True(t, errors.Is(err, ErrOpenAIRouteNoCandidate))

	expensive := testOpenAIRouteCandidate(2, "p2", 0.25)
	request := testOpenAIRouteAllocationRequest(expensive)
	request.Budget.CreditUSD = 0
	request.Policy.EmergencyDebtLimitUSD = 0
	_, err = BuildOpenAIRouteAllocationPlan(request)
	require.True(t, errors.Is(err, ErrOpenAIRouteBudgetExhausted))
}

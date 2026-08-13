package service

import (
	"fmt"
	"math"
	"sort"
)

type openAIRouteRNG struct {
	state uint64
}

func newOpenAIRouteRNG(seed uint64) openAIRouteRNG {
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15
	}
	return openAIRouteRNG{state: seed}
}

func (r *openAIRouteRNG) nextFloat64() float64 {
	x := r.state
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	r.state = x
	value := x * 2685821657736338717
	return float64(value>>11) / (1 << 53)
}

type openAIRouteFeasibleCandidate struct {
	candidate               OpenAIRouteCandidate
	preview                 OpenAIRouteBudgetPreview
	estimatedBaseCostUSD    float64
	estimatedAccountCostUSD float64
}

func BuildOpenAIRouteAllocationPlan(req OpenAIRouteAllocationRequest) (OpenAIRouteAllocationPlan, error) {
	plan := OpenAIRouteAllocationPlan{}
	policy, err := NormalizeOpenAIRoutePolicy(req.Policy)
	if err != nil {
		return plan, err
	}
	if !isFiniteNonNegative(req.EstimatedBaseCostUSD) {
		return plan, ErrOpenAIRouteInvalidCost
	}

	budgets := append([]OpenAIRouteBudgetLedger(nil), req.Budgets...)
	multiWindowBudget := len(budgets) > 0
	if !multiWindowBudget {
		budgets = []OpenAIRouteBudgetLedger{req.Budget}
	}
	for i := range budgets {
		if !multiWindowBudget {
			budgets[i].TargetAverageMultiplier = policy.TargetAverageMultiplier
			budgets[i].HardAverageMultiplier = policy.HardAverageMultiplier
			budgets[i].EmergencyDebtLimitUSD = policy.EmergencyDebtLimitUSD
			budgets[i].MaxCreditUSD = policy.MaxCreditUSD
		}
		budgets[i].clampCredit()
		if err := budgets[i].Validate(); err != nil {
			return plan, err
		}
	}

	feasible := make([]openAIRouteFeasibleCandidate, 0, len(req.Candidates))
	healthEligibleCostRejected := 0
	for _, candidate := range req.Candidates {
		if !validOpenAIRouteCandidate(candidate) {
			plan.Excluded = append(plan.Excluded, OpenAIRouteExclusion{AccountID: candidate.Key.AccountID, Reason: OpenAIRouteExcludedInvalid})
			continue
		}
		candidate.CircuitState = normalizeOpenAIRouteCircuitState(candidate.CircuitState)
		candidate.ProviderCircuitState = normalizeOpenAIRouteProviderCircuitState(candidate.ProviderCircuitState)
		if candidate.CircuitState == OpenAIRouteCircuitOpen || candidate.CircuitState == OpenAIRouteCircuitDisabled {
			plan.Excluded = append(plan.Excluded, OpenAIRouteExclusion{AccountID: candidate.Key.AccountID, Reason: OpenAIRouteExcludedCircuitOpen})
			continue
		}
		if candidate.ProviderCircuitState == OpenAIRouteCircuitOpen || candidate.ProviderCircuitState == OpenAIRouteCircuitDisabled {
			plan.Excluded = append(plan.Excluded, OpenAIRouteExclusion{AccountID: candidate.Key.AccountID, Reason: OpenAIRouteExcludedProviderOpen})
			continue
		}
		if (candidate.CircuitState == OpenAIRouteCircuitHalfOpen || candidate.ProviderCircuitState == OpenAIRouteCircuitHalfOpen) && !candidate.HalfOpenPermit {
			plan.Excluded = append(plan.Excluded, OpenAIRouteExclusion{AccountID: candidate.Key.AccountID, Reason: OpenAIRouteExcludedHalfOpen})
			continue
		}
		state := OpenAIRouteHealthState{State: candidate.CircuitState, RecoveryStep: candidate.RecoveryStep}
		shareCap := state.TrafficShareCap(policy)
		if shareCap < 1 && candidate.CurrentAccountShare+1e-12 >= shareCap {
			plan.Excluded = append(plan.Excluded, OpenAIRouteExclusion{AccountID: candidate.Key.AccountID, Reason: OpenAIRouteExcludedRecoveryCap})
			continue
		}

		estimatedBaseCostUSD := openAIRouteCandidateEstimatedBaseCost(candidate, req.EstimatedBaseCostUSD)
		preview := OpenAIRouteBudgetPreview{Allowed: true}
		for _, budget := range budgets {
			windowPreview, previewErr := budget.Preview(candidate.RateMultiplier, estimatedBaseCostUSD)
			if previewErr != nil {
				return plan, previewErr
			}
			if !windowPreview.Allowed {
				preview.Allowed = false
				preview.Reason = windowPreview.Reason
				break
			}
			preview.Emergency = preview.Emergency || windowPreview.Emergency
			if windowPreview.PredictedExtraCostUSD > preview.PredictedExtraCostUSD {
				preview.PredictedExtraCostUSD = windowPreview.PredictedExtraCostUSD
			}
		}
		if !preview.Allowed {
			healthEligibleCostRejected++
			plan.Excluded = append(plan.Excluded, OpenAIRouteExclusion{AccountID: candidate.Key.AccountID, Reason: OpenAIRouteExcludedCost})
			continue
		}
		feasible = append(feasible, openAIRouteFeasibleCandidate{
			candidate:               candidate,
			preview:                 preview,
			estimatedBaseCostUSD:    estimatedBaseCostUSD,
			estimatedAccountCostUSD: estimatedBaseCostUSD * candidate.RateMultiplier,
		})
	}

	if len(feasible) == 0 {
		if healthEligibleCostRejected > 0 {
			return plan, ErrOpenAIRouteBudgetExhausted
		}
		return plan, ErrOpenAIRouteNoCandidate
	}

	feasible = applyOpenAIRouteAccountShareCap(feasible, policy.MaxAccountShare, req.HardShareCaps, &plan)
	feasible = applyOpenAIRouteProviderShareCap(feasible, policy.MaxProviderShare, req.HardShareCaps, &plan)
	if len(feasible) == 0 {
		return plan, ErrOpenAIRouteNoCandidate
	}

	minRate := feasible[0].candidate.RateMultiplier
	minAccountCost := feasible[0].estimatedAccountCostUSD
	minPriority := feasible[0].candidate.Priority
	minTTFT := 0.0
	minCompletionLatency := 0.0
	for _, item := range feasible {
		candidate := item.candidate
		if candidate.RateMultiplier < minRate {
			minRate = candidate.RateMultiplier
		}
		if item.estimatedAccountCostUSD < minAccountCost {
			minAccountCost = item.estimatedAccountCostUSD
		}
		if candidate.Priority < minPriority {
			minPriority = candidate.Priority
		}
		if candidate.P90TTFTMilliseconds > 0 && (minTTFT == 0 || candidate.P90TTFTMilliseconds < minTTFT) {
			minTTFT = candidate.P90TTFTMilliseconds
		}
		if candidate.P95CompletionLatencyMilliseconds > 0 && (minCompletionLatency == 0 || candidate.P95CompletionLatencyMilliseconds < minCompletionLatency) {
			minCompletionLatency = candidate.P95CompletionLatencyMilliseconds
		}
	}
	plan.MinHealthyMultiplier = minRate

	weighted := make([]OpenAIRouteWeightedCandidate, 0, len(feasible))
	for _, item := range feasible {
		candidate := item.candidate
		healthFactor := openAIRouteHealthFactor(candidate, policy)
		latencyFactor := openAIRouteLatencyFactor(candidate.P90TTFTMilliseconds, minTTFT, policy.LatencyBeta)
		tailLatencyFactor := openAIRouteLatencyFactor(candidate.P95CompletionLatencyMilliseconds, minCompletionLatency, policy.LatencyBeta*0.35)
		streamIntegrityFactor := openAIRouteStreamIntegrityFactor(candidate.PartialStreamRate)
		headroomFactor := openAIRouteHeadroomFactor(candidate.LoadRatio, candidate.WaitingCount)
		priceFactor := openAIRoutePriceFactor(item.estimatedAccountCostUSD, minAccountCost, policy.PriceExponent)
		priorityFactor := 1 / (1 + policy.PriorityPenalty*float64(maxOpenAIRouteInt(candidate.Priority-minPriority, 0)))
		explorationBoost := candidate.ExplorationBoost
		if explorationBoost <= 0 || math.IsNaN(explorationBoost) || math.IsInf(explorationBoost, 0) {
			explorationBoost = 1
		}
		explorationBoost = math.Max(0.1, math.Min(5, explorationBoost))
		weight := healthFactor * latencyFactor * tailLatencyFactor * streamIntegrityFactor * headroomFactor * priceFactor * priorityFactor * explorationBoost
		if weight <= 0 || math.IsNaN(weight) || math.IsInf(weight, 0) {
			weight = 1e-9
		}
		weighted = append(weighted, OpenAIRouteWeightedCandidate{
			Candidate:             candidate,
			Weight:                weight,
			HealthFactor:          healthFactor,
			LatencyFactor:         latencyFactor,
			TailLatencyFactor:     tailLatencyFactor,
			StreamIntegrityFactor: streamIntegrityFactor,
			HeadroomFactor:        headroomFactor,
			PriceFactor:           priceFactor,
			PriorityFactor:        priorityFactor,
			PredictedExtraCost:    item.preview.PredictedExtraCostUSD,
			EmergencyBudgetUsed:   item.preview.Emergency,
		})
	}

	plan.Ranked = openAIRouteWeightedOrder(weighted, req.Seed)
	if len(plan.Ranked) == 0 {
		return plan, fmt.Errorf("%w: empty weighted order", ErrOpenAIRouteNoCandidate)
	}
	plan.Selected = plan.Ranked[0]
	plan.Emergency = plan.Selected.EmergencyBudgetUsed
	return plan, nil
}

func validOpenAIRouteCandidate(candidate OpenAIRouteCandidate) bool {
	if !candidate.Key.Valid() || !isFiniteNonNegative(candidate.RateMultiplier) {
		return false
	}
	if math.IsNaN(candidate.LoadRatio) || math.IsInf(candidate.LoadRatio, 0) {
		return false
	}
	if math.IsNaN(candidate.SuccessLowerBound) || math.IsInf(candidate.SuccessLowerBound, 0) {
		return false
	}
	if math.IsNaN(candidate.P90TTFTMilliseconds) || math.IsInf(candidate.P90TTFTMilliseconds, 0) || candidate.P90TTFTMilliseconds < 0 {
		return false
	}
	if math.IsNaN(candidate.P95CompletionLatencyMilliseconds) || math.IsInf(candidate.P95CompletionLatencyMilliseconds, 0) || candidate.P95CompletionLatencyMilliseconds < 0 {
		return false
	}
	if math.IsNaN(candidate.PartialStreamRate) || math.IsInf(candidate.PartialStreamRate, 0) || candidate.PartialStreamRate < 0 || candidate.PartialStreamRate > 1 {
		return false
	}
	if !isFiniteNonNegative(candidate.EstimatedBaseCostUSD) || !isFiniteNonNegative(candidate.ObservedMeanCostUSD) {
		return false
	}
	return true
}

func openAIRouteCandidateEstimatedBaseCost(candidate OpenAIRouteCandidate, fallback float64) float64 {
	if candidate.EstimatedBaseCostUSD > 0 {
		return candidate.EstimatedBaseCostUSD
	}
	return fallback
}

func normalizeOpenAIRouteCircuitState(state OpenAIRouteCircuitState) OpenAIRouteCircuitState {
	if state == "" {
		return OpenAIRouteCircuitWarmup
	}
	return state
}

func normalizeOpenAIRouteProviderCircuitState(state OpenAIRouteCircuitState) OpenAIRouteCircuitState {
	if state == "" {
		return OpenAIRouteCircuitHealthy
	}
	return state
}

func applyOpenAIRouteAccountShareCap(candidates []openAIRouteFeasibleCandidate, maxShare float64, hard bool, plan *OpenAIRouteAllocationPlan) []openAIRouteFeasibleCandidate {
	underCap := 0
	for _, item := range candidates {
		if item.candidate.CurrentAccountShare < maxShare {
			underCap++
		}
	}
	if underCap == 0 && !hard {
		return candidates
	}
	out := make([]openAIRouteFeasibleCandidate, 0, len(candidates))
	for _, item := range candidates {
		if item.candidate.CurrentAccountShare >= maxShare {
			plan.Excluded = append(plan.Excluded, OpenAIRouteExclusion{AccountID: item.candidate.Key.AccountID, Reason: OpenAIRouteExcludedAccountCap})
			continue
		}
		out = append(out, item)
	}
	return out
}

func applyOpenAIRouteProviderShareCap(candidates []openAIRouteFeasibleCandidate, maxShare float64, hard bool, plan *OpenAIRouteAllocationPlan) []openAIRouteFeasibleCandidate {
	underCapProviders := make(map[string]struct{})
	providers := make(map[string]struct{})
	for _, item := range candidates {
		provider := openAIRouteProviderKey(item.candidate)
		providers[provider] = struct{}{}
		if item.candidate.CurrentProviderShare < maxShare {
			underCapProviders[provider] = struct{}{}
		}
	}
	if len(underCapProviders) == 0 && !hard {
		return candidates
	}
	// If every remaining candidate belongs to one provider, the provider cap is
	// a normal-mode soft cap. Relaxing it is necessary to preserve availability.
	if len(providers) == 1 && !hard {
		return candidates
	}
	out := make([]openAIRouteFeasibleCandidate, 0, len(candidates))
	for _, item := range candidates {
		provider := openAIRouteProviderKey(item.candidate)
		if _, ok := underCapProviders[provider]; !ok {
			plan.Excluded = append(plan.Excluded, OpenAIRouteExclusion{AccountID: item.candidate.Key.AccountID, Reason: OpenAIRouteExcludedProviderCap})
			continue
		}
		out = append(out, item)
	}
	return out
}

func openAIRouteProviderKey(candidate OpenAIRouteCandidate) string {
	if candidate.Key.FailureDomain != "" {
		return candidate.Key.FailureDomain
	}
	return fmt.Sprintf("account:%d", candidate.Key.AccountID)
}

func openAIRouteHealthFactor(candidate OpenAIRouteCandidate, policy OpenAIRoutePolicy) float64 {
	factor := 0.5
	if candidate.HasReliabilitySample {
		factor = math.Max(policy.MinHealthFactor, math.Min(1, candidate.SuccessLowerBound))
	}
	switch candidate.CircuitState {
	case OpenAIRouteCircuitDegraded:
		factor *= policy.DegradedShare
	case OpenAIRouteCircuitWarmup, OpenAIRouteCircuitHalfOpen:
		factor *= 0.5
	case OpenAIRouteCircuitRecovering:
		factor *= 0.75
	}
	if candidate.ProviderCircuitState == OpenAIRouteCircuitDegraded {
		factor *= policy.DegradedShare
	}
	return math.Max(policy.MinHealthFactor, math.Min(1, factor))
}

func openAIRouteLatencyFactor(ttft, minTTFT, beta float64) float64 {
	if ttft <= 0 || minTTFT <= 0 {
		return 0.7
	}
	ratioDelta := ttft/minTTFT - 1
	factor := math.Exp(-beta * math.Max(0, ratioDelta))
	return math.Max(0.05, math.Min(1, factor))
}

func openAIRouteStreamIntegrityFactor(partialStreamRate float64) float64 {
	partialStreamRate = math.Max(0, math.Min(1, partialStreamRate))
	integrity := 1 - partialStreamRate
	return math.Max(0.05, integrity*integrity)
}

func openAIRouteHeadroomFactor(loadRatio float64, waiting int) float64 {
	loadRatio = math.Max(0, math.Min(1, loadRatio))
	if waiting < 0 {
		waiting = 0
	}
	headroom := math.Max(0.05, 1-loadRatio)
	queue := 1 / (1 + float64(waiting))
	return math.Max(0.01, headroom*queue)
}

func openAIRoutePriceFactor(accountCost, minAccountCost, exponent float64) float64 {
	if minAccountCost == 0 {
		if accountCost == 0 {
			return 1
		}
		return 0.01
	}
	if accountCost <= 0 {
		return 1
	}
	factor := math.Pow(minAccountCost/accountCost, exponent)
	return math.Max(0.01, math.Min(1, factor))
}

func openAIRouteWeightedOrder(candidates []OpenAIRouteWeightedCandidate, seed uint64) []OpenAIRouteWeightedCandidate {
	if len(candidates) <= 1 {
		return append([]OpenAIRouteWeightedCandidate(nil), candidates...)
	}
	if seed == 0 {
		// Stable fallback for tests and shadow decisions. Production callers
		// should provide a request/session-derived seed.
		seed = 0x9e3779b97f4a7c15
	}
	rng := newOpenAIRouteRNG(seed)
	pool := append([]OpenAIRouteWeightedCandidate(nil), candidates...)
	// Stable input order prevents account snapshot order from becoming hidden
	// entropy when all request dimensions are otherwise equal.
	sort.SliceStable(pool, func(i, j int) bool {
		return pool[i].Candidate.Key.AccountID < pool[j].Candidate.Key.AccountID
	})
	ordered := make([]OpenAIRouteWeightedCandidate, 0, len(pool))
	for len(pool) > 0 {
		total := 0.0
		for _, item := range pool {
			total += item.Weight
		}
		selected := 0
		if total > 0 {
			target := rng.nextFloat64() * total
			cumulative := 0.0
			for idx, item := range pool {
				cumulative += item.Weight
				if target <= cumulative {
					selected = idx
					break
				}
			}
		}
		ordered = append(ordered, pool[selected])
		pool = append(pool[:selected], pool[selected+1:]...)
	}
	return ordered
}

func maxOpenAIRouteInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

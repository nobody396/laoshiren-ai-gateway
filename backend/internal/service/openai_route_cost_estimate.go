package service

import "math"

const (
	// A configured policy estimate remains the prior until enough settled text
	// requests exist for the exact route. This prevents a handful of unusually
	// short prompts from collapsing the cost guard.
	openAIRouteCostEstimateMinimumSamples = uint64(20)
	openAIRouteCostEstimatePriorSamples   = 20.0
	openAIRouteCostEstimateMinimumRatio   = 0.25
	openAIRouteCostEstimateMaximumRatio   = 4.0
)

const (
	OpenAIRouteCostEstimateSourcePolicy      = "policy_configured"
	OpenAIRouteCostEstimateSourceSettledText = "shared_settled_text"
)

// OpenAIRouteCostEstimate is a conservative, auditable prediction for one
// exact route. Only settled text supplier cost may replace the configured
// prior. Image/video cost stays on the policy value until raw upstream
// deduction reconciliation is authoritative.
type OpenAIRouteCostEstimate struct {
	EstimatedBaseCostUSD float64
	ObservedMeanCostUSD  float64
	Samples              uint64
	Source               string
}

func EstimateOpenAIRouteBaseCost(
	requestClass OpenAIRouteRequestClass,
	configuredBaseCostUSD float64,
	aggregate OpenAIRouteObservationAggregate,
) OpenAIRouteCostEstimate {
	result := OpenAIRouteCostEstimate{
		EstimatedBaseCostUSD: configuredBaseCostUSD,
		Source:               OpenAIRouteCostEstimateSourcePolicy,
	}
	if requestClass != OpenAIRouteRequestClassText ||
		!isFinitePositive(configuredBaseCostUSD) ||
		aggregate.ActualCostSamples < openAIRouteCostEstimateMinimumSamples ||
		!isFinitePositive(aggregate.ActualBaseCostUSD) {
		return result
	}

	observedMean := aggregate.ActualBaseCostUSD / float64(aggregate.ActualCostSamples)
	if !isFinitePositive(observedMean) {
		return result
	}
	shrunk := (configuredBaseCostUSD*openAIRouteCostEstimatePriorSamples + aggregate.ActualBaseCostUSD) /
		(openAIRouteCostEstimatePriorSamples + float64(aggregate.ActualCostSamples))
	if !isFinitePositive(shrunk) {
		return result
	}
	minimum := configuredBaseCostUSD * openAIRouteCostEstimateMinimumRatio
	maximum := configuredBaseCostUSD * openAIRouteCostEstimateMaximumRatio
	result.EstimatedBaseCostUSD = math.Max(minimum, math.Min(maximum, shrunk))
	result.ObservedMeanCostUSD = observedMean
	result.Samples = aggregate.ActualCostSamples
	result.Source = OpenAIRouteCostEstimateSourceSettledText
	return result
}

func isFinitePositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

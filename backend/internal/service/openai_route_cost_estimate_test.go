package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEstimateOpenAIRouteBaseCostUsesShrunkSettledTextEvidence(t *testing.T) {
	aggregate := NewOpenAIRouteObservationAggregate()
	aggregate.ActualCostSamples = 80
	aggregate.ActualBaseCostUSD = 16 // observed mean = 0.20

	estimate := EstimateOpenAIRouteBaseCost(OpenAIRouteRequestClassText, 0.10, aggregate)

	require.Equal(t, OpenAIRouteCostEstimateSourceSettledText, estimate.Source)
	require.Equal(t, uint64(80), estimate.Samples)
	require.InDelta(t, 0.20, estimate.ObservedMeanCostUSD, 1e-12)
	require.InDelta(t, 0.18, estimate.EstimatedBaseCostUSD, 1e-12)
}

func TestEstimateOpenAIRouteBaseCostKeepsSparseOrInvalidEvidenceOnPolicyPrior(t *testing.T) {
	for name, aggregate := range map[string]OpenAIRouteObservationAggregate{
		"sparse": {
			ActualCostSamples: 19,
			ActualBaseCostUSD: 19,
		},
		"zero sum": {
			ActualCostSamples: 100,
		},
		"non finite": {
			ActualCostSamples: 100,
			ActualBaseCostUSD: math.Inf(1),
		},
	} {
		t.Run(name, func(t *testing.T) {
			estimate := EstimateOpenAIRouteBaseCost(OpenAIRouteRequestClassText, 0.10, aggregate)
			require.Equal(t, OpenAIRouteCostEstimateSourcePolicy, estimate.Source)
			require.InDelta(t, 0.10, estimate.EstimatedBaseCostUSD, 1e-12)
			require.Zero(t, estimate.Samples)
		})
	}
}

func TestEstimateOpenAIRouteBaseCostNeverLearnsImageCost(t *testing.T) {
	aggregate := NewOpenAIRouteObservationAggregate()
	aggregate.ActualCostSamples = 1_000
	aggregate.ActualBaseCostUSD = 1

	estimate := EstimateOpenAIRouteBaseCost(OpenAIRouteRequestClassImage, 0.30, aggregate)

	require.Equal(t, OpenAIRouteCostEstimateSourcePolicy, estimate.Source)
	require.InDelta(t, 0.30, estimate.EstimatedBaseCostUSD, 1e-12)
	require.Zero(t, estimate.Samples)
}

func TestEstimateOpenAIRouteBaseCostBoundsCorruptingHistoricalDrift(t *testing.T) {
	veryLow := NewOpenAIRouteObservationAggregate()
	veryLow.ActualCostSamples = 10_000
	veryLow.ActualBaseCostUSD = 0.001
	low := EstimateOpenAIRouteBaseCost(OpenAIRouteRequestClassText, 0.10, veryLow)
	require.InDelta(t, 0.025, low.EstimatedBaseCostUSD, 1e-12)

	veryHigh := NewOpenAIRouteObservationAggregate()
	veryHigh.ActualCostSamples = 10_000
	veryHigh.ActualBaseCostUSD = 10_000
	high := EstimateOpenAIRouteBaseCost(OpenAIRouteRequestClassText, 0.10, veryHigh)
	require.InDelta(t, 0.40, high.EstimatedBaseCostUSD, 1e-12)
}

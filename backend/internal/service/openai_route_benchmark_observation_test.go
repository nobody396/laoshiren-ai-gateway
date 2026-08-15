package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testOpenAIRouteBenchmarkProfile(samples, successes uint64, observedAt time.Time) OpenAIRouteBenchmarkObservationProfile {
	profile := NewOpenAIRouteBenchmarkObservationProfile()
	profile.Global.AttemptCount = samples
	profile.Global.ReliabilityCount = samples
	profile.Global.SuccessCount = successes
	profile.Global.FailureCount = samples - successes
	profile.Global.LastObservedAt = observedAt
	profile.Global.TTFTSampleCount = successes
	profile.Global.TTFTHistogram[OpenAIRouteLatencyHistogramBucket(500)] = successes
	profile.Global.LatencySampleCount = successes
	profile.Global.LatencyHistogram[OpenAIRouteLatencyHistogramBucket(2_000)] = successes
	profile.Recent = profile.Global
	profile.HourOfWeek = profile.Global
	return profile
}

func TestFinalizeOpenAIRouteBenchmarkObservationProfileRejectsSparseEvidence(t *testing.T) {
	now := time.Date(2026, 8, 15, 8, 30, 0, 0, time.UTC)
	profile := FinalizeOpenAIRouteBenchmarkObservationProfile(
		testOpenAIRouteBenchmarkProfile(OpenAIRouteBenchmarkMinimumSamples-1, 11, now.Add(-time.Minute)),
		now,
	)

	require.Equal(t, uint64(11), profile.Evidence.RawSamples)
	require.Zero(t, profile.Evidence.Confidence)
	require.Zero(t, profile.Evidence.EffectiveSamples)
	_, applied := ApplyOpenAIRouteBenchmarkPrior(NewOpenAIRouteObservationAggregate(), profile)
	require.False(t, applied, "a sparse time-slot winner must have exactly zero scoring influence")
}

func TestApplyOpenAIRouteBenchmarkPriorIsBoundedAndLeavesPassiveAccountingUntouched(t *testing.T) {
	now := time.Date(2026, 8, 15, 8, 30, 0, 0, time.UTC)
	profile := FinalizeOpenAIRouteBenchmarkObservationProfile(
		testOpenAIRouteBenchmarkProfile(OpenAIRouteBenchmarkFullSamples, 80, now),
		now,
	)
	passive := NewOpenAIRouteObservationAggregate()
	passive.AttemptCount = 100
	passive.ReliabilityCount = 100
	passive.SuccessCount = 90
	passive.FailureCount = 10
	passive.ActualCostSamples = 100
	passive.ActualBaseCostUSD = 20
	passive.ActualAccountCostUSD = 4

	blended, applied := ApplyOpenAIRouteBenchmarkPrior(passive, profile)

	require.True(t, applied)
	require.InDelta(t, OpenAIRouteBenchmarkMaxConfidence, profile.Evidence.Confidence, 1e-12)
	require.Equal(t, uint64(20), profile.Evidence.EffectiveSamples)
	require.Equal(t, uint64(120), blended.ReliabilityCount)
	require.Equal(t, uint64(100), blended.ActualCostSamples, "active probes cannot train customer cost")
	require.InDelta(t, 20, blended.ActualBaseCostUSD, 1e-12)
	require.InDelta(t, 4, blended.ActualAccountCostUSD, 1e-12)
}

func TestFinalizeOpenAIRouteBenchmarkObservationProfileDecaysWithRecency(t *testing.T) {
	now := time.Date(2026, 8, 15, 8, 30, 0, 0, time.UTC)
	fresh := FinalizeOpenAIRouteBenchmarkObservationProfile(
		testOpenAIRouteBenchmarkProfile(80, 80, now),
		now,
	)
	aged := FinalizeOpenAIRouteBenchmarkObservationProfile(
		testOpenAIRouteBenchmarkProfile(80, 80, now.Add(-OpenAIRouteBenchmarkRecencyHalfLife)),
		now,
	)

	require.InDelta(t, 1, fresh.Evidence.RecencyWeight, 1e-12)
	require.InDelta(t, 0.5, aged.Evidence.RecencyWeight, 1e-12)
	require.InDelta(t, fresh.Evidence.Confidence/2, aged.Evidence.Confidence, 1e-12)
	require.Less(t, aged.Evidence.EffectiveSamples, fresh.Evidence.EffectiveSamples)
}

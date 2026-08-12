package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOpenAIRouteObservationValidate(t *testing.T) {
	key := OpenAIRouteKey{
		GroupID:       7,
		AccountID:     23,
		Model:         "gpt-5.6-sol",
		RequestClass:  OpenAIRouteRequestClassText,
		EndpointHash:  "endpoint",
		Transport:     "http_sse",
		FailureDomain: "pomoai",
	}
	require.NoError(t, (OpenAIRouteObservation{Key: key, Success: true, FailureClass: OpenAIRouteFailureNone}).Validate())
	require.NoError(t, (OpenAIRouteObservation{Key: key, FailureClass: OpenAIRouteFailureUserRequest}).Validate())
	require.Error(t, (OpenAIRouteObservation{Key: key}).Validate())
	require.Error(t, (OpenAIRouteObservation{Key: key, Success: true, FailureClass: OpenAIRouteFailureRateLimit}).Validate())
	require.Error(t, (OpenAIRouteObservation{Key: key, FailureClass: OpenAIRouteFailureRateLimit, CompletionLatencyMS: -1}).Validate())
}

func TestOpenAIRouteHistogramPercentile(t *testing.T) {
	histogram := make([]uint64, len(OpenAIRouteLatencyHistogramUpperBoundsMS))
	histogram[OpenAIRouteLatencyHistogramBucket(90)]++
	histogram[OpenAIRouteLatencyHistogramBucket(700)]++
	histogram[OpenAIRouteLatencyHistogramBucket(4_100)]++
	require.Equal(t, float64(1_000), OpenAIRouteHistogramPercentile(histogram, 3, 0.50))
	require.Equal(t, float64(5_000), OpenAIRouteHistogramPercentile(histogram, 3, 0.95))
	require.Zero(t, OpenAIRouteHistogramPercentile(histogram, 0, 0.95))
}

func TestBlendOpenAIRouteObservationProfileRequiresSeasonalEvidence(t *testing.T) {
	global := NewOpenAIRouteObservationAggregate()
	global.ReliabilityCount = 100
	global.SuccessCount = 99
	global.TTFTSampleCount = 100
	global.TTFTHistogram[OpenAIRouteLatencyHistogramBucket(1_000)] = 100
	global.LastObservedAt = time.Date(2026, 8, 12, 13, 0, 0, 0, time.UTC)

	recent := NewOpenAIRouteObservationAggregate()
	recent.ReliabilityCount = 20
	recent.SuccessCount = 16
	recent.TTFTSampleCount = 20
	recent.TTFTHistogram[OpenAIRouteLatencyHistogramBucket(5_000)] = 20

	sparseSeasonal := NewOpenAIRouteObservationAggregate()
	sparseSeasonal.ReliabilityCount = 19
	sparseSeasonal.SuccessCount = 1
	sparseSeasonal.TTFTSampleCount = 19
	sparseSeasonal.TTFTHistogram[OpenAIRouteLatencyHistogramBucket(30_000)] = 19

	withoutSeasonal := BlendOpenAIRouteObservationProfile(OpenAIRouteObservationProfile{
		Global: global, Recent: recent, HourOfWeek: sparseSeasonal,
	})
	withNoSeasonal := BlendOpenAIRouteObservationProfile(OpenAIRouteObservationProfile{
		Global: global, Recent: recent,
	})
	require.InDelta(t, withNoSeasonal.SuccessLowerBound(), withoutSeasonal.SuccessLowerBound(), 1e-12)
	require.Equal(t, withNoSeasonal.TTFTPercentile(0.90), withoutSeasonal.TTFTPercentile(0.90))

	seasonal := sparseSeasonal
	seasonal.ReliabilityCount = 80
	seasonal.SuccessCount = 40
	seasonal.TTFTSampleCount = 80
	seasonal.TTFTHistogram[OpenAIRouteLatencyHistogramBucket(30_000)] = 80
	withSeasonal := BlendOpenAIRouteObservationProfile(OpenAIRouteObservationProfile{
		Global: global, Recent: recent, HourOfWeek: seasonal,
	})
	require.Less(t, withSeasonal.SuccessLowerBound(), withNoSeasonal.SuccessLowerBound())
	require.GreaterOrEqual(t, withSeasonal.TTFTPercentile(0.90), withNoSeasonal.TTFTPercentile(0.90))
}

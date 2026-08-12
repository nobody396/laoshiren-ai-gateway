package repository

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func testOpenAIRouteObservationKey(requestClass service.OpenAIRouteRequestClass) service.OpenAIRouteKey {
	return service.OpenAIRouteKey{
		GroupID:       7,
		AccountID:     23,
		Model:         "gpt-5.6-sol",
		RequestClass:  requestClass,
		EndpointHash:  "endpoint",
		Transport:     "http_sse",
		FailureDomain: "pomoai",
	}
}

func newOpenAIRouteObservationCacheTest(t *testing.T) (service.OpenAIRouteObservationStore, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	return NewOpenAIRouteObservationCache(client), server
}

func TestOpenAIRouteObservationCache_RecordAndReadWindows(t *testing.T) {
	store, _ := newOpenAIRouteObservationCacheTest(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 13, 30, 0, 0, time.UTC) // 21:30 Beijing
	key := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText)

	require.NoError(t, store.Record(ctx, service.OpenAIRouteObservation{
		Key:                     key,
		ObservedAt:              now,
		Success:                 true,
		FailureClass:            service.OpenAIRouteFailureNone,
		TTFTMilliseconds:        820,
		CompletionLatencyMS:     4_100,
		ActualBaseCostUSD:       0.12,
		ActualAccountCostUSD:    0.024,
		ActualCostAuthoritative: true,
	}))
	require.NoError(t, store.Record(ctx, service.OpenAIRouteObservation{
		Key:                 key,
		ObservedAt:          now.Add(time.Minute),
		FailureClass:        service.OpenAIRouteFailureRateLimit,
		PenalizeRoute:       true,
		CompletionLatencyMS: 1_200,
		PartialStream:       true,
	}))

	profiles, err := store.GetBatch(ctx, []service.OpenAIRouteKey{key}, now.Add(2*time.Minute))
	require.NoError(t, err)
	profile := profiles[service.OpenAIRouteObservationFingerprint(key)]
	for _, aggregate := range []service.OpenAIRouteObservationAggregate{profile.Global, profile.Recent, profile.HourOfWeek} {
		require.Equal(t, uint64(2), aggregate.AttemptCount)
		require.Equal(t, uint64(2), aggregate.ReliabilityCount)
		require.Equal(t, uint64(1), aggregate.SuccessCount)
		require.Equal(t, uint64(1), aggregate.FailureCount)
		require.Equal(t, uint64(1), aggregate.PartialStreams)
		require.Equal(t, uint64(1), aggregate.FailureCounts[service.OpenAIRouteFailureRateLimit])
		require.Equal(t, uint64(1), aggregate.TTFTSampleCount)
		require.Equal(t, float64(1_000), aggregate.TTFTPercentile(0.90))
		require.Equal(t, uint64(2), aggregate.LatencySampleCount)
		require.Equal(t, float64(5_000), aggregate.LatencyPercentile(0.95))
		require.Equal(t, uint64(1), aggregate.ActualCostSamples)
		require.InDelta(t, 0.12, aggregate.ActualBaseCostUSD, 1e-12)
		require.InDelta(t, 0.024, aggregate.ActualAccountCostUSD, 1e-12)
		require.Equal(t, now.Add(time.Minute), aggregate.LastObservedAt)
	}
}

func TestOpenAIRouteObservationCache_RecentAndSeasonalAreBounded(t *testing.T) {
	store, _ := newOpenAIRouteObservationCacheTest(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 13, 30, 0, 0, time.UTC)
	key := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText)

	require.NoError(t, store.Record(ctx, service.OpenAIRouteObservation{
		Key:          key,
		ObservedAt:   now.Add(-2 * time.Hour),
		Success:      true,
		FailureClass: service.OpenAIRouteFailureNone,
	}))
	profiles, err := store.GetBatch(ctx, []service.OpenAIRouteKey{key}, now)
	require.NoError(t, err)
	profile := profiles[service.OpenAIRouteObservationFingerprint(key)]
	require.Equal(t, uint64(1), profile.Global.AttemptCount)
	require.Zero(t, profile.Recent.AttemptCount)
	require.Zero(t, profile.HourOfWeek.AttemptCount, "a different Beijing hour must not train this hourly slice")
}

func TestOpenAIRouteObservationCache_RequestClassesNeverShareStatistics(t *testing.T) {
	store, _ := newOpenAIRouteObservationCacheTest(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 12, 13, 30, 0, 0, time.UTC)
	textKey := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText)
	imageKey := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassImage)

	require.NoError(t, store.Record(ctx, service.OpenAIRouteObservation{
		Key:          textKey,
		ObservedAt:   now,
		Success:      true,
		FailureClass: service.OpenAIRouteFailureNone,
	}))
	profiles, err := store.GetBatch(ctx, []service.OpenAIRouteKey{textKey, imageKey}, now)
	require.NoError(t, err)
	require.Equal(t, uint64(1), profiles[service.OpenAIRouteObservationFingerprint(textKey)].Global.AttemptCount)
	require.Zero(t, profiles[service.OpenAIRouteObservationFingerprint(imageKey)].Global.AttemptCount)
}

func TestOpenAIRouteObservationCache_UserRequestFailureDoesNotPenalizeReliability(t *testing.T) {
	store, _ := newOpenAIRouteObservationCacheTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	key := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText)
	require.NoError(t, store.Record(ctx, service.OpenAIRouteObservation{
		Key:                 key,
		ObservedAt:          now,
		FailureClass:        service.OpenAIRouteFailureUserRequest,
		PartialStream:       true,
		TTFTMilliseconds:    100,
		CompletionLatencyMS: 500,
	}))
	profiles, err := store.GetBatch(ctx, []service.OpenAIRouteKey{key}, now)
	require.NoError(t, err)
	aggregate := profiles[service.OpenAIRouteObservationFingerprint(key)].Global
	require.Equal(t, uint64(1), aggregate.AttemptCount)
	require.Zero(t, aggregate.ReliabilityCount)
	require.Equal(t, uint64(1), aggregate.FailureCount)
	require.Zero(t, aggregate.PartialStreams)
	require.Zero(t, aggregate.TTFTSampleCount)
	require.Zero(t, aggregate.LatencySampleCount)
}

func TestOpenAIRouteObservationCache_ClientCancellationIsNeutralToScoring(t *testing.T) {
	store, _ := newOpenAIRouteObservationCacheTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	key := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText)

	require.NoError(t, store.Record(ctx, service.OpenAIRouteObservation{
		Key:                 key,
		ObservedAt:          now,
		FailureClass:        service.OpenAIRouteFailureClientCancelled,
		TTFTMilliseconds:    250,
		CompletionLatencyMS: 2_000,
	}))
	profiles, err := store.GetBatch(ctx, []service.OpenAIRouteKey{key}, now)
	require.NoError(t, err)
	aggregate := profiles[service.OpenAIRouteObservationFingerprint(key)].Global
	require.Equal(t, uint64(1), aggregate.AttemptCount)
	require.Equal(t, uint64(1), aggregate.FailureCount)
	require.Zero(t, aggregate.ReliabilityCount)
	require.Zero(t, aggregate.PartialStreams)
	require.Zero(t, aggregate.TTFTSampleCount)
	require.Zero(t, aggregate.LatencySampleCount)
}

func TestOpenAIRouteObservationCache_CostSettlementDoesNotDoubleCountAttempt(t *testing.T) {
	store, _ := newOpenAIRouteObservationCacheTest(t)
	ctx := context.Background()
	now := time.Now().UTC()
	key := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText)
	require.NoError(t, store.Record(ctx, service.OpenAIRouteObservation{
		Key: key, ObservedAt: now, Success: true, FailureClass: service.OpenAIRouteFailureNone,
	}))
	require.NoError(t, store.RecordCost(ctx, service.OpenAIRouteActualCostObservation{
		Key: key, ObservedAt: now, ActualBaseCostUSD: 0.20, ActualAccountCostUSD: 0.04,
	}))
	profiles, err := store.GetBatch(ctx, []service.OpenAIRouteKey{key}, now)
	require.NoError(t, err)
	aggregate := profiles[service.OpenAIRouteObservationFingerprint(key)].Global
	require.Equal(t, uint64(1), aggregate.AttemptCount)
	require.Equal(t, uint64(1), aggregate.SuccessCount)
	require.Equal(t, uint64(1), aggregate.ActualCostSamples)
	require.InDelta(t, 0.20, aggregate.ActualBaseCostUSD, 1e-12)
	require.InDelta(t, 0.04, aggregate.ActualAccountCostUSD, 1e-12)
}

//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIRouteObservationCheckpoint_RoundTripAndRestartRecovery(t *testing.T) {
	ctx := context.Background()
	repository := NewOpenAIRouteObservationCheckpointRepository(integrationDB)
	key := service.OpenAIRouteKey{
		GroupID: 700001, AccountID: 230001, FailureDomain: "integration-pomelo-hk",
		Model: "gpt-5.6-sol-integration", RequestClass: service.OpenAIRouteRequestClassText,
		EndpointHash: "0123456789abcdef", Transport: "http_sse",
	}
	fingerprint := service.OpenAIRouteObservationFingerprint(key)
	now := time.Date(2026, 8, 12, 13, 30, 0, 0, time.UTC)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `
DELETE FROM openai_route_observation_hourly WHERE route_fingerprint = $1
`, fingerprint)
	})

	require.NoError(t, repository.Record(ctx, service.OpenAIRouteObservation{
		Key: key, ObservedAt: now, Success: true, FailureClass: service.OpenAIRouteFailureNone,
		TTFTMilliseconds: 820, CompletionLatencyMS: 4_100,
		ActualBaseCostUSD: 0.12, ActualAccountCostUSD: 0.024, ActualCostAuthoritative: true,
	}))
	require.NoError(t, repository.Record(ctx, service.OpenAIRouteObservation{
		Key: key, ObservedAt: now.Add(time.Minute), FailureClass: service.OpenAIRouteFailureRateLimit,
		PenalizeRoute: true, PartialStream: true, CompletionLatencyMS: 1_200,
	}))
	require.NoError(t, repository.RecordCost(ctx, service.OpenAIRouteActualCostObservation{
		Key: key, ObservedAt: now.Add(2 * time.Minute), ActualBaseCostUSD: 0.03, ActualAccountCostUSD: 0.006,
	}))

	profiles, err := repository.GetBatch(ctx, []service.OpenAIRouteKey{key}, now.Add(3*time.Minute))
	require.NoError(t, err)
	profile := profiles[fingerprint]
	for _, aggregate := range []service.OpenAIRouteObservationAggregate{profile.Global, profile.HourOfWeek} {
		require.Equal(t, uint64(2), aggregate.AttemptCount)
		require.Equal(t, uint64(2), aggregate.ReliabilityCount)
		require.Equal(t, uint64(1), aggregate.SuccessCount)
		require.Equal(t, uint64(1), aggregate.FailureCount)
		require.Equal(t, uint64(1), aggregate.PartialStreams)
		require.Equal(t, uint64(1), aggregate.FailureCounts[service.OpenAIRouteFailureRateLimit])
		require.Equal(t, uint64(2), aggregate.ActualCostSamples)
		require.InDelta(t, 0.15, aggregate.ActualBaseCostUSD, 1e-12)
		require.InDelta(t, 0.03, aggregate.ActualAccountCostUSD, 1e-12)
		require.Equal(t, now.Add(2*time.Minute), aggregate.LastObservedAt)
	}
	require.Zero(t, profile.Recent.AttemptCount, "durable hourly storage never fabricates the recent Redis signal")
}

func TestOpenAIRouteObservationCheckpoint_RejectsFingerprintIdentityMismatch(t *testing.T) {
	ctx := context.Background()
	repository := NewOpenAIRouteObservationCheckpointRepository(integrationDB)
	key := service.OpenAIRouteKey{
		GroupID: 700002, AccountID: 230002, FailureDomain: "integration-domain-a",
		Model: "gpt-5.6-sol-integration", RequestClass: service.OpenAIRouteRequestClassText,
		EndpointHash: "fedcba9876543210", Transport: "http_sse",
	}
	fingerprint := service.OpenAIRouteObservationFingerprint(key)
	now := time.Date(2026, 8, 12, 13, 30, 0, 0, time.UTC)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `
DELETE FROM openai_route_observation_hourly WHERE route_fingerprint = $1
`, fingerprint)
	})
	require.NoError(t, repository.Record(ctx, service.OpenAIRouteObservation{
		Key: key, ObservedAt: now, Success: true, FailureClass: service.OpenAIRouteFailureNone,
	}))

	// Force a corrupted identity under the same fingerprint and prove the
	// conflict-update guard refuses to merge unrelated route dimensions.
	result, err := integrationDB.ExecContext(ctx, `
UPDATE openai_route_observation_hourly
SET failure_domain = 'tampered-domain'
WHERE route_fingerprint = $1 AND hour_start = $2
`, fingerprint, now.Truncate(time.Hour))
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.Equal(t, int64(1), affected)
	err = repository.Record(ctx, service.OpenAIRouteObservation{
		Key: key, ObservedAt: now.Add(2 * time.Minute), Success: true, FailureClass: service.OpenAIRouteFailureNone,
	})
	require.ErrorContains(t, err, "fingerprint identity mismatch")
}

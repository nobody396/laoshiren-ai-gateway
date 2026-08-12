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

func TestOpenAIRouteHealthCache_ProviderEvidenceRequiresDistinctAccounts(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store := NewOpenAIRouteHealthCache(client, time.Hour, 10*time.Second)
	ctx := context.Background()
	policy := service.DefaultOpenAIRoutePolicy()
	start := time.Now().UTC()
	first := service.OpenAIRouteKey{
		GroupID: 7, AccountID: 23, FailureDomain: "pomoai", Model: "gpt-5.6-sol",
		RequestClass: service.OpenAIRouteRequestClassText, EndpointHash: "hk", Transport: "http_sse",
	}
	second := first
	second.AccountID = 24
	second.EndpointHash = "jp"
	failure := service.OpenAIRouteHealthEvent{At: start, FailureClass: service.OpenAIRouteFailureUpstream5xx}

	result, err := store.RecordProviderEvidence(ctx, first, failure, policy, 2)
	require.NoError(t, err)
	require.Equal(t, 1, result.DistinctFailingAccounts)
	require.False(t, result.ProviderEventApplied)

	failure.At = start.Add(time.Millisecond)
	result, err = store.RecordProviderEvidence(ctx, first, failure, policy, 2)
	require.NoError(t, err)
	require.Equal(t, 1, result.DistinctFailingAccounts)
	require.False(t, result.ProviderEventApplied)

	failure.At = start.Add(2 * time.Millisecond)
	result, err = store.RecordProviderEvidence(ctx, second, failure, policy, 2)
	require.NoError(t, err)
	require.Equal(t, 2, result.DistinctFailingAccounts)
	require.True(t, result.ProviderEventApplied)
	require.Equal(t, service.OpenAIRouteCircuitOpen, result.State.State)

	success := service.OpenAIRouteHealthEvent{At: start.Add(3 * time.Millisecond), Success: true, FailureClass: service.OpenAIRouteFailureNone}
	result, err = store.RecordProviderEvidence(ctx, first, success, policy, 2)
	require.NoError(t, err)
	require.Equal(t, 1, result.DistinctFailingAccounts)
	require.False(t, result.ProviderEventApplied)

	success.At = start.Add(4 * time.Millisecond)
	result, err = store.RecordProviderEvidence(ctx, second, success, policy, 2)
	require.NoError(t, err)
	require.Zero(t, result.DistinctFailingAccounts)
	require.True(t, result.ProviderEventApplied)
	require.Equal(t, service.OpenAIRouteCircuitRecovering, result.State.State)
}

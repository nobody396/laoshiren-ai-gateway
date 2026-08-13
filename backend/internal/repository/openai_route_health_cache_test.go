package repository

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

type openAIRouteHealthPipelineHook struct {
	pipelines atomic.Uint64
	commands  atomic.Uint64
}

func (*openAIRouteHealthPipelineHook) DialHook(next redis.DialHook) redis.DialHook { return next }

func (*openAIRouteHealthPipelineHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return next
}

func (h *openAIRouteHealthPipelineHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		h.pipelines.Add(1)
		h.commands.Add(uint64(len(cmds)))
		return next(ctx, cmds)
	}
}

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

func TestOpenAIRouteHealthCache_GetBatchDeduplicatesAndDefaultsMissingState(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	hook := &openAIRouteHealthPipelineHook{}
	client.AddHook(hook)
	store := NewOpenAIRouteHealthCache(client, time.Hour, 10*time.Second)
	ctx := context.Background()
	require.NoError(t, client.Ping(ctx).Err())
	hook.pipelines.Store(0)
	hook.commands.Store(0)
	route := service.OpenAIRouteHealthStoreKey{
		Scope: service.OpenAIRouteHealthScopeRoute, GroupID: 7, AccountID: 23,
		FailureDomain: "pomoai", Model: "gpt-5.6-sol", RequestClass: service.OpenAIRouteRequestClassText,
		EndpointHash: "hk", Transport: "http_sse",
	}
	provider := service.OpenAIRouteHealthStoreKey{
		Scope: service.OpenAIRouteHealthScopeProvider, GroupID: 7, FailureDomain: "pomoai",
		Model: "gpt-5.6-sol", RequestClass: service.OpenAIRouteRequestClassText,
	}

	states, err := store.GetBatch(ctx, []service.OpenAIRouteHealthStoreKey{route, provider, route})
	require.NoError(t, err)
	require.Len(t, states, 2)
	require.Equal(t, service.OpenAIRouteCircuitWarmup, states[route.Fingerprint()].State)
	require.Equal(t, service.OpenAIRouteCircuitWarmup, states[provider.Fingerprint()].State)
	require.Equal(t, uint64(1), hook.pipelines.Load())
	require.Equal(t, uint64(2), hook.commands.Load(), "duplicate health keys must not generate duplicate Redis commands")

	openedAt := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	_, err = store.ApplyEvent(ctx, route, service.OpenAIRouteHealthEvent{
		At: openedAt, FailureClass: service.OpenAIRouteFailureUpstream5xx,
	}, service.DefaultOpenAIRoutePolicy())
	require.NoError(t, err)
	states, err = store.GetBatch(ctx, []service.OpenAIRouteHealthStoreKey{provider, route})
	require.NoError(t, err)
	require.Equal(t, service.OpenAIRouteCircuitOpen, states[route.Fingerprint()].State)
	require.Equal(t, service.OpenAIRouteCircuitWarmup, states[provider.Fingerprint()].State)

	require.NoError(t, client.Set(ctx, openAIRouteHealthRedisKey(provider), "not-json", time.Hour).Err())
	_, err = store.GetBatch(ctx, []service.OpenAIRouteHealthStoreKey{route, provider})
	require.ErrorContains(t, err, "decode OpenAI route health state")

	_, err = store.GetBatch(ctx, []service.OpenAIRouteHealthStoreKey{{}})
	require.ErrorIs(t, err, service.ErrOpenAIRouteNoCandidate)
}

func TestOpenAIRouteHealthCache_RefreshPermitNeverReacquires(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, client.Close()) })
	store := NewOpenAIRouteHealthCache(client, time.Hour, 10*time.Second)
	ctx := context.Background()
	key := service.OpenAIRouteHealthStoreKey{
		Scope: service.OpenAIRouteHealthScopeRoute, GroupID: 7, AccountID: 23,
		FailureDomain: "pomoai", Model: "gpt-5.6-sol", RequestClass: service.OpenAIRouteRequestClassText,
		EndpointHash: "hk", Transport: "http_sse",
	}

	ok, err := store.RefreshHalfOpenPermit(ctx, key, "worker-a")
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = store.AcquireHalfOpenPermit(ctx, key, "worker-a")
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = store.RefreshHalfOpenPermit(ctx, key, "worker-b")
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = store.RefreshHalfOpenPermit(ctx, key, "worker-a")
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, store.ReleaseHalfOpenPermit(ctx, key, "worker-a"))
	ok, err = store.RefreshHalfOpenPermit(ctx, key, "worker-a")
	require.NoError(t, err)
	require.False(t, ok)
}

//go:build unit

package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestGatewayCacheReasoningContentIsTenantScopedAndBounded(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := &gatewayCache{rdb: client}
	ctx := context.Background()

	scope := service.ReasoningCacheScope{UserID: 7, APIKeyID: 11, Model: "deepseek-v4"}
	require.NoError(t, cache.SetReasoningContent(ctx, scope, "rs_1", "private plan", time.Hour))
	got, err := cache.GetReasoningContent(ctx, scope, "rs_1")
	require.NoError(t, err)
	require.Equal(t, "private plan", got)

	_, err = cache.GetReasoningContent(ctx, service.ReasoningCacheScope{UserID: 8, APIKeyID: 11, Model: "deepseek-v4"}, "rs_1")
	require.ErrorIs(t, err, service.ErrReasoningContentNotFound)

	err = cache.SetReasoningContent(ctx, scope, "rs_big", strings.Repeat("x", service.ReasoningContentMaxBytes+1), time.Hour)
	require.ErrorIs(t, err, service.ErrReasoningContentTooLarge)
}

func TestGatewayCacheReasoningContentTTLAndInvalidScopeFailClosed(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cache := &gatewayCache{rdb: client}
	ctx := context.Background()
	scope := service.ReasoningCacheScope{UserID: 7, APIKeyID: 11, Model: "deepseek-v4"}

	require.NoError(t, cache.SetReasoningContent(ctx, scope, "rs_ttl", "plan", time.Minute))
	server.FastForward(2 * time.Minute)
	_, err := cache.GetReasoningContent(ctx, scope, "rs_ttl")
	require.ErrorIs(t, err, service.ErrReasoningContentNotFound)

	require.NoError(t, cache.SetReasoningContent(ctx, service.ReasoningCacheScope{}, "rs_invalid", "ignored", time.Hour))
	_, err = cache.GetReasoningContent(ctx, service.ReasoningCacheScope{}, "rs_invalid")
	require.ErrorIs(t, err, service.ErrReasoningContentNotFound)
}

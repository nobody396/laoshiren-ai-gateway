package repository

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai_compat"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestFilterSchedulerExtraPreservesOpenAIImageRouting(t *testing.T) {
	extra := map[string]any{
		service.OpenAIImageGenerationPriorityExtraKey: 1,
		service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6", "gpt-5.6-sol"},
		"unrelated_admin_only_field":                  "drop-me",
	}

	filtered := filterSchedulerExtra(extra)

	require.Equal(t, 1, filtered[service.OpenAIImageGenerationPriorityExtraKey])
	require.Equal(t, []any{"gpt-5.6", "gpt-5.6-sol"}, filtered[service.OpenAIImageGenerationModelsExtraKey])
	require.NotContains(t, filtered, "unrelated_admin_only_field")
}

func TestFilterSchedulerExtraPreservesOpenAIProtocolCapabilities(t *testing.T) {
	protocols := map[string]any{
		"ZHIPU/GLM-5.3":      openai_compat.UpstreamProtocolChatCompletions,
		"MiniMax/MiniMax-M3": openai_compat.UpstreamProtocolChatCompletions,
		"glm-5.2":            openai_compat.UpstreamProtocolResponses,
	}
	extra := map[string]any{
		openai_compat.ExtraKeyResponsesSupported:      true,
		openai_compat.ExtraKeyResponsesMode:           string(openai_compat.ResponsesSupportModeAuto),
		openai_compat.ExtraKeyUpstreamProtocolByModel: protocols,
		"unrelated_admin_only_field":                  "drop-me",
	}

	filtered := filterSchedulerExtra(extra)

	require.Equal(t, true, filtered[openai_compat.ExtraKeyResponsesSupported])
	require.Equal(t, string(openai_compat.ResponsesSupportModeAuto), filtered[openai_compat.ExtraKeyResponsesMode])
	require.Equal(t, protocols, filtered[openai_compat.ExtraKeyUpstreamProtocolByModel])
	require.NotContains(t, filtered, "unrelated_admin_only_field")
}

func TestBuildSchedulerMetadataAccountRetainsOpenAIImageRouting(t *testing.T) {
	account := service.Account{
		ID:       33,
		Platform: service.PlatformOpenAI,
		Extra: map[string]any{
			service.OpenAIImageGenerationPriorityExtraKey: 1,
			service.OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
		},
	}

	metadata := buildSchedulerMetadataAccount(account)

	priority, configured := metadata.OpenAIImageGenerationRoutingPriority("gpt-5.6-sol")
	require.True(t, configured)
	require.Equal(t, 1, priority)
}

func TestSchedulerSnapshotRoundTripRetainsOpenAIProtocolMatrix(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	cache := NewSchedulerCache(rdb)
	bucket := service.SchedulerBucket{GroupID: 6, Platform: service.PlatformOpenAI, Mode: service.AccountTypeAPIKey}
	account := service.Account{
		ID: 44, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported: true,
			openai_compat.ExtraKeyUpstreamProtocolByModel: map[string]any{
				"ZHIPU/GLM-5.3": openai_compat.UpstreamProtocolChatCompletions,
				"glm-5.2":       openai_compat.UpstreamProtocolResponses,
			},
		},
	}

	require.NoError(t, cache.SetSnapshot(ctx, bucket, []service.Account{account}))
	accounts, ready, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, ready)
	require.Len(t, accounts, 1)
	require.False(t, openai_compat.ShouldUseResponsesAPIForModel(accounts[0].Extra, "ZHIPU/GLM-5.3"))
	require.True(t, openai_compat.ShouldUseResponsesAPIForModel(accounts[0].Extra, "glm-5.2"))
}

func TestFilterSchedulerExtraPreservesOnlyFailureDomainRoutingMetadata(t *testing.T) {
	extra := map[string]any{
		service.OpenAIRouteFailureDomainExtraKey: " morecode:gpt-pro-0.2 ",
		"routing": map[string]any{
			"failure_domain_id": "legacy-morecode",
			"admin_secret":      "drop-me",
		},
		"unrelated_admin_only_field": "drop-me-too",
	}

	filtered := filterSchedulerExtra(extra)

	require.Equal(t, " morecode:gpt-pro-0.2 ", filtered[service.OpenAIRouteFailureDomainExtraKey])
	require.Equal(t, map[string]any{"failure_domain_id": "legacy-morecode"}, filtered["routing"])
	require.NotContains(t, filtered, "unrelated_admin_only_field")
	require.NotContains(t, filtered["routing"], "admin_secret")
}

func TestSchedulerSnapshotRoundTripRetainsExplicitFailureDomain(t *testing.T) {
	ctx := context.Background()
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { require.NoError(t, rdb.Close()) })

	cache := NewSchedulerCache(rdb)
	bucket := service.SchedulerBucket{GroupID: 6, Platform: service.PlatformOpenAI, Mode: service.AccountTypeAPIKey}
	account := service.Account{
		ID:       33,
		Platform: service.PlatformOpenAI,
		Type:     service.AccountTypeAPIKey,
		Extra: map[string]any{
			service.OpenAIRouteFailureDomainExtraKey: "morecode:gpt-pro-0.2",
			"unrelated_admin_only_field":             "drop-me",
		},
	}

	require.NoError(t, cache.SetSnapshot(ctx, bucket, []service.Account{account}))
	accounts, ready, err := cache.GetSnapshot(ctx, bucket)
	require.NoError(t, err)
	require.True(t, ready)
	require.Len(t, accounts, 1)
	require.Equal(t, "morecode:gpt-pro-0.2", service.OpenAIRouteFailureDomainID(accounts[0]))
	require.NotContains(t, accounts[0].Extra, "unrelated_admin_only_field")
}

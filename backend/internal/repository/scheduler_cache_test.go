package repository

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
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

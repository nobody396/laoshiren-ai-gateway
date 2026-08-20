package handler

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/claude"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestFilterInternalOnlyModelsHidesCompactVariants(t *testing.T) {
	models := filterInternalOnlyModels([]string{
		"gpt-5.6-sol",
		"gpt-5.6-sol-openai-compact",
		"gpt-5.5",
		"gpt-5.5-OpenAI-Compact",
		"gpt-5.4-openai-compact",
		"codex-auto-review",
		" Codex-Auto-Review ",
	}, 52)

	require.Equal(t, []string{"gpt-5.6-sol", "gpt-5.5"}, models)
}

func TestFilterInternalOnlyModelsHidesImageRendererOutsideImageGroups(t *testing.T) {
	candidates := []string{"gpt-5.6-sol", "gpt-image-2", " GPT-Image-2 "}

	// The image renderer is an auto-invoked routing target in chat groups and
	// in the no-group path: hidden from discovery, still routable.
	for _, groupID := range []int64{0, 6, 52, 59} {
		require.Equal(t, []string{"gpt-5.6-sol"}, filterInternalOnlyModels(candidates, groupID), groupID)
	}

	// The dedicated image group keeps it discoverable.
	require.Equal(t, candidates, filterInternalOnlyModels(candidates, 51))
}

func TestGatewayModelInfoFromIDsUsesUnifiedOwner(t *testing.T) {
	models := gatewayModelInfoFromIDs([]string{"gpt-5.5"}, 0)

	require.Len(t, models, 1)
	require.Equal(t, "gpt-5.5", models[0].ID)
	require.Equal(t, "model", models[0].Object)
	require.Equal(t, gatewayDefaultModelCreatedUnix, models[0].Created)
	require.Equal(t, gatewayModelOwner, models[0].OwnedBy)
	require.Equal(t, "model", models[0].Type)
	require.Equal(t, "gpt-5.5", models[0].DisplayName)
	require.Equal(t, gatewayDefaultModelCreatedAt, models[0].CreatedAt)
}

func TestGatewayModelInfoFromIDsHidesRetiredOpenAIModels(t *testing.T) {
	models := gatewayModelInfoFromIDs([]string{
		"gpt-5.4",
		"gpt-5.4-mini",
		"gpt-5.6-sol",
		"gpt-5.6-luna",
	}, 6)

	require.Len(t, models, 2)
	require.Equal(t, []string{"gpt-5.4", "gpt-5.6-sol"}, []string{models[0].ID, models[1].ID})
}

func TestGatewayModelInfoFromIDsKeepsExemptedGroupModels(t *testing.T) {
	for _, groupID := range []int64{52, 59} {
		models := gatewayModelInfoFromIDs([]string{
			"gpt-5.4",
			"gpt-5.4-mini",
			"gpt-5.6-sol",
			"gpt-5.6-luna",
		}, groupID)

		require.Len(t, models, 4, groupID)
		require.Equal(t, "gpt-5.4-mini", models[1].ID, groupID)
		require.Equal(t, "gpt-5.6-luna", models[3].ID, groupID)
	}
}

func TestGatewayModelInfoFromOpenAINormalizesOwner(t *testing.T) {
	models := gatewayModelInfoFromOpenAI([]openai.Model{
		{
			ID:          "gpt-5.5",
			Object:      "model",
			Created:     1776873600,
			OwnedBy:     "openai",
			Type:        "model",
			DisplayName: "GPT-5.5",
		},
	}, 0)

	require.Len(t, models, 1)
	require.Equal(t, "gpt-5.5", models[0].ID)
	require.Equal(t, int64(1776873600), models[0].Created)
	require.Equal(t, gatewayModelOwner, models[0].OwnedBy)
	require.Equal(t, "GPT-5.5", models[0].DisplayName)
	require.Equal(t, gatewayDefaultModelCreatedAt, models[0].CreatedAt)
}

func TestGatewayModelInfoFromClaudeAddsOpenAICompatibilityFields(t *testing.T) {
	models := gatewayModelInfoFromClaude([]claude.Model{
		{
			ID:          "claude-opus-4-5-20251101",
			Type:        "model",
			DisplayName: "Claude Opus 4.5",
			CreatedAt:   "2025-11-01T00:00:00Z",
		},
	})

	require.Len(t, models, 1)
	require.Equal(t, "claude-opus-4-5-20251101", models[0].ID)
	require.Equal(t, "model", models[0].Object)
	require.Equal(t, gatewayDefaultModelCreatedUnix, models[0].Created)
	require.Equal(t, gatewayModelOwner, models[0].OwnedBy)
	require.Equal(t, "model", models[0].Type)
	require.Equal(t, "Claude Opus 4.5", models[0].DisplayName)
	require.Equal(t, "2025-11-01T00:00:00Z", models[0].CreatedAt)
}

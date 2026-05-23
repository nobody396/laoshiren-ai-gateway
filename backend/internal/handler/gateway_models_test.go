package handler

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/claude"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestGatewayModelInfoFromIDsUsesUnifiedOwner(t *testing.T) {
	models := gatewayModelInfoFromIDs([]string{"gpt-5.5"})

	require.Len(t, models, 1)
	require.Equal(t, "gpt-5.5", models[0].ID)
	require.Equal(t, "model", models[0].Object)
	require.Equal(t, gatewayDefaultModelCreatedUnix, models[0].Created)
	require.Equal(t, gatewayModelOwner, models[0].OwnedBy)
	require.Equal(t, "model", models[0].Type)
	require.Equal(t, "gpt-5.5", models[0].DisplayName)
	require.Equal(t, gatewayDefaultModelCreatedAt, models[0].CreatedAt)
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
	})

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

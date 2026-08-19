package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveMessagesDispatchModelIgnoresRetiredTargets(t *testing.T) {
	t.Parallel()

	group := &Group{MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{
		OpusMappedModel:   "gpt-5.6-sol",
		SonnetMappedModel: "gpt-5.6-terra",
		HaikuMappedModel:  "gpt-5.6-luna",
		ExactModelMappings: map[string]string{
			"claude-haiku-4-5": "gpt-5.6-luna-xhigh",
		},
	}}

	require.Equal(t, "gpt-5.6-sol", group.ResolveMessagesDispatchModel("claude-opus-4-6"))
	require.Equal(t, "gpt-5.6-terra", group.ResolveMessagesDispatchModel("claude-sonnet-4-6"))
	require.Equal(t, defaultOpenAIMessagesDispatchHaikuMappedModel, group.ResolveMessagesDispatchModel("claude-haiku-4-5"))
}

func TestNormalizeOpenAIMessagesDispatchModelConfigDropsRetiredTargets(t *testing.T) {
	t.Parallel()

	normalized := normalizeOpenAIMessagesDispatchModelConfig(OpenAIMessagesDispatchModelConfig{
		HaikuMappedModel: "gpt-5.6-luna",
		ExactModelMappings: map[string]string{
			"claude-haiku-4-5":  "gpt-5.6-luna",
			"claude-sonnet-4-6": "gpt-5.6-terra",
		},
	}, 6)

	require.Empty(t, normalized.HaikuMappedModel)
	require.NotContains(t, normalized.ExactModelMappings, "claude-haiku-4-5")
	require.Equal(t, "gpt-5.6-terra", normalized.ExactModelMappings["claude-sonnet-4-6"])
}

func TestNormalizeOpenAIMessagesDispatchModelConfigKeepsExemptedGroupTargets(t *testing.T) {
	t.Parallel()

	normalized := normalizeOpenAIMessagesDispatchModelConfig(OpenAIMessagesDispatchModelConfig{
		HaikuMappedModel: "gpt-5.6-luna",
		ExactModelMappings: map[string]string{
			"claude-haiku-4-5": "gpt-5.6-luna",
		},
	}, 59)

	require.Equal(t, "gpt-5.6-luna", normalized.HaikuMappedModel)
	require.Equal(t, "gpt-5.6-luna", normalized.ExactModelMappings["claude-haiku-4-5"])
}

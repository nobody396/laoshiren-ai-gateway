//go:build unit

package service

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai"
	"github.com/stretchr/testify/require"
)

func TestOpenAIResponsesProbeModelForAccount(t *testing.T) {
	t.Run("nil account falls back to default test model", func(t *testing.T) {
		require.Equal(t, openai.DefaultTestModel, openaiResponsesProbeModelForAccount(nil))
	})

	t.Run("empty mapping falls back to default test model", func(t *testing.T) {
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{}}
		require.Equal(t, openai.DefaultTestModel, openaiResponsesProbeModelForAccount(account))
	})

	t.Run("uses upstream value of the first sorted mapping key", func(t *testing.T) {
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gpt-5.6":     "gpt-5.6-sol",
				"gpt-5.6-sol": "gpt-5.6-sol",
			},
		}}
		require.Equal(t, "gpt-5.6-sol", openaiResponsesProbeModelForAccount(account))
	})

	t.Run("falls back to the public key when the upstream value is blank", func(t *testing.T) {
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gpt-5.6-sol": "  ",
			},
		}}
		require.Equal(t, "gpt-5.6-sol", openaiResponsesProbeModelForAccount(account))
	})

	t.Run("skips image-only mappings that cannot answer a text probe", func(t *testing.T) {
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gpt-image-2": "gpt-image-2-count",
				"gpt-5.5":     "gpt-5.5",
			},
		}}
		require.Equal(t, "gpt-5.5", openaiResponsesProbeModelForAccount(account))
	})

	t.Run("image-only account keeps legacy default instead of probing with an image model", func(t *testing.T) {
		account := &Account{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
			"model_mapping": map[string]any{
				"gpt-image-2": "gpt-image-2-count",
			},
		}}
		require.Equal(t, openai.DefaultTestModel, openaiResponsesProbeModelForAccount(account))
	})
}

func TestOpenAIResponsesProbePayloadUsesSelectedModel(t *testing.T) {
	require.Contains(t, string(openaiResponsesProbePayload("gpt-5.6-sol")), `"model":"gpt-5.6-sol"`)
	require.Contains(t, string(openaiResponsesProbePayload("")), `"model":"`+openai.DefaultTestModel+`"`)
}

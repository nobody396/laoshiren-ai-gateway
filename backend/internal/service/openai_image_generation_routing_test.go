//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsExplicitOpenAIImageGenerationIntent(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "native image tool",
			body: `{"model":"gpt-5.4","tools":[{"type":"image_generation"}],"input":"draw"}`,
			want: true,
		},
		{
			name: "string tool choice",
			body: `{"model":"gpt-5.4","tool_choice":"image_generation"}`,
			want: true,
		},
		{
			name: "object tool choice",
			body: `{"model":"gpt-5.4","tool_choice":{"type":"image_generation"}}`,
			want: true,
		},
		{
			name: "nested object tool choice",
			body: `{"model":"gpt-5.4","tool_choice":{"tool":{"type":"image_generation"}}}`,
			want: true,
		},
		{
			name: "passive namespace catalog",
			body: `{"model":"gpt-5.4","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"tool_choice":"auto"}`,
			want: false,
		},
		{
			name: "ordinary function named image generation",
			body: `{"model":"gpt-5.4","tools":[{"type":"function","name":"image_generation"}],"tool_choice":"auto"}`,
			want: false,
		},
		{
			name: "text only",
			body: `{"model":"gpt-5.4","input":"hello"}`,
			want: false,
		},
		{
			name: "invalid json",
			body: `{`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsExplicitOpenAIImageGenerationIntent([]byte(tt.body)))
		})
	}
}

func TestAccountOpenAIImageGenerationRoutingPriority(t *testing.T) {
	account := &Account{
		Platform: PlatformOpenAI,
		Extra: map[string]any{
			OpenAIImageGenerationPriorityExtraKey: 1.0,
			OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.4", "gpt-5.6-*"},
		},
	}

	priority, configured := account.OpenAIImageGenerationRoutingPriority("gpt-5.4")
	require.True(t, configured)
	require.Equal(t, 1, priority)

	priority, configured = account.OpenAIImageGenerationRoutingPriority("gpt-5.6-sol")
	require.True(t, configured)
	require.Equal(t, 1, priority)

	priority, configured = account.OpenAIImageGenerationRoutingPriority("gpt-5.4-mini")
	require.False(t, configured)
	require.Zero(t, priority)

	account.Extra[OpenAIImageGenerationModelsExtraKey] = []any{}
	_, configured = account.OpenAIImageGenerationRoutingPriority("gpt-5.4")
	require.False(t, configured, "present empty allowlist must fail closed")

	delete(account.Extra, OpenAIImageGenerationModelsExtraKey)
	_, configured = account.OpenAIImageGenerationRoutingPriority("gpt-5.4-mini")
	require.True(t, configured, "missing allowlist intentionally applies to all models")

	account.Extra[OpenAIImageGenerationPriorityExtraKey] = 0
	_, configured = account.OpenAIImageGenerationRoutingPriority("gpt-5.4")
	require.False(t, configured)
}

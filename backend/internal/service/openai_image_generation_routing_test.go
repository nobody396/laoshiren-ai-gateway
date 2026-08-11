//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
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

func TestIsOpenAICodexSemanticImageGenerationIntent(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "chinese direct generation string input",
			body: `{"model":"gpt-5.6","input":"帮我生成一张图片：一只在月球上的橘猫"}`,
			want: true,
		},
		{
			name: "english direct generation",
			body: `{"model":"gpt-5.6-sol","input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"Create a cinematic poster of Shanghai at night"}]}]}`,
			want: true,
		},
		{
			name: "responses lite additional tools before latest user",
			body: `{"model":"gpt-5.6","input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"image_gen"}]},{"type":"message","role":"user","content":"给我画一张海边日落图"}]}`,
			want: true,
		},
		{
			name: "edit attached image",
			body: `{"model":"gpt-5.6","input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,AA=="},{"type":"input_text","text":"把它改成水彩风格"}]}]}`,
			want: true,
		},
		{
			name: "ordinary coding request",
			body: `{"model":"gpt-5.6","input":"帮我修复这段 Go 代码"}`,
			want: false,
		},
		{
			name: "image analysis is not generation",
			body: `{"model":"gpt-5.6","input":"分析这张图片里有什么"}`,
			want: false,
		},
		{
			name: "negative request",
			body: `{"model":"gpt-5.6","input":"先不要生成图片，只帮我完善提示词"}`,
			want: false,
		},
		{
			name: "how to question",
			body: `{"model":"gpt-5.6","input":"如何生成一张图片？请解释 API"}`,
			want: false,
		},
		{
			name: "prompt writing request",
			body: `{"model":"gpt-5.6","input":"帮我写一个生成图片的提示词"}`,
			want: false,
		},
		{
			name: "tool continuation does not replay previous user intent",
			body: `{"model":"gpt-5.6","input":[{"type":"message","role":"user","content":"帮我生成一张图片"},{"type":"function_call_output","call_id":"call_1","output":"done"}]}`,
			want: false,
		},
		{
			name: "latest user wins over old image request",
			body: `{"model":"gpt-5.6","input":[{"type":"message","role":"user","content":"帮我生成一张图片"},{"type":"message","role":"assistant","content":"done"},{"type":"message","role":"user","content":"谢谢"}]}`,
			want: false,
		},
		{
			name: "passive catalog alone",
			body: `{"model":"gpt-5.6","tools":[{"type":"namespace","name":"image_gen"}],"input":"hello"}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsOpenAICodexSemanticImageGenerationIntent([]byte(tt.body)))
		})
	}
}

func TestPrepareOpenAICodexImageGenerationRequest(t *testing.T) {
	t.Run("preserves tools and automatic model choice", func(t *testing.T) {
		body := []byte(`{
			"model":"gpt-5.6",
			"input":[{"type":"message","role":"user","content":"帮我生成一张图片"}],
			"tools":[{"type":"custom","name":"exec"},{"type":"namespace","name":"image_gen"}],
			"tool_choice":"auto",
			"stream":true
		}`)

		prepared, activated, err := PrepareOpenAICodexImageGenerationRequest(body)

		require.NoError(t, err)
		require.True(t, activated)
		require.True(t, gjson.GetBytes(prepared, `tools.#(type=="image_generation")`).Exists())
		require.True(t, gjson.GetBytes(prepared, `tools.#(type=="custom")`).Exists())
		require.True(t, gjson.GetBytes(prepared, `tools.#(type=="namespace")`).Exists())
		require.Equal(t, "auto", gjson.GetBytes(prepared, "tool_choice").String())
		require.Equal(t, "gpt-5.6", gjson.GetBytes(prepared, "model").String())
		require.True(t, gjson.GetBytes(prepared, "stream").Bool())
	})

	t.Run("does not duplicate native image tool", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5.6","input":"生成一张图片","tools":[{"type":"image_generation"}],"tool_choice":"auto"}`)

		prepared, activated, err := PrepareOpenAICodexImageGenerationRequest(body)

		require.NoError(t, err)
		require.True(t, activated)
		require.Len(t, gjson.GetBytes(prepared, `tools.#(type=="image_generation")`).Array(), 1)
		require.Equal(t, "auto", gjson.GetBytes(prepared, "tool_choice").String())
	})

	t.Run("leaves omitted tool choice to Responses default auto", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5.6","input":"生成一张图片"}`)

		prepared, activated, err := PrepareOpenAICodexImageGenerationRequest(body)

		require.NoError(t, err)
		require.True(t, activated)
		require.True(t, gjson.GetBytes(prepared, `tools.#(type=="image_generation")`).Exists())
		require.False(t, gjson.GetBytes(prepared, "tool_choice").Exists())
	})

	t.Run("respects client forced non image choice", func(t *testing.T) {
		body := []byte(`{"model":"gpt-5.6","input":"生成一张图片","tools":[{"type":"function","name":"save_file"}],"tool_choice":{"type":"function","name":"save_file"}}`)

		prepared, activated, err := PrepareOpenAICodexImageGenerationRequest(body)

		require.NoError(t, err)
		require.False(t, activated)
		require.Equal(t, string(body), string(prepared))
	})

	t.Run("rejects malformed tools field", func(t *testing.T) {
		_, activated, err := PrepareOpenAICodexImageGenerationRequest([]byte(`{"model":"gpt-5.6","input":"生成一张图片","tools":{}}`))

		require.ErrorContains(t, err, "tools must be an array")
		require.False(t, activated)
	})
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

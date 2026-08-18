//go:build unit

package service

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func TestIsExplicitOpenAIImageGenerationIntent(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "native image tool",
			body: `{"model":"gpt-5.4","tools":[{"type":"image_generation"}],"input":"draw an image"}`,
			want: true,
		},
		{
			name: "passive native image tool catalog",
			body: `{"model":"gpt-5.4","tools":[{"type":"image_generation"}],"tool_choice":"auto","input":"explain this code"}`,
			want: false,
		},
		{
			name: "forced non image tool fails closed",
			body: `{"model":"gpt-5.4","tools":[{"type":"image_generation"},{"type":"function","name":"save_file"}],"tool_choice":{"type":"function","name":"save_file"},"input":"generate an image"}`,
			want: false,
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
			name: "natural chinese landscape image request",
			body: `{"model":"gpt-5.6-sol","input":"给我生成一张雪山的风景图。"}`,
			want: true,
		},
		{
			name: "project file generation stays in native agent workflow",
			body: `{"model":"gpt-5.6-sol","input":"生成一张雪山风景图并保存到项目素材目录"}`,
			want: false,
		},
		{
			name: "batch generation stays in native agent workflow",
			body: `{"model":"gpt-5.6-sol","input":"批量生成多张产品图"}`,
			want: false,
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
			name: "edit attached image stays in native agent workflow",
			body: `{"model":"gpt-5.6","input":[{"type":"message","role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,AA=="},{"type":"input_text","text":"把它改成水彩风格"}]}]}`,
			want: false,
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
			name: "embedded chinese how to question",
			body: `{"model":"gpt-5.6","input":"请告诉我如何生成图片"}`,
			want: false,
		},
		{
			name: "english how to draw question",
			body: `{"model":"gpt-5.6","input":"How to draw an image?"}`,
			want: false,
		},
		{
			name: "embedded english how to question",
			body: `{"model":"gpt-5.6","input":"Can you explain how to generate an image?"}`,
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

func TestHasOpenAICodexExecImageRenderTool(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "codex freeform exec",
			body: `{"tools":[{"type":"custom","name":"exec","description":"Run JavaScript"}]}`,
			want: true,
		},
		{
			name: "case insensitive",
			body: `{"tools":[{"type":"CUSTOM","name":"EXEC"}]}`,
			want: true,
		},
		{
			name: "responses lite nested exec",
			body: `{"input":[{"type":"additional_tools","tools":[{"type":"custom","name":"exec"}]},{"type":"message","role":"user","content":"draw"}]}`,
			want: true,
		},
		{
			name: "function named exec is not the code mode host",
			body: `{"tools":[{"type":"function","name":"exec"}]}`,
		},
		{
			name: "native image tool only",
			body: `{"tools":[{"type":"image_generation"}]}`,
		},
		{name: "invalid json", body: `{`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, HasOpenAICodexExecImageRenderTool([]byte(tt.body)))
		})
	}
}

func TestHasOpenAICodexAdditionalToolsEnvelope(t *testing.T) {
	require.True(t, HasOpenAICodexAdditionalToolsEnvelope([]byte(`{
		"input":[
			{"type":"additional_tools","tools":[{"type":"namespace","name":"functions"}]},
			{"type":"message","role":"user","content":"draw a cat"}
		]
	}`)))
	require.False(t, HasOpenAICodexAdditionalToolsEnvelope([]byte(`{"input":"draw a cat"}`)))
	require.False(t, HasOpenAICodexAdditionalToolsEnvelope([]byte(`{"input":[{"type":"additional_tools","tools":{}}]}`)))
	require.False(t, HasOpenAICodexAdditionalToolsEnvelope([]byte(`{`)))
}

const codexGeneratedImageAckTestSecret = "unit-test-only-codex-image-ack-signing-secret"

func signedCodexGeneratedImageContinuationBody(t *testing.T, images []codexExecRenderedImage) []byte {
	t.Helper()
	execInput, err := codexExecGeneratedImageInput(images)
	require.NoError(t, err)
	dataURLs, ok := parseCodexExecGeneratedImageInput(execInput)
	require.True(t, ok)
	callID, err := newCodexGeneratedImageAckCallID(codexGeneratedImageAckTestSecret, dataURLs)
	require.NoError(t, err)
	output := []any{map[string]any{"type": "input_text", "text": "Script completed"}}
	for index, dataURL := range dataURLs {
		output = append(output,
			map[string]any{"type": "input_image", "image_url": dataURL, "detail": "high"},
			map[string]any{"type": "input_text", "text": "已生成第 " + fmt.Sprint(index+1) + " 张图片。"},
		)
	}
	body, err := json.Marshal(map[string]any{
		"model": "gpt-5.6-sol",
		"input": []any{
			map[string]any{"type": "additional_tools", "tools": []any{map[string]any{"type": "namespace", "name": "functions"}}},
			map[string]any{"type": "message", "role": "user", "content": "生成图片"},
			map[string]any{"type": "custom_tool_call", "id": "ctc_image", "call_id": callID, "name": "exec", "status": "completed", "input": execInput},
			map[string]any{"type": "custom_tool_call_output", "call_id": callID, "output": output},
		},
		"stream": true,
	})
	require.NoError(t, err)
	return body
}

func TestIsOpenAICodexGeneratedImageToolContinuation(t *testing.T) {
	single := signedCodexGeneratedImageContinuationBody(t, []codexExecRenderedImage{{
		MediaType: "image/png",
		B64JSON:   codexBridgeTestPNG,
	}})
	require.True(t, IsOpenAICodexGeneratedImageToolContinuation(single, codexGeneratedImageAckTestSecret))
	require.True(t, IsOpenAICodexGeneratedImageToolContinuation(single, codexGeneratedImageAckTestSecret), "replay is a zero-side-effect local acknowledgement")

	multiple := signedCodexGeneratedImageContinuationBody(t, []codexExecRenderedImage{
		{MediaType: "image/png", B64JSON: codexBridgeTestPNG},
		{MediaType: "image/webp", B64JSON: openAIImagesTestWebP},
	})
	require.True(t, IsOpenAICodexGeneratedImageToolContinuation(multiple, codexGeneratedImageAckTestSecret))
	require.False(t, IsOpenAICodexGeneratedImageToolContinuation(single, "wrong-secret"))

	t.Run("unrelated completed tool history may precede the signed terminal pair", func(t *testing.T) {
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(single, &decoded))
		inputs := decoded["input"].([]any)
		inputs = append(inputs[:2], append([]any{
			map[string]any{"type": "custom_tool_call", "call_id": "call_history", "name": "exec", "status": "completed", "input": "text(true);"},
			map[string]any{"type": "custom_tool_call_output", "call_id": "call_history", "output": []any{map[string]any{"type": "input_text", "text": "ok"}}},
		}, inputs[2:]...)...)
		decoded["input"] = inputs
		body, err := json.Marshal(decoded)
		require.NoError(t, err)
		require.True(t, IsOpenAICodexGeneratedImageToolContinuation(body, codexGeneratedImageAckTestSecret))
	})

	t.Run("tampered call id", func(t *testing.T) {
		body, err := sjson.SetBytes(single, "input.2.call_id", "call_img_0000000000000000_00000000000000000000000000000000")
		require.NoError(t, err)
		body, err = sjson.SetBytes(body, "input.3.call_id", "call_img_0000000000000000_00000000000000000000000000000000")
		require.NoError(t, err)
		require.False(t, IsOpenAICodexGeneratedImageToolContinuation(body, codexGeneratedImageAckTestSecret))
	})

	t.Run("mismatched output call id", func(t *testing.T) {
		body, err := sjson.SetBytes(single, "input.3.call_id", "call_other")
		require.NoError(t, err)
		require.False(t, IsOpenAICodexGeneratedImageToolContinuation(body, codexGeneratedImageAckTestSecret))
	})

	t.Run("mismatched output image", func(t *testing.T) {
		body, err := sjson.SetBytes(single, "input.3.output.1.image_url", "data:image/webp;base64,"+openAIImagesTestWebP)
		require.NoError(t, err)
		require.False(t, IsOpenAICodexGeneratedImageToolContinuation(body, codexGeneratedImageAckTestSecret))
	})

	t.Run("missing image output", func(t *testing.T) {
		body, err := sjson.SetBytes(single, "input.3.output", []any{map[string]any{"type": "input_text", "text": "done"}})
		require.NoError(t, err)
		require.False(t, IsOpenAICodexGeneratedImageToolContinuation(body, codexGeneratedImageAckTestSecret))
	})

	t.Run("call and output must be adjacent and terminal", func(t *testing.T) {
		body, err := sjson.SetBytes(single, "input.3", map[string]any{"type": "message", "role": "user", "content": "new turn"})
		require.NoError(t, err)
		require.False(t, IsOpenAICodexGeneratedImageToolContinuation(body, codexGeneratedImageAckTestSecret))
	})

	t.Run("arbitrary exec is not acknowledged", func(t *testing.T) {
		body, err := sjson.SetBytes(single, "input.2.input", `generatedImage({image_url:"https://example.test/image.png",output_hint:"done"});`)
		require.NoError(t, err)
		require.False(t, IsOpenAICodexGeneratedImageToolContinuation(body, codexGeneratedImageAckTestSecret))
	})

	t.Run("unrelated custom tool after generated image is not swallowed", func(t *testing.T) {
		var decoded map[string]any
		require.NoError(t, json.Unmarshal(single, &decoded))
		inputs := decoded["input"].([]any)
		inputs = append(inputs,
			map[string]any{"type": "custom_tool_call", "call_id": "call_other", "name": "exec", "status": "completed", "input": "text(true);"},
			map[string]any{"type": "custom_tool_call_output", "call_id": "call_other", "output": []any{map[string]any{"type": "input_text", "text": "ok"}}},
		)
		decoded["input"] = inputs
		body, err := json.Marshal(decoded)
		require.NoError(t, err)
		require.False(t, IsOpenAICodexGeneratedImageToolContinuation(body, codexGeneratedImageAckTestSecret))
	})
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
		require.True(t, HasOpenAICodexExecImageRenderTool(prepared))
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

func TestForceOpenAICodexImageGenerationToolChoice(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-terra","input":"帮我生成一张图片","tools":[{"type":"custom","name":"exec"}],"tool_choice":"auto","stream":true}`)

	forced, err := ForceOpenAICodexImageGenerationToolChoice(body)

	require.NoError(t, err)
	require.Equal(t, "image_generation", gjson.GetBytes(forced, "tool_choice.type").String())
	require.True(t, gjson.GetBytes(forced, `tools.#(type=="image_generation")`).Exists())
	require.True(t, gjson.GetBytes(forced, `tools.#(type=="custom")`).Exists())
	require.True(t, gjson.GetBytes(forced, "stream").Bool())
}

func TestShouldUseFixedOpenAIImageRenderer(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "luna image intent is detectable but must be rejected by the handler retirement gate",
			body: `{"model":"gpt-5.6-luna","input":"帮我生成一张月球橘猫的图片"}`,
			want: true,
		},
		{
			name: "retired luna ordinary text does not enter the image renderer",
			body: `{"model":"gpt-5.6-luna","input":"explain this code"}`,
		},
		{
			name: "forced hosted image tool",
			body: `{"model":"gpt-5.6-sol","input":"a watercolor city","tools":[{"type":"image_generation"}],"tool_choice":{"type":"image_generation"}}`,
			want: true,
		},
		{
			name: "passive hosted tool catalog",
			body: `{"model":"gpt-5.6-sol","input":"explain this code","tools":[{"type":"image_generation"}],"tool_choice":"auto"}`,
		},
		{
			name: "forced non image tool",
			body: `{"model":"gpt-5.6-sol","input":"generate an image","tools":[{"type":"image_generation"},{"type":"function","name":"save_file"}],"tool_choice":{"type":"function","name":"save_file"}}`,
		},
		{
			name: "attached image edit stays on responses path",
			body: `{"model":"gpt-5.6-sol","input":[{"role":"user","content":[{"type":"input_image","image_url":"data:image/png;base64,AA=="},{"type":"input_text","text":"把这张图改成水彩风格"}]}]}`,
		},
		{
			name: "negative request",
			body: `{"model":"gpt-5.6-sol","input":"先不要生成图片，只解释原理"}`,
		},
		{
			name: "how to request",
			body: `{"model":"gpt-5.6-sol","input":"如何生成一张图片？"}`,
		},
		{
			name: "edit image generation implementation instead of generating",
			body: `{"model":"gpt-5.6-sol","input":"帮我修改这个生成图片的函数，让错误更清楚"}`,
		},
		{
			name: "english image generation code request",
			body: `{"model":"gpt-5.6-sol","input":"Please edit the image generation code to handle retries"}`,
		},
		{
			name: "direct chinese logo request remains generation",
			body: `{"model":"gpt-5.6-sol","input":"生成一个极简风格的 logo"}`,
			want: true,
		},
		{
			name: "direct english image request remains generation",
			body: `{"model":"gpt-5.6-sol","input":"Create an image of a moonlit orange cat"}`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ShouldUseFixedOpenAIImageRenderer([]byte(tt.body)))
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

func TestAccountOpenAIImageGenerationTransportRouting(t *testing.T) {
	t.Run("Responses bridge accepts exact Codex image alias", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Extra: map[string]any{
				OpenAIImageGenerationPriorityExtraKey: 1,
				OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
			},
		}

		priority, configured := account.OpenAIImageGenerationRoutingPriority("gpt-image-2")
		require.True(t, configured)
		require.Equal(t, 1, priority)
		transport, configured := account.OpenAIImageGenerationTransport("gpt-image-2")
		require.True(t, configured)
		require.Equal(t, OpenAIImageGenerationTransportResponses, transport)
	})

	t.Run("native Images route requires explicit model mapping", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Type:     AccountTypeAPIKey,
			Credentials: map[string]any{
				"model_mapping": map[string]any{"gpt-image-2": "gpt-image-2-count"},
			},
			Extra: map[string]any{
				"supports_images":                      true,
				OpenAIImageGenerationPriorityExtraKey:  2,
				OpenAIImageGenerationModelsExtraKey:    []any{"gpt-image-2"},
				OpenAIImageGenerationTransportExtraKey: OpenAIImageGenerationTransportImages,
			},
		}

		priority, configured := account.OpenAIImageGenerationRoutingPriority("gpt-image-2")
		require.True(t, configured)
		require.Equal(t, 2, priority)
		transport, configured := account.OpenAIImageGenerationTransport("gpt-image-2")
		require.True(t, configured)
		require.Equal(t, OpenAIImageGenerationTransportImages, transport)

		delete(account.Credentials, "model_mapping")
		_, configured = account.OpenAIImageGenerationRoutingPriority("gpt-image-2")
		require.False(t, configured, "native image route must fail closed without its explicit public-to-upstream mapping")
	})

	t.Run("invalid transport fails closed", func(t *testing.T) {
		account := &Account{
			Platform: PlatformOpenAI,
			Extra: map[string]any{
				OpenAIImageGenerationPriorityExtraKey:  1,
				OpenAIImageGenerationTransportExtraKey: "unknown",
			},
		}
		_, configured := account.OpenAIImageGenerationRoutingPriority("gpt-image-2")
		require.False(t, configured)
	})
}

func TestAccountOmitOpenAIImageGenerationResponseFormat(t *testing.T) {
	account := &Account{Extra: map[string]any{
		OpenAIImageGenerationOmitResponseFormatExtraKey: true,
	}}
	require.True(t, account.OmitOpenAIImageGenerationResponseFormat())

	account.Extra[OpenAIImageGenerationOmitResponseFormatExtraKey] = "true"
	require.False(t, account.OmitOpenAIImageGenerationResponseFormat(), "invalid config must fail closed")
	require.False(t, (*Account)(nil).OmitOpenAIImageGenerationResponseFormat())
}

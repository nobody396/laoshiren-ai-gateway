package openai_compat

import "testing"

func TestResolveResponsesSupport(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string]any
		want  AccountResponsesSupport
	}{
		{"nil extra", nil, ResponsesSupportUnknown},
		{"empty extra", map[string]any{}, ResponsesSupportUnknown},
		{"key missing", map[string]any{"other": "value"}, ResponsesSupportUnknown},
		{"value true", map[string]any{ExtraKeyResponsesSupported: true}, ResponsesSupportYes},
		{"value false", map[string]any{ExtraKeyResponsesSupported: false}, ResponsesSupportNo},
		{"wrong type", map[string]any{ExtraKeyResponsesSupported: "true"}, ResponsesSupportUnknown},
		{"force responses", map[string]any{ExtraKeyResponsesMode: "force_responses", ExtraKeyResponsesSupported: false}, ResponsesSupportYes},
		{"force chat", map[string]any{ExtraKeyResponsesMode: "force_chat_completions", ExtraKeyResponsesSupported: true}, ResponsesSupportNo},
		{"invalid mode uses probe", map[string]any{ExtraKeyResponsesMode: "invalid", ExtraKeyResponsesSupported: false}, ResponsesSupportNo},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveResponsesSupport(tc.extra); got != tc.want {
				t.Fatalf("ResolveResponsesSupport() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldUseResponsesAPI(t *testing.T) {
	tests := []struct {
		name  string
		extra map[string]any
		want  bool
	}{
		{"unknown defaults to responses", nil, true},
		{"wrong type defaults to responses", map[string]any{ExtraKeyResponsesSupported: "yes"}, true},
		{"supported", map[string]any{ExtraKeyResponsesSupported: true}, true},
		{"unsupported", map[string]any{ExtraKeyResponsesSupported: false}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldUseResponsesAPI(tc.extra); got != tc.want {
				t.Fatalf("ShouldUseResponsesAPI() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldUseResponsesAPIForModel(t *testing.T) {
	extra := map[string]any{
		ExtraKeyResponsesSupported: true,
		ExtraKeyUpstreamProtocolByModel: map[string]any{
			"ZHIPU/GLM-5.3":      "chat_completions",
			"MiniMax/MiniMax-M3": "chat",
			"glm-5.2":            "responses",
		},
	}

	if ShouldUseResponsesAPIForModel(extra, "zhipu/glm-5.3") {
		t.Fatal("GLM-5.3 should use chat_completions")
	}
	if ShouldUseResponsesAPIForModel(extra, "public-alias", "MINIMAX/MINIMAX-M3") {
		t.Fatal("mapped MiniMax model should use chat_completions")
	}
	if !ShouldUseResponsesAPIForModel(extra, "glm-5.2") {
		t.Fatal("glm-5.2 should use responses")
	}
	if !ShouldUseResponsesAPIForModel(extra, "qwen3.8-max") {
		t.Fatal("unconfigured model should preserve the account-wide default")
	}
}

func TestPerModelProtocolOverridesLegacyAccountFlag(t *testing.T) {
	extra := map[string]any{
		ExtraKeyResponsesSupported: false,
		ExtraKeyUpstreamProtocolByModel: map[string]string{
			"glm-5.2": "responses",
		},
	}
	if !ShouldUseResponsesAPIForModel(extra, "glm-5.2") {
		t.Fatal("explicit model override must win over account-wide fallback")
	}
	if ShouldUseResponsesAPIForModel(extra, "glm-5.3") {
		t.Fatal("unconfigured model should use the account-wide chat fallback")
	}
}

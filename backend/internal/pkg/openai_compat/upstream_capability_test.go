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

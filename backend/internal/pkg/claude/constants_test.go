package claude

import "testing"

func TestDefaultModelsContainsClaudeOpus48(t *testing.T) {
	t.Parallel()

	for _, model := range DefaultModels {
		if model.ID == "claude-opus-4-8" {
			if model.DisplayName != "Claude Opus 4.8" {
				t.Fatalf("unexpected display name: got %q", model.DisplayName)
			}
			return
		}
	}

	t.Fatal("expected claude-opus-4-8 in DefaultModels")
}

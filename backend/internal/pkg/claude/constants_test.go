package claude

import "testing"

func TestDefaultModelsExcludesSuspendedFable5AndContainsOpus48(t *testing.T) {
	t.Parallel()

	foundCurrent := false
	for _, model := range DefaultModels {
		if model.ID == "claude-fable-5" {
			t.Fatal("did not expect suspended claude-fable-5 in DefaultModels")
		}
		if model.ID == "claude-opus-4-8" {
			foundCurrent = true
			if model.DisplayName != "Claude Opus 4.8" {
				t.Fatalf("unexpected display name: got %q", model.DisplayName)
			}
		}
	}

	if !foundCurrent {
		t.Fatal("expected claude-opus-4-8 in DefaultModels")
	}
}

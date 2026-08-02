package claude

import "testing"

func TestDefaultModelsExcludesSuspendedFable5AndContainsOpus5(t *testing.T) {
	t.Parallel()

	foundCurrent := false
	for _, model := range DefaultModels {
		if model.ID == "claude-fable-5" {
			t.Fatal("did not expect suspended claude-fable-5 in DefaultModels")
		}
		if model.ID == "claude-opus-5" {
			foundCurrent = true
			if model.DisplayName != "Claude Opus 5" {
				t.Fatalf("unexpected display name: got %q", model.DisplayName)
			}
		}
	}

	if !foundCurrent {
		t.Fatal("expected claude-opus-5 in DefaultModels")
	}
}

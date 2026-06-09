package claude

import "testing"

func TestDefaultModelsContainsClaudeFable5AndOpus48(t *testing.T) {
	t.Parallel()

	foundFable := false
	foundCurrent := false
	for _, model := range DefaultModels {
		if model.ID == "claude-fable-5" {
			foundFable = true
			if model.DisplayName != "Claude Fable 5" {
				t.Fatalf("unexpected fable display name: got %q", model.DisplayName)
			}
		}
		if model.ID == "claude-opus-4-8" {
			foundCurrent = true
			if model.DisplayName != "Claude Opus 4.8" {
				t.Fatalf("unexpected display name: got %q", model.DisplayName)
			}
		}
	}

	if !foundFable {
		t.Fatal("expected claude-fable-5 in DefaultModels")
	}
	if !foundCurrent {
		t.Fatal("expected claude-opus-4-8 in DefaultModels")
	}
}

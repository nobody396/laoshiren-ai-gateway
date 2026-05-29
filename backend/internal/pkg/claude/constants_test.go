package claude

import "testing"

func TestDefaultModelsContainsClaudeOpus48(t *testing.T) {
	t.Parallel()

	foundLatest := false
	foundCurrent := false
	for _, model := range DefaultModels {
		if model.ID == "claude-opus-latest" {
			foundLatest = true
			if model.DisplayName != "Claude Opus Latest (current 4.8)" {
				t.Fatalf("unexpected latest display name: got %q", model.DisplayName)
			}
		}
		if model.ID == "claude-opus-4-8" {
			foundCurrent = true
			if model.DisplayName != "Claude Opus 4.8" {
				t.Fatalf("unexpected display name: got %q", model.DisplayName)
			}
		}
	}

	if !foundLatest {
		t.Fatal("expected claude-opus-latest in DefaultModels")
	}
	if !foundCurrent {
		t.Fatal("expected claude-opus-4-8 in DefaultModels")
	}
}

func TestNormalizeModelIDResolvesClaudeOpusLatest(t *testing.T) {
	t.Parallel()

	if got := NormalizeModelID("claude-opus-latest"); got != "claude-opus-4-8" {
		t.Fatalf("NormalizeModelID(latest) = %q, want claude-opus-4-8", got)
	}
}

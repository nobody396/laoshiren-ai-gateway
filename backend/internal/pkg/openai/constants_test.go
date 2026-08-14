package openai

import "testing"

func TestDefaultModelsExcludeUnsupportedModels(t *testing.T) {
	for _, model := range DefaultModels {
		switch model.ID {
		case "gpt-5.3-codex-spark", "gpt-5.6-luna", "gpt-5.4-mini":
			t.Fatalf("unsupported %s must not be advertised", model.ID)
		}
	}
}

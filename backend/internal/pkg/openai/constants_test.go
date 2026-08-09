package openai

import "testing"

func TestDefaultModelsExcludeUnsupportedCodexSpark(t *testing.T) {
	for _, model := range DefaultModels {
		if model.ID == "gpt-5.3-codex-spark" {
			t.Fatal("unsupported gpt-5.3-codex-spark must not be advertised")
		}
	}
}

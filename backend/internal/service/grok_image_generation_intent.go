package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

// IsExplicitGrokImageGenerationIntent detects hard image-generation signals
// without treating Codex's passive image_gen namespace catalog as a request.
func IsExplicitGrokImageGenerationIntent(model string, body []byte) bool {
	if isGrokImageGenerationModel(model) {
		return true
	}
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	if tools := gjson.GetBytes(body, "tools"); tools.IsArray() {
		found := false
		tools.ForEach(func(_, item gjson.Result) bool {
			found = strings.TrimSpace(item.Get("type").String()) == "image_generation"
			return !found
		})
		if found {
			return true
		}
	}
	return grokToolChoiceSelectsImageGeneration(gjson.GetBytes(body, "tool_choice"))
}

func grokToolChoiceSelectsImageGeneration(choice gjson.Result) bool {
	if !choice.Exists() {
		return false
	}
	if choice.Type == gjson.String {
		return strings.TrimSpace(choice.String()) == "image_generation"
	}
	if !choice.IsObject() {
		return false
	}
	choiceType := strings.TrimSpace(choice.Get("type").String())
	if choiceType == "image_generation" {
		return true
	}
	if choiceType == "namespace" && isGrokImageGenNamespace(
		choice.Get("name").String(),
		choice.Get("namespace").String(),
	) {
		return true
	}
	if tool := choice.Get("tool"); tool.IsObject() && grokToolChoiceSelectsImageGeneration(tool) {
		return true
	}
	if isGrokImageGenFunctionReference(choice.Get("namespace").String(), choice.Get("name").String()) {
		return true
	}
	if fn := choice.Get("function"); fn.IsObject() {
		return isGrokImageGenFunctionReference(fn.Get("namespace").String(), fn.Get("name").String())
	}
	return isGrokImageGenFunctionReference("", choice.Get("function.name").String())
}

func isGrokImageGenNamespace(values ...string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "image_gen" {
			return true
		}
	}
	return false
}

func isGrokImageGenFunctionReference(namespace, name string) bool {
	namespace = strings.TrimSpace(namespace)
	name = strings.TrimSpace(name)
	if namespace == "image_gen" && name == "imagegen" {
		return true
	}
	return name == "image_gen.imagegen" || name == "image_gen__imagegen"
}

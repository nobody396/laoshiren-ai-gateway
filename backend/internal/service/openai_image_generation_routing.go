package service

import (
	"strings"

	"github.com/tidwall/gjson"
)

const (
	// OpenAIImageGenerationPriorityExtraKey opts an OpenAI account into
	// Responses API image-generation routing. Lower positive values win.
	OpenAIImageGenerationPriorityExtraKey = "openai_image_generation_priority"
	// OpenAIImageGenerationModelsExtraKey optionally restricts that preference
	// to public request models. Entries support the same trailing-* wildcard as
	// account model mappings. A present but empty/invalid list fails closed.
	OpenAIImageGenerationModelsExtraKey = "openai_image_generation_models"
)

// IsExplicitOpenAIImageGenerationIntent detects only native Responses API
// image-generation signals. Passive tool catalogs (for example an image_gen
// namespace with tool_choice=auto) deliberately do not activate this route.
func IsExplicitOpenAIImageGenerationIntent(body []byte) bool {
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
	return openAIToolChoiceSelectsImageGeneration(gjson.GetBytes(body, "tool_choice"))
}

func openAIToolChoiceSelectsImageGeneration(choice gjson.Result) bool {
	if !choice.Exists() {
		return false
	}
	if choice.Type == gjson.String {
		return strings.TrimSpace(choice.String()) == "image_generation"
	}
	if !choice.IsObject() {
		return false
	}
	if strings.TrimSpace(choice.Get("type").String()) == "image_generation" {
		return true
	}
	if tool := choice.Get("tool"); tool.IsObject() {
		return openAIToolChoiceSelectsImageGeneration(tool)
	}
	return false
}

// OpenAIImageGenerationRoutingPriority returns an account's opt-in image route
// priority for the public requested model. Invalid configuration fails closed
// so an accidental extra value cannot redirect production traffic.
func (a *Account) OpenAIImageGenerationRoutingPriority(requestedModel string) (int, bool) {
	if a == nil || !a.IsOpenAI() || a.Extra == nil {
		return 0, false
	}
	rawPriority, ok := a.Extra[OpenAIImageGenerationPriorityExtraKey]
	if !ok {
		return 0, false
	}
	priority := ParseExtraInt(rawPriority)
	if priority <= 0 {
		return 0, false
	}

	rawModels, restricted := a.Extra[OpenAIImageGenerationModelsExtraKey]
	if restricted && !openAIImageGenerationModelAllowed(rawModels, requestedModel) {
		return 0, false
	}
	return priority, true
}

func openAIImageGenerationModelAllowed(raw any, requestedModel string) bool {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return false
	}

	matches := func(pattern string) bool {
		pattern = strings.TrimSpace(pattern)
		return pattern != "" && matchWildcard(pattern, requestedModel)
	}

	switch values := raw.(type) {
	case []string:
		for _, value := range values {
			if matches(value) {
				return true
			}
		}
	case []any:
		for _, value := range values {
			pattern, ok := value.(string)
			if ok && matches(pattern) {
				return true
			}
		}
	}
	return false
}

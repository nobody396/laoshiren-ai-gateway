package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	maxReasoningEffortMappings   = 64
	maxReasoningEffortValueLen   = 64
	reasoningEffortDefaultSource = "default"
)

var openAIReasoningEffortValues = []string{"minimal", "low", "medium", "high", "xhigh", "max"}

// clientReasoningEffortAliases maps client-only effort tiers onto the wire
// values the upstream Responses API accepts. Codex exposes "ultra" as a
// product tier (maximum reasoning with automatic task delegation) rather than a
// Responses wire value, and upstream rejects it with 400. Rewriting it to "max"
// keeps the highest reasoning budget the API actually offers instead of failing
// the request. Group ceilings and mappings still apply to the rewritten value.
var clientReasoningEffortAliases = map[string]string{"ultra": "max"}

// normalizeClientReasoningEffortAlias resolves a client-only tier to its wire
// value. The second result reports whether raw was such an alias.
func normalizeClientReasoningEffortAlias(raw string) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.NewReplacer("-", "", "_", "", " ", "").Replace(value)
	mapped, ok := clientReasoningEffortAliases[value]
	return mapped, ok
}

// bodyHasClientReasoningEffortAlias reports whether the request carries an
// effort value that only the client vocabulary knows, so a group without a
// ceiling or mappings still gets the value rewritten.
func bodyHasClientReasoningEffortAlias(body []byte) bool {
	for _, path := range []string{"reasoning.effort", "reasoning_effort"} {
		field := gjson.GetBytes(body, path)
		if !field.Exists() || field.Type != gjson.String {
			continue
		}
		if _, ok := normalizeClientReasoningEffortAlias(field.String()); ok {
			return true
		}
	}
	return false
}

type openAIReasoningEffortPolicyContextKey struct{}

type openAIReasoningEffortPolicy struct {
	maxEffort string
	mappings  []ReasoningEffortMapping
}

// NormalizeMaxReasoningEffort validates and canonicalizes a group policy value.
// Empty means that the group does not impose a ceiling.
func NormalizeMaxReasoningEffort(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	value = strings.NewReplacer("-", "", "_", "", " ", "").Replace(value)
	switch value {
	case "":
		return ""
	case "minimal":
		return "minimal"
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	case "xhigh", "extrahigh":
		return "xhigh"
	case "max":
		return "max"
	default:
		return ""
	}
}

func reasoningEffortValuesForPlatform(platform string) []string {
	if platform != PlatformOpenAI {
		return nil
	}
	return openAIReasoningEffortValues
}

func normalizeMaxReasoningEffortForPlatform(platform, raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}

	allowedValues := reasoningEffortValuesForPlatform(platform)
	if len(allowedValues) == 0 {
		return "", fmt.Errorf(
			"reasoning effort policy is only supported for platform %q",
			PlatformOpenAI,
		)
	}

	value := NormalizeMaxReasoningEffort(raw)
	for _, allowed := range allowedValues {
		if value == allowed {
			return value, nil
		}
	}
	return "", fmt.Errorf(
		"reasoning effort %q is not supported for platform %q; allowed values: %s",
		raw,
		platform,
		strings.Join(allowedValues, ", "),
	)
}

func reasoningEffortRank(raw string) (int, bool) {
	switch NormalizeMaxReasoningEffort(raw) {
	case "minimal":
		return 1, true
	case "low":
		return 2, true
	case "medium":
		return 3, true
	case "high":
		return 4, true
	case "xhigh":
		return 5, true
	case "max":
		return 6, true
	default:
		return 0, false
	}
}

func normalizeReasoningEffortMappingSource(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == reasoningEffortDefaultSource {
		return reasoningEffortDefaultSource
	}
	return NormalizeMaxReasoningEffort(raw)
}

// NormalizeReasoningEffortMappings validates group mapping rules against the
// fixed effort values supported by OpenAI routes.
func NormalizeReasoningEffortMappings(platform string, raw []ReasoningEffortMapping) ([]ReasoningEffortMapping, error) {
	if len(raw) > maxReasoningEffortMappings {
		return nil, fmt.Errorf("reasoning effort mappings cannot exceed %d entries", maxReasoningEffortMappings)
	}

	normalized := make([]ReasoningEffortMapping, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for i, mapping := range raw {
		from := normalizeReasoningEffortMappingSource(mapping.From)
		to := NormalizeMaxReasoningEffort(mapping.To)
		if from == "" {
			return nil, fmt.Errorf("reasoning effort mapping %d source contains an empty or unknown value", i+1)
		}
		if to == "" {
			return nil, fmt.Errorf("reasoning effort mapping %d target contains an empty or unknown value", i+1)
		}
		if len(from) > maxReasoningEffortValueLen || len(to) > maxReasoningEffortValueLen {
			return nil, fmt.Errorf("reasoning effort mapping %d values cannot exceed %d characters", i+1, maxReasoningEffortValueLen)
		}
		if from == reasoningEffortDefaultSource {
			if platform != PlatformOpenAI {
				return nil, fmt.Errorf(
					"reasoning effort mapping %d source: default reasoning effort is only supported for platform %q",
					i+1,
					PlatformOpenAI,
				)
			}
		} else {
			if _, err := normalizeMaxReasoningEffortForPlatform(platform, from); err != nil {
				return nil, fmt.Errorf("reasoning effort mapping %d source: %w", i+1, err)
			}
		}
		if _, err := normalizeMaxReasoningEffortForPlatform(platform, to); err != nil {
			return nil, fmt.Errorf("reasoning effort mapping %d target: %w", i+1, err)
		}
		key := from
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("duplicate reasoning effort mapping source %q", from)
		}
		seen[key] = struct{}{}
		normalized = append(normalized, ReasoningEffortMapping{From: from, To: to})
	}
	return normalized, nil
}

// WithOpenAIReasoningEffortPolicy binds a group policy to a request after its
// concrete target platform has been resolved to OpenAI. The policy is copied so
// retries and asynchronous forwarding cannot observe later slice mutations.
func WithOpenAIReasoningEffortPolicy(ctx context.Context, maxEffort string, mappings []ReasoningEffortMapping) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	policy := openAIReasoningEffortPolicy{
		maxEffort: maxEffort,
		mappings:  append([]ReasoningEffortMapping(nil), mappings...),
	}
	return context.WithValue(ctx, openAIReasoningEffortPolicyContextKey{}, policy)
}

// ApplyOpenAIReasoningEffortPolicyFromContext applies a policy previously bound
// to the request. An unbound request is returned byte-for-byte unchanged.
func ApplyOpenAIReasoningEffortPolicyFromContext(ctx context.Context, body []byte) ([]byte, bool) {
	if ctx == nil {
		return body, false
	}
	policy, ok := ctx.Value(openAIReasoningEffortPolicyContextKey{}).(openAIReasoningEffortPolicy)
	if !ok {
		return body, false
	}
	return ApplyOpenAIReasoningEffortPolicy(body, policy.maxEffort, policy.mappings)
}

func mapReasoningEffort(raw string, mappings []ReasoningEffortMapping) (string, bool) {
	value := strings.TrimSpace(raw)
	canonical := NormalizeMaxReasoningEffort(value)
	for _, mapping := range mappings {
		if normalizeReasoningEffortMappingSource(mapping.From) == reasoningEffortDefaultSource {
			continue
		}
		if canonical != "" && canonical == NormalizeMaxReasoningEffort(mapping.From) {
			return strings.TrimSpace(mapping.To), true
		}
	}
	return value, false
}

func defaultReasoningEffort(mappings []ReasoningEffortMapping) (string, bool) {
	for _, mapping := range mappings {
		if normalizeReasoningEffortMappingSource(mapping.From) == reasoningEffortDefaultSource {
			value := NormalizeMaxReasoningEffort(mapping.To)
			return value, value != ""
		}
	}
	return "", false
}

func requestHasExplicitReasoningChoice(body []byte) bool {
	for _, path := range []string{"reasoning.effort", "reasoning_effort"} {
		if gjson.GetBytes(body, path).Exists() {
			return true
		}
	}
	if thinkingType := gjson.GetBytes(body, "thinking.type"); thinkingType.Exists() {
		// An explicit enabled toggle still needs a default effort. Disabled and
		// unknown toggles are preserved byte-for-byte instead of being
		// contradicted or masking an upstream validation error.
		if thinkingType.Type != gjson.String || !strings.EqualFold(strings.TrimSpace(thinkingType.String()), "enabled") {
			return true
		}
	}
	model := strings.TrimSpace(gjson.GetBytes(body, "model").String())
	return deriveOpenAIReasoningEffortFromModel(model) != ""
}

func defaultReasoningEffortPath(body []byte) string {
	// Chat Completions owns a top-level messages array and a flat
	// reasoning_effort field. HTTP/WS Responses requests use reasoning.effort;
	// default to that shape so stateless response.create frames and valid
	// previous_response_id-only requests do not get the Chat field by mistake.
	if gjson.GetBytes(body, "messages").Exists() {
		return "reasoning_effort"
	}
	return "reasoning.effort"
}

func sanitizeGroupReasoningEffortPolicy(group *Group) {
	if group == nil {
		return
	}
	maxEffort, maxErr := normalizeMaxReasoningEffortForPlatform(group.Platform, group.MaxReasoningEffort)
	mappings, mappingsErr := NormalizeReasoningEffortMappings(group.Platform, group.ReasoningEffortMappings)
	if maxErr != nil {
		maxEffort = ""
	}
	if mappingsErr != nil {
		mappings = []ReasoningEffortMapping{}
	}
	group.MaxReasoningEffort = maxEffort
	group.ReasoningEffortMappings = mappings
}

// ApplyOpenAIReasoningEffortPolicy resolves client-only effort aliases, applies
// one exact mapping, and then caps known effort levels. A reserved
// {"from":"default"} mapping supplies an omitted value without overriding an
// explicit effort, disabled/invalid thinking toggle, or model-suffix effort.
func ApplyOpenAIReasoningEffortPolicy(body []byte, maxEffort string, mappings []ReasoningEffortMapping) ([]byte, bool) {
	maxRank, hasMax := reasoningEffortRank(maxEffort)
	if len(body) == 0 {
		return body, false
	}
	if !hasMax && len(mappings) == 0 && !bodyHasClientReasoningEffortAlias(body) {
		return body, false
	}

	result := body
	changed := false
	for _, path := range []string{"reasoning.effort", "reasoning_effort"} {
		field := gjson.GetBytes(result, path)
		if !field.Exists() || field.Type != gjson.String {
			continue
		}
		original := strings.TrimSpace(field.String())
		if original == "" {
			continue
		}

		resolved := original
		if aliased, isAlias := normalizeClientReasoningEffortAlias(resolved); isAlias {
			resolved = aliased
		}
		effective, _ := mapReasoningEffort(resolved, mappings)
		if currentRank, recognized := reasoningEffortRank(effective); recognized {
			effective = NormalizeMaxReasoningEffort(effective)
			if hasMax && currentRank > maxRank {
				effective = NormalizeMaxReasoningEffort(maxEffort)
			}
		}
		if effective == original {
			continue
		}

		updated, err := sjson.SetBytes(result, path, effective)
		if err != nil {
			continue
		}
		result = updated
		changed = true
	}

	if !requestHasExplicitReasoningChoice(body) {
		if effective, ok := defaultReasoningEffort(mappings); ok {
			if currentRank, recognized := reasoningEffortRank(effective); recognized && hasMax && currentRank > maxRank {
				effective = NormalizeMaxReasoningEffort(maxEffort)
			}
			updated, err := sjson.SetBytes(result, defaultReasoningEffortPath(body), effective)
			if err == nil {
				result = updated
				changed = true
			}
		}
	}
	return result, changed
}

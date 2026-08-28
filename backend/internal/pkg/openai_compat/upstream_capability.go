package openai_compat

import "strings"

// AccountResponsesSupport describes whether an OpenAI APIKey upstream supports
// the Responses API. It is meaningful only for platform=openai + type=apikey.
type AccountResponsesSupport int

type ResponsesSupportMode string

const (
	// ResponsesSupportUnknown means no probe result is stored. Callers should
	// preserve the old behavior and use the Responses API.
	ResponsesSupportUnknown AccountResponsesSupport = iota
	ResponsesSupportYes
	ResponsesSupportNo
)

const (
	ResponsesSupportModeAuto                 ResponsesSupportMode = "auto"
	ResponsesSupportModeForceResponses       ResponsesSupportMode = "force_responses"
	ResponsesSupportModeForceChatCompletions ResponsesSupportMode = "force_chat_completions"
)

// ExtraKeyResponsesSupported is stored in accounts.extra.
const ExtraKeyResponsesSupported = "openai_responses_supported"

// ExtraKeyResponsesMode is the official Sub2API account-wide manual override.
// Per-model overrides below take precedence when a mixed-protocol account is
// used, while this mode remains useful for single-protocol accounts.
const ExtraKeyResponsesMode = "openai_responses_mode"

// ExtraKeyUpstreamProtocolByModel stores exact, case-insensitive model
// overrides in accounts.extra. Supported values are "responses" and
// "chat_completions". The account-wide openai_responses_supported flag remains
// the backwards-compatible fallback when no model override matches.
const ExtraKeyUpstreamProtocolByModel = "openai_upstream_protocol_by_model"

const (
	UpstreamProtocolResponses       = "responses"
	UpstreamProtocolChatCompletions = "chat_completions"
)

// ResolveResponsesSupport reads the persisted probe result.
func ResolveResponsesSupport(extra map[string]any) AccountResponsesSupport {
	if extra == nil {
		return ResponsesSupportUnknown
	}
	if mode, ok := extra[ExtraKeyResponsesMode].(string); ok {
		switch NormalizeResponsesSupportMode(mode) {
		case ResponsesSupportModeForceResponses:
			return ResponsesSupportYes
		case ResponsesSupportModeForceChatCompletions:
			return ResponsesSupportNo
		}
	}
	v, ok := extra[ExtraKeyResponsesSupported]
	if !ok {
		return ResponsesSupportUnknown
	}
	supported, ok := v.(bool)
	if !ok {
		return ResponsesSupportUnknown
	}
	if supported {
		return ResponsesSupportYes
	}
	return ResponsesSupportNo
}

func NormalizeResponsesSupportMode(mode string) ResponsesSupportMode {
	switch ResponsesSupportMode(strings.ToLower(strings.TrimSpace(mode))) {
	case ResponsesSupportModeForceResponses:
		return ResponsesSupportModeForceResponses
	case ResponsesSupportModeForceChatCompletions:
		return ResponsesSupportModeForceChatCompletions
	default:
		return ResponsesSupportModeAuto
	}
}

// ShouldUseResponsesAPI returns false only when a probe explicitly confirmed
// that the APIKey upstream does not expose /v1/responses.
func ShouldUseResponsesAPI(extra map[string]any) bool {
	return ResolveResponsesSupport(extra) != ResponsesSupportNo
}

// ResolveUpstreamProtocolForModel resolves a per-model protocol override. The
// candidates are checked in order so callers can prefer the public model name
// and then fall back to mapped/upstream aliases.
func ResolveUpstreamProtocolForModel(extra map[string]any, modelCandidates ...string) string {
	if extra == nil {
		return ""
	}
	raw, ok := extra[ExtraKeyUpstreamProtocolByModel]
	if !ok {
		return ""
	}

	protocols := make(map[string]string)
	switch values := raw.(type) {
	case map[string]any:
		for model, value := range values {
			if protocol, ok := value.(string); ok {
				protocols[strings.ToLower(strings.TrimSpace(model))] = normalizeUpstreamProtocol(protocol)
			}
		}
	case map[string]string:
		for model, protocol := range values {
			protocols[strings.ToLower(strings.TrimSpace(model))] = normalizeUpstreamProtocol(protocol)
		}
	default:
		return ""
	}

	for _, candidate := range modelCandidates {
		if protocol := protocols[strings.ToLower(strings.TrimSpace(candidate))]; protocol != "" {
			return protocol
		}
	}
	return ""
}

// ShouldUseResponsesAPIForModel applies an exact per-model override first and
// then preserves the legacy account-wide behavior.
func ShouldUseResponsesAPIForModel(extra map[string]any, modelCandidates ...string) bool {
	switch ResolveUpstreamProtocolForModel(extra, modelCandidates...) {
	case UpstreamProtocolResponses:
		return true
	case UpstreamProtocolChatCompletions:
		return false
	default:
		return ShouldUseResponsesAPI(extra)
	}
}

func normalizeUpstreamProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case UpstreamProtocolResponses:
		return UpstreamProtocolResponses
	case UpstreamProtocolChatCompletions, "chat", "chat-completions":
		return UpstreamProtocolChatCompletions
	default:
		return ""
	}
}

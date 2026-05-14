package openai_compat

// AccountResponsesSupport describes whether an OpenAI APIKey upstream supports
// the Responses API. It is meaningful only for platform=openai + type=apikey.
type AccountResponsesSupport int

const (
	// ResponsesSupportUnknown means no probe result is stored. Callers should
	// preserve the old behavior and use the Responses API.
	ResponsesSupportUnknown AccountResponsesSupport = iota
	ResponsesSupportYes
	ResponsesSupportNo
)

// ExtraKeyResponsesSupported is stored in accounts.extra.
const ExtraKeyResponsesSupported = "openai_responses_supported"

// ResolveResponsesSupport reads the persisted probe result.
func ResolveResponsesSupport(extra map[string]any) AccountResponsesSupport {
	if extra == nil {
		return ResponsesSupportUnknown
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

// ShouldUseResponsesAPI returns false only when a probe explicitly confirmed
// that the APIKey upstream does not expose /v1/responses.
func ShouldUseResponsesAPI(extra map[string]any) bool {
	return ResolveResponsesSupport(extra) != ResponsesSupportNo
}

package service

import (
	"errors"
	"fmt"
	"strings"
)

var ErrNoCompatibleAPIProtocol = errors.New("no compatible API protocol endpoint")

type APIProtocolDecision struct {
	InboundProtocol  string
	UpstreamProtocol string
	BaseURL          string
	Native           bool
	RequiresBridge   bool
}

func NormalizeAPIProtocol(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case APIProtocolChatCompletions:
		return APIProtocolChatCompletions
	case APIProtocolAnthropic:
		return APIProtocolAnthropic
	case APIProtocolResponses:
		return APIProtocolResponses
	case APIProtocolAdaptive:
		return APIProtocolAdaptive
	default:
		return ""
	}
}

func (a *Account) SupportsAdaptiveAPIProtocol() bool {
	return a != nil && a.Type == AccountTypeAPIKey
}

func (a *Account) GetAPIProtocol() string {
	if a == nil || a.Type != AccountTypeAPIKey {
		return ""
	}
	if configured := NormalizeAPIProtocol(a.GetCredential("api_protocol")); configured != "" {
		return configured
	}
	switch a.Platform {
	case PlatformAnthropic:
		return APIProtocolAnthropic
	case PlatformOpenAI, PlatformGrok:
		return APIProtocolResponses
	default:
		return APIProtocolChatCompletions
	}
}

func (a *Account) GetProtocolBaseURL(protocol string) string {
	if a == nil || a.Type != AccountTypeAPIKey {
		return ""
	}
	protocol = NormalizeAPIProtocol(protocol)
	if protocol == "" || protocol == APIProtocolAdaptive {
		return ""
	}
	if explicit := a.getExplicitProtocolBaseURL(protocol); explicit != "" {
		return explicit
	}
	return strings.TrimSpace(a.GetCredential("base_url"))
}

func (a *Account) getExplicitProtocolBaseURL(protocol string) string {
	if a == nil || a.Type != AccountTypeAPIKey || a.Credentials == nil {
		return ""
	}
	protocol = NormalizeAPIProtocol(protocol)
	if protocol == "" || protocol == APIProtocolAdaptive {
		return ""
	}
	if raw, ok := a.Credentials["api_base_urls"]; ok {
		switch values := raw.(type) {
		case map[string]any:
			if value, ok := values[protocol].(string); ok {
				if trimmed := strings.TrimSpace(value); trimmed != "" {
					return trimmed
				}
			}
		case map[string]string:
			if trimmed := strings.TrimSpace(values[protocol]); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func ResolveAPIProtocolForRequest(account *Account, inboundProtocol string) (APIProtocolDecision, error) {
	inboundProtocol = NormalizeAPIProtocol(inboundProtocol)
	if inboundProtocol == "" || inboundProtocol == APIProtocolAdaptive {
		return APIProtocolDecision{}, fmt.Errorf("%w: invalid inbound protocol", ErrNoCompatibleAPIProtocol)
	}
	if account == nil || account.Type != AccountTypeAPIKey {
		return APIProtocolDecision{}, fmt.Errorf("%w: adaptive routing is API-key only", ErrNoCompatibleAPIProtocol)
	}

	configured := account.GetAPIProtocol()
	if configured != APIProtocolAdaptive {
		baseURL := account.GetProtocolBaseURL(configured)
		if baseURL == "" {
			return APIProtocolDecision{}, fmt.Errorf("%w: %s endpoint is not configured", ErrNoCompatibleAPIProtocol, configured)
		}
		return APIProtocolDecision{
			InboundProtocol: inboundProtocol, UpstreamProtocol: configured, BaseURL: baseURL,
			Native: inboundProtocol == configured, RequiresBridge: inboundProtocol != configured,
		}, nil
	}

	for _, protocol := range adaptiveProtocolPreference(inboundProtocol) {
		if baseURL := account.getExplicitProtocolBaseURL(protocol); baseURL != "" {
			return APIProtocolDecision{
				InboundProtocol: inboundProtocol, UpstreamProtocol: protocol, BaseURL: baseURL,
				Native: inboundProtocol == protocol, RequiresBridge: inboundProtocol != protocol,
			}, nil
		}
	}
	return APIProtocolDecision{}, fmt.Errorf("%w: no endpoint configured for %s", ErrNoCompatibleAPIProtocol, inboundProtocol)
}

func adaptiveProtocolPreference(inbound string) []string {
	switch inbound {
	case APIProtocolAnthropic:
		return []string{APIProtocolAnthropic, APIProtocolResponses, APIProtocolChatCompletions}
	case APIProtocolResponses:
		return []string{APIProtocolResponses, APIProtocolChatCompletions, APIProtocolAnthropic}
	default:
		return []string{APIProtocolChatCompletions, APIProtocolResponses, APIProtocolAnthropic}
	}
}

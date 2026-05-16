package accountcredentials

import "strings"

// RedactForResponse returns a response-safe copy of account credentials.
func RedactForResponse(credentials map[string]any) map[string]any {
	if credentials == nil {
		return nil
	}

	redacted := make(map[string]any, len(credentials))
	for key, value := range credentials {
		if IsSensitiveKey(key) {
			continue
		}
		redacted[key] = redactValue(value)
	}
	return redacted
}

// MergeForUpdate applies incoming credentials while preserving existing
// sensitive values that are absent from the incoming payload.
func MergeForUpdate(existing, incoming map[string]any) map[string]any {
	if incoming == nil {
		return nil
	}

	merged := make(map[string]any, len(incoming)+len(existing))
	for key, value := range incoming {
		merged[key] = value
	}
	for key, value := range existing {
		if !IsSensitiveKey(key) {
			continue
		}
		if _, ok := incoming[key]; ok {
			continue
		}
		merged[key] = value
	}
	return merged
}

func redactValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return RedactForResponse(v)
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = redactValue(v[i])
		}
		return out
	default:
		return value
	}
}

func IsSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(key, "-", "_")))
	switch normalized {
	case "access_token",
		"refresh_token",
		"id_token",
		"api_key",
		"apikey",
		"authorization",
		"password",
		"secret",
		"client_secret",
		"aws_secret_access_key",
		"aws_session_token",
		"session_token",
		"bearer_token",
		"private_key",
		"token":
		return true
	}

	for _, suffix := range []string{
		"_access_token",
		"_refresh_token",
		"_id_token",
		"_api_key",
		"_authorization",
		"_password",
		"_secret",
		"_session_token",
		"_private_key",
	} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}

	for _, marker := range []string{
		"_secret_",
		"_password_",
		"_api_key_",
		"_authorization_",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}

	for _, prefix := range []string{
		"secret_",
		"password_",
		"api_key_",
		"authorization_",
	} {
		if strings.HasPrefix(normalized, prefix) {
			return true
		}
	}

	return false
}

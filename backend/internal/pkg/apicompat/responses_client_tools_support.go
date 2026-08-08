package apicompat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// NamespacedToolName records the original owner of a flattened namespace tool.
type NamespacedToolName struct {
	Namespace string
	Name      string
}

const (
	chatToolNameMaxLen    = 64
	toolSearchProxyName   = "tool_search"
	customToolInputSchema = `{"type":"object","properties":{"input":{"type":"string","description":"The raw input for this tool, passed through verbatim."}},"required":["input"]}`
	toolSearchProxySchema = `{"type":"object","properties":{"query":{"type":"string","description":"Search query for tools or connectors to load."},"limit":{"type":"integer","description":"Maximum number of tool groups to return."}},"required":["query"]}`
)

func flattenNamespaceToolName(namespace, name string) string {
	full := namespace + "__" + name
	if len(full) <= chatToolNameMaxLen {
		return full
	}
	sum := sha256.Sum256([]byte(full))
	suffix := "__" + hex.EncodeToString(sum[:4])
	prefixLen := chatToolNameMaxLen - len(suffix)
	var prefix strings.Builder
	for _, ch := range full {
		if prefix.Len()+len(string(ch)) > prefixLen {
			break
		}
		_, _ = prefix.WriteRune(ch)
	}
	return prefix.String() + suffix
}

func extractCustomToolCallInput(arguments string) string {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return ""
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
		return trimmed
	}
	if raw, ok := obj["input"]; ok {
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			return value
		}
		return trimmed
	}
	if len(obj) == 0 {
		return ""
	}
	return trimmed
}

func toolSearchCallArgumentsJSON(arguments string) json.RawMessage {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return json.RawMessage(`{}`)
	}
	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}
	fallback, _ := json.Marshal(arguments)
	return fallback
}

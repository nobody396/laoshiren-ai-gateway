package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// OpenAIRouteFailureDomainExtraKey is deliberately exported because scheduler
// snapshots keep an allowlisted subset of Account.Extra. Both the writer and
// the route adapter must use the same key or an explicitly configured provider
// failure domain silently degrades to per-account isolation after cache load.
const OpenAIRouteFailureDomainExtraKey = "routing_failure_domain_id"

// OpenAIRouteFailureDomainID returns an explicit failure-domain identifier.
// Missing metadata deliberately falls back to account isolation instead of
// guessing from an account name or CDN hostname.
func OpenAIRouteFailureDomainID(account *Account) string {
	if account == nil {
		return ""
	}
	if direct := strings.TrimSpace(account.getExtraString(OpenAIRouteFailureDomainExtraKey)); direct != "" {
		return direct
	}
	if account.Extra != nil {
		if routing, ok := account.Extra["routing"].(map[string]any); ok {
			if nested, ok := routing["failure_domain_id"].(string); ok && strings.TrimSpace(nested) != "" {
				return strings.TrimSpace(nested)
			}
		}
	}
	if account.ID <= 0 {
		return ""
	}
	return fmt.Sprintf("account:%d", account.ID)
}

func OpenAIRouteEndpointHash(endpoint string) string {
	normalized := normalizeOpenAIRouteEndpoint(endpoint)
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:8])
}

// normalizeOpenAIRouteEndpoint keeps route identity stable while ensuring
// credentials and volatile query parameters can never affect or leak through
// the fingerprint. Host and scheme are case-insensitive; path case is not.
func normalizeOpenAIRouteEndpoint(endpoint string) string {
	normalized := strings.TrimSpace(endpoint)
	if normalized == "" {
		return ""
	}
	if parsed, err := url.Parse(normalized); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.Scheme = strings.ToLower(parsed.Scheme)
		parsed.Host = strings.ToLower(parsed.Host)
		parsed.User = nil
		parsed.RawQuery = ""
		parsed.ForceQuery = false
		parsed.Fragment = ""
		parsed.Path = strings.TrimRight(parsed.Path, "/")
		parsed.RawPath = ""
		return strings.TrimRight(parsed.String(), "/")
	}
	if idx := strings.IndexAny(normalized, "?#"); idx >= 0 {
		normalized = normalized[:idx]
	}
	return strings.TrimRight(strings.TrimSpace(normalized), "/")
}

func NewOpenAIRouteKey(
	account *Account,
	groupID int64,
	model string,
	requestClass OpenAIRouteRequestClass,
	endpoint string,
	transport string,
) (OpenAIRouteKey, error) {
	if account == nil || account.ID <= 0 || groupID <= 0 || strings.TrimSpace(model) == "" || !requestClass.Valid() {
		return OpenAIRouteKey{}, ErrOpenAIRouteNoCandidate
	}
	key := OpenAIRouteKey{
		GroupID:       groupID,
		AccountID:     account.ID,
		Model:         strings.TrimSpace(model),
		RequestClass:  requestClass,
		EndpointHash:  OpenAIRouteEndpointHash(endpoint),
		Transport:     strings.TrimSpace(transport),
		FailureDomain: OpenAIRouteFailureDomainID(account),
	}
	if !key.Valid() {
		return OpenAIRouteKey{}, ErrOpenAIRouteNoCandidate
	}
	return key, nil
}

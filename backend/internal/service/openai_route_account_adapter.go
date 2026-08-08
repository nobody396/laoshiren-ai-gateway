package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const openAIRouteFailureDomainExtraKey = "routing_failure_domain_id"

// OpenAIRouteFailureDomainID returns an explicit failure-domain identifier.
// Missing metadata deliberately falls back to account isolation instead of
// guessing from an account name or CDN hostname.
func OpenAIRouteFailureDomainID(account *Account) string {
	if account == nil {
		return ""
	}
	if direct := strings.TrimSpace(account.getExtraString(openAIRouteFailureDomainExtraKey)); direct != "" {
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
	normalized := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:8])
}

func NewOpenAIRouteKey(account *Account, groupID int64, model, endpoint, transport string) (OpenAIRouteKey, error) {
	if account == nil || account.ID <= 0 || groupID <= 0 || strings.TrimSpace(model) == "" {
		return OpenAIRouteKey{}, ErrOpenAIRouteNoCandidate
	}
	key := OpenAIRouteKey{
		GroupID:       groupID,
		AccountID:     account.ID,
		Model:         strings.TrimSpace(model),
		EndpointHash:  OpenAIRouteEndpointHash(endpoint),
		Transport:     strings.TrimSpace(transport),
		FailureDomain: OpenAIRouteFailureDomainID(account),
	}
	if !key.Valid() {
		return OpenAIRouteKey{}, ErrOpenAIRouteNoCandidate
	}
	return key, nil
}

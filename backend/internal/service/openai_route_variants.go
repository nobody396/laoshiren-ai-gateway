package service

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
)

const maxOpenAIRoutePolicyVariants = 8

type openAIRouteRouteVariantConfig struct {
	AccountID int64  `json:"account_id"`
	BaseURL   string `json:"base_url"`
}

type normalizedOpenAIRouteVariant struct {
	accountID    int64
	endpoint     string
	endpointHash string
}

// expandOpenAIRouteShadowVariants adds policy-declared endpoints only to the
// diagnostic candidate set. It reuses an already hard-filtered Account, so a
// route variant cannot bypass group/model/schedulability/transport eligibility
// and cannot mutate the Account or the Legacy endpoint.
func expandOpenAIRouteShadowVariants(
	sources []OpenAIRouteShadowCandidate,
	configs []openAIRouteRouteVariantConfig,
) ([]OpenAIRouteShadowCandidate, []OpenAIRouteShadowAuditRouteVariant, error) {
	expanded := append([]OpenAIRouteShadowCandidate(nil), sources...)
	if len(configs) == 0 {
		return expanded, nil, nil
	}
	if len(configs) > maxOpenAIRoutePolicyVariants {
		return nil, nil, fmt.Errorf("%w: route_variants exceeds %d entries", ErrOpenAIRouteInvalidPolicy, maxOpenAIRoutePolicyVariants)
	}

	normalized := make([]normalizedOpenAIRouteVariant, 0, len(configs))
	seen := make(map[string]struct{}, len(configs))
	for _, config := range configs {
		if config.AccountID <= 0 {
			return nil, nil, fmt.Errorf("%w: route variant account_id must be positive", ErrOpenAIRouteInvalidPolicy)
		}
		baseURL, err := normalizeOpenAIRouteVariantBaseURL(config.BaseURL)
		if err != nil {
			return nil, nil, err
		}
		endpoint := OpenAIRouteEndpointForBaseURL(baseURL, "/v1/responses")
		endpointHash := OpenAIRouteEndpointHash(endpoint)
		if endpointHash == "" {
			return nil, nil, fmt.Errorf("%w: route variant endpoint is invalid", ErrOpenAIRouteInvalidPolicy)
		}
		identity := fmt.Sprintf("%d:%s", config.AccountID, endpointHash)
		if _, exists := seen[identity]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate route variant %s", ErrOpenAIRouteInvalidPolicy, identity)
		}
		seen[identity] = struct{}{}
		normalized = append(normalized, normalizedOpenAIRouteVariant{
			accountID: config.AccountID, endpoint: endpoint, endpointHash: endpointHash,
		})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].accountID != normalized[j].accountID {
			return normalized[i].accountID < normalized[j].accountID
		}
		return normalized[i].endpointHash < normalized[j].endpointHash
	})

	audit := make([]OpenAIRouteShadowAuditRouteVariant, 0, len(normalized))
	for _, variant := range normalized {
		audit = append(audit, OpenAIRouteShadowAuditRouteVariant{
			AccountID: variant.accountID, EndpointHash: variant.endpointHash,
		})
	}

	existing := make(map[string]struct{}, len(sources)+len(normalized))
	for _, source := range sources {
		if source.Account == nil {
			continue
		}
		hash := OpenAIRouteEndpointHash(source.Endpoint)
		if hash != "" {
			existing[fmt.Sprintf("%d:%s", source.Account.ID, hash)] = struct{}{}
		}
	}
	for _, source := range sources {
		if source.Account == nil || !source.Account.IsOpenAI() || source.Account.Type != AccountTypeAPIKey {
			continue
		}
		if strings.TrimSpace(source.Transport) != string(OpenAIUpstreamTransportHTTPSSE) || !isOpenAIResponsesEndpoint(source.Endpoint) {
			continue
		}
		for _, variant := range normalized {
			if variant.accountID != source.Account.ID {
				continue
			}
			identity := fmt.Sprintf("%d:%s", variant.accountID, variant.endpointHash)
			if _, exists := existing[identity]; exists {
				continue
			}
			clone := source
			clone.Endpoint = variant.endpoint
			clone.RouteVariant = true
			// Account-local EWMA belongs to the actually executed endpoint. A
			// synthetic endpoint must earn route-specific evidence instead of
			// inheriting it and looking independently reliable.
			clone.HasReliabilitySample = false
			clone.SuccessLowerBound = 0
			clone.TTFTMilliseconds = 0
			expanded = append(expanded, clone)
			existing[identity] = struct{}{}
		}
	}
	return expanded, audit, nil
}

func normalizeOpenAIRouteVariantBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Host == "" {
		return "", fmt.Errorf("%w: route variant base_url must be an absolute HTTPS URL", ErrOpenAIRouteInvalidPolicy)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%w: route variant base_url cannot contain credentials, query or fragment", ErrOpenAIRouteInvalidPolicy)
	}
	hostname := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if hostname == "" || hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") || strings.HasSuffix(hostname, ".local") {
		return "", fmt.Errorf("%w: route variant base_url host is not public", ErrOpenAIRouteInvalidPolicy)
	}
	if ip := net.ParseIP(hostname); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()) {
		return "", fmt.Errorf("%w: route variant base_url IP is not public", ErrOpenAIRouteInvalidPolicy)
	}
	parsed.Scheme = "https"
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func isOpenAIResponsesEndpoint(endpoint string) bool {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}
	return strings.TrimRight(parsed.Path, "/") == "/v1/responses"
}

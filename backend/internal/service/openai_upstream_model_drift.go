package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/tlsfingerprint"
)

const chatGPTCodexModelsManifestURL = "https://chatgpt.com/backend-api/codex/models"

var ErrOpenAIModelDriftUnsupported = errors.New("OpenAI model drift inspection is unsupported for this account")

// OpenAIModelDrift is a read-only comparison between the models reported by
// one upstream account and the account's explicit local request/upstream map.
// It deliberately has no apply path: an administrator must review and save any
// model mapping through the existing account editor.
type OpenAIModelDrift struct {
	Status                    string   `json:"status"`
	Applied                   bool     `json:"applied"`
	UpstreamModels            []string `json:"upstream_models"`
	ConfiguredRequestModels   []string `json:"configured_request_models"`
	ConfiguredUpstreamModels  []string `json:"configured_upstream_models"`
	UpstreamUnmapped          []string `json:"upstream_unmapped"`
	ConfiguredMissingUpstream []string `json:"configured_missing_upstream"`
}

type openAIUpstreamModelEntry struct {
	ID    string `json:"id"`
	Slug  string `json:"slug"`
	Model string `json:"model"`
	Name  string `json:"name"`
}

// InspectOpenAIModelDrift reads a bounded upstream model manifest and compares
// it with local configuration. It never mutates account credentials, mappings,
// groups, the public catalog, or runtime routing.
func (s *AccountTestService) InspectOpenAIModelDrift(ctx context.Context, account *Account) (*OpenAIModelDrift, error) {
	if s == nil || s.httpUpstream == nil {
		return nil, errors.New("upstream model inspection is not configured")
	}
	if account == nil || !account.IsOpenAI() {
		return nil, ErrOpenAIModelDriftUnsupported
	}

	req, err := s.buildOpenAIModelManifestRequest(ctx, account)
	if err != nil {
		return nil, err
	}
	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}
	profile := (*tlsfingerprint.Profile)(nil)
	if s.tlsFPProfileService != nil {
		profile = s.tlsFPProfileService.ResolveTLSProfile(account)
	}
	resp, err := s.httpUpstream.DoWithTLS(req, proxyURL, account.ID, account.Concurrency, profile)
	if err != nil {
		return nil, fmt.Errorf("request upstream model manifest: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	limit := resolveModelsListReadLimit(s.cfg)
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, errors.New("read upstream model manifest failed")
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("upstream model manifest exceeds %d bytes", limit)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("upstream model manifest returned HTTP %d", resp.StatusCode)
	}

	upstreamModels, err := parseOpenAIUpstreamModelIDs(body)
	if err != nil {
		return nil, errors.New("upstream model manifest is invalid")
	}
	if len(upstreamModels) == 0 {
		return nil, errors.New("upstream model manifest contains no models")
	}

	mapping := account.GetModelMapping()
	requestModels := make([]string, 0, len(mapping))
	configuredUpstream := make([]string, 0, len(mapping))
	for requested, upstream := range mapping {
		requestModels = append(requestModels, requested)
		configuredUpstream = append(configuredUpstream, upstream)
	}
	requestModels = dedupeSortedModelIDs(requestModels)
	configuredUpstream = dedupeSortedModelIDs(configuredUpstream)

	drift := &OpenAIModelDrift{
		Applied:                   false,
		UpstreamModels:            upstreamModels,
		ConfiguredRequestModels:   requestModels,
		ConfiguredUpstreamModels:  configuredUpstream,
		UpstreamUnmapped:          modelSetDifference(upstreamModels, configuredUpstream),
		ConfiguredMissingUpstream: modelSetDifference(configuredUpstream, upstreamModels),
	}
	if len(drift.UpstreamUnmapped) == 0 && len(drift.ConfiguredMissingUpstream) == 0 {
		drift.Status = "in_sync"
	} else {
		drift.Status = "review_required"
	}
	return drift, nil
}

func (s *AccountTestService) buildOpenAIModelManifestRequest(ctx context.Context, account *Account) (*http.Request, error) {
	var endpoint, token string
	if account.IsOpenAIOAuth() {
		endpoint = chatGPTCodexModelsManifestURL + "?client_version=" + codexCLIVersion
		token = strings.TrimSpace(account.GetOpenAIAccessToken())
	} else if account.Type == AccountTypeAPIKey {
		baseURL := strings.TrimSpace(account.GetOpenAIBaseURL())
		if baseURL == "" {
			baseURL = "https://api.openai.com"
		}
		normalized, err := s.validateUpstreamBaseURL(baseURL)
		if err != nil {
			return nil, errors.New("invalid OpenAI base URL")
		}
		endpoint = buildOpenAIEndpointURL(normalized, "/v1/models")
		token = strings.TrimSpace(account.GetOpenAIApiKey())
	} else {
		return nil, ErrOpenAIModelDriftUnsupported
	}
	if token == "" {
		return nil, errors.New("OpenAI credential is unavailable")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("invalid OpenAI model manifest request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if account.IsOpenAIOAuth() {
		ensureCodexIdentityHeaders(req.Header)
		setOpenAIChatGPTAccountHeaders(req.Header, account)
		enforceCodexIdentityHeaders(req.Header)
	}
	account.ApplyHeaderOverrides(req.Header)
	return req, nil
}

func parseOpenAIUpstreamModelIDs(body []byte) ([]string, error) {
	var envelope struct {
		Data   []openAIUpstreamModelEntry `json:"data"`
		Models []openAIUpstreamModelEntry `json:"models"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		var direct []openAIUpstreamModelEntry
		if directErr := json.Unmarshal(body, &direct); directErr != nil {
			return nil, err
		}
		envelope.Data = direct
	}
	entries := make([]openAIUpstreamModelEntry, 0, len(envelope.Data)+len(envelope.Models))
	entries = append(entries, envelope.Data...)
	entries = append(entries, envelope.Models...)
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			id = strings.TrimSpace(entry.Slug)
		}
		if id == "" {
			id = strings.TrimSpace(entry.Model)
		}
		if id == "" {
			id = strings.TrimSpace(entry.Name)
		}
		ids = append(ids, strings.TrimPrefix(id, "models/"))
	}
	return dedupeSortedModelIDs(ids), nil
}

func dedupeSortedModelIDs(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	result := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		result = append(result, model)
	}
	sort.Strings(result)
	return result
}

func modelSetDifference(left, right []string) []string {
	rightSet := make(map[string]struct{}, len(right))
	for _, model := range right {
		rightSet[model] = struct{}{}
	}
	result := make([]string, 0)
	for _, model := range left {
		if _, ok := rightSet[model]; !ok {
			result = append(result, model)
		}
	}
	return result
}

//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

type modelDriftHTTPUpstream struct {
	response *http.Response
	request  *http.Request
}

func (u *modelDriftHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	return u.response, nil
}

func (u *modelDriftHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func TestInspectOpenAIModelDriftIsReadOnlyAndComparesMappedUpstreamModels(t *testing.T) {
	upstream := &modelDriftHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"data":[{"id":"gpt-5.6-sol"},{"id":"gpt-5.7"},{"id":"gpt-5.7"}]}`)),
	}}
	cfg := &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}}
	svc := &AccountTestService{httpUpstream: upstream, cfg: cfg}
	account := &Account{
		ID:       42,
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "secret-test-key",
			"base_url": "https://api.openai.com",
			"model_mapping": map[string]any{
				"gpt-current": "gpt-5.6-sol",
				"gpt-retired": "gpt-5.5",
			},
		},
	}

	drift, err := svc.InspectOpenAIModelDrift(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5.6-sol", "gpt-5.7"}, drift.UpstreamModels)
	require.Equal(t, []string{"gpt-current", "gpt-retired"}, drift.ConfiguredRequestModels)
	require.Equal(t, []string{"gpt-5.5", "gpt-5.6-sol"}, drift.ConfiguredUpstreamModels)
	require.Equal(t, []string{"gpt-5.7"}, drift.UpstreamUnmapped)
	require.Equal(t, []string{"gpt-5.5"}, drift.ConfiguredMissingUpstream)
	require.False(t, drift.Applied)
	require.Equal(t, "review_required", drift.Status)
	require.Equal(t, "Bearer secret-test-key", upstream.request.Header.Get("Authorization"))
	require.Equal(t, "https://api.openai.com/v1/models", upstream.request.URL.String())
	// Inspection must not mutate the account mapping.
	require.Equal(t, "gpt-5.6-sol", account.GetModelMapping()["gpt-current"])
}

func TestInspectOpenAIModelDriftRejectsOversizedResponse(t *testing.T) {
	upstream := &modelDriftHTTPUpstream{response: &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"data":[]}` + strings.Repeat(" ", 32))),
	}}
	cfg := &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}}
	cfg.Gateway.ModelsListReadMaxBytes = 8
	svc := &AccountTestService{httpUpstream: upstream, cfg: cfg}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "x"}}

	_, err := svc.InspectOpenAIModelDrift(context.Background(), account)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "secret")
}

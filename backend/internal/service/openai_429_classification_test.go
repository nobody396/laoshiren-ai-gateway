//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIOAuthTransient429IsRetryableOnSameAccount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Retry-After": []string{"1"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"try again"}}`)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, rateLimitService: NewRateLimitService(&rateLimitAccountRepoStub{}, nil, &config.Config{}, nil, nil)}
	account := &Account{
		ID: 429, Name: "oauth", Platform: PlatformOpenAI, Type: AccountTypeOAuth,
		Concurrency: 1, Status: StatusActive, Schedulable: true, RateMultiplier: f64p(1),
		Credentials: map[string]any{"access_token": "token"}, Extra: map[string]any{"openai_passthrough": true},
	}

	_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.6","stream":false,"instructions":"test","input":"hi"}`))

	var failover *UpstreamFailoverError
	require.Error(t, err)
	require.True(t, errors.As(err, &failover))
	require.True(t, failover.RetryableOnSameAccount)
}

func TestClassifyOpenAIOAuth429DistinguishesQuotaAndTransient(t *testing.T) {
	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "604800")
	headers.Set("x-codex-primary-window-minutes", "10080")

	disposition, resetAt := classifyOpenAIOAuth429(headers, nil)
	require.Equal(t, openAIOAuth429Quota7d, disposition)
	require.NotNil(t, resetAt)
	require.Greater(t, time.Until(*resetAt), 6*24*time.Hour)

	disposition, resetAt = classifyOpenAIOAuth429(http.Header{"Retry-After": []string{"1"}}, []byte(`{"error":{"message":"try again"}}`))
	require.Equal(t, openAIOAuth429Transient, disposition)
	require.Nil(t, resetAt)
}

func TestRateLimitServiceDoesNotPersistTransientOpenAIOAuth429(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{ID: 429, Platform: PlatformOpenAI, Type: AccountTypeOAuth}

	svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, http.Header{"Retry-After": []string{"1"}}, []byte(`{"error":{"message":"try again"}}`))
	require.Zero(t, repo.setRateLimitedCalls)

	headers := http.Header{}
	headers.Set("x-codex-primary-used-percent", "100")
	headers.Set("x-codex-primary-reset-after-seconds", "3600")
	headers.Set("x-codex-primary-window-minutes", "300")
	svc.HandleUpstreamError(context.Background(), account, http.StatusTooManyRequests, headers, nil)
	require.Equal(t, 1, repo.setRateLimitedCalls)
}

func TestKimiConcurrency403UsesTemporaryCooldown(t *testing.T) {
	repo := &rateLimitAccountRepoStub{}
	svc := NewRateLimitService(repo, nil, &config.Config{}, nil, nil)
	account := &Account{ID: 403, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": "https://api.kimi.com/v1"}}
	body := []byte(`{"error":{"message":"You've reached your concurrent request limit. Please wait for your ongoing requests to finish and try again."}}`)

	shouldFailover := svc.HandleUpstreamError(context.Background(), account, http.StatusForbidden, http.Header{}, body)

	require.True(t, shouldFailover)
	require.Zero(t, repo.setErrorCalls)
	require.Equal(t, 1, repo.tempCalls)
	require.Contains(t, repo.lastTempReason, kimiConcurrencyLimitReasonPrefix)
	require.True(t, repo.lastTempUntil.After(time.Now()))
}

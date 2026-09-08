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

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIAccountCapabilityRejection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		status  int
		message string
		body    string
		want    bool
	}{
		{
			name:    "kiro client tool search",
			status:  http.StatusBadRequest,
			message: "Client-executed tool_search is not supported by the Kiro upstream.",
			want:    true,
		},
		{
			name:   "kiro external web access",
			status: http.StatusBadRequest,
			body:   `{"error":{"message":"web_search external_web_access=false is not supported by the configured Kiro MCP backend."}}`,
			want:   true,
		},
		{
			name:    "daybreak account permission",
			status:  http.StatusBadRequest,
			message: "The 'gpt-daybreak-blue-latest' model is not supported when using Codex with a ChatGPT account.",
			want:    true,
		},
		{
			name:    "tool state id protocol",
			status:  http.StatusBadRequest,
			message: "Invalid 'input[95].id': 'ts_123'. Expected an ID that begins with 'tsc'.",
			want:    true,
		},
		{
			name:    "reasoning max resolved to gpt 5.4",
			status:  http.StatusBadRequest,
			message: "Unsupported value: 'max' is not supported with the 'gpt-5.4' model.",
			want:    true,
		},
		{
			name:    "compatible gateway account pool misses model",
			status:  http.StatusNotFound,
			message: `Model "gpt-5.4" is not supported by any configured account in this group`,
			want:    true,
		},
		{
			name:    "ordinary not found remains terminal",
			status:  http.StatusNotFound,
			message: "resource not found",
			want:    false,
		},
		{
			name:    "ordinary invalid request remains terminal",
			status:  http.StatusBadRequest,
			message: "Invalid input: field is required",
			want:    false,
		},
		{
			name:    "same wording on success is ignored",
			status:  http.StatusOK,
			message: "Client-executed tool_search is not supported by the Kiro upstream.",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, isOpenAIAccountCapabilityRejection(tt.status, tt.message, []byte(tt.body)))
		})
	}
}

func TestShouldFailoverOpenAIUpstreamResponseUsesCapabilitySignaturesOnly(t *testing.T) {
	t.Parallel()

	svc := &OpenAIGatewayService{}
	require.True(t, svc.shouldFailoverOpenAIUpstreamResponse(
		http.StatusBadRequest,
		"Client-executed tool_search is not supported by the Kiro upstream.",
		nil,
	))
	require.False(t, svc.shouldFailoverOpenAIUpstreamResponse(
		http.StatusBadRequest,
		"Invalid input: field is required",
		nil,
	))
	require.False(t, svc.shouldFailoverOpenAIUpstreamResponse(
		http.StatusBadRequest,
		"Your input exceeds the context window of this model",
		[]byte(`{"error":{"code":"context_length_exceeded"}}`),
	))
}

func TestOpenAIGatewayServiceCapability400ReturnsFailoverWithoutWritingClientError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("User-Agent", "Codex Desktop/0.149.0")
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"message":"Client-executed tool_search is not supported by the Kiro upstream.","type":"invalid_request_error"}}`,
			)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             69,
		Name:           "kiro-compatible-upstream",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeAPIKey,
		Concurrency:    1,
		Credentials:    map[string]any{"api_key": "sk-test"},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.6-sol","stream":false,"input":"hello"}`))
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusBadRequest, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.False(t, c.Writer.Written(), "capability rejection must return to the handler for next-account failover")
}

func TestOpenAIGatewayServiceCapability404ReturnsFailoverWithoutWritingClientError(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"error":{"message":"Model \"gpt-5.4\" is not supported by any configured account in this group","type":"not_found_error"}}`,
		)),
	}}
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}}, httpUpstream: upstream}
	account := &Account{
		ID: 79, Name: "compatible-gateway", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Concurrency: 1, Credentials: map[string]any{"api_key": "sk-test"}, Status: StatusActive,
		Schedulable: true, RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.4","stream":false,"input":"hello"}`))
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusNotFound, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount)
	require.False(t, c.Writer.Written())
}

func TestOpenAIGatewayServiceSessionPolicyBlockSkipsSameAccountRetry(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(nil))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{
		resp: &http.Response{
			StatusCode: http.StatusForbidden,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body: io.NopCloser(strings.NewReader(
				`{"error":{"code":"session_blocked_by_cyber_policy","message":"This session is blocked by cyber-security policy, please start a new session","type":"permission_error"}}`,
			)),
		},
	}
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Gateway: config.GatewayConfig{ForceCodexCLI: false}},
		httpUpstream: upstream,
	}
	account := &Account{
		ID:             80,
		Name:           "pool-mode-openai-upstream",
		Platform:       PlatformOpenAI,
		Type:           AccountTypeAPIKey,
		Concurrency:    1,
		Credentials:    map[string]any{"api_key": "sk-test", "pool_mode": true},
		Status:         StatusActive,
		Schedulable:    true,
		RateMultiplier: f64p(1),
	}

	_, err := svc.Forward(context.Background(), c, account, []byte(`{"model":"gpt-5.6-terra","stream":false,"input":"hello"}`))
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusForbidden, failoverErr.StatusCode)
	require.False(t, failoverErr.RetryableOnSameAccount, "session policy blocks must switch suppliers immediately")
	require.False(t, c.Writer.Written(), "session policy block must return to the handler for next-account failover")
}

//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRawChatFastFilterDoesNotForwardOrBillPriorityTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false,"service_tier":"priority"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_ok","object":"chat.completion","model":"gpt-5.4","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		cfg:            rawChatCompletionsTestConfig(),
		httpUpstream:   upstream,
		settingService: &SettingService{},
	}
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{
		"api_key": "test-key", "base_url": "https://provider.example/v1",
	}}
	ctx := withOpenAIFastPolicyContext(context.Background(), DefaultOpenAIFastPolicySettings())

	result, err := svc.forwardAsRawChatCompletions(ctx, c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.ServiceTier, "filtered Fast must not be billed as priority")
	require.False(t, gjson.GetBytes(upstream.lastBody, "service_tier").Exists(), "filtered Fast must not reach upstream")
}

func TestResponsesBridgeFastFilterDoesNotForwardOrBillPriorityTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gpt-5.4","messages":[{"role":"user","content":"hello"}],"stream":false,"service_tier":"priority"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_ok\",\"status\":\"completed\",\"model\":\"gpt-5.4\",\"output\":[],\"usage\":{\"input_tokens\":2,\"output_tokens\":1}}}\n\n",
		)),
	}}
	svc := &OpenAIGatewayService{
		cfg:            rawChatCompletionsTestConfig(),
		httpUpstream:   upstream,
		settingService: &SettingService{},
	}
	account := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{
		"api_key": "test-key", "base_url": "https://provider.example/v1",
	}}
	ctx := withOpenAIFastPolicyContext(context.Background(), DefaultOpenAIFastPolicySettings())

	result, err := svc.ForwardAsChatCompletions(ctx, c, account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.ServiceTier, "filtered Fast must not be billed as priority")
	require.False(t, gjson.GetBytes(upstream.lastBody, "service_tier").Exists(), "filtered Fast must not reach upstream")
}

func TestGeminiCompatibleChatFastFilterDoesNotBypassGlobalPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"gemini-3.1-pro","messages":[{"role":"user","content":"hello"}],"stream":false,"service_tier":"priority"}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_gemini","object":"chat.completion","model":"gemini-3.1-pro","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		cfg:            rawChatCompletionsTestConfig(),
		httpUpstream:   upstream,
		settingService: &SettingService{},
	}
	account := &Account{ID: 3, Platform: PlatformGemini, Type: AccountTypeAPIKey, Concurrency: 1, Credentials: map[string]any{
		"api_key": "test-key", "base_url": "https://provider.example/v1",
	}}
	ctx := withOpenAIFastPolicyContext(context.Background(), DefaultOpenAIFastPolicySettings())

	result, err := svc.forwardGeminiChatCompletions(ctx, c, account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.ServiceTier)
	require.False(t, gjson.GetBytes(upstream.lastBody, "service_tier").Exists())
}

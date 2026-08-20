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
	"github.com/tidwall/gjson"
)

func geminiChatBridgeTestConfig() *config.Config {
	return &config.Config{
		Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{
				Enabled:           false,
				AllowInsecureHTTP: true,
			},
		},
	}
}

func geminiChatBridgeGinContext(body []byte) (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, rec
}

func TestForwardGeminiChatCompletions_NonStreamPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gemini-3.1-pro","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c, rec := geminiChatBridgeGinContext(body)

	upstreamJSON := `{"id":"chatcmpl_gemini","object":"chat.completion","model":"gemini-3.1-pro-preview","choices":[{"index":0,"message":{"role":"assistant","content":"hi there"},"finish_reason":"stop"}],"usage":{"prompt_tokens":7,"completion_tokens":3,"total_tokens":10}}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"rid_gemini_bridge"}},
		Body:       io.NopCloser(strings.NewReader(upstreamJSON)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          geminiChatBridgeTestConfig(),
		httpUpstream: upstream,
	}
	account := geminiBridgeTestAccount(101, map[string]any{
		"gemini-3.1-pro": "gemini-3.1-pro-preview",
	})

	result, err := svc.forwardGeminiChatCompletions(context.Background(), c, &account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// URL 与鉴权：{base_url}/v1/chat/completions + Bearer api_key。
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://hk2.pomoai.xyz/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "Bearer sk-gemini-test", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Accept"))

	// model_mapping 应用到上游 model 字段，其余请求体保持原样。
	require.Equal(t, "gemini-3.1-pro-preview", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "hello", gjson.GetBytes(upstream.lastBody, "messages.0.content").String())

	// OpenAI 格式 usage 透传到计费结果。
	require.Equal(t, 7, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.Equal(t, "gemini-3.1-pro", result.Model)
	require.Equal(t, "gemini-3.1-pro-preview", result.UpstreamModel)
	require.Equal(t, "/v1/chat/completions", result.UpstreamEndpoint)

	// 非流式响应体原样回写给客户端。
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), "hi there")
}

func TestForwardGeminiChatCompletions_StreamPassthroughCapturesUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gemini-3.7-flash","messages":[{"role":"user","content":"hello"}],"stream":true}`)
	c, rec := geminiChatBridgeGinContext(body)

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_g","object":"chat.completion.chunk","model":"gemini-3.7-flash","choices":[{"index":0,"delta":{"content":"ok"}}]}`,
		"",
		`data: {"id":"chatcmpl_g","object":"chat.completion.chunk","model":"gemini-3.7-flash","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":5,"total_tokens":16}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_gemini_stream"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          geminiChatBridgeTestConfig(),
		httpUpstream: upstream,
	}
	account := geminiBridgeTestAccount(102, map[string]any{
		"gemini-3.7-flash": "gemini-3.7-flash",
	})

	result, err := svc.forwardGeminiChatCompletions(context.Background(), c, &account, body, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// 流式请求强制上游上报 usage，SSE 逐行透传给客户端。
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.Equal(t, "text/event-stream", upstream.lastReq.Header.Get("Accept"))
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
	require.NotNil(t, result.FirstTokenMs)
	require.Contains(t, rec.Body.String(), `"content":"ok"`)
	require.Contains(t, rec.Body.String(), "data: [DONE]")
}

func TestForwardGeminiChatCompletions_Upstream429TriggersFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gemini-3.1-pro","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c, _ := geminiChatBridgeGinContext(body)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited","type":"rate_limit_error"}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          geminiChatBridgeTestConfig(),
		httpUpstream: upstream,
	}
	account := geminiBridgeTestAccount(103, map[string]any{
		"gemini-3.1-pro": "gemini-3.1-pro",
	})

	result, err := svc.forwardGeminiChatCompletions(context.Background(), c, &account, body, "")
	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusTooManyRequests, failoverErr.StatusCode)
	// pool_mode=true 的账号允许同账号重试（与 openai 账号失败切换语义一致）。
	require.True(t, failoverErr.RetryableOnSameAccount)
}

func TestForwardGeminiChatCompletions_Upstream400DoesNotFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gemini-3.1-pro","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c, rec := geminiChatBridgeGinContext(body)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusBadRequest,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad request","type":"invalid_request_error"}}`)),
	}}

	svc := &OpenAIGatewayService{
		cfg:          geminiChatBridgeTestConfig(),
		httpUpstream: upstream,
	}
	account := geminiBridgeTestAccount(104, map[string]any{
		"gemini-3.1-pro": "gemini-3.1-pro",
	})

	result, err := svc.forwardGeminiChatCompletions(context.Background(), c, &account, body, "")
	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestForwardGeminiChatCompletions_MissingCredentialFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gemini-3.1-pro","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c, _ := geminiChatBridgeGinContext(body)

	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK}}
	svc := &OpenAIGatewayService{
		cfg:          geminiChatBridgeTestConfig(),
		httpUpstream: upstream,
	}
	account := geminiBridgeTestAccount(105, nil)
	account.Credentials = map[string]any{"base_url": "https://hk2.pomoai.xyz"}

	result, err := svc.forwardGeminiChatCompletions(context.Background(), c, &account, body, "")
	require.Nil(t, result)
	require.Error(t, err)
	require.Contains(t, err.Error(), "api_key")
	require.Nil(t, upstream.lastReq)
}

// TestForwardAsChatCompletions_GeminiDispatch 回归：ForwardAsChatCompletions 把
// gemini 平台账号分发到桥接，openai apikey 账号仍走 raw 路径（行为不变）。
func TestForwardAsChatCompletions_GeminiDispatch(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := []byte(`{"model":"gemini-3.1-pro","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	c, _ := geminiChatBridgeGinContext(body)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"chatcmpl_dispatch","object":"chat.completion","model":"gemini-3.1-pro","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		cfg:          geminiChatBridgeTestConfig(),
		httpUpstream: upstream,
	}
	account := geminiBridgeTestAccount(106, map[string]any{
		"gemini-3.1-pro": "gemini-3.1-pro",
	})

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, &account, body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, upstream.lastReq)
	require.Equal(t, "https://hk2.pomoai.xyz/v1/chat/completions", upstream.lastReq.URL.String())
	require.Equal(t, "/v1/chat/completions", result.UpstreamEndpoint)
}

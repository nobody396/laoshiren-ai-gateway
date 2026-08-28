package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestForwardResponsesViaChatCompletionsNonStreaming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"ZHIPU/GLM-5.3","instructions":"be concise","input":"hello","max_output_tokens":256,"stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_glm","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_glm","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"delta":{"content":"fast answer"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_glm","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
		`data: {"id":"chatcmpl_glm","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":3,"total_tokens":14,"prompt_tokens_details":{"cached_tokens":4}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "x-request-id": []string{"rid_bridge"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}
	account := protocolBridgeTestAccount()

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, openAIResponsesViaChatEndpoint, result.UpstreamEndpoint)
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.Equal(t, 4, result.Usage.CacheReadInputTokens)
	require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
	require.True(t, gjson.GetBytes(upstream.lastBody, "stream_options.include_usage").Bool())
	require.Equal(t, 256, int(gjson.GetBytes(upstream.lastBody, "max_completion_tokens").Int()))
	require.Equal(t, "system", gjson.GetBytes(upstream.lastBody, "messages.0.role").String())
	require.Equal(t, "fast answer", gjson.Get(recorder.Body.String(), "output.0.content.0.text").String())
	require.Equal(t, "ZHIPU/GLM-5.3", gjson.Get(recorder.Body.String(), "model").String())
}

func TestForwardResponsesViaChatCompletionsStreaming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"MiniMax/MiniMax-M3","input":"hello","stream":true}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_m3","object":"chat.completion.chunk","model":"MiniMax/MiniMax-M3","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_m3","object":"chat.completion.chunk","model":"MiniMax/MiniMax-M3","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
		`data: {"id":"chatcmpl_m3","object":"chat.completion.chunk","model":"MiniMax/MiniMax-M3","choices":[],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(upstreamBody))}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}
	account := protocolBridgeTestAccount()

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 2, result.Usage.InputTokens)
	require.Equal(t, 1, result.Usage.OutputTokens)
	require.Contains(t, recorder.Body.String(), "response.output_text.delta")
	require.Contains(t, recorder.Body.String(), `"delta":"ok"`)
	require.Contains(t, recorder.Body.String(), "response.completed")
}

func TestForwardAnthropicViaChatCompletionsBridge(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"ZHIPU/GLM-5.3","max_tokens":256,"messages":[{"role":"user","content":"hello"}],"stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_claude","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"delta":{"content":"anthropic answer"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_claude","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
		`data: {"id":"chatcmpl_claude","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[],"usage":{"prompt_tokens":7,"completion_tokens":2,"total_tokens":9}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(upstreamBody))}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, protocolBridgeTestAccount(), body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, openAIResponsesViaChatEndpoint, result.UpstreamEndpoint)
	require.Equal(t, 7, result.Usage.InputTokens)
	require.Equal(t, "anthropic answer", gjson.Get(recorder.Body.String(), "content.0.text").String())
	require.Equal(t, "message", gjson.Get(recorder.Body.String(), "type").String())
}

func TestForwardChatCompletionsUsesPerModelRawProtocolOverride(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"ZHIPU/GLM-5.3","messages":[{"role":"user","content":"hello"}],"stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	upstreamJSON := `{"id":"chatcmpl_raw","object":"chat.completion","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"message":{"role":"assistant","content":"raw answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2,"total_tokens":5}}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(upstreamJSON))}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}

	result, err := svc.ForwardAsChatCompletions(context.Background(), c, protocolBridgeTestAccount(), body, "", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "/v1/chat/completions", upstream.lastReq.URL.Path)
	require.Equal(t, "raw answer", gjson.Get(recorder.Body.String(), "choices.0.message.content").String())
}

func protocolBridgeTestConfig() *config.Config {
	return &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false, AllowInsecureHTTP: true}}}
}

func protocolBridgeTestAccount() *Account {
	return &Account{
		ID: 901, Name: "mixed-protocol-upstream", Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Concurrency: 1,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "http://upstream.example/v1"},
		Extra: map[string]any{openai_compat.ExtraKeyUpstreamProtocolByModel: map[string]any{
			"ZHIPU/GLM-5.3": "chat_completions", "MiniMax/MiniMax-M3": "chat_completions",
		}},
	}
}

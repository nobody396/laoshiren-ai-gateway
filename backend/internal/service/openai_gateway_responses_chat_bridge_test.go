package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type responsesBridgeFailingWriter struct {
	gin.ResponseWriter
	failAfter int
	writes    int
}

type responsesBridgeErrorReadCloser struct{ err error }

func (r *responsesBridgeErrorReadCloser) Read([]byte) (int, error) { return 0, r.err }
func (r *responsesBridgeErrorReadCloser) Close() error             { return nil }

type responsesBridgeReasoningCache struct {
	GatewayCache
	values map[string]string
}

func (c *responsesBridgeReasoningCache) cacheKey(scope ReasoningCacheScope, itemID string) string {
	return fmt.Sprintf("%d/%d/%s/%s", scope.UserID, scope.APIKeyID, scope.Model, itemID)
}

func (c *responsesBridgeReasoningCache) SetReasoningContent(_ context.Context, scope ReasoningCacheScope, itemID, content string, _ time.Duration) error {
	if c.values == nil {
		c.values = make(map[string]string)
	}
	c.values[c.cacheKey(scope, itemID)] = content
	return nil
}

func (c *responsesBridgeReasoningCache) GetReasoningContent(_ context.Context, scope ReasoningCacheScope, itemID string) (string, error) {
	if value := c.values[c.cacheKey(scope, itemID)]; value != "" {
		return value, nil
	}
	return "", ErrReasoningContentNotFound
}

func (w *responsesBridgeFailingWriter) Write(p []byte) (int, error) {
	if w.writes >= w.failAfter {
		return 0, errors.New("client disconnected")
	}
	w.writes++
	return w.ResponseWriter.Write(p)
}

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

func TestForwardResponsesViaChatCompletionsStreamsReasoningAndMultipleTools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"ZHIPU/GLM-5.3","input":"use both tools","stream":true,"tools":[{"type":"function","name":"one","parameters":{"type":"object"}},{"type":"function","name":"two","parameters":{"type":"object"}}]}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_multi","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"delta":{"reasoning_content":"plan"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_multi","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"one","arguments":"{\"a\":"}},{"index":1,"id":"call_2","type":"function","function":{"name":"two","arguments":"{\"b\":"}}]},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_multi","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}},{"index":1,"function":{"arguments":"2}"}}]},"finish_reason":"tool_calls"}]}`,
		"",
		`data: {"id":"chatcmpl_multi","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[],"usage":{"prompt_tokens":12,"completion_tokens":8,"total_tokens":20,"prompt_tokens_details":{"cached_tokens":5,"cache_creation_tokens":4},"completion_tokens_details":{"reasoning_tokens":2}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(upstreamBody))}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}

	result, err := svc.Forward(context.Background(), c, protocolBridgeTestAccount(), body)
	require.NoError(t, err)
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, 8, result.Usage.OutputTokens)
	require.Equal(t, 5, result.Usage.CacheReadInputTokens)
	require.Equal(t, 4, result.Usage.CacheCreationInputTokens)
	got := recorder.Body.String()
	require.Contains(t, got, "response.reasoning_summary_text.delta")
	require.Equal(t, 2, strings.Count(got, "event: response.function_call_arguments.done"))
	require.Contains(t, got, `"arguments":"{\"a\":1}"`)
	require.Contains(t, got, `"arguments":"{\"b\":2}"`)
	require.Contains(t, got, "response.completed")
}

func TestResponsesChatBridgeFailClosedBoundaries(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"previous response id", `{"model":"ZHIPU/GLM-5.3","input":"hello","previous_response_id":"resp_other"}`, "previous_response_id"},
		{"server web search", `{"model":"ZHIPU/GLM-5.3","input":"hello","tools":[{"type":"web_search"}]}`, "server tool"},
		{"encrypted reasoning cache miss", `{"model":"ZHIPU/GLM-5.3","input":[{"type":"reasoning","id":"rs_missing","encrypted_content":"opaque"},{"role":"user","content":"continue"}]}`, "encrypted reasoning"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(tt.body)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
			upstream := &httpUpstreamRecorder{}
			svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}

			result, err := svc.Forward(context.Background(), c, protocolBridgeTestAccount(), body)
			require.Error(t, err)
			require.Nil(t, result)
			require.Contains(t, strings.ToLower(err.Error()), strings.ToLower(tt.want))
			require.Nil(t, upstream.lastReq, "fail-closed validation must run before the upstream request")
		})
	}
}

func TestResponsesChatBridgeRestoresEncryptedReasoningFromTenantScopedCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"ZHIPU/GLM-5.3","input":[{"type":"reasoning","id":"rs_cached","encrypted_content":"opaque"},{"type":"function_call","call_id":"call_1","name":"lookup","arguments":"{}"},{"type":"function_call_output","call_id":"call_1","output":"done"},{"role":"user","content":"continue"}],"stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Set("api_key", &APIKey{ID: 71, UserID: 72})
	cache := &responsesBridgeReasoningCache{values: map[string]string{}}
	scope := ReasoningCacheScope{UserID: 72, APIKeyID: 71, Model: "ZHIPU/GLM-5.3"}
	cache.values[cache.cacheKey(scope, "rs_cached")] = "cached private reasoning"
	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_cache","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"ok"},"finish_reason":"stop"}]}`,
		"",
		`data: {"id":"chatcmpl_cache","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":4,"completion_tokens":1,"total_tokens":5}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(upstreamBody))}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream, cache: cache}

	result, err := svc.Forward(context.Background(), c, protocolBridgeTestAccount(), body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "cached private reasoning", gjson.GetBytes(upstream.lastBody, "messages.#(role==\"assistant\").reasoning_content").String())
}

func TestForwardResponsesViaChatCompletionsClientDisconnectDrainsUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"ZHIPU/GLM-5.3","input":"hello","stream":true}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Writer = &responsesBridgeFailingWriter{ResponseWriter: c.Writer, failAfter: 0}
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_disconnect","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"content":"partial"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_disconnect","object":"chat.completion.chunk","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"",
		`data: {"id":"chatcmpl_disconnect","object":"chat.completion.chunk","choices":[],"usage":{"prompt_tokens":19,"completion_tokens":7,"total_tokens":26,"prompt_tokens_details":{"cached_tokens":6}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(upstreamBody))}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}

	result, err := svc.Forward(context.Background(), c, protocolBridgeTestAccount(), body)
	require.NoError(t, err)
	require.Equal(t, 19, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, 6, result.Usage.CacheReadInputTokens)
}

func TestForwardResponsesViaChatCompletionsMapsRetryableUpstreamError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"ZHIPU/GLM-5.3","input":"hello","stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"rate limited"}}`)),
	}}
	account := protocolBridgeTestAccount()
	account.Credentials["pool_mode"] = true
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream, rateLimitService: &RateLimitService{}}

	result, err := svc.Forward(context.Background(), c, account, body)
	require.Nil(t, result)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Equal(t, http.StatusTooManyRequests, failover.StatusCode)
}

func TestForwardResponsesViaChatCompletionsFailsOnUpstreamStreamDisconnect(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"ZHIPU/GLM-5.3","input":"hello","stream":true}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       &responsesBridgeErrorReadCloser{err: io.ErrUnexpectedEOF},
	}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}

	result, err := svc.Forward(context.Background(), c, protocolBridgeTestAccount(), body)
	require.Nil(t, result)
	require.Error(t, err)
	require.NotContains(t, recorder.Body.String(), "response.completed")
}

func TestForwardResponsesViaChatCompletionsDoesNotForgeCompletedOnCleanTruncation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"ZHIPU/GLM-5.3","input":"hello","stream":true}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	partial := "data: {\"id\":\"chatcmpl_truncated\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\n"
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(partial)),
	}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}

	result, err := svc.Forward(context.Background(), c, protocolBridgeTestAccount(), body)
	require.Nil(t, result)
	require.Error(t, err)
	require.NotContains(t, recorder.Body.String(), "response.completed")
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

func TestForwardAnthropicViaChatCompletionsBridgeStreamingReasoningAndTool(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := []byte(`{"model":"ZHIPU/GLM-5.3","max_tokens":256,"messages":[{"role":"user","content":"call lookup"}],"tools":[{"name":"lookup","description":"lookup","input_schema":{"type":"object","properties":{"q":{"type":"string"}},"required":["q"]}}],"stream":true}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	upstreamBody := strings.Join([]string{
		`data: {"id":"chatcmpl_anth_tool","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"delta":{"reasoning_content":"need lookup"},"finish_reason":null}]}`,
		"",
		`data: {"id":"chatcmpl_anth_tool","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":\"x\"}"}}]},"finish_reason":"tool_calls"}]}`,
		"",
		`data: {"id":"chatcmpl_anth_tool","object":"chat.completion.chunk","model":"ZHIPU/GLM-5.3","choices":[],"usage":{"prompt_tokens":8,"completion_tokens":5,"total_tokens":13,"completion_tokens_details":{"reasoning_tokens":2}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(upstreamBody))}}
	svc := &OpenAIGatewayService{cfg: protocolBridgeTestConfig(), httpUpstream: upstream}

	result, err := svc.ForwardAsAnthropic(context.Background(), c, protocolBridgeTestAccount(), body, "", "")
	require.NoError(t, err)
	require.Equal(t, 8, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
	got := recorder.Body.String()
	require.Contains(t, got, "thinking_delta")
	require.Contains(t, got, "need lookup")
	require.Contains(t, got, `"type":"tool_use"`)
	require.Contains(t, got, `"name":"lookup"`)
	require.Contains(t, got, "input_json_delta")
	require.Contains(t, got, "message_stop")
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

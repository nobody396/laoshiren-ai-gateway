//go:build live_protocol

package service

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/openai_compat"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const defaultAliyunBeijingWorkspaceBaseURL = "https://ws-a1y59n5tlo58xfci.cn-beijing.maas.aliyuncs.com/compatible-mode/v1"

var aliyunChatOnlyModels = []string{
	"ZHIPU/GLM-5.3",
	"MiniMax/MiniMax-M3",
	"kimi-k3",
	"kimi-k2.7-code",
}

var aliyunNativeResponsesModels = []string{
	"glm-5.2",
	"deepseek-v4-pro",
	"deepseek-v4-pro-0813",
	"deepseek-v4-flash-0731",
	"qwen3.8-max",
	"qwen3.7-max",
	"qwen3.7-plus",
	"qwen3.7-flash",
	"qwen3.6-plus",
	"qwen3.6-flash",
}

type liveProtocolHTTPUpstream struct {
	client *http.Client
	mu     sync.Mutex
	paths  []string
}

func newLiveProtocolHTTPUpstream() *liveProtocolHTTPUpstream {
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	return &liveProtocolHTTPUpstream{client: &http.Client{
		Timeout: 3 * time.Minute,
		Transport: &http.Transport{
			Proxy:             http.ProxyFromEnvironment,
			DialContext:       dialer.DialContext,
			TLSClientConfig:   &tls.Config{MinVersion: tls.VersionTLS12},
			ForceAttemptHTTP2: true,
		},
	}}
}

func (u *liveProtocolHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.paths = append(u.paths, req.URL.Path)
	u.mu.Unlock()
	return u.client.Do(req)
}

func (u *liveProtocolHTTPUpstream) DoWithTLS(req *http.Request, proxyURL string, accountID int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxyURL, accountID, concurrency)
}

func (u *liveProtocolHTTPUpstream) lastPath() string {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.paths) == 0 {
		return ""
	}
	return u.paths[len(u.paths)-1]
}

func TestLiveAliyunBeijingProtocolMatrix(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("LAOSHIRENAI_ALIYUN_BEIJING_DOMESTIC_KEY"))
	if apiKey == "" {
		t.Skip("LAOSHIRENAI_ALIYUN_BEIJING_DOMESTIC_KEY is not available")
	}
	baseURL := strings.TrimSpace(os.Getenv("LAOSHIRENAI_ALIYUN_BEIJING_DOMESTIC_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultAliyunBeijingWorkspaceBaseURL
	}

	gin.SetMode(gin.TestMode)
	upstream := newLiveProtocolHTTPUpstream()
	svc := &OpenAIGatewayService{
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
		httpUpstream: upstream,
	}
	protocols := make(map[string]any, len(aliyunChatOnlyModels)+len(aliyunNativeResponsesModels))
	for _, model := range aliyunChatOnlyModels {
		protocols[model] = openai_compat.UpstreamProtocolChatCompletions
	}
	for _, model := range aliyunNativeResponsesModels {
		protocols[model] = openai_compat.UpstreamProtocolResponses
	}
	account := &Account{
		ID: 990001, Name: "aliyun-beijing-owner-live-probe", Platform: PlatformOpenAI,
		Type: AccountTypeAPIKey, Concurrency: 2,
		Credentials: map[string]any{"api_key": apiKey, "base_url": baseURL},
		Extra: map[string]any{
			openai_compat.ExtraKeyResponsesSupported:      true,
			openai_compat.ExtraKeyUpstreamProtocolByModel: protocols,
		},
	}

	for _, model := range aliyunChatOnlyModels {
		model := model
		t.Run("chat-only/text/non-stream/"+model, func(t *testing.T) {
			body := mustLiveProtocolJSON(t, map[string]any{
				"model": model, "input": "Reply with exactly BRIDGE_OK.", "max_output_tokens": 1024, "stream": false,
			})
			result, recorder, err := runLiveResponsesForward(t, svc, account, body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, openAIResponsesViaChatEndpoint, result.UpstreamEndpoint)
			require.Equal(t, "/compatible-mode/v1/chat/completions", upstream.lastPath())
			require.Equal(t, "completed", gjson.Get(recorder.Body.String(), "status").String())
			require.Contains(t, strings.ToUpper(gjson.Get(recorder.Body.String(), "output.#(type==\"message\").content.0.text").String()), "BRIDGE_OK")
			require.Greater(t, result.Usage.InputTokens, 0)
			require.Greater(t, result.Usage.OutputTokens, 0)
		})

		t.Run("chat-only/text/stream/"+model, func(t *testing.T) {
			body := mustLiveProtocolJSON(t, map[string]any{
				"model": model, "input": "Reply with exactly STREAM_OK.", "max_output_tokens": 1024, "stream": true,
			})
			result, recorder, err := runLiveResponsesForward(t, svc, account, body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Contains(t, recorder.Body.String(), "response.created")
			require.Contains(t, recorder.Body.String(), "response.output_text.delta")
			require.Contains(t, strings.ToUpper(recorder.Body.String()), "STREAM_OK")
			require.Contains(t, recorder.Body.String(), "response.completed")
			require.Greater(t, result.Usage.InputTokens, 0)
			require.Greater(t, result.Usage.OutputTokens, 0)
		})

		t.Run("chat-only/tool/stream/"+model, func(t *testing.T) {
			body := mustLiveProtocolJSON(t, map[string]any{
				"model":       model,
				"input":       "Call the lookup function once with q set to bridge.",
				"stream":      true,
				"tools":       []map[string]any{{"type": "function", "name": "lookup", "description": "lookup test", "parameters": map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}, "required": []string{"q"}}}},
				"tool_choice": map[string]any{"type": "function", "name": "lookup"},
			})
			result, recorder, err := runLiveResponsesForward(t, svc, account, body)
			require.NoError(t, err)
			require.Contains(t, recorder.Body.String(), "response.output_item.added")
			require.Contains(t, recorder.Body.String(), "response.function_call_arguments.done")
			require.Contains(t, recorder.Body.String(), `"name":"lookup"`)
			require.Contains(t, recorder.Body.String(), "response.completed")
			require.Greater(t, result.Usage.InputTokens, 0)
		})

		t.Run("chat-only/messages/non-stream/"+model, func(t *testing.T) {
			body := mustLiveProtocolJSON(t, map[string]any{
				"model": model, "max_tokens": 1024, "stream": false,
				"messages": []map[string]any{{"role": "user", "content": "Reply with exactly MESSAGES_OK."}},
			})
			result, recorder, err := runLiveAnthropicForward(t, svc, account, body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, openAIResponsesViaChatEndpoint, result.UpstreamEndpoint)
			require.Equal(t, "/compatible-mode/v1/chat/completions", upstream.lastPath())
			require.Equal(t, "message", gjson.Get(recorder.Body.String(), "type").String())
			require.Contains(t, strings.ToUpper(recorder.Body.String()), "MESSAGES_OK")
			require.Greater(t, result.Usage.InputTokens, 0)
			require.Greater(t, result.Usage.OutputTokens, 0)
		})
	}

	for _, model := range aliyunNativeResponsesModels {
		model := model
		t.Run("native-responses/"+model, func(t *testing.T) {
			body := mustLiveProtocolJSON(t, map[string]any{
				"model": model, "input": "Reply with exactly NATIVE_OK.", "max_output_tokens": 128, "stream": false,
			})
			result, recorder, err := runLiveResponsesForward(t, svc, account, body)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, recorder.Code)
			require.NotEqual(t, openAIResponsesViaChatEndpoint, result.UpstreamEndpoint)
			require.Equal(t, "/compatible-mode/v1/responses", upstream.lastPath())
			require.Equal(t, "completed", gjson.Get(recorder.Body.String(), "status").String())
			require.Contains(t, strings.ToUpper(recorder.Body.String()), "NATIVE_OK")
			require.Greater(t, result.Usage.InputTokens, 0)
			require.Greater(t, result.Usage.OutputTokens, 0)
		})
	}
}

func runLiveResponsesForward(t *testing.T, svc *OpenAIGatewayService, account *Account, body []byte) (*OpenAIForwardResult, *httptest.ResponseRecorder, error) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("api_key", &APIKey{ID: 990001, UserID: 990001, Status: StatusAPIKeyActive})
	result, err := svc.Forward(context.Background(), c, account, body)
	if result == nil {
		result = &OpenAIForwardResult{}
	}
	return result, recorder, err
}

func runLiveAnthropicForward(t *testing.T, svc *OpenAIGatewayService, account *Account, body []byte) (*OpenAIForwardResult, *httptest.ResponseRecorder, error) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("api_key", &APIKey{ID: 990001, UserID: 990001, Status: StatusAPIKeyActive})
	result, err := svc.ForwardAsAnthropic(context.Background(), c, account, body, "", "")
	if result == nil {
		result = &OpenAIForwardResult{}
	}
	return result, recorder, err
}

func mustLiveProtocolJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	require.NoError(t, err)
	return body
}

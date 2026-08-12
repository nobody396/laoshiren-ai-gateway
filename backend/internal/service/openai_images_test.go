package service

import (
	"bytes"
	"context"
	"encoding/base64"
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

const codexBridgeTestPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII="

func TestParseOpenAIImagesRequest(t *testing.T) {
	svc := &OpenAIGatewayService{}
	req, err := svc.ParseOpenAIImagesRequest([]byte(`{
		"model":"gpt-image-1",
		"prompt":"draw a dragon",
		"n":2,
		"size":"1024x1024",
		"response_format":"b64_json"
	}`))
	require.NoError(t, err)
	require.Equal(t, "gpt-image-1", req.Model)
	require.Equal(t, "draw a dragon", req.Prompt)
	require.Equal(t, 2, req.N)
	require.Equal(t, "1K", req.SizeTier)
	require.Equal(t, "b64_json", req.ResponseFormat)
}

func TestParseOpenAIImagesRequestRejectsStream(t *testing.T) {
	svc := &OpenAIGatewayService{}
	_, err := svc.ParseOpenAIImagesRequest([]byte(`{
		"model":"gpt-image-1",
		"prompt":"draw a dragon",
		"stream":true
	}`))
	require.ErrorContains(t, err, "not supported")
}

func TestSupportsOpenAIImages(t *testing.T) {
	require.True(t, supportsOpenAIImages(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{},
	}))

	require.False(t, supportsOpenAIImages(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeOAuth,
	}))

	require.False(t, supportsOpenAIImages(&Account{
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Extra:    map[string]any{"supports_images": false},
	}))
}

func TestExtractOpenAIImageCountFromJSONBytes(t *testing.T) {
	count := extractOpenAIImageCountFromJSONBytes([]byte(`{
		"created": 1,
		"data": [{"b64_json":"a"},{"b64_json":"b"},{"b64_json":"c"}]
	}`))
	require.Equal(t, 3, count)
}

func TestCodexNativeImageBridgeValidation(t *testing.T) {
	parsed := &OpenAIImagesRequest{Model: "gpt-image-2", Prompt: "draw", N: 1}
	require.True(t, ShouldBridgeCodexNativeImageGeneration(true, parsed))
	require.False(t, ShouldBridgeCodexNativeImageGeneration(false, parsed))
	require.False(t, ShouldBridgeCodexNativeImageGeneration(true, &OpenAIImagesRequest{Model: "gpt-image-1"}))
	require.NoError(t, ValidateCodexNativeImageBridgeRequest(parsed))

	invalidN := *parsed
	invalidN.N = 2
	require.ErrorContains(t, ValidateCodexNativeImageBridgeRequest(&invalidN), "n=1")
	invalidFormat := *parsed
	invalidFormat.ResponseFormat = "url"
	require.ErrorContains(t, ValidateCodexNativeImageBridgeRequest(&invalidFormat), "b64_json")
	emptyPrompt := *parsed
	emptyPrompt.Prompt = ""
	require.ErrorContains(t, ValidateCodexNativeImageBridgeRequest(&emptyPrompt), "prompt")
}

func TestExtractCompletedCodexImageResult(t *testing.T) {
	body := []byte(`{"status":"completed","output":[{"type":"image_generation_call","status":"completed","result":"` + codexBridgeTestPNG + `"},{"type":"message","status":"completed"}]}`)
	got, err := extractCompletedCodexImageResult(body)
	require.NoError(t, err)
	require.Equal(t, codexBridgeTestPNG, got)

	_, err = extractCompletedCodexImageResult([]byte(`{"status":"completed","output":[{"type":"message","status":"completed","content":[{"type":"output_text","text":""}]}]}`))
	require.ErrorIs(t, err, errCodexImageToolNotInvoked)

	_, err = extractCompletedCodexImageResult([]byte(`{"status":"completed","output":[{"type":"message","status":"completed","content":[]}],"tool_usage":{"image_gen":{"output_tokens_details":{"image_tokens":158}}}}`))
	require.Error(t, err)
	require.NotErrorIs(t, err, errCodexImageToolNotInvoked, "recorded image usage must never be regenerated on another account")

	for _, invalid := range []string{
		`{"status":"failed","output":[{"type":"image_generation_call","status":"completed","result":"` + codexBridgeTestPNG + `"}]}`,
		`{"status":"completed","output":[{"type":"image_generation_call","status":"in_progress","result":"` + codexBridgeTestPNG + `"}]}`,
		`{"status":"completed","output":[{"type":"image_generation_call","status":"completed","result":"not-base64"}]}`,
		`{"status":"completed","output":[{"type":"image_generation_call","status":"completed","result":"` + base64.StdEncoding.EncodeToString([]byte("not an image")) + `"}]}`,
	} {
		_, err := extractCompletedCodexImageResult([]byte(invalid))
		require.Error(t, err)
	}
}

func TestForwardCodexNativeImageGenerationBridgeNoImageCallSafelyFailsOver(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_no_tool","status":"completed","output":[{"type":"message","status":"completed","content":[{"type":"output_text","text":""}]}],"usage":{"input_tokens":467,"output_tokens":8}}`,
		)),
	}}
	svc, account, recorder, c := newCodexNativeImageBridgeFailureTest(upstream)

	result, err := svc.ForwardCodexNativeImageGenerationBridge(context.Background(), c, account, &OpenAIImagesRequest{Model: "gpt-image-2", Prompt: "draw", N: 1})

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.True(t, failoverErr.RequestScopedTransient)
	require.Equal(t, GatewayFailureScopeRequest, failoverErr.Scope)
	require.Equal(t, GatewayFailureReason("image_tool_not_invoked"), failoverErr.Reason)
	require.Equal(t, NextAccountRetry, failoverErr.NextAccountAction)
	require.Empty(t, recorder.Body.String(), "safe failover must not commit the empty upstream response")
}

func TestForwardCodexNativeImageGenerationBridge(t *testing.T) {
	upstreamBody := `{
		"id":"resp_bridge","status":"completed","model":"gpt-5.6-sol",
		"output":[{"type":"image_generation_call","id":"ig_1","status":"completed","result":"` + codexBridgeTestPNG + `"}],
		"usage":{"input_tokens":12,"output_tokens":3},
		"tool_usage":{"image_gen":{"output_tokens_details":{"image_tokens":44}}}
	}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "x-request-id": []string{"upstream-1"}},
		Body:       io.NopCloser(strings.NewReader(upstreamBody)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{
			Gateway:  config.GatewayConfig{Pipeline: config.GatewayPipelineConfig{OpenAIResponsesEnabled: false}},
			Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
		},
	}
	account := &Account{
		ID: 33, Name: "morecode", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://morecode.example/v1"},
	}
	gatewayRecorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(gatewayRecorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{}`))
	c.Request.Header.Set("User-Agent", "codex_cli_rs/1.0")
	parsed := &OpenAIImagesRequest{Model: "gpt-image-2", Prompt: "draw breakfast", N: 1, SizeTier: "2K"}

	result, err := svc.ForwardCodexNativeImageGenerationBridge(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, gatewayRecorder.Code)
	require.Equal(t, codexBridgeTestPNG, gjson.Get(gatewayRecorder.Body.String(), "data.0.b64_json").String())
	require.Equal(t, "gpt-image-2", result.Model)
	require.Equal(t, "gpt-5.6-sol", result.BillingModel)
	require.Equal(t, "gpt-5.6-sol", result.UpstreamModel)
	require.Equal(t, "/v1/responses", result.UpstreamEndpoint)
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, 3, result.Usage.OutputTokens)
	require.Equal(t, 44, result.Usage.ImageOutputTokens)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "gpt-5.6-sol", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "draw breakfast", gjson.GetBytes(upstream.lastBody, "input").String())
	require.Equal(t, "image_generation", gjson.GetBytes(upstream.lastBody, "tools.0.type").String())
	require.Equal(t, "image_generation", gjson.GetBytes(upstream.lastBody, "tool_choice.type").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool())
}

func TestForwardCodexNativeImageGenerationUsesNativeImagesFallback(t *testing.T) {
	body := []byte(`{
		"model":"gpt-image-2","prompt":"draw breakfast","n":1,
		"size":"auto","quality":"auto","background":"auto"
	}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"created":1710000000,"data":[{"b64_json":"` + codexBridgeTestPNG + `"}],"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
	}
	account := &Account{
		ID: 34, Name: "pomo-native-image", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "test", "base_url": "https://pomo.example/v1",
			"model_mapping": map[string]any{"gpt-image-2": "gpt-image-2-count"},
		},
		Extra: map[string]any{
			"supports_images":                      true,
			OpenAIImageGenerationPriorityExtraKey:  2,
			OpenAIImageGenerationModelsExtraKey:    []any{"gpt-image-2"},
			OpenAIImageGenerationTransportExtraKey: OpenAIImageGenerationTransportImages,
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))

	result, err := svc.ForwardCodexNativeImageGeneration(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, codexBridgeTestPNG, gjson.GetBytes(recorder.Body.Bytes(), "data.0.b64_json").String())
	require.Equal(t, "/v1/images/generations", upstream.lastReq.URL.Path)
	require.Equal(t, "gpt-image-2-count", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "b64_json", gjson.GetBytes(upstream.lastBody, "response_format").String())
	require.Equal(t, "auto", gjson.GetBytes(upstream.lastBody, "size").String())
	require.Equal(t, "auto", gjson.GetBytes(upstream.lastBody, "quality").String())
	require.Equal(t, "auto", gjson.GetBytes(upstream.lastBody, "background").String())
	require.Equal(t, "gpt-image-2", result.Model)
	require.Equal(t, "gpt-5.6-sol", result.BillingModel)
	require.Equal(t, "gpt-image-2-count", result.UpstreamModel)
	require.Equal(t, "/v1/images/generations", result.UpstreamEndpoint)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "2K", result.ImageSize)
	require.Equal(t, "gpt-image-2-count", c.GetString("ops_upstream_model"))
}

func TestForwardCodexNativeImageGenerationOmitsUnsupportedResponseFormat(t *testing.T) {
	body := []byte(`{
		"model":"gpt-image-2","prompt":"draw breakfast","n":1,
		"size":"1024x1024","response_format":"b64_json"
	}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"created":1710000000,"data":[{"b64_json":"` + codexBridgeTestPNG + `"}],"usage":{"total_tokens":30}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
	}
	account := &Account{
		ID: 39, Name: "pomo-azure-image", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "test", "base_url": "https://pomo.example/v1",
			"model_mapping": map[string]any{"gpt-image-2": "gpt-image-2"},
		},
		Extra: map[string]any{
			"supports_images":                               true,
			OpenAIImageGenerationPriorityExtraKey:           4,
			OpenAIImageGenerationModelsExtraKey:             []any{"gpt-image-2"},
			OpenAIImageGenerationTransportExtraKey:          OpenAIImageGenerationTransportImages,
			OpenAIImageGenerationOmitResponseFormatExtraKey: true,
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))

	result, err := svc.ForwardCodexNativeImageGeneration(context.Background(), c, account, parsed)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, codexBridgeTestPNG, gjson.GetBytes(recorder.Body.Bytes(), "data.0.b64_json").String())
	require.Equal(t, "gpt-image-2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.False(t, gjson.GetBytes(upstream.lastBody, "response_format").Exists())
	require.Equal(t, "gpt-image-2", result.UpstreamModel)
}

func TestForwardCodexNativeImageGenerationNativeNoOutputSafelyFailsOver(t *testing.T) {
	for _, tt := range []struct {
		name         string
		responseBody string
		wantFailover bool
	}{
		{name: "empty data without usage", responseBody: `{"created":1710000000,"data":[]}`, wantFailover: true},
		{name: "usage proves possible billing", responseBody: `{"created":1710000000,"data":[],"usage":{"total_tokens":1}}`},
		{name: "malformed generated item", responseBody: `{"created":1710000000,"data":[{"b64_json":"not-base64"}]}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			body := []byte(`{"model":"gpt-image-2","prompt":"draw","n":1}`)
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tt.responseBody)),
			}}
			svc := &OpenAIGatewayService{
				httpUpstream: upstream,
				cfg: &config.Config{Security: config.SecurityConfig{
					URLAllowlist: config.URLAllowlistConfig{Enabled: false},
				}},
			}
			account := &Account{
				ID: 34, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key": "test", "base_url": "https://pomo.example/v1",
					"model_mapping": map[string]any{"gpt-image-2": "gpt-image-2-count"},
				},
				Extra: map[string]any{
					"supports_images":                      true,
					OpenAIImageGenerationPriorityExtraKey:  2,
					OpenAIImageGenerationModelsExtraKey:    []any{"gpt-image-2"},
					OpenAIImageGenerationTransportExtraKey: OpenAIImageGenerationTransportImages,
				},
			}
			parsed, err := svc.ParseOpenAIImagesRequest(body)
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))

			result, err := svc.ForwardCodexNativeImageGeneration(context.Background(), c, account, parsed)

			require.Nil(t, result)
			require.Error(t, err)
			var failoverErr *UpstreamFailoverError
			if tt.wantFailover {
				require.ErrorAs(t, err, &failoverErr)
				require.Equal(t, GatewayFailureReason("native_image_no_output"), failoverErr.Reason)
				require.Empty(t, recorder.Body.String())
			} else {
				require.False(t, errors.As(err, &failoverErr), "possibly billable results must never be regenerated")
				require.Equal(t, http.StatusBadGateway, recorder.Code)
			}
		})
	}
}

func TestForwardCodexNativeImageGenerationBridgeProtocolFailureTriggersFailover(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_partial","status":"completed","output":[{"type":"image_generation_call","status":"in_progress","result":"` + codexBridgeTestPNG + `"}],"usage":{"input_tokens":1}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{ID: 33, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test", "base_url": "https://morecode.example/v1"}}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{}`))

	result, err := svc.ForwardCodexNativeImageGenerationBridge(context.Background(), c, account, &OpenAIImagesRequest{Model: "gpt-image-2", Prompt: "draw", N: 1})

	require.Nil(t, result)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "a completed but malformed result must not be regenerated on another account")
	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.NotContains(t, recorder.Body.String(), codexBridgeTestPNG, "partial Responses payload must never reach the Images client")
}

func TestForwardCodexNativeImageGenerationBridgeRawServerFailuresTriggerFailover(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: status,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"temporarily unavailable"}}`)),
			}}
			svc, account, recorder, c := newCodexNativeImageBridgeFailureTest(upstream)
			// This diagnostics key may be normalized by generic forwarding. Bridge
			// failover must instead use the independently captured raw HTTP status.
			c.Set(OpsUpstreamStatusCodeKey, http.StatusBadRequest)

			_, err := svc.ForwardCodexNativeImageGenerationBridge(context.Background(), c, account, &OpenAIImagesRequest{Model: "gpt-image-2", Prompt: "draw", N: 1})

			var failoverErr *UpstreamFailoverError
			require.ErrorAs(t, err, &failoverErr)
			require.Equal(t, status, failoverErr.StatusCode)
			require.Empty(t, recorder.Body.String(), "a retryable response must not be committed before account switch")
		})
	}
}

func TestForwardCodexNativeImageGenerationBridgeContentPolicyNeverFailsOver(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "HTTP 200 response.failed",
			statusCode: http.StatusOK,
			body:       `{"id":"resp_policy","status":"failed","error":{"code":"content_policy_violation","message":"image request rejected by policy"},"output":[]}`,
		},
		{
			name:       "HTTP 400 content policy",
			statusCode: http.StatusBadRequest,
			body:       `{"error":{"type":"invalid_request_error","code":"content_policy_violation","message":"image request rejected by policy"}}`,
		},
		{
			name:       "HTTP 403 content policy",
			statusCode: http.StatusForbidden,
			body:       `{"error":{"type":"invalid_request_error","code":"content_policy_violation","message":"image request rejected by policy"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}}
			svc, account, recorder, c := newCodexNativeImageBridgeFailureTest(upstream)
			// A stale/reclassified ops status must never turn a deterministic policy
			// rejection into a second, potentially billable image generation.
			c.Set(OpsUpstreamStatusCodeKey, http.StatusServiceUnavailable)

			result, err := svc.ForwardCodexNativeImageGenerationBridge(context.Background(), c, account, &OpenAIImagesRequest{Model: "gpt-image-2", Prompt: "draw", N: 1})

			require.Nil(t, result)
			require.Error(t, err)
			var failoverErr *UpstreamFailoverError
			require.False(t, errors.As(err, &failoverErr), "content policy is deterministic and must not be replayed on another account")
			require.NotEmpty(t, recorder.Body.String(), "the deterministic failure must be returned to the client")
			require.NotContains(t, recorder.Body.String(), codexBridgeTestPNG)
		})
	}
}

func newCodexNativeImageBridgeFailureTest(upstream HTTPUpstream) (*OpenAIGatewayService, *Account, *httptest.ResponseRecorder, *gin.Context) {
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{
			Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
		},
	}
	account := &Account{
		ID: 33, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test", "base_url": "https://morecode.example/v1"},
		Extra:       map[string]any{"openai_passthrough": true},
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{}`))
	return svc, account, recorder, c
}

func TestForwardCodexNativeImageGenerationBridgeContentPolicyDoesNotFailover(t *testing.T) {
	failedEvent := `event: response.failed
data: {"type":"response.failed","response":{"status":"failed","error":{"code":"content_policy_violation","message":"content policy rejection"}}}

data: [DONE]

`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(failedEvent)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID: 33, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test", "base_url": "https://morecode.example/v1"},
		Extra:       map[string]any{"openai_passthrough": true},
	}
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{}`))

	_, err := svc.ForwardCodexNativeImageGenerationBridge(context.Background(), c, account, &OpenAIImagesRequest{Model: "gpt-image-2", Prompt: "draw", N: 1})

	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "HTTP 200 content-policy rejection must never regenerate on another account")
	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.Equal(t, http.StatusOK, c.GetInt(openAIRawUpstreamHTTPStatusKey))
}

func TestForwardOpenAIImagesDetachesCanceledClientContext(t *testing.T) {
	body := []byte(`{"model":"gpt-image-1","prompt":"draw a cat","response_format":"b64_json"}`)
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"created":1710000000,"data":[{"b64_json":"aGVsbG8="}],"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{Security: config.SecurityConfig{
			URLAllowlist: config.URLAllowlistConfig{Enabled: false},
		}},
	}
	account := &Account{
		ID:       31,
		Name:     "openai-apikey-images",
		Platform: PlatformOpenAI,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  "sk-test",
			"base_url": "https://api.openai.com/v1",
		},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(body)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := svc.ForwardImages(ctx, c, account, body, parsed, "")

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.NotNil(t, upstream.lastReq)
	require.NoError(t, upstream.lastReq.Context().Err(),
		"upstream image generation must not inherit client cancellation")
}

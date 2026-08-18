package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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
const openAIImagesTestWebP = "UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA"

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

func TestParseOpenAIImagesRequestRejectsAmbiguousFieldTypes(t *testing.T) {
	svc := &OpenAIGatewayService{}
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "fractional n", body: `{"model":"gpt-image-2","prompt":"draw","n":1.5}`, want: "integer"},
		{name: "exponent n", body: `{"model":"gpt-image-2","prompt":"draw","n":1e0}`, want: "integer"},
		{name: "string n", body: `{"model":"gpt-image-2","prompt":"draw","n":"1"}`, want: "n field type"},
		{name: "string stream", body: `{"model":"gpt-image-2","prompt":"draw","stream":"false"}`, want: "stream field type"},
		{name: "object prompt", body: `{"model":"gpt-image-2","prompt":{},"n":1}`, want: "prompt field type"},
		{name: "numeric size", body: `{"model":"gpt-image-2","prompt":"draw","size":1024}`, want: "size field type"},
		{name: "boolean response format", body: `{"model":"gpt-image-2","prompt":"draw","response_format":true}`, want: "response_format field type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ParseOpenAIImagesRequest([]byte(tt.body))
			require.ErrorContains(t, err, tt.want)
		})
	}
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
		"data": [
			{"b64_json":"` + codexBridgeTestPNG + `"},
			{"url":"https://cdn.example.test/image.png"},
			{"b64_json":"not-an-image"}
		]
	}`))
	require.Equal(t, 2, count)
	require.Zero(t, extractOpenAIImageCountFromJSONBytes([]byte(`{"data":[{}, {"url":"javascript:alert(1)"}]}`)))
	require.Equal(t, 1, extractOpenAIImageCountFromJSONBytes([]byte(`{"data":[{"b64_json":"`+openAIImagesTestWebP+`"}]}`)),
		"native Images output_format=webp must remain a valid billable image")
}

func TestForwardOpenAIImagesEmptySuccessDoesNotBillAndSafelyFailsOver(t *testing.T) {
	body := []byte(`{"model":"gpt-image-2","prompt":"draw","n":1}`)
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"created":1710000000,"data":[]}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID: 38, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "test", "base_url": "https://images.example/v1",
			"model_mapping": map[string]any{"gpt-image-2": "gpt-image-2-count"},
		},
		Extra: map[string]any{"supports_images": true},
	}
	parsed, err := svc.ParseOpenAIImagesRequest(body)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))

	result, err := svc.ForwardImages(context.Background(), c, account, body, parsed, "")

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, GatewayFailureReason("native_image_no_output"), failoverErr.Reason)
	require.Empty(t, recorder.Body.String())
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

	staleInner := []byte(`{"status":"completed","output":[{"type":"image_generation_call","status":"generating","result":"` + codexBridgeTestPNG + `"}]}`)
	got, err = extractCompletedCodexImageResult(staleInner)
	require.NoError(t, err)
	require.Equal(t, codexBridgeTestPNG, got)
	require.True(t, codexImageBridgeUsesCompletedOuterGeneratingInner(staleInner))

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

func TestNormalizeAndCountCompletedOpenAIResponseImages(t *testing.T) {
	body := []byte(`{
		"id":"resp_images","status":"completed","output":[
			{"id":"ig_1","type":"image_generation_call","status":"generating","result":"` + codexBridgeTestPNG + `"},
			{"id":"ig_2","type":"image_generation_call","status":"completed","result":"` + openAIImagesTestWebP + `"},
			{"id":"ig_bad","type":"image_generation_call","status":"generating","result":"not-base64"}
		]
	}`)

	normalized, count := normalizeCompletedOpenAIResponseImageStatuses(body)
	require.Equal(t, 1, count)
	require.Equal(t, "completed", gjson.GetBytes(normalized, "output.0.status").String())
	require.Equal(t, "completed", gjson.GetBytes(normalized, "output.1.status").String())
	require.Equal(t, "generating", gjson.GetBytes(normalized, "output.2.status").String(), "malformed output must fail closed")
	require.Equal(t, 2, countCompletedOpenAIResponseImages(normalized))

	event := []byte(`{"type":"response.completed","response":` + string(normalized) + `}`)
	require.Equal(t, 2, countCompletedOpenAIResponseImages(event))
	require.Zero(t, countCompletedOpenAIResponseImages([]byte(`{"status":"in_progress","output":[{"type":"image_generation_call","status":"completed","result":"`+codexBridgeTestPNG+`"}]}`)))
	require.Zero(t, countCompletedOpenAIResponseImages([]byte(`{"status":"completed","output":[{"type":"image_generation_call","status":"in_progress","result":"`+codexBridgeTestPNG+`"}]}`)))
}

func TestForwardResponsesCountsCompletedImagesAcrossHTTPShapes(t *testing.T) {
	completedResponse := `{
		"id":"resp_images","status":"completed","model":"gpt-5.6-sol",
		"output":[
			{"id":"ig_1","type":"image_generation_call","status":"generating","result":"` + codexBridgeTestPNG + `"},
			{"id":"ig_2","type":"image_generation_call","status":"completed","result":"` + openAIImagesTestWebP + `"}
		],
		"usage":{"input_tokens":4,"output_tokens":2}
	}`
	for _, passthrough := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			name := fmt.Sprintf("passthrough_%v_stream_%v", passthrough, stream)
			t.Run(name, func(t *testing.T) {
				contentType := "application/json"
				upstreamBody := completedResponse
				if stream {
					contentType = "text/event-stream"
					var compact bytes.Buffer
					require.NoError(t, json.Compact(&compact, []byte(completedResponse)))
					upstreamBody = "event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":" + compact.String() + "}\n\n"
				}
				upstream := &httpUpstreamRecorder{resp: &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{contentType}},
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
					Extra:       map[string]any{"openai_passthrough": passthrough},
				}
				requestBody := []byte(fmt.Sprintf(`{"model":"gpt-5.6-sol","input":"draw two images","stream":%v}`, stream))
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))

				result, err := svc.Forward(context.Background(), c, account, requestBody)

				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 2, result.ImageCount)
				require.Equal(t, OpenAIFixedImageRendererModel, result.BillingModel)
				require.Equal(t, ImageBillingSize2K, result.ImageSize)
				require.Equal(t, 4, result.Usage.InputTokens)
				require.Contains(t, recorder.Body.String(), `"status":"completed"`)
			})
		}
	}
}

func TestForwardNativeOpenAIImageGenerationResponsesPreservesMultipleImages(t *testing.T) {
	upstreamBody := `{
		"id":"resp_native_images","status":"completed","model":"gpt-5.6-sol",
		"output":[
			{"id":"ig_1","type":"image_generation_call","status":"generating","result":"` + codexBridgeTestPNG + `"},
			{"id":"ig_2","type":"image_generation_call","status":"completed","result":"` + openAIImagesTestWebP + `"}
		],
		"usage":{"input_tokens":5,"output_tokens":3}
	}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
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
	requestBody := []byte(`{"model":"gpt-5.6-sol","input":"draw two images","tools":[{"type":"image_generation"}],"tool_choice":{"type":"image_generation"},"stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))

	result, err := svc.ForwardNativeOpenAIImageGenerationResponses(context.Background(), c, account, requestBody, false)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 2, result.ImageCount)
	require.Equal(t, OpenAIFixedImageRendererModel, result.BillingModel)
	require.Equal(t, ImageBillingSize2K, result.ImageSize)
	require.Len(t, gjson.GetBytes(recorder.Body.Bytes(), "output").Array(), 2)
	require.Equal(t, "completed", gjson.GetBytes(recorder.Body.Bytes(), "output.0.status").String())
}

func TestForwardNativeOpenAIImageGenerationResponsesPreservesStreamingMultipleImages(t *testing.T) {
	completedResponse := `{
		"id":"resp_native_stream_images","status":"completed","model":"gpt-5.6-sol",
		"output":[
			{"id":"ig_1","type":"image_generation_call","status":"generating","result":"` + codexBridgeTestPNG + `"},
			{"id":"ig_2","type":"image_generation_call","status":"completed","result":"` + openAIImagesTestWebP + `"}
		],
		"usage":{"input_tokens":5,"output_tokens":3}
	}`
	var compact bytes.Buffer
	require.NoError(t, json.Compact(&compact, []byte(completedResponse)))
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":" + compact.String() + "}\n\n",
		)),
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
	requestBody := []byte(`{"model":"gpt-5.6-sol","input":"draw two images","tools":[{"type":"image_generation"}],"tool_choice":{"type":"image_generation"},"stream":true}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))

	result, err := svc.ForwardNativeOpenAIImageGenerationResponses(context.Background(), c, account, requestBody, true)

	require.NoError(t, err)
	require.Equal(t, 2, result.ImageCount)
	require.Equal(t, OpenAIFixedImageRendererModel, result.BillingModel)
	require.Equal(t, "/v1/responses", result.UpstreamEndpoint)
	require.Contains(t, recorder.Body.String(), `"type":"response.completed"`)
	require.Contains(t, recorder.Body.String(), `"status":"completed"`)
}

func TestForwardNativeOpenAIImageGenerationResponsesRendersThroughCodexExec(t *testing.T) {
	upstreamBody := `{
		"id":"resp_codex_exec_images","status":"completed","model":"gpt-5.6-sol",
		"output":[
			{"id":"ig_1","type":"image_generation_call","status":"generating","result":"` + codexBridgeTestPNG + `"},
			{"id":"ig_2","type":"image_generation_call","status":"completed","result":"` + openAIImagesTestWebP + `"}
		],
		"usage":{"input_tokens":5,"output_tokens":3,"total_tokens":8}
	}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
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
	requestBody := []byte(`{
		"model":"gpt-5.6-sol","input":"draw two images","stream":true,
		"tools":[{"type":"custom","name":"exec"},{"type":"image_generation"}],
		"tool_choice":{"type":"image_generation"}
	}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))

	result, err := svc.ForwardNativeOpenAIImageGenerationResponses(context.Background(), c, account, requestBody, true)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, 2, result.ImageCount)
	require.True(t, result.Stream)
	require.False(t, gjson.GetBytes(upstream.lastBody, "stream").Bool(), "upstream must be buffered before client-side rendering")
	rendered := recorder.Body.String()
	require.Contains(t, rendered, `"type":"custom_tool_call"`)
	require.Contains(t, rendered, `generatedImage({image_url:`)
	require.Contains(t, rendered, `data:image/png;base64,`)
	require.Contains(t, rendered, `data:image/webp;base64,`)
	// The completed tool item appears once in output_item.done and once in the
	// terminal response envelope. Each copy must carry both generated images.
	require.Equal(t, 4, strings.Count(rendered, "generatedImage({image_url:"))
	require.NotContains(t, rendered, `"type":"image_generation_call"`)
	require.Contains(t, rendered, `"type":"response.completed"`)
}

func TestWriteCodexExecRenderedImageResponsesNonStreaming(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	response := map[string]any{
		"id":     "resp_exec_render",
		"status": "completed",
		"model":  "gpt-5.6-sol",
		"usage":  map[string]any{"input_tokens": 1, "output_tokens": 2, "total_tokens": 3},
	}

	err := writeCodexExecRenderedImageResponses(c, false, response, []codexExecRenderedImage{{
		MediaType: "image/png",
		B64JSON:   codexBridgeTestPNG,
	}})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "custom_tool_call", gjson.GetBytes(recorder.Body.Bytes(), "output.0.type").String())
	require.Equal(t, "exec", gjson.GetBytes(recorder.Body.Bytes(), "output.0.name").String())
	require.Contains(t, gjson.GetBytes(recorder.Body.Bytes(), "output.0.input").String(), "generatedImage({image_url:")
	require.Equal(t, int64(3), gjson.GetBytes(recorder.Body.Bytes(), "usage.total_tokens").Int())
}

func TestForwardNativeOpenAIImageGenerationResponsesEmptyOutputSafelyFailsOver(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_empty","status":"completed","model":"gpt-5.6-sol","output":[],"usage":{"input_tokens":0,"output_tokens":0,"total_tokens":0}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{
			Gateway:  config.GatewayConfig{Pipeline: config.GatewayPipelineConfig{OpenAIResponsesEnabled: false}},
			Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
		},
	}
	account := &Account{
		ID: 33, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://morecode.example/v1"},
	}
	requestBody := []byte(`{"model":"gpt-5.6-sol","input":"draw","stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))

	result, err := svc.ForwardNativeOpenAIImageGenerationResponses(context.Background(), c, account, requestBody, false)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, GatewayFailureReason("image_tool_not_invoked"), failoverErr.Reason)
	require.Empty(t, recorder.Body.String(), "safe failover must not commit an empty upstream response")
}

func TestForwardNativeOpenAIImageGenerationResponsesRejectsMixedValidAndMalformedImages(t *testing.T) {
	upstreamBody := `{
		"id":"resp_mixed_images","status":"completed","model":"gpt-5.6-sol",
		"output":[
			{"id":"ig_ok","type":"image_generation_call","status":"completed","result":"` + codexBridgeTestPNG + `"},
			{"id":"ig_bad","type":"image_generation_call","status":"completed","result":"not-base64"}
		],
		"usage":{"input_tokens":5,"output_tokens":3}
	}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
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
		ID: 33, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://morecode.example/v1"},
	}
	requestBody := []byte(`{"model":"gpt-5.6-sol","input":"draw","stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(requestBody))

	result, err := svc.ForwardNativeOpenAIImageGenerationResponses(context.Background(), c, account, requestBody, false)

	require.Nil(t, result)
	require.ErrorContains(t, err, "malformed")
	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.NotContains(t, recorder.Body.String(), codexBridgeTestPNG)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr), "a possibly billed mixed output must never be regenerated")
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

func TestForwardCodexNativeImageGenerationBridgeAcceptsCompletedOuterWithStaleGeneratingInner(t *testing.T) {
	upstreamBody := `{
		"id":"resp_morecode","status":"completed","model":"gpt-5.6-sol",
		"output":[{"type":"image_generation_call","id":"ig_1","status":"generating","result":"` + codexBridgeTestPNG + `"}],
		"usage":{"input_tokens":8,"output_tokens":2}
	}`
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
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
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{}`))

	result, err := svc.ForwardCodexNativeImageGenerationBridge(
		context.Background(), c, account,
		&OpenAIImagesRequest{Model: "gpt-image-2", Prompt: "draw breakfast", N: 1, SizeTier: "2K"},
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, codexBridgeTestPNG, gjson.GetBytes(recorder.Body.Bytes(), "data.0.b64_json").String())
	require.Equal(t, 1, result.ImageCount)
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

func TestForwardFixedOpenAIImageGenerationResponsesPreservesNativeProtocolAcrossTransports(t *testing.T) {
	tests := []struct {
		name              string
		stream            bool
		account           *Account
		upstreamBody      string
		wantUpstreamPath  string
		wantUpstreamModel string
	}{
		{
			name:   "responses primary returns non streaming response",
			stream: false,
			account: &Account{
				ID: 33, Name: "responses-image-primary", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: map[string]any{"api_key": "test", "base_url": "https://responses.example/v1"},
				Extra: map[string]any{
					OpenAIImageGenerationPriorityExtraKey: 1,
					OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
				},
			},
			upstreamBody:      `{"id":"resp_upstream","status":"completed","model":"gpt-5.6-sol","output":[{"id":"ig_upstream","type":"image_generation_call","status":"completed","result":"` + codexBridgeTestPNG + `"}],"usage":{"input_tokens":7,"output_tokens":9}}`,
			wantUpstreamPath:  "/v1/responses",
			wantUpstreamModel: "gpt-5.6-sol",
		},
		{
			name:   "native images fallback returns streaming response",
			stream: true,
			account: &Account{
				ID: 38, Name: "native-image-fallback", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Credentials: map[string]any{
					"api_key": "test", "base_url": "https://images.example/v1",
					"model_mapping": map[string]any{"gpt-image-2": "gpt-image-2-count"},
				},
				Extra: map[string]any{
					"supports_images":                      true,
					OpenAIImageGenerationPriorityExtraKey:  2,
					OpenAIImageGenerationModelsExtraKey:    []any{"gpt-image-2"},
					OpenAIImageGenerationTransportExtraKey: OpenAIImageGenerationTransportImages,
				},
			},
			upstreamBody:      `{"created":1710000000,"data":[{"b64_json":"` + codexBridgeTestPNG + `"}],"usage":{"total_tokens":30}}`,
			wantUpstreamPath:  "/v1/images/generations",
			wantUpstreamModel: "gpt-image-2-count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(tt.upstreamBody)),
			}}
			svc := &OpenAIGatewayService{
				httpUpstream: upstream,
				cfg: &config.Config{
					Gateway: config.GatewayConfig{
						Pipeline: config.GatewayPipelineConfig{OpenAIResponsesEnabled: false},
						CodexImagePreview: config.CodexImagePreviewConfig{
							Enabled: true, DataDir: t.TempDir(), TTLSeconds: 3600, MaxImageBytes: 1024 * 1024,
						},
					},
					Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
				},
			}
			body := []byte(`{"model":"gpt-5.6-luna","input":"帮我生成一张月球橘猫的图片","stream":` + fmt.Sprint(tt.stream) + `}`)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
			c.Request.Header.Set("User-Agent", "codex_cli_rs/1.0")

			result, err := svc.ForwardFixedOpenAIImageGenerationResponses(
				context.Background(), c, tt.account, body, "gpt-5.6-luna", tt.stream,
			)

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, "gpt-5.6-luna", result.Model)
			require.Equal(t, OpenAIFixedImageRendererModel, result.BillingModel)
			require.Equal(t, 1, result.ImageCount)
			require.Equal(t, ImageBillingSize2K, result.ImageSize)
			require.Equal(t, tt.stream, result.Stream)
			require.Equal(t, tt.wantUpstreamPath, upstream.lastReq.URL.Path)
			require.Equal(t, tt.wantUpstreamModel, gjson.GetBytes(upstream.lastBody, "model").String())

			if !tt.stream {
				require.Equal(t, "response", gjson.GetBytes(recorder.Body.Bytes(), "object").String())
				require.Equal(t, "completed", gjson.GetBytes(recorder.Body.Bytes(), "status").String())
				require.Equal(t, "gpt-5.6-luna", gjson.GetBytes(recorder.Body.Bytes(), "model").String())
				require.Equal(t, "message", gjson.GetBytes(recorder.Body.Bytes(), "output.0.type").String())
				text := gjson.GetBytes(recorder.Body.Bytes(), "output.0.content.0.text").String()
				require.Contains(t, text, "![生成的图片](https://example.com/v1/codex-image/preview?token=")
				require.Contains(t, text, "点击这里打开原图")
				require.NotContains(t, recorder.Body.String(), codexBridgeTestPNG,
					"the response must carry a short URL instead of multi-megabyte Base64")
				require.Equal(t, "null", gjson.GetBytes(recorder.Body.Bytes(), "usage").Raw)
				return
			}

			eventTypes := make([]string, 0, 8)
			var addedPayload []byte
			var completedPayload []byte
			for _, line := range strings.Split(recorder.Body.String(), "\n") {
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				payload := []byte(strings.TrimPrefix(line, "data: "))
				eventType := gjson.GetBytes(payload, "type").String()
				eventTypes = append(eventTypes, eventType)
				if eventType == "response.output_item.added" {
					addedPayload = payload
				}
				if eventType == "response.completed" {
					completedPayload = payload
				}
			}
			require.Equal(t, []string{
				"response.created",
				"response.in_progress",
				"response.output_item.added",
				"response.content_part.added",
				"response.output_text.delta",
				"response.output_text.done",
				"response.content_part.done",
				"response.output_item.done",
				"response.completed",
			}, eventTypes)
			require.Equal(t, "message", gjson.GetBytes(addedPayload, "item.type").String())
			text := gjson.GetBytes(completedPayload, "response.output.0.content.0.text").String()
			require.Contains(t, text, "![生成的图片](https://example.com/v1/codex-image/preview?token=")
			require.Contains(t, text, "点击这里打开原图")
			require.NotContains(t, string(completedPayload), codexBridgeTestPNG)
			require.Equal(t, "null", gjson.GetBytes(completedPayload, "response.usage").Raw)
		})
	}
}

func TestForwardFixedOpenAIImageGenerationResponsesUsesMarkdownEvenWhenExecIsAdvertised(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"created":1710000000,"data":[{"b64_json":"` + codexBridgeTestPNG + `"}],"usage":{"total_tokens":30}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{
			Gateway: config.GatewayConfig{CodexImagePreview: config.CodexImagePreviewConfig{
				Enabled: true, DataDir: t.TempDir(), TTLSeconds: 3600, MaxImageBytes: 1024 * 1024,
			}},
			Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
		},
	}
	account := &Account{
		ID: 38, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "test", "base_url": "https://images.example/v1",
			"model_mapping": map[string]any{"gpt-image-2": "gpt-image-2-count"},
		},
		Extra: map[string]any{
			"supports_images":                      true,
			OpenAIImageGenerationPriorityExtraKey:  2,
			OpenAIImageGenerationModelsExtraKey:    []any{"gpt-image-2"},
			OpenAIImageGenerationTransportExtraKey: OpenAIImageGenerationTransportImages,
		},
	}
	body := []byte(`{
		"model":"gpt-5.6-sol","input":"生成一张图片","stream":true,
		"tools":[{"type":"custom","name":"exec"},{"type":"image_generation"}]
	}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	result, err := svc.ForwardFixedOpenAIImageGenerationResponses(context.Background(), c, account, body, "gpt-5.6-sol", true)

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.Contains(t, recorder.Body.String(), `"type":"message"`)
	require.Contains(t, recorder.Body.String(), `![生成的图片](https://example.com/v1/codex-image/preview?token=`)
	require.Contains(t, recorder.Body.String(), `点击这里打开原图`)
	require.NotContains(t, recorder.Body.String(), `"type":"custom_tool_call"`)
	require.NotContains(t, recorder.Body.String(), `"type":"image_generation_call"`)
}

func TestForwardFixedOpenAIImageGenerationResponsesStorageFailureIsHonestAndNonReplayable(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"resp_paid","object":"response","status":"completed","model":"gpt-5.6-sol","output":[{"id":"ig_paid","type":"image_generation_call","status":"completed","result":"` + codexBridgeTestPNG + `"}],"usage":{"input_tokens":7,"output_tokens":9,"total_tokens":16}}`,
		)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg: &config.Config{
			Gateway:  config.GatewayConfig{CodexImagePreview: config.CodexImagePreviewConfig{Enabled: false}},
			Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}},
		},
	}
	account := &Account{
		ID: 33, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{"api_key": "test", "base_url": "https://responses.example/v1"},
		Extra: map[string]any{
			OpenAIImageGenerationPriorityExtraKey: 1,
			OpenAIImageGenerationModelsExtraKey:   []any{"gpt-5.6-sol"},
		},
	}
	body := []byte(`{"model":"gpt-5.6-sol","input":"生成一张图片","stream":false}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	result, err := svc.ForwardFixedOpenAIImageGenerationResponses(context.Background(), c, account, body, "gpt-5.6-sol", false)

	require.NoError(t, err, "a paid image must not be exposed to account failover after delivery storage fails")
	require.NotNil(t, result)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, "message", gjson.GetBytes(recorder.Body.Bytes(), "output.0.type").String())
	require.Contains(t, gjson.GetBytes(recorder.Body.Bytes(), "output.0.content.0.text").String(), "不会自动重试")
	require.NotContains(t, recorder.Body.String(), codexBridgeTestPNG)
	require.NotNil(t, upstream.lastReq)
}

func TestForwardFixedOpenAIImageGenerationResponsesDoesNotCommitBeforeSafeFailover(t *testing.T) {
	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"created":1710000000,"data":[]}`)),
	}}
	svc := &OpenAIGatewayService{
		httpUpstream: upstream,
		cfg:          &config.Config{Security: config.SecurityConfig{URLAllowlist: config.URLAllowlistConfig{Enabled: false}}},
	}
	account := &Account{
		ID: 38, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key": "test", "base_url": "https://images.example/v1",
			"model_mapping": map[string]any{"gpt-image-2": "gpt-image-2-count"},
		},
		Extra: map[string]any{
			"supports_images":                      true,
			OpenAIImageGenerationPriorityExtraKey:  2,
			OpenAIImageGenerationModelsExtraKey:    []any{"gpt-image-2"},
			OpenAIImageGenerationTransportExtraKey: OpenAIImageGenerationTransportImages,
		},
	}
	body := []byte(`{"model":"gpt-5.6-sol","input":"生成一张图片","stream":true}`)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))

	result, err := svc.ForwardFixedOpenAIImageGenerationResponses(context.Background(), c, account, body, "gpt-5.6-sol", true)

	require.Nil(t, result)
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Empty(t, recorder.Body.String(), "no Responses bytes may be written before the next account is selected")
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

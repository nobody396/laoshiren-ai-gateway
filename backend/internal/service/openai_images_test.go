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
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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

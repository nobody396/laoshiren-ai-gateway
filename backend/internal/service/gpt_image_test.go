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
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gptImageForwardHTTPUpstreamStub struct {
	lastReq  *http.Request
	lastBody []byte
	resp     *http.Response
	err      error
}

func (s *gptImageForwardHTTPUpstreamStub) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	s.lastReq = req
	if req != nil && req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		s.lastBody = b
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(b))
	}
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func (s *gptImageForwardHTTPUpstreamStub) DoWithTLS(req *http.Request, proxyURL string, accountID int64, accountConcurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return s.Do(req, proxyURL, accountID, accountConcurrency)
}

func TestParseGPTImageRequestDefaultsModelAndN(t *testing.T) {
	svc := &OpenAIGatewayService{}

	req, err := svc.ParseGPTImageRequest([]byte(`{
		"prompt":"draw a cat",
		"size":"1:1"
	}`))

	require.NoError(t, err)
	require.Equal(t, "gpt-image-2", req.Model)
	require.Equal(t, 1, req.N)
	require.Equal(t, "1K", req.Resolution)
	require.JSONEq(t, `{
		"prompt":"draw a cat",
		"size":"1:1",
		"resolution":"1k",
		"model":"gpt-image-2",
		"n":1
	}`, string(req.Body))
}

func TestParseGPTImageRequestRejectsUnsupportedModelAndN(t *testing.T) {
	svc := &OpenAIGatewayService{}

	_, err := svc.ParseGPTImageRequest([]byte(`{"model":"gpt-image-1","prompt":"x"}`))
	require.ErrorContains(t, err, `only supports model "gpt-image-2"`)

	_, err = svc.ParseGPTImageRequest([]byte(`{"model":"gpt-image-2","prompt":"x","n":2}`))
	require.ErrorContains(t, err, "only supports n=1")
}

func TestParseGPTImageRequestNormalizesResolutionAndValidates4KSize(t *testing.T) {
	svc := &OpenAIGatewayService{}

	req, err := svc.ParseGPTImageRequest([]byte(`{
		"model":"gpt-image-2",
		"prompt":"x",
		"size":"16:9",
		"resolution":"4K"
	}`))

	require.NoError(t, err)
	require.Equal(t, "4K", req.Resolution)
	require.JSONEq(t, `{
		"model":"gpt-image-2",
		"prompt":"x",
		"size":"16:9",
		"resolution":"4k",
		"n":1
	}`, string(req.Body))

	_, err = svc.ParseGPTImageRequest([]byte(`{
		"model":"gpt-image-2",
		"prompt":"x",
		"size":"1:1",
		"resolution":"4k"
	}`))
	require.ErrorContains(t, err, "resolution 4k only supports size")
}

func TestParseGPTImageRequestValidatesReferenceImages(t *testing.T) {
	svc := &OpenAIGatewayService{}

	req, err := svc.ParseGPTImageRequest([]byte(`{
		"model":"gpt-image-2",
		"prompt":"x",
		"image_urls":["data:image/png;base64,aGVsbG8="]
	}`))
	require.NoError(t, err)
	require.Contains(t, string(req.Body), `"image_urls"`)

	_, err = svc.ParseGPTImageRequest([]byte(`{
		"model":"gpt-image-2",
		"prompt":"x",
		"image_urls":"https://example.com/a.png"
	}`))
	require.ErrorContains(t, err, "image_urls must be an array")

	_, err = svc.ParseGPTImageRequest([]byte(`{
		"model":"gpt-image-2",
		"prompt":"x",
		"image_urls":["https://127.0.0.1/a.png"]
	}`))
	require.ErrorContains(t, err, "host is not allowed")

	_, err = svc.ParseGPTImageRequest([]byte(`{
		"model":"gpt-image-2",
		"prompt":"x",
		"image_urls":["http://example.com/a.png"]
	}`))
	require.ErrorContains(t, err, "invalid url scheme")

	_, err = svc.ParseGPTImageRequest([]byte(`{
		"model":"gpt-image-2",
		"prompt":"x",
		"image_urls":["data:text/plain;base64,aGVsbG8="]
	}`))
	require.ErrorContains(t, err, "data URI must be image/* base64")
}

func TestParseGPTImageRequestRejectsTooManyReferenceImagesAndLargeDataURI(t *testing.T) {
	svc := &OpenAIGatewayService{cfg: &config.Config{}}
	svc.cfg.Gateway.GPTImageS3.MaxImageBytes = 3

	_, err := svc.ParseGPTImageRequest([]byte(`{
		"model":"gpt-image-2",
		"prompt":"x",
		"image_urls":["data:image/png;base64,aGVsbG8="]
	}`))
	require.ErrorContains(t, err, "data URI image exceeds max 3 bytes")

	_, err = svc.ParseGPTImageRequest([]byte(`{
		"model":"gpt-image-2",
		"prompt":"x",
		"image_urls":[
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA==",
			"data:image/png;base64,aA=="
		]
	}`))
	require.ErrorContains(t, err, "image_urls exceeds max 16")
}

func TestExtractGPTImageSubmittedTaskIDs(t *testing.T) {
	taskIDs := extractGPTImageSubmittedTaskIDs([]byte(`{
		"code":200,
		"data":[{"status":"submitted","task_id":"task_123"}]
	}`))

	require.Equal(t, []string{"task_123"}, taskIDs)
}

func TestForwardGPTImageSubmittedTaskDoesNotSetImageCount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"code":200,"data":[{"status":"submitted","task_id":"task_123"}]}`)),
	}
	upstream := &gptImageForwardHTTPUpstreamStub{resp: resp}
	svc := &OpenAIGatewayService{httpUpstream: upstream, cfg: &config.Config{}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	result, err := svc.ForwardGPTImage(context.Background(), c, &Account{
		ID:       1,
		Platform: PlatformGPTImage,
		Type:     AccountTypeUpstream,
		Credentials: map[string]any{
			"base_url": "https://api.apimart.ai",
			"api_key":  "test-key",
		},
	}, &GPTImageRequest{
		Model:      "gpt-image-2",
		Resolution: "2K",
		Body:       []byte(`{"model":"gpt-image-2","prompt":"x","resolution":"2k"}`),
	})

	require.NoError(t, err)
	require.Equal(t, 0, result.ImageCount)
	require.Equal(t, "2K", result.ImageSize)
	require.Equal(t, []string{"task_123"}, result.GPTImageTaskIDs)
}

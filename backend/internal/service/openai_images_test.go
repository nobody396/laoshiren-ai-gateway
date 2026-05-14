package service

import (
	"testing"

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

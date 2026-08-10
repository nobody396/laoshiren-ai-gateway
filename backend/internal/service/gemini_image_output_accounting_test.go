package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const geminiImageTestPNG = "iVBORw0KGgoAAAANSUhEUg=="

func newGeminiImageAccountingContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/custom-image:generateContent", strings.NewReader("{}"))
	return c
}

func geminiImageResponse(parts string) string {
	return `{"candidates":[{"content":{"role":"model","parts":[` + parts + `]},"finishReason":"STOP"}]}`
}

func TestCountGeminiInlineImageOutputs(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    int
	}{
		{"camelCase", geminiImageResponse(`{"inlineData":{"mimeType":"image/png","data":"` + geminiImageTestPNG + `"}}`), 1},
		{"snake_case", geminiImageResponse(`{"inline_data":{"mime_type":"image/jpeg","data":"` + geminiImageTestPNG + `"}}`), 1},
		{"multiple", geminiImageResponse(`{"inlineData":{"mimeType":"image/png","data":"` + geminiImageTestPNG + `"}},{"inlineData":{"mimeType":"image/webp","data":"` + geminiImageTestPNG + `"}}`), 2},
		{"text only", geminiImageResponse(`{"text":"hello"}`), 0},
		{"non image", geminiImageResponse(`{"inlineData":{"mimeType":"audio/mpeg","data":"` + geminiImageTestPNG + `"}}`), 0},
		{"empty data", geminiImageResponse(`{"inlineData":{"mimeType":"image/png","data":""}}`), 0},
		{"invalid", "not-json", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, countGeminiInlineImageOutputs([]byte(tt.payload)))
		})
	}
}

func TestGeminiImageObservationUsesLargestPayloadAndResets(t *testing.T) {
	c := newGeminiImageAccountingContext(t)
	one := []byte(geminiImageResponse(`{"inlineData":{"mimeType":"image/png","data":"` + geminiImageTestPNG + `"}}`))
	two := []byte(geminiImageResponse(`{"inlineData":{"mimeType":"image/png","data":"` + geminiImageTestPNG + `"}},{"inlineData":{"mimeType":"image/png","data":"` + geminiImageTestPNG + `"}}`))

	beginGeminiImageOutputObservation(c)
	observeGeminiImageOutputs(c, one)
	observeGeminiImageOutputs(c, one)
	observeGeminiImageOutputs(c, two)
	require.Equal(t, 2, observedGeminiImageOutputs(c))

	beginGeminiImageOutputObservation(c)
	require.Zero(t, observedGeminiImageOutputs(c))
}

func TestResolveGeminiImageCountPrefersObservedOutput(t *testing.T) {
	c := newGeminiImageAccountingContext(t)
	beginGeminiImageOutputObservation(c)
	observeGeminiImageOutputs(c, []byte(geminiImageResponse(`{"inlineData":{"mimeType":"image/png","data":"`+geminiImageTestPNG+`"}}`)))

	require.Equal(t, 1, resolveGeminiImageCount(c, "custom-alias", "custom-upstream-name"))
	beginGeminiImageOutputObservation(c)
	require.Equal(t, 1, resolveGeminiImageCount(c, "custom-alias", "gemini-2.5-flash-image"))
	require.Zero(t, resolveGeminiImageCount(c, "gemini-2.5-pro", "gemini-2.5-pro"))
}

func TestHandleNativeNonStreamingResponseFeedsImageCounter(t *testing.T) {
	c := newGeminiImageAccountingContext(t)
	beginGeminiImageOutputObservation(c)
	body := geminiImageResponse(`{"inlineData":{"mimeType":"image/png","data":"` + geminiImageTestPNG + `"}}`)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	usage, err := (&GeminiMessagesCompatService{}).handleNativeNonStreamingResponse(c, resp, false)
	require.NoError(t, err)
	require.NotNil(t, usage)
	require.Equal(t, 1, resolveGeminiImageCount(c, "custom-alias", "custom-upstream-name"))
}

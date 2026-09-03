package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type gemini38FailedReader struct{ err error }

func (r gemini38FailedReader) Read([]byte) (int, error) { return 0, r.err }
func (r gemini38FailedReader) Close() error             { return nil }

// Pre-admission evidence for the unchanged native adapter, not upstream SLA.
func TestGemini38PreAdmissionReadFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GeminiMessagesCompatService{}
	for _, failure := range []error{io.ErrUnexpectedEOF, context.DeadlineExceeded, context.Canceled} {
		t.Run(failure.Error(), func(t *testing.T) {
			writer := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(writer)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/gemini-3.8-flash:streamGenerateContent", nil)
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: gemini38FailedReader{failure}}
			result, err := svc.handleNativeStreamingResponse(c, resp, time.Now(), false)
			require.Nil(t, result)
			require.True(t, errors.Is(err, failure))
		})
	}
}

func TestGemini38PreAdmissionRetryPolicy(t *testing.T) {
	svc := &GeminiMessagesCompatService{}
	account := &Account{Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-3.8-flash": "gemini-3.8-flash"}}}
	for _, status := range []int{429, 500, 502, 503, 504, 529} {
		require.True(t, svc.shouldRetryGeminiUpstreamError(account, status))
		require.True(t, svc.shouldFailoverGeminiUpstreamError(status))
	}
	for _, status := range []int{400, 401, 403, 404} {
		require.False(t, svc.shouldRetryGeminiUpstreamError(account, status))
	}
}

func TestGemini38PreAdmissionUsageSemantics(t *testing.T) {
	// Cached tokens are separated from fresh input; thinking joins output.
	usage := extractGeminiUsage([]byte(`{"modelVersion":"gemini-3.8-flash","usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":30,"candidatesTokenCount":20,"thoughtsTokenCount":50,"totalTokenCount":170}}`))
	require.NotNil(t, usage)
	require.Equal(t, 70, usage.InputTokens)
	require.Equal(t, 30, usage.CacheReadInputTokens)
	require.Equal(t, 70, usage.OutputTokens)
}

func TestGemini38CatalogBilling(t *testing.T) {
	svc := NewBillingService(&config.Config{}, nil)
	price, priceErr := svc.GetModelPricing("gemini-3.8-flash")
	require.NoError(t, priceErr)
	require.NotNil(t, price)
	tokens := UsageTokens{InputTokens: 70, OutputTokens: 70, CacheReadTokens: 30}
	cost, err := svc.CalculateCost("gemini-3.8-flash", tokens, 0.6)
	require.NoError(t, err)
	require.InDelta(t, 70*0.75/1e6, cost.InputCost, 1e-12)
	require.InDelta(t, 70*3.75/1e6, cost.OutputCost, 1e-12)
	require.InDelta(t, 30*0.075/1e6, cost.CacheReadCost, 1e-12)
	require.InDelta(t, 0.00031725, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.00019035, cost.ActualCost, 1e-12)
	// Flash has no 200K-context surcharge; no Pro tariff inheritance.
	large, err := svc.CalculateCost("gemini-3.8-flash", UsageTokens{InputTokens: 300000, OutputTokens: 10}, 0.6)
	require.NoError(t, err)
	require.InDelta(t, 0.225, large.InputCost, 1e-12)
}

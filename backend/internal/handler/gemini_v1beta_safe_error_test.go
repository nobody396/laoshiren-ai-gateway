package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestWriteUpstreamResponse_SanitizesUpstreamErrorsWithRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1beta/models", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-gemini-models"))

	writeUpstreamResponse(c, &service.UpstreamHTTPResult{
		StatusCode: http.StatusInternalServerError,
		Headers:    http.Header{"Content-Type": []string{"application/json"}},
		Body:       []byte(`{"error":{"message":"upstream account quota exceeded at provider"}}`),
	})

	require.Equal(t, http.StatusBadGateway, rec.Code)
	require.Contains(t, rec.Body.String(), service.ClientMessageServiceUnavailable)
	require.Contains(t, rec.Body.String(), `"request_id":"req-gemini-models"`)
	require.NotContains(t, rec.Body.String(), "upstream account quota")
	require.NotContains(t, rec.Body.String(), "provider")
}

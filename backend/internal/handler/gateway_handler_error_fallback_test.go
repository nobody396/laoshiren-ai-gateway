package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGatewayEnsureForwardErrorResponse_WritesFallbackWhenNotWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false)

	require.True(t, wrote)
	require.Equal(t, http.StatusBadGateway, w.Code)

	var parsed map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "error", parsed["type"])
	errorObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "api_error", errorObj["type"])
	assert.Equal(t, service.ClientMessageServiceUnavailable, errorObj["message"])
}

func TestGatewayHandleAccountSelectionError_ModelNotSupported(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointMessages, nil)

	h := &GatewayHandler{}
	handled := h.handleAccountSelectionError(c, &service.ModelNotSupportedError{
		RequestedModel: "claude-opus-5",
		Platform:       service.PlatformAnthropic,
	}, false)

	require.True(t, handled)
	require.Equal(t, http.StatusBadRequest, w.Code)
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &parsed))
	errorObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "invalid_request_error", errorObj["type"])
	require.Contains(t, errorObj["message"], "claude-opus-5")
	require.Contains(t, errorObj["message"], service.ModelPricingPageURL)
}

// Writer 已写后 ensureForwardErrorResponse 必须把错误以 SSE 形式追加，
// 而不是 silent EOF。非 /responses 路径走 legacy data:{"type":"error"} 分支。
func TestGatewayEnsureForwardErrorResponse_AppendsSSEAfterWritten(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-gateway-stream"))
	c.String(http.StatusTeapot, "already written")

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false)

	require.True(t, wrote)
	require.Equal(t, http.StatusTeapot, w.Code)
	assert.Contains(t, w.Body.String(), "already written")
	assert.Contains(t, w.Body.String(), `data: `)

	idx := strings.LastIndex(w.Body.String(), "data: ")
	require.NotEqual(t, -1, idx)
	dataLine := strings.TrimSuffix(w.Body.String()[idx:], "\n\n")
	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimPrefix(dataLine, "data: ")), &parsed))
	errorObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "req-gateway-stream", errorObj["request_id"])
}

// case B 回归：Anthropic-backed /responses，Writer 已被写过时
// ensureForwardErrorResponse 仍要发 response.failed。
func TestGatewayEnsureForwardErrorResponse_ResponsesRouteAfterWrittenEmitsResponseFailed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointResponses, nil)
	_, _ = c.Writer.WriteString(":\n\n")

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false)

	require.True(t, wrote)
	body := w.Body.String()
	assert.Contains(t, body, ":\n\n")
	assert.Contains(t, body, "event: response.failed\n")
	assert.Contains(t, body, `"type":"response.failed"`)
}

func TestGatewayEnsureForwardErrorResponse_SkipsCommittedResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, EndpointResponses, nil)
	c.JSON(http.StatusBadRequest, gin.H{"type": "error", "error": gin.H{"message": "upstream json"}})
	service.MarkResponseCommitted(c)

	h := &GatewayHandler{}
	wrote := h.ensureForwardErrorResponse(c, false)

	require.False(t, wrote)
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, w.Body.String(), "response.failed")
	assert.Contains(t, w.Body.String(), "upstream json")
}

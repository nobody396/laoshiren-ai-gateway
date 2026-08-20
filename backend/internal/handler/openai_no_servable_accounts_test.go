package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestHandleOpenAINoServableAccountsError_ClassifiedAsClient 锁定端点/平台结构性
// 不匹配时的响应：400 + invalid_request_error，且 ops 错误日志按 request 阶段
// 归类为客户端误用（error_owner=client），不计入平台成功率告警。
func TestHandleOpenAINoServableAccountsError_ClassifiedAsClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	h := &OpenAIGatewayHandler{}
	h.handleOpenAINoServableAccountsError(c, "gemini-3.1-pro", false)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := rec.Body.Bytes()
	require.Contains(t, string(body), "gemini-3.1-pro")
	require.Contains(t, string(body), "model_not_supported")
	require.Contains(t, string(body), "is not supported on this endpoint for this group")

	parsed := parseOpsErrorResponse(body)
	require.Equal(t, "invalid_request_error", parsed.ErrorType)

	normalizedType := normalizeOpsErrorType(parsed.ErrorType, parsed.Code)
	phase := classifyOpsPhase(normalizedType, parsed.Message, parsed.Code)
	require.Equal(t, "request", phase)
	require.Equal(t, "client", classifyOpsErrorOwner(phase, parsed.Message))
	require.Equal(t, "client_request", classifyOpsErrorSource(phase, parsed.Message))
}

// TestHandleAnthropicNoServableAccountsError_ClassifiedAsClient 锁定 Anthropic
// 格式端点（/v1/messages dispatch）的结构性不可服务响应：400 +
// invalid_request_error，ops 同样归类为客户端误用。
func TestHandleAnthropicNoServableAccountsError_ClassifiedAsClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	h := &OpenAIGatewayHandler{}
	h.handleAnthropicNoServableAccountsError(c, "gemini-3.7-flash", false)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	body := rec.Body.Bytes()
	require.Contains(t, string(body), "gemini-3.7-flash")
	require.Contains(t, string(body), "is not supported on this endpoint for this group")

	parsed := parseOpsErrorResponse(body)
	require.Equal(t, "invalid_request_error", parsed.ErrorType)

	normalizedType := normalizeOpsErrorType(parsed.ErrorType, parsed.Code)
	phase := classifyOpsPhase(normalizedType, parsed.Message, parsed.Code)
	require.Equal(t, "request", phase)
	require.Equal(t, "client", classifyOpsErrorOwner(phase, parsed.Message))
	require.Equal(t, "client_request", classifyOpsErrorSource(phase, parsed.Message))
}

// TestOpenAISelectionFailure503_StaysPlatformOwned 锁定“暂时性不可用”语义：
// 账号存在但不可调度时仍返回 503 api_error，ops 归类保持 platform，
// 继续计入平台成功率告警。
func TestOpenAISelectionFailure503_StaysPlatformOwned(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	h := &OpenAIGatewayHandler{}
	h.handleStreamingAwareError(c, http.StatusServiceUnavailable, "api_error", "Service temporarily unavailable", false)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)

	parsed := parseOpsErrorResponse(rec.Body.Bytes())
	normalizedType := normalizeOpsErrorType(parsed.ErrorType, parsed.Code)
	phase := classifyOpsPhase(normalizedType, parsed.Message, parsed.Code)
	require.Equal(t, "internal", phase)
	require.Equal(t, "platform", classifyOpsErrorOwner(phase, parsed.Message))
}

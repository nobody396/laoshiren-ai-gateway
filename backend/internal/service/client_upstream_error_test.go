package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeClientUpstreamError(t *testing.T) {
	tests := []struct {
		name           string
		upstreamStatus int
		wantStatus     int
		wantType       string
		wantMessage    string
		wantCode       string
	}{
		{
			name:           "invalid request",
			upstreamStatus: http.StatusUnprocessableEntity,
			wantStatus:     http.StatusBadRequest,
			wantType:       "invalid_request_error",
			wantMessage:    ClientMessageRequestFailed,
		},
		{
			name:           "request body too large",
			upstreamStatus: http.StatusRequestEntityTooLarge,
			wantStatus:     http.StatusRequestEntityTooLarge,
			wantType:       "invalid_request_error",
			wantMessage:    ClientMessageRequestBodyTooLarge,
			wantCode:       ClientCodeRequestBodyTooLarge,
		},
		{
			name:           "upstream auth hidden as service unavailable",
			upstreamStatus: http.StatusForbidden,
			wantStatus:     http.StatusServiceUnavailable,
			wantType:       "api_error",
			wantMessage:    ClientMessageServiceUnavailable,
		},
		{
			name:           "rate limited",
			upstreamStatus: http.StatusTooManyRequests,
			wantStatus:     http.StatusTooManyRequests,
			wantType:       "rate_limit_error",
			wantMessage:    ClientMessageServiceBusy,
		},
		{
			name:           "overloaded",
			upstreamStatus: 529,
			wantStatus:     http.StatusServiceUnavailable,
			wantType:       "overloaded_error",
			wantMessage:    ClientMessageServiceBusy,
		},
		{
			name:           "generic failure",
			upstreamStatus: http.StatusBadGateway,
			wantStatus:     http.StatusBadGateway,
			wantType:       "api_error",
			wantMessage:    ClientMessageServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SafeClientUpstreamError(tt.upstreamStatus)
			assert.Equal(t, tt.wantStatus, got.StatusCode)
			assert.Equal(t, tt.wantType, got.Type)
			assert.Equal(t, tt.wantMessage, got.Message)
			assert.Equal(t, tt.wantCode, got.Code)
		})
	}
}

func TestSafeOpenAIClientUpstreamError_ContextWindowExceeded(t *testing.T) {
	tests := []string{
		"Your input exceeds the context window of this model.",
		"context_length_exceeded",
		"This model's maximum context length is 250000 tokens.",
		"请求上下文过长，请减少消息数量",
	}

	for _, upstreamError := range tests {
		t.Run(upstreamError, func(t *testing.T) {
			got := SafeOpenAIClientUpstreamError(http.StatusBadGateway, upstreamError)
			assert.Equal(t, http.StatusBadRequest, got.StatusCode)
			assert.Equal(t, "invalid_request_error", got.Type)
			assert.Equal(t, ClientCodeContextWindowExceeded, got.Code)
			assert.Equal(t, ClientMessageContextWindowExceeded, got.Message)
			assert.NotContains(t, got.Message, "try again later")
		})
	}

	generic := SafeOpenAIClientUpstreamError(http.StatusBadGateway, "upstream connection reset")
	assert.Equal(t, SafeClientUpstreamError(http.StatusBadGateway), generic)
}

func TestOpenAIClientUpstreamErrorEnvelopeIncludesCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	payload := OpenAIClientUpstreamErrorEnvelope(c, SafeClientUpstreamError(http.StatusRequestEntityTooLarge))
	errorObj, ok := payload["error"].(gin.H)
	require.True(t, ok)
	require.Equal(t, ClientCodeRequestBodyTooLarge, errorObj["code"])
}

func TestClientErrorEnvelopesIncludeRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-visible-1"))

	openAIError, ok := OpenAIClientErrorEnvelope(c, "api_error", ClientMessageServiceUnavailable)["error"].(gin.H)
	require.True(t, ok)
	assert.Equal(t, "req-visible-1", openAIError["request_id"])
	assert.Equal(t, ClientCodeServiceUnavailable, openAIError["code"])
	assert.Contains(t, openAIError["message"], "Request ID: req-visible-1")

	genericEnvelope := ClientErrorEnvelope(c, "api_error", ClientMessageServiceUnavailable)
	assert.Equal(t, "req-visible-1", genericEnvelope["request_id"])
	genericError, ok := genericEnvelope["error"].(gin.H)
	require.True(t, ok)
	assert.Equal(t, "req-visible-1", genericError["request_id"])
	assert.Equal(t, ClientCodeServiceUnavailable, genericError["code"])
	assert.Contains(t, genericError["message"], "Request ID: req-visible-1")

	googleError, ok := GoogleClientErrorEnvelope(c, http.StatusBadGateway, ClientMessageServiceUnavailable)["error"].(gin.H)
	require.True(t, ok)
	assert.Equal(t, "req-visible-1", googleError["request_id"])
	assert.Contains(t, googleError["message"], "Request ID: req-visible-1")
}

func TestClientErrorObjectUsesStableDefaultCodes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)

	tests := map[string]string{
		"invalid_request_error": ClientCodeInvalidRequest,
		"authentication_error":  ClientCodeAuthenticationFailed,
		"permission_error":      ClientCodePermissionDenied,
		"billing_error":         ClientCodeInsufficientBalance,
		"subscription_error":    ClientCodeSubscriptionLimit,
		"rate_limit_error":      ClientCodeRateLimitExceeded,
		"overloaded_error":      ClientCodeServiceOverloaded,
		"timeout_error":         ClientCodeRequestTimeout,
		"not_found_error":       ClientCodeResourceNotFound,
		"api_error":             ClientCodeServiceUnavailable,
		"upstream_error":        ClientCodeUpstreamFailure,
	}

	for errType, wantCode := range tests {
		t.Run(errType, func(t *testing.T) {
			assert.Equal(t, wantCode, ClientErrorObject(c, errType, "message")["code"])
		})
	}
}

func TestClientErrorMessageDoesNotDuplicateVisibleRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-visible-2"))

	errorObj, ok := OpenAIClientErrorEnvelope(
		c,
		"api_error",
		"Already tagged. [Request ID: req-visible-2]",
	)["error"].(gin.H)
	require.True(t, ok)
	message, ok := errorObj["message"].(string)
	require.True(t, ok)
	assert.Equal(t, 1, strings.Count(message, "Request ID: req-visible-2"))
}

func TestOpenAIResponsesFailedEnvelopeIncludesRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ctxkey.RequestID, "req-visible-2"))

	payload := OpenAIResponsesFailedEnvelope(c, "", "gpt-test", "server_error", ClientMessageServiceUnavailable)
	response, ok := payload["response"].(gin.H)
	require.True(t, ok)
	errObj, ok := response["error"].(gin.H)
	require.True(t, ok)

	assert.Equal(t, "response.failed", payload["type"])
	assert.Equal(t, "resp_reqvisible2", response["id"])
	assert.Equal(t, "gpt-test", response["model"])
	assert.Equal(t, "req-visible-2", errObj["request_id"])
	assert.Contains(t, errObj["message"], ClientMessageServiceUnavailable)
	assert.Contains(t, errObj["message"], "Request ID: req-visible-2")
}

func TestOpenAIFastPolicyBlockedWSEventIncludesRequestID(t *testing.T) {
	payload := buildOpenAIFastPolicyBlockedWSEvent(&OpenAIFastBlockedError{Message: "openai fast policy blocked this request"}, "req-ws-policy")

	var parsed map[string]any
	assert.NoError(t, json.Unmarshal(payload, &parsed))
	errObj, ok := parsed["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "error", parsed["type"])
	assert.Equal(t, "policy_violation", errObj["code"])
	assert.Equal(t, "req-ws-policy", errObj["request_id"])
	assert.Contains(t, errObj["message"], "Request ID: req-ws-policy")
}

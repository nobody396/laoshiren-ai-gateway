package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	}{
		{
			name:           "invalid request",
			upstreamStatus: http.StatusUnprocessableEntity,
			wantStatus:     http.StatusBadRequest,
			wantType:       "invalid_request_error",
			wantMessage:    ClientMessageRequestFailed,
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
		})
	}
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

	genericError, ok := ClientErrorEnvelope(c, "api_error", ClientMessageServiceUnavailable)["error"].(gin.H)
	require.True(t, ok)
	assert.Equal(t, "req-visible-1", genericError["request_id"])

	googleError, ok := GoogleClientErrorEnvelope(c, http.StatusBadGateway, ClientMessageServiceUnavailable)["error"].(gin.H)
	require.True(t, ok)
	assert.Equal(t, "req-visible-1", googleError["request_id"])
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
	assert.Equal(t, ClientMessageServiceUnavailable, errObj["message"])
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
}

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCustomerErrorContract_RequestLoggerIDIsVisibleInOpenAIMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware2.RequestLogger())
	router.POST(EndpointResponses, func(c *gin.Context) {
		(&OpenAIGatewayHandler{}).errorResponse(
			c,
			http.StatusBadGateway,
			"api_error",
			service.ClientMessageServiceUnavailable,
		)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, EndpointResponses, nil))

	requestID := recorder.Header().Get("X-Request-ID")
	require.NotEmpty(t, requestID)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	errorObj, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, requestID, errorObj["request_id"])
	require.Contains(t, errorObj["message"], "Request ID: "+requestID)
}

func TestCustomerErrorContract_AnthropicEnvelopeAlsoUsesTopLevelRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware2.RequestLogger())
	router.POST(EndpointMessages, func(c *gin.Context) {
		(&GatewayHandler{}).errorResponse(
			c,
			http.StatusServiceUnavailable,
			"overloaded_error",
			service.ClientMessageServiceBusy,
		)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, EndpointMessages, nil))

	requestID := recorder.Header().Get("X-Request-ID")
	require.NotEmpty(t, requestID)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Equal(t, requestID, payload["request_id"])
	errorObj, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, requestID, errorObj["request_id"])
	require.Contains(t, errorObj["message"], "Request ID: "+requestID)
}

package service

import (
	"net/http"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

const (
	ClientMessageServiceUnavailable = "The service is temporarily unavailable. Please try again later."
	ClientMessageServiceBusy        = "The service is currently busy. Please try again later."
	ClientMessageRequestFailed      = "The request could not be processed. Please verify the request and try again."
	ClientMessageResourceNotFound   = "The requested resource could not be found."
	ClientMessageRequestTimeout     = "The request timed out. Please try again later."
)

type ClientUpstreamError struct {
	StatusCode int
	Type       string
	Message    string
}

func SafeClientUpstreamError(upstreamStatus int) ClientUpstreamError {
	switch upstreamStatus {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return ClientUpstreamError{
			StatusCode: http.StatusBadRequest,
			Type:       "invalid_request_error",
			Message:    ClientMessageRequestFailed,
		}
	case http.StatusNotFound:
		return ClientUpstreamError{
			StatusCode: http.StatusNotFound,
			Type:       "not_found_error",
			Message:    ClientMessageResourceNotFound,
		}
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return ClientUpstreamError{
			StatusCode: http.StatusGatewayTimeout,
			Type:       "timeout_error",
			Message:    ClientMessageRequestTimeout,
		}
	case http.StatusTooManyRequests:
		return ClientUpstreamError{
			StatusCode: http.StatusTooManyRequests,
			Type:       "rate_limit_error",
			Message:    ClientMessageServiceBusy,
		}
	case 529, http.StatusServiceUnavailable:
		return ClientUpstreamError{
			StatusCode: http.StatusServiceUnavailable,
			Type:       "overloaded_error",
			Message:    ClientMessageServiceBusy,
		}
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden:
		return ClientUpstreamError{
			StatusCode: http.StatusServiceUnavailable,
			Type:       "api_error",
			Message:    ClientMessageServiceUnavailable,
		}
	case http.StatusInternalServerError, http.StatusBadGateway:
		return ClientUpstreamError{
			StatusCode: http.StatusBadGateway,
			Type:       "api_error",
			Message:    ClientMessageServiceUnavailable,
		}
	default:
		return ClientUpstreamError{
			StatusCode: http.StatusBadGateway,
			Type:       "api_error",
			Message:    ClientMessageServiceUnavailable,
		}
	}
}

func ClientRequestID(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	if rid, ok := c.Request.Context().Value(ctxkey.RequestID).(string); ok {
		return strings.TrimSpace(rid)
	}
	return ""
}

func ClientErrorObject(c *gin.Context, errType, message string) gin.H {
	obj := gin.H{
		"type":    errType,
		"message": message,
	}
	if requestID := ClientRequestID(c); requestID != "" {
		obj["request_id"] = requestID
	}
	return obj
}

func ClientResponsesErrorObject(c *gin.Context, code, message string) gin.H {
	obj := gin.H{
		"code":    code,
		"message": message,
	}
	if requestID := ClientRequestID(c); requestID != "" {
		obj["request_id"] = requestID
	}
	return obj
}

func OpenAIResponsesFailedEnvelope(c *gin.Context, responseID, model, code, message string) gin.H {
	responseID = strings.TrimSpace(responseID)
	if responseID == "" {
		responseID = "resp_" + strings.ReplaceAll(ClientRequestID(c), "-", "")
	}
	if responseID == "resp_" {
		responseID = "resp_gateway_error"
	}
	response := gin.H{
		"id":     responseID,
		"object": "response",
		"status": "failed",
		"output": []any{},
		"error":  ClientResponsesErrorObject(c, code, message),
	}
	if model = strings.TrimSpace(model); model != "" {
		response["model"] = model
	}
	return gin.H{
		"type":     "response.failed",
		"response": response,
	}
}

func ClientErrorEnvelope(c *gin.Context, errType, message string) gin.H {
	return gin.H{
		"type":  "error",
		"error": ClientErrorObject(c, errType, message),
	}
}

func OpenAIClientErrorEnvelope(c *gin.Context, errType, message string) gin.H {
	return gin.H{
		"error": ClientErrorObject(c, errType, message),
	}
}

func GoogleClientErrorEnvelope(c *gin.Context, status int, message string) gin.H {
	errObj := gin.H{
		"code":    status,
		"message": message,
		"status":  googleStatusForClient(status),
	}
	if requestID := ClientRequestID(c); requestID != "" {
		errObj["request_id"] = requestID
	}
	return gin.H{"error": errObj}
}

func googleStatusForClient(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "INVALID_ARGUMENT"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusTooManyRequests:
		return "RESOURCE_EXHAUSTED"
	case http.StatusGatewayTimeout:
		return "DEADLINE_EXCEEDED"
	case http.StatusInternalServerError:
		return "INTERNAL"
	case http.StatusBadGateway, http.StatusServiceUnavailable:
		return "UNAVAILABLE"
	default:
		return http.StatusText(status)
	}
}

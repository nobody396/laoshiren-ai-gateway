package service

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

const (
	ClientMessageServiceUnavailable    = "No healthy service route is currently available. Retry later or switch to another model."
	ClientMessageServiceBusy           = "No service route currently has available capacity. Retry in about 60 seconds or switch to another model."
	ClientMessageRequestFailed         = "This request is invalid for the selected model or endpoint. Check the model, endpoint, and request parameters before retrying."
	ClientMessageRequestBodyTooLarge   = "Request body is too large. Start a new task or remove large attachments before retrying."
	ClientMessageContextWindowExceeded = "Context window exceeded: this conversation is too long for the model. Retrying the same request will not help; start a new task or remove earlier messages or large attachments."
	ClientMessageResourceNotFound      = "The requested model or resource is not available for this API key and group. Check /v1/models and your selected group."
	ClientMessageRequestTimeout        = "The request did not complete before the timeout. Retry later or switch to another model."
	ClientCodeInvalidRequest           = "invalid_request"
	ClientCodeRequestBodyTooLarge      = "request_body_too_large"
	ClientCodeContextWindowExceeded    = "context_length_exceeded"
	ClientCodeModelNotSupported        = "model_not_supported"
	ClientCodeEndpointNotSupported     = "endpoint_not_supported"
	ClientCodeAuthenticationFailed     = "authentication_failed"
	ClientCodePermissionDenied         = "permission_denied"
	ClientCodeInsufficientBalance      = "insufficient_balance"
	ClientCodeSubscriptionLimit        = "subscription_limit"
	ClientCodeRateLimitExceeded        = "rate_limit_exceeded"
	ClientCodeServiceOverloaded        = "service_overloaded"
	ClientCodeUpstreamFailure          = "upstream_failure"
	ClientCodeRequestTimeout           = "request_timeout"
	ClientCodeResourceNotFound         = "resource_not_found"
	ClientCodeServiceUnavailable       = "service_unavailable"
)

// ModelPricingPageURL 用户可查看全部已上架支持模型的公开页面。
const ModelPricingPageURL = "https://laoshirenai.com/models"

// ClientMessageModelNotSupported 生成模型不支持的用户侧错误文案，
// 动态带上请求的模型名与支持模型查看入口。
func ClientMessageModelNotSupported(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "the requested model"
	}
	return fmt.Sprintf(
		"The model %q is not supported. Please switch to a supported model. You can view all supported models at %s",
		model,
		ModelPricingPageURL,
	)
}

// ClientMessageEndpointNotSupported 生成分组协议与请求端点结构性不匹配时
// 的用户侧错误文案。端点不匹配与模型未上架是两个不同错误，不能复用
// model_not_supported，否则会把已开放的模型误报为未开放。
func ClientMessageEndpointNotSupported(model, requestedEndpoint, groupProtocol, expectedEndpoint string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		model = "the requested model"
	}
	requestedEndpoint = strings.TrimSpace(requestedEndpoint)
	if requestedEndpoint == "" {
		requestedEndpoint = "the requested endpoint"
	}
	groupProtocol = strings.TrimSpace(groupProtocol)
	expectedEndpoint = strings.TrimSpace(expectedEndpoint)
	if groupProtocol == "" || expectedEndpoint == "" {
		return fmt.Sprintf(
			"The endpoint %q is not supported for this group. Use the endpoint documented for this group with model %q. You can view all supported models at %s",
			requestedEndpoint,
			model,
			ModelPricingPageURL,
		)
	}
	return fmt.Sprintf(
		"The endpoint %q is not supported for this %s group. Use the %s endpoint %q with model %q. You can view all supported models at %s",
		requestedEndpoint,
		groupProtocol,
		groupProtocol,
		expectedEndpoint,
		model,
		ModelPricingPageURL,
	)
}

type ClientUpstreamError struct {
	StatusCode int
	Type       string
	Message    string
	Code       string
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
	case http.StatusRequestEntityTooLarge:
		return ClientUpstreamError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Type:       "invalid_request_error",
			Message:    ClientMessageRequestBodyTooLarge,
			Code:       ClientCodeRequestBodyTooLarge,
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

// SafeOpenAIClientUpstreamError preserves one deterministic, actionable
// OpenAI failure class: an input that cannot fit in the model's context
// window. Retrying the same request or rotating accounts cannot fix it, so the
// client must receive a terminal invalid-request error instead of a transient
// service-unavailable wrapper.
func SafeOpenAIClientUpstreamError(upstreamStatus int, errorFields ...string) ClientUpstreamError {
	if IsOpenAIContextWindowExceeded(errorFields...) {
		return ClientUpstreamError{
			StatusCode: http.StatusBadRequest,
			Type:       "invalid_request_error",
			Message:    ClientMessageContextWindowExceeded,
			Code:       ClientCodeContextWindowExceeded,
		}
	}
	return SafeClientUpstreamError(upstreamStatus)
}

// IsOpenAIContextWindowExceeded recognizes the stable codes and wording used
// by OpenAI-compatible upstreams without exposing their raw response body.
func IsOpenAIContextWindowExceeded(errorFields ...string) bool {
	combined := strings.ToLower(strings.Join(errorFields, " "))
	if strings.TrimSpace(combined) == "" {
		return false
	}
	markers := []string{
		"context_length_exceeded",
		"context_window_exceeded",
		"maximum context length",
		"max context length",
		"exceeds the context window",
		"exceeded the context window",
		"context window limit",
		"too many input tokens",
		"reduce the length of the messages",
		"input is too long for the model",
		"上下文长度超出",
		"上下文过长",
		"超过模型上下文",
	}
	for _, marker := range markers {
		if strings.Contains(combined, marker) {
			return true
		}
	}
	return false
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

// ClientMessageWithRequestID keeps the platform Customer Request ID inside the
// human-readable message because many SDKs and CLIs discard unknown structured
// error fields and show only error.message. The structured request_id remains
// present as well for clients that preserve it.
func ClientMessageWithRequestID(c *gin.Context, message string) string {
	return clientMessageWithRequestID(message, ClientRequestID(c))
}

func clientMessageWithRequestID(message, requestID string) string {
	message = strings.TrimSpace(message)
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return message
	}
	marker := "Request ID: " + requestID
	if strings.Contains(strings.ToLower(message), strings.ToLower(marker)) {
		return message
	}
	if message == "" {
		return marker
	}
	return message + " [" + marker + "]"
}

func clientDefaultErrorCode(errType string) string {
	switch strings.TrimSpace(errType) {
	case "invalid_request_error":
		return ClientCodeInvalidRequest
	case "authentication_error":
		return ClientCodeAuthenticationFailed
	case "permission_error", "forbidden_error":
		return ClientCodePermissionDenied
	case "billing_error":
		return ClientCodeInsufficientBalance
	case "subscription_error":
		return ClientCodeSubscriptionLimit
	case "rate_limit_error":
		return ClientCodeRateLimitExceeded
	case "overloaded_error":
		return ClientCodeServiceOverloaded
	case "timeout_error":
		return ClientCodeRequestTimeout
	case "not_found_error":
		return ClientCodeResourceNotFound
	case "upstream_error":
		return ClientCodeUpstreamFailure
	case "api_error", "server_error":
		return ClientCodeServiceUnavailable
	default:
		return ""
	}
}

func ClientErrorObject(c *gin.Context, errType, message string) gin.H {
	obj := gin.H{
		"type":    errType,
		"message": ClientMessageWithRequestID(c, message),
	}
	if code := clientDefaultErrorCode(errType); code != "" {
		obj["code"] = code
	}
	if requestID := ClientRequestID(c); requestID != "" {
		obj["request_id"] = requestID
	}
	return obj
}

func ClientResponsesErrorObject(c *gin.Context, code, message string) gin.H {
	obj := gin.H{
		"code":    code,
		"message": ClientMessageWithRequestID(c, message),
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
	envelope := gin.H{
		"type":  "error",
		"error": ClientErrorObject(c, errType, message),
	}
	if requestID := ClientRequestID(c); requestID != "" {
		envelope["request_id"] = requestID
	}
	return envelope
}

func OpenAIClientErrorEnvelope(c *gin.Context, errType, message string) gin.H {
	return gin.H{
		"error": ClientErrorObject(c, errType, message),
	}
}

func OpenAIClientErrorEnvelopeWithCode(c *gin.Context, errType, code, message string) gin.H {
	obj := ClientErrorObject(c, errType, message)
	if code = strings.TrimSpace(code); code != "" {
		obj["code"] = code
	}
	return gin.H{"error": obj}
}

func OpenAIClientUpstreamErrorEnvelope(c *gin.Context, err ClientUpstreamError) gin.H {
	return OpenAIClientErrorEnvelopeWithCode(c, err.Type, err.Code, err.Message)
}

func GoogleClientErrorEnvelope(c *gin.Context, status int, message string) gin.H {
	errObj := gin.H{
		"code":    status,
		"message": ClientMessageWithRequestID(c, message),
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

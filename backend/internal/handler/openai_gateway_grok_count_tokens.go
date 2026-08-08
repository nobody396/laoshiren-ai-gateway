package handler

import (
	"net/http"
	"strings"

	pkghttputil "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/httputil"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GrokCountTokens handles Anthropic-compatible count_tokens requests locally.
// Authentication and group resolution are already enforced by route middleware.
func (h *OpenAIGatewayHandler) GrokCountTokens(c *gin.Context) {
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	parsed, err := service.ParseGatewayRequest(body, service.PlatformAnthropic)
	if err != nil {
		requestLogger(c, "handler.openai_gateway.grok_count_tokens").Warn("grok_count_tokens.parse_failed", zap.Error(err))
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}
	if strings.TrimSpace(parsed.Model) == "" {
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	estimated, err := service.EstimateGrokCountTokens(body)
	if err != nil {
		requestLogger(c, "handler.openai_gateway.grok_count_tokens").Warn("grok_count_tokens.local_estimate_failed", zap.Error(err))
		h.anthropicErrorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}
	setOpsRequestContext(c, parsed.Model, false, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
	c.JSON(http.StatusOK, gin.H{"input_tokens": estimated})
}

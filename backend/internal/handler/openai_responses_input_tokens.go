package handler

import (
	"net/http"

	pkghttputil "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/httputil"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ResponsesInputTokens serves OpenAI's native token preflight shape without
// account selection or billing. A response header makes the local-estimate
// provenance explicit while preserving the client-compatible JSON contract.
func (h *OpenAIGatewayHandler) ResponsesInputTokens(c *gin.Context) {
	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}

	estimated, model, err := service.EstimateOpenAIResponsesInputTokens(body)
	if err != nil {
		requestLogger(c, "handler.openai_gateway.responses_input_tokens").Warn(
			"responses_input_tokens.local_estimate_failed",
			zap.Error(err),
		)
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}

	setOpsRequestContext(c, model, false, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))
	c.Header("X-Token-Count-Source", "local-estimate")
	c.JSON(http.StatusOK, gin.H{
		"object":       "response.input_tokens",
		"input_tokens": estimated,
	})
}

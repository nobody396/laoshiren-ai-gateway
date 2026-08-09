package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ListOpenAIRouteShadowDecisions returns the complete non-sensitive decision
// snapshot for every persisted shadow evaluation.
// GET /api/v1/admin/ops/openai-route-shadow/decisions
func (h *OpsHandler) ListOpenAIRouteShadowDecisions(c *gin.Context) {
	filter, err := parseOpenAIRouteShadowDecisionFilter(c, true)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.opsService.ListOpenAIRouteShadowDecisions(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, result.Decisions, int64(result.Total), result.Page, result.PageSize)
}

// GetOpenAIRouteShadowDecisionStats aggregates audit completeness, selection
// shares, divergence, evaluation latency, and linked real request TTFT.
// GET /api/v1/admin/ops/openai-route-shadow/stats
func (h *OpsHandler) GetOpenAIRouteShadowDecisionStats(c *gin.Context) {
	filter, err := parseOpenAIRouteShadowDecisionFilter(c, false)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	result, err := h.opsService.GetOpenAIRouteShadowDecisionStats(c.Request.Context(), filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// GetOpenAIRouteAuditHealth exposes write failures explicitly. Shadow samples
// are invalidated on a failed write instead of silently disappearing.
// GET /api/v1/admin/ops/openai-route-shadow/health
func (h *OpsHandler) GetOpenAIRouteAuditHealth(c *gin.Context) {
	if h.opsService == nil {
		response.Error(c, http.StatusServiceUnavailable, "Ops service not available")
		return
	}
	if err := h.opsService.RequireMonitoringEnabled(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, h.opsService.GetOpenAIRouteAuditHealth(c.Request.Context()))
}

func parseOpenAIRouteShadowDecisionFilter(c *gin.Context, withPagination bool) (*service.OpenAIRouteShadowDecisionFilter, error) {
	start, end, err := parseOpsTimeRange(c, "24h")
	if err != nil {
		return nil, err
	}
	filter := &service.OpenAIRouteShadowDecisionFilter{
		StartTime:       &start,
		EndTime:         &end,
		Model:           strings.TrimSpace(c.Query("model")),
		Reason:          strings.TrimSpace(c.Query("reason")),
		RequestID:       strings.TrimSpace(c.Query("request_id")),
		ClientRequestID: strings.TrimSpace(c.Query("client_request_id")),
	}
	if withPagination {
		filter.Page, filter.PageSize = response.ParsePagination(c)
		if filter.PageSize > 200 {
			filter.PageSize = 200
		}
	}
	if raw := strings.TrimSpace(c.Query("group_id")); raw != "" {
		value, parseErr := strconv.ParseInt(raw, 10, 64)
		if parseErr != nil || value <= 0 {
			return nil, strconv.ErrSyntax
		}
		filter.GroupID = &value
	}
	if raw := strings.TrimSpace(c.Query("policy_version")); raw != "" {
		value, parseErr := strconv.Atoi(raw)
		if parseErr != nil || value < 0 {
			return nil, strconv.ErrSyntax
		}
		filter.PolicyVersion = &value
	}
	if filter.Evaluated, err = parseOpenAIRouteOptionalBool(c.Query("evaluated")); err != nil {
		return nil, err
	}
	if filter.Diverged, err = parseOpenAIRouteOptionalBool(c.Query("diverged")); err != nil {
		return nil, err
	}
	if filter.Emergency, err = parseOpenAIRouteOptionalBool(c.Query("emergency")); err != nil {
		return nil, err
	}
	return filter, nil
}

func parseOpenAIRouteOptionalBool(raw string) (*bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

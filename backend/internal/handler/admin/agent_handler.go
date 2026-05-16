package admin

import (
	"strconv"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type AgentHandler struct {
	commissionService *service.CommissionService
}

func NewAgentHandler(commissionService *service.CommissionService) *AgentHandler {
	return &AgentHandler{commissionService: commissionService}
}

type updateCommissionRatesRequest struct {
	ConsumptionRate           float64 `json:"consumption_rate"`
	FirstRechargeInviteeRate  float64 `json:"first_recharge_invitee_rate"`
	FirstRechargeReferralRate float64 `json:"first_recharge_referral_rate"`
}

type updateAgentRateRequest struct {
	ConsumptionRate float64 `json:"consumption_rate"`
	Enabled         bool    `json:"enabled"`
}

type updateAgentLevelRulesRequest struct {
	Rules []service.AgentLevelRule `json:"rules"`
}

type createSettlementRequest struct {
	Amount float64 `json:"amount"`
	Note   string  `json:"note"`
}

type bindAgentUserRequest struct {
	Email          string `json:"email" binding:"required,email"`
	OverwriteAgent bool   `json:"overwrite_agent"`
}

func (h *AgentHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	start, end := parseAgentDateRange(c)
	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}
	items, result, err := h.commissionService.ListAdminAgents(c.Request.Context(), params, service.AdminAgentListFilters{
		Search: c.Query("search"),
		Start:  start,
		End:    end,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *AgentHandler) Get(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	start, end := parseAgentDateRange(c)
	item, err := h.commissionService.GetAdminAgent(c.Request.Context(), agentID, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *AgentHandler) ListUsers(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	start, end := parseAgentDateRange(c)
	items, result, err := h.commissionService.ListAdminAgentUsers(c.Request.Context(), agentID, pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.Query("sort_by"),
		SortOrder: c.Query("sort_order"),
	}, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *AgentHandler) BindUser(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	var req bindAgentUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	result, err := h.commissionService.BindUserToAgent(c.Request.Context(), agentID, service.BindAgentUserInput{
		Email:          req.Email,
		OverwriteAgent: req.OverwriteAgent,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AgentHandler) ListCommissions(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	start, end := parseAgentDateRange(c)
	items, result, err := h.commissionService.ListAdminAgentCommissions(c.Request.Context(), agentID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, c.Query("type"), start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *AgentHandler) ListSettlements(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.commissionService.ListAgentSettlements(c.Request.Context(), agentID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, result.Page, result.PageSize)
}

func (h *AgentHandler) CreateSettlement(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}
	var req createSettlementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	settlement, err := h.commissionService.CreateAgentSettlement(c.Request.Context(), agentID, subject.UserID, req.Amount, req.Note)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settlement)
}

func (h *AgentHandler) GetRates(c *gin.Context) {
	rates, err := h.commissionService.GetCommissionRates(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rates)
}

func (h *AgentHandler) UpdateRates(c *gin.Context) {
	var req updateCommissionRatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rates, err := h.commissionService.UpdateCommissionRates(c.Request.Context(), &service.CommissionRates{
		ConsumptionRate:           req.ConsumptionRate,
		FirstRechargeInviteeRate:  req.FirstRechargeInviteeRate,
		FirstRechargeReferralRate: req.FirstRechargeReferralRate,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rates)
}

func (h *AgentHandler) GetLevelRules(c *gin.Context) {
	rules, err := h.commissionService.GetAgentLevelRules(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rules)
}

func (h *AgentHandler) UpdateLevelRules(c *gin.Context) {
	var req updateAgentLevelRulesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	rules, err := h.commissionService.UpdateAgentLevelRules(c.Request.Context(), req.Rules)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, rules)
}

func (h *AgentHandler) RunLevelEvaluations(c *gin.Context) {
	result, err := h.commissionService.RunAllAgentLevelEvaluations(c.Request.Context(), time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AgentHandler) RunAgentLevelEvaluation(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	result, err := h.commissionService.RunAgentLevelEvaluation(c.Request.Context(), agentID, time.Now())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *AgentHandler) GetAgentRate(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	config, err := h.commissionService.GetAgentRateConfig(c.Request.Context(), agentID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func (h *AgentHandler) UpdateAgentRate(c *gin.Context) {
	agentID, ok := parseAgentIDParam(c)
	if !ok {
		return
	}
	var req updateAgentRateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	config, err := h.commissionService.UpdateAgentRateConfig(c.Request.Context(), &service.AgentRateConfig{
		AgentID:         agentID,
		ConsumptionRate: req.ConsumptionRate,
		Enabled:         req.Enabled,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, config)
}

func parseAgentIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid agent ID")
		return 0, false
	}
	return id, true
}

func parseAgentDateRange(c *gin.Context) (*time.Time, *time.Time) {
	var start, end *time.Time
	if raw := c.Query("start"); raw != "" {
		if parsed, err := time.Parse("2006-01-02", raw); err == nil {
			start = &parsed
		}
	}
	if raw := c.Query("end"); raw != "" {
		if parsed, err := time.Parse("2006-01-02", raw); err == nil {
			parsed = parsed.Add(24*time.Hour - time.Second)
			end = &parsed
		}
	}
	return start, end
}

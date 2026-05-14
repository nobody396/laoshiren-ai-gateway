package handler

import (
	"strconv"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AgentHandler 代理商相关请求处理
type AgentHandler struct {
	commissionService *service.CommissionService
}

// NewAgentHandler 创建 AgentHandler
func NewAgentHandler(commissionService *service.CommissionService) *AgentHandler {
	return &AgentHandler{commissionService: commissionService}
}

// GetInviteCode 获取或生成当前用户的邀请码
// GET /api/v1/agent/invite-code
func (h *AgentHandler) GetInviteCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	code, err := h.commissionService.GetOrCreateInviteCode(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"invite_code": code})
}

// GetDashboard 获取代理商总览统计
// GET /api/v1/agent/dashboard?start=2026-01-01&end=2026-01-31
func (h *AgentHandler) GetDashboard(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	start, end := parseDateRange(c)
	dashboard, err := h.commissionService.GetAgentDashboard(c.Request.Context(), subject.UserID, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dashboard)
}

// GetInvitedUsers 获取代理商邀请的用户列表及消费统计
// GET /api/v1/agent/users?page=1&page_size=20&start=2026-01-01&end=2026-01-31
func (h *AgentHandler) GetInvitedUsers(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	params := pagination.PaginationParams{
		Page:     parseIntQuery(c, "page", 1),
		PageSize: parseIntQuery(c, "page_size", 20),
	}
	start, end := parseDateRange(c)

	users, paginationResult, err := h.commissionService.GetAgentInvitedUsers(c.Request.Context(), subject.UserID, params, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"items":      users,
		"pagination": paginationResult,
	})
}

// GetCommissions 获取代理商的分佣记录明细
// GET /api/v1/agent/commissions?type=consumption&start=2026-01-01&end=2026-01-31
func (h *AgentHandler) GetCommissions(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	params := pagination.PaginationParams{
		Page:     parseIntQuery(c, "page", 1),
		PageSize: parseIntQuery(c, "page_size", 20),
	}
	typeFilter := c.Query("type")
	start, end := parseDateRange(c)

	records, paginationResult, err := h.commissionService.GetAgentCommissions(c.Request.Context(), subject.UserID, params, typeFilter, start, end)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"items":      records,
		"pagination": paginationResult,
	})
}

// GetMyInviteCode 普通用户获取自己的邀请码
// GET /api/v1/user/invite-code
func (h *AgentHandler) GetMyInviteCode(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	code, err := h.commissionService.GetOrCreateInviteCode(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"invite_code": code})
}

// parseDateRange 从 query 参数解析 start/end 时间（格式：2006-01-02）
func parseDateRange(c *gin.Context) (*time.Time, *time.Time) {
	var start, end *time.Time
	if s := c.Query("start"); s != "" {
		if t, err := time.Parse("2006-01-02", s); err == nil {
			start = &t
		}
	}
	if e := c.Query("end"); e != "" {
		if t, err := time.Parse("2006-01-02", e); err == nil {
			// end of day
			t = t.Add(24*time.Hour - time.Second)
			end = &t
		}
	}
	return start, end
}

// parseIntQuery 从 query 参数解析整数，fallback 到 defaultVal
func parseIntQuery(c *gin.Context, key string, defaultVal int) int {
	val := c.Query(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return defaultVal
	}
	return n
}

// ValidateReferralCode 验证邀请码是否有效（公开接口）
// GET /api/v1/validate-referral-code?code=XXXX
func (h *AgentHandler) ValidateReferralCode(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		response.Success(c, gin.H{"valid": false})
		return
	}

	inviter, err := h.commissionService.ValidateAndGetInviter(c.Request.Context(), code)
	if err != nil {
		response.Success(c, gin.H{"valid": false})
		return
	}

	response.Success(c, gin.H{"valid": inviter != nil})
}

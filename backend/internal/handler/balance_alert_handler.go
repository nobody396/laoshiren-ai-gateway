package handler

import (
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// BalanceAlertHandler 余额预警用户端 API
type BalanceAlertHandler struct {
	alertService *service.BalanceAlertService
}

// NewBalanceAlertHandler creates a new BalanceAlertHandler.
func NewBalanceAlertHandler(alertService *service.BalanceAlertService) *BalanceAlertHandler {
	return &BalanceAlertHandler{alertService: alertService}
}

// GetConfig handles GET /api/v1/user/balance-alert
func (h *BalanceAlertHandler) GetConfig(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	cfg, err := h.alertService.GetUserAlertConfig(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, cfg)
}

type updateBalanceAlertRequest struct {
	Enabled        *bool    `json:"enabled"`
	Threshold      *float64 `json:"threshold"`
	ClearThreshold bool     `json:"clear_threshold"`
	Email          *string  `json:"email"`
}

// UpdateConfig handles PUT /api/v1/user/balance-alert
func (h *BalanceAlertHandler) UpdateConfig(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req updateBalanceAlertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}

	input := service.UpdateUserAlertConfigInput{
		Enabled:        req.Enabled,
		Threshold:      req.Threshold,
		ClearThreshold: req.ClearThreshold,
		Email:          req.Email,
	}

	if err := h.alertService.UpdateUserAlertConfig(c.Request.Context(), subject.UserID, input); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "ok"})
}

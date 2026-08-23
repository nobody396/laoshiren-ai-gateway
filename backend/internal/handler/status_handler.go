package handler

import (
	"net/http"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type StatusHandler struct{ service *service.StatusControlService }

func NewStatusHandler(statusService *service.StatusControlService) *StatusHandler {
	return &StatusHandler{service: statusService}
}

func (h *StatusHandler) GetAdminStatus(c *gin.Context) {
	snapshot, err := h.service.AdminSnapshot(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, snapshot)
}

func (h *StatusHandler) GetSettings(c *gin.Context) {
	response.Success(c, h.service.Settings(c.Request.Context()))
}

func (h *StatusHandler) UpdateSettings(c *gin.Context) {
	var request service.StatusSettings
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.service.UpdateSettings(c.Request.Context(), request); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, h.service.Settings(c.Request.Context()))
}

type statusOverrideRequest struct {
	Status          service.ServiceStatus `json:"status" binding:"required"`
	Reason          string                `json:"reason" binding:"required"`
	DurationMinutes int                   `json:"duration_minutes" binding:"required"`
}

func (h *StatusHandler) CreateOverride(c *gin.Context) {
	var request statusOverrideRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	userID := int64(0)
	if value, exists := c.Get(string(middleware2.ContextKeyUser)); exists {
		if subject, ok := value.(middleware2.AuthSubject); ok {
			userID = subject.UserID
		}
	}
	err := h.service.CreateOverride(c.Request.Context(), service.StatusOverrideCommand{
		ProductCode: c.Param("code"), Status: request.Status, Reason: request.Reason,
		Duration: time.Duration(request.DurationMinutes) * time.Minute, CreatedByUserID: userID,
	})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"created": true})
}

func (h *StatusHandler) GetPublicStatus(c *gin.Context) {
	if h == nil || h.service == nil {
		response.Error(c, http.StatusServiceUnavailable, "Service Status is unavailable")
		return
	}
	snapshot, err := h.service.PublicSnapshot(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Service Status is temporarily unavailable")
		return
	}
	response.Success(c, snapshot)
}

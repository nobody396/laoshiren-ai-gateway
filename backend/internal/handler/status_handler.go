package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type StatusHandler struct {
	service  *service.StatusControlService
	incident *service.IncidentControlService
}

func NewStatusHandler(statusService *service.StatusControlService) *StatusHandler {
	return &StatusHandler{service: statusService}
}

func ProvideStatusHandler(statusService *service.StatusControlService, incidentService *service.IncidentControlService) *StatusHandler {
	return &StatusHandler{service: statusService, incident: incidentService}
}

func (h *StatusHandler) GetAdminStatus(c *gin.Context) {
	snapshot, err := h.service.AdminSnapshot(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, snapshot)
}

func (h *StatusHandler) GetChannelMonitoring(c *gin.Context) {
	window, err := parseChannelMonitoringWindow(c.Query("window_minutes"))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	snapshot, err := h.service.MonitoringSnapshot(c.Request.Context(), window)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Channel Monitoring is temporarily unavailable")
		return
	}
	response.Success(c, snapshot)
}

func parseChannelMonitoringWindow(raw string) (time.Duration, error) {
	if raw == "" {
		return 15 * time.Minute, nil
	}
	minutes, err := strconv.Atoi(raw)
	if err != nil || minutes < 5 || minutes > 120 {
		return 0, fmt.Errorf("window_minutes must be between 5 and 120")
	}
	return time.Duration(minutes) * time.Minute, nil
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

func (h *StatusHandler) GetAdminIncidents(c *gin.Context) {
	if h == nil || h.incident == nil {
		response.Error(c, http.StatusServiceUnavailable, "Incident Control is unavailable")
		return
	}
	snapshot, err := h.incident.AdminSnapshot(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Incident Control is temporarily unavailable")
		return
	}
	response.Success(c, snapshot)
}

func (h *StatusHandler) UpdateIncidentSettings(c *gin.Context) {
	var request service.IncidentControlSettings
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.incident.UpdateSettings(c.Request.Context(), request, statusActorUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, h.incident.Settings(c.Request.Context()))
}

type incidentConfirmRequest struct {
	Title           string `json:"title" binding:"required"`
	InternalSummary string `json:"internal_summary"`
}

func (h *StatusHandler) ConfirmIncidentCandidate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid candidate id")
		return
	}
	var request incidentConfirmRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	incident, err := h.incident.ConfirmCandidate(c.Request.Context(), service.IncidentConfirmCommand{CandidateID: id, Title: request.Title, InternalSummary: request.InternalSummary, ActorUserID: statusActorUserID(c)})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, incident)
}

type incidentMessageRequest struct {
	Message string `json:"message" binding:"required"`
}

func (h *StatusHandler) DismissIncidentCandidate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid candidate id")
		return
	}
	var request incidentMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.incident.DismissCandidate(c.Request.Context(), service.IncidentDismissCommand{CandidateID: id, Reason: request.Message, ActorUserID: statusActorUserID(c)}); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"dismissed": true})
}

type incidentTransitionRequest struct {
	Phase   service.IncidentPhase `json:"phase" binding:"required"`
	Message string                `json:"message" binding:"required"`
}

func (h *StatusHandler) TransitionIncident(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid incident id")
		return
	}
	var request incidentTransitionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.incident.Transition(c.Request.Context(), service.IncidentTransitionCommand{IncidentID: id, Target: request.Phase, Message: request.Message, ActorUserID: statusActorUserID(c)}); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"transitioned": true})
}

func (h *StatusHandler) AddIncidentUpdate(c *gin.Context) {
	h.writeIncidentMessage(c, false)
}

func (h *StatusHandler) PublishIncidentUpdate(c *gin.Context) {
	h.writeIncidentMessage(c, true)
}

func (h *StatusHandler) AcknowledgeIncidentEvidenceGap(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid incident id")
		return
	}
	var request incidentMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.incident.AcknowledgeEvidenceGap(c.Request.Context(), service.IncidentEvidenceGapCommand{IncidentID: id, Reason: request.Message, ActorUserID: statusActorUserID(c)}); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"acknowledged": true})
}

func (h *StatusHandler) writeIncidentMessage(c *gin.Context, public bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid incident id")
		return
	}
	var request incidentMessageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if public {
		err = h.incident.PublishUpdate(c.Request.Context(), service.IncidentPublishCommand{IncidentID: id, Message: request.Message, ActorUserID: statusActorUserID(c)})
	} else {
		err = h.incident.AddUpdate(c.Request.Context(), service.IncidentUpdateCommand{IncidentID: id, Message: request.Message, ActorUserID: statusActorUserID(c)})
	}
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"created": true})
}

func (h *StatusHandler) GetPublicIncidents(c *gin.Context) {
	if h == nil || h.incident == nil {
		response.Error(c, http.StatusServiceUnavailable, "Incident timeline is unavailable")
		return
	}
	snapshot, err := h.incident.PublicSnapshot(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusServiceUnavailable, "Incident timeline is temporarily unavailable")
		return
	}
	response.Success(c, snapshot)
}

func statusActorUserID(c *gin.Context) int64 {
	if c == nil {
		return 0
	}
	if value, exists := c.Get(string(middleware2.ContextKeyUser)); exists {
		if subject, ok := value.(middleware2.AuthSubject); ok {
			return subject.UserID
		}
	}
	return 0
}

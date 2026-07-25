package handler

import (
	"strconv"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/dto"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// AnnouncementHandler handles user announcement operations
type AnnouncementHandler struct {
	announcementService *service.AnnouncementService
}

type markPopupBatchPromptedRequest struct {
	ThroughAnnouncementID int64 `json:"through_announcement_id" binding:"required,gt=0"`
}

// NewAnnouncementHandler creates a new user announcement handler
func NewAnnouncementHandler(announcementService *service.AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{
		announcementService: announcementService,
	}
}

// List handles listing announcements visible to current user
// GET /api/v1/announcements
func (h *AnnouncementHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	unreadOnly := parseBoolQuery(c.Query("unread_only"))

	items, err := h.announcementService.ListForUser(c.Request.Context(), subject.UserID, unreadOnly)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.UserAnnouncement, 0, len(items))
	for i := range items {
		out = append(out, *dto.UserAnnouncementFromService(&items[i]))
	}
	response.Success(c, out)
}

// MarkRead marks an announcement as read for current user
// POST /api/v1/announcements/:id/read
func (h *AnnouncementHandler) MarkRead(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	announcementID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || announcementID <= 0 {
		response.BadRequest(c, "Invalid announcement ID")
		return
	}

	if err := h.announcementService.MarkRead(c.Request.Context(), subject.UserID, announcementID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "ok"})
}

// PopupState returns the delivery cursor used to avoid replaying an old popup backlog.
// GET /api/v1/announcements/popup-state
func (h *AnnouncementHandler) PopupState(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	lastPromptedID, err := h.announcementService.GetPopupState(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"last_prompted_announcement_id": lastPromptedID})
}

// MarkPopupBatchPrompted records that a group of announcements already interrupted the user.
// It intentionally does not mark any individual announcement as read.
// POST /api/v1/announcements/popup-prompted
func (h *AnnouncementHandler) MarkPopupBatchPrompted(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	var req markPopupBatchPromptedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request")
		return
	}

	if err := h.announcementService.MarkPopupBatchPrompted(
		c.Request.Context(),
		subject.UserID,
		req.ThroughAnnouncementID,
	); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func parseBoolQuery(v string) bool {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

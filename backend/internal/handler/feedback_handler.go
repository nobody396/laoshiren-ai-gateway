package handler

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/dto"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type FeedbackHandler struct {
	feedbackService *service.FeedbackService
}

func NewFeedbackHandler(feedbackService *service.FeedbackService) *FeedbackHandler {
	return &FeedbackHandler{feedbackService: feedbackService}
}

type createFeedbackRequest struct {
	Category  string   `json:"category" binding:"required"`
	Title     string   `json:"title" binding:"required"`
	Content   string   `json:"content" binding:"required"`
	Images    []string `json:"images"`
	Contact   string   `json:"contact"`
	RequestID string   `json:"request_id"`
}

type createFeedbackReplyRequest struct {
	Content string   `json:"content" binding:"required"`
	Images  []string `json:"images"`
}

type updateFeedbackRequest struct {
	Category  string   `json:"category" binding:"required"`
	Title     string   `json:"title" binding:"required"`
	Content   string   `json:"content" binding:"required"`
	Images    []string `json:"images"`
	Contact   string   `json:"contact"`
	RequestID string   `json:"request_id"`
}

type verifyFeedbackRequest struct {
	Resolved bool   `json:"resolved"`
	Note     string `json:"note"`
}

type uploadFeedbackImageResponse struct {
	URL string `json:"url"`
}

func (h *FeedbackHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	var req createFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	feedback, err := h.feedbackService.Create(c.Request.Context(), subject.UserID, service.CreateFeedbackInput{
		Category:  req.Category,
		Title:     req.Title,
		Content:   req.Content,
		Images:    req.Images,
		Contact:   req.Contact,
		RequestID: req.RequestID,
	})
	if err != nil {
		writeRetryAfterHeader(c, err)
		response.ErrorFrom(c, err)
		return
	}

	response.Created(c, dto.FeedbackForUserFromService(feedback))
}

func (h *FeedbackHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}

	items, result, err := h.feedbackService.ListByUser(c.Request.Context(), subject.UserID, params, service.FeedbackListFilters{
		Status: strings.TrimSpace(c.Query("status")),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.Feedback, 0, len(items))
	for i := range items {
		out = append(out, *dto.FeedbackForUserFromService(&items[i]))
	}
	response.PaginatedWithResult(c, out, &response.PaginationResult{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
		Pages:    result.Pages,
	})
}

func (h *FeedbackHandler) GetByID(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	feedbackID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || feedbackID <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}

	item, err := h.feedbackService.GetByUser(c.Request.Context(), subject.UserID, feedbackID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.FeedbackDetailForUserFromService(item))
}

func (h *FeedbackHandler) Update(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	feedbackID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || feedbackID <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}

	var req updateFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	updated, err := h.feedbackService.UpdateByUser(c.Request.Context(), subject.UserID, feedbackID, service.UpdateFeedbackByUserInput{
		Category:  req.Category,
		Title:     req.Title,
		Content:   req.Content,
		Images:    req.Images,
		Contact:   req.Contact,
		RequestID: req.RequestID,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.FeedbackForUserFromService(updated))
}

func (h *FeedbackHandler) Verify(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	feedbackID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || feedbackID <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}
	var req verifyFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.feedbackService.VerifyByUser(c.Request.Context(), subject.UserID, feedbackID, service.VerifyFeedbackInput{Resolved: req.Resolved, Note: req.Note}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *FeedbackHandler) ListNotifications(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	result, err := h.feedbackService.ListNotifications(c.Request.Context(), subject.UserID, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	items := make([]dto.UserNotification, 0, len(result.Items))
	for i := range result.Items {
		items = append(items, *dto.UserNotificationFromService(&result.Items[i]))
	}
	response.Success(c, gin.H{"items": items, "unread_count": result.UnreadCount})
}

func (h *FeedbackHandler) MarkNotificationRead(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid notification ID")
		return
	}
	if err := h.feedbackService.MarkNotificationRead(c.Request.Context(), subject.UserID, id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *FeedbackHandler) MarkAllNotificationsRead(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	updated, err := h.feedbackService.MarkAllNotificationsRead(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"updated": updated})
}

func (h *FeedbackHandler) GetImage(c *gin.Context) {
	url, err := h.feedbackService.GetFeedbackImageAccess(c.Request.Context(), c.Param("token"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func (h *FeedbackHandler) CreateReply(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	feedbackID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || feedbackID <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}

	var req createFeedbackReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	reply, err := h.feedbackService.ReplyByUser(c.Request.Context(), subject.UserID, feedbackID, service.CreateFeedbackReplyInput{
		Content: req.Content,
		Images:  req.Images,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.FeedbackReplyFromService(reply))
}

func (h *FeedbackHandler) UploadImage(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file is required")
		return
	}
	if fileHeader.Size <= 0 {
		response.BadRequest(c, "empty file is not allowed")
		return
	}
	if fileHeader.Size > 5<<20 {
		response.BadRequest(c, "file size must be at most 5MB")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to open file")
		return
	}
	defer func() { _ = file.Close() }()

	header := make([]byte, 512)
	n, readErr := file.Read(header)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, http.ErrBodyReadAfterClose) {
		response.Error(c, http.StatusInternalServerError, "failed to read file")
		return
	}

	contentType := http.DetectContentType(header[:n])
	switch contentType {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
	default:
		response.BadRequest(c, "only jpg/png/gif/webp images are supported")
		return
	}

	if seeker, ok := file.(interface {
		Seek(offset int64, whence int) (int64, error)
	}); ok {
		if _, err := seeker.Seek(0, 0); err != nil {
			response.Error(c, http.StatusInternalServerError, "failed to reset file reader")
			return
		}
	}

	uploadedURL, err := h.feedbackService.UploadImage(
		c.Request.Context(),
		subject.UserID,
		filepath.Base(fileHeader.Filename),
		contentType,
		fileHeader.Size,
		file,
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Created(c, uploadFeedbackImageResponse{URL: uploadedURL})
}

func writeRetryAfterHeader(c *gin.Context, err error) {
	if !infraerrors.IsTooManyRequests(err) {
		return
	}
	appErr := infraerrors.FromError(err)
	if appErr == nil || appErr.Metadata == nil {
		return
	}
	if retryAfter := strings.TrimSpace(appErr.Metadata["retry_after"]); retryAfter != "" {
		c.Header("Retry-After", retryAfter)
	}
}

package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/dto"
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

type updateFeedbackStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

type updateFeedbackPriorityRequest struct {
	Priority string `json:"priority" binding:"required"`
}

type batchUpdateFeedbackStatusRequest struct {
	IDs    []int64 `json:"ids" binding:"required"`
	Status string  `json:"status" binding:"required"`
}

type createAdminFeedbackReplyRequest struct {
	Content string   `json:"content" binding:"required"`
	Images  []string `json:"images"`
}

type triageFeedbackRequest struct {
	TriageStatus         string   `json:"triage_status" binding:"required"`
	TriagePriority       string   `json:"triage_priority" binding:"required"`
	TriageSummary        string   `json:"triage_summary" binding:"required"`
	TriageConfidence     *float64 `json:"triage_confidence"`
	RepairDifficulty     string   `json:"repair_difficulty" binding:"required"`
	RepairRecommendation string   `json:"repair_recommendation"`
	DuplicateOfID        *int64   `json:"duplicate_of_id"`
}

type acceptFeedbackBatchRequest struct {
	IDs           []int64 `json:"ids" binding:"required"`
	BatchID       string  `json:"batch_id" binding:"required"`
	OwnerOverride bool    `json:"owner_override"`
}

type completeFeedbackRequest struct {
	ResolvedVersion string `json:"resolved_version"`
	NotifyInApp     bool   `json:"notify_in_app"`
}

type recordFeedbackNotificationRequest struct {
	Channel           string `json:"channel" binding:"required"`
	DeliveryReference string `json:"delivery_reference" binding:"required"`
}

func (h *FeedbackHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	filters, err := parseAdminFeedbackFilters(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	items, result, err := h.feedbackService.ListForAdmin(c.Request.Context(), params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.Feedback, 0, len(items))
	for i := range items {
		out = append(out, *dto.FeedbackFromService(&items[i]))
	}
	response.PaginatedWithResult(c, out, &response.PaginationResult{
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
		Pages:    result.Pages,
	})
}

func (h *FeedbackHandler) GetByID(c *gin.Context) {
	feedbackID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || feedbackID <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}

	item, err := h.feedbackService.GetForAdmin(c.Request.Context(), feedbackID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.FeedbackDetailFromService(item))
}

func (h *FeedbackHandler) AgentQueue(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	items, result, err := h.feedbackService.ListAgentQueue(c.Request.Context(), params)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.Feedback, 0, len(items))
	for i := range items {
		out = append(out, *dto.FeedbackFromService(&items[i]))
	}
	response.PaginatedWithResult(c, out, &response.PaginationResult{Total: result.Total, Page: result.Page, PageSize: result.PageSize, Pages: result.Pages})
}

func (h *FeedbackHandler) AgentTriage(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}
	var req triageFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.feedbackService.TriageByAgent(c.Request.Context(), id, service.AgentTriageFeedbackInput{TriageStatus: req.TriageStatus, TriagePriority: req.TriagePriority, TriageSummary: req.TriageSummary, TriageConfidence: req.TriageConfidence, RepairDifficulty: req.RepairDifficulty, RepairRecommendation: req.RepairRecommendation, DuplicateOfID: req.DuplicateOfID})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.FeedbackFromService(item))
}

func (h *FeedbackHandler) AcceptBatch(c *gin.Context) {
	var req acceptFeedbackBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var operatorID *int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		operatorID = &subject.UserID
	}
	results := h.feedbackService.AcceptBatch(c.Request.Context(), service.AcceptFeedbackBatchInput{IDs: req.IDs, BatchID: req.BatchID, OperatorUserID: operatorID, OwnerOverride: req.OwnerOverride})
	response.Success(c, gin.H{"results": results, "reward_amount": 5.0})
}

func (h *FeedbackHandler) MarkFixing(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}
	var operatorID *int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		operatorID = &subject.UserID
	}
	if err := h.feedbackService.MarkFixing(c.Request.Context(), id, operatorID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *FeedbackHandler) Complete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}
	var req completeFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var operatorID *int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		operatorID = &subject.UserID
	}
	if err := h.feedbackService.Complete(c.Request.Context(), id, service.CompleteFeedbackInput{ResolvedVersion: req.ResolvedVersion, NotifyInApp: req.NotifyInApp, OperatorUserID: operatorID}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *FeedbackHandler) RecordNotification(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}
	var req recordFeedbackNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	var operatorID *int64
	if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
		operatorID = &subject.UserID
	}
	if err := h.feedbackService.RecordNotification(c.Request.Context(), id, service.RecordFeedbackNotificationInput{Channel: req.Channel, DeliveryReference: req.DeliveryReference, OperatorUserID: operatorID}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *FeedbackHandler) ListRewards(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	filters := service.FeedbackRewardListFilters{BatchID: strings.TrimSpace(c.Query("batch_id"))}
	if raw := strings.TrimSpace(c.Query("user_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id <= 0 {
			response.BadRequest(c, "Invalid user ID")
			return
		}
		filters.UserID = &id
	}
	items, result, err := h.feedbackService.ListRewards(c.Request.Context(), params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.FeedbackReward, 0, len(items))
	for i := range items {
		out = append(out, *dto.FeedbackRewardFromService(&items[i]))
	}
	response.PaginatedWithResult(c, out, &response.PaginationResult{Total: result.Total, Page: result.Page, PageSize: result.PageSize, Pages: result.Pages})
}

func (h *FeedbackHandler) CreateReply(c *gin.Context) {
	feedbackID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || feedbackID <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	var req createAdminFeedbackReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	reply, err := h.feedbackService.ReplyByAdmin(c.Request.Context(), subject.UserID, feedbackID, service.CreateFeedbackReplyInput{
		Content: req.Content,
		Images:  req.Images,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.FeedbackReplyFromService(reply))
}

func (h *FeedbackHandler) UpdateStatus(c *gin.Context) {
	feedbackID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || feedbackID <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}

	var req updateFeedbackStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.feedbackService.UpdateStatus(c.Request.Context(), feedbackID, service.UpdateFeedbackStatusInput{Status: req.Status}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *FeedbackHandler) UpdatePriority(c *gin.Context) {
	feedbackID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || feedbackID <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}

	var req updateFeedbackPriorityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if err := h.feedbackService.UpdatePriority(c.Request.Context(), feedbackID, service.UpdateFeedbackPriorityInput{Priority: req.Priority}); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *FeedbackHandler) BatchUpdateStatus(c *gin.Context) {
	var req batchUpdateFeedbackStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	affected, err := h.feedbackService.BatchUpdateStatus(c.Request.Context(), service.BatchUpdateFeedbackStatusInput{
		IDs:    req.IDs,
		Status: req.Status,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok", "updated": affected})
}

func (h *FeedbackHandler) Delete(c *gin.Context) {
	feedbackID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || feedbackID <= 0 {
		response.BadRequest(c, "Invalid feedback ID")
		return
	}

	if err := h.feedbackService.Delete(c.Request.Context(), feedbackID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok"})
}

func (h *FeedbackHandler) BatchDelete(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	affected, err := h.feedbackService.BatchDelete(c.Request.Context(), req.IDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "ok", "deleted": affected})
}

func parseAdminFeedbackFilters(c *gin.Context) (service.AdminFeedbackListFilters, error) {
	filters := service.AdminFeedbackListFilters{
		Category:      strings.TrimSpace(c.Query("category")),
		Status:        strings.TrimSpace(c.Query("status")),
		Priority:      strings.TrimSpace(c.Query("priority")),
		Search:        strings.TrimSpace(c.Query("search")),
		TriageStatus:  strings.TrimSpace(c.Query("triage_status")),
		OwnerDecision: strings.TrimSpace(c.Query("owner_decision")),
		FixStatus:     strings.TrimSpace(c.Query("fix_status")),
	}
	if start := strings.TrimSpace(c.Query("start_time")); start != "" {
		parsed, err := time.Parse(time.RFC3339, start)
		if err != nil {
			return filters, err
		}
		filters.StartTime = &parsed
	}
	if end := strings.TrimSpace(c.Query("end_time")); end != "" {
		parsed, err := time.Parse(time.RFC3339, end)
		if err != nil {
			return filters, err
		}
		filters.EndTime = &parsed
	}
	return filters, nil
}

package handler

import (
	"net/http"
	"strconv"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CompensationHandler struct {
	service   *service.CompensationControlService
	execution *service.CompensationExecutionService
}

func NewCompensationHandler(compensation *service.CompensationControlService, execution *service.CompensationExecutionService) *CompensationHandler {
	return &CompensationHandler{service: compensation, execution: execution}
}

type compensationApprovalRequest struct {
	ApprovalKey  string `json:"approval_key" binding:"required"`
	Reason       string `json:"reason" binding:"required"`
	Confirmation string `json:"confirmation" binding:"required"`
	PreviewHash  string `json:"preview_hash" binding:"required"`
}

func (h *CompensationHandler) ApproveDraft(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid draft id")
		return
	}
	var request compensationApprovalRequest
	if err = c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	result, err := h.execution.ApproveDraft(c.Request.Context(), service.CompensationApprovalCommand{DraftID: id, ApprovalKey: request.ApprovalKey, Reason: request.Reason, Confirmation: request.Confirmation, PreviewHash: request.PreviewHash, ActorUserID: statusActorUserID(c)})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}
func (h *CompensationHandler) ExecuteDraft(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid draft id")
		return
	}
	result, err := h.execution.ExecuteApprovedDraft(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusConflict, response.Response{Code: http.StatusConflict, Message: err.Error(), Data: result})
		return
	}
	response.Success(c, result)
}

type compensationExecutionSettingsRequest struct {
	Enabled      bool   `json:"enabled"`
	Confirmation string `json:"confirmation"`
}

func (h *CompensationHandler) UpdateExecutionSettings(c *gin.Context) {
	var request compensationExecutionSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.execution.UpdateEnabled(c.Request.Context(), service.CompensationExecutionSettingsCommand{Enabled: request.Enabled, Confirmation: request.Confirmation, ActorUserID: statusActorUserID(c)}); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"enabled": request.Enabled})
}
func (h *CompensationHandler) GetExecution(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid execution id")
		return
	}
	result, err := h.execution.GetExecution(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Execution not found")
		return
	}
	response.Success(c, result)
}
func (h *CompensationHandler) GetExecutionPreview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid draft id")
		return
	}
	result, err := h.execution.Preview(c.Request.Context(), id)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

type erroneousChargeRefundRequest struct {
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
	UsageLogID     int64  `json:"usage_log_id" binding:"required"`
	UserID         int64  `json:"user_id" binding:"required"`
	Reason         string `json:"reason" binding:"required"`
}

func (h *CompensationHandler) RefundErroneousCharge(c *gin.Context) {
	var request erroneousChargeRefundRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	result, err := h.execution.RefundErroneousCharge(c.Request.Context(), service.ErroneousChargeRefundCommand{IdempotencyKey: request.IdempotencyKey, UsageLogID: request.UsageLogID, UserID: request.UserID, Reason: request.Reason, ActorUserID: statusActorUserID(c)})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}
func (h *CompensationHandler) GetSnapshot(c *gin.Context) {
	result, err := h.service.AdminSnapshot(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Compensation Shadow data is temporarily unavailable")
		return
	}
	response.Success(c, result)
}
func (h *CompensationHandler) GetDraft(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid draft id")
		return
	}
	result, err := h.service.GetDraft(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Compensation draft not found")
		return
	}
	response.Success(c, result)
}
func (h *CompensationHandler) GenerateDraft(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid incident id")
		return
	}
	result, err := h.service.DraftIncident(c.Request.Context(), id, statusActorUserID(c))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

type compensationRevisionRequest struct {
	Reason      string                                   `json:"reason" binding:"required"`
	Adjustments []service.CompensationRevisionAdjustment `json:"adjustments" binding:"required"`
}

func (h *CompensationHandler) ReviseDraft(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid draft id")
		return
	}
	var request compensationRevisionRequest
	if err = c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	result, err := h.service.ReviseDraft(c.Request.Context(), service.CompensationRevisionCommand{DraftID: id, Reason: request.Reason, ActorUserID: statusActorUserID(c), Adjustments: request.Adjustments})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

type compensationReviewRequest struct {
	OwnerJudgementTotalCNYFen int64  `json:"owner_judgement_total_cny_fen"`
	Result                    string `json:"result" binding:"required"`
	Notes                     string `json:"notes" binding:"required"`
}

func (h *CompensationHandler) ReviewDraft(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid draft id")
		return
	}
	var request compensationReviewRequest
	if err = c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	result, err := h.service.ReviewShadowDraft(c.Request.Context(), service.CompensationShadowReviewCommand{DraftID: id, OwnerJudgementTotalCNYFen: request.OwnerJudgementTotalCNYFen, Result: request.Result, Notes: request.Notes, ActorUserID: statusActorUserID(c)})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

type compensationSettingsRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *CompensationHandler) UpdateSettings(c *gin.Context) {
	var request compensationSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.service.UpdateShadowEnabled(c.Request.Context(), request.Enabled, statusActorUserID(c)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"enabled": request.Enabled})
}

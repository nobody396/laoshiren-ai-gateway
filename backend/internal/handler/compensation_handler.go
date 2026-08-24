package handler

import (
	"net/http"
	"strconv"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CompensationHandler struct {
	service *service.CompensationControlService
}

func NewCompensationHandler(compensation *service.CompensationControlService) *CompensationHandler {
	return &CompensationHandler{service: compensation}
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

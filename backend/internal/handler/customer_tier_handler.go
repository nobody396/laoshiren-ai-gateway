package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CustomerTierHandler struct{ service *service.CustomerTierService }

func NewCustomerTierHandler(tierService *service.CustomerTierService) *CustomerTierHandler {
	return &CustomerTierHandler{service: tierService}
}

func (h *CustomerTierHandler) GetSnapshot(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))
	snapshot, err := h.service.AdminSnapshot(c.Request.Context(), service.AdminCustomerTierFilter{Page: page, PageSize: pageSize, Search: c.Query("search"), Tier: service.CustomerTier(c.Query("tier"))})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "Customer Tier data is temporarily unavailable")
		return
	}
	response.Success(c, snapshot)
}

type customerTierRefundRequest struct {
	IdempotencyKey string     `json:"idempotency_key" binding:"required"`
	SourceType     string     `json:"source_type" binding:"required"`
	SourceID       int64      `json:"source_id" binding:"required"`
	AmountCNYFen   int64      `json:"amount_cny_fen" binding:"required"`
	Reason         string     `json:"reason" binding:"required"`
	RefundedAt     *time.Time `json:"refunded_at"`
}

func (h *CustomerTierHandler) RecordRefund(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user id")
		return
	}
	var request customerTierRefundRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	refundedAt := time.Now().UTC()
	if request.RefundedAt != nil {
		refundedAt = request.RefundedAt.UTC()
	}
	item, err := h.service.RecordPaidValueRefund(c.Request.Context(), service.CustomerPaidValueRefundCommand{IdempotencyKey: request.IdempotencyKey, UserID: userID, SourceType: request.SourceType, SourceID: request.SourceID, AmountCNYFen: request.AmountCNYFen, Reason: request.Reason, ActorUserID: statusActorUserID(c), RefundedAt: refundedAt})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, item)
}

func (h *CustomerTierHandler) GetUser(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user id")
		return
	}
	explanation, err := h.service.ExplainUser(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "Customer Tier evaluation not found")
		return
	}
	response.Success(c, explanation)
}

type customerTierSettingsRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *CustomerTierHandler) UpdateSettings(c *gin.Context) {
	var request customerTierSettingsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	if err := h.service.UpdateEnabled(c.Request.Context(), request.Enabled); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, gin.H{"enabled": request.Enabled})
}

type customerTierOverrideRequest struct {
	Tier         service.CustomerTier `json:"tier" binding:"required"`
	Reason       string               `json:"reason" binding:"required"`
	StartsAt     *time.Time           `json:"starts_at"`
	DurationDays int                  `json:"duration_days" binding:"required"`
}

func (h *CustomerTierHandler) CreateOverride(c *gin.Context) {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || userID <= 0 {
		response.BadRequest(c, "Invalid user id")
		return
	}
	var request customerTierOverrideRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		response.BadRequest(c, "Invalid request body")
		return
	}
	startsAt := time.Now().UTC()
	if request.StartsAt != nil {
		startsAt = request.StartsAt.UTC()
	}
	item, err := h.service.CreateOverride(c.Request.Context(), service.CustomerTierOverrideCommand{UserID: userID, Tier: request.Tier, Reason: request.Reason, StartsAt: startsAt, ExpiresAt: startsAt.Add(time.Duration(request.DurationDays) * 24 * time.Hour), ActorUserID: statusActorUserID(c)})
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, item)
}

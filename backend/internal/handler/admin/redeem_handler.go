package admin

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/handler/dto"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// RedeemHandler handles admin redeem code management
type RedeemHandler struct {
	adminService  service.AdminService
	redeemService *service.RedeemService
}

// NewRedeemHandler creates a new admin redeem handler
func NewRedeemHandler(adminService service.AdminService, redeemService *service.RedeemService) *RedeemHandler {
	return &RedeemHandler{
		adminService:  adminService,
		redeemService: redeemService,
	}
}

// GenerateRedeemCodesRequest represents generate redeem codes request
type GenerateRedeemCodesRequest struct {
	Count        int     `json:"count" binding:"required,min=1,max=100"`
	Type         string  `json:"type" binding:"required,oneof=balance concurrency subscription invitation"`
	Value        float64 `json:"value" binding:"min=0"`
	GroupID      *int64  `json:"group_id"`                                    // 订阅类型旧版单分组字段
	GroupIDs     []int64 `json:"group_ids"`                                   // 订阅组合包字段；传入后可一次分配多个分组
	ValidityDays int     `json:"validity_days" binding:"omitempty,max=36500"` // 订阅类型使用，默认30天，最大100年

	BatchName        string `json:"batch_name"`
	Purpose          string `json:"purpose" binding:"omitempty,oneof=sale_recharge gift compensation internal_test migration"`
	SalesStatus      string `json:"sales_status" binding:"omitempty,oneof=inventory sold gifted void"`
	SalesChannel     string `json:"sales_channel"`
	ExternalURL      string `json:"external_url"`
	SoldToNote       string `json:"sold_to_note"`
	ExternalOrderNo  string `json:"external_order_no"`
	ExternalOrderURL string `json:"external_order_url"`
	InternalNotes    string `json:"internal_notes"`
}

// CreateAndRedeemCodeRequest represents creating a fixed code and redeeming it for a target user.
// Type 为 omitempty 而非 required 是为了向后兼容旧版调用方（不传 type 时默认 balance）。
type CreateAndRedeemCodeRequest struct {
	Code         string  `json:"code" binding:"required,min=3,max=128"`
	Type         string  `json:"type" binding:"omitempty,oneof=balance concurrency subscription invitation"` // 不传时默认 balance（向后兼容）
	Value        float64 `json:"value" binding:"required,gt=0"`
	UserID       int64   `json:"user_id" binding:"required,gt=0"`
	GroupID      *int64  `json:"group_id"`                                    // subscription 旧版单分组字段
	GroupIDs     []int64 `json:"group_ids"`                                   // subscription 组合包字段
	ValidityDays int     `json:"validity_days" binding:"omitempty,max=36500"` // subscription 类型必填，>0
	Notes        string  `json:"notes"`
}

type BatchUpdateRedeemCodeBillingRequest struct {
	IDs    []int64                            `json:"ids" binding:"required,min=1,max=100,dive,gt=0"`
	Fields BatchUpdateRedeemCodeBillingFields `json:"fields" binding:"required"`
}

type BatchUpdateRedeemCodeBillingFields struct {
	Purpose          *string `json:"purpose" binding:"omitempty,oneof=sale_recharge gift compensation internal_test migration"`
	SalesStatus      *string `json:"sales_status" binding:"omitempty,oneof=inventory sold gifted void"`
	SoldToNote       *string `json:"sold_to_note"`
	ExternalOrderNo  *string `json:"external_order_no"`
	ExternalOrderURL *string `json:"external_order_url"`
	InternalNotes    *string `json:"internal_notes"`
}

// List handles listing all redeem codes with pagination
// GET /api/v1/admin/redeem-codes
func (h *RedeemHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	codeType := c.Query("type")
	status := c.Query("status")
	search := c.Query("search")
	// 标准化和验证 search 参数
	search = strings.TrimSpace(search)
	if len(search) > 100 {
		search = search[:100]
	}

	codes, total, err := h.adminService.ListRedeemCodes(c.Request.Context(), page, pageSize, codeType, status, search)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.AdminRedeemCode, 0, len(codes))
	for i := range codes {
		out = append(out, *dto.RedeemCodeFromServiceAdmin(&codes[i]))
	}
	response.Paginated(c, out, total, page, pageSize)
}

// GetByID handles getting a redeem code by ID
// GET /api/v1/admin/redeem-codes/:id
func (h *RedeemHandler) GetByID(c *gin.Context) {
	codeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid redeem code ID")
		return
	}

	code, err := h.adminService.GetRedeemCode(c.Request.Context(), codeID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.RedeemCodeFromServiceAdmin(code))
}

// Generate handles generating new redeem codes
// POST /api/v1/admin/redeem-codes/generate
func (h *RedeemHandler) Generate(c *gin.Context) {
	var req GenerateRedeemCodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.redeem_codes.generate", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		// Only attach created_by for real admin users. Admin API Key auth uses a
		// synthetic principal (UserID=-1) which is not a valid users FK target.
		var createdBy *int64
		if subject, ok := middleware2.GetAuthSubjectFromContext(c); ok && subject.UserID > 0 {
			createdBy = &subject.UserID
		}
		codes, execErr := h.adminService.GenerateRedeemCodes(ctx, &service.GenerateRedeemCodesInput{
			Count:            req.Count,
			Type:             req.Type,
			Value:            req.Value,
			GroupID:          req.GroupID,
			GroupIDs:         req.GroupIDs,
			ValidityDays:     req.ValidityDays,
			BatchName:        req.BatchName,
			Purpose:          req.Purpose,
			SalesStatus:      req.SalesStatus,
			SalesChannel:     req.SalesChannel,
			ExternalURL:      req.ExternalURL,
			SoldToNote:       req.SoldToNote,
			ExternalOrderNo:  req.ExternalOrderNo,
			ExternalOrderURL: req.ExternalOrderURL,
			InternalNotes:    req.InternalNotes,
			CreatedBy:        createdBy,
		})
		if execErr != nil {
			return nil, execErr
		}

		out := make([]dto.AdminRedeemCode, 0, len(codes))
		for i := range codes {
			out = append(out, *dto.RedeemCodeFromServiceAdmin(&codes[i]))
		}
		return out, nil
	})
}

// BatchUpdateBilling changes reconciliation metadata only. It deliberately
// cannot alter the redeemed user, code status, value, groups, validity, or any
// entitlement granted by a prior redemption.
// POST /api/v1/admin/redeem-codes/batch-update
func (h *RedeemHandler) BatchUpdateBilling(c *gin.Context) {
	var req BatchUpdateRedeemCodeBillingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Fields.Purpose == nil &&
		req.Fields.SalesStatus == nil &&
		req.Fields.SoldToNote == nil &&
		req.Fields.ExternalOrderNo == nil &&
		req.Fields.ExternalOrderURL == nil &&
		req.Fields.InternalNotes == nil {
		response.BadRequest(c, "At least one billing field is required")
		return
	}

	updated, err := h.adminService.BatchUpdateRedeemCodeBilling(
		c.Request.Context(),
		&service.BatchUpdateRedeemCodeBillingInput{
			IDs: req.IDs,
			Fields: service.RedeemCodeBillingUpdateFields{
				Purpose:          req.Fields.Purpose,
				SalesStatus:      req.Fields.SalesStatus,
				SoldToNote:       req.Fields.SoldToNote,
				ExternalOrderNo:  req.Fields.ExternalOrderNo,
				ExternalOrderURL: req.Fields.ExternalOrderURL,
				InternalNotes:    req.Fields.InternalNotes,
			},
		},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.AdminRedeemCode, 0, len(updated))
	for i := range updated {
		out = append(out, *dto.RedeemCodeFromServiceAdmin(&updated[i]))
	}
	response.Success(c, gin.H{"updated": out, "count": len(out)})
}

// CreateAndRedeem creates a fixed redeem code and redeems it for a target user in one step.
// POST /api/v1/admin/redeem-codes/create-and-redeem
func (h *RedeemHandler) CreateAndRedeem(c *gin.Context) {
	if h.redeemService == nil {
		response.InternalError(c, "redeem service not configured")
		return
	}

	var req CreateAndRedeemCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	req.Code = strings.TrimSpace(req.Code)
	// 向后兼容：旧版调用方（如 Sub2ApiPay）不传 type 字段，默认当作 balance 充值处理。
	// 请勿删除此默认值逻辑，否则会导致旧版调用方 400 报错。
	if req.Type == "" {
		req.Type = "balance"
	}

	if req.Type == "subscription" {
		if req.GroupID == nil && len(req.GroupIDs) == 0 {
			response.BadRequest(c, "group_id or group_ids is required for subscription type")
			return
		}
		if req.ValidityDays <= 0 {
			response.BadRequest(c, "validity_days must be greater than 0 for subscription type")
			return
		}
	}

	executeAdminIdempotentJSON(c, "admin.redeem_codes.create_and_redeem", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		existing, err := h.redeemService.GetByCode(ctx, req.Code)
		if err == nil {
			return h.resolveCreateAndRedeemExisting(ctx, existing, req.UserID)
		}
		if !errors.Is(err, service.ErrRedeemCodeNotFound) {
			return nil, err
		}

		createErr := h.redeemService.CreateCode(ctx, &service.RedeemCode{
			Code:         req.Code,
			Type:         req.Type,
			Value:        req.Value,
			Status:       service.StatusUnused,
			Notes:        req.Notes,
			GroupID:      req.GroupID,
			GroupIDs:     req.GroupIDs,
			ValidityDays: req.ValidityDays,
		})
		if createErr != nil {
			// Unique code race: if code now exists, use idempotent semantics by used_by.
			existingAfterCreateErr, getErr := h.redeemService.GetByCode(ctx, req.Code)
			if getErr == nil {
				return h.resolveCreateAndRedeemExisting(ctx, existingAfterCreateErr, req.UserID)
			}
			return nil, createErr
		}

		redeemed, redeemErr := h.redeemService.Redeem(ctx, req.UserID, req.Code)
		if redeemErr != nil {
			return nil, redeemErr
		}
		return gin.H{"redeem_code": dto.RedeemCodeFromServiceAdmin(redeemed)}, nil
	})
}

func (h *RedeemHandler) resolveCreateAndRedeemExisting(ctx context.Context, existing *service.RedeemCode, userID int64) (any, error) {
	if existing == nil {
		return nil, infraerrors.Conflict("REDEEM_CODE_CONFLICT", "redeem code conflict")
	}

	// If previous run created the code but crashed before redeem, redeem it now.
	if existing.CanUse() {
		redeemed, err := h.redeemService.Redeem(ctx, userID, existing.Code)
		if err == nil {
			return gin.H{"redeem_code": dto.RedeemCodeFromServiceAdmin(redeemed)}, nil
		}
		if !errors.Is(err, service.ErrRedeemCodeUsed) {
			return nil, err
		}
		latest, getErr := h.redeemService.GetByCode(ctx, existing.Code)
		if getErr == nil {
			existing = latest
		}
	}

	if existing.UsedBy != nil && *existing.UsedBy == userID {
		return gin.H{"redeem_code": dto.RedeemCodeFromServiceAdmin(existing)}, nil
	}

	return nil, infraerrors.Conflict("REDEEM_CODE_CONFLICT", "redeem code already used by another user")
}

// Delete handles deleting a redeem code
// DELETE /api/v1/admin/redeem-codes/:id
func (h *RedeemHandler) Delete(c *gin.Context) {
	codeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid redeem code ID")
		return
	}

	err = h.adminService.DeleteRedeemCode(c.Request.Context(), codeID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Redeem code deleted successfully"})
}

// BatchDelete handles batch deleting redeem codes
// POST /api/v1/admin/redeem-codes/batch-delete
func (h *RedeemHandler) BatchDelete(c *gin.Context) {
	var req struct {
		IDs []int64 `json:"ids" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	deleted, err := h.adminService.BatchDeleteRedeemCodes(c.Request.Context(), req.IDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"deleted": deleted,
		"message": "Redeem codes deleted successfully",
	})
}

// Expire handles expiring a redeem code
// POST /api/v1/admin/redeem-codes/:id/expire
func (h *RedeemHandler) Expire(c *gin.Context) {
	codeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid redeem code ID")
		return
	}

	code, err := h.adminService.ExpireRedeemCode(c.Request.Context(), codeID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.RedeemCodeFromServiceAdmin(code))
}

// GetStats handles getting redeem code statistics
// GET /api/v1/admin/redeem-codes/stats
func (h *RedeemHandler) GetStats(c *gin.Context) {
	// Return mock data for now
	response.Success(c, gin.H{
		"total_codes":             0,
		"active_codes":            0,
		"used_codes":              0,
		"expired_codes":           0,
		"total_value_distributed": 0.0,
		"by_type": gin.H{
			"balance":     0,
			"concurrency": 0,
			"trial":       0,
		},
	})
}

// ListBilling handles card-code billing/reconciliation records.
// GET /api/v1/admin/redeem-codes/billing
func (h *RedeemHandler) ListBilling(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)

	batchID, err := parseOptionalInt64Query(c.Query("batch_id"))
	if err != nil {
		response.BadRequest(c, "Invalid batch_id")
		return
	}
	amountMin, err := parseOptionalFloatQuery(c.Query("amount_min"))
	if err != nil {
		response.BadRequest(c, "Invalid amount_min")
		return
	}
	amountMax, err := parseOptionalFloatQuery(c.Query("amount_max"))
	if err != nil {
		response.BadRequest(c, "Invalid amount_max")
		return
	}
	usedStart, usedEnd, err := parseFlexibleTimeRange(c.Query("used_start_time"), c.Query("used_end_time"))
	if err != nil {
		response.BadRequest(c, "Invalid used time range")
		return
	}
	createdStart, createdEnd, err := parseFlexibleTimeRange(c.Query("created_start_time"), c.Query("created_end_time"))
	if err != nil {
		response.BadRequest(c, "Invalid created time range")
		return
	}

	result, err := h.adminService.ListRedeemCodeBilling(c.Request.Context(), page, pageSize, service.RedeemCodeBillingFilters{
		Search:            strings.TrimSpace(c.Query("search")),
		Purpose:           strings.TrimSpace(c.Query("purpose")),
		SalesStatus:       strings.TrimSpace(c.Query("sales_status")),
		RedeemStatus:      strings.TrimSpace(c.Query("redeem_status")),
		BatchID:           batchID,
		AmountMin:         amountMin,
		AmountMax:         amountMax,
		UsedStartTime:     usedStart,
		UsedEndTime:       usedEnd,
		CreatedStartTime:  createdStart,
		CreatedEndTime:    createdEnd,
		IncludeNonRevenue: c.Query("include_non_revenue") == "true",
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// ListClassificationAnomalies lists redeemed paid-inventory candidates that
// require evidence review instead of automatic sale inference.
// GET /api/v1/admin/redeem-codes/classification-anomalies
func (h *RedeemHandler) ListClassificationAnomalies(c *gin.Context) {
	result, err := h.adminService.ListRedeemCodeClassificationAnomalies(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func parseOptionalInt64Query(raw string) (*int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseOptionalFloatQuery(raw string) (*float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

// Export handles exporting redeem codes to CSV
// GET /api/v1/admin/redeem-codes/export
func (h *RedeemHandler) Export(c *gin.Context) {
	codeType := c.Query("type")
	status := c.Query("status")
	redeemURLBase := strings.TrimRight(strings.TrimSpace(c.Query("redeem_url_base")), "/")
	includeRedeemURL := c.Query("include_redeem_url") == "true" && redeemURLBase != ""
	if includeRedeemURL {
		parsed, err := url.Parse(redeemURLBase)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			response.BadRequest(c, "redeem_url_base must be an absolute http(s) URL")
			return
		}
	}

	// Get all codes without pagination (use large page size)
	codes, _, err := h.adminService.ListRedeemCodes(c.Request.Context(), 1, 10000, codeType, status, "")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	// Create CSV buffer
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{"id", "code", "type", "value", "status", "used_by", "used_by_email", "used_at", "created_at"}
	if includeRedeemURL {
		header = append(header, "redeem_url")
	}
	if err := writer.Write(header); err != nil {
		response.InternalError(c, "Failed to export redeem codes: "+err.Error())
		return
	}

	// Write data rows
	for _, code := range codes {
		usedBy := ""
		if code.UsedBy != nil {
			usedBy = fmt.Sprintf("%d", *code.UsedBy)
		}
		usedByEmail := ""
		if code.User != nil {
			usedByEmail = code.User.Email
		}
		usedAt := ""
		if code.UsedAt != nil {
			usedAt = code.UsedAt.Format("2006-01-02 15:04:05")
		}
		row := []string{
			fmt.Sprintf("%d", code.ID),
			code.Code,
			code.Type,
			fmt.Sprintf("%.2f", code.Value),
			code.Status,
			usedBy,
			usedByEmail,
			usedAt,
			code.CreatedAt.Format("2006-01-02 15:04:05"),
		}
		if includeRedeemURL {
			row = append(row, redeemURLBase+"/redeem?code="+url.QueryEscape(code.Code))
		}
		if err := writer.Write(row); err != nil {
			response.InternalError(c, "Failed to export redeem codes: "+err.Error())
			return
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		response.InternalError(c, "Failed to export redeem codes: "+err.Error())
		return
	}

	c.Header("Content-Type", "text/csv")
	c.Header("Content-Disposition", "attachment; filename=redeem_codes.csv")
	c.Data(200, "text/csv", buf.Bytes())
}

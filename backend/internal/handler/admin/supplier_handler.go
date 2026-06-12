package admin

import (
	"io"
	"strconv"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// SupplierHandler handles admin supplier evaluation management.
type SupplierHandler struct {
	supplierService *service.SupplierService
}

func NewSupplierHandler(supplierService *service.SupplierService) *SupplierHandler {
	return &SupplierHandler{supplierService: supplierService}
}

type createSupplierRequest struct {
	Name                 string   `json:"name" binding:"required,max=100"`
	WebsiteURL           string   `json:"website_url" binding:"omitempty,max=500"`
	BaseURL              string   `json:"base_url" binding:"omitempty,max=500"`
	APIKey               string   `json:"api_key" binding:"omitempty,max=4000"`
	UpstreamGroup        string   `json:"upstream_group" binding:"omitempty,max=100"`
	ContactPlatform      string   `json:"contact_platform" binding:"omitempty,oneof=wechat telegram qq email phone other"`
	ContactValue         string   `json:"contact_value" binding:"omitempty,max=200"`
	Status               string   `json:"status" binding:"omitempty,oneof=evaluating active"`
	CostRMBPerUSD        *float64 `json:"cost_rmb_per_usd" binding:"omitempty,min=0"`
	Notes                string   `json:"notes"`
	ProbeEnabled         *bool    `json:"probe_enabled"`
	ProbeModel           string   `json:"probe_model" binding:"omitempty,max=100"`
	ProbeIntervalMinutes int      `json:"probe_interval_minutes" binding:"omitempty,min=1,max=1440"`
	TargetGroupIDs       []int64  `json:"target_group_ids"`
}

type updateSupplierRequest struct {
	Name                 *string  `json:"name" binding:"omitempty,max=100"`
	WebsiteURL           *string  `json:"website_url" binding:"omitempty,max=500"`
	BaseURL              *string  `json:"base_url" binding:"omitempty,max=500"`
	APIKey               *string  `json:"api_key" binding:"omitempty,max=4000"`
	UpstreamGroup        *string  `json:"upstream_group" binding:"omitempty,max=100"`
	ContactPlatform      *string  `json:"contact_platform" binding:"omitempty,oneof=wechat telegram qq email phone other"`
	ContactValue         *string  `json:"contact_value" binding:"omitempty,max=200"`
	Status               *string  `json:"status" binding:"omitempty,oneof=evaluating active"`
	CostRMBPerUSD        *float64 `json:"cost_rmb_per_usd" binding:"omitempty,min=0"`
	Notes                *string  `json:"notes"`
	ProbeEnabled         *bool    `json:"probe_enabled"`
	ProbeModel           *string  `json:"probe_model" binding:"omitempty,max=100"`
	ProbeIntervalMinutes *int     `json:"probe_interval_minutes" binding:"omitempty,min=1,max=1440"`
	TargetGroupIDs       *[]int64 `json:"target_group_ids"`
}

type bulkSupplierProbeEnabledRequest struct {
	IDs     []int64 `json:"ids"`
	Enabled *bool   `json:"enabled" binding:"required"`
}

type supplierProbeBatchRequest struct {
	IDs []int64 `json:"ids"`
}

// List handles supplier list with pagination.
func (h *SupplierHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	suppliers, pag, err := h.supplierService.List(c.Request.Context(), pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    c.DefaultQuery("sort_by", "updated_at"),
		SortOrder: c.DefaultQuery("sort_order", "desc"),
	}, service.SupplierListFilter{
		Status:      c.Query("status"),
		ProbeStatus: c.Query("probe_status"),
		Search:      c.Query("search"),
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, suppliers, pag.Total, pag.Page, pag.PageSize)
}

// GetByID handles supplier detail.
func (h *SupplierHandler) GetByID(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}
	supplier, err := h.supplierService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, supplier)
}

// Create handles supplier creation.
func (h *SupplierHandler) Create(c *gin.Context) {
	var req createSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	supplier, err := h.supplierService.Create(c.Request.Context(), &service.CreateSupplierInput{
		Name:                 req.Name,
		WebsiteURL:           req.WebsiteURL,
		BaseURL:              req.BaseURL,
		APIKey:               req.APIKey,
		UpstreamGroup:        req.UpstreamGroup,
		ContactPlatform:      req.ContactPlatform,
		ContactValue:         req.ContactValue,
		Status:               req.Status,
		CostRMBPerUSD:        req.CostRMBPerUSD,
		Notes:                req.Notes,
		ProbeEnabled:         boolPtrValue(req.ProbeEnabled, true),
		ProbeModel:           req.ProbeModel,
		ProbeIntervalMinutes: req.ProbeIntervalMinutes,
		TargetGroupIDs:       req.TargetGroupIDs,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, supplier)
}

// Update handles supplier updates.
func (h *SupplierHandler) Update(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}

	var req updateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}

	input := &service.UpdateSupplierInput{
		Name:                 req.Name,
		WebsiteURL:           req.WebsiteURL,
		BaseURL:              req.BaseURL,
		APIKey:               req.APIKey,
		UpstreamGroup:        req.UpstreamGroup,
		ContactPlatform:      req.ContactPlatform,
		ContactValue:         req.ContactValue,
		Status:               req.Status,
		CostRMBPerUSD:        req.CostRMBPerUSD,
		Notes:                req.Notes,
		ProbeEnabled:         req.ProbeEnabled,
		ProbeModel:           req.ProbeModel,
		ProbeIntervalMinutes: req.ProbeIntervalMinutes,
		TargetGroupIDs:       req.TargetGroupIDs,
	}

	supplier, err := h.supplierService.Update(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, supplier)
}

// ProbeSnapshot returns supplier probe monitoring aggregation.
func (h *SupplierHandler) ProbeSnapshot(c *gin.Context) {
	windowMinutes := parsePositiveQueryInt(c.Query("window_minutes"), 60)
	days := parsePositiveQueryInt(c.Query("days"), 7)
	snapshot, err := h.supplierService.GetProbeSnapshot(c.Request.Context(), windowMinutes, days)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, snapshot)
}

// BulkSetProbeEnabled enables or disables supplier probes in batch.
func (h *SupplierHandler) BulkSetProbeEnabled(c *gin.Context) {
	var req bulkSupplierProbeEnabledRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	if req.Enabled == nil {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", "enabled is required"))
		return
	}
	affected, err := h.supplierService.BulkSetProbeEnabled(c.Request.Context(), req.IDs, *req.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"affected": affected})
}

// SyncAccounts imports active API key accounts into supplier probe rows.
func (h *SupplierHandler) SyncAccounts(c *gin.Context) {
	result, err := h.supplierService.SyncAccountsToSuppliers(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// ProbeAll runs an immediate probe for selected or all enabled suppliers.
func (h *SupplierHandler) ProbeAll(c *gin.Context) {
	var req supplierProbeBatchRequest
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		response.ErrorFrom(c, infraerrors.BadRequest("VALIDATION_ERROR", err.Error()))
		return
	}
	result, err := h.supplierService.RunEnabledProbes(c.Request.Context(), req.IDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

// Probe runs one immediate supplier probe.
func (h *SupplierHandler) Probe(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}
	supplier, result, err := h.supplierService.RunProbe(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{
		"supplier": supplier,
		"result":   result,
	})
}

// ProbeHistory returns recent probe history for one supplier.
func (h *SupplierHandler) ProbeHistory(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}
	days := parsePositiveQueryInt(c.Query("days"), 7)
	limit := parsePositiveQueryInt(c.Query("limit"), 200)
	history, err := h.supplierService.GetProbeHistory(c.Request.Context(), id, days, limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, history)
}

// Delete handles supplier soft deletion.
func (h *SupplierHandler) Delete(c *gin.Context) {
	id, ok := parseSupplierID(c)
	if !ok {
		return
	}
	if err := h.supplierService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Supplier deleted successfully"})
}

func parseSupplierID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.ErrorFrom(c, infraerrors.BadRequest("INVALID_SUPPLIER_ID", "Invalid supplier ID"))
		return 0, false
	}
	return id, true
}

func boolPtrValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func parsePositiveQueryInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

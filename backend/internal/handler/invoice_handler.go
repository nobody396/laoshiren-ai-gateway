package handler

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/response"
	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

type InvoiceHandler struct {
	invoiceService *service.InvoiceService
}

func NewInvoiceHandler(invoiceService *service.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{invoiceService: invoiceService}
}

type invoiceProfileRequest struct {
	Title       string  `json:"title" binding:"required"`
	TaxNumber   string  `json:"tax_number" binding:"required"`
	Email       string  `json:"email" binding:"required,email"`
	Address     *string `json:"address"`
	Phone       *string `json:"phone"`
	BankName    *string `json:"bank_name"`
	BankAccount *string `json:"bank_account"`
}

type createInvoiceRequestPayload struct {
	ProfileID int64   `json:"profile_id" binding:"required,gt=0"`
	OrderIDs  []int64 `json:"order_ids" binding:"required,min=1"`
	Remark    *string `json:"remark"`
}

func (h *InvoiceHandler) ListUserTopupOrders(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	startTime, endTime, err := ParseFlexibleTimeRange(c.Query("start_time"), c.Query("end_time"))
	if err != nil {
		response.BadRequest(c, "Invalid time range, use RFC3339 or YYYY-MM-DD")
		return
	}

	result, err := h.invoiceService.ListUserTopupOrders(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, service.InvoiceTopupOrderListFilters{
		Status:        strings.TrimSpace(c.Query("status")),
		InvoiceStatus: strings.TrimSpace(c.Query("invoice_status")),
		StartTime:     startTime,
		EndTime:       endTime,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *InvoiceHandler) ListProfiles(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	items, err := h.invoiceService.ListProfiles(c.Request.Context(), subject.UserID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *InvoiceHandler) CreateProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req invoiceProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	created, err := h.invoiceService.CreateProfile(c.Request.Context(), subject.UserID, service.InvoiceProfileInput(req))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, created)
}

func (h *InvoiceHandler) UpdateProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	profileID, err := parseInt64Param(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid invoice profile ID")
		return
	}

	var req invoiceProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	updated, err := h.invoiceService.UpdateProfile(c.Request.Context(), subject.UserID, profileID, service.InvoiceProfileInput(req))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *InvoiceHandler) DeleteProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	profileID, err := parseInt64Param(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid invoice profile ID")
		return
	}

	if err := h.invoiceService.DeleteProfile(c.Request.Context(), subject.UserID, profileID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "invoice profile deleted"})
}

func (h *InvoiceHandler) SetDefaultProfile(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	profileID, err := parseInt64Param(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "Invalid invoice profile ID")
		return
	}

	updated, err := h.invoiceService.SetDefaultProfile(c.Request.Context(), subject.UserID, profileID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *InvoiceHandler) CreateRequest(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req createInvoiceRequestPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	executeUserIdempotentJSON(c, "user.invoice.requests.create", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.invoiceService.CreateRequest(ctx, subject.UserID, service.CreateInvoiceRequestInput{
			ProfileID: req.ProfileID,
			OrderIDs:  req.OrderIDs,
			Remark:    req.Remark,
		})
	})
}

func (h *InvoiceHandler) ListRequests(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	page, pageSize := response.ParsePagination(c)
	startTime, endTime, err := ParseFlexibleTimeRange(c.Query("start_time"), c.Query("end_time"))
	if err != nil {
		response.BadRequest(c, "Invalid time range, use RFC3339 or YYYY-MM-DD")
		return
	}

	result, err := h.invoiceService.ListUserRequests(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, service.InvoiceRequestListFilters{
		Status:    strings.TrimSpace(c.Query("status")),
		Search:    strings.TrimSpace(c.Query("search")),
		StartTime: startTime,
		EndTime:   endTime,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func ParseFlexibleTimeRange(startRaw, endRaw string) (*time.Time, *time.Time, error) {
	start, err := parseFlexibleTime(startRaw, false)
	if err != nil {
		return nil, nil, err
	}
	end, err := parseFlexibleTime(endRaw, true)
	if err != nil {
		return nil, nil, err
	}
	return start, end, nil
}

func parseFlexibleTime(raw string, endOfDay bool) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}

	layouts := []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	for _, layout := range layouts {
		if ts, err := time.Parse(layout, value); err == nil {
			if layout == "2006-01-02" && endOfDay {
				ts = ts.Add(24 * time.Hour)
			}
			return &ts, nil
		}
	}
	return nil, strconv.ErrSyntax
}

func parseInt64Param(raw string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
}

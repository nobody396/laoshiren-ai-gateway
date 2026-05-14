package admin

import (
	"context"
	"errors"
	"io"
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

type rejectInvoiceRequestPayload struct {
	RejectReason string `json:"reject_reason" binding:"required"`
}

type exportInvoiceRequestsPayload struct {
	Status          string `json:"status"`
	Search          string `json:"search"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time"`
	IncludeExported bool   `json:"include_exported"`
}

func (h *InvoiceHandler) ListTopupOrders(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	startTime, endTime, err := parseFlexibleTimeRange(c.Query("start_time"), c.Query("end_time"))
	if err != nil {
		response.BadRequest(c, "Invalid time range")
		return
	}

	result, err := h.invoiceService.ListAdminTopupOrders(c.Request.Context(), pagination.PaginationParams{
		Page:     page,
		PageSize: pageSize,
	}, service.InvoiceTopupOrderListFilters{
		Status:        strings.TrimSpace(c.Query("status")),
		InvoiceStatus: strings.TrimSpace(c.Query("invoice_status")),
		Search:        strings.TrimSpace(c.Query("search")),
		StartTime:     startTime,
		EndTime:       endTime,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, result)
}

func (h *InvoiceHandler) ListRequests(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	startTime, endTime, err := parseFlexibleTimeRange(c.Query("start_time"), c.Query("end_time"))
	if err != nil {
		response.BadRequest(c, "Invalid time range")
		return
	}

	result, err := h.invoiceService.ListAdminRequests(c.Request.Context(), pagination.PaginationParams{
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

func (h *InvoiceHandler) CompleteRequest(c *gin.Context) {
	requestID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || requestID <= 0 {
		response.BadRequest(c, "Invalid invoice request ID")
		return
	}

	executeAdminIdempotentJSON(c, "admin.invoice.requests.complete", gin.H{"id": requestID}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		subject, ok := middleware2.GetAuthSubjectFromContext(c)
		if !ok {
			return nil, service.ErrInvoiceRequestNotFound
		}
		if err := h.invoiceService.CompleteRequest(ctx, subject.UserID, requestID); err != nil {
			return nil, err
		}
		return gin.H{"message": "invoice request completed"}, nil
	})
}

func (h *InvoiceHandler) RejectRequest(c *gin.Context) {
	requestID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || requestID <= 0 {
		response.BadRequest(c, "Invalid invoice request ID")
		return
	}

	var req rejectInvoiceRequestPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	executeAdminIdempotentJSON(c, "admin.invoice.requests.reject", gin.H{"id": requestID, "reject_reason": req.RejectReason}, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		subject, ok := middleware2.GetAuthSubjectFromContext(c)
		if !ok {
			return nil, service.ErrInvoiceRequestNotFound
		}
		if err := h.invoiceService.RejectRequest(ctx, subject.UserID, requestID, service.RejectInvoiceRequestInput{RejectReason: req.RejectReason}); err != nil {
			return nil, err
		}
		return gin.H{"message": "invoice request rejected"}, nil
	})
}

func (h *InvoiceHandler) ExportRequests(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not authenticated")
		return
	}

	var req exportInvoiceRequestsPayload
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	startTime, endTime, err := parseFlexibleTimeRange(req.StartTime, req.EndTime)
	if err != nil {
		response.BadRequest(c, "Invalid time range")
		return
	}

	result, err := h.invoiceService.ExportRequests(c.Request.Context(), subject.UserID, service.ExportInvoiceRequestsInput{
		Status:          strings.TrimSpace(req.Status),
		Search:          strings.TrimSpace(req.Search),
		StartTime:       startTime,
		EndTime:         endTime,
		IncludeExported: req.IncludeExported,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", "attachment; filename="+result.FileName)
	c.Data(200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", result.Content)
}

func parseFlexibleTimeRange(startRaw, endRaw string) (*time.Time, *time.Time, error) {
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

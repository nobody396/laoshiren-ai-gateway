package admin

import (
	"io"
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

// FinanceTransactionHandler handles admin manual bookkeeping (real cash in/out), independent
// of the theoretical cost-accounting margin calculator.
type FinanceTransactionHandler struct {
	financeService *service.FinanceTransactionService
	receiptStorage *service.FinanceReceiptS3Storage
}

func NewFinanceTransactionHandler(
	financeService *service.FinanceTransactionService,
	receiptStorage *service.FinanceReceiptS3Storage,
) *FinanceTransactionHandler {
	return &FinanceTransactionHandler{
		financeService: financeService,
		receiptStorage: receiptStorage,
	}
}

type CreateFinanceTransactionRequest struct {
	Type           string `json:"type" binding:"required,oneof=income expense"`
	Category       string `json:"category" binding:"required"`
	AmountFen      int64  `json:"amount_fen" binding:"required,gt=0"`
	OccurredAt     int64  `json:"occurred_at"` // Unix seconds, 0 = now
	Note           string `json:"note"`
	ReceiptKey     string `json:"receipt_key"`
	PaymentChannel string `json:"payment_channel"`
	Source         string `json:"source" binding:"omitempty,oneof=manual skill"`
}

type UpdateFinanceTransactionRequest struct {
	Type           *string `json:"type" binding:"omitempty,oneof=income expense"`
	Category       *string `json:"category"`
	AmountFen      *int64  `json:"amount_fen" binding:"omitempty,gt=0"`
	OccurredAt     *int64  `json:"occurred_at"` // Unix seconds
	Note           *string `json:"note"`
	ReceiptKey     *string `json:"receipt_key"`
	PaymentChannel *string `json:"payment_channel"`
}

// List handles listing finance transactions with filters
// GET /api/v1/admin/finance-transactions
func (h *FinanceTransactionHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	txType := strings.TrimSpace(c.Query("type"))
	category := strings.TrimSpace(c.Query("category"))
	search := strings.TrimSpace(c.Query("search"))
	sortBy := strings.TrimSpace(c.DefaultQuery("sort_by", "occurred_at"))
	sortOrder := strings.TrimSpace(c.DefaultQuery("sort_order", "desc"))
	if len(search) > 200 {
		search = search[:200]
	}

	filters := service.FinanceTransactionListFilters{
		Type:     txType,
		Category: category,
		Search:   search,
	}
	if from, ok := parseUnixSecondsQuery(c, "from"); ok {
		filters.From = &from
	}
	if to, ok := parseUnixSecondsQuery(c, "to"); ok {
		filters.To = &to
	}

	params := pagination.PaginationParams{
		Page:      page,
		PageSize:  pageSize,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}

	items, paginationResult, err := h.financeService.List(c.Request.Context(), params, filters)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.FinanceTransaction, 0, len(items))
	for i := range items {
		out = append(out, *dto.FinanceTransactionFromService(&items[i]))
	}
	response.Paginated(c, out, paginationResult.Total, page, pageSize)
}

// Summary handles aggregating income/expense/profit for a time range (defaults to the current month).
// GET /api/v1/admin/finance-transactions/summary
func (h *FinanceTransactionHandler) Summary(c *gin.Context) {
	scope := strings.TrimSpace(c.Query("scope"))
	if scope == "all" {
		summary, err := h.financeService.SummaryAll(c.Request.Context(), time.Now())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, dto.FinanceTransactionSummaryFromService(summary))
		return
	}
	if scope != "" && scope != "month" {
		response.BadRequest(c, "Invalid finance summary scope")
		return
	}

	from, hasFrom := parseUnixSecondsQuery(c, "from")
	to, hasTo := parseUnixSecondsQuery(c, "to")
	if !hasFrom || !hasTo {
		now := time.Now()
		monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		if !hasFrom {
			from = monthStart
		}
		if !hasTo {
			to = monthStart.AddDate(0, 1, 0)
		}
	}

	summary, err := h.financeService.Summary(c.Request.Context(), from, to)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.FinanceTransactionSummaryFromService(summary))
}

// GetByID handles getting a finance transaction by ID
// GET /api/v1/admin/finance-transactions/:id
func (h *FinanceTransactionHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid finance transaction ID")
		return
	}

	item, err := h.financeService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.FinanceTransactionFromService(item))
}

// Create handles creating a finance transaction
// POST /api/v1/admin/finance-transactions
func (h *FinanceTransactionHandler) Create(c *gin.Context) {
	var req CreateFinanceTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	input := &service.CreateFinanceTransactionInput{
		Type:           req.Type,
		Category:       req.Category,
		AmountFen:      req.AmountFen,
		Note:           nilIfEmpty(req.Note),
		PaymentChannel: nilIfEmpty(req.PaymentChannel),
		Source:         req.Source,
		ActorID:        &subject.UserID,
	}
	if req.ReceiptKey != "" {
		key := req.ReceiptKey
		input.ReceiptKey = &key
	}
	if req.OccurredAt > 0 {
		input.OccurredAt = time.Unix(req.OccurredAt, 0)
	}

	created, err := h.financeService.Create(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.FinanceTransactionFromService(created))
}

// Update handles updating a finance transaction
// PUT /api/v1/admin/finance-transactions/:id
func (h *FinanceTransactionHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid finance transaction ID")
		return
	}

	var req UpdateFinanceTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	input := &service.UpdateFinanceTransactionInput{
		Type:      req.Type,
		Category:  req.Category,
		AmountFen: req.AmountFen,
	}
	if req.OccurredAt != nil {
		t := time.Unix(*req.OccurredAt, 0)
		input.OccurredAt = &t
	}
	if req.Note != nil {
		input.Note = &req.Note
	}
	if req.ReceiptKey != nil {
		input.ReceiptKey = &req.ReceiptKey
	}
	if req.PaymentChannel != nil {
		input.PaymentChannel = &req.PaymentChannel
	}

	updated, err := h.financeService.Update(c.Request.Context(), id, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.FinanceTransactionFromService(updated))
}

// Delete handles deleting a finance transaction
// DELETE /api/v1/admin/finance-transactions/:id
func (h *FinanceTransactionHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid finance transaction ID")
		return
	}

	if err := h.financeService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Finance transaction deleted successfully"})
}

// UploadReceipt handles uploading a (already client-compressed) receipt image and returns its S3 key.
// POST /api/v1/admin/finance-transactions/receipts
func (h *FinanceTransactionHandler) UploadReceipt(c *gin.Context) {
	if !h.receiptStorage.Enabled() {
		response.BadRequest(c, "Finance receipt storage is not configured")
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		response.BadRequest(c, "Missing file")
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		response.BadRequest(c, "Cannot open uploaded file")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(io.LimitReader(file, 8*1024*1024+1))
	if err != nil {
		response.BadRequest(c, "Cannot read uploaded file")
		return
	}

	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}

	stored, err := h.receiptStorage.Upload(c.Request.Context(), subject.UserID, data, fileHeader.Header.Get("Content-Type"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"key":          stored.Key,
		"size_bytes":   stored.SizeBytes,
		"content_type": stored.ContentType,
	})
}

// GetReceiptURL returns a short-lived presigned URL to view a transaction's receipt image.
// GET /api/v1/admin/finance-transactions/:id/receipt-url
func (h *FinanceTransactionHandler) GetReceiptURL(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid finance transaction ID")
		return
	}

	item, err := h.financeService.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	if item.ReceiptKey == nil || *item.ReceiptKey == "" {
		response.BadRequest(c, "Finance transaction has no receipt")
		return
	}

	url, err := h.receiptStorage.PresignGetURL(c.Request.Context(), *item.ReceiptKey, 10*time.Minute)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"url": url, "expires_in_seconds": 600})
}

func parseUnixSecondsQuery(c *gin.Context, key string) (time.Time, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return time.Time{}, false
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || seconds <= 0 {
		return time.Time{}, false
	}
	return time.Unix(seconds, 0), true
}

func nilIfEmpty(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

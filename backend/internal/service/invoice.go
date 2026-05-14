package service

import (
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const (
	TopupInvoiceStatusNone     = "none"
	TopupInvoiceStatusApplied  = "applied"
	TopupInvoiceStatusInvoiced = "invoiced"

	InvoiceRequestStatusPending   = "pending"
	InvoiceRequestStatusExported  = "exported"
	InvoiceRequestStatusCompleted = "completed"
	InvoiceRequestStatusRejected  = "rejected"
)

// InvoiceRequestMinimumAmountFen is the minimum total selected order amount required for one invoice request.
const InvoiceRequestMinimumAmountFen int64 = 30000

const (
	InvoiceProfileTitleMaxRunes       = 200
	InvoiceProfileTaxNumberMaxRunes   = 20
	InvoiceProfileEmailMaxRunes       = 100
	InvoiceProfileAddressMaxRunes     = 500
	InvoiceProfilePhoneMaxRunes       = 40
	InvoiceProfileBankNameMaxRunes    = 200
	InvoiceProfileBankAccountMaxRunes = 100
)

var (
	ErrInvoiceProfileNotFound      = infraerrors.NotFound("INVOICE_PROFILE_NOT_FOUND", "invoice profile not found")
	ErrInvoiceProfileInUse         = infraerrors.Conflict("INVOICE_PROFILE_IN_USE", "invoice profile is referenced by in-progress requests")
	ErrInvoiceRequestNotFound      = infraerrors.NotFound("INVOICE_REQUEST_NOT_FOUND", "invoice request not found")
	ErrInvoiceRequestStatusInvalid = infraerrors.Conflict("INVOICE_REQUEST_STATUS_INVALID", "invoice request status is invalid for this operation")
	ErrInvoiceOrderNotFound        = infraerrors.NotFound("INVOICE_ORDER_NOT_FOUND", "topup order not found")
	ErrInvoiceOrderOwnership       = infraerrors.Forbidden("INVOICE_ORDER_OWNERSHIP_INVALID", "topup order does not belong to current user")
	ErrInvoiceOrderStatusInvalid   = infraerrors.BadRequest("INVOICE_ORDER_STATUS_INVALID", "topup order is not eligible for invoicing")
	ErrInvoiceOrderAlreadyApplied  = infraerrors.Conflict("INVOICE_ORDER_ALREADY_APPLIED", "topup order has already been included in an invoice request")
	ErrInvoiceEmptyOrderSelection  = infraerrors.BadRequest("INVOICE_EMPTY_ORDER_SELECTION", "at least one topup order must be selected")
	ErrInvoiceAmountBelowMinimum   = infraerrors.BadRequest("INVOICE_AMOUNT_BELOW_MINIMUM", "invoice request total amount must be at least CNY 300")
	ErrInvoiceExportEmpty          = infraerrors.BadRequest("INVOICE_EXPORT_EMPTY", "no invoice requests available for export")
)

type InvoiceProfileSnapshot = domain.InvoiceProfileSnapshot

type InvoiceProfile struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	Title       string    `json:"title"`
	TaxNumber   string    `json:"tax_number"`
	Email       string    `json:"email"`
	Address     *string   `json:"address,omitempty"`
	Phone       *string   `json:"phone,omitempty"`
	BankName    *string   `json:"bank_name,omitempty"`
	BankAccount *string   `json:"bank_account,omitempty"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type InvoiceProfileInput struct {
	Title       string  `json:"title"`
	TaxNumber   string  `json:"tax_number"`
	Email       string  `json:"email"`
	Address     *string `json:"address,omitempty"`
	Phone       *string `json:"phone,omitempty"`
	BankName    *string `json:"bank_name,omitempty"`
	BankAccount *string `json:"bank_account,omitempty"`
}

type InvoiceTopupOrder struct {
	ID            int64      `json:"id"`
	OrderNo       string     `json:"order_no"`
	UserID        int64      `json:"user_id"`
	UserEmail     string     `json:"user_email,omitempty"`
	UserName      string     `json:"user_name,omitempty"`
	AmountCNYFen  int        `json:"amount_cny_fen"`
	PayType       string     `json:"pay_type"`
	Status        string     `json:"status"`
	InvoiceStatus string     `json:"invoice_status"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type InvoiceTopupOrderListFilters struct {
	Status        string
	InvoiceStatus string
	Search        string
	StartTime     *time.Time
	EndTime       *time.Time
}

type InvoiceTopupOrderListResult struct {
	Items               []InvoiceTopupOrder `json:"items"`
	Total               int64               `json:"total"`
	Page                int                 `json:"page"`
	PageSize            int                 `json:"page_size"`
	Pages               int                 `json:"pages"`
	SelectableAmountFen int64               `json:"selectable_amount_fen,omitempty"`
	PendingRequestCount int64               `json:"pending_request_count,omitempty"`
}

type InvoiceRequestOrder struct {
	ID           int64             `json:"id"`
	TopupOrderID int64             `json:"topup_order_id"`
	TopupOrder   InvoiceTopupOrder `json:"topup_order"`
}

type InvoiceRequest struct {
	ID              int64                  `json:"id"`
	SerialNo        string                 `json:"serial_no"`
	UserID          int64                  `json:"user_id"`
	UserEmail       string                 `json:"user_email,omitempty"`
	UserName        string                 `json:"user_name,omitempty"`
	ProfileID       *int64                 `json:"profile_id,omitempty"`
	ProfileSnapshot InvoiceProfileSnapshot `json:"profile_snapshot"`
	TotalAmountFen  int64                  `json:"total_amount_fen"`
	Status          string                 `json:"status"`
	ExportBatchNo   *string                `json:"export_batch_no,omitempty"`
	ExportedAt      *time.Time             `json:"exported_at,omitempty"`
	ExportedBy      *int64                 `json:"exported_by,omitempty"`
	Remark          *string                `json:"remark,omitempty"`
	RejectReason    *string                `json:"reject_reason,omitempty"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	CompletedBy     *int64                 `json:"completed_by,omitempty"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	Orders          []InvoiceRequestOrder  `json:"orders"`
}

type InvoiceRequestListFilters struct {
	Status          string
	Search          string
	StartTime       *time.Time
	EndTime         *time.Time
	IncludeExported bool
}

type InvoiceRequestListResult struct {
	Items        []InvoiceRequest `json:"items"`
	Total        int64            `json:"total"`
	Page         int              `json:"page"`
	PageSize     int              `json:"page_size"`
	Pages        int              `json:"pages"`
	PendingCount int64            `json:"pending_count,omitempty"`
}

type CreateInvoiceRequestInput struct {
	ProfileID int64   `json:"profile_id"`
	OrderIDs  []int64 `json:"order_ids"`
	Remark    *string `json:"remark,omitempty"`
}

type CreateInvoiceRequestResult struct {
	InvoiceRequestID int64  `json:"invoice_request_id"`
	SerialNo         string `json:"serial_no"`
	TotalAmountFen   int64  `json:"total_amount_fen"`
	Status           string `json:"status"`
}

type RejectInvoiceRequestInput struct {
	RejectReason string `json:"reject_reason"`
}

type ExportInvoiceRequestsInput struct {
	Status          string     `json:"status,omitempty"`
	Search          string     `json:"search,omitempty"`
	StartTime       *time.Time `json:"start_time,omitempty"`
	EndTime         *time.Time `json:"end_time,omitempty"`
	IncludeExported bool       `json:"include_exported,omitempty"`
}

type ExportInvoiceRequestsResult struct {
	FileName string `json:"file_name"`
	Content  []byte `json:"-"`
	BatchNo  string `json:"batch_no"`
	Exported int    `json:"exported"`
}

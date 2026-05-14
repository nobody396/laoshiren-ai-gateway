package service

import (
	"context"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

// InvoiceRepository defines the persistence interface for invoice-related data access.
type InvoiceRepository interface {
	// --- TopupOrder queries ---

	// ListUserTopupOrders returns a paginated list of topup orders for a specific user.
	ListUserTopupOrders(ctx context.Context, userID int64, params pagination.PaginationParams, filters InvoiceTopupOrderListFilters) (items []InvoiceTopupOrder, total int, err error)

	// ListAdminTopupOrders returns a paginated list of all topup orders (admin view, includes user info).
	ListAdminTopupOrders(ctx context.Context, params pagination.PaginationParams, filters InvoiceTopupOrderListFilters) (items []InvoiceTopupOrder, total int, err error)

	// SumSelectableAmountFen returns the total invoiceable amount (in fen) for a user's eligible orders.
	SumSelectableAmountFen(ctx context.Context, userID int64, filters InvoiceTopupOrderListFilters) (int64, error)

	// GetTopupOrdersByIDs returns topup orders matching the given IDs.
	GetTopupOrdersByIDs(ctx context.Context, orderIDs []int64) ([]InvoiceTopupOrder, error)

	// ClaimTopupOrdersForInvoicing atomically marks orders as invoice-applied.
	// Returns the count of rows actually updated.
	ClaimTopupOrdersForInvoicing(ctx context.Context, userID int64, orderIDs []int64) (int, error)

	// ReleaseTopupOrdersFromInvoicing resets invoice status back to none for orders that were applied.
	ReleaseTopupOrdersFromInvoicing(ctx context.Context, orderIDs []int64) (int, error)

	// MarkTopupOrdersInvoiced marks topup orders as fully invoiced.
	MarkTopupOrdersInvoiced(ctx context.Context, orderIDs []int64) (int, error)

	// --- InvoiceProfile operations ---

	// ListProfiles returns all invoice profiles for a user ordered by default-first then newest.
	ListProfiles(ctx context.Context, userID int64) ([]InvoiceProfile, error)

	// CountProfiles returns the number of profiles belonging to the user.
	CountProfiles(ctx context.Context, userID int64) (int, error)

	// GetProfile returns a single profile belonging to the given user.
	GetProfile(ctx context.Context, userID, profileID int64) (*InvoiceProfile, error)

	// CreateProfile inserts a new invoice profile and returns the created entity.
	CreateProfile(ctx context.Context, userID int64, input InvoiceProfileInput, isDefault bool) (*InvoiceProfile, error)

	// UpdateProfile updates an existing profile and returns the updated entity.
	UpdateProfile(ctx context.Context, profileID int64, input InvoiceProfileInput) (*InvoiceProfile, error)

	// DeleteProfile removes a profile by ID.
	DeleteProfile(ctx context.Context, profileID int64) error

	// ClearDefaultProfiles sets is_default=false for all profiles of a user.
	ClearDefaultProfiles(ctx context.Context, userID int64) error

	// SetProfileDefault sets is_default=true for a specific profile.
	SetProfileDefault(ctx context.Context, profileID int64) (*InvoiceProfile, error)

	// FindLatestProfileForUser returns the most recently created profile for a user (used when promoting a fallback default).
	FindLatestProfileForUser(ctx context.Context, userID int64) (*InvoiceProfile, error)

	// ProfileInUse returns true when the profile is referenced by a pending or exported invoice request.
	ProfileInUse(ctx context.Context, profileID int64) (bool, error)

	// --- InvoiceRequest operations ---

	// ListUserRequests returns paginated invoice requests for a specific user.
	ListUserRequests(ctx context.Context, userID int64, params pagination.PaginationParams, filters InvoiceRequestListFilters) (items []InvoiceRequest, total int, err error)

	// ListAdminRequests returns paginated invoice requests (admin view, includes user info).
	ListAdminRequests(ctx context.Context, params pagination.PaginationParams, filters InvoiceRequestListFilters) (items []InvoiceRequest, total int, err error)

	// CountPendingRequests returns the number of requests in pending status.
	CountPendingRequests(ctx context.Context) (int, error)

	// CreateRequest inserts a new invoice request with a temporary serial number and returns the created ID and created-at.
	CreateRequest(ctx context.Context, input CreateInvoiceRequestRepoInput) (id int64, serialNo string, err error)

	// UpdateRequestSerialNo sets the final serial number on a newly created request.
	UpdateRequestSerialNo(ctx context.Context, requestID int64, serialNo string) error

	// CreateRequestOrderLinks bulk-inserts the invoice_request_order join rows.
	CreateRequestOrderLinks(ctx context.Context, requestID int64, orderIDs []int64) error

	// GetRequestWithOrders returns a single invoice request with its order links loaded.
	GetRequestWithOrders(ctx context.Context, requestID int64) (*InvoiceRequest, error)

	// UpdateRequestStatus updates the status (and related audit fields) of a request.
	UpdateRequestStatus(ctx context.Context, requestID int64, input UpdateRequestStatusInput) error

	// DeleteRequestOrderLinks removes all order link rows for a request.
	DeleteRequestOrderLinks(ctx context.Context, requestID int64) error

	// ListRequestsForExport returns all invoice requests matching the export filters (no pagination).
	ListRequestsForExport(ctx context.Context, filters InvoiceRequestListFilters) ([]InvoiceRequest, error)

	// MarkRequestsExported bulk-updates pending requests to exported status.
	// Returns the count of rows actually updated.
	MarkRequestsExported(ctx context.Context, requestIDs []int64, batchNo string, actorID int64) (int, error)
}

// CreateInvoiceRequestRepoInput holds the data needed to insert an invoice request row.
type CreateInvoiceRequestRepoInput struct {
	TempSerialNo        string
	UserID              int64
	ProfileID           int64
	ProfileSnapshotJSON InvoiceProfileSnapshot
	TotalAmountFen      int64
	Status              string
	Remark              *string
}

// UpdateRequestStatusInput carries the fields that may be set when transitioning a request status.
type UpdateRequestStatusInput struct {
	Status       string
	ActorID      int64
	RejectReason *string
}

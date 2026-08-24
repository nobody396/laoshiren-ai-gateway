package repository

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/invoiceprofile"
	"github.com/bozhouDev/DragonCode-sub2api/ent/invoicerequest"
	"github.com/bozhouDev/DragonCode-sub2api/ent/invoicerequestorder"
	"github.com/bozhouDev/DragonCode-sub2api/ent/predicate"
	"github.com/bozhouDev/DragonCode-sub2api/ent/topuporder"
	"github.com/bozhouDev/DragonCode-sub2api/ent/user"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type invoiceRepository struct {
	client *dbent.Client
}

// NewInvoiceRepository creates a new InvoiceRepository backed by ent.
func NewInvoiceRepository(client *dbent.Client) service.InvoiceRepository {
	return &invoiceRepository{client: client}
}

// ---------------------------------------------------------------------------
// TopupOrder
// ---------------------------------------------------------------------------

func (r *invoiceRepository) ListUserTopupOrders(ctx context.Context, userID int64, params pagination.PaginationParams, filters service.InvoiceTopupOrderListFilters) ([]service.InvoiceTopupOrder, int, error) {
	client := clientFromContext(ctx, r.client)
	if err := expireStalePendingTopupOrders(ctx, client); err != nil {
		return nil, 0, err
	}
	query := client.TopupOrder.Query().Where(topuporder.UserIDEQ(userID))
	query = applyTopupOrderFilters(query, filters, false)

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count user topup orders: %w", err)
	}

	entities, err := query.
		Order(dbent.Desc(topuporder.FieldCreatedAt), dbent.Desc(topuporder.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list user topup orders: %w", err)
	}
	return mapTopupOrders(entities), total, nil
}

func (r *invoiceRepository) ListAdminTopupOrders(ctx context.Context, params pagination.PaginationParams, filters service.InvoiceTopupOrderListFilters) ([]service.InvoiceTopupOrder, int, error) {
	client := clientFromContext(ctx, r.client)
	if err := expireStalePendingTopupOrders(ctx, client); err != nil {
		return nil, 0, err
	}
	query := client.TopupOrder.Query().WithUser()
	query = applyTopupOrderFilters(query, filters, true)

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count admin topup orders: %w", err)
	}

	entities, err := query.
		Order(dbent.Desc(topuporder.FieldCreatedAt), dbent.Desc(topuporder.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin topup orders: %w", err)
	}
	return mapTopupOrders(entities), total, nil
}

func expireStalePendingTopupOrders(ctx context.Context, client *dbent.Client) error {
	cutoff := time.Now().Add(-service.TopupOrderTTL)
	_, err := client.TopupOrder.Update().
		Where(
			topuporder.StatusEQ(service.TopupStatusPending),
			topuporder.CreatedAtLTE(cutoff),
		).
		SetStatus(service.TopupStatusExpired).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("expire stale pending topup orders: %w", err)
	}
	return nil
}

func (r *invoiceRepository) SumSelectableAmountFen(ctx context.Context, userID int64, filters service.InvoiceTopupOrderListFilters) (int64, error) {
	client := clientFromContext(ctx, r.client)
	query := client.TopupOrder.Query().
		Where(
			topuporder.UserIDEQ(userID),
			topuporder.StatusEQ(service.TopupStatusCompleted),
			topuporder.InvoiceStatusEQ(service.TopupInvoiceStatusNone),
		)
	if filters.StartTime != nil {
		query = query.Where(topuporder.CreatedAtGTE(*filters.StartTime))
	}
	if filters.EndTime != nil {
		query = query.Where(topuporder.CreatedAtLT(*filters.EndTime))
	}

	var rows []struct {
		Sum int64 `json:"sum"`
	}
	if err := query.Aggregate(dbent.As(dbent.Sum(topuporder.FieldAmountCnyFen), "sum")).Scan(ctx, &rows); err != nil {
		return 0, fmt.Errorf("sum selectable topup order amount: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].Sum, nil
}

func (r *invoiceRepository) GetTopupOrdersByIDs(ctx context.Context, orderIDs []int64) ([]service.InvoiceTopupOrder, error) {
	client := clientFromContext(ctx, r.client)
	entities, err := client.TopupOrder.Query().
		Where(topuporder.IDIn(orderIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query topup orders: %w", err)
	}
	return mapTopupOrders(entities), nil
}

func (r *invoiceRepository) ClaimTopupOrdersForInvoicing(ctx context.Context, userID int64, orderIDs []int64) (int, error) {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	n, err := client.TopupOrder.Update().
		Where(
			topuporder.IDIn(orderIDs...),
			topuporder.UserIDEQ(userID),
			topuporder.StatusEQ(service.TopupStatusCompleted),
			topuporder.InvoiceStatusEQ(service.TopupInvoiceStatusNone),
		).
		SetInvoiceStatus(service.TopupInvoiceStatusApplied).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("claim topup orders for invoicing: %w", err)
	}
	return n, nil
}

func (r *invoiceRepository) ReleaseTopupOrdersFromInvoicing(ctx context.Context, orderIDs []int64) (int, error) {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	n, err := client.TopupOrder.Update().
		Where(topuporder.IDIn(orderIDs...), topuporder.InvoiceStatusEQ(service.TopupInvoiceStatusApplied)).
		SetInvoiceStatus(service.TopupInvoiceStatusNone).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("reset topup order invoice status: %w", err)
	}
	return n, nil
}

func (r *invoiceRepository) MarkTopupOrdersInvoiced(ctx context.Context, orderIDs []int64) (int, error) {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	n, err := client.TopupOrder.Update().
		Where(topuporder.IDIn(orderIDs...), topuporder.InvoiceStatusEQ(service.TopupInvoiceStatusApplied)).
		SetInvoiceStatus(service.TopupInvoiceStatusInvoiced).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("mark topup orders invoiced: %w", err)
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// InvoiceProfile
// ---------------------------------------------------------------------------

func (r *invoiceRepository) ListProfiles(ctx context.Context, userID int64) ([]service.InvoiceProfile, error) {
	client := clientFromContext(ctx, r.client)
	entities, err := client.InvoiceProfile.Query().
		Where(invoiceprofile.UserIDEQ(userID)).
		Order(dbent.Desc(invoiceprofile.FieldIsDefault), dbent.Desc(invoiceprofile.FieldCreatedAt), dbent.Desc(invoiceprofile.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list invoice profiles: %w", err)
	}
	return mapInvoiceProfiles(entities), nil
}

func (r *invoiceRepository) CountProfiles(ctx context.Context, userID int64) (int, error) {
	client := clientFromContext(ctx, r.client)
	n, err := client.InvoiceProfile.Query().Where(invoiceprofile.UserIDEQ(userID)).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count invoice profiles: %w", err)
	}
	return n, nil
}

func (r *invoiceRepository) GetProfile(ctx context.Context, userID, profileID int64) (*service.InvoiceProfile, error) {
	client := clientFromContext(ctx, r.client)
	entity, err := client.InvoiceProfile.Query().
		Where(invoiceprofile.IDEQ(profileID), invoiceprofile.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrInvoiceProfileNotFound
		}
		return nil, fmt.Errorf("get invoice profile: %w", err)
	}
	out := mapInvoiceProfile(entity)
	return &out, nil
}

func (r *invoiceRepository) CreateProfile(ctx context.Context, userID int64, input service.InvoiceProfileInput, isDefault bool) (*service.InvoiceProfile, error) {
	client := clientFromContext(ctx, r.client)
	created, err := client.InvoiceProfile.Create().
		SetUserID(userID).
		SetTitle(strings.TrimSpace(input.Title)).
		SetTaxNumber(strings.TrimSpace(input.TaxNumber)).
		SetEmail(strings.TrimSpace(input.Email)).
		SetNillableAddress(trimNullableString(input.Address)).
		SetNillablePhone(trimNullableString(input.Phone)).
		SetNillableBankName(trimNullableString(input.BankName)).
		SetNillableBankAccount(trimNullableString(input.BankAccount)).
		SetIsDefault(isDefault).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create invoice profile: %w", err)
	}
	out := mapInvoiceProfile(created)
	return &out, nil
}

func (r *invoiceRepository) UpdateProfile(ctx context.Context, profileID int64, input service.InvoiceProfileInput) (*service.InvoiceProfile, error) {
	client := clientFromContext(ctx, r.client)
	updated, err := client.InvoiceProfile.UpdateOneID(profileID).
		SetTitle(strings.TrimSpace(input.Title)).
		SetTaxNumber(strings.TrimSpace(input.TaxNumber)).
		SetEmail(strings.TrimSpace(input.Email)).
		SetNillableAddress(trimNullableString(input.Address)).
		SetNillablePhone(trimNullableString(input.Phone)).
		SetNillableBankName(trimNullableString(input.BankName)).
		SetNillableBankAccount(trimNullableString(input.BankAccount)).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("update invoice profile: %w", err)
	}
	out := mapInvoiceProfile(updated)
	return &out, nil
}

func (r *invoiceRepository) DeleteProfile(ctx context.Context, profileID int64) error {
	client := clientFromContext(ctx, r.client)
	if err := client.InvoiceProfile.DeleteOneID(profileID).Exec(ctx); err != nil {
		return fmt.Errorf("delete invoice profile: %w", err)
	}
	return nil
}

func (r *invoiceRepository) ClearDefaultProfiles(ctx context.Context, userID int64) error {
	client := clientFromContext(ctx, r.client)
	if _, err := client.InvoiceProfile.Update().
		Where(invoiceprofile.UserIDEQ(userID), invoiceprofile.IsDefaultEQ(true)).
		SetIsDefault(false).
		Save(ctx); err != nil {
		return fmt.Errorf("clear default invoice profiles: %w", err)
	}
	return nil
}

func (r *invoiceRepository) SetProfileDefault(ctx context.Context, profileID int64) (*service.InvoiceProfile, error) {
	client := clientFromContext(ctx, r.client)
	updated, err := client.InvoiceProfile.UpdateOneID(profileID).SetIsDefault(true).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("set default invoice profile: %w", err)
	}
	out := mapInvoiceProfile(updated)
	return &out, nil
}

func (r *invoiceRepository) FindLatestProfileForUser(ctx context.Context, userID int64) (*service.InvoiceProfile, error) {
	client := clientFromContext(ctx, r.client)
	entity, err := client.InvoiceProfile.Query().
		Where(invoiceprofile.UserIDEQ(userID)).
		Order(dbent.Desc(invoiceprofile.FieldCreatedAt), dbent.Desc(invoiceprofile.FieldID)).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("find latest invoice profile: %w", err)
	}
	out := mapInvoiceProfile(entity)
	return &out, nil
}

func (r *invoiceRepository) ProfileInUse(ctx context.Context, profileID int64) (bool, error) {
	client := clientFromContext(ctx, r.client)
	inUse, err := client.InvoiceRequest.Query().
		Where(
			invoicerequest.ProfileIDEQ(profileID),
			invoicerequest.StatusIn(service.InvoiceRequestStatusPending, service.InvoiceRequestStatusExported),
		).
		Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("check invoice profile in use: %w", err)
	}
	return inUse, nil
}

// ---------------------------------------------------------------------------
// InvoiceRequest
// ---------------------------------------------------------------------------

func (r *invoiceRepository) ListUserRequests(ctx context.Context, userID int64, params pagination.PaginationParams, filters service.InvoiceRequestListFilters) ([]service.InvoiceRequest, int, error) {
	client := clientFromContext(ctx, r.client)
	query := client.InvoiceRequest.Query().
		Where(invoicerequest.UserIDEQ(userID)).
		WithRequestOrders(func(q *dbent.InvoiceRequestOrderQuery) {
			q.WithTopupOrder()
		})
	query = applyInvoiceRequestFilters(query, filters, false)

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count user invoice requests: %w", err)
	}

	entities, err := query.
		Order(dbent.Desc(invoicerequest.FieldCreatedAt), dbent.Desc(invoicerequest.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list user invoice requests: %w", err)
	}
	return mapInvoiceRequests(entities), total, nil
}

func (r *invoiceRepository) ListAdminRequests(ctx context.Context, params pagination.PaginationParams, filters service.InvoiceRequestListFilters) ([]service.InvoiceRequest, int, error) {
	client := clientFromContext(ctx, r.client)
	query := client.InvoiceRequest.Query().
		WithUser().
		WithRequestOrders(func(q *dbent.InvoiceRequestOrderQuery) {
			q.WithTopupOrder()
		})
	query = applyInvoiceRequestFilters(query, filters, true)

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count admin invoice requests: %w", err)
	}

	entities, err := query.
		Order(dbent.Desc(invoicerequest.FieldCreatedAt), dbent.Desc(invoicerequest.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin invoice requests: %w", err)
	}
	return mapInvoiceRequests(entities), total, nil
}

func (r *invoiceRepository) CountPendingRequests(ctx context.Context) (int, error) {
	client := clientFromContext(ctx, r.client)
	n, err := client.InvoiceRequest.Query().
		Where(invoicerequest.StatusEQ(service.InvoiceRequestStatusPending)).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count pending invoice requests: %w", err)
	}
	return n, nil
}

func (r *invoiceRepository) CreateRequest(ctx context.Context, input service.CreateInvoiceRequestRepoInput) (int64, string, error) {
	client := clientFromContext(ctx, r.client)
	created, err := client.InvoiceRequest.Create().
		SetSerialNo(input.TempSerialNo).
		SetUserID(input.UserID).
		SetProfileID(input.ProfileID).
		SetProfileSnapshotJSON(input.ProfileSnapshotJSON).
		SetTotalAmountFen(input.TotalAmountFen).
		SetStatus(input.Status).
		SetNillableRemark(input.Remark).
		Save(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("create invoice request: %w", err)
	}
	return created.ID, created.SerialNo, nil
}

func (r *invoiceRepository) UpdateRequestSerialNo(ctx context.Context, requestID int64, serialNo string) error {
	client := clientFromContext(ctx, r.client)
	if err := client.InvoiceRequest.UpdateOneID(requestID).SetSerialNo(serialNo).Exec(ctx); err != nil {
		return fmt.Errorf("assign invoice serial number: %w", err)
	}
	return nil
}

func (r *invoiceRepository) CreateRequestOrderLinks(ctx context.Context, requestID int64, orderIDs []int64) error {
	client := clientFromContext(ctx, r.client)
	builders := make([]*dbent.InvoiceRequestOrderCreate, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		builders = append(builders, client.InvoiceRequestOrder.Create().
			SetInvoiceRequestID(requestID).
			SetTopupOrderID(orderID))
	}
	if len(builders) > 0 {
		if _, err := client.InvoiceRequestOrder.CreateBulk(builders...).Save(ctx); err != nil {
			return fmt.Errorf("create invoice request orders: %w", err)
		}
	}
	return nil
}

func (r *invoiceRepository) GetRequestWithOrders(ctx context.Context, requestID int64) (*service.InvoiceRequest, error) {
	client := clientFromContext(ctx, r.client)
	entity, err := client.InvoiceRequest.Query().
		Where(invoicerequest.IDEQ(requestID)).
		WithRequestOrders().
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrInvoiceRequestNotFound
		}
		return nil, fmt.Errorf("get invoice request: %w", err)
	}
	out := mapInvoiceRequest(entity)
	return &out, nil
}

func (r *invoiceRepository) UpdateRequestStatus(ctx context.Context, requestID int64, input service.UpdateRequestStatusInput) error {
	client := clientFromContext(ctx, r.client)
	update := client.InvoiceRequest.UpdateOneID(requestID).SetStatus(input.Status)
	now := time.Now()
	switch input.Status {
	case service.InvoiceRequestStatusCompleted:
		update.SetCompletedAt(now).SetCompletedBy(input.ActorID)
	case service.InvoiceRequestStatusRejected:
		update.SetRejectReason(strings.TrimSpace(*input.RejectReason))
	}
	if err := update.Exec(ctx); err != nil {
		return fmt.Errorf("update invoice request status: %w", err)
	}
	return nil
}

func (r *invoiceRepository) DeleteRequestOrderLinks(ctx context.Context, requestID int64) error {
	client := clientFromContext(ctx, r.client)
	if _, err := client.InvoiceRequestOrder.Delete().
		Where(invoicerequestorder.InvoiceRequestIDEQ(requestID)).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete invoice request orders: %w", err)
	}
	return nil
}

func (r *invoiceRepository) ListRequestsForExport(ctx context.Context, filters service.InvoiceRequestListFilters) ([]service.InvoiceRequest, error) {
	client := clientFromContext(ctx, r.client)
	query := client.InvoiceRequest.Query().
		WithRequestOrders(func(q *dbent.InvoiceRequestOrderQuery) {
			q.WithTopupOrder()
		})
	query = applyExportFilters(query, filters)

	entities, err := query.
		Order(dbent.Desc(invoicerequest.FieldCreatedAt), dbent.Desc(invoicerequest.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query invoice requests for export: %w", err)
	}
	return mapInvoiceRequests(entities), nil
}

func (r *invoiceRepository) MarkRequestsExported(ctx context.Context, requestIDs []int64, batchNo string, actorID int64) (int, error) {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	n, err := client.InvoiceRequest.Update().
		Where(invoicerequest.IDIn(requestIDs...), invoicerequest.StatusEQ(service.InvoiceRequestStatusPending)).
		SetStatus(service.InvoiceRequestStatusExported).
		SetExportBatchNo(batchNo).
		SetExportedAt(now).
		SetExportedBy(actorID).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("mark invoice requests exported: %w", err)
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Filter helpers
// ---------------------------------------------------------------------------

func applyTopupOrderFilters(query *dbent.TopupOrderQuery, filters service.InvoiceTopupOrderListFilters, includeUserSearch bool) *dbent.TopupOrderQuery {
	if filters.Status != "" {
		query = query.Where(topuporder.StatusEQ(filters.Status))
	}
	if filters.InvoiceStatus != "" {
		query = query.Where(topuporder.InvoiceStatusEQ(filters.InvoiceStatus))
	}
	if filters.StartTime != nil {
		query = query.Where(topuporder.CreatedAtGTE(*filters.StartTime))
	}
	if filters.EndTime != nil {
		query = query.Where(topuporder.CreatedAtLT(*filters.EndTime))
	}

	search := strings.TrimSpace(filters.Search)
	if search == "" {
		return query
	}

	preds := []predicate.TopupOrder{
		topuporder.OrderNoContainsFold(search),
	}
	if includeUserSearch {
		userPreds := []predicate.User{
			user.EmailContainsFold(search),
			user.UsernameContainsFold(search),
		}
		if id, err := strconv.ParseInt(search, 10, 64); err == nil && id > 0 {
			userPreds = append(userPreds, user.IDEQ(id))
		}
		preds = append(preds, topuporder.HasUserWith(user.Or(userPreds...)))
	}
	return query.Where(topuporder.Or(preds...))
}

func applyInvoiceRequestFilters(query *dbent.InvoiceRequestQuery, filters service.InvoiceRequestListFilters, includeUserSearch bool) *dbent.InvoiceRequestQuery {
	if filters.Status != "" {
		query = query.Where(invoicerequest.StatusEQ(filters.Status))
	}
	if filters.StartTime != nil {
		query = query.Where(invoicerequest.CreatedAtGTE(*filters.StartTime))
	}
	if filters.EndTime != nil {
		query = query.Where(invoicerequest.CreatedAtLT(*filters.EndTime))
	}

	search := strings.TrimSpace(filters.Search)
	if search == "" {
		return query
	}

	preds := []predicate.InvoiceRequest{
		invoicerequest.SerialNoContainsFold(search),
	}
	if includeUserSearch {
		userPreds := []predicate.User{
			user.EmailContainsFold(search),
			user.UsernameContainsFold(search),
		}
		if id, err := strconv.ParseInt(search, 10, 64); err == nil && id > 0 {
			userPreds = append(userPreds, user.IDEQ(id))
		}
		preds = append(preds, invoicerequest.HasUserWith(user.Or(userPreds...)))
	}
	return query.Where(invoicerequest.Or(preds...))
}

func applyExportFilters(query *dbent.InvoiceRequestQuery, filters service.InvoiceRequestListFilters) *dbent.InvoiceRequestQuery {
	query = applyInvoiceRequestFilters(query, filters, true)
	if filters.Status != "" {
		return query
	}
	if filters.IncludeExported {
		return query
	}
	return query.Where(invoicerequest.StatusEQ(service.InvoiceRequestStatusPending))
}

// ---------------------------------------------------------------------------
// Mappers (Ent entity -> service type)
// ---------------------------------------------------------------------------

func mapInvoiceProfiles(entities []*dbent.InvoiceProfile) []service.InvoiceProfile {
	out := make([]service.InvoiceProfile, 0, len(entities))
	for _, e := range entities {
		out = append(out, mapInvoiceProfile(e))
	}
	return out
}

func mapInvoiceProfile(e *dbent.InvoiceProfile) service.InvoiceProfile {
	return service.InvoiceProfile{
		ID:          e.ID,
		UserID:      e.UserID,
		Title:       e.Title,
		TaxNumber:   e.TaxNumber,
		Email:       e.Email,
		Address:     e.Address,
		Phone:       e.Phone,
		BankName:    e.BankName,
		BankAccount: e.BankAccount,
		IsDefault:   e.IsDefault,
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

func mapTopupOrders(entities []*dbent.TopupOrder) []service.InvoiceTopupOrder {
	out := make([]service.InvoiceTopupOrder, 0, len(entities))
	for _, e := range entities {
		out = append(out, mapTopupOrder(e))
	}
	return out
}

func mapTopupOrder(e *dbent.TopupOrder) service.InvoiceTopupOrder {
	if e == nil {
		return service.InvoiceTopupOrder{}
	}
	quote := service.StoredTopupCreditQuote(e.AmountCnyFen, e.BonusAmountCnyFen)
	item := service.InvoiceTopupOrder{
		ID:                   e.ID,
		OrderNo:              e.OrderNo,
		UserID:               e.UserID,
		AmountCNYFen:         e.AmountCnyFen,
		BonusAmountCNYFen:    quote.BonusAmountCNYFen,
		CreditedAmountCNYFen: quote.CreditedAmountCNYFen,
		PayType:              e.PayType,
		Status:               e.Status,
		InvoiceStatus:        e.InvoiceStatus,
		CompletedAt:          e.CompletedAt,
		CreatedAt:            e.CreatedAt,
		UpdatedAt:            e.UpdatedAt,
	}
	if e.Edges.User != nil {
		item.UserEmail = e.Edges.User.Email
		item.UserName = e.Edges.User.Username
	}
	return item
}

func mapInvoiceRequests(entities []*dbent.InvoiceRequest) []service.InvoiceRequest {
	out := make([]service.InvoiceRequest, 0, len(entities))
	for _, e := range entities {
		out = append(out, mapInvoiceRequest(e))
	}
	return out
}

func mapInvoiceRequest(e *dbent.InvoiceRequest) service.InvoiceRequest {
	item := service.InvoiceRequest{
		ID:              e.ID,
		SerialNo:        e.SerialNo,
		UserID:          e.UserID,
		ProfileID:       e.ProfileID,
		ProfileSnapshot: e.ProfileSnapshotJSON,
		TotalAmountFen:  e.TotalAmountFen,
		Status:          e.Status,
		ExportBatchNo:   e.ExportBatchNo,
		ExportedAt:      e.ExportedAt,
		ExportedBy:      e.ExportedBy,
		Remark:          e.Remark,
		RejectReason:    e.RejectReason,
		CompletedAt:     e.CompletedAt,
		CompletedBy:     e.CompletedBy,
		CreatedAt:       e.CreatedAt,
		UpdatedAt:       e.UpdatedAt,
		Orders:          make([]service.InvoiceRequestOrder, 0, len(e.Edges.RequestOrders)),
	}
	if e.Edges.User != nil {
		item.UserEmail = e.Edges.User.Email
		item.UserName = e.Edges.User.Username
	}
	for _, ro := range e.Edges.RequestOrders {
		item.Orders = append(item.Orders, service.InvoiceRequestOrder{
			ID:           ro.ID,
			TopupOrderID: ro.TopupOrderID,
			TopupOrder:   mapTopupOrder(ro.Edges.TopupOrder),
		})
	}
	return item
}

// ---------------------------------------------------------------------------
// Shared string helpers (mirrors those in service layer)
// ---------------------------------------------------------------------------

func trimNullableString(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

package service

// This file provides a test-only InvoiceRepository implementation backed by an
// ent client so that invoice_service_test.go can exercise the full service
// logic against a real (SQLite in-memory) database without importing the
// repository package (which would create an import cycle).
//
// The implementation deliberately mirrors repository/invoice_repo.go so that
// any divergence is caught at compile time via the interface check below.

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
)

var _ InvoiceRepository = (*inMemoryInvoiceRepo)(nil)

type inMemoryInvoiceRepo struct {
	client *dbent.Client
}

func newInMemoryInvoiceRepo(client *dbent.Client) InvoiceRepository {
	return &inMemoryInvoiceRepo{client: client}
}

func (r *inMemoryInvoiceRepo) entClient(ctx context.Context) *dbent.Client {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return r.client
}

// ---------------------------------------------------------------------------
// TopupOrder
// ---------------------------------------------------------------------------

func (r *inMemoryInvoiceRepo) ListUserTopupOrders(ctx context.Context, userID int64, params pagination.PaginationParams, filters InvoiceTopupOrderListFilters) ([]InvoiceTopupOrder, int, error) {
	client := r.entClient(ctx)
	query := client.TopupOrder.Query().Where(topuporder.UserIDEQ(userID))
	query = testApplyTopupOrderFilters(query, filters, false)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count user topup orders: %w", err)
	}
	entities, err := query.Order(dbent.Desc(topuporder.FieldCreatedAt), dbent.Desc(topuporder.FieldID)).
		Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list user topup orders: %w", err)
	}
	return testMapTopupOrders(entities), total, nil
}

func (r *inMemoryInvoiceRepo) ListAdminTopupOrders(ctx context.Context, params pagination.PaginationParams, filters InvoiceTopupOrderListFilters) ([]InvoiceTopupOrder, int, error) {
	client := r.entClient(ctx)
	query := client.TopupOrder.Query().WithUser()
	query = testApplyTopupOrderFilters(query, filters, true)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count admin topup orders: %w", err)
	}
	entities, err := query.Order(dbent.Desc(topuporder.FieldCreatedAt), dbent.Desc(topuporder.FieldID)).
		Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list admin topup orders: %w", err)
	}
	return testMapTopupOrders(entities), total, nil
}

func (r *inMemoryInvoiceRepo) SumSelectableAmountFen(ctx context.Context, userID int64, filters InvoiceTopupOrderListFilters) (int64, error) {
	client := r.entClient(ctx)
	query := client.TopupOrder.Query().Where(
		topuporder.UserIDEQ(userID),
		topuporder.StatusEQ(TopupStatusCompleted),
		topuporder.InvoiceStatusEQ(TopupInvoiceStatusNone),
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
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].Sum, nil
}

func (r *inMemoryInvoiceRepo) GetTopupOrdersByIDs(ctx context.Context, orderIDs []int64) ([]InvoiceTopupOrder, error) {
	client := r.entClient(ctx)
	entities, err := client.TopupOrder.Query().Where(topuporder.IDIn(orderIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query topup orders: %w", err)
	}
	return testMapTopupOrders(entities), nil
}

func (r *inMemoryInvoiceRepo) ClaimTopupOrdersForInvoicing(ctx context.Context, userID int64, orderIDs []int64) (int, error) {
	client := r.entClient(ctx)
	now := time.Now()
	n, err := client.TopupOrder.Update().
		Where(topuporder.IDIn(orderIDs...), topuporder.UserIDEQ(userID),
			topuporder.StatusEQ(TopupStatusCompleted), topuporder.InvoiceStatusEQ(TopupInvoiceStatusNone)).
		SetInvoiceStatus(TopupInvoiceStatusApplied).SetUpdatedAt(now).Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("claim topup orders: %w", err)
	}
	return n, nil
}

func (r *inMemoryInvoiceRepo) ReleaseTopupOrdersFromInvoicing(ctx context.Context, orderIDs []int64) (int, error) {
	client := r.entClient(ctx)
	now := time.Now()
	n, err := client.TopupOrder.Update().
		Where(topuporder.IDIn(orderIDs...), topuporder.InvoiceStatusEQ(TopupInvoiceStatusApplied)).
		SetInvoiceStatus(TopupInvoiceStatusNone).SetUpdatedAt(now).Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("release topup orders: %w", err)
	}
	return n, nil
}

func (r *inMemoryInvoiceRepo) MarkTopupOrdersInvoiced(ctx context.Context, orderIDs []int64) (int, error) {
	client := r.entClient(ctx)
	now := time.Now()
	n, err := client.TopupOrder.Update().
		Where(topuporder.IDIn(orderIDs...), topuporder.InvoiceStatusEQ(TopupInvoiceStatusApplied)).
		SetInvoiceStatus(TopupInvoiceStatusInvoiced).SetUpdatedAt(now).Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("mark invoiced: %w", err)
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// InvoiceProfile
// ---------------------------------------------------------------------------

func (r *inMemoryInvoiceRepo) ListProfiles(ctx context.Context, userID int64) ([]InvoiceProfile, error) {
	client := r.entClient(ctx)
	entities, err := client.InvoiceProfile.Query().
		Where(invoiceprofile.UserIDEQ(userID)).
		Order(dbent.Desc(invoiceprofile.FieldIsDefault), dbent.Desc(invoiceprofile.FieldCreatedAt), dbent.Desc(invoiceprofile.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return testMapInvoiceProfiles(entities), nil
}

func (r *inMemoryInvoiceRepo) CountProfiles(ctx context.Context, userID int64) (int, error) {
	client := r.entClient(ctx)
	return client.InvoiceProfile.Query().Where(invoiceprofile.UserIDEQ(userID)).Count(ctx)
}

func (r *inMemoryInvoiceRepo) GetProfile(ctx context.Context, userID, profileID int64) (*InvoiceProfile, error) {
	client := r.entClient(ctx)
	entity, err := client.InvoiceProfile.Query().
		Where(invoiceprofile.IDEQ(profileID), invoiceprofile.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrInvoiceProfileNotFound
		}
		return nil, err
	}
	out := testMapInvoiceProfile(entity)
	return &out, nil
}

func (r *inMemoryInvoiceRepo) CreateProfile(ctx context.Context, userID int64, input InvoiceProfileInput, isDefault bool) (*InvoiceProfile, error) {
	client := r.entClient(ctx)
	created, err := client.InvoiceProfile.Create().
		SetUserID(userID).
		SetTitle(strings.TrimSpace(input.Title)).
		SetTaxNumber(strings.TrimSpace(input.TaxNumber)).
		SetEmail(strings.TrimSpace(input.Email)).
		SetNillableAddress(testTrimNullable(input.Address)).
		SetNillablePhone(testTrimNullable(input.Phone)).
		SetNillableBankName(testTrimNullable(input.BankName)).
		SetNillableBankAccount(testTrimNullable(input.BankAccount)).
		SetIsDefault(isDefault).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	out := testMapInvoiceProfile(created)
	return &out, nil
}

func (r *inMemoryInvoiceRepo) UpdateProfile(ctx context.Context, profileID int64, input InvoiceProfileInput) (*InvoiceProfile, error) {
	client := r.entClient(ctx)
	updated, err := client.InvoiceProfile.UpdateOneID(profileID).
		SetTitle(strings.TrimSpace(input.Title)).
		SetTaxNumber(strings.TrimSpace(input.TaxNumber)).
		SetEmail(strings.TrimSpace(input.Email)).
		SetNillableAddress(testTrimNullable(input.Address)).
		SetNillablePhone(testTrimNullable(input.Phone)).
		SetNillableBankName(testTrimNullable(input.BankName)).
		SetNillableBankAccount(testTrimNullable(input.BankAccount)).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	out := testMapInvoiceProfile(updated)
	return &out, nil
}

func (r *inMemoryInvoiceRepo) DeleteProfile(ctx context.Context, profileID int64) error {
	client := r.entClient(ctx)
	return client.InvoiceProfile.DeleteOneID(profileID).Exec(ctx)
}

func (r *inMemoryInvoiceRepo) ClearDefaultProfiles(ctx context.Context, userID int64) error {
	client := r.entClient(ctx)
	_, err := client.InvoiceProfile.Update().
		Where(invoiceprofile.UserIDEQ(userID), invoiceprofile.IsDefaultEQ(true)).
		SetIsDefault(false).Save(ctx)
	return err
}

func (r *inMemoryInvoiceRepo) SetProfileDefault(ctx context.Context, profileID int64) (*InvoiceProfile, error) {
	client := r.entClient(ctx)
	updated, err := client.InvoiceProfile.UpdateOneID(profileID).SetIsDefault(true).Save(ctx)
	if err != nil {
		return nil, err
	}
	out := testMapInvoiceProfile(updated)
	return &out, nil
}

func (r *inMemoryInvoiceRepo) FindLatestProfileForUser(ctx context.Context, userID int64) (*InvoiceProfile, error) {
	client := r.entClient(ctx)
	entity, err := client.InvoiceProfile.Query().
		Where(invoiceprofile.UserIDEQ(userID)).
		Order(dbent.Desc(invoiceprofile.FieldCreatedAt), dbent.Desc(invoiceprofile.FieldID)).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	out := testMapInvoiceProfile(entity)
	return &out, nil
}

func (r *inMemoryInvoiceRepo) ProfileInUse(ctx context.Context, profileID int64) (bool, error) {
	client := r.entClient(ctx)
	return client.InvoiceRequest.Query().
		Where(invoicerequest.ProfileIDEQ(profileID),
			invoicerequest.StatusIn(InvoiceRequestStatusPending, InvoiceRequestStatusExported)).
		Exist(ctx)
}

// ---------------------------------------------------------------------------
// InvoiceRequest
// ---------------------------------------------------------------------------

func (r *inMemoryInvoiceRepo) ListUserRequests(ctx context.Context, userID int64, params pagination.PaginationParams, filters InvoiceRequestListFilters) ([]InvoiceRequest, int, error) {
	client := r.entClient(ctx)
	query := client.InvoiceRequest.Query().
		Where(invoicerequest.UserIDEQ(userID)).
		WithRequestOrders(func(q *dbent.InvoiceRequestOrderQuery) { q.WithTopupOrder() })
	query = testApplyInvoiceRequestFilters(query, filters, false)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	entities, err := query.Order(dbent.Desc(invoicerequest.FieldCreatedAt), dbent.Desc(invoicerequest.FieldID)).
		Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return testMapInvoiceRequests(entities), total, nil
}

func (r *inMemoryInvoiceRepo) ListAdminRequests(ctx context.Context, params pagination.PaginationParams, filters InvoiceRequestListFilters) ([]InvoiceRequest, int, error) {
	client := r.entClient(ctx)
	query := client.InvoiceRequest.Query().WithUser().
		WithRequestOrders(func(q *dbent.InvoiceRequestOrderQuery) { q.WithTopupOrder() })
	query = testApplyInvoiceRequestFilters(query, filters, true)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	entities, err := query.Order(dbent.Desc(invoicerequest.FieldCreatedAt), dbent.Desc(invoicerequest.FieldID)).
		Offset(params.Offset()).Limit(params.Limit()).All(ctx)
	if err != nil {
		return nil, 0, err
	}
	return testMapInvoiceRequests(entities), total, nil
}

func (r *inMemoryInvoiceRepo) CountPendingRequests(ctx context.Context) (int, error) {
	client := r.entClient(ctx)
	return client.InvoiceRequest.Query().Where(invoicerequest.StatusEQ(InvoiceRequestStatusPending)).Count(ctx)
}

func (r *inMemoryInvoiceRepo) CreateRequest(ctx context.Context, input CreateInvoiceRequestRepoInput) (int64, string, error) {
	client := r.entClient(ctx)
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
		return 0, "", err
	}
	return created.ID, created.SerialNo, nil
}

func (r *inMemoryInvoiceRepo) UpdateRequestSerialNo(ctx context.Context, requestID int64, serialNo string) error {
	client := r.entClient(ctx)
	return client.InvoiceRequest.UpdateOneID(requestID).SetSerialNo(serialNo).Exec(ctx)
}

func (r *inMemoryInvoiceRepo) CreateRequestOrderLinks(ctx context.Context, requestID int64, orderIDs []int64) error {
	client := r.entClient(ctx)
	builders := make([]*dbent.InvoiceRequestOrderCreate, 0, len(orderIDs))
	for _, id := range orderIDs {
		builders = append(builders, client.InvoiceRequestOrder.Create().
			SetInvoiceRequestID(requestID).SetTopupOrderID(id))
	}
	if len(builders) > 0 {
		if _, err := client.InvoiceRequestOrder.CreateBulk(builders...).Save(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (r *inMemoryInvoiceRepo) GetRequestWithOrders(ctx context.Context, requestID int64) (*InvoiceRequest, error) {
	client := r.entClient(ctx)
	entity, err := client.InvoiceRequest.Query().
		Where(invoicerequest.IDEQ(requestID)).WithRequestOrders().Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrInvoiceRequestNotFound
		}
		return nil, err
	}
	out := testMapInvoiceRequest(entity)
	return &out, nil
}

func (r *inMemoryInvoiceRepo) UpdateRequestStatus(ctx context.Context, requestID int64, input UpdateRequestStatusInput) error {
	client := r.entClient(ctx)
	update := client.InvoiceRequest.UpdateOneID(requestID).SetStatus(input.Status)
	now := time.Now()
	switch input.Status {
	case InvoiceRequestStatusCompleted:
		update.SetCompletedAt(now).SetCompletedBy(input.ActorID)
	case InvoiceRequestStatusRejected:
		update.SetRejectReason(strings.TrimSpace(*input.RejectReason))
	}
	return update.Exec(ctx)
}

func (r *inMemoryInvoiceRepo) DeleteRequestOrderLinks(ctx context.Context, requestID int64) error {
	client := r.entClient(ctx)
	_, err := client.InvoiceRequestOrder.Delete().
		Where(invoicerequestorder.InvoiceRequestIDEQ(requestID)).Exec(ctx)
	return err
}

func (r *inMemoryInvoiceRepo) ListRequestsForExport(ctx context.Context, filters InvoiceRequestListFilters) ([]InvoiceRequest, error) {
	client := r.entClient(ctx)
	query := client.InvoiceRequest.Query().
		WithRequestOrders(func(q *dbent.InvoiceRequestOrderQuery) { q.WithTopupOrder() })
	query = testApplyExportFilters(query, filters)
	entities, err := query.Order(dbent.Desc(invoicerequest.FieldCreatedAt), dbent.Desc(invoicerequest.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	return testMapInvoiceRequests(entities), nil
}

func (r *inMemoryInvoiceRepo) MarkRequestsExported(ctx context.Context, requestIDs []int64, batchNo string, actorID int64) (int, error) {
	client := r.entClient(ctx)
	now := time.Now()
	return client.InvoiceRequest.Update().
		Where(invoicerequest.IDIn(requestIDs...), invoicerequest.StatusEQ(InvoiceRequestStatusPending)).
		SetStatus(InvoiceRequestStatusExported).
		SetExportBatchNo(batchNo).
		SetExportedAt(now).
		SetExportedBy(actorID).
		Save(ctx)
}

// ---------------------------------------------------------------------------
// Filter helpers (local copies for the test adapter)
// ---------------------------------------------------------------------------

func testApplyTopupOrderFilters(query *dbent.TopupOrderQuery, filters InvoiceTopupOrderListFilters, includeUserSearch bool) *dbent.TopupOrderQuery {
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
	preds := []predicate.TopupOrder{topuporder.OrderNoContainsFold(search)}
	if includeUserSearch {
		userPreds := []predicate.User{user.EmailContainsFold(search), user.UsernameContainsFold(search)}
		if id, err := strconv.ParseInt(search, 10, 64); err == nil && id > 0 {
			userPreds = append(userPreds, user.IDEQ(id))
		}
		preds = append(preds, topuporder.HasUserWith(user.Or(userPreds...)))
	}
	return query.Where(topuporder.Or(preds...))
}

func testApplyInvoiceRequestFilters(query *dbent.InvoiceRequestQuery, filters InvoiceRequestListFilters, includeUserSearch bool) *dbent.InvoiceRequestQuery {
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
	preds := []predicate.InvoiceRequest{invoicerequest.SerialNoContainsFold(search)}
	if includeUserSearch {
		userPreds := []predicate.User{user.EmailContainsFold(search), user.UsernameContainsFold(search)}
		if id, err := strconv.ParseInt(search, 10, 64); err == nil && id > 0 {
			userPreds = append(userPreds, user.IDEQ(id))
		}
		preds = append(preds, invoicerequest.HasUserWith(user.Or(userPreds...)))
	}
	return query.Where(invoicerequest.Or(preds...))
}

func testApplyExportFilters(query *dbent.InvoiceRequestQuery, filters InvoiceRequestListFilters) *dbent.InvoiceRequestQuery {
	query = testApplyInvoiceRequestFilters(query, filters, true)
	if filters.Status != "" {
		return query
	}
	if filters.IncludeExported {
		return query
	}
	return query.Where(invoicerequest.StatusEQ(InvoiceRequestStatusPending))
}

// ---------------------------------------------------------------------------
// Mapper helpers
// ---------------------------------------------------------------------------

func testMapInvoiceProfiles(entities []*dbent.InvoiceProfile) []InvoiceProfile {
	out := make([]InvoiceProfile, 0, len(entities))
	for _, e := range entities {
		out = append(out, testMapInvoiceProfile(e))
	}
	return out
}

func testMapInvoiceProfile(e *dbent.InvoiceProfile) InvoiceProfile {
	return InvoiceProfile{
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

func testMapTopupOrders(entities []*dbent.TopupOrder) []InvoiceTopupOrder {
	out := make([]InvoiceTopupOrder, 0, len(entities))
	for _, e := range entities {
		out = append(out, testMapTopupOrder(e))
	}
	return out
}

func testMapTopupOrder(e *dbent.TopupOrder) InvoiceTopupOrder {
	if e == nil {
		return InvoiceTopupOrder{}
	}
	item := InvoiceTopupOrder{
		ID:            e.ID,
		OrderNo:       e.OrderNo,
		UserID:        e.UserID,
		AmountCNYFen:  e.AmountCnyFen,
		PayType:       e.PayType,
		Status:        e.Status,
		InvoiceStatus: e.InvoiceStatus,
		CompletedAt:   e.CompletedAt,
		CreatedAt:     e.CreatedAt,
		UpdatedAt:     e.UpdatedAt,
	}
	if e.Edges.User != nil {
		item.UserEmail = e.Edges.User.Email
		item.UserName = e.Edges.User.Username
	}
	return item
}

func testMapInvoiceRequests(entities []*dbent.InvoiceRequest) []InvoiceRequest {
	out := make([]InvoiceRequest, 0, len(entities))
	for _, e := range entities {
		out = append(out, testMapInvoiceRequest(e))
	}
	return out
}

func testMapInvoiceRequest(e *dbent.InvoiceRequest) InvoiceRequest {
	item := InvoiceRequest{
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
		Orders:          make([]InvoiceRequestOrder, 0, len(e.Edges.RequestOrders)),
	}
	if e.Edges.User != nil {
		item.UserEmail = e.Edges.User.Email
		item.UserName = e.Edges.User.Username
	}
	for _, ro := range e.Edges.RequestOrders {
		item.Orders = append(item.Orders, InvoiceRequestOrder{
			ID:           ro.ID,
			TopupOrderID: ro.TopupOrderID,
			TopupOrder:   testMapTopupOrder(ro.Edges.TopupOrder),
		})
	}
	return item
}

func testTrimNullable(v *string) *string {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return &s
}

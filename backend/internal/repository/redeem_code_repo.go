package repository

import (
	"context"
	"strings"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/redeemcode"
	"github.com/bozhouDev/DragonCode-sub2api/ent/user"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type redeemCodeRepository struct {
	client *dbent.Client
}

func NewRedeemCodeRepository(client *dbent.Client) service.RedeemCodeRepository {
	return &redeemCodeRepository{client: client}
}

func (r *redeemCodeRepository) Create(ctx context.Context, code *service.RedeemCode) error {
	created, err := r.client.RedeemCode.Create().
		SetCode(code.Code).
		SetType(code.Type).
		SetValue(code.Value).
		SetStatus(code.Status).
		SetNotes(code.Notes).
		SetValidityDays(code.ValidityDays).
		SetGroupIds(redeemCodeGroupIDsForPersistence(code.GroupIDs)).
		SetNillableUsedBy(code.UsedBy).
		SetNillableUsedAt(code.UsedAt).
		SetNillableGroupID(code.GroupID).
		SetNillableBatchID(code.BatchID).
		SetPurpose(normalizeRedeemCodePurpose(code.Type, code.Purpose)).
		SetSalesStatus(normalizeRedeemCodeSalesStatus(code.Purpose, code.SalesStatus)).
		SetNillableSoldAt(code.SoldAt).
		SetNillableSoldToNote(trimStringPointer(code.SoldToNote)).
		SetNillableExternalOrderNo(trimStringPointer(code.ExternalOrderNo)).
		SetNillableExternalOrderURL(trimStringPointer(code.ExternalOrderURL)).
		SetNillableInternalNotes(trimStringPointer(code.InternalNotes)).
		Save(ctx)
	if err == nil {
		code.ID = created.ID
		code.CreatedAt = created.CreatedAt
		code.UpdatedAt = created.UpdatedAt
	}
	return err
}

func (r *redeemCodeRepository) CreateBatch(ctx context.Context, codes []service.RedeemCode) error {
	if len(codes) == 0 {
		return nil
	}

	builders := make([]*dbent.RedeemCodeCreate, 0, len(codes))
	for i := range codes {
		c := &codes[i]
		b := r.client.RedeemCode.Create().
			SetCode(c.Code).
			SetType(c.Type).
			SetValue(c.Value).
			SetStatus(c.Status).
			SetNotes(c.Notes).
			SetValidityDays(c.ValidityDays).
			SetGroupIds(redeemCodeGroupIDsForPersistence(c.GroupIDs)).
			SetNillableUsedBy(c.UsedBy).
			SetNillableUsedAt(c.UsedAt).
			SetNillableGroupID(c.GroupID).
			SetNillableBatchID(c.BatchID).
			SetPurpose(normalizeRedeemCodePurpose(c.Type, c.Purpose)).
			SetSalesStatus(normalizeRedeemCodeSalesStatus(c.Purpose, c.SalesStatus)).
			SetNillableSoldAt(c.SoldAt).
			SetNillableSoldToNote(trimStringPointer(c.SoldToNote)).
			SetNillableExternalOrderNo(trimStringPointer(c.ExternalOrderNo)).
			SetNillableExternalOrderURL(trimStringPointer(c.ExternalOrderURL)).
			SetNillableInternalNotes(trimStringPointer(c.InternalNotes))
		builders = append(builders, b)
	}

	return r.client.RedeemCode.CreateBulk(builders...).Exec(ctx)
}

func (r *redeemCodeRepository) GetByID(ctx context.Context, id int64) (*service.RedeemCode, error) {
	m, err := r.client.RedeemCode.Query().
		Where(redeemcode.IDEQ(id)).
		WithUser().
		WithGroup().
		WithBatch().
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}
	return redeemCodeEntityToService(m), nil
}

func (r *redeemCodeRepository) GetByCode(ctx context.Context, code string) (*service.RedeemCode, error) {
	m, err := r.client.RedeemCode.Query().
		Where(redeemcode.CodeEQ(code)).
		WithUser().
		WithGroup().
		WithBatch().
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrRedeemCodeNotFound
		}
		return nil, err
	}
	return redeemCodeEntityToService(m), nil
}

func (r *redeemCodeRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.client.RedeemCode.Delete().Where(redeemcode.IDEQ(id)).Exec(ctx)
	return err
}

func (r *redeemCodeRepository) List(ctx context.Context, params pagination.PaginationParams) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	return r.ListWithFilters(ctx, params, "", "", "")
}

func (r *redeemCodeRepository) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	q := r.client.RedeemCode.Query()

	if codeType != "" {
		q = q.Where(redeemcode.TypeEQ(codeType))
	} else {
		q = q.Where(redeemcode.TypeIn(
			service.RedeemTypeBalance,
			service.RedeemTypeConcurrency,
			service.RedeemTypeSubscription,
			service.RedeemTypeInvitation,
		))
	}
	if status != "" {
		q = q.Where(redeemcode.StatusEQ(status))
	}
	if search != "" {
		q = q.Where(
			redeemcode.Or(
				redeemcode.CodeContainsFold(search),
				redeemcode.HasUserWith(user.EmailContainsFold(search)),
			),
		)
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	codes, err := q.
		WithUser().
		WithGroup().
		WithBatch().
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(dbent.Desc(redeemcode.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	outCodes := redeemCodeEntitiesToService(codes)

	return outCodes, paginationResultFromTotal(int64(total), params), nil
}

func (r *redeemCodeRepository) Update(ctx context.Context, code *service.RedeemCode) error {
	up := r.client.RedeemCode.UpdateOneID(code.ID).
		SetCode(code.Code).
		SetType(code.Type).
		SetValue(code.Value).
		SetStatus(code.Status).
		SetNotes(code.Notes).
		SetValidityDays(code.ValidityDays).
		SetGroupIds(redeemCodeGroupIDsForPersistence(code.GroupIDs)).
		SetPurpose(normalizeRedeemCodePurpose(code.Type, code.Purpose)).
		SetSalesStatus(normalizeRedeemCodeSalesStatus(code.Purpose, code.SalesStatus)).
		SetNillableSoldAt(code.SoldAt).
		SetNillableSoldToNote(trimStringPointer(code.SoldToNote)).
		SetNillableExternalOrderNo(trimStringPointer(code.ExternalOrderNo)).
		SetNillableExternalOrderURL(trimStringPointer(code.ExternalOrderURL)).
		SetNillableInternalNotes(trimStringPointer(code.InternalNotes))

	if code.UsedBy != nil {
		up.SetUsedBy(*code.UsedBy)
	} else {
		up.ClearUsedBy()
	}
	if code.UsedAt != nil {
		up.SetUsedAt(*code.UsedAt)
	} else {
		up.ClearUsedAt()
	}
	if code.GroupID != nil {
		up.SetGroupID(*code.GroupID)
	} else {
		up.ClearGroupID()
	}
	if code.BatchID != nil {
		up.SetBatchID(*code.BatchID)
	} else {
		up.ClearBatchID()
	}

	updated, err := up.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrRedeemCodeNotFound
		}
		return err
	}
	code.CreatedAt = updated.CreatedAt
	code.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *redeemCodeRepository) Use(ctx context.Context, id, userID int64) error {
	now := time.Now()
	client := clientFromContext(ctx, r.client)
	affected, err := client.RedeemCode.Update().
		Where(redeemcode.IDEQ(id), redeemcode.StatusEQ(service.StatusUnused)).
		SetStatus(service.StatusUsed).
		SetUsedBy(userID).
		SetUsedAt(now).
		Save(ctx)
	if err != nil {
		return err
	}
	if affected == 0 {
		return service.ErrRedeemCodeUsed
	}
	_, err = client.RedeemCode.Update().
		Where(
			redeemcode.IDEQ(id),
			redeemcode.TypeEQ(service.RedeemTypeBalance),
			redeemcode.PurposeEQ(service.RedeemCodePurposeSaleRecharge),
			redeemcode.SalesStatusEQ(service.RedeemCodeSalesStatusInventory),
		).
		SetSalesStatus(service.RedeemCodeSalesStatusSold).
		SetSoldAt(now).
		Save(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *redeemCodeRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]service.RedeemCode, error) {
	if limit <= 0 {
		limit = 10
	}

	codes, err := r.client.RedeemCode.Query().
		Where(redeemcode.UsedByEQ(userID)).
		WithGroup().
		WithBatch().
		Order(dbent.Desc(redeemcode.FieldUsedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return redeemCodeEntitiesToService(codes), nil
}

// ListByUserPaginated returns paginated balance/concurrency history for a user.
// Supports optional type filter (e.g. "balance", "admin_balance", "concurrency", "admin_concurrency", "subscription").
func (r *redeemCodeRepository) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]service.RedeemCode, *pagination.PaginationResult, error) {
	q := r.client.RedeemCode.Query().
		Where(redeemcode.UsedByEQ(userID))

	// Optional type filter
	if codeType != "" {
		q = q.Where(redeemcode.TypeEQ(codeType))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	codes, err := q.
		WithGroup().
		WithBatch().
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(dbent.Desc(redeemcode.FieldUsedAt)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	return redeemCodeEntitiesToService(codes), paginationResultFromTotal(int64(total), params), nil
}

// SumPositiveBalanceByUser returns total recharged amount (sum of value > 0 where type is balance/admin_balance/topup).
func (r *redeemCodeRepository) SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error) {
	var result []struct {
		Sum float64 `json:"sum"`
	}
	err := r.client.RedeemCode.Query().
		Where(
			redeemcode.UsedByEQ(userID),
			redeemcode.ValueGT(0),
			redeemcode.TypeIn("balance", "admin_balance", "topup"),
		).
		Aggregate(dbent.As(dbent.Sum(redeemcode.FieldValue), "sum")).
		Scan(ctx, &result)
	if err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Sum, nil
}

func redeemCodeEntityToService(m *dbent.RedeemCode) *service.RedeemCode {
	if m == nil {
		return nil
	}
	out := &service.RedeemCode{
		ID:               m.ID,
		Code:             m.Code,
		Type:             m.Type,
		Value:            m.Value,
		Status:           m.Status,
		UsedBy:           m.UsedBy,
		UsedAt:           m.UsedAt,
		Notes:            derefString(m.Notes),
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
		GroupID:          m.GroupID,
		GroupIDs:         append([]int64(nil), m.GroupIds...),
		ValidityDays:     m.ValidityDays,
		BatchID:          m.BatchID,
		Purpose:          m.Purpose,
		SalesStatus:      m.SalesStatus,
		SoldAt:           m.SoldAt,
		SoldToNote:       derefString(m.SoldToNote),
		ExternalOrderNo:  derefString(m.ExternalOrderNo),
		ExternalOrderURL: derefString(m.ExternalOrderURL),
		InternalNotes:    derefString(m.InternalNotes),
	}
	if m.Edges.User != nil {
		out.User = userEntityToService(m.Edges.User)
	}
	if m.Edges.Group != nil {
		out.Group = groupEntityToService(m.Edges.Group)
	}
	if m.Edges.Batch != nil {
		out.Batch = redeemCodeBatchEntityToService(m.Edges.Batch)
	}
	return out
}

func redeemCodeEntitiesToService(models []*dbent.RedeemCode) []service.RedeemCode {
	out := make([]service.RedeemCode, 0, len(models))
	for i := range models {
		if s := redeemCodeEntityToService(models[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}

func redeemCodeGroupIDsForPersistence(groupIDs []int64) []int64 {
	if len(groupIDs) == 0 {
		return []int64{}
	}
	return append([]int64(nil), groupIDs...)
}

func trimStringPointer(v string) *string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func normalizeRedeemCodePurpose(codeType, purpose string) string {
	trimmed := strings.TrimSpace(purpose)
	switch trimmed {
	case service.RedeemCodePurposeSaleRecharge,
		service.RedeemCodePurposeGift,
		service.RedeemCodePurposeCompensation,
		service.RedeemCodePurposeInternalTest,
		service.RedeemCodePurposeMigration:
		return trimmed
	}
	if codeType == service.RedeemTypeBalance {
		return service.RedeemCodePurposeSaleRecharge
	}
	return service.RedeemCodePurposeMigration
}

func normalizeRedeemCodeSalesStatus(purpose, status string) string {
	trimmed := strings.TrimSpace(status)
	switch trimmed {
	case service.RedeemCodeSalesStatusInventory,
		service.RedeemCodeSalesStatusSold,
		service.RedeemCodeSalesStatusGifted,
		service.RedeemCodeSalesStatusVoid:
		return trimmed
	}
	switch strings.TrimSpace(purpose) {
	case service.RedeemCodePurposeGift, service.RedeemCodePurposeCompensation:
		return service.RedeemCodeSalesStatusGifted
	default:
		return service.RedeemCodeSalesStatusInventory
	}
}

func redeemCodeBatchEntityToService(m *dbent.RedeemCodeBatch) *service.RedeemCodeBatch {
	if m == nil {
		return nil
	}
	return &service.RedeemCodeBatch{
		ID:           m.ID,
		Name:         m.Name,
		Purpose:      m.Purpose,
		FaceValue:    m.FaceValue,
		Currency:     m.Currency,
		SalesChannel: m.SalesChannel,
		ExternalURL:  derefString(m.ExternalURL),
		Notes:        derefString(m.Notes),
		CreatedBy:    m.CreatedBy,
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
}

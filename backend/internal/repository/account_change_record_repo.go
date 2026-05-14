package repository

import (
	"context"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/accountchangerecord"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type accountChangeRecordRepository struct {
	client *dbent.Client
}

func NewAccountChangeRecordRepository(client *dbent.Client) service.AccountChangeRecordRepository {
	return &accountChangeRecordRepository{client: client}
}

func (r *accountChangeRecordRepository) Create(ctx context.Context, record *service.AccountChangeRecord) error {
	client := clientFromContext(ctx, r.client)
	create := client.AccountChangeRecord.Create().
		SetUserID(record.UserID).
		SetAssetType(record.AssetType).
		SetReason(record.Reason).
		SetDelta(record.Delta).
		SetSourceType(record.SourceType).
		SetNotes(record.Notes).
		SetValidityDays(record.ValidityDays)

	if !record.CreatedAt.IsZero() {
		create.SetCreatedAt(record.CreatedAt)
	}
	if record.SourceID != nil {
		create.SetSourceID(*record.SourceID)
	}
	if record.ReferenceNo != "" {
		create.SetReferenceNo(record.ReferenceNo)
	}
	if record.OperatorUserID != nil {
		create.SetOperatorUserID(*record.OperatorUserID)
	}
	if record.GroupID != nil {
		create.SetGroupID(*record.GroupID)
	}
	if record.DedupeKey != nil {
		create.SetDedupeKey(*record.DedupeKey)
	}

	created, err := create.Save(ctx)
	if err != nil {
		return err
	}
	record.ID = created.ID
	record.CreatedAt = created.CreatedAt
	return nil
}

func (r *accountChangeRecordRepository) ListByUser(ctx context.Context, userID int64, limit int, filters service.AccountChangeRecordListFilters) ([]service.AccountChangeRecord, error) {
	if limit <= 0 {
		limit = 10
	}

	query := r.applyFilters(
		r.client.AccountChangeRecord.Query().Where(accountchangerecord.UserIDEQ(userID)),
		filters,
	)

	records, err := query.
		WithGroup().
		Order(dbent.Desc(accountchangerecord.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return accountChangeRecordEntitiesToService(records), nil
}

func (r *accountChangeRecordRepository) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, filters service.AccountChangeRecordListFilters) ([]service.AccountChangeRecord, *pagination.PaginationResult, error) {
	query := r.applyFilters(
		r.client.AccountChangeRecord.Query().Where(accountchangerecord.UserIDEQ(userID)),
		filters,
	)

	total, err := query.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	records, err := query.
		WithGroup().
		Order(dbent.Desc(accountchangerecord.FieldCreatedAt)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	return accountChangeRecordEntitiesToService(records), paginationResultFromTotal(int64(total), params), nil
}

func (r *accountChangeRecordRepository) SumByUser(ctx context.Context, userID int64, assetType, reason string) (float64, error) {
	query := r.client.AccountChangeRecord.Query().
		Where(accountchangerecord.UserIDEQ(userID))

	if assetType != "" {
		query = query.Where(accountchangerecord.AssetTypeEQ(assetType))
	}
	if reason != "" {
		query = query.Where(accountchangerecord.ReasonEQ(reason))
	}

	var result []struct {
		Sum float64 `json:"sum"`
	}
	if err := query.
		Aggregate(dbent.As(dbent.Sum(accountchangerecord.FieldDelta), "sum")).
		Scan(ctx, &result); err != nil {
		return 0, err
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Sum, nil
}

func (r *accountChangeRecordRepository) applyFilters(query *dbent.AccountChangeRecordQuery, filters service.AccountChangeRecordListFilters) *dbent.AccountChangeRecordQuery {
	if len(filters.Reasons) > 0 {
		query = query.Where(accountchangerecord.ReasonIn(filters.Reasons...))
	}

	switch filters.DisplayType {
	case "":
		return query
	case service.RedeemTypeBalance:
		return query.Where(
			accountchangerecord.AssetTypeEQ(service.AccountChangeAssetBalance),
			accountchangerecord.ReasonEQ(service.AccountChangeReasonRedeemCode),
		)
	case service.AdjustmentTypeAdminBalance:
		return query.Where(
			accountchangerecord.AssetTypeEQ(service.AccountChangeAssetBalance),
			accountchangerecord.ReasonEQ(service.AccountChangeReasonAdminAdjustment),
		)
	case service.AccountChangeDisplayTopup:
		return query.Where(
			accountchangerecord.AssetTypeEQ(service.AccountChangeAssetBalance),
			accountchangerecord.ReasonEQ(service.AccountChangeReasonTopup),
		)
	case service.RedeemTypeConcurrency:
		return query.Where(
			accountchangerecord.AssetTypeEQ(service.AccountChangeAssetConcurrency),
			accountchangerecord.ReasonEQ(service.AccountChangeReasonRedeemCode),
		)
	case service.AdjustmentTypeAdminConcurrency:
		return query.Where(
			accountchangerecord.AssetTypeEQ(service.AccountChangeAssetConcurrency),
			accountchangerecord.ReasonEQ(service.AccountChangeReasonAdminAdjustment),
		)
	case service.RedeemTypeSubscription:
		return query.Where(
			accountchangerecord.AssetTypeEQ(service.AccountChangeAssetSubscription),
			accountchangerecord.ReasonEQ(service.AccountChangeReasonRedeemCode),
		)
	default:
		return query.Where(accountchangerecord.IDEQ(-1))
	}
}

func accountChangeRecordEntitiesToService(records []*dbent.AccountChangeRecord) []service.AccountChangeRecord {
	out := make([]service.AccountChangeRecord, 0, len(records))
	for _, record := range records {
		if record == nil {
			continue
		}
		out = append(out, *accountChangeRecordEntityToService(record))
	}
	return out
}

func accountChangeRecordEntityToService(record *dbent.AccountChangeRecord) *service.AccountChangeRecord {
	if record == nil {
		return nil
	}

	out := &service.AccountChangeRecord{
		ID:             record.ID,
		UserID:         record.UserID,
		AssetType:      record.AssetType,
		Reason:         record.Reason,
		Delta:          record.Delta,
		SourceType:     record.SourceType,
		SourceID:       record.SourceID,
		ReferenceNo:    stringOrEmpty(record.ReferenceNo),
		OperatorUserID: record.OperatorUserID,
		Notes:          stringOrEmpty(record.Notes),
		GroupID:        record.GroupID,
		ValidityDays:   record.ValidityDays,
		DedupeKey:      record.DedupeKey,
		CreatedAt:      record.CreatedAt,
	}

	if record.Edges.Group != nil {
		out.Group = groupEntityToService(record.Edges.Group)
	}

	return out
}

func stringOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

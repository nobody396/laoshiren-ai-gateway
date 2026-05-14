package service

import (
	"context"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

type AccountChangeRecordRepository interface {
	Create(ctx context.Context, record *AccountChangeRecord) error
	ListByUser(ctx context.Context, userID int64, limit int, filters AccountChangeRecordListFilters) ([]AccountChangeRecord, error)
	ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, filters AccountChangeRecordListFilters) ([]AccountChangeRecord, *pagination.PaginationResult, error)
	SumByUser(ctx context.Context, userID int64, assetType, reason string) (float64, error)
}

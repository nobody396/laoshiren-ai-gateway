package service

import (
	"context"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

const (
	ChangelogStatusDraft     = domain.ChangelogStatusDraft
	ChangelogStatusPublished = domain.ChangelogStatusPublished
	ChangelogStatusArchived  = domain.ChangelogStatusArchived
)

const (
	ChangelogCategoryFeature     = domain.ChangelogCategoryFeature
	ChangelogCategoryModelConfig = domain.ChangelogCategoryModelConfig
	ChangelogCategoryImprovement = domain.ChangelogCategoryImprovement
	ChangelogCategoryFix         = domain.ChangelogCategoryFix
)

var (
	ErrChangelogNotFound     = domain.ErrChangelogNotFound
	ErrChangelogSlugConflict = domain.ErrChangelogSlugConflict
	ErrChangelogPublished    = domain.ErrChangelogPublished
)

type ChangelogEntry = domain.ChangelogEntry

type ChangelogListFilters struct {
	Status   string
	Category string
	Search   string
}

type ChangelogRepository interface {
	Create(ctx context.Context, entry *ChangelogEntry) error
	GetByID(ctx context.Context, id int64) (*ChangelogEntry, error)
	GetPublishedBySlug(ctx context.Context, slug string, now time.Time) (*ChangelogEntry, error)
	Update(ctx context.Context, entry *ChangelogEntry) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, params pagination.PaginationParams, filters ChangelogListFilters) ([]ChangelogEntry, *pagination.PaginationResult, error)
	ListPublished(ctx context.Context, params pagination.PaginationParams, filters ChangelogListFilters, now time.Time) ([]ChangelogEntry, *pagination.PaginationResult, error)
	LatestPublished(ctx context.Context, now time.Time) (*ChangelogEntry, error)
}

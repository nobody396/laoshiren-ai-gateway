package domain

import (
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const (
	ChangelogStatusDraft     = "draft"
	ChangelogStatusPublished = "published"
	ChangelogStatusArchived  = "archived"
)

const (
	ChangelogCategoryFeature     = "feature"
	ChangelogCategoryModelConfig = "model_config"
	ChangelogCategoryImprovement = "improvement"
	ChangelogCategoryFix         = "fix"
)

var (
	ErrChangelogNotFound     = infraerrors.NotFound("CHANGELOG_NOT_FOUND", "changelog entry not found")
	ErrChangelogSlugConflict = infraerrors.Conflict("CHANGELOG_SLUG_CONFLICT", "changelog slug already exists")
	ErrChangelogPublished    = infraerrors.Conflict("CHANGELOG_PUBLISHED", "published changelog entries must be archived instead of deleted")
)

type ChangelogEntry struct {
	ID              int64
	Slug            string
	Title           string
	Summary         string
	Rationale       string
	Content         string
	Category        string
	RelatedProducts []string
	Status          string
	PublishedAt     *time.Time

	// Git metadata is internal traceability only. It is never included in the
	// public DTO and never used to generate changelog content.
	CommitSHA      *string
	PullRequestURL *string

	CreatedBy *int64
	UpdatedBy *int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

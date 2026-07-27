package web

import (
	"context"
	"time"
)

// ChangelogPage contains the public metadata needed to render a direct
// changelog detail request before the SPA starts.
type ChangelogPage struct {
	Slug        string
	Title       string
	Summary     string
	PublishedAt *time.Time
	UpdatedAt   time.Time
}

// ChangelogPageResolver resolves only published changelog entries. A nil page
// with a nil error means that the public slug does not exist.
type ChangelogPageResolver interface {
	ResolvePublishedChangelogPage(ctx context.Context, slug string) (*ChangelogPage, error)
}

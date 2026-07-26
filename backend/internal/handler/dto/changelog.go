package dto

import (
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type PublicChangelogEntry struct {
	ID              int64      `json:"id"`
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	Summary         string     `json:"summary"`
	Rationale       string     `json:"rationale"`
	Content         string     `json:"content"`
	Category        string     `json:"category"`
	RelatedProducts []string   `json:"related_products"`
	PublishedAt     *time.Time `json:"published_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type AdminChangelogEntry struct {
	PublicChangelogEntry
	Status         string    `json:"status"`
	CommitSHA      *string   `json:"commit_sha,omitempty"`
	PullRequestURL *string   `json:"pull_request_url,omitempty"`
	CreatedBy      *int64    `json:"created_by,omitempty"`
	UpdatedBy      *int64    `json:"updated_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func PublicChangelogFromService(entry *service.ChangelogEntry) *PublicChangelogEntry {
	if entry == nil {
		return nil
	}
	return &PublicChangelogEntry{
		ID:              entry.ID,
		Slug:            entry.Slug,
		Title:           entry.Title,
		Summary:         entry.Summary,
		Rationale:       entry.Rationale,
		Content:         entry.Content,
		Category:        entry.Category,
		RelatedProducts: entry.RelatedProducts,
		PublishedAt:     entry.PublishedAt,
		UpdatedAt:       entry.UpdatedAt,
	}
}

func AdminChangelogFromService(entry *service.ChangelogEntry) *AdminChangelogEntry {
	if entry == nil {
		return nil
	}
	return &AdminChangelogEntry{
		PublicChangelogEntry: *PublicChangelogFromService(entry),
		Status:               entry.Status,
		CommitSHA:            entry.CommitSHA,
		PullRequestURL:       entry.PullRequestURL,
		CreatedBy:            entry.CreatedBy,
		UpdatedBy:            entry.UpdatedBy,
		CreatedAt:            entry.CreatedAt,
	}
}

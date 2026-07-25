package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

var (
	changelogSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	commitSHAPattern     = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)
)

type ChangelogService struct {
	repo ChangelogRepository
	now  func() time.Time
}

func NewChangelogService(repo ChangelogRepository) *ChangelogService {
	return &ChangelogService{repo: repo, now: time.Now}
}

type CreateChangelogInput struct {
	Slug            string
	Title           string
	Summary         string
	Rationale       string
	Content         string
	Category        string
	RelatedProducts []string
	Status          string
	PublishedAt     *time.Time
	CommitSHA       *string
	PullRequestURL  *string
	ActorID         *int64
}

type UpdateChangelogInput struct {
	Slug            *string
	Title           *string
	Summary         *string
	Rationale       *string
	Content         *string
	Category        *string
	RelatedProducts *[]string
	Status          *string
	PublishedAt     *time.Time
	CommitSHA       **string
	PullRequestURL  **string
	ActorID         *int64
}

func (s *ChangelogService) Create(ctx context.Context, input *CreateChangelogInput) (*ChangelogEntry, error) {
	if input == nil {
		return nil, fmt.Errorf("create changelog: nil input")
	}

	entry := &domain.ChangelogEntry{
		Slug:            strings.TrimSpace(strings.ToLower(input.Slug)),
		Title:           strings.TrimSpace(input.Title),
		Summary:         strings.TrimSpace(input.Summary),
		Rationale:       strings.TrimSpace(input.Rationale),
		Content:         strings.TrimSpace(input.Content),
		Category:        strings.TrimSpace(strings.ToLower(input.Category)),
		RelatedProducts: normalizeRelatedProducts(input.RelatedProducts),
		Status:          strings.TrimSpace(strings.ToLower(input.Status)),
		PublishedAt:     input.PublishedAt,
	}
	if entry.Status == "" {
		entry.Status = ChangelogStatusDraft
	}
	if entry.Slug == "" {
		entry.Slug = generateChangelogSlug(s.now())
	}
	if entry.Status == ChangelogStatusPublished && entry.PublishedAt == nil {
		now := s.now().UTC()
		entry.PublishedAt = &now
	}
	if err := validateChangelogEntry(entry); err != nil {
		return nil, fmt.Errorf("create changelog: %w", err)
	}

	commitSHA, err := normalizeCommitSHA(input.CommitSHA)
	if err != nil {
		return nil, fmt.Errorf("create changelog: %w", err)
	}
	pullRequestURL, err := normalizeTraceURL(input.PullRequestURL)
	if err != nil {
		return nil, fmt.Errorf("create changelog: %w", err)
	}
	entry.CommitSHA = commitSHA
	entry.PullRequestURL = pullRequestURL

	if input.ActorID != nil && *input.ActorID > 0 {
		entry.CreatedBy = input.ActorID
		entry.UpdatedBy = input.ActorID
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("create changelog: %w", err)
	}
	return entry, nil
}

func (s *ChangelogService) Update(ctx context.Context, id int64, input *UpdateChangelogInput) (*ChangelogEntry, error) {
	if input == nil {
		return nil, fmt.Errorf("update changelog: nil input")
	}
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	previousStatus := entry.Status

	if input.Slug != nil {
		entry.Slug = strings.TrimSpace(strings.ToLower(*input.Slug))
	}
	if input.Title != nil {
		entry.Title = strings.TrimSpace(*input.Title)
	}
	if input.Summary != nil {
		entry.Summary = strings.TrimSpace(*input.Summary)
	}
	if input.Rationale != nil {
		entry.Rationale = strings.TrimSpace(*input.Rationale)
	}
	if input.Content != nil {
		entry.Content = strings.TrimSpace(*input.Content)
	}
	if input.Category != nil {
		entry.Category = strings.TrimSpace(strings.ToLower(*input.Category))
	}
	if input.RelatedProducts != nil {
		entry.RelatedProducts = normalizeRelatedProducts(*input.RelatedProducts)
	}
	if input.Status != nil {
		entry.Status = strings.TrimSpace(strings.ToLower(*input.Status))
	}
	if input.PublishedAt != nil {
		t := input.PublishedAt.UTC()
		entry.PublishedAt = &t
	}
	if input.CommitSHA != nil {
		entry.CommitSHA, err = normalizeCommitSHA(*input.CommitSHA)
		if err != nil {
			return nil, fmt.Errorf("update changelog: %w", err)
		}
	}
	if input.PullRequestURL != nil {
		entry.PullRequestURL, err = normalizeTraceURL(*input.PullRequestURL)
		if err != nil {
			return nil, fmt.Errorf("update changelog: %w", err)
		}
	}

	if entry.Status == ChangelogStatusPublished && previousStatus != ChangelogStatusPublished && input.PublishedAt == nil {
		now := s.now().UTC()
		entry.PublishedAt = &now
	}
	if err := validateChangelogEntry(entry); err != nil {
		return nil, fmt.Errorf("update changelog: %w", err)
	}
	if input.ActorID != nil && *input.ActorID > 0 {
		entry.UpdatedBy = input.ActorID
	}
	if err := s.repo.Update(ctx, entry); err != nil {
		return nil, fmt.Errorf("update changelog: %w", err)
	}
	return entry, nil
}

func (s *ChangelogService) Delete(ctx context.Context, id int64) error {
	entry, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if entry.Status == ChangelogStatusPublished {
		return ErrChangelogPublished
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete changelog: %w", err)
	}
	return nil
}

func (s *ChangelogService) GetByID(ctx context.Context, id int64) (*ChangelogEntry, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *ChangelogService) GetPublishedBySlug(ctx context.Context, slug string) (*ChangelogEntry, error) {
	return s.repo.GetPublishedBySlug(ctx, strings.TrimSpace(strings.ToLower(slug)), s.now())
}

func (s *ChangelogService) List(ctx context.Context, params pagination.PaginationParams, filters ChangelogListFilters) ([]ChangelogEntry, *pagination.PaginationResult, error) {
	filters.Status = strings.TrimSpace(strings.ToLower(filters.Status))
	filters.Category = strings.TrimSpace(strings.ToLower(filters.Category))
	filters.Search = strings.TrimSpace(filters.Search)
	return s.repo.List(ctx, params, filters)
}

func (s *ChangelogService) ListPublished(ctx context.Context, params pagination.PaginationParams, filters ChangelogListFilters) ([]ChangelogEntry, *pagination.PaginationResult, error) {
	filters.Status = ChangelogStatusPublished
	filters.Category = strings.TrimSpace(strings.ToLower(filters.Category))
	filters.Search = strings.TrimSpace(filters.Search)
	return s.repo.ListPublished(ctx, params, filters, s.now())
}

func (s *ChangelogService) LatestPublished(ctx context.Context) (*ChangelogEntry, error) {
	return s.repo.LatestPublished(ctx, s.now())
}

func validateChangelogEntry(entry *domain.ChangelogEntry) error {
	if entry == nil {
		return fmt.Errorf("entry is required")
	}
	if entry.Slug == "" || len(entry.Slug) > 180 || !changelogSlugPattern.MatchString(entry.Slug) {
		return fmt.Errorf("invalid slug")
	}
	if entry.Title == "" || len([]rune(entry.Title)) > 200 {
		return fmt.Errorf("invalid title")
	}
	if entry.Summary == "" || len([]rune(entry.Summary)) > 500 {
		return fmt.Errorf("invalid summary")
	}
	if entry.Rationale == "" || len([]rune(entry.Rationale)) > 500 {
		return fmt.Errorf("invalid rationale")
	}
	if entry.Content == "" {
		return fmt.Errorf("content is required")
	}
	if !isValidChangelogCategory(entry.Category) {
		return fmt.Errorf("invalid category")
	}
	if !isValidChangelogStatus(entry.Status) {
		return fmt.Errorf("invalid status")
	}
	if entry.Status == ChangelogStatusPublished && entry.PublishedAt == nil {
		return fmt.Errorf("published_at is required for published entries")
	}
	if len(entry.RelatedProducts) > 20 {
		return fmt.Errorf("too many related products")
	}
	for _, item := range entry.RelatedProducts {
		if len([]rune(item)) > 80 {
			return fmt.Errorf("related product is too long")
		}
	}
	return nil
}

func isValidChangelogStatus(value string) bool {
	switch value {
	case ChangelogStatusDraft, ChangelogStatusPublished, ChangelogStatusArchived:
		return true
	default:
		return false
	}
}

func isValidChangelogCategory(value string) bool {
	switch value {
	case ChangelogCategoryFeature, ChangelogCategoryModelConfig, ChangelogCategoryImprovement, ChangelogCategoryFix:
		return true
	default:
		return false
	}
}

func normalizeRelatedProducts(items []string) []string {
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

func normalizeCommitSHA(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil, nil
	}
	if !commitSHAPattern.MatchString(normalized) {
		return nil, fmt.Errorf("invalid commit_sha")
	}
	normalized = strings.ToLower(normalized)
	return &normalized, nil
}

func normalizeTraceURL(value *string) (*string, error) {
	if value == nil {
		return nil, nil
	}
	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil, nil
	}
	if len(normalized) > 500 {
		return nil, fmt.Errorf("pull_request_url is too long")
	}
	parsed, err := url.ParseRequestURI(normalized)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return nil, fmt.Errorf("invalid pull_request_url")
	}
	return &normalized, nil
}

func generateChangelogSlug(now time.Time) string {
	random := make([]byte, 4)
	if _, err := rand.Read(random); err != nil {
		return fmt.Sprintf("update-%s-%d", now.UTC().Format("20060102"), now.UnixNano())
	}
	return fmt.Sprintf("update-%s-%s", now.UTC().Format("20060102"), hex.EncodeToString(random))
}

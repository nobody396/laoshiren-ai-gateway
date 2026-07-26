package service

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type changelogRepositoryStub struct {
	entries     map[int64]*ChangelogEntry
	nextID      int64
	lastFilters ChangelogListFilters
}

func newChangelogRepositoryStub(entries ...*ChangelogEntry) *changelogRepositoryStub {
	repo := &changelogRepositoryStub{entries: make(map[int64]*ChangelogEntry), nextID: 1}
	for _, entry := range entries {
		copied := *entry
		repo.entries[entry.ID] = &copied
		if entry.ID >= repo.nextID {
			repo.nextID = entry.ID + 1
		}
	}
	return repo
}

func (r *changelogRepositoryStub) Create(_ context.Context, entry *ChangelogEntry) error {
	entry.ID = r.nextID
	r.nextID++
	copied := *entry
	r.entries[entry.ID] = &copied
	return nil
}

func (r *changelogRepositoryStub) GetByID(_ context.Context, id int64) (*ChangelogEntry, error) {
	entry, ok := r.entries[id]
	if !ok {
		return nil, ErrChangelogNotFound
	}
	copied := *entry
	return &copied, nil
}

func (r *changelogRepositoryStub) GetPublishedBySlug(_ context.Context, slug string, now time.Time) (*ChangelogEntry, error) {
	for _, entry := range r.entries {
		if entry.Slug == slug && entry.Status == ChangelogStatusPublished &&
			entry.PublishedAt != nil && !entry.PublishedAt.After(now) {
			copied := *entry
			return &copied, nil
		}
	}
	return nil, ErrChangelogNotFound
}

func (r *changelogRepositoryStub) Update(_ context.Context, entry *ChangelogEntry) error {
	copied := *entry
	r.entries[entry.ID] = &copied
	return nil
}

func (r *changelogRepositoryStub) Delete(_ context.Context, id int64) error {
	delete(r.entries, id)
	return nil
}

func (r *changelogRepositoryStub) List(_ context.Context, params pagination.PaginationParams, filters ChangelogListFilters) ([]ChangelogEntry, *pagination.PaginationResult, error) {
	r.lastFilters = filters
	return r.list(params, filters, time.Time{}, false)
}

func (r *changelogRepositoryStub) ListPublished(_ context.Context, params pagination.PaginationParams, filters ChangelogListFilters, now time.Time) ([]ChangelogEntry, *pagination.PaginationResult, error) {
	r.lastFilters = filters
	return r.list(params, filters, now, true)
}

func (r *changelogRepositoryStub) LatestPublished(_ context.Context, now time.Time) (*ChangelogEntry, error) {
	var latest *ChangelogEntry
	for _, entry := range r.entries {
		if entry.Status != ChangelogStatusPublished || entry.PublishedAt == nil || entry.PublishedAt.After(now) {
			continue
		}
		if latest == nil || latest.PublishedAt.Before(*entry.PublishedAt) {
			copied := *entry
			latest = &copied
		}
	}
	return latest, nil
}

func (r *changelogRepositoryStub) list(params pagination.PaginationParams, filters ChangelogListFilters, now time.Time, public bool) ([]ChangelogEntry, *pagination.PaginationResult, error) {
	items := make([]ChangelogEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		if public && (entry.Status != ChangelogStatusPublished || entry.PublishedAt == nil || entry.PublishedAt.After(now)) {
			continue
		}
		if filters.Status != "" && entry.Status != filters.Status {
			continue
		}
		if filters.Category != "" && entry.Category != filters.Category {
			continue
		}
		items = append(items, *entry)
	}
	return items, &pagination.PaginationResult{
		Total:    int64(len(items)),
		Page:     params.Page,
		PageSize: params.PageSize,
		Pages:    1,
	}, nil
}

func TestChangelogCreatePublishedSetsDateAndKeepsGitInternalMetadata(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	repo := newChangelogRepositoryStub()
	svc := NewChangelogService(repo)
	svc.now = func() time.Time { return now }
	sha := "ABCDEF1234567"
	prURL := " https://github.com/example/project/pull/42 "

	entry, err := svc.Create(context.Background(), &CreateChangelogInput{
		Title:          "我们做出了更新日志",
		Summary:        "现在可以持续公开记录产品进展。",
		Rationale:      "普通产品更新不应该打扰所有用户。",
		Content:        "## 做了什么\n\n新增公开时间线与后台编辑器。",
		Category:       ChangelogCategoryFeature,
		Status:         ChangelogStatusPublished,
		CommitSHA:      &sha,
		PullRequestURL: &prURL,
	})

	require.NoError(t, err)
	require.NotEmpty(t, entry.Slug)
	require.Equal(t, now, *entry.PublishedAt)
	require.Equal(t, "abcdef1234567", *entry.CommitSHA)
	require.Equal(t, "https://github.com/example/project/pull/42", *entry.PullRequestURL)
}

func TestChangelogPublishedFeedExcludesDraftAndScheduledEntries(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	repo := newChangelogRepositoryStub(
		&ChangelogEntry{ID: 1, Slug: "public", Status: ChangelogStatusPublished, Category: ChangelogCategoryFeature, PublishedAt: &past},
		&ChangelogEntry{ID: 2, Slug: "draft", Status: ChangelogStatusDraft, Category: ChangelogCategoryFeature},
		&ChangelogEntry{ID: 3, Slug: "scheduled", Status: ChangelogStatusPublished, Category: ChangelogCategoryFeature, PublishedAt: &future},
	)
	svc := NewChangelogService(repo)
	svc.now = func() time.Time { return now }

	items, result, err := svc.ListPublished(context.Background(), pagination.DefaultPagination(), ChangelogListFilters{})

	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)
	require.Len(t, items, 1)
	require.Equal(t, "public", items[0].Slug)
}

func TestChangelogSearchLimitPreservesUTF8(t *testing.T) {
	t.Parallel()

	repo := newChangelogRepositoryStub()
	svc := NewChangelogService(repo)

	_, _, err := svc.ListPublished(
		context.Background(),
		pagination.DefaultPagination(),
		ChangelogListFilters{Search: strings.Repeat("更", 201)},
	)

	require.NoError(t, err)
	require.True(t, utf8.ValidString(repo.lastFilters.Search))
	require.Len(t, []rune(repo.lastFilters.Search), 200)
}

func TestChangelogPublishedEntryMustBeArchivedBeforeDelete(t *testing.T) {
	t.Parallel()

	publishedAt := time.Now().UTC()
	repo := newChangelogRepositoryStub(&ChangelogEntry{
		ID:          9,
		Slug:        "published",
		Status:      ChangelogStatusPublished,
		PublishedAt: &publishedAt,
	})
	svc := NewChangelogService(repo)

	err := svc.Delete(context.Background(), 9)

	require.ErrorIs(t, err, ErrChangelogPublished)
	require.Contains(t, repo.entries, int64(9))
}

func TestChangelogRejectsInvalidGitTraceability(t *testing.T) {
	t.Parallel()

	repo := newChangelogRepositoryStub()
	svc := NewChangelogService(repo)
	sha := "not-a-sha"

	_, err := svc.Create(context.Background(), &CreateChangelogInput{
		Title:     "有效标题",
		Summary:   "有效摘要",
		Rationale: "有效原因",
		Content:   "有效正文",
		Category:  ChangelogCategoryImprovement,
		CommitSHA: &sha,
	})

	require.ErrorContains(t, err, "invalid commit_sha")
	require.Empty(t, repo.entries)
}

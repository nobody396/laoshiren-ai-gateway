package repository

import (
	"context"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/changelogentry"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type changelogRepository struct {
	client *dbent.Client
}

func NewChangelogRepository(client *dbent.Client) service.ChangelogRepository {
	return &changelogRepository{client: client}
}

func (r *changelogRepository) Create(ctx context.Context, entry *service.ChangelogEntry) error {
	client := clientFromContext(ctx, r.client)
	builder := client.ChangelogEntry.Create().
		SetSlug(entry.Slug).
		SetTitle(entry.Title).
		SetSummary(entry.Summary).
		SetRationale(entry.Rationale).
		SetContent(entry.Content).
		SetCategory(entry.Category).
		SetRelatedProducts(entry.RelatedProducts).
		SetStatus(entry.Status)

	if entry.PublishedAt != nil {
		builder.SetPublishedAt(*entry.PublishedAt)
	}
	if entry.CommitSHA != nil {
		builder.SetCommitSha(*entry.CommitSHA)
	}
	if entry.PullRequestURL != nil {
		builder.SetPullRequestURL(*entry.PullRequestURL)
	}
	if entry.CreatedBy != nil {
		builder.SetCreatedBy(*entry.CreatedBy)
	}
	if entry.UpdatedBy != nil {
		builder.SetUpdatedBy(*entry.UpdatedBy)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrChangelogSlugConflict)
	}
	applyChangelogEntity(entry, created)
	return nil
}

func (r *changelogRepository) GetByID(ctx context.Context, id int64) (*service.ChangelogEntry, error) {
	item, err := r.client.ChangelogEntry.Query().
		Where(changelogentry.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrChangelogNotFound, nil)
	}
	return changelogEntityToService(item), nil
}

func (r *changelogRepository) GetPublishedBySlug(ctx context.Context, slug string, now time.Time) (*service.ChangelogEntry, error) {
	item, err := r.client.ChangelogEntry.Query().
		Where(
			changelogentry.SlugEQ(slug),
			changelogentry.StatusEQ(service.ChangelogStatusPublished),
			changelogentry.PublishedAtNotNil(),
			changelogentry.PublishedAtLTE(now),
		).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrChangelogNotFound, nil)
	}
	return changelogEntityToService(item), nil
}

func (r *changelogRepository) Update(ctx context.Context, entry *service.ChangelogEntry) error {
	client := clientFromContext(ctx, r.client)
	builder := client.ChangelogEntry.UpdateOneID(entry.ID).
		SetSlug(entry.Slug).
		SetTitle(entry.Title).
		SetSummary(entry.Summary).
		SetRationale(entry.Rationale).
		SetContent(entry.Content).
		SetCategory(entry.Category).
		SetRelatedProducts(entry.RelatedProducts).
		SetStatus(entry.Status)

	if entry.PublishedAt != nil {
		builder.SetPublishedAt(*entry.PublishedAt)
	} else {
		builder.ClearPublishedAt()
	}
	if entry.CommitSHA != nil {
		builder.SetCommitSha(*entry.CommitSHA)
	} else {
		builder.ClearCommitSha()
	}
	if entry.PullRequestURL != nil {
		builder.SetPullRequestURL(*entry.PullRequestURL)
	} else {
		builder.ClearPullRequestURL()
	}
	if entry.CreatedBy != nil {
		builder.SetCreatedBy(*entry.CreatedBy)
	} else {
		builder.ClearCreatedBy()
	}
	if entry.UpdatedBy != nil {
		builder.SetUpdatedBy(*entry.UpdatedBy)
	} else {
		builder.ClearUpdatedBy()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrChangelogNotFound, service.ErrChangelogSlugConflict)
	}
	entry.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *changelogRepository) Delete(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.ChangelogEntry.Delete().
		Where(changelogentry.IDEQ(id)).
		Exec(ctx)
	return err
}

func (r *changelogRepository) List(
	ctx context.Context,
	params pagination.PaginationParams,
	filters service.ChangelogListFilters,
) ([]service.ChangelogEntry, *pagination.PaginationResult, error) {
	query := r.applyFilters(r.client.ChangelogEntry.Query(), filters)
	return r.listQuery(ctx, query, params)
}

func (r *changelogRepository) ListPublished(
	ctx context.Context,
	params pagination.PaginationParams,
	filters service.ChangelogListFilters,
	now time.Time,
) ([]service.ChangelogEntry, *pagination.PaginationResult, error) {
	filters.Status = service.ChangelogStatusPublished
	query := r.applyFilters(r.client.ChangelogEntry.Query(), filters).
		Where(
			changelogentry.PublishedAtNotNil(),
			changelogentry.PublishedAtLTE(now),
		)
	return r.listQuery(ctx, query, params)
}

func (r *changelogRepository) LatestPublished(ctx context.Context, now time.Time) (*service.ChangelogEntry, error) {
	item, err := r.client.ChangelogEntry.Query().
		Where(
			changelogentry.StatusEQ(service.ChangelogStatusPublished),
			changelogentry.PublishedAtNotNil(),
			changelogentry.PublishedAtLTE(now),
		).
		Order(dbent.Desc(changelogentry.FieldPublishedAt), dbent.Desc(changelogentry.FieldID)).
		First(ctx)
	if dbent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return changelogEntityToService(item), nil
}

func (r *changelogRepository) applyFilters(
	query *dbent.ChangelogEntryQuery,
	filters service.ChangelogListFilters,
) *dbent.ChangelogEntryQuery {
	if filters.Status != "" {
		query = query.Where(changelogentry.StatusEQ(filters.Status))
	}
	if filters.Category != "" {
		query = query.Where(changelogentry.CategoryEQ(filters.Category))
	}
	if filters.Search != "" {
		query = query.Where(changelogentry.Or(
			changelogentry.TitleContainsFold(filters.Search),
			changelogentry.SummaryContainsFold(filters.Search),
			changelogentry.RationaleContainsFold(filters.Search),
			changelogentry.ContentContainsFold(filters.Search),
		))
	}
	return query
}

func (r *changelogRepository) listQuery(
	ctx context.Context,
	query *dbent.ChangelogEntryQuery,
	params pagination.PaginationParams,
) ([]service.ChangelogEntry, *pagination.PaginationResult, error) {
	total, err := query.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	itemsQuery := query.Offset(params.Offset()).Limit(params.Limit())
	for _, order := range changelogListOrders(params) {
		itemsQuery = itemsQuery.Order(order)
	}
	items, err := itemsQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}
	return changelogEntitiesToService(items), paginationResultFromTotal(int64(total), params), nil
}

func changelogListOrders(params pagination.PaginationParams) []func(*entsql.Selector) {
	field := changelogentry.FieldPublishedAt
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	switch sortBy {
	case "title":
		field = changelogentry.FieldTitle
	case "category":
		field = changelogentry.FieldCategory
	case "status":
		field = changelogentry.FieldStatus
	case "created_at":
		field = changelogentry.FieldCreatedAt
	case "updated_at":
		field = changelogentry.FieldUpdatedAt
	case "id":
		field = changelogentry.FieldID
	case "", "published_at":
		field = changelogentry.FieldPublishedAt
	}

	if params.NormalizedSortOrder(pagination.SortOrderDesc) == pagination.SortOrderAsc {
		if field == changelogentry.FieldID {
			return []func(*entsql.Selector){dbent.Asc(field)}
		}
		return []func(*entsql.Selector){dbent.Asc(field), dbent.Asc(changelogentry.FieldID)}
	}
	if field == changelogentry.FieldID {
		return []func(*entsql.Selector){dbent.Desc(field)}
	}
	return []func(*entsql.Selector){dbent.Desc(field), dbent.Desc(changelogentry.FieldID)}
}

func applyChangelogEntity(dst *service.ChangelogEntry, src *dbent.ChangelogEntry) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func changelogEntityToService(item *dbent.ChangelogEntry) *service.ChangelogEntry {
	if item == nil {
		return nil
	}
	return &service.ChangelogEntry{
		ID:              item.ID,
		Slug:            item.Slug,
		Title:           item.Title,
		Summary:         item.Summary,
		Rationale:       item.Rationale,
		Content:         item.Content,
		Category:        item.Category,
		RelatedProducts: item.RelatedProducts,
		Status:          item.Status,
		PublishedAt:     item.PublishedAt,
		CommitSHA:       item.CommitSha,
		PullRequestURL:  item.PullRequestURL,
		CreatedBy:       item.CreatedBy,
		UpdatedBy:       item.UpdatedBy,
		CreatedAt:       item.CreatedAt,
		UpdatedAt:       item.UpdatedAt,
	}
}

func changelogEntitiesToService(items []*dbent.ChangelogEntry) []service.ChangelogEntry {
	out := make([]service.ChangelogEntry, 0, len(items))
	for _, item := range items {
		if entry := changelogEntityToService(item); entry != nil {
			out = append(out, *entry)
		}
	}
	return out
}

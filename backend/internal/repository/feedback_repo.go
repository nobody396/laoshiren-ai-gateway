package repository

import (
	"context"
	"strings"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	dbfeedback "github.com/bozhouDev/DragonCode-sub2api/ent/feedback"
	dbfeedbackreply "github.com/bozhouDev/DragonCode-sub2api/ent/feedbackreply"
	dbuser "github.com/bozhouDev/DragonCode-sub2api/ent/user"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type feedbackRepository struct {
	client *dbent.Client
}

func NewFeedbackRepository(client *dbent.Client) service.FeedbackRepository {
	return &feedbackRepository{client: client}
}

func (r *feedbackRepository) Create(ctx context.Context, feedback *service.Feedback) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.Feedback.Create().
		SetUserID(feedback.UserID).
		SetCategory(feedback.Category).
		SetTitle(feedback.Title).
		SetContent(feedback.Content).
		SetImages(feedback.Images).
		SetContact(feedback.Contact).
		SetPriority(feedback.Priority).
		SetStatus(feedback.Status).
		SetReplyCount(feedback.ReplyCount).
		Save(ctx)
	if err != nil {
		return err
	}
	applyFeedbackEntityToService(feedback, created)
	return nil
}

func (r *feedbackRepository) GetByID(ctx context.Context, id int64) (*service.Feedback, error) {
	item, err := r.client.Feedback.Query().
		Where(dbfeedback.IDEQ(id)).
		WithUser(func(query *dbent.UserQuery) {
			query.Select(dbuser.FieldID, dbuser.FieldEmail, dbuser.FieldUsername)
		}).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrFeedbackNotFound, nil)
	}
	return feedbackEntityToService(item), nil
}

func (r *feedbackRepository) Update(ctx context.Context, feedback *service.Feedback) error {
	client := clientFromContext(ctx, r.client)
	builder := client.Feedback.UpdateOneID(feedback.ID).
		SetCategory(feedback.Category).
		SetTitle(feedback.Title).
		SetContent(feedback.Content).
		SetImages(feedback.Images).
		SetContact(feedback.Contact).
		SetPriority(feedback.Priority).
		SetStatus(feedback.Status).
		SetReplyCount(feedback.ReplyCount)

	if feedback.LastReplyAt != nil {
		builder.SetLastReplyAt(*feedback.LastReplyAt)
	} else {
		builder.ClearLastReplyAt()
	}
	if feedback.LastReplyRole != nil {
		builder.SetLastReplyRole(*feedback.LastReplyRole)
	} else {
		builder.ClearLastReplyRole()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrFeedbackNotFound, nil)
	}
	feedback.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *feedbackRepository) ListByUser(
	ctx context.Context,
	userID int64,
	params pagination.PaginationParams,
	filters service.FeedbackListFilters,
) ([]service.Feedback, *pagination.PaginationResult, error) {
	query := r.client.Feedback.Query().
		Where(dbfeedback.UserIDEQ(userID))

	if filters.Status != "" {
		query = query.Where(dbfeedback.StatusEQ(filters.Status))
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	items, err := query.
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(dbent.Desc(dbfeedback.FieldLastReplyAt), dbent.Desc(dbfeedback.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	return feedbackEntitiesToService(items), paginationResultFromTotal(int64(total), params), nil
}

func (r *feedbackRepository) ListForAdmin(
	ctx context.Context,
	params pagination.PaginationParams,
	filters service.AdminFeedbackListFilters,
) ([]service.Feedback, *pagination.PaginationResult, error) {
	query := r.client.Feedback.Query().
		WithUser(func(userQuery *dbent.UserQuery) {
			userQuery.Select(dbuser.FieldID, dbuser.FieldEmail, dbuser.FieldUsername)
		})

	if filters.Category != "" {
		query = query.Where(dbfeedback.CategoryEQ(filters.Category))
	}
	if filters.Status != "" {
		query = query.Where(dbfeedback.StatusEQ(filters.Status))
	}
	if filters.Priority != "" {
		query = query.Where(dbfeedback.PriorityEQ(filters.Priority))
	}
	if filters.StartTime != nil {
		query = query.Where(dbfeedback.CreatedAtGTE(*filters.StartTime))
	}
	if filters.EndTime != nil {
		query = query.Where(dbfeedback.CreatedAtLTE(*filters.EndTime))
	}
	if filters.Search != "" {
		search := strings.TrimSpace(filters.Search)
		query = query.Where(
			dbfeedback.Or(
				dbfeedback.TitleContainsFold(search),
				dbfeedback.ContentContainsFold(search),
				dbfeedback.ContactContainsFold(search),
				dbfeedback.HasUserWith(
					dbuser.Or(
						dbuser.EmailContainsFold(search),
						dbuser.UsernameContainsFold(search),
					),
				),
			),
		)
	}

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	items, err := query.
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(dbent.Desc(dbfeedback.FieldLastReplyAt), dbent.Desc(dbfeedback.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	return feedbackEntitiesToService(items), paginationResultFromTotal(int64(total), params), nil
}

func (r *feedbackRepository) CreateReply(ctx context.Context, reply *service.FeedbackReply) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.FeedbackReply.Create().
		SetFeedbackID(reply.FeedbackID).
		SetUserID(reply.UserID).
		SetRole(reply.Role).
		SetContent(reply.Content).
		SetImages(reply.Images).
		Save(ctx)
	if err != nil {
		return err
	}
	applyFeedbackReplyEntityToService(reply, created)
	return nil
}

func (r *feedbackRepository) ListRepliesByFeedbackID(ctx context.Context, feedbackID int64) ([]service.FeedbackReply, error) {
	items, err := r.client.FeedbackReply.Query().
		Where(dbfeedbackreply.FeedbackIDEQ(feedbackID)).
		WithUser(func(query *dbent.UserQuery) {
			query.Select(dbuser.FieldID, dbuser.FieldEmail, dbuser.FieldUsername)
		}).
		Order(dbent.Asc(dbfeedbackreply.FieldCreatedAt), dbent.Asc(dbfeedbackreply.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return feedbackRepliesToService(items), nil
}

func (r *feedbackRepository) BatchUpdateStatus(ctx context.Context, ids []int64, status string) (int, error) {
	client := clientFromContext(ctx, r.client)
	affected, err := client.Feedback.Update().
		Where(dbfeedback.IDIn(ids...)).
		Where(dbfeedback.DeletedAtIsNil()).
		SetStatus(status).
		Save(ctx)
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func (r *feedbackRepository) TransitionStatus(ctx context.Context, id int64, fromStatus, toStatus string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.Feedback.Update().
		Where(dbfeedback.ID(id)).
		Where(dbfeedback.Status(fromStatus)).
		Where(dbfeedback.DeletedAtIsNil()).
		SetStatus(toStatus).
		Save(ctx)
	return err
}

func (r *feedbackRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.client.Feedback.Delete().Where(dbfeedback.IDEQ(id)).Exec(ctx)
	return err
}

func (r *feedbackRepository) BatchDelete(ctx context.Context, ids []int64) (int, error) {
	affected, err := r.client.Feedback.Delete().Where(dbfeedback.IDIn(ids...)).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return affected, nil
}

func applyFeedbackEntityToService(dst *service.Feedback, src *dbent.Feedback) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
	dst.DeletedAt = src.DeletedAt
}

func feedbackEntityToService(item *dbent.Feedback) *service.Feedback {
	if item == nil {
		return nil
	}
	feedback := &service.Feedback{
		ID:            item.ID,
		UserID:        item.UserID,
		Category:      item.Category,
		Title:         item.Title,
		Content:       item.Content,
		Images:        append([]string(nil), item.Images...),
		Contact:       item.Contact,
		Priority:      item.Priority,
		Status:        item.Status,
		ReplyCount:    item.ReplyCount,
		LastReplyAt:   item.LastReplyAt,
		LastReplyRole: item.LastReplyRole,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		DeletedAt:     item.DeletedAt,
	}
	if item.Edges.User != nil {
		feedback.User = &service.FeedbackUser{
			ID:       item.Edges.User.ID,
			Email:    item.Edges.User.Email,
			Username: item.Edges.User.Username,
		}
	}
	return feedback
}

func feedbackEntitiesToService(items []*dbent.Feedback) []service.Feedback {
	out := make([]service.Feedback, 0, len(items))
	for i := range items {
		if feedback := feedbackEntityToService(items[i]); feedback != nil {
			out = append(out, *feedback)
		}
	}
	return out
}

func applyFeedbackReplyEntityToService(dst *service.FeedbackReply, src *dbent.FeedbackReply) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.CreatedAt = src.CreatedAt
}

func feedbackReplyEntityToService(item *dbent.FeedbackReply) *service.FeedbackReply {
	if item == nil {
		return nil
	}
	reply := &service.FeedbackReply{
		ID:         item.ID,
		FeedbackID: item.FeedbackID,
		UserID:     item.UserID,
		Role:       item.Role,
		Content:    item.Content,
		Images:     append([]string(nil), item.Images...),
		CreatedAt:  item.CreatedAt,
	}
	if item.Edges.User != nil {
		reply.User = &service.FeedbackUser{
			ID:       item.Edges.User.ID,
			Email:    item.Edges.User.Email,
			Username: item.Edges.User.Username,
		}
	}
	return reply
}

func feedbackRepliesToService(items []*dbent.FeedbackReply) []service.FeedbackReply {
	out := make([]service.FeedbackReply, 0, len(items))
	for i := range items {
		if reply := feedbackReplyEntityToService(items[i]); reply != nil {
			out = append(out, *reply)
		}
	}
	return out
}

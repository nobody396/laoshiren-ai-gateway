package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/enttest"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/repository"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type feedbackRateLimitCacheStub struct {
	allowed    bool
	retryAfter time.Duration
	err        error
}

func (s feedbackRateLimitCacheStub) CheckCreateLimit(context.Context, int64, int, time.Duration) (bool, time.Duration, error) {
	return s.allowed, s.retryAfter, s.err
}

func newFeedbackServiceSQLite(t *testing.T) (*service.FeedbackService, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:feedback_service?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	svc := service.NewFeedbackService(
		repository.NewFeedbackRepository(client),
		nil,
		nil,
		nil,
		nil,
		nil,
		client,
	)
	return svc, client
}

func mustCreateFeedbackUser(t *testing.T, ctx context.Context, client *dbent.Client, email string) int64 {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	return user.ID
}

func TestFeedbackServiceCreateAssignsDefaultPriority(t *testing.T) {
	svc, client := newFeedbackServiceSQLite(t)
	ctx := context.Background()
	userID := mustCreateFeedbackUser(t, ctx, client, "feedback-create@test.com")

	created, err := svc.Create(ctx, userID, service.CreateFeedbackInput{
		Category: service.FeedbackCategoryComplaint,
		Title:    "Need help",
		Content:  "Something is wrong",
	})
	require.NoError(t, err)
	require.Equal(t, service.FeedbackPriorityUrgent, created.Priority)
	require.Equal(t, service.FeedbackStatusPending, created.Status)
	require.Equal(t, 0, created.ReplyCount)
}

func TestFeedbackServiceReplyByUserTransitionsToProcessing(t *testing.T) {
	svc, client := newFeedbackServiceSQLite(t)
	ctx := context.Background()
	userID := mustCreateFeedbackUser(t, ctx, client, "feedback-reply-user@test.com")

	created, err := svc.Create(ctx, userID, service.CreateFeedbackInput{
		Category: service.FeedbackCategoryBug,
		Title:    "Bug here",
		Content:  "Initial content",
	})
	require.NoError(t, err)

	reply, err := svc.ReplyByUser(ctx, userID, created.ID, service.CreateFeedbackReplyInput{
		Content: "More details",
	})
	require.NoError(t, err)
	require.Equal(t, service.FeedbackReplyRoleUser, reply.Role)

	detail, err := svc.GetByUser(ctx, userID, created.ID)
	require.NoError(t, err)
	require.Equal(t, service.FeedbackStatusProcessing, detail.Feedback.Status)
	require.Equal(t, 1, detail.Feedback.ReplyCount)
	require.NotNil(t, detail.Feedback.LastReplyRole)
	require.Equal(t, service.FeedbackReplyRoleUser, *detail.Feedback.LastReplyRole)
	require.Len(t, detail.Replies, 1)
}

func TestFeedbackServiceReplyByAdminTransitionsToReplied(t *testing.T) {
	svc, client := newFeedbackServiceSQLite(t)
	ctx := context.Background()
	userID := mustCreateFeedbackUser(t, ctx, client, "feedback-reply-admin-user@test.com")
	adminID := mustCreateFeedbackUser(t, ctx, client, "feedback-reply-admin@test.com")

	created, err := svc.Create(ctx, userID, service.CreateFeedbackInput{
		Category: service.FeedbackCategorySuggestion,
		Title:    "Feature request",
		Content:  "Please add this",
	})
	require.NoError(t, err)

	reply, err := svc.ReplyByAdmin(ctx, adminID, created.ID, service.CreateFeedbackReplyInput{
		Content: "We are working on it",
	})
	require.NoError(t, err)
	require.Equal(t, service.FeedbackReplyRoleAdmin, reply.Role)

	detail, err := svc.GetForAdmin(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, service.FeedbackStatusReplied, detail.Feedback.Status)
	require.Equal(t, 1, detail.Feedback.ReplyCount)
	require.NotNil(t, detail.Feedback.LastReplyRole)
	require.Equal(t, service.FeedbackReplyRoleAdmin, *detail.Feedback.LastReplyRole)
	require.Len(t, detail.Replies, 1)
}

func TestFeedbackServiceCreateReturnsRateLimitMetadata(t *testing.T) {
	svc := service.NewFeedbackService(
		nil,
		nil,
		nil,
		nil,
		feedbackRateLimitCacheStub{allowed: false, retryAfter: 42 * time.Second},
		nil,
		nil,
	)

	_, err := svc.Create(context.Background(), 1, service.CreateFeedbackInput{
		Category: service.FeedbackCategoryBug,
		Title:    "Too fast",
		Content:  "Rate limited",
	})
	require.Error(t, err)
	require.True(t, errors.IsTooManyRequests(err))
	appErr := errors.FromError(err)
	require.Equal(t, "42", appErr.Metadata["retry_after"])
}

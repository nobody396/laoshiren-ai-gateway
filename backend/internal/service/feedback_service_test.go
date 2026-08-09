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
		repository.NewUserRepository(client, db),
		nil,
		nil,
		nil,
		nil,
		client,
		nil,
		nil,
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

func TestFeedbackServiceCreateDerivesInternalFields(t *testing.T) {
	svc, client := newFeedbackServiceSQLite(t)
	ctx := context.Background()
	userID := mustCreateFeedbackUser(t, ctx, client, "feedback-create@test.com")

	created, err := svc.Create(ctx, userID, service.CreateFeedbackInput{
		Content:   "Something is wrong and the page is confusing",
		RequestID: "request-123\nupstream timed out",
	})
	require.NoError(t, err)
	require.Equal(t, service.FeedbackCategoryOther, created.Category)
	require.Equal(t, "Something is wrong and the page is confusing", created.Title)
	require.Equal(t, "feedback-create@test.com", created.Contact)
	require.Equal(t, "request-123\nupstream timed out", created.RequestID)
	require.Equal(t, service.FeedbackPriorityLow, created.Priority)
	require.Equal(t, service.FeedbackStatusPending, created.Status)
	require.Equal(t, 0, created.ReplyCount)
}

func TestFeedbackServiceReplyByUserTransitionsToProcessing(t *testing.T) {
	svc, client := newFeedbackServiceSQLite(t)
	ctx := context.Background()
	userID := mustCreateFeedbackUser(t, ctx, client, "feedback-reply-user@test.com")

	created, err := svc.Create(ctx, userID, service.CreateFeedbackInput{
		Content: "Initial content",
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
		Content: "Please add this",
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
		nil,
		nil,
	)

	_, err := svc.Create(context.Background(), 1, service.CreateFeedbackInput{
		Content: "Rate limited",
	})
	require.Error(t, err)
	require.True(t, errors.IsTooManyRequests(err))
	appErr := errors.FromError(err)
	require.Equal(t, "42", appErr.Metadata["retry_after"])
}

func TestFeedbackWorkflowRewardIsAtomicAndIdempotent(t *testing.T) {
	svc, client := newFeedbackServiceSQLite(t)
	ctx := context.Background()
	userID := mustCreateFeedbackUser(t, ctx, client, "feedback-reward@test.com")
	created, err := svc.Create(ctx, userID, service.CreateFeedbackInput{Content: "Reproducible failure", RequestID: "req-123"})
	require.NoError(t, err)

	triaged, err := svc.TriageByAgent(ctx, created.ID, service.AgentTriageFeedbackInput{TriageStatus: "confirmed", TriagePriority: "P1", TriageSummary: "Reproduced with the submitted request", TriageConfidence: floatPtr(0.98), RepairDifficulty: "medium", RepairRecommendation: "Validate imported state"})
	require.NoError(t, err)
	require.Equal(t, "confirmed", triaged.TriageStatus)

	results := svc.AcceptBatch(ctx, service.AcceptFeedbackBatchInput{IDs: []int64{created.ID}, BatchID: "FB-20260809-01"})
	require.Len(t, results, 1)
	require.Empty(t, results[0].Error)
	require.NotNil(t, results[0].Reward)
	require.Equal(t, 5.0, results[0].Reward.Amount)

	user, err := client.User.Get(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, 5.0, user.Balance)
	require.Equal(t, 1, client.FeedbackReward.Query().CountX(ctx))
	require.Equal(t, 1, client.AccountChangeRecord.Query().CountX(ctx))
	require.Equal(t, 1, client.UserNotification.Query().CountX(ctx))

	repeated := svc.AcceptBatch(ctx, service.AcceptFeedbackBatchInput{IDs: []int64{created.ID}, BatchID: "FB-20260809-01"})
	require.True(t, repeated[0].AlreadyAccepted)
	user, err = client.User.Get(ctx, userID)
	require.NoError(t, err)
	require.Equal(t, 5.0, user.Balance)
	require.Equal(t, 1, client.FeedbackReward.Query().CountX(ctx))
	require.Equal(t, 1, client.AccountChangeRecord.Query().CountX(ctx))
}

func TestFeedbackWorkflowOwnerOverrideConfirmsTriagedFeedbackBeforeAcceptance(t *testing.T) {
	svc, client := newFeedbackServiceSQLite(t)
	ctx := context.Background()
	userID := mustCreateFeedbackUser(t, ctx, client, "feedback-owner-override@test.com")
	created, err := svc.Create(ctx, userID, service.CreateFeedbackInput{Content: "Please reconsider this product request"})
	require.NoError(t, err)

	_, err = svc.TriageByAgent(ctx, created.ID, service.AgentTriageFeedbackInput{
		TriageStatus:         "not_bug",
		TriagePriority:       "P2",
		TriageSummary:        "Current behavior matches the original policy",
		RepairDifficulty:     "low",
		RepairRecommendation: "Keep the current behavior",
	})
	require.NoError(t, err)

	withoutOverride := svc.AcceptBatch(ctx, service.AcceptFeedbackBatchInput{
		IDs: []int64{created.ID}, BatchID: "FB-20260810-01",
	})
	require.Contains(t, withoutOverride[0].Error, "feedback has not been confirmed")
	require.Zero(t, client.FeedbackReward.Query().CountX(ctx))

	withOverride := svc.AcceptBatch(ctx, service.AcceptFeedbackBatchInput{
		IDs: []int64{created.ID}, BatchID: "FB-20260810-01", OwnerOverride: true,
	})
	require.Empty(t, withOverride[0].Error)
	require.NotNil(t, withOverride[0].Reward)
	require.Equal(t, 5.0, withOverride[0].Reward.Amount)

	detail, err := svc.GetForAdmin(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "confirmed", detail.Feedback.TriageStatus)
	require.Equal(t, "approved", detail.Feedback.OwnerDecision)
	require.Equal(t, 5.0, client.User.GetX(ctx, userID).Balance)
	require.Equal(t, 1, client.FeedbackReward.Query().CountX(ctx))
	require.GreaterOrEqual(t, len(detail.Events), 5)
}

func TestFeedbackWorkflowOwnerOverrideDoesNotBypassInitialTriage(t *testing.T) {
	svc, client := newFeedbackServiceSQLite(t)
	ctx := context.Background()
	userID := mustCreateFeedbackUser(t, ctx, client, "feedback-owner-override-unreviewed@test.com")
	created, err := svc.Create(ctx, userID, service.CreateFeedbackInput{Content: "This still needs investigation"})
	require.NoError(t, err)

	result := svc.AcceptBatch(ctx, service.AcceptFeedbackBatchInput{
		IDs: []int64{created.ID}, BatchID: "FB-20260810-02", OwnerOverride: true,
	})
	require.Contains(t, result[0].Error, "feedback has not been confirmed")
	require.Zero(t, client.FeedbackReward.Query().CountX(ctx))
	require.Zero(t, client.User.GetX(ctx, userID).Balance)
}

func TestFeedbackWorkflowCompleteNotifyAndUserVerification(t *testing.T) {
	svc, client := newFeedbackServiceSQLite(t)
	ctx := context.Background()
	userID := mustCreateFeedbackUser(t, ctx, client, "feedback-verify@test.com")
	created, err := svc.Create(ctx, userID, service.CreateFeedbackInput{Content: "The next step is unclear"})
	require.NoError(t, err)
	_, err = svc.TriageByAgent(ctx, created.ID, service.AgentTriageFeedbackInput{TriageStatus: "confirmed", TriagePriority: "P2", TriageSummary: "Confirmed usability issue", RepairDifficulty: "low", RepairRecommendation: "Add guidance"})
	require.NoError(t, err)
	require.Empty(t, svc.AcceptBatch(ctx, service.AcceptFeedbackBatchInput{IDs: []int64{created.ID}, BatchID: "FB-20260809-02"})[0].Error)
	require.NoError(t, svc.MarkFixing(ctx, created.ID, nil))
	require.NoError(t, svc.Complete(ctx, created.ID, service.CompleteFeedbackInput{ResolvedVersion: "0.1.90", NotifyInApp: true}))

	detail, err := svc.GetByUser(ctx, userID, created.ID)
	require.NoError(t, err)
	require.Equal(t, "awaiting_verification", detail.Feedback.FixStatus)
	require.NotNil(t, detail.Reward)
	require.GreaterOrEqual(t, len(detail.Events), 6)
	require.Equal(t, 2, client.UserNotification.Query().CountX(ctx))
	require.NoError(t, svc.RecordNotification(ctx, created.ID, service.RecordFeedbackNotificationInput{Channel: "email", DeliveryReference: "message-123"}))

	require.NoError(t, svc.VerifyByUser(ctx, userID, created.ID, service.VerifyFeedbackInput{Resolved: true, Note: "Works now"}))
	detail, err = svc.GetByUser(ctx, userID, created.ID)
	require.NoError(t, err)
	require.Equal(t, "verified", detail.Feedback.FixStatus)
	require.Equal(t, service.FeedbackStatusClosed, detail.Feedback.Status)
	require.NotNil(t, detail.Feedback.VerifiedAt)
}

func floatPtr(value float64) *float64 { return &value }

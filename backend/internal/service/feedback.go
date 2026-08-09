package service

import (
	"context"
	"io"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

const (
	FeedbackCategoryBug        = domain.FeedbackCategoryBug
	FeedbackCategorySuggestion = domain.FeedbackCategorySuggestion
	FeedbackCategoryComplaint  = domain.FeedbackCategoryComplaint
	FeedbackCategoryOther      = domain.FeedbackCategoryOther
)

const (
	FeedbackPriorityLow    = domain.FeedbackPriorityLow
	FeedbackPriorityNormal = domain.FeedbackPriorityNormal
	FeedbackPriorityHigh   = domain.FeedbackPriorityHigh
	FeedbackPriorityUrgent = domain.FeedbackPriorityUrgent
)

const (
	FeedbackStatusPending    = domain.FeedbackStatusPending
	FeedbackStatusProcessing = domain.FeedbackStatusProcessing
	FeedbackStatusReplied    = domain.FeedbackStatusReplied
	FeedbackStatusClosed     = domain.FeedbackStatusClosed
)

const (
	FeedbackReplyRoleUser  = domain.FeedbackReplyRoleUser
	FeedbackReplyRoleAdmin = domain.FeedbackReplyRoleAdmin
)

var (
	ErrFeedbackNotFound           = domain.ErrFeedbackNotFound
	ErrFeedbackClosed             = domain.ErrFeedbackClosed
	ErrFeedbackBatchLimitExceeded = domain.ErrFeedbackBatchLimitExceeded
	ErrFeedbackInvalidState       = domain.ErrFeedbackInvalidState
	ErrFeedbackRateLimited        = domain.ErrFeedbackRateLimited
	ErrFeedbackUploadUnavailable  = domain.ErrFeedbackUploadUnavailable
)

type Feedback = domain.Feedback
type FeedbackReply = domain.FeedbackReply
type FeedbackUser = domain.FeedbackUser
type FeedbackEvent = domain.FeedbackEvent
type FeedbackReward = domain.FeedbackReward
type UserNotification = domain.UserNotification

type FeedbackListFilters struct {
	Status string
}

type AdminFeedbackListFilters struct {
	Category      string
	Status        string
	Priority      string
	Search        string
	StartTime     *time.Time
	EndTime       *time.Time
	TriageStatus  string
	OwnerDecision string
	FixStatus     string
}

type CreateFeedbackInput struct {
	Content   string
	Images    []string
	RequestID string
}

type CreateFeedbackReplyInput struct {
	Content string
	Images  []string
}

type UpdateFeedbackByUserInput struct {
	Content   string
	Images    []string
	RequestID string
}

type AgentTriageFeedbackInput struct {
	TriageStatus         string
	TriagePriority       string
	TriageSummary        string
	TriageConfidence     *float64
	RepairDifficulty     string
	RepairRecommendation string
	DuplicateOfID        *int64
}

type AcceptFeedbackBatchInput struct {
	IDs            []int64
	BatchID        string
	OperatorUserID *int64
	OwnerOverride  bool
}

type AcceptFeedbackResult struct {
	FeedbackID      int64           `json:"feedback_id"`
	Reward          *FeedbackReward `json:"reward,omitempty"`
	AlreadyAccepted bool            `json:"already_accepted"`
	Error           string          `json:"error,omitempty"`
}

type CompleteFeedbackInput struct {
	ResolvedVersion string
	NotifyInApp     bool
	OperatorUserID  *int64
}

type RecordFeedbackNotificationInput struct {
	Channel           string
	DeliveryReference string
	OperatorUserID    *int64
}

type VerifyFeedbackInput struct {
	Resolved bool
	Note     string
}

type FeedbackRewardListFilters struct {
	UserID  *int64
	BatchID string
}

type UserNotificationListResult struct {
	Items       []UserNotification `json:"items"`
	UnreadCount int                `json:"unread_count"`
}

type UpdateFeedbackStatusInput struct {
	Status string
}

type UpdateFeedbackPriorityInput struct {
	Priority string
}

type BatchUpdateFeedbackStatusInput struct {
	IDs    []int64
	Status string
}

type FeedbackDetail struct {
	Feedback Feedback
	Replies  []FeedbackReply
	Events   []FeedbackEvent
	Reward   *FeedbackReward
}

type FeedbackRepository interface {
	Create(ctx context.Context, feedback *Feedback) error
	GetByID(ctx context.Context, id int64) (*Feedback, error)
	Update(ctx context.Context, feedback *Feedback) error
	ListByUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters FeedbackListFilters) ([]Feedback, *pagination.PaginationResult, error)
	ListForAdmin(ctx context.Context, params pagination.PaginationParams, filters AdminFeedbackListFilters) ([]Feedback, *pagination.PaginationResult, error)
	CreateReply(ctx context.Context, reply *FeedbackReply) error
	ListRepliesByFeedbackID(ctx context.Context, feedbackID int64) ([]FeedbackReply, error)
	BatchUpdateStatus(ctx context.Context, ids []int64, status string) (int, error)
	TransitionStatus(ctx context.Context, id int64, fromStatus, toStatus string) error
	Delete(ctx context.Context, id int64) error
	BatchDelete(ctx context.Context, ids []int64) (int, error)
}

type FeedbackRateLimitCache interface {
	CheckCreateLimit(ctx context.Context, userID int64, limit int, window time.Duration) (allowed bool, retryAfter time.Duration, err error)
}

type FeedbackImageStorage interface {
	Enabled(ctx context.Context) bool
	UploadObject(ctx context.Context, objectKey string, body io.Reader, size int64, contentType string) error
	GetAccessURL(ctx context.Context, objectKey string) (string, error)
	ResolveToken(ctx context.Context, token string, expiry time.Duration) (*FeedbackImageAccess, error)
}

type FeedbackImageAccess struct {
	RedirectURL   string
	Reader        io.ReadCloser
	ContentType   string
	ContentLength int64
}

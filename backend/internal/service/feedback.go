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

type FeedbackListFilters struct {
	Status string
}

type AdminFeedbackListFilters struct {
	Category  string
	Status    string
	Priority  string
	Search    string
	StartTime *time.Time
	EndTime   *time.Time
}

type CreateFeedbackInput struct {
	Category string
	Title    string
	Content  string
	Images   []string
	Contact  string
}

type CreateFeedbackReplyInput struct {
	Content string
	Images  []string
}

type UpdateFeedbackByUserInput struct {
	Category string
	Title    string
	Content  string
	Images   []string
	Contact  string
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
}

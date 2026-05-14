package domain

import (
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const (
	FeedbackCategoryBug        = "bug"
	FeedbackCategorySuggestion = "suggestion"
	FeedbackCategoryComplaint  = "complaint"
	FeedbackCategoryOther      = "other"
)

const (
	FeedbackPriorityLow    = "low"
	FeedbackPriorityNormal = "normal"
	FeedbackPriorityHigh   = "high"
	FeedbackPriorityUrgent = "urgent"
)

const (
	FeedbackStatusPending    = "pending"
	FeedbackStatusProcessing = "processing"
	FeedbackStatusReplied    = "replied"
	FeedbackStatusClosed     = "closed"
)

const (
	FeedbackReplyRoleUser  = "user"
	FeedbackReplyRoleAdmin = "admin"
)

var (
	ErrFeedbackNotFound           = infraerrors.NotFound("FEEDBACK_NOT_FOUND", "feedback not found")
	ErrFeedbackClosed             = infraerrors.Conflict("FEEDBACK_CLOSED", "feedback is closed")
	ErrFeedbackBatchLimitExceeded = infraerrors.BadRequest("FEEDBACK_BATCH_LIMIT_EXCEEDED", "batch update supports at most 50 items")
	ErrFeedbackInvalidState       = infraerrors.BadRequest("FEEDBACK_INVALID_STATE", "invalid feedback state")
	ErrFeedbackRateLimited        = infraerrors.TooManyRequests("FEEDBACK_RATE_LIMITED", "too many feedback submissions, please try again later")
	ErrFeedbackUploadUnavailable  = infraerrors.ServiceUnavailable("FEEDBACK_UPLOAD_UNAVAILABLE", "feedback image upload is unavailable")
)

type FeedbackUser struct {
	ID       int64
	Email    string
	Username string
}

type Feedback struct {
	ID            int64
	UserID        int64
	Category      string
	Title         string
	Content       string
	Images        []string
	Contact       string
	Priority      string
	Status        string
	ReplyCount    int
	LastReplyAt   *time.Time
	LastReplyRole *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
	User          *FeedbackUser
}

type FeedbackReply struct {
	ID         int64
	FeedbackID int64
	UserID     int64
	Role       string
	Content    string
	Images     []string
	CreatedAt  time.Time
	User       *FeedbackUser
}

func NormalizeFeedbackCategory(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func NormalizeFeedbackPriority(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func NormalizeFeedbackStatus(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func NormalizeFeedbackReplyRole(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func IsValidFeedbackCategory(value string) bool {
	switch NormalizeFeedbackCategory(value) {
	case FeedbackCategoryBug, FeedbackCategorySuggestion, FeedbackCategoryComplaint, FeedbackCategoryOther:
		return true
	default:
		return false
	}
}

func IsValidFeedbackPriority(value string) bool {
	switch NormalizeFeedbackPriority(value) {
	case FeedbackPriorityLow, FeedbackPriorityNormal, FeedbackPriorityHigh, FeedbackPriorityUrgent:
		return true
	default:
		return false
	}
}

func IsValidFeedbackStatus(value string) bool {
	switch NormalizeFeedbackStatus(value) {
	case FeedbackStatusPending, FeedbackStatusProcessing, FeedbackStatusReplied, FeedbackStatusClosed:
		return true
	default:
		return false
	}
}

func IsValidFeedbackReplyRole(value string) bool {
	switch NormalizeFeedbackReplyRole(value) {
	case FeedbackReplyRoleUser, FeedbackReplyRoleAdmin:
		return true
	default:
		return false
	}
}

func DefaultFeedbackPriority(category string) string {
	switch NormalizeFeedbackCategory(category) {
	case FeedbackCategoryBug:
		return FeedbackPriorityHigh
	case FeedbackCategoryComplaint:
		return FeedbackPriorityUrgent
	case FeedbackCategorySuggestion:
		return FeedbackPriorityNormal
	default:
		return FeedbackPriorityLow
	}
}

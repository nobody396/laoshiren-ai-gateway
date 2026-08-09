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
	FeedbackTriageUnreviewed      = "unreviewed"
	FeedbackTriageConfirmed       = "confirmed"
	FeedbackTriageNeedsInfo       = "needs_info"
	FeedbackTriageDuplicate       = "duplicate"
	FeedbackTriageCannotReproduce = "cannot_reproduce"
	FeedbackTriageNotBug          = "not_bug"
)

const (
	FeedbackTriagePriorityP0 = "P0"
	FeedbackTriagePriorityP1 = "P1"
	FeedbackTriagePriorityP2 = "P2"
	FeedbackTriagePriorityP3 = "P3"
)

const (
	FeedbackDifficultyUnknown = "unknown"
	FeedbackDifficultyLow     = "low"
	FeedbackDifficultyMedium  = "medium"
	FeedbackDifficultyHigh    = "high"
)

const (
	FeedbackDecisionPending  = "pending"
	FeedbackDecisionApproved = "approved"
	FeedbackDecisionDeferred = "deferred"
	FeedbackDecisionRejected = "rejected"
)

const (
	FeedbackFixNotStarted     = "not_started"
	FeedbackFixFixing         = "fixing"
	FeedbackFixAwaitingVerify = "awaiting_verification"
	FeedbackFixVerified       = "verified"
	FeedbackFixReopened       = "reopened"
)

const FeedbackRewardAmount = 5.0

const (
	FeedbackEventSubmitted     = "submitted"
	FeedbackEventTriaged       = "triaged"
	FeedbackEventAccepted      = "accepted"
	FeedbackEventRewardGranted = "reward_granted"
	FeedbackEventFixStarted    = "fix_started"
	FeedbackEventFixed         = "fixed"
	FeedbackEventNotified      = "notified"
	FeedbackEventVerified      = "verified"
	FeedbackEventReopened      = "reopened"
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
	ErrFeedbackAlreadyTriaged     = infraerrors.Conflict("FEEDBACK_ALREADY_TRIAGED", "feedback has already been triaged")
	ErrFeedbackNotConfirmed       = infraerrors.Conflict("FEEDBACK_NOT_CONFIRMED", "feedback has not been confirmed")
	ErrFeedbackNotApproved        = infraerrors.Conflict("FEEDBACK_NOT_APPROVED", "feedback has not been approved")
	ErrFeedbackNotAwaitingVerify  = infraerrors.Conflict("FEEDBACK_NOT_AWAITING_VERIFICATION", "feedback is not awaiting verification")
)

type FeedbackUser struct {
	ID       int64
	Email    string
	Username string
}

type Feedback struct {
	ID                   int64
	UserID               int64
	Category             string
	Title                string
	Content              string
	Images               []string
	Contact              string
	RequestID            string
	Priority             string
	Status               string
	TriageStatus         string
	TriagePriority       string
	TriageSummary        string
	TriageConfidence     *float64
	RepairDifficulty     string
	RepairRecommendation string
	OwnerDecision        string
	FixStatus            string
	DuplicateOfID        *int64
	ResolvedVersion      string
	AcceptedAt           *time.Time
	ResolvedAt           *time.Time
	VerifiedAt           *time.Time
	ReplyCount           int
	LastReplyAt          *time.Time
	LastReplyRole        *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            *time.Time
	User                 *FeedbackUser
}

type FeedbackEvent struct {
	ID          int64
	FeedbackID  int64
	EventType   string
	ActorType   string
	ActorUserID *int64
	Summary     string
	Metadata    map[string]string
	CreatedAt   time.Time
}

type FeedbackReward struct {
	ID                    int64         `json:"id"`
	FeedbackID            int64         `json:"feedback_id"`
	UserID                int64         `json:"user_id"`
	Amount                float64       `json:"amount"`
	Reason                string        `json:"reason"`
	BatchID               string        `json:"batch_id"`
	OperatorUserID        *int64        `json:"operator_user_id,omitempty"`
	AccountChangeRecordID *int64        `json:"account_change_record_id,omitempty"`
	GrantedAt             time.Time     `json:"granted_at"`
	CreatedAt             time.Time     `json:"created_at"`
	User                  *FeedbackUser `json:"user,omitempty"`
}

type UserNotification struct {
	ID         int64
	UserID     int64
	FeedbackID *int64
	Type       string
	Title      string
	Body       string
	ActionURL  string
	ReadAt     *time.Time
	CreatedAt  time.Time
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

func NormalizeFeedbackTriageStatus(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}
func NormalizeFeedbackTriagePriority(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}
func NormalizeFeedbackDifficulty(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func IsValidFeedbackTriageStatus(value string) bool {
	switch NormalizeFeedbackTriageStatus(value) {
	case FeedbackTriageConfirmed, FeedbackTriageNeedsInfo, FeedbackTriageDuplicate, FeedbackTriageCannotReproduce, FeedbackTriageNotBug:
		return true
	default:
		return false
	}
}

func IsValidFeedbackTriagePriority(value string) bool {
	switch NormalizeFeedbackTriagePriority(value) {
	case FeedbackTriagePriorityP0, FeedbackTriagePriorityP1, FeedbackTriagePriorityP2, FeedbackTriagePriorityP3:
		return true
	default:
		return false
	}
}

func IsValidFeedbackDifficulty(value string) bool {
	switch NormalizeFeedbackDifficulty(value) {
	case FeedbackDifficultyUnknown, FeedbackDifficultyLow, FeedbackDifficultyMedium, FeedbackDifficultyHigh:
		return true
	default:
		return false
	}
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

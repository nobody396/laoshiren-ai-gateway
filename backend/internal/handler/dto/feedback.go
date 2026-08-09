package dto

import (
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type FeedbackUser struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

type Feedback struct {
	ID                   int64         `json:"id"`
	UserID               int64         `json:"user_id"`
	Category             string        `json:"category"`
	Title                string        `json:"title"`
	Content              string        `json:"content"`
	Images               []string      `json:"images"`
	Contact              string        `json:"contact"`
	RequestID            string        `json:"request_id"`
	Priority             string        `json:"priority"`
	Status               string        `json:"status"`
	TriageStatus         string        `json:"triage_status"`
	TriagePriority       string        `json:"triage_priority"`
	TriageSummary        string        `json:"triage_summary"`
	TriageConfidence     *float64      `json:"triage_confidence,omitempty"`
	RepairDifficulty     string        `json:"repair_difficulty"`
	RepairRecommendation string        `json:"repair_recommendation"`
	OwnerDecision        string        `json:"owner_decision"`
	FixStatus            string        `json:"fix_status"`
	DuplicateOfID        *int64        `json:"duplicate_of_id,omitempty"`
	ResolvedVersion      string        `json:"resolved_version"`
	AcceptedAt           *time.Time    `json:"accepted_at,omitempty"`
	ResolvedAt           *time.Time    `json:"resolved_at,omitempty"`
	VerifiedAt           *time.Time    `json:"verified_at,omitempty"`
	ReplyCount           int           `json:"reply_count"`
	LastReplyAt          *time.Time    `json:"last_reply_at,omitempty"`
	LastReplyRole        *string       `json:"last_reply_role,omitempty"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
	User                 *FeedbackUser `json:"user,omitempty"`
}

type FeedbackReply struct {
	ID         int64         `json:"id"`
	FeedbackID int64         `json:"feedback_id"`
	UserID     int64         `json:"user_id"`
	Role       string        `json:"role"`
	Content    string        `json:"content"`
	Images     []string      `json:"images"`
	CreatedAt  time.Time     `json:"created_at"`
	User       *FeedbackUser `json:"user,omitempty"`
}

type FeedbackDetail struct {
	Feedback
	Replies []FeedbackReply `json:"replies"`
	Events  []FeedbackEvent `json:"events"`
	Reward  *FeedbackReward `json:"reward,omitempty"`
}

type FeedbackEvent struct {
	ID          int64             `json:"id"`
	FeedbackID  int64             `json:"feedback_id"`
	EventType   string            `json:"event_type"`
	ActorType   string            `json:"actor_type"`
	ActorUserID *int64            `json:"actor_user_id,omitempty"`
	Summary     string            `json:"summary"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
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
	User                  *FeedbackUser `json:"user,omitempty"`
}

type UserNotification struct {
	ID         int64      `json:"id"`
	FeedbackID *int64     `json:"feedback_id,omitempty"`
	Type       string     `json:"type"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	ActionURL  string     `json:"action_url"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func UserNotificationFromService(item *service.UserNotification) *UserNotification {
	if item == nil {
		return nil
	}
	return &UserNotification{ID: item.ID, FeedbackID: item.FeedbackID, Type: item.Type, Title: item.Title, Body: item.Body, ActionURL: item.ActionURL, ReadAt: item.ReadAt, CreatedAt: item.CreatedAt}
}

func FeedbackFromService(item *service.Feedback) *Feedback {
	if item == nil {
		return nil
	}
	dto := &Feedback{
		ID:                   item.ID,
		UserID:               item.UserID,
		Category:             item.Category,
		Title:                item.Title,
		Content:              item.Content,
		Images:               append([]string(nil), item.Images...),
		Contact:              item.Contact,
		RequestID:            item.RequestID,
		Priority:             item.Priority,
		Status:               item.Status,
		TriageStatus:         item.TriageStatus,
		TriagePriority:       item.TriagePriority,
		TriageSummary:        item.TriageSummary,
		TriageConfidence:     item.TriageConfidence,
		RepairDifficulty:     item.RepairDifficulty,
		RepairRecommendation: item.RepairRecommendation,
		OwnerDecision:        item.OwnerDecision,
		FixStatus:            item.FixStatus,
		DuplicateOfID:        item.DuplicateOfID,
		ResolvedVersion:      item.ResolvedVersion,
		AcceptedAt:           item.AcceptedAt,
		ResolvedAt:           item.ResolvedAt,
		VerifiedAt:           item.VerifiedAt,
		ReplyCount:           item.ReplyCount,
		LastReplyAt:          item.LastReplyAt,
		LastReplyRole:        item.LastReplyRole,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
	if item.User != nil {
		dto.User = &FeedbackUser{
			ID:       item.User.ID,
			Email:    item.User.Email,
			Username: item.User.Username,
		}
	}
	return dto
}

func FeedbackForUserFromService(item *service.Feedback) *Feedback {
	out := FeedbackFromService(item)
	if out == nil {
		return nil
	}
	out.TriageSummary = ""
	out.TriageConfidence = nil
	out.RepairDifficulty = ""
	out.RepairRecommendation = ""
	out.DuplicateOfID = nil
	out.User = nil
	return out
}

func FeedbackReplyFromService(item *service.FeedbackReply) *FeedbackReply {
	if item == nil {
		return nil
	}
	dto := &FeedbackReply{
		ID:         item.ID,
		FeedbackID: item.FeedbackID,
		UserID:     item.UserID,
		Role:       item.Role,
		Content:    item.Content,
		Images:     append([]string(nil), item.Images...),
		CreatedAt:  item.CreatedAt,
	}
	if item.User != nil {
		dto.User = &FeedbackUser{
			ID:       item.User.ID,
			Email:    item.User.Email,
			Username: item.User.Username,
		}
	}
	return dto
}

func FeedbackDetailFromService(item *service.FeedbackDetail) *FeedbackDetail {
	if item == nil {
		return nil
	}
	base := FeedbackFromService(&item.Feedback)
	if base == nil {
		return nil
	}
	out := &FeedbackDetail{Feedback: *base}
	out.Replies = make([]FeedbackReply, 0, len(item.Replies))
	for i := range item.Replies {
		if reply := FeedbackReplyFromService(&item.Replies[i]); reply != nil {
			out.Replies = append(out.Replies, *reply)
		}
	}
	out.Events = make([]FeedbackEvent, 0, len(item.Events))
	for _, event := range item.Events {
		out.Events = append(out.Events, FeedbackEvent{ID: event.ID, FeedbackID: event.FeedbackID, EventType: event.EventType, ActorType: event.ActorType, ActorUserID: event.ActorUserID, Summary: event.Summary, Metadata: event.Metadata, CreatedAt: event.CreatedAt})
	}
	if item.Reward != nil {
		r := item.Reward
		out.Reward = &FeedbackReward{ID: r.ID, FeedbackID: r.FeedbackID, UserID: r.UserID, Amount: r.Amount, Reason: r.Reason, BatchID: r.BatchID, OperatorUserID: r.OperatorUserID, AccountChangeRecordID: r.AccountChangeRecordID, GrantedAt: r.GrantedAt}
	}
	return out
}

func FeedbackRewardFromService(item *service.FeedbackReward) *FeedbackReward {
	if item == nil {
		return nil
	}
	out := &FeedbackReward{
		ID:                    item.ID,
		FeedbackID:            item.FeedbackID,
		UserID:                item.UserID,
		Amount:                item.Amount,
		Reason:                item.Reason,
		BatchID:               item.BatchID,
		OperatorUserID:        item.OperatorUserID,
		AccountChangeRecordID: item.AccountChangeRecordID,
		GrantedAt:             item.GrantedAt,
	}
	if item.User != nil {
		out.User = &FeedbackUser{ID: item.User.ID, Email: item.User.Email, Username: item.User.Username}
	}
	return out
}

func FeedbackDetailForUserFromService(item *service.FeedbackDetail) *FeedbackDetail {
	if item == nil {
		return nil
	}
	base := FeedbackForUserFromService(&item.Feedback)
	if base == nil {
		return nil
	}
	out := &FeedbackDetail{Feedback: *base, Reward: nil}
	out.Replies = make([]FeedbackReply, 0, len(item.Replies))
	for i := range item.Replies {
		if reply := FeedbackReplyFromService(&item.Replies[i]); reply != nil {
			out.Replies = append(out.Replies, *reply)
		}
	}
	out.Events = make([]FeedbackEvent, 0, len(item.Events))
	for _, event := range item.Events {
		metadata := map[string]string{}
		for _, key := range []string{"amount", "version", "channel", "note"} {
			if value := event.Metadata[key]; value != "" {
				metadata[key] = value
			}
		}
		out.Events = append(out.Events, FeedbackEvent{ID: event.ID, FeedbackID: event.FeedbackID, EventType: event.EventType, ActorType: event.ActorType, Summary: event.Summary, Metadata: metadata, CreatedAt: event.CreatedAt})
	}
	if item.Reward != nil {
		r := item.Reward
		out.Reward = &FeedbackReward{ID: r.ID, FeedbackID: r.FeedbackID, UserID: r.UserID, Amount: r.Amount, Reason: r.Reason, GrantedAt: r.GrantedAt}
	}
	return out
}

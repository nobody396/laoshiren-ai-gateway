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
	ID            int64         `json:"id"`
	UserID        int64         `json:"user_id"`
	Category      string        `json:"category"`
	Title         string        `json:"title"`
	Content       string        `json:"content"`
	Images        []string      `json:"images"`
	Contact       string        `json:"contact"`
	Priority      string        `json:"priority"`
	Status        string        `json:"status"`
	ReplyCount    int           `json:"reply_count"`
	LastReplyAt   *time.Time    `json:"last_reply_at,omitempty"`
	LastReplyRole *string       `json:"last_reply_role,omitempty"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	User          *FeedbackUser `json:"user,omitempty"`
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
}

func FeedbackFromService(item *service.Feedback) *Feedback {
	if item == nil {
		return nil
	}
	dto := &Feedback{
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
	return out
}

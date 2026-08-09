package dto

import (
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

func TestFeedbackRewardFromServiceIncludesUserIdentity(t *testing.T) {
	now := time.Now()
	item := &service.FeedbackReward{
		ID:         2,
		FeedbackID: 5,
		UserID:     47,
		Amount:     5,
		BatchID:    "FB-20260810-004133",
		GrantedAt:  now,
		User: &service.FeedbackUser{
			ID:       47,
			Email:    "user@example.com",
			Username: "tester",
		},
	}

	out := FeedbackRewardFromService(item)
	if out == nil || out.User == nil {
		t.Fatal("expected mapped reward user")
	}
	if out.User.ID != 47 || out.User.Email != "user@example.com" || out.User.Username != "tester" {
		t.Fatalf("unexpected mapped reward user: %+v", out.User)
	}
}

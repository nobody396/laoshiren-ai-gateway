package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type pendingAuthSessionRepoStub struct {
	session       *PendingAuthSession
	resolvedInput PendingAuthSessionResolveInput
}

func (r *pendingAuthSessionRepoStub) Create(context.Context, PendingAuthSessionCreateInput) (*PendingAuthSession, error) {
	return nil, nil
}

func (r *pendingAuthSessionRepoStub) GetByState(context.Context, string) (*PendingAuthSession, error) {
	return r.session, nil
}

func (r *pendingAuthSessionRepoStub) Resolve(_ context.Context, _ string, input PendingAuthSessionResolveInput) (*PendingAuthSession, error) {
	r.resolvedInput = input
	resolved := *r.session
	resolved.ProviderUserID = &input.ProviderUserID
	resolved.ClaimsSnapshot = input.ClaimsSnapshot
	return &resolved, nil
}

func (r *pendingAuthSessionRepoStub) MarkConsumed(context.Context, string, time.Time) error {
	return nil
}

func (r *pendingAuthSessionRepoStub) DeleteExpired(context.Context, time.Time) (int64, error) {
	return 0, nil
}

func TestResolvePendingSessionPreservesRegistrationContext(t *testing.T) {
	repo := &pendingAuthSessionRepoStub{
		session: &PendingAuthSession{
			State:          "state-1",
			Provider:       AuthProviderGoogle,
			IntendedAction: PendingAuthActionLogin,
			ClaimsSnapshot: map[string]any{
				"referral_code":   "AGENT123",
				"invitation_code": "INV456",
			},
			ExpiresAt: time.Now().Add(10 * time.Minute),
		},
	}
	svc := NewIdentityService(nil, nil, repo, nil)

	session, err := svc.ResolvePendingSession(context.Background(), "state-1", PendingAuthSessionResolveInput{
		ProviderUserID: "google-sub-1",
		ClaimsSnapshot: map[string]any{
			"email":    "user@example.com",
			"username": "user",
			"subject":  "google-sub-1",
		},
	})

	require.NoError(t, err)
	require.Equal(t, "AGENT123", session.ClaimsSnapshot["referral_code"])
	require.Equal(t, "INV456", session.ClaimsSnapshot["invitation_code"])
	require.Equal(t, "user@example.com", session.ClaimsSnapshot["email"])
	require.Equal(t, "google-sub-1", repo.resolvedInput.ClaimsSnapshot["subject"])
}

func TestAuthIdentityJSONUsesFrontendFieldNames(t *testing.T) {
	raw, err := json.Marshal(AuthIdentity{
		ID:             1,
		UserID:         2,
		Provider:       AuthProviderGoogle,
		ProviderUserID: "google-sub-1",
		Email:          "user@example.com",
		EmailVerified:  true,
		DisplayName:    "User",
		AvatarURL:      "https://example.com/avatar.png",
		RawProfile:     map[string]any{"sub": "google-sub-1"},
		BoundAt:        time.Date(2026, 5, 5, 1, 2, 3, 0, time.UTC),
		CreatedAt:      time.Date(2026, 5, 5, 1, 2, 3, 0, time.UTC),
		UpdatedAt:      time.Date(2026, 5, 5, 1, 2, 3, 0, time.UTC),
	})
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.Equal(t, "google", payload["provider"])
	require.Equal(t, "google-sub-1", payload["provider_user_id"])
	require.Equal(t, "user@example.com", payload["email"])
	require.Equal(t, true, payload["email_verified"])
	require.Equal(t, "User", payload["display_name"])
	require.NotContains(t, payload, "Provider")
	require.NotContains(t, payload, "ProviderUserID")
}

package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type oauthReferralSettingRepoStub struct {
	values map[string]string
}

func (s *oauthReferralSettingRepoStub) Get(context.Context, string) (*service.Setting, error) {
	panic("unexpected Get call")
}

func (s *oauthReferralSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if v, ok := s.values[key]; ok {
		return v, nil
	}
	return "", service.ErrSettingNotFound
}

func (s *oauthReferralSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *oauthReferralSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *oauthReferralSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *oauthReferralSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *oauthReferralSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type oauthReferralUserRepoStub struct {
	getByEmailCalls int
	getByEmailUser  *service.User
	getByEmailErr   error
	createdUsers    []*service.User
	nextUserID      int64
	inviteCodeUser  *service.User
	setInviterCalls []oauthReferralSetInviterCall
	touchCalls      int
}

type oauthReferralSetInviterCall struct {
	userID    int64
	inviterID int64
	agentID   *int64
}

func (r *oauthReferralUserRepoStub) Create(_ context.Context, user *service.User) error {
	if r.nextUserID == 0 {
		r.nextUserID = 100
	}
	user.ID = r.nextUserID
	r.nextUserID++
	r.createdUsers = append(r.createdUsers, user)
	return nil
}

func (r *oauthReferralUserRepoStub) GetByID(context.Context, int64) (*service.User, error) {
	panic("unexpected GetByID call")
}

func (r *oauthReferralUserRepoStub) GetByEmail(context.Context, string) (*service.User, error) {
	r.getByEmailCalls++
	if r.getByEmailErr != nil {
		return nil, r.getByEmailErr
	}
	if r.getByEmailUser != nil {
		return r.getByEmailUser, nil
	}
	return nil, service.ErrUserNotFound
}

func (r *oauthReferralUserRepoStub) GetFirstAdmin(context.Context) (*service.User, error) {
	panic("unexpected GetFirstAdmin call")
}

func (r *oauthReferralUserRepoStub) Update(context.Context, *service.User) error {
	return nil
}

func (r *oauthReferralUserRepoStub) Delete(context.Context, int64) error {
	panic("unexpected Delete call")
}

func (r *oauthReferralUserRepoStub) TouchLastActive(context.Context, int64, time.Time) error {
	r.touchCalls++
	return nil
}

func (r *oauthReferralUserRepoStub) List(context.Context, pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (r *oauthReferralUserRepoStub) ListWithFilters(context.Context, pagination.PaginationParams, service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (r *oauthReferralUserRepoStub) UpdateBalance(context.Context, int64, float64) error {
	panic("unexpected UpdateBalance call")
}

func (r *oauthReferralUserRepoStub) DeductBalance(context.Context, int64, float64) error {
	panic("unexpected DeductBalance call")
}

func (r *oauthReferralUserRepoStub) UpdateConcurrency(context.Context, int64, int) error {
	panic("unexpected UpdateConcurrency call")
}

func (r *oauthReferralUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	panic("unexpected ExistsByEmail call")
}

func (r *oauthReferralUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}

func (r *oauthReferralUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}

func (r *oauthReferralUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	panic("unexpected RemoveGroupFromUserAllowedGroups call")
}

func (r *oauthReferralUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	panic("unexpected UpdateTotpSecret call")
}

func (r *oauthReferralUserRepoStub) EnableTotp(context.Context, int64) error {
	panic("unexpected EnableTotp call")
}

func (r *oauthReferralUserRepoStub) DisableTotp(context.Context, int64) error {
	panic("unexpected DisableTotp call")
}

func (r *oauthReferralUserRepoStub) GetByInviteCode(context.Context, string) (*service.User, error) {
	if r.inviteCodeUser != nil {
		return r.inviteCodeUser, nil
	}
	return nil, service.ErrUserNotFound
}

func (r *oauthReferralUserRepoStub) SetInviteCode(context.Context, int64, string) error {
	panic("unexpected SetInviteCode call")
}

func (r *oauthReferralUserRepoStub) SetInviterAndAgent(_ context.Context, userID, inviterID int64, agentID *int64) error {
	r.setInviterCalls = append(r.setInviterCalls, oauthReferralSetInviterCall{
		userID:    userID,
		inviterID: inviterID,
		agentID:   agentID,
	})
	return nil
}

func (r *oauthReferralUserRepoStub) AdminBindUserToAgent(context.Context, int64, int64, bool) error {
	panic("unexpected AdminBindUserToAgent call")
}

func (r *oauthReferralUserRepoStub) SetFirstRecharged(context.Context, int64) (int, error) {
	panic("unexpected SetFirstRecharged call")
}

func (r *oauthReferralUserRepoStub) MarkFirstInvitedTopup(context.Context, int64, int64) (int, error) {
	panic("unexpected MarkFirstInvitedTopup call")
}

func (r *oauthReferralUserRepoStub) GetInviteCodeByUserID(context.Context, int64) (*string, error) {
	panic("unexpected GetInviteCodeByUserID call")
}

func (r *oauthReferralUserRepoStub) CountInvitedByInviterID(context.Context, int64) (int64, error) {
	panic("unexpected CountInvitedByInviterID call")
}

type oauthReferralIdentityRepoStub struct{}

func (r *oauthReferralIdentityRepoStub) ListByUserID(context.Context, int64) ([]*service.AuthIdentity, error) {
	panic("unexpected ListByUserID call")
}

func (r *oauthReferralIdentityRepoStub) GetByProviderAccount(context.Context, string, string) (*service.AuthIdentity, error) {
	return nil, service.ErrAuthIdentityNotFound
}

func (r *oauthReferralIdentityRepoStub) Upsert(context.Context, service.AuthIdentityUpsertInput) (*service.AuthIdentity, error) {
	return &service.AuthIdentity{}, nil
}

func (r *oauthReferralIdentityRepoStub) DeleteByUserProvider(context.Context, int64, string) error {
	panic("unexpected DeleteByUserProvider call")
}

type oauthReferralPendingSessionRepoStub struct {
	session     *service.PendingAuthSession
	consumed    bool
	consumedFor string
}

func (r *oauthReferralPendingSessionRepoStub) Create(context.Context, service.PendingAuthSessionCreateInput) (*service.PendingAuthSession, error) {
	panic("unexpected Create call")
}

func (r *oauthReferralPendingSessionRepoStub) GetByState(_ context.Context, state string) (*service.PendingAuthSession, error) {
	if r.session == nil || r.session.State != state {
		return nil, service.ErrPendingAuthSessionNotFound
	}
	return r.session, nil
}

func (r *oauthReferralPendingSessionRepoStub) Resolve(context.Context, string, service.PendingAuthSessionResolveInput) (*service.PendingAuthSession, error) {
	panic("unexpected Resolve call")
}

func (r *oauthReferralPendingSessionRepoStub) MarkConsumed(_ context.Context, state string, _ time.Time) error {
	r.consumed = true
	r.consumedFor = state
	return nil
}

func (r *oauthReferralPendingSessionRepoStub) DeleteExpired(context.Context, time.Time) (int64, error) {
	panic("unexpected DeleteExpired call")
}

type oauthReferralRefreshTokenCacheStub struct{}

func (c *oauthReferralRefreshTokenCacheStub) StoreRefreshToken(context.Context, string, *service.RefreshTokenData, time.Duration) error {
	return nil
}

func (c *oauthReferralRefreshTokenCacheStub) GetRefreshToken(context.Context, string) (*service.RefreshTokenData, error) {
	return nil, service.ErrRefreshTokenNotFound
}

func (c *oauthReferralRefreshTokenCacheStub) DeleteRefreshToken(context.Context, string) error {
	return nil
}

func (c *oauthReferralRefreshTokenCacheStub) ConsumeRefreshToken(context.Context, string) (*service.RefreshTokenData, error) {
	return nil, service.ErrRefreshTokenNotFound
}

func (c *oauthReferralRefreshTokenCacheStub) DeleteUserRefreshTokens(context.Context, int64) error {
	return nil
}

func (c *oauthReferralRefreshTokenCacheStub) DeleteTokenFamily(context.Context, string) error {
	return nil
}

func (c *oauthReferralRefreshTokenCacheStub) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}

func (c *oauthReferralRefreshTokenCacheStub) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}

func (c *oauthReferralRefreshTokenCacheStub) GetUserTokenHashes(context.Context, int64) ([]string, error) {
	return nil, nil
}

func (c *oauthReferralRefreshTokenCacheStub) GetFamilyTokenHashes(context.Context, string) ([]string, error) {
	return nil, nil
}

func (c *oauthReferralRefreshTokenCacheStub) IsTokenInFamily(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestExternalOAuthCallback_NewUserWithoutReferralRedirectsReferralOptional(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/callback", nil)

	cfg := &config.Config{
		JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1},
		Default: config.DefaultConfig{
			UserBalance:     1,
			UserConcurrency: 1,
		},
	}
	settingSvc := service.NewSettingService(&oauthReferralSettingRepoStub{values: map[string]string{
		service.SettingKeyRegistrationEnabled:   "true",
		service.SettingKeyInvitationCodeEnabled: "true",
	}}, cfg)
	userRepo := &oauthReferralUserRepoStub{}
	h := &AuthHandler{
		authService: service.NewAuthService(nil, userRepo, nil, nil, nil, cfg, settingSvc, nil, nil, nil, nil, nil, nil),
		identitySvc: service.NewIdentityService(nil, &oauthReferralIdentityRepoStub{}, nil, nil),
		settingSvc:  settingSvc,
	}

	h.completeExternalOAuthLogin(
		c,
		"/auth/google/callback",
		"/dashboard",
		&service.PendingAuthSession{
			State:          "state-1",
			Provider:       service.AuthProviderGoogle,
			IntendedAction: service.PendingAuthActionLogin,
			ExpiresAt:      time.Now().Add(10 * time.Minute),
		},
		service.AuthProviderGoogle,
		&externalOAuthIdentity{
			Email:         "new@example.com",
			Username:      "New User",
			Subject:       "google-sub-1",
			EmailVerified: true,
		},
	)

	require.Equal(t, http.StatusFound, w.Code)
	location := w.Header().Get("Location")
	parsed, err := url.Parse(location)
	require.NoError(t, err)
	fragment, err := url.ParseQuery(parsed.Fragment)
	require.NoError(t, err)
	require.Equal(t, "referral_optional", fragment.Get("error"))
	require.Equal(t, "state-1", fragment.Get("pending_oauth_token"))
	require.Equal(t, service.AuthProviderGoogle, fragment.Get("provider"))
	require.Equal(t, "/dashboard", fragment.Get("redirect"))
	require.Equal(t, "true", fragment.Get("invitation_required"))
	require.Equal(t, 1, userRepo.getByEmailCalls)
}

func TestExternalOAuthCompleteRegistration_BindsReferralCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := map[string]string{
		"pending_oauth_token": "state-2",
		"referral_code":       "AGENT123",
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/oauth/google/complete-registration", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "test-secret",
			ExpireHour:             1,
			RefreshTokenExpireDays: 7,
		},
		Default: config.DefaultConfig{
			UserBalance:     1,
			UserConcurrency: 1,
		},
	}
	settingSvc := service.NewSettingService(&oauthReferralSettingRepoStub{values: map[string]string{
		service.SettingKeyRegistrationEnabled:   "true",
		service.SettingKeyInvitationCodeEnabled: "false",
	}}, cfg)
	agentID := int64(42)
	userRepo := &oauthReferralUserRepoStub{
		inviteCodeUser: &service.User{
			ID:     agentID,
			Email:  "agent@example.com",
			Role:   service.RoleAgent,
			Status: service.StatusActive,
		},
	}
	pendingRepo := &oauthReferralPendingSessionRepoStub{session: &service.PendingAuthSession{
		State:          "state-2",
		Provider:       service.AuthProviderGoogle,
		IntendedAction: service.PendingAuthActionLogin,
		ClaimsSnapshot: map[string]any{
			"email":          "new@example.com",
			"username":       "New User",
			"subject":        "google-sub-2",
			"email_verified": true,
		},
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}}
	commissionSvc := service.NewCommissionService(userRepo, nil)
	h := &AuthHandler{
		authService: service.NewAuthService(
			nil,
			userRepo,
			nil,
			&oauthReferralRefreshTokenCacheStub{},
			nil,
			cfg,
			settingSvc,
			nil,
			nil,
			nil,
			nil,
			nil,
			commissionSvc,
		),
		identitySvc: service.NewIdentityService(nil, &oauthReferralIdentityRepoStub{}, pendingRepo, nil),
		settingSvc:  settingSvc,
	}

	h.CompleteGoogleOAuthRegistration(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, userRepo.createdUsers, 1)
	require.Len(t, userRepo.setInviterCalls, 1)
	require.Equal(t, userRepo.createdUsers[0].ID, userRepo.setInviterCalls[0].userID)
	require.Equal(t, agentID, userRepo.setInviterCalls[0].inviterID)
	require.NotNil(t, userRepo.setInviterCalls[0].agentID)
	require.Equal(t, agentID, *userRepo.setInviterCalls[0].agentID)
	require.True(t, pendingRepo.consumed)
	require.Equal(t, "state-2", pendingRepo.consumedFor)
}

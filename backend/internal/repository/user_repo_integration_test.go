//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/suite"
)

type UserRepoSuite struct {
	suite.Suite
	ctx    context.Context
	client *dbent.Client
	repo   *userRepository
}

func (s *UserRepoSuite) SetupTest() {
	s.ctx = context.Background()
	s.client = testEntClient(s.T())
	s.repo = newUserRepositoryWithSQL(s.client, integrationDB)

	// Tests in this package intentionally exercise committed, cross-transaction
	// paths. Reset the user aggregate explicitly instead of silently ignoring
	// foreign-key cleanup failures from earlier integration fixtures.
	_, err := integrationDB.ExecContext(s.ctx, "TRUNCATE TABLE users CASCADE")
	s.Require().NoError(err, "reset user integration fixtures")
	_, err = integrationDB.ExecContext(s.ctx, `
		INSERT INTO affiliate_program_settings (id)
		VALUES (1)
		ON CONFLICT (id) DO NOTHING
	`)
	s.Require().NoError(err, "restore affiliate singleton after cascade reset")
}

func (s *UserRepoSuite) TestCreateAssignsInviteCodeAndPersistsOrdinaryBinding() {
	inviter := s.mustCreateUser(&service.User{})
	invitee := s.mustCreateUser(&service.User{})
	s.Require().NotNil(inviter.InviteCode)
	s.Require().NotEmpty(*inviter.InviteCode)
	s.Require().NotNil(invitee.InviteCode)
	s.Require().NotEqual(*inviter.InviteCode, *invitee.InviteCode)

	s.Require().NoError(s.repo.SetInviterAndAgent(s.ctx, invitee.ID, inviter.ID, nil))

	var bindingKind string
	var boundInviterID int64
	s.Require().NoError(integrationDB.QueryRowContext(s.ctx, `
		SELECT binding_kind, inviter_user_id
		FROM affiliate_bindings
		WHERE customer_user_id = $1
	`, invitee.ID).Scan(&bindingKind, &boundInviterID))
	s.Equal(service.AffiliateBindingOrdinary, bindingKind)
	s.Equal(inviter.ID, boundInviterID)
}

func TestUserRepoSuite(t *testing.T) {
	suite.Run(t, new(UserRepoSuite))
}

func (s *UserRepoSuite) mustCreateUser(u *service.User) *service.User {
	s.T().Helper()

	if u.Email == "" {
		u.Email = "user-" + time.Now().Format(time.RFC3339Nano) + "@example.com"
	}
	if u.PasswordHash == "" {
		u.PasswordHash = "test-password-hash"
	}
	if u.Role == "" {
		u.Role = service.RoleUser
	}
	if u.Status == "" {
		u.Status = service.StatusActive
	}
	if u.Concurrency == 0 {
		u.Concurrency = 5
	}

	s.Require().NoError(s.repo.Create(s.ctx, u), "create user")
	return u
}

func (s *UserRepoSuite) mustCreateGroup(name string) *service.Group {
	s.T().Helper()

	g, err := s.client.Group.Create().
		SetName(name).
		SetStatus(service.StatusActive).
		Save(s.ctx)
	s.Require().NoError(err, "create group")
	return groupEntityToService(g)
}

func (s *UserRepoSuite) mustCreateSubscription(userID, groupID int64, mutate func(*dbent.UserSubscriptionCreate)) *dbent.UserSubscription {
	s.T().Helper()

	now := time.Now()
	create := s.client.UserSubscription.Create().
		SetUserID(userID).
		SetGroupID(groupID).
		SetStartsAt(now.Add(-1 * time.Hour)).
		SetExpiresAt(now.Add(24 * time.Hour)).
		SetStatus(service.SubscriptionStatusActive).
		SetAssignedAt(now).
		SetNotes("")

	if mutate != nil {
		mutate(create)
	}

	sub, err := create.Save(s.ctx)
	s.Require().NoError(err, "create subscription")
	return sub
}

func (s *UserRepoSuite) mustCreateUserWithPassword(email, password string) *service.User {
	s.T().Helper()

	user := &service.User{
		Email:       email,
		Role:        service.RoleUser,
		Status:      service.StatusActive,
		Balance:     1,
		Concurrency: 5,
	}
	s.Require().NoError(user.SetPassword(password), "hash password")
	return s.mustCreateUser(user)
}

func (s *UserRepoSuite) newAuthRevocationServices() (*service.AuthService, *service.UserService) {
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "test-jwt-secret-32bytes-long!!!",
			ExpireHour:             1,
			RefreshTokenExpireDays: 30,
		},
	}

	authSvc := service.NewAuthService(
		nil,
		s.repo,
		nil,
		newRepositoryMemoryRefreshTokenCache(),
		nil,
		cfg,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	userSvc := service.NewUserService(s.repo, nil, nil)
	return authSvc, userSvc
}

type repositoryMemoryRefreshTokenCache struct {
	data map[string]*service.RefreshTokenData
}

func newRepositoryMemoryRefreshTokenCache() *repositoryMemoryRefreshTokenCache {
	return &repositoryMemoryRefreshTokenCache{data: map[string]*service.RefreshTokenData{}}
}

func (c *repositoryMemoryRefreshTokenCache) StoreRefreshToken(_ context.Context, tokenHash string, data *service.RefreshTokenData, _ time.Duration) error {
	c.data[tokenHash] = data
	return nil
}

func (c *repositoryMemoryRefreshTokenCache) GetRefreshToken(_ context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	data, ok := c.data[tokenHash]
	if !ok {
		return nil, service.ErrRefreshTokenNotFound
	}
	return data, nil
}

func (c *repositoryMemoryRefreshTokenCache) DeleteRefreshToken(_ context.Context, tokenHash string) error {
	delete(c.data, tokenHash)
	return nil
}

func (c *repositoryMemoryRefreshTokenCache) ConsumeRefreshToken(_ context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	data, ok := c.data[tokenHash]
	if !ok {
		return nil, service.ErrRefreshTokenNotFound
	}
	delete(c.data, tokenHash)
	return data, nil
}

func (c *repositoryMemoryRefreshTokenCache) DeleteUserRefreshTokens(_ context.Context, userID int64) error {
	for tokenHash, data := range c.data {
		if data.UserID == userID {
			delete(c.data, tokenHash)
		}
	}
	return nil
}

func (c *repositoryMemoryRefreshTokenCache) DeleteTokenFamily(_ context.Context, familyID string) error {
	for tokenHash, data := range c.data {
		if data.FamilyID == familyID {
			delete(c.data, tokenHash)
		}
	}
	return nil
}

func (c *repositoryMemoryRefreshTokenCache) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}

func (c *repositoryMemoryRefreshTokenCache) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}

func (c *repositoryMemoryRefreshTokenCache) GetUserTokenHashes(context.Context, int64) ([]string, error) {
	return nil, nil
}

func (c *repositoryMemoryRefreshTokenCache) GetFamilyTokenHashes(context.Context, string) ([]string, error) {
	return nil, nil
}

func (c *repositoryMemoryRefreshTokenCache) IsTokenInFamily(context.Context, string, string) (bool, error) {
	return false, nil
}

// --- Create / GetByID / GetByEmail / Update / Delete ---

func (s *UserRepoSuite) TestCreate() {
	user := s.mustCreateUser(&service.User{
		Email:        "create@test.com",
		Username:     "testuser",
		PasswordHash: "test-password-hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	s.Require().NotZero(user.ID, "expected ID to be set")

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err, "GetByID")
	s.Require().Equal("create@test.com", got.Email)
}

func (s *UserRepoSuite) TestGetByID_NotFound() {
	_, err := s.repo.GetByID(s.ctx, 999999)
	s.Require().Error(err, "expected error for non-existent ID")
}

func (s *UserRepoSuite) TestGetByEmail() {
	user := s.mustCreateUser(&service.User{Email: "byemail@test.com"})

	got, err := s.repo.GetByEmail(s.ctx, user.Email)
	s.Require().NoError(err, "GetByEmail")
	s.Require().Equal(user.ID, got.ID)
}

func (s *UserRepoSuite) TestGetByEmail_NotFound() {
	_, err := s.repo.GetByEmail(s.ctx, "nonexistent@test.com")
	s.Require().Error(err, "expected error for non-existent email")
}

func (s *UserRepoSuite) TestUpdate() {
	user := s.mustCreateUser(&service.User{Email: "update@test.com", Username: "original"})

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	got.Username = "updated"
	s.Require().NoError(s.repo.Update(s.ctx, got), "Update")

	updated, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err, "GetByID after update")
	s.Require().Equal("updated", updated.Username)
}

func (s *UserRepoSuite) TestChangePasswordPersistsTokenVersionAndRejectsOldAccessToken() {
	user := s.mustCreateUserWithPassword("access-revoked@test.com", "old-password")
	authSvc, userSvc := s.newAuthRevocationServices()

	oldAccessToken, loggedInUser, err := authSvc.Login(s.ctx, user.Email, "old-password")
	s.Require().NoError(err, "login before password change")

	err = userSvc.ChangePassword(s.ctx, user.ID, service.ChangePasswordRequest{
		CurrentPassword: "old-password",
		NewPassword:     "new-password",
	})
	s.Require().NoError(err, "change password")

	reloaded, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err, "reload user after password change")
	s.Require().Greater(reloaded.TokenVersion, loggedInUser.TokenVersion, "password change must persist a higher token version")

	_, err = authSvc.RefreshToken(s.ctx, oldAccessToken)
	s.Require().ErrorIs(err, service.ErrTokenRevoked, "old access token must be rejected after password change")
}

func (s *UserRepoSuite) TestChangePasswordRejectsOldRefreshToken() {
	user := s.mustCreateUserWithPassword("refresh-revoked@test.com", "old-password")
	authSvc, userSvc := s.newAuthRevocationServices()

	_, loggedInUser, err := authSvc.Login(s.ctx, user.Email, "old-password")
	s.Require().NoError(err, "login before password change")
	pair, err := authSvc.GenerateTokenPair(s.ctx, loggedInUser, "")
	s.Require().NoError(err, "generate token pair before password change")

	err = userSvc.ChangePassword(s.ctx, user.ID, service.ChangePasswordRequest{
		CurrentPassword: "old-password",
		NewPassword:     "new-password",
	})
	s.Require().NoError(err, "change password")

	_, err = authSvc.RefreshTokenPair(s.ctx, pair.RefreshToken)
	s.Require().ErrorIs(err, service.ErrTokenRevoked, "old refresh token must be rejected after password change")
}

func (s *UserRepoSuite) TestRevokeAllUserSessionsRejectsOldAccessToken() {
	user := s.mustCreateUserWithPassword("revoke-all@test.com", "password")
	authSvc, _ := s.newAuthRevocationServices()

	oldAccessToken, loggedInUser, err := authSvc.Login(s.ctx, user.Email, "password")
	s.Require().NoError(err, "login before revoke-all")

	s.Require().NoError(authSvc.RevokeAllUserSessions(s.ctx, user.ID), "revoke all sessions")

	reloaded, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err, "reload user after revoke-all")
	s.Require().Greater(reloaded.TokenVersion, loggedInUser.TokenVersion, "revoke-all must persist a higher token version")

	_, err = authSvc.RefreshToken(s.ctx, oldAccessToken)
	s.Require().ErrorIs(err, service.ErrTokenRevoked, "old access token must be rejected after revoke-all")
}

func (s *UserRepoSuite) TestDelete() {
	user := s.mustCreateUser(&service.User{Email: "delete@test.com"})

	err := s.repo.Delete(s.ctx, user.ID)
	s.Require().NoError(err, "Delete")

	_, err = s.repo.GetByID(s.ctx, user.ID)
	s.Require().Error(err, "expected error after delete")
}

// --- List / ListWithFilters ---

func (s *UserRepoSuite) TestList() {
	s.mustCreateUser(&service.User{Email: "list1@test.com"})
	s.mustCreateUser(&service.User{Email: "list2@test.com"})

	users, page, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err, "List")
	s.Require().Len(users, 2)
	s.Require().Equal(int64(2), page.Total)
}

func (s *UserRepoSuite) TestListWithFilters_Status() {
	s.mustCreateUser(&service.User{Email: "active@test.com", Status: service.StatusActive})
	s.mustCreateUser(&service.User{Email: "disabled@test.com", Status: service.StatusDisabled})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, service.UserListFilters{Status: service.StatusActive})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Require().Equal(service.StatusActive, users[0].Status)
}

func (s *UserRepoSuite) TestListWithFilters_Role() {
	s.mustCreateUser(&service.User{Email: "user@test.com", Role: service.RoleUser})
	s.mustCreateUser(&service.User{Email: "admin@test.com", Role: service.RoleAdmin})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, service.UserListFilters{Role: service.RoleAdmin})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Require().Equal(service.RoleAdmin, users[0].Role)
}

func (s *UserRepoSuite) TestListWithFilters_Search() {
	s.mustCreateUser(&service.User{Email: "alice@test.com", Username: "Alice"})
	s.mustCreateUser(&service.User{Email: "bob@test.com", Username: "Bob"})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, service.UserListFilters{Search: "alice"})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Require().Contains(users[0].Email, "alice")
}

func (s *UserRepoSuite) TestListWithFilters_SearchByUsername() {
	s.mustCreateUser(&service.User{Email: "u1@test.com", Username: "JohnDoe"})
	s.mustCreateUser(&service.User{Email: "u2@test.com", Username: "JaneSmith"})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, service.UserListFilters{Search: "john"})
	s.Require().NoError(err)
	s.Require().Len(users, 1)
	s.Require().Equal("JohnDoe", users[0].Username)
}

func (s *UserRepoSuite) TestListWithFilters_LoadsActiveSubscriptions() {
	user := s.mustCreateUser(&service.User{Email: "sub@test.com", Status: service.StatusActive})
	groupActive := s.mustCreateGroup("g-sub-active")
	groupExpired := s.mustCreateGroup("g-sub-expired")

	_ = s.mustCreateSubscription(user.ID, groupActive.ID, func(c *dbent.UserSubscriptionCreate) {
		c.SetStatus(service.SubscriptionStatusActive)
		c.SetExpiresAt(time.Now().Add(1 * time.Hour))
	})
	_ = s.mustCreateSubscription(user.ID, groupExpired.ID, func(c *dbent.UserSubscriptionCreate) {
		c.SetStatus(service.SubscriptionStatusExpired)
		c.SetExpiresAt(time.Now().Add(-1 * time.Hour))
	})

	users, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, service.UserListFilters{Search: "sub@"})
	s.Require().NoError(err, "ListWithFilters")
	s.Require().Len(users, 1, "expected 1 user")
	s.Require().Len(users[0].Subscriptions, 1, "expected 1 active subscription")
	s.Require().NotNil(users[0].Subscriptions[0].Group, "expected subscription group preload")
	s.Require().Equal(groupActive.ID, users[0].Subscriptions[0].Group.ID, "group ID mismatch")
}

func (s *UserRepoSuite) TestListWithFilters_CombinedFilters() {
	s.mustCreateUser(&service.User{
		Email:    "a@example.com",
		Username: "Alice",
		Role:     service.RoleUser,
		Status:   service.StatusActive,
		Balance:  10,
	})
	target := s.mustCreateUser(&service.User{
		Email:    "b@example.com",
		Username: "Bob",
		Role:     service.RoleAdmin,
		Status:   service.StatusActive,
		Balance:  1,
	})
	s.mustCreateUser(&service.User{
		Email:  "c@example.com",
		Role:   service.RoleAdmin,
		Status: service.StatusDisabled,
	})

	users, page, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, service.UserListFilters{Status: service.StatusActive, Role: service.RoleAdmin, Search: "b@"})
	s.Require().NoError(err, "ListWithFilters")
	s.Require().Equal(int64(1), page.Total, "ListWithFilters total mismatch")
	s.Require().Len(users, 1, "ListWithFilters len mismatch")
	s.Require().Equal(target.ID, users[0].ID, "ListWithFilters result mismatch")
}

// --- Balance operations ---

func (s *UserRepoSuite) TestUpdateBalance() {
	user := s.mustCreateUser(&service.User{Email: "bal@test.com", Balance: 10})

	err := s.repo.UpdateBalance(s.ctx, user.ID, 2.5)
	s.Require().NoError(err, "UpdateBalance")

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().InDelta(12.5, got.Balance, 1e-6)
}

func (s *UserRepoSuite) TestUpdateBalance_Negative() {
	user := s.mustCreateUser(&service.User{Email: "balneg@test.com", Balance: 10})

	err := s.repo.UpdateBalance(s.ctx, user.ID, -3)
	s.Require().NoError(err, "UpdateBalance with negative")

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().InDelta(7.0, got.Balance, 1e-6)
}

func (s *UserRepoSuite) TestApplyAdminBalanceAdjustment_BlocksNegativeChangeWithEligibleLot() {
	user := s.mustCreateUser(&service.User{Email: "admin-guard-user@test.com", Balance: 10})
	partner := s.mustCreateUser(&service.User{Email: "admin-guard-partner@test.com"})
	_, err := integrationDB.ExecContext(s.ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, affiliate_policy, direct_partner_id,
			customer_rebate_rate_bps, partner_commission_rate_bps
		)
		VALUES ($1, 'paid_topup', $2, 10000000, 10000000, TRUE, 'PARTNER_USAGE', $3, 500, 500)
	`, user.ID, fmt.Sprintf("test:admin-guard:%d", user.ID), partner.ID)
	s.Require().NoError(err)

	_, err = s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 1, "subtract")
	s.Require().ErrorIs(err, service.ErrAdminBalanceSourceReversalRequired)

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().InDelta(10, got.Balance, 1e-6)
}

func (s *UserRepoSuite) TestApplyAdminBalanceAdjustment_AddAndNoopNeverCreateEligibleLot() {
	user := s.mustCreateUser(&service.User{Email: "admin-add-ineligible@test.com", Balance: 10})

	result, err := s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 2, "add")
	s.Require().NoError(err)
	s.Require().InDelta(10, result.OldBalance, 1e-6)
	s.Require().InDelta(12, result.NewBalance, 1e-6)

	result, err = s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 12, "set")
	s.Require().NoError(err)
	s.Require().InDelta(12, result.OldBalance, 1e-6)
	s.Require().InDelta(12, result.NewBalance, 1e-6)

	var (
		sourceType        string
		affiliateEligible bool
		affiliatePolicy   string
		remainingMicros   int64
		lotCount          int
	)
	s.Require().NoError(integrationDB.QueryRowContext(s.ctx, `
		SELECT source_type, affiliate_eligible, affiliate_policy, remaining_amount_micros
		FROM balance_lots
		WHERE user_id = $1
	`, user.ID).Scan(&sourceType, &affiliateEligible, &affiliatePolicy, &remainingMicros))
	s.Equal(service.AffiliateSourceAdminAdjustment, sourceType)
	s.False(affiliateEligible, "manual positive balance must not inherit an affiliate policy")
	s.Equal(service.AffiliateSourcePolicyNone, affiliatePolicy)
	s.Equal(int64(2_000_000), remainingMicros)
	s.Require().NoError(integrationDB.QueryRowContext(s.ctx, `
		SELECT COUNT(*)
		FROM balance_lots
		WHERE user_id = $1
	`, user.ID).Scan(&lotCount))
	s.Equal(1, lotCount, "a no-op must not create another balance lot")
}

func (s *UserRepoSuite) TestApplyAdminBalanceAdjustment_SubtractWithoutEligibleLot() {
	user := s.mustCreateUser(&service.User{Email: "admin-subtract-safe@test.com"})

	_, err := s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 10, "add")
	s.Require().NoError(err)

	result, err := s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 2, "subtract")
	s.Require().NoError(err)
	s.Require().InDelta(10, result.OldBalance, 1e-6)
	s.Require().InDelta(8, result.NewBalance, 1e-6)

	var remainingMicros int64
	s.Require().NoError(integrationDB.QueryRowContext(s.ctx, `
		SELECT remaining_amount_micros
		FROM balance_lots
		WHERE user_id = $1
	`, user.ID).Scan(&remainingMicros))
	s.Equal(int64(8_000_000), remainingMicros)
}

func (s *UserRepoSuite) TestApplyAdminBalanceAdjustment_SetAndSubtractConsumeMultipleLotsFIFO() {
	user := s.mustCreateUser(&service.User{Email: "admin-multi-lot@test.com"})

	_, err := s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 4, "add")
	s.Require().NoError(err)
	_, err = s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 6, "add")
	s.Require().NoError(err)

	result, err := s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 5, "set")
	s.Require().NoError(err)
	s.Require().InDelta(10, result.OldBalance, 1e-6)
	s.Require().InDelta(5, result.NewBalance, 1e-6)

	rows, err := integrationDB.QueryContext(s.ctx, `
		SELECT remaining_amount_micros
		FROM balance_lots
		WHERE user_id = $1
		ORDER BY occurred_at, id
	`, user.ID)
	s.Require().NoError(err)
	defer func() { _ = rows.Close() }()
	var remaining []int64
	for rows.Next() {
		var amount int64
		s.Require().NoError(rows.Scan(&amount))
		remaining = append(remaining, amount)
	}
	s.Require().NoError(rows.Err())
	s.Equal([]int64{0, 5_000_000}, remaining)

	_, err = s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 2, "add")
	s.Require().NoError(err)
	result, err = s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 6, "subtract")
	s.Require().NoError(err)
	s.Require().InDelta(7, result.OldBalance, 1e-6)
	s.Require().InDelta(1, result.NewBalance, 1e-6)

	var totalRemaining int64
	s.Require().NoError(integrationDB.QueryRowContext(s.ctx, `
		SELECT COALESCE(SUM(remaining_amount_micros), 0)
		FROM balance_lots
		WHERE user_id = $1
	`, user.ID).Scan(&totalRemaining))
	s.Equal(int64(1_000_000), totalRemaining)
}

func (s *UserRepoSuite) TestApplyAdminBalanceAdjustment_InsufficientAttributedLotsFailsClosed() {
	user := s.mustCreateUser(&service.User{Email: "admin-insufficient-lots@test.com", Balance: 10})

	_, err := s.repo.ApplyAdminBalanceAdjustment(s.ctx, user.ID, 1, "subtract")
	s.Require().ErrorIs(err, service.ErrAdminBalanceSourceReversalRequired)

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().InDelta(10, got.Balance, 1e-6)
}

func (s *UserRepoSuite) TestApplyAdminBalanceAdjustment_WaitsForConcurrentEligiblePurchase() {
	user := s.mustCreateUser(&service.User{Email: "admin-guard-race-user@test.com", Balance: 10})
	partner := s.mustCreateUser(&service.User{Email: "admin-guard-race-partner@test.com"})

	purchaseTx, err := integrationDB.BeginTx(s.ctx, nil)
	s.Require().NoError(err)
	defer func() { _ = purchaseTx.Rollback() }()
	_, err = purchaseTx.ExecContext(s.ctx, `
		UPDATE users
		SET balance = balance + 5
		WHERE id = $1
	`, user.ID)
	s.Require().NoError(err)
	_, err = purchaseTx.ExecContext(s.ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, affiliate_policy, direct_partner_id,
			customer_rebate_rate_bps, partner_commission_rate_bps
		)
		VALUES ($1, 'paid_topup', $2, 5000000, 5000000, TRUE, 'PARTNER_USAGE', $3, 500, 500)
	`, user.ID, fmt.Sprintf("test:admin-guard-race:%d", user.ID), partner.ID)
	s.Require().NoError(err)

	errCh := make(chan error, 1)
	go func() {
		_, adjustErr := s.repo.ApplyAdminBalanceAdjustment(context.Background(), user.ID, 1, "subtract")
		errCh <- adjustErr
	}()

	select {
	case err := <-errCh:
		s.Fail("negative adjustment did not wait for the purchase transaction", "error: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	s.Require().NoError(purchaseTx.Commit())
	err = <-errCh
	s.Require().True(errors.Is(err, service.ErrAdminBalanceSourceReversalRequired), "unexpected error: %v", err)

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().InDelta(15, got.Balance, 1e-6)
}

func (s *UserRepoSuite) TestDeductBalance() {
	user := s.mustCreateUser(&service.User{Email: "deduct@test.com", Balance: 10})

	err := s.repo.DeductBalance(s.ctx, user.ID, 5)
	s.Require().NoError(err, "DeductBalance")

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().InDelta(5.0, got.Balance, 1e-6)
}

func (s *UserRepoSuite) TestDeductBalance_InsufficientFunds() {
	user := s.mustCreateUser(&service.User{Email: "insuf@test.com", Balance: 5})

	// 透支策略：允许扣除超过余额的金额
	err := s.repo.DeductBalance(s.ctx, user.ID, 999)
	s.Require().NoError(err, "DeductBalance should allow overdraft")

	// 验证余额变为负数
	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().InDelta(-994.0, got.Balance, 1e-6, "Balance should be negative after overdraft")
}

func (s *UserRepoSuite) TestDeductBalance_ExactAmount() {
	user := s.mustCreateUser(&service.User{Email: "exact@test.com", Balance: 10})

	err := s.repo.DeductBalance(s.ctx, user.ID, 10)
	s.Require().NoError(err, "DeductBalance exact amount")

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().InDelta(0.0, got.Balance, 1e-6)
}

func (s *UserRepoSuite) TestDeductBalance_AllowsOverdraft() {
	user := s.mustCreateUser(&service.User{Email: "overdraft@test.com", Balance: 5.0})

	// 扣除超过余额的金额 - 应该成功
	err := s.repo.DeductBalance(s.ctx, user.ID, 10.0)
	s.Require().NoError(err, "DeductBalance should allow overdraft")

	// 验证余额为负
	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().InDelta(-5.0, got.Balance, 1e-6, "Balance should be -5.0 after overdraft")
}

// --- Concurrency ---

func (s *UserRepoSuite) TestUpdateConcurrency() {
	user := s.mustCreateUser(&service.User{Email: "conc@test.com", Concurrency: 5})

	err := s.repo.UpdateConcurrency(s.ctx, user.ID, 3)
	s.Require().NoError(err, "UpdateConcurrency")

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().Equal(8, got.Concurrency)
}

func (s *UserRepoSuite) TestUpdateConcurrency_Negative() {
	user := s.mustCreateUser(&service.User{Email: "concneg@test.com", Concurrency: 5})

	err := s.repo.UpdateConcurrency(s.ctx, user.ID, -2)
	s.Require().NoError(err, "UpdateConcurrency negative")

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().Equal(3, got.Concurrency)
}

// --- ExistsByEmail ---

func (s *UserRepoSuite) TestExistsByEmail() {
	s.mustCreateUser(&service.User{Email: "exists@test.com"})

	exists, err := s.repo.ExistsByEmail(s.ctx, "exists@test.com")
	s.Require().NoError(err, "ExistsByEmail")
	s.Require().True(exists)

	notExists, err := s.repo.ExistsByEmail(s.ctx, "notexists@test.com")
	s.Require().NoError(err)
	s.Require().False(notExists)
}

// --- RemoveGroupFromAllowedGroups ---

func (s *UserRepoSuite) TestRemoveGroupFromAllowedGroups() {
	target := s.mustCreateGroup("target-42")
	other := s.mustCreateGroup("other-7")

	userA := s.mustCreateUser(&service.User{
		Email:         "a1@example.com",
		AllowedGroups: []int64{target.ID, other.ID},
	})
	s.mustCreateUser(&service.User{
		Email:         "a2@example.com",
		AllowedGroups: []int64{other.ID},
	})

	affected, err := s.repo.RemoveGroupFromAllowedGroups(s.ctx, target.ID)
	s.Require().NoError(err, "RemoveGroupFromAllowedGroups")
	s.Require().Equal(int64(1), affected, "expected 1 affected row")

	got, err := s.repo.GetByID(s.ctx, userA.ID)
	s.Require().NoError(err, "GetByID")
	s.Require().NotContains(got.AllowedGroups, target.ID)
	s.Require().Contains(got.AllowedGroups, other.ID)
}

func (s *UserRepoSuite) TestRemoveGroupFromAllowedGroups_NoMatch() {
	groupA := s.mustCreateGroup("nomatch-a")
	groupB := s.mustCreateGroup("nomatch-b")

	s.mustCreateUser(&service.User{
		Email:         "nomatch@test.com",
		AllowedGroups: []int64{groupA.ID, groupB.ID},
	})

	affected, err := s.repo.RemoveGroupFromAllowedGroups(s.ctx, 999999)
	s.Require().NoError(err)
	s.Require().Zero(affected, "expected no affected rows")
}

// --- GetFirstAdmin ---

func (s *UserRepoSuite) TestGetFirstAdmin() {
	admin1 := s.mustCreateUser(&service.User{
		Email:  "admin1@example.com",
		Role:   service.RoleAdmin,
		Status: service.StatusActive,
	})
	s.mustCreateUser(&service.User{
		Email:  "admin2@example.com",
		Role:   service.RoleAdmin,
		Status: service.StatusActive,
	})

	got, err := s.repo.GetFirstAdmin(s.ctx)
	s.Require().NoError(err, "GetFirstAdmin")
	s.Require().Equal(admin1.ID, got.ID, "GetFirstAdmin mismatch")
}

func (s *UserRepoSuite) TestGetFirstAdmin_NoAdmin() {
	s.mustCreateUser(&service.User{
		Email:  "user@example.com",
		Role:   service.RoleUser,
		Status: service.StatusActive,
	})

	_, err := s.repo.GetFirstAdmin(s.ctx)
	s.Require().Error(err, "expected error when no admin exists")
}

func (s *UserRepoSuite) TestGetFirstAdmin_DisabledAdminIgnored() {
	s.mustCreateUser(&service.User{
		Email:  "disabled@example.com",
		Role:   service.RoleAdmin,
		Status: service.StatusDisabled,
	})
	activeAdmin := s.mustCreateUser(&service.User{
		Email:  "active@example.com",
		Role:   service.RoleAdmin,
		Status: service.StatusActive,
	})

	got, err := s.repo.GetFirstAdmin(s.ctx)
	s.Require().NoError(err, "GetFirstAdmin")
	s.Require().Equal(activeAdmin.ID, got.ID, "should return only active admin")
}

// --- Combined ---

func (s *UserRepoSuite) TestCRUD_And_Filters_And_AtomicUpdates() {
	user1 := s.mustCreateUser(&service.User{
		Email:    "a@example.com",
		Username: "Alice",
		Role:     service.RoleUser,
		Status:   service.StatusActive,
		Balance:  10,
	})
	user2 := s.mustCreateUser(&service.User{
		Email:    "b@example.com",
		Username: "Bob",
		Role:     service.RoleAdmin,
		Status:   service.StatusActive,
		Balance:  1,
	})
	s.mustCreateUser(&service.User{
		Email:  "c@example.com",
		Role:   service.RoleAdmin,
		Status: service.StatusDisabled,
	})

	got, err := s.repo.GetByID(s.ctx, user1.ID)
	s.Require().NoError(err, "GetByID")
	s.Require().Equal(user1.Email, got.Email, "GetByID email mismatch")

	gotByEmail, err := s.repo.GetByEmail(s.ctx, user2.Email)
	s.Require().NoError(err, "GetByEmail")
	s.Require().Equal(user2.ID, gotByEmail.ID, "GetByEmail ID mismatch")

	got.Username = "Alice2"
	s.Require().NoError(s.repo.Update(s.ctx, got), "Update")
	got2, err := s.repo.GetByID(s.ctx, user1.ID)
	s.Require().NoError(err, "GetByID after update")
	s.Require().Equal("Alice2", got2.Username, "Update did not persist")

	s.Require().NoError(s.repo.UpdateBalance(s.ctx, user1.ID, 2.5), "UpdateBalance")
	got3, err := s.repo.GetByID(s.ctx, user1.ID)
	s.Require().NoError(err, "GetByID after UpdateBalance")
	s.Require().InDelta(12.5, got3.Balance, 1e-6)

	s.Require().NoError(s.repo.DeductBalance(s.ctx, user1.ID, 5), "DeductBalance")
	got4, err := s.repo.GetByID(s.ctx, user1.ID)
	s.Require().NoError(err, "GetByID after DeductBalance")
	s.Require().InDelta(7.5, got4.Balance, 1e-6)

	// 透支策略：允许扣除超过余额的金额
	err = s.repo.DeductBalance(s.ctx, user1.ID, 999)
	s.Require().NoError(err, "DeductBalance should allow overdraft")
	gotOverdraft, err := s.repo.GetByID(s.ctx, user1.ID)
	s.Require().NoError(err, "GetByID after overdraft")
	s.Require().Less(gotOverdraft.Balance, 0.0, "Balance should be negative after overdraft")

	s.Require().NoError(s.repo.UpdateConcurrency(s.ctx, user1.ID, 3), "UpdateConcurrency")
	got5, err := s.repo.GetByID(s.ctx, user1.ID)
	s.Require().NoError(err, "GetByID after UpdateConcurrency")
	s.Require().Equal(user1.Concurrency+3, got5.Concurrency)

	params := pagination.PaginationParams{Page: 1, PageSize: 10}
	users, page, err := s.repo.ListWithFilters(s.ctx, params, service.UserListFilters{Status: service.StatusActive, Role: service.RoleAdmin, Search: "b@"})
	s.Require().NoError(err, "ListWithFilters")
	s.Require().Equal(int64(1), page.Total, "ListWithFilters total mismatch")
	s.Require().Len(users, 1, "ListWithFilters len mismatch")
	s.Require().Equal(user2.ID, users[0].ID, "ListWithFilters result mismatch")
}

// --- UpdateBalance/UpdateConcurrency 影响行数校验测试 ---

func (s *UserRepoSuite) TestUpdateBalance_NotFound() {
	err := s.repo.UpdateBalance(s.ctx, 999999, 10.0)
	s.Require().Error(err, "expected error for non-existent user")
	s.Require().ErrorIs(err, service.ErrUserNotFound)
}

func (s *UserRepoSuite) TestUpdateConcurrency_NotFound() {
	err := s.repo.UpdateConcurrency(s.ctx, 999999, 5)
	s.Require().Error(err, "expected error for non-existent user")
	s.Require().ErrorIs(err, service.ErrUserNotFound)
}

func (s *UserRepoSuite) TestDeductBalance_NotFound() {
	err := s.repo.DeductBalance(s.ctx, 999999, 5)
	s.Require().Error(err, "expected error for non-existent user")
	// DeductBalance 在用户不存在时返回 ErrUserNotFound
	s.Require().ErrorIs(err, service.ErrUserNotFound)
}

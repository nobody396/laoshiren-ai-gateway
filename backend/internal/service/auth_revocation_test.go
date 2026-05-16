//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type authRevocationUserRepo struct {
	UserRepository
	user *User
}

func newAuthRevocationUserRepo(t *testing.T, email, password string) *authRevocationUserRepo {
	t.Helper()

	user := &User{
		ID:          42,
		Email:       email,
		Role:        RoleUser,
		Status:      StatusActive,
		Concurrency: 5,
	}
	require.NoError(t, user.SetPassword(password))

	return &authRevocationUserRepo{user: user}
}

func (r *authRevocationUserRepo) GetByID(context.Context, int64) (*User, error) {
	return cloneAuthRevocationUser(r.user), nil
}

func (r *authRevocationUserRepo) GetByEmail(context.Context, string) (*User, error) {
	return cloneAuthRevocationUser(r.user), nil
}

func (r *authRevocationUserRepo) Update(_ context.Context, user *User) error {
	r.user.Email = user.Email
	r.user.Username = user.Username
	r.user.Notes = user.Notes
	r.user.PasswordHash = user.PasswordHash
	r.user.Role = user.Role
	r.user.Balance = user.Balance
	r.user.Concurrency = user.Concurrency
	r.user.Status = user.Status
	r.user.TotalRecharged = user.TotalRecharged
	return nil
}

func (r *authRevocationUserRepo) TouchLastActive(context.Context, int64, time.Time) error {
	return nil
}

func (r *authRevocationUserRepo) UpdatePasswordAndIncrementTokenVersion(_ context.Context, userID int64, passwordHash string) (int64, error) {
	r.user.PasswordHash = passwordHash
	r.user.TokenVersion++
	return r.user.TokenVersion, nil
}

func (r *authRevocationUserRepo) IncrementTokenVersion(context.Context, int64) (int64, error) {
	r.user.TokenVersion++
	return r.user.TokenVersion, nil
}

func cloneAuthRevocationUser(user *User) *User {
	if user == nil {
		return nil
	}
	clone := *user
	return &clone
}

func newAuthRevocationServices(repo *authRevocationUserRepo) (*AuthService, *UserService) {
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "test-jwt-secret-32bytes-long!!!",
			ExpireHour:             1,
			RefreshTokenExpireDays: 30,
		},
	}
	authSvc := NewAuthService(nil, repo, nil, newMemoryRefreshTokenCache(), nil, cfg, nil, nil, nil, nil, nil, nil, nil)
	userSvc := NewUserService(repo, nil, nil)
	return authSvc, userSvc
}

func newAuthRevocationServiceWithPasswordReset(repo *authRevocationUserRepo, cache EmailCache) *AuthService {
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "test-jwt-secret-32bytes-long!!!",
			ExpireHour:             1,
			RefreshTokenExpireDays: 30,
		},
	}
	settings := map[string]string{
		SettingKeyEmailVerifyEnabled:   "true",
		SettingKeyPasswordResetEnabled: "true",
		SettingKeyRegistrationEnabled:  "true",
	}
	settingRepo := &settingRepoStub{values: settings}
	return NewAuthService(
		nil,
		repo,
		nil,
		newMemoryRefreshTokenCache(),
		nil,
		cfg,
		NewSettingService(settingRepo, cfg),
		NewEmailService(settingRepo, cache),
		nil,
		nil,
		nil,
		nil,
		nil,
	)
}

func TestAuthRevocation_ChangePasswordRejectsOldAccessAndRefreshTokens(t *testing.T) {
	repo := newAuthRevocationUserRepo(t, "change-password@test.com", "old-password")
	authSvc, userSvc := newAuthRevocationServices(repo)

	oldAccessToken, loggedInUser, err := authSvc.Login(context.Background(), repo.user.Email, "old-password")
	require.NoError(t, err)
	pair, err := authSvc.GenerateTokenPair(context.Background(), loggedInUser, "")
	require.NoError(t, err)

	err = userSvc.ChangePassword(context.Background(), repo.user.ID, ChangePasswordRequest{
		CurrentPassword: "old-password",
		NewPassword:     "new-password",
	})
	require.NoError(t, err)

	reloaded, err := repo.GetByID(context.Background(), repo.user.ID)
	require.NoError(t, err)
	require.Greater(t, reloaded.TokenVersion, loggedInUser.TokenVersion)

	_, err = authSvc.RefreshToken(context.Background(), oldAccessToken)
	require.ErrorIs(t, err, ErrTokenRevoked)

	_, err = authSvc.RefreshTokenPair(context.Background(), pair.RefreshToken)
	require.ErrorIs(t, err, ErrTokenRevoked)
}

func TestAuthRevocation_ResetPasswordPersistsTokenVersionAndRejectsOldTokens(t *testing.T) {
	repo := newAuthRevocationUserRepo(t, "reset-password@test.com", "old-password")
	cache := &authRevocationEmailCache{
		reset: &PasswordResetTokenData{Token: "reset-token", CreatedAt: time.Now()},
	}
	authSvc := newAuthRevocationServiceWithPasswordReset(repo, cache)

	oldAccessToken, loggedInUser, err := authSvc.Login(context.Background(), repo.user.Email, "old-password")
	require.NoError(t, err)
	pair, err := authSvc.GenerateTokenPair(context.Background(), loggedInUser, "")
	require.NoError(t, err)

	err = authSvc.ResetPassword(context.Background(), repo.user.Email, "reset-token", "new-password")
	require.NoError(t, err)
	require.True(t, cache.deletedResetToken)

	reloaded, err := repo.GetByID(context.Background(), repo.user.ID)
	require.NoError(t, err)
	require.Greater(t, reloaded.TokenVersion, loggedInUser.TokenVersion)

	_, err = authSvc.RefreshToken(context.Background(), oldAccessToken)
	require.ErrorIs(t, err, ErrTokenRevoked)

	_, err = authSvc.RefreshTokenPair(context.Background(), pair.RefreshToken)
	require.ErrorIs(t, err, ErrTokenRevoked)
}

func TestAuthRevocation_RevokeAllSessionsRejectsOldAccessToken(t *testing.T) {
	repo := newAuthRevocationUserRepo(t, "revoke-all@test.com", "password")
	authSvc, _ := newAuthRevocationServices(repo)

	oldAccessToken, loggedInUser, err := authSvc.Login(context.Background(), repo.user.Email, "password")
	require.NoError(t, err)

	require.NoError(t, authSvc.RevokeAllUserSessions(context.Background(), repo.user.ID))

	reloaded, err := repo.GetByID(context.Background(), repo.user.ID)
	require.NoError(t, err)
	require.Greater(t, reloaded.TokenVersion, loggedInUser.TokenVersion)

	_, err = authSvc.RefreshToken(context.Background(), oldAccessToken)
	require.ErrorIs(t, err, ErrTokenRevoked)
}

type authRevocationEmailCache struct {
	reset             *PasswordResetTokenData
	deletedResetToken bool
}

func (c *authRevocationEmailCache) GetVerificationCode(context.Context, string) (*VerificationCodeData, error) {
	return nil, nil
}

func (c *authRevocationEmailCache) SetVerificationCode(context.Context, string, *VerificationCodeData, time.Duration) error {
	return nil
}

func (c *authRevocationEmailCache) DeleteVerificationCode(context.Context, string) error {
	return nil
}

func (c *authRevocationEmailCache) GetPasswordResetToken(context.Context, string) (*PasswordResetTokenData, error) {
	return c.reset, nil
}

func (c *authRevocationEmailCache) SetPasswordResetToken(context.Context, string, *PasswordResetTokenData, time.Duration) error {
	return nil
}

func (c *authRevocationEmailCache) DeletePasswordResetToken(context.Context, string) error {
	c.deletedResetToken = true
	c.reset = nil
	return nil
}

func (c *authRevocationEmailCache) IsPasswordResetEmailInCooldown(context.Context, string) bool {
	return false
}

func (c *authRevocationEmailCache) SetPasswordResetEmailCooldown(context.Context, string, time.Duration) error {
	return nil
}

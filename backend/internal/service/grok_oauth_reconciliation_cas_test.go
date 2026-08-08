//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type grokReconcileCASRepo struct {
	*tokenRefreshAccountRepo
	active []Account
}

func (r *grokReconcileCASRepo) ListActive(context.Context) ([]Account, error) {
	return append([]Account(nil), r.active...), nil
}

func newGrokReconcileCASTestService(repo *grokReconcileCASRepo, invalidator TokenCacheInvalidator) *TokenRefreshService {
	cfg := &config.Config{TokenRefresh: config.TokenRefreshConfig{MaxRetries: 1}}
	service := NewTokenRefreshService(repo, nil, nil, nil, nil, invalidator, nil, cfg, nil)
	refresher := &tokenRefresherStub{}
	service.refreshers = []TokenRefresher{refresher}
	service.executors = []OAuthRefreshExecutor{refresher}
	return service
}

func TestReconcileGrokOAuthMissingRefreshUsesExactCredentialCAS(t *testing.T) {
	account := &Account{
		ID:          201,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"access_token": "access-only"},
	}
	baseRepo := &tokenRefreshAccountRepo{}
	baseRepo.accountsByID = map[int64]*Account{account.ID: account}
	repo := &grokReconcileCASRepo{tokenRefreshAccountRepo: baseRepo, active: []Account{*account}}
	invalidator := &tokenCacheInvalidatorStub{}
	service := newGrokReconcileCASTestService(repo, invalidator)

	result, err := service.ReconcileGrokOAuth(context.Background(), GrokOAuthReconcileInput{Apply: true})

	require.NoError(t, err)
	require.Equal(t, 1, result.Blocked)
	require.Equal(t, 1, repo.conditionalReconcileCalls)
	require.Equal(t, 1, repo.setErrorCalls)
	require.Equal(t, 1, invalidator.calls)
	require.Equal(t, StatusError, account.Status)
	require.False(t, account.Schedulable)
}

func TestReconcileGrokOAuthStaleMissingRefreshCannotDisableReauthorizedAccount(t *testing.T) {
	account := &Account{
		ID:          202,
		Platform:    PlatformGrok,
		Type:        AccountTypeOAuth,
		Status:      StatusActive,
		Schedulable: true,
		Credentials: map[string]any{"access_token": "old-access"},
	}
	baseRepo := &tokenRefreshAccountRepo{}
	baseRepo.accountsByID = map[int64]*Account{account.ID: account}
	baseRepo.beforeConditionalState = func() {
		account.Credentials = map[string]any{"access_token": "fresh-access", "refresh_token": "fresh-refresh"}
	}
	repo := &grokReconcileCASRepo{tokenRefreshAccountRepo: baseRepo, active: []Account{*account}}
	invalidator := &tokenCacheInvalidatorStub{}
	service := newGrokReconcileCASTestService(repo, invalidator)

	result, err := service.ReconcileGrokOAuth(context.Background(), GrokOAuthReconcileInput{Apply: true})

	require.NoError(t, err)
	require.Equal(t, 1, result.Skipped)
	require.Zero(t, result.Blocked)
	require.Equal(t, 1, repo.conditionalReconcileCalls)
	require.Zero(t, repo.setErrorCalls)
	require.Zero(t, invalidator.calls)
	require.Equal(t, StatusActive, account.Status)
	require.True(t, account.Schedulable)
	require.Equal(t, "fresh-refresh", account.GetGrokRefreshToken())
}

//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type refreshRotationMemoryCache struct {
	mu              sync.Mutex
	data            map[string]*RefreshTokenData
	consumed        map[string]*RefreshTokenData
	userSets        map[int64]map[string]struct{}
	familySets      map[string]map[string]struct{}
	revokedFamilies map[string]struct{}

	failAddUser   error
	failAddFamily error

	storeCount             int
	unblockReuseAfterStore int
	storeReached           chan struct{}
}

func newRefreshRotationMemoryCache() *refreshRotationMemoryCache {
	return &refreshRotationMemoryCache{
		data:            map[string]*RefreshTokenData{},
		consumed:        map[string]*RefreshTokenData{},
		userSets:        map[int64]map[string]struct{}{},
		familySets:      map[string]map[string]struct{}{},
		revokedFamilies: map[string]struct{}{},
	}
}

func (c *refreshRotationMemoryCache) blockReuseUntilStoreCount(storeCount int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.unblockReuseAfterStore = storeCount
	c.storeReached = make(chan struct{})
}

func (c *refreshRotationMemoryCache) StoreRefreshToken(_ context.Context, tokenHash string, data *RefreshTokenData, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[tokenHash] = cloneRefreshTokenData(data)
	c.storeCount++
	if c.storeReached != nil && c.storeCount >= c.unblockReuseAfterStore {
		close(c.storeReached)
		c.storeReached = nil
	}
	return nil
}

func (c *refreshRotationMemoryCache) GetRefreshToken(_ context.Context, tokenHash string) (*RefreshTokenData, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, ok := c.data[tokenHash]
	if !ok {
		return nil, ErrRefreshTokenNotFound
	}
	return cloneRefreshTokenData(data), nil
}

func (c *refreshRotationMemoryCache) DeleteRefreshToken(_ context.Context, tokenHash string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, tokenHash)
	return nil
}

func (c *refreshRotationMemoryCache) ConsumeRefreshToken(_ context.Context, tokenHash string) (*RefreshTokenData, error) {
	c.mu.Lock()
	if data, ok := c.data[tokenHash]; ok {
		delete(c.data, tokenHash)
		c.consumed[tokenHash] = cloneRefreshTokenData(data)
		clone := cloneRefreshTokenData(data)
		c.mu.Unlock()
		return clone, nil
	}
	if data, ok := c.consumed[tokenHash]; ok {
		wait := c.storeReached
		clone := cloneRefreshTokenData(data)
		c.mu.Unlock()
		if wait != nil {
			<-wait
		}
		return clone, ErrRefreshTokenReused
	}
	c.mu.Unlock()
	return nil, ErrRefreshTokenNotFound
}

func (c *refreshRotationMemoryCache) DeleteUserRefreshTokens(_ context.Context, userID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	for tokenHash := range c.userSets[userID] {
		delete(c.data, tokenHash)
	}
	delete(c.userSets, userID)
	return nil
}

func (c *refreshRotationMemoryCache) DeleteTokenFamily(_ context.Context, familyID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revokedFamilies[familyID] = struct{}{}
	for tokenHash := range c.familySets[familyID] {
		delete(c.data, tokenHash)
	}
	delete(c.familySets, familyID)
	return nil
}

func (c *refreshRotationMemoryCache) AddToUserTokenSet(_ context.Context, userID int64, tokenHash string, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failAddUser != nil {
		return c.failAddUser
	}
	if c.userSets[userID] == nil {
		c.userSets[userID] = map[string]struct{}{}
	}
	c.userSets[userID][tokenHash] = struct{}{}
	return nil
}

func (c *refreshRotationMemoryCache) AddToFamilyTokenSet(_ context.Context, familyID string, tokenHash string, _ time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failAddFamily != nil {
		return c.failAddFamily
	}
	if _, revoked := c.revokedFamilies[familyID]; revoked {
		return ErrRefreshTokenReused
	}
	if c.familySets[familyID] == nil {
		c.familySets[familyID] = map[string]struct{}{}
	}
	c.familySets[familyID][tokenHash] = struct{}{}
	return nil
}

func (c *refreshRotationMemoryCache) GetUserTokenHashes(_ context.Context, userID int64) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hashes := make([]string, 0, len(c.userSets[userID]))
	for tokenHash := range c.userSets[userID] {
		hashes = append(hashes, tokenHash)
	}
	return hashes, nil
}

func (c *refreshRotationMemoryCache) GetFamilyTokenHashes(_ context.Context, familyID string) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hashes := make([]string, 0, len(c.familySets[familyID]))
	for tokenHash := range c.familySets[familyID] {
		hashes = append(hashes, tokenHash)
	}
	return hashes, nil
}

func (c *refreshRotationMemoryCache) IsTokenInFamily(_ context.Context, familyID string, tokenHash string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.familySets[familyID][tokenHash]
	return ok, nil
}

func (c *refreshRotationMemoryCache) activeTokenCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data)
}

func cloneRefreshTokenData(data *RefreshTokenData) *RefreshTokenData {
	if data == nil {
		return nil
	}
	clone := *data
	return &clone
}

func TestAuthServiceRefreshTokenPair_ConcurrentRefreshAllowsExactlyOneSuccess(t *testing.T) {
	user := newRefreshRotationUser()
	cache := newRefreshRotationMemoryCache()
	authSvc := newRefreshRotationAuthService(user, cache)

	initialPair, err := authSvc.GenerateTokenPair(context.Background(), user, "")
	require.NoError(t, err)
	cache.blockReuseUntilStoreCount(2)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := authSvc.RefreshTokenPair(context.Background(), initialPair.RefreshToken)
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	reuses := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, ErrRefreshTokenReused) {
			reuses++
		}
	}

	require.Equal(t, 1, successes)
	require.Equal(t, 1, reuses)
}

func TestAuthServiceRefreshTokenPair_ReusingConsumedTokenRevokesDescendant(t *testing.T) {
	user := newRefreshRotationUser()
	cache := newRefreshRotationMemoryCache()
	authSvc := newRefreshRotationAuthService(user, cache)

	initialPair, err := authSvc.GenerateTokenPair(context.Background(), user, "")
	require.NoError(t, err)

	descendant, err := authSvc.RefreshTokenPair(context.Background(), initialPair.RefreshToken)
	require.NoError(t, err)
	require.NotEmpty(t, descendant.RefreshToken)

	_, err = authSvc.RefreshTokenPair(context.Background(), initialPair.RefreshToken)
	require.ErrorIs(t, err, ErrRefreshTokenReused)

	_, err = authSvc.RefreshTokenPair(context.Background(), descendant.RefreshToken)
	require.ErrorIs(t, err, ErrRefreshTokenInvalid)
}

func TestAuthServiceGenerateTokenPair_SetMembershipFailureFailsClosed(t *testing.T) {
	tests := []struct {
		name          string
		failAddUser   error
		failAddFamily error
	}{
		{
			name:        "user_set_failure",
			failAddUser: errors.New("user set failed"),
		},
		{
			name:          "family_set_failure",
			failAddFamily: errors.New("family set failed"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user := newRefreshRotationUser()
			cache := newRefreshRotationMemoryCache()
			cache.failAddUser = tt.failAddUser
			cache.failAddFamily = tt.failAddFamily
			authSvc := newRefreshRotationAuthService(user, cache)

			_, err := authSvc.GenerateTokenPair(context.Background(), user, "")
			require.Error(t, err)
			require.Equal(t, 0, cache.activeTokenCount())
		})
	}
}

func TestAuthServiceRefreshTokenPair_SetMembershipFailureFailsClosed(t *testing.T) {
	user := newRefreshRotationUser()
	cache := newRefreshRotationMemoryCache()
	authSvc := newRefreshRotationAuthService(user, cache)

	initialPair, err := authSvc.GenerateTokenPair(context.Background(), user, "")
	require.NoError(t, err)

	cache.failAddFamily = errors.New("family set failed")
	_, err = authSvc.RefreshTokenPair(context.Background(), initialPair.RefreshToken)
	require.Error(t, err)
	require.Equal(t, 0, cache.activeTokenCount())

	_, err = authSvc.RefreshTokenPair(context.Background(), initialPair.RefreshToken)
	require.ErrorIs(t, err, ErrRefreshTokenReused)
}

func newRefreshRotationUser() *User {
	return &User{
		ID:           42,
		Email:        "refresh-rotation@test.com",
		Role:         RoleUser,
		Status:       StatusActive,
		TokenVersion: 7,
	}
}

func newRefreshRotationAuthService(user *User, cache RefreshTokenCache) *AuthService {
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:                 "test-jwt-secret-32bytes-long!!!",
			ExpireHour:             1,
			RefreshTokenExpireDays: 30,
		},
	}
	return NewAuthService(nil, &userRepoStub{user: user}, nil, cache, nil, cfg, nil, nil, nil, nil, nil, nil, nil)
}

//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memorySSOTicketCache struct {
	data map[string]SSOTicketData
	ttl  map[string]time.Duration
}

func newMemorySSOTicketCache() *memorySSOTicketCache {
	return &memorySSOTicketCache{
		data: map[string]SSOTicketData{},
		ttl:  map[string]time.Duration{},
	}
}

func (c *memorySSOTicketCache) StoreSSOTicket(_ context.Context, ticket string, data SSOTicketData, ttl time.Duration) error {
	c.data[ticket] = data
	c.ttl[ticket] = ttl
	return nil
}

func (c *memorySSOTicketCache) ConsumeSSOTicket(_ context.Context, ticket string) (*SSOTicketData, error) {
	data, ok := c.data[ticket]
	if !ok {
		return nil, ErrSSOTicketNotFound
	}
	delete(c.data, ticket)
	return &data, nil
}

type memoryRefreshTokenCache struct {
	data map[string]*RefreshTokenData
}

func newMemoryRefreshTokenCache() *memoryRefreshTokenCache {
	return &memoryRefreshTokenCache{data: map[string]*RefreshTokenData{}}
}

func (c *memoryRefreshTokenCache) StoreRefreshToken(_ context.Context, tokenHash string, data *RefreshTokenData, _ time.Duration) error {
	c.data[tokenHash] = data
	return nil
}

func (c *memoryRefreshTokenCache) GetRefreshToken(_ context.Context, tokenHash string) (*RefreshTokenData, error) {
	data, ok := c.data[tokenHash]
	if !ok {
		return nil, ErrRefreshTokenNotFound
	}
	return data, nil
}

func (c *memoryRefreshTokenCache) DeleteRefreshToken(_ context.Context, tokenHash string) error {
	delete(c.data, tokenHash)
	return nil
}

func (c *memoryRefreshTokenCache) DeleteUserRefreshTokens(context.Context, int64) error { return nil }
func (c *memoryRefreshTokenCache) DeleteTokenFamily(context.Context, string) error      { return nil }
func (c *memoryRefreshTokenCache) AddToUserTokenSet(context.Context, int64, string, time.Duration) error {
	return nil
}
func (c *memoryRefreshTokenCache) AddToFamilyTokenSet(context.Context, string, string, time.Duration) error {
	return nil
}
func (c *memoryRefreshTokenCache) GetUserTokenHashes(context.Context, int64) ([]string, error) {
	return nil, nil
}
func (c *memoryRefreshTokenCache) GetFamilyTokenHashes(context.Context, string) ([]string, error) {
	return nil, nil
}
func (c *memoryRefreshTokenCache) IsTokenInFamily(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestAuthService_SSOIssueAndExchange_ConsumesTicketAndReturnsTokenPair(t *testing.T) {
	user := &User{
		ID:           42,
		Email:        "sso@test.com",
		Username:     "sso-user",
		Role:         RoleUser,
		Status:       StatusActive,
		TokenVersion: 3,
	}
	repo := &userRepoStub{user: user}
	service := newAuthService(repo, nil, nil)
	service.refreshTokenCache = newMemoryRefreshTokenCache()
	service.ssoTicketCache = newMemorySSOTicketCache()

	apiKeyID := int64(99)
	ticket, ttl, err := service.IssueSSOTicket(context.Background(), user.ID, &apiKeyID)
	require.NoError(t, err)
	require.NotEmpty(t, ticket)
	require.Equal(t, 60*time.Second, ttl)
	require.Equal(t, ttl, service.ssoTicketCache.(*memorySSOTicketCache).ttl[ticket])

	pair, exchangedUser, exchangedAPIKeyID, err := service.ExchangeSSOTicket(context.Background(), ticket)
	require.NoError(t, err)
	require.NotNil(t, pair)
	require.NotEmpty(t, pair.AccessToken)
	require.NotEmpty(t, pair.RefreshToken)
	require.Equal(t, user.ID, exchangedUser.ID)
	require.NotNil(t, exchangedAPIKeyID)
	require.Equal(t, apiKeyID, *exchangedAPIKeyID)

	_, _, _, err = service.ExchangeSSOTicket(context.Background(), ticket)
	require.ErrorIs(t, err, ErrInvalidSSOTicket)
}

func TestAuthService_ExchangeSSOTicket_InvalidTicket(t *testing.T) {
	service := newAuthService(&userRepoStub{}, nil, nil)
	service.refreshTokenCache = newMemoryRefreshTokenCache()
	service.ssoTicketCache = newMemorySSOTicketCache()

	_, _, _, err := service.ExchangeSSOTicket(context.Background(), "missing")
	require.ErrorIs(t, err, ErrInvalidSSOTicket)
}

func TestAuthService_ExchangeSSOTicket_InactiveUser(t *testing.T) {
	user := &User{ID: 7, Email: "inactive@test.com", Role: RoleUser, Status: StatusDisabled}
	service := newAuthService(&userRepoStub{user: user}, nil, nil)
	service.refreshTokenCache = newMemoryRefreshTokenCache()
	service.ssoTicketCache = newMemorySSOTicketCache()

	ticket, _, err := service.IssueSSOTicket(context.Background(), user.ID, nil)
	require.NoError(t, err)

	_, _, _, err = service.ExchangeSSOTicket(context.Background(), ticket)
	require.True(t, errors.Is(err, ErrUserNotActive))
}

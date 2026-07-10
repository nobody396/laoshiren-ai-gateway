//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

type memorySSOTicketCache struct {
	data map[string]SSOTicketData
	ttl  map[string]time.Duration
}

type embedTargetResolverStub struct {
	target *EmbedTarget
	err    error
}

func (s *embedTargetResolverStub) ResolveEmbedTarget(_ context.Context, _, _, _ string) (*EmbedTarget, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.target, nil
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

func (c *memoryRefreshTokenCache) ConsumeRefreshToken(_ context.Context, tokenHash string) (*RefreshTokenData, error) {
	data, ok := c.data[tokenHash]
	if !ok {
		return nil, ErrRefreshTokenNotFound
	}
	delete(c.data, tokenHash)
	return data, nil
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

func TestAuthService_EmbedTicket_BindsAudiencePurposeAndDelivery(t *testing.T) {
	user := &User{ID: 42, Email: "embed@test.com", Role: RoleUser, Status: StatusActive, TokenVersion: 1}
	svc := newAuthService(&userRepoStub{user: user}, nil, nil)
	cache := newMemorySSOTicketCache()
	svc.ssoTicketCache = cache
	svc.embedTargetResolver = &embedTargetResolverStub{target: &EmbedTarget{
		Kind: EmbedTargetKindCustomMenu, ID: "reports", URL: "https://consumer.example/embed", Audience: "https://consumer.example",
	}}

	issued, err := svc.IssueEmbedTicket(context.Background(), user.ID, EmbedTargetKindCustomMenu, "reports", EmbedDeliveryIframe)
	require.NoError(t, err)
	require.Equal(t, 60, issued.ExpiresIn)
	require.Equal(t, "https://consumer.example", issued.Audience)
	require.Equal(t, embedTicketPurpose, cache.data[issued.Ticket].Purpose)

	session, err := svc.ExchangeEmbedTicket(context.Background(), issued.Ticket, "https://CONSUMER.example:443/path", EmbedTargetKindCustomMenu, "reports", EmbedDeliveryIframe)
	require.NoError(t, err)
	require.NotEmpty(t, session.SessionToken)
	require.Equal(t, int(embedSessionTTL.Seconds()), session.ExpiresIn)

	claims := &embedSessionClaims{}
	parsed, err := jwt.ParseWithClaims(session.SessionToken, claims, func(*jwt.Token) (any, error) {
		return []byte(svc.cfg.JWT.Secret), nil
	})
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	require.Equal(t, embedSessionPurpose, claims.Purpose)
	require.Equal(t, jwt.ClaimStrings{"https://consumer.example"}, claims.Audience)
	require.Equal(t, EmbedDeliveryIframe, claims.Delivery)

	_, err = svc.ExchangeEmbedTicket(context.Background(), issued.Ticket, issued.Audience, EmbedTargetKindCustomMenu, "reports", EmbedDeliveryIframe)
	require.ErrorIs(t, err, ErrInvalidEmbedTicket)
}

func TestAuthService_EmbedTicket_WrongAudienceConsumesTicket(t *testing.T) {
	user := &User{ID: 7, Email: "embed@test.com", Role: RoleUser, Status: StatusActive}
	svc := newAuthService(&userRepoStub{user: user}, nil, nil)
	svc.ssoTicketCache = newMemorySSOTicketCache()
	svc.embedTargetResolver = &embedTargetResolverStub{target: &EmbedTarget{
		Kind: EmbedTargetKindPurchase, ID: "purchase", URL: "https://pay.example/path", Audience: "https://pay.example",
	}}
	issued, err := svc.IssueEmbedTicket(context.Background(), user.ID, EmbedTargetKindPurchase, "purchase", EmbedDeliveryNewTab)
	require.NoError(t, err)

	_, err = svc.ExchangeEmbedTicket(context.Background(), issued.Ticket, "https://evil.example", EmbedTargetKindPurchase, "purchase", EmbedDeliveryNewTab)
	require.ErrorIs(t, err, ErrInvalidEmbedTicket)
	_, err = svc.ExchangeEmbedTicket(context.Background(), issued.Ticket, issued.Audience, EmbedTargetKindPurchase, "purchase", EmbedDeliveryNewTab)
	require.ErrorIs(t, err, ErrInvalidEmbedTicket)
}

func TestAuthService_EmbedTicket_ExpiredAndInactiveFail(t *testing.T) {
	user := &User{ID: 8, Email: "embed@test.com", Role: RoleUser, Status: StatusDisabled}
	svc := newAuthService(&userRepoStub{user: user}, nil, nil)
	cache := newMemorySSOTicketCache()
	svc.ssoTicketCache = cache
	svc.embedTargetResolver = &embedTargetResolverStub{target: &EmbedTarget{
		Kind: EmbedTargetKindPurchase, ID: "purchase", URL: "https://pay.example", Audience: "https://pay.example",
	}}
	_, err := svc.IssueEmbedTicket(context.Background(), user.ID, EmbedTargetKindPurchase, "purchase", EmbedDeliveryIframe)
	require.ErrorIs(t, err, ErrUserNotActive)

	user.Status = StatusActive
	ticket, err := randomHexString(32)
	require.NoError(t, err)
	cache.data[ticket] = SSOTicketData{
		Purpose: embedTicketPurpose, UserID: user.ID, Audience: "https://pay.example",
		TargetKind: EmbedTargetKindPurchase, TargetID: "purchase", Delivery: EmbedDeliveryIframe,
		CreatedAt: time.Now().Add(-ssoTicketTTL - time.Second),
	}
	_, err = svc.ExchangeEmbedTicket(context.Background(), ticket, "https://pay.example", EmbedTargetKindPurchase, "purchase", EmbedDeliveryIframe)
	require.ErrorIs(t, err, ErrInvalidEmbedTicket)
}

func TestAuthService_ChatbotExchangeRejectsEmbedPurpose(t *testing.T) {
	user := &User{ID: 9, Email: "embed@test.com", Role: RoleUser, Status: StatusActive}
	svc := newAuthService(&userRepoStub{user: user}, nil, nil)
	cache := newMemorySSOTicketCache()
	svc.ssoTicketCache = cache
	ticket, err := randomHexString(32)
	require.NoError(t, err)
	cache.data[ticket] = SSOTicketData{Purpose: embedTicketPurpose, UserID: user.ID, CreatedAt: time.Now()}
	_, _, _, err = svc.ExchangeSSOTicket(context.Background(), ticket)
	require.ErrorIs(t, err, ErrInvalidSSOTicket)
}

func TestNormalizeConfiguredEmbedURL(t *testing.T) {
	normalized, audience, err := normalizeConfiguredEmbedURL("https://EXAMPLE.com:443/path?q=1&token=jwt&user_id=42", false)
	require.NoError(t, err)
	require.Equal(t, "https://example.com/path?q=1", normalized)
	require.Equal(t, "https://example.com", audience)

	_, _, err = normalizeConfiguredEmbedURL("http://example.com/path", true)
	require.Error(t, err)
	_, localhostAudience, err := normalizeConfiguredEmbedURL("http://localhost:3000/embed", true)
	require.NoError(t, err)
	require.Equal(t, "http://localhost:3000", localhostAudience)
	_, _, err = normalizeConfiguredEmbedURL("https://user:pass@example.com", false)
	require.Error(t, err)
}

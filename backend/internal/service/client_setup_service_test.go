package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type clientSetupAPIKeysStub struct {
	keys map[int64]*APIKey
}

type clientSetupEnsureAPIKeysStub struct {
	keys        []APIKey
	groups      []Group
	createdUser int64
	createdReq  CreateAPIKeyRequest
}

func (s *clientSetupEnsureAPIKeysStub) Create(_ context.Context, userID int64, req CreateAPIKeyRequest) (*APIKey, error) {
	s.createdUser = userID
	s.createdReq = req
	return &APIKey{ID: 99, UserID: userID, Key: "sk-created", Name: req.Name, Status: StatusActive}, nil
}

func (s *clientSetupEnsureAPIKeysStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	for i := range s.keys {
		if s.keys[i].ID == id {
			return &s.keys[i], nil
		}
	}
	return nil, ErrAPIKeyNotFound
}

func (s *clientSetupEnsureAPIKeysStub) GetAvailableGroups(context.Context, int64) ([]Group, error) {
	return s.groups, nil
}

func (s *clientSetupEnsureAPIKeysStub) List(context.Context, int64, pagination.PaginationParams, APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	return s.keys, &pagination.PaginationResult{}, nil
}

func (s *clientSetupAPIKeysStub) Create(context.Context, int64, CreateAPIKeyRequest) (*APIKey, error) {
	return nil, errors.New("unexpected Create call")
}

func (s *clientSetupAPIKeysStub) GetByID(_ context.Context, id int64) (*APIKey, error) {
	key, ok := s.keys[id]
	if !ok {
		return nil, ErrAPIKeyNotFound
	}
	return key, nil
}

func (s *clientSetupAPIKeysStub) GetAvailableGroups(context.Context, int64) ([]Group, error) {
	return nil, errors.New("unexpected GetAvailableGroups call")
}

func (s *clientSetupAPIKeysStub) List(context.Context, int64, pagination.PaginationParams, APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	return nil, nil, errors.New("unexpected List call")
}

type clientSetupTicketCacheStub struct {
	data map[string]SSOTicketData
	ttl  map[string]time.Duration
}

func newClientSetupTicketCacheStub() *clientSetupTicketCacheStub {
	return &clientSetupTicketCacheStub{
		data: make(map[string]SSOTicketData),
		ttl:  make(map[string]time.Duration),
	}
}

func (c *clientSetupTicketCacheStub) StoreSSOTicket(_ context.Context, ticket string, data SSOTicketData, ttl time.Duration) error {
	c.data[ticket] = data
	c.ttl[ticket] = ttl
	return nil
}

func (c *clientSetupTicketCacheStub) ConsumeSSOTicket(_ context.Context, ticket string) (*SSOTicketData, error) {
	data, ok := c.data[ticket]
	if !ok {
		return nil, ErrSSOTicketNotFound
	}
	delete(c.data, ticket)
	return &data, nil
}

func TestSelectClientSetupGroupUsesFixedCodexGroup(t *testing.T) {
	groups := []Group{
		{ID: 1, Name: "Codex 钱包", Platform: PlatformOpenAI, Status: StatusActive, SortOrder: 1},
		{ID: 2, Name: "Pro 20X", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, SortOrder: 2},
		{ID: 3, Name: "Codex 月卡 B", Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeCredit, SortOrder: 3},
	}

	selected := selectClientSetupGroup(ClientSetupTargetCodex, groups)
	require.NotNil(t, selected)
	require.Equal(t, int64(2), selected.ID)
}

func TestSelectClientSetupGroupUsesFixedClaudeGroup(t *testing.T) {
	groups := []Group{
		{ID: 1, Name: "Claude 钱包", Platform: PlatformAnthropic, Status: StatusActive, SortOrder: 1},
		{ID: 2, Name: "Claude MAX 20X 外接分组", Platform: PlatformAnthropic, Status: StatusActive, SortOrder: 2},
	}

	selected := selectClientSetupGroup(ClientSetupTargetClaude, groups)
	require.NotNil(t, selected)
	require.Equal(t, int64(2), selected.ID)
}

func TestSelectClientSetupGroupUsesPublicGrokBalanceGroup(t *testing.T) {
	groups := []Group{
		{ID: 35, Name: "Grok Lite 月卡组", Platform: PlatformGrok, Status: StatusActive, SubscriptionType: SubscriptionTypeCredit},
		{ID: 34, Name: "Grok 分组", Platform: PlatformGrok, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
		{ID: 49, Name: "Grok Pro V3 月卡组", Platform: PlatformGrok, Status: StatusActive, SubscriptionType: SubscriptionTypeCredit},
	}

	selected := selectClientSetupGroup(ClientSetupTargetGrok, groups)
	require.NotNil(t, selected)
	require.Equal(t, int64(34), selected.ID)
}

func TestSelectClientSetupGroupPrefersNeutralNameOverVersionedLegacyNames(t *testing.T) {
	groups := []Group{
		{ID: 34, Name: "Grok 4.5 分组", Platform: PlatformGrok, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
		{ID: 51, Name: "Grok 4.6 分组", Platform: PlatformGrok, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
		{ID: 52, Name: "Grok 分组", Platform: PlatformGrok, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
	}

	selected := selectClientSetupGroup(ClientSetupTargetGrok, groups)
	require.NotNil(t, selected)
	require.Equal(t, int64(52), selected.ID)
}

func TestSelectClientSetupGroupPrefersNewestVersionedLegacyName(t *testing.T) {
	groups := []Group{
		{ID: 34, Name: "Grok 4.5 分组", Platform: PlatformGrok, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
		{ID: 51, Name: "Grok 4.6 分组", Platform: PlatformGrok, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard},
	}

	selected := selectClientSetupGroup(ClientSetupTargetGrok, groups)
	require.NotNil(t, selected)
	require.Equal(t, int64(51), selected.ID)
}

func TestSelectClientSetupGroupDoesNotUseGrokMonthlyGroup(t *testing.T) {
	groups := []Group{
		{ID: 35, Name: "Grok Lite 月卡组", Platform: PlatformGrok, Status: StatusActive, SubscriptionType: SubscriptionTypeCredit},
	}

	require.Nil(t, selectClientSetupGroup(ClientSetupTargetGrok, groups))
}

func TestSelectClientSetupGroupDoesNotUseInactiveGrokTier(t *testing.T) {
	groups := []Group{
		{ID: 35, Name: "Grok Lite 月卡组", Platform: PlatformGrok, Status: "inactive"},
	}

	require.Nil(t, selectClientSetupGroup(ClientSetupTargetGrok, groups))
}

func TestIssueTicketCreatesGrokKeyForPublicBalanceGroup(t *testing.T) {
	apiKeys := &clientSetupEnsureAPIKeysStub{
		groups: []Group{{ID: 34, Name: "Grok 分组", Platform: PlatformGrok, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard}},
	}
	svc := &ClientSetupService{apiKeys: apiKeys, tickets: newClientSetupTicketCacheStub()}

	ticket, err := svc.IssueTicket(context.Background(), 2, ClientSetupTargetGrok)
	require.NoError(t, err)
	require.Equal(t, ClientSetupTargetGrok, ticket.Target)
	require.Equal(t, "Grok 分组", ticket.GroupName)
	require.Equal(t, int64(2), apiKeys.createdUser)
	require.NotNil(t, apiKeys.createdReq.GroupID)
	require.Equal(t, int64(34), *apiKeys.createdReq.GroupID)
}

func TestIssueTicketDoesNotReuseMonthlyGrokSetupKey(t *testing.T) {
	apiKeys := &clientSetupEnsureAPIKeysStub{
		keys: []APIKey{{
			ID:     88,
			UserID: 2,
			Name:   clientSetupKeyName(ClientSetupTargetGrok),
			Status: StatusActive,
			Group: &Group{
				ID:               35,
				Name:             "Grok Lite 月卡组",
				Platform:         PlatformGrok,
				Status:           StatusActive,
				SubscriptionType: SubscriptionTypeCredit,
			},
		}},
		groups: []Group{{
			ID:               34,
			Name:             "Grok 分组",
			Platform:         PlatformGrok,
			Status:           StatusActive,
			SubscriptionType: SubscriptionTypeStandard,
		}},
	}
	svc := &ClientSetupService{apiKeys: apiKeys, tickets: newClientSetupTicketCacheStub()}

	ticket, err := svc.IssueTicket(context.Background(), 2, ClientSetupTargetGrok)
	require.NoError(t, err)
	require.Equal(t, "Grok 分组", ticket.GroupName)
	require.NotNil(t, apiKeys.createdReq.GroupID)
	require.Equal(t, int64(34), *apiKeys.createdReq.GroupID)
}

func TestSelectClientSetupGroupDoesNotFallBack(t *testing.T) {
	groups := []Group{
		{ID: 1, Name: "Codex 钱包", Platform: PlatformOpenAI, Status: StatusActive},
		{ID: 2, Name: "MAX 20X", Platform: PlatformAnthropic, Status: StatusActive},
	}

	require.Nil(t, selectClientSetupGroup(ClientSetupTargetCodex, groups))
}

func TestClientSetupGroupCompatibility(t *testing.T) {
	require.True(t, clientSetupGroupCompatible(ClientSetupTargetClaude, &Group{
		Platform: PlatformAnthropic,
		Status:   StatusActive,
	}))
	require.True(t, clientSetupGroupCompatible(ClientSetupTargetClaude, &Group{
		Platform: PlatformAntigravity,
		Status:   StatusActive,
	}))
	require.False(t, clientSetupGroupCompatible(ClientSetupTargetClaude, &Group{
		Platform: PlatformOpenAI,
		Status:   StatusActive,
	}))
	require.True(t, clientSetupGroupCompatible(ClientSetupTargetCodex, &Group{
		Platform: PlatformOpenAI,
		Status:   StatusActive,
	}))
	require.True(t, clientSetupGroupCompatible(ClientSetupTargetGrok, &Group{
		Platform: PlatformGrok,
		Status:   StatusActive,
	}))
	require.False(t, clientSetupGroupCompatible(ClientSetupTargetGrok, &Group{
		Platform: PlatformOpenAI,
		Status:   StatusActive,
	}))
	require.False(t, clientSetupGroupCompatible(ClientSetupTargetCodex, &Group{
		Platform: PlatformOpenAI,
		Status:   "inactive",
	}))
}

func TestNormalizeClientSetupTargetRejectsUnknownTarget(t *testing.T) {
	_, err := normalizeClientSetupTarget("gemini")
	require.ErrorIs(t, err, ErrInvalidClientSetupTarget)
}

func TestClientSetupTargetForGroup(t *testing.T) {
	require.Equal(t, ClientSetupTargetCodex, clientSetupTargetForGroup(&Group{Platform: PlatformOpenAI, Status: StatusActive}))
	require.Equal(t, ClientSetupTargetClaude, clientSetupTargetForGroup(&Group{Platform: PlatformAnthropic, Status: StatusActive}))
	require.Equal(t, ClientSetupTargetClaude, clientSetupTargetForGroup(&Group{Platform: PlatformAntigravity, Status: StatusActive}))
	require.Equal(t, ClientSetupTargetGrok, clientSetupTargetForGroup(&Group{Platform: PlatformGrok, Status: StatusActive}))
	require.Empty(t, clientSetupTargetForGroup(&Group{Platform: PlatformGemini, Status: StatusActive}))
	require.Empty(t, clientSetupTargetForGroup(&Group{Platform: PlatformOpenAI, Status: "inactive"}))
}

func TestIssueTicketForAPIKeyAndExchangeKeepsExistingGrokCredential(t *testing.T) {
	group := &Group{ID: 49, Name: "Grok Pro V3 月卡组", Platform: PlatformGrok, Status: StatusActive}
	key := &APIKey{ID: 45, UserID: 9, Key: "sk-existing-grok-key", Name: "我的 Grok", Status: StatusActive, Group: group}
	svc := &ClientSetupService{
		apiKeys: &clientSetupAPIKeysStub{keys: map[int64]*APIKey{key.ID: key}},
		tickets: newClientSetupTicketCacheStub(),
	}

	ticket, err := svc.IssueTicketForAPIKey(context.Background(), key.UserID, key.ID)
	require.NoError(t, err)
	require.Equal(t, ClientSetupTargetGrok, ticket.Target)

	credential, err := svc.ExchangeTicket(context.Background(), ticket.Ticket)
	require.NoError(t, err)
	require.Equal(t, ClientSetupTargetGrok, credential.Target)
	require.Equal(t, key.Key, credential.APIKey)
	require.Equal(t, clientSetupAPIBaseURL, credential.BaseURL)
}

func TestIssueTicketForAPIKeyAndExchangeKeepsExistingCodexCredential(t *testing.T) {
	group := &Group{ID: 7, Name: "Codex 自选分组", Platform: PlatformOpenAI, Status: StatusActive}
	key := &APIKey{ID: 41, UserID: 9, Key: "sk-existing-codex-key", Name: "我的 Codex", Status: StatusActive, Group: group}
	cache := newClientSetupTicketCacheStub()
	svc := &ClientSetupService{
		apiKeys: &clientSetupAPIKeysStub{keys: map[int64]*APIKey{key.ID: key}},
		tickets: cache,
	}

	ticket, err := svc.IssueTicketForAPIKey(context.Background(), key.UserID, key.ID)
	require.NoError(t, err)
	require.Len(t, ticket.Ticket, 64)
	require.Equal(t, 600, ticket.ExpiresIn)
	require.Equal(t, ClientSetupTargetCodex, ticket.Target)
	require.Equal(t, key.Name, ticket.KeyName)
	require.Equal(t, group.Name, ticket.GroupName)
	require.Equal(t, clientSetupTicketTTL, cache.ttl[ticket.Ticket])
	require.Equal(t, key.ID, *cache.data[ticket.Ticket].APIKeyID)

	credential, err := svc.ExchangeTicket(context.Background(), ticket.Ticket)
	require.NoError(t, err)
	require.Equal(t, ClientSetupTargetCodex, credential.Target)
	require.Equal(t, key.Key, credential.APIKey)
	require.Equal(t, clientSetupAPIBaseURL, credential.BaseURL)

	_, err = svc.ExchangeTicket(context.Background(), ticket.Ticket)
	require.ErrorIs(t, err, ErrInvalidClientSetupTicket)
}

func TestIssueTicketForAPIKeyUsesAntigravityEndpoint(t *testing.T) {
	group := &Group{ID: 8, Name: "Claude 自选分组", Platform: PlatformAntigravity, Status: StatusActive}
	key := &APIKey{ID: 42, UserID: 9, Key: "sk-existing-claude-key", Name: "我的 Claude", Status: StatusActive, Group: group}
	svc := &ClientSetupService{
		apiKeys: &clientSetupAPIKeysStub{keys: map[int64]*APIKey{key.ID: key}},
		tickets: newClientSetupTicketCacheStub(),
	}

	ticket, err := svc.IssueTicketForAPIKey(context.Background(), key.UserID, key.ID)
	require.NoError(t, err)
	require.Equal(t, ClientSetupTargetClaude, ticket.Target)

	credential, err := svc.ExchangeTicket(context.Background(), ticket.Ticket)
	require.NoError(t, err)
	require.Equal(t, key.Key, credential.APIKey)
	require.Equal(t, clientSetupAPIBaseURL+"/antigravity", credential.BaseURL)
}

func TestIssueTicketForAPIKeyRejectsUnsafeSelections(t *testing.T) {
	activeGroup := &Group{ID: 7, Name: "Codex", Platform: PlatformOpenAI, Status: StatusActive}
	tests := []struct {
		name   string
		userID int64
		key    *APIKey
	}{
		{name: "other owner", userID: 10, key: &APIKey{ID: 1, UserID: 9, Status: StatusActive, Group: activeGroup}},
		{name: "inactive key", userID: 9, key: &APIKey{ID: 2, UserID: 9, Status: "inactive", Group: activeGroup}},
		{name: "missing group", userID: 9, key: &APIKey{ID: 3, UserID: 9, Status: StatusActive}},
		{name: "unsupported group", userID: 9, key: &APIKey{ID: 4, UserID: 9, Status: StatusActive, Group: &Group{Platform: PlatformGemini, Status: StatusActive}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ClientSetupService{
				apiKeys: &clientSetupAPIKeysStub{keys: map[int64]*APIKey{tt.key.ID: tt.key}},
				tickets: newClientSetupTicketCacheStub(),
			}
			_, err := svc.IssueTicketForAPIKey(context.Background(), tt.userID, tt.key.ID)
			require.ErrorIs(t, err, ErrClientSetupKeyUnavailable)
		})
	}
}

func TestExchangeTicketRejectsExpiredTicketBeforeReturningKey(t *testing.T) {
	group := &Group{ID: 7, Name: "Codex", Platform: PlatformOpenAI, Status: StatusActive}
	key := &APIKey{ID: 41, UserID: 9, Key: "sk-existing-codex-key", Status: StatusActive, Group: group}
	cache := newClientSetupTicketCacheStub()
	cache.data["aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"] = SSOTicketData{
		Purpose:    clientSetupTicketPurpose,
		UserID:     key.UserID,
		APIKeyID:   &key.ID,
		TargetKind: ClientSetupTargetCodex,
		CreatedAt:  time.Now().UTC().Add(-clientSetupTicketTTL - time.Second),
	}
	svc := &ClientSetupService{
		apiKeys: &clientSetupAPIKeysStub{keys: map[int64]*APIKey{key.ID: key}},
		tickets: cache,
	}

	_, err := svc.ExchangeTicket(context.Background(), "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	require.ErrorIs(t, err, ErrInvalidClientSetupTicket)
}

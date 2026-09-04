package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAPIKeyGroupIDsExplicitSnapshot(t *testing.T) {
	legacy := int64(6)
	require.NoError(t, validateAPIKeyGroupIDs(&legacy, nil))
	require.NoError(t, validateAPIKeyGroupIDs(nil, []int64{6, 5, 57}))
	for _, ids := range [][]int64{{}, {0}, {-1}, {6, 6}, make([]int64, 101)} {
		require.Error(t, validateAPIKeyGroupIDs(nil, ids))
	}
	require.Error(t, validateAPIKeyGroupIDs(&legacy, []int64{6}))
	require.False(t, (&APIKey{GroupID: &legacy}).IsMultiGroup())
	require.True(t, (&APIKey{GroupIDs: []int64{6}}).IsMultiGroup())
}

func TestMultiGroupProtocolAdmissionDoesNotInventBridge(t *testing.T) {
	claude := &Group{Platform: PlatformAnthropic}
	require.True(t, MultiGroupProtocolSupported(claude, "messages"))
	require.False(t, MultiGroupProtocolSupported(claude, "responses"))
	gpt := &Group{Platform: PlatformOpenAI}
	require.False(t, MultiGroupProtocolSupported(gpt, "messages"))
	gpt.AllowMessagesDispatch = true
	require.True(t, MultiGroupProtocolSupported(gpt, "messages"))
	require.True(t, MultiGroupProtocolSupported(&Group{Platform: PlatformGemini}, "generate_content"))
	require.False(t, MultiGroupProtocolSupported(&Group{Platform: PlatformUniversal}, "messages"))
	require.False(t, MultiGroupProtocolSupported(gpt, "realtime"))
}

func TestMultiGroupAuthSnapshotCopiesAuthorizationOrder(t *testing.T) {
	s := &APIKeyService{}
	key := &APIKey{ID: 1, User: &User{ID: 2}, GroupIDs: []int64{6, 5, 57}}
	snapshot := s.snapshotFromAPIKey(key)
	key.GroupIDs[0] = 99
	require.Equal(t, []int64{6, 5, 57}, snapshot.GroupIDs)
	restored := s.snapshotToAPIKey("dummy-key", snapshot)
	restored.GroupIDs[0] = 88
	require.Equal(t, []int64{6, 5, 57}, snapshot.GroupIDs)
	require.Equal(t, []int64{88, 5, 57}, restored.GroupIDs)
}

type multiGroupRepoStub struct {
	GroupRepository
	groups map[int64]*Group
}

func (s *multiGroupRepoStub) GetByID(_ context.Context, id int64) (*Group, error) {
	g := s.groups[id]
	if g == nil {
		return nil, ErrGroupNotFound
	}
	return g, nil
}

type multiGroupUserStub struct {
	UserRepository
	user   *User
	readID int64
}

func (s *multiGroupUserStub) GetByID(_ context.Context, id int64) (*User, error) {
	s.readID = id
	return s.user, nil
}

type multiGroupKeyRepoStub struct {
	APIKeyRepository
	key    *APIKey
	writes int
}

func (s *multiGroupKeyRepoStub) ExistsByKey(context.Context, string) (bool, error) { return false, nil }
func (s *multiGroupKeyRepoStub) Create(_ context.Context, k *APIKey) error {
	s.key = k
	s.writes++
	k.ID = 42
	return nil
}
func (s *multiGroupKeyRepoStub) Update(_ context.Context, k *APIKey) error {
	s.key = k
	s.writes++
	return nil
}
func (s *multiGroupKeyRepoStub) GetByID(context.Context, int64) (*APIKey, error) {
	copy := *s.key
	copy.GroupIDs = append([]int64(nil), s.key.GroupIDs...)
	return &copy, nil
}

func TestMultiGroupCreateUpdateAndRevocationUsePayerAuthorization(t *testing.T) {
	ctx := context.Background()
	user := &User{ID: 2, Status: StatusActive, AllowedGroups: []int64{6}}
	users := &multiGroupUserStub{user: user}
	groups := &multiGroupRepoStub{groups: map[int64]*Group{6: {ID: 6, Status: StatusActive, Platform: PlatformOpenAI, IsExclusive: true}, 57: {ID: 57, Status: StatusActive, Platform: PlatformGemini}, 90: {ID: 90, Status: StatusActive, Platform: PlatformUniversal}}}
	keys := &multiGroupKeyRepoStub{}
	svc := &APIKeyService{apiKeyRepo: keys, userRepo: users, groupRepo: groups}
	custom := "multi-group-fixture-key"
	key, err := svc.Create(ctx, 2, CreateAPIKeyRequest{Name: "multi", CustomKey: &custom, GroupIDs: []int64{6, 57}})
	require.NoError(t, err)
	require.Equal(t, []int64{6, 57}, key.GroupIDs)
	require.Nil(t, key.GroupID)
	changed, err := svc.Update(ctx, key.ID, 2, UpdateAPIKeyRequest{GroupIDs: []int64{57, 6}})
	require.NoError(t, err)
	require.Equal(t, []int64{57, 6}, changed.GroupIDs)
	writes := keys.writes
	_, err = svc.Update(ctx, key.ID, 2, UpdateAPIKeyRequest{GroupIDs: []int64{}})
	require.Error(t, err)
	require.Equal(t, writes, keys.writes)
	require.Error(t, svc.validateMultiGroupAccess(ctx, user, nil, []int64{90}))
	// The cached key still contains an old grant, while the payer's current grant was revoked.
	users.user = &User{ID: 2, Status: StatusActive}
	_, err = svc.AuthorizeMultiGroupTarget(ctx, key, groups.groups[6])
	require.Error(t, err)
	require.Equal(t, int64(2), users.readID)
	require.Equal(t, []int64{6}, key.User.AllowedGroups)
	users.user = user
	id := int64(57)
	single, err := svc.Update(ctx, key.ID, 2, UpdateAPIKeyRequest{GroupID: &id})
	require.NoError(t, err)
	require.Empty(t, single.GroupIDs)
	require.Equal(t, &id, single.GroupID)
}

func TestMultiGroupCatalogUpdatesWithoutReissuingKey(t *testing.T) {
	channelService := &ChannelService{}
	accounts := &multiGroupInventoryStub{}
	gateway := &GatewayService{channelService: channelService, accountRepo: accounts}
	group := &Group{ID: 6, Platform: PlatformOpenAI, Status: StatusActive}
	key := &APIKey{ID: 10, GroupIDs: []int64{6}}
	update := func(status string, models []string) {
		mapping := map[string]any{}
		for _, model := range models {
			mapping[model] = model
		}
		accounts.accounts = []Account{{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": mapping}}}
		channelService.cache.Store(populateChannelCache([]Channel{{ID: 1, Status: status, GroupIDs: []int64{6}, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: models}}}}, map[int64]string{6: PlatformOpenAI}))
	}
	update(StatusActive, []string{"model-old"})
	models, err := gateway.MultiGroupModels(context.Background(), group)
	require.NoError(t, err)
	require.Equal(t, []string{"model-old"}, models)
	update(StatusActive, []string{"model-new", "future-family-*"})
	models, err = gateway.MultiGroupModels(context.Background(), group)
	require.NoError(t, err)
	require.NotContains(t, models, "model-old")
	require.Contains(t, models, "model-new")
	declaration, err := gateway.MultiGroupCatalog(context.Background(), group)
	require.NoError(t, err)
	require.True(t, declaration.Matches(group, "future-family-unreleased-name"))
	require.False(t, declaration.Matches(group, "another-family-model"))
	require.Equal(t, []int64{6}, key.GroupIDs)
	update("disabled", []string{"model-new"})
	_, err = gateway.MultiGroupModels(context.Background(), group)
	require.Error(t, err, "disabled catalog must not look like no matching model and trigger another funding group")
	channelService.cache.Store(newEmptyChannelCache())
	// A cached database failure is not the same as a valid default-price group.
	channelService.storeErrorCache()
	_, err = gateway.MultiGroupModels(context.Background(), group)
	require.Error(t, err)
}

func TestMultiGroupTargetUsesCurrentTeamPayerNotActor(t *testing.T) {
	users := &multiGroupUserStub{user: &User{ID: 2, Status: StatusActive, AllowedGroups: []int64{6}}}
	service := &APIKeyService{userRepo: users}
	group := &Group{ID: 6, Platform: PlatformOpenAI, Status: StatusActive, IsExclusive: true}
	key := &APIKey{UserID: 17, User: &User{ID: 2}, GroupIDs: []int64{6}}
	payer, err := service.AuthorizeMultiGroupTarget(context.Background(), key, group)
	require.NoError(t, err)
	require.Equal(t, int64(2), payer.ID)
	require.Equal(t, int64(2), users.readID)
	// Team hydration changes the payer after owner transfer; an actor grant is irrelevant.
	key.User = &User{ID: 9}
	users.user = &User{ID: 9, Status: StatusActive}
	_, err = service.AuthorizeMultiGroupTarget(context.Background(), key, group)
	require.Error(t, err)
	require.Equal(t, int64(9), users.readID)
}

func TestMultiGroupAdminExplicitRebindClearsPreviousAuthorization(t *testing.T) {
	for _, id := range []int64{0, 57} {
		keys := &multiGroupKeyRepoStub{key: &APIKey{ID: 42, GroupIDs: []int64{6, 57}}}
		groups := &multiGroupRepoStub{groups: map[int64]*Group{57: {ID: 57, Status: StatusActive, Platform: PlatformGemini}}}
		admin := &adminServiceImpl{apiKeyRepo: keys, groupRepo: groups}
		result, err := admin.AdminUpdateAPIKeyGroupID(context.Background(), 42, &id)
		require.NoError(t, err)
		require.Empty(t, result.APIKey.GroupIDs)
		require.Empty(t, keys.key.GroupIDs)
		if id == 0 {
			require.Nil(t, keys.key.GroupID)
		} else {
			require.Equal(t, &id, keys.key.GroupID)
		}
	}
}

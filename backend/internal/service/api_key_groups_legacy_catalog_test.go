package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type multiGroupInventoryStub struct {
	AccountRepository
	accounts []Account
	err      error
}

func (s *multiGroupInventoryStub) ListByGroup(context.Context, int64) ([]Account, error) {
	return s.accounts, s.err
}

func TestMultiGroupLegacyDefaultPricingUsesAccountInventory(t *testing.T) {
	for _, withChannel := range []bool{false, true} {
		name := "no_channel"
		if withChannel {
			name = "active_channel_without_price_overrides"
		}
		t.Run(name, func(t *testing.T) {
			channels := &ChannelService{}
			cache := newEmptyChannelCache()
			cache.loadedAt = time.Now()
			if withChannel {
				cache = populateChannelCache([]Channel{{ID: 4, Status: StatusActive, GroupIDs: []int64{6}, RestrictModels: false}}, map[int64]string{6: PlatformOpenAI})
			}
			channels.cache.Store(cache)
			accounts := &multiGroupInventoryStub{accounts: []Account{{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, Credentials: map[string]any{"model_mapping": map[string]any{"configured-current-model": "provider-model"}}}}}
			gateway := &GatewayService{channelService: channels, accountRepo: accounts}
			models, err := gateway.MultiGroupModels(context.Background(), &Group{ID: 6, Platform: PlatformOpenAI, Status: StatusActive})
			require.NoError(t, err, "legitimate inherited/default pricing must not make the group unavailable")
			require.Equal(t, []string{"configured-current-model"}, models)
		})
	}
}

func (s *multiGroupInventoryStub) ListGroupModelInventory(context.Context, int64) ([]AccountModelInventory, error) {
	if s.err != nil {
		return nil, s.err
	}
	rows := []AccountModelInventory{}
	for _, account := range s.accounts {
		mapping, _ := account.Credentials["model_mapping"].(map[string]any)
		rows = append(rows, AccountModelInventory{Platform: account.Platform, Type: account.Type, ModelMapping: mapping})
	}
	return rows, nil
}

func TestMultiGroupLegacyInventoryRetainsUnavailableAccountsAndRefreshes(t *testing.T) {
	channels := &ChannelService{}
	cache := newEmptyChannelCache()
	cache.loadedAt = time.Now()
	channels.cache.Store(cache)
	account := Account{ID: 10, Platform: PlatformOpenAI, Status: StatusError, Schedulable: false, Credentials: map[string]any{"model_mapping": map[string]any{"configured-old": "upstream-old"}}}
	accounts := &multiGroupInventoryStub{accounts: []Account{account}}
	gateway := &GatewayService{channelService: channels, accountRepo: accounts}
	group := &Group{ID: 6, Platform: PlatformOpenAI}
	models, err := gateway.MultiGroupModels(context.Background(), group)
	require.NoError(t, err)
	require.Equal(t, []string{"configured-old"}, models, "temporary health must not remove first-priority declarations")
	accounts.accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"configured-new": "upstream-new"}}
	models, err = gateway.MultiGroupModels(context.Background(), group)
	require.NoError(t, err)
	require.Equal(t, []string{"configured-new"}, models)
	accounts.err = errors.New("database unavailable")
	_, err = gateway.MultiGroupModels(context.Background(), group)
	require.Error(t, err)
}

func TestMultiGroupOptionalChannelRestrictionsAndAliases(t *testing.T) {
	channels := &ChannelService{}
	accounts := &multiGroupInventoryStub{accounts: []Account{{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"family-*": "provider-*", "other-model": "other-model"}}}}}
	gateway := &GatewayService{channelService: channels, accountRepo: accounts}
	group := &Group{ID: 6, Platform: PlatformOpenAI}
	channel := Channel{ID: 4, Status: StatusActive, GroupIDs: []int64{6}, RestrictModels: true, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"family-approved", "public-alias", "unbacked-alias"}}}, ModelMapping: map[string]map[string]string{PlatformOpenAI: {"public-alias": "other-model", "unbacked-alias": "not-configured"}}}
	channels.cache.Store(populateChannelCache([]Channel{channel}, map[int64]string{6: PlatformOpenAI}))
	models, err := gateway.MultiGroupModels(context.Background(), group)
	require.NoError(t, err)
	require.Equal(t, []string{"family-approved"}, models)
	require.NotContains(t, models, "public-alias", "a channel-only alias cannot bypass requested-name account admission")
	channel.ModelPricing = nil
	channels.cache.Store(populateChannelCache([]Channel{channel}, map[int64]string{6: PlatformOpenAI}))
	models, err = gateway.MultiGroupModels(context.Background(), group)
	require.NoError(t, err)
	require.Empty(t, models, "an explicitly restrictive empty list remains deny-all")
	channel.Status = "disabled"
	channels.cache.Store(populateChannelCache([]Channel{channel}, map[int64]string{6: PlatformOpenAI}))
	_, err = gateway.MultiGroupModels(context.Background(), group)
	require.Error(t, err, "disabled is not an invitation to inherit pricing or use another funding group")
}

func TestMultiGroupErrorCacheCannotBecomeUnrestrictedDefaultPricing(t *testing.T) {
	channels := &ChannelService{}
	channels.storeErrorCache()
	accounts := &multiGroupInventoryStub{accounts: []Account{{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"model": "model"}}}}}
	gateway := &GatewayService{channelService: channels, accountRepo: accounts}
	_, err := gateway.MultiGroupModels(context.Background(), &Group{ID: 6, Platform: PlatformOpenAI})
	require.Error(t, err)
}

func TestMultiGroupReusesExistingModelAdmissionAndAliases(t *testing.T) {
	standard := &Group{ID: 6, Platform: PlatformOpenAI}
	enterprise := &Group{ID: 59, Platform: PlatformOpenAI}
	require.False(t, MultiGroupRequestModelMatches(standard, []string{"gpt-*"}, "gpt-5.4-mini"))
	require.True(t, MultiGroupRequestModelMatches(enterprise, []string{"gpt-*"}, "gpt-5.4-mini"))
	require.True(t, MultiGroupRequestModelMatches(standard, []string{"gpt-5.6-sol"}, "gpt-5.6-sol-openai-compact"))
}

func TestMultiGroupEmptyMappingRetainsWildcardDeclaration(t *testing.T) {
	channels := &ChannelService{}
	cache := newEmptyChannelCache()
	cache.loadedAt = time.Now()
	channels.cache.Store(cache)
	accounts := &multiGroupInventoryStub{accounts: []Account{{Platform: PlatformOpenAI}}}
	group := &Group{ID: 6, Platform: PlatformOpenAI}
	declaration, err := (&GatewayService{channelService: channels, accountRepo: accounts}).MultiGroupCatalog(context.Background(), group)
	require.NoError(t, err)
	require.NotEmpty(t, declaration.Models)
	require.NotContains(t, declaration.Models, "*")
	require.True(t, declaration.Matches(group, "future-model"), "empty mappings mean all models and must retain first billing priority")
}

func TestMultiGroupRestrictionUsesBillingModelSource(t *testing.T) {
	for _, source := range []string{BillingModelSourceRequested, BillingModelSourceChannelMapped, BillingModelSourceUpstream} {
		t.Run(source, func(t *testing.T) {
			priced := "public"
			if source == BillingModelSourceChannelMapped {
				priced = "channel-target"
			}
			if source == BillingModelSourceUpstream {
				priced = "account-target"
			}
			ch := Channel{ID: 4, Status: StatusActive, GroupIDs: []int64{6}, RestrictModels: true, BillingModelSource: source, ModelMapping: map[string]map[string]string{PlatformOpenAI: {"public": "channel-target"}}, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{priced}}}}
			channels := &ChannelService{}
			channels.cache.Store(populateChannelCache([]Channel{ch}, map[int64]string{6: PlatformOpenAI}))
			accounts := &multiGroupInventoryStub{accounts: []Account{{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"public": "account-target"}}}}}
			models, err := (&GatewayService{channelService: channels, accountRepo: accounts}).MultiGroupModels(context.Background(), &Group{ID: 6, Platform: PlatformOpenAI})
			require.NoError(t, err)
			require.Contains(t, models, "public")
		})
	}
}

func TestMultiGroupMappedPricingRetainsWildcardExclusionHoles(t *testing.T) {
	for _, source := range []string{BillingModelSourceUpstream, BillingModelSourceChannelMapped} {
		t.Run(source, func(t *testing.T) {
			mapping := map[string]any{"family-*": "allowed", "family-secret": "denied"}
			channelMapping := map[string]string{"family-*": "allowed", "family-secret": "denied"}
			ch := Channel{ID: 4, Status: StatusActive, GroupIDs: []int64{6}, RestrictModels: true, BillingModelSource: source, ModelMapping: map[string]map[string]string{PlatformOpenAI: channelMapping}, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"allowed"}}}}
			channels := &ChannelService{}
			channels.cache.Store(populateChannelCache([]Channel{ch}, map[int64]string{6: PlatformOpenAI}))
			accounts := &multiGroupInventoryStub{accounts: []Account{{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": mapping}}}}
			group := &Group{ID: 6, Platform: PlatformOpenAI}
			declaration, err := (&GatewayService{channelService: channels, accountRepo: accounts}).MultiGroupCatalog(context.Background(), group)
			require.NoError(t, err)
			require.True(t, declaration.Matches(group, "family-future"))
			require.False(t, declaration.Matches(group, "family-secret"))
			require.NotContains(t, declaration.Models, "family-secret")
		})
	}
}

func TestMultiGroupMatchesMessagesDispatchWithoutChangingResponses(t *testing.T) {
	channels := &ChannelService{}
	cache := newEmptyChannelCache()
	cache.loadedAt = time.Now()
	channels.cache.Store(cache)
	accounts := &multiGroupInventoryStub{accounts: []Account{{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5.6-sol": "gpt-5.6-sol"}}}}}
	group := &Group{ID: 6, Platform: PlatformOpenAI, AllowMessagesDispatch: true, MessagesDispatchModelConfig: OpenAIMessagesDispatchModelConfig{OpusMappedModel: "gpt-5.6-sol"}}
	declaration, err := (&GatewayService{channelService: channels, accountRepo: accounts}).MultiGroupCatalog(context.Background(), group)
	require.NoError(t, err)
	for _, model := range []string{"gpt-5.6-sol-high", "claude-opus-4-6"} {
		require.True(t, declaration.MatchesProtocol(group, "messages", model))
		require.False(t, declaration.MatchesProtocol(group, "responses", model))
	}
}

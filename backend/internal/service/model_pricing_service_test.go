package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

// --- stubs ---

type modelPricingGroupRepoStub struct {
	groups          []Group
	err             error
	listActiveCalls int
}

func (s *modelPricingGroupRepoStub) ListActive(_ context.Context) ([]Group, error) {
	s.listActiveCalls++
	return s.groups, s.err
}

func (s *modelPricingGroupRepoStub) Create(_ context.Context, _ *Group) error { return nil }
func (s *modelPricingGroupRepoStub) GetByID(_ context.Context, _ int64) (*Group, error) {
	return nil, nil
}
func (s *modelPricingGroupRepoStub) GetByIDLite(_ context.Context, _ int64) (*Group, error) {
	return nil, nil
}
func (s *modelPricingGroupRepoStub) Update(_ context.Context, _ *Group) error { return nil }
func (s *modelPricingGroupRepoStub) Delete(_ context.Context, _ int64) error  { return nil }
func (s *modelPricingGroupRepoStub) DeleteCascade(_ context.Context, _ int64) ([]int64, error) {
	return nil, nil
}
func (s *modelPricingGroupRepoStub) List(_ context.Context, _ pagination.PaginationParams) ([]Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *modelPricingGroupRepoStub) ListWithFilters(_ context.Context, _ pagination.PaginationParams, _, _, _ string, _ *bool) ([]Group, *pagination.PaginationResult, error) {
	return nil, nil, nil
}
func (s *modelPricingGroupRepoStub) ListActiveByPlatform(_ context.Context, _ string) ([]Group, error) {
	return nil, nil
}
func (s *modelPricingGroupRepoStub) ExistsByName(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (s *modelPricingGroupRepoStub) GetAccountCount(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (s *modelPricingGroupRepoStub) DeleteAccountGroupsByGroupID(_ context.Context, _ int64) (int64, error) {
	return 0, nil
}
func (s *modelPricingGroupRepoStub) GetAccountIDsByGroupIDs(_ context.Context, _ []int64) ([]int64, error) {
	return nil, nil
}
func (s *modelPricingGroupRepoStub) BindAccountsToGroup(_ context.Context, _ int64, _ []int64) error {
	return nil
}
func (s *modelPricingGroupRepoStub) UpdateSortOrders(_ context.Context, _ []GroupSortOrderUpdate) error {
	return nil
}

type modelPricingProviderStub struct {
	prices map[string]*LiteLLMModelPricing
}

func (s *modelPricingProviderStub) GetModelPricing(model string) *LiteLLMModelPricing {
	return s.prices[model]
}

type modelsListerStub struct {
	models map[int64][]string
	calls  int
}

type channelModelPricingProviderStub struct {
	prices     map[int64]map[string]*ChannelModelPricing
	restricted map[int64]map[string]bool
}

func (s *channelModelPricingProviderStub) GetChannelModelPricing(_ context.Context, groupID int64, model string) *ChannelModelPricing {
	return s.prices[groupID][model]
}

func (s *channelModelPricingProviderStub) IsModelRestricted(_ context.Context, groupID int64, model string) bool {
	return s.restricted[groupID][model]
}

func (s *modelsListerStub) GetAvailableModels(_ context.Context, groupID *int64, _ string) []string {
	s.calls++
	id := int64(0)
	if groupID != nil {
		id = *groupID
	}
	return s.models[id]
}

// --- tests ---

func newModelPricingServiceForTest(groups []Group, prices map[string]*LiteLLMModelPricing, models map[int64][]string) (*ModelPricingService, *modelPricingGroupRepoStub, *modelsListerStub) {
	repo := &modelPricingGroupRepoStub{groups: groups}
	lister := &modelsListerStub{models: models}
	svc := NewModelPricingService(repo, &modelPricingProviderStub{prices: prices}, lister, nil)
	return svc, repo, lister
}

func TestModelPricingPriceFormula(t *testing.T) {
	groups := []Group{{ID: 1, Name: "CodeX Pro 20X 分组", Platform: "openai", RateMultiplier: 0.5}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.4": {
			InputCostPerToken:       2.5e-6, // $2.5 / 1M
			OutputCostPerToken:      1.5e-5, // $15 / 1M
			CacheReadInputTokenCost: 2.5e-7, // $0.25 / 1M
		},
	}
	models := map[int64][]string{1: {"gpt-5.4"}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(catalog.Groups))
	}
	g := catalog.Groups[0]
	if g.GroupID != 1 || g.RateMultiplier != 0.5 {
		t.Fatalf("unexpected group: %+v", g)
	}
	if len(g.Models) != 2 {
		t.Fatalf("expected active GPT-5.4 plus disabled Mini, got %d", len(g.Models))
	}
	m := g.Models[0]
	if m.Model != "gpt-5.4" {
		t.Fatalf("unexpected model: %s", m.Model)
	}
	assertPrice(t, "input", m.InputPrice, 1.25)
	assertPrice(t, "output", m.OutputPrice, 7.5)
	assertPrice(t, "cache_read", m.CacheReadPrice, 0.125)
}

func TestModelPricingUsesGroupChannelOverrideIncludingCacheWrite(t *testing.T) {
	groups := []Group{{ID: 52, Name: "GPT CYBER 分组（特价！）", Platform: "openai", RateMultiplier: 2}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-daybreak-blue-latest": {
			InputCostPerToken:           1.25e-6,
			OutputCostPerToken:          10e-6,
			CacheReadInputTokenCost:     0.125e-6,
			CacheCreationInputTokenCost: 0,
		},
	}
	models := map[int64][]string{52: {"gpt-daybreak-blue-latest"}}
	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	svc.channelPricing = &channelModelPricingProviderStub{prices: map[int64]map[string]*ChannelModelPricing{
		52: {
			"gpt-daybreak-blue-latest": {
				BillingMode:     BillingModeToken,
				InputPrice:      ptr(5e-6),
				OutputPrice:     ptr(30e-6),
				CacheWritePrice: ptr(6.25e-6),
				CacheReadPrice:  ptr(0.25e-6),
			},
		},
	}}

	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 || len(catalog.Groups[0].Models) != 1 {
		t.Fatalf("unexpected catalog: %+v", catalog.Groups)
	}
	m := catalog.Groups[0].Models[0]
	assertPrice(t, "input", m.InputPrice, 10)
	assertPrice(t, "output", m.OutputPrice, 60)
	assertPrice(t, "cache_write", m.CacheWritePrice, 12.5)
	assertPrice(t, "cache_read", m.CacheReadPrice, 0.5)
	if m.LongContext == nil {
		t.Fatal("expected Daybreak long-context pricing disclosure")
	}
	if m.LongContext.InputThreshold != 272000 || m.LongContext.InputMultiplier != 2 || m.LongContext.OutputMultiplier != 1.5 {
		t.Fatalf("unexpected long-context pricing disclosure: %+v", m.LongContext)
	}
}

func TestModelPricingOmitsModelsRestrictedByFamilyChannel(t *testing.T) {
	groups := []Group{{ID: 60, Name: "阿里云官方 GLM 分组", Platform: "openai", RateMultiplier: 1}}
	models := map[int64][]string{60: {"glm-5.3", "glm-5.2", "kimi-k3", "qwen3.8-max"}}
	svc, _, _ := newModelPricingServiceForTest(groups, map[string]*LiteLLMModelPricing{
		"glm-5.3":     {InputCostPerToken: 8e-6, OutputCostPerToken: 28e-6},
		"glm-5.2":     {InputCostPerToken: 8e-6, OutputCostPerToken: 28e-6},
		"kimi-k3":     {InputCostPerToken: 20e-6, OutputCostPerToken: 100e-6},
		"qwen3.8-max": {InputCostPerToken: 12e-6, OutputCostPerToken: 36e-6},
	}, models)
	svc.channelPricing = &channelModelPricingProviderStub{
		prices: map[int64]map[string]*ChannelModelPricing{60: {
			"glm-5.3": {BillingMode: BillingModeToken, InputPrice: ptr(8e-6), OutputPrice: ptr(28e-6)},
			"glm-5.2": {BillingMode: BillingModeToken, InputPrice: ptr(8e-6), OutputPrice: ptr(28e-6)},
		}},
		restricted: map[int64]map[string]bool{60: {"kimi-k3": true, "qwen3.8-max": true}},
	}

	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	require.Len(t, catalog.Groups, 1)
	require.Equal(t, []string{"glm-5.3", "glm-5.2"}, []string{
		catalog.Groups[0].Models[0].Model,
		catalog.Groups[0].Models[1].Model,
	})
}

func TestModelPricingPublishesContextIntervalsAndTimeSchedule(t *testing.T) {
	groups := []Group{{ID: 6, Name: "按量 GPT", Platform: "openai", RateMultiplier: 2}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.6-sol": {InputCostPerToken: 5e-6, OutputCostPerToken: 30e-6},
	}
	models := map[int64][]string{6: {"gpt-5.6-sol"}}
	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	max := 272000
	svc.channelPricing = &channelModelPricingProviderStub{prices: map[int64]map[string]*ChannelModelPricing{
		6: {
			"gpt-5.6-sol": {
				BillingMode: BillingModeToken,
				Intervals:   []PricingInterval{{MinTokens: 0, MaxTokens: &max, InputPrice: ptr(4e-6), OutputPrice: ptr(20e-6)}},
				TimePricing: &ChannelTimePricing{
					Timezone: "Asia/Shanghai",
					Periods:  []ChannelTimePricingPeriod{{StartTime: "09:00", EndTime: "12:00", Multiplier: 1.5}},
				},
			},
		},
	}}

	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	model := catalog.Groups[0].Models[0]
	if model.LongContext != nil {
		t.Fatalf("explicit context intervals must replace built-in long context disclosure: %+v", model.LongContext)
	}
	if len(model.ContextIntervals) != 1 {
		t.Fatalf("expected one context interval, got %+v", model.ContextIntervals)
	}
	assertPrice(t, "interval input", model.ContextIntervals[0].InputPrice, 8)
	assertPrice(t, "interval output", model.ContextIntervals[0].OutputPrice, 40)
	if model.TimePricing == nil || model.TimePricing.Timezone != "Asia/Shanghai" || model.TimePricing.Periods[0].Multiplier != 1.5 {
		t.Fatalf("missing time pricing disclosure: %+v", model.TimePricing)
	}
}

func TestModelPricingFiltersInternalGroups(t *testing.T) {
	groups := []Group{
		{ID: 46, Name: "测试专用月卡 · GPT", Platform: "openai", RateMultiplier: 0.5},
		{ID: 47, Name: "测试专用月卡 · Claude", Platform: "anthropic", RateMultiplier: 2.4},
		{ID: 99, Name: "内部测试组", Platform: "openai", RateMultiplier: 1},
		{ID: 6, Name: "CodeX Pro 20X 分组", Platform: "openai", RateMultiplier: 0.5},
	}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.4": {InputCostPerToken: 2.5e-6, OutputCostPerToken: 1.5e-5, CacheReadInputTokenCost: 2.5e-7},
	}
	models := map[int64][]string{46: {"gpt-5.4"}, 47: {"gpt-5.4"}, 99: {"gpt-5.4"}, 6: {"gpt-5.4"}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 {
		t.Fatalf("expected only the public group, got %d groups", len(catalog.Groups))
	}
	if catalog.Groups[0].GroupID != 6 {
		t.Fatalf("expected group 6 to survive, got %d", catalog.Groups[0].GroupID)
	}
}

func TestModelPricingShowsMonthlyCardGroups(t *testing.T) {
	// 旧倍率月卡组（Lite/Pro/Apex）从价格页下线；统一倍率月卡组
	// （GPT ×0.5 / Grok ×0.4 / Claude ×2.4，Plus/Pro/Max 系列）继续公开，
	// 由前端归入厂商分块和「Builder Pass 月卡」分块同时展示。
	groups := []Group{
		{ID: 7, Name: "GPT Lite 月卡组", Platform: "openai", RateMultiplier: 0.3774, SubscriptionType: SubscriptionTypeCredit},
		{ID: 40, Name: "GPT Plus 月卡组", Platform: "openai", RateMultiplier: 0.5, SubscriptionType: SubscriptionTypeCredit},
	}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.4": {InputCostPerToken: 2.5e-6, OutputCostPerToken: 1.5e-5, CacheReadInputTokenCost: 2.5e-7},
	}
	models := map[int64][]string{7: {"gpt-5.4"}, 40: {"gpt-5.4"}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 {
		t.Fatalf("expected only the unified-rate monthly card group, got %d groups", len(catalog.Groups))
	}
	if catalog.Groups[0].GroupID != 40 {
		t.Fatalf("expected GPT Plus 月卡组 (id=40) to survive, got %d", catalog.Groups[0].GroupID)
	}
	if catalog.Groups[0].SubscriptionType != SubscriptionTypeCredit {
		t.Fatalf("expected subscription_type=credit, got %q", catalog.Groups[0].SubscriptionType)
	}
}

func TestModelPricingStripsV3FromGroupName(t *testing.T) {
	groups := []Group{{ID: 42, Name: "GPT Pro V3 月卡组", Platform: "openai", RateMultiplier: 0.5}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.4": {InputCostPerToken: 2.5e-6, OutputCostPerToken: 1.5e-5, CacheReadInputTokenCost: 2.5e-7},
	}
	models := map[int64][]string{42: {"gpt-5.4"}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(catalog.Groups))
	}
	if catalog.Groups[0].Name != "GPT Pro 月卡组" {
		t.Fatalf("expected display name 'GPT Pro 月卡组', got %q", catalog.Groups[0].Name)
	}
}

func TestModelPricingHidesGPT56(t *testing.T) {
	groups := []Group{{ID: 6, Name: "CodeX Pro 20X 分组", Platform: "openai", RateMultiplier: 0.5}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.6":       {InputCostPerToken: 1.25e-6, OutputCostPerToken: 1e-5},
		"gpt-5.6-sol":   {InputCostPerToken: 5e-6, OutputCostPerToken: 3e-5},
		"gpt-5.6-terra": {InputCostPerToken: 2e-6, OutputCostPerToken: 1.2e-5},
		"gpt-5.6-luna":  {InputCostPerToken: 2e-7, OutputCostPerToken: 1.2e-6},
	}
	models := map[int64][]string{6: {"gpt-5.6", "gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := make([]string, 0, 3)
	for _, m := range catalog.Groups[0].Models {
		got = append(got, m.Model)
	}
	for _, name := range got {
		if name == "gpt-5.6" {
			t.Fatalf("gpt-5.6 should be hidden from display, got %v", got)
		}
	}
	want := []string{"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
	if !catalog.Groups[0].Models[2].Disabled {
		t.Fatal("expected gpt-5.6-luna to be marked disabled")
	}
}

func TestModelPricingAddsDisabledLunaWhenRoutingNoLongerExposesIt(t *testing.T) {
	groups := []Group{{ID: 6, Name: "CodeX Pro 20X 分组", Platform: "openai", RateMultiplier: 0.5}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.6-sol":   {InputCostPerToken: 5e-6, OutputCostPerToken: 30e-6, CacheReadInputTokenCost: 0.5e-6},
		"gpt-5.6-terra": {InputCostPerToken: 2.5e-6, OutputCostPerToken: 15e-6, CacheReadInputTokenCost: 0.25e-6},
		"gpt-5.6-luna":  {InputCostPerToken: 1e-6, OutputCostPerToken: 6e-6, CacheReadInputTokenCost: 0.1e-6},
	}
	models := map[int64][]string{6: {"gpt-5.6-sol", "gpt-5.6-terra"}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 || len(catalog.Groups[0].Models) != 3 {
		t.Fatalf("expected sol, terra and disabled luna, got %+v", catalog.Groups)
	}
	luna := catalog.Groups[0].Models[2]
	if luna.Model != "gpt-5.6-luna" || !luna.Disabled {
		t.Fatalf("expected disabled Luna row, got %+v", luna)
	}
	assertPrice(t, "luna input", luna.InputPrice, 0.5)
	assertPrice(t, "luna output", luna.OutputPrice, 3)
	assertPrice(t, "luna cache", luna.CacheReadPrice, 0.05)
}

func TestModelPricingAddsDisabledLunaWithoutProviderPrice(t *testing.T) {
	groups := []Group{{ID: 6, Name: "CodeX Pro 20X 分组", Platform: "openai", RateMultiplier: 0.5}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.6-sol": {InputCostPerToken: 5e-6, OutputCostPerToken: 30e-6},
	}
	models := map[int64][]string{6: {"gpt-5.6-sol"}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 || len(catalog.Groups[0].Models) != 2 {
		t.Fatalf("expected Sol and disabled Luna, got %+v", catalog.Groups)
	}
	luna := catalog.Groups[0].Models[1]
	if luna.Model != "gpt-5.6-luna" || !luna.Disabled {
		t.Fatalf("expected disabled Luna row, got %+v", luna)
	}
	if luna.InputPrice != nil || luna.OutputPrice != nil || luna.CacheReadPrice != nil {
		t.Fatalf("expected unavailable Luna price to remain empty, got %+v", luna)
	}
}

func TestModelPricingHidesOpenAICompactVariants(t *testing.T) {
	groups := []Group{{ID: 52, Name: "GPT CYBER 分组（特价！）", Platform: "openai", RateMultiplier: 0.5}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.6-sol":                {InputCostPerToken: 5e-6, OutputCostPerToken: 30e-6, CacheReadInputTokenCost: 0.5e-6},
		"gpt-5.6-sol-openai-compact": {InputCostPerToken: 5e-6, OutputCostPerToken: 30e-6, CacheReadInputTokenCost: 0.5e-6},
		"gpt-5.5":                    {InputCostPerToken: 2.5e-6, OutputCostPerToken: 15e-6, CacheReadInputTokenCost: 0.25e-6},
		"gpt-5.5-openai-compact":     {InputCostPerToken: 2.5e-6, OutputCostPerToken: 15e-6, CacheReadInputTokenCost: 0.25e-6},
		"codex-auto-review":          {InputCostPerToken: 2.5e-6, OutputCostPerToken: 15e-6, CacheReadInputTokenCost: 0.25e-6},
	}
	models := map[int64][]string{52: {
		"gpt-5.6-sol",
		"gpt-5.6-sol-openai-compact",
		"gpt-5.5",
		"gpt-5.5-openai-compact",
		"codex-auto-review",
	}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 {
		t.Fatalf("expected 1 group, got %+v", catalog.Groups)
	}
	got := make([]string, 0, len(catalog.Groups[0].Models))
	for _, m := range catalog.Groups[0].Models {
		got = append(got, m.Model)
		if IsInternalOnlyModel(m.Model) {
			t.Fatalf("internal-only model %s must not appear in public pricing", m.Model)
		}
	}
	for _, base := range []string{"gpt-5.6-sol", "gpt-5.5"} {
		found := false
		for _, name := range got {
			if name == base {
				found = true
			}
		}
		if !found {
			t.Fatalf("base model %s missing from public pricing, got %v", base, got)
		}
	}
}

func TestModelPricingAddsDisabledGPT54MiniWhenRoutingNoLongerExposesIt(t *testing.T) {
	groups := []Group{{ID: 6, Name: "CodeX Pro 20X 分组", Platform: "openai", RateMultiplier: 0.5}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.4":      {InputCostPerToken: 2.5e-6, OutputCostPerToken: 15e-6, CacheReadInputTokenCost: 0.25e-6},
		"gpt-5.4-mini": {InputCostPerToken: 0.8e-6, OutputCostPerToken: 3.2e-6, CacheReadInputTokenCost: 0.08e-6},
	}
	models := map[int64][]string{6: {"gpt-5.4"}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 || len(catalog.Groups[0].Models) != 2 {
		t.Fatalf("expected GPT-5.4 and disabled Mini, got %+v", catalog.Groups)
	}
	mini := catalog.Groups[0].Models[1]
	if mini.Model != "gpt-5.4-mini" || !mini.Disabled {
		t.Fatalf("expected disabled GPT-5.4 Mini row, got %+v", mini)
	}
	assertPrice(t, "mini input", mini.InputPrice, 0.4)
	assertPrice(t, "mini output", mini.OutputPrice, 1.6)
	assertPrice(t, "mini cache", mini.CacheReadPrice, 0.04)
}

func TestModelPricingExemptedGroupShowsRetiredModelsAsAvailable(t *testing.T) {
	groups := []Group{
		{ID: 6, Name: "CodeX Pro 20X 分组", Platform: "openai", RateMultiplier: 0.5},
		{ID: 52, Name: "GPT CYBER 分组（特价！）", Platform: "openai", RateMultiplier: 2},
		{ID: 59, Name: "CodeX 企业级分组", Platform: "openai", RateMultiplier: 0.5},
	}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.6-sol":   {InputCostPerToken: 5e-6, OutputCostPerToken: 30e-6, CacheReadInputTokenCost: 0.5e-6},
		"gpt-5.6-terra": {InputCostPerToken: 2.5e-6, OutputCostPerToken: 15e-6, CacheReadInputTokenCost: 0.25e-6},
		"gpt-5.6-luna":  {InputCostPerToken: 1e-6, OutputCostPerToken: 6e-6, CacheReadInputTokenCost: 0.1e-6},
		"gpt-5.4":       {InputCostPerToken: 2.5e-6, OutputCostPerToken: 15e-6, CacheReadInputTokenCost: 0.25e-6},
		"gpt-5.4-mini":  {InputCostPerToken: 0.8e-6, OutputCostPerToken: 3.2e-6, CacheReadInputTokenCost: 0.08e-6},
	}
	models := map[int64][]string{
		6:  {"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.4", "gpt-5.4-mini"},
		52: {"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.4", "gpt-5.4-mini"},
		59: {"gpt-5.6-sol", "gpt-5.6-terra", "gpt-5.6-luna", "gpt-5.4", "gpt-5.4-mini"},
	}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 3 {
		t.Fatalf("expected 3 groups, got %+v", catalog.Groups)
	}
	byID := make(map[int64]PublicModelPricingGroup, 3)
	for _, g := range catalog.Groups {
		byID[g.GroupID] = g
	}
	for groupID, wantDisabled := range map[int64]bool{6: true, 52: false, 59: false} {
		g := byID[groupID]
		for _, name := range []string{"gpt-5.6-luna", "gpt-5.4-mini"} {
			var row *PublicModelPrice
			for i := range g.Models {
				if g.Models[i].Model == name {
					row = &g.Models[i]
					break
				}
			}
			if row == nil {
				t.Fatalf("group %d missing %s row: %+v", groupID, name, g.Models)
			}
			if row.Disabled != wantDisabled {
				t.Fatalf("group %d %s Disabled=%v, want %v", groupID, name, row.Disabled, wantDisabled)
			}
		}
	}
}

func TestModelPricingExemptedGroupSkipsSynthesizedDisabledRow(t *testing.T) {
	// Group 59 does not expose luna through routing; the exemption must not
	// synthesize a struck-through luna row for it, while group 6 still gets one.
	groups := []Group{
		{ID: 6, Name: "CodeX Pro 20X 分组", Platform: "openai", RateMultiplier: 0.5},
		{ID: 59, Name: "CodeX 企业级分组", Platform: "openai", RateMultiplier: 0.5},
	}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.6-sol":   {InputCostPerToken: 5e-6, OutputCostPerToken: 30e-6, CacheReadInputTokenCost: 0.5e-6},
		"gpt-5.6-terra": {InputCostPerToken: 2.5e-6, OutputCostPerToken: 15e-6, CacheReadInputTokenCost: 0.25e-6},
		"gpt-5.6-luna":  {InputCostPerToken: 1e-6, OutputCostPerToken: 6e-6, CacheReadInputTokenCost: 0.1e-6},
	}
	models := map[int64][]string{
		6:  {"gpt-5.6-sol", "gpt-5.6-terra"},
		59: {"gpt-5.6-sol", "gpt-5.6-terra"},
	}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	byID := make(map[int64]PublicModelPricingGroup, 2)
	for _, g := range catalog.Groups {
		byID[g.GroupID] = g
	}
	var luna6 *PublicModelPrice
	for i := range byID[6].Models {
		if byID[6].Models[i].Model == "gpt-5.6-luna" {
			luna6 = &byID[6].Models[i]
		}
	}
	if luna6 == nil || !luna6.Disabled {
		t.Fatalf("group 6 should still get a disabled luna row, got %+v", byID[6].Models)
	}
	for _, m := range byID[59].Models {
		if m.Model == "gpt-5.6-luna" {
			t.Fatalf("group 59 must not get a synthesized luna row, got %+v", byID[59].Models)
		}
	}
}

func TestModelPricingPublishesGPTImage2ModalPricesAtImageMultiplier(t *testing.T) {
	groups := []Group{{
		ID:                   51,
		Name:                 "GPT Image 2 生图分组",
		Description:          "支持 quality、size、output_format 等参数。",
		Platform:             "openai",
		RateMultiplier:       4,
		AllowImageGeneration: true,
		ImageRateIndependent: true,
		ImageRateMultiplier:  4,
	}}
	prices := map[string]*LiteLLMModelPricing{
		// 故意放入错误/过期的通用价，验证页面不会再走 LiteLLM 三列。
		"gpt-image-2": {InputCostPerToken: 1.25e-6, OutputCostPerToken: 10e-6, CacheReadInputTokenCost: 0.125e-6},
	}
	models := map[int64][]string{51: {"gpt-image-2"}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 {
		t.Fatalf("expected image-only group to remain visible, got %+v", catalog.Groups)
	}
	group := catalog.Groups[0]
	if group.Models == nil {
		t.Fatal("image-only group models must be an empty array, not nil")
	}
	if len(group.Models) != 0 {
		t.Fatalf("gpt-image-2 must not be rendered as a generic text model: %+v", group.Models)
	}
	encoded, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("marshal image-only group: %v", err)
	}
	if !strings.Contains(string(encoded), `"models":[]`) {
		t.Fatalf("image-only group must serialize models as []: %s", encoded)
	}
	if group.Description != groups[0].Description || group.ImageGeneration == nil {
		t.Fatalf("missing image metadata: %+v", group)
	}
	image := group.ImageGeneration
	if image.Mode != "token" {
		t.Fatalf("expected token image billing, got %q", image.Mode)
	}
	assertPrice(t, "text input", image.TextInputPrice, 20)
	assertPrice(t, "text cached input", image.TextCachedInputPrice, 5)
	assertPrice(t, "image input", image.ImageInputPrice, 32)
	assertPrice(t, "image cached input", image.ImageCachedInputPrice, 8)
	assertPrice(t, "image output", image.ImageOutputPrice, 120)
}

func TestModelPricingPublishesFixedSuccessfulImagePrice(t *testing.T) {
	price := 0.3
	groups := []Group{{
		ID:                   6,
		Name:                 "CodeX Pro 20X 分组",
		Platform:             "openai",
		RateMultiplier:       0.5,
		AllowImageGeneration: true,
		ImageRateIndependent: true,
		ImageRateMultiplier:  1,
		GPTImageCallPrice:    &price,
	}}
	models := map[int64][]string{6: {"gpt-image-2"}}

	svc, _, _ := newModelPricingServiceForTest(groups, nil, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 || catalog.Groups[0].ImageGeneration == nil {
		t.Fatalf("expected fixed image pricing group, got %+v", catalog.Groups)
	}
	image := catalog.Groups[0].ImageGeneration
	if image.Mode != "fixed_per_image" {
		t.Fatalf("expected fixed_per_image, got %q", image.Mode)
	}
	assertPrice(t, "fixed image", image.PricePerImage, 0.3)
	if len(catalog.Groups[0].Models) != 0 {
		t.Fatalf("expected no duplicate generic image row, got %+v", catalog.Groups[0].Models)
	}
}

func TestModelPricingSortsGPTNewestFirst(t *testing.T) {
	groups := []Group{{ID: 6, Name: "CodeX Pro 20X 分组", Platform: "openai", RateMultiplier: 0.5}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.4-mini": {InputCostPerToken: 7.5e-7, OutputCostPerToken: 4.5e-6},
		"gpt-5.4":      {InputCostPerToken: 2.5e-6, OutputCostPerToken: 1.5e-5},
		"gpt-5.5":      {InputCostPerToken: 5e-6, OutputCostPerToken: 3e-5},
		"gpt-5.6-sol":  {InputCostPerToken: 5e-6, OutputCostPerToken: 3e-5},
		"gpt-5.6-luna": {InputCostPerToken: 2e-7, OutputCostPerToken: 1.2e-6},
	}
	// 故意乱序
	models := map[int64][]string{6: {"gpt-5.4-mini", "gpt-5.5", "gpt-5.6-luna", "gpt-5.6-sol", "gpt-5.4"}}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := make([]string, 0, 5)
	for _, m := range catalog.Groups[0].Models {
		got = append(got, m.Model)
	}
	want := []string{"gpt-5.6-sol", "gpt-5.6-luna", "gpt-5.5", "gpt-5.4", "gpt-5.4-mini"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("GPT sort: expected %v, got %v", want, got)
		}
	}
}

func TestModelPricingSortsClaudeFamilyThenVersion(t *testing.T) {
	groups := []Group{{ID: 5, Name: "Claude MAX 20X 分组", Platform: "anthropic", RateMultiplier: 2.4}}
	prices := map[string]*LiteLLMModelPricing{}
	modelsList := []string{
		"claude-sonnet-5", "claude-opus-4-7", "claude-fable-5", "claude-opus-5",
		"claude-sonnet-4-5", "claude-haiku-4-5", "claude-opus-4-5", "claude-opus-4-8",
		"claude-sonnet-4-6", "claude-opus-4-6",
	}
	for _, m := range modelsList {
		prices[m] = &LiteLLMModelPricing{InputCostPerToken: 5e-6, OutputCostPerToken: 2.5e-5}
	}
	models := map[int64][]string{5: modelsList}

	svc, _, _ := newModelPricingServiceForTest(groups, prices, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := make([]string, 0, len(modelsList))
	for _, m := range catalog.Groups[0].Models {
		got = append(got, m.Model)
	}
	want := []string{
		"claude-fable-5", "claude-opus-5", "claude-opus-4-8", "claude-opus-4-7",
		"claude-opus-4-6", "claude-opus-4-5", "claude-sonnet-5", "claude-sonnet-4-6",
		"claude-sonnet-4-5", "claude-haiku-4-5",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Claude sort: expected %v, got %v", want, got)
		}
	}
}

func TestModelPricingManualFallback(t *testing.T) {
	groups := []Group{{ID: 34, Name: "Grok 4.5 分组", Platform: "anthropic", RateMultiplier: 0.4}}
	// grok-4.5 不在 LiteLLM 价表里（prices 无此 key），走手动价表：$2 / $6 / 缓存 $0.3
	models := map[int64][]string{34: {"grok-4.5"}}

	svc, _, _ := newModelPricingServiceForTest(groups, nil, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 || len(catalog.Groups[0].Models) != 1 {
		t.Fatalf("expected 1 group with 1 model, got %d/%d", len(catalog.Groups), len(catalog.Groups[0].Models))
	}
	m := catalog.Groups[0].Models[0]
	assertPrice(t, "input", m.InputPrice, 0.8)           // 2.0 × 0.4
	assertPrice(t, "output", m.OutputPrice, 2.4)         // 6.0 × 0.4
	assertPrice(t, "cache_read", m.CacheReadPrice, 0.12) // 0.3 × 0.4
}

func TestModelPricingGrok46UsesVerifiedPomoRateCard(t *testing.T) {
	groups := []Group{{ID: 34, Name: "Grok 4.6 分组", Platform: "grok", RateMultiplier: 0.4}}
	models := map[int64][]string{34: {"grok-4.6"}}

	svc, _, _ := newModelPricingServiceForTest(groups, nil, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 || len(catalog.Groups[0].Models) != 1 {
		t.Fatalf("expected 1 group with 1 model, got %+v", catalog.Groups)
	}
	m := catalog.Groups[0].Models[0]
	if m.Model != "grok-4.6" {
		t.Fatalf("expected grok-4.6, got %+v", m)
	}
	assertPrice(t, "input", m.InputPrice, 0.8)          // 2.0 x 0.4
	assertPrice(t, "output", m.OutputPrice, 2.4)        // 6.0 x 0.4
	assertPrice(t, "cache_read", m.CacheReadPrice, 0.2) // 0.5 x 0.4
}

func TestModelPricingSkipsUnknownModel(t *testing.T) {
	groups := []Group{{ID: 34, Name: "Grok 4.5 分组", Platform: "anthropic", RateMultiplier: 0.4}}
	models := map[int64][]string{34: {"grok-4.5", "totally-unknown-model"}}

	svc, _, _ := newModelPricingServiceForTest(groups, nil, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(catalog.Groups))
	}
	if len(catalog.Groups[0].Models) != 1 || catalog.Groups[0].Models[0].Model != "grok-4.5" {
		t.Fatalf("expected only grok-4.5 to survive, got %+v", catalog.Groups[0].Models)
	}
}

func TestModelPricingGroupWithoutPricedModelsDropped(t *testing.T) {
	groups := []Group{{ID: 123, Name: "按量分组", Platform: "openai", RateMultiplier: 0.5}}
	models := map[int64][]string{123: {"unknown-model-1"}}

	svc, _, _ := newModelPricingServiceForTest(groups, nil, models)
	catalog, err := svc.GetPublicModelPricing(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(catalog.Groups) != 0 {
		t.Fatalf("expected no groups (no priced models), got %d", len(catalog.Groups))
	}
}

func TestModelPricingCatalogCache(t *testing.T) {
	groups := []Group{{ID: 6, Name: "CodeX Pro 20X 分组", Platform: "openai", RateMultiplier: 0.5}}
	prices := map[string]*LiteLLMModelPricing{
		"gpt-5.4": {InputCostPerToken: 2.5e-6, OutputCostPerToken: 1.5e-5, CacheReadInputTokenCost: 2.5e-7},
	}
	models := map[int64][]string{6: {"gpt-5.4"}}

	svc, repo, lister := newModelPricingServiceForTest(groups, prices, models)
	ctx := context.Background()
	if _, err := svc.GetPublicModelPricing(ctx); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := svc.GetPublicModelPricing(ctx); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if lister.calls != 1 {
		t.Fatalf("expected lister called once (cached), got %d", lister.calls)
	}
	if repo.listActiveCalls != 1 {
		t.Fatalf("expected ListActive called once (cached), got %d", repo.listActiveCalls)
	}
}

func assertPrice(t *testing.T, field string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s: expected non-nil price, got nil", field)
	}
	if *got != want {
		t.Fatalf("%s: expected %v, got %v", field, want, *got)
	}
}

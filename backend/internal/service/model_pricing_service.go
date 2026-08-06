package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	gocache "github.com/patrickmn/go-cache"
)

// AvailableModelsLister 列出某个分组当前可用的模型名（绑定账号 model_mapping 的并集）。
type AvailableModelsLister interface {
	GetAvailableModels(ctx context.Context, groupID *int64, platform string) []string
}

// ModelPricingProvider 返回某个模型的官方 LiteLLM 定价（USD/token）。
type ModelPricingProvider interface {
	GetModelPricing(model string) *LiteLLMModelPricing
}

// catalogCacheKey 公开模型价格目录的缓存 key。
const catalogCacheKey = "public-model-pricing-catalog"

// defaultCatalogCacheTTL 目录缓存时长：底层 GetAvailableModels 只缓存 15s，
// 公开页面会被爬虫/访客高频命中，必须加这一层缓存避免打爆 DB。
const defaultCatalogCacheTTL = 5 * time.Minute

// defaultHiddenPublicModelPricingGroupIDs 默认不在公开价格页展示的分组。
// 运营要求：内部测试分组 + 一部分月卡基础档位（Lite/Pro/Apex）不上价格页。
// 如需调整，直接增删这里的 ID；后续可改为 DB 设置 model_pricing_hidden_group_ids 免发版维护。
var defaultHiddenPublicModelPricingGroupIDs = map[int64]struct{}{
	46: {}, // 测试专用月卡 · GPT
	47: {}, // 测试专用月卡 · Claude
	7:  {}, // GPT Lite 月卡组
	8:  {}, // GPT Pro 月卡组
	18: {}, // GPT Apex 月卡组
	11: {}, // Claude Lite 月卡组
	12: {}, // Claude Pro 月卡组
	19: {}, // Claude Apex 月卡组
	35: {}, // Grok Lite 月卡组
	36: {}, // Grok Pro 月卡组
	39: {}, // Grok Apex 月卡组
}

// manualOfficialPrices 手动维护的官方价表（USD per 1M tokens）。
// 这些模型不在 LiteLLM 官方价表（model_prices_and_context_window.json）里，
// 但分组 model_mapping 会暴露它们。价格乘以分组倍率得到客户实付价（元/1M）。
// 所有价格取自对应厂商官方定价页（2026-08 核对）：
//   - grok-4.5:        xAI 官方 https://docs.x.ai/docs/models（<200k 档）
//   - glm-5.2:         Z.ai 官方 https://docs.z.ai/guides/overview/pricing.md
//   - claude-opus-5:   Anthropic 官方 https://platform.claude.com/docs/en/about-claude/models/overview
//   - claude-sonnet-5: 同上；注意 Sonnet 5 目前处于官方促销价 $2/$10，至 2026-08-31 到期，
//     之后恢复标准价 $3/$15（届时需更新本表）
//
// 缓存读取按对应厂商规则：Anthropic 为输入价的 10%。
type manualOfficialPrice struct {
	input     float64
	output    float64
	cacheRead float64
}

var manualOfficialPrices = map[string]manualOfficialPrice{
	"grok-4.5":        {input: 2.0, output: 6.0, cacheRead: 0.3},
	"glm-5.2":         {input: 1.4, output: 4.4, cacheRead: 0.26},
	"claude-opus-5":   {input: 5.0, output: 25.0, cacheRead: 0.5},
	"claude-sonnet-5": {input: 2.0, output: 10.0, cacheRead: 0.2}, // 官方促销价，2026-08-31 后改回 $3/$15
}

// ModelPricingService 聚合"全部 active 分组 × 分组内可用模型 × 官方价"生成公开价格目录。
type ModelPricingService struct {
	groupRepo       GroupRepository
	pricing         ModelPricingProvider
	modelsLister    AvailableModelsLister
	catalogCache    *gocache.Cache
	catalogCacheTTL time.Duration
}

// NewModelPricingService 创建 ModelPricingService。
// pricing / modelsLister 用窄接口注入，便于单元测试。
func NewModelPricingService(
	groupRepo GroupRepository,
	pricing ModelPricingProvider,
	modelsLister AvailableModelsLister,
) *ModelPricingService {
	return &ModelPricingService{
		groupRepo:       groupRepo,
		pricing:         pricing,
		modelsLister:    modelsLister,
		catalogCache:    gocache.New(defaultCatalogCacheTTL, time.Minute),
		catalogCacheTTL: defaultCatalogCacheTTL,
	}
}

// PublicModelPricingCatalog 公开价格目录的响应体。
type PublicModelPricingCatalog struct {
	UpdatedAt time.Time                 `json:"updated_at"`
	Currency  string                    `json:"currency"`
	Unit      string                    `json:"unit"`
	Groups    []PublicModelPricingGroup `json:"groups"`
}

// PublicModelPricingGroup 单个分组的模型价格。
type PublicModelPricingGroup struct {
	GroupID          int64              `json:"group_id"`
	Name             string             `json:"name"`
	Platform         string             `json:"platform"`
	RateMultiplier   float64            `json:"rate_multiplier"`
	IsExclusive      bool               `json:"is_exclusive"`
	SubscriptionType string             `json:"subscription_type"`
	Models           []PublicModelPrice `json:"models"`
}

// PublicModelPrice 单个模型的实付价（元/1M tokens），价格未知时为 nil。
type PublicModelPrice struct {
	Model          string   `json:"model"`
	InputPrice     *float64 `json:"input_price"`
	OutputPrice    *float64 `json:"output_price"`
	CacheReadPrice *float64 `json:"cache_read_price"`
}

// GetPublicModelPricing 返回全部 active 分组（排除内部测试分组）的模型价格目录。
// 客户实付价 = 官方 USD/1M × 分组倍率（充值 1 元 = 1 USD 额度，固定 1:1，不乘汇率）。
func (s *ModelPricingService) GetPublicModelPricing(ctx context.Context) (*PublicModelPricingCatalog, error) {
	if cached, found := s.catalogCache.Get(catalogCacheKey); found {
		if catalog, ok := cached.(*PublicModelPricingCatalog); ok {
			return cloneCatalog(catalog), nil
		}
	}

	groups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active groups: %w", err)
	}

	catalog := &PublicModelPricingCatalog{
		UpdatedAt: time.Now().UTC(),
		Currency:  "CNY",
		Unit:      "per_1m_tokens",
		Groups:    make([]PublicModelPricingGroup, 0, len(groups)),
	}

	for _, g := range groups {
		if s.isInternalGroup(g) {
			continue
		}
		groupID := g.ID
		models := s.modelsLister.GetAvailableModels(ctx, &groupID, "")
		if len(models) == 0 {
			continue
		}
		prices := make([]PublicModelPrice, 0, len(models))
		for _, model := range models {
			price, ok := s.priceForModel(model, g.RateMultiplier)
			if !ok {
				slog.Debug("model_pricing: skip model without official price",
					"group", g.Name, "model", model)
				continue
			}
			prices = append(prices, price)
		}
		if len(prices) == 0 {
			continue
		}
		catalog.Groups = append(catalog.Groups, PublicModelPricingGroup{
			GroupID:          g.ID,
			Name:             publicGroupDisplayName(g.Name),
			Platform:         g.Platform,
			RateMultiplier:   g.RateMultiplier,
			IsExclusive:      g.IsExclusive,
			SubscriptionType: g.SubscriptionType,
			Models:           prices,
		})
	}

	sort.Slice(catalog.Groups, func(i, j int) bool {
		a, b := catalog.Groups[i], catalog.Groups[j]
		if a.Platform != b.Platform {
			return a.Platform < b.Platform
		}
		return a.GroupID < b.GroupID
	})

	s.catalogCache.Set(catalogCacheKey, catalog, s.catalogCacheTTL)
	return cloneCatalog(catalog), nil
}

// priceForModel 计算某个模型在给定分组倍率下的实付价。
// 查询顺序：LiteLLM 官方价 → 手动维护价表；都没有则返回 false。
func (s *ModelPricingService) priceForModel(model string, rateMultiplier float64) (PublicModelPrice, bool) {
	if p := s.pricing.GetModelPricing(model); p != nil {
		return PublicModelPrice{
			Model:          model,
			InputPrice:     ptr(round4(p.InputCostPerToken * 1e6 * rateMultiplier)),
			OutputPrice:    ptr(round4(p.OutputCostPerToken * 1e6 * rateMultiplier)),
			CacheReadPrice: ptr(round4(p.CacheReadInputTokenCost * 1e6 * rateMultiplier)),
		}, true
	}

	if mp, ok := manualOfficialPrices[strings.ToLower(model)]; ok {
		return PublicModelPrice{
			Model:          model,
			InputPrice:     ptr(round4(mp.input * rateMultiplier)),
			OutputPrice:    ptr(round4(mp.output * rateMultiplier)),
			CacheReadPrice: ptr(round4(mp.cacheRead * rateMultiplier)),
		}, true
	}

	return PublicModelPrice{}, false
}

// publicGroupDisplayName 对外展示的分组名：去掉内部版本后缀 V3
// （如 "GPT Pro V3 月卡组" → "GPT Pro 月卡组"），并折叠多余空白。
func publicGroupDisplayName(name string) string {
	name = strings.ReplaceAll(name, "V3", "")
	return strings.Join(strings.Fields(name), " ")
}

// isInternalGroup 判断分组是否为内部测试分组（公开价格页不展示）。
func (s *ModelPricingService) isInternalGroup(g Group) bool {
	if _, ok := defaultHiddenPublicModelPricingGroupIDs[g.ID]; ok {
		return true
	}
	name := strings.ToLower(g.Name)
	for _, marker := range []string{"测试", "内部", "test", "internal"} {
		if strings.Contains(name, marker) {
			return true
		}
	}
	return false
}

func ptr(v float64) *float64 {
	return &v
}

func cloneCatalog(c *PublicModelPricingCatalog) *PublicModelPricingCatalog {
	if c == nil {
		return nil
	}
	out := *c
	out.Groups = make([]PublicModelPricingGroup, len(c.Groups))
	for i, g := range c.Groups {
		out.Groups[i] = g
		out.Groups[i].Models = append([]PublicModelPrice(nil), g.Models...)
	}
	return &out
}

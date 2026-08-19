package service

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strconv"
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

// ChannelModelPricingProvider returns the same group-bound override used by
// billing. The public price page must not advertise a stale catalog price when
// a channel rate card is authoritative for the group.
type ChannelModelPricingProvider interface {
	GetChannelModelPricing(ctx context.Context, groupID int64, model string) *ChannelModelPricing
}

// catalogCacheKey 公开模型价格目录的缓存 key。
const catalogCacheKey = "public-model-pricing-catalog"

// defaultCatalogCacheTTL 目录缓存时长：底层 GetAvailableModels 只缓存 15s，
// 公开页面会被爬虫/访客高频命中，必须加这一层缓存避免打爆 DB。
const defaultCatalogCacheTTL = 5 * time.Minute

// defaultHiddenPublicModelPricingGroupIDs 默认不在公开价格页展示的分组。
// 运营要求：内部测试分组不上价格页；旧倍率月卡组（Lite/Pro/Apex，GPT ×0.3774-0.5051、
// Claude ×0.7548-1.0102、Grok ×0.3208-0.4294）自 2026-08-19 起从价格页下线，
// 只保留统一倍率月卡组（GPT ×0.5、Grok ×0.4、Claude ×2.4，即 Plus/Pro/Max 系列）。
// 如需调整，直接增删这里的 ID；后续可改为 DB 设置 model_pricing_hidden_group_ids 免发版维护。
var defaultHiddenPublicModelPricingGroupIDs = map[int64]struct{}{
	7:  {}, // 旧 GPT Lite 月卡组（×0.3774）
	8:  {}, // 旧 GPT Pro 月卡组（×0.393）
	18: {}, // 旧 GPT Apex 月卡组（×0.5051）
	11: {}, // 旧 Claude Lite 月卡组（×0.7548）
	12: {}, // 旧 Claude Pro 月卡组（×0.7859）
	19: {}, // 旧 Claude Apex 月卡组（×1.0102）
	35: {}, // 旧 Grok Lite 月卡组（×0.3208）
	36: {}, // 旧 Grok Pro 月卡组（×0.334）
	39: {}, // 旧 Grok Apex 月卡组（×0.4294）
	46: {}, // 测试专用月卡 · GPT
	47: {}, // 测试专用月卡 · Claude
}

// displayHiddenModelNames 不在价格页展示但仍可正常请求的模型别名。
// gpt-5.6 是 OpenAI 官方别名(路由到 GPT-5.6 Sol),与 sol 重复展示没有意义。
var displayHiddenModelNames = map[string]struct{}{
	"gpt-5.6": {},
}

type disabledPublicModelRule struct {
	model   string
	anchors []string
	// allowedGroupIDs lists groups that intentionally re-open the retired
	// model (e.g. the enterprise line). Requests and price rows scoped to one
	// of these groups bypass the retirement gate.
	allowedGroupIDs []int64
}

func (rule disabledPublicModelRule) allowsGroup(groupID int64) bool {
	for _, id := range rule.allowedGroupIDs {
		if id == groupID {
			return true
		}
	}
	return false
}

// Disabled models remain visible as struck-through rows when a related active
// model is present. This tells users they were intentionally retired instead
// of making them look accidentally omitted from the price catalog.
var disabledPublicModelRules = []disabledPublicModelRule{
	{model: "gpt-5.6-luna", anchors: []string{"gpt-5.6-sol", "gpt-5.6-terra"}, allowedGroupIDs: []int64{59}},
	{model: "gpt-5.4-mini", anchors: []string{"gpt-5.4"}, allowedGroupIDs: []int64{59}},
}

// GPT Image 2 官方标准价（USD / 1M tokens）。图片模型同时存在文本与图片两套
// 输入/缓存费率，不能压扁成普通文本模型的 input/output/cache 三列。
// https://openai.com/api/pricing/
var gptImage2OfficialPrice = PublicImageGenerationPricing{
	Mode:                  "token",
	TextInputPrice:        ptr(5),
	TextCachedInputPrice:  ptr(1.25),
	ImageInputPrice:       ptr(8),
	ImageCachedInputPrice: ptr(2),
	ImageOutputPrice:      ptr(30),
}

func (s *ModelPricingService) isDisplayHiddenModel(model string) bool {
	_, ok := displayHiddenModelNames[strings.ToLower(strings.TrimSpace(model))]
	return ok
}

// manualOfficialPrices 手动维护的官方价表（USD per 1M tokens）。
// 这些模型不在 LiteLLM 官方价表（model_prices_and_context_window.json）里，
// 但分组 model_mapping 会暴露它们。价格乘以分组倍率得到客户实付价（元/1M）。
// 所有价格取自对应厂商官方定价页（2026-08 核对）；新模型的已审价卡
// 由 model-catalog/catalog.json 生成并在 init 中合并，避免重复手改：
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

func init() {
	for model, price := range generatedCatalogDisplayPrices {
		manualOfficialPrices[model] = price
	}
}

// ModelPricingService 聚合"全部 active 分组 × 分组内可用模型 × 官方价"生成公开价格目录。
type ModelPricingService struct {
	groupRepo       GroupRepository
	pricing         ModelPricingProvider
	modelsLister    AvailableModelsLister
	channelPricing  ChannelModelPricingProvider
	catalogCache    *gocache.Cache
	catalogCacheTTL time.Duration
}

// NewModelPricingService 创建 ModelPricingService。
// pricing / modelsLister 用窄接口注入，便于单元测试。
func NewModelPricingService(
	groupRepo GroupRepository,
	pricing ModelPricingProvider,
	modelsLister AvailableModelsLister,
	channelService *ChannelService,
) *ModelPricingService {
	service := &ModelPricingService{
		groupRepo:       groupRepo,
		pricing:         pricing,
		modelsLister:    modelsLister,
		catalogCache:    gocache.New(defaultCatalogCacheTTL, time.Minute),
		catalogCacheTTL: defaultCatalogCacheTTL,
	}
	if channelService != nil {
		service.channelPricing = channelService
	}
	return service
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
	GroupID          int64                         `json:"group_id"`
	Name             string                        `json:"name"`
	Description      string                        `json:"description,omitempty"`
	Platform         string                        `json:"platform"`
	RateMultiplier   float64                       `json:"rate_multiplier"`
	IsExclusive      bool                          `json:"is_exclusive"`
	SubscriptionType string                        `json:"subscription_type"`
	Models           []PublicModelPrice            `json:"models"`
	ImageGeneration  *PublicImageGenerationPricing `json:"image_generation,omitempty"`
}

// PublicModelPrice 单个模型的实付价（元/1M tokens），价格未知时为 nil。
type PublicModelPrice struct {
	Model           string                    `json:"model"`
	InputPrice      *float64                  `json:"input_price"`
	OutputPrice     *float64                  `json:"output_price"`
	CacheWritePrice *float64                  `json:"cache_write_price"`
	CacheReadPrice  *float64                  `json:"cache_read_price"`
	LongContext     *PublicLongContextPricing `json:"long_context,omitempty"`
	Disabled        bool                      `json:"disabled,omitempty"`
}

// PublicLongContextPricing discloses the full-request surcharge applied when
// the input side (uncached + cached input) exceeds the threshold.
type PublicLongContextPricing struct {
	InputThreshold   int     `json:"input_threshold"`
	InputMultiplier  float64 `json:"input_multiplier"`
	OutputMultiplier float64 `json:"output_multiplier"`
}

// PublicImageGenerationPricing 描述分组真实执行的生图计费方式。
// fixed_per_image 仅使用 PricePerImage；token 使用五个分模态 token 价格。
type PublicImageGenerationPricing struct {
	Mode                  string   `json:"mode"`
	PricePerImage         *float64 `json:"price_per_image,omitempty"`
	TextInputPrice        *float64 `json:"text_input_price,omitempty"`
	TextCachedInputPrice  *float64 `json:"text_cached_input_price,omitempty"`
	ImageInputPrice       *float64 `json:"image_input_price,omitempty"`
	ImageCachedInputPrice *float64 `json:"image_cached_input_price,omitempty"`
	ImageOutputPrice      *float64 `json:"image_output_price,omitempty"`
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
		imagePricing := publicImageGenerationPricing(g, models)
		if len(models) == 0 && imagePricing == nil {
			continue
		}
		prices := make([]PublicModelPrice, 0, len(models))
		for _, model := range models {
			if s.isDisplayHiddenModel(model) {
				continue
			}
			// 生图费率由分组独立配置决定；不要再把 gpt-image-2 当普通文本
			// 模型套用 LiteLLM 三列价格，否则缓存模态和实际倍率都会显示错误。
			if imagePricing != nil && strings.EqualFold(strings.TrimSpace(model), "gpt-image-2") {
				continue
			}
			price, ok := s.priceForModel(ctx, g.ID, model, g.RateMultiplier)
			if !ok {
				slog.Debug("model_pricing: skip model without official price",
					"group", g.Name, "model", model)
				continue
			}
			if IsDisabledPublicModelForGroup(model, g.ID) {
				price.Disabled = true
			}
			prices = append(prices, price)
		}
		prices = s.withDisabledModels(ctx, g.ID, prices, g.RateMultiplier)
		if len(prices) == 0 && imagePricing == nil {
			continue
		}
		sort.Slice(prices, func(i, j int) bool {
			return modelDisplayLess(prices[i], prices[j])
		})
		catalog.Groups = append(catalog.Groups, PublicModelPricingGroup{
			GroupID:          g.ID,
			Name:             publicGroupDisplayName(g.Name),
			Description:      g.Description,
			Platform:         g.Platform,
			RateMultiplier:   g.RateMultiplier,
			IsExclusive:      g.IsExclusive,
			SubscriptionType: g.SubscriptionType,
			Models:           prices,
			ImageGeneration:  imagePricing,
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

// IsDisabledPublicModel reports whether a requested OpenAI model has been
// retired from public routing. Known Codex aliases are normalized first so a
// reasoning suffix cannot bypass retirement at the request boundary.
func IsDisabledPublicModel(model string) bool {
	return IsDisabledPublicModelForGroup(model, 0)
}

// IsDisabledPublicModelForGroup is the group-aware variant: a rule does not
// apply when the request or price row belongs to one of the rule's
// allowedGroupIDs. groupID 0 (unknown/no group) keeps the global semantics.
func IsDisabledPublicModelForGroup(model string, groupID int64) bool {
	name := strings.ToLower(strings.TrimSpace(model))
	if normalized, ok := normalizeKnownCodexModel(name); ok {
		name = normalized
	}
	for _, rule := range disabledPublicModelRules {
		if name == rule.model && !rule.allowsGroup(groupID) {
			return true
		}
	}
	return false
}

func (s *ModelPricingService) withDisabledModels(ctx context.Context, groupID int64, prices []PublicModelPrice, rateMultiplier float64) []PublicModelPrice {
	present := make(map[string]bool, len(prices))
	for i := range prices {
		name := strings.ToLower(strings.TrimSpace(prices[i].Model))
		present[name] = true
		if IsDisabledPublicModelForGroup(name, groupID) {
			prices[i].Disabled = true
		}
	}
	for _, rule := range disabledPublicModelRules {
		// Exempted groups treat the model as available: never synthesize a
		// struck-through row for them.
		if rule.allowsGroup(groupID) || present[rule.model] || !containsAnyModelName(present, rule.anchors) {
			continue
		}
		disabled, ok := s.priceForModel(ctx, groupID, rule.model, rateMultiplier)
		if !ok {
			// A retired model can disappear from the provider price source before
			// the public notice is removed. Keep an empty disabled row in that case.
			disabled = PublicModelPrice{Model: rule.model}
		}
		disabled.Disabled = true
		prices = append(prices, disabled)
		present[rule.model] = true
	}
	return prices
}

func containsAnyModelName(present map[string]bool, models []string) bool {
	for _, model := range models {
		if present[model] {
			return true
		}
	}
	return false
}

func publicImageGenerationPricing(g Group, models []string) *PublicImageGenerationPricing {
	if !g.AllowImageGeneration || !containsModelName(models, "gpt-image-2") {
		return nil
	}
	multiplier := g.RateMultiplier
	if g.ImageRateIndependent {
		multiplier = g.ImageRateMultiplier
	}
	if g.GPTImageCallPrice != nil && *g.GPTImageCallPrice > 0 {
		return &PublicImageGenerationPricing{
			Mode:          "fixed_per_image",
			PricePerImage: ptr(round4(*g.GPTImageCallPrice * multiplier)),
		}
	}
	p := gptImage2OfficialPrice
	p.TextInputPrice = multipliedPrice(p.TextInputPrice, multiplier)
	p.TextCachedInputPrice = multipliedPrice(p.TextCachedInputPrice, multiplier)
	p.ImageInputPrice = multipliedPrice(p.ImageInputPrice, multiplier)
	p.ImageCachedInputPrice = multipliedPrice(p.ImageCachedInputPrice, multiplier)
	p.ImageOutputPrice = multipliedPrice(p.ImageOutputPrice, multiplier)
	return &p
}

func containsModelName(models []string, want string) bool {
	for _, model := range models {
		if strings.EqualFold(strings.TrimSpace(model), want) {
			return true
		}
	}
	return false
}

func multipliedPrice(price *float64, multiplier float64) *float64 {
	if price == nil {
		return nil
	}
	return ptr(round4(*price * multiplier))
}

// priceForModel calculates the customer price for one model. A flat token
// override bound to the group wins over the external catalog, matching the
// billing resolver. Nil override fields keep their catalog/manual defaults.
func (s *ModelPricingService) priceForModel(ctx context.Context, groupID int64, model string, rateMultiplier float64) (PublicModelPrice, bool) {
	var input, output, cacheWrite, cacheRead *float64
	if p := s.pricing.GetModelPricing(model); p != nil {
		input = nonZeroPricePerMTok(p.InputCostPerToken)
		output = nonZeroPricePerMTok(p.OutputCostPerToken)
		cacheWrite = nonZeroPricePerMTok(p.CacheCreationInputTokenCost)
		cacheRead = nonZeroPricePerMTok(p.CacheReadInputTokenCost)
	}

	if input == nil && output == nil && cacheRead == nil {
		if mp, ok := manualOfficialPrices[strings.ToLower(model)]; ok {
			input = ptr(mp.input)
			output = ptr(mp.output)
			cacheRead = ptr(mp.cacheRead)
		}
	}

	if s.channelPricing != nil {
		if override := s.channelPricing.GetChannelModelPricing(ctx, groupID, model); override != nil &&
			(override.BillingMode == "" || override.BillingMode == BillingModeToken) && len(override.Intervals) == 0 {
			if override.InputPrice != nil {
				input = pricePerMTok(*override.InputPrice)
			}
			if override.OutputPrice != nil {
				output = pricePerMTok(*override.OutputPrice)
			}
			if override.CacheWritePrice != nil {
				cacheWrite = pricePerMTok(*override.CacheWritePrice)
			}
			if override.CacheReadPrice != nil {
				cacheRead = pricePerMTok(*override.CacheReadPrice)
			}
		}
	}

	if input == nil && output == nil && cacheWrite == nil && cacheRead == nil {
		return PublicModelPrice{}, false
	}
	return PublicModelPrice{
		Model:           model,
		InputPrice:      multipliedPrice(input, rateMultiplier),
		OutputPrice:     multipliedPrice(output, rateMultiplier),
		CacheWritePrice: multipliedPrice(cacheWrite, rateMultiplier),
		CacheReadPrice:  multipliedPrice(cacheRead, rateMultiplier),
		LongContext:     publicLongContextPricing(model),
	}, true
}

func publicLongContextPricing(model string) *PublicLongContextPricing {
	if !isOpenAILongContextTierModel(model) {
		return nil
	}
	return &PublicLongContextPricing{
		InputThreshold:   openAILongContextInputThreshold,
		InputMultiplier:  openAILongContextInputMultiplier,
		OutputMultiplier: openAILongContextOutputMultiplier,
	}
}

func pricePerMTok(perToken float64) *float64 {
	if perToken < 0 {
		return nil
	}
	return ptr(perToken * 1e6)
}

func nonZeroPricePerMTok(perToken float64) *float64 {
	if perToken <= 0 {
		return nil
	}
	return pricePerMTok(perToken)
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
		// Preserve the public API contract that models is always a JSON array.
		// append(nil, empty...) collapses an intentionally empty image-only
		// group to nil, which json.Marshal emits as null and older clients cannot
		// iterate safely.
		out.Groups[i].Models = make([]PublicModelPrice, len(g.Models))
		copy(out.Groups[i].Models, g.Models)
		for j := range out.Groups[i].Models {
			if g.Models[j].LongContext != nil {
				longContext := *g.Models[j].LongContext
				out.Groups[i].Models[j].LongContext = &longContext
			}
		}
		if g.ImageGeneration != nil {
			imagePricing := *g.ImageGeneration
			out.Groups[i].ImageGeneration = &imagePricing
		}
	}
	return &out
}

// modelVersionPattern 匹配模型版本号,兼容 "5.6"、"4-8"、"5"(单独大版本)等格式。
// 例如 claude-opus-5 → (5,0);claude-opus-4-8 → (4,8);gpt-5.6-luna → (5,6)。
var modelVersionPattern = regexp.MustCompile(`(\d+)(?:[.\-](\d+))?`)

// modelDisplayRankValue 模型展示排序键:家族 → 大版本 → 小版本 → 档位。
type modelDisplayRankValue struct {
	family int
	major  int
	minor  int
	tier   int
}

// modelDisplayLess 分组内模型展示顺序:
//   - 家族在前(Claude: Fable < Opus < Sonnet < Haiku),再版本从新到旧,
//   - 档位(旗舰/最贵)在前(sol > terra > luna;完整版 > mini/nano),
//   - 同版本同档位按输入价从高到低,再按名称兜底。
//
// 效果:GPT → 5.6(sol/terra/luna) → 5.5 → 5.4 → 5.4-mini;
// Claude → Fable 5 → Opus 5 → Opus 4.8/4.7/4.6/4.5 → Sonnet 5 → Sonnet 4.6/4.5 → Haiku。
func modelDisplayLess(a, b PublicModelPrice) bool {
	ra, rb := modelDisplayRank(a.Model), modelDisplayRank(b.Model)
	if ra.family != rb.family {
		return ra.family < rb.family
	}
	if ra.major != rb.major {
		return ra.major > rb.major
	}
	if ra.minor != rb.minor {
		return ra.minor > rb.minor
	}
	if ra.tier != rb.tier {
		return ra.tier < rb.tier
	}
	pa, pb := priceVal(a.InputPrice), priceVal(b.InputPrice)
	if pa != pb {
		return pa > pb
	}
	return a.Model < b.Model
}

func modelDisplayRank(model string) modelDisplayRankValue {
	m := strings.ToLower(strings.TrimSpace(model))
	r := modelDisplayRankValue{family: 4}
	// 生图模型（如 gpt-image-2）在分组内固定排最后，不打断文本模型的家族/版本排序。
	if strings.Contains(m, "image") {
		r.family = 9
		return r
	}
	switch {
	case strings.Contains(m, "fable"):
		r.family = 0
	case strings.Contains(m, "opus"):
		r.family = 1
	case strings.Contains(m, "sonnet"):
		r.family = 2
	case strings.Contains(m, "haiku"):
		r.family = 3
	}
	if mm := modelVersionPattern.FindStringSubmatch(m); len(mm) >= 2 {
		r.major, _ = strconv.Atoi(mm[1])
		if len(mm) >= 3 {
			r.minor, _ = strconv.Atoi(mm[2])
		}
	}
	switch {
	case strings.Contains(m, "sol"):
		r.tier = 0
	case strings.Contains(m, "terra"):
		r.tier = 1
	case strings.Contains(m, "luna"):
		r.tier = 2
	case strings.Contains(m, "pro"):
		r.tier = 0
	case strings.Contains(m, "codex"):
		r.tier = 3
	case strings.Contains(m, "mini"):
		r.tier = 4
	case strings.Contains(m, "nano"):
		r.tier = 5
	default:
		r.tier = 1 // 无后缀基础档
	}
	return r
}

func priceVal(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ShopChannelFeePercent is the payment-processor cut on shop-channel (pay.ldxp.cn)
// checkouts. Direct WeChat/Alipay collections have no equivalent cut modeled here.
const ShopChannelFeePercent = 3.0

// costAccountingPlanPricing is the face-value catalog price per monthly-card plan.
// Source of truth today is duplicated by necessity: frontend/src/constants/monthlyCreditCards.ts
// (customer-facing display) and .agents/skills/laoshirenai-monthly-cards/config/monthly_plans.json
// (skill tooling) both hardcode the same numbers. Keep all three in sync on any price change;
// this is a known gap to fix later by making pricing DB-driven.
var costAccountingPlanPricing = map[string]struct {
	Name           string
	ShopPriceCNY   float64
	DirectPriceCNY float64
}{
	"lite": {Name: "Lite", ShopPriceCNY: 329, DirectPriceCNY: 319},
	"pro":  {Name: "Pro", ShopPriceCNY: 639, DirectPriceCNY: 619},
}

var costAccountingMonthlyCardGroupIDs = map[string]map[string]int64{
	"lite": {"gpt": 7, "claude": 11, "grok": 35},
	"pro":  {"gpt": 8, "claude": 12, "grok": 36},
}

var costAccountingMonthlyCardPlanOrder = []string{"lite", "pro"}

const costAccountingPayAsYouGoTopupCNY = 100.0

type CostAccountingRate struct {
	GroupID                      int64   `json:"group_id"`
	GroupName                    string  `json:"group_name"`
	GroupRateMultiplier          float64 `json:"group_rate_multiplier"`
	PrimaryAccountRateMultiplier float64 `json:"primary_account_rate_multiplier"`
	WorstAccountRateMultiplier   float64 `json:"worst_account_rate_multiplier"`
	SchedulableAccountCount      int     `json:"schedulable_account_count"`
}

type CostAccountingMoney struct {
	CostCNY       float64 `json:"cost_cny"`
	ProfitCNY     float64 `json:"profit_cny"`
	MarginPercent float64 `json:"margin_percent"`
}

type CostAccountingScenario struct {
	CostPerCreditWorstAccount float64             `json:"cost_per_credit_worst_account"`
	VsShopNetPrice            CostAccountingMoney `json:"vs_shop_net_price"`
	VsDirectPrice             CostAccountingMoney `json:"vs_direct_price"`
}

type CostAccountingRealUsage struct {
	Available                 bool                 `json:"available"`
	Note                      string               `json:"note,omitempty"`
	ObservedRequestCount      int64                `json:"observed_request_count,omitempty"`
	ObservedRawCredits        float64              `json:"observed_raw_credits_consumed,omitempty"`
	ObservedRealCostCNY       float64              `json:"observed_real_cost_cny,omitempty"`
	ProductMixPercent         map[string]float64   `json:"product_mix_percent,omitempty"`
	BlendedCostPerCredit      float64              `json:"blended_cost_per_credit,omitempty"`
	ProjectedFullQuotaCostCNY float64              `json:"projected_full_quota_cost_cny,omitempty"`
	VsShopNetPrice            *CostAccountingMoney `json:"vs_shop_net_price,omitempty"`
	VsDirectPrice             *CostAccountingMoney `json:"vs_direct_price,omitempty"`
}

type CostAccountingMarginRange struct {
	WorstPercent        float64  `json:"worst_percent"`
	BestPercent         float64  `json:"best_percent"`
	ConservativePercent float64  `json:"conservative_percent"`
	RealPercent         *float64 `json:"real_percent,omitempty"`
}

type CostAccountingMonthlyPlan struct {
	ID                     string                            `json:"id"`
	Name                   string                            `json:"name"`
	ShopPriceCNY           float64                           `json:"shop_price_cny"`
	ShopFeePercent         float64                           `json:"shop_fee_percent"`
	ShopNetPriceCNY        float64                           `json:"shop_net_price_cny"`
	DirectPriceCNY         float64                           `json:"direct_price_cny"`
	MonthlyCredits         float64                           `json:"monthly_credits"`
	Products               map[string]CostAccountingRate     `json:"products"`
	SingleProductScenarios map[string]CostAccountingScenario `json:"single_product_scenarios"`
	RealUsage              CostAccountingRealUsage           `json:"real_usage"`
	// BestCaseScenario: full quota, all traffic through whichever product's
	// cheapest bound (primary) account is cheapest overall. See MarginRange.BestPercent.
	BestCaseScenario CostAccountingMoney       `json:"best_case_scenario"`
	MarginRange      CostAccountingMarginRange `json:"margin_range"`
}

type CostAccountingPayAsYouGoGroup struct {
	GroupID                      int64                   `json:"group_id"`
	GroupName                    string                  `json:"group_name"`
	Product                      string                  `json:"product"`
	Platform                     string                  `json:"platform"`
	GroupRateMultiplier          float64                 `json:"group_rate_multiplier"`
	PrimaryAccountRateMultiplier float64                 `json:"primary_account_rate_multiplier"`
	WorstAccountRateMultiplier   float64                 `json:"worst_account_rate_multiplier"`
	SchedulableAccountCount      int                     `json:"schedulable_account_count"`
	Topup100CNYScenario          *CostAccountingMoney    `json:"topup_100_cny_scenario,omitempty"`
	RealUsage                    CostAccountingRealUsage `json:"real_usage"`
	Warning                      string                  `json:"warning,omitempty"`
}

type CostAccountingOverview struct {
	GeneratedAt                 time.Time                       `json:"generated_at"`
	ShopChannelFeePercent       float64                         `json:"shop_channel_fee_percent"`
	UsageWindowStart            time.Time                       `json:"usage_window_start"`
	UsageWindowEnd              time.Time                       `json:"usage_window_end"`
	PricingSourceNote           string                          `json:"pricing_source_note"`
	ScopeNote                   string                          `json:"scope_note"`
	LegacyMonthlyCardGroupCount int                             `json:"legacy_monthly_card_group_count"`
	LegacyMonthlyCardRealUsage  CostAccountingRealUsage         `json:"legacy_monthly_card_real_usage"`
	MonthlyCards                []CostAccountingMonthlyPlan     `json:"monthly_cards"`
	PayAsYouGo                  []CostAccountingPayAsYouGoGroup `json:"pay_as_you_go"`
}

type CostAccountingUsageRow struct {
	GroupID      int64
	RequestCount int64
	RawCredits   float64
	RealCostCNY  float64
}

func money(price, cost float64) CostAccountingMoney {
	profit := price - cost
	margin := 0.0
	if price > 0 {
		margin = (profit / price) * 100
	}
	return CostAccountingMoney{CostCNY: round2(cost), ProfitCNY: round2(profit), MarginPercent: round2(margin)}
}

func round2(v float64) float64 {
	return float64(int64(v*100+sign(v)*0.5)) / 100
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// accountRateSummary computes primary (highest-priority, i.e. lowest priority
// number) and worst (max among schedulable) account rate multipliers bound to
// a group, matching the methodology already used in monthly_margin.py.
func accountRateSummary(groupID int64, accounts []Account) (primary float64, worst float64, schedulableCount int) {
	var bestPriority int
	haveBest := false
	for i := range accounts {
		acc := &accounts[i]
		if !acc.IsSchedulable() {
			continue
		}
		schedulableCount++
		rate := acc.BillingRateMultiplier()
		if rate > worst {
			worst = rate
		}
		priority := acc.EffectivePriorityForGroup(&groupID)
		if !haveBest || priority < bestPriority || (priority == bestPriority && rate > primary) {
			bestPriority = priority
			primary = rate
			haveBest = true
		}
	}
	return primary, worst, schedulableCount
}

func (s *OpsService) loadGroupAndAccounts(ctx context.Context, groupID int64) (*Group, []Account, error) {
	group, err := s.groupRepo.GetByIDLite(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	if group == nil {
		return nil, nil, fmt.Errorf("group %d not found", groupID)
	}
	accounts, err := s.accountRepo.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	return group, accounts, nil
}

// costAccountingPayAsYouGoTargets discovers the complete current public
// pay-as-you-go catalog instead of relying on a stale list of group IDs.
// Historical monthly-card groups are credit subscriptions and therefore never
// enter this list.
func costAccountingPayAsYouGoTargets(groups []Group) []Group {
	targets := make([]Group, 0, len(groups))
	for _, group := range groups {
		if group.Status != StatusActive ||
			group.IsExclusive ||
			group.SubscriptionType != SubscriptionTypeStandard {
			continue
		}
		targets = append(targets, group)
	}
	sort.SliceStable(targets, func(i, j int) bool {
		if targets[i].SortOrder != targets[j].SortOrder {
			return targets[i].SortOrder < targets[j].SortOrder
		}
		return targets[i].ID < targets[j].ID
	})
	return targets
}

func costAccountingProductForGroup(group Group) string {
	name := strings.ToLower(group.Name)
	switch {
	case strings.Contains(name, "glm"):
		return "glm"
	case strings.Contains(name, "grok"):
		return "grok"
	case group.Platform == PlatformOpenAI:
		return "gpt"
	case group.Platform == PlatformAnthropic:
		return "claude"
	case group.Platform != "":
		return strings.ToLower(group.Platform)
	default:
		return "other"
	}
}

// GetCostAccountingOverview computes a full cost/margin snapshot across
// the current monthly-card products (GPT/Claude/Grok x Lite/Pro) and every
// active public standard-billing group, including this month's real usage mix.
// Historical monthly-card groups remain available for existing entitlements
// but are intentionally excluded from the current commercial catalog.
func (s *OpsService) GetCostAccountingOverview(ctx context.Context) (*CostAccountingOverview, error) {
	if s == nil || s.groupRepo == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("cost accounting: service not fully wired")
	}

	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	windowStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)

	activeGroups, err := s.groupRepo.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("cost accounting: list active groups: %w", err)
	}
	payAsYouGoTargets := costAccountingPayAsYouGoTargets(activeGroups)

	allGroupIDs := make([]int64, 0, 32)
	currentMonthlyGroupIDs := make(map[int64]struct{}, 8)
	for _, plan := range costAccountingMonthlyCardPlanOrder {
		for _, gid := range costAccountingMonthlyCardGroupIDs[plan] {
			currentMonthlyGroupIDs[gid] = struct{}{}
			allGroupIDs = append(allGroupIDs, gid)
		}
	}
	legacyMonthlyCardGroupIDs := make([]int64, 0, 16)
	for _, group := range activeGroups {
		if group.SubscriptionType != SubscriptionTypeCredit {
			continue
		}
		if _, current := currentMonthlyGroupIDs[group.ID]; current {
			continue
		}
		legacyMonthlyCardGroupIDs = append(legacyMonthlyCardGroupIDs, group.ID)
		allGroupIDs = append(allGroupIDs, group.ID)
	}
	for _, group := range payAsYouGoTargets {
		allGroupIDs = append(allGroupIDs, group.ID)
	}

	var usageByGroup map[int64]CostAccountingUsageRow
	var usageErr error
	if s.opsRepo != nil {
		usageByGroup, usageErr = s.opsRepo.GetCostAccountingRealUsage(ctx, allGroupIDs, windowStart, now)
	}

	overview := &CostAccountingOverview{
		GeneratedAt:           now,
		ShopChannelFeePercent: ShopChannelFeePercent,
		UsageWindowStart:      windowStart,
		UsageWindowEnd:        now,
		PricingSourceNote: "shop/direct prices are hardcoded in this handler; keep in sync with " +
			"frontend/src/constants/monthlyCreditCards.ts and the monthly-cards skill's config/monthly_plans.json",
		ScopeNote:                   "当前月卡只展示在售 Lite/Pro；历史月卡分组不再作为产品卡展示，但其真实请求成本仍计入本月实际上游成本。按量付费自动读取全部启用的公开标准计费分组。",
		LegacyMonthlyCardGroupCount: len(legacyMonthlyCardGroupIDs),
		LegacyMonthlyCardRealUsage:  CostAccountingRealUsage{Available: false, Note: "usage query unavailable"},
	}
	if usageErr == nil && usageByGroup != nil {
		var legacyRequests int64
		var legacyCredits float64
		var legacyCost float64
		for _, groupID := range legacyMonthlyCardGroupIDs {
			usage := usageByGroup[groupID]
			legacyRequests += usage.RequestCount
			legacyCredits += usage.RawCredits
			legacyCost += usage.RealCostCNY
		}
		overview.LegacyMonthlyCardRealUsage = CostAccountingRealUsage{
			Available:            true,
			ObservedRequestCount: legacyRequests,
			ObservedRawCredits:   round4(legacyCredits),
			ObservedRealCostCNY:  round4(legacyCost),
		}
		if legacyCredits <= 0 {
			overview.LegacyMonthlyCardRealUsage.Note = "no legacy monthly-card usage recorded in this window"
		} else {
			overview.LegacyMonthlyCardRealUsage.BlendedCostPerCredit = round6(legacyCost / legacyCredits)
		}
	}

	for _, planID := range costAccountingMonthlyCardPlanOrder {
		pricing, ok := costAccountingPlanPricing[planID]
		if !ok {
			continue
		}
		groupIDs := costAccountingMonthlyCardGroupIDs[planID]
		products := map[string]CostAccountingRate{}
		worstCostPerCredit := map[string]float64{}
		primaryCostPerCredit := map[string]float64{}
		var monthlyCredits float64

		for _, product := range []string{"gpt", "claude", "grok"} {
			gid := groupIDs[product]
			group, accounts, err := s.loadGroupAndAccounts(ctx, gid)
			if err != nil {
				return nil, fmt.Errorf("cost accounting: load group %d (%s/%s): %w", gid, planID, product, err)
			}
			if group.MonthlyLimitUSD != nil && *group.MonthlyLimitUSD > monthlyCredits {
				monthlyCredits = *group.MonthlyLimitUSD
			}
			primary, worst, schedulable := accountRateSummary(gid, accounts)
			products[product] = CostAccountingRate{
				GroupID:                      gid,
				GroupName:                    group.Name,
				GroupRateMultiplier:          group.RateMultiplier,
				PrimaryAccountRateMultiplier: primary,
				WorstAccountRateMultiplier:   worst,
				SchedulableAccountCount:      schedulable,
			}
			if group.RateMultiplier > 0 {
				worstCostPerCredit[product] = worst / group.RateMultiplier
				primaryCostPerCredit[product] = primary / group.RateMultiplier
			}
		}

		shopNet := pricing.ShopPriceCNY * (1 - ShopChannelFeePercent/100)
		direct := pricing.DirectPriceCNY

		scenarios := map[string]CostAccountingScenario{}
		conservativeCP := 0.0
		balancedSum := 0.0
		for _, product := range []string{"gpt", "claude", "grok"} {
			cp := worstCostPerCredit[product]
			if cp > conservativeCP {
				conservativeCP = cp
			}
			balancedSum += primaryCostPerCredit[product]
			cost := monthlyCredits * cp
			scenarios["all_"+product] = CostAccountingScenario{
				CostPerCreditWorstAccount: round4(cp),
				VsShopNetPrice:            money(shopNet, cost),
				VsDirectPrice:             money(direct, cost),
			}
		}
		balancedCP := balancedSum / 3
		conservativeCost := monthlyCredits * conservativeCP
		balancedCost := monthlyCredits * balancedCP
		conservativeMargin := money(direct, conservativeCost).MarginPercent
		balancedMargin := money(direct, balancedCost).MarginPercent

		// Best case: all traffic goes through whichever product's cheapest
		// (primary/highest-priority) bound account is cheapest overall — e.g.
		// GPT's primary account at 0.15 vs its own worst-case fallback at
		// 0.20. This is what "全部用便宜账号" actually means; it must use
		// primaryCostPerCredit, not worstCostPerCredit, or it collapses to
		// the same number as the worst/conservative case.
		bestCP := 0.0
		haveBestCP := false
		for _, product := range []string{"gpt", "claude", "grok"} {
			cp, ok := primaryCostPerCredit[product]
			if !ok {
				continue
			}
			if !haveBestCP || cp < bestCP {
				bestCP = cp
				haveBestCP = true
			}
		}
		bestCost := monthlyCredits * bestCP
		bestScenario := money(direct, bestCost)

		realUsage := CostAccountingRealUsage{Available: false, Note: "usage query unavailable"}
		var realPercentPtr *float64
		if usageErr == nil && usageByGroup != nil {
			var rawTotal, costTotal float64
			var requestTotal int64
			mix := map[string]float64{}
			for _, product := range []string{"gpt", "claude", "grok"} {
				row := usageByGroup[groupIDs[product]]
				mix[product] = row.RawCredits
				rawTotal += row.RawCredits
				costTotal += row.RealCostCNY
				requestTotal += row.RequestCount
			}
			if rawTotal > 0 {
				blended := costTotal / rawTotal
				projected := monthlyCredits * blended
				mixPercent := map[string]float64{}
				for product, raw := range mix {
					mixPercent[product] = round2((raw / rawTotal) * 100)
				}
				vsShop := money(shopNet, projected)
				vsDirect := money(direct, projected)
				realUsage = CostAccountingRealUsage{
					Available:                 true,
					ObservedRequestCount:      requestTotal,
					ObservedRawCredits:        round4(rawTotal),
					ObservedRealCostCNY:       round4(costTotal),
					ProductMixPercent:         mixPercent,
					BlendedCostPerCredit:      round6(blended),
					ProjectedFullQuotaCostCNY: round2(projected),
					VsShopNetPrice:            &vsShop,
					VsDirectPrice:             &vsDirect,
				}
				realPercentPtr = &vsDirect.MarginPercent
			} else {
				realUsage = CostAccountingRealUsage{Available: true, Note: "no monthly-card usage recorded in this window for this plan"}
			}
		}

		overview.MonthlyCards = append(overview.MonthlyCards, CostAccountingMonthlyPlan{
			ID:                     planID,
			Name:                   pricing.Name,
			ShopPriceCNY:           pricing.ShopPriceCNY,
			ShopFeePercent:         ShopChannelFeePercent,
			ShopNetPriceCNY:        round2(shopNet),
			DirectPriceCNY:         direct,
			MonthlyCredits:         monthlyCredits,
			Products:               products,
			SingleProductScenarios: scenarios,
			RealUsage:              realUsage,
			BestCaseScenario:       bestScenario,
			MarginRange: CostAccountingMarginRange{
				WorstPercent:        conservativeMargin,
				BestPercent:         bestScenario.MarginPercent,
				ConservativePercent: conservativeMargin,
				RealPercent:         realPercentPtr,
			},
		})
		_ = balancedMargin
	}

	for _, group := range payAsYouGoTargets {
		accounts, err := s.accountRepo.ListByGroup(ctx, group.ID)
		if err != nil {
			return nil, fmt.Errorf("cost accounting: load pay-as-you-go group %d accounts: %w", group.ID, err)
		}
		primary, worst, schedulable := accountRateSummary(group.ID, accounts)
		row := CostAccountingPayAsYouGoGroup{
			GroupID:                      group.ID,
			GroupName:                    group.Name,
			Product:                      costAccountingProductForGroup(group),
			Platform:                     group.Platform,
			GroupRateMultiplier:          group.RateMultiplier,
			PrimaryAccountRateMultiplier: primary,
			WorstAccountRateMultiplier:   worst,
			SchedulableAccountCount:      schedulable,
			RealUsage:                    CostAccountingRealUsage{Available: false, Note: "usage query unavailable"},
		}
		if usageErr == nil && usageByGroup != nil {
			usage := usageByGroup[group.ID]
			row.RealUsage = CostAccountingRealUsage{
				Available:            true,
				ObservedRequestCount: usage.RequestCount,
				ObservedRawCredits:   round4(usage.RawCredits),
				ObservedRealCostCNY:  round4(usage.RealCostCNY),
			}
			if usage.RawCredits <= 0 {
				row.RealUsage.Note = "no usage recorded in this window for this group"
			} else {
				row.RealUsage.BlendedCostPerCredit = round6(usage.RealCostCNY / usage.RawCredits)
			}
		}
		if schedulable == 0 {
			row.Warning = "no schedulable account bound to this group; it cannot currently serve requests"
		} else if group.RateMultiplier > 0 {
			netRevenue := costAccountingPayAsYouGoTopupCNY * (1 - ShopChannelFeePercent/100)
			costPerCredit := worst / group.RateMultiplier
			cost := costAccountingPayAsYouGoTopupCNY * costPerCredit
			scenario := money(netRevenue, cost)
			row.Topup100CNYScenario = &scenario
		}
		overview.PayAsYouGo = append(overview.PayAsYouGo, row)
	}

	return overview, nil
}

func round4(v float64) float64 {
	scaled := v * 10000
	return float64(int64(scaled+sign(scaled)*0.5)) / 10000
}

func round6(v float64) float64 {
	scaled := v * 1000000
	return float64(int64(scaled+sign(scaled)*0.5)) / 1000000
}

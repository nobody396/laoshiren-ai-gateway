package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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
	"plus": {Name: "Plus", ShopPriceCNY: 299, DirectPriceCNY: 299},
	"pro":  {Name: "Pro", ShopPriceCNY: 599, DirectPriceCNY: 599},
	"max":  {Name: "Max", ShopPriceCNY: 999, DirectPriceCNY: 999},
}

var costAccountingMonthlyCardGroupNames = map[string]map[string]string{
	"plus": {"gpt": "GPT Plus 月卡组", "claude": "Claude Plus 月卡组", "grok": "Grok Plus 月卡组"},
	"pro":  {"gpt": "GPT Pro V3 月卡组", "claude": "Claude Pro V3 月卡组", "grok": "Grok Pro V3 月卡组"},
	"max":  {"gpt": "GPT Max V3 月卡组", "claude": "Claude Max V3 月卡组", "grok": "Grok Max V3 月卡组"},
}

var costAccountingMonthlyCardPlanOrder = []string{"plus", "pro", "max"}

func costAccountingResolveMonthlyGroupIDs(groups []Group) map[string]map[string]int64 {
	groupIDByName := make(map[string]int64, len(groups))
	for _, group := range groups {
		if group.Status == StatusActive {
			groupIDByName[group.Name] = group.ID
		}
	}
	resolved := make(map[string]map[string]int64, len(costAccountingMonthlyCardGroupNames))
	for planID, productNames := range costAccountingMonthlyCardGroupNames {
		resolved[planID] = make(map[string]int64, len(productNames))
		for product, name := range productNames {
			resolved[planID][product] = groupIDByName[name]
		}
	}
	return resolved
}

const costAccountingPayAsYouGoTopupCNY = 100.0

const CostAccountingScenarioBasisObservedUsage = "observed_real_usage"

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
	Topup100CNYScenarioBasis     string                  `json:"topup_100_cny_scenario_basis,omitempty"`
	RealUsage                    CostAccountingRealUsage `json:"real_usage"`
	Warning                      string                  `json:"warning,omitempty"`
}

type CostAccountingUpstreamAccount struct {
	AccountID              int64      `json:"account_id"`
	AccountName            string     `json:"account_name"`
	Priority               int        `json:"priority"`
	ConfiguredMultiplier   float64    `json:"configured_multiplier"`
	ObservedMultiplier     *float64   `json:"observed_multiplier,omitempty"`
	MultiplierDriftPercent *float64   `json:"multiplier_drift_percent,omitempty"`
	MultiplierAuditStatus  string     `json:"multiplier_audit_status"`
	BalanceValue           *float64   `json:"balance_value,omitempty"`
	BalanceCurrency        string     `json:"balance_currency,omitempty"`
	BalanceStatus          string     `json:"balance_status"`
	AuditSampledAt         *time.Time `json:"audit_sampled_at,omitempty"`
	BaseURL                string     `json:"base_url"`
	Models                 []string   `json:"models"`
	Schedulable            bool       `json:"schedulable"`
	Status                 string     `json:"status"`
}

type CostAccountingUpstreamGroup struct {
	GroupID             int64                           `json:"group_id"`
	GroupName           string                          `json:"group_name"`
	GroupRateMultiplier float64                         `json:"group_rate_multiplier"`
	Platform            string                          `json:"platform"`
	Accounts            []CostAccountingUpstreamAccount `json:"accounts"`
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
	UpstreamRouting             []CostAccountingUpstreamGroup   `json:"upstream_routing"`
}

func auditFloat(value any) *float64 {
	switch raw := value.(type) {
	case float64:
		v := raw
		return &v
	case float32:
		v := float64(raw)
		return &v
	case int:
		v := float64(raw)
		return &v
	case int64:
		v := float64(raw)
		return &v
	case json.Number:
		if parsed, err := raw.Float64(); err == nil {
			return &parsed
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
			return &parsed
		}
	}
	return nil
}

func auditString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func upstreamFinanceAudit(account *Account) (observed, balance *float64, currency, multiplierStatus, balanceStatus string, sampledAt *time.Time) {
	if account == nil || account.Extra == nil {
		return nil, nil, "", "unobserved", "unobserved", nil
	}
	raw, ok := account.Extra["upstream_finance_audit"].(map[string]any)
	if !ok {
		return nil, nil, "", "unobserved", "unobserved", nil
	}
	observed = auditFloat(raw["observed_multiplier"])
	balance = auditFloat(raw["balance_value"])
	currency = auditString(raw["balance_currency"])
	multiplierStatus = auditString(raw["multiplier_status"])
	balanceStatus = auditString(raw["balance_status"])
	if multiplierStatus == "" {
		multiplierStatus = "unobserved"
	}
	if balanceStatus == "" {
		balanceStatus = "unobserved"
	}
	if value := auditString(raw["sampled_at"]); value != "" {
		if parsed, err := time.Parse(time.RFC3339, value); err == nil {
			sampledAt = &parsed
		}
	}
	return
}

func costAccountingUpstreamRoutes(groups []Group, accountsByGroup map[int64][]Account) []CostAccountingUpstreamGroup {
	out := make([]CostAccountingUpstreamGroup, 0, len(groups))
	for _, group := range groups {
		accounts := accountsByGroup[group.ID]
		row := CostAccountingUpstreamGroup{
			GroupID: group.ID, GroupName: group.Name, GroupRateMultiplier: group.RateMultiplier,
			Platform: group.Platform, Accounts: make([]CostAccountingUpstreamAccount, 0, len(accounts)),
		}
		for index := range accounts {
			account := &accounts[index]
			mapping := account.GetModelMapping()
			models := make([]string, 0, len(mapping))
			for model := range mapping {
				models = append(models, model)
			}
			sort.Strings(models)
			observed, balance, currency, multiplierStatus, balanceStatus, sampledAt := upstreamFinanceAudit(account)
			var drift *float64
			configured := account.BillingRateMultiplier()
			if observed != nil && configured > 0 {
				value := round2(((*observed - configured) / configured) * 100)
				drift = &value
			}
			row.Accounts = append(row.Accounts, CostAccountingUpstreamAccount{
				AccountID: account.ID, AccountName: account.Name,
				Priority: account.EffectivePriorityForGroup(&group.ID), ConfiguredMultiplier: configured,
				ObservedMultiplier: observed, MultiplierDriftPercent: drift,
				MultiplierAuditStatus: multiplierStatus, BalanceValue: balance,
				BalanceCurrency: currency, BalanceStatus: balanceStatus, AuditSampledAt: sampledAt,
				BaseURL: strings.TrimSpace(account.GetCredential("base_url")), Models: models,
				Schedulable: account.IsSchedulable(), Status: account.Status,
			})
		}
		sort.Slice(row.Accounts, func(i, j int) bool {
			if row.Accounts[i].Schedulable != row.Accounts[j].Schedulable {
				return row.Accounts[i].Schedulable
			}
			if row.Accounts[i].Priority != row.Accounts[j].Priority {
				return row.Accounts[i].Priority < row.Accounts[j].Priority
			}
			return row.Accounts[i].AccountID < row.Accounts[j].AccountID
		})
		out = append(out, row)
	}
	return out
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

// payAsYouGoTopupScenario projects the unit economics of a future ¥100 shop
// top-up from this group's observed customer charges and true upstream cost.
//
// Account multipliers are intentionally not used here. A public group can
// contain accounts for different model capabilities (for example, text and a
// fixed-price image renderer), so taking the largest bound multiplier can mix
// incompatible products and manufacture a false loss. The usage ledger has
// already preserved the account/model-specific billing semantics per request.
func payAsYouGoTopupScenario(rawCredits, realCostCNY float64) (*CostAccountingMoney, string) {
	if rawCredits <= 0 || realCostCNY < 0 {
		return nil, ""
	}

	netRevenue := costAccountingPayAsYouGoTopupCNY * (1 - ShopChannelFeePercent/100)
	blendedCostPerCredit := realCostCNY / rawCredits
	scenario := money(netRevenue, costAccountingPayAsYouGoTopupCNY*blendedCostPerCredit)
	return &scenario, CostAccountingScenarioBasisObservedUsage
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
// the current monthly-card products (GPT/Claude x Plus/Pro/Max) and every
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
	monthlyGroupIDs := costAccountingResolveMonthlyGroupIDs(activeGroups)

	creditGroupIDs := make([]int64, 0, 24)
	payAsYouGoGroupIDs := make([]int64, 0, len(payAsYouGoTargets))
	currentMonthlyGroupIDs := make(map[int64]struct{}, 8)
	for _, plan := range costAccountingMonthlyCardPlanOrder {
		for _, gid := range monthlyGroupIDs[plan] {
			currentMonthlyGroupIDs[gid] = struct{}{}
			creditGroupIDs = append(creditGroupIDs, gid)
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
		creditGroupIDs = append(creditGroupIDs, group.ID)
	}
	for _, group := range payAsYouGoTargets {
		payAsYouGoGroupIDs = append(payAsYouGoGroupIDs, group.ID)
	}

	var usageByGroup map[int64]CostAccountingUsageRow
	var usageErr error
	if s.opsRepo != nil {
		usageByGroup, usageErr = s.opsRepo.GetCostAccountingRealUsage(ctx, creditGroupIDs, payAsYouGoGroupIDs, windowStart, now)
	}

	overview := &CostAccountingOverview{
		GeneratedAt:                 now,
		ShopChannelFeePercent:       ShopChannelFeePercent,
		UsageWindowStart:            windowStart,
		UsageWindowEnd:              now,
		PricingSourceNote:           "guarded V3 catalog shared with redeem-code generation; keep the frontend product cards in sync",
		ScopeNote:                   "当前月卡只展示在售 Plus / Pro / Max；历史月卡分组不再作为产品卡展示，但其真实请求成本仍计入本月实际上游成本。按量付费自动读取全部启用的公开标准计费分组。",
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
		groupIDs := monthlyGroupIDs[planID]
		products := map[string]CostAccountingRate{}
		worstCostPerCredit := map[string]float64{}
		primaryCostPerCredit := map[string]float64{}
		var monthlyCredits float64

		for _, product := range []string{"gpt", "claude"} {
			gid := groupIDs[product]
			if gid <= 0 {
				return nil, fmt.Errorf("cost accounting: V3 monthly group missing (%s/%s)", planID, product)
			}
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
		for _, product := range []string{"gpt", "claude"} {
			cp := worstCostPerCredit[product]
			if cp > conservativeCP {
				conservativeCP = cp
			}
			cost := monthlyCredits * cp
			scenarios["all_"+product] = CostAccountingScenario{
				CostPerCreditWorstAccount: round4(cp),
				VsShopNetPrice:            money(shopNet, cost),
				VsDirectPrice:             money(direct, cost),
			}
		}
		conservativeCost := monthlyCredits * conservativeCP
		conservativeMargin := money(direct, conservativeCost).MarginPercent

		// Best case: all traffic goes through whichever product's cheapest
		// (primary/highest-priority) bound account is cheapest overall — e.g.
		// GPT's primary account at 0.15 vs its own worst-case fallback at
		// 0.20. This is what "全部用便宜账号" actually means; it must use
		// primaryCostPerCredit, not worstCostPerCredit, or it collapses to
		// the same number as the worst/conservative case.
		bestCP := 0.0
		haveBestCP := false
		for _, product := range []string{"gpt", "claude"} {
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
			for _, product := range []string{"gpt", "claude"} {
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
	}

	accountsByPayAsYouGoGroup := make(map[int64][]Account, len(payAsYouGoTargets))
	for _, group := range payAsYouGoTargets {
		accounts, err := s.accountRepo.ListByGroup(ctx, group.ID)
		if err != nil {
			return nil, fmt.Errorf("cost accounting: load pay-as-you-go group %d accounts: %w", group.ID, err)
		}
		accountsByPayAsYouGoGroup[group.ID] = accounts
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
			row.Warning = "当前无可调度账号，分组无法服务请求"
		}
		if row.RealUsage.Available {
			row.Topup100CNYScenario, row.Topup100CNYScenarioBasis = payAsYouGoTopupScenario(
				row.RealUsage.ObservedRawCredits,
				row.RealUsage.ObservedRealCostCNY,
			)
			if row.RealUsage.ObservedRawCredits <= 0 && row.RealUsage.ObservedRealCostCNY > 0 {
				row.Warning = "存在实际上游成本但没有用户计费额，需要对账"
			}
		}
		overview.PayAsYouGo = append(overview.PayAsYouGo, row)
	}
	overview.UpstreamRouting = costAccountingUpstreamRoutes(payAsYouGoTargets, accountsByPayAsYouGoGroup)

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

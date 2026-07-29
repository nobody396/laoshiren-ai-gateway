package service

import (
	"fmt"
	"math"
)

const (
	AffiliateCommercialShopFeeBPS            = int32(300)
	AffiliateCommercialMaxRewardPoolBPS      = int32(1000)
	AffiliateCommercialMaxRewardBurdenBPS    = int32(1200)
	AffiliateCommercialOperationalReserveBPS = int32(200)
	AffiliateCommercialStressCostPerCredit   = 0.37
	affiliatePayAsYouGoStressCostPerCredit   = 0.50
	AffiliateCommercialPricingTableVersionV3 = "v3-2026-07-28"
)

type AffiliateCommercialPackage struct {
	ID                         string  `json:"id"`
	Name                       string  `json:"name"`
	Kind                       string  `json:"kind"`
	ShopPriceCNY               float64 `json:"shop_price_cny"`
	DirectPriceCNY             float64 `json:"direct_price_cny"`
	PlatformCredits            float64 `json:"platform_credits"`
	DailyPlatformCredits       float64 `json:"daily_platform_credits,omitempty"`
	StressCostCNY              float64 `json:"stress_cost_cny"`
	ShopStressMarginPercent    float64 `json:"shop_stress_margin_percent"`
	DirectStressMarginPercent  float64 `json:"direct_stress_margin_percent"`
	PassesConfiguredMarginGate bool    `json:"passes_configured_margin_gate"`
}

type AffiliateCommercialGroupTarget struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	RateMultiplier float64 `json:"rate_multiplier"`
}

type AffiliateCommercialGPTCostMix struct {
	CheapAccountMultiplier     float64 `json:"cheap_account_multiplier"`
	ExpensiveAccountMultiplier float64 `json:"expensive_account_multiplier"`
	CheapTrafficPercent        float64 `json:"cheap_traffic_percent"`
	ExpensiveTrafficPercent    float64 `json:"expensive_traffic_percent"`
	BlendedAccountMultiplier   float64 `json:"blended_account_multiplier"`
}

type AffiliateCommercialPolicy struct {
	CashAssetSymbol            string                           `json:"cash_asset_symbol"`
	CreditAssetSymbol          string                           `json:"credit_asset_symbol"`
	ShopFeeBPS                 int32                            `json:"shop_fee_bps"`
	MaxRewardPoolBPS           int32                            `json:"max_reward_pool_bps"`
	MaxRewardBurdenBPS         int32                            `json:"max_reward_burden_bps"`
	OperationalReserveBPS      int32                            `json:"operational_reserve_bps"`
	MarginFloorBPS             int32                            `json:"margin_floor_bps"`
	StressCostPerCredit        float64                          `json:"stress_cost_per_credit"`
	PricingTableVersion        string                           `json:"pricing_table_version"`
	GPTCostMix                 AffiliateCommercialGPTCostMix    `json:"gpt_cost_mix"`
	GroupTargets               []AffiliateCommercialGroupTarget `json:"group_targets"`
	Packages                   []AffiliateCommercialPackage     `json:"packages"`
	MinimumStressMargin        float64                          `json:"minimum_stress_margin_percent"`
	PassesConfiguredMarginGate bool                             `json:"passes_configured_margin_gate"`
}

type affiliateCommercialPackageInput struct {
	id, name, kind string
	shopPrice      float64
	directPrice    float64
	credits        float64
	dailyCredits   float64
}

var affiliateCommercialPackageCatalog = []affiliateCommercialPackageInput{
	{id: "payg-20", name: "按量 ¥20", kind: "payg", shopPrice: 20, directPrice: 20, credits: 20},
	{id: "payg-50", name: "按量 ¥50", kind: "payg", shopPrice: 50, directPrice: 50, credits: 50},
	{id: "payg-100", name: "按量 ¥100", kind: "payg", shopPrice: 100, directPrice: 100, credits: 100},
	{id: "plus", name: "Plus", kind: "monthly", shopPrice: 259, directPrice: 249, credits: 300},
	{id: "pro", name: "Pro", kind: "monthly", shopPrice: 729, directPrice: 699, credits: 900},
	{id: "max", name: "Max", kind: "monthly", shopPrice: 1549, directPrice: 1499, credits: 2000},
}

var affiliateCommercialGroupTargets = []AffiliateCommercialGroupTarget{
	{ID: "gpt", Name: "GPT", RateMultiplier: 0.50},
	{ID: "claude-max", Name: "Claude / MAX", RateMultiplier: 2.40},
	{ID: "glm", Name: "GLM", RateMultiplier: 2.80},
	{ID: "grok", Name: "Grok", RateMultiplier: 0.40},
	{ID: "claude-external", Name: "Claude 外接", RateMultiplier: 2.60},
	{ID: "bedrock", Name: "Bedrock", RateMultiplier: 6.00},
	{ID: "high-cost", Name: "高成本", RateMultiplier: 8.50},
}

func BuildAffiliateCommercialPolicy(marginFloorBPS int32) AffiliateCommercialPolicy {
	return buildAffiliateCommercialPolicy(
		marginFloorBPS,
		AffiliateCommercialStressCostPerCredit,
		AffiliateCommercialOperationalReserveBPS,
	)
}

func BuildAffiliateCommercialPolicyFromSettings(settings AffiliateProgramSettings) AffiliateCommercialPolicy {
	stressCost := float64(settings.StressCostPerRawCreditMicros) / 1_000_000
	return buildAffiliateCommercialPolicy(
		settings.MarginFloorBPS,
		stressCost,
		settings.OperationalReserveBPS,
	)
}

func buildAffiliateCommercialPolicy(
	marginFloorBPS int32,
	monthlyStressCostPerCredit float64,
	operationalReserveBPS int32,
) AffiliateCommercialPolicy {
	gptMix := AffiliateCommercialGPTCostMix{
		CheapAccountMultiplier:     0.15,
		ExpensiveAccountMultiplier: 0.20,
		CheapTrafficPercent:        30,
		ExpensiveTrafficPercent:    70,
	}
	gptMix.BlendedAccountMultiplier =
		gptMix.CheapAccountMultiplier*gptMix.CheapTrafficPercent/100 +
			gptMix.ExpensiveAccountMultiplier*gptMix.ExpensiveTrafficPercent/100

	policy := AffiliateCommercialPolicy{
		CashAssetSymbol:            "¥",
		CreditAssetSymbol:          "⚡",
		ShopFeeBPS:                 AffiliateCommercialShopFeeBPS,
		MaxRewardPoolBPS:           AffiliateCommercialMaxRewardPoolBPS,
		MaxRewardBurdenBPS:         AffiliateCommercialMaxRewardBurdenBPS,
		OperationalReserveBPS:      operationalReserveBPS,
		MarginFloorBPS:             marginFloorBPS,
		StressCostPerCredit:        monthlyStressCostPerCredit,
		PricingTableVersion:        AffiliateCommercialPricingTableVersionV3,
		GPTCostMix:                 gptMix,
		GroupTargets:               append([]AffiliateCommercialGroupTarget(nil), affiliateCommercialGroupTargets...),
		Packages:                   make([]AffiliateCommercialPackage, 0, len(affiliateCommercialPackageCatalog)),
		MinimumStressMargin:        math.Inf(1),
		PassesConfiguredMarginGate: true,
	}

	for _, input := range affiliateCommercialPackageCatalog {
		stressUnitCost := monthlyStressCostPerCredit
		rewardBurdenBPS := AffiliateCommercialMaxRewardBurdenBPS
		if input.kind == "payg" {
			stressUnitCost = affiliatePayAsYouGoStressCostPerCredit
			rewardBurdenBPS = AffiliateCommercialMaxRewardPoolBPS
		}
		stressCost := input.credits * stressUnitCost
		displayCredits := input.credits
		dailyDisplayCredits := input.dailyCredits
		if input.kind == "monthly" {
			displayCredits *= 10
			dailyDisplayCredits *= 10
		}
		shopMargin := affiliateContributionMargin(
			input.shopPrice,
			stressCost,
			AffiliateCommercialShopFeeBPS,
			rewardBurdenBPS+operationalReserveBPS,
		)
		directMargin := affiliateContributionMargin(
			input.directPrice,
			stressCost,
			0,
			rewardBurdenBPS+operationalReserveBPS,
		)
		minMargin := math.Min(shopMargin, directMargin)
		passes := minMargin*100 >= float64(marginFloorBPS)-0.000001
		policy.Packages = append(policy.Packages, AffiliateCommercialPackage{
			ID:                         input.id,
			Name:                       input.name,
			Kind:                       input.kind,
			ShopPriceCNY:               input.shopPrice,
			DirectPriceCNY:             input.directPrice,
			PlatformCredits:            displayCredits,
			DailyPlatformCredits:       dailyDisplayCredits,
			StressCostCNY:              roundAffiliateCommercial(stressCost),
			ShopStressMarginPercent:    roundAffiliateCommercial(shopMargin),
			DirectStressMarginPercent:  roundAffiliateCommercial(directMargin),
			PassesConfiguredMarginGate: passes,
		})
		if minMargin < policy.MinimumStressMargin {
			policy.MinimumStressMargin = minMargin
		}
		if !passes {
			policy.PassesConfiguredMarginGate = false
		}
	}
	policy.MinimumStressMargin = roundAffiliateCommercial(policy.MinimumStressMargin)
	return policy
}

func ValidateAffiliateCommercialMarginFloor(marginFloorBPS int32) error {
	policy := BuildAffiliateCommercialPolicy(marginFloorBPS)
	return validateAffiliateCommercialPolicy(policy)
}

func ValidateAffiliateCommercialSettings(settings AffiliateProgramSettings) error {
	return validateAffiliateCommercialPolicy(BuildAffiliateCommercialPolicyFromSettings(settings))
}

func validateAffiliateCommercialPolicy(policy AffiliateCommercialPolicy) error {
	if policy.PassesConfiguredMarginGate {
		return nil
	}
	return fmt.Errorf(
		"%w: catalog minimum stress margin %.2f%% is below configured floor %.2f%%",
		ErrAffiliateProgramSettingsInvalid,
		policy.MinimumStressMargin,
		float64(policy.MarginFloorBPS)/100,
	)
}

func affiliateContributionMargin(price, stressCost float64, feeBPS, rewardBPS int32) float64 {
	if price <= 0 {
		return 0
	}
	fee := price * float64(feeBPS) / 10_000
	reward := price * float64(rewardBPS) / 10_000
	return ((price - fee - reward - stressCost) / price) * 100
}

func roundAffiliateCommercial(value float64) float64 {
	return math.Round(value*100) / 100
}

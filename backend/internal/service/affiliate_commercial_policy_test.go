package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAffiliateCommercialPolicy_CatalogAndMarginGate(t *testing.T) {
	policy := BuildAffiliateCommercialPolicy(3500)

	require.Equal(t, "¥", policy.CashAssetSymbol)
	require.Equal(t, "⚡", policy.CreditAssetSymbol)
	require.Equal(t, int32(300), policy.ShopFeeBPS)
	require.Equal(t, int32(1000), policy.MaxRewardPoolBPS)
	require.Equal(t, 0.185, policy.GPTCostMix.BlendedAccountMultiplier)
	require.True(t, policy.PassesConfiguredMarginGate)
	require.GreaterOrEqual(t, policy.MinimumStressMargin, 35.0)
	require.Len(t, policy.Packages, 6)

	byID := make(map[string]AffiliateCommercialPackage, len(policy.Packages))
	for _, item := range policy.Packages {
		byID[item.ID] = item
		require.True(t, item.PassesConfiguredMarginGate, item.ID)
	}
	require.Equal(t, float64(20), byID["payg-20"].ShopPriceCNY)
	require.Equal(t, float64(20), byID["payg-20"].PlatformCredits)
	require.Equal(t, float64(259), byID["starter"].ShopPriceCNY)
	require.Equal(t, float64(249), byID["starter"].DirectPriceCNY)
	require.Equal(t, float64(2400), byID["starter"].PlatformCredits)
	require.Equal(t, float64(80), byID["starter"].DailyPlatformCredits)
	require.Equal(t, float64(469), byID["lite"].ShopPriceCNY)
	require.Equal(t, float64(459), byID["lite"].DirectPriceCNY)
	require.Equal(t, float64(4500), byID["lite"].PlatformCredits)
	require.Equal(t, float64(150), byID["lite"].DailyPlatformCredits)
	require.Equal(t, float64(869), byID["pro"].ShopPriceCNY)
	require.Equal(t, float64(839), byID["pro"].DirectPriceCNY)
	require.Equal(t, float64(8500), byID["pro"].PlatformCredits)
	require.Equal(t, float64(280), byID["pro"].DailyPlatformCredits)
}

func TestAffiliateCommercialPolicy_RejectsUnmetHigherMarginFloor(t *testing.T) {
	err := ValidateAffiliateCommercialMarginFloor(4000)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAffiliateProgramSettingsInvalid))
}

func TestAffiliateCommercialPolicy_GroupTargets(t *testing.T) {
	policy := BuildAffiliateCommercialPolicy(3500)
	targets := make(map[string]float64, len(policy.GroupTargets))
	for _, item := range policy.GroupTargets {
		targets[item.ID] = item.RateMultiplier
	}
	require.Equal(t, 0.50, targets["gpt"])
	require.Equal(t, 2.40, targets["claude-max"])
	require.Equal(t, 2.80, targets["glm"])
	require.Equal(t, 0.40, targets["grok"])
	require.Equal(t, 2.60, targets["claude-external"])
	require.Equal(t, 6.00, targets["bedrock"])
	require.Equal(t, 8.50, targets["high-cost"])
}

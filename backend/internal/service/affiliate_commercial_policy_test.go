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
	require.Equal(t, int32(1200), policy.MaxRewardBurdenBPS)
	require.Equal(t, int32(200), policy.OperationalReserveBPS)
	require.Equal(t, 0.37, policy.StressCostPerCredit)
	require.Equal(t, AffiliateCommercialPricingTableVersionV3, policy.PricingTableVersion)
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
	require.Equal(t, float64(259), byID["plus"].ShopPriceCNY)
	require.Equal(t, float64(249), byID["plus"].DirectPriceCNY)
	require.Equal(t, float64(3000), byID["plus"].PlatformCredits)
	require.Zero(t, byID["plus"].DailyPlatformCredits)
	require.Equal(t, float64(729), byID["pro"].ShopPriceCNY)
	require.Equal(t, float64(699), byID["pro"].DirectPriceCNY)
	require.Equal(t, float64(9000), byID["pro"].PlatformCredits)
	require.Zero(t, byID["pro"].DailyPlatformCredits)
	require.Equal(t, float64(1549), byID["max"].ShopPriceCNY)
	require.Equal(t, float64(1499), byID["max"].DirectPriceCNY)
	require.Equal(t, float64(20000), byID["max"].PlatformCredits)
	require.Zero(t, byID["max"].DailyPlatformCredits)
	require.InDelta(t, 35.23, byID["max"].ShopStressMarginPercent, 0.01)
}

func TestAffiliateCommercialPolicy_RejectsUnmetHigherMarginFloor(t *testing.T) {
	err := ValidateAffiliateCommercialMarginFloor(4000)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAffiliateProgramSettingsInvalid))
}

func TestAffiliateCommercialPolicy_UsesPersistedStressSnapshotInputs(t *testing.T) {
	settings := DefaultAffiliateProgramSettings()
	settings.StressCostPerRawCreditMicros = 540_000
	settings.OperationalReserveBPS = 250

	policy := BuildAffiliateCommercialPolicyFromSettings(settings)

	require.Equal(t, 0.54, policy.StressCostPerCredit)
	require.Equal(t, int32(250), policy.OperationalReserveBPS)
	require.False(t, policy.PassesConfiguredMarginGate)
	require.Error(t, ValidateAffiliateCommercialSettings(settings))
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

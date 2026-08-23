package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClassifyCustomerTierUsesExactRollingPaidValueBoundaries(t *testing.T) {
	tests := []struct {
		paid int64
		want CustomerTier
	}{
		{0, CustomerTierStandard}, {24_999, CustomerTierStandard},
		{25_000, CustomerTierPriority}, {99_999, CustomerTierPriority},
		{100_000, CustomerTierStrategic},
	}
	for _, test := range tests {
		require.Equal(t, test.want, ClassifyCustomerTier(test.paid), test.paid)
	}
}

func TestResolveCustomerTierAppliesImmediateUpgradeAndThirtyDayDowngradeGrace(t *testing.T) {
	now := time.Date(2026, 8, 23, 4, 0, 0, 0, time.UTC)
	upgrade := ResolveCustomerTier(CustomerTierResolutionInput{Now: now, CurrentTier: CustomerTierStandard, CalculatedTier: CustomerTierPriority})
	require.Equal(t, CustomerTierPriority, upgrade.EffectiveTier)
	require.Nil(t, upgrade.GraceExpiresAt)
	require.Equal(t, "upgrade", upgrade.Reason)

	downgrade := ResolveCustomerTier(CustomerTierResolutionInput{Now: now, CurrentTier: CustomerTierStrategic, CalculatedTier: CustomerTierStandard})
	require.Equal(t, CustomerTierStrategic, downgrade.EffectiveTier)
	require.Equal(t, now.Add(30*24*time.Hour), *downgrade.GraceExpiresAt)
	require.Equal(t, "downgrade_grace_started", downgrade.Reason)

	duringGrace := ResolveCustomerTier(CustomerTierResolutionInput{Now: now.Add(20 * 24 * time.Hour), CurrentTier: CustomerTierStrategic, CalculatedTier: CustomerTierStandard, GraceExpiresAt: downgrade.GraceExpiresAt})
	require.Equal(t, CustomerTierStrategic, duringGrace.EffectiveTier)
	require.Equal(t, "downgrade_grace_active", duringGrace.Reason)

	afterGrace := ResolveCustomerTier(CustomerTierResolutionInput{Now: now.Add(30 * 24 * time.Hour), CurrentTier: CustomerTierStrategic, CalculatedTier: CustomerTierStandard, GraceExpiresAt: downgrade.GraceExpiresAt})
	require.Equal(t, CustomerTierStandard, afterGrace.EffectiveTier)
	require.Nil(t, afterGrace.GraceExpiresAt)
	require.Equal(t, "downgrade_grace_expired", afterGrace.Reason)
}

func TestResolveCustomerTierOverrideIsTimeBoundAndDoesNotCreateDowngradeGrace(t *testing.T) {
	now := time.Now().UTC()
	active := ResolveCustomerTier(CustomerTierResolutionInput{Now: now, CurrentTier: CustomerTierStandard, CalculatedTier: CustomerTierStandard, OverrideTier: CustomerTierStrategic, OverrideActive: true})
	require.Equal(t, CustomerTierStrategic, active.EffectiveTier)
	require.Equal(t, "active_override", active.Reason)

	expired := ResolveCustomerTier(CustomerTierResolutionInput{Now: now.Add(time.Hour), CurrentTier: CustomerTierStrategic, CalculatedTier: CustomerTierStandard, PreviousOverrideActive: true})
	require.Equal(t, CustomerTierStandard, expired.EffectiveTier)
	require.Nil(t, expired.GraceExpiresAt)
	require.Equal(t, "override_expired", expired.Reason)
}

func TestCustomerTierBenefitsAffectSizeAndCapOnly(t *testing.T) {
	require.Equal(t, CustomerTierBenefit{Multiplier: 1, CapCNYFen: 1_000}, CustomerTierStandard.Benefit())
	require.Equal(t, CustomerTierBenefit{Multiplier: 1.25, CapCNYFen: 5_000}, CustomerTierPriority.Benefit())
	require.Equal(t, CustomerTierBenefit{Multiplier: 1.5, CapCNYFen: 20_000}, CustomerTierStrategic.Benefit())
}

func TestCustomerTierAnonymizedDistributionReplay(t *testing.T) {
	values := []int64{0, 100, 200, 300, 400, 500, 600, 700, 800, 900, 1_000, 1_100, 1_200, 1_300, 1_400, 1_500, 1_600, 1_700, 1_800, 1_900, 2_000, 3_000, 5_000, 10_000, 20_000, 24_999, 25_000, 30_000, 50_000, 75_000, 99_999, 100_000}
	counts := map[CustomerTier]int{}
	for _, value := range values {
		counts[ClassifyCustomerTier(value)]++
	}
	require.Equal(t, 26, counts[CustomerTierStandard])
	require.Equal(t, 5, counts[CustomerTierPriority])
	require.Equal(t, 1, counts[CustomerTierStrategic])
}

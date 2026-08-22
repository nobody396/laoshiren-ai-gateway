//go:build unit

package service

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func float64Pointer(value float64) *float64 { return &value }

func TestChannelFastAndFlexMultipliersApplyToChannelStandardPrice(t *testing.T) {
	svc := newTestBillingService()
	pricing := &ModelPricing{
		InputPricePerToken:  1e-6,
		OutputPricePerToken: 4e-6,
		FastMultiplier:      float64Pointer(2.5),
		FlexMultiplier:      float64Pointer(0.4),
	}
	tokens := UsageTokens{InputTokens: 1000, OutputTokens: 100}

	fast := svc.computeTokenBreakdown(pricing, tokens, 1, "priority", false)
	flex := svc.computeTokenBreakdown(pricing, tokens, 1, "flex", false)
	standard := svc.computeTokenBreakdown(pricing, tokens, 1, "", false)

	require.InDelta(t, standard.TotalCost*2.5, fast.TotalCost, 1e-12)
	require.InDelta(t, standard.TotalCost*0.4, flex.TotalCost, 1e-12)
}

func TestValidateChannelPricingRequiresVerifiedServiceTierCapability(t *testing.T) {
	multiplier := 2.0
	err := validatePricingEntries([]ChannelModelPricing{{
		Platform:       PlatformOpenAI,
		Models:         []string{"gpt-5.4"},
		BillingMode:    BillingModeToken,
		FastMultiplier: &multiplier,
		FastSupported:  true,
	}})
	require.Error(t, err)

	verifiedAt := time.Now().UTC().Add(-time.Hour)
	err = validatePricingEntries([]ChannelModelPricing{{
		Platform:       PlatformOpenAI,
		Models:         []string{"gpt-5.4"},
		BillingMode:    BillingModeToken,
		FastMultiplier: &multiplier,
		FastSupported:  true,
		FastVerifiedAt: &verifiedAt,
	}})
	require.NoError(t, err)

	err = validatePricingEntries([]ChannelModelPricing{{
		Platform:       PlatformAnthropic,
		Models:         []string{"claude-sonnet-4"},
		BillingMode:    BillingModeToken,
		FastMultiplier: &multiplier,
		FastSupported:  true,
		FastVerifiedAt: &verifiedAt,
	}})
	require.Error(t, err)
}

func TestChannelOverridePreservesCatalogPriorityRatioWithoutExplicitMultiplier(t *testing.T) {
	pricing := &ModelPricing{
		InputPricePerToken:          2e-6,
		InputPricePerTokenPriority:  6e-6,
		OutputPricePerToken:         4e-6,
		OutputPricePerTokenPriority: 8e-6,
	}
	applyChannelTokenPriceOverrides(pricing, &ChannelModelPricing{
		InputPrice:  float64Pointer(10e-6),
		OutputPrice: float64Pointer(20e-6),
	})

	require.InDelta(t, 10e-6, pricing.InputPricePerToken, 1e-15)
	require.InDelta(t, 30e-6, pricing.InputPricePerTokenPriority, 1e-15)
	require.InDelta(t, 20e-6, pricing.OutputPricePerToken, 1e-15)
	require.InDelta(t, 40e-6, pricing.OutputPricePerTokenPriority, 1e-15)
}

func TestGetModelPricingWithChannelDoesNotMutateSharedFallback(t *testing.T) {
	svc := newTestBillingService()
	before, err := svc.GetModelPricing("gpt-5.4")
	require.NoError(t, err)
	baseInput := before.InputPricePerToken

	_, err = svc.GetModelPricingWithChannel("gpt-5.4", &ChannelModelPricing{InputPrice: float64Pointer(99e-6)})
	require.NoError(t, err)

	after, err := svc.GetModelPricing("gpt-5.4")
	require.NoError(t, err)
	require.InDelta(t, baseInput, after.InputPricePerToken, 1e-15)
}

func TestValidateChannelPricingRejectsNonPositiveServiceTierMultiplier(t *testing.T) {
	zero := 0.0
	err := validatePricingEntries([]ChannelModelPricing{{
		Models:         []string{"gpt-5.4"},
		BillingMode:    BillingModeToken,
		FastMultiplier: &zero,
	}})
	require.Error(t, err)

	nan := math.NaN()
	err = validatePricingEntries([]ChannelModelPricing{{
		Models:         []string{"gpt-5.4"},
		BillingMode:    BillingModeToken,
		FastMultiplier: &nan,
	}})
	require.Error(t, err)
}

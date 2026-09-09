package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTryCustomRulesUsesSettledBaseCostMultiplier(t *testing.T) {
	multiplier := 0.7
	channel := &Channel{AccountStatsPricingRules: []AccountStatsPricingRule{{
		AccountIDs: []int64{77},
		Pricing: []ChannelModelPricing{{
			Platform: PlatformOpenAI, Models: []string{"deepseek-v4-pro-0813"}, CostMultiplier: &multiplier,
		}},
	}}}

	cost := tryCustomRules(channel, 77, 12, PlatformOpenAI, "deepseek-v4-pro-0813", UsageTokens{
		InputTokens: 1000, OutputTokens: 200,
	}, 1, 9.0)

	require.NotNil(t, cost)
	require.InDelta(t, 6.3, *cost, 1e-12)
}

func TestTryCustomRulesCostMultiplierKeepsPerRequestSelection(t *testing.T) {
	multiplier := 0.45
	channel := &Channel{AccountStatsPricingRules: []AccountStatsPricingRule{{
		GroupIDs: []int64{88},
		Pricing: []ChannelModelPricing{{
			Platform: PlatformOpenAI, Models: []string{"glm-5.2"}, CostMultiplier: &multiplier,
		}},
	}}}

	// totalCost already contains the customer-side context/time price selected
	// at request start. The supplier calculation must reuse it verbatim.
	cost := tryCustomRules(channel, 1, 88, PlatformOpenAI, "glm-5.2", UsageTokens{}, 1, 28.0)

	require.NotNil(t, cost)
	require.InDelta(t, 12.6, *cost, 1e-12)
}

func TestTryCustomRulesWithoutMultiplierKeepsExplicitPricing(t *testing.T) {
	input := 2e-6
	channel := &Channel{AccountStatsPricingRules: []AccountStatsPricingRule{{
		AccountIDs: []int64{77},
		Pricing: []ChannelModelPricing{{
			Platform: PlatformOpenAI, Models: []string{"legacy"}, InputPrice: &input,
		}},
	}}}

	cost := tryCustomRules(channel, 77, 12, PlatformOpenAI, "legacy", UsageTokens{InputTokens: 1000}, 1, 99)

	require.NotNil(t, cost)
	require.InDelta(t, 0.002, *cost, 1e-12)
}

func TestValidatePricingRejectsInvalidCostMultiplier(t *testing.T) {
	zero := 0.0
	err := validatePricingEntries([]ChannelModelPricing{{
		Platform: PlatformOpenAI, Models: []string{"bad"}, CostMultiplier: &zero,
	}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cost_multiplier")
}

func TestTryModelFilePricingDoesNotDoubleChargeImageOutput(t *testing.T) {
	svc := &BillingService{}
	tokens := UsageTokens{
		InputTokens:       40,
		ImageInputTokens:  100,
		OutputTokens:      196,
		ImageOutputTokens: 196,
	}

	cost := tryModelFilePricing(svc, "gpt-image-2.5-sunburst", tokens)
	require.NotNil(t, cost)
	require.InDelta(t, 40*5e-6+100*8e-6+196*30e-6, *cost, 1e-12)
}

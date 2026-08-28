//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func channelTimePricingFloat(v float64) *float64 { return &v }

func TestCalculateCostUnifiedAppliesChannelTimeMultiplierToTokenBuckets(t *testing.T) {
	groupID := int64(71)
	pricing := &ChannelModelPricing{
		BillingMode: BillingModeToken,
		InputPrice:  channelTimePricingFloat(5e-6),
		TimePricing: &ChannelTimePricing{
			Timezone: "Asia/Shanghai",
			Periods:  []ChannelTimePricingPeriod{{StartTime: "09:00", EndTime: "12:00", Multiplier: 2}},
		},
	}
	resolved := &ResolvedPricing{
		Mode:           BillingModeToken,
		Source:         PricingSourceChannel,
		BasePricing:    &ModelPricing{InputPricePerToken: 5e-6, OutputPricePerToken: 15e-6},
		channelPricing: pricing,
	}
	service := newTestBillingService()
	resolver := NewModelPricingResolver(nil, service)

	cost, err := service.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "claude-sonnet-4",
		GroupID:        &groupID,
		Tokens:         UsageTokens{InputTokens: 100, OutputTokens: 10},
		RateMultiplier: 3,
		PricingAt:      time.Date(2026, 6, 29, 1, 0, 0, 0, time.UTC),
		Resolver:       resolver,
		Resolved:       resolved,
	})
	require.NoError(t, err)
	want := (100*5e-6 + 10*15e-6) * 2
	require.InDelta(t, want, cost.TotalCost, 1e-12)
	require.InDelta(t, want*3, cost.ActualCost, 1e-12)
	require.Contains(t, cost.BillingTier, "time=Asia/Shanghai,09:00-12:00,x2")
	require.Contains(t, cost.BillingTier, "at=2026-06-29T01:00:00Z")
}

func TestUsagePricingAtUsesRequestStart(t *testing.T) {
	settled := time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC)
	require.Equal(t, settled.Add(-2*time.Minute), usagePricingAt(func() time.Time { return settled }, 2*time.Minute))
}

func TestCalculateCostUnifiedRecordsContextTierEvidence(t *testing.T) {
	maxTokens := 100000
	resolved := &ResolvedPricing{
		Mode:        BillingModeToken,
		BasePricing: &ModelPricing{InputPricePerToken: 2e-6, OutputPricePerToken: 10e-6},
		Intervals: []PricingInterval{{
			MinTokens: 0, MaxTokens: &maxTokens,
			InputPrice: channelTimePricingFloat(3e-6), OutputPrice: channelTimePricingFloat(12e-6),
		}},
	}
	service := newTestBillingService()
	cost, err := service.CalculateCostUnified(CostInput{
		Ctx: context.Background(), Model: "model", Tokens: UsageTokens{InputTokens: 90000},
		RateMultiplier: 1, Resolver: NewModelPricingResolver(nil, service), Resolved: resolved,
	})
	require.NoError(t, err)
	require.Equal(t, "context=(0,100000]", cost.BillingTier)
}

func TestCalculateCostUnifiedDoesNotApplyChannelTimeMultiplierToPerRequest(t *testing.T) {
	groupID := int64(72)
	pricing := &ChannelModelPricing{
		BillingMode:     BillingModePerRequest,
		PerRequestPrice: channelTimePricingFloat(0.05),
		TimePricing: &ChannelTimePricing{
			Timezone: "Asia/Shanghai",
			Periods:  []ChannelTimePricingPeriod{{StartTime: "09:00", EndTime: "12:00", Multiplier: 2}},
		},
	}
	resolved := &ResolvedPricing{Mode: BillingModePerRequest, Source: PricingSourceChannel, channelPricing: pricing, DefaultPerRequestPrice: 0.05}
	service := newTestBillingService()
	resolver := NewModelPricingResolver(nil, service)

	cost, err := service.CalculateCostUnified(CostInput{
		Ctx:            context.Background(),
		Model:          "image-model",
		GroupID:        &groupID,
		RequestCount:   3,
		RateMultiplier: 2,
		PricingAt:      time.Date(2026, 6, 29, 1, 0, 0, 0, time.UTC),
		Resolver:       resolver,
		Resolved:       resolved,
	})
	require.NoError(t, err)
	require.InDelta(t, 0.15, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.30, cost.ActualCost, 1e-12)
}

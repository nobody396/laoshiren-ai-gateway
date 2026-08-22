//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelPricingServiceTierMultipliersRoundTrip(t *testing.T) {
	ctx := context.Background()
	repo := NewChannelRepository(integrationDB)
	fast := 2.5
	flex := 0.4
	channel := &service.Channel{
		Name:               fmt.Sprintf("pr1-tier-multiplier-%d", time.Now().UnixNano()),
		Status:             service.StatusActive,
		BillingModelSource: service.BillingModelSourceRequested,
		ModelPricing: []service.ChannelModelPricing{{
			Platform: service.PlatformOpenAI, Models: []string{"gpt-5.4"}, BillingMode: service.BillingModeToken,
			FastMultiplier: &fast, FlexMultiplier: &flex,
		}},
		ApplyPricingToAccountStats: true,
		AccountStatsPricingRules: []service.AccountStatsPricingRule{{
			Name: "supplier cost", GroupIDs: []int64{}, AccountIDs: []int64{},
			Pricing: []service.ChannelModelPricing{{
				Platform: service.PlatformOpenAI, Models: []string{"gpt-5.4"}, BillingMode: service.BillingModeToken,
				FastMultiplier: &fast, FlexMultiplier: &flex,
			}},
		}},
	}

	require.NoError(t, repo.Create(ctx, channel))
	t.Cleanup(func() { _ = repo.Delete(context.Background(), channel.ID) })

	readback, err := repo.GetByID(ctx, channel.ID)
	require.NoError(t, err)
	require.Len(t, readback.ModelPricing, 1)
	require.NotNil(t, readback.ModelPricing[0].FastMultiplier)
	require.NotNil(t, readback.ModelPricing[0].FlexMultiplier)
	require.InDelta(t, fast, *readback.ModelPricing[0].FastMultiplier, 1e-9)
	require.InDelta(t, flex, *readback.ModelPricing[0].FlexMultiplier, 1e-9)

	require.Len(t, readback.AccountStatsPricingRules, 1)
	require.Len(t, readback.AccountStatsPricingRules[0].Pricing, 1)
	require.InDelta(t, fast, *readback.AccountStatsPricingRules[0].Pricing[0].FastMultiplier, 1e-9)
	require.InDelta(t, flex, *readback.AccountStatsPricingRules[0].Pricing[0].FlexMultiplier, 1e-9)
}

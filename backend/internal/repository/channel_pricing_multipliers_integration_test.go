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
	verifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	channel := &service.Channel{
		Name:               fmt.Sprintf("pr1-tier-multiplier-%d", time.Now().UnixNano()),
		Status:             service.StatusActive,
		BillingModelSource: service.BillingModelSourceRequested,
		ModelPricing: []service.ChannelModelPricing{{
			Platform: service.PlatformOpenAI, Models: []string{"gpt-5.4"}, BillingMode: service.BillingModeToken,
			FastMultiplier: &fast, FlexMultiplier: &flex,
			FastSupported: true, FlexSupported: true, FastVerifiedAt: &verifiedAt, FlexVerifiedAt: &verifiedAt,
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
	require.True(t, readback.ModelPricing[0].FastSupported)
	require.True(t, readback.ModelPricing[0].FlexSupported)
	require.NotNil(t, readback.ModelPricing[0].FastVerifiedAt)
	require.NotNil(t, readback.ModelPricing[0].FlexVerifiedAt)
	require.Equal(t, verifiedAt, readback.ModelPricing[0].FastVerifiedAt.UTC())
	require.Equal(t, verifiedAt, readback.ModelPricing[0].FlexVerifiedAt.UTC())

	_, err = integrationDB.ExecContext(ctx,
		`UPDATE channel_model_pricing SET fast_multiplier = NULL WHERE id = $1`,
		readback.ModelPricing[0].ID,
	)
	require.Error(t, err, "database must reject a supported tier without a positive multiplier")

	require.Len(t, readback.AccountStatsPricingRules, 1)
	require.Len(t, readback.AccountStatsPricingRules[0].Pricing, 1)
	require.InDelta(t, fast, *readback.AccountStatsPricingRules[0].Pricing[0].FastMultiplier, 1e-9)
	require.InDelta(t, flex, *readback.AccountStatsPricingRules[0].Pricing[0].FlexMultiplier, 1e-9)
}

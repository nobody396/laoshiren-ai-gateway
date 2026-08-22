package admin

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPricingRequestToServiceNormalizesCapabilityVerification(t *testing.T) {
	multiplier := 2.0
	verifiedAt := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	pricing := pricingRequestToService([]channelModelPricingRequest{{
		Models:         []string{"gpt-5.4"},
		FastMultiplier: &multiplier,
		FastSupported:  true,
		FastVerifiedAt: &verifiedAt,
		FlexSupported:  false,
		FlexVerifiedAt: &verifiedAt,
	}})

	require.Len(t, pricing, 1)
	require.True(t, pricing[0].FastSupported)
	require.NotNil(t, pricing[0].FastVerifiedAt)
	require.Equal(t, verifiedAt, *pricing[0].FastVerifiedAt)
	require.False(t, pricing[0].FlexSupported)
	require.Nil(t, pricing[0].FlexVerifiedAt, "disabled tiers must not retain stale verification evidence")
}

func TestPricingRequestToServiceStampsNewCapabilityConfirmation(t *testing.T) {
	multiplier := 2.0
	before := time.Now().UTC().Add(-time.Second)
	pricing := pricingRequestToService([]channelModelPricingRequest{{
		Models:         []string{"gpt-5.4"},
		FastMultiplier: &multiplier,
		FastSupported:  true,
	}})
	after := time.Now().UTC().Add(time.Second)

	require.NotNil(t, pricing[0].FastVerifiedAt)
	require.True(t, pricing[0].FastVerifiedAt.After(before))
	require.True(t, pricing[0].FastVerifiedAt.Before(after))
}

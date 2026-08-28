//go:build unit

package admin

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPricingRequestToServiceTimePricing(t *testing.T) {
	pricing := pricingRequestToService([]channelModelPricingRequest{{
		Platform: "openai",
		Models:   []string{"gpt-5"},
		TimePricing: &channelTimePricingRequest{
			Timezone:     "Asia/Shanghai",
			WeekdaysOnly: true,
			Periods: []channelTimePricingPeriodRequest{{
				StartTime: "09:00", EndTime: "12:00", Multiplier: 1.5,
			}},
		},
	}})
	require.Len(t, pricing, 1)
	require.NotNil(t, pricing[0].TimePricing)
	require.Equal(t, "Asia/Shanghai", pricing[0].TimePricing.Timezone)
	require.True(t, pricing[0].TimePricing.WeekdaysOnly)
	require.Equal(t, 1.5, pricing[0].TimePricing.Periods[0].Multiplier)

	response := pricingToResponse(&pricing[0])
	require.NotNil(t, response.TimePricing)
	require.Equal(t, "09:00", response.TimePricing.Periods[0].StartTime)
}

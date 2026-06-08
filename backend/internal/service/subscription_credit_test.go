package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroup_IsSubscriptionTypeIncludesCredit(t *testing.T) {
	require.False(t, (&Group{SubscriptionType: SubscriptionTypeStandard}).IsSubscriptionType())
	require.True(t, (&Group{SubscriptionType: SubscriptionTypeSubscription}).IsSubscriptionType())
	require.True(t, (&Group{SubscriptionType: SubscriptionTypeCredit}).IsSubscriptionType())
}

func TestSubscriptionUsageCost(t *testing.T) {
	cost := &CostBreakdown{TotalCost: 10, ActualCost: 6}

	require.Equal(t, 10.0, subscriptionUsageCost(&postUsageBillingParams{
		Cost:               cost,
		IsSubscriptionBill: true,
		APIKey: &APIKey{
			Group: &Group{SubscriptionType: SubscriptionTypeSubscription},
		},
	}))

	require.Equal(t, 6.0, subscriptionUsageCost(&postUsageBillingParams{
		Cost:               cost,
		IsSubscriptionBill: true,
		APIKey: &APIKey{
			Group: &Group{SubscriptionType: SubscriptionTypeCredit},
		},
	}))

	require.Equal(t, 0.0, subscriptionUsageCost(&postUsageBillingParams{
		Cost:               cost,
		IsSubscriptionBill: false,
		APIKey: &APIKey{
			Group: &Group{SubscriptionType: SubscriptionTypeCredit},
		},
	}))
}

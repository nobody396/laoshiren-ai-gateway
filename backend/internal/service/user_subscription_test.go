package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserSubscriptionCheckMonthlyLimitTreatsTinyTailAsExhausted(t *testing.T) {
	limit := 450.0
	group := &Group{MonthlyLimitUSD: &limit}

	require.False(t, (&UserSubscription{MonthlyUsageUSD: 449.9999175202}).CheckMonthlyLimit(group, 0))
	require.False(t, (&UserSubscription{MonthlyUsageUSD: 449.9998}).CheckMonthlyLimit(group, 0.0002))
	require.True(t, (&UserSubscription{MonthlyUsageUSD: 449.99}).CheckMonthlyLimit(group, 0))
}

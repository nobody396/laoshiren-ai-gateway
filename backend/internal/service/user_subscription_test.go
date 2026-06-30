package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUserSubscriptionCheckMonthlyLimitTreatsTinyTailAsExhausted(t *testing.T) {
	limit := 450.0
	group := &Group{MonthlyLimitUSD: &limit}

	require.False(t, (&UserSubscription{MonthlyUsageUSD: 449.9999175202}).CheckMonthlyLimit(group, 0))
	require.False(t, (&UserSubscription{MonthlyUsageUSD: 449.9998}).CheckMonthlyLimit(group, 0.0002))
	require.True(t, (&UserSubscription{MonthlyUsageUSD: 449.99}).CheckMonthlyLimit(group, 0))
}

func TestRollingUsageWindowStartPreservesActivationMoment(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	activation := time.Date(2026, 6, 30, 14, 22, 31, 123456000, loc)

	windowStart := rollingUsageWindowStart(activation)

	require.Equal(t, activation, windowStart)
	require.NotEqual(t, startOfDay(activation), windowStart, "monthly windows must not be rounded down to midnight")
	require.Equal(t, activation.Add(30*24*time.Hour), windowStart.Add(30*24*time.Hour))
}

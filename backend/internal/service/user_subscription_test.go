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
	require.Equal(t, activation.Add(SubscriptionMonthlyWindowDuration), windowStart.Add(SubscriptionMonthlyWindowDuration))
}

func TestMonthlyResetTimeUsesThirtyOneDayCycle(t *testing.T) {
	start := time.Date(2026, 6, 30, 21, 13, 49, 0, time.FixedZone("CST", 8*3600))

	resetAt := (&UserSubscription{MonthlyWindowStart: &start}).MonthlyResetTime()

	require.NotNil(t, resetAt)
	require.Equal(t, start.Add(31*24*time.Hour), *resetAt)
}

func TestMonthlyResetTimeIsCappedAtSubscriptionExpiry(t *testing.T) {
	start := time.Date(2026, 6, 12, 0, 0, 0, 0, time.FixedZone("CST", 8*3600))
	expiresAt := time.Date(2026, 7, 12, 14, 49, 41, 0, time.FixedZone("CST", 8*3600))

	resetAt := (&UserSubscription{ExpiresAt: expiresAt, MonthlyWindowStart: &start}).MonthlyResetTime()

	require.NotNil(t, resetAt)
	require.Equal(t, expiresAt, *resetAt)
}

func TestDaysRemainingRoundsUpPartialDays(t *testing.T) {
	sub := &UserSubscription{ExpiresAt: time.Now().Add(30*24*time.Hour + time.Hour)}

	require.Equal(t, 31, sub.DaysRemaining())
}

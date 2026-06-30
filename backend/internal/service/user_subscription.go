package service

import (
	"math"
	"time"
)

// SubscriptionUsageLimitEpsilonUSD is the smallest remaining subscription
// balance we still treat as usable. It closes floating-point / decimal tail
// gaps such as 449.9999175202 / 450.00000000, where the UI is already full and
// any real model request would exceed the cap after the response has streamed.
const SubscriptionUsageLimitEpsilonUSD = 0.0001

// SubscriptionMonthlyWindowDuration is the default monthly-card quota window.
// Product-wise, a monthly card is sold as a 31-day cycle; keep quota reset and
// remaining-day displays aligned with that customer-facing entitlement.
const SubscriptionMonthlyWindowDuration = 31 * 24 * time.Hour

type UserSubscription struct {
	ID      int64
	UserID  int64
	GroupID int64

	StartsAt  time.Time
	ExpiresAt time.Time
	Status    string

	DailyWindowStart   *time.Time
	WeeklyWindowStart  *time.Time
	MonthlyWindowStart *time.Time

	DailyUsageUSD   float64
	WeeklyUsageUSD  float64
	MonthlyUsageUSD float64

	AssignedBy *int64
	AssignedAt time.Time
	Notes      string

	CreatedAt time.Time
	UpdatedAt time.Time

	User           *User
	Group          *Group
	AssignedByUser *User
}

func (s *UserSubscription) IsActive() bool {
	return s.Status == SubscriptionStatusActive && time.Now().Before(s.ExpiresAt)
}

func (s *UserSubscription) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

func (s *UserSubscription) DaysRemaining() int {
	if s.IsExpired() {
		return 0
	}
	return int(math.Ceil(time.Until(s.ExpiresAt).Hours() / 24))
}

func (s *UserSubscription) IsWindowActivated() bool {
	return s.DailyWindowStart != nil || s.WeeklyWindowStart != nil || s.MonthlyWindowStart != nil
}

func (s *UserSubscription) NeedsDailyReset() bool {
	if s.DailyWindowStart == nil {
		return false
	}
	return time.Since(*s.DailyWindowStart) >= 24*time.Hour
}

func (s *UserSubscription) NeedsWeeklyReset() bool {
	if s.WeeklyWindowStart == nil {
		return false
	}
	return time.Since(*s.WeeklyWindowStart) >= 7*24*time.Hour
}

func (s *UserSubscription) NeedsMonthlyReset() bool {
	if s.MonthlyWindowStart == nil {
		return false
	}
	return time.Since(*s.MonthlyWindowStart) >= SubscriptionMonthlyWindowDuration
}

func (s *UserSubscription) DailyResetTime() *time.Time {
	if s.DailyWindowStart == nil {
		return nil
	}
	t := subscriptionWindowResetTime(*s.DailyWindowStart, 24*time.Hour, s.ExpiresAt)
	return &t
}

func (s *UserSubscription) WeeklyResetTime() *time.Time {
	if s.WeeklyWindowStart == nil {
		return nil
	}
	t := subscriptionWindowResetTime(*s.WeeklyWindowStart, 7*24*time.Hour, s.ExpiresAt)
	return &t
}

func (s *UserSubscription) MonthlyResetTime() *time.Time {
	if s.MonthlyWindowStart == nil {
		return nil
	}
	t := subscriptionWindowResetTime(*s.MonthlyWindowStart, SubscriptionMonthlyWindowDuration, s.ExpiresAt)
	return &t
}

// subscriptionWindowResetTime caps a quota-window reset at subscription expiry.
//
// A subscription cannot receive a fresh quota after it has already expired.
// This keeps customer-facing "resets in" and "days remaining" timelines aligned
// for one-cycle monthly cards, including legacy 30-day rows after the product
// default moved to a 31-day cycle.
func subscriptionWindowResetTime(windowStart time.Time, duration time.Duration, expiresAt time.Time) time.Time {
	resetAt := windowStart.Add(duration)
	if !expiresAt.IsZero() && resetAt.After(expiresAt) {
		return expiresAt
	}
	return resetAt
}

func (s *UserSubscription) CheckDailyLimit(group *Group, additionalCost float64) bool {
	if !group.HasDailyLimit() {
		return true
	}
	return subscriptionUsageWithinLimit(s.DailyUsageUSD, *group.DailyLimitUSD, additionalCost)
}

func (s *UserSubscription) CheckWeeklyLimit(group *Group, additionalCost float64) bool {
	if !group.HasWeeklyLimit() {
		return true
	}
	return subscriptionUsageWithinLimit(s.WeeklyUsageUSD, *group.WeeklyLimitUSD, additionalCost)
}

func (s *UserSubscription) CheckMonthlyLimit(group *Group, additionalCost float64) bool {
	if !group.HasMonthlyLimit() {
		return true
	}
	return subscriptionUsageWithinLimit(s.MonthlyUsageUSD, *group.MonthlyLimitUSD, additionalCost)
}

func (s *UserSubscription) CheckAllLimits(group *Group, additionalCost float64) (daily, weekly, monthly bool) {
	daily = s.CheckDailyLimit(group, additionalCost)
	weekly = s.CheckWeeklyLimit(group, additionalCost)
	monthly = s.CheckMonthlyLimit(group, additionalCost)
	return
}

func subscriptionUsageWithinLimit(current, limit, additionalCost float64) bool {
	if limit <= 0 {
		return true
	}
	remaining := limit - current - additionalCost
	if remaining < 0 {
		return false
	}
	return remaining > SubscriptionUsageLimitEpsilonUSD
}

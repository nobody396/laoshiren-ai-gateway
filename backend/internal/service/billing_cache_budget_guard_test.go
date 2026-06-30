package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type budgetGuardCacheStub struct {
	balance float64
	subData *SubscriptionCacheData
}

func (s *budgetGuardCacheStub) GetUserBalance(context.Context, int64) (float64, error) {
	return s.balance, nil
}

func (s *budgetGuardCacheStub) SetUserBalance(context.Context, int64, float64) error { return nil }
func (s *budgetGuardCacheStub) DeductUserBalance(context.Context, int64, float64) error {
	return nil
}
func (s *budgetGuardCacheStub) InvalidateUserBalance(context.Context, int64) error { return nil }

func (s *budgetGuardCacheStub) GetSubscriptionCache(context.Context, int64, int64) (*SubscriptionCacheData, error) {
	if s.subData == nil {
		return nil, errors.New("missing subscription")
	}
	cp := *s.subData
	return &cp, nil
}

func (s *budgetGuardCacheStub) SetSubscriptionCache(context.Context, int64, int64, *SubscriptionCacheData) error {
	return nil
}
func (s *budgetGuardCacheStub) UpdateSubscriptionUsage(context.Context, int64, int64, float64) error {
	return nil
}
func (s *budgetGuardCacheStub) InvalidateSubscriptionCache(context.Context, int64, int64) error {
	return nil
}

func (s *budgetGuardCacheStub) GetAPIKeyRateLimit(context.Context, int64) (*APIKeyRateLimitCacheData, error) {
	return nil, errors.New("not implemented")
}
func (s *budgetGuardCacheStub) SetAPIKeyRateLimit(context.Context, int64, *APIKeyRateLimitCacheData) error {
	return nil
}
func (s *budgetGuardCacheStub) UpdateAPIKeyRateLimitUsage(context.Context, int64, float64) error {
	return nil
}
func (s *budgetGuardCacheStub) InvalidateAPIKeyRateLimit(context.Context, int64) error {
	return nil
}

func TestBillingCacheServiceResolveBudgetGuardConcurrencyBalance(t *testing.T) {
	ctx := context.Background()
	cache := &budgetGuardCacheStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	user := &User{ID: 42}

	tests := []struct {
		name    string
		balance float64
		base    int
		want    int
		wantErr error
	}{
		{name: "normal_balance_keeps_base", balance: 5, base: 100, want: 100},
		{name: "low_balance_caps_to_three", balance: 4.99, base: 100, want: 3},
		{name: "critical_balance_caps_to_one", balance: 0.99, base: 100, want: 1},
		{name: "existing_lower_concurrency_stays_lower", balance: 4.99, base: 2, want: 2},
		{name: "unlimited_base_is_capped_when_low", balance: 4.99, base: 0, want: 3},
		{name: "zero_balance_rejects", balance: 0, base: 100, want: 100, wantErr: ErrInsufficientBalance},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.balance = tt.balance
			got, err := svc.ResolveBudgetGuardConcurrency(ctx, user, nil, nil, tt.base)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBillingCacheServiceResolveBudgetGuardConcurrencySubscriptionUsesOriginalCreditRatio(t *testing.T) {
	ctx := context.Background()
	cache := &budgetGuardCacheStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	limit := 450.0 // 450 raw = 4500 display credits
	group := &Group{
		ID:               7,
		SubscriptionType: SubscriptionTypeSubscription,
		MonthlyLimitUSD:  &limit,
	}
	user := &User{ID: 42}
	sub := &UserSubscription{ID: 7}

	tests := []struct {
		name    string
		used    float64
		want    int
		wantErr error
	}{
		{name: "above_one_percent_keeps_base", used: 445.0, want: 100},       // remaining 50 display credits
		{name: "within_one_percent_caps_to_three", used: 445.6, want: 3},     // remaining 44 display credits
		{name: "within_point_two_percent_caps_to_one", used: 449.2, want: 1}, // remaining 8 display credits
		{name: "exhausted_rejects", used: 450.0, want: 100, wantErr: ErrMonthlyLimitExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.subData = &SubscriptionCacheData{
				Status:       SubscriptionStatusActive,
				ExpiresAt:    time.Now().Add(time.Hour),
				MonthlyUsage: tt.used,
			}
			got, err := svc.ResolveBudgetGuardConcurrency(ctx, user, group, sub, 100)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBillingCacheServiceResolveBudgetGuardConcurrencySubscriptionScalesWithPlanSize(t *testing.T) {
	ctx := context.Background()
	cache := &budgetGuardCacheStub{}
	svc := NewBillingCacheService(cache, nil, nil, nil, &config.Config{})
	t.Cleanup(svc.Stop)

	limit := 3000.0 // 3000 raw = 30000 display credits
	group := &Group{
		ID:               18,
		SubscriptionType: SubscriptionTypeSubscription,
		MonthlyLimitUSD:  &limit,
	}
	user := &User{ID: 42}
	sub := &UserSubscription{ID: 18}

	cache.subData = &SubscriptionCacheData{
		Status:       SubscriptionStatusActive,
		ExpiresAt:    time.Now().Add(time.Hour),
		MonthlyUsage: 2975.0, // remaining 250 display credits, below 1% of 30000
	}
	got, err := svc.ResolveBudgetGuardConcurrency(ctx, user, group, sub, 100)
	require.NoError(t, err)
	require.Equal(t, 3, got)

	cache.subData.MonthlyUsage = 2995.0 // remaining 50 display credits, below 0.2% of 30000
	got, err = svc.ResolveBudgetGuardConcurrency(ctx, user, group, sub, 100)
	require.NoError(t, err)
	require.Equal(t, 1, got)
}

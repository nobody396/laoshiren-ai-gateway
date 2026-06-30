package handler

import (
	"context"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

func resolveBudgetGuardConcurrency(
	ctx context.Context,
	billingCacheService *service.BillingCacheService,
	apiKey *service.APIKey,
	subscription *service.UserSubscription,
	baseConcurrency int,
) (int, error) {
	if billingCacheService == nil || apiKey == nil {
		return baseConcurrency, nil
	}
	return billingCacheService.ResolveBudgetGuardConcurrency(ctx, apiKey.User, apiKey.Group, subscription, baseConcurrency)
}

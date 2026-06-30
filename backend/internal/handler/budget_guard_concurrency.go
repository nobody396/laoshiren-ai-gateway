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
) (service.BudgetGuardConcurrencyDecision, error) {
	baseDecision := service.BudgetGuardConcurrencyDecision{EffectiveConcurrency: baseConcurrency}
	if billingCacheService == nil || apiKey == nil {
		return baseDecision, nil
	}
	return billingCacheService.ResolveBudgetGuardConcurrencyDecision(ctx, apiKey.User, apiKey.Group, subscription, baseConcurrency)
}

func budgetGuardRateLimitMessage(decision service.BudgetGuardConcurrencyDecision) (string, bool) {
	if !decision.Limited {
		return "", false
	}
	switch decision.Reason {
	case service.BudgetGuardReasonBalanceCritical:
		return "账户余额不足 1 元。为避免超额消耗，系统已临时限制为 1 并发，请充值后重试。", true
	case service.BudgetGuardReasonBalanceLow:
		return "账户余额不足 5 元。为避免超额消耗，系统已临时限制为 3 并发，请充值后重试。", true
	case service.BudgetGuardReasonSubscriptionCritical:
		return "月卡剩余额度不足 0.2%。为避免超额使用，系统已临时限制为 1 并发，请续费、升级或等待额度恢复后重试。", true
	case service.BudgetGuardReasonSubscriptionLow:
		return "月卡剩余额度不足 1%。为避免超额使用，系统已临时限制为 3 并发，请续费、升级或等待额度恢复后重试。", true
	default:
		return "", false
	}
}

func budgetGuardOrDefaultRateLimitMessage(decision service.BudgetGuardConcurrencyDecision, fallback string) string {
	if message, ok := budgetGuardRateLimitMessage(decision); ok {
		return message
	}
	return fallback
}

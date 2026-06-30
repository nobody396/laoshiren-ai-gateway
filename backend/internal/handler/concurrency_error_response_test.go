package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestConcurrencyErrorResponse(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		slotType    string
		wantStatus  int
		wantType    string
		wantMessage string
	}{
		{
			name:        "true concurrency timeout remains rate limit",
			err:         &ConcurrencyError{SlotType: "account", IsTimeout: true},
			slotType:    "user",
			wantStatus:  http.StatusTooManyRequests,
			wantType:    "rate_limit_error",
			wantMessage: "Concurrency limit exceeded for account, please retry later",
		},
		{
			name:        "client cancellation is not classified as concurrency limit",
			err:         context.Canceled,
			slotType:    "user",
			wantStatus:  statusClientClosedRequest,
			wantType:    "api_error",
			wantMessage: "context canceled",
		},
		{
			name:        "deadline exceeded is service unavailable",
			err:         context.DeadlineExceeded,
			slotType:    "user",
			wantStatus:  http.StatusServiceUnavailable,
			wantType:    "api_error",
			wantMessage: "Service temporarily unavailable, please retry later",
		},
		{
			name:        "redis acquire error is service unavailable",
			err:         errors.New("redis unavailable"),
			slotType:    "user",
			wantStatus:  http.StatusServiceUnavailable,
			wantType:    "api_error",
			wantMessage: "Service temporarily unavailable, please retry later",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, errType, message := concurrencyErrorResponse(tt.err, tt.slotType)
			require.Equal(t, tt.wantStatus, status)
			require.Equal(t, tt.wantType, errType)
			require.Equal(t, tt.wantMessage, message)
		})
	}
}

func TestBudgetGuardRateLimitMessage(t *testing.T) {
	tests := []struct {
		name    string
		reason  service.BudgetGuardReason
		want    string
		wantHit bool
	}{
		{
			name:    "balance low",
			reason:  service.BudgetGuardReasonBalanceLow,
			want:    "账户余额不足 5 元。为避免超额消耗，系统已临时限制为 3 并发，请充值后重试。",
			wantHit: true,
		},
		{
			name:    "balance critical",
			reason:  service.BudgetGuardReasonBalanceCritical,
			want:    "账户余额不足 1 元。为避免超额消耗，系统已临时限制为 1 并发，请充值后重试。",
			wantHit: true,
		},
		{
			name:    "subscription low",
			reason:  service.BudgetGuardReasonSubscriptionLow,
			want:    "月卡剩余额度不足 1%。为避免超额使用，系统已临时限制为 3 并发，请续费、升级或等待额度恢复后重试。",
			wantHit: true,
		},
		{
			name:    "subscription critical",
			reason:  service.BudgetGuardReasonSubscriptionCritical,
			want:    "月卡剩余额度不足 0.2%。为避免超额使用，系统已临时限制为 1 并发，请续费、升级或等待额度恢复后重试。",
			wantHit: true,
		},
		{
			name:    "not budget limited",
			reason:  service.BudgetGuardReasonBalanceLow,
			want:    "",
			wantHit: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := service.BudgetGuardConcurrencyDecision{
				EffectiveConcurrency: 1,
				Limited:              tt.wantHit,
				Reason:               tt.reason,
			}
			got, ok := budgetGuardRateLimitMessage(decision)
			require.Equal(t, tt.wantHit, ok)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestBudgetGuardAwareConcurrencyErrorResponseOverridesOnlyUserConcurrency429(t *testing.T) {
	decision := service.BudgetGuardConcurrencyDecision{
		EffectiveConcurrency: 1,
		Limited:              true,
		Reason:               service.BudgetGuardReasonBalanceCritical,
	}

	status, errType, message := budgetGuardAwareConcurrencyErrorResponse(&ConcurrencyError{SlotType: "user", IsTimeout: true}, "user", decision)
	require.Equal(t, http.StatusTooManyRequests, status)
	require.Equal(t, "rate_limit_error", errType)
	require.Equal(t, "账户余额不足 1 元。为避免超额消耗，系统已临时限制为 1 并发，请充值后重试。", message)

	status, errType, message = budgetGuardAwareConcurrencyErrorResponse(&ConcurrencyError{SlotType: "account", IsTimeout: true}, "account", decision)
	require.Equal(t, http.StatusTooManyRequests, status)
	require.Equal(t, "rate_limit_error", errType)
	require.Equal(t, "Concurrency limit exceeded for account, please retry later", message)
}

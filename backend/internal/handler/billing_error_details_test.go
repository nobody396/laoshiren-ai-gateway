package handler

import (
	"net/http"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBillingErrorDetailsSubscriptionLimits(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantText   string
	}{
		{name: "daily remains retryable", err: service.ErrDailyLimitExceeded, wantStatus: http.StatusTooManyRequests, wantText: "daily usage limit exceeded"},
		{name: "weekly remains retryable", err: service.ErrWeeklyLimitExceeded, wantStatus: http.StatusTooManyRequests, wantText: "weekly usage limit exceeded"},
		{name: "monthly is account action required", err: service.ErrMonthlyLimitExceeded, wantStatus: http.StatusForbidden, wantText: "Retrying will not help"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, message := billingErrorDetails(tt.err)
			require.Equal(t, tt.wantStatus, status)
			require.Equal(t, "subscription_error", code)
			require.Contains(t, message, tt.wantText)
		})
	}
}

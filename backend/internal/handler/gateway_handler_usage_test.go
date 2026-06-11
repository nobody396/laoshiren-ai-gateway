package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUsageUnrestrictedCreditSubscriptionIncludesTypeForDisplay(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/usage", nil)

	dailyLimit := 15.0
	weeklyLimit := 105.0
	monthlyLimit := 450.0
	apiKey := &service.APIKey{
		Group: &service.Group{
			Name:             "GPT Lite 月卡组",
			SubscriptionType: service.SubscriptionTypeCredit,
			DailyLimitUSD:    &dailyLimit,
			WeeklyLimitUSD:   &weeklyLimit,
			MonthlyLimitUSD:  &monthlyLimit,
		},
	}
	c.Set(string(middleware.ContextKeySubscription), &service.UserSubscription{
		DailyUsageUSD:   1.81,
		WeeklyUsageUSD:  1.81,
		MonthlyUsageUSD: 1.81,
	})

	h := &GatewayHandler{}
	h.usageUnrestricted(c, context.Background(), apiKey, middleware.AuthSubject{UserID: 1}, nil, nil)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, service.SubscriptionTypeCredit, body["subscription_type"])
	require.Equal(t, "USD", body["unit"])

	subscription, ok := body["subscription"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, service.SubscriptionTypeCredit, subscription["subscription_type"])
	require.Equal(t, "USD", subscription["unit"])
	require.Equal(t, weeklyLimit, subscription["weekly_limit_usd"])
}

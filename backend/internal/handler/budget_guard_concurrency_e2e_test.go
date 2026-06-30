package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBudgetGuardLowBalanceUserSlotResponseIncludesMessageAndRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cache := &concurrencyCacheMock{
		acquireUserSlotFn: func(ctx context.Context, userID int64, maxConcurrency int, requestID string) (bool, error) {
			require.Equal(t, int64(42), userID)
			require.Equal(t, 1, maxConcurrency)
			return false, nil
		},
		incrementWaitFn: func(ctx context.Context, userID int64, maxWait int) (bool, error) {
			require.Equal(t, int64(42), userID)
			require.Equal(t, 21, maxWait)
			return false, nil
		},
	}
	h := &OpenAIGatewayHandler{
		concurrencyHelper: NewConcurrencyHelper(service.NewConcurrencyService(cache), SSEPingFormatNone, time.Second),
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxkey.RequestID, "req-budget-guard-1"))
	c.Request = req

	streamStarted := false
	release, acquired := h.acquireResponsesUserSlot(
		c,
		42,
		service.BudgetGuardConcurrencyDecision{
			EffectiveConcurrency: 1,
			Limited:              true,
			Reason:               service.BudgetGuardReasonBalanceCritical,
		},
		false,
		&streamStarted,
		zap.NewNop(),
	)
	require.False(t, acquired)
	require.Nil(t, release)
	require.Equal(t, http.StatusTooManyRequests, rec.Code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	errObj, ok := payload["error"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "rate_limit_error", errObj["type"])
	require.Equal(t, "账户余额不足 1 元。为避免超额消耗，系统已临时限制为 1 并发，请充值后重试。", errObj["message"])
	require.Equal(t, "req-budget-guard-1", errObj["request_id"])
}

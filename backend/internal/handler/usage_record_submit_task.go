package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"go.uber.org/zap"
)

const usageRecordSubmitFallbackTimeout = 10 * time.Second

// withUsageRecordRequestCorrelation copies only the stable request identifiers
// needed by asynchronous usage recording. The worker pool deliberately creates
// a fresh bounded context, so passing the original request context would either
// lose these identifiers or retain a canceled request and unrelated values.
func withUsageRecordRequestCorrelation(requestCtx context.Context, task service.UsageRecordTask) service.UsageRecordTask {
	if task == nil {
		return nil
	}
	requestID, clientRequestID := usageRecordRequestCorrelation(requestCtx)
	return func(workerCtx context.Context) {
		if workerCtx == nil {
			workerCtx = context.Background()
		}
		if requestID != "" {
			workerCtx = context.WithValue(workerCtx, ctxkey.RequestID, requestID)
		}
		if clientRequestID != "" {
			workerCtx = context.WithValue(workerCtx, ctxkey.ClientRequestID, clientRequestID)
		}
		task(workerCtx)
	}
}

func usageRecordRequestCorrelation(ctx context.Context) (requestID, clientRequestID string) {
	if ctx == nil {
		return "", ""
	}
	requestID, _ = ctx.Value(ctxkey.RequestID).(string)
	clientRequestID, _ = ctx.Value(ctxkey.ClientRequestID).(string)
	return strings.TrimSpace(requestID), strings.TrimSpace(clientRequestID)
}

// withUsageRecordWSTurnCorrelation gives every billable turn on one inbound
// WebSocket connection its own stable idempotency key. Reusing the connection
// request IDs for every turn would make later turns collide with turn 1 in
// usage_billing_dedup.
func withUsageRecordWSTurnCorrelation(requestCtx context.Context, turn int) context.Context {
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	if turn < 1 {
		turn = 1
	}
	suffix := fmt.Sprintf(":ws-turn:%d", turn)
	requestID, clientRequestID := usageRecordRequestCorrelation(requestCtx)
	if requestID != "" {
		requestCtx = context.WithValue(requestCtx, ctxkey.RequestID, requestID+suffix)
	}
	if clientRequestID != "" {
		requestCtx = context.WithValue(requestCtx, ctxkey.ClientRequestID, clientRequestID+suffix)
	}
	return requestCtx
}

func submitUsageRecordTaskFailClosed(
	pool *service.UsageRecordWorkerPool,
	task service.UsageRecordTask,
	component string,
	panicEvent string,
) {
	if task == nil {
		return
	}
	if pool != nil {
		mode := pool.Submit(task)
		if mode != service.UsageRecordSubmitModeDropped {
			return
		}
		logger.L().With(
			zap.String("component", component),
			zap.String("submit_mode", string(mode)),
		).Warn("usage_record.submit_dropped_sync_fallback")
	}

	executeUsageRecordTaskSync(task, component, panicEvent)
}

func executeUsageRecordTaskSync(task service.UsageRecordTask, component string, panicEvent string) {
	ctx, cancel := context.WithTimeout(context.Background(), usageRecordSubmitFallbackTimeout)
	defer cancel()
	defer func() {
		if recovered := recover(); recovered != nil {
			logger.L().With(
				zap.String("component", component),
				zap.Any("panic", recovered),
			).Error(panicEvent)
		}
	}()
	task(ctx)
}

package handler

import (
	"context"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"go.uber.org/zap"
)

const usageRecordSubmitFallbackTimeout = 10 * time.Second

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

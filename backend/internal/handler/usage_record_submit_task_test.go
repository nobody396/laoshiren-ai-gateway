package handler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/ctxkey"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func newUsageRecordTestPool(t *testing.T) *service.UsageRecordWorkerPool {
	t.Helper()
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:           1,
		QueueSize:             8,
		TaskTimeout:           time.Second,
		OverflowPolicy:        "drop",
		OverflowSamplePercent: 0,
		AutoScaleEnabled:      false,
	})
	t.Cleanup(pool.Stop)
	return pool
}

func newSaturatedUsageRecordTestPool(t *testing.T, overflowPolicy string, overflowSamplePercent int) *service.UsageRecordWorkerPool {
	t.Helper()
	pool := service.NewUsageRecordWorkerPoolWithOptions(service.UsageRecordWorkerPoolOptions{
		WorkerCount:           1,
		QueueSize:             1,
		TaskTimeout:           time.Second,
		OverflowPolicy:        overflowPolicy,
		OverflowSamplePercent: overflowSamplePercent,
		AutoScaleEnabled:      false,
	})

	block := make(chan struct{})
	started := make(chan struct{})
	queuedDone := make(chan struct{})

	require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(ctx context.Context) {
		close(started)
		<-block
	}))
	<-started

	require.Equal(t, service.UsageRecordSubmitModeEnqueued, pool.Submit(func(ctx context.Context) {
		close(queuedDone)
	}))

	t.Cleanup(func() {
		close(block)
		select {
		case <-queuedDone:
		case <-time.After(time.Second):
			t.Fatal("queued task not executed")
		}
		pool.Stop()
	})

	return pool
}

func TestGatewayHandlerSubmitUsageRecordTask_WithPool(t *testing.T) {
	pool := newUsageRecordTestPool(t)
	h := &GatewayHandler{usageRecordWorkerPool: pool}

	done := make(chan struct{})
	h.submitUsageRecordTask(func(ctx context.Context) {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task not executed")
	}
}

func TestGatewayHandlerSubmitUsageRecordTask_WithoutPoolSyncFallback(t *testing.T) {
	h := &GatewayHandler{}
	var called atomic.Bool

	h.submitUsageRecordTask(func(ctx context.Context) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected deadline in fallback context")
		}
		called.Store(true)
	})

	require.True(t, called.Load())
}

func TestGatewayHandlerSubmitUsageRecordTask_NilTask(t *testing.T) {
	h := &GatewayHandler{}
	require.NotPanics(t, func() {
		h.submitUsageRecordTask(nil)
	})
}

func TestGatewayHandlerSubmitUsageRecordTask_WithoutPool_TaskPanicRecovered(t *testing.T) {
	h := &GatewayHandler{}
	var called atomic.Bool

	require.NotPanics(t, func() {
		h.submitUsageRecordTask(func(ctx context.Context) {
			panic("usage task panic")
		})
	})

	h.submitUsageRecordTask(func(ctx context.Context) {
		called.Store(true)
	})
	require.True(t, called.Load(), "panic 后后续任务应仍可执行")
}

func TestGatewayHandlerSubmitUsageRecordTask_PoolDropSyncFallback(t *testing.T) {
	tests := []struct {
		name                  string
		overflowPolicy        string
		overflowSamplePercent int
		primeSampleDrop       bool
	}{
		{
			name:           "drop",
			overflowPolicy: config.UsageRecordOverflowPolicyDrop,
		},
		{
			name:                  "sample_drop",
			overflowPolicy:        config.UsageRecordOverflowPolicySample,
			overflowSamplePercent: 1,
			primeSampleDrop:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := newSaturatedUsageRecordTestPool(t, tt.overflowPolicy, tt.overflowSamplePercent)
			if tt.primeSampleDrop {
				require.Equal(t, service.UsageRecordSubmitModeSync, pool.Submit(func(ctx context.Context) {}))
			}

			h := &GatewayHandler{usageRecordWorkerPool: pool}
			var called atomic.Bool

			h.submitUsageRecordTask(func(ctx context.Context) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("expected deadline in fallback context")
				}
				called.Store(true)
			})

			require.True(t, called.Load(), "dropped usage record task must execute synchronously")
		})
	}
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_WithPool(t *testing.T) {
	pool := newUsageRecordTestPool(t)
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}

	done := make(chan struct{})
	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task not executed")
	}
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_WithoutPoolSyncFallback(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	var called atomic.Bool

	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("expected deadline in fallback context")
		}
		called.Store(true)
	})

	require.True(t, called.Load())
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_NilTask(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	require.NotPanics(t, func() {
		h.submitUsageRecordTask(context.Background(), nil)
	})
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_WithoutPool_TaskPanicRecovered(t *testing.T) {
	h := &OpenAIGatewayHandler{}
	var called atomic.Bool

	require.NotPanics(t, func() {
		h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
			panic("usage task panic")
		})
	})

	h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
		called.Store(true)
	})
	require.True(t, called.Load(), "panic 后后续任务应仍可执行")
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_PoolDropSyncFallback(t *testing.T) {
	tests := []struct {
		name                  string
		overflowPolicy        string
		overflowSamplePercent int
		primeSampleDrop       bool
	}{
		{
			name:           "drop",
			overflowPolicy: config.UsageRecordOverflowPolicyDrop,
		},
		{
			name:                  "sample_drop",
			overflowPolicy:        config.UsageRecordOverflowPolicySample,
			overflowSamplePercent: 1,
			primeSampleDrop:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := newSaturatedUsageRecordTestPool(t, tt.overflowPolicy, tt.overflowSamplePercent)
			if tt.primeSampleDrop {
				require.Equal(t, service.UsageRecordSubmitModeSync, pool.Submit(func(ctx context.Context) {}))
			}

			h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
			var called atomic.Bool

			h.submitUsageRecordTask(context.Background(), func(ctx context.Context) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("expected deadline in fallback context")
				}
				called.Store(true)
			})

			require.True(t, called.Load(), "dropped usage record task must execute synchronously")
		})
	}
}

func TestOpenAIGatewayHandlerSubmitUsageRecordTask_PreservesRequestCorrelation(t *testing.T) {
	pool := newUsageRecordTestPool(t)
	h := &OpenAIGatewayHandler{usageRecordWorkerPool: pool}
	requestCtx := context.WithValue(context.Background(), ctxkey.RequestID, " request-stable ")
	requestCtx = context.WithValue(requestCtx, ctxkey.ClientRequestID, " client-stable ")

	result := make(chan [2]string, 1)
	h.submitUsageRecordTask(requestCtx, func(ctx context.Context) {
		requestID, _ := ctx.Value(ctxkey.RequestID).(string)
		clientRequestID, _ := ctx.Value(ctxkey.ClientRequestID).(string)
		result <- [2]string{requestID, clientRequestID}
	})

	select {
	case got := <-result:
		require.Equal(t, [2]string{"request-stable", "client-stable"}, got)
	case <-time.After(time.Second):
		t.Fatal("task not executed")
	}
}

func TestWithUsageRecordWSTurnCorrelation_IsolatesTurns(t *testing.T) {
	base := context.WithValue(context.Background(), ctxkey.RequestID, "request-stable")
	base = context.WithValue(base, ctxkey.ClientRequestID, "client-stable")

	turn1RequestID, turn1ClientRequestID := usageRecordRequestCorrelation(withUsageRecordWSTurnCorrelation(base, 1))
	turn2RequestID, turn2ClientRequestID := usageRecordRequestCorrelation(withUsageRecordWSTurnCorrelation(base, 2))
	baseRequestID, baseClientRequestID := usageRecordRequestCorrelation(base)

	require.Equal(t, "request-stable:ws-turn:1", turn1RequestID)
	require.Equal(t, "client-stable:ws-turn:1", turn1ClientRequestID)
	require.Equal(t, "request-stable:ws-turn:2", turn2RequestID)
	require.Equal(t, "client-stable:ws-turn:2", turn2ClientRequestID)
	require.Equal(t, "request-stable", baseRequestID)
	require.Equal(t, "client-stable", baseClientRequestID)
}

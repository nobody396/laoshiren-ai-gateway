//go:build unit

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLifecycle_DeterministicStartAndReverseStop(t *testing.T) {
	var mu sync.Mutex
	order := []string{}
	component := func(name string) LifecycleComponent {
		return LifecycleFunc{ComponentName: name,
			StartFunc: func(context.Context) error { mu.Lock(); order = append(order, "start:"+name); mu.Unlock(); return nil },
			StopFunc:  func(context.Context) error { mu.Lock(); order = append(order, "stop:"+name); mu.Unlock(); return nil },
		}
	}
	lifecycle := NewLifecycle(component("a"), component("b"), component("c"))
	require.NoError(t, lifecycle.Start(context.Background()))
	require.NoError(t, lifecycle.Start(context.Background()))
	require.NoError(t, lifecycle.Stop(context.Background()))
	require.NoError(t, lifecycle.Stop(context.Background()))
	require.Equal(t, []string{"start:a", "start:b", "start:c", "stop:c", "stop:b", "stop:a"}, order)
}

func TestLifecycle_StartFailureCleansPredecessors(t *testing.T) {
	order := []string{}
	lifecycle := NewLifecycle(
		LifecycleFunc{ComponentName: "a", StartFunc: func(context.Context) error { order = append(order, "start:a"); return nil }, StopFunc: func(context.Context) error { order = append(order, "stop:a"); return nil }},
		LifecycleFunc{ComponentName: "b", StartFunc: func(context.Context) error { return errors.New("boom") }},
	)
	require.Error(t, lifecycle.Start(context.Background()))
	require.Equal(t, []string{"start:a", "stop:a"}, order)
}

func TestLifecycle_StuckStopHonorsRootDeadline(t *testing.T) {
	lifecycle := NewLifecycle(LifecycleFunc{
		ComponentName: "stuck",
		StartFunc:     func(context.Context) error { return nil },
		StopFunc:      func(context.Context) error { select {} },
	})
	require.NoError(t, lifecycle.Start(context.Background()))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	started := time.Now()
	require.ErrorIs(t, lifecycle.Stop(ctx), context.DeadlineExceeded)
	require.Less(t, time.Since(started), 250*time.Millisecond)
}

func TestLifecycle_ConcurrentStartStopRaceSafe(t *testing.T) {
	lifecycle := NewLifecycle(LifecycleFunc{ComponentName: "noop"})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _ = lifecycle.Start(context.Background()) }()
	}
	wg.Wait()
	require.NoError(t, lifecycle.Stop(context.Background()))
}

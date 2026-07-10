package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type LifecycleComponent interface {
	Name() string
	Start(context.Context) error
	Stop(context.Context) error
}

type LifecycleFunc struct {
	ComponentName string
	StartFunc     func(context.Context) error
	StopFunc      func(context.Context) error
}

func (c LifecycleFunc) Name() string { return c.ComponentName }
func (c LifecycleFunc) Start(ctx context.Context) error {
	if c.StartFunc == nil {
		return nil
	}
	return c.StartFunc(ctx)
}
func (c LifecycleFunc) Stop(ctx context.Context) error {
	if c.StopFunc == nil {
		return nil
	}
	return c.StopFunc(ctx)
}

type Lifecycle struct {
	mu         sync.Mutex
	components []LifecycleComponent
	started    []LifecycleComponent
	running    bool
	stopped    bool
}

func NewLifecycle(components ...LifecycleComponent) *Lifecycle {
	copyOf := append([]LifecycleComponent(nil), components...)
	return &Lifecycle{components: copyOf}
}

func (l *Lifecycle) Start(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running {
		return nil
	}
	if l.stopped {
		return errors.New("lifecycle cannot restart after stop")
	}

	for _, component := range l.components {
		if component == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			_ = stopLifecycleComponents(ctx, l.started)
			return err
		}
		if err := component.Start(ctx); err != nil {
			cleanupErr := stopLifecycleComponents(ctx, l.started)
			l.started = nil
			return errors.Join(fmt.Errorf("start %s: %w", component.Name(), err), cleanupErr)
		}
		l.started = append(l.started, component)
	}
	l.running = true
	return nil
}

func (l *Lifecycle) Stop(ctx context.Context) error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped {
		return nil
	}
	l.stopped = true
	l.running = false
	err := stopLifecycleComponents(ctx, l.started)
	l.started = nil
	return err
}

func stopLifecycleComponents(ctx context.Context, started []LifecycleComponent) error {
	var errs []error
	for i := len(started) - 1; i >= 0; i-- {
		component := started[i]
		if component == nil {
			continue
		}
		if err := runLifecycleStop(ctx, component); err != nil {
			errs = append(errs, fmt.Errorf("stop %s: %w", component.Name(), err))
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(errs...)
}

func runLifecycleStop(ctx context.Context, component LifecycleComponent) error {
	done := make(chan error, 1)
	go func() { done <- component.Stop(ctx) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

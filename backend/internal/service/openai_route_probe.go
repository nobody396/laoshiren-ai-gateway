package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// OpenAIRouteActiveProbesCodeAvailable deliberately remains false. A later
	// release must flip this guard only after the runner is wired to an owned,
	// non-billable test identity and the production activation is approved.
	OpenAIRouteActiveProbesCodeAvailable = false

	defaultOpenAIRouteProbeTimeout           = 10 * time.Second
	defaultOpenAIRouteProbeStateWriteTimeout = 2 * time.Second
)

var ErrOpenAIRouteInvalidProbe = errors.New("invalid OpenAI route active probe")

type OpenAIRouteProbeRequest struct {
	Key     OpenAIRouteKey
	OwnerID string
	Now     time.Time
	Policy  OpenAIRoutePolicy
}

type OpenAIRouteProbeOutcome struct {
	Success      bool
	FailureClass OpenAIRouteFailureClass
	RetryAfter   time.Duration
	RetryAt      time.Time
}

func (o OpenAIRouteProbeOutcome) Validate() error {
	if o.Success {
		if o.FailureClass != "" && o.FailureClass != OpenAIRouteFailureNone {
			return fmt.Errorf("%w: successful outcome has a failure class", ErrOpenAIRouteInvalidProbe)
		}
		return nil
	}
	if o.FailureClass == "" || o.FailureClass == OpenAIRouteFailureNone ||
		o.FailureClass == OpenAIRouteFailureUserRequest || o.FailureClass == OpenAIRouteFailureClientCancelled {
		return fmt.Errorf("%w: failed outcome needs an upstream-owned failure class", ErrOpenAIRouteInvalidProbe)
	}
	return nil
}

// OpenAIRouteActiveProbeClient is intentionally narrow. Implementations may
// call only an owned test identity and must return a classified outcome; they
// must never expose a credential, prompt, response body, or raw upstream URL.
type OpenAIRouteActiveProbeClient interface {
	Probe(ctx context.Context, key OpenAIRouteKey) (OpenAIRouteProbeOutcome, error)
}

type OpenAIRouteActiveProbeDecision struct {
	Executed              bool                    `json:"executed"`
	Reason                string                  `json:"reason"`
	LeaseOwner            string                  `json:"lease_owner,omitempty"`
	StartedAt             time.Time               `json:"started_at,omitempty"`
	FinishedAt            time.Time               `json:"finished_at,omitempty"`
	StateBefore           OpenAIRouteHealthState  `json:"state_before"`
	StateAfter            OpenAIRouteHealthState  `json:"state_after"`
	Success               bool                    `json:"success"`
	FailureClass          OpenAIRouteFailureClass `json:"failure_class,omitempty"`
	ActiveProbesAvailable bool                    `json:"active_probes_available"`
}

type OpenAIRouteActiveProbeRunner struct {
	healthStore OpenAIRouteHealthStore
	client      OpenAIRouteActiveProbeClient
	timeout     time.Duration
	renewEvery  time.Duration

	mu      sync.Mutex
	running map[string]struct{}

	attempted   atomic.Uint64
	executed    atomic.Uint64
	skipped     atomic.Uint64
	succeeded   atomic.Uint64
	failed      atomic.Uint64
	leaseLost   atomic.Uint64
	lastSuccess atomic.Int64
	lastFailure atomic.Int64
}

type OpenAIRouteActiveProbeStats struct {
	Attempted     uint64    `json:"attempted"`
	Executed      uint64    `json:"executed"`
	Skipped       uint64    `json:"skipped"`
	Succeeded     uint64    `json:"succeeded"`
	Failed        uint64    `json:"failed"`
	LeaseLost     uint64    `json:"lease_lost"`
	Running       int       `json:"running"`
	LastSuccessAt time.Time `json:"last_success_at,omitempty"`
	LastFailureAt time.Time `json:"last_failure_at,omitempty"`
	Enabled       bool      `json:"enabled"`
}

func NewOpenAIRouteActiveProbeRunner(healthStore OpenAIRouteHealthStore, client OpenAIRouteActiveProbeClient) *OpenAIRouteActiveProbeRunner {
	return &OpenAIRouteActiveProbeRunner{
		healthStore: healthStore,
		client:      client,
		timeout:     defaultOpenAIRouteProbeTimeout,
		running:     make(map[string]struct{}),
	}
}

// Run applies the compile-time guard. Even a fully configured caller cannot
// issue network traffic in this release.
func (r *OpenAIRouteActiveProbeRunner) Run(ctx context.Context, req OpenAIRouteProbeRequest) (OpenAIRouteActiveProbeDecision, error) {
	return r.run(ctx, req, OpenAIRouteActiveProbesCodeAvailable)
}

func (r *OpenAIRouteActiveProbeRunner) run(
	ctx context.Context,
	req OpenAIRouteProbeRequest,
	activeProbeCodeAvailable bool,
) (decision OpenAIRouteActiveProbeDecision, err error) {
	decision.Reason = "invalid_scope"
	decision.ActiveProbesAvailable = activeProbeCodeAvailable
	if r == nil || r.healthStore == nil || r.client == nil || !req.Key.Valid() || strings.TrimSpace(req.OwnerID) == "" {
		return decision, fmt.Errorf("%w: incomplete runner or request", ErrOpenAIRouteInvalidProbe)
	}
	r.attempted.Add(1)
	if ctx == nil {
		ctx = context.Background()
	}
	if !activeProbeCodeAvailable {
		r.skipped.Add(1)
		decision.Reason = "active_probe_release_guard_disabled"
		return decision, nil
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	policy, err := NormalizeOpenAIRoutePolicy(req.Policy)
	if err != nil {
		return decision, err
	}
	routeHealthKey := OpenAIRouteHealthStoreKeyForRoute(req.Key)
	state, err := r.healthStore.Get(ctx, routeHealthKey)
	if err != nil {
		return decision, err
	}
	decision.StateBefore = state
	if !state.ProbeDue(now) {
		r.skipped.Add(1)
		decision.Reason = "probe_not_due"
		return decision, nil
	}

	localKey := routeHealthKey.Fingerprint()
	r.mu.Lock()
	if _, exists := r.running[localKey]; exists {
		r.mu.Unlock()
		r.skipped.Add(1)
		decision.Reason = "probe_already_running_locally"
		return decision, nil
	}
	r.running[localKey] = struct{}{}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.running, localKey)
		r.mu.Unlock()
	}()

	owner := strings.TrimSpace(req.OwnerID)
	acquired, err := r.healthStore.AcquireHalfOpenPermit(ctx, routeHealthKey, owner)
	if err != nil {
		return decision, err
	}
	if !acquired {
		r.skipped.Add(1)
		decision.Reason = "probe_lease_held_elsewhere"
		return decision, nil
	}
	decision.LeaseOwner = owner
	defer func() {
		releaseCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = r.healthStore.ReleaseHalfOpenPermit(releaseCtx, routeHealthKey, owner)
	}()
	// Re-read under the distributed lease. Another health transition between
	// the optimistic due check and lease acquisition must cancel this probe.
	state, err = r.healthStore.Get(ctx, routeHealthKey)
	if err != nil {
		return decision, err
	}
	decision.StateBefore = state
	if !state.ProbeDue(now) {
		r.skipped.Add(1)
		decision.Reason = "probe_no_longer_due"
		return decision, nil
	}

	probeCtx, probeCancel := context.WithTimeout(ctx, r.timeout)
	defer probeCancel()
	decision.Executed = true
	decision.StartedAt = now
	r.executed.Add(1)

	leaseLost := make(chan error, 1)
	stopRenewal := make(chan struct{})
	renewalStopped := make(chan struct{})
	go r.renewHalfOpenPermit(probeCtx, probeCancel, routeHealthKey, owner, stopRenewal, renewalStopped, leaseLost)
	outcome, probeErr := r.client.Probe(probeCtx, req.Key)
	close(stopRenewal)
	<-renewalStopped
	decision.FinishedAt = time.Now().UTC()
	select {
	case renewalErr := <-leaseLost:
		if renewalErr != nil {
			r.leaseLost.Add(1)
			r.failed.Add(1)
			r.lastFailure.Store(decision.FinishedAt.UnixNano())
			return decision, renewalErr
		}
	default:
	}
	// Confirm ownership once more after stopping the renewal loop. This both
	// closes the fast-probe gap (where no renewal tick fired) and gives the
	// subsequent health write a fresh full lease interval.
	confirmCtx, confirmCancel := context.WithTimeout(context.Background(), defaultOpenAIRouteProbeStateWriteTimeout)
	owned, confirmErr := r.healthStore.RefreshHalfOpenPermit(confirmCtx, routeHealthKey, owner)
	confirmCancel()
	if confirmErr != nil || !owned {
		if confirmErr == nil {
			confirmErr = fmt.Errorf("%w: half-open lease ownership lost before health write", ErrOpenAIRouteInvalidProbe)
		}
		r.leaseLost.Add(1)
		r.failed.Add(1)
		r.lastFailure.Store(decision.FinishedAt.UnixNano())
		return decision, confirmErr
	}
	if probeErr != nil {
		r.failed.Add(1)
		r.lastFailure.Store(decision.FinishedAt.UnixNano())
		return decision, probeErr
	}
	if err := outcome.Validate(); err != nil {
		r.failed.Add(1)
		r.lastFailure.Store(decision.FinishedAt.UnixNano())
		return decision, err
	}
	decision.Success = outcome.Success
	decision.FailureClass = outcome.FailureClass
	event := OpenAIRouteHealthEvent{
		At:           decision.FinishedAt,
		Success:      outcome.Success,
		Probe:        true,
		FailureClass: outcome.FailureClass,
		RetryAfter:   outcome.RetryAfter,
		RetryAt:      outcome.RetryAt,
	}
	applyCtx, applyCancel := context.WithTimeout(context.Background(), defaultOpenAIRouteProbeStateWriteTimeout)
	defer applyCancel()
	updated, err := r.healthStore.ApplyEvent(applyCtx, routeHealthKey, event, policy)
	if err != nil {
		r.failed.Add(1)
		r.lastFailure.Store(decision.FinishedAt.UnixNano())
		return decision, err
	}
	decision.StateAfter = updated
	if outcome.Success {
		r.succeeded.Add(1)
		r.lastSuccess.Store(decision.FinishedAt.UnixNano())
		decision.Reason = "probe_succeeded"
	} else {
		r.failed.Add(1)
		r.lastFailure.Store(decision.FinishedAt.UnixNano())
		decision.Reason = "probe_failed"
	}
	return decision, nil
}

func (r *OpenAIRouteActiveProbeRunner) renewHalfOpenPermit(
	ctx context.Context,
	cancelProbe context.CancelFunc,
	key OpenAIRouteHealthStoreKey,
	owner string,
	stop <-chan struct{},
	done chan<- struct{},
	lost chan<- error,
) {
	defer close(done)
	interval := r.renewEvery
	if interval <= 0 {
		interval = r.timeout / 3
		if interval < time.Second {
			interval = time.Second
		}
	} else if interval < 10*time.Millisecond {
		interval = 10 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			renewCtx, cancel := context.WithTimeout(ctx, interval)
			owned, err := r.healthStore.RefreshHalfOpenPermit(renewCtx, key, owner)
			cancel()
			if err == nil && owned {
				continue
			}
			if err == nil {
				err = fmt.Errorf("%w: half-open lease ownership lost", ErrOpenAIRouteInvalidProbe)
			}
			if cancelProbe != nil {
				cancelProbe()
			}
			select {
			case lost <- err:
			default:
			}
			return
		}
	}
}

func (r *OpenAIRouteActiveProbeRunner) Stats() OpenAIRouteActiveProbeStats {
	if r == nil {
		return OpenAIRouteActiveProbeStats{}
	}
	stats := OpenAIRouteActiveProbeStats{
		Attempted: r.attempted.Load(), Executed: r.executed.Load(), Skipped: r.skipped.Load(),
		Succeeded: r.succeeded.Load(), Failed: r.failed.Load(), LeaseLost: r.leaseLost.Load(),
		Enabled: OpenAIRouteActiveProbesCodeAvailable,
	}
	r.mu.Lock()
	stats.Running = len(r.running)
	r.mu.Unlock()
	if value := r.lastSuccess.Load(); value > 0 {
		stats.LastSuccessAt = time.Unix(0, value).UTC()
	}
	if value := r.lastFailure.Load(); value > 0 {
		stats.LastFailureAt = time.Unix(0, value).UTC()
	}
	return stats
}

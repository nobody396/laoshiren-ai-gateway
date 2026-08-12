package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAIRouteProbeHealthStoreStub struct {
	mu sync.Mutex

	state          OpenAIRouteHealthState
	stateOnAcquire *OpenAIRouteHealthState
	permitOwner    string
	refreshAllowed bool
	acquireCalls   int
	refreshCalls   int
	releaseCalls   int
	applyEvents    []OpenAIRouteHealthEvent
}

func (s *openAIRouteProbeHealthStoreStub) Get(context.Context, OpenAIRouteHealthStoreKey) (OpenAIRouteHealthState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, nil
}

func (s *openAIRouteProbeHealthStoreStub) GetBatch(context.Context, []OpenAIRouteHealthStoreKey) (map[string]OpenAIRouteHealthState, error) {
	return nil, errors.New("unused")
}

func (s *openAIRouteProbeHealthStoreStub) ApplyEvent(_ context.Context, _ OpenAIRouteHealthStoreKey, event OpenAIRouteHealthEvent, policy OpenAIRoutePolicy) (OpenAIRouteHealthState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applyEvents = append(s.applyEvents, event)
	updated, err := ApplyOpenAIRouteHealthEvent(s.state, event, policy)
	if err == nil {
		s.state = updated
	}
	return updated, err
}

func (s *openAIRouteProbeHealthStoreStub) RecordProviderEvidence(context.Context, OpenAIRouteKey, OpenAIRouteHealthEvent, OpenAIRoutePolicy, int) (OpenAIRouteProviderEvidenceResult, error) {
	return OpenAIRouteProviderEvidenceResult{}, errors.New("unused")
}

func (s *openAIRouteProbeHealthStoreStub) AcquireHalfOpenPermit(_ context.Context, _ OpenAIRouteHealthStoreKey, owner string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acquireCalls++
	if s.permitOwner == "" || s.permitOwner == owner {
		s.permitOwner = owner
		if s.stateOnAcquire != nil {
			s.state = *s.stateOnAcquire
		}
		return true, nil
	}
	return false, nil
}

func (s *openAIRouteProbeHealthStoreStub) RefreshHalfOpenPermit(_ context.Context, _ OpenAIRouteHealthStoreKey, owner string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshCalls++
	return s.refreshAllowed && s.permitOwner == owner, nil
}

func (s *openAIRouteProbeHealthStoreStub) ReleaseHalfOpenPermit(_ context.Context, _ OpenAIRouteHealthStoreKey, owner string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.releaseCalls++
	if s.permitOwner == owner {
		s.permitOwner = ""
	}
	return nil
}

type openAIRouteProbeClientStub struct {
	mu      sync.Mutex
	calls   int
	outcome OpenAIRouteProbeOutcome
	err     error
	started chan struct{}
	release <-chan struct{}
}

func (s *openAIRouteProbeClientStub) Probe(ctx context.Context, _ OpenAIRouteKey) (OpenAIRouteProbeOutcome, error) {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	if s.release != nil {
		select {
		case <-ctx.Done():
			return OpenAIRouteProbeOutcome{}, ctx.Err()
		case <-s.release:
		}
	}
	return s.outcome, s.err
}

func testOpenAIRouteProbeRequest(now time.Time) OpenAIRouteProbeRequest {
	return OpenAIRouteProbeRequest{
		Key: OpenAIRouteKey{
			GroupID: 7, AccountID: 23, FailureDomain: "pomelo-hk", Model: "gpt-5.6-sol",
			RequestClass: OpenAIRouteRequestClassText, EndpointHash: "endpoint-hash", Transport: "http_sse",
		},
		OwnerID: "instance-1",
		Now:     now,
		Policy:  DefaultOpenAIRoutePolicy(),
	}
}

func dueOpenAIRouteProbeState(now time.Time) OpenAIRouteHealthState {
	return OpenAIRouteHealthState{
		State:       OpenAIRouteCircuitOpen,
		OpenUntil:   now.Add(-time.Second),
		LastEventAt: now.Add(-time.Minute),
	}
}

func TestOpenAIRouteActiveProbePublicGuardNeverCallsNetwork(t *testing.T) {
	now := time.Now().UTC()
	store := &openAIRouteProbeHealthStoreStub{state: dueOpenAIRouteProbeState(now), refreshAllowed: true}
	client := &openAIRouteProbeClientStub{outcome: OpenAIRouteProbeOutcome{Success: true}}
	runner := NewOpenAIRouteActiveProbeRunner(store, client)

	decision, err := runner.Run(context.Background(), testOpenAIRouteProbeRequest(now))

	require.NoError(t, err)
	require.False(t, decision.Executed)
	require.False(t, decision.ActiveProbesAvailable)
	require.Equal(t, "active_probe_release_guard_disabled", decision.Reason)
	require.Zero(t, client.calls)
	require.Zero(t, store.acquireCalls)
	require.Equal(t, uint64(1), runner.Stats().Skipped)
}

func TestOpenAIRouteActiveProbeSuccessUsesLeaseAndOnlyUpdatesHealth(t *testing.T) {
	now := time.Now().UTC()
	store := &openAIRouteProbeHealthStoreStub{state: dueOpenAIRouteProbeState(now), refreshAllowed: true}
	client := &openAIRouteProbeClientStub{outcome: OpenAIRouteProbeOutcome{Success: true}}
	runner := NewOpenAIRouteActiveProbeRunner(store, client)

	decision, err := runner.run(context.Background(), testOpenAIRouteProbeRequest(now), true)

	require.NoError(t, err)
	require.True(t, decision.Executed)
	require.True(t, decision.Success)
	require.Equal(t, "probe_succeeded", decision.Reason)
	require.Equal(t, OpenAIRouteCircuitHalfOpen, decision.StateAfter.State)
	require.Equal(t, 1, store.acquireCalls)
	require.Equal(t, 1, store.releaseCalls)
	require.Len(t, store.applyEvents, 1)
	require.True(t, store.applyEvents[0].Probe)
	stats := runner.Stats()
	require.Equal(t, uint64(1), stats.Executed)
	require.Equal(t, uint64(1), stats.Succeeded)
	require.False(t, stats.Enabled, "runtime tests cannot change the compile-time production guard")
}

func TestOpenAIRouteActiveProbeSkipsWhenNotDueOrLeaseHeld(t *testing.T) {
	now := time.Now().UTC()
	client := &openAIRouteProbeClientStub{outcome: OpenAIRouteProbeOutcome{Success: true}}
	store := &openAIRouteProbeHealthStoreStub{
		state: OpenAIRouteHealthState{State: OpenAIRouteCircuitOpen, OpenUntil: now.Add(time.Minute)},
	}
	runner := NewOpenAIRouteActiveProbeRunner(store, client)
	decision, err := runner.run(context.Background(), testOpenAIRouteProbeRequest(now), true)
	require.NoError(t, err)
	require.Equal(t, "probe_not_due", decision.Reason)
	require.Zero(t, client.calls)

	store.state = dueOpenAIRouteProbeState(now)
	store.permitOwner = "other-instance"
	decision, err = runner.run(context.Background(), testOpenAIRouteProbeRequest(now), true)
	require.NoError(t, err)
	require.Equal(t, "probe_lease_held_elsewhere", decision.Reason)
	require.Zero(t, client.calls)
}

func TestOpenAIRouteActiveProbeRechecksDueStateUnderLease(t *testing.T) {
	now := time.Now().UTC()
	healthy := OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy, LastEventAt: now}
	store := &openAIRouteProbeHealthStoreStub{
		state: dueOpenAIRouteProbeState(now), stateOnAcquire: &healthy, refreshAllowed: true,
	}
	client := &openAIRouteProbeClientStub{outcome: OpenAIRouteProbeOutcome{Success: true}}
	runner := NewOpenAIRouteActiveProbeRunner(store, client)

	decision, err := runner.run(context.Background(), testOpenAIRouteProbeRequest(now), true)

	require.NoError(t, err)
	require.Equal(t, "probe_no_longer_due", decision.Reason)
	require.Zero(t, client.calls)
	require.Equal(t, 1, store.releaseCalls)
}

func TestOpenAIRouteActiveProbeRejectsConcurrentLocalDuplicate(t *testing.T) {
	now := time.Now().UTC()
	release := make(chan struct{})
	client := &openAIRouteProbeClientStub{
		outcome: OpenAIRouteProbeOutcome{Success: true},
		started: make(chan struct{}, 1),
		release: release,
	}
	store := &openAIRouteProbeHealthStoreStub{state: dueOpenAIRouteProbeState(now), refreshAllowed: true}
	runner := NewOpenAIRouteActiveProbeRunner(store, client)
	done := make(chan error, 1)
	go func() {
		_, err := runner.run(context.Background(), testOpenAIRouteProbeRequest(now), true)
		done <- err
	}()
	<-client.started

	decision, err := runner.run(context.Background(), testOpenAIRouteProbeRequest(now), true)
	require.NoError(t, err)
	require.Equal(t, "probe_already_running_locally", decision.Reason)
	require.Equal(t, 1, client.calls)
	close(release)
	require.NoError(t, <-done)
}

func TestOpenAIRouteActiveProbeCancelsWhenLeaseOwnershipIsLost(t *testing.T) {
	now := time.Now().UTC()
	neverRelease := make(chan struct{})
	client := &openAIRouteProbeClientStub{started: make(chan struct{}, 1), release: neverRelease}
	store := &openAIRouteProbeHealthStoreStub{state: dueOpenAIRouteProbeState(now), refreshAllowed: false}
	runner := NewOpenAIRouteActiveProbeRunner(store, client)
	runner.timeout = time.Second
	runner.renewEvery = 10 * time.Millisecond

	decision, err := runner.run(context.Background(), testOpenAIRouteProbeRequest(now), true)

	require.ErrorIs(t, err, ErrOpenAIRouteInvalidProbe)
	require.True(t, decision.Executed)
	require.Empty(t, store.applyEvents, "a result obtained without a retained single-owner lease must not mutate health")
	require.Equal(t, uint64(1), runner.Stats().LeaseLost)
}

func TestOpenAIRouteActiveProbeConfirmsOwnershipBeforeHealthWrite(t *testing.T) {
	now := time.Now().UTC()
	client := &openAIRouteProbeClientStub{outcome: OpenAIRouteProbeOutcome{Success: true}}
	store := &openAIRouteProbeHealthStoreStub{state: dueOpenAIRouteProbeState(now), refreshAllowed: false}
	runner := NewOpenAIRouteActiveProbeRunner(store, client)

	decision, err := runner.run(context.Background(), testOpenAIRouteProbeRequest(now), true)

	require.ErrorIs(t, err, ErrOpenAIRouteInvalidProbe)
	require.True(t, decision.Executed)
	require.Equal(t, 1, client.calls)
	require.Empty(t, store.applyEvents)
	require.Equal(t, 1, store.refreshCalls)
	require.Equal(t, uint64(1), runner.Stats().LeaseLost)
}

func TestOpenAIRouteProbeOutcomeRejectsNeutralFailure(t *testing.T) {
	require.ErrorIs(t, (OpenAIRouteProbeOutcome{}).Validate(), ErrOpenAIRouteInvalidProbe)
	require.ErrorIs(t, (OpenAIRouteProbeOutcome{FailureClass: OpenAIRouteFailureClientCancelled}).Validate(), ErrOpenAIRouteInvalidProbe)
	require.NoError(t, (OpenAIRouteProbeOutcome{FailureClass: OpenAIRouteFailureCapacity}).Validate())
}

package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClassifyOpenAIRouteFailure_RecognizesInnerSSECapacityError(t *testing.T) {
	classification := ClassifyOpenAIRouteFailure(OpenAIRouteFailureSignal{
		StatusCode: 200,
		Message:    "upstream returned no available token",
		HasError:   true,
	})
	require.Equal(t, OpenAIRouteFailureCapacity, classification.Class)
	require.True(t, classification.PenalizeRoute)
	require.False(t, classification.PartialStream)

	classification = ClassifyOpenAIRouteFailure(OpenAIRouteFailureSignal{
		StatusCode:    200,
		Message:       "overloaded",
		HasError:      true,
		StreamStarted: true,
	})
	require.Equal(t, OpenAIRouteFailureCapacity, classification.Class)
	require.True(t, classification.PartialStream)
}

func TestClassifyOpenAIRouteFailure_DoesNotPenalizeUserRequest(t *testing.T) {
	classification := ClassifyOpenAIRouteFailure(OpenAIRouteFailureSignal{
		StatusCode: 400,
		Message:    "invalid request parameter",
		HasError:   true,
	})
	require.Equal(t, OpenAIRouteFailureUserRequest, classification.Class)
	require.False(t, classification.PenalizeRoute)
}

func TestClassifyOpenAIRouteFailure_RecognizesBalanceBeforeGeneric403Auth(t *testing.T) {
	classification := ClassifyOpenAIRouteFailure(OpenAIRouteFailureSignal{
		StatusCode: 403,
		Message:    "insufficient balance",
		HasError:   true,
	})
	require.Equal(t, OpenAIRouteFailurePayment, classification.Class)
	require.True(t, classification.PenalizeRoute)
}

func TestOpenAIRouteFailureCanEscalateProvider_IsConservative(t *testing.T) {
	for _, class := range []OpenAIRouteFailureClass{
		OpenAIRouteFailureCapacity,
		OpenAIRouteFailureUpstream5xx,
		OpenAIRouteFailureMalformedStream,
		OpenAIRouteFailurePartialStream,
	} {
		require.True(t, OpenAIRouteFailureCanEscalateProvider(class), class)
	}
	for _, class := range []OpenAIRouteFailureClass{
		OpenAIRouteFailureRateLimit,
		OpenAIRouteFailureAuthentication,
		OpenAIRouteFailurePayment,
		OpenAIRouteFailureModelUnsupported,
		OpenAIRouteFailureLocalTransport,
		OpenAIRouteFailureUserRequest,
	} {
		require.False(t, OpenAIRouteFailureCanEscalateProvider(class), class)
	}
}

func TestApplyOpenAIRouteHealthEvent_GenericFailureDegradesThenOpens(t *testing.T) {
	policy := DefaultOpenAIRoutePolicy()
	start := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	state := OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}

	state, err := ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{
		At:           start,
		FailureClass: OpenAIRouteFailureUpstream5xx,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteCircuitDegraded, state.State)
	require.Equal(t, 1, state.ConsecutiveFailures)

	state, err = ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{
		At:           start.Add(10 * time.Second),
		FailureClass: OpenAIRouteFailureUpstream5xx,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteCircuitOpen, state.State)
	require.Equal(t, 1, state.EjectionCount)
	require.Equal(t, start.Add(15*time.Second), state.OpenUntil)
	require.False(t, state.ProbeDue(start.Add(14*time.Second)))
	require.True(t, state.ProbeDue(start.Add(15*time.Second)))
}

func TestApplyOpenAIRouteHealthEvent_CapacityFastRecoveryAndRamp(t *testing.T) {
	policy := DefaultOpenAIRoutePolicy()
	start := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	state := OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}

	state, err := ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{
		At:           start,
		FailureClass: OpenAIRouteFailureCapacity,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteCircuitOpen, state.State)
	require.Equal(t, start.Add(5*time.Second), state.OpenUntil)

	state, err = ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{
		At:      start.Add(5 * time.Second),
		Success: true,
		Probe:   true,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteCircuitHalfOpen, state.State)

	state, err = ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{
		At:      start.Add(6 * time.Second),
		Success: true,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteCircuitRecovering, state.State)
	require.InDelta(t, 0.01, state.TrafficShareCap(policy), 1e-12)

	for step, expectedShare := range []float64{0.05, 0.20, 0.50} {
		state, err = ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{
			At:      start.Add(time.Duration(7+step) * time.Second),
			Success: true,
		}, policy)
		require.NoError(t, err)
		require.Equal(t, OpenAIRouteCircuitRecovering, state.State)
		require.InDelta(t, expectedShare, state.TrafficShareCap(policy), 1e-12)
	}

	state, err = ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{
		At:      start.Add(10 * time.Second),
		Success: true,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteCircuitHealthy, state.State)
	require.InDelta(t, 1, state.TrafficShareCap(policy), 1e-12)
}

func TestApplyOpenAIRouteHealthEvent_RepeatFailureIncreasesBackoff(t *testing.T) {
	policy := DefaultOpenAIRoutePolicy()
	start := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	state := OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}
	state, _ = ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{At: start, FailureClass: OpenAIRouteFailureCapacity}, policy)
	state, _ = ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{At: start.Add(5 * time.Second), Success: true, Probe: true}, policy)
	state, err := ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{At: start.Add(6 * time.Second), FailureClass: OpenAIRouteFailureCapacity}, policy)
	require.NoError(t, err)
	require.Equal(t, 2, state.EjectionCount)
	require.Equal(t, start.Add(16*time.Second), state.OpenUntil)
}

func TestApplyOpenAIRouteHealthEvent_AuthAndModelFailuresUseLongBackoff(t *testing.T) {
	policy := DefaultOpenAIRoutePolicy()
	start := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

	auth, err := ApplyOpenAIRouteHealthEvent(OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}, OpenAIRouteHealthEvent{
		At:           start,
		FailureClass: OpenAIRouteFailureAuthentication,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, start.Add(15*time.Minute), auth.OpenUntil)

	model, err := ApplyOpenAIRouteHealthEvent(OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}, OpenAIRouteHealthEvent{
		At:           start,
		FailureClass: OpenAIRouteFailureModelUnsupported,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, start.Add(30*time.Minute), model.OpenUntil)

	rate, err := ApplyOpenAIRouteHealthEvent(OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}, OpenAIRouteHealthEvent{
		At:           start,
		FailureClass: OpenAIRouteFailureRateLimit,
		RetryAfter:   45 * time.Second,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, start.Add(45*time.Second), rate.OpenUntil)
}

func TestApplyOpenAIRouteHealthEvent_UserFailureAndDisabledStateAreStable(t *testing.T) {
	policy := DefaultOpenAIRoutePolicy()
	start := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)

	healthy := OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}
	got, err := ApplyOpenAIRouteHealthEvent(healthy, OpenAIRouteHealthEvent{At: start, FailureClass: OpenAIRouteFailureUserRequest}, policy)
	require.NoError(t, err)
	require.Equal(t, healthy, got)

	disabled := OpenAIRouteHealthState{State: OpenAIRouteCircuitDisabled}
	got, err = ApplyOpenAIRouteHealthEvent(disabled, OpenAIRouteHealthEvent{At: start, Success: true}, policy)
	require.NoError(t, err)
	require.Equal(t, disabled, got)
}

func TestApplyOpenAIRouteHealthEvent_IgnoresOutOfOrderDistributedEvent(t *testing.T) {
	policy := DefaultOpenAIRoutePolicy()
	start := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	state := OpenAIRouteHealthState{State: OpenAIRouteCircuitHealthy}
	state, err := ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{
		At:           start.Add(10 * time.Second),
		FailureClass: OpenAIRouteFailureCapacity,
	}, policy)
	require.NoError(t, err)

	got, err := ApplyOpenAIRouteHealthEvent(state, OpenAIRouteHealthEvent{
		At:      start,
		Success: true,
	}, policy)
	require.NoError(t, err)
	require.Equal(t, state, got)
}

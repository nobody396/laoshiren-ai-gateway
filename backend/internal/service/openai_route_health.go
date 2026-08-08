package service

import (
	"strings"
	"time"
)

type OpenAIRouteHealthState struct {
	State OpenAIRouteCircuitState

	ConsecutiveFailures int
	EjectionCount       int
	RecoveryStep        int

	LastFailureAt    time.Time
	LastSuccessAt    time.Time
	LastEventAt      time.Time
	OpenUntil        time.Time
	LastFailureClass OpenAIRouteFailureClass
}

type OpenAIRouteHealthEvent struct {
	At      time.Time
	Success bool
	Probe   bool

	FailureClass OpenAIRouteFailureClass
	RetryAfter   time.Duration
	RetryAt      time.Time
}

type OpenAIRouteFailureSignal struct {
	StatusCode    int
	ErrorCode     string
	Message       string
	LocalOrigin   bool
	StreamStarted bool
	MalformedSSE  bool
	HasError      bool
}

type OpenAIRouteFailureClassification struct {
	Class         OpenAIRouteFailureClass
	PartialStream bool
	PenalizeRoute bool
}

func NewOpenAIRouteHealthState() OpenAIRouteHealthState {
	return OpenAIRouteHealthState{State: OpenAIRouteCircuitWarmup}
}

// ClassifyOpenAIRouteFailure converts a structured upstream signal into a
// routing class. It deliberately does not retain the original error body.
func ClassifyOpenAIRouteFailure(signal OpenAIRouteFailureSignal) OpenAIRouteFailureClassification {
	partial := signal.StreamStarted && (signal.HasError || signal.StatusCode >= 400 || signal.LocalOrigin || signal.MalformedSSE)
	result := OpenAIRouteFailureClassification{PartialStream: partial}
	if !signal.HasError && signal.StatusCode >= 200 && signal.StatusCode < 400 && !signal.LocalOrigin && !signal.MalformedSSE {
		result.Class = OpenAIRouteFailureNone
		return result
	}

	message := strings.ToLower(strings.TrimSpace(signal.ErrorCode + " " + signal.Message))
	if signal.LocalOrigin {
		result.Class = OpenAIRouteFailureLocalTransport
		result.PenalizeRoute = true
		return result
	}
	if containsAnyOpenAIRouteKeyword(message,
		"model not supported", "unsupported model", "model_not_found", "unknown model", "does not support the requested model",
	) {
		result.Class = OpenAIRouteFailureModelUnsupported
		result.PenalizeRoute = true
		return result
	}
	if signal.StatusCode == 429 || containsAnyOpenAIRouteKeyword(message, "rate limit", "rate_limit", "too many requests") {
		result.Class = OpenAIRouteFailureRateLimit
		result.PenalizeRoute = true
		return result
	}
	if containsAnyOpenAIRouteKeyword(message,
		"no token", "no available token", "token unavailable", "overload", "overloaded", "capacity", "service busy", "server busy",
	) {
		result.Class = OpenAIRouteFailureCapacity
		result.PenalizeRoute = true
		return result
	}
	if signal.StatusCode == 402 || containsAnyOpenAIRouteKeyword(message, "insufficient balance", "insufficient quota", "payment required") {
		result.Class = OpenAIRouteFailurePayment
		result.PenalizeRoute = true
		return result
	}
	if signal.StatusCode == 401 || signal.StatusCode == 403 || containsAnyOpenAIRouteKeyword(message, "invalid api key", "unauthorized", "authentication") {
		result.Class = OpenAIRouteFailureAuthentication
		result.PenalizeRoute = true
		return result
	}
	if signal.StatusCode >= 500 {
		result.Class = OpenAIRouteFailureUpstream5xx
		result.PenalizeRoute = true
		return result
	}
	if signal.MalformedSSE || (signal.StatusCode >= 200 && signal.StatusCode < 300 && signal.HasError) {
		result.Class = OpenAIRouteFailureMalformedStream
		result.PenalizeRoute = true
		return result
	}
	if signal.StatusCode >= 400 && signal.StatusCode < 500 {
		result.Class = OpenAIRouteFailureUserRequest
		return result
	}
	if partial {
		result.Class = OpenAIRouteFailurePartialStream
		result.PenalizeRoute = true
		return result
	}
	if signal.HasError {
		result.Class = OpenAIRouteFailureLocalTransport
		result.PenalizeRoute = true
		return result
	}
	result.Class = OpenAIRouteFailureNone
	return result
}

func containsAnyOpenAIRouteKeyword(value string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(value, keyword) {
			return true
		}
	}
	return false
}

func ApplyOpenAIRouteHealthEvent(state OpenAIRouteHealthState, event OpenAIRouteHealthEvent, policy OpenAIRoutePolicy) (OpenAIRouteHealthState, error) {
	normalized, err := NormalizeOpenAIRoutePolicy(policy)
	if err != nil {
		return state, err
	}
	if state.State == "" {
		state = NewOpenAIRouteHealthState()
	}
	if state.State == OpenAIRouteCircuitDisabled {
		return state, nil
	}
	if event.At.IsZero() {
		event.At = time.Now()
	}
	if !state.LastEventAt.IsZero() && event.At.Before(state.LastEventAt) {
		return state, nil
	}
	if event.Success || event.FailureClass == OpenAIRouteFailureNone {
		return applyOpenAIRouteSuccess(state, event, normalized), nil
	}
	if event.FailureClass == OpenAIRouteFailureUserRequest {
		return state, nil
	}
	return applyOpenAIRouteFailure(state, event, normalized), nil
}

func applyOpenAIRouteSuccess(state OpenAIRouteHealthState, event OpenAIRouteHealthEvent, policy OpenAIRoutePolicy) OpenAIRouteHealthState {
	state.LastEventAt = event.At
	state.LastSuccessAt = event.At
	state.ConsecutiveFailures = 0
	state.LastFailureClass = OpenAIRouteFailureNone

	if event.Probe {
		switch state.State {
		case OpenAIRouteCircuitOpen, OpenAIRouteCircuitWarmup:
			state.State = OpenAIRouteCircuitHalfOpen
			state.OpenUntil = time.Time{}
		case OpenAIRouteCircuitDegraded:
			state.State = OpenAIRouteCircuitHealthy
		}
		return state
	}

	switch state.State {
	case OpenAIRouteCircuitWarmup, OpenAIRouteCircuitHalfOpen, OpenAIRouteCircuitOpen:
		state.State = OpenAIRouteCircuitRecovering
		state.RecoveryStep = 0
		state.OpenUntil = time.Time{}
	case OpenAIRouteCircuitRecovering:
		state.RecoveryStep++
		if state.RecoveryStep >= len(policy.RecoveryShares)-1 {
			state.State = OpenAIRouteCircuitHealthy
			state.RecoveryStep = len(policy.RecoveryShares) - 1
			state.EjectionCount = 0
		}
	case OpenAIRouteCircuitDegraded:
		state.State = OpenAIRouteCircuitHealthy
		state.RecoveryStep = len(policy.RecoveryShares) - 1
	case OpenAIRouteCircuitHealthy:
		state.EjectionCount = 0
	}
	return state
}

func applyOpenAIRouteFailure(state OpenAIRouteHealthState, event OpenAIRouteHealthEvent, policy OpenAIRoutePolicy) OpenAIRouteHealthState {
	state.LastEventAt = event.At
	withinWindow := !state.LastFailureAt.IsZero() && event.At.Sub(state.LastFailureAt) <= policy.FailureWindow
	if !withinWindow {
		state.ConsecutiveFailures = 0
	}
	state.ConsecutiveFailures++
	state.LastFailureAt = event.At
	state.LastFailureClass = event.FailureClass

	immediate := isImmediateOpenAIRouteFailure(event.FailureClass)
	unstableState := state.State == OpenAIRouteCircuitWarmup || state.State == OpenAIRouteCircuitHalfOpen || state.State == OpenAIRouteCircuitRecovering || state.State == OpenAIRouteCircuitOpen
	if immediate || unstableState || state.ConsecutiveFailures >= policy.GenericFailThreshold {
		state.State = OpenAIRouteCircuitOpen
		state.RecoveryStep = 0
		state.EjectionCount++
		state.OpenUntil = openAIRouteNextProbeAt(event, state.EjectionCount, policy)
		return state
	}

	state.State = OpenAIRouteCircuitDegraded
	return state
}

func isImmediateOpenAIRouteFailure(class OpenAIRouteFailureClass) bool {
	switch class {
	case OpenAIRouteFailureModelUnsupported,
		OpenAIRouteFailureCapacity,
		OpenAIRouteFailureRateLimit,
		OpenAIRouteFailureAuthentication,
		OpenAIRouteFailurePayment:
		return true
	default:
		return false
	}
}

func openAIRouteNextProbeAt(event OpenAIRouteHealthEvent, ejectionCount int, policy OpenAIRoutePolicy) time.Time {
	if ejectionCount <= 0 {
		ejectionCount = 1
	}
	idx := ejectionCount - 1
	if idx >= len(policy.ProbeBackoff) {
		idx = len(policy.ProbeBackoff) - 1
	}
	backoff := policy.ProbeBackoff[idx]
	switch event.FailureClass {
	case OpenAIRouteFailureAuthentication, OpenAIRouteFailurePayment:
		if backoff < 15*time.Minute {
			backoff = 15 * time.Minute
		}
	case OpenAIRouteFailureModelUnsupported:
		if backoff < 30*time.Minute {
			backoff = 30 * time.Minute
		}
	}
	if event.RetryAfter > backoff {
		backoff = event.RetryAfter
	}
	next := event.At.Add(backoff)
	if event.RetryAt.After(next) {
		next = event.RetryAt
	}
	return next
}

func (s OpenAIRouteHealthState) ProbeDue(now time.Time) bool {
	return s.State == OpenAIRouteCircuitOpen && !now.Before(s.OpenUntil)
}

func (s OpenAIRouteHealthState) TrafficShareCap(policy OpenAIRoutePolicy) float64 {
	normalized, err := NormalizeOpenAIRoutePolicy(policy)
	if err != nil {
		return 0
	}
	switch s.State {
	case OpenAIRouteCircuitHealthy:
		return 1
	case OpenAIRouteCircuitDegraded:
		return normalized.DegradedShare
	case OpenAIRouteCircuitWarmup, OpenAIRouteCircuitHalfOpen:
		return normalized.NewAccountShare
	case OpenAIRouteCircuitRecovering:
		idx := s.RecoveryStep
		if idx < 0 {
			idx = 0
		}
		if idx >= len(normalized.RecoveryShares) {
			idx = len(normalized.RecoveryShares) - 1
		}
		return normalized.RecoveryShares[idx]
	default:
		return 0
	}
}

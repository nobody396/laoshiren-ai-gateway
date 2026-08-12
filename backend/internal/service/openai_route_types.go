package service

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// OpenAIRouteCircuitState is intentionally independent from Account.Status.
// Account.Status is control-plane state; this state is short-lived, scoped to a
// concrete account/model/endpoint/transport route, and belongs in the shared
// routing runtime store.
type OpenAIRouteCircuitState string

const (
	OpenAIRouteCircuitWarmup     OpenAIRouteCircuitState = "warmup"
	OpenAIRouteCircuitHealthy    OpenAIRouteCircuitState = "healthy"
	OpenAIRouteCircuitDegraded   OpenAIRouteCircuitState = "degraded"
	OpenAIRouteCircuitOpen       OpenAIRouteCircuitState = "open"
	OpenAIRouteCircuitHalfOpen   OpenAIRouteCircuitState = "half_open"
	OpenAIRouteCircuitRecovering OpenAIRouteCircuitState = "recovering"
	OpenAIRouteCircuitDisabled   OpenAIRouteCircuitState = "disabled"
)

type OpenAIRouteFailureClass string

const (
	OpenAIRouteFailureNone             OpenAIRouteFailureClass = "none"
	OpenAIRouteFailureUserRequest      OpenAIRouteFailureClass = "user_request"
	OpenAIRouteFailureClientCancelled  OpenAIRouteFailureClass = "client_cancelled"
	OpenAIRouteFailureModelUnsupported OpenAIRouteFailureClass = "model_unsupported"
	OpenAIRouteFailureCapacity         OpenAIRouteFailureClass = "capacity"
	OpenAIRouteFailureRateLimit        OpenAIRouteFailureClass = "rate_limit"
	OpenAIRouteFailureAuthentication   OpenAIRouteFailureClass = "authentication"
	OpenAIRouteFailurePayment          OpenAIRouteFailureClass = "payment"
	OpenAIRouteFailureUpstream5xx      OpenAIRouteFailureClass = "upstream_5xx"
	OpenAIRouteFailureLocalTransport   OpenAIRouteFailureClass = "local_transport"
	OpenAIRouteFailureMalformedStream  OpenAIRouteFailureClass = "malformed_stream"
	OpenAIRouteFailurePartialStream    OpenAIRouteFailureClass = "partial_stream"
)

type OpenAIRoutePolicyMode string

const (
	OpenAIRoutePolicyLegacy  OpenAIRoutePolicyMode = "legacy"
	OpenAIRoutePolicyShadow  OpenAIRoutePolicyMode = "shadow"
	OpenAIRoutePolicyEnforce OpenAIRoutePolicyMode = "enforce"
)

// OpenAIRouteRequestClass separates semantically different workloads that may
// share the same public model and account. Text observations must never train
// image routing (or vice versa).
type OpenAIRouteRequestClass string

const (
	OpenAIRouteRequestClassUnknown OpenAIRouteRequestClass = "unknown"
	OpenAIRouteRequestClassText    OpenAIRouteRequestClass = "text"
	OpenAIRouteRequestClassImage   OpenAIRouteRequestClass = "image"
)

func (c OpenAIRouteRequestClass) Valid() bool {
	switch c {
	case OpenAIRouteRequestClassText, OpenAIRouteRequestClassImage:
		return true
	default:
		return false
	}
}

var (
	ErrOpenAIRouteNoCandidate         = errors.New("no eligible OpenAI route candidate")
	ErrOpenAIRouteBudgetExhausted     = errors.New("OpenAI route cost budget exhausted")
	ErrOpenAIRouteInvalidPolicy       = errors.New("invalid OpenAI route policy")
	ErrOpenAIRouteInvalidCost         = errors.New("invalid OpenAI route cost")
	ErrOpenAIRouteReservationConflict = errors.New("OpenAI route budget reservation conflict")
)

// OpenAIRoutePolicy contains only policy values needed by the parallel-safe
// core. Storage, admin APIs, feature flags, and production wiring are separate
// integration concerns.
type OpenAIRoutePolicy struct {
	Mode OpenAIRoutePolicyMode

	TargetAverageMultiplier float64
	HardAverageMultiplier   float64
	EmergencyDebtLimitUSD   float64
	MaxCreditUSD            float64

	PriceExponent   float64
	LatencyBeta     float64
	PriorityPenalty float64
	MinHealthFactor float64

	MaxAccountShare      float64
	MaxProviderShare     float64
	NewAccountShare      float64
	DegradedShare        float64
	RecoveryShares       []float64
	GenericFailThreshold int
	FailureWindow        time.Duration
	ProbeBackoff         []time.Duration
}

func DefaultOpenAIRoutePolicy() OpenAIRoutePolicy {
	return OpenAIRoutePolicy{
		Mode:                    OpenAIRoutePolicyShadow,
		TargetAverageMultiplier: 1,
		HardAverageMultiplier:   1,
		EmergencyDebtLimitUSD:   0,
		MaxCreditUSD:            1,
		PriceExponent:           2,
		LatencyBeta:             1,
		PriorityPenalty:         0.15,
		MinHealthFactor:         0.01,
		MaxAccountShare:         0.80,
		MaxProviderShare:        0.90,
		NewAccountShare:         0.01,
		DegradedShare:           0.10,
		RecoveryShares:          []float64{0.01, 0.05, 0.20, 0.50, 1},
		GenericFailThreshold:    2,
		FailureWindow:           30 * time.Second,
		ProbeBackoff:            []time.Duration{5 * time.Second, 10 * time.Second, 20 * time.Second, 40 * time.Second, 60 * time.Second},
	}
}

func NormalizeOpenAIRoutePolicy(policy OpenAIRoutePolicy) (OpenAIRoutePolicy, error) {
	defaults := DefaultOpenAIRoutePolicy()
	if policy.Mode == "" {
		policy.Mode = defaults.Mode
	}
	switch policy.Mode {
	case OpenAIRoutePolicyLegacy, OpenAIRoutePolicyShadow, OpenAIRoutePolicyEnforce:
	default:
		return policy, fmt.Errorf("%w: unsupported mode %q", ErrOpenAIRouteInvalidPolicy, policy.Mode)
	}
	if !isFiniteNonNegative(policy.TargetAverageMultiplier) {
		return policy, fmt.Errorf("%w: target multiplier must be finite and non-negative", ErrOpenAIRouteInvalidPolicy)
	}
	if policy.HardAverageMultiplier == 0 && policy.TargetAverageMultiplier > 0 {
		policy.HardAverageMultiplier = policy.TargetAverageMultiplier
	}
	if !isFiniteNonNegative(policy.HardAverageMultiplier) || policy.HardAverageMultiplier < policy.TargetAverageMultiplier {
		return policy, fmt.Errorf("%w: hard multiplier must be finite and >= target multiplier", ErrOpenAIRouteInvalidPolicy)
	}
	if !isFiniteNonNegative(policy.EmergencyDebtLimitUSD) || !isFiniteNonNegative(policy.MaxCreditUSD) {
		return policy, fmt.Errorf("%w: budget limits must be finite and non-negative", ErrOpenAIRouteInvalidPolicy)
	}
	if policy.PriceExponent <= 0 || math.IsNaN(policy.PriceExponent) || math.IsInf(policy.PriceExponent, 0) {
		policy.PriceExponent = defaults.PriceExponent
	}
	if policy.LatencyBeta <= 0 || math.IsNaN(policy.LatencyBeta) || math.IsInf(policy.LatencyBeta, 0) {
		policy.LatencyBeta = defaults.LatencyBeta
	}
	if policy.PriorityPenalty < 0 || math.IsNaN(policy.PriorityPenalty) || math.IsInf(policy.PriorityPenalty, 0) {
		policy.PriorityPenalty = defaults.PriorityPenalty
	}
	if policy.MinHealthFactor <= 0 || policy.MinHealthFactor > 1 || math.IsNaN(policy.MinHealthFactor) {
		policy.MinHealthFactor = defaults.MinHealthFactor
	}
	policy.MaxAccountShare = normalizeShare(policy.MaxAccountShare, defaults.MaxAccountShare)
	policy.MaxProviderShare = normalizeShare(policy.MaxProviderShare, defaults.MaxProviderShare)
	policy.NewAccountShare = normalizeShare(policy.NewAccountShare, defaults.NewAccountShare)
	policy.DegradedShare = normalizeShare(policy.DegradedShare, defaults.DegradedShare)
	policy.RecoveryShares = normalizeRecoveryShares(policy.RecoveryShares, defaults.RecoveryShares)
	if policy.GenericFailThreshold <= 0 {
		policy.GenericFailThreshold = defaults.GenericFailThreshold
	}
	if policy.FailureWindow <= 0 {
		policy.FailureWindow = defaults.FailureWindow
	}
	policy.ProbeBackoff = normalizeProbeBackoff(policy.ProbeBackoff, defaults.ProbeBackoff)
	return policy, nil
}

func normalizeShare(value, fallback float64) float64 {
	if value <= 0 || value > 1 || math.IsNaN(value) || math.IsInf(value, 0) {
		return fallback
	}
	return value
}

func normalizeRecoveryShares(values, fallback []float64) []float64 {
	if len(values) == 0 {
		return append([]float64(nil), fallback...)
	}
	out := make([]float64, 0, len(values))
	last := 0.0
	for _, value := range values {
		if value <= 0 || value > 1 || math.IsNaN(value) || math.IsInf(value, 0) || value < last {
			return append([]float64(nil), fallback...)
		}
		out = append(out, value)
		last = value
	}
	if out[len(out)-1] < 1 {
		out = append(out, 1)
	}
	return out
}

func normalizeProbeBackoff(values, fallback []time.Duration) []time.Duration {
	if len(values) == 0 {
		return append([]time.Duration(nil), fallback...)
	}
	out := make([]time.Duration, 0, len(values))
	last := time.Duration(0)
	for _, value := range values {
		if value <= 0 || value < last {
			return append([]time.Duration(nil), fallback...)
		}
		out = append(out, value)
		last = value
	}
	return out
}

func isFiniteNonNegative(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

type OpenAIRouteKey struct {
	GroupID       int64
	AccountID     int64
	Model         string
	RequestClass  OpenAIRouteRequestClass
	EndpointHash  string
	Transport     string
	FailureDomain string
}

func (k OpenAIRouteKey) Valid() bool {
	return k.GroupID > 0 &&
		k.AccountID > 0 &&
		strings.TrimSpace(k.Model) != "" &&
		k.RequestClass.Valid() &&
		strings.TrimSpace(k.EndpointHash) != "" &&
		strings.TrimSpace(k.Transport) != ""
}

// OpenAIRouteCandidate is a point-in-time projection of an Account plus shared
// route health. The allocator intentionally knows nothing about concrete
// suppliers or account IDs.
type OpenAIRouteCandidate struct {
	Key OpenAIRouteKey

	RateMultiplier float64
	Priority       int

	CircuitState         OpenAIRouteCircuitState
	ProviderCircuitState OpenAIRouteCircuitState
	HalfOpenPermit       bool
	RecoveryStep         int

	HasReliabilitySample             bool
	SuccessLowerBound                float64
	P90TTFTMilliseconds              float64
	P95CompletionLatencyMilliseconds float64
	PartialStreamRate                float64
	ObservationSampleCount           uint64
	LoadRatio                        float64
	WaitingCount                     int

	CurrentAccountShare  float64
	CurrentProviderShare float64
	ExplorationBoost     float64

	// EstimatedBaseCostUSD is route-specific when enough authoritative text
	// settlements exist. Zero keeps the request-level policy estimate for
	// compatibility and for image routes whose supplier cost is not verified.
	EstimatedBaseCostUSD   float64
	ObservedMeanCostUSD    float64
	CostObservationSamples uint64
	CostEstimateSource     string
}

type OpenAIRouteExclusionReason string

const (
	OpenAIRouteExcludedInvalid      OpenAIRouteExclusionReason = "invalid_candidate"
	OpenAIRouteExcludedCircuitOpen  OpenAIRouteExclusionReason = "route_circuit_open"
	OpenAIRouteExcludedProviderOpen OpenAIRouteExclusionReason = "provider_circuit_open"
	OpenAIRouteExcludedHalfOpen     OpenAIRouteExclusionReason = "half_open_without_permit"
	OpenAIRouteExcludedRecoveryCap  OpenAIRouteExclusionReason = "recovery_share_cap"
	OpenAIRouteExcludedAccountCap   OpenAIRouteExclusionReason = "account_share_cap"
	OpenAIRouteExcludedProviderCap  OpenAIRouteExclusionReason = "provider_share_cap"
	OpenAIRouteExcludedCost         OpenAIRouteExclusionReason = "cost_budget"
)

type OpenAIRouteExclusion struct {
	AccountID int64
	Reason    OpenAIRouteExclusionReason
}

type OpenAIRouteWeightedCandidate struct {
	Candidate             OpenAIRouteCandidate
	Weight                float64
	HealthFactor          float64
	LatencyFactor         float64
	TailLatencyFactor     float64
	StreamIntegrityFactor float64
	HeadroomFactor        float64
	PriceFactor           float64
	PriorityFactor        float64
	PredictedExtraCost    float64
	EmergencyBudgetUsed   bool
}

type OpenAIRouteAllocationRequest struct {
	Policy OpenAIRoutePolicy
	// Budget preserves the single-window call shape for legacy/shadow callers.
	// When Budgets is non-empty, every window must admit the candidate.
	Budget  OpenAIRouteBudgetLedger
	Budgets []OpenAIRouteBudgetLedger

	Candidates           []OpenAIRouteCandidate
	EstimatedBaseCostUSD float64
	Seed                 uint64
	HardShareCaps        bool
}

type OpenAIRouteAllocationPlan struct {
	Ranked               []OpenAIRouteWeightedCandidate
	Selected             OpenAIRouteWeightedCandidate
	Excluded             []OpenAIRouteExclusion
	Emergency            bool
	MinHealthyMultiplier float64
}

func (p OpenAIRouteAllocationPlan) Empty() bool {
	return len(p.Ranked) == 0 || p.Selected.Candidate.Key.AccountID <= 0
}

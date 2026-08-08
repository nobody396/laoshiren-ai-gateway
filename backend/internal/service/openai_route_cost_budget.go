package service

import (
	"fmt"
	"math"
)

// OpenAIRouteBudgetLedger is a bounded, rolling-window cost view. The storage
// layer owns window rotation; this value owns only reservation and settlement
// math. EquivalentCostUSD is the cost the request would have had at multiplier
// 1. ActualAccountCostUSD is the upstream account cost after its multiplier.
type OpenAIRouteBudgetLedger struct {
	TargetAverageMultiplier float64
	HardAverageMultiplier   float64
	EmergencyDebtLimitUSD   float64
	MaxCreditUSD            float64

	CreditUSD            float64
	EquivalentCostUSD    float64
	ActualAccountCostUSD float64
}

type OpenAIRouteBudgetPreview struct {
	Allowed               bool
	Emergency             bool
	PredictedExtraCostUSD float64
	AvailableCreditUSD    float64
	Reason                OpenAIRouteExclusionReason
}

type OpenAIRouteBudgetReservation struct {
	RateMultiplier       float64
	EstimatedBaseCostUSD float64
	ReservedExtraCostUSD float64
	Emergency            bool
	settled              bool
	cancelled            bool
}

func NewOpenAIRouteBudgetLedger(policy OpenAIRoutePolicy, creditUSD, equivalentCostUSD, actualAccountCostUSD float64) (OpenAIRouteBudgetLedger, error) {
	normalized, err := NormalizeOpenAIRoutePolicy(policy)
	if err != nil {
		return OpenAIRouteBudgetLedger{}, err
	}
	ledger := OpenAIRouteBudgetLedger{
		TargetAverageMultiplier: normalized.TargetAverageMultiplier,
		HardAverageMultiplier:   normalized.HardAverageMultiplier,
		EmergencyDebtLimitUSD:   normalized.EmergencyDebtLimitUSD,
		MaxCreditUSD:            normalized.MaxCreditUSD,
		CreditUSD:               creditUSD,
		EquivalentCostUSD:       equivalentCostUSD,
		ActualAccountCostUSD:    actualAccountCostUSD,
	}
	if err := ledger.Validate(); err != nil {
		return OpenAIRouteBudgetLedger{}, err
	}
	ledger.clampCredit()
	return ledger, nil
}

func (l OpenAIRouteBudgetLedger) Validate() error {
	if !isFiniteNonNegative(l.TargetAverageMultiplier) {
		return fmt.Errorf("%w: target multiplier", ErrOpenAIRouteInvalidPolicy)
	}
	if !isFiniteNonNegative(l.HardAverageMultiplier) || l.HardAverageMultiplier < l.TargetAverageMultiplier {
		return fmt.Errorf("%w: hard multiplier", ErrOpenAIRouteInvalidPolicy)
	}
	if !isFiniteNonNegative(l.EmergencyDebtLimitUSD) || !isFiniteNonNegative(l.MaxCreditUSD) {
		return fmt.Errorf("%w: budget bounds", ErrOpenAIRouteInvalidPolicy)
	}
	if math.IsNaN(l.CreditUSD) || math.IsInf(l.CreditUSD, 0) {
		return fmt.Errorf("%w: credit", ErrOpenAIRouteInvalidCost)
	}
	if !isFiniteNonNegative(l.EquivalentCostUSD) || !isFiniteNonNegative(l.ActualAccountCostUSD) {
		return fmt.Errorf("%w: rolling totals", ErrOpenAIRouteInvalidCost)
	}
	return nil
}

func (l OpenAIRouteBudgetLedger) AverageMultiplier() (float64, bool) {
	if l.EquivalentCostUSD <= 0 {
		return 0, false
	}
	return l.ActualAccountCostUSD / l.EquivalentCostUSD, true
}

func (l OpenAIRouteBudgetLedger) HardLimitExceeded() bool {
	average, ok := l.AverageMultiplier()
	return ok && average > l.HardAverageMultiplier+1e-12
}

func (l OpenAIRouteBudgetLedger) Preview(rateMultiplier, estimatedBaseCostUSD float64) (OpenAIRouteBudgetPreview, error) {
	if err := l.Validate(); err != nil {
		return OpenAIRouteBudgetPreview{}, err
	}
	if !isFiniteNonNegative(rateMultiplier) || !isFiniteNonNegative(estimatedBaseCostUSD) {
		return OpenAIRouteBudgetPreview{}, ErrOpenAIRouteInvalidCost
	}

	extra := math.Max(0, (rateMultiplier-l.TargetAverageMultiplier)*estimatedBaseCostUSD)
	available := math.Max(0, l.CreditUSD+l.EmergencyDebtLimitUSD)
	preview := OpenAIRouteBudgetPreview{
		Allowed:               true,
		PredictedExtraCostUSD: extra,
		AvailableCreditUSD:    available,
	}

	if l.HardLimitExceeded() && rateMultiplier > l.TargetAverageMultiplier {
		preview.Allowed = false
		preview.Reason = OpenAIRouteExcludedCost
		return preview, nil
	}
	if extra > available+1e-12 {
		preview.Allowed = false
		preview.Reason = OpenAIRouteExcludedCost
		return preview, nil
	}
	preview.Emergency = extra > math.Max(0, l.CreditUSD)+1e-12
	return preview, nil
}

func (l *OpenAIRouteBudgetLedger) Reserve(rateMultiplier, estimatedBaseCostUSD float64) (*OpenAIRouteBudgetReservation, error) {
	if l == nil {
		return nil, ErrOpenAIRouteInvalidCost
	}
	preview, err := l.Preview(rateMultiplier, estimatedBaseCostUSD)
	if err != nil {
		return nil, err
	}
	if !preview.Allowed {
		return nil, ErrOpenAIRouteBudgetExhausted
	}
	reservation := &OpenAIRouteBudgetReservation{
		RateMultiplier:       rateMultiplier,
		EstimatedBaseCostUSD: estimatedBaseCostUSD,
		ReservedExtraCostUSD: preview.PredictedExtraCostUSD,
		Emergency:            preview.Emergency,
	}
	l.CreditUSD -= reservation.ReservedExtraCostUSD
	l.clampCredit()
	return reservation, nil
}

// Settle replaces the conservative pre-request reservation with authoritative
// post-request costs. actualAccountCostUSD should come from the accounting path
// when available, rather than being reconstructed from token counts here.
func (l *OpenAIRouteBudgetLedger) Settle(reservation *OpenAIRouteBudgetReservation, actualBaseCostUSD, actualAccountCostUSD float64) error {
	if l == nil || reservation == nil || reservation.settled || reservation.cancelled {
		return ErrOpenAIRouteInvalidCost
	}
	if !isFiniteNonNegative(actualBaseCostUSD) || !isFiniteNonNegative(actualAccountCostUSD) {
		return ErrOpenAIRouteInvalidCost
	}

	actualCreditDelta := l.TargetAverageMultiplier*actualBaseCostUSD - actualAccountCostUSD
	reservedCreditDelta := -reservation.ReservedExtraCostUSD
	l.CreditUSD += actualCreditDelta - reservedCreditDelta
	l.EquivalentCostUSD += actualBaseCostUSD
	l.ActualAccountCostUSD += actualAccountCostUSD
	l.clampCredit()
	reservation.settled = true
	return nil
}

func (l *OpenAIRouteBudgetLedger) Cancel(reservation *OpenAIRouteBudgetReservation) error {
	if l == nil || reservation == nil || reservation.settled || reservation.cancelled {
		return ErrOpenAIRouteInvalidCost
	}
	l.CreditUSD += reservation.ReservedExtraCostUSD
	l.clampCredit()
	reservation.cancelled = true
	return nil
}

func (l *OpenAIRouteBudgetLedger) clampCredit() {
	if l == nil {
		return
	}
	if l.CreditUSD > l.MaxCreditUSD {
		l.CreditUSD = l.MaxCreditUSD
	}
	minCredit := -l.EmergencyDebtLimitUSD
	if l.CreditUSD < minCredit {
		l.CreditUSD = minCredit
	}
}

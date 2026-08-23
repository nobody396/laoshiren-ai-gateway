package service

import "time"

type CustomerTier string

const (
	CustomerTierStandard  CustomerTier = "standard"
	CustomerTierPriority  CustomerTier = "priority"
	CustomerTierStrategic CustomerTier = "strategic"

	customerTierPolicyVersion   = 1
	customerTierWindow          = 90 * 24 * time.Hour
	customerTierDowngradeGrace  = 30 * 24 * time.Hour
	customerTierPriorityCNYFen  = int64(25_000)
	customerTierStrategicCNYFen = int64(100_000)
)

type CustomerTierPolicy struct {
	Version                  int       `json:"version"`
	RollingWindowDays        int       `json:"rolling_window_days"`
	DowngradeGraceDays       int       `json:"downgrade_grace_days"`
	PriorityThresholdCNYFen  int64     `json:"priority_threshold_cny_fen"`
	StrategicThresholdCNYFen int64     `json:"strategic_threshold_cny_fen"`
	StandardMultiplier       float64   `json:"standard_multiplier"`
	PriorityMultiplier       float64   `json:"priority_multiplier"`
	StrategicMultiplier      float64   `json:"strategic_multiplier"`
	StandardCapCNYFen        int64     `json:"standard_cap_cny_fen"`
	PriorityCapCNYFen        int64     `json:"priority_cap_cny_fen"`
	StrategicCapCNYFen       int64     `json:"strategic_cap_cny_fen"`
	EffectiveFrom            time.Time `json:"effective_from"`
}

func defaultCustomerTierPolicy() CustomerTierPolicy {
	return CustomerTierPolicy{Version: customerTierPolicyVersion, RollingWindowDays: 90, DowngradeGraceDays: 30,
		PriorityThresholdCNYFen: customerTierPriorityCNYFen, StrategicThresholdCNYFen: customerTierStrategicCNYFen,
		StandardMultiplier: 1, PriorityMultiplier: 1.25, StrategicMultiplier: 1.5,
		StandardCapCNYFen: 1_000, PriorityCapCNYFen: 5_000, StrategicCapCNYFen: 20_000}
}

func (p CustomerTierPolicy) Valid() bool {
	return p.Version > 0 && p.RollingWindowDays > 0 && p.DowngradeGraceDays >= 0 && p.PriorityThresholdCNYFen > 0 &&
		p.StrategicThresholdCNYFen > p.PriorityThresholdCNYFen && p.StandardMultiplier > 0 && p.PriorityMultiplier > 0 &&
		p.StrategicMultiplier > 0 && p.StandardCapCNYFen >= 0 && p.PriorityCapCNYFen >= 0 && p.StrategicCapCNYFen >= 0
}

func (p CustomerTierPolicy) Window() time.Duration {
	return time.Duration(p.RollingWindowDays) * 24 * time.Hour
}
func (p CustomerTierPolicy) Grace() time.Duration {
	return time.Duration(p.DowngradeGraceDays) * 24 * time.Hour
}

func (p CustomerTierPolicy) Classify(value int64) CustomerTier {
	switch {
	case value >= p.StrategicThresholdCNYFen:
		return CustomerTierStrategic
	case value >= p.PriorityThresholdCNYFen:
		return CustomerTierPriority
	default:
		return CustomerTierStandard
	}
}

func (p CustomerTierPolicy) Benefit(tier CustomerTier) CustomerTierBenefit {
	switch tier {
	case CustomerTierStrategic:
		return CustomerTierBenefit{Multiplier: p.StrategicMultiplier, CapCNYFen: p.StrategicCapCNYFen}
	case CustomerTierPriority:
		return CustomerTierBenefit{Multiplier: p.PriorityMultiplier, CapCNYFen: p.PriorityCapCNYFen}
	default:
		return CustomerTierBenefit{Multiplier: p.StandardMultiplier, CapCNYFen: p.StandardCapCNYFen}
	}
}

func (t CustomerTier) Valid() bool {
	return t == CustomerTierStandard || t == CustomerTierPriority || t == CustomerTierStrategic
}

func (t CustomerTier) rank() int {
	switch t {
	case CustomerTierStrategic:
		return 3
	case CustomerTierPriority:
		return 2
	case CustomerTierStandard:
		return 1
	default:
		return 0
	}
}

type CustomerTierBenefit struct {
	Multiplier float64 `json:"multiplier"`
	CapCNYFen  int64   `json:"cap_cny_fen"`
}

func (t CustomerTier) Benefit() CustomerTierBenefit {
	return defaultCustomerTierPolicy().Benefit(t)
}

func ClassifyCustomerTier(verifiedPaidValueCNYFen int64) CustomerTier {
	return defaultCustomerTierPolicy().Classify(verifiedPaidValueCNYFen)
}

type CustomerTierResolutionInput struct {
	Now                    time.Time
	CurrentTier            CustomerTier
	CalculatedTier         CustomerTier
	GraceExpiresAt         *time.Time
	OverrideTier           CustomerTier
	OverrideActive         bool
	PreviousOverrideActive bool
	DowngradeGrace         time.Duration
}

type CustomerTierResolution struct {
	EffectiveTier  CustomerTier
	GraceExpiresAt *time.Time
	Reason         string
}

func ResolveCustomerTier(input CustomerTierResolutionInput) CustomerTierResolution {
	if !input.CurrentTier.Valid() {
		input.CurrentTier = CustomerTierStandard
	}
	if !input.CalculatedTier.Valid() {
		input.CalculatedTier = CustomerTierStandard
	}
	if input.OverrideActive && input.OverrideTier.Valid() {
		return CustomerTierResolution{EffectiveTier: input.OverrideTier, Reason: "active_override"}
	}
	if input.PreviousOverrideActive {
		return CustomerTierResolution{EffectiveTier: input.CalculatedTier, Reason: "override_expired"}
	}
	if input.CalculatedTier.rank() > input.CurrentTier.rank() {
		return CustomerTierResolution{EffectiveTier: input.CalculatedTier, Reason: "upgrade"}
	}
	if input.CalculatedTier.rank() < input.CurrentTier.rank() {
		if input.GraceExpiresAt != nil && input.Now.Before(*input.GraceExpiresAt) {
			value := input.GraceExpiresAt.UTC()
			return CustomerTierResolution{EffectiveTier: input.CurrentTier, GraceExpiresAt: &value, Reason: "downgrade_grace_active"}
		}
		if input.GraceExpiresAt != nil {
			return CustomerTierResolution{EffectiveTier: input.CalculatedTier, Reason: "downgrade_grace_expired"}
		}
		grace := input.DowngradeGrace
		if grace <= 0 {
			grace = customerTierDowngradeGrace
		}
		expires := input.Now.UTC().Add(grace)
		return CustomerTierResolution{EffectiveTier: input.CurrentTier, GraceExpiresAt: &expires, Reason: "downgrade_grace_started"}
	}
	return CustomerTierResolution{EffectiveTier: input.CalculatedTier, Reason: "stable"}
}

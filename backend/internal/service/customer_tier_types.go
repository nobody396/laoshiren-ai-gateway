package service

import "time"

const SettingKeyCustomerTierEvaluationEnabled = "customer_tier_evaluation_enabled"

type CustomerTierEvidenceItem struct {
	SourceType           string    `json:"source_type"`
	SourceID             int64     `json:"source_id"`
	GrossAmountCNYFen    int64     `json:"gross_amount_cny_fen"`
	RefundedAmountCNYFen int64     `json:"refunded_amount_cny_fen"`
	AmountCNYFen         int64     `json:"amount_cny_fen"`
	OccurredAt           time.Time `json:"occurred_at"`
	Included             bool      `json:"included"`
	ExclusionReason      string    `json:"exclusion_reason,omitempty"`
}

type CustomerTierEvidence struct {
	WindowStartedAt                          time.Time                  `json:"window_started_at"`
	WindowEndedAt                            time.Time                  `json:"window_ended_at"`
	PaidSources                              []CustomerTierEvidenceItem `json:"paid_sources"`
	VerifiedPaidValueCNYFen                  int64                      `json:"verified_paid_value_cny_fen"`
	BalancePaidConsumptionMicros             int64                      `json:"balance_paid_consumption_micros"`
	BuilderPassConsumptionMicros             int64                      `json:"builder_pass_consumption_micros"`
	UnattributedBuilderPassConsumptionMicros int64                      `json:"unattributed_builder_pass_consumption_micros"`
	VerifiedPaidConsumptionMicros            int64                      `json:"verified_paid_consumption_micros"`
	Policy                                   CustomerTierPolicy         `json:"policy"`
}

type CustomerTierOverride struct {
	ID              int64        `json:"id"`
	UserID          int64        `json:"user_id"`
	Tier            CustomerTier `json:"tier"`
	Reason          string       `json:"reason"`
	StartsAt        time.Time    `json:"starts_at"`
	ExpiresAt       time.Time    `json:"expires_at"`
	CreatedByUserID *int64       `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
	RefreshStatus   string       `json:"refresh_status"`
	RefreshError    string       `json:"refresh_error,omitempty"`
}

type CustomerTierEvaluation struct {
	ID                            int64                `json:"id"`
	UserID                        int64                `json:"user_id"`
	PolicyVersion                 int                  `json:"policy_version"`
	CalculatedTier                CustomerTier         `json:"calculated_tier"`
	EffectiveTier                 CustomerTier         `json:"effective_tier"`
	ResolutionReason              string               `json:"resolution_reason"`
	VerifiedPaidValueCNYFen       int64                `json:"verified_paid_value_cny_fen"`
	VerifiedPaidConsumptionMicros int64                `json:"verified_paid_consumption_micros"`
	GraceExpiresAt                *time.Time           `json:"grace_expires_at,omitempty"`
	OverrideID                    *int64               `json:"override_id,omitempty"`
	Evidence                      CustomerTierEvidence `json:"evidence"`
	EvidenceHash                  string               `json:"evidence_hash"`
	EvaluatedAt                   time.Time            `json:"evaluated_at"`
}

type CustomerTierCurrent struct {
	UserID                        int64        `json:"user_id"`
	Email                         string       `json:"email,omitempty"`
	CalculatedTier                CustomerTier `json:"calculated_tier"`
	EffectiveTier                 CustomerTier `json:"effective_tier"`
	VerifiedPaidValueCNYFen       int64        `json:"verified_paid_value_cny_fen"`
	VerifiedPaidConsumptionMicros int64        `json:"verified_paid_consumption_micros"`
	GraceExpiresAt                *time.Time   `json:"grace_expires_at,omitempty"`
	OverrideID                    *int64       `json:"override_id,omitempty"`
	LastEvaluationID              int64        `json:"last_evaluation_id"`
	EvaluatedAt                   time.Time    `json:"evaluated_at"`
	ProjectionStatus              string       `json:"projection_status,omitempty"`
}

type CustomerTierHistoryItem struct {
	ID           int64         `json:"id"`
	PreviousTier *CustomerTier `json:"previous_tier,omitempty"`
	NewTier      CustomerTier  `json:"new_tier"`
	ChangeReason string        `json:"change_reason"`
	ChangedAt    time.Time     `json:"changed_at"`
}

type CustomerTierExplanation struct {
	Current    CustomerTierCurrent       `json:"current"`
	Evaluation CustomerTierEvaluation    `json:"evaluation"`
	History    []CustomerTierHistoryItem `json:"history"`
	Overrides  []CustomerTierOverride    `json:"overrides"`
	Benefit    CustomerTierBenefit       `json:"benefit"`
}

type AdminCustomerTierSnapshot struct {
	Enabled     bool                  `json:"enabled"`
	GeneratedAt time.Time             `json:"generated_at"`
	Customers   []CustomerTierCurrent `json:"customers"`
	Page        int                   `json:"page"`
	PageSize    int                   `json:"page_size"`
	Total       int64                 `json:"total"`
}

type AdminCustomerTierFilter struct {
	Page     int
	PageSize int
	Search   string
	Tier     CustomerTier
}

type CustomerTierOverrideCommand struct {
	UserID      int64
	Tier        CustomerTier
	Reason      string
	StartsAt    time.Time
	ExpiresAt   time.Time
	ActorUserID int64
}

type CustomerPaidValueRefundCommand struct {
	IdempotencyKey string
	UserID         int64
	SourceType     string
	SourceID       int64
	AmountCNYFen   int64
	Reason         string
	ActorUserID    int64
	RefundedAt     time.Time
}

type CustomerPaidValueRefund struct {
	ID              int64     `json:"id"`
	IdempotencyKey  string    `json:"idempotency_key"`
	UserID          int64     `json:"user_id"`
	SourceType      string    `json:"source_type"`
	SourceID        int64     `json:"source_id"`
	AmountCNYFen    int64     `json:"amount_cny_fen"`
	Reason          string    `json:"reason"`
	CreatedByUserID *int64    `json:"created_by_user_id,omitempty"`
	RefundedAt      time.Time `json:"refunded_at"`
	CreatedAt       time.Time `json:"created_at"`
}

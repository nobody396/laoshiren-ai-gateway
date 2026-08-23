package service

import "time"

const SettingKeyCompensationShadowDraftEnabled = "compensation_shadow_draft_enabled"

type CompensationDraft struct {
	ID                                    int64                     `json:"id"`
	SeriesID                              string                    `json:"series_id"`
	IncidentID                            int64                     `json:"incident_id"`
	ShadowPeriodID                        int64                     `json:"shadow_period_id"`
	PolicyVersion                         int                       `json:"policy_version"`
	RevisionNumber                        int                       `json:"revision_number"`
	ReplacesDraftID                       *int64                    `json:"replaces_draft_id,omitempty"`
	State                                 string                    `json:"state"`
	RevisionReason                        string                    `json:"revision_reason,omitempty"`
	Redesigned                            bool                      `json:"redesigned"`
	EligibleUserCount                     int                       `json:"eligible_user_count"`
	AffectedUserCount                     int                       `json:"affected_user_count"`
	ProposedTotalCNYFen                   int64                     `json:"proposed_total_cny_fen"`
	AffectedProductRollingPaidValueCNYFen int64                     `json:"affected_product_rolling_paid_value_cny_fen"`
	HighValueThresholdCNYFen              int64                     `json:"high_value_threshold_cny_fen"`
	HighValue                             bool                      `json:"high_value"`
	EvidenceHash                          string                    `json:"evidence_hash"`
	CreatedByUserID                       *int64                    `json:"created_by_user_id,omitempty"`
	CreatedAt                             time.Time                 `json:"created_at"`
	Users                                 []CompensationDraftUser   `json:"users,omitempty"`
	Review                                *CompensationShadowReview `json:"review,omitempty"`
}

type CompensationDraftUser struct {
	ID                            int64                   `json:"id"`
	UserID                        int64                   `json:"user_id"`
	Email                         string                  `json:"email,omitempty"`
	TierSnapshotID                *int64                  `json:"tier_snapshot_id,omitempty"`
	Tier                          CustomerTier            `json:"tier"`
	Eligible                      bool                    `json:"eligible"`
	Included                      bool                    `json:"included"`
	EvidenceComplete              bool                    `json:"evidence_complete"`
	FinalFailureCount             int                     `json:"final_failure_count"`
	FirstQualifyingFailureAt      *time.Time              `json:"first_qualifying_failure_at,omitempty"`
	VerifiedPaidValueCNYFen       int64                   `json:"verified_paid_value_cny_fen"`
	RollingGoodwillExecutedCNYFen int64                   `json:"rolling_goodwill_executed_cny_fen"`
	RawValueCNYMicros             int64                   `json:"raw_value_cny_micros"`
	TierCapCNYFen                 int64                   `json:"tier_cap_cny_fen"`
	RelationshipCapCNYFen         int64                   `json:"relationship_cap_cny_fen"`
	RollingCapCNYFen              *int64                  `json:"rolling_cap_cny_fen,omitempty"`
	RollingRemainingCNYFen        *int64                  `json:"rolling_remaining_cny_fen,omitempty"`
	ProposedTotalCNYFen           int64                   `json:"proposed_total_cny_fen"`
	BalanceBenefitCNYFen          int64                   `json:"balance_benefit_cny_fen"`
	BuilderPassBenefitCNYFen      int64                   `json:"builder_pass_benefit_cny_fen"`
	LimitingCap                   string                  `json:"limiting_cap"`
	ExclusionReason               string                  `json:"exclusion_reason,omitempty"`
	EvidenceHash                  string                  `json:"evidence_hash"`
	Items                         []CompensationDraftItem `json:"items,omitempty"`
}

type CompensationDraftItem struct {
	ID                       int64                         `json:"id"`
	UserID                   int64                         `json:"user_id"`
	ProductID                int64                         `json:"product_id"`
	ProductName              string                        `json:"product_name,omitempty"`
	GroupID                  *int64                        `json:"group_id,omitempty"`
	GroupName                string                        `json:"group_name,omitempty"`
	BenefitChannel           string                        `json:"benefit_channel"`
	FirstQualifyingFailureAt time.Time                     `json:"first_qualifying_failure_at"`
	CompensableDurationMS    int64                         `json:"compensable_duration_ms"`
	ProductRateVersionID     int64                         `json:"product_rate_version_id"`
	ProductRateCNYFenPerHour int64                         `json:"product_rate_cny_fen_per_hour"`
	GroupWeightVersionID     *int64                        `json:"group_weight_version_id,omitempty"`
	GroupWeightBPS           int64                         `json:"group_weight_bps"`
	TierMultiplierBPS        int64                         `json:"tier_multiplier_bps"`
	RawValueCNYMicros        int64                         `json:"raw_value_cny_micros"`
	ProposedCNYFen           int64                         `json:"proposed_cny_fen"`
	ExclusionReason          string                        `json:"exclusion_reason,omitempty"`
	SourceObservationIDs     []int64                       `json:"source_observation_ids,omitempty"`
	SegmentIDs               []int64                       `json:"segment_ids,omitempty"`
	SourceFacts              []CompensationEvidenceFact    `json:"source_facts,omitempty"`
	Segments                 []CompensationEvidenceSegment `json:"segments,omitempty"`
}

type CompensationEvidenceFact struct {
	ObservationID       int64     `json:"observation_id"`
	RequestID           string    `json:"request_id"`
	FactType            string    `json:"fact_type"`
	Outcome             string    `json:"outcome"`
	ErrorOwner          string    `json:"error_owner"`
	ExclusionReason     string    `json:"exclusion_reason,omitempty"`
	CustomerImpact      bool      `json:"customer_impact"`
	ProductID           int64     `json:"product_id"`
	GroupID             *int64    `json:"group_id,omitempty"`
	ObservedAt          time.Time `json:"observed_at"`
	Qualified           bool      `json:"qualified"`
	QualificationReason string    `json:"qualification_reason"`
}
type CompensationEvidenceSegment struct {
	SegmentID        int64     `json:"segment_id"`
	ProductID        int64     `json:"product_id"`
	StartedAt        time.Time `json:"started_at"`
	EndedAt          time.Time `json:"ended_at"`
	ClippedStartedAt time.Time `json:"clipped_started_at"`
	ClippedEndedAt   time.Time `json:"clipped_ended_at"`
	DurationMS       int64     `json:"duration_ms"`
}
type CompensationUserEvidenceSnapshot struct {
	User   CompensationDraftUser   `json:"user"`
	Items  []CompensationDraftItem `json:"items"`
	Paid30 CustomerTierEvidence    `json:"paid_30_day_evidence"`
	Policy CompensationPolicy      `json:"policy"`
}
type CompensationReproduction struct {
	PolicyEligible           bool  `json:"policy_eligible"`
	Included                 bool  `json:"included"`
	DistinctFinalFailures    int   `json:"distinct_final_failures"`
	RawValueCNYMicros        int64 `json:"raw_value_cny_micros"`
	ProposedTotalCNYFen      int64 `json:"proposed_total_cny_fen"`
	BalanceBenefitCNYFen     int64 `json:"balance_benefit_cny_fen"`
	BuilderPassBenefitCNYFen int64 `json:"builder_pass_benefit_cny_fen"`
}

type CompensationRevisionAdjustment struct {
	UserID         int64 `json:"user_id"`
	Included       bool  `json:"included"`
	ProposedCNYFen int64 `json:"proposed_cny_fen"`
}
type CompensationRevisionCommand struct {
	DraftID     int64
	Reason      string
	ActorUserID int64
	Adjustments []CompensationRevisionAdjustment
}

type CompensationShadowReview struct {
	ID                        int64     `json:"id"`
	DraftID                   int64     `json:"draft_id"`
	OwnerJudgementTotalCNYFen int64     `json:"owner_judgement_total_cny_fen"`
	VarianceCNYFen            int64     `json:"variance_cny_fen"`
	Result                    string    `json:"result"`
	Notes                     string    `json:"notes"`
	CreatedByUserID           *int64    `json:"created_by_user_id,omitempty"`
	CreatedAt                 time.Time `json:"created_at"`
}
type CompensationShadowReviewCommand struct {
	DraftID                   int64
	OwnerJudgementTotalCNYFen int64
	Result                    string
	Notes                     string
	ActorUserID               int64
}

type CompensationShadowAssessment struct {
	Enabled                bool       `json:"enabled"`
	StartedAt              *time.Time `json:"started_at,omitempty"`
	ElapsedDays            int        `json:"elapsed_days"`
	EvaluatedIncidentCount int        `json:"evaluated_incident_count"`
	ReviewedIncidentCount  int        `json:"reviewed_incident_count"`
	EvidenceComplete       bool       `json:"evidence_complete"`
	MinimumDays            int        `json:"minimum_days"`
	MinimumIncidents       int        `json:"minimum_incidents"`
	EligibleForOwnerReview bool       `json:"eligible_for_owner_review"`
	ExecutionAvailable     bool       `json:"execution_available"`
}

type AdminCompensationSnapshot struct {
	Enabled     bool                         `json:"enabled"`
	GeneratedAt time.Time                    `json:"generated_at"`
	Assessment  CompensationShadowAssessment `json:"assessment"`
	Drafts      []CompensationDraft          `json:"drafts"`
}

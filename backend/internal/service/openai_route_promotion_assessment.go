package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	openAIRoutePromotionInitialCheckpoint = 24 * time.Hour
	openAIRoutePromotionMinimumWindow     = 72 * time.Hour
	// Decision timestamps are selected from an end-exclusive query window and
	// real traffic is not synchronized to the exact T0/T0+72h boundaries. A
	// literal 72h MIN/MAX span is therefore impossible for a 72h query. Keep a
	// small, explicit total boundary-gap allowance while a full >=72h query
	// window and the 200-sample gate remain mandatory.
	openAIRoutePromotionMaxBoundaryGap      = time.Hour
	openAIRoutePromotionRetryInterval       = 24 * time.Hour
	openAIRoutePromotionMinimumDecisions    = int64(200)
	openAIRoutePromotionMinimumCompleteness = 0.99
	// New text sessions are the independent routing samples. Sticky follow-up
	// requests deliberately bypass a new allocation, so requiring one decision
	// in almost every wall-clock hour makes a safe experiment impossible to
	// finish even while durable passive evidence remains continuous. Require a
	// finite but broad temporal sample instead: 36 distinct hours, three Beijing
	// dates, and all four six-hour Beijing dayparts, in addition to the existing
	// 72h/200-decision/completeness/continuity gates.
	openAIRoutePromotionMinimumHourBuckets     = int64(36)
	openAIRoutePromotionMinimumBeijingDates    = int64(3)
	openAIRoutePromotionMinimumBeijingDayparts = int64(4)
)

var ErrOpenAIRouteInvalidPromotionScope = errors.New("invalid OpenAI route promotion assessment scope")

type OpenAIRoutePromotionAssessmentGate struct {
	Name     string  `json:"name"`
	Passed   bool    `json:"passed"`
	Required string  `json:"required"`
	Observed string  `json:"observed"`
	Ratio    float64 `json:"ratio,omitempty"`
	Detail   string  `json:"detail,omitempty"`
}

// OpenAIRoutePromotionReviewSchedule is a deterministic plan, not a running
// timer. The operator creates one-shot task wakeups only after a separately
// authorized Shadow policy has a real evidence start. No timestamp here can
// enable traffic or mutate a production policy.
type OpenAIRoutePromotionReviewSchedule struct {
	EvidenceStartAt      time.Time `json:"evidence_start_at"`
	InitialCheckpointAt  time.Time `json:"initial_checkpoint_at"`
	PrimaryAssessmentAt  time.Time `json:"primary_assessment_at"`
	RetryIntervalHours   float64   `json:"retry_interval_hours"`
	Timezone             string    `json:"timezone"`
	AutomaticPromotion   bool      `json:"automatic_promotion"`
	TimerActivationState string    `json:"timer_activation_state"`
}

// OpenAIRoutePromotionAssessment is intentionally read-only. Passing every
// automated evidence gate means only that an operator may consider a 1%
// canary after the remaining business and user-regression checks plus explicit
// owner approval. It never changes a policy, account or traffic allocation.
type OpenAIRoutePromotionAssessment struct {
	AssessedAt time.Time `json:"assessed_at"`
	// WindowStart/WindowEnd preserve the immutable activation lineage requested
	// by the operator. Historical observations in this window remain useful for
	// analysis even when loss-proof process epochs were introduced later.
	WindowStart time.Time `json:"window_start"`
	WindowEnd   time.Time `json:"window_end"`
	WindowHours float64   `json:"window_hours"`
	// PromotionEvidenceStart is the first instant covered by both durable audit
	// and observation epochs, bounded below by the activation T0. Promotion
	// gates use this qualified window instead of discarding older observations or
	// pretending that pre-epoch counters were durable.
	PromotionEvidenceStart time.Time               `json:"promotion_evidence_start"`
	PromotionEvidenceHours float64                 `json:"promotion_evidence_hours"`
	ObservedSpanHours      float64                 `json:"observed_span_hours"`
	GroupID                int64                   `json:"group_id"`
	Model                  string                  `json:"model"`
	RequestClass           OpenAIRouteRequestClass `json:"request_class"`
	PolicyVersion          int                     `json:"policy_version"`
	PolicyMode             OpenAIRoutePolicyMode   `json:"policy_mode"`
	ActivationID           string                  `json:"activation_id"`
	ShadowStartedAt        time.Time               `json:"shadow_started_at,omitempty"`
	Status                 string                  `json:"status"`
	AutomatedEvidenceReady bool                    `json:"automated_evidence_ready"`
	EligibleNextStage      string                  `json:"eligible_next_stage"`
	ManualApprovalRequired bool                    `json:"manual_approval_required"`
	EnforceAvailable       bool                    `json:"enforce_available"`
	// Stats is the complete activation-lineage view. PromotionEvidenceStats is
	// the strict loss-proof subset used by every automated promotion gate.
	Stats                  OpenAIRouteShadowDecisionStats       `json:"stats"`
	PromotionEvidenceStats OpenAIRouteShadowDecisionStats       `json:"promotion_evidence_stats"`
	Health                 OpenAIRouteAuditHealth               `json:"health"`
	HealthSamplingScope    string                               `json:"health_sampling_scope"`
	ReviewSchedule         OpenAIRoutePromotionReviewSchedule   `json:"review_schedule"`
	Gates                  []OpenAIRoutePromotionAssessmentGate `json:"gates"`
	Blockers               []string                             `json:"blockers"`
	ManualChecks           []string                             `json:"manual_checks"`
	Warnings               []string                             `json:"warnings"`
}

// OpenAIRoutePromotionAssessmentEngine keeps the assessment deterministic and
// independently testable. OpsService only supplies current durable evidence.
type OpenAIRoutePromotionAssessmentEngine struct{}

func NewOpenAIRoutePromotionAssessmentEngine() *OpenAIRoutePromotionAssessmentEngine {
	return &OpenAIRoutePromotionAssessmentEngine{}
}

func (s *OpsService) AssessOpenAIRouteShadowPromotion(
	ctx context.Context,
	filter *OpenAIRouteShadowDecisionFilter,
) (*OpenAIRoutePromotionAssessment, error) {
	if s == nil || s.openAIRouteAuditService == nil {
		return nil, ErrOpenAIRouteAuditUnavailable
	}
	if err := ValidateOpenAIRoutePromotionFilter(filter); err != nil {
		return nil, err
	}
	lineageHealth := s.getOpenAIRouteAuditHealthForWindow(ctx, filter.StartTime.UTC(), filter.EndTime.UTC())
	evidenceStart := deriveOpenAIRoutePromotionEvidenceStart(filter.StartTime.UTC(), lineageHealth)
	evidenceFilter := cloneOpenAIRoutePromotionFilterWithStart(filter, evidenceStart)
	health := lineageHealth
	if evidenceStart.After(filter.StartTime.UTC()) {
		health = s.getOpenAIRouteAuditHealthForWindow(ctx, evidenceStart, filter.EndTime.UTC())
	}
	evidenceStats, err := s.openAIRouteAuditService.Stats(ctx, evidenceFilter)
	if err != nil {
		return nil, err
	}
	lineageStats := evidenceStats
	if evidenceStart.After(filter.StartTime.UTC()) {
		// Query the wider lineage window last so its totals cannot lag behind the
		// qualified subset merely because a decision landed between two reads.
		lineageStats, err = s.openAIRouteAuditService.Stats(ctx, filter)
		if err != nil {
			return nil, err
		}
	}
	return NewOpenAIRoutePromotionAssessmentEngine().AssessWithLineage(
		filter,
		lineageStats,
		evidenceFilter,
		evidenceStats,
		health,
	), nil
}

func (*OpenAIRoutePromotionAssessmentEngine) Assess(
	filter *OpenAIRouteShadowDecisionFilter,
	stats *OpenAIRouteShadowDecisionStats,
	health OpenAIRouteAuditHealth,
) *OpenAIRoutePromotionAssessment {
	return buildOpenAIRoutePromotionAssessmentWithLineage(filter, stats, filter, stats, health)
}

func (*OpenAIRoutePromotionAssessmentEngine) AssessWithLineage(
	lineageFilter *OpenAIRouteShadowDecisionFilter,
	lineageStats *OpenAIRouteShadowDecisionStats,
	evidenceFilter *OpenAIRouteShadowDecisionFilter,
	evidenceStats *OpenAIRouteShadowDecisionStats,
	health OpenAIRouteAuditHealth,
) *OpenAIRoutePromotionAssessment {
	return buildOpenAIRoutePromotionAssessmentWithLineage(
		lineageFilter,
		lineageStats,
		evidenceFilter,
		evidenceStats,
		health,
	)
}

func deriveOpenAIRoutePromotionEvidenceStart(
	activationStart time.Time,
	health OpenAIRouteAuditHealth,
) time.Time {
	start := activationStart.UTC()
	if health.DurableEvidence == nil || !health.DurableEvidence.Available {
		return start
	}
	for _, candidate := range []time.Time{
		health.DurableEvidence.Audit.CounterStartedAt,
		health.DurableEvidence.Observation.CounterStartedAt,
	} {
		candidate = candidate.UTC()
		if !candidate.IsZero() && candidate.After(start) {
			start = candidate
		}
	}
	return start
}

func cloneOpenAIRoutePromotionFilterWithStart(
	filter *OpenAIRouteShadowDecisionFilter,
	start time.Time,
) *OpenAIRouteShadowDecisionFilter {
	cloned := *filter
	start = start.UTC()
	cloned.StartTime = &start
	return &cloned
}

func ValidateOpenAIRoutePromotionFilter(filter *OpenAIRouteShadowDecisionFilter) error {
	if filter == nil || filter.StartTime == nil || filter.EndTime == nil || filter.StartTime.IsZero() || filter.EndTime.IsZero() {
		return fmt.Errorf("%w: explicit start_time and end_time are required", ErrOpenAIRouteInvalidPromotionScope)
	}
	if !filter.StartTime.Before(*filter.EndTime) {
		return fmt.Errorf("%w: start_time must be before end_time", ErrOpenAIRouteInvalidPromotionScope)
	}
	if filter.GroupID == nil || *filter.GroupID <= 0 {
		return fmt.Errorf("%w: group_id is required", ErrOpenAIRouteInvalidPromotionScope)
	}
	if strings.TrimSpace(filter.Model) == "" {
		return fmt.Errorf("%w: model is required", ErrOpenAIRouteInvalidPromotionScope)
	}
	if !filter.RequestClass.Valid() {
		return fmt.Errorf("%w: request_class must be text or image", ErrOpenAIRouteInvalidPromotionScope)
	}
	if filter.PolicyVersion == nil || *filter.PolicyVersion <= 0 {
		return fmt.Errorf("%w: positive policy_version is required", ErrOpenAIRouteInvalidPromotionScope)
	}
	if activationID := strings.TrimSpace(filter.ActivationID); activationID == "" || len(activationID) > 128 {
		return fmt.Errorf("%w: activation_id is required and must not exceed 128 bytes", ErrOpenAIRouteInvalidPromotionScope)
	}
	if filter.PolicyMode != OpenAIRoutePolicyShadow {
		return fmt.Errorf("%w: policy_mode must be shadow", ErrOpenAIRouteInvalidPromotionScope)
	}
	if strings.TrimSpace(filter.Reason) != "" || strings.TrimSpace(filter.RequestID) != "" || strings.TrimSpace(filter.ClientRequestID) != "" ||
		filter.Evaluated != nil || filter.Diverged != nil || filter.Emergency != nil {
		return fmt.Errorf("%w: outcome filters are not allowed", ErrOpenAIRouteInvalidPromotionScope)
	}
	return nil
}

func buildOpenAIRoutePromotionAssessment(
	filter *OpenAIRouteShadowDecisionFilter,
	stats *OpenAIRouteShadowDecisionStats,
	health OpenAIRouteAuditHealth,
) *OpenAIRoutePromotionAssessment {
	return buildOpenAIRoutePromotionAssessmentWithLineage(filter, stats, filter, stats, health)
}

func buildOpenAIRoutePromotionAssessmentWithLineage(
	lineageFilter *OpenAIRouteShadowDecisionFilter,
	lineageStats *OpenAIRouteShadowDecisionStats,
	evidenceFilter *OpenAIRouteShadowDecisionFilter,
	evidenceStats *OpenAIRouteShadowDecisionStats,
	health OpenAIRouteAuditHealth,
) *OpenAIRoutePromotionAssessment {
	if lineageStats == nil {
		lineageStats = &OpenAIRouteShadowDecisionStats{}
	}
	if evidenceStats == nil {
		evidenceStats = &OpenAIRouteShadowDecisionStats{}
	}
	lineageStart := lineageFilter.StartTime.UTC()
	end := lineageFilter.EndTime.UTC()
	lineageWindow := end.Sub(lineageStart)
	evidenceStart := evidenceFilter.StartTime.UTC()
	evidenceWindow := end.Sub(evidenceStart)
	observedSpan := time.Duration(0)
	if !evidenceStats.FirstDecisionAt.IsZero() && !evidenceStats.LastDecisionAt.IsZero() && evidenceStats.LastDecisionAt.After(evidenceStats.FirstDecisionAt) {
		observedSpan = evidenceStats.LastDecisionAt.Sub(evidenceStats.FirstDecisionAt)
	}
	activationStart := lineageStats.ShadowStartedAt.UTC()
	reviewSchedule := OpenAIRoutePromotionReviewSchedule{
		EvidenceStartAt:      evidenceStart,
		RetryIntervalHours:   openAIRoutePromotionRetryInterval.Hours(),
		Timezone:             "Asia/Shanghai",
		AutomaticPromotion:   false,
		TimerActivationState: "not_managed_by_assessment",
	}
	if !evidenceStart.IsZero() {
		reviewSchedule.InitialCheckpointAt = evidenceStart.Add(openAIRoutePromotionInitialCheckpoint)
		reviewSchedule.PrimaryAssessmentAt = evidenceStart.Add(openAIRoutePromotionMinimumWindow)
	}

	evaluatedRatio := safeOpenAIRouteRatio(evidenceStats.Evaluated, evidenceStats.Total)
	linkedEvaluated := evidenceStats.EvaluatedLinkedSuccessfulUsage + evidenceStats.EvaluatedLinkedLegacyFailure
	linkageRatio := safeOpenAIRouteRatio(linkedEvaluated, evidenceStats.Evaluated)
	emergencyRatio := safeOpenAIRouteRatio(evidenceStats.Emergency, evidenceStats.Evaluated)
	maxAccountShare := maxOpenAIRouteSelectedAccountShare(evidenceStats.SelectedAccounts)
	maxProviderShare := maxOpenAIRouteSelectedProviderShare(evidenceStats.SelectedProviders)
	selectedAccountTotal := sumOpenAIRouteSelectedAccountCount(evidenceStats.SelectedAccounts)
	selectedProviderTotal := sumOpenAIRouteSelectedProviderCount(evidenceStats.SelectedProviders)
	selectedRouteTotal := sumOpenAIRouteSelectedRouteCount(evidenceStats.SelectedRoutes)

	assessment := &OpenAIRoutePromotionAssessment{
		AssessedAt:             time.Now().UTC(),
		WindowStart:            lineageStart,
		WindowEnd:              end,
		WindowHours:            lineageWindow.Hours(),
		PromotionEvidenceStart: evidenceStart,
		PromotionEvidenceHours: evidenceWindow.Hours(),
		ObservedSpanHours:      observedSpan.Hours(),
		GroupID:                *lineageFilter.GroupID,
		Model:                  strings.TrimSpace(lineageFilter.Model),
		RequestClass:           lineageFilter.RequestClass,
		PolicyVersion:          *lineageFilter.PolicyVersion,
		PolicyMode:             lineageFilter.PolicyMode,
		ActivationID:           strings.TrimSpace(lineageFilter.ActivationID),
		ShadowStartedAt:        activationStart,
		Status:                 "continue_shadow",
		EligibleNextStage:      "none",
		ManualApprovalRequired: true,
		EnforceAvailable:       false,
		Stats:                  *lineageStats,
		PromotionEvidenceStats: *evidenceStats,
		Health:                 health,
		HealthSamplingScope:    "process_instance_since_start_global",
		ReviewSchedule:         reviewSchedule,
		ManualChecks: []string{
			"multi-replica deployments must verify per-instance audit and observation completeness until cluster-wide counters are durable",
			"authoritative upstream billing reconciliation must show no inconsistency",
			"user-visible error and recovery rates must not regress versus a comparable legacy baseline",
			"P95 and P99 TTFT/completion latency must not regress versus a comparable legacy baseline",
			"route coverage and legacy-selection bias must be reviewed before treating passive observations as counterfactual evidence",
			"predicted route cost must be calibrated against later authoritative text settlements without using image token samples",
			"text stickiness or image stateless behavior must be verified for this request class",
			"an owner must explicitly authorize the next traffic stage",
		},
	}
	if health.DurableEvidence != nil {
		assessment.HealthSamplingScope = health.DurableEvidence.Scope
	}

	assessment.addGate("single_activation_identity", lineageStats.ActivationIDVariants == 1,
		"exactly 1 non-empty activation_id", fmt.Sprintf("%d", lineageStats.ActivationIDVariants), 0,
		"Every separately authorized Shadow enablement cycle has a new immutable identity; historical and restarted evidence cannot be mixed.")
	assessment.addGate("single_experiment_identity", lineageStats.ExperimentIDVariants == 1,
		"exactly 1 non-empty experiment_id", fmt.Sprintf("%d", lineageStats.ExperimentIDVariants), 0,
		"Parallel experiments must be assessed independently rather than pooled into one apparent sample.")
	assessment.addGate("single_variant_identity", lineageStats.VariantIDVariants == 1,
		"exactly 1 non-empty variant_id", fmt.Sprintf("%d", lineageStats.VariantIDVariants), 0,
		"Each Shadow treatment has its own statistics and promotion decision.")
	assessment.addGate("single_treatment_fingerprint", lineageStats.TreatmentFingerprintVariants == 1,
		"exactly 1 normalized treatment fingerprint", fmt.Sprintf("%d", lineageStats.TreatmentFingerprintVariants), 0,
		"Changing a policy body creates a new treatment instead of silently rewriting mature evidence.")
	assessment.addGate("single_shadow_start", lineageStats.ShadowStartedAtVariants == 1 && !activationStart.IsZero(),
		"exactly 1 non-null shadow_started_at", fmt.Sprintf("variants=%d value=%s", lineageStats.ShadowStartedAtVariants, formatOpenAIRouteEvidenceTimestamp(activationStart)), 0,
		"All rows in one activation must carry the same durable T0.")
	assessment.addGate("window_starts_at_activation", !activationStart.IsZero() && lineageStart.Equal(activationStart),
		"lineage window_start exactly equals persisted shadow_started_at", fmt.Sprintf("window=%s activation=%s", formatOpenAIRouteEvidenceTimestamp(lineageStart), formatOpenAIRouteEvidenceTimestamp(activationStart)), boolOpenAIRouteRatio(!activationStart.IsZero() && lineageStart.Equal(activationStart)),
		"Assessment windows begin at the authorized T0; later windows cannot hide early evidence gaps and earlier policy cycles cannot be included.")
	assessment.addGate("requested_window", evidenceWindow >= openAIRoutePromotionMinimumWindow,
		">=72h of loss-proof promotion evidence", fmt.Sprintf("%.2fh", evidenceWindow.Hours()), evidenceWindow.Hours()/openAIRoutePromotionMinimumWindow.Hours(),
		"Historical observations remain available for analysis, but the primary promotion review requires three full days covered by durable audit and observation epochs.")
	minimumObservedSpan := evidenceWindow - openAIRoutePromotionMaxBoundaryGap
	if floor := openAIRoutePromotionMinimumWindow - openAIRoutePromotionMaxBoundaryGap; minimumObservedSpan < floor {
		minimumObservedSpan = floor
	}
	assessment.addGate("observed_span", observedSpan >= minimumObservedSpan,
		fmt.Sprintf(">=%.2fh between first and last decision inside the %.2fh promotion window", minimumObservedSpan.Hours(), evidenceWindow.Hours()), fmt.Sprintf("%.2fh", observedSpan.Hours()), observedSpan.Hours()/minimumObservedSpan.Hours(),
		"The end-exclusive query permits at most one hour of total boundary gap, including for an extended retry window; a wide query containing only a short traffic burst is not continuous evidence.")
	expectedHourBuckets := int64(math.Ceil(evidenceWindow.Hours()))
	minimumCoveredHourBuckets := min(expectedHourBuckets-1, openAIRoutePromotionMinimumHourBuckets)
	if minimumCoveredHourBuckets < 1 {
		minimumCoveredHourBuckets = 1
	}
	assessment.addGate("hourly_coverage", evidenceStats.CoveredHourBuckets >= minimumCoveredHourBuckets,
		fmt.Sprintf(">=%d distinct one-hour buckets relative to promotion_evidence_start", minimumCoveredHourBuckets), fmt.Sprintf("%d", evidenceStats.CoveredHourBuckets), safeOpenAIRouteRatio(evidenceStats.CoveredHourBuckets, minimumCoveredHourBuckets),
		"Only evaluated independent routing decisions count. The finite hour target prevents sticky follow-ups or an ever-growing observation window from making the gate impossible, while the 72h span, Beijing-date/daypart, 200-sample and durable-epoch gates preserve temporal safety.")
	minimumBeijingDates := min(int64(math.Ceil(evidenceWindow.Hours()/24)), openAIRoutePromotionMinimumBeijingDates)
	if minimumBeijingDates < 1 {
		minimumBeijingDates = 1
	}
	assessment.addGate("beijing_date_coverage", evidenceStats.CoveredBeijingDates >= minimumBeijingDates,
		fmt.Sprintf(">=%d distinct Beijing dates with evaluated decisions", minimumBeijingDates), fmt.Sprintf("%d", evidenceStats.CoveredBeijingDates), safeOpenAIRouteRatio(evidenceStats.CoveredBeijingDates, minimumBeijingDates),
		"Independent routing evidence must span multiple Beijing business dates instead of being concentrated in one traffic burst.")
	minimumBeijingDayparts := min(int64(math.Ceil(evidenceWindow.Hours()/6)), openAIRoutePromotionMinimumBeijingDayparts)
	if minimumBeijingDayparts < 1 {
		minimumBeijingDayparts = 1
	}
	assessment.addGate("beijing_daypart_coverage", evidenceStats.CoveredBeijingDayparts >= minimumBeijingDayparts,
		fmt.Sprintf(">=%d distinct Beijing six-hour dayparts with evaluated decisions", minimumBeijingDayparts), fmt.Sprintf("%d", evidenceStats.CoveredBeijingDayparts), safeOpenAIRouteRatio(evidenceStats.CoveredBeijingDayparts, minimumBeijingDayparts),
		"The treatment must be observed across overnight, morning, afternoon and evening conditions rather than relying on one favorable period.")
	assessment.addGate("evaluated_samples", evidenceStats.Evaluated >= openAIRoutePromotionMinimumDecisions,
		">=200", fmt.Sprintf("%d", evidenceStats.Evaluated), float64(evidenceStats.Evaluated)/float64(openAIRoutePromotionMinimumDecisions),
		"Only successfully evaluated Shadow decisions count as valid samples.")
	assessment.addGate("evaluation_completeness", evaluatedRatio >= openAIRoutePromotionMinimumCompleteness,
		">=99%", formatOpenAIRoutePercent(evaluatedRatio), evaluatedRatio,
		"Unevaluated rows expose policy, runtime or audit problems and remain in the denominator.")
	assessment.addGate("outcome_linkage", linkageRatio >= openAIRoutePromotionMinimumCompleteness,
		">=99% of evaluated decisions", formatOpenAIRoutePercent(linkageRatio), linkageRatio,
		"Ambiguous and unlinked outcomes are not counted as successful linkage.")
	assessment.addGate("unambiguous_outcomes", evidenceStats.EvaluatedAmbiguousOutcome == 0,
		"0", fmt.Sprintf("%d", evidenceStats.EvaluatedAmbiguousOutcome), 0,
		"A decision linked to both success and failure cannot prove its real outcome.")
	assessment.addGate("audit_and_observation_health", health.Ready,
		"ready=true", fmt.Sprintf("ready=%t", health.Ready), boolOpenAIRouteRatio(health.Ready),
		"Decision storage and the passive observation collector must both be healthy.")
	durableReady := health.DurableEvidence != nil && health.DurableEvidence.Available && health.DurableEvidence.Ready
	durableObserved := "missing"
	if health.DurableEvidence != nil {
		durableObserved = fmt.Sprintf(
			"available=%t ready=%t audit_epochs=%d observation_epochs=%d audit_gap=%.1fs observation_gap=%.1fs audit_unclean=%d observation_unclean=%d",
			health.DurableEvidence.Available,
			health.DurableEvidence.Ready,
			health.DurableEvidence.Audit.Epochs,
			health.DurableEvidence.Observation.Epochs,
			health.DurableEvidence.Audit.MaximumGapSeconds,
			health.DurableEvidence.Observation.MaximumGapSeconds,
			health.DurableEvidence.Audit.UncleanEpochs,
			health.DurableEvidence.Observation.UncleanEpochs,
		)
	}
	assessment.addGate("durable_evidence_continuity", durableReady,
		"durable audit and observation epochs continuously cover the window with clean handoffs",
		durableObserved, boolOpenAIRouteRatio(durableReady),
		"Graceful deployments may create a new process epoch without restarting T0; stale unclean epochs, excessive gaps or lost counters remain blocking evidence.")
	healthCountersCoverWindow := !health.AuditCounterStartedAt.IsZero() &&
		!health.ObservationCounterStartedAt.IsZero() &&
		!health.AuditCounterStartedAt.After(evidenceStart) &&
		!health.ObservationCounterStartedAt.After(evidenceStart)
	assessment.addGate("health_counter_coverage", healthCountersCoverWindow,
		"audit and observation counters started at or before promotion_evidence_start",
		fmt.Sprintf("audit=%s observation=%s", formatOpenAIRouteEvidenceTimestamp(health.AuditCounterStartedAt), formatOpenAIRouteEvidenceTimestamp(health.ObservationCounterStartedAt)),
		boolOpenAIRouteRatio(healthCountersCoverWindow),
		"Durable epoch counters must cover the qualified promotion window; historical pre-epoch observations stay reusable but cannot certify collector completeness.")
	storageCheckHistoryClean := health.StorageCheckFailed == 0 && health.ObservationStorageFailed == 0
	assessment.addGate("storage_check_history", storageCheckHistoryClean,
		"0 audit and observation storage-check failures since counter start",
		fmt.Sprintf("audit=%d observation=%d", health.StorageCheckFailed, health.ObservationStorageFailed),
		boolOpenAIRouteRatio(storageCheckHistoryClean),
		"A later successful probe cannot erase an earlier interval where enabled Shadow evaluations or passive observations may have been skipped.")
	assessment.addGate("audit_completeness", health.Completeness >= openAIRoutePromotionMinimumCompleteness,
		">=99% of attempted decisions durably written", formatOpenAIRoutePercent(health.Completeness), health.Completeness,
		"The database slice cannot reveal Shadow decisions that were rejected, dropped, still queued, or failed before persistence.")
	assessment.addGate("audit_queue_drained", health.InFlight == 0,
		"0 in-flight audit writes", fmt.Sprintf("%d", health.InFlight), 0,
		"Run the read-only assessment after the bounded audit queue has drained so its evidence boundary is complete.")
	assessment.addGate("observation_completeness", health.ObservationCollectorAvailable && health.ObservationCompleteness >= openAIRoutePromotionMinimumCompleteness,
		">=99% and collector available", fmt.Sprintf("available=%t completeness=%s", health.ObservationCollectorAvailable, formatOpenAIRoutePercent(health.ObservationCompleteness)), health.ObservationCompleteness,
		"Dropped, rejected or failed passive evidence invalidates promotion readiness.")
	assessment.addGate("health_outcome_completeness", health.ObservationCollectorAvailable && health.ObservationOutcomeCompleteness >= openAIRoutePromotionMinimumCompleteness,
		">=99% of expected route health outcomes", fmt.Sprintf("available=%t completeness=%s", health.ObservationCollectorAvailable, formatOpenAIRoutePercent(health.ObservationOutcomeCompleteness)), health.ObservationOutcomeCompleteness,
		"A stored observation whose health transition was dropped can leave circuit state inconsistent with the learner evidence.")
	assessment.addGate("single_policy_snapshot", lineageStats.PolicySnapshotVariants == 1 && evidenceStats.PolicySnapshotVariants == 1,
		"exactly 1 normalized policy snapshot in both lineage and promotion windows", fmt.Sprintf("lineage=%d promotion=%d", lineageStats.PolicySnapshotVariants, evidenceStats.PolicySnapshotVariants), 0,
		"Reusing a policy_version for multiple policy bodies contaminates the evidence slice.")
	assessment.addGate("no_emergency_budget", evidenceStats.Emergency == 0,
		"0 emergency decisions", fmt.Sprintf("%d (%s)", evidenceStats.Emergency, formatOpenAIRoutePercent(emergencyRatio)), emergencyRatio,
		"A normal canary must not depend on emergency cost debt.")
	selectionCompleteness := minOpenAIRouteRatio(
		safeOpenAIRouteRatio(selectedAccountTotal, evidenceStats.Evaluated),
		minOpenAIRouteRatio(
			safeOpenAIRouteRatio(selectedProviderTotal, evidenceStats.Evaluated),
			safeOpenAIRouteRatio(selectedRouteTotal, evidenceStats.Evaluated),
		),
	)
	assessment.addGate("adaptive_selection_completeness", selectedAccountTotal == evidenceStats.Evaluated && selectedProviderTotal == evidenceStats.Evaluated && selectedRouteTotal == evidenceStats.Evaluated,
		"account, provider and route selection totals all equal evaluated decisions",
		fmt.Sprintf("evaluated=%d accounts=%d providers=%d routes=%d", evidenceStats.Evaluated, selectedAccountTotal, selectedProviderTotal, selectedRouteTotal),
		selectionCompleteness,
		"Missing adaptive account, provider or endpoint assignments can dilute concentration and route-variant evidence and must not be treated as valid evaluated evidence.")
	assessment.addGate("account_concentration", evidenceStats.PolicyMaxAccountShare > 0 && maxAccountShare <= evidenceStats.PolicyMaxAccountShare*100+1e-9,
		fmt.Sprintf("<= policy cap %s", formatOpenAIRoutePercent(evidenceStats.PolicyMaxAccountShare)), formatOpenAIRoutePercent(maxAccountShare/100), maxAccountShare/100,
		"Observed adaptive selection concentration must stay inside the audited policy cap.")
	assessment.addGate("provider_concentration", evidenceStats.PolicyMaxProviderShare > 0 && maxProviderShare <= evidenceStats.PolicyMaxProviderShare*100+1e-9,
		fmt.Sprintf("<= policy cap %s", formatOpenAIRoutePercent(evidenceStats.PolicyMaxProviderShare)), formatOpenAIRoutePercent(maxProviderShare/100), maxProviderShare/100,
		"Observed adaptive selection concentration must stay inside the audited provider cap.")

	if evidenceWindow >= openAIRoutePromotionInitialCheckpoint && evidenceWindow < openAIRoutePromotionMinimumWindow {
		assessment.Warnings = append(assessment.Warnings, "the 24-hour health checkpoint is available, but the primary review remains blocked until 72 hours")
	}
	if evidenceStart.After(lineageStart) {
		assessment.Warnings = append(assessment.Warnings,
			"historical activation observations are retained in stats; automated promotion gates use only the later loss-proof promotion_evidence_stats window")
	}
	if evidenceStats.Total == 0 {
		assessment.Warnings = append(assessment.Warnings, "no Shadow decisions matched this exact policy slice")
	}
	assessment.AutomatedEvidenceReady = len(assessment.Blockers) == 0
	if assessment.AutomatedEvidenceReady {
		assessment.Status = "automated_evidence_ready_for_manual_review"
		assessment.EligibleNextStage = "consider_1_percent_canary"
	}
	sort.Strings(assessment.Blockers)
	return assessment
}

func (a *OpenAIRoutePromotionAssessment) addGate(name string, passed bool, required, observed string, ratio float64, detail string) {
	if a == nil {
		return
	}
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 {
		ratio = 0
	}
	a.Gates = append(a.Gates, OpenAIRoutePromotionAssessmentGate{
		Name: name, Passed: passed, Required: required, Observed: observed, Ratio: ratio, Detail: detail,
	})
	if !passed {
		a.Blockers = append(a.Blockers, name)
	}
}

func safeOpenAIRouteRatio(numerator, denominator int64) float64 {
	if denominator <= 0 || numerator <= 0 {
		return 0
	}
	return math.Min(1, float64(numerator)/float64(denominator))
}

func boolOpenAIRouteRatio(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func formatOpenAIRoutePercent(ratio float64) string {
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio < 0 {
		ratio = 0
	}
	return fmt.Sprintf("%.2f%%", ratio*100)
}

func formatOpenAIRouteEvidenceTimestamp(value time.Time) string {
	if value.IsZero() {
		return "missing"
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func maxOpenAIRouteSelectedAccountShare(values []OpenAIRouteShadowSelectedAccountStats) float64 {
	var max float64
	for _, value := range values {
		if value.SelectedPercent > max {
			max = value.SelectedPercent
		}
	}
	return max
}

func maxOpenAIRouteSelectedProviderShare(values []OpenAIRouteShadowSelectedProviderStats) float64 {
	var max float64
	for _, value := range values {
		if value.SelectedPercent > max {
			max = value.SelectedPercent
		}
	}
	return max
}

func sumOpenAIRouteSelectedAccountCount(values []OpenAIRouteShadowSelectedAccountStats) int64 {
	var total int64
	for _, value := range values {
		if value.SelectedCount > 0 {
			total += value.SelectedCount
		}
	}
	return total
}

func sumOpenAIRouteSelectedProviderCount(values []OpenAIRouteShadowSelectedProviderStats) int64 {
	var total int64
	for _, value := range values {
		if value.SelectedCount > 0 {
			total += value.SelectedCount
		}
	}
	return total
}

func sumOpenAIRouteSelectedRouteCount(values []OpenAIRouteShadowSelectedRouteStats) int64 {
	var total int64
	for _, value := range values {
		if value.SelectedCount > 0 {
			total += value.SelectedCount
		}
	}
	return total
}

func minOpenAIRouteRatio(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

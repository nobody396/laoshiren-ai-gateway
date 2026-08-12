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
	AssessedAt             time.Time                            `json:"assessed_at"`
	WindowStart            time.Time                            `json:"window_start"`
	WindowEnd              time.Time                            `json:"window_end"`
	WindowHours            float64                              `json:"window_hours"`
	ObservedSpanHours      float64                              `json:"observed_span_hours"`
	GroupID                int64                                `json:"group_id"`
	Model                  string                               `json:"model"`
	RequestClass           OpenAIRouteRequestClass              `json:"request_class"`
	PolicyVersion          int                                  `json:"policy_version"`
	PolicyMode             OpenAIRoutePolicyMode                `json:"policy_mode"`
	Status                 string                               `json:"status"`
	AutomatedEvidenceReady bool                                 `json:"automated_evidence_ready"`
	EligibleNextStage      string                               `json:"eligible_next_stage"`
	ManualApprovalRequired bool                                 `json:"manual_approval_required"`
	EnforceAvailable       bool                                 `json:"enforce_available"`
	Stats                  OpenAIRouteShadowDecisionStats       `json:"stats"`
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
	stats, err := s.openAIRouteAuditService.Stats(ctx, filter)
	if err != nil {
		return nil, err
	}
	health := s.GetOpenAIRouteAuditHealth(ctx)
	return NewOpenAIRoutePromotionAssessmentEngine().Assess(filter, stats, health), nil
}

func (*OpenAIRoutePromotionAssessmentEngine) Assess(
	filter *OpenAIRouteShadowDecisionFilter,
	stats *OpenAIRouteShadowDecisionStats,
	health OpenAIRouteAuditHealth,
) *OpenAIRoutePromotionAssessment {
	return buildOpenAIRoutePromotionAssessment(filter, stats, health)
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
	if stats == nil {
		stats = &OpenAIRouteShadowDecisionStats{}
	}
	start := filter.StartTime.UTC()
	end := filter.EndTime.UTC()
	window := end.Sub(start)
	observedSpan := time.Duration(0)
	if !stats.FirstDecisionAt.IsZero() && !stats.LastDecisionAt.IsZero() && stats.LastDecisionAt.After(stats.FirstDecisionAt) {
		observedSpan = stats.LastDecisionAt.Sub(stats.FirstDecisionAt)
	}
	evidenceStart := start
	if !stats.FirstDecisionAt.IsZero() && !stats.FirstDecisionAt.Before(start) && stats.FirstDecisionAt.Before(end) {
		evidenceStart = stats.FirstDecisionAt.UTC()
	}

	evaluatedRatio := safeOpenAIRouteRatio(stats.Evaluated, stats.Total)
	linkedEvaluated := stats.EvaluatedLinkedSuccessfulUsage + stats.EvaluatedLinkedLegacyFailure
	linkageRatio := safeOpenAIRouteRatio(linkedEvaluated, stats.Evaluated)
	emergencyRatio := safeOpenAIRouteRatio(stats.Emergency, stats.Evaluated)
	maxAccountShare := maxOpenAIRouteSelectedAccountShare(stats.SelectedAccounts)
	maxProviderShare := maxOpenAIRouteSelectedProviderShare(stats.SelectedProviders)
	selectedAccountTotal := sumOpenAIRouteSelectedAccountCount(stats.SelectedAccounts)
	selectedProviderTotal := sumOpenAIRouteSelectedProviderCount(stats.SelectedProviders)

	assessment := &OpenAIRoutePromotionAssessment{
		AssessedAt:             time.Now().UTC(),
		WindowStart:            start,
		WindowEnd:              end,
		WindowHours:            window.Hours(),
		ObservedSpanHours:      observedSpan.Hours(),
		GroupID:                *filter.GroupID,
		Model:                  strings.TrimSpace(filter.Model),
		RequestClass:           filter.RequestClass,
		PolicyVersion:          *filter.PolicyVersion,
		PolicyMode:             filter.PolicyMode,
		Status:                 "continue_shadow",
		EligibleNextStage:      "none",
		ManualApprovalRequired: true,
		EnforceAvailable:       false,
		Stats:                  *stats,
		Health:                 health,
		HealthSamplingScope:    "process_instance_since_start_global",
		ReviewSchedule: OpenAIRoutePromotionReviewSchedule{
			EvidenceStartAt:      evidenceStart,
			InitialCheckpointAt:  evidenceStart.Add(openAIRoutePromotionInitialCheckpoint),
			PrimaryAssessmentAt:  evidenceStart.Add(openAIRoutePromotionMinimumWindow),
			RetryIntervalHours:   openAIRoutePromotionRetryInterval.Hours(),
			Timezone:             "Asia/Shanghai",
			AutomaticPromotion:   false,
			TimerActivationState: "not_managed_by_assessment",
		},
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

	assessment.addGate("requested_window", window >= openAIRoutePromotionMinimumWindow,
		">=72h", fmt.Sprintf("%.2fh", window.Hours()), window.Hours()/openAIRoutePromotionMinimumWindow.Hours(),
		"The primary promotion review requires a full three-day Shadow window; 24 hours is only an early health checkpoint.")
	minimumObservedSpan := window - openAIRoutePromotionMaxBoundaryGap
	if floor := openAIRoutePromotionMinimumWindow - openAIRoutePromotionMaxBoundaryGap; minimumObservedSpan < floor {
		minimumObservedSpan = floor
	}
	assessment.addGate("observed_span", observedSpan >= minimumObservedSpan,
		fmt.Sprintf(">=%.2fh between first and last decision inside the %.2fh window", minimumObservedSpan.Hours(), window.Hours()), fmt.Sprintf("%.2fh", observedSpan.Hours()), observedSpan.Hours()/minimumObservedSpan.Hours(),
		"The end-exclusive query permits at most one hour of total boundary gap, including for an extended retry window; a wide query containing only a short traffic burst is not continuous evidence.")
	assessment.addGate("evaluated_samples", stats.Evaluated >= openAIRoutePromotionMinimumDecisions,
		">=200", fmt.Sprintf("%d", stats.Evaluated), float64(stats.Evaluated)/float64(openAIRoutePromotionMinimumDecisions),
		"Only successfully evaluated Shadow decisions count as valid samples.")
	assessment.addGate("evaluation_completeness", evaluatedRatio >= openAIRoutePromotionMinimumCompleteness,
		">=99%", formatOpenAIRoutePercent(evaluatedRatio), evaluatedRatio,
		"Unevaluated rows expose policy, runtime or audit problems and remain in the denominator.")
	assessment.addGate("outcome_linkage", linkageRatio >= openAIRoutePromotionMinimumCompleteness,
		">=99% of evaluated decisions", formatOpenAIRoutePercent(linkageRatio), linkageRatio,
		"Ambiguous and unlinked outcomes are not counted as successful linkage.")
	assessment.addGate("unambiguous_outcomes", stats.EvaluatedAmbiguousOutcome == 0,
		"0", fmt.Sprintf("%d", stats.EvaluatedAmbiguousOutcome), 0,
		"A decision linked to both success and failure cannot prove its real outcome.")
	assessment.addGate("audit_and_observation_health", health.Ready,
		"ready=true", fmt.Sprintf("ready=%t", health.Ready), boolOpenAIRouteRatio(health.Ready),
		"Decision storage and the passive observation collector must both be healthy.")
	healthCountersCoverWindow := !health.AuditCounterStartedAt.IsZero() &&
		!health.ObservationCounterStartedAt.IsZero() &&
		!health.AuditCounterStartedAt.After(start) &&
		!health.ObservationCounterStartedAt.After(start)
	assessment.addGate("health_counter_coverage", healthCountersCoverWindow,
		"audit and observation counters started at or before window_start",
		fmt.Sprintf("audit=%s observation=%s", formatOpenAIRouteEvidenceTimestamp(health.AuditCounterStartedAt), formatOpenAIRouteEvidenceTimestamp(health.ObservationCounterStartedAt)),
		boolOpenAIRouteRatio(healthCountersCoverWindow),
		"Completeness counters are process-local; a restart after T0 invalidates the current Shadow evidence slice instead of resetting loss history to a misleading 100%.")
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
	assessment.addGate("single_policy_snapshot", stats.PolicySnapshotVariants == 1,
		"exactly 1 normalized policy snapshot", fmt.Sprintf("%d", stats.PolicySnapshotVariants), 0,
		"Reusing a policy_version for multiple policy bodies contaminates the evidence slice.")
	assessment.addGate("no_emergency_budget", stats.Emergency == 0,
		"0 emergency decisions", fmt.Sprintf("%d (%s)", stats.Emergency, formatOpenAIRoutePercent(emergencyRatio)), emergencyRatio,
		"A normal canary must not depend on emergency cost debt.")
	assessment.addGate("adaptive_selection_completeness", selectedAccountTotal == stats.Evaluated && selectedProviderTotal == stats.Evaluated,
		"account and provider selection totals both equal evaluated decisions",
		fmt.Sprintf("evaluated=%d accounts=%d providers=%d", stats.Evaluated, selectedAccountTotal, selectedProviderTotal),
		minOpenAIRouteRatio(safeOpenAIRouteRatio(selectedAccountTotal, stats.Evaluated), safeOpenAIRouteRatio(selectedProviderTotal, stats.Evaluated)),
		"Missing adaptive account or provider assignments can dilute concentration percentages and must not be treated as valid evaluated evidence.")
	assessment.addGate("account_concentration", stats.PolicyMaxAccountShare > 0 && maxAccountShare <= stats.PolicyMaxAccountShare*100+1e-9,
		fmt.Sprintf("<= policy cap %s", formatOpenAIRoutePercent(stats.PolicyMaxAccountShare)), formatOpenAIRoutePercent(maxAccountShare/100), maxAccountShare/100,
		"Observed adaptive selection concentration must stay inside the audited policy cap.")
	assessment.addGate("provider_concentration", stats.PolicyMaxProviderShare > 0 && maxProviderShare <= stats.PolicyMaxProviderShare*100+1e-9,
		fmt.Sprintf("<= policy cap %s", formatOpenAIRoutePercent(stats.PolicyMaxProviderShare)), formatOpenAIRoutePercent(maxProviderShare/100), maxProviderShare/100,
		"Observed adaptive selection concentration must stay inside the audited provider cap.")

	if window >= openAIRoutePromotionInitialCheckpoint && window < openAIRoutePromotionMinimumWindow {
		assessment.Warnings = append(assessment.Warnings, "the 24-hour health checkpoint is available, but the primary review remains blocked until 72 hours")
	}
	if stats.Total == 0 {
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

func minOpenAIRouteRatio(left, right float64) float64 {
	if left < right {
		return left
	}
	return right
}

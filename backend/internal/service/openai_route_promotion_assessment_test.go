package service

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testOpenAIRoutePromotionFilter(start, end time.Time) *OpenAIRouteShadowDecisionFilter {
	groupID := int64(7)
	version := 4
	return &OpenAIRouteShadowDecisionFilter{
		StartTime:     &start,
		EndTime:       &end,
		GroupID:       &groupID,
		Model:         "gpt-5.6-sol",
		RequestClass:  OpenAIRouteRequestClassText,
		PolicyMode:    OpenAIRoutePolicyShadow,
		PolicyVersion: &version,
		ActivationID:  "activation-20260809-001",
	}
}

func healthyOpenAIRoutePromotionEvidence(start, end time.Time) (*OpenAIRouteShadowDecisionStats, OpenAIRouteAuditHealth) {
	stats := &OpenAIRouteShadowDecisionStats{
		Total:                          200,
		Evaluated:                      200,
		LinkedSuccessfulUsage:          196,
		LinkedLegacyFailure:            4,
		EvaluatedLinkedSuccessfulUsage: 196,
		EvaluatedLinkedLegacyFailure:   4,
		PolicySnapshotVariants:         1,
		ActivationIDVariants:           1,
		ShadowStartedAtVariants:        1,
		ExperimentIDVariants:           1,
		VariantIDVariants:              1,
		TreatmentFingerprintVariants:   1,
		ShadowStartedAt:                start,
		PolicyMaxAccountShare:          0.80,
		PolicyMaxProviderShare:         0.90,
		CoveredHourBuckets:             int64(math.Ceil(end.Sub(start).Hours())),
		CoveredBeijingDates:            3,
		CoveredBeijingDayparts:         4,
		// Production stats come from an end-exclusive SQL window, so the first
		// and last decisions cannot be assumed to land exactly on its edges.
		FirstDecisionAt: start.Add(30 * time.Second),
		LastDecisionAt:  end.Add(-30 * time.Second),
		SelectedAccounts: []OpenAIRouteShadowSelectedAccountStats{
			{AccountID: 23, SelectedCount: 120, SelectedPercent: 60},
			{AccountID: 28, SelectedCount: 80, SelectedPercent: 40},
		},
		SelectedProviders: []OpenAIRouteShadowSelectedProviderStats{
			{ProviderKey: "pomelo-hk", SelectedCount: 120, SelectedPercent: 60},
			{ProviderKey: "morecode", SelectedCount: 80, SelectedPercent: 40},
		},
		SelectedRoutes: []OpenAIRouteShadowSelectedRouteStats{
			{AccountID: 23, EndpointHash: "pomo-hk", FailureDomain: "pomelo-hk", SelectedCount: 120, SelectedPercent: 60},
			{AccountID: 28, EndpointHash: "morecode", FailureDomain: "morecode", SelectedCount: 80, SelectedPercent: 40},
		},
	}
	health := OpenAIRouteAuditHealth{
		Ready:                          true,
		AuditCounterStartedAt:          start.Add(-time.Hour),
		Completeness:                   1,
		ObservationCollectorAvailable:  true,
		ObservationReady:               true,
		ObservationCounterStartedAt:    start.Add(-time.Hour),
		ObservationCompleteness:        1,
		ObservationOutcomeCompleteness: 1,
		DurableEvidence: &OpenAIRouteEvidenceWindowHealth{
			Available: true,
			Ready:     true,
			Scope:     "durable_process_epochs_overlap_conservative",
			Audit: OpenAIRouteEvidenceComponentHealth{
				Ready: true,
			},
			Observation: OpenAIRouteEvidenceComponentHealth{
				Ready: true,
			},
		},
	}
	return stats, health
}

func TestBuildOpenAIRoutePromotionAssessmentBlocksCounterResetInsideWindow(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)
	health.AuditCounterStartedAt = start.Add(time.Second)

	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)

	require.Contains(t, assessment.Blockers, "health_counter_coverage")
}

func TestBuildOpenAIRoutePromotionAssessmentKeepsReliabilityTreatmentNoGoUntilIsolated(t *testing.T) {
	end := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)
	// Legacy-observation and Reliability-Evidence treatments must never be
	// pooled into one promotion window even when all aggregate SLOs are green.
	stats.TreatmentFingerprintVariants = 2

	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)

	require.False(t, assessment.AutomatedEvidenceReady)
	require.Contains(t, assessment.Blockers, "single_treatment_fingerprint")
}

func TestBuildOpenAIRoutePromotionAssessmentBlocksRecoveredStorageGap(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)
	health.StorageCheckFailed = 1
	health.ObservationStorageFailed = 2

	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)

	require.Contains(t, assessment.Blockers, "storage_check_history")
}

func TestBuildOpenAIRoutePromotionAssessmentBlocksMissingAdaptiveAssignment(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)
	stats.SelectedAccounts[0].SelectedCount--

	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)

	require.Contains(t, assessment.Blockers, "adaptive_selection_completeness")

	stats, health = healthyOpenAIRoutePromotionEvidence(start, end)
	stats.SelectedRoutes[0].SelectedCount--
	assessment = buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.Contains(t, assessment.Blockers, "adaptive_selection_completeness")
}

func TestBuildOpenAIRoutePromotionAssessmentAllowsOnlyBoundedEdgeGap(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)

	stats.FirstDecisionAt = start.Add(30 * time.Minute)
	stats.LastDecisionAt = end.Add(-30 * time.Minute)
	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "observed_span")

	stats.LastDecisionAt = end.Add(-30*time.Minute - time.Second)
	assessment = buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.Contains(t, assessment.Blockers, "observed_span")
}

func TestBuildOpenAIRoutePromotionAssessmentKeepsFiniteSpanAndRequiresFreshEvidenceInWiderWindow(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-96 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)

	// A 71.5-hour observed span and a decision exactly 24 hours old satisfy the
	// finite primary-span and freshness gates even though sampling was extended.
	stats.LastDecisionAt = start.Add(72 * time.Hour)
	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "observed_span")
	require.NotContains(t, assessment.Blockers, "latest_decision_freshness")

	// Stale evidence remains blocked without making the historical span target
	// grow with every retry window.
	stats.LastDecisionAt = end.Add(-24*time.Hour - time.Second)
	assessment = buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "observed_span")
	require.Contains(t, assessment.Blockers, "latest_decision_freshness")

	stats.LastDecisionAt = end.Add(-23 * time.Hour)
	assessment = buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "observed_span")
	require.NotContains(t, assessment.Blockers, "latest_decision_freshness")
}

func TestBuildOpenAIRoutePromotionAssessmentClassifiesBoundedNoCandidateAsAbstention(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)

	stats.Total = 205
	stats.NotEvaluated = 5
	stats.NoCandidateAbstentions = 5
	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "evaluation_completeness")
	require.NotContains(t, assessment.Blockers, "no_candidate_abstention_rate")

	stats.Total = 211
	stats.NotEvaluated = 11
	stats.NoCandidateAbstentions = 11
	assessment = buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "evaluation_completeness")
	require.Contains(t, assessment.Blockers, "no_candidate_abstention_rate")

	// A timeout is not an intentional abstention and still fails completeness.
	stats.NoCandidateAbstentions = 8
	assessment = buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.Contains(t, assessment.Blockers, "evaluation_completeness")
}

func TestBuildOpenAIRoutePromotionAssessmentRequiresFiniteStratifiedTimeCoverage(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)

	// First/last timestamps alone can be faked by two bursts at the edges. The
	// treatment still needs broad independent-decision coverage, but the target
	// is finite because sticky follow-ups are not new routing opportunities.
	stats.CoveredHourBuckets = 2
	stats.CoveredBeijingDates = 1
	stats.CoveredBeijingDayparts = 1
	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "observed_span")
	require.Contains(t, assessment.Blockers, "hourly_coverage")
	require.Contains(t, assessment.Blockers, "beijing_date_coverage")
	require.Contains(t, assessment.Blockers, "beijing_daypart_coverage")

	stats.CoveredHourBuckets = 36
	stats.CoveredBeijingDates = 3
	stats.CoveredBeijingDayparts = 4
	assessment = buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "hourly_coverage")
	require.NotContains(t, assessment.Blockers, "beijing_date_coverage")
	require.NotContains(t, assessment.Blockers, "beijing_daypart_coverage")

	// Extending a low-volume Shadow window to collect 200 independent decisions
	// must not move the hour target forever. Span, date and daypart gates still
	// cover the wider interval.
	wideEnd := end.Add(7 * 24 * time.Hour)
	wideFilter := testOpenAIRoutePromotionFilter(start, wideEnd)
	stats.LastDecisionAt = wideEnd.Add(-30 * time.Second)
	assessment = buildOpenAIRoutePromotionAssessment(wideFilter, stats, health)
	require.NotContains(t, assessment.Blockers, "hourly_coverage")
}

func TestBuildOpenAIRoutePromotionAssessmentRequiresOneActivationAndExactT0(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)

	stats.ActivationIDVariants = 2
	stats.ShadowStartedAtVariants = 2
	stats.ShadowStartedAt = start.Add(time.Hour)
	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.Contains(t, assessment.Blockers, "single_activation_identity")
	require.Contains(t, assessment.Blockers, "single_shadow_start")
	require.Contains(t, assessment.Blockers, "window_starts_at_activation")

	stats.ActivationIDVariants = 1
	stats.ShadowStartedAtVariants = 1
	stats.ShadowStartedAt = start
	assessment = buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "single_activation_identity")
	require.NotContains(t, assessment.Blockers, "single_shadow_start")
	require.NotContains(t, assessment.Blockers, "window_starts_at_activation")
}

func TestBuildOpenAIRoutePromotionAssessmentReadyOnlyForManualReview(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)
	evidenceStart := stats.ShadowStartedAt

	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)

	require.True(t, assessment.AutomatedEvidenceReady)
	require.Equal(t, "automated_evidence_ready_for_manual_review", assessment.Status)
	require.Equal(t, "consider_1_percent_canary", assessment.EligibleNextStage)
	require.True(t, assessment.ManualApprovalRequired)
	require.False(t, assessment.EnforceAvailable)
	require.Equal(t, "durable_process_epochs_overlap_conservative", assessment.HealthSamplingScope)
	require.Equal(t, evidenceStart, assessment.ReviewSchedule.EvidenceStartAt)
	require.Equal(t, evidenceStart.Add(24*time.Hour), assessment.ReviewSchedule.InitialCheckpointAt)
	require.Equal(t, evidenceStart.Add(72*time.Hour), assessment.ReviewSchedule.PrimaryAssessmentAt)
	require.Equal(t, float64(24), assessment.ReviewSchedule.RetryIntervalHours)
	require.Equal(t, "Asia/Shanghai", assessment.ReviewSchedule.Timezone)
	require.False(t, assessment.ReviewSchedule.AutomaticPromotion)
	require.Equal(t, "not_managed_by_assessment", assessment.ReviewSchedule.TimerActivationState)
	require.Empty(t, assessment.Blockers)
	require.NotEmpty(t, assessment.ManualChecks)
	for _, gate := range assessment.Gates {
		require.True(t, gate.Passed, gate.Name)
	}
}

func TestBuildOpenAIRoutePromotionAssessmentPreservesLineageButQualifiesPromotionWindow(t *testing.T) {
	activationStart := time.Date(2026, 8, 16, 1, 45, 44, 0, time.UTC)
	evidenceStart := activationStart.Add(6 * time.Hour)
	end := evidenceStart.Add(72 * time.Hour)
	lineageFilter := testOpenAIRoutePromotionFilter(activationStart, end)
	evidenceFilter := testOpenAIRoutePromotionFilter(evidenceStart, end)
	lineageStats, _ := healthyOpenAIRoutePromotionEvidence(activationStart, end)
	lineageStats.Total = 260
	lineageStats.Evaluated = 260
	evidenceStats, health := healthyOpenAIRoutePromotionEvidence(evidenceStart, end)
	// Every decision keeps the immutable activation T0 even though the strict
	// collector-continuity subset begins at the first durable process epoch.
	evidenceStats.ShadowStartedAt = activationStart

	assessment := buildOpenAIRoutePromotionAssessmentWithLineage(
		lineageFilter,
		lineageStats,
		evidenceFilter,
		evidenceStats,
		health,
	)

	require.True(t, assessment.AutomatedEvidenceReady)
	require.Equal(t, activationStart, assessment.WindowStart)
	require.Equal(t, float64(78), assessment.WindowHours)
	require.Equal(t, evidenceStart, assessment.PromotionEvidenceStart)
	require.Equal(t, float64(72), assessment.PromotionEvidenceHours)
	require.Equal(t, int64(260), assessment.Stats.Total)
	require.Equal(t, int64(200), assessment.PromotionEvidenceStats.Total)
	require.Equal(t, activationStart, assessment.ShadowStartedAt)
	require.Equal(t, evidenceStart, assessment.ReviewSchedule.EvidenceStartAt)
	require.NotContains(t, assessment.Blockers, "window_starts_at_activation")
	require.NotContains(t, assessment.Blockers, "requested_window")
	require.Contains(t, assessment.Warnings,
		"historical activation observations are retained in stats; automated promotion gates use only the later loss-proof promotion_evidence_stats window")
}

func TestBuildOpenAIRoutePromotionAssessmentDoesNotUseHistoricalHoursToSatisfyPromotionWindow(t *testing.T) {
	activationStart := time.Date(2026, 8, 16, 1, 45, 44, 0, time.UTC)
	evidenceStart := activationStart.Add(12 * time.Hour)
	end := activationStart.Add(78 * time.Hour)
	lineageFilter := testOpenAIRoutePromotionFilter(activationStart, end)
	evidenceFilter := testOpenAIRoutePromotionFilter(evidenceStart, end)
	lineageStats, _ := healthyOpenAIRoutePromotionEvidence(activationStart, end)
	evidenceStats, health := healthyOpenAIRoutePromotionEvidence(evidenceStart, end)
	evidenceStats.ShadowStartedAt = activationStart

	assessment := buildOpenAIRoutePromotionAssessmentWithLineage(
		lineageFilter,
		lineageStats,
		evidenceFilter,
		evidenceStats,
		health,
	)

	require.Equal(t, float64(78), assessment.WindowHours)
	require.Equal(t, float64(66), assessment.PromotionEvidenceHours)
	require.Contains(t, assessment.Blockers, "requested_window")
	require.False(t, assessment.AutomatedEvidenceReady)
}

func TestDeriveOpenAIRoutePromotionEvidenceStartIsOneTimeAndMonotonic(t *testing.T) {
	activationStart := time.Date(2026, 8, 16, 1, 45, 44, 0, time.UTC)
	auditStart := activationStart.Add(6 * time.Hour)
	observationStart := auditStart.Add(time.Microsecond)
	health := OpenAIRouteAuditHealth{DurableEvidence: &OpenAIRouteEvidenceWindowHealth{
		Available: true,
		Audit: OpenAIRouteEvidenceComponentHealth{
			CounterStartedAt: auditStart,
		},
		Observation: OpenAIRouteEvidenceComponentHealth{
			CounterStartedAt: observationStart,
		},
	}}

	require.Equal(t, observationStart, deriveOpenAIRoutePromotionEvidenceStart(activationStart, health))
	// A later activation is not pulled backwards into old process evidence.
	newActivation := observationStart.Add(24 * time.Hour)
	require.Equal(t, newActivation, deriveOpenAIRoutePromotionEvidenceStart(newActivation, health))
	// Missing durable epochs never invent a qualified start.
	require.Equal(t, activationStart, deriveOpenAIRoutePromotionEvidenceStart(activationStart, OpenAIRouteAuditHealth{}))
}

func TestBuildOpenAIRoutePromotionAssessmentTreats24HoursAsCheckpointOnly(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-24 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)

	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)

	require.False(t, assessment.AutomatedEvidenceReady)
	require.Equal(t, "continue_shadow", assessment.Status)
	require.Equal(t, "none", assessment.EligibleNextStage)
	require.Contains(t, assessment.Blockers, "requested_window")
	require.Contains(t, assessment.Blockers, "observed_span")
	require.Contains(t, assessment.Warnings, "the 24-hour health checkpoint is available, but the primary review remains blocked until 72 hours")
	require.True(t, assessment.ManualApprovalRequired)
	require.False(t, assessment.EnforceAvailable)
}

func TestBuildOpenAIRoutePromotionAssessmentBlocksWeakEvidence(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)
	stats.Total = 205
	stats.Evaluated = 200
	stats.EvaluatedLinkedSuccessfulUsage = 197
	stats.EvaluatedLinkedLegacyFailure = 0
	stats.EvaluatedAmbiguousOutcome = 1
	stats.EvaluatedUnlinkedOutcome = 2
	stats.PolicySnapshotVariants = 2
	stats.Emergency = 1
	stats.FirstDecisionAt = end.Add(-time.Hour)
	stats.SelectedAccounts[0].SelectedPercent = 81
	stats.SelectedProviders[0].SelectedPercent = 91
	health.Ready = false
	health.Completeness = 0.98
	health.InFlight = 1
	health.ObservationCompleteness = 0.98
	health.ObservationOutcomeCompleteness = 0.97

	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)

	require.False(t, assessment.AutomatedEvidenceReady)
	require.Equal(t, "continue_shadow", assessment.Status)
	require.Equal(t, "none", assessment.EligibleNextStage)
	for _, blocker := range []string{
		"account_concentration", "audit_and_observation_health", "audit_completeness", "audit_queue_drained", "evaluation_completeness",
		"health_outcome_completeness",
		"no_emergency_budget", "observation_completeness", "observed_span", "outcome_linkage",
		"provider_concentration", "single_policy_snapshot", "unambiguous_outcomes",
	} {
		require.Contains(t, assessment.Blockers, blocker)
	}
}

func TestValidateOpenAIRoutePromotionFilterRequiresExactUnbiasedSlice(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-24 * time.Hour)
	require.NoError(t, ValidateOpenAIRoutePromotionFilter(testOpenAIRoutePromotionFilter(start, end)))

	tests := map[string]func(*OpenAIRouteShadowDecisionFilter){
		"group":          func(f *OpenAIRouteShadowDecisionFilter) { f.GroupID = nil },
		"model":          func(f *OpenAIRouteShadowDecisionFilter) { f.Model = "" },
		"class":          func(f *OpenAIRouteShadowDecisionFilter) { f.RequestClass = OpenAIRouteRequestClassUnknown },
		"version":        func(f *OpenAIRouteShadowDecisionFilter) { f.PolicyVersion = nil },
		"activation":     func(f *OpenAIRouteShadowDecisionFilter) { f.ActivationID = "" },
		"mode":           func(f *OpenAIRouteShadowDecisionFilter) { f.PolicyMode = OpenAIRoutePolicyLegacy },
		"outcome_filter": func(f *OpenAIRouteShadowDecisionFilter) { value := true; f.Evaluated = &value },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			filter := testOpenAIRoutePromotionFilter(start, end)
			mutate(filter)
			require.ErrorIs(t, ValidateOpenAIRoutePromotionFilter(filter), ErrOpenAIRouteInvalidPromotionScope)
		})
	}
}

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
		ShadowStartedAt:                start,
		PolicyMaxAccountShare:          0.80,
		PolicyMaxProviderShare:         0.90,
		CoveredHourBuckets:             int64(math.Ceil(end.Sub(start).Hours())),
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

func TestBuildOpenAIRoutePromotionAssessmentDoesNotDiluteWithWiderWindow(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-96 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)

	stats.LastDecisionAt = start.Add(72 * time.Hour)
	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.Contains(t, assessment.Blockers, "observed_span")

	stats.LastDecisionAt = end.Add(-30 * time.Minute)
	assessment = buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "observed_span")
}

func TestBuildOpenAIRoutePromotionAssessmentRequiresContinuousHourlyCoverage(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)

	// First/last timestamps alone can be faked by two bursts at the edges.
	// Promotion therefore requires evaluated evidence in almost every relative
	// hour bucket across the complete slice.
	stats.CoveredHourBuckets = 2
	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)
	require.NotContains(t, assessment.Blockers, "observed_span")
	require.Contains(t, assessment.Blockers, "hourly_coverage")

	stats.CoveredHourBuckets = 71
	assessment = buildOpenAIRoutePromotionAssessment(filter, stats, health)
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
	require.Equal(t, "process_instance_since_start_global", assessment.HealthSamplingScope)
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

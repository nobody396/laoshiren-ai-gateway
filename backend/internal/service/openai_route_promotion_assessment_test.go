package service

import (
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
		PolicyMaxAccountShare:          0.80,
		PolicyMaxProviderShare:         0.90,
		FirstDecisionAt:                start,
		LastDecisionAt:                 end,
		SelectedAccounts: []OpenAIRouteShadowSelectedAccountStats{
			{AccountID: 23, SelectedCount: 120, SelectedPercent: 60},
			{AccountID: 28, SelectedCount: 80, SelectedPercent: 40},
		},
		SelectedProviders: []OpenAIRouteShadowSelectedProviderStats{
			{ProviderKey: "pomelo-hk", SelectedCount: 120, SelectedPercent: 60},
			{ProviderKey: "morecode", SelectedCount: 80, SelectedPercent: 40},
		},
	}
	health := OpenAIRouteAuditHealth{
		Ready:                         true,
		ObservationCollectorAvailable: true,
		ObservationReady:              true,
		ObservationCompleteness:       1,
	}
	return stats, health
}

func TestBuildOpenAIRoutePromotionAssessmentReadyOnlyForManualReview(t *testing.T) {
	end := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	start := end.Add(-72 * time.Hour)
	filter := testOpenAIRoutePromotionFilter(start, end)
	stats, health := healthyOpenAIRoutePromotionEvidence(start, end)

	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)

	require.True(t, assessment.AutomatedEvidenceReady)
	require.Equal(t, "automated_evidence_ready_for_manual_review", assessment.Status)
	require.Equal(t, "consider_1_percent_canary", assessment.EligibleNextStage)
	require.True(t, assessment.ManualApprovalRequired)
	require.False(t, assessment.EnforceAvailable)
	require.Equal(t, "process_since_start_global", assessment.HealthSamplingScope)
	require.Empty(t, assessment.Blockers)
	require.NotEmpty(t, assessment.ManualChecks)
	for _, gate := range assessment.Gates {
		require.True(t, gate.Passed, gate.Name)
	}
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
	health.ObservationCompleteness = 0.98

	assessment := buildOpenAIRoutePromotionAssessment(filter, stats, health)

	require.False(t, assessment.AutomatedEvidenceReady)
	require.Equal(t, "continue_shadow", assessment.Status)
	require.Equal(t, "none", assessment.EligibleNextStage)
	for _, blocker := range []string{
		"account_concentration", "audit_and_observation_health", "evaluation_completeness",
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

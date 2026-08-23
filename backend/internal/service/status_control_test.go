package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEvaluateComputedStatusCustomerFailureOutweighsSuccessfulProbe(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	result := EvaluateComputedStatus(StatusEvaluationInput{
		Now: now,
		Observations: []StatusObservation{
			{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-time.Minute)},
			{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-90 * time.Second)},
			{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-60 * time.Second)},
			{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-30 * time.Second)},
		},
	})

	require.Equal(t, ServiceStatusPartialOutage, result.Status)
	require.Equal(t, "customer_impact", result.Reason)
	require.Equal(t, 3, result.CustomerRequestCount)
	require.Equal(t, 3, result.CustomerFailureCount)
	require.Equal(t, 1, result.ProbeCount)
	require.Equal(t, 1, result.ProbeSuccessCount)
}

func TestEvaluateComputedStatusUsesAcceptedFailureHysteresis(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	makeFailures := func(count int) []StatusObservation {
		items := make([]StatusObservation, 0, count)
		for index := 0; index < count; index++ {
			items = append(items, StatusObservation{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-time.Duration(index+1) * time.Second)})
		}
		return items
	}
	require.Equal(t, ServiceStatusOperational, EvaluateComputedStatus(StatusEvaluationInput{Now: now, Observations: makeFailures(1)}).Status)
	require.Equal(t, ServiceStatusDegradedPerformance, EvaluateComputedStatus(StatusEvaluationInput{Now: now, Observations: makeFailures(2)}).Status)
	require.Equal(t, ServiceStatusPartialOutage, EvaluateComputedStatus(StatusEvaluationInput{Now: now, Observations: makeFailures(3)}).Status)
}

func TestEvaluateComputedStatusCountsOnlyTrailingCustomerFailures(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	result := EvaluateComputedStatus(StatusEvaluationInput{Now: now, Observations: []StatusObservation{
		{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-4 * time.Minute)},
		{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-3 * time.Minute)},
		{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-2 * time.Minute)},
	}})
	require.Equal(t, ServiceStatusOperational, result.Status)
	require.Equal(t, "isolated_failure", result.Reason)
}

func TestEvaluateComputedStatusUsesFailedProbesWhenCustomerTrafficIsSparse(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	makeProbes := func(count int) []StatusObservation {
		items := make([]StatusObservation, 0, count)
		for index := 0; index < count; index++ {
			items = append(items, StatusObservation{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-time.Duration(count-index) * time.Second)})
		}
		return items
	}
	require.Equal(t, ServiceStatusOperational, EvaluateComputedStatus(StatusEvaluationInput{Now: now, Observations: makeProbes(1)}).Status)
	require.Equal(t, ServiceStatusDegradedPerformance, EvaluateComputedStatus(StatusEvaluationInput{Now: now, Observations: makeProbes(2)}).Status)
	require.Equal(t, ServiceStatusPartialOutage, EvaluateComputedStatus(StatusEvaluationInput{Now: now, Observations: makeProbes(3)}).Status)
}

func TestEvaluateComputedStatusReplaysCustomerAndProbeAvailabilitySeparately(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	result := EvaluateComputedStatus(StatusEvaluationInput{Now: now, Observations: []StatusObservation{
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-4 * time.Minute)},
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-3 * time.Minute)},
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-2 * time.Minute)},
		{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-time.Minute)},
	}})
	require.Equal(t, ServiceStatusOperational, result.Status, "a newer final customer success outweighs older probe failures")
	require.Equal(t, 1, result.CustomerRequestCount)
	require.Equal(t, 1, result.CustomerSuccessCount)
	require.Equal(t, 3, result.ProbeCount)
	require.Equal(t, 3, result.ProbeFailureCount)
}

func TestEvaluateComputedStatusTwoSuccessesRecoverDespiteOlderFailures(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	result := EvaluateComputedStatus(StatusEvaluationInput{Now: now, PreviousStatus: ServiceStatusPartialOutage, Observations: []StatusObservation{
		{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-4 * time.Minute)},
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-2 * time.Minute)},
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-time.Minute)},
	}})
	require.Equal(t, ServiceStatusMonitoring, result.Status)
	require.Equal(t, "recovery_observed", result.Reason)
}

func TestEvaluateComputedStatusRecoveryRequiresMonitoringWindow(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	successes := []StatusObservation{
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-time.Minute)},
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-30 * time.Second)},
	}
	result := EvaluateComputedStatus(StatusEvaluationInput{Now: now, PreviousStatus: ServiceStatusPartialOutage, Observations: successes})
	require.Equal(t, ServiceStatusMonitoring, result.Status)
	require.Equal(t, "recovery_observed", result.Reason)

	result = EvaluateComputedStatus(StatusEvaluationInput{
		Now: now, PreviousStatus: ServiceStatusMonitoring, MonitoringSince: now.Add(-11 * time.Minute), RecoveryConfirmedAt: now.Add(-11 * time.Minute), Observations: successes,
	})
	require.Equal(t, ServiceStatusOperational, result.Status)
	require.Equal(t, "monitoring_complete", result.Reason)
}

func TestEvaluateComputedStatusMonitoringCannotCompleteWithoutPersistedProbeRecovery(t *testing.T) {
	now := time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
	result := EvaluateComputedStatus(StatusEvaluationInput{
		Now: now, PreviousStatus: ServiceStatusMonitoring, MonitoringSince: now.Add(-11 * time.Minute),
		Observations: []StatusObservation{{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-time.Minute)}},
	})
	require.Equal(t, ServiceStatusMonitoring, result.Status)
	require.Equal(t, "monitoring_observation", result.Reason)
}

func TestEvaluateComputedStatusMonitoringRelapseStaysMonitoringAndResetsWindow(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	failedAt := now.Add(-time.Minute)
	result := EvaluateComputedStatus(StatusEvaluationInput{
		Now: now, PreviousStatus: ServiceStatusMonitoring, MonitoringSince: now.Add(-11 * time.Minute),
		Observations: []StatusObservation{{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: failedAt}},
	})
	require.Equal(t, ServiceStatusMonitoring, result.Status)
	require.Equal(t, "monitoring_relapse", result.Reason)
	require.Equal(t, failedAt, result.MonitoringResetAt)
}

func TestEvaluateComputedStatusFreshCustomerVolumeOutweighsLaterFailedProbes(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	observations := make([]StatusObservation, 0, 7)
	for index := 0; index < 5; index++ {
		observations = append(observations, StatusObservation{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-time.Duration(10-index) * time.Second)})
	}
	observations = append(observations,
		StatusObservation{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-2 * time.Second)},
		StatusObservation{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-time.Second)},
	)
	result := EvaluateComputedStatus(StatusEvaluationInput{Now: now, Observations: observations})
	require.Equal(t, ServiceStatusOperational, result.Status)
	require.Equal(t, "fresh_customer_evidence", result.Reason)
	require.Equal(t, 5, result.CustomerRequestCount)
	require.Equal(t, 2, result.ProbeFailureCount)
}

func TestEvaluateComputedStatusNewerFailedProbesCanEscalateAfterIsolatedCustomerFailure(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	result := EvaluateComputedStatus(StatusEvaluationInput{Now: now, Observations: []StatusObservation{
		{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-4 * time.Minute)},
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-3 * time.Minute)},
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-2 * time.Minute)},
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-time.Minute)},
	}})
	require.Equal(t, ServiceStatusPartialOutage, result.Status)
	require.Equal(t, "probe_impact", result.Reason)
}

func TestEvaluateComputedStatusBroadCriticalLossIsMajorOutage(t *testing.T) {
	now := time.Now()
	result := EvaluateComputedStatus(StatusEvaluationInput{
		Now: now, CriticalService: true, TotalCustomerRequests: 4,
		Observations: []StatusObservation{
			{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-4 * time.Second)},
			{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-3 * time.Second)},
			{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-2 * time.Second)},
			{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-time.Second)},
		},
	})
	require.Equal(t, ServiceStatusMajorOutage, result.Status)
}

func TestEvaluateComputedStatusEscalatesAcrossReconciliations(t *testing.T) {
	now := time.Now()
	failures := func(count int) []StatusObservation {
		result := make([]StatusObservation, 0, count)
		for index := 0; index < count; index++ {
			result = append(result, StatusObservation{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-time.Duration(count-index) * time.Second)})
		}
		return result
	}
	partial := EvaluateComputedStatus(StatusEvaluationInput{Now: now, PreviousStatus: ServiceStatusDegradedPerformance, Observations: failures(3)})
	require.Equal(t, ServiceStatusPartialOutage, partial.Status)
	major := EvaluateComputedStatus(StatusEvaluationInput{Now: now, PreviousStatus: ServiceStatusPartialOutage, CriticalService: true, Observations: failures(4)})
	require.Equal(t, ServiceStatusMajorOutage, major.Status)
}

func TestEvaluateComputedStatusDoesNotBounceMonitoringBackOnPreRecoveryFailures(t *testing.T) {
	now := time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
	observations := []StatusObservation{
		{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-4 * time.Minute)},
		{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-210 * time.Second)},
		{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-3 * time.Minute)},
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-2 * time.Minute)},
		{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-time.Minute)},
	}
	first := EvaluateComputedStatus(StatusEvaluationInput{Now: now, PreviousStatus: ServiceStatusPartialOutage, Observations: observations})
	require.Equal(t, ServiceStatusMonitoring, first.Status)
	second := EvaluateComputedStatus(StatusEvaluationInput{Now: now.Add(30 * time.Second), PreviousStatus: ServiceStatusMonitoring, MonitoringSince: now, Observations: observations})
	require.Equal(t, ServiceStatusMonitoring, second.Status)
	require.Equal(t, "monitoring_observation", second.Reason)
}

func TestEvaluateComputedStatusMonitoringRelapseWindowStartsAtConfirmedRecovery(t *testing.T) {
	now := time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
	confirmedAt := now.Add(-time.Minute)
	result := EvaluateComputedStatus(StatusEvaluationInput{
		Now: now, PreviousStatus: ServiceStatusMonitoring, MonitoringSince: now.Add(-11 * time.Minute),
		Observations: []StatusObservation{
			{Kind: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true, ObservedAt: now.Add(-4 * time.Minute)},
			{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-2 * time.Minute)},
			{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: confirmedAt},
		},
	})
	require.Equal(t, ServiceStatusMonitoring, result.Status)
	require.Equal(t, "monitoring_relapse", result.Reason)
	require.Equal(t, confirmedAt, result.MonitoringResetAt)
}

func TestEvaluateComputedStatusRecordsRecoveredProbeRelapse(t *testing.T) {
	now := time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
	confirmedAt := now.Add(-30 * time.Second)
	result := EvaluateComputedStatus(StatusEvaluationInput{
		Now: now, PreviousStatus: ServiceStatusMonitoring, MonitoringSince: now.Add(-11 * time.Minute),
		Observations: []StatusObservation{
			{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-4 * time.Minute)},
			{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-3 * time.Minute)},
			{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, ObservedAt: now.Add(-2 * time.Minute)},
			{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-time.Minute)},
			{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: confirmedAt},
		},
	})
	require.Equal(t, ServiceStatusMonitoring, result.Status)
	require.Equal(t, confirmedAt, result.MonitoringResetAt)
}

func TestEvaluateComputedStatusStaleEvidenceBecomesMonitoring(t *testing.T) {
	now := time.Date(2026, 8, 23, 0, 40, 0, 0, time.FixedZone("CST", 8*60*60))
	result := EvaluateComputedStatus(StatusEvaluationInput{
		Now: now, Freshness: 5 * time.Minute,
		Observations: []StatusObservation{{Kind: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-10 * time.Minute)}},
	})
	require.Equal(t, ServiceStatusMonitoring, result.Status)
	require.Equal(t, "stale_evidence", result.Reason)
}

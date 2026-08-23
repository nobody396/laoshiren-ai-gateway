package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEvaluateIncidentLifecycleAggregatesProductsAndClosesOnlyRecoveredSegment(t *testing.T) {
	started := time.Date(2026, 8, 23, 4, 0, 0, 0, time.UTC)
	state := IncidentLifecycleState{Phase: IncidentPhaseInvestigating, Products: []IncidentProductLifecycle{
		{ProductID: 1, CurrentStatus: ServiceStatusPartialOutage, Segments: []IncidentImpactSegment{{StartedAt: started}}},
		{ProductID: 2, CurrentStatus: ServiceStatusMajorOutage, Segments: []IncidentImpactSegment{{StartedAt: started.Add(time.Minute)}}},
	}}

	next, err := EvaluateIncidentLifecycle(state, IncidentLifecycleInput{Now: started.Add(6 * time.Minute), Products: []IncidentProductSignal{
		{ProductID: 1, Status: ServiceStatusMonitoring, MonitoringSince: started.Add(5 * time.Minute)},
		{ProductID: 2, Status: ServiceStatusMajorOutage},
	}})

	require.NoError(t, err)
	require.Equal(t, IncidentPhaseInvestigating, next.Phase)
	require.Equal(t, started.Add(5*time.Minute), *next.Products[0].Segments[0].EndedAt)
	require.Nil(t, next.Products[1].Segments[0].EndedAt)
	require.Equal(t, 5*time.Minute, IncidentCompensableDuration(next.Products[0]))
}

func TestEvaluateIncidentLifecycleRelapseCreatesSecondSegmentAndExcludesHealthyGap(t *testing.T) {
	started := time.Date(2026, 8, 23, 4, 0, 0, 0, time.UTC)
	firstEnd := started.Add(5 * time.Minute)
	state := IncidentLifecycleState{Phase: IncidentPhaseMonitoring, MonitoringSince: firstEnd, Products: []IncidentProductLifecycle{{
		ProductID: 1, CurrentStatus: ServiceStatusMonitoring, MonitoringSince: firstEnd,
		Segments: []IncidentImpactSegment{{StartedAt: started, EndedAt: &firstEnd}},
	}}}
	relapseAt := started.Add(12 * time.Minute)

	next, err := EvaluateIncidentLifecycle(state, IncidentLifecycleInput{Now: relapseAt.Add(time.Minute), Products: []IncidentProductSignal{{
		ProductID: 1, Status: ServiceStatusPartialOutage,
		CustomerFailures: []IncidentCustomerFailure{{ObservationID: 99, ObservedAt: relapseAt}},
	}}})

	require.NoError(t, err)
	require.Equal(t, IncidentPhaseInvestigating, next.Phase)
	require.True(t, next.Relapsed)
	require.True(t, next.CustomerImpactRelapsed)
	require.Len(t, next.Products[0].Segments, 2)
	require.Equal(t, relapseAt, next.Products[0].Segments[1].StartedAt)
	require.Equal(t, int64(99), next.Products[0].Segments[1].StartObservationID)
	require.Equal(t, 5*time.Minute, IncidentCompensableDuration(next.Products[0]), "healthy Monitoring gap must not be compensable")
}

func TestEvaluateIncidentLifecycleProbeOnlyRelapseReopensInvestigationWithoutInventingCustomerImpact(t *testing.T) {
	now := time.Now().UTC()
	ended := now.Add(-10 * time.Minute)
	state := IncidentLifecycleState{Phase: IncidentPhaseMonitoring, MonitoringSince: ended, Products: []IncidentProductLifecycle{{ProductID: 1, CurrentStatus: ServiceStatusMonitoring, MonitoringSince: ended, Segments: []IncidentImpactSegment{{StartedAt: ended.Add(-5 * time.Minute), EndedAt: &ended}}}}}

	next, err := EvaluateIncidentLifecycle(state, IncidentLifecycleInput{Now: now, Products: []IncidentProductSignal{{ProductID: 1, Status: ServiceStatusDegradedPerformance}}})

	require.NoError(t, err)
	require.Equal(t, IncidentPhaseInvestigating, next.Phase)
	require.True(t, next.Relapsed)
	require.False(t, next.CustomerImpactRelapsed)
	require.Len(t, next.Products[0].Segments, 1, "probe-only relapse must not invent customer-impact duration")
}

func TestEvaluateIncidentLifecycleResolvesOnlyAfterTenHealthyMinutesAndOperational(t *testing.T) {
	started := time.Date(2026, 8, 23, 4, 0, 0, 0, time.UTC)
	monitoring := started.Add(5 * time.Minute)
	state := IncidentLifecycleState{Phase: IncidentPhaseMonitoring, MonitoringSince: monitoring, Products: []IncidentProductLifecycle{{
		ProductID: 1, CurrentStatus: ServiceStatusMonitoring, MonitoringSince: monitoring,
		Segments: []IncidentImpactSegment{{StartedAt: started, EndedAt: &monitoring}},
	}}}

	tooEarly, err := EvaluateIncidentLifecycle(state, IncidentLifecycleInput{Now: monitoring.Add(9*time.Minute + 59*time.Second), Products: []IncidentProductSignal{{ProductID: 1, Status: ServiceStatusOperational, MonitoringSince: monitoring}}})
	require.NoError(t, err)
	require.Equal(t, IncidentPhaseMonitoring, tooEarly.Phase)

	resolved, err := EvaluateIncidentLifecycle(state, IncidentLifecycleInput{Now: monitoring.Add(10 * time.Minute), Products: []IncidentProductSignal{{ProductID: 1, Status: ServiceStatusOperational, MonitoringSince: monitoring}}})
	require.NoError(t, err)
	require.Equal(t, IncidentPhaseResolved, resolved.Phase)
	require.NotNil(t, resolved.ResolvedAt)
	require.Equal(t, monitoring.Add(10*time.Minute), *resolved.ResolvedAt)
}

func TestEvaluateIncidentLifecycleEvidenceGapBlocksAutomaticResolution(t *testing.T) {
	now := time.Now().UTC()
	monitoring := now.Add(-20 * time.Minute)
	state := IncidentLifecycleState{Phase: IncidentPhaseMonitoring, MonitoringSince: monitoring, EvidenceGap: true, Products: []IncidentProductLifecycle{{ProductID: 1, CurrentStatus: ServiceStatusOperational, MonitoringSince: monitoring}}}

	next, err := EvaluateIncidentLifecycle(state, IncidentLifecycleInput{Now: now, Products: []IncidentProductSignal{{ProductID: 1, Status: ServiceStatusOperational, MonitoringSince: monitoring}}})

	require.NoError(t, err)
	require.Equal(t, IncidentPhaseMonitoring, next.Phase)
	require.Nil(t, next.ResolvedAt)
}

func TestIncidentPhaseTransitionRejectsSkippedOrPrematureResolution(t *testing.T) {
	now := time.Now().UTC()
	require.Error(t, ValidateIncidentPhaseTransition(IncidentLifecycleState{Phase: IncidentPhaseInvestigating}, IncidentPhaseResolved, now))
	require.NoError(t, ValidateIncidentPhaseTransition(IncidentLifecycleState{Phase: IncidentPhaseInvestigating}, IncidentPhaseIdentified, now))
	require.NoError(t, ValidateIncidentPhaseTransition(IncidentLifecycleState{Phase: IncidentPhaseIdentified}, IncidentPhaseMitigating, now))
	require.Error(t, ValidateIncidentPhaseTransition(IncidentLifecycleState{Phase: IncidentPhaseMonitoring, MonitoringSince: now.Add(-9 * time.Minute), Products: []IncidentProductLifecycle{{ProductID: 1, CurrentStatus: ServiceStatusOperational}}}, IncidentPhaseResolved, now))
	require.Error(t, ValidateIncidentPhaseTransition(IncidentLifecycleState{Phase: IncidentPhaseMonitoring, MonitoringSince: now.Add(-10 * time.Minute), Products: []IncidentProductLifecycle{{ProductID: 1, CurrentStatus: ServiceStatusMonitoring}}}, IncidentPhaseResolved, now))
	require.Error(t, ValidateIncidentPhaseTransition(IncidentLifecycleState{Phase: IncidentPhaseMonitoring, MonitoringSince: now.Add(-10 * time.Minute), EvidenceGap: true, Products: []IncidentProductLifecycle{{ProductID: 1, CurrentStatus: ServiceStatusOperational}}}, IncidentPhaseResolved, now))
	require.NoError(t, ValidateIncidentPhaseTransition(IncidentLifecycleState{Phase: IncidentPhaseMonitoring, MonitoringSince: now.Add(-10 * time.Minute), Products: []IncidentProductLifecycle{{ProductID: 1, CurrentStatus: ServiceStatusOperational}}}, IncidentPhaseResolved, now))
}

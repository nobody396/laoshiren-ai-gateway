package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildOpenAIRouteEvidenceWindowAllowsGracefulReleaseHandoff(t *testing.T) {
	start := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	end := start.Add(72 * time.Hour)
	epochs := []OpenAIRouteEvidenceEpoch{
		testOpenAIRouteEvidenceEpoch("audit-1", OpenAIRouteEvidenceComponentAudit, start.Add(-time.Hour), start.Add(36*time.Hour), true),
		testOpenAIRouteEvidenceEpoch("audit-2", OpenAIRouteEvidenceComponentAudit, start.Add(36*time.Hour+20*time.Second), end.Add(time.Minute), false),
		testOpenAIRouteEvidenceEpoch("observation-1", OpenAIRouteEvidenceComponentObservation, start.Add(-time.Hour), start.Add(36*time.Hour), true),
		testOpenAIRouteEvidenceEpoch("observation-2", OpenAIRouteEvidenceComponentObservation, start.Add(36*time.Hour+20*time.Second), end.Add(time.Minute), false),
	}
	window := BuildOpenAIRouteEvidenceWindowHealth(start, end, epochs)
	require.True(t, window.Available)
	require.True(t, window.Ready)
	require.Equal(t, 2, window.Audit.Epochs)
	require.Equal(t, 2, window.Observation.Epochs)
	require.InDelta(t, 20, window.Audit.MaximumGapSeconds, 0.001)
}

func TestBuildOpenAIRouteEvidenceWindowBlocksStaleUncleanEpoch(t *testing.T) {
	start := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	end := start.Add(72 * time.Hour)
	epochs := []OpenAIRouteEvidenceEpoch{
		testOpenAIRouteEvidenceEpoch("audit-crash", OpenAIRouteEvidenceComponentAudit, start.Add(-time.Hour), start.Add(36*time.Hour), false),
		testOpenAIRouteEvidenceEpoch("audit-new", OpenAIRouteEvidenceComponentAudit, start.Add(36*time.Hour+20*time.Second), end.Add(time.Minute), false),
		testOpenAIRouteEvidenceEpoch("observation-1", OpenAIRouteEvidenceComponentObservation, start.Add(-time.Hour), end.Add(time.Minute), false),
	}
	window := BuildOpenAIRouteEvidenceWindowHealth(start, end, epochs)
	require.False(t, window.Ready)
	require.Equal(t, 1, window.Audit.UncleanEpochs)
}

func testOpenAIRouteEvidenceEpoch(id, component string, startedAt, coveredAt time.Time, clean bool) OpenAIRouteEvidenceEpoch {
	epoch := OpenAIRouteEvidenceEpoch{
		EpochID:       id,
		InstanceID:    id,
		Component:     component,
		StartedAt:     startedAt,
		HeartbeatAt:   coveredAt,
		CleanShutdown: clean,
		Counters: OpenAIRouteEvidenceCounters{
			Attempted:       10,
			Written:         10,
			OutcomeExpected: 10,
			OutcomeApplied:  10,
			StorageChecks:   1,
		},
	}
	if clean {
		epoch.StoppedAt = coveredAt
	}
	return epoch
}

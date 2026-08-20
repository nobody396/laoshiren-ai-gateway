//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestOpenAIRouteDecisionRepositoryRoundTrip(t *testing.T) {
	repo := NewOpenAIRouteDecisionRepository(integrationDB)
	require.NoError(t, repo.CheckOpenAIRouteShadowDecisionStorage(context.Background()))
	var probeRows int
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM openai_route_shadow_decisions WHERE model = '__storage_probe__'").Scan(&probeRows))
	require.Zero(t, probeRows, "writable-storage probe must roll back without audit noise")
	decisionID := "shadow:" + uuid.NewString()
	requestID := "req-" + uuid.NewString()
	clientRequestID := uuid.NewString()
	createdAt := time.Now().UTC().Truncate(time.Microsecond)
	shadowStartedAt := createdAt.Add(-72 * time.Hour)
	record := &service.OpenAIRouteShadowDecisionRecord{
		DecisionID:                decisionID,
		RequestID:                 requestID,
		ClientRequestID:           clientRequestID,
		Attempt:                   2,
		GroupID:                   7,
		Model:                     "gpt-5.6-sol",
		RequestClass:              service.OpenAIRouteRequestClassText,
		PolicyMode:                service.OpenAIRoutePolicyShadow,
		PolicyVersion:             4,
		ActivationID:              "activation-integration-4",
		ExperimentID:              "experiment-integration",
		VariantID:                 "latency-v2",
		ShadowStartedAt:           shadowStartedAt,
		Reason:                    "shadow_selected",
		Evaluated:                 true,
		EvaluationDurationMicros:  321,
		LegacySelectedAccountID:   23,
		AdaptiveSelectedAccountID: 28,
		AdaptiveSelectedRate:      0.15,
		CandidateCount:            3,
		ExcludedCount:             1,
		Diverged:                  true,
		Snapshot: &service.OpenAIRouteShadowAuditSnapshot{
			ActivationID:         "activation-integration-4",
			ShadowStartedAt:      shadowStartedAt,
			RequestClass:         service.OpenAIRouteRequestClassText,
			EstimatedBaseCostUSD: 0.01,
			Policy: service.OpenAIRouteShadowAuditPolicy{
				MaxAccountShare:  0.80,
				MaxProviderShare: 0.90,
			},
			Candidates: []service.OpenAIRouteShadowAuditCandidate{{
				AccountID:      28,
				RateMultiplier: 0.15,
				Selected:       true,
			}},
		},
		CreatedAt: createdAt,
	}
	require.NoError(t, repo.CreateOpenAIRouteShadowDecision(context.Background(), record))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM openai_route_shadow_decisions WHERE decision_id = $1", decisionID)
	})

	start := createdAt.Add(-time.Minute)
	end := createdAt.Add(time.Minute)
	list, err := repo.ListOpenAIRouteShadowDecisions(context.Background(), &service.OpenAIRouteShadowDecisionFilter{
		StartTime:    &start,
		EndTime:      &end,
		RequestID:    requestID,
		RequestClass: service.OpenAIRouteRequestClassText,
		ActivationID: "activation-integration-4",
		ExperimentID: "experiment-integration",
		VariantID:    "latency-v2",
	})
	require.NoError(t, err)
	require.Equal(t, 1, list.Total)
	require.Len(t, list.Decisions, 1)
	require.Equal(t, int64(28), list.Decisions[0].AdaptiveSelectedAccountID)
	require.Equal(t, service.OpenAIRouteRequestClassText, list.Decisions[0].RequestClass)
	require.Equal(t, "activation-integration-4", list.Decisions[0].ActivationID)
	require.Equal(t, "experiment-integration", list.Decisions[0].ExperimentID)
	require.Equal(t, "latency-v2", list.Decisions[0].VariantID)
	require.Len(t, list.Decisions[0].TreatmentFingerprint, 32)
	require.Equal(t, shadowStartedAt, list.Decisions[0].ShadowStartedAt)
	require.Len(t, list.Decisions[0].Snapshot.Candidates, 1)

	stats, err := repo.GetOpenAIRouteShadowDecisionStats(context.Background(), &service.OpenAIRouteShadowDecisionFilter{
		StartTime:    &start,
		EndTime:      &end,
		RequestID:    requestID,
		ActivationID: "activation-integration-4",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.Total)
	require.Equal(t, int64(1), stats.Evaluated)
	require.Zero(t, stats.NoCandidateAbstentions)
	require.Equal(t, int64(1), stats.Diverged)
	require.Equal(t, int64(1), stats.UnlinkedOutcome)
	require.Equal(t, int64(1), stats.EvaluatedUnlinkedOutcome)
	require.Equal(t, int64(1), stats.PolicySnapshotVariants)
	require.Equal(t, int64(1), stats.ActivationIDVariants)
	require.Equal(t, int64(1), stats.ShadowStartedAtVariants)
	require.Equal(t, int64(1), stats.ExperimentIDVariants)
	require.Equal(t, int64(1), stats.VariantIDVariants)
	require.Equal(t, int64(1), stats.TreatmentFingerprintVariants)
	require.Equal(t, shadowStartedAt, stats.ShadowStartedAt)
	require.Equal(t, int64(1), stats.CoveredHourBuckets)
	require.Equal(t, 0.80, stats.PolicyMaxAccountShare)
	require.Equal(t, 0.90, stats.PolicyMaxProviderShare)
	require.Equal(t, createdAt, stats.FirstDecisionAt)
	require.Equal(t, createdAt, stats.LastDecisionAt)
	require.Len(t, stats.SelectedAccounts, 1)
	require.Equal(t, int64(28), stats.SelectedAccounts[0].AccountID)
	require.Len(t, stats.SelectedProviders, 1)
	require.Equal(t, "account:28", stats.SelectedProviders[0].ProviderKey)
}

func TestOpenAIRouteEvidenceEpochRepositoryRoundTrip(t *testing.T) {
	repo := NewOpenAIRouteDecisionRepository(integrationDB)
	store, ok := repo.(service.OpenAIRouteEvidenceEpochStore)
	require.True(t, ok)
	now := time.Now().UTC().Truncate(time.Microsecond)
	epoch := service.OpenAIRouteEvidenceEpoch{
		EpochID:     "route-evidence:audit:" + uuid.NewString(),
		InstanceID:  uuid.NewString(),
		Component:   service.OpenAIRouteEvidenceComponentAudit,
		StartedAt:   now.Add(-time.Minute),
		HeartbeatAt: now,
		Counters: service.OpenAIRouteEvidenceCounters{
			Attempted:     3,
			Written:       3,
			StorageChecks: 1,
		},
	}
	require.NoError(t, store.BeginOpenAIRouteEvidenceEpoch(context.Background(), epoch))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM openai_route_evidence_epochs WHERE epoch_id = $1", epoch.EpochID)
	})
	require.NoError(t, store.CheckpointOpenAIRouteEvidenceEpoch(context.Background(), epoch))

	epoch.HeartbeatAt = now.Add(time.Second)
	epoch.StoppedAt = epoch.HeartbeatAt
	epoch.CleanShutdown = true
	epoch.Counters.Attempted = 4
	epoch.Counters.Written = 4
	require.NoError(t, store.CheckpointOpenAIRouteEvidenceEpoch(context.Background(), epoch))

	epochs, err := store.ListOpenAIRouteEvidenceEpochs(context.Background(), now.Add(-2*time.Minute), now.Add(2*time.Minute))
	require.NoError(t, err)
	var found *service.OpenAIRouteEvidenceEpoch
	for idx := range epochs {
		if epochs[idx].EpochID == epoch.EpochID {
			found = &epochs[idx]
			break
		}
	}
	require.NotNil(t, found)
	require.True(t, found.CleanShutdown)
	require.Equal(t, uint64(4), found.Counters.Attempted)
	require.Equal(t, epoch.StoppedAt, found.StoppedAt)
}

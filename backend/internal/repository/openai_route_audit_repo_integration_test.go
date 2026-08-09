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
	record := &service.OpenAIRouteShadowDecisionRecord{
		DecisionID:                decisionID,
		RequestID:                 requestID,
		ClientRequestID:           clientRequestID,
		Attempt:                   2,
		GroupID:                   7,
		Model:                     "gpt-5.6-sol",
		PolicyMode:                service.OpenAIRoutePolicyShadow,
		PolicyVersion:             4,
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
			EstimatedBaseCostUSD: 0.01,
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
		StartTime: &start,
		EndTime:   &end,
		RequestID: requestID,
	})
	require.NoError(t, err)
	require.Equal(t, 1, list.Total)
	require.Len(t, list.Decisions, 1)
	require.Equal(t, int64(28), list.Decisions[0].AdaptiveSelectedAccountID)
	require.Len(t, list.Decisions[0].Snapshot.Candidates, 1)

	stats, err := repo.GetOpenAIRouteShadowDecisionStats(context.Background(), &service.OpenAIRouteShadowDecisionFilter{
		StartTime: &start,
		EndTime:   &end,
		RequestID: requestID,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.Total)
	require.Equal(t, int64(1), stats.Evaluated)
	require.Equal(t, int64(1), stats.Diverged)
	require.Equal(t, int64(1), stats.UnlinkedOutcome)
	require.Len(t, stats.SelectedAccounts, 1)
	require.Equal(t, int64(28), stats.SelectedAccounts[0].AccountID)
}

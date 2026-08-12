package repository

import (
	"context"
	"database/sql/driver"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIRouteDecisionRepositoryStatsScansPromotionEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &openAIRouteDecisionRepository{db: db}

	start := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	end := start.Add(72 * time.Hour)
	groupID := int64(7)
	version := 4
	filter := &service.OpenAIRouteShadowDecisionFilter{
		StartTime:     &start,
		EndTime:       &end,
		GroupID:       &groupID,
		Model:         "gpt-5.6-sol",
		RequestClass:  service.OpenAIRouteRequestClassText,
		PolicyMode:    service.OpenAIRoutePolicyShadow,
		PolicyVersion: &version,
	}
	args := []driver.Value{start, end, groupID, "gpt-5.6-sol", "text", "shadow", version}

	aggregateColumns := []string{
		"total", "evaluated", "not_evaluated", "diverged", "emergency",
		"linked_success", "linked_failure", "ambiguous", "unlinked",
		"evaluated_linked_success", "evaluated_linked_failure", "evaluated_ambiguous", "evaluated_unlinked",
		"policy_variants", "max_account_share", "max_provider_share", "first_at", "last_at",
		"evaluation_p50", "evaluation_p95", "ttft_p50", "ttft_p95",
	}
	mock.ExpectQuery(`(?s)COUNT\(DISTINCT snapshot->'policy'\).*MIN\(created_at\).*FROM linked`).
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows(aggregateColumns).AddRow(
			200, 200, 0, 40, 0,
			196, 4, 0, 0,
			196, 4, 0, 0,
			1, 0.8, 0.9, start, end,
			120.0, 240.0, 500.0, 900.0,
		))
	mock.ExpectQuery(regexp.QuoteMeta("GROUP BY adaptive_selected_account_id, adaptive_selected_rate")).
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"account_id", "rate", "count"}).
			AddRow(int64(23), 0.15, int64(120)).
			AddRow(int64(28), 0.30, int64(80)))
	mock.ExpectQuery(`(?s)jsonb_array_elements\(d\.snapshot->'candidates'\).*GROUP BY provider_key`).
		WithArgs(args...).
		WillReturnRows(sqlmock.NewRows([]string{"provider_key", "count"}).
			AddRow("pomelo-hk", int64(120)).
			AddRow("morecode", int64(80)))

	stats, err := repo.GetOpenAIRouteShadowDecisionStats(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, int64(200), stats.Evaluated)
	require.Equal(t, int64(196), stats.EvaluatedLinkedSuccessfulUsage)
	require.Equal(t, int64(4), stats.EvaluatedLinkedLegacyFailure)
	require.Equal(t, int64(1), stats.PolicySnapshotVariants)
	require.Equal(t, start, stats.FirstDecisionAt)
	require.Equal(t, end, stats.LastDecisionAt)
	require.Len(t, stats.SelectedAccounts, 2)
	require.InDelta(t, 60, stats.SelectedAccounts[0].SelectedPercent, 1e-12)
	require.Len(t, stats.SelectedProviders, 2)
	require.Equal(t, "pomelo-hk", stats.SelectedProviders[0].ProviderKey)
	require.InDelta(t, 60, stats.SelectedProviders[0].SelectedPercent, 1e-12)
	require.NoError(t, mock.ExpectationsWereMet())
}

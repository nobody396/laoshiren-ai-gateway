package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestGetCostAccountingRealUsageIncludesCreditAndPayAsYouGoLedgers(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}

	creditGroupIDs := []int64{7, 8}
	payAsYouGoGroupIDs := []int64{34}
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	end := start.Add(12 * 24 * time.Hour)

	mock.ExpectQuery(`(?s)COALESCE\(account_stats_cost, total_cost\).*billing_type = 1 AND group_id = ANY\(\$1\).*billing_type = 0 AND group_id = ANY\(\$2\)`).
		WithArgs(pq.Array(creditGroupIDs), pq.Array(payAsYouGoGroupIDs), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"group_id", "request_count", "raw_credits", "real_cost"}).
			AddRow(int64(7), int64(12), 98.5, 11.25).
			AddRow(int64(34), int64(20), 188.5, 37.75))

	usage, err := repo.GetCostAccountingRealUsage(context.Background(), creditGroupIDs, payAsYouGoGroupIDs, start, end)

	require.NoError(t, err)
	require.Equal(t, int64(12), usage[7].RequestCount)
	require.Equal(t, 11.25, usage[7].RealCostCNY)
	require.Equal(t, int64(20), usage[34].RequestCount)
	require.Equal(t, 188.5, usage[34].RawCredits)
	require.Equal(t, 37.75, usage[34].RealCostCNY)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetCostAccountingRealUsageSkipsQueryWithoutGroups(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}
	now := time.Now()

	usage, err := repo.GetCostAccountingRealUsage(context.Background(), nil, nil, now.Add(-time.Hour), now)

	require.NoError(t, err)
	require.Empty(t, usage)
	require.NoError(t, mock.ExpectationsWereMet())
}

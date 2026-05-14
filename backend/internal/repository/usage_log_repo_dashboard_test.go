package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestFillDashboardYesterdayActualCostAggregated(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	today := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	stats := &usagestats.DashboardStats{}

	mock.ExpectQuery("FROM usage_dashboard_daily").
		WithArgs(today.AddDate(0, 0, -1)).
		WillReturnRows(sqlmock.NewRows([]string{"yesterday_actual_cost"}).AddRow(8362.25))

	err := repo.fillDashboardYesterdayActualCostAggregated(context.Background(), stats, today)
	require.NoError(t, err)
	require.Equal(t, 8362.25, stats.YesterdayActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFillDashboardYesterdayActualCostAggregatedMissingRow(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	today := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	stats := &usagestats.DashboardStats{}

	mock.ExpectQuery("FROM usage_dashboard_daily").
		WithArgs(today.AddDate(0, 0, -1)).
		WillReturnRows(sqlmock.NewRows([]string{"yesterday_actual_cost"}))

	err := repo.fillDashboardYesterdayActualCostAggregated(context.Background(), stats, today)
	require.NoError(t, err)
	require.Zero(t, stats.YesterdayActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFillDashboardYesterdayActualCostFromUsageLogs(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &usageLogRepository{sql: db}
	today := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	stats := &usagestats.DashboardStats{}

	mock.ExpectQuery("FROM usage_logs").
		WithArgs(today.AddDate(0, 0, -1), today).
		WillReturnRows(sqlmock.NewRows([]string{"yesterday_actual_cost"}).AddRow(8362.25))

	err := repo.fillDashboardYesterdayActualCostFromUsageLogs(context.Background(), stats, today)
	require.NoError(t, err)
	require.Equal(t, 8362.25, stats.YesterdayActualCost)
	require.NoError(t, mock.ExpectationsWereMet())
}

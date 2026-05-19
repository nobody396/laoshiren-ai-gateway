package repository

import (
	"context"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAgentLevelRepositoryGetUsageStatsUsesDirectUsersAndMonthWindow(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &commissionRepository{sql: db}
	start := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`FROM usage_logs ul\s+JOIN users u ON u\.id = ul\.user_id\s+WHERE u\.agent_id = \$1 AND u\.deleted_at IS NULL`).
		WithArgs(int64(7), start, end).
		WillReturnRows(sqlmock.NewRows([]string{"last_month_consumption", "total_consumption"}).AddRow(1500.0, 10000.0))

	stats, err := repo.GetAgentLevelUsageStats(context.Background(), 7, start, end)
	require.NoError(t, err)
	require.Equal(t, 1500.0, stats.LastMonthConsumption)
	require.Equal(t, 10000.0, stats.TotalConsumption)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAgentLevelRepositoryUpsertStateIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := &commissionRepository{sql: db}
	temp := service.AgentLevelCore
	next := service.AgentLevelSuper
	evaluatedAt := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	mock.ExpectQuery(`INSERT INTO agent_level_states`).
		WithArgs(
			int64(7),
			service.AgentLevelLight,
			0.05,
			service.AgentLevelStandard,
			temp,
			service.AgentLevelCore,
			0.15,
			service.AgentLevelRateSourceMonthly,
			"2026-05",
			1500.0,
			2000.0,
			next,
			48000.0,
			evaluatedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(evaluatedAt))

	err = repo.UpsertAgentLevelState(context.Background(), &service.AgentLevelState{
		AgentID:              7,
		BaseLevelKey:         service.AgentLevelLight,
		BaseRate:             0.05,
		PermanentLevelKey:    service.AgentLevelStandard,
		TemporaryLevelKey:    &temp,
		CurrentLevelKey:      service.AgentLevelCore,
		CurrentRate:          0.15,
		RateSource:           service.AgentLevelRateSourceMonthly,
		LastEvaluatedPeriod:  "2026-05",
		LastMonthConsumption: 1500,
		TotalConsumption:     2000,
		NextLevelKey:         &next,
		NextLevelGap:         48000,
		EvaluatedAt:          &evaluatedAt,
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

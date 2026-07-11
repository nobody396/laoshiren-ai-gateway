package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpsRepositoryListRequestDetails_SortsAndReturnsFirstTokenLatency(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &opsRepository{db: db}

	start := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	filter := &service.OpsRequestDetailFilter{
		StartTime: &start,
		EndTime:   &end,
		Sort:      "first_token_desc",
		Page:      1,
		PageSize:  10,
	}

	mock.ExpectQuery(`SELECT COUNT\(1\) FROM combined`).
		WithArgs(start, end).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))

	rows := sqlmock.NewRows([]string{
		"kind",
		"created_at",
		"request_id",
		"platform",
		"model",
		"duration_ms",
		"first_token_ms",
		"status_code",
		"error_id",
		"phase",
		"severity",
		"message",
		"user_id",
		"api_key_id",
		"account_id",
		"group_id",
		"stream",
	}).AddRow(
		"success",
		start.Add(5*time.Minute),
		"req-owned",
		"anthropic",
		"model-a",
		1800,
		240,
		nil,
		nil,
		nil,
		nil,
		nil,
		int64(2),
		int64(9),
		int64(11),
		int64(13),
		true,
	)

	mock.ExpectQuery(`ORDER BY first_token_ms DESC NULLS LAST, created_at DESC\s+LIMIT \$3 OFFSET \$4`).
		WithArgs(start, end, 10, 0).
		WillReturnRows(rows)

	items, total, err := repo.ListRequestDetails(context.Background(), filter)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.NotNil(t, items[0].DurationMs)
	require.Equal(t, 1800, *items[0].DurationMs)
	require.NotNil(t, items[0].FirstTokenMs)
	require.Equal(t, 240, *items[0].FirstTokenMs)
	require.NoError(t, mock.ExpectationsWereMet())
}

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUserRepositoryTouchLastActive(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &userRepository{sql: db}
	now := time.Date(2026, 4, 23, 9, 30, 0, 0, time.UTC)

	mock.ExpectExec("UPDATE users").
		WithArgs(int64(7), now.UTC(), now.Add(-60*time.Second).UTC()).
		WillReturnResult(newTestResult(1))

	err := repo.TouchLastActive(context.Background(), 7, now)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUserRepositoryTouchLastActiveSkipsWithoutSQL(t *testing.T) {
	repo := &userRepository{}
	err := repo.TouchLastActive(context.Background(), 7, time.Now())
	require.NoError(t, err)
}

func TestUserRepositoryTouchLastActiveSkipsInvalidUserID(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &userRepository{sql: db}

	err := repo.TouchLastActive(context.Background(), 0, time.Now())
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

type testResult int64

func newTestResult(rows int64) testResult {
	return testResult(rows)
}

func (r testResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (r testResult) RowsAffected() (int64, error) {
	return int64(r), nil
}

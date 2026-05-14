package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPendingAuthSessionRepositoryGetByStateNotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &pendingAuthSessionRepository{db: db}

	mock.ExpectQuery("SELECT id, state, provider, provider_user_id, intended_action, claims_snapshot, redirect_uri,").
		WithArgs("state-1").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetByState(context.Background(), "state-1")
	require.ErrorIs(t, err, service.ErrPendingAuthSessionNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPendingAuthSessionRepositoryResolve(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &pendingAuthSessionRepository{db: db}
	now := time.Date(2026, 4, 23, 11, 0, 0, 0, time.UTC)
	expiresAt := now.Add(10 * time.Minute)

	mock.ExpectQuery("UPDATE pending_auth_sessions").
		WithArgs("state-1", "linuxdo-sub-1", []byte(`{"email":"linuxdo-sub-1@oauth.local","subject":"linuxdo-sub-1"}`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "state", "provider", "provider_user_id", "intended_action", "claims_snapshot",
			"redirect_uri", "user_id", "expires_at", "consumed_at", "created_at", "ip", "user_agent",
		}).AddRow(
			int64(1), "state-1", "linuxdo", "linuxdo-sub-1", "login",
			[]byte(`{"email":"linuxdo-sub-1@oauth.local","subject":"linuxdo-sub-1"}`),
			"/profile", nil, expiresAt, nil, now, "127.0.0.1", "UA",
		))

	session, err := repo.Resolve(context.Background(), "state-1", service.PendingAuthSessionResolveInput{
		ProviderUserID: "linuxdo-sub-1",
		ClaimsSnapshot: map[string]any{
			"email":   "linuxdo-sub-1@oauth.local",
			"subject": "linuxdo-sub-1",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, "linuxdo-sub-1", *session.ProviderUserID)
	require.Equal(t, "linuxdo-sub-1@oauth.local", session.ClaimsSnapshot["email"])
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPendingAuthSessionRepositoryMarkConsumedAlreadyConsumed(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &pendingAuthSessionRepository{db: db}

	mock.ExpectExec("UPDATE pending_auth_sessions").
		WithArgs("state-1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.MarkConsumed(context.Background(), "state-1", time.Now())
	require.ErrorIs(t, err, service.ErrPendingAuthSessionConsumed)
	require.NoError(t, mock.ExpectationsWereMet())
}

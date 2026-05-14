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

func TestAuthIdentityRepositoryGetByProviderAccountNotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &authIdentityRepository{db: db}

	mock.ExpectQuery("SELECT id, user_id, provider, provider_user_id, email, email_verified, display_name, avatar_url,").
		WithArgs("linuxdo", "sub-1").
		WillReturnError(sql.ErrNoRows)

	_, err := repo.GetByProviderAccount(context.Background(), "linuxdo", "sub-1")
	require.ErrorIs(t, err, service.ErrAuthIdentityNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthIdentityRepositoryUpsertConflict(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &authIdentityRepository{db: db}
	now := time.Date(2026, 4, 23, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT id, user_id, provider, provider_user_id, email, email_verified, display_name, avatar_url,").
		WithArgs("linuxdo", "sub-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "provider", "provider_user_id", "email", "email_verified", "display_name",
			"avatar_url", "raw_profile", "last_login_at", "bound_at", "created_at", "updated_at",
		}).AddRow(
			int64(1), int64(99), "linuxdo", "sub-1", "linuxdo-sub-1@oauth.local", false, "tester",
			"", []byte(`{"subject":"sub-1"}`), nil, now, now, now,
		))

	_, err := repo.Upsert(context.Background(), service.AuthIdentityUpsertInput{
		UserID:         42,
		Provider:       "linuxdo",
		ProviderUserID: "sub-1",
		Email:          "linuxdo-sub-1@oauth.local",
		DisplayName:    "tester",
		RawProfile:     map[string]any{"subject": "sub-1"},
	})
	require.ErrorIs(t, err, service.ErrAuthIdentityConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAuthIdentityRepositoryDeleteByUserProviderNotFound(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &authIdentityRepository{db: db}

	mock.ExpectExec("DELETE FROM auth_identities").
		WithArgs(int64(42), "linuxdo").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.DeleteByUserProvider(context.Background(), 42, "linuxdo")
	require.ErrorIs(t, err, service.ErrAuthIdentityNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

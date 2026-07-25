package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func newAnnouncementPopupStateTestRepo(t *testing.T) (*announcementReadRepository, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sql mock: %v", err)
	}
	return newAnnouncementReadRepositoryWithSQL(nil, db), mock, db
}

func TestAnnouncementReadRepository_GetLastPromptedAnnouncementIDDefaultsToZero(t *testing.T) {
	repo, mock, db := newAnnouncementPopupStateTestRepo(t)
	defer db.Close()

	mock.ExpectQuery(`SELECT last_prompted_announcement_id`).
		WithArgs(int64(42)).
		WillReturnError(sql.ErrNoRows)

	got, err := repo.GetLastPromptedAnnouncementID(context.Background(), 42)
	if err != nil {
		t.Fatalf("get popup state: %v", err)
	}
	if got != 0 {
		t.Fatalf("expected zero cursor, got %d", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestAnnouncementReadRepository_MarkPopupBatchPromptedIsMonotonic(t *testing.T) {
	repo, mock, db := newAnnouncementPopupStateTestRepo(t)
	defer db.Close()

	promptedAt := time.Date(2026, 7, 25, 12, 0, 0, 0, time.FixedZone("EDT", -4*60*60))
	mock.ExpectExec(`INSERT INTO user_announcement_states`).
		WithArgs(int64(42), int64(21), promptedAt.UTC()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	if err := repo.MarkPopupBatchPrompted(context.Background(), 42, 21, promptedAt); err != nil {
		t.Fatalf("mark popup batch prompted: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

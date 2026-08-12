//go:build unit

package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSupplierRepositoryUpsertFromAccountPreservesDisabledProbeOnInsert(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &supplierRepository{db: db}
	now := time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)
	accountID := int64(39)
	supplier := &service.Supplier{
		Name:                 "image-only-route",
		BaseURL:              "https://images.example.invalid",
		APIKey:               "sk-test",
		UpstreamGroup:        service.PlatformOpenAI,
		Status:               service.SupplierStatusActive,
		Notes:                "synced",
		SourceAccountID:      &accountID,
		SourcePlatform:       service.PlatformOpenAI,
		ProbeEnabled:         false,
		ProbeModel:           service.DefaultSupplierOpenAIProbeModel,
		ProbeIntervalMinutes: service.DefaultSupplierProbeIntervalMinutes,
		LastProbeStatus:      service.SupplierProbeStatusUnknown,
		TargetGroupIDs:       []int64{},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("UPDATE suppliers SET")).
		WithArgs(
			accountID,
			supplier.Name,
			supplier.BaseURL,
			supplier.APIKey,
			supplier.UpstreamGroup,
			supplier.Status,
			supplier.Notes,
			supplier.SourcePlatform,
			supplier.ProbeModel,
			supplier.ProbeIntervalMinutes,
		).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta("INSERT INTO suppliers (")).
		WithArgs(
			supplier.Name,
			supplier.BaseURL,
			supplier.APIKey,
			supplier.UpstreamGroup,
			supplier.Status,
			supplier.Notes,
			accountID,
			supplier.SourcePlatform,
			false,
			supplier.ProbeModel,
			supplier.ProbeIntervalMinutes,
			supplier.LastProbeStatus,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(int64(47), now, now))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM supplier_groups WHERE supplier_id = $1")).
		WithArgs(int64(47)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	created, err := repo.UpsertFromAccount(context.Background(), supplier)

	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, int64(47), supplier.ID)
	require.False(t, supplier.ProbeEnabled)
	require.NoError(t, mock.ExpectationsWereMet())
}

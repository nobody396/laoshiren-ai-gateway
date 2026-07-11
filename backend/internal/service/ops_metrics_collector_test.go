package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestQueryErrorCountsExcludesSyntheticProbeFromSLA(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock: %v", err)
	}
	defer func() { _ = db.Close() }()

	mock.ExpectQuery(`(?s).*COALESCE\(error_owner, ''\) <> 'client'.*COALESCE\(error_source, ''\) <> 'monthly_upstream_probe'.*`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"error_total",
			"business_limited",
			"error_sla",
			"upstream_excl",
			"upstream_429",
			"upstream_529",
		}).AddRow(7, 1, 2, 2, 0, 0))

	collector := &OpsMetricsCollector{db: db}
	start := time.Now().Add(-time.Hour)
	end := time.Now()
	total, business, sla, upstream, rateLimited, overloaded, err := collector.queryErrorCounts(context.Background(), start, end)
	if err != nil {
		t.Fatalf("query error counts: %v", err)
	}
	if total != 7 || business != 1 || sla != 2 || upstream != 2 || rateLimited != 0 || overloaded != 0 {
		t.Fatalf("unexpected counts: total=%d business=%d sla=%d upstream=%d 429=%d 529=%d", total, business, sla, upstream, rateLimited, overloaded)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

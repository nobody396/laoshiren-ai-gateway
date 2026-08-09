package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestExpandCommissionTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:   "canonical consumption expands with legacy alias",
			input:  service.CommissionTypeConsumption,
			expect: []string{service.CommissionTypeConsumption, "consumption_commission"},
		},
		{
			name:   "legacy consumption expands with canonical alias",
			input:  "consumption_commission",
			expect: []string{service.CommissionTypeConsumption, "consumption_commission"},
		},
		{
			name:   "self consumption remains independently filterable",
			input:  "self_consumption_commission",
			expect: []string{"self_consumption_commission"},
		},
		{
			name:   "canonical referral expands with legacy alias",
			input:  service.CommissionTypeFirstRechargeReferral,
			expect: []string{service.CommissionTypeFirstRechargeReferral, "first_recharge_referral_bonus"},
		},
		{
			name:   "unknown type remains unchanged",
			input:  "custom_type",
			expect: []string{"custom_type"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := expandCommissionTypes(tc.input)
			if len(got) != len(tc.expect) {
				t.Fatalf("unexpected alias count: got %d want %d", len(got), len(tc.expect))
			}
			for i := range tc.expect {
				if got[i] != tc.expect[i] {
					t.Fatalf("unexpected alias at index %d: got %q want %q", i, got[i], tc.expect[i])
				}
			}
		})
	}
}

func TestListInvitedUsersWithAffiliateStatsReturnsDirectInviteeFullEmail(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	repo := NewCommissionRepository(nil, db)
	registeredAt := time.Date(2026, 8, 9, 10, 59, 41, 0, time.UTC)

	mock.ExpectQuery("SELECT\\s+COALESCE\\(to_regclass").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectQuery("(?s)WITH direct_users AS .*SELECT COUNT\\(\\*\\) FROM direct_users").
		WithArgs(int64(47)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery("(?s)WITH direct_users AS .*recharge_totals AS .*ORDER BY d.created_at DESC").
		WithArgs(int64(47), nil, nil, 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "username", "created_at", "recharged_amount", "consumed_amount", "commission_amount",
		}).AddRow(int64(120), "direct.invitee@example.com", "Direct Invitee", registeredAt, 20.0, 7.5, 0.38))

	users, page, err := repo.ListInvitedUsersWithStats(
		context.Background(),
		47,
		pagination.PaginationParams{Page: 1, PageSize: 20},
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), page.Total)
	require.Len(t, users, 1)
	require.Equal(t, int64(120), users[0].UserID)
	require.Equal(t, "direct.invitee@example.com", users[0].Email)
	require.NoError(t, mock.ExpectationsWereMet())
}

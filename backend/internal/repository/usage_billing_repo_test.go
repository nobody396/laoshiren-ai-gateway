package repository

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageBillingRepositoryApply_BalanceFinalLimitRollbackOnInsufficientFunds(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewUsageBillingRepository(nil, db)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO usage_billing_dedup`).
		WithArgs("req-low-balance", int64(22), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectQuery(`SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WithArgs("req-low-balance", int64(22)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`UPDATE users`).
		WithArgs(1.00, int64(11)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT EXISTS`).
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectRollback()

	_, err = repo.Apply(context.Background(), &service.UsageBillingCommand{
		RequestID:   "req-low-balance",
		APIKeyID:    22,
		UserID:      11,
		BalanceCost: 1.00,
	})
	require.ErrorIs(t, err, service.ErrInsufficientBalance)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryApply_SubscriptionFinalLimitRollbackOnDailyOverage(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewUsageBillingRepository(nil, db)
	subscriptionID := int64(55)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO usage_billing_dedup`).
		WithArgs("req-sub-overage", int64(22), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(8)))
	mock.ExpectQuery(`SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WithArgs("req-sub-overage", int64(22)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(`UPDATE user_subscriptions`).
		WithArgs(1.00, subscriptionID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`SELECT\s+us\.daily_usage_usd`).
		WithArgs(subscriptionID).
		WillReturnRows(sqlmock.NewRows([]string{
			"daily_usage_usd",
			"weekly_usage_usd",
			"monthly_usage_usd",
			"daily_limit_usd",
			"weekly_limit_usd",
			"monthly_limit_usd",
		}).AddRow(9.99, 0.00, 0.00, 10.00, 0.00, 0.00))
	mock.ExpectRollback()

	_, err = repo.Apply(context.Background(), &service.UsageBillingCommand{
		RequestID:        "req-sub-overage",
		APIKeyID:         22,
		UserID:           11,
		SubscriptionID:   &subscriptionID,
		SubscriptionCost: 1.00,
	})
	require.ErrorIs(t, err, service.ErrDailyLimitExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

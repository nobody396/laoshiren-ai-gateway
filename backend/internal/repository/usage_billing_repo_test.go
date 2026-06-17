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
	mock.ExpectQuery(`SELECT id, user_id, group_id, COALESCE\(notes, ''\)`).
		WithArgs(subscriptionID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "group_id", "notes"}).
			AddRow(subscriptionID, int64(11), int64(33), ""))
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

func TestUsageBillingRepositoryApply_SharedSubscriptionIgnoresNilWeeklyLimit(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewUsageBillingRepository(nil, db)
	subscriptionID := int64(55)
	userID := int64(11)
	marker := "redeem:monthly-only"
	explicitNeedle := service.SubscriptionSharedQuotaNoteKey + marker
	legacyNeedle := "通过兑换码 monthly-only 兑换"

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO usage_billing_dedup`).
		WithArgs("req-shared-nil-weekly", int64(22), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
	mock.ExpectQuery(`SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WithArgs("req-shared-nil-weekly", int64(22)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT id, user_id, group_id, COALESCE\(notes, ''\)`).
		WithArgs(subscriptionID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "group_id", "notes"}).
			AddRow(subscriptionID, userID, int64(7), explicitNeedle))
	mock.ExpectQuery(`SELECT\s+us\.id,\s+us\.user_id,\s+us\.group_id`).
		WithArgs(userID, service.SubscriptionStatusActive, explicitNeedle, legacyNeedle).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"user_id",
			"group_id",
			"daily_usage_usd",
			"weekly_usage_usd",
			"monthly_usage_usd",
			"daily_limit_usd",
			"weekly_limit_usd",
			"monthly_limit_usd",
		}).
			AddRow(subscriptionID, userID, int64(7), 0.00, 999.00, 449.00, nil, nil, 450.00).
			AddRow(int64(56), userID, int64(11), 0.00, 998.00, 448.00, nil, nil, 450.00))
	mock.ExpectQuery(`UPDATE user_subscriptions us\s+SET`).
		WithArgs(userID, service.SubscriptionStatusActive, explicitNeedle, 0.50, 999.50, 449.50, legacyNeedle).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "group_id"}).
			AddRow(userID, int64(7)).
			AddRow(userID, int64(11)))
	mock.ExpectCommit()

	result, err := repo.Apply(context.Background(), &service.UsageBillingCommand{
		RequestID:        "req-shared-nil-weekly",
		APIKeyID:         22,
		UserID:           userID,
		SubscriptionID:   &subscriptionID,
		SubscriptionCost: 0.50,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Len(t, result.SubscriptionUsageUpdates, 2)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryApply_SharedSubscriptionStillRejectsMonthlyOverageWithNilWeeklyLimit(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	repo := NewUsageBillingRepository(nil, db)
	subscriptionID := int64(55)
	userID := int64(11)
	marker := "redeem:monthly-only"
	explicitNeedle := service.SubscriptionSharedQuotaNoteKey + marker
	legacyNeedle := "通过兑换码 monthly-only 兑换"

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO usage_billing_dedup`).
		WithArgs("req-shared-monthly-overage", int64(22), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(10)))
	mock.ExpectQuery(`SELECT request_fingerprint\s+FROM usage_billing_dedup_archive`).
		WithArgs("req-shared-monthly-overage", int64(22)).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(`SELECT id, user_id, group_id, COALESCE\(notes, ''\)`).
		WithArgs(subscriptionID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "group_id", "notes"}).
			AddRow(subscriptionID, userID, int64(7), explicitNeedle))
	mock.ExpectQuery(`SELECT\s+us\.id,\s+us\.user_id,\s+us\.group_id`).
		WithArgs(userID, service.SubscriptionStatusActive, explicitNeedle, legacyNeedle).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"user_id",
			"group_id",
			"daily_usage_usd",
			"weekly_usage_usd",
			"monthly_usage_usd",
			"daily_limit_usd",
			"weekly_limit_usd",
			"monthly_limit_usd",
		}).
			AddRow(subscriptionID, userID, int64(7), 0.00, 999.00, 449.80, nil, nil, 450.00).
			AddRow(int64(56), userID, int64(11), 0.00, 998.00, 449.70, nil, nil, 450.00))
	mock.ExpectRollback()

	_, err = repo.Apply(context.Background(), &service.UsageBillingCommand{
		RequestID:        "req-shared-monthly-overage",
		APIKeyID:         22,
		UserID:           userID,
		SubscriptionID:   &subscriptionID,
		SubscriptionCost: 0.50,
	})
	require.ErrorIs(t, err, service.ErrMonthlyLimitExceeded)
	require.NoError(t, mock.ExpectationsWereMet())
}

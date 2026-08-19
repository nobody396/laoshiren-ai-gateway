package repository

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageBillingRepositoryApply_BalanceFinalLimitClampsToZeroOnInsufficientFunds(t *testing.T) {
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
		WithArgs(int64(1_000_000), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"new_balance", "deducted_amount"}).AddRow(0.00, 0.01))
	mock.ExpectCommit()

	result, err := repo.Apply(context.Background(), &service.UsageBillingCommand{
		RequestID:   "req-low-balance",
		APIKeyID:    22,
		UserID:      11,
		BalanceCost: 1.00,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.NewBalance)
	require.InDelta(t, 0.00, *result.NewBalance, 0.000001)
	require.Equal(t, int64(10_000), result.BalanceDeductedMicros)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryApply_SubscriptionFinalLimitCapsDailyOverage(t *testing.T) {
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
	mock.ExpectExec(`UPDATE user_subscriptions us\s+SET`).
		WithArgs(1.00, subscriptionID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	result, err := repo.Apply(context.Background(), &service.UsageBillingCommand{
		RequestID:        "req-sub-overage",
		APIKeyID:         22,
		UserID:           11,
		SubscriptionID:   &subscriptionID,
		SubscriptionCost: 1.00,
	})
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Len(t, result.SubscriptionUsageUpdates, 1)
	require.Equal(t, int64(11), result.SubscriptionUsageUpdates[0].UserID)
	require.Equal(t, int64(33), result.SubscriptionUsageUpdates[0].GroupID)
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

func TestSettleUsageBillingSelfPoolPostsFixedCashCommission(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	mock.ExpectQuery(`SELECT status, risk_status\s+FROM agent_principals`).
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "risk_status"}).
			AddRow("active", "clear"))
	mock.ExpectExec(`INSERT INTO agent_cash_commission_entries`).
		WithArgs(
			int64(11),
			int64(1_000_000),
			int64(10_000_000),
			service.AffiliateAgentPoolRateBPS,
			"posted",
			int64(99),
			"confirmed:99:self-cash",
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	settlement, err := settleUsageBillingSelfPool(
		context.Background(),
		tx,
		99,
		11,
		11,
		10_000_000,
		0,
		service.AffiliateAgentPoolRateBPS,
	)
	require.NoError(t, err)
	require.Zero(t, settlement.CustomerRebateMicros)
	require.Equal(t, int64(1_000_000), settlement.AgentCommissionMicros)
	mock.ExpectCommit()
	require.NoError(t, tx.Commit())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettleUsageBillingSelfPoolHoldsRiskAndStopsTerminated(t *testing.T) {
	t.Run("risk hold", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		mock.ExpectBegin()
		tx, err := db.BeginTx(context.Background(), nil)
		require.NoError(t, err)
		mock.ExpectQuery(`SELECT status, risk_status\s+FROM agent_principals`).
			WithArgs(int64(12)).
			WillReturnRows(sqlmock.NewRows([]string{"status", "risk_status"}).
				AddRow("suspended", "review"))
		mock.ExpectExec(`INSERT INTO agent_cash_commission_entries`).
			WithArgs(
				int64(12),
				int64(500_000),
				int64(5_000_000),
				service.AffiliateAgentPoolRateBPS,
				"risk_hold",
				int64(100),
				"confirmed:100:self-cash",
			).
			WillReturnResult(sqlmock.NewResult(0, 1))

		settlement, err := settleUsageBillingSelfPool(
			context.Background(),
			tx,
			100,
			12,
			12,
			5_000_000,
			0,
			service.AffiliateAgentPoolRateBPS,
		)
		require.NoError(t, err)
		require.Zero(t, settlement.AgentCommissionMicros)
		mock.ExpectCommit()
		require.NoError(t, tx.Commit())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("terminated", func(t *testing.T) {
		db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
		require.NoError(t, err)
		defer func() { _ = db.Close() }()

		mock.ExpectBegin()
		tx, err := db.BeginTx(context.Background(), nil)
		require.NoError(t, err)
		mock.ExpectQuery(`SELECT status, risk_status\s+FROM agent_principals`).
			WithArgs(int64(13)).
			WillReturnRows(sqlmock.NewRows([]string{"status", "risk_status"}).
				AddRow("terminated", "blocked"))

		settlement, err := settleUsageBillingSelfPool(
			context.Background(),
			tx,
			101,
			13,
			13,
			5_000_000,
			0,
			service.AffiliateAgentPoolRateBPS,
		)
		require.NoError(t, err)
		require.Zero(t, settlement.AgentCommissionMicros)
		mock.ExpectCommit()
		require.NoError(t, tx.Commit())
		require.NoError(t, mock.ExpectationsWereMet())
	})
}

func TestSettleUsageBillingSelfPoolRejectsAnyNonSelfOrSplitPool(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)

	_, err = settleUsageBillingSelfPool(
		context.Background(),
		tx,
		102,
		14,
		15,
		5_000_000,
		0,
		service.AffiliateAgentPoolRateBPS,
	)
	require.Error(t, err)
	_, err = settleUsageBillingSelfPool(
		context.Background(),
		tx,
		102,
		14,
		14,
		5_000_000,
		500,
		500,
	)
	require.Error(t, err)
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUsageBillingRepositoryApply_SharedSubscriptionCapsMonthlyOverageWithNilWeeklyLimit(t *testing.T) {
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
	mock.ExpectQuery(`UPDATE user_subscriptions us\s+SET`).
		WithArgs(userID, service.SubscriptionStatusActive, explicitNeedle, legacyNeedle, false, false, true).
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "group_id"}).
			AddRow(userID, int64(7)).
			AddRow(userID, int64(11)))
	mock.ExpectCommit()

	result, err := repo.Apply(context.Background(), &service.UsageBillingCommand{
		RequestID:        "req-shared-monthly-overage",
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

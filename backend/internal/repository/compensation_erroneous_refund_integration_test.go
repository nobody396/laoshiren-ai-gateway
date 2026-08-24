//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestErroneousChargeRefundIsExactSeparateAndIdempotentForBalanceAndBuilderPass(t *testing.T) {
	ctx := service.ContextWithRBACActorSuperAdmin(context.Background(), true)
	client := testEntClient(t)
	settings := NewSettingRepository(integrationEntClient)
	execution := service.NewCompensationExecutionService(integrationDB, settings, service.NewCompensationControlService(integrationDB, settings, nil), nil)
	balanceUser := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("refund-balance-%s@example.com", uuid.NewString()), PasswordHash: "hash", Balance: 10})
	balanceKey := mustCreateApiKey(t, client, &service.APIKey{UserID: balanceUser.ID, Key: "sk-refund-" + uuid.NewString(), Name: "refund"})
	account := mustCreateAccount(t, client, &service.Account{Name: "refund-account-" + uuid.NewString(), Type: service.AccountTypeAPIKey})
	var balanceUsageID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,total_cost,actual_cost,billing_type,created_at) VALUES($1,$2,$3,$4,'refund-model',1.25,1.25,0,NOW()) RETURNING id`, balanceUser.ID, balanceKey.ID, account.ID, uuid.NewString()).Scan(&balanceUsageID))
	_, err := integrationDB.ExecContext(ctx, `INSERT INTO billing_usage_entries(usage_log_id,user_id,api_key_id,billing_type,applied,delta_usd) VALUES($1,$2,$3,0,TRUE,-1.25)`, balanceUsageID, balanceUser.ID, balanceKey.ID)
	require.NoError(t, err)
	_, err = execution.RefundErroneousCharge(context.Background(), service.ErroneousChargeRefundCommand{IdempotencyKey: "refund-non-owner-" + uuid.NewString(), UsageLogID: balanceUsageID, UserID: balanceUser.ID, Reason: "must fail", ActorUserID: 42})
	require.ErrorContains(t, err, "required")
	balanceRefund, err := execution.RefundErroneousCharge(ctx, service.ErroneousChargeRefundCommand{IdempotencyKey: "refund-balance-" + uuid.NewString(), UsageLogID: balanceUsageID, UserID: balanceUser.ID, Reason: "confirmed erroneous successful-charge classification", ActorUserID: 42})
	require.NoError(t, err)
	require.Equal(t, int64(1_250_000), balanceRefund.AmountMicros)
	require.Equal(t, "balance_lot", balanceRefund.AssetType)
	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance::double precision FROM users WHERE id=$1`, balanceUser.ID).Scan(&balance))
	require.InDelta(t, 11.25, balance, 1e-9)
	duplicate, err := execution.RefundErroneousCharge(ctx, service.ErroneousChargeRefundCommand{IdempotencyKey: balanceRefund.IdempotencyKey, UsageLogID: balanceUsageID, UserID: balanceUser.ID, Reason: "confirmed erroneous successful-charge classification", ActorUserID: 42})
	require.NoError(t, err)
	require.Equal(t, balanceRefund.ID, duplicate.ID)
	_, err = execution.RefundErroneousCharge(ctx, service.ErroneousChargeRefundCommand{IdempotencyKey: balanceRefund.IdempotencyKey, UsageLogID: balanceUsageID, UserID: balanceUser.ID, Reason: "different reason", ActorUserID: 42})
	require.ErrorContains(t, err, "different input")
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT balance::double precision FROM users WHERE id=$1`, balanceUser.ID).Scan(&balance))
	require.InDelta(t, 11.25, balance, 1e-9)
	builderUser := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("refund-builder-%s@example.com", uuid.NewString()), PasswordHash: "hash"})
	builderKey := mustCreateApiKey(t, client, &service.APIKey{UserID: builderUser.ID, Key: "sk-refund-" + uuid.NewString(), Name: "refund-builder"})
	var groupID, subscriptionID, cycleID, builderUsageID, consumptionID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO groups(name,rate_multiplier,subscription_type) VALUES($1,1,'credit') RETURNING id`, `refund-builder-`+uuid.NewString()).Scan(&groupID))
	now := time.Now().UTC()
	expires := now.Add(30 * 24 * time.Hour)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO user_subscriptions(user_id,group_id,starts_at,expires_at,status,assigned_by) VALUES($1,$2,$3,$4,'active',$1) RETURNING id`, builderUser.ID, groupID, now.Add(-time.Hour), expires).Scan(&subscriptionID))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO monthly_entitlement_cycles(user_id,source_type,source_key,product_code,sale_price_micros,credit_limit_micros,used_credit_micros,confirmed_consumption_micros,starts_at,ends_at) VALUES($1,'paid_redeem',$2,'builder-pass',2000000,2000000,1250000,500000,$3,$4) RETURNING id`, builderUser.ID, "refund-paid-cycle-"+uuid.NewString(), now.Add(-time.Hour), expires).Scan(&cycleID))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO monthly_entitlement_cycle_subscriptions(cycle_id,user_subscription_id,group_id) VALUES($1,$2,$3)`, cycleID, subscriptionID, groupID)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,total_cost,actual_cost,billing_type,subscription_id,group_id,created_at) VALUES($1,$2,$3,$4,'refund-model',1.25,1.25,1,$5,$6,NOW()) RETURNING id`, builderUser.ID, builderKey.ID, account.ID, uuid.NewString(), subscriptionID, groupID).Scan(&builderUsageID))
	_, err = integrationDB.ExecContext(ctx, `INSERT INTO billing_usage_entries(usage_log_id,user_id,api_key_id,subscription_id,billing_type,applied,delta_usd) VALUES($1,$2,$3,$4,1,TRUE,-1.25)`, builderUsageID, builderUser.ID, builderKey.ID, subscriptionID)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `INSERT INTO monthly_entitlement_consumptions(cycle_id,user_id,usage_log_id,usage_event_key,confirmed_amount_micros,created_at) VALUES($1,$2,$3,$4,500000,NOW()) RETURNING id`, cycleID, builderUser.ID, builderUsageID, "refund-consumption-"+uuid.NewString()).Scan(&consumptionID))
	builderRefund, err := execution.RefundErroneousCharge(ctx, service.ErroneousChargeRefundCommand{IdempotencyKey: "refund-builder-" + uuid.NewString(), UsageLogID: builderUsageID, UserID: builderUser.ID, Reason: "confirmed erroneous subscription charge", ActorUserID: 42})
	require.NoError(t, err)
	require.Equal(t, "monthly_entitlement_cycle", builderRefund.AssetType)
	var used, confirmed, reversed int64
	var cycleEnd time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT used_credit_micros,confirmed_consumption_micros,ends_at FROM monthly_entitlement_cycles WHERE id=$1`, cycleID).Scan(&used, &confirmed, &cycleEnd))
	require.Zero(t, used)
	require.Zero(t, confirmed)
	require.True(t, cycleEnd.Equal(expires), "erroneous refund must not alter validity")
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT confirmed_amount_micros FROM monthly_entitlement_consumption_reversals WHERE consumption_id=$1`, consumptionID).Scan(&reversed))
	require.Equal(t, int64(500000), reversed)
	tierService := service.NewCustomerTierService(integrationDB, settings)
	evaluation, err := tierService.EvaluateUserAt(ctx, builderUser.ID, now.Add(time.Minute))
	require.NoError(t, err)
	require.Zero(t, evaluation.Evidence.BuilderPassConsumptionMicros, "reversed Builder Pass consumption cannot remain in paid-consumption evidence")
	cleanupErroneousRefundFixture(t, []int64{balanceUser.ID, builderUser.ID}, []int64{balanceKey.ID, builderKey.ID}, account.ID, groupID, subscriptionID, []int64{balanceUsageID, builderUsageID}, []int64{balanceRefund.ID, builderRefund.ID})
}

func cleanupErroneousRefundFixture(t *testing.T, userIDs, keyIDs []int64, accountID, groupID, subscriptionID int64, usageIDs, refundIDs []int64) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		tx, err := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		for _, table := range []string{"monthly_entitlement_consumption_reversals", "erroneous_charge_refunds", "customer_tier_history", "customer_tier_evaluations", "compensation_group_weight_versions"} {
			_, err = tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s DISABLE TRIGGER ALL`, table))
			require.NoError(t, err)
		}
		_, _ = tx.ExecContext(ctx, `DELETE FROM monthly_entitlement_consumption_reversals WHERE refund_id=ANY($1)`, pq.Array(refundIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM erroneous_charge_refunds WHERE id=ANY($1)`, pq.Array(refundIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM customer_tier_current WHERE user_id=ANY($1)`, pq.Array(userIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM customer_tier_history WHERE user_id=ANY($1)`, pq.Array(userIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM customer_tier_evaluations WHERE user_id=ANY($1)`, pq.Array(userIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM compensation_group_weight_versions WHERE group_id=$1`, groupID)
		for _, table := range []string{"monthly_entitlement_consumption_reversals", "erroneous_charge_refunds", "customer_tier_history", "customer_tier_evaluations", "compensation_group_weight_versions"} {
			_, err = tx.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ENABLE TRIGGER ALL`, table))
			require.NoError(t, err)
		}
		_, _ = tx.ExecContext(ctx, `DELETE FROM account_change_records WHERE user_id=ANY($1) AND reason='erroneous_charge_refund'`, pq.Array(userIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM balance_lots WHERE user_id=ANY($1) AND source_type='erroneous_charge_refund'`, pq.Array(userIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM monthly_entitlement_consumptions WHERE usage_log_id=ANY($1)`, pq.Array(usageIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM monthly_entitlement_cycle_subscriptions WHERE user_subscription_id=$1`, subscriptionID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM monthly_entitlement_cycles WHERE user_id=ANY($1)`, pq.Array(userIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM billing_usage_entries WHERE usage_log_id=ANY($1)`, pq.Array(usageIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM usage_logs WHERE id=ANY($1)`, pq.Array(usageIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM user_subscriptions WHERE id=$1`, subscriptionID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM api_keys WHERE id=ANY($1)`, pq.Array(keyIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM users WHERE id=ANY($1)`, pq.Array(userIDs))
		_, _ = tx.ExecContext(ctx, `DELETE FROM groups WHERE id=$1`, groupID)
		_, _ = tx.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, accountID)
		require.NoError(t, tx.Commit())
	})
}

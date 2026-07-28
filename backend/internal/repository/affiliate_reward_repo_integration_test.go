//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateRewardRepository_FirstPaidFivePlusFiveT0IsIdempotent(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateRewardRepository(client, integrationDB)
	rewardService := service.NewAffiliateRewardService(repo)
	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-reward-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-reward-invitee-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_bindings (
			customer_user_id, inviter_user_id, binding_kind,
			customer_rebate_rate_snapshot_bps,
			agent_commission_rate_snapshot_bps
		)
		VALUES ($1, $2, 'ordinary', 0, 0)
	`, invitee.ID, inviter.ID)
	require.NoError(t, err)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	tx, err := client.Tx(ctx)
	require.NoError(t, err)
	txCtx := dbent.NewTxContext(ctx, tx)
	result, err := rewardService.ProcessFirstPaidPurchase(txCtx, service.AffiliateFirstPaidPurchaseInput{
		UserID:       invitee.ID,
		PurchaseType: service.AffiliatePurchaseBalanceTopup,
		SourceID:     7001,
		PurchaseKey:  "integration-topup:7001",
		AmountMicros: 100_000_000,
		OccurredAt:   time.Now(),
	})
	require.NoError(t, err)
	require.True(t, result.ProgramLive)
	require.True(t, result.Claimed)
	require.Equal(t, int64(5_000_000), result.OrdinaryReferralMicros)
	require.Equal(t, int64(5_000_000), result.OrdinaryInviteeMicros)
	require.NoError(t, tx.Commit())

	var inviterBalance, inviteeBalance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", inviter.ID).Scan(&inviterBalance))
	require.InDelta(t, 5, inviterBalance, 0.000001)
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", invitee.ID).Scan(&inviteeBalance))
	require.InDelta(t, 5, inviteeBalance, 0.000001)

	var inviterPostedCount, inviteePostedCount, pendingCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE reward_type='ordinary_referral' AND status='posted'),
			COUNT(*) FILTER (WHERE reward_type='ordinary_invitee' AND status='posted'),
			COUNT(*) FILTER (WHERE status='pending')
		FROM affiliate_reward_entries
		WHERE consumer_user_id=$1
	`, invitee.ID).Scan(&inviterPostedCount, &inviteePostedCount, &pendingCount))
	require.Equal(t, 1, inviterPostedCount)
	require.Equal(t, 1, inviteePostedCount)
	require.Zero(t, pendingCount)

	commissionRepo := NewCommissionRepository(client, integrationDB)
	referralTotal, err := commissionRepo.SumByBeneficiaryTypeAndPeriod(
		ctx,
		inviter.ID,
		service.CommissionTypeFirstRechargeReferral,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.InDelta(t, 5, referralTotal, 0.000001)

	tx, err = client.Tx(ctx)
	require.NoError(t, err)
	txCtx = dbent.NewTxContext(ctx, tx)
	second, err := rewardService.ProcessFirstPaidPurchase(txCtx, service.AffiliateFirstPaidPurchaseInput{
		UserID:       invitee.ID,
		PurchaseType: service.AffiliatePurchaseBalanceTopup,
		SourceID:     7002,
		PurchaseKey:  "integration-topup:7002",
		AmountMicros: 200_000_000,
		OccurredAt:   time.Now(),
	})
	require.NoError(t, err)
	require.False(t, second.Claimed)
	require.NoError(t, tx.Commit())

	var rewardCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM affiliate_reward_entries
		WHERE consumer_user_id=$1
	`, invitee.ID).Scan(&rewardCount))
	require.Equal(t, 2, rewardCount)
}

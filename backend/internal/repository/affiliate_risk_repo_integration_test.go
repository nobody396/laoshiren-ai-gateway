//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateRiskRepository_ReleasesHoldsAndReversesExactlyOnce(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	admin := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-risk-admin-%d@example.com", time.Now().UnixNano()),
		Role:  service.RoleAdmin,
	})
	agent := createActiveAffiliatePaymentAgent(t, ctx, client, "affiliate-risk-agent")
	customer := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-risk-customer-%d@example.com", time.Now().UnixNano()),
	})

	var eventID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id, direct_agent_id, event_type,
			amount_micros, source_type, source_id,
			event_key, occurred_at, metadata
		)
		VALUES (
			$1, $2, 'confirmed_consumption',
			100000000, 'integration', 1,
			$3, NOW(), jsonb_build_object('program_mode', 'live')
		)
		RETURNING id
	`, customer.ID, agent.ID, fmt.Sprintf("risk-event:%d", time.Now().UnixNano())).Scan(&eventID))
	require.NoError(t, insertAffiliateRiskIntegrationHolds(ctx, eventID, agent.ID, customer.ID))

	repo := NewAffiliateRiskRepository(integrationDB)
	risk := service.NewAffiliateRiskService(repo)
	blocked, err := risk.SetAgentRisk(
		ctx,
		agent.ID,
		service.AffiliateRiskStatusBlocked,
		"异常账号关联",
		admin.ID,
	)
	require.NoError(t, err)
	require.Equal(t, service.AffiliateRiskStatusClear, blocked.PreviousRiskStatus)
	require.Equal(t, service.AffiliateRiskStatusBlocked, blocked.NextRiskStatus)

	principals, err := risk.List(ctx, 100)
	require.NoError(t, err)
	var listed *service.AffiliateRiskPrincipal
	for index := range principals {
		if principals[index].AgentID == agent.ID {
			listed = &principals[index]
			break
		}
	}
	require.NotNil(t, listed)
	require.Equal(t, int64(5_000_000), listed.HeldRewardMicros)
	require.Equal(t, int64(5_000_000), listed.HeldCashMicros)

	cleared, err := risk.SetAgentRisk(
		ctx,
		agent.ID,
		service.AffiliateRiskStatusClear,
		"人工复核通过",
		admin.ID,
	)
	require.NoError(t, err)
	require.Equal(t, int32(1), cleared.ReleasedRewardCount)
	require.Equal(t, int64(5_000_000), cleared.ReleasedRewardMicros)
	require.Equal(t, int32(1), cleared.ReleasedCashCount)
	require.Equal(t, int64(5_000_000), cleared.ReleasedCashMicros)

	var balance float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT balance FROM users WHERE id = $1
	`, customer.ID).Scan(&balance))
	require.InDelta(t, 5, balance, 0.000001)

	reversal, err := risk.ReversePerformanceEvent(ctx, eventID, "订单退款", admin.ID)
	require.NoError(t, err)
	require.Equal(t, int64(5_000_000), reversal.ReversedRewardMicros)
	require.Equal(t, int64(5_000_000), reversal.ReversedCashMicros)

	second, err := risk.ReversePerformanceEvent(ctx, eventID, "重复请求不应重复扣款", admin.ID)
	require.NoError(t, err)
	require.Equal(t, reversal.ID, second.ID)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT balance FROM users WHERE id = $1
	`, customer.ID).Scan(&balance))
	require.InDelta(t, 0, balance, 0.000001)

	var cashBalance int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_micros), 0)::bigint
		FROM agent_cash_commission_entries
		WHERE agent_id = $1
			AND posting_status = 'posted'
	`, agent.ID).Scan(&cashBalance))
	require.Zero(t, cashBalance)

	var reversalEvents, rewardReversals, cashReversals int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM affiliate_performance_events
		WHERE source_type = 'performance_reversal'
			AND source_id = $1
	`, eventID).Scan(&reversalEvents))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM affiliate_reward_entries
		WHERE reward_type = 'reversal'
			AND source_id = $1
	`, reversal.ReversalEventID).Scan(&rewardReversals))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM agent_cash_commission_entries
		WHERE entry_type = 'reversal'
			AND source_id = $1
	`, reversal.ReversalEventID).Scan(&cashReversals))
	require.Equal(t, 1, reversalEvents)
	require.Equal(t, 1, rewardReversals)
	require.Equal(t, 1, cashReversals)
}

func insertAffiliateRiskIntegrationHolds(
	ctx context.Context,
	eventID int64,
	agentID int64,
	customerID int64,
) error {
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_reward_entries (
			beneficiary_user_id, consumer_user_id,
			reward_type, amount_micros, source_amount_micros,
			rate_bps, status, available_at,
			source_type, source_id, idempotency_key
		)
		VALUES (
			$1, $1,
			'customer_rebate', 5000000, 100000000,
			500, 'risk_hold', NOW(),
			'confirmed_consumption', $2, $3
		)
	`,
		customerID,
		eventID,
		fmt.Sprintf("risk-reward:%d", eventID),
	)
	if err != nil {
		return err
	}
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, consumer_user_id, entry_type,
			amount_micros, source_amount_micros,
			customer_rebate_rate_bps, agent_commission_rate_bps,
			posting_status, source_type, source_id,
			idempotency_key, occurred_at
		)
		VALUES (
			$1, $2, 'earned',
			5000000, 100000000,
			500, 500,
			'risk_hold', 'confirmed_consumption', $3,
			$4, NOW()
		)
	`,
		agentID,
		customerID,
		eventID,
		fmt.Sprintf("risk-cash:%d", eventID),
	)
	return err
}

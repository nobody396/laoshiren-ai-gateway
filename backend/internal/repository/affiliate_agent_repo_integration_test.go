//go:build integration

package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAffiliateAgentRepository_QualifiesAppliesReviewsAndPreservesUpstream(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	agentService := service.NewAffiliateAgentService(repo)
	reviewEmailQueue := &activationEmailQueueFake{}
	agentService.SetActivationNotificationDeps(activationUserLookupFake{}, activationSettingsFake{}, reviewEmailQueue)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	upstream := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-upstream-%d@example.com", time.Now().UnixNano()),
	})
	candidate := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-candidate-%d@example.com", time.Now().UnixNano()),
	})
	operator := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-reviewer-%d@example.com", time.Now().UnixNano()),
	})
	_, err := integrationDB.ExecContext(ctx, `
		UPDATE users
		SET inviter_id = $1
		WHERE id = $2
	`, upstream.ID, candidate.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_bindings (
			customer_user_id, inviter_user_id, binding_kind,
			customer_rebate_rate_snapshot_bps,
			agent_commission_rate_snapshot_bps
		)
		VALUES ($1, $2, 'ordinary', 0, 0)
	`, candidate.ID, upstream.ID)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		direct := mustCreateUser(t, client, &service.User{
			Email: fmt.Sprintf("affiliate-direct-%d-%d@example.com", time.Now().UnixNano(), i),
		})
		_, err = integrationDB.ExecContext(ctx, `
			INSERT INTO affiliate_bindings (
				customer_user_id, inviter_user_id, binding_kind,
				customer_rebate_rate_snapshot_bps,
				agent_commission_rate_snapshot_bps
			)
			VALUES ($1, $2, 'ordinary', 0, 0)
		`, direct.ID, candidate.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `
			INSERT INTO affiliate_performance_events (
				user_id, direct_agent_id, event_type, amount_micros,
				source_type, source_id, event_key, occurred_at, metadata
			)
			VALUES (
				$1, $2, 'confirmed_consumption', 200000000,
				'integration', $3, $4, NOW(),
				'{"program_mode":"live"}'::jsonb
			)
		`, direct.ID, candidate.ID, i+1, fmt.Sprintf("qualification:%d:%d", candidate.ID, i))
		require.NoError(t, err)
	}

	qualification, err := repo.GetAgentQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.True(t, qualification.DirectRouteQualified)
	require.False(t, qualification.SelfRouteQualified)
	require.True(t, qualification.Qualified)
	require.False(t, qualification.CanActivate)
	require.True(t, qualification.CanApply)
	require.Equal(t, "direct_team", qualification.QualificationRoute)
	require.Equal(t, int32(5), qualification.ValidDirectUserCount)
	require.Equal(t, int64(1_000_000_000), qualification.DirectTeamConsumptionMicros)

	// The repo-level submit preserves the legacy pending flow that an admin
	// reviews manually; the service-level Apply auto-activates instead.
	application, err := repo.SubmitAgentApplication(ctx, candidate.ID, "申请成为合伙人")
	require.NoError(t, err)
	require.Equal(t, "pending_review", application.Status)
	require.Zero(t, application.SelfConsumptionMicros)
	require.Equal(t, int64(1_000_000_000), application.DirectTeamConsumptionMicros)
	require.Equal(t, int64(1_000_000_000), application.CombinedConsumptionMicros)

	review, err := agentService.ReviewApplication(ctx, application.ID, true, "资料与消费确认无误", operator.ID)
	require.NoError(t, err)
	require.NotNil(t, review.Activation)
	activation := review.Activation
	require.Equal(t, "active", activation.Qualification.AgentStatus)
	require.False(t, activation.Qualification.CanActivate)
	require.False(t, activation.Qualification.CanApply)
	require.True(t, activation.DefaultLink.IsDefault)
	require.Equal(t, service.AffiliateDefaultCustomerRebateRateBPS, activation.DefaultLink.CustomerRebateRateBPS)
	require.Equal(t, int32(500), activation.DefaultLink.AgentCommissionRateBPS)

	var role string
	var inviterID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT role, inviter_id
		FROM users
		WHERE id = $1
	`, candidate.ID).Scan(&role, &inviterID))
	require.Equal(t, service.RoleAgent, role)
	require.Equal(t, upstream.ID, inviterID, "activation must preserve the agent's own upstream edge")

	var principalStatus string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT status
		FROM agent_principals
		WHERE agent_id = $1
	`, candidate.ID).Scan(&principalStatus))
	require.Equal(t, "active", principalStatus)

	var defaultCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM affiliate_links
		WHERE agent_id = $1
			AND is_default = TRUE
	`, candidate.ID).Scan(&defaultCount))
	require.Equal(t, 1, defaultCount)

	// Manual approval fires the same activation side effects as auto activation.
	requireAgentActivatedNoticeCount(t, ctx, candidate.ID, 1)
	require.Equal(t, 1, reviewEmailQueue.count(), "manual approval must enqueue the activation email once")
}

func TestAffiliateAgentRepository_OperationsSummaryAndPartnerPerformance(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	agent := createActiveAffiliatePaymentAgent(t, ctx, client, "affiliate-performance-agent")
	direct := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-performance-direct-%d@example.com", time.Now().UnixNano()),
	})
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_bindings (
			customer_user_id, inviter_user_id, binding_kind,
			customer_rebate_rate_snapshot_bps, agent_commission_rate_snapshot_bps
		) VALUES ($1, $2, 'ordinary', 0, 0)
	`, direct.ID, agent.ID)
	require.NoError(t, err)

	key := uuid.NewString()
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_key, original_amount_micros,
			remaining_amount_micros, affiliate_eligible, occurred_at
		) VALUES
			($1, 'paid_topup', $3 || ':self', 10000000, 10000000, FALSE, NOW()),
			($2, 'paid_redeem', $3 || ':direct', 20000000, 20000000, FALSE, NOW())
	`, agent.ID, direct.ID, key)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO monthly_entitlement_cycles (
			user_id, source_type, source_key, product_code, sale_price_micros,
			credit_limit_micros, affiliate_eligible, starts_at, ends_at
		) VALUES ($1, 'paid_redeem', $2, 'integration-monthly', 30000000,
			30000000, FALSE, NOW(), NOW() + INTERVAL '30 days')
	`, direct.ID, key+":monthly")
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id, direct_agent_id, event_type, amount_micros,
			source_type, source_id, event_key, occurred_at, metadata
		) VALUES
			($1, $1, 'confirmed_consumption', 4000000, 'integration', 1, $3 || ':self-use', NOW(), '{"program_mode":"live"}'),
			($2, $1, 'confirmed_consumption', 7000000, 'integration', 2, $3 || ':team-use', NOW(), '{"program_mode":"live"}'),
			($2, $1, 'consumption_reversal', 2000000, 'integration', 3, $3 || ':team-reversal', NOW(), '{"program_mode":"live"}')
	`, agent.ID, direct.ID, key)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, consumer_user_id, entry_type, amount_micros, posting_status,
			source_amount_micros, customer_rebate_rate_bps, agent_commission_rate_bps,
			source_type, source_id, idempotency_key, occurred_at
		) VALUES
			($1, $2, 'earned', 1000000, 'posted', 20000000, 500, 500, 'integration', 1, $3 || ':earned', NOW()),
			($1, NULL, 'withdrawal_hold', -400000, 'posted', NULL, NULL, NULL, 'integration', 2, $3 || ':hold', NOW())
	`, agent.ID, direct.ID, key)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO agent_withdrawal_requests (
			agent_id, amount_micros, status, idempotency_key,
			payment_alipay_real_name, payment_alipay_account, payment_qr_object_key,
			requested_at, due_at, paid_at, payment_reference
		) VALUES ($1, 300000, 'paid', $2, '测试', 'test@example.com', 'integration/test.png',
			NOW(), NOW() + INTERVAL '1 day', NOW(), 'integration-ref')
	`, agent.ID, key+":withdrawal")
	require.NoError(t, err)

	repo := NewAffiliateAgentRepository(integrationDB)
	items, err := repo.ListPartnerPerformance(ctx, 500)
	require.NoError(t, err)
	var listed *service.AffiliatePartnerPerformance
	for index := range items {
		if items[index].AgentID == agent.ID {
			listed = &items[index]
			break
		}
	}
	require.NotNil(t, listed)
	require.Equal(t, int64(1), listed.DirectUserCount)
	require.Equal(t, int64(1), listed.PaidDirectUserCount)
	require.Equal(t, int64(10_000_000), listed.SelfRechargeMicros)
	require.Equal(t, int64(50_000_000), listed.DirectTeamRechargeMicros)
	require.Equal(t, int64(4_000_000), listed.SelfConsumptionMicros)
	require.Equal(t, int64(5_000_000), listed.DirectTeamConsumptionMicros)
	require.Equal(t, int64(1_000_000), listed.LifetimeEarnedMicros)
	require.Equal(t, int64(600_000), listed.AvailableCommissionMicros)
	require.Equal(t, int64(300_000), listed.PaidCommissionMicros)

	detail, err := repo.GetPartnerPerformance(ctx, agent.ID, time.Time{}, time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, detail.DirectUsers, 1)
	require.Equal(t, int64(50_000_000), detail.DirectUsers[0].RechargeMicros)
	require.Equal(t, int64(5_000_000), detail.DirectUsers[0].ConsumptionMicros)
	require.Len(t, detail.CommissionLedger, 2)
	var earned *service.AffiliatePartnerCommissionEntry
	for index := range detail.CommissionLedger {
		if detail.CommissionLedger[index].EntryType == "earned" {
			earned = &detail.CommissionLedger[index]
			break
		}
	}
	require.NotNil(t, earned)
	require.Equal(t, int64(20_000_000), earned.SourceAmountMicros)
	require.Equal(t, int32(500), earned.CustomerRebateRateBPS)
	require.Equal(t, int32(500), earned.AgentCommissionRateBPS)
	require.Equal(t, "integration", earned.SourceType)
	require.Len(t, detail.Withdrawals, 1)

	summary, err := repo.GetOperationsSummary(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, summary.ActionableTotal, int64(0))
}

func TestAffiliateAgentRepository_NewcomerOfferIsVisibleWithoutCommission(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	agent := createActiveAffiliatePaymentAgent(t, ctx, client, "affiliate-newcomer-agent")
	direct := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-newcomer-direct-%d@example.com", time.Now().UnixNano()),
	})
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_bindings (
			customer_user_id, inviter_user_id, binding_kind,
			customer_rebate_rate_snapshot_bps, agent_commission_rate_snapshot_bps
		) VALUES ($1, $2, 'ordinary', 0, 0)
	`, direct.ID, agent.ID)
	require.NoError(t, err)

	usedAt := time.Now()
	code := &service.RedeemCode{
		Code:        fmt.Sprintf("%032x", time.Now().UnixNano()),
		Type:        service.RedeemTypeBalance,
		Value:       10,
		PaidValue:   0,
		Status:      service.StatusUsed,
		UsedBy:      &direct.ID,
		UsedAt:      &usedAt,
		Purpose:     service.RedeemCodePurposeGift,
		SalesStatus: service.RedeemCodeSalesStatusGifted,
	}
	require.NoError(t, NewRedeemCodeRepository(client).Create(ctx, code))
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		cleanupQueries := []struct {
			query string
			args  []any
		}{
			{`DELETE FROM balance_lot_consumptions WHERE balance_lot_id IN (
				SELECT id FROM balance_lots
				WHERE user_id = $1 AND source_type = 'gift' AND source_id = $2
			)`, []any{direct.ID, code.ID}},
			{`DELETE FROM balance_lots
				WHERE user_id = $1 AND source_type = 'gift' AND source_id = $2`, []any{direct.ID, code.ID}},
			{`DELETE FROM native_checkout_manual_claims
				WHERE user_id = $1 AND redeem_code_id = $2`, []any{direct.ID, code.ID}},
			{`DELETE FROM redeem_codes WHERE id = $1`, []any{code.ID}},
		}
		for _, cleanup := range cleanupQueries {
			_, cleanupErr := integrationDB.ExecContext(cleanupCtx, cleanup.query, cleanup.args...)
			require.NoError(t, cleanupErr)
		}
	})
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO native_checkout_manual_claims (
			offer_code, user_id, redeem_code_id, created_at
		) VALUES ('newcomer-balance-5-to-10', $1, $2, $3)
	`, direct.ID, code.ID, usedAt)
	require.NoError(t, err)

	var lotID int64
	err = integrationDB.QueryRowContext(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_id, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, affiliate_policy, occurred_at
		) VALUES ($1, 'gift', $2, $3, 10000000, 6000000, FALSE, 'NONE', $4)
		RETURNING id
	`, direct.ID, code.ID, fmt.Sprintf("redeem:balance:%d", code.ID), usedAt).Scan(&lotID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO balance_lot_consumptions (
			balance_lot_id, user_id, usage_event_key,
			amount_micros, affiliate_eligible_amount_micros, created_at
		) VALUES ($1, $2, $3, 4000000, 0, $4)
	`, lotID, direct.ID, fmt.Sprintf("integration:newcomer:%d", code.ID), usedAt)
	require.NoError(t, err)

	repo := NewAffiliateAgentRepository(integrationDB)
	items, err := repo.ListPartnerPerformance(ctx, 500)
	require.NoError(t, err)
	var listed *service.AffiliatePartnerPerformance
	for index := range items {
		if items[index].AgentID == agent.ID {
			listed = &items[index]
			break
		}
	}
	require.NotNil(t, listed)
	require.Equal(t, int64(1), listed.PaidDirectUserCount)
	require.Equal(t, int64(5_000_000), listed.DirectTeamRechargeMicros)
	require.Equal(t, int64(4_000_000), listed.DirectTeamConsumptionMicros)
	require.Zero(t, listed.LifetimeEarnedMicros)

	detail, err := repo.GetPartnerPerformance(ctx, agent.ID, time.Time{}, time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Len(t, detail.DirectUsers, 1)
	require.Equal(t, int64(5_000_000), detail.DirectUsers[0].RechargeMicros)
	require.Equal(t, int64(4_000_000), detail.DirectUsers[0].ConsumptionMicros)
	require.Zero(t, detail.DirectUsers[0].GeneratedCommissionMicros)

	commissionRepo := NewCommissionRepository(nil, integrationDB)
	users, _, err := commissionRepo.ListInvitedUsersWithStats(
		ctx,
		agent.ID,
		pagination.PaginationParams{Page: 1, PageSize: 20},
		nil,
		nil,
	)
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.InDelta(t, 5.0, users[0].RechargedAmount, 0.000001)
	require.InDelta(t, 4.0, users[0].ConsumedAmount, 0.000001)
	require.Zero(t, users[0].CommissionAmount)
}

func TestAffiliateAgentRepository_RejectsUnqualifiedApplication(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	agentService := service.NewAffiliateAgentService(repo)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	user := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-unqualified-%d@example.com", time.Now().UnixNano()),
	})
	qualification, err := agentService.GetQualification(ctx, user.ID)
	require.NoError(t, err)
	require.False(t, qualification.Qualified)
	require.False(t, qualification.CanActivate)
	require.False(t, qualification.CanApply)

	_, err = agentService.Apply(ctx, user.ID, "")
	require.True(t, errors.Is(err, service.ErrAffiliateQualificationNotMet), "unexpected error: %v", err)

	var sideEffects int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM agent_principals WHERE agent_id = $1)
			+
			(SELECT COUNT(*) FROM affiliate_agent_applications WHERE user_id = $1)
			+
			(SELECT COUNT(*) FROM affiliate_links WHERE agent_id = $1)
	`, user.ID).Scan(&sideEffects))
	require.Zero(t, sideEffects, "viewing qualification while unqualified must not activate anything")
}

func TestAffiliateAgentRepository_RouteAOnlySumsValidDirectUsers(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	candidate := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-route-a-candidate-%d@example.com", time.Now().UnixNano()),
	})
	validDirectIDs := make([]int64, 0, 5)

	addDirectConsumption := func(index int, amountMicros int64, valid bool) int64 {
		direct := mustCreateUser(t, client, &service.User{
			Email: fmt.Sprintf("affiliate-route-a-direct-%d-%d@example.com", time.Now().UnixNano(), index),
		})
		_, err := integrationDB.ExecContext(ctx, `
			INSERT INTO affiliate_bindings (
				customer_user_id, inviter_user_id, binding_kind,
				customer_rebate_rate_snapshot_bps,
				agent_commission_rate_snapshot_bps
			)
			VALUES ($1, $2, 'ordinary', 0, 0)
		`, direct.ID, candidate.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `
			INSERT INTO affiliate_performance_events (
				user_id, direct_agent_id, event_type, amount_micros,
				source_type, source_id, event_key, occurred_at, metadata
			)
			VALUES (
				$1, $2, 'confirmed_consumption', $3,
				'integration', $4, $5, NOW(),
				'{"program_mode":"live"}'::jsonb
			)
		`, direct.ID, candidate.ID, amountMicros, index+1, "route-a:"+uuid.NewString())
		require.NoError(t, err)
		if valid {
			validDirectIDs = append(validDirectIDs, direct.ID)
		}
		return direct.ID
	}

	for i := 0; i < 5; i++ {
		addDirectConsumption(i, 180_000_000, true)
	}
	for i := 0; i < 6; i++ {
		addDirectConsumption(100+i, 19_000_000, false)
	}

	qualification, err := repo.GetAgentQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.Equal(t, int32(5), qualification.ValidDirectUserCount)
	require.Equal(t, int64(900_000_000), qualification.DirectTeamConsumptionMicros)
	require.False(t, qualification.DirectRouteQualified)
	require.False(t, qualification.Qualified)

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id, direct_agent_id, event_type, amount_micros,
			source_type, source_id, event_key, occurred_at, metadata
		)
		VALUES (
			$1, $2, 'confirmed_consumption', 100000000,
			'integration', 999, $3, NOW(),
			'{"program_mode":"live"}'::jsonb
		)
	`, validDirectIDs[0], candidate.ID, "route-a-topup:"+uuid.NewString())
	require.NoError(t, err)

	qualification, err = repo.GetAgentQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1_000_000_000), qualification.DirectTeamConsumptionMicros)
	require.True(t, qualification.DirectRouteQualified)
	require.Equal(t, "direct_team", qualification.QualificationRoute)
}

func TestAffiliateAgentRepository_QualificationUsesSelfOnlyForRouteB(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	candidate := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-self-candidate-%d@example.com", time.Now().UnixNano()),
	})
	var startedAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT started_at
		FROM affiliate_program_settings
		WHERE id = 1
	`).Scan(&startedAt))

	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_qualification_baseline_entries (
			user_id, source_key, source_type,
			confirmed_consumption_micros, cutoff_at, metadata
		)
		VALUES ($1, $2, 'manual_verified', 400000000, $3, '{"test":true}'::jsonb)
	`, candidate.ID, "qualification-self:"+uuid.NewString(), startedAt)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		direct := mustCreateUser(t, client, &service.User{
			Email: fmt.Sprintf("affiliate-combined-direct-%d-%d@example.com", time.Now().UnixNano(), i),
		})
		_, err = integrationDB.ExecContext(ctx, `
			INSERT INTO affiliate_bindings (
				customer_user_id, inviter_user_id, binding_kind,
				customer_rebate_rate_snapshot_bps,
				agent_commission_rate_snapshot_bps
			)
			VALUES ($1, $2, 'ordinary', 0, 0)
		`, direct.ID, candidate.ID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `
			INSERT INTO affiliate_qualification_baseline_entries (
				user_id, source_key, source_type,
				confirmed_consumption_micros, cutoff_at, metadata
			)
			VALUES ($1, $2, 'manual_verified', 10000000, $3, '{"test":true}'::jsonb)
		`, direct.ID, "qualification-direct:"+uuid.NewString(), startedAt)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `
			INSERT INTO affiliate_performance_events (
				user_id, event_type, amount_micros, affiliate_policy,
				source_type, source_id, event_key, occurred_at, metadata
			)
			VALUES (
				$1, 'confirmed_consumption', 90000000, 'NONE',
				'integration', $2, $3, NOW(), '{"program_mode":"live"}'::jsonb
			)
		`, direct.ID, i+1, "qualification-direct-live:"+uuid.NewString())
		require.NoError(t, err)
	}

	qualification, err := repo.GetAgentQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.Equal(t, int32(5), qualification.ValidDirectUserCount)
	require.Equal(t, int64(400_000_000), qualification.SelfConsumptionMicros)
	require.Equal(t, int64(500_000_000), qualification.DirectTeamConsumptionMicros)
	require.Equal(t, int64(900_000_000), qualification.CombinedConsumptionMicros)
	require.False(t, qualification.DirectRouteQualified)
	require.False(t, qualification.SelfRouteQualified, "direct users must not help Route B")
	require.False(t, qualification.Qualified)
	require.False(t, qualification.CanApply)

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id, event_type, amount_micros, affiliate_policy,
			source_type, source_id, event_key, occurred_at, metadata
		)
		VALUES (
			$1, 'confirmed_consumption', 100000000, 'NONE',
			'integration', 1, $2, NOW(), '{"program_mode":"live"}'::jsonb
		)
	`, candidate.ID, "qualification-self-live:"+uuid.NewString())
	require.NoError(t, err)

	qualification, err = repo.GetAgentQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.Equal(t, int64(500_000_000), qualification.SelfConsumptionMicros)
	require.False(t, qualification.DirectRouteQualified)
	require.True(t, qualification.SelfRouteQualified)
	require.Equal(t, "self_consumption", qualification.QualificationRoute)
	require.True(t, qualification.CanApply)

	application, err := repo.SubmitAgentApplication(ctx, candidate.ID, "路线 B 本人消费快照测试")
	require.NoError(t, err)
	require.Equal(t, "self_consumption", application.QualifyingRoute)
	require.Equal(t, int64(500_000_000), application.SelfConsumptionMicros)
	require.Equal(t, int64(500_000_000), application.DirectTeamConsumptionMicros)
	require.Equal(t, int64(1_000_000_000), application.CombinedConsumptionMicros)
}

func TestAffiliateAgentRepository_ListsQualifiedUsersUntilTheyApply(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	candidate := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-qualified-waiting-%d@example.com", time.Now().UnixNano()),
	})
	var startedAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT started_at
		FROM affiliate_program_settings
		WHERE id = 1
	`).Scan(&startedAt))
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_qualification_baseline_entries (
			user_id, source_key, source_type,
			confirmed_consumption_micros, cutoff_at, metadata
		)
		VALUES ($1, $2, 'manual_verified', 530000000, $3, '{"test":true}'::jsonb)
	`, candidate.ID, "qualification-candidate:"+uuid.NewString(), startedAt)
	require.NoError(t, err)

	items, err := repo.ListQualifiedCandidates(ctx, 500)
	require.NoError(t, err)
	var listed *service.AffiliateQualifiedCandidate
	for i := range items {
		if items[i].UserID == candidate.ID {
			listed = &items[i]
			break
		}
	}
	require.NotNil(t, listed)
	require.Equal(t, "self_consumption", listed.QualificationRoute)
	require.Equal(t, int64(530_000_000), listed.SelfConsumptionMicros)
	var mutationCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM agent_principals WHERE agent_id = $1)
			+
			(SELECT COUNT(*) FROM affiliate_agent_applications WHERE user_id = $1)
	`, candidate.ID).Scan(&mutationCount))
	require.Zero(t, mutationCount, "listing a qualified candidate must remain read-only")

	_, err = repo.SubmitAgentApplication(ctx, candidate.ID, "已达标待申请列表测试")
	require.NoError(t, err)

	items, err = repo.ListQualifiedCandidates(ctx, 500)
	require.NoError(t, err)
	for i := range items {
		require.NotEqual(t, candidate.ID, items[i].UserID, "pending applicants must leave the waiting list")
	}
}

func seedAffiliateSelfQualifiedUser(t *testing.T, ctx context.Context, userID int64, amountMicros int64) {
	t.Helper()
	var startedAt time.Time
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT started_at
		FROM affiliate_program_settings
		WHERE id = 1
	`).Scan(&startedAt))
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_qualification_baseline_entries (
			user_id, source_key, source_type,
			confirmed_consumption_micros, cutoff_at, metadata
		)
		VALUES ($1, $2, 'manual_verified', $3, $4, '{"test":true}'::jsonb)
	`, userID, "auto-activate:"+uuid.NewString(), amountMicros, startedAt)
	require.NoError(t, err)
}

func requireAffiliateActivationCounts(t *testing.T, ctx context.Context, userID int64, links, applications, activationEvents int) {
	t.Helper()
	var linkCount, applicationCount, eventCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM affiliate_links WHERE agent_id = $1 AND is_default = TRUE),
			(SELECT COUNT(*) FROM affiliate_agent_applications WHERE user_id = $1),
			(
				SELECT COUNT(*)
				FROM affiliate_performance_events
				WHERE user_id = $1
					AND event_type = 'agent_activated'
			)
	`, userID).Scan(&linkCount, &applicationCount, &eventCount))
	require.Equal(t, links, linkCount, "default link count")
	require.Equal(t, applications, applicationCount, "application count")
	require.Equal(t, activationEvents, eventCount, "agent_activated event count")
}

type activationEmailQueueFake struct {
	mu     sync.Mutex
	emails []string
}

func (f *activationEmailQueueFake) EnqueueAffiliateAgentActivated(email, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.emails = append(f.emails, email)
	return nil
}

func (f *activationEmailQueueFake) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.emails)
}

type activationUserLookupFake struct{}

func (activationUserLookupFake) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Email: "activated-partner@example.com"}, nil
}

type activationSettingsFake struct{}

func (activationSettingsFake) GetFrontendURL(context.Context) string {
	return "https://laoshirenai.com"
}
func (activationSettingsFake) GetSiteName(context.Context) string { return "老实人AI" }

func requireAgentActivatedNoticeCount(t *testing.T, ctx context.Context, userID int64, expected int) {
	t.Helper()
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM affiliate_agent_notices
		WHERE agent_id = $1
			AND notice_type = 'agent_activated'
	`, userID).Scan(&count))
	require.Equal(t, expected, count, "agent_activated notice count")
}

func TestAffiliateAgentRepository_AutoActivatesQualifiedUserOnQualificationView(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	agentService := service.NewAffiliateAgentService(repo)
	emailQueue := &activationEmailQueueFake{}
	agentService.SetActivationNotificationDeps(activationUserLookupFake{}, activationSettingsFake{}, emailQueue)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	candidate := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-auto-activate-%d@example.com", time.Now().UnixNano()),
	})
	seedAffiliateSelfQualifiedUser(t, ctx, candidate.ID, 530_000_000)

	qualification, err := agentService.GetQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.Equal(t, "active", qualification.AgentStatus)
	require.False(t, qualification.CanApply)
	require.NotNil(t, qualification.ActivatedAt)
	require.Equal(t, 1, emailQueue.count(), "fresh activation must enqueue the email once")
	requireAgentActivatedNoticeCount(t, ctx, candidate.ID, 1)

	var role string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT role FROM users WHERE id = $1
	`, candidate.ID).Scan(&role))
	require.Equal(t, service.RoleAgent, role)

	var principalStatus, principalDecision string
	var reviewedBy sql.NullInt64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT status, decision_note, reviewed_by
		FROM agent_principals
		WHERE agent_id = $1
	`, candidate.ID).Scan(&principalStatus, &principalDecision, &reviewedBy))
	require.Equal(t, "active", principalStatus)
	require.Equal(t, service.AffiliateAgentAutoActivationNote, principalDecision)
	require.False(t, reviewedBy.Valid, "auto activation must not impersonate a reviewer")

	var customerRate, agentRate int32
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT rv.customer_rebate_rate_bps, rv.agent_commission_rate_bps
		FROM affiliate_links l
		JOIN affiliate_link_rate_versions rv
			ON rv.link_id = l.id
			AND rv.version = l.current_rate_version
		WHERE l.agent_id = $1
			AND l.is_default = TRUE
	`, candidate.ID).Scan(&customerRate, &agentRate))
	require.Equal(t, service.AffiliateDefaultCustomerRebateRateBPS, customerRate)
	require.Equal(t, service.AffiliateAgentPoolRateBPS-service.AffiliateDefaultCustomerRebateRateBPS, agentRate)

	var applicationStatus, applicationDecision string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT status, decision_note
		FROM affiliate_agent_applications
		WHERE user_id = $1
	`, candidate.ID).Scan(&applicationStatus, &applicationDecision))
	require.Equal(t, "approved", applicationStatus)
	require.Equal(t, service.AffiliateAgentAutoActivationNote, applicationDecision)

	requireAffiliateActivationCounts(t, ctx, candidate.ID, 1, 1, 1)

	// Viewing again is a harmless no-op.
	qualification, err = agentService.GetQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.Equal(t, "active", qualification.AgentStatus)

	// Applying again keeps the legacy blocked semantics for active agents and
	// must not duplicate links, applications, or events.
	_, err = agentService.Apply(ctx, candidate.ID, "重复点击")
	require.True(t, errors.Is(err, service.ErrAffiliateAgentActivationBlocked), "unexpected error: %v", err)

	requireAffiliateActivationCounts(t, ctx, candidate.ID, 1, 1, 1)
	requireAgentActivatedNoticeCount(t, ctx, candidate.ID, 1)
	require.Equal(t, 1, emailQueue.count(), "idempotent triggers must not resend the activation email")
}

func TestAffiliateAgentRepository_AutoActivationConvergesPendingApplication(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	agentService := service.NewAffiliateAgentService(repo)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	candidate := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-auto-pending-%d@example.com", time.Now().UnixNano()),
	})
	seedAffiliateSelfQualifiedUser(t, ctx, candidate.ID, 530_000_000)

	legacy, err := repo.SubmitAgentApplication(ctx, candidate.ID, "旧流程待审申请")
	require.NoError(t, err)
	require.Equal(t, "pending_review", legacy.Status)

	qualification, err := agentService.GetQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.Equal(t, "active", qualification.AgentStatus)

	var status, decisionNote string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT status, decision_note
		FROM affiliate_agent_applications
		WHERE id = $1
	`, legacy.ID).Scan(&status, &decisionNote))
	require.Equal(t, "approved", status, "auto activation must converge the legacy pending application")
	require.Equal(t, service.AffiliateAgentAutoActivationNote, decisionNote)

	requireAffiliateActivationCounts(t, ctx, candidate.ID, 1, 1, 1)
}

func TestAffiliateAgentRepository_AutoActivationSkippedWhenRiskNotClear(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	agentService := service.NewAffiliateAgentService(repo)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	candidate := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-auto-risk-%d@example.com", time.Now().UnixNano()),
	})
	seedAffiliateSelfQualifiedUser(t, ctx, candidate.ID, 530_000_000)
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_principals (agent_id, status, risk_status)
		VALUES ($1, 'candidate', 'review')
	`, candidate.ID)
	require.NoError(t, err)

	qualification, err := agentService.GetQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.Equal(t, "review", qualification.RiskStatus)
	require.NotEqual(t, "active", qualification.AgentStatus)

	_, err = agentService.Apply(ctx, candidate.ID, "")
	require.True(t, errors.Is(err, service.ErrAffiliateAgentActivationBlocked), "unexpected error: %v", err)

	var role string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT role FROM users WHERE id = $1
	`, candidate.ID).Scan(&role))
	require.NotEqual(t, service.RoleAgent, role)
	requireAffiliateActivationCounts(t, ctx, candidate.ID, 0, 0, 0)
}

func TestAffiliateAgentRepository_ConcurrentQualificationViewsActivateOnce(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	emailQueue := &activationEmailQueueFake{}
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	candidate := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-auto-concurrent-%d@example.com", time.Now().UnixNano()),
	})
	seedAffiliateSelfQualifiedUser(t, ctx, candidate.ID, 530_000_000)

	const viewers = 4
	errCh := make(chan error, viewers)
	var wg sync.WaitGroup
	for i := 0; i < viewers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc := service.NewAffiliateAgentService(repo)
			svc.SetActivationNotificationDeps(activationUserLookupFake{}, activationSettingsFake{}, emailQueue)
			_, err := svc.GetQualification(ctx, candidate.ID)
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	qualification, err := repo.GetAgentQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.Equal(t, "active", qualification.AgentStatus)
	requireAffiliateActivationCounts(t, ctx, candidate.ID, 1, 1, 1)
	requireAgentActivatedNoticeCount(t, ctx, candidate.ID, 1)
	require.Equal(t, 1, emailQueue.count(), "concurrent activations must send exactly one email")
}

func TestAffiliateAgentRepository_DailySweepActivatesQualifiedUserOnce(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	agentService := service.NewAffiliateAgentService(repo)
	emailQueue := &activationEmailQueueFake{}
	agentService.SetActivationNotificationDeps(activationUserLookupFake{}, activationSettingsFake{}, emailQueue)
	scheduler := service.NewAffiliateAgentActivationScheduler(agentService)
	setAffiliateProgramLiveForIntegrationTest(t, ctx)

	// Qualified but never visits the partner page.
	idle := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-sweep-idle-%d@example.com", time.Now().UnixNano()),
	})
	seedAffiliateSelfQualifiedUser(t, ctx, idle.ID, 530_000_000)
	// Not qualified: below the self-consumption threshold.
	unqualified := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-sweep-unqualified-%d@example.com", time.Now().UnixNano()),
	})
	seedAffiliateSelfQualifiedUser(t, ctx, unqualified.ID, 100_000_000)
	// Qualified but under risk review: excluded from the candidate list.
	risky := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-sweep-risky-%d@example.com", time.Now().UnixNano()),
	})
	seedAffiliateSelfQualifiedUser(t, ctx, risky.ID, 530_000_000)
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_principals (agent_id, status, risk_status)
		VALUES ($1, 'candidate', 'review')
	`, risky.ID)
	require.NoError(t, err)

	scheduler.RunOnce(ctx)

	qualification, err := repo.GetAgentQualification(ctx, idle.ID)
	require.NoError(t, err)
	require.Equal(t, "active", qualification.AgentStatus)
	requireAffiliateActivationCounts(t, ctx, idle.ID, 1, 1, 1)
	requireAgentActivatedNoticeCount(t, ctx, idle.ID, 1)
	// The sweep covers every qualified candidate in the shared test database,
	// so earlier tests' leftovers may also be activated here; what matters is
	// the idle user got exactly one email.
	firstSweepEmails := emailQueue.count()
	require.GreaterOrEqual(t, firstSweepEmails, 1)

	for _, unaffected := range []int64{unqualified.ID, risky.ID} {
		other, err := repo.GetAgentQualification(ctx, unaffected)
		require.NoError(t, err)
		require.NotEqual(t, "active", other.AgentStatus)
		requireAffiliateActivationCounts(t, ctx, unaffected, 0, 0, 0)
		requireAgentActivatedNoticeCount(t, ctx, unaffected, 0)
	}

	// A second sweep the same day is a no-op: nothing newly activated, no
	// duplicate notice or email.
	scheduler.RunOnce(ctx)
	requireAffiliateActivationCounts(t, ctx, idle.ID, 1, 1, 1)
	requireAgentActivatedNoticeCount(t, ctx, idle.ID, 1)
	require.Equal(t, firstSweepEmails, emailQueue.count(), "repeat sweep must not resend activation emails")
}

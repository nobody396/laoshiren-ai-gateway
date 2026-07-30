//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestAffiliateSelfCommissionPolicyRepository_GuardsEligibilityAndAudit(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	admin := mustCreateUser(t, client, &service.User{
		Email:    fmt.Sprintf("self-commission-admin-%d@example.com", time.Now().UnixNano()),
		Username: "联盟审核员",
		Role:     service.RoleAdmin,
	})
	partner := createActiveAffiliatePaymentAgent(t, ctx, client, "self-commission-partner")
	upstream := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("self-commission-upstream-%d@example.com", time.Now().UnixNano()),
	})

	repo := NewAffiliateSelfCommissionPolicyRepository(integrationDB)
	policies := service.NewAffiliateSelfCommissionPolicyService(repo)

	enabled, err := policies.Update(
		ctx,
		partner.ID,
		true,
		0,
		"无上级合伙人本人返佣",
		admin.ID,
	)
	require.NoError(t, err)
	require.True(t, enabled.Enabled)
	require.Equal(t, service.AffiliateSelfCommissionRateBPS, enabled.RateBPS)
	require.Equal(t, int64(1), enabled.Revision)
	require.NotNil(t, enabled.EffectiveAt)
	require.False(t, enabled.HasUpstream)
	require.True(t, enabled.Eligible)
	require.Empty(t, enabled.BlockReasonCode)

	var eventCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM affiliate_agent_self_commission_events
		WHERE agent_id = $1
	`, partner.ID).Scan(&eventCount))
	require.Equal(t, 1, eventCount)

	// Same-state requests with the current revision are no-ops and must not
	// manufacture additional audit entries.
	unchanged, err := policies.Update(
		ctx,
		partner.ID,
		true,
		1,
		"重复点击",
		admin.ID,
	)
	require.NoError(t, err)
	require.Equal(t, int64(1), unchanged.Revision)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM affiliate_agent_self_commission_events
		WHERE agent_id = $1
	`, partner.ID).Scan(&eventCount))
	require.Equal(t, 1, eventCount)

	_, err = policies.Update(ctx, partner.ID, false, 0, "旧版本写入", admin.ID)
	require.ErrorIs(t, err, service.ErrAffiliateSelfCommissionRevisionConflict)

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_bindings (
			customer_user_id,
			inviter_user_id,
			binding_kind,
			customer_rebate_rate_snapshot_bps,
			agent_commission_rate_snapshot_bps
		)
		VALUES ($1, $2, 'ordinary', 0, 0)
	`, partner.ID, upstream.ID)
	requirePostgresConstraint(t, err, "affiliate_binding_self_commission_exclusion")

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE users
		SET inviter_id = $1
		WHERE id = $2
	`, upstream.ID, partner.ID)
	requirePostgresConstraint(t, err, "users_upstream_self_commission_exclusion")

	riskRepo := NewAffiliateRiskRepository(integrationDB)
	principals, err := riskRepo.ListAffiliateRiskPrincipals(ctx, 500)
	require.NoError(t, err)
	var listed *service.AffiliateRiskPrincipal
	for index := range principals {
		if principals[index].AgentID == partner.ID {
			listed = &principals[index]
			break
		}
	}
	require.NotNil(t, listed)
	require.True(t, listed.SelfCommissionEnabled)
	require.Equal(t, service.AffiliateSelfCommissionRateBPS, listed.SelfCommissionRateBPS)
	require.Equal(t, int64(1), listed.SelfCommissionRevision)
	require.NotNil(t, listed.SelfCommissionEffectiveAt)
	require.Equal(t, "无上级合伙人本人返佣", listed.SelfCommissionReason)
	require.NotNil(t, listed.SelfCommissionUpdatedBy)
	require.Equal(t, admin.ID, *listed.SelfCommissionUpdatedBy)
	require.Equal(t, admin.Email, listed.SelfCommissionUpdatedByEmail)
	require.Equal(t, admin.Username, listed.SelfCommissionUpdatedByUsername)
	require.NotNil(t, listed.SelfCommissionUpdatedAt)
	require.False(t, listed.HasUpstream)
	require.True(t, listed.SelfCommissionEligible)
	require.Empty(t, listed.SelfCommissionBlockReason)

	// Disabling is always allowed, even after the partner is suspended.
	_, err = integrationDB.ExecContext(ctx, `
		UPDATE agent_principals
		SET status = 'suspended', updated_at = NOW()
		WHERE agent_id = $1
	`, partner.ID)
	require.NoError(t, err)
	disabled, err := policies.Update(
		ctx,
		partner.ID,
		false,
		1,
		"暂停期间关闭本人返佣",
		admin.ID,
	)
	require.NoError(t, err)
	require.False(t, disabled.Enabled)
	require.Equal(t, int64(2), disabled.Revision)
	require.Nil(t, disabled.EffectiveAt)
	require.False(t, disabled.Eligible)
	require.Equal(
		t,
		service.AffiliateSelfCommissionBlockAgentNotActive,
		disabled.BlockReasonCode,
	)

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_bindings (
			customer_user_id,
			inviter_user_id,
			binding_kind,
			customer_rebate_rate_snapshot_bps,
			agent_commission_rate_snapshot_bps
		)
		VALUES ($1, $2, 'ordinary', 0, 0)
	`, partner.ID, upstream.ID)
	require.NoError(t, err)

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE agent_principals
		SET status = 'active', updated_at = NOW()
		WHERE agent_id = $1
	`, partner.ID)
	require.NoError(t, err)
	_, err = policies.Update(
		ctx,
		partner.ID,
		true,
		2,
		"已有上级不得开启",
		admin.ID,
	)
	require.ErrorIs(t, err, service.ErrAffiliateSelfCommissionNotEligible)

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE affiliate_agent_self_commission_policies
		SET enabled = TRUE, effective_at = NOW()
		WHERE agent_id = $1
	`, partner.ID)
	requirePostgresConstraint(t, err, "affiliate_self_commission_no_upstream")

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE affiliate_agent_self_commission_policies
		SET rate_bps = 900
		WHERE agent_id = $1
	`, partner.ID)
	requirePostgresConstraint(t, err, "chk_affiliate_self_commission_rate")

	_, err = integrationDB.ExecContext(ctx, `
		UPDATE affiliate_agent_self_commission_events
		SET reason = '不得修改'
		WHERE agent_id = $1
	`, partner.ID)
	requirePostgresConstraint(t, err, "affiliate_self_commission_events_immutable")
	_, err = integrationDB.ExecContext(ctx, `
		DELETE FROM affiliate_agent_self_commission_events
		WHERE agent_id = $1
	`, partner.ID)
	requirePostgresConstraint(t, err, "affiliate_self_commission_events_immutable")
}

func TestAffiliateSelfCommissionPolicyRepository_AbsentPolicyIsDisabledRevisionZero(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	admin := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("self-commission-noop-admin-%d@example.com", time.Now().UnixNano()),
		Role:  service.RoleAdmin,
	})
	partner := createActiveAffiliatePaymentAgent(t, ctx, client, "self-commission-noop")

	policies := service.NewAffiliateSelfCommissionPolicyService(
		NewAffiliateSelfCommissionPolicyRepository(integrationDB),
	)
	disabled, err := policies.Update(
		ctx,
		partner.ID,
		false,
		0,
		"保持关闭",
		admin.ID,
	)
	require.NoError(t, err)
	require.False(t, disabled.Enabled)
	require.Zero(t, disabled.Revision)
	require.Equal(t, service.AffiliateSelfCommissionRateBPS, disabled.RateBPS)
	require.True(t, disabled.Eligible)

	var policyCount, eventCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM affiliate_agent_self_commission_policies WHERE agent_id = $1),
			(SELECT COUNT(*) FROM affiliate_agent_self_commission_events WHERE agent_id = $1)
	`, partner.ID).Scan(&policyCount, &eventCount))
	require.Zero(t, policyCount)
	require.Zero(t, eventCount)
}

func TestAffiliateSelfCommissionMigration_EnforcesAttributionShapes(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	consumer := createActiveAffiliatePaymentAgent(t, ctx, client, "self-shape-consumer")
	other := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("self-shape-other-%d@example.com", time.Now().UnixNano()),
	})
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())

	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, affiliate_policy, direct_partner_id,
			customer_rebate_rate_bps, partner_commission_rate_bps
		)
		VALUES (
			$1, 'paid_topup', $2,
			100000000, 100000000,
			TRUE, 'PARTNER_SELF_USAGE', $1,
			0, 1000
		)
	`, consumer.ID, "self-shape-valid-balance:"+suffix)
	require.NoError(t, err)

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, affiliate_policy, direct_partner_id,
			customer_rebate_rate_bps, partner_commission_rate_bps
		)
		VALUES (
			$1, 'paid_topup', $2,
			100000000, 100000000,
			TRUE, 'PARTNER_USAGE', $1,
			500, 500
		)
	`, consumer.ID, "self-shape-regular-self:"+suffix)
	requirePostgresConstraint(t, err, "chk_balance_lot_affiliate_shape")

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, affiliate_policy, direct_partner_id,
			customer_rebate_rate_bps, partner_commission_rate_bps
		)
		VALUES (
			$1, 'paid_topup', $2,
			100000000, 100000000,
			TRUE, 'PARTNER_SELF_USAGE', $3,
			0, 1000
		)
	`, consumer.ID, "self-shape-wrong-partner:"+suffix, other.ID)
	requirePostgresConstraint(t, err, "chk_balance_lot_affiliate_shape")

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO monthly_entitlement_cycles (
			user_id, source_type, source_key, product_code,
			sale_price_micros, credit_limit_micros,
			affiliate_eligible, affiliate_policy, direct_partner_id,
			customer_rebate_rate_bps, partner_commission_rate_bps,
			pricing_table_version, starts_at, ends_at
		)
		VALUES (
			$1, 'paid_redeem', $2, 'self-shape',
			100000000, 200000000,
			TRUE, 'PARTNER_SELF_USAGE', $1,
			500, 500,
			'test', NOW(), NOW() + INTERVAL '31 days'
		)
	`, consumer.ID, "self-shape-split-monthly:"+suffix)
	requirePostgresConstraint(t, err, "chk_monthly_entitlement_affiliate_shape")

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id, direct_agent_id, event_type, amount_micros,
			affiliate_policy,
			customer_rebate_rate_bps, partner_commission_rate_bps,
			source_type, source_id, event_key, occurred_at, metadata
		)
		VALUES (
			$1, $2, 'confirmed_consumption', 10000000,
			'PARTNER_SELF_USAGE',
			0, 1000,
			'integration', 1, $3, NOW(), '{}'::jsonb
		)
	`, consumer.ID, other.ID, "self-shape-wrong-event:"+suffix)
	requirePostgresConstraint(t, err, "chk_affiliate_performance_rates")

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id, direct_agent_id, event_type, amount_micros,
			affiliate_policy,
			customer_rebate_rate_bps, partner_commission_rate_bps,
			source_type, source_id, event_key, occurred_at, metadata
		)
		VALUES (
			$1, $1, 'confirmed_consumption', 10000000,
			'PARTNER_SELF_USAGE',
			0, 1000,
			'integration', 1, $2, NOW(), '{}'::jsonb
		)
	`, consumer.ID, "self-shape-valid-event:"+suffix)
	require.NoError(t, err)

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, consumer_user_id, entry_type,
			amount_micros, source_amount_micros,
			customer_rebate_rate_bps, agent_commission_rate_bps,
			posting_status, source_type, source_id,
			idempotency_key, metadata
		)
		VALUES (
			$1, $1, 'earned',
			5000000, 100000000,
			500, 500,
			'posted', 'integration', 1,
			$2, '{}'::jsonb
		)
	`, consumer.ID, "self-shape-split-cash:"+suffix)
	requirePostgresConstraint(t, err, "chk_agent_cash_self_commission_shape")

	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, consumer_user_id, entry_type,
			amount_micros, source_amount_micros,
			customer_rebate_rate_bps, agent_commission_rate_bps,
			posting_status, source_type, source_id,
			idempotency_key, metadata
		)
		VALUES (
			$1, $1, 'earned',
			10000000, 100000000,
			0, 1000,
			'posted', 'integration', 1,
			$2, jsonb_build_object('attribution_policy', 'PARTNER_SELF_USAGE')
		)
	`, consumer.ID, "self-shape-valid-cash:"+suffix)
	require.NoError(t, err)
}

func TestAffiliateSelfCommissionPolicyRepository_ConcurrentEnableAndBindingAreExclusive(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	admin := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("self-race-admin-%d@example.com", time.Now().UnixNano()),
		Role:  service.RoleAdmin,
	})
	policies := service.NewAffiliateSelfCommissionPolicyService(
		NewAffiliateSelfCommissionPolicyRepository(integrationDB),
	)

	for iteration := 0; iteration < 5; iteration++ {
		partner := createActiveAffiliatePaymentAgent(
			t,
			ctx,
			client,
			fmt.Sprintf("self-race-partner-%d", iteration),
		)
		upstream := mustCreateUser(t, client, &service.User{
			Email: fmt.Sprintf(
				"self-race-upstream-%d-%d@example.com",
				iteration,
				time.Now().UnixNano(),
			),
		})

		start := make(chan struct{})
		enableResult := make(chan error, 1)
		bindingResult := make(chan error, 1)
		go func() {
			<-start
			_, err := policies.Update(
				ctx,
				partner.ID,
				true,
				0,
				"并发开启互斥测试",
				admin.ID,
			)
			enableResult <- err
		}()
		go func() {
			<-start
			_, err := integrationDB.ExecContext(ctx, `
				INSERT INTO affiliate_bindings (
					customer_user_id,
					inviter_user_id,
					binding_kind,
					customer_rebate_rate_snapshot_bps,
					agent_commission_rate_snapshot_bps
				)
				VALUES ($1, $2, 'ordinary', 0, 0)
			`, partner.ID, upstream.ID)
			bindingResult <- err
		}()
		close(start)

		enableErr := <-enableResult
		bindingErr := <-bindingResult
		require.NotEqual(
			t,
			enableErr == nil,
			bindingErr == nil,
			"exactly one concurrent operation must succeed; enable=%v binding=%v",
			enableErr,
			bindingErr,
		)

		var enabled, bound bool
		require.NoError(t, integrationDB.QueryRowContext(ctx, `
			SELECT
				COALESCE((
					SELECT policy.enabled
					FROM affiliate_agent_self_commission_policies policy
					WHERE policy.agent_id = $1
				), FALSE),
				EXISTS (
					SELECT 1
					FROM affiliate_bindings binding
					WHERE binding.customer_user_id = $1
				)
		`, partner.ID).Scan(&enabled, &bound))
		require.NotEqual(t, enabled, bound)
	}
}

func requirePostgresConstraint(t *testing.T, err error, constraint string) {
	t.Helper()
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr), "expected pq error, got %T: %v", err, err)
	require.Equal(t, constraint, pqErr.Constraint)
}

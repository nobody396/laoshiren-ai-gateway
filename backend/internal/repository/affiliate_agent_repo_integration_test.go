//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestAffiliateAgentRepository_QualifiesAppliesReviewsAndPreservesUpstream(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateAgentRepository(integrationDB)
	agentService := service.NewAffiliateAgentService(repo)
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

	qualification, err := agentService.GetQualification(ctx, candidate.ID)
	require.NoError(t, err)
	require.True(t, qualification.DirectRouteQualified)
	require.False(t, qualification.SelfRouteQualified)
	require.True(t, qualification.Qualified)
	require.False(t, qualification.CanActivate)
	require.True(t, qualification.CanApply)
	require.Equal(t, "direct_team", qualification.QualificationRoute)
	require.Equal(t, int32(5), qualification.ValidDirectUserCount)
	require.Equal(t, int64(1_000_000_000), qualification.DirectTeamConsumptionMicros)

	application, err := agentService.Apply(ctx, candidate.ID, "申请成为合伙人")
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

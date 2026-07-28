//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateLinkRepository_DynamicRatesPreserveExactCustomerSnapshot(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewAffiliateLinkRepository(integrationDB)
	linkService := service.NewAffiliateLinkService(repo)
	agent := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-link-agent-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	customer := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-link-customer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_principals (
			agent_id, status, risk_status, qualified_at, activated_at
		)
		VALUES ($1, 'active', 'clear', NOW(), NOW())
	`, agent.ID)
	require.NoError(t, err)

	link, err := linkService.Create(ctx, agent.ID, "视频渠道", "video", 300)
	require.NoError(t, err)
	require.Equal(t, int32(300), link.CustomerRebateRateBPS)
	require.Equal(t, int32(700), link.AgentCommissionRateBPS)

	referral, err := repo.ResolveActiveLink(ctx, link.Code)
	require.NoError(t, err)
	require.NoError(t, repo.BindAgentReferral(ctx, customer.ID, *referral))

	link, err = linkService.UpdateRate(ctx, agent.ID, link.ID, 100)
	require.NoError(t, err)
	require.Equal(t, int32(100), link.CustomerRebateRateBPS)
	var customerRate, agentRate int32
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			customer_rebate_rate_snapshot_bps,
			agent_commission_rate_snapshot_bps
		FROM affiliate_bindings
		WHERE customer_user_id = $1
	`, customer.ID).Scan(&customerRate, &agentRate))
	require.Equal(t, int32(300), customerRate, "existing customer rebate cannot be changed")
	require.Equal(t, int32(700), agentRate)

	link, err = linkService.UpdateRate(ctx, agent.ID, link.ID, 500)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			customer_rebate_rate_snapshot_bps,
			agent_commission_rate_snapshot_bps
		FROM affiliate_bindings
		WHERE customer_user_id = $1
	`, customer.ID).Scan(&customerRate, &agentRate))
	require.Equal(t, int32(300), customerRate, "existing customer rebate cannot be changed")
	require.Equal(t, int32(700), agentRate)

	for i := 0; i < 4; i++ {
		_, err := linkService.Create(ctx, agent.ID, fmt.Sprintf("活动-%d", i), "campaign", int32(i)*100)
		require.NoError(t, err)
	}
	_, err = linkService.Create(ctx, agent.ID, "超限活动", "campaign", 0)
	require.True(t, errors.Is(err, service.ErrAffiliateLinkLimit), "sixth campaign link should be rejected: %v", err)
}

func TestAffiliateLinkRepository_ActiveAgentOrdinaryCodeUsesDefaultPoolLink(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	userRepo := NewUserRepository(client, integrationDB)
	commissionRepo := NewCommissionRepository(client, integrationDB)
	linkRepo := NewAffiliateLinkRepository(integrationDB)
	commissionService := service.NewCommissionService(userRepo, commissionRepo)
	commissionService.SetAffiliateLinkRepository(linkRepo)

	agent := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-default-agent-%d@example.com", time.Now().UnixNano()),
		Role:  service.RoleAgent,
	})
	customer := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("affiliate-default-customer-%d@example.com", time.Now().UnixNano()),
	})
	ordinaryCode := fmt.Sprintf("ORD%d", agent.ID)
	_, err := integrationDB.ExecContext(ctx, `
		UPDATE users
		SET invite_code = $1
		WHERE id = $2
	`, ordinaryCode, agent.ID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, `
		INSERT INTO agent_principals (
			agent_id, status, risk_status, qualified_at, activated_at
		)
		VALUES ($1, 'active', 'clear', NOW(), NOW())
	`, agent.ID)
	require.NoError(t, err)
	defaultLink, err := linkRepo.CreateLink(ctx, service.CreateAffiliateLinkInput{
		AgentID:               agent.ID,
		Code:                  fmt.Sprintf("ADEFAULT%d", agent.ID),
		Name:                  "默认推广链接",
		Channel:               "default",
		CustomerRebateRateBPS: 400,
		CreatedBy:             agent.ID,
		IsDefault:             true,
	})
	require.NoError(t, err)

	require.NoError(t, commissionService.BindReferralCode(ctx, customer.ID, ordinaryCode))

	var (
		bindingKind string
		linkID      int64
		customerBPS int32
		agentBPS    int32
	)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT
			binding_kind,
			affiliate_link_id,
			customer_rebate_rate_snapshot_bps,
			agent_commission_rate_snapshot_bps
		FROM affiliate_bindings
		WHERE customer_user_id = $1
	`, customer.ID).Scan(&bindingKind, &linkID, &customerBPS, &agentBPS))
	require.Equal(t, service.AffiliateBindingAgent, bindingKind)
	require.Equal(t, defaultLink.ID, linkID)
	require.Equal(t, int32(400), customerBPS)
	require.Equal(t, int32(600), agentBPS)

	_, err = linkRepo.SetLinkStatus(ctx, agent.ID, defaultLink.ID, "disabled")
	require.ErrorIs(t, err, service.ErrAffiliateDefaultLinkRequired)
}

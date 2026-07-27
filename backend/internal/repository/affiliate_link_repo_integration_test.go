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

func TestAffiliateLinkRepository_DynamicRatesPreserveCustomerFloor(t *testing.T) {
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
	require.Equal(t, int32(300), customerRate, "existing customer rebate cannot decrease")
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
	require.Equal(t, int32(500), customerRate, "existing customer rebate may increase")
	require.Equal(t, int32(500), agentRate)

	for i := 0; i < 4; i++ {
		_, err := linkService.Create(ctx, agent.ID, fmt.Sprintf("活动-%d", i), "campaign", int32(i)*100)
		require.NoError(t, err)
	}
	_, err = linkService.Create(ctx, agent.ID, "超限活动", "campaign", 0)
	require.True(t, errors.Is(err, service.ErrAffiliateLinkLimit), "sixth campaign link should be rejected: %v", err)
}

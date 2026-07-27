//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAgentPaymentProfileV2_VerificationAndPrincipalIdentityGuard(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	userRepo := NewUserRepository(client, integrationDB)
	commissionRepo := NewCommissionRepository(client, integrationDB)
	commissionService := service.NewCommissionService(userRepo, commissionRepo)
	paymentRepo := commissionRepo.(service.AgentPaymentRepository)

	admin := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("payment-review-admin-%d@example.com", time.Now().UnixNano()),
		Role:  service.RoleAdmin,
	})
	first := createActiveAffiliatePaymentAgent(t, ctx, client, "payment-agent-first")
	second := createActiveAffiliatePaymentAgent(t, ctx, client, "payment-agent-second")

	firstProfile, err := commissionService.UpdateAgentPaymentProfile(ctx, &service.AgentPaymentProfile{
		AgentID:        first.ID,
		AlipayRealName: "测试姓名",
		AlipayAccount:  "same-account@example.com",
		ContactPhone:   "13800000000",
	})
	require.NoError(t, err)
	require.Equal(t, "incomplete", firstProfile.VerificationStatus)
	require.False(t, firstProfile.Verified)

	_, err = paymentRepo.UpdateAgentPaymentQRCode(
		ctx,
		first.ID,
		"agent-payment-qrcodes/test/first.png",
		"image/png",
		"first.png",
		128,
	)
	require.NoError(t, err)
	firstProfile, err = commissionService.GetAgentPaymentProfile(ctx, first.ID)
	require.NoError(t, err)
	require.True(t, firstProfile.Complete)
	require.Equal(t, "pending_review", firstProfile.VerificationStatus)
	pending, err := commissionService.ListPendingAgentPaymentProfiles(ctx, 100)
	require.NoError(t, err)
	require.Contains(t, pendingAgentPaymentProfileIDs(pending), first.ID)

	firstProfile, err = commissionService.ReviewAgentPaymentProfile(
		ctx,
		first.ID,
		admin.ID,
		"verified",
		"资料一致",
	)
	require.NoError(t, err)
	require.True(t, firstProfile.Verified)
	require.NotNil(t, firstProfile.VerifiedAt)
	require.NotNil(t, firstProfile.VerifiedBy)
	require.Equal(t, admin.ID, *firstProfile.VerifiedBy)
	pending, err = commissionService.ListPendingAgentPaymentProfiles(ctx, 100)
	require.NoError(t, err)
	require.NotContains(t, pendingAgentPaymentProfileIDs(pending), first.ID)

	var firstPrincipalHash string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT principal_key_hash
		FROM agent_principals
		WHERE agent_id = $1
	`, first.ID).Scan(&firstPrincipalHash))
	require.NotEmpty(t, firstPrincipalHash)

	_, err = commissionService.UpdateAgentPaymentProfile(ctx, &service.AgentPaymentProfile{
		AgentID:        second.ID,
		AlipayRealName: "测试姓名",
		AlipayAccount:  "SAME-ACCOUNT@example.com",
	})
	require.NoError(t, err)
	_, err = paymentRepo.UpdateAgentPaymentQRCode(
		ctx,
		second.ID,
		"agent-payment-qrcodes/test/second.png",
		"image/png",
		"second.png",
		128,
	)
	require.NoError(t, err)
	_, err = commissionService.ReviewAgentPaymentProfile(
		ctx,
		second.ID,
		admin.ID,
		"verified",
		"",
	)
	require.True(t, errors.Is(err, service.ErrAgentPaymentIdentityConflict), "unexpected error: %v", err)

	firstProfile, err = commissionService.UpdateAgentPaymentProfile(ctx, &service.AgentPaymentProfile{
		AgentID:        first.ID,
		AlipayRealName: "测试姓名",
		AlipayAccount:  "new-account@example.com",
	})
	require.NoError(t, err)
	require.Equal(t, "pending_review", firstProfile.VerificationStatus)
	require.False(t, firstProfile.Verified)
	var clearedHash *string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT principal_key_hash
		FROM agent_principals
		WHERE agent_id = $1
	`, first.ID).Scan(&clearedHash))
	require.Nil(t, clearedHash, "editing a verified profile must revoke its principal fingerprint")
}

func pendingAgentPaymentProfileIDs(items []service.AgentPaymentProfile) []int64 {
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.AgentID)
	}
	return ids
}

func createActiveAffiliatePaymentAgent(
	t *testing.T,
	ctx context.Context,
	client *dbent.Client,
	prefix string,
) *service.User {
	t.Helper()
	agent := mustCreateUser(t, client, &service.User{
		Email: fmt.Sprintf("%s-%d@example.com", prefix, time.Now().UnixNano()),
		Role:  service.RoleAgent,
	})
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO agent_principals (
			agent_id, status, risk_status,
			qualified_at, activated_at
		)
		VALUES ($1, 'active', 'clear', NOW(), NOW())
	`, agent.ID)
	require.NoError(t, err)
	return agent
}

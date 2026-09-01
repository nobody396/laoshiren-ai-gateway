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

func TestRedeemCurrentMonthlyPairAutoCompletesAndAssignsThreeHosts(t *testing.T) {
	ctx := context.Background()
	var occupiedIDs int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT count(*) FROM groups WHERE id=ANY(ARRAY[40,41,48]::bigint[])`,
	).Scan(&occupiedIDs))
	require.Zero(t, occupiedIDs, "isolated integration catalog must leave production IDs available")
	_, err := integrationDB.ExecContext(ctx, `
		INSERT INTO groups (
			id,name,description,rate_multiplier,is_exclusive,status,platform,
			subscription_type,monthly_limit_usd,default_validity_days,created_at,updated_at
		) VALUES
			(40,'integration-plus-gpt','','0.5',TRUE,'active','openai','credit',380,31,NOW(),NOW()),
			(41,'integration-plus-claude','','2.4',TRUE,'active','anthropic','credit',380,31,NOW(),NOW()),
			(48,'integration-plus-grok','','0.4',TRUE,'active','grok','credit',380,31,NOW(),NOW());
		SELECT setval(pg_get_serial_sequence('groups','id'),(SELECT max(id) FROM groups),TRUE);
	`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(),
			`DELETE FROM groups WHERE id=ANY(ARRAY[40,41,48]::bigint[])`)
	})

	client := testEntClient(t)
	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	groupRepo := NewGroupRepository(client, integrationDB)
	subscriptionRepo := NewUserSubscriptionRepository(client)
	subscriptionService := service.NewSubscriptionService(groupRepo, subscriptionRepo, nil, client, nil)
	redeemRepo := NewRedeemCodeRepository(client)
	redeemService := service.NewRedeemService(
		redeemRepo, nil, userRepo, subscriptionService, nil, nil, client,
		nil, nil, nil, nil, nil,
	)

	customer := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("monthly-autocomplete-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	primaryGroupID := int64(40)
	code := &service.RedeemCode{
		Code:         fmt.Sprintf("%032x", time.Now().UnixNano()),
		Type:         service.RedeemTypeSubscription,
		Value:        299,
		Status:       service.StatusUnused,
		GroupID:      &primaryGroupID,
		GroupIDs:     []int64{40, 41},
		ValidityDays: 31,
		Purpose:      service.RedeemCodePurposeInternalTest,
		SalesStatus:  service.RedeemCodeSalesStatusGifted,
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = integrationDB.ExecContext(cleanupCtx, `DELETE FROM user_subscriptions WHERE user_id=$1`, customer.ID)
		_, _ = integrationDB.ExecContext(cleanupCtx, `DELETE FROM redeem_codes WHERE code=$1`, code.Code)
		_, _ = integrationDB.ExecContext(cleanupCtx, `DELETE FROM users WHERE id=$1`, customer.ID)
	})
	// Direct repository insert simulates inventory created before the current
	// three-host contract and proves the redemption fallback, not just creation.
	require.NoError(t, redeemRepo.Create(ctx, code))

	redeemed, err := redeemService.Redeem(ctx, customer.ID, code.Code)
	require.NoError(t, err)
	require.Equal(t, []int64{40, 41, 48}, redeemed.GroupIDs)
	require.NotNil(t, redeemed.GroupID)
	require.Equal(t, int64(40), *redeemed.GroupID)

	var persistedGroupIDs string
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		`SELECT group_ids::text FROM redeem_codes WHERE id=$1`, code.ID,
	).Scan(&persistedGroupIDs))
	require.JSONEq(t, `[40,41,48]`, persistedGroupIDs)

	var subscriptionCount, markerCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT count(*),count(DISTINCT substring(notes FROM 'shared_quota=([^;,[:space:]]+)'))
		FROM user_subscriptions
		WHERE user_id=$1 AND group_id=ANY(ARRAY[40,41,48]::bigint[])
		  AND status='active' AND expires_at>NOW()
	`, customer.ID).Scan(&subscriptionCount, &markerCount))
	require.Equal(t, 3, subscriptionCount)
	require.Equal(t, 1, markerCount)
}

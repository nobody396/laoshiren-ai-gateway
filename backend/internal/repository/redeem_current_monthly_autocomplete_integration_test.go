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
		Code:         fmt.Sprintf("MONTHLY-AUTOCOMPLETE-%d", time.Now().UnixNano()),
		Type:         service.RedeemTypeSubscription,
		Value:        299,
		Status:       service.StatusUnused,
		GroupID:      &primaryGroupID,
		GroupIDs:     []int64{40, 41},
		ValidityDays: 31,
		Purpose:      service.RedeemCodePurposeInternalTest,
		SalesStatus:  service.RedeemCodeSalesStatusGifted,
	}
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

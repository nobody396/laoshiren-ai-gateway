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

func TestNativeCheckoutRepositoryEnforcesOnceAndClaimsRestrictedInventory(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("native-checkout-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	redeemRepo := NewRedeemCodeRepository(client)
	code := &service.RedeemCode{
		Code:         fmt.Sprintf("%032x", time.Now().UnixNano()),
		Type:         service.RedeemTypeBalance,
		Value:        10,
		PaidValue:    0,
		Status:       service.StatusUnused,
		Purpose:      service.RedeemCodePurposeGift,
		SalesStatus:  service.RedeemCodeSalesStatusGifted,
		ValidityDays: 0,
	}
	require.NoError(t, redeemRepo.Create(ctx, code))
	mismatchedCode := &service.RedeemCode{
		Code:         fmt.Sprintf("%032x", time.Now().UnixNano()+1),
		Type:         service.RedeemTypeBalance,
		Value:        11,
		PaidValue:    0,
		Status:       service.StatusUnused,
		Purpose:      service.RedeemCodePurposeGift,
		SalesStatus:  service.RedeemCodeSalesStatusGifted,
		ValidityDays: 0,
	}
	require.NoError(t, redeemRepo.Create(ctx, mismatchedCode))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_redeem_inventory WHERE redeem_code_id = $1`, code.ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_orders WHERE user_id = $1`, user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_offer_testers WHERE user_id = $1`, user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE id = $1`, code.ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE id = $1`, mismatchedCode.ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})
	_, err := integrationDB.ExecContext(ctx, `
INSERT INTO native_checkout_redeem_inventory (redeem_code_id, offer_code)
VALUES ($1, 'newcomer-balance-5-to-10')
`, mismatchedCode.ID)
	require.Error(t, err, "inventory whose entitlement differs from the canonical offer must be rejected")
	require.NoError(t, insertNativeCheckoutInventory(ctx, code.ID))
	_, err = integrationDB.ExecContext(ctx, `
UPDATE native_checkout_offers
SET redeem_paid_value = 1
WHERE code = 'newcomer-balance-5-to-10'
`)
	require.Error(t, err, "stocked offer semantics must be immutable")

	repo := NewNativeCheckoutRepository(integrationDB)
	hidden, err := repo.ListVisibleOffers(ctx, user.ID)
	require.NoError(t, err)
	require.Empty(t, hidden, "a disabled offer must stay invisible before the owned tester is allowlisted")
	_, err = repo.GetVisibleOffer(ctx, user.ID, "newcomer-balance-5-to-10")
	require.ErrorIs(t, err, service.ErrNativeCheckoutOfferNotFound)
	require.NoError(t, allowNativeCheckoutTester(ctx, user.ID))
	visible, err := repo.ListVisibleOffers(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, visible, 1)
	require.Equal(t, "newcomer-balance-5-to-10", visible[0].Code)
	require.False(t, visible[0].Enabled, "tester visibility must not globally enable the offer")
	visibleOffer, err := repo.GetVisibleOffer(ctx, user.ID, "newcomer-balance-5-to-10")
	require.NoError(t, err)
	require.False(t, visibleOffer.Enabled)

	first := integrationNativeCheckoutOrder(user.ID, "NC-"+fmt.Sprint(time.Now().UnixNano()))
	reserved, created, err := repo.ReserveOrder(ctx, first)
	require.NoError(t, err)
	require.True(t, created)

	second := integrationNativeCheckoutOrder(user.ID, "NC-"+fmt.Sprint(time.Now().UnixNano()+1))
	reused, created, err := repo.ReserveOrder(ctx, second)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, reserved.ID, reused.ID, "the once-only index must own one durable row per account")

	providerOrder, err := repo.SetProviderOrder(
		ctx,
		reserved.ID,
		"LD-"+fmt.Sprint(time.Now().UnixNano()),
		"https://pay.ldxp.cn/pay/test",
		service.NativeCheckoutPaymentMethodWeChat,
	)
	require.NoError(t, err)
	require.Equal(t, service.NativeCheckoutPaymentMethodWeChat, providerOrder.PaymentMethod)

	leaseUntil := time.Now().Add(time.Minute)
	leased, err := repo.ClaimReconcileOrders(ctx, 4, time.Now().Add(-time.Minute), leaseUntil)
	require.NoError(t, err)
	require.Len(t, leased, 1)
	require.Equal(t, providerOrder.ID, leased[0].ID)
	leasedAgain, err := repo.ClaimReconcileOrders(ctx, 4, time.Now().Add(-time.Minute), leaseUntil)
	require.NoError(t, err)
	require.Empty(t, leasedAgain, "a leased order must not be selected by another server worker")

	providerOrder, err = repo.SetOrderState(
		ctx,
		providerOrder.ID,
		service.NativeCheckoutStatusChecking,
		"",
		time.Now(),
	)
	require.NoError(t, err)
	claimed, didClaim, err := repo.ClaimFulfillment(ctx, providerOrder.ID, code.ID, time.Now().Add(-time.Minute))
	require.NoError(t, err)
	require.True(t, didClaim)
	require.Equal(t, code.ID, *claimed.RedeemCodeID)

	var assignedOrderID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT assigned_order_id FROM native_checkout_redeem_inventory WHERE redeem_code_id = $1
`, code.ID).Scan(&assignedOrderID))
	require.Equal(t, claimed.ID, assignedOrderID)
	restricted, err := repo.IsNativeCheckoutRestricted(ctx, code.ID)
	require.NoError(t, err)
	require.True(t, restricted)

	_, err = integrationDB.ExecContext(ctx, `
UPDATE redeem_codes
SET status = 'used', used_by = $2, used_at = NOW()
WHERE id = $1
`, code.ID, user.ID)
	require.NoError(t, err)
	claimedEntitlement, err := repo.HasRedeemedOffer(ctx, user.ID, first.OfferCode)
	require.NoError(t, err)
	require.True(t, claimedEntitlement, "a recovered native inventory card must still consume the once-only offer")

	// A retry sees the current order and does not claim or redeem twice. Keep a
	// one-connection pool here to guard against querying before tx rollback.
	oldMaxOpen := integrationDB.Stats().MaxOpenConnections
	integrationDB.SetMaxOpenConns(1)
	t.Cleanup(func() { integrationDB.SetMaxOpenConns(oldMaxOpen) })
	retryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	retried, didClaim, err := repo.ClaimFulfillment(retryCtx, claimed.ID, code.ID, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.False(t, didClaim)
	require.Equal(t, claimed.ID, retried.ID)
}

func allowNativeCheckoutTester(ctx context.Context, userID int64) error {
	_, err := integrationDB.ExecContext(ctx, `
INSERT INTO native_checkout_offer_testers (offer_code, user_id)
VALUES ('newcomer-balance-5-to-10', $1)
ON CONFLICT DO NOTHING
`, userID)
	return err
}

func insertNativeCheckoutInventory(ctx context.Context, redeemCodeID int64) error {
	_, err := integrationDB.ExecContext(ctx, `
INSERT INTO native_checkout_redeem_inventory (redeem_code_id, offer_code)
VALUES ($1, 'newcomer-balance-5-to-10')
`, redeemCodeID)
	return err
}

func integrationNativeCheckoutOrder(userID int64, orderNo string) *service.NativeCheckoutOrder {
	return &service.NativeCheckoutOrder{
		OrderNo:             orderNo,
		UserID:              userID,
		OfferCode:           "newcomer-balance-5-to-10",
		Provider:            "ldxp",
		ProviderGoodsKey:    "oc3w4r",
		ContactHash:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ProductKind:         "balance",
		PayAmountCNYFen:     500,
		BenefitAmountCNYFen: 1000,
		RedeemType:          service.RedeemTypeBalance,
		RedeemValue:         10,
		RedeemPaidValue:     0,
		RedeemPurpose:       service.RedeemCodePurposeGift,
		RedeemSalesStatus:   service.RedeemCodeSalesStatusGifted,
		RedeemValidityDays:  0,
		EnforceOnce:         true,
		Status:              service.NativeCheckoutStatusCreating,
		NextCheckAt:         time.Now(),
	}
}

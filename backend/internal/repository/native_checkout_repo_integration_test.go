//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestManualNewcomerRedeemCreditsBalanceAndRejectsSecondCode(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("manual-newcomer-service-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	redeemRepo := NewRedeemCodeRepository(client)
	codes := make([]*service.RedeemCode, 2)
	for i := range codes {
		codes[i] = &service.RedeemCode{
			Code:         fmt.Sprintf("%032x", time.Now().UnixNano()+int64(i)),
			Type:         service.RedeemTypeBalance,
			Value:        10,
			PaidValue:    0,
			Status:       service.StatusUnused,
			Purpose:      service.RedeemCodePurposeGift,
			SalesStatus:  service.RedeemCodeSalesStatusGifted,
			ValidityDays: 0,
		}
		require.NoError(t, redeemRepo.Create(ctx, codes[i]))
		require.NoError(t, insertNativeCheckoutInventory(ctx, codes[i].ID))
	}
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_manual_claims WHERE user_id = $1`, user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_redeem_inventory WHERE redeem_code_id IN ($1, $2)`, codes[0].ID, codes[1].ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE id IN ($1, $2)`, codes[0].ID, codes[1].ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})

	guard := NewNativeCheckoutRepository(integrationDB)
	userRepo := newUserRepositoryWithSQL(client, integrationDB)
	redeemService := service.NewRedeemService(
		redeemRepo, nil, userRepo, nil, nil, nil, client, nil, nil, nil, nil, nil,
	)
	redeemService.SetNativeCheckoutRedeemGuard(guard)

	result, err := redeemService.Redeem(ctx, user.ID, codes[0].Code)
	require.NoError(t, err)
	require.Equal(t, float64(10), result.Value)
	refreshed, err := userRepo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, float64(10), refreshed.Balance)

	_, err = redeemService.Redeem(ctx, user.ID, codes[1].Code)
	require.ErrorIs(t, err, service.ErrRedeemOfferClaimed)
	require.Equal(t, "REDEEM_OFFER_ALREADY_CLAIMED", infraerrors.Reason(err))
	second, err := redeemRepo.GetByID(ctx, codes[1].ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, second.Status)
	refreshed, err = userRepo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, float64(10), refreshed.Balance)
}

func TestManualNewcomerRedemptionAllowsOneConcurrentClaimPerAccount(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("manual-newcomer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	otherUser := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("manual-newcomer-other-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	redeemRepo := NewRedeemCodeRepository(client)
	codes := make([]*service.RedeemCode, 3)
	for i := range codes {
		codes[i] = &service.RedeemCode{
			Code:         fmt.Sprintf("%032x", time.Now().UnixNano()+int64(i)),
			Type:         service.RedeemTypeBalance,
			Value:        10,
			PaidValue:    0,
			Status:       service.StatusUnused,
			Purpose:      service.RedeemCodePurposeGift,
			SalesStatus:  service.RedeemCodeSalesStatusGifted,
			ValidityDays: 0,
		}
		require.NoError(t, redeemRepo.Create(ctx, codes[i]))
		require.NoError(t, insertNativeCheckoutInventory(ctx, codes[i].ID))
	}
	t.Cleanup(func() {
		ids := []any{codes[0].ID, codes[1].ID, codes[2].ID}
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_manual_claims WHERE redeem_code_id IN ($1, $2, $3) OR user_id IN ($4, $5)`, append(ids, user.ID, otherUser.ID)...)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_redeem_inventory WHERE redeem_code_id IN ($1, $2, $3)`, ids...)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE id IN ($1, $2, $3)`, ids...)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id IN ($1, $2)`, user.ID, otherUser.ID)
	})

	repo := NewNativeCheckoutRepository(integrationDB)
	policy, err := repo.GetNativeCheckoutRedeemPolicy(ctx, codes[0].ID, user.ID)
	require.NoError(t, err)
	require.Equal(t, service.NativeCheckoutRedeemPolicy{
		Restricted: true, ManualRedeemEnabled: true, AlreadyClaimed: false,
	}, policy)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, code := range codes[:2] {
		wg.Add(1)
		go func(codeID int64) {
			defer wg.Done()
			<-start
			errs <- redeemRepo.Use(ctx, codeID, user.ID)
		}(code.ID)
	}
	close(start)
	wg.Wait()
	close(errs)

	var succeeded, rejected int
	for useErr := range errs {
		switch {
		case useErr == nil:
			succeeded++
		case errors.Is(useErr, service.ErrRedeemOfferClaimed):
			rejected++
		default:
			require.NoError(t, useErr)
		}
	}
	require.Equal(t, 1, succeeded)
	require.Equal(t, 1, rejected)

	var claimCount, usedCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM native_checkout_manual_claims WHERE offer_code = 'newcomer-balance-5-to-10' AND user_id = $1`, user.ID).Scan(&claimCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM redeem_codes WHERE id IN ($1, $2) AND status = 'used'`, codes[0].ID, codes[1].ID).Scan(&usedCount))
	require.Equal(t, 1, claimCount)
	require.Equal(t, 1, usedCount)

	require.NoError(t, redeemRepo.Use(ctx, codes[2].ID, otherUser.ID), "another account must retain its own one-time claim")
}

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
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_manual_claims WHERE redeem_code_id IN ($1, $2) OR user_id = $3`, code.ID, mismatchedCode.ID, user.ID)
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
	manualOffer, err := repo.GetManualRedeemOffer(ctx, "newcomer-balance-5-to-10")
	require.NoError(t, err)
	require.Equal(t, "newcomer-balance-5-to-10", manualOffer.Code)
	require.False(t, manualOffer.Enabled, "manual status reads must not enable the retired native checkout path")
	_, err = repo.GetManualRedeemOffer(ctx, "missing-offer")
	require.ErrorIs(t, err, service.ErrNativeCheckoutOfferNotFound)

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
	policy, err := repo.GetNativeCheckoutRedeemPolicy(ctx, code.ID, user.ID)
	require.NoError(t, err)
	require.True(t, policy.Restricted)
	require.True(t, policy.ManualRedeemEnabled)
	require.False(t, policy.AlreadyClaimed)

	_, err = integrationDB.ExecContext(ctx, `
UPDATE redeem_codes
SET status = 'used', used_by = $2, used_at = NOW()
WHERE id = $1
`, code.ID, user.ID)
	require.NoError(t, err)
	policy, err = repo.GetNativeCheckoutRedeemPolicy(ctx, code.ID, user.ID)
	require.NoError(t, err)
	require.True(t, policy.AlreadyClaimed)
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

func TestNativeCheckoutRepositoryMintRedeemCodeIsAtomicAndIdempotent(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("native-checkout-mint-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})
	repo := NewNativeCheckoutRepository(integrationDB)

	order := integrationNativeCheckoutOrder(user.ID, "NC-"+fmt.Sprint(time.Now().UnixNano()))
	order.Provider = "easypay"
	order.ProviderGoodsKey = "newcomer-balance-5-to-10"
	reserved, created, err := repo.ReserveOrder(ctx, order)
	require.NoError(t, err)
	require.True(t, created)
	pending, err := repo.SetProviderOrder(ctx, reserved.ID, reserved.OrderNo, "https://pay.example.com/cashier/"+reserved.OrderNo, service.NativeCheckoutPaymentMethodAlipay)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_manual_claims WHERE user_id = $1`, user.ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_redeem_inventory WHERE redeem_code_id IN (SELECT id FROM redeem_codes WHERE external_order_no = $1)`, pending.ProviderTradeNo)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM native_checkout_orders WHERE id = $1`, pending.ID)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE external_order_no = $1`, pending.ProviderTradeNo)
		_, _ = integrationDB.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})

	mintedCode := fmt.Sprintf("%032x", time.Now().UnixNano())
	minted, err := repo.MintRedeemCode(ctx, pending, mintedCode)
	require.NoError(t, err)
	require.Equal(t, mintedCode, minted)

	// The code row carries the order snapshot semantics and the trade link, and
	// the inventory trigger accepted it (semantics match the canonical offer).
	var codeType, purpose, salesStatus, externalOrderNo, internalNotes string
	var value, paidValue float64
	var validityDays int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT type, value::double precision, paid_value::double precision, purpose, sales_status,
       validity_days, external_order_no, COALESCE(internal_notes, '')
FROM redeem_codes WHERE code = $1
`, mintedCode).Scan(&codeType, &value, &paidValue, &purpose, &salesStatus, &validityDays, &externalOrderNo, &internalNotes))
	require.Equal(t, service.RedeemTypeBalance, codeType)
	require.Equal(t, float64(10), value)
	require.Zero(t, paidValue)
	require.Equal(t, service.RedeemCodePurposeGift, purpose)
	require.Equal(t, service.RedeemCodeSalesStatusGifted, salesStatus)
	require.Zero(t, validityDays)
	require.Equal(t, pending.ProviderTradeNo, externalOrderNo)
	require.Contains(t, internalNotes, "native-checkout-easypay")

	var inventoryCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
SELECT COUNT(*) FROM native_checkout_redeem_inventory WHERE offer_code = $1 AND redeem_code_id = (SELECT id FROM redeem_codes WHERE code = $2)
`, pending.OfferCode, mintedCode).Scan(&inventoryCount))
	require.Equal(t, 1, inventoryCount)

	found, err := repo.FindMintedRedeemCode(ctx, pending.ProviderTradeNo)
	require.NoError(t, err)
	require.Equal(t, mintedCode, found)

	// A concurrent mint with a different code value must lose the unique race
	// and return the already-committed code.
	duplicate, err := repo.MintRedeemCode(ctx, pending, fmt.Sprintf("%032x", time.Now().UnixNano()+1))
	require.NoError(t, err)
	require.Equal(t, mintedCode, duplicate, "the unique index on external_order_no resolves the mint race to the first code")

	// Minting against the offer with wrong snapshot semantics must fail the
	// inventory trigger and roll the code row back atomically.
	badOrder := *pending
	badOrder.ProviderTradeNo = "NC-bad-" + fmt.Sprint(time.Now().UnixNano())
	badOrder.RedeemValue = 11
	badCode := fmt.Sprintf("%032x", time.Now().UnixNano()+2)
	_, err = repo.MintRedeemCode(ctx, &badOrder, badCode)
	require.Error(t, err, "inventory trigger must reject semantics that diverge from the canonical offer")
	_, err = repo.FindMintedRedeemCode(ctx, badOrder.ProviderTradeNo)
	require.ErrorIs(t, err, service.ErrRedeemCodeNotFound, "a rejected mint must roll back the redeem code row too")
	var orphanCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM redeem_codes WHERE code = $1`, badCode).Scan(&orphanCount))
	require.Zero(t, orphanCount)

	// Nudge moves a pending order's reconciliation to now.
	future, err := repo.SetOrderState(ctx, pending.ID, service.NativeCheckoutStatusPending, "", time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.True(t, future.NextCheckAt.After(time.Now().Add(30*time.Minute)))
	require.NoError(t, repo.NudgeReconcileNow(ctx, pending.ID))
	nudged, err := repo.GetOrder(ctx, pending.OrderNo)
	require.NoError(t, err)
	require.False(t, nudged.NextCheckAt.After(time.Now()), "a paid notify must make the order immediately reconcileable")
}

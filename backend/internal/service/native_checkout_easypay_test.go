//go:build unit

package service

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/payment"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func testNativeCheckoutEasyPayOffer() NativeCheckoutOffer {
	offer := testNativeCheckoutOffer()
	offer.Provider = NativeCheckoutProviderEasyPay
	offer.ProviderGoodsKey = "newcomer-balance-5-to-10"
	return offer
}

// testNativeCheckoutEasyPayOrder mirrors what production holds after create:
// provider_trade_no is our own NC- order number (the EasyPay out_trade_no).
func testNativeCheckoutEasyPayOrder(offer NativeCheckoutOffer) *NativeCheckoutOrder {
	order := testNativeCheckoutOrder(offer)
	order.Provider = NativeCheckoutProviderEasyPay
	order.ProviderTradeNo = order.OrderNo
	order.PaymentURL = "https://pay.example.com/cashier/" + order.OrderNo
	order.PaymentMethod = NativeCheckoutPaymentMethodAlipay
	return order
}

func TestNativeCheckoutProviderResolverRejectsUnknownProvider(t *testing.T) {
	resolver := NewNativeCheckoutProviderResolver(map[string]NativeCheckoutProvider{
		NativeCheckoutProviderLDXP:    &nativeCheckoutProviderFake{},
		NativeCheckoutProviderEasyPay: nil, // nil entries are dropped
	})
	provider, err := resolver.ProviderFor(NativeCheckoutProviderLDXP)
	require.NoError(t, err)
	require.NotNil(t, provider)

	_, err = resolver.ProviderFor("bogus")
	require.ErrorIs(t, err, ErrNativeCheckoutUnavailable)
	_, err = resolver.ProviderFor(NativeCheckoutProviderEasyPay)
	require.ErrorIs(t, err, ErrNativeCheckoutUnavailable, "a nil provider entry must resolve to a coded error, not a panic")
}

func TestNativeCheckoutEasyPayCreatePassesOrderNoAndDefaultsAlipay(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutEasyPayOffer())
	provider := &nativeCheckoutProviderFake{
		created: &NativeCheckoutProviderOrder{
			TradeNo: "NC-echo", PaymentURL: "https://pay.example.com/cashier/NC-echo", PaymentMethod: NativeCheckoutPaymentMethodAlipay,
		},
	}
	svc := NewNativeCheckoutService(
		repo,
		nativeCheckoutTestResolver(provider),
		&nativeCheckoutUserRepoFake{user: &User{ID: 42, Email: "buyer@example.com"}},
		&nativeCheckoutRedeemerFake{},
		nativeCheckoutTestContactKey,
	)

	order, err := svc.CreateOrder(context.Background(), 42, repo.offer.Code, "")
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusPending, order.Status)
	require.Equal(t, NativeCheckoutProviderEasyPay, order.Provider)
	require.Equal(t, 1, provider.createCalls)
	require.Equal(t, NativeCheckoutPaymentMethodAlipay, provider.payType, "easypay defaults to alipay when pay_type is empty")
	require.Equal(t, order.OrderNo, provider.orderNo, "easypay must receive our NC- order number as out_trade_no")
	require.Equal(t, int64(500), provider.amountFen)
}

func TestNativeCheckoutEasyPayCreateHonorsWechatSelection(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutEasyPayOffer())
	provider := &nativeCheckoutProviderFake{
		created: &NativeCheckoutProviderOrder{
			TradeNo: "NC-echo", PaymentURL: "https://pay.example.com/cashier/NC-echo", PaymentMethod: NativeCheckoutPaymentMethodWeChat,
		},
	}
	svc := NewNativeCheckoutService(
		repo,
		nativeCheckoutTestResolver(provider),
		&nativeCheckoutUserRepoFake{user: &User{ID: 42, Email: "buyer@example.com"}},
		&nativeCheckoutRedeemerFake{},
		nativeCheckoutTestContactKey,
	)

	_, err := svc.CreateOrder(context.Background(), 42, repo.offer.Code, NativeCheckoutPaymentMethodWeChat)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutPaymentMethodWeChat, provider.payType)
}

func TestNativeCheckoutEasyPayCreateRejectsInvalidPayType(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutEasyPayOffer())
	provider := &nativeCheckoutProviderFake{}
	svc := NewNativeCheckoutService(
		repo,
		nativeCheckoutTestResolver(provider),
		&nativeCheckoutUserRepoFake{user: &User{ID: 42, Email: "buyer@example.com"}},
		&nativeCheckoutRedeemerFake{},
		nativeCheckoutTestContactKey,
	)

	_, err := svc.CreateOrder(context.Background(), 42, repo.offer.Code, "unionpay")
	require.Error(t, err)
	require.Equal(t, "NATIVE_CHECKOUT_PAY_TYPE_INVALID", infraerrors.Reason(err))
	require.Zero(t, provider.createCalls)
	require.Nil(t, repo.order, "an invalid pay type must fail before any durable reservation")
}

func TestNativeCheckoutLDXPCreateIgnoresPayType(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutOffer())
	provider := &nativeCheckoutProviderFake{
		created: &NativeCheckoutProviderOrder{
			TradeNo: "LD-1", PaymentURL: "https://pay.ldxp.cn/pay/LD-1", PaymentMethod: NativeCheckoutPaymentMethodWeChat,
		},
	}
	svc := NewNativeCheckoutService(
		repo,
		nativeCheckoutTestResolver(provider),
		&nativeCheckoutUserRepoFake{user: &User{ID: 42, Email: "buyer@example.com"}},
		&nativeCheckoutRedeemerFake{},
		nativeCheckoutTestContactKey,
	)

	_, err := svc.CreateOrder(context.Background(), 42, repo.offer.Code, NativeCheckoutPaymentMethodWeChat)
	require.NoError(t, err)
	require.Empty(t, provider.payType, "providers that pick their own channel must not receive the pay type hint")
	require.Equal(t, 1, provider.createCalls)
}

// easyPayMintRedeemerFake mirrors the database state after a real mint: the
// only codes that exist are ones the repo fake minted, with semantics copied
// from the order snapshot and external_order_no set to the trade no.
type easyPayMintRedeemerFake struct {
	repo        *nativeCheckoutRepoFake
	mu          sync.Mutex
	byCode      map[string]*RedeemCode
	nextID      int64
	redeemCalls int
}

func newEasyPayMintRedeemer(repo *nativeCheckoutRepoFake) *easyPayMintRedeemerFake {
	return &easyPayMintRedeemerFake{repo: repo, byCode: make(map[string]*RedeemCode), nextID: 100}
}

func (r *easyPayMintRedeemerFake) GetByCode(_ context.Context, code string) (*RedeemCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.byCode[code]; ok {
		copy := *existing
		return &copy, nil
	}
	r.repo.mu.Lock()
	order := r.repo.order
	tradeNo, minted := "", false
	for candidateTradeNo, candidateCode := range r.repo.mintedCodes {
		if candidateCode == code {
			tradeNo, minted = candidateTradeNo, true
			break
		}
	}
	r.repo.mu.Unlock()
	if !minted || order == nil {
		return nil, ErrRedeemCodeNotFound
	}
	created := &RedeemCode{
		ID: r.nextID, Code: code, Type: order.RedeemType, Value: order.RedeemValue,
		PaidValue: order.RedeemPaidValue, Status: StatusUnused, Purpose: order.RedeemPurpose,
		SalesStatus: order.RedeemSalesStatus, GroupIDs: append([]int64(nil), order.RedeemGroupIDs...),
		ValidityDays: order.RedeemValidityDays, ExternalOrderNo: tradeNo,
	}
	r.nextID++
	r.byCode[code] = created
	copy := *created
	return &copy, nil
}

func (r *easyPayMintRedeemerFake) GetByID(_ context.Context, id int64) (*RedeemCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.byCode {
		if existing.ID == id {
			copy := *existing
			return &copy, nil
		}
	}
	return nil, ErrRedeemCodeNotFound
}

func (r *easyPayMintRedeemerFake) Redeem(_ context.Context, userID int64, code string) (*RedeemCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.redeemCalls++
	existing, ok := r.byCode[code]
	if !ok {
		return nil, ErrRedeemCodeNotFound
	}
	existing.Status = StatusUsed
	existing.UsedBy = &userID
	copy := *existing
	return &copy, nil
}

var nativeCheckoutMintedCodePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func TestNativeCheckoutEasyPayPaidOrderMintsAndCompletes(t *testing.T) {
	offer := testNativeCheckoutEasyPayOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	provider := &nativeCheckoutProviderFake{info: &NativeCheckoutProviderOrderInfo{
		// EasyPay's query shape: no goods key, contact, delivery, or codes.
		TradeNo: repo.order.ProviderTradeNo, TotalCNYFen: 500, Paid: true,
	}}
	redeemer := newEasyPayMintRedeemer(repo)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(provider), &nativeCheckoutUserRepoFake{}, redeemer, nativeCheckoutTestContactKey)

	completed, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusCompleted, completed.Status)
	require.Equal(t, 1, repo.mintCalls)

	minted := repo.mintedCodes[repo.order.ProviderTradeNo]
	require.Regexp(t, nativeCheckoutMintedCodePattern, minted, "minted codes use the same 32-hex format as card-shop stock")
	require.Equal(t, 1, redeemer.redeemCalls)
	redeemed := redeemer.byCode[minted]
	require.NotNil(t, redeemed)
	require.Equal(t, StatusUsed, redeemed.Status)
	require.NotNil(t, redeemed.UsedBy)
	require.Equal(t, repo.order.UserID, *redeemed.UsedBy)
	require.Equal(t, repo.order.ProviderTradeNo, repo.linkedTradeNo)
	require.NotNil(t, completed.RedeemCodeID)
}

func TestNativeCheckoutEasyPayMintIsIdempotent(t *testing.T) {
	offer := testNativeCheckoutEasyPayOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(&nativeCheckoutProviderFake{}), &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	first, err := svc.ensureMintedNativeRedeemCode(context.Background(), repo.order)
	require.NoError(t, err)
	second, err := svc.ensureMintedNativeRedeemCode(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, repo.mintCalls, "a second reconcile pass must reuse the committed mint")
}

func TestNativeCheckoutEasyPayCrashResumeFromCheckingDoesNotRemint(t *testing.T) {
	offer := testNativeCheckoutEasyPayOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	repo.order.Status = NativeCheckoutStatusChecking
	// A previous attempt minted the code and then crashed before fulfillment.
	seeded, err := repo.MintRedeemCode(context.Background(), repo.order, "0123456789abcdef0123456789abcdef")
	require.NoError(t, err)
	provider := &nativeCheckoutProviderFake{info: &NativeCheckoutProviderOrderInfo{
		TradeNo: repo.order.ProviderTradeNo, TotalCNYFen: 500, Paid: true,
	}}
	redeemer := newEasyPayMintRedeemer(repo)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(provider), &nativeCheckoutUserRepoFake{}, redeemer, nativeCheckoutTestContactKey)

	completed, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusCompleted, completed.Status)
	require.Equal(t, 1, repo.mintCalls, "the resume must reuse the seeded mint, not mint again")
	require.Equal(t, seeded, redeemer.byCode[seeded].Code)
	require.Equal(t, 1, redeemer.redeemCalls)
}

func TestNativeCheckoutEasyPayQueryAmountMismatchIsHeldForReview(t *testing.T) {
	offer := testNativeCheckoutEasyPayOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	provider := &nativeCheckoutProviderFake{info: &NativeCheckoutProviderOrderInfo{
		TradeNo: repo.order.ProviderTradeNo, TotalCNYFen: 999, Paid: true,
	}}
	redeemer := newEasyPayMintRedeemer(repo)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(provider), &nativeCheckoutUserRepoFake{}, redeemer, nativeCheckoutTestContactKey)

	order, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusManualReview, order.Status)
	require.Equal(t, "provider_order_mismatch", order.FailureCode)
	require.Zero(t, repo.mintCalls, "an amount mismatch must never mint an entitlement")
	require.Zero(t, redeemer.redeemCalls)
}

func TestNativeCheckoutEasyPayUnpaidOrderKeepsPolling(t *testing.T) {
	offer := testNativeCheckoutEasyPayOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	provider := &nativeCheckoutProviderFake{info: &NativeCheckoutProviderOrderInfo{
		TradeNo: repo.order.ProviderTradeNo, TotalCNYFen: 500, Paid: false,
	}}
	redeemer := newEasyPayMintRedeemer(repo)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(provider), &nativeCheckoutUserRepoFake{}, redeemer, nativeCheckoutTestContactKey)

	order, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusPending, order.Status)
	require.Zero(t, repo.mintCalls)
	require.Zero(t, redeemer.redeemCalls)
}

func TestNativeCheckoutEasyPayMintFailureIsHeldForReview(t *testing.T) {
	offer := testNativeCheckoutEasyPayOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	repo.mintErr = errors.New("db down")
	provider := &nativeCheckoutProviderFake{info: &NativeCheckoutProviderOrderInfo{
		TradeNo: repo.order.ProviderTradeNo, TotalCNYFen: 500, Paid: true,
	}}
	redeemer := newEasyPayMintRedeemer(repo)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(provider), &nativeCheckoutUserRepoFake{}, redeemer, nativeCheckoutTestContactKey)

	order, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusManualReview, order.Status)
	require.Equal(t, "redeem_mint_failed", order.FailureCode)
	require.Zero(t, redeemer.redeemCalls, "a failed mint must never redeem anything")
}

func TestNativeCheckoutEasyPayMintedCodeMissingFromLookupIsHeld(t *testing.T) {
	offer := testNativeCheckoutEasyPayOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	provider := &nativeCheckoutProviderFake{info: &NativeCheckoutProviderOrderInfo{
		TradeNo: repo.order.ProviderTradeNo, TotalCNYFen: 500, Paid: true,
	}}
	// A plain redeemer that finds nothing simulates mint/lookup divergence.
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(provider), &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	order, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusManualReview, order.Status)
	require.Equal(t, "redeem_code_not_found", order.FailureCode)
	require.Equal(t, 1, repo.mintCalls)
}

func TestNativeCheckoutHandleEasyPayNotifyNudgesReconcile(t *testing.T) {
	offer := testNativeCheckoutEasyPayOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(&nativeCheckoutProviderFake{}), &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	err := svc.HandleEasyPayNotify(context.Background(), &payment.NotifyResult{
		OutTradeNo: repo.order.OrderNo, TradeNo: "EP-9", Method: "alipay", AmountCNYFen: 500, Paid: true,
	})
	require.NoError(t, err)
	require.Equal(t, []int64{repo.order.ID}, repo.nudgedOrderIDs, "a verified paid notify must wake the reconcile worker immediately")
}

func TestNativeCheckoutHandleEasyPayNotifyAmountMismatchFailsForRetry(t *testing.T) {
	offer := testNativeCheckoutEasyPayOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(&nativeCheckoutProviderFake{}), &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	err := svc.HandleEasyPayNotify(context.Background(), &payment.NotifyResult{
		OutTradeNo: repo.order.OrderNo, AmountCNYFen: 501, Paid: true,
	})
	require.ErrorIs(t, err, ErrNativeCheckoutAmountMismatch)
	require.Empty(t, repo.nudgedOrderIDs, "a mismatched amount must not schedule fulfillment")
}

func TestNativeCheckoutHandleEasyPayNotifyIgnoresUnpaidStatus(t *testing.T) {
	offer := testNativeCheckoutEasyPayOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(&nativeCheckoutProviderFake{}), &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	err := svc.HandleEasyPayNotify(context.Background(), &payment.NotifyResult{
		OutTradeNo: repo.order.OrderNo, AmountCNYFen: 500, Paid: false,
	})
	require.NoError(t, err, "non-success statuses are acknowledged so the platform stops retrying")
	require.Empty(t, repo.nudgedOrderIDs)
}

func TestNativeCheckoutHandleEasyPayNotifyRejectsForeignProviderOrder(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutOffer()) // ldxp offer/order
	repo.order = testNativeCheckoutOrder(repo.offer)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(&nativeCheckoutProviderFake{}), &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	err := svc.HandleEasyPayNotify(context.Background(), &payment.NotifyResult{
		OutTradeNo: repo.order.OrderNo, AmountCNYFen: 500, Paid: true,
	})
	require.ErrorIs(t, err, ErrNativeCheckoutProviderMismatch)
	require.Empty(t, repo.nudgedOrderIDs)
}

func TestNativeCheckoutHandleEasyPayNotifyUnknownOrder(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutEasyPayOffer())
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(&nativeCheckoutProviderFake{}), &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	err := svc.HandleEasyPayNotify(context.Background(), &payment.NotifyResult{
		OutTradeNo: "NC-does-not-exist", AmountCNYFen: 500, Paid: true,
	})
	require.ErrorIs(t, err, ErrNativeCheckoutOrderNotFound)

	err = svc.HandleEasyPayNotify(context.Background(), &payment.NotifyResult{OutTradeNo: "  ", Paid: true})
	require.Error(t, err)
	require.Equal(t, "EASYPAY_INVALID_NOTIFY", infraerrors.Reason(err))
}

// testNativeCheckoutEasyPaySubscriptionOffer models a monthly card sold through
// EasyPay native checkout: a paid sale (not a gift) granting subscription
// groups for 31 days, repurchasable.
func testNativeCheckoutEasyPaySubscriptionOffer() NativeCheckoutOffer {
	return NativeCheckoutOffer{
		Code: "plus", Provider: NativeCheckoutProviderEasyPay, ProviderGoodsKey: "plus",
		Name: "Plus 月卡", ProductKind: RedeemTypeSubscription,
		PayAmountCNYFen: 25900, BenefitAmountCNYFen: 25900,
		RedeemType: RedeemTypeSubscription, RedeemValue: 259, RedeemPaidValue: 0,
		RedeemPurpose: RedeemCodePurposeSaleRecharge, RedeemSalesStatus: RedeemCodeSalesStatusSold,
		RedeemGroupIDs: []int64{11, 22}, RedeemValidityDays: 31,
		OncePerUser: false, Enabled: true,
	}
}

func TestNativeCheckoutEasyPaySubscriptionOrderMintsAndCompletes(t *testing.T) {
	offer := testNativeCheckoutEasyPaySubscriptionOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutEasyPayOrder(offer)
	provider := &nativeCheckoutProviderFake{info: &NativeCheckoutProviderOrderInfo{
		TradeNo: repo.order.ProviderTradeNo, TotalCNYFen: 25900, Paid: true,
	}}
	redeemer := newEasyPayMintRedeemer(repo)
	svc := NewNativeCheckoutService(repo, nativeCheckoutTestResolver(provider), &nativeCheckoutUserRepoFake{}, redeemer, nativeCheckoutTestContactKey)

	completed, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusCompleted, completed.Status)
	require.Equal(t, 1, repo.mintCalls)

	minted := repo.mintedCodes[repo.order.ProviderTradeNo]
	require.Regexp(t, nativeCheckoutMintedCodePattern, minted)
	redeemed := redeemer.byCode[minted]
	require.NotNil(t, redeemed)
	// The minted subscription code must carry the order snapshot's groups,
	// validity and sale semantics — this is what RedeemService turns into the
	// actual subscription assignment.
	require.Equal(t, RedeemTypeSubscription, redeemed.Type)
	require.Equal(t, []int64{11, 22}, redeemed.GroupIDs)
	require.Equal(t, 31, redeemed.ValidityDays)
	require.Equal(t, float64(259), redeemed.Value)
	require.Zero(t, redeemed.PaidValue)
	require.Equal(t, RedeemCodePurposeSaleRecharge, redeemed.Purpose)
	require.Equal(t, RedeemCodeSalesStatusSold, redeemed.SalesStatus)
	require.Equal(t, StatusUsed, redeemed.Status)
	require.NotNil(t, redeemed.UsedBy)
	require.Equal(t, repo.order.UserID, *redeemed.UsedBy)
	require.Equal(t, repo.order.ProviderTradeNo, redeemed.ExternalOrderNo)
}

func TestNativeCheckoutRepeatableOfferAllowsRepurchaseAfterCompletion(t *testing.T) {
	offer := testNativeCheckoutEasyPaySubscriptionOffer()
	repo := newNativeCheckoutRepoFake(offer)
	provider := &nativeCheckoutProviderFake{
		created: &NativeCheckoutProviderOrder{
			TradeNo: "NC-echo", PaymentURL: "https://pay.example.com/cashier/x", PaymentMethod: NativeCheckoutPaymentMethodAlipay,
		},
	}
	svc := NewNativeCheckoutService(
		repo,
		nativeCheckoutTestResolver(provider),
		&nativeCheckoutUserRepoFake{user: &User{ID: 42, Email: "buyer@example.com"}},
		&nativeCheckoutRedeemerFake{},
		nativeCheckoutTestContactKey,
	)

	first, err := svc.CreateOrder(context.Background(), 42, offer.Code, "")
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusPending, first.Status)

	// The first purchase completes; the same account buys the same monthly card
	// again (renewal). A new durable order must be reserved.
	repo.mu.Lock()
	repo.order.Status = NativeCheckoutStatusCompleted
	repo.mu.Unlock()

	second, err := svc.CreateOrder(context.Background(), 42, offer.Code, NativeCheckoutPaymentMethodWeChat)
	require.NoError(t, err)
	require.NotEqual(t, first.OrderNo, second.OrderNo, "a completed order must not block repurchase of a repeatable offer")
	require.NotEqual(t, first.ID, second.ID)
	require.Equal(t, 2, provider.createCalls)
	require.Equal(t, NativeCheckoutPaymentMethodWeChat, provider.payType)

	// A repeatable offer is never "claimed": the catalog must keep it buyable.
	views, err := svc.ListOffers(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, views, 1)
	require.False(t, views[0].Claimed, "repurchasable offers are never marked claimed")
}

func TestNativeCheckoutRepeatableOfferBlocksConcurrentActiveOrder(t *testing.T) {
	offer := testNativeCheckoutEasyPaySubscriptionOffer()
	repo := newNativeCheckoutRepoFake(offer)
	provider := &nativeCheckoutProviderFake{
		created: &NativeCheckoutProviderOrder{
			TradeNo: "NC-echo", PaymentURL: "https://pay.example.com/cashier/x", PaymentMethod: NativeCheckoutPaymentMethodAlipay,
		},
	}
	svc := NewNativeCheckoutService(
		repo,
		nativeCheckoutTestResolver(provider),
		&nativeCheckoutUserRepoFake{user: &User{ID: 42, Email: "buyer@example.com"}},
		&nativeCheckoutRedeemerFake{},
		nativeCheckoutTestContactKey,
	)

	first, err := svc.CreateOrder(context.Background(), 42, offer.Code, "")
	require.NoError(t, err)
	second, err := svc.CreateOrder(context.Background(), 42, offer.Code, "")
	require.NoError(t, err)
	require.Equal(t, first.OrderNo, second.OrderNo, "an in-flight order is reused instead of duplicated")
	require.Equal(t, 1, provider.createCalls)
}

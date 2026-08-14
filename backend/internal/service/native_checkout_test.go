package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

const nativeCheckoutTestContactKey = "test-native-checkout-contact-key"

func TestNativeCheckoutCreateUsesRegisteredEmailAndReusesOnceOnlyOrder(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutOffer())
	provider := &nativeCheckoutProviderFake{
		created: &NativeCheckoutProviderOrder{
			TradeNo: "LD-1", PaymentURL: "https://pay.ldxp.cn/pay/LD-1", PaymentMethod: NativeCheckoutPaymentMethodWeChat,
		},
	}
	service := NewNativeCheckoutService(
		repo,
		provider,
		&nativeCheckoutUserRepoFake{user: &User{ID: 42, Email: "Buyer@Example.com"}},
		&nativeCheckoutRedeemerFake{},
		nativeCheckoutTestContactKey,
	)

	first, err := service.CreateOrder(context.Background(), 42, "newcomer-balance-5-to-10")
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusPending, first.Status)
	require.Equal(t, NativeCheckoutPaymentMethodWeChat, first.PaymentMethod)
	require.Equal(t, "buyer@example.com", provider.contact)
	require.Equal(t, 1, provider.createCalls)
	require.NotEqual(t, "", first.ContactHash)

	second, err := service.CreateOrder(context.Background(), 42, "newcomer-balance-5-to-10")
	require.NoError(t, err)
	require.Equal(t, first.OrderNo, second.OrderNo)
	require.Equal(t, 1, provider.createCalls, "a repeated click must not create another LDXP order")
}

func TestNativeCheckoutListDoesNotCallProviderWhileRenderingCatalog(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutOffer())
	provider := &nativeCheckoutProviderFake{validateErr: errors.New("product offline")}
	svc := NewNativeCheckoutService(repo, provider, &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	offers, err := svc.ListOffers(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, offers, 1)
}

func TestNativeCheckoutRecoveredRedeemCountsAsOnceOnlyPurchase(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutOffer())
	repo.redeemed = true
	provider := &nativeCheckoutProviderFake{}
	svc := NewNativeCheckoutService(
		repo,
		provider,
		&nativeCheckoutUserRepoFake{user: &User{ID: 42, Email: "buyer@example.com"}},
		&nativeCheckoutRedeemerFake{},
		nativeCheckoutTestContactKey,
	)

	offers, err := svc.ListOffers(context.Background(), 42)
	require.NoError(t, err)
	require.Len(t, offers, 1)
	require.True(t, offers[0].Claimed)
	require.Nil(t, offers[0].Order)

	_, err = svc.CreateOrder(context.Background(), 42, repo.offer.Code)
	require.ErrorIs(t, err, ErrNativeCheckoutAlreadyClaimed)
	require.Zero(t, provider.createCalls)

	// A stale local order must not reopen its QR after an out-of-band recovery.
	repo.order = testNativeCheckoutOrder(repo.offer)
	offers, err = svc.ListOffers(context.Background(), 42)
	require.NoError(t, err)
	require.True(t, offers[0].Claimed)
	_, err = svc.CreateOrder(context.Background(), 42, repo.offer.Code)
	require.ErrorIs(t, err, ErrNativeCheckoutAlreadyClaimed)
}

func TestNativeCheckoutCreateFinishesAfterRequestCancellation(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutOffer())
	provider := &nativeCheckoutProviderFake{
		created: &NativeCheckoutProviderOrder{
			TradeNo: "LD-1", PaymentURL: "https://pay.ldxp.cn/pay/LD-1", PaymentMethod: NativeCheckoutPaymentMethodWeChat,
		},
	}
	svc := NewNativeCheckoutService(
		repo,
		provider,
		&nativeCheckoutUserRepoFake{user: &User{ID: 42, Email: "buyer@example.com"}},
		&nativeCheckoutRedeemerFake{},
		nativeCheckoutTestContactKey,
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	order, err := svc.CreateOrder(ctx, 42, "newcomer-balance-5-to-10")
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusPending, order.Status)
	require.NoError(t, provider.createContextErr, "the durable provider operation must be detached from the browser request")
}

func TestNativeCheckoutCreateRejectsUnlabeledProviderPaymentMethod(t *testing.T) {
	repo := newNativeCheckoutRepoFake(testNativeCheckoutOffer())
	provider := &nativeCheckoutProviderFake{
		created: &NativeCheckoutProviderOrder{
			TradeNo: "LD-UNKNOWN", PaymentURL: "https://pay.ldxp.cn/pay/LD-UNKNOWN", PaymentMethod: "",
		},
	}
	svc := NewNativeCheckoutService(
		repo,
		provider,
		&nativeCheckoutUserRepoFake{user: &User{ID: 42, Email: "buyer@example.com"}},
		&nativeCheckoutRedeemerFake{},
		nativeCheckoutTestContactKey,
	)

	_, err := svc.CreateOrder(context.Background(), 42, "newcomer-balance-5-to-10")
	require.Error(t, err)
	require.Equal(t, NativeCheckoutStatusManualReview, repo.order.Status)
	require.Equal(t, "provider_payment_method_invalid", repo.order.FailureCode)
}

func TestNativeCheckoutStaleCreatingOrderIsHeldForReviewWithoutRetry(t *testing.T) {
	offer := testNativeCheckoutOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutOrder(offer)
	repo.order.Status = NativeCheckoutStatusCreating
	repo.order.ProviderTradeNo = ""
	repo.order.PaymentURL = ""
	repo.order.UpdatedAt = time.Now().Add(-2 * time.Minute)
	provider := &nativeCheckoutProviderFake{}
	svc := NewNativeCheckoutService(repo, provider, &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	order, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusManualReview, order.Status)
	require.Equal(t, "provider_create_interrupted", order.FailureCode)
	require.Zero(t, provider.createCalls, "an ambiguous interrupted provider create must never be retried")
}

func TestNativeCheckoutPaidGiftCardIsValidatedLinkedAndRedeemed(t *testing.T) {
	offer := testNativeCheckoutOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutOrder(offer)
	provider := &nativeCheckoutProviderFake{
		paid: true,
		info: &NativeCheckoutProviderOrderInfo{
			TradeNo: "LD-1", GoodsKey: offer.ProviderGoodsKey, Contact: "buyer@example.com",
			Quantity: 1, TotalCNYFen: 500, Paid: true, Delivered: true,
			RedeemCodes: []string{"0123456789abcdef0123456789abcdef"},
		},
	}
	redeem := &nativeCheckoutRedeemerFake{code: &RedeemCode{
		ID: 7, Code: provider.info.RedeemCodes[0], Type: RedeemTypeBalance, Value: 10,
		PaidValue: 0, Status: StatusUnused, Purpose: RedeemCodePurposeGift,
		SalesStatus: RedeemCodeSalesStatusGifted, ValidityDays: 0,
	}}
	svc := NewNativeCheckoutService(repo, provider, &nativeCheckoutUserRepoFake{}, redeem, nativeCheckoutTestContactKey)

	order, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusCompleted, order.Status)
	require.Equal(t, 1, redeem.redeemCalls)
	require.Equal(t, "LD-1", repo.linkedTradeNo)
	require.Equal(t, int64(7), *order.RedeemCodeID)
}

func TestNativeCheckoutPaidOrderStaysCheckingWhileDeliveryIsPending(t *testing.T) {
	offer := testNativeCheckoutOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutOrder(offer)
	provider := &nativeCheckoutProviderFake{info: &NativeCheckoutProviderOrderInfo{
		TradeNo: "LD-1", GoodsKey: offer.ProviderGoodsKey, Contact: "buyer@example.com",
		Quantity: 1, TotalCNYFen: 500, Paid: true, Delivered: false,
	}}
	svc := NewNativeCheckoutService(repo, provider, &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	order, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusChecking, order.Status)
	require.Empty(t, order.FailureCode)
}

func TestNativeCheckoutRejectsPaidCardThatIsNotPureGift(t *testing.T) {
	offer := testNativeCheckoutOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutOrder(offer)
	provider := &nativeCheckoutProviderFake{
		paid: true,
		info: &NativeCheckoutProviderOrderInfo{
			TradeNo: "LD-1", GoodsKey: offer.ProviderGoodsKey, Contact: "buyer@example.com",
			Quantity: 1, TotalCNYFen: 500, Paid: true, Delivered: true,
			RedeemCodes: []string{"0123456789abcdef0123456789abcdef"},
		},
	}
	redeem := &nativeCheckoutRedeemerFake{code: &RedeemCode{
		ID: 7, Code: provider.info.RedeemCodes[0], Type: RedeemTypeBalance, Value: 10,
		PaidValue: 1, Status: StatusUnused, Purpose: RedeemCodePurposeSaleRecharge,
		SalesStatus: RedeemCodeSalesStatusSold, ValidityDays: 0,
	}}
	svc := NewNativeCheckoutService(repo, provider, &nativeCheckoutUserRepoFake{}, redeem, nativeCheckoutTestContactKey)

	order, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusManualReview, order.Status)
	require.Equal(t, 0, redeem.redeemCalls)
	require.Equal(t, "redeem_code_mismatch", order.FailureCode)
}

func TestNativeCheckoutCrashRecoveryAcceptsCodeAlreadyUsedBySameUser(t *testing.T) {
	offer := testNativeCheckoutOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutOrder(offer)
	repo.order.Status = NativeCheckoutStatusFulfilling
	repo.order.UpdatedAt = time.Now().Add(-2 * time.Minute)
	codeID := int64(7)
	repo.order.RedeemCodeID = &codeID
	provider := &nativeCheckoutProviderFake{
		info: &NativeCheckoutProviderOrderInfo{
			TradeNo: "LD-1", GoodsKey: offer.ProviderGoodsKey, Contact: "buyer@example.com",
			Quantity: 1, TotalCNYFen: 500, Paid: true, Delivered: true,
			RedeemCodes: []string{"0123456789abcdef0123456789abcdef"},
		},
	}
	usedBy := int64(42)
	redeem := &nativeCheckoutRedeemerFake{code: &RedeemCode{
		ID: 7, Code: provider.info.RedeemCodes[0], Type: RedeemTypeBalance, Value: 10,
		Status: StatusUsed, UsedBy: &usedBy, Purpose: RedeemCodePurposeGift,
		SalesStatus: RedeemCodeSalesStatusGifted, ValidityDays: 0,
	}}
	svc := NewNativeCheckoutService(repo, provider, &nativeCheckoutUserRepoFake{}, redeem, nativeCheckoutTestContactKey)

	order, err := svc.syncOrder(context.Background(), repo.order)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusCompleted, order.Status)
	require.Equal(t, 0, redeem.redeemCalls)
}

func TestNativeCheckoutCustomerStatusReadNeverPollsProvider(t *testing.T) {
	offer := testNativeCheckoutOffer()
	repo := newNativeCheckoutRepoFake(offer)
	repo.order = testNativeCheckoutOrder(offer)
	provider := &nativeCheckoutProviderFake{info: &NativeCheckoutProviderOrderInfo{
		TradeNo: "LD-1", GoodsKey: offer.ProviderGoodsKey, Contact: "buyer@example.com",
		Quantity: 1, TotalCNYFen: 500, Paid: true, Delivered: false,
	}}
	svc := NewNativeCheckoutService(repo, provider, &nativeCheckoutUserRepoFake{}, &nativeCheckoutRedeemerFake{}, nativeCheckoutTestContactKey)

	order, err := svc.GetOrder(context.Background(), 42, repo.order.OrderNo)
	require.NoError(t, err)
	require.Equal(t, NativeCheckoutStatusPending, order.Status)
	require.Zero(t, provider.orderInfoCalls, "a browser status read must not query LDXP")
}

func TestNativeCheckoutPendingPollingCoolsDownWithOrderAge(t *testing.T) {
	svc := NewNativeCheckoutService(nil, nil, nil, nil, nativeCheckoutTestContactKey)
	order := &NativeCheckoutOrder{Status: NativeCheckoutStatusPending, CreatedAt: time.Now().Add(-time.Minute)}
	require.Equal(t, 3*time.Second, svc.nextCheckDelay(order))

	order.CreatedAt = time.Now().Add(-5 * time.Minute)
	require.Equal(t, 10*time.Second, svc.nextCheckDelay(order))
	order.CreatedAt = time.Now().Add(-30 * time.Minute)
	require.Equal(t, 30*time.Second, svc.nextCheckDelay(order))
	order.CreatedAt = time.Now().Add(-2 * time.Hour)
	require.Equal(t, 5*time.Minute, svc.nextCheckDelay(order))
	order.CreatedAt = time.Now().Add(-48 * time.Hour)
	require.Equal(t, time.Hour, svc.nextCheckDelay(order))

	order.Status = NativeCheckoutStatusChecking
	require.Equal(t, 3*time.Second, svc.nextCheckDelay(order), "paid orders wait for delivery on the fast path")
}

func TestRedeemServiceBlocksRestrictedCheckoutInventoryFromManualRedemption(t *testing.T) {
	repo := &nativeCheckoutRedeemCodeRepoFake{code: &RedeemCode{
		ID: 99, Code: "0123456789abcdef0123456789abcdef", Type: RedeemTypeBalance,
		Value: 10, Status: StatusUnused,
	}}
	svc := NewRedeemService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	svc.SetNativeCheckoutRedeemGuard(nativeCheckoutGuardFake(true))

	_, err := svc.Redeem(context.Background(), 42, repo.code.Code)
	require.Error(t, err)
	require.Equal(t, "REDEEM_CODE_CHECKOUT_RESTRICTED", infraerrors.Reason(err))

	// The internal authorization bypasses only the checkout restriction. The
	// normal status validation still runs.
	repo.code.Status = StatusUsed
	_, err = svc.Redeem(withNativeCheckoutRedeemAuthorization(context.Background()), 42, repo.code.Code)
	require.ErrorIs(t, err, ErrRedeemCodeUsed)
}

func testNativeCheckoutOffer() NativeCheckoutOffer {
	return NativeCheckoutOffer{
		Code: "newcomer-balance-5-to-10", Provider: "ldxp", ProviderGoodsKey: "trial-key",
		Name: "新人专享 · 10 元余额包", ProductKind: "balance", PayAmountCNYFen: 500,
		BenefitAmountCNYFen: 1000, RedeemType: RedeemTypeBalance, RedeemValue: 10,
		RedeemPurpose: RedeemCodePurposeGift, RedeemSalesStatus: RedeemCodeSalesStatusGifted,
		RedeemValidityDays: 0, OncePerUser: true, Enabled: true,
	}
}

func testNativeCheckoutOrder(offer NativeCheckoutOffer) *NativeCheckoutOrder {
	return &NativeCheckoutOrder{
		ID: 1, OrderNo: "NC-1", UserID: 42, OfferCode: offer.Code, Provider: offer.Provider,
		ProviderGoodsKey: offer.ProviderGoodsKey, ProviderTradeNo: "LD-1",
		PaymentURL: "https://pay.ldxp.cn/pay/LD-1", ContactHash: hashNativeCheckoutContact(deriveNativeCheckoutContactKey(nativeCheckoutTestContactKey), "buyer@example.com"),
		ProductKind: offer.ProductKind, PayAmountCNYFen: offer.PayAmountCNYFen,
		BenefitAmountCNYFen: offer.BenefitAmountCNYFen, RedeemType: offer.RedeemType,
		RedeemValue: offer.RedeemValue, RedeemPaidValue: offer.RedeemPaidValue,
		RedeemPurpose: offer.RedeemPurpose, RedeemSalesStatus: offer.RedeemSalesStatus,
		RedeemValidityDays: offer.RedeemValidityDays, EnforceOnce: true,
		Status: NativeCheckoutStatusPending, UpdatedAt: time.Now(),
	}
}

type nativeCheckoutRepoFake struct {
	NativeCheckoutRepository
	mu            sync.Mutex
	offer         NativeCheckoutOffer
	order         *NativeCheckoutOrder
	redeemed      bool
	linkedTradeNo string
}

func newNativeCheckoutRepoFake(offer NativeCheckoutOffer) *nativeCheckoutRepoFake {
	return &nativeCheckoutRepoFake{offer: offer}
}

func (r *nativeCheckoutRepoFake) ListVisibleOffers(context.Context, int64) ([]NativeCheckoutOffer, error) {
	return []NativeCheckoutOffer{r.offer}, nil
}

func (r *nativeCheckoutRepoFake) GetVisibleOffer(_ context.Context, _ int64, code string) (*NativeCheckoutOffer, error) {
	if code != r.offer.Code {
		return nil, ErrNativeCheckoutOfferNotFound
	}
	copy := r.offer
	return &copy, nil
}

func (r *nativeCheckoutRepoFake) GetLatestOrderForOffer(context.Context, int64, string) (*NativeCheckoutOrder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.order == nil {
		return nil, ErrNativeCheckoutOrderNotFound
	}
	copy := *r.order
	return &copy, nil
}

func (r *nativeCheckoutRepoFake) HasRedeemedOffer(context.Context, int64, string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.redeemed, nil
}

func (r *nativeCheckoutRepoFake) ReserveOrder(_ context.Context, order *NativeCheckoutOrder) (*NativeCheckoutOrder, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.order != nil {
		copy := *r.order
		return &copy, false, nil
	}
	copy := *order
	copy.ID = 1
	r.order = &copy
	return &copy, true, nil
}

func (r *nativeCheckoutRepoFake) ResetFailedOrder(context.Context, int64, string) (*NativeCheckoutOrder, bool, error) {
	return nil, false, errors.New("unexpected reset")
}

func (r *nativeCheckoutRepoFake) SetProviderOrder(_ context.Context, _ int64, tradeNo, paymentURL, paymentMethod string) (*NativeCheckoutOrder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order.ProviderTradeNo = tradeNo
	r.order.PaymentURL = paymentURL
	r.order.PaymentMethod = paymentMethod
	r.order.Status = NativeCheckoutStatusPending
	copy := *r.order
	return &copy, nil
}

func (r *nativeCheckoutRepoFake) SetOrderState(_ context.Context, _ int64, status, failureCode string, _ time.Time) (*NativeCheckoutOrder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order.Status = status
	r.order.FailureCode = failureCode
	copy := *r.order
	return &copy, nil
}

func (r *nativeCheckoutRepoFake) GetOrderForUser(context.Context, string, int64) (*NativeCheckoutOrder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.order == nil {
		return nil, ErrNativeCheckoutOrderNotFound
	}
	copy := *r.order
	return &copy, nil
}

func (r *nativeCheckoutRepoFake) RecordPendingCheck(context.Context, int64, time.Time) error {
	return nil
}

func (r *nativeCheckoutRepoFake) ClaimFulfillment(_ context.Context, _, redeemCodeID int64, _ time.Time) (*NativeCheckoutOrder, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order.Status = NativeCheckoutStatusFulfilling
	r.order.RedeemCodeID = &redeemCodeID
	copy := *r.order
	return &copy, true, nil
}

func (r *nativeCheckoutRepoFake) LinkRedeemCode(_ context.Context, _ int64, tradeNo string) error {
	r.linkedTradeNo = tradeNo
	return nil
}

func (r *nativeCheckoutRepoFake) CompleteOrder(context.Context, int64) (*NativeCheckoutOrder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.order.Status = NativeCheckoutStatusCompleted
	copy := *r.order
	return &copy, nil
}

type nativeCheckoutProviderFake struct {
	NativeCheckoutProvider
	created          *NativeCheckoutProviderOrder
	paid             bool
	info             *NativeCheckoutProviderOrderInfo
	contact          string
	createCalls      int
	createContextErr error
	validateErr      error
	orderInfoCalls   int
}

func (p *nativeCheckoutProviderFake) ValidateOffer(context.Context, string, int64) error {
	return p.validateErr
}

func (p *nativeCheckoutProviderFake) CreateOrder(ctx context.Context, _, contact string, _ int64) (*NativeCheckoutProviderOrder, error) {
	p.contact = contact
	p.createCalls++
	p.createContextErr = ctx.Err()
	return p.created, nil
}

func (p *nativeCheckoutProviderFake) IsPaid(context.Context, string) (bool, error) {
	return p.paid, nil
}
func (p *nativeCheckoutProviderFake) GetOrderInfo(context.Context, string) (*NativeCheckoutProviderOrderInfo, error) {
	p.orderInfoCalls++
	return p.info, nil
}

type nativeCheckoutUserRepoFake struct {
	UserRepository
	user *User
}

func (r *nativeCheckoutUserRepoFake) GetByID(context.Context, int64) (*User, error) {
	if r.user == nil {
		return &User{ID: 42, Email: "buyer@example.com"}, nil
	}
	return r.user, nil
}

type nativeCheckoutRedeemerFake struct {
	code        *RedeemCode
	redeemCalls int
}

type nativeCheckoutRedeemCodeRepoFake struct {
	RedeemCodeRepository
	code *RedeemCode
}

func (r *nativeCheckoutRedeemCodeRepoFake) GetByCode(context.Context, string) (*RedeemCode, error) {
	copy := *r.code
	return &copy, nil
}

type nativeCheckoutGuardFake bool

func (g nativeCheckoutGuardFake) IsNativeCheckoutRestricted(context.Context, int64) (bool, error) {
	return bool(g), nil
}

func (r *nativeCheckoutRedeemerFake) GetByCode(context.Context, string) (*RedeemCode, error) {
	if r.code == nil {
		return nil, ErrRedeemCodeNotFound
	}
	copy := *r.code
	return &copy, nil
}

func (r *nativeCheckoutRedeemerFake) GetByID(context.Context, int64) (*RedeemCode, error) {
	return r.GetByCode(context.Background(), "")
}

func (r *nativeCheckoutRedeemerFake) Redeem(_ context.Context, userID int64, _ string) (*RedeemCode, error) {
	r.redeemCalls++
	r.code.Status = StatusUsed
	r.code.UsedBy = &userID
	copy := *r.code
	return &copy, nil
}

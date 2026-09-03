package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/mail"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/payment"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

const (
	NativeCheckoutStatusCreating     = "creating"
	NativeCheckoutStatusPending      = "pending"
	NativeCheckoutStatusChecking     = "checking"
	NativeCheckoutStatusFulfilling   = "fulfilling"
	NativeCheckoutStatusCompleted    = "completed"
	NativeCheckoutStatusFailed       = "failed"
	NativeCheckoutStatusManualReview = "manual_review"

	NativeCheckoutPaymentMethodWeChat           = "wechat"
	NativeCheckoutPaymentMethodAlipay           = "alipay"
	NativeCheckoutPaymentMethodCommissionWallet = "commission_wallet"

	// NativeCheckoutProviderLDXP is the LDXP card shop: it collects payment and
	// delivers a redeem code from pre-stocked inventory.
	NativeCheckoutProviderLDXP = "ldxp"
	// NativeCheckoutProviderEasyPay is the EasyPay gateway (彩虹易支付 MD5
	// protocol): it collects payment only; the redeem code is minted locally
	// from the order snapshot after the payment is confirmed.
	NativeCheckoutProviderEasyPay         = payment.ProviderEasyPay
	NativeCheckoutProviderAffiliateWallet = "affiliate_wallet"

	nativeCheckoutWorkerInterval    = time.Second
	nativeCheckoutWorkerLease       = 30 * time.Second
	nativeCheckoutWorkerConcurrency = 4
	nativeCheckoutOrderSyncTimeout  = 20 * time.Second
)

var (
	ErrNativeCheckoutOfferNotFound  = infraerrors.NotFound("NATIVE_CHECKOUT_OFFER_NOT_FOUND", "checkout offer not found")
	ErrNativeCheckoutOrderNotFound  = infraerrors.NotFound("NATIVE_CHECKOUT_ORDER_NOT_FOUND", "checkout order not found")
	ErrNativeCheckoutAlreadyClaimed = infraerrors.Conflict("NATIVE_CHECKOUT_ALREADY_CLAIMED", "checkout offer was already claimed")
	ErrNativeCheckoutUnavailable    = infraerrors.ServiceUnavailable("NATIVE_CHECKOUT_UNAVAILABLE", "checkout is temporarily unavailable")
	ErrNativeCheckoutPriceChanged   = infraerrors.Conflict("NATIVE_CHECKOUT_PRICE_CHANGED", "the live product price changed; refresh and confirm again")

	// ErrNativeCheckoutProviderMismatch / ErrNativeCheckoutAmountMismatch are
	// returned by HandleEasyPayNotify. Any error makes the gateway handler
	// answer "fail" so the platform retries while ops investigates.
	ErrNativeCheckoutProviderMismatch = infraerrors.BadRequest("NATIVE_CHECKOUT_PROVIDER_MISMATCH", "notify provider does not match checkout order provider")
	ErrNativeCheckoutAmountMismatch   = infraerrors.BadRequest("NATIVE_CHECKOUT_AMOUNT_MISMATCH", "notify amount does not match checkout order amount")
)

type NativeCheckoutOffer struct {
	Code                string
	Provider            string
	ProviderGoodsKey    string
	Name                string
	Description         string
	ProductKind         string
	PayAmountCNYFen     int64
	BenefitAmountCNYFen int64
	RedeemType          string
	RedeemValue         float64
	RedeemPaidValue     float64
	RedeemPurpose       string
	RedeemSalesStatus   string
	RedeemGroupIDs      []int64
	RedeemValidityDays  int
	OncePerUser         bool
	Enabled             bool
	SortOrder           int
}

type NativeCheckoutOrder struct {
	ID                   int64
	OrderNo              string
	UserID               int64
	OfferCode            string
	Provider             string
	ProviderGoodsKey     string
	ProviderTradeNo      string
	PaymentURL           string
	PaymentMethod        string
	ContactHash          string
	ProductKind          string
	PayAmountCNYFen      int64
	BenefitAmountCNYFen  int64
	RedeemType           string
	RedeemValue          float64
	RedeemPaidValue      float64
	RedeemPurpose        string
	RedeemSalesStatus    string
	RedeemGroupIDs       []int64
	RedeemValidityDays   int
	EnforceOnce          bool
	Status               string
	RedeemCodeID         *int64
	FailureCode          string
	CheckCount           int
	NextCheckAt          time.Time
	FulfillmentStartedAt *time.Time
	CompletedAt          *time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type NativeCheckoutOfferView struct {
	Code                string
	Provider            string
	Name                string
	Description         string
	ProductKind         string
	PayAmountCNYFen     int64
	BenefitAmountCNYFen int64
	RedeemValidityDays  int
	OncePerUser         bool
	Claimed             bool
	Order               *NativeCheckoutOrder
}

type NativeCheckoutManualOfferStatus struct {
	Code        string
	Claimed     bool
	PurchaseURL string
}

type NativeCheckoutRepository interface {
	ListVisibleOffers(ctx context.Context, userID int64) ([]NativeCheckoutOffer, error)
	GetVisibleOffer(ctx context.Context, userID int64, code string) (*NativeCheckoutOffer, error)
	GetManualRedeemOffer(ctx context.Context, code string) (*NativeCheckoutOffer, error)
	GetLatestOrderForOffer(ctx context.Context, userID int64, offerCode string) (*NativeCheckoutOrder, error)
	HasRedeemedOffer(ctx context.Context, userID int64, offerCode string) (bool, error)
	ReserveOrder(ctx context.Context, order *NativeCheckoutOrder) (*NativeCheckoutOrder, bool, error)
	ResetFailedOrder(ctx context.Context, id int64, contactHash string) (*NativeCheckoutOrder, bool, error)
	SetProviderOrder(ctx context.Context, id int64, providerTradeNo, paymentURL, paymentMethod string) (*NativeCheckoutOrder, error)
	SetOrderState(ctx context.Context, id int64, status, failureCode string, nextCheckAt time.Time) (*NativeCheckoutOrder, error)
	GetOrderForUser(ctx context.Context, orderNo string, userID int64) (*NativeCheckoutOrder, error)
	GetOrder(ctx context.Context, orderNo string) (*NativeCheckoutOrder, error)
	ClaimReconcileOrders(ctx context.Context, limit int, fulfillingStaleBefore, leaseUntil time.Time) ([]NativeCheckoutOrder, error)
	RecordPendingCheck(ctx context.Context, id int64, nextCheckAt time.Time) error
	ClaimFulfillment(ctx context.Context, id, redeemCodeID int64, staleBefore time.Time) (*NativeCheckoutOrder, bool, error)
	LinkRedeemCode(ctx context.Context, redeemCodeID int64, providerTradeNo string) error
	CompleteOrder(ctx context.Context, id int64) (*NativeCheckoutOrder, error)
	// FindMintedRedeemCode returns the code string previously minted for a
	// provider trade no (redeem_codes.external_order_no), or
	// ErrRedeemCodeNotFound when no code has been minted yet.
	FindMintedRedeemCode(ctx context.Context, providerTradeNo string) (string, error)
	// MintRedeemCode atomically creates the redeem code and its
	// native_checkout_redeem_inventory row for a paid provider-collected
	// order. Semantics are copied from the order snapshot. A unique conflict
	// on external_order_no (concurrent mint) re-reads and returns the
	// already-minted code.
	MintRedeemCode(ctx context.Context, order *NativeCheckoutOrder, code string) (string, error)
	// NudgeReconcileNow moves a pending/checking order's next_check_at to now
	// so the reconcile worker re-queries the provider immediately. It does not
	// touch updated_at (lease staleness) or check_count.
	NudgeReconcileNow(ctx context.Context, orderID int64) error
	SetCommissionWalletOrderReady(ctx context.Context, id int64, tradeNo string) (*NativeCheckoutOrder, error)
}

// NativeCheckoutCreateRequest carries everything a provider needs to open a
// payable order for a native checkout reservation. OrderNo is our durable
// NC- order number: providers that accept a merchant order reference (EasyPay
// out_trade_no) must use it so asynchronous notifies can be matched back to
// the reservation. Providers without such a reference (LDXP) ignore it.
// PayType is the customer-selected channel (alipay/wechat); an empty PayType
// lets the provider pick its channel.
type NativeCheckoutCreateRequest struct {
	OrderNo              string
	GoodsKey             string
	Contact              string
	ExpectedAmountCNYFen int64
	PayType              string
	ClientIP             string
}

type NativeCheckoutProviderOrder struct {
	TradeNo       string
	PaymentURL    string
	PaymentMethod string
}

type NativeCheckoutProviderOrderInfo struct {
	TradeNo     string
	GoodsKey    string
	Contact     string
	Quantity    int
	TotalCNYFen int64
	Paid        bool
	Delivered   bool
	RedeemCodes []string
}

// NativeCheckoutProviderError marks whether the external order request may
// have reached the provider.  Ambiguous create failures must never be retried
// automatically because doing so could create two payable orders.
type NativeCheckoutProviderError struct {
	Ambiguous bool
	Cause     error
}

func (e *NativeCheckoutProviderError) Error() string {
	if e == nil || e.Cause == nil {
		return "native checkout provider error"
	}
	return e.Cause.Error()
}

func (e *NativeCheckoutProviderError) Unwrap() error { return e.Cause }

type NativeCheckoutProvider interface {
	ValidateOffer(ctx context.Context, goodsKey string, expectedAmountCNYFen int64) error
	CreateOrder(ctx context.Context, req *NativeCheckoutCreateRequest) (*NativeCheckoutProviderOrder, error)
	IsPaid(ctx context.Context, tradeNo string) (bool, error)
	GetOrderInfo(ctx context.Context, tradeNo string) (*NativeCheckoutProviderOrderInfo, error)
	FetchDirectPaymentQR(ctx context.Context, paymentURL string) ([]byte, string, error)
}

// NativeCheckoutProviderResolver resolves the provider implementation recorded
// on an offer or order row. Native checkout offers of different providers run
// through the same order lifecycle, so every provider call resolves by the
// durable provider column instead of a single injected implementation.
type NativeCheckoutProviderResolver interface {
	ProviderFor(provider string) (NativeCheckoutProvider, error)
}

type nativeCheckoutProviderMap map[string]NativeCheckoutProvider

// NewNativeCheckoutProviderResolver builds a resolver from a static provider
// table. Nil entries are dropped so a missing provider resolves to a coded
// error instead of a nil-interface panic.
func NewNativeCheckoutProviderResolver(providers map[string]NativeCheckoutProvider) NativeCheckoutProviderResolver {
	table := make(nativeCheckoutProviderMap, len(providers))
	for name, provider := range providers {
		name = strings.TrimSpace(name)
		if name == "" || provider == nil {
			continue
		}
		table[name] = provider
	}
	return table
}

func (m nativeCheckoutProviderMap) ProviderFor(provider string) (NativeCheckoutProvider, error) {
	if p, ok := m[strings.TrimSpace(provider)]; ok && p != nil {
		return p, nil
	}
	return nil, ErrNativeCheckoutUnavailable.WithCause(fmt.Errorf("unsupported native checkout provider %q", provider))
}

type NativeCheckoutRedeemer interface {
	GetByCode(ctx context.Context, code string) (*RedeemCode, error)
	GetByID(ctx context.Context, id int64) (*RedeemCode, error)
	Redeem(ctx context.Context, userID int64, code string) (*RedeemCode, error)
}

type NativeCheckoutService struct {
	repo            NativeCheckoutRepository
	providers       NativeCheckoutProviderResolver
	userRepo        UserRepository
	redeem          NativeCheckoutRedeemer
	affiliateWallet *AffiliateWalletService
	contactKey      []byte
	pollInterval    time.Duration
	staleAfter      time.Duration

	workerMu sync.Mutex
	stopCh   chan struct{}
	doneCh   chan struct{}
}

func (s *NativeCheckoutService) SetAffiliateWalletService(wallet *AffiliateWalletService) {
	if s != nil {
		s.affiliateWallet = wallet
	}
}

type nativeCheckoutRedeemAuthorizationKey struct{}
type affiliateCommissionPurchaseAuthorizationKey struct{}

func withNativeCheckoutRedeemAuthorization(ctx context.Context) context.Context {
	return context.WithValue(ctx, nativeCheckoutRedeemAuthorizationKey{}, true)
}

func nativeCheckoutRedeemAuthorized(ctx context.Context) bool {
	allowed, _ := ctx.Value(nativeCheckoutRedeemAuthorizationKey{}).(bool)
	return allowed
}

func withAffiliateCommissionPurchaseAuthorization(ctx context.Context) context.Context {
	return context.WithValue(ctx, affiliateCommissionPurchaseAuthorizationKey{}, true)
}

func affiliateCommissionPurchaseAuthorized(ctx context.Context) bool {
	allowed, _ := ctx.Value(affiliateCommissionPurchaseAuthorizationKey{}).(bool)
	return allowed
}

func NewNativeCheckoutService(
	repo NativeCheckoutRepository,
	providers NativeCheckoutProviderResolver,
	userRepo UserRepository,
	redeem NativeCheckoutRedeemer,
	contactHashKey string,
) *NativeCheckoutService {
	return &NativeCheckoutService{
		repo:         repo,
		providers:    providers,
		userRepo:     userRepo,
		redeem:       redeem,
		contactKey:   deriveNativeCheckoutContactKey(contactHashKey),
		pollInterval: 3 * time.Second,
		staleAfter:   time.Minute,
	}
}

func (s *NativeCheckoutService) ListOffers(ctx context.Context, userID int64) ([]NativeCheckoutOfferView, error) {
	offers, err := s.repo.ListVisibleOffers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list native checkout offers: %w", err)
	}
	views := make([]NativeCheckoutOfferView, 0, len(offers))
	for i := range offers {
		offer := offers[i]
		// Do not query LDXP while merely rendering the recharge page. Provider
		// availability, exact price, contact format, and payment channel are all
		// validated again inside CreateOrder before any payable order is returned.
		// Keeping catalog reads local prevents idle pages from rate-limiting the
		// payment provider for every customer.
		order, orderErr := s.repo.GetLatestOrderForOffer(ctx, userID, offer.Code)
		if orderErr != nil && !errors.Is(orderErr, ErrNativeCheckoutOrderNotFound) {
			return nil, fmt.Errorf("get native checkout order: %w", orderErr)
		}
		// "Claimed" is the lifetime once-only entitlement. Repeatable offers
		// (e.g. monthly cards) are never claimed: a completed order must not
		// block or label the next purchase.
		claimed := offer.OncePerUser && order != nil && order.Status == NativeCheckoutStatusCompleted
		if !claimed && offer.OncePerUser {
			claimed, orderErr = s.repo.HasRedeemedOffer(ctx, userID, offer.Code)
			if orderErr != nil {
				return nil, fmt.Errorf("check native checkout entitlement: %w", orderErr)
			}
		}
		views = append(views, NativeCheckoutOfferView{
			Code:                offer.Code,
			Provider:            offer.Provider,
			Name:                offer.Name,
			Description:         offer.Description,
			ProductKind:         offer.ProductKind,
			PayAmountCNYFen:     offer.PayAmountCNYFen,
			BenefitAmountCNYFen: offer.BenefitAmountCNYFen,
			RedeemValidityDays:  offer.RedeemValidityDays,
			OncePerUser:         offer.OncePerUser,
			Claimed:             claimed,
			Order:               order,
		})
	}
	return views, nil
}

func (s *NativeCheckoutService) GetManualOfferStatus(ctx context.Context, userID int64, offerCode string, includePurchaseURL bool) (*NativeCheckoutManualOfferStatus, error) {
	offerCode = strings.TrimSpace(offerCode)
	if offerCode == "" {
		return nil, infraerrors.BadRequest("NATIVE_CHECKOUT_OFFER_REQUIRED", "checkout offer is required")
	}
	offer, err := s.repo.GetManualRedeemOffer(ctx, offerCode)
	if err != nil {
		if errors.Is(err, ErrNativeCheckoutOfferNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("get manual checkout offer: %w", err)
	}
	claimed := false
	if offer.OncePerUser {
		claimed, err = s.repo.HasRedeemedOffer(ctx, userID, offer.Code)
		if err != nil {
			return nil, fmt.Errorf("check manual checkout entitlement: %w", err)
		}
	}
	status := &NativeCheckoutManualOfferStatus{Code: offer.Code, Claimed: claimed}
	if includePurchaseURL && !claimed {
		status.PurchaseURL, err = manualCheckoutPurchaseURL(offer)
		if err != nil {
			return nil, ErrNativeCheckoutUnavailable.WithCause(err)
		}
	}
	return status, nil
}

func manualCheckoutPurchaseURL(offer *NativeCheckoutOffer) (string, error) {
	if offer == nil {
		return "", errors.New("missing manual checkout offer")
	}
	if !strings.EqualFold(strings.TrimSpace(offer.Provider), NativeCheckoutProviderLDXP) {
		return "", errors.New("unsupported manual checkout provider")
	}
	goodsKey := strings.TrimSpace(offer.ProviderGoodsKey)
	if goodsKey == "" {
		return "", errors.New("missing manual checkout goods key")
	}
	return "https://pay.ldxp.cn/item/" + url.PathEscape(goodsKey), nil
}

func (s *NativeCheckoutService) CreateOrder(ctx context.Context, userID int64, offerCode, payType, clientIP string) (*NativeCheckoutOrder, error) {
	offerCode = strings.TrimSpace(offerCode)
	if offerCode == "" {
		return nil, infraerrors.BadRequest("NATIVE_CHECKOUT_OFFER_REQUIRED", "checkout offer is required")
	}
	offer, err := s.repo.GetVisibleOffer(ctx, userID, offerCode)
	if err != nil {
		if errors.Is(err, ErrNativeCheckoutOfferNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("get native checkout offer: %w", err)
	}
	// The customer-selected payment channel is only meaningful for providers
	// that expose both channels (EasyPay). Providers that pick their own
	// channel (LDXP) receive an empty PayType and ignore the hint entirely.
	payType = strings.TrimSpace(payType)
	if offer.Provider == NativeCheckoutProviderEasyPay {
		if payType == "" {
			payType = NativeCheckoutPaymentMethodAlipay
		}
		if !isSupportedNativeCheckoutPaymentMethod(payType) {
			return nil, infraerrors.BadRequest("NATIVE_CHECKOUT_PAY_TYPE_INVALID", "pay_type must be alipay or wechat")
		}
	} else {
		payType = ""
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get checkout user: %w", err)
	}
	contact, err := normalizedCheckoutEmail(user.Email)
	if err != nil {
		return nil, infraerrors.BadRequest("NATIVE_CHECKOUT_EMAIL_INVALID", "your registered email cannot be used for checkout")
	}
	contactHash := s.hashContact(contact)

	existing, err := s.repo.GetLatestOrderForOffer(ctx, userID, offer.Code)
	if err != nil && !errors.Is(err, ErrNativeCheckoutOrderNotFound) {
		return nil, fmt.Errorf("get existing native checkout order: %w", err)
	}
	if existing != nil && existing.Status == NativeCheckoutStatusCompleted {
		if offer.OncePerUser {
			return existing, nil
		}
		existing = nil
	}
	if offer.OncePerUser {
		claimed, claimedErr := s.repo.HasRedeemedOffer(ctx, userID, offer.Code)
		if claimedErr != nil {
			return nil, fmt.Errorf("check native checkout entitlement: %w", claimedErr)
		}
		if claimed {
			return nil, ErrNativeCheckoutAlreadyClaimed
		}
	}
	if existing != nil && existing.Status != NativeCheckoutStatusFailed {
		return existing, nil
	}
	if existing != nil {
		var reset bool
		existing, reset, err = s.repo.ResetFailedOrder(ctx, existing.ID, contactHash)
		if err != nil {
			return nil, fmt.Errorf("reset native checkout order: %w", err)
		}
		if !reset {
			return existing, nil
		}
		return s.createProviderOrder(ctx, existing, contact, payType, clientIP)
	}

	order := &NativeCheckoutOrder{
		OrderNo:             "NC-" + strings.ReplaceAll(uuid.NewString(), "-", ""),
		UserID:              userID,
		OfferCode:           offer.Code,
		Provider:            offer.Provider,
		ProviderGoodsKey:    offer.ProviderGoodsKey,
		ContactHash:         contactHash,
		ProductKind:         offer.ProductKind,
		PayAmountCNYFen:     offer.PayAmountCNYFen,
		BenefitAmountCNYFen: offer.BenefitAmountCNYFen,
		RedeemType:          offer.RedeemType,
		RedeemValue:         offer.RedeemValue,
		RedeemPaidValue:     offer.RedeemPaidValue,
		RedeemPurpose:       offer.RedeemPurpose,
		RedeemSalesStatus:   offer.RedeemSalesStatus,
		RedeemGroupIDs:      append([]int64(nil), offer.RedeemGroupIDs...),
		RedeemValidityDays:  offer.RedeemValidityDays,
		EnforceOnce:         offer.OncePerUser,
		Status:              NativeCheckoutStatusCreating,
		NextCheckAt:         time.Now(),
	}
	reserved, created, err := s.repo.ReserveOrder(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("reserve native checkout order: %w", err)
	}
	if !created {
		return reserved, nil
	}
	return s.createProviderOrder(ctx, reserved, contact, payType, clientIP)
}

func (s *NativeCheckoutService) CreateCommissionWalletOrder(
	ctx context.Context,
	userID int64,
	offerCode string,
	expectedPriceCNYFen int64,
	idempotencyKey string,
) (*NativeCheckoutOrder, error) {
	if s == nil || s.affiliateWallet == nil {
		return nil, ErrNativeCheckoutUnavailable
	}
	offerCode = strings.TrimSpace(offerCode)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if offerCode == "" || idempotencyKey == "" || len(idempotencyKey) > 180 {
		return nil, ErrInvalidInput
	}
	offer, err := s.repo.GetVisibleOffer(ctx, userID, offerCode)
	if err != nil {
		return nil, err
	}
	if offer.ProductKind != "subscription" || offer.RedeemType != "subscription" {
		return nil, ErrInvalidInput
	}
	if expectedPriceCNYFen != offer.PayAmountCNYFen {
		return nil, ErrNativeCheckoutPriceChanged
	}
	wallet, err := s.affiliateWallet.GetWallet(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !wallet.WalletCheckoutEnabled || wallet.WalletPurchaseRateBPS <= 0 {
		return nil, ErrAffiliateWalletCheckoutDisabled
	}
	chargeFen := ceilPositiveRatio(offer.PayAmountCNYFen, int64(wallet.WalletPurchaseRateBPS), 10_000)
	chargeMicros := chargeFen * 10_000
	if chargeMicros > wallet.AvailableCashMicros {
		return nil, ErrAffiliateInsufficientCash
	}
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	contact, err := normalizedCheckoutEmail(user.Email)
	if err != nil {
		return nil, ErrNativeCheckoutUnavailable.WithCause(err)
	}
	orderNo := commissionWalletOrderNo(userID, idempotencyKey)
	if existing, lookupErr := s.repo.GetOrderForUser(ctx, orderNo, userID); lookupErr == nil {
		return s.syncCommissionWalletOrder(ctx, existing)
	} else if !errors.Is(lookupErr, ErrNativeCheckoutOrderNotFound) {
		return nil, lookupErr
	}
	order := &NativeCheckoutOrder{
		OrderNo: orderNo, UserID: userID, OfferCode: offer.Code,
		Provider: NativeCheckoutProviderAffiliateWallet, ProviderGoodsKey: offer.ProviderGoodsKey,
		ContactHash: s.hashContact(contact), ProductKind: offer.ProductKind,
		PayAmountCNYFen: chargeFen, BenefitAmountCNYFen: offer.BenefitAmountCNYFen,
		RedeemType: offer.RedeemType, RedeemValue: offer.RedeemValue,
		RedeemPaidValue: offer.RedeemPaidValue, RedeemPurpose: offer.RedeemPurpose,
		RedeemSalesStatus: offer.RedeemSalesStatus, RedeemGroupIDs: append([]int64(nil), offer.RedeemGroupIDs...),
		RedeemValidityDays: offer.RedeemValidityDays, EnforceOnce: offer.OncePerUser,
		Status: NativeCheckoutStatusCreating, NextCheckAt: time.Now(),
	}
	reserved, _, err := s.repo.ReserveOrder(ctx, order)
	if err != nil {
		return nil, err
	}
	return s.syncCommissionWalletOrder(ctx, reserved)
}

func ceilPositiveRatio(value, numerator, denominator int64) int64 {
	if value <= 0 || numerator <= 0 || denominator <= 0 {
		return 0
	}
	return (value*numerator + denominator - 1) / denominator
}

func commissionWalletOrderNo(userID int64, idempotencyKey string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s", userID, idempotencyKey)))
	return "CW-" + hex.EncodeToString(sum[:16])
}

func (s *NativeCheckoutService) createProviderOrder(ctx context.Context, order *NativeCheckoutOrder, contact, payType, clientIP string) (*NativeCheckoutOrder, error) {
	// Once the durable reservation exists, finish the provider call and record
	// its outcome even if the browser disconnects. Otherwise a cancelled HTTP
	// request can strand a once-only order forever in "creating".
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	provider, err := s.providers.ProviderFor(order.Provider)
	var providerOrder *NativeCheckoutProviderOrder
	if err == nil {
		providerOrder, err = provider.CreateOrder(opCtx, &NativeCheckoutCreateRequest{
			OrderNo:              order.OrderNo,
			GoodsKey:             order.ProviderGoodsKey,
			Contact:              contact,
			ExpectedAmountCNYFen: order.PayAmountCNYFen,
			PayType:              payType,
			ClientIP:             clientIP,
		})
	}
	if err != nil {
		status := NativeCheckoutStatusFailed
		failureCode := "provider_unavailable"
		var providerErr *NativeCheckoutProviderError
		if errors.As(err, &providerErr) && providerErr.Ambiguous {
			status = NativeCheckoutStatusManualReview
			failureCode = "provider_create_ambiguous"
		}
		updated, updateErr := s.repo.SetOrderState(opCtx, order.ID, status, failureCode, time.Now().Add(s.pollInterval))
		if updateErr != nil {
			return nil, fmt.Errorf("record native checkout create failure: %w", updateErr)
		}
		if status == NativeCheckoutStatusManualReview {
			return updated, nil
		}
		return nil, ErrNativeCheckoutUnavailable.WithCause(err)
	}
	if providerOrder == nil || !isSupportedNativeCheckoutPaymentMethod(providerOrder.PaymentMethod) {
		// The external order may already exist. Do not retry under another channel;
		// hold it for review rather than ever showing a mislabeled QR code.
		_, _ = s.repo.SetOrderState(opCtx, order.ID, NativeCheckoutStatusManualReview, "provider_payment_method_invalid", time.Now())
		return nil, ErrNativeCheckoutUnavailable.WithCause(errors.New("provider returned an unsupported payment method"))
	}
	updated, err := s.repo.SetProviderOrder(
		opCtx,
		order.ID,
		providerOrder.TradeNo,
		providerOrder.PaymentURL,
		providerOrder.PaymentMethod,
	)
	if err != nil {
		// The provider order exists but could not be persisted.  Never create a
		// replacement; hold the durable reservation for operator reconciliation.
		_, _ = s.repo.SetOrderState(opCtx, order.ID, NativeCheckoutStatusManualReview, "provider_order_persist_failed", time.Now())
		return nil, ErrNativeCheckoutUnavailable.WithCause(err)
	}
	return updated, nil
}

func isSupportedNativeCheckoutPaymentMethod(method string) bool {
	return method == NativeCheckoutPaymentMethodWeChat || method == NativeCheckoutPaymentMethodAlipay
}

func (s *NativeCheckoutService) GetOrder(ctx context.Context, userID int64, orderNo string) (*NativeCheckoutOrder, error) {
	// Customer-facing reads never call the payment provider. This makes status
	// polling a cheap database read and leaves upstream reconciliation under the
	// server worker's database lease.
	return s.repo.GetOrderForUser(ctx, strings.TrimSpace(orderNo), userID)
}

func (s *NativeCheckoutService) FetchDirectPaymentQR(ctx context.Context, userID int64, orderNo string) ([]byte, string, error) {
	order, err := s.repo.GetOrderForUser(ctx, strings.TrimSpace(orderNo), userID)
	if err != nil {
		return nil, "", err
	}
	if order.Status != NativeCheckoutStatusPending || strings.TrimSpace(order.PaymentURL) == "" {
		return nil, "", infraerrors.NotFound("NATIVE_CHECKOUT_QR_NOT_AVAILABLE", "direct payment QR is not available")
	}
	provider, err := s.providers.ProviderFor(order.Provider)
	if err != nil {
		return nil, "", infraerrors.NotFound("NATIVE_CHECKOUT_QR_NOT_AVAILABLE", "direct payment QR is not available")
	}
	body, contentType, err := provider.FetchDirectPaymentQR(ctx, order.PaymentURL)
	if err != nil {
		return nil, "", infraerrors.NotFound("NATIVE_CHECKOUT_QR_NOT_AVAILABLE", "direct payment QR is not available")
	}
	return body, contentType, nil
}

func (s *NativeCheckoutService) syncOrder(ctx context.Context, order *NativeCheckoutOrder) (*NativeCheckoutOrder, error) {
	if order.Provider == NativeCheckoutProviderAffiliateWallet {
		return s.syncCommissionWalletOrder(ctx, order)
	}
	if order.Status == NativeCheckoutStatusCreating {
		if order.UpdatedAt.IsZero() || time.Since(order.UpdatedAt) < s.staleAfter {
			return order, nil
		}
		// A server crash can occur while the provider request is in flight. Since
		// LDXP exposes no merchant idempotency key or safe lookup by our order ID,
		// never retry this ambiguous create automatically.
		return s.holdForReview(ctx, order, "provider_create_interrupted")
	}
	provider, err := s.providers.ProviderFor(order.Provider)
	if err != nil {
		return nil, fmt.Errorf("resolve native checkout provider: %w", err)
	}
	info, err := provider.GetOrderInfo(ctx, order.ProviderTradeNo)
	if err != nil {
		_ = s.repo.RecordPendingCheck(ctx, order.ID, time.Now().Add(s.nextCheckDelay(order)))
		return order, nil
	}
	if err := s.validateProviderOrderIdentity(order, info); err != nil {
		return s.holdForReview(ctx, order, "provider_order_mismatch")
	}
	if order.Status == NativeCheckoutStatusPending && info.Paid {
		// Persist the provider's paid signal before delivery/redeem processing.
		// The UI can then remove the payable QR immediately and a restart resumes
		// from a durable "checking" state instead of asking the user to pay again.
		order, err = s.repo.SetOrderState(
			ctx,
			order.ID,
			NativeCheckoutStatusChecking,
			"",
			time.Now().Add(s.pollInterval),
		)
		if err != nil {
			return nil, fmt.Errorf("record native checkout payment: %w", err)
		}
	}
	if order.Provider == NativeCheckoutProviderEasyPay && info.Paid {
		// EasyPay collects money but delivers no card: mint the internal redeem
		// code from the order snapshot, then let the standard fulfillment chain
		// (validation, claim, link, redeem, complete) run unchanged below.
		mintedCode, mintErr := s.ensureMintedNativeRedeemCode(ctx, order)
		if mintErr != nil {
			return s.holdForReview(ctx, order, "redeem_mint_failed")
		}
		info.Delivered = true
		info.RedeemCodes = []string{mintedCode}
	}
	if err := s.validateProviderOrder(order, info); err != nil {
		if errors.Is(err, errNativeCheckoutDeliveryPending) {
			_ = s.repo.RecordPendingCheck(ctx, order.ID, time.Now().Add(s.nextCheckDelay(order)))
			return order, nil
		}
		return s.holdForReview(ctx, order, "provider_order_mismatch")
	}

	return s.fulfillNativeCheckoutCode(ctx, order, info.RedeemCodes[0])
}

func (s *NativeCheckoutService) syncCommissionWalletOrder(ctx context.Context, order *NativeCheckoutOrder) (*NativeCheckoutOrder, error) {
	if order.Status == NativeCheckoutStatusCompleted {
		return order, nil
	}
	if order.Status == NativeCheckoutStatusFailed || order.Status == NativeCheckoutStatusManualReview {
		return order, ErrNativeCheckoutUnavailable
	}
	_, err := s.affiliateWallet.Purchase(
		ctx, order.UserID, order.PayAmountCNYFen*10_000, order.UserID,
		AffiliateCommissionPurchaseKindMonthlyCard, order.OfferCode, order.OrderNo,
		fmt.Sprintf("%s paid from affiliate commission wallet", order.OfferCode),
		"commission-wallet-order:"+order.OrderNo,
	)
	if err != nil {
		_, _ = s.repo.SetOrderState(ctx, order.ID, NativeCheckoutStatusFailed, "commission_wallet_debit_failed", time.Now())
		return nil, err
	}
	if order.Status == NativeCheckoutStatusCreating {
		order, err = s.repo.SetCommissionWalletOrderReady(ctx, order.ID, order.OrderNo)
		if err != nil {
			return nil, err
		}
	}
	mintedCode, err := s.ensureMintedNativeRedeemCode(ctx, order)
	if err != nil {
		return s.holdForReview(ctx, order, "redeem_mint_failed")
	}
	return s.fulfillNativeCheckoutCode(withAffiliateCommissionPurchaseAuthorization(ctx), order, mintedCode)
}

func (s *NativeCheckoutService) fulfillNativeCheckoutCode(ctx context.Context, order *NativeCheckoutOrder, codeValue string) (*NativeCheckoutOrder, error) {
	redeemCode, err := s.redeem.GetByCode(ctx, codeValue)
	if err != nil {
		return s.holdForReview(ctx, order, "redeem_code_not_found")
	}
	if err := validateNativeRedeemCode(order, redeemCode); err != nil {
		return s.holdForReview(ctx, order, "redeem_code_mismatch")
	}
	claimed, didClaim, err := s.repo.ClaimFulfillment(ctx, order.ID, redeemCode.ID, time.Now().Add(-s.staleAfter))
	if err != nil {
		return s.holdForReview(ctx, order, "redeem_inventory_claim_failed")
	}
	if !didClaim {
		return claimed, nil
	}
	order = claimed
	if err := s.repo.LinkRedeemCode(ctx, redeemCode.ID, order.ProviderTradeNo); err != nil {
		return s.holdForReview(ctx, order, "redeem_code_link_failed")
	}

	if redeemCode.Status == StatusUnused {
		if _, err := s.redeem.Redeem(withNativeCheckoutRedeemAuthorization(ctx), order.UserID, redeemCode.Code); err != nil {
			// A racing retry may have redeemed the code after our initial read.
			fresh, lookupErr := s.redeem.GetByID(ctx, redeemCode.ID)
			if lookupErr != nil || fresh.Status != StatusUsed || fresh.UsedBy == nil || *fresh.UsedBy != order.UserID {
				return s.holdForReview(ctx, order, "redeem_failed")
			}
		}
	} else if redeemCode.UsedBy == nil || *redeemCode.UsedBy != order.UserID {
		return s.holdForReview(ctx, order, "redeem_code_already_used")
	}

	completed, err := s.repo.CompleteOrder(ctx, order.ID)
	if err != nil {
		return nil, fmt.Errorf("complete native checkout order: %w", err)
	}
	return completed, nil
}

// nextCheckDelay keeps newly created orders responsive without polling an
// unpaid, abandoned QR every three seconds forever. Paid orders stay on the
// fast path until their code is delivered or minted; unpaid orders cool down
// with age.
func (s *NativeCheckoutService) nextCheckDelay(order *NativeCheckoutOrder) time.Duration {
	if order == nil || order.Status == NativeCheckoutStatusChecking {
		return s.pollInterval
	}
	if order.CreatedAt.IsZero() {
		return s.pollInterval
	}
	age := time.Since(order.CreatedAt)
	switch {
	case age < 2*time.Minute:
		return s.pollInterval
	case age < 10*time.Minute:
		return 10 * time.Second
	case age < time.Hour:
		return 30 * time.Second
	case age < 24*time.Hour:
		return 5 * time.Minute
	default:
		return time.Hour
	}
}

func (s *NativeCheckoutService) holdForReview(ctx context.Context, order *NativeCheckoutOrder, code string) (*NativeCheckoutOrder, error) {
	updated, err := s.repo.SetOrderState(ctx, order.ID, NativeCheckoutStatusManualReview, code, time.Now())
	if err != nil {
		return nil, fmt.Errorf("hold native checkout order for review: %w", err)
	}
	return updated, nil
}

var errNativeCheckoutDeliveryPending = errors.New("native checkout delivery pending")

func (s *NativeCheckoutService) validateProviderOrder(order *NativeCheckoutOrder, info *NativeCheckoutProviderOrderInfo) error {
	if err := s.validateProviderOrderIdentity(order, info); err != nil {
		return err
	}
	if !info.Paid || !info.Delivered || len(info.RedeemCodes) == 0 {
		return errNativeCheckoutDeliveryPending
	}
	if len(info.RedeemCodes) != 1 {
		return errors.New("unexpected redeem code count")
	}
	return nil
}

func (s *NativeCheckoutService) validateProviderOrderIdentity(order *NativeCheckoutOrder, info *NativeCheckoutProviderOrderInfo) error {
	if order == nil || info == nil {
		return errors.New("missing provider order")
	}
	if order.Provider == NativeCheckoutProviderEasyPay {
		// EasyPay queries key on our NC- order number, so the echoed trade
		// reference plus the paid amount are the only identity signals the
		// gateway returns. A missing amount is tolerated here; the paid notify
		// and the order snapshot still bound the minted entitlement.
		if info.TradeNo != order.ProviderTradeNo {
			return errors.New("provider order identity mismatch")
		}
		if info.TotalCNYFen != 0 && info.TotalCNYFen != order.PayAmountCNYFen {
			return errors.New("provider order amount mismatch")
		}
		return nil
	}
	if info.TradeNo != order.ProviderTradeNo || info.GoodsKey != order.ProviderGoodsKey || info.Quantity != 1 || info.TotalCNYFen != order.PayAmountCNYFen {
		return errors.New("provider order identity mismatch")
	}
	contact, err := normalizedCheckoutEmail(info.Contact)
	if err != nil || !secureStringEqual(s.hashContact(contact), order.ContactHash) {
		return errors.New("provider contact mismatch")
	}
	return nil
}

// ensureMintedNativeRedeemCode returns the redeem code belonging to a paid
// provider-collected order, minting it on first use. The mint is idempotent:
// the lookup by external_order_no finds a code committed by an earlier
// attempt, and a concurrent-mint unique violation re-reads the winner. The
// worker's SKIP LOCKED lease already serializes processors per order, and the
// partial unique index on redeem_codes.external_order_no (migration 195) is
// the database-level backstop for the lookup-then-insert window.
func (s *NativeCheckoutService) ensureMintedNativeRedeemCode(ctx context.Context, order *NativeCheckoutOrder) (string, error) {
	if existing, err := s.repo.FindMintedRedeemCode(ctx, order.ProviderTradeNo); err == nil {
		return existing, nil
	} else if !errors.Is(err, ErrRedeemCodeNotFound) {
		return "", fmt.Errorf("look up minted native checkout code: %w", err)
	}
	// 32-char lowercase hex, the same format as card-shop stock, so the
	// existing redeem chain and card-format parsing treat it identically.
	code, err := GenerateRedeemCode()
	if err != nil {
		return "", fmt.Errorf("generate native checkout redeem code: %w", err)
	}
	minted, err := s.repo.MintRedeemCode(ctx, order, code)
	if err != nil {
		return "", fmt.Errorf("mint native checkout redeem code: %w", err)
	}
	return minted, nil
}

// HandleEasyPayNotify processes a verified EasyPay asynchronous notify for an
// NC- order (signature and params were already verified by the gateway
// handler). Returning nil answers "success"; any error answers "fail" so the
// platform retries. The notify never fulfills directly: it validates and
// nudges, and the reconcile worker performs the authoritative provider query
// plus fulfillment (defense in depth against forged or partial signals).
func (s *NativeCheckoutService) HandleEasyPayNotify(ctx context.Context, n *payment.NotifyResult) error {
	orderNo := strings.TrimSpace(n.OutTradeNo)
	if orderNo == "" {
		return infraerrors.BadRequest("EASYPAY_INVALID_NOTIFY", "out_trade_no is required")
	}
	order, err := s.repo.GetOrder(ctx, orderNo)
	if err != nil {
		return fmt.Errorf("get native checkout order for easypay notify: %w", err)
	}
	if order.Provider != NativeCheckoutProviderEasyPay {
		return ErrNativeCheckoutProviderMismatch
	}
	if !n.Paid {
		// A non-success status only confirms receipt; the worker's polling
		// remains the source of truth.
		return nil
	}
	if int64(n.AmountCNYFen) != order.PayAmountCNYFen {
		return ErrNativeCheckoutAmountMismatch
	}
	return s.repo.NudgeReconcileNow(ctx, order.ID)
}

func validateNativeRedeemCode(order *NativeCheckoutOrder, code *RedeemCode) error {
	if order == nil || code == nil {
		return errors.New("missing redeem code")
	}
	orderGroupIDs := order.RedeemGroupIDs
	codeGroupIDs := subscriptionRedeemGroupIDs(code)
	if order.RedeemType == RedeemTypeSubscription && code.Type == RedeemTypeSubscription {
		var err error
		orderGroupIDs, _, err = completeCurrentMonthlyCardGroupIDs(orderGroupIDs)
		if err != nil {
			return errors.New("redeem code entitlement mismatch")
		}
		codeGroupIDs, _, err = completeCurrentMonthlyCardGroupIDs(codeGroupIDs)
		if err != nil {
			return errors.New("redeem code entitlement mismatch")
		}
	}
	if code.Type != order.RedeemType || !floatNearlyEqual(code.Value, order.RedeemValue) ||
		!floatNearlyEqual(code.PaidValue, order.RedeemPaidValue) || code.Purpose != order.RedeemPurpose ||
		code.SalesStatus != order.RedeemSalesStatus || code.ValidityDays != order.RedeemValidityDays ||
		!sameInt64Set(codeGroupIDs, orderGroupIDs) {
		return errors.New("redeem code entitlement mismatch")
	}
	if code.Status != StatusUnused && code.Status != StatusUsed {
		return errors.New("redeem code status mismatch")
	}
	if code.ExternalOrderNo != "" && code.ExternalOrderNo != order.ProviderTradeNo {
		return errors.New("redeem code external order mismatch")
	}
	return nil
}

func normalizedCheckoutEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(value)
	if err != nil || strings.ToLower(strings.TrimSpace(parsed.Address)) != value || len(value) > 254 {
		return "", errors.New("invalid email")
	}
	return value, nil
}

func (s *NativeCheckoutService) hashContact(value string) string {
	return hashNativeCheckoutContact(s.contactKey, value)
}

func deriveNativeCheckoutContactKey(secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte("laoshirenai/native-checkout/contact/v1"))
	return mac.Sum(nil)
}

func hashNativeCheckoutContact(key []byte, value string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func secureStringEqual(a, b string) bool {
	return hmac.Equal([]byte(a), []byte(b))
}

func floatNearlyEqual(a, b float64) bool { return math.Abs(a-b) <= 0.00000001 }

func sameInt64Set(a, b []int64) bool {
	a = append([]int64(nil), a...)
	b = append([]int64(nil), b...)
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *NativeCheckoutService) Start() {
	s.workerMu.Lock()
	defer s.workerMu.Unlock()
	if s.stopCh != nil {
		return
	}
	s.stopCh = make(chan struct{})
	s.doneCh = make(chan struct{})
	go s.runWorker(s.stopCh, s.doneCh)
}

func (s *NativeCheckoutService) Stop() {
	s.workerMu.Lock()
	if s.stopCh == nil {
		s.workerMu.Unlock()
		return
	}
	stopCh, doneCh := s.stopCh, s.doneCh
	s.stopCh, s.doneCh = nil, nil
	close(stopCh)
	s.workerMu.Unlock()
	<-doneCh
}

func (s *NativeCheckoutService) runWorker(stop <-chan struct{}, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(nativeCheckoutWorkerInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			s.reconcileBatch()
		}
	}
}

func (s *NativeCheckoutService) reconcileBatch() {
	claimCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	orders, err := s.repo.ClaimReconcileOrders(
		claimCtx,
		nativeCheckoutWorkerConcurrency,
		time.Now().Add(-s.staleAfter),
		time.Now().Add(nativeCheckoutWorkerLease),
	)
	cancel()
	if err != nil {
		logger.LegacyPrintf("service.native-checkout", "reconcile list failed: %v", err)
		return
	}

	// The database lease above ensures that browser tabs, other goroutines, and
	// additional application replicas cannot process the same order at once.
	// A small fixed concurrency keeps unrelated customers independent without
	// turning simultaneous checkouts into an unbounded burst against LDXP.
	var wg sync.WaitGroup
	for i := range orders {
		order := orders[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, syncCancel := context.WithTimeout(context.Background(), nativeCheckoutOrderSyncTimeout)
			defer syncCancel()
			if _, syncErr := s.syncOrder(ctx, &order); syncErr != nil {
				// Order number is an internal random identifier. Never log provider
				// trade numbers, payment URLs, contacts, or redeem codes.
				logger.LegacyPrintf("service.native-checkout", "reconcile failed order=%s: %v", order.OrderNo, syncErr)
			}
		}()
	}
	wg.Wait()
}

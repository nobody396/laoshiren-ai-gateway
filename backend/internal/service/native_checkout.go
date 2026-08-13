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
	"sort"
	"strings"
	"sync"
	"time"

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

	NativeCheckoutPaymentMethodWeChat = "wechat"
	NativeCheckoutPaymentMethodAlipay = "alipay"
)

var (
	ErrNativeCheckoutOfferNotFound  = infraerrors.NotFound("NATIVE_CHECKOUT_OFFER_NOT_FOUND", "checkout offer not found")
	ErrNativeCheckoutOrderNotFound  = infraerrors.NotFound("NATIVE_CHECKOUT_ORDER_NOT_FOUND", "checkout order not found")
	ErrNativeCheckoutAlreadyClaimed = infraerrors.Conflict("NATIVE_CHECKOUT_ALREADY_CLAIMED", "checkout offer was already claimed")
	ErrNativeCheckoutUnavailable    = infraerrors.ServiceUnavailable("NATIVE_CHECKOUT_UNAVAILABLE", "checkout is temporarily unavailable")
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
	Name                string
	Description         string
	ProductKind         string
	PayAmountCNYFen     int64
	BenefitAmountCNYFen int64
	OncePerUser         bool
	Claimed             bool
	Order               *NativeCheckoutOrder
}

type NativeCheckoutRepository interface {
	ListEnabledOffers(ctx context.Context) ([]NativeCheckoutOffer, error)
	GetEnabledOffer(ctx context.Context, code string) (*NativeCheckoutOffer, error)
	GetLatestOrderForOffer(ctx context.Context, userID int64, offerCode string) (*NativeCheckoutOrder, error)
	HasRedeemedOffer(ctx context.Context, userID int64, offerCode string) (bool, error)
	ReserveOrder(ctx context.Context, order *NativeCheckoutOrder) (*NativeCheckoutOrder, bool, error)
	ResetFailedOrder(ctx context.Context, id int64, contactHash string) (*NativeCheckoutOrder, bool, error)
	SetProviderOrder(ctx context.Context, id int64, providerTradeNo, paymentURL, paymentMethod string) (*NativeCheckoutOrder, error)
	SetOrderState(ctx context.Context, id int64, status, failureCode string, nextCheckAt time.Time) (*NativeCheckoutOrder, error)
	GetOrderForUser(ctx context.Context, orderNo string, userID int64) (*NativeCheckoutOrder, error)
	GetOrder(ctx context.Context, orderNo string) (*NativeCheckoutOrder, error)
	ListReconcileOrders(ctx context.Context, limit int, fulfillingStaleBefore time.Time) ([]NativeCheckoutOrder, error)
	RecordPendingCheck(ctx context.Context, id int64, nextCheckAt time.Time) error
	ClaimFulfillment(ctx context.Context, id, redeemCodeID int64, staleBefore time.Time) (*NativeCheckoutOrder, bool, error)
	LinkRedeemCode(ctx context.Context, redeemCodeID int64, providerTradeNo string) error
	CompleteOrder(ctx context.Context, id int64) (*NativeCheckoutOrder, error)
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
	CreateOrder(ctx context.Context, goodsKey, contact string, expectedAmountCNYFen int64) (*NativeCheckoutProviderOrder, error)
	IsPaid(ctx context.Context, tradeNo string) (bool, error)
	GetOrderInfo(ctx context.Context, tradeNo string) (*NativeCheckoutProviderOrderInfo, error)
	FetchDirectPaymentQR(ctx context.Context, paymentURL string) ([]byte, string, error)
}

type NativeCheckoutRedeemer interface {
	GetByCode(ctx context.Context, code string) (*RedeemCode, error)
	GetByID(ctx context.Context, id int64) (*RedeemCode, error)
	Redeem(ctx context.Context, userID int64, code string) (*RedeemCode, error)
}

type NativeCheckoutService struct {
	repo         NativeCheckoutRepository
	provider     NativeCheckoutProvider
	userRepo     UserRepository
	redeem       NativeCheckoutRedeemer
	contactKey   []byte
	pollInterval time.Duration
	staleAfter   time.Duration

	workerMu sync.Mutex
	stopCh   chan struct{}
	doneCh   chan struct{}
}

type nativeCheckoutRedeemAuthorizationKey struct{}

func withNativeCheckoutRedeemAuthorization(ctx context.Context) context.Context {
	return context.WithValue(ctx, nativeCheckoutRedeemAuthorizationKey{}, true)
}

func nativeCheckoutRedeemAuthorized(ctx context.Context) bool {
	allowed, _ := ctx.Value(nativeCheckoutRedeemAuthorizationKey{}).(bool)
	return allowed
}

func NewNativeCheckoutService(
	repo NativeCheckoutRepository,
	provider NativeCheckoutProvider,
	userRepo UserRepository,
	redeem NativeCheckoutRedeemer,
	contactHashKey string,
) *NativeCheckoutService {
	return &NativeCheckoutService{
		repo:         repo,
		provider:     provider,
		userRepo:     userRepo,
		redeem:       redeem,
		contactKey:   deriveNativeCheckoutContactKey(contactHashKey),
		pollInterval: 3 * time.Second,
		staleAfter:   time.Minute,
	}
}

func (s *NativeCheckoutService) ListOffers(ctx context.Context, userID int64) ([]NativeCheckoutOfferView, error) {
	offers, err := s.repo.ListEnabledOffers(ctx)
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
		claimed := order != nil && order.Status == NativeCheckoutStatusCompleted
		if !claimed && offer.OncePerUser {
			claimed, orderErr = s.repo.HasRedeemedOffer(ctx, userID, offer.Code)
			if orderErr != nil {
				return nil, fmt.Errorf("check native checkout entitlement: %w", orderErr)
			}
		}
		views = append(views, NativeCheckoutOfferView{
			Code:                offer.Code,
			Name:                offer.Name,
			Description:         offer.Description,
			ProductKind:         offer.ProductKind,
			PayAmountCNYFen:     offer.PayAmountCNYFen,
			BenefitAmountCNYFen: offer.BenefitAmountCNYFen,
			OncePerUser:         offer.OncePerUser,
			Claimed:             claimed,
			Order:               order,
		})
	}
	return views, nil
}

func (s *NativeCheckoutService) CreateOrder(ctx context.Context, userID int64, offerCode string) (*NativeCheckoutOrder, error) {
	offerCode = strings.TrimSpace(offerCode)
	if offerCode == "" {
		return nil, infraerrors.BadRequest("NATIVE_CHECKOUT_OFFER_REQUIRED", "checkout offer is required")
	}
	offer, err := s.repo.GetEnabledOffer(ctx, offerCode)
	if err != nil {
		if errors.Is(err, ErrNativeCheckoutOfferNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("get native checkout offer: %w", err)
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
		return s.createProviderOrder(ctx, existing, contact)
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
	return s.createProviderOrder(ctx, reserved, contact)
}

func (s *NativeCheckoutService) createProviderOrder(ctx context.Context, order *NativeCheckoutOrder, contact string) (*NativeCheckoutOrder, error) {
	// Once the durable reservation exists, finish the provider call and record
	// its outcome even if the browser disconnects. Otherwise a cancelled HTTP
	// request can strand a once-only order forever in "creating".
	opCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	providerOrder, err := s.provider.CreateOrder(opCtx, order.ProviderGoodsKey, contact, order.PayAmountCNYFen)
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

func (s *NativeCheckoutService) GetOrder(ctx context.Context, userID int64, orderNo string, sync bool) (*NativeCheckoutOrder, error) {
	order, err := s.repo.GetOrderForUser(ctx, strings.TrimSpace(orderNo), userID)
	if err != nil {
		return nil, err
	}
	if sync && (order.Status == NativeCheckoutStatusCreating || order.Status == NativeCheckoutStatusPending || order.Status == NativeCheckoutStatusChecking || order.Status == NativeCheckoutStatusFulfilling) {
		return s.syncOrder(ctx, order)
	}
	return order, nil
}

func (s *NativeCheckoutService) FetchDirectPaymentQR(ctx context.Context, userID int64, orderNo string) ([]byte, string, error) {
	order, err := s.repo.GetOrderForUser(ctx, strings.TrimSpace(orderNo), userID)
	if err != nil {
		return nil, "", err
	}
	if order.Status != NativeCheckoutStatusPending || strings.TrimSpace(order.PaymentURL) == "" {
		return nil, "", infraerrors.NotFound("NATIVE_CHECKOUT_QR_NOT_AVAILABLE", "direct payment QR is not available")
	}
	body, contentType, err := s.provider.FetchDirectPaymentQR(ctx, order.PaymentURL)
	if err != nil {
		return nil, "", infraerrors.NotFound("NATIVE_CHECKOUT_QR_NOT_AVAILABLE", "direct payment QR is not available")
	}
	return body, contentType, nil
}

func (s *NativeCheckoutService) syncOrder(ctx context.Context, order *NativeCheckoutOrder) (*NativeCheckoutOrder, error) {
	if order.Status == NativeCheckoutStatusCreating {
		if order.UpdatedAt.IsZero() || time.Since(order.UpdatedAt) < s.staleAfter {
			return order, nil
		}
		// A server crash can occur while the provider request is in flight. Since
		// LDXP exposes no merchant idempotency key or safe lookup by our order ID,
		// never retry this ambiguous create automatically.
		return s.holdForReview(ctx, order, "provider_create_interrupted")
	}
	info, err := s.provider.GetOrderInfo(ctx, order.ProviderTradeNo)
	if err != nil {
		_ = s.repo.RecordPendingCheck(ctx, order.ID, time.Now().Add(s.pollInterval))
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
	if err := s.validateProviderOrder(order, info); err != nil {
		if errors.Is(err, errNativeCheckoutDeliveryPending) {
			_ = s.repo.RecordPendingCheck(ctx, order.ID, time.Now().Add(s.pollInterval))
			return order, nil
		}
		return s.holdForReview(ctx, order, "provider_order_mismatch")
	}

	redeemCode, err := s.redeem.GetByCode(ctx, info.RedeemCodes[0])
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
	if info.TradeNo != order.ProviderTradeNo || info.GoodsKey != order.ProviderGoodsKey || info.Quantity != 1 || info.TotalCNYFen != order.PayAmountCNYFen {
		return errors.New("provider order identity mismatch")
	}
	contact, err := normalizedCheckoutEmail(info.Contact)
	if err != nil || !secureStringEqual(s.hashContact(contact), order.ContactHash) {
		return errors.New("provider contact mismatch")
	}
	return nil
}

func validateNativeRedeemCode(order *NativeCheckoutOrder, code *RedeemCode) error {
	if order == nil || code == nil {
		return errors.New("missing redeem code")
	}
	if code.Type != order.RedeemType || !floatNearlyEqual(code.Value, order.RedeemValue) ||
		!floatNearlyEqual(code.PaidValue, order.RedeemPaidValue) || code.Purpose != order.RedeemPurpose ||
		code.SalesStatus != order.RedeemSalesStatus || code.ValidityDays != order.RedeemValidityDays ||
		!sameInt64Set(code.GroupIDs, order.RedeemGroupIDs) {
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
	ticker := time.NewTicker(15 * time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	orders, err := s.repo.ListReconcileOrders(ctx, 50, time.Now().Add(-s.staleAfter))
	if err != nil {
		logger.LegacyPrintf("service.native-checkout", "reconcile list failed: %v", err)
		return
	}
	for i := range orders {
		if ctx.Err() != nil {
			return
		}
		if _, err := s.syncOrder(ctx, &orders[i]); err != nil {
			// Order number is an internal random identifier. Never log provider
			// trade numbers, payment URLs, contacts, or redeem codes.
			logger.LegacyPrintf("service.native-checkout", "reconcile failed order=%s: %v", orders[i].OrderNo, err)
		}
	}
}

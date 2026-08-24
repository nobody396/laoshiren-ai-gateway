//go:build unit

package service

import (
	"context"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/payment"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

type topupOrderRepoFake struct {
	mu            sync.Mutex
	orders        map[string]*TopupOrder
	byID          map[int64]*TopupOrder
	nextID        int64
	qrUpdates     map[int64][]string
	completeCalls int
}

func newTopupOrderRepoFake() *topupOrderRepoFake {
	return &topupOrderRepoFake{
		orders:    map[string]*TopupOrder{},
		byID:      map[int64]*TopupOrder{},
		qrUpdates: map[int64][]string{},
	}
}

func (f *topupOrderRepoFake) Create(_ context.Context, order *TopupOrder) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	order.ID = f.nextID
	stored := *order
	f.orders[order.OrderNo] = &stored
	f.byID[stored.ID] = &stored
	return nil
}

func (f *topupOrderRepoFake) GetByOrderNo(_ context.Context, orderNo string) (*TopupOrder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored, ok := f.orders[orderNo]
	if !ok {
		return nil, ErrTopupNotFound
	}
	cp := *stored
	return &cp, nil
}

func (f *topupOrderRepoFake) UpdateStatus(_ context.Context, id int64, status string, tradeNo *string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := f.byID[id]
	if stored == nil {
		return ErrTopupNotFound
	}
	stored.Status = status
	if tradeNo != nil && *tradeNo != "" {
		stored.XunhuTradeNo = tradeNo
	}
	return nil
}

func (f *topupOrderRepoFake) UpdateQRCodeURL(_ context.Context, id int64, qrCodeURL string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored := f.byID[id]
	if stored == nil {
		return ErrTopupNotFound
	}
	stored.QRCodeURL = ptrString(qrCodeURL)
	f.qrUpdates[id] = append(f.qrUpdates[id], qrCodeURL)
	return nil
}

func (f *topupOrderRepoFake) CompleteIfUnsettled(_ context.Context, id int64, tradeNo *string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completeCalls++
	stored := f.byID[id]
	if stored == nil || (stored.Status != TopupStatusPending && stored.Status != TopupStatusExpired) {
		return false, nil
	}
	stored.Status = TopupStatusCompleted
	stored.XunhuTradeNo = tradeNo
	return true, nil
}

func (f *topupOrderRepoFake) storedOrder(orderNo string) *TopupOrder {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *f.orders[orderNo]
	return &cp
}

type topupUserRepoFake struct {
	*userRepoStub
	mu          sync.Mutex
	balanceAdds []float64
}

func (f *topupUserRepoFake) UpdateBalance(_ context.Context, _ int64, amount float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.balanceAdds = append(f.balanceAdds, amount)
	return nil
}

func (f *topupUserRepoFake) addCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.balanceAdds)
}

// topupSettingRepoStub 与 settingPublicRepoStub 类似，但 GetValue 可用（GetFrontendURL 需要）。
type topupSettingRepoStub struct {
	values map[string]string
}

func (s *topupSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *topupSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	return s.values[key], nil
}

func (s *topupSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *topupSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *topupSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *topupSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *topupSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type stubEasyPayProvider struct {
	mu           sync.Mutex
	createReqs   []*payment.CreateOrderRequest
	createResult *payment.CreateOrderResult
	createErr    error
	queryResult  *payment.QueryResult
	queryErr     error
	queryOutNos  []string
}

func (s *stubEasyPayProvider) Name() string { return payment.ProviderEasyPay }

func (s *stubEasyPayProvider) CreateOrder(_ context.Context, req *payment.CreateOrderRequest) (*payment.CreateOrderResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createReqs = append(s.createReqs, req)
	if s.createErr != nil {
		return nil, s.createErr
	}
	return s.createResult, nil
}

func (s *stubEasyPayProvider) VerifyNotify(url.Values) (*payment.NotifyResult, error) {
	panic("unexpected VerifyNotify call")
}

func (s *stubEasyPayProvider) QueryOrder(_ context.Context, outTradeNo string) (*payment.QueryResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queryOutNos = append(s.queryOutNos, outTradeNo)
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	return s.queryResult, nil
}

func (s *stubEasyPayProvider) createCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.createReqs)
}

// --- helpers ---

func newEasyPayTopupService(t *testing.T, repo *topupOrderRepoFake, userRepo *topupUserRepoFake, settings map[string]string, providers ...payment.Provider) *TopupService {
	t.Helper()
	settingSvc := NewSettingService(&topupSettingRepoStub{values: settings}, &config.Config{})
	registry := payment.NewRegistry(providers...)
	entClient := newPaymentServiceTestEntClient(t)
	return NewTopupService(repo, settingSvc, userRepo, nil, entClient, nil, nil, nil, nil, nil, nil, registry)
}

// seedTopupEntUser 在 sqlite ent 测试库里建行，供 completeOrder 的 total_recharged 更新使用。
func seedTopupEntUser(t *testing.T, svc *TopupService, ctx context.Context) int64 {
	t.Helper()
	user, err := svc.entClient.User.Create().
		SetEmail("topup-easypay@example.com").
		SetPasswordHash("x").
		Save(ctx)
	require.NoError(t, err)
	return user.ID
}

func seedPendingEasyPayOrder(repo *topupOrderRepoFake, orderNo string, userID int64, amountFen int, payType string) *TopupOrder {
	repo.nextID++
	stored := &TopupOrder{
		ID:           repo.nextID,
		OrderNo:      orderNo,
		UserID:       userID,
		AmountCNYFen: amountFen,
		PayType:      payType,
		Provider:     payment.ProviderEasyPay,
		Status:       TopupStatusPending,
	}
	repo.orders[orderNo] = stored
	repo.byID[stored.ID] = stored
	return stored
}

// --- CreateTopupOrder provider selection ---

func TestTopupService_CreateTopupOrder_EasyPayAlipay(t *testing.T) {
	repo := newTopupOrderRepoFake()
	provider := &stubEasyPayProvider{
		createResult: &payment.CreateOrderResult{QRContent: "qr://easypay-content", PayURL: "https://pay.example/cashier"},
	}
	svc := newEasyPayTopupService(t, repo, &topupUserRepoFake{}, map[string]string{
		SettingKeyTopupAlipayProvider: "easypay",
		SettingKeyFrontendURL:         "https://frontend.example/",
	}, provider)

	orderNo, qrCodeURL, err := svc.CreateTopupOrder(context.Background(), 42, 2000, "alipay", "203.0.113.9")
	require.NoError(t, err)
	require.NotEmpty(t, orderNo)
	require.Equal(t, "qr://easypay-content", qrCodeURL, "QR content preferred over cashier URL")

	require.Len(t, provider.createReqs, 1)
	req := provider.createReqs[0]
	require.Equal(t, orderNo, req.OutTradeNo)
	require.Equal(t, payment.MethodAlipay, req.Method)
	require.Equal(t, 2000, req.AmountCNYFen)
	require.Equal(t, "老实人AI API调用额度充值", req.Subject)
	require.Equal(t, "https://frontend.example/api/v1/pay/notify/easypay", req.NotifyURL)
	require.Equal(t, "https://frontend.example/dashboard", req.ReturnURL)
	require.Equal(t, "203.0.113.9", req.ClientIP)

	stored := repo.storedOrder(orderNo)
	require.Equal(t, payment.ProviderEasyPay, stored.Provider)
	require.Equal(t, "alipay", stored.PayType)
	require.Equal(t, TopupStatusPending, stored.Status)
	require.Equal(t, []string{"qr://easypay-content"}, repo.qrUpdates[stored.ID])
}

func TestTopupService_CreateTopupProductOrder_LocksPerCardPromotion(t *testing.T) {
	repo := newTopupOrderRepoFake()
	provider := &stubEasyPayProvider{
		createResult: &payment.CreateOrderResult{QRContent: "qr://quantity-promotion"},
	}
	svc := newEasyPayTopupService(t, repo, &topupUserRepoFake{}, map[string]string{
		SettingKeyTopupAlipayProvider: "easypay",
		SettingKeyFrontendURL:         "https://frontend.example",
	}, provider)

	orderNo, _, err := svc.CreateTopupProductOrder(
		context.Background(), 42, TopupPromotion500PaidFen, 3, "alipay", "203.0.113.9",
	)
	require.NoError(t, err)
	require.Len(t, provider.createReqs, 1)
	require.Equal(t, 150_000, provider.createReqs[0].AmountCNYFen)

	stored := repo.storedOrder(orderNo)
	require.Equal(t, 150_000, stored.AmountCNYFen)
	require.Equal(t, 15_000, stored.BonusAmountCNYFen)
	require.Equal(t, 165_000, StoredTopupCreditQuote(stored.AmountCNYFen, stored.BonusAmountCNYFen).CreditedAmountCNYFen)
}

func TestTopupService_CreateTopupOrder_EasyPayPrefersProviderQRImage(t *testing.T) {
	repo := newTopupOrderRepoFake()
	provider := &stubEasyPayProvider{createResult: &payment.CreateOrderResult{
		QRImageURL: "https://zpayz.cn/qrcode/order.jpg",
		QRContent:  "alipays://platformapi/startapp?appId=1",
		PayURL:     "https://zpayz.cn/pay/order",
	}}
	svc := newEasyPayTopupService(t, repo, &topupUserRepoFake{}, map[string]string{
		SettingKeyTopupAlipayProvider: "easypay",
		SettingKeyFrontendURL:         "https://frontend.example",
	}, provider)

	_, qrCodeURL, err := svc.CreateTopupOrder(context.Background(), 42, 2000, "alipay", "203.0.113.9")
	require.NoError(t, err)
	require.Equal(t, "https://zpayz.cn/qrcode/order.jpg", qrCodeURL)
}

func TestTopupService_CreateTopupOrder_EasyPayWechatFallsBackToPayURL(t *testing.T) {
	repo := newTopupOrderRepoFake()
	provider := &stubEasyPayProvider{
		createResult: &payment.CreateOrderResult{PayURL: "https://pay.example/wx-cashier"},
	}
	svc := newEasyPayTopupService(t, repo, &topupUserRepoFake{}, map[string]string{
		SettingKeyTopupWechatProvider: "easypay",
		SettingKeyFrontendURL:         "https://frontend.example",
	}, provider)

	orderNo, qrCodeURL, err := svc.CreateTopupOrder(context.Background(), 42, 2000, "wechat", "203.0.113.9")
	require.NoError(t, err)
	require.Equal(t, "https://pay.example/wx-cashier", qrCodeURL, "falls back to cashier URL when QR content is empty")

	require.Len(t, provider.createReqs, 1)
	require.Equal(t, payment.MethodWechat, provider.createReqs[0].Method)

	stored := repo.storedOrder(orderNo)
	require.Equal(t, payment.ProviderEasyPay, stored.Provider)
	require.Equal(t, "wechat", stored.PayType)
}

func TestTopupService_CreateTopupOrder_DefaultsToXunhuWithoutSetting(t *testing.T) {
	repo := newTopupOrderRepoFake()
	provider := &stubEasyPayProvider{
		createResult: &payment.CreateOrderResult{QRContent: "qr://unused"},
	}
	// 无任何 provider 设置：应走虎皮椒路径（配置缺失 → XUNHU_ALIPAY_DISABLED，不触网）
	svc := newEasyPayTopupService(t, repo, &topupUserRepoFake{}, map[string]string{}, provider)

	_, _, err := svc.CreateTopupOrder(context.Background(), 42, 2000, "alipay", "203.0.113.9")
	require.Error(t, err)
	require.Equal(t, "XUNHU_ALIPAY_DISABLED", infraerrors.Reason(err))
	require.Zero(t, provider.createCallCount(), "easypay gateway must not be consulted for xunhu orders")
}

func TestTopupService_CreateTopupOrder_EasyPayNotConfiguredSurfacesCodedError(t *testing.T) {
	repo := newTopupOrderRepoFake()
	provider := &stubEasyPayProvider{createErr: payment.ErrEasyPayNotConfigured}
	svc := newEasyPayTopupService(t, repo, &topupUserRepoFake{}, map[string]string{
		SettingKeyTopupAlipayProvider: "easypay",
		SettingKeyFrontendURL:         "https://frontend.example",
	}, provider)

	_, _, err := svc.CreateTopupOrder(context.Background(), 42, 2000, "alipay", "203.0.113.9")
	require.ErrorIs(t, err, payment.ErrEasyPayNotConfigured)
}

func TestTopupService_CreateTopupOrder_EasyPayMissingFrontendURLRejected(t *testing.T) {
	repo := newTopupOrderRepoFake()
	provider := &stubEasyPayProvider{
		createResult: &payment.CreateOrderResult{QRContent: "qr://unused"},
	}
	svc := newEasyPayTopupService(t, repo, &topupUserRepoFake{}, map[string]string{
		SettingKeyTopupAlipayProvider: "easypay",
	}, provider)

	_, _, err := svc.CreateTopupOrder(context.Background(), 42, 2000, "alipay", "203.0.113.9")
	require.Error(t, err)
	require.Equal(t, "EASYPAY_NOTIFY_URL_MISSING", infraerrors.Reason(err))
	require.Zero(t, provider.createCallCount())
}

// --- HandleEasyPayNotify ---

func TestTopupService_HandleEasyPayNotify_CreditsOnce(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	userRepo := &topupUserRepoFake{userRepoStub: &userRepoStub{}}
	svc := newEasyPayTopupService(t, repo, userRepo, map[string]string{}, &stubEasyPayProvider{})
	userID := seedTopupEntUser(t, svc, ctx)
	seedPendingEasyPayOrder(repo, "TP000421234567890123", userID, 5000, "alipay")

	notify := &payment.NotifyResult{
		OutTradeNo:   "TP000421234567890123",
		TradeNo:      "EP-TRADE-1",
		Method:       "alipay",
		AmountCNYFen: 5000,
		Paid:         true,
	}

	require.NoError(t, svc.HandleEasyPayNotify(ctx, notify))
	require.Equal(t, 1, userRepo.addCount())
	require.InDelta(t, 50.0, userRepo.balanceAdds[0], 0.000001, "5000 fen at 1:1 CNY->USD")
	stored := repo.storedOrder("TP000421234567890123")
	require.Equal(t, TopupStatusCompleted, stored.Status)
	require.NotNil(t, stored.XunhuTradeNo)
	require.Equal(t, "EP-TRADE-1", *stored.XunhuTradeNo, "easypay trade no shares the xunhu_trade_no column")

	// 重复回调必须幂等：不再入账
	require.NoError(t, svc.HandleEasyPayNotify(ctx, notify))
	require.Equal(t, 1, userRepo.addCount(), "duplicate notify must not credit twice")
}

func TestTopupService_HandleEasyPayNotify_LatePaidExpiredOrderStillCreditsOnce(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	userRepo := &topupUserRepoFake{userRepoStub: &userRepoStub{}}
	svc := newEasyPayTopupService(t, repo, userRepo, map[string]string{}, &stubEasyPayProvider{})
	userID := seedTopupEntUser(t, svc, ctx)
	order := seedPendingEasyPayOrder(repo, "TP000421234567890131", userID, 5000, "alipay")
	order.Status = TopupStatusExpired

	notify := &payment.NotifyResult{
		OutTradeNo: "TP000421234567890131", TradeNo: "EP-LATE-1",
		Method: "alipay", AmountCNYFen: 5000, Paid: true,
	}
	require.NoError(t, svc.HandleEasyPayNotify(ctx, notify))
	require.Equal(t, TopupStatusCompleted, repo.storedOrder(notify.OutTradeNo).Status)
	require.Equal(t, 1, userRepo.addCount())

	require.NoError(t, svc.HandleEasyPayNotify(ctx, notify))
	require.Equal(t, 1, userRepo.addCount(), "duplicate late notify must remain idempotent")
}

func TestTopupService_HandleEasyPayNotify_UnpaidAckOnly(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	userRepo := &topupUserRepoFake{userRepoStub: &userRepoStub{}}
	svc := newEasyPayTopupService(t, repo, userRepo, map[string]string{}, &stubEasyPayProvider{})
	seedPendingEasyPayOrder(repo, "TP000421234567890124", 7, 5000, "alipay")

	err := svc.HandleEasyPayNotify(ctx, &payment.NotifyResult{
		OutTradeNo:   "TP000421234567890124",
		AmountCNYFen: 5000,
		Paid:         false,
	})
	require.NoError(t, err, "unpaid notify must be acked so the platform stops retrying")
	require.Zero(t, userRepo.addCount())
	require.Equal(t, TopupStatusPending, repo.storedOrder("TP000421234567890124").Status)
}

func TestTopupService_HandleEasyPayNotify_AmountMismatchRejected(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	userRepo := &topupUserRepoFake{userRepoStub: &userRepoStub{}}
	svc := newEasyPayTopupService(t, repo, userRepo, map[string]string{}, &stubEasyPayProvider{})
	seedPendingEasyPayOrder(repo, "TP000421234567890125", 7, 5000, "alipay")

	err := svc.HandleEasyPayNotify(ctx, &payment.NotifyResult{
		OutTradeNo:   "TP000421234567890125",
		AmountCNYFen: 5001,
		Paid:         true,
	})
	require.ErrorIs(t, err, ErrTopupAmountMismatch)
	require.Zero(t, userRepo.addCount())
}

func TestTopupService_HandleEasyPayNotify_ProviderMismatchRejected(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	userRepo := &topupUserRepoFake{userRepoStub: &userRepoStub{}}
	svc := newEasyPayTopupService(t, repo, userRepo, map[string]string{}, &stubEasyPayProvider{})
	order := seedPendingEasyPayOrder(repo, "TP000421234567890126", 7, 5000, "alipay")
	order.Provider = "xunhu" // 虎皮椒订单被 easypay 回调命中必须拒绝

	err := svc.HandleEasyPayNotify(ctx, &payment.NotifyResult{
		OutTradeNo:   "TP000421234567890126",
		AmountCNYFen: 5000,
		Paid:         true,
	})
	require.ErrorIs(t, err, ErrTopupProviderMismatch)
	require.Zero(t, userRepo.addCount())
}

func TestTopupService_HandleEasyPayNotify_MethodMismatchRejected(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	userRepo := &topupUserRepoFake{userRepoStub: &userRepoStub{}}
	svc := newEasyPayTopupService(t, repo, userRepo, map[string]string{}, &stubEasyPayProvider{})
	seedPendingEasyPayOrder(repo, "TP000421234567890127", 7, 5000, "alipay")

	err := svc.HandleEasyPayNotify(ctx, &payment.NotifyResult{
		OutTradeNo:   "TP000421234567890127",
		AmountCNYFen: 5000,
		Method:       "wechat",
		Paid:         true,
	})
	require.ErrorIs(t, err, ErrTopupPayTypeMismatch)
	require.Zero(t, userRepo.addCount())
}

func TestTopupService_HandleEasyPayNotify_UnknownOrderFails(t *testing.T) {
	repo := newTopupOrderRepoFake()
	svc := newEasyPayTopupService(t, repo, &topupUserRepoFake{userRepoStub: &userRepoStub{}}, map[string]string{}, &stubEasyPayProvider{})

	err := svc.HandleEasyPayNotify(context.Background(), &payment.NotifyResult{
		OutTradeNo:   "TP999999999999999999",
		AmountCNYFen: 5000,
		Paid:         true,
	})
	require.Error(t, err, "unknown order must fail so the platform retries")
}

// --- QueryOrderStatus self-heal ---

func TestTopupService_QueryOrderStatus_EasyPaySelfHealCompletesPaidOrder(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	userRepo := &topupUserRepoFake{userRepoStub: &userRepoStub{}}
	provider := &stubEasyPayProvider{
		queryResult: &payment.QueryResult{Paid: true, TradeNo: "EP-Q-1", AmountCNYFen: 5000},
	}
	svc := newEasyPayTopupService(t, repo, userRepo, map[string]string{}, provider)
	userID := seedTopupEntUser(t, svc, ctx)
	seedPendingEasyPayOrder(repo, "TP000421234567890128", userID, 5000, "alipay")

	order, err := svc.QueryOrderStatus(ctx, "TP000421234567890128", userID)
	require.NoError(t, err)
	require.Equal(t, TopupStatusCompleted, order.Status)
	require.Equal(t, []string{"TP000421234567890128"}, provider.queryOutNos)
	require.Equal(t, 1, userRepo.addCount())
	stored := repo.storedOrder("TP000421234567890128")
	require.NotNil(t, stored.XunhuTradeNo)
	require.Equal(t, "EP-Q-1", *stored.XunhuTradeNo)
}

func TestTopupService_QueryOrderStatus_ExpiresUnpaidOrderAfterFiveMinutes(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	provider := &stubEasyPayProvider{queryResult: &payment.QueryResult{Paid: false}}
	svc := newEasyPayTopupService(t, repo, &topupUserRepoFake{userRepoStub: &userRepoStub{}}, map[string]string{}, provider)
	order := seedPendingEasyPayOrder(repo, "TP000421234567890132", 42, 5000, "alipay")
	order.CreatedAt = time.Now().Add(-TopupOrderTTL - time.Second)

	got, err := svc.QueryOrderStatus(ctx, order.OrderNo, order.UserID)
	require.NoError(t, err)
	require.Equal(t, TopupStatusExpired, got.Status)
	require.Equal(t, TopupStatusExpired, repo.storedOrder(order.OrderNo).Status)
	require.Equal(t, []string{order.OrderNo}, provider.queryOutNos, "provider is checked before local expiry")
}

func TestTopupService_QueryOrderStatus_SelfHealsLatePaidExpiredOrder(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	userRepo := &topupUserRepoFake{userRepoStub: &userRepoStub{}}
	provider := &stubEasyPayProvider{queryResult: &payment.QueryResult{Paid: true, TradeNo: "EP-LATE-Q", AmountCNYFen: 5000}}
	svc := newEasyPayTopupService(t, repo, userRepo, map[string]string{}, provider)
	userID := seedTopupEntUser(t, svc, ctx)
	order := seedPendingEasyPayOrder(repo, "TP000421234567890133", userID, 5000, "alipay")
	order.Status = TopupStatusExpired
	order.CreatedAt = time.Now().Add(-TopupOrderTTL - time.Minute)

	got, err := svc.QueryOrderStatus(ctx, order.OrderNo, userID)
	require.NoError(t, err)
	require.Equal(t, TopupStatusCompleted, got.Status)
	require.Equal(t, 1, userRepo.addCount())
}

func TestTopupService_QueryOrderStatus_EasyPaySelfHealQueryFailureKeepsPending(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	userRepo := &topupUserRepoFake{userRepoStub: &userRepoStub{}}
	provider := &stubEasyPayProvider{queryErr: payment.ErrEasyPayUpstream}
	svc := newEasyPayTopupService(t, repo, userRepo, map[string]string{}, provider)
	userID := seedTopupEntUser(t, svc, ctx)
	seedPendingEasyPayOrder(repo, "TP000421234567890129", userID, 5000, "alipay")

	order, err := svc.QueryOrderStatus(ctx, "TP000421234567890129", userID)
	require.NoError(t, err, "gateway query failure must not fail the user-facing status request")
	require.Equal(t, TopupStatusPending, order.Status)
	require.Zero(t, userRepo.addCount())
}

func TestTopupService_QueryOrderStatus_EasyPaySelfHealAmountMismatchNotCompleted(t *testing.T) {
	ctx := context.Background()
	repo := newTopupOrderRepoFake()
	userRepo := &topupUserRepoFake{userRepoStub: &userRepoStub{}}
	provider := &stubEasyPayProvider{
		queryResult: &payment.QueryResult{Paid: true, TradeNo: "EP-Q-2", AmountCNYFen: 9999},
	}
	svc := newEasyPayTopupService(t, repo, userRepo, map[string]string{}, provider)
	userID := seedTopupEntUser(t, svc, ctx)
	seedPendingEasyPayOrder(repo, "TP000421234567890130", userID, 5000, "alipay")

	order, err := svc.QueryOrderStatus(ctx, "TP000421234567890130", userID)
	require.NoError(t, err)
	require.Equal(t, TopupStatusPending, order.Status, "amount mismatch must block self-heal completion")
	require.Zero(t, userRepo.addCount())
}

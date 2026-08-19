//go:build unit

package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/payment"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type easyPayProviderStub struct {
	verifyResult *payment.NotifyResult
	verifyErr    error
	verifyCalls  int
	gotParams    url.Values
}

func (s *easyPayProviderStub) Name() string { return payment.ProviderEasyPay }

func (s *easyPayProviderStub) CreateOrder(context.Context, *payment.CreateOrderRequest) (*payment.CreateOrderResult, error) {
	panic("unexpected CreateOrder call")
}

func (s *easyPayProviderStub) VerifyNotify(params url.Values) (*payment.NotifyResult, error) {
	s.verifyCalls++
	s.gotParams = params
	if s.verifyErr != nil {
		return nil, s.verifyErr
	}
	return s.verifyResult, nil
}

func (s *easyPayProviderStub) QueryOrder(context.Context, string) (*payment.QueryResult, error) {
	panic("unexpected QueryOrder call")
}

type topupEasyPayNotifierStub struct {
	calls int
	got   *payment.NotifyResult
	err   error
}

func (s *topupEasyPayNotifierStub) HandleEasyPayNotify(_ context.Context, n *payment.NotifyResult) error {
	s.calls++
	s.got = n
	return s.err
}

type nativeEasyPayNotifierStub struct {
	calls int
	got   *payment.NotifyResult
	err   error
}

func (s *nativeEasyPayNotifierStub) HandleEasyPayNotify(_ context.Context, n *payment.NotifyResult) error {
	s.calls++
	s.got = n
	return s.err
}

func newEasyPayNotifyRouter(h *PaymentGatewayHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/pay/notify/easypay", h.HandleEasyPayNotify)
	r.GET("/pay/notify/easypay", h.HandleEasyPayNotify)
	return r
}

func paidNotify(outTradeNo string) *payment.NotifyResult {
	return &payment.NotifyResult{
		OutTradeNo:   outTradeNo,
		TradeNo:      "EP-1",
		Method:       "alipay",
		AmountCNYFen: 2000,
		Paid:         true,
	}
}

func TestPaymentGatewayHandler_EasyPayNotifyPostDispatchesTopup(t *testing.T) {
	provider := &easyPayProviderStub{verifyResult: paidNotify("TP000421234567890123")}
	topup := &topupEasyPayNotifierStub{}
	native := &nativeEasyPayNotifierStub{}
	h := NewPaymentGatewayHandler(payment.NewRegistry(provider), topup, native)
	r := newEasyPayNotifyRouter(h)

	body := strings.NewReader("out_trade_no=TP000421234567890123&trade_status=TRADE_SUCCESS&sign=abc")
	req := httptest.NewRequest(http.MethodPost, "/pay/notify/easypay", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "success", w.Body.String())
	require.Equal(t, 1, provider.verifyCalls)
	require.Equal(t, "TP000421234567890123", provider.gotParams.Get("out_trade_no"))
	require.Equal(t, 1, topup.calls)
	require.Equal(t, provider.verifyResult, topup.got)
	require.Zero(t, native.calls)
}

func TestPaymentGatewayHandler_EasyPayNotifyGetDispatchesTopup(t *testing.T) {
	provider := &easyPayProviderStub{verifyResult: paidNotify("TP000421234567890124")}
	topup := &topupEasyPayNotifierStub{}
	h := NewPaymentGatewayHandler(payment.NewRegistry(provider), topup, nil)
	r := newEasyPayNotifyRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/pay/notify/easypay?out_trade_no=TP000421234567890124&trade_status=TRADE_SUCCESS&sign=abc", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "success", w.Body.String())
	require.Equal(t, "TP000421234567890124", provider.gotParams.Get("out_trade_no"))
	require.Equal(t, 1, topup.calls)
}

func TestPaymentGatewayHandler_EasyPayNotifyBadSignAnswersFail(t *testing.T) {
	provider := &easyPayProviderStub{verifyErr: payment.ErrEasyPayInvalidSign}
	topup := &topupEasyPayNotifierStub{}
	h := NewPaymentGatewayHandler(payment.NewRegistry(provider), topup, nil)
	r := newEasyPayNotifyRouter(h)

	body := strings.NewReader("out_trade_no=TP000421234567890123&sign=wrong")
	req := httptest.NewRequest(http.MethodPost, "/pay/notify/easypay", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "fail", w.Body.String())
	require.Zero(t, topup.calls, "unverified notify must never reach the service")
}

func TestPaymentGatewayHandler_EasyPayNotifyUnknownPrefixAnswersFail(t *testing.T) {
	provider := &easyPayProviderStub{verifyResult: paidNotify("XX-UNKNOWN-1")}
	topup := &topupEasyPayNotifierStub{}
	native := &nativeEasyPayNotifierStub{}
	h := NewPaymentGatewayHandler(payment.NewRegistry(provider), topup, native)
	r := newEasyPayNotifyRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/pay/notify/easypay?out_trade_no=XX-UNKNOWN-1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "fail", w.Body.String())
	require.Zero(t, topup.calls)
	require.Zero(t, native.calls)
}

func TestPaymentGatewayHandler_EasyPayNotifyNilNativeAnswersFailForNativeOrder(t *testing.T) {
	provider := &easyPayProviderStub{verifyResult: paidNotify("NC-abcdef")}
	topup := &topupEasyPayNotifierStub{}
	h := NewPaymentGatewayHandler(payment.NewRegistry(provider), topup, nil)
	r := newEasyPayNotifyRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/pay/notify/easypay?out_trade_no=NC-abcdef", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "fail", w.Body.String(), "native checkout easypay 未接入时必须答 fail 让平台重试")
	require.Zero(t, topup.calls)
}

func TestPaymentGatewayHandler_EasyPayNotifyNativeOrderDispatchedToNative(t *testing.T) {
	provider := &easyPayProviderStub{verifyResult: paidNotify("NC-abcdef")}
	topup := &topupEasyPayNotifierStub{}
	native := &nativeEasyPayNotifierStub{}
	h := NewPaymentGatewayHandler(payment.NewRegistry(provider), topup, native)
	r := newEasyPayNotifyRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/pay/notify/easypay?out_trade_no=NC-abcdef", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "success", w.Body.String())
	require.Equal(t, 1, native.calls)
	require.Equal(t, provider.verifyResult, native.got)
	require.Zero(t, topup.calls)
}

func TestPaymentGatewayHandler_EasyPayNotifyNotifierErrorAnswersFail(t *testing.T) {
	provider := &easyPayProviderStub{verifyResult: paidNotify("TP000421234567890125")}
	topup := &topupEasyPayNotifierStub{err: errors.New("db down")}
	h := NewPaymentGatewayHandler(payment.NewRegistry(provider), topup, nil)
	r := newEasyPayNotifyRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/pay/notify/easypay?out_trade_no=TP000421234567890125", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Equal(t, "fail", w.Body.String())
	require.Equal(t, 1, topup.calls)
}

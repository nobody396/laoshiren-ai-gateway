//go:build unit

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/payment"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type easyPayGatewayStub struct {
	createResult *payment.CreateOrderResult
	createErr    error
	gotCreate    *payment.CreateOrderRequest
	queryResult  *payment.QueryResult
	queryErr     error
	gotQuery     []string
}

func (s *easyPayGatewayStub) CreateOrder(_ context.Context, req *payment.CreateOrderRequest) (*payment.CreateOrderResult, error) {
	s.gotCreate = req
	if s.createErr != nil {
		return nil, s.createErr
	}
	return s.createResult, nil
}

func (s *easyPayGatewayStub) QueryOrder(_ context.Context, outTradeNo string) (*payment.QueryResult, error) {
	s.gotQuery = append(s.gotQuery, outTradeNo)
	if s.queryErr != nil {
		return nil, s.queryErr
	}
	return s.queryResult, nil
}

type easyPayCheckoutSettingsStub struct {
	pid, key, apiBase string
	enabled           bool
	frontendURL       string
}

func (s easyPayCheckoutSettingsStub) GetEasyPayConfig(context.Context) (string, string, string, bool, error) {
	return s.pid, s.key, s.apiBase, s.enabled, nil
}

func (s easyPayCheckoutSettingsStub) GetFrontendURL(context.Context) string {
	return s.frontendURL
}

func newEasyPayCheckoutClientForTest(gateway easyPayGateway, settings easyPayCheckoutSettings) *EasyPayCheckoutClient {
	return &EasyPayCheckoutClient{gateway: gateway, settings: settings}
}

func easyPayCreateRequest(payType string) *service.NativeCheckoutCreateRequest {
	return &service.NativeCheckoutCreateRequest{
		OrderNo:              "NC-abc123",
		GoodsKey:             "newcomer-balance-5-to-10",
		Contact:              "buyer@example.com",
		ExpectedAmountCNYFen: 500,
		PayType:              payType,
		ClientIP:             "203.0.113.9",
	}
}

func TestEasyPayCheckoutClientCreateOrderMapsRequestAndResult(t *testing.T) {
	gateway := &easyPayGatewayStub{createResult: &payment.CreateOrderResult{
		TradeNo: "EP-INTERNAL-1", PayURL: "https://pay.example.com/cashier/EP-INTERNAL-1",
	}}
	settings := easyPayCheckoutSettingsStub{enabled: true, pid: "1001", key: "secret", frontendURL: "https://app.example.com/"}
	client := newEasyPayCheckoutClientForTest(gateway, settings)

	order, err := client.CreateOrder(context.Background(), easyPayCreateRequest(""))
	require.NoError(t, err)
	require.Equal(t, "NC-abc123", order.TradeNo, "the stored trade reference is our out_trade_no, never the gateway-internal one")
	require.Equal(t, "https://pay.example.com/cashier/EP-INTERNAL-1", order.PaymentURL)
	require.Equal(t, service.NativeCheckoutPaymentMethodAlipay, order.PaymentMethod)

	require.Equal(t, "NC-abc123", gateway.gotCreate.OutTradeNo)
	require.Equal(t, payment.MethodAlipay, gateway.gotCreate.Method)
	require.Equal(t, 500, gateway.gotCreate.AmountCNYFen)
	require.Equal(t, "https://app.example.com/api/v1/pay/notify/easypay", gateway.gotCreate.NotifyURL)
	require.Equal(t, "https://app.example.com/dashboard", gateway.gotCreate.ReturnURL)
	require.Equal(t, "203.0.113.9", gateway.gotCreate.ClientIP)
	require.NotEmpty(t, gateway.gotCreate.Subject)
}

func TestEasyPayCheckoutClientCreateOrderPrefersScannableQRContent(t *testing.T) {
	gateway := &easyPayGatewayStub{createResult: &payment.CreateOrderResult{
		QRImageURL: "https://zpayz.cn/qrcode/order.jpg",
		QRContent:  "alipays://platformapi/startapp?appId=1",
		PayURL:     "https://zpayz.cn/pay/order",
	}}
	settings := easyPayCheckoutSettingsStub{enabled: true, pid: "1001", key: "secret", frontendURL: "https://app.example.com"}
	client := newEasyPayCheckoutClientForTest(gateway, settings)

	order, err := client.CreateOrder(context.Background(), easyPayCreateRequest(""))
	require.NoError(t, err)
	require.Equal(t, "alipays://platformapi/startapp?appId=1", order.PaymentURL)
}

func TestEasyPayCheckoutClientCreateOrderWechatSelection(t *testing.T) {
	gateway := &easyPayGatewayStub{createResult: &payment.CreateOrderResult{PayURL: "https://pay.example.com/cashier/x"}}
	settings := easyPayCheckoutSettingsStub{enabled: true, pid: "1001", key: "secret", frontendURL: "https://app.example.com"}
	client := newEasyPayCheckoutClientForTest(gateway, settings)

	order, err := client.CreateOrder(context.Background(), easyPayCreateRequest(service.NativeCheckoutPaymentMethodWeChat))
	require.NoError(t, err)
	require.Equal(t, service.NativeCheckoutPaymentMethodWeChat, order.PaymentMethod)
	require.Equal(t, payment.MethodWechat, gateway.gotCreate.Method)
}

func TestEasyPayCheckoutClientCreateOrderFallsBackToQRContent(t *testing.T) {
	gateway := &easyPayGatewayStub{createResult: &payment.CreateOrderResult{QRContent: "weixin://wxpay/bizpayurl?pr=abc"}}
	settings := easyPayCheckoutSettingsStub{enabled: true, pid: "1001", key: "secret", frontendURL: "https://app.example.com"}
	client := newEasyPayCheckoutClientForTest(gateway, settings)

	order, err := client.CreateOrder(context.Background(), easyPayCreateRequest("wechat"))
	require.NoError(t, err)
	require.Equal(t, "weixin://wxpay/bizpayurl?pr=abc", order.PaymentURL)
}

func TestEasyPayCheckoutClientCreateOrderWithoutPaymentTargetIsAmbiguous(t *testing.T) {
	gateway := &easyPayGatewayStub{createResult: &payment.CreateOrderResult{}}
	settings := easyPayCheckoutSettingsStub{enabled: true, pid: "1001", key: "secret", frontendURL: "https://app.example.com"}
	client := newEasyPayCheckoutClientForTest(gateway, settings)

	_, err := client.CreateOrder(context.Background(), easyPayCreateRequest(""))
	var providerErr *service.NativeCheckoutProviderError
	require.ErrorAs(t, err, &providerErr)
	require.True(t, providerErr.Ambiguous, "an accepted create with no payment target may exist upstream and must not be retried")
}

func TestEasyPayCheckoutClientCreateOrderErrorAmbiguity(t *testing.T) {
	settings := easyPayCheckoutSettingsStub{enabled: true, pid: "1001", key: "secret", frontendURL: "https://app.example.com"}
	tests := []struct {
		name      string
		createErr error
		ambiguous bool
	}{
		{name: "transport", createErr: fmt.Errorf("%w: dial tcp: timeout", payment.ErrEasyPayUpstream), ambiguous: true},
		{name: "undecodable response", createErr: fmt.Errorf("%w: decode create response", payment.ErrEasyPayUpstream), ambiguous: true},
		{name: "decoded rejection", createErr: infraerrors.BadRequest("EASYPAY_API_ERROR", "easypay create order failed: 订单号重复"), ambiguous: false},
		{name: "not configured", createErr: payment.ErrEasyPayNotConfigured, ambiguous: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newEasyPayCheckoutClientForTest(&easyPayGatewayStub{createErr: tt.createErr}, settings)
			_, err := client.CreateOrder(context.Background(), easyPayCreateRequest(""))
			var providerErr *service.NativeCheckoutProviderError
			require.ErrorAs(t, err, &providerErr)
			require.Equal(t, tt.ambiguous, providerErr.Ambiguous)
			require.ErrorIs(t, err, tt.createErr)
		})
	}
}

func TestEasyPayCheckoutClientCreateOrderRequiresNotifyURL(t *testing.T) {
	gateway := &easyPayGatewayStub{createResult: &payment.CreateOrderResult{PayURL: "https://pay.example.com/cashier/x"}}
	settings := easyPayCheckoutSettingsStub{enabled: true, pid: "1001", key: "secret", frontendURL: "  "}
	client := newEasyPayCheckoutClientForTest(gateway, settings)

	_, err := client.CreateOrder(context.Background(), easyPayCreateRequest(""))
	var providerErr *service.NativeCheckoutProviderError
	require.ErrorAs(t, err, &providerErr)
	require.False(t, providerErr.Ambiguous, "a missing notify URL fails before the upstream call, so no order can exist")
	require.Equal(t, "EASYPAY_NOTIFY_URL_MISSING", infraerrors.Reason(err))
	require.Nil(t, gateway.gotCreate, "the gateway must not be called without a notify URL")
}

func TestEasyPayCheckoutClientValidateOffer(t *testing.T) {
	gateway := &easyPayGatewayStub{}

	disabled := newEasyPayCheckoutClientForTest(gateway, easyPayCheckoutSettingsStub{enabled: false, pid: "1001", key: "secret"})
	require.ErrorIs(t, disabled.ValidateOffer(context.Background(), "newcomer-balance-5-to-10", 500), payment.ErrEasyPayNotConfigured)

	missingKey := newEasyPayCheckoutClientForTest(gateway, easyPayCheckoutSettingsStub{enabled: true, pid: "1001"})
	require.ErrorIs(t, missingKey.ValidateOffer(context.Background(), "newcomer-balance-5-to-10", 500), payment.ErrEasyPayNotConfigured)

	ok := newEasyPayCheckoutClientForTest(gateway, easyPayCheckoutSettingsStub{enabled: true, pid: "1001", key: "secret"})
	require.NoError(t, ok.ValidateOffer(context.Background(), "newcomer-balance-5-to-10", 500))

	require.Error(t, ok.ValidateOffer(context.Background(), "  ", 500))
	require.Error(t, ok.ValidateOffer(context.Background(), "newcomer-balance-5-to-10", 0))
}

func TestEasyPayCheckoutClientGetOrderInfo(t *testing.T) {
	gateway := &easyPayGatewayStub{queryResult: &payment.QueryResult{Paid: true, TradeNo: "EP-INTERNAL-1", AmountCNYFen: 500}}
	client := newEasyPayCheckoutClientForTest(gateway, easyPayCheckoutSettingsStub{enabled: true, pid: "1001", key: "secret"})

	info, err := client.GetOrderInfo(context.Background(), "NC-abc123")
	require.NoError(t, err)
	require.True(t, info.Paid)
	require.Equal(t, int64(500), info.TotalCNYFen)
	require.Equal(t, "NC-abc123", info.TradeNo, "identity checks compare our persisted out_trade_no reference")
	require.False(t, info.Delivered, "easypay delivers money only; the service mints the code")
	require.Empty(t, info.RedeemCodes)
	require.Equal(t, []string{"NC-abc123"}, gateway.gotQuery)

	paid, err := client.IsPaid(context.Background(), "NC-abc123")
	require.NoError(t, err)
	require.True(t, paid)

	gateway.queryResult = &payment.QueryResult{Paid: false}
	paid, err = client.IsPaid(context.Background(), "NC-abc123")
	require.NoError(t, err)
	require.False(t, paid)

	gateway.queryErr = fmt.Errorf("%w: dial tcp: timeout", payment.ErrEasyPayUpstream)
	_, err = client.GetOrderInfo(context.Background(), "NC-abc123")
	require.ErrorIs(t, err, payment.ErrEasyPayUpstream)
}

func TestEasyPayCheckoutClientFetchDirectPaymentQRUnsupported(t *testing.T) {
	client := newEasyPayCheckoutClientForTest(&easyPayGatewayStub{}, easyPayCheckoutSettingsStub{})
	_, _, err := client.FetchDirectPaymentQR(context.Background(), "https://pay.example.com/cashier/x")
	require.Error(t, err)
	require.Equal(t, "NATIVE_CHECKOUT_QR_UNSUPPORTED", infraerrors.Reason(err))
}

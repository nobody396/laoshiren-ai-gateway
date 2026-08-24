// Package payment provides the payment-gateway abstraction shared by topup and
// native checkout flows. Providers are identified by name (e.g. "easypay") and
// resolve their runtime configuration from DB-backed settings on every call.
package payment

import (
	"context"
	"net/url"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

// Method is the internal payment channel name. Provider-specific spellings
// (e.g. EasyPay uses "wxpay" for WeChat) are mapped inside each provider.
type Method string

const (
	MethodAlipay Method = "alipay"
	MethodWechat Method = "wechat"
)

// CreateOrderRequest carries everything a provider needs to open a payment.
type CreateOrderRequest struct {
	OutTradeNo   string
	Method       Method
	AmountCNYFen int
	Subject      string
	NotifyURL    string
	ReturnURL    string
	ClientIP     string
}

// CreateOrderResult is the provider's response to a successful order creation.
type CreateOrderResult struct {
	TradeNo    string
	PayURL     string
	QRContent  string
	QRImageURL string
}

// NotifyResult is the verified content of an asynchronous payment notify.
type NotifyResult struct {
	OutTradeNo   string
	TradeNo      string
	Method       string
	AmountCNYFen int
	Paid         bool
}

// QueryResult is the provider-side state of an order.
type QueryResult struct {
	Paid         bool
	TradeNo      string
	AmountCNYFen int
}

// Provider is a payment gateway that can create orders, verify asynchronous
// notifies, and query order state.
type Provider interface {
	Name() string
	CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResult, error)
	VerifyNotify(params url.Values) (*NotifyResult, error)
	QueryOrder(ctx context.Context, outTradeNo string) (*QueryResult, error)
}

// ErrPaymentProviderNotFound is returned by Registry.Get for unknown providers.
var ErrPaymentProviderNotFound = infraerrors.NotFound("PAYMENT_PROVIDER_NOT_FOUND", "payment provider is not registered")

// EasyPayNotifyPath is the public route that receives EasyPay asynchronous
// notifies (registered in server/router.go). Topup and native checkout orders
// share this single endpoint; the handler dispatches on the out_trade_no
// prefix (TP* vs NC-*).
const EasyPayNotifyPath = "/api/v1/pay/notify/easypay"

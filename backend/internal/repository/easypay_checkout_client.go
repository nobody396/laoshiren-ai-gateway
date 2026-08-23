package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/payment"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

// easyPayCheckoutSubject is the concrete goods title shown on the EasyPay
// cashier. ZPay requires the title to describe the actual product rather than
// a vague internal promotion name.
const easyPayCheckoutSubject = "老实人AI API调用额度包"

// easyPayGateway is the subset of *payment.EasyPayClient the native checkout
// provider needs. Kept as a narrow interface so tests can stub the gateway.
type easyPayGateway interface {
	CreateOrder(ctx context.Context, req *payment.CreateOrderRequest) (*payment.CreateOrderResult, error)
	QueryOrder(ctx context.Context, outTradeNo string) (*payment.QueryResult, error)
}

// easyPayCheckoutSettings supplies the runtime configuration the provider
// reads per call. *service.SettingService satisfies it implicitly.
type easyPayCheckoutSettings interface {
	GetEasyPayConfig(ctx context.Context) (pid, key, apiBase string, enabled bool, err error)
	GetFrontendURL(ctx context.Context) string
}

// EasyPayCheckoutClient adapts the EasyPay payment gateway to the native
// checkout provider contract. EasyPay collects money but delivers no card
// stock: the checkout order's out_trade_no is our own NC- order number (so
// notifies and queries match the durable reservation), and the redeem code is
// minted locally by the service after the payment is confirmed.
type EasyPayCheckoutClient struct {
	gateway  easyPayGateway
	settings easyPayCheckoutSettings
}

func NewEasyPayCheckoutClient(gateway *payment.EasyPayClient, settings *service.SettingService) *EasyPayCheckoutClient {
	return &EasyPayCheckoutClient{gateway: gateway, settings: settings}
}

// ValidateOffer verifies the EasyPay gateway is enabled and fully configured.
// EasyPay goods are generic payment pages, so the goods key itself needs no
// upstream validation; the amount is enforced again at create time by the
// provider order identity checks.
func (c *EasyPayCheckoutClient) ValidateOffer(ctx context.Context, goodsKey string, expectedAmountCNYFen int64) error {
	if strings.TrimSpace(goodsKey) == "" || expectedAmountCNYFen <= 0 {
		return infraerrors.BadRequest("NATIVE_CHECKOUT_OFFER_INVALID", "native checkout offer is missing goods key or amount")
	}
	pid, key, _, enabled, err := c.settings.GetEasyPayConfig(ctx)
	if err != nil {
		return err
	}
	if !enabled || strings.TrimSpace(pid) == "" || strings.TrimSpace(key) == "" {
		return payment.ErrEasyPayNotConfigured
	}
	return nil
}

// CreateOrder opens an EasyPay payment. The platform out_trade_no MUST be our
// NC- order number: the notify endpoint dispatches on that prefix and the
// reconcile query looks orders up by it.
func (c *EasyPayCheckoutClient) CreateOrder(ctx context.Context, req *service.NativeCheckoutCreateRequest) (*service.NativeCheckoutProviderOrder, error) {
	notifyURL := c.notifyURL(ctx)
	if notifyURL == "" {
		return nil, &service.NativeCheckoutProviderError{
			Cause: infraerrors.BadRequest("EASYPAY_NOTIFY_URL_MISSING", "easypay notify url is not configured (frontend url missing)"),
		}
	}
	method := payment.MethodAlipay
	paymentMethod := service.NativeCheckoutPaymentMethodAlipay
	if strings.TrimSpace(req.PayType) == service.NativeCheckoutPaymentMethodWeChat {
		method = payment.MethodWechat
		paymentMethod = service.NativeCheckoutPaymentMethodWeChat
	}
	result, err := c.gateway.CreateOrder(ctx, &payment.CreateOrderRequest{
		OutTradeNo:   req.OrderNo,
		Method:       method,
		AmountCNYFen: int(req.ExpectedAmountCNYFen),
		Subject:      easyPayCheckoutSubject,
		NotifyURL:    notifyURL,
		ReturnURL:    c.returnURL(ctx),
		ClientIP:     req.ClientIP,
	})
	if err != nil {
		// Transport failures, timeouts, 5xx and undecodable responses are
		// ambiguous: the payable order may exist upstream and must never be
		// blindly recreated. Decoded rejections (config, params, business
		// rules) are definitive.
		return nil, &service.NativeCheckoutProviderError{
			Ambiguous: errors.Is(err, payment.ErrEasyPayUpstream),
			Cause:     err,
		}
	}
	// Native checkout persists a scannable payload, not a provider-hosted QR
	// image URL. ZPay normally returns both qrcode and img; qrcode remains
	// renderable after image-CDN expiry and avoids treating a page URL as an
	// image. Topup may use QRImageURL directly in its own response path.
	paymentURL := result.QRContent
	if paymentURL == "" {
		paymentURL = result.PayURL
	}
	if paymentURL == "" {
		// The provider accepted the create but returned nothing the customer
		// can pay with. The order may exist upstream; hold for review.
		return nil, &service.NativeCheckoutProviderError{
			Ambiguous: true,
			Cause:     errors.New("easypay create returned no payment target"),
		}
	}
	return &service.NativeCheckoutProviderOrder{
		// Our order number is the only reference needed downstream: EasyPay
		// queries and notifies both key on out_trade_no.
		TradeNo:       req.OrderNo,
		PaymentURL:    paymentURL,
		PaymentMethod: paymentMethod,
	}, nil
}

func (c *EasyPayCheckoutClient) IsPaid(ctx context.Context, tradeNo string) (bool, error) {
	result, err := c.gateway.QueryOrder(ctx, tradeNo)
	if err != nil {
		return false, err
	}
	return result.Paid, nil
}

func (c *EasyPayCheckoutClient) GetOrderInfo(ctx context.Context, tradeNo string) (*service.NativeCheckoutProviderOrderInfo, error) {
	result, err := c.gateway.QueryOrder(ctx, tradeNo)
	if err != nil {
		return nil, err
	}
	return &service.NativeCheckoutProviderOrderInfo{
		// Echo the queried out_trade_no (our NC- order number, persisted as
		// provider_trade_no) so the service identity check compares the same
		// reference; the gateway's own trade_no is never queried by.
		TradeNo:     tradeNo,
		TotalCNYFen: int64(result.AmountCNYFen),
		Paid:        result.Paid,
		// Delivered stays false and RedeemCodes empty: the native checkout
		// service mints the internal redeem code itself once Paid is seen.
	}, nil
}

// FetchDirectPaymentQR is unsupported: EasyPay orders carry the payment URL
// (or QR content) directly and the frontend renders the QR client-side.
func (c *EasyPayCheckoutClient) FetchDirectPaymentQR(context.Context, string) ([]byte, string, error) {
	return nil, "", infraerrors.BadRequest("NATIVE_CHECKOUT_QR_UNSUPPORTED", "easypay checkout renders the payment QR client-side")
}

// notifyURL mirrors TopupService.resolveEasyPayNotifyURL: both flows share
// the single EasyPay notify endpoint defined by the payment package.
func (c *EasyPayCheckoutClient) notifyURL(ctx context.Context) string {
	baseURL := strings.TrimRight(strings.TrimSpace(c.settings.GetFrontendURL(ctx)), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + payment.EasyPayNotifyPath
}

func (c *EasyPayCheckoutClient) returnURL(ctx context.Context) string {
	baseURL := strings.TrimRight(strings.TrimSpace(c.settings.GetFrontendURL(ctx)), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/dashboard"
}

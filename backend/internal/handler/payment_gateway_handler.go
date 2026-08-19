package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/bozhouDev/DragonCode-sub2api/internal/payment"

	"github.com/gin-gonic/gin"
)

var (
	errEasyPayNotifierUnavailable = errors.New("easypay notifier not available for this order")
	errEasyPayUnknownOrderPrefix  = errors.New("unknown easypay out_trade_no prefix")
)

// TopupEasyPayNotifier 是 TopupService 对 EasyPay 回调的处理入口。
type TopupEasyPayNotifier interface {
	HandleEasyPayNotify(ctx context.Context, n *payment.NotifyResult) error
}

// NativeCheckoutEasyPayNotifier 是 NativeCheckoutService 对 EasyPay 回调的处理入口。
// 与 TopupEasyPayNotifier 形状相同，但保持独立类型以便 wire 显式注入。
type NativeCheckoutEasyPayNotifier interface {
	HandleEasyPayNotify(ctx context.Context, n *payment.NotifyResult) error
}

// PaymentGatewayHandler 处理统一支付网关回调（无 JWT 鉴权，靠网关签名验证）。
type PaymentGatewayHandler struct {
	registry *payment.Registry
	topup    TopupEasyPayNotifier
	native   NativeCheckoutEasyPayNotifier
}

// NewPaymentGatewayHandler creates a new PaymentGatewayHandler. native is
// always wired in production via ProvideNativeCheckoutEasyPayNotifier; it
// remains nullable so tests can exercise the fail-fast fallback that keeps
// the platform retrying when no notifier is available.
func NewPaymentGatewayHandler(
	registry *payment.Registry,
	topup TopupEasyPayNotifier,
	native NativeCheckoutEasyPayNotifier,
) *PaymentGatewayHandler {
	return &PaymentGatewayHandler{registry: registry, topup: topup, native: native}
}

// HandleEasyPayNotify 处理 EasyPay 异步回调
// POST/GET /api/v1/pay/notify/easypay
// 成功应答纯文本 "success"（200），失败应答 "fail"（400），与虎皮椒回调契约一致。
func (h *PaymentGatewayHandler) HandleEasyPayNotify(c *gin.Context) {
	var params url.Values
	if c.Request.Method == http.MethodGet {
		params = c.Request.URL.Query()
	} else {
		if err := c.Request.ParseForm(); err != nil {
			slog.Warn("easypay notify parse form failed", "error", err)
			c.String(http.StatusBadRequest, "fail")
			return
		}
		params = c.Request.Form
	}

	gateway, err := h.registry.Get(payment.ProviderEasyPay)
	if err != nil {
		slog.Warn("easypay notify rejected: provider unavailable", "error", err)
		c.String(http.StatusBadRequest, "fail")
		return
	}

	notify, err := gateway.VerifyNotify(params)
	if err != nil {
		// 验签/参数错误均为脱敏后的 coded error，不含商户密钥
		slog.Warn("easypay notify verify failed", "error", err)
		c.String(http.StatusBadRequest, "fail")
		return
	}

	if err := h.dispatchEasyPayNotify(c, notify); err != nil {
		slog.Warn("easypay notify dispatch failed", "out_trade_no", notify.OutTradeNo, "error", err)
		c.String(http.StatusBadRequest, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

// dispatchEasyPayNotify 按商户订单号前缀路由到对应业务方。
// TP* → 余额充值；NC-* → native checkout；其余前缀拒绝。
func (h *PaymentGatewayHandler) dispatchEasyPayNotify(c *gin.Context, n *payment.NotifyResult) error {
	switch {
	case strings.HasPrefix(n.OutTradeNo, "TP"):
		if h.topup == nil {
			return errEasyPayNotifierUnavailable
		}
		return h.topup.HandleEasyPayNotify(c.Request.Context(), n)
	case strings.HasPrefix(n.OutTradeNo, "NC-"):
		if h.native == nil {
			// 无 native notifier 时答 fail 让平台重试（仅测试环境会出现）。
			return errEasyPayNotifierUnavailable
		}
		return h.native.HandleEasyPayNotify(c.Request.Context(), n)
	default:
		return errEasyPayUnknownOrderPrefix
	}
}

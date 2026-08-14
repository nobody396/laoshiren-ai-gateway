package handler

import (
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNativeCheckoutOrderDTOIncludesSelectedPaymentMethod(t *testing.T) {
	createdAt := time.Date(2026, time.August, 13, 12, 0, 0, 0, time.UTC)
	order := &service.NativeCheckoutOrder{
		OrderNo:             "NC-ALIPAY",
		Status:              service.NativeCheckoutStatusPending,
		PayAmountCNYFen:     500,
		BenefitAmountCNYFen: 1000,
		PaymentURL:          "https://pay.ldxp.cn/pay/NC-ALIPAY",
		PaymentMethod:       service.NativeCheckoutPaymentMethodAlipay,
		CreatedAt:           createdAt,
	}

	result := nativeCheckoutOrderDTO(order)
	require.Equal(t, service.NativeCheckoutPaymentMethodAlipay, result.PaymentMethod)
	require.Equal(t, order.PaymentURL, result.PaymentURL)
	require.Equal(t, "/native-checkout/orders/NC-ALIPAY/qr", result.DirectQRURL)
	require.Equal(t, createdAt, result.CreatedAt)
}

package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLDXPCheckoutClientBuyerFlowAndDirectQR(t *testing.T) {
	const redeemCode = "0123456789abcdef0123456789abcdef"
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shopApi/Shop/goodsInfo":
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{
				"goods_type": "card", "goods_key": "trial-key", "status": 1,
				"price": 1, "real_price": 1, "contact_format": "email",
				"user": map[string]any{"token": "public-shop-token"},
			}})
		case "/shopApi/Shop/getUserChannel":
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": []map[string]any{
				{"id": 4, "code": "WeixinNative", "status": 1, "custom_status": 1},
			}})
		case "/shopApi/Pay/order":
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{
				"trade_no": "LD-TEST-1", "total_amount": 1, "payurl": server.URL + "/pay/LD-TEST-1",
			}})
		case "/shopApi/Pay/query":
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": nil})
		case "/shopApi/Order/info":
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{
				"trade_no": "LD-TEST-1", "goods": map[string]any{"goods_key": "trial-key"},
				"contact": "buyer@example.com", "quantity": 1, "total_amount": "1.00",
				"status": 1, "sendout": 1,
				"response": map[string]any{"cards": []string{"兑换码：" + redeemCode}},
			}})
		case "/pay/LD-TEST-1":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body><img class="code" src="/generateQrcode/test"></body></html>`))
		case "/generateQrcode/test":
			// LDXP has historically sent a wrong HTML content type for this PNG;
			// magic-byte validation is the security boundary.
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write(append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 24)...))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := newLDXPCheckoutClient(server.URL, true)
	require.NoError(t, err)
	ctx := context.Background()
	order, err := client.CreateOrder(ctx, "trial-key", "buyer@example.com", 100)
	require.NoError(t, err)
	require.Equal(t, "LD-TEST-1", order.TradeNo)
	require.Equal(t, server.URL+"/pay/LD-TEST-1", order.PaymentURL)

	paid, err := client.IsPaid(ctx, order.TradeNo)
	require.NoError(t, err)
	require.True(t, paid)

	info, err := client.GetOrderInfo(ctx, order.TradeNo)
	require.NoError(t, err)
	require.Equal(t, int64(100), info.TotalCNYFen)
	require.Equal(t, []string{redeemCode}, info.RedeemCodes)
	require.True(t, info.Paid)
	require.True(t, info.Delivered)

	image, contentType, err := client.FetchDirectPaymentQR(ctx, order.PaymentURL)
	require.NoError(t, err)
	require.Equal(t, "image/png", contentType)
	require.True(t, len(image) > 8)
}

func TestLDXPCheckoutClientRejectsWrongGoodsAmountBeforeOrder(t *testing.T) {
	orderCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/shopApi/Shop/goodsInfo" {
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{
				"goods_type": "card", "goods_key": "trial-key", "status": 1,
				"price": 20, "real_price": 20, "contact_format": "email",
				"user": map[string]any{"token": "public-shop-token"},
			}})
			return
		}
		if r.URL.Path == "/shopApi/Pay/order" {
			orderCalled = true
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	client, err := newLDXPCheckoutClient(server.URL, true)
	require.NoError(t, err)

	_, err = client.CreateOrder(context.Background(), "trial-key", "buyer@example.com", 100)
	require.Error(t, err)
	require.False(t, orderCalled)
	var providerErr *service.NativeCheckoutProviderError
	require.ErrorAs(t, err, &providerErr)
	require.False(t, providerErr.Ambiguous)
}

func TestLDXPCheckoutClientDoesNotSubstituteAnotherPaymentChannel(t *testing.T) {
	orderCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shopApi/Shop/goodsInfo":
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{
				"goods_type": "card", "goods_key": "trial-key", "status": 1,
				"price": 1, "real_price": 1, "contact_format": "email",
				"user": map[string]any{"token": "public-shop-token"},
			}})
		case "/shopApi/Shop/getUserChannel":
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": []map[string]any{
				{"id": 2, "code": "Alipay", "status": 1, "custom_status": 1},
			}})
		case "/shopApi/Pay/order":
			orderCalled = true
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := newLDXPCheckoutClient(server.URL, true)
	require.NoError(t, err)

	_, err = client.CreateOrder(context.Background(), "trial-key", "buyer@example.com", 100)
	require.Error(t, err)
	require.False(t, orderCalled)
}

func TestLDXPCheckoutClientDoesNotFollowQRToAnotherHost(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<img class="code" src="https://example.com/qr.png">`))
	}))
	defer server.Close()
	client, err := newLDXPCheckoutClient(server.URL, true)
	require.NoError(t, err)

	_, _, err = client.FetchDirectPaymentQR(context.Background(), server.URL+"/pay/test")
	require.Error(t, err)
}

func writeLDXPJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(value))
}

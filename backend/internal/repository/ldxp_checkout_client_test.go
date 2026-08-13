package repository

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLDXPCheckoutClientBuyerFlowAndDirectQR(t *testing.T) {
	const redeemCode = "0123456789abcdef0123456789abcdef"
	createdChannelID := 0
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
			var request struct {
				ChannelID int `json:"channel_id"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			createdChannelID = request.ChannelID
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
	require.Equal(t, service.NativeCheckoutPaymentMethodWeChat, order.PaymentMethod)
	require.Equal(t, 4, createdChannelID)

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

func TestLDXPCheckoutClientSupportsAlipayAndLabelsSelectedMethod(t *testing.T) {
	createdChannelID := 0
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
				{"id": 2, "code": "Alipay", "status": 1, "custom_status": 1},
			}})
		case "/shopApi/Pay/order":
			var request struct {
				ChannelID int `json:"channel_id"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			createdChannelID = request.ChannelID
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{
				"trade_no": "LD-ALIPAY-1", "total_amount": 1, "payurl": server.URL + "/pay/LD-ALIPAY-1",
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client, err := newLDXPCheckoutClient(server.URL, true)
	require.NoError(t, err)

	order, err := client.CreateOrder(context.Background(), "trial-key", "buyer@example.com", 100)
	require.NoError(t, err)
	require.Equal(t, 2, createdChannelID)
	require.Equal(t, service.NativeCheckoutPaymentMethodAlipay, order.PaymentMethod)
}

func TestLDXPCheckoutClientCollapsesConcurrentOfferMetadataLookups(t *testing.T) {
	var goodsCalls atomic.Int32
	var channelCalls atomic.Int32
	var orderCalls atomic.Int32
	var activeOrders atomic.Int32
	var maxActiveOrders atomic.Int32
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shopApi/Shop/goodsInfo":
			goodsCalls.Add(1)
			// Keep the first lookup in flight so all concurrent buyers join it.
			time.Sleep(50 * time.Millisecond)
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{
				"goods_type": "card", "goods_key": "trial-key", "status": 1,
				"price": 1, "real_price": 1, "contact_format": "email",
				"user": map[string]any{"token": "public-shop-token"},
			}})
		case "/shopApi/Shop/getUserChannel":
			channelCalls.Add(1)
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": []map[string]any{
				{"id": 4, "code": "WeixinNative", "status": 1, "custom_status": 1},
			}})
		case "/shopApi/Pay/order":
			active := activeOrders.Add(1)
			defer activeOrders.Add(-1)
			for {
				previous := maxActiveOrders.Load()
				if active <= previous || maxActiveOrders.CompareAndSwap(previous, active) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			orderID := orderCalls.Add(1)
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{
				"trade_no":     "LD-CONCURRENT-" + strconv.FormatInt(int64(orderID), 10),
				"total_amount": 1,
				"payurl":       server.URL + "/pay/concurrent-" + strconv.FormatInt(int64(orderID), 10),
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := newLDXPCheckoutClient(server.URL, true)
	require.NoError(t, err)

	const buyers = 16
	var wg sync.WaitGroup
	errs := make(chan error, buyers)
	for i := range buyers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, createErr := client.CreateOrder(
				context.Background(),
				"trial-key",
				"buyer-"+strconv.Itoa(i)+"@example.com",
				100,
			)
			errs <- createErr
		}()
	}
	wg.Wait()
	close(errs)
	for createErr := range errs {
		require.NoError(t, createErr)
	}
	require.Equal(t, int32(1), goodsCalls.Load())
	require.Equal(t, int32(1), channelCalls.Load())
	require.Equal(t, int32(buyers), orderCalls.Load(), "each customer still needs an independent payable order")
	require.LessOrEqual(t, maxActiveOrders.Load(), int32(ldxpCreateConcurrency), "provider order creation must have bounded concurrency")
	require.Greater(t, maxActiveOrders.Load(), int32(1), "independent customers should still make progress concurrently")
}

func TestLDXPCheckoutClientRejectsUnsupportedPaymentChannel(t *testing.T) {
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
				{"id": 9, "code": "UnionPay", "status": 1, "custom_status": 1},
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

func TestLDXPCheckoutClientFallsBackToCachedMerchantSession(t *testing.T) {
	const redeemCode = "fedcba9876543210fedcba9876543210"
	loginCalls := 0
	merchantInfoCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shopApi/Order/info":
			writeLDXPJSON(t, w, map[string]any{"code": 0, "msg": "contact lookup required", "data": nil})
		case "/merchantApi/user/login":
			loginCalls++
			var request struct {
				Username string `json:"username"`
				Password string `json:"password"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			require.Equal(t, "merchant-user", request.Username)
			require.Equal(t, "merchant-password", request.Password)
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{"merchant_token": "session-token"}})
		case "/merchantApi/Order/orderInfo":
			merchantInfoCalls++
			require.Equal(t, "session-token", r.Header.Get("Merchant-Token"))
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{
				"trade_no": "LD-FALLBACK-1", "goods": map[string]any{"goods_key": "trial-key"},
				"contact": "buyer@example.com", "quantity": 1, "total_amount": "1.00",
				"status": 1, "sendout": 1,
				"response": map[string]any{"cards": []string{"兑换码：" + redeemCode}},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := newLDXPCheckoutClient(server.URL, true)
	require.NoError(t, err)
	client.merchantUsername = "merchant-user"
	client.merchantPassword = "merchant-password"

	for range 2 {
		info, lookupErr := client.GetOrderInfo(context.Background(), "LD-FALLBACK-1")
		require.NoError(t, lookupErr)
		require.True(t, info.Paid)
		require.True(t, info.Delivered)
		require.Equal(t, []string{redeemCode}, info.RedeemCodes)
	}
	require.Equal(t, 1, loginCalls, "the merchant token must remain process-local and be reused")
	require.Equal(t, 2, merchantInfoCalls)
}

func TestLDXPCheckoutClientBacksOffBlockedBuyerDetailButKeepsMerchantRecovery(t *testing.T) {
	const redeemCode = "fedcba9876543210fedcba9876543210"
	buyerCalls := 0
	loginCalls := 0
	merchantInfoCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shopApi/Order/info":
			buyerCalls++
			http.Error(w, "blocked", http.StatusForbidden)
		case "/merchantApi/user/login":
			loginCalls++
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{"merchant_token": "session-token"}})
		case "/merchantApi/Order/orderInfo":
			merchantInfoCalls++
			writeLDXPJSON(t, w, map[string]any{"code": 1, "data": map[string]any{
				"trade_no": "LD-FALLBACK-1", "goods": map[string]any{"goods_key": "trial-key"},
				"contact": "buyer@example.com", "quantity": 1, "total_amount": "1.00",
				"status": 1, "sendout": 1,
				"response": map[string]any{"cards": []string{"兑换码：" + redeemCode}},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := newLDXPCheckoutClient(server.URL, true)
	require.NoError(t, err)
	client.merchantUsername = "merchant-user"
	client.merchantPassword = "merchant-password"

	for range 2 {
		info, lookupErr := client.GetOrderInfo(context.Background(), "LD-FALLBACK-1")
		require.NoError(t, lookupErr)
		require.Equal(t, []string{redeemCode}, info.RedeemCodes)
	}
	require.Equal(t, 1, buyerCalls, "a provider 403 must open the buyer-detail circuit")
	require.Equal(t, 1, loginCalls)
	require.Equal(t, 2, merchantInfoCalls)
}

func TestLDXPCheckoutClientBacksOffAfterMerchantLoginFailure(t *testing.T) {
	loginCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/shopApi/Order/info":
			writeLDXPJSON(t, w, map[string]any{"code": 0, "msg": "contact lookup required", "data": nil})
		case "/merchantApi/user/login":
			loginCalls++
			writeLDXPJSON(t, w, map[string]any{"code": 0, "msg": "try later", "data": nil})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := newLDXPCheckoutClient(server.URL, true)
	require.NoError(t, err)
	client.merchantUsername = "merchant-user"
	client.merchantPassword = "merchant-password"

	for range 2 {
		_, lookupErr := client.GetOrderInfo(context.Background(), "LD-FALLBACK-1")
		require.Error(t, lookupErr)
	}
	require.Equal(t, 1, loginCalls, "provider rate limits must not trigger a login storm")
}

func writeLDXPJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(value))
}

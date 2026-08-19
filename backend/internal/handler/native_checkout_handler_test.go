package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	middleware2 "github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
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

func TestNativeCheckoutManualPurchaseEndpointRechecksCurrentAccountClaim(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name        string
		claimed     bool
		purchaseURL string
	}{
		{name: "eligible", purchaseURL: "https://pay.ldxp.cn/item/oc3w4r"},
		{name: "already claimed", claimed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &nativeCheckoutManualOfferRepoStub{claimed: tt.claimed}
			svc := service.NewNativeCheckoutService(repo, nil, nil, nil, "test-contact-key")
			handler := NewNativeCheckoutHandler(svc)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest("POST", "/native-checkout/manual-offers/newcomer-balance-5-to-10/purchase", nil)
			c.Params = gin.Params{{Key: "offerCode", Value: "newcomer-balance-5-to-10"}}
			c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

			handler.GetManualOfferPurchase(c)

			require.Equal(t, 200, recorder.Code)
			var payload struct {
				Data nativeCheckoutManualOfferStatusResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
			require.Equal(t, tt.claimed, payload.Data.Claimed)
			require.Equal(t, tt.purchaseURL, payload.Data.PurchaseURL)
		})
	}
}

type nativeCheckoutManualOfferRepoStub struct {
	service.NativeCheckoutRepository
	claimed bool
}

func (r *nativeCheckoutManualOfferRepoStub) GetManualRedeemOffer(context.Context, string) (*service.NativeCheckoutOffer, error) {
	return &service.NativeCheckoutOffer{
		Code:             "newcomer-balance-5-to-10",
		Provider:         "ldxp",
		ProviderGoodsKey: "oc3w4r",
		OncePerUser:      true,
	}, nil
}

func (r *nativeCheckoutManualOfferRepoStub) HasRedeemedOffer(context.Context, int64, string) (bool, error) {
	return r.claimed, nil
}

func TestNativeCheckoutListOffersExposesProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &nativeCheckoutOfferListRepoStub{offer: service.NativeCheckoutOffer{
		Code: "newcomer-balance-5-to-10", Provider: "easypay", ProviderGoodsKey: "newcomer-balance-5-to-10",
		Name: "新人专享 · 10 元余额包", ProductKind: "balance", PayAmountCNYFen: 500, BenefitAmountCNYFen: 1000,
		OncePerUser: true,
	}}
	svc := service.NewNativeCheckoutService(repo, nil, nil, nil, "test-contact-key")
	handler := NewNativeCheckoutHandler(svc)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/native-checkout/offers", nil)
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	handler.ListOffers(c)

	require.Equal(t, 200, recorder.Code)
	var payload struct {
		Data []nativeCheckoutOfferResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	require.Len(t, payload.Data, 1)
	require.Equal(t, "easypay", payload.Data[0].Provider)
}

func TestNativeCheckoutCreateOrderRejectsInvalidPayType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewNativeCheckoutHandler(nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/native-checkout/orders", strings.NewReader(`{"offer_code":"newcomer-balance-5-to-10","pay_type":"unionpay"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(string(middleware2.ContextKeyUser), middleware2.AuthSubject{UserID: 42})

	handler.CreateOrder(c)

	require.Equal(t, 400, recorder.Code, "pay_type outside alipay/wechat must be rejected at binding")
}

func TestNativeCheckoutOrderDTOIncludesProvider(t *testing.T) {
	order := &service.NativeCheckoutOrder{
		OrderNo: "NC-1", Status: service.NativeCheckoutStatusPending, Provider: "easypay",
		PayAmountCNYFen: 500, BenefitAmountCNYFen: 1000,
		PaymentURL: "https://pay.example.com/cashier/NC-1", PaymentMethod: service.NativeCheckoutPaymentMethodAlipay,
	}
	result := nativeCheckoutOrderDTO(order)
	require.Equal(t, "easypay", result.Provider)
	require.Equal(t, order.PaymentURL, result.PaymentURL)
}

type nativeCheckoutOfferListRepoStub struct {
	service.NativeCheckoutRepository
	offer service.NativeCheckoutOffer
}

func (r *nativeCheckoutOfferListRepoStub) ListVisibleOffers(context.Context, int64) ([]service.NativeCheckoutOffer, error) {
	return []service.NativeCheckoutOffer{r.offer}, nil
}

func (r *nativeCheckoutOfferListRepoStub) GetLatestOrderForOffer(context.Context, int64, string) (*service.NativeCheckoutOrder, error) {
	return nil, service.ErrNativeCheckoutOrderNotFound
}

func (r *nativeCheckoutOfferListRepoStub) HasRedeemedOffer(context.Context, int64, string) (bool, error) {
	return false, nil
}

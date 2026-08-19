//go:build unit

package payment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type stubEasyPaySettings struct {
	pid     string
	key     string
	apiBase string
	enabled bool
	err     error
}

func (s stubEasyPaySettings) GetEasyPayConfig(ctx context.Context) (string, string, string, bool, error) {
	return s.pid, s.key, s.apiBase, s.enabled, s.err
}

const (
	testEasyPayPID = "1001"
	testEasyPayKey = "testkey123"
)

func testEasyPayClient(apiBase string) *EasyPayClient {
	return NewEasyPayClient(stubEasyPaySettings{
		pid:     testEasyPayPID,
		key:     testEasyPayKey,
		apiBase: apiBase,
		enabled: true,
	})
}

// signedNotifyParams builds notify params signed exactly as the platform does.
func signedNotifyParams(t *testing.T, params map[string]string) url.Values {
	t.Helper()
	params["sign"] = easyPaySign(params, testEasyPayKey)
	params["sign_type"] = "MD5"
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	return form
}

func TestEasyPaySign_KnownVectors(t *testing.T) {
	t.Run("create order params", func(t *testing.T) {
		params := map[string]string{
			"money":        "10.00",
			"name":         "Test Order",
			"notify_url":   "https://example.com/notify",
			"out_trade_no": "T20260819001",
			"pid":          testEasyPayPID,
			"type":         "alipay",
		}
		require.Equal(t,
			"money=10.00&name=Test Order&notify_url=https://example.com/notify&out_trade_no=T20260819001&pid=1001&type=alipay",
			easyPaySignString(params))
		require.Equal(t, "9ae805432e63f82344af8ea3f01c75ef", easyPaySign(params, testEasyPayKey))
	})

	t.Run("notify params", func(t *testing.T) {
		params := map[string]string{
			"money":        "5.00",
			"out_trade_no": "N20260819002",
			"pid":          testEasyPayPID,
			"trade_no":     "2026081922001",
			"trade_status": "TRADE_SUCCESS",
			"type":         "wxpay",
		}
		require.Equal(t, "33f4aa5117033e9387be608577a8d660", easyPaySign(params, testEasyPayKey))
	})

	t.Run("excludes sign, sign_type and empty values", func(t *testing.T) {
		base := map[string]string{
			"money":        "10.00",
			"name":         "Test Order",
			"notify_url":   "https://example.com/notify",
			"out_trade_no": "T20260819001",
			"pid":          testEasyPayPID,
			"type":         "alipay",
		}
		withExtras := map[string]string{
			"money":        "10.00",
			"name":         "Test Order",
			"notify_url":   "https://example.com/notify",
			"out_trade_no": "T20260819001",
			"pid":          testEasyPayPID,
			"type":         "alipay",
			"sign":         "deadbeef",
			"sign_type":    "MD5",
			"return_url":   "",
		}
		require.Equal(t, easyPaySign(base, testEasyPayKey), easyPaySign(withExtras, testEasyPayKey))
	})
}

func TestEasyPayVerifyNotify(t *testing.T) {
	client := testEasyPayClient("")

	baseParams := func() map[string]string {
		return map[string]string{
			"pid":          testEasyPayPID,
			"trade_no":     "2026081922001",
			"out_trade_no": "N20260819002",
			"type":         "wxpay",
			"money":        "5.00",
			"trade_status": "TRADE_SUCCESS",
		}
	}

	t.Run("valid paid notify", func(t *testing.T) {
		result, err := client.VerifyNotify(signedNotifyParams(t, baseParams()))
		require.NoError(t, err)
		require.True(t, result.Paid)
		require.Equal(t, "N20260819002", result.OutTradeNo)
		require.Equal(t, "2026081922001", result.TradeNo)
		require.Equal(t, string(MethodWechat), result.Method)
		require.Equal(t, 500, result.AmountCNYFen)
	})

	t.Run("not paid status", func(t *testing.T) {
		params := baseParams()
		params["trade_status"] = "TRADE_CLOSED"
		result, err := client.VerifyNotify(signedNotifyParams(t, params))
		require.NoError(t, err)
		require.False(t, result.Paid)
	})

	t.Run("notify without name field", func(t *testing.T) {
		// Merchant backends can strip name; it must never be required.
		params := baseParams()
		result, err := client.VerifyNotify(signedNotifyParams(t, params))
		require.NoError(t, err)
		require.True(t, result.Paid)
	})

	t.Run("empty type yields empty method", func(t *testing.T) {
		params := baseParams()
		params["type"] = ""
		result, err := client.VerifyNotify(signedNotifyParams(t, params))
		require.NoError(t, err)
		require.Empty(t, result.Method)
	})

	t.Run("bad sign", func(t *testing.T) {
		form := signedNotifyParams(t, baseParams())
		form.Set("sign", "00000000000000000000000000000000")
		_, err := client.VerifyNotify(form)
		require.ErrorIs(t, err, ErrEasyPayInvalidSign)
	})

	t.Run("sign computed over tampered money", func(t *testing.T) {
		form := signedNotifyParams(t, baseParams())
		form.Set("money", "0.01")
		_, err := client.VerifyNotify(form)
		require.ErrorIs(t, err, ErrEasyPayInvalidSign)
	})

	t.Run("wrong pid", func(t *testing.T) {
		params := baseParams()
		params["pid"] = "9999"
		_, err := client.VerifyNotify(signedNotifyParams(t, params))
		require.ErrorIs(t, err, ErrEasyPayNotifyFailed)
	})

	t.Run("missing money", func(t *testing.T) {
		params := baseParams()
		delete(params, "money")
		_, err := client.VerifyNotify(signedNotifyParams(t, params))
		require.ErrorIs(t, err, ErrEasyPayNotifyFailed)
	})

	t.Run("unparseable money", func(t *testing.T) {
		params := baseParams()
		params["money"] = "abc"
		_, err := client.VerifyNotify(signedNotifyParams(t, params))
		require.ErrorIs(t, err, ErrEasyPayNotifyFailed)
	})

	t.Run("missing out_trade_no", func(t *testing.T) {
		params := baseParams()
		delete(params, "out_trade_no")
		_, err := client.VerifyNotify(signedNotifyParams(t, params))
		require.ErrorIs(t, err, ErrEasyPayNotifyFailed)
	})

	t.Run("not configured", func(t *testing.T) {
		disabled := NewEasyPayClient(stubEasyPaySettings{enabled: false})
		_, err := disabled.VerifyNotify(signedNotifyParams(t, baseParams()))
		require.ErrorIs(t, err, ErrEasyPayNotConfigured)
	})
}

func TestEasyPayCreateOrder(t *testing.T) {
	t.Run("success maps payurl, qrcode and trade_no", func(t *testing.T) {
		var gotForm url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, easyPayCreatePath, r.URL.Path)
			require.NoError(t, r.ParseForm())
			gotForm = r.PostForm
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":     1,
				"msg":      "success",
				"payurl":   "https://pay.hueling.cc/pay/abc",
				"qrcode":   "weixin://wxpay/bizpayurl?pr=abc",
				"trade_no": "2026081922001",
			})
		}))
		defer server.Close()

		client := testEasyPayClient(server.URL)
		result, err := client.CreateOrder(context.Background(), &CreateOrderRequest{
			OutTradeNo:   "T20260819001",
			Method:       MethodWechat,
			AmountCNYFen: 1000,
			Subject:      "Balance topup",
			NotifyURL:    "https://example.com/notify",
			ReturnURL:    "https://example.com/return",
		})
		require.NoError(t, err)
		require.Equal(t, "https://pay.hueling.cc/pay/abc", result.PayURL)
		require.Equal(t, "weixin://wxpay/bizpayurl?pr=abc", result.QRContent)
		require.Equal(t, "2026081922001", result.TradeNo)

		// Verify the exact form the client sent: internal wechat maps to wxpay,
		// money is yuan with 2 decimals, and the sign is valid.
		require.Equal(t, testEasyPayPID, gotForm.Get("pid"))
		require.Equal(t, "wxpay", gotForm.Get("type"))
		require.Equal(t, "10.00", gotForm.Get("money"))
		require.Equal(t, "T20260819001", gotForm.Get("out_trade_no"))
		require.Equal(t, "https://example.com/notify", gotForm.Get("notify_url"))
		require.Equal(t, "https://example.com/return", gotForm.Get("return_url"))
		require.Equal(t, "Balance topup", gotForm.Get("name"))
		require.Equal(t, "MD5", gotForm.Get("sign_type"))

		sent := make(map[string]string, len(gotForm))
		for k, vs := range gotForm {
			sent[k] = vs[0]
		}
		require.Equal(t, easyPaySign(sent, testEasyPayKey), gotForm.Get("sign"))
	})

	t.Run("return_url omitted when empty", func(t *testing.T) {
		var gotForm url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.NoError(t, r.ParseForm())
			gotForm = r.PostForm
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "success", "payurl": "https://pay.hueling.cc/pay/x"})
		}))
		defer server.Close()

		client := testEasyPayClient(server.URL)
		_, err := client.CreateOrder(context.Background(), &CreateOrderRequest{
			OutTradeNo:   "T20260819002",
			Method:       MethodAlipay,
			AmountCNYFen: 500,
			Subject:      "Balance topup",
			NotifyURL:    "https://example.com/notify",
		})
		require.NoError(t, err)
		require.Equal(t, "alipay", gotForm.Get("type"))
		_, hasReturnURL := gotForm["return_url"]
		require.False(t, hasReturnURL)
	})

	t.Run("missing trade_no tolerated", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "msg": "success", "payurl": "https://pay.hueling.cc/pay/y"})
		}))
		defer server.Close()

		client := testEasyPayClient(server.URL)
		result, err := client.CreateOrder(context.Background(), &CreateOrderRequest{
			OutTradeNo:   "T20260819003",
			Method:       MethodAlipay,
			AmountCNYFen: 500,
			Subject:      "Balance topup",
			NotifyURL:    "https://example.com/notify",
		})
		require.NoError(t, err)
		require.Empty(t, result.TradeNo)
	})

	t.Run("api error surfaces sanitized msg", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "没有找到商户信息"})
		}))
		defer server.Close()

		client := testEasyPayClient(server.URL)
		_, err := client.CreateOrder(context.Background(), &CreateOrderRequest{
			OutTradeNo:   "T20260819004",
			Method:       MethodAlipay,
			AmountCNYFen: 500,
			Subject:      "Balance topup",
			NotifyURL:    "https://example.com/notify",
		})
		require.Error(t, err)
		require.Equal(t, "EASYPAY_API_ERROR", infraerrors.Reason(err))
		require.Contains(t, err.Error(), "没有找到商户信息")
		require.NotContains(t, err.Error(), testEasyPayKey)
	})

	t.Run("unsupported method", func(t *testing.T) {
		client := testEasyPayClient("http://127.0.0.1:0")
		_, err := client.CreateOrder(context.Background(), &CreateOrderRequest{
			OutTradeNo:   "T20260819005",
			Method:       Method("unionpay"),
			AmountCNYFen: 500,
			Subject:      "Balance topup",
			NotifyURL:    "https://example.com/notify",
		})
		require.Error(t, err)
		require.Equal(t, "EASYPAY_UNSUPPORTED_METHOD", infraerrors.Reason(err))
	})

	t.Run("not configured", func(t *testing.T) {
		client := NewEasyPayClient(stubEasyPaySettings{enabled: true, pid: "", key: ""})
		_, err := client.CreateOrder(context.Background(), &CreateOrderRequest{
			OutTradeNo:   "T20260819006",
			Method:       MethodAlipay,
			AmountCNYFen: 500,
			Subject:      "Balance topup",
			NotifyURL:    "https://example.com/notify",
		})
		require.ErrorIs(t, err, ErrEasyPayNotConfigured)
	})
}

func TestEasyPayQueryOrder(t *testing.T) {
	t.Run("paid order", func(t *testing.T) {
		var gotQuery url.Values
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, easyPayQueryPath, r.URL.Path)
			gotQuery = r.URL.Query()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":     1,
				"status":   1,
				"trade_no": "2026081922001",
				"money":    "5.00",
			})
		}))
		defer server.Close()

		client := testEasyPayClient(server.URL)
		result, err := client.QueryOrder(context.Background(), "N20260819002")
		require.NoError(t, err)
		require.True(t, result.Paid)
		require.Equal(t, "2026081922001", result.TradeNo)
		require.Equal(t, 500, result.AmountCNYFen)

		require.Equal(t, "order", gotQuery.Get("act"))
		require.Equal(t, testEasyPayPID, gotQuery.Get("pid"))
		require.Equal(t, testEasyPayKey, gotQuery.Get("key"))
		require.Equal(t, "N20260819002", gotQuery.Get("out_trade_no"))
	})

	t.Run("unpaid order with numeric money", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "status": 0, "money": 10.5})
		}))
		defer server.Close()

		client := testEasyPayClient(server.URL)
		result, err := client.QueryOrder(context.Background(), "N20260819003")
		require.NoError(t, err)
		require.False(t, result.Paid)
		require.Equal(t, 1050, result.AmountCNYFen)
	})

	t.Run("missing optional fields tolerated", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 1, "status": 0})
		}))
		defer server.Close()

		client := testEasyPayClient(server.URL)
		result, err := client.QueryOrder(context.Background(), "N20260819004")
		require.NoError(t, err)
		require.False(t, result.Paid)
		require.Empty(t, result.TradeNo)
		require.Zero(t, result.AmountCNYFen)
	})

	t.Run("api error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "msg": "没有找到商户信息"})
		}))
		defer server.Close()

		client := testEasyPayClient(server.URL)
		_, err := client.QueryOrder(context.Background(), "N20260819005")
		require.Error(t, err)
		require.Equal(t, "EASYPAY_API_ERROR", infraerrors.Reason(err))
	})

	t.Run("upstream failure", func(t *testing.T) {
		client := testEasyPayClient("http://127.0.0.1:0")
		_, err := client.QueryOrder(context.Background(), "N20260819006")
		require.ErrorIs(t, err, ErrEasyPayUpstream)
	})

	t.Run("config source error propagates", func(t *testing.T) {
		boom := errors.New("db down")
		client := NewEasyPayClient(stubEasyPaySettings{err: boom})
		_, err := client.QueryOrder(context.Background(), "N20260819007")
		require.ErrorIs(t, err, boom)
	})
}

func TestRegistry(t *testing.T) {
	easypay := testEasyPayClient("")
	registry := NewRegistry(easypay)

	got, err := registry.Get(ProviderEasyPay)
	require.NoError(t, err)
	require.Equal(t, easypay, got)
	require.Equal(t, ProviderEasyPay, got.Name())

	_, err = registry.Get("stripe")
	require.ErrorIs(t, err, ErrPaymentProviderNotFound)
	require.Equal(t, "PAYMENT_PROVIDER_NOT_FOUND", infraerrors.Reason(err))
}

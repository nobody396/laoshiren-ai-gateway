package payment

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const (
	// defaultEasyPayAPIBase is the 皮卡丘支付 gateway used when the admin has
	// not overridden easypay_api_base.
	defaultEasyPayAPIBase = "https://pay.hueling.cc"
	easyPayCreatePath     = "/xpay/epay/mapi.php"
	easyPayQueryPath      = "/xpay/epay/api.php"
	easyPaySignType       = "MD5"
	easyPayTradeSuccess   = "TRADE_SUCCESS"
	easyPayMaxBodyBytes   = 1 << 20
	easyPayHTTPTimeout    = 15 * time.Second
)

var (
	// ErrEasyPayNotConfigured is returned when the gateway is disabled or its
	// pid/key settings are incomplete.
	ErrEasyPayNotConfigured = infraerrors.BadRequest("EASYPAY_NOT_CONFIGURED", "easypay gateway is disabled or not fully configured")
	// ErrEasyPayInvalidSign is returned when a notify signature does not match.
	ErrEasyPayInvalidSign = infraerrors.BadRequest("EASYPAY_INVALID_SIGN", "easypay notify signature mismatch")
	// ErrEasyPayNotifyFailed is returned when notify params are malformed
	// (missing required fields, pid mismatch, or unparseable money).
	ErrEasyPayNotifyFailed = infraerrors.BadRequest("EASYPAY_NOTIFY_FAILED", "easypay notify params are invalid")
	// ErrEasyPayUpstream wraps transport and response-decoding failures.
	ErrEasyPayUpstream = infraerrors.ServiceUnavailable("EASYPAY_UPSTREAM", "easypay upstream request failed")
)

// EasyPaySettingSource supplies the DB-backed EasyPay merchant configuration.
// It is resolved per call so runtime settings changes take effect immediately.
type EasyPaySettingSource interface {
	GetEasyPayConfig(ctx context.Context) (pid, key, apiBase string, enabled bool, err error)
}

// EasyPayClient implements Provider for the classic 彩虹易支付 MD5 protocol.
type EasyPayClient struct {
	settings   EasyPaySettingSource
	httpClient *http.Client
}

// NewEasyPayClient creates an EasyPay provider. pid/key are intentionally not
// baked in here; every operation loads the current DB settings.
func NewEasyPayClient(settingService EasyPaySettingSource) *EasyPayClient {
	return &EasyPayClient{
		settings:   settingService,
		httpClient: &http.Client{Timeout: easyPayHTTPTimeout},
	}
}

// Name implements Provider.
func (c *EasyPayClient) Name() string { return ProviderEasyPay }

type easyPayConfig struct {
	pid     string
	key     string
	apiBase string
}

func (c *EasyPayClient) loadConfig(ctx context.Context) (easyPayConfig, error) {
	pid, key, apiBase, enabled, err := c.settings.GetEasyPayConfig(ctx)
	if err != nil {
		return easyPayConfig{}, fmt.Errorf("load easypay config: %w", err)
	}
	pid = strings.TrimSpace(pid)
	key = strings.TrimSpace(key)
	if !enabled || pid == "" || key == "" {
		return easyPayConfig{}, ErrEasyPayNotConfigured
	}
	apiBase = strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if apiBase == "" {
		apiBase = defaultEasyPayAPIBase
	}
	return easyPayConfig{pid: pid, key: key, apiBase: apiBase}, nil
}

// easyPayMethod maps the internal method name to the provider's spelling.
func easyPayMethod(m Method) (string, error) {
	switch m {
	case MethodAlipay:
		return "alipay", nil
	case MethodWechat:
		return "wxpay", nil
	default:
		return "", infraerrors.BadRequest("EASYPAY_UNSUPPORTED_METHOD", "unsupported easypay method: "+string(m))
	}
}

// easyPayReverseMethod maps a notify "type" back to the internal method name.
// Unknown or empty types yield an empty method.
func easyPayReverseMethod(payType string) string {
	switch payType {
	case "alipay":
		return string(MethodAlipay)
	case "wxpay":
		return string(MethodWechat)
	default:
		return ""
	}
}

// easyPaySignString returns the sorted k=v&... string to sign (without the key).
// Empty values, sign, and sign_type are excluded. Values are not URL-encoded.
func easyPaySignString(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

// easyPaySign computes the MD5 (lowercase hex) of the sign string with the
// merchant key appended directly. Never log the input or the result.
func easyPaySign(params map[string]string, key string) string {
	sum := md5.Sum([]byte(easyPaySignString(params) + key))
	return hex.EncodeToString(sum[:])
}

// parseEasyPayMoneyFen converts a yuan string ("5.00") to fen, rounded to the
// nearest fen.
func parseEasyPayMoneyFen(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("money is required")
	}
	yuan, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("parse money: %w", err)
	}
	return int(math.Round(yuan * 100)), nil
}

// sanitizeEasyPayMsg caps an upstream error message for safe inclusion in our
// own errors. The merchant key is never part of upstream messages, and we never
// echo request params or signed strings.
func sanitizeEasyPayMsg(msg string) string {
	msg = strings.TrimSpace(msg)
	const maxLen = 120
	if len(msg) > maxLen {
		msg = msg[:maxLen] + "..."
	}
	return msg
}

type easyPayCreateResponse struct {
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	PayURL  string `json:"payurl"`
	QRCode  string `json:"qrcode"`
	TradeNo string `json:"trade_no"`
}

// CreateOrder implements Provider: POST {api_base}/xpay/epay/mapi.php with a
// signed form, returning the cashier URL and/or QR content.
func (c *EasyPayClient) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResult, error) {
	cfg, err := c.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	payType, err := easyPayMethod(req.Method)
	if err != nil {
		return nil, err
	}

	params := map[string]string{
		"pid":          cfg.pid,
		"type":         payType,
		"out_trade_no": req.OutTradeNo,
		"notify_url":   req.NotifyURL,
		"name":         req.Subject,
		"money":        fmt.Sprintf("%.2f", float64(req.AmountCNYFen)/100),
	}
	if req.ReturnURL != "" {
		params["return_url"] = req.ReturnURL
	}
	params["sign"] = easyPaySign(params, cfg.key)
	params["sign_type"] = easyPaySignType

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.apiBase+easyPayCreatePath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEasyPayUpstream, err)
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEasyPayUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, easyPayMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEasyPayUpstream, err)
	}

	var out easyPayCreateResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%w: decode create response: %v", ErrEasyPayUpstream, err)
	}
	if out.Code != 1 {
		return nil, infraerrors.BadRequest("EASYPAY_API_ERROR", "easypay create order failed: "+sanitizeEasyPayMsg(out.Msg))
	}
	return &CreateOrderResult{
		TradeNo:   strings.TrimSpace(out.TradeNo),
		PayURL:    strings.TrimSpace(out.PayURL),
		QRContent: strings.TrimSpace(out.QRCode),
	}, nil
}

// VerifyNotify implements Provider. The merchant backend can strip "name" from
// notify params, so it is never required. pid must match the configured
// merchant and money must be present and parseable; parse errors are
// verification failures.
func (c *EasyPayClient) VerifyNotify(params url.Values) (*NotifyResult, error) {
	// The Provider interface carries no context for notify verification; the
	// config read is a cheap DB lookup and does not depend on request scope.
	cfg, err := c.loadConfig(context.Background())
	if err != nil {
		return nil, err
	}

	flat := make(map[string]string, len(params))
	for k, vs := range params {
		if len(vs) > 0 {
			flat[k] = vs[0]
		}
	}

	outTradeNo := strings.TrimSpace(flat["out_trade_no"])
	tradeStatus := strings.TrimSpace(flat["trade_status"])
	if outTradeNo == "" || tradeStatus == "" {
		return nil, fmt.Errorf("%w: out_trade_no and trade_status are required", ErrEasyPayNotifyFailed)
	}
	if flat["pid"] != cfg.pid {
		return nil, fmt.Errorf("%w: pid mismatch", ErrEasyPayNotifyFailed)
	}
	receivedSign := strings.ToLower(strings.TrimSpace(flat["sign"]))
	if receivedSign == "" || easyPaySign(flat, cfg.key) != receivedSign {
		return nil, ErrEasyPayInvalidSign
	}
	amountFen, err := parseEasyPayMoneyFen(flat["money"])
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEasyPayNotifyFailed, err)
	}

	return &NotifyResult{
		OutTradeNo:   outTradeNo,
		TradeNo:      strings.TrimSpace(flat["trade_no"]),
		Method:       easyPayReverseMethod(strings.TrimSpace(flat["type"])),
		AmountCNYFen: amountFen,
		Paid:         tradeStatus == easyPayTradeSuccess,
	}, nil
}

// easyPayQueryResponse tolerates missing fields; money arrives as either a
// quoted string ("5.00") or a bare number depending on the deployment.
type easyPayQueryResponse struct {
	Code    int             `json:"code"`
	Msg     string          `json:"msg"`
	Status  int             `json:"status"`
	TradeNo string          `json:"trade_no"`
	Money   json.RawMessage `json:"money"`
}

// QueryOrder implements Provider: GET {api_base}/xpay/epay/api.php?act=order.
// The async notify is authoritative for crediting; this query exists only as a
// self-healing path when a notify was missed.
func (c *EasyPayClient) QueryOrder(ctx context.Context, outTradeNo string) (*QueryResult, error) {
	cfg, err := c.loadConfig(ctx)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Set("act", "order")
	q.Set("pid", cfg.pid)
	q.Set("key", cfg.key)
	q.Set("out_trade_no", outTradeNo)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.apiBase+easyPayQueryPath+"?"+q.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEasyPayUpstream, err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEasyPayUpstream, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, easyPayMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEasyPayUpstream, err)
	}

	var out easyPayQueryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("%w: decode query response: %v", ErrEasyPayUpstream, err)
	}
	if out.Code != 1 {
		return nil, infraerrors.BadRequest("EASYPAY_API_ERROR", "easypay query order failed: "+sanitizeEasyPayMsg(out.Msg))
	}

	result := &QueryResult{
		Paid:    out.Status == 1,
		TradeNo: strings.TrimSpace(out.TradeNo),
	}
	if money := easyPayRawMoneyString(out.Money); money != "" {
		// Query is a best-effort reconciliation aid; an unparseable amount
		// must not fail the lookup, so leave AmountCNYFen at zero.
		if fen, err := parseEasyPayMoneyFen(money); err == nil {
			result.AmountCNYFen = fen
		}
	}
	return result, nil
}

// easyPayRawMoneyString normalizes a money field that may be a JSON string or
// number into a plain string.
func easyPayRawMoneyString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	return strings.Trim(string(raw), " \t\r\n")
}

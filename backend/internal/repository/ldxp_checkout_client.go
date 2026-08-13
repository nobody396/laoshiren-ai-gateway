package repository

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"golang.org/x/net/html"
)

const (
	ldxpAPIBase              = "https://pay.ldxp.cn"
	ldxpMaxJSONResponseBytes = 1 << 20
	ldxpMaxHTMLBytes         = 1 << 20
	ldxpMaxImageBytes        = 2 << 20
)

var ldxpRedeemCodePattern = regexp.MustCompile(`(?i)(?:^|[^0-9a-f])([0-9a-f]{32})(?:$|[^0-9a-f])`)

type ldxpCheckoutClient struct {
	baseURL     *url.URL
	httpClient  *http.Client
	allowedHost string
	allowHTTP   bool
	userAgent   string
}

func NewLDXPCheckoutClient() service.NativeCheckoutProvider {
	client, err := newLDXPCheckoutClient(ldxpAPIBase, false)
	if err != nil {
		panic(err)
	}
	return client
}

func newLDXPCheckoutClient(baseURL string, allowHTTP bool) (*ldxpCheckoutClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Hostname() == "" {
		return nil, errors.New("invalid LDXP base URL")
	}
	client := &ldxpCheckoutClient{
		baseURL:     parsed,
		allowedHost: strings.ToLower(parsed.Hostname()),
		allowHTTP:   allowHTTP,
		userAgent:   "laoshirenai-native-checkout/1.0",
	}
	client.httpClient = &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			return client.validatePublicURL(req.URL)
		},
	}
	return client, nil
}

type ldxpEnvelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

func (c *ldxpCheckoutClient) CreateOrder(ctx context.Context, goodsKey, contact string, expectedAmountCNYFen int64) (*service.NativeCheckoutProviderOrder, error) {
	channel, err := c.checkoutChannel(ctx, goodsKey, expectedAmountCNYFen)
	if err != nil {
		return nil, &service.NativeCheckoutProviderError{Cause: err}
	}

	var created map[string]json.RawMessage
	err = c.postSuccess(ctx, "/shopApi/Pay/order", map[string]any{
		"goods_key":   goodsKey,
		"quantity":    1,
		"coupon_code": "",
		"channel_id":  channel.ID,
		"contact":     contact,
		"extend":      map[string]any{},
	}, &created, true)
	if err != nil {
		var requestErr *ldxpRequestError
		return nil, &service.NativeCheckoutProviderError{Ambiguous: errors.As(err, &requestErr) && requestErr.Ambiguous, Cause: err}
	}
	tradeNo := rawString(created, "trade_no", "order_no")
	paymentURL := rawString(created, "payurl", "pay_url", "url")
	createdAmountFen, amountErr := rawYuanToFen(created["total_amount"])
	parsedPaymentURL, err := url.Parse(paymentURL)
	if err != nil || amountErr != nil || createdAmountFen != expectedAmountCNYFen ||
		strings.TrimSpace(tradeNo) == "" || len(tradeNo) > 128 || c.validatePublicURL(parsedPaymentURL) != nil {
		return nil, &service.NativeCheckoutProviderError{Ambiguous: true, Cause: errors.New("invalid LDXP order response")}
	}
	return &service.NativeCheckoutProviderOrder{
		TradeNo:       tradeNo,
		PaymentURL:    parsedPaymentURL.String(),
		PaymentMethod: channel.PaymentMethod,
	}, nil
}

func (c *ldxpCheckoutClient) ValidateOffer(ctx context.Context, goodsKey string, expectedAmountCNYFen int64) error {
	_, err := c.checkoutChannel(ctx, goodsKey, expectedAmountCNYFen)
	return err
}

type ldxpCheckoutChannel struct {
	ID            int
	PaymentMethod string
}

func (c *ldxpCheckoutClient) checkoutChannel(ctx context.Context, goodsKey string, expectedAmountCNYFen int64) (ldxpCheckoutChannel, error) {
	var goods struct {
		GoodsType     string      `json:"goods_type"`
		GoodsKey      string      `json:"goods_key"`
		Status        int         `json:"status"`
		Price         json.Number `json:"price"`
		RealPrice     json.Number `json:"real_price"`
		ContactFormat string      `json:"contact_format"`
		User          struct {
			Token string `json:"token"`
		} `json:"user"`
	}
	if err := c.postSuccess(ctx, "/shopApi/Shop/goodsInfo", map[string]any{"goods_key": goodsKey}, &goods, false); err != nil {
		return ldxpCheckoutChannel{}, err
	}
	priceFen, err := yuanNumberToFen(firstNonEmptyNumber(goods.RealPrice, goods.Price))
	if err != nil || goods.GoodsKey != goodsKey || goods.GoodsType != "card" || goods.Status != 1 ||
		goods.ContactFormat != "email" || strings.TrimSpace(goods.User.Token) == "" || priceFen != expectedAmountCNYFen {
		return ldxpCheckoutChannel{}, errors.New("LDXP goods validation failed")
	}

	var channels []struct {
		ID           int    `json:"id"`
		Code         string `json:"code"`
		Status       int    `json:"status"`
		CustomStatus int    `json:"custom_status"`
	}
	if err := c.postSuccess(ctx, "/shopApi/Shop/getUserChannel", map[string]any{"token": goods.User.Token}, &channels, false); err != nil {
		return ldxpCheckoutChannel{}, err
	}
	for _, channel := range channels {
		if channel.ID <= 0 || channel.Status != 1 || channel.CustomStatus != 1 {
			continue
		}
		if paymentMethod, ok := ldxpPaymentMethod(channel.Code); ok {
			// Respect the provider's active-channel order. The exact selected method
			// is persisted with the order so the customer prompt cannot drift if the
			// merchant later switches between WeChat Pay and Alipay.
			return ldxpCheckoutChannel{ID: channel.ID, PaymentMethod: paymentMethod}, nil
		}
	}
	return ldxpCheckoutChannel{}, errors.New("LDXP supported QR payment channel is unavailable")
}

func ldxpPaymentMethod(channelCode string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(channelCode)) {
	case "weixinnative":
		return service.NativeCheckoutPaymentMethodWeChat, true
	case "alipay":
		return service.NativeCheckoutPaymentMethodAlipay, true
	default:
		return "", false
	}
}

func (c *ldxpCheckoutClient) IsPaid(ctx context.Context, tradeNo string) (bool, error) {
	envelope, err := c.post(ctx, "/shopApi/Pay/query", map[string]any{"trade_no": tradeNo}, false)
	if err != nil {
		return false, err
	}
	return envelope.Code == 1, nil
}

func (c *ldxpCheckoutClient) GetOrderInfo(ctx context.Context, tradeNo string) (*service.NativeCheckoutProviderOrderInfo, error) {
	var data map[string]json.RawMessage
	if err := c.postSuccess(ctx, "/shopApi/Order/info", map[string]any{"trade_no": tradeNo, "dump": 1}, &data, false); err != nil {
		return nil, err
	}
	goodsKey := ""
	if raw := data["goods"]; len(raw) > 0 {
		var goods map[string]json.RawMessage
		if json.Unmarshal(raw, &goods) == nil {
			goodsKey = rawString(goods, "goods_key")
		} else {
			_ = json.Unmarshal(raw, &goodsKey)
		}
	}
	if goodsKey == "" {
		goodsKey = rawString(data, "goods_key")
	}
	amountFen, err := rawYuanToFen(data["total_amount"])
	if err != nil {
		return nil, errors.New("invalid LDXP order amount")
	}
	quantity, err := rawInt(data["quantity"])
	if err != nil {
		return nil, errors.New("invalid LDXP order quantity")
	}
	status, _ := rawInt(data["status"])
	sendout, _ := rawInt(data["sendout"])
	return &service.NativeCheckoutProviderOrderInfo{
		TradeNo:     rawString(data, "trade_no"),
		GoodsKey:    goodsKey,
		Contact:     rawString(data, "contact"),
		Quantity:    quantity,
		TotalCNYFen: amountFen,
		Paid:        status == 1,
		Delivered:   sendout == 1,
		RedeemCodes: extractLDXPRedeemCodes(data),
	}, nil
}

func (c *ldxpCheckoutClient) FetchDirectPaymentQR(ctx context.Context, paymentURL string) ([]byte, string, error) {
	pageURL, err := url.Parse(paymentURL)
	if err != nil || c.validatePublicURL(pageURL) != nil {
		return nil, "", errors.New("invalid LDXP payment URL")
	}
	body, contentType, finalURL, err := c.getLimited(ctx, pageURL, paymentURL, ldxpMaxHTMLBytes)
	if err != nil {
		return nil, "", err
	}
	if imageType := detectedImageType(body); imageType != "" {
		return body, imageType, nil
	}
	if !strings.Contains(strings.ToLower(contentType), "html") {
		return nil, "", errors.New("LDXP payment page did not return HTML")
	}
	src := findLDXPQRImageSource(body)
	if src == "" {
		return nil, "", errors.New("LDXP direct payment QR is unavailable")
	}
	if strings.HasPrefix(strings.ToLower(src), "data:image/") {
		imageBody, err := decodeImageDataURL(src)
		if err != nil {
			return nil, "", err
		}
		imageType := detectedImageType(imageBody)
		if imageType == "" {
			return nil, "", errors.New("invalid embedded LDXP QR image")
		}
		return imageBody, imageType, nil
	}
	qrURL, err := finalURL.Parse(src)
	if err != nil || c.validatePublicURL(qrURL) != nil {
		return nil, "", errors.New("invalid LDXP QR image URL")
	}
	imageBody, _, _, err := c.getLimited(ctx, qrURL, finalURL.String(), ldxpMaxImageBytes)
	if err != nil {
		return nil, "", err
	}
	imageType := detectedImageType(imageBody)
	if imageType == "" {
		return nil, "", errors.New("LDXP QR response is not an image")
	}
	return imageBody, imageType, nil
}

type ldxpRequestError struct {
	Ambiguous bool
	Cause     error
}

func (e *ldxpRequestError) Error() string { return e.Cause.Error() }
func (e *ldxpRequestError) Unwrap() error { return e.Cause }

func (c *ldxpCheckoutClient) postSuccess(ctx context.Context, path string, payload, target any, createRequest bool) error {
	envelope, err := c.post(ctx, path, payload, createRequest)
	if err != nil {
		return err
	}
	if envelope.Code != 1 {
		return errors.New("LDXP rejected request")
	}
	if target != nil && len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, target); err != nil {
			responseErr := errors.New("invalid LDXP response data")
			if createRequest {
				return &ldxpRequestError{Ambiguous: true, Cause: responseErr}
			}
			return responseErr
		}
	}
	return nil
}

func (c *ldxpCheckoutClient) post(ctx context.Context, path string, payload any, createRequest bool) (*ldxpEnvelope, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: path})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, &ldxpRequestError{Ambiguous: createRequest, Cause: err}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := readLimited(resp.Body, ldxpMaxJSONResponseBytes)
	if err != nil {
		return nil, &ldxpRequestError{Ambiguous: createRequest, Cause: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ldxpRequestError{Ambiguous: createRequest && resp.StatusCode >= 500, Cause: fmt.Errorf("LDXP HTTP status %d", resp.StatusCode)}
	}
	var envelope ldxpEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, &ldxpRequestError{Ambiguous: createRequest, Cause: errors.New("invalid LDXP JSON response")}
	}
	return &envelope, nil
}

func (c *ldxpCheckoutClient) getLimited(ctx context.Context, target *url.URL, referer string, maxBytes int64) ([]byte, string, *url.URL, error) {
	if err := c.validatePublicURL(target); err != nil {
		return nil, "", nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "", nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "text/html,image/png,image/jpeg,image/gif,image/webp;q=0.9,*/*;q=0.5")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", nil, fmt.Errorf("LDXP HTTP status %d", resp.StatusCode)
	}
	if err := c.validatePublicURL(resp.Request.URL); err != nil {
		return nil, "", nil, err
	}
	body, err := readLimited(resp.Body, maxBytes)
	if err != nil {
		return nil, "", nil, err
	}
	return body, resp.Header.Get("Content-Type"), resp.Request.URL, nil
}

func (c *ldxpCheckoutClient) validatePublicURL(target *url.URL) error {
	if target == nil || target.User != nil || (target.Port() != "" && !c.allowHTTP) || strings.ToLower(target.Hostname()) != c.allowedHost {
		return errors.New("LDXP URL host is not allowed")
	}
	scheme := strings.ToLower(target.Scheme)
	validScheme := scheme == "https" || (c.allowHTTP && scheme == "http")
	if !validScheme {
		return errors.New("LDXP URL scheme is not allowed")
	}
	return nil
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, errors.New("LDXP response exceeds size limit")
	}
	return body, nil
}

func findLDXPQRImageSource(body []byte) string {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return ""
	}
	var visit func(*html.Node) string
	visit = func(node *html.Node) string {
		if node.Type == html.ElementNode && strings.EqualFold(node.Data, "img") {
			var className, src string
			for _, attr := range node.Attr {
				switch strings.ToLower(attr.Key) {
				case "class":
					className = attr.Val
				case "src":
					src = strings.TrimSpace(attr.Val)
				}
			}
			if src != "" && (containsClass(className, "code") || strings.Contains(strings.ToLower(src), "generateqrcode")) {
				return src
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if result := visit(child); result != "" {
				return result
			}
		}
		return ""
	}
	return visit(doc)
}

func containsClass(value, expected string) bool {
	for _, item := range strings.Fields(value) {
		if strings.EqualFold(item, expected) {
			return true
		}
	}
	return false
}

func detectedImageType(body []byte) string {
	switch {
	case len(body) >= 8 && bytes.Equal(body[:8], []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case len(body) >= 3 && bytes.Equal(body[:3], []byte("\xff\xd8\xff")):
		return "image/jpeg"
	case len(body) >= 6 && (bytes.Equal(body[:6], []byte("GIF87a")) || bytes.Equal(body[:6], []byte("GIF89a"))):
		return "image/gif"
	case len(body) >= 12 && bytes.Equal(body[:4], []byte("RIFF")) && bytes.Equal(body[8:12], []byte("WEBP")):
		return "image/webp"
	default:
		return ""
	}
}

func decodeImageDataURL(value string) ([]byte, error) {
	comma := strings.IndexByte(value, ',')
	if comma <= 0 || !strings.Contains(strings.ToLower(value[:comma]), ";base64") {
		return nil, errors.New("unsupported embedded LDXP QR image")
	}
	body, err := base64.StdEncoding.DecodeString(value[comma+1:])
	if err != nil || len(body) > ldxpMaxImageBytes {
		return nil, errors.New("invalid embedded LDXP QR image")
	}
	return body, nil
}

func extractLDXPRedeemCodes(data map[string]json.RawMessage) []string {
	seen := make(map[string]struct{})
	codes := make([]string, 0, 1)
	for _, key := range []string{"cards", "response", "buyer_value"} {
		collectLDXPCodes(data[key], seen, &codes)
	}
	return codes
}

func collectLDXPCodes(raw json.RawMessage, seen map[string]struct{}, result *[]string) {
	if len(raw) == 0 || string(raw) == "null" {
		return
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if decoder.Decode(&value) != nil {
		return
	}
	var walk func(any)
	walk = func(current any) {
		switch typed := current.(type) {
		case string:
			for _, match := range ldxpRedeemCodePattern.FindAllStringSubmatch(typed, -1) {
				code := strings.ToLower(match[1])
				if _, ok := seen[code]; !ok {
					seen[code] = struct{}{}
					*result = append(*result, code)
				}
			}
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
}

func rawString(values map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		var value string
		if raw := values[key]; len(raw) > 0 && json.Unmarshal(raw, &value) == nil && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func rawInt(raw json.RawMessage) (int, error) {
	if len(raw) == 0 {
		return 0, errors.New("missing integer")
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		value, err := strconv.Atoi(number.String())
		if err == nil {
			return value, nil
		}
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return strconv.Atoi(text)
	}
	return 0, errors.New("invalid integer")
}

func rawYuanToFen(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 {
		return 0, errors.New("missing amount")
	}
	var number json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&number); err == nil {
		return yuanNumberToFen(number)
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return yuanStringToFen(text)
	}
	return 0, errors.New("invalid amount")
}

func yuanNumberToFen(number json.Number) (int64, error) { return yuanStringToFen(number.String()) }

func yuanStringToFen(value string) (int64, error) {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || parsed < 0 {
		return 0, errors.New("invalid amount")
	}
	fen := math.Round(parsed * 100)
	if math.Abs(parsed*100-fen) > 0.000001 || fen > math.MaxInt64 {
		return 0, errors.New("amount has invalid precision")
	}
	return int64(fen), nil
}

func firstNonEmptyNumber(values ...json.Number) json.Number {
	for _, value := range values {
		if strings.TrimSpace(value.String()) != "" {
			return value
		}
	}
	return ""
}

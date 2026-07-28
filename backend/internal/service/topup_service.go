package service

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const (
	xunhuPayURL          = "https://api.dpweixin.com/payment/do.html"
	xunhuQueryURL        = "https://api.dpweixin.com/payment/query.html"
	xunhuVersion         = "1.1"
	legacyXunhuNotifyURL = "https://your-domain.example/api/v1/topup/notify"

	// 充值成功后写入余额的汇率：1 CNY = 1 USD（1:1 固定）
	topupCNYFenToUSD = 1.0 / 100.0
)

// TopupService 处理虎皮椒充值业务
type TopupService struct {
	topupRepo            TopupOrderRepository
	settingService       *SettingService
	userRepo             UserRepository
	accountChangeRepo    AccountChangeRecordRepository
	entClient            *dbent.Client
	billingCache         *BillingCacheService
	authInvalidator      APIKeyAuthCacheInvalidator
	commissionService    *CommissionService
	balanceAlertService  *BalanceAlertService
	affiliateConsumption AffiliateConsumptionRepository
	affiliateRewards     *AffiliateRewardService
}

// NewTopupService creates a new TopupService
func NewTopupService(
	topupRepo TopupOrderRepository,
	settingService *SettingService,
	userRepo UserRepository,
	accountChangeRepo AccountChangeRecordRepository,
	entClient *dbent.Client,
	billingCache *BillingCacheService,
	authInvalidator APIKeyAuthCacheInvalidator,
	commissionService *CommissionService,
	balanceAlertService *BalanceAlertService,
	affiliateConsumption AffiliateConsumptionRepository,
	affiliateRewards *AffiliateRewardService,
) *TopupService {
	return &TopupService{
		topupRepo:            topupRepo,
		settingService:       settingService,
		userRepo:             userRepo,
		accountChangeRepo:    accountChangeRepo,
		entClient:            entClient,
		billingCache:         billingCache,
		authInvalidator:      authInvalidator,
		commissionService:    commissionService,
		balanceAlertService:  balanceAlertService,
		affiliateConsumption: affiliateConsumption,
		affiliateRewards:     affiliateRewards,
	}
}

// xunhuResponse 虎皮椒创建订单响应
type xunhuResponse struct {
	ErrCode     any    `json:"errcode"` // 0 表示成功
	ErrMsg      string `json:"errmsg"`
	Hash        string `json:"hash"`
	URLQRCode   string `json:"url_qrcode"` // 二维码图片 URL
	URL         string `json:"url"`
	OpenOrderID string `json:"open_order_id"` // 虎皮椒内部订单号
}

// xunhuQueryResponse 虎皮椒查询订单响应
type xunhuQueryResponse struct {
	ErrCode     any                 `json:"errcode"`
	ErrMsg      string              `json:"errmsg"`
	Hash        string              `json:"hash"`
	Status      string              `json:"status"`        // 兼容部分扁平响应
	OpenOrderID string              `json:"open_order_id"` // 兼容部分扁平响应
	Data        xunhuQueryOrderData `json:"data"`
}

type xunhuQueryOrderData struct {
	Status      string `json:"status"`        // OD=成功, WP=待支付, CD=已取消
	OpenOrderID string `json:"open_order_id"` // 虎皮椒内部订单号
}

func xunhuChannelCodePrefix(payType string) string {
	if payType == "wechat" {
		return "XUNHU_WECHAT"
	}
	return "XUNHU_ALIPAY"
}

func xunhuChannelName(payType string) string {
	if payType == "wechat" {
		return "wechat"
	}
	return "alipay"
}

func validateXunhuChannelConfig(payType, appID, key string, enabled bool) error {
	channelName := xunhuChannelName(payType)
	codePrefix := xunhuChannelCodePrefix(payType)
	if !enabled {
		return infraerrors.BadRequest(codePrefix+"_DISABLED", "xunhu "+channelName+" channel is disabled")
	}
	if strings.TrimSpace(appID) == "" {
		return infraerrors.BadRequest(codePrefix+"_APPID_MISSING", "xunhu "+channelName+" appid is not configured")
	}
	if strings.TrimSpace(key) == "" {
		return infraerrors.BadRequest(codePrefix+"_KEY_MISSING", "xunhu "+channelName+" key is not configured")
	}
	return nil
}

func (s *TopupService) resolveTopupBaseURL(ctx context.Context, notifyURL string) string {
	if frontendURL := strings.TrimSpace(s.settingService.GetFrontendURL(ctx)); frontendURL != "" {
		return strings.TrimRight(frontendURL, "/")
	}

	parsed, err := url.Parse(strings.TrimSpace(notifyURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}

	return strings.TrimRight((&url.URL{Scheme: parsed.Scheme, Host: parsed.Host}).String(), "/")
}

func (s *TopupService) resolveXunhuNotifyURL(ctx context.Context, notifyURL string) string {
	trimmed := strings.TrimSpace(notifyURL)
	if baseURL := s.resolveTopupBaseURL(ctx, trimmed); baseURL != "" {
		if trimmed == "" || strings.EqualFold(trimmed, legacyXunhuNotifyURL) {
			return baseURL + "/api/v1/topup/notify"
		}
	}
	return trimmed
}

func (s *TopupService) resolveXunhuReturnURL(ctx context.Context, notifyURL string) string {
	if baseURL := s.resolveTopupBaseURL(ctx, notifyURL); baseURL != "" {
		return baseURL + "/dashboard"
	}
	return ""
}

// CreateTopupOrder 创建充值订单，返回订单号和二维码 URL
func (s *TopupService) CreateTopupOrder(ctx context.Context, userID int64, amountCNYFen int, payType string) (orderNo, qrCodeURL string, err error) {
	// 参数校验
	if amountCNYFen < TopupMinAmountFen {
		return "", "", ErrTopupMinAmount
	}
	if amountCNYFen > TopupMaxAmountFen {
		return "", "", ErrTopupMaxAmount
	}
	if payType != "alipay" && payType != "wechat" {
		return "", "", ErrTopupInvalidType
	}

	// 读取虎皮椒配置
	appID, key, notifyURL, enabled, err := s.settingService.GetXunhuConfig(ctx, payType)
	if err != nil {
		return "", "", fmt.Errorf("get xunhu config: %w", err)
	}
	if err := validateXunhuChannelConfig(payType, appID, key, enabled); err != nil {
		return "", "", err
	}
	notifyURL = s.resolveXunhuNotifyURL(ctx, notifyURL)
	if notifyURL == "" {
		return "", "", infraerrors.BadRequest("XUNHU_NOTIFY_URL_MISSING", "xunhu notify url is not configured")
	}

	// 生成订单号（TP + userID 左填充5位 + Unix 毫秒13位 = ≤20 位，满足虎皮椒 ≤32 位限制）
	orderNo = fmt.Sprintf("TP%05d%013d", userID%100000, time.Now().UnixMilli())

	// 写入 DB（pending 状态）
	order := &TopupOrder{
		OrderNo:      orderNo,
		UserID:       userID,
		AmountCNYFen: amountCNYFen,
		PayType:      payType,
		Status:       TopupStatusPending,
	}
	if err := s.topupRepo.Create(ctx, order); err != nil {
		return "", "", fmt.Errorf("create topup order: %w", err)
	}

	// 构造虎皮椒请求参数
	totalFee := fmt.Sprintf("%.2f", float64(amountCNYFen)/100.0)
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := generateNonceStr(16)
	returnURL := s.resolveXunhuReturnURL(ctx, notifyURL)
	if returnURL == "" {
		return "", "", infraerrors.BadRequest("XUNHU_RETURN_URL_MISSING", "xunhu return url is not configured")
	}

	params := map[string]string{
		"version":        xunhuVersion,
		"appid":          appID,
		"trade_order_id": orderNo,
		"total_fee":      totalFee,
		"title":          "Dragon Code 余额充值",
		"time":           ts,
		"notify_url":     notifyURL,
		"return_url":     returnURL,
		"nonce_str":      nonce,
		"type":           payType,
	}
	signStr := buildXunhuSignString(params)
	slog.Info("xunhu topup debug",
		"appid", appID,
		"key_len", len(key),
		"notify_url", notifyURL,
		"return_url", returnURL,
		"sign_str", signStr,
	)
	params["hash"] = calcXunhuHash(params, key)

	// 发起请求
	resp, err := postXunhu(xunhuPayURL, params)
	if err != nil {
		return "", "", fmt.Errorf("call xunhu api: %w", err)
	}

	if fmt.Sprintf("%v", resp.ErrCode) != "0" {
		return "", "", infraerrors.BadRequest("XUNHU_API_ERROR", resp.ErrMsg)
	}

	// 保存二维码 URL
	_ = s.topupRepo.UpdateQRCodeURL(ctx, order.ID, resp.URLQRCode)

	return orderNo, resp.URLQRCode, nil
}

// HandleNotify 处理虎皮椒异步回调
// 返回 "success" 或 "fail"（虎皮椒要求纯文本响应）
func (s *TopupService) HandleNotify(ctx context.Context, form url.Values) error {
	params := make(map[string]string)
	for k, vs := range form {
		if len(vs) > 0 {
			params[k] = vs[0]
		}
	}

	orderNo := params["trade_order_id"]
	if orderNo == "" {
		return infraerrors.BadRequest("XUNHU_INVALID_NOTIFY", "trade_order_id is required")
	}

	order, err := s.topupRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return fmt.Errorf("get topup order: %w", err)
	}

	// 1. 读取对应渠道配置并验签
	appID, key, _, enabled, err := s.settingService.GetXunhuConfig(ctx, order.PayType)
	if err != nil {
		return fmt.Errorf("get xunhu config: %w", err)
	}
	if !enabled || appID == "" || key == "" {
		return ErrXunhuNotConfigured
	}
	if callbackAppID := params["appid"]; callbackAppID != "" && callbackAppID != appID {
		return infraerrors.BadRequest("XUNHU_INVALID_APPID", "callback appid does not match order channel")
	}

	if !verifyXunhuHash(params, key) {
		return infraerrors.BadRequest("XUNHU_INVALID_HASH", "invalid hash signature")
	}

	// 2. 仅处理支付成功状态
	if params["status"] != "OD" {
		return nil // 非成功状态忽略，回调返回 success
	}

	// 3. 验证回调金额与订单金额一致
	if callbackFee := params["total_fee"]; callbackFee != "" {
		expectedFee := fmt.Sprintf("%.2f", float64(order.AmountCNYFen)/100.0)
		if callbackFee != expectedFee {
			return infraerrors.BadRequest("XUNHU_AMOUNT_MISMATCH", "callback amount does not match order")
		}
	}

	xunhuTradeNo := params["open_order_id"]

	return s.completeOrder(ctx, orderNo, order, &xunhuTradeNo)
}

// QueryOrderStatus 查询订单状态（先查本地，pending 时再查虎皮椒）
func (s *TopupService) QueryOrderStatus(ctx context.Context, orderNo string, userID int64) (*TopupOrder, error) {
	order, err := s.topupRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, ErrTopupNotFound
	}

	if order.Status == TopupStatusPending {
		// 查虎皮椒接口
		appID, key, _, _, err := s.settingService.GetXunhuConfig(ctx, order.PayType)
		if err == nil && appID != "" && key != "" {
			queryStatus, xunhuTradeNo, qErr := s.queryXunhu(ctx, appID, key, orderNo)
			if qErr == nil {
				switch queryStatus {
				case "OD":
					if cErr := s.completeOrder(ctx, orderNo, order, &xunhuTradeNo); cErr == nil {
						order, _ = s.topupRepo.GetByOrderNo(ctx, orderNo)
					}
				case "CD":
					_ = s.topupRepo.UpdateStatus(ctx, order.ID, TopupStatusExpired, nil)
					order.Status = TopupStatusExpired
				}
			}
		}
	}

	return order, nil
}

// completeOrder 幂等完成订单：写余额、写流水、触发首充奖励
func (s *TopupService) completeOrder(ctx context.Context, orderNo string, order *TopupOrder, xunhuTradeNo *string) error {
	// 充值金额换算（1 CNY = 1 USD，单位：分 → USD）
	amountUSD := float64(order.AmountCNYFen) * topupCNYFenToUSD
	affiliateV3Active := false

	// 开事务
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)

	// 原子标记订单完成（仅当状态为 pending 时生效，防止并发双重入账）
	done, err := s.topupRepo.CompleteIfPending(txCtx, order.ID, xunhuTradeNo)
	if err != nil {
		return fmt.Errorf("complete topup order: %w", err)
	}
	if !done {
		// 已被其他请求处理，幂等返回
		return nil
	}

	// 增加余额
	if err := s.userRepo.UpdateBalance(txCtx, order.UserID, amountUSD); err != nil {
		return fmt.Errorf("update user balance: %w", err)
	}
	if s.affiliateConsumption != nil {
		occurredAt := time.Now()
		var rewardResult *AffiliateFirstPaidPurchaseResult
		if s.affiliateRewards != nil {
			rewardResult, err = s.affiliateRewards.ProcessFirstPaidPurchase(txCtx, AffiliateFirstPaidPurchaseInput{
				UserID:       order.UserID,
				PurchaseType: AffiliatePurchaseBalanceTopup,
				SourceID:     order.ID,
				PurchaseKey:  fmt.Sprintf("topup:balance:%d", order.ID),
				AmountMicros: int64(order.AmountCNYFen) * 10_000,
				OccurredAt:   occurredAt,
			})
			if err != nil {
				return fmt.Errorf("process affiliate first paid topup: %w", err)
			}
			affiliateV3Active = AffiliateProgramHandlesPurchase(rewardResult)
		}
		policy, partnerID, customerRate, partnerRate := AffiliatePolicyFromPurchaseResult(
			order.AmountCNYFen > 0,
			rewardResult,
		)
		if err := s.affiliateConsumption.RecordBalanceLot(txCtx, AffiliateBalanceLotInput{
			UserID:                   order.UserID,
			SourceType:               AffiliateSourcePaidTopup,
			SourceID:                 order.ID,
			SourceKey:                fmt.Sprintf("topup:balance:%d", order.ID),
			AmountMicros:             int64(order.AmountCNYFen) * 10_000,
			AffiliatePolicy:          policy,
			DirectPartnerID:          partnerID,
			CustomerRebateRateBPS:    customerRate,
			PartnerCommissionRateBPS: partnerRate,
			OccurredAt:               occurredAt,
		}); err != nil {
			return fmt.Errorf("record affiliate balance lot: %w", err)
		}
	}

	if err := tx.User.UpdateOneID(order.UserID).AddTotalRecharged(amountUSD).Exec(txCtx); err != nil {
		return fmt.Errorf("update total recharged: %w", err)
	}

	if s.accountChangeRepo != nil {
		now := time.Now()
		record := &AccountChangeRecord{
			UserID:      order.UserID,
			AssetType:   AccountChangeAssetBalance,
			Reason:      AccountChangeReasonTopup,
			Delta:       amountUSD,
			SourceType:  AccountChangeSourceTopupOrder,
			SourceID:    &order.ID,
			ReferenceNo: orderNo,
			Notes:       fmt.Sprintf("充值到账，订单号: %s", orderNo),
			CreatedAt:   now,
			DedupeKey:   ptrString(fmt.Sprintf("topup_order:%d", order.ID)),
		}
		if err := s.accountChangeRepo.Create(txCtx, record); err != nil {
			return fmt.Errorf("create account change record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// 事务提交后：失效缓存
	if s.authInvalidator != nil {
		s.authInvalidator.InvalidateAuthCacheByUserID(ctx, order.UserID)
	}
	if s.billingCache != nil {
		go func() {
			cCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.billingCache.InvalidateUserBalance(cCtx, order.UserID)
		}()
	}

	if s.balanceAlertService != nil {
		go func() {
			resetCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			user, err := s.userRepo.GetByID(resetCtx, order.UserID)
			if err != nil {
				return
			}
			s.balanceAlertService.ResetNotifiedFlag(resetCtx, order.UserID, user.Balance)
		}()
	}

	// 异步触发“被邀请用户首次虎皮椒充值”奖励（幂等，不影响主流程）
	if s.commissionService != nil && !affiliateV3Active {
		uid := order.UserID
		topupOrderID := order.ID
		go func() {
			bCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.commissionService.ProcessFirstInvitedTopupBonus(bCtx, uid, amountUSD, topupOrderID); err != nil {
				fmt.Printf("[TopupService] ProcessFirstInvitedTopupBonus failed: user_id=%d err=%v\n", uid, err)
			}
		}()
	}

	return nil
}

// queryXunhu 调用虎皮椒查询接口，返回 status 和 open_order_id
func (s *TopupService) queryXunhu(_ context.Context, appID, key, orderNo string) (status, openOrderID string, err error) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	nonce := generateNonceStr(16)
	params := map[string]string{
		"appid":           appID,
		"out_trade_order": orderNo,
		"time":            ts,
		"nonce_str":       nonce,
	}
	params["hash"] = calcXunhuHash(params, key)

	var qResp xunhuQueryResponse
	body, err := postFormURLEncoded(xunhuQueryURL, params)
	if err != nil {
		return "", "", err
	}
	if err := json.Unmarshal(body, &qResp); err != nil {
		return "", "", err
	}
	if fmt.Sprintf("%v", qResp.ErrCode) != "0" {
		return "", "", fmt.Errorf("xunhu query error: %s", qResp.ErrMsg)
	}

	status = qResp.Data.Status
	if status == "" {
		status = qResp.Status
	}

	openOrderID = qResp.Data.OpenOrderID
	if openOrderID == "" {
		openOrderID = qResp.OpenOrderID
	}

	return status, openOrderID, nil
}

// --- 签名工具函数 ---

// buildXunhuSignString 返回排序后待签名字符串（不含 &key=<secret>），供调试使用
func buildXunhuSignString(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k != "hash" && params[k] != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	return strings.Join(parts, "&")
}

// calcXunhuHash 计算虎皮椒签名（MD5 小写 32 位）
// 规则：非空参数按字段名字母序排列，拼接 key=value&...，直接追加<密钥>（无分隔符），MD5 小写
func calcXunhuHash(params map[string]string, secret string) string {
	signStr := buildXunhuSignString(params)
	signStr += secret

	h := md5.New()
	_, _ = io.WriteString(h, signStr)
	return fmt.Sprintf("%x", h.Sum(nil)) // 小写 32 位
}

// verifyXunhuHash 验证回调签名
func verifyXunhuHash(params map[string]string, secret string) bool {
	received := params["hash"]
	if received == "" {
		return false
	}
	computed := calcXunhuHash(params, secret)
	return computed == strings.ToLower(received)
}

// generateNonceStr 生成指定长度的随机字符串
func generateNonceStr(n int) string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	randomBytes := make([]byte, n)
	if _, err := rand.Read(randomBytes); err != nil {
		src := time.Now().UnixNano()
		for i := range b {
			b[i] = chars[(src>>uint(i*3))%int64(len(chars))]
		}
		return string(b)
	}
	for i, v := range randomBytes {
		b[i] = chars[int(v)%len(chars)]
	}
	return string(b)
}

// postXunhu 向虎皮椒发起 POST 请求并解析响应
func postXunhu(apiURL string, params map[string]string) (*xunhuResponse, error) {
	body, err := postFormURLEncoded(apiURL, params)
	if err != nil {
		return nil, err
	}
	var resp xunhuResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse xunhu response: %w, body: %s", err, string(body))
	}
	return &resp, nil
}

// postFormURLEncoded 发起 form-urlencoded POST 请求，返回响应体
func postFormURLEncoded(apiURL string, params map[string]string) ([]byte, error) {
	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	resp, err := http.PostForm(apiURL, form)
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return body, nil
}

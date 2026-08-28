package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/smartwalle/alipay/v3"
)

// PaymentService handles Alipay Face-to-Face payment logic
type PaymentService struct {
	paymentRepo          PaymentOrderRepository
	subscriptionService  *SubscriptionService
	settingService       *SettingService
	entClient            *dbent.Client
	affiliateConsumption AffiliateConsumptionRepository
	affiliateRewards     *AffiliateRewardService
}

// NewPaymentService creates a new PaymentService
func NewPaymentService(
	paymentRepo PaymentOrderRepository,
	subscriptionService *SubscriptionService,
	settingService *SettingService,
	entClient *dbent.Client,
	affiliateConsumption AffiliateConsumptionRepository,
	affiliateRewards *AffiliateRewardService,
) *PaymentService {
	return &PaymentService{
		paymentRepo:          paymentRepo,
		subscriptionService:  subscriptionService,
		settingService:       settingService,
		entClient:            entClient,
		affiliateConsumption: affiliateConsumption,
		affiliateRewards:     affiliateRewards,
	}
}

// GetPlans returns the configured subscription plans
func (s *PaymentService) GetPlans(ctx context.Context) ([]SubscriptionPlan, error) {
	raw, err := s.settingService.GetAlipayPlansRaw(ctx)
	if err != nil || raw == "" {
		return nil, nil
	}

	var plans []SubscriptionPlan
	if err := json.Unmarshal([]byte(raw), &plans); err != nil {
		return nil, nil
	}
	return plans, nil
}

// getAlipayClient creates an Alipay client from settings
func (s *PaymentService) getAlipayClient(ctx context.Context) (*alipay.Client, error) {
	appID, privateKey, alipayPublicKey, _, enabled, err := s.settingService.GetAlipayConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled || appID == "" || privateKey == "" {
		return nil, ErrAlipayNotConfigured
	}

	client, err := alipay.New(appID, privateKey, false)
	if err != nil {
		return nil, fmt.Errorf("create alipay client: %w", err)
	}

	if err := client.LoadAliPayPublicKey(alipayPublicKey); err != nil {
		return nil, fmt.Errorf("load alipay public key: %w", err)
	}

	return client, nil
}

// CreateOrder creates a new payment order and generates an Alipay QR code
func (s *PaymentService) CreateOrder(ctx context.Context, userID int64, planID string) (orderNo, qrCodeURL string, err error) {
	// Validate plan
	plans, err := s.GetPlans(ctx)
	if err != nil {
		return "", "", err
	}
	var matchedPlan *SubscriptionPlan
	for i := range plans {
		if plans[i].ID == planID {
			matchedPlan = &plans[i]
			break
		}
	}
	if matchedPlan == nil {
		return "", "", ErrInvalidPlan
	}

	// Get Alipay client
	client, err := s.getAlipayClient(ctx)
	if err != nil {
		return "", "", err
	}

	// Get notify URL
	_, _, _, notifyURL, _, _ := s.settingService.GetAlipayConfig(ctx)

	// Generate order number
	orderNo = fmt.Sprintf("PAY-%d-%d", userID, time.Now().UnixMilli())

	// Create DB record
	order := &PaymentOrder{
		OrderNo:      orderNo,
		UserID:       userID,
		AmountCents:  matchedPlan.PriceCents,
		PlanID:       matchedPlan.ID,
		GroupID:      matchedPlan.GroupID,
		ValidityDays: matchedPlan.ValidityDays,
		Status:       PaymentStatusPending,
	}
	if err := s.paymentRepo.Create(ctx, order); err != nil {
		return "", "", fmt.Errorf("create payment order: %w", err)
	}

	// Call Alipay TradePreCreate
	totalAmount := fmt.Sprintf("%.2f", float64(matchedPlan.PriceCents)/100.0)
	req := alipay.TradePreCreate{
		Trade: alipay.Trade{
			Subject:     matchedPlan.Name,
			OutTradeNo:  orderNo,
			TotalAmount: totalAmount,
		},
	}
	if notifyURL != "" {
		req.NotifyURL = notifyURL
	}

	resp, err := client.TradePreCreate(ctx, req)
	if err != nil {
		return "", "", fmt.Errorf("alipay trade precreate: %w", err)
	}
	if !resp.IsSuccess() {
		return "", "", fmt.Errorf("alipay trade precreate failed: %s %s", resp.Msg, resp.SubMsg)
	}

	// Store QR code URL
	_ = s.paymentRepo.UpdateQRCodeURL(ctx, order.ID, removePostgresTextNUL(resp.QRCode))

	return orderNo, resp.QRCode, nil
}

// HandleNotify processes an Alipay async notification
func (s *PaymentService) HandleNotify(ctx context.Context, values url.Values) error {
	client, err := s.getAlipayClient(ctx)
	if err != nil {
		return err
	}

	notification, err := client.DecodeNotification(ctx, values)
	if err != nil {
		return fmt.Errorf("verify alipay notification: %w", err)
	}

	// Only process successful trades
	if notification.TradeStatus != "TRADE_SUCCESS" && notification.TradeStatus != "TRADE_FINISHED" {
		return nil
	}

	tradeNo := removePostgresTextNUL(notification.TradeNo)
	return s.completeOrder(ctx, notification.OutTradeNo, &tradeNo)
}

// QueryOrderStatus queries the status of an order, checking Alipay if still pending
func (s *PaymentService) QueryOrderStatus(ctx context.Context, orderNo string, userID int64) (*PaymentOrder, error) {
	order, err := s.paymentRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return nil, err
	}

	// Verify ownership
	if order.UserID != userID {
		return nil, ErrPaymentNotFound
	}

	// If still pending, query Alipay for real-time status
	if order.Status == PaymentStatusPending {
		client, err := s.getAlipayClient(ctx)
		if err == nil {
			resp, err := client.TradeQuery(ctx, alipay.TradeQuery{
				OutTradeNo: orderNo,
			})
			if err == nil && resp.IsSuccess() {
				switch resp.TradeStatus {
				case "TRADE_SUCCESS", "TRADE_FINISHED":
					tradeNo := removePostgresTextNUL(resp.TradeNo)
					if completeErr := s.completeOrder(ctx, orderNo, &tradeNo); completeErr != nil {
						slog.Error("failed to complete order from query", "orderNo", orderNo, "error", completeErr)
					} else {
						// Re-fetch updated order
						order, _ = s.paymentRepo.GetByOrderNo(ctx, orderNo)
					}
				case "TRADE_CLOSED":
					_ = s.paymentRepo.UpdateStatus(ctx, order.ID, PaymentStatusExpired, nil)
					order.Status = PaymentStatusExpired
				}
			}
		}
	}

	return order, nil
}

// PostgreSQL text columns reject U+0000. Payment providers are external input,
// so sanitize persisted response details instead of turning a paid order into a 500.
func removePostgresTextNUL(value string) string {
	if !strings.ContainsRune(value, 0) {
		return value
	}
	return strings.ReplaceAll(value, "\x00", "")
}

// completeOrder marks an order as completed and assigns the subscription (idempotent)
func (s *PaymentService) completeOrder(ctx context.Context, orderNo string, alipayTradeNo *string) error {
	order, err := s.paymentRepo.GetByOrderNo(ctx, orderNo)
	if err != nil {
		return fmt.Errorf("get payment order: %w", err)
	}

	// Idempotency: skip terminal orders without starting the side-effect transaction.
	if order.Status != PaymentStatusPending {
		return nil
	}

	// Use transaction
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	txCtx := dbent.NewTxContext(ctx, tx)

	completed, err := s.paymentRepo.CompleteIfPending(txCtx, order.ID, alipayTradeNo)
	if err != nil {
		return fmt.Errorf("complete payment order: %w", err)
	}
	if !completed {
		return nil
	}

	// Assign or extend subscription
	subscription, _, err := s.subscriptionService.AssignOrExtendSubscription(txCtx, &AssignSubscriptionInput{
		UserID:       order.UserID,
		GroupID:      order.GroupID,
		ValidityDays: order.ValidityDays,
		AssignedBy:   0, // system auto-assign
		Notes:        fmt.Sprintf("Purchased via Alipay, order: %s", order.OrderNo),
		// 支付购买即开启新计费周期：延长到期时间的同时清零已用额度
		ResetQuotaOnExtend: true,
	})
	if err != nil {
		return fmt.Errorf("assign subscription: %w", err)
	}
	if s.affiliateConsumption != nil {
		group, err := s.subscriptionService.groupRepo.GetByID(txCtx, order.GroupID)
		if err != nil {
			return fmt.Errorf("load subscription group for affiliate attribution: %w", err)
		}
		creditLimitMicros := AffiliateMonthlyCreditLimitMicros([]*Group{group}, order.ValidityDays)
		if creditLimitMicros > 0 {
			cycleStartsAt := subscription.ExpiresAt.AddDate(0, 0, -order.ValidityDays)
			occurredAt := time.Now()
			var rewardResult *AffiliateFirstPaidPurchaseResult
			if s.affiliateRewards != nil {
				rewardResult, err = s.affiliateRewards.ProcessFirstPaidPurchase(txCtx, AffiliateFirstPaidPurchaseInput{
					UserID:       order.UserID,
					PurchaseType: AffiliatePurchaseMonthlyPayment,
					SourceID:     order.ID,
					PurchaseKey:  fmt.Sprintf("payment:subscription:%d", order.ID),
					AmountMicros: int64(order.AmountCents) * 10_000,
					OccurredAt:   occurredAt,
				})
				if err != nil {
					return fmt.Errorf("process affiliate first paid monthly purchase: %w", err)
				}
			}
			policy, partnerID, customerRate, partnerRate := AffiliatePolicyFromPurchaseResult(
				order.AmountCents > 0,
				rewardResult,
			)
			_, pricingTableVersion := AffiliateMonthlyCatalogIdentity([]*Group{group})
			if err := s.affiliateConsumption.RecordMonthlyEntitlement(txCtx, AffiliateMonthlyEntitlementInput{
				UserID:                   order.UserID,
				SourceType:               AffiliateSourcePaidTopup,
				SourceID:                 order.ID,
				SourceKey:                fmt.Sprintf("payment:subscription:%d", order.ID),
				ProductCode:              order.PlanID,
				SalePriceMicros:          int64(order.AmountCents) * 10_000,
				CreditLimitMicros:        creditLimitMicros,
				AffiliatePolicy:          policy,
				DirectPartnerID:          partnerID,
				CustomerRebateRateBPS:    customerRate,
				PartnerCommissionRateBPS: partnerRate,
				PricingTableVersion:      pricingTableVersion,
				StartsAt:                 cycleStartsAt,
				EndsAt:                   subscription.ExpiresAt,
				Subscriptions: []AffiliateMonthlySubscription{{
					UserSubscriptionID: subscription.ID,
					GroupID:            order.GroupID,
				}},
			}); err != nil {
				return fmt.Errorf("record affiliate monthly entitlement: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// GetUserPaymentHistory returns payment history for a user
func (s *PaymentService) GetUserPaymentHistory(ctx context.Context, userID int64, limit int) ([]PaymentOrder, error) {
	orders, err := s.paymentRepo.ListByUser(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("get payment history: %w", err)
	}
	return orders, nil
}

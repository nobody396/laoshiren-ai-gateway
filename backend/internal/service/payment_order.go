package service

import (
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

var (
	ErrAlipayNotConfigured = infraerrors.BadRequest("ALIPAY_NOT_CONFIGURED", "alipay payment is not configured")
	ErrInvalidPlan         = infraerrors.BadRequest("INVALID_PLAN", "invalid subscription plan")
	ErrPaymentNotFound     = infraerrors.NotFound("PAYMENT_NOT_FOUND", "payment order not found")
	ErrPaymentCompleted    = infraerrors.Conflict("PAYMENT_COMPLETED", "payment order already completed")
)

const (
	PaymentStatusPending   = "pending"
	PaymentStatusCompleted = "completed"
	PaymentStatusExpired   = "expired"
	PaymentStatusFailed    = "failed"
)

// SubscriptionPlan represents a purchasable subscription plan
type SubscriptionPlan struct {
	ID           string `json:"id"`            // unique plan ID (e.g. "plan_claude_30d")
	Name         string `json:"name"`          // display name
	PriceCents   int    `json:"price_cents"`   // price in CNY fen (e.g. 9900 = ¥99.00)
	GroupID      int64  `json:"group_id"`      // linked subscription group
	ValidityDays int    `json:"validity_days"` // subscription duration in days
	Description  string `json:"description"`   // optional description
}

// PaymentOrder represents a payment order domain model
type PaymentOrder struct {
	ID           int64      `json:"id"`
	OrderNo      string     `json:"order_no"`
	UserID       int64      `json:"user_id"`
	AmountCents  int        `json:"amount_cents"`  // price in CNY fen
	PlanID       string     `json:"plan_id"`       // subscription plan ID
	GroupID      int64      `json:"group_id"`      // target group
	ValidityDays int        `json:"validity_days"` // subscription days
	Status       string     `json:"status"`
	AlipayTradeNo *string   `json:"alipay_trade_no,omitempty"`
	QRCodeURL    *string    `json:"qr_code_url,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

package service

import (
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

var (
	ErrXunhuNotConfigured    = infraerrors.BadRequest("XUNHU_NOT_CONFIGURED", "xunhu payment is not configured")
	ErrTopupProviderMismatch = infraerrors.BadRequest("TOPUP_PROVIDER_MISMATCH", "notify provider does not match order provider")
	ErrTopupAmountMismatch   = infraerrors.BadRequest("TOPUP_AMOUNT_MISMATCH", "notify amount does not match order amount")
	ErrTopupPayTypeMismatch  = infraerrors.BadRequest("TOPUP_PAY_TYPE_MISMATCH", "notify pay type does not match order pay type")
	ErrTopupNotFound         = infraerrors.NotFound("TOPUP_NOT_FOUND", "topup order not found")
	ErrTopupMinAmount        = infraerrors.BadRequest("TOPUP_MIN_AMOUNT", "minimum topup amount is ¥20")
	ErrTopupMaxAmount        = infraerrors.BadRequest("TOPUP_MAX_AMOUNT", "单次充值最高为 ¥3000")
	ErrTopupInvalidType      = infraerrors.BadRequest("TOPUP_INVALID_TYPE", "pay_type must be alipay or wechat")
	ErrTopupInvalidProduct   = infraerrors.BadRequest("TOPUP_INVALID_PRODUCT", "不支持的余额卡面额")
	ErrTopupInvalidQuantity  = infraerrors.BadRequest("TOPUP_INVALID_QUANTITY", "余额卡数量必须大于 0")
	ErrTopupProductMismatch  = infraerrors.BadRequest("TOPUP_PRODUCT_MISMATCH", "充值总额与余额卡面额和数量不一致")
)

const (
	TopupStatusPending   = "pending"
	TopupStatusCompleted = "completed"
	TopupStatusExpired   = "expired"

	// TopupMinAmountFen 最低充值金额（分）
	TopupMinAmountFen = 2000 // ¥20
	// TopupMaxAmountFen 单次充值最高金额（分）
	TopupMaxAmountFen = 300000 // ¥3000
	// TopupOrderTTL is the customer-facing payment window for QR orders.
	TopupOrderTTL = 5 * time.Minute
)

// TopupOrder represents a topup order domain model
type TopupOrder struct {
	ID                int64      `json:"id"`
	OrderNo           string     `json:"order_no"`
	UserID            int64      `json:"user_id"`
	AmountCNYFen      int        `json:"amount_cny_fen"`       // 充值金额，单位：分（CNY）
	BonusAmountCNYFen int        `json:"bonus_amount_cny_fen"` // 下单时锁定的活动赠送额度，单位：分
	PayType           string     `json:"pay_type"`             // alipay / wechat
	Provider          string     `json:"provider"`             // 支付网关：xunhu / easypay
	Status            string     `json:"status"`
	InvoiceStatus     string     `json:"invoice_status"`
	XunhuTradeNo      *string    `json:"xunhu_trade_no,omitempty"`
	QRCodeURL         *string    `json:"qr_code_url,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

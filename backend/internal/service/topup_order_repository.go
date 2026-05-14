package service

import "context"

// TopupOrderRepository defines the persistence interface for topup orders
type TopupOrderRepository interface {
	Create(ctx context.Context, order *TopupOrder) error
	GetByOrderNo(ctx context.Context, orderNo string) (*TopupOrder, error)
	UpdateStatus(ctx context.Context, id int64, status string, xunhuTradeNo *string) error
	UpdateQRCodeURL(ctx context.Context, id int64, qrCodeURL string) error
	// CompleteIfPending 原子地将 pending 订单标记为 completed，返回是否成功（false 表示已被处理过）
	CompleteIfPending(ctx context.Context, id int64, xunhuTradeNo *string) (bool, error)
}

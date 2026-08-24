package service

import "context"

// TopupOrderRepository defines the persistence interface for topup orders
type TopupOrderRepository interface {
	Create(ctx context.Context, order *TopupOrder) error
	GetByOrderNo(ctx context.Context, orderNo string) (*TopupOrder, error)
	UpdateStatus(ctx context.Context, id int64, status string, xunhuTradeNo *string) error
	UpdateQRCodeURL(ctx context.Context, id int64, qrCodeURL string) error
	// CompleteIfUnsettled atomically completes a pending or locally expired
	// order. A verified late provider callback must still credit the customer.
	CompleteIfUnsettled(ctx context.Context, id int64, xunhuTradeNo *string) (bool, error)
}

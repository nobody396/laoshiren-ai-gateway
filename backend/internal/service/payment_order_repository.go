package service

import "context"

// PaymentOrderRepository defines the persistence interface for payment orders
type PaymentOrderRepository interface {
	Create(ctx context.Context, order *PaymentOrder) error
	GetByOrderNo(ctx context.Context, orderNo string) (*PaymentOrder, error)
	UpdateStatus(ctx context.Context, id int64, status string, alipayTradeNo *string) error
	CompleteIfPending(ctx context.Context, id int64, alipayTradeNo *string) (bool, error)
	UpdateQRCodeURL(ctx context.Context, id int64, qrCodeURL string) error
	ListByUser(ctx context.Context, userID int64, limit int) ([]PaymentOrder, error)
}

package repository

import (
	"context"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/topuporder"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type topupOrderRepository struct {
	client *dbent.Client
}

// NewTopupOrderRepository creates a new TopupOrderRepository backed by ent
func NewTopupOrderRepository(client *dbent.Client) service.TopupOrderRepository {
	return &topupOrderRepository{client: client}
}

func (r *topupOrderRepository) Create(ctx context.Context, order *service.TopupOrder) error {
	client := clientFromContext(ctx, r.client)
	create := client.TopupOrder.Create().
		SetOrderNo(order.OrderNo).
		SetUserID(order.UserID).
		SetAmountCnyFen(order.AmountCNYFen).
		SetBonusAmountCnyFen(order.BonusAmountCNYFen).
		SetPayType(order.PayType).
		SetStatus(order.Status)
	if order.Provider != "" {
		create.SetProvider(order.Provider)
	}
	created, err := create.Save(ctx)
	if err != nil {
		return err
	}
	order.ID = created.ID
	order.Provider = created.Provider
	order.CreatedAt = created.CreatedAt
	order.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *topupOrderRepository) GetByOrderNo(ctx context.Context, orderNo string) (*service.TopupOrder, error) {
	m, err := r.client.TopupOrder.Query().
		Where(topuporder.OrderNoEQ(orderNo)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrTopupNotFound
		}
		return nil, err
	}
	return topupOrderEntityToService(m), nil
}

func (r *topupOrderRepository) UpdateStatus(ctx context.Context, id int64, status string, xunhuTradeNo *string) error {
	client := clientFromContext(ctx, r.client)
	up := client.TopupOrder.UpdateOneID(id).
		SetStatus(status).
		SetUpdatedAt(time.Now())

	if status == service.TopupStatusCompleted {
		now := time.Now()
		up.SetCompletedAt(now)
	}
	if xunhuTradeNo != nil && *xunhuTradeNo != "" {
		up.SetXunhuTradeNo(*xunhuTradeNo)
	}

	return up.Exec(ctx)
}

func (r *topupOrderRepository) CompleteIfUnsettled(ctx context.Context, id int64, xunhuTradeNo *string) (bool, error) {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	up := client.TopupOrder.Update().
		Where(
			topuporder.IDEQ(id),
			topuporder.StatusIn(service.TopupStatusPending, service.TopupStatusExpired),
		).
		SetStatus(service.TopupStatusCompleted).
		SetCompletedAt(now).
		SetUpdatedAt(now).
		SetNillableXunhuTradeNo(xunhuTradeNo)

	n, err := up.Save(ctx)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *topupOrderRepository) UpdateQRCodeURL(ctx context.Context, id int64, qrCodeURL string) error {
	client := clientFromContext(ctx, r.client)
	return client.TopupOrder.UpdateOneID(id).
		SetQrCodeURL(qrCodeURL).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
}

func topupOrderEntityToService(m *dbent.TopupOrder) *service.TopupOrder {
	if m == nil {
		return nil
	}
	return &service.TopupOrder{
		ID:                m.ID,
		OrderNo:           m.OrderNo,
		UserID:            m.UserID,
		AmountCNYFen:      m.AmountCnyFen,
		BonusAmountCNYFen: m.BonusAmountCnyFen,
		PayType:           m.PayType,
		Provider:          m.Provider,
		Status:            m.Status,
		InvoiceStatus:     m.InvoiceStatus,
		XunhuTradeNo:      m.XunhuTradeNo,
		QRCodeURL:         m.QrCodeURL,
		CompletedAt:       m.CompletedAt,
		CreatedAt:         m.CreatedAt,
		UpdatedAt:         m.UpdatedAt,
	}
}

package repository

import (
	"context"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/paymentorder"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type paymentOrderRepository struct {
	client *dbent.Client
}

func NewPaymentOrderRepository(client *dbent.Client) service.PaymentOrderRepository {
	return &paymentOrderRepository{client: client}
}

func (r *paymentOrderRepository) Create(ctx context.Context, order *service.PaymentOrder) error {
	client := clientFromContext(ctx, r.client)
	created, err := client.PaymentOrder.Create().
		SetOrderNo(order.OrderNo).
		SetUserID(order.UserID).
		SetAmountCents(order.AmountCents).
		SetPlanID(order.PlanID).
		SetGroupID(order.GroupID).
		SetValidityDays(order.ValidityDays).
		SetStatus(order.Status).
		Save(ctx)
	if err != nil {
		return err
	}
	order.ID = created.ID
	order.CreatedAt = created.CreatedAt
	order.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *paymentOrderRepository) GetByOrderNo(ctx context.Context, orderNo string) (*service.PaymentOrder, error) {
	m, err := r.client.PaymentOrder.Query().
		Where(paymentorder.OrderNoEQ(orderNo)).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrPaymentNotFound
		}
		return nil, err
	}
	return paymentOrderEntityToService(m), nil
}

func (r *paymentOrderRepository) UpdateStatus(ctx context.Context, id int64, status string, alipayTradeNo *string) error {
	client := clientFromContext(ctx, r.client)
	up := client.PaymentOrder.UpdateOneID(id).
		SetStatus(status).
		SetUpdatedAt(time.Now())

	if status == service.PaymentStatusCompleted {
		now := time.Now()
		up.SetCompletedAt(now)
	}
	if alipayTradeNo != nil {
		up.SetAlipayTradeNo(*alipayTradeNo)
	}

	return up.Exec(ctx)
}

func (r *paymentOrderRepository) CompleteIfPending(ctx context.Context, id int64, alipayTradeNo *string) (bool, error) {
	client := clientFromContext(ctx, r.client)
	now := time.Now()
	n, err := client.PaymentOrder.Update().
		Where(paymentorder.IDEQ(id), paymentorder.StatusEQ(service.PaymentStatusPending)).
		SetStatus(service.PaymentStatusCompleted).
		SetCompletedAt(now).
		SetUpdatedAt(now).
		SetNillableAlipayTradeNo(alipayTradeNo).
		Save(ctx)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *paymentOrderRepository) UpdateQRCodeURL(ctx context.Context, id int64, qrCodeURL string) error {
	client := clientFromContext(ctx, r.client)
	return client.PaymentOrder.UpdateOneID(id).
		SetQrCodeURL(qrCodeURL).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
}

func (r *paymentOrderRepository) ListByUser(ctx context.Context, userID int64, limit int) ([]service.PaymentOrder, error) {
	if limit <= 0 {
		limit = 20
	}

	orders, err := r.client.PaymentOrder.Query().
		Where(paymentorder.UserIDEQ(userID)).
		Order(dbent.Desc(paymentorder.FieldCreatedAt)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}

	return paymentOrderEntitiesToService(orders), nil
}

func paymentOrderEntityToService(m *dbent.PaymentOrder) *service.PaymentOrder {
	if m == nil {
		return nil
	}
	return &service.PaymentOrder{
		ID:            m.ID,
		OrderNo:       m.OrderNo,
		UserID:        m.UserID,
		AmountCents:   m.AmountCents,
		PlanID:        m.PlanID,
		GroupID:       m.GroupID,
		ValidityDays:  m.ValidityDays,
		Status:        m.Status,
		AlipayTradeNo: m.AlipayTradeNo,
		QRCodeURL:     m.QrCodeURL,
		CompletedAt:   m.CompletedAt,
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}

func paymentOrderEntitiesToService(models []*dbent.PaymentOrder) []service.PaymentOrder {
	out := make([]service.PaymentOrder, 0, len(models))
	for i := range models {
		if s := paymentOrderEntityToService(models[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}

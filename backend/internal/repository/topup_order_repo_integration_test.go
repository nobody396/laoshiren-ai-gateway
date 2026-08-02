//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTopupOrderRepositoryPersistsPromotionBonus(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewTopupOrderRepository(client)
	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("topup-promotion-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
	})

	order := &service.TopupOrder{
		OrderNo:           fmt.Sprintf("TPPROMO%d", time.Now().UnixNano()),
		UserID:            user.ID,
		AmountCNYFen:      service.TopupPromotion1000PaidFen,
		BonusAmountCNYFen: service.TopupPromotion1000BonusFen,
		PayType:           "alipay",
		Status:            service.TopupStatusPending,
	}
	require.NoError(t, repo.Create(ctx, order))

	got, err := repo.GetByOrderNo(ctx, order.OrderNo)
	require.NoError(t, err)
	require.Equal(t, service.TopupPromotion1000PaidFen, got.AmountCNYFen)
	require.Equal(t, service.TopupPromotion1000BonusFen, got.BonusAmountCNYFen)
	require.Equal(t, 120_000, service.StoredTopupCreditQuote(got.AmountCNYFen, got.BonusAmountCNYFen).CreditedAmountCNYFen)
}

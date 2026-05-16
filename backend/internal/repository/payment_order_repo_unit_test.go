package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/enttest"
	"github.com/bozhouDev/DragonCode-sub2api/ent/paymentorder"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newPaymentOrderRepoSQLite(t *testing.T) (*paymentOrderRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:payment_order_repo_%d?mode=memory&cache=shared&_fk=1", time.Now().UnixNano()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return &paymentOrderRepository{client: client}, client
}

func mustCreatePaymentOrderRepoUser(t *testing.T, ctx context.Context, client *dbent.Client, email string) int64 {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	return user.ID
}

func TestPaymentOrderRepositoryCompleteIfPendingCompletesOnce(t *testing.T) {
	repo, client := newPaymentOrderRepoSQLite(t)
	ctx := context.Background()
	userID := mustCreatePaymentOrderRepoUser(t, ctx, client, "payment-complete-once@test.com")
	order := &service.PaymentOrder{
		OrderNo:      "payment-complete-once",
		UserID:       userID,
		AmountCents:  9900,
		PlanID:       "plan-30d",
		GroupID:      7,
		ValidityDays: 30,
		Status:       service.PaymentStatusPending,
	}
	require.NoError(t, repo.Create(ctx, order))

	tradeNo := "trade-first"
	done, err := repo.CompleteIfPending(ctx, order.ID, &tradeNo)
	require.NoError(t, err)
	require.True(t, done)

	got, err := client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.Equal(t, service.PaymentStatusCompleted, got.Status)
	require.NotNil(t, got.CompletedAt)
	require.NotNil(t, got.AlipayTradeNo)
	require.Equal(t, tradeNo, *got.AlipayTradeNo)

	secondTradeNo := "trade-second"
	done, err = repo.CompleteIfPending(ctx, order.ID, &secondTradeNo)
	require.NoError(t, err)
	require.False(t, done)

	got, err = client.PaymentOrder.Get(ctx, order.ID)
	require.NoError(t, err)
	require.NotNil(t, got.AlipayTradeNo)
	require.Equal(t, tradeNo, *got.AlipayTradeNo, "duplicate completion must not overwrite existing trade number")
}

func TestPaymentOrderRepositoryCompleteIfPendingNonPendingReturnsFalse(t *testing.T) {
	repo, client := newPaymentOrderRepoSQLite(t)
	ctx := context.Background()
	userID := mustCreatePaymentOrderRepoUser(t, ctx, client, "payment-non-pending@test.com")

	for _, status := range []string{service.PaymentStatusCompleted, service.PaymentStatusExpired} {
		order, err := client.PaymentOrder.Create().
			SetOrderNo("payment-non-pending-" + status).
			SetUserID(userID).
			SetAmountCents(9900).
			SetPlanID("plan-30d").
			SetGroupID(7).
			SetValidityDays(30).
			SetStatus(status).
			SetAlipayTradeNo("existing-" + status).
			Save(ctx)
		require.NoError(t, err)

		tradeNo := "new-" + status
		done, err := repo.CompleteIfPending(ctx, order.ID, &tradeNo)
		require.NoError(t, err)
		require.False(t, done)

		got, err := client.PaymentOrder.Get(ctx, order.ID)
		require.NoError(t, err)
		require.Equal(t, status, got.Status)
		require.NotNil(t, got.AlipayTradeNo)
		require.Equal(t, "existing-"+status, *got.AlipayTradeNo)
	}
}

func TestPaymentOrderRepositoryCompleteIfPendingConcurrentOnlyOneWinner(t *testing.T) {
	repo, client := newPaymentOrderRepoSQLite(t)
	ctx := context.Background()
	userID := mustCreatePaymentOrderRepoUser(t, ctx, client, "payment-concurrent@test.com")
	order := &service.PaymentOrder{
		OrderNo:      "payment-concurrent",
		UserID:       userID,
		AmountCents:  9900,
		PlanID:       "plan-30d",
		GroupID:      7,
		ValidityDays: 30,
		Status:       service.PaymentStatusPending,
	}
	require.NoError(t, repo.Create(ctx, order))

	start := make(chan struct{})
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			tradeNo := fmt.Sprintf("trade-%d", i)
			done, err := repo.CompleteIfPending(context.Background(), order.ID, &tradeNo)
			if err != nil {
				errs <- err
				return
			}
			results <- done
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)

	require.Empty(t, errs)
	winners := 0
	for done := range results {
		if done {
			winners++
		}
	}
	require.Equal(t, 1, winners)

	got, err := client.PaymentOrder.Query().Where(paymentorder.IDEQ(order.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, service.PaymentStatusCompleted, got.Status)
	require.NotNil(t, got.CompletedAt)
}

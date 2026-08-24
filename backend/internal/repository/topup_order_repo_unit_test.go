package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/enttest"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func TestTopupOrderRepositoryCompleteIfUnsettledAcceptsLatePaidExpiredOrder(t *testing.T) {
	db, err := sql.Open("sqlite", fmt.Sprintf("file:topup_order_repo_%d?mode=memory&cache=shared&_fk=1", time.Now().UnixNano()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	user := client.User.Create().SetEmail("late-topup@example.com").SetPasswordHash("hash").SaveX(ctx)
	order := client.TopupOrder.Create().
		SetOrderNo("TP-LATE-EXPIRED").
		SetUserID(user.ID).
		SetAmountCnyFen(2000).
		SetPayType("alipay").
		SetStatus(service.TopupStatusExpired).
		SaveX(ctx)
	repo := &topupOrderRepository{client: client}
	tradeNo := "late-trade"

	done, err := repo.CompleteIfUnsettled(ctx, order.ID, &tradeNo)
	require.NoError(t, err)
	require.True(t, done)
	got := client.TopupOrder.GetX(ctx, order.ID)
	require.Equal(t, service.TopupStatusCompleted, got.Status)
	require.Equal(t, tradeNo, *got.XunhuTradeNo)

	done, err = repo.CompleteIfUnsettled(ctx, order.ID, &tradeNo)
	require.NoError(t, err)
	require.False(t, done)
}

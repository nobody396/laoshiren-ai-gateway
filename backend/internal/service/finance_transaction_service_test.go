package service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/enttest"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/repository"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newFinanceTransactionServiceSQLite(t *testing.T) *service.FinanceTransactionService {
	t.Helper()

	db, err := sql.Open("sqlite", "file:finance_transaction_service?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return service.NewFinanceTransactionService(repository.NewFinanceTransactionRepository(client))
}

func financeIncomeEvidence() (*string, *string) {
	receipt := "finance-receipts/test.jpg"
	channel := "wechat"
	return &receipt, &channel
}

func TestFinanceTransactionServiceCreateRejectsMismatchedCategory(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type:      service.FinanceTransactionTypeIncome,
		Category:  service.FinanceTransactionCategoryServerCost, // expense-only category
		AmountFen: 1000,
	})
	require.ErrorIs(t, err, service.ErrFinanceTransactionInvalidCat)
}

func TestFinanceTransactionServiceCreateRejectsNonPositiveAmount(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type:      service.FinanceTransactionTypeExpense,
		Category:  service.FinanceTransactionCategoryServerCost,
		AmountFen: 0,
	})
	require.ErrorIs(t, err, service.ErrFinanceTransactionInvalidAmount)
}

func TestFinanceTransactionServiceCreateDefaultsSourceAndOccurredAt(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	before := time.Now()
	receipt, channel := financeIncomeEvidence()
	created, err := svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type:           service.FinanceTransactionTypeIncome,
		Category:       service.FinanceTransactionCategorySaleRevenue,
		AmountFen:      5000,
		ReceiptKey:     receipt,
		PaymentChannel: channel,
	})
	require.NoError(t, err)
	require.Equal(t, service.FinanceTransactionSourceManual, created.Source)
	require.False(t, created.OccurredAt.Before(before))
	require.Greater(t, created.ID, int64(0))
}

func TestFinanceTransactionServiceUpdatePartialFields(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type:      service.FinanceTransactionTypeExpense,
		Category:  service.FinanceTransactionCategoryServerCost,
		AmountFen: 2000,
	})
	require.NoError(t, err)

	newAmount := int64(3500)
	note := "换了更贵的服务器套餐"
	notePtr := &note
	updated, err := svc.Update(ctx, created.ID, &service.UpdateFinanceTransactionInput{
		AmountFen: &newAmount,
		Note:      &notePtr,
	})
	require.NoError(t, err)
	require.Equal(t, newAmount, updated.AmountFen)
	require.NotNil(t, updated.Note)
	require.Equal(t, note, *updated.Note)
	// Untouched fields survive the partial update.
	require.Equal(t, service.FinanceTransactionCategoryServerCost, updated.Category)
}

func TestFinanceTransactionServiceUpdateRejectsStaleCategoryOnTypeChange(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	created, err := svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type:      service.FinanceTransactionTypeExpense,
		Category:  service.FinanceTransactionCategoryServerCost,
		AmountFen: 2000,
	})
	require.NoError(t, err)

	newType := service.FinanceTransactionTypeIncome
	_, err = svc.Update(ctx, created.ID, &service.UpdateFinanceTransactionInput{
		Type: &newType, // no new Category given: old "server_cost" is not a valid income category
	})
	require.ErrorIs(t, err, service.ErrFinanceTransactionInvalidCat)
}

func TestFinanceTransactionServiceDeleteAndGetByID(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	receipt, channel := financeIncomeEvidence()
	created, err := svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type:           service.FinanceTransactionTypeIncome,
		Category:       service.FinanceTransactionCategoryOtherIncome,
		AmountFen:      100,
		ReceiptKey:     receipt,
		PaymentChannel: channel,
	})
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, created.ID))

	_, err = svc.GetByID(ctx, created.ID)
	require.ErrorIs(t, err, service.ErrFinanceTransactionNotFound)
}

func TestFinanceTransactionServiceListFiltersByType(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	receipt, channel := financeIncomeEvidence()
	_, err := svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type: service.FinanceTransactionTypeIncome, Category: service.FinanceTransactionCategorySaleRevenue, AmountFen: 1000,
		ReceiptKey: receipt, PaymentChannel: channel,
	})
	require.NoError(t, err)
	_, err = svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type: service.FinanceTransactionTypeExpense, Category: service.FinanceTransactionCategoryServerCost, AmountFen: 300,
	})
	require.NoError(t, err)

	items, page, err := svc.List(ctx, pagination.DefaultPagination(), service.FinanceTransactionListFilters{
		Type: service.FinanceTransactionTypeExpense,
	})
	require.NoError(t, err)
	require.EqualValues(t, 1, page.Total)
	require.Len(t, items, 1)
	require.Equal(t, service.FinanceTransactionCategoryServerCost, items[0].Category)
}

func TestFinanceTransactionServiceSummaryComputesMarginAndCategoryTotals(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	from := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	mustCreate := func(txType, category string, amountFen int64) {
		input := &service.CreateFinanceTransactionInput{
			Type:       txType,
			Category:   category,
			AmountFen:  amountFen,
			OccurredAt: now,
		}
		if txType == service.FinanceTransactionTypeIncome {
			input.ReceiptKey, input.PaymentChannel = financeIncomeEvidence()
		}
		_, err := svc.Create(ctx, input)
		require.NoError(t, err)
	}

	mustCreate(service.FinanceTransactionTypeIncome, service.FinanceTransactionCategorySaleRevenue, 100000) // ¥1000
	mustCreate(service.FinanceTransactionTypeIncome, service.FinanceTransactionCategorySaleRevenue, 50000)  // ¥500
	mustCreate(service.FinanceTransactionTypeExpense, service.FinanceTransactionCategoryUpstreamTopup, 30000)
	mustCreate(service.FinanceTransactionTypeExpense, service.FinanceTransactionCategoryServerCost, 20000)

	// Outside the [from, to) range: must not leak into the summary.
	receipt, channel := financeIncomeEvidence()
	_, err := svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type:           service.FinanceTransactionTypeIncome,
		Category:       service.FinanceTransactionCategorySaleRevenue,
		AmountFen:      999999,
		OccurredAt:     time.Date(2026, 6, 30, 23, 59, 59, 0, time.UTC),
		ReceiptKey:     receipt,
		PaymentChannel: channel,
	})
	require.NoError(t, err)

	summary, err := svc.Summary(ctx, from, to)
	require.NoError(t, err)

	require.EqualValues(t, 150000, summary.TotalIncomeFen)
	require.EqualValues(t, 50000, summary.TotalExpenseFen)
	require.EqualValues(t, 100000, summary.NetProfitFen)
	require.InDelta(t, 66.666, summary.MarginPercent, 0.01)

	byCategory := make(map[string]int64)
	for _, c := range summary.ByCategory {
		byCategory[c.Type+":"+c.Category] = c.TotalFen
	}
	require.EqualValues(t, 150000, byCategory["income:sale_revenue"])
	require.EqualValues(t, 30000, byCategory["expense:upstream_topup"])
	require.EqualValues(t, 20000, byCategory["expense:server_cost"])
	require.Len(t, summary.MonthlySeries, 1)
	require.Equal(t, "2026-07", summary.MonthlySeries[0].Month)
}

func TestFinanceTransactionServiceSummaryRejectsInvalidRange(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	now := time.Now()
	_, err := svc.Summary(ctx, now, now)
	require.Error(t, err)
}

func TestFinanceTransactionServiceIncomeRequiresReceiptAndPaymentChannel(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	_, err := svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type:      service.FinanceTransactionTypeIncome,
		Category:  service.FinanceTransactionCategorySaleRevenue,
		AmountFen: 1000,
	})
	require.ErrorIs(t, err, service.ErrFinanceTransactionIncomeReceiptRequired)

	receipt, _ := financeIncomeEvidence()
	_, err = svc.Create(ctx, &service.CreateFinanceTransactionInput{
		Type:       service.FinanceTransactionTypeIncome,
		Category:   service.FinanceTransactionCategorySaleRevenue,
		AmountFen:  1000,
		ReceiptKey: receipt,
	})
	require.ErrorIs(t, err, service.ErrFinanceTransactionIncomeChannelRequired)
}

func TestFinanceTransactionServiceSummaryAllIncludesAllMonths(t *testing.T) {
	svc := newFinanceTransactionServiceSQLite(t)
	ctx := context.Background()

	for _, occurredAt := range []time.Time{
		time.Date(2026, 5, 14, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
		time.Date(2026, 6, 14, 0, 0, 0, 0, time.FixedZone("CST", 8*60*60)),
	} {
		_, err := svc.Create(ctx, &service.CreateFinanceTransactionInput{
			Type:       service.FinanceTransactionTypeExpense,
			Category:   service.FinanceTransactionCategoryHostingCost,
			AmountFen:  17899,
			OccurredAt: occurredAt,
		})
		require.NoError(t, err)
	}

	summary, err := svc.SummaryAll(ctx, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.EqualValues(t, 35798, summary.TotalExpenseFen)
	require.Len(t, summary.MonthlySeries, 2)
	require.Equal(t, "2026-05", summary.MonthlySeries[0].Month)
	require.Equal(t, "2026-06", summary.MonthlySeries[1].Month)
}

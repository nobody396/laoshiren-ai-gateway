package repository

import (
	"context"
	"sort"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/financetransaction"
	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type financeTransactionRepository struct {
	client *dbent.Client
}

func NewFinanceTransactionRepository(client *dbent.Client) service.FinanceTransactionRepository {
	return &financeTransactionRepository{client: client}
}

func (r *financeTransactionRepository) Create(ctx context.Context, t *service.FinanceTransaction) error {
	client := clientFromContext(ctx, r.client)
	builder := client.FinanceTransaction.Create().
		SetType(t.Type).
		SetCategory(t.Category).
		SetAmountFen(t.AmountFen).
		SetOccurredAt(t.OccurredAt).
		SetSource(t.Source)

	if t.Note != nil {
		builder.SetNote(*t.Note)
	}
	if t.ReceiptKey != nil {
		builder.SetReceiptKey(*t.ReceiptKey)
	}
	if t.PaymentChannel != nil {
		builder.SetPaymentChannel(*t.PaymentChannel)
	}
	if t.CreatedBy != nil {
		builder.SetCreatedBy(*t.CreatedBy)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return err
	}

	applyFinanceTransactionEntityToService(t, created)
	return nil
}

func (r *financeTransactionRepository) GetByID(ctx context.Context, id int64) (*service.FinanceTransaction, error) {
	m, err := r.client.FinanceTransaction.Query().
		Where(financetransaction.IDEQ(id)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrFinanceTransactionNotFound, nil)
	}
	return financeTransactionEntityToService(m), nil
}

func (r *financeTransactionRepository) Update(ctx context.Context, t *service.FinanceTransaction) error {
	client := clientFromContext(ctx, r.client)
	builder := client.FinanceTransaction.UpdateOneID(t.ID).
		SetType(t.Type).
		SetCategory(t.Category).
		SetAmountFen(t.AmountFen).
		SetOccurredAt(t.OccurredAt)

	if t.Note != nil {
		builder.SetNote(*t.Note)
	} else {
		builder.ClearNote()
	}
	if t.ReceiptKey != nil {
		builder.SetReceiptKey(*t.ReceiptKey)
	} else {
		builder.ClearReceiptKey()
	}
	if t.PaymentChannel != nil {
		builder.SetPaymentChannel(*t.PaymentChannel)
	} else {
		builder.ClearPaymentChannel()
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrFinanceTransactionNotFound, nil)
	}

	t.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *financeTransactionRepository) Delete(ctx context.Context, id int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.FinanceTransaction.Delete().Where(financetransaction.IDEQ(id)).Exec(ctx)
	return err
}

func (r *financeTransactionRepository) List(
	ctx context.Context,
	params pagination.PaginationParams,
	filters service.FinanceTransactionListFilters,
) ([]service.FinanceTransaction, *pagination.PaginationResult, error) {
	q := r.client.FinanceTransaction.Query()
	q = applyFinanceTransactionFilters(q, filters)

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	itemsQuery := q.
		Offset(params.Offset()).
		Limit(params.Limit())
	for _, order := range financeTransactionListOrders(params) {
		itemsQuery = itemsQuery.Order(order)
	}

	items, err := itemsQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}

	out := financeTransactionEntitiesToService(items)
	return out, paginationResultFromTotal(int64(total), params), nil
}

// Summary 汇总 [from, to) 区间内的收支：总收入、总支出、净利润、利润率、按 (type, category) 小计。
// 账本体量是个人手工记账规模，直接把区间内的行拉到内存里聚合，比手写 SQL GROUP BY 更简单可靠。
func (r *financeTransactionRepository) Summary(ctx context.Context, from, to time.Time) (*service.FinanceTransactionSummary, error) {
	items, err := r.client.FinanceTransaction.Query().
		Where(
			financetransaction.OccurredAtGTE(from),
			financetransaction.OccurredAtLT(to),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return summarizeFinanceTransactions(items, from, to), nil
}

func (r *financeTransactionRepository) SummaryAll(ctx context.Context, to time.Time) (*service.FinanceTransactionSummary, error) {
	items, err := r.client.FinanceTransaction.Query().
		Where(financetransaction.OccurredAtLT(to)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	from := to
	for _, item := range items {
		if item.OccurredAt.Before(from) {
			from = item.OccurredAt
		}
	}
	return summarizeFinanceTransactions(items, from, to), nil
}

func summarizeFinanceTransactions(
	items []*dbent.FinanceTransaction,
	from, to time.Time,
) *service.FinanceTransactionSummary {
	summary := &service.FinanceTransactionSummary{RangeFrom: from, RangeTo: to}

	type key struct {
		txType   string
		category string
	}
	totals := make(map[key]*domain.FinanceCategoryTotal)
	order := make([]key, 0)
	monthlyTotals := make(map[string]*domain.FinanceMonthlyTotal)
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)

	for _, m := range items {
		switch m.Type {
		case domain.FinanceTransactionTypeIncome:
			summary.TotalIncomeFen += m.AmountFen
		case domain.FinanceTransactionTypeExpense:
			summary.TotalExpenseFen += m.AmountFen
		}

		k := key{txType: m.Type, category: m.Category}
		agg, ok := totals[k]
		if !ok {
			agg = &domain.FinanceCategoryTotal{Type: m.Type, Category: m.Category}
			totals[k] = agg
			order = append(order, k)
		}
		agg.TotalFen += m.AmountFen
		agg.TxCount++

		month := m.OccurredAt.In(shanghai).Format("2006-01")
		monthly, ok := monthlyTotals[month]
		if !ok {
			monthly = &domain.FinanceMonthlyTotal{Month: month}
			monthlyTotals[month] = monthly
		}
		if m.Type == domain.FinanceTransactionTypeIncome {
			monthly.TotalIncomeFen += m.AmountFen
		} else if m.Type == domain.FinanceTransactionTypeExpense {
			monthly.TotalExpenseFen += m.AmountFen
		}
	}

	summary.NetProfitFen = summary.TotalIncomeFen - summary.TotalExpenseFen
	if summary.TotalIncomeFen > 0 {
		summary.MarginPercent = float64(summary.NetProfitFen) / float64(summary.TotalIncomeFen) * 100
	}

	summary.ByCategory = make([]domain.FinanceCategoryTotal, 0, len(order))
	for _, k := range order {
		summary.ByCategory = append(summary.ByCategory, *totals[k])
	}

	months := make([]string, 0, len(monthlyTotals))
	for month := range monthlyTotals {
		months = append(months, month)
	}
	sort.Strings(months)
	summary.MonthlySeries = make([]domain.FinanceMonthlyTotal, 0, len(months))
	for _, month := range months {
		total := monthlyTotals[month]
		total.NetProfitFen = total.TotalIncomeFen - total.TotalExpenseFen
		summary.MonthlySeries = append(summary.MonthlySeries, *total)
	}

	return summary
}

func applyFinanceTransactionFilters(q *dbent.FinanceTransactionQuery, filters service.FinanceTransactionListFilters) *dbent.FinanceTransactionQuery {
	if filters.Type != "" {
		q = q.Where(financetransaction.TypeEQ(filters.Type))
	}
	if filters.Category != "" {
		q = q.Where(financetransaction.CategoryEQ(filters.Category))
	}
	if filters.From != nil {
		q = q.Where(financetransaction.OccurredAtGTE(*filters.From))
	}
	if filters.To != nil {
		q = q.Where(financetransaction.OccurredAtLT(*filters.To))
	}
	if filters.Search != "" {
		q = q.Where(financetransaction.NoteContainsFold(filters.Search))
	}
	return q
}

func financeTransactionListOrder(params pagination.PaginationParams) (string, string) {
	sortBy := strings.ToLower(strings.TrimSpace(params.SortBy))
	sortOrder := params.NormalizedSortOrder(pagination.SortOrderDesc)

	switch sortBy {
	case "amount_fen":
		return financetransaction.FieldAmountFen, sortOrder
	case "type":
		return financetransaction.FieldType, sortOrder
	case "category":
		return financetransaction.FieldCategory, sortOrder
	case "id":
		return financetransaction.FieldID, sortOrder
	case "", "occurred_at":
		return financetransaction.FieldOccurredAt, sortOrder
	default:
		return financetransaction.FieldOccurredAt, pagination.SortOrderDesc
	}
}

func financeTransactionListOrders(params pagination.PaginationParams) []func(*entsql.Selector) {
	field, sortOrder := financeTransactionListOrder(params)

	if sortOrder == pagination.SortOrderAsc {
		if field == financetransaction.FieldID {
			return []func(*entsql.Selector){dbent.Asc(field)}
		}
		return []func(*entsql.Selector){
			dbent.Asc(field),
			dbent.Asc(financetransaction.FieldID),
		}
	}

	if field == financetransaction.FieldID {
		return []func(*entsql.Selector){dbent.Desc(field)}
	}
	return []func(*entsql.Selector){
		dbent.Desc(field),
		dbent.Desc(financetransaction.FieldID),
	}
}

func applyFinanceTransactionEntityToService(dst *service.FinanceTransaction, src *dbent.FinanceTransaction) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func financeTransactionEntityToService(m *dbent.FinanceTransaction) *service.FinanceTransaction {
	if m == nil {
		return nil
	}
	return &service.FinanceTransaction{
		ID:             m.ID,
		Type:           m.Type,
		Category:       m.Category,
		AmountFen:      m.AmountFen,
		OccurredAt:     m.OccurredAt,
		Note:           m.Note,
		ReceiptKey:     m.ReceiptKey,
		PaymentChannel: m.PaymentChannel,
		Source:         m.Source,
		CreatedBy:      m.CreatedBy,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func financeTransactionEntitiesToService(models []*dbent.FinanceTransaction) []service.FinanceTransaction {
	out := make([]service.FinanceTransaction, 0, len(models))
	for i := range models {
		if s := financeTransactionEntityToService(models[i]); s != nil {
			out = append(out, *s)
		}
	}
	return out
}

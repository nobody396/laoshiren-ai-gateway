package dto

import (
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type FinanceTransaction struct {
	ID             int64     `json:"id"`
	Type           string    `json:"type"`
	Category       string    `json:"category"`
	AmountFen      int64     `json:"amount_fen"`
	OccurredAt     time.Time `json:"occurred_at"`
	Note           *string   `json:"note,omitempty"`
	ReceiptKey     *string   `json:"receipt_key,omitempty"`
	PaymentChannel *string   `json:"payment_channel,omitempty"`
	Source         string    `json:"source"`
	CreatedBy      *int64    `json:"created_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type FinanceCategoryTotal struct {
	Type     string `json:"type"`
	Category string `json:"category"`
	TotalFen int64  `json:"total_fen"`
	TxCount  int64  `json:"tx_count"`
}

type FinanceMonthlyTotal struct {
	Month           string `json:"month"`
	TotalIncomeFen  int64  `json:"total_income_fen"`
	TotalExpenseFen int64  `json:"total_expense_fen"`
	NetProfitFen    int64  `json:"net_profit_fen"`
}

type FinanceTransactionSummary struct {
	RangeFrom       time.Time              `json:"range_from"`
	RangeTo         time.Time              `json:"range_to"`
	TotalIncomeFen  int64                  `json:"total_income_fen"`
	TotalExpenseFen int64                  `json:"total_expense_fen"`
	NetProfitFen    int64                  `json:"net_profit_fen"`
	MarginPercent   float64                `json:"margin_percent"`
	ByCategory      []FinanceCategoryTotal `json:"by_category"`
	MonthlySeries   []FinanceMonthlyTotal  `json:"monthly_series"`
}

func FinanceTransactionFromService(t *service.FinanceTransaction) *FinanceTransaction {
	if t == nil {
		return nil
	}
	return &FinanceTransaction{
		ID:             t.ID,
		Type:           t.Type,
		Category:       t.Category,
		AmountFen:      t.AmountFen,
		OccurredAt:     t.OccurredAt,
		Note:           t.Note,
		ReceiptKey:     t.ReceiptKey,
		PaymentChannel: t.PaymentChannel,
		Source:         t.Source,
		CreatedBy:      t.CreatedBy,
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
	}
}

func FinanceTransactionSummaryFromService(s *service.FinanceTransactionSummary) *FinanceTransactionSummary {
	if s == nil {
		return nil
	}
	byCategory := make([]FinanceCategoryTotal, 0, len(s.ByCategory))
	for _, c := range s.ByCategory {
		byCategory = append(byCategory, FinanceCategoryTotal{
			Type:     c.Type,
			Category: c.Category,
			TotalFen: c.TotalFen,
			TxCount:  c.TxCount,
		})
	}
	monthlySeries := make([]FinanceMonthlyTotal, 0, len(s.MonthlySeries))
	for _, month := range s.MonthlySeries {
		monthlySeries = append(monthlySeries, FinanceMonthlyTotal{
			Month:           month.Month,
			TotalIncomeFen:  month.TotalIncomeFen,
			TotalExpenseFen: month.TotalExpenseFen,
			NetProfitFen:    month.NetProfitFen,
		})
	}
	return &FinanceTransactionSummary{
		RangeFrom:       s.RangeFrom,
		RangeTo:         s.RangeTo,
		TotalIncomeFen:  s.TotalIncomeFen,
		TotalExpenseFen: s.TotalExpenseFen,
		NetProfitFen:    s.NetProfitFen,
		MarginPercent:   s.MarginPercent,
		ByCategory:      byCategory,
		MonthlySeries:   monthlySeries,
	}
}

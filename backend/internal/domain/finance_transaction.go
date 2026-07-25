package domain

import (
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

const (
	FinanceTransactionTypeIncome  = "income"
	FinanceTransactionTypeExpense = "expense"
)

const (
	// 收入分类
	FinanceTransactionCategorySaleRevenue = "sale_revenue"
	FinanceTransactionCategoryOtherIncome = "other_income"

	// 支出分类
	FinanceTransactionCategoryUpstreamTopup = "upstream_topup"
	FinanceTransactionCategoryServerCost    = "server_cost" // legacy; retained for old rows
	FinanceTransactionCategoryHostingCost   = "hosting_cost"
	FinanceTransactionCategoryCDNCost       = "cdn_cost"
	FinanceTransactionCategoryDomainCost    = "domain_cost"
	FinanceTransactionCategoryEarlyCost     = "early_cost"
	FinanceTransactionCategoryOtherExpense  = "other_expense"
)

const (
	FinancePaymentChannelWechat       = "wechat"
	FinancePaymentChannelAlipay       = "alipay"
	FinancePaymentChannelBankTransfer = "bank_transfer"
	FinancePaymentChannelOther        = "other"
)

const (
	FinanceTransactionSourceManual = "manual"
	FinanceTransactionSourceSkill  = "skill"
)

var (
	ErrFinanceTransactionNotFound       = infraerrors.NotFound("FINANCE_TRANSACTION_NOT_FOUND", "finance transaction not found")
	ErrFinanceTransactionInvalidType    = infraerrors.BadRequest("FINANCE_TRANSACTION_INVALID_TYPE", "invalid finance transaction type")
	ErrFinanceTransactionInvalidCat     = infraerrors.BadRequest("FINANCE_TRANSACTION_INVALID_CATEGORY", "invalid finance transaction category for the given type")
	ErrFinanceTransactionInvalidAmount  = infraerrors.BadRequest("FINANCE_TRANSACTION_INVALID_AMOUNT", "amount_fen must be positive")
	ErrFinanceTransactionInvalidSource  = infraerrors.BadRequest("FINANCE_TRANSACTION_INVALID_SOURCE", "invalid finance transaction source")
	ErrFinanceTransactionInvalidChannel = infraerrors.BadRequest(
		"FINANCE_TRANSACTION_INVALID_PAYMENT_CHANNEL",
		"invalid finance transaction payment channel",
	)
	ErrFinanceTransactionIncomeReceiptRequired = infraerrors.BadRequest(
		"FINANCE_TRANSACTION_INCOME_RECEIPT_REQUIRED",
		"income transactions require a receipt",
	)
	ErrFinanceTransactionIncomeChannelRequired = infraerrors.BadRequest(
		"FINANCE_TRANSACTION_INCOME_PAYMENT_CHANNEL_REQUIRED",
		"income transactions require a payment channel",
	)
)

// financeIncomeCategories / financeExpenseCategories 限定每种 type 下允许的 category 取值。
var financeIncomeCategories = map[string]struct{}{
	FinanceTransactionCategorySaleRevenue: {},
	FinanceTransactionCategoryOtherIncome: {},
}

var financeExpenseCategories = map[string]struct{}{
	FinanceTransactionCategoryUpstreamTopup: {},
	FinanceTransactionCategoryServerCost:    {},
	FinanceTransactionCategoryHostingCost:   {},
	FinanceTransactionCategoryCDNCost:       {},
	FinanceTransactionCategoryDomainCost:    {},
	FinanceTransactionCategoryEarlyCost:     {},
	FinanceTransactionCategoryOtherExpense:  {},
}

func IsValidFinanceTransactionType(t string) bool {
	switch t {
	case FinanceTransactionTypeIncome, FinanceTransactionTypeExpense:
		return true
	default:
		return false
	}
}

func IsValidFinanceTransactionSource(s string) bool {
	switch s {
	case FinanceTransactionSourceManual, FinanceTransactionSourceSkill:
		return true
	default:
		return false
	}
}

func IsValidFinancePaymentChannel(channel string) bool {
	switch channel {
	case FinancePaymentChannelWechat,
		FinancePaymentChannelAlipay,
		FinancePaymentChannelBankTransfer,
		FinancePaymentChannelOther:
		return true
	default:
		return false
	}
}

// IsValidFinanceTransactionCategory 校验 category 是否属于给定 type 的允许集合。
func IsValidFinanceTransactionCategory(transactionType, category string) bool {
	switch transactionType {
	case FinanceTransactionTypeIncome:
		_, ok := financeIncomeCategories[category]
		return ok
	case FinanceTransactionTypeExpense:
		_, ok := financeExpenseCategories[category]
		return ok
	default:
		return false
	}
}

type FinanceTransaction struct {
	ID             int64
	Type           string
	Category       string
	AmountFen      int64
	OccurredAt     time.Time
	Note           *string
	ReceiptKey     *string
	PaymentChannel *string
	Source         string
	CreatedBy      *int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SignedAmountFen 返回带方向的金额：收入为正，支出为负，方便直接求和算净利润。
func (t *FinanceTransaction) SignedAmountFen() int64 {
	if t == nil {
		return 0
	}
	if t.Type == FinanceTransactionTypeExpense {
		return -t.AmountFen
	}
	return t.AmountFen
}

// FinanceCategoryTotal 是按分类聚合后的金额小计。
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

// FinanceTransactionSummary 是一段时间范围内的营收/成本汇总。
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

package service

import (
	"context"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

const (
	FinanceTransactionTypeIncome  = domain.FinanceTransactionTypeIncome
	FinanceTransactionTypeExpense = domain.FinanceTransactionTypeExpense
)

const (
	FinanceTransactionCategorySaleRevenue   = domain.FinanceTransactionCategorySaleRevenue
	FinanceTransactionCategoryOtherIncome   = domain.FinanceTransactionCategoryOtherIncome
	FinanceTransactionCategoryUpstreamTopup = domain.FinanceTransactionCategoryUpstreamTopup
	FinanceTransactionCategoryServerCost    = domain.FinanceTransactionCategoryServerCost
	FinanceTransactionCategoryHostingCost   = domain.FinanceTransactionCategoryHostingCost
	FinanceTransactionCategoryCDNCost       = domain.FinanceTransactionCategoryCDNCost
	FinanceTransactionCategoryDomainCost    = domain.FinanceTransactionCategoryDomainCost
	FinanceTransactionCategoryEarlyCost     = domain.FinanceTransactionCategoryEarlyCost
	FinanceTransactionCategoryOtherExpense  = domain.FinanceTransactionCategoryOtherExpense
)

const (
	FinanceTransactionSourceManual = domain.FinanceTransactionSourceManual
	FinanceTransactionSourceSkill  = domain.FinanceTransactionSourceSkill
)

var (
	ErrFinanceTransactionNotFound              = domain.ErrFinanceTransactionNotFound
	ErrFinanceTransactionInvalidType           = domain.ErrFinanceTransactionInvalidType
	ErrFinanceTransactionInvalidCat            = domain.ErrFinanceTransactionInvalidCat
	ErrFinanceTransactionInvalidAmount         = domain.ErrFinanceTransactionInvalidAmount
	ErrFinanceTransactionInvalidSource         = domain.ErrFinanceTransactionInvalidSource
	ErrFinanceTransactionInvalidChannel        = domain.ErrFinanceTransactionInvalidChannel
	ErrFinanceTransactionIncomeReceiptRequired = domain.ErrFinanceTransactionIncomeReceiptRequired
	ErrFinanceTransactionIncomeChannelRequired = domain.ErrFinanceTransactionIncomeChannelRequired
)

type FinanceTransaction = domain.FinanceTransaction

type FinanceCategoryTotal = domain.FinanceCategoryTotal

type FinanceMonthlyTotal = domain.FinanceMonthlyTotal

type FinanceTransactionSummary = domain.FinanceTransactionSummary

type FinanceTransactionListFilters struct {
	Type     string
	Category string
	From     *time.Time
	To       *time.Time
	Search   string
}

type FinanceTransactionRepository interface {
	Create(ctx context.Context, t *FinanceTransaction) error
	GetByID(ctx context.Context, id int64) (*FinanceTransaction, error)
	Update(ctx context.Context, t *FinanceTransaction) error
	Delete(ctx context.Context, id int64) error

	List(ctx context.Context, params pagination.PaginationParams, filters FinanceTransactionListFilters) ([]FinanceTransaction, *pagination.PaginationResult, error)
	Summary(ctx context.Context, from, to time.Time) (*FinanceTransactionSummary, error)
	SummaryAll(ctx context.Context, to time.Time) (*FinanceTransactionSummary, error)
}

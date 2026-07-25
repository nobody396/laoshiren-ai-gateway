package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

type FinanceTransactionService struct {
	repo FinanceTransactionRepository
}

func NewFinanceTransactionService(repo FinanceTransactionRepository) *FinanceTransactionService {
	return &FinanceTransactionService{repo: repo}
}

type CreateFinanceTransactionInput struct {
	Type       string
	Category   string
	AmountFen  int64
	OccurredAt time.Time
	Note       *string
	ReceiptKey *string
	Source     string
	ActorID    *int64 // 管理员用户ID
}

type UpdateFinanceTransactionInput struct {
	Type       *string
	Category   *string
	AmountFen  *int64
	OccurredAt *time.Time
	Note       **string
	ReceiptKey **string
}

func (s *FinanceTransactionService) Create(ctx context.Context, input *CreateFinanceTransactionInput) (*FinanceTransaction, error) {
	if input == nil {
		return nil, fmt.Errorf("create finance transaction: nil input")
	}

	txType := strings.TrimSpace(input.Type)
	if !domain.IsValidFinanceTransactionType(txType) {
		return nil, ErrFinanceTransactionInvalidType
	}

	category := strings.TrimSpace(input.Category)
	if !domain.IsValidFinanceTransactionCategory(txType, category) {
		return nil, ErrFinanceTransactionInvalidCat
	}

	if input.AmountFen <= 0 {
		return nil, ErrFinanceTransactionInvalidAmount
	}

	source := strings.TrimSpace(input.Source)
	if source == "" {
		source = FinanceTransactionSourceManual
	}
	if !domain.IsValidFinanceTransactionSource(source) {
		return nil, ErrFinanceTransactionInvalidSource
	}

	occurredAt := input.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now()
	}

	note := trimOptionalString(input.Note)
	receiptKey := trimOptionalString(input.ReceiptKey)

	t := &FinanceTransaction{
		Type:       txType,
		Category:   category,
		AmountFen:  input.AmountFen,
		OccurredAt: occurredAt,
		Note:       note,
		ReceiptKey: receiptKey,
		Source:     source,
	}
	if input.ActorID != nil && *input.ActorID > 0 {
		t.CreatedBy = input.ActorID
	}

	if err := s.repo.Create(ctx, t); err != nil {
		return nil, fmt.Errorf("create finance transaction: %w", err)
	}
	return t, nil
}

func (s *FinanceTransactionService) Update(ctx context.Context, id int64, input *UpdateFinanceTransactionInput) (*FinanceTransaction, error) {
	if input == nil {
		return nil, fmt.Errorf("update finance transaction: nil input")
	}

	t, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	newType := t.Type
	if input.Type != nil {
		newType = strings.TrimSpace(*input.Type)
		if !domain.IsValidFinanceTransactionType(newType) {
			return nil, ErrFinanceTransactionInvalidType
		}
		t.Type = newType
	}

	if input.Category != nil {
		category := strings.TrimSpace(*input.Category)
		if !domain.IsValidFinanceTransactionCategory(newType, category) {
			return nil, ErrFinanceTransactionInvalidCat
		}
		t.Category = category
	} else if input.Type != nil && !domain.IsValidFinanceTransactionCategory(newType, t.Category) {
		// type 变更但没有同时给出新 category：旧 category 不再合法，拒绝更新。
		return nil, ErrFinanceTransactionInvalidCat
	}

	if input.AmountFen != nil {
		if *input.AmountFen <= 0 {
			return nil, ErrFinanceTransactionInvalidAmount
		}
		t.AmountFen = *input.AmountFen
	}

	if input.OccurredAt != nil {
		t.OccurredAt = *input.OccurredAt
	}

	if input.Note != nil {
		t.Note = trimOptionalString(*input.Note)
	}

	if input.ReceiptKey != nil {
		t.ReceiptKey = trimOptionalString(*input.ReceiptKey)
	}

	if err := s.repo.Update(ctx, t); err != nil {
		return nil, fmt.Errorf("update finance transaction: %w", err)
	}
	return t, nil
}

func (s *FinanceTransactionService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete finance transaction: %w", err)
	}
	return nil
}

func (s *FinanceTransactionService) GetByID(ctx context.Context, id int64) (*FinanceTransaction, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *FinanceTransactionService) List(ctx context.Context, params pagination.PaginationParams, filters FinanceTransactionListFilters) ([]FinanceTransaction, *pagination.PaginationResult, error) {
	return s.repo.List(ctx, params, filters)
}

// Summary 汇总 [from, to) 区间内的真实收支：营收、成本、净利润、利润率、按分类小计。
func (s *FinanceTransactionService) Summary(ctx context.Context, from, to time.Time) (*FinanceTransactionSummary, error) {
	if !from.Before(to) {
		return nil, fmt.Errorf("summary finance transactions: from must be before to")
	}
	return s.repo.Summary(ctx, from, to)
}

func trimOptionalString(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

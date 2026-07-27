package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

var (
	ErrAffiliateWalletNotAvailable = infraerrors.Forbidden(
		"AFFILIATE_WALLET_NOT_AVAILABLE",
		"affiliate cash wallet is not available",
	)
	ErrAffiliateInsufficientCash = infraerrors.Conflict(
		"AFFILIATE_INSUFFICIENT_CASH",
		"available affiliate cash is insufficient",
	)
	ErrAffiliateWithdrawalMinimum = infraerrors.Conflict(
		"AFFILIATE_WITHDRAWAL_MINIMUM",
		"withdrawal amount is below the configured minimum",
	)
	ErrAffiliatePaymentNotVerified = infraerrors.Conflict(
		"AFFILIATE_PAYMENT_NOT_VERIFIED",
		"verified Alipay payment profile is required",
	)
	ErrAffiliateWithdrawalNotProcessing = infraerrors.Conflict(
		"AFFILIATE_WITHDRAWAL_NOT_PROCESSING",
		"withdrawal is no longer processing",
	)
	ErrAffiliateWithdrawalNotFound = infraerrors.NotFound(
		"AFFILIATE_WITHDRAWAL_NOT_FOUND",
		"withdrawal request not found",
	)
	ErrAffiliateWithdrawalAlreadyProcessing = infraerrors.Conflict(
		"AFFILIATE_WITHDRAWAL_ALREADY_PROCESSING",
		"another affiliate withdrawal is already processing",
	)
	ErrAffiliateNoticeNotFound = infraerrors.NotFound(
		"AFFILIATE_NOTICE_NOT_FOUND",
		"affiliate notice not found",
	)
	ErrAffiliateIdempotencyKeyRequired = infraerrors.BadRequest(
		"AFFILIATE_IDEMPOTENCY_KEY_REQUIRED",
		"Idempotency-Key is required",
	)
)

type AffiliateWalletSummary struct {
	AgentID                    int64  `json:"agent_id"`
	AvailableCashMicros        int64  `json:"available_cash_micros"`
	ProcessingWithdrawalMicros int64  `json:"processing_withdrawal_micros"`
	LifetimeEarnedMicros       int64  `json:"lifetime_earned_micros"`
	WithdrawalMinimumMicros    int64  `json:"withdrawal_minimum_micros"`
	WithdrawalSLAHours         int32  `json:"withdrawal_sla_hours"`
	ConversionMultiplierMillis int32  `json:"conversion_multiplier_millis"`
	PaymentProfileVerified     bool   `json:"payment_profile_verified"`
	CanWithdraw                bool   `json:"can_withdraw"`
	CashAssetSymbol            string `json:"cash_asset_symbol"`
	CreditAssetSymbol          string `json:"credit_asset_symbol"`
	DisplayTimezone            string `json:"display_timezone"`
}

type AffiliateWithdrawal struct {
	ID                    int64      `json:"id"`
	AgentID               int64      `json:"agent_id"`
	AmountMicros          int64      `json:"amount_micros"`
	Status                string     `json:"status"`
	AgentRiskStatus       string     `json:"agent_risk_status"`
	PaymentAlipayRealName string     `json:"payment_alipay_real_name,omitempty"`
	PaymentAlipayAccount  string     `json:"payment_alipay_account,omitempty"`
	PaymentContactPhone   string     `json:"payment_contact_phone,omitempty"`
	PaymentNote           string     `json:"payment_note,omitempty"`
	PaymentQRObjectKey    string     `json:"-"`
	PaymentQRContentType  string     `json:"-"`
	PaymentQROriginalName string     `json:"-"`
	RequestedAt           time.Time  `json:"requested_at"`
	DueAt                 time.Time  `json:"due_at"`
	PaidAt                *time.Time `json:"paid_at,omitempty"`
	FailedAt              *time.Time `json:"failed_at,omitempty"`
	HandledBy             *int64     `json:"handled_by,omitempty"`
	PaymentReference      string     `json:"payment_reference,omitempty"`
	FailureReason         string     `json:"failure_reason,omitempty"`
}

type AffiliateCommissionConversion struct {
	ID                 int64     `json:"id"`
	AgentID            int64     `json:"agent_id"`
	CashAmountMicros   int64     `json:"cash_amount_micros"`
	CreditAmountMicros int64     `json:"credit_amount_micros"`
	MultiplierMillis   int32     `json:"multiplier_millis"`
	CreatedAt          time.Time `json:"created_at"`
}

type AffiliateAgentNotice struct {
	ID         int64      `json:"id"`
	NoticeType string     `json:"notice_type"`
	Title      string     `json:"title"`
	Message    string     `json:"message"`
	SourceType string     `json:"source_type"`
	SourceID   *int64     `json:"source_id,omitempty"`
	ReadAt     *time.Time `json:"read_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type AffiliateWalletRepository interface {
	GetAffiliateWallet(ctx context.Context, agentID int64) (*AffiliateWalletSummary, error)
	ListAffiliateWithdrawals(ctx context.Context, agentID int64, includeInternal bool, limit int) ([]AffiliateWithdrawal, error)
	CreateAffiliateWithdrawal(ctx context.Context, agentID, amountMicros int64, idempotencyKey string) (*AffiliateWithdrawal, error)
	CompleteAffiliateWithdrawal(ctx context.Context, withdrawalID, operatorID int64, paymentReference string) (*AffiliateWithdrawal, error)
	FailAffiliateWithdrawal(ctx context.Context, withdrawalID, operatorID int64, reason string) (*AffiliateWithdrawal, error)
	GetAffiliateWithdrawal(ctx context.Context, withdrawalID int64) (*AffiliateWithdrawal, error)
	ListProcessingAffiliateWithdrawals(ctx context.Context, limit int) ([]AffiliateWithdrawal, error)
	ConvertAffiliateCommission(ctx context.Context, agentID, amountMicros int64, idempotencyKey string) (*AffiliateCommissionConversion, error)
	ListAffiliateAgentNotices(ctx context.Context, agentID int64, limit int) ([]AffiliateAgentNotice, error)
	MarkAffiliateAgentNoticeRead(ctx context.Context, agentID, noticeID int64) error
}

type affiliateWithdrawalQRCodeAccessAuditor interface {
	RecordAffiliateWithdrawalQRCodeAccess(
		ctx context.Context,
		withdrawalID int64,
		accessorUserID int64,
	) error
}

type AffiliateWalletService struct {
	repo         AffiliateWalletRepository
	balanceCache interface {
		InvalidateUserBalance(ctx context.Context, userID int64) error
	}
}

func NewAffiliateWalletService(repo AffiliateWalletRepository) *AffiliateWalletService {
	return &AffiliateWalletService{repo: repo}
}

func (s *AffiliateWalletService) SetBalanceCache(cache *BillingCacheService) {
	if s != nil {
		s.balanceCache = cache
	}
}

func (s *AffiliateWalletService) GetWallet(ctx context.Context, agentID int64) (*AffiliateWalletSummary, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate wallet repository is not configured")
	}
	wallet, err := s.repo.GetAffiliateWallet(ctx, agentID)
	if err != nil {
		return nil, err
	}
	wallet.CashAssetSymbol = "¥"
	wallet.CreditAssetSymbol = "⚡"
	wallet.DisplayTimezone = "Asia/Shanghai"
	wallet.CanWithdraw =
		wallet.PaymentProfileVerified &&
			wallet.AvailableCashMicros >= wallet.WithdrawalMinimumMicros
	return wallet, nil
}

func (s *AffiliateWalletService) ListWithdrawals(
	ctx context.Context,
	agentID int64,
) ([]AffiliateWithdrawal, error) {
	return s.repo.ListAffiliateWithdrawals(ctx, agentID, false, 100)
}

func (s *AffiliateWalletService) RequestWithdrawal(
	ctx context.Context,
	agentID, amountMicros int64,
	idempotencyKey string,
) (*AffiliateWithdrawal, error) {
	if err := validateAffiliateIdempotencyKey(idempotencyKey); err != nil {
		return nil, err
	}
	if amountMicros < 0 {
		return nil, ErrInvalidInput
	}
	return s.repo.CreateAffiliateWithdrawal(
		ctx,
		agentID,
		amountMicros,
		strings.TrimSpace(idempotencyKey),
	)
}

func (s *AffiliateWalletService) Convert(
	ctx context.Context,
	agentID, amountMicros int64,
	idempotencyKey string,
) (*AffiliateCommissionConversion, error) {
	if err := validateAffiliateIdempotencyKey(idempotencyKey); err != nil {
		return nil, err
	}
	if amountMicros < 0 {
		return nil, ErrInvalidInput
	}
	conversion, err := s.repo.ConvertAffiliateCommission(
		ctx,
		agentID,
		amountMicros,
		strings.TrimSpace(idempotencyKey),
	)
	if err != nil {
		return nil, err
	}
	if s.balanceCache != nil {
		if err := s.balanceCache.InvalidateUserBalance(ctx, agentID); err != nil {
			slog.Error("invalidate converted affiliate credit balance failed", "agent_id", agentID, "error", err)
		}
	}
	return conversion, nil
}

func (s *AffiliateWalletService) ListProcessing(
	ctx context.Context,
	limit int,
) ([]AffiliateWithdrawal, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.repo.ListProcessingAffiliateWithdrawals(ctx, limit)
}

func (s *AffiliateWalletService) CompleteWithdrawal(
	ctx context.Context,
	withdrawalID, operatorID int64,
	paymentReference string,
) (*AffiliateWithdrawal, error) {
	paymentReference = strings.TrimSpace(paymentReference)
	if withdrawalID <= 0 || operatorID <= 0 || len([]rune(paymentReference)) > 200 {
		return nil, ErrInvalidInput
	}
	return s.repo.CompleteAffiliateWithdrawal(ctx, withdrawalID, operatorID, paymentReference)
}

func (s *AffiliateWalletService) FailWithdrawal(
	ctx context.Context,
	withdrawalID, operatorID int64,
	reason string,
) (*AffiliateWithdrawal, error) {
	reason = strings.TrimSpace(reason)
	if withdrawalID <= 0 || operatorID <= 0 || reason == "" || len([]rune(reason)) > 500 {
		return nil, ErrInvalidInput
	}
	return s.repo.FailAffiliateWithdrawal(ctx, withdrawalID, operatorID, reason)
}

func (s *AffiliateWalletService) GetWithdrawalQRCodeFile(
	ctx context.Context,
	withdrawalID int64,
	accessorUserID int64,
) (*AgentPaymentQRCodeFile, error) {
	if withdrawalID <= 0 || accessorUserID <= 0 {
		return nil, ErrInvalidInput
	}
	withdrawal, err := s.repo.GetAffiliateWithdrawal(ctx, withdrawalID)
	if err != nil {
		return nil, err
	}
	path, err := agentPaymentObjectPath(withdrawal.PaymentQRObjectKey)
	if err != nil {
		return nil, err
	}
	if auditor, ok := s.repo.(affiliateWithdrawalQRCodeAccessAuditor); ok {
		if err := auditor.RecordAffiliateWithdrawalQRCodeAccess(
			ctx,
			withdrawalID,
			accessorUserID,
		); err != nil {
			return nil, err
		}
	}
	return &AgentPaymentQRCodeFile{
		Path:        path,
		ContentType: withdrawal.PaymentQRContentType,
		Filename:    withdrawal.PaymentQROriginalName,
	}, nil
}

func (s *AffiliateWalletService) ListNotices(
	ctx context.Context,
	agentID int64,
) ([]AffiliateAgentNotice, error) {
	return s.repo.ListAffiliateAgentNotices(ctx, agentID, 100)
}

func (s *AffiliateWalletService) MarkNoticeRead(
	ctx context.Context,
	agentID, noticeID int64,
) error {
	if agentID <= 0 || noticeID <= 0 {
		return ErrInvalidInput
	}
	return s.repo.MarkAffiliateAgentNoticeRead(ctx, agentID, noticeID)
}

func validateAffiliateIdempotencyKey(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return ErrAffiliateIdempotencyKeyRequired
	}
	if len(value) > 180 {
		return fmt.Errorf("%w: key is too long", ErrInvalidInput)
	}
	return nil
}

func AffiliateMultiplyMicros(amount int64, multiplierMillis int32) (int64, error) {
	if amount <= 0 || multiplierMillis < 1000 {
		return 0, ErrInvalidInput
	}
	value := new(big.Int).Mul(big.NewInt(amount), big.NewInt(int64(multiplierMillis)))
	value.Quo(value, big.NewInt(1000))
	if !value.IsInt64() || value.Sign() <= 0 {
		return 0, ErrInvalidInput
	}
	return value.Int64(), nil
}

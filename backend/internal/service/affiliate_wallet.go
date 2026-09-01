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
		"合伙人钱包暂不可用，有疑问请联系客服。",
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
	ErrAffiliatePurchaseIdempotencyConflict = infraerrors.Conflict(
		"AFFILIATE_PURCHASE_IDEMPOTENCY_CONFLICT",
		"affiliate commission purchase idempotency key was already used with different purchase details",
	)
	ErrAffiliatePurchaseRefundConflict = infraerrors.Conflict(
		"AFFILIATE_PURCHASE_REFUND_CONFLICT",
		"affiliate commission purchase refund does not match the original purchase or prior refunds",
	)
	ErrAffiliatePurchaseNotFound = infraerrors.NotFound(
		"AFFILIATE_PURCHASE_NOT_FOUND",
		"affiliate commission purchase not found",
	)
	ErrAffiliateConversionDisabled = infraerrors.Conflict(
		"AFFILIATE_CONVERSION_DISABLED",
		"commission conversion is no longer available; use commission-wallet checkout or withdrawal",
	)
	ErrAffiliateWalletCheckoutDisabled = infraerrors.Conflict(
		"AFFILIATE_WALLET_CHECKOUT_DISABLED",
		"commission-wallet checkout is not available",
	)
)

const (
	AffiliateCommissionPurchaseKindMonthlyCard = "monthly_card"
	AffiliateCommissionPurchaseKindBalance     = "balance"
	AffiliateCommissionMachineOperatorID       = int64(-1)
)

type AffiliateBalancePurchase struct {
	LedgerEntryID       int64     `json:"ledger_entry_id"`
	AgentID             int64     `json:"agent_id"`
	CashAmountMicros    int64     `json:"cash_amount_micros"`
	CreditAmountMicros  int64     `json:"credit_amount_micros"`
	RateBPS             int32     `json:"rate_bps"`
	RemainingCashMicros int64     `json:"remaining_cash_micros"`
	CreatedAt           time.Time `json:"created_at"`
}

type AffiliateWalletSummary struct {
	AgentID                    int64  `json:"agent_id"`
	AvailableCashMicros        int64  `json:"available_cash_micros"`
	ProcessingWithdrawalMicros int64  `json:"processing_withdrawal_micros"`
	LifetimeEarnedMicros       int64  `json:"lifetime_earned_micros"`
	WithdrawalMinimumMicros    int64  `json:"withdrawal_minimum_micros"`
	WithdrawalSLAHours         int32  `json:"withdrawal_sla_hours"`
	ConversionMultiplierMillis int32  `json:"conversion_multiplier_millis"`
	WalletCheckoutEnabled      bool   `json:"wallet_checkout_enabled"`
	WalletPurchaseRateBPS      int32  `json:"wallet_purchase_rate_bps"`
	ConversionEnabled          bool   `json:"conversion_enabled"`
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

type AffiliateCommissionPurchase struct {
	ID                  int64     `json:"id"`
	AgentID             int64     `json:"agent_id"`
	AmountMicros        int64     `json:"amount_micros"`
	PurchaseKind        string    `json:"purchase_kind"`
	ProductCode         string    `json:"product_code"`
	ExternalReference   string    `json:"external_reference"`
	Note                string    `json:"note,omitempty"`
	OperatorID          int64     `json:"operator_id"`
	RemainingCashMicros int64     `json:"remaining_cash_micros"`
	CreatedAt           time.Time `json:"created_at"`
}

type AffiliateCommissionPurchaseRefund struct {
	ID                    int64     `json:"id"`
	AgentID               int64     `json:"agent_id"`
	PurchaseEntryID       int64     `json:"purchase_entry_id"`
	OriginalAmountMicros  int64     `json:"original_amount_micros"`
	RateBPS               int32     `json:"rate_bps"`
	PayableAmountMicros   int64     `json:"payable_amount_micros"`
	RefundAmountMicros    int64     `json:"refund_amount_micros"`
	RemainingCashMicros   int64     `json:"remaining_cash_micros"`
	NotificationID        int64     `json:"notification_id"`
	NotificationDedupeKey string    `json:"notification_dedupe_key"`
	CreatedAt             time.Time `json:"created_at"`
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
	ListAdminAffiliateWithdrawals(ctx context.Context, status string, limit int) ([]AffiliateWithdrawal, error)
	ConvertAffiliateCommission(ctx context.Context, agentID, amountMicros int64, idempotencyKey string) (*AffiliateCommissionConversion, error)
	PurchaseWithAffiliateCommission(ctx context.Context, agentID, amountMicros, operatorID int64, purchaseKind, productCode, externalReference, note, idempotencyKey string) (*AffiliateCommissionPurchase, error)
	RefundAffiliateCommissionPurchase(ctx context.Context, agentID, purchaseEntryID, expectedOriginalAmountMicros, expectedRefundAmountMicros, operatorID int64, rateBPS int32, note, idempotencyKey string) (*AffiliateCommissionPurchaseRefund, error)
	PurchaseBalanceWithAffiliateCommission(ctx context.Context, agentID, creditAmountCNYFen int64, idempotencyKey string) (*AffiliateBalancePurchase, error)
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

func (s *AffiliateWalletService) Purchase(
	ctx context.Context,
	agentID, amountMicros, operatorID int64,
	purchaseKind, productCode, externalReference, note, idempotencyKey string,
) (*AffiliateCommissionPurchase, error) {
	if err := validateAffiliateIdempotencyKey(idempotencyKey); err != nil {
		return nil, err
	}
	purchaseKind = strings.TrimSpace(purchaseKind)
	productCode = strings.ToLower(strings.TrimSpace(productCode))
	externalReference = strings.TrimSpace(externalReference)
	note = strings.TrimSpace(note)
	// Admin API-key authentication uses the deterministic service principal -1.
	// Other negative values stay invalid; zero is retained for compatibility
	// with internal callers that do not attach a human operator.
	if agentID <= 0 || amountMicros <= 0 ||
		(operatorID < 0 && operatorID != AffiliateCommissionMachineOperatorID) ||
		purchaseKind != AffiliateCommissionPurchaseKindMonthlyCard ||
		productCode == "" || len(productCode) > 64 ||
		externalReference == "" || len(externalReference) > 180 ||
		len(note) > 500 {
		return nil, ErrInvalidInput
	}
	switch productCode {
	case "plus", "pro", "max":
	default:
		return nil, ErrInvalidInput
	}
	return s.repo.PurchaseWithAffiliateCommission(
		ctx,
		agentID,
		amountMicros,
		operatorID,
		purchaseKind,
		productCode,
		externalReference,
		note,
		strings.TrimSpace(idempotencyKey),
	)
}

func (s *AffiliateWalletService) RefundPurchase(
	ctx context.Context,
	agentID, purchaseEntryID, expectedOriginalAmountMicros, expectedRefundAmountMicros, operatorID int64,
	rateBPS int32,
	note, idempotencyKey string,
) (*AffiliateCommissionPurchaseRefund, error) {
	if err := validateAffiliateIdempotencyKey(idempotencyKey); err != nil {
		return nil, err
	}
	note = strings.TrimSpace(note)
	if agentID <= 0 || purchaseEntryID <= 0 || expectedOriginalAmountMicros <= 0 ||
		expectedRefundAmountMicros <= 0 || rateBPS <= 0 || rateBPS >= 10_000 ||
		(operatorID < 0 && operatorID != AffiliateCommissionMachineOperatorID) ||
		len(note) > 500 {
		return nil, ErrInvalidInput
	}
	return s.repo.RefundAffiliateCommissionPurchase(
		ctx,
		agentID,
		purchaseEntryID,
		expectedOriginalAmountMicros,
		expectedRefundAmountMicros,
		operatorID,
		rateBPS,
		note,
		strings.TrimSpace(idempotencyKey),
	)
}

func (s *AffiliateWalletService) PurchaseBalance(
	ctx context.Context,
	agentID, creditAmountCNYFen int64,
	idempotencyKey string,
) (*AffiliateBalancePurchase, error) {
	if err := validateAffiliateIdempotencyKey(idempotencyKey); err != nil {
		return nil, err
	}
	if agentID <= 0 || creditAmountCNYFen < 2_000 || creditAmountCNYFen > 300_000 {
		return nil, ErrInvalidInput
	}
	result, err := s.repo.PurchaseBalanceWithAffiliateCommission(
		ctx, agentID, creditAmountCNYFen, strings.TrimSpace(idempotencyKey),
	)
	if err != nil {
		return nil, err
	}
	if s.balanceCache != nil {
		if err := s.balanceCache.InvalidateUserBalance(ctx, agentID); err != nil {
			slog.Error("invalidate commission-wallet balance purchase failed", "agent_id", agentID, "error", err)
		}
	}
	return result, nil
}

func (s *AffiliateWalletService) ListProcessing(
	ctx context.Context,
	limit int,
) ([]AffiliateWithdrawal, error) {
	return s.ListAdminWithdrawals(ctx, "processing", limit)
}

func (s *AffiliateWalletService) ListAdminWithdrawals(
	ctx context.Context,
	status string,
	limit int,
) ([]AffiliateWithdrawal, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		status = "processing"
	}
	switch status {
	case "processing", "paid", "failed", "all":
	default:
		return nil, ErrInvalidInput
	}
	return s.repo.ListAdminAffiliateWithdrawals(ctx, status, limit)
}

func (s *AffiliateWalletService) CompleteWithdrawal(
	ctx context.Context,
	withdrawalID, operatorID int64,
	paymentReference string,
) (*AffiliateWithdrawal, error) {
	paymentReference = strings.TrimSpace(paymentReference)
	if withdrawalID <= 0 ||
		operatorID <= 0 ||
		paymentReference == "" ||
		len([]rune(paymentReference)) > 200 {
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

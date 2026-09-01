package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type affiliateWalletRepository struct {
	db *sql.DB
}

func NewAffiliateWalletRepository(db *sql.DB) service.AffiliateWalletRepository {
	return &affiliateWalletRepository{db: db}
}

func (r *affiliateWalletRepository) GetAffiliateWallet(
	ctx context.Context,
	agentID int64,
) (*service.AffiliateWalletSummary, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("affiliate wallet repository db is nil")
	}
	out := &service.AffiliateWalletSummary{AgentID: agentID}
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COALESCE((
				SELECT SUM(amount_micros)
				FROM agent_cash_commission_entries
				WHERE agent_id = ap.agent_id
					AND posting_status = 'posted'
			), 0)::bigint AS available_cash_micros,
			COALESCE((
				SELECT SUM(amount_micros)
				FROM agent_withdrawal_requests
				WHERE agent_id = ap.agent_id
					AND status = 'processing'
			), 0)::bigint AS processing_withdrawal_micros,
			COALESCE((
				SELECT SUM(amount_micros)
				FROM agent_cash_commission_entries
				WHERE agent_id = ap.agent_id
					AND posting_status = 'posted'
					AND entry_type IN ('earned', 'risk_release', 'reversal')
			), 0)::bigint AS lifetime_earned_micros,
			s.withdrawal_min_micros,
			s.withdrawal_sla_hours,
			s.commission_conversion_multiplier_millis,
			s.commission_wallet_checkout_enabled,
			s.commission_wallet_purchase_rate_bps,
			s.commission_conversion_enabled,
			COALESCE(
				p.verification_status = 'verified'
					AND p.privacy_consent_version = $2
					AND p.privacy_consented_at IS NOT NULL,
				FALSE
			)
		FROM agent_principals ap
		CROSS JOIN affiliate_program_settings s
		LEFT JOIN agent_payment_profiles p ON p.agent_id = ap.agent_id
		WHERE ap.agent_id = $1
			AND ap.status = 'active'
			AND ap.risk_status = 'clear'
			AND s.id = 1
		`, agentID, service.AgentPaymentPrivacyNoticeVersion).Scan(
		&out.AvailableCashMicros,
		&out.ProcessingWithdrawalMicros,
		&out.LifetimeEarnedMicros,
		&out.WithdrawalMinimumMicros,
		&out.WithdrawalSLAHours,
		&out.ConversionMultiplierMillis,
		&out.WalletCheckoutEnabled,
		&out.WalletPurchaseRateBPS,
		&out.ConversionEnabled,
		&out.PaymentProfileVerified,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateWalletNotAvailable
	}
	return out, err
}

func (r *affiliateWalletRepository) ListAffiliateWithdrawals(
	ctx context.Context,
	agentID int64,
	includeInternal bool,
	limit int,
) ([]service.AffiliateWithdrawal, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	statusFilter := "AND status IN ('processing', 'paid')"
	if includeInternal {
		statusFilter = ""
	}
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT
			id, agent_id, amount_micros, status,
			COALESCE((
				SELECT risk_status
				FROM agent_principals
				WHERE agent_id = agent_withdrawal_requests.agent_id
			), 'blocked') AS agent_risk_status,
			payment_alipay_real_name, payment_alipay_account,
			payment_contact_phone, payment_note,
			payment_qr_object_key, payment_qr_content_type,
			payment_qr_original_filename,
			requested_at, due_at, paid_at, failed_at,
			handled_by, payment_reference, failure_reason
		FROM agent_withdrawal_requests
		WHERE agent_id = $1
			%s
		ORDER BY requested_at DESC, id DESC
		LIMIT $2
	`, statusFilter), agentID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.AffiliateWithdrawal, 0)
	for rows.Next() {
		item, err := scanAffiliateWithdrawal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *affiliateWalletRepository) CreateAffiliateWithdrawal(
	ctx context.Context,
	agentID, requestedMicros int64,
	idempotencyKey string,
) (_ *service.AffiliateWithdrawal, err error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockAffiliateAgent(ctx, tx, agentID); err != nil {
		return nil, err
	}
	existing, err := getAffiliateWithdrawalByIdempotency(ctx, tx, agentID, idempotencyKey)
	if err == nil {
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	var (
		principalStatus string
		riskStatus      string
		minimumMicros   int64
		slaHours        int32
	)
	err = tx.QueryRowContext(ctx, `
		SELECT
			ap.status,
			ap.risk_status,
			s.withdrawal_min_micros,
			s.withdrawal_sla_hours
		FROM agent_principals ap
		CROSS JOIN affiliate_program_settings s
		WHERE ap.agent_id = $1
			AND s.id = 1
		FOR UPDATE OF ap
	`, agentID).Scan(
		&principalStatus,
		&riskStatus,
		&minimumMicros,
		&slaHours,
	)
	if errors.Is(err, sql.ErrNoRows) ||
		principalStatus != "active" ||
		riskStatus != "clear" {
		return nil, service.ErrAffiliateWalletNotAvailable
	}
	if err != nil {
		return nil, err
	}

	var (
		profileStatus         string
		realName              string
		account               string
		phone                 string
		note                  string
		qrObjectKey           string
		qrContentType         string
		qrOriginalName        string
		privacyConsentVersion string
		privacyConsentedAt    sql.NullTime
	)
	err = tx.QueryRowContext(ctx, `
		SELECT
			verification_status,
			alipay_real_name,
			alipay_account,
			contact_phone,
			payment_note,
			alipay_qr_object_key,
			alipay_qr_content_type,
			alipay_qr_original_filename,
			privacy_consent_version,
			privacy_consented_at
		FROM agent_payment_profiles
		WHERE agent_id = $1
		FOR UPDATE
	`, agentID).Scan(
		&profileStatus,
		&realName,
		&account,
		&phone,
		&note,
		&qrObjectKey,
		&qrContentType,
		&qrOriginalName,
		&privacyConsentVersion,
		&privacyConsentedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliatePaymentNotVerified
	}
	if err != nil {
		return nil, err
	}
	if profileStatus != "verified" ||
		strings.TrimSpace(privacyConsentVersion) != service.AgentPaymentPrivacyNoticeVersion ||
		!privacyConsentedAt.Valid ||
		strings.TrimSpace(realName) == "" ||
		strings.TrimSpace(account) == "" ||
		strings.TrimSpace(qrObjectKey) == "" {
		return nil, service.ErrAffiliatePaymentNotVerified
	}
	var processingExists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM agent_withdrawal_requests
			WHERE agent_id = $1
				AND status = 'processing'
		)
	`, agentID).Scan(&processingExists); err != nil {
		return nil, err
	}
	if processingExists {
		return nil, service.ErrAffiliateWithdrawalAlreadyProcessing
	}

	availableMicros, err := lockAffiliateAvailableCash(ctx, tx, agentID)
	if err != nil {
		return nil, err
	}
	amountMicros := requestedMicros
	if amountMicros == 0 {
		amountMicros = availableMicros
	}
	if amountMicros < minimumMicros {
		return nil, service.ErrAffiliateWithdrawalMinimum
	}
	if amountMicros > availableMicros {
		return nil, service.ErrAffiliateInsufficientCash
	}

	var withdrawalID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO agent_withdrawal_requests (
			agent_id, amount_micros, status, idempotency_key,
			payment_alipay_real_name, payment_alipay_account,
			payment_contact_phone, payment_note,
			payment_qr_object_key, payment_qr_content_type,
			payment_qr_original_filename,
			requested_at, due_at
		)
		VALUES (
			$1, $2, 'processing', $3,
			$4, $5, $6, $7, $8, $9, $10,
			NOW(), NOW() + make_interval(hours => $11::integer)
		)
		RETURNING id
	`, agentID, amountMicros, idempotencyKey,
		realName, account, phone, note,
		qrObjectKey, qrContentType, qrOriginalName,
		slaHours,
	).Scan(&withdrawalID)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_withdrawal_events (
			withdrawal_id, agent_id, event_type,
			previous_status, next_status,
			metadata
		)
		VALUES (
			$1, $2, 'requested',
			NULL, 'processing',
			jsonb_build_object('timezone', 'Asia/Shanghai')
		)
		ON CONFLICT (withdrawal_id, event_type) DO NOTHING
	`, withdrawalID, agentID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, entry_type, amount_micros,
			posting_status, source_type, source_id,
			idempotency_key, metadata, occurred_at
		)
		VALUES (
			$1, 'withdrawal_hold', -($2::bigint),
			'posted', 'withdrawal_request', $3,
			$4, jsonb_build_object('asset_symbol', '¥'), NOW()
		)
	`, agentID, amountMicros, withdrawalID, fmt.Sprintf("withdrawal:%d:hold", withdrawalID)); err != nil {
		return nil, err
	}
	item, err := getAffiliateWithdrawalByID(ctx, tx, withdrawalID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *affiliateWalletRepository) CompleteAffiliateWithdrawal(
	ctx context.Context,
	withdrawalID, operatorID int64,
	paymentReference string,
) (*service.AffiliateWithdrawal, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := getAffiliateWithdrawalByIDForUpdate(ctx, tx, withdrawalID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateWithdrawalNotFound
	}
	if err != nil {
		return nil, err
	}
	if item.Status == "paid" {
		return item, tx.Commit()
	}
	if item.Status != "processing" {
		return nil, service.ErrAffiliateWithdrawalNotProcessing
	}
	var riskStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT risk_status
		FROM agent_principals
		WHERE agent_id = $1
		FOR SHARE
	`, item.AgentID).Scan(&riskStatus); err != nil {
		return nil, err
	}
	if riskStatus != service.AffiliateRiskStatusClear {
		return nil, service.ErrAffiliateWalletNotAvailable
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_withdrawal_requests
		SET status = 'paid',
			paid_at = NOW(),
			handled_by = $1,
			payment_reference = $2,
			updated_at = NOW()
		WHERE id = $3
	`, operatorID, paymentReference, withdrawalID); err != nil {
		if isPostgresUniqueViolation(err) {
			return nil, service.ErrInvalidInput
		}
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_withdrawal_events (
			withdrawal_id, agent_id, event_type,
			previous_status, next_status,
			operator_id, note, metadata
		)
		VALUES (
			$1, $2, 'paid',
			'processing', 'paid',
			$3, $4, jsonb_build_object('timezone', 'Asia/Shanghai')
		)
		ON CONFLICT (withdrawal_id, event_type) DO NOTHING
	`, withdrawalID, item.AgentID, operatorID, paymentReference); err != nil {
		return nil, err
	}
	if err := insertAffiliateAgentNotice(
		ctx, tx, item.AgentID,
		"withdrawal_paid",
		"提现已到账",
		"您的提现申请已处理完成，请查收支付宝。",
		"withdrawal",
		withdrawalID,
		fmt.Sprintf("withdrawal:%d:paid-notice", withdrawalID),
	); err != nil {
		return nil, err
	}
	item, err = getAffiliateWithdrawalByID(ctx, tx, withdrawalID)
	if err != nil {
		return nil, err
	}
	return item, tx.Commit()
}

func (r *affiliateWalletRepository) FailAffiliateWithdrawal(
	ctx context.Context,
	withdrawalID, operatorID int64,
	reason string,
) (*service.AffiliateWithdrawal, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := getAffiliateWithdrawalByIDForUpdate(ctx, tx, withdrawalID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateWithdrawalNotFound
	}
	if err != nil {
		return nil, err
	}
	if item.Status == "failed" {
		return item, tx.Commit()
	}
	if item.Status != "processing" {
		return nil, service.ErrAffiliateWithdrawalNotProcessing
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE agent_withdrawal_requests
		SET status = 'failed',
			failed_at = NOW(),
			handled_by = $1,
			failure_reason = $2,
			updated_at = NOW()
		WHERE id = $3
	`, operatorID, reason, withdrawalID); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_withdrawal_events (
			withdrawal_id, agent_id, event_type,
			previous_status, next_status,
			operator_id, note, metadata
		)
		VALUES (
			$1, $2, 'failed',
			'processing', 'failed',
			$3, $4, jsonb_build_object('timezone', 'Asia/Shanghai')
		)
		ON CONFLICT (withdrawal_id, event_type) DO NOTHING
	`, withdrawalID, item.AgentID, operatorID, reason); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, entry_type, amount_micros,
			posting_status, source_type, source_id,
			idempotency_key, metadata, occurred_at
		)
		VALUES (
			$1, 'withdrawal_release', $2,
			'posted', 'withdrawal_failure', $3,
			$4, jsonb_build_object('reason', $5::text, 'asset_symbol', '¥'), NOW()
		)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, item.AgentID, item.AmountMicros, withdrawalID,
		fmt.Sprintf("withdrawal:%d:release", withdrawalID), reason,
	); err != nil {
		return nil, err
	}
	if err := insertAffiliateAgentNotice(
		ctx, tx, item.AgentID,
		"withdrawal_failed",
		"提现未能完成",
		"本次提现未能完成，金额已自动退回可提现余额，请核对收款资料后重试。",
		"withdrawal",
		withdrawalID,
		fmt.Sprintf("withdrawal:%d:failed-notice", withdrawalID),
	); err != nil {
		return nil, err
	}
	item, err = getAffiliateWithdrawalByID(ctx, tx, withdrawalID)
	if err != nil {
		return nil, err
	}
	return item, tx.Commit()
}

func (r *affiliateWalletRepository) GetAffiliateWithdrawal(
	ctx context.Context,
	withdrawalID int64,
) (*service.AffiliateWithdrawal, error) {
	item, err := getAffiliateWithdrawalByID(ctx, r.db, withdrawalID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliateWithdrawalNotFound
	}
	return item, err
}

func (r *affiliateWalletRepository) RecordAffiliateWithdrawalQRCodeAccess(
	ctx context.Context,
	withdrawalID int64,
	accessorUserID int64,
) error {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO agent_payment_qr_access_events (
			agent_id, withdrawal_id,
			accessor_user_id, access_context
		)
		SELECT
			agent_id, id,
			$2, 'withdrawal_snapshot'
		FROM agent_withdrawal_requests
		WHERE id = $1
	`, withdrawalID, accessorUserID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return service.ErrAffiliateWithdrawalNotFound
	}
	return nil
}

func (r *affiliateWalletRepository) ListAdminAffiliateWithdrawals(
	ctx context.Context,
	status string,
	limit int,
) ([]service.AffiliateWithdrawal, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var suffix string
	switch status {
	case "paid":
		suffix = `
		WHERE status = 'paid'
		ORDER BY paid_at DESC NULLS LAST, id DESC
		LIMIT $1`
	case "failed":
		suffix = `
		WHERE status = 'failed'
		ORDER BY failed_at DESC NULLS LAST, id DESC
		LIMIT $1`
	case "all":
		suffix = `
		WHERE status IN ('processing', 'paid', 'failed')
		ORDER BY requested_at DESC, id DESC
		LIMIT $1`
	default:
		suffix = `
		WHERE status = 'processing'
		ORDER BY due_at, id
		LIMIT $1`
	}
	rows, err := r.db.QueryContext(ctx, affiliateWithdrawalSelect(suffix), limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.AffiliateWithdrawal, 0)
	for rows.Next() {
		item, err := scanAffiliateWithdrawal(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *item)
	}
	return out, rows.Err()
}

func (r *affiliateWalletRepository) ConvertAffiliateCommission(
	ctx context.Context,
	agentID, requestedMicros int64,
	idempotencyKey string,
) (_ *service.AffiliateCommissionConversion, err error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockAffiliateAgent(ctx, tx, agentID); err != nil {
		return nil, err
	}
	existing, err := getAffiliateConversionByIdempotency(ctx, tx, agentID, idempotencyKey)
	if err == nil {
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var (
		principalStatus   string
		riskStatus        string
		multiplier        int32
		conversionEnabled bool
	)
	err = tx.QueryRowContext(ctx, `
		SELECT
			ap.status,
			ap.risk_status,
			s.commission_conversion_multiplier_millis,
			s.commission_conversion_enabled
		FROM agent_principals ap
		CROSS JOIN affiliate_program_settings s
		WHERE ap.agent_id = $1
			AND s.id = 1
		FOR UPDATE OF ap
	`, agentID).Scan(&principalStatus, &riskStatus, &multiplier, &conversionEnabled)
	if errors.Is(err, sql.ErrNoRows) ||
		principalStatus != "active" ||
		riskStatus != "clear" {
		return nil, service.ErrAffiliateWalletNotAvailable
	}
	if err != nil {
		return nil, err
	}
	if !conversionEnabled {
		return nil, service.ErrAffiliateConversionDisabled
	}
	availableMicros, err := lockAffiliateAvailableCash(ctx, tx, agentID)
	if err != nil {
		return nil, err
	}
	amountMicros := requestedMicros
	if amountMicros == 0 {
		amountMicros = availableMicros
	}
	if amountMicros <= 0 || amountMicros > availableMicros {
		return nil, service.ErrAffiliateInsufficientCash
	}
	creditMicros, err := service.AffiliateMultiplyMicros(amountMicros, multiplier)
	if err != nil {
		return nil, err
	}
	var conversion service.AffiliateCommissionConversion
	err = tx.QueryRowContext(ctx, `
		INSERT INTO agent_commission_conversions (
			agent_id, cash_amount_micros,
			credit_amount_micros, multiplier_millis,
			idempotency_key
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, agent_id, cash_amount_micros,
			credit_amount_micros, multiplier_millis, created_at
	`, agentID, amountMicros, creditMicros, multiplier, idempotencyKey).Scan(
		&conversion.ID,
		&conversion.AgentID,
		&conversion.CashAmountMicros,
		&conversion.CreditAmountMicros,
		&conversion.MultiplierMillis,
		&conversion.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, entry_type, amount_micros,
			posting_status, source_type, source_id,
			idempotency_key, metadata, occurred_at
		)
		VALUES (
			$1, 'conversion', -($2::bigint),
			'posted', 'commission_conversion', $3,
			$4, jsonb_build_object(
				'multiplier_millis', $5::integer,
				'credit_amount_micros', $6::bigint,
				'asset_symbol', '¥'
			), NOW()
		)
	`, agentID, amountMicros, conversion.ID,
		fmt.Sprintf("conversion:%d:cash", conversion.ID),
		multiplier, creditMicros,
	); err != nil {
		return nil, err
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE users
		SET balance = balance + ($1::numeric / 1000000),
			updated_at = NOW()
		WHERE id = $2
			AND deleted_at IS NULL
	`, creditMicros, agentID)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected != 1 {
		return nil, service.ErrUserNotFound
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_id, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, occurred_at
		)
		VALUES (
			$1, 'commission_conversion', $2, $3,
			$4, $4, FALSE, NOW()
		)
	`, agentID, conversion.ID, fmt.Sprintf("commission_conversion:%d", conversion.ID), creditMicros); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &conversion, nil
}

func (r *affiliateWalletRepository) PurchaseWithAffiliateCommission(
	ctx context.Context,
	agentID, amountMicros, operatorID int64,
	purchaseKind, productCode, externalReference, note, idempotencyKey string,
) (_ *service.AffiliateCommissionPurchase, err error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockAffiliateAgent(ctx, tx, agentID); err != nil {
		return nil, err
	}
	if existing, lookupErr := getAffiliatePurchaseByIdempotency(ctx, tx, agentID, idempotencyKey); lookupErr == nil {
		if !sameAffiliatePurchase(existing, amountMicros, operatorID, purchaseKind, productCode, externalReference, note) {
			return nil, service.ErrAffiliatePurchaseIdempotencyConflict
		}
		return existing, tx.Commit()
	} else if !errors.Is(lookupErr, sql.ErrNoRows) {
		return nil, lookupErr
	}
	if existing, lookupErr := getAffiliatePurchaseByReference(ctx, tx, agentID, externalReference); lookupErr == nil {
		if !sameAffiliatePurchase(existing, amountMicros, operatorID, purchaseKind, productCode, externalReference, note) {
			return nil, service.ErrAffiliatePurchaseIdempotencyConflict
		}
		return existing, tx.Commit()
	} else if !errors.Is(lookupErr, sql.ErrNoRows) {
		return nil, lookupErr
	}

	var principalStatus, riskStatus string
	err = tx.QueryRowContext(ctx, `
		SELECT status, risk_status
		FROM agent_principals
		WHERE agent_id = $1
		FOR UPDATE
	`, agentID).Scan(&principalStatus, &riskStatus)
	if errors.Is(err, sql.ErrNoRows) ||
		principalStatus != "active" ||
		riskStatus != "clear" {
		return nil, service.ErrAffiliateWalletNotAvailable
	}
	if err != nil {
		return nil, err
	}

	availableMicros, err := lockAffiliateAvailableCash(ctx, tx, agentID)
	if err != nil {
		return nil, err
	}
	if amountMicros > availableMicros {
		return nil, service.ErrAffiliateInsufficientCash
	}
	remainingMicros := availableMicros - amountMicros

	var entryID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, entry_type, amount_micros,
			posting_status, source_type, source_id,
			idempotency_key, metadata, occurred_at
		)
		VALUES (
			$1, 'platform_purchase', -($2::bigint),
			'posted', 'platform_purchase', NULL,
			$3, jsonb_build_object(
				'asset_symbol', '¥',
				'purchase_kind', $4::text,
				'product_code', $5::text,
				'external_reference', $6::text,
				'note', $7::text,
				'operator_id', $8::bigint,
				'remaining_cash_micros', $9::bigint
			), NOW()
		)
		RETURNING id
	`, agentID, amountMicros, idempotencyKey, purchaseKind, productCode,
		externalReference, note, operatorID, remainingMicros).Scan(&entryID)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return nil, service.ErrAffiliatePurchaseIdempotencyConflict
		}
		return nil, err
	}
	if err := insertAffiliateAgentNotice(
		ctx,
		tx,
		agentID,
		"commission_purchase",
		"佣金已用于购买平台套餐",
		fmt.Sprintf(
			"已从可提现佣金中扣除 %s，用于购买 %s 月卡；剩余可提现佣金 %s。",
			formatAffiliateCashMicros(amountMicros),
			affiliateMonthlyProductLabel(productCode),
			formatAffiliateCashMicros(remainingMicros),
		),
		"platform_purchase",
		entryID,
		fmt.Sprintf("platform-purchase:%d:notice", entryID),
	); err != nil {
		return nil, err
	}
	item, err := getAffiliatePurchaseByID(ctx, tx, entryID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *affiliateWalletRepository) RefundAffiliateCommissionPurchase(
	ctx context.Context,
	agentID, purchaseEntryID, expectedOriginalAmountMicros, expectedRefundAmountMicros, operatorID int64,
	rateBPS int32,
	note, idempotencyKey string,
) (_ *service.AffiliateCommissionPurchaseRefund, err error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockAffiliateAgent(ctx, tx, agentID); err != nil {
		return nil, err
	}

	if existing, lookupErr := getAffiliatePurchaseRefundByIdempotency(ctx, tx, agentID, idempotencyKey); lookupErr == nil {
		if !sameAffiliatePurchaseRefund(existing, purchaseEntryID, expectedOriginalAmountMicros, expectedRefundAmountMicros, rateBPS) {
			return nil, service.ErrAffiliatePurchaseRefundConflict
		}
		return existing, tx.Commit()
	} else if !errors.Is(lookupErr, sql.ErrNoRows) {
		return nil, lookupErr
	}
	if existing, lookupErr := getAffiliatePurchaseRefundByRate(ctx, tx, agentID, purchaseEntryID, rateBPS); lookupErr == nil {
		if !sameAffiliatePurchaseRefund(existing, purchaseEntryID, expectedOriginalAmountMicros, expectedRefundAmountMicros, rateBPS) {
			return nil, service.ErrAffiliatePurchaseRefundConflict
		}
		return existing, tx.Commit()
	} else if !errors.Is(lookupErr, sql.ErrNoRows) {
		return nil, lookupErr
	}

	var originalAmountMicros int64
	var purchaseKind, productCode, externalReference string
	var purchasedAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT
			-(amount_micros),
			metadata ->> 'purchase_kind',
			metadata ->> 'product_code',
			metadata ->> 'external_reference',
			occurred_at
		FROM agent_cash_commission_entries
		WHERE id=$1 AND agent_id=$2 AND entry_type='platform_purchase'
		  AND posting_status='posted'
		FOR UPDATE
	`, purchaseEntryID, agentID).Scan(
		&originalAmountMicros,
		&purchaseKind,
		&productCode,
		&externalReference,
		&purchasedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAffiliatePurchaseNotFound
	}
	if err != nil {
		return nil, err
	}
	if purchaseKind != service.AffiliateCommissionPurchaseKindMonthlyCard ||
		originalAmountMicros != expectedOriginalAmountMicros ||
		originalAmountMicros%10_000 != 0 {
		return nil, service.ErrAffiliatePurchaseRefundConflict
	}
	originalFen := originalAmountMicros / 10_000
	payableFen := (originalFen*int64(rateBPS) + 9_999) / 10_000
	payableAmountMicros := payableFen * 10_000
	refundAmountMicros := originalAmountMicros - payableAmountMicros
	if refundAmountMicros <= 0 || refundAmountMicros != expectedRefundAmountMicros {
		return nil, service.ErrAffiliatePurchaseRefundConflict
	}
	var priorRefundMicros int64
	if err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_micros),0)::bigint
		FROM agent_cash_commission_entries
		WHERE agent_id=$1 AND related_entry_id=$2
		  AND entry_type='platform_purchase_refund' AND posting_status='posted'
	`, agentID, purchaseEntryID).Scan(&priorRefundMicros); err != nil {
		return nil, err
	}
	if priorRefundMicros != 0 {
		return nil, service.ErrAffiliatePurchaseRefundConflict
	}
	availableMicros, err := lockAffiliateAvailableCash(ctx, tx, agentID)
	if err != nil {
		return nil, err
	}
	remainingMicros := availableMicros + refundAmountMicros
	notificationDedupe := fmt.Sprintf("platform-purchase-refund:%d:%d", purchaseEntryID, rateBPS)

	var refundID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, entry_type, amount_micros,
			posting_status, source_type, source_id,
			idempotency_key, related_entry_id, metadata, occurred_at
		) VALUES (
			$1, 'platform_purchase_refund', $2,
			'posted', 'platform_purchase_refund', $3,
			$4, $3, jsonb_build_object(
				'asset_symbol','¥',
				'purchase_kind',$5::text,
				'product_code',$6::text,
				'external_reference',$7::text,
				'original_amount_micros',$8::bigint,
				'rate_bps',$9::integer,
				'payable_amount_micros',$10::bigint,
				'refund_amount_micros',$2::bigint,
				'note',$11::text,
				'operator_id',$12::bigint,
				'remaining_cash_micros',$13::bigint,
				'notification_dedupe_key',$14::text
			), NOW()
		) RETURNING id
	`, agentID, refundAmountMicros, purchaseEntryID, idempotencyKey,
		purchaseKind, productCode, externalReference, originalAmountMicros, rateBPS,
		payableAmountMicros, note, operatorID, remainingMicros, notificationDedupe).Scan(&refundID)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return nil, service.ErrAffiliatePurchaseRefundConflict
		}
		return nil, err
	}

	beijing := time.FixedZone("Asia/Shanghai", 8*60*60)
	title := fmt.Sprintf("%s 月卡合伙人优惠差额已退回", affiliateMonthlyProductLabel(productCode))
	body := fmt.Sprintf(
		"您好，您于 %s购买的 %s 月卡，成交原价为 %s。按照合伙人佣金钱包 %s 结算规则，实际应付 %s，差额 %s 已退回您的佣金钱包。\n\n退款后佣金钱包余额为 %s。本次 %s 月卡额度升级为免费升级，不会重置已用额度，也不会改变到期时间。感谢您的支持。",
		purchasedAt.In(beijing).Format("2006 年 1 月 2 日"),
		affiliateMonthlyProductLabel(productCode),
		formatAffiliateCashMicros(originalAmountMicros),
		formatAffiliateRateBPS(rateBPS),
		formatAffiliateCashMicros(payableAmountMicros),
		formatAffiliateCashMicros(refundAmountMicros),
		formatAffiliateCashMicros(remainingMicros),
		affiliateMonthlyProductLabel(productCode),
	)
	var notificationID int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO user_notifications (
			user_id,type,title,body,action_url,dedupe_key
		) VALUES ($1,'affiliate_wallet_refund',$2,$3,'/affiliate',$4)
		ON CONFLICT(dedupe_key) DO NOTHING
		RETURNING id
	`, agentID, title, body, notificationDedupe).Scan(&notificationID)
	if errors.Is(err, sql.ErrNoRows) {
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM user_notifications
			WHERE user_id=$1 AND type='affiliate_wallet_refund' AND title=$2
			  AND body=$3 AND action_url='/affiliate' AND dedupe_key=$4
		`, agentID, title, body, notificationDedupe).Scan(&notificationID)
	}
	if err != nil {
		return nil, err
	}
	item, err := getAffiliatePurchaseRefundByID(ctx, tx, refundID)
	if err != nil {
		return nil, err
	}
	if item.NotificationID != notificationID || item.NotificationDedupeKey != notificationDedupe {
		return nil, fmt.Errorf("affiliate purchase refund notification readback mismatch")
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *affiliateWalletRepository) PurchaseBalanceWithAffiliateCommission(
	ctx context.Context,
	agentID, creditAmountCNYFen int64,
	idempotencyKey string,
) (_ *service.AffiliateBalancePurchase, err error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if err := lockAffiliateAgent(ctx, tx, agentID); err != nil {
		return nil, err
	}
	if existing, lookupErr := getAffiliateBalancePurchase(ctx, tx, agentID, idempotencyKey); lookupErr == nil {
		if existing.CreditAmountMicros != creditAmountCNYFen*10_000 {
			return nil, service.ErrAffiliatePurchaseIdempotencyConflict
		}
		return existing, tx.Commit()
	} else if !errors.Is(lookupErr, sql.ErrNoRows) {
		return nil, lookupErr
	}
	var principalStatus, riskStatus string
	var enabled bool
	var rateBPS int32
	err = tx.QueryRowContext(ctx, `
		SELECT ap.status, ap.risk_status,
		       s.commission_wallet_checkout_enabled,
		       s.commission_wallet_purchase_rate_bps
		FROM agent_principals ap
		CROSS JOIN affiliate_program_settings s
		WHERE ap.agent_id=$1 AND s.id=1
		FOR UPDATE OF ap
	`, agentID).Scan(&principalStatus, &riskStatus, &enabled, &rateBPS)
	if errors.Is(err, sql.ErrNoRows) || principalStatus != "active" || riskStatus != "clear" {
		return nil, service.ErrAffiliateWalletNotAvailable
	}
	if err != nil {
		return nil, err
	}
	if !enabled || rateBPS <= 0 {
		return nil, service.ErrAffiliateWalletCheckoutDisabled
	}
	chargeFen := (creditAmountCNYFen*int64(rateBPS) + 9_999) / 10_000
	chargeMicros := chargeFen * 10_000
	creditMicros := creditAmountCNYFen * 10_000
	availableMicros, err := lockAffiliateAvailableCash(ctx, tx, agentID)
	if err != nil {
		return nil, err
	}
	if chargeMicros > availableMicros {
		return nil, service.ErrAffiliateInsufficientCash
	}
	remainingMicros := availableMicros - chargeMicros
	var result service.AffiliateBalancePurchase
	err = tx.QueryRowContext(ctx, `
		INSERT INTO agent_cash_commission_entries (
			agent_id, entry_type, amount_micros, posting_status,
			source_type, idempotency_key, metadata, occurred_at
		) VALUES (
			$1, 'platform_purchase', -($2::bigint), 'posted',
			'platform_purchase', $3::text,
			jsonb_build_object(
				'asset_symbol','¥', 'purchase_kind','balance',
				'product_code','standard_balance', 'external_reference',$3::text,
				'operator_id',$1::bigint, 'credit_amount_micros',$4::bigint,
				'rate_bps',$5::integer, 'remaining_cash_micros',$6::bigint
			), NOW()
		) RETURNING id, agent_id, -amount_micros, occurred_at
	`, agentID, chargeMicros, idempotencyKey, creditMicros, rateBPS, remainingMicros).Scan(
		&result.LedgerEntryID, &result.AgentID, &result.CashAmountMicros, &result.CreatedAt,
	)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return nil, service.ErrAffiliatePurchaseIdempotencyConflict
		}
		return nil, err
	}
	result.CreditAmountMicros = creditMicros
	result.RateBPS = rateBPS
	result.RemainingCashMicros = remainingMicros
	update, err := tx.ExecContext(ctx, `
		UPDATE users SET balance=balance+($1::numeric/1000000), updated_at=NOW()
		WHERE id=$2 AND deleted_at IS NULL
	`, creditMicros, agentID)
	if err != nil {
		return nil, err
	}
	if affected, _ := update.RowsAffected(); affected != 1 {
		return nil, service.ErrUserNotFound
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO balance_lots (
			user_id, source_type, source_id, source_key,
			original_amount_micros, remaining_amount_micros,
			affiliate_eligible, affiliate_policy, occurred_at
		) VALUES ($1,'commission_purchase',$2,$3,$4,$4,FALSE,'NONE',NOW())
	`, agentID, result.LedgerEntryID, "commission_purchase:"+idempotencyKey, creditMicros); err != nil {
		return nil, err
	}
	if err := insertAffiliateAgentNotice(
		ctx, tx, agentID, "commission_purchase", "佣金已用于余额充值",
		fmt.Sprintf("已从可提现佣金中扣除 %s，充值平台余额 %s；剩余可提现佣金 %s。",
			formatAffiliateCashMicros(chargeMicros), formatAffiliateCashMicros(creditMicros), formatAffiliateCashMicros(remainingMicros)),
		"platform_purchase", result.LedgerEntryID, fmt.Sprintf("platform-purchase:%d:notice", result.LedgerEntryID),
	); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &result, nil
}

func getAffiliateBalancePurchase(
	ctx context.Context, q affiliateWalletQuerier, agentID int64, idempotencyKey string,
) (*service.AffiliateBalancePurchase, error) {
	out := &service.AffiliateBalancePurchase{}
	err := q.QueryRowContext(ctx, `
		SELECT id, agent_id, -amount_micros,
		       (metadata->>'credit_amount_micros')::bigint,
		       (metadata->>'rate_bps')::integer,
		       (metadata->>'remaining_cash_micros')::bigint,
		       occurred_at
		FROM agent_cash_commission_entries
		WHERE agent_id=$1 AND idempotency_key=$2
		  AND entry_type='platform_purchase'
		  AND metadata->>'purchase_kind'='balance'
	`, agentID, idempotencyKey).Scan(
		&out.LedgerEntryID, &out.AgentID, &out.CashAmountMicros,
		&out.CreditAmountMicros, &out.RateBPS, &out.RemainingCashMicros, &out.CreatedAt,
	)
	return out, err
}

func formatAffiliateCashMicros(amountMicros int64) string {
	cents := (amountMicros + 5_000) / 10_000
	return fmt.Sprintf("¥%d.%02d", cents/100, cents%100)
}

func formatAffiliateRateBPS(rateBPS int32) string {
	if rateBPS%100 == 0 {
		return fmt.Sprintf("%d%%", rateBPS/100)
	}
	return fmt.Sprintf("%d.%02d%%", rateBPS/100, rateBPS%100)
}

func affiliateMonthlyProductLabel(productCode string) string {
	switch productCode {
	case "plus":
		return "Plus"
	case "pro":
		return "Pro"
	case "max":
		return "Max"
	default:
		return productCode
	}
}

func sameAffiliatePurchase(
	item *service.AffiliateCommissionPurchase,
	amountMicros, operatorID int64,
	purchaseKind, productCode, externalReference, note string,
) bool {
	return item != nil &&
		item.AmountMicros == amountMicros &&
		item.OperatorID == operatorID &&
		item.PurchaseKind == purchaseKind &&
		item.ProductCode == productCode &&
		item.ExternalReference == externalReference &&
		item.Note == note
}

func (r *affiliateWalletRepository) ListAffiliateAgentNotices(
	ctx context.Context,
	agentID int64,
	limit int,
) ([]service.AffiliateAgentNotice, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, notice_type, title, message,
			source_type, source_id, read_at, created_at
		FROM affiliate_agent_notices
		WHERE agent_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make([]service.AffiliateAgentNotice, 0)
	for rows.Next() {
		var item service.AffiliateAgentNotice
		if err := rows.Scan(
			&item.ID,
			&item.NoticeType,
			&item.Title,
			&item.Message,
			&item.SourceType,
			&item.SourceID,
			&item.ReadAt,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *affiliateWalletRepository) MarkAffiliateAgentNoticeRead(
	ctx context.Context,
	agentID, noticeID int64,
) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE affiliate_agent_notices
		SET read_at = COALESCE(read_at, NOW())
		WHERE id = $1
			AND agent_id = $2
	`, noticeID, agentID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return service.ErrAffiliateNoticeNotFound
	}
	return nil
}

func lockAffiliateAvailableCash(ctx context.Context, tx *sql.Tx, agentID int64) (int64, error) {
	if err := lockAffiliateAgent(ctx, tx, agentID); err != nil {
		return 0, err
	}
	var available int64
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_micros), 0)::bigint
		FROM agent_cash_commission_entries
		WHERE agent_id = $1
			AND posting_status = 'posted'
	`, agentID).Scan(&available)
	return available, err
}

func lockAffiliateAgent(ctx context.Context, tx *sql.Tx, agentID int64) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, agentID)
	return err
}

type affiliateWithdrawalScanner interface {
	Scan(dest ...any) error
}

func scanAffiliateWithdrawal(scanner affiliateWithdrawalScanner) (*service.AffiliateWithdrawal, error) {
	out := &service.AffiliateWithdrawal{}
	err := scanner.Scan(
		&out.ID,
		&out.AgentID,
		&out.AmountMicros,
		&out.Status,
		&out.AgentRiskStatus,
		&out.PaymentAlipayRealName,
		&out.PaymentAlipayAccount,
		&out.PaymentContactPhone,
		&out.PaymentNote,
		&out.PaymentQRObjectKey,
		&out.PaymentQRContentType,
		&out.PaymentQROriginalName,
		&out.RequestedAt,
		&out.DueAt,
		&out.PaidAt,
		&out.FailedAt,
		&out.HandledBy,
		&out.PaymentReference,
		&out.FailureReason,
	)
	return out, err
}

type affiliateWalletQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func getAffiliatePurchaseByID(
	ctx context.Context,
	q affiliateWalletQuerier,
	entryID int64,
) (*service.AffiliateCommissionPurchase, error) {
	return scanAffiliatePurchase(q.QueryRowContext(ctx, affiliatePurchaseSelect(`
		WHERE id = $1
	`), entryID))
}

func getAffiliatePurchaseByIdempotency(
	ctx context.Context,
	q affiliateWalletQuerier,
	agentID int64,
	idempotencyKey string,
) (*service.AffiliateCommissionPurchase, error) {
	return scanAffiliatePurchase(q.QueryRowContext(ctx, affiliatePurchaseSelect(`
		WHERE agent_id = $1
			AND idempotency_key = $2
	`), agentID, idempotencyKey))
}

func getAffiliatePurchaseByReference(
	ctx context.Context,
	q affiliateWalletQuerier,
	agentID int64,
	externalReference string,
) (*service.AffiliateCommissionPurchase, error) {
	return scanAffiliatePurchase(q.QueryRowContext(ctx, affiliatePurchaseSelect(`
		WHERE agent_id = $1
			AND metadata ->> 'external_reference' = $2
	`), agentID, externalReference))
}

func affiliatePurchaseSelect(suffix string) string {
	return `
		SELECT
			id,
			agent_id,
			-(amount_micros),
			metadata ->> 'purchase_kind',
			metadata ->> 'product_code',
			metadata ->> 'external_reference',
			COALESCE(metadata ->> 'note', ''),
			(metadata ->> 'operator_id')::bigint,
			(metadata ->> 'remaining_cash_micros')::bigint,
			occurred_at
		FROM agent_cash_commission_entries
	` + suffix + `
			AND entry_type = 'platform_purchase'
		LIMIT 1`
}

type affiliatePurchaseScanner interface {
	Scan(dest ...any) error
}

func scanAffiliatePurchase(scanner affiliatePurchaseScanner) (*service.AffiliateCommissionPurchase, error) {
	out := &service.AffiliateCommissionPurchase{}
	err := scanner.Scan(
		&out.ID,
		&out.AgentID,
		&out.AmountMicros,
		&out.PurchaseKind,
		&out.ProductCode,
		&out.ExternalReference,
		&out.Note,
		&out.OperatorID,
		&out.RemainingCashMicros,
		&out.CreatedAt,
	)
	return out, err
}

func getAffiliatePurchaseRefundByID(
	ctx context.Context,
	q affiliateWalletQuerier,
	refundID int64,
) (*service.AffiliateCommissionPurchaseRefund, error) {
	return scanAffiliatePurchaseRefund(q.QueryRowContext(ctx, affiliatePurchaseRefundSelect(`
		WHERE refund.id=$1
	`), refundID))
}

func getAffiliatePurchaseRefundByIdempotency(
	ctx context.Context,
	q affiliateWalletQuerier,
	agentID int64,
	idempotencyKey string,
) (*service.AffiliateCommissionPurchaseRefund, error) {
	return scanAffiliatePurchaseRefund(q.QueryRowContext(ctx, affiliatePurchaseRefundSelect(`
		WHERE refund.agent_id=$1 AND refund.idempotency_key=$2
	`), agentID, idempotencyKey))
}

func getAffiliatePurchaseRefundByRate(
	ctx context.Context,
	q affiliateWalletQuerier,
	agentID, purchaseEntryID int64,
	rateBPS int32,
) (*service.AffiliateCommissionPurchaseRefund, error) {
	return scanAffiliatePurchaseRefund(q.QueryRowContext(ctx, affiliatePurchaseRefundSelect(`
		WHERE refund.agent_id=$1 AND refund.related_entry_id=$2
		  AND (refund.metadata->>'rate_bps')::integer=$3
	`), agentID, purchaseEntryID, rateBPS))
}

func affiliatePurchaseRefundSelect(suffix string) string {
	return `
		SELECT
			refund.id,
			refund.agent_id,
			refund.related_entry_id,
			(refund.metadata->>'original_amount_micros')::bigint,
			(refund.metadata->>'rate_bps')::integer,
			(refund.metadata->>'payable_amount_micros')::bigint,
			refund.amount_micros,
			(refund.metadata->>'remaining_cash_micros')::bigint,
			notification.id,
			refund.metadata->>'notification_dedupe_key',
			refund.occurred_at
		FROM agent_cash_commission_entries refund
		JOIN user_notifications notification
		  ON notification.dedupe_key=refund.metadata->>'notification_dedupe_key'
	` + suffix + `
		  AND refund.entry_type='platform_purchase_refund'
		  AND refund.posting_status='posted'
		LIMIT 1`
}

type affiliatePurchaseRefundScanner interface {
	Scan(dest ...any) error
}

func scanAffiliatePurchaseRefund(scanner affiliatePurchaseRefundScanner) (*service.AffiliateCommissionPurchaseRefund, error) {
	out := &service.AffiliateCommissionPurchaseRefund{}
	err := scanner.Scan(
		&out.ID,
		&out.AgentID,
		&out.PurchaseEntryID,
		&out.OriginalAmountMicros,
		&out.RateBPS,
		&out.PayableAmountMicros,
		&out.RefundAmountMicros,
		&out.RemainingCashMicros,
		&out.NotificationID,
		&out.NotificationDedupeKey,
		&out.CreatedAt,
	)
	return out, err
}

func sameAffiliatePurchaseRefund(
	item *service.AffiliateCommissionPurchaseRefund,
	purchaseEntryID, expectedOriginalAmountMicros, expectedRefundAmountMicros int64,
	rateBPS int32,
) bool {
	return item != nil &&
		item.PurchaseEntryID == purchaseEntryID &&
		item.OriginalAmountMicros == expectedOriginalAmountMicros &&
		item.RefundAmountMicros == expectedRefundAmountMicros &&
		item.RateBPS == rateBPS
}

func getAffiliateWithdrawalByID(
	ctx context.Context,
	q affiliateWalletQuerier,
	withdrawalID int64,
) (*service.AffiliateWithdrawal, error) {
	return scanAffiliateWithdrawal(q.QueryRowContext(ctx, affiliateWithdrawalSelect(`
		WHERE id = $1
	`), withdrawalID))
}

func getAffiliateWithdrawalByIDForUpdate(
	ctx context.Context,
	q affiliateWalletQuerier,
	withdrawalID int64,
) (*service.AffiliateWithdrawal, error) {
	return scanAffiliateWithdrawal(q.QueryRowContext(ctx, affiliateWithdrawalSelect(`
		WHERE id = $1
		FOR UPDATE
	`), withdrawalID))
}

func getAffiliateWithdrawalByIdempotency(
	ctx context.Context,
	q affiliateWalletQuerier,
	agentID int64,
	idempotencyKey string,
) (*service.AffiliateWithdrawal, error) {
	return scanAffiliateWithdrawal(q.QueryRowContext(ctx, affiliateWithdrawalSelect(`
		WHERE agent_id = $1
			AND idempotency_key = $2
	`), agentID, idempotencyKey))
}

func affiliateWithdrawalSelect(suffix string) string {
	return `
		SELECT
			id, agent_id, amount_micros, status,
			COALESCE((
				SELECT risk_status
				FROM agent_principals
				WHERE agent_id = agent_withdrawal_requests.agent_id
			), 'blocked') AS agent_risk_status,
			payment_alipay_real_name, payment_alipay_account,
			payment_contact_phone, payment_note,
			payment_qr_object_key, payment_qr_content_type,
			payment_qr_original_filename,
			requested_at, due_at, paid_at, failed_at,
			handled_by, payment_reference, failure_reason
		FROM agent_withdrawal_requests
	` + suffix
}

func getAffiliateConversionByIdempotency(
	ctx context.Context,
	q affiliateWalletQuerier,
	agentID int64,
	idempotencyKey string,
) (*service.AffiliateCommissionConversion, error) {
	out := &service.AffiliateCommissionConversion{}
	err := q.QueryRowContext(ctx, `
		SELECT
			id, agent_id, cash_amount_micros,
			credit_amount_micros, multiplier_millis, created_at
		FROM agent_commission_conversions
		WHERE agent_id = $1
			AND idempotency_key = $2
	`, agentID, idempotencyKey).Scan(
		&out.ID,
		&out.AgentID,
		&out.CashAmountMicros,
		&out.CreditAmountMicros,
		&out.MultiplierMillis,
		&out.CreatedAt,
	)
	return out, err
}

func insertAffiliateAgentNotice(
	ctx context.Context,
	tx *sql.Tx,
	agentID int64,
	noticeType, title, message, sourceType string,
	sourceID int64,
	idempotencyKey string,
) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO affiliate_agent_notices (
			agent_id, notice_type, title, message,
			source_type, source_id, idempotency_key
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (idempotency_key) DO NOTHING
	`, agentID, noticeType, title, message, sourceType, sourceID, idempotencyKey)
	return err
}

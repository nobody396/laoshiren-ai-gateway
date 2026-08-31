package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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
		principalStatus string
		riskStatus      string
		multiplier      int32
	)
	err = tx.QueryRowContext(ctx, `
		SELECT
			ap.status,
			ap.risk_status,
			s.commission_conversion_multiplier_millis
		FROM agent_principals ap
		CROSS JOIN affiliate_program_settings s
		WHERE ap.agent_id = $1
			AND s.id = 1
		FOR UPDATE OF ap
	`, agentID).Scan(&principalStatus, &riskStatus, &multiplier)
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

func formatAffiliateCashMicros(amountMicros int64) string {
	cents := (amountMicros + 5_000) / 10_000
	return fmt.Sprintf("¥%d.%02d", cents/100, cents%100)
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

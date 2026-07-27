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

func (r *commissionRepository) GetAgentSettlementSettings(ctx context.Context) (*service.AgentSettlementSettings, error) {
	if r.sql == nil {
		return &service.AgentSettlementSettings{MinimumAmount: 50}, nil
	}
	var settings service.AgentSettlementSettings
	err := scanSingleRow(ctx, r.sql, `
		SELECT settlement_min_amount, updated_at
		FROM agent_commission_settings
		WHERE id = 1
	`, nil, &settings.MinimumAmount, &settings.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) || isMissingAgentManagementRelation(err) {
		return &service.AgentSettlementSettings{MinimumAmount: 50}, nil
	}
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "settlement_min_amount") {
		return &service.AgentSettlementSettings{MinimumAmount: 50}, nil
	}
	return &settings, err
}

func (r *commissionRepository) UpdateAgentSettlementSettings(ctx context.Context, settings *service.AgentSettlementSettings) error {
	if r.sql == nil {
		return fmt.Errorf("sql executor is not configured")
	}
	return scanSingleRow(ctx, r.sql, `
		INSERT INTO agent_commission_settings (id, settlement_min_amount)
		VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET
			settlement_min_amount = EXCLUDED.settlement_min_amount,
			updated_at = NOW()
		RETURNING updated_at
	`, []any{settings.MinimumAmount}, &settings.UpdatedAt)
}

func (r *commissionRepository) GetAgentPaymentProfile(ctx context.Context, agentID int64) (*service.AgentPaymentProfile, error) {
	if r.sql == nil {
		return nil, nil
	}
	var profile service.AgentPaymentProfile
	var createdAt, updatedAt sql.NullTime
	err := scanSingleRow(ctx, r.sql, `
		SELECT
			agent_id,
			alipay_real_name,
			alipay_account,
			contact_phone,
			payment_note,
			identity_fingerprint_hash,
			verification_status,
			verification_note,
			verified_at,
			verified_by,
			alipay_qr_object_key,
			alipay_qr_content_type,
			alipay_qr_original_filename,
			alipay_qr_size,
			created_at,
			updated_at
		FROM agent_payment_profiles
		WHERE agent_id = $1
	`, []any{agentID},
		&profile.AgentID,
		&profile.AlipayRealName,
		&profile.AlipayAccount,
		&profile.ContactPhone,
		&profile.PaymentNote,
		&profile.IdentityFingerprintHash,
		&profile.VerificationStatus,
		&profile.VerificationNote,
		&profile.VerifiedAt,
		&profile.VerifiedBy,
		&profile.AlipayQRCodeObjectKey,
		&profile.AlipayQRCodeContentType,
		&profile.AlipayQRCodeOriginalName,
		&profile.AlipayQRCodeSize,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) || isMissingAgentManagementRelation(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if createdAt.Valid {
		profile.CreatedAt = &createdAt.Time
	}
	if updatedAt.Valid {
		profile.UpdatedAt = &updatedAt.Time
	}
	return &profile, nil
}

func (r *commissionRepository) UpsertAgentPaymentProfile(ctx context.Context, profile *service.AgentPaymentProfile) error {
	if r.sql == nil {
		return fmt.Errorf("sql executor is not configured")
	}
	var createdAt, updatedAt time.Time
	err := scanSingleRow(ctx, r.sql, `
		INSERT INTO agent_payment_profiles (
			agent_id,
			alipay_real_name,
			alipay_account,
			contact_phone,
			payment_note,
			identity_fingerprint_hash
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (agent_id) DO UPDATE SET
			alipay_real_name = EXCLUDED.alipay_real_name,
			alipay_account = EXCLUDED.alipay_account,
			contact_phone = EXCLUDED.contact_phone,
			payment_note = EXCLUDED.payment_note,
			identity_fingerprint_hash = EXCLUDED.identity_fingerprint_hash,
			verification_status = CASE
				WHEN BTRIM(EXCLUDED.alipay_real_name) <> ''
				 AND BTRIM(EXCLUDED.alipay_account) <> ''
				 AND BTRIM(agent_payment_profiles.alipay_qr_object_key) <> ''
				THEN 'pending_review'
				ELSE 'incomplete'
			END,
			verification_note = '',
			verified_at = NULL,
			verified_by = NULL,
			updated_at = NOW()
		RETURNING created_at, updated_at
	`, []any{
		profile.AgentID,
		profile.AlipayRealName,
		profile.AlipayAccount,
		profile.ContactPhone,
		profile.PaymentNote,
		profile.IdentityFingerprintHash,
	}, &createdAt, &updatedAt)
	if err != nil {
		return err
	}
	profile.CreatedAt = &createdAt
	profile.UpdatedAt = &updatedAt
	if _, err := r.sql.ExecContext(ctx, `
		UPDATE agent_principals
		SET principal_key_hash = NULL,
			updated_at = NOW()
		WHERE agent_id = $1
	`, profile.AgentID); err != nil && !isMissingAgentManagementRelation(err) {
		return err
	}
	return nil
}

func (r *commissionRepository) UpdateAgentPaymentQRCode(ctx context.Context, agentID int64, objectKey, contentType, originalName string, size int64) (*service.AgentPaymentProfile, error) {
	if r.sql == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}
	var profile service.AgentPaymentProfile
	var createdAt, updatedAt sql.NullTime
	err := scanSingleRow(ctx, r.sql, `
		INSERT INTO agent_payment_profiles (
			agent_id,
			alipay_qr_object_key,
			alipay_qr_content_type,
			alipay_qr_original_filename,
			alipay_qr_size
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (agent_id) DO UPDATE SET
			alipay_qr_object_key = EXCLUDED.alipay_qr_object_key,
			alipay_qr_content_type = EXCLUDED.alipay_qr_content_type,
			alipay_qr_original_filename = EXCLUDED.alipay_qr_original_filename,
			alipay_qr_size = EXCLUDED.alipay_qr_size,
			verification_status = CASE
				WHEN BTRIM(agent_payment_profiles.alipay_real_name) <> ''
				 AND BTRIM(agent_payment_profiles.alipay_account) <> ''
				THEN 'pending_review'
				ELSE 'incomplete'
			END,
			verification_note = '',
			verified_at = NULL,
			verified_by = NULL,
			updated_at = NOW()
		RETURNING
			agent_id,
			alipay_real_name,
			alipay_account,
			contact_phone,
			payment_note,
			identity_fingerprint_hash,
			verification_status,
			verification_note,
			verified_at,
			verified_by,
			alipay_qr_object_key,
			alipay_qr_content_type,
			alipay_qr_original_filename,
			alipay_qr_size,
			created_at,
			updated_at
	`, []any{agentID, objectKey, contentType, originalName, size},
		&profile.AgentID,
		&profile.AlipayRealName,
		&profile.AlipayAccount,
		&profile.ContactPhone,
		&profile.PaymentNote,
		&profile.IdentityFingerprintHash,
		&profile.VerificationStatus,
		&profile.VerificationNote,
		&profile.VerifiedAt,
		&profile.VerifiedBy,
		&profile.AlipayQRCodeObjectKey,
		&profile.AlipayQRCodeContentType,
		&profile.AlipayQRCodeOriginalName,
		&profile.AlipayQRCodeSize,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if createdAt.Valid {
		profile.CreatedAt = &createdAt.Time
	}
	if updatedAt.Valid {
		profile.UpdatedAt = &updatedAt.Time
	}
	if _, err := r.sql.ExecContext(ctx, `
		UPDATE agent_principals
		SET principal_key_hash = NULL,
			updated_at = NOW()
		WHERE agent_id = $1
	`, agentID); err != nil && !isMissingAgentManagementRelation(err) {
		return nil, err
	}
	return &profile, nil
}

func (r *commissionRepository) ReviewAgentPaymentProfile(
	ctx context.Context,
	agentID, reviewerID int64,
	status, note string,
) (_ *service.AgentPaymentProfile, err error) {
	if r.db == nil {
		return nil, fmt.Errorf("sql db is not configured")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		realName    string
		account     string
		qrObjectKey string
		fingerprint string
	)
	err = tx.QueryRowContext(ctx, `
		SELECT
			alipay_real_name,
			alipay_account,
			alipay_qr_object_key,
			identity_fingerprint_hash
		FROM agent_payment_profiles
		WHERE agent_id = $1
		FOR UPDATE
	`, agentID).Scan(&realName, &account, &qrObjectKey, &fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, service.ErrAgentPaymentProfileIncomplete
	}
	if err != nil {
		return nil, err
	}

	if status == "verified" {
		if strings.TrimSpace(realName) == "" ||
			strings.TrimSpace(account) == "" ||
			strings.TrimSpace(qrObjectKey) == "" ||
			strings.TrimSpace(fingerprint) == "" {
			return nil, service.ErrAgentPaymentProfileIncomplete
		}
		result, err := tx.ExecContext(ctx, `
			UPDATE agent_principals
			SET principal_key_hash = $1,
				updated_at = NOW()
			WHERE agent_id = $2
				AND status = 'active'
		`, fingerprint, agentID)
		if err != nil {
			if isPostgresUniqueViolation(err) {
				return nil, service.ErrAgentPaymentIdentityConflict
			}
			return nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return nil, err
		}
		if affected != 1 {
			return nil, service.ErrAffiliateAgentNotActive
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			UPDATE agent_principals
			SET principal_key_hash = NULL,
				updated_at = NOW()
			WHERE agent_id = $1
		`, agentID); err != nil {
			return nil, err
		}
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE agent_payment_profiles
		SET verification_status = $1::varchar,
			verification_note = $2,
			verified_at = CASE WHEN $1::text = 'verified' THEN NOW() ELSE NULL END,
			verified_by = $3,
			updated_at = NOW()
		WHERE agent_id = $4
	`, status, note, reviewerID, agentID)
	if err != nil {
		if isPostgresUniqueViolation(err) {
			return nil, service.ErrAgentPaymentIdentityConflict
		}
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		if isPostgresUniqueViolation(err) {
			return nil, service.ErrAgentPaymentIdentityConflict
		}
		return nil, err
	}
	return r.GetAgentPaymentProfile(ctx, agentID)
}

func (r *commissionRepository) ListPendingAgentPaymentProfiles(
	ctx context.Context,
	limit int,
) ([]service.AgentPaymentProfile, error) {
	if r.db == nil {
		return nil, fmt.Errorf("sql db is not configured")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			agent_id,
			alipay_real_name,
			alipay_account,
			contact_phone,
			payment_note,
			identity_fingerprint_hash,
			verification_status,
			verification_note,
			verified_at,
			verified_by,
			alipay_qr_object_key,
			alipay_qr_content_type,
			alipay_qr_original_filename,
			alipay_qr_size,
			created_at,
			updated_at
		FROM agent_payment_profiles
		WHERE verification_status = 'pending_review'
		ORDER BY updated_at ASC, agent_id ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]service.AgentPaymentProfile, 0)
	for rows.Next() {
		var item service.AgentPaymentProfile
		var createdAt, updatedAt sql.NullTime
		if err := rows.Scan(
			&item.AgentID,
			&item.AlipayRealName,
			&item.AlipayAccount,
			&item.ContactPhone,
			&item.PaymentNote,
			&item.IdentityFingerprintHash,
			&item.VerificationStatus,
			&item.VerificationNote,
			&item.VerifiedAt,
			&item.VerifiedBy,
			&item.AlipayQRCodeObjectKey,
			&item.AlipayQRCodeContentType,
			&item.AlipayQRCodeOriginalName,
			&item.AlipayQRCodeSize,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			item.CreatedAt = &createdAt.Time
		}
		if updatedAt.Valid {
			item.UpdatedAt = &updatedAt.Time
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

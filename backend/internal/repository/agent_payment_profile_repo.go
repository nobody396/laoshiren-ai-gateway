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
			payment_note
		)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (agent_id) DO UPDATE SET
			alipay_real_name = EXCLUDED.alipay_real_name,
			alipay_account = EXCLUDED.alipay_account,
			contact_phone = EXCLUDED.contact_phone,
			payment_note = EXCLUDED.payment_note,
			updated_at = NOW()
		RETURNING created_at, updated_at
	`, []any{
		profile.AgentID,
		profile.AlipayRealName,
		profile.AlipayAccount,
		profile.ContactPhone,
		profile.PaymentNote,
	}, &createdAt, &updatedAt)
	if err != nil {
		return err
	}
	profile.CreatedAt = &createdAt
	profile.UpdatedAt = &updatedAt
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
			updated_at = NOW()
		RETURNING
			agent_id,
			alipay_real_name,
			alipay_account,
			contact_phone,
			payment_note,
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
	return &profile, nil
}

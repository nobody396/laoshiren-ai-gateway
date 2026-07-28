package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type affiliateProgramRepository struct {
	db *sql.DB
}

func NewAffiliateProgramRepository(db *sql.DB) service.AffiliateProgramRepository {
	return &affiliateProgramRepository{db: db}
}

func (r *affiliateProgramRepository) GetSettings(ctx context.Context) (*service.AffiliateProgramSettings, error) {
	if r.db == nil {
		return nil, errors.New("affiliate program repository: nil database")
	}

	var settings service.AffiliateProgramSettings
	var startedAt sql.NullTime
	var updatedBy sql.NullInt64
	err := scanSingleRow(ctx, r.db, `
		SELECT
			id,
			program_version,
			mode,
			started_at,
			ordinary_referral_rate_bps,
			ordinary_invitee_rate_bps,
			first_paid_bonus_threshold_micros,
			first_paid_bonus_micros,
			agent_pool_rate_bps,
			qualification_direct_user_count,
			qualification_min_user_consumption_micros,
			qualification_direct_team_consumption_micros,
			qualification_combined_consumption_micros,
			max_campaign_links,
			commission_conversion_multiplier_millis,
			withdrawal_min_micros,
			withdrawal_sla_hours,
			margin_floor_bps,
			operational_reserve_bps,
			stress_cost_per_raw_credit_micros,
			revision,
			updated_by,
			created_at,
			updated_at
		FROM affiliate_program_settings
		WHERE id = 1
	`, nil,
		&settings.ID,
		&settings.ProgramVersion,
		&settings.Mode,
		&startedAt,
		&settings.OrdinaryReferralRateBPS,
		&settings.OrdinaryInviteeRateBPS,
		&settings.FirstPaidBonusThresholdMicros,
		&settings.FirstPaidBonusMicros,
		&settings.AgentPoolRateBPS,
		&settings.QualificationDirectUserCount,
		&settings.QualificationMinUserConsumptionMicros,
		&settings.QualificationDirectTeamConsumptionMicros,
		&settings.QualificationCombinedConsumptionMicros,
		&settings.MaxCampaignLinks,
		&settings.CommissionConversionMultiplierMillis,
		&settings.WithdrawalMinMicros,
		&settings.WithdrawalSLAHours,
		&settings.MarginFloorBPS,
		&settings.OperationalReserveBPS,
		&settings.StressCostPerRawCreditMicros,
		&settings.Revision,
		&updatedBy,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get affiliate program settings: %w", err)
	}
	if startedAt.Valid {
		settings.StartedAt = &startedAt.Time
	}
	if updatedBy.Valid {
		settings.UpdatedBy = &updatedBy.Int64
	}
	return &settings, nil
}

func (r *affiliateProgramRepository) UpdateSettings(
	ctx context.Context,
	settings *service.AffiliateProgramSettings,
	expectedRevision int64,
) error {
	if r.db == nil {
		return errors.New("affiliate program repository: nil database")
	}
	if settings == nil {
		return errors.New("affiliate program repository: nil settings")
	}

	var revision int64
	var updatedAt sql.NullTime
	err := scanSingleRow(ctx, r.db, `
		UPDATE affiliate_program_settings
		SET
			mode = $1,
			started_at = $2,
			ordinary_referral_rate_bps = $3,
			ordinary_invitee_rate_bps = $4,
			first_paid_bonus_threshold_micros = $5,
			first_paid_bonus_micros = $6,
			agent_pool_rate_bps = $7,
			qualification_direct_user_count = $8,
			qualification_min_user_consumption_micros = $9,
			qualification_direct_team_consumption_micros = $10,
			qualification_combined_consumption_micros = $11,
			max_campaign_links = $12,
			commission_conversion_multiplier_millis = $13,
			withdrawal_min_micros = $14,
			withdrawal_sla_hours = $15,
			margin_floor_bps = $16,
			operational_reserve_bps = $17,
			stress_cost_per_raw_credit_micros = $18,
			updated_by = $19,
			revision = revision + 1,
			updated_at = NOW()
		WHERE id = 1 AND revision = $20
		RETURNING revision, updated_at
	`, []any{
		settings.Mode,
		settings.StartedAt,
		settings.OrdinaryReferralRateBPS,
		settings.OrdinaryInviteeRateBPS,
		settings.FirstPaidBonusThresholdMicros,
		settings.FirstPaidBonusMicros,
		settings.AgentPoolRateBPS,
		settings.QualificationDirectUserCount,
		settings.QualificationMinUserConsumptionMicros,
		settings.QualificationDirectTeamConsumptionMicros,
		settings.QualificationCombinedConsumptionMicros,
		settings.MaxCampaignLinks,
		settings.CommissionConversionMultiplierMillis,
		settings.WithdrawalMinMicros,
		settings.WithdrawalSLAHours,
		settings.MarginFloorBPS,
		settings.OperationalReserveBPS,
		settings.StressCostPerRawCreditMicros,
		settings.UpdatedBy,
		expectedRevision,
	}, &revision, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return service.ErrAffiliateProgramRevisionConflict
	}
	if err != nil {
		return fmt.Errorf("update affiliate program settings: %w", err)
	}
	settings.Revision = revision
	if updatedAt.Valid {
		settings.UpdatedAt = updatedAt.Time
	}
	return nil
}

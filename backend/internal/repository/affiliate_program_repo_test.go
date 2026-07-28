package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateProgramRepositoryGetSettings(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	createdAt := time.Date(2026, 7, 26, 0, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	columns := []string{
		"id", "program_version", "mode", "started_at",
		"ordinary_referral_rate_bps", "ordinary_invitee_rate_bps",
		"first_paid_bonus_threshold_micros", "first_paid_bonus_micros",
		"agent_pool_rate_bps", "qualification_direct_user_count",
		"qualification_min_user_consumption_micros", "qualification_direct_team_consumption_micros",
		"qualification_combined_consumption_micros", "max_campaign_links",
		"commission_conversion_multiplier_millis", "withdrawal_min_micros", "withdrawal_sla_hours",
		"margin_floor_bps", "operational_reserve_bps",
		"stress_cost_per_raw_credit_micros",
		"revision", "updated_by", "created_at", "updated_at",
	}
	mock.ExpectQuery(`(?s)SELECT\s+id,.*FROM affiliate_program_settings`).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			int16(1), "v3", "off", nil,
			int32(500), int32(500), int64(0), int64(0),
			int32(1000), int32(10), int64(20_000_000), int64(1_000_000_000),
			int64(2_000_000_000), int32(5), int32(1200), int64(100_000_000), int32(24),
			int32(3500), int32(200), int64(530_000),
			int64(1), nil, createdAt, updatedAt,
		))

	repo := NewAffiliateProgramRepository(db)
	settings, err := repo.GetSettings(context.Background())
	require.NoError(t, err)
	require.NoError(t, settings.Validate())
	require.Nil(t, settings.StartedAt)
	require.Nil(t, settings.UpdatedBy)
	require.Equal(t, updatedAt, settings.UpdatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAffiliateProgramRepositoryUpdateSettingsUsesRevisionLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	settings := service.DefaultAffiliateProgramSettings()
	settings.Mode = service.AffiliateProgramModeShadow
	actorID := int64(7)
	settings.UpdatedBy = &actorID
	updatedAt := time.Now().UTC()

	mock.ExpectQuery(`(?s)UPDATE affiliate_program_settings.*WHERE id = 1 AND revision = \$20`).
		WithArgs(
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
			int64(1),
		).
		WillReturnRows(sqlmock.NewRows([]string{"revision", "updated_at"}).AddRow(int64(2), updatedAt))

	repo := NewAffiliateProgramRepository(db)
	err = repo.UpdateSettings(context.Background(), &settings, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), settings.Revision)
	require.Equal(t, updatedAt, settings.UpdatedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAffiliateProgramRepositoryUpdateSettingsDetectsConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	settings := service.DefaultAffiliateProgramSettings()
	mock.ExpectQuery(`(?s)UPDATE affiliate_program_settings.*RETURNING revision, updated_at`).
		WillReturnError(sql.ErrNoRows)

	repo := NewAffiliateProgramRepository(db)
	err = repo.UpdateSettings(context.Background(), &settings, 99)
	require.ErrorIs(t, err, service.ErrAffiliateProgramRevisionConflict)
	require.NoError(t, mock.ExpectationsWereMet())
}

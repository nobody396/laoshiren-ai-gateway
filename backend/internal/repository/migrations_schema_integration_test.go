//go:build integration

package repository

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"

	embeddedmigrations "github.com/bozhouDev/DragonCode-sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMigrationsRunner_IsIdempotent_AndSchemaIsUpToDate(t *testing.T) {
	tx := testTx(t)

	// Re-apply migrations to verify idempotency (no errors, no duplicate rows).
	require.NoError(t, ApplyMigrations(context.Background(), integrationDB))

	// The integration harness starts from an empty schema. Every prerequisite,
	// including the immutable migration-138 compatibility seed, must therefore
	// come from the embedded forward-only migration stream itself.
	var applied int
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM schema_migrations").Scan(&applied))
	require.Equal(t, nonEmptyEmbeddedMigrationCount(t), applied, "every non-empty embedded migration must be recorded exactly once")

	// users: columns required by repository queries
	requireColumn(t, tx, "users", "username", "character varying", 100, false)
	requireColumn(t, tx, "users", "notes", "text", 0, false)
	requireColumn(t, tx, "users", "token_version", "bigint", 0, false)

	// accounts: schedulable and rate-limit fields
	requireColumn(t, tx, "accounts", "notes", "text", 0, true)
	requireColumn(t, tx, "accounts", "schedulable", "boolean", 0, false)
	requireColumn(t, tx, "accounts", "rate_limited_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "rate_limit_reset_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "overload_until", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "accounts", "session_window_status", "character varying", 20, true)

	// groups: OpenAI Live 默认关闭，管理员显式开启后才可访问。
	requireColumn(t, tx, "groups", "allow_live", "boolean", 0, false)

	// api_keys: key length should be 128
	requireColumn(t, tx, "api_keys", "key", "character varying", 128, false)

	// redeem_codes: subscription fields
	requireColumn(t, tx, "redeem_codes", "group_id", "bigint", 0, true)
	requireColumn(t, tx, "redeem_codes", "validity_days", "integer", 0, false)

	// usage_logs: billing_type used by filters/stats
	requireColumn(t, tx, "usage_logs", "billing_type", "smallint", 0, false)
	requireColumn(t, tx, "usage_logs", "request_type", "smallint", 0, false)
	requireColumn(t, tx, "usage_logs", "openai_ws_mode", "boolean", 0, false)
	requireColumn(t, tx, "usage_logs", "video_count", "integer", 0, false)
	requireColumn(t, tx, "usage_logs", "video_resolution", "character varying", 10, true)
	requireColumn(t, tx, "usage_logs", "video_duration_seconds", "integer", 0, true)

	// openai_route_shadow_decisions: durable, append-only routing evidence
	requireColumn(t, tx, "openai_route_shadow_decisions", "decision_id", "character varying", 64, false)
	requireColumn(t, tx, "openai_route_shadow_decisions", "request_id", "character varying", 128, false)
	requireColumn(t, tx, "openai_route_shadow_decisions", "client_request_id", "character varying", 128, false)
	requireColumn(t, tx, "openai_route_shadow_decisions", "request_class", "character varying", 16, false)
	requireColumn(t, tx, "openai_route_shadow_decisions", "activation_id", "character varying", 128, false)
	requireColumn(t, tx, "openai_route_shadow_decisions", "shadow_started_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "openai_route_shadow_decisions", "snapshot", "jsonb", 0, false)
	requireColumn(t, tx, "openai_route_shadow_decisions", "created_at", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "openai_route_shadow_decisions", "idx_openai_route_shadow_decisions_created_at")
	requireIndex(t, tx, "openai_route_shadow_decisions", "idx_openai_route_shadow_decisions_group_model_created")
	requireIndex(t, tx, "openai_route_shadow_decisions", "idx_openai_route_shadow_decisions_group_model_class_created")
	requireIndex(t, tx, "openai_route_shadow_decisions", "idx_openai_route_shadow_decisions_activation_scope")

	// openai_route_observation_hourly: durable, non-sensitive aggregate checkpoints
	requireColumn(t, tx, "openai_route_observation_hourly", "route_fingerprint", "character varying", 32, false)
	requireColumn(t, tx, "openai_route_observation_hourly", "hour_start", "timestamp with time zone", 0, false)
	requireColumn(t, tx, "openai_route_observation_hourly", "request_class", "character varying", 16, false)
	requireColumn(t, tx, "openai_route_observation_hourly", "metrics", "jsonb", 0, false)
	requireColumn(t, tx, "openai_route_observation_hourly", "last_observed_at", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "openai_route_observation_hourly", "openai_route_observation_hourly_pkey")
	requireIndex(t, tx, "openai_route_observation_hourly", "idx_openai_route_observation_hourly_hour_start")

	// groups: Grok video billing controls (migration 173)
	requireColumn(t, tx, "groups", "video_rate_independent", "boolean", 0, false)
	requireColumn(t, tx, "groups", "video_rate_multiplier", "numeric", 0, false)
	requireColumn(t, tx, "groups", "video_price_480p", "numeric", 0, true)
	requireColumn(t, tx, "groups", "video_price_720p", "numeric", 0, true)
	requireColumn(t, tx, "groups", "video_price_1080p", "numeric", 0, true)

	// usage_billing_dedup: billing idempotency narrow table
	var usageBillingDedupRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.usage_billing_dedup')").Scan(&usageBillingDedupRegclass))
	require.True(t, usageBillingDedupRegclass.Valid, "expected usage_billing_dedup table to exist")
	requireColumn(t, tx, "usage_billing_dedup", "request_fingerprint", "character varying", 64, false)
	requireIndex(t, tx, "usage_billing_dedup", "idx_usage_billing_dedup_request_api_key")
	requireIndex(t, tx, "usage_billing_dedup", "idx_usage_billing_dedup_created_at_brin")

	var usageBillingDedupArchiveRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.usage_billing_dedup_archive')").Scan(&usageBillingDedupArchiveRegclass))
	require.True(t, usageBillingDedupArchiveRegclass.Valid, "expected usage_billing_dedup_archive table to exist")
	requireColumn(t, tx, "usage_billing_dedup_archive", "request_fingerprint", "character varying", 64, false)
	requireIndex(t, tx, "usage_billing_dedup_archive", "usage_billing_dedup_archive_pkey")

	// settings table should exist
	var settingsRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.settings')").Scan(&settingsRegclass))
	require.True(t, settingsRegclass.Valid, "expected settings table to exist")

	// security_secrets table should exist
	var securitySecretsRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.security_secrets')").Scan(&securitySecretsRegclass))
	require.True(t, securitySecretsRegclass.Valid, "expected security_secrets table to exist")

	// user_allowed_groups table should exist
	var uagRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.user_allowed_groups')").Scan(&uagRegclass))
	require.True(t, uagRegclass.Valid, "expected user_allowed_groups table to exist")

	// user_subscriptions: deleted_at for soft delete support (migration 012)
	requireColumn(t, tx, "user_subscriptions", "deleted_at", "timestamp with time zone", 0, true)

	// orphan_allowed_groups_audit table should exist (migration 013)
	var orphanAuditRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.orphan_allowed_groups_audit')").Scan(&orphanAuditRegclass))
	require.True(t, orphanAuditRegclass.Valid, "expected orphan_allowed_groups_audit table to exist")

	// account_groups: created_at should be timestamptz
	requireColumn(t, tx, "account_groups", "created_at", "timestamp with time zone", 0, false)

	// user_allowed_groups: created_at should be timestamptz
	requireColumn(t, tx, "user_allowed_groups", "created_at", "timestamp with time zone", 0, false)

	// migration 142: legacy rollback compensation and old-image compatibility
	requireColumn(t, tx, "users", "wechat", "character varying", 100, false)

	var activeWechatDefinitions int
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM user_attribute_definitions
WHERE key = 'wechat' AND deleted_at IS NULL AND enabled = true
`).Scan(&activeWechatDefinitions))
	require.Equal(t, 1, activeWechatDefinitions, "expected exactly one active wechat attribute definition")

	var opsAlertSilencesRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), "SELECT to_regclass('public.ops_alert_silences')").Scan(&opsAlertSilencesRegclass))
	require.True(t, opsAlertSilencesRegclass.Valid, "expected ops_alert_silences table to exist")
	requireIndex(t, tx, "ops_alert_silences", "idx_ops_alert_silences_lookup")

	var unsafeFreshCompatibilityGroups int
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM groups
WHERE description = '[fresh-install compatibility template] Disabled source for the immutable Apex group migration.'
  AND status <> 'disabled'
`).Scan(&unsafeFreshCompatibilityGroups))
	require.Zero(t, unsafeFreshCompatibilityGroups, "fresh-install compatibility groups must remain disabled")

	// migration 149: Affiliate V2 is additive, fixed-point, and disabled by default.
	for _, table := range []string{
		"affiliate_program_settings",
		"agent_principals",
		"affiliate_links",
		"affiliate_link_rate_versions",
		"affiliate_bindings",
		"affiliate_reward_entries",
		"affiliate_performance_events",
		"affiliate_qualification_states",
		"agent_cash_commission_entries",
	} {
		var regclass sql.NullString
		require.NoError(t, tx.QueryRowContext(
			context.Background(),
			"SELECT to_regclass('public.' || $1)",
			table,
		).Scan(&regclass))
		require.True(t, regclass.Valid, "expected %s table to exist", table)
	}
	requireColumn(t, tx, "affiliate_program_settings", "withdrawal_min_micros", "bigint", 0, false)
	requireColumn(t, tx, "affiliate_reward_entries", "amount_micros", "bigint", 0, false)
	requireColumn(t, tx, "agent_cash_commission_entries", "amount_micros", "bigint", 0, false)

	var affiliateMode, affiliateVersion string
	var agentPoolRateBPS, marginFloorBPS int
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT mode, program_version, agent_pool_rate_bps, margin_floor_bps
FROM affiliate_program_settings
WHERE id = 1
`).Scan(&affiliateMode, &affiliateVersion, &agentPoolRateBPS, &marginFloorBPS))
	require.Equal(t, "off", affiliateMode)
	require.Equal(t, "v3", affiliateVersion)
	require.Equal(t, 1000, agentPoolRateBPS)
	require.Equal(t, 3500, marginFloorBPS)

	// migration 150: source-aware paid balance/monthly-card attribution.
	for _, table := range []string{
		"balance_lots",
		"balance_lot_consumptions",
		"monthly_entitlement_cycles",
		"monthly_entitlement_cycle_subscriptions",
	} {
		var regclass sql.NullString
		require.NoError(t, tx.QueryRowContext(
			context.Background(),
			"SELECT to_regclass('public.' || $1)",
			table,
		).Scan(&regclass))
		require.True(t, regclass.Valid, "expected %s table to exist", table)
	}
	requireColumn(t, tx, "balance_lots", "remaining_amount_micros", "bigint", 0, false)
	requireColumn(t, tx, "balance_lot_consumptions", "affiliate_eligible_amount_micros", "bigint", 0, false)
	requireColumn(t, tx, "monthly_entitlement_cycles", "confirmed_consumption_micros", "bigint", 0, false)
	requireIndex(t, tx, "balance_lots", "idx_balance_lots_fifo")
	requireIndex(t, tx, "monthly_entitlement_cycle_subscriptions", "idx_monthly_cycle_subscription_lookup")

	// migration 151: first-paid reward claim and maturity.
	var firstPaidRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(
		context.Background(),
		"SELECT to_regclass('public.affiliate_first_paid_purchases')",
	).Scan(&firstPaidRegclass))
	require.True(t, firstPaidRegclass.Valid)
	requireColumn(t, tx, "affiliate_first_paid_purchases", "amount_micros", "bigint", 0, false)
	requireColumn(t, tx, "affiliate_reward_entries", "posted_at", "timestamp with time zone", 0, true)

	// migration 152: dynamic link lookup and direct-edge indexes.
	requireIndex(t, tx, "affiliate_links", "idx_affiliate_links_code_active")
	requireIndex(t, tx, "affiliate_bindings", "idx_affiliate_bindings_agent_link")

	// migration 153: bounded direct-team qualification scans.
	requireIndex(t, tx, "affiliate_performance_events", "idx_affiliate_performance_direct_consumption")
	requireIndex(t, tx, "affiliate_performance_events", "idx_affiliate_performance_user_consumption")

	// migration 154: reviewed Alipay profiles and one-principal identity guard.
	requireColumn(t, tx, "agent_payment_profiles", "verification_status", "character varying", 24, false)
	requireColumn(t, tx, "agent_payment_profiles", "identity_fingerprint_hash", "character varying", 64, false)
	requireIndex(t, tx, "agent_payment_profiles", "uq_agent_payment_verified_identity")
	requireIndex(t, tx, "agent_payment_profiles", "idx_agent_payment_verification_queue")

	// migration 155: private agent-community image + text configuration.
	var affiliateCommunityRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(
		context.Background(),
		"SELECT to_regclass('public.affiliate_community_settings')",
	).Scan(&affiliateCommunityRegclass))
	require.True(t, affiliateCommunityRegclass.Valid)
	requireColumn(t, tx, "affiliate_community_settings", "revision", "bigint", 0, false)

	// migration 156: on-demand withdrawal, conversion, and exception notices.
	for _, table := range []string{
		"agent_withdrawal_requests",
		"agent_commission_conversions",
		"affiliate_agent_notices",
	} {
		var regclass sql.NullString
		require.NoError(t, tx.QueryRowContext(
			context.Background(),
			"SELECT to_regclass('public.' || $1)",
			table,
		).Scan(&regclass))
		require.True(t, regclass.Valid, "expected %s table to exist", table)
	}
	requireColumn(t, tx, "agent_withdrawal_requests", "amount_micros", "bigint", 0, false)
	requireIndex(t, tx, "agent_withdrawal_requests", "idx_agent_withdrawals_processing_due")
	requireIndex(t, tx, "agent_commission_conversions", "idx_agent_conversions_agent_time")
	requireIndex(t, tx, "affiliate_agent_notices", "idx_affiliate_agent_notices_unread")

	// migration 157: risk release, exactly-once reversals, and withdrawal audit.
	for _, table := range []string{
		"affiliate_risk_actions",
		"affiliate_performance_reversals",
		"agent_withdrawal_events",
		"agent_payment_qr_access_events",
	} {
		var regclass sql.NullString
		require.NoError(t, tx.QueryRowContext(
			context.Background(),
			"SELECT to_regclass('public.' || $1)",
			table,
		).Scan(&regclass))
		require.True(t, regclass.Valid, "expected %s table to exist", table)
	}
	requireColumn(t, tx, "affiliate_risk_actions", "released_cash_micros", "bigint", 0, false)
	requireColumn(t, tx, "affiliate_performance_reversals", "original_event_id", "bigint", 0, false)
	requireIndex(t, tx, "affiliate_reward_entries", "uq_affiliate_reward_reversal")
	requireIndex(t, tx, "agent_cash_commission_entries", "uq_agent_cash_reversal")
	requireIndex(t, tx, "agent_withdrawal_events", "uq_agent_withdrawal_event_once")
	requireIndex(t, tx, "agent_payment_qr_access_events", "idx_agent_payment_qr_access_agent_time")

	// migration 160: V3 manual review, immutable source policy, and margin inputs.
	for _, table := range []string{
		"affiliate_agent_applications",
		"affiliate_agent_status_events",
	} {
		var regclass sql.NullString
		require.NoError(t, tx.QueryRowContext(
			context.Background(),
			"SELECT to_regclass('public.' || $1)",
			table,
		).Scan(&regclass))
		require.True(t, regclass.Valid, "expected %s table to exist", table)
	}
	requireColumn(t, tx, "affiliate_program_settings", "ordinary_invitee_rate_bps", "integer", 0, false)
	requireColumn(t, tx, "affiliate_program_settings", "stress_cost_per_raw_credit_micros", "bigint", 0, false)
	requireColumn(t, tx, "agent_principals", "applied_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "balance_lots", "affiliate_policy", "character varying", 32, false)
	requireColumn(t, tx, "monthly_entitlement_cycles", "affiliate_policy", "character varying", 32, false)
	requireColumn(t, tx, "monthly_entitlement_cycles", "pricing_table_version", "character varying", 32, false)
	requireColumn(t, tx, "affiliate_performance_events", "affiliate_policy", "character varying", 32, false)
	requireIndex(t, tx, "affiliate_agent_applications", "uq_affiliate_application_pending")
	requireIndex(t, tx, "affiliate_agent_status_events", "idx_affiliate_status_events_agent_time")
	requireIndex(t, tx, "agent_withdrawal_requests", "uq_agent_withdrawal_payment_reference")

	var ordinaryReferralRateBPS, ordinaryInviteeRateBPS, fixedBonusMicros, conversionMillis, reserveBPS int
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT
    ordinary_referral_rate_bps,
    ordinary_invitee_rate_bps,
    first_paid_bonus_micros,
    commission_conversion_multiplier_millis,
    operational_reserve_bps
FROM affiliate_program_settings
WHERE id = 1
`).Scan(
		&ordinaryReferralRateBPS,
		&ordinaryInviteeRateBPS,
		&fixedBonusMicros,
		&conversionMillis,
		&reserveBPS,
	))
	require.Equal(t, 500, ordinaryReferralRateBPS)
	require.Equal(t, 500, ordinaryInviteeRateBPS)
	require.Zero(t, fixedBonusMicros)
	require.Equal(t, 1200, conversionMillis)
	require.Equal(t, 200, reserveBPS)

	// migration 161: additive monthly-only V3 groups.  Historical groups stay
	// untouched while new cards target these six rows.
	for _, groupName := range []string{
		"GPT Plus 月卡组", "Claude Plus 月卡组",
		"GPT Pro V3 月卡组", "Claude Pro V3 月卡组",
		"GPT Max V3 月卡组", "Claude Max V3 月卡组",
	} {
		var count int
		var description string
		var daily, weekly sql.NullFloat64
		var validity int
		require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT COUNT(*), MAX(description), MAX(daily_limit_usd), MAX(weekly_limit_usd), MAX(default_validity_days)
FROM groups
WHERE deleted_at IS NULL AND name = $1
`, groupName).Scan(&count, &description, &daily, &weekly, &validity))
		require.Equal(t, 1, count, "expected one active V3 group %s", groupName)
		require.Empty(t, description, "V3 group %s description must stay empty", groupName)
		require.False(t, daily.Valid, "V3 group %s must not have a daily limit", groupName)
		require.False(t, weekly.Valid, "V3 group %s must not have a weekly limit", groupName)
		require.Equal(t, 31, validity)
	}

	// migration 162: the persisted qualification-state constraint must accept
	// the V3 direct-volume route used by manual approval.
	var qualificationRouteConstraint string
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conrelid = 'affiliate_qualification_states'::regclass
  AND conname = 'chk_affiliate_qualification_route'
`).Scan(&qualificationRouteConstraint))
	require.Contains(t, qualificationRouteConstraint, "direct_volume")
	require.NotContains(t, qualificationRouteConstraint, "'combined'")

	// migration 163: the manual conservative cost remains guarded by the
	// margin floor, without a fake time-based snapshot expiry.
	requireColumnAbsent(t, tx, "affiliate_program_settings", "stress_cost_snapshot_at")
	requireColumnAbsent(t, tx, "affiliate_program_settings", "cost_snapshot_max_age_hours")
	var programVersionDefault sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT column_default
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'affiliate_program_settings'
  AND column_name = 'program_version'
`).Scan(&programVersionDefault))
	require.True(t, programVersionDefault.Valid)
	require.Contains(t, programVersionDefault.String, "'v3'")

	// migration 165: separate consent audit for sensitive payout information.
	requireColumn(t, tx, "agent_payment_profiles", "privacy_consent_version", "character varying", 80, false)
	requireColumn(t, tx, "agent_payment_profiles", "privacy_consented_at", "timestamp with time zone", 0, true)

	// migration 166: immutable, qualification-only historical baseline. Values
	// are internal 1-unit-equals-1-CNY micros and never use an FX conversion.
	var qualificationBaselineRegclass sql.NullString
	require.NoError(t, tx.QueryRowContext(
		context.Background(),
		"SELECT to_regclass('public.affiliate_qualification_baseline_entries')",
	).Scan(&qualificationBaselineRegclass))
	require.True(t, qualificationBaselineRegclass.Valid)
	requireColumn(t, tx, "affiliate_qualification_baseline_entries", "confirmed_consumption_micros", "bigint", 0, false)
	requireColumn(t, tx, "affiliate_qualification_baseline_entries", "cutoff_at", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "affiliate_qualification_baseline_entries", "idx_affiliate_qualification_baseline_user")
	var refreshFunctionRegprocedure sql.NullString
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT to_regprocedure(
	'refresh_affiliate_qualification_legacy_baseline(timestamp with time zone)'
)
`).Scan(&refreshFunctionRegprocedure))
	require.True(t, refreshFunctionRegprocedure.Valid)

	// migration 167: review records preserve the complete Route B snapshot.
	requireColumn(t, tx, "affiliate_agent_applications", "self_consumption_micros", "bigint", 0, false)
	requireColumn(t, tx, "affiliate_agent_applications", "combined_consumption_micros", "bigint", 0, false)

	// migration 168: Route B is now based on the applicant's own confirmed
	// consumption. Legacy route/threshold fields remain rollback-compatible.
	requireColumn(t, tx, "affiliate_program_settings", "qualification_self_consumption_micros", "bigint", 0, false)
	var (
		directUserCount  int
		perUserMicros    int64
		directTeamMicros int64
		selfMicros       int64
	)
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT
	qualification_direct_user_count,
	qualification_min_user_consumption_micros,
	qualification_direct_team_consumption_micros,
	qualification_self_consumption_micros
FROM affiliate_program_settings
WHERE id = 1
`).Scan(&directUserCount, &perUserMicros, &directTeamMicros, &selfMicros))
	require.Equal(t, 5, directUserCount)
	require.Equal(t, int64(20_000_000), perUserMicros)
	require.Equal(t, int64(1_000_000_000), directTeamMicros)
	require.Equal(t, int64(500_000_000), selfMicros)

	var applicationRouteConstraint string
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conrelid = 'affiliate_agent_applications'::regclass
  AND conname = 'chk_affiliate_application_route'
`).Scan(&applicationRouteConstraint))
	require.Contains(t, applicationRouteConstraint, "self_consumption")
	require.Contains(t, applicationRouteConstraint, "direct_volume")

	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT pg_get_constraintdef(oid)
FROM pg_constraint
WHERE conrelid = 'affiliate_qualification_states'::regclass
  AND conname = 'chk_affiliate_qualification_route'
`).Scan(&qualificationRouteConstraint))
	require.Contains(t, qualificationRouteConstraint, "self_consumption")

	// migration 170: promotion value is locked on the order so historical
	// ¥500/¥1000 orders are never retroactively treated as promotional.
	requireColumn(t, tx, "topup_orders", "bonus_amount_cny_fen", "integer", 0, false)

	// migration 171: sellable redeem codes preserve actual cash separately
	// from promotional balance credited.
	requireColumn(t, tx, "redeem_codes", "paid_value", "numeric", 0, false)

	// migrations 185-190: native card-shop checkout owns a durable once-per-user
	// order, restricts its inventory, snapshots the selected payment method, and
	// configures the single approved ¥5 -> ¥10 pure-gift newcomer offer. Native
	// checkout remains disabled while manual card redemption is enabled with an
	// atomic lifetime claim.
	requireColumn(t, tx, "native_checkout_offers", "provider_goods_key", "character varying", 64, false)
	requireColumn(t, tx, "native_checkout_offers", "manual_redeem_enabled", "boolean", 0, false)
	requireColumn(t, tx, "native_checkout_orders", "contact_hash", "character", 64, false)
	requireColumn(t, tx, "native_checkout_orders", "payment_method", "character varying", 16, true)
	requireColumn(t, tx, "native_checkout_orders", "redeem_code_id", "bigint", 0, true)
	requireColumn(t, tx, "native_checkout_offer_testers", "user_id", "bigint", 0, false)
	requireColumn(t, tx, "native_checkout_redeem_inventory", "assigned_order_id", "bigint", 0, true)
	requireColumn(t, tx, "native_checkout_manual_claims", "redeem_code_id", "bigint", 0, false)
	requireIndex(t, tx, "native_checkout_orders", "uq_native_checkout_orders_once_per_user")
	requireIndex(t, tx, "native_checkout_orders", "uq_native_checkout_orders_active_per_user")
	requireIndex(t, tx, "native_checkout_offer_testers", "idx_native_checkout_offer_testers_user")
	requireIndex(t, tx, "native_checkout_redeem_inventory", "idx_native_checkout_redeem_inventory_offer_unassigned")
	requireIndex(t, tx, "native_checkout_manual_claims", "idx_native_checkout_manual_claims_user")

	var (
		providerGoodsKey string
		payFen           int64
		benefitFen       int64
		redeemValue      float64
		paidValue        float64
		purpose          string
		salesStatus      string
		validityDays     int
		oncePerUser      bool
		enabled          bool
		manualRedeem     bool
	)
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT provider_goods_key, pay_amount_cny_fen, benefit_amount_cny_fen,
       redeem_value::double precision, redeem_paid_value::double precision,
       redeem_purpose, redeem_sales_status, redeem_validity_days,
       once_per_user, enabled, manual_redeem_enabled
FROM native_checkout_offers
WHERE code = 'newcomer-balance-5-to-10'
`).Scan(
		&providerGoodsKey, &payFen, &benefitFen, &redeemValue, &paidValue,
		&purpose, &salesStatus, &validityDays, &oncePerUser, &enabled, &manualRedeem,
	))
	require.Equal(t, "oc3w4r", providerGoodsKey)
	require.Equal(t, int64(500), payFen)
	require.Equal(t, int64(1000), benefitFen)
	require.Equal(t, float64(10), redeemValue)
	require.Zero(t, paidValue, "the full ¥10 entitlement must be pure gift balance")
	require.Equal(t, "gift", purpose)
	require.Equal(t, "gifted", salesStatus)
	require.Zero(t, validityDays)
	require.True(t, oncePerUser)
	require.False(t, enabled)
	require.True(t, manualRedeem)
	var testerCount int
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT COUNT(*) FROM native_checkout_offer_testers
WHERE offer_code = 'newcomer-balance-5-to-10'
`).Scan(&testerCount))
	require.Zero(t, testerCount, "the final gate migration must not pre-authorize any test account")

	var inventoryMatchTrigger, stockedOfferGuardTrigger, manualClaimTrigger bool
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
    SELECT 1 FROM pg_trigger
    WHERE tgrelid = 'native_checkout_redeem_inventory'::regclass
      AND tgname = 'trg_native_checkout_inventory_offer_match'
      AND NOT tgisinternal
)
`).Scan(&inventoryMatchTrigger))
	require.True(t, inventoryMatchTrigger)
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
    SELECT 1 FROM pg_trigger
    WHERE tgrelid = 'native_checkout_offers'::regclass
      AND tgname = 'trg_native_checkout_stocked_offer_semantics'
      AND NOT tgisinternal
)
`).Scan(&stockedOfferGuardTrigger))
	require.True(t, stockedOfferGuardTrigger)
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
    SELECT 1 FROM pg_trigger
    WHERE tgrelid = 'redeem_codes'::regclass
      AND tgname = 'trg_claim_manual_checkout_offer_once'
      AND NOT tgisinternal
)
`).Scan(&manualClaimTrigger))
	require.True(t, manualClaimTrigger)
}

func nonEmptyEmbeddedMigrationCount(t *testing.T) int {
	t.Helper()

	files, err := fs.Glob(embeddedmigrations.FS, "*.sql")
	require.NoError(t, err)

	count := 0
	for _, name := range files {
		content, readErr := fs.ReadFile(embeddedmigrations.FS, name)
		require.NoError(t, readErr, name)
		if strings.TrimSpace(string(content)) != "" {
			count++
		}
	}
	return count
}

func requireIndex(t *testing.T, tx *sql.Tx, table, index string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1
	FROM pg_indexes
	WHERE schemaname = 'public'
	  AND tablename = $1
	  AND indexname = $2
)
`, table, index).Scan(&exists)
	require.NoError(t, err, "query pg_indexes for %s.%s", table, index)
	require.True(t, exists, "expected index %s on %s", index, table)
}

func requireColumn(t *testing.T, tx *sql.Tx, table, column, dataType string, maxLen int, nullable bool) {
	t.Helper()

	var row struct {
		DataType string
		MaxLen   sql.NullInt64
		Nullable string
	}

	err := tx.QueryRowContext(context.Background(), `
SELECT
  data_type,
  character_maximum_length,
  is_nullable
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = $1
  AND column_name = $2
`, table, column).Scan(&row.DataType, &row.MaxLen, &row.Nullable)
	require.NoError(t, err, "query information_schema.columns for %s.%s", table, column)
	require.Equal(t, dataType, row.DataType, "data_type mismatch for %s.%s", table, column)

	if maxLen > 0 {
		require.True(t, row.MaxLen.Valid, "expected maxLen for %s.%s", table, column)
		require.Equal(t, int64(maxLen), row.MaxLen.Int64, "maxLen mismatch for %s.%s", table, column)
	}

	if nullable {
		require.Equal(t, "YES", row.Nullable, "nullable mismatch for %s.%s", table, column)
	} else {
		require.Equal(t, "NO", row.Nullable, "nullable mismatch for %s.%s", table, column)
	}
}

func requireColumnAbsent(t *testing.T, tx *sql.Tx, table, column string) {
	t.Helper()

	var exists bool
	err := tx.QueryRowContext(context.Background(), `
SELECT EXISTS (
	SELECT 1
	FROM information_schema.columns
	WHERE table_schema = 'public'
	  AND table_name = $1
	  AND column_name = $2
)
`, table, column).Scan(&exists)
	require.NoError(t, err, "query information_schema.columns for %s.%s", table, column)
	require.False(t, exists, "expected column %s.%s to be absent", table, column)
}

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

	// api_keys: key length should be 128
	requireColumn(t, tx, "api_keys", "key", "character varying", 128, false)

	// redeem_codes: subscription fields
	requireColumn(t, tx, "redeem_codes", "group_id", "bigint", 0, true)
	requireColumn(t, tx, "redeem_codes", "validity_days", "integer", 0, false)

	// usage_logs: billing_type used by filters/stats
	requireColumn(t, tx, "usage_logs", "billing_type", "smallint", 0, false)
	requireColumn(t, tx, "usage_logs", "request_type", "smallint", 0, false)
	requireColumn(t, tx, "usage_logs", "openai_ws_mode", "boolean", 0, false)

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
	require.Equal(t, "v2", affiliateVersion)
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

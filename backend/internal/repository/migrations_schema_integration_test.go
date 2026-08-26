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
	requireColumn(t, tx, "groups", "universal_routes", "jsonb", 0, false)

	// channel pricing: optional service-tier multipliers remain NULL until an
	// upstream's Fast/Flex capability and price are verified.
	requireColumn(t, tx, "channel_model_pricing", "fast_multiplier", "numeric", 0, true)
	requireColumn(t, tx, "channel_model_pricing", "flex_multiplier", "numeric", 0, true)
	requireColumn(t, tx, "channel_model_pricing", "fast_supported", "boolean", 0, false)
	requireColumn(t, tx, "channel_model_pricing", "flex_supported", "boolean", 0, false)
	requireColumn(t, tx, "channel_model_pricing", "fast_verified_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "channel_model_pricing", "flex_verified_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "channel_account_stats_model_pricing", "fast_multiplier", "numeric", 0, true)
	requireColumn(t, tx, "channel_account_stats_model_pricing", "flex_multiplier", "numeric", 0, true)

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
	requireColumn(t, tx, "openai_route_shadow_decisions", "access_group_id", "bigint", 0, true)
	requireColumn(t, tx, "openai_route_shadow_decisions", "inbound_protocol", "character varying", 32, false)
	requireColumn(t, tx, "openai_route_shadow_decisions", "requested_service_tier", "character varying", 16, false)
	requireColumn(t, tx, "openai_route_shadow_decisions", "activation_id", "character varying", 128, false)
	requireColumn(t, tx, "openai_route_shadow_decisions", "shadow_started_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "openai_route_shadow_decisions", "snapshot", "jsonb", 0, false)
	requireColumn(t, tx, "openai_route_shadow_decisions", "created_at", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "openai_route_shadow_decisions", "idx_openai_route_shadow_decisions_created_at")
	requireIndex(t, tx, "openai_route_shadow_decisions", "idx_openai_route_shadow_decisions_group_model_created")
	requireIndex(t, tx, "openai_route_shadow_decisions", "idx_openai_route_shadow_decisions_group_model_class_created")
	requireIndex(t, tx, "openai_route_shadow_decisions", "idx_openai_route_shadow_decisions_activation_scope")
	requireIndex(t, tx, "openai_route_shadow_decisions", "idx_openai_route_shadow_decisions_access_scope")

	// openai_route_observation_hourly: durable, non-sensitive aggregate checkpoints
	requireColumn(t, tx, "openai_route_observation_hourly", "route_fingerprint", "character varying", 32, false)
	requireColumn(t, tx, "openai_route_observation_hourly", "hour_start", "timestamp with time zone", 0, false)
	requireColumn(t, tx, "openai_route_observation_hourly", "request_class", "character varying", 16, false)
	requireColumn(t, tx, "openai_route_observation_hourly", "metrics", "jsonb", 0, false)
	requireColumn(t, tx, "openai_route_observation_hourly", "last_observed_at", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "openai_route_observation_hourly", "openai_route_observation_hourly_pkey")
	requireIndex(t, tx, "openai_route_observation_hourly", "idx_openai_route_observation_hourly_hour_start")

	// reliability_observations: normalized append-only customer/attempt/probe evidence.
	requireColumn(t, tx, "reliability_observations", "idempotency_key", "character varying", 180, false)
	requireColumn(t, tx, "reliability_observations", "fact_type", "character varying", 32, false)
	requireColumn(t, tx, "reliability_observations", "endpoint_hash", "character varying", 16, false)
	requireColumn(t, tx, "reliability_observations", "route_fingerprint", "character varying", 32, false)
	requireColumn(t, tx, "reliability_observations", "access_group_id", "bigint", 0, false)
	requireColumn(t, tx, "reliability_observations", "transport", "character varying", 32, false)
	requireColumn(t, tx, "reliability_observations", "routing_fingerprint", "character varying", 32, false)
	requireColumn(t, tx, "reliability_observations", "customer_impact", "boolean", 0, false)
	requireColumn(t, tx, "reliability_observations", "observed_at", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "reliability_observations", "reliability_observations_idempotency_key_key")
	requireIndex(t, tx, "reliability_observations", "idx_reliability_observations_observed")
	requireIndex(t, tx, "reliability_observations", "idx_reliability_observations_route_observed")
	requireIndex(t, tx, "reliability_observations", "idx_reliability_observations_routing_scope_observed")
	requireIndex(t, tx, "reliability_observations", "idx_reliability_observations_probe_route_observed")
	requireIndex(t, tx, "reliability_observations", "idx_reliability_observations_customer_client_request")
	requireIndex(t, tx, "reliability_observations", "idx_reliability_observations_customer_request")
	requireColumn(t, tx, "reliability_probe_claims", "claim_key", "character varying", 180, false)
	requireColumn(t, tx, "reliability_probe_claims", "route_fingerprint", "character varying", 32, false)
	requireColumn(t, tx, "reliability_probe_claims", "interval_start", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "reliability_probe_claims", "reliability_probe_claims_pkey")
	requireIndex(t, tx, "reliability_probe_claims", "reliability_probe_claim_route_interval_key")
	var sensitiveReliabilityColumns int
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name = 'reliability_observations'
  AND column_name = ANY(ARRAY['prompt','response_body','credential','raw_url','api_key','account_name','user_email'])
`).Scan(&sensitiveReliabilityColumns))
	require.Zero(t, sensitiveReliabilityColumns)
	var reliabilityEnabled string
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT value FROM settings WHERE key = 'reliability_observation_enabled'`).Scan(&reliabilityEnabled))
	require.Equal(t, "false", reliabilityEnabled)
	var impactIdentityConstraintExists, impactIdentityConstraintValidated bool
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT TRUE, convalidated
FROM pg_constraint
WHERE conrelid='reliability_observations'::regclass
  AND conname='reliability_observation_customer_identity_check'`).Scan(&impactIdentityConstraintExists, &impactIdentityConstraintValidated))
	require.True(t, impactIdentityConstraintExists)
	require.False(t, impactIdentityConstraintValidated, "historical append-only observations remain unchanged while new writes fail closed")

	// migration 202: explicit Status Catalog and default-off current state.
	requireColumn(t, tx, "service_status_families", "code", "character varying", 64, false)
	requireIndex(t, tx, "reliability_observations", "idx_reliability_observations_platform_model_observed")
	requireColumn(t, tx, "service_status_products", "critical", "boolean", 0, false)
	requireColumn(t, tx, "service_status_components", "access_mode", "character varying", 16, false)
	requireColumn(t, tx, "service_status_bindings", "binding_key", "character varying", 180, false)
	requireColumn(t, tx, "service_status_bindings", "group_name", "character varying", 100, false)
	requireColumn(t, tx, "service_status_bindings", "route_fingerprint", "character varying", 32, false)
	requireColumn(t, tx, "service_status_component_current", "computed_status", "character varying", 32, false)
	requireColumn(t, tx, "service_status_component_current", "recovery_confirmed_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "service_status_current", "effective_status", "character varying", 32, false)
	requireColumn(t, tx, "service_status_current", "computed_reason", "character varying", 64, false)
	requireColumn(t, tx, "service_status_current", "effective_reason", "character varying", 64, false)
	requireColumn(t, tx, "service_status_current", "recovery_confirmed_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "service_status_overrides", "expires_at", "timestamp with time zone", 0, false)
	requireIndex(t, tx, "service_status_products", "service_status_products_code_key")
	requireIndex(t, tx, "service_status_bindings", "service_status_bindings_binding_key_key")
	var statusEnabled, publicStatusEnabled string
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT value FROM settings WHERE key='service_status_enabled'`).Scan(&statusEnabled))
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT value FROM settings WHERE key='service_status_public_enabled'`).Scan(&publicStatusEnabled))
	require.Equal(t, "false", statusEnabled)
	require.Equal(t, "false", publicStatusEnabled)

	// migration 206: Incident Control is additive, auditable, and default-off.
	requireColumn(t, tx, "reliability_incident_candidates", "candidate_key", "character varying", 120, false)
	requireColumn(t, tx, "reliability_incident_candidates", "last_reconciled_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "reliability_incidents", "public_id", "character varying", 36, false)
	requireColumn(t, tx, "reliability_incidents", "phase", "character varying", 24, false)
	requireColumn(t, tx, "reliability_incidents", "last_reconciled_at", "timestamp with time zone", 0, true)
	requireColumn(t, tx, "reliability_incidents", "evidence_gap", "boolean", 0, false)
	requireColumn(t, tx, "reliability_incident_products", "current_status", "character varying", 32, false)
	requireColumn(t, tx, "reliability_incident_impact_segments", "started_at", "timestamp with time zone", 0, false)
	requireColumn(t, tx, "reliability_incident_observation_links", "observation_id", "bigint", 0, false)
	requireColumn(t, tx, "reliability_incident_public_timeline", "message", "character varying", 1000, false)
	requireColumn(t, tx, "reliability_incident_audit_log", "after_state", "jsonb", 0, false)
	requireColumn(t, tx, "reliability_incident_settings_audit", "after_state", "jsonb", 0, false)
	requireIndex(t, tx, "reliability_incident_impact_segments", "idx_reliability_incident_segments_one_open")
	requireIndex(t, tx, "reliability_incident_public_timeline", "idx_reliability_incident_public_timeline")
	var incidentsEnabled, incidentsPublicEnabled string
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT value FROM settings WHERE key='reliability_incidents_enabled'`).Scan(&incidentsEnabled))
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT value FROM settings WHERE key='reliability_incidents_public_enabled'`).Scan(&incidentsPublicEnabled))
	require.Equal(t, "false", incidentsEnabled)
	require.Equal(t, "false", incidentsPublicEnabled)

	// migration 208: Customer Tier evidence/history/overrides/snapshots.
	requireColumn(t, tx, "customer_tier_policy_versions", "priority_threshold_cny_fen", "bigint", 0, false)
	requireColumn(t, tx, "customer_tier_current", "effective_tier", "character varying", 16, false)
	requireColumn(t, tx, "customer_tier_evaluations", "evidence_hash", "character", 64, false)
	requireColumn(t, tx, "customer_tier_history", "evaluation_id", "bigint", 0, false)
	requireColumn(t, tx, "customer_tier_overrides", "expires_at", "timestamp with time zone", 0, false)
	requireColumn(t, tx, "customer_paid_value_refunds", "amount_cny_fen", "bigint", 0, false)
	requireColumn(t, tx, "monthly_entitlement_consumptions", "confirmed_amount_micros", "bigint", 0, false)
	requireColumn(t, tx, "customer_tier_incident_snapshots", "multiplier", "numeric", 0, false)
	requireColumn(t, tx, "customer_tier_incident_snapshots", "policy_snapshot", "jsonb", 0, false)
	requireColumn(t, tx, "customer_tier_evaluations", "policy_snapshot", "jsonb", 0, false)
	requireColumn(t, tx, "compensation_drafts", "proposed_total_cny_fen", "bigint", 0, false)
	requireColumn(t, tx, "compensation_draft_users", "builder_pass_benefit_cny_fen", "bigint", 0, false)
	requireColumn(t, tx, "compensation_draft_users", "evidence_complete", "boolean", 0, false)
	requireColumn(t, tx, "compensation_draft_items", "product_rate_version_id", "bigint", 0, true)
	requireColumn(t, tx, "compensation_group_weight_versions", "benefit_channel", "character varying", 24, false)
	requireColumn(t, tx, "compensation_evidence_snapshots", "retention_until", "timestamp with time zone", 0, false)
	requireColumn(t, tx, "compensation_executions", "amount_cny_fen", "bigint", 0, false)
	requireColumn(t, tx, "compensation_draft_approvals", "approved_preview_hash", "character", 64, false)
	requireColumn(t, tx, "compensation_approval_notices", "body", "text", 0, false)
	requireColumn(t, tx, "compensation_execution_receipts", "payload_hash", "character", 64, false)
	requireColumn(t, tx, "compensation_notices", "approval_notice_id", "bigint", 0, false)
	requireColumn(t, tx, "erroneous_charge_refunds", "amount_micros", "bigint", 0, false)
	requireIndex(t, tx, "customer_tier_current", "idx_customer_tier_current_effective")
	requireIndex(t, tx, "customer_tier_incident_snapshots", "customer_tier_incident_snapshots_incident_id_user_id_key")
	var tierEvaluationEnabled string
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT value FROM settings WHERE key='customer_tier_evaluation_enabled'`).Scan(&tierEvaluationEnabled))
	require.Equal(t, "false", tierEvaluationEnabled)
	var compensationShadowEnabled string
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT value FROM settings WHERE key='compensation_shadow_draft_enabled'`).Scan(&compensationShadowEnabled))
	require.Equal(t, "false", compensationShadowEnabled)
	var compensationExecutionEnabled string
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT value FROM settings WHERE key='compensation_execution_enabled'`).Scan(&compensationExecutionEnabled))
	require.Equal(t, "false", compensationExecutionEnabled)
	var policyWindow, policyGrace int
	var priorityThreshold, strategicThreshold int64
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT rolling_window_days,downgrade_grace_days,priority_threshold_cny_fen,strategic_threshold_cny_fen FROM customer_tier_policy_versions WHERE version=1`).Scan(&policyWindow, &policyGrace, &priorityThreshold, &strategicThreshold))
	require.Equal(t, 90, policyWindow)
	require.Equal(t, 30, policyGrace)
	require.Equal(t, int64(25_000), priorityThreshold)
	require.Equal(t, int64(100_000), strategicThreshold)
	var channelMonitoringAPI int
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM admin_apis WHERE method='GET' AND path IN ('/admin/ops/channel-monitoring','/admin/ops/openai-route-shadow/stats','/admin/ops/openai-route-shadow/health') AND status='active'`).Scan(&channelMonitoringAPI))
	require.Equal(t, 3, channelMonitoringAPI)
	var opsRoleID, opsMenuID int64
	require.NoError(t, tx.QueryRowContext(context.Background(), `INSERT INTO admin_roles(name,description,is_super_admin,status) VALUES('integration_ops_monitor','',FALSE,'active') RETURNING id`).Scan(&opsRoleID))
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT id FROM admin_menus WHERE permission_key='admin:ops'`).Scan(&opsMenuID))
	_, err := tx.ExecContext(context.Background(), `INSERT INTO admin_role_menus(role_id,menu_id) VALUES($1,$2)`, opsRoleID, opsMenuID)
	require.NoError(t, err)
	rbacMigration, err := fs.ReadFile(embeddedmigrations.FS, "203_grant_channel_monitoring_rbac.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(context.Background(), string(rbacMigration))
	require.NoError(t, err)
	var grantedOpsAPIs int
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM admin_role_apis ra JOIN admin_apis a ON a.id=ra.api_id WHERE ra.role_id=$1 AND a.path IN ('/admin/ops/channel-monitoring','/admin/ops/openai-route-shadow/stats','/admin/ops/openai-route-shadow/health')`, opsRoleID).Scan(&grantedOpsAPIs))
	require.Equal(t, 3, grantedOpsAPIs)
	shadowDecisionMigration, err := fs.ReadFile(embeddedmigrations.FS, "204_grant_shadow_decision_read_rbac.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(context.Background(), string(shadowDecisionMigration))
	require.NoError(t, err)
	var grantedShadowDecision int
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM admin_role_apis ra JOIN admin_apis a ON a.id=ra.api_id WHERE ra.role_id=$1 AND a.method='GET' AND a.path='/admin/ops/openai-route-shadow/decisions'`, opsRoleID).Scan(&grantedShadowDecision))
	require.Equal(t, 1, grantedShadowDecision)
	incidentRBACMigration, err := fs.ReadFile(embeddedmigrations.FS, "207_grant_incident_control_rbac.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(context.Background(), string(incidentRBACMigration))
	require.NoError(t, err)
	var grantedIncidentAPIs int
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM admin_role_apis ra JOIN admin_apis a ON a.id=ra.api_id WHERE ra.role_id=$1 AND a.path LIKE '/admin/ops/incidents%'`, opsRoleID).Scan(&grantedIncidentAPIs))
	require.Equal(t, 8, grantedIncidentAPIs)
	tierRBACMigration, err := fs.ReadFile(embeddedmigrations.FS, "209_grant_customer_tier_rbac.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(context.Background(), string(tierRBACMigration))
	require.NoError(t, err)
	var grantedTierAPIs int
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM admin_role_apis ra JOIN admin_apis a ON a.id=ra.api_id WHERE ra.role_id=$1 AND a.path LIKE '/admin/ops/customer-tiers%'`, opsRoleID).Scan(&grantedTierAPIs))
	require.Equal(t, 5, grantedTierAPIs)
	compensationRBACMigration, err := fs.ReadFile(embeddedmigrations.FS, "211_grant_compensation_shadow_rbac.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(context.Background(), string(compensationRBACMigration))
	require.NoError(t, err)
	var grantedCompensationAPIs int
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM admin_role_apis ra JOIN admin_apis a ON a.id=ra.api_id WHERE ra.role_id=$1 AND a.path LIKE '/admin/ops/compensation%'`, opsRoleID).Scan(&grantedCompensationAPIs))
	require.Equal(t, 6, grantedCompensationAPIs)
	executionRBACMigration, err := fs.ReadFile(embeddedmigrations.FS, "215_grant_compensation_execution_rbac.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(context.Background(), string(executionRBACMigration))
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM admin_role_apis ra JOIN admin_apis a ON a.id=ra.api_id WHERE ra.role_id=$1 AND a.path LIKE '/admin/ops/compensation%'`, opsRoleID).Scan(&grantedCompensationAPIs))
	require.Equal(t, 8, grantedCompensationAPIs)
	var nonOwnerMoneyAPIs int
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM admin_role_apis ra JOIN admin_apis a ON a.id=ra.api_id WHERE ra.role_id=$1 AND (a.method,a.path) IN (('POST','/admin/ops/compensation/drafts/:id/approve'),('POST','/admin/ops/compensation/drafts/:id/execute'),('PUT','/admin/ops/compensation/execution-settings'),('POST','/admin/ops/compensation/erroneous-charge-refunds'))`, opsRoleID).Scan(&nonOwnerMoneyAPIs))
	require.Zero(t, nonOwnerMoneyAPIs)
	var catalogProducts, builderPassBindings, legacyBindings, nonHTTPComponents int
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM service_status_products WHERE enabled=TRUE`).Scan(&catalogProducts))
	require.Equal(t, 7, catalogProducts)
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT COUNT(*)
FROM service_status_bindings b
JOIN service_status_components c ON c.id=b.component_id
JOIN service_status_products p ON p.id=c.product_id
WHERE p.code LIKE 'builder-pass-%' AND b.group_name <> ''`).Scan(&builderPassBindings))
	require.Equal(t, 9, builderPassBindings, "GPT, Claude and Grok bindings must survive clean installs without pre-created groups")
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT COUNT(*) FROM service_status_bindings
WHERE group_name ILIKE '%Apex%' OR group_name ILIKE '%Lite%' OR group_name ILIKE '%Ultra%'`).Scan(&legacyBindings))
	require.Zero(t, legacyBindings)
	require.NoError(t, tx.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM service_status_components WHERE access_mode <> 'http'`).Scan(&nonHTTPComponents))
	require.Zero(t, nonHTTPComponents)

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

	// migration 194: topup orders record which gateway (xunhu / easypay)
	// collected the payment.
	requireColumn(t, tx, "topup_orders", "provider", "character varying", 16, false)

	// migration 171: sellable redeem codes preserve actual cash separately
	// from promotional balance credited.
	requireColumn(t, tx, "redeem_codes", "paid_value", "numeric", 0, false)
	requireColumn(t, tx, "finance_transactions", "external_order_no", "text", 0, true)
	requireColumn(t, tx, "finance_transactions", "gross_amount_fen", "bigint", 0, false)
	requireIndex(t, tx, "finance_transactions", "idx_finance_transactions_external_sale_order")
	requireIndex(t, tx, "redeem_codes", "idx_redeem_codes_external_order_lookup")

	// migrations 185-190: native card-shop checkout owns a durable once-per-user
	// order, restricts its inventory, snapshots the selected payment method, and
	// configures the single approved ¥5 -> ¥10 pure-gift newcomer offer. Native
	// checkout remains disabled while manual card redemption is enabled with an
	// atomic lifetime claim. Migration 194 repoints the offer at the EasyPay
	// goods page while keeping the manual redeem fallback enabled.
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

	// migration 195: EasyPay native checkout mints one redeem code per NC-
	// order; the partial unique index backstops the lookup-then-insert mint.
	requireIndex(t, tx, "redeem_codes", "uq_redeem_codes_external_order_no_native_checkout")

	// migration 196: the default success<95 rule exactly mirrors error>5 on
	// the same SLA sample set, so only the error-rate rule should notify.
	var mirroredSuccessRuleEnabled bool
	require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT enabled
FROM ops_alert_rules
WHERE name = '成功率过低'
`).Scan(&mirroredSuccessRuleEnabled))
	require.False(t, mirroredSuccessRuleEnabled)

	var (
		provider         string
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
SELECT provider, provider_goods_key, pay_amount_cny_fen, benefit_amount_cny_fen,
       redeem_value::double precision, redeem_paid_value::double precision,
       redeem_purpose, redeem_sales_status, redeem_validity_days,
       once_per_user, enabled, manual_redeem_enabled
FROM native_checkout_offers
WHERE code = 'newcomer-balance-5-to-10'
`).Scan(
		&provider, &providerGoodsKey, &payFen, &benefitFen, &redeemValue, &paidValue,
		&purpose, &salesStatus, &validityDays, &oncePerUser, &enabled, &manualRedeem,
	))
	require.Equal(t, "ldxp", provider, "migration 194 must leave the newcomer offer on LDXP; the easypay flip is a launch-time ops step")
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

	// migration 212: monthly-card scan checkout uses the owner-approved EasyPay
	// prices while preserving the existing card-shop face values/entitlements.
	type monthlyOfferExpectation struct {
		payFen       int64
		benefitFen   int64
		redeemValue  float64
		groupIDsJSON string
	}
	monthlyOffers := map[string]monthlyOfferExpectation{
		"plus": {payFen: 25500, benefitFen: 25900, redeemValue: 259, groupIDsJSON: "[40, 41]"},
		"pro":  {payFen: 71500, benefitFen: 72900, redeemValue: 729, groupIDsJSON: "[42, 43]"},
		"max":  {payFen: 152500, benefitFen: 154900, redeemValue: 1549, groupIDsJSON: "[44, 45]"},
	}
	for code, expected := range monthlyOffers {
		var (
			actualProvider     string
			actualProductKind  string
			actualPayFen       int64
			actualBenefitFen   int64
			actualRedeemValue  float64
			actualGroupIDsJSON string
			actualValidityDays int
			actualOncePerUser  bool
			actualEnabled      bool
			actualManualRedeem bool
		)
		require.NoError(t, tx.QueryRowContext(context.Background(), `
SELECT provider, product_kind, pay_amount_cny_fen, benefit_amount_cny_fen,
       redeem_value::double precision, redeem_group_ids::text,
       redeem_validity_days, once_per_user, enabled, manual_redeem_enabled
FROM native_checkout_offers
WHERE code = $1
`, code).Scan(
			&actualProvider, &actualProductKind, &actualPayFen, &actualBenefitFen,
			&actualRedeemValue, &actualGroupIDsJSON, &actualValidityDays,
			&actualOncePerUser, &actualEnabled, &actualManualRedeem,
		))
		require.Equal(t, "easypay", actualProvider, code)
		require.Equal(t, "subscription", actualProductKind, code)
		require.Equal(t, expected.payFen, actualPayFen, code)
		require.Equal(t, expected.benefitFen, actualBenefitFen, code)
		require.Equal(t, expected.redeemValue, actualRedeemValue, code)
		require.JSONEq(t, expected.groupIDsJSON, actualGroupIDsJSON, code)
		require.Equal(t, 31, actualValidityDays, code)
		require.False(t, actualOncePerUser, code)
		require.True(t, actualEnabled, code)
		require.False(t, actualManualRedeem, code)
	}

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

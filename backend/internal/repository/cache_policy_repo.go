package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type cachePolicyRepository struct {
	db *sql.DB
}

func NewCachePolicyRepository(db *sql.DB) service.CachePolicyRepository {
	return &cachePolicyRepository{db: db}
}

func (r *cachePolicyRepository) CreateDecision(ctx context.Context, decision *service.CachePolicyDecision) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil cache policy repository")
	}
	if decision == nil {
		return fmt.Errorf("nil cache policy decision")
	}
	createdAt := decision.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO cache_policy_decisions (
  request_id,
  client_request_id,
  user_id,
  api_key_id,
  group_id,
  account_id,
  client_type,
  model,
  policy_mode,
  policy_version,
  actual_ttl,
  shadow_ttl,
  decision_reason,
  cache_control_paths_count,
  normalized,
  downgraded,
  retried,
  duration_ms,
  first_token_ms,
  cache_creation_5m_tokens,
  cache_creation_1h_tokens,
  cache_read_tokens,
  cost,
  created_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24
)`,
		decision.RequestID,
		decision.ClientRequestID,
		nullableInt64(decision.UserID),
		nullableInt64(decision.APIKeyID),
		nullableInt64(decision.GroupID),
		nullableInt64(decision.AccountID),
		decision.ClientType,
		decision.Model,
		decision.PolicyMode,
		decision.PolicyVersion,
		decision.ActualTTL,
		decision.ShadowTTL,
		decision.DecisionReason,
		decision.CacheControlPathsCount,
		decision.Normalized,
		decision.Downgraded,
		decision.Retried,
		nullableInt(decision.DurationMs),
		nullableInt(decision.FirstTokenMs),
		decision.CacheCreation5mTokens,
		decision.CacheCreation1hTokens,
		decision.CacheReadTokens,
		decision.Cost,
		createdAt,
	)
	return err
}

func (r *cachePolicyRepository) UpsertDailyReport(ctx context.Context, report *service.CachePolicyDailyReport) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil cache policy repository")
	}
	if report == nil {
		return fmt.Errorf("nil cache policy daily report")
	}
	reportDate := report.ReportDate
	if reportDate.IsZero() {
		reportDate = time.Now()
	}
	createdAt := report.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	updatedAt := report.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now()
	}

	_, err := r.db.ExecContext(ctx, `
INSERT INTO cache_policy_daily_reports (
  report_date,
  phase,
  policy_version,
  client_profiles,
  group_performance,
  recommended_thresholds,
  auto_action,
  auto_action_reason,
  risk_metrics,
  ttl_order_400_count,
  adaptive_enabled_ratio,
  cost_delta_pct,
  created_at,
  updated_at
) VALUES (
  $1,$2,$3,$4::jsonb,$5::jsonb,$6::jsonb,$7,$8,$9::jsonb,$10,$11,$12,$13,$14
)
ON CONFLICT (report_date) DO UPDATE SET
  phase = EXCLUDED.phase,
  policy_version = EXCLUDED.policy_version,
  client_profiles = EXCLUDED.client_profiles,
  group_performance = EXCLUDED.group_performance,
  recommended_thresholds = EXCLUDED.recommended_thresholds,
  auto_action = EXCLUDED.auto_action,
  auto_action_reason = EXCLUDED.auto_action_reason,
  risk_metrics = EXCLUDED.risk_metrics,
  ttl_order_400_count = EXCLUDED.ttl_order_400_count,
  adaptive_enabled_ratio = EXCLUDED.adaptive_enabled_ratio,
  cost_delta_pct = EXCLUDED.cost_delta_pct,
  updated_at = EXCLUDED.updated_at`,
		reportDate,
		report.Phase,
		report.PolicyVersion,
		nonEmptyJSON(report.ClientProfilesJSON),
		nonEmptyJSON(report.GroupPerformanceJSON),
		nonEmptyJSON(report.RecommendedJSON),
		report.AutoAction,
		report.AutoActionReason,
		nonEmptyJSON(report.RiskMetricsJSON),
		report.TTLOrder400Count,
		report.AdaptiveEnabledRatio,
		report.CostDeltaPct,
		createdAt,
		updatedAt,
	)
	return err
}

func nullableInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
}

func nullableInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func nonEmptyJSON(v string) string {
	if v == "" {
		return "{}"
	}
	return v
}

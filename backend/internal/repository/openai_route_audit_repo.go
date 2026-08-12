package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type openAIRouteDecisionRepository struct {
	db *sql.DB
}

func NewOpenAIRouteDecisionRepository(db *sql.DB) service.OpenAIRouteDecisionRepository {
	return &openAIRouteDecisionRepository{db: db}
}

func (r *openAIRouteDecisionRepository) CheckOpenAIRouteShadowDecisionStorage(ctx context.Context) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil OpenAI route decision repository")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	probeID := fmt.Sprintf("probe:%d", time.Now().UnixNano())
	if _, err := tx.ExecContext(ctx, `
INSERT INTO openai_route_shadow_decisions (
  decision_id, request_id, client_request_id, attempt, group_id, model, request_class,
  policy_mode, policy_version, reason, evaluated, snapshot
) VALUES ($1, '', '', 1, 1, '__storage_probe__', 'text', 'shadow', 0, 'storage_probe', FALSE, '{}'::jsonb)
`, probeID); err != nil {
		return err
	}
	return tx.Rollback()
}

func (r *openAIRouteDecisionRepository) CreateOpenAIRouteShadowDecision(
	ctx context.Context,
	record *service.OpenAIRouteShadowDecisionRecord,
) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("nil OpenAI route decision repository")
	}
	if record == nil || record.Snapshot == nil {
		return fmt.Errorf("nil OpenAI route shadow decision")
	}
	snapshot, err := json.Marshal(record.Snapshot)
	if err != nil {
		return fmt.Errorf("marshal OpenAI route shadow snapshot: %w", err)
	}
	createdAt := record.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO openai_route_shadow_decisions (
  decision_id, request_id, client_request_id, attempt, group_id, model, request_class,
  policy_mode, policy_version, reason, evaluated, evaluation_duration_us,
  legacy_selected_account_id, adaptive_selected_account_id, adaptive_selected_rate,
  candidate_count, excluded_count, diverged, emergency, snapshot, created_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20::jsonb,$21
)`,
		record.DecisionID,
		record.RequestID,
		record.ClientRequestID,
		record.Attempt,
		record.GroupID,
		record.Model,
		string(record.RequestClass),
		string(record.PolicyMode),
		record.PolicyVersion,
		record.Reason,
		record.Evaluated,
		record.EvaluationDurationMicros,
		nullablePositiveInt64(record.LegacySelectedAccountID),
		nullablePositiveInt64(record.AdaptiveSelectedAccountID),
		nullableAdaptiveRate(record.AdaptiveSelectedAccountID, record.AdaptiveSelectedRate),
		record.CandidateCount,
		record.ExcludedCount,
		record.Diverged,
		record.Emergency,
		string(snapshot),
		createdAt.UTC(),
	)
	return err
}

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

func nullableAdaptiveRate(accountID int64, rate float64) any {
	if accountID <= 0 {
		return nil
	}
	return rate
}

func (r *openAIRouteDecisionRepository) ListOpenAIRouteShadowDecisions(
	ctx context.Context,
	filter *service.OpenAIRouteShadowDecisionFilter,
) (*service.OpenAIRouteShadowDecisionList, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil OpenAI route decision repository")
	}
	filter = normalizeOpenAIRouteShadowFilter(filter)
	where, args := buildOpenAIRouteShadowWhere(filter, "d")
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM openai_route_shadow_decisions d "+where, args...).Scan(&total); err != nil {
		return nil, err
	}

	offset := (filter.Page - 1) * filter.PageSize
	queryArgs := append(append([]any(nil), args...), filter.PageSize, offset)
	query := `
SELECT
  d.id, d.decision_id, d.request_id, d.client_request_id, d.attempt,
  d.group_id, d.model, d.request_class, d.policy_mode, d.policy_version, d.reason,
  d.evaluated, d.evaluation_duration_us,
  d.legacy_selected_account_id, d.adaptive_selected_account_id,
  d.adaptive_selected_rate, d.candidate_count, d.excluded_count,
  d.diverged, d.emergency, d.snapshot::text, d.created_at
FROM openai_route_shadow_decisions d
` + where + `
ORDER BY d.created_at DESC, d.id DESC
LIMIT $` + fmt.Sprint(len(args)+1) + ` OFFSET $` + fmt.Sprint(len(args)+2)

	rows, err := r.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	decisions := make([]*service.OpenAIRouteShadowDecisionRecord, 0, filter.PageSize)
	for rows.Next() {
		item := &service.OpenAIRouteShadowDecisionRecord{}
		var legacyID sql.NullInt64
		var adaptiveID sql.NullInt64
		var adaptiveRate sql.NullFloat64
		var snapshotRaw string
		if err := rows.Scan(
			&item.ID, &item.DecisionID, &item.RequestID, &item.ClientRequestID, &item.Attempt,
			&item.GroupID, &item.Model, &item.RequestClass, &item.PolicyMode, &item.PolicyVersion, &item.Reason,
			&item.Evaluated, &item.EvaluationDurationMicros,
			&legacyID, &adaptiveID, &adaptiveRate, &item.CandidateCount, &item.ExcludedCount,
			&item.Diverged, &item.Emergency, &snapshotRaw, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		if legacyID.Valid {
			item.LegacySelectedAccountID = legacyID.Int64
		}
		if adaptiveID.Valid {
			item.AdaptiveSelectedAccountID = adaptiveID.Int64
		}
		if adaptiveRate.Valid {
			item.AdaptiveSelectedRate = adaptiveRate.Float64
		}
		item.Snapshot = &service.OpenAIRouteShadowAuditSnapshot{}
		if err := json.Unmarshal([]byte(snapshotRaw), item.Snapshot); err != nil {
			return nil, fmt.Errorf("decode OpenAI route shadow snapshot %d: %w", item.ID, err)
		}
		decisions = append(decisions, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &service.OpenAIRouteShadowDecisionList{
		Decisions: decisions,
		Total:     total,
		Page:      filter.Page,
		PageSize:  filter.PageSize,
	}, nil
}

func (r *openAIRouteDecisionRepository) GetOpenAIRouteShadowDecisionStats(
	ctx context.Context,
	filter *service.OpenAIRouteShadowDecisionFilter,
) (*service.OpenAIRouteShadowDecisionStats, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("nil OpenAI route decision repository")
	}
	filter = normalizeOpenAIRouteShadowFilter(filter)
	where, args := buildOpenAIRouteShadowWhere(filter, "d")
	stats := &service.OpenAIRouteShadowDecisionStats{}
	aggregate := `
WITH filtered AS (
  SELECT d.* FROM openai_route_shadow_decisions d ` + where + `
), linked AS (
  SELECT d.*,
         u.id AS usage_id,
         u.first_token_ms AS legacy_first_token_ms,
         e.id AS error_id
  FROM filtered d
  LEFT JOIN LATERAL (
    SELECT ul.id, ul.first_token_ms
    FROM usage_logs ul
    WHERE (
        (d.client_request_id <> '' AND ul.request_id = 'client:' || d.client_request_id)
        OR
        (d.client_request_id = '' AND d.request_id <> '' AND ul.request_id = 'local:' || d.request_id)
      )
      AND ul.group_id = d.group_id
      AND ul.account_id = d.legacy_selected_account_id
    ORDER BY ul.created_at ASC, ul.id ASC
    LIMIT 1
  ) u ON TRUE
  LEFT JOIN LATERAL (
    SELECT oe.id
    FROM ops_error_logs oe
    WHERE oe.account_id = d.legacy_selected_account_id
      AND (
        (d.client_request_id <> '' AND oe.client_request_id = d.client_request_id)
        OR (d.request_id <> '' AND oe.request_id = d.request_id)
      )
    ORDER BY oe.created_at ASC, oe.id ASC
    LIMIT 1
  ) e ON TRUE
)
SELECT
  COUNT(*)::bigint,
  COUNT(*) FILTER (WHERE evaluated)::bigint,
  COUNT(*) FILTER (WHERE NOT evaluated)::bigint,
  COUNT(*) FILTER (WHERE diverged)::bigint,
  COUNT(*) FILTER (WHERE emergency)::bigint,
  COUNT(*) FILTER (WHERE usage_id IS NOT NULL AND error_id IS NULL)::bigint,
  COUNT(*) FILTER (WHERE usage_id IS NULL AND error_id IS NOT NULL)::bigint,
  COUNT(*) FILTER (WHERE usage_id IS NOT NULL AND error_id IS NOT NULL)::bigint,
  COUNT(*) FILTER (WHERE usage_id IS NULL AND error_id IS NULL)::bigint,
  COUNT(*) FILTER (WHERE evaluated AND usage_id IS NOT NULL AND error_id IS NULL)::bigint,
  COUNT(*) FILTER (WHERE evaluated AND usage_id IS NULL AND error_id IS NOT NULL)::bigint,
  COUNT(*) FILTER (WHERE evaluated AND usage_id IS NOT NULL AND error_id IS NOT NULL)::bigint,
  COUNT(*) FILTER (WHERE evaluated AND usage_id IS NULL AND error_id IS NULL)::bigint,
  COUNT(DISTINCT snapshot->'policy')::bigint,
  CASE WHEN COUNT(DISTINCT snapshot->'policy') = 1
       THEN COALESCE(MIN((snapshot->'policy'->>'max_account_share')::float8), 0)
       ELSE 0 END::float8,
  CASE WHEN COUNT(DISTINCT snapshot->'policy') = 1
       THEN COALESCE(MIN((snapshot->'policy'->>'max_provider_share')::float8), 0)
       ELSE 0 END::float8,
  MIN(created_at),
  MAX(created_at),
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY evaluation_duration_us), 0)::float8,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY evaluation_duration_us), 0)::float8,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY legacy_first_token_ms) FILTER (WHERE legacy_first_token_ms IS NOT NULL), 0)::float8,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY legacy_first_token_ms) FILTER (WHERE legacy_first_token_ms IS NOT NULL), 0)::float8
FROM linked`
	var firstDecisionAt sql.NullTime
	var lastDecisionAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, aggregate, args...).Scan(
		&stats.Total,
		&stats.Evaluated,
		&stats.NotEvaluated,
		&stats.Diverged,
		&stats.Emergency,
		&stats.LinkedSuccessfulUsage,
		&stats.LinkedLegacyFailure,
		&stats.AmbiguousOutcome,
		&stats.UnlinkedOutcome,
		&stats.EvaluatedLinkedSuccessfulUsage,
		&stats.EvaluatedLinkedLegacyFailure,
		&stats.EvaluatedAmbiguousOutcome,
		&stats.EvaluatedUnlinkedOutcome,
		&stats.PolicySnapshotVariants,
		&stats.PolicyMaxAccountShare,
		&stats.PolicyMaxProviderShare,
		&firstDecisionAt,
		&lastDecisionAt,
		&stats.EvaluationDurationP50US,
		&stats.EvaluationDurationP95US,
		&stats.LegacyTTFTP50Ms,
		&stats.LegacyTTFTP95Ms,
	); err != nil {
		return nil, err
	}
	if firstDecisionAt.Valid {
		stats.FirstDecisionAt = firstDecisionAt.Time.UTC()
	}
	if lastDecisionAt.Valid {
		stats.LastDecisionAt = lastDecisionAt.Time.UTC()
	}

	selectedQuery := `
WITH filtered AS (
  SELECT d.* FROM openai_route_shadow_decisions d ` + where + `
)
SELECT
  adaptive_selected_account_id,
  COALESCE(adaptive_selected_rate, 0)::float8,
  COUNT(*)::bigint
FROM filtered
WHERE evaluated AND adaptive_selected_account_id IS NOT NULL
GROUP BY adaptive_selected_account_id, adaptive_selected_rate
ORDER BY COUNT(*) DESC, adaptive_selected_account_id ASC`
	rows, err := r.db.QueryContext(ctx, selectedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		item := service.OpenAIRouteShadowSelectedAccountStats{}
		if err := rows.Scan(&item.AccountID, &item.RateMultiplier, &item.SelectedCount); err != nil {
			return nil, err
		}
		if stats.Evaluated > 0 {
			item.SelectedPercent = float64(item.SelectedCount) * 100 / float64(stats.Evaluated)
		}
		stats.SelectedAccounts = append(stats.SelectedAccounts, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	providerQuery := `
WITH filtered AS (
  SELECT d.* FROM openai_route_shadow_decisions d ` + where + `
), selected AS (
  SELECT
    COALESCE(
      (
        SELECT NULLIF(candidate->>'failure_domain', '')
        FROM jsonb_array_elements(d.snapshot->'candidates') candidate
        WHERE candidate->>'account_id' = d.adaptive_selected_account_id::text
        LIMIT 1
      ),
      'account:' || d.adaptive_selected_account_id::text
    ) AS provider_key
  FROM filtered d
  WHERE d.evaluated AND d.adaptive_selected_account_id IS NOT NULL
)
SELECT provider_key, COUNT(*)::bigint
FROM selected
GROUP BY provider_key
ORDER BY COUNT(*) DESC, provider_key ASC`
	providerRows, err := r.db.QueryContext(ctx, providerQuery, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = providerRows.Close() }()
	for providerRows.Next() {
		item := service.OpenAIRouteShadowSelectedProviderStats{}
		if err := providerRows.Scan(&item.ProviderKey, &item.SelectedCount); err != nil {
			return nil, err
		}
		if stats.Evaluated > 0 {
			item.SelectedPercent = float64(item.SelectedCount) * 100 / float64(stats.Evaluated)
		}
		stats.SelectedProviders = append(stats.SelectedProviders, item)
	}
	if err := providerRows.Err(); err != nil {
		return nil, err
	}
	return stats, nil
}

func normalizeOpenAIRouteShadowFilter(filter *service.OpenAIRouteShadowDecisionFilter) *service.OpenAIRouteShadowDecisionFilter {
	if filter == nil {
		filter = &service.OpenAIRouteShadowDecisionFilter{}
	}
	copyFilter := *filter
	if copyFilter.Page <= 0 {
		copyFilter.Page = 1
	}
	if copyFilter.PageSize <= 0 {
		copyFilter.PageSize = 50
	}
	if copyFilter.PageSize > 200 {
		copyFilter.PageSize = 200
	}
	return &copyFilter
}

func buildOpenAIRouteShadowWhere(filter *service.OpenAIRouteShadowDecisionFilter, alias string) (string, []any) {
	if filter == nil {
		return "", nil
	}
	prefix := strings.TrimSpace(alias)
	if prefix != "" {
		prefix += "."
	}
	conditions := make([]string, 0, 13)
	args := make([]any, 0, 13)
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}
	if filter.StartTime != nil && !filter.StartTime.IsZero() {
		add(prefix+"created_at >= $%d", filter.StartTime.UTC())
	}
	if filter.EndTime != nil && !filter.EndTime.IsZero() {
		add(prefix+"created_at < $%d", filter.EndTime.UTC())
	}
	if filter.GroupID != nil && *filter.GroupID > 0 {
		add(prefix+"group_id = $%d", *filter.GroupID)
	}
	if value := strings.TrimSpace(filter.Model); value != "" {
		add(prefix+"model = $%d", value)
	}
	if filter.RequestClass.Valid() {
		add(prefix+"request_class = $%d", string(filter.RequestClass))
	}
	if value := strings.TrimSpace(string(filter.PolicyMode)); value != "" {
		add(prefix+"policy_mode = $%d", value)
	}
	if filter.PolicyVersion != nil {
		add(prefix+"policy_version = $%d", *filter.PolicyVersion)
	}
	if value := strings.TrimSpace(filter.Reason); value != "" {
		add(prefix+"reason = $%d", value)
	}
	if value := strings.TrimSpace(filter.RequestID); value != "" {
		add(prefix+"request_id = $%d", value)
	}
	if value := strings.TrimSpace(filter.ClientRequestID); value != "" {
		add(prefix+"client_request_id = $%d", value)
	}
	if filter.Evaluated != nil {
		add(prefix+"evaluated = $%d", *filter.Evaluated)
	}
	if filter.Diverged != nil {
		add(prefix+"diverged = $%d", *filter.Diverged)
	}
	if filter.Emergency != nil {
		add(prefix+"emergency = $%d", *filter.Emergency)
	}
	if len(conditions) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(conditions, " AND "), args
}

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
	db       *sql.DB
	evidence *openAIRouteEvidenceEpochRepository
}

func NewOpenAIRouteDecisionRepository(db *sql.DB) service.OpenAIRouteDecisionRepository {
	return &openAIRouteDecisionRepository{db: db, evidence: newOpenAIRouteEvidenceEpochRepository(db)}
}

func (r *openAIRouteDecisionRepository) BeginOpenAIRouteEvidenceEpoch(ctx context.Context, epoch service.OpenAIRouteEvidenceEpoch) error {
	return r.evidence.BeginOpenAIRouteEvidenceEpoch(ctx, epoch)
}

func (r *openAIRouteDecisionRepository) CheckpointOpenAIRouteEvidenceEpoch(ctx context.Context, epoch service.OpenAIRouteEvidenceEpoch) error {
	return r.evidence.CheckpointOpenAIRouteEvidenceEpoch(ctx, epoch)
}

func (r *openAIRouteDecisionRepository) ListOpenAIRouteEvidenceEpochs(ctx context.Context, start, end time.Time) ([]service.OpenAIRouteEvidenceEpoch, error) {
	return r.evidence.ListOpenAIRouteEvidenceEpochs(ctx, start, end)
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
	experimentID := strings.TrimSpace(record.ExperimentID)
	if experimentID == "" {
		experimentID = strings.TrimSpace(record.ActivationID)
	}
	variantID := strings.TrimSpace(record.VariantID)
	if variantID == "" {
		variantID = "default"
	}
	_, err = r.db.ExecContext(ctx, `
INSERT INTO openai_route_shadow_decisions (
  decision_id, request_id, client_request_id, attempt, group_id, model, request_class,
  policy_mode, policy_version, activation_id, experiment_id, variant_id, treatment_fingerprint,
  shadow_started_at, reason, evaluated, evaluation_duration_us,
  legacy_selected_account_id, adaptive_selected_account_id, adaptive_selected_rate,
  candidate_count, excluded_count, diverged, emergency, snapshot, created_at
) VALUES (
  $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,md5(COALESCE(($24::jsonb->'policy')::text, '{}')),
  $13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24::jsonb,$25
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
		record.ActivationID,
		experimentID,
		variantID,
		nullableOpenAIRouteTimestamp(record.ShadowStartedAt),
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

func nullableOpenAIRouteTimestamp(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC()
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
  d.group_id, d.model, d.request_class, d.policy_mode, d.policy_version,
  d.activation_id, d.experiment_id, d.variant_id, d.treatment_fingerprint,
  d.shadow_started_at, d.reason,
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
		var shadowStartedAt sql.NullTime
		var snapshotRaw string
		if err := rows.Scan(
			&item.ID, &item.DecisionID, &item.RequestID, &item.ClientRequestID, &item.Attempt,
			&item.GroupID, &item.Model, &item.RequestClass, &item.PolicyMode, &item.PolicyVersion,
			&item.ActivationID, &item.ExperimentID, &item.VariantID, &item.TreatmentFingerprint,
			&shadowStartedAt, &item.Reason,
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
		if shadowStartedAt.Valid {
			item.ShadowStartedAt = shadowStartedAt.Time.UTC()
		}
		item.Snapshot = &service.OpenAIRouteShadowAuditSnapshot{}
		if err := json.Unmarshal([]byte(snapshotRaw), item.Snapshot); err != nil {
			return nil, fmt.Errorf("decode OpenAI route shadow snapshot %d: %w", item.ID, err)
		}
		item.AdaptiveSelectedEndpointHash = item.Snapshot.AdaptiveSelectedEndpointHash
		item.AdaptiveSelectedRouteFingerprint = item.Snapshot.AdaptiveSelectedRouteFingerprint
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
	coverageExpression := "0::bigint"
	aggregateArgs := append([]any(nil), args...)
	if filter.StartTime != nil && !filter.StartTime.IsZero() {
		aggregateArgs = append(aggregateArgs, filter.StartTime.UTC())
		coverageExpression = fmt.Sprintf(`COUNT(DISTINCT FLOOR(EXTRACT(EPOCH FROM (created_at - $%d::timestamptz)) / 3600))
    FILTER (WHERE evaluated)::bigint`, len(aggregateArgs))
	}
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
  COUNT(*) FILTER (WHERE NOT evaluated AND reason = 'no_shadow_candidate')::bigint,
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
  COUNT(DISTINCT NULLIF(activation_id, ''))::bigint,
  COUNT(DISTINCT shadow_started_at)::bigint,
  COUNT(DISTINCT NULLIF(experiment_id, ''))::bigint,
  COUNT(DISTINCT NULLIF(variant_id, ''))::bigint,
  COUNT(DISTINCT NULLIF(treatment_fingerprint, ''))::bigint,
  CASE WHEN COUNT(DISTINCT shadow_started_at) = 1
       THEN MIN(shadow_started_at)
       ELSE NULL END,
  CASE WHEN COUNT(DISTINCT snapshot->'policy') = 1
       THEN COALESCE(MIN((snapshot->'policy'->>'max_account_share')::float8), 0)
       ELSE 0 END::float8,
  CASE WHEN COUNT(DISTINCT snapshot->'policy') = 1
       THEN COALESCE(MIN((snapshot->'policy'->>'max_provider_share')::float8), 0)
       ELSE 0 END::float8,
  ` + coverageExpression + `,
  COUNT(DISTINCT (created_at AT TIME ZONE 'Asia/Shanghai')::date)
    FILTER (WHERE evaluated)::bigint,
  COUNT(DISTINCT FLOOR(EXTRACT(HOUR FROM created_at AT TIME ZONE 'Asia/Shanghai') / 6))
    FILTER (WHERE evaluated)::bigint,
  MIN(created_at) FILTER (WHERE evaluated),
  MAX(created_at) FILTER (WHERE evaluated),
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY evaluation_duration_us), 0)::float8,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY evaluation_duration_us), 0)::float8,
  COALESCE(percentile_cont(0.50) WITHIN GROUP (ORDER BY legacy_first_token_ms) FILTER (WHERE legacy_first_token_ms IS NOT NULL), 0)::float8,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY legacy_first_token_ms) FILTER (WHERE legacy_first_token_ms IS NOT NULL), 0)::float8
FROM linked`
	var firstDecisionAt sql.NullTime
	var lastDecisionAt sql.NullTime
	var shadowStartedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, aggregate, aggregateArgs...).Scan(
		&stats.Total,
		&stats.Evaluated,
		&stats.NotEvaluated,
		&stats.NoCandidateAbstentions,
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
		&stats.ActivationIDVariants,
		&stats.ShadowStartedAtVariants,
		&stats.ExperimentIDVariants,
		&stats.VariantIDVariants,
		&stats.TreatmentFingerprintVariants,
		&shadowStartedAt,
		&stats.PolicyMaxAccountShare,
		&stats.PolicyMaxProviderShare,
		&stats.CoveredHourBuckets,
		&stats.CoveredBeijingDates,
		&stats.CoveredBeijingDayparts,
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
	if shadowStartedAt.Valid {
		stats.ShadowStartedAt = shadowStartedAt.Time.UTC()
	}

	selectedQuery := `
WITH filtered AS (
  SELECT d.* FROM openai_route_shadow_decisions d ` + where + `
)
SELECT
  adaptive_selected_account_id,
  COALESCE(AVG(adaptive_selected_rate), 0)::float8,
  COUNT(*)::bigint
FROM filtered
WHERE evaluated AND adaptive_selected_account_id IS NOT NULL
GROUP BY adaptive_selected_account_id
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

	routeQuery := `
WITH filtered AS (
  SELECT d.* FROM openai_route_shadow_decisions d ` + where + `
), selected AS (
  SELECT
    (candidate->>'account_id')::bigint AS account_id,
    candidate->>'endpoint_hash' AS endpoint_hash,
    COALESCE(NULLIF(candidate->>'failure_domain', ''), 'account:' || (candidate->>'account_id')) AS failure_domain,
    COALESCE((candidate->>'route_variant')::boolean, FALSE) AS route_variant
  FROM filtered d
  CROSS JOIN LATERAL jsonb_array_elements(d.snapshot->'candidates') candidate
  WHERE d.evaluated AND COALESCE((candidate->>'selected')::boolean, FALSE)
)
SELECT account_id, endpoint_hash, failure_domain, route_variant, COUNT(*)::bigint
FROM selected
GROUP BY account_id, endpoint_hash, failure_domain, route_variant
ORDER BY COUNT(*) DESC, account_id ASC, endpoint_hash ASC`
	routeRows, err := r.db.QueryContext(ctx, routeQuery, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = routeRows.Close() }()
	for routeRows.Next() {
		item := service.OpenAIRouteShadowSelectedRouteStats{}
		if err := routeRows.Scan(
			&item.AccountID,
			&item.EndpointHash,
			&item.FailureDomain,
			&item.RouteVariant,
			&item.SelectedCount,
		); err != nil {
			return nil, err
		}
		if stats.Evaluated > 0 {
			item.SelectedPercent = float64(item.SelectedCount) * 100 / float64(stats.Evaluated)
		}
		stats.SelectedRoutes = append(stats.SelectedRoutes, item)
	}
	if err := routeRows.Err(); err != nil {
		return nil, err
	}

	providerQuery := `
WITH filtered AS (
  SELECT d.* FROM openai_route_shadow_decisions d ` + where + `
), selected AS (
  SELECT
    COALESCE(NULLIF(candidate->>'failure_domain', ''), 'account:' || (candidate->>'account_id')) AS provider_key
  FROM filtered d
  CROSS JOIN LATERAL jsonb_array_elements(d.snapshot->'candidates') candidate
  WHERE d.evaluated AND COALESCE((candidate->>'selected')::boolean, FALSE)
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
	conditions := make([]string, 0, 17)
	args := make([]any, 0, 17)
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
	if value := strings.TrimSpace(filter.ActivationID); value != "" {
		add(prefix+"activation_id = $%d", value)
	}
	if value := strings.TrimSpace(filter.ExperimentID); value != "" {
		add(prefix+"experiment_id = $%d", value)
	}
	if value := strings.TrimSpace(filter.VariantID); value != "" {
		add(prefix+"variant_id = $%d", value)
	}
	if value := strings.TrimSpace(filter.TreatmentFingerprint); value != "" {
		add(prefix+"treatment_fingerprint = $%d", value)
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

package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
)

const openAIRouteObservationCheckpointQueryTimeout = 15 * time.Millisecond

// openAIRouteObservationStore mirrors each passive observation into Redis and
// an hourly PostgreSQL checkpoint. A write is complete only when both stores
// acknowledge it; collector completeness therefore fails closed on either
// side. Recent traffic remains Redis-backed while PostgreSQL is authoritative
// for the longer evidence windows.
type openAIRouteObservationStore struct {
	cache      service.OpenAIRouteObservationStore
	checkpoint *openAIRouteObservationCheckpointRepository
	benchmark  *openAIRouteBenchmarkObservationRepository
}

func NewOpenAIRouteObservationStore(rdbStore service.OpenAIRouteObservationStore, db *sql.DB) service.OpenAIRouteObservationStore {
	return &openAIRouteObservationStore{
		cache:      rdbStore,
		checkpoint: NewOpenAIRouteObservationCheckpointRepository(db),
		benchmark:  NewOpenAIRouteBenchmarkObservationRepository(db),
	}
}

func (s *openAIRouteObservationStore) GetBenchmarkBatch(
	ctx context.Context,
	keys []service.OpenAIRouteKey,
	now time.Time,
) (map[string]service.OpenAIRouteBenchmarkObservationProfile, error) {
	if s == nil || s.benchmark == nil {
		return nil, service.ErrOpenAIRouteNoCandidate
	}
	return s.benchmark.GetBenchmarkBatch(ctx, keys, now)
}

func (s *openAIRouteObservationStore) Check(ctx context.Context) error {
	if s == nil || s.cache == nil || s.checkpoint == nil {
		return service.ErrOpenAIRouteNoCandidate
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return errors.Join(s.cache.Check(ctx), s.checkpoint.Check(ctx))
}

func (s *openAIRouteObservationStore) Record(ctx context.Context, observation service.OpenAIRouteObservation) error {
	if s == nil || s.cache == nil || s.checkpoint == nil {
		return service.ErrOpenAIRouteNoCandidate
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now().UTC()
	} else {
		observation.ObservedAt = observation.ObservedAt.UTC()
	}
	// Both calls are deliberately attempted. A transient Redis failure must not
	// erase the durable checkpoint, and a PostgreSQL failure must be visible to
	// the readiness/completeness gate rather than silently trusting Redis TTLs.
	cacheErr := s.cache.Record(ctx, observation)
	checkpointErr := s.checkpoint.Record(ctx, observation)
	return errors.Join(cacheErr, checkpointErr)
}

func (s *openAIRouteObservationStore) RecordCost(ctx context.Context, observation service.OpenAIRouteActualCostObservation) error {
	if s == nil || s.cache == nil || s.checkpoint == nil {
		return service.ErrOpenAIRouteNoCandidate
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now().UTC()
	} else {
		observation.ObservedAt = observation.ObservedAt.UTC()
	}
	cacheErr := s.cache.RecordCost(ctx, observation)
	checkpointErr := s.checkpoint.RecordCost(ctx, observation)
	return errors.Join(cacheErr, checkpointErr)
}

func (s *openAIRouteObservationStore) GetBatch(
	ctx context.Context,
	keys []service.OpenAIRouteKey,
	now time.Time,
) (map[string]service.OpenAIRouteObservationProfile, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(keys) == 0 {
		return map[string]service.OpenAIRouteObservationProfile{}, nil
	}
	if s == nil || s.cache == nil || s.checkpoint == nil {
		return nil, service.ErrOpenAIRouteNoCandidate
	}
	checkpointCtx, cancel := context.WithTimeout(ctx, openAIRouteObservationCheckpointQueryTimeout)
	checkpointProfiles, checkpointErr := s.checkpoint.GetBatch(checkpointCtx, keys, now)
	cancel()
	if checkpointErr != nil {
		return nil, checkpointErr
	}
	profiles, err := s.cache.GetBatch(ctx, keys, now)
	if err != nil {
		// PostgreSQL owns the durable global/seasonal evidence. If Redis is
		// unavailable, continue with an empty recent window rather than erasing
		// all long-term learning; collector readiness still reports the Redis
		// write/check failure and therefore blocks promotion.
		profiles = make(map[string]service.OpenAIRouteObservationProfile, len(checkpointProfiles))
	}
	for fingerprint, checkpointProfile := range checkpointProfiles {
		profile, ok := profiles[fingerprint]
		if !ok {
			profile = newOpenAIRouteObservationProfile()
		}
		// Do not merge Redis global/seasonal values: every observation is
		// dual-written, so doing so would double count. Redis is retained only
		// for the sub-hour recent signal.
		profile.Global = checkpointProfile.Global
		profile.HourOfWeek = checkpointProfile.HourOfWeek
		profiles[fingerprint] = profile
	}
	return profiles, nil
}

type openAIRouteObservationCheckpointRepository struct {
	db *sql.DB
}

func NewOpenAIRouteObservationCheckpointRepository(db *sql.DB) *openAIRouteObservationCheckpointRepository {
	return &openAIRouteObservationCheckpointRepository{db: db}
}

func (r *openAIRouteObservationCheckpointRepository) Check(ctx context.Context) error {
	if r == nil || r.db == nil {
		return service.ErrOpenAIRouteNoCandidate
	}
	var tableExists bool
	if err := r.db.QueryRowContext(ctx, `
SELECT to_regclass('public.openai_route_observation_hourly') IS NOT NULL
`).Scan(&tableExists); err != nil {
		return err
	}
	if !tableExists {
		return errors.New("openai_route_observation_hourly table is missing")
	}
	return nil
}

func (r *openAIRouteObservationCheckpointRepository) Record(ctx context.Context, observation service.OpenAIRouteObservation) error {
	if r == nil || r.db == nil {
		return service.ErrOpenAIRouteNoCandidate
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now().UTC()
	} else {
		observation.ObservedAt = observation.ObservedAt.UTC()
	}
	metrics, err := openAIRouteObservationMetrics(observation)
	if err != nil {
		return err
	}
	baseCost := 0.0
	accountCost := 0.0
	if observation.ActualCostAuthoritative {
		baseCost = observation.ActualBaseCostUSD
		accountCost = observation.ActualAccountCostUSD
	}
	return r.upsert(ctx, observation.Key, observation.ObservedAt, metrics, baseCost, accountCost)
}

func (r *openAIRouteObservationCheckpointRepository) RecordCost(ctx context.Context, observation service.OpenAIRouteActualCostObservation) error {
	if r == nil || r.db == nil {
		return service.ErrOpenAIRouteNoCandidate
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now().UTC()
	} else {
		observation.ObservedAt = observation.ObservedAt.UTC()
	}
	return r.upsert(
		ctx,
		observation.Key,
		observation.ObservedAt,
		map[string]uint64{"actual_cost_samples": 1},
		observation.ActualBaseCostUSD,
		observation.ActualAccountCostUSD,
	)
}

func (r *openAIRouteObservationCheckpointRepository) upsert(
	ctx context.Context,
	key service.OpenAIRouteKey,
	observedAt time.Time,
	metrics map[string]uint64,
	baseCost float64,
	accountCost float64,
) error {
	if !key.Valid() || !finiteOpenAIRouteCheckpointCost(baseCost) || !finiteOpenAIRouteCheckpointCost(accountCost) {
		if !key.Valid() {
			return service.ErrOpenAIRouteNoCandidate
		}
		return service.ErrOpenAIRouteInvalidCost
	}
	metricsJSON, err := json.Marshal(metrics)
	if err != nil {
		return fmt.Errorf("encode OpenAI route checkpoint metrics: %w", err)
	}
	result, err := r.db.ExecContext(ctx, `
INSERT INTO openai_route_observation_hourly (
  route_fingerprint, hour_start,
  group_id, account_id, failure_domain, model, request_class, endpoint_hash, transport,
  metrics, actual_base_cost_usd, actual_account_cost_usd, last_observed_at
) VALUES (
  $1, $2,
  $3, $4, $5, $6, $7, $8, $9,
  $10::jsonb, $11, $12, $13
)
ON CONFLICT (route_fingerprint, hour_start) DO UPDATE SET
  metrics = (
    SELECT COALESCE(jsonb_object_agg(metric_key, to_jsonb(metric_total)), '{}'::jsonb)
    FROM (
      SELECT metric_key, SUM(metric_value)::bigint AS metric_total
      FROM (
        SELECT key AS metric_key, value::bigint AS metric_value
        FROM jsonb_each_text(openai_route_observation_hourly.metrics)
        UNION ALL
        SELECT key AS metric_key, value::bigint AS metric_value
        FROM jsonb_each_text(EXCLUDED.metrics)
      ) AS metric_deltas
      GROUP BY metric_key
    ) AS merged_metrics
  ),
  actual_base_cost_usd = openai_route_observation_hourly.actual_base_cost_usd + EXCLUDED.actual_base_cost_usd,
  actual_account_cost_usd = openai_route_observation_hourly.actual_account_cost_usd + EXCLUDED.actual_account_cost_usd,
  last_observed_at = GREATEST(openai_route_observation_hourly.last_observed_at, EXCLUDED.last_observed_at),
  updated_at = NOW()
WHERE
  openai_route_observation_hourly.group_id = EXCLUDED.group_id
  AND openai_route_observation_hourly.account_id = EXCLUDED.account_id
  AND openai_route_observation_hourly.failure_domain = EXCLUDED.failure_domain
  AND openai_route_observation_hourly.model = EXCLUDED.model
  AND openai_route_observation_hourly.request_class = EXCLUDED.request_class
  AND openai_route_observation_hourly.endpoint_hash = EXCLUDED.endpoint_hash
  AND openai_route_observation_hourly.transport = EXCLUDED.transport
`,
		service.OpenAIRouteObservationFingerprint(key), observedAt.Truncate(time.Hour),
		key.GroupID, key.AccountID, strings.TrimSpace(key.FailureDomain), strings.TrimSpace(key.Model),
		string(key.RequestClass), strings.TrimSpace(key.EndpointHash), strings.TrimSpace(key.Transport),
		string(metricsJSON), baseCost, accountCost, observedAt,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("OpenAI route checkpoint fingerprint identity mismatch")
	}
	return nil
}

func (r *openAIRouteObservationCheckpointRepository) GetBatch(
	ctx context.Context,
	keys []service.OpenAIRouteKey,
	now time.Time,
) (result map[string]service.OpenAIRouteObservationProfile, err error) {
	result, fingerprints, err := initializeOpenAIRouteCheckpointProfiles(keys)
	if err != nil || len(fingerprints) == 0 {
		return result, err
	}
	if r == nil || r.db == nil {
		return nil, service.ErrOpenAIRouteNoCandidate
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	beijing := now.In(beijingFixedZone)
	globalStart := time.Date(beijing.Year(), beijing.Month(), beijing.Day(), 0, 0, 0, 0, beijingFixedZone).
		AddDate(0, 0, -(openAIRouteGlobalDayCount - 1)).UTC()
	globalEnd := time.Date(beijing.Year(), beijing.Month(), beijing.Day(), 0, 0, 0, 0, beijingFixedZone).
		AddDate(0, 0, 1).UTC()
	seasonalStarts := make([]time.Time, 0, service.OpenAIRouteObservationSeasonalWeeks)
	currentHour := now.Truncate(time.Hour)
	for offset := 0; offset < service.OpenAIRouteObservationSeasonalWeeks; offset++ {
		seasonalStarts = append(seasonalStarts, currentHour.AddDate(0, 0, -7*offset))
	}

	rows, err := r.db.QueryContext(ctx, `
SELECT
  route_fingerprint,
  hour_start,
  metrics,
  actual_base_cost_usd::text,
  actual_account_cost_usd::text,
  last_observed_at
FROM openai_route_observation_hourly
WHERE route_fingerprint = ANY($1)
  AND (
    (hour_start >= $2 AND hour_start < $3)
    OR hour_start = ANY($4)
  )
ORDER BY route_fingerprint, hour_start
`, pq.Array(fingerprints), globalStart, globalEnd, pq.Array(seasonalStarts))
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, rows.Close())
	}()
	seasonalSet := make(map[int64]struct{}, len(seasonalStarts))
	for _, value := range seasonalStarts {
		seasonalSet[value.Unix()] = struct{}{}
	}
	for rows.Next() {
		var fingerprint string
		var hourStart, lastObservedAt time.Time
		var metricsJSON []byte
		var baseCostRaw, accountCostRaw string
		if err := rows.Scan(&fingerprint, &hourStart, &metricsJSON, &baseCostRaw, &accountCostRaw, &lastObservedAt); err != nil {
			return nil, err
		}
		aggregate, err := decodeOpenAIRouteCheckpointAggregate(metricsJSON, baseCostRaw, accountCostRaw, lastObservedAt)
		if err != nil {
			return nil, err
		}
		profile, ok := result[fingerprint]
		if !ok {
			return nil, fmt.Errorf("unexpected OpenAI route checkpoint fingerprint %q", fingerprint)
		}
		if !hourStart.Before(globalStart) && hourStart.Before(globalEnd) {
			profile.Global.Merge(aggregate)
		}
		if _, ok := seasonalSet[hourStart.UTC().Unix()]; ok {
			profile.HourOfWeek.Merge(aggregate)
		}
		result[fingerprint] = profile
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func openAIRouteObservationMetrics(observation service.OpenAIRouteObservation) (map[string]uint64, error) {
	metrics := map[string]uint64{"attempt_count": 1}
	if observation.Success {
		metrics["reliability_count"] = 1
		metrics["success_count"] = 1
	} else {
		metrics["failure_count"] = 1
		failureMetric, err := openAIRouteCheckpointFailureMetric(observation.FailureClass)
		if err != nil {
			return nil, err
		}
		metrics[failureMetric] = 1
		if observation.PenalizeRoute {
			metrics["reliability_count"] = 1
		}
	}
	scoreEligible := observation.Success || observation.PenalizeRoute
	if scoreEligible && observation.PartialStream {
		metrics["partial_streams"] = 1
	}
	if scoreEligible {
		if bucket := service.OpenAIRouteLatencyHistogramBucket(observation.TTFTMilliseconds); bucket >= 0 {
			metrics["ttft_sample_count"] = 1
			metrics[fmt.Sprintf("ttft:%02d", bucket)] = 1
		}
		if bucket := service.OpenAIRouteLatencyHistogramBucket(observation.CompletionLatencyMS); bucket >= 0 {
			metrics["latency_sample_count"] = 1
			metrics[fmt.Sprintf("latency:%02d", bucket)] = 1
		}
	}
	if observation.ActualCostAuthoritative {
		metrics["actual_cost_samples"] = 1
	}
	return metrics, nil
}

func openAIRouteCheckpointFailureMetric(class service.OpenAIRouteFailureClass) (string, error) {
	for _, candidate := range []service.OpenAIRouteFailureClass{
		service.OpenAIRouteFailureModelUnsupported,
		service.OpenAIRouteFailureRateLimit,
		service.OpenAIRouteFailureAuthentication,
		service.OpenAIRouteFailurePayment,
		service.OpenAIRouteFailureCapacity,
		service.OpenAIRouteFailureUpstream5xx,
		service.OpenAIRouteFailureMalformedStream,
		service.OpenAIRouteFailurePartialStream,
		service.OpenAIRouteFailureLocalTransport,
		service.OpenAIRouteFailureUserRequest,
		service.OpenAIRouteFailureClientCancelled,
	} {
		if class == candidate {
			return "failure:" + string(class), nil
		}
	}
	return "", fmt.Errorf("invalid OpenAI route checkpoint failure class %q", class)
}

func initializeOpenAIRouteCheckpointProfiles(keys []service.OpenAIRouteKey) (map[string]service.OpenAIRouteObservationProfile, []string, error) {
	result := make(map[string]service.OpenAIRouteObservationProfile, len(keys))
	fingerprints := make([]string, 0, len(keys))
	for _, key := range keys {
		if !key.Valid() {
			return nil, nil, service.ErrOpenAIRouteNoCandidate
		}
		fingerprint := service.OpenAIRouteObservationFingerprint(key)
		if _, exists := result[fingerprint]; exists {
			continue
		}
		result[fingerprint] = newOpenAIRouteObservationProfile()
		fingerprints = append(fingerprints, fingerprint)
	}
	return result, fingerprints, nil
}

func newOpenAIRouteObservationProfile() service.OpenAIRouteObservationProfile {
	return service.OpenAIRouteObservationProfile{
		Global:     service.NewOpenAIRouteObservationAggregate(),
		Recent:     service.NewOpenAIRouteObservationAggregate(),
		HourOfWeek: service.NewOpenAIRouteObservationAggregate(),
	}
}

func decodeOpenAIRouteCheckpointAggregate(
	metricsJSON []byte,
	baseCostRaw string,
	accountCostRaw string,
	lastObservedAt time.Time,
) (service.OpenAIRouteObservationAggregate, error) {
	var raw map[string]json.RawMessage
	if len(metricsJSON) == 0 {
		metricsJSON = []byte("{}")
	}
	if err := json.Unmarshal(metricsJSON, &raw); err != nil {
		return service.OpenAIRouteObservationAggregate{}, fmt.Errorf("decode OpenAI route checkpoint metrics: %w", err)
	}
	values := make(map[string]string, len(raw)+3)
	for key, value := range raw {
		var count uint64
		if err := json.Unmarshal(value, &count); err != nil {
			return service.OpenAIRouteObservationAggregate{}, fmt.Errorf("decode OpenAI route checkpoint metric %q: %w", key, err)
		}
		values[key] = strconv.FormatUint(count, 10)
	}
	values["actual_base_cost_usd"] = baseCostRaw
	values["actual_account_cost_usd"] = accountCostRaw
	values["last_observed_ms"] = strconv.FormatInt(lastObservedAt.UTC().UnixMilli(), 10)
	return decodeOpenAIRouteObservationAggregate(values)
}

func finiteOpenAIRouteCheckpointCost(value float64) bool {
	return value >= 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

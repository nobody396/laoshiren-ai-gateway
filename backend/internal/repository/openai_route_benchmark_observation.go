package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
)

const openAIRouteBenchmarkMaximumRows = 20_000

type openAIRouteBenchmarkObservationRepository struct {
	db *sql.DB
}

func NewOpenAIRouteBenchmarkObservationRepository(db *sql.DB) *openAIRouteBenchmarkObservationRepository {
	return &openAIRouteBenchmarkObservationRepository{db: db}
}

type openAIRouteBenchmarkMatchKey struct {
	accountID    int64
	model        string
	endpointHash string
}

// GetBenchmarkBatch converts the low-frequency self-owned Base URL sweep into
// a provenance-preserving active-probe prior. It never writes passive learner
// tables and never returns a raw URL to the service/audit layer.
func (r *openAIRouteBenchmarkObservationRepository) GetBenchmarkBatch(
	ctx context.Context,
	keys []service.OpenAIRouteKey,
	now time.Time,
) (result map[string]service.OpenAIRouteBenchmarkObservationProfile, err error) {
	result = make(map[string]service.OpenAIRouteBenchmarkObservationProfile, len(keys))
	if len(keys) == 0 {
		return result, nil
	}
	if r == nil || r.db == nil {
		return nil, service.ErrOpenAIRouteNoCandidate
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}

	keyByRoute := make(map[openAIRouteBenchmarkMatchKey][]service.OpenAIRouteKey)
	accountSet := make(map[int64]struct{})
	modelSet := make(map[string]struct{})
	for _, key := range keys {
		if !key.Valid() {
			return nil, service.ErrOpenAIRouteNoCandidate
		}
		fingerprint := service.OpenAIRouteObservationFingerprint(key)
		result[fingerprint] = service.NewOpenAIRouteBenchmarkObservationProfile()
		if key.RequestClass != service.OpenAIRouteRequestClassText || strings.TrimSpace(key.Transport) != string(service.OpenAIUpstreamTransportHTTPSSE) {
			continue
		}
		match := openAIRouteBenchmarkMatchKey{
			accountID: key.AccountID, model: strings.TrimSpace(key.Model), endpointHash: strings.TrimSpace(key.EndpointHash),
		}
		keyByRoute[match] = append(keyByRoute[match], key)
		accountSet[key.AccountID] = struct{}{}
		modelSet[strings.TrimSpace(key.Model)] = struct{}{}
	}
	if len(keyByRoute) == 0 {
		return result, nil
	}
	accountIDs := make([]int64, 0, len(accountSet))
	for accountID := range accountSet {
		accountIDs = append(accountIDs, accountID)
	}
	sort.Slice(accountIDs, func(i, j int) bool { return accountIDs[i] < accountIDs[j] })
	models := make([]string, 0, len(modelSet))
	for model := range modelSet {
		models = append(models, model)
	}
	sort.Strings(models)
	seasonalStart := now.Add(-time.Duration(service.OpenAIRouteBenchmarkSeasonalWeeks) * 7 * 24 * time.Hour)
	globalStart := now.Add(-service.OpenAIRouteBenchmarkGlobalWindow)
	recentStart := now.Add(-service.OpenAIRouteBenchmarkRecentWindow)
	beijingNow := now.In(beijingFixedZone)

	rows, err := r.db.QueryContext(ctx, `
SELECT
  account_id,
  model,
  base_url,
  sampled_at,
  passed,
  first_text_ms,
  total_ms,
  COALESCE(error_type, ''),
  COALESCE(stream_complete, passed)
FROM base_url_benchmarks
WHERE product = 'gpt'
  AND sampled_at >= $1
  AND account_id = ANY($2)
  AND model = ANY($3)
  AND (
    sampled_at >= $4
    OR (
      EXTRACT(DOW FROM sampled_at AT TIME ZONE 'Asia/Shanghai') = $5
      AND EXTRACT(HOUR FROM sampled_at AT TIME ZONE 'Asia/Shanghai') = $6
    )
  )
ORDER BY sampled_at ASC
LIMIT $7
`,
		seasonalStart,
		pq.Array(accountIDs),
		pq.Array(models),
		globalStart,
		int(beijingNow.Weekday()),
		beijingNow.Hour(),
		openAIRouteBenchmarkMaximumRows+1,
	)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, rows.Close())
	}()
	rowCount := 0
	for rows.Next() {
		rowCount++
		if rowCount > openAIRouteBenchmarkMaximumRows {
			return nil, fmt.Errorf("OpenAI route benchmark result exceeds %d rows", openAIRouteBenchmarkMaximumRows)
		}
		var (
			accountID      int64
			model          string
			baseURL        string
			sampledAt      time.Time
			passed         bool
			firstTextMS    sql.NullInt64
			totalMS        sql.NullInt64
			errorType      string
			streamComplete bool
		)
		if scanErr := rows.Scan(
			&accountID, &model, &baseURL, &sampledAt, &passed,
			&firstTextMS, &totalMS, &errorType, &streamComplete,
		); scanErr != nil {
			return nil, scanErr
		}
		endpoint := service.OpenAIRouteEndpointForBaseURL(baseURL, "/v1/responses")
		match := openAIRouteBenchmarkMatchKey{
			accountID: accountID, model: strings.TrimSpace(model), endpointHash: service.OpenAIRouteEndpointHash(endpoint),
		}
		matchedKeys := keyByRoute[match]
		if len(matchedKeys) == 0 {
			continue
		}
		aggregate := openAIRouteBenchmarkRowAggregate(
			sampledAt.UTC(), passed && streamComplete, strings.TrimSpace(errorType), firstTextMS, totalMS,
		)
		beijingSample := sampledAt.In(beijingFixedZone)
		seasonalMatch := beijingSample.Weekday() == beijingNow.Weekday() && beijingSample.Hour() == beijingNow.Hour()
		for _, key := range matchedKeys {
			fingerprint := service.OpenAIRouteObservationFingerprint(key)
			profile := result[fingerprint]
			if !sampledAt.Before(globalStart) {
				profile.Global.Merge(aggregate)
			}
			if !sampledAt.Before(recentStart) {
				profile.Recent.Merge(aggregate)
			}
			if seasonalMatch {
				profile.HourOfWeek.Merge(aggregate)
			}
			result[fingerprint] = profile
		}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	for fingerprint, profile := range result {
		result[fingerprint] = service.FinalizeOpenAIRouteBenchmarkObservationProfile(profile, now)
	}
	return result, nil
}

func openAIRouteBenchmarkRowAggregate(
	sampledAt time.Time,
	success bool,
	errorType string,
	firstTextMS sql.NullInt64,
	totalMS sql.NullInt64,
) service.OpenAIRouteObservationAggregate {
	aggregate := service.NewOpenAIRouteObservationAggregate()
	aggregate.AttemptCount = 1
	aggregate.ReliabilityCount = 1
	aggregate.LastObservedAt = sampledAt.UTC()
	if success {
		aggregate.SuccessCount = 1
	} else {
		aggregate.FailureCount = 1
		failureClass := openAIRouteBenchmarkFailureClass(errorType)
		aggregate.FailureCounts[failureClass] = 1
		if failureClass == service.OpenAIRouteFailurePartialStream {
			aggregate.PartialStreams = 1
		}
	}
	if firstTextMS.Valid && firstTextMS.Int64 > 0 {
		aggregate.TTFTSampleCount = 1
		aggregate.TTFTHistogram[service.OpenAIRouteLatencyHistogramBucket(firstTextMS.Int64)] = 1
	}
	if totalMS.Valid && totalMS.Int64 > 0 {
		aggregate.LatencySampleCount = 1
		aggregate.LatencyHistogram[service.OpenAIRouteLatencyHistogramBucket(totalMS.Int64)] = 1
	}
	return aggregate
}

func openAIRouteBenchmarkFailureClass(errorType string) service.OpenAIRouteFailureClass {
	switch strings.ToLower(strings.TrimSpace(errorType)) {
	case "rate_limited":
		return service.OpenAIRouteFailureRateLimit
	case "http_5xx", "upstream_error", "capacity":
		return service.OpenAIRouteFailureUpstream5xx
	case "http_4xx", "authentication":
		return service.OpenAIRouteFailureAuthentication
	case "stream_incomplete":
		return service.OpenAIRouteFailurePartialStream
	case "wrong_response", "wrong_model", "usage_missing":
		return service.OpenAIRouteFailureMalformedStream
	case "timeout", "connection":
		return service.OpenAIRouteFailureLocalTransport
	default:
		return service.OpenAIRouteFailureCapacity
	}
}

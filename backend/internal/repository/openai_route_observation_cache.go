package repository

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	openAIRouteObservationKeyPrefix = "route:v2:observation:"
	openAIRouteRecentBucket         = 5 * time.Minute
	openAIRouteRecentBucketCount    = int(service.OpenAIRouteObservationRecentWindow / openAIRouteRecentBucket)
	openAIRouteGlobalDayCount       = int(service.OpenAIRouteObservationGlobalWindow / (24 * time.Hour))
	openAIRouteRecentTTL            = 2 * time.Hour
	openAIRouteGlobalTTL            = 10 * 24 * time.Hour
	openAIRouteSeasonalTTL          = 10 * 7 * 24 * time.Hour
)

var beijingFixedZone = time.FixedZone("Asia/Shanghai", 8*60*60)

var recordOpenAIRouteObservationScript = redis.NewScript(`
local ttl_recent = tonumber(ARGV[1])
local ttl_global = tonumber(ARGV[2])
local ttl_seasonal = tonumber(ARGV[3])
local observed_ms = tonumber(ARGV[4])
local count_attempt = tonumber(ARGV[5])
local success = tonumber(ARGV[6])
local reliability = tonumber(ARGV[7])
local failure = tonumber(ARGV[8])
local partial = tonumber(ARGV[9])
local failure_field = ARGV[10]
local ttft_field = ARGV[11]
local latency_field = ARGV[12]
local authoritative_cost = tonumber(ARGV[13])
local base_cost = ARGV[14]
local account_cost = ARGV[15]

for index, key in ipairs(KEYS) do
  if count_attempt == 1 then
    redis.call('HINCRBY', key, 'attempt_count', 1)
    redis.call('HINCRBY', key, 'reliability_count', reliability)
    redis.call('HINCRBY', key, 'success_count', success)
    redis.call('HINCRBY', key, 'failure_count', failure)
    redis.call('HINCRBY', key, 'partial_streams', partial)
    if failure_field ~= '' then
      redis.call('HINCRBY', key, failure_field, 1)
    end
    if ttft_field ~= '' then
      redis.call('HINCRBY', key, 'ttft_sample_count', 1)
      redis.call('HINCRBY', key, ttft_field, 1)
    end
    if latency_field ~= '' then
      redis.call('HINCRBY', key, 'latency_sample_count', 1)
      redis.call('HINCRBY', key, latency_field, 1)
    end
  end
  if authoritative_cost == 1 then
    redis.call('HINCRBY', key, 'actual_cost_samples', 1)
    redis.call('HINCRBYFLOAT', key, 'actual_base_cost_usd', base_cost)
    redis.call('HINCRBYFLOAT', key, 'actual_account_cost_usd', account_cost)
  end
  local previous_ms = tonumber(redis.call('HGET', key, 'last_observed_ms') or '0')
  if observed_ms > previous_ms then
    redis.call('HSET', key, 'last_observed_ms', observed_ms)
  end
  local ttl = ttl_seasonal
  if index == 1 then
    ttl = ttl_recent
  elseif index == 2 then
    ttl = ttl_global
  end
  redis.call('EXPIRE', key, ttl)
end
return #KEYS
`)

type openAIRouteObservationCache struct {
	rdb *redis.Client
}

func NewOpenAIRouteObservationCache(rdb *redis.Client) service.OpenAIRouteObservationStore {
	return &openAIRouteObservationCache{rdb: rdb}
}

func (c *openAIRouteObservationCache) Check(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return service.ErrOpenAIRouteNoCandidate
	}
	return c.rdb.Ping(ctx).Err()
}

func (c *openAIRouteObservationCache) Record(ctx context.Context, observation service.OpenAIRouteObservation) error {
	if c == nil || c.rdb == nil {
		return service.ErrOpenAIRouteNoCandidate
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now()
	}
	beijing := observation.ObservedAt.In(beijingFixedZone)
	fingerprint := service.OpenAIRouteObservationFingerprint(observation.Key)
	keys := []string{
		openAIRouteObservationRecentKey(fingerprint, beijing),
		openAIRouteObservationGlobalKey(fingerprint, beijing),
		openAIRouteObservationSeasonalKey(fingerprint, beijing),
	}

	success := 0
	reliability := 0
	failure := 0
	failureField := ""
	if observation.Success {
		success = 1
		reliability = 1
	} else {
		failure = 1
		failureField = "failure:" + string(observation.FailureClass)
		if observation.PenalizeRoute {
			reliability = 1
		}
	}
	partial := 0
	// Neutral outcomes (for example client cancellation or a user-invalid 4xx)
	// remain visible in attempt/failure counts but must not train latency or
	// stream-integrity scoring.
	scoreEligible := observation.Success || observation.PenalizeRoute
	if scoreEligible && observation.PartialStream {
		partial = 1
	}
	ttftField := ""
	latencyField := ""
	if scoreEligible {
		ttftField = openAIRouteObservationHistogramField("ttft", observation.TTFTMilliseconds)
		latencyField = openAIRouteObservationHistogramField("latency", observation.CompletionLatencyMS)
	}
	authoritativeCost := 0
	if observation.ActualCostAuthoritative {
		authoritativeCost = 1
	}
	args := []any{
		int64(openAIRouteRecentTTL / time.Second),
		int64(openAIRouteGlobalTTL / time.Second),
		int64(openAIRouteSeasonalTTL / time.Second),
		observation.ObservedAt.UnixMilli(),
		1,
		success,
		reliability,
		failure,
		partial,
		failureField,
		ttftField,
		latencyField,
		authoritativeCost,
		strconv.FormatFloat(observation.ActualBaseCostUSD, 'f', -1, 64),
		strconv.FormatFloat(observation.ActualAccountCostUSD, 'f', -1, 64),
	}
	return recordOpenAIRouteObservationScript.Run(ctx, c.rdb, keys, args...).Err()
}

func (c *openAIRouteObservationCache) RecordCost(ctx context.Context, observation service.OpenAIRouteActualCostObservation) error {
	if c == nil || c.rdb == nil {
		return service.ErrOpenAIRouteNoCandidate
	}
	if err := observation.Validate(); err != nil {
		return err
	}
	if observation.ObservedAt.IsZero() {
		observation.ObservedAt = time.Now()
	}
	beijing := observation.ObservedAt.In(beijingFixedZone)
	fingerprint := service.OpenAIRouteObservationFingerprint(observation.Key)
	keys := []string{
		openAIRouteObservationRecentKey(fingerprint, beijing),
		openAIRouteObservationGlobalKey(fingerprint, beijing),
		openAIRouteObservationSeasonalKey(fingerprint, beijing),
	}
	args := []any{
		int64(openAIRouteRecentTTL / time.Second),
		int64(openAIRouteGlobalTTL / time.Second),
		int64(openAIRouteSeasonalTTL / time.Second),
		observation.ObservedAt.UnixMilli(),
		0, 0, 0, 0, 0, "", "", "", 1,
		strconv.FormatFloat(observation.ActualBaseCostUSD, 'f', -1, 64),
		strconv.FormatFloat(observation.ActualAccountCostUSD, 'f', -1, 64),
	}
	return recordOpenAIRouteObservationScript.Run(ctx, c.rdb, keys, args...).Err()
}

type openAIRouteObservationQuery struct {
	fingerprint string
	view        string
	command     *redis.MapStringStringCmd
}

func (c *openAIRouteObservationCache) GetBatch(
	ctx context.Context,
	keys []service.OpenAIRouteKey,
	now time.Time,
) (map[string]service.OpenAIRouteObservationProfile, error) {
	result := make(map[string]service.OpenAIRouteObservationProfile, len(keys))
	if len(keys) == 0 {
		return result, nil
	}
	if c == nil || c.rdb == nil {
		return nil, service.ErrOpenAIRouteNoCandidate
	}
	if now.IsZero() {
		now = time.Now()
	}
	beijing := now.In(beijingFixedZone)

	seen := make(map[string]struct{}, len(keys))
	queries := make([]openAIRouteObservationQuery, 0, len(keys)*(openAIRouteRecentBucketCount+openAIRouteGlobalDayCount+service.OpenAIRouteObservationSeasonalWeeks))
	_, err := c.rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, key := range keys {
			if !key.Valid() {
				return service.ErrOpenAIRouteNoCandidate
			}
			fingerprint := service.OpenAIRouteObservationFingerprint(key)
			if _, ok := seen[fingerprint]; ok {
				continue
			}
			seen[fingerprint] = struct{}{}
			result[fingerprint] = service.OpenAIRouteObservationProfile{
				Global:     service.NewOpenAIRouteObservationAggregate(),
				Recent:     service.NewOpenAIRouteObservationAggregate(),
				HourOfWeek: service.NewOpenAIRouteObservationAggregate(),
			}
			for _, redisKey := range openAIRouteObservationRecentReadKeys(fingerprint, beijing) {
				queries = append(queries, openAIRouteObservationQuery{fingerprint: fingerprint, view: "recent", command: pipe.HGetAll(ctx, redisKey)})
			}
			for _, redisKey := range openAIRouteObservationGlobalReadKeys(fingerprint, beijing) {
				queries = append(queries, openAIRouteObservationQuery{fingerprint: fingerprint, view: "global", command: pipe.HGetAll(ctx, redisKey)})
			}
			for _, redisKey := range openAIRouteObservationSeasonalReadKeys(fingerprint, beijing) {
				queries = append(queries, openAIRouteObservationQuery{fingerprint: fingerprint, view: "seasonal", command: pipe.HGetAll(ctx, redisKey)})
			}
		}
		return nil
	})
	if err != nil && err != redis.Nil {
		return nil, err
	}

	for _, query := range queries {
		values, commandErr := query.command.Result()
		if commandErr != nil && commandErr != redis.Nil {
			return nil, commandErr
		}
		if len(values) == 0 {
			continue
		}
		aggregate, parseErr := decodeOpenAIRouteObservationAggregate(values)
		if parseErr != nil {
			return nil, parseErr
		}
		profile := result[query.fingerprint]
		switch query.view {
		case "recent":
			profile.Recent.Merge(aggregate)
		case "global":
			profile.Global.Merge(aggregate)
		case "seasonal":
			profile.HourOfWeek.Merge(aggregate)
		}
		result[query.fingerprint] = profile
	}
	return result, nil
}

func openAIRouteObservationRecentKey(fingerprint string, now time.Time) string {
	bucket := now.Unix() / int64(openAIRouteRecentBucket/time.Second)
	return openAIRouteObservationKeyPrefix + fingerprint + ":recent:" + strconv.FormatInt(bucket, 10)
}

func openAIRouteObservationGlobalKey(fingerprint string, now time.Time) string {
	return openAIRouteObservationKeyPrefix + fingerprint + ":day:" + now.In(beijingFixedZone).Format("20060102")
}

func openAIRouteObservationSeasonalKey(fingerprint string, now time.Time) string {
	beijing := now.In(beijingFixedZone)
	year, week := beijing.ISOWeek()
	return fmt.Sprintf("%s%s:hour:%04d%02d:%03d", openAIRouteObservationKeyPrefix, fingerprint, year, week, openAIRouteBeijingHourOfWeek(beijing))
}

func openAIRouteObservationRecentReadKeys(fingerprint string, now time.Time) []string {
	keys := make([]string, 0, openAIRouteRecentBucketCount)
	current := now.Unix() / int64(openAIRouteRecentBucket/time.Second)
	for offset := int64(0); offset < int64(openAIRouteRecentBucketCount); offset++ {
		keys = append(keys, openAIRouteObservationKeyPrefix+fingerprint+":recent:"+strconv.FormatInt(current-offset, 10))
	}
	return keys
}

func openAIRouteObservationGlobalReadKeys(fingerprint string, now time.Time) []string {
	keys := make([]string, 0, openAIRouteGlobalDayCount)
	beijing := now.In(beijingFixedZone)
	for offset := 0; offset < openAIRouteGlobalDayCount; offset++ {
		keys = append(keys, openAIRouteObservationGlobalKey(fingerprint, beijing.AddDate(0, 0, -offset)))
	}
	return keys
}

func openAIRouteObservationSeasonalReadKeys(fingerprint string, now time.Time) []string {
	keys := make([]string, 0, service.OpenAIRouteObservationSeasonalWeeks)
	beijing := now.In(beijingFixedZone)
	for offset := 0; offset < service.OpenAIRouteObservationSeasonalWeeks; offset++ {
		keys = append(keys, openAIRouteObservationSeasonalKey(fingerprint, beijing.AddDate(0, 0, -7*offset)))
	}
	return keys
}

func openAIRouteBeijingHourOfWeek(value time.Time) int {
	beijing := value.In(beijingFixedZone)
	weekday := (int(beijing.Weekday()) + 6) % 7 // Monday=0
	return weekday*24 + beijing.Hour()
}

func openAIRouteObservationHistogramField(prefix string, milliseconds int64) string {
	bucket := service.OpenAIRouteLatencyHistogramBucket(milliseconds)
	if bucket < 0 {
		return ""
	}
	return fmt.Sprintf("%s:%02d", prefix, bucket)
}

func decodeOpenAIRouteObservationAggregate(values map[string]string) (service.OpenAIRouteObservationAggregate, error) {
	aggregate := service.NewOpenAIRouteObservationAggregate()
	var err error
	if aggregate.AttemptCount, err = parseOpenAIRouteObservationUint(values, "attempt_count"); err != nil {
		return aggregate, err
	}
	if aggregate.ReliabilityCount, err = parseOpenAIRouteObservationUint(values, "reliability_count"); err != nil {
		return aggregate, err
	}
	if aggregate.SuccessCount, err = parseOpenAIRouteObservationUint(values, "success_count"); err != nil {
		return aggregate, err
	}
	if aggregate.FailureCount, err = parseOpenAIRouteObservationUint(values, "failure_count"); err != nil {
		return aggregate, err
	}
	if aggregate.PartialStreams, err = parseOpenAIRouteObservationUint(values, "partial_streams"); err != nil {
		return aggregate, err
	}
	if aggregate.TTFTSampleCount, err = parseOpenAIRouteObservationUint(values, "ttft_sample_count"); err != nil {
		return aggregate, err
	}
	if aggregate.LatencySampleCount, err = parseOpenAIRouteObservationUint(values, "latency_sample_count"); err != nil {
		return aggregate, err
	}
	if aggregate.ActualCostSamples, err = parseOpenAIRouteObservationUint(values, "actual_cost_samples"); err != nil {
		return aggregate, err
	}
	if aggregate.ActualBaseCostUSD, err = parseOpenAIRouteObservationFloat(values, "actual_base_cost_usd"); err != nil {
		return aggregate, err
	}
	if aggregate.ActualAccountCostUSD, err = parseOpenAIRouteObservationFloat(values, "actual_account_cost_usd"); err != nil {
		return aggregate, err
	}
	for key, raw := range values {
		switch {
		case strings.HasPrefix(key, "failure:"):
			count, parseErr := strconv.ParseUint(raw, 10, 64)
			if parseErr != nil {
				return aggregate, fmt.Errorf("decode route observation %s: %w", key, parseErr)
			}
			aggregate.FailureCounts[service.OpenAIRouteFailureClass(strings.TrimPrefix(key, "failure:"))] += count
		case strings.HasPrefix(key, "ttft:"):
			if parseErr := decodeOpenAIRouteObservationHistogramValue(aggregate.TTFTHistogram, key, raw); parseErr != nil {
				return aggregate, parseErr
			}
		case strings.HasPrefix(key, "latency:"):
			if parseErr := decodeOpenAIRouteObservationHistogramValue(aggregate.LatencyHistogram, key, raw); parseErr != nil {
				return aggregate, parseErr
			}
		}
	}
	lastObservedMS, parseErr := parseOpenAIRouteObservationInt(values, "last_observed_ms")
	if parseErr != nil {
		return aggregate, parseErr
	}
	if lastObservedMS > 0 {
		aggregate.LastObservedAt = time.UnixMilli(lastObservedMS).UTC()
	}
	return aggregate, nil
}

func decodeOpenAIRouteObservationHistogramValue(histogram []uint64, key, raw string) error {
	separator := strings.LastIndexByte(key, ':')
	if separator < 0 || separator == len(key)-1 {
		return fmt.Errorf("decode route observation histogram key %q", key)
	}
	idx, err := strconv.Atoi(key[separator+1:])
	if err != nil || idx < 0 || idx >= len(histogram) {
		return fmt.Errorf("decode route observation histogram key %q", key)
	}
	count, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return fmt.Errorf("decode route observation %s: %w", key, err)
	}
	histogram[idx] += count
	return nil
}

func parseOpenAIRouteObservationUint(values map[string]string, field string) (uint64, error) {
	raw := strings.TrimSpace(values[field])
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("decode route observation %s: %w", field, err)
	}
	return parsed, nil
}

func parseOpenAIRouteObservationInt(values map[string]string, field string) (int64, error) {
	raw := strings.TrimSpace(values[field])
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("decode route observation %s: %w", field, err)
	}
	return parsed, nil
}

func parseOpenAIRouteObservationFloat(values map[string]string, field string) (float64, error) {
	raw := strings.TrimSpace(values[field])
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
		return 0, fmt.Errorf("decode route observation %s", field)
	}
	return parsed, nil
}

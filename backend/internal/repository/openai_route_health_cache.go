package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	openAIRouteHealthKeyPrefix           = "route:v1:health:"
	openAIRouteHalfOpenKeyPrefix         = "route:v1:halfopen:"
	openAIRouteProviderEvidenceKeyPrefix = "route:v2:provider-evidence:"
	openAIRouteProviderEpisodeKeyPrefix  = "route:v2:provider-episode:"
	defaultOpenAIRouteHealthTTL          = 24 * time.Hour
	defaultOpenAIRoutePermitTTL          = 30 * time.Second
	openAIRouteHealthCASRetries          = 32
)

var (
	acquireOpenAIRouteHalfOpenScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if current == false then
  redis.call('SET', KEYS[1], ARGV[1], 'EX', ARGV[2])
  return 1
end
if current == ARGV[1] then
  redis.call('EXPIRE', KEYS[1], ARGV[2])
  return 1
end
return 0
`)
	releaseOpenAIRouteHalfOpenScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if current ~= false and current == ARGV[1] then
  return redis.call('DEL', KEYS[1])
end
return 0
`)
	refreshOpenAIRouteHalfOpenScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if current ~= false and current == ARGV[1] then
  redis.call('EXPIRE', KEYS[1], ARGV[2])
  return 1
end
return 0
`)
	recordOpenAIRouteProviderEvidenceScript = redis.NewScript(`
local member = ARGV[1]
local observed_ms = tonumber(ARGV[2])
local cutoff_ms = tonumber(ARGV[3])
local ttl_ms = tonumber(ARGV[4])
local min_distinct = tonumber(ARGV[5])
local success = tonumber(ARGV[6])
local episode = ARGV[7]

redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', cutoff_ms)
if success == 1 then
  redis.call('ZREM', KEYS[1], member)
else
  redis.call('ZADD', KEYS[1], observed_ms, member)
end
local count = redis.call('ZCARD', KEYS[1])
if count > 0 then
  redis.call('PEXPIRE', KEYS[1], ttl_ms)
else
  redis.call('DEL', KEYS[1])
end

if success == 1 then
  if count == 0 then
    redis.call('DEL', KEYS[2])
    return {count, 1}
  end
  return {count, 0}
end

if count >= min_distinct then
  local acquired = redis.call('SET', KEYS[2], episode, 'NX', 'PX', ttl_ms)
  if acquired then
    return {count, 1}
  end
end
return {count, 0}
`)
)

type openAIRouteHealthCache struct {
	rdb       *redis.Client
	healthTTL time.Duration
	permitTTL time.Duration
}

func NewOpenAIRouteHealthCache(rdb *redis.Client, healthTTL, permitTTL time.Duration) service.OpenAIRouteHealthStore {
	if healthTTL <= 0 {
		healthTTL = defaultOpenAIRouteHealthTTL
	}
	if permitTTL <= 0 {
		permitTTL = defaultOpenAIRoutePermitTTL
	}
	if permitTTL < time.Second {
		permitTTL = time.Second
	}
	return &openAIRouteHealthCache{rdb: rdb, healthTTL: healthTTL, permitTTL: permitTTL}
}

func (c *openAIRouteHealthCache) Get(ctx context.Context, key service.OpenAIRouteHealthStoreKey) (service.OpenAIRouteHealthState, error) {
	if c == nil || c.rdb == nil || !key.Valid() {
		return service.OpenAIRouteHealthState{}, service.ErrOpenAIRouteNoCandidate
	}
	raw, err := c.rdb.Get(ctx, openAIRouteHealthRedisKey(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return service.NewOpenAIRouteHealthState(), nil
	}
	if err != nil {
		return service.OpenAIRouteHealthState{}, err
	}
	return decodeOpenAIRouteHealthState(raw)
}

func (c *openAIRouteHealthCache) GetBatch(ctx context.Context, keys []service.OpenAIRouteHealthStoreKey) (map[string]service.OpenAIRouteHealthState, error) {
	result := make(map[string]service.OpenAIRouteHealthState, len(keys))
	if len(keys) == 0 {
		return result, nil
	}
	if c == nil || c.rdb == nil {
		return nil, service.ErrOpenAIRouteNoCandidate
	}
	commands := make(map[string]*redis.StringCmd, len(keys))
	_, err := c.rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, key := range keys {
			if !key.Valid() {
				return service.ErrOpenAIRouteNoCandidate
			}
			fingerprint := key.Fingerprint()
			if _, exists := commands[fingerprint]; exists {
				continue
			}
			commands[fingerprint] = pipe.Get(ctx, openAIRouteHealthRedisKey(key))
		}
		return nil
	})
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	for fingerprint, command := range commands {
		raw, commandErr := command.Bytes()
		switch {
		case errors.Is(commandErr, redis.Nil):
			result[fingerprint] = service.NewOpenAIRouteHealthState()
		case commandErr != nil:
			return nil, commandErr
		default:
			state, decodeErr := decodeOpenAIRouteHealthState(raw)
			if decodeErr != nil {
				return nil, decodeErr
			}
			result[fingerprint] = state
		}
	}
	return result, nil
}

func (c *openAIRouteHealthCache) ApplyEvent(ctx context.Context, key service.OpenAIRouteHealthStoreKey, event service.OpenAIRouteHealthEvent, policy service.OpenAIRoutePolicy) (service.OpenAIRouteHealthState, error) {
	if c == nil || c.rdb == nil || !key.Valid() {
		return service.OpenAIRouteHealthState{}, service.ErrOpenAIRouteNoCandidate
	}
	redisKey := openAIRouteHealthRedisKey(key)
	var updated service.OpenAIRouteHealthState
	for attempt := 0; attempt < openAIRouteHealthCASRetries; attempt++ {
		err := c.rdb.Watch(ctx, func(tx *redis.Tx) error {
			current := service.NewOpenAIRouteHealthState()
			raw, getErr := tx.Get(ctx, redisKey).Bytes()
			switch {
			case getErr == nil:
				decoded, decodeErr := decodeOpenAIRouteHealthState(raw)
				if decodeErr != nil {
					return decodeErr
				}
				current = decoded
			case errors.Is(getErr, redis.Nil):
			default:
				return getErr
			}

			next, transitionErr := service.ApplyOpenAIRouteHealthEvent(current, event, policy)
			if transitionErr != nil {
				return transitionErr
			}
			encoded, encodeErr := json.Marshal(next)
			if encodeErr != nil {
				return encodeErr
			}
			_, pipelineErr := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, redisKey, encoded, c.healthTTL)
				return nil
			})
			if pipelineErr == nil {
				updated = next
			}
			return pipelineErr
		}, redisKey)
		if err == nil {
			return updated, nil
		}
		if !errors.Is(err, redis.TxFailedErr) {
			return service.OpenAIRouteHealthState{}, err
		}
	}
	return service.OpenAIRouteHealthState{}, fmt.Errorf("openai route health CAS retries exhausted")
}

func (c *openAIRouteHealthCache) RecordProviderEvidence(
	ctx context.Context,
	routeKey service.OpenAIRouteKey,
	event service.OpenAIRouteHealthEvent,
	policy service.OpenAIRoutePolicy,
	minDistinctAccounts int,
) (service.OpenAIRouteProviderEvidenceResult, error) {
	result := service.OpenAIRouteProviderEvidenceResult{}
	if c == nil || c.rdb == nil || !routeKey.Valid() || !service.OpenAIRouteHasSharedFailureDomain(routeKey) {
		return result, service.ErrOpenAIRouteNoCandidate
	}
	normalized, err := service.NormalizeOpenAIRoutePolicy(policy)
	if err != nil {
		return result, err
	}
	if minDistinctAccounts < 2 {
		minDistinctAccounts = 2
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	providerKey := service.OpenAIRouteHealthStoreKeyForProvider(routeKey)
	fingerprint := providerKey.Fingerprint()
	// The shared hash tag keeps both Lua keys in one Redis Cluster slot.
	evidenceKey := openAIRouteProviderEvidenceKeyPrefix + "{" + fingerprint + "}"
	episodeKey := openAIRouteProviderEpisodeKeyPrefix + "{" + fingerprint + "}"
	evidenceTTL := 2 * normalized.FailureWindow
	if evidenceTTL < time.Minute {
		evidenceTTL = time.Minute
	}
	success := 0
	if event.Success || event.FailureClass == service.OpenAIRouteFailureNone {
		success = 1
	}
	episode := fmt.Sprintf("%d:%s", event.At.UnixNano(), service.OpenAIRouteHealthStoreKeyForRoute(routeKey).Fingerprint())
	raw, err := recordOpenAIRouteProviderEvidenceScript.Run(ctx, c.rdb, []string{evidenceKey, episodeKey},
		strconv.FormatInt(routeKey.AccountID, 10),
		event.At.UnixMilli(),
		event.At.Add(-normalized.FailureWindow).UnixMilli(),
		evidenceTTL.Milliseconds(),
		minDistinctAccounts,
		success,
		episode,
	).Slice()
	if err != nil {
		return result, err
	}
	if len(raw) != 2 {
		return result, fmt.Errorf("invalid OpenAI provider evidence result")
	}
	count, ok := raw[0].(int64)
	if !ok {
		return result, fmt.Errorf("invalid OpenAI provider evidence count")
	}
	action, ok := raw[1].(int64)
	if !ok {
		return result, fmt.Errorf("invalid OpenAI provider evidence action")
	}
	result.DistinctFailingAccounts = int(count)
	if action != 1 {
		return result, nil
	}

	if success == 1 {
		current, getErr := c.Get(ctx, providerKey)
		if getErr != nil {
			return result, getErr
		}
		if current.State == service.OpenAIRouteCircuitHealthy || current.State == service.OpenAIRouteCircuitWarmup {
			result.State = current
			return result, nil
		}
	}
	state, applyErr := c.ApplyEvent(ctx, providerKey, event, normalized)
	if applyErr != nil {
		if success == 0 {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			_ = c.rdb.Del(cleanupCtx, episodeKey).Err()
			cleanupCancel()
		}
		return result, applyErr
	}
	result.ProviderEventApplied = true
	result.State = state
	return result, nil
}

func (c *openAIRouteHealthCache) AcquireHalfOpenPermit(ctx context.Context, key service.OpenAIRouteHealthStoreKey, owner string) (bool, error) {
	if c == nil || c.rdb == nil || !key.Valid() || strings.TrimSpace(owner) == "" {
		return false, service.ErrOpenAIRouteNoCandidate
	}
	result, err := acquireOpenAIRouteHalfOpenScript.Run(
		ctx,
		c.rdb,
		[]string{openAIRouteHalfOpenRedisKey(key)},
		strings.TrimSpace(owner),
		int64(c.permitTTL.Seconds()),
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (c *openAIRouteHealthCache) ReleaseHalfOpenPermit(ctx context.Context, key service.OpenAIRouteHealthStoreKey, owner string) error {
	if c == nil || c.rdb == nil || !key.Valid() || strings.TrimSpace(owner) == "" {
		return service.ErrOpenAIRouteNoCandidate
	}
	return releaseOpenAIRouteHalfOpenScript.Run(
		ctx,
		c.rdb,
		[]string{openAIRouteHalfOpenRedisKey(key)},
		strings.TrimSpace(owner),
	).Err()
}

func (c *openAIRouteHealthCache) RefreshHalfOpenPermit(ctx context.Context, key service.OpenAIRouteHealthStoreKey, owner string) (bool, error) {
	if c == nil || c.rdb == nil || !key.Valid() || strings.TrimSpace(owner) == "" {
		return false, service.ErrOpenAIRouteNoCandidate
	}
	result, err := refreshOpenAIRouteHalfOpenScript.Run(
		ctx,
		c.rdb,
		[]string{openAIRouteHalfOpenRedisKey(key)},
		strings.TrimSpace(owner),
		int64(c.permitTTL.Seconds()),
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func openAIRouteHealthRedisKey(key service.OpenAIRouteHealthStoreKey) string {
	return openAIRouteHealthKeyPrefix + key.Fingerprint()
}

func openAIRouteHalfOpenRedisKey(key service.OpenAIRouteHealthStoreKey) string {
	return openAIRouteHalfOpenKeyPrefix + key.Fingerprint()
}

func decodeOpenAIRouteHealthState(raw []byte) (service.OpenAIRouteHealthState, error) {
	var state service.OpenAIRouteHealthState
	if err := json.Unmarshal(raw, &state); err != nil {
		return service.OpenAIRouteHealthState{}, fmt.Errorf("decode OpenAI route health state: %w", err)
	}
	return state, nil
}

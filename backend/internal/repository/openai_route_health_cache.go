package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	openAIRouteHealthKeyPrefix   = "route:v1:health:"
	openAIRouteHalfOpenKeyPrefix = "route:v1:halfopen:"
	defaultOpenAIRouteHealthTTL  = 24 * time.Hour
	defaultOpenAIRoutePermitTTL  = 30 * time.Second
	openAIRouteHealthCASRetries  = 32
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

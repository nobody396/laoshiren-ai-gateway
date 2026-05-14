package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	balanceAlertNotifiedPrefix     = "balance_alert:notified:v2:"
	balanceAlertDispatchPrefix     = "balance_alert:dispatch:v2:"
	balanceAlertConfigPrefix       = "balance_alert:config:"
	balanceAlertGlobalEnabledKey   = "balance_alert:global_enabled"
	balanceAlertGlobalThresholdKey = "balance_alert:global_threshold"

	balanceAlertConfigTTL      = 10 * time.Minute
	balanceAlertGlobalCacheTTL = 5 * time.Minute
	balanceAlertDispatchTTL    = 15 * time.Minute
)

type balanceAlertCache struct {
	rdb *redis.Client
}

// NewBalanceAlertCache creates a new BalanceAlertCache backed by Redis.
func NewBalanceAlertCache(rdb *redis.Client) service.BalanceAlertCache {
	return &balanceAlertCache{rdb: rdb}
}

func balanceAlertNotifiedKey(userID int64) string {
	return balanceAlertNotifiedPrefix + strconv.FormatInt(userID, 10)
}

func balanceAlertConfigKey(userID int64) string {
	return balanceAlertConfigPrefix + strconv.FormatInt(userID, 10)
}

func balanceAlertDispatchKey(userID int64) string {
	return balanceAlertDispatchPrefix + strconv.FormatInt(userID, 10)
}

func (c *balanceAlertCache) IsNotified(ctx context.Context, userID int64) (bool, error) {
	exists, err := c.rdb.Exists(ctx, balanceAlertNotifiedKey(userID)).Result()
	if err != nil {
		return false, fmt.Errorf("balance alert: check notified: %w", err)
	}
	return exists > 0, nil
}

func (c *balanceAlertCache) SetNotified(ctx context.Context, userID int64) error {
	return c.rdb.Set(ctx, balanceAlertNotifiedKey(userID), "1", 0).Err()
}

func (c *balanceAlertCache) ClearNotified(ctx context.Context, userID int64) error {
	return c.rdb.Del(ctx, balanceAlertNotifiedKey(userID)).Err()
}

func (c *balanceAlertCache) AcquireDispatchLock(ctx context.Context, userID int64) (bool, error) {
	ok, err := c.rdb.SetNX(ctx, balanceAlertDispatchKey(userID), "1", balanceAlertDispatchTTL).Result()
	if err != nil {
		return false, fmt.Errorf("balance alert: acquire dispatch lock: %w", err)
	}
	return ok, nil
}

func (c *balanceAlertCache) HasDispatchLock(ctx context.Context, userID int64) (bool, error) {
	exists, err := c.rdb.Exists(ctx, balanceAlertDispatchKey(userID)).Result()
	if err != nil {
		return false, fmt.Errorf("balance alert: check dispatch lock: %w", err)
	}
	return exists > 0, nil
}

func (c *balanceAlertCache) ClearDispatchLock(ctx context.Context, userID int64) error {
	return c.rdb.Del(ctx, balanceAlertDispatchKey(userID)).Err()
}

func (c *balanceAlertCache) GetCachedConfig(ctx context.Context, userID int64) (*service.BalanceAlertConfig, error) {
	val, err := c.rdb.Get(ctx, balanceAlertConfigKey(userID)).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("balance alert: get cached config: %w", err)
	}

	var cfg service.BalanceAlertConfig
	if err := json.Unmarshal(val, &cfg); err != nil {
		return nil, fmt.Errorf("balance alert: unmarshal config: %w", err)
	}
	return &cfg, nil
}

func (c *balanceAlertCache) SetCachedConfig(ctx context.Context, userID int64, cfg *service.BalanceAlertConfig) error {
	val, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("balance alert: marshal config: %w", err)
	}
	return c.rdb.Set(ctx, balanceAlertConfigKey(userID), val, balanceAlertConfigTTL).Err()
}

func (c *balanceAlertCache) ClearCachedConfig(ctx context.Context, userID int64) error {
	return c.rdb.Del(ctx, balanceAlertConfigKey(userID)).Err()
}

func (c *balanceAlertCache) GetGlobalEnabled(ctx context.Context) (*bool, error) {
	val, err := c.rdb.Get(ctx, balanceAlertGlobalEnabledKey).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("balance alert: get global enabled: %w", err)
	}
	enabled := val == "true"
	return &enabled, nil
}

func (c *balanceAlertCache) SetGlobalEnabled(ctx context.Context, enabled bool) error {
	val := "false"
	if enabled {
		val = "true"
	}
	return c.rdb.Set(ctx, balanceAlertGlobalEnabledKey, val, balanceAlertGlobalCacheTTL).Err()
}

func (c *balanceAlertCache) GetGlobalThreshold(ctx context.Context) (*float64, error) {
	val, err := c.rdb.Get(ctx, balanceAlertGlobalThresholdKey).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("balance alert: get global threshold: %w", err)
	}
	threshold, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return nil, fmt.Errorf("balance alert: parse global threshold: %w", err)
	}
	return &threshold, nil
}

func (c *balanceAlertCache) SetGlobalThreshold(ctx context.Context, threshold float64) error {
	val := strconv.FormatFloat(threshold, 'f', 2, 64)
	return c.rdb.Set(ctx, balanceAlertGlobalThresholdKey, val, balanceAlertGlobalCacheTTL).Err()
}

func (c *balanceAlertCache) ClearGlobalCache(ctx context.Context) error {
	return c.rdb.Del(ctx, balanceAlertGlobalEnabledKey, balanceAlertGlobalThresholdKey).Err()
}

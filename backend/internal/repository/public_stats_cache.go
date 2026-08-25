package repository

import (
	"context"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const publicStatsCacheKey = "public:stats:v1"

type publicStatsCache struct {
	rdb       *redis.Client
	keyPrefix string
}

// NewPublicStatsCache 创建公开平台统计缓存（键前缀逻辑与仪表盘缓存一致）。
func NewPublicStatsCache(rdb *redis.Client, cfg *config.Config) service.PublicStatsCache {
	prefix := "sub2api:"
	if cfg != nil {
		prefix = strings.TrimSpace(cfg.Dashboard.KeyPrefix)
	}
	if prefix != "" && !strings.HasSuffix(prefix, ":") {
		prefix += ":"
	}
	return &publicStatsCache{
		rdb:       rdb,
		keyPrefix: prefix,
	}
}

func (c *publicStatsCache) GetPublicStats(ctx context.Context) (string, error) {
	val, err := c.rdb.Get(ctx, c.buildKey()).Result()
	if err != nil {
		if err == redis.Nil {
			return "", service.ErrPublicStatsCacheMiss
		}
		return "", err
	}
	return val, nil
}

func (c *publicStatsCache) SetPublicStats(ctx context.Context, data string, ttl time.Duration) error {
	return c.rdb.Set(ctx, c.buildKey(), data, ttl).Err()
}

func (c *publicStatsCache) buildKey() string {
	if c.keyPrefix == "" {
		return publicStatsCacheKey
	}
	return c.keyPrefix + publicStatsCacheKey
}

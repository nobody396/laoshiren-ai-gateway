package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

type feedbackRateLimitCache struct {
	client *redis.Client
}

func NewFeedbackRateLimitCache(client *redis.Client) service.FeedbackRateLimitCache {
	return &feedbackRateLimitCache{client: client}
}

func (c *feedbackRateLimitCache) CheckCreateLimit(ctx context.Context, userID int64, limit int, window time.Duration) (bool, time.Duration, error) {
	if c == nil || c.client == nil {
		return true, 0, nil
	}

	now := time.Now()
	nowUnix := now.UnixMilli()
	windowStart := now.Add(-window).UnixMilli()
	key := fmt.Sprintf("feedback:create:%d", userID)
	member := strconv.FormatInt(nowUnix, 10)

	// Phase 1: prune expired entries and count remaining
	pipe := c.client.TxPipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))
	countCmd := pipe.ZCard(ctx, key)
	pipe.Expire(ctx, key, window+time.Minute)
	if _, err := pipe.Exec(ctx); err != nil {
		return true, 0, err
	}

	count := int(countCmd.Val())
	if count >= limit {
		// Over limit — do NOT record the rejected request
		oldest, err := c.client.ZRange(ctx, key, 0, 0).Result()
		if err != nil || len(oldest) == 0 {
			return false, window, err
		}
		oldestTs, parseErr := strconv.ParseInt(oldest[0], 10, 64)
		if parseErr != nil {
			return false, window, nil
		}
		retryAfter := time.Duration(oldestTs+window.Milliseconds()-nowUnix) * time.Millisecond
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		return false, retryAfter, nil
	}

	// Phase 2: under limit — record this submission
	c.client.ZAdd(ctx, key, redis.Z{Score: float64(nowUnix), Member: member})
	return true, 0, nil
}

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	refreshTokenKeyPrefix    = "refresh_token:"
	consumedTokenKeyPrefix   = "consumed_refresh_token:"
	userRefreshTokensPrefix  = "user_refresh_tokens:"
	tokenFamilyPrefix        = "token_family:"
	tokenFamilyRevokedPrefix = "token_family_revoked:"
)

// refreshTokenKey generates the Redis key for a refresh token.
func refreshTokenKey(tokenHash string) string {
	return refreshTokenKeyPrefix + tokenHash
}

// consumedRefreshTokenKey generates the Redis key for a consumed refresh token marker.
func consumedRefreshTokenKey(tokenHash string) string {
	return consumedTokenKeyPrefix + tokenHash
}

// userRefreshTokensKey generates the Redis key for user's token set.
func userRefreshTokensKey(userID int64) string {
	return fmt.Sprintf("%s%d", userRefreshTokensPrefix, userID)
}

// tokenFamilyKey generates the Redis key for token family set.
func tokenFamilyKey(familyID string) string {
	return tokenFamilyPrefix + familyID
}

// tokenFamilyRevokedKey generates the Redis key for a revoked token family marker.
func tokenFamilyRevokedKey(familyID string) string {
	return tokenFamilyRevokedPrefix + familyID
}

type refreshTokenCache struct {
	rdb *redis.Client
}

var consumeRefreshTokenScript = redis.NewScript(`
local active = redis.call("GET", KEYS[1])
if active then
	local ttl = redis.call("PTTL", KEYS[1])
	redis.call("DEL", KEYS[1])
	if ttl > 0 then
		redis.call("SET", KEYS[2], active, "PX", ttl)
	end
	return {1, active}
end

local consumed = redis.call("GET", KEYS[2])
if consumed then
	return {2, consumed}
end

return {0, ""}
`)

var revokeTokenFamilyScript = redis.NewScript(`
local hashes = redis.call("SMEMBERS", KEYS[1])
local ttl = redis.call("PTTL", KEYS[1])
if ttl > 0 then
	redis.call("SET", KEYS[2], "1", "PX", ttl)
end

for _, hash in ipairs(hashes) do
	redis.call("DEL", ARGV[1] .. hash)
end
redis.call("DEL", KEYS[1])

return #hashes
`)

var addToFamilyTokenSetScript = redis.NewScript(`
if redis.call("EXISTS", KEYS[2]) == 1 then
	return 0
end

redis.call("SADD", KEYS[1], ARGV[1])
redis.call("PEXPIRE", KEYS[1], ARGV[2])
return 1
`)

// NewRefreshTokenCache creates a new RefreshTokenCache implementation.
func NewRefreshTokenCache(rdb *redis.Client) service.RefreshTokenCache {
	return &refreshTokenCache{rdb: rdb}
}

func (c *refreshTokenCache) StoreRefreshToken(ctx context.Context, tokenHash string, data *service.RefreshTokenData, ttl time.Duration) error {
	key := refreshTokenKey(tokenHash)
	val, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal refresh token data: %w", err)
	}
	return c.rdb.Set(ctx, key, val, ttl).Err()
}

func (c *refreshTokenCache) GetRefreshToken(ctx context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	key := refreshTokenKey(tokenHash)
	val, err := c.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, service.ErrRefreshTokenNotFound
		}
		return nil, err
	}
	var data service.RefreshTokenData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("unmarshal refresh token data: %w", err)
	}
	return &data, nil
}

func (c *refreshTokenCache) DeleteRefreshToken(ctx context.Context, tokenHash string) error {
	key := refreshTokenKey(tokenHash)
	consumedKey := consumedRefreshTokenKey(tokenHash)
	return c.rdb.Del(ctx, key, consumedKey).Err()
}

func (c *refreshTokenCache) ConsumeRefreshToken(ctx context.Context, tokenHash string) (*service.RefreshTokenData, error) {
	activeKey := refreshTokenKey(tokenHash)
	consumedKey := consumedRefreshTokenKey(tokenHash)

	result, err := consumeRefreshTokenScript.Run(ctx, c.rdb, []string{activeKey, consumedKey}).Result()
	if err != nil {
		return nil, err
	}

	items, ok := result.([]any)
	if !ok || len(items) != 2 {
		return nil, fmt.Errorf("unexpected consume refresh token result: %v", result)
	}

	status, ok := items[0].(int64)
	if !ok {
		return nil, fmt.Errorf("unexpected consume refresh token status: %v", items[0])
	}

	payload, ok := items[1].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected consume refresh token payload: %v", items[1])
	}

	switch status {
	case 0:
		return nil, service.ErrRefreshTokenNotFound
	case 1, 2:
		var data service.RefreshTokenData
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			return nil, fmt.Errorf("unmarshal refresh token data: %w", err)
		}
		if status == 2 {
			return &data, service.ErrRefreshTokenReused
		}
		return &data, nil
	default:
		return nil, fmt.Errorf("unexpected consume refresh token status: %d", status)
	}
}

func (c *refreshTokenCache) DeleteUserRefreshTokens(ctx context.Context, userID int64) error {
	// Get all token hashes for this user
	tokenHashes, err := c.GetUserTokenHashes(ctx, userID)
	if err != nil && err != redis.Nil {
		return fmt.Errorf("get user token hashes: %w", err)
	}

	if len(tokenHashes) == 0 {
		return nil
	}

	// Build keys to delete
	keys := make([]string, 0, len(tokenHashes)+1)
	for _, hash := range tokenHashes {
		keys = append(keys, refreshTokenKey(hash))
		keys = append(keys, consumedRefreshTokenKey(hash))
	}
	keys = append(keys, userRefreshTokensKey(userID))

	// Delete all keys in a pipeline
	pipe := c.rdb.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) DeleteTokenFamily(ctx context.Context, familyID string) error {
	return revokeTokenFamilyScript.Run(ctx, c.rdb, []string{
		tokenFamilyKey(familyID),
		tokenFamilyRevokedKey(familyID),
	}, refreshTokenKeyPrefix).Err()
}

func (c *refreshTokenCache) AddToUserTokenSet(ctx context.Context, userID int64, tokenHash string, ttl time.Duration) error {
	key := userRefreshTokensKey(userID)
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, key, tokenHash)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *refreshTokenCache) AddToFamilyTokenSet(ctx context.Context, familyID string, tokenHash string, ttl time.Duration) error {
	result, err := addToFamilyTokenSetScript.Run(ctx, c.rdb, []string{
		tokenFamilyKey(familyID),
		tokenFamilyRevokedKey(familyID),
	}, tokenHash, ttl.Milliseconds()).Int()
	if err != nil {
		return err
	}
	if result == 0 {
		return service.ErrRefreshTokenReused
	}
	return nil
}

func (c *refreshTokenCache) GetUserTokenHashes(ctx context.Context, userID int64) ([]string, error) {
	key := userRefreshTokensKey(userID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *refreshTokenCache) GetFamilyTokenHashes(ctx context.Context, familyID string) ([]string, error) {
	key := tokenFamilyKey(familyID)
	return c.rdb.SMembers(ctx, key).Result()
}

func (c *refreshTokenCache) IsTokenInFamily(ctx context.Context, familyID string, tokenHash string) (bool, error) {
	key := tokenFamilyKey(familyID)
	return c.rdb.SIsMember(ctx, key, tokenHash).Result()
}

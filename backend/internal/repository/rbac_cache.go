package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/redis/go-redis/v9"
)

const (
	// rbac 缓存 key 前缀
	rbacPermKeyPrefix = "rbac:perms:"
	rbacMenuKeyPrefix = "rbac:menu:"
	// rbac 缓存 TTL
	rbacCacheTTL = 30 * time.Minute
	// SCAN 每批数量
	rbacScanCount = 100
)

// rbacCache 实现 service.RBACCache 接口,
// 使用 Redis 缓存 RBAC 相关数据 (用户权限 key, 菜单树).
type rbacCache struct {
	rdb *redis.Client
}

// NewRBACCache 创建 RBAC 缓存实例.
func NewRBACCache(rdb *redis.Client) service.RBACCache {
	return &rbacCache{rdb: rdb}
}

// GetUserPermissionKeys 从 Redis SET `rbac:perms:{userID}` 获取用户权限 key 列表.
// 返回语义:
//   - (非空切片, nil): 缓存命中, 有权限
//   - ([]string{}, nil): 缓存命中, 用户显式无权限 (存在 `:empty` 占位键)
//   - (nil, nil): 缓存未命中, 调用方需要回源查询
//   - (nil, err): Redis 访问失败
func (c *rbacCache) GetUserPermissionKeys(ctx context.Context, userID int64) ([]string, error) {
	key := fmt.Sprintf("%s%d", rbacPermKeyPrefix, userID)
	emptyKey := key + ":empty"

	// 先检查 SET 主 key 是否存在 (超级管理员会写入 `*`, 普通用户会写入具体权限列表)
	exists, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if exists > 0 {
		val, err := c.rdb.SMembers(ctx, key).Result()
		if err != nil {
			return nil, err
		}
		return val, nil
	}

	// 再检查空权限占位键, 避免穿透
	emptyExists, err := c.rdb.Exists(ctx, emptyKey).Result()
	if err != nil {
		return nil, err
	}
	if emptyExists > 0 {
		return []string{}, nil
	}

	// 两者都不存在, 视为 cache miss, 由上层回源
	return nil, nil
}

// SetUserPermissionKeys 将用户权限 key 列表写入 Redis SET `rbac:perms:{userID}`, TTL 30 分钟.
func (c *rbacCache) SetUserPermissionKeys(ctx context.Context, userID int64, keys []string) error {
	key := fmt.Sprintf("%s%d", rbacPermKeyPrefix, userID)

	if len(keys) == 0 {
		// 空权限也要设置, 防止缓存穿透
		pipe := c.rdb.Pipeline()
		pipe.Del(ctx, key)
		pipe.Set(ctx, key+":empty", "1", rbacCacheTTL)
		_, err := pipe.Exec(ctx)
		return err
	}

	pipe := c.rdb.Pipeline()
	pipe.Del(ctx, key)
	pipe.SAdd(ctx, key, stringSliceToInterfaceSlice(keys)...)
	pipe.Expire(ctx, key, rbacCacheTTL)
	_, err := pipe.Exec(ctx)
	return err
}

// InvalidateUserPermissions 清除指定用户的所有 RBAC 缓存
// (包括 `rbac:perms:{userID}` 和 `rbac:menu:{userID}`).
func (c *rbacCache) InvalidateUserPermissions(ctx context.Context, userID int64) error {
	keys := []string{
		fmt.Sprintf("%s%d", rbacPermKeyPrefix, userID),
		fmt.Sprintf("%s%d", rbacMenuKeyPrefix, userID),
		fmt.Sprintf("%s%d:empty", rbacPermKeyPrefix, userID),
	}
	return c.rdb.Del(ctx, keys...).Err()
}

// InvalidateAllPermissions 使用 SCAN 命令扫描并删除所有 `rbac:*` 缓存 key.
// 不使用 KEYS 命令以避免阻塞 Redis.
func (c *rbacCache) InvalidateAllPermissions(ctx context.Context) error {
	var cursor uint64
	for {
		var keys []string
		var err error
		keys, cursor, err = c.rdb.Scan(ctx, cursor, "rbac:*", int64(rbacScanCount)).Result()
		if err != nil {
			return fmt.Errorf("scan rbac keys: %w", err)
		}

		if len(keys) > 0 {
			if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("delete rbac keys: %w", err)
			}
		}

		if cursor == 0 {
			break
		}
	}
	return nil
}

// GetUserMenuTree 从 Redis STRING `rbac:menu:{userID}` 获取用户的菜单树 JSON.
func (c *rbacCache) GetUserMenuTree(ctx context.Context, userID int64) ([]byte, error) {
	key := fmt.Sprintf("%s%d", rbacMenuKeyPrefix, userID)
	val, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

// SetUserMenuTree 将用户的菜单树 JSON 写入 Redis STRING `rbac:menu:{userID}`, TTL 30 分钟.
func (c *rbacCache) SetUserMenuTree(ctx context.Context, userID int64, data []byte) error {
	key := fmt.Sprintf("%s%d", rbacMenuKeyPrefix, userID)
	return c.rdb.Set(ctx, key, data, rbacCacheTTL).Err()
}

// stringSliceToInterfaceSlice 将 []string 转为 []any, 用于 Redis SAdd.
func stringSliceToInterfaceSlice(s []string) []any {
	result := make([]any, len(s))
	for i, v := range s {
		result[i] = v
	}
	return result
}

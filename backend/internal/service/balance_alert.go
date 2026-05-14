package service

import "context"

// BalanceAlertConfig 用户余额预警配置 (缓存 & 传输用)
type BalanceAlertConfig struct {
	Enabled   bool    `json:"enabled"`
	Threshold float64 `json:"threshold"` // 0 = 使用全局默认
	Email     string  `json:"email"`     // "" = 使用登录邮箱
}

// BalanceAlertCache 余额预警 Redis 操作接口 (Repository 层实现)
type BalanceAlertCache interface {
	// 已通知标记
	IsNotified(ctx context.Context, userID int64) (bool, error)
	SetNotified(ctx context.Context, userID int64) error
	ClearNotified(ctx context.Context, userID int64) error
	AcquireDispatchLock(ctx context.Context, userID int64) (bool, error)
	HasDispatchLock(ctx context.Context, userID int64) (bool, error)
	ClearDispatchLock(ctx context.Context, userID int64) error

	// 用户配置缓存
	GetCachedConfig(ctx context.Context, userID int64) (*BalanceAlertConfig, error) // nil = cache miss
	SetCachedConfig(ctx context.Context, userID int64, cfg *BalanceAlertConfig) error
	ClearCachedConfig(ctx context.Context, userID int64) error

	// 全局配置缓存
	GetGlobalEnabled(ctx context.Context) (*bool, error) // nil = cache miss
	SetGlobalEnabled(ctx context.Context, enabled bool) error
	GetGlobalThreshold(ctx context.Context) (*float64, error) // nil = cache miss
	SetGlobalThreshold(ctx context.Context, threshold float64) error
	ClearGlobalCache(ctx context.Context) error
}

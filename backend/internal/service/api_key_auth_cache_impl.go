package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/dgraph-io/ristretto"
)

const apiKeyAuthSnapshotVersion = 15 // v15: preserve ordered multi-group authorization in auth snapshots

type apiKeyAuthCacheConfig struct {
	l1Size        int
	l1TTL         time.Duration
	l2TTL         time.Duration
	negativeTTL   time.Duration
	jitterPercent int
	singleflight  bool
}

func newAPIKeyAuthCacheConfig(cfg *config.Config) apiKeyAuthCacheConfig {
	if cfg == nil {
		return apiKeyAuthCacheConfig{}
	}
	auth := cfg.APIKeyAuth
	return apiKeyAuthCacheConfig{
		l1Size:        auth.L1Size,
		l1TTL:         time.Duration(auth.L1TTLSeconds) * time.Second,
		l2TTL:         time.Duration(auth.L2TTLSeconds) * time.Second,
		negativeTTL:   time.Duration(auth.NegativeTTLSeconds) * time.Second,
		jitterPercent: auth.JitterPercent,
		singleflight:  auth.Singleflight,
	}
}

func (c apiKeyAuthCacheConfig) l1Enabled() bool {
	return c.l1Size > 0 && c.l1TTL > 0
}

func (c apiKeyAuthCacheConfig) l2Enabled() bool {
	return c.l2TTL > 0
}

func (c apiKeyAuthCacheConfig) negativeEnabled() bool {
	return c.negativeTTL > 0
}

// jitterTTL 为缓存 TTL 添加抖动，避免多个请求在同一时刻同时过期触发集中回源。
// 这里直接使用 rand/v2 的顶层函数：并发安全，无需全局互斥锁。
func (c apiKeyAuthCacheConfig) jitterTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return ttl
	}
	if c.jitterPercent <= 0 {
		return ttl
	}
	percent := c.jitterPercent
	if percent > 100 {
		percent = 100
	}
	delta := float64(percent) / 100
	randVal := rand.Float64()
	factor := 1 - delta + randVal*(2*delta)
	if factor <= 0 {
		return ttl
	}
	return time.Duration(float64(ttl) * factor)
}

func (s *APIKeyService) initAuthCache(cfg *config.Config) {
	s.authCfg = newAPIKeyAuthCacheConfig(cfg)
	if !s.authCfg.l1Enabled() {
		return
	}
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: int64(s.authCfg.l1Size) * 10,
		MaxCost:     int64(s.authCfg.l1Size),
		BufferItems: 64,
	})
	if err != nil {
		return
	}
	s.authCacheL1 = cache
}

// StartAuthCacheInvalidationSubscriber starts the Pub/Sub subscriber for L1 cache invalidation.
// This should be called after the service is fully initialized.
func (s *APIKeyService) StartAuthCacheInvalidationSubscriber(ctx context.Context) {
	if s.cache == nil || s.authCacheL1 == nil {
		return
	}
	if err := s.cache.SubscribeAuthCacheInvalidation(ctx, func(cacheKey string) {
		s.authCacheL1.Del(cacheKey)
	}); err != nil {
		// Log but don't fail - L1 cache will still work, just without cross-instance invalidation
		println("[Service] Warning: failed to start auth cache invalidation subscriber:", err.Error())
	}
}

func (s *APIKeyService) authCacheKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func (s *APIKeyService) getAuthCacheEntry(ctx context.Context, cacheKey string) (*APIKeyAuthCacheEntry, bool) {
	if s.authCacheL1 != nil {
		if val, ok := s.authCacheL1.Get(cacheKey); ok {
			if entry, ok := val.(*APIKeyAuthCacheEntry); ok {
				return entry, true
			}
		}
	}
	if s.cache == nil || !s.authCfg.l2Enabled() {
		return nil, false
	}
	entry, err := s.cache.GetAuthCache(ctx, cacheKey)
	if err != nil {
		return nil, false
	}
	s.setAuthCacheL1(cacheKey, entry)
	return entry, true
}

func (s *APIKeyService) setAuthCacheL1(cacheKey string, entry *APIKeyAuthCacheEntry) {
	if s.authCacheL1 == nil || entry == nil {
		return
	}
	ttl := s.authCfg.l1TTL
	if entry.NotFound && s.authCfg.negativeTTL > 0 && s.authCfg.negativeTTL < ttl {
		ttl = s.authCfg.negativeTTL
	}
	ttl = s.authCfg.jitterTTL(ttl)
	_ = s.authCacheL1.SetWithTTL(cacheKey, entry, 1, ttl)
}

func (s *APIKeyService) setAuthCacheEntry(ctx context.Context, cacheKey string, entry *APIKeyAuthCacheEntry, ttl time.Duration) {
	if entry == nil {
		return
	}
	s.setAuthCacheL1(cacheKey, entry)
	if s.cache == nil || !s.authCfg.l2Enabled() {
		return
	}
	_ = s.cache.SetAuthCache(ctx, cacheKey, entry, s.authCfg.jitterTTL(ttl))
}

func (s *APIKeyService) deleteAuthCache(ctx context.Context, cacheKey string) {
	if s.authCacheL1 != nil {
		s.authCacheL1.Del(cacheKey)
	}
	if s.cache == nil {
		return
	}
	_ = s.cache.DeleteAuthCache(ctx, cacheKey)
	// Publish invalidation message to other instances
	_ = s.cache.PublishAuthCacheInvalidation(ctx, cacheKey)
}

func (s *APIKeyService) loadAuthCacheEntry(ctx context.Context, key, cacheKey string) (*APIKeyAuthCacheEntry, error) {
	apiKey, err := s.apiKeyRepo.GetByKeyForAuth(ctx, key)
	if err != nil {
		if errors.Is(err, ErrAPIKeyNotFound) {
			entry := &APIKeyAuthCacheEntry{NotFound: true}
			if s.authCfg.negativeEnabled() {
				s.setAuthCacheEntry(ctx, cacheKey, entry, s.authCfg.negativeTTL)
			}
			return entry, nil
		}
		return nil, fmt.Errorf("get api key: %w", err)
	}
	apiKey.Key = key
	apiKey, err = s.hydrateTeamAPIKey(ctx, apiKey, nil)
	if err != nil {
		return nil, err
	}
	snapshot := s.snapshotFromAPIKey(apiKey)
	if snapshot == nil {
		return nil, fmt.Errorf("get api key: %w", ErrAPIKeyNotFound)
	}
	entry := &APIKeyAuthCacheEntry{Snapshot: snapshot}
	if apiKey.TeamID != nil {
		// 成员限额、成员关系和 Owner 会高频变化；首版每次实时解析，确保立即阻断。
		return entry, nil
	}
	s.setAuthCacheEntry(ctx, cacheKey, entry, s.authCfg.l2TTL)
	return entry, nil
}

func (s *APIKeyService) hydrateTeamAPIKey(ctx context.Context, apiKey *APIKey, err error) (*APIKey, error) {
	if err != nil || apiKey == nil || apiKey.TeamID == nil {
		return apiKey, err
	}
	// 已禁用 Key 先由中间件返回稳定 401；离队后的上下文缺失不能把它变成 500。
	if !apiKey.IsActive() && apiKey.Status != StatusAPIKeyExpired && apiKey.Status != StatusAPIKeyQuotaExhausted {
		return apiKey, nil
	}
	if s.cfg != nil && !s.cfg.Team.Enabled {
		return nil, ErrTeamFeatureDisabled
	}
	if s.teamRepo == nil {
		return nil, ErrTeamFeatureDisabled
	}
	teamCtx, err := s.teamRepo.GetContextByUserID(ctx, apiKey.UserID)
	if err != nil {
		if errors.Is(err, ErrTeamNotFound) {
			return nil, ErrTeamMembershipRequired
		}
		return nil, err
	}
	if teamCtx == nil || teamCtx.Team == nil || teamCtx.Membership == nil || teamCtx.Owner == nil || teamCtx.Team.ID != *apiKey.TeamID {
		return nil, ErrTeamMembershipRequired
	}
	if teamCtx.Membership.JoinedAt.After(apiKey.CreatedAt) {
		return nil, ErrTeamMembershipRequired
	}
	actor, err := s.userRepo.GetByID(ctx, apiKey.UserID)
	if err != nil {
		return nil, err
	}
	owner, err := s.userRepo.GetByID(ctx, teamCtx.Owner.UserID)
	if err != nil {
		return nil, err
	}
	apiKey.ActorUser = actor
	apiKey.User = owner
	apiKey.Team = teamCtx.Team
	apiKey.TeamMembership = teamCtx.Membership
	return apiKey, nil
}

func (s *APIKeyService) applyAuthCacheEntry(key string, entry *APIKeyAuthCacheEntry) (*APIKey, bool, error) {
	if entry == nil {
		return nil, false, nil
	}
	if entry.NotFound {
		return nil, true, ErrAPIKeyNotFound
	}
	if entry.Snapshot == nil {
		return nil, false, nil
	}
	if entry.Snapshot.Version != apiKeyAuthSnapshotVersion {
		return nil, false, nil
	}
	return s.snapshotToAPIKey(key, entry.Snapshot), true, nil
}

func (s *APIKeyService) snapshotFromAPIKey(apiKey *APIKey) *APIKeyAuthSnapshot {
	if apiKey == nil || apiKey.User == nil {
		return nil
	}
	snapshot := &APIKeyAuthSnapshot{
		Version:           apiKeyAuthSnapshotVersion,
		APIKeyID:          apiKey.ID,
		UserID:            apiKey.UserID,
		TeamID:            apiKey.TeamID,
		TeamOwnerDisabled: apiKey.TeamOwnerDisabled,
		GroupID:           apiKey.GroupID,
		GroupIDs:          append([]int64(nil), apiKey.GroupIDs...),
		Status:            apiKey.Status,
		IPWhitelist:       apiKey.IPWhitelist,
		IPBlacklist:       apiKey.IPBlacklist,
		Quota:             apiKey.Quota,
		QuotaUsed:         apiKey.QuotaUsed,
		ExpiresAt:         apiKey.ExpiresAt,
		RateLimit5h:       apiKey.RateLimit5h,
		RateLimit1d:       apiKey.RateLimit1d,
		RateLimit7d:       apiKey.RateLimit7d,
		CreatedAt:         apiKey.CreatedAt,
		User: APIKeyAuthUserSnapshot{
			ID:          apiKey.User.ID,
			Status:      apiKey.User.Status,
			Role:        apiKey.User.Role,
			Balance:     apiKey.User.Balance,
			Concurrency: apiKey.User.Concurrency,
		},
	}
	if apiKey.ActorUser != nil {
		snapshot.ActorUser = &APIKeyAuthUserSnapshot{
			ID: apiKey.ActorUser.ID, Status: apiKey.ActorUser.Status, Role: apiKey.ActorUser.Role,
			Balance: apiKey.ActorUser.Balance, Concurrency: apiKey.ActorUser.Concurrency,
		}
	}
	if apiKey.Team != nil {
		teamCopy := *apiKey.Team
		snapshot.Team = &teamCopy
		snapshot.TeamMembership = apiKey.TeamMembership
	}
	if apiKey.Group != nil {
		snapshot.Group = &APIKeyAuthGroupSnapshot{
			ID:                              apiKey.Group.ID,
			Name:                            apiKey.Group.Name,
			Platform:                        apiKey.Group.Platform,
			Status:                          apiKey.Group.Status,
			SubscriptionType:                apiKey.Group.SubscriptionType,
			RateMultiplier:                  apiKey.Group.RateMultiplier,
			DailyLimitUSD:                   apiKey.Group.DailyLimitUSD,
			WeeklyLimitUSD:                  apiKey.Group.WeeklyLimitUSD,
			MonthlyLimitUSD:                 apiKey.Group.MonthlyLimitUSD,
			AllowImageGeneration:            apiKey.Group.AllowImageGeneration,
			ImageRateIndependent:            apiKey.Group.ImageRateIndependent,
			ImageRateMultiplier:             apiKey.Group.ImageRateMultiplier,
			ImagePrice1K:                    apiKey.Group.ImagePrice1K,
			ImagePrice2K:                    apiKey.Group.ImagePrice2K,
			ImagePrice4K:                    apiKey.Group.ImagePrice4K,
			GPTImageCallPrice:               apiKey.Group.GPTImageCallPrice,
			VideoRateIndependent:            apiKey.Group.VideoRateIndependent,
			VideoRateMultiplier:             apiKey.Group.VideoRateMultiplier,
			VideoPrice480P:                  apiKey.Group.VideoPrice480P,
			VideoPrice720P:                  apiKey.Group.VideoPrice720P,
			VideoPrice1080P:                 apiKey.Group.VideoPrice1080P,
			ClaudeCodeOnly:                  apiKey.Group.ClaudeCodeOnly,
			FallbackGroupID:                 apiKey.Group.FallbackGroupID,
			FallbackGroupIDOnInvalidRequest: apiKey.Group.FallbackGroupIDOnInvalidRequest,
			ModelRouting:                    apiKey.Group.ModelRouting,
			ModelRoutingEnabled:             apiKey.Group.ModelRoutingEnabled,
			MCPXMLInject:                    apiKey.Group.MCPXMLInject,
			SupportedModelScopes:            apiKey.Group.SupportedModelScopes,
			AllowMessagesDispatch:           apiKey.Group.AllowMessagesDispatch,
			AllowLive:                       apiKey.Group.AllowLive,
			DefaultMappedModel:              apiKey.Group.DefaultMappedModel,
			MessagesDispatchModelConfig:     apiKey.Group.MessagesDispatchModelConfig,
			MaxReasoningEffort:              apiKey.Group.MaxReasoningEffort,
			ReasoningEffortMappings:         append([]ReasoningEffortMapping(nil), apiKey.Group.ReasoningEffortMappings...),
			UniversalRoutes:                 append([]UniversalRouteConfig(nil), apiKey.Group.UniversalRoutes...),
		}
	}
	return snapshot
}

func (s *APIKeyService) snapshotToAPIKey(key string, snapshot *APIKeyAuthSnapshot) *APIKey {
	if snapshot == nil {
		return nil
	}
	apiKey := &APIKey{
		ID:                snapshot.APIKeyID,
		UserID:            snapshot.UserID,
		TeamID:            snapshot.TeamID,
		TeamOwnerDisabled: snapshot.TeamOwnerDisabled,
		GroupID:           snapshot.GroupID,
		GroupIDs:          append([]int64(nil), snapshot.GroupIDs...),
		Key:               key,
		Status:            snapshot.Status,
		IPWhitelist:       snapshot.IPWhitelist,
		IPBlacklist:       snapshot.IPBlacklist,
		Quota:             snapshot.Quota,
		QuotaUsed:         snapshot.QuotaUsed,
		ExpiresAt:         snapshot.ExpiresAt,
		RateLimit5h:       snapshot.RateLimit5h,
		RateLimit1d:       snapshot.RateLimit1d,
		RateLimit7d:       snapshot.RateLimit7d,
		CreatedAt:         snapshot.CreatedAt,
		User: &User{
			ID:          snapshot.User.ID,
			Status:      snapshot.User.Status,
			Role:        snapshot.User.Role,
			Balance:     snapshot.User.Balance,
			Concurrency: snapshot.User.Concurrency,
		},
	}
	if snapshot.ActorUser != nil {
		apiKey.ActorUser = &User{
			ID: snapshot.ActorUser.ID, Status: snapshot.ActorUser.Status, Role: snapshot.ActorUser.Role,
			Balance: snapshot.ActorUser.Balance, Concurrency: snapshot.ActorUser.Concurrency,
		}
	} else {
		apiKey.ActorUser = apiKey.User
	}
	apiKey.Team = snapshot.Team
	apiKey.TeamMembership = snapshot.TeamMembership
	if snapshot.Group != nil {
		apiKey.Group = &Group{
			ID:                              snapshot.Group.ID,
			Name:                            snapshot.Group.Name,
			Platform:                        snapshot.Group.Platform,
			Status:                          snapshot.Group.Status,
			Hydrated:                        true,
			SubscriptionType:                snapshot.Group.SubscriptionType,
			RateMultiplier:                  snapshot.Group.RateMultiplier,
			DailyLimitUSD:                   snapshot.Group.DailyLimitUSD,
			WeeklyLimitUSD:                  snapshot.Group.WeeklyLimitUSD,
			MonthlyLimitUSD:                 snapshot.Group.MonthlyLimitUSD,
			AllowImageGeneration:            snapshot.Group.AllowImageGeneration,
			ImageRateIndependent:            snapshot.Group.ImageRateIndependent,
			ImageRateMultiplier:             snapshot.Group.ImageRateMultiplier,
			ImagePrice1K:                    snapshot.Group.ImagePrice1K,
			ImagePrice2K:                    snapshot.Group.ImagePrice2K,
			ImagePrice4K:                    snapshot.Group.ImagePrice4K,
			GPTImageCallPrice:               snapshot.Group.GPTImageCallPrice,
			VideoRateIndependent:            snapshot.Group.VideoRateIndependent,
			VideoRateMultiplier:             snapshot.Group.VideoRateMultiplier,
			VideoPrice480P:                  snapshot.Group.VideoPrice480P,
			VideoPrice720P:                  snapshot.Group.VideoPrice720P,
			VideoPrice1080P:                 snapshot.Group.VideoPrice1080P,
			ClaudeCodeOnly:                  snapshot.Group.ClaudeCodeOnly,
			FallbackGroupID:                 snapshot.Group.FallbackGroupID,
			FallbackGroupIDOnInvalidRequest: snapshot.Group.FallbackGroupIDOnInvalidRequest,
			ModelRouting:                    snapshot.Group.ModelRouting,
			ModelRoutingEnabled:             snapshot.Group.ModelRoutingEnabled,
			MCPXMLInject:                    snapshot.Group.MCPXMLInject,
			SupportedModelScopes:            snapshot.Group.SupportedModelScopes,
			AllowMessagesDispatch:           snapshot.Group.AllowMessagesDispatch,
			AllowLive:                       snapshot.Group.AllowLive,
			DefaultMappedModel:              snapshot.Group.DefaultMappedModel,
			MessagesDispatchModelConfig:     snapshot.Group.MessagesDispatchModelConfig,
			MaxReasoningEffort:              snapshot.Group.MaxReasoningEffort,
			ReasoningEffortMappings:         append([]ReasoningEffortMapping(nil), snapshot.Group.ReasoningEffortMappings...),
			UniversalRoutes:                 append([]UniversalRouteConfig(nil), snapshot.Group.UniversalRoutes...),
		}
	}
	s.compileAPIKeyIPRules(apiKey)
	return apiKey
}

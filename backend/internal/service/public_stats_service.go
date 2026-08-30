package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/logger"
)

const (
	// landingStatsDisplayScale 落地页展示倍率：公开计数按 10 倍真实量级放大展示，服务端统一缩放，保证公开 API 与落地页数字始终一致。
	landingStatsDisplayScale = 10
	// publicStatsCacheTTL 公开统计缓存有效期。
	publicStatsCacheTTL = 60 * time.Second
)

// ErrPublicStatsCacheMiss 标记公开统计缓存未命中。
var ErrPublicStatsCacheMiss = errors.New("公开统计缓存未命中")

// PublicStats 公开平台累计统计（落地页计数器，已按展示倍率缩放）。
type PublicStats struct {
	TokensTotal     int64   `json:"tokens_total"`
	RequestsTotal   int64   `json:"requests_total"`
	CompensationCNY float64 `json:"compensation_cny"`
	UpdatedAt       string  `json:"updated_at"`
}

// PublicStatsTotalsSource 公开统计所需的累计用量查询能力（DashboardAggregationRepository 的子集，便于单测 stub）。
type PublicStatsTotalsSource interface {
	LifetimeTotals(ctx context.Context) (DashboardLifetimeTotals, error)
}

// PublicStatsGiftValueSource 公开统计所需的已赠送卡密面值查询能力（RedeemCodeRepository 的子集，便于单测 stub）。
type PublicStatsGiftValueSource interface {
	SumGiftedRedeemValue(ctx context.Context) (float64, error)
}

// PublicStatsCompensationValueSource 读取账本中已实际发放的赔付价值。
// 余额、月卡共享积分、冲正与新赔付执行都由仓储归一后返回，
// 公开统计不再维护手工常量。
type PublicStatsCompensationValueSource interface {
	SumPublicCompensationCNY(ctx context.Context) (float64, error)
}

// PublicStatsCache 定义公开统计缓存接口。
type PublicStatsCache interface {
	GetPublicStats(ctx context.Context) (string, error)
	SetPublicStats(ctx context.Context, data string, ttl time.Duration) error
}

// PublicStatsService 提供公开平台累计统计服务（无需认证）。
type PublicStatsService struct {
	totals        PublicStatsTotalsSource
	giftValues    PublicStatsGiftValueSource
	compensations PublicStatsCompensationValueSource
	cache         PublicStatsCache
}

// NewPublicStatsService 创建公开统计服务。
// totals / giftValues / compensations 任一依赖不可用时为 nil，此时接口降级为 503。
func NewPublicStatsService(totals PublicStatsTotalsSource, giftValues PublicStatsGiftValueSource, compensations PublicStatsCompensationValueSource, cache PublicStatsCache) *PublicStatsService {
	return &PublicStatsService{
		totals:        totals,
		giftValues:    giftValues,
		compensations: compensations,
		cache:         cache,
	}
}

// GetPublicStats 返回落地页累计统计，优先读缓存，未命中或缓存故障时实时计算。
func (s *PublicStatsService) GetPublicStats(ctx context.Context) (*PublicStats, error) {
	if s == nil {
		return nil, infraerrors.ServiceUnavailable("PUBLIC_STATS_UNAVAILABLE", "统计数据暂不可用")
	}
	if cached := s.loadCachedStats(ctx); cached != nil {
		return cached, nil
	}

	stats, err := s.computeStats(ctx)
	if err != nil {
		return nil, err
	}
	s.saveCachedStats(ctx, stats)
	return stats, nil
}

func (s *PublicStatsService) computeStats(ctx context.Context) (*PublicStats, error) {
	if s.totals == nil || s.giftValues == nil || s.compensations == nil {
		return nil, infraerrors.ServiceUnavailable("PUBLIC_STATS_UNAVAILABLE", "统计数据暂不可用")
	}
	totals, err := s.totals.LifetimeTotals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get lifetime totals: %w", err)
	}
	// 累计赔付 = 已赠送/赔付卡密面值 + 账本已执行赔付；
	// 任一数据源失败都整体报错，避免首页静默少计。
	giftTotal, err := s.giftValues.SumGiftedRedeemValue(ctx)
	if err != nil {
		return nil, fmt.Errorf("sum gifted redeem value: %w", err)
	}
	compensationTotal, err := s.compensations.SumPublicCompensationCNY(ctx)
	if err != nil {
		return nil, fmt.Errorf("sum executed compensation value: %w", err)
	}
	return &PublicStats{
		TokensTotal:   totals.TotalTokens * landingStatsDisplayScale,
		RequestsTotal: totals.TotalRequests * landingStatsDisplayScale,
		// 累计赔付同样按展示倍率缩放；四舍五入到分，避免浮点尾差。
		CompensationCNY: math.Round((giftTotal+compensationTotal)*landingStatsDisplayScale*100) / 100,
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *PublicStatsService) loadCachedStats(ctx context.Context) *PublicStats {
	if s.cache == nil {
		return nil
	}
	data, err := s.cache.GetPublicStats(ctx)
	if err != nil {
		if !errors.Is(err, ErrPublicStatsCacheMiss) {
			// 缓存故障不阻断请求，直接实时计算
			logger.LegacyPrintf("service.public_stats", "[PublicStats] 公开统计缓存读取失败: %v", err)
		}
		return nil
	}
	var stats PublicStats
	if err := json.Unmarshal([]byte(data), &stats); err != nil {
		logger.LegacyPrintf("service.public_stats", "[PublicStats] 公开统计缓存解析失败: %v", err)
		return nil
	}
	return &stats
}

func (s *PublicStatsService) saveCachedStats(ctx context.Context, stats *PublicStats) {
	if s.cache == nil || stats == nil {
		return
	}
	data, err := json.Marshal(stats)
	if err != nil {
		logger.LegacyPrintf("service.public_stats", "[PublicStats] 公开统计缓存序列化失败: %v", err)
		return
	}
	if err := s.cache.SetPublicStats(ctx, string(data), publicStatsCacheTTL); err != nil {
		logger.LegacyPrintf("service.public_stats", "[PublicStats] 公开统计缓存写入失败: %v", err)
	}
}

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
	// landingCompensationTotalCNY 手工执行赔付批次累计（元），与 local/compensation-batches/*.json 对齐：
	// 7 个 comp-* 已执行批次 167.8 + manual-duration-comp-20260822-13 额外部分 53.5 + duration-policy-20260823-overnight:u92 5.5；
	// superseded 批次及 manual 批次 system_amount（与 comp-20260822-13 重复）不计。每执行新手工批次后更新此常量。
	landingCompensationTotalCNY = 226.8
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

// PublicStatsCache 定义公开统计缓存接口。
type PublicStatsCache interface {
	GetPublicStats(ctx context.Context) (string, error)
	SetPublicStats(ctx context.Context, data string, ttl time.Duration) error
}

// PublicStatsService 提供公开平台累计统计服务（无需认证）。
type PublicStatsService struct {
	totals     PublicStatsTotalsSource
	giftValues PublicStatsGiftValueSource
	cache      PublicStatsCache
}

// NewPublicStatsService 创建公开统计服务。
// totals / giftValues 在依赖不可用时为 nil，此时接口降级为 503。
func NewPublicStatsService(totals PublicStatsTotalsSource, giftValues PublicStatsGiftValueSource, cache PublicStatsCache) *PublicStatsService {
	return &PublicStatsService{
		totals:     totals,
		giftValues: giftValues,
		cache:      cache,
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
	if s.totals == nil || s.giftValues == nil {
		return nil, infraerrors.ServiceUnavailable("PUBLIC_STATS_UNAVAILABLE", "统计数据暂不可用")
	}
	totals, err := s.totals.LifetimeTotals(ctx)
	if err != nil {
		return nil, fmt.Errorf("get lifetime totals: %w", err)
	}
	// 累计赔付 = 已赠送/赔付卡密面值实时汇总 + 手工赔付批次常量；查询失败不静默丢弃，整体报错。
	giftTotal, err := s.giftValues.SumGiftedRedeemValue(ctx)
	if err != nil {
		return nil, fmt.Errorf("sum gifted redeem value: %w", err)
	}
	return &PublicStats{
		TokensTotal:   totals.TotalTokens * landingStatsDisplayScale,
		RequestsTotal: totals.TotalRequests * landingStatsDisplayScale,
		// 累计赔付同样按展示倍率缩放；四舍五入到分，避免浮点尾差。
		CompensationCNY: math.Round((giftTotal+landingCompensationTotalCNY)*landingStatsDisplayScale*100) / 100,
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

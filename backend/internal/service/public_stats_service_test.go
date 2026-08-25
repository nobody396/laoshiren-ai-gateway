package service

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

// --- stubs ---

type publicStatsTotalsStub struct {
	totals DashboardLifetimeTotals
	err    error
	calls  int
}

func (s *publicStatsTotalsStub) LifetimeTotals(_ context.Context) (DashboardLifetimeTotals, error) {
	s.calls++
	return s.totals, s.err
}

type publicStatsGiftValueStub struct {
	value float64
	err   error
	calls int
}

func (s *publicStatsGiftValueStub) SumGiftedRedeemValue(_ context.Context) (float64, error) {
	s.calls++
	return s.value, s.err
}

type publicStatsCacheStub struct {
	data     string
	getErr   error
	setErr   error
	setCalls int
	lastTTL  time.Duration
}

func (c *publicStatsCacheStub) GetPublicStats(_ context.Context) (string, error) {
	if c.getErr != nil {
		return "", c.getErr
	}
	if c.data == "" {
		return "", ErrPublicStatsCacheMiss
	}
	return c.data, nil
}

func (c *publicStatsCacheStub) SetPublicStats(_ context.Context, data string, ttl time.Duration) error {
	c.setCalls++
	if c.setErr == nil {
		c.data = data
	}
	c.lastTTL = ttl
	return c.setErr
}

// --- tests ---

func TestPublicStatsScaledTotalsAndCache(t *testing.T) {
	totals := &publicStatsTotalsStub{totals: DashboardLifetimeTotals{
		TotalRequests: 123456,
		TotalTokens:   219638600,
	}}
	gifts := &publicStatsGiftValueStub{value: 1557}
	cache := &publicStatsCacheStub{}
	svc := NewPublicStatsService(totals, gifts, cache)

	stats, err := svc.GetPublicStats(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TokensTotal != 2196386000 {
		t.Fatalf("expected scaled tokens_total 2196386000, got %d", stats.TokensTotal)
	}
	if stats.RequestsTotal != 1234560 {
		t.Fatalf("expected scaled requests_total 1234560, got %d", stats.RequestsTotal)
	}
	// (1557 赠送卡密 + 226.8 手工批次) × 10 = 17838
	if stats.CompensationCNY != 17838.0 {
		t.Fatalf("expected scaled compensation_cny 17838.0, got %v", stats.CompensationCNY)
	}
	if _, err := time.Parse(time.RFC3339, stats.UpdatedAt); err != nil {
		t.Fatalf("updated_at should be RFC3339, got %q", stats.UpdatedAt)
	}
	if gifts.calls != 1 {
		t.Fatalf("expected gift value source called once, got %d", gifts.calls)
	}
	if cache.setCalls != 1 {
		t.Fatalf("expected cache write once, got %d", cache.setCalls)
	}
	if cache.lastTTL != publicStatsCacheTTL {
		t.Fatalf("expected cache TTL %v, got %v", publicStatsCacheTTL, cache.lastTTL)
	}

	// 第二次调用命中缓存，不再访问仓储
	if _, err := svc.GetPublicStats(context.Background()); err != nil {
		t.Fatalf("second call: %v", err)
	}
	if totals.calls != 1 || gifts.calls != 1 {
		t.Fatalf("expected sources called once (cached), got totals=%d gifts=%d", totals.calls, gifts.calls)
	}
}

func TestPublicStatsNilRepoDegrades(t *testing.T) {
	cases := map[string]*PublicStatsService{
		"nil totals":      NewPublicStatsService(nil, &publicStatsGiftValueStub{}, nil),
		"nil gift source": NewPublicStatsService(&publicStatsTotalsStub{}, nil, nil),
		"all nil":         NewPublicStatsService(nil, nil, nil),
	}
	for name, svc := range cases {
		_, err := svc.GetPublicStats(context.Background())
		if err == nil {
			t.Fatalf("%s: expected error when a source is nil", name)
		}
		if !infraerrors.IsServiceUnavailable(err) {
			t.Fatalf("%s: expected 503 service unavailable, got %v", name, err)
		}
		if infraerrors.Code(err) != http.StatusServiceUnavailable {
			t.Fatalf("%s: expected status 503, got %d", name, infraerrors.Code(err))
		}
	}
}

func TestPublicStatsCacheErrorFallsBackToCompute(t *testing.T) {
	totals := &publicStatsTotalsStub{totals: DashboardLifetimeTotals{TotalRequests: 1, TotalTokens: 2}}
	gifts := &publicStatsGiftValueStub{value: 3}
	cache := &publicStatsCacheStub{
		getErr: errors.New("redis down"),
		setErr: errors.New("redis down"),
	}
	svc := NewPublicStatsService(totals, gifts, cache)

	stats, err := svc.GetPublicStats(context.Background())
	if err != nil {
		t.Fatalf("cache failure must not fail the request: %v", err)
	}
	if stats.TokensTotal != 20 || stats.RequestsTotal != 10 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if totals.calls != 1 || gifts.calls != 1 {
		t.Fatalf("expected sources called once, got totals=%d gifts=%d", totals.calls, gifts.calls)
	}
}

func TestPublicStatsRepoErrorPropagates(t *testing.T) {
	totals := &publicStatsTotalsStub{err: errors.New("db down")}
	svc := NewPublicStatsService(totals, &publicStatsGiftValueStub{}, nil)
	if _, err := svc.GetPublicStats(context.Background()); err == nil {
		t.Fatal("expected error when totals repo fails")
	}
}

func TestPublicStatsGiftValueErrorPropagates(t *testing.T) {
	// 赠送卡密汇总失败必须整体报错，不能静默丢掉赠送部分按常量兜底。
	totals := &publicStatsTotalsStub{totals: DashboardLifetimeTotals{TotalRequests: 1, TotalTokens: 2}}
	gifts := &publicStatsGiftValueStub{err: errors.New("db down")}
	svc := NewPublicStatsService(totals, gifts, nil)
	if _, err := svc.GetPublicStats(context.Background()); err == nil {
		t.Fatal("expected error when gift value query fails")
	}
}

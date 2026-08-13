//go:build integration

package repository

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OpenAIRouteBudgetCacheSuite struct {
	IntegrationRedisSuite
	cache   service.OpenAIRouteBudgetStore
	windows []service.OpenAIRouteBudgetWindowConfig
	route   service.OpenAIRouteKey
}

func TestOpenAIRouteBudgetCacheSuite(t *testing.T) {
	suite.Run(t, new(OpenAIRouteBudgetCacheSuite))
}

func (s *OpenAIRouteBudgetCacheSuite) SetupTest() {
	s.IntegrationRedisSuite.SetupTest()
	s.cache = NewOpenAIRouteBudgetCache(s.rdb)
	s.windows = []service.OpenAIRouteBudgetWindowConfig{
		openAIRouteBudgetTestWindow("5m", "2026-08-08T12:00Z", 10*time.Minute, 0.01),
		openAIRouteBudgetTestWindow("1h", "2026-08-08T12:00Z", 2*time.Hour, 0.01),
	}
	s.route = openAIRouteBudgetTestRoute(28)
}

func (s *OpenAIRouteBudgetCacheSuite) TestReserveAcrossWindowsAndSettleAuthoritativeCost() {
	cheap := service.OpenAIRouteBudgetStoreReserveRequest{
		ReservationID:        "cheap-request",
		RouteKey:             s.route,
		RateMultiplier:       0.15,
		EstimatedBaseCostUSD: 1,
		Windows:              s.windows,
		ReservationTTL:       time.Minute,
	}
	reservation, err := s.cache.Reserve(s.ctx, cheap)
	require.NoError(s.T(), err)
	require.True(s.T(), reservation.Allowed)
	require.False(s.T(), reservation.Emergency)
	require.NoError(s.T(), s.cache.Settle(s.ctx, service.OpenAIRouteBudgetStoreSettlement{
		ReservationID:        cheap.ReservationID,
		RouteKey:             s.route,
		ActualBaseCostUSD:    1,
		ActualAccountCostUSD: 0.15,
		Windows:              s.windows,
		AuditTTL:             time.Hour,
	}))

	ledgers, err := s.cache.GetLedgers(s.ctx, s.windows)
	require.NoError(s.T(), err)
	for _, ledger := range ledgers {
		require.InDelta(s.T(), 0.005, ledger.CreditUSD, 1e-9)
		require.InDelta(s.T(), 1, ledger.EquivalentCostUSD, 1e-9)
		require.InDelta(s.T(), 0.15, ledger.ActualAccountCostUSD, 1e-9)
	}

	expensive := service.OpenAIRouteBudgetStoreReserveRequest{
		ReservationID:        "expensive-request",
		RouteKey:             s.route,
		RateMultiplier:       0.20,
		EstimatedBaseCostUSD: 0.1,
		Windows:              s.windows,
		ReservationTTL:       time.Minute,
	}
	reservation, err = s.cache.Reserve(s.ctx, expensive)
	require.NoError(s.T(), err)
	require.True(s.T(), reservation.Allowed)
	require.False(s.T(), reservation.Emergency)
	require.NoError(s.T(), s.cache.Settle(s.ctx, service.OpenAIRouteBudgetStoreSettlement{
		ReservationID:        expensive.ReservationID,
		RouteKey:             s.route,
		ActualBaseCostUSD:    0.1,
		ActualAccountCostUSD: 0.02,
		Windows:              s.windows,
		AuditTTL:             time.Hour,
	}))

	ledgers, err = s.cache.GetLedgers(s.ctx, s.windows)
	require.NoError(s.T(), err)
	for _, ledger := range ledgers {
		require.InDelta(s.T(), 0.0005, ledger.CreditUSD, 1e-9)
		require.InDelta(s.T(), 1.1, ledger.EquivalentCostUSD, 1e-9)
		require.InDelta(s.T(), 0.17, ledger.ActualAccountCostUSD, 1e-9)
		average, ok := ledger.AverageMultiplier()
		require.True(s.T(), ok)
		require.InDelta(s.T(), 0.17/1.1, average, 1e-9)
	}

	// Settlement is idempotent and cannot double-count totals.
	require.NoError(s.T(), s.cache.Settle(s.ctx, service.OpenAIRouteBudgetStoreSettlement{
		ReservationID:        expensive.ReservationID,
		RouteKey:             s.route,
		ActualBaseCostUSD:    0.1,
		ActualAccountCostUSD: 0.02,
		Windows:              s.windows,
		AuditTTL:             time.Hour,
	}))
	ledgersAfterRetry, err := s.cache.GetLedgers(s.ctx, s.windows)
	require.NoError(s.T(), err)
	require.Equal(s.T(), ledgers, ledgersAfterRetry)

	err = s.cache.Settle(s.ctx, service.OpenAIRouteBudgetStoreSettlement{
		ReservationID:        expensive.ReservationID,
		RouteKey:             s.route,
		ActualBaseCostUSD:    0.1,
		ActualAccountCostUSD: 0.021,
		Windows:              s.windows,
		AuditTTL:             time.Hour,
	})
	require.ErrorIs(s.T(), err, service.ErrOpenAIRouteReservationConflict)
}

func (s *OpenAIRouteBudgetCacheSuite) TestRejectedWindowLeavesAllWindowsUnchanged() {
	windows := []service.OpenAIRouteBudgetWindowConfig{
		openAIRouteBudgetTestWindow("5m", "2026-08-08T12:00Z", 10*time.Minute, 0.01),
		openAIRouteBudgetTestWindow("1h", "2026-08-08T12:00Z", 2*time.Hour, 0),
	}
	reservation, err := s.cache.Reserve(s.ctx, service.OpenAIRouteBudgetStoreReserveRequest{
		ReservationID:        "atomic-rejection",
		RouteKey:             s.route,
		RateMultiplier:       0.20,
		EstimatedBaseCostUSD: 0.1,
		Windows:              windows,
		ReservationTTL:       time.Minute,
	})
	require.NoError(s.T(), err)
	require.False(s.T(), reservation.Allowed)
	require.Equal(s.T(), 1, reservation.RejectedWindow)

	ledgers, err := s.cache.GetLedgers(s.ctx, windows)
	require.NoError(s.T(), err)
	for _, ledger := range ledgers {
		require.Zero(s.T(), ledger.CreditUSD)
		require.Zero(s.T(), ledger.EquivalentCostUSD)
		require.Zero(s.T(), ledger.ActualAccountCostUSD)
	}

	// A rejected attempt did not create a reservation record with this ID.
	windows[1].EmergencyDebtLimitUSD = 0.01
	reservation, err = s.cache.Reserve(s.ctx, service.OpenAIRouteBudgetStoreReserveRequest{
		ReservationID:        "atomic-rejection",
		RouteKey:             s.route,
		RateMultiplier:       0.20,
		EstimatedBaseCostUSD: 0.1,
		Windows:              windows,
		ReservationTTL:       time.Minute,
	})
	require.NoError(s.T(), err)
	require.True(s.T(), reservation.Allowed)
	require.True(s.T(), reservation.Emergency)
}

func (s *OpenAIRouteBudgetCacheSuite) TestReservationIsIdempotentAndConflictingReuseFails() {
	req := service.OpenAIRouteBudgetStoreReserveRequest{
		ReservationID:        "idempotent-reservation",
		RouteKey:             s.route,
		RateMultiplier:       0.20,
		EstimatedBaseCostUSD: 0.1,
		Windows:              s.windows,
		ReservationTTL:       time.Minute,
	}
	first, err := s.cache.Reserve(s.ctx, req)
	require.NoError(s.T(), err)
	require.True(s.T(), first.Allowed)
	require.True(s.T(), first.Emergency)

	second, err := s.cache.Reserve(s.ctx, req)
	require.NoError(s.T(), err)
	require.Equal(s.T(), first, second)
	ledgers, err := s.cache.GetLedgers(s.ctx, s.windows)
	require.NoError(s.T(), err)
	for _, ledger := range ledgers {
		require.InDelta(s.T(), -0.0045, ledger.CreditUSD, 1e-9)
	}

	req.RouteKey = openAIRouteBudgetTestRoute(29)
	_, err = s.cache.Reserve(s.ctx, req)
	require.ErrorIs(s.T(), err, service.ErrOpenAIRouteReservationConflict)
	req.RouteKey = s.route
	req.EstimatedBaseCostUSD = 0.2
	_, err = s.cache.Reserve(s.ctx, req)
	require.ErrorIs(s.T(), err, service.ErrOpenAIRouteReservationConflict)
}

func (s *OpenAIRouteBudgetCacheSuite) TestCancelRefundsOnceAndCannotCancelSettledReservation() {
	req := service.OpenAIRouteBudgetStoreReserveRequest{
		ReservationID:        "cancel-reservation",
		RouteKey:             s.route,
		RateMultiplier:       0.20,
		EstimatedBaseCostUSD: 0.1,
		Windows:              s.windows,
		ReservationTTL:       time.Minute,
	}
	reservation, err := s.cache.Reserve(s.ctx, req)
	require.NoError(s.T(), err)
	require.True(s.T(), reservation.Allowed)

	cancel := service.OpenAIRouteBudgetStoreSettlement{
		ReservationID: req.ReservationID,
		RouteKey:      s.route,
		Windows:       s.windows,
		AuditTTL:      time.Hour,
	}
	wrongRouteCancel := cancel
	wrongRouteCancel.RouteKey = openAIRouteBudgetTestRoute(29)
	require.ErrorIs(s.T(), s.cache.Cancel(s.ctx, wrongRouteCancel), service.ErrOpenAIRouteReservationConflict)
	require.NoError(s.T(), s.cache.Cancel(s.ctx, cancel))
	require.NoError(s.T(), s.cache.Cancel(s.ctx, cancel))
	ledgers, err := s.cache.GetLedgers(s.ctx, s.windows)
	require.NoError(s.T(), err)
	for _, ledger := range ledgers {
		require.Zero(s.T(), ledger.CreditUSD)
	}

	settledReq := req
	settledReq.ReservationID = "settled-reservation"
	reservation, err = s.cache.Reserve(s.ctx, settledReq)
	require.NoError(s.T(), err)
	require.True(s.T(), reservation.Allowed)
	require.NoError(s.T(), s.cache.Settle(s.ctx, service.OpenAIRouteBudgetStoreSettlement{
		ReservationID:        settledReq.ReservationID,
		RouteKey:             s.route,
		ActualBaseCostUSD:    0.1,
		ActualAccountCostUSD: 0.02,
		Windows:              s.windows,
		AuditTTL:             time.Hour,
	}))
	cancel.ReservationID = settledReq.ReservationID
	require.ErrorIs(s.T(), s.cache.Cancel(s.ctx, cancel), service.ErrOpenAIRouteReservationConflict)
}

func (s *OpenAIRouteBudgetCacheSuite) TestConcurrentReservationsCannotOverspendEmergencyDebt() {
	window := []service.OpenAIRouteBudgetWindowConfig{
		openAIRouteBudgetTestWindow("5m", "2026-08-08T12:00Z", 10*time.Minute, 0.01),
	}

	const workers = 20
	var allowed atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			reservation, err := s.cache.Reserve(s.ctx, service.OpenAIRouteBudgetStoreReserveRequest{
				ReservationID:        fmt.Sprintf("concurrent-%d", worker),
				RouteKey:             s.route,
				RateMultiplier:       0.20,
				EstimatedBaseCostUSD: 0.1,
				Windows:              window,
				ReservationTTL:       time.Minute,
			})
			if err == nil && reservation.Allowed {
				allowed.Add(1)
			}
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(s.T(), err)
	}

	// Each request needs 0.0045 USD extra credit. A 0.01 USD emergency
	// budget can admit only two atomically, regardless of caller concurrency.
	require.Equal(s.T(), int64(2), allowed.Load())
	ledgers, err := s.cache.GetLedgers(s.ctx, window)
	require.NoError(s.T(), err)
	require.InDelta(s.T(), -0.009, ledgers[0].CreditUSD, 1e-9)
}

func (s *OpenAIRouteBudgetCacheSuite) TestHardAverageBlocksOnlyAboveTargetRoutes() {
	window := []service.OpenAIRouteBudgetWindowConfig{
		openAIRouteBudgetTestWindow("5m", "2026-08-08T12:00Z", 10*time.Minute, 1),
	}
	req := service.OpenAIRouteBudgetStoreReserveRequest{
		ReservationID:        "hard-line-seed",
		RouteKey:             s.route,
		RateMultiplier:       0.25,
		EstimatedBaseCostUSD: 1,
		Windows:              window,
		ReservationTTL:       time.Minute,
	}
	reservation, err := s.cache.Reserve(s.ctx, req)
	require.NoError(s.T(), err)
	require.True(s.T(), reservation.Allowed)
	require.NoError(s.T(), s.cache.Settle(s.ctx, service.OpenAIRouteBudgetStoreSettlement{
		ReservationID:        req.ReservationID,
		RouteKey:             s.route,
		ActualBaseCostUSD:    1,
		ActualAccountCostUSD: 0.25,
		Windows:              window,
		AuditTTL:             time.Hour,
	}))

	req.ReservationID = "hard-line-expensive"
	req.RateMultiplier = 0.20
	reservation, err = s.cache.Reserve(s.ctx, req)
	require.NoError(s.T(), err)
	require.False(s.T(), reservation.Allowed)

	req.ReservationID = "hard-line-cheap"
	req.RateMultiplier = 0.15
	reservation, err = s.cache.Reserve(s.ctx, req)
	require.NoError(s.T(), err)
	require.True(s.T(), reservation.Allowed)
}

func (s *OpenAIRouteBudgetCacheSuite) TestSettlementPolicyConflictDoesNotPartiallyMutateWindows() {
	req := service.OpenAIRouteBudgetStoreReserveRequest{
		ReservationID:        "settlement-policy-conflict",
		RouteKey:             s.route,
		RateMultiplier:       0.20,
		EstimatedBaseCostUSD: 0.1,
		Windows:              s.windows,
		ReservationTTL:       time.Minute,
	}
	reservation, err := s.cache.Reserve(s.ctx, req)
	require.NoError(s.T(), err)
	require.True(s.T(), reservation.Allowed)

	prepared, err := prepareOpenAIRouteBudgetWindows(s.windows)
	require.NoError(s.T(), err)
	require.Len(s.T(), prepared, 2)
	before, err := s.rdb.HGetAll(s.ctx, prepared[0].redisKey).Result()
	require.NoError(s.T(), err)
	require.NotEmpty(s.T(), before)

	require.NoError(s.T(), s.rdb.HSet(s.ctx, prepared[1].redisKey, "policy", "corrupted-policy").Err())
	err = s.cache.Settle(s.ctx, service.OpenAIRouteBudgetStoreSettlement{
		ReservationID:        req.ReservationID,
		RouteKey:             s.route,
		ActualBaseCostUSD:    0.1,
		ActualAccountCostUSD: 0.02,
		Windows:              s.windows,
		AuditTTL:             time.Hour,
	})
	require.ErrorIs(s.T(), err, service.ErrOpenAIRouteReservationConflict)

	after, err := s.rdb.HGetAll(s.ctx, prepared[0].redisKey).Result()
	require.NoError(s.T(), err)
	require.Equal(s.T(), before, after, "a later-window conflict must not partially settle an earlier window")
}

func openAIRouteBudgetTestWindow(name, epoch string, ttl time.Duration, emergencyDebtUSD float64) service.OpenAIRouteBudgetWindowConfig {
	return service.OpenAIRouteBudgetWindowConfig{
		Scope: service.OpenAIRouteBudgetScope{
			GroupID:      7,
			Model:        "gpt-5.6-sol",
			RequestClass: service.OpenAIRouteRequestClassText,
			Window:       name,
			Epoch:        epoch,
		},
		TargetAverageMultiplier: 0.155,
		HardAverageMultiplier:   0.18,
		EmergencyDebtLimitUSD:   emergencyDebtUSD,
		MaxCreditUSD:            1,
		TTL:                     ttl,
	}
}

func openAIRouteBudgetTestRoute(accountID int64) service.OpenAIRouteKey {
	return service.OpenAIRouteKey{
		GroupID:       7,
		AccountID:     accountID,
		Model:         "gpt-5.6-sol",
		RequestClass:  service.OpenAIRouteRequestClassText,
		EndpointHash:  "endpoint",
		Transport:     "sse",
		FailureDomain: "test-provider",
	}
}

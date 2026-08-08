//go:build integration

package repository

import (
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type OpenAIRouteHealthCacheSuite struct {
	IntegrationRedisSuite
	cache service.OpenAIRouteHealthStore
	key   service.OpenAIRouteHealthStoreKey
}

func TestOpenAIRouteHealthCacheSuite(t *testing.T) {
	suite.Run(t, new(OpenAIRouteHealthCacheSuite))
}

func (s *OpenAIRouteHealthCacheSuite) SetupTest() {
	s.IntegrationRedisSuite.SetupTest()
	s.cache = NewOpenAIRouteHealthCache(s.rdb, time.Hour, 10*time.Second)
	s.key = service.OpenAIRouteHealthStoreKey{
		Scope:         service.OpenAIRouteHealthScopeRoute,
		GroupID:       7,
		AccountID:     28,
		FailureDomain: "anyroute",
		Model:         "gpt-5.6-sol",
		EndpointHash:  "endpoint",
		Transport:     "sse",
	}
}

func (s *OpenAIRouteHealthCacheSuite) TestMissingStateStartsWarmupAndTransitionsAtomically() {
	state, err := s.cache.Get(s.ctx, s.key)
	require.NoError(s.T(), err)
	require.Equal(s.T(), service.OpenAIRouteCircuitWarmup, state.State)

	start := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	state, err = s.cache.ApplyEvent(s.ctx, s.key, service.OpenAIRouteHealthEvent{
		At:           start,
		FailureClass: service.OpenAIRouteFailureCapacity,
	}, service.DefaultOpenAIRoutePolicy())
	require.NoError(s.T(), err)
	require.Equal(s.T(), service.OpenAIRouteCircuitOpen, state.State)

	state, err = s.cache.Get(s.ctx, s.key)
	require.NoError(s.T(), err)
	require.Equal(s.T(), service.OpenAIRouteCircuitOpen, state.State)
	require.Equal(s.T(), start.Add(5*time.Second), state.OpenUntil)
}

func (s *OpenAIRouteHealthCacheSuite) TestConcurrentFailuresDoNotLoseUpdates() {
	policy := service.DefaultOpenAIRoutePolicy()
	start := time.Now().UTC()
	_, err := s.cache.ApplyEvent(s.ctx, s.key, service.OpenAIRouteHealthEvent{At: start, Success: true, Probe: true}, policy)
	require.NoError(s.T(), err)
	_, err = s.cache.ApplyEvent(s.ctx, s.key, service.OpenAIRouteHealthEvent{At: start.Add(time.Millisecond), Success: true}, policy)
	require.NoError(s.T(), err)

	const workers = 12
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, applyErr := s.cache.ApplyEvent(s.ctx, s.key, service.OpenAIRouteHealthEvent{
				At:           start.Add(time.Duration(idx+2) * time.Millisecond),
				FailureClass: service.OpenAIRouteFailureUpstream5xx,
			}, policy)
			errs <- applyErr
		}(i)
	}
	wg.Wait()
	close(errs)
	for applyErr := range errs {
		require.NoError(s.T(), applyErr)
	}
	state, err := s.cache.Get(s.ctx, s.key)
	require.NoError(s.T(), err)
	require.Equal(s.T(), service.OpenAIRouteCircuitOpen, state.State)
	require.GreaterOrEqual(s.T(), state.EjectionCount, 1)
}

func (s *OpenAIRouteHealthCacheSuite) TestHalfOpenPermitIsSingleOwnerAndIdempotent() {
	ok, err := s.cache.AcquireHalfOpenPermit(s.ctx, s.key, "worker-a")
	require.NoError(s.T(), err)
	require.True(s.T(), ok)
	ok, err = s.cache.AcquireHalfOpenPermit(s.ctx, s.key, "worker-a")
	require.NoError(s.T(), err)
	require.True(s.T(), ok)
	ok, err = s.cache.AcquireHalfOpenPermit(s.ctx, s.key, "worker-b")
	require.NoError(s.T(), err)
	require.False(s.T(), ok)

	require.NoError(s.T(), s.cache.ReleaseHalfOpenPermit(s.ctx, s.key, "worker-b"))
	ok, err = s.cache.AcquireHalfOpenPermit(s.ctx, s.key, "worker-b")
	require.NoError(s.T(), err)
	require.False(s.T(), ok)

	require.NoError(s.T(), s.cache.ReleaseHalfOpenPermit(s.ctx, s.key, "worker-a"))
	ok, err = s.cache.AcquireHalfOpenPermit(s.ctx, s.key, "worker-b")
	require.NoError(s.T(), err)
	require.True(s.T(), ok)
}

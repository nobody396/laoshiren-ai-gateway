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
		RequestClass:  service.OpenAIRouteRequestClassText,
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

func (s *OpenAIRouteHealthCacheSuite) TestBatchReadReturnsRouteAndProviderThroughOneCall() {
	provider := service.OpenAIRouteHealthStoreKey{
		Scope: service.OpenAIRouteHealthScopeProvider, GroupID: s.key.GroupID,
		FailureDomain: s.key.FailureDomain, Model: s.key.Model, RequestClass: s.key.RequestClass,
	}
	start := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	_, err := s.cache.ApplyEvent(s.ctx, s.key, service.OpenAIRouteHealthEvent{
		At: start, FailureClass: service.OpenAIRouteFailureUpstream5xx,
	}, service.DefaultOpenAIRoutePolicy())
	require.NoError(s.T(), err)

	states, err := s.cache.GetBatch(s.ctx, []service.OpenAIRouteHealthStoreKey{s.key, provider, s.key})
	require.NoError(s.T(), err)
	require.Len(s.T(), states, 2)
	require.Equal(s.T(), service.OpenAIRouteCircuitOpen, states[s.key.Fingerprint()].State)
	require.Equal(s.T(), service.OpenAIRouteCircuitWarmup, states[provider.Fingerprint()].State)
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

func (s *OpenAIRouteHealthCacheSuite) TestProviderOpensOnlyAfterDistinctRoutesAndRecoversAfterTheirSuccesses() {
	policy := service.DefaultOpenAIRoutePolicy()
	start := time.Now().UTC()
	first := service.OpenAIRouteKey{
		GroupID: 7, AccountID: 23, FailureDomain: "pomoai", Model: "gpt-5.6-sol",
		RequestClass: service.OpenAIRouteRequestClassText, EndpointHash: "hk", Transport: "http_sse",
	}
	second := first
	second.AccountID = 24
	second.EndpointHash = "jp"
	failure := service.OpenAIRouteHealthEvent{At: start, FailureClass: service.OpenAIRouteFailureUpstream5xx}

	result, err := s.cache.RecordProviderEvidence(s.ctx, first, failure, policy, 2)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, result.DistinctFailingAccounts)
	require.False(s.T(), result.ProviderEventApplied)

	failure.At = start.Add(time.Millisecond)
	result, err = s.cache.RecordProviderEvidence(s.ctx, first, failure, policy, 2)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, result.DistinctFailingAccounts, "repeated failures on one account are not correlation")
	require.False(s.T(), result.ProviderEventApplied)

	failure.At = start.Add(2 * time.Millisecond)
	result, err = s.cache.RecordProviderEvidence(s.ctx, second, failure, policy, 2)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, result.DistinctFailingAccounts)
	require.True(s.T(), result.ProviderEventApplied)
	require.Equal(s.T(), service.OpenAIRouteCircuitOpen, result.State.State)

	success := service.OpenAIRouteHealthEvent{At: start.Add(3 * time.Millisecond), Success: true, FailureClass: service.OpenAIRouteFailureNone}
	result, err = s.cache.RecordProviderEvidence(s.ctx, first, success, policy, 2)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 1, result.DistinctFailingAccounts)
	require.False(s.T(), result.ProviderEventApplied)

	success.At = start.Add(4 * time.Millisecond)
	result, err = s.cache.RecordProviderEvidence(s.ctx, second, success, policy, 2)
	require.NoError(s.T(), err)
	require.Zero(s.T(), result.DistinctFailingAccounts)
	require.True(s.T(), result.ProviderEventApplied)
	require.Equal(s.T(), service.OpenAIRouteCircuitRecovering, result.State.State)
}

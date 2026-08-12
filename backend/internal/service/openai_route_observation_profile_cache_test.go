package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAIRouteProfileCacheStoreStub struct {
	calls    atomic.Uint64
	delay    time.Duration
	started  chan struct{}
	mu       sync.RWMutex
	err      error
	profiles map[string]OpenAIRouteObservationProfile
}

func (*openAIRouteProfileCacheStoreStub) Check(context.Context) error { return nil }
func (*openAIRouteProfileCacheStoreStub) Record(context.Context, OpenAIRouteObservation) error {
	return nil
}
func (*openAIRouteProfileCacheStoreStub) RecordCost(context.Context, OpenAIRouteActualCostObservation) error {
	return nil
}

func (s *openAIRouteProfileCacheStoreStub) GetBatch(ctx context.Context, _ []OpenAIRouteKey, _ time.Time) (map[string]OpenAIRouteObservationProfile, error) {
	s.calls.Add(1)
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	if s.delay > 0 {
		timer := time.NewTimer(s.delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.err != nil {
		return nil, s.err
	}
	return cloneOpenAIRouteObservationProfiles(s.profiles), nil
}

func (s *openAIRouteProfileCacheStoreStub) setError(err error) {
	s.mu.Lock()
	s.err = err
	s.mu.Unlock()
}

func testOpenAIRouteProfileCacheKey(accountID int64) OpenAIRouteKey {
	return OpenAIRouteKey{
		GroupID:       7,
		AccountID:     accountID,
		Model:         "gpt-5.6-sol",
		RequestClass:  OpenAIRouteRequestClassText,
		EndpointHash:  "endpoint-" + string(rune('a'+accountID)),
		Transport:     string(OpenAIUpstreamTransportHTTPSSE),
		FailureDomain: "provider-test",
	}
}

func testOpenAIRouteProfileCacheProfiles(keys ...OpenAIRouteKey) map[string]OpenAIRouteObservationProfile {
	profiles := make(map[string]OpenAIRouteObservationProfile, len(keys))
	for _, key := range keys {
		aggregate := NewOpenAIRouteObservationAggregate()
		aggregate.AttemptCount = uint64(key.AccountID * 10)
		aggregate.ReliabilityCount = aggregate.AttemptCount
		aggregate.SuccessCount = aggregate.AttemptCount
		aggregate.FailureCounts[OpenAIRouteFailureRateLimit] = uint64(key.AccountID)
		aggregate.TTFTHistogram[0] = aggregate.AttemptCount
		profiles[OpenAIRouteObservationFingerprint(key)] = OpenAIRouteObservationProfile{Global: aggregate}
	}
	return profiles
}

func TestOpenAIRouteObservationProfileCacheHitIsDeeplyIsolated(t *testing.T) {
	key := testOpenAIRouteProfileCacheKey(1)
	store := &openAIRouteProfileCacheStoreStub{profiles: testOpenAIRouteProfileCacheProfiles(key)}
	cache := newOpenAIRouteObservationProfileCache(store, time.Second, time.Second, 8)
	queryTime := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)

	first, err := cache.GetBatch(context.Background(), []OpenAIRouteKey{key}, queryTime)
	require.NoError(t, err)
	fingerprint := OpenAIRouteObservationFingerprint(key)
	mutated := first[fingerprint]
	mutated.Global.AttemptCount = 999
	mutated.Global.FailureCounts[OpenAIRouteFailureRateLimit] = 999
	mutated.Global.TTFTHistogram[0] = 999
	first[fingerprint] = mutated

	second, err := cache.GetBatch(context.Background(), []OpenAIRouteKey{key}, queryTime)
	require.NoError(t, err)
	require.Equal(t, uint64(10), second[fingerprint].Global.AttemptCount)
	require.Equal(t, uint64(1), second[fingerprint].Global.FailureCounts[OpenAIRouteFailureRateLimit])
	require.Equal(t, uint64(10), second[fingerprint].Global.TTFTHistogram[0])
	require.Equal(t, uint64(1), store.calls.Load())
	stats := cache.Stats()
	require.Equal(t, uint64(1), stats.Hits)
	require.Equal(t, uint64(1), stats.Misses)
	require.Equal(t, uint64(1), stats.Loads)
	require.Equal(t, 1, stats.Entries)
}

func TestOpenAIRouteObservationProfileCacheCanonicalizesOrderAndDuplicates(t *testing.T) {
	first := testOpenAIRouteProfileCacheKey(1)
	second := testOpenAIRouteProfileCacheKey(2)
	store := &openAIRouteProfileCacheStoreStub{profiles: testOpenAIRouteProfileCacheProfiles(first, second)}
	cache := newOpenAIRouteObservationProfileCache(store, time.Second, time.Second, 8)
	queryTime := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)

	_, err := cache.GetBatch(context.Background(), []OpenAIRouteKey{first, second, first}, queryTime)
	require.NoError(t, err)
	_, err = cache.GetBatch(context.Background(), []OpenAIRouteKey{second, first}, queryTime)
	require.NoError(t, err)
	require.Equal(t, uint64(1), store.calls.Load())
}

func TestOpenAIRouteObservationProfileCacheCoalescesConcurrentMisses(t *testing.T) {
	key := testOpenAIRouteProfileCacheKey(1)
	store := &openAIRouteProfileCacheStoreStub{
		delay:    20 * time.Millisecond,
		started:  make(chan struct{}, 1),
		profiles: testOpenAIRouteProfileCacheProfiles(key),
	}
	cache := newOpenAIRouteObservationProfileCache(store, time.Second, time.Second, 8)
	queryTime := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)

	const callers = 32
	start := make(chan struct{})
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for range callers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := cache.GetBatch(context.Background(), []OpenAIRouteKey{key}, queryTime)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, uint64(1), store.calls.Load())
	stats := cache.Stats()
	require.Equal(t, uint64(1), stats.Loads)
	require.GreaterOrEqual(t, stats.SharedReturns, uint64(2))
}

func TestOpenAIRouteObservationProfileCacheCallerCancellationDoesNotPoisonWarmup(t *testing.T) {
	key := testOpenAIRouteProfileCacheKey(1)
	store := &openAIRouteProfileCacheStoreStub{
		delay:    20 * time.Millisecond,
		started:  make(chan struct{}, 1),
		profiles: testOpenAIRouteProfileCacheProfiles(key),
	}
	cache := newOpenAIRouteObservationProfileCache(store, time.Second, 100*time.Millisecond, 8)
	queryTime := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, err := cache.GetBatch(ctx, []OpenAIRouteKey{key}, queryTime)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Eventually(t, func() bool { return !cache.Stats().LastLoadSuccess.IsZero() }, time.Second, time.Millisecond)
	_, err = cache.GetBatch(context.Background(), []OpenAIRouteKey{key}, queryTime)
	require.NoError(t, err)
	require.Equal(t, uint64(1), store.calls.Load())
}

func TestOpenAIRouteObservationProfileCacheDoesNotCacheErrors(t *testing.T) {
	key := testOpenAIRouteProfileCacheKey(1)
	store := &openAIRouteProfileCacheStoreStub{profiles: testOpenAIRouteProfileCacheProfiles(key)}
	store.setError(errors.New("redis unavailable"))
	cache := newOpenAIRouteObservationProfileCache(store, time.Second, time.Second, 8)
	queryTime := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)

	_, err := cache.GetBatch(context.Background(), []OpenAIRouteKey{key}, queryTime)
	require.ErrorContains(t, err, "redis unavailable")
	store.setError(nil)
	_, err = cache.GetBatch(context.Background(), []OpenAIRouteKey{key}, queryTime)
	require.NoError(t, err)
	require.Equal(t, uint64(2), store.calls.Load())
	require.Equal(t, uint64(1), cache.Stats().LoadErrors)
}

func TestOpenAIRouteObservationProfileCacheExpiresAndStaysBounded(t *testing.T) {
	first := testOpenAIRouteProfileCacheKey(1)
	second := testOpenAIRouteProfileCacheKey(2)
	store := &openAIRouteProfileCacheStoreStub{profiles: testOpenAIRouteProfileCacheProfiles(first, second)}
	cache := newOpenAIRouteObservationProfileCache(store, 10*time.Millisecond, time.Second, 2)
	queryTime := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)

	_, err := cache.GetBatch(context.Background(), []OpenAIRouteKey{first}, queryTime)
	require.NoError(t, err)
	time.Sleep(15 * time.Millisecond)
	_, err = cache.GetBatch(context.Background(), []OpenAIRouteKey{first}, queryTime)
	require.NoError(t, err)
	// Three different query buckets force three live keys; maxEntries keeps only two.
	_, err = cache.GetBatch(context.Background(), []OpenAIRouteKey{second}, queryTime.Add(time.Second))
	require.NoError(t, err)
	_, err = cache.GetBatch(context.Background(), []OpenAIRouteKey{first, second}, queryTime.Add(2*time.Second))
	require.NoError(t, err)
	stats := cache.Stats()
	require.Equal(t, 2, stats.Entries)
	require.GreaterOrEqual(t, stats.Evictions, uint64(1))
	require.Equal(t, uint64(4), store.calls.Load())
}

func BenchmarkOpenAIRouteObservationProfileCacheHit(b *testing.B) {
	key := testOpenAIRouteProfileCacheKey(1)
	store := &openAIRouteProfileCacheStoreStub{profiles: testOpenAIRouteProfileCacheProfiles(key)}
	cache := newOpenAIRouteObservationProfileCache(store, time.Hour, time.Second, 8)
	queryTime := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	_, _ = cache.GetBatch(context.Background(), []OpenAIRouteKey{key}, queryTime)
	b.ResetTimer()
	for range b.N {
		_, _ = cache.GetBatch(context.Background(), []OpenAIRouteKey{key}, queryTime)
	}
}

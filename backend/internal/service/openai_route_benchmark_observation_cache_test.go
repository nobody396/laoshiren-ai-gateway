package service

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAIRouteBenchmarkProfileCacheStoreStub struct {
	calls    atomic.Uint64
	profiles map[string]OpenAIRouteBenchmarkObservationProfile
}

func (s *openAIRouteBenchmarkProfileCacheStoreStub) GetBenchmarkBatch(
	context.Context,
	[]OpenAIRouteKey,
	time.Time,
) (map[string]OpenAIRouteBenchmarkObservationProfile, error) {
	s.calls.Add(1)
	return cloneOpenAIRouteBenchmarkProfiles(s.profiles), nil
}

func TestOpenAIRouteBenchmarkObservationProfileCacheAvoidsHotPathReadsAndClones(t *testing.T) {
	key := testOpenAIRouteProfileCacheKey(1)
	now := time.Date(2026, 8, 15, 8, 30, 0, 0, time.UTC)
	profile := FinalizeOpenAIRouteBenchmarkObservationProfile(testOpenAIRouteBenchmarkProfile(80, 80, now), now)
	fingerprint := OpenAIRouteObservationFingerprint(key)
	store := &openAIRouteBenchmarkProfileCacheStoreStub{profiles: map[string]OpenAIRouteBenchmarkObservationProfile{fingerprint: profile}}
	cache := newOpenAIRouteBenchmarkObservationProfileCache(store, time.Minute, time.Second, 8)

	first, err := cache.GetBatch(context.Background(), []OpenAIRouteKey{key}, now)
	require.NoError(t, err)
	mutated := first[fingerprint]
	mutated.Global.ReliabilityCount = 999
	mutated.Global.TTFTHistogram[0] = 999
	first[fingerprint] = mutated

	second, err := cache.GetBatch(context.Background(), []OpenAIRouteKey{key, key}, now)
	require.NoError(t, err)
	require.Equal(t, uint64(80), second[fingerprint].Global.ReliabilityCount)
	require.NotEqual(t, uint64(999), second[fingerprint].Global.TTFTHistogram[0])
	require.Equal(t, uint64(1), store.calls.Load())
}

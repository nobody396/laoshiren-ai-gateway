package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	// Active probes arrive at most every thirty minutes, so one minute keeps
	// evidence fresh without putting the operational PostgreSQL table on the
	// per-request Shadow hot path.
	defaultOpenAIRouteBenchmarkProfileCacheTTL        = time.Minute
	defaultOpenAIRouteBenchmarkProfileLoadTimeout     = 20 * time.Millisecond
	defaultOpenAIRouteBenchmarkProfileCacheMaxEntries = 128
)

type cachedOpenAIRouteBenchmarkProfiles struct {
	profiles  map[string]OpenAIRouteBenchmarkObservationProfile
	expiresAt time.Time
}

type openAIRouteBenchmarkObservationProfileCache struct {
	store       OpenAIRouteBenchmarkObservationStore
	ttl         time.Duration
	loadTimeout time.Duration
	maxEntries  int

	mu      sync.Mutex
	entries map[string]cachedOpenAIRouteBenchmarkProfiles
	flight  singleflight.Group
}

func newOpenAIRouteBenchmarkObservationProfileCache(
	store OpenAIRouteBenchmarkObservationStore,
	ttl time.Duration,
	loadTimeout time.Duration,
	maxEntries int,
) *openAIRouteBenchmarkObservationProfileCache {
	if ttl <= 0 {
		ttl = defaultOpenAIRouteBenchmarkProfileCacheTTL
	}
	if loadTimeout <= 0 {
		loadTimeout = defaultOpenAIRouteBenchmarkProfileLoadTimeout
	}
	if maxEntries <= 0 {
		maxEntries = defaultOpenAIRouteBenchmarkProfileCacheMaxEntries
	}
	return &openAIRouteBenchmarkObservationProfileCache{
		store: store, ttl: ttl, loadTimeout: loadTimeout, maxEntries: maxEntries,
		entries: make(map[string]cachedOpenAIRouteBenchmarkProfiles),
	}
}

func (c *openAIRouteBenchmarkObservationProfileCache) GetBatch(
	ctx context.Context,
	keys []OpenAIRouteKey,
	queryTime time.Time,
) (map[string]OpenAIRouteBenchmarkObservationProfile, error) {
	if len(keys) == 0 {
		return map[string]OpenAIRouteBenchmarkObservationProfile{}, nil
	}
	if c == nil || c.store == nil {
		return nil, ErrOpenAIRouteNoCandidate
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if queryTime.IsZero() {
		queryTime = time.Now().UTC()
	} else {
		queryTime = queryTime.UTC()
	}
	canonicalKeys, cacheKey, err := c.canonicalize(keys, queryTime)
	if err != nil {
		return nil, err
	}
	if profiles, ok := c.lookup(cacheKey, time.Now()); ok {
		return profiles, nil
	}

	loadCtx := context.WithoutCancel(ctx)
	resultCh := c.flight.DoChan(cacheKey, func() (any, error) {
		if profiles, ok := c.lookup(cacheKey, time.Now()); ok {
			return profiles, nil
		}
		boundedCtx, cancel := context.WithTimeout(loadCtx, c.loadTimeout)
		defer cancel()
		profiles, loadErr := c.store.GetBenchmarkBatch(boundedCtx, canonicalKeys, queryTime)
		if loadErr != nil {
			return nil, loadErr
		}
		loadedAt := time.Now()
		c.storeProfiles(cacheKey, profiles, loadedAt)
		return cloneOpenAIRouteBenchmarkProfiles(profiles), nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		profiles, ok := result.Val.(map[string]OpenAIRouteBenchmarkObservationProfile)
		if !ok {
			return nil, errors.New("invalid OpenAI route benchmark profile cache result")
		}
		return cloneOpenAIRouteBenchmarkProfiles(profiles), nil
	}
}

func (c *openAIRouteBenchmarkObservationProfileCache) canonicalize(
	keys []OpenAIRouteKey,
	queryTime time.Time,
) ([]OpenAIRouteKey, string, error) {
	byFingerprint := make(map[string]OpenAIRouteKey, len(keys))
	fingerprints := make([]string, 0, len(keys))
	for _, key := range keys {
		if !key.Valid() {
			return nil, "", ErrOpenAIRouteNoCandidate
		}
		fingerprint := OpenAIRouteObservationFingerprint(key)
		if _, exists := byFingerprint[fingerprint]; exists {
			continue
		}
		byFingerprint[fingerprint] = key
		fingerprints = append(fingerprints, fingerprint)
	}
	sort.Strings(fingerprints)
	canonical := make([]OpenAIRouteKey, 0, len(fingerprints))
	for _, fingerprint := range fingerprints {
		canonical = append(canonical, byFingerprint[fingerprint])
	}
	bucket := queryTime.UnixNano() / c.ttl.Nanoseconds()
	bucketStart := time.Unix(0, bucket*c.ttl.Nanoseconds()).UTC().Format(time.RFC3339Nano)
	digest := sha256.Sum256([]byte(strings.Join(fingerprints, ",") + ":" + bucketStart))
	return canonical, hex.EncodeToString(digest[:16]), nil
}

func (c *openAIRouteBenchmarkObservationProfileCache) lookup(
	key string,
	now time.Time,
) (map[string]OpenAIRouteBenchmarkObservationProfile, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if !now.Before(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return cloneOpenAIRouteBenchmarkProfiles(entry.profiles), true
}

func (c *openAIRouteBenchmarkObservationProfileCache) storeProfiles(
	key string,
	profiles map[string]OpenAIRouteBenchmarkObservationProfile,
	now time.Time,
) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for existingKey, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, existingKey)
		}
	}
	if _, replacing := c.entries[key]; !replacing && len(c.entries) >= c.maxEntries {
		oldestKey := ""
		var oldestExpiry time.Time
		for existingKey, entry := range c.entries {
			if oldestKey == "" || entry.expiresAt.Before(oldestExpiry) {
				oldestKey = existingKey
				oldestExpiry = entry.expiresAt
			}
		}
		if oldestKey != "" {
			delete(c.entries, oldestKey)
		}
	}
	c.entries[key] = cachedOpenAIRouteBenchmarkProfiles{
		profiles: cloneOpenAIRouteBenchmarkProfiles(profiles), expiresAt: now.Add(c.ttl),
	}
}

func cloneOpenAIRouteBenchmarkProfiles(
	values map[string]OpenAIRouteBenchmarkObservationProfile,
) map[string]OpenAIRouteBenchmarkObservationProfile {
	cloned := make(map[string]OpenAIRouteBenchmarkObservationProfile, len(values))
	for key, value := range values {
		cloned[key] = OpenAIRouteBenchmarkObservationProfile{
			Global:     cloneOpenAIRouteObservationAggregate(value.Global),
			Recent:     cloneOpenAIRouteObservationAggregate(value.Recent),
			HourOfWeek: cloneOpenAIRouteObservationAggregate(value.HourOfWeek),
			Evidence:   value.Evidence,
		}
	}
	return cloned
}

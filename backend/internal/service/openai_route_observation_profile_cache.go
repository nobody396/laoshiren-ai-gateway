package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	defaultOpenAIRouteObservationProfileCacheTTL        = 5 * time.Second
	defaultOpenAIRouteObservationProfileLoadTimeout     = 20 * time.Millisecond
	defaultOpenAIRouteObservationProfileCacheMaxEntries = 512
)

type OpenAIRouteObservationProfileCacheStats struct {
	Hits            uint64    `json:"hits"`
	Misses          uint64    `json:"misses"`
	Loads           uint64    `json:"loads"`
	LoadErrors      uint64    `json:"load_errors"`
	SharedReturns   uint64    `json:"shared_returns"`
	Evictions       uint64    `json:"evictions"`
	Entries         int       `json:"entries"`
	LastLoadSuccess time.Time `json:"last_load_success_at,omitempty"`
	LastLoadFailure time.Time `json:"last_load_failure_at,omitempty"`
	LastError       string    `json:"last_error"`
}

type cachedOpenAIRouteObservationProfiles struct {
	profiles  map[string]OpenAIRouteObservationProfile
	expiresAt time.Time
}

// openAIRouteObservationProfileCache is a bounded L1 read-through cache over
// the shared Redis/PostgreSQL learner. Redis is authoritative for the recent
// view and PostgreSQL for long windows. The five-second TTL removes repeated
// aggregate reads from the 25ms Shadow hot path; route/provider circuit state
// is still read separately on every evaluation.
type openAIRouteObservationProfileCache struct {
	store       OpenAIRouteObservationStore
	ttl         time.Duration
	loadTimeout time.Duration
	maxEntries  int

	mu      sync.Mutex
	entries map[string]cachedOpenAIRouteObservationProfiles
	flight  singleflight.Group

	hits          atomic.Uint64
	misses        atomic.Uint64
	loads         atomic.Uint64
	loadErrors    atomic.Uint64
	sharedReturns atomic.Uint64
	evictions     atomic.Uint64
	lastSuccessNS atomic.Int64
	lastFailureNS atomic.Int64
	lastError     atomic.Value
}

func newOpenAIRouteObservationProfileCache(
	store OpenAIRouteObservationStore,
	ttl time.Duration,
	loadTimeout time.Duration,
	maxEntries int,
) *openAIRouteObservationProfileCache {
	if ttl <= 0 {
		ttl = defaultOpenAIRouteObservationProfileCacheTTL
	}
	if loadTimeout <= 0 {
		loadTimeout = defaultOpenAIRouteObservationProfileLoadTimeout
	}
	if maxEntries <= 0 {
		maxEntries = defaultOpenAIRouteObservationProfileCacheMaxEntries
	}
	cache := &openAIRouteObservationProfileCache{
		store:       store,
		ttl:         ttl,
		loadTimeout: loadTimeout,
		maxEntries:  maxEntries,
		entries:     make(map[string]cachedOpenAIRouteObservationProfiles),
	}
	cache.lastError.Store("")
	return cache
}

func (c *openAIRouteObservationProfileCache) GetBatch(
	ctx context.Context,
	keys []OpenAIRouteKey,
	queryTime time.Time,
) (map[string]OpenAIRouteObservationProfile, error) {
	if len(keys) == 0 {
		return map[string]OpenAIRouteObservationProfile{}, nil
	}
	if c == nil || c.store == nil {
		return nil, ErrOpenAIRouteNoCandidate
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if queryTime.IsZero() {
		queryTime = time.Now()
	}
	canonicalKeys, cacheKey, err := c.canonicalize(keys, queryTime)
	if err != nil {
		return nil, err
	}
	if profiles, ok := c.lookup(cacheKey, time.Now()); ok {
		c.hits.Add(1)
		return profiles, nil
	}
	c.misses.Add(1)

	loadCtx := context.WithoutCancel(ctx)
	resultCh := c.flight.DoChan(cacheKey, func() (any, error) {
		if profiles, ok := c.lookup(cacheKey, time.Now()); ok {
			return profiles, nil
		}
		c.loads.Add(1)
		boundedCtx, cancel := context.WithTimeout(loadCtx, c.loadTimeout)
		defer cancel()
		profiles, loadErr := c.store.GetBatch(boundedCtx, canonicalKeys, queryTime)
		if loadErr != nil {
			c.loadErrors.Add(1)
			c.recordLoadFailure(loadErr)
			return nil, loadErr
		}
		loadedAt := time.Now()
		c.storeProfiles(cacheKey, profiles, loadedAt)
		c.recordLoadSuccess(loadedAt)
		return cloneOpenAIRouteObservationProfiles(profiles), nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		if result.Shared {
			c.sharedReturns.Add(1)
		}
		if result.Err != nil {
			return nil, result.Err
		}
		profiles, ok := result.Val.(map[string]OpenAIRouteObservationProfile)
		if !ok {
			return nil, errors.New("invalid OpenAI route observation profile cache result")
		}
		return cloneOpenAIRouteObservationProfiles(profiles), nil
	}
}

func (c *openAIRouteObservationProfileCache) canonicalize(keys []OpenAIRouteKey, queryTime time.Time) ([]OpenAIRouteKey, string, error) {
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
	// A time bucket keeps historical/test reads distinct while bounding stale
	// real-time data to the configured TTL.
	bucket := queryTime.UnixNano() / c.ttl.Nanoseconds()
	digest := sha256.Sum256([]byte(strings.Join(fingerprints, ",") + ":" + time.Unix(0, bucket*c.ttl.Nanoseconds()).UTC().Format(time.RFC3339Nano)))
	return canonical, hex.EncodeToString(digest[:16]), nil
}

func (c *openAIRouteObservationProfileCache) lookup(key string, now time.Time) (map[string]OpenAIRouteObservationProfile, bool) {
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
	return cloneOpenAIRouteObservationProfiles(entry.profiles), true
}

func (c *openAIRouteObservationProfileCache) storeProfiles(key string, profiles map[string]OpenAIRouteObservationProfile, now time.Time) {
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
			c.evictions.Add(1)
		}
	}
	c.entries[key] = cachedOpenAIRouteObservationProfiles{
		profiles:  cloneOpenAIRouteObservationProfiles(profiles),
		expiresAt: now.Add(c.ttl),
	}
}

func (c *openAIRouteObservationProfileCache) Stats() OpenAIRouteObservationProfileCacheStats {
	if c == nil {
		return OpenAIRouteObservationProfileCacheStats{}
	}
	stats := OpenAIRouteObservationProfileCacheStats{
		Hits:          c.hits.Load(),
		Misses:        c.misses.Load(),
		Loads:         c.loads.Load(),
		LoadErrors:    c.loadErrors.Load(),
		SharedReturns: c.sharedReturns.Load(),
		Evictions:     c.evictions.Load(),
	}
	now := time.Now()
	c.mu.Lock()
	for key, entry := range c.entries {
		if !now.Before(entry.expiresAt) {
			delete(c.entries, key)
		}
	}
	stats.Entries = len(c.entries)
	c.mu.Unlock()
	if ns := c.lastSuccessNS.Load(); ns > 0 {
		stats.LastLoadSuccess = time.Unix(0, ns).UTC()
	}
	if ns := c.lastFailureNS.Load(); ns > 0 {
		stats.LastLoadFailure = time.Unix(0, ns).UTC()
	}
	if value, ok := c.lastError.Load().(string); ok {
		stats.LastError = strings.TrimSpace(value)
	}
	return stats
}

func (c *openAIRouteObservationProfileCache) recordLoadSuccess(now time.Time) {
	c.lastSuccessNS.Store(now.UTC().UnixNano())
	c.lastError.Store("")
}

func (c *openAIRouteObservationProfileCache) recordLoadFailure(err error) {
	c.lastFailureNS.Store(time.Now().UTC().UnixNano())
	if err != nil {
		c.lastError.Store(err.Error())
	}
}

func cloneOpenAIRouteObservationProfiles(values map[string]OpenAIRouteObservationProfile) map[string]OpenAIRouteObservationProfile {
	cloned := make(map[string]OpenAIRouteObservationProfile, len(values))
	for key, value := range values {
		cloned[key] = OpenAIRouteObservationProfile{
			Global:     cloneOpenAIRouteObservationAggregate(value.Global),
			Recent:     cloneOpenAIRouteObservationAggregate(value.Recent),
			HourOfWeek: cloneOpenAIRouteObservationAggregate(value.HourOfWeek),
		}
	}
	return cloned
}

func cloneOpenAIRouteObservationAggregate(value OpenAIRouteObservationAggregate) OpenAIRouteObservationAggregate {
	cloned := value
	cloned.FailureCounts = make(map[OpenAIRouteFailureClass]uint64, len(value.FailureCounts))
	for class, count := range value.FailureCounts {
		cloned.FailureCounts[class] = count
	}
	cloned.TTFTHistogram = append([]uint64(nil), value.TTFTHistogram...)
	cloned.LatencyHistogram = append([]uint64(nil), value.LatencyHistogram...)
	return cloned
}

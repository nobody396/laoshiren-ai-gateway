package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	openAIRouteReliabilityAdapterWindow       = time.Hour
	openAIRouteReliabilityAdapterFreshness    = 5 * time.Minute
	openAIRouteReliabilityAdapterTailLag      = time.Second
	openAIRouteReliabilityAdapterReadyTTL     = 15 * time.Second
	openAIRouteReliabilityAdapterFallbackTTL  = 5 * time.Second
	openAIRouteReliabilityAdapterLoadTimeout  = 2 * time.Second
	openAIRouteReliabilityAdapterMinimumRoute = 2
	openAIRouteReliabilityAdapterMaxEntries   = 256
	openAIRouteReliabilityAdapterMaxRefreshes = 2
)

type OpenAIRouteReliabilityEvidenceRequest struct {
	Keys            []OpenAIRouteKey
	GroupID         int64
	AccessGroupID   int64
	Model           string
	RequestClass    OpenAIRouteRequestClass
	InboundProtocol string
	Now             time.Time
}

type OpenAIRouteReliabilityEvidenceMeta struct {
	Applied         bool      `json:"applied"`
	Reason          string    `json:"reason"`
	GeneratedAt     time.Time `json:"generated_at,omitempty"`
	ObservedThrough time.Time `json:"observed_through,omitempty"`
	SampleCount     uint64    `json:"sample_count"`
	PendingCutoffID uint64    `json:"pending_cutoff_id,omitempty"`
}

type OpenAIRouteReliabilityEvidenceResult struct {
	Profiles map[string]OpenAIRouteObservationProfile
	Meta     OpenAIRouteReliabilityEvidenceMeta
}

type OpenAIRouteReliabilityEvidenceAdapter interface {
	Lookup(req OpenAIRouteReliabilityEvidenceRequest) OpenAIRouteReliabilityEvidenceResult
}

type cachedOpenAIRouteReliabilityEvidence struct {
	result     OpenAIRouteReliabilityEvidenceResult
	expiresAt  time.Time
	loading    bool
	generation uint64
}

type ReliabilityEvidenceOpenAIRouteAdapter struct {
	evidence       *ReliabilityEvidenceService
	mu             sync.Mutex
	cache          map[string]cachedOpenAIRouteReliabilityEvidence
	baseline       ReliabilityEvidenceCompleteness
	lossDropped    int64
	lossFailed     int64
	lossDetectedAt time.Time
	ctx            context.Context
	cancel         context.CancelFunc
	refreshSlots   chan struct{}
	refreshWG      sync.WaitGroup
	stopped        bool
}

func NewReliabilityEvidenceOpenAIRouteAdapter(evidence *ReliabilityEvidenceService) *ReliabilityEvidenceOpenAIRouteAdapter {
	ctx, cancel := context.WithCancel(context.Background())
	adapter := &ReliabilityEvidenceOpenAIRouteAdapter{
		evidence:     evidence,
		cache:        map[string]cachedOpenAIRouteReliabilityEvidence{},
		ctx:          ctx,
		cancel:       cancel,
		refreshSlots: make(chan struct{}, openAIRouteReliabilityAdapterMaxRefreshes),
	}
	if evidence != nil {
		adapter.baseline = evidence.Completeness()
		evidence.registerDependentStop(adapter.Stop)
	}
	return adapter
}

func (a *ReliabilityEvidenceOpenAIRouteAdapter) Stop() {
	if a == nil {
		return
	}
	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return
	}
	a.stopped = true
	a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
	a.refreshWG.Wait()
}

func (a *ReliabilityEvidenceOpenAIRouteAdapter) Lookup(req OpenAIRouteReliabilityEvidenceRequest) OpenAIRouteReliabilityEvidenceResult {
	if a == nil || a.evidence == nil {
		return openAIRouteReliabilityFallback("adapter_unavailable")
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	req.Now = now
	cacheKey, ok := openAIRouteReliabilityCacheKey(req)
	if !ok {
		return openAIRouteReliabilityFallback("request_scope_invalid")
	}
	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return openAIRouteReliabilityFallback("adapter_stopped")
	}
	entry, exists := a.cache[cacheKey]
	if exists && now.Before(entry.expiresAt) {
		result := cloneOpenAIRouteReliabilityResult(entry.result)
		a.mu.Unlock()
		return result
	}
	if !entry.loading {
		select {
		case a.refreshSlots <- struct{}{}:
			if !exists && len(a.cache) >= openAIRouteReliabilityAdapterMaxEntries {
				a.pruneLocked(now)
			}
			entry.generation++
			entry.loading = true
			a.cache[cacheKey] = entry
			a.refreshWG.Add(1)
			go a.refreshAsync(cacheKey, entry.generation, req)
		default:
			a.mu.Unlock()
			return openAIRouteReliabilityFallback("refresh_saturated")
		}
	}
	reason := "cache_warming"
	if exists {
		reason = "cache_stale"
	}
	a.mu.Unlock()
	return openAIRouteReliabilityFallback(reason)
}

func (a *ReliabilityEvidenceOpenAIRouteAdapter) refreshForTest(ctx context.Context, req OpenAIRouteReliabilityEvidenceRequest) OpenAIRouteReliabilityEvidenceResult {
	if a == nil || a.evidence == nil {
		return openAIRouteReliabilityFallback("adapter_unavailable")
	}
	if req.Now.IsZero() {
		req.Now = time.Now().UTC()
	} else {
		req.Now = req.Now.UTC()
	}
	cacheKey, ok := openAIRouteReliabilityCacheKey(req)
	if !ok {
		return openAIRouteReliabilityFallback("request_scope_invalid")
	}
	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return openAIRouteReliabilityFallback("adapter_stopped")
	}
	entry := a.cache[cacheKey]
	entry.generation++
	entry.loading = true
	a.cache[cacheKey] = entry
	generation := entry.generation
	a.mu.Unlock()
	result := a.load(ctx, req)
	a.storeResult(cacheKey, generation, req.Now, result)
	return result
}

func (a *ReliabilityEvidenceOpenAIRouteAdapter) refreshAsync(cacheKey string, generation uint64, req OpenAIRouteReliabilityEvidenceRequest) {
	defer a.refreshWG.Done()
	defer func() { <-a.refreshSlots }()
	ctx, cancel := context.WithTimeout(a.ctx, openAIRouteReliabilityAdapterLoadTimeout)
	defer cancel()
	result := a.load(ctx, req)
	a.storeResult(cacheKey, generation, req.Now, result)
}

func (a *ReliabilityEvidenceOpenAIRouteAdapter) storeResult(cacheKey string, generation uint64, now time.Time, result OpenAIRouteReliabilityEvidenceResult) {
	ttl := openAIRouteReliabilityAdapterFallbackTTL
	if result.Meta.Applied {
		ttl = openAIRouteReliabilityAdapterReadyTTL
	}
	a.mu.Lock()
	if a.stopped {
		a.mu.Unlock()
		return
	}
	entry, exists := a.cache[cacheKey]
	if !exists || entry.generation != generation {
		a.mu.Unlock()
		return
	}
	entry.result = cloneOpenAIRouteReliabilityResult(result)
	entry.expiresAt = now.Add(ttl)
	entry.loading = false
	a.cache[cacheKey] = entry
	a.mu.Unlock()
}

func (a *ReliabilityEvidenceOpenAIRouteAdapter) load(ctx context.Context, req OpenAIRouteReliabilityEvidenceRequest) OpenAIRouteReliabilityEvidenceResult {
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if _, ok := openAIRouteReliabilityCacheKey(req); !ok {
		return openAIRouteReliabilityFallback("request_scope_invalid")
	}
	end := now.Add(-openAIRouteReliabilityAdapterTailLag)
	routingFingerprints := make([]string, 0, len(req.Keys))
	transports := make([]string, 0, len(req.Keys))
	probeRoutes := make([]ReliabilityEvidenceProbeRoute, 0, len(req.Keys))
	for _, key := range req.Keys {
		routingFingerprints = append(routingFingerprints, OpenAIRouteObservationFingerprint(key))
		transports = append(transports, key.Transport)
		probeRoutes = append(probeRoutes, ReliabilityEvidenceProbeRoute{AccountID: key.AccountID, EndpointHash: key.EndpointHash, Transport: key.Transport})
	}
	query := &ReliabilityEvidenceQuery{
		Start: now.Add(-openAIRouteReliabilityAdapterWindow), End: end,
		AnyModelPatterns: []string{req.Model}, Limit: 5001,
		AttemptScope: &ReliabilityEvidenceAttemptScope{
			GroupID: req.GroupID, AccessGroupID: req.AccessGroupID, Protocol: req.InboundProtocol,
			Transports: transports, RoutingFingerprints: routingFingerprints,
		},
	}
	if openAIRouteReliabilityAllowsHTTPProbe(req.InboundProtocol) {
		query.ProbeScope = &ReliabilityEvidenceProbeScope{Protocol: "http", Routes: probeRoutes}
	}
	snapshot, err := a.evidence.Snapshot(ctx, query)
	if err != nil {
		return openAIRouteReliabilityFallback("snapshot_unavailable")
	}
	meta := OpenAIRouteReliabilityEvidenceMeta{GeneratedAt: snapshot.GeneratedAt, ObservedThrough: end, PendingCutoffID: snapshot.Completeness.PendingCutoffID}
	if !snapshot.Completeness.Enabled || !snapshot.Completeness.Running {
		meta.Reason = "collector_unavailable"
		return OpenAIRouteReliabilityEvidenceResult{Profiles: map[string]OpenAIRouteObservationProfile{}, Meta: meta}
	}
	if a.evidenceLossContaminatesWindow(snapshot.Completeness, now) {
		meta.Reason = "evidence_loss"
		return OpenAIRouteReliabilityEvidenceResult{Profiles: map[string]OpenAIRouteObservationProfile{}, Meta: meta}
	}
	if snapshot.Completeness.OldestPendingAt != nil && !snapshot.Completeness.OldestPendingAt.After(end) {
		meta.Reason = "evidence_pending"
		return OpenAIRouteReliabilityEvidenceResult{Profiles: map[string]OpenAIRouteObservationProfile{}, Meta: meta}
	}
	if len(snapshot.Observations) > 5000 {
		meta.Reason = "evidence_truncated"
		return OpenAIRouteReliabilityEvidenceResult{Profiles: map[string]OpenAIRouteObservationProfile{}, Meta: meta}
	}
	aggregates := make(map[string]OpenAIRouteObservationAggregate, len(req.Keys))
	for _, key := range req.Keys {
		aggregates[OpenAIRouteObservationFingerprint(key)] = NewOpenAIRouteObservationAggregate()
	}
	for _, observation := range snapshot.Observations {
		if observation == nil || observation.Outcome == ReliabilityOutcomeExcluded {
			continue
		}
		for _, key := range req.Keys {
			if !reliabilityObservationMatchesOpenAIRoute(observation, key, req) {
				continue
			}
			fingerprint := OpenAIRouteObservationFingerprint(key)
			aggregate := aggregates[fingerprint]
			addReliabilityObservationToOpenAIRouteAggregate(&aggregate, observation)
			aggregates[fingerprint] = aggregate
		}
	}
	profiles := make(map[string]OpenAIRouteObservationProfile, len(req.Keys))
	for _, key := range req.Keys {
		fingerprint := OpenAIRouteObservationFingerprint(key)
		aggregate := aggregates[fingerprint]
		if aggregate.ReliabilityCount < openAIRouteReliabilityAdapterMinimumRoute || now.Sub(aggregate.LastObservedAt) > openAIRouteReliabilityAdapterFreshness {
			meta.Reason = "route_evidence_incomplete"
			return OpenAIRouteReliabilityEvidenceResult{Profiles: map[string]OpenAIRouteObservationProfile{}, Meta: meta}
		}
		profiles[fingerprint] = OpenAIRouteObservationProfile{Global: aggregate, Recent: aggregate}
		meta.SampleCount += aggregate.ReliabilityCount
	}
	meta.Applied = true
	meta.Reason = "reliability_evidence_ready"
	return OpenAIRouteReliabilityEvidenceResult{Profiles: profiles, Meta: meta}
}

func (a *ReliabilityEvidenceOpenAIRouteAdapter) evidenceLossContaminatesWindow(completeness ReliabilityEvidenceCompleteness, now time.Time) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if completeness.Dropped < a.baseline.Dropped || completeness.Failed < a.baseline.Failed {
		a.baseline.Dropped = completeness.Dropped
		a.baseline.Failed = completeness.Failed
		a.lossDetectedAt = time.Time{}
	}
	if completeness.Dropped <= a.baseline.Dropped && completeness.Failed <= a.baseline.Failed {
		return false
	}
	if a.lossDetectedAt.IsZero() || completeness.Dropped != a.lossDropped || completeness.Failed != a.lossFailed {
		a.lossDropped = completeness.Dropped
		a.lossFailed = completeness.Failed
		a.lossDetectedAt = now
	}
	if now.Before(a.lossDetectedAt.Add(openAIRouteReliabilityAdapterWindow + openAIRouteReliabilityAdapterTailLag)) {
		return true
	}
	a.baseline.Dropped = completeness.Dropped
	a.baseline.Failed = completeness.Failed
	a.lossDetectedAt = time.Time{}
	return false
}

func reliabilityObservationMatchesOpenAIRoute(observation *ReliabilityObservation, key OpenAIRouteKey, req OpenAIRouteReliabilityEvidenceRequest) bool {
	if observation.AccountID == nil || *observation.AccountID != key.AccountID || observation.EndpointHash != key.EndpointHash || observation.Model != key.Model || observation.RequestClass != string(key.RequestClass) || !strings.EqualFold(observation.Transport, key.Transport) {
		return false
	}
	switch observation.FactType {
	case ReliabilityFactUpstreamAttempt:
		return observation.GroupID != nil && *observation.GroupID == key.GroupID &&
			observation.AccessGroupID == req.AccessGroupID &&
			observation.RoutingFingerprint == OpenAIRouteObservationFingerprint(key) &&
			strings.EqualFold(observation.Protocol, req.InboundProtocol)
	case ReliabilityFactActiveProbe:
		return observation.GroupID == nil && strings.EqualFold(observation.Protocol, "http") && openAIRouteReliabilityAllowsHTTPProbe(req.InboundProtocol)
	default:
		return false
	}
}

func openAIRouteReliabilityAllowsHTTPProbe(inboundProtocol string) bool {
	protocol := strings.ToLower(strings.TrimSpace(inboundProtocol))
	return !strings.Contains(protocol, "ws") && !strings.Contains(protocol, "websocket")
}

func addReliabilityObservationToOpenAIRouteAggregate(aggregate *OpenAIRouteObservationAggregate, observation *ReliabilityObservation) {
	if aggregate == nil || observation == nil {
		return
	}
	aggregate.AttemptCount++
	switch observation.Outcome {
	case ReliabilityOutcomeSuccess, ReliabilityOutcomeRecovered:
		aggregate.ReliabilityCount++
		aggregate.SuccessCount++
	case ReliabilityOutcomeFailure:
		aggregate.ReliabilityCount++
		aggregate.FailureCount++
		class := openAIRouteFailureClassFromReliability(observation)
		aggregate.FailureCounts[class]++
	default:
		return
	}
	if observation.LatencyMs > 0 {
		aggregate.LatencySampleCount++
		bucket := OpenAIRouteLatencyHistogramBucket(observation.LatencyMs)
		if bucket >= 0 {
			aggregate.LatencyHistogram[bucket]++
		}
	}
	if observation.ObservedAt.After(aggregate.LastObservedAt) {
		aggregate.LastObservedAt = observation.ObservedAt
	}
}

func openAIRouteFailureClassFromReliability(observation *ReliabilityObservation) OpenAIRouteFailureClass {
	if observation.StatusCode != nil {
		switch {
		case *observation.StatusCode == 429:
			return OpenAIRouteFailureRateLimit
		case *observation.StatusCode >= 500:
			return OpenAIRouteFailureUpstream5xx
		}
	}
	if observation.ErrorOwner == "provider" {
		return OpenAIRouteFailureUpstream5xx
	}
	return OpenAIRouteFailureLocalTransport
}

func overlayOpenAIRouteReliability(base, shared OpenAIRouteObservationProfile) OpenAIRouteObservationProfile {
	base.Global = overlayOpenAIRouteReliabilityAggregate(base.Global, shared.Global)
	base.Recent = overlayOpenAIRouteReliabilityAggregate(base.Recent, shared.Recent)
	return base
}

func overlayOpenAIRouteReliabilityAggregate(base, shared OpenAIRouteObservationAggregate) OpenAIRouteObservationAggregate {
	base.AttemptCount = shared.AttemptCount
	base.ReliabilityCount = shared.ReliabilityCount
	base.SuccessCount = shared.SuccessCount
	base.FailureCount = shared.FailureCount
	// Reliability Observations do not encode partial-stream outcomes. Retain the
	// numerator for provenance, but never derive its rate against the replaced
	// denominator: the controller carries the exact legacy blended rate directly
	// into candidate scoring and audit.
	base.FailureCounts = shared.FailureCounts
	base.LatencySampleCount = shared.LatencySampleCount
	base.LatencyHistogram = shared.LatencyHistogram
	base.LastObservedAt = shared.LastObservedAt
	return base
}

func openAIRouteReliabilityCacheKey(req OpenAIRouteReliabilityEvidenceRequest) (string, bool) {
	if req.GroupID <= 0 || req.AccessGroupID < 0 || strings.TrimSpace(req.Model) == "" || !req.RequestClass.Valid() || strings.TrimSpace(req.InboundProtocol) == "" || len(req.Keys) == 0 {
		return "", false
	}
	fingerprints := make([]string, 0, len(req.Keys))
	for _, key := range req.Keys {
		if !key.Valid() {
			return "", false
		}
		fingerprints = append(fingerprints, OpenAIRouteObservationFingerprint(key))
	}
	sort.Strings(fingerprints)
	// Experiment and treatment IDs intentionally do not participate in this
	// technical-evidence cache. The immutable route facts are safe to reuse for
	// the same access/group/protocol scope; the per-treatment feature flag and
	// audit fingerprint decide whether those facts affect Shadow scoring.
	return fmt.Sprintf("%d|%d|%s|%s|%s|%s", req.GroupID, req.AccessGroupID, req.Model, req.RequestClass, strings.ToLower(req.InboundProtocol), strings.Join(fingerprints, ",")), true
}

func openAIRouteReliabilityFallback(reason string) OpenAIRouteReliabilityEvidenceResult {
	return OpenAIRouteReliabilityEvidenceResult{Profiles: map[string]OpenAIRouteObservationProfile{}, Meta: OpenAIRouteReliabilityEvidenceMeta{Reason: reason}}
}

func cloneOpenAIRouteReliabilityResult(source OpenAIRouteReliabilityEvidenceResult) OpenAIRouteReliabilityEvidenceResult {
	return OpenAIRouteReliabilityEvidenceResult{Profiles: cloneOpenAIRouteObservationProfiles(source.Profiles), Meta: source.Meta}
}

func (a *ReliabilityEvidenceOpenAIRouteAdapter) pruneLocked(now time.Time) {
	for key, entry := range a.cache {
		if !entry.loading && now.After(entry.expiresAt) {
			delete(a.cache, key)
		}
	}
	for len(a.cache) >= openAIRouteReliabilityAdapterMaxEntries {
		removed := false
		for key, entry := range a.cache {
			if entry.loading {
				continue
			}
			delete(a.cache, key)
			removed = true
			break
		}
		if !removed {
			break
		}
	}
}

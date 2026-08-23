package service

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func reliabilityAdapterTestKey(accountID, groupID int64, model string) OpenAIRouteKey {
	return OpenAIRouteKey{GroupID: groupID, AccountID: accountID, Model: model, RequestClass: OpenAIRouteRequestClassText, EndpointHash: "0123456789abcdef", Transport: string(OpenAIUpstreamTransportHTTPSSE), FailureDomain: "provider:test"}
}
func reliabilityAdapterObservation(key OpenAIRouteKey, groupID *int64, protocol string, outcome ReliabilityOutcome, at time.Time) *ReliabilityObservation {
	accountID := key.AccountID
	status := 503
	return &ReliabilityObservation{IdempotencyKey: string(outcome) + at.String(), FactType: ReliabilityFactUpstreamAttempt, GroupID: groupID, AccountID: &accountID, Platform: PlatformOpenAI, Model: key.Model, RequestClass: string(key.RequestClass), Protocol: protocol, Transport: key.Transport, EndpointHash: key.EndpointHash, RoutingFingerprint: OpenAIRouteObservationFingerprint(key), Outcome: outcome, StatusCode: &status, ErrorOwner: "provider", LatencyMs: 600, ObservedAt: at}
}
func newReliabilityAdapterEvidence(observations []*ReliabilityObservation) *ReliabilityEvidenceService {
	evidence := NewReliabilityEvidenceService(nil, nil)
	evidence.enabled = true
	evidence.running.Store(true)
	evidence.listEvidence = func(context.Context, *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		return observations, nil
	}
	return evidence
}
func TestReliabilityEvidenceOpenAIRouteAdapterBuildsExactScopedProfiles(t *testing.T) {
	now := time.Date(2026, 8, 23, 5, 0, 0, 0, time.UTC)
	groupID, wrongGroup := int64(7), int64(8)
	key := reliabilityAdapterTestKey(53, groupID, "gpt-5.6")
	observations := []*ReliabilityObservation{
		reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-4*time.Second)),
		reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeFailure, now.Add(-3*time.Second)),
		reliabilityAdapterObservation(key, &wrongGroup, "responses", ReliabilityOutcomeFailure, now.Add(-2*time.Second)),
		reliabilityAdapterObservation(key, &groupID, "chat_completions", ReliabilityOutcomeFailure, now.Add(-2*time.Second)),
	}
	adapter := NewReliabilityEvidenceOpenAIRouteAdapter(newReliabilityAdapterEvidence(observations))
	result := adapter.refreshForTest(context.Background(), OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{key}, GroupID: groupID, Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "responses", Now: now})
	require.True(t, result.Meta.Applied)
	profile := result.Profiles[OpenAIRouteObservationFingerprint(key)]
	require.Equal(t, uint64(2), profile.Global.ReliabilityCount)
	require.Equal(t, uint64(1), profile.Global.SuccessCount)
	require.Equal(t, uint64(1), profile.Global.FailureCount)
	require.Equal(t, uint64(1), profile.Global.FailureCounts[OpenAIRouteFailureUpstream5xx])
	require.Equal(t, uint64(2), profile.Global.LatencySampleCount)
}

func TestReliabilityAttemptPersistenceFeedsExactRouteAdapter(t *testing.T) {
	now := time.Now().UTC()
	groupID, accountID := int64(7), int64(53)
	accessGroupID := int64(90)
	key := reliabilityAdapterTestKey(accountID, groupID, "gpt-5.6")
	var persisted []*ReliabilityObservation
	repo := &reliabilityRepoStub{batch: func(_ context.Context, inputs []*ReliabilityObservation) (int64, error) {
		persisted = append(persisted, inputs...)
		return int64(len(inputs)), nil
	}}
	evidence := newReliabilityEvidenceForTest(true, repo)
	for index, observedAt := range []time.Time{now.Add(-3 * time.Second), now.Add(-2 * time.Second)} {
		_, err := evidence.recordAttemptOutcomes(context.Background(), []*ReliabilityAttemptOutcome{{
			RequestIdentity: fmt.Sprintf("gateway-request-%d", index), AttemptIdentity: "success",
			GroupID: &groupID, AccessGroupID: accessGroupID, AccountID: &accountID,
			Platform: PlatformOpenAI, Model: key.Model, RequestClass: string(key.RequestClass),
			Protocol: "responses", Transport: key.Transport, EndpointHash: key.EndpointHash,
			RoutingFingerprint: OpenAIRouteObservationFingerprint(key), Outcome: ReliabilityOutcomeSuccess, ObservedAt: observedAt,
		}})
		require.NoError(t, err)
	}
	require.Len(t, persisted, 2)
	evidence.running.Store(true)
	evidence.listEvidence = func(context.Context, *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		return persisted, nil
	}
	adapter := NewReliabilityEvidenceOpenAIRouteAdapter(evidence)
	t.Cleanup(adapter.Stop)
	result := adapter.refreshForTest(context.Background(), OpenAIRouteReliabilityEvidenceRequest{
		Keys: []OpenAIRouteKey{key}, GroupID: groupID, AccessGroupID: accessGroupID,
		Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "responses", Now: now,
	})
	require.True(t, result.Meta.Applied)
	require.Equal(t, uint64(2), result.Profiles[OpenAIRouteObservationFingerprint(key)].Global.SuccessCount)
}
func TestReliabilityEvidenceOpenAIRouteAdapterFallsBackWhenAnyRouteMissing(t *testing.T) {
	now := time.Now().UTC()
	groupID := int64(7)
	first, second := reliabilityAdapterTestKey(53, groupID, "gpt-5.6"), reliabilityAdapterTestKey(54, groupID, "gpt-5.6")
	observations := []*ReliabilityObservation{reliabilityAdapterObservation(first, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-4*time.Second)), reliabilityAdapterObservation(first, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-3*time.Second))}
	adapter := NewReliabilityEvidenceOpenAIRouteAdapter(newReliabilityAdapterEvidence(observations))
	result := adapter.refreshForTest(context.Background(), OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{first, second}, GroupID: groupID, Model: first.Model, RequestClass: first.RequestClass, InboundProtocol: "responses", Now: now})
	require.False(t, result.Meta.Applied)
	require.Equal(t, "route_evidence_incomplete", result.Meta.Reason)
	require.Empty(t, result.Profiles)
}
func TestReliabilityEvidenceOpenAIRouteAdapterCacheMissIsNeutralAndNonBlocking(t *testing.T) {
	now := time.Now().UTC()
	key := reliabilityAdapterTestKey(53, 7, "gpt-5.6")
	evidence := newReliabilityAdapterEvidence(nil)
	evidence.listEvidence = func(ctx context.Context, _ *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	adapter := NewReliabilityEvidenceOpenAIRouteAdapter(evidence)
	started := time.Now()
	result := adapter.Lookup(OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{key}, GroupID: 7, Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "responses", Now: now})
	require.False(t, result.Meta.Applied)
	require.Equal(t, "cache_warming", result.Meta.Reason)
	require.Less(t, time.Since(started), 10*time.Millisecond)
}
func TestOverlayOpenAIRouteReliabilityPreservesTTFTAndCost(t *testing.T) {
	base := OpenAIRouteObservationProfile{Global: NewOpenAIRouteObservationAggregate(), Recent: NewOpenAIRouteObservationAggregate()}
	base.Global.TTFTSampleCount, base.Global.ActualCostSamples, base.Global.ActualBaseCostUSD = 3, 2, 0.3
	base.Global.ReliabilityCount, base.Global.PartialStreams = 100, 1
	shared := OpenAIRouteObservationProfile{Global: NewOpenAIRouteObservationAggregate(), Recent: NewOpenAIRouteObservationAggregate()}
	shared.Global.ReliabilityCount, shared.Global.SuccessCount = 5, 4
	result := overlayOpenAIRouteReliability(base, shared)
	require.Equal(t, uint64(5), result.Global.ReliabilityCount)
	require.Equal(t, uint64(3), result.Global.TTFTSampleCount)
	require.Equal(t, uint64(2), result.Global.ActualCostSamples)
	require.Equal(t, uint64(1), result.Global.PartialStreams)
}

func TestReliabilityEvidenceOpenAIRouteAdapterDoesNotLeakHTTPProbeIntoWebSocketScope(t *testing.T) {
	now := time.Now().UTC()
	key := reliabilityAdapterTestKey(53, 7, "gpt-5.6")
	accountID := key.AccountID
	probe := func(at time.Time) *ReliabilityObservation {
		return &ReliabilityObservation{IdempotencyKey: at.String(), FactType: ReliabilityFactActiveProbe, AccountID: &accountID, Platform: PlatformOpenAI, Model: key.Model, RequestClass: string(key.RequestClass), Protocol: "http", Transport: key.Transport, EndpointHash: key.EndpointHash, Outcome: ReliabilityOutcomeSuccess, LatencyMs: 200, ObservedAt: at}
	}
	adapter := NewReliabilityEvidenceOpenAIRouteAdapter(newReliabilityAdapterEvidence([]*ReliabilityObservation{probe(now.Add(-4 * time.Second)), probe(now.Add(-3 * time.Second))}))
	result := adapter.refreshForTest(context.Background(), OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{key}, GroupID: 7, Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "websocket_responses", Now: now})
	require.False(t, result.Meta.Applied)
	require.Equal(t, "route_evidence_incomplete", result.Meta.Reason)
}

func TestReliabilityEvidenceOpenAIRouteAdapterRejectsCrossAccessGroupRouteAndTransportEvidence(t *testing.T) {
	now := time.Now().UTC()
	groupID := int64(7)
	key := reliabilityAdapterTestKey(53, groupID, "gpt-5.6")
	wrongAccess := reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-4*time.Second))
	wrongAccess.AccessGroupID = 101
	wrongRoute := reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-3*time.Second))
	wrongRoute.AccessGroupID = 202
	wrongRoute.RoutingFingerprint = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	wrongTransport := reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-2*time.Second))
	wrongTransport.AccessGroupID = 202
	wrongTransport.Transport = string(OpenAIUpstreamTransportResponsesWebsocketV2)
	adapter := NewReliabilityEvidenceOpenAIRouteAdapter(newReliabilityAdapterEvidence([]*ReliabilityObservation{wrongAccess, wrongRoute, wrongTransport}))
	t.Cleanup(adapter.Stop)

	result := adapter.refreshForTest(context.Background(), OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{key}, GroupID: groupID, AccessGroupID: 202, Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "responses", Now: now})
	require.False(t, result.Meta.Applied)
	require.Equal(t, "route_evidence_incomplete", result.Meta.Reason)

	valid := reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-time.Second))
	valid.AccessGroupID = 202
	valid2 := *valid
	valid2.IdempotencyKey += "-2"
	valid2.ObservedAt = now.Add(-500 * time.Millisecond)
	adapter.evidence.listEvidence = func(context.Context, *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		return []*ReliabilityObservation{valid, &valid2}, nil
	}
	result = adapter.refreshForTest(context.Background(), OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{key}, GroupID: groupID, AccessGroupID: 202, Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "responses", Now: now})
	require.True(t, result.Meta.Applied)
}

func TestReliabilityEvidenceOpenAIRouteAdapterCompletenessGatesAndRecoversAfterContaminatedWindow(t *testing.T) {
	for _, counter := range []string{"dropped", "failed"} {
		t.Run(counter, func(t *testing.T) {
			now := time.Now().UTC()
			groupID := int64(7)
			key := reliabilityAdapterTestKey(53, groupID, "gpt-5.6")
			evidence := newReliabilityAdapterEvidence(nil)
			evidence.listEvidence = func(_ context.Context, query *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
				first := reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, query.End.Add(-2*time.Second))
				second := reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, query.End.Add(-time.Second))
				return []*ReliabilityObservation{first, second}, nil
			}
			adapter := NewReliabilityEvidenceOpenAIRouteAdapter(evidence)
			t.Cleanup(adapter.Stop)
			if counter == "dropped" {
				evidence.dropped.Add(1)
			} else {
				evidence.failed.Add(1)
			}
			req := OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{key}, GroupID: groupID, Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "responses", Now: now}
			result := adapter.refreshForTest(context.Background(), req)
			require.Equal(t, "evidence_loss", result.Meta.Reason)
			req.Now = now.Add(openAIRouteReliabilityAdapterWindow + 2*openAIRouteReliabilityAdapterTailLag)
			result = adapter.refreshForTest(context.Background(), req)
			require.True(t, result.Meta.Applied)
		})
	}
}

func TestReliabilityEvidenceOpenAIRouteAdapterRejectsCollectorTOCTOUOldPendingTruncationAndStaleRoutes(t *testing.T) {
	now := time.Now().UTC()
	groupID := int64(7)
	key := reliabilityAdapterTestKey(53, groupID, "gpt-5.6")
	valid := []*ReliabilityObservation{
		reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-3*time.Second)),
		reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-2*time.Second)),
	}
	req := OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{key}, GroupID: groupID, Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "responses", Now: now}

	evidence := newReliabilityAdapterEvidence(valid)
	evidence.listEvidence = func(context.Context, *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		evidence.running.Store(false)
		return valid, nil
	}
	adapter := NewReliabilityEvidenceOpenAIRouteAdapter(evidence)
	require.Equal(t, "collector_unavailable", adapter.refreshForTest(context.Background(), req).Meta.Reason)
	adapter.Stop()

	evidence = newReliabilityAdapterEvidence(valid)
	pendingAt := now.Add(-time.Minute)
	evidence.publishedPendingID.Store(1)
	evidence.pendingObserved = map[uint64]time.Time{}
	evidence.pendingObserved[1] = pendingAt
	adapter = NewReliabilityEvidenceOpenAIRouteAdapter(evidence)
	require.Equal(t, "evidence_pending", adapter.refreshForTest(context.Background(), req).Meta.Reason)
	adapter.Stop()

	truncated := make([]*ReliabilityObservation, 5001)
	for index := range truncated {
		copy := *valid[index%2]
		copy.IdempotencyKey = fmt.Sprintf("truncated-%d", index)
		truncated[index] = &copy
	}
	adapter = NewReliabilityEvidenceOpenAIRouteAdapter(newReliabilityAdapterEvidence(truncated))
	require.Equal(t, "evidence_truncated", adapter.refreshForTest(context.Background(), req).Meta.Reason)
	adapter.Stop()

	stale := []*ReliabilityObservation{
		reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-10*time.Minute)),
		reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-9*time.Minute)),
	}
	adapter = NewReliabilityEvidenceOpenAIRouteAdapter(newReliabilityAdapterEvidence(stale))
	require.Equal(t, "route_evidence_incomplete", adapter.refreshForTest(context.Background(), req).Meta.Reason)
	adapter.Stop()
}

func TestReliabilityEvidenceOpenAIRouteAdapterBoundsAsyncRefreshesAndStopsWithEvidence(t *testing.T) {
	evidence := newReliabilityAdapterEvidence(nil)
	var active atomic.Int64
	var maximum atomic.Int64
	evidence.listEvidence = func(ctx context.Context, _ *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		current := active.Add(1)
		defer active.Add(-1)
		for {
			seen := maximum.Load()
			if current <= seen || maximum.CompareAndSwap(seen, current) {
				break
			}
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	adapter := NewReliabilityEvidenceOpenAIRouteAdapter(evidence)
	now := time.Now().UTC()
	for groupID := int64(1); groupID <= 12; groupID++ {
		key := reliabilityAdapterTestKey(53, groupID, "gpt-5.6")
		adapter.Lookup(OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{key}, GroupID: groupID, Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "responses", Now: now})
	}
	require.Eventually(t, func() bool { return active.Load() == openAIRouteReliabilityAdapterMaxRefreshes }, time.Second, time.Millisecond)
	require.Equal(t, int64(openAIRouteReliabilityAdapterMaxRefreshes), maximum.Load())
	require.NoError(t, evidence.Stop(context.Background()))
	require.Zero(t, active.Load())
	key := reliabilityAdapterTestKey(53, 99, "gpt-5.6")
	require.Equal(t, "adapter_stopped", adapter.Lookup(OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{key}, GroupID: 99, Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "responses", Now: now}).Meta.Reason)
}

func TestReliabilityEvidenceOpenAIRouteAdapterOlderAsyncLoadCannotOverwriteExplicitRefresh(t *testing.T) {
	now := time.Now().UTC()
	groupID := int64(7)
	key := reliabilityAdapterTestKey(53, groupID, "gpt-5.6")
	valid := []*ReliabilityObservation{
		reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-3*time.Second)),
		reliabilityAdapterObservation(key, &groupID, "responses", ReliabilityOutcomeSuccess, now.Add(-2*time.Second)),
	}
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	var calls atomic.Int64
	evidence := newReliabilityAdapterEvidence(nil)
	evidence.listEvidence = func(ctx context.Context, _ *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		if calls.Add(1) == 1 {
			close(firstEntered)
			select {
			case <-releaseFirst:
				return nil, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return valid, nil
	}
	adapter := NewReliabilityEvidenceOpenAIRouteAdapter(evidence)
	t.Cleanup(adapter.Stop)
	req := OpenAIRouteReliabilityEvidenceRequest{Keys: []OpenAIRouteKey{key}, GroupID: groupID, Model: key.Model, RequestClass: key.RequestClass, InboundProtocol: "responses", Now: now}
	require.Equal(t, "cache_warming", adapter.Lookup(req).Meta.Reason)
	<-firstEntered
	require.True(t, adapter.refreshForTest(context.Background(), req).Meta.Applied)
	close(releaseFirst)
	require.Eventually(t, func() bool { return calls.Load() >= 2 }, time.Second, time.Millisecond)
	require.True(t, adapter.Lookup(req).Meta.Applied)
}

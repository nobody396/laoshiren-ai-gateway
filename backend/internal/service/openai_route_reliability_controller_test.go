package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type openAIRouteReliabilityAdapterStub struct {
	result OpenAIRouteReliabilityEvidenceResult
	calls  int
}

func (s *openAIRouteReliabilityAdapterStub) Lookup(OpenAIRouteReliabilityEvidenceRequest) OpenAIRouteReliabilityEvidenceResult {
	s.calls++
	return cloneOpenAIRouteReliabilityResult(s.result)
}

func reliabilityControllerProfile(successes uint64) OpenAIRouteObservationProfile {
	aggregate := NewOpenAIRouteObservationAggregate()
	aggregate.AttemptCount, aggregate.ReliabilityCount, aggregate.SuccessCount, aggregate.FailureCount = 100, 100, successes, 100-successes
	aggregate.LatencySampleCount = 100
	aggregate.LatencyHistogram[OpenAIRouteLatencyHistogramBucket(500)] = 100
	return OpenAIRouteObservationProfile{Global: aggregate, Recent: aggregate}
}

func TestOpenAIRouteControllerReliabilityAdapterFallbackPreservesExistingShadowProfile(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":31,"activation_id":"activation-31","shadow_started_at":"2026-08-01T00:00:00Z",
		"target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,
		"reliability_evidence_enabled":true
	}]`}
	account := testOpenAIRouteControllerAccount(1, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	store := &openAIRouteObservationStoreStub{profiles: map[string]OpenAIRouteObservationProfile{OpenAIRouteObservationFingerprint(key): reliabilityControllerProfile(90)}}
	adapter := &openAIRouteReliabilityAdapterStub{result: openAIRouteReliabilityFallback("cache_warming")}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, store)
	controller.SetReliabilityEvidenceAdapter(adapter)
	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{GroupID: 7, Model: key.Model, InboundProtocol: APIProtocolResponses, RequestClass: key.RequestClass, Candidates: []OpenAIRouteShadowCandidate{{Account: account, Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE)}}})
	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.False(t, decision.Audit.ReliabilityEvidenceAdapterApplied)
	require.Equal(t, "cache_warming", decision.Audit.ReliabilityEvidenceAdapterReason)
	require.Equal(t, uint64(100), decision.Audit.Candidates[0].ObservationSamples)
	require.Equal(t, "shared", decision.Audit.Candidates[0].ObservationSource)
}

func TestOpenAIRouteControllerReliabilityAdapterFailsNeutralForDeferredWebSocketScope(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow","policy_version":35,"activation_id":"activation-35","shadow_started_at":"2026-08-01T00:00:00Z","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,"reliability_evidence_enabled":true}]`}
	account := testOpenAIRouteControllerAccount(1, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportResponsesWebsocketV2))
	require.NoError(t, err)
	adapter := &openAIRouteReliabilityAdapterStub{result: OpenAIRouteReliabilityEvidenceResult{Meta: OpenAIRouteReliabilityEvidenceMeta{Applied: true}}}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, &openAIRouteObservationStoreStub{profiles: map[string]OpenAIRouteObservationProfile{OpenAIRouteObservationFingerprint(key): reliabilityControllerProfile(99)}})
	controller.SetReliabilityEvidenceAdapter(adapter)

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{GroupID: 7, Model: key.Model, InboundProtocol: "websocket_responses", RequestClass: key.RequestClass, Candidates: []OpenAIRouteShadowCandidate{{Account: account, Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportResponsesWebsocketV2)}}})

	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.Equal(t, "inbound_protocol_not_supported", decision.Audit.ReliabilityEvidenceAdapterReason)
	require.False(t, decision.Audit.ReliabilityEvidenceAdapterApplied)
	require.Zero(t, adapter.calls)
}

func TestOpenAIRouteControllerReliabilityAdapterOverlaysShadowScoringAndKeepsComparison(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{
		"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow",
		"policy_version":32,"activation_id":"activation-32","shadow_started_at":"2026-08-01T00:00:00Z",
		"target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,
		"reliability_evidence_enabled":true
	}]`}
	account := testOpenAIRouteControllerAccount(1, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	store := &openAIRouteObservationStoreStub{profiles: map[string]OpenAIRouteObservationProfile{OpenAIRouteObservationFingerprint(key): reliabilityControllerProfile(20)}}
	shared := reliabilityControllerProfile(99)
	adapter := &openAIRouteReliabilityAdapterStub{result: OpenAIRouteReliabilityEvidenceResult{Profiles: map[string]OpenAIRouteObservationProfile{OpenAIRouteObservationFingerprint(key): shared}, Meta: OpenAIRouteReliabilityEvidenceMeta{Applied: true, Reason: "reliability_evidence_ready", SampleCount: 100, PendingCutoffID: 44}}}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, store)
	controller.SetReliabilityEvidenceAdapter(adapter)
	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{GroupID: 7, Model: key.Model, InboundProtocol: APIProtocolResponses, RequestClass: key.RequestClass, Candidates: []OpenAIRouteShadowCandidate{{Account: account, Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE)}}})
	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.True(t, decision.Audit.ReliabilityEvidenceAdapterApplied)
	require.Equal(t, uint64(44), decision.Audit.ReliabilityEvidencePendingCutoffID)
	candidate := decision.Audit.Candidates[0]
	require.True(t, candidate.ReliabilityEvidenceApplied)
	require.Equal(t, uint64(100), candidate.LegacyObservationSamples)
	require.Equal(t, uint64(100), candidate.ReliabilityEvidenceSamples)
	require.Greater(t, candidate.ReliabilityEvidenceSuccessLowerBound, candidate.LegacySuccessLowerBound)
	require.Equal(t, "reliability_evidence", candidate.ObservationSource)
	require.Equal(t, OpenAIRoutePolicyShadow, decision.Mode, "adapter must never authorize Enforce")
}

func TestOpenAIRouteControllerReliabilityAdapterPreservesExactLegacyPartialStreamRate(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `[{"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow","policy_version":34,"activation_id":"activation-34","shadow_started_at":"2026-08-01T00:00:00Z","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,"reliability_evidence_enabled":true}]`}
	account := testOpenAIRouteControllerAccount(1, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	legacy := reliabilityControllerProfile(99)
	legacy.Global.PartialStreams, legacy.Recent.PartialStreams = 1, 1
	sharedAggregate := NewOpenAIRouteObservationAggregate()
	sharedAggregate.AttemptCount, sharedAggregate.ReliabilityCount, sharedAggregate.SuccessCount = 5, 5, 5
	shared := OpenAIRouteObservationProfile{Global: sharedAggregate, Recent: sharedAggregate}
	adapter := &openAIRouteReliabilityAdapterStub{result: OpenAIRouteReliabilityEvidenceResult{Profiles: map[string]OpenAIRouteObservationProfile{OpenAIRouteObservationFingerprint(key): shared}, Meta: OpenAIRouteReliabilityEvidenceMeta{Applied: true, Reason: "reliability_evidence_ready", SampleCount: 5}}}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, &openAIRouteObservationStoreStub{profiles: map[string]OpenAIRouteObservationProfile{OpenAIRouteObservationFingerprint(key): legacy}})
	controller.SetReliabilityEvidenceAdapter(adapter)

	decision, err := controller.EvaluateShadow(context.Background(), OpenAIRouteShadowRequest{GroupID: 7, Model: key.Model, InboundProtocol: APIProtocolResponses, RequestClass: key.RequestClass, Candidates: []OpenAIRouteShadowCandidate{{Account: account, Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE)}}})

	require.NoError(t, err)
	require.True(t, decision.Evaluated)
	require.InDelta(t, 0.01, decision.Audit.Candidates[0].PartialStreamRate, 1e-12)
}

func TestOpenAIRouteControllerBatchKeepsReliabilityAdapterTreatmentIsolated(t *testing.T) {
	reader := &openAIRoutePolicyReaderStub{value: `{"policies":[
		{"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow","policy_version":33,"activation_id":"activation-33","shadow_started_at":"2026-08-01T00:00:00Z","experiment_id":"rel-evidence","variant_id":"control","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01},
		{"group_id":7,"model":"gpt-5.6-sol","request_class":"text","enabled":true,"mode":"shadow","policy_version":33,"activation_id":"activation-33","shadow_started_at":"2026-08-01T00:00:00Z","experiment_id":"rel-evidence","variant_id":"adapter","target_avg_multiplier":0.30,"hard_avg_multiplier":0.30,"estimated_base_cost_usd":0.01,"reliability_evidence_enabled":true}
	]}`}
	account := testOpenAIRouteControllerAccount(1, 0.15)
	key, err := NewOpenAIRouteKey(account, 7, "gpt-5.6-sol", OpenAIRouteRequestClassText, "https://example.invalid/v1/responses", string(OpenAIUpstreamTransportHTTPSSE))
	require.NoError(t, err)
	adapter := &openAIRouteReliabilityAdapterStub{result: OpenAIRouteReliabilityEvidenceResult{Profiles: map[string]OpenAIRouteObservationProfile{OpenAIRouteObservationFingerprint(key): reliabilityControllerProfile(99)}, Meta: OpenAIRouteReliabilityEvidenceMeta{Applied: true, Reason: "reliability_evidence_ready", SampleCount: 100}}}
	controller := NewOpenAIRouteController(reader, &openAIRouteHealthStoreStub{}, &openAIRouteBudgetSnapshotStoreStub{}, &openAIRouteObservationStoreStub{profiles: map[string]OpenAIRouteObservationProfile{OpenAIRouteObservationFingerprint(key): reliabilityControllerProfile(50)}})
	controller.SetReliabilityEvidenceAdapter(adapter)
	decisions, err := controller.EvaluateShadows(context.Background(), OpenAIRouteShadowRequest{GroupID: 7, Model: key.Model, InboundProtocol: APIProtocolResponses, RequestClass: key.RequestClass, Candidates: []OpenAIRouteShadowCandidate{{Account: account, Endpoint: "https://example.invalid/v1/responses", Transport: string(OpenAIUpstreamTransportHTTPSSE)}}})
	require.NoError(t, err)
	require.Len(t, decisions, 2)
	require.Equal(t, 1, adapter.calls)
	require.False(t, decisions[0].Audit.ReliabilityEvidenceAdapterEnabled)
	require.True(t, decisions[1].Audit.ReliabilityEvidenceAdapterEnabled)
	require.True(t, decisions[1].Audit.ReliabilityEvidenceAdapterApplied)
}

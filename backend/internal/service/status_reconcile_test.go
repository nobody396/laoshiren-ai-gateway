package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProjectStatusObservationsPrefersExplicitGroupBinding(t *testing.T) {
	groupID, userID := int64(40), int64(92)
	products := []statusProductDefinition{
		{ID: 1, Code: "openai-codex-api", Components: []statusComponentDefinition{{ID: 11, AccessMode: "http", Bindings: []statusBinding{{Platform: PlatformOpenAI}}}}},
		{ID: 2, Code: "builder-pass-gpt", Components: []statusComponentDefinition{{ID: 22, AccessMode: "http", Bindings: []statusBinding{{GroupID: &groupID}}}}},
	}
	observed := time.Now()
	projection := projectStatusObservations(products, []*ReliabilityObservation{{
		FactType: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure,
		CustomerImpact: true, UserID: &userID, GroupID: &groupID, Platform: PlatformOpenAI, ObservedAt: observed,
	}})

	require.Empty(t, projection[11], "group-bound Builder Pass traffic must not leak into generic OpenAI status")
	require.Len(t, projection[22], 1)
}

func TestProjectStatusObservationsFallsBackToPlatformBinding(t *testing.T) {
	products := []statusProductDefinition{{ID: 1, Components: []statusComponentDefinition{{ID: 11, AccessMode: "http", Bindings: []statusBinding{{Platform: PlatformOpenAI}}}}}}
	projection := projectStatusObservations(products, []*ReliabilityObservation{{
		FactType: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess,
		Platform: PlatformOpenAI, ObservedAt: time.Now(),
	}})
	require.Len(t, projection[11], 1)
}

func TestProjectStatusObservationsFansOneRouteProbeToEveryDependentComponent(t *testing.T) {
	accountID := int64(77)
	route := "0123456789abcdef0123456789abcdef"
	dependent := statusBinding{RouteFingerprint: route, AccountIDs: map[int64]struct{}{accountID: {}}}
	products := []statusProductDefinition{
		{ID: 1, Components: []statusComponentDefinition{{ID: 11, AccessMode: "http", Bindings: []statusBinding{dependent}}}},
		{ID: 2, Components: []statusComponentDefinition{{ID: 22, AccessMode: "http", Bindings: []statusBinding{dependent}}}},
	}
	projection := projectStatusObservations(products, []*ReliabilityObservation{{
		FactType: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, AccountID: &accountID,
		Platform: PlatformOpenAI, Model: "gpt-5.6", Protocol: "http", RouteFingerprint: route, ObservedAt: time.Now(),
	}})
	require.Len(t, projection[11], 1)
	require.Len(t, projection[22], 1)
}

func TestProjectStatusObservationsFansGroupRouteProbeToBuilderPassAndGenericAPI(t *testing.T) {
	accountID := int64(77)
	groupID := int64(40)
	products := []statusProductDefinition{
		{ID: 1, Components: []statusComponentDefinition{{ID: 11, AccessMode: "http", Bindings: []statusBinding{{Platform: PlatformOpenAI}}}}},
		{ID: 2, Components: []statusComponentDefinition{{ID: 22, AccessMode: "http", Bindings: []statusBinding{{GroupID: &groupID, AccountIDs: map[int64]struct{}{accountID: {}}}}}}},
	}
	projection := projectStatusObservations(products, []*ReliabilityObservation{{
		FactType: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, AccountID: &accountID,
		Platform: PlatformOpenAI, Model: "gpt-5.6", Protocol: "http", ObservedAt: time.Now(),
	}})
	require.Len(t, projection[11], 1)
	require.Len(t, projection[22], 1)
}

func TestProjectStatusObservationsExcludesAttemptsAndExcludedProbeFromAvailability(t *testing.T) {
	products := []statusProductDefinition{{ID: 1, Components: []statusComponentDefinition{{ID: 11, AccessMode: "http", Bindings: []statusBinding{{Platform: PlatformOpenAI}}}}}}
	projection := projectStatusObservations(products, []*ReliabilityObservation{
		{FactType: ReliabilityFactUpstreamAttempt, Outcome: ReliabilityOutcomeFailure, Platform: PlatformOpenAI, ObservedAt: time.Now()},
		{FactType: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeExcluded, Platform: PlatformOpenAI, ObservedAt: time.Now()},
		{FactType: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeSuccess, Platform: PlatformOpenAI, ObservedAt: time.Now()},
	})
	require.Len(t, projection[11], 1)
	evaluation := EvaluateComputedStatus(StatusEvaluationInput{Now: time.Now(), Observations: projection[11]})
	require.Equal(t, 1, evaluation.CustomerRequestCount)
	require.Zero(t, evaluation.ProbeCount)
}

func TestProjectStatusObservationsDoesNotLetDirectDiagnosticMaskGatewayProbeFailure(t *testing.T) {
	products := []statusProductDefinition{{ID: 1, Components: []statusComponentDefinition{{ID: 11, AccessMode: "http", Bindings: []statusBinding{{Platform: PlatformOpenAI}}}}}}
	now := time.Now()
	projection := projectStatusObservations(products, []*ReliabilityObservation{
		{FactType: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, Platform: PlatformOpenAI, Protocol: "http", ObservedAt: now.Add(-time.Second)},
		{FactType: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeSuccess, Platform: PlatformOpenAI, Protocol: "http_direct", ObservedAt: now},
	})
	require.Len(t, projection[11], 1)
	require.Equal(t, ReliabilityOutcomeFailure, projection[11][0].Outcome)
}

func TestProjectStatusObservationsNeverPublishesDirectDiagnosticThroughExplicitRouteBinding(t *testing.T) {
	route := "0123456789abcdef0123456789abcdef"
	products := []statusProductDefinition{{ID: 1, Components: []statusComponentDefinition{{ID: 11, AccessMode: "http", Bindings: []statusBinding{{RouteFingerprint: route}}}}}}
	observation := &ReliabilityObservation{FactType: ReliabilityFactActiveProbe, Outcome: ReliabilityOutcomeFailure, Platform: PlatformOpenAI, Protocol: "http_direct", RouteFingerprint: route, ObservedAt: time.Now()}
	require.Empty(t, projectStatusObservations(products, []*ReliabilityObservation{observation}))
	require.Empty(t, matchingStatusComponentIDs(products, observation, true), "operator monitoring must keep direct diagnostics in the non-status section")
}

func TestProjectStatusObservationsExcludesUnpublishedWebSocketFromHTTPComponent(t *testing.T) {
	products := []statusProductDefinition{{ID: 1, Components: []statusComponentDefinition{{ID: 11, AccessMode: "http", Bindings: []statusBinding{{Platform: PlatformOpenAI}}}}}}
	projection := projectStatusObservations(products, []*ReliabilityObservation{{
		FactType: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure, CustomerImpact: true,
		Platform: PlatformOpenAI, Protocol: "websocket_responses", ObservedAt: time.Now(),
	}})
	require.Empty(t, projection[11])
}

func TestProjectStatusObservationsExcludesLegacyUnattributedCustomerImpact(t *testing.T) {
	products := []statusProductDefinition{{
		ID: 1,
		Components: []statusComponentDefinition{{
			ID: 11, AccessMode: "http", Bindings: []statusBinding{{Platform: PlatformGemini}},
		}},
	}}
	observation := &ReliabilityObservation{
		FactType: ReliabilityFactCustomerRequest, Outcome: ReliabilityOutcomeFailure,
		CustomerImpact: true, Platform: PlatformGemini, StatusCode: intPointer(401), ObservedAt: time.Now(),
	}

	require.Empty(t, projectStatusObservations(products, []*ReliabilityObservation{observation}),
		"historical unowned failures must never change a customer-facing product status")
}

func TestStatusEvidenceReadinessUsesCurrentLifecycleEpoch(t *testing.T) {
	now := time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
	svc := &StatusControlService{readinessBaseline: ReliabilityEvidenceCompleteness{Dropped: 4, Failed: 2}}
	newPending := now.Add(-500 * time.Millisecond)
	current := ReliabilityEvidenceCompleteness{Enabled: true, Running: true, Enqueued: 12, Processed: 10, QueueDepth: 1, InFlight: 1, Dropped: 4, Failed: 2, OldestPendingAt: &newPending}
	require.True(t, svc.evidenceReadyAt(current, now), "historical failures before this evaluator epoch must not freeze all products")
	require.True(t, svc.evidenceReadyAt(current, now), "bounded queue activity without loss must use the lagged watermark rather than reset every product")
	oldPending := now.Add(-2 * time.Second)
	current.OldestPendingAt = &oldPending
	require.False(t, svc.evidenceReadyAt(current, now), "pending evidence older than the query watermark must fail closed")
	current.OldestPendingAt = &newPending
	current.Failed++
	require.False(t, svc.evidenceReadyAt(current, now), "a new failure in the current evaluator epoch must fail closed")
	svc.readinessBaseline = current
	svc.incompleteUntil = now.Add(5 * time.Minute)
	require.False(t, svc.evidenceReadyAt(current, now.Add(30*time.Second)), "lost evidence must fail closed for the full evaluation window")
	current.OldestPendingAt = nil
	require.True(t, svc.evidenceReadyAt(current, now.Add(5*time.Minute)), "a clean epoch may recover after the contaminated window expires")
}

func TestNextMonitoringSinceResetsToLatestRelapseWithoutSlidingEveryTick(t *testing.T) {
	previous := time.Date(2026, 8, 23, 1, 0, 0, 0, time.UTC)
	relapse := previous.Add(4 * time.Minute)
	now := previous.Add(5 * time.Minute)
	require.Equal(t, relapse, nextMonitoringSince(ServiceStatusMonitoring, previous, ServiceStatusMonitoring, relapse, now))
	require.Equal(t, relapse, nextMonitoringSince(ServiceStatusMonitoring, relapse, ServiceStatusMonitoring, relapse, now.Add(30*time.Second)))
}

func TestLoadStatusEvidencePartitionsQueriesSoHighVolumeProductCannotStarveSparseProduct(t *testing.T) {
	now := time.Now()
	queries := []*ReliabilityEvidenceQuery{}
	evidence := NewReliabilityEvidenceService(nil, nil)
	evidence.enabled = true
	evidence.running.Store(true)
	evidence.listEvidence = func(_ context.Context, query *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		copyQuery := *query
		queries = append(queries, &copyQuery)
		if len(query.AnyPlatforms) == 1 && query.AnyPlatforms[0] == PlatformOpenAI {
			items := make([]*ReliabilityObservation, 0, 5000)
			for index := 0; index < 5000; index++ {
				items = append(items, &ReliabilityObservation{IdempotencyKey: fmt.Sprintf("openai-%d", index), FactType: ReliabilityFactCustomerRequest, Platform: PlatformOpenAI, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-2 * time.Second)})
			}
			return items, nil
		}
		return []*ReliabilityObservation{{IdempotencyKey: "claude-probe", FactType: ReliabilityFactActiveProbe, Platform: PlatformAnthropic, Outcome: ReliabilityOutcomeSuccess, ObservedAt: now.Add(-2 * time.Second)}}, nil
	}
	svc := &StatusControlService{evidence: evidence}
	products := []statusProductDefinition{
		{ID: 1, Components: []statusComponentDefinition{{ID: 11, Bindings: []statusBinding{{Platform: PlatformOpenAI}}}}},
		{ID: 2, Components: []statusComponentDefinition{{ID: 22, Bindings: []statusBinding{{Platform: PlatformAnthropic}}}}},
	}
	snapshot, err := svc.loadStatusEvidence(context.Background(), products, now.Add(-5*time.Minute), now.Add(-time.Second))
	require.NoError(t, err)
	require.Len(t, queries, 2)
	require.Equal(t, []string{PlatformOpenAI}, queries[0].AnyPlatforms)
	require.Equal(t, []string{PlatformAnthropic}, queries[1].AnyPlatforms)
	require.Len(t, snapshot.Observations, 5001)
}

func TestLoadStatusEvidencePartitionsSharedPlatformByComponentModelPattern(t *testing.T) {
	now := time.Now()
	patterns := [][]string{}
	evidence := NewReliabilityEvidenceService(nil, nil)
	evidence.enabled = true
	evidence.running.Store(true)
	evidence.listEvidence = func(_ context.Context, query *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		patterns = append(patterns, append([]string(nil), query.AnyModelPatterns...))
		return nil, nil
	}
	svc := &StatusControlService{evidence: evidence}
	products := []statusProductDefinition{{ID: 1, Components: []statusComponentDefinition{
		{ID: 11, ModelPattern: "gpt-*", Bindings: []statusBinding{{Platform: PlatformOpenAI}}},
		{ID: 12, ModelPattern: "o3-*", Bindings: []statusBinding{{Platform: PlatformOpenAI}}},
	}}}
	_, err := svc.loadStatusEvidence(context.Background(), products, now.Add(-5*time.Minute), now.Add(-time.Second))
	require.NoError(t, err)
	require.Equal(t, [][]string{{"gpt-*"}, {"o3-*"}}, patterns)
}

func TestLoadStatusEvidenceConservativelyMergesPendingWatermarkAcrossComponents(t *testing.T) {
	now := time.Now()
	oldPending := now.Add(-2 * time.Second)
	evidence := NewReliabilityEvidenceService(nil, nil)
	evidence.enabled = true
	evidence.running.Store(true)
	evidence.pendingObserved = map[uint64]time.Time{1: oldPending}
	evidence.publishedPendingID.Store(1)
	queryCount := 0
	evidence.listEvidence = func(_ context.Context, _ *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		queryCount++
		if queryCount == 1 {
			evidence.pendingMu.Lock()
			delete(evidence.pendingObserved, 1) // commit/untrack between SELECT and post-query watermark
			evidence.pendingMu.Unlock()
			// A later A-component job fully publishes and commits before the B
			// query. The whole catalog load must still use the original cutoff=1.
			evidence.nextPendingID.Store(2)
			evidence.publishedPendingID.Store(2)
		}
		return nil, nil
	}
	svc := &StatusControlService{evidence: evidence}
	products := []statusProductDefinition{{ID: 1, Components: []statusComponentDefinition{
		{ID: 11, Bindings: []statusBinding{{Platform: PlatformOpenAI}}},
		{ID: 12, Bindings: []statusBinding{{Platform: PlatformAnthropic}}},
	}}}
	snapshot, err := svc.loadStatusEvidence(context.Background(), products, now.Add(-5*time.Minute), now.Add(-time.Second))
	require.NoError(t, err)
	require.Equal(t, 2, queryCount)
	require.NotNil(t, snapshot.Completeness.OldestPendingAt)
	require.Equal(t, oldPending, *snapshot.Completeness.OldestPendingAt)
	require.Equal(t, uint64(1), snapshot.Completeness.PendingCutoffID)
}

func TestLoadStatusEvidenceUsesOnePendingCutoffAcrossEveryComponent(t *testing.T) {
	now := time.Now()
	evidence := NewReliabilityEvidenceService(nil, nil)
	evidence.enabled = true
	evidence.running.Store(true)
	evidence.nextPendingID.Store(1)
	evidence.publishedPendingID.Store(1)
	queryCount := 0
	evidence.listEvidence = func(_ context.Context, _ *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		queryCount++
		if queryCount == 1 {
			// A new A-component fact publishes and durably completes between A
			// and B queries. It belongs to the next catalog snapshot, not half of
			// this one.
			evidence.nextPendingID.Store(2)
			evidence.publishedPendingID.Store(2)
		}
		return nil, nil
	}
	svc := &StatusControlService{evidence: evidence}
	products := []statusProductDefinition{{ID: 1, Components: []statusComponentDefinition{
		{ID: 11, Bindings: []statusBinding{{Platform: PlatformOpenAI}}},
		{ID: 12, Bindings: []statusBinding{{Platform: PlatformAnthropic}}},
	}}}
	snapshot, err := svc.loadStatusEvidence(context.Background(), products, now.Add(-5*time.Minute), now.Add(-time.Second))
	require.NoError(t, err)
	require.Equal(t, 2, queryCount)
	require.Equal(t, uint64(1), snapshot.Completeness.PendingCutoffID)
}

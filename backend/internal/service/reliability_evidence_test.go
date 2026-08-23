package service

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type reliabilityRepoStub struct {
	batch func(context.Context, []*ReliabilityObservation) (int64, error)
	list  func(context.Context, *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error)
	claim func(context.Context, *ReliabilityProbeClaim) (bool, error)
}

func (r *reliabilityRepoStub) TryClaimReliabilityProbe(ctx context.Context, claim *ReliabilityProbeClaim) (bool, error) {
	if r.claim != nil {
		return r.claim(ctx, claim)
	}
	return true, nil
}

func newReliabilityEvidenceForTest(enabled bool, repo *reliabilityRepoStub) *ReliabilityEvidenceService {
	if repo == nil {
		repo = &reliabilityRepoStub{}
	}
	return &ReliabilityEvidenceService{
		enabled:      enabled,
		batchInsert:  repo.BatchInsertReliabilityObservations,
		listEvidence: repo.ListReliabilityObservations,
		claimProbe:   repo.TryClaimReliabilityProbe,
	}
}

func (r *reliabilityRepoStub) BatchInsertReliabilityObservations(ctx context.Context, inputs []*ReliabilityObservation) (int64, error) {
	if r.batch != nil {
		return r.batch(ctx, inputs)
	}
	return int64(len(inputs)), nil
}

func (r *reliabilityRepoStub) ListReliabilityObservations(ctx context.Context, query *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
	if r.list != nil {
		return r.list(ctx, query)
	}
	return []*ReliabilityObservation{}, nil
}

func TestRecordReliabilityObservationBatchPersistsNormalizedFactsWhenEnabled(t *testing.T) {
	var persisted []*ReliabilityObservation
	repo := &reliabilityRepoStub{
		batch: func(_ context.Context, inputs []*ReliabilityObservation) (int64, error) {
			persisted = append(persisted, inputs...)
			return int64(len(inputs)), nil
		},
	}
	svc := newReliabilityEvidenceForTest(true, repo)

	inserted, err := svc.recordFinalOutcomes(context.Background(), []*ReliabilityFinalOutcome{
		{
			RequestIdentity: " req-1 ",
			RequestID:       " req-1 ",
			Platform:        " openai ",
			Model:           " gpt-5.6-sol ",
			RequestClass:    ReliabilityRequestClassText,
			Protocol:        " responses ",
			Outcome:         ReliabilityOutcomeFailure,
			ErrorOwner:      " provider ",
			StatusCode:      intPointer(503),
			LatencyMs:       1250,
		},
	})

	require.NoError(t, err)
	require.Equal(t, int64(1), inserted)
	require.Len(t, persisted, 1)
	require.Equal(t, "customer:req-1", persisted[0].IdempotencyKey)
	require.Equal(t, "gateway_final", persisted[0].Source)
	require.Equal(t, "openai", persisted[0].Platform)
	require.Equal(t, "gpt-5.6-sol", persisted[0].Model)
	require.Equal(t, "provider", persisted[0].ErrorOwner)
	require.True(t, persisted[0].CustomerImpact)
	require.False(t, persisted[0].ObservedAt.IsZero())
}

func TestRecordProbeOutcomesCanNeverBecomeCustomerImpact(t *testing.T) {
	var persisted []*ReliabilityObservation
	repo := &reliabilityRepoStub{
		batch: func(_ context.Context, inputs []*ReliabilityObservation) (int64, error) {
			persisted = append(persisted, inputs...)
			return int64(len(inputs)), nil
		},
	}
	svc := newReliabilityEvidenceForTest(true, repo)

	_, err := svc.recordProbeOutcomes(context.Background(), []*ReliabilityProbeOutcome{
		{
			ProbeIdentity: "1", Outcome: ReliabilityOutcomeFailure, ObservedAt: time.Now(),
		},
	})

	require.NoError(t, err)
	require.Len(t, persisted, 1)
	require.False(t, persisted[0].CustomerImpact)
	require.Equal(t, ReliabilityFactActiveProbe, persisted[0].FactType)
}

func TestRecordAttemptOutcomePersistsExactRoutingIdentity(t *testing.T) {
	var persisted []*ReliabilityObservation
	repo := &reliabilityRepoStub{batch: func(_ context.Context, inputs []*ReliabilityObservation) (int64, error) {
		persisted = append(persisted, inputs...)
		return int64(len(inputs)), nil
	}}
	svc := newReliabilityEvidenceForTest(true, repo)
	groupID, accountID := int64(7), int64(53)
	_, err := svc.recordAttemptOutcomes(context.Background(), []*ReliabilityAttemptOutcome{{
		RequestIdentity: "req-route", AttemptIdentity: "1", GroupID: &groupID, AccessGroupID: 90, AccountID: &accountID,
		Platform: PlatformOpenAI, Model: "gpt-5.6-sol", RequestClass: ReliabilityRequestClassText,
		Protocol: "responses", Transport: "http_sse", EndpointHash: "0123456789abcdef",
		RoutingFingerprint: "0123456789abcdef0123456789abcdef", Outcome: ReliabilityOutcomeSuccess, ObservedAt: time.Now(),
	}})
	require.NoError(t, err)
	require.Len(t, persisted, 1)
	require.Equal(t, int64(90), persisted[0].AccessGroupID)
	require.Equal(t, "http_sse", persisted[0].Transport)
	require.Equal(t, "0123456789abcdef", persisted[0].EndpointHash)
	require.Equal(t, "0123456789abcdef0123456789abcdef", persisted[0].RoutingFingerprint)
}

func TestRecordReliabilityObservationBatchIsDisabledByDefault(t *testing.T) {
	called := false
	repo := &reliabilityRepoStub{
		batch: func(_ context.Context, _ []*ReliabilityObservation) (int64, error) {
			called = true
			return 0, nil
		},
	}
	svc := newReliabilityEvidenceForTest(false, repo)

	inserted, err := svc.recordFinalOutcomes(context.Background(), []*ReliabilityFinalOutcome{{
		RequestIdentity: "req-1", Outcome: ReliabilityOutcomeSuccess, ObservedAt: time.Now(),
	}})

	require.NoError(t, err)
	require.Zero(t, inserted)
	require.False(t, called)
}

func TestReliabilityObservationContractHasNoSensitivePayloadFields(t *testing.T) {
	typeOf := reflect.TypeOf(ReliabilityObservation{})
	forbidden := map[string]bool{
		"Prompt": true, "ResponseBody": true, "Credential": true,
		"RawURL": true, "APIKey": true, "AccountName": true, "UserEmail": true,
	}
	for index := 0; index < typeOf.NumField(); index++ {
		require.False(t, forbidden[typeOf.Field(index).Name], "sensitive field %s must not enter Reliability Observation", typeOf.Field(index).Name)
	}
}

func TestReliabilityEvidenceSnapshotSerializationExcludesSensitivePayloads(t *testing.T) {
	payload, err := json.Marshal(ReliabilityEvidenceSnapshot{Observations: []*ReliabilityObservation{{
		IdempotencyKey: "customer:req-1", Source: "gateway_final", Model: "gpt-5.6-sol",
	}}})
	require.NoError(t, err)
	serialized := strings.ToLower(string(payload))
	for _, forbidden := range []string{"prompt", "response_body", "credential", "raw_url", "api_key", "account_name", "user_email"} {
		require.NotContains(t, serialized, forbidden)
	}
}

func TestGetReliabilityEvidenceSnapshotUsesBoundedQuery(t *testing.T) {
	start := time.Date(2026, 8, 22, 4, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	repo := &reliabilityRepoStub{
		list: func(_ context.Context, query *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
			require.Equal(t, start, query.Start)
			require.Equal(t, end, query.End)
			require.Equal(t, 5001, query.Limit)
			return []*ReliabilityObservation{{IdempotencyKey: "customer:req-1"}}, nil
		},
	}
	svc := newReliabilityEvidenceForTest(true, repo)
	svc.enqueued.Store(1)
	svc.processed.Store(1)
	svc.written.Store(1)
	svc.running.Store(true)

	snapshot, err := svc.Snapshot(context.Background(), &ReliabilityEvidenceQuery{
		Start: start, End: end, Limit: 99999,
	})

	require.NoError(t, err)
	require.Len(t, snapshot.Observations, 1)
	require.False(t, snapshot.GeneratedAt.IsZero())
	require.True(t, snapshot.Completeness.Ready)
	require.Equal(t, int64(1), snapshot.Completeness.Written)
}

func TestGetReliabilityEvidenceSnapshotRejectsInvalidWindow(t *testing.T) {
	svc := newReliabilityEvidenceForTest(true, &reliabilityRepoStub{})
	start := time.Now()
	_, err := svc.Snapshot(context.Background(), &ReliabilityEvidenceQuery{Start: start, End: start})
	require.ErrorContains(t, err, "window")
}

func TestGetReliabilityEvidenceSnapshotNormalizesAnySelectors(t *testing.T) {
	now := time.Now()
	repo := &reliabilityRepoStub{list: func(_ context.Context, query *ReliabilityEvidenceQuery) ([]*ReliabilityObservation, error) {
		require.Equal(t, []int64{7}, query.AnyGroupIDs)
		require.Equal(t, []int64{53}, query.AnyAccountIDs)
		require.Equal(t, []string{"openai"}, query.AnyPlatforms)
		require.Equal(t, []string{"0123456789abcdef0123456789abcdef"}, query.AnyRouteFingerprints)
		require.Equal(t, []string{"gpt-*"}, query.AnyModelPatterns)
		return nil, nil
	}}
	svc := newReliabilityEvidenceForTest(true, repo)
	_, err := svc.Snapshot(context.Background(), &ReliabilityEvidenceQuery{
		Start: now.Add(-time.Minute), End: now, AnyGroupIDs: []int64{0, 7, 7}, AnyAccountIDs: []int64{53, -1},
		AnyPlatforms: []string{" OpenAI ", "openai"}, AnyRouteFingerprints: []string{"0123456789ABCDEF0123456789ABCDEF"}, AnyModelPatterns: []string{" GPT-* "},
	})
	require.NoError(t, err)
}

func TestReliabilityModelGlobToLikeEscapesSQLWildcards(t *testing.T) {
	require.Equal(t, `gpt-%`, reliabilityModelGlobToLike("GPT-*"))
	require.Equal(t, `model\_v_`, reliabilityModelGlobToLike("model_v?"))
}

func TestClaimProbeUsesDurableRepositoryWhenEnabled(t *testing.T) {
	interval := time.Date(2026, 8, 22, 4, 0, 0, 0, time.UTC)
	called := false
	repo := &reliabilityRepoStub{claim: func(_ context.Context, claim *ReliabilityProbeClaim) (bool, error) {
		called = true
		require.Equal(t, interval, claim.IntervalStart)
		return false, nil
	}}
	svc := newReliabilityEvidenceForTest(true, repo)

	claimed, err := svc.ClaimProbe(context.Background(), &ReliabilityProbeClaim{
		ClaimIdentity: "monthly:route:1", RouteFingerprint: "0123456789abcdef0123456789abcdef",
		IntervalStart: interval, ExpiresAt: interval.Add(2 * time.Minute),
	})

	require.NoError(t, err)
	require.False(t, claimed)
	require.True(t, called)
}

func TestReliabilityEvidenceQueueDropsWithoutBlockingWhenFull(t *testing.T) {
	svc := newReliabilityEvidenceForTest(true, &reliabilityRepoStub{})
	svc.queueSize = 1
	svc.queue = make(chan reliabilityEvidenceJob, 1)
	svc.queueStarted = true

	require.True(t, svc.SubmitFinalOutcome(&ReliabilityFinalOutcome{RequestIdentity: "first"}))
	require.False(t, svc.SubmitFinalOutcome(&ReliabilityFinalOutcome{RequestIdentity: "second"}))
	stats := svc.Completeness()
	require.Equal(t, int64(1), stats.Enqueued)
	require.Equal(t, int64(1), stats.Dropped)
	require.Equal(t, int64(1), stats.QueueDepth)
	require.NotNil(t, stats.OldestPendingAt)
	require.False(t, stats.Ready)
	close(svc.queue)
}

func TestReliabilityEvidenceCompletenessIsNotReadyWhileWriteIsInFlight(t *testing.T) {
	svc := newReliabilityEvidenceForTest(true, &reliabilityRepoStub{})
	svc.enqueued.Store(1)
	svc.inFlight.Store(1)

	stats := svc.Completeness()
	require.Equal(t, int64(1), stats.InFlight)
	require.False(t, stats.Ready)
}

func TestReliabilityEvidenceCompletenessTracksOldestPendingWatermark(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	repo := &reliabilityRepoStub{batch: func(_ context.Context, inputs []*ReliabilityObservation) (int64, error) {
		close(entered)
		<-release
		return int64(len(inputs)), nil
	}}
	svc := newReliabilityEvidenceForTest(true, repo)
	require.NoError(t, svc.Start(context.Background()))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		require.NoError(t, svc.Stop(ctx))
	})
	observedAt := time.Now().Add(-2 * time.Second)
	require.True(t, svc.SubmitFinalOutcome(&ReliabilityFinalOutcome{
		RequestIdentity: "watermark", Platform: PlatformOpenAI, Model: "gpt-5.6", RequestClass: ReliabilityRequestClassText,
		Protocol: "http", Outcome: ReliabilityOutcomeSuccess, ObservedAt: observedAt,
	}))
	<-entered
	stats := svc.Completeness()
	require.NotNil(t, stats.OldestPendingAt)
	require.Equal(t, observedAt, *stats.OldestPendingAt)
	close(release)
	require.Eventually(t, func() bool { return svc.Completeness().OldestPendingAt == nil }, time.Second, 10*time.Millisecond)
}

func TestReliabilityEvidenceLifecycleControlsReadiness(t *testing.T) {
	svc := newReliabilityEvidenceForTest(true, &reliabilityRepoStub{})
	require.False(t, svc.Completeness().Ready)
	require.NoError(t, svc.Start(context.Background()))
	require.True(t, svc.Completeness().Ready)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, svc.Stop(ctx))
	require.False(t, svc.Completeness().Ready)
}

func intPointer(value int) *int { return &value }

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAIRouteObservationStoreStub struct {
	mu           sync.Mutex
	observations []OpenAIRouteObservation
	costs        []OpenAIRouteActualCostObservation
	recordErr    error
	started      chan struct{}
	release      chan struct{}
	profiles     map[string]OpenAIRouteObservationProfile
}

type openAIRouteOutcomeRecorderStub struct {
	mu    sync.Mutex
	err   error
	calls int
}

func (s *openAIRouteOutcomeRecorderStub) RecordOpenAIRouteOutcome(context.Context, OpenAIRouteObservation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.err
}

func (s *openAIRouteObservationStoreStub) Check(context.Context) error { return s.recordErr }

func (s *openAIRouteObservationStoreStub) Record(_ context.Context, observation OpenAIRouteObservation) error {
	if s.started != nil {
		select {
		case s.started <- struct{}{}:
		default:
		}
	}
	if s.release != nil {
		<-s.release
	}
	s.mu.Lock()
	s.observations = append(s.observations, observation)
	s.mu.Unlock()
	return s.recordErr
}

func (s *openAIRouteObservationStoreStub) RecordCost(_ context.Context, observation OpenAIRouteActualCostObservation) error {
	s.mu.Lock()
	s.costs = append(s.costs, observation)
	s.mu.Unlock()
	return s.recordErr
}

func (s *openAIRouteObservationStoreStub) GetBatch(_ context.Context, _ []OpenAIRouteKey, _ time.Time) (map[string]OpenAIRouteObservationProfile, error) {
	return s.profiles, nil
}

func testOpenAIRouteObservation() OpenAIRouteObservation {
	return OpenAIRouteObservation{
		Key: OpenAIRouteKey{
			GroupID:       7,
			AccountID:     23,
			Model:         "gpt-5.6-sol",
			RequestClass:  OpenAIRouteRequestClassText,
			EndpointHash:  "endpoint",
			Transport:     "http_sse",
			FailureDomain: "pomoai",
		},
		Success:      true,
		FailureClass: OpenAIRouteFailureNone,
	}
}

func TestOpenAIRouteObservationCollectorWritesAsynchronously(t *testing.T) {
	store := &openAIRouteObservationStoreStub{}
	collector := NewOpenAIRouteObservationCollectorWithOptions(store, 1, 4)
	collector.Start()
	require.True(t, collector.TryRecord(testOpenAIRouteObservation()))
	collector.Stop()
	stats := collector.Stats()
	require.False(t, stats.CounterStartedAt.IsZero())
	require.Equal(t, uint64(1), stats.Submitted)
	require.Equal(t, uint64(1), stats.Written)
	require.Zero(t, stats.Failed)
	require.True(t, stats.Ready)
	require.Equal(t, 1.0, stats.Completeness)
	store.mu.Lock()
	require.Len(t, store.observations, 1)
	store.mu.Unlock()
}

func TestOpenAIRouteObservationCollectorCountsQueuedEvidenceAsIncomplete(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	store := &openAIRouteObservationStoreStub{started: started, release: release}
	collector := NewOpenAIRouteObservationCollectorWithOptions(store, 1, 4)
	collector.Start()
	require.True(t, collector.TryRecord(testOpenAIRouteObservation()))
	<-started

	stats := collector.Stats()
	require.Equal(t, uint64(1), stats.Submitted)
	require.Equal(t, uint64(1), stats.InFlight)
	require.Zero(t, stats.Completeness)
	require.False(t, stats.Ready)

	close(release)
	collector.Stop()
	require.InDelta(t, 1, collector.Stats().Completeness, 1e-12)
}

func TestOpenAIRouteObservationCollectorExposesFailureAndOverflow(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	store := &openAIRouteObservationStoreStub{recordErr: errors.New("redis unavailable"), started: started, release: release}
	collector := NewOpenAIRouteObservationCollectorWithOptions(store, 1, 1)
	collector.Start()
	require.True(t, collector.TryRecord(testOpenAIRouteObservation()))
	<-started
	require.True(t, collector.TryRecord(testOpenAIRouteObservation()))
	require.False(t, collector.TryRecord(testOpenAIRouteObservation()))
	close(release)
	collector.Stop()
	stats := collector.Stats()
	require.Equal(t, uint64(3), stats.Submitted)
	require.Equal(t, uint64(1), stats.Dropped)
	require.Equal(t, uint64(2), stats.Failed)
	require.False(t, stats.Ready)
	require.InDelta(t, 0, stats.Completeness, 1e-12)
}

func TestOpenAIRouteObservationCollectorSeparatesEvidenceWriteFromHealthApply(t *testing.T) {
	store := &openAIRouteObservationStoreStub{}
	recorder := &openAIRouteOutcomeRecorderStub{err: errors.New("health unavailable")}
	collector := NewOpenAIRouteObservationCollectorWithOptions(store, 1, 4, recorder)
	collector.Start()
	require.True(t, collector.TryRecord(testOpenAIRouteObservation()))
	collector.Stop()

	stats := collector.Stats()
	require.Equal(t, uint64(1), stats.Written)
	require.Zero(t, stats.Failed, "the rolling observation was durably written")
	require.Equal(t, uint64(1), stats.OutcomeFailed)
	require.Zero(t, stats.OutcomeInFlight)
	require.Zero(t, stats.OutcomeCompleteness)
	require.Equal(t, 1.0, stats.Completeness)
	require.False(t, stats.Ready, "health transition loss must still block readiness")
}

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAIRouteDecisionRepositoryStub struct {
	record    *OpenAIRouteShadowDecisionRecord
	checkErr  error
	err       error
	list      *OpenAIRouteShadowDecisionList
	stats     *OpenAIRouteShadowDecisionStats
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
}

func (s *openAIRouteDecisionRepositoryStub) CheckOpenAIRouteShadowDecisionStorage(context.Context) error {
	return s.checkErr
}

func (s *openAIRouteDecisionRepositoryStub) CreateOpenAIRouteShadowDecision(_ context.Context, record *OpenAIRouteShadowDecisionRecord) error {
	s.record = record
	if s.started != nil {
		s.startOnce.Do(func() { close(s.started) })
	}
	if s.release != nil {
		<-s.release
	}
	return s.err
}

func (s *openAIRouteDecisionRepositoryStub) ListOpenAIRouteShadowDecisions(context.Context, *OpenAIRouteShadowDecisionFilter) (*OpenAIRouteShadowDecisionList, error) {
	return s.list, s.err
}

func (s *openAIRouteDecisionRepositoryStub) GetOpenAIRouteShadowDecisionStats(context.Context, *OpenAIRouteShadowDecisionFilter) (*OpenAIRouteShadowDecisionStats, error) {
	return s.stats, s.err
}

func testOpenAIRouteShadowDecisionRecord() *OpenAIRouteShadowDecisionRecord {
	return &OpenAIRouteShadowDecisionRecord{
		DecisionID:    "shadow:test",
		GroupID:       7,
		Model:         "gpt-5.6-sol",
		RequestClass:  OpenAIRouteRequestClassText,
		PolicyMode:    OpenAIRoutePolicyShadow,
		PolicyVersion: 1,
		Reason:        "shadow_selected",
		Snapshot:      &OpenAIRouteShadowAuditSnapshot{},
	}
}

func TestOpenAIRouteAuditServiceRecordAndHealth(t *testing.T) {
	repo := &openAIRouteDecisionRepositoryStub{}
	svc := NewOpenAIRouteAuditService(repo)
	record := testOpenAIRouteShadowDecisionRecord()
	record.RequestID = string(make([]byte, 200))
	require.False(t, svc.Health().Ready, "storage must be verified before shadow is eligible")
	require.True(t, svc.VerifyStorage(context.Background()).Ready)

	require.NoError(t, svc.Record(context.Background(), record))
	require.Same(t, record, repo.record)
	require.Len(t, repo.record.RequestID, 128)
	health := svc.Health()
	require.True(t, health.Ready)
	require.False(t, health.AuditCounterStartedAt.IsZero())
	require.Equal(t, uint64(1), health.Attempted)
	require.Equal(t, uint64(1), health.Written)
	require.Zero(t, health.Failed)
	require.Zero(t, health.InFlight)
	require.InDelta(t, 1, health.Completeness, 1e-12)
	require.Equal(t, uint64(1), health.StorageChecks)
	require.False(t, health.LastSuccessAt.IsZero())
}

func TestOpenAIRouteAuditServiceFailureIsVisibleAndCanRecover(t *testing.T) {
	repo := &openAIRouteDecisionRepositoryStub{err: errors.New("database unavailable")}
	svc := NewOpenAIRouteAuditService(repo)

	require.Error(t, svc.Record(context.Background(), testOpenAIRouteShadowDecisionRecord()))
	health := svc.Health()
	require.False(t, health.Ready)
	require.Equal(t, uint64(1), health.Failed)
	require.Zero(t, health.Completeness)
	require.Contains(t, health.LastError, "database unavailable")

	time.Sleep(time.Millisecond)
	repo.err = nil
	require.NoError(t, svc.Record(context.Background(), testOpenAIRouteShadowDecisionRecord()))
	health = svc.Health()
	require.False(t, health.Ready, "a recovered store cannot recreate the failed decision")
	require.True(t, health.StorageReady)
	require.Equal(t, uint64(2), health.Attempted)
	require.Equal(t, uint64(1), health.Written)
	require.Equal(t, uint64(1), health.Failed)
	require.Zero(t, health.InFlight)
	require.InDelta(t, 0.5, health.Completeness, 1e-12)
	require.Empty(t, health.LastError)
}

func TestOpenAIRouteAuditServiceHealthExposesInFlightWrite(t *testing.T) {
	repo := &openAIRouteDecisionRepositoryStub{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	svc := NewOpenAIRouteAuditService(repo)
	done := make(chan error, 1)
	go func() {
		done <- svc.Record(context.Background(), testOpenAIRouteShadowDecisionRecord())
	}()
	<-repo.started

	health := svc.Health()
	require.Equal(t, uint64(1), health.Attempted)
	require.Zero(t, health.Written)
	require.Zero(t, health.Failed)
	require.Equal(t, uint64(1), health.InFlight)

	close(repo.release)
	require.NoError(t, <-done)
	health = svc.Health()
	require.Equal(t, uint64(1), health.Written)
	require.Zero(t, health.InFlight)
}

func TestOpenAIRouteAuditServiceRejectsIncompleteRecord(t *testing.T) {
	svc := NewOpenAIRouteAuditService(&openAIRouteDecisionRepositoryStub{})
	require.ErrorIs(t, svc.Record(context.Background(), &OpenAIRouteShadowDecisionRecord{}), ErrOpenAIRouteAuditUnavailable)
	health := svc.Health()
	require.False(t, health.Ready)
	require.Equal(t, uint64(1), health.Attempted)
	require.Equal(t, uint64(1), health.Failed)
	require.Zero(t, health.Written)
	require.Zero(t, health.InFlight)
	require.Contains(t, health.LastError, "incomplete decision record")
	var nilService *OpenAIRouteAuditService
	require.ErrorIs(t, nilService.Record(context.Background(), testOpenAIRouteShadowDecisionRecord()), ErrOpenAIRouteAuditUnavailable)
}

func TestOpenAIRouteAuditServiceTryRecordUsesBoundedAsyncQueue(t *testing.T) {
	repo := &openAIRouteDecisionRepositoryStub{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	svc := NewOpenAIRouteAuditServiceWithOptions(repo, 1, 1)
	svc.Start()
	record := func(id string) *OpenAIRouteShadowDecisionRecord {
		value := testOpenAIRouteShadowDecisionRecord()
		value.DecisionID = id
		return value
	}

	require.True(t, svc.TryRecord(record("shadow:running")))
	<-repo.started
	require.True(t, svc.TryRecord(record("shadow:queued")))
	require.False(t, svc.TryRecord(record("shadow:dropped")))
	health := svc.Health()
	require.Equal(t, uint64(3), health.Attempted)
	require.Equal(t, uint64(1), health.Dropped)
	require.Equal(t, uint64(1), health.Failed)
	require.Equal(t, uint64(2), health.InFlight)
	require.Zero(t, health.Completeness)

	close(repo.release)
	svc.Stop()
	health = svc.Health()
	require.Equal(t, uint64(2), health.Written)
	require.Equal(t, uint64(1), health.Failed)
	require.Zero(t, health.InFlight)
	require.InDelta(t, 2.0/3.0, health.Completeness, 1e-12)
}

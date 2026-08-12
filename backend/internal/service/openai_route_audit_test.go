package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type openAIRouteDecisionRepositoryStub struct {
	record   *OpenAIRouteShadowDecisionRecord
	checkErr error
	err      error
	list     *OpenAIRouteShadowDecisionList
	stats    *OpenAIRouteShadowDecisionStats
	started  chan struct{}
	release  chan struct{}
}

func (s *openAIRouteDecisionRepositoryStub) CheckOpenAIRouteShadowDecisionStorage(context.Context) error {
	return s.checkErr
}

func (s *openAIRouteDecisionRepositoryStub) CreateOpenAIRouteShadowDecision(_ context.Context, record *OpenAIRouteShadowDecisionRecord) error {
	s.record = record
	if s.started != nil {
		close(s.started)
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
	require.Equal(t, uint64(1), health.Attempted)
	require.Equal(t, uint64(1), health.Written)
	require.Zero(t, health.Failed)
	require.Zero(t, health.InFlight)
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
	require.Contains(t, health.LastError, "database unavailable")

	time.Sleep(time.Millisecond)
	repo.err = nil
	require.NoError(t, svc.Record(context.Background(), testOpenAIRouteShadowDecisionRecord()))
	health = svc.Health()
	require.True(t, health.Ready)
	require.Equal(t, uint64(2), health.Attempted)
	require.Equal(t, uint64(1), health.Written)
	require.Equal(t, uint64(1), health.Failed)
	require.Zero(t, health.InFlight)
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

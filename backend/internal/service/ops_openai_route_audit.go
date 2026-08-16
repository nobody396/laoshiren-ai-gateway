package service

import (
	"context"
	"time"
)

func (s *OpsService) ListOpenAIRouteShadowDecisions(
	ctx context.Context,
	filter *OpenAIRouteShadowDecisionFilter,
) (*OpenAIRouteShadowDecisionList, error) {
	if s == nil || s.openAIRouteAuditService == nil {
		return nil, ErrOpenAIRouteAuditUnavailable
	}
	return s.openAIRouteAuditService.List(ctx, filter)
}

func (s *OpsService) GetOpenAIRouteShadowDecisionStats(
	ctx context.Context,
	filter *OpenAIRouteShadowDecisionFilter,
) (*OpenAIRouteShadowDecisionStats, error) {
	if s == nil || s.openAIRouteAuditService == nil {
		return nil, ErrOpenAIRouteAuditUnavailable
	}
	return s.openAIRouteAuditService.Stats(ctx, filter)
}

func (s *OpsService) GetOpenAIRouteAuditHealth(ctx context.Context) OpenAIRouteAuditHealth {
	if s == nil || s.openAIRouteAuditService == nil {
		return OpenAIRouteAuditHealth{}
	}
	health := s.openAIRouteAuditService.VerifyStorage(ctx)
	stats, available := s.openAIGatewayService.SnapshotOpenAIRouteObservationCollector()
	health.ObservationCollectorAvailable = available
	if available {
		health.ObservationCounterStartedAt = stats.CounterStartedAt
		health.ObservationSubmitted = stats.Submitted
		health.ObservationWritten = stats.Written
		health.ObservationFailed = stats.Failed
		health.ObservationDropped = stats.Dropped
		health.ObservationRejected = stats.Rejected
		health.ObservationInFlight = stats.InFlight
		health.ObservationWaiting = stats.Waiting
		health.ObservationRunning = stats.Running
		health.ObservationCompleteness = stats.Completeness
		health.ObservationStorageChecks = stats.StorageChecks
		health.ObservationStorageFailed = stats.StorageFailed
		health.ObservationLastSuccessAt = stats.LastSuccessAt
		health.ObservationLastFailureAt = stats.LastFailureAt
		health.ObservationLastError = stats.LastError
		health.ObservationOutcomeApplied = stats.OutcomeApplied
		health.ObservationOutcomeFailed = stats.OutcomeFailed
		health.ObservationOutcomeInFlight = stats.OutcomeInFlight
		health.ObservationOutcomeCompleteness = stats.OutcomeCompleteness
		health.ObservationOutcomeLastSuccessAt = stats.OutcomeLastSuccessAt
		health.ObservationOutcomeLastFailureAt = stats.OutcomeLastFailureAt
		health.ObservationOutcomeLastError = stats.OutcomeLastError
		health.ObservationReady = stats.Ready
	}
	if stats, available := s.openAIGatewayService.SnapshotOpenAIRouteObservationProfileCache(); available {
		health.ObservationProfileCache = &stats
	}
	health.Ready = health.Ready && health.ObservationReady
	return health
}

func (s *OpsService) getOpenAIRouteAuditHealthForWindow(
	ctx context.Context,
	start time.Time,
	end time.Time,
) OpenAIRouteAuditHealth {
	health := s.GetOpenAIRouteAuditHealth(ctx)
	if s == nil || s.openAIRouteAuditService == nil || s.openAIGatewayService == nil ||
		s.openAIGatewayService.openAIRouteObservations == nil {
		return health
	}
	if err := s.openAIGatewayService.openAIRouteObservations.FlushDurableEvidence(ctx); err != nil {
		return health
	}
	window, err := s.openAIRouteAuditService.DurableEvidenceWindow(ctx, start, end)
	if err != nil {
		return health
	}
	health.DurableEvidence = &window
	if !window.Available {
		health.Ready = false
		return health
	}

	health.AuditCounterStartedAt = window.Audit.CounterStartedAt
	health.Attempted = window.Audit.Attempted
	health.Written = window.Audit.Written
	health.Failed = window.Audit.Failed
	health.Dropped = window.Audit.Dropped
	health.InFlight = window.Audit.InFlight
	health.Completeness = window.Audit.Completeness
	health.StorageChecks = window.Audit.StorageChecks
	health.StorageCheckFailed = window.Audit.StorageFailed

	health.ObservationCounterStartedAt = window.Observation.CounterStartedAt
	health.ObservationSubmitted = window.Observation.Attempted
	health.ObservationWritten = window.Observation.Written
	health.ObservationFailed = window.Observation.Failed
	health.ObservationDropped = window.Observation.Dropped
	health.ObservationRejected = window.Observation.Rejected
	health.ObservationInFlight = window.Observation.InFlight
	health.ObservationCompleteness = window.Observation.Completeness
	health.ObservationStorageChecks = window.Observation.StorageChecks
	health.ObservationStorageFailed = window.Observation.StorageFailed
	health.ObservationOutcomeApplied = window.Observation.OutcomeApplied
	health.ObservationOutcomeFailed = window.Observation.OutcomeFailed
	health.ObservationOutcomeInFlight = window.Observation.OutcomeInFlight
	health.ObservationOutcomeCompleteness = window.Observation.OutcomeCompleteness
	health.ObservationReady = health.ObservationReady && window.Observation.Ready
	health.Ready = health.StorageReady && health.ObservationCollectorAvailable &&
		health.ObservationReady && window.Ready
	return health
}

package service

import "context"

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

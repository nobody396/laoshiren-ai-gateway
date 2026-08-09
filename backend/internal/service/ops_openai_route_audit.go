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
	return s.openAIRouteAuditService.VerifyStorage(ctx)
}

package service

import "context"

// PreselectedGatewaySelector is the first strangler adapter: current handlers
// retain selection/lease ownership while the pipeline receives a neutral fact.
type PreselectedGatewaySelector struct{}

func (PreselectedGatewaySelector) Select(_ context.Context, request GatewayPipelineRequest) (GatewayPipelineSelection, error) {
	return GatewayPipelineSelection{AccountID: request.AccountID}, nil
}

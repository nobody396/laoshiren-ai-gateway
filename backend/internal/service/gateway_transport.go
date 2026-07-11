package service

import "context"

type GatewayPipelineTransportFunc func(context.Context, GatewayPipelineRequest, GatewayPipelineSelection) (GatewayPipelineResult, error)

func (f GatewayPipelineTransportFunc) Forward(ctx context.Context, request GatewayPipelineRequest, selection GatewayPipelineSelection) (GatewayPipelineResult, error) {
	return f(ctx, request, selection)
}

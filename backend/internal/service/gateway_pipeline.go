package service

import (
	"context"
	"time"
)

type GatewayPipelineRequest struct {
	Platform  string
	Endpoint  string
	AccountID int64
	Stream    bool
}

type GatewayPipelineSelection struct {
	AccountID int64
}

type GatewayPipelineResult struct {
	Value any
}

type GatewayPipelineSelector interface {
	Select(context.Context, GatewayPipelineRequest) (GatewayPipelineSelection, error)
}

type GatewayPipelineTransport interface {
	Forward(context.Context, GatewayPipelineRequest, GatewayPipelineSelection) (GatewayPipelineResult, error)
}

type GatewayPipelineFailover interface {
	Classify(context.Context, GatewayPipelineRequest, error) error
}

type GatewayPipelineMeter interface {
	Record(context.Context, GatewayPipelineObservation)
}

type GatewayPipelineObservation struct {
	Platform  string
	Endpoint  string
	AccountID int64
	Stream    bool
	Duration  time.Duration
	Failed    bool
}

type GatewayPipeline struct {
	selector GatewayPipelineSelector
	failover GatewayPipelineFailover
	meter    GatewayPipelineMeter
}

func NewGatewayPipeline(selector GatewayPipelineSelector, failover GatewayPipelineFailover, meter GatewayPipelineMeter) *GatewayPipeline {
	return &GatewayPipeline{selector: selector, failover: failover, meter: meter}
}

func (p *GatewayPipeline) Execute(ctx context.Context, request GatewayPipelineRequest, transport GatewayPipelineTransport) (result GatewayPipelineResult, err error) {
	started := time.Now()
	defer func() {
		if p != nil && p.meter != nil {
			p.meter.Record(ctx, GatewayPipelineObservation{
				Platform: request.Platform, Endpoint: request.Endpoint,
				AccountID: request.AccountID, Stream: request.Stream,
				Duration: time.Since(started), Failed: err != nil,
			})
		}
	}()
	selection := GatewayPipelineSelection{AccountID: request.AccountID}
	if p != nil && p.selector != nil {
		selection, err = p.selector.Select(ctx, request)
		if err != nil {
			return GatewayPipelineResult{}, err
		}
	}
	result, err = transport.Forward(ctx, request, selection)
	if err != nil && p != nil && p.failover != nil {
		err = p.failover.Classify(ctx, request, err)
	}
	return result, err
}

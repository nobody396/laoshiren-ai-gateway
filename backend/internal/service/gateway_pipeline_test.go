//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type pipelineSelectorStub struct{ order *[]string }

func (s pipelineSelectorStub) Select(_ context.Context, request GatewayPipelineRequest) (GatewayPipelineSelection, error) {
	*s.order = append(*s.order, "select")
	return GatewayPipelineSelection{AccountID: request.AccountID}, nil
}

type pipelineFailoverStub struct{ order *[]string }

func (s pipelineFailoverStub) Classify(_ context.Context, _ GatewayPipelineRequest, err error) error {
	*s.order = append(*s.order, "classify")
	return err
}

type pipelineMeterStub struct {
	order        *[]string
	observations []GatewayPipelineObservation
}

func (s *pipelineMeterStub) Record(_ context.Context, observation GatewayPipelineObservation) {
	*s.order = append(*s.order, "meter")
	s.observations = append(s.observations, observation)
}

func TestGatewayPipeline_PreservesStreamingResultAndStageOrder(t *testing.T) {
	order := []string{}
	meter := &pipelineMeterStub{order: &order}
	pipeline := NewGatewayPipeline(pipelineSelectorStub{&order}, pipelineFailoverStub{&order}, meter)
	want := &OpenAIForwardResult{Stream: true}
	result, err := pipeline.Execute(context.Background(), GatewayPipelineRequest{
		Platform: PlatformOpenAI, Endpoint: "/v1/responses", AccountID: 42, Stream: true,
	}, GatewayPipelineTransportFunc(func(_ context.Context, _ GatewayPipelineRequest, selection GatewayPipelineSelection) (GatewayPipelineResult, error) {
		order = append(order, "forward")
		require.Equal(t, int64(42), selection.AccountID)
		return GatewayPipelineResult{Value: want}, nil
	}))
	require.NoError(t, err)
	require.Same(t, want, result.Value)
	require.Equal(t, []string{"select", "forward", "meter"}, order)
	require.Len(t, meter.observations, 1)
	require.True(t, meter.observations[0].Stream)
	require.False(t, meter.observations[0].Failed)
}

func TestGatewayPipeline_PreservesFailoverErrorAndMetersOnce(t *testing.T) {
	order := []string{}
	meter := &pipelineMeterStub{order: &order}
	pipeline := NewGatewayPipeline(pipelineSelectorStub{&order}, pipelineFailoverStub{&order}, meter)
	want := errors.New("upstream unavailable")
	_, err := pipeline.Execute(context.Background(), GatewayPipelineRequest{AccountID: 7}, GatewayPipelineTransportFunc(
		func(context.Context, GatewayPipelineRequest, GatewayPipelineSelection) (GatewayPipelineResult, error) {
			order = append(order, "forward")
			return GatewayPipelineResult{}, want
		},
	))
	require.ErrorIs(t, err, want)
	require.Equal(t, []string{"select", "forward", "classify", "meter"}, order)
	require.Len(t, meter.observations, 1)
	require.True(t, meter.observations[0].Failed)
}

func TestAccountingPipelineMeter_UsesAccountingService(t *testing.T) {
	accounting := NewAccountingService(nil)
	meter := NewAccountingPipelineMeter(accounting)
	meter.Record(context.Background(), GatewayPipelineObservation{})
	meter.Record(context.Background(), GatewayPipelineObservation{Failed: true})
	total, failed := accounting.PipelineOutcomeCounts()
	require.Equal(t, uint64(2), total)
	require.Equal(t, uint64(1), failed)
}

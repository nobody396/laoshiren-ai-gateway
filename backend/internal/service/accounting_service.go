package service

import (
	"context"
	"sync/atomic"
)

// AccountingService exposes redacted queue health and explicit dead-command
// replay controls to operator-facing adapters.
type AccountingService struct {
	repo           AccountingCommandRepository
	pipelineTotal  atomic.Uint64
	pipelineFailed atomic.Uint64
}

func (s *AccountingService) ObservePipelineOutcome(_ context.Context, observation GatewayPipelineObservation) {
	if s == nil {
		return
	}
	s.pipelineTotal.Add(1)
	if observation.Failed {
		s.pipelineFailed.Add(1)
	}
}

func (s *AccountingService) PipelineOutcomeCounts() (total, failed uint64) {
	if s == nil {
		return 0, 0
	}
	return s.pipelineTotal.Load(), s.pipelineFailed.Load()
}

func NewAccountingService(repo AccountingCommandRepository) *AccountingService {
	return &AccountingService{repo: repo}
}

func (s *AccountingService) Stats(ctx context.Context) (*AccountingCommandStats, error) {
	return s.repo.Stats(ctx)
}

func (s *AccountingService) ReplayDead(ctx context.Context, id int64) error {
	return s.repo.ReplayDead(ctx, id)
}

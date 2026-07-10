package service

import "context"

// AccountingService exposes redacted queue health and explicit dead-command
// replay controls to operator-facing adapters.
type AccountingService struct {
	repo AccountingCommandRepository
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

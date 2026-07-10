//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type accountingWorkerRepoStub struct {
	completed int
	failed    int
	dead      bool
	retryAt   time.Time
}

func (s *accountingWorkerRepoStub) Enqueue(context.Context, int64, *UsageBillingCommand) (*AccountingCommand, bool, error) {
	panic("unexpected enqueue")
}
func (s *accountingWorkerRepoStub) ClaimDue(context.Context, string, time.Duration, int) ([]AccountingCommand, error) {
	return nil, nil
}
func (s *accountingWorkerRepoStub) Complete(context.Context, int64, string) error {
	s.completed++
	return nil
}
func (s *accountingWorkerRepoStub) Fail(_ context.Context, _ int64, _, _, _ string, retryAt time.Time, dead bool) error {
	s.failed++
	s.dead = dead
	s.retryAt = retryAt
	return nil
}
func (s *accountingWorkerRepoStub) Stats(context.Context) (*AccountingCommandStats, error) {
	return &AccountingCommandStats{}, nil
}
func (s *accountingWorkerRepoStub) ReplayDead(context.Context, int64) error { return nil }
func (s *accountingWorkerRepoStub) GetByUsageLogID(context.Context, int64) (*AccountingCommand, error) {
	return nil, nil
}

type idempotentBillingRepoStub struct {
	calls   int
	charges int
	err     error
	seen    map[string]struct{}
}

func (s *idempotentBillingRepoStub) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.seen == nil {
		s.seen = map[string]struct{}{}
	}
	key := cmd.RequestID
	if _, ok := s.seen[key]; ok {
		return &UsageBillingApplyResult{Applied: false}, nil
	}
	s.seen[key] = struct{}{}
	s.charges++
	return &UsageBillingApplyResult{Applied: true}, nil
}

func TestAccountingWorker_CrashAfterBillingReplaysWithoutDoubleCharge(t *testing.T) {
	repo := &accountingWorkerRepoStub{}
	billing := &idempotentBillingRepoStub{}
	worker := NewAccountingWorker(repo, billing)
	command := AccountingCommand{ID: 1, Attempts: 1, Payload: UsageBillingCommand{RequestID: "req-1", APIKeyID: 2}}
	worker.afterApply = func(AccountingCommand, *UsageBillingApplyResult) error {
		return errAccountingWorkerInterrupted
	}

	err := worker.processCommand(context.Background(), command)
	require.ErrorIs(t, err, errAccountingWorkerInterrupted)
	require.Zero(t, repo.completed)
	require.Zero(t, repo.failed)
	require.Equal(t, 1, billing.charges)

	worker.afterApply = nil
	require.NoError(t, worker.processCommand(context.Background(), command))
	require.Equal(t, 1, repo.completed)
	require.Equal(t, 2, billing.calls)
	require.Equal(t, 1, billing.charges)
}

func TestAccountingWorker_TransientBackoffAndPermanentDead(t *testing.T) {
	repo := &accountingWorkerRepoStub{}
	billing := &idempotentBillingRepoStub{err: errors.New("database unavailable")}
	worker := NewAccountingWorker(repo, billing)
	command := AccountingCommand{ID: 2, Attempts: 2, Payload: UsageBillingCommand{RequestID: "req-2", APIKeyID: 2}}
	require.NoError(t, worker.processCommand(context.Background(), command))
	require.Equal(t, 1, repo.failed)
	require.False(t, repo.dead)
	require.True(t, repo.retryAt.After(time.Now()))

	repo = &accountingWorkerRepoStub{}
	billing.err = ErrUsageBillingRequestConflict
	worker = NewAccountingWorker(repo, billing)
	require.NoError(t, worker.processCommand(context.Background(), command))
	require.True(t, repo.dead)
}

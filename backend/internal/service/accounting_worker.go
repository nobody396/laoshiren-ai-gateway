package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

const (
	defaultAccountingLease        = 30 * time.Second
	defaultAccountingPollInterval = time.Second
	defaultAccountingBatchSize    = 20
	defaultAccountingMaxAttempts  = 8
)

var errAccountingWorkerInterrupted = errors.New("accounting worker interrupted after billing commit")

type AccountingWorker struct {
	repo        AccountingCommandRepository
	billingRepo UsageBillingRepository
	workerID    string
	lease       time.Duration
	poll        time.Duration
	batchSize   int
	maxAttempts int
	stopCh      chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
	afterApply  func(AccountingCommand, *UsageBillingApplyResult) error
}

func NewAccountingWorker(repo AccountingCommandRepository, billingRepo UsageBillingRepository) *AccountingWorker {
	return &AccountingWorker{
		repo: repo, billingRepo: billingRepo,
		workerID: fmt.Sprintf("accounting-%d", time.Now().UnixNano()),
		lease:    defaultAccountingLease, poll: defaultAccountingPollInterval,
		batchSize: defaultAccountingBatchSize, maxAttempts: defaultAccountingMaxAttempts,
		stopCh: make(chan struct{}),
	}
}

func (w *AccountingWorker) Start() {
	if w == nil || w.repo == nil || w.billingRepo == nil {
		return
	}
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.poll)
		defer ticker.Stop()
		for {
			if _, err := w.ProcessOnce(context.Background()); err != nil {
				slog.Error("accounting worker poll failed", "error", err)
			}
			select {
			case <-ticker.C:
			case <-w.stopCh:
				return
			}
		}
	}()
}

func (w *AccountingWorker) Stop() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() { close(w.stopCh) })
	w.wg.Wait()
}

func (w *AccountingWorker) ProcessOnce(ctx context.Context) (int, error) {
	commands, err := w.repo.ClaimDue(ctx, w.workerID, w.lease, w.batchSize)
	if err != nil {
		return 0, err
	}
	processed := 0
	for i := range commands {
		if err := w.processCommand(ctx, commands[i]); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (w *AccountingWorker) processCommand(ctx context.Context, command AccountingCommand) error {
	result, err := w.billingRepo.Apply(ctx, &command.Payload)
	if err == nil && w.afterApply != nil {
		err = w.afterApply(command, result)
	}
	if errors.Is(err, errAccountingWorkerInterrupted) {
		return err
	}
	if err == nil {
		return w.repo.Complete(ctx, command.ID, w.workerID)
	}

	dead := errors.Is(err, ErrUsageBillingRequestConflict) || command.Attempts >= w.maxAttempts
	code := "transient"
	if dead {
		code = "review_required"
	}
	retryAt := time.Now().Add(accountingRetryBackoff(command.Attempts))
	if failErr := w.repo.Fail(ctx, command.ID, w.workerID, code, err.Error(), retryAt, dead); failErr != nil {
		return failErr
	}
	return nil
}

func accountingRetryBackoff(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	delay := time.Second << (attempt - 1)
	if delay > 5*time.Minute {
		return 5 * time.Minute
	}
	return delay
}

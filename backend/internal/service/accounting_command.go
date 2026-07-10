package service

import (
	"context"
	"time"
)

const (
	AccountingCommandVersion          = 1
	AccountingCommandStatusPending    = "pending"
	AccountingCommandStatusProcessing = "processing"
	AccountingCommandStatusCompleted  = "completed"
	AccountingCommandStatusDead       = "dead"
)

type AccountingCommand struct {
	ID               int64
	RequestID        string
	APIKeyID         int64
	UsageLogID       int64
	Version          int
	Payload          UsageBillingCommand
	Status           string
	Attempts         int
	AvailableAt      time.Time
	LeaseOwner       *string
	LeaseExpiresAt   *time.Time
	LastErrorCode    *string
	LastErrorMessage *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CompletedAt      *time.Time
}

type AccountingCommandStats struct {
	PendingCount int64
	DeadCount    int64
	RetryCount   int64
	OldestAge    time.Duration
}

type AccountingCommandRepository interface {
	Enqueue(ctx context.Context, usageLogID int64, cmd *UsageBillingCommand) (*AccountingCommand, bool, error)
	ClaimDue(ctx context.Context, workerID string, lease time.Duration, limit int) ([]AccountingCommand, error)
	Complete(ctx context.Context, id int64, workerID string) error
	Fail(ctx context.Context, id int64, workerID, code, message string, retryAt time.Time, dead bool) error
	Stats(ctx context.Context) (*AccountingCommandStats, error)
	ReplayDead(ctx context.Context, id int64) error
	GetByUsageLogID(ctx context.Context, usageLogID int64) (*AccountingCommand, error)
}

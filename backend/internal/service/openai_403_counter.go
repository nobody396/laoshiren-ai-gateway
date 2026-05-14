package service

import "context"

// OpenAI403CounterCache tracks consecutive OpenAI 403 failures per account.
type OpenAI403CounterCache interface {
	IncrementOpenAI403Count(ctx context.Context, accountID int64, windowMinutes int) (int64, error)
	ResetOpenAI403Count(ctx context.Context, accountID int64) error
}

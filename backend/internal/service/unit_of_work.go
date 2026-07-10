package service

import "context"

// UnitOfWork owns a single database transaction and propagates it through ctx.
// Repository methods must reuse the propagated transaction and must not commit
// or roll it back themselves.
type UnitOfWork interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}

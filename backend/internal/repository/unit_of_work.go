package repository

import (
	"context"
	"database/sql"
	"errors"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

type unitOfWorkContextKey struct{}

type transactionResources struct {
	client *dbent.Client
	sql    *sql.Tx
}

type unitOfWork struct {
	db *sql.DB
}

// NewUnitOfWork creates the application transaction boundary. The SQL and Ent
// repositories receive the same *sql.Tx through context.
func NewUnitOfWork(db *sql.DB) service.UnitOfWork {
	return &unitOfWork{db: db}
}

func (u *unitOfWork) WithinTx(ctx context.Context, fn func(context.Context) error) (err error) {
	if fn == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := transactionResourcesFromContext(ctx); ok {
		if err := ctx.Err(); err != nil {
			return err
		}
		return fn(ctx)
	}
	if u == nil || u.db == nil {
		return errors.New("unit of work database is not configured")
	}

	tx, err := u.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	driver := entsql.NewDriver(dialect.Postgres, entsql.Conn{ExecQuerier: tx})
	client := dbent.NewClient(dbent.Driver(driver))
	txCtx := context.WithValue(ctx, unitOfWorkContextKey{}, &transactionResources{
		client: client,
		sql:    tx,
	})

	defer func() {
		if recovered := recover(); recovered != nil {
			_ = tx.Rollback()
			panic(recovered)
		}
	}()

	if err = fn(txCtx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err = ctx.Err(); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err = tx.Commit(); err != nil {
		_ = tx.Rollback()
		return err
	}
	return nil
}

func transactionResourcesFromContext(ctx context.Context) (*transactionResources, bool) {
	if ctx == nil {
		return nil, false
	}
	resources, ok := ctx.Value(unitOfWorkContextKey{}).(*transactionResources)
	return resources, ok && resources != nil
}

func transactionClientFromContext(ctx context.Context) (*dbent.Client, bool) {
	if resources, ok := transactionResourcesFromContext(ctx); ok && resources.client != nil {
		return resources.client, true
	}
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client(), true
	}
	return nil, false
}

func sqlExecutorFromContext(ctx context.Context, fallback sqlExecutor) sqlExecutor {
	if resources, ok := transactionResourcesFromContext(ctx); ok && resources.sql != nil {
		return resources.sql
	}
	return fallback
}

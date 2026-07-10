//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestUnitOfWork_NestedReusesSingleTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectBegin()
	mock.ExpectCommit()

	uow := NewUnitOfWork(db)
	var outer, inner *transactionResources
	err = uow.WithinTx(context.Background(), func(ctx context.Context) error {
		outer, _ = transactionResourcesFromContext(ctx)
		return uow.WithinTx(ctx, func(nested context.Context) error {
			inner, _ = transactionResourcesFromContext(nested)
			return nil
		})
	})
	require.NoError(t, err)
	require.NotNil(t, outer)
	require.Same(t, outer, inner)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_RollsBackOnError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectBegin()
	mock.ExpectRollback()

	want := errors.New("write failed")
	err = NewUnitOfWork(db).WithinTx(context.Background(), func(context.Context) error { return want })
	require.ErrorIs(t, err, want)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_RollsBackOnPanic(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectBegin()
	mock.ExpectRollback()

	require.PanicsWithValue(t, "boom", func() {
		_ = NewUnitOfWork(db).WithinTx(context.Background(), func(context.Context) error {
			panic("boom")
		})
	})
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_RollsBackWhenContextCancelled(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectBegin()
	mock.ExpectRollback()

	ctx, cancel := context.WithCancel(context.Background())
	err = NewUnitOfWork(db).WithinTx(ctx, func(context.Context) error {
		cancel()
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestUnitOfWork_ReturnsCommitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectBegin()
	want := errors.New("commit failed")
	mock.ExpectCommit().WillReturnError(want)

	err = NewUnitOfWork(db).WithinTx(context.Background(), func(context.Context) error { return nil })
	require.ErrorIs(t, err, want)
	require.NoError(t, mock.ExpectationsWereMet())
}

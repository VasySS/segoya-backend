package postgres

import (
	"context"
	"fmt"

	"github.com/VasySS/segoya-backend/internal/infrastructure/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ repository.TxManager = (*TxManager)(nil)

// QueryEngine is a query engine for repository (tx, pool, etc).
type QueryEngine interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txManagerKey struct{}

// TxManager is a transaction manager for repository.
type TxManager struct {
	pool *pgxpool.Pool
}

// NewTxManager creates a new transaction manager for repository.
func NewTxManager(pool *pgxpool.Pool) *TxManager {
	return &TxManager{
		pool: pool,
	}
}

// RunTx begins a new transaction with default isolation level.
func (tm *TxManager) RunTx(ctx context.Context, fn repository.TxFunc) error {
	return tm.beginFunc(ctx, pgx.TxOptions{}, fn)
}

// RunReadTx begins a new transaction with ReadOnly access mode.
func (tm *TxManager) RunReadTx(ctx context.Context, fn repository.TxFunc) error {
	opts := pgx.TxOptions{
		AccessMode: pgx.ReadOnly,
	}

	return tm.beginFunc(ctx, opts, fn)
}

// ReadUncommitted begins a new transaction with ReadUncommitted isolation level.
func (tm *TxManager) ReadUncommitted(ctx context.Context, fn repository.TxFunc) error {
	opts := pgx.TxOptions{
		IsoLevel:   pgx.ReadUncommitted,
		AccessMode: pgx.ReadWrite,
	}

	return tm.beginFunc(ctx, opts, fn)
}

// RunReadCommitted begins a new transaction with ReadCommitted isolation level.
func (tm *TxManager) RunReadCommitted(ctx context.Context, fn repository.TxFunc) error {
	opts := pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: pgx.ReadWrite,
	}

	return tm.beginFunc(ctx, opts, fn)
}

// RunRepeatableRead begins a new transaction with RepeatableRead isolation level.
func (tm *TxManager) RunRepeatableRead(ctx context.Context, fn repository.TxFunc) error {
	opts := pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadWrite,
	}

	return tm.beginFunc(ctx, opts, fn)
}

// RunSerializable begins a new transaction with Serializable isolation level.
func (tm *TxManager) RunSerializable(ctx context.Context, fn repository.TxFunc) error {
	opts := pgx.TxOptions{
		IsoLevel:   pgx.Serializable,
		AccessMode: pgx.ReadWrite,
	}

	return tm.beginFunc(ctx, opts, fn)
}

// GetQueryEngine returns the query engine (tx or pool) for the current transaction from context.
func (tm *TxManager) GetQueryEngine(ctx context.Context) QueryEngine { //nolint:ireturn
	tx, ok := ctx.Value(txManagerKey{}).(pgx.Tx)
	if ok && tx != nil {
		return tx
	}

	return tm.pool
}

func (tm *TxManager) beginFunc(
	ctx context.Context,
	txOpts pgx.TxOptions,
	fn func(txCtx context.Context) error,
) error {
	if tx, ok := ctx.Value(txManagerKey{}).(QueryEngine); ok && tx != nil {
		return fn(ctx)
	}

	tx, err := tm.pool.BeginTx(ctx, txOpts)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	ctx = context.WithValue(ctx, txManagerKey{}, tx)
	if err := fn(ctx); err != nil {
		return err
	}

	return tx.Commit(ctx) //nolint:wrapcheck
}

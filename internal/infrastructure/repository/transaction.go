// Package repository contains methods for working with repositories.
package repository

import "context"

// TxFunc is a function, inside of which the transaction is executed.
type TxFunc func(context.Context) error

// TxManager is a transaction manager for repository.
type TxManager interface {
	RunTx(ctx context.Context, fn TxFunc) error
	RunReadTx(ctx context.Context, fn TxFunc) error
	ReadUncommitted(ctx context.Context, fn TxFunc) error
	RunReadCommitted(ctx context.Context, fn TxFunc) error
	RunRepeatableRead(ctx context.Context, fn TxFunc) error
	RunSerializable(ctx context.Context, fn TxFunc) error
}

// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package transaction provides transaction management capabilities.
package transaction

import (
	"context"
	"database/sql"
)

type contextKey string

// There is no default context key to enforce explicit database naming in transactions.

func getTxContextKey(dbName string) contextKey {
	return contextKey("tx_" + dbName)
}

// WithKeyedTx stores a transaction in the context with a database name.
func WithKeyedTx(ctx context.Context, dbName string, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, getTxContextKey(dbName), tx)
}

// KeyedTxFromContext retrieves a transaction from the context with a database name.
func KeyedTxFromContext(ctx context.Context, dbName string) *sql.Tx {
	if tx, ok := ctx.Value(getTxContextKey(dbName)).(*sql.Tx); ok {
		return tx
	}
	return nil
}

// HasKeyedTx checks if the context contains a transaction for a database name.
func HasKeyedTx(ctx context.Context, dbName string) bool {
	return KeyedTxFromContext(ctx, dbName) != nil
}

// endHooksKey carries the functions to run when the innermost transaction in a context ends.
const endHooksKey contextKey = "tx_end_hooks"

// withEndHooks returns ctx carrying a new, empty list of end hooks.
func withEndHooks(ctx context.Context) (context.Context, *[]func()) {
	hooks := &[]func(){}
	return context.WithValue(ctx, endHooksKey, hooks), hooks
}

// OnTransactionEnd runs fn once the innermost transaction ctx carries ends, whether it commits or rolls
// back, so work that must follow a write's outcome, such as dropping a cached copy a concurrent read
// may have refilled, runs after it. It does nothing when ctx carries no transaction, where the write
// has already taken effect.
func OnTransactionEnd(ctx context.Context, fn func()) {
	if hooks, ok := ctx.Value(endHooksKey).(*[]func()); ok {
		*hooks = append(*hooks, fn)
	}
}

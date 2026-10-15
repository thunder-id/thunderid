// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package transaction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// NewTransactioner creates a new database-backed Transactioner instance.
func NewTransactioner(db *sql.DB, dbName string) providers.Transactioner {
	return &dbTransactioner{
		db:     db,
		dbName: dbName,
	}
}

// NewNoOpTransactioner creates a Transactioner that simply executes the function without
// wrapping it in a database transaction. This is used for file-based (declarative) store modes
// where no database transaction management is needed.
func NewNoOpTransactioner() providers.Transactioner {
	return &noOpTransactioner{}
}

// dbTransactioner is the database-backed implementation of Transactioner.
type dbTransactioner struct {
	db     *sql.DB
	dbName string
}

// Transact executes the given function within a database transaction.
func (t *dbTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) (err error) {
	// Check if we're already in a transaction for this database
	if HasKeyedTx(ctx, t.dbName) {
		// Already in a transaction - just execute the function without creating a new one
		return txFunc(ctx)
	}

	// 1. Begin transaction
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		log.GetLogger().Error(ctx, "failed to begin transaction",
			log.String("dbName", t.dbName),
			log.Error(err),
		)
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// The end hooks run once the transaction has committed or rolled back.
	ctx, hooks := withEndHooks(ctx)
	defer func() {
		for _, hook := range *hooks {
			hook()
		}
	}()

	// 2. Setup recovery and commit/rollback handling
	defer func() {
		if p := recover(); p != nil {
			// Capture stack trace
			stack := string(debug.Stack())
			log.GetLogger().Error(ctx, "panic occurred during transaction",
				log.String("dbName", t.dbName),
				log.Any("panic", p),
				log.String("stack", stack),
			)

			// Panic occurred - rollback and convert panic to error
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.GetLogger().Error(ctx, "failed to rollback transaction after unexpected error",
					log.String("dbName", t.dbName),
					log.Error(rollbackErr),
				)
				// Convert panic to error and join with rollback error
				switch v := p.(type) {
				case error:
					err = errors.Join(fmt.Errorf("transaction aborted unexpectedly: %w", v), rollbackErr)
				default:
					err = errors.Join(fmt.Errorf("transaction aborted unexpectedly: %v", v), rollbackErr)
				}
			} else {
				// Convert panic to error
				switch v := p.(type) {
				case error:
					err = fmt.Errorf("transaction aborted unexpectedly: %w", v)
				default:
					err = fmt.Errorf("transaction aborted unexpectedly: %v", v)
				}
			}
		} else if err != nil {
			// Error occurred - rollback
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.GetLogger().Error(ctx, "failed to rollback transaction",
					log.String("dbName", t.dbName),
					log.Error(rollbackErr),
				)
				err = errors.Join(err, rollbackErr)
			}
		} else {
			// Success - commit
			err = tx.Commit()
			if err != nil {
				log.GetLogger().Error(ctx, "failed to commit transaction",
					log.String("dbName", t.dbName),
					log.Error(err),
				)
			}
		}
	}()

	// 3. Create context with transaction
	txCtx := WithKeyedTx(ctx, t.dbName, tx)

	// 4. Execute the user-provided function
	err = txFunc(txCtx)
	return err
}

// noOpTransactioner is a no-op implementation that simply executes the function directly.
// Used for declarative (file-based) store modes where no database transaction is needed.
type noOpTransactioner struct{}

// Transact executes the given function directly without transaction wrapping.
func (n *noOpTransactioner) Transact(ctx context.Context, txFunc func(context.Context) error) error {
	return txFunc(ctx)
}

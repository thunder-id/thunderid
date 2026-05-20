/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package transaction

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/thunder-id/thunderid/internal/system/log"
)

// Transactioner provides transaction management with automatic nesting detection.
type Transactioner interface {
	// Transact executes the given function within a transaction.
	// If a transaction already exists in the context, it reuses it.
	// Otherwise, it creates a new transaction and commits/rolls back automatically.
	Transact(ctx context.Context, txFunc func(context.Context) error) error
}

// NewTransactioner creates a new database-backed Transactioner instance.
func NewTransactioner(db *sql.DB, dbName string) Transactioner {
	return &dbTransactioner{
		db:     db,
		dbName: dbName,
	}
}

// NewNoOpTransactioner creates a Transactioner that simply executes the function without
// wrapping it in a database transaction. This is used for file-based (declarative) store modes
// where no database transaction management is needed.
func NewNoOpTransactioner() Transactioner {
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
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// 2. Setup recovery and commit/rollback handling
	defer func() {
		if p := recover(); p != nil {
			// Capture stack trace
			stack := string(debug.Stack())
			log.GetLogger().Error("panic occurred during transaction",
				log.String("dbName", t.dbName),
				log.Any("panic", p),
				log.String("stack", stack),
			)

			// Panic occurred - rollback and convert panic to error
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				log.GetLogger().Error("failed to rollback transaction after unexpected error",
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
				log.GetLogger().Error("failed to rollback transaction",
					log.String("dbName", t.dbName),
					log.Error(rollbackErr),
				)
				err = errors.Join(err, rollbackErr)
			}
		} else {
			// Success - commit
			err = tx.Commit()
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

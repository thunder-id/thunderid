// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package transaction

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type TransactionerTestSuite struct {
	suite.Suite
	db            *sql.DB
	mock          sqlmock.Sqlmock
	transactioner providers.Transactioner
}

func TestTransactionerTestSuite(t *testing.T) {
	suite.Run(t, new(TransactionerTestSuite))
}

func (suite *TransactionerTestSuite) SetupTest() {
	db, mock, err := sqlmock.New()
	suite.Require().NoError(err)

	suite.db = db
	suite.mock = mock
	suite.transactioner = NewTransactioner(db, "test")
}

func (suite *TransactionerTestSuite) TearDownTest() {
	_ = suite.db.Close()
}

func (suite *TransactionerTestSuite) TestTransact_Success() {
	ctx := context.Background()

	// Expect transaction to be started and committed
	suite.mock.ExpectBegin()
	suite.mock.ExpectCommit()

	executed := false
	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		executed = true
		// Verify transaction is in context
		suite.True(HasKeyedTx(txCtx, "test"))
		suite.NotNil(KeyedTxFromContext(txCtx, "test"))
		return nil
	})

	suite.NoError(err)
	suite.True(executed)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestTransact_Error() {
	ctx := context.Background()
	expectedErr := errors.New("business logic error")

	// Expect transaction to be started and rolled back
	suite.mock.ExpectBegin()
	suite.mock.ExpectRollback()

	executed := false
	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		executed = true
		return expectedErr
	})

	suite.Error(err)
	suite.Equal(expectedErr, err)
	suite.True(executed)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestTransact_Panic() {
	ctx := context.Background()

	// Expect transaction to be started and rolled back
	suite.mock.ExpectBegin()
	suite.mock.ExpectRollback()

	executed := false
	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		executed = true
		panic("something went wrong")
	})

	suite.Error(err)
	suite.Contains(err.Error(), "transaction aborted unexpectedly")
	suite.Contains(err.Error(), "something went wrong")
	suite.True(executed)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestTransact_NestedTransaction() {
	ctx := context.Background()

	// Expect only ONE transaction to be started and committed
	suite.mock.ExpectBegin()
	suite.mock.ExpectCommit()

	outerExecuted := false
	innerExecuted := false

	err := suite.transactioner.Transact(ctx, func(txCtx1 context.Context) error {
		outerExecuted = true
		suite.True(HasKeyedTx(txCtx1, "test"))
		tx1 := KeyedTxFromContext(txCtx1, "test")
		suite.NotNil(tx1)

		// Nested call - should reuse the same transaction
		err := suite.transactioner.Transact(txCtx1, func(txCtx2 context.Context) error {
			innerExecuted = true
			suite.True(HasKeyedTx(txCtx2, "test"))
			tx2 := KeyedTxFromContext(txCtx2, "test")
			suite.NotNil(tx2)

			// Should be the same transaction
			suite.Equal(tx1, tx2)
			return nil
		})

		return err
	})

	suite.NoError(err)
	suite.True(outerExecuted)
	suite.True(innerExecuted)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestTransact_NestedTransactionWithError() {
	ctx := context.Background()
	expectedErr := errors.New("inner error")

	// Expect only ONE transaction to be started and rolled back
	suite.mock.ExpectBegin()
	suite.mock.ExpectRollback()

	outerExecuted := false
	innerExecuted := false

	err := suite.transactioner.Transact(ctx, func(txCtx1 context.Context) error {
		outerExecuted = true

		// Nested call that returns an error
		err := suite.transactioner.Transact(txCtx1, func(txCtx2 context.Context) error {
			innerExecuted = true
			return expectedErr
		})

		return err
	})

	suite.Error(err)
	suite.Equal(expectedErr, err)
	suite.True(outerExecuted)
	suite.True(innerExecuted)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestTransact_BeginError() {
	ctx := context.Background()
	expectedErr := errors.New("begin transaction failed")

	// Expect BeginTx to fail
	suite.mock.ExpectBegin().WillReturnError(expectedErr)

	executed := false
	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		executed = true
		return nil
	})

	suite.Error(err)
	suite.ErrorIs(err, expectedErr)
	suite.Contains(err.Error(), "failed to begin transaction")
	suite.False(executed) // Function should not be executed if BeginTx fails
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestTransact_CommitError() {
	ctx := context.Background()
	expectedErr := errors.New("commit failed")

	// Expect transaction to be started, but commit should fail
	suite.mock.ExpectBegin()
	suite.mock.ExpectCommit().WillReturnError(expectedErr)

	executed := false
	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		executed = true
		suite.True(HasKeyedTx(txCtx, "test"))
		return nil
	})

	suite.Error(err)
	suite.Equal(expectedErr, err)
	suite.True(executed) // Function should be executed even if commit fails
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestTransact_RollbackErrorOnPanic() {
	ctx := context.Background()
	rollbackErr := errors.New("rollback failed")

	// Expect transaction to be started and rolled back with error
	suite.mock.ExpectBegin()
	suite.mock.ExpectRollback().WillReturnError(rollbackErr)

	executed := false
	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		executed = true
		panic("something went wrong")
	})

	// Should still get the panic error, not the rollback error
	suite.Error(err)
	suite.Contains(err.Error(), "transaction aborted unexpectedly")
	suite.Contains(err.Error(), "something went wrong")
	suite.True(executed)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestTransact_RollbackErrorOnError() {
	ctx := context.Background()
	businessErr := errors.New("business logic error")
	rollbackErr := errors.New("rollback failed")

	// Expect transaction to be started and rolled back with error
	suite.mock.ExpectBegin()
	suite.mock.ExpectRollback().WillReturnError(rollbackErr)

	executed := false
	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		executed = true
		return businessErr
	})

	// Should get both the original business error and the rollback error
	suite.Error(err)
	suite.ErrorIs(err, businessErr)
	suite.ErrorIs(err, rollbackErr)
	suite.Contains(err.Error(), "business logic error")
	suite.Contains(err.Error(), "rollback failed")
	suite.True(executed)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestTransact_PanicWithNonErrorValue() {
	ctx := context.Background()

	// Expect transaction to be started and rolled back
	suite.mock.ExpectBegin()
	suite.mock.ExpectRollback()

	executed := false
	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		executed = true
		panic("string panic value") // Panic with non-error type
	})

	// Should convert string panic to error
	suite.Error(err)
	suite.Contains(err.Error(), "transaction aborted unexpectedly")
	suite.Contains(err.Error(), "string panic value")
	suite.True(executed)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

// NoOpTransactionerTestSuite tests the no-op transactioner implementation.
type NoOpTransactionerTestSuite struct {
	suite.Suite
	transactioner providers.Transactioner
}

func TestNoOpTransactionerTestSuite(t *testing.T) {
	suite.Run(t, new(NoOpTransactionerTestSuite))
}

func (suite *NoOpTransactionerTestSuite) SetupTest() {
	suite.transactioner = NewNoOpTransactioner()
}

func (suite *NoOpTransactionerTestSuite) TestTransact_Success() {
	ctx := context.Background()

	executed := false
	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		executed = true
		// No transaction should be in context
		suite.False(HasKeyedTx(txCtx, "test"))
		return nil
	})

	suite.NoError(err)
	suite.True(executed)
}

func (suite *NoOpTransactionerTestSuite) TestTransact_Error() {
	ctx := context.Background()
	expectedErr := errors.New("business logic error")

	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return expectedErr
	})

	suite.Error(err)
	suite.Equal(expectedErr, err)
}

func (suite *NoOpTransactionerTestSuite) TestTransact_ContextPassthrough() {
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("key"), "value")

	err := suite.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Original context values should be preserved
		suite.Equal("value", txCtx.Value(ctxKey("key")))
		return nil
	})

	suite.NoError(err)
}

func (suite *TransactionerTestSuite) TestOnTransactionEnd_RunsAfterCommit() {
	suite.mock.ExpectBegin()
	suite.mock.ExpectCommit()

	ran := false
	err := suite.transactioner.Transact(context.Background(), func(txCtx context.Context) error {
		OnTransactionEnd(txCtx, func() { ran = true })
		suite.False(ran, "an end hook does not run before the transaction ends")
		return nil
	})

	suite.NoError(err)
	suite.True(ran)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestOnTransactionEnd_RunsAfterRollback() {
	suite.mock.ExpectBegin()
	suite.mock.ExpectRollback()

	ran := false
	err := suite.transactioner.Transact(context.Background(), func(txCtx context.Context) error {
		OnTransactionEnd(txCtx, func() { ran = true })
		return errors.New("business logic error")
	})

	suite.Error(err)
	suite.True(ran)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

// A hook registered in a nested transaction on the same database runs once the outer one ends.
func (suite *TransactionerTestSuite) TestOnTransactionEnd_NestedRunsAfterTheOuterTransaction() {
	suite.mock.ExpectBegin()
	suite.mock.ExpectCommit()

	ran := false
	err := suite.transactioner.Transact(context.Background(), func(outerCtx context.Context) error {
		innerErr := suite.transactioner.Transact(outerCtx, func(innerCtx context.Context) error {
			OnTransactionEnd(innerCtx, func() { ran = true })
			return nil
		})
		suite.False(ran, "the nested call does not end the transaction")
		return innerErr
	})

	suite.NoError(err)
	suite.True(ran)
	suite.NoError(suite.mock.ExpectationsWereMet())
}

func (suite *TransactionerTestSuite) TestOnTransactionEnd_DoesNothingOutsideATransaction() {
	ran := false
	OnTransactionEnd(context.Background(), func() { ran = true })
	suite.False(ran)
}

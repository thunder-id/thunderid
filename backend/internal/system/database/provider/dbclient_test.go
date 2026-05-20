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

package provider

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"

	"github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/transaction"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type DBClientTestSuite struct {
	suite.Suite
	mockDB   *sql.DB
	mock     sqlmock.Sqlmock
	dbClient DBClientInterface
}

func TestDBClientSuite(t *testing.T) {
	suite.Run(t, new(DBClientTestSuite))
}

func (suite *DBClientTestSuite) SetupTest() {
	var err error
	suite.mockDB, suite.mock, err = sqlmock.New()
	if err != nil {
		suite.T().Fatalf("Failed to create mock database: %v", err)
	}

	db := model.NewDB(suite.mockDB)
	suite.dbClient = NewDBClient(db, "mock", "test", retryConfig{})
}

func (suite *DBClientTestSuite) TearDownTest() {
	if suite.mock != nil {
		if err := suite.mock.ExpectationsWereMet(); err != nil {
			suite.T().Fatalf("There were unfulfilled expectations: %v", err)
		}
	}
}

func (suite *DBClientTestSuite) TestQuerySuccess() {
	testQuery := model.DBQuery{
		ID:    "test_query_success",
		Query: "SELECT id, name FROM users WHERE id = ?",
	}
	args := []interface{}{1}
	mockArgs := []driver.Value{1}

	columns := []string{"id", "name"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "John Doe").
		AddRow(2, "Jane Smith")
	suite.mock.ExpectQuery("SELECT id, name FROM users WHERE id = ?").
		WithArgs(mockArgs...).
		WillReturnRows(rows)

	// Execute the Query method
	results, err := suite.dbClient.Query(testQuery, args...)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), results, 2)
	assert.Equal(suite.T(), int64(1), results[0]["id"])
	assert.Equal(suite.T(), "John Doe", results[0]["name"])
	assert.Equal(suite.T(), int64(2), results[1]["id"])
	assert.Equal(suite.T(), "Jane Smith", results[1]["name"])
	assert.NoError(suite.T(), suite.mock.ExpectationsWereMet())
}

func (suite *DBClientTestSuite) TestQueryEmptyResults() {
	testQuery := model.DBQuery{
		ID:    "test_query_empty",
		Query: "SELECT id, name FROM users WHERE id = ?",
	}
	args := []interface{}{999}
	mockArgs := []driver.Value{999}

	columns := []string{"id", "name"}
	rows := sqlmock.NewRows(columns)
	suite.mock.ExpectQuery("SELECT id, name FROM users WHERE id = ?").
		WithArgs(mockArgs...).
		WillReturnRows(rows)

	results, err := suite.dbClient.Query(testQuery, args...)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), results)
	assert.NoError(suite.T(), suite.mock.ExpectationsWereMet())
}

func (suite *DBClientTestSuite) TestQueryDatabaseError() {
	testQuery := model.DBQuery{
		ID:    "test_query_error",
		Query: "SELECT id, name FROM non_existent_table",
	}

	expectedErr := errors.New("table not found")
	suite.mock.ExpectQuery("SELECT id, name FROM non_existent_table").
		WillReturnError(expectedErr)

	results, err := suite.dbClient.Query(testQuery)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), expectedErr, err)
	assert.Nil(suite.T(), results)
	assert.NoError(suite.T(), suite.mock.ExpectationsWereMet())
}

func (suite *DBClientTestSuite) TestExecuteSuccess() {
	testQuery := model.DBQuery{
		ID:    "test_execute_success",
		Query: "UPDATE users SET name = ? WHERE id = ?",
	}
	args := []interface{}{"Jane Doe", 1}
	mockArgs := []driver.Value{"Jane Doe", 1}

	suite.mock.ExpectExec("UPDATE users SET name = \\? WHERE id = \\?").
		WithArgs(mockArgs...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	rowsAffected, err := suite.dbClient.Execute(testQuery, args...)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), rowsAffected)
	assert.NoError(suite.T(), suite.mock.ExpectationsWereMet())
}

func (suite *DBClientTestSuite) TestExecuteMultipleRowsAffected() {
	testQuery := model.DBQuery{
		ID:    "test_execute_multiple",
		Query: "DELETE FROM users WHERE role = ?",
	}
	args := []interface{}{"guest"}
	mockArgs := []driver.Value{"guest"}

	// Setup mock to return result with multiple rows affected
	suite.mock.ExpectExec("DELETE FROM users WHERE role = ?").
		WithArgs(mockArgs...).
		WillReturnResult(sqlmock.NewResult(0, 5))

	rowsAffected, err := suite.dbClient.Execute(testQuery, args...)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(5), rowsAffected)
	assert.NoError(suite.T(), suite.mock.ExpectationsWereMet())
}

func (suite *DBClientTestSuite) TestExecuteZeroRowsAffected() {
	testQuery := model.DBQuery{
		ID:    "test_execute_zero",
		Query: "UPDATE users SET name = ? WHERE id = ?",
	}
	args := []interface{}{"Jane Doe", 999}
	mockArgs := []driver.Value{"Jane Doe", 999}

	// Setup mock to return result with zero rows affected
	suite.mock.ExpectExec("UPDATE users SET name = \\? WHERE id = \\?").
		WithArgs(mockArgs...).
		WillReturnResult(sqlmock.NewResult(0, 0))

	rowsAffected, err := suite.dbClient.Execute(testQuery, args...)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(0), rowsAffected)
	assert.NoError(suite.T(), suite.mock.ExpectationsWereMet())
}

func (suite *DBClientTestSuite) TestExecuteDatabaseError() {
	testQuery := model.DBQuery{
		ID:    "test_execute_db_error",
		Query: "UPDATE non_existent_table SET name = ? WHERE id = ?",
	}
	args := []interface{}{"Jane Doe", 1}
	mockArgs := []driver.Value{"Jane Doe", 1}

	expectedErr := errors.New("table not found")
	suite.mock.ExpectExec("UPDATE non_existent_table SET name = \\? WHERE id = \\?").
		WithArgs(mockArgs...).
		WillReturnError(expectedErr)

	rowsAffected, err := suite.dbClient.Execute(testQuery, args...)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), expectedErr, err)
	assert.Equal(suite.T(), int64(0), rowsAffected)
	assert.NoError(suite.T(), suite.mock.ExpectationsWereMet())
}

func (suite *DBClientTestSuite) TestExecuteRowsAffectedError() {
	testQuery := model.DBQuery{
		ID:    "test_execute_rows_error",
		Query: "INSERT INTO users (name) VALUES (?)",
	}
	args := []interface{}{"John Doe"}
	mockArgs := []driver.Value{"John Doe"}

	// Setup mock to return result with error on RowsAffected call
	result := sqlmock.NewErrorResult(errors.New("rows affected error"))
	suite.mock.ExpectExec("INSERT INTO users \\(name\\) VALUES \\(\\?\\)").
		WithArgs(mockArgs...).
		WillReturnResult(result)

	rowsAffected, err := suite.dbClient.Execute(testQuery, args...)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "rows affected error")
	assert.Equal(suite.T(), int64(0), rowsAffected)
	assert.NoError(suite.T(), suite.mock.ExpectationsWereMet())
}

func (suite *DBClientTestSuite) TestBeginTxSuccess() {
	// Setup mock to begin transaction
	suite.mock.ExpectBegin()

	tx, err := suite.dbClient.BeginTx()

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), tx)
	assert.Implements(suite.T(), (*model.TxInterface)(nil), tx)
	assert.NoError(suite.T(), suite.mock.ExpectationsWereMet())
}

func (suite *DBClientTestSuite) TestBeginTxError() {
	// Setup mock to return error
	expectedErr := errors.New("transaction error")
	suite.mock.ExpectBegin().WillReturnError(expectedErr)

	tx, err := suite.dbClient.BeginTx()

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), expectedErr, err)
	assert.Nil(suite.T(), tx)
	assert.NoError(suite.T(), suite.mock.ExpectationsWereMet())
}

func (suite *DBClientTestSuite) TestQueryContextWithoutTransaction() {
	testQuery := model.DBQuery{
		ID:    "test_query_ctx_no_tx",
		Query: "SELECT id, name FROM users WHERE id = ?",
	}
	args := []interface{}{1}
	mockArgs := []driver.Value{1}

	columns := []string{"id", "name"}
	rows := sqlmock.NewRows(columns).AddRow(1, "John Doe")
	suite.mock.ExpectQuery("SELECT id, name FROM users WHERE id = ?").
		WithArgs(mockArgs...).
		WillReturnRows(rows)

	ctx := context.Background()
	results, err := suite.dbClient.QueryContext(ctx, testQuery, args...)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), results, 1)
	assert.Equal(suite.T(), int64(1), results[0]["id"])
	assert.Equal(suite.T(), "John Doe", results[0]["name"])
}

func (suite *DBClientTestSuite) TestQueryContextWithTransaction() {
	testQuery := model.DBQuery{
		ID:    "test_query_ctx_with_tx",
		Query: "SELECT id FROM users",
	}

	columns := []string{"id"}
	rows := sqlmock.NewRows(columns).AddRow(1)

	suite.mock.ExpectBegin()
	suite.mock.ExpectQuery("SELECT id FROM users").WillReturnRows(rows)

	tx, _ := suite.mockDB.Begin()
	ctx := transaction.WithKeyedTx(context.Background(), "test", tx)

	results, err := suite.dbClient.QueryContext(ctx, testQuery)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), results, 1)
	assert.Equal(suite.T(), int64(1), results[0]["id"])
}

func (suite *DBClientTestSuite) TestQueryContextError() {
	testQuery := model.DBQuery{
		ID:    "test_query_ctx_error",
		Query: "SELECT * FROM invalid",
	}

	expectedErr := errors.New("query error")
	suite.mock.ExpectQuery("SELECT \\* FROM invalid").WillReturnError(expectedErr)

	ctx := context.Background()
	results, err := suite.dbClient.QueryContext(ctx, testQuery)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), results)
	assert.Equal(suite.T(), expectedErr, err)
}

func (suite *DBClientTestSuite) TestQueryContextRetriesOnTransientError() {
	retryClient := NewDBClient(model.NewDB(suite.mockDB), "mock", "test", retryConfig{
		MaxAttempts: 2,
		MinBackoff:  time.Millisecond,
		MaxBackoff:  time.Millisecond,
		RandFloat64: func() float64 { return 0 },
	})

	testQuery := model.DBQuery{
		ID:    "test_query_ctx_retry",
		Query: "SELECT id FROM users WHERE id = ?",
	}

	suite.mock.ExpectQuery("SELECT id FROM users WHERE id = ?").
		WithArgs(1).
		WillReturnError(&net.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED},
		})

	rows := sqlmock.NewRows([]string{"id"}).AddRow(1)
	suite.mock.ExpectQuery("SELECT id FROM users WHERE id = ?").
		WithArgs(1).
		WillReturnRows(rows)

	results, err := retryClient.QueryContext(context.Background(), testQuery, 1)

	assert.NoError(suite.T(), err)
	assert.Len(suite.T(), results, 1)
	assert.Equal(suite.T(), int64(1), results[0]["id"])
}

func (suite *DBClientTestSuite) TestQueryContextDoesNotRetryOnNonRetryableError() {
	testQuery := model.DBQuery{
		ID:    "test_query_ctx_no_retry",
		Query: "SELECT id FROM users WHERE id = ?",
	}

	expectedErr := errors.New("syntax error at or near SELECT")
	suite.mock.ExpectQuery("SELECT id FROM users WHERE id = ?").
		WithArgs(1).
		WillReturnError(expectedErr)

	results, err := suite.dbClient.QueryContext(context.Background(), testQuery, 1)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), expectedErr, err)
	assert.Nil(suite.T(), results)
}

func (suite *DBClientTestSuite) TestExecuteContextWithoutTransaction() {
	testQuery := model.DBQuery{
		ID:    "test_execute_ctx_no_tx",
		Query: "UPDATE users SET name = ? WHERE id = ?",
	}
	args := []interface{}{"Updated Name", 1}
	mockArgs := []driver.Value{"Updated Name", 1}

	suite.mock.ExpectExec("UPDATE users SET name = \\? WHERE id = \\?").
		WithArgs(mockArgs...).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ctx := context.Background()
	rowsAffected, err := suite.dbClient.ExecuteContext(ctx, testQuery, args...)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), rowsAffected)
}

func (suite *DBClientTestSuite) TestExecuteContextWithTransaction() {
	testQuery := model.DBQuery{
		ID:    "test_execute_ctx_with_tx",
		Query: "DELETE FROM users WHERE id = ?",
	}
	args := []interface{}{1}

	suite.mock.ExpectBegin()
	suite.mock.ExpectExec("DELETE FROM users WHERE id = \\?").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	tx, _ := suite.mockDB.Begin()
	ctx := transaction.WithKeyedTx(context.Background(), "test", tx)

	rowsAffected, err := suite.dbClient.ExecuteContext(ctx, testQuery, args...)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), int64(1), rowsAffected)
}

func (suite *DBClientTestSuite) TestExecuteContextError() {
	testQuery := model.DBQuery{
		ID:    "test_execute_ctx_error",
		Query: "UPDATE invalid SET x = 1",
	}

	expectedErr := errors.New("execute error")
	suite.mock.ExpectExec("UPDATE invalid SET x = 1").WillReturnError(expectedErr)

	ctx := context.Background()
	rowsAffected, err := suite.dbClient.ExecuteContext(ctx, testQuery)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), int64(0), rowsAffected)
	assert.Equal(suite.T(), expectedErr, err)
}

func (suite *DBClientTestSuite) TestExecuteContextRowsAffectedError() {
	testQuery := model.DBQuery{
		ID:    "test_execute_ctx_rows_error",
		Query: "UPDATE users SET name = ?",
	}
	args := []interface{}{"Test"}

	expectedErr := errors.New("rows affected error")
	suite.mock.ExpectExec("UPDATE users SET name = \\?").
		WithArgs("Test").
		WillReturnResult(sqlmock.NewErrorResult(expectedErr))

	ctx := context.Background()
	rowsAffected, err := suite.dbClient.ExecuteContext(ctx, testQuery, args...)

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), int64(0), rowsAffected)
}

func (suite *DBClientTestSuite) TestExecuteContextDoesNotRetryToAvoidDuplicateWrites() {
	testQuery := model.DBQuery{
		ID:    "test_execute_ctx_no_retry",
		Query: "INSERT INTO users(name) VALUES (?)",
	}

	suite.mock.ExpectExec(`INSERT INTO users\(name\) VALUES \(\?\)`).
		WithArgs("Alice").
		WillReturnError(&net.OpError{
			Op:  "dial",
			Net: "tcp",
			Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNREFUSED},
		})

	rowsAffected, err := suite.dbClient.ExecuteContext(context.Background(), testQuery, "Alice")

	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), int64(0), rowsAffected)
}

func (suite *DBClientTestSuite) TestIsRetryableDBError() {
	assert.True(suite.T(), isRetryableDBError(driver.ErrBadConn))
	assert.True(suite.T(), isRetryableDBError(context.DeadlineExceeded))
	assert.True(suite.T(), isRetryableDBError(&pq.Error{Code: "40P01"}))
	assert.True(suite.T(), isRetryableDBError(&net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: &os.SyscallError{Syscall: "connect", Err: syscall.ECONNRESET},
	}))
	assert.False(suite.T(), isRetryableDBError(sql.ErrNoRows))
	assert.False(suite.T(), isRetryableDBError(errors.New("syntax error near FROM")))
}

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"

	"github.com/thunder-id/thunderid/internal/system/config"

	_ "modernc.org/sqlite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"

	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

const testDeploymentID = "test-deployment-id"

// testRuntimePersistentDBName keys the transaction a sqlmock-backed transactioner places on the context.
const testRuntimePersistentDBName = "runtime_persistent"

type RevocationStoreTestSuite struct {
	suite.Suite
	mockdbProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *revocationStore
	testToken      RevokedToken
	testCriterion  revocationCriterion
}

func TestRevocationStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RevocationStoreTestSuite))
}

func (suite *RevocationStoreTestSuite) SetupTest() {
	// The store resolves its deployment from the loaded runtime rather than holding one, and
	// other suites in this package reset the runtime, so load it per test.
	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime("", &config.Config{
		Server: engineconfig.ServerConfig{Identifier: testDeploymentID},
	})
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			RuntimePersistent: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockdbProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())

	suite.store = &revocationStore{
		dbProvider: suite.mockdbProvider,
	}

	suite.testToken = RevokedToken{
		ID:               "test-revoked-id",
		JTI:              "test-jti",
		RevocationReason: RevocationReasonExplicit,
		RevokedAt:        time.Now().UTC(),
		ExpiryTime:       time.Now().UTC().Add(time.Hour),
	}

	suite.testCriterion = revocationCriterion{
		ID:         "test-criterion-id",
		Type:       CriterionTypeTokenFamily,
		Value:      "tfid-123",
		Reason:     RevocationReasonRefreshReplay,
		RevokedAt:  time.Now().UTC(),
		ExpiryTime: time.Now().UTC().Add(time.Hour),
	}
}

func (suite *RevocationStoreTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *RevocationStoreTestSuite) TestNewRevocationStore() {
	store := newRevocationStore()
	assert.NotNil(suite.T(), store)
	assert.Implements(suite.T(), (*revocationStoreInterface)(nil), store)
}

func (suite *RevocationStoreTestSuite) TestInsertRevokedToken_Success() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)

	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertRevokedToken,
		suite.testToken.ID, suite.testToken.JTI,
		string(suite.testToken.RevocationReason), suite.testToken.RevokedAt, suite.testToken.ExpiryTime,
		testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.InsertRevokedToken(context.Background(), suite.testToken)
	assert.NoError(suite.T(), err)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestInsertRevokedToken_GeneratesIDWhenEmpty() {
	suite.testToken.ID = ""
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)

	// ID is generated internally, so it is matched with mock.Anything.
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertRevokedToken,
		mock.Anything, suite.testToken.JTI,
		string(suite.testToken.RevocationReason), suite.testToken.RevokedAt, suite.testToken.ExpiryTime,
		testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.InsertRevokedToken(context.Background(), suite.testToken)
	assert.NoError(suite.T(), err)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestInsertRevokedToken_DBClientError() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(nil, errors.New("db client error"))

	err := suite.store.InsertRevokedToken(context.Background(), suite.testToken)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "db client error")

	suite.mockdbProvider.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestInsertRevokedToken_ExecError() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)

	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertRevokedToken,
		suite.testToken.ID, suite.testToken.JTI,
		string(suite.testToken.RevocationReason), suite.testToken.RevokedAt, suite.testToken.ExpiryTime,
		testDeploymentID).
		Return(int64(0), errors.New("execute error"))

	err := suite.store.InsertRevokedToken(context.Background(), suite.testToken)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error inserting revoked token")

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestIsTokenRevoked_True() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)

	suite.mockDBClient.On("QueryContext", mock.Anything, queryIsTokenRevoked,
		"test-jti", mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{{"1": 1}}, nil)

	revoked, err := suite.store.IsTokenRevoked(context.Background(), "test-jti")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), revoked)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestIsTokenRevoked_False() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)

	suite.mockDBClient.On("QueryContext", mock.Anything, queryIsTokenRevoked,
		"test-jti", mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	revoked, err := suite.store.IsTokenRevoked(context.Background(), "test-jti")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), revoked)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestIsTokenRevoked_DBClientError() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(nil, errors.New("db client error"))

	revoked, err := suite.store.IsTokenRevoked(context.Background(), "test-jti")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), revoked)

	suite.mockdbProvider.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestIsTokenRevoked_QueryError() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)

	suite.mockDBClient.On("QueryContext", mock.Anything, queryIsTokenRevoked,
		"test-jti", mock.Anything, testDeploymentID).
		Return([]map[string]interface{}(nil), errors.New("query error"))

	revoked, err := suite.store.IsTokenRevoked(context.Background(), "test-jti")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), revoked)
	assert.Contains(suite.T(), err.Error(), "error checking token revocation")

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestInsertCriterion_Success() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)

	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertRevocationCriterion,
		suite.testCriterion.ID, string(suite.testCriterion.Type), suite.testCriterion.Value,
		string(suite.testCriterion.Reason), suite.testCriterion.RevokedAt, suite.testCriterion.ExpiryTime,
		testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.insertCriterion(context.Background(), suite.testCriterion)
	assert.NoError(suite.T(), err)
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestInsertCriterion_GeneratesIDWhenEmpty() {
	suite.testCriterion.ID = ""
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)

	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertRevocationCriterion,
		mock.Anything, string(suite.testCriterion.Type), suite.testCriterion.Value,
		string(suite.testCriterion.Reason), suite.testCriterion.RevokedAt, suite.testCriterion.ExpiryTime,
		testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.insertCriterion(context.Background(), suite.testCriterion)
	assert.NoError(suite.T(), err)
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestInsertCriterion_WithBoundaryReason() {
	suite.testCriterion.Reason = RevocationReasonApplicationSecretRegenerated
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertRevocationCriterion,
		suite.testCriterion.ID, string(suite.testCriterion.Type), suite.testCriterion.Value,
		string(RevocationReasonApplicationSecretRegenerated), suite.testCriterion.RevokedAt,
		suite.testCriterion.ExpiryTime, testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.insertCriterion(context.Background(), suite.testCriterion)
	assert.NoError(suite.T(), err)
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *RevocationStoreTestSuite) TestInsertCriterion_DBClientError() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(nil, errors.New("db client error"))

	err := suite.store.insertCriterion(context.Background(), suite.testCriterion)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "db client error")
}

func (suite *RevocationStoreTestSuite) TestInsertCriterion_ExecError() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertRevocationCriterion,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		testDeploymentID).
		Return(int64(0), errors.New("execute error"))

	err := suite.store.insertCriterion(context.Background(), suite.testCriterion)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error inserting revocation criterion")
}

func (suite *RevocationStoreTestSuite) TestInsertCriteria_EmptySliceSkipsTheClient() {
	err := suite.store.insertCriteria(context.Background(), nil)
	assert.NoError(suite.T(), err)
	suite.mockdbProvider.AssertNotCalled(suite.T(), "GetRuntimePersistentDBClient")
}

func (suite *RevocationStoreTestSuite) TestInsertCriteria_GeneratesIDsForRowsWithoutOne() {
	suite.expectNoOpTransaction()
	rows := []revocationCriterion{
		{Type: CriterionTypeApplicationKey, Value: "client-1", Reason: RevocationReasonApplicationDeleted,
			RevokedAt: suite.testCriterion.RevokedAt, ExpiryTime: suite.testCriterion.ExpiryTime},
		{ID: "already-set", Type: CriterionTypeSubject, Value: "user-1",
			Reason:    RevocationReasonUserDeleted,
			RevokedAt: suite.testCriterion.RevokedAt, ExpiryTime: suite.testCriterion.ExpiryTime},
	}
	var query dbmodel.DBQuery
	var args []interface{}
	suite.mockDBClient.On("ExecuteContext", anyExecuteContextArgs(2)...).
		Run(func(callArgs mock.Arguments) {
			query = callArgs.Get(1).(dbmodel.DBQuery)
			args = callArgs[2:]
		}).
		Return(int64(2), nil)

	err := suite.store.insertCriteria(context.Background(), rows)

	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), query.Query)
	assert.Len(suite.T(), args, 2*criteriaInsertColumns)
	assert.NotEmpty(suite.T(), args[0], "the row with no id must have one generated")
	assert.Equal(suite.T(), "already-set", args[criteriaInsertColumns],
		"a row that already carries an id keeps it")
}

// Refused whatever the order, so the store never picks which of two revocations stands.
func (suite *RevocationStoreTestSuite) TestInsertCriteria_RejectsRepeatedCriterion() {
	now := suite.testCriterion.RevokedAt
	terminal := revocationCriterion{Type: CriterionTypeSubject, Value: "user-1",
		Reason: RevocationReasonUserDeleted, RevokedAt: now, ExpiryTime: now.Add(time.Hour)}
	boundary := revocationCriterion{Type: CriterionTypeSubject, Value: "user-1",
		Reason: RevocationReasonRoleAssignmentRemoved, RevokedAt: now.Add(time.Hour),
		ExpiryTime: now.Add(2 * time.Hour)}

	testCases := []struct {
		name string
		rows []revocationCriterion
	}{
		{"TerminalThenBoundary", []revocationCriterion{terminal, boundary}},
		{"BoundaryThenTerminal", []revocationCriterion{boundary, terminal}},
		{"Identical", []revocationCriterion{boundary, boundary}},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockdbProvider = providermock.NewDBProviderInterfaceMock(suite.T())
			suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
			suite.store.dbProvider = suite.mockdbProvider
			suite.expectNoOpTransaction()

			err := suite.store.insertCriteria(context.Background(), tc.rows)

			suite.Require().Error(err)
			suite.Contains(err.Error(), "repeats criterion")
			suite.mockDBClient.AssertNotCalled(suite.T(), "ExecuteContext")
		})
	}
}

func (suite *RevocationStoreTestSuite) TestInsertCriteria_ChunksAboveTheStatementLimit() {
	suite.expectNoOpTransaction()
	rowCount := maxCriteriaRowsPerStatement + 1
	rows := make([]revocationCriterion, rowCount)
	for i := range rows {
		rows[i] = revocationCriterion{
			ID: fmt.Sprintf("row-%d", i), Type: CriterionTypeEntityScope,
			Value: fmt.Sprintf("digest-%d", i), Reason: RevocationReasonRoleAssignmentRemoved,
			RevokedAt: suite.testCriterion.RevokedAt, ExpiryTime: suite.testCriterion.ExpiryTime,
		}
	}
	var chunkSizes []int
	record := func(callArgs mock.Arguments) {
		chunkSizes = append(chunkSizes, len(callArgs[2:])/criteriaInsertColumns)
	}
	suite.mockDBClient.On("ExecuteContext", anyExecuteContextArgs(maxCriteriaRowsPerStatement)...).
		Run(record).Return(int64(1), nil).Once()
	suite.mockDBClient.On("ExecuteContext", anyExecuteContextArgs(1)...).
		Run(record).Return(int64(1), nil).Once()

	err := suite.store.insertCriteria(context.Background(), rows)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), []int{maxCriteriaRowsPerStatement, 1}, chunkSizes,
		"one full statement and one carrying the remainder")
}

func anyExecuteContextArgs(rows int) []interface{} {
	args := make([]interface{}, 2+rows*criteriaInsertColumns)
	for i := range args {
		args[i] = mock.Anything
	}
	return args
}

func (suite *RevocationStoreTestSuite) TestInsertCriteria_DBClientError() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(nil, errors.New("db client error"))

	err := suite.store.insertCriteria(context.Background(), []revocationCriterion{suite.testCriterion})
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "db client error")
}

func (suite *RevocationStoreTestSuite) TestInsertCriteria_ExecError() {
	suite.expectNoOpTransaction()
	suite.mockDBClient.On("ExecuteContext", anyExecuteContextArgs(1)...).
		Return(int64(0), errors.New("execute error"))

	err := suite.store.insertCriteria(context.Background(), []revocationCriterion{suite.testCriterion})
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error inserting revocation criteria")
}

func (suite *RevocationStoreTestSuite) TestInsertCriteria_TransactionerError() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockdbProvider.On("GetRuntimePersistentDBTransactioner").
		Return(nil, errors.New("transactioner error"))

	err := suite.store.insertCriteria(context.Background(), []revocationCriterion{suite.testCriterion})
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "transactioner error")
	suite.mockDBClient.AssertNotCalled(suite.T(), "ExecuteContext")
}

// A batch spanning several statements is one revocation.
func (suite *RevocationStoreTestSuite) TestInsertCriteria_RollsBackEarlierChunksWhenALaterOneFails() {
	db, sqlMock, err := sqlmock.New()
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { _ = db.Close() })
	sqlMock.ExpectBegin()
	sqlMock.ExpectRollback()
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockdbProvider.On("GetRuntimePersistentDBTransactioner").
		Return(transaction.NewTransactioner(db, testRuntimePersistentDBName), nil)

	rows := make([]revocationCriterion, maxCriteriaRowsPerStatement+1)
	for i := range rows {
		rows[i] = revocationCriterion{
			ID: fmt.Sprintf("row-%d", i), Type: CriterionTypeEntityScope,
			Value: fmt.Sprintf("digest-%d", i), Reason: RevocationReasonRoleAssignmentRemoved,
			RevokedAt: suite.testCriterion.RevokedAt, ExpiryTime: suite.testCriterion.ExpiryTime,
		}
	}
	inTransaction := func(callArgs mock.Arguments) {
		ctx := callArgs.Get(0).(context.Context)
		suite.True(transaction.HasKeyedTx(ctx, testRuntimePersistentDBName),
			"every chunk must be written inside the one transaction")
	}
	suite.mockDBClient.On("ExecuteContext", anyExecuteContextArgs(maxCriteriaRowsPerStatement)...).
		Run(inTransaction).Return(int64(maxCriteriaRowsPerStatement), nil).Once()
	suite.mockDBClient.On("ExecuteContext", anyExecuteContextArgs(1)...).
		Run(inTransaction).Return(int64(0), errors.New("second chunk failed")).Once()

	err = suite.store.insertCriteria(context.Background(), rows)

	suite.Require().Error(err)
	suite.Contains(err.Error(), "second chunk failed")
	suite.NoError(sqlMock.ExpectationsWereMet(), "the transaction must roll back, never commit")
}

func (suite *RevocationStoreTestSuite) TestInsertCriteria_JoinsAnOuterTransaction() {
	db, sqlMock, err := sqlmock.New()
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { _ = db.Close() })
	sqlMock.ExpectBegin()
	outerTx, err := db.Begin()
	suite.Require().NoError(err)
	ctx := transaction.WithKeyedTx(context.Background(), testRuntimePersistentDBName, outerTx)
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockdbProvider.On("GetRuntimePersistentDBTransactioner").
		Return(transaction.NewTransactioner(db, testRuntimePersistentDBName), nil)
	suite.mockDBClient.On("ExecuteContext", anyExecuteContextArgs(1)...).
		Run(func(callArgs mock.Arguments) {
			txCtx := callArgs.Get(0).(context.Context)
			suite.Same(outerTx, transaction.KeyedTxFromContext(txCtx, testRuntimePersistentDBName))
		}).
		Return(int64(1), nil)

	err = suite.store.insertCriteria(ctx, []revocationCriterion{suite.testCriterion})

	suite.Require().NoError(err)
	suite.NoError(sqlMock.ExpectationsWereMet(), "no second transaction may be begun or committed")
}

func (suite *RevocationStoreTestSuite) expectNoOpTransaction() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockdbProvider.On("GetRuntimePersistentDBTransactioner").
		Return(transaction.NewNoOpTransactioner(), nil)
}

func (suite *RevocationStoreTestSuite) TestAreCriteriaRevoked_True() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, mock.Anything,
		testDeploymentID, mock.Anything, time.Time{}, string(CriterionTypeTokenFamily), "tfid-123").
		Return([]map[string]interface{}{{"1": 1}}, nil)

	revoked, err := suite.store.areCriteriaRevoked(context.Background(),
		[]Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-123"}}, time.Time{})
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), revoked)
}

func (suite *RevocationStoreTestSuite) TestAreCriteriaRevoked_False() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, mock.Anything,
		testDeploymentID, mock.Anything, time.Time{}, string(CriterionTypeTokenFamily), "tfid-123").
		Return([]map[string]interface{}{}, nil)

	revoked, err := suite.store.areCriteriaRevoked(context.Background(),
		[]Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-123"}}, time.Time{})
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), revoked)
}

func (suite *RevocationStoreTestSuite) TestAreCriteriaRevoked_PassesEstablishedAt() {
	establishedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, mock.Anything,
		testDeploymentID, mock.Anything, establishedAt, string(CriterionTypeSubject), "user-123").
		Return([]map[string]interface{}{{"1": 1}}, nil)

	revoked, err := suite.store.areCriteriaRevoked(context.Background(),
		[]Criterion{{Type: CriterionTypeSubject, Value: "user-123"}}, establishedAt)
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), revoked)
}

// Every dimension must be covered by one statement: token validation is a hot path, so a second
// criterion must not cost a second round trip.
func (suite *RevocationStoreTestSuite) TestAreCriteriaRevoked_ChecksAllDimensionsInOneQuery() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, mock.Anything,
		testDeploymentID, mock.Anything, time.Time{},
		string(CriterionTypeTokenFamily), "tfid-123",
		string(CriterionTypeSubject), "user-123").
		Return([]map[string]interface{}{{"1": 1}}, nil).Once()

	revoked, err := suite.store.areCriteriaRevoked(context.Background(), []Criterion{
		{Type: CriterionTypeTokenFamily, Value: "tfid-123"},
		{Type: CriterionTypeSubject, Value: "user-123"},
	}, time.Time{})
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), revoked)
	suite.mockDBClient.AssertNumberOfCalls(suite.T(), "QueryContext", 1)
}

func (suite *RevocationStoreTestSuite) TestAreCriteriaRevoked_EmptyCriteriaSkipsQuery() {
	revoked, err := suite.store.areCriteriaRevoked(context.Background(), nil, time.Time{})
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), revoked)
	suite.mockDBClient.AssertNotCalled(suite.T(), "QueryContext")
}

func (suite *RevocationStoreTestSuite) TestAreCriteriaRevoked_QueryError() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, mock.Anything,
		testDeploymentID, mock.Anything, time.Time{}, string(CriterionTypeTokenFamily), "tfid-123").
		Return([]map[string]interface{}(nil), errors.New("query error"))

	revoked, err := suite.store.areCriteriaRevoked(context.Background(),
		[]Criterion{{Type: CriterionTypeTokenFamily, Value: "tfid-123"}}, time.Time{})
	assert.Error(suite.T(), err)
	assert.False(suite.T(), revoked)
	assert.Contains(suite.T(), err.Error(), "error checking revocation criteria")
}

func (suite *RevocationStoreTestSuite) TestCriterionUpsertDoesNotWeakenTerminalRevocation() {
	db, err := sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { suite.Require().NoError(db.Close()) })
	suite.createCriteriaTable(db)

	now := time.Now().UTC().Truncate(time.Second)
	terminalExpiry := now.Add(24 * time.Hour)
	_, err = db.Exec(queryInsertRevocationCriterion.Query, "terminal", "subject", "user-1",
		"user_deleted", now, terminalExpiry, "deployment-1")
	suite.Require().NoError(err)
	_, err = db.Exec(queryInsertRevocationCriterion.Query, "boundary", "subject", "user-1",
		string(RevocationReasonRoleAssignmentRemoved), now.Add(time.Hour), now.Add(2*time.Hour), "deployment-1")
	suite.Require().NoError(err)

	var reason string
	var revokedAt, expiry time.Time
	err = db.QueryRow(`SELECT REASON, REVOKED_AT, EXPIRY_TIME FROM "REVOCATION_CRITERIA"
		WHERE DEPLOYMENT_ID = ? AND CRITERION_TYPE = ? AND CRITERION_VALUE = ?`,
		"deployment-1", "subject", "user-1").Scan(&reason, &revokedAt, &expiry)
	suite.Require().NoError(err)
	suite.Equal("user_deleted", reason)
	suite.True(revokedAt.Equal(now))
	suite.True(expiry.Equal(terminalExpiry))
}

func (suite *RevocationStoreTestSuite) TestCriterionUpsertNeverMovesABoundaryCutoffBackwards() {
	db, err := sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { suite.Require().NoError(db.Close()) })
	suite.createCriteriaTable(db)

	now := time.Now().UTC().Truncate(time.Second)
	later := now.Add(time.Hour)
	_, err = db.Exec(queryInsertRevocationCriterion.Query, "boundary-late", "subject", "user-1",
		string(RevocationReasonRoleAssignmentRemoved), later, later.Add(time.Hour), "deployment-1")
	suite.Require().NoError(err)
	// The same key written again with an earlier cutoff, as a flow that stamped first but committed second.
	_, err = db.Exec(queryInsertRevocationCriterion.Query, "boundary-early", "subject", "user-1",
		string(RevocationReasonRoleAssignmentRemoved), now, now.Add(time.Hour), "deployment-1")
	suite.Require().NoError(err)

	var revokedAt time.Time
	err = db.QueryRow(`SELECT REVOKED_AT FROM "REVOCATION_CRITERIA"
		WHERE DEPLOYMENT_ID = ? AND CRITERION_TYPE = ? AND CRITERION_VALUE = ?`,
		"deployment-1", "subject", "user-1").Scan(&revokedAt)
	suite.Require().NoError(err)
	suite.True(revokedAt.Equal(later), "the later cutoff must stand")
}

func (suite *RevocationStoreTestSuite) TestCriterionUpsertPromotesBoundaryToTerminalRevocation() {
	db, err := sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { suite.Require().NoError(db.Close()) })
	suite.createCriteriaTable(db)

	now := time.Now().UTC().Truncate(time.Second)
	_, err = db.Exec(queryInsertRevocationCriterion.Query, "boundary", "subject", "user-1",
		string(RevocationReasonRoleAssignmentRemoved), now, now.Add(time.Hour), "deployment-1")
	suite.Require().NoError(err)
	terminalExpiry := now.Add(24 * time.Hour)
	_, err = db.Exec(queryInsertRevocationCriterion.Query, "terminal", "subject", "user-1",
		"user_deleted", now.Add(time.Hour), terminalExpiry, "deployment-1")
	suite.Require().NoError(err)

	var reason string
	var expiry time.Time
	err = db.QueryRow(`SELECT REASON, EXPIRY_TIME FROM "REVOCATION_CRITERIA"
		WHERE DEPLOYMENT_ID = ? AND CRITERION_TYPE = ? AND CRITERION_VALUE = ?`,
		"deployment-1", "subject", "user-1").Scan(&reason, &expiry)
	suite.Require().NoError(err)
	suite.Equal("user_deleted", reason)
	suite.True(expiry.Equal(terminalExpiry))
}

func (suite *RevocationStoreTestSuite) TestBuildInsertRevocationCriteriaQuery_WritesAndUpsertsTogether() {
	db, err := sql.Open("sqlite", ":memory:")
	suite.Require().NoError(err)
	suite.T().Cleanup(func() { suite.Require().NoError(db.Close()) })
	suite.createCriteriaTable(db)

	now := time.Now().UTC().Truncate(time.Second)
	terminalExpiry := now.Add(24 * time.Hour)
	_, err = db.Exec(queryInsertRevocationCriterion.Query, "pre-existing-terminal", "subject", "user-1",
		"user_deleted", now, terminalExpiry, "deployment-1")
	suite.Require().NoError(err)
	_, err = db.Exec(queryInsertRevocationCriterion.Query, "pre-existing-boundary", "entity.scope", "digest-1",
		string(RevocationReasonRoleAssignmentRemoved), now, now.Add(time.Hour), "deployment-1")
	suite.Require().NoError(err)

	rows := []revocationCriterion{
		// A fresh key: must simply be inserted.
		{ID: "fresh-1", Type: CriterionTypeApplicationKey, Value: "client-1",
			Reason: RevocationReasonApplicationDeleted, RevokedAt: now, ExpiryTime: now.Add(time.Hour)},
		// Conflicts with the pre-existing terminal row.
		{ID: "conflict-terminal", Type: CriterionTypeSubject, Value: "user-1",
			Reason: RevocationReasonRoleAssignmentRemoved, RevokedAt: now.Add(2 * time.Hour),
			ExpiryTime: now.Add(2 * time.Hour)},
		// Conflicts with the pre-existing boundary row at a later cutoff: the cutoff must advance.
		{ID: "conflict-boundary", Type: CriterionTypeEntityScope, Value: "digest-1",
			Reason: RevocationReasonRoleAssignmentRemoved, RevokedAt: now.Add(2 * time.Hour),
			ExpiryTime: now.Add(2 * time.Hour)},
	}
	query, args := buildInsertRevocationCriteriaQuery(rows, "deployment-1")
	_, err = db.Exec(query.Query, args...)
	suite.Require().NoError(err)

	var reason string
	var revokedAt time.Time
	err = db.QueryRow(`SELECT REASON, REVOKED_AT FROM "REVOCATION_CRITERIA"
		WHERE DEPLOYMENT_ID = ? AND CRITERION_TYPE = ? AND CRITERION_VALUE = ?`,
		"deployment-1", "app.key", "client-1").Scan(&reason, &revokedAt)
	suite.Require().NoError(err, "the fresh row must have been written")
	suite.Equal(string(RevocationReasonApplicationDeleted), reason)

	err = db.QueryRow(`SELECT REASON, REVOKED_AT FROM "REVOCATION_CRITERIA"
		WHERE DEPLOYMENT_ID = ? AND CRITERION_TYPE = ? AND CRITERION_VALUE = ?`,
		"deployment-1", "subject", "user-1").Scan(&reason, &revokedAt)
	suite.Require().NoError(err)
	suite.Equal("user_deleted", reason, "the terminal reason must survive a boundary row in the batch")

	err = db.QueryRow(`SELECT REASON, REVOKED_AT FROM "REVOCATION_CRITERIA"
		WHERE DEPLOYMENT_ID = ? AND CRITERION_TYPE = ? AND CRITERION_VALUE = ?`,
		"deployment-1", "entity.scope", "digest-1").Scan(&reason, &revokedAt)
	suite.Require().NoError(err)
	suite.True(revokedAt.Equal(now.Add(2*time.Hour)), "the later cutoff in the batch must advance it")
}

func TestBuildInsertRevocationCriteriaQuery_PanicsAboveTheStatementLimit(t *testing.T) {
	rows := make([]revocationCriterion, maxCriteriaRowsPerStatement+1)
	assert.Panics(t, func() { buildInsertRevocationCriteriaQuery(rows, "deployment-1") })
}

func (suite *RevocationStoreTestSuite) createCriteriaTable(db *sql.DB) {
	_, err := db.Exec(`CREATE TABLE "REVOCATION_CRITERIA" (
		ID TEXT PRIMARY KEY, CRITERION_TYPE TEXT NOT NULL, CRITERION_VALUE TEXT NOT NULL,
		REASON TEXT NOT NULL, REVOKED_AT TIMESTAMP NOT NULL, EXPIRY_TIME TIMESTAMP NOT NULL,
		DEPLOYMENT_ID TEXT NOT NULL,
		UNIQUE (DEPLOYMENT_ID, CRITERION_TYPE, CRITERION_VALUE))`)
	suite.Require().NoError(err)
}

// A request names the deployment it acts for, and the store must scope by that rather than by the
// identifier this server was configured with. Getting this wrong reads another deployment's rows,
// which no other assertion here would catch: every other test runs on an unscoped context, where
// the two values coincide.
func (suite *RevocationStoreTestSuite) TestInsertRevokedToken_ScopesByTheRequestDeployment() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertRevokedToken,
		suite.testToken.ID, suite.testToken.JTI,
		string(suite.testToken.RevocationReason), suite.testToken.RevokedAt, suite.testToken.ExpiryTime,
		"acme").
		Return(int64(1), nil)

	err := suite.store.InsertRevokedToken(deployment.WithID(context.Background(), "acme"), suite.testToken)

	assert.NoError(suite.T(), err)
	suite.mockDBClient.AssertExpectations(suite.T())
}

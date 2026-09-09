// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"

	_ "modernc.org/sqlite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"

	"github.com/thunder-id/thunderid/internal/system/deployment"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

const testDeploymentID = "test-deployment-id"

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

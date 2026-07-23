/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package revocation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
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
		dbProvider:   suite.mockdbProvider,
		deploymentID: testDeploymentID,
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
		Type:       criterionTypeTokenFamily,
		Value:      "tfid-123",
		Reason:     RevocationReasonRefreshReuse,
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

func (suite *RevocationStoreTestSuite) TestIsCriterionRevoked_True() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryIsCriterionRevoked,
		string(criterionTypeTokenFamily), "tfid-123", mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{{"1": 1}}, nil)

	revoked, err := suite.store.isCriterionRevoked(context.Background(), criterionTypeTokenFamily, "tfid-123")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), revoked)
}

func (suite *RevocationStoreTestSuite) TestIsCriterionRevoked_False() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryIsCriterionRevoked,
		string(criterionTypeTokenFamily), "tfid-123", mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	revoked, err := suite.store.isCriterionRevoked(context.Background(), criterionTypeTokenFamily, "tfid-123")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), revoked)
}

func (suite *RevocationStoreTestSuite) TestIsCriterionRevoked_QueryError() {
	suite.mockdbProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryIsCriterionRevoked,
		string(criterionTypeTokenFamily), "tfid-123", mock.Anything, testDeploymentID).
		Return([]map[string]interface{}(nil), errors.New("query error"))

	revoked, err := suite.store.isCriterionRevoked(context.Background(), criterionTypeTokenFamily, "tfid-123")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), revoked)
	assert.Contains(suite.T(), err.Error(), "error checking revocation criterion")
}

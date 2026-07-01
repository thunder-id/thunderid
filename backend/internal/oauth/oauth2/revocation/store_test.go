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

type RevokedTokenWriterTestSuite struct {
	suite.Suite
	mockdbProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *revokedTokenWriter
	testToken      RevokedToken
}

func TestRevokedTokenWriterTestSuite(t *testing.T) {
	suite.Run(t, new(RevokedTokenWriterTestSuite))
}

func (suite *RevokedTokenWriterTestSuite) SetupTest() {
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Operation: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockdbProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())

	suite.store = &revokedTokenWriter{
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
}

func (suite *RevokedTokenWriterTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *RevokedTokenWriterTestSuite) TestNewRevokedTokenWriter() {
	store := newRevokedTokenWriter()
	assert.NotNil(suite.T(), store)
	assert.Implements(suite.T(), (*RevokedTokenWriterInterface)(nil), store)
}

func (suite *RevokedTokenWriterTestSuite) TestInsertRevokedToken_Success() {
	suite.mockdbProvider.On("GetOperationDBClient").Return(suite.mockDBClient, nil)

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

func (suite *RevokedTokenWriterTestSuite) TestInsertRevokedToken_GeneratesIDWhenEmpty() {
	suite.testToken.ID = ""
	suite.mockdbProvider.On("GetOperationDBClient").Return(suite.mockDBClient, nil)

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

func (suite *RevokedTokenWriterTestSuite) TestInsertRevokedToken_DBClientError() {
	suite.mockdbProvider.On("GetOperationDBClient").Return(nil, errors.New("db client error"))

	err := suite.store.InsertRevokedToken(context.Background(), suite.testToken)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "db client error")

	suite.mockdbProvider.AssertExpectations(suite.T())
}

func (suite *RevokedTokenWriterTestSuite) TestInsertRevokedToken_ExecError() {
	suite.mockdbProvider.On("GetOperationDBClient").Return(suite.mockDBClient, nil)

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

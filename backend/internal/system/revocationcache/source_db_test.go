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

package revocationcache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "test-deployment-id"

type DBSourceTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	source         *dbSource
}

func TestDBSourceTestSuite(t *testing.T) {
	suite.Run(t, new(DBSourceTestSuite))
}

func (suite *DBSourceTestSuite) SetupTest() {
	suite.mockDBProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.source = &dbSource{
		dbProvider:   suite.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

func (suite *DBSourceTestSuite) TestSnapshot_Success() {
	expiry := time.Now().Add(time.Hour).UTC()
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{
			{"jti": "jti-1", "expiry_time": expiry},
			{"jti": "jti-2", "expiry_time": expiry},
		}, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokenFamilies,
		criterionTypeTokenFamily, mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{
			{"criterion_value": "tfid-1", "expiry_time": expiry},
		}, nil)

	snapshot, err := suite.source.Snapshot(context.Background())

	suite.Require().NoError(err)
	assert.Len(suite.T(), snapshot.Tokens, 2)
	assert.Equal(suite.T(), "jti-1", snapshot.Tokens[0].Value)
	assert.Equal(suite.T(), expiry, snapshot.Tokens[0].ExpiryTime)
	assert.Len(suite.T(), snapshot.Families, 1)
	assert.Equal(suite.T(), "tfid-1", snapshot.Families[0].Value)
}

func (suite *DBSourceTestSuite) TestSnapshot_Empty() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{}, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokenFamilies,
		criterionTypeTokenFamily, mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{}, nil)

	snapshot, err := suite.source.Snapshot(context.Background())

	suite.Require().NoError(err)
	assert.Empty(suite.T(), snapshot.Tokens)
	assert.Empty(suite.T(), snapshot.Families)
}

func (suite *DBSourceTestSuite) TestSnapshot_DBClientError() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(nil, errors.New("db client error"))

	snapshot, err := suite.source.Snapshot(context.Background())

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), snapshot.Tokens)
	assert.Contains(suite.T(), err.Error(), "db client error")
}

func (suite *DBSourceTestSuite) TestSnapshot_QueryError() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return(nil, errors.New("query error"))

	snapshot, err := suite.source.Snapshot(context.Background())

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), snapshot.Tokens)
	assert.Contains(suite.T(), err.Error(), "error reading revoked token snapshot")
}

func (suite *DBSourceTestSuite) TestSnapshot_TokenFamilyQueryError() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{}, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokenFamilies,
		criterionTypeTokenFamily, mock.Anything, testDeploymentID).
		Return(nil, errors.New("query error"))

	snapshot, err := suite.source.Snapshot(context.Background())

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), snapshot.Families)
	assert.Contains(suite.T(), err.Error(), "error reading revoked token family snapshot")
}

func (suite *DBSourceTestSuite) TestSnapshot_InvalidJTI() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{
			{"jti": "", "expiry_time": time.Now().Add(time.Hour)},
		}, nil)

	snapshot, err := suite.source.Snapshot(context.Background())

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), snapshot.Tokens)
	assert.Contains(suite.T(), err.Error(), "jti")
}

func (suite *DBSourceTestSuite) TestSnapshot_InvalidExpiryTime() {
	suite.mockDBProvider.On("GetRuntimePersistentDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, querySnapshotRevokedTokens,
		mock.Anything, testDeploymentID).
		Return([]map[string]interface{}{
			{"jti": "jti-1", "expiry_time": 12345},
		}, nil)

	snapshot, err := suite.source.Snapshot(context.Background())

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), snapshot.Tokens)
	assert.Contains(suite.T(), err.Error(), "error parsing revocation snapshot")
}

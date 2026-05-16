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

package jti

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const testDeploymentID = "deployment-1"

type DBStoreTestSuite struct {
	suite.Suite
	dbProvider *providermock.DBProviderInterfaceMock
	dbClient   *providermock.DBClientInterfaceMock
	store      *jtiStore
}

func TestDBStoreTestSuite(t *testing.T) {
	suite.Run(t, new(DBStoreTestSuite))
}

func (suite *DBStoreTestSuite) SetupTest() {
	suite.dbProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.dbClient = providermock.NewDBClientInterfaceMock(suite.T())
	suite.store = &jtiStore{dbProvider: suite.dbProvider, deploymentID: testDeploymentID}
}

func (suite *DBStoreTestSuite) TestRecordJTI_Inserted() {
	expiry := time.Now().Add(time.Minute).UTC()
	suite.dbProvider.On("GetRuntimeDBClient").Return(suite.dbClient, nil)
	suite.dbClient.On("ExecuteContext", mock.Anything, queryInsertJTI,
		"dpop", "jti-1",
		mock.MatchedBy(func(t time.Time) bool { return !t.IsZero() }),
		testDeploymentID,
	).Return(int64(1), nil)

	inserted, err := suite.store.RecordJTI(context.Background(), "dpop", "jti-1", expiry)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), inserted)
}

func (suite *DBStoreTestSuite) TestRecordJTI_Replay() {
	suite.dbProvider.On("GetRuntimeDBClient").Return(suite.dbClient, nil)
	suite.dbClient.On("ExecuteContext", mock.Anything, queryInsertJTI,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(int64(0), nil)

	inserted, err := suite.store.RecordJTI(context.Background(), "dpop", "jti-1", time.Now().Add(time.Minute))
	require.NoError(suite.T(), err)
	assert.False(suite.T(), inserted, "RowsAffected==0 must be reported as a replay")
}

func (suite *DBStoreTestSuite) TestRecordJTI_DBClientError() {
	suite.dbProvider.On("GetRuntimeDBClient").Return(nil, errors.New("conn failed"))

	inserted, err := suite.store.RecordJTI(context.Background(), "dpop", "jti-1", time.Now().Add(time.Minute))
	require.Error(suite.T(), err)
	assert.False(suite.T(), inserted)
	assert.Contains(suite.T(), err.Error(), "failed to get database client")
}

func (suite *DBStoreTestSuite) TestRecordJTI_ExecuteError() {
	suite.dbProvider.On("GetRuntimeDBClient").Return(suite.dbClient, nil)
	suite.dbClient.On("ExecuteContext", mock.Anything, queryInsertJTI,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(int64(0), errors.New("insert failed"))

	inserted, err := suite.store.RecordJTI(context.Background(), "dpop", "jti-1", time.Now().Add(time.Minute))
	require.Error(suite.T(), err)
	assert.False(suite.T(), inserted)
	assert.Contains(suite.T(), err.Error(), "failed to insert jti")
}

func (suite *DBStoreTestSuite) TestRecordJTI_PassesUTCExpiry() {
	// Local-time inputs must be persisted in UTC for cross-timezone consistency.
	loc, err := time.LoadLocation("America/Los_Angeles")
	require.NoError(suite.T(), err)
	local := time.Now().In(loc)

	suite.dbProvider.On("GetRuntimeDBClient").Return(suite.dbClient, nil)
	suite.dbClient.On("ExecuteContext", mock.Anything, queryInsertJTI,
		"dpop", "jti-utc",
		mock.MatchedBy(func(t time.Time) bool { return t.Location() == time.UTC }),
		testDeploymentID,
	).Return(int64(1), nil)

	_, err = suite.store.RecordJTI(context.Background(), "dpop", "jti-utc", local)
	require.NoError(suite.T(), err)
}

// TestRecordJTI_NamespaceIsolation locks in the contract that two distinct
// namespaces can carry the same jti without colliding — i.e. namespace participates
// in the primary key.
func (suite *DBStoreTestSuite) TestRecordJTI_NamespaceIsolation() {
	suite.dbProvider.On("GetRuntimeDBClient").Return(suite.dbClient, nil)
	suite.dbClient.On("ExecuteContext", mock.Anything, queryInsertJTI,
		"dpop", "j", mock.Anything, testDeploymentID,
	).Return(int64(1), nil).Once()
	suite.dbClient.On("ExecuteContext", mock.Anything, queryInsertJTI,
		"client_assertion", "j", mock.Anything, testDeploymentID,
	).Return(int64(1), nil).Once()

	ok1, err := suite.store.RecordJTI(context.Background(), "dpop", "j", time.Now().Add(time.Minute))
	require.NoError(suite.T(), err)
	assert.True(suite.T(), ok1)
	ok2, err := suite.store.RecordJTI(context.Background(), "client_assertion", "j", time.Now().Add(time.Minute))
	require.NoError(suite.T(), err)
	assert.True(suite.T(), ok2)
}

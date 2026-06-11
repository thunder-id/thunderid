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

package ciba

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

const testDeploymentID = "test-deployment"

type CIBARequestStoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *cibaRequestStore
}

func TestCIBARequestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(CIBARequestStoreTestSuite))
}

func (suite *CIBARequestStoreTestSuite) SetupTest() {
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockDBProvider = &providermock.DBProviderInterfaceMock{}
	suite.mockDBClient = &providermock.DBClientInterfaceMock{}
	suite.store = &cibaRequestStore{
		dbProvider:   suite.mockDBProvider,
		deploymentID: testDeploymentID,
	}
}

func (suite *CIBARequestStoreTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *CIBARequestStoreTestSuite) sampleRequest() *CIBAAuthRequest {
	return &CIBAAuthRequest{
		AuthReqID:      "auth-req-1",
		ClientID:       "client-1",
		StandardScopes: "openid profile",
		State:          CIBAStatePending,
		ExpiryTime:     time.Now().Add(2 * time.Minute),
	}
}

func (suite *CIBARequestStoreTestSuite) TestNewCIBARequestStore() {
	store := newCIBARequestStore()
	assert.NotNil(suite.T(), store)
	assert.Implements(suite.T(), (*CIBARequestStoreInterface)(nil), store)
}

func (suite *CIBARequestStoreTestSuite) TestAdd_Success() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertCIBAAuthRequest,
		"auth-req-1", "client-1", "openid profile", string(CIBAStatePending),
		mock.AnythingOfType("time.Time"), testDeploymentID).Return(int64(1), nil)

	err := suite.store.Add(context.Background(), suite.sampleRequest())
	assert.NoError(suite.T(), err)

	suite.mockDBProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestAdd_DBClientError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db client error"))

	err := suite.store.Add(context.Background(), suite.sampleRequest())
	assert.Error(suite.T(), err)

	suite.mockDBProvider.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestAdd_ExecuteError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertCIBAAuthRequest,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).Return(int64(0), errors.New("execute error"))

	err := suite.store.Add(context.Background(), suite.sampleRequest())
	assert.Error(suite.T(), err)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestGetByID_Success() {
	expiry := time.Now().Add(2 * time.Minute)
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetCIBAAuthRequest,
		"auth-req-1", testDeploymentID).Return([]map[string]interface{}{
		{
			dbColumnAuthReqID:        "auth-req-1",
			dbColumnClientID:         "client-1",
			dbColumnUserID:           "user-1",
			dbColumnStandardScopes:   "openid profile",
			dbColumnState:            string(CIBAStateAuthenticated),
			dbColumnAttributeCacheID: "cache-1",
			dbColumnCompletedACR:     "urn:acr:pwd",
			dbColumnAuthTime:         expiry.Format("2006-01-02 15:04:05.999999999"),
			dbColumnLastPolledAt:     nil,
			dbColumnExpiryTime:       expiry.Format("2006-01-02 15:04:05.999999999"),
		},
	}, nil)

	record, err := suite.store.GetByID(context.Background(), "auth-req-1")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "auth-req-1", record.AuthReqID)
	assert.Equal(suite.T(), "client-1", record.ClientID)
	assert.Equal(suite.T(), "user-1", record.UserID)
	assert.Equal(suite.T(), CIBAStateAuthenticated, record.State)
	assert.Equal(suite.T(), "cache-1", record.AttributeCacheID)
	assert.Equal(suite.T(), "urn:acr:pwd", record.CompletedACR)
	assert.True(suite.T(), record.LastPolledAt.IsZero())

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestGetByID_EmptyID() {
	record, err := suite.store.GetByID(context.Background(), "")
	assert.ErrorIs(suite.T(), err, ErrCIBARequestNotFound)
	assert.Nil(suite.T(), record)
}

func (suite *CIBARequestStoreTestSuite) TestGetByID_NotFound() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetCIBAAuthRequest,
		"missing", testDeploymentID).Return([]map[string]interface{}{}, nil)

	record, err := suite.store.GetByID(context.Background(), "missing")
	assert.ErrorIs(suite.T(), err, ErrCIBARequestNotFound)
	assert.Nil(suite.T(), record)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestGetByID_QueryError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetCIBAAuthRequest,
		"auth-req-1", testDeploymentID).Return(nil, errors.New("query error"))

	record, err := suite.store.GetByID(context.Background(), "auth-req-1")
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), record)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestMarkAuthenticated_Success() {
	authTime := time.Now()
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryMarkCIBAAuthRequestAuthenticated,
		string(CIBAStateAuthenticated), "user-1", "openid customer:update", "cache-1", "urn:acr:pwd",
		authTime.UTC(), "auth-req-1", string(CIBAStatePending), testDeploymentID).Return(int64(1), nil)

	err := suite.store.MarkAuthenticated(context.Background(),
		"auth-req-1", "user-1", "openid customer:update", "cache-1", "urn:acr:pwd", authTime)
	assert.NoError(suite.T(), err)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestMarkAuthenticated_DBClientError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db client error"))

	err := suite.store.MarkAuthenticated(context.Background(),
		"auth-req-1", "user-1", "openid", "cache-1", "acr", time.Now())
	assert.Error(suite.T(), err)

	suite.mockDBProvider.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestMarkAuthenticated_ExecuteError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryMarkCIBAAuthRequestAuthenticated,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(int64(0), errors.New("execute error"))

	err := suite.store.MarkAuthenticated(context.Background(),
		"auth-req-1", "user-1", "openid", "cache-1", "acr", time.Now())
	assert.Error(suite.T(), err)
}

func (suite *CIBARequestStoreTestSuite) TestMarkConsumed_DBClientError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db client error"))

	consumed, err := suite.store.MarkConsumed(context.Background(), "auth-req-1")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), consumed)

	suite.mockDBProvider.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestMarkConsumed_ExecuteError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryConsumeCIBAAuthRequest,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(int64(0), errors.New("execute error"))

	consumed, err := suite.store.MarkConsumed(context.Background(), "auth-req-1")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), consumed)
}

func (suite *CIBARequestStoreTestSuite) TestUpdateLastPolled_DBClientError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db client error"))

	err := suite.store.UpdateLastPolled(context.Background(), "auth-req-1", time.Now())
	assert.Error(suite.T(), err)

	suite.mockDBProvider.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestUpdateLastPolled_ExecuteError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateCIBALastPolled,
		mock.Anything, mock.Anything, mock.Anything).Return(int64(0), errors.New("execute error"))

	err := suite.store.UpdateLastPolled(context.Background(), "auth-req-1", time.Now())
	assert.Error(suite.T(), err)
}

func (suite *CIBARequestStoreTestSuite) TestUpdateState_ExecuteError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateCIBAAuthRequestState,
		mock.Anything, mock.Anything, mock.Anything).Return(int64(0), errors.New("execute error"))

	err := suite.store.UpdateState(context.Background(), "auth-req-1", CIBAStateExpired)
	assert.Error(suite.T(), err)
}

func (suite *CIBARequestStoreTestSuite) TestGetByID_DBClientError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db client error"))

	record, err := suite.store.GetByID(context.Background(), "auth-req-1")
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), record)

	suite.mockDBProvider.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestMarkConsumed_Success() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryConsumeCIBAAuthRequest,
		string(CIBAStateConsumed), "auth-req-1", string(CIBAStateAuthenticated), testDeploymentID).
		Return(int64(1), nil)

	consumed, err := suite.store.MarkConsumed(context.Background(), "auth-req-1")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), consumed)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestMarkConsumed_NoRowsAffected() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryConsumeCIBAAuthRequest,
		string(CIBAStateConsumed), "auth-req-1", string(CIBAStateAuthenticated), testDeploymentID).
		Return(int64(0), nil)

	consumed, err := suite.store.MarkConsumed(context.Background(), "auth-req-1")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), consumed)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestUpdateLastPolled_Success() {
	polledAt := time.Now()
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateCIBALastPolled,
		polledAt.UTC(), "auth-req-1", testDeploymentID).Return(int64(1), nil)

	err := suite.store.UpdateLastPolled(context.Background(), "auth-req-1", polledAt)
	assert.NoError(suite.T(), err)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestUpdateState_Success() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryUpdateCIBAAuthRequestState,
		string(CIBAStateExpired), "auth-req-1", testDeploymentID).Return(int64(1), nil)

	err := suite.store.UpdateState(context.Background(), "auth-req-1", CIBAStateExpired)
	assert.NoError(suite.T(), err)

	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestUpdateState_DBClientError() {
	suite.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db client error"))

	err := suite.store.UpdateState(context.Background(), "auth-req-1", CIBAStateExpired)
	assert.Error(suite.T(), err)

	suite.mockDBProvider.AssertExpectations(suite.T())
}

func (suite *CIBARequestStoreTestSuite) TestIsNumericOffset_ValidPositive() {
	assert.True(suite.T(), isNumericOffset("+0530"))
}

func (suite *CIBARequestStoreTestSuite) TestIsNumericOffset_ValidNegative() {
	assert.True(suite.T(), isNumericOffset("-0700"))
}

func (suite *CIBARequestStoreTestSuite) TestIsNumericOffset_TooShort() {
	assert.False(suite.T(), isNumericOffset("+053"))
}

func (suite *CIBARequestStoreTestSuite) TestIsNumericOffset_WrongSign() {
	assert.False(suite.T(), isNumericOffset("x0530"))
}

func (suite *CIBARequestStoreTestSuite) TestIsNumericOffset_NonDigitSuffix() {
	assert.False(suite.T(), isNumericOffset("+05AB"))
}

func (suite *CIBARequestStoreTestSuite) TestBuildCIBAAuthRequestFromRow_MissingExpiry() {
	_, err := buildCIBAAuthRequestFromRow(map[string]interface{}{
		dbColumnAuthReqID: "auth-req-1",
	})
	assert.Error(suite.T(), err)
}

func (suite *CIBARequestStoreTestSuite) TestStringFromRow() {
	assert.Equal(suite.T(), "value", stringFromRow("value"))
	assert.Equal(suite.T(), "bytes", stringFromRow([]byte("bytes")))
	assert.Equal(suite.T(), "", stringFromRow(123))
	assert.Equal(suite.T(), "", stringFromRow(nil))
}

func (suite *CIBARequestStoreTestSuite) TestParseOptionalTimeField() {
	_, ok := parseOptionalTimeField(nil)
	assert.False(suite.T(), ok)

	now := time.Now()
	parsed, ok := parseOptionalTimeField(now)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), now, parsed)

	_, ok = parseOptionalTimeField(12345)
	assert.False(suite.T(), ok)
}

// TestParseTimeField_PreservesOffset asserts that a time string rendered with a non-UTC offset
// (as Go's time.Time.String() produces for a local-zone value) parses back to the same instant
// rather than being reinterpreted as UTC and shifted by the offset.
func (suite *CIBARequestStoreTestSuite) TestParseTimeField_PreservesOffset() {
	loc := time.FixedZone("IST", 5*3600+30*60)
	original := time.Date(2026, 6, 2, 21, 57, 49, 157215000, loc)

	parsed, err := parseTimeField(original.String(), "expiry_time")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), original.Equal(parsed),
		"expected %s to equal %s (same instant)", original, parsed)
}

// TestParseTimeField_ZonelessTreatedAsUTC asserts that a zoneless time string is interpreted as
// UTC, matching the UTC-normalized write side.
func (suite *CIBARequestStoreTestSuite) TestParseTimeField_ISO8601Format() {
	original := time.Date(2026, 6, 2, 21, 57, 49, 0, time.UTC)
	parsed, err := parseTimeField(original.Format("2006-01-02T15:04:05Z07:00"), "expiry_time")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), original.Equal(parsed))
}

func (suite *CIBARequestStoreTestSuite) TestParseTimeField_UnexpectedType() {
	_, err := parseTimeField(12345, "expiry_time")
	assert.Error(suite.T(), err)
}

func (suite *CIBARequestStoreTestSuite) TestParseTimeField_ZonelessTreatedAsUTC() {
	original := time.Date(2026, 6, 2, 21, 57, 49, 157215000, time.UTC)

	parsed, err := parseTimeField(original.Format("2006-01-02 15:04:05.999999999"), "expiry_time")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), original.Equal(parsed),
		"expected %s to equal %s (same instant)", original, parsed)
}

// TestGetByID_NonUTCStoredTimestampReadsSameInstant exercises the full read path with timestamps
// rendered in a non-UTC zone, asserting the request is not treated as expired when it is still
// valid. This reproduces the timezone round-trip defect at the store boundary.
func (suite *CIBARequestStoreTestSuite) TestGetByID_NonUTCStoredTimestampReadsSameInstant() {
	loc := time.FixedZone("IST", 5*3600+30*60)
	expiry := time.Now().In(loc).Add(2 * time.Minute)
	lastPolled := time.Now().In(loc).Add(-30 * time.Second)
	authTime := time.Now().In(loc).Add(-1 * time.Minute)

	suite.mockDBProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetCIBAAuthRequest,
		"auth-req-1", testDeploymentID).Return([]map[string]interface{}{
		{
			dbColumnAuthReqID:      "auth-req-1",
			dbColumnClientID:       "client-1",
			dbColumnUserID:         "user-1",
			dbColumnStandardScopes: "openid",
			dbColumnState:          string(CIBAStatePending),
			dbColumnAuthTime:       authTime.String(),
			dbColumnLastPolledAt:   lastPolled.String(),
			dbColumnExpiryTime:     expiry.String(),
		},
	}, nil)

	record, err := suite.store.GetByID(context.Background(), "auth-req-1")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), expiry.Equal(record.ExpiryTime))
	assert.True(suite.T(), lastPolled.Equal(record.LastPolledAt))
	assert.Equal(suite.T(), authTime.Unix(), record.AuthTime.Unix())
	assert.True(suite.T(), record.ExpiryTime.After(time.Now()), "valid request must not read as expired")

	suite.mockDBClient.AssertExpectations(suite.T())
}

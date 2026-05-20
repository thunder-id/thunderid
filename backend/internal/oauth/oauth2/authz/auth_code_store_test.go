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

package authz

import (
	"context"
	"encoding/json"
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

type AuthorizationCodeStoreTestSuite struct {
	suite.Suite
	mockdbProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *authorizationCodeStore
	testAuthzCode  AuthorizationCode
}

func TestAuthorizationCodeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationCodeStoreTestSuite))
}

func (suite *AuthorizationCodeStoreTestSuite) SetupTest() {
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	}
	_ = config.InitializeServerRuntime("test", testConfig)

	suite.mockdbProvider = providermock.NewDBProviderInterfaceMock(suite.T())
	suite.mockDBClient = providermock.NewDBClientInterfaceMock(suite.T())

	suite.store = &authorizationCodeStore{
		dbProvider:   suite.mockdbProvider,
		deploymentID: testDeploymentID,
	}

	suite.testAuthzCode = AuthorizationCode{
		CodeID:              "test-code-id",
		Code:                "test-code",
		ClientID:            "test-client-id",
		RedirectURI:         "https://client.example.com/callback",
		AuthorizedUserID:    "test-user-id",
		TimeCreated:         time.Now(),
		Scopes:              "read write",
		State:               AuthCodeStateActive,
		CodeChallenge:       "",
		CodeChallengeMethod: "",
	}
}

func (suite *AuthorizationCodeStoreTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *AuthorizationCodeStoreTestSuite) TestnewAuthorizationCodeStore() {
	store := newAuthorizationCodeStore()
	assert.NotNil(suite.T(), store)
	assert.Implements(suite.T(), (*AuthorizationCodeStoreInterface)(nil), store)
}

func (suite *AuthorizationCodeStoreTestSuite) TestInsertAuthorizationCode_Success() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)

	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertAuthorizationCode,
		suite.testAuthzCode.CodeID, suite.testAuthzCode.Code, suite.testAuthzCode.ClientID,
		suite.testAuthzCode.State, mock.Anything, suite.testAuthzCode.TimeCreated, mock.Anything,
		testDeploymentID).
		Return(int64(1), nil)

	err := suite.store.InsertAuthorizationCode(context.Background(), suite.testAuthzCode)
	assert.NoError(suite.T(), err)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestInsertAuthorizationCode_DBClientError() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db client error"))

	err := suite.store.InsertAuthorizationCode(context.Background(), suite.testAuthzCode)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "db client error")

	suite.mockdbProvider.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestInsertAuthorizationCode_ExecError() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)

	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertAuthorizationCode,
		suite.testAuthzCode.CodeID, suite.testAuthzCode.Code, suite.testAuthzCode.ClientID,
		suite.testAuthzCode.State, mock.Anything, suite.testAuthzCode.TimeCreated, mock.Anything,
		testDeploymentID).
		Return(int64(0), errors.New("execute error"))

	err := suite.store.InsertAuthorizationCode(context.Background(), suite.testAuthzCode)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error inserting authorization code")

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_Success() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)

	authzData := map[string]interface{}{
		"redirect_uri":          "https://client.example.com/callback",
		"authorized_user_id":    "test-user-id",
		"scopes":                "read write",
		"code_challenge":        "abc123",
		"code_challenge_method": "s256",
		"resource":              "",
		"attribute_cache_id":    "test-cache-id",
	}
	authzDataJSON, _ := json.Marshal(authzData)

	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return([]map[string]interface{}{
			{
				"code_id":            "test-code-id",
				"authorization_code": "test-code",
				"client_id":          "test-client-id",
				"state":              AuthCodeStateActive,
				"authz_data":         string(authzDataJSON),
				"time_created":       "2023-01-01 12:00:00",
				"expiry_time":        "2023-01-01 12:10:00",
			},
		}, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-code-id", result.CodeID)
	assert.Equal(suite.T(), "test-code", result.Code)
	assert.Equal(suite.T(), "test-client-id", result.ClientID)
	assert.Equal(suite.T(), "https://client.example.com/callback", result.RedirectURI)
	assert.Equal(suite.T(), "test-user-id", result.AuthorizedUserID)
	assert.Equal(suite.T(), "abc123", result.CodeChallenge)
	assert.Equal(suite.T(), "s256", result.CodeChallengeMethod)
	assert.Equal(suite.T(), "test-cache-id", result.AttributeCacheID)
	assert.NotZero(suite.T(), result.TimeCreated)
	assert.NotZero(suite.T(), result.ExpiryTime)
	assert.Equal(suite.T(), "read write", result.Scopes)
	assert.Equal(suite.T(), AuthCodeStateActive, result.State)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_DBClientError() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db client error"))

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_QueryError() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(nil, errors.New("query error"))

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error while retrieving authorization code")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_NoResults() {
	queryResults := []map[string]interface{}{}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), errAuthorizationCodeNotFound, err)
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_EmptyCodeID() {
	queryResults := []map[string]interface{}{
		{
			"code_id": "",
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Equal(suite.T(), errAuthorizationCodeNotFound, err)
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_Success() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryConsumeAuthorizationCode,
		AuthCodeStateInactive, "test-code", AuthCodeStateActive, testDeploymentID).
		Return(int64(1), nil)

	consumed, err := suite.store.ConsumeAuthorizationCode(context.Background(), "test-code")
	assert.NoError(suite.T(), err)
	assert.True(suite.T(), consumed)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_AlreadyConsumed() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryConsumeAuthorizationCode,
		AuthCodeStateInactive, "test-code", AuthCodeStateActive, testDeploymentID).
		Return(int64(0), nil)

	consumed, err := suite.store.ConsumeAuthorizationCode(context.Background(), "test-code")
	assert.NoError(suite.T(), err)
	assert.False(suite.T(), consumed)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_DBClientError() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db client error"))

	consumed, err := suite.store.ConsumeAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.False(suite.T(), consumed)

	suite.mockdbProvider.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_ExecuteError() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("ExecuteContext", mock.Anything, queryConsumeAuthorizationCode,
		AuthCodeStateInactive, "test-code", AuthCodeStateActive, testDeploymentID).
		Return(int64(0), errors.New("execute error"))

	consumed, err := suite.store.ConsumeAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error consuming authorization code")
	assert.False(suite.T(), consumed)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

const testTimeString = "2023-12-01 10:30:45.123456789"

func (suite *AuthorizationCodeStoreTestSuite) TestParseTimeField_StringInput() {
	testTime := testTimeString + " extra content"
	expectedTime, _ := time.Parse("2006-01-02 15:04:05.999999999", testTimeString)

	result, err := parseTimeField(testTime, "test_field")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), expectedTime, result)
}

func (suite *AuthorizationCodeStoreTestSuite) TestParseTimeField_TimeInput() {
	testTime := time.Now()

	result, err := parseTimeField(testTime, "test_field")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), testTime, result)
}

func (suite *AuthorizationCodeStoreTestSuite) TestTrimTimeString() {
	input := testTimeString + " extra content here"
	expected := testTimeString

	result := trimTimeString(input)
	assert.Equal(suite.T(), expected, result)
}

func (suite *AuthorizationCodeStoreTestSuite) TestTrimTimeString_ShortInput() {
	input := "2023-12-01"

	result := trimTimeString(input)
	assert.Equal(suite.T(), input, result)
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_InvalidCodeIDType() {
	queryResults := []map[string]interface{}{
		{
			"code_id": 12345, // Invalid type (int instead of string)
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "code ID is of unexpected type")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_InvalidAuthCodeType() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": 12345, // Invalid type
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "authorization code is of unexpected type")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_EmptyAuthCode() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "", // Empty authorization code
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "authorization code is empty")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_InvalidClientIDType() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          12345, // Invalid type
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "client ID is of unexpected type")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_EmptyClientID() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "", // Empty client ID
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "client ID is empty")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_InvalidStateType() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "test-client-id",
			"state":              12345, // Invalid type
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "state is of unexpected type")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_EmptyState() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "test-client-id",
			"state":              "", // Empty state
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "state is empty")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_InvalidTimeCreatedType() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "test-client-id",
			"state":              AuthCodeStateActive,
			"time_created":       12345, // Invalid type
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unexpected type for time_created")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_InvalidExpiryTimeType() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "test-client-id",
			"state":              AuthCodeStateActive,
			"time_created":       "2023-01-01 12:00:00",
			"expiry_time":        12345, // Invalid type
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "unexpected type for expiry_time")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_MissingAuthzData() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "test-client-id",
			"state":              AuthCodeStateActive,
			"time_created":       "2023-01-01 12:00:00",
			"expiry_time":        "2023-01-01 12:10:00",
			// Missing authz_data
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "authz_data is missing or of unexpected type")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_EmptyAuthzDataString() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "test-client-id",
			"state":              AuthCodeStateActive,
			"time_created":       "2023-01-01 12:00:00",
			"expiry_time":        "2023-01-01 12:10:00",
			"authz_data":         "", // Empty string
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "authz_data is missing or of unexpected type")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_EmptyAuthzDataJSON() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "test-client-id",
			"state":              AuthCodeStateActive,
			"time_created":       "2023-01-01 12:00:00",
			"expiry_time":        "2023-01-01 12:10:00",
			"authz_data":         "{}", // Empty JSON object
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "authz_data is empty")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_InvalidAuthzDataJSON() {
	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "test-client-id",
			"state":              AuthCodeStateActive,
			"time_created":       "2023-01-01 12:00:00",
			"expiry_time":        "2023-01-01 12:10:00",
			"authz_data":         "{invalid json", // Invalid JSON
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to unmarshal authz_data JSON")
	assert.Nil(suite.T(), result)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_AuthzDataAsBytes() {
	authzData := map[string]interface{}{
		"redirect_uri":       "https://client.example.com/callback",
		"authorized_user_id": "test-user-id",
		"scopes":             "read write",
	}
	authzDataJSON, _ := json.Marshal(authzData)

	queryResults := []map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "test-client-id",
			"state":              AuthCodeStateActive,
			"time_created":       "2023-01-01 12:00:00",
			"expiry_time":        "2023-01-01 12:10:00",
			"authz_data":         authzDataJSON, // Byte array
		},
	}

	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)
	suite.mockDBClient.On("QueryContext", mock.Anything, queryGetAuthorizationCode,
		"test-code", testDeploymentID).
		Return(queryResults, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-code-id", result.CodeID)
	assert.Equal(suite.T(), "https://client.example.com/callback", result.RedirectURI)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

func (suite *AuthorizationCodeStoreTestSuite) TestParseTimeField_InvalidStringFormat() {
	testTime := "invalid-time-format"

	result, err := parseTimeField(testTime, "test_field")
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error parsing test_field")
	assert.True(suite.T(), result.IsZero())
}

func (suite *AuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_WithNonce() {
	suite.mockdbProvider.On("GetRuntimeDBClient").Return(suite.mockDBClient, nil)

	authzData := map[string]interface{}{
		"redirect_uri":       "https://client.example.com/callback",
		"authorized_user_id": "test-user-id",
		"scopes":             "read write",
		"nonce":              "test-nonce-123",
	}

	authzDataJSON, _ := json.Marshal(authzData)

	suite.mockDBClient.On("QueryContext",
		mock.Anything,
		queryGetAuthorizationCode,
		"test-code",
		testDeploymentID,
	).Return([]map[string]interface{}{
		{
			"code_id":            "test-code-id",
			"authorization_code": "test-code",
			"client_id":          "test-client-id",
			"state":              AuthCodeStateActive,
			"authz_data":         string(authzDataJSON),
			"time_created":       "2023-01-01 12:00:00",
			"expiry_time":        "2023-01-01 12:10:00",
		},
	}, nil)

	result, err := suite.store.GetAuthorizationCode(context.Background(), "test-code")

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "test-nonce-123", result.Nonce)

	suite.mockdbProvider.AssertExpectations(suite.T())
	suite.mockDBClient.AssertExpectations(suite.T())
}

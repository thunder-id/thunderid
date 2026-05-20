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

package par

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/tests/mocks/database/providermock"
)

const (
	testDeploymentID = "test-deployment-id"
	testRandomKey    = "abc"
)

type StoreTestSuite struct {
	suite.Suite
	mockDBProvider *providermock.DBProviderInterfaceMock
	mockDBClient   *providermock.DBClientInterfaceMock
	store          *parRequestStore
	ctx            context.Context
	testRequest    pushedAuthorizationRequest
}

func TestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(StoreTestSuite))
}

func (s *StoreTestSuite) SetupTest() {
	s.mockDBProvider = &providermock.DBProviderInterfaceMock{}
	s.mockDBClient = &providermock.DBClientInterfaceMock{}
	s.store = &parRequestStore{
		dbProvider:   s.mockDBProvider,
		deploymentID: testDeploymentID,
	}
	s.ctx = context.Background()
	s.testRequest = pushedAuthorizationRequest{
		ClientID: "test-client",
		OAuthParameters: model.OAuthParameters{
			ClientID:            "test-client",
			RedirectURI:         "https://example.com/callback",
			ResponseType:        "code",
			State:               "test-state",
			StandardScopes:      []string{"openid", "profile"},
			PermissionScopes:    []string{"read", "write"},
			CodeChallenge:       "challenge123",
			CodeChallengeMethod: "S256",
			Resources:           []string{"https://api.example.com"},
			ClaimsLocales:       "en",
			Nonce:               "nonce123",
		},
	}
}

// Tests for Store

func (s *StoreTestSuite) TestStore_Success() {
	const expirySeconds int64 = 60
	before := time.Now().UTC()
	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertPARRequest,
		mock.MatchedBy(func(key string) bool { return key != "" }),
		testDeploymentID,
		mock.MatchedBy(func(data []byte) bool { return len(data) > 0 }),
		mock.MatchedBy(func(t time.Time) bool {
			expected := before.Add(time.Duration(expirySeconds) * time.Second)
			diff := t.Sub(expected)
			return diff >= -time.Second && diff <= time.Second
		}),
	).Return(int64(1), nil)

	randomKey, err := s.store.Store(s.ctx, s.testRequest, expirySeconds)

	assert.NoError(s.T(), err)
	assert.NotEmpty(s.T(), randomKey)
	s.mockDBProvider.AssertExpectations(s.T())
	s.mockDBClient.AssertExpectations(s.T())
}

func (s *StoreTestSuite) TestStore_DBClientError() {
	s.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db client error"))

	randomKey, err := s.store.Store(s.ctx, s.testRequest, int64(60))

	assert.Error(s.T(), err)
	assert.Empty(s.T(), randomKey)
}

func (s *StoreTestSuite) TestStore_ExecuteError() {
	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertPARRequest,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(int64(0), errors.New("insert failed"))

	randomKey, err := s.store.Store(s.ctx, s.testRequest, int64(60))

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to insert PAR request")
	assert.Empty(s.T(), randomKey)
}

func (s *StoreTestSuite) TestStore_GeneratesUniqueURIs() {
	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryInsertPARRequest,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(int64(1), nil)

	key1, err1 := s.store.Store(s.ctx, s.testRequest, int64(60))
	key2, err2 := s.store.Store(s.ctx, s.testRequest, int64(60))

	assert.NoError(s.T(), err1)
	assert.NoError(s.T(), err2)
	assert.NotEqual(s.T(), key1, key2)
}

// Tests for Consume

func (s *StoreTestSuite) TestConsume_Success() {
	data, _ := json.Marshal(s.testRequest)
	randomKey := "abc123"

	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetPARRequest,
		randomKey, mock.Anything, testDeploymentID,
	).Return([]map[string]any{
		{
			dbColumnRequestURI:    randomKey,
			dbColumnRequestParams: string(data),
		},
	}, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeletePARRequest,
		randomKey, testDeploymentID,
	).Return(int64(1), nil)

	result, found, err := s.store.Consume(s.ctx, randomKey)

	assert.NoError(s.T(), err)
	assert.True(s.T(), found)
	assert.Equal(s.T(), s.testRequest.ClientID, result.ClientID)
	assert.Equal(s.T(), s.testRequest.OAuthParameters.RedirectURI, result.OAuthParameters.RedirectURI)
	assert.Equal(s.T(), s.testRequest.OAuthParameters.StandardScopes, result.OAuthParameters.StandardScopes)
	assert.Equal(s.T(), s.testRequest.OAuthParameters.PermissionScopes, result.OAuthParameters.PermissionScopes)
}

func (s *StoreTestSuite) TestConsume_NotFound() {
	randomKey := "missing"
	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetPARRequest,
		randomKey, mock.Anything, testDeploymentID,
	).Return([]map[string]any{}, nil)

	result, found, err := s.store.Consume(s.ctx, randomKey)

	assert.NoError(s.T(), err)
	assert.False(s.T(), found)
	assert.Equal(s.T(), pushedAuthorizationRequest{}, result)
}

func (s *StoreTestSuite) TestConsume_DBClientError() {
	s.mockDBProvider.On("GetRuntimeDBClient").Return(nil, errors.New("db error"))

	_, found, err := s.store.Consume(s.ctx, testRandomKey)

	assert.Error(s.T(), err)
	assert.False(s.T(), found)
}

func (s *StoreTestSuite) TestConsume_QueryError() {
	randomKey := testRandomKey
	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetPARRequest,
		randomKey, mock.Anything, testDeploymentID,
	).Return(nil, errors.New("query error"))

	_, found, err := s.store.Consume(s.ctx, randomKey)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to query PAR request")
	assert.False(s.T(), found)
}

func (s *StoreTestSuite) TestConsume_DeleteError() {
	randomKey := testRandomKey
	data, _ := json.Marshal(s.testRequest)

	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetPARRequest,
		randomKey, mock.Anything, testDeploymentID,
	).Return([]map[string]any{
		{dbColumnRequestURI: randomKey, dbColumnRequestParams: string(data)},
	}, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeletePARRequest,
		randomKey, testDeploymentID,
	).Return(int64(0), errors.New("delete error"))

	_, found, err := s.store.Consume(s.ctx, randomKey)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to delete PAR request")
	assert.False(s.T(), found)
}

func (s *StoreTestSuite) TestConsume_RaceLost_DeleteReturnsZero() {
	// Another consumer won the race between the SELECT and the DELETE.
	randomKey := testRandomKey
	data, _ := json.Marshal(s.testRequest)

	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetPARRequest,
		randomKey, mock.Anything, testDeploymentID,
	).Return([]map[string]any{
		{dbColumnRequestURI: randomKey, dbColumnRequestParams: string(data)},
	}, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeletePARRequest,
		randomKey, testDeploymentID,
	).Return(int64(0), nil)

	_, found, err := s.store.Consume(s.ctx, randomKey)

	assert.NoError(s.T(), err)
	assert.False(s.T(), found)
}

func (s *StoreTestSuite) TestConsume_RequestParamsAsBytes() {
	randomKey := testRandomKey
	data, _ := json.Marshal(s.testRequest)

	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetPARRequest,
		randomKey, mock.Anything, testDeploymentID,
	).Return([]map[string]any{
		{dbColumnRequestURI: randomKey, dbColumnRequestParams: data},
	}, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeletePARRequest,
		randomKey, testDeploymentID,
	).Return(int64(1), nil)

	result, found, err := s.store.Consume(s.ctx, randomKey)

	assert.NoError(s.T(), err)
	assert.True(s.T(), found)
	assert.Equal(s.T(), s.testRequest.ClientID, result.ClientID)
}

func (s *StoreTestSuite) TestConsume_InvalidJSON() {
	randomKey := testRandomKey

	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetPARRequest,
		randomKey, mock.Anything, testDeploymentID,
	).Return([]map[string]any{
		{dbColumnRequestURI: randomKey, dbColumnRequestParams: "{not valid json"},
	}, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeletePARRequest,
		randomKey, testDeploymentID,
	).Return(int64(1), nil)

	_, found, err := s.store.Consume(s.ctx, randomKey)

	assert.Error(s.T(), err)
	assert.False(s.T(), found)
}

func (s *StoreTestSuite) TestConsume_MissingRequestParams() {
	randomKey := testRandomKey

	s.mockDBProvider.On("GetRuntimeDBClient").Return(s.mockDBClient, nil)
	s.mockDBClient.On("QueryContext", mock.Anything, queryGetPARRequest,
		randomKey, mock.Anything, testDeploymentID,
	).Return([]map[string]any{
		{dbColumnRequestURI: randomKey},
	}, nil)
	s.mockDBClient.On("ExecuteContext", mock.Anything, queryDeletePARRequest,
		randomKey, testDeploymentID,
	).Return(int64(1), nil)

	_, found, err := s.store.Consume(s.ctx, randomKey)

	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "request_params is missing")
	assert.False(s.T(), found)
}

// Tests for helpers

func (s *StoreTestSuite) TestGenerateRandomKey() {
	key, err := generateRandomKey()

	assert.NoError(s.T(), err)
	// 32 bytes base64url encoded = 43 chars.
	assert.True(s.T(), len(key) == 43, "expected random key of length 43, got %d", len(key))
}

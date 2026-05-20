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

package authz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
)

const redisTestReqKey = "test-req-key"

type RedisAuthorizationRequestStoreTestSuite struct {
	suite.Suite
	store      *redisAuthorizationRequestStore
	mockClient *authReqRedisClientMock
	ctx        context.Context
	authReq    authRequestContext
	redisKey   string
}

func TestRedisAuthorizationRequestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RedisAuthorizationRequestStoreTestSuite))
}

func (suite *RedisAuthorizationRequestStoreTestSuite) SetupTest() {
	suite.mockClient = newAuthReqRedisClientMock(suite.T())
	suite.ctx = context.Background()
	suite.store = &redisAuthorizationRequestStore{
		client:         suite.mockClient,
		keyPrefix:      redisTestKeyPrefix,
		deploymentID:   redisTestDeploymentID,
		validityPeriod: 10 * time.Minute,
	}
	suite.authReq = authRequestContext{
		OAuthParameters: model.OAuthParameters{
			ClientID:    "test-client-id",
			RedirectURI: "https://client.example.com/callback",
		},
	}
	suite.redisKey = fmt.Sprintf("%s:runtime:%s:authreq:%s",
		redisTestKeyPrefix, redisTestDeploymentID, redisTestReqKey)
}

// Tests for authReqKey

func (suite *RedisAuthorizationRequestStoreTestSuite) TestAuthReqKey() {
	key := suite.store.authReqKey(redisTestReqKey)
	suite.Equal(suite.redisKey, key)
}

// Tests for AddRequest

func (suite *RedisAuthorizationRequestStoreTestSuite) TestAddRequest_Success() {
	statusCmd := redis.NewStatusCmd(suite.ctx)
	// The key is a dynamically generated UUID — match any non-empty string.
	suite.mockClient.On("Set", suite.ctx,
		mock.MatchedBy(func(k string) bool { return k != "" }),
		mock.Anything, suite.store.validityPeriod).Return(statusCmd)

	key, err := suite.store.AddRequest(suite.ctx, suite.authReq)
	suite.NoError(err)
	suite.NotEmpty(key)
}

func (suite *RedisAuthorizationRequestStoreTestSuite) TestAddRequest_SetError() {
	statusCmd := redis.NewStatusCmd(suite.ctx)
	statusCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Set", suite.ctx,
		mock.MatchedBy(func(k string) bool { return k != "" }),
		mock.Anything, suite.store.validityPeriod).Return(statusCmd)

	key, err := suite.store.AddRequest(suite.ctx, suite.authReq)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to store authorization request in Redis")
	suite.Empty(key)
}

// Tests for GetRequest

func (suite *RedisAuthorizationRequestStoreTestSuite) TestGetRequest_Success() {
	data, _ := json.Marshal(suite.authReq)
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetVal(string(data))
	suite.mockClient.On("Get", suite.ctx, suite.redisKey).Return(stringCmd)

	found, result, err := suite.store.GetRequest(suite.ctx, redisTestReqKey)
	suite.NoError(err)
	suite.True(found)
	suite.Equal(suite.authReq.OAuthParameters.ClientID, result.OAuthParameters.ClientID)
}

func (suite *RedisAuthorizationRequestStoreTestSuite) TestGetRequest_EmptyKey() {
	found, result, err := suite.store.GetRequest(suite.ctx, "")
	suite.NoError(err)
	suite.False(found)
	suite.Equal(authRequestContext{}, result)
}

func (suite *RedisAuthorizationRequestStoreTestSuite) TestGetRequest_NotFound() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetErr(redis.Nil)
	suite.mockClient.On("Get", suite.ctx, suite.redisKey).Return(stringCmd)

	found, result, err := suite.store.GetRequest(suite.ctx, redisTestReqKey)
	suite.NoError(err)
	suite.False(found)
	suite.Equal(authRequestContext{}, result)
}

func (suite *RedisAuthorizationRequestStoreTestSuite) TestGetRequest_GetError() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Get", suite.ctx, suite.redisKey).Return(stringCmd)

	found, result, err := suite.store.GetRequest(suite.ctx, redisTestReqKey)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get authorization request from Redis")
	suite.False(found)
	suite.Equal(authRequestContext{}, result)
}

func (suite *RedisAuthorizationRequestStoreTestSuite) TestGetRequest_UnmarshalError() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetVal("not valid json{{{")
	suite.mockClient.On("Get", suite.ctx, suite.redisKey).Return(stringCmd)

	found, result, err := suite.store.GetRequest(suite.ctx, redisTestReqKey)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to unmarshal authorization request")
	suite.False(found)
	suite.Equal(authRequestContext{}, result)
}

// Tests for ClearRequest

func (suite *RedisAuthorizationRequestStoreTestSuite) TestClearRequest_Success() {
	intCmd := redis.NewIntCmd(suite.ctx)
	intCmd.SetVal(1)
	suite.mockClient.On("Del", suite.ctx, suite.redisKey).Return(intCmd)

	err := suite.store.ClearRequest(suite.ctx, redisTestReqKey)
	suite.NoError(err)
}

func (suite *RedisAuthorizationRequestStoreTestSuite) TestClearRequest_EmptyKey() {
	err := suite.store.ClearRequest(suite.ctx, "")
	suite.NoError(err)
}

func (suite *RedisAuthorizationRequestStoreTestSuite) TestClearRequest_DelError() {
	intCmd := redis.NewIntCmd(suite.ctx)
	intCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Del", suite.ctx, suite.redisKey).Return(intCmd)

	err := suite.store.ClearRequest(suite.ctx, redisTestReqKey)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to delete authorization request from Redis")
}

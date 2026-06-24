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
)

const (
	redisTestKeyPrefix    = "thunderid"
	redisTestDeploymentID = "test-redis-deployment"
	redisTestAuthCode     = "test-auth-code"
)

type RedisAuthorizationCodeStoreTestSuite struct {
	suite.Suite
	store      *redisAuthorizationCodeStore
	mockClient *authCodeRedisClientMock
	ctx        context.Context
	authCode   AuthorizationCode
	redisKey   string
}

func TestRedisAuthorizationCodeStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RedisAuthorizationCodeStoreTestSuite))
}

func (suite *RedisAuthorizationCodeStoreTestSuite) SetupTest() {
	suite.mockClient = newAuthCodeRedisClientMock(suite.T())
	suite.ctx = context.Background()
	suite.store = &redisAuthorizationCodeStore{
		client:       suite.mockClient,
		keyPrefix:    redisTestKeyPrefix,
		deploymentID: redisTestDeploymentID,
	}
	suite.authCode = AuthorizationCode{
		CodeID:           "test-code-id",
		Code:             redisTestAuthCode,
		ClientID:         "test-client-id",
		RedirectURI:      "https://client.example.com/callback",
		AuthorizedUserID: "test-user-id",
		TimeCreated:      time.Now(),
		ExpiryTime:       time.Now().Add(10 * time.Minute),
		Scopes:           "read write",
		State:            AuthCodeStateActive,
	}
	suite.redisKey = fmt.Sprintf("%s:runtime:%s:authcode:%s",
		redisTestKeyPrefix, redisTestDeploymentID, redisTestAuthCode)
}

// Tests for authCodeKey

func (suite *RedisAuthorizationCodeStoreTestSuite) TestAuthCodeKey() {
	key := suite.store.authCodeKey(redisTestAuthCode)
	suite.Equal(suite.redisKey, key)
}

// Tests for InsertAuthorizationCode

func (suite *RedisAuthorizationCodeStoreTestSuite) TestInsertAuthorizationCode_Success() {
	statusCmd := redis.NewStatusCmd(suite.ctx)
	// TTL is time.Until(expiry) which changes slightly; accept any positive duration.
	suite.mockClient.On("Set", suite.ctx, suite.redisKey, mock.Anything,
		mock.MatchedBy(func(d time.Duration) bool { return d > 0 })).Return(statusCmd)

	err := suite.store.InsertAuthorizationCode(suite.ctx, suite.authCode)
	suite.NoError(err)
}

func (suite *RedisAuthorizationCodeStoreTestSuite) TestInsertAuthorizationCode_AlreadyExpired() {
	expiredCode := suite.authCode
	expiredCode.ExpiryTime = time.Now().Add(-1 * time.Minute)

	err := suite.store.InsertAuthorizationCode(suite.ctx, expiredCode)
	suite.Error(err)
	suite.Contains(err.Error(), "authorization code already expired")
}

func (suite *RedisAuthorizationCodeStoreTestSuite) TestInsertAuthorizationCode_SetError() {
	statusCmd := redis.NewStatusCmd(suite.ctx)
	statusCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Set", suite.ctx, suite.redisKey, mock.Anything,
		mock.MatchedBy(func(d time.Duration) bool { return d > 0 })).Return(statusCmd)

	err := suite.store.InsertAuthorizationCode(suite.ctx, suite.authCode)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to store authorization code in Redis")
}

// Tests for GetAuthorizationCode

func (suite *RedisAuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_Success() {
	data, _ := json.Marshal(suite.authCode)
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetVal(string(data))
	suite.mockClient.On("Get", suite.ctx, suite.redisKey).Return(stringCmd)

	result, err := suite.store.GetAuthorizationCode(suite.ctx, redisTestAuthCode)
	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(suite.authCode.CodeID, result.CodeID)
	suite.Equal(suite.authCode.Code, result.Code)
	suite.Equal(suite.authCode.ClientID, result.ClientID)
}

func (suite *RedisAuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_NotFound() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetErr(redis.Nil)
	suite.mockClient.On("Get", suite.ctx, suite.redisKey).Return(stringCmd)

	result, err := suite.store.GetAuthorizationCode(suite.ctx, redisTestAuthCode)
	suite.Error(err)
	suite.Equal(ErrAuthorizationCodeNotFound, err)
	suite.Nil(result)
}

func (suite *RedisAuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_GetError() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Get", suite.ctx, suite.redisKey).Return(stringCmd)

	result, err := suite.store.GetAuthorizationCode(suite.ctx, redisTestAuthCode)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get authorization code from Redis")
	suite.Nil(result)
}

func (suite *RedisAuthorizationCodeStoreTestSuite) TestGetAuthorizationCode_UnmarshalError() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetVal("not valid json{{{")
	suite.mockClient.On("Get", suite.ctx, suite.redisKey).Return(stringCmd)

	result, err := suite.store.GetAuthorizationCode(suite.ctx, redisTestAuthCode)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to unmarshal authorization code")
	suite.Nil(result)
}

// Tests for ConsumeAuthorizationCode
//
// consumeAuthCodeScript.Run() calls EvalSha with the script's precomputed SHA.
// The Lua script returns 1 when consumed, 0 when not found or already consumed.

func (suite *RedisAuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_Success() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetVal(int64(1))
	suite.mockClient.On("EvalSha", suite.ctx, consumeAuthCodeScript.Hash(),
		[]string{suite.redisKey}, AuthCodeStateActive, AuthCodeStateInactive).Return(cmd)

	consumed, err := suite.store.ConsumeAuthorizationCode(suite.ctx, redisTestAuthCode)
	suite.NoError(err)
	suite.True(consumed)
}

func (suite *RedisAuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_AlreadyConsumed() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetVal(int64(0))
	suite.mockClient.On("EvalSha", suite.ctx, consumeAuthCodeScript.Hash(),
		[]string{suite.redisKey}, AuthCodeStateActive, AuthCodeStateInactive).Return(cmd)

	consumed, err := suite.store.ConsumeAuthorizationCode(suite.ctx, redisTestAuthCode)
	suite.NoError(err)
	suite.False(consumed)
}

func (suite *RedisAuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_RedisNil_TreatedAsNotConsumed() {
	// redis.Nil is treated as "not consumed" rather than an error.
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetErr(redis.Nil)
	suite.mockClient.On("EvalSha", suite.ctx, consumeAuthCodeScript.Hash(),
		[]string{suite.redisKey}, AuthCodeStateActive, AuthCodeStateInactive).Return(cmd)

	consumed, err := suite.store.ConsumeAuthorizationCode(suite.ctx, redisTestAuthCode)
	suite.NoError(err)
	suite.False(consumed)
}

func (suite *RedisAuthorizationCodeStoreTestSuite) TestConsumeAuthorizationCode_ScriptError() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetErr(errors.New("WRONGTYPE Operation against a key holding the wrong kind of value"))
	suite.mockClient.On("EvalSha", suite.ctx, consumeAuthCodeScript.Hash(),
		[]string{suite.redisKey}, AuthCodeStateActive, AuthCodeStateInactive).Return(cmd)

	consumed, err := suite.store.ConsumeAuthorizationCode(suite.ctx, redisTestAuthCode)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to consume authorization code")
	suite.False(consumed)
}

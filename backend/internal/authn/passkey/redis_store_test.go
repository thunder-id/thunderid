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

package passkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

const (
	redisTestKeyPrefix    = "thunderid"
	redisTestDeploymentID = "test-deployment"
)

type RedisSessionStoreTestSuite struct {
	suite.Suite
	store      *redisSessionStore
	mockClient *redisClientMock
	sessionKey string
	redisKey   string
}

func TestRedisSessionStoreSuite(t *testing.T) {
	suite.Run(t, new(RedisSessionStoreTestSuite))
}

func (suite *RedisSessionStoreTestSuite) SetupTest() {
	suite.mockClient = newRedisClientMock(suite.T())
	suite.store = &redisSessionStore{
		client:       suite.mockClient,
		keyPrefix:    redisTestKeyPrefix,
		deploymentID: redisTestDeploymentID,
	}
	suite.sessionKey = testSessionKey
	suite.redisKey = fmt.Sprintf("%s:runtime:%s:passkey:%s",
		redisTestKeyPrefix, redisTestDeploymentID, testSessionKey)
}

// Tests for sessionKey

func (suite *RedisSessionStoreTestSuite) TestSessionKey() {
	key := suite.store.sessionKey(testSessionKey)
	suite.Equal(suite.redisKey, key)
}

// Tests for storeSession

func (suite *RedisSessionStoreTestSuite) TestStoreSession_Success() {
	sd := &sessionData{
		Challenge:        "challenge123",
		UserID:           []byte(testUserID),
		RelyingPartyID:   testRelyingPartyID,
		UserVerification: "preferred",
	}

	statusCmd := redis.NewStatusCmd(context.Background())
	suite.mockClient.On("Set", context.Background(), suite.redisKey,
		suite.serializedSessionData(sd), 300*time.Second).Return(statusCmd)

	err := suite.store.storeSession(context.Background(), testSessionKey, sd, 300)
	suite.NoError(err)
}

func (suite *RedisSessionStoreTestSuite) TestStoreSession_SetError() {
	sd := &sessionData{Challenge: "challenge123"}

	statusCmd := redis.NewStatusCmd(context.Background())
	statusCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Set", context.Background(), suite.redisKey,
		suite.serializedSessionData(sd), 300*time.Second).Return(statusCmd)

	err := suite.store.storeSession(context.Background(), testSessionKey, sd, 300)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to store passkey session in Redis")
}

// Tests for retrieveSession

func (suite *RedisSessionStoreTestSuite) TestRetrieveSession_Success() {
	sd := &sessionData{
		Challenge:        "challenge123",
		UserID:           []byte(testUserID),
		RelyingPartyID:   testRelyingPartyID,
		UserVerification: "preferred",
	}

	data, _ := json.Marshal(sd)
	stringCmd := redis.NewStringCmd(context.Background())
	stringCmd.SetVal(string(data))
	suite.mockClient.On("Get", context.Background(), suite.redisKey).Return(stringCmd)

	result, err := suite.store.retrieveSession(context.Background(), testSessionKey)
	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal("challenge123", result.Challenge)
	suite.Equal(testRelyingPartyID, result.RelyingPartyID)
}

func (suite *RedisSessionStoreTestSuite) TestRetrieveSession_EmptyKey() {
	result, err := suite.store.retrieveSession(context.Background(), "")
	suite.NoError(err)
	suite.Nil(result)
}

func (suite *RedisSessionStoreTestSuite) TestRetrieveSession_NotFound() {
	stringCmd := redis.NewStringCmd(context.Background())
	stringCmd.SetErr(redis.Nil)
	suite.mockClient.On("Get", context.Background(), suite.redisKey).Return(stringCmd)

	result, err := suite.store.retrieveSession(context.Background(), testSessionKey)
	suite.NoError(err)
	suite.Nil(result)
}

func (suite *RedisSessionStoreTestSuite) TestRetrieveSession_GetError() {
	stringCmd := redis.NewStringCmd(context.Background())
	stringCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Get", context.Background(), suite.redisKey).Return(stringCmd)

	result, err := suite.store.retrieveSession(context.Background(), testSessionKey)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get passkey session from Redis")
	suite.Nil(result)
}

func (suite *RedisSessionStoreTestSuite) TestRetrieveSession_UnmarshalError() {
	stringCmd := redis.NewStringCmd(context.Background())
	stringCmd.SetVal("not valid json{{{")
	suite.mockClient.On("Get", context.Background(), suite.redisKey).Return(stringCmd)

	result, err := suite.store.retrieveSession(context.Background(), testSessionKey)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to unmarshal passkey session")
	suite.Nil(result)
}

// Tests for deleteSession

func (suite *RedisSessionStoreTestSuite) TestDeleteSession_Success() {
	intCmd := redis.NewIntCmd(context.Background())
	intCmd.SetVal(1)
	suite.mockClient.On("Del", context.Background(), suite.redisKey).Return(intCmd)

	err := suite.store.deleteSession(context.Background(), testSessionKey)
	suite.NoError(err)
}

func (suite *RedisSessionStoreTestSuite) TestDeleteSession_EmptyKey() {
	err := suite.store.deleteSession(context.Background(), "")
	suite.NoError(err)
}

func (suite *RedisSessionStoreTestSuite) TestDeleteSession_DelError() {
	intCmd := redis.NewIntCmd(context.Background())
	intCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Del", context.Background(), suite.redisKey).Return(intCmd)

	err := suite.store.deleteSession(context.Background(), testSessionKey)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to delete passkey session from Redis")
}

// serializedSessionData marshals sessionData to []byte for use in mock matchers.
func (suite *RedisSessionStoreTestSuite) serializedSessionData(sd *sessionData) []byte {
	data, err := json.Marshal(sd)
	suite.Require().NoError(err)
	return data
}

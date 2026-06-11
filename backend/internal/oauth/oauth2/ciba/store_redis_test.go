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
	redisTestKeyPrefix    = "test-prefix"
	redisTestDeploymentID = "test-deployment"
)

type RedisCIBARequestStoreTestSuite struct {
	suite.Suite
	store      *redisCIBARequestStore
	mockClient *cibaRedisClientMock
	ctx        context.Context
}

func TestRedisCIBARequestStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RedisCIBARequestStoreTestSuite))
}

func (suite *RedisCIBARequestStoreTestSuite) SetupTest() {
	suite.mockClient = newCibaRedisClientMock(suite.T())
	suite.ctx = context.Background()
	suite.store = &redisCIBARequestStore{
		client:       suite.mockClient,
		keyPrefix:    redisTestKeyPrefix,
		deploymentID: redisTestDeploymentID,
	}
}

func (suite *RedisCIBARequestStoreTestSuite) sampleRecord() *CIBAAuthRequest {
	return &CIBAAuthRequest{
		AuthReqID:      "auth-req-1",
		ClientID:       "client-1",
		StandardScopes: "openid profile",
		State:          CIBAStatePending,
		ExpiryTime:     time.Now().Add(2 * time.Minute),
	}
}

func (suite *RedisCIBARequestStoreTestSuite) expectedKey(authReqID string) string {
	return fmt.Sprintf("%s:runtime:%s:ciba-auth-req:%s",
		redisTestKeyPrefix, redisTestDeploymentID, authReqID)
}

func (suite *RedisCIBARequestStoreTestSuite) marshalRecord(r *CIBAAuthRequest) []byte {
	data, _ := json.Marshal(r)
	return data
}

func (suite *RedisCIBARequestStoreTestSuite) TestCIBAKey() {
	key := suite.store.cibaKey("auth-req-1")
	suite.Equal(suite.expectedKey("auth-req-1"), key)
}

func (suite *RedisCIBARequestStoreTestSuite) TestAdd_Success() {
	record := suite.sampleRecord()
	statusCmd := redis.NewStatusCmd(suite.ctx)
	suite.mockClient.On("Set", suite.ctx, suite.expectedKey("auth-req-1"),
		mock.AnythingOfType("[]uint8"), mock.AnythingOfType("time.Duration")).
		Return(statusCmd)

	err := suite.store.Add(suite.ctx, record)
	suite.NoError(err)
}

func (suite *RedisCIBARequestStoreTestSuite) TestAdd_AlreadyExpired() {
	record := suite.sampleRecord()
	record.ExpiryTime = time.Now().Add(-1 * time.Minute)

	err := suite.store.Add(suite.ctx, record)
	suite.Error(err)
	suite.Contains(err.Error(), "expired")
}

func (suite *RedisCIBARequestStoreTestSuite) TestAdd_RedisError() {
	record := suite.sampleRecord()
	statusCmd := redis.NewStatusCmd(suite.ctx)
	statusCmd.SetErr(errors.New("redis error"))
	suite.mockClient.On("Set", suite.ctx, suite.expectedKey("auth-req-1"),
		mock.AnythingOfType("[]uint8"), mock.AnythingOfType("time.Duration")).
		Return(statusCmd)

	err := suite.store.Add(suite.ctx, record)
	suite.Error(err)
}

func (suite *RedisCIBARequestStoreTestSuite) TestGetByID_EmptyID() {
	record, err := suite.store.GetByID(suite.ctx, "")
	suite.ErrorIs(err, ErrCIBARequestNotFound)
	suite.Nil(record)
}

func (suite *RedisCIBARequestStoreTestSuite) TestGetByID_NotFound() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetErr(redis.Nil)
	suite.mockClient.On("Get", suite.ctx, suite.expectedKey("missing")).Return(stringCmd)

	record, err := suite.store.GetByID(suite.ctx, "missing")
	suite.ErrorIs(err, ErrCIBARequestNotFound)
	suite.Nil(record)
}

func (suite *RedisCIBARequestStoreTestSuite) TestGetByID_Success() {
	expected := suite.sampleRecord()
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetVal(string(suite.marshalRecord(expected)))
	suite.mockClient.On("Get", suite.ctx, suite.expectedKey("auth-req-1")).Return(stringCmd)

	record, err := suite.store.GetByID(suite.ctx, "auth-req-1")
	suite.NoError(err)
	suite.Equal("auth-req-1", record.AuthReqID)
	suite.Equal("client-1", record.ClientID)
	suite.Equal(CIBAStatePending, record.State)
}

func (suite *RedisCIBARequestStoreTestSuite) TestMarkAuthenticated_Success() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetVal(int64(1))
	suite.mockClient.On("EvalSha", suite.ctx, markAuthenticatedScript.Hash(),
		[]string{suite.expectedKey("auth-req-1")},
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(cmd)

	err := suite.store.MarkAuthenticated(
		suite.ctx, "auth-req-1", "user-1", "openid customer:update", "cache-1", "urn:acr:pwd", time.Now())
	suite.NoError(err)
}

func (suite *RedisCIBARequestStoreTestSuite) TestMarkAuthenticated_NotPending() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetVal(int64(0))
	suite.mockClient.On("EvalSha", suite.ctx, markAuthenticatedScript.Hash(),
		[]string{suite.expectedKey("auth-req-1")},
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(cmd)

	err := suite.store.MarkAuthenticated(suite.ctx, "auth-req-1", "user-1", "", "cache-1", "", time.Now())
	suite.Error(err)
}

func (suite *RedisCIBARequestStoreTestSuite) TestMarkAuthenticated_ScriptError() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetErr(errors.New("redis script error"))
	suite.mockClient.On("EvalSha", suite.ctx, markAuthenticatedScript.Hash(),
		[]string{suite.expectedKey("auth-req-1")},
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(cmd)

	err := suite.store.MarkAuthenticated(suite.ctx, "auth-req-1", "user-1", "", "cache-1", "", time.Now())
	suite.Error(err)
	suite.Contains(err.Error(), "failed to mark CIBA request as authenticated")
}

func (suite *RedisCIBARequestStoreTestSuite) TestMarkAuthenticated_NotFound() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetErr(redis.Nil)
	suite.mockClient.On("EvalSha", suite.ctx, markAuthenticatedScript.Hash(),
		[]string{suite.expectedKey("auth-req-1")},
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(cmd)

	err := suite.store.MarkAuthenticated(suite.ctx, "auth-req-1", "user-1", "", "cache-1", "", time.Now())
	suite.Error(err)
	suite.Contains(err.Error(), "not found")
}

func (suite *RedisCIBARequestStoreTestSuite) TestMarkConsumed_Success() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetVal(int64(1))
	suite.mockClient.On("EvalSha", suite.ctx, consumeCIBAScript.Hash(),
		[]string{suite.expectedKey("auth-req-1")},
		string(CIBAStateAuthenticated), string(CIBAStateConsumed)).Return(cmd)

	consumed, err := suite.store.MarkConsumed(suite.ctx, "auth-req-1")
	suite.NoError(err)
	suite.True(consumed)
}

func (suite *RedisCIBARequestStoreTestSuite) TestMarkConsumed_NotAuthenticated() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetVal(int64(0))
	suite.mockClient.On("EvalSha", suite.ctx, consumeCIBAScript.Hash(),
		[]string{suite.expectedKey("auth-req-1")},
		string(CIBAStateAuthenticated), string(CIBAStateConsumed)).Return(cmd)

	consumed, err := suite.store.MarkConsumed(suite.ctx, "auth-req-1")
	suite.NoError(err)
	suite.False(consumed)
}

func (suite *RedisCIBARequestStoreTestSuite) TestMarkConsumed_ScriptError() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetErr(errors.New("redis script error"))
	suite.mockClient.On("EvalSha", suite.ctx, consumeCIBAScript.Hash(),
		[]string{suite.expectedKey("auth-req-1")},
		string(CIBAStateAuthenticated), string(CIBAStateConsumed)).Return(cmd)

	consumed, err := suite.store.MarkConsumed(suite.ctx, "auth-req-1")
	suite.Error(err)
	suite.Contains(err.Error(), "failed to consume CIBA request")
	suite.False(consumed)
}

func (suite *RedisCIBARequestStoreTestSuite) TestMarkConsumed_RedisNil() {
	cmd := redis.NewCmd(suite.ctx)
	cmd.SetErr(redis.Nil)
	suite.mockClient.On("EvalSha", suite.ctx, consumeCIBAScript.Hash(),
		[]string{suite.expectedKey("auth-req-1")},
		string(CIBAStateAuthenticated), string(CIBAStateConsumed)).Return(cmd)

	consumed, err := suite.store.MarkConsumed(suite.ctx, "auth-req-1")
	suite.NoError(err)
	suite.False(consumed)
}

func (suite *RedisCIBARequestStoreTestSuite) TestUpdateLastPolled_Success() {
	record := suite.sampleRecord()
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetVal(string(suite.marshalRecord(record)))
	suite.mockClient.On("Get", suite.ctx, suite.expectedKey("auth-req-1")).Return(stringCmd)

	statusCmd := redis.NewStatusCmd(suite.ctx)
	suite.mockClient.On("Set", suite.ctx, suite.expectedKey("auth-req-1"),
		mock.AnythingOfType("[]uint8"), mock.AnythingOfType("time.Duration")).Return(statusCmd)

	err := suite.store.UpdateLastPolled(suite.ctx, "auth-req-1", time.Now())
	suite.NoError(err)
}

func (suite *RedisCIBARequestStoreTestSuite) TestUpdateState_Success() {
	record := suite.sampleRecord()
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetVal(string(suite.marshalRecord(record)))
	suite.mockClient.On("Get", suite.ctx, suite.expectedKey("auth-req-1")).Return(stringCmd)

	statusCmd := redis.NewStatusCmd(suite.ctx)
	suite.mockClient.On("Set", suite.ctx, suite.expectedKey("auth-req-1"),
		mock.AnythingOfType("[]uint8"), mock.AnythingOfType("time.Duration")).Return(statusCmd)

	err := suite.store.UpdateState(suite.ctx, "auth-req-1", CIBAStateExpired)
	suite.NoError(err)
}

func (suite *RedisCIBARequestStoreTestSuite) TestUpdateLastPolled_RedisGetError() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetErr(errors.New("redis get error"))
	suite.mockClient.On("Get", suite.ctx, suite.expectedKey("auth-req-1")).Return(stringCmd)

	err := suite.store.UpdateLastPolled(suite.ctx, "auth-req-1", time.Now())
	suite.Error(err)
}

func (suite *RedisCIBARequestStoreTestSuite) TestUpdateState_RedisGetError() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetErr(errors.New("redis get error"))
	suite.mockClient.On("Get", suite.ctx, suite.expectedKey("auth-req-1")).Return(stringCmd)

	err := suite.store.UpdateState(suite.ctx, "auth-req-1", CIBAStateExpired)
	suite.Error(err)
}

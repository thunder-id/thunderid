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

package flowexec

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

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const (
	redisTestKeyPrefix    = "thunderid"
	redisTestDeploymentID = "test-redis-deployment"
	redisTestFlowID       = "test-flow-id"
)

type RedisFlowStoreTestSuite struct {
	suite.Suite
	store      *redisFlowStore
	mockClient *redisClientMock
	ctx        context.Context
	flowKey    string
}

func TestRedisFlowStoreSuite(t *testing.T) {
	suite.Run(t, new(RedisFlowStoreTestSuite))
}

func (suite *RedisFlowStoreTestSuite) SetupTest() {
	suite.mockClient = newRedisClientMock(suite.T())
	suite.ctx = context.Background()
	suite.store = &redisFlowStore{
		client:       suite.mockClient,
		keyPrefix:    redisTestKeyPrefix,
		deploymentID: redisTestDeploymentID,
	}
	suite.flowKey = fmt.Sprintf("%s:runtime:%s:flow:%s",
		redisTestKeyPrefix, redisTestDeploymentID, redisTestFlowID)
}

// buildEngineContext creates a minimal EngineContext for use in tests.
// GetID is registered as Maybe because not all test paths reach FromEngineContext.
func (suite *RedisFlowStoreTestSuite) buildEngineContext() EngineContext {
	mockGraph := coremock.NewGraphInterfaceMock(suite.T())
	mockGraph.On("GetID").Return("test-graph-id").Maybe()
	return EngineContext{
		ExecutionID: redisTestFlowID,
		AppID:       "test-app-id",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: false,
			Attributes:      map[string]interface{}{},
		},
		UserInputs:       map[string]string{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Graph:            mockGraph,
	}
}

// serializedFlowContext converts an EngineContext to the JSON bytes the store would write.
func (suite *RedisFlowStoreTestSuite) serializedFlowContext(engineCtx EngineContext) []byte {
	dbModel, err := FromEngineContext(engineCtx)
	suite.Require().NoError(err)
	data, err := json.Marshal(dbModel)
	suite.Require().NoError(err)
	return data
}

// Tests for flowKey

func (suite *RedisFlowStoreTestSuite) TestFlowKey() {
	key := suite.store.flowKey(redisTestFlowID)
	suite.Equal(suite.flowKey, key)
}

// Tests for StoreFlowContext

func (suite *RedisFlowStoreTestSuite) TestStoreFlowContext_Success() {
	engineCtx := suite.buildEngineContext()
	expirySeconds := int64(1800)

	dbModel, err := FromEngineContext(engineCtx)
	suite.Require().NoError(err)

	statusCmd := redis.NewStatusCmd(suite.ctx)
	suite.mockClient.On("Set", suite.ctx, suite.flowKey, mock.Anything,
		time.Duration(expirySeconds)*time.Second).Return(statusCmd)

	err = suite.store.StoreFlowContext(suite.ctx, *dbModel, expirySeconds)
	suite.NoError(err)
}

func (suite *RedisFlowStoreTestSuite) TestStoreFlowContext_SetError() {
	engineCtx := suite.buildEngineContext()
	expirySeconds := int64(1800)

	dbModel, err := FromEngineContext(engineCtx)
	suite.Require().NoError(err)

	statusCmd := redis.NewStatusCmd(suite.ctx)
	statusCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Set", suite.ctx, suite.flowKey, mock.Anything,
		time.Duration(expirySeconds)*time.Second).Return(statusCmd)

	err = suite.store.StoreFlowContext(suite.ctx, *dbModel, expirySeconds)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to store flow context in Redis")
}

// Tests for GetFlowContext

func (suite *RedisFlowStoreTestSuite) TestGetFlowContext_Success() {
	engineCtx := suite.buildEngineContext()
	data := suite.serializedFlowContext(engineCtx)

	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetVal(string(data))
	suite.mockClient.On("Get", suite.ctx, suite.flowKey).Return(stringCmd)

	result, err := suite.store.GetFlowContext(suite.ctx, redisTestFlowID)
	suite.NoError(err)
	suite.NotNil(result)
	suite.Equal(redisTestFlowID, result.ExecutionID)
}

func (suite *RedisFlowStoreTestSuite) TestGetFlowContext_NotFound() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetErr(redis.Nil)
	suite.mockClient.On("Get", suite.ctx, suite.flowKey).Return(stringCmd)

	result, err := suite.store.GetFlowContext(suite.ctx, redisTestFlowID)
	suite.NoError(err)
	suite.Nil(result)
}

func (suite *RedisFlowStoreTestSuite) TestGetFlowContext_GetError() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Get", suite.ctx, suite.flowKey).Return(stringCmd)

	result, err := suite.store.GetFlowContext(suite.ctx, redisTestFlowID)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to get flow context from Redis")
	suite.Nil(result)
}

func (suite *RedisFlowStoreTestSuite) TestGetFlowContext_UnmarshalError() {
	stringCmd := redis.NewStringCmd(suite.ctx)
	stringCmd.SetVal("not valid json{{{")
	suite.mockClient.On("Get", suite.ctx, suite.flowKey).Return(stringCmd)

	result, err := suite.store.GetFlowContext(suite.ctx, redisTestFlowID)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to unmarshal flow context")
	suite.Nil(result)
}

// Tests for UpdateFlowContext

func (suite *RedisFlowStoreTestSuite) TestUpdateFlowContext_Success() {
	engineCtx := suite.buildEngineContext()
	dbModel, err := FromEngineContext(engineCtx)
	suite.Require().NoError(err)

	cmd := redis.NewCmd(suite.ctx)
	cmd.SetVal(int64(1))
	suite.mockClient.On("EvalSha", suite.ctx, updateFlowScript.Hash(),
		[]string{suite.flowKey}, mock.Anything).Return(cmd)

	err = suite.store.UpdateFlowContext(suite.ctx, *dbModel)
	suite.NoError(err)
}

func (suite *RedisFlowStoreTestSuite) TestUpdateFlowContext_KeyNotFound() {
	engineCtx := suite.buildEngineContext()
	dbModel, err := FromEngineContext(engineCtx)
	suite.Require().NoError(err)

	cmd := redis.NewCmd(suite.ctx)
	cmd.SetVal(int64(0))
	suite.mockClient.On("EvalSha", suite.ctx, updateFlowScript.Hash(),
		[]string{suite.flowKey}, mock.Anything).Return(cmd)

	err = suite.store.UpdateFlowContext(suite.ctx, *dbModel)
	suite.Error(err)
	suite.Contains(err.Error(), "flow context not found for executionID")
}

func (suite *RedisFlowStoreTestSuite) TestUpdateFlowContext_ScriptError() {
	engineCtx := suite.buildEngineContext()
	dbModel, err := FromEngineContext(engineCtx)
	suite.Require().NoError(err)

	cmd := redis.NewCmd(suite.ctx)
	cmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("EvalSha", suite.ctx, updateFlowScript.Hash(),
		[]string{suite.flowKey}, mock.Anything).Return(cmd)

	err = suite.store.UpdateFlowContext(suite.ctx, *dbModel)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to update flow context in Redis")
}

// Tests for DeleteFlowContext

func (suite *RedisFlowStoreTestSuite) TestDeleteFlowContext_Success() {
	intCmd := redis.NewIntCmd(suite.ctx)
	intCmd.SetVal(1)
	suite.mockClient.On("Del", suite.ctx, suite.flowKey).Return(intCmd)

	err := suite.store.DeleteFlowContext(suite.ctx, redisTestFlowID)
	suite.NoError(err)
}

func (suite *RedisFlowStoreTestSuite) TestDeleteFlowContext_DelError() {
	intCmd := redis.NewIntCmd(suite.ctx)
	intCmd.SetErr(errors.New("connection refused"))
	suite.mockClient.On("Del", suite.ctx, suite.flowKey).Return(intCmd)

	err := suite.store.DeleteFlowContext(suite.ctx, redisTestFlowID)
	suite.Error(err)
	suite.Contains(err.Error(), "failed to delete flow context from Redis")
}

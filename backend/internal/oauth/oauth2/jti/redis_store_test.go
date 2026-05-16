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

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	redisTestKeyPrefix    = "thunder"
	redisTestDeploymentID = "deployment-1"
)

type RedisStoreTestSuite struct {
	suite.Suite
	client *redisClientMock
	store  *redisStore
}

func TestRedisStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RedisStoreTestSuite))
}

func (suite *RedisStoreTestSuite) SetupTest() {
	suite.client = newRedisClientMock(suite.T())
	suite.store = &redisStore{
		client:       suite.client,
		keyPrefix:    redisTestKeyPrefix,
		deploymentID: redisTestDeploymentID,
	}
}

func (suite *RedisStoreTestSuite) TestJtiKey() {
	got := suite.store.jtiKey("dpop", "xyz")
	assert.Equal(suite.T(), "thunder:runtime:deployment-1:jti:dpop:xyz", got)
}

func (suite *RedisStoreTestSuite) TestRecordJTI_Inserted() {
	expiry := time.Now().Add(30 * time.Second)
	expectedKey := "thunder:runtime:deployment-1:jti:dpop:jti-1"

	cmd := redis.NewBoolCmd(context.Background())
	cmd.SetVal(true)
	suite.client.On("SetNX",
		mock.Anything,
		expectedKey,
		mock.Anything,
		mock.MatchedBy(func(d time.Duration) bool { return d > 0 && d <= 31*time.Second }),
	).Return(cmd)

	inserted, err := suite.store.RecordJTI(context.Background(), "dpop", "jti-1", expiry)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), inserted)
}

func (suite *RedisStoreTestSuite) TestRecordJTI_Replay() {
	cmd := redis.NewBoolCmd(context.Background())
	cmd.SetVal(false)
	suite.client.On("SetNX", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cmd)

	inserted, err := suite.store.RecordJTI(context.Background(), "dpop", "jti-1", time.Now().Add(time.Minute))
	require.NoError(suite.T(), err)
	assert.False(suite.T(), inserted)
}

func (suite *RedisStoreTestSuite) TestRecordJTI_BackendError() {
	cmd := redis.NewBoolCmd(context.Background())
	cmd.SetErr(errors.New("connection refused"))
	suite.client.On("SetNX", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(cmd)

	inserted, err := suite.store.RecordJTI(context.Background(), "dpop", "jti-1", time.Now().Add(time.Minute))
	require.Error(suite.T(), err)
	assert.False(suite.T(), inserted)
	assert.Contains(suite.T(), err.Error(), "failed to record jti in Redis")
}

func (suite *RedisStoreTestSuite) TestRecordJTI_ExpiryInPastIsNotInserted() {
	// Callers already reject expired proofs before reaching the store, so SetNX must
	// never be invoked when TTL would be non-positive.
	inserted, err := suite.store.RecordJTI(context.Background(), "dpop", "jti-1", time.Now().Add(-time.Minute))
	require.NoError(suite.T(), err)
	assert.True(suite.T(), inserted)
}

// TestRecordJTI_NamespaceIsolation guards against accidental key-format
// changes that would collapse two namespaces with the same jti into the same Redis key.
func (suite *RedisStoreTestSuite) TestRecordJTI_NamespaceIsolation() {
	expiry := time.Now().Add(30 * time.Second)

	cmd1 := redis.NewBoolCmd(context.Background())
	cmd1.SetVal(true)
	cmd2 := redis.NewBoolCmd(context.Background())
	cmd2.SetVal(true)

	suite.client.On("SetNX", mock.Anything,
		"thunder:runtime:deployment-1:jti:dpop:j",
		mock.Anything, mock.Anything).Return(cmd1).Once()
	suite.client.On("SetNX", mock.Anything,
		"thunder:runtime:deployment-1:jti:client_assertion:j",
		mock.Anything, mock.Anything).Return(cmd2).Once()

	ok1, err := suite.store.RecordJTI(context.Background(), "dpop", "j", expiry)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), ok1)
	ok2, err := suite.store.RecordJTI(context.Background(), "client_assertion", "j", expiry)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), ok2)
}

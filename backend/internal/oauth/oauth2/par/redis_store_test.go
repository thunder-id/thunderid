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
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
)

const (
	redisTestKeyPrefix    = "thunderid"
	redisTestDeploymentID = "test-deployment-id"
)

type RedisStoreTestSuite struct {
	suite.Suite
	mockClient *parRedisClientMock
	store      *redisPARRequestStore
	ctx        context.Context
	testReq    pushedAuthorizationRequest
}

func TestRedisStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RedisStoreTestSuite))
}

func (s *RedisStoreTestSuite) SetupTest() {
	s.mockClient = newParRedisClientMock(s.T())
	s.store = &redisPARRequestStore{
		client:       s.mockClient,
		keyPrefix:    redisTestKeyPrefix,
		deploymentID: redisTestDeploymentID,
	}
	s.ctx = context.Background()
	s.testReq = pushedAuthorizationRequest{
		ClientID: "test-client",
		OAuthParameters: model.OAuthParameters{
			ClientID:    "test-client",
			RedirectURI: "https://example.com/callback",
			State:       "test-state",
		},
	}
}

func (s *RedisStoreTestSuite) buildRedisKey(randomKey string) string {
	return fmt.Sprintf("%s:runtime:%s:par:%s",
		redisTestKeyPrefix, redisTestDeploymentID, randomKey)
}

// Tests for parKey

func (s *RedisStoreTestSuite) TestParKey() {
	randomKey := testRandomKey
	expected := s.buildRedisKey(randomKey)
	s.Equal(expected, s.store.parKey(randomKey))
}

// Tests for Store

func (s *RedisStoreTestSuite) TestStore_Success() {
	const expirySeconds int64 = 60
	statusCmd := redis.NewStatusCmd(s.ctx)
	s.mockClient.On("Set", s.ctx,
		mock.MatchedBy(func(k string) bool {
			return strings.HasPrefix(k, redisTestKeyPrefix+":runtime:"+redisTestDeploymentID+":par:")
		}),
		mock.Anything,
		time.Duration(expirySeconds)*time.Second,
	).Return(statusCmd)

	randomKey, err := s.store.Store(s.ctx, s.testReq, expirySeconds)

	s.NoError(err)
	s.NotEmpty(randomKey)
}

func (s *RedisStoreTestSuite) TestStore_SetError() {
	statusCmd := redis.NewStatusCmd(s.ctx)
	statusCmd.SetErr(errors.New("connection refused"))
	s.mockClient.On("Set", s.ctx,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(statusCmd)

	randomKey, err := s.store.Store(s.ctx, s.testReq, int64(60))

	s.Error(err)
	s.Contains(err.Error(), "failed to store PAR request in Redis")
	s.Empty(randomKey)
}

// Tests for Consume

func (s *RedisStoreTestSuite) TestConsume_Success() {
	randomKey := testRandomKey
	data, _ := json.Marshal(s.testReq)
	stringCmd := redis.NewStringCmd(s.ctx)
	stringCmd.SetVal(string(data))

	s.mockClient.On("GetDel", s.ctx, s.buildRedisKey(randomKey)).Return(stringCmd)

	result, found, err := s.store.Consume(s.ctx, randomKey)

	s.NoError(err)
	s.True(found)
	s.Equal(s.testReq.ClientID, result.ClientID)
	s.Equal(s.testReq.OAuthParameters.RedirectURI, result.OAuthParameters.RedirectURI)
}

func (s *RedisStoreTestSuite) TestConsume_NotFound() {
	randomKey := "missing"
	stringCmd := redis.NewStringCmd(s.ctx)
	stringCmd.SetErr(redis.Nil)

	s.mockClient.On("GetDel", s.ctx, s.buildRedisKey(randomKey)).Return(stringCmd)

	_, found, err := s.store.Consume(s.ctx, randomKey)

	s.NoError(err)
	s.False(found)
}

func (s *RedisStoreTestSuite) TestConsume_GetDelError() {
	randomKey := testRandomKey
	stringCmd := redis.NewStringCmd(s.ctx)
	stringCmd.SetErr(errors.New("connection refused"))

	s.mockClient.On("GetDel", s.ctx, s.buildRedisKey(randomKey)).Return(stringCmd)

	_, found, err := s.store.Consume(s.ctx, randomKey)

	s.Error(err)
	s.Contains(err.Error(), "failed to get PAR request from Redis")
	s.False(found)
}

func (s *RedisStoreTestSuite) TestConsume_UnmarshalError() {
	randomKey := testRandomKey
	stringCmd := redis.NewStringCmd(s.ctx)
	stringCmd.SetVal("not valid json{{{")

	s.mockClient.On("GetDel", s.ctx, s.buildRedisKey(randomKey)).Return(stringCmd)

	_, found, err := s.store.Consume(s.ctx, randomKey)

	s.Error(err)
	s.Contains(err.Error(), "failed to unmarshal PAR request")
	s.False(found)
}

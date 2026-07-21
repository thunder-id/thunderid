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

package redisstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type RedisStoreTestSuite struct {
	suite.Suite
	client *redisClientMock
	store  *redisStore
	ctx    context.Context
}

func TestRedisStoreTestSuite(t *testing.T) {
	suite.Run(t, new(RedisStoreTestSuite))
}

func (s *RedisStoreTestSuite) SetupTest() {
	s.client = newRedisClientMock(s.T())
	s.store = &redisStore{
		keyPrefix:    "thunderid",
		deploymentID: "dep1",
		client:       s.client,
		logger:       log.GetLogger(),
	}
	s.ctx = context.Background()
}

func (s *RedisStoreTestSuite) TestGetFormattedKey() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "abc123")
	s.Equal("thunderid:runtime:dep1:flow:state:abc123", key)
}

func (s *RedisStoreTestSuite) TestGetFormattedKey_DifferentNamespaces() {
	cases := []struct {
		namespace providers.RuntimeStoreNamespace
		key       string
		want      string
	}{
		{providers.NamespaceAuthzCode, "code1", "thunderid:runtime:dep1:authz:code:code1"},
		{providers.NamespacePAR, "req42", "thunderid:runtime:dep1:par:req:req42"},
		{providers.NamespaceJTI, "tok99", "thunderid:runtime:dep1:jti:token:tok99"},
	}
	for _, tc := range cases {
		s.Equal(tc.want, s.store.getFormattedKey(tc.namespace, tc.key), "namespace=%s", tc.namespace)
	}
}

func (s *RedisStoreTestSuite) TestGetFormattedKey_EmptyKeyPrefix() {
	store := &redisStore{keyPrefix: "", deploymentID: "dep1"}
	key := store.getFormattedKey(providers.NamespaceFlow, "k")
	s.Equal(":runtime:dep1:flow:state:k", key)
}

func (s *RedisStoreTestSuite) TestGetFormattedKey_EmptyDeploymentID() {
	store := &redisStore{keyPrefix: "pfx", deploymentID: ""}
	key := store.getFormattedKey(providers.NamespaceFlow, "k")
	s.Equal("pfx:runtime::flow:state:k", key)
}

func (s *RedisStoreTestSuite) TestPut_WithTTL_Success() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "k")
	s.client.On("Set", mock.Anything, key, []byte("v"),
		mock.MatchedBy(func(d time.Duration) bool { return d == 60*time.Second })).
		Return(redis.NewStatusResult("OK", nil))

	err := s.store.Put(s.ctx, providers.NamespaceFlow, "k", []byte("v"), 60)
	s.NoError(err)
}

func (s *RedisStoreTestSuite) TestPut_ZeroTTL_NoExpiry() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "k")
	s.client.On("Set", mock.Anything, key, []byte("v"), time.Duration(0)).
		Return(redis.NewStatusResult("OK", nil))

	err := s.store.Put(s.ctx, providers.NamespaceFlow, "k", []byte("v"), 0)
	s.NoError(err)
}

func (s *RedisStoreTestSuite) TestPut_BackendError() {
	s.client.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(redis.NewStatusResult("", errors.New("connection refused")))

	err := s.store.Put(s.ctx, providers.NamespaceFlow, "k", []byte("v"), 60)
	s.Error(err)
	s.Contains(err.Error(), "failed to store in Redis")
}

func (s *RedisStoreTestSuite) TestGet_Success() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "k")
	s.client.On("Get", mock.Anything, key).Return(redis.NewStringResult("v", nil))

	got, err := s.store.Get(s.ctx, providers.NamespaceFlow, "k")
	s.NoError(err)
	s.Equal([]byte("v"), got)
}

func (s *RedisStoreTestSuite) TestGet_MissingKey_ReturnsNil() {
	s.client.On("Get", mock.Anything, mock.Anything).Return(redis.NewStringResult("", redis.Nil))

	got, err := s.store.Get(s.ctx, providers.NamespaceFlow, "missing")
	s.NoError(err)
	s.Nil(got)
}

func (s *RedisStoreTestSuite) TestGet_BackendError() {
	s.client.On("Get", mock.Anything, mock.Anything).
		Return(redis.NewStringResult("", errors.New("connection refused")))

	got, err := s.store.Get(s.ctx, providers.NamespaceFlow, "k")
	s.Error(err)
	s.Nil(got)
	s.Contains(err.Error(), "failed to get data from Redis")
}

func (s *RedisStoreTestSuite) TestUpdate_Success() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "k")
	s.client.On("SetArgs", mock.Anything, key, []byte("v"),
		redis.SetArgs{Mode: "XX", KeepTTL: true}).
		Return(redis.NewStatusResult("OK", nil))

	err := s.store.Update(s.ctx, providers.NamespaceFlow, "k", []byte("v"))
	s.NoError(err)
}

func (s *RedisStoreTestSuite) TestUpdate_MissingKey_ReturnsError() {
	s.client.On("SetArgs", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(redis.NewStatusResult("", redis.Nil))

	err := s.store.Update(s.ctx, providers.NamespaceFlow, "k", []byte("v"))
	s.ErrorIs(err, providers.ErrRuntimeStoreKeyNotFound)
}

func (s *RedisStoreTestSuite) TestUpdate_BackendError() {
	s.client.On("SetArgs", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(redis.NewStatusResult("", errors.New("connection refused")))

	err := s.store.Update(s.ctx, providers.NamespaceFlow, "k", []byte("v"))
	s.Error(err)
	s.Contains(err.Error(), "failed to update in Redis")
}

func (s *RedisStoreTestSuite) TestDelete_Success() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "k")
	s.client.On("Del", mock.Anything, key).Return(redis.NewIntResult(1, nil))

	err := s.store.Delete(s.ctx, providers.NamespaceFlow, "k")
	s.NoError(err)
}

func (s *RedisStoreTestSuite) TestDelete_BackendError() {
	s.client.On("Del", mock.Anything, mock.Anything).
		Return(redis.NewIntResult(0, errors.New("connection refused")))

	err := s.store.Delete(s.ctx, providers.NamespaceFlow, "k")
	s.Error(err)
	s.Contains(err.Error(), "failed to delete from Redis")
}

func (s *RedisStoreTestSuite) TestTake_Success() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "k")
	s.client.On("GetDel", mock.Anything, key).Return(redis.NewStringResult("v", nil))

	got, err := s.store.Take(s.ctx, providers.NamespaceFlow, "k")
	s.NoError(err)
	s.Equal([]byte("v"), got)
}

func (s *RedisStoreTestSuite) TestTake_MissingKey_ReturnsNil() {
	s.client.On("GetDel", mock.Anything, mock.Anything).Return(redis.NewStringResult("", redis.Nil))

	got, err := s.store.Take(s.ctx, providers.NamespaceFlow, "missing")
	s.NoError(err)
	s.Nil(got)
}

func (s *RedisStoreTestSuite) TestTake_BackendError() {
	s.client.On("GetDel", mock.Anything, mock.Anything).
		Return(redis.NewStringResult("", errors.New("connection refused")))

	got, err := s.store.Take(s.ctx, providers.NamespaceFlow, "k")
	s.Error(err)
	s.Nil(got)
	s.Contains(err.Error(), "failed to take data from Redis")
}

func (s *RedisStoreTestSuite) TestExtendTTL_Success() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "k")
	s.client.On("Expire", mock.Anything, key, 60*time.Second).
		Return(redis.NewBoolResult(true, nil))

	err := s.store.ExtendTTL(s.ctx, providers.NamespaceFlow, "k", 60)
	s.NoError(err)
}

func (s *RedisStoreTestSuite) TestExtendTTL_ZeroTTL_CallsExpireWithZeroDuration() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "k")
	s.client.On("Expire", mock.Anything, key, time.Duration(0)).
		Return(redis.NewBoolResult(true, nil))

	err := s.store.ExtendTTL(s.ctx, providers.NamespaceFlow, "k", 0)
	s.NoError(err)
}

func (s *RedisStoreTestSuite) TestExtendTTL_NegativeTTL_CallsExpireWithNegativeDuration() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "k")
	s.client.On("Expire", mock.Anything, key, -1*time.Second).
		Return(redis.NewBoolResult(true, nil))

	err := s.store.ExtendTTL(s.ctx, providers.NamespaceFlow, "k", -1)
	s.NoError(err)
}

func (s *RedisStoreTestSuite) TestExtendTTL_MissingKey_ReturnsError() {
	s.client.On("Expire", mock.Anything, mock.Anything, mock.Anything).
		Return(redis.NewBoolResult(false, nil))

	err := s.store.ExtendTTL(s.ctx, providers.NamespaceFlow, "k", 60)
	s.ErrorIs(err, providers.ErrRuntimeStoreKeyNotFound)
}

func (s *RedisStoreTestSuite) TestExtendTTL_ZeroTTL_MissingKey_ReturnsError() {
	s.client.On("Expire", mock.Anything, mock.Anything, time.Duration(0)).
		Return(redis.NewBoolResult(false, nil))

	err := s.store.ExtendTTL(s.ctx, providers.NamespaceFlow, "k", 0)
	s.ErrorIs(err, providers.ErrRuntimeStoreKeyNotFound)
}

func (s *RedisStoreTestSuite) TestExtendTTL_BackendError() {
	s.client.On("Expire", mock.Anything, mock.Anything, mock.Anything).
		Return(redis.NewBoolResult(false, errors.New("connection refused")))

	err := s.store.ExtendTTL(s.ctx, providers.NamespaceFlow, "k", 60)
	s.Error(err)
	s.Contains(err.Error(), "failed to extend TTL in Redis")
}

func (s *RedisStoreTestSuite) TestExtendTTL_ValuePreserved() {
	key := s.store.getFormattedKey(providers.NamespaceFlow, "k")
	s.client.On("Set", mock.Anything, key, []byte("v"), time.Duration(0)).
		Return(redis.NewStatusResult("OK", nil))
	s.client.On("Expire", mock.Anything, key, 60*time.Second).
		Return(redis.NewBoolResult(true, nil))
	s.client.On("Get", mock.Anything, key).Return(redis.NewStringResult("v", nil))

	s.Require().NoError(s.store.Put(s.ctx, providers.NamespaceFlow, "k", []byte("v"), 0))
	s.Require().NoError(s.store.ExtendTTL(s.ctx, providers.NamespaceFlow, "k", 60))

	got, err := s.store.Get(s.ctx, providers.NamespaceFlow, "k")
	s.NoError(err)
	s.Equal([]byte("v"), got)
}

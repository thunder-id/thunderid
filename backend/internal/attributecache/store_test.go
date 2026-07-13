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

package attributecache

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// AttributeCacheStoreTestSuite exercises the attributeCacheStore adapter against a real in-memory
// runtime store, verifying the marshal/namespace/key round-trip and the not-found semantics.
type AttributeCacheStoreTestSuite struct {
	suite.Suite
	store     attributeCacheStoreInterface
	ctx       context.Context
	testCache AttributeCache
}

func TestAttributeCacheStoreSuite(t *testing.T) {
	suite.Run(t, new(AttributeCacheStoreTestSuite))
}

func (suite *AttributeCacheStoreTestSuite) SetupTest() {
	suite.store = newAttributeCacheStore(inmemory.Initialize("test-deployment"))
	suite.ctx = context.Background()

	suite.testCache = AttributeCache{
		ID:         "test-cache-id",
		Attributes: map[string]interface{}{"key": "value"},
		TTLSeconds: 3600, // 1 hour
	}
}

// Tests for CreateAttributeCache

func (suite *AttributeCacheStoreTestSuite) TestCreateAttributeCache_Success() {
	err := suite.store.CreateAttributeCache(suite.ctx, suite.testCache)
	suite.Require().NoError(err)

	got, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)
	suite.Require().NoError(err)
	suite.Equal(suite.testCache.ID, got.ID)
	suite.Equal(suite.testCache.Attributes, got.Attributes)
}

func (suite *AttributeCacheStoreTestSuite) TestCreateAttributeCache_MarshalError() {
	cache := AttributeCache{
		ID:         suite.testCache.ID,
		Attributes: map[string]interface{}{"key": make(chan int)},
		TTLSeconds: suite.testCache.TTLSeconds,
	}

	err := suite.store.CreateAttributeCache(suite.ctx, cache)

	suite.Error(err)
}

// Tests for GetAttributeCache

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_NotFound() {
	got, err := suite.store.GetAttributeCache(suite.ctx, "missing")

	suite.ErrorIs(err, errAttributeCacheNotFound)
	suite.Equal(AttributeCache{}, got)
}

// Tests for ExtendAttributeCacheTTL

func (suite *AttributeCacheStoreTestSuite) TestExtendAttributeCacheTTL_Success() {
	suite.Require().NoError(suite.store.CreateAttributeCache(suite.ctx, suite.testCache))

	err := suite.store.ExtendAttributeCacheTTL(suite.ctx, suite.testCache.ID, 7200)
	suite.Require().NoError(err)

	got, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)
	suite.Require().NoError(err)
	suite.Equal(suite.testCache.Attributes, got.Attributes)
}

func (suite *AttributeCacheStoreTestSuite) TestExtendAttributeCacheTTL_NotFound() {
	err := suite.store.ExtendAttributeCacheTTL(suite.ctx, "missing", 7200)

	suite.Error(err)
}

// Tests for DeleteAttributeCache

func (suite *AttributeCacheStoreTestSuite) TestDeleteAttributeCache_Success() {
	suite.Require().NoError(suite.store.CreateAttributeCache(suite.ctx, suite.testCache))
	suite.Require().NoError(suite.store.DeleteAttributeCache(suite.ctx, suite.testCache.ID))

	_, err := suite.store.GetAttributeCache(suite.ctx, suite.testCache.ID)
	suite.ErrorIs(err, errAttributeCacheNotFound)
}

func (suite *AttributeCacheStoreTestSuite) TestDeleteAttributeCache_NotFound() {
	err := suite.store.DeleteAttributeCache(suite.ctx, "missing")

	suite.NoError(err)
}

// TestUnmarshalError verifies GetAttributeCache surfaces a non-not-found error when the stored
// payload cannot be deserialized.
func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_UnmarshalError() {
	store := inmemory.Initialize("test-deployment")
	suite.Require().NoError(store.Put(suite.ctx, providers.NamespaceAttributeCache, suite.testCache.ID,
		[]byte("not-json"), 60))

	s := newAttributeCacheStore(store)
	_, err := s.GetAttributeCache(suite.ctx, suite.testCache.ID)

	suite.Error(err)
	suite.False(errors.Is(err, errAttributeCacheNotFound))
}

// erroringRuntimeStore wraps a RuntimeStoreProvider and forces Get to fail, so that
// GetAttributeCache's error-propagation path (as opposed to its not-found path) can be exercised.
type erroringRuntimeStore struct {
	providers.RuntimeStoreProvider
}

func (e *erroringRuntimeStore) Get(_ context.Context, _ providers.RuntimeStoreNamespace,
	_ string) ([]byte, error) {
	return nil, errors.New("store unavailable")
}

func (suite *AttributeCacheStoreTestSuite) TestGetAttributeCache_StoreError() {
	s := newAttributeCacheStore(&erroringRuntimeStore{RuntimeStoreProvider: inmemory.Initialize("test-deployment")})

	_, err := s.GetAttributeCache(suite.ctx, suite.testCache.ID)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to get attribute cache")
	suite.False(errors.Is(err, errAttributeCacheNotFound))
}

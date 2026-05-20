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

package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

type RedisCacheTestSuite struct {
	suite.Suite
}

func TestRedisCacheSuite(t *testing.T) {
	suite.Run(t, new(RedisCacheTestSuite))
}

func (suite *RedisCacheTestSuite) SetupSuite() {
	mockConfig := &config.Config{
		Cache: config.CacheConfig{
			Disabled:        false,
			Type:            "redis",
			Size:            1000,
			TTL:             3600,
			EvictionPolicy:  "LRU",
			CleanupInterval: 300,
			Redis: config.RedisConfig{
				Address:   "localhost:6379",
				KeyPrefix: "test",
			},
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/test/thunderid/home", mockConfig)
	if err != nil {
		suite.T().Fatal("Failed to initialize server runtime:", err)
	}
}

func (suite *RedisCacheTestSuite) TearDownSuite() {
	config.ResetServerRuntime()
}

func (suite *RedisCacheTestSuite) TestNewRedisCacheDisabled() {
	t := suite.T()

	cache := newRedisCache[string](
		"TestDisabledCache",
		false,
		nil,
		"test",
		config.CacheConfig{TTL: 60},
		config.CacheProperty{})

	assert.NotNil(t, cache)
	assert.False(t, cache.IsEnabled())
	assert.Equal(t, "TestDisabledCache", cache.GetName())
}

func (suite *RedisCacheTestSuite) TestDisabledCacheOperations() {
	t := suite.T()

	cache := newRedisCache[string](
		"TestDisabledOps",
		false,
		nil,
		"test",
		config.CacheConfig{TTL: 60},
		config.CacheProperty{})

	// Set should be a no-op
	err := cache.Set(context.Background(), CacheKey{Key: "testKey"}, "testValue")
	assert.NoError(t, err)

	// Get should return zero value and false
	val, found := cache.Get(context.Background(), CacheKey{Key: "testKey"})
	assert.False(t, found)
	assert.Empty(t, val)

	// Delete should be a no-op
	err = cache.Delete(context.Background(), CacheKey{Key: "testKey"})
	assert.NoError(t, err)
}

func (suite *RedisCacheTestSuite) TestDisabledCacheStats() {
	t := suite.T()

	cache := newRedisCache[string]("TestDisabledStats",
		false,
		nil,
		"test",
		config.CacheConfig{TTL: 60},
		config.CacheProperty{})

	stats := cache.GetStats()
	assert.False(t, stats.Enabled)
}

func (suite *RedisCacheTestSuite) TestCleanupExpiredIsNoOp() {
	t := suite.T()

	cache := newRedisCache[string]("TestCleanupExpired",
		false,
		nil,
		"test",
		config.CacheConfig{TTL: 60},
		config.CacheProperty{})

	// CleanupExpired is a no-op for Redis as TTL-based expiry is handled natively
	assert.NotPanics(t, func() {
		cache.CleanupExpired()
	})
}

func (suite *RedisCacheTestSuite) TestBuildKey() {
	t := suite.T()

	cache := &redisCache[string]{
		enabled:   true,
		name:      "TestCache",
		keyPrefix: "thunderid",
	}

	key := cache.buildKey(CacheKey{Key: "myKey"})
	assert.Equal(t, "thunderid:TestCache:myKey", key)
}

func (suite *RedisCacheTestSuite) TestBuildKeyWithEmptyPrefix() {
	t := suite.T()

	cache := &redisCache[string]{
		enabled:   true,
		name:      "TestCache",
		keyPrefix: "",
	}

	key := cache.buildKey(CacheKey{Key: "myKey"})
	assert.Equal(t, ":TestCache:myKey", key)
}

func (suite *RedisCacheTestSuite) TestGetCacheTypeRedis() {
	t := suite.T()

	cacheConfig := config.CacheConfig{
		Type: "redis",
	}
	assert.Equal(t, cacheTypeRedis, getCacheType(cacheConfig))
}

func (suite *RedisCacheTestSuite) TestGetCacheTypeInMemory() {
	t := suite.T()

	cacheConfig := config.CacheConfig{
		Type: "inmemory",
	}
	assert.Equal(t, cacheTypeInMemory, getCacheType(cacheConfig))
}

func (suite *RedisCacheTestSuite) TestGetCacheTypeEmpty() {
	t := suite.T()

	cacheConfig := config.CacheConfig{
		Type: "",
	}
	assert.Equal(t, cacheTypeInMemory, getCacheType(cacheConfig))
}

func (suite *RedisCacheTestSuite) TestGetCacheTypeUnknown() {
	t := suite.T()

	cacheConfig := config.CacheConfig{
		Type: "memcached",
	}
	assert.Equal(t, cacheTypeInMemory, getCacheType(cacheConfig))
}

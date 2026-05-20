/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
 * KIND, either express or cacheImplied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
)

const (
	testValue = "testValue"
)

type CacheTestSuite struct {
	suite.Suite
}

func TestCacheTestSuite(t *testing.T) {
	suite.Run(t, new(CacheTestSuite))
}

func (suite *CacheTestSuite) SetupSuite() {
	mockConfig := &config.Config{
		Cache: config.CacheConfig{
			Disabled:        false,
			Size:            1000,
			TTL:             3600,
			EvictionPolicy:  "LRU",
			CleanupInterval: 300,
		},
	}
	config.ResetServerRuntime()
	err := config.InitializeServerRuntime("/test/thunderid/home", mockConfig)
	if err != nil {
		suite.T().Fatal("Failed to initialize server runtime:", err)
	}
}

func (suite *CacheTestSuite) TestIsEnabled() {
	t := suite.T()

	// Test enabled cache
	mockCache := NewCacheInterfaceMock[string](t)
	enabledCache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}
	assert.True(t, enabledCache.IsEnabled())

	// Test disabled cache
	disabledCache := &Cache[string]{
		enabled:   false,
		cacheImpl: nil,
	}
	assert.False(t, disabledCache.IsEnabled())
}

func (suite *CacheTestSuite) TestSet() {
	t := suite.T()

	// Test with enabled cache
	mockCache := NewCacheInterfaceMock[string](t)
	mockCache.EXPECT().IsEnabled().Return(true)

	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	key := CacheKey{Key: "testKey"}

	// Set up expectation for Set
	mockCache.EXPECT().Set(context.Background(), key, testValue).Return(nil)

	// Call Set and verify
	err := cache.Set(context.Background(), key, testValue)
	assert.NoError(t, err)

	// Test with disabled cache
	disabledCache := &Cache[string]{
		enabled:   false,
		cacheImpl: nil,
	}

	// Should be a no-op with disabled cache
	err = disabledCache.Set(context.Background(), key, testValue)
	assert.NoError(t, err)
}

func (suite *CacheTestSuite) TestSetWithError() {
	t := suite.T()

	mockCache := NewCacheInterfaceMock[string](t)
	mockCache.EXPECT().IsEnabled().Return(true)

	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	key := CacheKey{Key: "testKey"}

	// Set up expectation for Set to return error
	mockCache.EXPECT().Set(context.Background(), key, testValue).Return(fmt.Errorf("set error"))

	// Even with error, Set should not return error (logged instead)
	err := cache.Set(context.Background(), key, testValue)
	assert.NoError(t, err)
}

func (suite *CacheTestSuite) TestGet() {
	t := suite.T()

	// Test 1: Test with enabled cache and value found
	mockCache1 := NewCacheInterfaceMock[string](t)
	mockCache1.EXPECT().IsEnabled().Return(true)

	cache1 := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache1,
	}

	key := CacheKey{Key: "testKey"}

	mockCache1.EXPECT().Get(context.Background(), key).Return(testValue, true)
	value, found := cache1.Get(context.Background(), key)
	assert.True(t, found)
	assert.Equal(t, testValue, value)

	// Test 2: Test with enabled cache and value not found
	mockCache2 := NewCacheInterfaceMock[string](t)
	mockCache2.EXPECT().IsEnabled().Return(true)

	cache2 := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache2,
	}

	mockCache2.EXPECT().Get(context.Background(), key).Return("", false)
	value2, found2 := cache2.Get(context.Background(), key)
	assert.False(t, found2)
	assert.Equal(t, "", value2)

	// Test 3: Test with disabled cache
	disabledCache := &Cache[string]{
		enabled:   false,
		cacheImpl: nil,
	}

	// Should return not found with disabled cache
	value3, found3 := disabledCache.Get(context.Background(), key)
	assert.False(t, found3)
	assert.Equal(t, "", value3)
}

func (suite *CacheTestSuite) TestDelete() {
	t := suite.T()

	// Test with enabled cache
	mockCache := NewCacheInterfaceMock[string](t)
	mockCache.EXPECT().IsEnabled().Return(true)

	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	key := CacheKey{Key: "testKey"}

	// Set up expectation for Delete
	mockCache.EXPECT().Delete(context.Background(), key).Return(nil)

	// Call Delete and verify
	err := cache.Delete(context.Background(), key)
	assert.NoError(t, err)

	// Test with disabled cache
	disabledCache := &Cache[string]{
		enabled:   false,
		cacheImpl: nil,
	}

	// Should be a no-op with disabled cache
	err = disabledCache.Delete(context.Background(), key)
	assert.NoError(t, err)
}

func (suite *CacheTestSuite) TestDeleteWithError() {
	t := suite.T()

	mockCache := NewCacheInterfaceMock[string](t)
	mockCache.EXPECT().IsEnabled().Return(true)

	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	key := CacheKey{Key: "testKey"}

	// Set up expectation for Delete to return error
	mockCache.EXPECT().Delete(context.Background(), key).Return(fmt.Errorf("delete error"))

	// Even with error, Delete should not return error (logged instead)
	err := cache.Delete(context.Background(), key)
	assert.NoError(t, err)
}

func (suite *CacheTestSuite) TestClear() {
	t := suite.T()

	// Test with enabled cache
	mockCache := NewCacheInterfaceMock[string](t)
	mockCache.EXPECT().IsEnabled().Return(true)

	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	// Set up expectation for Clear
	mockCache.EXPECT().Clear(context.Background()).Return(nil)

	// Call Clear and verify
	err := cache.Clear(context.Background())
	assert.NoError(t, err)

	// Test with disabled cache
	disabledCache := &Cache[string]{
		enabled:   false,
		cacheImpl: nil,
	}

	// Should be a no-op with disabled cache
	err = disabledCache.Clear(context.Background())
	assert.NoError(t, err)
}

func (suite *CacheTestSuite) TestClearWithError() {
	t := suite.T()

	mockCache := NewCacheInterfaceMock[string](t)
	mockCache.EXPECT().IsEnabled().Return(true)

	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	// Set up expectation for Clear to return error
	mockCache.EXPECT().Clear(context.Background()).Return(fmt.Errorf("clear error"))

	// Even with error, Clear should not return error (logged instead)
	err := cache.Clear(context.Background())
	assert.NoError(t, err)
}

func (suite *CacheTestSuite) TestGetCacheProperty() {
	testCases := []struct {
		name             string
		cacheName        string
		cacheConfig      config.CacheConfig
		expectedProperty config.CacheProperty
	}{
		{
			name:      "ExistingProperty",
			cacheName: "testCache",
			cacheConfig: config.CacheConfig{
				Properties: []config.CacheProperty{
					{
						Name:     "testCache",
						Disabled: false,
						Size:     100,
						TTL:      60,
					},
				},
			},
			expectedProperty: config.CacheProperty{
				Name:     "testCache",
				Disabled: false,
				Size:     100,
				TTL:      60,
			},
		},
		{
			name:      "NonExistingProperty",
			cacheName: "nonExistingCache",
			cacheConfig: config.CacheConfig{
				Properties: []config.CacheProperty{
					{
						Name:     "testCache",
						Disabled: false,
						Size:     100,
						TTL:      60,
					},
				},
			},
			expectedProperty: config.CacheProperty{},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			property := getCacheProperty(tc.cacheConfig, tc.cacheName)
			assert.Equal(t, tc.expectedProperty, property)
		})
	}
}

func (suite *CacheTestSuite) TestGetEvictionPolicy() {
	testCases := []struct {
		name                   string
		cacheConfig            config.CacheConfig
		cacheProperty          config.CacheProperty
		expectedEvictionPolicy evictionPolicy
	}{
		{
			name: "PropertyLFUEvictionPolicy",
			cacheConfig: config.CacheConfig{
				EvictionPolicy: string(evictionPolicyLRU),
			},
			cacheProperty: config.CacheProperty{
				EvictionPolicy: string(evictionPolicyLFU),
			},
			expectedEvictionPolicy: evictionPolicyLFU,
		},
		{
			name: "ConfigLRUEvictionPolicy",
			cacheConfig: config.CacheConfig{
				EvictionPolicy: string(evictionPolicyLRU),
			},
			cacheProperty:          config.CacheProperty{},
			expectedEvictionPolicy: evictionPolicyLRU,
		},
		{
			name:                   "DefaultLRUEvictionPolicy",
			cacheConfig:            config.CacheConfig{},
			cacheProperty:          config.CacheProperty{},
			expectedEvictionPolicy: evictionPolicyLRU,
		},
		{
			name: "InvalidEvictionPolicy",
			cacheConfig: config.CacheConfig{
				EvictionPolicy: "INVALID",
			},
			cacheProperty:          config.CacheProperty{},
			expectedEvictionPolicy: evictionPolicyLRU,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			evictionPolicy := getEvictionPolicy(tc.cacheConfig, tc.cacheProperty)
			assert.Equal(t, tc.expectedEvictionPolicy, evictionPolicy)
		})
	}
}

func (suite *CacheTestSuite) TestGetCacheType() {
	testCases := []struct {
		name              string
		cacheConfig       config.CacheConfig
		expectedCacheType cacheType
	}{
		{
			name: "InMemoryCacheType",
			cacheConfig: config.CacheConfig{
				Type: string(cacheTypeInMemory),
			},
			expectedCacheType: cacheTypeInMemory,
		},
		{
			name:              "DefaultCacheType",
			cacheConfig:       config.CacheConfig{},
			expectedCacheType: cacheTypeInMemory,
		},
		{
			name: "UnknownCacheType",
			cacheConfig: config.CacheConfig{
				Type: "unknown",
			},
			expectedCacheType: cacheTypeInMemory,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			cacheType := getCacheType(tc.cacheConfig)
			assert.Equal(t, tc.expectedCacheType, cacheType)
		})
	}
}

//nolint:dupl // Testing different functions with similar test patterns
func (suite *CacheTestSuite) TestGetCacheSize() {
	testCases := []struct {
		name              string
		cacheConfig       config.CacheConfig
		cacheProperty     config.CacheProperty
		expectedCacheSize int
	}{
		{
			name: "PropertySize",
			cacheConfig: config.CacheConfig{
				Size: 500,
			},
			cacheProperty: config.CacheProperty{
				Size: 200,
			},
			expectedCacheSize: 200,
		},
		{
			name: "ConfigSize",
			cacheConfig: config.CacheConfig{
				Size: 500,
			},
			cacheProperty:     config.CacheProperty{},
			expectedCacheSize: 500,
		},
		{
			name: "ZeroPropertySize",
			cacheConfig: config.CacheConfig{
				Size: 500,
			},
			cacheProperty: config.CacheProperty{
				Size: 0,
			},
			expectedCacheSize: 500,
		},
		{
			name: "NegativePropertySize",
			cacheConfig: config.CacheConfig{
				Size: 500,
			},
			cacheProperty: config.CacheProperty{
				Size: -1,
			},
			expectedCacheSize: 500,
		},
		{
			name: "ZeroConfigSize",
			cacheConfig: config.CacheConfig{
				Size: 0,
			},
			cacheProperty:     config.CacheProperty{},
			expectedCacheSize: 0,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			size := getCacheSize(tc.cacheConfig, tc.cacheProperty)
			assert.Equal(t, tc.expectedCacheSize, size)
		})
	}
}

//nolint:dupl // Testing different functions with similar test patterns
func (suite *CacheTestSuite) TestGetCacheTTL() {
	testCases := []struct {
		name             string
		cacheConfig      config.CacheConfig
		cacheProperty    config.CacheProperty
		expectedCacheTTL time.Duration
	}{
		{
			name: "PropertyTTL",
			cacheConfig: config.CacheConfig{
				TTL: 1800,
			},
			cacheProperty: config.CacheProperty{
				TTL: 900,
			},
			expectedCacheTTL: 900 * time.Second,
		},
		{
			name: "ConfigTTL",
			cacheConfig: config.CacheConfig{
				TTL: 1800,
			},
			cacheProperty:    config.CacheProperty{},
			expectedCacheTTL: 1800 * time.Second,
		},
		{
			name: "ZeroPropertyTTL",
			cacheConfig: config.CacheConfig{
				TTL: 1800,
			},
			cacheProperty: config.CacheProperty{
				TTL: 0,
			},
			expectedCacheTTL: 1800 * time.Second,
		},
		{
			name: "NegativePropertyTTL",
			cacheConfig: config.CacheConfig{
				TTL: 1800,
			},
			cacheProperty: config.CacheProperty{
				TTL: -1,
			},
			expectedCacheTTL: 1800 * time.Second,
		},
		{
			name: "ZeroConfigTTL",
			cacheConfig: config.CacheConfig{
				TTL: 0,
			},
			cacheProperty:    config.CacheProperty{},
			expectedCacheTTL: 0,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			ttl := getCacheTTL(tc.cacheConfig, tc.cacheProperty)
			assert.Equal(t, tc.expectedCacheTTL, ttl)
		})
	}
}

func (suite *CacheTestSuite) TestCacheWithFailingOperations() {
	t := suite.T()

	// Create a mock cache for testing error scenarios
	mockCache := NewCacheInterfaceMock[string](t)

	// Configure the mock
	mockCache.EXPECT().IsEnabled().Return(true).Maybe()
	mockCache.EXPECT().GetName().Return("mockErrorCache").Maybe()

	// Create a cache with the mock
	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	// Test Set with error
	key := CacheKey{Key: "testKey"}

	// Configure mock to return error on Set
	mockCache.EXPECT().Set(context.Background(), key, testValue).Return(fmt.Errorf("set error"))

	// Set should not return the error but log it
	err := cache.Set(context.Background(), key, testValue)
	assert.NoError(t, err)

	// Test Delete with error
	// Configure mock to return error on Delete
	mockCache.EXPECT().Delete(context.Background(), key).Return(fmt.Errorf("delete error"))

	// Delete should not return the error but log it
	err = cache.Delete(context.Background(), key)
	assert.NoError(t, err)

	// Test Clear with error
	// Configure mock to return error on Clear
	mockCache.EXPECT().Clear(context.Background()).Return(fmt.Errorf("clear error"))

	// Clear should not return the error but log it
	err = cache.Clear(context.Background())
	assert.NoError(t, err)
}

func (suite *CacheTestSuite) TestDisabledInnerCacheScenario() {
	t := suite.T()

	// Create a mock cache for testing
	mockCache := NewCacheInterfaceMock[string](t)

	// Configure the mock to indicate it's disabled
	mockCache.EXPECT().IsEnabled().Return(false)
	// Since it's disabled, no other methods should be called

	// Create a cache with the mock
	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	// Test operations with disabled inner cache
	key := CacheKey{Key: "testKey"}

	// Set should be a no-op with disabled inner cache
	err := cache.Set(context.Background(), key, testValue)
	assert.NoError(t, err)

	// Get should return not found with disabled inner cache
	retrievedValue, found := cache.Get(context.Background(), key)
	assert.False(t, found)
	var zero string
	assert.Equal(t, zero, retrievedValue)

	// Delete should be a no-op with disabled inner cache
	err = cache.Delete(context.Background(), key)
	assert.NoError(t, err)

	// Clear should be a no-op with disabled inner cache
	err = cache.Clear(context.Background())
	assert.NoError(t, err)
}

func (suite *CacheTestSuite) TestDisabledInnerCacheOnly() {
	t := suite.T()

	mockCache := NewCacheInterfaceMock[string](t)
	mockCache.EXPECT().IsEnabled().Return(false)
	// Since it's disabled, check IsEnabled multiple times for each operation
	mockCache.EXPECT().IsEnabled().Return(false)
	mockCache.EXPECT().IsEnabled().Return(false)
	mockCache.EXPECT().IsEnabled().Return(false)

	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	// Test operations with disabled inner cache
	key := CacheKey{Key: "testKey"}

	// All operations should be no-ops when inner cache is disabled
	err := cache.Set(context.Background(), key, testValue)
	assert.NoError(t, err)

	val, found := cache.Get(context.Background(), key)
	assert.False(t, found)
	assert.Equal(t, "", val)

	err = cache.Delete(context.Background(), key)
	assert.NoError(t, err)

	err = cache.Clear(context.Background())
	assert.NoError(t, err)
}

func (suite *CacheTestSuite) TestGetStats() {
	t := suite.T()

	mockCache := NewCacheInterfaceMock[string](t)

	expectedStats := CacheStat{
		Enabled:    true,
		Size:       10,
		MaxSize:    100,
		HitCount:   5,
		MissCount:  3,
		HitRate:    0.625,
		EvictCount: 1,
	}
	mockCache.EXPECT().GetStats().Return(expectedStats)

	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	stats := cache.GetStats()
	assert.Equal(t, expectedStats, stats)

	// Test with disabled cache (nil cacheImpl)
	disabledCache := &Cache[string]{
		enabled:   false,
		cacheImpl: nil,
	}
	stats = disabledCache.GetStats()
	assert.Equal(t, CacheStat{Enabled: false}, stats)
}

func (suite *CacheTestSuite) TestMultipleValues() {
	t := suite.T()

	mockCache := NewCacheInterfaceMock[string](t)
	// Need to set multiple expectations for multiple IsEnabled calls
	mockCache.EXPECT().IsEnabled().Return(true)
	mockCache.EXPECT().IsEnabled().Return(true)
	mockCache.EXPECT().IsEnabled().Return(true)

	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	// Define test data
	keys := []CacheKey{
		{Key: "key1"},
		{Key: "key2"},
		{Key: "key3"},
	}
	values := []string{"value1", "value2", "value3"}

	// Test Set operations
	for i := range keys {
		mockCache.EXPECT().Set(context.Background(), keys[i], values[i]).Return(nil)
		err := cache.Set(context.Background(), keys[i], values[i])
		assert.NoError(t, err)
	}

	// Test Get operations with different outcomes
	mockCache.EXPECT().Get(context.Background(), keys[0]).Return(values[0], true)
	mockCache.EXPECT().Get(context.Background(), keys[1]).Return("", false)
	mockCache.EXPECT().Get(context.Background(), keys[2]).Return(values[2], true)

	val1, found1 := cache.Get(context.Background(), keys[0])
	assert.True(t, found1)
	assert.Equal(t, values[0], val1)

	val2, found2 := cache.Get(context.Background(), keys[1])
	assert.False(t, found2)
	assert.Equal(t, "", val2)

	val3, found3 := cache.Get(context.Background(), keys[2])
	assert.True(t, found3)
	assert.Equal(t, values[2], val3)
}

func (suite *CacheTestSuite) TestCleanupExpired() {
	t := suite.T()

	// Use a real inMemoryCache to verify CleanupExpired is delegated to the inner cache.
	internalCache := newInMemoryCache[string]("testCleanup", true,
		config.CacheConfig{Size: 100, TTL: 1, EvictionPolicy: "LRU"}, config.CacheProperty{})
	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: internalCache,
	}

	key := CacheKey{Key: "expiredKey"}
	_ = internalCache.Set(context.Background(), key, "value")

	// Wait for TTL to expire
	time.Sleep(1100 * time.Millisecond)

	cache.CleanupExpired()

	_, found := internalCache.Get(context.Background(), key)
	assert.False(t, found, "entry should have been removed by CleanupExpired")
}

func (suite *CacheTestSuite) TestGetName() {
	t := suite.T()

	// Test with a named cache
	cacheName := "testCacheName"
	cache := &Cache[string]{
		enabled:   true,
		cacheName: cacheName,
	}

	// Verify the GetName method returns the correct name
	assert.Equal(t, cacheName, cache.GetName(), "GetName should return the cache name")
}

func (suite *CacheTestSuite) TestCacheKeyToString() {
	t := suite.T()

	key := CacheKey{Key: "testKey"}
	assert.Equal(t, "testKey", key.ToString(), "ToString should return the Key value")

	emptyKey := CacheKey{Key: ""}
	assert.Equal(t, "", emptyKey.ToString(), "ToString should return empty string for empty Key")
}

func (suite *CacheTestSuite) TestCacheWithNilcacheImpl() {
	t := suite.T()

	// Create a cache with nil internal cache but enabled flag set to false
	// This is important because cache.Set checks both enabled and cacheImpl.IsEnabled()
	cache := &Cache[string]{
		enabled:   false, // Set to false since cacheImpl is nil
		cacheImpl: nil,
		cacheName: "nilcacheImpl",
	}

	// Test operations with nil internal cache
	key := CacheKey{Key: "testKey"}

	// All operations should be no-ops and not panic
	err := cache.Set(context.Background(), key, testValue)
	assert.NoError(t, err)

	val, found := cache.Get(context.Background(), key)
	assert.False(t, found)
	assert.Equal(t, "", val)

	err = cache.Delete(context.Background(), key)
	assert.NoError(t, err)

	err = cache.Clear(context.Background())
	assert.NoError(t, err)

	// Should not panic
	cache.CleanupExpired()
}

func (suite *CacheTestSuite) TestCacheWithEmptyKeyOperations() {
	t := suite.T()

	// Create a mock cache for testing
	mockCache := NewCacheInterfaceMock[string](t)
	mockCache.EXPECT().IsEnabled().Return(true).Times(3)

	// Create a cache with the mock
	cache := &Cache[string]{
		enabled:   true,
		cacheImpl: mockCache,
	}

	// Test operations with empty key
	emptyKey := CacheKey{Key: ""}

	// Set should work with empty key
	mockCache.EXPECT().Set(context.Background(), emptyKey, testValue).Return(nil)
	err := cache.Set(context.Background(), emptyKey, testValue)
	assert.NoError(t, err)

	// Get should work with empty key
	mockCache.EXPECT().Get(context.Background(), emptyKey).Return(testValue, true)
	val, found := cache.Get(context.Background(), emptyKey)
	assert.True(t, found)
	assert.Equal(t, testValue, val)

	// Delete should work with empty key
	mockCache.EXPECT().Delete(context.Background(), emptyKey).Return(nil)
	err = cache.Delete(context.Background(), emptyKey)
	assert.NoError(t, err)
}

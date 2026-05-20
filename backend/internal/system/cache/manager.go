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
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package cache

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// CacheManagerInterface defines the interface for managing caches.
type CacheManagerInterface interface {
	Close()
	IsEnabled() bool
	getMutex() *sync.RWMutex
	getCache(cacheKey string) (interface{}, bool)
	addCache(cacheKey string, cacheInstance interface{})
	getRedisClient() *redis.Client
	startCleanupRoutine()
	cleanupAllCaches()
	reset()
}

// CacheManager implements the CacheManagerInterface for managing multiple caches.
type CacheManager struct {
	caches          map[string]interface{}
	mu              sync.RWMutex
	enabled         bool
	cleanupInterval time.Duration
	redisClient     *redis.Client
}

// Initialize creates and returns a new CacheManagerInterface instance.
// Initialize creates, configures, and returns a ready-to-use CacheManagerInterface.
func Initialize() CacheManagerInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CacheManager"))
	logger.Debug("Initializing Cache Manager")

	cm := &CacheManager{
		caches: make(map[string]interface{}),
	}

	cacheConfig := config.GetServerRuntime().Config.Cache
	if cacheConfig.Disabled {
		logger.Debug("Caching is disabled. Skipping initialization")
		return cm
	}

	cm.enabled = true

	if getCacheType(cacheConfig) == cacheTypeRedis {
		cm.redisClient = redis.NewClient(&redis.Options{
			Addr:            cacheConfig.Redis.Address,
			Username:        cacheConfig.Redis.Username,
			Password:        cacheConfig.Redis.Password,
			DB:              cacheConfig.Redis.DB,
			MaxRetries:      cacheConfig.Redis.MaxRetries,
			MinRetryBackoff: time.Duration(cacheConfig.Redis.MinRetryBackoffMS) * time.Millisecond,
			MaxRetryBackoff: time.Duration(cacheConfig.Redis.MaxRetryBackoffMS) * time.Millisecond,
			DialTimeout:     time.Duration(cacheConfig.Redis.DialTimeoutMS) * time.Millisecond,
			ReadTimeout:     time.Duration(cacheConfig.Redis.ReadTimeoutMS) * time.Millisecond,
			WriteTimeout:    time.Duration(cacheConfig.Redis.WriteTimeoutMS) * time.Millisecond,
		})
		if err := cm.redisClient.Ping(context.Background()).Err(); err != nil {
			logger.Error("Failed to connect to Redis. Cache initialization aborted.", log.Error(err))
			if closeErr := cm.redisClient.Close(); closeErr != nil {
				logger.Warn("Failed to close Redis client after ping failure", log.Error(closeErr))
			}
			cm.redisClient = nil
			cm.enabled = false
			return cm
		}
		logger.Debug("Connected to Redis successfully", log.String("address", cacheConfig.Redis.Address))
	} else {
		cm.cleanupInterval = getCleanupInterval(cacheConfig)
		cm.startCleanupRoutine()
	}

	logger.Debug("Cache Manager initialized", log.Bool("enabled", cm.enabled),
		log.Any("cleanupInterval", cm.cleanupInterval))
	return cm
}

// Close shuts down the CacheManager and releases resources.
func (cm *CacheManager) Close() {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CacheManager"))

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.redisClient != nil {
		if err := cm.redisClient.Close(); err != nil {
			logger.Warn("Failed to close Redis client", log.Error(err))
		} else {
			logger.Debug("Redis client closed")
		}
		cm.redisClient = nil
	}

	cm.caches = nil
	cm.enabled = false
}

// IsEnabled checks if the CacheManager is enabled.
func (cm *CacheManager) IsEnabled() bool {
	return cm.enabled
}

// getMutex returns the mutex for synchronizing access to the caches.
func (cm *CacheManager) getMutex() *sync.RWMutex {
	return &cm.mu
}

// getCache retrieves a cache instance by its key.
func (cm *CacheManager) getCache(cacheKey string) (interface{}, bool) {
	cacheInstance, exists := cm.caches[cacheKey]
	return cacheInstance, exists
}

// addCache adds a new cache instance to the manager.
func (cm *CacheManager) addCache(cacheKey string, cacheInstance interface{}) {
	if _, exists := cm.caches[cacheKey]; !exists {
		cm.caches[cacheKey] = cacheInstance
		log.GetLogger().Debug("Cache added", log.String("cacheKey", cacheKey))
	}
}

// getRedisClient returns the shared Redis client, or nil if Redis is not configured.
func (cm *CacheManager) getRedisClient() *redis.Client {
	return cm.redisClient
}

// startCleanupRoutine starts a background routine to clean up expired caches at regular intervals.
func (cm *CacheManager) startCleanupRoutine() {
	if cm.cleanupInterval <= 0 {
		return
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CacheManager"))
	logger.Debug("Starting cleanup routine for caches")

	go func() {
		ticker := time.NewTicker(cm.cleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			cm.cleanupAllCaches()
		}
	}()

	logger.Debug("Cleanup routine started", log.Any("interval", cm.cleanupInterval))
}

// cleanupAllCaches cleans up expired entries in all caches.
func (cm *CacheManager) cleanupAllCaches() {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CacheManager"))
	logger.Debug("Cleaning up expired caches")

	for _, cacheEntry := range cm.caches {
		// Use type switch to handle different cache types
		switch cache := cacheEntry.(type) {
		case interface {
			IsEnabled() bool
			GetName() string
			CleanupExpired()
		}:
			if cache.IsEnabled() {
				logger.Debug("Cleaning up cache", log.String("cacheName", cache.GetName()))
				cache.CleanupExpired()
			}
		default:
			logger.Warn("Unknown cache type encountered", log.Any("type", reflect.TypeOf(cacheEntry)))
		}
	}
}

// reset resets the CacheManager and clears all state.
func (cm *CacheManager) reset() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.caches = make(map[string]interface{})
	cm.enabled = false
}

// buildRedisKeyPrefix composes the Redis key prefix with deployment ID for per-deployment isolation.
func buildRedisKeyPrefix(basePrefix string) string {
	deploymentID := config.GetServerRuntime().Config.Server.Identifier
	if deploymentID == "" {
		return basePrefix
	}

	if basePrefix == "" {
		return deploymentID
	}

	return basePrefix + ":" + deploymentID
}

// newCache creates a new cache instance.
func newCache[T any](cm CacheManagerInterface, cacheName string) CacheInterface[T] {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CacheManager"),
		log.String("cacheName", cacheName))

	cacheConfig := config.GetServerRuntime().Config.Cache
	if cacheConfig.Disabled {
		logger.Debug("Caching is disabled, returning empty")
		return &Cache[T]{
			enabled:   false,
			cacheName: cacheName,
			cacheImpl: nil,
		}
	}

	cacheProperty := getCacheProperty(cacheConfig, cacheName)

	if cacheProperty.Disabled {
		logger.Debug("Individual cache is disabled, returning empty")
		return &Cache[T]{
			enabled:   false,
			cacheName: cacheName,
			cacheImpl: nil,
		}
	}

	logger.Debug("Initializing the cache")

	var internalCache CacheInterface[T]
	switch getCacheType(cacheConfig) {
	case cacheTypeInMemory:
		internalCache = newInMemoryCache[T](
			cacheName,
			!cacheProperty.Disabled,
			cacheConfig,
			cacheProperty,
		)
	case cacheTypeRedis:
		redisClient := cm.getRedisClient()
		if redisClient == nil {
			logger.Warn("Redis client not available, disabling cache")
			return &Cache[T]{
				enabled:   false,
				cacheName: cacheName,
				cacheImpl: nil,
			}
		} else {
			keyPrefix := buildRedisKeyPrefix(cacheConfig.Redis.KeyPrefix)
			internalCache = newRedisCache[T](
				cacheName,
				!cacheProperty.Disabled,
				redisClient,
				keyPrefix,
				cacheConfig,
				cacheProperty,
			)
		}
	default:
		logger.Warn("Unknown cache type, defaulting to in-memory cache")
		internalCache = newInMemoryCache[T](
			cacheName,
			!cacheProperty.Disabled,
			cacheConfig,
			cacheProperty,
		)
	}

	cacheInst := &Cache[T]{
		enabled:   true,
		cacheName: cacheName,
		cacheImpl: internalCache,
	}

	return cacheInst
}

// GetInMemoryCache returns a singleton in-memory cache instance for the given type and cache name.
func GetInMemoryCache[T any](cm CacheManagerInterface, cacheName string) CacheInterface[T] {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CacheManager"))

	var t T
	typeName := reflect.TypeOf(t).String()
	cacheKey := cacheName + ":" + typeName

	cm.getMutex().RLock()
	if cache, exists := cm.getCache(cacheKey); exists {
		cm.getMutex().RUnlock()
		if retCache, ok := cache.(CacheInterface[T]); ok {
			return retCache
		}
	} else {
		cm.getMutex().RUnlock()
	}

	cm.getMutex().Lock()
	defer cm.getMutex().Unlock()

	if cache, exists := cm.getCache(cacheKey); exists {
		if retCache, ok := cache.(CacheInterface[T]); ok {
			return retCache
		}
	}

	logger.Debug("Creating new in-memory cache", log.String("cacheName", cacheName), log.String("type", typeName))

	cacheConfig := config.GetServerRuntime().Config.Cache
	cacheProperty := getCacheProperty(cacheConfig, cacheName)

	var internalCache CacheInterface[T]
	if cacheConfig.Disabled || cacheProperty.Disabled {
		internalCache = &inMemoryCache[T]{name: cacheName, enabled: false}
	} else {
		internalCache = newInMemoryCache[T](cacheName, true, cacheConfig, cacheProperty)
	}

	newCacheInst := &Cache[T]{
		enabled:   !cacheConfig.Disabled && !cacheProperty.Disabled,
		cacheName: cacheName,
		cacheImpl: internalCache,
	}
	cm.addCache(cacheKey, newCacheInst)
	return newCacheInst
}

// GetCache returns a singleton cache instance for the given type and cache name.
func GetCache[T any](cm CacheManagerInterface, cacheName string) CacheInterface[T] {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "CacheManager"))

	// Create unique key for the cache
	var t T
	typeName := reflect.TypeOf(t).String()
	cacheKey := cacheName + ":" + typeName

	// First try to get from the map
	cm.getMutex().RLock()
	if cache, exists := cm.getCache(cacheKey); exists {
		cm.getMutex().RUnlock()
		if retCache, ok := cache.(CacheInterface[T]); ok {
			return retCache
		}
		logger.Warn("Type mismatch for cache", log.String("cacheName", cacheName),
			log.String("expectedType", typeName), log.String("actualType", reflect.TypeOf(cache).String()))

		return nil
	}
	cm.getMutex().RUnlock()

	// Acquire write lock to create a new cache
	cm.getMutex().Lock()
	defer cm.getMutex().Unlock()

	if cache, exists := cm.getCache(cacheKey); exists {
		if retCache, ok := cache.(CacheInterface[T]); ok {
			return retCache
		}
		logger.Warn("Type mismatch for cache", log.String("cacheName", cacheName),
			log.String("expectedType", typeName), log.String("actualType", reflect.TypeOf(cache).String()))

		return nil
	}

	// Create a new cache
	logger.Debug("Creating new cache", log.String("cacheName", cacheName), log.String("type", typeName))
	newCacheInst := newCache[T](cm, cacheName)
	cm.addCache(cacheKey, newCacheInst)

	return newCacheInst
}

// getCacheType retrieves the cache type from the configuration.
func getCacheType(cacheConfig config.CacheConfig) cacheType {
	if cacheConfig.Type == "" {
		return cacheTypeInMemory
	}
	switch cacheConfig.Type {
	case string(cacheTypeInMemory):
		return cacheTypeInMemory
	case string(cacheTypeRedis):
		return cacheTypeRedis
	default:
		log.GetLogger().Warn("Unknown cache type, defaulting to in-memory cache")
		return cacheTypeInMemory
	}
}

// getCacheProperty retrieves the cache property for the specified cache name.
func getCacheProperty(cacheConfig config.CacheConfig, cacheName string) config.CacheProperty {
	for _, property := range cacheConfig.Properties {
		if property.Name == cacheName {
			return property
		}
	}
	return config.CacheProperty{}
}

// getCleanupInterval retrieves the cleanup interval from the cache configuration.
func getCleanupInterval(cacheConfig config.CacheConfig) time.Duration {
	cleanupIntervalInt := cacheConfig.CleanupInterval
	return time.Duration(cleanupIntervalInt) * time.Second
}

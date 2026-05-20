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

package provider

import (
	"context"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// DataSourceTypeRedis is the type identifier for a Redis data source.
const DataSourceTypeRedis = "redis"

// RedisProviderInterface provides a Redis client for runtime store operations.
type RedisProviderInterface interface {
	GetRedisClient() *redis.Client
	GetKeyPrefix() string
}

// RedisProviderCloser is a separate interface for closing the provider.
// Only the lifecycle manager should use this interface.
type RedisProviderCloser interface {
	Close() error
}

// redisProvider is the implementation of RedisProviderInterface.
type redisProvider struct {
	client    *redis.Client
	keyPrefix string
	mu        sync.RWMutex
}

var (
	redisInstance *redisProvider
	redisOnce     sync.Once
)

// initRedisProvider initializes the singleton Redis provider.
func initRedisProvider() {
	redisOnce.Do(func() {
		cfg := config.GetServerRuntime().Config.Database.Runtime
		// This is a no-op when runtime.type is not "redis".
		if cfg.Type != DataSourceTypeRedis {
			return
		}

		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RedisProvider"))

		r := cfg.Redis
		client := redis.NewClient(&redis.Options{
			Addr:            r.Address,
			Username:        r.Username,
			Password:        r.Password,
			DB:              r.DB,
			MaxRetries:      r.MaxRetries,
			MinRetryBackoff: time.Duration(r.MinRetryBackoffMS) * time.Millisecond,
			MaxRetryBackoff: time.Duration(r.MaxRetryBackoffMS) * time.Millisecond,
			DialTimeout:     time.Duration(r.DialTimeoutMS) * time.Millisecond,
			ReadTimeout:     time.Duration(r.ReadTimeoutMS) * time.Millisecond,
			WriteTimeout:    time.Duration(r.WriteTimeoutMS) * time.Millisecond,
		})

		if err := client.Ping(context.Background()).Err(); err != nil {
			if closeErr := client.Close(); closeErr != nil {
				logger.Fatal("Failed to connect to Redis runtime store; also failed to close client",
					log.Error(err), log.String("closeError", closeErr.Error()))
			}
			logger.Fatal("Failed to connect to Redis runtime store", log.Error(err))
		}

		logger.Info("Connected to Redis runtime store", log.String("address", r.Address))
		redisInstance = &redisProvider{
			client:    client,
			keyPrefix: r.KeyPrefix,
		}
	})
}

// GetRedisProvider returns the singleton Redis provider.
func GetRedisProvider() RedisProviderInterface {
	initRedisProvider()
	return redisInstance
}

// GetRedisProviderCloser returns the Redis provider with closing capability.
func GetRedisProviderCloser() RedisProviderCloser {
	initRedisProvider()
	return redisInstance
}

// GetRedisClient returns the underlying Redis client.
func (r *redisProvider) GetRedisClient() *redis.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client
}

// GetKeyPrefix returns the key prefix for namespacing Redis keys.
func (r *redisProvider) GetKeyPrefix() string {
	return r.keyPrefix
}

// Close closes the Redis connection. Called by the lifecycle manager on shutdown.
func (r *redisProvider) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		if err := r.client.Close(); err != nil {
			return err
		}
		r.client = nil
	}
	return nil
}

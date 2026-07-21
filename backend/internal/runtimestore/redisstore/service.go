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
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// redisClient is the minimal Redis API needed by redisStore.
type redisClient interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	SetArgs(ctx context.Context, key string, value any, a redis.SetArgs) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	GetDel(ctx context.Context, key string) *redis.StringCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
}

// keyFormat is the format string used to build Redis store keys.
const keyFormat = "%s:runtime:%s:%s:%s"

// redisStore implements the RuntimeStoreProvider interface using Redis as the backend.
type redisStore struct {
	keyPrefix    string
	deploymentID string
	client       redisClient
	logger       *log.Logger
}

func newRedisStore(deploymentID string) providers.RuntimeStoreProvider {
	p := dbprovider.GetRedisProvider()
	return &redisStore{
		keyPrefix:    p.GetKeyPrefix(),
		deploymentID: deploymentID,
		client:       p.GetRedisClient(),
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RedisStore")),
	}
}

// Put stores a value in the Redis store with the specified TTL.
func (r *redisStore) Put(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string, value []byte, ttlSeconds int64) error {
	ttl := time.Duration(0)
	if ttlSeconds > 0 {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	if err := r.client.Set(ctx, r.getFormattedKey(namespace, key), value, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store in Redis: %w", err)
	}

	r.logger.Debug(ctx, "Stored in Redis", log.String("key", key))
	return nil
}

// Get retrieves a value from the Redis store by its key.
func (r *redisStore) Get(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string) ([]byte, error) {
	data, err := r.client.Get(ctx, r.getFormattedKey(namespace, key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get data from Redis: %w", err)
	}
	return data, nil
}

// Update updates the value associated with a key in the Redis store, preserving its TTL.
func (r *redisStore) Update(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string, value []byte) error {
	formattedKey := r.getFormattedKey(namespace, key)
	err := r.client.SetArgs(ctx, formattedKey, value, redis.SetArgs{
		Mode:    "XX",
		KeepTTL: true,
	}).Err()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return providers.ErrRuntimeStoreKeyNotFound
		}
		return fmt.Errorf("failed to update in Redis: %w", err)
	}
	return nil
}

// Delete removes a value from the Redis store by its key.
func (r *redisStore) Delete(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string) error {
	if err := r.client.Del(ctx, r.getFormattedKey(namespace, key)).Err(); err != nil {
		return fmt.Errorf("failed to delete from Redis: %w", err)
	}
	return nil
}

// Take retrieves and removes a value from the Redis store by its key.
func (r *redisStore) Take(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string) ([]byte, error) {
	data, err := r.client.GetDel(ctx, r.getFormattedKey(namespace, key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to take data from Redis: %w", err)
	}

	r.logger.Debug(ctx, "Taken from Redis", log.String("key", key))
	return data, nil
}

// ExtendTTL extends the TTL of an existing entry in the Redis store.
func (r *redisStore) ExtendTTL(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string, ttlSeconds int64) error {
	formattedKey := r.getFormattedKey(namespace, key)
	ttl := time.Duration(ttlSeconds) * time.Second
	ok, err := r.client.Expire(ctx, formattedKey, ttl).Result()
	if err != nil {
		return fmt.Errorf("failed to extend TTL in Redis: %w", err)
	}
	if !ok {
		return providers.ErrRuntimeStoreKeyNotFound
	}
	return nil
}

// getFormattedKey builds the Redis key.
func (r *redisStore) getFormattedKey(namespace providers.RuntimeStoreNamespace, key string) string {
	return fmt.Sprintf(keyFormat, r.keyPrefix, r.deploymentID, namespace, key)
}

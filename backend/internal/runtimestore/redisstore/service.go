// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	redis.Scripter
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	SetArgs(ctx context.Context, key string, value any, a redis.SetArgs) *redis.StatusCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	GetDel(ctx context.Context, key string) *redis.StringCmd
	Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd
}

// compareFieldAndSwapScript atomically replaces the stored value with a new one only when the
// top-level JSON string field named ARGV[1] equals ARGV[2], preserving the existing TTL.
// Returns 1 on swap, 0 when the key is absent or the field differs.
var compareFieldAndSwapScript = redis.NewScript(`
local val = redis.call('GET', KEYS[1])
if not val then return 0 end
local data = cjson.decode(val)
if data[ARGV[1]] ~= ARGV[2] then return 0 end
redis.call('SET', KEYS[1], ARGV[3], 'KEEPTTL')
return 1
`)

// compareFieldAndSwapBeforeExpiryScript also checks an application-level expiry against Redis time.
var compareFieldAndSwapBeforeExpiryScript = redis.NewScript(`
local val = redis.call('GET', KEYS[1])
if not val then return 0 end
local data = cjson.decode(val)
if data[ARGV[1]] ~= ARGV[2] then return 0 end
local now = redis.call('TIME')
local nowMillis = (tonumber(now[1]) * 1000) + math.floor(tonumber(now[2]) / 1000)
if tonumber(ARGV[3]) <= nowMillis then return 0 end
redis.call('SET', KEYS[1], ARGV[4], 'KEEPTTL')
return 1
`)

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

// PutIfNotExists atomically stores a value only if the key does not already hold a value, using
// Redis's native SET NX.
func (r *redisStore) PutIfNotExists(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string, value []byte, ttlSeconds int64) (bool, error) {
	ttl := time.Duration(0)
	if ttlSeconds > 0 {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	ok, err := r.client.SetArgs(ctx, r.getFormattedKey(namespace, key), value, redis.SetArgs{
		Mode: "NX",
		TTL:  ttl,
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("failed to store in Redis: %w", err)
	}

	r.logger.Debug(ctx, "Stored in Redis", log.String("key", key))
	return ok == "OK", nil
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

// CompareFieldAndSwap replaces the stored value with newValue only when the top-level JSON string
// field of the current value equals expected, preserving the existing TTL.
func (r *redisStore) CompareFieldAndSwap(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key, field, expected string, newValue []byte) (bool, error) {
	n, err := compareFieldAndSwapScript.Run(ctx, r.client,
		[]string{r.getFormattedKey(namespace, key)}, field, expected, newValue).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("failed to compare-and-swap in Redis: %w", err)
	}
	return n == 1, nil
}

// CompareFieldAndSwapBeforeExpiry updates a value only when its JSON field matches and the
// supplied application-level expiry has not passed.
func (r *redisStore) CompareFieldAndSwapBeforeExpiry(ctx context.Context,
	namespace providers.RuntimeStoreNamespace, key, field, expected string, expiresAt time.Time,
	newValue []byte,
) (bool, error) {
	n, err := compareFieldAndSwapBeforeExpiryScript.Run(ctx, r.client,
		[]string{r.getFormattedKey(namespace, key)}, field, expected, expiresAt.UnixMilli(), newValue).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("failed to compare-and-swap before expiry in Redis: %w", err)
	}
	return n == 1, nil
}

// getFormattedKey builds the Redis key.
func (r *redisStore) getFormattedKey(namespace providers.RuntimeStoreNamespace, key string) string {
	return fmt.Sprintf(keyFormat, r.keyPrefix, r.deploymentID, namespace, key)
}

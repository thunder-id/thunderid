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

package authz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// authReqRedisClient abstracts the Redis commands used by the authorization request store.
type authReqRedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

// redisAuthorizationRequestStore is the Redis-backed implementation of authorizationRequestStoreInterface.
type redisAuthorizationRequestStore struct {
	client         authReqRedisClient
	keyPrefix      string
	deploymentID   string
	validityPeriod time.Duration
}

// newRedisAuthorizationRequestStore creates a new Redis-backed authorization request store.
func newRedisAuthorizationRequestStore(p provider.RedisProviderInterface) authorizationRequestStoreInterface {
	return &redisAuthorizationRequestStore{
		client:         p.GetRedisClient(),
		keyPrefix:      p.GetKeyPrefix(),
		deploymentID:   config.GetServerRuntime().Config.Server.Identifier,
		validityPeriod: 10 * time.Minute,
	}
}

// authReqKey builds the Redis key for an authorization request.
func (s *redisAuthorizationRequestStore) authReqKey(key string) string {
	return fmt.Sprintf("%s:runtime:%s:authreq:%s", s.keyPrefix, s.deploymentID, key)
}

// AddRequest adds an authorization request context entry to Redis with a TTL.
func (s *redisAuthorizationRequestStore) AddRequest(ctx context.Context, value authRequestContext) (string, error) {
	key, err := utils.GenerateUUIDv7()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID: %w", err)
	}

	data, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request context: %w", err)
	}

	if err := s.client.Set(ctx, s.authReqKey(key), data, s.validityPeriod).Err(); err != nil {
		return "", fmt.Errorf("failed to store authorization request in Redis: %w", err)
	}

	return key, nil
}

// GetRequest retrieves an authorization request context entry from Redis.
func (s *redisAuthorizationRequestStore) GetRequest(
	ctx context.Context, key string,
) (bool, authRequestContext, error) {
	if key == "" {
		return false, authRequestContext{}, nil
	}

	data, err := s.client.Get(ctx, s.authReqKey(key)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, authRequestContext{}, nil
		}
		return false, authRequestContext{}, fmt.Errorf("failed to get authorization request from Redis: %w", err)
	}

	var result authRequestContext
	if err := json.Unmarshal(data, &result); err != nil {
		return false, authRequestContext{}, fmt.Errorf("failed to unmarshal authorization request: %w", err)
	}

	return true, result, nil
}

// ClearRequest removes a specific authorization request context entry from Redis.
func (s *redisAuthorizationRequestStore) ClearRequest(ctx context.Context, key string) error {
	if key == "" {
		return nil
	}

	if err := s.client.Del(ctx, s.authReqKey(key)).Err(); err != nil {
		return fmt.Errorf("failed to delete authorization request from Redis: %w", err)
	}

	return nil
}

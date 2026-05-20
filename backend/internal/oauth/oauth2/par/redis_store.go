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
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// parRedisClient abstracts the Redis commands used by the PAR store.
type parRedisClient interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	GetDel(ctx context.Context, key string) *redis.StringCmd
}

// redisPARRequestStore is the Redis-backed implementation of parStoreInterface.
type redisPARRequestStore struct {
	client       parRedisClient
	keyPrefix    string
	deploymentID string
}

// newRedisPARRequestStore creates a new Redis-backed PAR request store.
func newRedisPARRequestStore(
	p provider.RedisProviderInterface, deploymentID string,
) parStoreInterface {
	return &redisPARRequestStore{
		client:       p.GetRedisClient(),
		keyPrefix:    p.GetKeyPrefix(),
		deploymentID: deploymentID,
	}
}

// parKey builds the Redis key for a PAR random key.
func (s *redisPARRequestStore) parKey(randomKey string) string {
	return fmt.Sprintf("%s:runtime:%s:par:%s", s.keyPrefix, s.deploymentID, randomKey)
}

// Store persists a pushed authorization request in Redis with a TTL.
func (s *redisPARRequestStore) Store(
	ctx context.Context, request pushedAuthorizationRequest, expirySeconds int64,
) (string, error) {
	randomKey, err := generateRandomKey()
	if err != nil {
		return "", fmt.Errorf("failed to generate request URI: %w", err)
	}

	data, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal PAR request: %w", err)
	}

	ttl := time.Duration(expirySeconds) * time.Second
	if err := s.client.Set(ctx, s.parKey(randomKey), data, ttl).Err(); err != nil {
		return "", fmt.Errorf("failed to store PAR request in Redis: %w", err)
	}

	return randomKey, nil
}

// Consume atomically retrieves and deletes a pushed authorization request via Redis GETDEL.
func (s *redisPARRequestStore) Consume(
	ctx context.Context, randomKey string,
) (pushedAuthorizationRequest, bool, error) {
	data, err := s.client.GetDel(ctx, s.parKey(randomKey)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return pushedAuthorizationRequest{}, false, nil
		}
		return pushedAuthorizationRequest{}, false, fmt.Errorf("failed to get PAR request from Redis: %w", err)
	}

	var request pushedAuthorizationRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return pushedAuthorizationRequest{}, false, fmt.Errorf("failed to unmarshal PAR request: %w", err)
	}
	return request, true, nil
}

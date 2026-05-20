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

package passkey

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// redisClient abstracts the Redis commands used by the passkey session store.
type redisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

// redisSessionStore is the Redis-backed implementation of sessionStoreInterface.
type redisSessionStore struct {
	client       redisClient
	keyPrefix    string
	deploymentID string
}

// newRedisSessionStore creates a new Redis-backed passkey session store.
func newRedisSessionStore(p provider.RedisProviderInterface) sessionStoreInterface {
	return &redisSessionStore{
		client:       p.GetRedisClient(),
		keyPrefix:    p.GetKeyPrefix(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// sessionKey builds the Redis key for a passkey session.
func (s *redisSessionStore) sessionKey(key string) string {
	return fmt.Sprintf("%s:runtime:%s:passkey:%s", s.keyPrefix, s.deploymentID, key)
}

// storeSession serializes the WebAuthn session data and stores it in Redis with a TTL.
func (s *redisSessionStore) storeSession(sessionKey string, session *sessionData, expirySeconds int64) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal passkey session: %w", err)
	}

	ttl := time.Duration(expirySeconds) * time.Second
	if err := s.client.Set(context.Background(), s.sessionKey(sessionKey), data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store passkey session in Redis: %w", err)
	}

	return nil
}

// retrieveSession retrieves the WebAuthn session data from Redis.
func (s *redisSessionStore) retrieveSession(sessionKey string) (*sessionData, error) {
	if sessionKey == "" {
		return nil, nil
	}

	data, err := s.client.Get(context.Background(), s.sessionKey(sessionKey)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get passkey session from Redis: %w", err)
	}

	var result sessionData
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal passkey session: %w", err)
	}

	return &result, nil
}

// deleteSession removes the passkey session from Redis.
func (s *redisSessionStore) deleteSession(sessionKey string) error {
	if sessionKey == "" {
		return nil
	}

	if err := s.client.Del(context.Background(), s.sessionKey(sessionKey)).Err(); err != nil {
		return fmt.Errorf("failed to delete passkey session from Redis: %w", err)
	}

	return nil
}

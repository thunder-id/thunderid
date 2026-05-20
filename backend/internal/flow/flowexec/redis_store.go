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

package flowexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
)

// updateFlowScript atomically updates a flow context preserving its TTL.
// Returns 1 on success, 0 if the key does not exist.
var updateFlowScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then return 0 end
redis.call('SET', KEYS[1], ARGV[1], 'KEEPTTL')
return 1
`)

// redisClient abstracts the Redis commands used by the flow store.
type redisClient interface {
	redis.Scripter
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

// redisFlowStore is the Redis-backed implementation of flowStoreInterface.
type redisFlowStore struct {
	client       redisClient
	keyPrefix    string
	deploymentID string
}

// newRedisFlowStore creates a new Redis-backed flow store.
func newRedisFlowStore(p provider.RedisProviderInterface) flowStoreInterface {
	return &redisFlowStore{
		client:       p.GetRedisClient(),
		keyPrefix:    p.GetKeyPrefix(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// flowKey builds the Redis key for a flow context.
func (s *redisFlowStore) flowKey(executionID string) string {
	return fmt.Sprintf("%s:runtime:%s:flow:%s", s.keyPrefix, s.deploymentID, executionID)
}

// StoreFlowContext stores the flow context in Redis with a TTL.
func (s *redisFlowStore) StoreFlowContext(ctx context.Context, dbModel FlowContextDB, expirySeconds int64) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "RedisFlowStore"))

	data, err := json.Marshal(dbModel)
	if err != nil {
		return fmt.Errorf("failed to marshal flow context: %w", err)
	}

	ttl := time.Duration(expirySeconds) * time.Second
	if err := s.client.Set(ctx, s.flowKey(dbModel.ExecutionID), data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store flow context in Redis: %w", err)
	}

	logger.Debug("Stored flow context in Redis", log.String("executionID", dbModel.ExecutionID))
	return nil
}

// GetFlowContext retrieves the flow context from Redis.
func (s *redisFlowStore) GetFlowContext(ctx context.Context, executionID string) (*FlowContextDB, error) {
	data, err := s.client.Get(ctx, s.flowKey(executionID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get flow context from Redis: %w", err)
	}

	var result FlowContextDB
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal flow context: %w", err)
	}

	return &result, nil
}

// UpdateFlowContext updates the stored flow context, preserving the remaining TTL.
func (s *redisFlowStore) UpdateFlowContext(ctx context.Context, dbModel FlowContextDB) error {
	key := s.flowKey(dbModel.ExecutionID)

	data, err := json.Marshal(dbModel)
	if err != nil {
		return fmt.Errorf("failed to marshal flow context: %w", err)
	}

	n, err := updateFlowScript.Run(ctx, s.client, []string{key}, data).Int()
	if err != nil {
		return fmt.Errorf("failed to update flow context in Redis: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("flow context not found for executionID: %s", dbModel.ExecutionID)
	}

	return nil
}

// DeleteFlowContext removes the flow context from Redis.
func (s *redisFlowStore) DeleteFlowContext(ctx context.Context, executionID string) error {
	if err := s.client.Del(ctx, s.flowKey(executionID)).Err(); err != nil {
		return fmt.Errorf("failed to delete flow context from Redis: %w", err)
	}
	return nil
}

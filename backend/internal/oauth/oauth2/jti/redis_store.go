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

package jti

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// redisClient is the minimal Redis API needed by redisStore.
type redisClient interface {
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) *redis.BoolCmd
}

// redisStore persists JTIs in Redis, scoped by deployment and key prefix.
type redisStore struct {
	client       redisClient
	keyPrefix    string
	deploymentID string
}

// newRedisStore builds a Redis-backed JTI store from the given provider.
func newRedisStore(p provider.RedisProviderInterface, deploymentID string) JTIStoreInterface {
	return &redisStore{
		client:       p.GetRedisClient(),
		keyPrefix:    p.GetKeyPrefix(),
		deploymentID: deploymentID,
	}
}

// jtiKey returns the namespaced Redis key for a given JTI.
func (s *redisStore) jtiKey(namespace, jti string) string {
	return fmt.Sprintf("%s:runtime:%s:jti:%s:%s",
		s.keyPrefix, s.deploymentID, namespace, jti)
}

// RecordJTI returns false without error when the key already existed, signaling a replay.
func (s *redisStore) RecordJTI(
	ctx context.Context, namespace, jti string, expiry time.Time,
) (bool, error) {
	ttl := time.Until(expiry)
	if ttl <= 0 {
		return true, nil
	}
	ok, err := s.client.SetNX(ctx, s.jtiKey(namespace, jti), 1, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("failed to record jti in Redis: %w", err)
	}
	return ok, nil
}

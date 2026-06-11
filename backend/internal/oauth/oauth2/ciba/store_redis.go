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

package ciba

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

// markAuthenticatedScript atomically transitions a CIBA auth request from PENDING to AUTHENTICATED,
// setting userID, authorizedScopes, attributeCacheID, completedACR, and authTime in one operation.
// Returns 1 on success, 0 if not found or not in PENDING state.
var markAuthenticatedScript = redis.NewScript(`
local val = redis.call('GET', KEYS[1])
if not val then return 0 end
local data = cjson.decode(val)
if data['State'] ~= ARGV[1] then return 0 end
data['State'] = ARGV[2]
data['UserID'] = ARGV[3]
data['AuthorizedScopes'] = ARGV[4]
data['AttributeCacheID'] = ARGV[5]
data['CompletedACR'] = ARGV[6]
data['AuthTime'] = ARGV[7]
redis.call('SET', KEYS[1], cjson.encode(data), 'KEEPTTL')
return 1
`)

// consumeCIBAScript atomically transitions a CIBA auth request from AUTHENTICATED to CONSUMED.
// Returns 1 on success, 0 if not found or already consumed/in a different state.
var consumeCIBAScript = redis.NewScript(`
local val = redis.call('GET', KEYS[1])
if not val then return 0 end
local data = cjson.decode(val)
if data['State'] ~= ARGV[1] then return 0 end
data['State'] = ARGV[2]
redis.call('SET', KEYS[1], cjson.encode(data), 'KEEPTTL')
return 1
`)

// cibaRedisClient abstracts the Redis commands used by the CIBA request store.
type cibaRedisClient interface {
	redis.Scripter
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
}

// redisCIBARequestStore is the Redis-backed implementation of CIBARequestStoreInterface.
type redisCIBARequestStore struct {
	client       cibaRedisClient
	keyPrefix    string
	deploymentID string
}

// newRedisCIBARequestStore creates a new Redis-backed CIBA request store.
func newRedisCIBARequestStore(p provider.RedisProviderInterface) CIBARequestStoreInterface {
	return &redisCIBARequestStore{
		client:       p.GetRedisClient(),
		keyPrefix:    p.GetKeyPrefix(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// cibaKey builds the Redis key for a CIBA authentication request.
func (s *redisCIBARequestStore) cibaKey(authReqID string) string {
	return fmt.Sprintf("%s:runtime:%s:ciba-auth-req:%s", s.keyPrefix, s.deploymentID, authReqID)
}

// Add inserts a new CIBA authentication request into Redis with a TTL derived from ExpiryTime.
func (s *redisCIBARequestStore) Add(ctx context.Context, request *CIBAAuthRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal CIBA request: %w", err)
	}

	ttl := time.Until(request.ExpiryTime)
	if ttl <= 0 {
		return fmt.Errorf("CIBA request already expired")
	}

	if err := s.client.Set(ctx, s.cibaKey(request.AuthReqID), data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to store CIBA request in Redis: %w", err)
	}

	return nil
}

// GetByID retrieves a CIBA authentication request by ID. Returns ErrCIBARequestNotFound if absent.
func (s *redisCIBARequestStore) GetByID(ctx context.Context, authReqID string) (*CIBAAuthRequest, error) {
	if authReqID == "" {
		return nil, ErrCIBARequestNotFound
	}

	data, err := s.client.Get(ctx, s.cibaKey(authReqID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCIBARequestNotFound
		}
		return nil, fmt.Errorf("failed to get CIBA request from Redis: %w", err)
	}

	var request CIBAAuthRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CIBA request: %w", err)
	}

	return &request, nil
}

// MarkAuthenticated atomically transitions a pending request to authenticated using a Lua script,
// preventing concurrent callbacks from both succeeding on the same request.
func (s *redisCIBARequestStore) MarkAuthenticated(ctx context.Context, authReqID, userID,
	authorizedScopes, attributeCacheID, completedACR string, authTime time.Time) error {
	n, err := markAuthenticatedScript.Run(ctx, s.client, []string{s.cibaKey(authReqID)},
		string(CIBAStatePending), string(CIBAStateAuthenticated),
		userID, authorizedScopes, attributeCacheID, completedACR,
		authTime.UTC().Format(time.RFC3339Nano),
	).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return fmt.Errorf("CIBA request %s not found", authReqID)
		}
		return fmt.Errorf("failed to mark CIBA request as authenticated: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("CIBA request %s is not pending", authReqID)
	}
	return nil
}

// MarkConsumed atomically transitions an authenticated request to consumed using a Lua script,
// preventing double-token issuance under concurrent polls. Returns false if the request is not
// in the AUTHENTICATED state (already consumed or otherwise terminal).
func (s *redisCIBARequestStore) MarkConsumed(ctx context.Context, authReqID string) (bool, error) {
	n, err := consumeCIBAScript.Run(ctx, s.client, []string{s.cibaKey(authReqID)},
		string(CIBAStateAuthenticated), string(CIBAStateConsumed)).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("failed to consume CIBA request: %w", err)
	}
	return n == 1, nil
}

// UpdateLastPolled updates the last polled timestamp of a CIBA authentication request.
func (s *redisCIBARequestStore) UpdateLastPolled(ctx context.Context, authReqID string, polledAt time.Time) error {
	record, err := s.GetByID(ctx, authReqID)
	if err != nil {
		return err
	}

	record.LastPolledAt = polledAt
	return s.save(ctx, record)
}

// UpdateState updates the state of a CIBA authentication request.
func (s *redisCIBARequestStore) UpdateState(ctx context.Context, authReqID string, state CIBARequestState) error {
	record, err := s.GetByID(ctx, authReqID)
	if err != nil {
		return err
	}

	record.State = state
	return s.save(ctx, record)
}

// save serializes and writes the record back to Redis preserving the remaining TTL.
func (s *redisCIBARequestStore) save(ctx context.Context, record *CIBAAuthRequest) error {
	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("failed to marshal CIBA request: %w", err)
	}

	ttl := time.Until(record.ExpiryTime)
	if ttl <= 0 {
		ttl = time.Second
	}

	if err := s.client.Set(ctx, s.cibaKey(record.AuthReqID), data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to update CIBA request in Redis: %w", err)
	}

	return nil
}

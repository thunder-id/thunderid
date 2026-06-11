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
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// newCIBAStore returns a Redis-backed store when the runtime database is configured for Redis,
// and falls back to the SQL-backed store otherwise. This mirrors the selection pattern used
// by the authz package for its authorization request and code stores.
func newCIBAStore() CIBARequestStoreInterface {
	if config.GetServerRuntime().Config.Database.Runtime.Type == provider.DataSourceTypeRedis {
		return newRedisCIBARequestStore(provider.GetRedisProvider())
	}
	return newCIBARequestStore()
}

// CIBARequestStoreInterface defines the interface for CIBA authentication request storage.
type CIBARequestStoreInterface interface {
	Add(ctx context.Context, request *CIBAAuthRequest) error
	GetByID(ctx context.Context, authReqID string) (*CIBAAuthRequest, error)
	MarkAuthenticated(ctx context.Context, authReqID, userID, authorizedScopes, attributeCacheID,
		completedACR string, authTime time.Time) error
	MarkConsumed(ctx context.Context, authReqID string) (bool, error)
	UpdateLastPolled(ctx context.Context, authReqID string, polledAt time.Time) error
	UpdateState(ctx context.Context, authReqID string, state CIBARequestState) error
}

// cibaRequestStore provides the CIBA authentication request store functionality using database.
type cibaRequestStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newCIBARequestStore creates a new instance of cibaRequestStore with injected dependencies.
func newCIBARequestStore() CIBARequestStoreInterface {
	return &cibaRequestStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// Add inserts a new CIBA authentication request into the store.
// UserID is not included at creation — it is populated by MarkAuthenticated once the
// callback verifies the assertion.
func (s *cibaRequestStore) Add(ctx context.Context, request *CIBAAuthRequest) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryInsertCIBAAuthRequest,
		request.AuthReqID, request.ClientID, request.StandardScopes, string(request.State),
		request.ExpiryTime.UTC(), s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to insert CIBA authentication request: %w", err)
	}

	return nil
}

// GetByID retrieves a CIBA authentication request by ID. Returns ErrCIBARequestNotFound if absent.
func (s *cibaRequestStore) GetByID(ctx context.Context, authReqID string) (*CIBAAuthRequest, error) {
	if authReqID == "" {
		return nil, ErrCIBARequestNotFound
	}

	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetCIBAAuthRequest, authReqID, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to query CIBA authentication request: %w", err)
	}

	if len(results) == 0 {
		return nil, ErrCIBARequestNotFound
	}

	request, err := buildCIBAAuthRequestFromRow(results[0])
	if err != nil {
		return nil, fmt.Errorf("failed to build CIBA authentication request: %w", err)
	}

	return request, nil
}

// MarkAuthenticated transitions a pending request to authenticated and records the user ID
// (from the assertion sub claim), attribute cache ID, completed ACR, and authentication time.
// The WHERE STATE = 'PENDING' guard in the query prevents a double-callback race condition.
func (s *cibaRequestStore) MarkAuthenticated(ctx context.Context, authReqID, userID,
	authorizedScopes, attributeCacheID, completedACR string, authTime time.Time) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryMarkCIBAAuthRequestAuthenticated,
		string(CIBAStateAuthenticated), userID, authorizedScopes, attributeCacheID, completedACR,
		authTime.UTC(), authReqID, string(CIBAStatePending), s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to mark CIBA authentication request as authenticated: %w", err)
	}

	return nil
}

// MarkConsumed atomically transitions an authenticated request to consumed. It returns false when
// no row was updated (i.e. the request was already consumed or is not authenticated), enabling
// one-time-use enforcement without a separate read under concurrent polling.
func (s *cibaRequestStore) MarkConsumed(ctx context.Context, authReqID string) (bool, error) {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return false, fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryConsumeCIBAAuthRequest,
		string(CIBAStateConsumed), authReqID, string(CIBAStateAuthenticated), s.deploymentID)
	if err != nil {
		return false, fmt.Errorf("failed to consume CIBA authentication request: %w", err)
	}

	return rowsAffected > 0, nil
}

// UpdateLastPolled updates the last polled timestamp of a CIBA authentication request.
func (s *cibaRequestStore) UpdateLastPolled(ctx context.Context, authReqID string, polledAt time.Time) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryUpdateCIBALastPolled, polledAt.UTC(), authReqID, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to update CIBA last polled time: %w", err)
	}

	return nil
}

// UpdateState updates the state of a CIBA authentication request.
func (s *cibaRequestStore) UpdateState(ctx context.Context, authReqID string, state CIBARequestState) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryUpdateCIBAAuthRequestState, string(state), authReqID,
		s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to update CIBA authentication request state: %w", err)
	}

	return nil
}

// buildCIBAAuthRequestFromRow builds a CIBAAuthRequest from a database result row.
func buildCIBAAuthRequestFromRow(row map[string]interface{}) (*CIBAAuthRequest, error) {
	request := &CIBAAuthRequest{
		AuthReqID:        stringFromRow(row[dbColumnAuthReqID]),
		ClientID:         stringFromRow(row[dbColumnClientID]),
		UserID:           stringFromRow(row[dbColumnUserID]),
		StandardScopes:   stringFromRow(row[dbColumnStandardScopes]),
		AuthorizedScopes: stringFromRow(row[dbColumnAuthorizedScopes]),
		State:            CIBARequestState(stringFromRow(row[dbColumnState])),
		AttributeCacheID: stringFromRow(row[dbColumnAttributeCacheID]),
		CompletedACR:     stringFromRow(row[dbColumnCompletedACR]),
	}

	expiryTime, err := parseTimeField(row[dbColumnExpiryTime], dbColumnExpiryTime)
	if err != nil {
		return nil, err
	}
	request.ExpiryTime = expiryTime

	if authTime, ok := parseOptionalTimeField(row[dbColumnAuthTime]); ok {
		request.AuthTime = authTime
	}
	if lastPolled, ok := parseOptionalTimeField(row[dbColumnLastPolledAt]); ok {
		request.LastPolledAt = lastPolled
	}

	return request, nil
}

// stringFromRow extracts a string value from a database row column, handling both string and []byte.
func stringFromRow(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// parseOptionalTimeField parses a nullable time column, returning ok=false when the value is absent.
func parseOptionalTimeField(value interface{}) (time.Time, bool) {
	if value == nil {
		return time.Time{}, false
	}
	parsed, err := parseTimeField(value, "time")
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

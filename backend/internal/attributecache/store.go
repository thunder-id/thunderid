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

package attributecache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	dbprovider "github.com/thunder-id/thunderid/internal/system/database/provider"
)

// attributeCacheStoreInterface defines the interface for the attribute cache store.
type attributeCacheStoreInterface interface {
	// CreateAttributeCache creates a new attribute cache entry in the store.
	CreateAttributeCache(ctx context.Context, cache AttributeCache) error

	// GetAttributeCache retrieves an attribute cache entry by ID from the store.
	GetAttributeCache(ctx context.Context, id string) (AttributeCache, error)

	// ExtendAttributeCacheTTL extends the TTL of an attribute cache entry in the store.
	ExtendAttributeCacheTTL(ctx context.Context, id string, ttlSeconds int) error

	// DeleteAttributeCache deletes an attribute cache entry by ID from the store.
	DeleteAttributeCache(ctx context.Context, id string) error
}

// attributeCacheStore is the SQL implementation of attributeCacheStoreInterface.
type attributeCacheStore struct {
	dbProvider   dbprovider.DBProviderInterface
	deploymentID string
}

// newAttributeCacheStore creates a new instance of attributeCacheStore.
func newAttributeCacheStore() attributeCacheStoreInterface {
	return &attributeCacheStore{
		dbProvider:   dbprovider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// CreateAttributeCache creates a new attribute cache entry in the database.
func (s *attributeCacheStore) CreateAttributeCache(ctx context.Context, cache AttributeCache) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	attributesJSON, err := json.Marshal(cache.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}

	// Calculate expiry time from TTL
	expiryTime := time.Now().Add(time.Duration(cache.TTLSeconds) * time.Second)

	rows, err := dbClient.ExecuteContext(ctx, queryInsertAttributeCache,
		cache.ID, string(attributesJSON), expiryTime, time.Now(), s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to insert attribute cache: %w", err)
	}
	if rows == 0 {
		return errors.New("no rows affected, attribute cache creation failed")
	}

	return nil
}

// GetAttributeCache retrieves an attribute cache entry by ID from the database.
func (s *attributeCacheStore) GetAttributeCache(ctx context.Context, id string) (AttributeCache, error) {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return AttributeCache{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetAttributeCache, id, s.deploymentID)
	if err != nil {
		return AttributeCache{}, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return AttributeCache{}, errAttributeCacheNotFound
	}
	if len(results) > 1 {
		return AttributeCache{}, errors.New("multiple attribute cache entries found")
	}

	cache, err := s.buildAttributeCacheFromResultRow(results[0])
	if err != nil {
		return AttributeCache{}, fmt.Errorf("failed to build attribute cache from result row: %w", err)
	}

	return cache, nil
}

// ExtendAttributeCacheTTL extends the TTL of an attribute cache entry in the database.
func (s *attributeCacheStore) ExtendAttributeCacheTTL(ctx context.Context, id string, ttlSeconds int) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	// Calculate expiry time from TTL
	expiryTime := time.Now().Add(time.Duration(ttlSeconds) * time.Second)

	rows, err := dbClient.ExecuteContext(ctx, queryUpdateAttributeCacheExpiry,
		id, expiryTime, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to update attribute cache expiry: %w", err)
	}
	if rows == 0 {
		return errAttributeCacheNotFound
	}

	return nil
}

// DeleteAttributeCache deletes an attribute cache entry by ID from the database.
func (s *attributeCacheStore) DeleteAttributeCache(ctx context.Context, id string) error {
	dbClient, err := s.dbProvider.GetRuntimeDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rows, err := dbClient.ExecuteContext(ctx, queryDeleteAttributeCache, id, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete attribute cache: %w", err)
	}
	if rows == 0 {
		return errAttributeCacheNotFound
	}

	return nil
}

// buildAttributeCacheFromResultRow builds an AttributeCache object from a database result row.
func (s *attributeCacheStore) buildAttributeCacheFromResultRow(row map[string]interface{}) (AttributeCache, error) {
	id, ok := row["id"].(string)
	if !ok {
		return AttributeCache{}, errors.New("failed to parse id as string")
	}

	var attributesStr string
	switch v := row["attributes"].(type) {
	case string:
		attributesStr = v
	case []byte:
		attributesStr = string(v)
	default:
		return AttributeCache{}, errors.New("failed to parse attributes: expected string or []byte")
	}

	var attributes map[string]interface{}
	if err := json.Unmarshal([]byte(attributesStr), &attributes); err != nil {
		return AttributeCache{}, fmt.Errorf("failed to unmarshal attributes: %w", err)
	}

	expiryTime, err := parseTimeField(row["expiry_time"], "expiry_time")
	if err != nil {
		return AttributeCache{}, err
	}

	// Calculate remaining TTL from expiry time
	ttlSeconds := int(time.Until(expiryTime).Seconds())
	if ttlSeconds < 0 {
		ttlSeconds = 0
	}

	return AttributeCache{
		ID:         id,
		Attributes: attributes,
		TTLSeconds: ttlSeconds,
	}, nil
}

// parseTimeField parses a time field from the database result.
func parseTimeField(field interface{}, fieldName string) (time.Time, error) {
	const customTimeFormat = "2006-01-02 15:04:05.999999999"

	switch v := field.(type) {
	case string:
		// Handle SQLite datetime strings
		trimmedTime := trimTimeString(v)
		parsedTime, err := time.Parse(customTimeFormat, trimmedTime)
		if err != nil {
			// Try alternative ISO 8601 format as fallback
			parsedTime, err = time.Parse("2006-01-02T15:04:05Z07:00", v)
			if err != nil {
				return time.Time{}, fmt.Errorf("error parsing %s: %w", fieldName, err)
			}
		}
		return parsedTime, nil
	case time.Time:
		return v, nil
	default:
		return time.Time{}, fmt.Errorf("unexpected type for %s", fieldName)
	}
}

// trimTimeString trims extra information from a time string to match the expected format.
func trimTimeString(timeStr string) string {
	parts := strings.SplitN(timeStr, " ", 3)
	if len(parts) >= 2 {
		return parts[0] + " " + parts[1]
	}
	return timeStr
}

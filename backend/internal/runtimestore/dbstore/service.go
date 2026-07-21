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

package dbstore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// dbStore implements the RuntimeStoreProvider interface using the database as the backend.
type dbStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
	logger       *log.Logger
}

func newDBStore(dbProvider provider.DBProviderInterface, deploymentID string) providers.RuntimeStoreProvider {
	return &dbStore{
		dbProvider:   dbProvider,
		deploymentID: deploymentID,
		logger:       log.GetLogger().With(log.String(log.LoggerKeyComponentName, "DBStore")),
	}
}

// Put stores a value in the database runtime store with the specified TTL.
// A non-positive ttlSeconds stores the entry without expiry. Existing entries are overwritten.
func (d *dbStore) Put(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string, value []byte, ttlSeconds int64) error {
	dbClient, err := d.dbProvider.GetRuntimeTransientDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	var expiryTime interface{}
	if ttlSeconds > 0 {
		expiryTime = time.Now().UTC().Add(time.Duration(ttlSeconds) * time.Second)
	}

	if _, err := dbClient.ExecuteContext(
		ctx, queryPutRuntimeStore, d.deploymentID, string(namespace), key, value, expiryTime,
	); err != nil {
		return fmt.Errorf("failed to store in database: %w", err)
	}

	d.logger.Debug(ctx, "Stored in database", log.String("key", key))
	return nil
}

// Get retrieves a value from the database runtime store by its key.
// Returns (nil, nil) when the key is missing or expired.
func (d *dbStore) Get(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string) ([]byte, error) {
	dbClient, err := d.dbProvider.GetRuntimeTransientDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(
		ctx, queryGetRuntimeStore, d.deploymentID, string(namespace), key, time.Now().UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get data from database: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	return parseStoreValue(results[0])
}

// Update updates the value associated with an existing key, preserving its TTL.
// Returns an error when the key is missing or expired.
func (d *dbStore) Update(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string, value []byte) error {
	dbClient, err := d.dbProvider.GetRuntimeTransientDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(
		ctx, queryUpdateRuntimeStore, d.deploymentID, string(namespace), key, value, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed to update in database: %w", err)
	}
	if rowsAffected == 0 {
		return providers.ErrRuntimeStoreKeyNotFound
	}
	return nil
}

// Delete removes a value from the database runtime store by its key. It is idempotent.
func (d *dbStore) Delete(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string) error {
	dbClient, err := d.dbProvider.GetRuntimeTransientDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	if _, err := dbClient.ExecuteContext(
		ctx, queryDeleteRuntimeStore, d.deploymentID, string(namespace), key,
	); err != nil {
		return fmt.Errorf("failed to delete from database: %w", err)
	}
	return nil
}

// Take retrieves and removes a value from the database runtime store by its key.
// The fetch and delete run as a single atomic statement, so a concurrent caller cannot
// consume the same value twice. Returns (nil, nil) when the key is missing or expired.
func (d *dbStore) Take(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string) ([]byte, error) {
	dbClient, err := d.dbProvider.GetRuntimeTransientDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(
		ctx, queryTakeRuntimeStore, d.deploymentID, string(namespace), key, time.Now().UTC(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to take data from database: %w", err)
	}
	if len(results) == 0 {
		return nil, nil
	}

	d.logger.Debug(ctx, "Taken from database", log.String("key", key))
	return parseStoreValue(results[0])
}

// ExtendTTL extends the TTL of an existing, non-expired entry in the database runtime store.
func (d *dbStore) ExtendTTL(ctx context.Context, namespace providers.RuntimeStoreNamespace,
	key string, ttlSeconds int64) error {
	if ttlSeconds <= 0 {
		return fmt.Errorf("ttl seconds cannot be negative or zero: %d", ttlSeconds)
	}

	dbClient, err := d.dbProvider.GetRuntimeTransientDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	expiryTime := time.Now().UTC().Add(time.Duration(ttlSeconds) * time.Second)

	rowsAffected, err := dbClient.ExecuteContext(
		ctx, queryExtendTTLRuntimeStore, d.deploymentID, string(namespace), key, expiryTime, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("failed to extend TTL in database: %w", err)
	}
	if rowsAffected == 0 {
		return providers.ErrRuntimeStoreKeyNotFound
	}
	return nil
}

// parseStoreValue extracts the VALUE column from a result row, handling both string and []byte.
func parseStoreValue(row map[string]interface{}) ([]byte, error) {
	switch v := row[columnNameValue].(type) {
	case string:
		return []byte(v), nil
	case []byte:
		return v, nil
	default:
		return nil, errors.New("value is missing or of unexpected type")
	}
}

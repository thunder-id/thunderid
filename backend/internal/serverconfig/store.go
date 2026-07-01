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

package serverconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// serverConfigStoreInterface is the unified store contract. A read returns the section's layers; each
// backing store fills the layers it owns (db → writable, file → readOnly, composite → both), and the
// service derives the merged value. Writes target the writable layer; the file store rejects them.
type serverConfigStoreInterface interface {
	GetServerConfig(ctx context.Context, name ConfigName) (storeLayers, error)
	UpsertServerConfig(ctx context.Context, cfg ServerConfig) error
}

// serverConfigStore is the database-backed (writable) store.
type serverConfigStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newServerConfigStore creates a new instance of serverConfigStore.
func newServerConfigStore() serverConfigStoreInterface {
	return &serverConfigStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// getDBClient is a helper method to get the database client.
func (s *serverConfigStore) getDBClient() (provider.DBClientInterface, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}
	return dbClient, nil
}

// GetServerConfig returns the writable layer for a section, or an empty result when it is unset.
func (s *serverConfigStore) GetServerConfig(ctx context.Context, name ConfigName) (storeLayers, error) {
	dbClient, err := s.getDBClient()
	if err != nil {
		return storeLayers{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetServerConfigByName, string(name), s.deploymentID)
	if err != nil {
		return storeLayers{}, fmt.Errorf("failed to get server config: %w", err)
	}
	if len(results) == 0 {
		return storeLayers{}, nil
	}

	value, err := valueFromRow(results[0])
	if err != nil {
		return storeLayers{}, err
	}
	return storeLayers{Writable: value}, nil
}

// UpsertServerConfig inserts or updates a single server config in the writable layer.
func (s *serverConfigStore) UpsertServerConfig(ctx context.Context, cfg ServerConfig) error {
	dbClient, err := s.getDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.ExecuteContext(ctx, queryUpsertServerConfig, string(cfg.Name), string(cfg.Value),
		s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to upsert server config: %w", err)
	}
	return nil
}

// valueFromRow extracts the JSON value column from a database result row.
func valueFromRow(row map[string]interface{}) (json.RawMessage, error) {
	switch v := row["value"].(type) {
	case string:
		return json.RawMessage(v), nil
	case []byte:
		return json.RawMessage(v), nil
	default:
		return nil, fmt.Errorf("failed to parse value")
	}
}

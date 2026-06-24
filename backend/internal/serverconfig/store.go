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
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

// serverConfigStoreInterface defines the interface for server config store operations.
type serverConfigStoreInterface interface {
	GetServerConfigByName(ctx context.Context, name ConfigName) (*ServerConfig, error)
	GetServerConfigList(ctx context.Context) ([]ServerConfig, error)
	UpsertServerConfig(ctx context.Context, cfg ServerConfig) error
	UpsertServerConfigs(ctx context.Context, configs []ServerConfig) error
	DeleteServerConfig(ctx context.Context, name ConfigName) error
}

// serverConfigStore is the default implementation of serverConfigStoreInterface.
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

// GetServerConfigByName retrieves a single server config by name, or nil if it does not exist.
func (s *serverConfigStore) GetServerConfigByName(ctx context.Context, name ConfigName) (*ServerConfig, error) {
	dbClient, err := s.getDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetServerConfigByName, string(name), s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server config: %w", err)
	}

	if len(results) == 0 {
		return nil, nil
	}
	return buildServerConfigFromRow(results[0])
}

// GetServerConfigList retrieves all server configs for the deployment.
func (s *serverConfigStore) GetServerConfigList(ctx context.Context) ([]ServerConfig, error) {
	dbClient, err := s.getDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryListServerConfigs, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to list server configs: %w", err)
	}

	configs := make([]ServerConfig, 0, len(results))
	for _, row := range results {
		cfg, err := buildServerConfigFromRow(row)
		if err != nil {
			return nil, err
		}
		configs = append(configs, *cfg)
	}
	return configs, nil
}

// UpsertServerConfig inserts or updates a single server config.
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

// UpsertServerConfigs inserts or updates multiple server configs in a single transaction.
func (s *serverConfigStore) UpsertServerConfigs(ctx context.Context, configs []ServerConfig) error {
	dbClient, err := s.getDBClient()
	if err != nil {
		return err
	}

	tx, err := dbClient.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	for _, cfg := range configs {
		if _, err = tx.Exec(queryUpsertServerConfig, string(cfg.Name), string(cfg.Value),
			s.deploymentID); err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				err = errors.Join(err, fmt.Errorf("failed to rollback transaction: %w", rollbackErr))
			}
			return fmt.Errorf("failed to upsert server config: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}
	return nil
}

// DeleteServerConfig removes a server config by name.
func (s *serverConfigStore) DeleteServerConfig(ctx context.Context, name ConfigName) error {
	dbClient, err := s.getDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.ExecuteContext(ctx, queryDeleteServerConfig, string(name), s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete server config: %w", err)
	}
	return nil
}

// buildServerConfigFromRow constructs a ServerConfig from a database result row.
func buildServerConfigFromRow(row map[string]interface{}) (*ServerConfig, error) {
	name, ok := row["name"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse name")
	}

	var value string
	switch v := row["value"].(type) {
	case string:
		value = v
	case []byte:
		value = string(v)
	default:
		return nil, fmt.Errorf("failed to parse value")
	}

	return &ServerConfig{
		Name:  ConfigName(name),
		Value: json.RawMessage(value),
	}, nil
}

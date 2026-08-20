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

package environmentvariable

import (
	"context"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// environmentVariableStoreInterface defines persistence operations for environment variables.
type environmentVariableStoreInterface interface {
	CreateEnvironmentVariable(ctx context.Context, envID string, ev EnvironmentVariable) error
	GetEnvironmentVariableCount(ctx context.Context, envID string) (int, error)
	GetEnvironmentVariableList(ctx context.Context, envID string, limit,
		offset int) ([]EnvironmentVariable, error)
	GetEnvironmentVariableByID(ctx context.Context, envID, id string) (EnvironmentVariable, error)
	GetEnvironmentVariableByKey(ctx context.Context, envID, key string) (EnvironmentVariable, error)
	UpdateEnvironmentVariableByID(ctx context.Context, envID, id, description, value string) error
	DeleteEnvironmentVariableByID(ctx context.Context, envID, id string) error
	GetEnvironmentVariableValues(ctx context.Context, envID string) (map[string]string, error)
}

// environmentVariableStore is the default DB-backed implementation of
// environmentVariableStoreInterface.
type environmentVariableStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newEnvironmentVariableStore creates a new environmentVariableStore bound to the environment
// database, which it shares with the environment manager the variables are resolved for.
func newEnvironmentVariableStore() environmentVariableStoreInterface {
	return &environmentVariableStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// CreateEnvironmentVariable inserts a new environment variable row.
func (s *environmentVariableStore) CreateEnvironmentVariable(ctx context.Context, envID string,
	ev EnvironmentVariable) error {
	dbClient, err := s.dbProvider.GetEnvironmentDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.QueryContext(ctx, queryCreateEnvironmentVariable,
		ev.ID, envID, ev.Key, ev.Value, ev.Description, deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return fmt.Errorf("failed to create environment variable: %w", err)
	}
	return nil
}

// GetEnvironmentVariableCount returns the total number of environment variables for the deployment.
func (s *environmentVariableStore) GetEnvironmentVariableCount(ctx context.Context,
	envID string) (int, error) {
	dbClient, err := s.dbProvider.GetEnvironmentDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetEnvironmentVariableCount,
		envID, deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	if len(results) > 0 {
		if count, ok := results[0]["total"].(int64); ok {
			return int(count), nil
		}
		return 0, fmt.Errorf("failed to parse count result")
	}
	return 0, nil
}

// GetEnvironmentVariableList returns a paginated list of environment variables.
func (s *environmentVariableStore) GetEnvironmentVariableList(ctx context.Context, envID string,
	limit, offset int) ([]EnvironmentVariable, error) {
	dbClient, err := s.dbProvider.GetEnvironmentDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetEnvironmentVariableList,
		limit, offset, envID, deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	variables := make([]EnvironmentVariable, 0, len(results))
	for _, row := range results {
		variables = append(variables, parseEnvironmentVariable(row))
	}
	return variables, nil
}

// GetEnvironmentVariableByID returns a single environment variable by id.
func (s *environmentVariableStore) GetEnvironmentVariableByID(ctx context.Context,
	envID, id string) (EnvironmentVariable, error) {
	dbClient, err := s.dbProvider.GetEnvironmentDBClient()
	if err != nil {
		return EnvironmentVariable{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetEnvironmentVariableByID, id, envID,
		deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return EnvironmentVariable{}, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return EnvironmentVariable{}, errEnvironmentVariableNotFound
	}
	return parseEnvironmentVariable(results[0]), nil
}

// GetEnvironmentVariableByKey returns a single environment variable by key.
func (s *environmentVariableStore) GetEnvironmentVariableByKey(ctx context.Context,
	envID, key string) (EnvironmentVariable, error) {
	dbClient, err := s.dbProvider.GetEnvironmentDBClient()
	if err != nil {
		return EnvironmentVariable{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetEnvironmentVariableByKey, key, envID,
		deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return EnvironmentVariable{}, fmt.Errorf("failed to execute query: %w", err)
	}
	if len(results) == 0 {
		return EnvironmentVariable{}, errEnvironmentVariableNotFound
	}
	return parseEnvironmentVariable(results[0]), nil
}

// UpdateEnvironmentVariableByID updates an environment variable's description and value.
func (s *environmentVariableStore) UpdateEnvironmentVariableByID(ctx context.Context, envID, id,
	description, value string) error {
	dbClient, err := s.dbProvider.GetEnvironmentDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryUpdateEnvironmentVariableByID,
		description, value, id, envID, deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return fmt.Errorf("failed to update environment variable: %w", err)
	}
	if rowsAffected == 0 {
		return errEnvironmentVariableNotFound
	}
	return nil
}

// DeleteEnvironmentVariableByID deletes an environment variable by id.
func (s *environmentVariableStore) DeleteEnvironmentVariableByID(ctx context.Context,
	envID, id string) error {
	dbClient, err := s.dbProvider.GetEnvironmentDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryDeleteEnvironmentVariableByID, id, envID,
		deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return fmt.Errorf("failed to delete environment variable: %w", err)
	}
	if rowsAffected == 0 {
		return errEnvironmentVariableNotFound
	}
	return nil
}

// GetEnvironmentVariableValues returns every environment variable key mapped to its value for one
// environment.
func (s *environmentVariableStore) GetEnvironmentVariableValues(ctx context.Context,
	envID string) (map[string]string, error) {
	dbClient, err := s.dbProvider.GetEnvironmentDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetEnvironmentVariableValues,
		envID, deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	values := make(map[string]string, len(results))
	for _, row := range results {
		values[parseString(row["key"])] = parseString(row["value"])
	}
	return values, nil
}

// parseEnvironmentVariable maps a database row to an EnvironmentVariable.
func parseEnvironmentVariable(row map[string]interface{}) EnvironmentVariable {
	return EnvironmentVariable{
		ID:          parseString(row["id"]),
		Key:         parseString(row["key"]),
		Value:       parseString(row["value"]),
		Description: parseString(row["description"]),
		CreatedAt:   parseTimeString(row["created_at"]),
		UpdatedAt:   parseTimeString(row["updated_at"]),
	}
}

// parseString coerces a nullable text column value to a string.
func parseString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return ""
	}
}

// parseTimeString coerces a timestamp column value (string on SQLite, time.Time on PostgreSQL) to a
// string.
func parseTimeString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case time.Time:
		return v.UTC().Format(time.RFC3339)
	default:
		return ""
	}
}

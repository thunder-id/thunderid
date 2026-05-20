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

package layoutmgt

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
)

var errLayoutNotFound = errors.New("layout not found")

// layoutMgtStoreInterface defines the interface for layout management store operations.
type layoutMgtStoreInterface interface {
	GetLayoutListCount() (int, error)
	GetLayoutList(limit, offset int) ([]Layout, error)
	CreateLayout(id string, layout CreateLayoutRequest) error
	GetLayout(id string) (Layout, error)
	IsLayoutExist(id string) (bool, error)
	UpdateLayout(id string, layout UpdateLayoutRequest) error
	DeleteLayout(id string) error
	GetApplicationsCountByLayoutID(id string) (int, error)
	IsLayoutDeclarative(id string) bool
	IsLayoutHandleConflict(handle string, excludeID string) (bool, error)
}

// layoutMgtStore is the default implementation of layoutMgtStoreInterface.
type layoutMgtStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newLayoutMgtStore creates a new instance of layoutMgtStore.
func newLayoutMgtStore() layoutMgtStoreInterface {
	return &layoutMgtStore{
		dbProvider:   provider.GetDBProvider(),
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}
}

// GetLayoutListCount retrieves the total count of layout configurations.
func (s *layoutMgtStore) GetLayoutListCount() (int, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return 0, err
	}

	countResults, err := dbClient.Query(queryGetLayoutListCount, s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	return parseCountResult(countResults)
}

// GetLayoutList retrieves layout configurations with pagination.
func (s *layoutMgtStore) GetLayoutList(limit, offset int) ([]Layout, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.Query(queryGetLayoutList, limit, offset, s.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute layout list query: %w", err)
	}

	layouts := make([]Layout, 0)
	for _, row := range results {
		layout, err := s.buildLayoutListItemFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build layout from result row: %w", err)
		}
		layouts = append(layouts, layout)
	}

	return layouts, nil
}

// CreateLayout creates a new layout configuration in the database.
func (s *layoutMgtStore) CreateLayout(id string, layout CreateLayoutRequest) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	layoutJSON, err := json.Marshal(layout.Layout)
	if err != nil {
		return fmt.Errorf("failed to marshal layout: %w", err)
	}

	_, err = dbClient.Execute(queryCreateLayout, id, layout.Handle, layout.DisplayName, layout.Description,
		layoutJSON, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// GetLayout retrieves a layout configuration by its id.
func (s *layoutMgtStore) GetLayout(id string) (Layout, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return Layout{}, err
	}

	results, err := dbClient.Query(queryGetLayoutByID, id, s.deploymentID)
	if err != nil {
		return Layout{}, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return Layout{}, errLayoutNotFound
	}

	if len(results) != 1 {
		return Layout{}, fmt.Errorf("unexpected number of results: %d", len(results))
	}

	return s.buildLayoutFromResultRow(results[0])
}

// IsLayoutExist checks if a layout configuration exists by its ID.
func (s *layoutMgtStore) IsLayoutExist(id string) (bool, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return false, err
	}

	results, err := dbClient.Query(queryCheckLayoutExists, id, s.deploymentID)
	if err != nil {
		return false, fmt.Errorf("failed to check layout existence: %w", err)
	}

	if len(results) == 0 {
		return false, nil
	}

	count, err := parseCountResult(results)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// UpdateLayout updates a layout configuration.
func (s *layoutMgtStore) UpdateLayout(id string, layout UpdateLayoutRequest) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	layoutJSON, err := json.Marshal(layout.Layout)
	if err != nil {
		return fmt.Errorf("failed to marshal layout: %w", err)
	}

	_, err = dbClient.Execute(queryUpdateLayout, layout.DisplayName, layout.Description, layoutJSON, id, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// DeleteLayout deletes a layout configuration.
func (s *layoutMgtStore) DeleteLayout(id string) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.Execute(queryDeleteLayout, id, s.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// GetApplicationsCountByLayoutID returns the count of applications using a specific layout.
func (s *layoutMgtStore) GetApplicationsCountByLayoutID(id string) (int, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return 0, err
	}

	results, err := dbClient.Query(queryGetApplicationsCountByLayoutID, id, s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get applications count: %w", err)
	}

	return parseCountResult(results)
}

// IsLayoutDeclarative checks if a layout is immutable (in database store, all layouts are mutable).
func (s *layoutMgtStore) IsLayoutDeclarative(id string) bool {
	return false
}

// getConfigDBClient retrieves the config database client.
func (s *layoutMgtStore) getConfigDBClient() (provider.DBClientInterface, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get config database client: %w", err)
	}
	return dbClient, nil
}

// parseCountResult parses count query results.
func parseCountResult(results []map[string]interface{}) (int, error) {
	if len(results) == 0 {
		return 0, fmt.Errorf("no results returned from count query")
	}

	totalInterface, exists := results[0]["total"]
	if !exists {
		return 0, fmt.Errorf("total field not found in result")
	}

	var total int
	switch v := totalInterface.(type) {
	case int64:
		total = int(v)
	case int:
		total = v
	default:
		return 0, fmt.Errorf("unexpected type for total: %T", totalInterface)
	}

	return total, nil
}

// getTimestamp safely extracts a timestamp value from a database row and formats it as ISO 8601.
func (s *layoutMgtStore) getTimestamp(row map[string]interface{}, key string) (string, error) {
	val := row[key]
	switch v := val.(type) {
	case string:
		return v, nil
	case time.Time:
		// Convert time.Time to RFC3339 format for consistency
		return v.Format(time.RFC3339), nil
	default:
		return "", fmt.Errorf("%s field is missing or invalid", key)
	}
}

// buildLayoutListItemFromResultRow builds a Layout from a database result row (list view).
func (s *layoutMgtStore) buildLayoutListItemFromResultRow(row map[string]interface{}) (Layout, error) {
	id, ok := row["id"].(string)
	if !ok {
		return Layout{}, fmt.Errorf("id not found or invalid type")
	}

	handle := ""
	if h, ok := row["handle"].(string); ok {
		handle = h
	}

	displayName, ok := row["display_name"].(string)
	if !ok {
		return Layout{}, fmt.Errorf("display_name not found or invalid type")
	}

	description := ""
	if descInterface, ok := row["description"]; ok && descInterface != nil {
		description, _ = descInterface.(string)
	}

	createdAt, err := s.getTimestamp(row, "created_at")
	if err != nil {
		return Layout{}, fmt.Errorf("failed to extract created_at: %w", err)
	}

	updatedAt, err := s.getTimestamp(row, "updated_at")
	if err != nil {
		return Layout{}, fmt.Errorf("failed to extract updated_at: %w", err)
	}

	return Layout{
		ID:          id,
		Handle:      handle,
		DisplayName: displayName,
		Description: description,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// buildLayoutFromResultRow builds a Layout from a database result row (detail view).
func (s *layoutMgtStore) buildLayoutFromResultRow(row map[string]interface{}) (Layout, error) {
	id, ok := row["id"].(string)
	if !ok {
		return Layout{}, fmt.Errorf("id not found or invalid type")
	}

	handle := ""
	if h, ok := row["handle"].(string); ok {
		handle = h
	}

	displayName, ok := row["display_name"].(string)
	if !ok {
		return Layout{}, fmt.Errorf("display_name not found or invalid type")
	}

	description := ""
	if descInterface, ok := row["description"]; ok && descInterface != nil {
		description, _ = descInterface.(string)
	}

	layoutInterface, ok := row["layout"]
	if !ok {
		return Layout{}, fmt.Errorf("layout not found")
	}

	var layout json.RawMessage
	switch v := layoutInterface.(type) {
	case string:
		layout = json.RawMessage(v)
	case []byte:
		layout = json.RawMessage(v)
	default:
		return Layout{}, fmt.Errorf("unexpected type for layout: %T", layoutInterface)
	}

	createdAt, err := s.getTimestamp(row, "created_at")
	if err != nil {
		return Layout{}, fmt.Errorf("failed to extract created_at: %w", err)
	}

	updatedAt, err := s.getTimestamp(row, "updated_at")
	if err != nil {
		return Layout{}, fmt.Errorf("failed to extract updated_at: %w", err)
	}

	return Layout{
		ID:          id,
		Handle:      handle,
		DisplayName: displayName,
		Description: description,
		Layout:      layout,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// IsLayoutHandleConflict checks if a layout handle already exists for the deployment, excluding a specific ID.
func (s *layoutMgtStore) IsLayoutHandleConflict(handle string, excludeID string) (bool, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return false, err
	}

	results, err := dbClient.Query(queryCheckLayoutHandleConflict, handle, s.deploymentID, excludeID)
	if err != nil {
		return false, fmt.Errorf("failed to check layout handle conflict: %w", err)
	}

	count, err := parseCountResult(results)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

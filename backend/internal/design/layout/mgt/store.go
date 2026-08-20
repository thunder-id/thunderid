// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package layoutmgt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
)

var errLayoutNotFound = errors.New("layout not found")

// layoutMgtStoreInterface defines the interface for layout management store operations.
type layoutMgtStoreInterface interface {
	GetLayoutListCount(ctx context.Context) (int, error)
	GetLayoutList(ctx context.Context, limit, offset int) ([]Layout, error)
	CreateLayout(ctx context.Context, id string, layout CreateLayoutRequest) error
	GetLayout(ctx context.Context, id string) (Layout, error)
	IsLayoutExist(ctx context.Context, id string) (bool, error)
	UpdateLayout(ctx context.Context, id string, layout UpdateLayoutRequest) error
	DeleteLayout(ctx context.Context, id string) error
	IsLayoutDeclarative(id string) bool
	IsLayoutHandleConflict(ctx context.Context, handle string, excludeID string) (bool, error)
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
func (s *layoutMgtStore) GetLayoutListCount(ctx context.Context) (int, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return 0, err
	}

	countResults, err := dbClient.QueryContext(ctx, queryGetLayoutListCount, deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	return parseCountResult(countResults)
}

// GetLayoutList retrieves layout configurations with pagination.
func (s *layoutMgtStore) GetLayoutList(ctx context.Context, limit, offset int) ([]Layout, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return nil, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetLayoutList, limit, offset, deployment.Resolve(ctx,
		s.deploymentID))
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
func (s *layoutMgtStore) CreateLayout(ctx context.Context, id string, layout CreateLayoutRequest) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	layoutJSON, err := json.Marshal(layout.Layout)
	if err != nil {
		return fmt.Errorf("failed to marshal layout: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryCreateLayout, id, layout.Handle, layout.DisplayName, layout.Description,
		layoutJSON, deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// GetLayout retrieves a layout configuration by its id.
func (s *layoutMgtStore) GetLayout(ctx context.Context, id string) (Layout, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return Layout{}, err
	}

	results, err := dbClient.QueryContext(ctx, queryGetLayoutByID, id, deployment.Resolve(ctx, s.deploymentID))
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
func (s *layoutMgtStore) IsLayoutExist(ctx context.Context, id string) (bool, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return false, err
	}

	results, err := dbClient.QueryContext(ctx, queryCheckLayoutExists, id, deployment.Resolve(ctx, s.deploymentID))
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
func (s *layoutMgtStore) UpdateLayout(ctx context.Context, id string, layout UpdateLayoutRequest) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	layoutJSON, err := json.Marshal(layout.Layout)
	if err != nil {
		return fmt.Errorf("failed to marshal layout: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, queryUpdateLayout, layout.DisplayName, layout.Description, layoutJSON, id,
		deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

// DeleteLayout deletes a layout configuration.
func (s *layoutMgtStore) DeleteLayout(ctx context.Context, id string) error {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return err
	}

	_, err = dbClient.ExecuteContext(ctx, queryDeleteLayout, id, deployment.Resolve(ctx, s.deploymentID))
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
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
func (s *layoutMgtStore) IsLayoutHandleConflict(ctx context.Context, handle string, excludeID string) (bool, error) {
	dbClient, err := s.getConfigDBClient()
	if err != nil {
		return false, err
	}

	results, err := dbClient.QueryContext(ctx, queryCheckLayoutHandleConflict, handle, deployment.Resolve(ctx,
		s.deploymentID), excludeID)
	if err != nil {
		return false, fmt.Errorf("failed to check layout handle conflict: %w", err)
	}

	count, err := parseCountResult(results)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

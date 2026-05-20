/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package entitytype

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// marshalSystemAttributes marshals SystemAttributes to a JSON string for DB storage.
// Returns nil if the input is nil, so the DB column stores NULL.
func marshalSystemAttributes(sa *SystemAttributes) (interface{}, error) {
	if sa == nil {
		return nil, nil
	}
	data, err := json.Marshal(sa)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal system attributes: %w", err)
	}
	return string(data), nil
}

// parseSystemAttributes parses the SYSTEM_ATTRIBUTES column value into a *SystemAttributes.
// Returns nil if the column is NULL or empty.
func parseSystemAttributes(value interface{}) (*SystemAttributes, error) {
	if value == nil {
		return nil, nil
	}

	var raw string
	switch v := value.(type) {
	case string:
		raw = v
	case []byte:
		raw = string(v)
	default:
		return nil, fmt.Errorf("unexpected type for system_attributes: %T", value)
	}

	if raw == "" {
		return nil, nil
	}

	var sa SystemAttributes
	if err := json.Unmarshal([]byte(raw), &sa); err != nil {
		return nil, fmt.Errorf("failed to unmarshal system_attributes: %w", err)
	}
	return &sa, nil
}

// entityTypeStoreInterface defines the interface for entity type store operations.
type entityTypeStoreInterface interface {
	GetEntityTypeListCount(ctx context.Context, category TypeCategory) (int, error)
	GetEntityTypeList(ctx context.Context, category TypeCategory, limit, offset int) ([]EntityTypeListItem, error)
	GetEntityTypeListByOUIDs(ctx context.Context, category TypeCategory, ouIDs []string,
		limit, offset int) ([]EntityTypeListItem, error)
	GetEntityTypeListCountByOUIDs(ctx context.Context, category TypeCategory, ouIDs []string) (int, error)
	CreateEntityType(ctx context.Context, entityType EntityType) error
	GetEntityTypeByID(ctx context.Context, category TypeCategory, schemaID string) (EntityType, error)
	GetEntityTypeByName(ctx context.Context, category TypeCategory, name string) (EntityType, error)
	UpdateEntityTypeByID(ctx context.Context, category TypeCategory, schemaID string,
		entityType EntityType) error
	DeleteEntityTypeByID(ctx context.Context, category TypeCategory, schemaID string) error
	IsEntityTypeDeclarative(category TypeCategory, schemaID string) bool
	GetDisplayAttributesByNames(ctx context.Context, category TypeCategory,
		names []string) (map[string]string, error)
}

// entityTypeStore is the default implementation of entityTypeStoreInterface.
type entityTypeStore struct {
	dbProvider   provider.DBProviderInterface
	deploymentID string
}

// newEntityTypeStore creates a new instance of entityTypeStore.
func newEntityTypeStore() (entityTypeStoreInterface, transaction.Transactioner, error) {
	dbProvider := provider.GetDBProvider()
	transactioner, err := dbProvider.GetConfigDBTransactioner()
	if err != nil {
		return nil, nil, err
	}
	return &entityTypeStore{
		dbProvider:   dbProvider,
		deploymentID: config.GetServerRuntime().Config.Server.Identifier,
	}, transactioner, nil
}

// GetEntityTypeListCount retrieves the total count of entity types for the given category.
func (s *entityTypeStore) GetEntityTypeListCount(ctx context.Context, category TypeCategory) (int, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	countResults, err := dbClient.QueryContext(ctx, queryGetEntityTypeCount, string(category), s.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	var totalCount int
	if len(countResults) > 0 {
		if count, ok := countResults[0]["total"].(int64); ok {
			totalCount = int(count)
		} else {
			return 0, fmt.Errorf("failed to parse count result")
		}
	}

	return totalCount, nil
}

// GetEntityTypeList retrieves a paginated list of entity types for the given category.
func (s *entityTypeStore) GetEntityTypeList(ctx context.Context, category TypeCategory,
	limit, offset int) ([]EntityTypeListItem, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EntityTypePersistence"))

	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetEntityTypeList, limit, offset, s.deploymentID, string(category))
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	entityTypes := make([]EntityTypeListItem, 0, len(results))
	for _, row := range results {
		entityType, err := parseEntityTypeListItemFromRow(row)
		if err != nil {
			logger.Error("Failed to parse entity type list item from row", log.Error(err))
			continue
		}
		entityTypes = append(entityTypes, entityType)
	}

	return entityTypes, nil
}

// GetEntityTypeListByOUIDs retrieves a paginated list of entity types filtered by OU IDs and category.
func (s *entityTypeStore) GetEntityTypeListByOUIDs(ctx context.Context, category TypeCategory,
	ouIDs []string, limit, offset int) ([]EntityTypeListItem, error) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EntityTypePersistence"))

	if len(ouIDs) == 0 {
		return []EntityTypeListItem{}, nil
	}

	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query := buildGetEntityTypeListByOUIDsQuery(ouIDs)
	args := make([]interface{}, 0, len(ouIDs)+4)
	for _, id := range ouIDs {
		args = append(args, id)
	}
	args = append(args, string(category), s.deploymentID, limit, offset)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	entityTypes := make([]EntityTypeListItem, 0, len(results))
	for _, row := range results {
		entityType, err := parseEntityTypeListItemFromRow(row)
		if err != nil {
			logger.Error("Failed to parse entity type list item from row", log.Error(err))
			continue
		}
		entityTypes = append(entityTypes, entityType)
	}

	return entityTypes, nil
}

// GetEntityTypeListCountByOUIDs retrieves the total count of entity types filtered by OU IDs and category.
func (s *entityTypeStore) GetEntityTypeListCountByOUIDs(ctx context.Context, category TypeCategory,
	ouIDs []string) (int, error) {
	if len(ouIDs) == 0 {
		return 0, nil
	}

	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	query := buildGetEntityTypeCountByOUIDsQuery(ouIDs)
	args := make([]interface{}, 0, len(ouIDs)+2)
	for _, id := range ouIDs {
		args = append(args, id)
	}
	args = append(args, string(category), s.deploymentID)

	countResults, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	var totalCount int
	if len(countResults) > 0 {
		if count, ok := countResults[0]["total"].(int64); ok {
			totalCount = int(count)
		} else {
			return 0, fmt.Errorf("failed to parse count result")
		}
	}

	return totalCount, nil
}

// CreateEntityType creates a new entity type. The schema's Category field must be set.
func (s *entityTypeStore) CreateEntityType(ctx context.Context, entityType EntityType) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	sysAttrs, err := marshalSystemAttributes(entityType.SystemAttributes)
	if err != nil {
		return err
	}

	_, err = dbClient.QueryContext(
		ctx,
		queryCreateEntityType,
		entityType.ID,
		string(entityType.Category),
		entityType.Name,
		entityType.OUID,
		entityType.AllowSelfRegistration,
		string(entityType.Schema),
		sysAttrs,
		s.deploymentID,
	)
	if err != nil {
		return fmt.Errorf("failed to create entity type: %w", err)
	}

	return nil
}

// GetEntityTypeByID retrieves an entity type by its ID within a category.
func (s *entityTypeStore) GetEntityTypeByID(ctx context.Context, category TypeCategory,
	schemaID string) (EntityType, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return EntityType{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetEntityTypeByID, schemaID, s.deploymentID, string(category))
	if err != nil {
		return EntityType{}, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return EntityType{}, ErrEntityTypeNotFound
	}

	return parseEntityTypeFromRow(results[0])
}

// GetEntityTypeByName retrieves an entity type by its name within a category.
func (s *entityTypeStore) GetEntityTypeByName(ctx context.Context, category TypeCategory,
	name string) (EntityType, error) {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return EntityType{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, queryGetEntityTypeByName, name, s.deploymentID, string(category))
	if err != nil {
		return EntityType{}, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return EntityType{}, ErrEntityTypeNotFound
	}

	return parseEntityTypeFromRow(results[0])
}

// UpdateEntityTypeByID updates an entity type by its ID within a category.
func (s *entityTypeStore) UpdateEntityTypeByID(ctx context.Context, category TypeCategory,
	schemaID string, entityType EntityType) error {
	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	sysAttrs, err := marshalSystemAttributes(entityType.SystemAttributes)
	if err != nil {
		return err
	}

	_, err = dbClient.QueryContext(
		ctx,
		queryUpdateEntityTypeByID,
		entityType.Name,
		entityType.OUID,
		entityType.AllowSelfRegistration,
		string(entityType.Schema),
		sysAttrs,
		schemaID,
		s.deploymentID,
		string(category),
	)
	if err != nil {
		return fmt.Errorf("failed to update entity type: %w", err)
	}

	return nil
}

// DeleteEntityTypeByID deletes an entity type by its ID within a category.
func (s *entityTypeStore) DeleteEntityTypeByID(ctx context.Context, category TypeCategory,
	schemaID string) error {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EntityTypePersistence"))

	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, queryDeleteEntityTypeByID, schemaID, s.deploymentID,
		string(category))
	if err != nil {
		return fmt.Errorf("failed to delete entity type: %w", err)
	}

	if rowsAffected == 0 {
		logger.Debug("entity type not found with id: " + schemaID)
	}

	return nil
}

// IsEntityTypeDeclarative returns false as database-backed schemas are always mutable.
func (s *entityTypeStore) IsEntityTypeDeclarative(category TypeCategory, schemaID string) bool {
	return false
}

// GetDisplayAttributesByNames retrieves display attributes for a list of entity type names within a category.
func (s *entityTypeStore) GetDisplayAttributesByNames(ctx context.Context, category TypeCategory,
	names []string) (map[string]string, error) {
	if len(names) == 0 {
		return map[string]string{}, nil
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EntityTypePersistence"))

	dbClient, err := s.dbProvider.GetConfigDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query := buildGetDisplayAttributesByNamesQuery(names)
	args := make([]interface{}, 0, len(names)+2)
	for _, name := range names {
		args = append(args, name)
	}
	args = append(args, string(category), s.deploymentID)

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute display attributes query: %w", err)
	}

	displayAttrs := make(map[string]string, len(results))
	for _, row := range results {
		name, ok := row["name"].(string)
		if !ok {
			logger.Error("Failed to parse name from display attributes query")
			continue
		}

		sysAttrs, err := parseSystemAttributes(row["system_attributes"])
		if err != nil {
			logger.Error("Failed to parse system attributes", log.String("schemaName", name), log.Error(err))
			continue
		}

		if sysAttrs != nil {
			displayAttrs[name] = sysAttrs.Display
		} else {
			displayAttrs[name] = ""
		}
	}

	return displayAttrs, nil
}

// parseEntityTypeFromRow parses an entity type from a database row.
func parseEntityTypeFromRow(row map[string]interface{}) (EntityType, error) {
	schemaID, ok := row["id"].(string)
	if !ok {
		return EntityType{}, fmt.Errorf("failed to parse id as string")
	}

	categoryStr, ok := row["category"].(string)
	if !ok {
		return EntityType{}, fmt.Errorf("failed to parse category as string")
	}

	name, ok := row["name"].(string)
	if !ok {
		return EntityType{}, fmt.Errorf("failed to parse name as string")
	}

	oUID, ok := row["ou_id"].(string)
	if !ok {
		return EntityType{}, fmt.Errorf("failed to parse ou_id as string")
	}

	allowSelfRegistration, err := parseBool(row["allow_self_registration"], "allow_self_registration")
	if err != nil {
		return EntityType{}, err
	}

	var schemaDef string
	switch v := row["schema_def"].(type) {
	case string:
		schemaDef = v
	case []byte:
		schemaDef = string(v)
	default:
		return EntityType{}, fmt.Errorf("failed to parse schema_def as string")
	}

	systemAttributes, err := parseSystemAttributes(row["system_attributes"])
	if err != nil {
		return EntityType{}, err
	}

	entityType := EntityType{
		ID:                    schemaID,
		Category:              TypeCategory(categoryStr),
		Name:                  name,
		OUID:                  oUID,
		AllowSelfRegistration: allowSelfRegistration,
		SystemAttributes:      systemAttributes,
		Schema:                json.RawMessage(schemaDef),
	}

	return entityType, nil
}

// parseEntityTypeListItemFromRow parses a simplified entity type list item from a database row.
func parseEntityTypeListItemFromRow(row map[string]interface{}) (EntityTypeListItem, error) {
	schemaID, ok := row["id"].(string)
	if !ok {
		return EntityTypeListItem{}, fmt.Errorf("failed to parse id as string")
	}

	categoryStr, ok := row["category"].(string)
	if !ok {
		return EntityTypeListItem{}, fmt.Errorf("failed to parse category as string")
	}

	name, ok := row["name"].(string)
	if !ok {
		return EntityTypeListItem{}, fmt.Errorf("failed to parse name as string")
	}

	oUID, ok := row["ou_id"].(string)
	if !ok {
		return EntityTypeListItem{}, fmt.Errorf("failed to parse ou_id as string")
	}

	allowSelfRegistration, err := parseBool(row["allow_self_registration"], "allow_self_registration")
	if err != nil {
		return EntityTypeListItem{}, err
	}

	systemAttributes, err := parseSystemAttributes(row["system_attributes"])
	if err != nil {
		return EntityTypeListItem{}, err
	}

	entityTypeListItem := EntityTypeListItem{
		ID:                    schemaID,
		Category:              TypeCategory(categoryStr),
		Name:                  name,
		OUID:                  oUID,
		AllowSelfRegistration: allowSelfRegistration,
		SystemAttributes:      systemAttributes,
	}

	return entityTypeListItem, nil
}

func parseBool(value interface{}, fieldName string) (bool, error) {
	switch v := value.(type) {
	case nil:
		return false, fmt.Errorf("required boolean field '%s' is nil", fieldName)
	case bool:
		return v, nil
	case int64:
		return v != 0, nil
	case float64:
		return v != 0, nil
	case string:
		return strings.EqualFold(v, "true") || v == "1", nil
	case []byte:
		strVal := string(v)
		return strings.EqualFold(strVal, "true") || strVal == "1", nil
	default:
		return false, fmt.Errorf("failed to parse %s as bool", fieldName)
	}
}

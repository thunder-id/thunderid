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

package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/transaction"
)

// entityStoreInterface defines the interface for entity store operations.
type entityStoreInterface interface {
	// Entity CRUD
	CreateEntity(ctx context.Context, entity Entity,
		credentials json.RawMessage, systemCredentials json.RawMessage) error
	GetEntity(ctx context.Context, id string) (Entity, error)
	GetEntityWithCredentials(ctx context.Context, id string) (*entityWithCredentials, error)
	UpdateEntity(ctx context.Context, entity *Entity) error
	UpdateAttributes(ctx context.Context, entityID string, attributes json.RawMessage) error
	UpdateSystemAttributes(ctx context.Context, entityID string,
		attrs json.RawMessage) error
	UpdateCredentials(ctx context.Context, entityID string,
		creds json.RawMessage) error
	UpdateSystemCredentials(ctx context.Context, entityID string,
		creds json.RawMessage) error
	DeleteEntity(ctx context.Context, id string) error

	// Query
	IdentifyEntity(ctx context.Context, filters map[string]interface{}) (*string, error)
	SearchEntities(ctx context.Context, filters map[string]interface{}) ([]Entity, error)
	GetEntityListCount(ctx context.Context, category string,
		filters map[string]interface{}) (int, error)
	GetEntityList(ctx context.Context, category string,
		limit, offset int, filters map[string]interface{}) ([]Entity, error)
	GetEntityListCountByOUIDs(ctx context.Context, category string,
		ouIDs []string, filters map[string]interface{}) (int, error)
	GetEntityListByOUIDs(ctx context.Context, category string,
		ouIDs []string, limit, offset int, filters map[string]interface{}) ([]Entity, error)
	ValidateEntityIDs(ctx context.Context, entityIDs []string) ([]string, error)
	GetEntitiesByIDs(ctx context.Context, entityIDs []string) ([]Entity, error)
	ValidateEntityIDsInOUs(ctx context.Context, entityIDs []string, ouIDs []string) ([]string, error)

	// Groups
	GetGroupCountForEntity(ctx context.Context, entityID string) (int, error)
	GetEntityGroups(ctx context.Context, entityID string, limit, offset int) ([]EntityGroup, error)
	GetTransitiveEntityGroups(ctx context.Context, entityID string) ([]EntityGroup, error)

	// Declarative
	IsEntityDeclarative(ctx context.Context, id string) (bool, error)

	// Config
	GetIndexedAttributes() map[string]bool
	LoadIndexedAttributes(attributes []string) error
}

var getDBProvider = provider.GetDBProvider

// entityDBStore is the database implementation of entityStoreInterface.
type entityDBStore struct {
	deploymentID      string
	indexedAttributes map[string]bool
	dbProvider        provider.DBProviderInterface
	logger            *log.Logger
}

// newEntityDBStore creates a new instance of entityDBStore.
// Indexed attributes start empty; consumers must call LoadIndexedAttributes after init.
func newEntityDBStore() (entityStoreInterface, transaction.Transactioner, error) {
	runtime := config.GetServerRuntime()

	dbProvider := getDBProvider()
	client, err := dbProvider.GetUserDBClient()
	if err != nil {
		return nil, nil, err
	}
	transactioner, err := client.GetTransactioner()
	if err != nil {
		return nil, nil, err
	}

	return &entityDBStore{
		deploymentID:      runtime.Config.Server.Identifier,
		indexedAttributes: make(map[string]bool),
		dbProvider:        dbProvider,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EntityStore")),
	}, transactioner, nil
}

// LoadIndexedAttributes merges the given attributes into the indexed set.
// The cumulative total must not exceed MaxIndexedAttributesCount.
func (es *entityDBStore) LoadIndexedAttributes(attributes []string) error {
	combined := make([]string, 0, len(es.indexedAttributes)+len(attributes))
	for attr := range es.indexedAttributes {
		combined = append(combined, attr)
	}
	for _, attr := range attributes {
		if !es.indexedAttributes[attr] {
			combined = append(combined, attr)
		}
	}
	if err := validateIndexedAttributesConfig(combined); err != nil {
		return fmt.Errorf("indexed attributes load failed: %w", err)
	}
	for _, attr := range attributes {
		es.indexedAttributes[attr] = true
	}
	return nil
}

// CreateEntity creates a new entity in the database.
func (es *entityDBStore) CreateEntity(ctx context.Context, entity Entity,
	credentials json.RawMessage, systemCredentials json.RawMessage) error {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	attributes, err := json.Marshal(entity.Attributes)
	if err != nil {
		return ErrBadAttributesInRequest
	}

	systemAttrs := "{}"
	if len(entity.SystemAttributes) > 0 {
		systemAttrs = string(entity.SystemAttributes)
	}

	credsJSON := "{}"
	if len(credentials) > 0 {
		credsJSON = string(credentials)
	}

	sysCredsJSON := "{}"
	if len(systemCredentials) > 0 {
		sysCredsJSON = string(systemCredentials)
	}

	now := time.Now().UTC()
	_, err = dbClient.ExecuteContext(
		ctx,
		QueryCreateEntity,
		entity.ID,
		es.deploymentID,
		string(entity.Category),
		entity.Type,
		string(entity.State),
		entity.OUID,
		string(attributes),
		systemAttrs,
		credsJSON,
		sysCredsJSON,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create entity: %w", err)
	}

	if err := es.syncAttributeIdentifiers(
		ctx, entity.ID, entity.Attributes, entity.SystemAttributes, es.indexedAttributes); err != nil {
		return fmt.Errorf("failed to sync identifiers: %w", err)
	}

	return nil
}

// GetEntity retrieves an entity by ID (without credentials).
func (es *entityDBStore) GetEntity(ctx context.Context, id string) (Entity, error) {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return Entity{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryGetEntityByID, id, es.deploymentID)
	if err != nil {
		return Entity{}, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return Entity{}, ErrEntityNotFound
	}

	if len(results) != 1 {
		return Entity{}, fmt.Errorf("unexpected number of results: %d", len(results))
	}

	return buildEntityFromResultRow(results[0])
}

// GetEntityWithCredentials retrieves an entity with all credential columns.
func (es *entityDBStore) GetEntityWithCredentials(ctx context.Context, id string) (
	*entityWithCredentials, error) {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryGetEntityWithCredentials, id, es.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return nil, ErrEntityNotFound
	}

	if len(results) != 1 {
		return nil, fmt.Errorf("unexpected number of results: %d", len(results))
	}

	row := results[0]
	entity, err := buildEntityFromResultRow(row)
	if err != nil {
		return nil, err
	}

	return &entityWithCredentials{
		Entity:            &entity,
		SchemaCredentials: parseJSONColumn(row, "credentials"),
		SystemCredentials: parseJSONColumn(row, "system_credentials"),
	}, nil
}

// UpdateEntity fully updates an entity including system attributes, and re-syncs all identifiers.
func (es *entityDBStore) UpdateEntity(ctx context.Context, entity *Entity) error {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	attributes, err := json.Marshal(entity.Attributes)
	if err != nil {
		return fmt.Errorf("failed to marshal attributes: %w", err)
	}

	systemAttrs := "{}"
	if len(entity.SystemAttributes) > 0 {
		systemAttrs = string(entity.SystemAttributes)
	}

	rowsAffected, err := dbClient.ExecuteContext(
		ctx,
		QueryUpdateEntity,
		entity.ID, entity.OUID, entity.Type,
		string(entity.State), string(attributes), systemAttrs, time.Now().UTC(), es.deploymentID,
	)
	if err != nil {
		return fmt.Errorf("failed to execute update entity query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEntityNotFound
	}

	// Reload the entity to get the authoritative post-update state for identifier sync.
	// This ensures identifiers reflect what is actually stored in the DB, regardless of
	// what the caller passed in SystemAttributes.
	current, err := es.GetEntity(ctx, entity.ID)
	if err != nil {
		return fmt.Errorf("failed to reload entity for identifier sync: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, QueryDeleteIdentifiersByEntity, entity.ID, es.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to delete identifiers: %w", err)
	}

	if err := es.syncAttributeIdentifiers(
		ctx, entity.ID, current.Attributes, current.SystemAttributes, es.indexedAttributes); err != nil {
		return fmt.Errorf("failed to sync identifiers: %w", err)
	}

	return nil
}

// UpdateAttributes updates only the schema attributes of an entity and re-syncs attribute-sourced identifiers.
func (es *entityDBStore) UpdateAttributes(ctx context.Context, entityID string, attributes json.RawMessage) error {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryUpdateAttributes,
		entityID, string(attributes), time.Now().UTC(), es.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute update attributes query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEntityNotFound
	}

	if _, err = dbClient.ExecuteContext(ctx, QueryDeleteAttributeIdentifiersByEntity,
		entityID, es.deploymentID); err != nil {
		return fmt.Errorf("failed to delete attribute identifiers: %w", err)
	}

	if err = es.syncAttributeIdentifiers(ctx, entityID, attributes, nil, es.indexedAttributes); err != nil {
		return fmt.Errorf("failed to sync attribute identifiers: %w", err)
	}

	return nil
}

// UpdateSystemAttributes updates the system attributes of an entity and re-syncs system-sourced identifiers.
func (es *entityDBStore) UpdateSystemAttributes(ctx context.Context, entityID string,
	attrs json.RawMessage) error {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryUpdateSystemAttributes,
		entityID, string(attrs), time.Now().UTC(), es.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEntityNotFound
	}

	if _, err = dbClient.ExecuteContext(ctx, QueryDeleteSystemIdentifiersByEntity,
		entityID, es.deploymentID); err != nil {
		return fmt.Errorf("failed to delete system identifiers: %w", err)
	}

	if err = es.syncAttributeIdentifiers(ctx, entityID, nil, attrs, es.indexedAttributes); err != nil {
		return fmt.Errorf("failed to sync system identifiers: %w", err)
	}

	return nil
}

// UpdateCredentials updates the credentials of an entity.
func (es *entityDBStore) UpdateCredentials(ctx context.Context, entityID string,
	creds json.RawMessage) error {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryUpdateCredentials,
		entityID, string(creds), time.Now().UTC(), es.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEntityNotFound
	}

	return nil
}

// UpdateSystemCredentials updates the system credentials of an entity.
func (es *entityDBStore) UpdateSystemCredentials(ctx context.Context, entityID string,
	creds json.RawMessage) error {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryUpdateSystemCredentials,
		entityID, string(creds), time.Now().UTC(), es.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEntityNotFound
	}

	return nil
}

// DeleteEntity deletes an entity and its indexed identifiers from the database.
func (es *entityDBStore) DeleteEntity(ctx context.Context, id string) error {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryDeleteEntity, id, es.deploymentID)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEntityNotFound
	}

	if _, err = dbClient.ExecuteContext(ctx, QueryDeleteIdentifiersByEntity, id, es.deploymentID); err != nil {
		return fmt.Errorf("failed to delete entity identifiers: %w", err)
	}

	return nil
}

// syncAttributeIdentifiers synchronizes indexed attributes from both Attributes and SystemAttributes
// to the identifier store. Schema attributes get source="attribute", system attributes get source="system".
func (es *entityDBStore) syncAttributeIdentifiers(ctx context.Context, entityID string,
	attributes json.RawMessage, systemAttributes json.RawMessage,
	indexedAttrs map[string]bool) error {
	query, args, err := prepareIdentifierQuery(entityID, attributes, systemAttributes, indexedAttrs, es.deploymentID)
	if err != nil {
		return err
	}
	if query == nil {
		return nil
	}

	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, *query, args...)
	if err != nil {
		return fmt.Errorf("failed to batch insert identifiers: %w", err)
	}

	return nil
}

// IdentifyEntity identifies an entity with the given filters.
func (es *entityDBStore) IdentifyEntity(ctx context.Context,
	filters map[string]interface{}) (*string, error) {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	// Fast path: try indexed identifier store first for all lookups.
	// This covers both schema-indexed attributes (email, username) and
	// system identifiers without requiring config.
	identifyQuery, args, err := buildIdentifyQueryFromIdentifiers(filters, es.deploymentID)
	if err == nil {
		results, qErr := dbClient.QueryContext(ctx, identifyQuery, args...)
		if qErr == nil && len(results) == 1 {
			if entityID, ok := results[0]["id"].(string); ok {
				return &entityID, nil
			}
		}
	}

	// Fallback: categorize filters into indexed and non-indexed for JSONB search.
	indexedFilters := make(map[string]interface{})
	nonIndexedFilters := make(map[string]interface{})

	for key, value := range filters {
		if es.indexedAttributes[key] {
			indexedFilters[key] = value
		} else {
			nonIndexedFilters[key] = value
		}
	}

	var fallbackQuery dbmodel.DBQuery
	var fallbackArgs []interface{}

	if len(indexedFilters) > 0 && len(nonIndexedFilters) > 0 {
		// Mixed: identifier table for indexed filters + JSON for non-indexed filters.
		fallbackQuery, fallbackArgs, err = buildIdentifyQueryHybrid(indexedFilters, nonIndexedFilters, es.deploymentID)
		if err != nil {
			return nil, fmt.Errorf("failed to build hybrid query: %w", err)
		}
	} else {
		// All-indexed: fast path already tried the identifier table; fall back to JSON search.
		// All non-indexed: always use JSON search.
		fallbackQuery, fallbackArgs, err = buildIdentifyQuery(filters, es.deploymentID)
		if err != nil {
			return nil, fmt.Errorf("failed to build identify query: %w", err)
		}
	}

	results, err := dbClient.QueryContext(ctx, fallbackQuery, fallbackArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		if es.logger.IsDebugEnabled() {
			es.logger.Debug("Entity not found with the provided filters", log.MaskedMap("filters", filters))
		}
		return nil, ErrEntityNotFound
	}

	if len(results) != 1 {
		if es.logger.IsDebugEnabled() {
			es.logger.Debug(
				"Unexpected number of results for the provided filters",
				log.MaskedMap("filters", filters),
				log.Int("result_count", len(results)),
			)
		}
		return nil, ErrAmbiguousEntity
	}

	row := results[0]
	entityID, ok := row["id"].(string)
	if !ok {
		return nil, fmt.Errorf("failed to parse id as string")
	}

	return &entityID, nil
}

// SearchEntities searches for all entities matching the provided filters.
// Unlike IdentifyEntity, this returns all matching entities instead of erroring on ambiguity.
// Results are capped at MaxPageSize (100) entries; matches beyond that limit are not returned.
// Column-level filters (category, ouId) should be handled at the service layer.
func (es *entityDBStore) SearchEntities(ctx context.Context,
	filters map[string]interface{}) ([]Entity, error) {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	searchQuery, args, err := buildEntityListQuery(
		"", filters, serverconst.MaxPageSize, 0, es.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to build search query: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, searchQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}

	if len(results) == 0 {
		return nil, ErrEntityNotFound
	}

	return buildEntitiesFromResults(results)
}

// GetIndexedAttributes returns the set of configured indexed attributes.
func (es *entityDBStore) GetIndexedAttributes() map[string]bool {
	return es.indexedAttributes
}

// GetEntityListCount retrieves the total count of entities by category.
func (es *entityDBStore) GetEntityListCount(ctx context.Context, category string,
	filters map[string]interface{}) (int, error) {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	countQuery, args, err := buildEntityCountQuery(category, filters, es.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}

	return executeCountQuery(dbClient, ctx, countQuery, args)
}

// GetEntityList retrieves a list of entities by category.
func (es *entityDBStore) GetEntityList(ctx context.Context, category string,
	limit, offset int, filters map[string]interface{}) ([]Entity, error) {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	listQuery, args, err := buildEntityListQuery(category, filters, limit, offset, es.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to build list query: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute paginated query: %w", err)
	}

	return buildEntitiesFromResults(results)
}

// GetEntityListCountByOUIDs retrieves the total count of entities scoped to OU IDs.
func (es *entityDBStore) GetEntityListCountByOUIDs(ctx context.Context, category string,
	ouIDs []string, filters map[string]interface{}) (int, error) {
	if len(ouIDs) == 0 {
		return 0, nil
	}
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	countQuery, args, err := buildEntityCountQueryByOUIDs(category, ouIDs, filters, es.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}

	return executeCountQuery(dbClient, ctx, countQuery, args)
}

// GetEntityListByOUIDs retrieves a list of entities scoped to OU IDs.
func (es *entityDBStore) GetEntityListByOUIDs(ctx context.Context, category string,
	ouIDs []string, limit, offset int, filters map[string]interface{}) ([]Entity, error) {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	listQuery, args, err := buildEntityListQueryByOUIDs(category, ouIDs, filters, limit, offset, es.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to build list query: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, listQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute paginated query: %w", err)
	}

	return buildEntitiesFromResults(results)
}

// ValidateEntityIDs checks if all provided entity IDs exist.
func (es *entityDBStore) ValidateEntityIDs(ctx context.Context, entityIDs []string) ([]string, error) {
	if len(entityIDs) == 0 {
		return []string{}, nil
	}

	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query, args, err := buildBulkEntityExistsQuery(entityIDs, es.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to build bulk entity exists query: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	existingIDs := make(map[string]bool)
	for _, row := range results {
		if id, ok := row["id"].(string); ok {
			existingIDs[id] = true
		}
	}

	var invalidIDs []string
	for _, id := range entityIDs {
		if !existingIDs[id] {
			invalidIDs = append(invalidIDs, id)
		}
	}

	return invalidIDs, nil
}

// GetEntitiesByIDs retrieves entities by a list of IDs.
func (es *entityDBStore) GetEntitiesByIDs(ctx context.Context, entityIDs []string) ([]Entity, error) {
	const batchSize = 100

	if len(entityIDs) == 0 {
		return []Entity{}, nil
	}

	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	entities := make([]Entity, 0, len(entityIDs))

	for start := 0; start < len(entityIDs); start += batchSize {
		end := start + batchSize
		if end > len(entityIDs) {
			end = len(entityIDs)
		}
		chunk := entityIDs[start:end]

		query, args, err := buildGetEntitiesByIDsQuery(chunk, es.deploymentID)
		if err != nil {
			return nil, fmt.Errorf("failed to build get entities by IDs query: %w", err)
		}

		results, err := dbClient.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute query: %w", err)
		}

		batch, err := buildEntitiesFromResults(results)
		if err != nil {
			return nil, err
		}
		entities = append(entities, batch...)
	}

	return entities, nil
}

// ValidateEntityIDsInOUs checks which of the provided entity IDs belong to the given OU scope.
func (es *entityDBStore) ValidateEntityIDsInOUs(
	ctx context.Context, entityIDs []string, ouIDs []string,
) ([]string, error) {
	if len(entityIDs) == 0 {
		return []string{}, nil
	}
	if len(ouIDs) == 0 {
		return append([]string{}, entityIDs...), nil
	}

	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query, args, err := buildBulkEntityExistsQueryInOUs(entityIDs, ouIDs, es.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	inScopeIDs := make(map[string]bool, len(results))
	for _, row := range results {
		if id, ok := row["id"].(string); ok {
			inScopeIDs[id] = true
		}
	}

	outOfScopeIDs := make([]string, 0)
	for _, id := range entityIDs {
		if !inScopeIDs[id] {
			outOfScopeIDs = append(outOfScopeIDs, id)
		}
	}
	return outOfScopeIDs, nil
}

// GetGroupCountForEntity retrieves the total count of groups an entity belongs to.
func (es *entityDBStore) GetGroupCountForEntity(ctx context.Context, entityID string) (int, error) {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	countResults, err := dbClient.QueryContext(ctx, QueryGetGroupCountForEntity, entityID, es.deploymentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get group count for entity: %w", err)
	}

	if len(countResults) == 0 {
		return 0, nil
	}

	if count, ok := countResults[0]["total"].(int64); ok {
		return int(count), nil
	}
	return 0, fmt.Errorf("unexpected type for total: %T", countResults[0]["total"])
}

// GetEntityGroups retrieves groups that an entity belongs to with pagination.
func (es *entityDBStore) GetEntityGroups(
	ctx context.Context, entityID string, limit, offset int) ([]EntityGroup, error) {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryGetGroupsForEntity,
		entityID, limit, offset, es.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get groups for entity: %w", err)
	}

	groups := make([]EntityGroup, 0, len(results))
	for _, row := range results {
		group, err := buildGroupFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build group from result row: %w", err)
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// GetTransitiveEntityGroups retrieves all groups an entity belongs to, including nested group membership.
func (es *entityDBStore) GetTransitiveEntityGroups(
	ctx context.Context, entityID string) ([]EntityGroup, error) {
	dbClient, err := es.dbProvider.GetUserDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryGetTransitiveGroupsForEntity,
		entityID, es.deploymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get transitive groups for entity: %w", err)
	}

	groups := make([]EntityGroup, 0, len(results))
	for _, row := range results {
		group, err := buildGroupFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build group from result row: %w", err)
		}
		groups = append(groups, group)
	}

	return groups, nil
}

// IsEntityDeclarative returns false for database store (all database entities are mutable).
func (es *entityDBStore) IsEntityDeclarative(ctx context.Context, id string) (bool, error) {
	_, err := es.GetEntity(ctx, id)
	if err != nil {
		return false, err
	}
	return false, nil
}

// Helper functions
func buildEntityFromResultRow(row map[string]interface{}) (Entity, error) {
	entityID, ok := row["id"].(string)
	if !ok {
		return Entity{}, fmt.Errorf("failed to parse id as string")
	}

	ouID, ok := row["ou_id"].(string)
	if !ok {
		return Entity{}, fmt.Errorf("failed to parse ou_id as string")
	}

	category, ok := row["category"].(string)
	if !ok {
		return Entity{}, fmt.Errorf("failed to parse category as string")
	}

	entityType, ok := row["type"].(string)
	if !ok {
		return Entity{}, fmt.Errorf("failed to parse type as string")
	}

	state, ok := row["state"].(string)
	if !ok {
		return Entity{}, fmt.Errorf("failed to parse state as string")
	}

	var attributes string
	switch v := row["attributes"].(type) {
	case string:
		attributes = v
	case []byte:
		attributes = string(v)
	default:
		return Entity{}, fmt.Errorf("failed to parse attributes as string")
	}

	entity := Entity{
		ID:       entityID,
		Category: EntityCategory(category),
		Type:     entityType,
		State:    EntityState(state),
		OUID:     ouID,
	}

	if err := json.Unmarshal([]byte(attributes), &entity.Attributes); err != nil {
		return Entity{}, fmt.Errorf("failed to unmarshal attributes")
	}

	entity.SystemAttributes = parseJSONColumn(row, "system_attributes")

	return entity, nil
}

func buildGroupFromResultRow(row map[string]interface{}) (EntityGroup, error) {
	groupID, ok := row["id"].(string)
	if !ok {
		return EntityGroup{}, fmt.Errorf("failed to parse id as string")
	}

	name, ok := row["name"].(string)
	if !ok {
		return EntityGroup{}, fmt.Errorf("failed to parse name as string")
	}

	ouID, ok := row["ou_id"].(string)
	if !ok {
		return EntityGroup{}, fmt.Errorf("failed to parse ou_id as string")
	}

	return EntityGroup{ID: groupID, Name: name, OUID: ouID}, nil
}

func buildEntitiesFromResults(results []map[string]interface{}) ([]Entity, error) {
	entities := make([]Entity, 0, len(results))
	for _, row := range results {
		entity, err := buildEntityFromResultRow(row)
		if err != nil {
			return nil, fmt.Errorf("failed to build entity from result row: %w", err)
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func parseJSONColumn(row map[string]interface{}, column string) json.RawMessage {
	val, exists := row[column]
	if !exists || val == nil {
		return nil
	}
	switch v := val.(type) {
	case string:
		if v == "" {
			return nil
		}
		return json.RawMessage(v)
	case []byte:
		s := string(v)
		if s == "" {
			return nil
		}
		return json.RawMessage(s)
	default:
		return nil
	}
}

func executeCountQuery(dbClient provider.DBClientInterface, ctx context.Context,
	query dbmodel.DBQuery, args []interface{}) (int, error) {
	countResults, err := dbClient.QueryContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to execute count query: %w", err)
	}

	var totalCount int
	if len(countResults) > 0 {
		if count, ok := countResults[0]["total"].(int64); ok {
			totalCount = int(count)
		} else {
			return 0, fmt.Errorf("unexpected type for total: %T", countResults[0]["total"])
		}
	}

	return totalCount, nil
}

func prepareIdentifierQuery(
	entityID string, attributes json.RawMessage, systemAttributes json.RawMessage,
	indexedAttrs map[string]bool, deploymentID string,
) (*dbmodel.DBQuery, []interface{}, error) {
	type indexedAttr struct {
		name   string
		value  string
		source string
	}
	var toInsert []indexedAttr

	// Extract indexed attributes from schema attributes (source = "attribute").
	if len(attributes) > 0 {
		var attrMap map[string]interface{}
		if err := json.Unmarshal(attributes, &attrMap); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
		}
		for attrName, attrValue := range attrMap {
			if !indexedAttrs[attrName] {
				continue
			}
			if valueStr := attrValueToString(attrValue); valueStr != "" {
				toInsert = append(toInsert, indexedAttr{name: attrName, value: valueStr, source: "attribute"})
			}
		}
	}

	// Extract indexed attributes from system attributes (source = "system").
	if len(systemAttributes) > 0 {
		var sysAttrMap map[string]interface{}
		if err := json.Unmarshal(systemAttributes, &sysAttrMap); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal system attributes: %w", err)
		}
		for attrName, attrValue := range sysAttrMap {
			if !indexedAttrs[attrName] {
				continue
			}
			if valueStr := attrValueToString(attrValue); valueStr != "" {
				toInsert = append(toInsert, indexedAttr{name: attrName, value: valueStr, source: "system"})
			}
		}
	}

	if len(toInsert) == 0 {
		return nil, nil, nil
	}

	// Deduplicate by attribute name; if the same key appears in both schema and system attributes,
	// the system attribute entry wins (it was appended last and overwrites the schema one).
	dedupMap := make(map[string]indexedAttr, len(toInsert))
	dedupOrder := make([]string, 0, len(toInsert))
	for _, attr := range toInsert {
		if _, exists := dedupMap[attr.name]; !exists {
			dedupOrder = append(dedupOrder, attr.name)
		}
		dedupMap[attr.name] = attr
	}
	deduped := make([]indexedAttr, 0, len(dedupMap))
	for _, name := range dedupOrder {
		deduped = append(deduped, dedupMap[name])
	}
	toInsert = deduped

	now := time.Now().UTC()
	valuePlaceholders := make([]string, 0, len(toInsert))
	args := make([]interface{}, 0, len(toInsert)*6)
	paramIndex := 1

	for _, attr := range toInsert {
		valuePlaceholders = append(valuePlaceholders,
			fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)",
				paramIndex, paramIndex+1, paramIndex+2, paramIndex+3, paramIndex+4, paramIndex+5))
		args = append(args, entityID, attr.name, attr.value, attr.source, deploymentID, now)
		paramIndex += 6
	}

	queryStr := QueryBatchInsertIdentifiers.Query + strings.Join(valuePlaceholders, ", ")
	query := &dbmodel.DBQuery{
		ID:    QueryBatchInsertIdentifiers.ID,
		Query: queryStr,
	}

	return query, args, nil
}

// attrValueToString converts an attribute value to string for indexing.
// Returns empty string for complex types that can't be indexed.
func attrValueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64, int, int64, bool:
		return fmt.Sprintf("%v", v)
	default:
		return ""
	}
}

func validateIndexedAttributesConfig(configuredAttrs []string) error {
	if len(configuredAttrs) > MaxIndexedAttributesCount {
		return fmt.Errorf("indexed attributes count (%d) must not exceed %d",
			len(configuredAttrs), MaxIndexedAttributesCount)
	}
	return nil
}

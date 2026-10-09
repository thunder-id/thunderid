// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	dbmodel "github.com/thunder-id/thunderid/internal/system/database/model"
	"github.com/thunder-id/thunderid/internal/system/database/provider"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// indexedAttr is one row destined for ENTITY_IDENTIFIER.
type indexedAttr struct {
	name   string
	value  string
	source string
}

// entityStoreInterface defines the interface for entity store operations.
type entityStoreInterface interface {
	// providers.Entity CRUD
	CreateEntity(ctx context.Context, entity providers.Entity,
		credentials json.RawMessage, systemCredentials json.RawMessage) error
	GetEntity(ctx context.Context, id string) (providers.Entity, error)
	GetEntityWithCredentials(ctx context.Context, id string) (*entityWithCredentials, error)
	UpdateEntity(ctx context.Context, entity *providers.Entity) error
	UpdateAttributes(ctx context.Context, entityID string, attributes json.RawMessage) error
	UpdateSystemAttributes(ctx context.Context, entityID string,
		attrs json.RawMessage) error
	UpdateCredentials(ctx context.Context, entityID string,
		creds json.RawMessage) error
	UpdateSystemCredentials(ctx context.Context, entityID string,
		creds json.RawMessage) error
	DeleteEntity(ctx context.Context, id string) error
	LockEntity(ctx context.Context, id string) (providers.Entity, error)

	// Query
	IdentifyEntity(ctx context.Context, filters map[string]interface{}) (*string, error)
	ResolveLinkedAccount(ctx context.Context, idpID, sub string) (*string, error)
	SearchEntities(ctx context.Context, filters map[string]interface{}) ([]providers.Entity, error)
	GetEntityListCount(ctx context.Context, category string,
		filters map[string]interface{}) (int, error)
	GetEntityList(ctx context.Context, category string,
		limit, offset int, filters map[string]interface{}) ([]providers.Entity, error)
	GetEntityListCountByOUIDs(ctx context.Context, category string,
		ouIDs []string, filters map[string]interface{}) (int, error)
	GetEntityListByOUIDs(ctx context.Context, category string,
		ouIDs []string, limit, offset int, filters map[string]interface{}) ([]providers.Entity, error)
	ValidateEntityIDs(ctx context.Context, entityIDs []string) ([]string, error)
	GetEntitiesByIDs(ctx context.Context, entityIDs []string) ([]providers.Entity, error)
	ValidateEntityIDsInOUs(ctx context.Context, entityIDs []string, ouIDs []string) ([]string, error)

	// Groups
	GetGroupCountForEntity(ctx context.Context, entityID string) (int, error)
	GetEntityGroups(ctx context.Context, entityID string, limit, offset int) ([]providers.EntityGroup, error)

	// Declarative
	IsEntityDeclarative(ctx context.Context, id string) (bool, error)

	// Config
	GetIndexedAttributes() map[string]bool
	LoadIndexedAttributes(attributes []string) error
}

var getDBProvider = provider.GetDBProvider

// entityDBStore is the database implementation of entityStoreInterface.
type entityDBStore struct {
	indexedAttributes map[string]bool
	dbProvider        provider.DBProviderInterface
	logger            *log.Logger
}

// newEntityDBStore creates a new instance of entityDBStore.
// Indexed attributes start empty; consumers must call LoadIndexedAttributes after init.
func newEntityDBStore() (entityStoreInterface, providers.Transactioner, error) {
	dbProvider := getDBProvider()
	client, err := dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, nil, err
	}
	transactioner, err := client.GetTransactioner()
	if err != nil {
		return nil, nil, err
	}

	return &entityDBStore{
		indexedAttributes: make(map[string]bool),
		dbProvider:        dbProvider,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EntityStore")),
	}, transactioner, nil
}

// scope returns the deployment id this request acts for, falling back to the configured
// identifier for a context that never passed through the edge.
func (es *entityDBStore) scope(ctx context.Context) string {
	return deployment.Resolve(ctx)
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
func (es *entityDBStore) CreateEntity(ctx context.Context, entity providers.Entity,
	credentials json.RawMessage, systemCredentials json.RawMessage) error {
	dbClient, err := es.dbProvider.GetEntityDBClient()
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
		es.scope(ctx),
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
func (es *entityDBStore) GetEntity(ctx context.Context, id string) (providers.Entity, error) {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return providers.Entity{}, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryGetEntityByID, id, es.scope(ctx))
	if err != nil {
		return providers.Entity{}, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return providers.Entity{}, ErrEntityNotFound
	}

	if len(results) != 1 {
		return providers.Entity{}, fmt.Errorf("unexpected number of results: %d", len(results))
	}

	return buildEntityFromResultRow(results[0])
}

// GetEntityWithCredentials retrieves an entity with all credential columns.
func (es *entityDBStore) GetEntityWithCredentials(ctx context.Context, id string) (
	*entityWithCredentials, error) {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryGetEntityWithCredentials, id, es.scope(ctx))
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
func (es *entityDBStore) UpdateEntity(ctx context.Context, entity *providers.Entity) error {
	dbClient, err := es.dbProvider.GetEntityDBClient()
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
		string(entity.State), string(attributes), systemAttrs, time.Now().UTC(), es.scope(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to execute update entity query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEntityNotFound
	}

	return es.resyncIdentifiers(ctx, dbClient, entity.ID)
}

// resyncIdentifiers rebuilds all identifiers of an entity from its stored schema and system attributes.
// Both sources are re-indexed together, since the system-over-schema precedence for a name depends on
// both. The entity is reloaded so identifiers reflect what is actually stored in the DB.
func (es *entityDBStore) resyncIdentifiers(ctx context.Context, dbClient provider.DBClientInterface,
	entityID string) error {
	current, err := es.GetEntity(ctx, entityID)
	if err != nil {
		return fmt.Errorf("failed to reload entity for identifier sync: %w", err)
	}

	if _, err = dbClient.ExecuteContext(ctx, QueryDeleteIdentifiersByEntity, entityID, es.scope(ctx)); err != nil {
		return fmt.Errorf("failed to delete identifiers: %w", err)
	}

	if err = es.syncAttributeIdentifiers(
		ctx, entityID, current.Attributes, current.SystemAttributes, es.indexedAttributes); err != nil {
		return fmt.Errorf("failed to sync identifiers: %w", err)
	}

	return nil
}

// UpdateAttributes updates only the schema attributes of an entity and re-syncs all identifiers.
func (es *entityDBStore) UpdateAttributes(ctx context.Context, entityID string, attributes json.RawMessage) error {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryUpdateAttributes,
		entityID, string(attributes), time.Now().UTC(), es.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to execute update attributes query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEntityNotFound
	}

	return es.resyncIdentifiers(ctx, dbClient, entityID)
}

// UpdateSystemAttributes updates only the system attributes of an entity and re-syncs all identifiers.
func (es *entityDBStore) UpdateSystemAttributes(ctx context.Context, entityID string,
	attrs json.RawMessage) error {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryUpdateSystemAttributes,
		entityID, string(attrs), time.Now().UTC(), es.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEntityNotFound
	}

	return es.resyncIdentifiers(ctx, dbClient, entityID)
}

// UpdateCredentials updates the credentials of an entity.
func (es *entityDBStore) UpdateCredentials(ctx context.Context, entityID string,
	creds json.RawMessage) error {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryUpdateCredentials,
		entityID, string(creds), time.Now().UTC(), es.scope(ctx))
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
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryUpdateSystemCredentials,
		entityID, string(creds), time.Now().UTC(), es.scope(ctx))
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
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryDeleteEntity, id, es.scope(ctx))
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	if rowsAffected == 0 {
		return ErrEntityNotFound
	}

	if _, err = dbClient.ExecuteContext(ctx, QueryDeleteIdentifiersByEntity, id, es.scope(ctx)); err != nil {
		return fmt.Errorf("failed to delete entity identifiers: %w", err)
	}

	return nil
}

// syncAttributeIdentifiers synchronizes indexed attributes from both Attributes and SystemAttributes
// to the identifier store. Schema attributes get source="attribute", system attributes get source="system".
func (es *entityDBStore) syncAttributeIdentifiers(ctx context.Context, entityID string,
	attributes json.RawMessage, systemAttributes json.RawMessage,
	indexedAttrs map[string]bool) error {
	query, args, err := prepareIdentifierQuery(entityID, attributes, systemAttributes, indexedAttrs, es.scope(ctx))
	if err != nil {
		return err
	}
	if query == nil {
		return nil
	}

	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return fmt.Errorf("failed to get database client: %w", err)
	}

	_, err = dbClient.ExecuteContext(ctx, *query, args...)
	if provider.IsUniqueIndexViolation(err, linkedIDIndexName, linkedIDIndexColumns) {
		return ErrLinkedAccountConflict
	}
	if err != nil {
		return fmt.Errorf("failed to batch insert identifiers: %w", err)
	}

	return nil
}

// IdentifyEntity identifies an entity with the given filters.
func (es *entityDBStore) IdentifyEntity(ctx context.Context,
	filters map[string]interface{}) (*string, error) {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	// Categorize filters into indexed and non-indexed for the fast path and the JSONB fallback.
	indexedFilters := make(map[string]interface{})
	nonIndexedFilters := make(map[string]interface{})
	// A key with no identifier rows makes the fast path match nothing, so it is skipped.
	allHaveIdentifierRows := true

	for key, value := range filters {
		if es.indexedAttributes[key] {
			indexedFilters[key] = value
		} else {
			nonIndexedFilters[key] = value
			if !strings.HasPrefix(key, authnprovidercm.SystemAttrLinkedIDs+".") {
				allHaveIdentifierRows = false
			}
		}
	}

	// Fast path: try indexed identifier store first for all lookups.
	// This covers both schema-indexed attributes (email, username) and
	// system identifiers without requiring config.
	identifyQuery, args, err := buildIdentifyQueryFromIdentifiers(filters, es.scope(ctx))
	if err == nil && allHaveIdentifierRows {
		results, qErr := dbClient.QueryContext(ctx, identifyQuery, args...)
		if qErr == nil && len(results) == 1 {
			if entityID, ok := results[0]["id"].(string); ok {
				return &entityID, nil
			}
		}
		// Identifier rows are positive matches, so more than one entity for all-indexed filters is a
		// real ambiguity. The JSON fallback compares whole values and cannot see array items, so it
		// would report not found, or a single scalar holder, instead.
		if qErr == nil && len(results) > 1 && len(nonIndexedFilters) == 0 {
			return nil, ErrAmbiguousEntity
		}
	}

	// Fallback: JSONB search, using the identifier table for any indexed filters.
	var fallbackQuery dbmodel.DBQuery
	var fallbackArgs []interface{}

	if len(indexedFilters) > 0 && len(nonIndexedFilters) > 0 {
		// Mixed: identifier table for indexed filters + JSON for non-indexed filters.
		fallbackQuery, fallbackArgs, err = buildIdentifyQueryHybrid(indexedFilters, nonIndexedFilters, es.scope(ctx))
		if err != nil {
			return nil, fmt.Errorf("failed to build hybrid query: %w", err)
		}
	} else {
		// All-indexed: fast path already tried the identifier table; fall back to JSON search.
		// All non-indexed: always use JSON search.
		fallbackQuery, fallbackArgs, err = buildIdentifyQuery(filters, es.scope(ctx))
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
			es.logger.Debug(ctx, "Entity not found with the provided filters",
				log.MaskedMap("filters", filters))
		}
		return nil, ErrEntityNotFound
	}

	if len(results) != 1 {
		if es.logger.IsDebugEnabled() {
			es.logger.Debug(ctx,
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

// LockEntity holds the entity's write lock until the surrounding transaction ends and returns the
// entity as read under that lock.
func (es *entityDBStore) LockEntity(ctx context.Context, id string) (providers.Entity, error) {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return providers.Entity{}, fmt.Errorf("failed to get database client: %w", err)
	}

	rowsAffected, err := dbClient.ExecuteContext(ctx, QueryLockEntity, id, es.scope(ctx))
	if err != nil {
		return providers.Entity{}, fmt.Errorf("failed to execute query: %w", err)
	}
	if rowsAffected == 0 {
		return providers.Entity{}, ErrEntityNotFound
	}
	return es.GetEntity(ctx, id)
}

// ResolveLinkedAccount resolves the entity linked to a subject at a connection from the identifier
// index only.
func (es *entityDBStore) ResolveLinkedAccount(ctx context.Context, idpID, sub string) (*string, error) {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryResolveIdentifier,
		linkedIdentifierName(idpID), sub, identifierSourceSystem, es.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}

	if len(results) == 0 {
		return nil, ErrEntityNotFound
	}
	if len(results) > 1 {
		return nil, ErrAmbiguousEntity
	}

	entityID, ok := results[0]["id"].(string)
	if !ok || entityID == "" {
		return nil, fmt.Errorf("unexpected type for id: %T", results[0]["id"])
	}
	return &entityID, nil
}

// SearchEntities searches for all entities matching the provided filters.
// Unlike IdentifyEntity, this returns all matching entities instead of erroring on ambiguity.
// Results are capped at MaxPageSize (100) entries; matches beyond that limit are not returned.
// Column-level filters (category, ouId) should be handled at the service layer.
func (es *entityDBStore) SearchEntities(ctx context.Context,
	filters map[string]interface{}) ([]providers.Entity, error) {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	searchQuery, args, err := buildEntityListQuery(
		"", filters, es.indexedAttributes, serverconst.MaxPageSize, 0, es.scope(ctx))
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
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	countQuery, args, err := buildEntityCountQuery(category, filters, es.indexedAttributes, es.scope(ctx))
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}

	return executeCountQuery(dbClient, ctx, countQuery, args)
}

// GetEntityList retrieves a list of entities by category.
func (es *entityDBStore) GetEntityList(ctx context.Context, category string,
	limit, offset int, filters map[string]interface{}) ([]providers.Entity, error) {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	listQuery, args, err := buildEntityListQuery(
		category, filters, es.indexedAttributes, limit, offset, es.scope(ctx))
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
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	countQuery, args, err := buildEntityCountQueryByOUIDs(
		category, ouIDs, filters, es.indexedAttributes, es.scope(ctx))
	if err != nil {
		return 0, fmt.Errorf("failed to build count query: %w", err)
	}

	return executeCountQuery(dbClient, ctx, countQuery, args)
}

// GetEntityListByOUIDs retrieves a list of entities scoped to OU IDs.
func (es *entityDBStore) GetEntityListByOUIDs(ctx context.Context, category string,
	ouIDs []string, limit, offset int, filters map[string]interface{}) ([]providers.Entity, error) {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	listQuery, args, err := buildEntityListQueryByOUIDs(
		category, ouIDs, filters, es.indexedAttributes, limit, offset, es.scope(ctx))
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

	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query, args, err := buildBulkEntityExistsQuery(entityIDs, es.scope(ctx))
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
func (es *entityDBStore) GetEntitiesByIDs(ctx context.Context, entityIDs []string) ([]providers.Entity, error) {
	const batchSize = 100

	if len(entityIDs) == 0 {
		return []providers.Entity{}, nil
	}

	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	entities := make([]providers.Entity, 0, len(entityIDs))

	for start := 0; start < len(entityIDs); start += batchSize {
		end := start + batchSize
		if end > len(entityIDs) {
			end = len(entityIDs)
		}
		chunk := entityIDs[start:end]

		query, args, err := buildGetEntitiesByIDsQuery(chunk, es.scope(ctx))
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

	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	query, args, err := buildBulkEntityExistsQueryInOUs(entityIDs, ouIDs, es.scope(ctx))
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
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return 0, fmt.Errorf("failed to get database client: %w", err)
	}

	countResults, err := dbClient.QueryContext(ctx, QueryGetGroupCountForEntity, entityID, es.scope(ctx))
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
	ctx context.Context, entityID string, limit, offset int) ([]providers.EntityGroup, error) {
	dbClient, err := es.dbProvider.GetEntityDBClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get database client: %w", err)
	}

	results, err := dbClient.QueryContext(ctx, QueryGetGroupsForEntity,
		entityID, limit, offset, es.scope(ctx))
	if err != nil {
		return nil, fmt.Errorf("failed to get groups for entity: %w", err)
	}

	groups := make([]providers.EntityGroup, 0, len(results))
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
func buildEntityFromResultRow(row map[string]interface{}) (providers.Entity, error) {
	entityID, ok := row["id"].(string)
	if !ok {
		return providers.Entity{}, fmt.Errorf("failed to parse id as string")
	}

	ouID, ok := row["ou_id"].(string)
	if !ok {
		return providers.Entity{}, fmt.Errorf("failed to parse ou_id as string")
	}

	category, ok := row["category"].(string)
	if !ok {
		return providers.Entity{}, fmt.Errorf("failed to parse category as string")
	}

	entityType, ok := row["type"].(string)
	if !ok {
		return providers.Entity{}, fmt.Errorf("failed to parse type as string")
	}

	state, ok := row["state"].(string)
	if !ok {
		return providers.Entity{}, fmt.Errorf("failed to parse state as string")
	}

	var attributes string
	switch v := row["attributes"].(type) {
	case string:
		attributes = v
	case []byte:
		attributes = string(v)
	default:
		return providers.Entity{}, fmt.Errorf("failed to parse attributes as string")
	}

	entity := providers.Entity{
		ID:       entityID,
		Category: providers.EntityCategory(category),
		Type:     entityType,
		State:    providers.EntityState(state),
		OUID:     ouID,
	}

	if err := json.Unmarshal([]byte(attributes), &entity.Attributes); err != nil {
		return providers.Entity{}, fmt.Errorf("failed to unmarshal attributes")
	}

	entity.SystemAttributes = parseJSONColumn(row, "system_attributes")

	return entity, nil
}

func buildGroupFromResultRow(row map[string]interface{}) (providers.EntityGroup, error) {
	groupID, ok := row["id"].(string)
	if !ok {
		return providers.EntityGroup{}, fmt.Errorf("failed to parse id as string")
	}

	name, ok := row["name"].(string)
	if !ok {
		return providers.EntityGroup{}, fmt.Errorf("failed to parse name as string")
	}

	ouID, ok := row["ou_id"].(string)
	if !ok {
		return providers.EntityGroup{}, fmt.Errorf("failed to parse ou_id as string")
	}

	return providers.EntityGroup{ID: groupID, Name: name, OUID: ouID}, nil
}

func buildEntitiesFromResults(results []map[string]interface{}) ([]providers.Entity, error) {
	entities := make([]providers.Entity, 0, len(results))
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

// linkedIdentifierName derives the ENTITY_IDENTIFIER name a connection's links are indexed under.
func linkedIdentifierName(idpID string) string {
	return fmt.Sprintf("%s.%s", authnprovidercm.SystemAttrLinkedIDs, idpID)
}

// linkedIdentifierRows converts the linkedIds system attribute into one indexed identifier per
// subject. Malformed entries are skipped.
func linkedIdentifierRows(value interface{}) []indexedAttr {
	byIDP, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	var rows []indexedAttr
	for idpID, subjects := range byIDP {
		bySub, ok := subjects.(map[string]interface{})
		if idpID == "" || !ok {
			continue
		}
		for sub := range bySub {
			if sub == "" {
				continue
			}
			rows = append(rows, indexedAttr{
				name:   linkedIdentifierName(idpID),
				value:  sub,
				source: identifierSourceSystem,
			})
		}
	}
	return rows
}

func prepareIdentifierQuery(
	entityID string, attributes json.RawMessage, systemAttributes json.RawMessage,
	indexedAttrs map[string]bool, deploymentID string,
) (*dbmodel.DBQuery, []interface{}, error) {
	var attrEntries, sysEntries []indexedAttr

	// Extract indexed attributes from schema attributes (source = "attribute").
	// A value may be a scalar (one identifier value) or an array of scalars (multiple values).
	if len(attributes) > 0 {
		var attrMap map[string]interface{}
		if err := json.Unmarshal(attributes, &attrMap); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
		}
		for attrName, attrValue := range attrMap {
			if !indexedAttrs[attrName] {
				continue
			}
			for _, valueStr := range attrValueToStrings(attrValue) {
				attrEntries = append(attrEntries,
					indexedAttr{name: attrName, value: valueStr, source: identifierSourceAttribute})
			}
		}
	}

	// Extract indexed attributes from system attributes (source = "system").
	sysNames := make(map[string]bool)
	if len(systemAttributes) > 0 {
		var sysAttrMap map[string]interface{}
		if err := json.Unmarshal(systemAttributes, &sysAttrMap); err != nil {
			return nil, nil, fmt.Errorf("failed to unmarshal system attributes: %w", err)
		}
		for attrName, attrValue := range sysAttrMap {
			// Linked accounts are server-owned and indexed unconditionally. Gating them on
			// user.indexed_attributes would let a missing config line silently break sign-in through a link.
			if attrName == authnprovidercm.SystemAttrLinkedIDs {
				for _, row := range linkedIdentifierRows(attrValue) {
					sysNames[row.name] = true
					sysEntries = append(sysEntries, row)
				}
				continue
			}
			if !indexedAttrs[attrName] {
				continue
			}
			values := attrValueToStrings(attrValue)
			if len(values) > 0 {
				sysNames[attrName] = true
			}
			for _, valueStr := range values {
				sysEntries = append(sysEntries,
					indexedAttr{name: attrName, value: valueStr, source: identifierSourceSystem})
			}
		}
	}

	// If the same name has indexable values in both schema and system attributes, the system
	// attribute values win entirely (schema values for that name are dropped, not merged).
	toInsert := make([]indexedAttr, 0, len(attrEntries)+len(sysEntries))
	for _, attr := range attrEntries {
		if !sysNames[attr.name] {
			toInsert = append(toInsert, attr)
		}
	}
	toInsert = append(toInsert, sysEntries...)

	if len(toInsert) == 0 {
		return nil, nil, nil
	}

	// Deduplicate exact (name, value) repeats, preserving first-seen order.
	seen := make(map[string]bool, len(toInsert))
	deduped := make([]indexedAttr, 0, len(toInsert))
	for _, attr := range toInsert {
		key := attr.name + "\x00" + attr.value
		if seen[key] {
			continue
		}
		seen[key] = true
		deduped = append(deduped, attr)
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

// attrValueToStrings converts an attribute value into the identifier values to index.
// A scalar produces at most one value; an array produces one value per indexable element.
// Non-indexable elements (e.g. nested objects) are skipped.
func attrValueToStrings(value interface{}) []string {
	elements, ok := value.([]interface{})
	if !ok {
		if s := attrValueToString(value); s != "" {
			return []string{s}
		}
		return nil
	}

	values := make([]string, 0, len(elements))
	for _, elem := range elements {
		if s := attrValueToString(elem); s != "" {
			values = append(values, s)
		}
	}
	return values
}

// validateIndexedValueCounts rejects attributes in which an indexed name has more than
// maxIndexedValuesPerAttribute values to index. Linked accounts are indexed whatever the
// configuration, so their subjects are held to the same limit in total across every connection.
func validateIndexedValueCounts(attrMap map[string]interface{}, indexedAttrs map[string]bool) error {
	for attrName, attrValue := range attrMap {
		if attrName == authnprovidercm.SystemAttrLinkedIDs {
			if len(linkedIdentifierRows(attrValue)) > maxIndexedValuesPerAttribute {
				return fmt.Errorf("%w: %s holds more than %d links",
					ErrIndexedValueLimitExceeded, attrName, maxIndexedValuesPerAttribute)
			}
			continue
		}
		if indexedAttrs[attrName] && len(attrValueToStrings(attrValue)) > maxIndexedValuesPerAttribute {
			return fmt.Errorf("%w: indexed attribute '%s' has more than %d values",
				ErrIndexedValueLimitExceeded, attrName, maxIndexedValuesPerAttribute)
		}
	}
	return nil
}

func validateIndexedAttributesConfig(configuredAttrs []string) error {
	if len(configuredAttrs) > MaxIndexedAttributesCount {
		return fmt.Errorf("indexed attributes count (%d) must not exceed %d",
			len(configuredAttrs), MaxIndexedAttributesCount)
	}
	return nil
}

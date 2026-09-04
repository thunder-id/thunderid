// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/secretresolver"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// EntityServiceInterface is the interface for managing entities.
type EntityServiceInterface interface {
	// Core CRUD
	CreateEntity(ctx context.Context, entity *providers.Entity,
		systemCredentials json.RawMessage) (*providers.Entity, error)
	GetEntity(ctx context.Context, entityID string) (*providers.Entity, error)
	GetCredentialsByType(ctx context.Context, entityID string,
		credType string) ([]StoredCredential, error)
	UpdateEntity(ctx context.Context, entityID string, entity *providers.Entity) (*providers.Entity, error)
	DeleteEntity(ctx context.Context, entityID string) error

	// Partial updates
	UpdateAttributes(ctx context.Context, entityID string, attributes json.RawMessage) error
	UpdateSystemAttributes(ctx context.Context, entityID string, attrs json.RawMessage) error
	UpdateCredentials(ctx context.Context, entityID string,
		plaintextUpdates json.RawMessage) error
	UpdateSystemCredentials(ctx context.Context, entityID string,
		plaintextUpdates json.RawMessage) error

	// Identification
	IdentifyEntity(ctx context.Context, filters map[string]interface{}) (*string, error)
	SearchEntities(ctx context.Context, filters map[string]interface{}) ([]providers.Entity, error)

	// Lists (category-scoped)
	GetEntityListCount(ctx context.Context, category providers.EntityCategory,
		filters map[string]interface{}) (int, error)
	GetEntityList(ctx context.Context, category providers.EntityCategory,
		limit, offset int, filters map[string]interface{}) ([]providers.Entity, error)
	GetEntityListCountByOUIDs(ctx context.Context, category providers.EntityCategory,
		ouIDs []string, filters map[string]interface{}) (int, error)
	GetEntityListByOUIDs(ctx context.Context, category providers.EntityCategory,
		ouIDs []string, limit, offset int, filters map[string]interface{}) ([]providers.Entity, error)

	// Bulk
	ValidateEntityIDs(ctx context.Context, entityIDs []string) ([]string, error)
	GetEntitiesByIDs(ctx context.Context, entityIDs []string) ([]providers.Entity, error)
	ValidateEntityIDsInOUs(ctx context.Context, entityIDs []string, ouIDs []string) ([]string, error)

	// Groups
	GetGroupCountForEntity(ctx context.Context, entityID string) (int, error)
	GetEntityGroups(ctx context.Context, entityID string, limit, offset int) ([]providers.EntityGroup, error)
	GetTransitiveEntityGroups(ctx context.Context, entityID string) ([]providers.EntityGroup, error)

	// Authentication
	AuthenticateEntity(ctx context.Context, identifiers map[string]interface{},
		credentials map[string]interface{}) (*AuthenticateResult, error)
	AuthenticateEntityByID(ctx context.Context, entityID string,
		credentials map[string]interface{}) (*AuthenticateResult, error)

	// Declarative
	IsEntityDeclarative(ctx context.Context, entityID string) (bool, error)
	LoadDeclarativeResources(config DeclarativeLoaderConfig) error

	// Config
	LoadIndexedAttributes(attributes []string) error

	// GroupMembershipProvider registration
	SetGroupMembershipProvider(provider GroupMembershipProvider)
}

// GroupMembershipProvider resolves group memberships for entities. Implemented by the group
// package's store and injected after group initialization to avoid a circular import.
// Covers both DB-backed and declarative (YAML) group memberships.
type GroupMembershipProvider interface {
	GetTransitiveGroupsForEntity(ctx context.Context, entityID string) ([]providers.EntityGroup, error)
}

// entityService is the default implementation of EntityServiceInterface.
type entityService struct {
	store                   entityStoreInterface
	hashService             cryptolib.HashServiceInterface
	entityTypeService       entitytype.EntityTypeServiceInterface
	ouService               ou.OrganizationUnitServiceInterface
	transactioner           providers.Transactioner
	logger                  *log.Logger
	groupMembershipProvider GroupMembershipProvider
}

// usesEntityType reports whether entities of the given category route through the entity type
// infrastructure for attribute validation, credential extraction, and uniqueness checks.
func usesEntityType(category providers.EntityCategory) bool {
	return category == providers.EntityCategoryUser || category == providers.EntityCategoryAgent
}

// newEntityService creates a new entity service.
func newEntityService(
	store entityStoreInterface,
	hashService cryptolib.HashServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	transactioner providers.Transactioner,
) EntityServiceInterface {
	return &entityService{
		store:             store,
		hashService:       hashService,
		entityTypeService: entityTypeService,
		ouService:         ouService,
		transactioner:     transactioner,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EntityService")),
	}
}

// CreateEntity creates a new entity.
// Uses a transaction to ensure the entity row and its indexed identifiers are created atomically.
func (s *entityService) CreateEntity(ctx context.Context, entity *providers.Entity,
	systemCredentials json.RawMessage) (*providers.Entity, error) {
	if entity == nil {
		return nil, ErrEntityNotFound
	}

	if entity.ID == "" {
		id, err := sysutils.GenerateUUIDv7()
		if err != nil {
			return nil, fmt.Errorf("failed to generate entity ID: %w", err)
		}
		entity.ID = id
	}
	s.logger.Debug(ctx, "Creating entity", log.MaskedString("id", entity.ID))

	// Validate entity attributes and uniqueness via schema.
	if err := s.validateEntityType(ctx, entity.Category, entity.Type, entity.Attributes, "", false); err != nil {
		return nil, err
	}

	// Extract schema-defined credential fields from Attributes.
	schemaCredsJSON, err := s.extractAndHashSchemaCredentials(ctx, entity)
	if err != nil {
		return nil, fmt.Errorf("failed to extract schema credentials: %w", err)
	}

	// Hash plaintext system credentials.
	hashedSysCreds, err := s.hashPlaintextCredentials(systemCredentials)
	if err != nil {
		return nil, fmt.Errorf("failed to hash system credentials: %w", err)
	}

	var created providers.Entity
	err = s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.CreateEntity(txCtx, *entity, schemaCredsJSON, hashedSysCreds); err != nil {
			return err
		}

		result, err := s.store.GetEntity(txCtx, entity.ID)
		if err != nil {
			return err
		}
		created = result
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &created, nil
}

// GetEntity retrieves an entity by ID.
func (s *entityService) GetEntity(ctx context.Context, entityID string) (*providers.Entity, error) {
	entity, err := s.store.GetEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// GetCredentialsByType retrieves the slice of credentials matching the given credential type.
func (s *entityService) GetCredentialsByType(
	ctx context.Context, entityID string, credType string,
) ([]StoredCredential, error) {
	result, err := s.store.GetEntityWithCredentials(ctx, entityID)
	if err != nil {
		return nil, err
	}

	var creds []StoredCredential
	if len(result.SchemaCredentials) > 0 {
		var schemaMap map[string][]StoredCredential
		if err := json.Unmarshal(result.SchemaCredentials, &schemaMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal schema credentials: %w", err)
		}
		if v, ok := schemaMap[credType]; ok {
			creds = v
		}
	}
	if len(result.SystemCredentials) > 0 {
		var systemMap map[string][]StoredCredential
		if err := json.Unmarshal(result.SystemCredentials, &systemMap); err != nil {
			return nil, fmt.Errorf("failed to unmarshal system credentials: %w", err)
		}
		if v, ok := systemMap[credType]; ok {
			creds = v
		}
	}
	return creds, nil
}

// UpdateEntity updates an entity.
// Uses a transaction to ensure the entity update and identifier re-sync are atomic.
func (s *entityService) UpdateEntity(
	ctx context.Context, entityID string, entity *providers.Entity,
) (*providers.Entity, error) {
	if entity == nil {
		return nil, ErrEntityNotFound
	}
	s.logger.Debug(ctx, "Updating entity", log.MaskedString("id", entityID))

	// Drop stale attributes no longer in the schema so they don't block the update.
	cleanedAttrs, err := s.stripUndeclaredAttributes(ctx, entity.Category, entity.Type, entity.Attributes)
	if err != nil {
		return nil, err
	}
	entity.Attributes = cleanedAttrs

	// Validate entity attributes and uniqueness via schema (excludes self for uniqueness).
	if err := s.validateEntityType(ctx, entity.Category, entity.Type, entity.Attributes, entityID, true); err != nil {
		return nil, err
	}

	// Extract schema credentials from attributes.
	// These will be merged with existing credentials atomically.
	schemaCredsJSON, err := s.extractAndHashSchemaCredentials(ctx, entity)
	if err != nil {
		return nil, fmt.Errorf("failed to extract schema credentials: %w", err)
	}

	var updated providers.Entity
	err = s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		entity.ID = entityID
		preserved, prErr := s.mergeReservedAttributes(txCtx, entityID, entity.SystemAttributes)
		if prErr != nil {
			return prErr
		}
		entity.SystemAttributes = preserved

		if err := s.store.UpdateEntity(txCtx, entity); err != nil {
			return err
		}

		// Merge extracted schema credentials with existing credentials.
		if len(schemaCredsJSON) > 0 {
			existing, getErr := s.store.GetEntityWithCredentials(txCtx, entityID)
			if getErr != nil {
				return getErr
			}

			mergedCreds := mergeCredentialJSON(existing.SchemaCredentials, schemaCredsJSON)
			if err := s.store.UpdateCredentials(txCtx, entityID, mergedCreds); err != nil {
				return err
			}
		}

		result, err := s.store.GetEntity(txCtx, entityID)
		if err != nil {
			return err
		}
		updated = result
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &updated, nil
}

// DeleteEntity deletes an entity.
// Uses a transaction to ensure the entity row and its indexed identifiers are deleted atomically.
func (s *entityService) DeleteEntity(ctx context.Context, entityID string) error {
	s.logger.Debug(ctx, "Deleting entity", log.MaskedString("id", entityID))
	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.store.DeleteEntity(txCtx, entityID)
	})
	return err
}

// UpdateAttributes updates only the schema attributes of an entity.
// Any credential fields present in the attributes are extracted, hashed, and merged
// with the existing credentials atomically.
func (s *entityService) UpdateAttributes(ctx context.Context, entityID string, attributes json.RawMessage) error {
	s.logger.Debug(ctx, "Updating entity attributes", log.MaskedString("id", entityID))

	// Load entity to get its category and type for schema validation and credential extraction.
	existing, err := s.store.GetEntity(ctx, entityID)
	if err != nil {
		return err
	}

	// Drop stale attributes no longer in the schema so they don't block the update.
	attributes, err = s.stripUndeclaredAttributes(ctx, existing.Category, existing.Type, attributes)
	if err != nil {
		return err
	}

	// Validate attribute uniqueness via schema (excludes self, credentials not required for updates).
	if err := s.validateEntityType(ctx, existing.Category, existing.Type, attributes, entityID, true); err != nil {
		return err
	}

	// Extract and hash any schema-defined credential fields from the attributes.
	entityForExtraction := &providers.Entity{
		Category:   existing.Category,
		Type:       existing.Type,
		Attributes: attributes,
	}
	schemaCredsJSON, err := s.extractAndHashSchemaCredentials(ctx, entityForExtraction)
	if err != nil {
		return fmt.Errorf("failed to extract schema credentials: %w", err)
	}
	// entityForExtraction.Attributes has credential fields removed.
	cleanedAttrs := entityForExtraction.Attributes

	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := s.store.UpdateAttributes(txCtx, entityID, cleanedAttrs); err != nil {
			return err
		}

		// Merge extracted schema credentials with existing credentials.
		if len(schemaCredsJSON) > 0 {
			existingWithCreds, getErr := s.store.GetEntityWithCredentials(txCtx, entityID)
			if getErr != nil {
				return getErr
			}
			mergedCreds := mergeCredentialJSON(existingWithCreds.SchemaCredentials, schemaCredsJSON)
			return s.store.UpdateCredentials(txCtx, entityID, mergedCreds)
		}

		return nil
	})
}

// UpdateSystemAttributes updates the system-managed attributes of an entity.
func (s *entityService) UpdateSystemAttributes(ctx context.Context, entityID string,
	attrs json.RawMessage) error {
	s.logger.Debug(ctx, "Updating entity system attributes", log.MaskedString("id", entityID))
	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		preserved, err := s.mergeReservedAttributes(txCtx, entityID, attrs)
		if err != nil {
			return err
		}
		return s.store.UpdateSystemAttributes(txCtx, entityID, preserved)
	})
}

// IdentifyEntity identifies an entity using the given filters.
func (s *entityService) IdentifyEntity(ctx context.Context,
	filters map[string]interface{}) (*string, error) {
	id, err := s.store.IdentifyEntity(ctx, filters)
	if err != nil {
		return nil, err
	}
	return id, nil
}

// SearchEntities searches for all entities matching the provided filters. The returned
// entities have their OUHandle populated for presentation/disambiguation consumers.
func (s *entityService) SearchEntities(ctx context.Context,
	filters map[string]interface{}) ([]providers.Entity, error) {
	entities, err := s.store.SearchEntities(ctx, filters)
	if err != nil {
		return nil, err
	}
	s.populateOUHandles(ctx, entities)
	return entities, nil
}

// GetEntityListCount retrieves the total count of entities by category.
func (s *entityService) GetEntityListCount(ctx context.Context, category providers.EntityCategory,
	filters map[string]interface{}) (int, error) {
	return s.store.GetEntityListCount(ctx, string(category), filters)
}

// GetEntityList retrieves a list of entities by category.
func (s *entityService) GetEntityList(ctx context.Context, category providers.EntityCategory,
	limit, offset int, filters map[string]interface{}) ([]providers.Entity, error) {
	return s.store.GetEntityList(ctx, string(category), limit, offset, filters)
}

// GetEntityListCountByOUIDs retrieves the total count of entities scoped to OU IDs.
func (s *entityService) GetEntityListCountByOUIDs(ctx context.Context, category providers.EntityCategory,
	ouIDs []string, filters map[string]interface{}) (int, error) {
	return s.store.GetEntityListCountByOUIDs(ctx, string(category), ouIDs, filters)
}

// GetEntityListByOUIDs retrieves a list of entities scoped to OU IDs.
func (s *entityService) GetEntityListByOUIDs(ctx context.Context, category providers.EntityCategory,
	ouIDs []string, limit, offset int, filters map[string]interface{}) ([]providers.Entity, error) {
	return s.store.GetEntityListByOUIDs(ctx, string(category), ouIDs, limit, offset, filters)
}

// ValidateEntityIDs checks if all provided entity IDs exist.
func (s *entityService) ValidateEntityIDs(ctx context.Context, entityIDs []string) ([]string, error) {
	return s.store.ValidateEntityIDs(ctx, entityIDs)
}

// GetEntitiesByIDs retrieves entities by a list of IDs.
func (s *entityService) GetEntitiesByIDs(ctx context.Context, entityIDs []string) ([]providers.Entity, error) {
	return s.store.GetEntitiesByIDs(ctx, entityIDs)
}

// ValidateEntityIDsInOUs checks which of the provided entity IDs belong to the given OU scope.
func (s *entityService) ValidateEntityIDsInOUs(ctx context.Context,
	entityIDs []string, ouIDs []string) ([]string, error) {
	return s.store.ValidateEntityIDsInOUs(ctx, entityIDs, ouIDs)
}

// GetGroupCountForEntity retrieves the total count of groups an entity belongs to.
func (s *entityService) GetGroupCountForEntity(ctx context.Context, entityID string) (int, error) {
	return s.store.GetGroupCountForEntity(ctx, entityID)
}

// GetEntityGroups retrieves groups that an entity belongs to with pagination.
func (s *entityService) GetEntityGroups(ctx context.Context, entityID string,
	limit, offset int) ([]providers.EntityGroup, error) {
	return s.store.GetEntityGroups(ctx, entityID, limit, offset)
}

// SetGroupMembershipProvider registers the group store used to resolve all group memberships.
func (s *entityService) SetGroupMembershipProvider(provider GroupMembershipProvider) {
	s.groupMembershipProvider = provider
}

// GetTransitiveEntityGroups retrieves all groups an entity belongs to, including nested group membership.
// Delegates entirely to the group membership provider which covers both DB and declarative groups.
func (s *entityService) GetTransitiveEntityGroups(
	ctx context.Context, entityID string,
) ([]providers.EntityGroup, error) {
	if s.groupMembershipProvider == nil {
		return []providers.EntityGroup{}, nil
	}
	return s.groupMembershipProvider.GetTransitiveGroupsForEntity(ctx, entityID)
}

// AuthenticateEntity authenticates an entity by combining identify and verify operations.
// Identifiers are used to find the entity, and credentials are verified against stored credentials.
func (s *entityService) AuthenticateEntity(
	ctx context.Context,
	identifiers map[string]interface{},
	credentials map[string]interface{},
) (*AuthenticateResult, error) {
	if len(identifiers) == 0 {
		return nil, ErrEntityNotFound
	}
	if len(credentials) == 0 {
		return nil, ErrAuthenticationFailed
	}

	entityID, err := s.IdentifyEntity(ctx, identifiers)
	if err != nil {
		return nil, err
	}

	return s.AuthenticateEntityByID(ctx, *entityID, credentials)
}

// AuthenticateEntityByID authenticates an entity using its known primary key and the
// provided credentials. This skips the identification step, which is useful when the
// entity ID has already been resolved (e.g., after user disambiguation).
func (s *entityService) AuthenticateEntityByID(
	ctx context.Context,
	entityID string,
	credentials map[string]interface{},
) (*AuthenticateResult, error) {
	if entityID == "" {
		return nil, ErrEntityNotFound
	}
	if len(credentials) == 0 {
		return nil, ErrAuthenticationFailed
	}

	result, err := s.store.GetEntityWithCredentials(ctx, entityID)
	if err != nil {
		return nil, err
	}

	if result.Entity.State != providers.EntityStateActive {
		return nil, ErrEntityNotFound
	}

	if err := s.verifyCredentials(credentials, result.SchemaCredentials, result.SystemCredentials); err != nil {
		return nil, err
	}

	return &AuthenticateResult{
		EntityID:       result.Entity.ID,
		EntityCategory: result.Entity.Category,
		EntityType:     result.Entity.Type,
		OUID:           result.Entity.OUID,
	}, nil
}

// verifyCredentials verifies provided credentials from both schema and system credentials.
func (s *entityService) verifyCredentials(credentials map[string]interface{},
	schemaCredsJSON, systemCredsJSON json.RawMessage) error {
	// Merge both credential columns for verification.
	storedCreds := make(map[string][]StoredCredential)
	if len(schemaCredsJSON) > 0 {
		var schemaCreds map[string][]StoredCredential
		if err := json.Unmarshal(schemaCredsJSON, &schemaCreds); err != nil {
			return fmt.Errorf("failed to unmarshal schema credentials: %w", err)
		}
		for k, v := range schemaCreds {
			storedCreds[k] = v
		}
	}
	if len(systemCredsJSON) > 0 {
		var sysCreds map[string][]StoredCredential
		if err := json.Unmarshal(systemCredsJSON, &sysCreds); err != nil {
			return fmt.Errorf("failed to unmarshal system credentials: %w", err)
		}
		for k, v := range sysCreds {
			storedCreds[k] = v
		}
	}

	if len(storedCreds) == 0 {
		return ErrAuthenticationFailed
	}

	// Filter to credentials that have stored entries.
	credentialsToVerify := make(map[string]string)
	for credType, credValueInterface := range credentials {
		if _, exists := storedCreds[credType]; !exists {
			continue
		}
		credValue, ok := credValueInterface.(string)
		if !ok || credValue == "" {
			continue
		}
		credentialsToVerify[credType] = credValue
	}

	if len(credentialsToVerify) == 0 {
		return ErrAuthenticationFailed
	}

	// Verify each credential against stored values.
	for credType, credValue := range credentialsToVerify {
		credList := storedCreds[credType]
		verified := false
		for _, stored := range credList {
			ref, usable := credentialReference(stored)
			if !usable {
				continue
			}
			ok, verifyErr := s.hashService.Verify([]byte(credValue), ref)
			if verifyErr == nil && ok {
				verified = true
				break
			}
		}
		if !verified {
			return ErrAuthenticationFailed
		}
	}

	return nil
}

// credentialReference builds what a presented value is verified against.
//
// A promoted credential is stored as a reference, its hash living in the secret provider rather than
// the database, so it is resolved here and verification is the same comparison either way.
//
// usable is false when a reference cannot be resolved, which must reject rather than pass.
func credentialReference(stored StoredCredential) (cryptolib.Credential, bool) {
	if secretresolver.IsReference(stored.Value) {
		h, found, err := secretresolver.Default().ResolveHash(context.Background(), stored.Value)
		if err != nil || !found {
			return cryptolib.Credential{}, false
		}
		return cryptolib.Credential{
			Algorithm: cryptolib.CredAlgorithm(h.Algorithm),
			Hash:      h.Value,
			Parameters: cryptolib.CredParameters{
				Salt:        h.Salt,
				Iterations:  h.Iterations,
				KeySize:     h.KeySize,
				Memory:      h.Memory,
				Parallelism: h.Parallelism,
			},
		}, true
	}

	return cryptolib.Credential{
		Algorithm: stored.StorageAlgo,
		Hash:      stored.Value,
		Parameters: cryptolib.CredParameters{
			Salt:       stored.StorageAlgoParams.Salt,
			Iterations: stored.StorageAlgoParams.Iterations,
			KeySize:    stored.StorageAlgoParams.KeySize,
		},
	}, true
}

// UpdateCredentials updates schema-defined credentials (e.g., password) by hashing new
// plaintext values and merging with existing stored credentials. Payload keys are
// restricted to fields declared as credentials in the entity's schema.
func (s *entityService) UpdateCredentials(ctx context.Context, entityID string,
	plaintextUpdates json.RawMessage) error {
	if len(plaintextUpdates) == 0 {
		return nil
	}

	// Parse and validate new credential updates.
	var updates map[string]interface{}
	if err := json.Unmarshal(plaintextUpdates, &updates); err != nil {
		return fmt.Errorf("%w: failed to parse credentials", ErrInvalidCredential)
	}

	for credType, credValue := range updates {
		switch v := credValue.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("%w: empty value for credential type %q", ErrInvalidCredential, credType)
			}
		case nil:
			return fmt.Errorf("%w: nil value for credential type %q", ErrInvalidCredential, credType)
		default:
			_ = v
		}
	}

	// Load entity to route schema by category/type and enforce the credential-field allowlist.
	existing, err := s.store.GetEntity(ctx, entityID)
	if err != nil {
		return err
	}
	if err := s.validateCredentialKeys(ctx, existing.Category, existing.Type, updates); err != nil {
		return err
	}

	// Hash new plaintext values.
	hashedUpdates, err := s.hashPlaintextCredentials(plaintextUpdates)
	if err != nil {
		return fmt.Errorf("failed to hash credential updates: %w", err)
	}

	var hashedMap map[string]interface{}
	if err := json.Unmarshal(hashedUpdates, &hashedMap); err != nil {
		return fmt.Errorf("failed to unmarshal hashed updates: %w", err)
	}

	// Fetch existing, merge, and store.
	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existingWithCreds, err := s.store.GetEntityWithCredentials(txCtx, entityID)
		if err != nil {
			return err
		}

		existingCreds := make(map[string]interface{})
		if len(existingWithCreds.SchemaCredentials) > 0 {
			if err := json.Unmarshal(existingWithCreds.SchemaCredentials, &existingCreds); err != nil {
				return fmt.Errorf("failed to unmarshal existing credentials: %w", err)
			}
		}

		// Merge: existing preserved, new/updated types replaced.
		for k, v := range hashedMap {
			existingCreds[k] = v
		}

		mergedJSON, err := json.Marshal(existingCreds)
		if err != nil {
			return fmt.Errorf("failed to marshal merged credentials: %w", err)
		}

		if err := s.store.UpdateCredentials(txCtx, entityID, mergedJSON); err != nil {
			return err
		}

		// Record the change so the refresh grant can reject tokens established before it. Every
		// password change lands here, and the marker shares this transaction with the write.
		markedAttrs, err := setCredentialUpdatedAt(
			existingWithCreds.Entity.SystemAttributes, time.Now().UTC())
		if err != nil {
			return err
		}
		return s.store.UpdateSystemAttributes(txCtx, entityID, markedAttrs)
	})
}

// validateCredentialKeys rejects any payload key that isn't declared as a credential field
// in the entity's schema. Non-user categories are skipped until they get schema validation.
func (s *entityService) validateCredentialKeys(
	ctx context.Context, category providers.EntityCategory, entityType string, updates map[string]interface{},
) error {
	if !usesEntityType(category) || s.entityTypeService == nil {
		return nil
	}

	credInfos, svcErr := s.entityTypeService.GetAttributes(ctx,
		entitytype.TypeCategory(category), entityType, entitytype.AttributeFilter{AllowCredential: true})
	if svcErr != nil {
		return fmt.Errorf("failed to get credential attributes from schema: %s", svcErr.ErrorDescription)
	}
	allowed := make(map[string]struct{}, len(credInfos))
	for _, a := range credInfos {
		allowed[a.Attribute] = struct{}{}
	}
	for key := range updates {
		if _, ok := allowed[key]; !ok {
			return fmt.Errorf("%w: %q is not a declared credential", ErrInvalidCredential, key)
		}
	}
	return nil
}

// stripUndeclaredAttributes drops top-level attribute keys not declared in the entity type's current
// schema, so a schema changed after an entity was created (attribute renamed or removed) does not
// block updates. Only used on update; create still rejects undeclared attributes.
func (s *entityService) stripUndeclaredAttributes(
	ctx context.Context, category providers.EntityCategory, entityType string, attributes json.RawMessage,
) (json.RawMessage, error) {
	if !usesEntityType(category) || s.entityTypeService == nil || len(attributes) == 0 {
		return attributes, nil
	}

	attrInfos, svcErr := s.entityTypeService.GetAttributes(ctx,
		entitytype.TypeCategory(category), entityType,
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true})
	if svcErr != nil {
		return nil, fmt.Errorf("failed to get schema attributes: %s", svcErr.ErrorDescription)
	}
	// No declared attributes means an unconstrained schema, which Schema.Validate never rejects.
	if len(attrInfos) == 0 {
		return attributes, nil
	}

	declared := make(map[string]struct{}, len(attrInfos))
	for _, a := range attrInfos {
		declared[a.Attribute] = struct{}{}
	}

	var attrs map[string]json.RawMessage
	if err := json.Unmarshal(attributes, &attrs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attributes: %w", err)
	}

	dropped := 0
	for key := range attrs {
		if _, ok := declared[key]; !ok {
			delete(attrs, key)
			dropped++
		}
	}
	if dropped == 0 {
		return attributes, nil
	}

	s.logger.Debug(ctx, "Dropping attributes not declared in schema on update", log.Int("count", dropped))
	cleaned, err := json.Marshal(attrs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cleaned attributes: %w", err)
	}
	return cleaned, nil
}

// UpdateSystemCredentials updates system credentials by hashing new plaintext values and
// merging with existing stored credentials. Existing credential types not in the update
// are preserved.
func (s *entityService) UpdateSystemCredentials(ctx context.Context, entityID string,
	plaintextUpdates json.RawMessage) error {
	if len(plaintextUpdates) == 0 {
		return nil
	}

	// Parse and validate new credential updates.
	var updates map[string]interface{}
	if err := json.Unmarshal(plaintextUpdates, &updates); err != nil {
		return fmt.Errorf("%w: failed to parse credentials", ErrInvalidCredential)
	}

	for credType, credValue := range updates {
		switch v := credValue.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return fmt.Errorf("%w: empty value for credential type %q", ErrInvalidCredential, credType)
			}
		case []interface{}:
			// Structured credentials (e.g., passkey objects) — validate non-empty.
			if len(v) == 0 {
				return fmt.Errorf("%w: empty array for credential type %q", ErrInvalidCredential, credType)
			}
		case nil:
			return fmt.Errorf("%w: nil value for credential type %q", ErrInvalidCredential, credType)
		}
	}

	// Hash new plaintext values.
	hashedUpdates, err := s.hashPlaintextCredentials(plaintextUpdates)
	if err != nil {
		return fmt.Errorf("failed to hash credential updates: %w", err)
	}

	var hashedMap map[string]interface{}
	if err := json.Unmarshal(hashedUpdates, &hashedMap); err != nil {
		return fmt.Errorf("failed to unmarshal hashed updates: %w", err)
	}

	// Fetch existing, merge, and store.
	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		existing, err := s.store.GetEntityWithCredentials(txCtx, entityID)
		if err != nil {
			return err
		}

		existingCreds := make(map[string]interface{})
		if len(existing.SystemCredentials) > 0 {
			if err := json.Unmarshal(existing.SystemCredentials, &existingCreds); err != nil {
				return fmt.Errorf("failed to unmarshal existing credentials: %w", err)
			}
		}

		// Merge: existing preserved, new/updated types replaced.
		for k, v := range hashedMap {
			existingCreds[k] = v
		}

		mergedJSON, err := json.Marshal(existingCreds)
		if err != nil {
			return fmt.Errorf("failed to marshal merged credentials: %w", err)
		}

		if err := s.store.UpdateSystemCredentials(txCtx, entityID, mergedJSON); err != nil {
			return err
		}

		// Only a client secret rotation marks the entity. A passkey adds an authentication option
		// rather than replacing one, and the flow secret does not authenticate the client.
		if _, rotatesClientSecret := updates[authnprovidercm.CredentialTypeClientSecret]; !rotatesClientSecret {
			return nil
		}
		markedAttrs, err := setCredentialUpdatedAt(existing.Entity.SystemAttributes, time.Now().UTC())
		if err != nil {
			return err
		}
		return s.store.UpdateSystemAttributes(txCtx, entityID, markedAttrs)
	})
}

// populateOUHandles resolves OU handles for a slice of entities in-place.
func (s *entityService) populateOUHandles(ctx context.Context, entities []providers.Entity) {
	if s.ouService == nil || len(entities) == 0 {
		return
	}
	ouIDs := make([]string, 0, len(entities))
	seen := make(map[string]bool, len(entities))
	for i := range entities {
		if entities[i].OUID != "" && !seen[entities[i].OUID] {
			ouIDs = append(ouIDs, entities[i].OUID)
			seen[entities[i].OUID] = true
		}
	}
	if len(ouIDs) == 0 {
		return
	}
	handleMap, svcErr := s.ouService.GetOrganizationUnitHandlesByIDs(ctx, ouIDs)
	if svcErr != nil {
		s.logger.Warn(ctx, "Failed to resolve OU handles, skipping", log.Any("error", svcErr))
		return
	}
	for i := range entities {
		if handle, ok := handleMap[entities[i].OUID]; ok {
			entities[i].OUHandle = handle
		}
	}
}

// validateEntityType validates entity attributes and uniqueness against the entity type.
// excludeEntityID is used to exclude the entity itself from uniqueness
// checks during updates (empty string for creates). skipCredentialRequired controls whether
// credential fields are required (false for creates, true for updates).
func (s *entityService) validateEntityType(
	ctx context.Context,
	category providers.EntityCategory,
	entityType string,
	attributes json.RawMessage,
	excludeEntityID string,
	skipCredentialRequired bool,
) error {
	if !usesEntityType(category) || s.entityTypeService == nil {
		return nil
	}

	schemaCategory := entitytype.TypeCategory(category)

	// Validate attributes against schema (required fields, regex patterns, types).
	isValid, svcErr := s.entityTypeService.ValidateEntity(ctx, schemaCategory, entityType, attributes,
		skipCredentialRequired)
	if svcErr != nil {
		return fmt.Errorf("%w: %s", ErrSchemaValidationFailed, svcErr.ErrorDescription)
	}
	if !isValid {
		return ErrSchemaValidationFailed
	}

	// Validate attribute uniqueness
	isValid, svcErr = s.entityTypeService.ValidateEntityUniqueness(ctx, schemaCategory, entityType, attributes,
		func(filters map[string]interface{}) (bool, error) {
			id, err := s.IdentifyEntity(ctx, filters)
			if err != nil {
				if errors.Is(err, ErrEntityNotFound) {
					return false, nil // Not found = unique
				}
				if errors.Is(err, ErrAmbiguousEntity) {
					return true, nil // Multiple matches = definite conflict
				}
				return false, err
			}
			// Exclude self from uniqueness check during updates.
			if excludeEntityID != "" && id != nil && *id == excludeEntityID {
				return false, nil
			}
			return true, nil
		})
	if svcErr != nil {
		return fmt.Errorf("%w: %s", ErrAttributeConflict, svcErr.ErrorDescription)
	}
	if !isValid {
		return ErrAttributeConflict
	}

	return nil
}

// mergeReservedAttributes carries the reserved, server-owned keys of an entity's stored system
// attributes into a replacement blob. Both write paths replace the blob wholesale, and the services
// that own an entity rebuild it from their own model, so without this a rename would drop the
// credential-change marker this package writes and revive the tokens a credential change invalidated.
func (s *entityService) mergeReservedAttributes(ctx context.Context, entityID string,
	incoming json.RawMessage) (json.RawMessage, error) {
	current, err := s.store.GetEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}
	marker := credentialUpdatedAtOf(current.SystemAttributes)
	if marker == "" {
		return incoming, nil
	}

	attrs := map[string]interface{}{}
	if len(incoming) > 0 {
		if err := json.Unmarshal(incoming, &attrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal system attributes: %w", err)
		}
	}
	attrs[authnprovidercm.SystemAttrCredentialUpdatedAt] = marker

	merged, err := json.Marshal(attrs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal system attributes: %w", err)
	}
	return merged, nil
}

// credentialUpdatedAtOf returns the credential-change marker in the given system attributes, or empty
// when none is recorded.
func credentialUpdatedAtOf(systemAttributes json.RawMessage) string {
	if len(systemAttributes) == 0 {
		return ""
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(systemAttributes, &attrs); err != nil {
		return ""
	}
	marker, _ := attrs[authnprovidercm.SystemAttrCredentialUpdatedAt].(string)
	return marker
}

// setCredentialUpdatedAt returns systemAttributes with the credential-change marker set to at. The
// marker is merged in rather than replacing the blob, whose other keys belong to the service that
// owns the entity. Unlike mergeCredentialJSON, an unparsable blob is an error rather than a silent
// overwrite, since dropping those keys would go unnoticed.
func setCredentialUpdatedAt(systemAttributes json.RawMessage, at time.Time) (json.RawMessage, error) {
	attrs := map[string]interface{}{}
	if len(systemAttributes) > 0 {
		if err := json.Unmarshal(systemAttributes, &attrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal system attributes: %w", err)
		}
	}
	attrs[authnprovidercm.SystemAttrCredentialUpdatedAt] = at.UTC().Format(time.RFC3339)

	marked, err := json.Marshal(attrs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal system attributes: %w", err)
	}
	return marked, nil
}

// mergeCredentialJSON merges new credential JSON into existing credential JSON.
// New credential types replace existing ones; types not in the update are preserved.
func mergeCredentialJSON(existing, updates json.RawMessage) json.RawMessage {
	if len(updates) == 0 {
		return existing
	}
	if len(existing) == 0 {
		return updates
	}

	var existingMap map[string]interface{}
	if err := json.Unmarshal(existing, &existingMap); err != nil {
		return updates
	}

	var updatesMap map[string]interface{}
	if err := json.Unmarshal(updates, &updatesMap); err != nil {
		return existing
	}

	for k, v := range updatesMap {
		existingMap[k] = v
	}

	merged, err := json.Marshal(existingMap)
	if err != nil {
		return updates
	}
	return merged
}

// extractAndHashSchemaCredentials extracts schema-defined credential fields from entity.Attributes,
// hashes them, and returns the hashed credentials.
func (s *entityService) extractAndHashSchemaCredentials(
	ctx context.Context, entity *providers.Entity,
) (json.RawMessage, error) {
	// User and agent entities both use schema-defined credentials for now.
	if !usesEntityType(entity.Category) {
		return nil, nil
	}

	if s.entityTypeService == nil || len(entity.Attributes) == 0 {
		return nil, nil
	}

	credentialInfos, svcErr := s.entityTypeService.GetAttributes(ctx,
		entitytype.TypeCategory(entity.Category), entity.Type, entitytype.AttributeFilter{AllowCredential: true})
	if svcErr != nil {
		return nil, fmt.Errorf("failed to get credential attributes from schema: %s", svcErr.ErrorDescription)
	}

	if len(credentialInfos) == 0 {
		return nil, nil
	}

	var attrsMap map[string]interface{}
	if err := json.Unmarshal(entity.Attributes, &attrsMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal entity attributes: %w", err)
	}

	plaintextCreds := make(map[string]string)
	for _, info := range credentialInfos {
		if val, ok := attrsMap[info.Attribute].(string); ok && val != "" {
			plaintextCreds[info.Attribute] = val
			delete(attrsMap, info.Attribute)
		}
	}

	if len(plaintextCreds) == 0 {
		return nil, nil
	}

	// Update entity.Attributes with credentials removed.
	cleanAttrs, err := json.Marshal(attrsMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cleaned attributes: %w", err)
	}
	entity.Attributes = cleanAttrs

	// Hash and return as JSON.
	plaintextJSON, err := json.Marshal(plaintextCreds)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plaintext credentials: %w", err)
	}

	return s.hashPlaintextCredentials(plaintextJSON)
}

// hashPlaintextCredentials processes system credentials JSON, hashing any plaintext values.
// Values that are already in the stored format (arrays of credential objects) are passed through as-is.
// This allows declarative resource loaders to pre-hash credentials.
func (s *entityService) hashPlaintextCredentials(creds json.RawMessage) (json.RawMessage, error) {
	if len(creds) == 0 {
		return creds, nil
	}

	var credsMap map[string]interface{}
	if err := json.Unmarshal(creds, &credsMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	if len(credsMap) == 0 {
		return creds, nil
	}

	result := make(map[string]interface{}, len(credsMap))
	for credType, credValue := range credsMap {
		switch v := credValue.(type) {
		case string:
			// Plaintext string value — hash it.
			if v == "" {
				continue
			}
			// A reference is not a credential to hash: the hash it points at lives in this
			// deployment's secret provider. Hashing it here would store the hash of the reference
			// text and every authentication would fail, so it is kept as it is and resolved when a
			// presented value is verified.
			if secretresolver.IsReference(v) {
				result[credType] = []StoredCredential{{Value: v}}
				continue
			}
			credHash, err := s.hashService.Generate([]byte(v))
			if err != nil {
				return nil, fmt.Errorf("failed to hash credential %q: %w", credType, err)
			}
			result[credType] = []StoredCredential{
				{
					StorageAlgo: credHash.Algorithm,
					StorageAlgoParams: cryptolib.CredParameters{
						Salt:       credHash.Parameters.Salt,
						Iterations: credHash.Parameters.Iterations,
						KeySize:    credHash.Parameters.KeySize,
					},
					Value: credHash.Hash,
				},
			}
		default:
			// Already in stored format (array of credential objects) — pass through.
			result[credType] = credValue
		}
	}

	return json.Marshal(result)
}

// IsEntityDeclarative checks if an entity is declarative (immutable).
func (s *entityService) IsEntityDeclarative(ctx context.Context, entityID string) (bool, error) {
	return s.store.IsEntityDeclarative(ctx, entityID)
}

// LoadDeclarativeResources loads declarative resources for a given entity category.
// Consumer packages provide parser/validator callbacks for type-specific YAML processing.
func (s *entityService) LoadDeclarativeResources(config DeclarativeLoaderConfig) error {
	return loadDeclarativeResources(s.store, s, config)
}

// LoadIndexedAttributes loads attributes to be indexed for fast lookups.
// Consumers call this at startup to declare which of their attributes should be indexed.
func (s *entityService) LoadIndexedAttributes(attributes []string) error {
	return s.store.LoadIndexedAttributes(attributes)
}

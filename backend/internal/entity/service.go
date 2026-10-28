// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/identitygovernance"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/log"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// EntityServiceInterface is the interface for managing entities.
type EntityServiceInterface interface {
	// Core CRUD
	CreateEntity(ctx context.Context, entity *providers.Entity,
		systemCredentials json.RawMessage, link *providers.LinkedAccount) (*providers.Entity, error)
	GetEntity(ctx context.Context, entityID string) (*providers.Entity, error)
	// GetEntityProfile reads an entity without its runtime state. State is empty.
	GetEntityProfile(ctx context.Context, entityID string) (*providers.Entity, error)
	GetCredentialsByType(ctx context.Context, entityID string,
		credType string) ([]StoredCredential, error)
	UpdateEntity(ctx context.Context, entityID string, entity *providers.Entity) (*providers.Entity, error)
	DeleteEntity(ctx context.Context, entityID string) error

	// Partial updates
	UpdateAttributes(ctx context.Context, entityID string, attributes json.RawMessage) error
	UpdateSystemAttributes(ctx context.Context, entityID string, attrs json.RawMessage) error

	// Credentials
	UpdateCredentials(ctx context.Context, entityID string,
		plaintextUpdates json.RawMessage) error
	UpdateSystemCredentials(ctx context.Context, entityID string,
		plaintextUpdates json.RawMessage) error

	// Identification
	IdentifyEntity(ctx context.Context, filters map[string]interface{}) (*string, error)
	ResolveLinkedAccounts(ctx context.Context, idpID, sub string) ([]string, error)
	GetLinkedEntities(ctx context.Context, idpID, sub string) ([]providers.Entity, error)
	LinkAccount(ctx context.Context, entityID, idpID, sub string) error
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
	// GetEntityPageInOU lists the entities of a category an organization unit owns, in the store's list
	// order, then the ones shared into it.
	GetEntityPageInOU(ctx context.Context, category providers.EntityCategory, ouID string,
		scope OUEntityScope, limit, offset int, filters map[string]interface{}) (*OUEntityPage, error)
	GetEntityListCountBySystemAttributes(ctx context.Context, category providers.EntityCategory,
		sysQuery SystemAttributeQuery) (int, error)
	GetEntityListBySystemAttributes(ctx context.Context, category providers.EntityCategory,
		sysQuery SystemAttributeQuery, limit, offset int) ([]providers.Entity, error)
	GetEntityListCountBySystemAttributesAndOUIDs(ctx context.Context, category providers.EntityCategory,
		sysQuery SystemAttributeQuery, ouIDs []string) (int, error)
	GetEntityListBySystemAttributesAndOUIDs(ctx context.Context, category providers.EntityCategory,
		sysQuery SystemAttributeQuery, ouIDs []string, limit, offset int) ([]providers.Entity, error)

	// Bulk
	ValidateEntityIDs(ctx context.Context, entityIDs []string) ([]string, error)
	GetEntitiesByIDs(ctx context.Context, entityIDs []string) ([]providers.Entity, error)
	ValidateEntityIDsInOUs(ctx context.Context, entityIDs []string, ouIDs []string) ([]string, error)

	// Groups
	GetGroupCountForEntity(ctx context.Context, entityID string) (int, error)
	GetEntityGroups(ctx context.Context, entityID string, limit, offset int) ([]providers.EntityGroup, error)
	GetTransitiveEntityGroups(ctx context.Context, entityID string) ([]providers.EntityGroup, error)
	// GetTransitiveGroupAncestors resolves the ancestor chain of a single group.
	GetTransitiveGroupAncestors(ctx context.Context, groupID string) ([]string, error)

	// Authentication
	AuthenticateEntity(ctx context.Context, identifiers map[string]interface{},
		credentials map[string]interface{}) (*AuthenticateResult, string, error)
	AuthenticateEntityByID(ctx context.Context, entityID string,
		credentials map[string]interface{}) (*AuthenticateResult, string, error)

	// Declarative
	IsEntityDeclarative(ctx context.Context, entityID string) (bool, error)
	LoadDeclarativeResources(config DeclarativeLoaderConfig) error

	// Config
	LoadIndexedAttributes(attributes []string) error

	// GroupMembershipProvider registration
	SetGroupMembershipProvider(provider GroupMembershipProvider)

	// Sharing registration
	// RegisterSharedCategory makes entities of a category shareable under the given sharing resource
	// type, so an organization unit's entity pages list the ones shared into it.
	RegisterSharedCategory(category providers.EntityCategory, resourceType sharing.ResourceType)
	// GetSharedResourceType returns the sharing resource type a category is shareable under, and
	// false when the category cannot be shared.
	GetSharedResourceType(category providers.EntityCategory) (sharing.ResourceType, bool)

	// Runtime state, used by the governance service.
	GetGovernedEntity(ctx context.Context, entityID string,
		seedMissing bool) (*identitygovernance.GovernedEntity, error)
	IncrementFailure(ctx context.Context, entityID string, method providers.AuthenticationMethod,
		now, windowStart time.Time) (identitygovernance.LockEntry, bool, error)
	FormLock(ctx context.Context, entityID string, method providers.AuthenticationMethod,
		observed identitygovernance.LockEntry, episode identitygovernance.LockEpisode, now time.Time) (bool, error)
	ClearAccessState(ctx context.Context, entityID string, methods []providers.AuthenticationMethod) error
	ClearAccessStateIfUnchanged(ctx context.Context, entityID string, method providers.AuthenticationMethod,
		revision int64) (bool, error)
	RecordLogin(ctx context.Context, entityID string, at, notBefore time.Time) error
	SetSuspension(ctx context.Context, entityID string, at time.Time, operatorNote string) (bool, error)
	ClearSuspension(ctx context.Context, entityID string, hold *identitygovernance.AccountHold,
		suspendedAt string) (bool, error)
}

// GroupMembershipProvider resolves group memberships for entities. Implemented by the group
// package's store and injected after group initialization to avoid a circular import.
// Covers both DB-backed and declarative (YAML) group memberships.
type GroupMembershipProvider interface {
	GetTransitiveGroupsForEntity(ctx context.Context, entityID string) ([]providers.EntityGroup, error)
	GetTransitiveAncestorGroups(ctx context.Context, groupID string) ([]string, error)
}

// entityService is the default implementation of EntityServiceInterface.
type entityService struct {
	store                   entityStoreInterface
	runtimeStore            entityRuntimeStoreInterface
	hashService             cryptolib.HashServiceInterface
	entityTypeService       entitytype.EntityTypeServiceInterface
	ouService               ou.OrganizationUnitServiceInterface
	sharingService          sharing.SharingServiceInterface
	sharedTypes             map[providers.EntityCategory]sharing.ResourceType
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
	runtimeStore entityRuntimeStoreInterface,
	hashService cryptolib.HashServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	ouService ou.OrganizationUnitServiceInterface,
	transactioner providers.Transactioner,
	sharingService sharing.SharingServiceInterface,
) EntityServiceInterface {
	return &entityService{
		store:             store,
		runtimeStore:      runtimeStore,
		hashService:       hashService,
		entityTypeService: entityTypeService,
		ouService:         ouService,
		sharingService:    sharingService,
		sharedTypes:       map[providers.EntityCategory]sharing.ResourceType{},
		transactioner:     transactioner,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "EntityService")),
	}
}

// CreateEntity creates a new entity, linked to the identity provider subject link names when it is
// set. Links come only from link: any linkedIds in the entity's system attributes are dropped.
// Uses a transaction to ensure the entity row and its indexed identifiers are created atomically.
func (s *entityService) CreateEntity(ctx context.Context, entity *providers.Entity,
	systemCredentials json.RawMessage, link *providers.LinkedAccount) (*providers.Entity, error) {
	if entity == nil {
		return nil, ErrEntityNotFound
	}
	if link != nil && (link.IdpID == "" || link.Sub == "") {
		return nil, ErrBadAttributesInRequest
	}
	sysAttrs, err := withoutLinkedIDs(entity.SystemAttributes)
	if err != nil {
		return nil, err
	}
	if link != nil {
		if sysAttrs, err = withLinkedSubject(sysAttrs, link.IdpID, link.Sub); err != nil {
			return nil, err
		}
	}
	entity.SystemAttributes = sysAttrs

	idSupplied := entity.ID != ""
	if !idSupplied {
		id, err := sysutils.GenerateUUIDv7()
		if err != nil {
			return nil, fmt.Errorf("failed to generate entity ID: %w", err)
		}
		entity.ID = id
	}
	s.logger.Debug(ctx, "Creating entity", log.MaskedString("id", entity.ID))

	if err := s.validateIndexedValueLimit(entity.Attributes); err != nil {
		return nil, err
	}
	if err := s.validateIndexedValueLimit(entity.SystemAttributes); err != nil {
		return nil, err
	}

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

	if idSupplied {
		if err := s.checkIDNotDeclarative(ctx, entity.ID); err != nil {
			return nil, err
		}
	}

	entity.State = providers.EntityStateActive
	entity.RuntimeAttributes = nil
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

	// The runtime row is in another database, so it is written after the entity commits. A failed
	// write is logged and the create stands: a missing row reads as the starting state.
	if err := s.runtimeStore.ResetRuntimeData(ctx, entity.ID, entity.State); err != nil {
		s.logger.Warn(ctx, "Failed to reset entity runtime data", log.MaskedString("id", entity.ID),
			log.Error(err))
	}

	created.State = entity.State
	created.RuntimeAttributes = json.RawMessage(`{}`)
	return &created, nil
}

// GetEntity retrieves an entity by ID.
func (s *entityService) GetEntity(ctx context.Context, entityID string) (*providers.Entity, error) {
	entity, err := s.store.GetEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}
	entities := []providers.Entity{entity}
	if err := s.populateRuntime(ctx, entities); err != nil {
		return nil, err
	}
	return &entities[0], nil
}

// GetEntityProfile reads an entity without its runtime state. State is empty.
func (s *entityService) GetEntityProfile(ctx context.Context, entityID string) (*providers.Entity, error) {
	profile, err := s.store.GetEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}
	profile.State = ""
	profile.RuntimeAttributes = nil
	return &profile, nil
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
// The entity's state is not written: lifecycle state lives in the runtime store.
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

	if err := s.validateIndexedValueLimit(entity.Attributes); err != nil {
		return nil, err
	}
	if err := s.validateIndexedValueLimit(entity.SystemAttributes); err != nil {
		return nil, err
	}

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

	// The runtime row is in another database and the update never changes it, so it is read before
	// the transaction rather than while it holds the entity row.
	runtimeRecords, err := s.runtimeStore.GetRuntimeData(ctx, []string{entityID})
	if err != nil {
		return nil, err
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

	entities := []providers.Entity{updated}
	applyRuntimeData(entities, runtimeRecords)
	return &entities[0], nil
}

// DeleteEntity deletes an entity.
// Uses a transaction to ensure the entity row and its indexed identifiers are deleted atomically.
func (s *entityService) DeleteEntity(ctx context.Context, entityID string) error {
	s.logger.Debug(ctx, "Deleting entity", log.MaskedString("id", entityID))
	err := s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return s.store.DeleteEntity(txCtx, entityID)
	})
	if err != nil {
		return err
	}

	// Deleted after the entity commit, so a failed entity delete never loses its holds. A row left
	// behind is reset by a later create at the same id.
	if err := s.runtimeStore.DeleteRuntimeData(ctx, entityID); err != nil {
		s.logger.Warn(ctx, "Failed to delete entity runtime data", log.MaskedString("id", entityID),
			log.Error(err))
	}
	return nil
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

	if err := s.validateIndexedValueLimit(attributes); err != nil {
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
	if err := s.validateIndexedValueLimit(attrs); err != nil {
		return err
	}
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

// ResolveLinkedAccounts returns the entities linked to an identity provider subject. A subject no
// entity is linked to returns an empty list.
func (s *entityService) ResolveLinkedAccounts(ctx context.Context, idpID, sub string) ([]string, error) {
	if idpID == "" || sub == "" {
		return nil, ErrBadAttributesInRequest
	}
	return s.store.ResolveLinkedAccounts(ctx, idpID, sub)
}

// GetLinkedEntities returns the entities linked to an identity provider subject, with their OUHandle
// populated for disambiguation consumers.
func (s *entityService) GetLinkedEntities(ctx context.Context, idpID, sub string) ([]providers.Entity, error) {
	entityIDs, err := s.ResolveLinkedAccounts(ctx, idpID, sub)
	if err != nil {
		return nil, err
	}
	entities, err := s.store.GetEntitiesByIDs(ctx, entityIDs)
	if err != nil {
		return nil, err
	}
	s.populateOUHandles(ctx, entities)
	return entities, nil
}

// LinkAccount records that an entity authenticates as the given subject at the given connection.
// Other entities may hold the same subject.
func (s *entityService) LinkAccount(ctx context.Context, entityID, idpID, sub string) error {
	if entityID == "" || idpID == "" || sub == "" {
		return ErrBadAttributesInRequest
	}
	s.logger.Debug(ctx, "Linking account", log.MaskedString("id", entityID),
		log.String(log.LoggerKeyIDPID, idpID))

	return s.transactioner.Transact(ctx, func(txCtx context.Context) error {
		current, err := s.store.LockEntity(txCtx, entityID)
		if err != nil {
			return err
		}

		merged, err := withLinkedSubject(current.SystemAttributes, idpID, sub)
		if err != nil || merged == nil {
			return err
		}
		return s.store.UpdateSystemAttributes(txCtx, entityID, merged)
	})
}

// SearchEntities searches for all entities matching the provided filters. The returned
// entities have their OUHandle populated for presentation/disambiguation consumers.
func (s *entityService) SearchEntities(ctx context.Context,
	filters map[string]interface{}) ([]providers.Entity, error) {
	entities, err := s.store.SearchEntities(ctx, filters)
	if err != nil {
		return nil, err
	}
	if err := s.populateRuntime(ctx, entities); err != nil {
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
	entities, err := s.store.GetEntityList(ctx, string(category), limit, offset, filters)
	if err != nil {
		return nil, err
	}
	if err := s.populateRuntime(ctx, entities); err != nil {
		return nil, err
	}
	return entities, nil
}

// GetEntityListCountByOUIDs retrieves the total count of entities scoped to OU IDs.
func (s *entityService) GetEntityListCountByOUIDs(ctx context.Context, category providers.EntityCategory,
	ouIDs []string, filters map[string]interface{}) (int, error) {
	return s.store.GetEntityListCountByOUIDs(ctx, string(category), ouIDs, filters)
}

// GetEntityListByOUIDs retrieves a list of entities scoped to OU IDs.
func (s *entityService) GetEntityListByOUIDs(ctx context.Context, category providers.EntityCategory,
	ouIDs []string, limit, offset int, filters map[string]interface{}) ([]providers.Entity, error) {
	entities, err := s.store.GetEntityListByOUIDs(ctx, string(category), ouIDs, limit, offset, filters)
	if err != nil {
		return nil, err
	}
	if err := s.populateRuntime(ctx, entities); err != nil {
		return nil, err
	}
	return entities, nil
}

// GetEntityPageInOU retrieves one page of the entities of a category an organization unit owns,
// followed by the ones shared into it, as the scope selects. Only the page's entities are read.
func (s *entityService) GetEntityPageInOU(ctx context.Context, category providers.EntityCategory, ouID string,
	scope OUEntityScope, limit, offset int, filters map[string]interface{}) (*OUEntityPage, error) {
	// A shared entity is matched on its own values and the sharee's overlays, which no filter reads
	// yet. Refusing keeps a page from listing shared entities a filter would have left out.
	if len(filters) > 0 && scope != OUEntityScopeOwned {
		return nil, errSharedEntityFilterUnsupported
	}
	ouIDs := []string{ouID}
	ownedCount := 0
	if scope != OUEntityScopeShared {
		count, err := s.store.GetEntityListCountByOUIDs(ctx, string(category), ouIDs, filters)
		if err != nil {
			return nil, err
		}
		ownedCount = count
	}

	var sharedIDs []string
	if scope != OUEntityScopeOwned {
		ids, err := s.sharedEntityIDs(ctx, category, ouID)
		if err != nil {
			return nil, err
		}
		sharedIDs = ids
	}

	owned, shared, err := sysutils.PaginateOwnedThenShared(
		ownedCount,
		func(limit, offset int) ([]providers.Entity, error) {
			return s.store.GetEntityListByOUIDs(ctx, string(category), ouIDs, limit, offset, filters)
		},
		sharedIDs,
		func(ids []string) ([]providers.Entity, error) {
			entities, err := s.store.GetEntitiesByIDs(ctx, ids)
			if err != nil {
				return nil, err
			}
			return slices.DeleteFunc(entities, func(e providers.Entity) bool {
				return e.Category != category
			}), nil
		},
		func(e providers.Entity) string { return e.ID },
		limit, offset,
	)
	if err != nil {
		return nil, err
	}

	page := slices.Concat(owned, shared)
	if err := s.populateRuntime(ctx, page); err != nil {
		return nil, err
	}
	return &OUEntityPage{
		Owned:        page[:len(owned)],
		Shared:       page[len(owned):],
		TotalResults: ownedCount + len(sharedIDs),
	}, nil
}

// sharedEntityIDs returns the entities of a category shared into an organization unit, sorted by ID.
// A category that cannot be shared, or an organization unit that does not exist, has none.
func (s *entityService) sharedEntityIDs(
	ctx context.Context, category providers.EntityCategory, ouID string,
) ([]string, error) {
	resourceType, ok := s.sharedTypes[category]
	if !ok {
		return nil, nil
	}
	ids, svcErr := s.sharingService.ListVisibleResourceIDs(ctx, resourceType, ouID)
	if svcErr != nil {
		if svcErr.Code == ou.ErrorOrganizationUnitNotFound.Code {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to list %s entities shared into organization unit: %s",
			category, svcErr.Error.DefaultValue)
	}
	return ids, nil
}

// GetEntityListCountBySystemAttributes retrieves the number of entities a system attribute query selects.
func (s *entityService) GetEntityListCountBySystemAttributes(ctx context.Context,
	category providers.EntityCategory, sysQuery SystemAttributeQuery) (int, error) {
	return s.store.GetEntityListCountBySystemAttributes(ctx, string(category), sysQuery)
}

// GetEntityListBySystemAttributes retrieves the entities a system attribute query selects.
func (s *entityService) GetEntityListBySystemAttributes(ctx context.Context,
	category providers.EntityCategory, sysQuery SystemAttributeQuery, limit, offset int) (
	[]providers.Entity, error) {
	entities, err := s.store.GetEntityListBySystemAttributes(ctx, string(category), sysQuery, limit, offset)
	if err != nil {
		return nil, err
	}
	if err := s.populateRuntime(ctx, entities); err != nil {
		return nil, err
	}
	return entities, nil
}

// GetEntityListCountBySystemAttributesAndOUIDs retrieves the number of entities a system attribute query
// selects within the given organization units.
func (s *entityService) GetEntityListCountBySystemAttributesAndOUIDs(ctx context.Context,
	category providers.EntityCategory, sysQuery SystemAttributeQuery, ouIDs []string) (int, error) {
	return s.store.GetEntityListCountBySystemAttributesAndOUIDs(ctx, string(category), sysQuery, ouIDs)
}

// GetEntityListBySystemAttributesAndOUIDs retrieves the entities a system attribute query selects within
// the given organization units.
func (s *entityService) GetEntityListBySystemAttributesAndOUIDs(ctx context.Context,
	category providers.EntityCategory, sysQuery SystemAttributeQuery, ouIDs []string, limit, offset int) (
	[]providers.Entity, error) {
	entities, err := s.store.GetEntityListBySystemAttributesAndOUIDs(
		ctx, string(category), sysQuery, ouIDs, limit, offset)
	if err != nil {
		return nil, err
	}
	if err := s.populateRuntime(ctx, entities); err != nil {
		return nil, err
	}
	return entities, nil
}

// ValidateEntityIDs checks if all provided entity IDs exist.
func (s *entityService) ValidateEntityIDs(ctx context.Context, entityIDs []string) ([]string, error) {
	return s.store.ValidateEntityIDs(ctx, entityIDs)
}

// GetEntitiesByIDs retrieves entities by a list of IDs.
func (s *entityService) GetEntitiesByIDs(ctx context.Context, entityIDs []string) ([]providers.Entity, error) {
	entities, err := s.store.GetEntitiesByIDs(ctx, entityIDs)
	if err != nil {
		return nil, err
	}
	if err := s.populateRuntime(ctx, entities); err != nil {
		return nil, err
	}
	return entities, nil
}

// ValidateEntityIDsInOUs checks which of the provided entity IDs belong to the given OU method.
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

// RegisterSharedCategory makes entities of a category shareable under the given sharing resource type.
// It is called while the server starts, before any page is read.
func (s *entityService) RegisterSharedCategory(category providers.EntityCategory, resourceType sharing.ResourceType) {
	s.sharedTypes[category] = resourceType
}

// GetSharedResourceType returns the sharing resource type a category is shareable under.
func (s *entityService) GetSharedResourceType(category providers.EntityCategory) (sharing.ResourceType, bool) {
	resourceType, ok := s.sharedTypes[category]
	return resourceType, ok
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

// GetTransitiveGroupAncestors resolves the ancestor chain of a single group.
func (s *entityService) GetTransitiveGroupAncestors(
	ctx context.Context, groupID string,
) ([]string, error) {
	if s.groupMembershipProvider == nil {
		return []string{}, nil
	}
	return s.groupMembershipProvider.GetTransitiveAncestorGroups(ctx, groupID)
}

// AuthenticateEntity authenticates an entity by combining identify and verify operations.
// Identifiers are used to find the entity, and credentials are verified against stored credentials.
func (s *entityService) AuthenticateEntity(
	ctx context.Context,
	identifiers map[string]interface{},
	credentials map[string]interface{},
) (*AuthenticateResult, string, error) {
	if len(identifiers) == 0 {
		return nil, "", ErrEntityNotFound
	}
	if len(credentials) == 0 {
		return nil, "", ErrAuthenticationFailed
	}

	entityID, err := s.IdentifyEntity(ctx, identifiers)
	if err != nil {
		return nil, "", err
	}

	return s.AuthenticateEntityByID(ctx, *entityID, credentials)
}

// AuthenticateEntityByID authenticates an entity using its known primary key and the
// provided credentials. This skips the identification step, which is useful when the
// entity ID has already been resolved (e.g., after user disambiguation). On a credential that
// was compared and did not match, it also returns the ID of the entity the credential belongs to.
func (s *entityService) AuthenticateEntityByID(
	ctx context.Context,
	entityID string,
	credentials map[string]interface{},
) (*AuthenticateResult, string, error) {
	if entityID == "" {
		return nil, "", ErrEntityNotFound
	}
	if len(credentials) == 0 {
		return nil, "", ErrAuthenticationFailed
	}

	result, err := s.store.GetEntityWithCredentials(ctx, entityID)
	if err != nil {
		return nil, "", err
	}

	mismatched, err := s.verifyCredentials(ctx, credentials, result.SchemaCredentials, result.SystemCredentials)
	if err != nil {
		if !mismatched {
			return nil, "", err
		}
		return nil, result.Entity.ID, err
	}

	return &AuthenticateResult{
		EntityID:       result.Entity.ID,
		EntityCategory: result.Entity.Category,
		EntityType:     result.Entity.Type,
		OUID:           result.Entity.OUID,
	}, "", nil
}

// verifyCredentials verifies provided credentials from both schema and system credentials.
// The first return reports a credential that matched no stored candidate.
func (s *entityService) verifyCredentials(ctx context.Context, credentials map[string]interface{},
	schemaCredsJSON, systemCredsJSON json.RawMessage) (bool, error) {
	// Merge both credential columns for verification.
	storedCreds := make(map[string][]StoredCredential)
	if len(schemaCredsJSON) > 0 {
		var schemaCreds map[string][]StoredCredential
		if err := json.Unmarshal(schemaCredsJSON, &schemaCreds); err != nil {
			return false, fmt.Errorf("failed to unmarshal schema credentials: %w", err)
		}
		for k, v := range schemaCreds {
			storedCreds[k] = v
		}
	}
	if len(systemCredsJSON) > 0 {
		var sysCreds map[string][]StoredCredential
		if err := json.Unmarshal(systemCredsJSON, &sysCreds); err != nil {
			return false, fmt.Errorf("failed to unmarshal system credentials: %w", err)
		}
		for k, v := range sysCreds {
			storedCreds[k] = v
		}
	}

	if len(storedCreds) == 0 {
		return false, ErrAuthenticationFailed
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
		return false, ErrAuthenticationFailed
	}

	// Verify each credential against stored values, in a fixed order. A mismatch decides over an
	// indeterminate result.
	indeterminateFailure := false
	for _, credType := range slices.Sorted(maps.Keys(credentialsToVerify)) {
		verified, indeterminate := s.verifyOneCredential(ctx, credType, credentialsToVerify[credType],
			storedCreds[credType])
		if verified {
			continue
		}
		if indeterminate {
			indeterminateFailure = true
			continue
		}
		return true, ErrAuthenticationFailed
	}

	if indeterminateFailure {
		return false, ErrAuthenticationFailed
	}
	return false, nil
}

// verifyOneCredential compares one supplied value against every stored candidate of its type. It
// reports whether one matched, and whether any candidate could not be evaluated.
func (s *entityService) verifyOneCredential(ctx context.Context, credType, credValue string,
	credList []StoredCredential) (bool, bool) {
	indeterminate := false
	for _, stored := range credList {
		ref := cryptolib.Credential{
			Algorithm:  stored.StorageAlgo,
			Hash:       stored.Value,
			Parameters: stored.StorageAlgoParams,
		}
		ok, verifyErr := s.hashService.Verify([]byte(credValue), ref)
		if verifyErr != nil {
			s.logger.Debug(ctx, "Credential verification error", log.String("credentialType", credType),
				log.Any("error", verifyErr))
			indeterminate = true
			continue
		}
		if ok {
			return true, false
		}
	}
	return false, indeterminate
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
		// password change lands here, and the marker shares this transaction with the write. The blob
		// is read under the entity's lock, since a cached copy could drop a concurrent link write.
		locked, err := s.store.LockEntity(txCtx, entityID)
		if err != nil {
			return err
		}
		markedAttrs, err := setCredentialUpdatedAt(locked.SystemAttributes, time.Now().UTC())
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
		locked, err := s.store.LockEntity(txCtx, entityID)
		if err != nil {
			return err
		}
		markedAttrs, err := setCredentialUpdatedAt(locked.SystemAttributes, time.Now().UTC())
		if err != nil {
			return err
		}
		return s.store.UpdateSystemAttributes(txCtx, entityID, markedAttrs)
	})
}

// withoutLinkedIDs returns systemAttributes without their linkedIds, unchanged when there are none.
func withoutLinkedIDs(systemAttributes json.RawMessage) (json.RawMessage, error) {
	if len(systemAttributes) == 0 {
		return systemAttributes, nil
	}
	attrs := map[string]interface{}{}
	if err := json.Unmarshal(systemAttributes, &attrs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal system attributes: %w", err)
	}
	if _, ok := attrs[authnprovidercm.SystemAttrLinkedIDs]; !ok {
		return systemAttributes, nil
	}
	delete(attrs, authnprovidercm.SystemAttrLinkedIDs)
	stripped, err := json.Marshal(attrs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal system attributes: %w", err)
	}
	return stripped, nil
}

// withLinkedSubject returns systemAttributes with the subject added to its linkedIds at the
// connection, or nil when the subject is already there.
func withLinkedSubject(systemAttributes json.RawMessage, idpID, sub string) (json.RawMessage, error) {
	attrs := map[string]interface{}{}
	if len(systemAttributes) > 0 {
		if err := json.Unmarshal(systemAttributes, &attrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal system attributes: %w", err)
		}
	}

	links, err := objectAt(attrs, authnprovidercm.SystemAttrLinkedIDs)
	if err != nil {
		return nil, err
	}
	if hasLinkedSubject(links, idpID, sub) {
		return nil, nil
	}
	if len(linkedIdentifierRows(links)) >= maxIndexedValuesPerAttribute {
		return nil, fmt.Errorf("%w: %s holds at most %d links", ErrIndexedValueLimitExceeded,
			authnprovidercm.SystemAttrLinkedIDs, maxIndexedValuesPerAttribute)
	}

	subjects, err := objectAt(links, idpID)
	if err != nil {
		return nil, err
	}
	subjects[sub] = map[string]interface{}{}
	links[idpID] = subjects
	attrs[authnprovidercm.SystemAttrLinkedIDs] = links

	merged, err := json.Marshal(attrs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal system attributes: %w", err)
	}
	return merged, nil
}

// hasLinkedSubject reports whether a subject is already recorded for a connection.
func hasLinkedSubject(links map[string]interface{}, idpID, sub string) bool {
	subjects, _ := links[idpID].(map[string]interface{})
	_, ok := subjects[sub]
	return ok
}

// objectAt returns the object stored under key, or an empty one when the key is absent or null. A
// value of any other shape is an error, since replacing it would silently drop what it holds.
func objectAt(m map[string]interface{}, key string) (map[string]interface{}, error) {
	raw := m[key]
	if raw == nil {
		return map[string]interface{}{}, nil
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("stored linked account entry %q is not an object", key)
	}
	return obj, nil
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

// validateIndexedValueLimit rejects input attributes in which an indexed name has more values than
// an entity may index. It checks input only: stored data is re-indexed as is on every write, so an
// entity stored over the limit stays updatable.
func (s *entityService) validateIndexedValueLimit(attributes json.RawMessage) error {
	if len(attributes) == 0 {
		return nil
	}
	var attrMap map[string]interface{}
	if err := json.Unmarshal(attributes, &attrMap); err != nil {
		return fmt.Errorf("%w: %w", ErrSchemaValidationFailed, err)
	}
	if err := validateIndexedValueCounts(attrMap, s.store.GetIndexedAttributes()); err != nil {
		return fmt.Errorf("%w: %w", ErrSchemaValidationFailed, err)
	}
	return nil
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

// checkIDNotDeclarative refuses a caller-supplied id that a declarative account holds. Creating it in
// the database would shadow that entity and reset its runtime row.
func (s *entityService) checkIDNotDeclarative(ctx context.Context, entityID string) error {
	declarative, err := s.store.IsEntityDeclarative(ctx, entityID)
	if err != nil && !errors.Is(err, ErrEntityNotFound) {
		return err
	}
	if declarative {
		return fmt.Errorf("%w: id is held by a declarative entity", ErrAttributeConflict)
	}
	return nil
}

// mergeReservedAttributes carries the reserved, server-owned keys of an entity's stored system
// attributes into a replacement blob. Both write paths replace the blob wholesale, and the services
// that own an entity rebuild it from their own model, so without this a rename would drop the
// credential-change marker this package writes and revive the tokens a credential change invalidated.
// A reserved key the caller sends is replaced by the stored value, or dropped when none is stored, so
// only this package writes them. The entity is read under its lock rather than from the cache, so a
// concurrent write to a reserved key is carried across instead of being overwritten.
func (s *entityService) mergeReservedAttributes(ctx context.Context, entityID string,
	incoming json.RawMessage) (json.RawMessage, error) {
	current, err := s.store.LockEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}
	preserved, err := reservedAttributesOf(current.SystemAttributes)
	if err != nil {
		return nil, err
	}

	var attrs map[string]json.RawMessage
	if len(incoming) > 0 {
		if err := json.Unmarshal(incoming, &attrs); err != nil {
			return nil, fmt.Errorf("failed to unmarshal system attributes: %w", err)
		}
	}
	if attrs == nil {
		attrs = map[string]json.RawMessage{}
	}

	changed := false
	for _, key := range reservedSystemAttributes {
		value, stored := preserved[key]
		if !stored {
			if _, sent := attrs[key]; sent {
				delete(attrs, key)
				changed = true
			}
			continue
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal system attribute %q: %w", key, err)
		}
		attrs[key] = raw
		changed = true
	}
	if !changed {
		return incoming, nil
	}

	merged, err := json.Marshal(attrs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal system attributes: %w", err)
	}
	return merged, nil
}

// reservedAttributesOf returns the reserved keys present in the given system attributes.
// An unparsable blob is an error rather than an empty result, since replacing it would silently drop
// the reserved keys it holds.
func reservedAttributesOf(systemAttributes json.RawMessage) (map[string]interface{}, error) {
	if len(systemAttributes) == 0 {
		return nil, nil
	}
	var attrs map[string]interface{}
	if err := json.Unmarshal(systemAttributes, &attrs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stored system attributes: %w", err)
	}

	preserved := map[string]interface{}{}
	for _, key := range reservedSystemAttributes {
		if value, ok := attrs[key]; ok {
			preserved[key] = value
		}
	}
	return preserved, nil
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
			credHash, err := s.hashService.Generate([]byte(v))
			if err != nil {
				return nil, fmt.Errorf("failed to hash credential %q: %w", credType, err)
			}
			result[credType] = []StoredCredential{
				{
					StorageAlgo:       credHash.Algorithm,
					StorageAlgoParams: credHash.Parameters,
					Value:             credHash.Hash,
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

// GetGovernedEntity reads the profile, a fresh runtime row and its parsed access state. It returns nil
// when no entity has the ID. A document that cannot be parsed fails the read. seedMissing creates a
// missing row; without it the read writes nothing.
func (s *entityService) GetGovernedEntity(ctx context.Context,
	entityID string, seedMissing bool) (*identitygovernance.GovernedEntity, error) {
	entity, err := s.store.GetEntity(ctx, entityID)
	if errors.Is(err, ErrEntityNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	record, err := s.governedRuntimeData(ctx, entity, seedMissing)
	if err != nil {
		return nil, err
	}
	entity.State = record.State
	entity.RuntimeAttributes = record.Attributes

	governed, err := governedEntityOf(entity)
	if err != nil {
		return nil, err
	}
	governed.Revision = record.Revision

	return governed, nil
}

// IncrementFailure records one failed attempt against a method. The second return is false when
// nothing was written: the entity has no runtime row or a lock is live.
func (s *entityService) IncrementFailure(ctx context.Context, entityID string, method providers.AuthenticationMethod,
	now, windowStart time.Time) (identitygovernance.LockEntry, bool, error) {
	return s.runtimeStore.IncrementAccessFailure(ctx, entityID, method, now, windowStart)
}

// FormLock opens a lock episode, guarded on the counters the increment observed. It returns false
// when the row changed since then.
func (s *entityService) FormLock(ctx context.Context, entityID string, method providers.AuthenticationMethod,
	observed identitygovernance.LockEntry, episode identitygovernance.LockEpisode, now time.Time) (bool, error) {
	return s.runtimeStore.FormAccessLock(ctx, entityID, method, observed, episode, now)
}

// ClearAccessState removes the given methods' failure and lock state.
func (s *entityService) ClearAccessState(ctx context.Context, entityID string,
	methods []providers.AuthenticationMethod) error {
	return s.runtimeStore.ClearAccessState(ctx, entityID, methods)
}

// ClearAccessStateIfUnchanged removes one method's state only if the runtime row still has the given
// revision, and reports whether it did.
func (s *entityService) ClearAccessStateIfUnchanged(ctx context.Context, entityID string,
	method providers.AuthenticationMethod, revision int64) (bool, error) {
	return s.runtimeStore.ClearAccessStateIfUnchanged(ctx, entityID, method, revision)
}

// RecordLogin stores when access was last granted, unless the stored value is already within
// [notBefore, at].
func (s *entityService) RecordLogin(ctx context.Context, entityID string,
	at, notBefore time.Time) error {
	return s.runtimeStore.RecordAccessLogin(ctx, entityID, at, notBefore)
}

// SetSuspension suspends the entity, recording when and the operator's note. It reports false when
// the entity is already suspended.
func (s *entityService) SetSuspension(ctx context.Context, entityID string,
	at time.Time, operatorNote string) (bool, error) {
	return s.runtimeStore.SetAccessSuspension(ctx, entityID, at, operatorNote)
}

// ClearSuspension removes the suspension placed at suspendedAt, optionally replacing it with an
// account-wide hold in the same write. It reports false when that suspension is no longer there.
func (s *entityService) ClearSuspension(ctx context.Context, entityID string,
	hold *identitygovernance.AccountHold, suspendedAt string) (bool, error) {
	return s.runtimeStore.ClearAccessSuspension(ctx, entityID, hold, suspendedAt)
}

// populateRuntime sets the runtime state and attributes on each entity, read outside the profile
// cache. It never writes: an entity with no row reads as the row it starts with.
func (s *entityService) populateRuntime(ctx context.Context, entities []providers.Entity) error {
	if len(entities) == 0 {
		return nil
	}

	ids := make([]string, len(entities))
	for i := range entities {
		ids[i] = entities[i].ID
	}

	records, err := s.runtimeStore.GetRuntimeData(ctx, ids)
	if err != nil {
		return err
	}

	applyRuntimeData(entities, records)
	return nil
}

// governedRuntimeData reads the entity's runtime row; a missing row reads as the starting row. Only the
// governance read seeds, with seedMissing, since every runtime write follows it and needs a row to update.
func (s *entityService) governedRuntimeData(ctx context.Context,
	entity providers.Entity, seedMissing bool) (entityRuntimeData, error) {
	ids := []string{entity.ID}
	records, err := s.runtimeStore.GetRuntimeData(ctx, ids)
	if err != nil {
		return entityRuntimeData{}, err
	}
	if record, ok := records[entity.ID]; ok {
		return record, nil
	}
	if !seedMissing {
		return startingRuntimeData(entity), nil
	}

	seed := []entityRuntimeData{startingRuntimeData(entity)}
	if err := s.runtimeStore.InitializeRuntimeData(ctx, seed); err != nil {
		return entityRuntimeData{}, err
	}

	// Read the seeded row back: a row another request wrote first wins over this seed.
	records, err = s.runtimeStore.GetRuntimeData(ctx, ids)
	if err != nil {
		return entityRuntimeData{}, err
	}
	record, ok := records[entity.ID]
	if !ok {
		return entityRuntimeData{}, fmt.Errorf("entity runtime row disappeared during initialization")
	}

	return record, nil
}

// governedEntityOf narrows a hydrated entity to the part governance needs.
func governedEntityOf(entity providers.Entity) (*identitygovernance.GovernedEntity, error) {
	var stored identitygovernance.RuntimeAttributes
	if len(entity.RuntimeAttributes) > 0 {
		if err := json.Unmarshal(entity.RuntimeAttributes, &stored); err != nil {
			return nil, fmt.Errorf("failed to unmarshal runtime attributes: %w", err)
		}
	}

	return &identitygovernance.GovernedEntity{
		ID:          entity.ID,
		Category:    entity.Category,
		Type:        entity.Type,
		OUID:        entity.OUID,
		State:       entity.State,
		AccessState: stored.AccessState,
	}, nil
}

// applyRuntimeData sets each entity's runtime state and attributes from its row, or from the row it
// starts with when it has none.
func applyRuntimeData(entities []providers.Entity, records map[string]entityRuntimeData) {
	for i := range entities {
		record, ok := records[entities[i].ID]
		if !ok {
			record = startingRuntimeData(entities[i])
		}
		entities[i].State = record.State
		entities[i].RuntimeAttributes = record.Attributes
	}
}

// startingRuntimeData is the row an entity starts with: the declarative file's state, or ACTIVE.
func startingRuntimeData(entity providers.Entity) entityRuntimeData {
	state := entity.State
	if state == "" {
		state = providers.EntityStateActive
	}
	return entityRuntimeData{ID: entity.ID, State: state, Attributes: json.RawMessage(`{}`)}
}

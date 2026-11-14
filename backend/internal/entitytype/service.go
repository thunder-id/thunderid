// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package entitytype handles the entity type management operations.
package entitytype

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"unicode/utf8"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entitytype/model"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	entityTypeLoggerComponentName = "EntityTypeService"
	maxDisplayNameLength          = 100
)

// AttributeInfo is an alias for model.AttributeInfo, exported at the entitytype package
// level so callers do not need to import the internal model package directly.
type AttributeInfo = model.AttributeInfo

// AttributeFilter is an alias for model.AttributeFilter, exported at the entitytype package
// level so callers do not need to import the internal model package directly.
type AttributeFilter = model.AttributeFilter

// EntityTypeServiceInterface defines the interface for the entity type service.
type EntityTypeServiceInterface interface {
	GetEntityTypeList(ctx context.Context, category TypeCategory, limit, offset int,
		includeDisplay bool) (*EntityTypeListResponse, *tidcommon.ServiceError)
	CreateEntityType(
		ctx context.Context, category TypeCategory, request CreateEntityTypeRequestWithID,
	) (*EntityType, *tidcommon.ServiceError)
	GetEntityType(ctx context.Context, category TypeCategory, schemaID string,
		includeDisplay bool) (*EntityType, *tidcommon.ServiceError)
	GetEntityTypeByHandle(
		ctx context.Context, category TypeCategory, handle string,
	) (*EntityType, *tidcommon.ServiceError)
	UpdateEntityType(ctx context.Context, category TypeCategory, schemaID string,
		request UpdateEntityTypeRequest) (
		*EntityType, *tidcommon.ServiceError)
	DeleteEntityType(ctx context.Context, category TypeCategory,
		schemaID string) *tidcommon.ServiceError
	ValidateEntity(
		ctx context.Context, category TypeCategory, entityType string, attributes json.RawMessage,
		skipCredentialRequired bool,
	) (bool, *tidcommon.ServiceError)
	ValidateEntityValues(
		ctx context.Context, category TypeCategory, entityType string, attributes json.RawMessage,
	) (bool, *tidcommon.ServiceError)
	ValidateEntityUniqueness(
		ctx context.Context,
		category TypeCategory,
		entityType string,
		attributes json.RawMessage,
		exists func(map[string]interface{}) (bool, error),
	) (bool, *tidcommon.ServiceError)
	GetAttributes(
		ctx context.Context, category TypeCategory, entityType string, filter AttributeFilter,
	) ([]AttributeInfo, *tidcommon.ServiceError)
	GetAttributesForEntityType(
		ctx context.Context, entityType string, filter AttributeFilter,
	) (map[TypeCategory][]AttributeInfo, *tidcommon.ServiceError)
	GetUniqueAttributes(
		ctx context.Context, category TypeCategory, entityType string,
	) ([]string, *tidcommon.ServiceError)
	GetDisplayAttributesByHandles(
		ctx context.Context, category TypeCategory, handles []string,
	) (map[string]string, *tidcommon.ServiceError)
	ResolveEntityTypeHandles(ctx context.Context, entityType *EntityType) *tidcommon.ServiceError
	GetEntityTypeSchema(
		ctx context.Context, category TypeCategory, handle string,
	) (*EntityType, *tidcommon.ServiceError)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	SetDependencyRegistry(r resourcedependency.Registry)
}

// entityTypeService is the default implementation of the EntityTypeServiceInterface.
type entityTypeService struct {
	entityTypeStore    entityTypeStoreInterface
	ouService          oupkg.OrganizationUnitServiceInterface
	transactioner      providers.Transactioner
	authzService       sysauthz.SystemAuthorizationServiceInterface
	bus                eventbus.EventBusInterface
	dependencyRegistry resourcedependency.Registry
	// managed says which types a control plane deployed, which stay read only here. Nil where nothing
	// is deployed from one.
	managed managedresource.StoreInterface
}

// setManagedStore gives the service the record of what a control plane deployed.
func setManagedStore(service EntityTypeServiceInterface, managed managedresource.StoreInterface) {
	if us, ok := service.(*entityTypeService); ok {
		us.managed = managed
	}
}

// managedType is the type a control plane marks the types of a category with.
func managedType(category TypeCategory) string {
	switch category {
	case TypeCategoryAgent:
		return managedresource.TypeAgentType
	case TypeCategoryOU:
		return managedresource.TypeOUType
	default:
		return managedresource.TypeUserType
	}
}

// refuseManaged refuses a change to a type a control plane deployed, unless the change is the managed
// import's: only a deploy from the control plane changes or removes it.
func (us *entityTypeService) refuseManaged(
	ctx context.Context, category TypeCategory, schemaID string) *tidcommon.ServiceError {
	managed, err := managedresource.IsReadOnly(ctx, us.managed, schemaID, managedType(category))
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to read whether a control plane deployed the type",
			log.String("id", schemaID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if managed {
		return &ErrorManagedEntityType
	}
	return nil
}

// withManaged reports a type a control plane deployed as read only. A failed read of that is logged and
// taken as not: every write to such a type is refused whatever the read says.
func (us *entityTypeService) withManaged(ctx context.Context, category TypeCategory, entityType *EntityType) {
	deployed, err := managedresource.IsDeployed(ctx, us.managed, entityType.ID, managedType(category))
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to read whether a control plane deployed the type",
			log.String("id", entityType.ID), log.Error(err))
	}
	entityType.IsReadOnly = deployed
}

// newEntityTypeService creates a new instance of entityTypeService.
func newEntityTypeService(
	ouService oupkg.OrganizationUnitServiceInterface,
	store entityTypeStoreInterface,
	transactioner providers.Transactioner,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	bus eventbus.EventBusInterface,
) EntityTypeServiceInterface {
	return &entityTypeService{
		entityTypeStore: store,
		ouService:       ouService,
		transactioner:   transactioner,
		authzService:    authzService,
		bus:             bus,
	}
}

// GetEntityTypeList lists entity types for the given category with pagination.
func (us *entityTypeService) GetEntityTypeList(ctx context.Context, category TypeCategory,
	limit, offset int, includeDisplay bool) (
	*EntityTypeListResponse, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	accessible, svcErr := us.getAccessibleResources(ctx, category)
	if svcErr != nil {
		return nil, svcErr
	}

	var list *EntityTypeListResponse
	if accessible.AllAllowed {
		logger.Debug(ctx, "Caller has access to all entity types, retrieving without OU filtering",
			log.String("category", string(category)))
		list, svcErr = us.listAllEntityTypes(ctx, category, limit, offset, includeDisplay, logger)
	} else {
		list, svcErr = us.listAccessibleEntityTypes(ctx, category, accessible.IDs, limit, offset,
			includeDisplay, logger)
	}
	if list != nil {
		if err := managedresource.MarkDeployed(ctx, us.managed, list.Types,
			func(t *EntityTypeListItem) string { return t.ID },
			func(t *EntityTypeListItem) { t.IsReadOnly = true }, managedType(category)); err != nil {
			// Every write to such a type is refused whatever the list says, so it is served without the flag.
			logger.Error(ctx, "Failed to read which types a control plane deployed", log.Error(err))
		}
	}
	return list, svcErr
}

// listAllEntityTypes retrieves entity types without authorization filtering.
func (us *entityTypeService) listAllEntityTypes(
	ctx context.Context, category TypeCategory, limit, offset int, includeDisplay bool, logger *log.Logger,
) (*EntityTypeListResponse, *tidcommon.ServiceError) {
	totalCount, err := us.entityTypeStore.GetEntityTypeListCount(ctx, category)
	if err != nil {
		return nil, logAndReturnServerError(ctx, logger, "Failed to get entity type list count", err)
	}

	entityTypes, err := us.entityTypeStore.GetEntityTypeList(ctx, category, limit, offset)
	if err != nil {
		return nil, logAndReturnServerError(ctx, logger, "Failed to get entity type list", err)
	}

	if includeDisplay {
		us.populateEntityTypeOUHandles(ctx, entityTypes, logger)
	}

	return &EntityTypeListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(entityTypes),
		Types:        entityTypes,
		Links: buildPaginationLinks(category, limit, offset, totalCount,
			utils.DisplayQueryParam(includeDisplay)),
	}, nil
}

// listAccessibleEntityTypes retrieves only the entity types belonging to the caller's accessible OUs.
func (us *entityTypeService) listAccessibleEntityTypes(
	ctx context.Context, category TypeCategory, ouIDs []string, limit, offset int,
	includeDisplay bool, logger *log.Logger,
) (*EntityTypeListResponse, *tidcommon.ServiceError) {
	displayQuery := utils.DisplayQueryParam(includeDisplay)

	if len(ouIDs) == 0 {
		return &EntityTypeListResponse{
			TotalResults: 0,
			StartIndex:   offset + 1,
			Count:        0,
			Types:        []EntityTypeListItem{},
			Links:        buildPaginationLinks(category, limit, offset, 0, displayQuery),
		}, nil
	}

	totalCount, err := us.entityTypeStore.GetEntityTypeListCountByOUIDs(ctx, category, ouIDs)
	if err != nil {
		return nil, logAndReturnServerError(ctx, logger, "Failed to get accessible entity type count", err)
	}

	entityTypes, err := us.entityTypeStore.GetEntityTypeListByOUIDs(ctx, category, ouIDs, limit, offset)
	if err != nil {
		return nil, logAndReturnServerError(ctx, logger, "Failed to get accessible entity type list", err)
	}

	if includeDisplay {
		us.populateEntityTypeOUHandles(ctx, entityTypes, logger)
	}

	return &EntityTypeListResponse{
		TotalResults: totalCount,
		StartIndex:   offset + 1,
		Count:        len(entityTypes),
		Types:        entityTypes,
		Links:        buildPaginationLinks(category, limit, offset, totalCount, displayQuery),
	}, nil
}

// CreateEntityType creates a new entity type in the given category.
func (us *entityTypeService) CreateEntityType(
	ctx context.Context, category TypeCategory, request CreateEntityTypeRequestWithID,
) (*EntityType, *tidcommon.ServiceError) {
	created, svcErr := us.createEntityType(ctx, category, request)
	var id string
	if svcErr == nil {
		id = created.ID
	}
	us.publishEntityTypeEvent(ctx, category, createEventForCategory(category), audit.ActionCreate, id, svcErr)
	return created, svcErr
}

// createEntityType creates a new entity type in the given category without publishing its audit event.
func (us *entityTypeService) createEntityType(
	ctx context.Context, category TypeCategory, request CreateEntityTypeRequestWithID,
) (*EntityType, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if isDeclarativeModeEnabled() {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	ouOnly := &EntityType{ID: request.ID, Handle: request.Handle, OUID: request.OUID, OUHandle: request.OUHandle}
	if svcErr := us.resolveEntityTypeOUHandle(ctx, ouOnly); svcErr != nil {
		return nil, invalidEntityTypeRequestErr(category, "organization unit with handle not found")
	}
	request.OUID = ouOnly.OUID

	if !utils.IsValidHandle(request.Handle) {
		logger.Debug(ctx, "Invalid entity type handle", log.String("handle", request.Handle))
		return nil, &ErrorInvalidEntityTypeHandle
	}

	schemaToValidate := EntityType{
		Category:         category,
		Handle:           request.Handle,
		DisplayName:      request.DisplayName,
		OUID:             request.OUID,
		SystemAttributes: request.SystemAttributes,
		Schema:           request.Schema,
	}
	if validationErr := validateEntityTypeDefinition(ctx, category, &schemaToValidate); validationErr != nil {
		logger.Debug(ctx, "Entity type validation failed", log.String("handle", request.Handle))
		return nil, validationErr
	}

	if svcErr := us.ensureOrganizationUnitExists(
		ctx, request.OUID, category, logger); svcErr != nil {
		return nil, svcErr
	}

	if svcErr := us.checkEntityTypeAccess(
		ctx, category, createActionForCategory(category), request.OUID); svcErr != nil {
		return nil, svcErr
	}

	_, err := us.entityTypeStore.GetEntityTypeByHandle(ctx, category, request.Handle)
	if err == nil {
		return nil, entityTypeHandleConflictErr(category)
	} else if !errors.Is(err, ErrEntityTypeNotFound) {
		return nil, logAndReturnServerError(ctx, logger, "Failed to check existing entity type", err)
	}

	id := request.ID
	if id == "" {
		id, err = utils.GenerateUUIDv7()
		if err != nil {
			logger.Error(ctx, "Failed to generate UUID", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
	}

	entityType := EntityType{
		ID:                    id,
		Category:              category,
		Handle:                request.Handle,
		DisplayName:           request.DisplayName,
		OUID:                  request.OUID,
		AllowSelfRegistration: request.AllowSelfRegistration,
		SystemAttributes:      schemaToValidate.SystemAttributes,
		Schema:                request.Schema,
	}

	if err := us.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if category == TypeCategoryAgent {
			if err := us.ensureOUHasNoAgentType(txCtx, request.OUID); err != nil {
				return err
			}
		}
		return us.entityTypeStore.CreateEntityType(txCtx, entityType)
	}); err != nil {
		if errors.Is(err, errAgentTypeExistsInOU) {
			return nil, &ErrorAgentTypeAlreadyExistsInOU
		}
		return nil, logAndReturnServerError(ctx, logger, "Failed to create entity type", err)
	}

	return &entityType, nil
}

// GetEntityType retrieves an entity type by its ID within the given category.
func (us *entityTypeService) GetEntityType(
	ctx context.Context, category TypeCategory, schemaID string, includeDisplay bool,
) (*EntityType, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if schemaID == "" {
		return nil, invalidEntityTypeRequestErr(category, "schema id must not be empty")
	}

	entityType, err := us.entityTypeStore.GetEntityTypeByID(ctx, category, schemaID)
	if err != nil {
		if errors.Is(err, ErrEntityTypeNotFound) {
			return nil, entityTypeNotFoundErr(category)
		}
		return nil, logAndReturnServerError(ctx, logger, "Failed to get entity type", err)
	}

	if svcErr := us.checkEntityTypeAccess(
		ctx, category, readActionForCategory(category), entityType.OUID); svcErr != nil {
		return nil, svcErr
	}

	if includeDisplay {
		handleMap, svcErr := us.ouService.GetOrganizationUnitHandlesByIDs(
			ctx, []string{entityType.OUID})
		if svcErr != nil {
			logger.Warn(ctx, "Failed to resolve OU handle for entity type, skipping",
				log.String("id", schemaID), log.Any("error", svcErr))
		} else if handle, ok := handleMap[entityType.OUID]; ok {
			entityType.OUHandle = handle
		}
	}
	us.withManaged(ctx, category, &entityType)

	return &entityType, nil
}

// GetEntityTypeSchema retrieves the schema config by handle without admin access check
func (us *entityTypeService) GetEntityTypeSchema(
	ctx context.Context, category TypeCategory, handle string,
) (*EntityType, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if handle == "" {
		return nil, invalidEntityTypeRequestErr(category, "entity type handle must not be empty")
	}

	entityType, err := us.entityTypeStore.GetEntityTypeByHandle(ctx, category, handle)
	if err != nil {
		if errors.Is(err, ErrEntityTypeNotFound) {
			return nil, entityTypeNotFoundErr(category)
		}
		return nil, logAndReturnServerError(ctx, logger, "Failed to get entity type", err)
	}

	return &entityType, nil
}

// GetEntityTypeByHandle retrieves an entity type by its handle within the given category.
func (us *entityTypeService) GetEntityTypeByHandle(
	ctx context.Context, category TypeCategory, handle string,
) (*EntityType, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if handle == "" {
		return nil, invalidEntityTypeRequestErr(category, "entity type handle must not be empty")
	}

	entityType, err := us.entityTypeStore.GetEntityTypeByHandle(ctx, category, handle)
	if err != nil {
		if errors.Is(err, ErrEntityTypeNotFound) {
			return nil, entityTypeNotFoundErr(category)
		}
		return nil, logAndReturnServerError(ctx, logger, "Failed to get entity type by handle", err)
	}

	if svcErr := us.checkEntityTypeAccess(
		ctx, category, readActionForCategory(category), entityType.OUID); svcErr != nil {
		return nil, svcErr
	}
	us.withManaged(ctx, category, &entityType)

	return &entityType, nil
}

// UpdateEntityType updates an entity type by its ID within the given category.
func (us *entityTypeService) UpdateEntityType(ctx context.Context, category TypeCategory,
	schemaID string, request UpdateEntityTypeRequest) (
	*EntityType, *tidcommon.ServiceError) {
	updated, svcErr := us.updateEntityType(ctx, category, schemaID, request)
	us.publishEntityTypeEvent(ctx, category, updateEventForCategory(category), audit.ActionUpdate, schemaID,
		svcErr)
	return updated, svcErr
}

// updateEntityType updates an entity type by its ID within the given category without publishing its
// audit event.
func (us *entityTypeService) updateEntityType(ctx context.Context, category TypeCategory,
	schemaID string, request UpdateEntityTypeRequest) (
	*EntityType, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if schemaID == "" {
		return nil, invalidEntityTypeRequestErr(category, "schema id must not be empty")
	}

	if us.entityTypeStore.IsEntityTypeDeclarative(category, schemaID) {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	ouOnly := &EntityType{ID: schemaID, Handle: request.Handle, OUID: request.OUID, OUHandle: request.OUHandle}
	if svcErr := us.resolveEntityTypeOUHandle(ctx, ouOnly); svcErr != nil {
		return nil, invalidEntityTypeRequestErr(category, "organization unit with handle not found")
	}
	request.OUID = ouOnly.OUID

	schemaToValidate := EntityType{
		Category:         category,
		Handle:           request.Handle,
		DisplayName:      request.DisplayName,
		OUID:             request.OUID,
		SystemAttributes: request.SystemAttributes,
		Schema:           request.Schema,
	}
	if validationErr := validateEntityTypeDefinition(ctx, category, &schemaToValidate); validationErr != nil {
		logger.Debug(ctx, "Entity type validation failed", log.String("id", schemaID))
		return nil, validationErr
	}

	if svcErr := us.ensureOrganizationUnitExists(
		ctx, request.OUID, category, logger); svcErr != nil {
		return nil, svcErr
	}

	existingSchema, err := us.entityTypeStore.GetEntityTypeByID(ctx, category, schemaID)
	if err != nil {
		if errors.Is(err, ErrEntityTypeNotFound) {
			return nil, entityTypeNotFoundErr(category)
		}
		return nil, logAndReturnServerError(ctx, logger, "Failed to get existing entity type", err)
	}

	if svcErr := us.checkEntityTypeAccess(
		ctx, category, updateActionForCategory(category), existingSchema.OUID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := us.refuseManaged(ctx, category, schemaID); svcErr != nil {
		return nil, svcErr
	}

	if request.OUID != existingSchema.OUID {
		if svcErr := us.checkEntityTypeAccess(
			ctx, category, updateActionForCategory(category), request.OUID); svcErr != nil {
			return nil, svcErr
		}
	}

	if request.Handle != "" && request.Handle != existingSchema.Handle {
		return nil, &ErrorEntityTypeHandleUpdateNotAllowed
	}

	entityType := EntityType{
		ID:                    schemaID,
		Category:              category,
		Handle:                existingSchema.Handle,
		DisplayName:           request.DisplayName,
		OUID:                  request.OUID,
		AllowSelfRegistration: request.AllowSelfRegistration,
		SystemAttributes:      schemaToValidate.SystemAttributes,
		Schema:                request.Schema,
	}

	if err := us.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if category == TypeCategoryAgent && request.OUID != existingSchema.OUID {
			if err := us.ensureOUHasNoAgentType(txCtx, request.OUID); err != nil {
				return err
			}
		}
		return us.entityTypeStore.UpdateEntityTypeByID(txCtx, category, schemaID, entityType)
	}); err != nil {
		if errors.Is(err, errAgentTypeExistsInOU) {
			return nil, &ErrorAgentTypeAlreadyExistsInOU
		}
		return nil, logAndReturnServerError(ctx, logger, "Failed to update entity type", err)
	}

	return &entityType, nil
}

// DeleteEntityType deletes an entity type by its ID within the given category.
func (us *entityTypeService) DeleteEntityType(ctx context.Context, category TypeCategory,
	schemaID string) *tidcommon.ServiceError {
	svcErr := us.deleteEntityType(ctx, category, schemaID)
	// A delete that found nothing to remove succeeds without an event, so no caller can record a
	// deletion of an ID it made up.
	if svcErr == errEntityTypeNotFoundNoop {
		return nil
	}
	us.publishEntityTypeEvent(ctx, category, deleteEventForCategory(category), audit.ActionDelete, schemaID,
		svcErr)
	return svcErr
}

// deleteEntityType deletes an entity type by its ID within the given category without publishing its
// audit event.
func (us *entityTypeService) deleteEntityType(ctx context.Context, category TypeCategory,
	schemaID string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return svcErr
	}

	if schemaID == "" {
		return invalidEntityTypeRequestErr(category, "schema id must not be empty")
	}

	existingSchema, err := us.entityTypeStore.GetEntityTypeByID(ctx, category, schemaID)
	if err != nil {
		if errors.Is(err, ErrEntityTypeNotFound) {
			if svcErr := us.checkEntityTypeAccess(
				ctx, category, deleteActionForCategory(category), ""); svcErr != nil {
				return svcErr
			}
			return errEntityTypeNotFoundNoop
		}
		return logAndReturnServerError(ctx, logger, "Failed to get entity type for delete", err)
	}

	if svcErr := us.checkEntityTypeAccess(
		ctx, category, deleteActionForCategory(category), existingSchema.OUID); svcErr != nil {
		return svcErr
	}

	if us.entityTypeStore.IsEntityTypeDeclarative(category, schemaID) {
		return &ErrorCannotModifyDeclarativeResource
	}
	if svcErr := us.refuseManaged(ctx, category, schemaID); svcErr != nil {
		return svcErr
	}

	if category == TypeCategoryAgent {
		if svcErr := us.ensureAgentTypeNotInUse(ctx, existingSchema.Handle, logger); svcErr != nil {
			return svcErr
		}
	}

	if err := us.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return us.entityTypeStore.DeleteEntityTypeByID(txCtx, category, schemaID)
	}); err != nil {
		return logAndReturnServerError(ctx, logger, "Failed to delete entity type", err)
	}

	return nil
}

// publishEntityTypeEvent publishes the audit event of an attempt on the entity type with id, after the
// change committed or was refused. The category picks the target type, a user type or an agent type.
// Neither the schema nor any other part of the type is recorded.
func (us *entityTypeService) publishEntityTypeEvent(ctx context.Context, category TypeCategory, name string,
	action audit.Action, id string, svcErr *tidcommon.ServiceError) {
	// A failed attempt keeps the ID the caller asked for, even one that matches no entity type, as
	// forensic evidence of what was probed; the payload caps it and the sink escapes it.
	target := audit.Ref{Type: auditTargetTypeForCategory(category), ID: id}
	eventbus.TryPublish(ctx, us.bus,
		eventbus.NewEvent(ctx, name).With(audit.ChangePayload(ctx, target, action, svcErr)))
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (us *entityTypeService) SetDependencyRegistry(r resourcedependency.Registry) {
	us.dependencyRegistry = r
}

// GetResourceDependencies implements resourcedependency.Provider. It reports the user, agent and
// organization unit types that belong to the given organization unit, which block the organization
// unit's deletion (an entity type cannot exist without its organization unit). Only organization
// unit targets are handled; other resource types have no entity type dependencies. The number of
// entity types scanned per category is bounded by MaxCompositeStoreRecords.
func (us *entityTypeService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	if resourceType != resourcedependency.ResourceTypeOU {
		return []resourcedependency.ResourceDependency{}, nil
	}

	categoryResourceTypes := []struct {
		category     TypeCategory
		resourceType string
	}{
		{TypeCategoryUser, resourcedependency.ResourceTypeUserType},
		{TypeCategoryAgent, resourcedependency.ResourceTypeAgentType},
		{TypeCategoryOU, resourcedependency.ResourceTypeOUType},
	}

	deps := make([]resourcedependency.ResourceDependency, 0)
	for _, c := range categoryResourceTypes {
		entityTypes, err := us.entityTypeStore.GetEntityTypeListByOUIDs(
			ctx, c.category, []string{id}, serverconst.MaxCompositeStoreRecords, 0)
		if err != nil {
			return nil, err
		}
		for _, et := range entityTypes {
			deps = append(deps, resourcedependency.ResourceDependency{
				ResourceType:     c.resourceType,
				ID:               et.ID,
				DisplayName:      et.DisplayName,
				BehaviorOnDelete: resourcedependency.BehaviorRestrict,
			})
		}
	}
	return deps, nil
}

// ValidateEntity validates entity attributes against the schema for the given category and entity type.
func (us *entityTypeService) ValidateEntity(
	ctx context.Context, category TypeCategory, entityType string, attributes json.RawMessage,
	skipCredentialRequired bool,
) (bool, *tidcommon.ServiceError) {
	return us.validateAgainstSchema(ctx, category, entityType,
		func(schema *model.Schema, logger *log.Logger) (bool, error) {
			return schema.Validate(ctx, attributes, logger, skipCredentialRequired)
		})
}

// ValidateEntityValues validates only the given attributes against the schema for the given category and
// entity type, without requiring the attributes the payload leaves out.
func (us *entityTypeService) ValidateEntityValues(
	ctx context.Context, category TypeCategory, entityType string, attributes json.RawMessage,
) (bool, *tidcommon.ServiceError) {
	return us.validateAgainstSchema(ctx, category, entityType,
		func(schema *model.Schema, logger *log.Logger) (bool, error) {
			return schema.ValidateValues(ctx, attributes, logger)
		})
}

// validateAgainstSchema loads the compiled schema for the given category and entity type and runs
// validate against it.
func (us *entityTypeService) validateAgainstSchema(
	ctx context.Context, category TypeCategory, entityType string,
	validate func(schema *model.Schema, logger *log.Logger) (bool, error),
) (bool, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return false, svcErr
	}

	compiledSchema, err := us.getCompiledSchemaForEntityType(ctx, category, entityType, logger)
	if err != nil {
		if errors.Is(err, ErrEntityTypeNotFound) {
			return false, entityTypeNotFoundErr(category)
		}
		return false, logAndReturnServerError(ctx, logger, "Failed to load entity type", err)
	}

	isValid, err := validate(compiledSchema, logger)
	if err != nil {
		return false, logAndReturnServerError(ctx, logger, "Failed to validate entity attributes against schema", err)
	}
	if !isValid {
		logger.Debug(ctx, "Schema validation failed", log.String("category", string(category)),
			log.String("entityType", entityType))
		return false, nil
	}

	logger.Debug(ctx, "Schema validation successful", log.String("category", string(category)),
		log.String("entityType", entityType))
	return true, nil
}

// ValidateEntityUniqueness validates the uniqueness constraints of entity attributes.
func (us *entityTypeService) ValidateEntityUniqueness(
	ctx context.Context,
	category TypeCategory,
	entityType string,
	attributes json.RawMessage,
	exists func(map[string]interface{}) (bool, error),
) (bool, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return false, svcErr
	}

	compiledSchema, err := us.getCompiledSchemaForEntityType(ctx, category, entityType, logger)
	if err != nil {
		if errors.Is(err, ErrEntityTypeNotFound) {
			return false, entityTypeNotFoundErr(category)
		}
		return false, logAndReturnServerError(ctx, logger, "Failed to load entity type", err)
	}

	if len(attributes) == 0 {
		return true, nil
	}

	var attrs map[string]interface{}
	if err := json.Unmarshal(attributes, &attrs); err != nil {
		return false, logAndReturnServerError(ctx, logger, "Failed to unmarshal entity attributes", err)
	}

	isValid, err := compiledSchema.ValidateUniqueness(ctx, attrs, exists, logger)
	if err != nil {
		return false, logAndReturnServerError(ctx, logger, "Failed during uniqueness validation", err)
	}
	if !isValid {
		logger.Debug(ctx, "Entity attribute failed uniqueness validation",
			log.String("category", string(category)), log.String("entityType", entityType))
		return false, nil
	}

	return true, nil
}

// GetAttributes returns schema properties matching the given filter for the given entity type.
func (us *entityTypeService) GetAttributes(
	ctx context.Context, category TypeCategory, entityType string, filter AttributeFilter,
) ([]AttributeInfo, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	compiledSchema, err := us.getCompiledSchemaForEntityType(ctx, category, entityType, logger)
	if err != nil {
		if errors.Is(err, ErrEntityTypeNotFound) {
			return nil, entityTypeNotFoundErr(category)
		}
		return nil, logAndReturnServerError(ctx, logger, "Failed to load entity type for attribute infos", err)
	}

	return compiledSchema.GetAttributes(filter), nil
}

// GetAttributesForEntityType returns matching attributes for every category containing an entity type.
func (us *entityTypeService) GetAttributesForEntityType(
	ctx context.Context, entityType string, filter AttributeFilter,
) (map[TypeCategory][]AttributeInfo, *tidcommon.ServiceError) {
	attributesByCategory := make(map[TypeCategory][]AttributeInfo)
	for _, category := range []TypeCategory{TypeCategoryUser, TypeCategoryAgent} {
		attributes, svcErr := us.GetAttributes(ctx, category, entityType, filter)
		if svcErr == nil {
			attributesByCategory[category] = attributes
			continue
		}
		if svcErr.Code != ErrorEntityTypeNotFound.Code {
			return nil, svcErr
		}
	}
	if len(attributesByCategory) == 0 {
		return nil, &ErrorEntityTypeNotFound
	}
	return attributesByCategory, nil
}

// GetUniqueAttributes returns the names of schema properties marked as unique for a given entity type.
func (us *entityTypeService) GetUniqueAttributes(
	ctx context.Context, category TypeCategory, entityType string,
) ([]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	compiledSchema, err := us.getCompiledSchemaForEntityType(ctx, category, entityType, logger)
	if err != nil {
		if errors.Is(err, ErrEntityTypeNotFound) {
			return nil, entityTypeNotFoundErr(category)
		}
		return nil, logAndReturnServerError(ctx, logger, "Failed to load entity type for unique attributes", err)
	}

	return compiledSchema.GetUniqueAttributes(), nil
}

// GetDisplayAttributesByHandles returns display attributes for multiple entity types by handle within a
// category.
func (us *entityTypeService) GetDisplayAttributesByHandles(
	ctx context.Context, category TypeCategory, handles []string,
) (map[string]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if len(handles) == 0 {
		return map[string]string{}, nil
	}

	result, err := us.entityTypeStore.GetDisplayAttributesByHandles(ctx, category, handles)
	if err != nil {
		return nil, logAndReturnServerError(ctx, logger, "Failed to get display attributes by handles", err)
	}

	return result, nil
}

// ensureOUHasNoAgentType refuses a write that would give the organization unit a second agent type. The
// agent type being written is never already in the organization unit: a create adds a new one, and an
// update checks only an organization unit it moves the agent type into.
func (us *entityTypeService) ensureOUHasNoAgentType(ctx context.Context, ouID string) error {
	existing, err := us.entityTypeStore.GetEntityTypeListByOUIDs(ctx, TypeCategoryAgent, []string{ouID}, 1, 0)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		return errAgentTypeExistsInOU
	}
	return nil
}

// ensureAgentTypeNotInUse refuses the deletion of an agent type that agents are of, or that applications,
// agents or flows allow. Every reference names an agent type by its handle, so the registry target is the
// handle, unlike other targets, which are IDs; the dependencies reported back still carry their own IDs.
// Because deletion is destructive, it fails closed: if dependency data cannot be determined, the
// deletion is refused rather than allowed.
func (us *entityTypeService) ensureAgentTypeNotInUse(
	ctx context.Context, handle string, logger *log.Logger,
) *tidcommon.ServiceError {
	if us.dependencyRegistry == nil {
		logger.Error(ctx, "Dependency registry not set; refusing to delete agent type")
		return &tidcommon.InternalServerError
	}

	deps, err := us.dependencyRegistry.GetDependencies(ctx, resourcedependency.ResourceTypeAgentType, handle)
	if err != nil {
		logger.Error(ctx, "Failed to evaluate agent type dependencies", log.Error(err))
		return &tidcommon.InternalServerError
	}
	// Fail closed: nil TotalResults means a provider failed to report, so usage is unknown.
	if deps == nil || deps.TotalResults == nil {
		logger.Error(ctx, "Agent type dependency data unavailable; refusing to delete")
		return &tidcommon.InternalServerError
	}

	if len(resourcedependency.BlockingUsages(deps)) > 0 {
		return &ErrorAgentTypeInUse
	}
	return nil
}

func (us *entityTypeService) getCompiledSchemaForEntityType(
	ctx context.Context,
	category TypeCategory,
	entityType string,
	logger *log.Logger,
) (*model.Schema, error) {
	if entityType == "" {
		return nil, ErrEntityTypeNotFound
	}

	found, err := us.entityTypeStore.GetEntityTypeByHandle(ctx, category, entityType)
	if err != nil {
		return nil, err
	}

	compiled, err := model.CompileSchema(found.Schema)
	if err != nil {
		logger.Error(ctx, "Failed to compile stored entity type", log.String("category", string(category)),
			log.String("entityType", entityType), log.Error(err))
		return nil, fmt.Errorf("failed to compile stored entity type: %w", err)
	}

	return compiled, nil
}

// checkEntityTypeAccess validates that the caller is authorized to perform the given action on
// an entity type in the given category. Pass the schema's OU ID to scope the check to the
// caller's organization unit membership.
func (us *entityTypeService) checkEntityTypeAccess(
	ctx context.Context, category TypeCategory, action security.Action, ouID string,
) *tidcommon.ServiceError {
	if us.authzService == nil {
		return nil
	}
	allowed, svcErr := us.authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: resourceTypeForCategory(category), OUID: ouID})
	if svcErr != nil {
		return &tidcommon.InternalServerError
	}
	if !allowed {
		return &tidcommon.ErrorUnauthorized
	}
	return nil
}

// getAccessibleResources returns the set of OU IDs in which the caller may list entity types of the
// given category.
func (us *entityTypeService) getAccessibleResources(
	ctx context.Context, category TypeCategory,
) (*sysauthz.AccessibleResources, *tidcommon.ServiceError) {
	if us.authzService == nil {
		return &sysauthz.AccessibleResources{AllAllowed: true}, nil
	}
	accessible, svcErr := us.authzService.GetAccessibleResources(
		ctx, listActionForCategory(category), resourceTypeForCategory(category))
	if svcErr != nil {
		return nil, &tidcommon.InternalServerError
	}
	return accessible, nil
}

// ResolveEntityTypeHandles resolves ou_handle to an OU ID on the given entity type in-place.
// Called by the declarative loader validator (startup, no user context) so that file-based
// entity types support ou_handle. It elevates to runtime context internally.
func (us *entityTypeService) ResolveEntityTypeHandles(
	ctx context.Context, entityType *EntityType,
) *tidcommon.ServiceError {
	return us.resolveEntityTypeOUHandle(security.WithRuntimeContext(ctx), entityType)
}

// resolveEntityTypeOUHandle resolves ou_handle to an OU ID on the given entity type in-place
// using the caller's context. API/importer paths must pass their own caller context so the
// underlying OU lookup is still subject to the caller's ou:read authorization.
// If both ou_id and ou_handle are provided, ou_id wins and a warning is logged.
func (us *entityTypeService) resolveEntityTypeOUHandle(
	ctx context.Context, entityType *EntityType,
) *tidcommon.ServiceError {
	if entityType.OUID != "" && entityType.OUHandle != "" {
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))
		logger.Warn(ctx, "Both ouId and ouHandle provided for entity type; ouHandle ignored",
			log.String("entityTypeID", entityType.ID), log.String("handle", entityType.Handle))
		return nil
	}
	if entityType.OUID == "" && entityType.OUHandle != "" {
		if us.ouService == nil {
			return &tidcommon.InternalServerError
		}
		ou, svcErr := us.ouService.GetOrganizationUnitByPath(ctx, entityType.OUHandle)
		if svcErr != nil {
			return &ErrorInvalidRequestFormat
		}
		entityType.OUID = ou.ID
	}
	return nil
}

// ensureOrganizationUnitExists validates that the provided organization unit exists using the OU service.
func (us *entityTypeService) ensureOrganizationUnitExists(
	ctx context.Context,
	oUID string,
	category TypeCategory,
	logger *log.Logger,
) *tidcommon.ServiceError {
	if us.ouService == nil {
		logger.Error(ctx, "Organization unit service is not configured for entity type operations")
		return &tidcommon.InternalServerError
	}

	exists, svcErr := us.ouService.IsOrganizationUnitExists(ctx, oUID)
	if svcErr != nil {
		logger.Error(ctx, "Failed to verify organization unit existence",
			log.String("ouID", oUID), log.Any("error", svcErr))
		return &tidcommon.InternalServerError
	}

	if !exists {
		logger.Debug(ctx, "Organization unit does not exist",
			log.String("ouID", oUID))
		return invalidEntityTypeRequestErr(category, "organization unit id does not exist")
	}

	return nil
}

// validatePaginationParams validates the limit and offset parameters.
func validatePaginationParams(limit, offset int) *tidcommon.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}
	return nil
}

// populateEntityTypeOUHandles resolves OU handles for a slice of entity types in-place.
func (us *entityTypeService) populateEntityTypeOUHandles(
	ctx context.Context, schemas []EntityTypeListItem, logger *log.Logger,
) {
	ouIDs := make([]string, 0, len(schemas))
	seen := make(map[string]bool, len(schemas))
	for _, s := range schemas {
		if s.OUID != "" && !seen[s.OUID] {
			ouIDs = append(ouIDs, s.OUID)
			seen[s.OUID] = true
		}
	}

	handleMap, svcErr := us.ouService.GetOrganizationUnitHandlesByIDs(ctx, ouIDs)
	if svcErr != nil {
		logger.Warn(ctx, "Failed to resolve OU handles for entity types, skipping", log.Any("error", svcErr))
		return
	}

	for i := range schemas {
		if handle, ok := handleMap[schemas[i].OUID]; ok {
			schemas[i].OUHandle = handle
		}
	}
}

// pathForCategory returns the public API base path for a given schema category.
func pathForCategory(category TypeCategory) string {
	switch category {
	case TypeCategoryAgent:
		return "/agent-types"
	case TypeCategoryOU:
		return "/organization-unit-types"
	default:
		return "/user-types"
	}
}

// validateCategory ensures the supplied category is one of the supported values.
func validateCategory(category TypeCategory) *tidcommon.ServiceError {
	if !category.IsValid() {
		return invalidEntityTypeRequestErr(category, "invalid schema category")
	}
	return nil
}

// listActionForCategory returns the sysauthz list action that gates listing schemas of the given
// category.
func listActionForCategory(category TypeCategory) security.Action {
	switch category {
	case TypeCategoryAgent:
		return security.ActionListAgentTypes
	case TypeCategoryOU:
		return security.ActionListOUTypes
	default:
		return security.ActionListUserTypes
	}
}

// createActionForCategory returns the sysauthz create action for the given category.
func createActionForCategory(category TypeCategory) security.Action {
	switch category {
	case TypeCategoryAgent:
		return security.ActionCreateAgentType
	case TypeCategoryOU:
		return security.ActionCreateOUType
	default:
		return security.ActionCreateUserType
	}
}

// readActionForCategory returns the sysauthz read action for the given category.
func readActionForCategory(category TypeCategory) security.Action {
	switch category {
	case TypeCategoryAgent:
		return security.ActionReadAgentType
	case TypeCategoryOU:
		return security.ActionReadOUType
	default:
		return security.ActionReadUserType
	}
}

// updateActionForCategory returns the sysauthz update action for the given category.
func updateActionForCategory(category TypeCategory) security.Action {
	switch category {
	case TypeCategoryAgent:
		return security.ActionUpdateAgentType
	case TypeCategoryOU:
		return security.ActionUpdateOUType
	default:
		return security.ActionUpdateUserType
	}
}

// deleteActionForCategory returns the sysauthz delete action for the given category.
func deleteActionForCategory(category TypeCategory) security.Action {
	switch category {
	case TypeCategoryAgent:
		return security.ActionDeleteAgentType
	case TypeCategoryOU:
		return security.ActionDeleteOUType
	default:
		return security.ActionDeleteUserType
	}
}

// resourceTypeForCategory returns the sysauthz resource type for the given category.
func resourceTypeForCategory(category TypeCategory) security.ResourceType {
	switch category {
	case TypeCategoryAgent:
		return security.ResourceTypeAgentType
	case TypeCategoryOU:
		return security.ResourceTypeOUType
	default:
		return security.ResourceTypeUserType
	}
}

// createEventForCategory returns the audit event of an attempt to create an entity type of the given
// category.
func createEventForCategory(category TypeCategory) string {
	switch category {
	case TypeCategoryAgent:
		return audit.EventAgentTypeCreate
	case TypeCategoryOU:
		return audit.EventOUTypeCreate
	default:
		return audit.EventUserTypeCreate
	}
}

// updateEventForCategory returns the audit event of an attempt to change an entity type of the given
// category.
func updateEventForCategory(category TypeCategory) string {
	switch category {
	case TypeCategoryAgent:
		return audit.EventAgentTypeUpdate
	case TypeCategoryOU:
		return audit.EventOUTypeUpdate
	default:
		return audit.EventUserTypeUpdate
	}
}

// deleteEventForCategory returns the audit event of an attempt to delete an entity type of the given
// category.
func deleteEventForCategory(category TypeCategory) string {
	switch category {
	case TypeCategoryAgent:
		return audit.EventAgentTypeDelete
	case TypeCategoryOU:
		return audit.EventOUTypeDelete
	default:
		return audit.EventUserTypeDelete
	}
}

// auditTargetTypeForCategory returns the audit target type of an entity type of the given category.
func auditTargetTypeForCategory(category TypeCategory) string {
	switch category {
	case TypeCategoryAgent:
		return audit.TargetTypeAgentType
	case TypeCategoryOU:
		return audit.TargetTypeOUType
	default:
		return audit.TargetTypeUserType
	}
}

// buildPaginationLinks builds pagination links for the response.
func buildPaginationLinks(category TypeCategory, limit, offset, totalCount int, displayQuery string) []Link {
	links := make([]Link, 0)
	base := pathForCategory(category)

	if offset > 0 {
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=0&limit=%d%s", base, limit, displayQuery),
			Rel:  "first",
		})

		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=%d&limit=%d%s", base, prevOffset, limit, displayQuery),
			Rel:  "prev",
		})
	}

	if offset+limit < totalCount {
		nextOffset := offset + limit
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=%d&limit=%d%s", base, nextOffset, limit, displayQuery),
			Rel:  "next",
		})
	}

	lastPageOffset := ((totalCount - 1) / limit) * limit
	if offset < lastPageOffset {
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=%d&limit=%d%s", base, lastPageOffset, limit, displayQuery),
			Rel:  "last",
		})
	}

	return links
}

// logAndReturnServerError logs the error and returns a server error.
func logAndReturnServerError(ctx context.Context,
	logger *log.Logger,
	message string,
	err error,
) *tidcommon.ServiceError {
	logger.Error(ctx, message, log.Error(err))
	return &tidcommon.InternalServerError
}

// validateEntityTypeDefinition validates the entity type definition for create and update requests,
// without checking OU existence.
// Omitted primary contact mappings are defaulted in place.
func validateEntityTypeDefinition(
	ctx context.Context, category TypeCategory, schema *EntityType) *tidcommon.ServiceError {
	logger := log.GetLogger()

	if schema.DisplayName == "" {
		logger.Debug(ctx, "Entity type validation failed: display name is empty")
		return invalidEntityTypeRequestErr(category, "entity type display name must not be empty")
	}

	if utf8.RuneCountInString(schema.DisplayName) > maxDisplayNameLength {
		logger.Debug(ctx, "Entity type validation failed: display name is too long")
		return invalidEntityTypeRequestErr(category,
			fmt.Sprintf("entity type display name must not exceed %d characters", maxDisplayNameLength))
	}

	if schema.OUID == "" {
		logger.Debug(ctx, "Entity type validation failed: organization unit ID is empty")
		return invalidEntityTypeRequestErr(category, "organization unit id must not be empty")
	}

	if len(schema.Schema) == 0 {
		logger.Debug(ctx, "Entity type validation failed: schema definition is empty")
		return invalidEntityTypeRequestErr(category, "schema definition must not be empty")
	}

	compiledSchema, err := model.CompileSchema(schema.Schema)
	if err != nil {
		logger.Debug(ctx, "Entity type validation failed: schema compilation error",
			log.Error(err))
		return invalidEntityTypeRequestErr(category, err.Error())
	}

	systemAttrs, svcErr := validateSystemAttributes(category, compiledSchema, schema.SystemAttributes)
	if svcErr != nil {
		return svcErr
	}
	schema.SystemAttributes = systemAttrs
	return nil
}

// validateSystemAttributes validates the system attributes against the compiled schema and returns
// them with omitted primary contacts defaulted for user types.
func validateSystemAttributes(
	category TypeCategory, compiledSchema *model.Schema, systemAttrs *SystemAttributes,
) (*SystemAttributes, *tidcommon.ServiceError) {
	var attrs SystemAttributes
	if systemAttrs != nil {
		attrs = *systemAttrs
	}

	if svcErr := validateDisplayAttribute(compiledSchema, attrs.Display); svcErr != nil {
		return nil, svcErr
	}

	if category != TypeCategoryUser {
		if attrs.PrimaryEmail != "" || attrs.PrimaryMobile != "" {
			return nil, &ErrorPrimaryContactNotSupported
		}
		return systemAttrs, nil
	}

	var svcErr *tidcommon.ServiceError
	if attrs.PrimaryEmail, svcErr = resolvePrimaryContact(
		compiledSchema, attrs.PrimaryEmail, model.TypeEmail); svcErr != nil {
		return nil, svcErr
	}
	if attrs.PrimaryMobile, svcErr = resolvePrimaryContact(
		compiledSchema, attrs.PrimaryMobile, model.TypeMobile); svcErr != nil {
		return nil, svcErr
	}

	if systemAttrs == nil && attrs == (SystemAttributes{}) {
		return nil, nil
	}
	return &attrs, nil
}

// validateDisplayAttribute validates that the display attribute, if provided,
// references an existing, displayable, non-credential attribute in the compiled schema.
// String, contact, and number types are displayable.
func validateDisplayAttribute(
	compiledSchema *model.Schema, display string,
) *tidcommon.ServiceError {
	if display == "" {
		return nil
	}

	switch compiledSchema.ValidateAsDisplayAttribute(display) {
	case model.DisplayAttributeNotFound:
		return &ErrorInvalidDisplayAttribute
	case model.DisplayAttributeNotDisplayable:
		return &ErrorNonDisplayableAttribute
	case model.DisplayAttributeIsCredential:
		return &ErrorCredentialDisplayAttribute
	default:
		return nil
	}
}

// resolvePrimaryContact validates a primary contact mapping. An omitted mapping defaults to the
// only top-level attribute of the contact type, and stays unset when there are several.
func resolvePrimaryContact(
	compiledSchema *model.Schema, name, contactType string,
) (string, *tidcommon.ServiceError) {
	if name == "" {
		candidates := compiledSchema.GetAttributes(
			model.AttributeFilter{AllowNonCredential: true, Type: contactType})
		if len(candidates) == 1 {
			return candidates[0].Attribute, nil
		}
		return "", nil
	}

	switch compiledSchema.ValidateAsPrimaryContact(name, contactType) {
	case model.PrimaryContactNotFound:
		return "", &ErrorInvalidPrimaryContactAttribute
	case model.PrimaryContactTypeMismatch:
		return "", &ErrorPrimaryContactTypeMismatch
	default:
		return name, nil
	}
}

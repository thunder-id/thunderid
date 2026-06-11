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

// Package entitytype handles the entity type management operations.
package entitytype

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/consent"
	"github.com/thunder-id/thunderid/internal/entitytype/model"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const entityTypeLoggerComponentName = "EntityTypeService"

// AttributeInfo is an alias for model.AttributeInfo, exported at the entitytype package
// level so callers do not need to import the internal model package directly.
type AttributeInfo = model.AttributeInfo

// EntityTypeServiceInterface defines the interface for the entity type service.
// All methods take a TypeCategory to scope the operation to a specific entity kind
// (user or agent).
type EntityTypeServiceInterface interface {
	GetEntityTypeList(ctx context.Context, category TypeCategory, limit, offset int,
		includeDisplay bool) (*EntityTypeListResponse, *serviceerror.ServiceError)
	CreateEntityType(
		ctx context.Context, category TypeCategory, request CreateEntityTypeRequestWithID,
	) (*EntityType, *serviceerror.ServiceError)
	GetEntityType(ctx context.Context, category TypeCategory, schemaID string,
		includeDisplay bool) (*EntityType, *serviceerror.ServiceError)
	GetEntityTypeByName(
		ctx context.Context, category TypeCategory, schemaName string,
	) (*EntityType, *serviceerror.ServiceError)
	UpdateEntityType(ctx context.Context, category TypeCategory, schemaID string,
		request UpdateEntityTypeRequest) (
		*EntityType, *serviceerror.ServiceError)
	DeleteEntityType(ctx context.Context, category TypeCategory,
		schemaID string) *serviceerror.ServiceError
	ValidateEntity(
		ctx context.Context, category TypeCategory, entityType string, attributes json.RawMessage,
		skipCredentialRequired bool,
	) (bool, *serviceerror.ServiceError)
	ValidateEntityUniqueness(
		ctx context.Context,
		category TypeCategory,
		entityType string,
		attributes json.RawMessage,
		exists func(map[string]interface{}) (bool, error),
	) (bool, *serviceerror.ServiceError)
	GetAttributes(
		ctx context.Context, category TypeCategory, entityType string,
		allowCredential, allowNonCredential, requiredOnly bool,
	) ([]AttributeInfo, *serviceerror.ServiceError)
	GetUniqueAttributes(
		ctx context.Context, category TypeCategory, entityType string,
	) ([]string, *serviceerror.ServiceError)
	GetDisplayAttributesByNames(
		ctx context.Context, category TypeCategory, names []string,
	) (map[string]string, *serviceerror.ServiceError)
	ResolveEntityTypeHandles(ctx context.Context, entityType *EntityType) *serviceerror.ServiceError
}

// entityTypeService is the default implementation of the EntityTypeServiceInterface.
type entityTypeService struct {
	entityTypeStore entityTypeStoreInterface
	ouService       oupkg.OrganizationUnitServiceInterface
	transactioner   transaction.Transactioner
	authzService    sysauthz.SystemAuthorizationServiceInterface
	consentService  consent.ConsentServiceInterface
}

// newEntityTypeService creates a new instance of entityTypeService.
func newEntityTypeService(
	ouService oupkg.OrganizationUnitServiceInterface,
	store entityTypeStoreInterface,
	transactioner transaction.Transactioner,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	consentService consent.ConsentServiceInterface,
) EntityTypeServiceInterface {
	return &entityTypeService{
		entityTypeStore: store,
		ouService:       ouService,
		transactioner:   transactioner,
		authzService:    authzService,
		consentService:  consentService,
	}
}

// GetEntityTypeList lists entity types for the given category with pagination.
func (us *entityTypeService) GetEntityTypeList(ctx context.Context, category TypeCategory,
	limit, offset int, includeDisplay bool) (
	*EntityTypeListResponse, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	accessible, svcErr := us.getAccessibleResources(ctx, listActionForCategory(category))
	if svcErr != nil {
		return nil, svcErr
	}

	if accessible.AllAllowed {
		logger.Debug(ctx, "Caller has access to all entity types, retrieving without OU filtering",
			log.String("category", string(category)))
		return us.listAllEntityTypes(ctx, category, limit, offset, includeDisplay, logger)
	}

	return us.listAccessibleEntityTypes(ctx, category, accessible.IDs, limit, offset, includeDisplay, logger)
}

// listAllEntityTypes retrieves entity types without authorization filtering.
func (us *entityTypeService) listAllEntityTypes(
	ctx context.Context, category TypeCategory, limit, offset int, includeDisplay bool, logger *log.Logger,
) (*EntityTypeListResponse, *serviceerror.ServiceError) {
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
) (*EntityTypeListResponse, *serviceerror.ServiceError) {
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
) (*EntityType, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if category == TypeCategoryAgent && request.Name != DefaultAgentTypeName {
		return nil, &ErrorAgentTypeOnlyDefaultAllowed
	}

	if isDeclarativeModeEnabled() {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	ouOnly := &EntityType{ID: request.ID, Name: request.Name, OUID: request.OUID, OUHandle: request.OUHandle}
	if svcErr := us.resolveEntityTypeOUHandle(ctx, ouOnly); svcErr != nil {
		return nil, invalidEntityTypeRequestErr(category, "organization unit with handle not found")
	}
	request.OUID = ouOnly.OUID

	schemaToValidate := EntityType{
		Category:         category,
		Name:             request.Name,
		OUID:             request.OUID,
		SystemAttributes: request.SystemAttributes,
		Schema:           request.Schema,
	}
	if validationErr := validateEntityTypeDefinition(ctx, category, schemaToValidate); validationErr != nil {
		logger.Debug(ctx, "Entity type validation failed", log.String("name", request.Name))
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

	_, err := us.entityTypeStore.GetEntityTypeByName(ctx, category, request.Name)
	if err == nil {
		return nil, entityTypeNameConflictErr(category)
	} else if !errors.Is(err, ErrEntityTypeNotFound) {
		return nil, logAndReturnServerError(ctx, logger, "Failed to check existing entity type", err)
	}

	id := request.ID
	if id == "" {
		id, err = utils.GenerateUUIDv7()
		if err != nil {
			logger.Error(ctx, "Failed to generate UUID", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
	}

	entityType := EntityType{
		ID:                    id,
		Category:              category,
		Name:                  request.Name,
		OUID:                  request.OUID,
		AllowSelfRegistration: request.AllowSelfRegistration,
		SystemAttributes:      request.SystemAttributes,
		Schema:                request.Schema,
	}

	if err := us.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return us.entityTypeStore.CreateEntityType(txCtx, entityType)
	}); err != nil {
		return nil, logAndReturnServerError(ctx, logger, "Failed to create entity type", err)
	}

	if us.consentService.IsEnabled() {
		if svcErr := us.syncConsentElementsOnCreate(ctx, category, entityType.Schema, logger); svcErr != nil {
			if delErr := us.entityTypeStore.DeleteEntityTypeByID(ctx, category,
				entityType.ID); delErr != nil {
				logger.Error(ctx, "Failed to compensate schema creation after consent sync failure",
					log.String("schemaID", entityType.ID), log.Error(delErr))
			}

			return nil, svcErr
		}
	}

	return &entityType, nil
}

// GetEntityType retrieves an entity type by its ID within the given category.
func (us *entityTypeService) GetEntityType(
	ctx context.Context, category TypeCategory, schemaID string, includeDisplay bool,
) (*EntityType, *serviceerror.ServiceError) {
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

	return &entityType, nil
}

// GetEntityTypeByName retrieves an entity type by its name within the given category.
func (us *entityTypeService) GetEntityTypeByName(
	ctx context.Context, category TypeCategory, schemaName string,
) (*EntityType, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if schemaName == "" {
		return nil, invalidEntityTypeRequestErr(category, "schema name must not be empty")
	}

	entityType, err := us.entityTypeStore.GetEntityTypeByName(ctx, category, schemaName)
	if err != nil {
		if errors.Is(err, ErrEntityTypeNotFound) {
			return nil, entityTypeNotFoundErr(category)
		}
		return nil, logAndReturnServerError(ctx, logger, "Failed to get entity type by name", err)
	}

	if svcErr := us.checkEntityTypeAccess(
		ctx, category, readActionForCategory(category), entityType.OUID); svcErr != nil {
		return nil, svcErr
	}

	return &entityType, nil
}

// UpdateEntityType updates an entity type by its ID within the given category.
func (us *entityTypeService) UpdateEntityType(ctx context.Context, category TypeCategory,
	schemaID string, request UpdateEntityTypeRequest) (
	*EntityType, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if category == TypeCategoryAgent && request.Name != DefaultAgentTypeName {
		return nil, &ErrorAgentTypeOnlyDefaultAllowed
	}

	if schemaID == "" {
		return nil, invalidEntityTypeRequestErr(category, "schema id must not be empty")
	}

	if us.entityTypeStore.IsEntityTypeDeclarative(category, schemaID) {
		return nil, &ErrorCannotModifyDeclarativeResource
	}

	ouOnly := &EntityType{ID: schemaID, Name: request.Name, OUID: request.OUID, OUHandle: request.OUHandle}
	if svcErr := us.resolveEntityTypeOUHandle(ctx, ouOnly); svcErr != nil {
		return nil, invalidEntityTypeRequestErr(category, "organization unit with handle not found")
	}
	request.OUID = ouOnly.OUID

	schemaToValidate := EntityType{
		Category:         category,
		Name:             request.Name,
		OUID:             request.OUID,
		SystemAttributes: request.SystemAttributes,
		Schema:           request.Schema,
	}
	if validationErr := validateEntityTypeDefinition(ctx, category, schemaToValidate); validationErr != nil {
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

	if request.OUID != existingSchema.OUID {
		if svcErr := us.checkEntityTypeAccess(
			ctx, category, updateActionForCategory(category), request.OUID); svcErr != nil {
			return nil, svcErr
		}
	}

	if request.Name != existingSchema.Name {
		_, err := us.entityTypeStore.GetEntityTypeByName(ctx, category, request.Name)
		if err == nil {
			return nil, entityTypeNameConflictErr(category)
		} else if !errors.Is(err, ErrEntityTypeNotFound) {
			return nil, logAndReturnServerError(ctx, logger, "Failed to check existing entity type", err)
		}
	}

	entityType := EntityType{
		ID:                    schemaID,
		Category:              category,
		Name:                  request.Name,
		OUID:                  request.OUID,
		AllowSelfRegistration: request.AllowSelfRegistration,
		SystemAttributes:      request.SystemAttributes,
		Schema:                request.Schema,
	}

	if err := us.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return us.entityTypeStore.UpdateEntityTypeByID(txCtx, category, schemaID, entityType)
	}); err != nil {
		return nil, logAndReturnServerError(ctx, logger, "Failed to update entity type", err)
	}

	if us.consentService.IsEnabled() {
		if svcErr := us.syncConsentElementsOnUpdate(ctx, category, existingSchema.Schema,
			entityType.Schema, logger); svcErr != nil {
			if revertErr := us.entityTypeStore.UpdateEntityTypeByID(ctx, category, schemaID,
				existingSchema); revertErr != nil {
				logger.Error(ctx, "Failed to compensate schema update after consent sync failure",
					log.String("schemaID", schemaID), log.Error(revertErr))
			}

			return nil, svcErr
		}
	}

	return &entityType, nil
}

// DeleteEntityType deletes an entity type by its ID within the given category.
func (us *entityTypeService) DeleteEntityType(ctx context.Context, category TypeCategory,
	schemaID string) *serviceerror.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return svcErr
	}

	if category == TypeCategoryAgent {
		return &ErrorAgentTypeCannotDelete
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
			return nil
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

	var attributeNames []string
	if us.consentService.IsEnabled() {
		attrNames, err := extractAttributeNames(category, existingSchema.Schema)
		if err != nil {
			logger.Error(ctx,
				"Failed to extract attribute names for consent cleanup; proceeding with schema deletion",
				log.String("schemaID", schemaID), log.Any("error", err))
			attributeNames = []string{}
		} else {
			attributeNames = attrNames
		}
	}

	if err := us.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return us.entityTypeStore.DeleteEntityTypeByID(txCtx, category, schemaID)
	}); err != nil {
		return logAndReturnServerError(ctx, logger, "Failed to delete entity type", err)
	}

	// Sync consent elements for the deleted schema by deleting the associated consent elements
	// If consent deletion fails, we log the error but do NOT re-create the schema
	// since orphaned consent elements are safe and won't cause active harm.
	if us.consentService.IsEnabled() && len(attributeNames) > 0 {
		if svcErr := us.deleteConsentElements(ctx, attributeNames, logger); svcErr != nil {
			logger.Error(ctx, "Failed to delete consent elements for removed schema attributes; "+
				"orphaned consent elements may remain but schema deletion succeeded",
				log.Any("attributeNames", attributeNames), log.Any("error", svcErr))
		}
	}

	return nil
}

// ValidateEntity validates entity attributes against the schema for the given category and entity type.
func (us *entityTypeService) ValidateEntity(
	ctx context.Context, category TypeCategory, entityType string, attributes json.RawMessage,
	skipCredentialRequired bool,
) (bool, *serviceerror.ServiceError) {
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

	isValid, err := compiledSchema.Validate(ctx, attributes, logger, skipCredentialRequired)
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
) (bool, *serviceerror.ServiceError) {
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

// GetAttributes returns schema properties filtered by the provided flags for the given entity type.
// allowCredential includes credential properties; allowNonCredential includes non-credential properties.
// When requiredOnly is true, only required properties are included.
func (us *entityTypeService) GetAttributes(
	ctx context.Context, category TypeCategory, entityType string,
	allowCredential, allowNonCredential, requiredOnly bool,
) ([]AttributeInfo, *serviceerror.ServiceError) {
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

	return compiledSchema.GetAttributes(allowCredential, allowNonCredential, requiredOnly), nil
}

// GetUniqueAttributes returns the names of schema properties marked as unique for a given entity type.
func (us *entityTypeService) GetUniqueAttributes(
	ctx context.Context, category TypeCategory, entityType string,
) ([]string, *serviceerror.ServiceError) {
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

// GetDisplayAttributesByNames returns display attributes for multiple entity types by name within a category.
func (us *entityTypeService) GetDisplayAttributesByNames(
	ctx context.Context, category TypeCategory, names []string,
) (map[string]string, *serviceerror.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))

	if svcErr := validateCategory(category); svcErr != nil {
		return nil, svcErr
	}

	if len(names) == 0 {
		return map[string]string{}, nil
	}

	result, err := us.entityTypeStore.GetDisplayAttributesByNames(ctx, category, names)
	if err != nil {
		return nil, logAndReturnServerError(ctx, logger, "Failed to get display attributes by names", err)
	}

	return result, nil
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

	found, err := us.entityTypeStore.GetEntityTypeByName(ctx, category, entityType)
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
) *serviceerror.ServiceError {
	if us.authzService == nil {
		return nil
	}
	allowed, svcErr := us.authzService.IsActionAllowed(ctx, action,
		&sysauthz.ActionContext{ResourceType: resourceTypeForCategory(category), OUID: ouID})
	if svcErr != nil {
		return &serviceerror.InternalServerError
	}
	if !allowed {
		return &serviceerror.ErrorUnauthorized
	}
	return nil
}

// getAccessibleResources returns the set of OU IDs the caller is permitted to access for the
// given list action. The action implies the resource type (entity type vs agent schema).
func (us *entityTypeService) getAccessibleResources(
	ctx context.Context, action security.Action,
) (*sysauthz.AccessibleResources, *serviceerror.ServiceError) {
	if us.authzService == nil {
		return &sysauthz.AccessibleResources{AllAllowed: true}, nil
	}
	resourceType := security.ResourceTypeUserType
	if action == security.ActionListAgentTypes {
		resourceType = security.ResourceTypeAgentType
	}
	accessible, svcErr := us.authzService.GetAccessibleResources(
		ctx, action, resourceType)
	if svcErr != nil {
		return nil, &serviceerror.InternalServerError
	}
	return accessible, nil
}

// ResolveEntityTypeHandles resolves ou_handle to an OU ID on the given entity type in-place.
// Called by the declarative loader validator (startup, no user context) so that file-based
// entity types support ou_handle. It elevates to runtime context internally.
func (us *entityTypeService) ResolveEntityTypeHandles(
	ctx context.Context, entityType *EntityType,
) *serviceerror.ServiceError {
	return us.resolveEntityTypeOUHandle(security.WithRuntimeContext(ctx), entityType)
}

// resolveEntityTypeOUHandle resolves ou_handle to an OU ID on the given entity type in-place
// using the caller's context. API/importer paths must pass their own caller context so the
// underlying OU lookup is still subject to the caller's ou:read authorization.
// If both ou_id and ou_handle are provided, ou_id wins and a warning is logged.
func (us *entityTypeService) resolveEntityTypeOUHandle(
	ctx context.Context, entityType *EntityType,
) *serviceerror.ServiceError {
	if entityType.OUID != "" && entityType.OUHandle != "" {
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, entityTypeLoggerComponentName))
		logger.Warn(ctx, "Both ou_id and ou_handle provided for entity type; ou_handle ignored",
			log.String("entityTypeID", entityType.ID), log.String("name", entityType.Name))
		return nil
	}
	if entityType.OUID == "" && entityType.OUHandle != "" {
		if us.ouService == nil {
			return &serviceerror.InternalServerError
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
) *serviceerror.ServiceError {
	if us.ouService == nil {
		logger.Error(ctx, "Organization unit service is not configured for entity type operations")
		return &serviceerror.InternalServerError
	}

	exists, svcErr := us.ouService.IsOrganizationUnitExists(ctx, oUID)
	if svcErr != nil {
		logger.Error(ctx, "Failed to verify organization unit existence",
			log.String("oUID", oUID), log.Any("error", svcErr))
		return &serviceerror.InternalServerError
	}

	if !exists {
		logger.Debug(ctx, "Organization unit does not exist",
			log.String("oUID", oUID))
		return invalidEntityTypeRequestErr(category, "organization unit id does not exist")
	}

	return nil
}

// validatePaginationParams validates the limit and offset parameters.
func validatePaginationParams(limit, offset int) *serviceerror.ServiceError {
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
	if category == TypeCategoryAgent {
		return "/agent-types"
	}
	return "/user-types"
}

// validateCategory ensures the supplied category is one of the supported values.
func validateCategory(category TypeCategory) *serviceerror.ServiceError {
	if !category.IsValid() {
		return invalidEntityTypeRequestErr(category, "invalid schema category")
	}
	return nil
}

// listActionForCategory returns the sysauthz list action that gates listing schemas of the given
// category.
func listActionForCategory(category TypeCategory) security.Action {
	if category == TypeCategoryAgent {
		return security.ActionListAgentTypes
	}
	return security.ActionListUserTypes
}

// createActionForCategory returns the sysauthz create action for the given category.
func createActionForCategory(category TypeCategory) security.Action {
	if category == TypeCategoryAgent {
		return security.ActionCreateAgentType
	}
	return security.ActionCreateUserType
}

// readActionForCategory returns the sysauthz read action for the given category.
func readActionForCategory(category TypeCategory) security.Action {
	if category == TypeCategoryAgent {
		return security.ActionReadAgentType
	}
	return security.ActionReadUserType
}

// updateActionForCategory returns the sysauthz update action for the given category.
func updateActionForCategory(category TypeCategory) security.Action {
	if category == TypeCategoryAgent {
		return security.ActionUpdateAgentType
	}
	return security.ActionUpdateUserType
}

// deleteActionForCategory returns the sysauthz delete action for the given category.
func deleteActionForCategory(category TypeCategory) security.Action {
	if category == TypeCategoryAgent {
		return security.ActionDeleteAgentType
	}
	return security.ActionDeleteUserType
}

// resourceTypeForCategory returns the sysauthz resource type for the given category.
func resourceTypeForCategory(category TypeCategory) security.ResourceType {
	if category == TypeCategoryAgent {
		return security.ResourceTypeAgentType
	}
	return security.ResourceTypeUserType
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
) *serviceerror.ServiceError {
	logger.Error(ctx, message, log.Error(err))
	return &serviceerror.InternalServerError
}

// validateEntityTypeDefinition validates the entity type definition without checking OU existence.
// This is used during initialization to validate file-based configurations.
func validateEntityTypeDefinition(
	ctx context.Context, category TypeCategory, schema EntityType) *serviceerror.ServiceError {
	logger := log.GetLogger()

	if schema.Name == "" {
		logger.Debug(ctx, "Entity type validation failed: name is empty")
		return invalidEntityTypeRequestErr(category, "entity type name must not be empty")
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

	return validateSystemAttributes(compiledSchema, schema.SystemAttributes)
}

// validateSystemAttributes validates the system attributes against the compiled schema.
func validateSystemAttributes(
	compiledSchema *model.Schema, systemAttrs *SystemAttributes,
) *serviceerror.ServiceError {
	if systemAttrs == nil {
		return nil
	}

	return validateDisplayAttribute(compiledSchema, systemAttrs.Display)
}

// validateDisplayAttribute validates that the display attribute, if provided,
// references an existing, displayable, non-credential attribute in the compiled schema.
// Only string and number types are considered displayable.
func validateDisplayAttribute(
	compiledSchema *model.Schema, display string,
) *serviceerror.ServiceError {
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

// syncConsentElementsOnCreate creates missing consent elements for a new schema creation.
func (us *entityTypeService) syncConsentElementsOnCreate(ctx context.Context,
	category TypeCategory, schema json.RawMessage, logger *log.Logger) *serviceerror.ServiceError {
	// TODO: Replace "default" with the schema's actual OU when applications are associated with OUs.
	const ouID = "default"

	logger.Debug(ctx, "Synchronizing consent elements for the new schema", log.String("ouID", ouID))

	names, err := extractAttributeNames(category, schema)
	if err != nil {
		return err
	}

	if len(names) > 0 {
		logger.Debug(ctx, "Creating missing consent elements for the new schema",
			log.String("ouID", ouID), log.Int("elementCount", len(names)))
		if svcErr := us.createMissingConsentElements(ctx, ouID, names, logger); svcErr != nil {
			return svcErr
		}
	}

	return nil
}

// syncConsentElementsOnUpdate reconciles consent elements when a schema is updated.
// It creates elements that were added and deletes elements that were removed.
func (us *entityTypeService) syncConsentElementsOnUpdate(ctx context.Context,
	category TypeCategory, oldSchema, newSchema json.RawMessage, logger *log.Logger) *serviceerror.ServiceError {
	// TODO: Replace "default" with the schema's actual OU when applications are associated with OUs.
	const ouID = "default"

	logger.Debug(ctx, "Synchronizing consent elements for the updated schema", log.String("ouID", ouID))

	oldAttrs, err := extractAttributeNamesAsMap(category, oldSchema)
	if err != nil {
		return err
	}

	newAttrs, err := extractAttributeNamesAsMap(category, newSchema)
	if err != nil {
		return err
	}

	// Create consent elements for new attributes that were added in the updated schema.
	// createMissingConsentElements method will handle filtering out existing elements, so we can pass all
	// new attribute names here. This ensures that even consent service was disabled when creating the schema,
	// the necessary consent elements are created when updating the schema with consent service enabled.
	requiredNames := make([]string, 0, len(newAttrs))
	for name := range newAttrs {
		requiredNames = append(requiredNames, name)
	}

	if len(requiredNames) > 0 {
		logger.Debug(ctx, "Ensuring consent elements exist for all requested attributes",
			log.String("ouID", ouID), log.Int("requiredAttributesCount", len(requiredNames)))
		if err := us.createMissingConsentElements(ctx, ouID, requiredNames, logger); err != nil {
			return err
		}
	}

	// Delete variables that are no longer part of the current payload
	var removedNames []string
	for name := range oldAttrs {
		if _, exists := newAttrs[name]; !exists {
			removedNames = append(removedNames, name)
		}
	}

	return us.deleteConsentElements(ctx, removedNames, logger)
}

// createMissingConsentElements validates a list of consent element names and creates only
// the missing ones.
// nolint:unparam // ouID is always "default" in current usage but kept for future flexibility
func (us *entityTypeService) createMissingConsentElements(ctx context.Context,
	ouID string, names []string, logger *log.Logger) *serviceerror.ServiceError {
	if len(names) == 0 {
		logger.Debug(ctx, "No consent elements to create for the schema", log.String("ouID", ouID))
		return nil
	}

	logger.Debug(ctx, "Validating consent elements for the schema attributes",
		log.String("ouID", ouID), log.Int("elementCount", len(names)))

	validNames, err := us.consentService.ValidateConsentElements(ctx, ouID, names)
	if err != nil {
		return wrapConsentServiceError(ctx, err, logger)
	}

	// Create a map of existing elements for fast lookup
	existingMap := make(map[string]bool, len(validNames))
	for _, name := range validNames {
		existingMap[name] = true
	}

	// Filter out the existing elements
	var elementsToCreate []consent.ConsentElementInput
	for _, name := range names {
		if !existingMap[name] {
			elementsToCreate = append(elementsToCreate, consent.ConsentElementInput{
				Name:      name,
				Namespace: consent.NamespaceAttribute,
			})
		}
	}

	if len(elementsToCreate) > 0 {
		logger.Debug(ctx, "Creating new consent elements for the schema attributes",
			log.String("ouID", ouID), log.Int("elementCount", len(elementsToCreate)))
		if _, err := us.consentService.CreateConsentElements(ctx, ouID, elementsToCreate); err != nil {
			return wrapConsentServiceError(ctx, err, logger)
		}
	}

	return nil
}

// deleteConsentElements removes a list of consent elements associated with the given attribute names.
func (us *entityTypeService) deleteConsentElements(ctx context.Context,
	attributeNames []string, logger *log.Logger) *serviceerror.ServiceError {
	// TODO: Replace "default" with the schema's actual OU when applications are associated with OUs.
	const ouID = "default"

	logger.Debug(ctx, "Deleting consent elements for the removed schema attributes",
		log.String("ouID", ouID), log.Int("elementCount", len(attributeNames)))

	if len(attributeNames) == 0 {
		logger.Debug(ctx, "No consent elements to delete for the schema", log.String("ouID", ouID))
		return nil
	}

	for _, attrName := range attributeNames {
		// List existing consent elements for the removed attribute to find their IDs for deletion
		existing, err := us.consentService.ListConsentElements(ctx, ouID, consent.NamespaceAttribute, attrName)
		if err != nil {
			return wrapConsentServiceError(ctx, err, logger)
		}

		// Delete the first element if the list is not empty.
		// We assume there is only one consent element per attribute name.
		// TODO: This should be revisited when user type separation is onboarded to consent elements.
		if len(existing) > 0 {
			logger.Debug(ctx, "Deleting consent element for the removed schema attribute",
				log.String("ouID", ouID), log.String("attribute", attrName), log.String("elementID", existing[0].ID))
			if err := us.consentService.DeleteConsentElement(ctx, ouID, existing[0].ID); err != nil {
				// Silently ignore the error if it's due to associated purposes, but log a warning.
				// The same attribute can exist in a different schema and purpose can be associated with that,
				// so we should not block the schema update in that case.
				// If it's not associated with a purpose, but exists in a different schema, we still delete it,
				// as the consent element can be created again when configuring attribute for a application.
				if err.Code == consent.ErrorDeletingConsentElementWithAssociatedPurpose.Code {
					logger.Warn(ctx,
						"Cannot delete consent element for removed attribute due to associated purposes",
						log.String("attribute", attrName), log.String("elementID", existing[0].ID),
						log.String("error", err.ErrorDescription.DefaultValue))
					continue
				}

				return wrapConsentServiceError(ctx, err, logger)
			}
		}
	}

	return nil
}

// extractAttributeNames returns the set of attribute names from a schema JSON as a string slice.
func extractAttributeNames(category TypeCategory, schema json.RawMessage) ([]string, *serviceerror.ServiceError) {
	if len(schema) == 0 {
		return nil, nil
	}

	var schemaMap map[string]json.RawMessage
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		return nil, invalidEntityTypeRequestErr(category, "invalid schema json: "+err.Error())
	}

	names := make([]string, 0, len(schemaMap))
	for name := range schemaMap {
		names = append(names, name)
	}

	return names, nil
}

// extractAttributeNamesAsMap returns the set of attribute names from a schema JSON as a map
// for last lookups.
func extractAttributeNamesAsMap(
	category TypeCategory, schema json.RawMessage,
) (map[string]bool, *serviceerror.ServiceError) {
	result := make(map[string]bool)
	if len(schema) == 0 {
		return result, nil
	}

	var schemaMap map[string]json.RawMessage
	if err := json.Unmarshal(schema, &schemaMap); err != nil {
		return nil, invalidEntityTypeRequestErr(category, "invalid schema json: "+err.Error())
	}

	for name := range schemaMap {
		result[name] = true
	}

	return result, nil
}

// wrapConsentServiceError converts an ServiceError from the consent service into a ServiceError
// for the entity type service.
func wrapConsentServiceError(
	ctx context.Context, err *serviceerror.ServiceError, logger *log.Logger) *serviceerror.ServiceError {
	if err == nil {
		return nil
	}

	if err.Type == serviceerror.ClientErrorType {
		logger.Debug(ctx, "Failed to sync consent elements for the schema changes", log.Any("error", err))
		return serviceerror.CustomServiceError(ErrorConsentSyncFailed, core.I18nMessage{
			Key:          "error.entitytypeservice.consent_sync_failed_description",
			DefaultValue: fmt.Sprintf("%s : code - %s", ErrorConsentSyncFailed.ErrorDescription.DefaultValue, err.Code),
		})
	}

	logger.Error(ctx, "Failed to sync consent elements for the schema changes", log.Any("error", err))
	return &serviceerror.InternalServerError
}

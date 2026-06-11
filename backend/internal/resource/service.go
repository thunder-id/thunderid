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

// Package resource implements the resource management service.
package resource

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/consent"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const (
	loggerComponentName = "ResourceMgtService"

	// ValidPermissionDelimiters defines the allowed delimiter characters in permission strings.
	// Exported so callers parsing permission strings (e.g. consent enforcer rollup) can share the
	// single source of truth.
	ValidPermissionDelimiters = "._:-/"

	// validPermissionCharacters defines the allowed characters for permission strings.
	// Allowed: a-z A-Z 0-9 and delimiter characters (. _ : - /)
	validPermissionCharacters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789" +
		ValidPermissionDelimiters
)

// IsPermissionDelimiter reports whether r is one of the allowed delimiter characters in a
// permission string (see ValidPermissionDelimiters).
func IsPermissionDelimiter(r rune) bool {
	return strings.ContainsRune(ValidPermissionDelimiters, r)
}

// ResourceServiceInterface defines the interface for the resource service.
type ResourceServiceInterface interface {
	// Resource Server operations
	CreateResourceServer(ctx context.Context, rs ResourceServer) (*ResourceServer, *serviceerror.ServiceError)
	GetResourceServer(ctx context.Context, id string) (*ResourceServer, *serviceerror.ServiceError)
	GetResourceServerList(ctx context.Context, limit, offset int) (*ResourceServerList, *serviceerror.ServiceError)
	UpdateResourceServer(
		ctx context.Context, id string, rs ResourceServer,
	) (*ResourceServer, *serviceerror.ServiceError)
	DeleteResourceServer(ctx context.Context, id string) *serviceerror.ServiceError
	GetResourceServerByIdentifier(
		ctx context.Context, identifier string,
	) (*ResourceServer, *serviceerror.ServiceError)
	IsResourceServerDeclarative(id string) bool

	// Resource operations
	CreateResource(ctx context.Context, resourceServerID string, res Resource) (
		*Resource, *serviceerror.ServiceError)
	GetResource(ctx context.Context, resourceServerID, id string) (*Resource, *serviceerror.ServiceError)
	GetResourceList(
		ctx context.Context, resourceServerID string, parentID *string, limit, offset int,
	) (*ResourceList, *serviceerror.ServiceError)
	GetAllResourceList(
		ctx context.Context, resourceServerID string,
	) ([]Resource, *serviceerror.ServiceError)
	UpdateResource(
		ctx context.Context, resourceServerID, id string, res Resource,
	) (*Resource, *serviceerror.ServiceError)
	DeleteResource(ctx context.Context, resourceServerID, id string) *serviceerror.ServiceError

	// Action operations
	CreateAction(
		ctx context.Context, resourceServerID string, resourceID *string, action Action,
	) (*Action, *serviceerror.ServiceError)
	GetAction(
		ctx context.Context, resourceServerID string, resourceID *string, id string,
	) (*Action, *serviceerror.ServiceError)
	GetActionList(
		ctx context.Context, resourceServerID string, resourceID *string, limit, offset int,
	) (*ActionList, *serviceerror.ServiceError)
	UpdateAction(
		ctx context.Context, resourceServerID string, resourceID *string, id string, action Action,
	) (*Action, *serviceerror.ServiceError)
	DeleteAction(ctx context.Context, resourceServerID string, resourceID *string,
		id string) *serviceerror.ServiceError
	ValidatePermissions(
		ctx context.Context, resourceServerID string, permissions []string,
	) ([]string, *serviceerror.ServiceError)

	// FindResourceServersByPermissions returns registered resource servers that define at least
	// one permission in the supplied set. Used by the OAuth2 token layer to populate aud when no
	// explicit resource parameter was supplied.
	FindResourceServersByPermissions(
		ctx context.Context, permissions []string,
	) ([]ResourceServer, *serviceerror.ServiceError)

	// ResolveResourceServerOUHandle resolves ou_handle to an OU ID on the given resource server
	// in-place. Called by the declarative loader validator so that file-based resource servers
	// support ou_handle.
	ResolveResourceServerOUHandle(
		ctx context.Context, rs *ResourceServer,
	) *serviceerror.ServiceError
}

// resourceService is the default implementation of ResourceServiceInterface.
type resourceService struct {
	logger           log.Logger
	resourceStore    resourceStoreInterface
	ouService        oupkg.OrganizationUnitServiceInterface
	consentService   consent.ConsentServiceInterface
	defaultDelimiter string
	transactioner    transaction.Transactioner
}

// newResourceService creates a new instance of ResourceService.
func newResourceService(
	ouService oupkg.OrganizationUnitServiceInterface,
	consentService consent.ConsentServiceInterface,
	resourceStore resourceStoreInterface,
	transactionerInstance transaction.Transactioner,
) (ResourceServiceInterface, error) {
	// Load default delimiter from config
	defaultDelimiter := getDefaultDelimiter()
	if err := validateDelimiter(defaultDelimiter); err != nil {
		return nil, fmt.Errorf("configured permission delimiter is invalid")
	}

	return &resourceService{
		logger:           *log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
		resourceStore:    resourceStore,
		ouService:        ouService,
		consentService:   consentService,
		defaultDelimiter: defaultDelimiter,
		transactioner:    transactionerInstance,
	}, nil
}

// Resource Server Methods

// CreateResourceServer creates a new resource server.
func (rs *resourceService) CreateResourceServer(
	ctx context.Context,
	resourceServer ResourceServer,
) (*ResourceServer, *serviceerror.ServiceError) {
	rs.logger.Debug(ctx, "Creating resource server", log.String("name", resourceServer.Name))

	if err := rs.validateResourceServerCreate(resourceServer); err != nil {
		return nil, err
	}

	// Validate organization unit exists
	_, svcErr := rs.ouService.GetOrganizationUnit(ctx, resourceServer.OUID)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			rs.logger.Debug(ctx, "Organization unit not found", log.String("ouID", resourceServer.OUID))
			return nil, &ErrorOrganizationUnitNotFound
		}
		rs.logger.Error(ctx, "Failed to validate organization unit",
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &serviceerror.InternalServerError
	}

	// Check name uniqueness
	nameExists, err := rs.resourceStore.CheckResourceServerNameExists(ctx, resourceServer.Name)
	if err != nil {
		rs.logger.Error(ctx, "Failed to check resource server name", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if nameExists {
		rs.logger.Debug(ctx, "Resource server name already exists", log.String("name", resourceServer.Name))
		return nil, &ErrorNameConflict
	}

	// Check handle uniqueness (if provided)
	if resourceServer.Handle != "" {
		handleExists, err := rs.resourceStore.CheckResourceServerHandleExists(ctx, resourceServer.Handle)
		if err != nil {
			rs.logger.Error(ctx, "Failed to check resource server handle", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
		if handleExists {
			rs.logger.Debug(ctx, "Resource server handle already exists",
				log.String("handle", resourceServer.Handle))
			return nil, &ErrorHandleConflict
		}
	}

	// Check identifier uniqueness (if provided)
	if resourceServer.Identifier != "" {
		identifierExists, err := rs.resourceStore.CheckResourceServerIdentifierExists(ctx, resourceServer.Identifier)
		if err != nil {
			rs.logger.Error(ctx, "Failed to check resource server identifier", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
		if identifierExists {
			rs.logger.Debug(ctx, "Resource server identifier already exists",
				log.String("identifier", resourceServer.Identifier))
			return nil, &ErrorIdentifierConflict
		}
	}

	// Set default type if not provided
	if resourceServer.Type == "" {
		resourceServer.Type = ResourceServerTypeCustom
	}

	// Set default delimiter if not provided
	if resourceServer.Delimiter == "" {
		resourceServer.Delimiter = rs.defaultDelimiter
	}

	// Validate handle format and ensure it does not contain the delimiter character
	if resourceServer.Handle != "" {
		if svcErr := validateHandle(resourceServer.Handle, resourceServer.Delimiter); svcErr != nil {
			if svcErr.Code == ErrorDelimiterInHandle.Code {
				return nil, &ErrorDelimiterInResourceServerHandle
			}
			return nil, svcErr
		}
	}

	id := resourceServer.ID
	if id == "" {
		var err error
		id, err = utils.GenerateUUIDv7()
		if err != nil {
			rs.logger.Error(ctx, "Failed to generate UUID", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
	} else {
		_, svcErr := rs.GetResourceServer(ctx, id)
		if svcErr != nil && svcErr.Code != ErrorResourceServerNotFound.Code {
			return nil, svcErr
		}
		if svcErr == nil {
			rs.logger.Debug(ctx, "Resource server ID already exists", log.String("id", id))
			return nil, &ErrorResourceServerIDConflict
		}
	}

	// Use transaction for write operation
	var createdRS *ResourceServer
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.CreateResourceServer(txCtx, id, resourceServer); err != nil {
			rs.logger.Error(ctx, "Failed to create resource server", log.Error(err))
			return err
		}

		createdRS = &ResourceServer{
			ID:          id,
			Name:        resourceServer.Name,
			Description: resourceServer.Description,
			Handle:      resourceServer.Handle,
			Identifier:  resourceServer.Identifier,
			Type:        resourceServer.Type,
			OUID:        resourceServer.OUID,
			Delimiter:   resourceServer.Delimiter,
		}
		return nil
	}); err != nil {
		return nil, &serviceerror.InternalServerError
	}

	rs.logger.Debug(ctx, "Successfully created resource server", log.String("id", id))
	return createdRS, nil
}

// GetResourceServer retrieves a resource server by ID.
func (rs *resourceService) GetResourceServer(
	ctx context.Context, id string,
) (*ResourceServer, *serviceerror.ServiceError) {
	if id == "" {
		return nil, &ErrorMissingID
	}

	resourceServer, err := rs.resourceStore.GetResourceServer(ctx, id)
	if err != nil {
		if errors.Is(err, errResourceServerNotFound) {
			rs.logger.Debug(ctx, "Resource server not found", log.String("id", id))
			return nil, &ErrorResourceServerNotFound
		}
		rs.logger.Error(ctx, "Failed to get resource server", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return &resourceServer, nil
}

// GetResourceServerByIdentifier retrieves a resource server by its identifier.
func (rs *resourceService) GetResourceServerByIdentifier(
	ctx context.Context, identifier string,
) (*ResourceServer, *serviceerror.ServiceError) {
	if identifier == "" {
		return nil, &ErrorResourceServerNotFound
	}

	resourceServer, err := rs.resourceStore.GetResourceServerByIdentifier(ctx, identifier)
	if err != nil {
		if errors.Is(err, errResourceServerNotFound) {
			rs.logger.Debug(ctx, "Resource server not found for identifier",
				log.String("identifier", identifier))
			return nil, &ErrorResourceServerNotFound
		}
		rs.logger.Error(ctx, "Failed to get resource server by identifier", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return &resourceServer, nil
}

// GetResourceServerList retrieves a paginated list of resource servers.
func (rs *resourceService) GetResourceServerList(
	ctx context.Context, limit, offset int,
) (*ResourceServerList, *serviceerror.ServiceError) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	totalCount, err := rs.resourceStore.GetResourceServerListCount(ctx)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to get resource server count", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	resourceServers, err := rs.resourceStore.GetResourceServerList(ctx, limit, offset)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to list resource servers", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	response := &ResourceServerList{
		TotalResults:    totalCount,
		ResourceServers: resourceServers,
		StartIndex:      offset + 1,
		Count:           len(resourceServers),
		Links:           buildPaginationLinks("/resource-servers", limit, offset, totalCount),
	}

	return response, nil
}

// UpdateResourceServer updates a resource server.
func (rs *resourceService) UpdateResourceServer(
	ctx context.Context,
	id string, resourceServer ResourceServer,
) (*ResourceServer, *serviceerror.ServiceError) {
	if id == "" {
		return nil, &ErrorMissingID
	}

	if err := rs.validateResourceServerUpdate(resourceServer); err != nil {
		return nil, err
	}

	existingResServer, err := rs.resourceStore.GetResourceServer(ctx, id)
	if err != nil {
		if errors.Is(err, errResourceServerNotFound) {
			rs.logger.Debug(ctx, "Resource server not found", log.String("id", id))
			return nil, &ErrorResourceServerNotFound
		}
		rs.logger.Error(ctx, "Failed to check resource server existence", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(id) {
		rs.logger.Debug(ctx, "Cannot modify declarative resource server", log.String("id", id))
		return nil, serviceerror.CustomServiceError(ErrorImmutableResourceServer, core.I18nMessage{
			Key:          ErrorImmutableResourceServer.ErrorDescription.Key,
			DefaultValue: fmt.Sprintf(ErrorImmutableResourceServer.ErrorDescription.DefaultValue, id),
		})
	}

	// Delimiter is always preserved from the existing record
	resourceServer.Delimiter = existingResServer.Delimiter

	// Type is immutable and always preserved from the existing record
	resourceServer.Type = existingResServer.Type

	// Handle is immutable after creation. Preserve existing when omitted; reject any change.
	if resourceServer.Handle == "" {
		resourceServer.Handle = existingResServer.Handle
	} else if resourceServer.Handle != existingResServer.Handle {
		return nil, &ErrorImmutableHandle
	}

	// Identifier: preserve existing if not provided; check uniqueness if changed
	if resourceServer.Identifier == "" {
		resourceServer.Identifier = existingResServer.Identifier
	} else if resourceServer.Identifier != existingResServer.Identifier {
		identifierExists, err := rs.resourceStore.CheckResourceServerIdentifierExists(ctx, resourceServer.Identifier)
		if err != nil {
			rs.logger.Error(ctx, "Failed to check resource server identifier", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
		if identifierExists {
			rs.logger.Debug(ctx, "Resource server identifier already exists",
				log.String("identifier", resourceServer.Identifier))
			return nil, &ErrorIdentifierConflict
		}
	}

	// Validate organization unit
	_, svcErr := rs.ouService.GetOrganizationUnit(ctx, resourceServer.OUID)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return nil, &ErrorOrganizationUnitNotFound
		}
		return nil, &serviceerror.InternalServerError
	}

	// Check name uniqueness, if changed
	if existingResServer.Name != resourceServer.Name {
		nameExists, err := rs.resourceStore.CheckResourceServerNameExists(ctx, resourceServer.Name)
		if err != nil {
			rs.logger.Error(ctx, "Failed to check resource server name", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
		if nameExists {
			return nil, &ErrorNameConflict
		}
	}

	var updatedRS *ResourceServer
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.UpdateResourceServer(txCtx, id, resourceServer); err != nil {
			rs.logger.Error(ctx, "Failed to update resource server", log.Error(err))
			return err
		}

		updatedRS = &ResourceServer{
			ID:          id,
			Name:        resourceServer.Name,
			Description: resourceServer.Description,
			Handle:      resourceServer.Handle,
			Identifier:  resourceServer.Identifier,
			Type:        resourceServer.Type,
			OUID:        resourceServer.OUID,
			Delimiter:   resourceServer.Delimiter,
		}
		return nil
	}); err != nil {
		return nil, &serviceerror.InternalServerError
	}

	return updatedRS, nil
}

// DeleteResourceServer deletes a resource server.
func (rs *resourceService) DeleteResourceServer(ctx context.Context, id string) *serviceerror.ServiceError {
	if id == "" {
		return &ErrorMissingID
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(id) {
		rs.logger.Debug(ctx, "Cannot delete declarative resource server", log.String("id", id))
		return serviceerror.CustomServiceError(ErrorImmutableResourceServer, core.I18nMessage{
			Key:          ErrorImmutableResourceServer.ErrorDescription.Key,
			DefaultValue: fmt.Sprintf(ErrorImmutableResourceServer.ErrorDescription.DefaultValue, id),
		})
	}

	_, err := rs.resourceStore.GetResourceServer(ctx, id)
	if err != nil {
		if errors.Is(err, errResourceServerNotFound) {
			return nil // Idempotent delete
		}
		rs.logger.Error(ctx, "Failed to check resource server existence", log.Error(err))
		return &serviceerror.InternalServerError
	}

	// Check for dependencies
	hasDeps, err := rs.resourceStore.CheckResourceServerHasDependencies(ctx, id)
	if err != nil {
		rs.logger.Error(ctx, "Failed to check dependencies", log.Error(err))
		return &serviceerror.InternalServerError
	}
	if hasDeps {
		return &ErrorCannotDelete
	}

	// Use transaction for write operation
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.DeleteResourceServer(txCtx, id); err != nil {
			rs.logger.Error(ctx, "Failed to delete resource server", log.Error(err))
			return err
		}
		return nil
	}); err != nil {
		return &serviceerror.InternalServerError
	}

	return nil
}

// IsResourceServerDeclarative checks if a resource server is declarative (immutable).
func (rs *resourceService) IsResourceServerDeclarative(id string) bool {
	return rs.resourceStore.IsResourceServerDeclarative(id)
}

// Resource operations

// CreateResource creates a new resource under a resource server.
func (rs *resourceService) CreateResource(
	ctx context.Context,
	resourceServerID string, resource Resource,
) (*Resource, *serviceerror.ServiceError) {
	// Validate resource server exists
	resourceServer, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	if err := rs.validateResourceCreate(resource, resourceServer.Delimiter); err != nil {
		return nil, err
	}

	// Validate parent if specified
	var parentResource *Resource
	if resource.Parent != nil {
		res, err := rs.resourceStore.GetResource(ctx, *resource.Parent, resourceServerID)
		if err != nil {
			if errors.Is(err, errResourceNotFound) {
				return nil, &ErrorParentResourceNotFound
			}
			rs.logger.Error(ctx, "Failed to check parent resource", log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
		parentResource = &res
	}

	// Check handle uniqueness under parent
	handleExists, err := rs.resourceStore.CheckResourceHandleExists(
		ctx, resourceServerID, resource.Handle, resource.Parent,
	)
	if err != nil {
		rs.logger.Error(ctx, "Failed to check resource handle", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if handleExists {
		return nil, &ErrorHandleConflict
	}

	// Derive permission string based on hierarchy
	resource.Permission = derivePermission(resourceServer, parentResource, resource.Handle)

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		rs.logger.Error(ctx, "Failed to generate UUID", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Use transaction for write operation
	var createdResource *Resource
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.CreateResource(
			txCtx, id, resourceServerID, resource.Parent, resource,
		); err != nil {
			rs.logger.Error(ctx, "Failed to create resource", log.Error(err))
			return err
		}

		if err := rs.syncConsentOnPermissionCreate(
			txCtx, resource.Permission, resource.Description,
		); err != nil {
			rs.logger.Error(ctx, "Failed to sync consent element for resource", log.Error(err))
			return err
		}

		createdResource = &Resource{
			ID:          id,
			Name:        resource.Name,
			Handle:      resource.Handle,
			Description: resource.Description,
			Parent:      resource.Parent,
			Permission:  resource.Permission,
		}
		return nil
	}); err != nil {
		return nil, translateTxError(err)
	}

	return createdResource, nil
}

// GetResource retrieves a resource by ID.
func (rs *resourceService) GetResource(
	ctx context.Context, resourceServerID, id string,
) (*Resource, *serviceerror.ServiceError) {
	if id == "" || resourceServerID == "" {
		return nil, &ErrorMissingID
	}

	// Validate resource server exists
	_, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	resource, err := rs.resourceStore.GetResource(ctx, id, resourceServerID)
	if err != nil {
		if errors.Is(err, errResourceNotFound) {
			return nil, &ErrorResourceNotFound
		}
		rs.logger.Error(ctx, "Failed to get resource", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	return &resource, nil
}

// GetResourceList retrieves a paginated list of resources.
func (rs *resourceService) GetResourceList(
	ctx context.Context,
	resourceServerID string, parentID *string, limit, offset int,
) (*ResourceList, *serviceerror.ServiceError) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}
	if resourceServerID == "" {
		return nil, &ErrorMissingID
	}
	// Validate resource server exists
	_, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	var totalCount int
	var resources []Resource

	// Resolve parent if specified
	if parentID != nil {
		// ParentID specified - validate it exists
		_, svcErr := rs.validateAndGetResourceByID(ctx, *parentID, resourceServerID)
		if svcErr != nil {
			return nil, svcErr
		}
	}

	totalCount, err := rs.resourceStore.GetResourceListCountByParent(ctx, resourceServerID, parentID)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to get top-level resource count", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	resources, err = rs.resourceStore.GetResourceListByParent(ctx, resourceServerID, parentID, limit, offset)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to list resources", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	baseURL := fmt.Sprintf("/resource-servers/%s/resources", resourceServerID)
	response := &ResourceList{
		TotalResults: totalCount,
		Resources:    resources,
		StartIndex:   offset + 1,
		Count:        len(resources),
		Links:        buildPaginationLinks(baseURL, limit, offset, totalCount),
	}

	return response, nil
}

// GetAllResourceList retrieves all resources for a resource server without pagination.
func (rs *resourceService) GetAllResourceList(
	ctx context.Context, resourceServerID string,
) ([]Resource, *serviceerror.ServiceError) {
	if resourceServerID == "" {
		return nil, &ErrorMissingID
	}
	if _, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID); svcErr != nil {
		return nil, svcErr
	}

	totalCount, err := rs.resourceStore.GetResourceListCount(ctx, resourceServerID)
	if err != nil {
		rs.logger.Error(ctx, "Failed to get resource count", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if totalCount == 0 {
		return []Resource{}, nil
	}

	resources, err := rs.resourceStore.GetResourceList(ctx, resourceServerID, totalCount, 0)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to list all resources", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	return resources, nil
}

// UpdateResource updates a resource.
func (rs *resourceService) UpdateResource(
	ctx context.Context,
	resourceServerID, id string, resource Resource,
) (*Resource, *serviceerror.ServiceError) {
	if id == "" || resourceServerID == "" {
		return nil, &ErrorMissingID
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(resourceServerID) {
		rs.logger.Debug(ctx,
			"Cannot modify resource in declarative resource server",
			log.String("resource_server_id", resourceServerID),
		)
		return nil, serviceerror.CustomServiceError(ErrorImmutableResource, core.I18nMessage{
			Key:          ErrorImmutableResource.ErrorDescription.Key,
			DefaultValue: fmt.Sprintf(ErrorImmutableResource.ErrorDescription.DefaultValue, id),
		})
	}

	// Validate resource server exists
	_, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Validate resource exists
	currentResource, err := rs.resourceStore.GetResource(ctx, id, resourceServerID)
	if err != nil {
		if errors.Is(err, errResourceNotFound) {
			return nil, &ErrorResourceNotFound
		}
		rs.logger.Error(ctx, "Failed to check resource existence", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Update only mutable fields (name and description)
	// Note: handle and parent are immutable and preserved from current resource
	updateResource := Resource{
		Name:        resource.Name,          // Mutable
		Handle:      currentResource.Handle, // Immutable - preserve
		Description: resource.Description,
		Parent:      currentResource.Parent, // Immutable - preserve
	}

	// Use transaction for write operation
	var updatedResource *Resource
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.UpdateResource(txCtx, id, resourceServerID, updateResource); err != nil {
			rs.logger.Error(ctx, "Failed to update resource", log.Error(err))
			return err
		}

		if err := rs.syncConsentOnPermissionUpdate(
			txCtx, currentResource.Permission, updateResource.Description,
		); err != nil {
			rs.logger.Error(ctx, "Failed to sync consent element for resource", log.Error(err))
			return err
		}

		updatedResource = &Resource{
			ID:          id,
			Name:        updateResource.Name,
			Handle:      updateResource.Handle,
			Description: updateResource.Description,
			Parent:      updateResource.Parent,
		}
		return nil
	}); err != nil {
		return nil, translateTxError(err)
	}

	return updatedResource, nil
}

// DeleteResource deletes a resource.
func (rs *resourceService) DeleteResource(
	ctx context.Context, resourceServerID, id string) *serviceerror.ServiceError {
	if id == "" || resourceServerID == "" {
		return &ErrorMissingID
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(resourceServerID) {
		rs.logger.Debug(ctx,
			"Cannot delete resource in declarative resource server",
			log.String("resource_server_id", resourceServerID),
		)
		return serviceerror.CustomServiceError(ErrorImmutableResource, core.I18nMessage{
			Key:          ErrorImmutableResource.ErrorDescription.Key,
			DefaultValue: fmt.Sprintf(ErrorImmutableResource.ErrorDescription.DefaultValue, id),
		})
	}

	// Validate resource server exists
	_, err := rs.resourceStore.GetResourceServer(ctx, resourceServerID)
	if err != nil {
		if errors.Is(err, errResourceServerNotFound) {
			return nil // Idempotent delete
		}
		rs.logger.Error(ctx, "Failed to check resource server", log.Error(err))
		return &serviceerror.InternalServerError
	}

	// Check resource exists
	currentResource, err := rs.resourceStore.GetResource(ctx, id, resourceServerID)
	if err != nil {
		if errors.Is(err, errResourceNotFound) {
			return nil // Idempotent delete
		}
		rs.logger.Error(ctx, "Failed to check resource existence", log.Error(err))
		return &serviceerror.InternalServerError
	}

	// Check for dependencies
	hasDeps, err := rs.resourceStore.CheckResourceHasDependencies(ctx, id)
	if err != nil {
		rs.logger.Error(ctx, "Failed to check dependencies", log.Error(err))
		return &serviceerror.InternalServerError
	}
	if hasDeps {
		return &ErrorCannotDelete
	}

	// Use transaction for write operation
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.DeleteResource(txCtx, id, resourceServerID); err != nil {
			rs.logger.Error(ctx, "Failed to delete resource", log.Error(err))
			return err
		}
		if err := rs.syncConsentOnPermissionDelete(
			txCtx, currentResource.Permission,
		); err != nil {
			rs.logger.Error(ctx, "Failed to sync consent element for resource delete", log.Error(err))
			return err
		}
		return nil
	}); err != nil {
		return translateTxError(err)
	}

	return nil
}

// Action Methods

// CreateAction creates an action.
// If resourceID is nil, creates action at resource server level.
// If resourceID is provided, creates action at resource level.
func (rs *resourceService) CreateAction(
	ctx context.Context,
	resourceServerID string, resourceID *string, action Action,
) (*Action, *serviceerror.ServiceError) {
	// Validate resource server exists
	resourceServer, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Validate resource if provided
	var resource *Resource
	if resourceID != nil {
		res, svcErr := rs.validateAndGetResourceByID(ctx, *resourceID, resourceServerID)
		if svcErr != nil {
			return nil, svcErr
		}
		resource = &res
	}

	if err := rs.validateActionCreate(action, resourceServer.Delimiter); err != nil {
		return nil, err
	}

	// Check handle uniqueness
	handleExists, err := rs.resourceStore.CheckActionHandleExists(
		ctx, resourceServerID, resourceID, action.Handle,
	)
	if err != nil {
		rs.logger.Error(ctx, "Failed to check action handle", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	if handleExists {
		return nil, &ErrorHandleConflict
	}

	// Derive permission string based on hierarchy
	action.Permission = derivePermission(resourceServer, resource, action.Handle)

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		rs.logger.Error(ctx, "Failed to generate UUID", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Use transaction for write operation
	var createdAction *Action
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.CreateAction(txCtx, id, resourceServerID, resourceID, action); err != nil {
			rs.logger.Error(ctx, "Failed to create action", log.Error(err))
			return err
		}

		if err := rs.syncConsentOnPermissionCreate(
			txCtx, action.Permission, action.Description,
		); err != nil {
			rs.logger.Error(ctx, "Failed to sync consent element for action", log.Error(err))
			return err
		}

		createdAction = &Action{
			ID:          id,
			Name:        action.Name,
			Handle:      action.Handle,
			Description: action.Description,
			Permission:  action.Permission,
		}
		return nil
	}); err != nil {
		return nil, translateTxError(err)
	}

	return createdAction, nil
}

// GetAction retrieves an action by ID.
// If resourceID is nil, retrieves action at resource server level.
// If resourceID is provided, retrieves action at resource level.
func (rs *resourceService) GetAction(
	ctx context.Context,
	resourceServerID string, resourceID *string, id string,
) (*Action, *serviceerror.ServiceError) {
	if id == "" || resourceServerID == "" {
		return nil, &ErrorMissingID
	}

	if resourceID != nil && *resourceID == "" {
		return nil, &ErrorMissingID
	}

	// Validate resource server exists
	_, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Validate resource if provided
	var resID *string
	if resourceID != nil {
		_, svcErr := rs.validateAndGetResourceByID(ctx, *resourceID, resourceServerID)
		if svcErr != nil {
			return nil, svcErr
		}
		resID = resourceID
	}

	action, err := rs.resourceStore.GetAction(ctx, id, resourceServerID, resID)
	if err != nil {
		if errors.Is(err, errActionNotFound) {
			return nil, &ErrorActionNotFound
		}
		rs.logger.Error(ctx, "Failed to get action", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	return &action, nil
}

// GetActionList retrieves a paginated list of actions.
// If resourceID is nil, retrieves actions at resource server level.
// If resourceID is provided, retrieves actions at resource level.
func (rs *resourceService) GetActionList(
	ctx context.Context,
	resourceServerID string, resourceID *string, limit, offset int,
) (*ActionList, *serviceerror.ServiceError) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	if resourceServerID == "" {
		return nil, &ErrorMissingID
	}

	if resourceID != nil && *resourceID == "" {
		return nil, &ErrorMissingID
	}

	// Validate resource server exists
	_, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Validate resource if provided
	var resID *string
	if resourceID != nil {
		_, svcErr := rs.validateAndGetResourceByID(ctx, *resourceID, resourceServerID)
		if svcErr != nil {
			return nil, svcErr
		}
		resID = resourceID
	}

	totalCount, err := rs.resourceStore.GetActionListCount(ctx, resourceServerID, resID)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to get action count", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	actions, err := rs.resourceStore.GetActionList(ctx, resourceServerID, resID, limit, offset)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to list actions", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Build base URL based on whether resource ID is provided
	var baseURL string
	if resourceID == nil {
		baseURL = fmt.Sprintf("/resource-servers/%s/actions", resourceServerID)
	} else {
		baseURL = fmt.Sprintf("/resource-servers/%s/resources/%s/actions", resourceServerID, *resourceID)
	}

	response := &ActionList{
		TotalResults: totalCount,
		Actions:      actions,
		StartIndex:   offset + 1,
		Count:        len(actions),
		Links:        buildPaginationLinks(baseURL, limit, offset, totalCount),
	}

	return response, nil
}

// UpdateAction updates an action.
// If resourceID is nil, updates action at resource server level.
// If resourceID is provided, updates action at resource level.
func (rs *resourceService) UpdateAction(
	ctx context.Context,
	resourceServerID string, resourceID *string, id string, action Action,
) (*Action, *serviceerror.ServiceError) {
	if id == "" || resourceServerID == "" {
		return nil, &ErrorMissingID
	}

	if resourceID != nil && *resourceID == "" {
		return nil, &ErrorMissingID
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(resourceServerID) {
		return nil, &ErrorImmutableAction
	}
	// Validate resource server exists
	_, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Validate resource if provided
	var resID *string
	if resourceID != nil {
		_, svcErr := rs.validateAndGetResourceByID(ctx, *resourceID, resourceServerID)
		if svcErr != nil {
			return nil, svcErr
		}
		resID = resourceID
	}

	// Get current action to preserve immutable fields
	currentAction, err := rs.resourceStore.GetAction(ctx, id, resourceServerID, resID)
	if err != nil {
		if errors.Is(err, errActionNotFound) {
			return nil, &ErrorActionNotFound
		}
		rs.logger.Error(ctx, "Failed to get action", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}

	// Update only name and description (handle is immutable)
	updateAction := Action{
		Name:        action.Name,
		Handle:      currentAction.Handle, // Immutable - preserve
		Description: action.Description,
	}

	// Use transaction for write operation
	var updatedAction *Action
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.UpdateAction(
			txCtx, id, resourceServerID, resID, updateAction,
		); err != nil {
			rs.logger.Error(ctx, "Failed to update action", log.Error(err))
			return err
		}

		if err := rs.syncConsentOnPermissionUpdate(
			txCtx, currentAction.Permission, updateAction.Description,
		); err != nil {
			rs.logger.Error(ctx, "Failed to sync consent element for action", log.Error(err))
			return err
		}

		updatedAction = &Action{
			ID:          id,
			Name:        updateAction.Name,
			Handle:      updateAction.Handle,
			Description: updateAction.Description,
		}
		return nil
	}); err != nil {
		return nil, translateTxError(err)
	}

	return updatedAction, nil
}

// DeleteAction deletes an action.
// If resourceID is nil, deletes action at resource server level.
// If resourceID is provided, deletes action at resource level.
func (rs *resourceService) DeleteAction(
	ctx context.Context,
	resourceServerID string, resourceID *string, id string,
) *serviceerror.ServiceError {
	if id == "" || resourceServerID == "" {
		return &ErrorMissingID
	}

	if resourceID != nil && *resourceID == "" {
		return &ErrorMissingID
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(resourceServerID) {
		rs.logger.Debug(ctx,
			"Cannot delete action in declarative resource server",
			log.String("resource_server_id", resourceServerID),
		)
		return serviceerror.CustomServiceError(ErrorImmutableAction, core.I18nMessage{
			Key:          ErrorImmutableAction.ErrorDescription.Key,
			DefaultValue: fmt.Sprintf(ErrorImmutableAction.ErrorDescription.DefaultValue, id),
		})
	}

	// Validate resource server exists
	_, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		if svcErr.Code == ErrorResourceServerNotFound.Code {
			return nil // Idempotent delete
		}
		return svcErr
	}

	// Validate resource if provided
	var resID *string
	if resourceID != nil {
		_, svcErr := rs.validateAndGetResourceByID(ctx, *resourceID, resourceServerID)
		if svcErr != nil {
			if svcErr.Code == ErrorResourceNotFound.Code {
				return nil // Idempotent delete
			}
			return svcErr
		}
		resID = resourceID
	}

	// Check if action exists
	exists, err := rs.resourceStore.IsActionExist(ctx, id, resourceServerID, resID)
	if err != nil {
		rs.logger.Error(ctx, "Failed to check action existence", log.Error(err))
		return &serviceerror.InternalServerError
	}
	if !exists {
		return nil // Idempotent delete
	}

	// Fetch the action so its permission string is available for the consent sync inside the
	// transaction. When the consent service is disabled the lookup is skipped.
	var permissionToSync string
	if rs.consentService != nil && rs.consentService.IsEnabled() {
		act, getErr := rs.resourceStore.GetAction(ctx, id, resourceServerID, resID)
		switch {
		case getErr == nil:
			permissionToSync = act.Permission
		case errors.Is(getErr, errActionNotFound):
			// Concurrent delete — nothing to sync.
		default:
			// Any other failure must abort: deleting without syncing would leave the consent
			// element orphaned.
			rs.logger.Error(ctx, "Failed to load action for consent sync", log.Error(getErr))
			return &serviceerror.InternalServerError
		}
	}

	// Use transaction for write operation
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.DeleteAction(txCtx, id, resourceServerID, resID); err != nil {
			rs.logger.Error(ctx, "Failed to delete action", log.Error(err))
			return err
		}
		if permissionToSync != "" {
			if err := rs.syncConsentOnPermissionDelete(txCtx, permissionToSync); err != nil {
				rs.logger.Error(ctx, "Failed to sync consent element for action delete", log.Error(err))
				return err
			}
		}
		return nil
	}); err != nil {
		return translateTxError(err)
	}

	return nil
}

// ValidatePermissions checks if permissions exist for a given resource server.
// Returns array of invalid permissions (empty if all valid).
func (rs *resourceService) ValidatePermissions(
	ctx context.Context,
	resourceServerID string,
	permissions []string,
) ([]string, *serviceerror.ServiceError) {
	rs.logger.Debug(ctx, "Validating permissions",
		log.String("resourceServerId", resourceServerID),
		log.Int("permissionCount", len(permissions)))

	if len(permissions) == 0 {
		return []string{}, nil
	}

	// Validate resource server exists
	_, err := rs.resourceStore.GetResourceServer(ctx, resourceServerID)
	if err != nil {
		if !errors.Is(err, errResourceServerNotFound) {
			rs.logger.Error(ctx, "Failed to validate resource server existence",
				log.String("resourceServerId", resourceServerID),
				log.Error(err))
			return nil, &serviceerror.InternalServerError
		}
		rs.logger.Debug(ctx, "Resource server not found",
			log.String("resourceServerId", resourceServerID))
		// Return all permissions as invalid if resource server doesn't exist
		return permissions, nil
	}

	// Call store to validate permissions
	invalidPermissions, storeErr := rs.resourceStore.ValidatePermissions(ctx, resourceServerID, permissions)
	if storeErr != nil {
		rs.logger.Error(ctx, "Failed to validate permissions in store",
			log.String("resourceServerId", resourceServerID),
			log.Error(storeErr))
		return nil, &serviceerror.InternalServerError
	}

	return invalidPermissions, nil
}

// FindResourceServersByPermissions returns registered resource servers that define at least one
// permission in the supplied set.
func (rs *resourceService) FindResourceServersByPermissions(
	ctx context.Context,
	permissions []string,
) ([]ResourceServer, *serviceerror.ServiceError) {
	if len(permissions) == 0 {
		return []ResourceServer{}, nil
	}

	resourceServers, err := rs.resourceStore.FindResourceServersByPermissions(ctx, permissions)
	if err != nil {
		rs.logger.Error(ctx, "Failed to find resource servers by permissions", log.Error(err))
		return nil, &serviceerror.InternalServerError
	}
	return resourceServers, nil
}

// ResolveResourceServerOUHandle resolves ou_handle to an OU ID on the given resource server
// in-place. Called by the declarative loader validator so that file-based resource servers
// support ou_handle. If both ou_id and ou_handle are provided, ou_id wins and a warning is logged.
func (rs *resourceService) ResolveResourceServerOUHandle(
	ctx context.Context, server *ResourceServer,
) *serviceerror.ServiceError {
	if server.OUID != "" && server.OUHandle != "" {
		rs.logger.Warn(ctx, "Both ou_id and ou_handle provided for resource server; ou_handle ignored",
			log.String("resourceServerID", server.ID), log.String("name", server.Name))
		return nil
	}
	if server.OUID == "" && server.OUHandle != "" {
		if rs.ouService == nil {
			return &serviceerror.InternalServerError
		}
		ou, svcErr := rs.ouService.GetOrganizationUnitByPath(
			security.WithRuntimeContext(ctx), server.OUHandle)
		if svcErr != nil {
			return &ErrorInvalidRequestFormat
		}
		server.OUID = ou.ID
	}
	return nil
}

// Validation helper methods

// validateAndGetResourceServer validates resource server exists and returns it.
func (rs *resourceService) validateAndGetResourceServer(
	ctx context.Context,
	resourceServerID string,
) (ResourceServer, *serviceerror.ServiceError) {
	resourceServer, err := rs.resourceStore.GetResourceServer(ctx, resourceServerID)
	if err != nil {
		if errors.Is(err, errResourceServerNotFound) {
			return ResourceServer{}, &ErrorResourceServerNotFound
		}
		rs.logger.Error(ctx, "Failed to check resource server", log.Error(err))
		return ResourceServer{}, &serviceerror.InternalServerError
	}
	return resourceServer, nil
}

// validateAndGetResourceByID validates resource exists and returns it.
func (rs *resourceService) validateAndGetResourceByID(
	ctx context.Context,
	resourceID string,
	resourceServerID string,
) (Resource, *serviceerror.ServiceError) {
	resource, err := rs.resourceStore.GetResource(ctx, resourceID, resourceServerID)
	if err != nil {
		if errors.Is(err, errResourceNotFound) {
			return Resource{}, &ErrorResourceNotFound
		}
		rs.logger.Error(ctx, "Failed to check resource", log.Error(err))
		return Resource{}, &serviceerror.InternalServerError
	}
	return resource, nil
}

// validateResourceServerCreate validates the input for creating a resource server.
func (rs *resourceService) validateResourceServerCreate(resourceServer ResourceServer) *serviceerror.ServiceError {
	if resourceServer.Name == "" {
		return &ErrorInvalidRequestFormat
	}
	if resourceServer.OUID == "" {
		return &ErrorInvalidRequestFormat
	}
	if resourceServer.Type != "" && !resourceServer.Type.IsValid() {
		return &ErrorInvalidRequestFormat
	}
	if resourceServer.Delimiter != "" {
		if err := validateDelimiter(resourceServer.Delimiter); err != nil {
			return err
		}
	}
	return nil
}

// validateResourceServerUpdate validates the input for updating a resource server.
func (rs *resourceService) validateResourceServerUpdate(resourceServer ResourceServer) *serviceerror.ServiceError {
	if resourceServer.Name == "" {
		return &ErrorInvalidRequestFormat
	}
	if resourceServer.OUID == "" {
		return &ErrorInvalidRequestFormat
	}
	return nil
}

// validateResourceCreate validates the input for creating a resource.
func (rs *resourceService) validateResourceCreate(resource Resource, delimiter string) *serviceerror.ServiceError {
	if resource.Name == "" {
		return &ErrorInvalidRequestFormat
	}
	if resource.Handle == "" {
		return &ErrorInvalidRequestFormat
	}
	// Validate handle
	if err := validateHandle(resource.Handle, delimiter); err != nil {
		return err
	}
	return nil
}

// validateActionCreate validates the input for creating an action.
func (rs *resourceService) validateActionCreate(action Action, delimiter string) *serviceerror.ServiceError {
	if action.Name == "" {
		return &ErrorInvalidRequestFormat
	}
	if action.Handle == "" {
		return &ErrorInvalidRequestFormat
	}
	// Validate handle
	if err := validateHandle(action.Handle, delimiter); err != nil {
		return err
	}
	return nil
}

// validatePaginationParams validates pagination parameters.
func validatePaginationParams(limit, offset int) *serviceerror.ServiceError {
	if limit < 1 || limit > serverconst.MaxPageSize {
		return &ErrorInvalidLimit
	}
	if offset < 0 {
		return &ErrorInvalidOffset
	}

	return nil
}

// buildPaginationLinks constructs pagination links for a paginated response.
func buildPaginationLinks(base string, limit, offset, totalCount int) []Link {
	links := make([]Link, 0)

	if offset > 0 {
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=0&limit=%d", base, limit),
			Rel:  "first",
		})

		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=%d&limit=%d", base, prevOffset, limit),
			Rel:  "prev",
		})
	}

	if offset+limit < totalCount {
		nextOffset := offset + limit
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=%d&limit=%d", base, nextOffset, limit),
			Rel:  "next",
		})
	}

	lastPageOffset := ((totalCount - 1) / limit) * limit
	if offset < lastPageOffset {
		links = append(links, Link{
			Href: fmt.Sprintf("%s?offset=%d&limit=%d", base, lastPageOffset, limit),
			Rel:  "last",
		})
	}

	return links
}

// isValidPermissionCharacter checks if a character is valid for permission strings.
// Allowed characters: a-z A-Z 0-9 . _ : - /
func isValidPermissionCharacter(c rune) bool {
	return strings.ContainsRune(validPermissionCharacters, c)
}

// validateDelimiter validates delimiter is a single valid delimiter character.
func validateDelimiter(delimiter string) *serviceerror.ServiceError {
	if len(delimiter) != 1 {
		return &ErrorInvalidDelimiter
	}
	if !strings.ContainsRune(ValidPermissionDelimiters, rune(delimiter[0])) {
		return &ErrorInvalidDelimiter
	}
	return nil
}

// validateHandle validates a handle string.
func validateHandle(handle string, delimiter string) *serviceerror.ServiceError {
	if len(handle) > 100 {
		return &ErrorInvalidHandle
	}
	for _, c := range handle {
		if !isValidPermissionCharacter(c) {
			return &ErrorInvalidHandle
		}
		if string(c) == delimiter {
			return &ErrorDelimiterInHandle
		}
	}
	return nil
}

// getDefaultDelimiter returns the default delimiter from configuration.
func getDefaultDelimiter() string {
	delimiter := config.GetServerRuntime().Config.Resource.DefaultDelimiter
	if delimiter == "" {
		return ":" // Fallback default if not configured
	}
	return delimiter
}

// derivePermission builds permission string for a resource based on parent hierarchy.
func derivePermission(
	resourceServer ResourceServer,
	parentResource *Resource,
	handle string,
) string {
	if parentResource != nil {
		return parentResource.Permission + resourceServer.Delimiter + handle
	}
	if resourceServer.Handle != "" {
		return resourceServer.Handle + resourceServer.Delimiter + handle
	}
	return handle
}

// syncConsentOnPermissionCreate creates a consent element for the given permission string.
// Idempotent: existing elements with the same name are left untouched.
//
// This mirrors the attribute-consent sync model used by the inbound-client service
// (ValidateConsentElements followed by a batch CreateConsentElements for the missing ones), so that
// resource CRUD operations participate in the same transactional consent lifecycle as the
// neighboring services. Callers must run this inside the resource CRUD transaction; a failure
// rolls the transaction back.
func (rs *resourceService) syncConsentOnPermissionCreate(
	ctx context.Context, permission, description string,
) error {
	if rs.consentService == nil || !rs.consentService.IsEnabled() || permission == "" {
		return nil
	}
	// TODO: Replace with the resource server's actual OU when multi-OU consent is supported.
	const ouID = "default"

	validNames, err := rs.consentService.ValidateConsentElements(ctx, ouID, []string{permission})
	if err != nil {
		return rs.wrapConsentServiceError(ctx, err)
	}
	for _, n := range validNames {
		if n == permission {
			return nil
		}
	}

	if _, createErr := rs.consentService.CreateConsentElements(ctx, ouID, []consent.ConsentElementInput{{
		Name:        permission,
		Description: description,
		Namespace:   consent.NamespacePermission,
	}}); createErr != nil {
		return rs.wrapConsentServiceError(ctx, createErr)
	}
	return nil
}

// syncConsentOnPermissionDelete removes the consent element associated with the given permission
// string. Idempotent: a missing element is treated as success. An element still associated with a
// consent purpose cannot be deleted; that case is treated as success since the permission may still
// be referenced by an existing consent record.
func (rs *resourceService) syncConsentOnPermissionDelete(ctx context.Context, permission string) error {
	if rs.consentService == nil || !rs.consentService.IsEnabled() || permission == "" {
		return nil
	}
	// TODO: Replace with the resource server's actual OU when multi-OU consent is supported.
	const ouID = "default"

	existing, err := rs.consentService.ListConsentElements(ctx, ouID, consent.NamespacePermission, permission)
	if err != nil {
		return rs.wrapConsentServiceError(ctx, err)
	}
	if len(existing) == 0 {
		return nil
	}

	// Permission strings are unique within an OU, so at most one element is expected.
	if delErr := rs.consentService.DeleteConsentElement(ctx, ouID, existing[0].ID); delErr != nil {
		if delErr.Code == consent.ErrorDeletingConsentElementWithAssociatedPurpose.Code {
			return nil
		}
		return rs.wrapConsentServiceError(ctx, delErr)
	}
	return nil
}

// syncConsentOnPermissionUpdate refreshes the description of the consent element associated with
// the given permission string. When the element is missing it is created lazily so callers do not
// have to coordinate creates and updates.
func (rs *resourceService) syncConsentOnPermissionUpdate(
	ctx context.Context, permission, description string,
) error {
	if rs.consentService == nil || !rs.consentService.IsEnabled() || permission == "" {
		return nil
	}
	// TODO: Replace with the resource server's actual OU when multi-OU consent is supported.
	const ouID = "default"

	existing, err := rs.consentService.ListConsentElements(ctx, ouID, consent.NamespacePermission, permission)
	if err != nil {
		return rs.wrapConsentServiceError(ctx, err)
	}
	if len(existing) == 0 {
		return rs.syncConsentOnPermissionCreate(ctx, permission, description)
	}
	if existing[0].Description == description {
		return nil
	}

	if _, updErr := rs.consentService.UpdateConsentElement(ctx, ouID, existing[0].ID,
		&consent.ConsentElementInput{
			Name:        permission,
			Description: description,
			Namespace:   consent.NamespacePermission,
		}); updErr != nil {
		return rs.wrapConsentServiceError(ctx, updErr)
	}
	return nil
}

// wrapConsentServiceError wraps a consent service error in a consentSyncError so that callers can
// distinguish consent-service failures from other store or service errors during resource CRUD.
// Server-class failures are logged here so operators get a record even when the transaction
// closure collapses the error to InternalServerError on the way out.
func (rs *resourceService) wrapConsentServiceError(ctx context.Context, err *serviceerror.ServiceError) error {
	if err == nil {
		return nil
	}
	if err.Type == serviceerror.ServerErrorType {
		rs.logger.Error(ctx, "Consent service returned a server-class error during resource sync",
			log.String("code", err.Code),
			log.String("description", err.ErrorDescription.DefaultValue))
	}
	return &consentSyncError{Underlying: err}
}

// translateTxError converts a transaction-closure error into the resource service's
// *serviceerror.ServiceError API surface. A typed *consentSyncError is mapped to
// ErrorConsentSyncFailed for client-class consent failures (preserving the underlying code in the
// description) and to InternalServerError otherwise. All other transaction errors collapse to
// InternalServerError. This mirrors the inboundclient + agent/application translation pattern.
func translateTxError(err error) *serviceerror.ServiceError {
	var consentErr *consentSyncError
	if errors.As(err, &consentErr) {
		if consentErr.IsClientError() {
			return serviceerror.CustomServiceError(ErrorConsentSyncFailed, core.I18nMessage{
				Key: "error.resourceservice.consent_sync_failed_description",
				DefaultValue: fmt.Sprintf(
					ErrorConsentSyncFailed.ErrorDescription.DefaultValue+" : code - %s",
					consentErr.Underlying.Code,
				),
			})
		}
		return &serviceerror.InternalServerError
	}
	return &serviceerror.InternalServerError
}

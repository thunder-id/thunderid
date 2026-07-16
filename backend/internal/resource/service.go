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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/consent"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
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
	CreateResourceServer(
		ctx context.Context,
		rs providers.ResourceServer,
	) (*providers.ResourceServer, *tidcommon.ServiceError)
	GetResourceServer(ctx context.Context, id string) (*providers.ResourceServer, *tidcommon.ServiceError)
	GetResourceServerList(ctx context.Context, limit, offset int) (*ResourceServerList, *tidcommon.ServiceError)
	UpdateResourceServer(
		ctx context.Context, id string, rs providers.ResourceServer,
	) (*providers.ResourceServer, *tidcommon.ServiceError)
	DeleteResourceServer(ctx context.Context, id string) *tidcommon.ServiceError
	GetResourceServerByIdentifier(
		ctx context.Context, identifier string,
	) (*providers.ResourceServer, *tidcommon.ServiceError)
	IsResourceServerDeclarative(id string) bool

	// Resource operations
	CreateResource(ctx context.Context, resourceServerID string, res providers.Resource) (
		*providers.Resource, *tidcommon.ServiceError)
	GetResource(ctx context.Context, resourceServerID, id string) (*providers.Resource, *tidcommon.ServiceError)
	GetResourceList(
		ctx context.Context, resourceServerID string, parentID *string, limit, offset int,
	) (*ResourceList, *tidcommon.ServiceError)
	GetAllResourceList(
		ctx context.Context, resourceServerID string,
	) ([]providers.Resource, *tidcommon.ServiceError)
	UpdateResource(
		ctx context.Context, resourceServerID, id string, res providers.Resource,
	) (*providers.Resource, *tidcommon.ServiceError)
	DeleteResource(ctx context.Context, resourceServerID, id string) *tidcommon.ServiceError

	// Action operations
	CreateAction(
		ctx context.Context, resourceServerID string, resourceID *string, action providers.Action,
	) (*providers.Action, *tidcommon.ServiceError)
	GetAction(
		ctx context.Context, resourceServerID string, resourceID *string, id string,
	) (*providers.Action, *tidcommon.ServiceError)
	GetActionList(
		ctx context.Context, resourceServerID string, resourceID *string, kind providers.ActionKind, limit, offset int,
	) (*ActionList, *tidcommon.ServiceError)
	UpdateAction(
		ctx context.Context, resourceServerID string, resourceID *string, id string, action providers.Action,
	) (*providers.Action, *tidcommon.ServiceError)
	DeleteAction(ctx context.Context, resourceServerID string, resourceID *string,
		id string) *tidcommon.ServiceError
	ValidatePermissions(
		ctx context.Context, resourceServerID string, permissions []string,
	) ([]string, *tidcommon.ServiceError)

	// FindResourceServersByPermissions returns registered resource servers that define at least
	// one permission in the supplied set. Used by the OAuth2 token layer to populate aud when no
	// explicit resource parameter was supplied.
	FindResourceServersByPermissions(
		ctx context.Context, permissions []string,
	) ([]providers.ResourceServer, *tidcommon.ServiceError)

	// ResolveResourceServerOUHandle resolves ou_handle to an OU ID on the given resource server
	// in-place. Called by the declarative loader validator so that file-based resource servers
	// support ou_handle.
	ResolveResourceServerOUHandle(
		ctx context.Context, rs *providers.ResourceServer,
	) *tidcommon.ServiceError

	SetDependencyRegistry(r resourcedependency.Registry)
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
}

// resourceService is the default implementation of ResourceServiceInterface.
type resourceService struct {
	logger             log.Logger
	resourceStore      resourceStoreInterface
	ouService          oupkg.OrganizationUnitServiceInterface
	consentService     consent.ConsentServiceInterface
	defaultDelimiter   string
	transactioner      transaction.Transactioner
	dependencyRegistry resourcedependency.Registry
}

// SetDependencyRegistry injects the dependency registry. Called by servicemanager after the
// provider services are initialized to avoid a cyclic import.
func (rs *resourceService) SetDependencyRegistry(r resourcedependency.Registry) {
	rs.dependencyRegistry = r
}

// ensureNoBlockingDependencies refuses deletion when other resources depend on the target
// (behaviorOnDelete == restrict). Because deletion is destructive, it fails closed: if dependency
// data cannot be determined, the deletion is refused rather than allowed.
func (rs *resourceService) ensureNoBlockingDependencies(
	ctx context.Context, resourceType, id string,
) *tidcommon.ServiceError {
	if rs.dependencyRegistry == nil {
		rs.logger.Error(ctx, "Dependency registry not set; refusing to delete",
			log.String("resourceType", resourceType), log.String("id", id))
		return &tidcommon.InternalServerError
	}

	deps, err := rs.dependencyRegistry.GetDependencies(ctx, resourceType, id)
	if err != nil {
		rs.logger.Error(ctx, "Failed to evaluate dependencies",
			log.String("resourceType", resourceType), log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	// Fail closed: nil TotalResults means a provider failed to report, so usage is unknown.
	if deps == nil || deps.TotalResults == nil {
		rs.logger.Error(ctx, "Dependency data unavailable; refusing to delete",
			log.String("resourceType", resourceType), log.String("id", id))
		return &tidcommon.InternalServerError
	}

	if len(resourcedependency.BlockingUsages(deps)) == 0 {
		return nil
	}

	return &ErrorCannotDelete
}

// GetResourceDependencies implements resourcedependency.Provider. A resource server is blocked from
// deletion while it still has resources or resource-server-level actions; a resource is blocked while
// it still has sub-resources or actions. These dependents live in the same store, so their existence
// is resolved directly and reported as a single restrict usage. Other resource types have no
// dependencies here.
func (rs *resourceService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	var hasDeps bool
	var err error
	switch resourceType {
	case resourcedependency.ResourceTypeResourceServer:
		hasDeps, err = rs.resourceStore.CheckResourceServerHasDependencies(ctx, id)
	case resourcedependency.ResourceTypeResource:
		hasDeps, err = rs.resourceStore.CheckResourceHasDependencies(ctx, id)
	default:
		return []resourcedependency.ResourceDependency{}, nil
	}
	if err != nil {
		return nil, err
	}
	if !hasDeps {
		return []resourcedependency.ResourceDependency{}, nil
	}

	return []resourcedependency.ResourceDependency{{
		ResourceType:     resourcedependency.ResourceTypeResource,
		ID:               id,
		BehaviorOnDelete: resourcedependency.BehaviorRestrict,
	}}, nil
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
	resourceServer providers.ResourceServer,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}

	// Check name uniqueness
	nameExists, err := rs.resourceStore.CheckResourceServerNameExists(ctx, resourceServer.Name)
	if err != nil {
		rs.logger.Error(ctx, "Failed to check resource server name", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if nameExists {
		rs.logger.Debug(ctx, "Resource server name already exists", log.String("name", resourceServer.Name))
		return nil, &ErrorNameConflict
	}

	// Check identifier uniqueness
	identifierExists, err := rs.resourceStore.CheckResourceServerIdentifierExists(ctx, resourceServer.Identifier)
	if err != nil {
		rs.logger.Error(ctx, "Failed to check resource server identifier", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if identifierExists {
		rs.logger.Debug(ctx, "Resource server identifier already exists",
			log.String("identifier", resourceServer.Identifier))
		return nil, &ErrorIdentifierConflict
	}

	// Set default type if not provided
	if resourceServer.Type == "" {
		resourceServer.Type = providers.ResourceServerTypeCustom
	}

	// Set default delimiter if not provided
	if resourceServer.Delimiter == "" {
		resourceServer.Delimiter = rs.defaultDelimiter
	}

	id := resourceServer.ID
	if id == "" {
		var err error
		id, err = utils.GenerateUUIDv7()
		if err != nil {
			rs.logger.Error(ctx, "Failed to generate UUID", log.Error(err))
			return nil, &tidcommon.InternalServerError
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
	var createdRS *providers.ResourceServer
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.CreateResourceServer(txCtx, id, resourceServer); err != nil {
			rs.logger.Error(ctx, "Failed to create resource server", log.Error(err))
			return err
		}

		createdRS = &providers.ResourceServer{
			ID:          id,
			Name:        resourceServer.Name,
			Description: resourceServer.Description,
			Identifier:  resourceServer.Identifier,
			Type:        resourceServer.Type,
			OUID:        resourceServer.OUID,
			Delimiter:   resourceServer.Delimiter,
		}
		return nil
	}); err != nil {
		return nil, &tidcommon.InternalServerError
	}

	rs.logger.Debug(ctx, "Successfully created resource server", log.String("id", id))
	return createdRS, nil
}

// GetResourceServer retrieves a resource server by ID.
func (rs *resourceService) GetResourceServer(
	ctx context.Context, id string,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}

	return &resourceServer, nil
}

// GetResourceServerByIdentifier retrieves a resource server by its identifier.
func (rs *resourceService) GetResourceServerByIdentifier(
	ctx context.Context, identifier string,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}

	return &resourceServer, nil
}

// GetResourceServerList retrieves a paginated list of resource servers.
func (rs *resourceService) GetResourceServerList(
	ctx context.Context, limit, offset int,
) (*ResourceServerList, *tidcommon.ServiceError) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	totalCount, err := rs.resourceStore.GetResourceServerListCount(ctx)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to get resource server count", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	resourceServers, err := rs.resourceStore.GetResourceServerList(ctx, limit, offset)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to list resource servers", log.Error(err))
		return nil, &tidcommon.InternalServerError
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
	id string, resourceServer providers.ResourceServer,
) (*providers.ResourceServer, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(id) {
		rs.logger.Debug(ctx, "Cannot modify declarative resource server", log.String("id", id))
		return nil, ErrorImmutableResourceServer.WithParams(map[string]string{"id": id})
	}

	// Delimiter is always preserved from the existing record
	resourceServer.Delimiter = existingResServer.Delimiter

	// Type is immutable and always preserved from the existing record
	resourceServer.Type = existingResServer.Type

	// Identifier: preserve existing if not provided; check uniqueness if changed
	if resourceServer.Identifier == "" {
		resourceServer.Identifier = existingResServer.Identifier
	} else if resourceServer.Identifier != existingResServer.Identifier {
		identifierExists, err := rs.resourceStore.CheckResourceServerIdentifierExists(ctx, resourceServer.Identifier)
		if err != nil {
			rs.logger.Error(ctx, "Failed to check resource server identifier", log.Error(err))
			return nil, &tidcommon.InternalServerError
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
		return nil, &tidcommon.InternalServerError
	}

	// Check name uniqueness, if changed
	if existingResServer.Name != resourceServer.Name {
		nameExists, err := rs.resourceStore.CheckResourceServerNameExists(ctx, resourceServer.Name)
		if err != nil {
			rs.logger.Error(ctx, "Failed to check resource server name", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if nameExists {
			return nil, &ErrorNameConflict
		}
	}

	var updatedRS *providers.ResourceServer
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.UpdateResourceServer(txCtx, id, resourceServer); err != nil {
			rs.logger.Error(ctx, "Failed to update resource server", log.Error(err))
			return err
		}

		updatedRS = &providers.ResourceServer{
			ID:          id,
			Name:        resourceServer.Name,
			Description: resourceServer.Description,
			Identifier:  resourceServer.Identifier,
			Type:        resourceServer.Type,
			OUID:        resourceServer.OUID,
			Delimiter:   resourceServer.Delimiter,
		}
		return nil
	}); err != nil {
		return nil, &tidcommon.InternalServerError
	}

	return updatedRS, nil
}

// DeleteResourceServer deletes a resource server.
func (rs *resourceService) DeleteResourceServer(ctx context.Context, id string) *tidcommon.ServiceError {
	if id == "" {
		return &ErrorMissingID
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(id) {
		rs.logger.Debug(ctx, "Cannot delete declarative resource server", log.String("id", id))
		return ErrorImmutableResourceServer.WithParams(map[string]string{"id": id})
	}

	_, err := rs.resourceStore.GetResourceServer(ctx, id)
	if err != nil {
		if errors.Is(err, errResourceServerNotFound) {
			return nil // Idempotent delete
		}
		rs.logger.Error(ctx, "Failed to check resource server existence", log.Error(err))
		return &tidcommon.InternalServerError
	}

	// Refuse deletion when resources or actions still depend on this resource server. Dependencies
	// are aggregated through the dependency registry.
	if svcErr := rs.ensureNoBlockingDependencies(
		ctx, resourcedependency.ResourceTypeResourceServer, id); svcErr != nil {
		return svcErr
	}

	// Use transaction for write operation
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.DeleteResourceServer(txCtx, id); err != nil {
			rs.logger.Error(ctx, "Failed to delete resource server", log.Error(err))
			return err
		}
		return nil
	}); err != nil {
		return &tidcommon.InternalServerError
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
	resourceServerID string, resource providers.Resource,
) (*providers.Resource, *tidcommon.ServiceError) {
	// Validate resource server exists
	resourceServer, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	if err := rs.validateResourceCreate(resource, resourceServer.Delimiter); err != nil {
		return nil, err
	}

	// Validate parent if specified
	var parentResource *providers.Resource
	if resource.Parent != nil {
		res, err := rs.resourceStore.GetResource(ctx, *resource.Parent, resourceServerID)
		if err != nil {
			if errors.Is(err, errResourceNotFound) {
				return nil, &ErrorParentResourceNotFound
			}
			rs.logger.Error(ctx, "Failed to check parent resource", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		parentResource = &res
	}

	// Check handle uniqueness under parent
	handleExists, err := rs.resourceStore.CheckResourceHandleExists(
		ctx, resourceServerID, resource.Handle, resource.Parent,
	)
	if err != nil {
		rs.logger.Error(ctx, "Failed to check resource handle", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if handleExists {
		return nil, &ErrorHandleConflict
	}

	// For MCP resource servers, a resource (group) and an action (tool/resource) in the same parent
	// context must not share a handle, since they would derive an identical permission string.
	if resourceServer.Type == providers.ResourceServerTypeMCP {
		actionHandleExists, err := rs.resourceStore.CheckActionHandleExists(
			ctx, resourceServerID, resource.Parent, resource.Handle,
		)
		if err != nil {
			rs.logger.Error(ctx, "Failed to check action handle", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if actionHandleExists {
			return nil, &ErrorHandleConflict
		}
	}

	// Derive permission string based on hierarchy
	resource.Permission = derivePermission(resourceServer, parentResource, resource.Handle)

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		rs.logger.Error(ctx, "Failed to generate UUID", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	// Use transaction for write operation
	var createdResource *providers.Resource
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

		createdResource = &providers.Resource{
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
) (*providers.Resource, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}

	return &resource, nil
}

// GetResourceList retrieves a paginated list of resources.
func (rs *resourceService) GetResourceList(
	ctx context.Context,
	resourceServerID string, parentID *string, limit, offset int,
) (*ResourceList, *tidcommon.ServiceError) {
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
	var resources []providers.Resource

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
		return nil, &tidcommon.InternalServerError
	}

	resources, err = rs.resourceStore.GetResourceListByParent(ctx, resourceServerID, parentID, limit, offset)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to list resources", log.Error(err))
		return nil, &tidcommon.InternalServerError
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
) ([]providers.Resource, *tidcommon.ServiceError) {
	if resourceServerID == "" {
		return nil, &ErrorMissingID
	}
	if _, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID); svcErr != nil {
		return nil, svcErr
	}

	totalCount, err := rs.resourceStore.GetResourceListCount(ctx, resourceServerID)
	if err != nil {
		rs.logger.Error(ctx, "Failed to get resource count", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if totalCount == 0 {
		return []providers.Resource{}, nil
	}

	resources, err := rs.resourceStore.GetResourceList(ctx, resourceServerID, totalCount, 0)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to list all resources", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return resources, nil
}

// UpdateResource updates a resource.
func (rs *resourceService) UpdateResource(
	ctx context.Context,
	resourceServerID, id string, resource providers.Resource,
) (*providers.Resource, *tidcommon.ServiceError) {
	if id == "" || resourceServerID == "" {
		return nil, &ErrorMissingID
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(resourceServerID) {
		rs.logger.Debug(ctx,
			"Cannot modify resource in declarative resource server",
			log.String("resource_server_id", resourceServerID),
		)
		return nil, ErrorImmutableResource.WithParams(map[string]string{"id": id})
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
		return nil, &tidcommon.InternalServerError
	}

	// Update only mutable fields (name and description)
	// Note: handle and parent are immutable and preserved from current resource
	updateResource := providers.Resource{
		Name:        resource.Name,          // Mutable
		Handle:      currentResource.Handle, // Immutable - preserve
		Description: resource.Description,
		Parent:      currentResource.Parent, // Immutable - preserve
	}

	// Use transaction for write operation
	var updatedResource *providers.Resource
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.UpdateResource(txCtx, id, resourceServerID, updateResource); err != nil {
			rs.logger.Error(ctx, "Failed to update resource", log.Error(err))
			return err
		}

		updatedResource = &providers.Resource{
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
	ctx context.Context, resourceServerID, id string) *tidcommon.ServiceError {
	if id == "" || resourceServerID == "" {
		return &ErrorMissingID
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(resourceServerID) {
		rs.logger.Debug(ctx,
			"Cannot delete resource in declarative resource server",
			log.String("resource_server_id", resourceServerID),
		)
		return ErrorImmutableResource.WithParams(map[string]string{"id": id})
	}

	// Validate resource server exists
	_, err := rs.resourceStore.GetResourceServer(ctx, resourceServerID)
	if err != nil {
		if errors.Is(err, errResourceServerNotFound) {
			return nil // Idempotent delete
		}
		rs.logger.Error(ctx, "Failed to check resource server", log.Error(err))
		return &tidcommon.InternalServerError
	}

	// Check resource exists
	currentResource, err := rs.resourceStore.GetResource(ctx, id, resourceServerID)
	if err != nil {
		if errors.Is(err, errResourceNotFound) {
			return nil // Idempotent delete
		}
		rs.logger.Error(ctx, "Failed to check resource existence", log.Error(err))
		return &tidcommon.InternalServerError
	}

	// Refuse deletion when sub-resources or actions still depend on this resource. Dependencies are
	// aggregated through the dependency registry.
	if svcErr := rs.ensureNoBlockingDependencies(ctx, resourcedependency.ResourceTypeResource, id); svcErr != nil {
		return svcErr
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

// providers.Action Methods

// CreateAction creates an action.
// If resourceID is nil, creates action at resource server level.
// If resourceID is provided, creates action at resource level.
func (rs *resourceService) CreateAction(
	ctx context.Context,
	resourceServerID string, resourceID *string, action providers.Action,
) (*providers.Action, *tidcommon.ServiceError) {
	// Validate resource server exists
	resourceServer, svcErr := rs.validateAndGetResourceServer(ctx, resourceServerID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Validate resource if provided
	var resource *providers.Resource
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

	if resourceServer.Type == providers.ResourceServerTypeMCP && action.Kind == "" {
		action.Kind = providers.ActionKindTool
	}
	if svcErr := rs.validateActionKind(action.Kind); svcErr != nil {
		return nil, svcErr
	}

	// Check handle uniqueness
	handleExists, err := rs.resourceStore.CheckActionHandleExists(
		ctx, resourceServerID, resourceID, action.Handle,
	)
	if err != nil {
		rs.logger.Error(ctx, "Failed to check action handle", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if handleExists {
		return nil, &ErrorHandleConflict
	}

	// For MCP resource servers, an action (tool/resource) and a resource (group) in the same parent
	// context must not share a handle, since they would derive an identical permission string.
	if resourceServer.Type == providers.ResourceServerTypeMCP {
		resHandleExists, err := rs.resourceStore.CheckResourceHandleExists(
			ctx, resourceServerID, action.Handle, resourceID,
		)
		if err != nil {
			rs.logger.Error(ctx, "Failed to check resource handle", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if resHandleExists {
			return nil, &ErrorHandleConflict
		}
	}

	// Derive permission string based on hierarchy
	action.Permission = derivePermission(resourceServer, resource, action.Handle)

	id, err := utils.GenerateUUIDv7()
	if err != nil {
		rs.logger.Error(ctx, "Failed to generate UUID", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	// Use transaction for write operation
	var createdAction *providers.Action
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

		createdAction = &providers.Action{
			ID:          id,
			Name:        action.Name,
			Handle:      action.Handle,
			Description: action.Description,
			Permission:  action.Permission,
			Kind:        action.Kind,
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
) (*providers.Action, *tidcommon.ServiceError) {
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
		return nil, &tidcommon.InternalServerError
	}
	return &action, nil
}

// GetActionList retrieves a paginated list of actions.
// If resourceID is nil, retrieves actions at resource server level.
// If resourceID is provided, retrieves actions at resource level.
// If kind is non-empty, only actions of that kind are returned.
func (rs *resourceService) GetActionList(
	ctx context.Context,
	resourceServerID string, resourceID *string, kind providers.ActionKind, limit, offset int,
) (*ActionList, *tidcommon.ServiceError) {
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

	totalCount, err := rs.resourceStore.GetActionListCount(ctx, resourceServerID, resID, kind)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to get action count", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	actions, err := rs.resourceStore.GetActionList(ctx, resourceServerID, resID, kind, limit, offset)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ErrResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to list actions", log.Error(err))
		return nil, &tidcommon.InternalServerError
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
	resourceServerID string, resourceID *string, id string, action providers.Action,
) (*providers.Action, *tidcommon.ServiceError) {
	if id == "" || resourceServerID == "" {
		return nil, &ErrorMissingID
	}

	if resourceID != nil && *resourceID == "" {
		return nil, &ErrorMissingID
	}

	// Check if resource server is declarative (immutable)
	if rs.IsResourceServerDeclarative(resourceServerID) {
		return nil, ErrorImmutableAction.WithParams(map[string]string{"id": id})
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
		return nil, &tidcommon.InternalServerError
	}

	// Kind is immutable; reject any explicit change and preserve the stored value.
	if action.Kind != "" && action.Kind != currentAction.Kind {
		return nil, &ErrorInvalidRequestFormat
	}

	// Update only name and description (handle and kind are immutable)
	updateAction := providers.Action{
		Name:        action.Name,
		Handle:      currentAction.Handle, // Immutable - preserve
		Description: action.Description,
		Kind:        currentAction.Kind, // Immutable - preserve
	}

	// Use transaction for write operation
	var updatedAction *providers.Action
	if err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.resourceStore.UpdateAction(
			txCtx, id, resourceServerID, resID, updateAction,
		); err != nil {
			rs.logger.Error(ctx, "Failed to update action", log.Error(err))
			return err
		}

		updatedAction = &providers.Action{
			ID:          id,
			Name:        updateAction.Name,
			Handle:      updateAction.Handle,
			Description: updateAction.Description,
			Kind:        updateAction.Kind,
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
) *tidcommon.ServiceError {
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
		return ErrorImmutableAction.WithParams(map[string]string{"id": id})
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
		return &tidcommon.InternalServerError
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
			return &tidcommon.InternalServerError
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
) ([]string, *tidcommon.ServiceError) {
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
			return nil, &tidcommon.InternalServerError
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
		return nil, &tidcommon.InternalServerError
	}

	return invalidPermissions, nil
}

// FindResourceServersByPermissions returns registered resource servers that define at least one
// permission in the supplied set.
func (rs *resourceService) FindResourceServersByPermissions(
	ctx context.Context,
	permissions []string,
) ([]providers.ResourceServer, *tidcommon.ServiceError) {
	if len(permissions) == 0 {
		return []providers.ResourceServer{}, nil
	}

	resourceServers, err := rs.resourceStore.FindResourceServersByPermissions(ctx, permissions)
	if err != nil {
		rs.logger.Error(ctx, "Failed to find resource servers by permissions", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return resourceServers, nil
}

// ResolveResourceServerOUHandle resolves ou_handle to an OU ID on the given resource server
// in-place. Called by the declarative loader validator so that file-based resource servers
// support ou_handle. If both ou_id and ou_handle are provided, ou_id wins and a warning is logged.
func (rs *resourceService) ResolveResourceServerOUHandle(
	ctx context.Context, server *providers.ResourceServer,
) *tidcommon.ServiceError {
	if server.OUID != "" && server.OUHandle != "" {
		rs.logger.Warn(ctx, "Both ou_id and ou_handle provided for resource server; ou_handle ignored",
			log.String("resourceServerID", server.ID), log.String("name", server.Name))
		return nil
	}
	if server.OUID == "" && server.OUHandle != "" {
		if rs.ouService == nil {
			return &tidcommon.InternalServerError
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
) (providers.ResourceServer, *tidcommon.ServiceError) {
	resourceServer, err := rs.resourceStore.GetResourceServer(ctx, resourceServerID)
	if err != nil {
		if errors.Is(err, errResourceServerNotFound) {
			return providers.ResourceServer{}, &ErrorResourceServerNotFound
		}
		rs.logger.Error(ctx, "Failed to check resource server", log.Error(err))
		return providers.ResourceServer{}, &tidcommon.InternalServerError
	}
	return resourceServer, nil
}

// validateAndGetResourceByID validates resource exists and returns it.
func (rs *resourceService) validateAndGetResourceByID(
	ctx context.Context,
	resourceID string,
	resourceServerID string,
) (providers.Resource, *tidcommon.ServiceError) {
	resource, err := rs.resourceStore.GetResource(ctx, resourceID, resourceServerID)
	if err != nil {
		if errors.Is(err, errResourceNotFound) {
			return providers.Resource{}, &ErrorResourceNotFound
		}
		rs.logger.Error(ctx, "Failed to check resource", log.Error(err))
		return providers.Resource{}, &tidcommon.InternalServerError
	}
	return resource, nil
}

// validateResourceServerCreate validates the input for creating a resource server.
func (rs *resourceService) validateResourceServerCreate(
	resourceServer providers.ResourceServer,
) *tidcommon.ServiceError {
	if resourceServer.Name == "" {
		return &ErrorInvalidRequestFormat
	}
	if resourceServer.OUID == "" {
		return &ErrorInvalidRequestFormat
	}
	if resourceServer.Identifier == "" {
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
func (rs *resourceService) validateResourceServerUpdate(
	resourceServer providers.ResourceServer,
) *tidcommon.ServiceError {
	if resourceServer.Name == "" {
		return &ErrorInvalidRequestFormat
	}
	if resourceServer.OUID == "" {
		return &ErrorInvalidRequestFormat
	}
	return nil
}

// validateResourceCreate validates the input for creating a resource.
func (rs *resourceService) validateResourceCreate(
	resource providers.Resource,
	delimiter string,
) *tidcommon.ServiceError {
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
func (rs *resourceService) validateActionCreate(action providers.Action, delimiter string) *tidcommon.ServiceError {
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

// validateActionKind rejects a non-empty kind that is not one of the supported values (tool|resource).
// An empty kind is allowed for all resource server types; MCP defaulting is applied by the caller.
func (rs *resourceService) validateActionKind(kind providers.ActionKind) *tidcommon.ServiceError {
	if kind != "" && !kind.IsValid() {
		return &ErrorInvalidRequestFormat
	}
	return nil
}

// validatePaginationParams validates pagination parameters.
func validatePaginationParams(limit, offset int) *tidcommon.ServiceError {
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
func validateDelimiter(delimiter string) *tidcommon.ServiceError {
	if len(delimiter) != 1 {
		return &ErrorInvalidDelimiter
	}
	if !strings.ContainsRune(ValidPermissionDelimiters, rune(delimiter[0])) {
		return &ErrorInvalidDelimiter
	}
	return nil
}

// validateHandle validates a handle string.
func validateHandle(handle string, delimiter string) *tidcommon.ServiceError {
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
	resourceServer providers.ResourceServer,
	parentResource *providers.Resource,
	handle string,
) string {
	if parentResource != nil {
		return parentResource.Permission + resourceServer.Delimiter + handle
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
		Namespace:   providers.NamespacePermission,
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

	existing, err := rs.consentService.ListConsentElements(ctx, ouID, providers.NamespacePermission, permission)
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

// wrapConsentServiceError wraps a consent service error in a consentSyncError so that callers can
// distinguish consent-service failures from other store or service errors during resource CRUD.
// Server-class failures are logged here so operators get a record even when the transaction
// closure collapses the error to InternalServerError on the way out.
func (rs *resourceService) wrapConsentServiceError(ctx context.Context, err *tidcommon.ServiceError) error {
	if err == nil {
		return nil
	}
	if err.Type == tidcommon.ServerErrorType {
		rs.logger.Error(ctx, "Consent service returned a server-class error during resource sync",
			log.String("code", err.Code),
			log.String("description", err.ErrorDescription.DefaultValue))
	}
	return &consentSyncError{Underlying: err}
}

// translateTxError converts a transaction-closure error into the resource service's
// *tidcommon.ServiceError API surface. A typed *consentSyncError is mapped to
// ErrorConsentSyncFailed for client-class consent failures (preserving the underlying code in the
// description) and to InternalServerError otherwise. All other transaction errors collapse to
// InternalServerError. This mirrors the inboundclient + agent/application translation pattern.
func translateTxError(err error) *tidcommon.ServiceError {
	var consentErr *consentSyncError
	if errors.As(err, &consentErr) {
		if consentErr.IsClientError() {
			return ErrorConsentSyncFailed.WithParams(map[string]string{"code": consentErr.Underlying.Code})
		}
		return &tidcommon.InternalServerError
	}
	return &tidcommon.InternalServerError
}

// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package role provides role management functionality.
package role

import (
	"context"
	"errors"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "RoleMgtService"

// RoleServiceInterface defines the interface for the role service.
type RoleServiceInterface interface {
	GetRoleList(ctx context.Context, limit, offset int) (*RoleList, *tidcommon.ServiceError)
	CreateRole(ctx context.Context, role RoleCreationDetail) (
		*RoleWithPermissionsAndAssignments, *tidcommon.ServiceError)
	GetRoleWithPermissions(ctx context.Context, id string) (*RoleWithPermissions, *tidcommon.ServiceError)
	UpdateRoleWithPermissions(ctx context.Context, id string, role RoleUpdateDetail) (
		*RoleWithPermissions, *tidcommon.ServiceError)
	DeleteRole(ctx context.Context, id string) *tidcommon.ServiceError
	IsRoleDeclarative(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
	GetAuthorizedPermissionsByResourceServer(
		ctx context.Context, entityID string, groups []string, resourceServerID string, requestedPermissions []string,
	) ([]string, *tidcommon.ServiceError)
	// GetAllPermissions returns every permission the entity and/or groups hold, keyed by resource
	// server. Unlike GetAuthorizedPermissionsByResourceServer it enumerates rather than checks.
	GetAllPermissions(
		ctx context.Context, entityID string, groupIDs []string,
	) (security.PermissionSet, *tidcommon.ServiceError)
	GetUserRoles(ctx context.Context, entityID string, groupIDs []string) ([]string, *tidcommon.ServiceError)
	ResolveRoleOUHandle(
		ctx context.Context, role *RoleWithPermissionsAndAssignments,
	) *tidcommon.ServiceError
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	CascadeDeleteDependencies(ctx context.Context, resourceType, id string) (int, error)
}

// roleService is the default implementation of the RoleServiceInterface.
type roleService struct {
	roleStore       roleStoreInterface
	entityService   entity.EntityServiceInterface
	groupService    group.GroupServiceInterface
	ouService       oupkg.OrganizationUnitServiceInterface
	resourceService resourcepkg.ResourceServiceInterface
	transactioner   providers.Transactioner
	authzService    sysauthz.SystemAuthorizationServiceInterface
}

// newRoleService creates a new instance of RoleService with injected dependencies.
func newRoleService(
	roleStore roleStoreInterface,
	entityService entity.EntityServiceInterface,
	groupService group.GroupServiceInterface,
	ouService oupkg.OrganizationUnitServiceInterface,
	resourceService resourcepkg.ResourceServiceInterface,
	transactioner providers.Transactioner,
	authzService sysauthz.SystemAuthorizationServiceInterface,
) RoleServiceInterface {
	return &roleService{
		roleStore:       roleStore,
		entityService:   entityService,
		groupService:    groupService,
		ouService:       ouService,
		resourceService: resourceService,
		transactioner:   transactioner,
		authzService:    authzService,
	}
}

// GetRoleList retrieves a list of roles.
func (rs *roleService) GetRoleList(ctx context.Context, limit, offset int) (*RoleList, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	totalCount, err := rs.roleStore.GetRoleListCount(ctx)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		logger.Error(ctx, "Failed to get role count", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	roles, err := rs.roleStore.GetRoleList(ctx, limit, offset)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		logger.Error(ctx, "Failed to list roles", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if len(roles) > 0 {
		seen := make(map[string]struct{}, len(roles))
		ouIDs := make([]string, 0, len(roles))
		for _, r := range roles {
			if r.OUID != "" {
				if _, exists := seen[r.OUID]; !exists {
					ouIDs = append(ouIDs, r.OUID)
					seen[r.OUID] = struct{}{}
				}
			}
		}
		ouHandles, svcErr := rs.ouService.GetOrganizationUnitHandlesByIDs(ctx, ouIDs)
		if svcErr != nil {
			logger.Warn(ctx, "Failed to resolve OU handles for roles, skipping", log.Any("error", svcErr))
		} else {
			for i := range roles {
				roles[i].OUHandle = ouHandles[roles[i].OUID]
			}
		}
	}

	response := &RoleList{
		TotalResults: totalCount,
		Roles:        roles,
		StartIndex:   offset + 1,
		Count:        len(roles),
		Links:        utils.BuildPaginationLinks("/roles", limit, offset, totalCount, ""),
	}

	return response, nil
}

// CreateRole creates a new role.
func (rs *roleService) CreateRole(
	ctx context.Context, role RoleCreationDetail,
) (*RoleWithPermissionsAndAssignments, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Creating role", log.String("name", role.Name))

	// Check if role creation is allowed (not in declarative-only mode)
	if isDeclarativeModeEnabled() {
		logger.Debug(ctx, "Cannot create role in declarative-only mode")
		return nil, &ErrorDeclarativeModeCreateNotAllowed
	}

	if err := rs.validateCreateRoleRequest(role); err != nil {
		return nil, err
	}

	responseAssignments := role.Assignments

	// Validate organization unit exists using OU service
	ou, svcErr := rs.ouService.GetOrganizationUnit(ctx, role.OUID)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			logger.Debug(ctx, "Organization unit not found", log.String("ouID", role.OUID))
			return nil, &ErrorOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to validate organization unit",
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}

	// Validate permissions exist in resource management system
	if err := rs.validatePermissions(ctx, role.Permissions); err != nil {
		return nil, err
	}

	// Defining a role conveys its permissions to future assignees. Inline assignments need no
	// separate check: they assign this same role, already covered above.
	if svcErr := rs.authzService.CanGrantPermissions(
		ctx, toPermissionSet(role.Permissions),
	); svcErr != nil {
		return nil, svcErr
	}

	// Validate assignment IDs (existence + category check) before normalization.
	if len(role.Assignments) > 0 {
		if err := rs.validateAssignmentIDs(ctx, role.Assignments); err != nil {
			return nil, err
		}
	}

	role.Assignments = normalizeAssignments(role.Assignments)

	// Check if role name already exists in the organization unit
	nameExists, err := rs.roleStore.CheckRoleNameExists(ctx, role.OUID, role.Name)
	if err != nil {
		logger.Error(ctx, "Failed to check role name existence", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if nameExists {
		logger.Debug(ctx, "Role name already exists in organization unit",
			log.String("name", role.Name), log.String("ouID", role.OUID))
		return nil, &ErrorRoleNameConflict
	}

	id := role.ID
	if id == "" {
		id, err = utils.GenerateUUIDv7()
		if err != nil {
			logger.Error(ctx, "Failed to generate UUID", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
	} else {
		_, err = rs.roleStore.GetRole(ctx, id)
		if err != nil && !errors.Is(err, ErrRoleNotFound) {
			logger.Error(ctx, "Failed to check role ID existence", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		if err == nil {
			logger.Debug(ctx, "Role ID already exists", log.String("id", id))
			return nil, &ErrorRoleIDConflict
		}
	}

	serviceRole := &RoleWithPermissionsAndAssignments{
		ID:          id,
		Name:        role.Name,
		Description: role.Description,
		OUID:        role.OUID,
		OUHandle:    ou.Handle,
		Permissions: role.Permissions,
		Assignments: responseAssignments,
	}

	err = rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.roleStore.CreateRole(txCtx, id, role); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		logger.Error(ctx, "Failed to create role", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully created role", log.String("id", id), log.String("name", role.Name))
	return serviceRole, nil
}

// GetRoleWithPermissions retrieves a specific role by its id.
func (rs *roleService) GetRoleWithPermissions(ctx context.Context, id string) (
	*RoleWithPermissions, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Retrieving role", log.String("id", id))

	if id == "" {
		return nil, &ErrorMissingRoleID
	}

	role, err := rs.roleStore.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			logger.Debug(ctx, "Role not found", log.String("id", id))
			return nil, &ErrorRoleNotFound
		}
		logger.Error(ctx, "Failed to retrieve role", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	ou, svcErr := rs.ouService.GetOrganizationUnit(ctx, role.OUID)
	if svcErr != nil {
		logger.Warn(ctx, "Failed to resolve OU handle for role, skipping",
			log.String("id", id), log.Any("error", svcErr))
	} else {
		role.OUHandle = ou.Handle
	}

	logger.Debug(ctx, "Successfully retrieved role",
		log.String("id", role.ID), log.String("name", role.Name))
	return &role, nil
}

// UpdateRole updates an existing role.
func (rs *roleService) UpdateRoleWithPermissions(
	ctx context.Context, id string, role RoleUpdateDetail) (*RoleWithPermissions, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Updating role", log.String("id", id), log.String("name", role.Name))

	if id == "" {
		return nil, &ErrorMissingRoleID
	}

	if err := rs.validateUpdateRoleRequest(role); err != nil {
		return nil, err
	}

	// Validate permissions exist in resource management system
	if err := rs.validatePermissions(ctx, role.Permissions); err != nil {
		return nil, err
	}

	// An update replaces the permission list, so the incoming set is what the role will confer.
	if svcErr := rs.authzService.CanGrantPermissions(
		ctx, toPermissionSet(role.Permissions),
	); svcErr != nil {
		return nil, svcErr
	}

	exists, err := rs.roleStore.IsRoleExist(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to check role existence", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if !exists {
		logger.Debug(ctx, "Role not found", log.String("id", id))
		return nil, &ErrorRoleNotFound
	}

	// Check if role is declarative - cannot modify declarative roles
	if rs.isRoleDeclarative(ctx, id) {
		logger.Debug(ctx, "Cannot modify declarative role", log.String("id", id))
		return nil, &ErrorImmutableRole
	}

	// Validate organization unit exists using OU service
	ou, svcErr := rs.ouService.GetOrganizationUnit(ctx, role.OUID)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			logger.Debug(ctx, "Organization unit not found", log.String("ouID", role.OUID))
			return nil, &ErrorOrganizationUnitNotFound
		}
		logger.Error(ctx, "Failed to validate organization unit",
			log.String("error", svcErr.Error.DefaultValue))
		return nil, &tidcommon.InternalServerError
	}

	// Check if role name already exists in the organization unit (excluding the current role)
	nameExists, err := rs.roleStore.CheckRoleNameExistsExcludingID(ctx, role.OUID, role.Name, id)
	if err != nil {
		logger.Error(ctx, "Failed to check role name existence", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if nameExists {
		logger.Debug(ctx, "Role name already exists in organization unit",
			log.String("name", role.Name), log.String("ouID", role.OUID))
		return nil, &ErrorRoleNameConflict
	}

	err = rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return rs.roleStore.UpdateRole(txCtx, id, role)
	})

	if err != nil {
		logger.Error(ctx, "Failed to update role", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully updated role", log.String("id", id), log.String("name", role.Name))
	return &RoleWithPermissions{
		ID:          id,
		Name:        role.Name,
		Description: role.Description,
		OUID:        role.OUID,
		OUHandle:    ou.Handle,
		Permissions: role.Permissions,
	}, nil
}

// DeleteRole delete the specified role by its id.
func (rs *roleService) DeleteRole(ctx context.Context, id string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Deleting role", log.String("id", id))

	if id == "" {
		return &ErrorMissingRoleID
	}

	exists, err := rs.roleStore.IsRoleExist(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to check role existence", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if !exists {
		logger.Debug(ctx, "Role not found", log.String("id", id))
		return nil
	}

	// Check if role is declarative - cannot delete declarative roles
	if rs.isRoleDeclarative(ctx, id) {
		logger.Debug(ctx, "Cannot delete declarative role", log.String("id", id))
		return &ErrorImmutableRole
	}

	// Delete all assignments for the role before deleting the role itself (cascade delete).
	// The ROLE_ASSIGNMENT table does not have a FK constraint on ROLE_ID to allow assignments
	// for roles that live in the file-based store, so cascade delete is handled here in code.
	err = rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if err := rs.roleStore.DeleteAssignmentsByRoleID(txCtx, id); err != nil {
			return err
		}
		return rs.roleStore.DeleteRole(txCtx, id)
	})
	if err != nil {
		logger.Error(ctx, "Failed to delete role", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully deleted role", log.String("id", id))
	return nil
}

// GetAuthorizedPermissionsByResourceServer checks which requested permissions are authorized for the entity
// based on roles, scoped to a resource server when provided.
func (rs *roleService) GetAuthorizedPermissionsByResourceServer(
	ctx context.Context, entityID string, groups []string, resourceServerID string, requestedPermissions []string,
) ([]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Authorizing permissions",
		log.MaskedString(log.LoggerKeyUserID, entityID),
		log.Int("groupCount", len(groups)),
		log.String("resourceServerID", resourceServerID))

	// Handle nil groups slice
	if groups == nil {
		groups = []string{}
	}

	// Validate that at least entityID or groups is provided
	if entityID == "" && len(groups) == 0 {
		return nil, &ErrorMissingEntityOrGroups
	}

	// Return empty list if no permissions requested
	if len(requestedPermissions) == 0 {
		return []string{}, nil
	}

	// Get authorized permissions from store
	authorizedPermissions, err := rs.roleStore.GetAuthorizedPermissionsByResourceServer(
		ctx, entityID, groups, resourceServerID, requestedPermissions)
	if err != nil {
		logger.Error(ctx, "Failed to get authorized permissions",
			log.MaskedString(log.LoggerKeyUserID, entityID),
			log.Int("groupCount", len(groups)),
			log.String("resourceServerID", resourceServerID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Retrieved authorized permissions",
		log.MaskedString(log.LoggerKeyUserID, entityID),
		log.Int("groupCount", len(groups)),
		log.String("resourceServerID", resourceServerID),
		log.Int("requestedCount", len(requestedPermissions)),
		log.Int("authorizedCount", len(authorizedPermissions)))

	return authorizedPermissions, nil
}

// GetAllPermissions retrieves every permission the entity and/or groups hold through their assigned
// roles, keyed by resource server. Unlike GetAuthorizedPermissionsByResourceServer, no entity and no
// groups is not an error: callers legitimately ask what a set of groups confers.
func (rs *roleService) GetAllPermissions(
	ctx context.Context, entityID string, groupIDs []string,
) (security.PermissionSet, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if groupIDs == nil {
		groupIDs = []string{}
	}
	if entityID == "" && len(groupIDs) == 0 {
		return security.PermissionSet{}, nil
	}

	resourcePermissions, err := rs.roleStore.GetAllPermissionsForAssignees(ctx, entityID, groupIDs)
	if err != nil {
		logger.Error(ctx, "Failed to enumerate permissions for assignees",
			log.MaskedString(log.LoggerKeyUserID, entityID),
			log.Int("groupCount", len(groupIDs)),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	permissionSet := make(security.PermissionSet, len(resourcePermissions))
	for _, rp := range resourcePermissions {
		permissionSet[rp.ResourceServerID] = rp.Permissions
	}

	logger.Debug(ctx, "Enumerated permissions for assignees",
		log.MaskedString(log.LoggerKeyUserID, entityID),
		log.Int("groupCount", len(groupIDs)),
		log.Int("resourceServerCount", len(permissionSet)))

	return permissionSet, nil
}

// GetUserRoles retrieves the names of roles assigned to an entity directly and/or through group membership.
func (rs *roleService) GetUserRoles(
	ctx context.Context, entityID string, groupIDs []string,
) ([]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Getting entity roles",
		log.MaskedString("entityID", entityID), log.Int("groupCount", len(groupIDs)))

	if groupIDs == nil {
		groupIDs = []string{}
	}

	if entityID == "" && len(groupIDs) == 0 {
		return []string{}, nil
	}

	roles, err := rs.roleStore.GetUserRoles(ctx, entityID, groupIDs)
	if err != nil {
		logger.Error(ctx, "Failed to get entity roles",
			log.MaskedString("entityID", entityID), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return roles, nil
}

// IsRoleDeclarative returns true if the role is declarative.
func (rs *roleService) IsRoleDeclarative(ctx context.Context, id string) (bool, *tidcommon.ServiceError) {
	isDeclarative, err := rs.roleStore.IsRoleDeclarative(ctx, id)
	if err != nil {
		return false, &tidcommon.InternalServerError
	}

	return isDeclarative, nil
}

// ResolveRoleOUHandle resolves ou_handle to an OU ID on the given role in-place.
// Called by the declarative loader validator so that file-based roles support ou_handle.
// If both ou_id and ou_handle are provided, ou_id wins and a warning is logged.
func (rs *roleService) ResolveRoleOUHandle(
	ctx context.Context, role *RoleWithPermissionsAndAssignments,
) *tidcommon.ServiceError {
	if role.OUID != "" && role.OUHandle != "" {
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
		logger.Warn(ctx, "Both ouId and ouHandle provided for role; ouHandle ignored",
			log.String("roleID", role.ID), log.String("name", role.Name))
		return nil
	}
	if role.OUID == "" && role.OUHandle != "" {
		if rs.ouService == nil {
			return &tidcommon.InternalServerError
		}
		ou, svcErr := rs.ouService.GetOrganizationUnitByPath(
			security.WithRuntimeContext(ctx), role.OUHandle)
		if svcErr != nil {
			return &ErrorInvalidRequestFormat
		}
		role.OUID = ou.ID
	}
	return nil
}

// validateCreateRoleRequest validates the create role request.
func (rs *roleService) validateCreateRoleRequest(role RoleCreationDetail) *tidcommon.ServiceError {
	if role.Name == "" {
		return &ErrorInvalidRequestFormat
	}

	if role.OUID == "" {
		return &ErrorInvalidRequestFormat
	}

	if len(role.Assignments) > 0 {
		if err := rs.validateAssignmentsRequest(role.Assignments); err != nil {
			return err
		}
	}

	return nil
}

// validateUpdateRoleRequest validates the update role request.
func (rs *roleService) validateUpdateRoleRequest(request RoleUpdateDetail) *tidcommon.ServiceError {
	if request.Name == "" {
		return &ErrorInvalidRequestFormat
	}

	if request.OUID == "" {
		return &ErrorInvalidRequestFormat
	}

	return nil
}

// validateAssignmentsRequest validates the assignments request.
// Accepts public types 'user', 'app', 'group'.
func (rs *roleService) validateAssignmentsRequest(assignments []RoleAssignment) *tidcommon.ServiceError {
	if len(assignments) == 0 {
		return &ErrorEmptyAssignments
	}

	for _, assignment := range assignments {
		if !assignment.Type.IsEntityType() && assignment.Type != AssigneeTypeGroup {
			return &ErrorInvalidAssigneeType
		}
		if assignment.ID == "" {
			return &ErrorInvalidRequestFormat
		}
	}

	return nil
}

// validateAssignmentIDs validates assignment IDs before normalization.
// For user/app assignments it checks existence and verifies the claimed type matches the actual
// entity category. For group assignments it checks existence via the group service.
func (rs *roleService) validateAssignmentIDs(
	ctx context.Context, assignments []RoleAssignment) *tidcommon.ServiceError {
	return validateAssignmentIDs(ctx, assignments, rs.entityService, rs.groupService, loggerComponentName)
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

// validatePermissions validates that all permissions exist in the resource management system.
func (rs *roleService) validatePermissions(
	ctx context.Context, permissions []ResourcePermissions,
) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if len(permissions) == 0 {
		return nil
	}

	// Validate each resource server's permissions
	for _, resPerm := range permissions {
		if resPerm.ResourceServerID == "" {
			logger.Debug(ctx, "Empty resource server ID")
			return &ErrorInvalidPermissions
		}

		if len(resPerm.Permissions) == 0 {
			continue
		}

		// Call resource service to validate permissions
		invalidPerms, svcErr := rs.resourceService.ValidatePermissions(
			ctx,
			resPerm.ResourceServerID,
			resPerm.Permissions,
		)

		if svcErr != nil {
			logger.Error(ctx, "Failed to validate permissions",
				log.String("resourceServerId", resPerm.ResourceServerID),
				log.String("error", svcErr.Error.DefaultValue))
			return &tidcommon.InternalServerError
		}

		// If any permissions are invalid, return error
		if len(invalidPerms) > 0 {
			logger.Debug(ctx, "Invalid permissions found",
				log.String("resourceServerId", resPerm.ResourceServerID),
				log.Any("invalidPermissions", invalidPerms),
				log.Int("count", len(invalidPerms)))
			return &ErrorInvalidPermissions
		}
	}

	return nil
}

// isRoleDeclarative checks if a role is defined in declarative configuration.
func (rs *roleService) isRoleDeclarative(ctx context.Context, roleID string) bool {
	// Check the store mode - if it's mutable, no roles are declarative
	storeMode := getRoleStoreMode()
	if storeMode == serverconst.StoreModeMutable {
		return false
	}

	// For declarative and composite modes, check with store
	// Note: This is a placeholder implementation
	// Actual implementation would check against declarative config
	isDeclarative, err := rs.roleStore.IsRoleDeclarative(ctx, roleID)
	if err != nil {
		// Log at Warn level and fail open - treat as non-declarative on error
		// RISK: In composite mode, this could allow modification of declarative roles if the check fails
		logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
		logger.Warn(ctx, "Failed to check if role is declarative",
			log.String("roleID", roleID), log.Error(err))
		return false
	}

	return isDeclarative
}

// GetResourceDependencies implements resourcedependency.Provider. Role permissions are cleaned up
// via cascade rather than surfaced as blocking usages, so no dependencies are reported here.
func (rs *roleService) GetResourceDependencies(
	_ context.Context, _, _ string) ([]resourcedependency.ResourceDependency, error) {
	return []resourcedependency.ResourceDependency{}, nil
}

// CascadeDeleteDependencies implements resourcedependency.CascadeDeleter. When a resource server,
// resource or action is deleted, the permissions it contributed can no longer be resolved, so they
// are removed from every role holding them. Permissions are stored as opaque strings scoped to a
// resource server rather than as references to the resource that defines them, so the orphans are
// found by re-validating the referenced permissions against the resource service. This also clears
// any permission orphaned by an earlier deletion. Must be called after the target has been deleted,
// so the deleted permissions no longer validate.
func (rs *roleService) CascadeDeleteDependencies(
	ctx context.Context, resourceType, _ string) (int, error) {
	switch resourceType {
	case resourcedependency.ResourceTypeResourceServer,
		resourcedependency.ResourceTypeResource,
		resourcedependency.ResourceTypeAction:
	default:
		return 0, nil
	}

	referenced, err := rs.roleStore.GetReferencedPermissions(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get permissions referenced by roles: %w", err)
	}

	deleted := 0
	for _, resPerm := range referenced {
		invalid, svcErr := rs.resourceService.ValidatePermissions(
			ctx, resPerm.ResourceServerID, resPerm.Permissions)
		if svcErr != nil {
			return deleted, fmt.Errorf("failed to validate permissions of resource server %s: %s",
				resPerm.ResourceServerID, svcErr.Error.DefaultValue)
		}

		for _, permission := range invalid {
			removed, err := rs.roleStore.DeleteRolePermission(ctx, resPerm.ResourceServerID, permission)
			if err != nil {
				return deleted, fmt.Errorf("failed to delete orphaned role permission: %w", err)
			}
			deleted += int(removed)
		}
	}

	return deleted, nil
}

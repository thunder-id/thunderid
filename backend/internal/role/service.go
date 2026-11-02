// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package role provides role management functionality.
package role

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const loggerComponentName = "RoleMgtService"

// RoleServiceInterface defines the interface for the role service.
type RoleServiceInterface interface {
	GetRoleList(ctx context.Context, limit, offset int, filters map[string]interface{}) (
		*RoleList, *tidcommon.ServiceError)
	CreateRole(ctx context.Context, role RoleCreationDetail) (
		*RoleWithPermissionsAndAssignments, *tidcommon.ServiceError)
	GetRoleWithPermissions(ctx context.Context, id string) (*RoleWithPermissions, *tidcommon.ServiceError)
	GetRoleInOU(ctx context.Context, id, ouID string) (*RoleWithPermissions, *tidcommon.ServiceError)
	GetRolesByNames(ctx context.Context, names []string) (map[string][]*Role, *tidcommon.ServiceError)
	UpdateRoleWithPermissions(ctx context.Context, id string, role RoleUpdateDetail) (
		*RoleWithPermissions, *tidcommon.ServiceError)
	DeleteRole(ctx context.Context, id string) *tidcommon.ServiceError
	// ValidateDeleteRole reports whether the role may be deleted and returns what the deletion revokes.
	ValidateDeleteRole(ctx context.Context, id string) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError)
	// ValidateUpdateRolePermissions reports whether the permissions may be replaced and what that revokes.
	ValidateUpdateRolePermissions(ctx context.Context, id string, permissions []ResourcePermissions) (
		*revocation.AccessRevocationTarget, *tidcommon.ServiceError)
	// ValidateGroupDeletion reports whether the group may be deleted and returns what the deletion revokes.
	ValidateGroupDeletion(ctx context.Context, groupID string) (
		*revocation.AccessRevocationTarget, *tidcommon.ServiceError)
	// ValidateGroupMembershipRemoval reports whether the member may be removed and returns what that revokes.
	ValidateGroupMembershipRemoval(ctx context.Context, groupID, memberID string) (
		*revocation.AccessRevocationTarget, *tidcommon.ServiceError)
	IsRoleDeclarative(ctx context.Context, id string) (bool, *tidcommon.ServiceError)
	// GetAuthorizedPermissionsByResourceServer counts only the assignments made in assignmentOUID
	// when one is given, and assignments made anywhere otherwise.
	GetAuthorizedPermissionsByResourceServer(
		ctx context.Context, entityID string, groups, roleIDs []string, assignmentOUID, resourceServerID string,
		requestedPermissions []string,
	) ([]string, *tidcommon.ServiceError)
	// GetAllPermissions returns every permission the entity and/or groups hold, keyed by resource
	// server. Unlike GetAuthorizedPermissionsByResourceServer it enumerates rather than checks.
	GetAllPermissions(
		ctx context.Context, entityID string, groupIDs []string,
	) (security.PermissionSet, *tidcommon.ServiceError)
	// GetUserRoles returns the names of the roles assigned to the entity directly or through its
	// groups, counting only the assignments made in assignmentOUID when one is given.
	GetUserRoles(
		ctx context.Context, entityID string, groupIDs []string, assignmentOUID string,
	) ([]string, *tidcommon.ServiceError)
	ResolveRoleOUHandle(
		ctx context.Context, role *RoleWithPermissionsAndAssignments,
	) *tidcommon.ServiceError
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	CascadeDeleteDependencies(ctx context.Context, resourceType, id string) (int, error)

	// Sharing policies decide which organization units a role is shared with.
	CreateSharingPolicy(ctx context.Context, roleID string, req sharing.PolicyRequest) (
		sharing.Policy, *tidcommon.ServiceError)
	GetSharingPolicy(ctx context.Context, roleID, policyID string) (
		sharing.Policy, *tidcommon.ServiceError)
	ListSharingPolicies(ctx context.Context, roleID string, limit, offset int) (
		sharing.PolicyList, *tidcommon.ServiceError)
	UpdateSharingPolicy(ctx context.Context, roleID, policyID string, req sharing.PolicyRequest) (
		sharing.Policy, *tidcommon.ServiceError)
	DeleteSharingPolicy(ctx context.Context, roleID, policyID string) *tidcommon.ServiceError
	ResolveSharingOverlay(ctx context.Context, roleID, ouID string) (
		sharing.ResolvedOverlay, *tidcommon.ServiceError)
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
	sharingService  sharing.SharingServiceInterface
	logger          *log.Logger
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
	sharingService sharing.SharingServiceInterface,
) RoleServiceInterface {
	return &roleService{
		roleStore:       roleStore,
		entityService:   entityService,
		groupService:    groupService,
		ouService:       ouService,
		resourceService: resourceService,
		transactioner:   transactioner,
		authzService:    authzService,
		sharingService:  sharingService,
		logger:          log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
	}
}

// GetRoleList retrieves a list of roles. An organization unit filter lists the roles that unit
// owns, followed by the ones shared with it.
func (rs *roleService) GetRoleList(
	ctx context.Context, limit, offset int, filters map[string]interface{},
) (*RoleList, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	ouID, rest, err := filter.SplitOUIDFilter(filters)
	if err != nil || len(rest) > 0 {
		return nil, &ErrorInvalidFilter
	}

	var roles []Role
	var totalCount int
	if ouID != "" {
		var svcErr *tidcommon.ServiceError
		roles, totalCount, svcErr = rs.listRolesInOU(ctx, ouID, limit, offset)
		if svcErr != nil {
			return nil, svcErr
		}
	} else {
		totalCount, err = rs.roleStore.GetRoleListCount(ctx)
		if err != nil {
			if errors.Is(err, errResultLimitExceededInCompositeMode) {
				return nil, &ResultLimitExceededInCompositeMode
			}
			logger.Error(ctx, "Failed to get role count", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}

		roles, err = rs.roleStore.GetRoleList(ctx, limit, offset)
		if err != nil {
			if errors.Is(err, errResultLimitExceededInCompositeMode) {
				return nil, &ResultLimitExceededInCompositeMode
			}
			logger.Error(ctx, "Failed to list roles", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		for i := range roles {
			roles[i].IsOwnedByOU = true
		}
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
		Links:        utils.BuildPaginationLinks("/roles", limit, offset, totalCount, utils.FilterQueryParam(filters)),
	}

	return response, nil
}

// listRolesInOU lists the roles an organization unit owns, followed by the ones shared with it, and
// returns the total across every page.
func (rs *roleService) listRolesInOU(
	ctx context.Context, ouID string, limit, offset int,
) ([]Role, int, *tidcommon.ServiceError) {
	ownedCount, err := rs.roleStore.GetRoleListCountByOUID(ctx, ouID)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, 0, &ResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to count roles in organization unit", log.String("ouID", ouID), log.Error(err))
		return nil, 0, &tidcommon.InternalServerError
	}

	sharedIDs, svcErr := rs.sharingService.ListVisibleResourceIDs(ctx, RoleSharingType, ouID)
	if svcErr != nil {
		// An organization unit that does not exist has nothing shared with it.
		if svcErr.Code != oupkg.ErrorOrganizationUnitNotFound.Code {
			return nil, 0, translateSharingError(svcErr)
		}
		sharedIDs = nil
	}

	owned, shared, err := utils.PaginateOwnedThenShared(
		ownedCount,
		func(limit, offset int) ([]Role, error) {
			return rs.roleStore.GetRoleListByOUID(ctx, ouID, limit, offset)
		},
		sharedIDs,
		func(ids []string) ([]Role, error) { return rs.roleStore.GetRolesByIDs(ctx, ids) },
		func(r Role) string { return r.ID },
		limit, offset,
	)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, 0, &ResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to list roles in organization unit", log.String("ouID", ouID), log.Error(err))
		return nil, 0, &tidcommon.InternalServerError
	}

	for i := range owned {
		owned[i].IsOwnedByOU = true
	}
	// A shared role is listed under the organization unit it is shared with, where only what its
	// sharing policies make editable can change.
	for i := range shared {
		shared[i].OUID = ouID
		shared[i].IsOwnedByOU = false
		shared[i].IsReadOnly = true
	}
	return slices.Concat(owned, shared), ownedCount + len(sharedIDs), nil
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
		// A role is created in its owner, so that is where its assignees must be available.
		if err := validateAssignmentIDs(ctx, role.Assignments, role.OUID, rs.entityService, rs.groupService,
			rs.sharingService, loggerComponentName); err != nil {
			return nil, err
		}
	}

	role.Assignments = normalizeAssignments(role.Assignments)

	if svcErr := rs.requireRoleNameAvailable(ctx, role.OUID, role.Name, ""); svcErr != nil {
		return nil, svcErr
	}

	id := role.ID
	var err error
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

	role.IsOwnedByOU = true

	logger.Debug(ctx, "Successfully retrieved role",
		log.String("id", role.ID), log.String("name", role.Name))
	return &role, nil
}

// GetRoleInOU retrieves a role as an organization unit sees it: the role itself in the organization
// unit that owns it, and the same role under an organization unit it is shared with. A role carries
// nothing a shared organization unit sets for itself: its assignments there are read separately.
func (rs *roleService) GetRoleInOU(ctx context.Context, id, ouID string) (
	*RoleWithPermissions, *tidcommon.ServiceError) {
	role, svcErr := rs.GetRoleWithPermissions(ctx, id)
	if svcErr != nil || ouID == "" || role.OUID == ouID {
		return role, svcErr
	}

	visible, svcErr := rs.sharingService.IsVisible(ctx, RoleSharingType, id, ouID)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return nil, &ErrorRoleNotSharedToOU
		}
		return nil, translateSharingError(svcErr)
	}
	if !visible {
		return nil, &ErrorRoleNotSharedToOU
	}

	role.OUID = ouID
	role.OUHandle = ""
	role.IsOwnedByOU = false
	ou, svcErr := rs.ouService.GetOrganizationUnit(ctx, ouID)
	if svcErr != nil {
		rs.logger.Warn(ctx, "Failed to resolve OU handle for shared role, skipping",
			log.String("id", id), log.Any("error", svcErr))
	} else {
		role.OUHandle = ou.Handle
	}
	return role, nil
}

// GetRolesByNames retrieves roles by a list of names, keyed by name. A name may resolve to more than
// one role across organization units; the caller decides how to handle that ambiguity.
func (rs *roleService) GetRolesByNames(
	ctx context.Context, names []string,
) (map[string][]*Role, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	if len(names) == 0 {
		return map[string][]*Role{}, nil
	}

	seen := make(map[string]struct{}, len(names))
	uniqueNames := make([]string, 0, len(names))
	for _, name := range names {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			uniqueNames = append(uniqueNames, name)
		}
	}

	roles, err := rs.roleStore.GetRolesByNames(ctx, uniqueNames)
	if err != nil {
		logger.Error(ctx, "Failed to get roles by names", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	result := make(map[string][]*Role, len(roles))
	for i := range roles {
		result[roles[i].Name] = append(result[roles[i].Name], &roles[i])
	}

	return result, nil
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

	// A role's name, description and permissions stay with the organization unit that owns it, so an
	// update naming one it is shared with is refused. Any other organization unit is somewhere to
	// move the role to.
	ownerOUID, sharedWith, svcErr := rs.resolveUpdateTarget(ctx, id, role.OUID)
	if svcErr != nil {
		return nil, svcErr
	}
	if sharedWith {
		return nil, &ErrorSharedRoleNotEditable
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

	if svcErr := rs.validateRoleMutable(ctx, id); svcErr != nil {
		return nil, svcErr
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

	if svcErr := rs.requireRoleNameAvailable(ctx, role.OUID, role.Name, id); svcErr != nil {
		return nil, svcErr
	}

	if svcErr := rs.requireRoleMovable(ctx, id, ownerOUID, role.OUID); svcErr != nil {
		return nil, svcErr
	}

	err := rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
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
		IsOwnedByOU: true,
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

	if svcErr := rs.validateRoleMutable(ctx, id); svcErr != nil {
		return svcErr
	}

	var capturedSvcErr *tidcommon.ServiceError
	err = rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// The sharing policies recorded against the role go with it.
		if svcErr := rs.sharingService.DeletePoliciesForResource(txCtx, RoleSharingType, id); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("rollback for refused sharing policy removal")
		}

		// Delete all assignments for the role before deleting the role itself (cascade delete).
		// The ROLE_ASSIGNMENT table does not have a FK constraint on ROLE_ID to allow assignments
		// for roles that live in the file-based store, so cascade delete is handled here in code.
		if err := rs.roleStore.DeleteAssignmentsByRoleID(txCtx, id); err != nil {
			return err
		}
		return rs.roleStore.DeleteRole(txCtx, id)
	})
	if capturedSvcErr != nil {
		return translateSharingError(capturedSvcErr)
	}
	if err != nil {
		logger.Error(ctx, "Failed to delete role", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully deleted role", log.String("id", id))
	return nil
}

// ValidateDeleteRole reports whether the role may be deleted and returns what the deletion revokes.
func (rs *roleService) ValidateDeleteRole(ctx context.Context, id string) (
	*revocation.AccessRevocationTarget, *tidcommon.ServiceError) {
	role, svcErr := rs.GetRoleWithPermissions(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.validateRoleMutable(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	return rs.roleAccessChangeTarget(ctx, id, role.Permissions)
}

// ValidateUpdateRolePermissions runs the checks UpdateRoleWithPermissions runs on the new permissions.
func (rs *roleService) ValidateUpdateRolePermissions(ctx context.Context, id string,
	permissions []ResourcePermissions) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError) {
	role, svcErr := rs.GetRoleWithPermissions(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.validateRoleMutable(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.validatePermissions(ctx, permissions); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.authzService.CanGrantPermissions(ctx, toPermissionSet(permissions)); svcErr != nil {
		return nil, svcErr
	}
	return rs.roleAccessChangeTarget(ctx, id, droppedPermissions(role.Permissions, permissions))
}

// ValidateGroupDeletion runs the group service's checks for the deletion and returns what it revokes.
func (rs *roleService) ValidateGroupDeletion(ctx context.Context, groupID string) (
	*revocation.AccessRevocationTarget, *tidcommon.ServiceError) {
	if svcErr := rs.groupService.ValidateDeleteGroup(ctx, groupID); svcErr != nil {
		return nil, svcErr
	}
	scopes, svcErr := rs.groupScopes(ctx, groupID)
	if svcErr != nil {
		return nil, svcErr
	}
	entityIDs, svcErr := rs.groupService.GetTransitiveMemberEntities(ctx, groupID)
	if svcErr != nil {
		return nil, svcErr
	}
	return &revocation.AccessRevocationTarget{EntityIDs: entityIDs, Scopes: scopes}, nil
}

// ValidateGroupMembershipRemoval runs the group service's checks for the removal and returns what it revokes.
func (rs *roleService) ValidateGroupMembershipRemoval(ctx context.Context, groupID, memberID string) (
	*revocation.AccessRevocationTarget, *tidcommon.ServiceError) {
	// A removal of a non-member changes nothing, so it must not revoke anything.
	member, svcErr := rs.groupService.GetDirectGroupMember(ctx, groupID, memberID)
	if svcErr != nil {
		return nil, svcErr
	}
	if member == nil {
		return nil, &group.ErrorInvalidMemberID
	}
	if svcErr := rs.groupService.ValidateRemoveGroupMembers(ctx, groupID, []group.Member{*member}); svcErr != nil {
		return nil, svcErr
	}
	scopes, svcErr := rs.groupScopes(ctx, groupID)
	if svcErr != nil {
		return nil, svcErr
	}
	entityIDs, svcErr := expandPrincipal(ctx, rs.groupService, memberID, member.Type == group.MemberTypeGroup)
	if svcErr != nil {
		return nil, svcErr
	}
	return &revocation.AccessRevocationTarget{
		EntityIDs: entityIDs, Scopes: scopes, AssigneeType: string(member.Type),
	}, nil
}

// GetAuthorizedPermissionsByResourceServer checks which requested permissions are authorized for the entity
// based on roles, scoped to a resource server when provided.
func (rs *roleService) GetAuthorizedPermissionsByResourceServer(
	ctx context.Context, entityID string, groups, roleIDs []string, assignmentOUID, resourceServerID string,
	requestedPermissions []string,
) ([]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Authorizing permissions",
		log.MaskedString(log.LoggerKeyUserID, entityID),
		log.Int("groupCount", len(groups)),
		log.Int("roleCount", len(roleIDs)),
		log.String("resourceServerID", resourceServerID))

	// Handle nil groups/roleIDs slices
	if groups == nil {
		groups = []string{}
	}
	if roleIDs == nil {
		roleIDs = []string{}
	}

	// Validate that at least an entity, a group, or a role is provided. Role IDs stand in for the
	// entity/group assignment a locally managed subject would otherwise need: a federated subject
	// mapped directly to roles has neither an entity nor groups, but still has something to
	// evaluate.
	if entityID == "" && len(groups) == 0 && len(roleIDs) == 0 {
		return nil, &ErrorMissingEntityOrGroups
	}

	// Return empty list if no permissions requested
	if len(requestedPermissions) == 0 {
		return []string{}, nil
	}

	// Get authorized permissions from store
	authorizedPermissions, err := rs.roleStore.GetAuthorizedPermissionsByResourceServer(
		ctx, entityID, groups, roleIDs, assignmentOUID, resourceServerID, requestedPermissions)
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
	ctx context.Context, entityID string, groupIDs []string, assignmentOUID string,
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

	roles, err := rs.roleStore.GetUserRoles(ctx, entityID, groupIDs, assignmentOUID)
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

// roleAccessChangeTarget pairs the revoked permissions with every holder of the role; with none revoked
// it skips the holders, so a change that revokes nothing is never refused over them.
func (rs *roleService) roleAccessChangeTarget(ctx context.Context, id string,
	revoked []ResourcePermissions) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError) {
	scopes, svcErr := resolveScopes(ctx, rs.resourceService, revoked)
	if svcErr != nil {
		return nil, svcErr
	}
	if len(scopes) == 0 {
		return &revocation.AccessRevocationTarget{}, nil
	}
	entityIDs, svcErr := rs.roleAssigneeIDs(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	return &revocation.AccessRevocationTarget{EntityIDs: entityIDs, Scopes: scopes}, nil
}

// roleAssigneeIDs returns the distinct entities holding the role, directly or through a group.
func (rs *roleService) roleAssigneeIDs(ctx context.Context, id string) ([]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	total, err := rs.roleStore.GetRoleAssignmentsCount(ctx, id, "")
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		logger.Error(ctx, "Failed to get role assignments count", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	entityIDs := []string{}
	seen := map[string]struct{}{}
	for offset := 0; offset < total; offset += serverconst.MaxPageSize {
		assignments, err := rs.roleStore.GetRoleAssignments(ctx, id, "", serverconst.MaxPageSize, offset)
		if err != nil {
			if errors.Is(err, errResultLimitExceededInCompositeMode) {
				return nil, &ResultLimitExceededInCompositeMode
			}
			logger.Error(ctx, "Failed to get role assignments", log.String("id", id), log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		for _, assignment := range assignments {
			members, svcErr := expandPrincipal(ctx, rs.groupService, assignment.ID,
				assignment.Type == AssigneeTypeGroup)
			if svcErr != nil {
				return nil, svcErr
			}
			for _, entityID := range members {
				if _, ok := seen[entityID]; ok {
					continue
				}
				seen[entityID] = struct{}{}
				entityIDs = append(entityIDs, entityID)
			}
		}
	}
	return entityIDs, nil
}

// groupScopes returns the scopes membership of the group conveys, including through its ancestors.
func (rs *roleService) groupScopes(ctx context.Context, groupID string) (
	[]revocation.AudienceScope, *tidcommon.ServiceError) {
	ancestors, svcErr := rs.groupService.GetTransitiveAncestorGroups(ctx, groupID)
	if svcErr != nil {
		return nil, svcErr
	}
	permissionSet, svcErr := rs.GetAllPermissions(ctx, "", append([]string{groupID}, ancestors...))
	if svcErr != nil {
		return nil, svcErr
	}

	resourceServerIDs := slices.Sorted(maps.Keys(permissionSet))
	permissions := make([]ResourcePermissions, 0, len(resourceServerIDs))
	for _, resourceServerID := range resourceServerIDs {
		permissions = append(permissions, ResourcePermissions{
			ResourceServerID: resourceServerID,
			Permissions:      permissionSet[resourceServerID],
		})
	}
	return resolveScopes(ctx, rs.resourceService, permissions)
}

// validateRoleMutable refuses a change to a declarative role, failing closed when the check errors.
func (rs *roleService) validateRoleMutable(ctx context.Context, roleID string) *tidcommon.ServiceError {
	// Check the store mode - if it's mutable, no roles are declarative
	if getRoleStoreMode() == serverconst.StoreModeMutable {
		return nil
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	isDeclarative, err := rs.roleStore.IsRoleDeclarative(ctx, roleID)
	if err != nil {
		logger.Error(ctx, "Failed to check if role is declarative", log.String("roleID", roleID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if isDeclarative {
		logger.Debug(ctx, "Cannot modify declarative role", log.String("roleID", roleID))
		return &ErrorImmutableRole
	}
	return nil
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

// CreateSharingPolicy records which organization units a role is shared with.
func (rs *roleService) CreateSharingPolicy(
	ctx context.Context, roleID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, svcErr := rs.requireSharing(ctx, roleID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	// Roles that load from resource files only have all their sharing declared there.
	if isDeclarativeModeEnabled() {
		return sharing.Policy{}, &ErrorImmutableRole
	}
	// A declared role's own sharing is declared with it.
	if req.InitiatingOUID == "" || req.InitiatingOUID == owningOUID {
		if svcErr := rs.validateRoleMutable(ctx, roleID); svcErr != nil {
			return sharing.Policy{}, svcErr
		}
	}
	// The framework drops the caller's id and version on an interactive create, so a request
	// carrying either is answered as though it had not.
	policy, svcErr := rs.sharingService.CreatePolicy(ctx, RoleSharingType, roleID, owningOUID, req)
	return policy, translateSharingError(svcErr)
}

// GetSharingPolicy returns one of the role's sharing policies.
func (rs *roleService) GetSharingPolicy(
	ctx context.Context, roleID, policyID string,
) (sharing.Policy, *tidcommon.ServiceError) {
	return rs.requireOwnPolicy(ctx, roleID, policyID)
}

// ListSharingPolicies returns one page of the role's sharing policies.
func (rs *roleService) ListSharingPolicies(
	ctx context.Context, roleID string, limit, offset int,
) (sharing.PolicyList, *tidcommon.ServiceError) {
	if _, svcErr := rs.requireSharing(ctx, roleID); svcErr != nil {
		return sharing.PolicyList{}, svcErr
	}
	list, svcErr := rs.sharingService.GetPolicyList(ctx, RoleSharingType, roleID, limit, offset)
	return list, translateSharingError(svcErr)
}

// UpdateSharingPolicy replaces a sharing policy's contents.
func (rs *roleService) UpdateSharingPolicy(
	ctx context.Context, roleID, policyID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	if _, svcErr := rs.requireOwnPolicy(ctx, roleID, policyID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if isDeclarativeModeEnabled() {
		return sharing.Policy{}, &ErrorImmutableRole
	}
	policy, svcErr := rs.sharingService.UpdatePolicy(ctx, policyID, req)
	return policy, translateSharingError(svcErr)
}

// DeleteSharingPolicy removes a sharing policy and the reshares issued beneath it.
func (rs *roleService) DeleteSharingPolicy(
	ctx context.Context, roleID, policyID string,
) *tidcommon.ServiceError {
	policy, svcErr := rs.requireOwnPolicy(ctx, roleID, policyID)
	if svcErr != nil {
		if svcErr.Code == ErrorSharingPolicyNotFound.Code {
			return nil
		}
		return svcErr
	}
	if isDeclarativeModeEnabled() {
		return &ErrorImmutableRole
	}
	if policy.Declared {
		return &ErrorSharingPolicyDeclared
	}
	return translateSharingError(rs.sharingService.DeletePolicy(ctx, policyID))
}

// ResolveSharingOverlay answers whether one organization unit owns the role or is shared it, and
// which policies reached it.
func (rs *roleService) ResolveSharingOverlay(
	ctx context.Context, roleID, ouID string,
) (sharing.ResolvedOverlay, *tidcommon.ServiceError) {
	if _, svcErr := rs.requireSharing(ctx, roleID); svcErr != nil {
		return sharing.ResolvedOverlay{}, svcErr
	}
	if ouID == "" {
		return sharing.ResolvedOverlay{}, &ErrorMissingOUIDParameter
	}
	overlay, svcErr := rs.sharingService.ResolveOverlayRules(ctx, RoleSharingType, roleID, ouID)
	if svcErr != nil {
		// An id naming no organization unit leaves nothing to resolve either, and answering it as
		// a malformed policy would describe a mistake the caller did not make.
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return sharing.ResolvedOverlay{}, &ErrorRoleNotSharedToOU
		}
		return sharing.ResolvedOverlay{}, translateSharingError(svcErr)
	}
	// An organization unit the role never reached holds no terms to answer with.
	if !overlay.Visible {
		return sharing.ResolvedOverlay{}, &ErrorRoleNotSharedToOU
	}
	return overlay, nil
}

// requireSharing resolves the role a sharing call names and returns its owning organization unit,
// so an unknown role answers as one rather than as a sharing failure.
func (rs *roleService) requireSharing(
	ctx context.Context, roleID string,
) (string, *tidcommon.ServiceError) {
	if roleID == "" {
		return "", &ErrorMissingRoleID
	}
	role, svcErr := rs.GetRoleWithPermissions(ctx, roleID)
	if svcErr != nil {
		return "", svcErr
	}
	return role.OUID, nil
}

// requireOwnPolicy fetches a policy and refuses one belonging to another resource. Without it a
// policy id alone would reach any policy in the deployment through any role's path.
func (rs *roleService) requireOwnPolicy(
	ctx context.Context, roleID, policyID string,
) (sharing.Policy, *tidcommon.ServiceError) {
	if _, svcErr := rs.requireSharing(ctx, roleID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	policy, svcErr := rs.sharingService.GetPolicy(ctx, policyID)
	if svcErr != nil {
		return sharing.Policy{}, translateSharingError(svcErr)
	}
	if policy.ResourceType != RoleSharingType || policy.ResourceID != roleID {
		return sharing.Policy{}, &ErrorSharingPolicyNotFound
	}
	return policy, nil
}

// requireRoleMovable refuses to move a role from the organization unit owning it while that unit
// has shared it or assigned it: its policies were issued against that unit, and its assignees were
// checked against it.
func (rs *roleService) requireRoleMovable(
	ctx context.Context, roleID, fromOUID, toOUID string,
) *tidcommon.ServiceError {
	if fromOUID == toOUID {
		return nil
	}
	policies, svcErr := rs.sharingService.ListPolicies(ctx, RoleSharingType, roleID)
	if svcErr != nil {
		return translateSharingError(svcErr)
	}
	if len(policies) > 0 {
		return &ErrorSharedRoleCannotMove
	}
	assignments, err := rs.roleStore.GetRoleAssignmentsCount(ctx, roleID, "")
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return &ResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to count the assignments of the role being moved",
			log.String("id", roleID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if assignments > 0 {
		return &ErrorRoleWithAssignmentsCannotMove
	}
	return nil
}

// requireRoleNameAvailable refuses a name already taken in an organization unit, by a role the unit
// owns or one shared with it, since both are listed there. excludeID is the role being updated, which
// is never among the ones shared with the unit: the unit either owns it or is where it moves.
func (rs *roleService) requireRoleNameAvailable(
	ctx context.Context, ouID, name, excludeID string,
) *tidcommon.ServiceError {
	var owned bool
	var err error
	if excludeID == "" {
		owned, err = rs.roleStore.CheckRoleNameExists(ctx, ouID, name)
	} else {
		owned, err = rs.roleStore.CheckRoleNameExistsExcludingID(ctx, ouID, name, excludeID)
	}
	if err != nil {
		rs.logger.Error(ctx, "Failed to check role name existence", log.Error(err))
		return &tidcommon.InternalServerError
	}
	if owned {
		rs.logger.Debug(ctx, "Role name already exists in organization unit",
			log.String("name", name), log.String("ouID", ouID))
		return &ErrorRoleNameConflict
	}

	sharedIDs, svcErr := rs.sharingService.ListVisibleResourceIDs(ctx, RoleSharingType, ouID)
	if svcErr != nil {
		return translateSharingError(svcErr)
	}
	if len(sharedIDs) == 0 {
		return nil
	}
	shared, err := rs.roleStore.GetRolesByIDs(ctx, sharedIDs)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return &ResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to read the roles shared with the organization unit",
			log.String("ouID", ouID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	for _, r := range shared {
		if r.Name == name {
			rs.logger.Debug(ctx, "Role name taken by a role shared with the organization unit",
				log.String("name", name), log.String("ouID", ouID))
			return &ErrorRoleNameConflict
		}
	}
	return nil
}

// resolveUpdateTarget returns the organization unit owning a role, and whether the organization unit
// an update names is one the role is shared with rather than its owner or somewhere to move it.
func (rs *roleService) resolveUpdateTarget(
	ctx context.Context, roleID, ouID string,
) (string, bool, *tidcommon.ServiceError) {
	stored, err := rs.roleStore.GetRole(ctx, roleID)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			return "", false, &ErrorRoleNotFound
		}
		rs.logger.Error(ctx, "Failed to read the role being updated", log.String("id", roleID), log.Error(err))
		return "", false, &tidcommon.InternalServerError
	}
	if stored.OUID == ouID {
		return stored.OUID, false, nil
	}
	visible, svcErr := rs.sharingService.IsVisible(ctx, RoleSharingType, roleID, ouID)
	if svcErr != nil {
		// An organization unit that does not exist is not one the role is shared with. The move
		// that follows reports it.
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return stored.OUID, false, nil
		}
		return "", false, translateSharingError(svcErr)
	}
	return stored.OUID, visible, nil
}

// translateSharingError maps a sharing framework error into this service's own codes.
func translateSharingError(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	return sharingErrors.Translate(svcErr)
}

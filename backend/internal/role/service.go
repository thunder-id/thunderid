// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package role provides role management functionality.
package role

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	"github.com/thunder-id/thunderid/internal/system/projectscope"
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
	GetRoleListByProject(ctx context.Context, projectID string, limit, offset int,
		filters map[string]interface{}) (*RoleList, *tidcommon.ServiceError)
	// ProjectHasRoles reports whether any role, runtime or declared, belongs to the project.
	ProjectHasRoles(ctx context.Context, projectID string) (bool, *tidcommon.ServiceError)
	CreateRole(ctx context.Context, role RoleCreationDetail) (
		*RoleWithPermissionsAndAssignments, *tidcommon.ServiceError)
	GetRoleWithPermissions(ctx context.Context, id string) (*RoleWithPermissions, *tidcommon.ServiceError)
	GetRoleInOU(ctx context.Context, id, ouID string) (*RoleWithPermissions, *tidcommon.ServiceError)
	// GetRolesByIDs reads the roles with the given IDs, keyed by ID. An ID naming no role is left out.
	GetRolesByIDs(ctx context.Context, ids []string) (map[string]*Role, *tidcommon.ServiceError)
	GetRolesByNames(ctx context.Context, names []string, ouID string) (map[string][]*Role, *tidcommon.ServiceError)
	GetRolesByPermission(ctx context.Context, resourceServerID, permission string) ([]Role, *tidcommon.ServiceError)
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
	// ExpandHeldDescendants returns held descendants as alternatives to denied parent requests,
	// counting only the assignments made in assignmentOUID when one is given.
	ExpandHeldDescendants(
		ctx context.Context, entityID string, groups, roleIDs []string, assignmentOUID, resourceServerID string,
		permissions []string,
	) (map[string][]string, *tidcommon.ServiceError)
	// GetAllPermissions returns every permission the entity and/or groups hold, keyed by resource
	// server. Unlike GetAuthorizedPermissionsByResourceServer it enumerates rather than checks.
	GetAllPermissions(
		ctx context.Context, entityID string, groupIDs []string,
	) (security.PermissionSet, *tidcommon.ServiceError)
	// GetUserRoles returns the names of the roles assigned to the entity directly or through its
	// groups, counting only the assignments made in assignmentOUID when one is given, or else in the
	// sign-in boundary's organization unit when the context carries one.
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
	delimiters      *permissionDelimiterResolver
	roleStore       roleStoreInterface
	entityService   entity.EntityServiceInterface
	groupService    group.GroupServiceInterface
	ouService       oupkg.OrganizationUnitServiceInterface
	resourceService resourcepkg.ResourceServiceInterface
	transactioner   providers.Transactioner
	authzService    sysauthz.SystemAuthorizationServiceInterface
	sharingService  sharing.SharingServiceInterface
	bus             eventbus.EventBusInterface
	logger          *log.Logger
	// systemFlows is nil where system flows are off.
	systemFlows systemFlowProvider
	// projectValidator is nil on a deployment that serves no projects.
	projectValidator projectOUValidator
	// managed says which roles a control plane deployed, which stay read only here. Nil where nothing
	// is deployed from one.
	managed managedresource.StoreInterface
}

// projectOUValidator checks that the organization unit of a resource scoped to a project belongs to it.
type projectOUValidator interface {
	ValidateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError
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
	bus eventbus.EventBusInterface,
	projectValidator projectOUValidator,
) RoleServiceInterface {
	return &roleService{
		delimiters:       newPermissionDelimiterResolver(resourceService),
		roleStore:        roleStore,
		entityService:    entityService,
		groupService:     groupService,
		ouService:        ouService,
		resourceService:  resourceService,
		transactioner:    transactioner,
		authzService:     authzService,
		sharingService:   sharingService,
		bus:              bus,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)),
		projectValidator: projectValidator,
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
	ouID, visible, svcErr := sysauthz.ResolveListOU(ctx, rs.authzService, rs.logger, security.ActionListRoles, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	if !visible {
		return &RoleList{
			Roles:      []Role{},
			StartIndex: offset + 1,
			Links:      utils.BuildPaginationLinks("/roles", limit, offset, 0, utils.FilterQueryParam(filters)),
		}, nil
	}

	var roles []Role
	var totalCount int
	if ouID != "" {
		// With a filter, or without one for a caller confined to an organization unit, the list is what that
		// unit sees: the roles it owns, then the ones shared with it.
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

	rs.resolveRoleOUHandles(ctx, roles)
	rs.markManagedRoles(ctx, roles)

	response := &RoleList{
		TotalResults: totalCount,
		Roles:        roles,
		StartIndex:   offset + 1,
		Count:        len(roles),
		Links:        utils.BuildPaginationLinks("/roles", limit, offset, totalCount, utils.FilterQueryParam(filters)),
	}

	return response, nil
}

// GetRoleListByProject retrieves the roles of a project, narrowed to the ones an organization unit owns
// when the filter names one. A project that does not exist has none.
func (rs *roleService) GetRoleListByProject(
	ctx context.Context, projectID string, limit, offset int, filters map[string]interface{},
) (*RoleList, *tidcommon.ServiceError) {
	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}
	ouID, rest, err := filter.SplitOUIDFilter(filters)
	if err != nil || len(rest) > 0 {
		return nil, &ErrorInvalidFilter
	}

	extraQuery := "&" + projectIDQueryParam + "=" + url.QueryEscape(projectID) + utils.FilterQueryParam(filters)
	ouID, visible, svcErr := sysauthz.ResolveListOU(ctx, rs.authzService, rs.logger, security.ActionListRoles, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	if projectID == "" || !visible {
		return &RoleList{
			Roles:      []Role{},
			StartIndex: offset + 1,
			Links:      utils.BuildPaginationLinks("/roles", limit, offset, 0, extraQuery),
		}, nil
	}

	totalCount, err := rs.roleStore.GetRoleListCountByProject(ctx, projectID, ouID)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to count the roles of a project", log.String("projectID", projectID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	roles, err := rs.roleStore.GetRoleListByProject(ctx, projectID, ouID, limit, offset)
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		rs.logger.Error(ctx, "Failed to list the roles of a project", log.String("projectID", projectID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	for i := range roles {
		roles[i].IsOwnedByOU = true
	}
	rs.resolveRoleOUHandles(ctx, roles)
	rs.markManagedRoles(ctx, roles)

	return &RoleList{
		TotalResults: totalCount,
		Roles:        roles,
		StartIndex:   offset + 1,
		Count:        len(roles),
		Links:        utils.BuildPaginationLinks("/roles", limit, offset, totalCount, extraQuery),
	}, nil
}

// ProjectHasRoles reports whether any role, runtime or declared, belongs to the project. A project with more
// roles than the composite store can count has roles.
func (rs *roleService) ProjectHasRoles(ctx context.Context, projectID string) (bool, *tidcommon.ServiceError) {
	count, err := rs.roleStore.GetRoleListCountByProject(ctx, projectID, "")
	if errors.Is(err, errResultLimitExceededInCompositeMode) {
		return true, nil
	}
	if err != nil {
		rs.logger.Error(ctx, "Failed to count the roles of a project", log.String("projectID", projectID),
			log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	return count > 0, nil
}

// resolveRoleOUHandles fills in the handle of each listed role's organization unit. A failure to read the
// handles leaves them out rather than failing the listing.
func (rs *roleService) resolveRoleOUHandles(ctx context.Context, roles []Role) {
	if len(roles) == 0 {
		return
	}
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
		rs.logger.Warn(ctx, "Failed to resolve OU handles for roles, skipping", log.Any("error", svcErr))
		return
	}
	for i := range roles {
		roles[i].OUHandle = ouHandles[roles[i].OUID]
	}
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

// CreateRole creates a new role, assigned to the assignees it names.
func (rs *roleService) CreateRole(
	ctx context.Context, role RoleCreationDetail,
) (*RoleWithPermissionsAndAssignments, *tidcommon.ServiceError) {
	created, svcErr := rs.createRole(ctx, role)
	var roleID string
	if svcErr == nil {
		roleID = created.ID
	}
	rs.publishRoleEvent(ctx, audit.EventRoleCreate, audit.ActionCreate, roleID, svcErr)
	if svcErr == nil {
		publishAssignmentEvents(ctx, rs.bus, audit.EventRoleAssignmentAdd, audit.ActionAdd, roleID,
			created.Assignments, nil)
	}
	return created, svcErr
}

// createRole creates a new role without publishing its audit events.
func (rs *roleService) createRole(
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

	// A role that names no organization unit goes to the system organization unit.
	if role.OUID == "" {
		systemOUID, svcErr := rs.ouService.GetSystemOUID(ctx)
		if svcErr != nil {
			return nil, svcErr
		}
		role.OUID = systemOUID
	}
	if svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionCreateRole, role.OUID); svcErr != nil {
		return nil, svcErr
	}

	// A project checks that the role's organization unit belongs to it.
	if role.ProjectID != "" {
		if svcErr := rs.validateProjectOU(ctx, role.ProjectID, role.OUID); svcErr != nil {
			return nil, svcErr
		}
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
		ProjectID:   role.ProjectID,
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
	role, svcErr := rs.getRoleWithPermissions(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionReadRole, role.OUID); svcErr != nil {
		return nil, svcErr
	}
	return role, nil
}

// getRoleWithPermissions retrieves a role by its id without checking the caller.
func (rs *roleService) getRoleWithPermissions(ctx context.Context, id string) (
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
	role.IsReadOnly = rs.isReadOnlyRole(ctx, id, logger)

	logger.Debug(ctx, "Successfully retrieved role",
		log.String("id", role.ID), log.String("name", role.Name))
	return &role, nil
}

// GetRoleInOU retrieves a role as an organization unit sees it: the role itself in the organization
// unit that owns it, and the same role under an organization unit it is shared with. A role carries
// nothing a shared organization unit sets for itself: its assignments there are read separately.
func (rs *roleService) GetRoleInOU(ctx context.Context, id, ouID string) (
	*RoleWithPermissions, *tidcommon.ServiceError) {
	role, svcErr := rs.getRoleWithPermissions(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if ouID == "" || role.OUID == ouID {
		if svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionReadRole,
			role.OUID); svcErr != nil {
			if ouID == "" {
				return rs.getSharedRole(ctx, id, role.OUID, svcErr)
			}
			return nil, svcErr
		}
		return role, nil
	}
	if svcErr := sysauthz.CheckActionInAnyOU(ctx, rs.authzService, rs.logger, security.ActionReadRole,
		security.ResourceTypeRole, role.OUID, ouID); svcErr != nil {
		return nil, svcErr
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

// getSharedRole answers a read refused in the role's owning organization unit with the role as the organization
// unit the caller is confined to sees it, when the role is shared there, and with the refusal otherwise.
func (rs *roleService) getSharedRole(ctx context.Context, id, ownerOUID string,
	refusal *tidcommon.ServiceError) (*RoleWithPermissions, *tidcommon.ServiceError) {
	ouID := sysauthz.SharedAccessOU(ctx, rs.authzService, ownerOUID, refusal)
	if ouID == "" {
		return nil, refusal
	}
	role, svcErr := rs.GetRoleInOU(ctx, id, ouID)
	if svcErr != nil && svcErr.Code == ErrorRoleNotSharedToOU.Code {
		return nil, refusal
	}
	return role, svcErr
}

// sharedRoleRefusal answers a change refused in the role's owning organization unit with ErrorSharedRoleReadOnly
// when the role is shared into the organization unit the caller is confined to, and with the refusal otherwise.
func (rs *roleService) sharedRoleRefusal(ctx context.Context, id, ownerOUID string,
	refusal *tidcommon.ServiceError) *tidcommon.ServiceError {
	ouID, svcErr := sharing.SharedIntoCallerOU(ctx, rs.authzService, rs.sharingService, rs.logger,
		RoleSharingType, id, ownerOUID, refusal)
	if svcErr != nil {
		return svcErr
	}
	if ouID != "" {
		return &ErrorSharedRoleReadOnly
	}
	return refusal
}

// GetRolesByIDs retrieves the roles with the given IDs in one read, keyed by ID. An ID naming no role is left out.
func (rs *roleService) GetRolesByIDs(
	ctx context.Context, ids []string,
) (map[string]*Role, *tidcommon.ServiceError) {
	roles, err := rs.roleStore.GetRolesByIDs(ctx, utils.UniqueStrings(ids))
	if err != nil {
		rs.logger.Error(ctx, "Failed to get roles by IDs", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	result := make(map[string]*Role, len(roles))
	for i := range roles {
		result[roles[i].ID] = &roles[i]
	}
	return result, nil
}

// GetRolesByNames retrieves the roles with the given names that an organization unit owns or has shared
// with it, keyed by name. A name may resolve to more than one of them; the caller decides how to handle
// that ambiguity. An empty ouID matches roles in every organization unit.
func (rs *roleService) GetRolesByNames(
	ctx context.Context, names []string, ouID string,
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
	roles, svcErr := rs.readableRoles(ctx, roles)
	if svcErr != nil {
		return nil, svcErr
	}

	shared := make(map[string]bool)
	if ouID != "" && slices.ContainsFunc(roles, func(r Role) bool { return r.OUID != ouID }) {
		sharedIDs, svcErr := rs.sharingService.ListVisibleResourceIDs(ctx, RoleSharingType, ouID)
		// An organization unit that does not exist has nothing shared with it.
		if svcErr != nil && svcErr.Code != oupkg.ErrorOrganizationUnitNotFound.Code {
			return nil, translateSharingError(svcErr)
		}
		for _, id := range sharedIDs {
			shared[id] = true
		}
	}

	result := make(map[string][]*Role, len(roles))
	for i := range roles {
		if ouID == "" || roles[i].OUID == ouID || shared[roles[i].ID] {
			result[roles[i].Name] = append(result[roles[i].Name], &roles[i])
		}
	}

	return result, nil
}

// GetRolesByPermission returns the roles that grant a permission on a resource server, in every
// organization unit.
func (rs *roleService) GetRolesByPermission(
	ctx context.Context, resourceServerID, permission string,
) ([]Role, *tidcommon.ServiceError) {
	if resourceServerID == "" || permission == "" {
		return nil, &ErrorInvalidRequestFormat
	}

	roles, err := rs.roleStore.GetRolesByPermission(ctx, resourceServerID, permission)
	if err != nil {
		log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName)).
			Error(ctx, "Failed to get roles by permission", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	roles, svcErr := rs.readableRoles(ctx, roles)
	if svcErr != nil {
		return nil, svcErr
	}
	for i := range roles {
		roles[i].IsOwnedByOU = true
	}
	return roles, nil
}

// UpdateRoleWithPermissions updates an existing role.
func (rs *roleService) UpdateRoleWithPermissions(
	ctx context.Context, id string, role RoleUpdateDetail) (*RoleWithPermissions, *tidcommon.ServiceError) {
	updated, svcErr := rs.updateRoleWithPermissions(ctx, id, role)
	rs.publishRoleEvent(ctx, audit.EventRoleUpdate, audit.ActionUpdate, id, svcErr)
	return updated, svcErr
}

// updateRoleWithPermissions updates an existing role without publishing its audit event.
func (rs *roleService) updateRoleWithPermissions(
	ctx context.Context, id string, role RoleUpdateDetail) (*RoleWithPermissions, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Updating role", log.String("id", id), log.String("name", role.Name))

	if id == "" {
		return nil, &ErrorMissingRoleID
	}

	if err := rs.validateUpdateRoleRequest(role); err != nil {
		return nil, err
	}

	stored, err := rs.roleStore.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			return nil, &ErrorRoleNotFound
		}
		logger.Error(ctx, "Failed to read the role being updated", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionUpdateRole,
		stored.OUID); svcErr != nil {
		return nil, rs.sharedRoleRefusal(ctx, id, stored.OUID, svcErr)
	}
	// An update that names no organization unit keeps the role in its own, and one that names no project
	// keeps it in its project.
	if role.OUID == "" {
		role.OUID = stored.OUID
	}
	// Moving the role places it in another organization unit, which the caller must be allowed to act
	// for too.
	if role.OUID != stored.OUID {
		if svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionUpdateRole,
			role.OUID); svcErr != nil {
			return nil, svcErr
		}
	}
	if role.ProjectID == "" {
		role.ProjectID = stored.ProjectID
	}

	// A role's name, description and permissions stay with the organization unit that owns it, so an
	// update naming one it is shared with is refused. Any other organization unit is somewhere to
	// move the role to.
	ownerOUID, sharedWith, svcErr := rs.resolveUpdateTarget(ctx, stored, role.OUID)
	if svcErr != nil {
		return nil, svcErr
	}
	if sharedWith {
		return nil, &ErrorSharedRoleNotEditable
	}

	if svcErr := rs.authzService.CanGrantMembership(ctx, sysauthz.PrincipalTypeRole, id); svcErr != nil {
		return nil, svcErr
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

	// A project checks that the role's organization unit belongs to it. It is checked only when the update
	// changes the project or the organization unit, so a role whose project is gone can still be edited.
	if role.ProjectID != "" && (role.ProjectID != stored.ProjectID || role.OUID != stored.OUID) {
		if svcErr := rs.validateProjectOU(ctx, role.ProjectID, role.OUID); svcErr != nil {
			return nil, svcErr
		}
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

	if svcErr := rs.requireRoleMovable(
		ctx, id, ownerOUID, role.OUID, stored.ProjectID, role.ProjectID,
	); svcErr != nil {
		return nil, svcErr
	}

	// Dropping a permission revokes what it granted before the update is written, and again once it is.
	inputs, shared, svcErr := rs.revokeDroppedPermissions(ctx, id, stored.Permissions, role.Permissions, logger)
	if svcErr != nil {
		return nil, svcErr
	}

	err = rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		return rs.roleStore.UpdateRole(txCtx, id, role)
	})

	if err != nil {
		logger.Error(ctx, "Failed to update role", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if inputs != nil {
		if _, svcErr := runSystemFlow(ctx, rs.systemFlows, systemFlowRevocationPost, inputs, shared); svcErr != nil {
			return nil, svcErr
		}
	}

	logger.Debug(ctx, "Successfully updated role", log.String("id", id), log.String("name", role.Name))
	return &RoleWithPermissions{
		ID:          id,
		Name:        role.Name,
		Description: role.Description,
		OUID:        role.OUID,
		OUHandle:    ou.Handle,
		ProjectID:   role.ProjectID,
		IsOwnedByOU: true,
		Permissions: role.Permissions,
	}, nil
}

// DeleteRole delete the specified role by its id.
func (rs *roleService) DeleteRole(ctx context.Context, id string) *tidcommon.ServiceError {
	svcErr := rs.deleteRole(ctx, id)
	// A delete that found nothing to remove succeeds without an event, so no caller can record a
	// deletion of an ID it made up.
	if svcErr == errRoleNotFoundNoop {
		return nil
	}
	rs.publishRoleEvent(ctx, audit.EventRoleDelete, audit.ActionDelete, id, svcErr)
	return svcErr
}

// deleteRole deletes the role and its assignments without publishing its audit event.
func (rs *roleService) deleteRole(ctx context.Context, id string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	logger.Debug(ctx, "Deleting role", log.String("id", id))

	if id == "" {
		return &ErrorMissingRoleID
	}

	existing, err := rs.roleStore.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			logger.Debug(ctx, "Role not found", log.String("id", id))
			return errRoleNotFoundNoop
		}
		logger.Error(ctx, "Failed to check role existence", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionDeleteRole,
		existing.OUID); svcErr != nil {
		return rs.sharedRoleRefusal(ctx, id, existing.OUID, svcErr)
	}

	if svcErr := rs.authzService.CanGrantMembership(ctx, sysauthz.PrincipalTypeRole, id); svcErr != nil {
		return svcErr
	}

	if svcErr := rs.validateRoleMutable(ctx, id); svcErr != nil {
		return svcErr
	}

	// Revokes what the role grants its assignees before anything of it is removed. A refusal leaves the
	// role as it was.
	inputs := map[string]string{systemFlowInputRole: id}
	shared, svcErr := runSystemFlow(ctx, rs.systemFlows, systemFlowRoleDeletePre, inputs, nil)
	if svcErr != nil {
		return svcErr
	}

	var capturedSvcErr, refusal *tidcommon.ServiceError
	err = rs.transactioner.Transact(ctx, func(txCtx context.Context) error {
		if refusal = rs.requireRoleNotMandated(txCtx, id); refusal != nil {
			return errors.New("rollback for a mandated role")
		}

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
	if refusal != nil {
		return refusal
	}
	if capturedSvcErr != nil {
		return translateSharingError(capturedSvcErr)
	}
	if err != nil {
		logger.Error(ctx, "Failed to delete role", log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	// A token issued to an assignee while the role was being removed is revoked too.
	if _, svcErr := runSystemFlow(ctx, rs.systemFlows, systemFlowRevocationPost, inputs, shared); svcErr != nil {
		return svcErr
	}

	logger.Debug(ctx, "Successfully deleted role", log.String("id", id))
	return nil
}

// publishRoleEvent publishes the audit event of an attempt on the role with id, after the change
// committed or was refused.
func (rs *roleService) publishRoleEvent(ctx context.Context, name string, action audit.Action, id string,
	svcErr *tidcommon.ServiceError) {
	target := audit.Ref{Type: audit.TargetTypeRole, ID: id}
	eventbus.TryPublish(ctx, rs.bus,
		eventbus.NewEvent(ctx, name).With(audit.ChangePayload(ctx, target, action, svcErr)))
}

// ValidateDeleteRole reports whether the role may be deleted and returns what the deletion revokes.
func (rs *roleService) ValidateDeleteRole(ctx context.Context, id string) (
	*revocation.AccessRevocationTarget, *tidcommon.ServiceError) {
	role, svcErr := rs.getRoleWithPermissions(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionDeleteRole,
		role.OUID); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.validateRoleMutable(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.authzService.CanGrantMembership(ctx, sysauthz.PrincipalTypeRole, id); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.requireRoleNotMandated(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	revoked, svcErr := revokedPermissions(ctx, rs.resourceService, role.Permissions, nil)
	if svcErr != nil {
		return nil, svcErr
	}
	return rs.roleAccessChangeTarget(ctx, id, revoked)
}

// requireRoleNotMandated refuses to delete a role a user sharing policy still mandates.
func (rs *roleService) requireRoleNotMandated(ctx context.Context, id string) *tidcommon.ServiceError {
	mandated, err := rs.roleStore.ListMandatedAssignmentsForRole(ctx, id, "")
	if err != nil {
		rs.logger.Error(ctx, "Failed to list the mandated assignments of the role",
			log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if len(mandated) > 0 {
		return &ErrorMandatedRoleCannotBeDeleted
	}
	return nil
}

// ValidateUpdateRolePermissions runs the checks UpdateRoleWithPermissions runs on the new permissions.
func (rs *roleService) ValidateUpdateRolePermissions(ctx context.Context, id string,
	permissions []ResourcePermissions) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError) {
	role, svcErr := rs.getRoleWithPermissions(ctx, id)
	if svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.validateRoleMutable(ctx, id); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.authzService.CanGrantMembership(ctx, sysauthz.PrincipalTypeRole, id); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.validatePermissions(ctx, permissions); svcErr != nil {
		return nil, svcErr
	}
	if svcErr := rs.authzService.CanGrantPermissions(ctx, toPermissionSet(permissions)); svcErr != nil {
		return nil, svcErr
	}
	revoked, svcErr := revokedPermissions(ctx, rs.resourceService, role.Permissions, permissions)
	if svcErr != nil {
		return nil, svcErr
	}
	return rs.roleAccessChangeTarget(ctx, id, revoked)
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
	// A group that grants nothing revokes nothing, so its departing member is not expanded.
	if len(scopes) == 0 {
		return &revocation.AccessRevocationTarget{AssigneeType: string(member.Type)}, nil
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

	// Include ancestors before the store's indexed exact lookup.
	delimiter := rs.delimiters.resolve(ctx, resourceServerID, "")
	candidates, coverage := expandRequestedPermissions(requestedPermissions, delimiter)

	// Get authorized permissions from store
	authorizedCandidates, err := rs.roleStore.GetAuthorizedPermissionsByResourceServer(
		ctx, entityID, groups, roleIDs, assignmentOUID, resourceServerID, candidates)
	if err != nil {
		logger.Error(ctx, "Failed to get authorized permissions",
			log.MaskedString(log.LoggerKeyUserID, entityID),
			log.Int("groupCount", len(groups)),
			log.String("resourceServerID", resourceServerID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	// Map matched ancestors back to the requested permissions.
	authorizedPermissions := resolveCoveredPermissions(
		requestedPermissions, authorizedCandidates, coverage)

	// Inheritance must not authorize permission strings absent from the catalog.
	authorizedPermissions = rs.dropUndeclaredPermissions(
		ctx, resourceServerID, authorizedPermissions, authorizedCandidates, delimiter)

	logger.Debug(ctx, "Retrieved authorized permissions",
		log.MaskedString(log.LoggerKeyUserID, entityID),
		log.Int("groupCount", len(groups)),
		log.String("resourceServerID", resourceServerID),
		log.Int("requestedCount", len(requestedPermissions)),
		log.Int("authorizedCount", len(authorizedPermissions)))

	return authorizedPermissions, nil
}

// dropUndeclaredPermissions validates scopes authorized only through an ancestor.
// Unreadable catalogs exclude inherited scopes; direct grants retain their existing behavior.
func (rs *roleService) dropUndeclaredPermissions(
	ctx context.Context, resourceServerID string,
	authorizedPermissions, matchedByName []string, delimiter string,
) []string {
	if delimiter == "" || len(authorizedPermissions) == 0 {
		return authorizedPermissions
	}

	named := make(map[string]struct{}, len(matchedByName))
	for _, permission := range matchedByName {
		named[permission] = struct{}{}
	}

	inherited := make([]string, 0, len(authorizedPermissions))
	for _, permission := range authorizedPermissions {
		if _, ok := named[permission]; !ok {
			inherited = append(inherited, permission)
		}
	}
	if len(inherited) == 0 {
		return authorizedPermissions
	}

	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	undeclared, svcErr := rs.resourceService.ValidatePermissions(ctx, resourceServerID, inherited)
	if svcErr != nil {
		logger.Error(ctx, "Could not check inherited permissions against the catalog; refusing them",
			log.String("resourceServerID", resourceServerID),
			log.Int("inheritedCount", len(inherited)))
		undeclared = inherited
	}
	if len(undeclared) == 0 {
		return authorizedPermissions
	}

	refused := make(map[string]struct{}, len(undeclared))
	for _, permission := range undeclared {
		refused[permission] = struct{}{}
	}
	logger.Debug(ctx, "Refusing permissions the resource server does not declare",
		log.String("resourceServerID", resourceServerID),
		log.Int("refusedCount", len(refused)))

	declared := make([]string, 0, len(authorizedPermissions))
	for _, permission := range authorizedPermissions {
		if _, ok := refused[permission]; !ok {
			declared = append(declared, permission)
		}
	}
	return declared
}

// expandRequestedPermissions returns exact/ancestor lookup candidates and their requested-scope coverage.
func expandRequestedPermissions(
	requestedPermissions []string, delimiter string,
) ([]string, map[string][]string) {
	candidates := make([]string, 0, len(requestedPermissions))
	coverage := make(map[string][]string, len(requestedPermissions))

	for _, requested := range requestedPermissions {
		if requested == "" {
			continue
		}
		for _, candidate := range append(
			[]string{requested}, security.AncestorPermissions(requested, delimiter)...,
		) {
			if _, seen := coverage[candidate]; !seen {
				candidates = append(candidates, candidate)
			}
			coverage[candidate] = append(coverage[candidate], requested)
		}
	}

	return candidates, coverage
}

// resolveCoveredPermissions returns the requested permissions that at least one authorized candidate
// covers, in the order they were requested.
func resolveCoveredPermissions(
	requestedPermissions, authorizedCandidates []string, coverage map[string][]string,
) []string {
	authorized := make(map[string]bool, len(requestedPermissions))
	for _, candidate := range authorizedCandidates {
		for _, requested := range coverage[candidate] {
			authorized[requested] = true
		}
	}

	covered := make([]string, 0, len(requestedPermissions))
	for _, requested := range requestedPermissions {
		if authorized[requested] {
			covered = append(covered, requested)
			// Guard against a requested permission listed twice yielding it twice.
			authorized[requested] = false
		}
	}

	return covered
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
// When assignmentOUID is empty and the context carries a sign-in boundary, only the roles assigned in the
// boundary's organization unit are returned.
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

	if boundary, bounded := authnprovidercm.SignInBoundaryFrom(ctx); bounded && assignmentOUID == "" {
		assignmentOUID = boundary.OUID
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
				log.String("resourceServerID", resPerm.ResourceServerID),
				log.String("error", svcErr.Error.DefaultValue))
			return &tidcommon.InternalServerError
		}

		// If any permissions are invalid, return error
		if len(invalidPerms) > 0 {
			logger.Debug(ctx, "Invalid permissions found",
				log.String("resourceServerID", resPerm.ResourceServerID),
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
	scopes []revocation.AudienceScope) (*revocation.AccessRevocationTarget, *tidcommon.ServiceError) {
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
	return revokedPermissions(ctx, rs.resourceService, permissions, nil)
}

// validateRoleMutable refuses a change to a declarative role, failing closed when the check errors.
func (rs *roleService) validateRoleMutable(ctx context.Context, roleID string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	// In mutable store mode no role is declarative.
	if getRoleStoreMode() != serverconst.StoreModeMutable {
		isDeclarative, err := rs.roleStore.IsRoleDeclarative(ctx, roleID)
		if err != nil {
			logger.Error(ctx, "Failed to check if role is declarative", log.String("roleID", roleID), log.Error(err))
			return &tidcommon.InternalServerError
		}
		if isDeclarative {
			logger.Debug(ctx, "Cannot modify declarative role", log.String("roleID", roleID))
			return &ErrorImmutableRole
		}
	}

	// Checked whatever the store mode: a role a control plane deployed lives in the database.
	return rs.refuseManagedRole(ctx, roleID)
}

// refuseManagedRole refuses a change to a role a control plane deployed, unless the change is the managed
// import's, failing closed when the check errors.
func (rs *roleService) refuseManagedRole(ctx context.Context, roleID string) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	managed, err := managedresource.IsReadOnly(ctx, rs.managed, roleID, managedresource.TypeRole)
	if err != nil {
		logger.Error(ctx, "Failed to read whether a control plane deployed the role",
			log.String("roleID", roleID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if managed {
		logger.Debug(ctx, "Cannot modify a role a control plane deployed", log.String("roleID", roleID))
		return &ErrorManagedRole
	}
	return nil
}

// setManagedStore gives the role service the record of what a control plane deployed.
func setManagedStore(service RoleServiceInterface, managed managedresource.StoreInterface) {
	if rs, ok := service.(*roleService); ok {
		rs.managed = managed
	}
}

// isReadOnlyRole reports whether a control plane deployed the role, which is shown as read only. A failed
// read of that is logged and taken as not: every write to such a role is refused whatever the read says.
func (rs *roleService) isReadOnlyRole(ctx context.Context, roleID string, logger *log.Logger) bool {
	deployed, err := managedresource.IsDeployed(ctx, rs.managed, roleID, managedresource.TypeRole)
	if err != nil {
		logger.Error(ctx, "Failed to read whether a control plane deployed the role",
			log.String("roleID", roleID), log.Error(err))
	}
	return deployed
}

// markManagedRoles reports the listed roles a control plane deployed as read only.
func (rs *roleService) markManagedRoles(ctx context.Context, roles []Role) {
	if err := managedresource.MarkDeployed(ctx, rs.managed, roles,
		func(r *Role) string { return r.ID }, func(r *Role) { r.IsReadOnly = true },
		managedresource.TypeRole); err != nil {
		// Every write to such a role is refused whatever the list says, so it is served without the flag.
		rs.logger.Error(ctx, "Failed to read which roles a control plane deployed", log.Error(err))
	}
}

// GetResourceDependencies implements resourcedependency.Provider. It reports the roles an organization unit
// owns, which block its deletion. Role permissions are cleaned up via cascade rather than surfaced as
// blocking usages, so nothing is reported for other resource types. The number of roles scanned is bounded
// by MaxCompositeStoreRecords.
func (rs *roleService) GetResourceDependencies(
	ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error) {
	if resourceType != resourcedependency.ResourceTypeOU {
		return []resourcedependency.ResourceDependency{}, nil
	}
	roles, err := rs.roleStore.GetRoleListByOUID(ctx, id, serverconst.MaxCompositeStoreRecords, 0)
	if err != nil {
		return nil, err
	}
	deps := make([]resourcedependency.ResourceDependency, 0, len(roles))
	for _, r := range roles {
		deps = append(deps, resourcedependency.ResourceDependency{
			ResourceType:     resourcedependency.ResourceTypeRole,
			ID:               r.ID,
			DisplayName:      r.Name,
			BehaviorOnDelete: resourcedependency.BehaviorRestrict,
		})
	}
	return deps, nil
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

// CreateSharingPolicy records which organization units a role is shared with. A policy is the
// initiating organization unit's decision, so that unit is the one the caller must be allowed to act
// for.
func (rs *roleService) CreateSharingPolicy(
	ctx context.Context, roleID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, svcErr := rs.requireSharing(ctx, roleID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	initiatingOUID := req.InitiatingOUID
	if initiatingOUID == "" {
		initiatingOUID = owningOUID
	}
	if svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionUpdateRole,
		initiatingOUID); svcErr != nil {
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

// GetSharingPolicy returns one of the role's sharing policies. The owner sees every decision about its
// role; any other organization unit sees its own.
func (rs *roleService) GetSharingPolicy(
	ctx context.Context, roleID, policyID string,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, policy, svcErr := rs.requireOwnPolicy(ctx, roleID, policyID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if svcErr := sysauthz.CheckActionInAnyOU(ctx, rs.authzService, rs.logger, security.ActionReadRole,
		security.ResourceTypeRole, owningOUID, policy.InitiatingOUID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	return policy, nil
}

// ListSharingPolicies returns one page of the role's sharing policies. A caller who may read roles in
// the owning organization unit sees them all; any other caller sees the ones its own organization
// units made.
func (rs *roleService) ListSharingPolicies(
	ctx context.Context, roleID string, limit, offset int,
) (sharing.PolicyList, *tidcommon.ServiceError) {
	owningOUID, svcErr := rs.requireSharing(ctx, roleID)
	if svcErr != nil {
		return sharing.PolicyList{}, svcErr
	}
	return sharing.ListReadablePolicies(ctx, rs.sharingService, RoleSharingType, roleID, owningOUID, limit,
		offset, func(ouID string) *tidcommon.ServiceError {
			return checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionReadRole, ouID)
		}, translateSharingError)
}

// UpdateSharingPolicy replaces a sharing policy's contents. Only the organization unit whose decision
// it is may change it.
func (rs *roleService) UpdateSharingPolicy(
	ctx context.Context, roleID, policyID string, req sharing.PolicyRequest,
) (sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, policy, svcErr := rs.requireOwnPolicy(ctx, roleID, policyID)
	if svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionUpdateRole,
		policy.InitiatingOUID); svcErr != nil {
		return sharing.Policy{}, svcErr
	}
	if isDeclarativeModeEnabled() {
		return sharing.Policy{}, &ErrorImmutableRole
	}
	// A deployed role's own sharing is deployed with it.
	if policy.InitiatingOUID == owningOUID {
		if svcErr := rs.refuseManagedRole(ctx, roleID); svcErr != nil {
			return sharing.Policy{}, svcErr
		}
	}
	updated, svcErr := rs.sharingService.UpdatePolicy(ctx, policyID, req)
	return updated, translateSharingError(svcErr)
}

// DeleteSharingPolicy removes a sharing policy and the reshares issued beneath it.
func (rs *roleService) DeleteSharingPolicy(
	ctx context.Context, roleID, policyID string,
) *tidcommon.ServiceError {
	owningOUID, policy, svcErr := rs.requireOwnPolicy(ctx, roleID, policyID)
	if svcErr != nil {
		if svcErr.Code == ErrorSharingPolicyNotFound.Code {
			return nil
		}
		return svcErr
	}
	if svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionUpdateRole,
		policy.InitiatingOUID); svcErr != nil {
		return svcErr
	}
	if isDeclarativeModeEnabled() {
		return &ErrorImmutableRole
	}
	// A deployed role's own sharing is deployed with it.
	if policy.InitiatingOUID == owningOUID {
		if svcErr := rs.refuseManagedRole(ctx, roleID); svcErr != nil {
			return svcErr
		}
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
	owningOUID, svcErr := rs.requireSharing(ctx, roleID)
	if svcErr != nil {
		return sharing.ResolvedOverlay{}, svcErr
	}
	if ouID == "" {
		return sharing.ResolvedOverlay{}, &ErrorMissingOUIDParameter
	}
	if svcErr := sysauthz.CheckActionInAnyOU(ctx, rs.authzService, rs.logger, security.ActionReadRole,
		security.ResourceTypeRole, owningOUID, ouID); svcErr != nil {
		return sharing.ResolvedOverlay{}, svcErr
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
	role, svcErr := rs.getRoleWithPermissions(ctx, roleID)
	if svcErr != nil {
		return "", svcErr
	}
	return role.OUID, nil
}

// requireOwnPolicy fetches a policy and refuses one belonging to another resource. Without it a
// policy id alone would reach any policy in the deployment through any role's path.
func (rs *roleService) requireOwnPolicy(
	ctx context.Context, roleID, policyID string,
) (string, sharing.Policy, *tidcommon.ServiceError) {
	owningOUID, svcErr := rs.requireSharing(ctx, roleID)
	if svcErr != nil {
		return "", sharing.Policy{}, svcErr
	}
	policy, svcErr := rs.sharingService.GetPolicy(ctx, policyID)
	if svcErr != nil {
		return "", sharing.Policy{}, translateSharingError(svcErr)
	}
	if policy.ResourceType != RoleSharingType || policy.ResourceID != roleID {
		return "", sharing.Policy{}, &ErrorSharingPolicyNotFound
	}
	return owningOUID, policy, nil
}

// requireRoleMovable refuses to move a shared role to another organization unit or project, and an
// assigned role to another organization unit: its policies were issued against its unit and project,
// and its assignees were checked against its unit.
func (rs *roleService) requireRoleMovable(
	ctx context.Context, roleID, fromOUID, toOUID, fromProjectID, toProjectID string,
) *tidcommon.ServiceError {
	if fromOUID == toOUID && fromProjectID == toProjectID {
		return nil
	}
	policies, svcErr := rs.sharingService.ListPolicies(ctx, RoleSharingType, roleID)
	if svcErr != nil {
		return translateSharingError(svcErr)
	}
	if len(policies) > 0 {
		return &ErrorSharedRoleCannotMove
	}
	if fromOUID == toOUID {
		return nil
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
	ctx context.Context, stored RoleWithPermissions, ouID string,
) (string, bool, *tidcommon.ServiceError) {
	if stored.OUID == ouID {
		return stored.OUID, false, nil
	}
	visible, svcErr := rs.sharingService.IsVisible(ctx, RoleSharingType, stored.ID, ouID)
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

// translateSharingError maps a sharing framework error into this service's own codes. A refusal this
// package raised through the framework is already one of them.
func translateSharingError(svcErr *tidcommon.ServiceError) *tidcommon.ServiceError {
	if svcErr != nil && svcErr.Code == ErrorRoleMandatedInOU.Code {
		return svcErr
	}
	return sharingErrors.Translate(svcErr)
}

// validateProjectOU checks that a role's organization unit belongs to its project. It returns
// ErrorInvalidProject when no such project exists, and ErrorOUNotInProject when the organization unit is
// outside the project.
//
// A deployment that serves no projects, such as a Data Plane, cannot check the project. It keeps the
// project ID and organization unit as given so that imported configuration retains them.
func (rs *roleService) validateProjectOU(ctx context.Context, projectID, ouID string) *tidcommon.ServiceError {
	if rs.projectValidator == nil {
		return nil
	}
	svcErr := rs.projectValidator.ValidateProjectOU(ctx, projectID, ouID)
	switch {
	case svcErr == nil:
		return nil
	case svcErr.Code == projectscope.ProjectNotFoundErrorCode:
		return &ErrorInvalidProject
	case svcErr.Code == projectscope.OUNotInProjectErrorCode:
		return &ErrorOUNotInProject
	default:
		return &tidcommon.InternalServerError
	}
}

// revokeDroppedPermissions runs the permission removal pre flow when the update drops one of the role's
// current permissions, and returns the inputs it ran with and what it shared, for the post flow to run
// once the update is written. An update that drops nothing, or a service with system flows off, runs no
// flow and returns no inputs. A refusal leaves the role as it was.
func (rs *roleService) revokeDroppedPermissions(ctx context.Context, id string,
	current, permissions []ResourcePermissions, logger *log.Logger) (map[string]string, map[string]string,
	*tidcommon.ServiceError) {
	if rs.systemFlows == nil {
		return nil, nil, nil
	}
	if len(droppedPermissions(current, permissions)) == 0 {
		return nil, nil, nil
	}
	if permissions == nil {
		permissions = []ResourcePermissions{}
	}
	encoded, err := json.Marshal(permissions)
	if err != nil {
		logger.Error(ctx, "Failed to encode role permissions", log.String("id", id), log.Error(err))
		return nil, nil, &tidcommon.InternalServerError
	}
	inputs := map[string]string{systemFlowInputRole: id, systemFlowInputPermissions: string(encoded)}
	shared, svcErr := runSystemFlow(ctx, rs.systemFlows, systemFlowRolePermissionRemovalPre, inputs, nil)
	if svcErr != nil {
		return nil, nil, svcErr
	}
	return inputs, shared, nil
}

// readableRoles keeps the roles the caller may read.
func (rs *roleService) readableRoles(ctx context.Context, roles []Role) ([]Role, *tidcommon.ServiceError) {
	readable := make([]Role, 0, len(roles))
	for _, role := range roles {
		svcErr := checkRoleAccess(ctx, rs.authzService, rs.logger, security.ActionReadRole, role.OUID)
		if svcErr == nil {
			readable = append(readable, role)
			continue
		}
		if svcErr.Code != tidcommon.ErrorUnauthorized.Code {
			return nil, svcErr
		}
	}
	return readable, nil
}

// ExpandHeldDescendants returns held descendants of refused permissions without granting their parents.
// Only the assignments made in assignmentOUID count when one is given, matching
// GetAuthorizedPermissionsByResourceServer.
func (rs *roleService) ExpandHeldDescendants(
	ctx context.Context, entityID string, groups, roleIDs []string, assignmentOUID, resourceServerID string,
	permissions []string,
) (map[string][]string, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))

	delimiter := rs.delimiters.resolve(ctx, resourceServerID, "")
	if delimiter == "" || len(permissions) == 0 {
		return nil, nil
	}

	held, err := rs.roleStore.GetHeldPermissionsByResourceServer(
		ctx, entityID, groups, roleIDs, assignmentOUID, resourceServerID)
	if err != nil {
		logger.Error(ctx, "Failed to enumerate held permissions",
			log.MaskedString(log.LoggerKeyUserID, entityID),
			log.String("resourceServerID", resourceServerID),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	held = rs.declaredHeldDescendants(ctx, resourceServerID, held, permissions, delimiter)

	expanded := make(map[string][]string, len(permissions))
	for _, permission := range permissions {
		prefix := permission + delimiter
		descendants := make([]string, 0, len(held))
		for _, candidate := range held {
			if strings.HasPrefix(candidate, prefix) {
				descendants = append(descendants, candidate)
			}
		}
		if len(descendants) > 0 {
			slices.Sort(descendants)
			expanded[permission] = descendants
		}
	}

	if len(expanded) == 0 {
		return nil, nil
	}
	return expanded, nil
}

// declaredHeldDescendants checks replacement permissions against the catalog in one batch.
func (rs *roleService) declaredHeldDescendants(
	ctx context.Context, resourceServerID string, held, permissions []string, delimiter string,
) []string {
	candidates := make([]string, 0, len(held))
	for _, candidate := range held {
		if slices.ContainsFunc(permissions, func(permission string) bool {
			return strings.HasPrefix(candidate, permission+delimiter)
		}) {
			candidates = append(candidates, candidate)
		}
	}
	slices.Sort(candidates)
	candidates = slices.Compact(candidates)

	// Built-in System permissions are defined in code rather than in the catalog.
	var systemPermissions []string
	if resourceServerID == serverconst.SystemResourceServerID {
		if p := security.GetSystemPermissions(); p != nil {
			systemPermissions = []string{
				p.Root, p.OU, p.OUView, p.User, p.UserView, p.Group, p.GroupView,
				p.Agent, p.AgentView, p.UserType, p.UserTypeView, p.AgentType, p.AgentTypeView,
				p.NotificationTemplate, p.NotificationTemplateView,
			}
		}
	}
	return rs.dropUndeclaredPermissions(ctx, resourceServerID, candidates, systemPermissions, delimiter)
}

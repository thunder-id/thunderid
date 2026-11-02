// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/sharing"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/filter"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

const assignmentLoggerComponentName = "RoleAssignmentService"

// RoleAssignmentServiceInterface defines the interface for role assignment operations.
//
// Reads take the organization unit the assignments were made in, and an empty one reads them all.
// Writes take the organization unit to make them in, and an empty one means the role's owner.
type RoleAssignmentServiceInterface interface {
	GetRoleAssignments(ctx context.Context, id, ouID string, limit, offset int,
		includeDisplay bool) (*AssignmentList, *tidcommon.ServiceError)
	GetRoleAssignmentsByType(ctx context.Context, id, ouID string, limit, offset int,
		includeDisplay bool, assigneeType string) (*AssignmentList, *tidcommon.ServiceError)
	AddAssignments(ctx context.Context, id, ouID string, assignments []RoleAssignment) *tidcommon.ServiceError
	RemoveAssignments(ctx context.Context, id, ouID string, assignments []RoleAssignment) *tidcommon.ServiceError
	// ValidateRemoveAssignment reports whether the assignee may be unassigned in the organization unit
	// named in ouID, or in the role's owner when it is empty, and what that revokes.
	ValidateRemoveAssignment(ctx context.Context, id, ouID, assigneeID string) (
		*revocation.AccessRevocationTarget, *tidcommon.ServiceError)
	// AddAssigneesToRoles assigns each assignee to the roles in the organization unit it names, or in
	// the one owning it when it names none. All are added or none is.
	AddAssigneesToRoles(ctx context.Context, assignments []RoleAssignment,
		roleIDs []string) *tidcommon.ServiceError
	// DeleteAssignmentsInOU removes every assignment of the role made in the organization unit.
	DeleteAssignmentsInOU(ctx context.Context, id, ouID string) error
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	CascadeDeleteDependencies(ctx context.Context, resourceType, id string) (int, error)
}

// roleAssignmentService is the default implementation of RoleAssignmentServiceInterface.
type roleAssignmentService struct {
	roleStore         roleStoreInterface
	entityService     entity.EntityServiceInterface
	groupService      group.GroupServiceInterface
	entityTypeService entitytype.EntityTypeServiceInterface
	resourceService   resourcepkg.ResourceServiceInterface
	transactioner     providers.Transactioner
	authzService      sysauthz.SystemAuthorizationServiceInterface
	sharingService    sharing.SharingServiceInterface
	logger            *log.Logger
}

// newRoleAssignmentService creates a new instance of roleAssignmentService.
func newRoleAssignmentService(
	roleStore roleStoreInterface,
	entityService entity.EntityServiceInterface,
	groupService group.GroupServiceInterface,
	entityTypeService entitytype.EntityTypeServiceInterface,
	resourceService resourcepkg.ResourceServiceInterface,
	transactioner providers.Transactioner,
	authzService sysauthz.SystemAuthorizationServiceInterface,
	sharingService sharing.SharingServiceInterface,
) RoleAssignmentServiceInterface {
	return &roleAssignmentService{
		roleStore:         roleStore,
		entityService:     entityService,
		groupService:      groupService,
		entityTypeService: entityTypeService,
		resourceService:   resourceService,
		transactioner:     transactioner,
		authzService:      authzService,
		sharingService:    sharingService,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName)),
	}
}

// GetRoleAssignments retrieves assignments for a role with pagination.
func (as *roleAssignmentService) GetRoleAssignments(ctx context.Context, id, ouID string, limit, offset int,
	includeDisplay bool) (*AssignmentList, *tidcommon.ServiceError) {
	return as.GetRoleAssignmentsByType(ctx, id, ouID, limit, offset, includeDisplay, "")
}

// GetRoleAssignmentsByType retrieves assignments for a role filtered by assignee type with pagination.
func (as *roleAssignmentService) GetRoleAssignmentsByType(ctx context.Context, id, ouID string, limit, offset int,
	includeDisplay bool, assigneeType string) (*AssignmentList, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName))

	if err := validatePaginationParams(limit, offset); err != nil {
		return nil, err
	}

	if id == "" {
		return nil, &ErrorMissingRoleID
	}

	exists, err := as.roleStore.IsRoleExist(ctx, id)
	if err != nil {
		logger.Error(ctx, "Failed to check role existence", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if !exists {
		logger.Debug(ctx, "Role not found", log.String("id", id))
		return nil, &ErrorRoleNotFound
	}

	// user/app/agent filters require fetching all entity assignments and post-filtering by category.
	if assigneeType == string(providers.EntityCategoryUser) ||
		assigneeType == string(providers.EntityCategoryApp) ||
		assigneeType == string(providers.EntityCategoryAgent) {
		return as.getAssignmentsByEntityCategory(ctx, id, ouID, limit, offset, includeDisplay, assigneeType, logger)
	}

	// For no filter or 'group' filter, use DB-level pagination directly.
	var totalCount int
	var assignments []RoleAssignment
	if assigneeType != "" {
		totalCount, err = as.roleStore.GetRoleAssignmentsCountByType(ctx, id, ouID, assigneeType)
	} else {
		totalCount, err = as.roleStore.GetRoleAssignmentsCount(ctx, id, ouID)
	}
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		logger.Error(ctx, "Failed to get role assignments count", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if assigneeType != "" {
		assignments, err = as.roleStore.GetRoleAssignmentsByType(ctx, id, ouID, limit, offset, assigneeType)
	} else {
		assignments, err = as.roleStore.GetRoleAssignments(ctx, id, ouID, limit, offset)
	}
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		logger.Error(ctx, "Failed to get role assignments", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	serviceAssignments, svcErr := as.resolveAssignments(ctx, assignments, includeDisplay)
	if svcErr != nil {
		return nil, svcErr
	}

	baseURL := fmt.Sprintf("/roles/%s/assignments", id)
	extraQuery := assignmentFilterQuery(ouID) + utils.DisplayQueryParam(includeDisplay)
	if assigneeType != "" {
		extraQuery += "&type=" + assigneeType
	}
	links := utils.BuildPaginationLinks(baseURL, limit, offset, totalCount, extraQuery)

	return &AssignmentList{
		TotalResults: totalCount,
		Assignments:  serviceAssignments,
		StartIndex:   offset + 1,
		Count:        len(serviceAssignments),
		Links:        links,
	}, nil
}

// getAssignmentsByEntityCategory handles ?type=user and ?type=app filter cases.
// Since both are stored as 'entity' internally, it fetches all entity assignments,
// resolves their category, and paginates the filtered results in memory.
func (as *roleAssignmentService) getAssignmentsByEntityCategory(
	ctx context.Context, id, ouID string, limit, offset int,
	includeDisplay bool, category string, logger *log.Logger,
) (*AssignmentList, *tidcommon.ServiceError) {
	totalEntityCount, err := as.roleStore.GetRoleAssignmentsCountByType(ctx, id, ouID, string(assigneeTypeEntity))
	if err != nil {
		if errors.Is(err, errResultLimitExceededInCompositeMode) {
			return nil, &ResultLimitExceededInCompositeMode
		}
		logger.Error(ctx, "Failed to get entity assignments count", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	var allEntityAssignments []RoleAssignment
	if totalEntityCount > 0 {
		allEntityAssignments, err = as.roleStore.GetRoleAssignmentsByType(
			ctx, id, ouID, totalEntityCount, 0, string(assigneeTypeEntity))
		if err != nil {
			if errors.Is(err, errResultLimitExceededInCompositeMode) {
				return nil, &ResultLimitExceededInCompositeMode
			}
			logger.Error(ctx, "Failed to get entity assignments", log.String("id", id), log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
	}

	// Batch-resolve entity categories.
	entityCategoryMap := make(map[string]string)
	if len(allEntityAssignments) > 0 {
		entityIDs := make([]string, len(allEntityAssignments))
		for i, a := range allEntityAssignments {
			entityIDs[i] = a.ID
		}
		entities, fetchErr := as.entityService.GetEntitiesByIDs(ctx, entityIDs)
		if fetchErr != nil {
			logger.Error(ctx, "Failed to batch fetch entities for category filter", log.Error(fetchErr))
			return nil, &tidcommon.InternalServerError
		}
		for _, e := range entities {
			entityCategoryMap[e.ID] = string(e.Category)
		}
	}

	// Filter to matching category and paginate in memory.
	var filtered []RoleAssignment
	for _, a := range allEntityAssignments {
		if entityCategoryMap[a.ID] == category {
			filtered = append(filtered, a)
		}
	}

	totalCount := len(filtered)
	start := offset
	if start > totalCount {
		start = totalCount
	}
	end := start + limit
	if end > totalCount {
		end = totalCount
	}
	page := filtered[start:end]

	serviceAssignments, svcErr := as.resolveAssignments(ctx, page, includeDisplay)
	if svcErr != nil {
		return nil, svcErr
	}

	baseURL := fmt.Sprintf("/roles/%s/assignments", id)
	extraQuery := assignmentFilterQuery(ouID) + utils.DisplayQueryParam(includeDisplay) + "&type=" + category
	links := utils.BuildPaginationLinks(baseURL, limit, offset, totalCount, extraQuery)

	return &AssignmentList{
		TotalResults: totalCount,
		Assignments:  serviceAssignments,
		StartIndex:   offset + 1,
		Count:        len(serviceAssignments),
		Links:        links,
	}, nil
}

// AddAssignments adds assignments to a role in an organization unit.
// Assignments can be added to both mutable (DB-backed) and declarative (file-backed) roles.
func (as *roleAssignmentService) AddAssignments(
	ctx context.Context, id, ouID string, assignments []RoleAssignment) *tidcommon.ServiceError {
	return as.modifyAssignments(ctx, id, ouID, assignments, true,
		as.roleStore.AddAssignments,
		"add assignments to role", "added assignments to role")
}

// RemoveAssignments removes assignments from a role in an organization unit.
// Assignments can be removed from both mutable (DB-backed) and declarative (file-backed) roles.
func (as *roleAssignmentService) RemoveAssignments(
	ctx context.Context, id, ouID string, assignments []RoleAssignment) *tidcommon.ServiceError {
	return as.modifyAssignments(ctx, id, ouID, assignments, false,
		as.roleStore.RemoveAssignments,
		"remove assignments from role", "removed assignments from role")
}

// ValidateRemoveAssignment runs the checks RemoveAssignments runs.
func (as *roleAssignmentService) ValidateRemoveAssignment(ctx context.Context, id, ouID, assigneeID string) (
	*revocation.AccessRevocationTarget, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName))

	if assigneeID == "" {
		return nil, &ErrorMissingAssigneeID
	}
	if svcErr := as.authzService.CanGrantMembership(ctx, sysauthz.PrincipalTypeRole, id); svcErr != nil {
		return nil, svcErr
	}
	if id == "" {
		return nil, &ErrorMissingRoleID
	}

	role, err := as.roleStore.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			return nil, &ErrorRoleNotFound
		}
		logger.Error(ctx, "Failed to retrieve role", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	if ouID == "" {
		ouID = role.OUID
	}
	if ouID != role.OUID {
		if svcErr := as.requireAssignableInOU(ctx, id, ouID, false); svcErr != nil {
			return nil, svcErr
		}
	}

	// Removing an assignment that is not there changes nothing, so it must not revoke anything.
	assignment, svcErr := as.directAssignment(ctx, id, ouID, assigneeID)
	if svcErr != nil {
		return nil, svcErr
	}
	if assignment == nil {
		return nil, &ErrorRoleAssignmentNotFound
	}
	if _, svcErr := as.prepareAssignments(
		ctx, id, ouID, []RoleAssignment{{ID: assignment.ID, Type: assignment.Type}}, false,
	); svcErr != nil {
		return nil, svcErr
	}

	scopes, svcErr := resolveScopes(ctx, as.resourceService, role.Permissions)
	if svcErr != nil {
		return nil, svcErr
	}
	entityIDs, svcErr := expandPrincipal(ctx, as.groupService, assigneeID, assignment.Type == AssigneeTypeGroup)
	if svcErr != nil {
		return nil, svcErr
	}
	return &revocation.AccessRevocationTarget{
		EntityIDs: entityIDs, Scopes: scopes, AssigneeType: string(assignment.Type),
	}, nil
}

// modifyAssignments is the shared implementation for AddAssignments and RemoveAssignments. Both
// directions carry the same requirement, since assigning conveys the role's permissions to the
// assignee.
func (as *roleAssignmentService) modifyAssignments(
	ctx context.Context,
	id, ouID string,
	assignments []RoleAssignment,
	adding bool,
	storeOp func(context.Context, string, []RoleAssignment) error,
	action, successAction string,
) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName))
	logger.Debug(ctx, "Modifying role assignments",
		log.String("action", action), log.String("id", id))

	if svcErr := as.authzService.CanGrantMembership(ctx, sysauthz.PrincipalTypeRole, id); svcErr != nil {
		return svcErr
	}

	normalized, svcErr := as.prepareAssignments(ctx, id, ouID, assignments, adding)
	if svcErr != nil {
		return svcErr
	}

	var capturedSvcErr *tidcommon.ServiceError
	if err := as.transactioner.Transact(ctx, func(txCtx context.Context) error {
		// Re-evaluated inside the transaction, since the role's permissions may change after the
		// earlier check.
		if svcErr := as.authzService.CanGrantMembership(
			txCtx, sysauthz.PrincipalTypeRole, id,
		); svcErr != nil {
			capturedSvcErr = svcErr
			return errors.New("rollback for disallowed grant")
		}
		return storeOp(txCtx, id, normalized)
	}); err != nil {
		if capturedSvcErr != nil {
			return capturedSvcErr
		}
		logger.Error(ctx, "Failed to modify role assignments",
			log.String("action", action), log.String("id", id), log.Error(err))
		return &tidcommon.InternalServerError
	}

	logger.Debug(ctx, "Successfully modified role assignments",
		log.String("action", successAction), log.String("id", id))
	return nil
}

// AddAssigneesToRoles adds assignees to multiple roles in a single transaction.
// A single failure rolls back all role assignments.
func (as *roleAssignmentService) AddAssigneesToRoles(
	ctx context.Context,
	assignments []RoleAssignment,
	roleIDs []string,
) *tidcommon.ServiceError {
	if len(roleIDs) == 0 || len(assignments) == 0 {
		return nil
	}
	// Each assignment is made in the organization unit it names, or in the one owning its assignee,
	// and is checked there as AddAssignments checks it.
	ouIDs, byOU, svcErr := as.assignmentsByOU(ctx, assignments)
	if svcErr != nil {
		return svcErr
	}
	return as.addInEachOU(ctx, roleIDs, ouIDs, byOU)
}

// assignmentsByOU groups assignments by the organization unit each is made in: the one it names, or
// the one owning its assignee when it names none. ouIDs lists the units in the order they first
// appear.
func (as *roleAssignmentService) assignmentsByOU(
	ctx context.Context, assignments []RoleAssignment,
) ([]string, map[string][]RoleAssignment, *tidcommon.ServiceError) {
	var entityIDs, groupIDs []string
	for _, a := range assignments {
		if a.OUID != "" {
			continue
		}
		if a.Type == AssigneeTypeGroup {
			groupIDs = append(groupIDs, a.ID)
		} else {
			entityIDs = append(entityIDs, a.ID)
		}
	}

	ownerOU := make(map[string]string, len(entityIDs)+len(groupIDs))
	if len(entityIDs) > 0 {
		entities, err := as.entityService.GetEntitiesByIDs(ctx, utils.UniqueStrings(entityIDs))
		if err != nil {
			as.logger.Error(ctx, "Failed to fetch entities for role assignment", log.Error(err))
			return nil, nil, &tidcommon.InternalServerError
		}
		for _, e := range entities {
			ownerOU[e.ID] = e.OUID
		}
	}
	if len(groupIDs) > 0 {
		groups, svcErr := as.groupService.GetGroupsByIDs(ctx, utils.UniqueStrings(groupIDs))
		if svcErr != nil {
			as.logger.Error(ctx, "Failed to fetch groups for role assignment",
				log.String("error", svcErr.Error.DefaultValue))
			return nil, nil, &tidcommon.InternalServerError
		}
		for id, g := range groups {
			ownerOU[id] = g.OUID
		}
	}

	byOU := make(map[string][]RoleAssignment)
	ouIDs := make([]string, 0)
	for _, a := range assignments {
		ouID := a.OUID
		if ouID == "" {
			var ok bool
			if ouID, ok = ownerOU[a.ID]; !ok {
				return nil, nil, &ErrorInvalidAssignmentID
			}
		}
		if _, seen := byOU[ouID]; !seen {
			ouIDs = append(ouIDs, ouID)
		}
		byOU[ouID] = append(byOU[ouID], a)
	}
	return ouIDs, byOU, nil
}

// addInEachOU adds each organization unit's assignments to every role in one transaction, so a
// single refusal adds none.
func (as *roleAssignmentService) addInEachOU(
	ctx context.Context, roleIDs, ouIDs []string, byOU map[string][]RoleAssignment,
) *tidcommon.ServiceError {
	var capturedSvcErr *tidcommon.ServiceError
	err := as.transactioner.Transact(ctx, func(txCtx context.Context) error {
		for _, rid := range roleIDs {
			for _, ouID := range ouIDs {
				if svcErr := as.AddAssignments(txCtx, rid, ouID, byOU[ouID]); svcErr != nil {
					capturedSvcErr = svcErr
					return fmt.Errorf("failed to assign role %s: %s", rid, svcErr.Error.DefaultValue)
				}
			}
		}
		return nil
	})
	if capturedSvcErr != nil {
		return capturedSvcErr
	}
	if err != nil {
		as.logger.Error(ctx, "Failed to add role assignments", log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// prepareAssignments validates and normalizes assignments before a mutation, placing each in ouID,
// or in the role's owner when ouID is empty.
// Unlike the previous role service implementation, this allows modifying assignments for
// both mutable and declarative (file-backed) roles.
func (as *roleAssignmentService) prepareAssignments(
	ctx context.Context, id, ouID string, assignments []RoleAssignment, adding bool,
) ([]RoleAssignment, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName))

	if id == "" {
		return nil, &ErrorMissingRoleID
	}

	if err := as.validateAssignmentsRequest(assignments); err != nil {
		return nil, err
	}

	role, err := as.roleStore.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			logger.Debug(ctx, "Role not found", log.String("id", id))
			return nil, &ErrorRoleNotFound
		}
		logger.Error(ctx, "Failed to retrieve role", log.String("id", id), log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if ouID == "" {
		ouID = role.OUID
	}
	if ouID != role.OUID {
		if svcErr := as.requireAssignableInOU(ctx, id, ouID, adding); svcErr != nil {
			return nil, svcErr
		}
	}

	// Taking someone off a role only ever narrows what they hold, so where they live is not checked.
	assigneeOUID := ""
	if adding {
		assigneeOUID = ouID
	}
	if err := validateAssignmentIDs(ctx, assignments, assigneeOUID, as.entityService, as.groupService,
		as.sharingService, assignmentLoggerComponentName); err != nil {
		return nil, err
	}

	normalized := normalizeAssignments(assignments)
	for i := range normalized {
		normalized[i].OUID = ouID
	}

	return normalized, nil
}

// requireAssignableInOU refuses assignments in an organization unit the role is not shared with,
// and additions there when the role's sharing policies turn its assignments off. A removal is
// allowed even when the assignments are turned off, since it only takes away what was given.
func (as *roleAssignmentService) requireAssignableInOU(
	ctx context.Context, id, ouID string, adding bool,
) *tidcommon.ServiceError {
	overlay, svcErr := as.sharingService.ResolveOverlayRules(ctx, RoleSharingType, id, ouID)
	if svcErr != nil {
		if svcErr.Code == oupkg.ErrorOrganizationUnitNotFound.Code {
			return &ErrorRoleNotSharedToOU
		}
		return translateSharingError(svcErr)
	}
	if !overlay.Visible {
		return &ErrorRoleNotSharedToOU
	}
	if adding && !overlay.Rules[roleAssignmentsFieldKey].Editable {
		return &ErrorSharedRoleFieldNotEditable
	}
	return nil
}

// validateAssignmentsRequest validates the assignments request.
// Accepts public types 'user', 'app', 'agent', 'group'.
func (as *roleAssignmentService) validateAssignmentsRequest(
	assignments []RoleAssignment) *tidcommon.ServiceError {
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

// validateAssignmentIDs validates assignment IDs checking entity/group existence and type matching.
// A non-empty ouID also requires every assignee to be available there: an entity owned by or shared
// with it, or a group owned by it, since groups are never shared.
func validateAssignmentIDs(
	ctx context.Context,
	assignments []RoleAssignment,
	ouID string,
	entitySvc entity.EntityServiceInterface,
	groupSvc group.GroupServiceInterface,
	sharingSvc sharing.SharingServiceInterface,
	loggerComponent string,
) *tidcommon.ServiceError {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponent))

	typeByID := make(map[string]AssigneeType)
	var groupIDs []string

	for _, a := range assignments {
		if a.Type.IsEntityType() {
			if existing, ok := typeByID[a.ID]; ok && existing != a.Type {
				return &ErrorInvalidAssignmentID
			}
			typeByID[a.ID] = a.Type
		} else if a.Type == AssigneeTypeGroup {
			groupIDs = append(groupIDs, a.ID)
		}
	}

	groupIDs = utils.UniqueStrings(groupIDs)

	if len(typeByID) > 0 {
		entityIDs := make([]string, 0, len(typeByID))
		for id := range typeByID {
			entityIDs = append(entityIDs, id)
		}

		entities, err := entitySvc.GetEntitiesByIDs(ctx, entityIDs)
		if err != nil {
			logger.Error(ctx, "Failed to fetch entities for assignment validation", log.Error(err))
			return &tidcommon.InternalServerError
		}

		if len(entities) != len(entityIDs) {
			return &ErrorInvalidAssignmentID
		}

		for _, e := range entities {
			claimed := typeByID[e.ID]
			actual := AssigneeType(e.Category)
			if claimed != actual {
				logger.Debug(ctx, "Assignment type mismatch", log.String("id", e.ID),
					log.String("claimed", string(claimed)), log.String("actual", string(actual)))
				return &ErrorInvalidAssignmentID
			}
			if ouID == "" || e.OUID == ouID {
				continue
			}
			shared, svcErr := isEntitySharedWithOU(ctx, sharingSvc, e, ouID)
			if svcErr != nil {
				logger.Error(ctx, "Failed to check whether an assignee is shared with the organization unit",
					log.String("id", e.ID), log.Any("error", svcErr))
				return &tidcommon.InternalServerError
			}
			if !shared {
				return &ErrorAssigneeNotInOU
			}
		}
	}

	if len(groupIDs) > 0 {
		if err := groupSvc.ValidateGroupIDs(ctx, groupIDs); err != nil {
			if err.Code == group.ErrorInvalidGroupMemberID.Code {
				logger.Debug(ctx, "Invalid group member IDs found")
				return &ErrorInvalidAssignmentID
			}
			logger.Error(ctx, "Failed to validate group IDs", log.String("error", err.Error.DefaultValue))
			return &tidcommon.InternalServerError
		}
		if ouID != "" {
			groups, svcErr := groupSvc.GetGroupsByIDs(ctx, groupIDs)
			if svcErr != nil {
				logger.Error(ctx, "Failed to fetch groups for assignment validation",
					log.String("error", svcErr.Error.DefaultValue))
				return &tidcommon.InternalServerError
			}
			for _, id := range groupIDs {
				if g, ok := groups[id]; !ok || g.OUID != ouID {
					return &ErrorAssigneeNotInOU
				}
			}
		}
	}

	return nil
}

// isEntitySharedWithOU reports whether a sharing policy reaches ouID with the entity. Users and
// applications are shareable; an entity of any other category is never shared.
func isEntitySharedWithOU(
	ctx context.Context, sharingSvc sharing.SharingServiceInterface, e providers.Entity, ouID string,
) (bool, *tidcommon.ServiceError) {
	var rt sharing.ResourceType
	switch e.Category {
	case providers.EntityCategoryUser:
		rt = resourcedependency.ResourceTypeUser
	case providers.EntityCategoryApp:
		rt = resourcedependency.ResourceTypeApplication
	default:
		return false, nil
	}
	return sharingSvc.IsVisible(ctx, rt, e.ID, ouID)
}

// resolveAssignments resolves the public types and optionally display names for role assignments.
func (as *roleAssignmentService) resolveAssignments(
	ctx context.Context,
	assignments []RoleAssignment,
	includeDisplay bool,
) ([]RoleAssignmentWithDisplay, *tidcommon.ServiceError) {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, assignmentLoggerComponentName))

	var entityIDs, groupIDs []string
	for _, a := range assignments {
		switch a.Type {
		case assigneeTypeEntity:
			entityIDs = append(entityIDs, a.ID)
		case AssigneeTypeGroup:
			groupIDs = append(groupIDs, a.ID)
		}
	}

	// Always batch-fetch entities to resolve their category (user vs app) for the API response type.
	var entityMap map[string]*providers.Entity
	var displays map[string]string
	if len(entityIDs) > 0 {
		entities, err := as.entityService.GetEntitiesByIDs(ctx, entityIDs)
		if err != nil {
			logger.Error(ctx, "Failed to batch fetch entities for assignments", log.Error(err))
			return nil, &tidcommon.InternalServerError
		}
		entityMap = make(map[string]*providers.Entity, len(entities))
		for i := range entities {
			entityMap[entities[i].ID] = &entities[i]
		}
		if includeDisplay {
			displays = entitytype.ResolveEntityDisplays(ctx, entities, as.entityTypeService, logger)
		}
	}

	var groupsMap map[string]*group.Group
	if includeDisplay && len(groupIDs) > 0 {
		var svcErr *tidcommon.ServiceError
		groupsMap, svcErr = as.groupService.GetGroupsByIDs(ctx, groupIDs)
		if svcErr != nil {
			logger.Warn(ctx, "Failed to batch fetch groups for display names", log.Any("error", svcErr))
		}
	}

	// Build the result slice, skipping orphaned entity assignments.
	result := make([]RoleAssignmentWithDisplay, 0, len(assignments))
	for _, a := range assignments {
		ra := RoleAssignmentWithDisplay{ID: a.ID, OUID: a.OUID}
		switch a.Type {
		case assigneeTypeEntity:
			e, ok := entityMap[a.ID]
			if !ok {
				logger.Warn(ctx, "Skipping orphaned entity assignment", log.String("id", a.ID))
				continue
			}
			ra.Type = AssigneeType(e.Category)
			if includeDisplay {
				switch e.Category {
				case providers.EntityCategoryUser, providers.EntityCategoryAgent:
					ra.Display = displays[e.ID]
				default:
					ra.Display = resolveAppDisplay(*e)
				}
			}
		case AssigneeTypeGroup:
			ra.Type = AssigneeTypeGroup
			if includeDisplay {
				if groupsMap != nil {
					if g, ok := groupsMap[a.ID]; ok {
						ra.Display = g.Name
					} else {
						ra.Display = a.ID
					}
				} else {
					ra.Display = a.ID
				}
			}
		default:
			ra.Type = a.Type
			ra.Display = a.ID
		}
		result = append(result, ra)
	}
	return result, nil
}

// directAssignment returns the role's direct assignment to the assignee, or nil when there is none.
func (as *roleAssignmentService) directAssignment(ctx context.Context, id, ouID, assigneeID string) (
	*RoleAssignmentWithDisplay, *tidcommon.ServiceError) {
	for offset := 0; ; offset += serverconst.MaxPageSize {
		page, svcErr := as.GetRoleAssignments(ctx, id, ouID, serverconst.MaxPageSize, offset, false)
		if svcErr != nil {
			return nil, svcErr
		}
		for i := range page.Assignments {
			if page.Assignments[i].ID == assigneeID {
				return &page.Assignments[i], nil
			}
		}
		if offset+serverconst.MaxPageSize >= page.TotalResults {
			return nil, nil
		}
	}
}

// resolveAppDisplay extracts a display name for an app entity from its system attributes.
func resolveAppDisplay(e providers.Entity) string {
	if len(e.SystemAttributes) > 0 {
		var sysAttrs map[string]interface{}
		if err := json.Unmarshal(e.SystemAttributes, &sysAttrs); err == nil {
			if name, ok := sysAttrs["name"].(string); ok && name != "" {
				return name
			}
		}
	}
	return e.ID
}

// normalizeAssignments converts public 'user'/'app'/'agent' types to the internal 'entity' type.
func normalizeAssignments(assignments []RoleAssignment) []RoleAssignment {
	normalized := make([]RoleAssignment, len(assignments))
	for i, a := range assignments {
		t := a.Type
		if t.IsEntityType() {
			t = assigneeTypeEntity
		}
		normalized[i] = RoleAssignment{ID: a.ID, Type: t, OUID: a.OUID}
	}
	return normalized
}

// DeleteAssignmentsInOU removes every assignment of the role made in the organization unit.
func (as *roleAssignmentService) DeleteAssignmentsInOU(ctx context.Context, id, ouID string) error {
	return as.roleStore.DeleteAssignmentsInOU(ctx, id, ouID)
}

// GetResourceDependencies implements resourcedependency.Provider. Role assignments are cleaned up
// via cascade rather than surfaced as blocking usages, so no dependencies are reported here.
func (as *roleAssignmentService) GetResourceDependencies(
	_ context.Context, _, _ string) ([]resourcedependency.ResourceDependency, error) {
	return []resourcedependency.ResourceDependency{}, nil
}

// CascadeDeleteDependencies implements resourcedependency.CascadeDeleter. It removes the role
// assignments held by the given principal when that principal is deleted. Only user, app and agent
// principals (stored as entity assignees) are handled; other resource types have no assignments.
func (as *roleAssignmentService) CascadeDeleteDependencies(
	ctx context.Context, resourceType, id string) (int, error) {
	var assigneeType AssigneeType
	switch resourceType {
	case resourcedependency.ResourceTypeUser,
		resourcedependency.ResourceTypeApplication,
		resourcedependency.ResourceTypeAgent:
		assigneeType = assigneeTypeEntity
	case resourcedependency.ResourceTypeGroup:
		assigneeType = AssigneeTypeGroup
	default:
		return 0, nil
	}

	deleted, err := as.roleStore.DeleteAssignmentsByAssignee(ctx, string(assigneeType), id)
	if err != nil {
		return 0, err
	}
	return int(deleted), nil
}

// assignmentFilterQuery returns the filter an assignment list was narrowed by, for its pagination
// links to carry.
func assignmentFilterQuery(ouID string) string {
	if ouID == "" {
		return ""
	}
	return utils.FilterQueryParam(map[string]interface{}{filter.AttributeOUID: ouID})
}

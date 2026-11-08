// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/group"
	oupkg "github.com/thunder-id/thunderid/internal/ou"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
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
	// ListAssignableRoles lists the roles the caller may assign in the organization unit: the ones it owns
	// and the ones shared with it whose assignments it may change, as far as the caller may grant them.
	ListAssignableRoles(ctx context.Context, ouID string) ([]Role, *tidcommon.ServiceError)
	// AddAssigneesToRoles assigns each assignee to the roles in the organization unit it names, or in
	// the one owning it when it names none. All are added or none is.
	AddAssigneesToRoles(ctx context.Context, assignments []RoleAssignment,
		roleIDs []string) *tidcommon.ServiceError
	// DeleteAssignmentsInOU removes every assignment of the role made in the organization unit.
	DeleteAssignmentsInOU(ctx context.Context, id, ouID string) error
	// ApplyMandatedAssignments syncs a sharing policy's mandated assignments to desired (unit to role IDs).
	ApplyMandatedAssignments(ctx context.Context, policyID string, assignee RoleAssignment,
		desired map[string][]string) *tidcommon.ServiceError
	// RemoveMandatedAssignments removes every assignment a sharing policy mandates.
	RemoveMandatedAssignments(ctx context.Context, policyID string) *tidcommon.ServiceError
	GetResourceDependencies(
		ctx context.Context, resourceType, id string) ([]resourcedependency.ResourceDependency, error)
	CascadeDeleteDependencies(ctx context.Context, resourceType, id string) (int, error)
	// HasGroupRoleAssignments reports whether the group is assigned any role.
	HasGroupRoleAssignments(ctx context.Context, groupID string) (bool, *tidcommon.ServiceError)
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
	bus               eventbus.EventBusInterface
	logger            *log.Logger
	// systemFlows is nil where system flows are off.
	systemFlows systemFlowProvider
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
	bus eventbus.EventBusInterface,
) *roleAssignmentService {
	return &roleAssignmentService{
		roleStore:         roleStore,
		entityService:     entityService,
		groupService:      groupService,
		entityTypeService: entityTypeService,
		resourceService:   resourceService,
		transactioner:     transactioner,
		authzService:      authzService,
		sharingService:    sharingService,
		bus:               bus,
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
	svcErr := as.addAssignments(ctx, id, ouID, assignments)
	publishAssignmentEvents(ctx, as.bus, audit.EventRoleAssignmentAdd, audit.ActionAdd, id, assignments, svcErr)
	return svcErr
}

// addAssignments adds assignments to a role in an organization unit without publishing their audit events.
func (as *roleAssignmentService) addAssignments(
	ctx context.Context, id, ouID string, assignments []RoleAssignment) *tidcommon.ServiceError {
	return as.modifyAssignments(ctx, id, ouID, assignments, true,
		as.roleStore.AddAssignments,
		"add assignments to role", "added assignments to role")
}

// RemoveAssignments removes assignments from a role in an organization unit.
// Assignments can be removed from both mutable (DB-backed) and declarative (file-backed) roles.
func (as *roleAssignmentService) RemoveAssignments(
	ctx context.Context, id, ouID string, assignments []RoleAssignment) *tidcommon.ServiceError {
	svcErr := as.modifyAssignments(ctx, id, ouID, assignments, false,
		as.roleStore.RemoveAssignments,
		"remove assignments from role", "removed assignments from role")
	// The store does not report which assignments it removed, so the removal of a principal that did not
	// hold the role is recorded as succeeded: a known limitation.
	publishAssignmentEvents(ctx, as.bus, audit.EventRoleAssignmentRemove, audit.ActionRemove, id, assignments,
		svcErr)
	return svcErr
}

// publishAssignmentEvents publishes the audit events of an attempt to add assignees to the role with
// roleID, or remove them, after the change committed or was refused.
func publishAssignmentEvents(ctx context.Context, bus eventbus.EventBusInterface, name string,
	action audit.Action, roleID string, assignments []RoleAssignment, svcErr *tidcommon.ServiceError) {
	assignees := make([]audit.Ref, 0, len(assignments))
	for _, a := range assignments {
		assignees = append(assignees, audit.PrincipalRef(ctx, string(a.Type), a.ID))
	}
	target := audit.Ref{Type: audit.TargetTypeRole, ID: roleID}
	for _, payload := range audit.MembershipPayloads(ctx, target, assignees, action, svcErr) {
		eventbus.TryPublish(ctx, bus, eventbus.NewEvent(ctx, name).With(payload))
	}
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
	mandated, svcErr := as.mandatedAssignments(ctx, id, ouID)
	if svcErr != nil {
		return nil, svcErr
	}
	keptByMandate := mandatesAssignee(mandated, assigneeID)
	if assignment == nil {
		if keptByMandate {
			return nil, &ErrorAssignmentMandated
		}
		return nil, &ErrorRoleAssignmentNotFound
	}
	if _, svcErr := as.prepareAssignments(
		ctx, id, ouID, []RoleAssignment{{ID: assignment.ID, Type: assignment.Type}}, false,
	); svcErr != nil {
		return nil, svcErr
	}
	// The assignee keeps the role through the mandate, so removing the direct assignment revokes nothing.
	if keptByMandate {
		return &revocation.AccessRevocationTarget{AssigneeType: string(assignment.Type)}, nil
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
	var revocations []assignmentRevocation
	if !adding {
		direct, svcErr := as.directAssignees(ctx, id, normalized)
		if svcErr != nil {
			return svcErr
		}
		if svcErr := as.requireDirectRemoval(ctx, id, normalized, direct); svcErr != nil {
			return svcErr
		}
		if revocations, svcErr = as.revokeRemovedAssignments(ctx, id, normalized, direct); svcErr != nil {
			return svcErr
		}
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
	// Every post flow runs, so one assignee's failure does not leave another's revocation unfinished.
	var postErr *tidcommon.ServiceError
	for _, revoked := range revocations {
		if _, svcErr := runSystemFlow(ctx, as.systemFlows, systemFlowRevocationPost, revoked.inputs,
			revoked.shared); svcErr != nil {
			logger.Error(ctx, "Failed to run the revocation post flow", log.String("id", id),
				log.MaskedString("assignee", revoked.inputs[systemFlowInputAssignee]), log.String("code", svcErr.Code))
			if postErr == nil {
				postErr = svcErr
			}
		}
	}
	if postErr != nil {
		return postErr
	}

	logger.Debug(ctx, "Successfully modified role assignments",
		log.String("action", successAction), log.String("id", id))
	return nil
}

// assignmentRevocation is a removed assignment's revocation: the inputs its flows run with and what its
// pre flow shared, for the post flow.
type assignmentRevocation struct {
	inputs map[string]string
	shared map[string]string
}

// revokeRemovedAssignments runs the removal pre flow for each named assignee holding the role directly
// and returns the revocations whose post flows run once the removal is written.
func (as *roleAssignmentService) revokeRemovedAssignments(ctx context.Context, id string,
	normalized []RoleAssignment, direct map[string]bool) ([]assignmentRevocation, *tidcommon.ServiceError) {
	if as.systemFlows == nil {
		return nil, nil
	}
	ouID := normalized[0].OUID
	revoked := make(map[string]bool, len(direct))
	var revocations []assignmentRevocation
	for _, assignment := range normalized {
		if !direct[assignment.ID] || revoked[assignment.ID] {
			continue
		}
		revoked[assignment.ID] = true
		inputs := map[string]string{
			systemFlowInputRole: id, systemFlowInputAssignee: assignment.ID, systemFlowInputAssignmentOU: ouID,
		}
		shared, svcErr := runSystemFlow(ctx, as.systemFlows, systemFlowRoleAssignmentRemovalPre, inputs, nil)
		if svcErr != nil {
			return nil, svcErr
		}
		revocations = append(revocations, assignmentRevocation{inputs: inputs, shared: shared})
	}
	return revocations, nil
}

// AddAssigneesToRoles adds assignees to multiple roles in a single transaction.
// A single failure rolls back all role assignments.
func (as *roleAssignmentService) AddAssigneesToRoles(
	ctx context.Context,
	assignments []RoleAssignment,
	roleIDs []string,
) *tidcommon.ServiceError {
	svcErr := as.addAssigneesToRoles(ctx, assignments, roleIDs)
	// Published once the transaction is over, so a role whose additions were rolled back is never
	// recorded as assigned.
	for _, roleID := range utils.UniqueStrings(roleIDs) {
		publishAssignmentEvents(ctx, as.bus, audit.EventRoleAssignmentAdd, audit.ActionAdd, roleID, assignments,
			svcErr)
	}
	return svcErr
}

// addAssigneesToRoles adds assignees to multiple roles in a single transaction without publishing their
// audit events.
func (as *roleAssignmentService) addAssigneesToRoles(
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
				if svcErr := as.addAssignments(txCtx, rid, ouID, byOU[ouID]); svcErr != nil {
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
	if !adding && len(assignments) > maxRemovedAssignments {
		return nil, &ErrorTooManyAssignments
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

// ListAssignableRoles lists the roles the caller may assign in the organization unit.
func (as *roleAssignmentService) ListAssignableRoles(
	ctx context.Context, ouID string,
) ([]Role, *tidcommon.ServiceError) {
	ownedCount, err := as.roleStore.GetRoleListCountByOUID(ctx, ouID)
	if err != nil {
		return nil, as.listError(ctx, ouID, err)
	}
	var roles []Role
	if ownedCount > 0 {
		if roles, err = as.roleStore.GetRoleListByOUID(ctx, ouID, ownedCount, 0); err != nil {
			return nil, as.listError(ctx, ouID, err)
		}
	}

	sharedIDs, svcErr := as.sharingService.ListVisibleResourceIDs(ctx, RoleSharingType, ouID)
	if svcErr != nil {
		// An organization unit that does not exist has nothing shared with it.
		if svcErr.Code != oupkg.ErrorOrganizationUnitNotFound.Code {
			return nil, translateSharingError(svcErr)
		}
		sharedIDs = nil
	}
	editableIDs := make([]string, 0, len(sharedIDs))
	for _, id := range sharedIDs {
		svcErr := as.requireAssignableInOU(ctx, id, ouID, true)
		if svcErr == nil {
			editableIDs = append(editableIDs, id)
		} else if svcErr.Type == tidcommon.ServerErrorType {
			return nil, svcErr
		}
	}
	if len(editableIDs) > 0 {
		shared, err := as.roleStore.GetRolesByIDs(ctx, editableIDs)
		if err != nil {
			return nil, as.listError(ctx, ouID, err)
		}
		roles = append(roles, shared...)
	}

	ids := make([]string, 0, len(roles))
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	grantable, svcErr := as.authzService.GrantableMemberships(ctx, sysauthz.PrincipalTypeRole, ids)
	if svcErr != nil {
		return nil, svcErr
	}
	assignable := make([]Role, 0, len(grantable))
	for _, r := range roles {
		if slices.Contains(grantable, r.ID) {
			assignable = append(assignable, r)
		}
	}
	return assignable, nil
}

// listError maps a store failure while listing a unit's roles to a service error.
func (as *roleAssignmentService) listError(ctx context.Context, ouID string, err error) *tidcommon.ServiceError {
	if errors.Is(err, errResultLimitExceededInCompositeMode) {
		return &ResultLimitExceededInCompositeMode
	}
	as.logger.Error(ctx, "Failed to list roles in organization unit", log.String("ouID", ouID), log.Error(err))
	return &tidcommon.InternalServerError
}

// requireDirectRemoval refuses a removal naming an assignee who holds the role in the unit only through a mandate.
func (as *roleAssignmentService) requireDirectRemoval(
	ctx context.Context, id string, normalized []RoleAssignment, direct map[string]bool,
) *tidcommon.ServiceError {
	ouID := normalized[0].OUID
	mandated, svcErr := as.mandatedAssignments(ctx, id, ouID)
	if svcErr != nil {
		return svcErr
	}
	for _, a := range normalized {
		if mandatesAssignee(mandated, a.ID) && !direct[a.ID] {
			return &ErrorAssignmentMandated
		}
	}
	return nil
}

// mandatesAssignee reports whether the mandated assignments include the assignee, named by ID as removals name it.
func mandatesAssignee(mandated []RoleAssignment, assigneeID string) bool {
	return slices.ContainsFunc(mandated, func(m RoleAssignment) bool { return m.ID == assigneeID })
}

// mandatedAssignments lists the assignments sharing policies mandate for the role in the organization unit.
func (as *roleAssignmentService) mandatedAssignments(
	ctx context.Context, id, ouID string,
) ([]RoleAssignment, *tidcommon.ServiceError) {
	mandated, err := as.roleStore.ListMandatedAssignmentsForRole(ctx, id, ouID)
	if err != nil {
		as.logger.Error(ctx, "Failed to list the mandated assignments of the role", log.String("id", id),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	return mandated, nil
}

// directAssignees returns which of the named assignees hold a direct assignment of the role in the
// organization unit the assignments are removed from.
func (as *roleAssignmentService) directAssignees(
	ctx context.Context, id string, normalized []RoleAssignment,
) (map[string]bool, *tidcommon.ServiceError) {
	assigneeIDs := make([]string, 0, len(normalized))
	for _, a := range normalized {
		assigneeIDs = append(assigneeIDs, a.ID)
	}
	assignments, err := as.roleStore.GetDirectAssignments(ctx, id, normalized[0].OUID,
		utils.UniqueStrings(assigneeIDs))
	if err != nil {
		as.logger.Error(ctx, "Failed to get the direct assignments of the role", log.String("id", id),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	direct := make(map[string]bool, len(assignments))
	for _, a := range assignments {
		direct[a.ID] = true
	}
	return direct, nil
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
			shared, svcErr := isEntitySharedWithOU(ctx, entitySvc, sharingSvc, e, ouID)
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

// isEntitySharedWithOU reports whether a sharing policy reaches ouID with the entity. An entity whose
// category is not registered as shareable with the entity service is never shared.
func isEntitySharedWithOU(
	ctx context.Context, entitySvc entity.EntityServiceInterface, sharingSvc sharing.SharingServiceInterface,
	e providers.Entity, ouID string,
) (bool, *tidcommon.ServiceError) {
	rt, shareable := entitySvc.GetSharedResourceType(e.Category)
	if !shareable {
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
		ra := RoleAssignmentWithDisplay{ID: a.ID, OUID: a.OUID, PolicyID: a.PolicyID}
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
	assignments, err := as.roleStore.GetDirectAssignments(ctx, id, ouID, []string{assigneeID})
	if err != nil {
		as.logger.Error(ctx, "Failed to get the direct assignments of the role", log.String("id", id),
			log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	if len(assignments) == 0 {
		return nil, nil
	}
	resolved, svcErr := as.resolveAssignments(ctx, assignments[:1], false)
	if svcErr != nil {
		return nil, svcErr
	}
	return &resolved[0], nil
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

// ApplyMandatedAssignments syncs a policy's mandated rows to desired, regardless of the roles' assignments rule.
func (as *roleAssignmentService) ApplyMandatedAssignments(ctx context.Context, policyID string,
	assignee RoleAssignment, desired map[string][]string) *tidcommon.ServiceError {
	if svcErr := as.validateAssignmentsRequest([]RoleAssignment{assignee}); svcErr != nil {
		return svcErr
	}
	wanted := make(map[string][]string, len(desired))
	for ouID, roleIDs := range desired {
		if len(roleIDs) > 0 {
			wanted[ouID] = utils.UniqueStrings(roleIDs)
		}
	}
	// Roles that load only from resource files cannot hold assignments made at runtime.
	if isDeclarativeModeEnabled() {
		if len(wanted) > 0 {
			return &ErrorMandatedRolesNotSupported
		}
		return nil
	}

	normalized := normalizeAssignments([]RoleAssignment{assignee})[0]
	var refusal *tidcommon.ServiceError
	if err := as.transactioner.Transact(ctx, func(txCtx context.Context) error {
		for _, ouID := range slices.Sorted(maps.Keys(wanted)) {
			for _, roleID := range wanted[ouID] {
				if svcErr := as.requireRoleInOU(txCtx, roleID, ouID); svcErr != nil {
					refusal = svcErr
					return errMandateRefused
				}
			}
		}
		svcErr, err := as.syncMandatedAssignments(txCtx, policyID, normalized, wanted)
		if svcErr != nil {
			refusal = svcErr
			return errMandateRefused
		}
		return err
	}); err != nil {
		if refusal != nil {
			return refusal
		}
		as.logger.Error(ctx, "Failed to apply mandated role assignments", log.String("policyID", policyID),
			log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// RemoveMandatedAssignments removes every assignment a sharing policy mandates.
func (as *roleAssignmentService) RemoveMandatedAssignments(
	ctx context.Context, policyID string) *tidcommon.ServiceError {
	// Roles that load only from resource files hold no mandated assignments.
	if isDeclarativeModeEnabled() {
		return nil
	}
	if err := as.roleStore.DeleteMandatedAssignments(ctx, policyID, MandatedFilter{}); err != nil {
		as.logger.Error(ctx, "Failed to remove mandated role assignments", log.String("policyID", policyID),
			log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// reconcileMandatedAssignments drops, with a warning, the mandates whose policy is gone or role no longer reaches.
func (as *roleAssignmentService) reconcileMandatedAssignments(ctx context.Context) error {
	// Roles that load only from resource files hold no mandated assignments.
	if isDeclarativeModeEnabled() {
		return nil
	}
	units, err := as.roleStore.ListMandatedRoleUnits(ctx)
	if err != nil {
		return fmt.Errorf("failed to list mandated role assignments: %w", err)
	}

	policyExists := make(map[string]bool)
	roleReaches := make(map[[2]string]bool)
	for _, u := range units {
		exists, checked := policyExists[u.PolicyID]
		if !checked {
			_, svcErr := as.sharingService.GetPolicy(ctx, u.PolicyID)
			if svcErr != nil && svcErr.Code != sharing.ErrorPolicyNotFound.Code {
				return fmt.Errorf("failed to read sharing policy %s: %s", u.PolicyID, svcErr.Code)
			}
			exists = svcErr == nil
			policyExists[u.PolicyID] = exists
		}

		message := "Removing mandated role assignments: the sharing policy no longer exists"
		if exists {
			key := [2]string{u.RoleID, u.OUID}
			reaches, checked := roleReaches[key]
			if !checked {
				svcErr := as.requireRoleInOU(ctx, u.RoleID, u.OUID)
				if svcErr != nil && svcErr.Type != tidcommon.ClientErrorType {
					return fmt.Errorf("failed to check role %s reaches organization unit %s: %s",
						u.RoleID, u.OUID, svcErr.Code)
				}
				reaches = svcErr == nil
				roleReaches[key] = reaches
			}
			if reaches {
				continue
			}
			message = "Removing mandated role assignments: the role no longer reaches the organization unit " +
				"(it is unshared from it or the unit is gone), but the sharing policy still names the role " +
				"and must be edited"
		}

		as.logger.Warn(ctx, message,
			log.String("policyID", u.PolicyID), log.String("roleID", u.RoleID), log.String("ouID", u.OUID))
		if err := as.roleStore.DeleteMandatedAssignments(
			ctx, u.PolicyID, MandatedFilter{OUID: u.OUID, RoleID: u.RoleID}); err != nil {
			return fmt.Errorf("failed to remove the mandated assignments of sharing policy %s: %w", u.PolicyID, err)
		}
	}
	return nil
}

// requireRoleInOU refuses to mandate a role in an organization unit that neither owns it nor is shared it.
func (as *roleAssignmentService) requireRoleInOU(ctx context.Context, roleID, ouID string) *tidcommon.ServiceError {
	role, err := as.roleStore.GetRole(ctx, roleID)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) {
			return &ErrorMandatedRoleNotInOU
		}
		as.logger.Error(ctx, "Failed to retrieve role", log.String("id", roleID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if role.OUID == ouID {
		return nil
	}
	visible, svcErr := as.sharingService.IsVisible(ctx, RoleSharingType, roleID, ouID)
	if svcErr != nil {
		if svcErr.Type == tidcommon.ClientErrorType {
			return &ErrorMandatedRoleNotInOU
		}
		return &tidcommon.InternalServerError
	}
	if !visible {
		return &ErrorMandatedRoleNotInOU
	}
	return nil
}

// syncMandatedAssignments rewrites the policy's rows per unit, first refusing a new role the caller may not grant.
func (as *roleAssignmentService) syncMandatedAssignments(
	ctx context.Context, policyID string, assignee RoleAssignment, wanted map[string][]string,
) (*tidcommon.ServiceError, error) {
	existing, err := as.roleStore.ListMandatedAssignments(ctx, policyID)
	if err != nil {
		return nil, err
	}
	held := make(map[string][]string)
	for _, m := range existing {
		held[m.Assignment.OUID] = append(held[m.Assignment.OUID], m.RoleID)
	}

	ouIDs := utils.UniqueStrings(slices.Concat(slices.Collect(maps.Keys(wanted)), slices.Collect(maps.Keys(held))))
	slices.Sort(ouIDs)
	added := make(map[string][]string, len(ouIDs))
	var granted []string
	for _, ouID := range ouIDs {
		added[ouID] = slices.DeleteFunc(slices.Clone(wanted[ouID]), func(r string) bool {
			return slices.Contains(held[ouID], r)
		})
		for _, roleID := range added[ouID] {
			if slices.Contains(granted, roleID) {
				continue
			}
			// Mandating a role conveys its permissions as a direct assignment does; removals are never checked.
			if svcErr := as.authzService.CanGrantMembership(ctx, sysauthz.PrincipalTypeRole, roleID); svcErr != nil {
				return svcErr, nil
			}
			granted = append(granted, roleID)
		}
	}

	for _, ouID := range ouIDs {
		toAdd := added[ouID]
		// The store removes a policy's assignments one organization unit at a time, so dropping any role
		// rewrites the unit.
		if slices.ContainsFunc(held[ouID], func(r string) bool { return !slices.Contains(wanted[ouID], r) }) {
			if err := as.roleStore.DeleteMandatedAssignments(ctx, policyID, MandatedFilter{OUID: ouID}); err != nil {
				return nil, err
			}
			toAdd = wanted[ouID]
		}
		for _, roleID := range toAdd {
			if err := as.roleStore.AddMandatedAssignments(ctx, policyID, roleID, ouID, assignee); err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
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

// HasGroupRoleAssignments reports whether the group is assigned any role.
func (as *roleAssignmentService) HasGroupRoleAssignments(
	ctx context.Context, groupID string) (bool, *tidcommon.ServiceError) {
	found, err := as.roleStore.HasAssigneeAssignments(ctx, string(AssigneeTypeGroup), groupID)
	if err != nil {
		as.logger.Error(ctx, "Failed to check the group's role assignments",
			log.String("groupID", groupID), log.Error(err))
		return false, &tidcommon.InternalServerError
	}
	return found, nil
}

// assignmentFilterQuery returns the filter an assignment list was narrowed by, for its pagination
// links to carry.
func assignmentFilterQuery(ouID string) string {
	if ouID == "" {
		return ""
	}
	return utils.FilterQueryParam(map[string]interface{}{filter.AttributeOUID: ouID})
}

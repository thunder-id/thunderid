// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"sort"
	"strconv"

	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/group"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/log"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// memberPageSize and assignmentPageSize bound one page of a group's members and of a role's assignees
// while expanding. Both must track serverconst.MaxPageSize: a larger value is refused, not clamped.
const (
	memberPageSize     = serverconst.MaxPageSize
	assignmentPageSize = serverconst.MaxPageSize
)

// maxGroupDescentDepth bounds how far a group expansion descends into nested groups. It matches the
// ceiling the group store applies to the upward walk, so the two directions agree on what counts as
// too deeply nested to serve.
const maxGroupDescentDepth = 32

// AdminProviderInterface is the role and group surface the administration flows act through. One
// interface covers both because one object serves both: a group's scopes are the scopes of the roles it
// holds, so the resolver that answers for a role answers for a group too.
//
// The flow executors declare their own narrower views of this surface, so no executor depends on more
// of it than it uses.
type AdminProviderInterface interface {
	// ValidateRemoveRoleAssignment reports whether the assignment may be removed, and returns what a
	// revocation against it needs. It changes no state.
	ValidateRemoveRoleAssignment(ctx context.Context, roleID, assigneeID string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError)
	// RemoveRoleAssignment unassigns the role from the assignee.
	RemoveRoleAssignment(ctx context.Context, roleID, assigneeID string) *tidcommon.ServiceError
	// ValidateRoleScopeChange reports whether the role may be changed in a way that takes scopes away
	// from everyone holding it, and returns what a revocation against it needs. It changes no state.
	//
	// One validation serves both the deletion and the permission edit. They refuse the same roles, and
	// both revoke every scope the role grants today rather than the delta, so the target is the same
	// either way.
	ValidateRoleScopeChange(ctx context.Context, roleID string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError)
	// DeleteRole deletes the role, which unassigns it from everyone holding it.
	DeleteRole(ctx context.Context, roleID string) *tidcommon.ServiceError
	// UpdateRolePermissions replaces the permissions the role grants, leaving its other attributes as
	// they are. A caller that names no permissions leaves the role granting nothing.
	UpdateRolePermissions(ctx context.Context, roleID string,
		permissions []revocation.RolePermissions) *tidcommon.ServiceError
	// ValidateGroupMembershipChange reports whether the membership change may be made, and returns what
	// a revocation against it needs. It changes no state.
	//
	// An empty memberID means the group itself is going away, so every transitive member loses the path
	// through it. A named member means only that member does.
	ValidateGroupMembershipChange(ctx context.Context, groupID, memberID string) (
		*revocation.ScopeRevocationTarget, *tidcommon.ServiceError)
	// DeleteGroup deletes the group, which detaches its members and its own memberships.
	DeleteGroup(ctx context.Context, groupID string) *tidcommon.ServiceError
	// RemoveGroupMember removes one member from the group. The member's type is resolved by the
	// implementation, so a caller names a principal rather than a principal and its kind.
	RemoveGroupMember(ctx context.Context, groupID, memberID string) *tidcommon.ServiceError
}

// adminProvider serves the role and group operations an administration flow performs. It composes the
// role, assignment, group and resource services because planning a revocation needs the permissions
// being cut, the resource servers that give them meaning, and the principals behind a group.
type adminProvider struct {
	roleService       RoleServiceInterface
	assignmentService RoleAssignmentServiceInterface
	groupService      group.GroupServiceInterface
	entityService     entity.EntityServiceInterface
	resourceService   resourcepkg.ResourceServiceInterface
	logger            *log.Logger
}

var _ AdminProviderInterface = (*adminProvider)(nil)

// newAdminProvider creates the provider backing the role and group administration flows.
func newAdminProvider(
	roleService RoleServiceInterface,
	assignmentService RoleAssignmentServiceInterface,
	groupService group.GroupServiceInterface,
	entityService entity.EntityServiceInterface,
	resourceService resourcepkg.ResourceServiceInterface,
) AdminProviderInterface {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, loggerComponentName))
	return &adminProvider{
		roleService:       roleService,
		assignmentService: assignmentService,
		groupService:      groupService,
		entityService:     entityService,
		resourceService:   resourceService,
		logger:            logger,
	}
}

// ValidateRemoveRoleAssignment reports whether the assignment may be removed and returns the
// principals and scopes a revocation against it must cover. It changes no state.
//
// The scopes are every permission the role carries, not the delta against what the assignee keeps
// through another role: resolving that hypothetical state risks under-revoking, the unsafe direction.
// The revocation is bounded, so a principal who keeps a scope another way just refreshes once.
func (p *adminProvider) ValidateRemoveRoleAssignment(ctx context.Context, roleID, assigneeID string) (
	*revocation.ScopeRevocationTarget, *tidcommon.ServiceError) {
	if roleID == "" {
		return nil, &ErrorMissingRoleID
	}
	if assigneeID == "" {
		return nil, &ErrorMissingAssigneeID
	}

	role, svcErr := p.roleService.GetRoleWithPermissions(ctx, roleID)
	if svcErr != nil {
		return nil, svcErr
	}

	// Refuse an assignee that does not hold the role, for the reason ValidateGroupMembershipChange
	// refuses a non-member. RemoveAssignments issues a DELETE and never reports whether a row matched,
	// so removing an assignment that was never there is a silent no-op. Without this check the flow
	// would deny the principal every scope the role grants while changing nothing, report success, and
	// leave nothing to restore those tokens.
	assigned, svcErr := p.isAssigned(ctx, roleID, assigneeID)
	if svcErr != nil {
		return nil, svcErr
	}
	if !assigned {
		return nil, &ErrorRoleAssignmentNotFound
	}

	scopes, svcErr := p.resolveScopes(ctx, role.Permissions)
	if svcErr != nil {
		return nil, svcErr
	}
	entityIDs, svcErr := p.resolveAssignees(ctx, assigneeID)
	if svcErr != nil {
		return nil, svcErr
	}

	return &revocation.ScopeRevocationTarget{EntityIDs: entityIDs, Scopes: scopes}, nil
}

// isAssigned reports whether the assignee holds the role in its own right.
//
// Only a direct assignment counts, for the reason isDirectMember counts only direct membership: a
// principal holding the role through a group keeps that path, and this removal would not cut it.
func (p *adminProvider) isAssigned(ctx context.Context, roleID, assigneeID string) (
	bool, *tidcommon.ServiceError) {
	for offset := 0; ; offset += assignmentPageSize {
		page, svcErr := p.assignmentService.GetRoleAssignments(ctx, roleID, assignmentPageSize, offset, false)
		if svcErr != nil {
			return false, svcErr
		}
		for _, assignment := range page.Assignments {
			if assignment.ID == assigneeID {
				return true, nil
			}
		}
		if offset+assignmentPageSize >= page.TotalResults {
			return false, nil
		}
	}
}

// RemoveRoleAssignment unassigns the role from the assignee.
//
// The assignee type is resolved here rather than taken as a flow input: a caller names a principal,
// and a mismatched pair would be refused for something the operator could not act on.
func (p *adminProvider) RemoveRoleAssignment(ctx context.Context, roleID,
	assigneeID string) *tidcommon.ServiceError {
	assigneeType, svcErr := p.resolveAssigneeType(ctx, assigneeID)
	if svcErr != nil {
		p.logger.Error(ctx, "Failed to resolve assignee type for role assignment removal",
			log.String("roleId", roleID), log.String("code", svcErr.Code))
		return svcErr
	}
	if svcErr := p.assignmentService.RemoveAssignments(ctx, roleID,
		[]RoleAssignment{{ID: assigneeID, Type: assigneeType}}); svcErr != nil {
		p.logger.Error(ctx, "Failed to remove role assignment",
			log.String("roleId", roleID), log.String("assigneeType", string(assigneeType)),
			log.String("code", svcErr.Code),
			log.String("reason", svcErr.Error.DefaultValue))
		return svcErr
	}
	return nil
}

// resolveAssigneeType reports whether the assignee is a group or an entity, and for an entity which
// category it belongs to. Assignment validation accepts only the public types, so the internal
// storage type is never produced here.
func (p *adminProvider) resolveAssigneeType(ctx context.Context, assigneeID string) (
	AssigneeType, *tidcommon.ServiceError) {
	if _, svcErr := p.groupService.GetGroup(ctx, assigneeID, false); svcErr == nil {
		return AssigneeTypeGroup, nil
	} else if svcErr.Code != group.ErrorGroupNotFound.Code {
		return "", svcErr
	}

	target, err := p.entityService.GetEntity(ctx, assigneeID)
	if err != nil || target == nil {
		return "", &ErrorInvalidAssigneeType
	}
	switch target.Category {
	case providers.EntityCategoryUser:
		return AssigneeTypeUser, nil
	case providers.EntityCategoryApp:
		return AssigneeTypeApp, nil
	case providers.EntityCategoryAgent:
		return AssigneeTypeAgent, nil
	default:
		return "", &ErrorInvalidAssigneeType
	}
}

// resolveScopes pairs every permission the role carries with the resource server that defines it. The
// pairing matters because a permission is unique only within its server: "license" on two servers is
// two scopes. The audience is the identifier, the value a token carries in aud.
func (p *adminProvider) resolveScopes(ctx context.Context, permissions []ResourcePermissions) (
	[]revocation.AudienceScope, *tidcommon.ServiceError) {
	scopes := make([]revocation.AudienceScope, 0, len(permissions))
	for _, resource := range permissions {
		if len(resource.Permissions) == 0 {
			continue
		}
		resourceServer, svcErr := p.resourceService.GetResourceServer(ctx, resource.ResourceServerID)
		if svcErr != nil {
			return nil, svcErr
		}
		if resourceServer == nil || resourceServer.Identifier == "" {
			continue
		}
		for _, permission := range resource.Permissions {
			scopes = append(scopes, revocation.AudienceScope{
				Audience: resourceServer.Identifier,
				Scope:    permission,
			})
		}
	}
	return scopes, nil
}

// resolveAssignees expands the assignee into the principals whose tokens carry the role's scopes.
//
// An entity is itself. A group is its members, because the group holds no tokens and its members do.
// An assignee that resolves to neither yields an empty list, which the caller reports as nothing to
// revoke rather than treating as a failure.
func (p *adminProvider) resolveAssignees(ctx context.Context, assigneeID string) (
	[]string, *tidcommon.ServiceError) {
	members, svcErr := p.groupMemberIDs(ctx, assigneeID)
	if svcErr != nil {
		return nil, svcErr
	}
	if members != nil {
		return members, nil
	}
	return []string{assigneeID}, nil
}

// groupMemberIDs returns the entity ids belonging to the group, or nil when the assignee is not a
// group. A group that exists but holds no members returns an empty, non-nil slice.
//
// Membership is a graph, not a tree: cycles and shared nested members are both possible, so the
// descent carries a visited set and a depth bound like the group store's upward walk.
func (p *adminProvider) groupMemberIDs(ctx context.Context, assigneeID string) (
	[]string, *tidcommon.ServiceError) {
	if _, svcErr := p.groupService.GetGroup(ctx, assigneeID, false); svcErr != nil {
		if svcErr.Code == group.ErrorGroupNotFound.Code {
			return nil, nil
		}
		return nil, svcErr
	}

	entityIDs := []string{}
	seenEntities := map[string]struct{}{}
	visitedGroups := map[string]struct{}{assigneeID: {}}
	// Breadth-first rather than recursive, so the depth bound is a loop count and a cycle cannot grow
	// the call stack at all.
	frontier := []string{assigneeID}

	for depth := 0; depth < maxGroupDescentDepth && len(frontier) > 0; depth++ {
		next := []string{}
		for _, groupID := range frontier {
			members, svcErr := p.groupMembersPage(ctx, groupID)
			if svcErr != nil {
				return nil, svcErr
			}
			for _, member := range members {
				if member.Type == group.MemberTypeGroup {
					if _, seen := visitedGroups[member.ID]; seen {
						continue
					}
					visitedGroups[member.ID] = struct{}{}
					next = append(next, member.ID)
					continue
				}
				if _, seen := seenEntities[member.ID]; seen {
					continue
				}
				seenEntities[member.ID] = struct{}{}
				entityIDs = append(entityIDs, member.ID)
			}
		}
		frontier = next
	}

	if len(frontier) > 0 {
		return nil, ErrorGroupNestingTooDeep.WithParams(
			map[string]string{"max": strconv.Itoa(maxGroupDescentDepth)})
	}
	return entityIDs, nil
}

// groupMembersPage reads every member of one group, following the listing's pagination.
func (p *adminProvider) groupMembersPage(ctx context.Context, groupID string) (
	[]group.Member, *tidcommon.ServiceError) {
	members := []group.Member{}
	for offset := 0; ; offset += memberPageSize {
		page, svcErr := p.groupService.GetGroupMembers(ctx, groupID, memberPageSize, offset, false)
		if svcErr != nil {
			return nil, svcErr
		}
		members = append(members, page.Members...)
		if offset+memberPageSize >= page.TotalResults {
			break
		}
	}
	return members, nil
}

// ValidateRoleScopeChange reports whether the role may be changed in a way that takes scopes away from
// everyone holding it, and returns what a revocation against it must cover. It changes no state.
//
// Both callers, the deletion and the permission edit, revoke every scope the role grants today rather
// than the delta, for the reason ValidateRemoveRoleAssignment does.
func (p *adminProvider) ValidateRoleScopeChange(ctx context.Context, roleID string) (
	*revocation.ScopeRevocationTarget, *tidcommon.ServiceError) {
	if roleID == "" {
		return nil, &ErrorMissingRoleID
	}

	role, svcErr := p.roleService.GetRoleWithPermissions(ctx, roleID)
	if svcErr != nil {
		return nil, svcErr
	}
	// A declarative role can be neither deleted nor edited. Refusing here rather than letting the
	// acting node refuse keeps the flow from denying scopes for a change that was never going to
	// happen: the revocation runs first, and over-revoking on a guaranteed failure is pure cost.
	isDeclarative, svcErr := p.roleService.IsRoleDeclarative(ctx, roleID)
	if svcErr != nil {
		return nil, svcErr
	}
	if isDeclarative {
		return nil, &ErrorImmutableRole
	}

	scopes, svcErr := p.resolveScopes(ctx, role.Permissions)
	if svcErr != nil {
		return nil, svcErr
	}
	entityIDs, svcErr := p.resolveRoleAssignees(ctx, roleID)
	if svcErr != nil {
		return nil, svcErr
	}

	return &revocation.ScopeRevocationTarget{EntityIDs: entityIDs, Scopes: scopes}, nil
}

// DeleteRole deletes the role, which unassigns it from every principal holding it.
func (p *adminProvider) DeleteRole(ctx context.Context, roleID string) *tidcommon.ServiceError {
	if svcErr := p.roleService.DeleteRole(ctx, roleID); svcErr != nil {
		p.logger.Error(ctx, "Failed to delete role", log.String("roleId", roleID),
			log.String("code", svcErr.Code), log.String("reason", svcErr.Error.DefaultValue))
		return svcErr
	}
	return nil
}

// UpdateRolePermissions replaces the permissions the role grants and leaves its other attributes as
// they are.
//
// The other attributes are read here rather than carried through the flow. The role service's update
// takes a whole role, so a flow that made the caller restate the name and organization unit in order
// to change a permission would silently reset them whenever the caller left them out.
func (p *adminProvider) UpdateRolePermissions(ctx context.Context, roleID string,
	permissions []revocation.RolePermissions) *tidcommon.ServiceError {
	role, svcErr := p.roleService.GetRoleWithPermissions(ctx, roleID)
	if svcErr != nil {
		p.logger.Error(ctx, "Failed to read role for permission update",
			log.String("roleId", roleID), log.String("code", svcErr.Code))
		return svcErr
	}

	updated := make([]ResourcePermissions, 0, len(permissions))
	for _, permission := range permissions {
		updated = append(updated, ResourcePermissions{
			ResourceServerID: permission.ResourceServerID,
			Permissions:      permission.Permissions,
		})
	}

	if _, svcErr := p.roleService.UpdateRoleWithPermissions(ctx, roleID, RoleUpdateDetail{
		Name:        role.Name,
		Description: role.Description,
		OUID:        role.OUID,
		Permissions: updated,
	}); svcErr != nil {
		p.logger.Error(ctx, "Failed to update role permissions", log.String("roleId", roleID),
			log.String("code", svcErr.Code), log.String("reason", svcErr.Error.DefaultValue))
		return svcErr
	}
	return nil
}

// ValidateGroupMembershipChange reports whether the membership change may be made and returns the
// principals and scopes a revocation against it must cover. It changes no state.
//
// The scopes cover roles held by the group and by its ancestors, since membership conveys both. The
// principals are whoever loses that path: every transitive member for a group deletion (empty
// memberID), or the one departing member, itself expanded when it is a nested group.
func (p *adminProvider) ValidateGroupMembershipChange(ctx context.Context, groupID, memberID string) (
	*revocation.ScopeRevocationTarget, *tidcommon.ServiceError) {
	if groupID == "" {
		return nil, &group.ErrorMissingGroupID
	}

	targetGroup, svcErr := p.groupService.GetGroup(ctx, groupID, false)
	if svcErr != nil {
		return nil, svcErr
	}
	// As with a declarative role, refuse before revoking rather than after.
	if targetGroup.IsReadOnly {
		return nil, &group.ErrorImmutableGroup
	}

	scopes, svcErr := p.resolveGroupScopes(ctx, groupID)
	if svcErr != nil {
		return nil, svcErr
	}

	losing := memberID
	if losing == "" {
		losing = groupID
	} else {
		// Refuse a principal that is not in the group. The removal itself is a silent no-op for a
		// non-member, so without this check the flow would deny that principal every scope the group
		// conveys while changing nothing, and report success. Nothing would restore those tokens.
		isMember, svcErr := p.isDirectMember(ctx, groupID, memberID)
		if svcErr != nil {
			return nil, svcErr
		}
		if !isMember {
			return nil, &group.ErrorInvalidMemberID
		}
	}
	entityIDs, svcErr := p.resolveAssignees(ctx, losing)
	if svcErr != nil {
		return nil, svcErr
	}

	return &revocation.ScopeRevocationTarget{EntityIDs: entityIDs, Scopes: scopes}, nil
}

// isDirectMember reports whether the principal is a member of the group in its own right.
//
// Only direct membership counts: that is what a removal can actually cut. A principal nested inside a
// member group keeps its path through that group, so revoking for it would be over-revocation with no
// corresponding change.
func (p *adminProvider) isDirectMember(ctx context.Context, groupID, memberID string) (
	bool, *tidcommon.ServiceError) {
	members, svcErr := p.groupMembersPage(ctx, groupID)
	if svcErr != nil {
		return false, svcErr
	}
	for _, member := range members {
		if member.ID == memberID {
			return true, nil
		}
	}
	return false, nil
}

// DeleteGroup deletes the group, which detaches its members and its own memberships.
func (p *adminProvider) DeleteGroup(ctx context.Context, groupID string) *tidcommon.ServiceError {
	if svcErr := p.groupService.DeleteGroup(ctx, groupID); svcErr != nil {
		p.logger.Error(ctx, "Failed to delete group", log.String("groupId", groupID),
			log.String("code", svcErr.Code), log.String("reason", svcErr.Error.DefaultValue))
		return svcErr
	}
	return nil
}

// RemoveGroupMember removes one member from the group.
//
// The member's type is resolved here rather than taken as a flow input, for the reason
// RemoveRoleAssignment resolves an assignee's: the caller names a principal, and a mismatched pair
// would be rejected by membership validation for something the operator could not act on.
func (p *adminProvider) RemoveGroupMember(ctx context.Context, groupID,
	memberID string) *tidcommon.ServiceError {
	if memberID == "" {
		return &group.ErrorInvalidMemberID
	}

	assigneeType, svcErr := p.resolveAssigneeType(ctx, memberID)
	if svcErr != nil {
		p.logger.Error(ctx, "Failed to resolve member type for group membership removal",
			log.String("groupId", groupID), log.String("code", svcErr.Code))
		return svcErr
	}

	if _, svcErr := p.groupService.RemoveGroupMembers(ctx, groupID,
		[]group.Member{{ID: memberID, Type: group.MemberType(assigneeType)}}); svcErr != nil {
		p.logger.Error(ctx, "Failed to remove group member", log.String("groupId", groupID),
			log.String("memberType", string(assigneeType)), log.String("code", svcErr.Code),
			log.String("reason", svcErr.Error.DefaultValue))
		return svcErr
	}
	return nil
}

// resolveGroupScopes returns the scopes membership of the group conveys: every permission held by the
// group or by one of its ancestor groups, paired with the resource server that defines it.
//
// The role service answers this in one deduplicated call rather than walking each role. Resource
// servers are sorted so the same state twice builds criteria in the same order.
func (p *adminProvider) resolveGroupScopes(ctx context.Context, groupID string) (
	[]revocation.AudienceScope, *tidcommon.ServiceError) {
	ancestors, svcErr := p.groupService.GetTransitiveAncestorGroups(ctx, groupID)
	if svcErr != nil {
		return nil, svcErr
	}
	groupIDs := append([]string{groupID}, ancestors...)

	permissionSet, svcErr := p.roleService.GetAllPermissions(ctx, "", groupIDs)
	if svcErr != nil {
		return nil, svcErr
	}

	resourceServerIDs := make([]string, 0, len(permissionSet))
	for resourceServerID := range permissionSet {
		resourceServerIDs = append(resourceServerIDs, resourceServerID)
	}
	sort.Strings(resourceServerIDs)

	permissions := make([]ResourcePermissions, 0, len(resourceServerIDs))
	for _, resourceServerID := range resourceServerIDs {
		permissions = append(permissions, ResourcePermissions{
			ResourceServerID: resourceServerID,
			Permissions:      permissionSet[resourceServerID],
		})
	}
	return p.resolveScopes(ctx, permissions)
}

// resolveRoleAssignees returns the entity ids of every principal holding the role, expanding a group
// assignee into its transitive members.
//
// The assignment's own type decides whether to expand, rather than a lookup: the assignment already
// records it, and trusting it saves a group read per entity assignee.
func (p *adminProvider) resolveRoleAssignees(ctx context.Context, roleID string) (
	[]string, *tidcommon.ServiceError) {
	entityIDs := []string{}
	seen := map[string]struct{}{}
	appendUnique := func(candidates []string) {
		for _, entityID := range candidates {
			if _, ok := seen[entityID]; ok {
				continue
			}
			seen[entityID] = struct{}{}
			entityIDs = append(entityIDs, entityID)
		}
	}

	for offset := 0; ; offset += assignmentPageSize {
		page, svcErr := p.assignmentService.GetRoleAssignments(ctx, roleID, assignmentPageSize, offset, false)
		if svcErr != nil {
			return nil, svcErr
		}
		for _, assignment := range page.Assignments {
			if assignment.Type != AssigneeTypeGroup {
				appendUnique([]string{assignment.ID})
				continue
			}
			members, svcErr := p.groupMemberIDs(ctx, assignment.ID)
			if svcErr != nil {
				return nil, svcErr
			}
			appendUnique(members)
		}
		if offset+assignmentPageSize >= page.TotalResults {
			break
		}
	}
	return entityIDs, nil
}

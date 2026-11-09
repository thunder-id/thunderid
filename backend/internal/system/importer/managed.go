// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package importer

import (
	"context"

	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/managedresource"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// withManagedImport marks the context of a managed import, whose writes may change what a control plane
// deployed and which every service otherwise refuses.
func withManagedImport(ctx context.Context, request *ImportRequest) context.Context {
	if request.Managed {
		return managedresource.WithManagedImport(ctx)
	}
	return ctx
}

// markManaged records a resource a managed import wrote as deployed by a control plane, which makes it
// read only here, under the type of the document it was written from: an outcome names a user type and
// an agent type alike as a user type. A resource the import wrote but could not mark is reported as failed:
// left unmarked it would stay editable, and the control plane deploys it again.
func (s *importService) markManaged(
	ctx context.Context, request *ImportRequest, resourceType string, outcome ImportItemOutcome,
) ImportItemOutcome {
	if s.managed == nil || !request.Managed || request.DryRun || outcome.Status != statusSuccess {
		return outcome
	}
	if err := s.managed.Mark(ctx, resourceType, managedID(outcome)); err != nil {
		log.GetLogger().Error(ctx, "Failed to mark an imported resource as managed",
			log.String("resourceType", resourceType), log.String("resourceID", outcome.ResourceID),
			log.Error(err))
		outcome.Status = statusFailed
		outcome.Code = tidcommon.InternalServerError.Code
		outcome.Message = "the resource was written but could not be marked as deployed by the control plane"
	}
	return outcome
}

// refuseUnmarkedAdoption refuses a managed import taking over a flow it found by handle that no
// control plane deployed. It would be marked under this deployment's id, and a delete the control plane
// sends names the control plane's own id, so the resource could never be removed or unmarked again. Outside
// a managed import it does nothing.
func (s *importService) refuseUnmarkedAdoption(
	ctx context.Context, id string) *tidcommon.ServiceError {
	if s.managed == nil || !managedresource.IsManagedImport(ctx) {
		return nil
	}
	deployed, err := s.managed.IsManaged(ctx, id, managedresource.TypeFlow)
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to read whether a control plane deployed the resource",
			log.String("resourceID", id), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if !deployed {
		return &ErrorUnmanagedHandleMatch
	}
	return nil
}

// managedID is what a resource is marked by: its id, or its name for a resource that has none.
func managedID(outcome ImportItemOutcome) string {
	if outcome.ResourceID != "" {
		return outcome.ResourceID
	}
	return outcome.ResourceName
}

// managedPartsOf reads a deletion of the members of a group, or the assignees of a role, as the resource
// those entries are recorded under, the collection, and their keys. A deletion of anything else is not one.
func managedPartsOf(deletion ResourceDeletion) (resourceType, part string, keys []string, ok bool) {
	switch deletion.ResourceType {
	case removalTypeGroupMember:
		resourceType, part = managedresource.TypeGroup, managedresource.PartMember
		for _, member := range deletion.Members {
			keys = append(keys, group.MemberKey(group.Member{ID: member.ID, Type: group.MemberType(member.Type)}))
		}
	case removalTypeRoleAssignment:
		resourceType, part = managedresource.TypeRole, managedresource.PartAssignment
		for _, member := range deletion.Members {
			keys = append(keys, role.AssignmentKey(
				role.RoleAssignment{ID: member.ID, Type: role.AssigneeType(member.Type), OUID: member.OUID}))
		}
	default:
		return "", "", nil, false
	}
	return resourceType, part, keys, true
}

// unmarkManaged forgets a resource a managed import removed. Only a managed import gets this far with
// a managed resource; any other import is refused it before the delete.
func (s *importService) unmarkManaged(
	ctx context.Context, deletion ResourceDeletion, outcome ImportItemOutcome) ImportItemOutcome {
	if s.managed == nil || !managedresource.IsManagedImport(ctx) {
		return outcome
	}
	if resourceType, part, keys, ok := managedPartsOf(deletion); ok {
		if err := s.managed.UnmarkParts(ctx, resourceType, deletion.ID, part, keys); err != nil {
			log.GetLogger().Error(ctx, "Failed to forget removed managed entries",
				log.String("resourceType", resourceType), log.String("resourceID", deletion.ID), log.Error(err))
			outcome.Status = statusFailed
			outcome.Code = tidcommon.InternalServerError.Code
			outcome.Message = "the entries were removed but their record as deployed by the control plane was not"
		}
		return outcome
	}
	if err := s.managed.Unmark(ctx, deletion.ResourceType, deletion.ID); err != nil {
		log.GetLogger().Error(ctx, "Failed to forget a removed managed resource",
			log.String("resourceType", deletion.ResourceType), log.String("resourceID", deletion.ID),
			log.Error(err))
		outcome.Status = statusFailed
		outcome.Code = tidcommon.InternalServerError.Code
		outcome.Message = "the resource was removed but its record as deployed by the control plane was not"
	}
	return outcome
}

// reconcileManagedMembers makes a group's members deployed by the control plane the ones the managed
// import sent: a member deployed before and not sent any more is removed, and every member sent is
// recorded as deployed. Members added on this deployment are left as they are. Outside a managed
// import it does nothing.
func (s *importService) reconcileManagedMembers(
	ctx context.Context, groupID string, sent []group.Member) *tidcommon.ServiceError {
	if s.managed == nil || !managedresource.IsManagedImport(ctx) {
		return nil
	}
	previous, err := s.managed.PartKeys(ctx, managedresource.TypeGroup, groupID, managedresource.PartMember)
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to read the members a control plane deployed to a group",
			log.String("groupID", groupID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	sentKeys := make([]string, 0, len(sent))
	for _, member := range sent {
		sentKeys = append(sentKeys, group.MemberKey(member))
	}
	sentSet, previousSet := keySet(sentKeys), keySet(previous)
	var stale []group.Member
	var staleKeys []string
	for _, key := range previous {
		if _, kept := sentSet[key]; kept {
			continue
		}
		member, ok := group.ParseMemberKey(key)
		if !ok {
			continue
		}
		stale = append(stale, member)
		staleKeys = append(staleKeys, key)
	}
	// One by one, so a member that no longer exists, as a user the same deploy removed does not, is
	// taken as already gone rather than failing the rest.
	for _, member := range stale {
		_, svcErr := s.groupService.RemoveGroupMembers(ctx, groupID, []group.Member{member})
		if svcErr != nil && svcErr.Code != group.ErrorInvalidMemberID.Code &&
			svcErr.Code != group.ErrorInvalidGroupMemberID.Code {
			return svcErr
		}
	}
	if err := s.managed.UnmarkParts(ctx, managedresource.TypeGroup, groupID, managedresource.PartMember,
		staleKeys); err != nil {
		log.GetLogger().Error(ctx, "Failed to forget the members a control plane removed from a group",
			log.String("groupID", groupID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if err := s.managed.MarkParts(ctx, managedresource.TypeGroup, groupID, managedresource.PartMember,
		keysNotIn(sentKeys, previousSet)); err != nil {
		log.GetLogger().Error(ctx, "Failed to record the members a control plane deployed to a group",
			log.String("groupID", groupID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// reconcileManagedAssignments makes a role's assignments deployed by the control plane the ones the
// managed import sent, each placed in the organization unit it is made in: an assignment deployed before
// and not sent any more is removed, and every one sent is recorded as deployed. Assignments made on this
// deployment are left as they are. Outside a managed import it does nothing.
func (s *importService) reconcileManagedAssignments(
	ctx context.Context, roleID string, sent []role.RoleAssignment) *tidcommon.ServiceError {
	if s.managed == nil || !managedresource.IsManagedImport(ctx) {
		return nil
	}
	previous, err := s.managed.PartKeys(ctx, managedresource.TypeRole, roleID, managedresource.PartAssignment)
	if err != nil {
		log.GetLogger().Error(ctx, "Failed to read the assignments a control plane deployed to a role",
			log.String("roleID", roleID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	sentKeys := make([]string, 0, len(sent))
	for _, a := range sent {
		sentKeys = append(sentKeys, role.AssignmentKey(a))
	}
	sentSet, previousSet := keySet(sentKeys), keySet(previous)
	var staleKeys []string
	for _, key := range previous {
		if _, kept := sentSet[key]; kept {
			continue
		}
		staleKeys = append(staleKeys, key)
		stale, ok := role.ParseAssignmentKey(key)
		if !ok {
			continue
		}
		// One by one, so an assignee that no longer exists, as one the same deploy removed does not, is
		// taken as already unassigned rather than failing the rest.
		svcErr := s.roleAssignmentService.RemoveAssignments(ctx, roleID, stale.OUID,
			[]role.RoleAssignment{{ID: stale.ID, Type: stale.Type}})
		if svcErr != nil && svcErr.Code != role.ErrorInvalidAssignmentID.Code &&
			svcErr.Code != role.ErrorRoleAssignmentNotFound.Code {
			return svcErr
		}
	}
	if err := s.managed.UnmarkParts(ctx, managedresource.TypeRole, roleID, managedresource.PartAssignment,
		staleKeys); err != nil {
		log.GetLogger().Error(ctx, "Failed to forget the assignments a control plane removed from a role",
			log.String("roleID", roleID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	if err := s.managed.MarkParts(ctx, managedresource.TypeRole, roleID, managedresource.PartAssignment,
		keysNotIn(sentKeys, previousSet)); err != nil {
		log.GetLogger().Error(ctx, "Failed to record the assignments a control plane deployed to a role",
			log.String("roleID", roleID), log.Error(err))
		return &tidcommon.InternalServerError
	}
	return nil
}

// keySet makes a set of the keys.
func keySet(keys []string) map[string]struct{} {
	set := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		set[key] = struct{}{}
	}
	return set
}

// keysNotIn lists the keys that are not in the set, which are the entries not recorded as deployed before.
func keysNotIn(keys []string, set map[string]struct{}) []string {
	var absent []string
	for _, key := range keys {
		if _, ok := set[key]; !ok {
			absent = append(absent, key)
		}
	}
	return absent
}

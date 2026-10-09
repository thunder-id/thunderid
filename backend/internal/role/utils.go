// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package role

import (
	"context"
	"strconv"

	"github.com/thunder-id/thunderid/internal/group"
	resourcepkg "github.com/thunder-id/thunderid/internal/resource"
	"github.com/thunder-id/thunderid/internal/revocation"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// memberPageSize and assignmentPageSize must not exceed serverconst.MaxPageSize, which is refused, not clamped.
const (
	memberPageSize     = serverconst.MaxPageSize
	assignmentPageSize = serverconst.MaxPageSize
)

// maxGroupDescentDepth bounds a group expansion, matching the group store's upward walk.
const maxGroupDescentDepth = 32

func resolveScopes(ctx context.Context, resourceService resourcepkg.ResourceServiceInterface,
	permissions []ResourcePermissions) ([]revocation.AudienceScope, *tidcommon.ServiceError) {
	scopes := make([]revocation.AudienceScope, 0, len(permissions))
	for _, resource := range permissions {
		if len(resource.Permissions) == 0 {
			continue
		}
		resourceServer, svcErr := resourceService.GetResourceServer(ctx, resource.ResourceServerID)
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

func expandPrincipal(ctx context.Context, groupService group.GroupServiceInterface, principalID string,
	isGroup bool) ([]string, *tidcommon.ServiceError) {
	if !isGroup {
		return []string{principalID}, nil
	}
	return groupMemberIDs(ctx, groupService, principalID)
}

// groupMemberIDs returns the distinct entity IDs reachable from the group, tolerating cycles.
func groupMemberIDs(ctx context.Context, groupService group.GroupServiceInterface, groupID string) (
	[]string, *tidcommon.ServiceError) {
	entityIDs := []string{}
	seenEntities := map[string]struct{}{}
	visitedGroups := map[string]struct{}{groupID: {}}
	frontier := []string{groupID}

	for depth := 0; depth < maxGroupDescentDepth && len(frontier) > 0; depth++ {
		next := []string{}
		for _, current := range frontier {
			members, svcErr := groupMembers(ctx, groupService, current)
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

func groupMembers(ctx context.Context, groupService group.GroupServiceInterface, groupID string) (
	[]group.Member, *tidcommon.ServiceError) {
	members := []group.Member{}
	for offset := 0; ; offset += memberPageSize {
		page, svcErr := groupService.GetGroupMembers(ctx, groupID, memberPageSize, offset, false)
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

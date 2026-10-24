// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package engine provides authorization engine implementations.
// It includes various authorization engines such as RBAC (Role-Based Access Control)
// that delegate authorization decisions to the appropriate services.
package engine

import (
	"context"
	"fmt"
	"slices"

	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/role"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// newRBACEngine creates an RBAC authorization engine.
func newRBACEngine(
	roleService role.RoleServiceInterface, groupService group.GroupServiceInterface,
) AuthorizationEngine {
	return &rbacEngine{roleService: roleService, groupService: groupService}
}

// EvaluateAccess evaluates a single fine-grained access request.
func (e *rbacEngine) EvaluateAccess(
	ctx context.Context,
	request AccessEvaluationRequest,
) (*AccessEvaluationResponse, error) {
	response, err := e.EvaluateAccessBatch(ctx, AccessEvaluationsRequest{
		Evaluations: []AccessEvaluationRequest{request},
	})
	if err != nil {
		return nil, err
	}
	if len(response.Evaluations) == 0 {
		return &AccessEvaluationResponse{}, nil
	}
	return &response.Evaluations[0], nil
}

// EvaluateAccessBatch evaluates multiple fine-grained access requests based on role assignments.
func (e *rbacEngine) EvaluateAccessBatch(
	ctx context.Context,
	request AccessEvaluationsRequest,
) (*AccessEvaluationsResponse, error) {
	if len(request.Evaluations) == 0 {
		return &AccessEvaluationsResponse{Evaluations: []AccessEvaluationResponse{}}, nil
	}

	evaluations := make([]AccessEvaluationResponse, len(request.Evaluations))
	for _, evalGroup := range groupEvaluations(request.Evaluations) {
		authorizedPerms, err := e.authorizedPermissions(ctx, evalGroup)
		if err != nil {
			return nil, err
		}

		for _, index := range evalGroup.indexes {
			permission := request.Evaluations[index].Permission.Name
			evaluations[index] = AccessEvaluationResponse{
				Decision: slices.Contains(authorizedPerms, permission),
			}
		}
	}
	return &AccessEvaluationsResponse{Evaluations: evaluations}, nil
}

// authorizedPermissions returns the evaluation group's permissions that the subject holds through
// roles owned by the organization unit the evaluation is bounded to.
func (e *rbacEngine) authorizedPermissions(ctx context.Context, evalGroup evaluationGroup) ([]string, error) {
	if evalGroup.ouID == "" {
		// A subject whose organization unit could not be resolved holds no role.
		if evalGroup.subject.ID != "" {
			return []string{}, nil
		}
		return e.permissionsForMappedSubject(ctx, evalGroup)
	}
	return e.permissionsInOU(ctx, evalGroup, evalGroup.subject.ID, evalGroup.subject.GroupIDs,
		evalGroup.subject.RoleIDs, evalGroup.ouID)
}

// permissionsForMappedSubject returns the evaluation group's permissions for a subject known only by
// its roles and groups, such as a federated identity known only by what its connection's
// authorization mapping resolved. With no organization unit of its own, each named role counts in the
// organization unit that owns it, and each group counts only the roles owned by the group's
// organization unit.
//
// TODO: Update once roles and connections can be shared across organization units. A connection
// bound to an organization unit must then restrict its authorization mapping rules to the roles and
// groups available in that unit, and a shared role must count in the units it is shared to.
func (e *rbacEngine) permissionsForMappedSubject(
	ctx context.Context, evalGroup evaluationGroup,
) ([]string, error) {
	perms := []string{}
	if len(evalGroup.subject.RoleIDs) > 0 {
		// No organization unit leaves each named role bounded to the one that owns it.
		rolePerms, err := e.permissionsInOU(ctx, evalGroup, "", nil, evalGroup.subject.RoleIDs, "")
		if err != nil {
			return nil, err
		}
		perms = rolePerms
	}
	if len(evalGroup.subject.GroupIDs) == 0 {
		return perms, nil
	}

	groups, svcErr := e.groupService.GetGroupsByIDs(ctx, evalGroup.subject.GroupIDs)
	if svcErr != nil {
		return nil, fmt.Errorf("group service error: %s", svcErr.Error)
	}
	ouIDs := make([]string, 0, len(groups))
	groupIDsByOU := make(map[string][]string, len(groups))
	for _, groupID := range evalGroup.subject.GroupIDs {
		grp, ok := groups[groupID]
		if !ok || grp.OUID == "" {
			continue
		}
		if _, seen := groupIDsByOU[grp.OUID]; !seen {
			ouIDs = append(ouIDs, grp.OUID)
		}
		groupIDsByOU[grp.OUID] = append(groupIDsByOU[grp.OUID], groupID)
	}
	for _, ouID := range ouIDs {
		groupPerms, err := e.permissionsInOU(ctx, evalGroup, "", groupIDsByOU[ouID], nil, ouID)
		if err != nil {
			return nil, err
		}
		perms = utils.MergeUniqueStrings(perms, groupPerms)
	}
	return perms, nil
}

// permissionsInOU returns the evaluation group's permissions the entity, groups, or roles hold through
// roles owned by the organization unit. An empty organization unit leaves each role in the one that
// owns it.
func (e *rbacEngine) permissionsInOU(
	ctx context.Context, evalGroup evaluationGroup, entityID string, groupIDs, roleIDs []string, ouID string,
) ([]string, error) {
	perms, svcErr := e.roleService.GetAuthorizedPermissionsByResourceServer(
		ctx, entityID, groupIDs, roleIDs, evalGroup.resourceServerID, evalGroup.permissions, ouID)
	if svcErr != nil {
		return nil, fmt.Errorf("role service error: %s", svcErr.Error)
	}
	return perms, nil
}

// groupEvaluations groups access evaluations by subject, resource server and organization unit, and
// collects unique permissions.
func groupEvaluations(evaluations []AccessEvaluationRequest) []evaluationGroup {
	evalGroups := make([]evaluationGroup, 0, len(evaluations))
	for index, evaluation := range evaluations {
		evalGroupIndex := findEvaluationGroup(
			evalGroups, evaluation.Subject, evaluation.ResourceServer.ID, evaluation.OUID)
		if evalGroupIndex == -1 {
			evalGroups = append(evalGroups, evaluationGroup{
				subject:          evaluation.Subject,
				resourceServerID: evaluation.ResourceServer.ID,
				ouID:             evaluation.OUID,
			})
			evalGroupIndex = len(evalGroups) - 1
		}

		permission := evaluation.Permission.Name
		if !slices.Contains(evalGroups[evalGroupIndex].permissions, permission) {
			evalGroups[evalGroupIndex].permissions = append(evalGroups[evalGroupIndex].permissions, permission)
		}
		evalGroups[evalGroupIndex].indexes = append(evalGroups[evalGroupIndex].indexes, index)
	}
	return evalGroups
}

// findEvaluationGroup returns the index of the evaluation group matching the subject, resource server and
// organization unit.
func findEvaluationGroup(evalGroups []evaluationGroup, subject Subject, resourceServerID, ouID string) int {
	for i, evalGroup := range evalGroups {
		if evalGroup.subject.Category == subject.Category &&
			evalGroup.subject.ID == subject.ID &&
			slices.Equal(evalGroup.subject.GroupIDs, subject.GroupIDs) &&
			slices.Equal(evalGroup.subject.RoleIDs, subject.RoleIDs) &&
			evalGroup.resourceServerID == resourceServerID &&
			evalGroup.ouID == ouID {
			return i
		}
	}
	return -1
}

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/role"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type roleAction struct {
	reason revocation.Reason
	failed tidcommon.ServiceError
	act    func(ctx context.Context, e *roleExecutor, plan revocationPlan) (*tidcommon.ServiceError, error)
}

var roleActionsByMode = map[string]roleAction{
	ExecutorModeRemoveAssignment: {
		reason: revocation.ReasonRoleAssignmentRemoved,
		failed: ErrRoleAssignmentRemovalFailed,
		act: func(ctx context.Context, e *roleExecutor, plan revocationPlan) (*tidcommon.ServiceError, error) {
			if plan.AssigneeID == "" || plan.AssigneeType == "" {
				return nil, errors.New("trusted revocation plan has no target assignee")
			}
			return e.assignments.RemoveAssignments(ctx, plan.TargetID, []role.RoleAssignment{
				{ID: plan.AssigneeID, Type: role.AssigneeType(plan.AssigneeType)},
			}), nil
		},
	},
	ExecutorModeDelete: {
		reason: revocation.ReasonRoleDeleted,
		failed: ErrRoleDeletionFailed,
		act: func(ctx context.Context, e *roleExecutor, plan revocationPlan) (*tidcommon.ServiceError, error) {
			return e.roles.DeleteRole(ctx, plan.TargetID), nil
		},
	},
	ExecutorModeRemovePermissions: {
		reason: revocation.ReasonRolePermissionRemoved,
		failed: ErrRolePermissionRemovalFailed,
		act: func(ctx context.Context, e *roleExecutor, plan revocationPlan) (*tidcommon.ServiceError, error) {
			permissions, err := decodeRolePermissions(plan.ActionArgs[revocationInputPermissions])
			if err != nil {
				return nil, err
			}
			current, svcErr := e.roles.GetRoleWithPermissions(ctx, plan.TargetID)
			if svcErr != nil {
				return svcErr, nil
			}
			// The update replaces the whole role, so the attributes not being changed are carried over.
			_, svcErr = e.roles.UpdateRoleWithPermissions(ctx, plan.TargetID, role.RoleUpdateDetail{
				Name:        current.Name,
				Description: current.Description,
				OUID:        current.OUID,
				Permissions: permissions,
			})
			return svcErr, nil
		},
	},
}

// roleExecutor performs the role change named in the trusted revocation plan.
type roleExecutor struct {
	providers.Executor
	roles       roleAdminProvider
	assignments roleAssignmentAdminProvider
}

var _ providers.Executor = (*roleExecutor)(nil)

// newRoleExecutor creates the acting executor for the role administration flows.
func newRoleExecutor(factory core.FlowFactoryInterface, roles roleAdminProvider,
	assignments roleAssignmentAdminProvider) *roleExecutor {
	modes := make([]string, 0, len(roleActionsByMode))
	for mode := range roleActionsByMode {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	base := factory.CreateExecutor(ExecutorNameRole, providers.ExecutorTypeUtility, nil, nil,
		&providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
			SupportedModes:     modes,
		})
	return &roleExecutor{Executor: base, roles: roles, assignments: assignments}
}

// Execute performs the change identified by the trusted revocation plan.
func (e *roleExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	action, ok := roleActionsByMode[ctx.ExecutorMode]
	if !ok {
		return nil, fmt.Errorf("unsupported mode for %s: %q", ExecutorNameRole, ctx.ExecutorMode)
	}
	if e.roles == nil || e.assignments == nil {
		return nil, errors.New("role service is not configured")
	}
	plan, err := revocationPlanFor(ctx.SharedRuntimeData, action.reason)
	if err != nil {
		return nil, err
	}

	svcErr, err := action.act(ctx.Context, e, plan)
	if err != nil {
		return nil, err
	}
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to perform %s: %s", action.reason, svcErr.Error.DefaultValue)
		}
		refusal := *svcErr
		if refusal.Code == "" {
			refusal = action.failed
		}
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &refusal}, nil
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

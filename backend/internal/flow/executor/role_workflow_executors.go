// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// roleAction performs one administrative role change named by the trusted revocation plan.
type roleAction func(ctx context.Context, provider roleAdminProvider,
	plan revocationPlan) *tidcommon.ServiceError

// roleWorkflowExecutor performs the role change named in the trusted revocation plan. It runs after the
// scopes that change withdraws have been denied.
//
// One type backs all three role actions. They read the same plan and differ only in which reason the
// plan must carry and which provider call they make, so they are parameterized at construction. They
// stay separate registered executors so the designer palette names the action rather than hiding it in
// a property.
type roleWorkflowExecutor struct {
	providers.Executor
	provider roleAdminProvider
	reason   revocation.Reason
	failed   tidcommon.ServiceError
	act      roleAction
}

var _ providers.Executor = (*roleWorkflowExecutor)(nil)

// newRoleAssignmentRemovalExecutor creates an executor that performs the unassignment.
func newRoleAssignmentRemovalExecutor(factory core.FlowFactoryInterface,
	provider roleAdminProvider) *roleWorkflowExecutor {
	return newRoleWorkflowExecutor(factory, provider, ExecutorNameRoleAssignmentRemoval,
		revocation.ReasonRoleAssignmentRemoved, ErrRoleAssignmentRemovalFailed,
		func(ctx context.Context, p roleAdminProvider,
			plan revocationPlan) *tidcommon.ServiceError {
			if plan.AssigneeID == "" {
				// The assignee is half the target, and unlike the role it is not checked by
				// revocationPlanFor, which cannot know which actions name two resources.
				return internalFailure(errors.New("trusted revocation plan has no target assignee"))
			}
			return p.RemoveRoleAssignment(ctx, plan.TargetID, plan.AssigneeID)
		})
}

// newRoleDeletionExecutor creates an executor that deletes the role.
func newRoleDeletionExecutor(factory core.FlowFactoryInterface,
	provider roleAdminProvider) *roleWorkflowExecutor {
	return newRoleWorkflowExecutor(factory, provider, ExecutorNameRoleDeletion,
		revocation.ReasonRoleDeleted, ErrRoleDeletionFailed,
		func(ctx context.Context, p roleAdminProvider,
			plan revocationPlan) *tidcommon.ServiceError {
			return p.DeleteRole(ctx, plan.TargetID)
		})
}

// newRolePermissionRemovalExecutor creates an executor that replaces the permissions the role grants.
func newRolePermissionRemovalExecutor(factory core.FlowFactoryInterface,
	provider roleAdminProvider) *roleWorkflowExecutor {
	return newRoleWorkflowExecutor(factory, provider, ExecutorNameRolePermissionRemoval,
		revocation.ReasonRolePermissionRemoved, ErrRolePermissionRemovalFailed,
		func(ctx context.Context, p roleAdminProvider,
			plan revocationPlan) *tidcommon.ServiceError {
			permissions, err := decodeRolePermissions(plan.ActionArgs[revocationInputPermissions])
			if err != nil {
				return internalFailure(err)
			}
			return p.UpdateRolePermissions(ctx, plan.TargetID, permissions)
		})
}

// newRoleWorkflowExecutor creates an acting executor pinned to one role change.
func newRoleWorkflowExecutor(factory core.FlowFactoryInterface,
	provider roleAdminProvider, name string, reason revocation.Reason,
	failed tidcommon.ServiceError, act roleAction) *roleWorkflowExecutor {
	base := factory.CreateExecutor(name, providers.ExecutorTypeUtility, nil, nil,
		&providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &roleWorkflowExecutor{
		Executor: base,
		provider: provider,
		reason:   reason,
		failed:   failed,
		act:      act,
	}
}

// Execute performs the change identified by the trusted revocation plan.
func (e *roleWorkflowExecutor) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	if e.provider == nil {
		return nil, errors.New("role service is not configured")
	}
	plan, err := revocationPlanFor(ctx.SharedRuntimeData, e.reason)
	if err != nil {
		return nil, err
	}

	if svcErr := e.act(ctx.Context, e.provider, plan); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to perform %s: %s", e.reason, svcErr.Error.DefaultValue)
		}
		// Carry the service's own refusal, as the preparatory node does. An unknown assignee and an
		// assignment that was never there need different things from the operator, and collapsing them
		// into one code reports the wrong reason for all but one.
		refusal := *svcErr
		if refusal.Code == "" {
			refusal = e.failed
		}
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &refusal}, nil
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

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

// groupAction performs one administrative group change named by the trusted revocation plan.
type groupAction func(ctx context.Context, provider groupAdminProvider,
	plan revocationPlan) *tidcommon.ServiceError

// groupWorkflowExecutor performs the membership change named in the trusted revocation plan. It runs
// after the scopes that change withdraws have been denied.
//
// Both group actions record the same reason, so unlike the role executors the plan check cannot tell
// them apart. Each still checks what it needs from the plan: a member removal without a departing
// member is a mispaired graph, not an empty removal.
type groupWorkflowExecutor struct {
	providers.Executor
	provider groupAdminProvider
	failed   tidcommon.ServiceError
	act      groupAction
}

var _ providers.Executor = (*groupWorkflowExecutor)(nil)

// newGroupDeletionExecutor creates an executor that deletes the group.
func newGroupDeletionExecutor(factory core.FlowFactoryInterface,
	provider groupAdminProvider) *groupWorkflowExecutor {
	return newGroupWorkflowExecutor(factory, provider, ExecutorNameGroupDeletion, ErrGroupDeletionFailed,
		func(ctx context.Context, p groupAdminProvider,
			plan revocationPlan) *tidcommon.ServiceError {
			if plan.AssigneeID != "" {
				// A member removal's plan carries the same reason and the same group id, so the reason
				// check cannot tell the two apart. Deleting the group on a plan that named one departing
				// member would destroy the group and everyone else's membership with it, which is far
				// more than the graph was wired to do.
				return internalFailure(errors.New(
					"trusted revocation plan names a departing member, so it was produced for a " +
						"membership removal rather than a group deletion"))
			}
			return p.DeleteGroup(ctx, plan.TargetID)
		})
}

// newGroupMembershipRemovalExecutor creates an executor that removes one member from the group.
func newGroupMembershipRemovalExecutor(factory core.FlowFactoryInterface,
	provider groupAdminProvider) *groupWorkflowExecutor {
	return newGroupWorkflowExecutor(factory, provider, ExecutorNameGroupMembershipRemoval,
		ErrGroupMembershipRemovalFailed,
		func(ctx context.Context, p groupAdminProvider,
			plan revocationPlan) *tidcommon.ServiceError {
			if plan.AssigneeID == "" {
				// A group deletion plan carries the same reason and no member. Deleting the group here
				// instead of refusing would be a far larger change than the node was wired to make.
				return internalFailure(errors.New("trusted revocation plan has no departing member"))
			}
			return p.RemoveGroupMember(ctx, plan.TargetID, plan.AssigneeID)
		})
}

// newGroupWorkflowExecutor creates an acting executor pinned to one group change.
func newGroupWorkflowExecutor(factory core.FlowFactoryInterface,
	provider groupAdminProvider, name string, failed tidcommon.ServiceError,
	act groupAction) *groupWorkflowExecutor {
	base := factory.CreateExecutor(name, providers.ExecutorTypeUtility, nil, nil,
		&providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &groupWorkflowExecutor{Executor: base, provider: provider, failed: failed, act: act}
}

// Execute performs the change identified by the trusted revocation plan.
func (e *groupWorkflowExecutor) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	if e.provider == nil {
		return nil, errors.New("group service is not configured")
	}
	plan, err := revocationPlanFor(ctx.SharedRuntimeData, revocation.ReasonGroupMembershipRemoved)
	if err != nil {
		return nil, err
	}

	if svcErr := e.act(ctx.Context, e.provider, plan); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to change group membership: %s", svcErr.Error.DefaultValue)
		}
		// Carry the service's own refusal, as the preparatory node does: a declarative group, an unknown
		// member and a group that is simply gone need different things from the operator.
		refusal := *svcErr
		if refusal.Code == "" {
			refusal = e.failed
		}
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &refusal}, nil
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

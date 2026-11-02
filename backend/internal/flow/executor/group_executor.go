// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// groupAction is the group change made for one mode and the refusal reported when the service names none.
type groupAction struct {
	reason revocation.Reason
	failed tidcommon.ServiceError
	act    func(ctx context.Context, groups groupAdminProvider, plan revocationPlan) (
		*tidcommon.ServiceError, error)
}

// groupActionsByMode maps the node's mode to the group change it makes.
var groupActionsByMode = map[string]groupAction{
	ExecutorModeGroupDeletion: {
		reason: revocation.ReasonGroupDeleted,
		failed: ErrGroupDeletionFailed,
		act: func(ctx context.Context, groups groupAdminProvider, plan revocationPlan) (
			*tidcommon.ServiceError, error) {
			if plan.AssigneeID != "" {
				return nil, errors.New("trusted revocation plan names a departing member")
			}
			return groups.DeleteGroup(ctx, plan.TargetID), nil
		},
	},
	ExecutorModeGroupMembershipRemoval: {
		reason: revocation.ReasonGroupMembershipRemoved,
		failed: ErrGroupMembershipRemovalFailed,
		act: func(ctx context.Context, groups groupAdminProvider, plan revocationPlan) (
			*tidcommon.ServiceError, error) {
			if plan.AssigneeID == "" || plan.AssigneeType == "" {
				return nil, errors.New("trusted revocation plan has no departing member")
			}
			_, svcErr := groups.RemoveGroupMembers(ctx, plan.TargetID, []group.Member{
				{ID: plan.AssigneeID, Type: group.MemberType(plan.AssigneeType)},
			})
			return svcErr, nil
		},
	},
}

// groupExecutor performs the group change named in the trusted revocation plan.
type groupExecutor struct {
	providers.Executor
	groups groupAdminProvider
}

var _ providers.Executor = (*groupExecutor)(nil)

// newGroupExecutor creates the acting executor for the group administration flows.
func newGroupExecutor(factory core.FlowFactoryInterface, groups groupAdminProvider) *groupExecutor {
	base := factory.CreateExecutor(ExecutorNameGroup, providers.ExecutorTypeUtility, nil, nil,
		&providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
			SupportedModes:     slices.Sorted(maps.Keys(groupActionsByMode)),
		})
	return &groupExecutor{Executor: base, groups: groups}
}

// Execute performs the change identified by the trusted revocation plan.
func (e *groupExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	action, ok := groupActionsByMode[ctx.ExecutorMode]
	if !ok {
		return nil, fmt.Errorf("unsupported mode for %s: %q", ExecutorNameGroup, ctx.ExecutorMode)
	}
	if e.groups == nil {
		return nil, errors.New("group service is not configured")
	}
	plan, err := revocationPlanFor(ctx.SharedRuntimeData, action.reason)
	if err != nil {
		return nil, err
	}

	svcErr, err := action.act(ctx.Context, e.groups, plan)
	if err != nil {
		return nil, err
	}
	if svcErr != nil {
		return refusalResponse(&providers.ExecutorResponse{}, svcErr, action.failed,
			fmt.Sprintf("failed to perform %s", action.reason))
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

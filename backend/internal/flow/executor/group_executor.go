// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/group"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

type groupAction struct {
	failed tidcommon.ServiceError
	act    func(ctx context.Context, groups groupAdminProvider, plan revocationPlan) (
		*tidcommon.ServiceError, error)
}

var groupActionsByMode = map[string]groupAction{
	ExecutorModeDelete: {
		failed: ErrGroupDeletionFailed,
		act: func(ctx context.Context, groups groupAdminProvider, plan revocationPlan) (
			*tidcommon.ServiceError, error) {
			if plan.AssigneeID != "" {
				return nil, errors.New("trusted revocation plan names a departing member")
			}
			return groups.DeleteGroup(ctx, plan.TargetID), nil
		},
	},
	ExecutorModeRemoveMember: {
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
	modes := make([]string, 0, len(groupActionsByMode))
	for mode := range groupActionsByMode {
		modes = append(modes, mode)
	}
	sort.Strings(modes)
	base := factory.CreateExecutor(ExecutorNameGroup, providers.ExecutorTypeUtility, nil, nil,
		&providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
			SupportedModes:     modes,
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
	plan, err := revocationPlanFor(ctx.SharedRuntimeData, revocation.ReasonGroupMembershipRemoved)
	if err != nil {
		return nil, err
	}

	svcErr, err := action.act(ctx.Context, e.groups, plan)
	if err != nil {
		return nil, err
	}
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to change group membership: %s", svcErr.Error.DefaultValue)
		}
		refusal := *svcErr
		if refusal.Code == "" {
			refusal = action.failed
		}
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &refusal}, nil
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

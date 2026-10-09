// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"errors"
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// actionDeletionExecutor deletes the action whose scope the trusted revocation plan retired.
type actionDeletionExecutor struct {
	providers.Executor
	provider resourceAdminProvider
}

var _ providers.Executor = (*actionDeletionExecutor)(nil)

// newActionDeletionExecutor creates an executor that deletes the action behind a retired scope.
func newActionDeletionExecutor(factory core.FlowFactoryInterface,
	provider resourceAdminProvider) *actionDeletionExecutor {
	base := factory.CreateExecutor(ExecutorNameActionDeletion, providers.ExecutorTypeUtility, nil, nil,
		&providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &actionDeletionExecutor{Executor: base, provider: provider}
}

// Execute deletes the action identified by the trusted revocation plan.
func (e *actionDeletionExecutor) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	if e.provider == nil {
		return nil, errors.New("resource service is not configured")
	}
	plan, err := revocationPlanFor(ctx.SharedRuntimeData, revocation.ReasonActionDeleted)
	if err != nil {
		return nil, err
	}
	actionID := plan.ActionArgs[revocationInputAction]
	if actionID == "" {
		return nil, errors.New("trusted revocation plan has no target action")
	}

	if svcErr := e.provider.DeleteAction(ctx.Context, plan.TargetID,
		optionalID(plan.ActionArgs[revocationInputResource]), actionID); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to delete action: %s", svcErr.Error.DefaultValue)
		}
		refusal := *svcErr
		if refusal.Code == "" {
			refusal = ErrActionDeletionFailed
		}
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &refusal}, nil
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

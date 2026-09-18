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

// scopeDeletionExecutor deletes the action whose scope the trusted revocation plan retired. It runs
// after the scope has been denied deployment-wide.
type scopeDeletionExecutor struct {
	providers.Executor
	provider resourceAdminProvider
}

var _ providers.Executor = (*scopeDeletionExecutor)(nil)

// newScopeDeletionExecutor creates an executor that deletes the action behind a retired scope.
func newScopeDeletionExecutor(factory core.FlowFactoryInterface,
	provider resourceAdminProvider) *scopeDeletionExecutor {
	base := factory.CreateExecutor(ExecutorNameScopeDeletion, providers.ExecutorTypeUtility, nil, nil,
		&providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration},
		})
	return &scopeDeletionExecutor{Executor: base, provider: provider}
}

// Execute deletes the action identified by the trusted revocation plan.
//
// The action id travels in the plan's arguments rather than as this node's own input: the preparatory
// node validated the whole path before anything was revoked, and re-collecting it here would let a
// caller point the deletion at a different action than the one whose scope was denied.
func (e *scopeDeletionExecutor) Execute(ctx *providers.NodeContext) (
	*providers.ExecutorResponse, error) {
	if e.provider == nil {
		return nil, errors.New("resource service is not configured")
	}
	plan, err := revocationPlanFor(ctx.SharedRuntimeData, revocation.ReasonScopeDeleted)
	if err != nil {
		return nil, err
	}
	actionID := plan.ActionArgs[revocationInputAction]
	if actionID == "" {
		return nil, errors.New("trusted revocation plan has no target action")
	}

	if svcErr := e.provider.DeleteAction(ctx.Context, plan.TargetID,
		plan.ActionArgs[revocationInputResource], actionID); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to delete scope: %s", svcErr.Error.DefaultValue)
		}
		// Carry the service's own refusal, as the preparatory node does: a declarative catalog and an
		// action that is simply gone need different things from the operator.
		refusal := *svcErr
		if refusal.Code == "" {
			refusal = ErrScopeDeletionFailed
		}
		return &providers.ExecutorResponse{Status: providers.ExecFailure, Error: &refusal}, nil
	}
	return &providers.ExecutorResponse{Status: providers.ExecComplete}, nil
}

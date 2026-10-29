// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// userDeletionProvider exposes the user operations required by deletion executors.
type userDeletionProvider interface {
	ValidateDeleteUser(ctx context.Context, userID string) *tidcommon.ServiceError
	DeleteUser(ctx context.Context, userID string) *tidcommon.ServiceError
}

// preDeleteExecutor is the preparatory node of an administrative flow. It runs ahead of
// the executors that act, validates the target of the operation, and publishes the trusted
// revocation plan those executors consume.
//
// Nothing downstream re-derives the plan or reads it from the request: the plan travels on
// SharedRuntimeData, which is unexported on the engine context, never serialized to the client, and
// only writable by an executor. That is what makes the breadth recorded here binding on every node
// that follows, rather than something the caller can influence per request.
type preDeleteExecutor struct {
	providers.Executor
	userProvider userDeletionProvider
}

var _ providers.Executor = (*preDeleteExecutor)(nil)

// newPreDeleteExecutor creates the preparatory executor for administrative flows.
func newPreDeleteExecutor(factory core.FlowFactoryInterface,
	userProvider userDeletionProvider) *preDeleteExecutor {
	base := factory.CreateExecutor(ExecutorNamePreDelete, providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: revocationInputSubject, Type: providers.InputTypeText, Required: true},
		}, nil, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAdministration, providers.FlowTypeSystem},
			DefaultMode:        string(revocation.ModeAll),
			SupportedModes:     []string{string(revocation.ModeAll)},
		})
	return &preDeleteExecutor{Executor: base, userProvider: userProvider}
}

// resolveRevocationMode returns the revocation breadth configured on the node, falling back to the
// executor's default when the node omits it. An unsupported value cannot normally reach here because
// flow creation validates the mode, so this is the runtime backstop for a flow persisted before the
// mode was constrained.
func resolveRevocationMode(ctx *providers.NodeContext) (revocation.Mode, bool) {
	mode := revocation.Mode(ctx.ExecutorMode)
	if mode == "" {
		mode = revocation.ModeAll
	}
	return mode, mode == revocation.ModeAll
}

// Execute validates the target and publishes the trusted plan consumed by later executors. The plan
// carries the mode configured on this node, so CriteriaRevocationExecutor writes with the breadth the
// flow author chose rather than one supplied at execution time.
func (e *preDeleteExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	execResp := &providers.ExecutorResponse{Status: providers.ExecComplete}
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}
	subject, ok := ctx.ConsumeInput(revocationInputSubject)
	if !ok || subject == "" {
		return nil, fmt.Errorf("subject is required")
	}
	mode, supported := resolveRevocationMode(ctx)
	if !supported {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrInvalidRevocationMode
		return execResp, nil
	}
	if e.userProvider == nil {
		return nil, fmt.Errorf("user service is not configured")
	}
	if svcErr := e.userProvider.ValidateDeleteUser(ctx.Context, subject); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("failed to validate user deletion: %s", svcErr.Error.DefaultValue)
		}
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrUserDeletionNotAllowed
		return execResp, nil
	}
	plan, err := encodeRevocationPlan(revocationPlan{
		Criteria: []revocation.Criterion{{Type: revocation.CriterionTypeSubject, Value: subject}},
		Mode:     mode,
		Reason:   revocation.ReasonUserDeleted,
	})
	if err != nil {
		return nil, err
	}
	execResp.SharedRuntimeData = map[string]string{common.RuntimeKeyRevocationPlan: plan}
	return execResp, nil
}

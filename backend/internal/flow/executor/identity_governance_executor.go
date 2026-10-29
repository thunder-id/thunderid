// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"context"
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// identityGovernanceExecutor runs the lifecycle operation the node's mode names. It writes state only;
// session and token revocation are separate nodes in the flow.
type identityGovernanceExecutor struct {
	providers.Executor
	governance providers.IdentityGovernanceProvider
	logger     *log.Logger
}

var _ providers.Executor = (*identityGovernanceExecutor)(nil)

// newIdentityGovernanceExecutor creates the lifecycle executor for administration and recovery flows.
func newIdentityGovernanceExecutor(factory core.FlowFactoryInterface,
	governance providers.IdentityGovernanceProvider) *identityGovernanceExecutor {
	base := factory.CreateExecutor(ExecutorNameIdentityGovernance, providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: revocationInputSubject, Type: providers.InputTypeText, Required: true},
			{Identifier: governanceInputOperatorNote, Type: providers.InputTypeText, Required: false},
		}, nil, &providers.ExecutorMeta{
			// Execute refuses every mode but unlock in a recovery flow.
			SupportedFlowTypes: []providers.FlowType{
				providers.FlowTypeAdministration, providers.FlowTypeRecovery,
			},
			// No default mode, so a flow that omits the mode cannot be saved.
			SupportedModes: []string{
				ExecutorModeUnlock, ExecutorModeSuspend, ExecutorModeUnsuspend,
			},
		})
	return &identityGovernanceExecutor{
		Executor:   base,
		governance: governance,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, "IdentityGovernanceExecutor"),
			log.String(log.LoggerKeyExecutorName, ExecutorNameIdentityGovernance)),
	}
}

// Execute runs the lifecycle operation the node's mode names.
func (e *identityGovernanceExecutor) Execute(
	ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	switch ctx.ExecutorMode {
	case ExecutorModeUnlock:
		return e.executeUnlock(ctx)
	case ExecutorModeSuspend:
		return e.executeSuspend(ctx)
	case ExecutorModeUnsuspend:
		return e.executeUnsuspend(ctx)
	default:
		return nil, fmt.Errorf("unsupported identity governance mode: %s", ctx.ExecutorMode)
	}
}

// executeUnlock releases every automatic lock on the subject: the subject input in an administration
// flow, and the proven subject in a recovery flow.
func (e *identityGovernanceExecutor) executeUnlock(
	ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	selfService := ctx.FlowType != providers.FlowTypeAdministration
	if !selfService {
		return e.executeAdministration(ctx, logger,
			func(callCtx context.Context, subject, _ string) *tidcommon.ServiceError {
				return e.governance.Unlock(callCtx, subject)
			})
	}

	execResp := newIdentityGovernanceResponse()
	// A recovery flow never trusts a subject input; it is consumed so it is not re-prompted.
	ctx.ConsumeInput(revocationInputSubject)
	subject := ctx.SharedRuntimeData[common.RuntimeKeyProvenSubject]
	if subject == "" {
		logger.Debug(ctx.Context, "Release node reached with no proof of control recorded",
			log.String("mode", ctx.ExecutorMode))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrIdentityGovernanceFailed
		return execResp, nil
	}
	ctx.ConsumeInput(governanceInputOperatorNote)

	// A recovery completes without governance, as there is no lock to release.
	if e.governance == nil {
		logger.Debug(ctx.Context, "No identity governance provider; the recovery released nothing")
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	svcErr := e.governance.Unlock(security.WithRuntimeContext(ctx.Context), subject)
	return e.complete(ctx, logger, execResp, subject, svcErr, true)
}

// executeSuspend places the administrative hold on the target of the plan a PreSuspendExecutor
// published.
func (e *identityGovernanceExecutor) executeSuspend(
	ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	return e.executeAdministration(ctx, logger,
		func(callCtx context.Context, subject, operatorNote string) *tidcommon.ServiceError {
			return e.governance.Suspend(callCtx, subject, operatorNote)
		})
}

// executeUnsuspend lifts the administrative hold on the subject input.
func (e *identityGovernanceExecutor) executeUnsuspend(
	ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))
	return e.executeAdministration(ctx, logger,
		func(callCtx context.Context, subject, _ string) *tidcommon.ServiceError {
			return e.governance.Unsuspend(callCtx, subject)
		})
}

// executeAdministration runs an operation in an administration flow, in the operator's context. A
// PermissionValidator node gates the flow; the provider checks the operator's boundary.
func (e *identityGovernanceExecutor) executeAdministration(ctx *providers.NodeContext, logger *log.Logger,
	operation func(callCtx context.Context, subject, operatorNote string) *tidcommon.ServiceError,
) (*providers.ExecutorResponse, error) {
	if ctx.FlowType != providers.FlowTypeAdministration {
		return nil, fmt.Errorf("identity governance mode %s is administration-only", ctx.ExecutorMode)
	}
	if e.governance == nil {
		return nil, fmt.Errorf("identity governance provider is not configured")
	}
	execResp := newIdentityGovernanceResponse()
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}

	subject, err := e.resolveAdministrationSubject(ctx)
	if err != nil {
		return nil, err
	}
	// Consumed on every mode, since the engine re-prompts an unconsumed input.
	operatorNote, _ := ctx.ConsumeInput(governanceInputOperatorNote)

	return e.complete(ctx, logger, execResp, subject, operation(ctx.Context, subject, operatorNote), false)
}

// complete maps the provider's answer to the node's outcome. A refused release does not undo a
// recovery, which completes and releases nothing.
func (e *identityGovernanceExecutor) complete(ctx *providers.NodeContext, logger *log.Logger,
	execResp *providers.ExecutorResponse, subject string, svcErr *tidcommon.ServiceError,
	selfService bool) (*providers.ExecutorResponse, error) {
	mode := ctx.ExecutorMode
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("identity governance %s failed: %s", mode, svcErr.Error.DefaultValue)
		}
		logger.Debug(ctx.Context, "Identity governance operation refused",
			log.String("mode", mode), log.String("errorCode", svcErr.Code),
			log.MaskedString(log.LoggerKeyEntityID, subject))
		if selfService {
			execResp.Status = providers.ExecComplete
			return execResp, nil
		}
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrIdentityGovernanceFailed
		return execResp, nil
	}

	// Published for later nodes, since this executor consumed the input naming the subject.
	execResp.RuntimeData[userAttributeUserID] = subject

	logger.Debug(ctx.Context, "Identity governance operation applied",
		log.String("mode", mode), log.MaskedString(log.LoggerKeyEntityID, subject))
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// HasRequiredInputs prompts for the required inputs where the node reads its subject from them: an
// administration unlock or unsuspend. A suspend takes the subject from the plan and a recovery flow
// from the recorded proof.
func (e *identityGovernanceExecutor) HasRequiredInputs(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) bool {
	if ctx.FlowType != providers.FlowTypeAdministration || ctx.ExecutorMode == ExecutorModeSuspend {
		return true
	}
	complete := true
	for _, input := range e.GetRequiredInputs(ctx) {
		if !input.Required {
			continue
		}
		if _, ok := ctx.UserInputs[input.Identifier]; !ok {
			execResp.Inputs = append(execResp.Inputs, input)
			complete = false
		}
	}
	return complete
}

// resolveAdministrationSubject returns who an administration node acts on: the plan's target for a
// suspend, otherwise the subject input.
func (e *identityGovernanceExecutor) resolveAdministrationSubject(ctx *providers.NodeContext) (string, error) {
	if ctx.ExecutorMode == ExecutorModeSuspend {
		ctx.ConsumeInput(revocationInputSubject)
		plan, err := revocationPlanFor(ctx.SharedRuntimeData, revocation.ReasonIdentitySuspended)
		if err != nil {
			return "", fmt.Errorf("suspend requires the plan a PreSuspendExecutor publishes: %w", err)
		}
		return plan.TargetID, nil
	}
	subject, ok := ctx.ConsumeInput(revocationInputSubject)
	if !ok || subject == "" {
		return "", fmt.Errorf("subject is required")
	}
	return subject, nil
}

// newIdentityGovernanceResponse builds an empty executor response.
func newIdentityGovernanceResponse() *providers.ExecutorResponse {
	return &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}
}

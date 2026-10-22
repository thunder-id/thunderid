// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
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
		}, nil, &providers.ExecutorMeta{
			// Execute refuses every mode but unlock in a recovery flow.
			SupportedFlowTypes: []providers.FlowType{
				providers.FlowTypeAdministration, providers.FlowTypeRecovery,
			},
			// No default mode, so a flow that omits the mode cannot be saved.
			SupportedModes: []string{
				ExecutorModeUnlock,
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

// Execute applies the node's mode to the resolved subject.
func (e *identityGovernanceExecutor) Execute(
	ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	mode := ctx.ExecutorMode
	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	selfService := ctx.FlowType != providers.FlowTypeAdministration
	if selfService && mode != ExecutorModeUnlock {
		return nil, fmt.Errorf("identity governance mode %s is not available in a recovery flow", mode)
	}
	if e.governance == nil && !selfService {
		return nil, fmt.Errorf("identity governance provider is not configured")
	}
	if !e.HasRequiredInputs(ctx, execResp) {
		execResp.Status = providers.ExecUserInputRequired
		return execResp, nil
	}

	subject, err := e.resolveSubject(ctx, selfService)
	if err != nil {
		return nil, err
	}
	if subject == "" {
		logger.Debug(ctx.Context, "Release node reached with no proof of control recorded",
			log.String("mode", mode))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrIdentityGovernanceFailed
		return execResp, nil
	}

	// A recovery completes without governance, as there is no lock to release.
	if e.governance == nil {
		logger.Debug(ctx.Context, "No identity governance provider; the recovery released nothing")
		execResp.Status = providers.ExecComplete
		return execResp, nil
	}

	// A PermissionValidator node gates the flow; the provider checks the operator's boundary.
	svcErr, err := e.apply(ctx, mode, subject, selfService)
	if err != nil {
		return nil, err
	}
	if svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("identity governance %s failed: %s", mode, svcErr.Error.DefaultValue)
		}
		logger.Debug(ctx.Context, "Identity governance operation refused",
			log.String("mode", mode), log.String("errorCode", svcErr.Code),
			log.MaskedString(log.LoggerKeyEntityID, subject))
		// A refused release does not undo the recovery, which completes and releases nothing.
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

// HasRequiredInputs prompts for the required inputs in an administration flow, where the node reads
// its subject from them. A recovery flow takes the subject from the recorded proof.
func (e *identityGovernanceExecutor) HasRequiredInputs(ctx *providers.NodeContext,
	execResp *providers.ExecutorResponse) bool {
	if ctx.FlowType != providers.FlowTypeAdministration {
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

// apply calls the provider method the mode names. An administration flow keeps the operator's
// context; a recovery release runs with runtime privilege.
func (e *identityGovernanceExecutor) apply(ctx *providers.NodeContext,
	mode, subject string, selfService bool) (*tidcommon.ServiceError, error) {
	callCtx := ctx.Context
	if selfService {
		callCtx = security.WithRuntimeContext(callCtx)
	}

	switch mode {
	case ExecutorModeUnlock:
		return e.governance.Unlock(callCtx, subject), nil
	default:
		return nil, fmt.Errorf("unsupported identity governance mode: %s", mode)
	}
}

// resolveSubject returns who the node acts on: the subject input in an administration flow, and the
// proven subject in a recovery flow, empty when nothing was proven.
func (e *identityGovernanceExecutor) resolveSubject(ctx *providers.NodeContext,
	selfService bool) (string, error) {
	if !selfService {
		subject, ok := ctx.ConsumeInput(revocationInputSubject)
		if !ok || subject == "" {
			return "", fmt.Errorf("subject is required")
		}
		return subject, nil
	}

	// A recovery flow never trusts a subject input; it is consumed so it is not re-prompted.
	ctx.ConsumeInput(revocationInputSubject)
	return ctx.SharedRuntimeData[common.RuntimeKeyProvenSubject], nil
}

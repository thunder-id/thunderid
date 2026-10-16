// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/revocation"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// The lifecycle operations, as one executor selected by node mode. They share target resolution,
// trust boundary and response shape, and differ only in the method they call.
const (
	// governanceModeUnlock releases every lock, the post-suspension hold included. It restores
	// nothing, since a lock only withholds fresh authentication.
	governanceModeUnlock = "unlock"
	// governanceModeSuspend places an administrative security hold on the subject named by the
	// trusted plan a PreSuspendExecutor published, after the containment nodes have run on it.
	governanceModeSuspend = "suspend"
	// governanceModeUnsuspend releases the hold without running the revocation nodes. For a user it
	// leaves an entity-wide lock that a recovery or an unlock clears.
	governanceModeUnsuspend = "unsuspend"
)

// governanceInputOperatorNote is the optional free-text note a suspend node may collect. It is
// stored for management reads, never reaches a refusal, and is ignored by every other mode.
const governanceInputOperatorNote = "operatorNote"

// identityGovernanceExecutor runs one lifecycle operation on the subject an administration flow
// resolved. It writes state only; session and token revocation are separate nodes in the flow, as
// in the user-deletion flow.
type identityGovernanceExecutor struct {
	providers.Executor
	governance providers.IdentityGovernanceProvider
	logger     *log.Logger
}

var _ providers.Executor = (*identityGovernanceExecutor)(nil)

// newIdentityGovernanceExecutor creates the lifecycle executor for administration flows. It depends
// on `providers.IdentityGovernanceProvider` only.
func newIdentityGovernanceExecutor(factory core.FlowFactoryInterface,
	governance providers.IdentityGovernanceProvider) *identityGovernanceExecutor {
	base := factory.CreateExecutor(ExecutorNameIdentityGovernance, providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: revocationInputSubject, Type: providers.InputTypeText, Required: true},
			{Identifier: governanceInputOperatorNote, Type: providers.InputTypeText, Required: false},
		}, nil, &providers.ExecutorMeta{
			// Recovery is allowed for unlock only. Flow types are declared per executor, so Execute
			// refuses suspend and unsuspend in a recovery flow; never drop that check.
			SupportedFlowTypes: []providers.FlowType{
				providers.FlowTypeAdministration, providers.FlowTypeRecovery,
			},
			// No default mode, deliberately: there is no safe default, so a flow that omits the mode
			// cannot be saved.
			SupportedModes: []string{
				governanceModeUnlock, governanceModeSuspend, governanceModeUnsuspend,
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

	if e.governance == nil {
		return nil, fmt.Errorf("identity governance provider is not configured")
	}

	mode, ok := e.resolveMode(ctx)
	if !ok {
		return nil, fmt.Errorf("unsupported identity governance mode: %s", ctx.ExecutorMode)
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	selfService := ctx.FlowType != providers.FlowTypeAdministration
	if selfService && mode != governanceModeUnlock {
		return nil, fmt.Errorf("identity governance mode %s is administration-only", mode)
	}

	subject, err := e.resolveSubject(ctx, mode, selfService)
	if err != nil {
		return nil, err
	}
	if subject == "" {
		// Only reachable on the self-service path; the administration path errors above, as a flow
		// with no target is malformed.
		logger.Debug(ctx.Context, "Release node reached with no proof of control recorded",
			log.String("mode", mode))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrIdentityGovernanceFailed
		return execResp, nil
	}

	// Consumed on every mode, since the engine re-prompts an unconsumed input and a stray note
	// would loop.
	operatorNote, _ := ctx.ConsumeInput(governanceInputOperatorNote)

	// No permission check here: a PermissionValidator node gates the flow, and the authority checks
	// the target against the caller's organization-unit boundary.
	if svcErr := e.apply(ctx, mode, subject, operatorNote, selfService); svcErr != nil {
		if svcErr.Type == tidcommon.ServerErrorType {
			return nil, fmt.Errorf("identity governance %s failed: %s", mode, svcErr.Error.DefaultValue)
		}
		logger.Debug(ctx.Context, "Identity governance operation refused",
			log.String("mode", mode), log.MaskedString("subject", subject))
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrIdentityGovernanceFailed
		return execResp, nil
	}

	// Publish the subject under the key other executors resolve an identity by, since this executor
	// consumed the input naming it.
	execResp.RuntimeData[userAttributeUserID] = subject

	logger.Debug(ctx.Context, "Identity governance operation applied",
		log.String("mode", mode), log.MaskedString("subject", subject))
	execResp.Status = providers.ExecComplete
	return execResp, nil
}

// apply dispatches the mode to the port. An administration flow forwards the operator's context
// for the authority's checks; a self-service release runs with runtime privilege, bounded by the
// proof resolveSubject requires.
func (e *identityGovernanceExecutor) apply(ctx *providers.NodeContext,
	mode, subject, operatorNote string, selfService bool) *tidcommon.ServiceError {
	callCtx := ctx.Context
	if selfService {
		callCtx = security.WithRuntimeContext(callCtx)
	}

	switch mode {
	case governanceModeUnlock:
		return e.governance.Unlock(callCtx, subject)
	case governanceModeSuspend:
		return e.governance.Suspend(callCtx, subject, operatorNote)
	case governanceModeUnsuspend:
		return e.governance.Unsuspend(callCtx, subject)
	}
	return nil
}

// resolveSubject answers who this node acts on: a suspend uses the trusted plan, an administration
// flow its gated input, and a self-service flow only the proven subject, never an input. With
// nothing proven it returns an empty subject so the caller refuses opaquely.
func (e *identityGovernanceExecutor) resolveSubject(ctx *providers.NodeContext, mode string,
	selfService bool) (string, error) {
	if mode == governanceModeSuspend {
		// Consumed so a stray input is not re-prompted; the plan is the target.
		ctx.ConsumeInput(revocationInputSubject)
		plan, err := revocationPlanFor(ctx.SharedRuntimeData, revocation.ReasonIdentitySuspended)
		if err != nil {
			return "", fmt.Errorf("suspend requires the plan a PreSuspendExecutor publishes: %w", err)
		}
		return plan.TargetID, nil
	}
	if !selfService {
		subject, ok := ctx.ConsumeInput(revocationInputSubject)
		if !ok || subject == "" {
			return "", fmt.Errorf("subject is required")
		}
		return subject, nil
	}

	// Consumed and discarded, so a stray input is not re-prompted into a loop.
	ctx.ConsumeInput(revocationInputSubject)

	return ctx.SharedRuntimeData[common.RuntimeKeyProvenSubject], nil
}

// resolveMode reads the node's mode, as a runtime backstop to flow validation. It refuses an
// unknown mode rather than defaulting.
func (e *identityGovernanceExecutor) resolveMode(ctx *providers.NodeContext) (string, bool) {
	mode := ctx.ExecutorMode
	switch mode {
	case governanceModeUnlock, governanceModeSuspend, governanceModeUnsuspend:
		return mode, true
	default:
		return "", false
	}
}

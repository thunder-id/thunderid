// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// sessionSignOutExecutor is the task behind a session sign-out node. It ends the SSO session that the
// login flow established and signals the transport layer to clear that flow's per-flow cookie. The
// login flow whose session is targeted is resolved by the engine (SessionFlowID) and delivered
// through the SSO inputs, so this executor needs only the inbound handle and that flow id. It holds
// only the SSO session service, never the stores directly.
type sessionSignOutExecutor struct {
	providers.Executor
	sso    session.Service
	logger *log.Logger
}

var _ providers.Executor = (*sessionSignOutExecutor)(nil)

// newSessionSignOutExecutor creates a new session sign-out executor backed by the SSO session service.
func newSessionSignOutExecutor(flowFactory core.FlowFactoryInterface, sso session.Service) *sessionSignOutExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SessionSignOutExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameSessionSignOut))

	base := flowFactory.CreateExecutor(ExecutorNameSessionSignOut, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeSignOut},
		})

	return &sessionSignOutExecutor{
		Executor: base,
		sso:      sso,
		logger:   logger,
	}
}

// Execute ends the SSO session referenced by the inbound handle for the login flow and raises the
// cookie-clear signal. Terminate is idempotent, so a missing or already-ended session is not an
// error; the cookie is cleared regardless so the browser drops any stale handle. It routes to the
// success outcome — sign-out completes even when there was nothing to end.
//
// When the node opts in with the promptOnSignOut property and the RP-initiated logout arrived without
// a valid id_token_hint (RuntimeKeyLogoutPromptRequired), the executor first routes to the node's
// onIncomplete confirmation prompt and only terminates the session once the End-User confirms. This
// keeps the confirmation logic in the executor rather than a node condition the flow editor cannot
// represent. See decide for how the prompt's action types map to outcomes.
func (e *sessionSignOutExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		Status:      providers.ExecComplete,
		RuntimeData: make(map[string]string),
		EngineData:  make(map[string]string),
	}

	// Ask the End-User to confirm before terminating when the node requests it and no valid
	// id_token_hint established the request's legitimacy. Routing to the onIncomplete prompt and back
	// is enough: the prompt forwards the chosen action's type here on the re-run, so the decision is
	// read off that type without persisting any marker.
	if e.decide(ctx) == signOutPrompt {
		execResp.Status = providers.ExecUserInputRequired
		logger.Debug(ctx.Context, "Routing to sign-out confirmation prompt")
		return execResp, nil
	}

	in := session.SSOInputsFrom(ctx.Context)
	if _, err := e.sso.Terminate(ctx.Context, in.Handle, in.FlowID); err != nil {
		return execResp, err
	}

	// Signal the transport layer to clear the per-flow cookie. The engine resolves the flow id
	// (the login flow) from the execution's SessionFlowID. The post-logout redirect is not the flow's
	// concern — the OAuth layer resolves it on the sign-out completion callback.
	execResp.EngineData[common.RuntimeKeySSOSessionCleared] = dataValueTrue

	logger.Debug(ctx.Context, "Terminated SSO session on sign-out", log.String("flowID", in.FlowID))
	return execResp, nil
}

// signOutOutcome is what the executor does with the current request.
type signOutOutcome int

const (
	// signOutTerminate ends the SSO session.
	signOutTerminate signOutOutcome = iota
	// signOutPrompt routes to the node's onIncomplete confirmation prompt.
	signOutPrompt
)

// decide reports what the executor should do with the current request.
//
// Confirmation is skipped entirely unless the node opts in (promptOnSignOut) and the RP-initiated
// logout requires a prompt because it carried no valid id_token_hint. Past that, the outcome comes
// from the action type the confirmation prompt forwards on the re-run, so the End-User's choice is
// read off the flow definition rather than a persisted marker.
//
// Each action the confirmation prompt can raise maps to one outcome here: a new button on that
// prompt is supported by giving its action type a case, and a new outcome by adding a
// signOutOutcome and handling it in Execute.
func (e *sessionSignOutExecutor) decide(ctx *providers.NodeContext) signOutOutcome {
	promptEnabled, _ := ctx.NodeProperties[propertyKeyPromptOnSignOut].(bool)
	if !promptEnabled {
		return signOutTerminate
	}
	if ctx.RuntimeData[common.RuntimeKeyLogoutPromptRequired] != dataValueTrue {
		return signOutTerminate
	}

	actionType, _ := ctx.ForwardedData[common.ForwardedDataKeyActionType].(string)
	switch common.ActionType(actionType) {
	case common.ActionTypeConfirm:
		return signOutTerminate
	default:
		// The initial request forwards no action type at all, and a type this executor does not
		// recognize is not consent to end the session. Both ask the End-User to confirm.
		return signOutPrompt
	}
}

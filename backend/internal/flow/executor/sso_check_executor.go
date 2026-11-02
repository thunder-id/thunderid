// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"strconv"
	"time"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// ssoCheckExecutor resolves whether a live, compatible SSO session exists for the current flow and
// records the decision. It is the task behind the SSO-Check node and routes the Skip/Authenticate
// outcomes. It holds only the SSO session service, never the stores directly.
type ssoCheckExecutor struct {
	providers.Executor
	sso    session.Service
	logger *log.Logger
}

var _ providers.Executor = (*ssoCheckExecutor)(nil)

// newSSOCheckExecutor creates a new SSO-Check executor backed by the SSO session service.
func newSSOCheckExecutor(flowFactory core.FlowFactoryInterface, sso session.Service) *ssoCheckExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SSOCheckExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameSSOCheck))

	base := flowFactory.CreateExecutor(ExecutorNameSSOCheck, providers.ExecutorTypeUtility,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAuthentication},
			SupportedProperties: []providers.ExecutorSupportedProperties{
				{Property: common.NodePropertyCheckpointRef, IsRequired: true},
			},
		})

	return &ssoCheckExecutor{
		Executor: base,
		sso:      sso,
		logger:   logger,
	}
}

// Execute routes this SSO-Check node's two outcomes for its checkpoint (the Session node id named by
// NodePropertyCheckpointRef):
//   - Skip (a live session that already holds this checkpoint's snapshot): COMPLETE → onSuccess;
//     records the checkpoint-present flag and the shared session handle so the paired Session node
//     loads the saved flow state.
//   - Authenticate (no live session, or the session lacks this checkpoint): FAILURE → onFailure,
//     sending the flow down the full-authentication path for this stage. When a live session exists
//     but lacks the checkpoint, the handle is still shared so the fresh join attaches its new
//     checkpoint to that same session. This is a routing outcome, not a hard error.
func (e *ssoCheckExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	checkpoint := checkpointRef(ctx)
	presentKey := common.SSOCheckpointKey(common.RuntimeKeySSOSessionPresent, checkpoint)

	in := session.SSOInputsFrom(ctx.Context)
	resolved, err := e.sso.Resolve(ctx.Context, in.Handle, in.FlowID, in.FlowVersion, time.Now().UTC())
	if err != nil {
		return execResp, err
	}
	if resolved != nil {
		// A live session exists; share its handle so a fresh join attaches to it even when this
		// checkpoint is not yet present.
		execResp.RuntimeData[common.RuntimeKeySSOSessionHandle] = resolved.HandleID
	}

	// A request may demand a fresh authentication even when a live session exists: prompt=login
	// asks for one outright, and a max_age smaller than the session's age makes the existing
	// authentication too old to satisfy. Both are answered by re-authenticating here, at the
	// branch point, rather than by failing at the end of the flow once the assertion is due.
	if resolved != nil && reauthRequired(ctx, resolved.AuthenticatedAt, logger) {
		resolved = nil
	}

	var snapshot *session.SessionContext
	if resolved != nil && checkpoint != "" {
		if snapshot, err = e.sso.FindCheckpoint(ctx.Context, resolved.SessionID, checkpoint); err != nil {
			return execResp, err
		}
	}

	if snapshot != nil {
		execResp.Status = providers.ExecComplete
		execResp.RuntimeData[presentKey] = dataValueTrue
		// Hand the two rows this node just read to the paired Session node, which needs the same ones
		// to restore the checkpoint. ForwardedData reaches the immediate next node only, so this is
		// set on the Skip outcome alone: on the Authenticate outcome the flow prompts and suspends,
		// and forwarded data that outlived the node would be persisted with the flow context.
		execResp.ForwardedData = map[string]interface{}{
			common.ForwardedDataKeySSOSession:        resolved,
			common.ForwardedDataKeySSOSessionContext: snapshot,
		}
		logger.Debug(ctx.Context, "Live SSO checkpoint present; routing to the Skip outcome",
			log.String("flowId", in.FlowID),
			log.String("checkpoint", checkpoint))
	} else {
		execResp.Status = providers.ExecFailure
		execResp.Error = &ErrNoLiveSSOSession
		execResp.RuntimeData[presentKey] = "false"
		logger.Debug(ctx.Context, "No reusable SSO checkpoint; routing to the Authenticate outcome",
			log.String("checkpoint", checkpoint))
	}

	return execResp, nil
}

// reauthRequired reports whether the request rules out reusing a session that authenticated at
// authenticatedAt: prompt=login always does, and max_age does when more than that many seconds
// have passed since. A malformed max_age is treated as no constraint, matching the assurance
// check that also reads it.
func reauthRequired(ctx *providers.NodeContext, authenticatedAt time.Time, logger *log.Logger) bool {
	if ctx.RuntimeData[common.RuntimeKeyForceReauth] == dataValueTrue {
		logger.Debug(ctx.Context, "prompt=login requires a fresh authentication; not reusing the session")
		return true
	}

	// A silent request cannot answer a credential prompt, and the authorize endpoint already
	// confirmed this session satisfies it. Re-deciding here would compare max_age against a later
	// clock than that check used, so a session on the boundary would be declined and the flow would
	// prompt a request that forbids prompting. prompt=none cannot be combined with prompt=login, so
	// the forced-reauth case above still takes precedence.
	if ctx.RuntimeData[common.RuntimeKeySilentAuthOnly] == dataValueTrue {
		logger.Debug(ctx.Context,
			"Silent request already validated at the authorize endpoint; reusing the session")
		return false
	}

	rawMaxAge := ctx.RuntimeData[common.RuntimeKeyMaxAge]
	if rawMaxAge == "" || authenticatedAt.IsZero() {
		return false
	}
	maxAge, err := strconv.ParseInt(rawMaxAge, 10, 64)
	if err != nil || maxAge < 0 {
		logger.Debug(ctx.Context, "Ignoring malformed max_age", log.String("maxAge", rawMaxAge))
		return false
	}
	// max_age=0 admits no elapsed time at all, so it demands re-authentication even within the same
	// second as the authentication (OIDC Core 3.1.2.1 treats it as equivalent to prompt=login)
	if maxAge == 0 || time.Now().UTC().Unix()-authenticatedAt.Unix() > maxAge {
		logger.Debug(ctx.Context, "Session authentication is older than max_age; re-authenticating",
			log.String("maxAge", rawMaxAge))
		return true
	}
	return false
}

// checkpointRef returns the Session (join) node id this SSO-Check node guards, read from
// NodePropertyCheckpointRef. An empty value means the node is not paired with a checkpoint, which
// routes to the Authenticate outcome.
func checkpointRef(ctx *providers.NodeContext) string {
	if ctx.NodeProperties == nil {
		return ""
	}
	if v, ok := ctx.NodeProperties[common.NodePropertyCheckpointRef].(string); ok {
		return v
	}
	return ""
}

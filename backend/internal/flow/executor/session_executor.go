/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package executor

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// sessionExecutor is the task behind a Session node, which sits at the join where the SSO and
// fresh-authentication branches converge. Its node id is the checkpoint id: on the fresh path it
// saves this checkpoint's session context (establishing the flow execution's session if needed) and
// emits the handle; on the SSO path it loads the checkpoint's saved context into the execution
// context so downstream nodes continue authenticated. A flow may hold several such checkpoints, all
// sharing one session per flow execution.
//
// It is an authentication-type executor: on the SSO path the engine only adopts the loaded
// authenticated user from an authentication executor. All session persistence is delegated to the
// SSO session service; this executor owns only the authn resolution and the flow-context glue.
type sessionExecutor struct {
	providers.Executor
	sso           session.Service
	authnProvider providers.AuthnProviderManager
	logger        *log.Logger
}

var _ providers.Executor = (*sessionExecutor)(nil)

// newSessionExecutor creates a new Session executor. The SSO session service wraps all session
// persistence; the authn provider resolves the subject's entity reference when saving and is the
// contract downstream nodes use to read the subject reconstructed on the SSO load path.
func newSessionExecutor(flowFactory core.FlowFactoryInterface, sso session.Service,
	authnProvider providers.AuthnProviderManager) *sessionExecutor {
	logger := log.GetLogger().With(log.String(log.LoggerKeyComponentName, "SessionExecutor"),
		log.String(log.LoggerKeyExecutorName, ExecutorNameSession))

	base := flowFactory.CreateExecutor(ExecutorNameSession, providers.ExecutorTypeAuthentication,
		[]providers.Input{}, []providers.Input{}, &providers.ExecutorMeta{
			SupportedFlowTypes: []providers.FlowType{providers.FlowTypeAuthentication},
		})

	return &sessionExecutor{
		Executor:      base,
		sso:           sso,
		authnProvider: authnProvider,
		logger:        logger,
	}
}

// Execute saves or loads a checkpoint's session context depending on the SSO-Check decision for this
// join node. The checkpoint id is this node's own id; the paired SSO-Check node names it via
// NodePropertyCheckpointRef. The two paths handle failure asymmetrically:
//   - Save (fresh): the user authenticated through this stage's steps, so a save failure only
//     forfeits future reuse of this checkpoint — it degrades SSO but must not fail authentication.
//   - Load (SSO): SSO-Check committed to skipping this stage on the strength of the resolved
//     checkpoint, so the load is the authentication for this run. A load failure leaves no
//     authenticated subject and no fallback, so it fails the flow.
//
// All checkpoints of one flow execution share a single session (one handle, one cookie). The first
// join to establish it wins a database-level race keyed by the flow execution id; later joins — on
// any branch, in any request of the execution — attach their checkpoint to that same session.
func (e *sessionExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		Status:      providers.ExecComplete,
		RuntimeData: make(map[string]string),
	}

	checkpoint := ctx.CurrentNodeID
	if ctx.RuntimeData[common.SSOCheckpointKey(common.RuntimeKeySSOSessionPresent, checkpoint)] == dataValueTrue {
		// A load failure after SSO-Check already skipped the credential steps leaves no authenticated
		// subject, so the flow cannot proceed. Return it as a server error: the task-execution node
		// logs it and fails the flow.
		if err := e.loadCheckpoint(ctx, execResp, checkpoint, logger); err != nil {
			return execResp, fmt.Errorf("failed to load SSO checkpoint: %w", err)
		}
		return execResp, nil
	}

	if err := e.saveCheckpoint(ctx, execResp, checkpoint, logger); err != nil {
		logger.Error(ctx.Context, "Failed to save SSO checkpoint; continuing without SSO", log.Error(err))
	}
	return execResp, nil
}

// saveCheckpoint resolves the authenticated subject, builds the snapshot from this join's runtime
// state, and hands it to the SSO session service to attach to the flow execution's session. It emits
// the handle for the transport layer to set only when the service minted a new session.
func (e *sessionExecutor) saveCheckpoint(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	checkpoint string, logger *log.Logger) error {
	// Always preserve the already-authenticated user; this executor does not change it on the save
	// path, but it is an authentication-type executor so it must echo the AuthUser back to keep the
	// engine's authenticated subject.
	execResp.AuthUser = ctx.AuthUser

	if e.authnProvider == nil || !ctx.AuthUser.IsAuthenticated() {
		logger.Debug(ctx.Context, "No authenticated subject; skipping checkpoint save")
		return nil
	}

	// Idempotency: if this checkpoint was already saved in this flow execution, re-emit its handle
	// instead of saving again.
	savedKey := common.SSOCheckpointKey(common.RuntimeKeySSOSessionSaved, checkpoint)
	if existing := ctx.RuntimeData[savedKey]; existing != "" {
		setHandleOut(execResp, existing)
		return nil
	}

	// Resolve the subject's entity reference — authn is this executor's responsibility. Its entity id
	// keys the session and drives the cross-checkpoint subject-consistency check. The AuthUser is
	// snapshotted as-is below; this executor does not materialize attributes or otherwise change it.
	_, entityRef, svcErr := e.authnProvider.GetEntityReference(ctx.Context, ctx.AuthUser)
	if svcErr != nil {
		return fmt.Errorf("failed to resolve subject entity reference: %s", svcErr.ErrorDescription.DefaultValue)
	}
	if entityRef == nil || entityRef.EntityID == "" {
		logger.Debug(ctx.Context, "No resolved subject id; skipping checkpoint save")
		return nil
	}

	// Snapshot the AuthUser exactly as this flow left it — resolved values stay resolved, lazy tokens
	// stay lazy — so the load path replays it verbatim. The RuntimeData snapshot is sanitized of the
	// SSO control keys and a small deny-list of request-scoped keys so a replay cannot override the
	// joining app's own per-request state (see sanitizeSnapshotRuntimeData).
	authUserJSON, err := json.Marshal(&ctx.AuthUser)
	if err != nil {
		return fmt.Errorf("failed to marshal AuthUser for snapshot: %w", err)
	}

	ssoIn := session.SSOInputsFrom(ctx.Context)
	result, err := e.sso.SaveCheckpoint(ctx.Context, session.SaveCheckpointInput{
		SubjectID:      entityRef.EntityID,
		FlowID:         ssoIn.FlowID,
		FlowVersion:    ssoIn.FlowVersion,
		ExecutionID:    ctx.ExecutionID,
		HandleHint:     ctx.RuntimeData[common.RuntimeKeySSOSessionHandle],
		Checkpoint:     checkpoint,
		AuthUser:       authUserJSON,
		RuntimeData:    sanitizeSnapshotRuntimeData(ctx.RuntimeData),
		CompletedSteps: buildCompletedSteps(ctx.ExecutionHistory),
		AppID:          ctx.Application.ID,
	})
	if err != nil {
		return err
	}
	// The service declined the save because the freshly authenticated subject conflicts with the
	// existing session's subject; degrade SSO without failing authentication.
	if result.Skipped {
		return nil
	}

	execResp.RuntimeData[savedKey] = result.Handle
	// Publish the session handle as the shared hint so later joins in this execution attach to the
	// same session directly.
	execResp.RuntimeData[common.RuntimeKeySSOSessionHandle] = result.Handle
	// Emit the cookie only when this call minted the session, and only now that its first checkpoint
	// is durably saved — so a context-write failure never leaves a cookie for an empty session.
	if result.Created {
		setHandleOut(execResp, result.Handle)
	}
	logger.Debug(ctx.Context, "Saved SSO checkpoint", log.String("checkpoint", checkpoint))
	return nil
}

// setHandleOut records a minted session handle on the response's EngineData channel — engine-only
// output that the flow engine lifts onto the flow step for the transport layer to set the per-flow
// cookie. EngineData is never returned to the client, so the handle does not leak into the response,
// and using a generic channel (not a dedicated field) keeps SSO concepts off the engine contract.
func setHandleOut(execResp *providers.ExecutorResponse, handle string) {
	if execResp.EngineData == nil {
		execResp.EngineData = make(map[string]string)
	}
	execResp.EngineData[common.RuntimeKeySSOSessionHandle] = handle
}

// loadCheckpoint loads a checkpoint's saved flow state into the execution context so downstream
// nodes continue with the authenticated subject and claims. The SSO session service fetches the
// session and its checkpoint context (and refreshes the session's activity); this executor
// rehydrates the subject and replays the snapshotted runtime state.
func (e *sessionExecutor) loadCheckpoint(ctx *providers.NodeContext, execResp *providers.ExecutorResponse,
	checkpoint string, logger *log.Logger) error {
	handle := ctx.RuntimeData[common.RuntimeKeySSOSessionHandle]
	sess, sc, err := e.sso.LoadCheckpoint(ctx.Context, handle, checkpoint, ctx.Application.ID)
	if err != nil {
		return err
	}

	// Rehydrate the AuthUser from the snapshot verbatim — it was stored as-is — so downstream nodes
	// continue with the same subject and attributes this session resolved when the checkpoint was saved.
	var authUser providers.AuthUser
	if err := json.Unmarshal(sc.AuthUser, &authUser); err != nil {
		return fmt.Errorf("failed to rehydrate subject reference from snapshot: %w", err)
	}
	execResp.AuthUser = authUser

	// Replay the snapshotted RuntimeData (the effective attribute set captured at save) so downstream
	// nodes see the same attributes the fresh path produced.
	for k, v := range sc.RuntimeData {
		execResp.RuntimeData[k] = v
	}
	// auth_time comes from the lean session, not the context. Set it after the RuntimeData replay so
	// the live, session-derived value wins over any stale snapshot copy.
	if !sess.AuthenticatedAt.IsZero() {
		execResp.RuntimeData[common.RuntimeKeyAuthTime] = strconv.FormatInt(sess.AuthenticatedAt.Unix(), 10)
	}

	logger.Debug(ctx.Context, "Loaded SSO checkpoint",
		log.String("flowId", session.SSOInputsFrom(ctx.Context).FlowID),
		log.String("checkpoint", checkpoint))
	return nil
}

// requestScopedSnapshotDenyList holds request-scoped RuntimeData keys that must not ride along in a
// checkpoint snapshot: they belong to the establishing app's authorization request and, if replayed
// onto a different app joining via SSO, override that app's own attribute/scope requirements (so the
// joining app releases only the establishing app's attributes). This is a local stopgap for the keys
// observed to break attribute release; it should move to a central place when the flow-context data
// classification is implemented.
var requestScopedSnapshotDenyList = map[string]struct{}{
	common.RuntimeKeyRequestedPermissions:        {},
	common.RuntimeKeyRequiredEssentialAttributes: {},
	common.RuntimeKeyRequiredOptionalAttributes:  {},
	common.RuntimeKeyRequiredLocales:             {},
	common.RuntimeKeyClientID:                    {},
	common.RuntimeKeyAuthorizationRequestID:      {},
	// applicationId has no shared constant (set as a raw literal in enrichRuntimeData).
	"applicationId": {},
}

// sanitizeSnapshotRuntimeData copies RuntimeData for the durable snapshot, dropping the transient SSO
// control keys (the per-checkpoint present/saved flags and the shared handle hint) and the
// request-scoped keys in requestScopedSnapshotDenyList. Persisting the control keys would let a
// reused snapshot reinject a prior run's control state when its RuntimeData is replayed on load;
// persisting the request-scoped keys would override a joining app's own request. RuntimeData is
// otherwise persisted in full pending the flow-context data-classification revisit. Returns nil when
// nothing durable remains.
func sanitizeSnapshotRuntimeData(rd map[string]string) map[string]string {
	if len(rd) == 0 {
		return nil
	}
	out := make(map[string]string, len(rd))
	for k, v := range rd {
		if _, denied := requestScopedSnapshotDenyList[k]; denied {
			continue
		}
		if k == common.RuntimeKeySSOSessionHandle ||
			strings.HasPrefix(k, common.RuntimeKeySSOSessionPresent+":") ||
			strings.HasPrefix(k, common.RuntimeKeySSOSessionSaved+":") {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// buildCompletedSteps projects the execution history into the bounded per-node step facts kept
// in the session context. Only completed authentication steps are recorded; control/utility nodes
// (START/END, SSO-Check, prompts, authorization) are not authentication-event facts.
func buildCompletedSteps(history map[string]*providers.NodeExecutionRecord) map[string]session.StepFact {
	if len(history) == 0 {
		return nil
	}
	steps := make(map[string]session.StepFact)
	for nodeID, record := range history {
		if record == nil {
			continue
		}
		if record.ExecutorType != providers.ExecutorTypeAuthentication ||
			record.Status != providers.FlowStatusComplete {
			continue
		}
		steps[nodeID] = session.StepFact{
			Executor:    record.ExecutorName,
			Status:      string(record.Status),
			CompletedAt: record.EndTime / 1000, // NodeExecutionRecord.EndTime is Unix millis.
		}
	}
	if len(steps) == 0 {
		return nil
	}
	return steps
}

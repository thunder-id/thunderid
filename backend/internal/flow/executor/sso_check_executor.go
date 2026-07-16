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

	present := false
	if resolved != nil && checkpoint != "" {
		if present, err = e.sso.HasCheckpoint(ctx.Context, resolved.SessionID, checkpoint); err != nil {
			return execResp, err
		}
	}

	if present {
		execResp.Status = providers.ExecComplete
		execResp.RuntimeData[presentKey] = dataValueTrue
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

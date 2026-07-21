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
func (e *sessionSignOutExecutor) Execute(ctx *providers.NodeContext) (*providers.ExecutorResponse, error) {
	logger := e.logger.With(log.String(log.LoggerKeyExecutionID, ctx.ExecutionID))

	execResp := &providers.ExecutorResponse{
		Status:      providers.ExecComplete,
		RuntimeData: make(map[string]string),
		EngineData:  make(map[string]string),
	}

	in := session.SSOInputsFrom(ctx.Context)
	if _, err := e.sso.Terminate(ctx.Context, in.Handle, in.FlowID); err != nil {
		return execResp, err
	}

	// Signal the transport layer to clear the per-flow cookie. The engine resolves the flow id
	// (the login flow) from the execution's SessionFlowID. The post-logout redirect is not the flow's
	// concern — the OAuth layer resolves it on the sign-out completion callback.
	execResp.EngineData[common.RuntimeKeySSOSessionCleared] = dataValueTrue

	logger.Debug(ctx.Context, "Terminated SSO session on sign-out", log.String("flowId", in.FlowID))
	return execResp, nil
}

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
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
)

type SessionSignOutExecutorTestSuite struct {
	suite.Suite
}

func TestSessionSignOutExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(SessionSignOutExecutorTestSuite))
}

func (suite *SessionSignOutExecutorTestSuite) SetupTest() {
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &config.Config{}))
}

func (suite *SessionSignOutExecutorTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *SessionSignOutExecutorTestSuite) newExecutor(sso session.Service) *sessionSignOutExecutor {
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	return newSessionSignOutExecutor(flowFactory, sso)
}

// signOutNodeContext carries the login flow's inbound handle and flow id, as the engine delivers them
// through the SSO inputs for a sign-out flow.
func signOutNodeContext() *providers.NodeContext {
	return &providers.NodeContext{
		Context: session.WithSSOInputs(context.Background(), session.SSOInputs{
			Handle: "handle-abc",
			FlowID: "flow-1",
		}),
		ExecutionID: "exec-1",
	}
}

// TestTerminatesAndSignalsClear covers a live session: it is ended and the cookie-clear signal is
// raised on the engine-only channel.
func (suite *SessionSignOutExecutorTestSuite) TestTerminatesAndSignalsClear() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Terminate(mock.Anything, "handle-abc", "flow-1").
		Return(&session.Session{SessionID: "sess-1", State: session.StateEnded}, nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(signOutNodeContext())

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.EngineData[common.RuntimeKeySSOSessionCleared])
}

// TestClearsWhenNoSession covers sign-out when no session backs the handle: Terminate is a no-op but
// the cookie is still cleared so the browser drops any stale handle.
func (suite *SessionSignOutExecutorTestSuite) TestClearsWhenNoSession() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Terminate(mock.Anything, "handle-abc", "flow-1").Return(nil, nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(signOutNodeContext())

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.EngineData[common.RuntimeKeySSOSessionCleared])
}

// TestTerminateError covers a store failure during termination: the executor surfaces the error and
// does not raise the clear signal.
func (suite *SessionSignOutExecutorTestSuite) TestTerminateError() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Terminate(mock.Anything, "handle-abc", "flow-1").Return(nil, errors.New("store down"))
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(signOutNodeContext())

	suite.Require().Error(err)
	suite.Empty(resp.EngineData[common.RuntimeKeySSOSessionCleared])
}

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

type SSOCheckExecutorTestSuite struct {
	suite.Suite
}

func TestSSOCheckExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(SSOCheckExecutorTestSuite))
}

func (suite *SSOCheckExecutorTestSuite) SetupTest() {
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &config.Config{}))
}

func (suite *SSOCheckExecutorTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *SSOCheckExecutorTestSuite) newExecutor(sso session.Service) *ssoCheckExecutor {
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	return newSSOCheckExecutor(flowFactory, sso)
}

func ssoNodeContext() *providers.NodeContext {
	return &providers.NodeContext{
		Context: session.WithSSOInputs(context.Background(), session.SSOInputs{
			Handle:      "handle-abc",
			FlowID:      "flow-1",
			FlowVersion: 3,
		}),
		ExecutionID:    "exec-1",
		NodeProperties: map[string]interface{}{common.NodePropertyCheckpointRef: "session"},
	}
}

func liveSession() *session.Session {
	return &session.Session{
		SessionID:   "sess-1",
		HandleID:    "handle-abc",
		FlowID:      "flow-1",
		FlowVersion: 3,
		State:       session.StateActive,
	}
}

// assertAbsent asserts the Authenticate outcome: the node fails (routing to onFailure) with the
// no-live-session error, records the decision, and stashes no handle.
func (suite *SSOCheckExecutorTestSuite) assertAbsent(resp *providers.ExecutorResponse) {
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Require().NotNil(resp.Error)
	suite.Equal(ErrNoLiveSSOSession.Code, resp.Error.Code)
	suite.Equal("false",
		resp.RuntimeData[common.SSOCheckpointKey(common.RuntimeKeySSOSessionPresent, "session")])
	suite.Empty(resp.RuntimeData[common.RuntimeKeySSOSessionHandle])
}

// TestPresent covers a live session that already holds this checkpoint: routes to Skip and shares the
// handle so the paired Session node loads the saved state.
func (suite *SSOCheckExecutorTestSuite) TestPresent() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).Return(liveSession(), nil)
	sso.EXPECT().HasCheckpoint(mock.Anything, "sess-1", "session").Return(true, nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(ssoNodeContext())

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue,
		resp.RuntimeData[common.SSOCheckpointKey(common.RuntimeKeySSOSessionPresent, "session")])
	suite.Equal("handle-abc", resp.RuntimeData[common.RuntimeKeySSOSessionHandle])
}

// TestAbsentCheckpointNotPresent covers a live session that lacks this checkpoint: the node routes to
// Authenticate so the stage authenticates fresh, but still shares the session handle so the fresh join
// attaches its new checkpoint to that same session.
func (suite *SSOCheckExecutorTestSuite) TestAbsentCheckpointNotPresent() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(liveSession(), nil)
	sso.EXPECT().HasCheckpoint(mock.Anything, "sess-1", "session").Return(false, nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(ssoNodeContext())

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal("false",
		resp.RuntimeData[common.SSOCheckpointKey(common.RuntimeKeySSOSessionPresent, "session")])
	// The handle is still shared because a live session exists.
	suite.Equal("handle-abc", resp.RuntimeData[common.RuntimeKeySSOSessionHandle])
}

// TestAbsentNoLiveSession covers the service resolving no live session (nil): the node routes to
// Authenticate and shares no handle.
func (suite *SSOCheckExecutorTestSuite) TestAbsentNoLiveSession() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(ssoNodeContext())

	suite.Require().NoError(err)
	suite.assertAbsent(resp)
}

// TestNoCheckpointRef covers a node not paired with a checkpoint: it routes to Authenticate without a
// checkpoint lookup.
func (suite *SSOCheckExecutorTestSuite) TestNoCheckpointRef() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)
	exec := suite.newExecutor(sso)
	ctx := ssoNodeContext()
	ctx.NodeProperties = nil

	resp, err := exec.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
}

// TestResolverErrorFailsFlow covers a session-resolution store failure surfacing from the service:
// Execute returns a Go error for the task-execution node to log and fail the flow.
func (suite *SSOCheckExecutorTestSuite) TestResolverErrorFailsFlow() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("store down"))
	exec := suite.newExecutor(sso)

	_, err := exec.Execute(ssoNodeContext())

	suite.Require().Error(err)
	suite.Contains(err.Error(), "store down")
}

// TestCheckpointListErrorFailsFlow covers a checkpoint-existence lookup failure on a live session:
// Execute returns a Go error rather than skipping on incomplete information.
func (suite *SSOCheckExecutorTestSuite) TestCheckpointListErrorFailsFlow() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(liveSession(), nil)
	sso.EXPECT().HasCheckpoint(mock.Anything, mock.Anything, mock.Anything).
		Return(false, errors.New("store down"))
	exec := suite.newExecutor(sso)

	_, err := exec.Execute(ssoNodeContext())

	suite.Require().Error(err)
	suite.Contains(err.Error(), "store down")
}

func (suite *SSOCheckExecutorTestSuite) TestCheckpointRef() {
	// Nil properties, missing key, and non-string values all resolve to no checkpoint.
	suite.Empty(checkpointRef(&providers.NodeContext{}))
	suite.Empty(checkpointRef(&providers.NodeContext{NodeProperties: map[string]interface{}{}}))
	suite.Empty(checkpointRef(&providers.NodeContext{
		NodeProperties: map[string]interface{}{common.NodePropertyCheckpointRef: 42},
	}))
	// A string value is returned as the checkpoint id.
	suite.Equal("session", checkpointRef(&providers.NodeContext{
		NodeProperties: map[string]interface{}{common.NodePropertyCheckpointRef: "session"},
	}))
}

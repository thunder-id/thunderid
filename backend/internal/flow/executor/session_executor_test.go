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
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
)

type SessionExecutorTestSuite struct {
	suite.Suite
}

func TestSessionExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(SessionExecutorTestSuite))
}

func (suite *SessionExecutorTestSuite) SetupTest() {
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &config.Config{}))
}

func (suite *SessionExecutorTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *SessionExecutorTestSuite) newExecutor(sso session.Service,
	authn providers.AuthnProviderManager) *sessionExecutor {
	flowFactory, _ := core.Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"))
	return newSessionExecutor(flowFactory, sso, authn)
}

// saveAuthnMock returns a provider that resolves the fresh-save subject (user-1 / ou-1 / person).
// The expectation is optional so guard-short-circuit tests can reuse it.
func (suite *SessionExecutorTestSuite) saveAuthnMock() *managermock.AuthnProviderManagerMock {
	m := managermock.NewAuthnProviderManagerMock(suite.T())
	resolved := authenticatedAuthUser()
	m.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(resolved, &providers.EntityReference{EntityID: "user-1", OUID: "ou-1", EntityType: "person"}, nil).
		Maybe()
	return m
}

// authenticatedAuthUser returns an AuthUser that reports IsAuthenticated() == true. Its tokens
// are opaque; the resolved subject is supplied by the mocked provider in each test.
func authenticatedAuthUser() providers.AuthUser {
	var authUser providers.AuthUser
	if err := authUser.UnmarshalJSON([]byte(`{"entityReferenceToken":"tok","attributeToken":"tok"}`)); err != nil {
		panic("authenticatedAuthUser: malformed hardcoded JSON: " + err.Error())
	}
	return authUser
}

func freshCtx() *providers.NodeContext {
	return &providers.NodeContext{
		Context: session.WithSSOInputs(context.Background(),
			session.SSOInputs{FlowID: "flow-1", FlowVersion: 3}),
		ExecutionID:   "exec-1",
		CurrentNodeID: "session",
		RuntimeData: map[string]string{
			"email": "alice@example.com",
			// An attribute derived by an in-flow step: it must survive the snapshot verbatim so the
			// SSO path reproduces it without re-running the step.
			"department": "eng",
		},
		AuthUser: authenticatedAuthUser(),
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{
			// Control node: must be excluded from the auth-event facts.
			"sso_check": {NodeID: "sso_check", ExecutorName: "SSOCheckExecutor",
				ExecutorType: providers.ExecutorTypeUtility, Status: providers.FlowStatusComplete,
				EndTime: 1700000000000},
			// Completed authentication step: recorded with its completion time (ms → s).
			"basic_auth": {NodeID: "basic_auth", ExecutorName: "CredentialsAuthExecutor",
				ExecutorType: providers.ExecutorTypeAuthentication, Status: providers.FlowStatusComplete,
				EndTime: 1700000005000},
		},
		Application: providers.Application{ID: "app-123"},
	}
}

// captureSave configures the service mock to record the save input and return the given result.
func captureSave(sso *sessionmock.ServiceMock, captured *session.SaveCheckpointInput,
	result session.SaveCheckpointResult) {
	sso.EXPECT().SaveCheckpoint(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, in session.SaveCheckpointInput) (session.SaveCheckpointResult, error) {
			*captured = in
			return result, nil
		})
}

// TestFreshSave verifies the save path hands the SSO service a correctly-built snapshot and, on a
// freshly minted session, publishes the handle to the transport (EngineData) and the shared
// RuntimeData hints.
func (suite *SessionExecutorTestSuite) TestFreshSave() {
	sso := sessionmock.NewServiceMock(suite.T())
	var in session.SaveCheckpointInput
	captureSave(sso, &in, session.SaveCheckpointResult{Handle: "handle-xyz", Created: true})
	exec := suite.newExecutor(sso, suite.saveAuthnMock())

	resp, err := exec.Execute(freshCtx())
	suite.Require().NoError(err)

	// The executor resolved the subject and handed the service a complete save input.
	suite.Equal("user-1", in.SubjectID)
	suite.Equal("flow-1", in.FlowID)
	suite.Equal(3, in.FlowVersion)
	suite.Equal("exec-1", in.ExecutionID)
	suite.Equal("session", in.Checkpoint)
	suite.Equal("app-123", in.AppID)
	// RuntimeData snapshot carries the in-flow-derived attribute.
	suite.Equal("alice@example.com", in.RuntimeData["email"])
	suite.Equal("eng", in.RuntimeData["department"])
	// AuthUser is snapshotted as-is and round-trips to the authenticated subject.
	var snapAuthUser providers.AuthUser
	suite.Require().NoError(json.Unmarshal(in.AuthUser, &snapAuthUser))
	suite.True(snapAuthUser.IsAuthenticated())
	// Only the completed authentication step is recorded (control node excluded), ms → s.
	suite.Contains(in.CompletedSteps, "basic_auth")
	suite.NotContains(in.CompletedSteps, "sso_check")
	suite.Equal(int64(1700000005), in.CompletedSteps["basic_auth"].CompletedAt)

	// A minted session emits the handle to the transport (EngineData, never returned to the client)
	// and records it per-checkpoint for idempotency plus as the shared hint.
	suite.Equal("handle-xyz", resp.EngineData[common.RuntimeKeySSOSessionHandle])
	suite.Equal("handle-xyz",
		resp.RuntimeData[common.SSOCheckpointKey(common.RuntimeKeySSOSessionSaved, "session")])
	suite.Equal("handle-xyz", resp.RuntimeData[common.RuntimeKeySSOSessionHandle])
	// The already-authenticated subject is echoed back so the engine keeps it.
	suite.True(resp.AuthUser.IsAuthenticated())
}

// TestFreshSave_AttachNoCookie covers attaching to an existing session (service reports
// Created=false): the handle is recorded on RuntimeData but no cookie is emitted.
func (suite *SessionExecutorTestSuite) TestFreshSave_AttachNoCookie() {
	sso := sessionmock.NewServiceMock(suite.T())
	var in session.SaveCheckpointInput
	captureSave(sso, &in, session.SaveCheckpointResult{Handle: "handle-abc", Created: false})
	exec := suite.newExecutor(sso, suite.saveAuthnMock())
	ctx := freshCtx()
	ctx.CurrentNodeID = "step_up"
	ctx.RuntimeData[common.RuntimeKeySSOSessionHandle] = "handle-abc"

	resp, err := exec.Execute(ctx)
	suite.Require().NoError(err)

	suite.Equal("handle-abc", in.HandleHint, "the shared handle hint is passed to the service")
	suite.Empty(resp.EngineData[common.RuntimeKeySSOSessionHandle], "no cookie is emitted when attaching")
	suite.Equal("handle-abc",
		resp.RuntimeData[common.SSOCheckpointKey(common.RuntimeKeySSOSessionSaved, "step_up")])
}

// TestFreshSave_SkippedNoEmission covers the service declining the save (subject conflict): nothing is
// emitted or recorded.
func (suite *SessionExecutorTestSuite) TestFreshSave_SkippedNoEmission() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().SaveCheckpoint(mock.Anything, mock.Anything).
		Return(session.SaveCheckpointResult{Skipped: true}, nil)
	exec := suite.newExecutor(sso, suite.saveAuthnMock())

	resp, err := exec.Execute(freshCtx())
	suite.Require().NoError(err)

	suite.Empty(resp.EngineData[common.RuntimeKeySSOSessionHandle])
	suite.Empty(resp.RuntimeData[common.SSOCheckpointKey(common.RuntimeKeySSOSessionSaved, "session")])
}

// TestFreshSave_SaveErrorIsNonFatal covers a service error on save: it degrades SSO without failing
// authentication.
func (suite *SessionExecutorTestSuite) TestFreshSave_SaveErrorIsNonFatal() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().SaveCheckpoint(mock.Anything, mock.Anything).
		Return(session.SaveCheckpointResult{}, errors.New("db down"))
	exec := suite.newExecutor(sso, suite.saveAuthnMock())

	resp, err := exec.Execute(freshCtx())

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Empty(resp.EngineData[common.RuntimeKeySSOSessionHandle])
}

// TestFreshSave_Idempotent covers a checkpoint already saved in this execution: the handle is
// re-emitted from RuntimeData without calling the service (no SaveCheckpoint expectation is set).
func (suite *SessionExecutorTestSuite) TestFreshSave_Idempotent() {
	sso := sessionmock.NewServiceMock(suite.T())
	exec := suite.newExecutor(sso, suite.saveAuthnMock())
	ctx := freshCtx()
	ctx.RuntimeData[common.SSOCheckpointKey(common.RuntimeKeySSOSessionSaved, "session")] = "existing-handle"

	resp, err := exec.Execute(ctx)
	suite.Require().NoError(err)

	suite.Equal("existing-handle", resp.EngineData[common.RuntimeKeySSOSessionHandle])
}

// TestFreshSave_Unauthenticated covers no authenticated subject: the service is never called.
func (suite *SessionExecutorTestSuite) TestFreshSave_Unauthenticated() {
	sso := sessionmock.NewServiceMock(suite.T())
	exec := suite.newExecutor(sso, suite.saveAuthnMock())
	ctx := freshCtx()
	ctx.AuthUser = providers.AuthUser{}

	resp, err := exec.Execute(ctx)
	suite.Require().NoError(err)

	suite.Empty(resp.EngineData[common.RuntimeKeySSOSessionHandle])
}

// TestFreshSave_EntityReferenceErrorIsNonFatal covers a subject-resolution failure: the save is
// skipped (service never called) without failing authentication.
func (suite *SessionExecutorTestSuite) TestFreshSave_EntityReferenceErrorIsNonFatal() {
	m := managermock.NewAuthnProviderManagerMock(suite.T())
	m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
		Return(authenticatedAuthUser(), nil, &ErrNoLiveSSOSession)
	sso := sessionmock.NewServiceMock(suite.T())
	exec := suite.newExecutor(sso, m)

	resp, err := exec.Execute(freshCtx())

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

// TestFreshSave_NoResolvedSubjectSkips covers a resolved reference with no entity id: the save is
// skipped without calling the service.
func (suite *SessionExecutorTestSuite) TestFreshSave_NoResolvedSubjectSkips() {
	m := managermock.NewAuthnProviderManagerMock(suite.T())
	m.EXPECT().GetEntityReference(mock.Anything, mock.Anything).
		Return(authenticatedAuthUser(), &providers.EntityReference{EntityID: ""}, nil)
	sso := sessionmock.NewServiceMock(suite.T())
	exec := suite.newExecutor(sso, m)

	resp, err := exec.Execute(freshCtx())

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

// TestFreshSave_SanitizesSnapshot verifies the input the executor hands the service excludes both the
// transient SSO control keys and the request-scoped keys.
func (suite *SessionExecutorTestSuite) TestFreshSave_SanitizesSnapshot() {
	sso := sessionmock.NewServiceMock(suite.T())
	var in session.SaveCheckpointInput
	captureSave(sso, &in, session.SaveCheckpointResult{Handle: "h", Created: true})
	exec := suite.newExecutor(sso, suite.saveAuthnMock())

	ctx := freshCtx()
	ctx.RuntimeData[common.RuntimeKeySSOSessionHandle] = "some-handle"
	ctx.RuntimeData[common.SSOCheckpointKey(common.RuntimeKeySSOSessionPresent, "other")] = dataValueTrue
	ctx.RuntimeData[common.SSOCheckpointKey(common.RuntimeKeySSOSessionSaved, "other")] = "h"
	ctx.RuntimeData[common.RuntimeKeyRequiredEssentialAttributes] = "email"
	ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes] = "phone"
	ctx.RuntimeData[common.RuntimeKeyRequiredLocales] = "en-US"
	ctx.RuntimeData[common.RuntimeKeyRequestedPermissions] = "openid profile"
	ctx.RuntimeData["applicationId"] = "app-a"
	ctx.RuntimeData[common.RuntimeKeyClientID] = "sso_app_a"
	ctx.RuntimeData[common.RuntimeKeyAuthorizationRequestID] = "authz-req-1"

	_, err := exec.Execute(ctx)
	suite.Require().NoError(err)

	rd := in.RuntimeData
	// Durable business data survives.
	suite.Equal("alice@example.com", rd["email"])
	suite.Equal("eng", rd["department"])
	// Transient SSO control keys are stripped.
	suite.NotContains(rd, common.RuntimeKeySSOSessionHandle)
	suite.NotContains(rd, common.SSOCheckpointKey(common.RuntimeKeySSOSessionPresent, "other"))
	suite.NotContains(rd, common.SSOCheckpointKey(common.RuntimeKeySSOSessionSaved, "other"))
	// Request-scoped keys are stripped so they cannot override a joining app's request.
	suite.NotContains(rd, common.RuntimeKeyRequiredEssentialAttributes)
	suite.NotContains(rd, common.RuntimeKeyRequiredOptionalAttributes)
	suite.NotContains(rd, common.RuntimeKeyRequiredLocales)
	suite.NotContains(rd, common.RuntimeKeyRequestedPermissions)
	suite.NotContains(rd, "applicationId")
	suite.NotContains(rd, common.RuntimeKeyClientID)
	suite.NotContains(rd, common.RuntimeKeyAuthorizationRequestID)
}

func ssoLoadCtx() *providers.NodeContext {
	return &providers.NodeContext{
		Context: session.WithSSOInputs(context.Background(),
			session.SSOInputs{FlowID: "flow-1", FlowVersion: 3}),
		ExecutionID:   "exec-2",
		CurrentNodeID: "session",
		RuntimeData: map[string]string{
			common.SSOCheckpointKey(common.RuntimeKeySSOSessionPresent, "session"): dataValueTrue,
			common.RuntimeKeySSOSessionHandle:                                      "handle-abc",
		},
		Application: providers.Application{ID: "app-456"},
	}
}

// TestSSOLoad verifies the load path rehydrates the subject from the service-returned context, replays
// the snapshotted RuntimeData, and overrides auth_time from the lean session.
func (suite *SessionExecutorTestSuite) TestSSOLoad() {
	snapAuthUser := `{"entityReference":{"entityId":"user-2","ouId":"ou-9","type":"person"},` +
		`"attributes":{"attributes":{"email":{"value":"bob@example.com"}}}}`
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().LoadCheckpoint(mock.Anything, "handle-abc", "session", "app-456").Return(
		&session.Session{
			SessionID: "sess-1", SubjectID: "user-2", HandleID: "handle-abc",
			AuthenticatedAt: time.Unix(1700000000, 0).UTC(),
		},
		&session.SessionContext{
			SessionID: "sess-1",
			RuntimeData: map[string]string{
				"email":      "bob@example.com",
				"department": "eng",
				// A stale auth_time in the snapshot must be overridden by the session's value on load.
				common.RuntimeKeyAuthTime: "1600000000",
			},
			AuthUser:       json.RawMessage(snapAuthUser),
			ContextVersion: 1,
		}, nil)
	// The load path rehydrates the subject from the snapshot and never calls the provider.
	exec := suite.newExecutor(sso, managermock.NewAuthnProviderManagerMock(suite.T()))

	resp, err := exec.Execute(ssoLoadCtx())
	suite.Require().NoError(err)

	// The AuthUser is rehydrated verbatim from the snapshot.
	suite.True(resp.AuthUser.IsAuthenticated())
	au := resp.AuthUser
	raw, marshalErr := json.Marshal(&au)
	suite.Require().NoError(marshalErr)
	rendered := string(raw)
	suite.True(strings.Contains(rendered, `"entityId":"user-2"`), rendered)
	suite.True(strings.Contains(rendered, "bob@example.com"), rendered)
	// The snapshotted RuntimeData is replayed, including the in-flow-derived attribute.
	suite.Equal("bob@example.com", resp.RuntimeData["email"])
	suite.Equal("eng", resp.RuntimeData["department"])
	// auth_time comes from the lean session and wins over the stale snapshot copy.
	suite.Equal("1700000000", resp.RuntimeData[common.RuntimeKeyAuthTime])
}

// TestSSOLoad_ErrorFailsFlow covers a load failure surfacing as a server error so the task-execution
// node fails the flow (the credential steps were already skipped).
func (suite *SessionExecutorTestSuite) TestSSOLoad_ErrorFailsFlow() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().LoadCheckpoint(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil, errors.New("resolved session no longer exists"))
	exec := suite.newExecutor(sso, managermock.NewAuthnProviderManagerMock(suite.T()))

	_, err := exec.Execute(ssoLoadCtx())

	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to load SSO checkpoint")
}

// TestSSOLoad_RehydrateErrorFailsFlow covers an unparseable AuthUser snapshot: the executor cannot
// reconstruct the subject, so the flow fails.
func (suite *SessionExecutorTestSuite) TestSSOLoad_RehydrateErrorFailsFlow() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().LoadCheckpoint(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&session.Session{SessionID: "sess-1", HandleID: "handle-abc"},
		&session.SessionContext{SessionID: "sess-1", AuthUser: json.RawMessage("not-json")}, nil)
	exec := suite.newExecutor(sso, managermock.NewAuthnProviderManagerMock(suite.T()))

	_, err := exec.Execute(ssoLoadCtx())

	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to load SSO checkpoint")
}

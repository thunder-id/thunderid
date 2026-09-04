// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/session"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/flow/sessionmock"
)

// reauthNodeContext builds a node context carrying the given runtime data alongside the standard
// SSO inputs, so a test can state only the request parameters it cares about.
func reauthNodeContext(runtimeData map[string]string) *providers.NodeContext {
	ctx := ssoNodeContext()
	ctx.RuntimeData = runtimeData
	return ctx
}

// sessionAuthenticatedAgo returns a live session whose subject authenticated the given duration ago.
func sessionAuthenticatedAgo(d time.Duration) *session.Session {
	s := liveSession()
	s.AuthenticatedAt = time.Now().UTC().Add(-d)
	return s
}

// TestReauthPromptLogin covers prompt=login: the session is live and holds the checkpoint, but the
// request demands a fresh authentication, so the node routes to Authenticate anyway. The checkpoint
// is never looked up, because the decision does not depend on it.
func (suite *SSOCheckExecutorTestSuite) TestReauthPromptLogin() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).
		Return(sessionAuthenticatedAgo(time.Minute), nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(reauthNodeContext(map[string]string{
		common.RuntimeKeyForceReauth: dataValueTrue,
	}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Require().NotNil(resp.Error)
	suite.Equal(ErrNoLiveSSOSession.Code, resp.Error.Code)
	// The handle is still shared, so the fresh authentication attaches to the existing session
	// rather than minting a second one for the same subject.
	suite.Equal("handle-abc", resp.RuntimeData[common.RuntimeKeySSOSessionHandle])
}

// TestReauthMaxAgeExceeded covers a session older than max_age: the existing authentication cannot
// satisfy the request, so the node re-authenticates instead of reusing it.
func (suite *SSOCheckExecutorTestSuite) TestReauthMaxAgeExceeded() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).
		Return(sessionAuthenticatedAgo(time.Hour), nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(reauthNodeContext(map[string]string{
		common.RuntimeKeyMaxAge: "60",
	}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal("handle-abc", resp.RuntimeData[common.RuntimeKeySSOSessionHandle])
}

// TestReauthMaxAgeWithinLimit covers the opposite case: a max_age the session still satisfies
// leaves reuse intact, so the node routes to Skip.
func (suite *SSOCheckExecutorTestSuite) TestReauthMaxAgeWithinLimit() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).
		Return(sessionAuthenticatedAgo(time.Minute), nil)
	sso.EXPECT().FindCheckpoint(mock.Anything, "sess-1", "session").
		Return(&session.SessionContext{SessionID: "sess-1", CheckpointID: "session"}, nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(reauthNodeContext(map[string]string{
		common.RuntimeKeyMaxAge: "3600",
	}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

// TestReauthMalformedMaxAge covers a max_age that does not parse: it is treated as no constraint,
// matching how the assurance check reads the same value, so reuse is unaffected.
func (suite *SSOCheckExecutorTestSuite) TestReauthMalformedMaxAge() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).
		Return(sessionAuthenticatedAgo(time.Hour), nil)
	sso.EXPECT().FindCheckpoint(mock.Anything, "sess-1", "session").
		Return(&session.SessionContext{SessionID: "sess-1", CheckpointID: "session"}, nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(reauthNodeContext(map[string]string{
		common.RuntimeKeyMaxAge: "not-a-number",
	}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

// TestReauthZeroAuthenticatedAt covers a session carrying no authentication time: there is nothing
// to compare max_age against, so the value is not treated as a breach.
func (suite *SSOCheckExecutorTestSuite) TestReauthZeroAuthenticatedAt() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).
		Return(liveSession(), nil)
	sso.EXPECT().FindCheckpoint(mock.Anything, "sess-1", "session").
		Return(&session.SessionContext{SessionID: "sess-1", CheckpointID: "session"}, nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(reauthNodeContext(map[string]string{
		common.RuntimeKeyMaxAge: "60",
	}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

// TestReauthMaxAgeZeroAlwaysReauthenticates covers max_age=0, which admits no elapsed time at all:
// any session with a recorded authentication time is too old to reuse.
func (suite *SSOCheckExecutorTestSuite) TestReauthMaxAgeZeroAlwaysReauthenticates() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).
		Return(sessionAuthenticatedAgo(2*time.Second), nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(reauthNodeContext(map[string]string{
		common.RuntimeKeyMaxAge: "0",
	}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
}

// TestReauthNotRequestedReusesSession covers the baseline: with neither prompt=login nor max_age,
// a live session is reused unchanged.
func (suite *SSOCheckExecutorTestSuite) TestReauthNotRequestedReusesSession() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).
		Return(sessionAuthenticatedAgo(24*time.Hour), nil)
	sso.EXPECT().FindCheckpoint(mock.Anything, "sess-1", "session").
		Return(&session.SessionContext{SessionID: "sess-1", CheckpointID: "session"}, nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(reauthNodeContext(map[string]string{}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

// TestReauthAuthTimeBoundary covers a session exactly at the max_age boundary. The comparison is
// strictly greater than, so a session precisely max_age old still satisfies the request.
func (suite *SSOCheckExecutorTestSuite) TestReauthAuthTimeBoundary() {
	s := liveSession()
	s.AuthenticatedAt = time.Now().UTC().Add(-60 * time.Second)

	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).Return(s, nil)
	sso.EXPECT().FindCheckpoint(mock.Anything, "sess-1", "session").
		Return(&session.SessionContext{SessionID: "sess-1", CheckpointID: "session"}, nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(reauthNodeContext(map[string]string{
		common.RuntimeKeyMaxAge: "120",
	}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

// TestReauthSilentRequestReusesBoundarySession covers the race between the authorize endpoint's
// max_age check and this node's. Both compare against a fresh clock, so a session sitting on the
// boundary passes there and would fail here once the second ticks over, prompting a request that
// forbids prompting. A silent request therefore honors the decision already made.
func (suite *SSOCheckExecutorTestSuite) TestReauthSilentRequestReusesBoundarySession() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).
		Return(sessionAuthenticatedAgo(time.Hour), nil)
	sso.EXPECT().FindCheckpoint(mock.Anything, "sess-1", "session").
		Return(&session.SessionContext{SessionID: "sess-1", CheckpointID: "session"}, nil)
	exec := suite.newExecutor(sso)

	// Stale by this node's own comparison, but the request forbids interaction.
	resp, err := exec.Execute(reauthNodeContext(map[string]string{
		common.RuntimeKeyMaxAge:         "60",
		common.RuntimeKeySilentAuthOnly: dataValueTrue,
	}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status,
		"a silent request must reuse the session rather than route to a credential prompt")
}

// TestReauthSilentRequestStillHonorsForceReauth guards the precedence. prompt=none cannot be
// combined with prompt=login, so if both markers ever appear the explicit forced re-authentication
// must win rather than being suppressed by the silent marker.
func (suite *SSOCheckExecutorTestSuite) TestReauthSilentRequestStillHonorsForceReauth() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).
		Return(sessionAuthenticatedAgo(time.Minute), nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(reauthNodeContext(map[string]string{
		common.RuntimeKeyForceReauth:    dataValueTrue,
		common.RuntimeKeySilentAuthOnly: dataValueTrue,
	}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status,
		"an explicit force-reauth must not be suppressed by the silent marker")
}

// TestReauthMaxAgeZeroWithinSameSecond covers the boundary a strictly-greater comparison misses: a
// session authenticated in the current Unix second yields an elapsed time of zero, which is not
// greater than max_age=0. OIDC Core treats max_age=0 as equivalent to prompt=login, so it must
// still re-authenticate.
func (suite *SSOCheckExecutorTestSuite) TestReauthMaxAgeZeroWithinSameSecond() {
	sso := sessionmock.NewServiceMock(suite.T())
	sso.EXPECT().Resolve(mock.Anything, "handle-abc", "flow-1", 3, mock.Anything).
		Return(sessionAuthenticatedAgo(0), nil)
	exec := suite.newExecutor(sso)

	resp, err := exec.Execute(reauthNodeContext(map[string]string{
		common.RuntimeKeyMaxAge: "0",
	}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status,
		"max_age=0 must re-authenticate even within the same second as the authentication")
}

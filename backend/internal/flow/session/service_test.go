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

package session

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
)

type ServiceTestSuite struct {
	suite.Suite
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (suite *ServiceTestSuite) SetupTest() {
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &config.Config{}))
}

func (suite *ServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// serviceMocks bundles the generated store/transaction mocks a service test wires together. The one
// store mock backs every persistence operation (sessions, contexts, participants) since the service
// depends on the single sessionStore interface.
type serviceMocks struct {
	store *sessionStoreMock
	tx    *transactionmock.TransactionerMock
}

func (suite *ServiceTestSuite) newService() (*service, *serviceMocks) {
	m := &serviceMocks{
		store: newSessionStoreMock(suite.T()),
		tx:    transactionmock.NewTransactionerMock(suite.T()),
	}
	svc := &service{
		store:         m.store,
		resolver:      newResolver(m.store),
		transactioner: m.tx,
		timeouts:      DefaultTimeouts(),
		logger:        log.GetLogger(),
	}
	return svc, m
}

// runTx makes the transaction mock execute the callback it is handed (commit-on-success semantics).
func runTx(m *serviceMocks) {
	m.tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) })
}

func liveStoreSession() *Session {
	return &Session{
		SessionID: "sess-1", SubjectID: "user-1", HandleID: "handle-abc",
		FlowID: "flow-1", FlowVersion: 3, State: StateActive,
	}
}

// --- Resolve ---

func (suite *ServiceTestSuite) TestResolve_Hit() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)

	got, err := svc.Resolve(context.Background(), "handle-abc", "flow-1", 3, time.Now().UTC())

	suite.Require().NoError(err)
	suite.Require().NotNil(got)
	suite.Equal("sess-1", got.SessionID)
}

func (suite *ServiceTestSuite) TestResolve_NoHandle() {
	svc, _ := suite.newService()

	got, err := svc.Resolve(context.Background(), "", "flow-1", 3, time.Now().UTC())

	suite.Require().NoError(err)
	suite.Nil(got)
}

func (suite *ServiceTestSuite) TestResolve_DifferentFlow() {
	svc, m := suite.newService()
	s := liveStoreSession()
	s.FlowID = "other-flow"
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).Return(s, nil)

	got, err := svc.Resolve(context.Background(), "handle-abc", "flow-1", 3, time.Now().UTC())

	suite.Require().NoError(err)
	suite.Nil(got, "a session from a different flow must not be reused")
}

func (suite *ServiceTestSuite) TestResolve_VersionMismatch() {
	svc, m := suite.newService()
	s := liveStoreSession()
	s.FlowVersion = 2
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).Return(s, nil)

	got, err := svc.Resolve(context.Background(), "handle-abc", "flow-1", 3, time.Now().UTC())

	suite.Require().NoError(err)
	suite.Nil(got, "an incompatible flow version must force full authentication")
}

func (suite *ServiceTestSuite) TestResolve_StoreError() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).Return(nil, errors.New("store down"))

	_, err := svc.Resolve(context.Background(), "handle-abc", "flow-1", 3, time.Now().UTC())

	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to resolve SSO session")
}

// --- HasCheckpoint ---

func (suite *ServiceTestSuite) TestHasCheckpoint_Present() {
	svc, m := suite.newService()
	m.store.EXPECT().ListCheckpointIDs(mock.Anything, "sess-1").Return([]string{"password", "session"}, nil)

	present, err := svc.HasCheckpoint(context.Background(), "sess-1", "session")
	suite.Require().NoError(err)
	suite.True(present)
}

func (suite *ServiceTestSuite) TestHasCheckpoint_Absent() {
	svc, m := suite.newService()
	m.store.EXPECT().ListCheckpointIDs(mock.Anything, "sess-1").Return([]string{"password"}, nil)

	present, err := svc.HasCheckpoint(context.Background(), "sess-1", "session")
	suite.Require().NoError(err)
	suite.False(present)
}

func (suite *ServiceTestSuite) TestHasCheckpoint_ListError() {
	svc, m := suite.newService()
	m.store.EXPECT().ListCheckpointIDs(mock.Anything, mock.Anything).Return(nil, errors.New("store down"))

	_, err := svc.HasCheckpoint(context.Background(), "sess-1", "session")

	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to list SSO session checkpoints")
}

// --- SaveCheckpoint ---

func saveInput() SaveCheckpointInput {
	return SaveCheckpointInput{
		SubjectID: "user-1", FlowID: "flow-1", FlowVersion: 3, ExecutionID: "exec-1",
		Checkpoint: "session", AuthUser: json.RawMessage(`{"entityReference":{"entityId":"user-1"}}`),
		RuntimeData: map[string]string{"email": "alice@example.com"}, AppID: "app-123",
	}
}

func (suite *ServiceTestSuite) TestSaveCheckpoint_Establishes() {
	svc, m := suite.newService()
	// Model the establish sequence: GetByExecutionID returns nil until Create persists the row, then
	// returns it (so the re-read finds this call's own session and reports Created).
	var created *Session
	m.store.EXPECT().GetByExecutionID(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, string) (*Session, error) { return created, nil })
	m.store.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, s Session) error { created = &s; return nil })
	runTx(m)
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(nil)

	res, err := svc.SaveCheckpoint(context.Background(), saveInput())
	suite.Require().NoError(err)

	suite.True(res.Created)
	suite.NotEmpty(res.Handle)
	suite.Require().NotNil(created)
	suite.Equal("user-1", created.SubjectID)
	suite.Equal("flow-1", created.FlowID)
	suite.Equal(3, created.FlowVersion)
	suite.Equal(StateActive, created.State)
	suite.True(created.IdleExpiresAt.After(created.CreatedAt))
	suite.True(created.AbsoluteExpiresAt.After(created.IdleExpiresAt))
}

func (suite *ServiceTestSuite) TestSaveCheckpoint_AttachesToExisting() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	var savedCtx SessionContext
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, c SessionContext) error { savedCtx = c; return nil })
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(nil)

	in := saveInput()
	in.Checkpoint = "step_up"
	in.HandleHint = "handle-abc"

	res, err := svc.SaveCheckpoint(context.Background(), in)
	suite.Require().NoError(err)

	suite.False(res.Created, "attaching to an existing session must not mint a new one")
	suite.Equal("handle-abc", res.Handle)
	suite.Equal("sess-1", savedCtx.SessionID)
	suite.Equal("step_up", savedCtx.CheckpointID)
}

func (suite *ServiceTestSuite) TestSaveCheckpoint_SubjectMismatchSkips() {
	svc, m := suite.newService()
	existing := liveStoreSession()
	existing.SubjectID = "someone-else"
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(existing, nil)

	in := saveInput()
	in.HandleHint = "handle-abc"

	res, err := svc.SaveCheckpoint(context.Background(), in)
	suite.Require().NoError(err)

	suite.True(res.Skipped, "a subject mismatch must not cross-attach")
}

func (suite *ServiceTestSuite) TestSaveCheckpoint_EstablishError() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByExecutionID(mock.Anything, mock.Anything).Return(nil, nil)
	m.store.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("insert failed"))

	_, err := svc.SaveCheckpoint(context.Background(), saveInput())

	suite.Require().Error(err)
}

func (suite *ServiceTestSuite) TestSaveCheckpoint_ContextWriteError() {
	svc, m := suite.newService()
	var created *Session
	m.store.EXPECT().GetByExecutionID(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, string) (*Session, error) { return created, nil })
	m.store.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, s Session) error { created = &s; return nil })
	runTx(m)
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(errors.New("db down"))

	_, err := svc.SaveCheckpoint(context.Background(), saveInput())

	suite.Require().Error(err)
}

// --- LoadCheckpoint ---

func (suite *ServiceTestSuite) TestLoadCheckpoint_Success() {
	originalIdle := time.Unix(1700000600, 0).UTC()
	originalAbsolute := time.Unix(1700050000, 0).UTC()
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(&Session{
		SessionID: "sess-1", HandleID: "handle-abc", AuthenticatedAt: time.Unix(1700000000, 0).UTC(),
		IdleExpiresAt: originalIdle, AbsoluteExpiresAt: originalAbsolute, State: StateActive,
	}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").
		Return(&SessionContext{SessionID: "sess-1"}, nil)
	var updated *Session
	m.store.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, s *Session) error { updated = s; return nil })
	var recorded Participant
	m.store.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p Participant) error { recorded = p; return nil })

	sess, sc, err := svc.LoadCheckpoint(context.Background(), "handle-abc", "session", "app-456", "tfid-1")
	suite.Require().NoError(err)

	suite.Require().NotNil(sess)
	suite.Require().NotNil(sc)
	// Activity touch: last-active refreshed, idle slid forward, absolute unchanged.
	suite.Require().NotNil(updated)
	suite.False(updated.LastActiveAt.IsZero())
	suite.True(updated.IdleExpiresAt.After(originalIdle))
	suite.Equal(originalAbsolute, updated.AbsoluteExpiresAt)
	// The joining application is recorded.
	suite.Equal("app-456", recorded.AppID)
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_NoHandle() {
	svc, _ := suite.newService()

	_, _, err := svc.LoadCheckpoint(context.Background(), "", "session", "app-456", "tfid-1")

	suite.Require().Error(err)
	suite.Contains(err.Error(), "no resolved session handle")
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_MissingSession() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).Return(nil, nil)

	_, _, err := svc.LoadCheckpoint(context.Background(), "handle-abc", "session", "app-456", "tfid-1")

	suite.Require().Error(err)
	suite.Contains(err.Error(), "resolved session no longer exists")
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_MissingContext() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).
		Return(&Session{SessionID: "sess-1", HandleID: "handle-abc", State: StateActive}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	_, _, err := svc.LoadCheckpoint(context.Background(), "handle-abc", "session", "app-456", "tfid-1")

	suite.Require().Error(err)
	suite.Contains(err.Error(), "session context for checkpoint")
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_ParticipantErrorWithTokenFamilyIsFatal() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).
		Return(&Session{SessionID: "sess-1", HandleID: "handle-abc", State: StateActive}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, mock.Anything, mock.Anything).
		Return(&SessionContext{SessionID: "sess-1"}, nil)
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(errors.New("db down"))

	sess, sc, err := svc.LoadCheckpoint(context.Background(), "handle-abc", "session", "app-456", "tfid-1")

	suite.Require().Error(err, "issuing a token family whose mapping cannot persist must fail the load")
	suite.Contains(err.Error(), "token family")
	suite.Nil(sess)
	suite.Nil(sc)
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_ParticipantErrorWithoutTokenFamilyIsNonFatal() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).
		Return(&Session{SessionID: "sess-1", HandleID: "handle-abc", State: StateActive}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, mock.Anything, mock.Anything).
		Return(&SessionContext{SessionID: "sess-1"}, nil)
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(errors.New("db down"))

	sess, sc, err := svc.LoadCheckpoint(context.Background(), "handle-abc", "session", "app-456", "")

	suite.Require().NoError(err, "with no token family there is nothing to revoke, so the load survives")
	suite.NotNil(sess)
	suite.NotNil(sc)
}

// --- Terminate ---

func (suite *ServiceTestSuite) TestTerminate_DeletesSessionAndPurges() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)

	got, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().NoError(err)
	suite.Require().NotNil(got)
	suite.Equal("sess-1", got.SessionID, "the terminated session is returned")
}

func (suite *ServiceTestSuite) TestTerminate_RevokesParticipantFamilies() {
	m := &serviceMocks{
		store: newSessionStoreMock(suite.T()),
		tx:    transactionmock.NewTransactionerMock(suite.T()),
	}
	revoker := NewCriteriaRevokerMock(suite.T())
	svc := &service{
		store:           m.store,
		resolver:        newResolver(m.store),
		transactioner:   m.tx,
		criteriaRevoker: revoker,
		timeouts:        DefaultTimeouts(),
		logger:          log.GetLogger(),
	}

	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	// Families are revoked before the deletes, one per participant.
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-1", TokenFamilyID: "tfid-a"},
		{SessionID: "sess-1", AppID: "app-2", TokenFamilyID: "tfid-b"},
	}, nil)
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-a").Return(nil)
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-b").Return(nil)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)

	got, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().NoError(err)
	suite.Require().NotNil(got)
	revoker.AssertExpectations(suite.T())
}

func (suite *ServiceTestSuite) TestTerminate_NoHandle() {
	svc, _ := suite.newService()

	got, err := svc.Terminate(context.Background(), "", "flow-1")

	suite.Require().NoError(err)
	suite.Nil(got)
}

func (suite *ServiceTestSuite) TestTerminate_MissingSessionIsNoOp() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(nil, nil)

	got, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().NoError(err, "terminating an absent session must be an idempotent no-op")
	suite.Nil(got)
}

func (suite *ServiceTestSuite) TestTerminate_DifferentFlowErrors() {
	svc, m := suite.newService()
	s := liveStoreSession()
	s.FlowID = "other-flow"
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(s, nil)

	got, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().Error(err, "a handle grouped under a different flow must be an error")
	suite.Contains(err.Error(), "belongs to flow")
	suite.Nil(got)
}

func (suite *ServiceTestSuite) TestTerminate_DeleteError() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().DeleteSession(mock.Anything, mock.Anything).Return(errors.New("store down"))

	_, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to terminate session")
}

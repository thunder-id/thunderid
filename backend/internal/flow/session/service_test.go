// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

// testOtherFlowID is a flow other than the one under test, used to assert that a session grouped
// under a different flow is never reused.
const testOtherFlowID = "other-flow"

// testHandle is the session handle the suite's fixtures are keyed by.
const testHandle = "handle-abc"

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
	s.FlowID = testOtherFlowID
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

// --- FindCheckpoint ---

// TestFindCheckpoint_Present also pins that the row itself is returned, not just its existence: the
// SSO-Check node forwards it to the Session node so the load path does not fetch it again.
func (suite *ServiceTestSuite) TestFindCheckpoint_Present() {
	svc, m := suite.newService()
	sc := &SessionContext{SessionID: "sess-1", CheckpointID: "session"}
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").Return(sc, nil)

	got, err := svc.FindCheckpoint(context.Background(), "sess-1", "session")
	suite.Require().NoError(err)
	suite.Same(sc, got)
}

func (suite *ServiceTestSuite) TestFindCheckpoint_Absent() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").Return(nil, nil)

	got, err := svc.FindCheckpoint(context.Background(), "sess-1", "session")
	suite.Require().NoError(err)
	suite.Nil(got)
}

func (suite *ServiceTestSuite) TestFindCheckpoint_ReadError() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByCheckpoint(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("store down"))

	_, err := svc.FindCheckpoint(context.Background(), "sess-1", "session")

	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to read SSO session checkpoint")
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
	// Saving a checkpoint into an existing session means the subject just authenticated again, so the
	// session's authentication time moves forward with it.
	var touchedAt, touchedIdle time.Time
	m.store.EXPECT().TouchAuthenticatedAt(mock.Anything, "sess-1", mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, at, idle time.Time) error {
			touchedAt, touchedIdle = at, idle
			return nil
		})

	in := saveInput()
	in.Checkpoint = "step_up"
	in.HandleHint = testHandle

	res, err := svc.SaveCheckpoint(context.Background(), in)
	suite.Require().NoError(err)

	suite.False(res.Created, "attaching to an existing session must not mint a new one")
	suite.Equal("handle-abc", res.Handle)
	suite.Equal("sess-1", savedCtx.SessionID)
	suite.Equal("step_up", savedCtx.CheckpointID)
	suite.False(touchedAt.IsZero(), "the re-authentication must refresh the session's auth time")
	suite.True(touchedIdle.After(touchedAt), "the idle deadline must slide past the new auth time")
}

// TestSaveCheckpoint_ReauthRefreshFailureStillSaves pins the degradation: a failure to refresh the
// authentication time must not cost the user their login, since the checkpoint itself is still valid.
func (suite *ServiceTestSuite) TestSaveCheckpoint_ReauthRefreshFailureStillSaves() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().TouchAuthenticatedAt(mock.Anything, "sess-1", mock.Anything, mock.Anything).
		Return(errors.New("db down"))

	in := saveInput()
	in.HandleHint = testHandle

	res, err := svc.SaveCheckpoint(context.Background(), in)
	suite.Require().NoError(err, "a refresh failure must not fail the login")
	suite.False(res.Skipped, "the checkpoint should still be saved")
	suite.Equal("handle-abc", res.Handle)
}

// TestSaveCheckpoint_NewSessionDoesNotTouch verifies the refresh is scoped to re-authentication: a
// freshly minted session already carries the right authentication time from establishSession.
func (suite *ServiceTestSuite) TestSaveCheckpoint_NewSessionDoesNotTouch() {
	svc, m := suite.newService()
	// Same establish sequence as TestSaveCheckpoint_Establishes: no session exists, so this call
	// mints one and re-reads its own row.
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
	suite.True(res.Created, "no existing session means this call minted one")
	m.store.AssertNotCalled(suite.T(), "TouchAuthenticatedAt",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestSaveCheckpoint_SubjectMismatchSkips() {
	svc, m := suite.newService()
	existing := liveStoreSession()
	existing.SubjectID = "someone-else"
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(existing, nil)

	in := saveInput()
	in.HandleHint = testHandle

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

// loadInput is the baseline load input: a handle to resolve and nothing forwarded, so the service
// reads both rows itself. Tests override the fields they are exercising.
func loadInput() LoadCheckpointInput {
	return LoadCheckpointInput{
		Handle: "handle-abc", Checkpoint: "session", AppID: "app-456", TokenFamilyID: "tfid-1",
	}
}

// noTFIDLoadInput is a load with no token family minted, where the participant write is best-effort.
func noTFIDLoadInput() LoadCheckpointInput {
	in := loadInput()
	in.TokenFamilyID = ""
	return in
}

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

	sess, sc, err := svc.LoadCheckpoint(context.Background(), loadInput())
	suite.Require().NoError(err)

	suite.Require().NotNil(sess)
	suite.Require().NotNil(sc)
	// Activity refresh: last-active refreshed, idle slid forward, absolute unchanged.
	suite.Require().NotNil(updated)
	suite.False(updated.LastActiveAt.IsZero())
	suite.True(updated.IdleExpiresAt.After(originalIdle))
	suite.Equal(originalAbsolute, updated.AbsoluteExpiresAt)
	// The joining application is recorded.
	suite.Equal("app-456", recorded.AppID)
}

// TestLoadCheckpoint_UsesForwardedReads is the guard for the reuse-path read reduction: given the rows
// the SSO-Check node forwarded, the load path must issue neither read. No GetByHandle or
// GetByCheckpoint expectation is registered, so the mock fails the test if either query comes back.
func (suite *ServiceTestSuite) TestLoadCheckpoint_UsesForwardedReads() {
	svc, m := suite.newService()
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(nil)

	in := loadInput()
	in.Session = liveStoreSession()
	in.Context = &SessionContext{SessionID: "sess-1", CheckpointID: "session"}

	sess, sc, err := svc.LoadCheckpoint(context.Background(), in)

	suite.Require().NoError(err)
	suite.Same(in.Session, sess)
	suite.Same(in.Context, sc)
}

// TestLoadCheckpoint_ReadsWhenForwardedSessionIsAnotherHandle rejects a forwarded session that does
// not belong to the handle being loaded, rather than trusting whatever arrived.
func (suite *ServiceTestSuite) TestLoadCheckpoint_ReadsWhenForwardedSessionIsAnotherHandle() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-xyz").Return(&Session{
		SessionID: "sess-2", HandleID: "handle-xyz", State: StateActive,
	}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-2", "session").
		Return(&SessionContext{SessionID: "sess-2", CheckpointID: "session"}, nil)
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(nil)

	in := loadInput()
	in.Handle = "handle-xyz"
	in.Session = liveStoreSession() // handle-abc, not the handle being loaded
	in.Context = &SessionContext{SessionID: "sess-1", CheckpointID: "session"}

	sess, sc, err := svc.LoadCheckpoint(context.Background(), in)

	suite.Require().NoError(err)
	suite.Equal("sess-2", sess.SessionID)
	suite.Equal("sess-2", sc.SessionID)
}

// TestLoadCheckpoint_ReadsWhenForwardedContextIsAnotherCheckpoint rejects a forwarded context that
// describes a different checkpoint, so a flow holding several checkpoints cannot cross-load them.
func (suite *ServiceTestSuite) TestLoadCheckpoint_ReadsWhenForwardedContextIsAnotherCheckpoint() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "step_up").
		Return(&SessionContext{SessionID: "sess-1", CheckpointID: "step_up"}, nil)
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(nil)

	in := loadInput()
	in.Checkpoint = "step_up"
	in.Session = liveStoreSession()
	in.Context = &SessionContext{SessionID: "sess-1", CheckpointID: "session"}

	_, sc, err := svc.LoadCheckpoint(context.Background(), in)

	suite.Require().NoError(err)
	suite.Equal("step_up", sc.CheckpointID)
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_NoHandle() {
	svc, _ := suite.newService()

	_, _, err := svc.LoadCheckpoint(context.Background(), LoadCheckpointInput{Checkpoint: "session", AppID: "app-456"})

	suite.Require().Error(err)
	suite.Contains(err.Error(), "no resolved session handle")
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_MissingSession() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).Return(nil, nil)

	_, _, err := svc.LoadCheckpoint(context.Background(), loadInput())

	suite.Require().Error(err)
	suite.Contains(err.Error(), "resolved session no longer exists")
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_MissingContext() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).
		Return(&Session{SessionID: "sess-1", HandleID: "handle-abc", State: StateActive}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	_, _, err := svc.LoadCheckpoint(context.Background(), loadInput())

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

	sess, sc, err := svc.LoadCheckpoint(context.Background(), loadInput())

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

	sess, sc, err := svc.LoadCheckpoint(context.Background(), noTFIDLoadInput())

	suite.Require().NoError(err, "with no token family there is nothing to revoke, so the load survives")
	suite.NotNil(sess)
	suite.NotNil(sc)
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_ThrottledRefreshSkipsUpdate() {
	svc, m := suite.newService()
	// The last persisted activity refresh is within the activity-refresh window, so the idle-slide
	// UPDATE is skipped. No Update expectation is set: the store mock fails the test if the throttled
	// path writes.
	recent := time.Now().UTC()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(&Session{
		SessionID: "sess-1", HandleID: "handle-abc", State: StateActive, LastActiveAt: recent,
	}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").
		Return(&SessionContext{SessionID: "sess-1"}, nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(nil)

	sess, sc, err := svc.LoadCheckpoint(context.Background(), noTFIDLoadInput())
	suite.Require().NoError(err)

	suite.Require().NotNil(sess)
	suite.Require().NotNil(sc)
	suite.Equal(recent, sess.LastActiveAt, "a throttled load must leave the persisted last-active untouched")
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_WritesAfterRefreshWindow() {
	svc, m := suite.newService()
	// The last persisted activity refresh is older than the activity-refresh window, so the activity refresh persists.
	stale := time.Now().UTC().Add(-2 * defaultActivityRefreshInterval)
	originalIdle := stale.Add(time.Minute)
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(&Session{
		SessionID: "sess-1", HandleID: "handle-abc", State: StateActive,
		LastActiveAt: stale, IdleExpiresAt: originalIdle,
	}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").
		Return(&SessionContext{SessionID: "sess-1"}, nil)
	var updated *Session
	m.store.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, s *Session) error { updated = s; return nil })
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(nil)

	_, _, err := svc.LoadCheckpoint(context.Background(), noTFIDLoadInput())
	suite.Require().NoError(err)

	suite.Require().NotNil(updated, "an activity refresh past the throttle window must persist")
	suite.True(updated.LastActiveAt.After(stale), "last-active slides forward")
	suite.True(updated.IdleExpiresAt.After(originalIdle), "idle deadline slides forward")
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

// Subject-wide termination deletes sessions without enumerating token families: the caller's subject
// criterion already covers them, so a per-application revocation here would only add redundant rows.
// The pairing is enforced at flow-creation time, not here.
func (suite *ServiceTestSuite) TestTerminateBySubject_DeletesAllSessionsInOneTransaction() {
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

	m.store.EXPECT().ListBySubject(mock.Anything, "user-1").Return([]Session{
		{SessionID: "sess-1", SubjectID: "user-1"},
		{SessionID: "sess-2", SubjectID: "user-1"},
	}, nil)
	// One transaction covers the revocation and every session's deletes.
	m.tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Once()
	for _, sessionID := range []string{"sess-1", "sess-2"} {
		m.store.EXPECT().DeleteSession(mock.Anything, sessionID).Return(nil)
		m.store.EXPECT().Delete(mock.Anything, sessionID).Return(nil)
		m.store.EXPECT().DeleteBySessionID(mock.Anything, sessionID).Return(nil)
	}

	err := svc.TerminateBySubject(context.Background(), "user-1")

	suite.Require().NoError(err)
	// No token-family sweep: two sessions must not mean two revocation writes.
	revoker.AssertNotCalled(suite.T(), "RevokeTokenFamily", mock.Anything, mock.Anything)
	m.store.AssertNotCalled(suite.T(), "ListBySessionID", mock.Anything, mock.Anything)
}

// A failed delete rolls the whole batch back rather than leaving some sessions terminated.
func (suite *ServiceTestSuite) TestTerminateBySubject_DeleteFailureRollsBackBatch() {
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

	m.store.EXPECT().ListBySubject(mock.Anything, "user-1").Return([]Session{
		{SessionID: "sess-1", SubjectID: "user-1"},
	}, nil)
	m.tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Once()
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(errors.New("db down"))

	err := svc.TerminateBySubject(context.Background(), "user-1")

	suite.Require().Error(err)
}

// No sessions means no transaction at all.
func (suite *ServiceTestSuite) TestTerminateBySubject_NoSessionsIsNoOp() {
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

	m.store.EXPECT().ListBySubject(mock.Anything, "user-1").Return(nil, nil)

	suite.Require().NoError(svc.TerminateBySubject(context.Background(), "user-1"))
	m.tx.AssertNotCalled(suite.T(), "Transact", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestTerminateBySubject_EmptySubjectIsNoOp() {
	svc, m := suite.newService()

	suite.Require().NoError(svc.TerminateBySubject(context.Background(), ""))
	m.store.AssertNotCalled(suite.T(), "ListBySubject", mock.Anything, mock.Anything)
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
	s.FlowID = testOtherFlowID
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

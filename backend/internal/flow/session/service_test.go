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

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventlistener"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/transactionmock"
)

// testOtherFlowID is a flow other than the one under test, used to assert that a session grouped
// under a different flow is never reused.
const testOtherFlowID = "other-flow"

// testHandle is the session handle the suite's fixtures are keyed by.
const testHandle = "handle-abc"

// testTokenFamilyID is the token family the baseline load input offers for its grant.
const testTokenFamilyID = "tfid-1"

// testNewTokenFamilyID is the token family a save offers for a grant joining an existing session.
const testNewTokenFamilyID = "tfid-new"

type ServiceTestSuite struct {
	suite.Suite
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

func (suite *ServiceTestSuite) SetupSuite() {
	security.InitDataPlanePermissions("")
}

func (suite *ServiceTestSuite) SetupTest() {
	suite.Require().NoError(config.InitializeServerRuntime(suite.T().TempDir(), &config.Config{}))
}

func (suite *ServiceTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// serviceMocks bundles the generated mocks a service test wires together. The one
// store mock backs every persistence operation (sessions, contexts, participants) since the service
// depends on the single sessionStore interface.
type serviceMocks struct {
	store       *sessionStoreMock
	tx          *transactionmock.TransactionerMock
	entities    *entitymock.EntityServiceInterfaceMock
	entityTypes *entitytypemock.EntityTypeServiceInterfaceMock
	apps        *applicationProviderMock
}

// newBus returns an event bus mock that accepts every event, for a test the audit event does not matter
// to. A test of what the service publishes swaps in newRecordingBus.
func (suite *ServiceTestSuite) newBus() *eventbusmock.EventBusInterfaceMock {
	bus := eventbusmock.NewEventBusInterfaceMock(suite.T())
	bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()
	return bus
}

func (suite *ServiceTestSuite) newService() (*service, *serviceMocks) {
	m := &serviceMocks{
		store:       newSessionStoreMock(suite.T()),
		tx:          transactionmock.NewTransactionerMock(suite.T()),
		entities:    entitymock.NewEntityServiceInterfaceMock(suite.T()),
		entityTypes: entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T()),
		apps:        newApplicationProviderMock(suite.T()),
	}
	svc := &service{
		store:            m.store,
		resolver:         newResolver(m.store, defaultTimeouts(suite.T())),
		transactioner:    m.tx,
		bus:              suite.newBus(),
		terminationTopic: eventlistener.NewTopic[TerminatedSession]("test"),
		timeouts:         defaultTimeouts(suite.T()),
		entities:         m.entities,
		entityTypes:      m.entityTypes,
		apps:             m.apps,
		logger:           log.GetLogger(),
	}
	return svc, m
}

// defaultTimeouts returns a timeouts source that reads the built-in defaults. Most cases never reach
// the timeouts, so it need not be read; a case about them uses readTimeouts.
func defaultTimeouts(t *testing.T) timeoutsSource {
	source := newTimeoutsSourceMock(t)
	source.EXPECT().current(mock.Anything).Return(newTimeouts(Config{}, deploymentLimits{})).Maybe()
	return source
}

// readTimeouts returns a timeouts source with the built-in defaults that the case must read once.
func readTimeouts(t *testing.T) timeoutsSource {
	source := newTimeoutsSourceMock(t)
	source.EXPECT().current(mock.Anything).Return(newTimeouts(Config{}, deploymentLimits{})).Once()
	return source
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

func (suite *ServiceTestSuite) TestAbsoluteTimeout() {
	svc, _ := suite.newService()
	svc.timeouts = readTimeouts(suite.T())

	suite.Equal(newTimeouts(Config{}, deploymentLimits{}).Absolute, svc.AbsoluteTimeout(context.Background()))
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

	got, err := svc.FindCheckpoint(context.Background(), &Session{SessionID: "sess-1"}, "session")
	suite.Require().NoError(err)
	suite.Same(sc, got)
}

func (suite *ServiceTestSuite) TestFindCheckpoint_Absent() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").Return(nil, nil)

	got, err := svc.FindCheckpoint(context.Background(), &Session{SessionID: "sess-1"}, "session")
	suite.Require().NoError(err)
	suite.Nil(got)
}

func (suite *ServiceTestSuite) TestFindCheckpoint_ReadError() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByCheckpoint(mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("store down"))

	_, err := svc.FindCheckpoint(context.Background(), &Session{SessionID: "sess-1"}, "session")

	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to read SSO session checkpoint")
}

// --- SaveCheckpoint ---

// boundaryContext returns a context inside a sign-in boundary of ouID, or one with no boundary when
// ouID is empty.
func boundaryContext(ouID string) context.Context {
	if ouID == "" {
		return context.Background()
	}
	return authnprovidercm.WithSignInBoundary(context.Background(),
		authnprovidercm.SignInBoundary{OUID: ouID, ApplicationID: "app-123"})
}

func saveInput() SaveCheckpointInput {
	return SaveCheckpointInput{
		SubjectID: "user-1", FlowID: "flow-1", FlowVersion: 3, ExecutionID: "exec-1",
		Checkpoint: "session", AuthUser: json.RawMessage(`{"entityReference":{"entityId":"user-1"}}`),
		RuntimeData: map[string]string{"email": "alice@example.com"}, AppID: "app-123",
		ClientInfo: ClientInfo{IP: "203.0.113.10", UserAgent: "Mozilla/5.0 (Macintosh) Chrome/140.0"},
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
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)

	res, err := svc.SaveCheckpoint(context.Background(), saveInput())
	suite.Require().NoError(err)

	suite.NotEmpty(res.Handle)
	suite.Require().NotNil(created)
	suite.Equal(created.AbsoluteExpiresAt, res.AbsoluteExpiresAt, "the result says when the new session ends")
	suite.Equal(TypeSSO, created.Type)
	suite.Equal("user-1", created.SubjectID)
	suite.Equal("flow-1", created.FlowID)
	suite.Equal(3, created.FlowVersion)
	suite.Equal(StateActive, created.State)
	suite.True(created.IdleExpiresAt.After(created.CreatedAt))
	suite.True(created.AbsoluteExpiresAt.After(created.IdleExpiresAt))
	suite.Equal("Mozilla/5.0 (Macintosh) Chrome/140.0", created.Properties.UserAgent)
	suite.Equal("203.0.113.10", created.Properties.CreationIP, "a new session records the IP it was created from")
	suite.Equal("203.0.113.10", created.Properties.LastActiveIP, "a new session starts last active from that IP")
}

func (suite *ServiceTestSuite) TestSaveCheckpoint_AttachesToExisting() {
	svc, m := suite.newService()
	existing := liveStoreSession()
	existing.Properties = SessionProperties{
		UserAgent: "Mozilla/5.0 (Macintosh) Chrome/140.0", CreationIP: "192.0.2.1", LastActiveIP: "192.0.2.1",
	}
	existing.AbsoluteExpiresAt = time.Now().Add(2 * time.Hour).UTC()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(existing, nil)
	runTx(m)
	var savedCtx SessionContext
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, c SessionContext) error { savedCtx = c; return nil })
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)
	// Saving a checkpoint into an existing session means the subject just authenticated again, so the
	// session's authentication time moves forward with it.
	var touchedAt, touchedIdle time.Time
	var touched SessionProperties
	m.store.EXPECT().TouchAuthenticatedAt(mock.Anything, "sess-1", mock.Anything, mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, at, idle time.Time, properties SessionProperties) error {
			touchedAt, touchedIdle, touched = at, idle, properties
			return nil
		})

	in := saveInput()
	in.Checkpoint = "step_up"
	in.HandleHint = testHandle

	res, err := svc.SaveCheckpoint(context.Background(), in)
	suite.Require().NoError(err)

	m.store.AssertNotCalled(suite.T(), "Create", mock.Anything, mock.Anything)
	suite.Equal("handle-abc", res.Handle)
	suite.Equal(existing.AbsoluteExpiresAt, res.AbsoluteExpiresAt,
		"a joined session keeps the end it was created with")
	suite.Equal("sess-1", savedCtx.SessionID)
	suite.Equal("step_up", savedCtx.CheckpointID)
	suite.False(touchedAt.IsZero(), "the re-authentication must refresh the session's auth time")
	suite.True(touchedIdle.After(touchedAt), "the idle deadline must slide past the new auth time")
	suite.Equal("203.0.113.10", touched.LastActiveIP, "the re-authentication records the client's IP as last active")
	suite.Equal("Mozilla/5.0 (Macintosh) Chrome/140.0", touched.UserAgent,
		"rewriting the properties must keep the recorded User-Agent")
	suite.Equal("192.0.2.1", touched.CreationIP, "rewriting the properties must keep the creation IP")
}

// The joining participant is recorded under the sign-in's organization unit, with its token family.
func (suite *ServiceTestSuite) TestSaveCheckpoint_RecordsTheParticipantUnderItsOU() {
	for name, ouID := range map[string]string{"bounded sign-in": "ou-x", "sign-in with no OU": ""} {
		suite.Run(name, func() {
			svc, m := suite.newService()
			svc.ouEligibility = suite.subjectMaySignIn(true)
			m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
			m.store.EXPECT().TouchAuthenticatedAt(mock.Anything, "sess-1", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			runTx(m)
			m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(nil)
			var recorded Participant
			m.store.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
				func(_ context.Context, p Participant) (string, error) { recorded = p; return p.TokenFamilyID, nil })
			in := saveInput()
			in.HandleHint = testHandle
			in.TokenFamilyID = "tfid-1"

			_, err := svc.SaveCheckpoint(boundaryContext(ouID), in)

			suite.Require().NoError(err)
			suite.Equal(Participant{SessionID: "sess-1", AppID: "app-123", OUID: ouID, TokenFamilyID: "tfid-1",
				FirstJoinedAt: recorded.FirstJoinedAt, LastActiveAt: recorded.LastActiveAt}, recorded)
		})
	}
}

// subjectMaySignIn returns an eligibility check that answers eligible for any organization unit it is asked about.
func (suite *ServiceTestSuite) subjectMaySignIn(eligible bool) *OUEligibilityMock {
	e := NewOUEligibilityMock(suite.T())
	e.EXPECT().IsEligibleOU(mock.Anything, mock.Anything).Return(eligible, nil).Maybe()
	return e
}

// A sign-in its assertion will refuse is recorded under no organization unit, keeping its token family, so it never
// becomes the session's last-used one.
func (suite *ServiceTestSuite) TestSaveCheckpoint_RecordsARefusedSignInUnderNoOU() {
	svc, m := suite.newService()
	svc.ouEligibility = suite.subjectMaySignIn(false)
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	m.store.EXPECT().TouchAuthenticatedAt(mock.Anything, "sess-1", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	runTx(m)
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(nil)
	var recorded Participant
	m.store.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p Participant) (string, error) { recorded = p; return p.TokenFamilyID, nil })
	in := saveInput()
	in.HandleHint = testHandle
	in.TokenFamilyID = "tfid-1"

	_, err := svc.SaveCheckpoint(boundaryContext("ou-x"), in)

	suite.Require().NoError(err)
	suite.Empty(recorded.OUID)
	suite.Equal("tfid-1", recorded.TokenFamilyID)
}

// A sign-in bounded to an organization unit its subject cannot sign in to does not reuse the session, so nothing is
// recorded for it.
func (suite *ServiceTestSuite) TestLoadCheckpoint_RefusedInAnOUItsSubjectCannotSignInTo() {
	svc, m := suite.newService()
	svc.ouEligibility = suite.subjectMaySignIn(false)
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").
		Return(&SessionContext{SessionID: "sess-1"}, nil)
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil).Maybe()

	_, err := svc.LoadCheckpoint(boundaryContext("ou-x"), loadInput())

	suite.Require().Error(err)
	m.store.AssertNotCalled(suite.T(), "Record", mock.Anything, mock.Anything)
}

// The snapshot leaves out the flow's organization unit and its fixed mark, which are decided per sign-in, and the
// caller's runtime data is not modified.
func (suite *ServiceTestSuite) TestSaveCheckpoint_SnapshotDropsTheOrganizationUnit() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	m.store.EXPECT().TouchAuthenticatedAt(mock.Anything, "sess-1", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	runTx(m)
	var savedCtx SessionContext
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, c SessionContext) error { savedCtx = c; return nil })
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)
	in := saveInput()
	in.HandleHint = testHandle
	in.RuntimeData = map[string]string{
		common.RuntimeKeyOUID: "ou-old", common.RuntimeKeyOUFixed: "true", "email": "alice@example.com"}

	_, err := svc.SaveCheckpoint(context.Background(), in)

	suite.Require().NoError(err)
	suite.Equal(map[string]string{"email": "alice@example.com"}, savedCtx.RuntimeData)
	suite.Equal("ou-old", in.RuntimeData[common.RuntimeKeyOUID])
	suite.Equal("true", in.RuntimeData[common.RuntimeKeyOUFixed])
}

// TestSaveCheckpoint_ReauthRefreshFailureStillSaves pins the degradation: a failure to refresh the
// authentication time must not cost the user their login, since the checkpoint itself is still valid.
func (suite *ServiceTestSuite) TestSaveCheckpoint_ReauthRefreshFailureStillSaves() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)
	m.store.EXPECT().TouchAuthenticatedAt(mock.Anything, "sess-1", mock.Anything, mock.Anything, mock.Anything).
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
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)

	res, err := svc.SaveCheckpoint(context.Background(), saveInput())
	suite.Require().NoError(err)
	suite.Require().NotNil(created, "no existing session means this call minted one")
	suite.Equal(created.HandleID, res.Handle)
	m.store.AssertNotCalled(suite.T(), "TouchAuthenticatedAt",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
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
	runTx(m)
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

// saveIntoExistingSession saves a grant offering testNewTokenFamilyID into sess-1, whose participant holds held.
func (suite *ServiceTestSuite) saveIntoExistingSession(svc *service, m *serviceMocks, held string) (
	SaveCheckpointResult, error) {
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	m.store.EXPECT().TouchAuthenticatedAt(mock.Anything, "sess-1", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	runTx(m)
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.MatchedBy(func(p Participant) bool {
		return p.TokenFamilyID == testNewTokenFamilyID
	})).Return(held, nil)
	in := saveInput()
	in.HandleHint = testHandle
	in.TokenFamilyID = testNewTokenFamilyID
	return svc.SaveCheckpoint(context.Background(), in)
}

// A save's grant carries the held family unless it is revoked; a failed check or replacement keeps it.
func (suite *ServiceTestSuite) TestSaveCheckpoint_ReturnsTheParticipantsTokenFamily() {
	cases := []struct {
		name       string
		held       string
		revoked    bool
		checkErr   error
		replaceErr error
		want       string
	}{
		{name: "first join", held: testNewTokenFamilyID, want: testNewTokenFamilyID},
		{name: "rejoin keeps a live family", held: "tfid-existing", want: "tfid-existing"},
		{name: "rejoin replaces a revoked family", held: "tfid-existing", revoked: true, want: testNewTokenFamilyID},
		{name: "an unreadable deny list keeps the family", held: "tfid-existing", checkErr: errors.New("db down"),
			want: "tfid-existing"},
		{name: "a failed replacement keeps the revoked family", held: "tfid-existing", revoked: true,
			replaceErr: errors.New("db down"), want: "tfid-existing"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, m := suite.newService()
			revoker := NewCriteriaRevokerMock(suite.T())
			svc.criteriaRevoker = revoker
			if tc.held != testNewTokenFamilyID {
				revoker.EXPECT().IsTokenFamilyRevoked(mock.Anything, tc.held).Return(tc.revoked, tc.checkErr)
			}
			if tc.revoked {
				m.store.EXPECT().ReplaceTokenFamily(mock.Anything, mock.Anything, tc.held).
					Return(testNewTokenFamilyID, tc.replaceErr)
			}

			res, err := suite.saveIntoExistingSession(svc, m, tc.held)

			suite.Require().NoError(err)
			suite.Equal(tc.want, res.TokenFamilyID)
		})
	}
}

// txContextKey marks the context the transaction mock hands its callback.
type txContextKey struct{}

// The family is settled after the commit, since a failed read would abort a PostgreSQL transaction.
func (suite *ServiceTestSuite) TestSaveCheckpoint_SettlesTheTokenFamilyAfterTheCommit() {
	svc, m := suite.newService()
	committed := false
	m.tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			err := fn(context.WithValue(ctx, txContextKey{}, true))
			committed = err == nil
			return err
		})
	insideTx := mock.MatchedBy(func(ctx context.Context) bool { return ctx.Value(txContextKey{}) != nil })
	afterCommit := mock.MatchedBy(func(ctx context.Context) bool {
		return committed && ctx.Value(txContextKey{}) == nil
	})
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	m.store.EXPECT().TouchAuthenticatedAt(mock.Anything, "sess-1", mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	m.store.EXPECT().CreateContext(insideTx, mock.Anything).Return(nil)
	m.store.EXPECT().Record(insideTx, mock.Anything).Return("tfid-existing", nil)
	revoker := NewCriteriaRevokerMock(suite.T())
	revoker.EXPECT().IsTokenFamilyRevoked(afterCommit, "tfid-existing").Return(true, nil)
	svc.criteriaRevoker = revoker
	m.store.EXPECT().ReplaceTokenFamily(afterCommit, mock.Anything, "tfid-existing").
		Return(testNewTokenFamilyID, nil)
	in := saveInput()
	in.HandleHint = testHandle
	in.TokenFamilyID = testNewTokenFamilyID

	res, err := svc.SaveCheckpoint(context.Background(), in)

	suite.Require().NoError(err)
	suite.Equal(testNewTokenFamilyID, res.TokenFamilyID)
}

// --- LoadCheckpoint ---

// loadInput is the baseline load input: a handle to resolve and nothing forwarded, so the service
// reads both rows itself. Tests override the fields they are exercising.
func loadInput() LoadCheckpointInput {
	return LoadCheckpointInput{
		Handle: "handle-abc", Checkpoint: "session", AppID: "app-456", TokenFamilyID: testTokenFamilyID,
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
		func(_ context.Context, p Participant) (string, error) { recorded = p; return p.TokenFamilyID, nil })

	loaded, err := svc.LoadCheckpoint(context.Background(), loadInput())
	sess, sc := loaded.Session, loaded.Context
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

// An application joining one session under two organization units is recorded once under each, each
// with the token family of its own grant.
func (suite *ServiceTestSuite) TestLoadCheckpoint_RecordsEachOUOfAnApplication() {
	svc, m := suite.newService()
	svc.ouEligibility = suite.subjectMaySignIn(true)
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").
		Return(&SessionContext{SessionID: "sess-1"}, nil)
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil).Maybe()
	var recorded []Participant
	m.store.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p Participant) (string, error) {
			recorded = append(recorded, p)
			return p.TokenFamilyID, nil
		})

	for ouID, tfid := range map[string]string{"ou-x": "tfid-x", "ou-y": "tfid-y"} {
		in := loadInput()
		in.TokenFamilyID = tfid
		_, err := svc.LoadCheckpoint(boundaryContext(ouID), in)
		suite.Require().NoError(err)
	}

	suite.Require().Len(recorded, 2)
	families := map[string]string{}
	for _, p := range recorded {
		suite.Equal("app-456", p.AppID)
		families[p.OUID] = p.TokenFamilyID
	}
	suite.Equal(map[string]string{"ou-x": "tfid-x", "ou-y": "tfid-y"}, families)
}

// TestLoadCheckpoint_UsesForwardedReads is the guard for the reuse-path read reduction: given the rows
// the SSO-Check node forwarded, the load path must issue neither read. No GetByHandle or
// GetByCheckpoint expectation is registered, so the mock fails the test if either query comes back.
func (suite *ServiceTestSuite) TestLoadCheckpoint_UsesForwardedReads() {
	svc, m := suite.newService()
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(testTokenFamilyID, nil)

	in := loadInput()
	in.Session = liveStoreSession()
	in.Context = &SessionContext{SessionID: "sess-1", CheckpointID: "session"}

	loaded, err := svc.LoadCheckpoint(context.Background(), in)
	sess, sc := loaded.Session, loaded.Context

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
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(testTokenFamilyID, nil)

	in := loadInput()
	in.Handle = "handle-xyz"
	in.Session = liveStoreSession() // handle-abc, not the handle being loaded
	in.Context = &SessionContext{SessionID: "sess-1", CheckpointID: "session"}

	loaded, err := svc.LoadCheckpoint(context.Background(), in)
	sess, sc := loaded.Session, loaded.Context

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
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return(testTokenFamilyID, nil)

	in := loadInput()
	in.Checkpoint = "step_up"
	in.Session = liveStoreSession()
	in.Context = &SessionContext{SessionID: "sess-1", CheckpointID: "session"}

	loaded, err := svc.LoadCheckpoint(context.Background(), in)
	sc := loaded.Context

	suite.Require().NoError(err)
	suite.Equal("step_up", sc.CheckpointID)
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_NoHandle() {
	svc, _ := suite.newService()

	_, err := svc.LoadCheckpoint(context.Background(),
		LoadCheckpointInput{Checkpoint: "session", AppID: "app-456"})

	suite.Require().Error(err)
	suite.Contains(err.Error(), "no resolved session handle")
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_MissingSession() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).Return(nil, nil)

	_, err := svc.LoadCheckpoint(context.Background(), loadInput())

	suite.Require().Error(err)
	suite.Contains(err.Error(), "resolved session no longer exists")
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_MissingContext() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, mock.Anything).
		Return(&Session{SessionID: "sess-1", HandleID: "handle-abc", State: StateActive}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	_, err := svc.LoadCheckpoint(context.Background(), loadInput())

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
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", errors.New("db down"))

	loaded, err := svc.LoadCheckpoint(context.Background(), loadInput())
	sess, sc := loaded.Session, loaded.Context

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
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", errors.New("db down"))

	loaded, err := svc.LoadCheckpoint(context.Background(), noTFIDLoadInput())
	sess, sc := loaded.Session, loaded.Context

	suite.Require().NoError(err, "with no token family there is nothing to revoke, so the load survives")
	suite.NotNil(sess)
	suite.NotNil(sc)
}

// loadWithParticipant loads sess-1 for a grant offering testTokenFamilyID, whose participant holds held.
func (suite *ServiceTestSuite) loadWithParticipant(svc *service, m *serviceMocks, held string) (string, error) {
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil).Maybe()
	m.store.EXPECT().Record(mock.Anything, mock.MatchedBy(func(p Participant) bool {
		return p.TokenFamilyID == testTokenFamilyID
	})).Return(held, nil)
	in := loadInput()
	in.Session = liveStoreSession()
	in.Context = &SessionContext{SessionID: "sess-1", CheckpointID: "session"}
	loaded, err := svc.LoadCheckpoint(context.Background(), in)
	tokenFamilyID := loaded.TokenFamilyID
	return tokenFamilyID, err
}

// A load's grant carries the held family unless it is revoked; a failed check or replacement keeps it.
func (suite *ServiceTestSuite) TestLoadCheckpoint_ReturnsTheParticipantsTokenFamily() {
	cases := []struct {
		name       string
		held       string
		noRevoker  bool
		revoked    bool
		checkErr   error
		replaced   string
		replaceErr error
		want       string
	}{
		{name: "first join", held: testTokenFamilyID, want: testTokenFamilyID},
		{name: "rejoin keeps a live family", held: "tfid-existing", want: "tfid-existing"},
		{name: "rejoin replaces a revoked family", held: "tfid-existing", revoked: true, replaced: testTokenFamilyID,
			want: testTokenFamilyID},
		{name: "a concurrent replacement wins", held: "tfid-existing", revoked: true, replaced: "tfid-other",
			want: "tfid-other"},
		{name: "an unreadable deny list keeps the family", held: "tfid-existing", checkErr: errors.New("db down"),
			want: "tfid-existing"},
		{name: "a failed replacement keeps the revoked family", held: "tfid-existing", revoked: true,
			replaceErr: errors.New("participant gone"), want: "tfid-existing"},
		{name: "no revoker keeps the family", held: "tfid-existing", noRevoker: true, want: "tfid-existing"},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, m := suite.newService()
			if !tc.noRevoker {
				revoker := NewCriteriaRevokerMock(suite.T())
				svc.criteriaRevoker = revoker
				if tc.held != testTokenFamilyID {
					revoker.EXPECT().IsTokenFamilyRevoked(mock.Anything, tc.held).Return(tc.revoked, tc.checkErr)
				}
			}
			if tc.revoked {
				m.store.EXPECT().ReplaceTokenFamily(mock.Anything, mock.MatchedBy(func(p Participant) bool {
					return p.SessionID == "sess-1" && p.AppID == "app-456" && p.TokenFamilyID == testTokenFamilyID
				}), tc.held).Return(tc.replaced, tc.replaceErr)
			}

			tokenFamilyID, err := suite.loadWithParticipant(svc, m, tc.held)

			suite.Require().NoError(err)
			suite.Equal(tc.want, tokenFamilyID)
		})
	}
}

// Without an application id nothing is recorded, and the grant keeps the family it was given.
func (suite *ServiceTestSuite) TestLoadCheckpoint_WithoutAnApplicationKeepsTheGivenFamily() {
	svc, m := suite.newService()
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil).Maybe()
	in := loadInput()
	in.AppID = ""
	in.Session = liveStoreSession()
	in.Context = &SessionContext{SessionID: "sess-1", CheckpointID: "session"}

	loaded, err := svc.LoadCheckpoint(context.Background(), in)
	tokenFamilyID := loaded.TokenFamilyID

	suite.Require().NoError(err)
	suite.Equal(testTokenFamilyID, tokenFamilyID)
	m.store.AssertNotCalled(suite.T(), "Record", mock.Anything, mock.Anything)
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
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)

	loaded, err := svc.LoadCheckpoint(context.Background(), noTFIDLoadInput())
	sess, sc := loaded.Session, loaded.Context
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
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)

	_, err := svc.LoadCheckpoint(context.Background(), noTFIDLoadInput())
	suite.Require().NoError(err)

	suite.Require().NotNil(updated, "an activity refresh past the throttle window must persist")
	suite.True(updated.LastActiveAt.After(stale), "last-active slides forward")
	suite.True(updated.IdleExpiresAt.After(originalIdle), "idle deadline slides forward")
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_RefreshRecordsClientIP() {
	svc, m := suite.newService()
	stale := time.Now().UTC().Add(-2 * defaultActivityRefreshInterval)
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(&Session{
		SessionID: "sess-1", HandleID: "handle-abc", State: StateActive,
		LastActiveAt: stale,
		Properties: SessionProperties{
			UserAgent: "Mozilla/5.0 (Macintosh) Chrome/140.0", CreationIP: "203.0.113.10",
			LastActiveIP: "203.0.113.10",
		},
	}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").
		Return(&SessionContext{SessionID: "sess-1"}, nil)
	var updated *Session
	m.store.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, s *Session) error { updated = s; return nil })
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)

	in := noTFIDLoadInput()
	in.ClientInfo = ClientInfo{IP: "198.51.100.7", UserAgent: "Mozilla/5.0 (iPhone) Safari/17.0"}
	_, err := svc.LoadCheckpoint(context.Background(), in)
	suite.Require().NoError(err)

	suite.Require().NotNil(updated)
	suite.Equal("198.51.100.7", updated.Properties.LastActiveIP,
		"the refresh records where the session was reused from")
	suite.Equal("Mozilla/5.0 (Macintosh) Chrome/140.0", updated.Properties.UserAgent, "the User-Agent is write-once")
	suite.Equal("203.0.113.10", updated.Properties.CreationIP, "the creation IP is write-once")
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_RefreshKeepsKnownIPWhenClientHasNone() {
	svc, m := suite.newService()
	stale := time.Now().UTC().Add(-2 * defaultActivityRefreshInterval)
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(&Session{
		SessionID: "sess-1", HandleID: "handle-abc", State: StateActive,
		LastActiveAt: stale, Properties: SessionProperties{LastActiveIP: "203.0.113.10"},
	}, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").
		Return(&SessionContext{SessionID: "sess-1"}, nil)
	var updated *Session
	m.store.EXPECT().Update(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, s *Session) error { updated = s; return nil })
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)

	_, err := svc.LoadCheckpoint(context.Background(), noTFIDLoadInput())
	suite.Require().NoError(err)

	suite.Require().NotNil(updated)
	suite.Equal("203.0.113.10", updated.Properties.LastActiveIP,
		"a request without client info must not erase a known IP")
}

// --- RecordActivity ---

func (suite *ServiceTestSuite) TestRecordActivity_SlidesIdleUnderThrottle() {
	svc, m := suite.newService()
	svc.timeouts = readTimeouts(suite.T())
	var slide idleSlide
	m.store.EXPECT().SlideIdleDeadline(mock.Anything, "sess-1", mock.Anything).
		RunAndReturn(func(_ context.Context, _ string, s idleSlide) error {
			slide = s
			return nil
		})

	suite.Require().NoError(svc.RecordActivity(context.Background(), "sess-1"))

	now := slide.now
	suite.Equal(now.Add(newTimeouts(Config{}, deploymentLimits{}).Idle), slide.idleExpiresAt,
		"idle deadline moves a full idle window past now")
	suite.Equal(now.Add(-newTimeouts(Config{}, deploymentLimits{}).ActivityRefresh), slide.refreshedBefore,
		"writes are throttled by the refresh window")
	suite.Equal(now.Add(-newTimeouts(Config{}, deploymentLimits{}).Idle), slide.activeAfter,
		"a session idle past the timeout in effect is not slid")
	suite.Equal(now.Add(-newTimeouts(Config{}, deploymentLimits{}).Absolute), slide.createdAfter,
		"a session older than the absolute timeout in effect is not slid")
}

func (suite *ServiceTestSuite) TestRecordActivity_EmptySessionIDIsNoop() {
	svc, _ := suite.newService()

	suite.NoError(svc.RecordActivity(context.Background(), ""))
}

func (suite *ServiceTestSuite) TestRecordActivity_StoreError() {
	svc, m := suite.newService()
	m.store.EXPECT().SlideIdleDeadline(mock.Anything, "sess-1", mock.Anything).
		Return(errors.New("db down"))

	suite.Error(svc.RecordActivity(context.Background(), "sess-1"))
}

// --- Terminate ---

func (suite *ServiceTestSuite) TestTerminate_DeletesSessionAndPurges() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
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
		resolver:        newResolver(m.store, defaultTimeouts(suite.T())),
		transactioner:   m.tx,
		criteriaRevoker: revoker,
		bus:             suite.newBus(),
		timeouts:        defaultTimeouts(suite.T()),
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
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)

	got, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().NoError(err)
	suite.Require().NotNil(got)
	revoker.AssertExpectations(suite.T())
}

// Sign-out revokes the family of every organization unit an application signed in under.
func (suite *ServiceTestSuite) TestTerminate_RevokesTheFamilyOfEveryOURow() {
	svc, m, revoker := suite.newServiceWithRevoker()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-1", OUID: "ou-x", TokenFamilyID: "tfid-x"},
		{SessionID: "sess-1", AppID: "app-1", OUID: "ou-y", TokenFamilyID: "tfid-y"},
	}, nil)
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-x").Return(nil).Once()
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-y").Return(nil).Once()
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)

	_, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().NoError(err)
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
		resolver:        newResolver(m.store, defaultTimeouts(suite.T())),
		transactioner:   m.tx,
		criteriaRevoker: revoker,
		bus:             suite.newBus(),
		timeouts:        defaultTimeouts(suite.T()),
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
		m.store.EXPECT().DeleteSession(mock.Anything, sessionID).Return(true, nil)
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
		resolver:      newResolver(m.store, defaultTimeouts(suite.T())),
		transactioner: m.tx,
		bus:           suite.newBus(),
		timeouts:      defaultTimeouts(suite.T()),
		logger:        log.GetLogger(),
	}

	m.store.EXPECT().ListBySubject(mock.Anything, "user-1").Return([]Session{
		{SessionID: "sess-1", SubjectID: "user-1"},
	}, nil)
	m.tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }).Once()
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(false, errors.New("db down"))

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
		resolver:        newResolver(m.store, defaultTimeouts(suite.T())),
		transactioner:   m.tx,
		criteriaRevoker: revoker,
		bus:             suite.newBus(),
		timeouts:        defaultTimeouts(suite.T()),
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
	m.store.EXPECT().DeleteSession(mock.Anything, mock.Anything).Return(false, errors.New("store down"))

	_, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to terminate session")
}

func (suite *ServiceTestSuite) TestLastUsedOUID_LatestRowWithAnOUWins() {
	svc, m := suite.newService()
	earlier := time.Unix(1700000000, 0).UTC()
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-1", OUID: "ou-old", LastActiveAt: earlier},
		{SessionID: "sess-1", AppID: "app-2", OUID: "ou-new", LastActiveAt: earlier.Add(2 * time.Minute)},
		{SessionID: "sess-1", AppID: "app-1", OUID: "ou-mid", LastActiveAt: earlier.Add(time.Minute)},
		{SessionID: "sess-1", AppID: "app-3", LastActiveAt: earlier.Add(3 * time.Minute)},
	}, nil)

	ouID, err := svc.lastUsedOUID(context.Background(), "sess-1")

	suite.Require().NoError(err)
	suite.Equal("ou-new", ouID)
}

// Rows with an equal last-active time keep the first one joined.
func (suite *ServiceTestSuite) TestLastUsedOUID_TieKeepsTheFirstJoined() {
	svc, m := suite.newService()
	at := time.Unix(1700000000, 0).UTC()
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-1", OUID: "ou-first", LastActiveAt: at},
		{SessionID: "sess-1", AppID: "app-2", OUID: "ou-second", LastActiveAt: at},
	}, nil)

	ouID, err := svc.lastUsedOUID(context.Background(), "sess-1")

	suite.Require().NoError(err)
	suite.Equal("ou-first", ouID)
}

func (suite *ServiceTestSuite) TestLastUsedOUID_EmptyWithoutARowWithAnOU() {
	for name, participants := range map[string][]Participant{
		"only rows without one": {{SessionID: "sess-1", AppID: "app-1", LastActiveAt: time.Now().UTC()}},
		"no rows":               nil,
	} {
		suite.Run(name, func() {
			svc, m := suite.newService()
			m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(participants, nil)

			ouID, err := svc.lastUsedOUID(context.Background(), "sess-1")

			suite.Require().NoError(err)
			suite.Empty(ouID)
		})
	}
}

func (suite *ServiceTestSuite) TestLastUsedOUID_ListError() {
	svc, m := suite.newService()
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, errors.New("db down"))

	_, err := svc.lastUsedOUID(context.Background(), "sess-1")

	suite.Require().Error(err)
	suite.Contains(err.Error(), "db down")
}

// newServiceWithRevoker builds a service wired to a criteria revoker, for the paths that revoke token
// families as part of a session write.
// reuseCtx returns a context carrying the sign-in boundary of app-1 that has decided ouID.
func reuseCtx(ouID string) context.Context {
	return authnprovidercm.WithSignInBoundary(context.Background(),
		authnprovidercm.SignInBoundary{OUID: ouID, ApplicationID: "app-1"})
}

// reusedSession is a session user-2 signed in with.
func reusedSession() *Session {
	return &Session{SessionID: "sess-1", SubjectID: "user-2"}
}

// withLastUsedOU makes the session's only participant one that signed in to ou-last.
func withLastUsedOU(m *serviceMocks) {
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-1", OUID: "ou-last", LastActiveAt: time.Now().UTC()},
	}, nil).Once()
}

// eligibilityAsked returns an eligibility check asked once whether user-2 may land in ou-last, inside the
// boundary with no organization unit, answering eligible and svcErr.
func (suite *ServiceTestSuite) eligibilityAsked(eligible bool, svcErr *tidcommon.ServiceError) *OUEligibilityMock {
	e := NewOUEligibilityMock(suite.T())
	e.EXPECT().IsEligibleOU(mock.MatchedBy(func(c context.Context) bool {
		b, ok := authnprovidercm.SignInBoundaryFrom(c)
		return ok && b == authnprovidercm.SignInBoundary{ApplicationID: "app-1", EntityID: "user-2"}
	}), "ou-last").Return(eligible, svcErr).Once()
	return e
}

func (suite *ServiceTestSuite) TestReuseOU_FollowsTheLastUsedOUWhileTheSubjectIsEligible() {
	for name, eligible := range map[string]bool{"eligible": true, "not eligible": false} {
		suite.Run(name, func() {
			svc, m := suite.newService()
			withLastUsedOU(m)
			svc.ouEligibility = suite.eligibilityAsked(eligible, nil)

			ouID, reusable, err := svc.reuseOU(reuseCtx(""), reusedSession())

			suite.Require().NoError(err)
			suite.Equal(eligible, reusable)
			if eligible {
				suite.Equal("ou-last", ouID)
				return
			}
			suite.Empty(ouID)
		})
	}
}

func (suite *ServiceTestSuite) TestReuseOU_AuthenticatesWithoutALastUsedOU() {
	svc, m := suite.newService()
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, nil).Once()

	ouID, reusable, err := svc.reuseOU(reuseCtx(""), reusedSession())

	suite.Require().NoError(err)
	suite.False(reusable)
	suite.Empty(ouID)
}

// An unbounded sign-in has nothing to decide and reuses the session.
func (suite *ServiceTestSuite) TestReuseOU_UnboundedReusesTheSession() {
	svc, _ := suite.newService()

	ouID, reusable, err := svc.reuseOU(context.Background(), reusedSession())

	suite.Require().NoError(err)
	suite.True(reusable)
	suite.Empty(ouID)
}

// fixedOUEligibilityAsked returns an eligibility check asked once whether user-2 may land in ou-fixed, the
// organization unit the boundary fixed, answering eligible and svcErr.
func (suite *ServiceTestSuite) fixedOUEligibilityAsked(eligible bool,
	svcErr *tidcommon.ServiceError) *OUEligibilityMock {
	e := NewOUEligibilityMock(suite.T())
	e.EXPECT().IsEligibleOU(mock.MatchedBy(func(c context.Context) bool {
		b, ok := authnprovidercm.SignInBoundaryFrom(c)
		return ok && b == authnprovidercm.SignInBoundary{OUID: "ou-fixed", ApplicationID: "app-1", EntityID: "user-2"}
	}), "ou-fixed").Return(eligible, svcErr).Once()
	return e
}

// An organization unit already decided is never overridden: the session is reused there only while its subject
// may sign in to it, and the sign-in authenticates otherwise.
func (suite *ServiceTestSuite) TestReuseOU_FixedOUReusesOnlyForAnEligibleSubject() {
	for name, eligible := range map[string]bool{"eligible": true, "not eligible": false} {
		suite.Run(name, func() {
			svc, _ := suite.newService()
			svc.ouEligibility = suite.fixedOUEligibilityAsked(eligible, nil)

			ouID, reusable, err := svc.reuseOU(reuseCtx("ou-fixed"), reusedSession())

			suite.Require().NoError(err)
			suite.Equal(eligible, reusable)
			suite.Empty(ouID)
		})
	}
}

func (suite *ServiceTestSuite) TestReuseOU_Failures() {
	suite.Run("last-used organization unit", func() {
		svc, m := suite.newService()
		m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, errors.New("db down")).Once()

		_, reusable, err := svc.reuseOU(reuseCtx(""), reusedSession())

		suite.Require().Error(err)
		suite.False(reusable)
	})
	suite.Run("eligibility", func() {
		svc, m := suite.newService()
		withLastUsedOU(m)
		svc.ouEligibility = suite.eligibilityAsked(false, &tidcommon.InternalServerError)

		_, reusable, err := svc.reuseOU(reuseCtx(""), reusedSession())

		suite.Require().Error(err)
		suite.False(reusable)
		suite.Contains(err.Error(), "last-used organization unit")
	})
	suite.Run("fixed organization unit", func() {
		svc, _ := suite.newService()
		svc.ouEligibility = suite.fixedOUEligibilityAsked(false, &tidcommon.InternalServerError)

		_, reusable, err := svc.reuseOU(reuseCtx("ou-fixed"), reusedSession())

		suite.Require().Error(err)
		suite.False(reusable)
	})
}

// A sign-in that cannot reuse the session finds no checkpoint, so it authenticates.
func (suite *ServiceTestSuite) TestFindCheckpoint_NoneWhenTheSignInCannotReuseTheSession() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").
		Return(&SessionContext{SessionID: "sess-1", CheckpointID: "session"}, nil)
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, nil).Once()

	got, err := svc.FindCheckpoint(reuseCtx(""), reusedSession(), "session")

	suite.Require().NoError(err)
	suite.Nil(got)
}

// A bounded sign-in with no organization unit loads the checkpoint under the one it follows: the participant row
// records it, and the returned snapshot carries it for the flow, leaving the saved snapshot as it was.
func (suite *ServiceTestSuite) TestLoadCheckpoint_FollowsTheLastUsedOU() {
	svc, m := suite.newService()
	live := liveStoreSession()
	live.SubjectID = "user-2"
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(live, nil)
	saved := &SessionContext{SessionID: "sess-1", CheckpointID: "session", RuntimeData: map[string]string{"k": "v"}}
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").Return(saved, nil)
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil).Maybe()
	withLastUsedOU(m)
	svc.ouEligibility = suite.eligibilityAsked(true, nil)
	var recorded Participant
	m.store.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p Participant) (string, error) { recorded = p; return p.TokenFamilyID, nil })

	loaded, err := svc.LoadCheckpoint(reuseCtx(""), loadInput())
	sc := loaded.Context

	suite.Require().NoError(err)
	suite.Equal(map[string]string{"k": "v", common.RuntimeKeyOUID: "ou-last"}, sc.RuntimeData)
	suite.Equal("ou-last", recorded.OUID)
	suite.Equal(map[string]string{"k": "v"}, saved.RuntimeData)
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_RefusesASignInThatCanNoLongerReuseTheSession() {
	svc, m := suite.newService()
	live := liveStoreSession()
	live.SubjectID = "user-2"
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(live, nil)
	m.store.EXPECT().GetByCheckpoint(mock.Anything, "sess-1", "session").
		Return(&SessionContext{SessionID: "sess-1", CheckpointID: "session"}, nil)
	m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil).Maybe()
	withLastUsedOU(m)
	svc.ouEligibility = suite.eligibilityAsked(false, nil)

	_, err := svc.LoadCheckpoint(reuseCtx(""), loadInput())

	suite.Require().Error(err)
	m.store.AssertNotCalled(suite.T(), "Record", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) newServiceWithRevoker() (*service, *serviceMocks, *CriteriaRevokerMock) {
	m := &serviceMocks{
		store: newSessionStoreMock(suite.T()),
		tx:    transactionmock.NewTransactionerMock(suite.T()),
	}
	revoker := NewCriteriaRevokerMock(suite.T())
	svc := &service{
		store:           m.store,
		resolver:        newResolver(m.store, defaultTimeouts(suite.T())),
		transactioner:   m.tx,
		criteriaRevoker: revoker,
		bus:             suite.newBus(),
		timeouts:        defaultTimeouts(suite.T()),
		logger:          log.GetLogger(),
	}
	return svc, m, revoker
}

func (suite *ServiceTestSuite) TestDetachApplication_DetachesWithoutEndingSharedSessions() {
	svc, m, revoker := suite.newServiceWithRevoker()

	m.store.EXPECT().ListByAppID(mock.Anything, "app-doomed").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-doomed", TokenFamilyID: "tfid-a"},
	}, nil)
	runTx(m)
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-a").Return(nil)
	m.store.EXPECT().DeleteParticipant(mock.Anything, "sess-1", "app-doomed", "").Return(nil)
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-survivor", TokenFamilyID: "tfid-b"},
	}, nil)

	err := svc.DetachApplication(context.Background(), "app-doomed")

	suite.Require().NoError(err)
	m.store.AssertNotCalled(suite.T(), "DeleteSession", mock.Anything, mock.Anything)
	revoker.AssertNotCalled(suite.T(), "RevokeTokenFamily", mock.Anything, "tfid-b")
}

func (suite *ServiceTestSuite) TestDetachApplication_DeletesSessionWhenLastParticipantGoes() {
	svc, m, revoker := suite.newServiceWithRevoker()

	m.store.EXPECT().ListByAppID(mock.Anything, "app-only").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-only", TokenFamilyID: "tfid-a"},
	}, nil)
	runTx(m)
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-a").Return(nil)
	m.store.EXPECT().DeleteParticipant(mock.Anything, "sess-1", "app-only", "").Return(nil)
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, nil)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)

	err := svc.DetachApplication(context.Background(), "app-only")

	suite.Require().NoError(err)
}

// Each organization unit the application signed in under is its own row: each family is revoked and each
// row dropped, and the session ends only once the last of them is gone.
func (suite *ServiceTestSuite) TestDetachApplication_DetachesEveryOURow() {
	svc, m, revoker := suite.newServiceWithRevoker()

	m.store.EXPECT().ListByAppID(mock.Anything, "app-1").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-1", OUID: "ou-x", TokenFamilyID: "tfid-x"},
		{SessionID: "sess-1", AppID: "app-1", OUID: "ou-y", TokenFamilyID: "tfid-y"},
	}, nil)
	runTx(m)
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-x").Return(nil).Once()
	m.store.EXPECT().DeleteParticipant(mock.Anything, "sess-1", "app-1", "ou-x").Return(nil).Once()
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-1", OUID: "ou-y", TokenFamilyID: "tfid-y"},
	}, nil).Once()
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-y").Return(nil).Once()
	m.store.EXPECT().DeleteParticipant(mock.Anything, "sess-1", "app-1", "ou-y").Return(nil).Once()
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, nil).Once()
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil).Once()
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil).Once()

	err := svc.DetachApplication(context.Background(), "app-1")

	suite.Require().NoError(err)
}

func (suite *ServiceTestSuite) TestDetachApplication_DetachesParticipationWithNoTokenFamily() {
	svc, m, revoker := suite.newServiceWithRevoker()

	m.store.EXPECT().ListByAppID(mock.Anything, "app-embedded").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-embedded"},
	}, nil)
	runTx(m)
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "").Return(nil)
	m.store.EXPECT().DeleteParticipant(mock.Anything, "sess-1", "app-embedded", "").Return(nil)
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-other"},
	}, nil)

	err := svc.DetachApplication(context.Background(), "app-embedded")

	suite.Require().NoError(err)
}

func (suite *ServiceTestSuite) TestDetachApplication_RevocationFailureAbortsTheDetachment() {
	svc, m, revoker := suite.newServiceWithRevoker()

	m.store.EXPECT().ListByAppID(mock.Anything, "app-doomed").Return([]Participant{
		{SessionID: "sess-1", AppID: "app-doomed", TokenFamilyID: "tfid-a"},
	}, nil)
	runTx(m)
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-a").Return(errors.New("deny list unavailable"))

	err := svc.DetachApplication(context.Background(), "app-doomed")

	suite.Require().Error(err)
	m.store.AssertNotCalled(suite.T(), "DeleteParticipant", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything)
	m.store.AssertNotCalled(suite.T(), "DeleteSession", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestDetachApplication_NoParticipationIsANoOp() {
	svc, m, _ := suite.newServiceWithRevoker()

	m.store.EXPECT().ListByAppID(mock.Anything, "app-unused").Return(nil, nil)

	suite.Require().NoError(svc.DetachApplication(context.Background(), "app-unused"))
	m.tx.AssertNotCalled(suite.T(), "Transact", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestDetachApplication_EmptyAppIDIsANoOp() {
	svc, m, _ := suite.newServiceWithRevoker()

	suite.Require().NoError(svc.DetachApplication(context.Background(), ""))
	m.store.AssertNotCalled(suite.T(), "ListByAppID", mock.Anything, mock.Anything)
}

// --- Termination listener ---

// newServiceWithListener wires a service with the given revoker (nil allowed) and a listener mock.
func (suite *ServiceTestSuite) newServiceWithListener(revoker CriteriaRevoker) (
	*service, *serviceMocks, *TerminationListenerMock) {
	svc, m := suite.newService()
	svc.criteriaRevoker = revoker
	listener := NewTerminationListenerMock(suite.T())
	suite.Require().NoError(svc.terminationTopic.Hook().Add(listener))
	return svc, m, listener
}

func (suite *ServiceTestSuite) TestTerminate_NotifiesListenerAfterCommit() {
	revoker := NewCriteriaRevokerMock(suite.T())
	svc, m, listener := suite.newServiceWithListener(revoker)
	participants := []Participant{
		{SessionID: "sess-1", AppID: "app-1", TokenFamilyID: "tfid-a"},
		{SessionID: "sess-1", AppID: "app-2", TokenFamilyID: "tfid-b"},
	}

	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	committed := false
	m.tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			err := fn(ctx)
			committed = err == nil
			return err
		}).Once()
	// One read serves both the revoker and the listener.
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(participants, nil).Once()
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-a").Return(nil)
	revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-b").Return(nil)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)
	listener.EXPECT().OnEvent(mock.Anything, TerminatedSession{
		SessionID:    "sess-1",
		SubjectID:    "user-1",
		Participants: participants,
		Reason:       TerminationReasonSignOut,
	}).Run(func(context.Context, TerminatedSession) {
		suite.True(committed, "the listener must run after the terminating transaction")
	}).Once()

	got, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().NoError(err)
	suite.Require().NotNil(got)
}

func (suite *ServiceTestSuite) TestTerminate_ListenerWithoutRevokerStillGetsParticipants() {
	svc, m, listener := suite.newServiceWithListener(nil)
	participants := []Participant{{SessionID: "sess-1", AppID: "app-1"}}

	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	// Without a revoker the read still happens, because the listener needs the list.
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(participants, nil).Once()
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)
	listener.EXPECT().OnEvent(mock.Anything, mock.MatchedBy(func(ended TerminatedSession) bool {
		return ended.SessionID == "sess-1" && len(ended.Participants) == 1 &&
			ended.Reason == TerminationReasonSignOut
	})).Once()

	_, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().NoError(err)
}

func (suite *ServiceTestSuite) TestTerminate_NoListenerNoRevokerReadsNoParticipants() {
	svc, m := suite.newService()
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)

	_, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().NoError(err)
	m.store.AssertNotCalled(suite.T(), "ListBySessionID", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestTerminate_TransactionFailureDoesNotNotify() {
	svc, m, listener := suite.newServiceWithListener(nil)
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{{AppID: "app-1"}}, nil)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(false, errors.New("db down"))

	_, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().Error(err)
	listener.AssertNotCalled(suite.T(), "OnEvent", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestTerminate_ListenerOnlyParticipantReadErrorStillTerminates() {
	svc, m, listener := suite.newServiceWithListener(nil)
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, errors.New("db down"))
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)

	got, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().NoError(err, "a read that only the listener needs must not block sign-out")
	suite.Require().NotNil(got)
	listener.AssertNotCalled(suite.T(), "OnEvent", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestTerminate_ParticipantReadErrorWithRevokerFailsBeforeDeletes() {
	revoker := NewCriteriaRevokerMock(suite.T())
	svc, m, listener := suite.newServiceWithListener(revoker)
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, errors.New("db down"))

	_, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().Error(err, "the revoker cannot run without the list, so the session must stay")
	m.store.AssertNotCalled(suite.T(), "DeleteSession", mock.Anything, mock.Anything)
	listener.AssertNotCalled(suite.T(), "OnEvent", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestTerminateBySubject_NotifiesOncePerSessionAfterCommit() {
	svc, m, listener := suite.newServiceWithListener(nil)
	m.store.EXPECT().ListBySubject(mock.Anything, "user-1").Return([]Session{
		{SessionID: "sess-1", SubjectID: "user-1"},
		{SessionID: "sess-2", SubjectID: "user-1"},
	}, nil)
	// One bulk read before the transaction covers every session.
	m.store.EXPECT().ListBySessionIDs(mock.Anything, []string{"sess-1", "sess-2"}).Return([]Participant{
		{SessionID: "sess-1", AppID: "app-1", TokenFamilyID: "tfid-a"},
		{SessionID: "sess-1", AppID: "app-2", TokenFamilyID: "tfid-b"},
		{SessionID: "sess-2", AppID: "app-1", TokenFamilyID: "tfid-c"},
	}, nil).Once()
	committed := false
	m.tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			err := fn(ctx)
			committed = err == nil
			return err
		}).Once()
	for _, sessionID := range []string{"sess-1", "sess-2"} {
		m.store.EXPECT().DeleteSession(mock.Anything, sessionID).Return(true, nil)
		m.store.EXPECT().Delete(mock.Anything, sessionID).Return(nil)
		m.store.EXPECT().DeleteBySessionID(mock.Anything, sessionID).Return(nil)
	}
	var notified []TerminatedSession
	listener.EXPECT().OnEvent(mock.Anything, mock.Anything).Run(
		func(_ context.Context, ended TerminatedSession) {
			suite.True(committed, "listeners must run after the transaction commits")
			notified = append(notified, ended)
		}).Times(2)

	err := svc.TerminateBySubject(context.Background(), "user-1")

	suite.Require().NoError(err)
	suite.Require().Len(notified, 2)
	suite.Equal("sess-1", notified[0].SessionID)
	suite.Equal("user-1", notified[0].SubjectID)
	suite.Equal(TerminationReasonSubjectRevocation, notified[0].Reason)
	suite.Len(notified[0].Participants, 2, "participants are grouped by session")
	suite.Equal("sess-2", notified[1].SessionID)
	suite.Len(notified[1].Participants, 1)
	suite.Equal("tfid-c", notified[1].Participants[0].TokenFamilyID)
	m.store.AssertNotCalled(suite.T(), "ListBySessionID", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestTerminateBySubject_NoListenerSkipsParticipantRead() {
	svc, m := suite.newService()
	m.store.EXPECT().ListBySubject(mock.Anything, "user-1").Return([]Session{{SessionID: "sess-1"}}, nil)
	runTx(m)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)

	err := svc.TerminateBySubject(context.Background(), "user-1")

	suite.Require().NoError(err)
	m.store.AssertNotCalled(suite.T(), "ListBySessionIDs", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestTerminateBySubject_ParticipantReadErrorStillTerminates() {
	svc, m, listener := suite.newServiceWithListener(nil)
	m.store.EXPECT().ListBySubject(mock.Anything, "user-1").Return([]Session{{SessionID: "sess-1"}}, nil)
	m.store.EXPECT().ListBySessionIDs(mock.Anything, []string{"sess-1"}).Return(nil, errors.New("db down"))
	runTx(m)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)

	err := svc.TerminateBySubject(context.Background(), "user-1")

	suite.Require().NoError(err, "a failed participant read must not block the revocation")
	listener.AssertNotCalled(suite.T(), "OnEvent", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestTerminateBySubject_TransactionFailureDoesNotNotify() {
	svc, m, listener := suite.newServiceWithListener(nil)
	m.store.EXPECT().ListBySubject(mock.Anything, "user-1").Return([]Session{{SessionID: "sess-1"}}, nil)
	m.store.EXPECT().ListBySessionIDs(mock.Anything, []string{"sess-1"}).
		Return([]Participant{{SessionID: "sess-1", AppID: "app-1"}}, nil)
	runTx(m)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(false, errors.New("db down"))

	err := svc.TerminateBySubject(context.Background(), "user-1")

	suite.Require().Error(err)
	listener.AssertNotCalled(suite.T(), "OnEvent", mock.Anything, mock.Anything)
}

// A concurrent termination removed the row first, so this node stays silent and the other notifies.
func (suite *ServiceTestSuite) TestTerminate_DoesNotNotifyWhenAnotherTerminationRemovedTheRow() {
	svc, m, listener := suite.newServiceWithListener(nil)
	m.store.EXPECT().GetByHandle(mock.Anything, "handle-abc").Return(liveStoreSession(), nil)
	runTx(m)
	m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{{AppID: "app-1"}}, nil)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(false, nil)
	m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
	m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)

	got, err := svc.Terminate(context.Background(), "handle-abc", "flow-1")

	suite.Require().NoError(err)
	suite.Require().NotNil(got)
	listener.AssertNotCalled(suite.T(), "OnEvent", mock.Anything, mock.Anything)
}

// Only the sessions this call actually removed are announced; one already removed elsewhere is skipped.
func (suite *ServiceTestSuite) TestTerminateBySubject_NotifiesOnlyForRowsItRemoved() {
	svc, m, listener := suite.newServiceWithListener(nil)
	m.store.EXPECT().ListBySubject(mock.Anything, "user-1").Return([]Session{
		{SessionID: "sess-1", SubjectID: "user-1"},
		{SessionID: "sess-2", SubjectID: "user-1"},
	}, nil)
	m.store.EXPECT().ListBySessionIDs(mock.Anything, []string{"sess-1", "sess-2"}).Return([]Participant{
		{SessionID: "sess-1", AppID: "app-1"},
		{SessionID: "sess-2", AppID: "app-1"},
	}, nil)
	runTx(m)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(false, nil)
	m.store.EXPECT().DeleteSession(mock.Anything, "sess-2").Return(true, nil)
	for _, sessionID := range []string{"sess-1", "sess-2"} {
		m.store.EXPECT().Delete(mock.Anything, sessionID).Return(nil)
		m.store.EXPECT().DeleteBySessionID(mock.Anything, sessionID).Return(nil)
	}
	listener.EXPECT().OnEvent(mock.Anything, mock.MatchedBy(func(ended TerminatedSession) bool {
		return ended.SessionID == "sess-2"
	})).Once()

	err := svc.TerminateBySubject(context.Background(), "user-1")

	suite.Require().NoError(err)
}

// An expired session is deleted without a notification, since expiry already ended it.
func (suite *ServiceTestSuite) TestTerminateBySubject_NotifiesOnlyForLiveSessions() {
	svc, m, listener := suite.newServiceWithListener(nil)
	now := time.Now()
	m.store.EXPECT().ListBySubject(mock.Anything, "user-1").Return([]Session{
		{SessionID: "live", IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(2 * time.Hour)},
		{SessionID: "idle", IdleExpiresAt: now.Add(-time.Minute), AbsoluteExpiresAt: now.Add(time.Hour)},
		{SessionID: "ended", IdleExpiresAt: now.Add(time.Hour), AbsoluteExpiresAt: now.Add(-time.Minute)},
		// Within its stored deadlines, but idle longer than the timeout in effect allows.
		{SessionID: "shortened", LastActiveAt: now.Add(-31 * 24 * time.Hour), IdleExpiresAt: now.Add(time.Hour),
			AbsoluteExpiresAt: now.Add(2 * time.Hour)},
	}, nil)
	m.store.EXPECT().ListBySessionIDs(mock.Anything, []string{"live", "idle", "ended", "shortened"}).Return(nil, nil)
	runTx(m)
	for _, sessionID := range []string{"live", "idle", "ended", "shortened"} {
		m.store.EXPECT().DeleteSession(mock.Anything, sessionID).Return(true, nil)
		m.store.EXPECT().Delete(mock.Anything, sessionID).Return(nil)
		m.store.EXPECT().DeleteBySessionID(mock.Anything, sessionID).Return(nil)
	}
	listener.EXPECT().OnEvent(mock.Anything, mock.MatchedBy(func(ended TerminatedSession) bool {
		return ended.SessionID == "live"
	})).Once()

	err := svc.TerminateBySubject(context.Background(), "user-1")

	suite.Require().NoError(err)
}

// --- CreateDirect ---

func directInput() CreateDirectInput {
	return CreateDirectInput{
		SessionID: "direct-1", SubjectID: "user-1", FlowID: "flow-1", FlowVersion: 3, AppID: "app-123",
		TokenFamilyID: "tfid-1",
		ClientInfo:    ClientInfo{IP: "203.0.113.10", UserAgent: "Mozilla/5.0 (Macintosh) Chrome/140.0"},
	}
}

func (suite *ServiceTestSuite) TestCreateDirect_RecordsTheSessionAndItsParticipant() {
	svc, m := suite.newService()
	svc.timeouts = readTimeouts(suite.T())
	runTx(m)
	var created Session
	m.store.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, s Session) error { created = s; return nil })
	var recorded Participant
	m.store.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p Participant) (string, error) { recorded = p; return p.TokenFamilyID, nil })

	err := svc.CreateDirect(context.Background(), directInput())

	suite.Require().NoError(err)
	suite.Equal("direct-1", created.SessionID)
	suite.Equal(TypeDirect, created.Type)
	suite.Equal("user-1", created.SubjectID)
	suite.Equal("flow-1", created.FlowID)
	suite.Equal(3, created.FlowVersion)
	suite.Empty(created.FlowExecutionID, "a direct session records no flow execution")
	suite.Empty(created.HandleID, "a direct session has no handle to resume it by")
	suite.Equal(StateActive, created.State)
	defaults := newTimeouts(Config{}, deploymentLimits{})
	suite.Equal(created.CreatedAt.Add(defaults.Idle), created.IdleExpiresAt)
	suite.Equal(created.CreatedAt.Add(defaults.Absolute), created.AbsoluteExpiresAt)
	suite.Equal("Mozilla/5.0 (Macintosh) Chrome/140.0", created.Properties.UserAgent)
	suite.Equal("203.0.113.10", created.Properties.CreationIP)
	suite.Equal("203.0.113.10", created.Properties.LastActiveIP)
	suite.Equal(Participant{SessionID: "direct-1", AppID: "app-123", TokenFamilyID: "tfid-1",
		FirstJoinedAt: created.CreatedAt, LastActiveAt: created.CreatedAt}, recorded)
}

func (suite *ServiceTestSuite) TestCreateDirect_RecordsTheParticipantUnderItsOU() {
	svc, m := suite.newService()
	svc.ouEligibility = suite.subjectMaySignIn(true)
	runTx(m)
	m.store.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	var recorded Participant
	m.store.EXPECT().Record(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, p Participant) (string, error) { recorded = p; return p.TokenFamilyID, nil })

	err := svc.CreateDirect(boundaryContext("ou-x"), directInput())

	suite.Require().NoError(err)
	suite.Equal("ou-x", recorded.OUID)
}

func (suite *ServiceTestSuite) TestCreateDirect_FailsWhenTheOUCannotBeChecked() {
	svc, m := suite.newService()
	e := NewOUEligibilityMock(suite.T())
	e.EXPECT().IsEligibleOU(mock.Anything, "ou-x").Return(false, &tidcommon.InternalServerError).Once()
	svc.ouEligibility = e

	err := svc.CreateDirect(boundaryContext("ou-x"), directInput())

	suite.Error(err)
	m.store.AssertNotCalled(suite.T(), "Create", mock.Anything, mock.Anything)
}

func (suite *ServiceTestSuite) TestCreateDirect_FailsWhenTheSessionCannotBeWritten() {
	svc, m := suite.newService()
	runTx(m)
	m.store.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("db down"))

	err := svc.CreateDirect(context.Background(), directInput())

	suite.Error(err)
	m.store.AssertNotCalled(suite.T(), "Record", mock.Anything, mock.Anything)
}

// A session whose participant can't be written would hold a grant no sign-out could revoke, so it fails.
func (suite *ServiceTestSuite) TestCreateDirect_FailsWhenTheParticipantCannotBeWritten() {
	svc, m := suite.newService()
	runTx(m)
	m.store.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", errors.New("db down"))

	err := svc.CreateDirect(context.Background(), directInput())

	suite.Error(err)
}

// noOU is the organization unit scope of a listing that reaches every unit.
var noOU *string

// listNow is the time the listing tests read live sessions at.
var listNow = time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)

func liveListedSession(id string) Session {
	return Session{
		SessionID:         id,
		SubjectID:         "user-1",
		State:             StateActive,
		IdleExpiresAt:     listNow.Add(time.Minute),
		AbsoluteExpiresAt: listNow.Add(time.Hour),
	}
}

func (suite *ServiceTestSuite) TestListLiveBySubject_GroupsParticipantsBySession() {
	svc, m := suite.newService()
	ctx := context.Background()
	m.store.EXPECT().CountLiveBySubject(ctx, "user-1", noOU, listNow).Return(5, nil)
	m.store.EXPECT().ListLiveBySubject(ctx, "user-1", noOU, listNow, 2, 2).
		Return([]Session{liveListedSession("sess-2"), liveListedSession("sess-1")}, nil)
	m.store.EXPECT().ListBySessionIDs(ctx, []string{"sess-2", "sess-1"}).Return([]Participant{
		{SessionID: "sess-1", AppID: "app-a"},
		{SessionID: "sess-2", AppID: "app-b"},
		{SessionID: "sess-1", AppID: "app-c"},
	}, nil)

	page, err := svc.listLiveBySubject(ctx, "user-1", noOU, listNow, 2, 2)

	suite.NoError(err)
	suite.Equal(5, page.TotalResults)
	suite.Require().Len(page.Sessions, 2)
	suite.Equal("sess-2", page.Sessions[0].SessionID, "the store's order is kept")
	suite.Equal([]Participant{{SessionID: "sess-2", AppID: "app-b"}}, page.Sessions[0].Participants)
	suite.Equal([]Participant{{SessionID: "sess-1", AppID: "app-a"}, {SessionID: "sess-1", AppID: "app-c"}},
		page.Sessions[1].Participants)
}

// Under an organization unit, only the sessions and participants of that unit are listed.
func (suite *ServiceTestSuite) TestListLiveByApp_KeepsOnlyTheOUsParticipants() {
	svc, m := suite.newService()
	ctx := context.Background()
	m.store.EXPECT().CountLiveByApp(ctx, "app-2", new("ou-1"), listNow).Return(6, nil)
	m.store.EXPECT().ListLiveByApp(ctx, "app-2", new("ou-1"), listNow, 10, 5).
		Return([]Session{liveListedSession("sess-1")}, nil)
	m.store.EXPECT().ListBySessionIDs(ctx, []string{"sess-1"}).Return([]Participant{
		{SessionID: "sess-1", AppID: "app-2", OUID: "ou-1"},
		{SessionID: "sess-1", AppID: "app-2", OUID: "ou-2"},
		{SessionID: "sess-1", AppID: "app-1"},
	}, nil)

	page, err := svc.listLiveByApp(ctx, "app-2", new("ou-1"), listNow, 10, 5)

	suite.NoError(err)
	suite.Equal(6, page.TotalResults)
	suite.Require().Len(page.Sessions, 1)
	suite.Equal([]Participant{{SessionID: "sess-1", AppID: "app-2", OUID: "ou-1"}},
		page.Sessions[0].Participants)
}

// A scope of no organization unit keeps only the participants recorded under none.
func (suite *ServiceTestSuite) TestListLiveBySubject_NoOUScopeKeepsOnlyNoOUParticipants() {
	svc, m := suite.newService()
	ctx := context.Background()
	m.store.EXPECT().CountLiveBySubject(ctx, "user-2", new(""), listNow).Return(1, nil)
	m.store.EXPECT().ListLiveBySubject(ctx, "user-2", new(""), listNow, 30, 0).
		Return([]Session{liveListedSession("sess-1")}, nil)
	m.store.EXPECT().ListBySessionIDs(ctx, []string{"sess-1"}).Return([]Participant{
		{SessionID: "sess-1", AppID: "app-1"},
		{SessionID: "sess-1", AppID: "app-1", OUID: "ou-1"},
	}, nil)

	page, err := svc.listLiveBySubject(ctx, "user-2", new(""), listNow, 30, 0)

	suite.NoError(err)
	suite.Require().Len(page.Sessions, 1)
	suite.Equal([]Participant{{SessionID: "sess-1", AppID: "app-1"}}, page.Sessions[0].Participants)
}

// An offset at or past the total reads no page, so a deep offset costs only the count.
func (suite *ServiceTestSuite) TestListLive_OffsetPastTotalSkipsTheListing() {
	svc, m := suite.newService()
	ctx := context.Background()
	m.store.EXPECT().CountLiveByApp(ctx, "app-1", noOU, listNow).Return(0, nil)
	m.store.EXPECT().CountLiveBySubject(ctx, "user-1", noOU, listNow).Return(3, nil)

	empty, err := svc.listLiveByApp(ctx, "app-1", noOU, listNow, 30, 0)
	suite.NoError(err)
	suite.Zero(empty.TotalResults)
	suite.NotNil(empty.Sessions)
	suite.Empty(empty.Sessions)

	past, err := svc.listLiveBySubject(ctx, "user-1", noOU, listNow, 30, 1000000)
	suite.NoError(err)
	suite.Equal(3, past.TotalResults, "the total is still reported")
	suite.NotNil(past.Sessions)
	suite.Empty(past.Sessions)
}

func (suite *ServiceTestSuite) TestListLive_StoreErrors() {
	ctx := context.Background()
	storeErr := errors.New("db down")

	suite.Run("count by subject", func() {
		svc, m := suite.newService()
		m.store.EXPECT().CountLiveBySubject(ctx, "user-1", noOU, listNow).Return(0, storeErr)
		_, err := svc.listLiveBySubject(ctx, "user-1", noOU, listNow, 30, 0)
		suite.ErrorIs(err, storeErr)
	})
	suite.Run("list by subject", func() {
		svc, m := suite.newService()
		m.store.EXPECT().CountLiveBySubject(ctx, "user-1", noOU, listNow).Return(1, nil)
		m.store.EXPECT().ListLiveBySubject(ctx, "user-1", noOU, listNow, 30, 0).Return(nil, storeErr)
		_, err := svc.listLiveBySubject(ctx, "user-1", noOU, listNow, 30, 0)
		suite.ErrorIs(err, storeErr)
	})
	suite.Run("count by app", func() {
		svc, m := suite.newService()
		m.store.EXPECT().CountLiveByApp(ctx, "app-1", noOU, listNow).Return(0, storeErr)
		_, err := svc.listLiveByApp(ctx, "app-1", noOU, listNow, 30, 0)
		suite.ErrorIs(err, storeErr)
	})
	suite.Run("list by app", func() {
		svc, m := suite.newService()
		m.store.EXPECT().CountLiveByApp(ctx, "app-1", noOU, listNow).Return(1, nil)
		m.store.EXPECT().ListLiveByApp(ctx, "app-1", noOU, listNow, 30, 0).Return(nil, storeErr)
		_, err := svc.listLiveByApp(ctx, "app-1", noOU, listNow, 30, 0)
		suite.ErrorIs(err, storeErr)
	})
	suite.Run("participants", func() {
		svc, m := suite.newService()
		m.store.EXPECT().CountLiveByApp(ctx, "app-1", noOU, listNow).Return(1, nil)
		m.store.EXPECT().ListLiveByApp(ctx, "app-1", noOU, listNow, 30, 0).
			Return([]Session{liveListedSession("sess-1")}, nil)
		m.store.EXPECT().ListBySessionIDs(ctx, []string{"sess-1"}).Return(nil, storeErr)
		_, err := svc.listLiveByApp(ctx, "app-1", noOU, listNow, 30, 0)
		suite.ErrorIs(err, storeErr)
	})
}

func (suite *ServiceTestSuite) TestGetSubjectParticipantByTokenFamily() {
	svc, m := suite.newService()
	ctx := context.Background()
	want := &Participant{SessionID: "sess-1", AppID: "app-1", OUID: "ou-1", TokenFamilyID: "tfid-1"}
	m.store.EXPECT().GetSubjectParticipantByTokenFamily(ctx, "user-1", "tfid-1").Return(want, nil)
	m.store.EXPECT().GetSubjectParticipantByTokenFamily(ctx, "user-1", "tfid-2").Return(nil, errors.New("db down"))

	got, err := svc.getSubjectParticipantByTokenFamily(ctx, "user-1", "tfid-1")
	suite.NoError(err)
	suite.Equal(want, got)

	_, err = svc.getSubjectParticipantByTokenFamily(ctx, "user-1", "tfid-2")
	suite.ErrorContains(err, "failed to get participant by token family")
}

// listedSession is a live session of user-1, with every recorded property, that the store returns.
func listedSession(id string) Session {
	base := time.Date(2026, 6, 16, 10, 0, 0, 0, time.UTC)
	return Session{
		SessionID:         id,
		SubjectID:         "user-1",
		FlowID:            "flow-1",
		HandleID:          "handle-secret",
		AuthenticatedAt:   base,
		CreatedAt:         base,
		LastActiveAt:      base.Add(time.Minute),
		IdleExpiresAt:     base.Add(31 * time.Minute),
		AbsoluteExpiresAt: base.Add(8 * time.Hour),
		Properties: SessionProperties{
			UserAgent:    "Mozilla/5.0",
			CreationIP:   "198.51.100.7",
			LastActiveIP: "203.0.113.10",
		},
		State: StateActive,
	}
}

// expectLivePage makes the store answer one page of live sessions, listed by subject or, when byApp is set,
// by application: the total, the sessions, and the participants read for them.
func expectLivePage(m *serviceMocks, byApp bool, key string, ouID *string, limit, offset, total int,
	sessions []Session, participants ...Participant) {
	sessionIDs := make([]string, 0, len(sessions))
	for _, sess := range sessions {
		sessionIDs = append(sessionIDs, sess.SessionID)
	}
	if byApp {
		m.store.EXPECT().CountLiveByApp(mock.Anything, key, ouID, mock.Anything).Return(total, nil)
		m.store.EXPECT().ListLiveByApp(mock.Anything, key, ouID, mock.Anything, limit, offset).Return(sessions, nil)
	} else {
		m.store.EXPECT().CountLiveBySubject(mock.Anything, key, ouID, mock.Anything).Return(total, nil)
		m.store.EXPECT().ListLiveBySubject(mock.Anything, key, ouID, mock.Anything, limit, offset).
			Return(sessions, nil)
	}
	m.store.EXPECT().ListBySessionIDs(mock.Anything, sessionIDs).Return(participants, nil)
}

// adminContext is an administrator holding the given permissions, under ouID when not empty.
func adminContext(ouID string, permissions ...string) context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("admin-1", ouID, "", permissions, nil))
}

// selfContext is the user user-1 whose token holds the token family tfid, when not empty.
func selfContext(tfid string) context.Context {
	attributes := map[string]interface{}{}
	if tfid != "" {
		attributes["tfid"] = tfid
	}
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("user-1", "", "", nil, attributes))
}

func expectUserDisplay(m *serviceMocks, display string) {
	m.entities.EXPECT().GetEntitiesByIDs(mock.Anything, []string{"user-1"}).Return([]providers.Entity{{
		ID: "user-1", Category: providers.EntityCategoryUser, Type: "employee",
		Attributes: json.RawMessage(`{"name":"` + display + `"}`),
	}}, nil).Once()
	m.entityTypes.EXPECT().GetDisplayAttributesByHandles(mock.Anything, entitytype.TypeCategoryUser,
		[]string{"employee"}).Return(map[string]string{"employee": "name"}, nil).Once()
}

// expectApp makes the application read as the runtime return the application with the name.
func expectApp(m *serviceMocks, appID, name string) {
	m.apps.EXPECT().GetApplication(mock.MatchedBy(security.IsRuntimeContext), appID).
		Return(&providers.Application{Name: name}, nil).Once()
}

func (suite *ServiceTestSuite) TestListSessions_ByUser() {
	svc, m := suite.newService()
	expectLivePage(m, false, "user-1", noOU, 30, 0, 1, []Session{listedSession("sess-1")},
		Participant{SessionID: "sess-1", AppID: "app-a", TokenFamilyID: "tfid-secret"},
		Participant{SessionID: "sess-1", AppID: "app-b"})
	expectUserDisplay(m, "Alice")
	// The caller may hold only the session permission, so applications are read as the runtime.
	m.apps.EXPECT().GetApplication(mock.MatchedBy(security.IsRuntimeContext), "app-a").
		Return(&providers.Application{Name: "App A", LogoURL: "https://example.com/app-a.png"}, nil).Once()
	m.apps.EXPECT().GetApplication(mock.MatchedBy(security.IsRuntimeContext), "app-b").
		Return(nil, &tidcommon.InternalServerError).Once()

	resp, svcErr := svc.ListSessions(adminContext("", "system"), " user-1 ", "", 30, 0)

	suite.Require().Nil(svcErr)
	suite.Equal(1, resp.TotalResults)
	suite.Equal(1, resp.StartIndex)
	suite.Equal(1, resp.Count)
	suite.Require().Len(resp.Sessions, 1)
	got := resp.Sessions[0]
	suite.Equal("sess-1", got.ID)
	suite.Equal("user-1", got.UserID)
	suite.Equal("Alice", got.UserDisplayName)
	suite.Equal("Mozilla/5.0", got.UserAgent)
	suite.Equal("198.51.100.7", got.CreationIP)
	suite.Equal("203.0.113.10", got.LastActiveIP)
	suite.Equal("2026-06-16T10:01:00Z", got.LastActiveAt)
	suite.Equal("2026-06-16T18:00:00Z", got.AbsoluteExpiresAt)
	suite.Nil(got.IsCurrent, "admin listings don't mark a current session")
	suite.Equal([]participantResponse{
		{AppID: "app-a", AppName: "App A", AppLogoURL: "https://example.com/app-a.png"},
		{AppID: "app-b"},
	}, got.Participants)
}

func (suite *ServiceTestSuite) TestListSessions_ByAppResolvesEachNameOnce() {
	svc, m := suite.newService()
	expectLivePage(m, true, "app-a", noOU, 20, 20, 45,
		[]Session{listedSession("sess-1"), listedSession("sess-2")},
		Participant{SessionID: "sess-1", AppID: "app-a"}, Participant{SessionID: "sess-2", AppID: "app-a"})
	m.entities.EXPECT().GetEntitiesByIDs(mock.Anything, []string{"user-1"}).Return(nil, errors.New("db down")).Once()
	expectApp(m, "app-a", "App A")

	resp, svcErr := svc.ListSessions(adminContext("", "system"), "", "app-a", 20, 20)

	suite.Require().Nil(svcErr)
	suite.Equal(45, resp.TotalResults)
	suite.Equal(21, resp.StartIndex)
	suite.Equal(2, resp.Count)
	suite.Contains(resp.Links, sysutils.Link{Href: "/sessions?offset=40&limit=20&appId=app-a", Rel: "next"})
	suite.Empty(resp.Sessions[0].UserDisplayName, "a failed read leaves the name out")
}

// An administrator holding only the org permission sees the sessions and applications of the organization
// unit its token names, each application with the unit it signed in under.
func (suite *ServiceTestSuite) TestListSessions_OrgAdminSeesItsOwnOU() {
	svc, m := suite.newService()
	expectLivePage(m, false, "user-1", new("ou-1"), 30, 0, 1, []Session{listedSession("sess-1")},
		Participant{SessionID: "sess-1", AppID: "app-a", OUID: "ou-1"},
		Participant{SessionID: "sess-1", AppID: "app-b", OUID: "ou-2"})
	m.entities.EXPECT().GetEntitiesByIDs(mock.Anything, []string{"user-1"}).Return([]providers.Entity{}, nil).Once()
	expectApp(m, "app-a", "App A")

	resp, svcErr := svc.ListSessions(adminContext("ou-1", "org:session:view"), "user-1", "", 30, 0)

	suite.Require().Nil(svcErr)
	suite.Require().Len(resp.Sessions, 1)
	suite.Empty(resp.Sessions[0].UserDisplayName, "a subject that can't be found gets no name")
	suite.Equal([]participantResponse{{AppID: "app-a", OUID: "ou-1", AppName: "App A"}},
		resp.Sessions[0].Participants)
}

// The system permission reaches every organization unit, whichever unit the token names.
func (suite *ServiceTestSuite) TestListSessions_SystemPermissionSeesEveryOU() {
	svc, m := suite.newService()
	m.store.EXPECT().CountLiveByApp(mock.Anything, "app-a", noOU, mock.Anything).Return(0, nil)

	resp, svcErr := svc.ListSessions(adminContext("ou-1", "system"), "", "app-a", 30, 0)

	suite.Require().Nil(svcErr)
	suite.Empty(resp.Sessions)
}

// The middleware refuses an org permission without a unit; the service refuses it too rather than reading
// the sessions of every unit.
func (suite *ServiceTestSuite) TestListSessions_OrgAdminWithoutAUnitIsRefused() {
	svc, _ := suite.newService()

	_, svcErr := svc.ListSessions(adminContext("", "org:session:view"), "user-1", "", 30, 0)

	suite.Equal(&tidcommon.ErrorUnauthorized, svcErr)
}

func (suite *ServiceTestSuite) TestListSessions_NeedsExactlyOneFilter() {
	svc, _ := suite.newService()
	for name, filter := range map[string][2]string{
		"no filter": {"", ""}, "blank filter": {" ", ""}, "both filters": {"u", "a"},
	} {
		suite.Run(name, func() {
			_, svcErr := svc.ListSessions(adminContext("", "system"), filter[0], filter[1], 30, 0)
			suite.Equal(&ErrorInvalidListFilter, svcErr)
		})
	}
}

func (suite *ServiceTestSuite) TestListSessions_StoreError() {
	svc, m := suite.newService()
	m.store.EXPECT().CountLiveBySubject(mock.Anything, "user-1", noOU, mock.Anything).
		Return(0, errors.New("db down"))

	_, svcErr := svc.ListSessions(adminContext("", "system"), "user-1", "", 30, 0)

	suite.Equal(&tidcommon.InternalServerError, svcErr)
}

// The caller sees the sessions of the organization unit its own participant was recorded under, whatever its
// token's claims say, and the session that holds its token family is marked as current.
func (suite *ServiceTestSuite) TestListOwnSessions_ScopedByTheTokensParticipant() {
	svc, m := suite.newService()
	m.store.EXPECT().GetSubjectParticipantByTokenFamily(mock.Anything, "user-1", "tfid-mine").
		Return(&Participant{SessionID: "sess-2", AppID: "app-a", OUID: "ou-1"}, nil)
	expectLivePage(m, false, "user-1", new("ou-1"), 30, 0, 2,
		[]Session{listedSession("sess-1"), listedSession("sess-2")},
		Participant{SessionID: "sess-1", AppID: "app-a", OUID: "ou-1", TokenFamilyID: "tfid-other"},
		Participant{SessionID: "sess-2", AppID: "app-a", OUID: "ou-1", TokenFamilyID: "tfid-mine"})
	expectApp(m, "app-a", "App A")

	resp, svcErr := svc.ListOwnSessions(selfContext("tfid-mine"), 30, 0)

	suite.Require().Nil(svcErr)
	suite.Require().Len(resp.Sessions, 2)
	suite.Require().NotNil(resp.Sessions[0].IsCurrent)
	suite.False(*resp.Sessions[0].IsCurrent)
	suite.Require().NotNil(resp.Sessions[1].IsCurrent)
	suite.True(*resp.Sessions[1].IsCurrent)
	suite.Empty(resp.Sessions[1].UserDisplayName, "own sessions don't resolve the user's name")
	suite.Equal("App A", resp.Sessions[1].Participants[0].AppName)
	suite.Empty(resp.Links)
}

// A participant recorded under no organization unit sees only the sessions with a participant under none.
func (suite *ServiceTestSuite) TestListOwnSessions_NoOUParticipant() {
	svc, m := suite.newService()
	m.store.EXPECT().GetSubjectParticipantByTokenFamily(mock.Anything, "user-1", "tfid-mine").
		Return(&Participant{SessionID: "sess-1", AppID: "app-a"}, nil)
	m.store.EXPECT().CountLiveBySubject(mock.Anything, "user-1", new(""), mock.Anything).Return(0, nil)

	resp, svcErr := svc.ListOwnSessions(selfContext("tfid-mine"), 30, 0)

	suite.Require().Nil(svcErr)
	suite.Empty(resp.Sessions)
}

// A token without a family, or one no participant of an SSO session holds, sees no session.
func (suite *ServiceTestSuite) TestListOwnSessions_TokenWithoutAParticipantSeesNothing() {
	svc, m := suite.newService()

	resp, svcErr := svc.ListOwnSessions(selfContext(""), 30, 0)
	suite.Require().Nil(svcErr)
	suite.Zero(resp.TotalResults)
	suite.NotNil(resp.Sessions)
	suite.Empty(resp.Sessions)

	m.store.EXPECT().GetSubjectParticipantByTokenFamily(mock.Anything, "user-1", "tfid-gone").Return(nil, nil)
	resp, svcErr = svc.ListOwnSessions(selfContext("tfid-gone"), 30, 0)
	suite.Require().Nil(svcErr)
	suite.Zero(resp.TotalResults)
	suite.Empty(resp.Sessions)
}

func (suite *ServiceTestSuite) TestListOwnSessions_Errors() {
	suite.Run("participant", func() {
		svc, m := suite.newService()
		m.store.EXPECT().GetSubjectParticipantByTokenFamily(mock.Anything, "user-1", "tfid-mine").
			Return(nil, errors.New("db down"))
		_, svcErr := svc.ListOwnSessions(selfContext("tfid-mine"), 30, 0)
		suite.Equal(&tidcommon.InternalServerError, svcErr)
	})
	suite.Run("listing", func() {
		svc, m := suite.newService()
		m.store.EXPECT().GetSubjectParticipantByTokenFamily(mock.Anything, "user-1", "tfid-mine").
			Return(&Participant{OUID: "ou-1"}, nil)
		m.store.EXPECT().CountLiveBySubject(mock.Anything, "user-1", new("ou-1"), mock.Anything).
			Return(0, errors.New("db down"))
		_, svcErr := svc.ListOwnSessions(selfContext("tfid-mine"), 30, 0)
		suite.Equal(&tidcommon.InternalServerError, svcErr)
	})
}

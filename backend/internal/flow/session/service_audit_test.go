// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	testAuditAdminID   = "admin-1"
	testAuditOtherID   = "other-user"
	testAuditSessionID = "sess-1"
	testAuditUserID    = "user-1"
	testAuditSecondID  = "user-2"
	testAuditAppID     = "app-456"
	testAuditJoinAppID = "app-123"
	testAuditFamilyID  = "tfid-1"
)

var errAuditStore = errors.New("store down")

// publishedEvent is one audit event the session service published.
type publishedEvent struct {
	name    string
	payload audit.Payload
}

// recordingSink keeps every audit event it is handed, in order.
type recordingSink struct {
	mu     sync.Mutex
	events []publishedEvent
}

func (r *recordingSink) Handle(_ context.Context, env eventbus.Envelope, payload audit.Payload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, publishedEvent{name: env.Name, payload: payload})
	return nil
}

// newRecordingBus returns an event bus with a sink subscribed the way the binaries subscribe theirs, so
// an event the allowlist refuses is never recorded. The returned function stops the bus, which delivers
// every queued event, and returns what the sink received.
func newRecordingBus(t *testing.T) (eventbus.EventBusInterface, func() []publishedEvent) {
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	sink := &recordingSink{}
	require.NoError(t, audit.Subscribe(registry, "session-service-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// wantAudit is the whole audit record of one event, compared field by field.
type wantAudit struct {
	event       string
	actor       audit.Actor
	subject     audit.Ref
	target      audit.Ref
	action      audit.Action
	outcome     audit.Outcome
	sessionRef  string
	application string
	details     map[string]string
}

func userRef(id string) audit.Ref {
	return audit.Ref{Type: audit.TargetTypeUser, ID: id}
}

func userActor(id string) audit.Actor {
	return audit.Actor{Type: audit.ActorTypeUser, ID: id}
}

var (
	systemActor    = audit.Actor{Type: audit.ActorTypeSystem}
	anonymousActor = audit.Actor{Type: audit.ActorTypeAnonymous}
)

// auditAdminContext is the context of a management call made by an administrator.
func auditAdminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil))
}

// otherSubjectContext is the context of a flow on the server's authority that kept the bearer token of
// the request it came in on, which another user sent.
func otherSubjectContext() context.Context {
	return security.WithRuntimeContext(security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditOtherID, "ou-1", "", nil, nil)))
}

// boundedContext is otherSubjectContext inside a sign-in boundary of ouID, which may be empty.
func boundedContext(ouID string) context.Context {
	return authnprovidercm.WithSignInBoundary(otherSubjectContext(),
		authnprovidercm.SignInBoundary{OUID: ouID, ApplicationID: testAuditJoinAppID})
}

// refusesOU returns an organization unit check that answers not eligible, with svcErr when the check fails.
func (suite *ServiceTestSuite) refusesOU(svcErr *tidcommon.ServiceError) *OUEligibilityMock {
	e := NewOUEligibilityMock(suite.T())
	e.EXPECT().IsEligibleOU(mock.Anything, mock.Anything).Return(false, svcErr).Maybe()
	return e
}

// requireAudit checks the published events against want, in order, and that none records the session
// handle or a token family.
func (suite *ServiceTestSuite) requireAudit(published []publishedEvent, handle string, want ...wantAudit) {
	suite.Require().Len(published, len(want))
	for i, w := range want {
		got := published[i]
		suite.Equal(w.event, got.name)
		suite.Equal(w.actor, got.payload.Actor())
		suite.Equal(audit.Ref{}, got.payload.OnBehalfOf())
		suite.Equal(w.subject, got.payload.Subject())
		suite.Equal(w.target, got.payload.Target())
		suite.Equal(w.action, got.payload.Action())
		suite.Equal(w.outcome, got.payload.Outcome())
		suite.Equal(w.sessionRef, got.payload.SessionRef())
		suite.Equal(w.application, got.payload.Application())
		suite.Equal(w.details, got.payload.Details())

		recorded := fmt.Sprintf("%+v", got.payload)
		suite.NotContains(recorded, handle, "the session handle is a credential")
		suite.NotContains(recorded, "tfid", "a token family ID is never recorded")
	}
}

// establishes models the establish sequence: GetByExecutionID returns nil until Create persists the row,
// then returns it, so the re-read finds this call's own session and reports it created.
func establishes(m *serviceMocks) {
	var created *Session
	m.store.EXPECT().GetByExecutionID(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, string) (*Session, error) { return created, nil })
	m.store.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(
		func(_ context.Context, s Session) error { created = &s; return nil })
}

// auditSaveInput is a save with a token family, so the check that none is recorded means something.
func auditSaveInput() SaveCheckpointInput {
	in := saveInput()
	in.TokenFamilyID = testAuditFamilyID
	return in
}

// insideTransaction marks the context a transaction callback runs with.
type insideTransaction struct{}

// A first checkpoint that fails leaves no session behind, so the retry that succeeds establishes the session
// and publishes its creation, instead of finding a session it takes for one already audited.
func (suite *ServiceTestSuite) TestSaveCheckpoint_ARetryAfterAFailedFirstCheckpointIsAudited() {
	svc, m := suite.newService()
	bus, published := newRecordingBus(suite.T())
	svc.bus = bus
	var committed, pending *Session
	m.store.EXPECT().GetByExecutionID(mock.Anything, mock.Anything).RunAndReturn(
		func(context.Context, string) (*Session, error) {
			if pending != nil {
				return pending, nil
			}
			return committed, nil
		})
	// A write inside the transaction is held until it commits, and one outside it is committed at once, so
	// a session established before the transaction would survive the failed first save.
	m.store.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, s Session) error {
			if ctx.Value(insideTransaction{}) != nil {
				pending = &s
			} else {
				committed = &s
			}
			return nil
		})
	m.tx.EXPECT().Transact(mock.Anything, mock.Anything).RunAndReturn(
		func(ctx context.Context, fn func(context.Context) error) error {
			err := fn(context.WithValue(ctx, insideTransaction{}, true))
			if err == nil {
				committed = pending
			}
			pending = nil
			return err
		})
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(errAuditStore).Once()
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(nil).Once()
	m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)

	_, firstErr := svc.SaveCheckpoint(otherSubjectContext(), auditSaveInput())
	res, retryErr := svc.SaveCheckpoint(otherSubjectContext(), auditSaveInput())

	suite.ErrorIs(firstErr, errAuditStore)
	suite.Require().NoError(retryErr)
	suite.True(res.Created)
	suite.requireAudit(published(), res.Handle,
		wantAudit{event: audit.EventSessionCreate, actor: userActor(testAuditUserID), target: userRef(testAuditUserID),
			action: audit.ActionCreate, outcome: audit.OutcomeFailed(audit.ReasonServerError),
			application: testAuditJoinAppID},
		wantAudit{event: audit.EventSessionCreate, actor: userActor(testAuditUserID),
			subject: userRef(testAuditUserID), target: userRef(testAuditUserID), action: audit.ActionCreate,
			outcome: audit.OutcomeSucceeded, sessionRef: res.SessionID, application: testAuditJoinAppID})
}

// racesWith models an execution that has no session when the save starts, and finds winner established by a
// concurrent save when this one inserts its own.
func racesWith(m *serviceMocks, winner *Session) {
	m.store.EXPECT().GetByExecutionID(mock.Anything, mock.Anything).Return(nil, nil).Once()
	m.store.EXPECT().GetByExecutionID(mock.Anything, mock.Anything).Return(winner, nil)
	m.store.EXPECT().Create(mock.Anything, mock.Anything).Return(nil)
	runTx(m)
}

// A save that loses the race to a concurrent save of another subject joins nothing, and is a denied create.
func (suite *ServiceTestSuite) TestSaveCheckpoint_ALostRaceToAnotherSubjectIsDenied() {
	svc, m := suite.newService()
	bus, published := newRecordingBus(suite.T())
	svc.bus = bus
	winner := liveStoreSession()
	winner.SubjectID = testAuditOtherID
	racesWith(m, winner)

	res, err := svc.SaveCheckpoint(otherSubjectContext(), auditSaveInput())

	suite.Require().NoError(err)
	suite.True(res.Skipped)
	suite.Empty(res.Handle)
	suite.Empty(res.SessionID)
	suite.requireAudit(published(), winner.HandleID,
		wantAudit{event: audit.EventSessionCreate, actor: userActor(testAuditUserID),
			subject: userRef(testAuditUserID), target: userRef(testAuditUserID), action: audit.ActionCreate,
			outcome: audit.OutcomeDenied(auditReasonSubjectMismatch), application: testAuditJoinAppID})
}

// A save that loses the race to a concurrent save of the same subject and then fails did not fail to create a
// session, so it publishes no failed create.
func (suite *ServiceTestSuite) TestSaveCheckpoint_AFailureAfterALostRaceIsNotAFailedCreate() {
	svc, m := suite.newService()
	bus, published := newRecordingBus(suite.T())
	svc.bus = bus
	winner := liveStoreSession()
	winner.SubjectID = testAuditUserID
	racesWith(m, winner)
	m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(errAuditStore)

	_, err := svc.SaveCheckpoint(otherSubjectContext(), auditSaveInput())

	suite.ErrorIs(err, errAuditStore)
	suite.requireAudit(published(), winner.HandleID)
}

func (suite *ServiceTestSuite) TestSaveCheckpoint_Audit() {
	failed := audit.OutcomeFailed(audit.ReasonServerError)
	failedCreate := func(SaveCheckpointResult) []wantAudit {
		return []wantAudit{{
			event: audit.EventSessionCreate, actor: userActor(testAuditUserID), target: userRef(testAuditUserID),
			action: audit.ActionCreate, outcome: failed, application: testAuditJoinAppID,
		}}
	}
	nothing := func(SaveCheckpointResult) []wantAudit { return nil }
	joinsExisting := func() SaveCheckpointInput {
		in := auditSaveInput()
		in.HandleHint = testHandle
		return in
	}
	attaches := func(m *serviceMocks) {
		m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(liveStoreSession(), nil)
		m.store.EXPECT().TouchAuthenticatedAt(mock.Anything, testAuditSessionID, mock.Anything,
			mock.Anything, mock.Anything).Return(nil)
	}
	cases := []struct {
		name    string
		ctx     context.Context
		input   func() SaveCheckpointInput
		setup   func(svc *service, m *serviceMocks)
		wantErr error
		failed  bool
		want    func(res SaveCheckpointResult) []wantAudit
	}{
		{
			name:  "a new session is created for the authenticated user, whoever the request's bearer is",
			ctx:   otherSubjectContext(),
			input: auditSaveInput,
			setup: func(_ *service, m *serviceMocks) {
				establishes(m)
				runTx(m)
				m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(nil)
				m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)
			},
			want: func(res SaveCheckpointResult) []wantAudit {
				suite.True(res.Created)
				return []wantAudit{{
					event: audit.EventSessionCreate, actor: userActor(testAuditUserID),
					subject: userRef(testAuditUserID), target: userRef(testAuditUserID),
					action: audit.ActionCreate, outcome: audit.OutcomeSucceeded, sessionRef: res.SessionID,
					application: testAuditJoinAppID,
				}}
			},
		},
		{
			name:  "a session that cannot be inserted fails with no subject and no session reference",
			ctx:   otherSubjectContext(),
			input: auditSaveInput,
			setup: func(_ *service, m *serviceMocks) {
				m.store.EXPECT().GetByExecutionID(mock.Anything, mock.Anything).Return(nil, nil)
				runTx(m)
				m.store.EXPECT().Create(mock.Anything, mock.Anything).Return(errAuditStore)
			},
			wantErr: errAuditStore,
			want:    failedCreate,
		},
		{
			// The session is established in the checkpoint's transaction, so a failed write leaves no session.
			name:  "a new session whose checkpoint cannot be written fails",
			ctx:   otherSubjectContext(),
			input: auditSaveInput,
			setup: func(_ *service, m *serviceMocks) {
				establishes(m)
				runTx(m)
				m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(errAuditStore)
			},
			wantErr: errAuditStore,
			want:    failedCreate,
		},
		{
			name:  "a new session whose organization unit cannot be checked fails",
			ctx:   boundedContext("ou-x"),
			input: auditSaveInput,
			setup: func(svc *service, m *serviceMocks) {
				m.store.EXPECT().GetByExecutionID(mock.Anything, mock.Anything).Return(nil, nil)
				svc.ouEligibility = suite.refusesOU(&tidcommon.InternalServerError)
			},
			failed: true,
			want:   failedCreate,
		},
		{
			name:  "a failed lookup of the session the execution would join fails",
			ctx:   context.Background(),
			input: joinsExisting,
			setup: func(_ *service, m *serviceMocks) {
				m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(nil, errAuditStore)
			},
			wantErr: errAuditStore,
			want:    failedCreate,
		},
		{
			name:  "a subject that conflicts with the session it would join is denied",
			ctx:   otherSubjectContext(),
			input: joinsExisting,
			setup: func(_ *service, m *serviceMocks) {
				existing := liveStoreSession()
				existing.SubjectID = "someone-else"
				m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(existing, nil)
			},
			want: func(res SaveCheckpointResult) []wantAudit {
				suite.True(res.Skipped)
				return []wantAudit{{
					event: audit.EventSessionCreate, actor: userActor(testAuditUserID),
					subject: userRef(testAuditUserID), target: userRef(testAuditUserID),
					action: audit.ActionCreate, outcome: audit.OutcomeDenied(auditReasonSubjectMismatch),
					application: testAuditJoinAppID,
				}}
			},
		},
		{
			name:  "a checkpoint attached to an existing session publishes nothing",
			ctx:   otherSubjectContext(),
			input: joinsExisting,
			setup: func(_ *service, m *serviceMocks) {
				attaches(m)
				runTx(m)
				m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(nil)
				m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)
			},
			want: func(res SaveCheckpointResult) []wantAudit {
				suite.False(res.Created)
				return nil
			},
		},
		{
			name:  "a checkpoint that cannot be attached to an existing session publishes nothing",
			ctx:   otherSubjectContext(),
			input: joinsExisting,
			setup: func(_ *service, m *serviceMocks) {
				attaches(m)
				runTx(m)
				m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(errAuditStore)
			},
			wantErr: errAuditStore,
			want:    nothing,
		},
		{
			name:  "an organization unit that cannot be checked for an existing session publishes nothing",
			ctx:   boundedContext("ou-x"),
			input: joinsExisting,
			setup: func(svc *service, m *serviceMocks) {
				attaches(m)
				svc.ouEligibility = suite.refusesOU(&tidcommon.InternalServerError)
			},
			failed: true,
			want:   nothing,
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, m := suite.newService()
			bus, published := newRecordingBus(suite.T())
			svc.bus = bus
			tc.setup(svc, m)

			res, err := svc.SaveCheckpoint(tc.ctx, tc.input())

			switch {
			case tc.wantErr != nil:
				suite.ErrorIs(err, tc.wantErr)
			case tc.failed:
				suite.Require().Error(err)
			default:
				suite.Require().NoError(err)
			}
			handle := res.Handle
			if handle == "" {
				handle = testHandle
			}
			suite.requireAudit(published(), handle, tc.want(res)...)
		})
	}
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_Audit() {
	failed := audit.OutcomeFailed(audit.ReasonServerError)
	known := liveStoreSession
	checkpoint := &SessionContext{SessionID: testAuditSessionID, CheckpointID: "session"}
	reused := wantAudit{
		event: audit.EventSessionReuse, actor: userActor(testAuditUserID),
		subject: userRef(testAuditUserID), target: userRef(testAuditUserID),
		action: audit.ActionReuse, outcome: audit.OutcomeSucceeded,
		sessionRef: testAuditSessionID, application: testAuditAppID,
	}
	deniedOU := wantAudit{
		event: audit.EventSessionReuse, actor: userActor(testAuditUserID),
		subject: userRef(testAuditUserID), target: userRef(testAuditUserID),
		action: audit.ActionReuse, outcome: audit.OutcomeDenied(auditReasonOUNotEligible),
		sessionRef: testAuditSessionID, application: testAuditAppID,
	}
	failedForUser := wantAudit{
		event: audit.EventSessionReuse, actor: userActor(testAuditUserID), target: userRef(testAuditUserID),
		action: audit.ActionReuse, outcome: failed, application: testAuditAppID,
	}
	failedForNoUser := func(outcome audit.Outcome) wantAudit {
		return wantAudit{
			event: audit.EventSessionReuse, actor: anonymousActor, target: userRef(""),
			action: audit.ActionReuse, outcome: outcome, application: testAuditAppID,
		}
	}
	readsSession := func(m *serviceMocks) {
		m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(known(), nil)
		m.store.EXPECT().GetByCheckpoint(mock.Anything, testAuditSessionID, "session").Return(checkpoint, nil)
		m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
	}
	cases := []struct {
		name    string
		ctx     context.Context
		input   func() LoadCheckpointInput
		setup   func(svc *service, m *serviceMocks)
		wantErr bool
		want    []wantAudit
	}{
		{
			name:  "a reused session is recorded for its user and the joining application",
			ctx:   otherSubjectContext(),
			input: loadInput,
			setup: func(_ *service, m *serviceMocks) {
				readsSession(m)
				m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)
			},
			want: []wantAudit{reused},
		},
		{
			name: "a session and context the check node forwarded are recorded the same",
			ctx:  otherSubjectContext(),
			input: func() LoadCheckpointInput {
				in := loadInput()
				in.Session = known()
				in.Context = checkpoint
				return in
			},
			setup: func(_ *service, m *serviceMocks) {
				m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
				m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)
			},
			want: []wantAudit{reused},
		},
		{
			name:  "a participant write that is only best effort does not fail the reuse",
			ctx:   otherSubjectContext(),
			input: noTFIDLoadInput,
			setup: func(_ *service, m *serviceMocks) {
				readsSession(m)
				m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", errAuditStore)
			},
			want: []wantAudit{reused},
		},
		{
			name:  "a participant that cannot be recorded for a token family fails with the user as target",
			ctx:   otherSubjectContext(),
			input: loadInput,
			setup: func(_ *service, m *serviceMocks) {
				readsSession(m)
				m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", errAuditStore)
			},
			wantErr: true,
			want:    []wantAudit{failedForUser},
		},
		{
			name:  "a checkpoint that no longer exists fails with the user as target",
			ctx:   otherSubjectContext(),
			input: loadInput,
			setup: func(_ *service, m *serviceMocks) {
				m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(known(), nil)
				m.store.EXPECT().GetByCheckpoint(mock.Anything, testAuditSessionID, "session").Return(nil, nil)
			},
			wantErr: true,
			want:    []wantAudit{failedForUser},
		},
		{
			name:  "a session that cannot be read fails with an anonymous actor and no user",
			ctx:   otherSubjectContext(),
			input: loadInput,
			setup: func(_ *service, m *serviceMocks) {
				m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(nil, errAuditStore)
			},
			wantErr: true,
			want:    []wantAudit{failedForNoUser(failed)},
		},
		{
			name:  "a session a concurrent sign-out removed fails as not found with no user",
			ctx:   otherSubjectContext(),
			input: loadInput,
			setup: func(_ *service, m *serviceMocks) {
				m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(nil, nil)
			},
			wantErr: true,
			want:    []wantAudit{failedForNoUser(audit.OutcomeFailed(auditReasonSessionNotFound))},
		},
		{
			name:  "an organization unit the subject cannot sign in to is denied",
			ctx:   boundedContext("ou-x"),
			input: loadInput,
			setup: func(svc *service, m *serviceMocks) {
				readsSession(m)
				svc.ouEligibility = suite.refusesOU(nil)
			},
			wantErr: true,
			want:    []wantAudit{deniedOU},
		},
		{
			name:  "a session with no last-used organization unit cannot be reused and is denied",
			ctx:   boundedContext(""),
			input: loadInput,
			setup: func(_ *service, m *serviceMocks) {
				readsSession(m)
				m.store.EXPECT().ListBySessionID(mock.Anything, testAuditSessionID).Return(nil, nil)
			},
			wantErr: true,
			want:    []wantAudit{deniedOU},
		},
		{
			name:  "a last-used organization unit that cannot be read fails",
			ctx:   boundedContext(""),
			input: loadInput,
			setup: func(_ *service, m *serviceMocks) {
				readsSession(m)
				m.store.EXPECT().ListBySessionID(mock.Anything, testAuditSessionID).Return(nil, errAuditStore)
			},
			wantErr: true,
			want:    []wantAudit{failedForUser},
		},
		{
			name:  "a fixed organization unit that cannot be checked fails",
			ctx:   boundedContext("ou-x"),
			input: loadInput,
			setup: func(svc *service, m *serviceMocks) {
				readsSession(m)
				svc.ouEligibility = suite.refusesOU(&tidcommon.InternalServerError)
			},
			wantErr: true,
			want:    []wantAudit{failedForUser},
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, m := suite.newService()
			bus, published := newRecordingBus(suite.T())
			svc.bus = bus
			tc.setup(svc, m)

			res, err := svc.LoadCheckpoint(tc.ctx, tc.input())
			sess := res.Session

			if tc.wantErr {
				suite.Require().Error(err)
				suite.Nil(sess)
			} else {
				suite.Require().NoError(err)
			}
			suite.requireAudit(published(), testHandle, tc.want...)
		})
	}
}

func (suite *ServiceTestSuite) TestLoadCheckpoint_AuditWithoutAHandle() {
	svc, _ := suite.newService()
	bus, published := newRecordingBus(suite.T())
	svc.bus = bus

	_, err := svc.LoadCheckpoint(context.Background(), LoadCheckpointInput{AppID: testAuditAppID})

	suite.Require().Error(err)
	suite.requireAudit(published(), testHandle, wantAudit{
		event: audit.EventSessionReuse, actor: anonymousActor, target: userRef(""),
		action: audit.ActionReuse, outcome: audit.OutcomeFailed(audit.ReasonServerError),
		application: testAuditAppID,
	})
}

func (suite *ServiceTestSuite) TestTerminate_Audit() {
	signOut := map[string]string{audit.DetailEndReason: string(TerminationReasonSignOut)}
	deletes := func(m *serviceMocks, removed bool) {
		m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(liveStoreSession(), nil)
		runTx(m)
		m.store.EXPECT().DeleteSession(mock.Anything, testAuditSessionID).Return(removed, nil)
		m.store.EXPECT().Delete(mock.Anything, testAuditSessionID).Return(nil)
		m.store.EXPECT().DeleteBySessionID(mock.Anything, testAuditSessionID).Return(nil)
	}
	cases := []struct {
		name    string
		handle  string
		setup   func(m *serviceMocks)
		wantErr bool
		want    []wantAudit
	}{
		{
			name: "a sign-out is the session's user, not the request's bearer", handle: testHandle,
			setup: func(m *serviceMocks) { deletes(m, true) },
			want: []wantAudit{{
				event: audit.EventSessionEnd, actor: userActor(testAuditUserID), subject: userRef(testAuditUserID),
				target: userRef(testAuditUserID), action: audit.ActionEnd, outcome: audit.OutcomeSucceeded,
				sessionRef: testAuditSessionID, details: signOut,
			}},
		},
		{
			name: "a session a concurrent termination removed publishes nothing", handle: testHandle,
			setup: func(m *serviceMocks) { deletes(m, false) },
		},
		{
			name: "a session that does not exist publishes nothing", handle: testHandle,
			setup: func(m *serviceMocks) { m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(nil, nil) },
		},
		{
			name: "no handle publishes nothing", handle: "",
			setup: func(*serviceMocks) {},
		},
		{
			name: "a handle of another flow publishes nothing", handle: testHandle,
			setup: func(m *serviceMocks) {
				other := liveStoreSession()
				other.FlowID = testOtherFlowID
				m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(other, nil)
			},
			wantErr: true,
		},
		{
			name: "a terminating transaction that fails is one failed event for the user", handle: testHandle,
			setup: func(m *serviceMocks) {
				m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(liveStoreSession(), nil)
				runTx(m)
				m.store.EXPECT().DeleteSession(mock.Anything, testAuditSessionID).Return(false, errAuditStore)
			},
			wantErr: true,
			want: []wantAudit{{
				event: audit.EventSessionEnd, actor: userActor(testAuditUserID), target: userRef(testAuditUserID),
				action: audit.ActionEnd, outcome: audit.OutcomeFailed(audit.ReasonServerError), details: signOut,
			}},
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, m := suite.newService()
			bus, published := newRecordingBus(suite.T())
			svc.bus = bus
			tc.setup(m)

			_, err := svc.Terminate(otherSubjectContext(), tc.handle, "flow-1")

			if tc.wantErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
			suite.requireAudit(published(), testHandle, tc.want...)
		})
	}
}

func (suite *ServiceTestSuite) TestTerminateBySubject_Audit() {
	revoked := map[string]string{audit.DetailEndReason: string(TerminationReasonSubjectRevocation)}
	failed := audit.OutcomeFailed(audit.ReasonServerError)
	sessions := []Session{
		{SessionID: "sess-1", SubjectID: testAuditUserID, HandleID: testHandle},
		{SessionID: "sess-2", SubjectID: testAuditUserID, HandleID: "handle-def"},
	}
	endedBy := func(actor audit.Actor, sessionID string) wantAudit {
		return wantAudit{
			event: audit.EventSessionEnd, actor: actor, subject: userRef(testAuditUserID),
			target: userRef(testAuditUserID), action: audit.ActionEnd, outcome: audit.OutcomeSucceeded,
			sessionRef: sessionID, details: revoked,
		}
	}
	deletes := func(m *serviceMocks, removed map[string]bool) {
		m.store.EXPECT().ListBySubject(mock.Anything, testAuditUserID).Return(sessions, nil)
		runTx(m)
		for _, sess := range sessions {
			m.store.EXPECT().DeleteSession(mock.Anything, sess.SessionID).Return(removed[sess.SessionID], nil)
			m.store.EXPECT().Delete(mock.Anything, sess.SessionID).Return(nil)
			m.store.EXPECT().DeleteBySessionID(mock.Anything, sess.SessionID).Return(nil)
		}
	}
	both := map[string]bool{"sess-1": true, "sess-2": true}
	adminActor := userActor(testAuditAdminID)
	cases := []struct {
		name    string
		ctx     context.Context
		setup   func(m *serviceMocks)
		wantErr bool
		want    []wantAudit
	}{
		{
			name:  "every session that ended has its own event with the administrator as actor",
			ctx:   auditAdminContext(),
			setup: func(m *serviceMocks) { deletes(m, both) },
			want:  []wantAudit{endedBy(adminActor, "sess-1"), endedBy(adminActor, "sess-2")},
		},
		{
			name:  "a session a concurrent termination removed has no event",
			ctx:   auditAdminContext(),
			setup: func(m *serviceMocks) { deletes(m, map[string]bool{"sess-1": false, "sess-2": true}) },
			want:  []wantAudit{endedBy(adminActor, "sess-2")},
		},
		{
			name:  "a revocation in a runtime flow is the system, whoever the request's bearer is",
			ctx:   otherSubjectContext(),
			setup: func(m *serviceMocks) { deletes(m, both) },
			want:  []wantAudit{endedBy(systemActor, "sess-1"), endedBy(systemActor, "sess-2")},
		},
		{
			name:  "a revocation in a runtime flow with no caller is the system",
			ctx:   security.WithRuntimeContext(context.Background()),
			setup: func(m *serviceMocks) { deletes(m, both) },
			want:  []wantAudit{endedBy(systemActor, "sess-1"), endedBy(systemActor, "sess-2")},
		},
		{
			name: "a subject with no session publishes nothing",
			ctx:  auditAdminContext(),
			setup: func(m *serviceMocks) {
				m.store.EXPECT().ListBySubject(mock.Anything, testAuditUserID).Return(nil, nil)
			},
		},
		{
			name: "a terminating transaction that fails is one failed event for the user",
			ctx:  auditAdminContext(),
			setup: func(m *serviceMocks) {
				m.store.EXPECT().ListBySubject(mock.Anything, testAuditUserID).Return(sessions, nil)
				runTx(m)
				m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
				m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
				m.store.EXPECT().DeleteBySessionID(mock.Anything, "sess-1").Return(nil)
				m.store.EXPECT().DeleteSession(mock.Anything, "sess-2").Return(false, errAuditStore)
			},
			wantErr: true,
			want: []wantAudit{{
				event: audit.EventSessionEnd, actor: adminActor, target: userRef(testAuditUserID),
				action: audit.ActionEnd, outcome: failed, details: revoked,
			}},
		},
		{
			name: "a failed revocation in a runtime flow is the system",
			ctx:  otherSubjectContext(),
			setup: func(m *serviceMocks) {
				m.store.EXPECT().ListBySubject(mock.Anything, testAuditUserID).Return(sessions, nil)
				runTx(m)
				m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(false, errAuditStore)
			},
			wantErr: true,
			want: []wantAudit{{
				event: audit.EventSessionEnd, actor: systemActor, target: userRef(testAuditUserID),
				action: audit.ActionEnd, outcome: failed, details: revoked,
			}},
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, m := suite.newService()
			bus, published := newRecordingBus(suite.T())
			svc.bus = bus
			tc.setup(m)

			err := svc.TerminateBySubject(tc.ctx, testAuditUserID)

			if tc.wantErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
			suite.requireAudit(published(), testHandle, tc.want...)
		})
	}
}

func (suite *ServiceTestSuite) TestDetachApplication_Audit() {
	detached := map[string]string{audit.DetailEndReason: string(terminationReasonApplicationDetached)}
	endedBy := func(actor audit.Actor, sessionID, userID string) wantAudit {
		w := wantAudit{
			event: audit.EventSessionEnd, actor: actor, subject: userRef(userID),
			target: userRef(userID), action: audit.ActionEnd, outcome: audit.OutcomeSucceeded,
			sessionRef: sessionID, application: "app-1", details: detached,
		}
		if userID == "" {
			w.subject = audit.Ref{}
		}
		return w
	}
	participant := func(sessionID, userID, tfid string) Participant {
		return Participant{SessionID: sessionID, AppID: "app-1", TokenFamilyID: tfid, SubjectID: userID}
	}
	lastParticipant := func(userID string) func(m *serviceMocks, revoker *CriteriaRevokerMock) {
		return func(m *serviceMocks, revoker *CriteriaRevokerMock) {
			m.store.EXPECT().ListByAppID(mock.Anything, "app-1").Return(
				[]Participant{participant("sess-1", userID, "tfid-a")}, nil)
			runTx(m)
			revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-a").Return(nil)
			m.store.EXPECT().DeleteParticipant(mock.Anything, "sess-1", "app-1", "").Return(nil)
			m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, nil)
			m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(true, nil)
			m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
		}
	}
	adminActor := userActor(testAuditAdminID)
	cases := []struct {
		name    string
		ctx     context.Context
		setup   func(m *serviceMocks, revoker *CriteriaRevokerMock)
		wantErr bool
		want    []wantAudit
	}{
		{
			name:  "a session the application was the last participant of has an event for the administrator",
			ctx:   auditAdminContext(),
			setup: lastParticipant(testAuditUserID),
			want:  []wantAudit{endedBy(adminActor, "sess-1", testAuditUserID)},
		},
		{
			name:  "a detachment in a runtime flow is the system, whoever the request's bearer is",
			ctx:   otherSubjectContext(),
			setup: lastParticipant(testAuditUserID),
			want:  []wantAudit{endedBy(systemActor, "sess-1", testAuditUserID)},
		},
		{
			name: "a session whose row was already gone has no event",
			ctx:  auditAdminContext(),
			setup: func(m *serviceMocks, revoker *CriteriaRevokerMock) {
				m.store.EXPECT().ListByAppID(mock.Anything, "app-1").Return(
					[]Participant{participant("sess-1", "", "tfid-a")}, nil)
				runTx(m)
				revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-a").Return(nil)
				m.store.EXPECT().DeleteParticipant(mock.Anything, "sess-1", "app-1", "").Return(nil)
				m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, nil)
				m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(false, nil)
				m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
			},
		},
		{
			name: "each session that was deleted has its own event",
			ctx:  auditAdminContext(),
			setup: func(m *serviceMocks, revoker *CriteriaRevokerMock) {
				m.store.EXPECT().ListByAppID(mock.Anything, "app-1").Return([]Participant{
					participant("sess-1", testAuditUserID, "tfid-a"),
					participant("sess-2", testAuditSecondID, "tfid-b"),
				}, nil)
				runTx(m)
				revoker.EXPECT().RevokeTokenFamily(mock.Anything, mock.Anything).Return(nil)
				m.store.EXPECT().DeleteParticipant(mock.Anything, mock.Anything, "app-1", "").Return(nil)
				m.store.EXPECT().ListBySessionID(mock.Anything, mock.Anything).Return(nil, nil)
				m.store.EXPECT().DeleteSession(mock.Anything, mock.Anything).Return(true, nil)
				m.store.EXPECT().Delete(mock.Anything, mock.Anything).Return(nil)
			},
			want: []wantAudit{
				endedBy(adminActor, "sess-1", testAuditUserID), endedBy(adminActor, "sess-2", testAuditSecondID),
			},
		},
		{
			name: "a session another application still participates in has no event",
			ctx:  auditAdminContext(),
			setup: func(m *serviceMocks, revoker *CriteriaRevokerMock) {
				m.store.EXPECT().ListByAppID(mock.Anything, "app-1").Return(
					[]Participant{participant("sess-1", testAuditUserID, "tfid-a")}, nil)
				runTx(m)
				revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-a").Return(nil)
				m.store.EXPECT().DeleteParticipant(mock.Anything, "sess-1", "app-1", "").Return(nil)
				m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return([]Participant{
					{SessionID: "sess-1", AppID: "app-2"}}, nil)
			},
		},
		{
			name: "a session a concurrent termination removed has no event",
			ctx:  auditAdminContext(),
			setup: func(m *serviceMocks, revoker *CriteriaRevokerMock) {
				m.store.EXPECT().ListByAppID(mock.Anything, "app-1").Return(
					[]Participant{participant("sess-1", testAuditUserID, "tfid-a")}, nil)
				runTx(m)
				revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-a").Return(nil)
				m.store.EXPECT().DeleteParticipant(mock.Anything, "sess-1", "app-1", "").Return(nil)
				m.store.EXPECT().ListBySessionID(mock.Anything, "sess-1").Return(nil, nil)
				m.store.EXPECT().DeleteSession(mock.Anything, "sess-1").Return(false, nil)
				m.store.EXPECT().Delete(mock.Anything, "sess-1").Return(nil)
			},
		},
		{
			// An application-wide detachment has no single user or session to name, so a failed one publishes
			// no event.
			name: "a transaction that fails publishes nothing",
			ctx:  auditAdminContext(),
			setup: func(m *serviceMocks, revoker *CriteriaRevokerMock) {
				m.store.EXPECT().ListByAppID(mock.Anything, "app-1").Return(
					[]Participant{participant("sess-1", testAuditUserID, "tfid-a")}, nil)
				runTx(m)
				revoker.EXPECT().RevokeTokenFamily(mock.Anything, "tfid-a").Return(errAuditStore)
			},
			wantErr: true,
		},
		{
			name: "an application in no session publishes nothing",
			ctx:  auditAdminContext(),
			setup: func(m *serviceMocks, _ *CriteriaRevokerMock) {
				m.store.EXPECT().ListByAppID(mock.Anything, "app-1").Return(nil, nil)
			},
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, m, revoker := suite.newServiceWithRevoker()
			bus, published := newRecordingBus(suite.T())
			svc.bus = bus
			tc.setup(m, revoker)

			err := svc.DetachApplication(tc.ctx, "app-1")

			if tc.wantErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
			}
			suite.requireAudit(published(), testHandle, tc.want...)
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (suite *ServiceTestSuite) TestAFailingBusDoesNotFailTheOperation() {
	cases := []struct {
		name string
		run  func(svc *service, m *serviceMocks) error
	}{
		{"establishing a session", func(svc *service, m *serviceMocks) error {
			establishes(m)
			runTx(m)
			m.store.EXPECT().CreateContext(mock.Anything, mock.Anything).Return(nil)
			m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)
			_, err := svc.SaveCheckpoint(context.Background(), saveInput())
			return err
		}},
		{"reusing a session", func(svc *service, m *serviceMocks) error {
			m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(liveStoreSession(), nil)
			m.store.EXPECT().GetByCheckpoint(mock.Anything, testAuditSessionID, "session").
				Return(&SessionContext{SessionID: testAuditSessionID}, nil)
			m.store.EXPECT().Update(mock.Anything, mock.Anything).Return(nil)
			m.store.EXPECT().Record(mock.Anything, mock.Anything).Return("", nil)
			_, err := svc.LoadCheckpoint(context.Background(), loadInput())
			return err
		}},
		{"ending a session", func(svc *service, m *serviceMocks) error {
			m.store.EXPECT().GetByHandle(mock.Anything, testHandle).Return(liveStoreSession(), nil)
			runTx(m)
			m.store.EXPECT().DeleteSession(mock.Anything, testAuditSessionID).Return(true, nil)
			m.store.EXPECT().Delete(mock.Anything, testAuditSessionID).Return(nil)
			m.store.EXPECT().DeleteBySessionID(mock.Anything, testAuditSessionID).Return(nil)
			_, err := svc.Terminate(context.Background(), testHandle, "flow-1")
			return err
		}},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			svc, m := suite.newService()
			failing := eventbusmock.NewEventBusInterfaceMock(suite.T())
			failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
			svc.bus = failing

			suite.Require().NoError(tc.run(svc, m))
		})
	}
}

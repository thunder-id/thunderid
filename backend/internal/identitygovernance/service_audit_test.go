// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package identitygovernance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/sysauthz"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAuditAdminID = "admin-1"
	testAuditNote    = "Credential exposure reported by the help desk"
)

// ServiceAuditTestSuite tests the audit events of the lifecycle operations and lock formations the
// governance service publishes.
type ServiceAuditTestSuite struct {
	suite.Suite
	port   *entityStateProviderMock
	source *accountAccessSourceInterfaceMock
	now    time.Time
}

func TestServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAuditTestSuite))
}

func (s *ServiceAuditTestSuite) SetupTest() {
	s.port = newEntityStateProviderMock(s.T())
	s.source = newAccountAccessSourceInterfaceMock(s.T())
	s.now = time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
}

// discardingBus returns an event bus that drops every event, for a test the audit event does not matter
// to. A test of what the service publishes uses newRecordingBus.
func discardingBus(t *testing.T) eventbus.EventBusInterface {
	bus := eventbusmock.NewEventBusInterfaceMock(t)
	bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()
	return bus
}

// permittingAuthorizer allows the lifecycle operations and requires exactly calls authorizations.
func (s *ServiceAuditTestSuite) permittingAuthorizer(calls int) sysauthz.SystemAuthorizationServiceInterface {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	if calls > 0 {
		authz.EXPECT().IsActionAllowed(mock.Anything, mock.Anything, mock.Anything).Return(true, nil).Times(calls)
	}
	return authz
}

func (s *ServiceAuditTestSuite) at(offset time.Duration) string {
	return s.now.Add(offset).UTC().Format(time.RFC3339)
}

// activeEntity is an ordinary user with no lock state.
func (s *ServiceAuditTestSuite) activeEntity() GovernedEntity {
	return GovernedEntity{ID: "e1", Category: providers.EntityCategoryUser, Type: "person",
		State: providers.EntityStateActive}
}

// suspendedEntity is a user the organization suspended an hour ago.
func (s *ServiceAuditTestSuite) suspendedEntity() GovernedEntity {
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	entity.AccessState.Suspend = &SuspendState{SuspendedAt: s.at(-time.Hour)}
	return entity
}

func (s *ServiceAuditTestSuite) expectRead(entity GovernedEntity) {
	s.port.On("GetGovernedEntity", mock.Anything, entity.ID, true).Return(&entity, nil).Maybe()
}

// expectDependent reads one dependent agent, held by a cascaded suspension when held is true.
func (s *ServiceAuditTestSuite) expectDependent(id string, held bool) {
	dependent := GovernedEntity{ID: id, Category: providers.EntityCategoryAgent, State: providers.EntityStateActive}
	if held {
		dependent.AccessState.CascadeSuspend = &CascadeSuspendState{SuspendedAt: s.at(-time.Hour)}
	}
	s.port.On("GetGovernedEntity", mock.Anything, id, true).Return(&dependent, nil).Once()
}

// requireServiceError checks that svcErr is the expected service error.
func (s *ServiceAuditTestSuite) requireServiceError(svcErr *tidcommon.ServiceError, want tidcommon.ServiceError) {
	s.T().Helper()
	s.Require().NotNil(svcErr)
	s.Require().Equal(want.Code, svcErr.Code)
}

// publishedEvent is one audit event the governance service published.
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
// an event audit refuses never reaches it, and a function that stops the bus and returns the audit
// events written in order.
func newRecordingBus(t *testing.T) (eventbus.EventBusInterface, func() []publishedEvent) {
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	sink := &recordingSink{}
	require.NoError(t, audit.Subscribe(registry, "identity-governance-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// adminContext is the security context of an operator who is not the subject of any test.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil))
}

// auditedService builds the service over the suite's port and source, publishing on a recording bus.
func (s *ServiceAuditTestSuite) auditedService(
	authorizer sysauthz.SystemAuthorizationServiceInterface) (*identityGovernanceService, func() []publishedEvent) {
	bus, published := newRecordingBus(s.T())
	svc := newIdentityGovernanceService(s.port, s.source, transaction.NewNoOpTransactioner(), nil, authorizer, nil,
		bus)
	svc.now = func() time.Time { return s.now }
	return svc, published
}

// requireOnlyEvent checks that exactly one event was published, with the name and action, and returns
// its payload.
func (s *ServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string,
	action audit.Action) audit.Payload {
	s.T().Helper()
	s.Require().Len(events, 1, "one attempt must publish exactly one event")
	s.Require().Equal(name, events[0].name)
	s.Require().Equal(action, events[0].payload.Action())
	return events[0].payload
}

func (s *ServiceAuditTestSuite) TestLifecycleOperationsPublishTheirAuditEvents() {
	user := audit.Ref{Type: audit.TargetTypeUser, ID: "e1"}
	suspended := s.suspendedEntity()

	cases := []struct {
		name    string
		event   string
		action  audit.Action
		entity  GovernedEntity
		expect  func()
		call    func(*identityGovernanceService) *tidcommon.ServiceError
		details map[string]string
	}{
		{"suspend", audit.EventUserSuspend, audit.ActionSuspend, s.activeEntity(),
			func() {
				s.port.On("SetSuspension", mock.Anything, "e1", s.now, testAuditNote).Return(true, nil).Once()
			},
			func(svc *identityGovernanceService) *tidcommon.ServiceError {
				return svc.Suspend(adminContext(), "e1", testAuditNote)
			}, map[string]string{audit.DetailAppliedBy: "organization"}},
		{"unsuspend", audit.EventUserUnsuspend, audit.ActionUnsuspend, suspended,
			func() {
				s.port.On("ClearSuspension", mock.Anything, "e1", mock.Anything, s.at(-time.Hour)).
					Return(true, nil).Once()
			},
			func(svc *identityGovernanceService) *tidcommon.ServiceError {
				return svc.Unsuspend(adminContext(), "e1")
			}, map[string]string{audit.DetailAppliedBy: "organization"}},
		{"unlock", audit.EventUserUnlock, audit.ActionUnlock, s.activeEntity(),
			func() {
				s.port.On("ClearAccessState", mock.Anything, "e1", AllAuthenticationMethods).Return(nil).Once()
			},
			func(svc *identityGovernanceService) *tidcommon.ServiceError {
				return svc.Unlock(adminContext(), "e1")
			}, map[string]string{audit.DetailSelfService: "false"}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			svc, published := s.auditedService(s.permittingAuthorizer(1))
			s.expectRead(tc.entity)
			tc.expect()

			s.Require().Nil(tc.call(svc))

			payload := s.requireOnlyEvent(published(), tc.event, tc.action)
			s.Equal(user, payload.Target())
			s.Equal(user, payload.Subject())
			s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, payload.Actor())
			s.Equal(audit.OutcomeSucceeded, payload.Outcome())
			s.Equal(tc.details, payload.Details())
			s.NotContains(fmt.Sprintf("%+v", payload), testAuditNote, "the operator note must never be recorded")
		})
	}
}

// An owner's suspension and a recovery's unlock are told apart from an operator's in the audit event.
func (s *ServiceAuditTestSuite) TestTheAuditEventSaysWhoAppliedItAndWhetherItWasSelfService() {
	svc, published := s.auditedService(s.permittingAuthorizer(1))
	s.expectRead(s.activeEntity())
	s.port.On("SetOwnerSuspension", mock.Anything, "e1", s.now).Return(true, nil).Once()
	s.Require().Nil(svc.Suspend(WithOwnerApplier(adminContext()), "e1", ""))
	payload := s.requireOnlyEvent(published(), audit.EventUserSuspend, audit.ActionSuspend)
	s.Equal(map[string]string{audit.DetailAppliedBy: "owner"}, payload.Details())

	s.SetupTest()
	svc, published = s.auditedService(s.permittingAuthorizer(1))
	s.expectRead(s.activeEntity())
	s.port.On("ClearAccessState", mock.Anything, "e1", AllAuthenticationMethods).Return(nil).Once()
	s.Require().Nil(svc.Unlock(security.WithRuntimeContext(adminContext()), "e1"))
	payload = s.requireOnlyEvent(published(), audit.EventUserUnlock, audit.ActionUnlock)
	s.Equal(map[string]string{audit.DetailSelfService: "true"}, payload.Details())
}

// A repeated suspend writes nothing and still succeeds, so it is recorded as succeeded.
func (s *ServiceAuditTestSuite) TestARepeatedSuspendIsPublishedAsSucceeded() {
	svc, published := s.auditedService(s.permittingAuthorizer(1))
	entity := s.activeEntity()
	entity.State = providers.EntityStateSuspended
	s.expectRead(entity)

	s.Require().Nil(svc.Suspend(adminContext(), "e1", ""))

	payload := s.requireOnlyEvent(published(), audit.EventUserSuspend, audit.ActionSuspend)
	s.Equal(audit.OutcomeSucceeded, payload.Outcome())
	s.port.AssertNotCalled(s.T(), "SetSuspension", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

// The event name and target type follow the identity's category.
func (s *ServiceAuditTestSuite) TestTheEventAndTargetTypeFollowTheIdentitysCategory() {
	for category, names := range map[providers.EntityCategory][2]string{
		providers.EntityCategoryUser:  {audit.EventUserUnlock, audit.TargetTypeUser},
		providers.EntityCategoryAgent: {audit.EventAgentUnlock, audit.TargetTypeAgent},
		providers.EntityCategoryApp:   {audit.EventApplicationUnlock, audit.TargetTypeApplication},
	} {
		s.Run(string(category), func() {
			s.SetupTest()
			svc, published := s.auditedService(s.permittingAuthorizer(1))
			entity := s.activeEntity()
			entity.Category = category
			s.expectRead(entity)
			s.port.On("ClearAccessState", mock.Anything, "e1", AllAuthenticationMethods).Return(nil).Once()

			s.Require().Nil(svc.Unlock(adminContext(), "e1"))

			payload := s.requireOnlyEvent(published(), names[0], audit.ActionUnlock)
			s.Equal(audit.Ref{Type: names[1], ID: "e1"}, payload.Target())
		})
	}
}

// A refusal by the service's own authorization is recorded as denied, with the identity as subject.
func (s *ServiceAuditTestSuite) TestARefusedOperatorIsPublishedAsDenied() {
	user := audit.Ref{Type: audit.TargetTypeUser, ID: "e1"}
	cases := []struct {
		name       string
		ctx        context.Context
		authorizer func() sysauthz.SystemAuthorizationServiceInterface
	}{
		{"an operator suspending themselves",
			security.WithSecurityContextTest(context.Background(),
				security.NewSecurityContextForTest("e1", "ou-1", "", nil, nil)),
			func() sysauthz.SystemAuthorizationServiceInterface {
				return sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
			}},
		{"an operator outside the identity's boundary", adminContext(),
			func() sysauthz.SystemAuthorizationServiceInterface {
				authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
				authz.EXPECT().IsActionAllowed(mock.Anything, security.ActionUpdateUser, mock.Anything).
					Return(false, nil).Once()
				return authz
			}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			svc, published := s.auditedService(tc.authorizer())
			s.expectRead(s.activeEntity())

			s.requireServiceError(svc.Suspend(tc.ctx, "e1", ""), tidcommon.ErrorUnauthorized)

			payload := s.requireOnlyEvent(published(), audit.EventUserSuspend, audit.ActionSuspend)
			s.Equal(user, payload.Target())
			s.Equal(user, payload.Subject())
			s.Equal(audit.OutcomeDenied("forbidden"), payload.Outcome())
		})
	}
}

// A failed operation keeps the ID the caller sent as the target, names no subject, and records the
// reason from the error code. An identity that was never read writes no line.
func (s *ServiceAuditTestSuite) TestAFailedOperationIsPublishedWithItsReason() {
	cases := []struct {
		name   string
		event  string
		action audit.Action
		setup  func()
		call   func(*identityGovernanceService) *tidcommon.ServiceError
		target audit.Ref
		reason string
	}{
		{"unsuspend of an identity that is not suspended", audit.EventUserUnsuspend, audit.ActionUnsuspend,
			func() { s.expectRead(s.activeEntity()) },
			func(svc *identityGovernanceService) *tidcommon.ServiceError {
				return svc.Unsuspend(adminContext(), "e1")
			},
			audit.Ref{Type: audit.TargetTypeUser, ID: "e1"}, "igs_1002"},
		{"suspend with an over-long note", audit.EventUserSuspend, audit.ActionSuspend,
			func() { s.expectRead(s.activeEntity()) },
			func(svc *identityGovernanceService) *tidcommon.ServiceError {
				return svc.Suspend(adminContext(), "e1", strings.Repeat("a", maxOperatorNoteLength+1))
			},
			audit.Ref{Type: audit.TargetTypeUser, ID: "e1"}, "igs_1004"},
		{"unlock whose write fails", audit.EventUserUnlock, audit.ActionUnlock,
			func() {
				s.expectRead(s.activeEntity())
				s.port.On("ClearAccessState", mock.Anything, "e1", AllAuthenticationMethods).
					Return(errors.New("database unavailable")).Once()
			},
			func(svc *identityGovernanceService) *tidcommon.ServiceError {
				return svc.Unlock(adminContext(), "e1")
			},
			audit.Ref{Type: audit.TargetTypeUser, ID: "e1"}, "sse_5000"},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			svc, published := s.auditedService(s.permittingAuthorizer(1))
			tc.setup()

			s.Require().NotNil(tc.call(svc))

			payload := s.requireOnlyEvent(published(), tc.event, tc.action)
			s.Equal(tc.target, payload.Target())
			s.Equal(audit.Ref{}, payload.Subject(), "a failed attempt names no subject")
			s.Equal(audit.OutcomeFailed(tc.reason), payload.Outcome())
		})
	}
}

// An identity that cannot be read has no category, so no type-specific event can name it.
func (s *ServiceAuditTestSuite) TestUnreadIdentityPublishesNoAuditEvent() {
	s.port.On("GetGovernedEntity", mock.Anything, "missing", true).Return(nil, nil).Once()
	s.port.On("GetGovernedEntity", mock.Anything, "e1", true).
		Return(nil, errors.New("database unavailable")).Once()
	svc, published := s.auditedService(s.permittingAuthorizer(0))

	s.Require().NotNil(svc.Suspend(adminContext(), "missing", ""))
	s.Require().NotNil(svc.Unlock(adminContext(), "e1"))

	s.Empty(published())
}

// ValidateSuspend writes nothing, so it publishes only a refusal, as the suspend it stands for.
func (s *ServiceAuditTestSuite) TestValidateSuspendPublishesOnlyARefusal() {
	svc, published := s.auditedService(s.permittingAuthorizer(2))
	s.expectRead(s.activeEntity())

	s.Require().Nil(svc.ValidateSuspend(adminContext(), "e1", ""))
	s.requireServiceError(svc.ValidateSuspend(adminContext(), "e1", strings.Repeat("a", maxOperatorNoteLength+1)),
		ErrorOperatorNoteTooLong)

	payload := s.requireOnlyEvent(published(), audit.EventUserSuspend, audit.ActionSuspend)
	s.Equal(audit.OutcomeFailed("igs_1004"), payload.Outcome())
	s.Equal(map[string]string{audit.DetailAppliedBy: "organization"}, payload.Details())
}

// A recovery flow runs under a runtime context that may carry anyone's bearer token, so its unlock names
// the identity as actor once released, and anonymous when it was not.
func (s *ServiceAuditTestSuite) TestARecoveryUnlockIsPublishedWithTheIdentityAsActor() {
	user := audit.Ref{Type: audit.TargetTypeUser, ID: "e1"}
	for name, ctx := range map[string]context.Context{
		"no token":        security.WithRuntimeContext(context.Background()),
		"another's token": security.WithRuntimeContext(adminContext()),
	} {
		s.Run(name, func() {
			s.SetupTest()
			svc, published := s.auditedService(s.permittingAuthorizer(1))
			s.expectRead(s.activeEntity())
			s.port.On("ClearAccessState", mock.Anything, "e1", AllAuthenticationMethods).Return(nil).Once()

			s.Require().Nil(svc.Unlock(ctx, "e1"))

			payload := s.requireOnlyEvent(published(), audit.EventUserUnlock, audit.ActionUnlock)
			s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: user.ID}, payload.Actor())
			s.Equal(user, payload.Subject())
			s.Equal(audit.OutcomeSucceeded, payload.Outcome())
		})
	}

	s.SetupTest()
	svc, published := s.auditedService(s.permittingAuthorizer(1))
	s.expectRead(s.activeEntity())
	s.port.On("ClearAccessState", mock.Anything, "e1", AllAuthenticationMethods).
		Return(errors.New("database unavailable")).Once()

	s.Require().NotNil(svc.Unlock(security.WithRuntimeContext(adminContext()), "e1"))

	payload := s.requireOnlyEvent(published(), audit.EventUserUnlock, audit.ActionUnlock)
	s.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, payload.Actor(), "a failed recovery names no actor")
}

// A failure that reaches the threshold forms a lock and publishes it once, with the locked method and
// an anonymous actor even when the request carried a token. A failure below the threshold publishes
// nothing.
func (s *ServiceAuditTestSuite) TestALockFormationIsPublishedWithItsMethod() {
	svc, published := s.auditedService(nil)
	entity := s.activeEntity()
	s.expectRead(entity)
	s.source.EXPECT().Current(mock.Anything).Return(methodGranularityConfig()).Maybe()
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, s.now, mock.Anything).
		Return(LockEntry{FailureCount: 4}, true, nil).Once()
	s.port.On("IncrementFailure", mock.Anything, entity.ID, testMethod, s.now, mock.Anything).
		Return(LockEntry{FailureCount: 5}, true, nil).Once()
	s.port.On("FormLock", mock.Anything, entity.ID, testMethod, mock.Anything, mock.Anything, s.now).
		Return(true, nil).Once()

	for range 2 {
		decision, svcErr := svc.RecordFailure(security.WithRuntimeContext(adminContext()), entity.ID,
			providers.AccessPlaneAuthentication, testMethod)
		s.Require().Nil(svcErr)
		s.True(decision.Admitted)
	}

	payload := s.requireOnlyEvent(published(), audit.EventUserLock, audit.ActionLock)
	user := audit.Ref{Type: audit.TargetTypeUser, ID: entity.ID}
	s.Equal(user, payload.Target())
	s.Equal(user, payload.Subject())
	s.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, payload.Actor())
	s.Equal(audit.OutcomeSucceeded, payload.Outcome())
	s.Equal(map[string]string{audit.DetailAuthenticationMethod: string(testMethod)}, payload.Details())
}

// A failing bus does not fail the operation it records.
func (s *ServiceAuditTestSuite) TestAFailingBusDoesNotFailTheOperation() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	svc := newIdentityGovernanceService(s.port, s.source, transaction.NewNoOpTransactioner(), nil,
		s.permittingAuthorizer(1), nil, failing)
	svc.now = func() time.Time { return s.now }
	s.expectRead(s.activeEntity())
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "").Return(true, nil).Once()

	s.Nil(svc.Suspend(adminContext(), "e1", ""))
}

// Each dependent's cascaded hold is written to the audit event bus as that identity's own suspension or
// unsuspension, before the user's.
func (s *ServiceAuditTestSuite) TestEachCascadedHoldPublishesItsAuditEvent() {
	svc, published := s.auditedService(s.permittingAuthorizer(1))
	svc.SetDependencyRegistry(dependentsRegistry{dependents: []string{"agent-1"}, asked: &[]string{}})
	s.expectRead(s.activeEntity())
	s.expectDependent("agent-1", false)
	s.port.On("SetCascadeSuspension", mock.Anything, "agent-1", s.now).Return(true, nil).Once()
	s.port.On("SetSuspension", mock.Anything, "e1", s.now, "").Return(true, nil).Once()

	s.Require().Nil(svc.Suspend(adminContext(), "e1", ""))

	events := published()
	s.Require().Len(events, 2)
	s.Equal(audit.EventAgentSuspend, events[0].name)
	s.Equal(audit.ActionSuspend, events[0].payload.Action())
	s.Equal(audit.Ref{Type: audit.TargetTypeAgent, ID: "agent-1"}, events[0].payload.Target())
	s.Equal(audit.OutcomeSucceeded, events[0].payload.Outcome())
	s.Equal(map[string]string{audit.DetailAppliedBy: "cascade"}, events[0].payload.Details())
	s.Equal(audit.EventUserSuspend, events[1].name)
	s.Equal(map[string]string{audit.DetailAppliedBy: "organization"}, events[1].payload.Details())

	s.SetupTest()
	svc, published = s.auditedService(s.permittingAuthorizer(1))
	svc.SetDependencyRegistry(dependentsRegistry{dependents: []string{"agent-1"}, asked: &[]string{}})
	s.expectRead(s.suspendedEntity())
	s.expectDependent("agent-1", true)
	s.port.On("ClearCascadeSuspension", mock.Anything, "agent-1").Return(false, errors.New("store down")).Once()

	s.requireServiceError(svc.Unsuspend(adminContext(), "e1"), tidcommon.InternalServerError)

	events = published()
	s.Require().Len(events, 2, "the failed release and the user's failed unsuspension are both published")
	s.Equal(audit.EventAgentUnsuspend, events[0].name)
	s.Equal(audit.ActionUnsuspend, events[0].payload.Action())
	s.Equal(audit.Ref{Type: audit.TargetTypeAgent, ID: "agent-1"}, events[0].payload.Target())
	s.Equal(audit.ResultFailed, events[0].payload.Outcome().Result)
	s.Equal(audit.EventUserUnsuspend, events[1].name)
}

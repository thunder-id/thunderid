// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package ou

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAuditAdminID     = "admin-1"
	testAuditOUID        = "01900000-0000-7000-8000-0000000000aa"
	testAuditOUName      = "Audited Finance Unit"
	testAuditOUHandle    = "audited-finance"
	testAuditDescription = "Audited unit description"
	testAuditPath        = "audited-finance"
)

// publishedEvent is one audit event the organization unit service published.
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
	require.NoError(t, audit.Subscribe(registry, "ou-service-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// discardingBus is an event bus that drops every event, for a test the audit event does not matter to.
// A test of what the service publishes uses newRecordingBus.
type discardingBus struct{}

func (discardingBus) Publish(context.Context, *eventbus.Event) error {
	return nil
}

// OUServiceAuditTestSuite checks the audit event each organization unit change publishes.
type OUServiceAuditTestSuite struct {
	suite.Suite
	store *organizationUnitStoreInterfaceMock
	authz *sysauthzmock.SystemAuthorizationServiceInterfaceMock
}

func TestOUServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(OUServiceAuditTestSuite))
}

func (s *OUServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{}))
}

func (s *OUServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *OUServiceAuditTestSuite) SetupSubTest() {
	s.store = newOrganizationUnitStoreInterfaceMock(s.T())
	s.authz = sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
}

// service returns an organization unit service over the suite's store and access check, publishing on
// bus, that may delete units nothing depends on.
func (s *OUServiceAuditTestSuite) service(bus eventbus.EventBusInterface) *organizationUnitService {
	mtx := new(mockTransactioner)
	mtx.On("Transact", mock.Anything, mock.Anything).Return(nil)
	service := newOrganizationUnitService(s.authz, s.store, mtx, bus).(*organizationUnitService)
	service.dependencyRegistry = &stubDependencyRegistry{resp: emptyDeps()}
	return service
}

// allow makes the access check allow or refuse every caller.
func (s *OUServiceAuditTestSuite) allow(allowed bool) {
	s.authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(allowed, nil)
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "", "", nil, nil))
}

// auditedRequest is a request for a unit whose name, handle and description no event may carry.
func auditedRequest() OrganizationUnitRequestWithID {
	return OrganizationUnitRequestWithID{
		Handle:      testAuditOUHandle,
		Name:        testAuditOUName,
		Description: testAuditDescription,
	}
}

// existingUnit is the stored unit an update or a delete finds.
func existingUnit() OrganizationUnit {
	return OrganizationUnit{ID: testAuditOUID, Handle: testAuditOUHandle, Name: testAuditOUName}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the unit with ouID, with nothing of the unit recorded. A succeeded or denied attempt that names a
// unit has it as subject; a failed one has none.
func (s *OUServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string, action audit.Action,
	ouID string, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	unit := audit.Ref{Type: audit.TargetTypeOU, ID: ouID}
	s.Equal(name, event.name)
	s.Equal(unit, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultFailed || ouID == "" {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(unit, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditOUName, testAuditOUHandle, testAuditDescription} {
		s.NotContains(recorded, value, "no attribute of the unit is recorded")
	}
}

// expectCreate makes the store accept a root unit with the audited request's name and handle.
func (s *OUServiceAuditTestSuite) expectCreate(createErr error) {
	s.store.On("CheckOrganizationUnitNameConflict", mock.Anything, testAuditOUName, (*string)(nil)).
		Return(false, nil)
	s.store.On("CheckOrganizationUnitHandleConflict", mock.Anything, testAuditOUHandle, (*string)(nil)).
		Return(false, nil)
	s.store.On("CreateOrganizationUnit", mock.Anything, mock.AnythingOfType("OrganizationUnit")).
		Return(createErr)
}

// expectUpdate makes the store find the unit by ID and by path, and store its update.
func (s *OUServiceAuditTestSuite) expectUpdate() {
	s.store.On("GetOrganizationUnit", mock.Anything, testAuditOUID).Return(existingUnit(), nil).Maybe()
	s.store.On("GetOrganizationUnitByPath", mock.Anything, []string{testAuditPath}).
		Return(existingUnit(), nil).Maybe()
	s.store.On("IsOrganizationUnitDeclarative", mock.Anything, testAuditOUID).Return(false)
	s.store.On("UpdateOrganizationUnit", mock.Anything, mock.AnythingOfType("OrganizationUnit")).Return(nil)
}

// expectDelete makes the store find the unit by ID and by path, and delete it.
func (s *OUServiceAuditTestSuite) expectDelete() {
	s.store.On("IsOrganizationUnitExists", mock.Anything, testAuditOUID).Return(true, nil).Maybe()
	s.store.On("GetOrganizationUnitByPath", mock.Anything, []string{testAuditPath}).
		Return(existingUnit(), nil).Maybe()
	s.store.On("IsOrganizationUnitDeclarative", mock.Anything, testAuditOUID).Return(false)
	s.store.On("DeleteOrganizationUnit", mock.Anything, testAuditOUID).Return(nil)
}

func (s *OUServiceAuditTestSuite) TestCreateOrganizationUnit_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		request func() OrganizationUnitRequestWithID
		outcome audit.Outcome
	}{
		{
			name:    "the unit is created",
			arrange: func() { s.allow(true); s.expectCreate(nil) },
			request: auditedRequest,
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the request has no name",
			arrange: func() {},
			request: func() OrganizationUnitRequestWithID {
				request := auditedRequest()
				request.Name = ""
				return request
			},
			outcome: audit.OutcomeFailed("ou_1001"),
		},
		{
			name:    "the store fails",
			arrange: func() { s.allow(true); s.expectCreate(errors.New("store failure")) },
			request: auditedRequest,
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name:    "the access check refuses the caller",
			arrange: func() { s.allow(false) },
			request: auditedRequest,
			outcome: audit.OutcomeDenied("forbidden"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			tc.arrange()

			created, svcErr := s.service(bus).CreateOrganizationUnit(adminContext(), tc.request())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventOUCreate, audit.ActionCreate, created.ID, tc.outcome)
		})
	}
}

func (s *OUServiceAuditTestSuite) TestUpdateOrganizationUnit_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the unit is updated",
			arrange: func() { s.allow(true); s.expectUpdate() },
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the unit does not exist",
			arrange: func() {
				s.allow(true)
				s.store.On("GetOrganizationUnit", mock.Anything, testAuditOUID).
					Return(OrganizationUnit{}, ErrOrganizationUnitNotFound)
			},
			outcome: audit.OutcomeFailed("ou_1003"),
		},
		{
			name:    "the access check refuses the caller",
			arrange: func() { s.allow(false) },
			outcome: audit.OutcomeDenied("forbidden"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			tc.arrange()

			_, svcErr := s.service(bus).UpdateOrganizationUnit(adminContext(), testAuditOUID, auditedRequest())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventOUUpdate, audit.ActionUpdate, testAuditOUID, tc.outcome)
		})
	}
}

// An update by handle path names the unit the path resolved to, and no unit when it resolved to none,
// so the path itself is never recorded.
func (s *OUServiceAuditTestSuite) TestUpdateOrganizationUnitByPath_NamesTheUnitThePathResolvedTo() {
	cases := []struct {
		name    string
		path    string
		arrange func()
		ouID    string
		outcome audit.Outcome
	}{
		{
			name:    "the unit is updated",
			path:    testAuditPath,
			arrange: func() { s.allow(true); s.expectUpdate() },
			ouID:    testAuditOUID,
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the access check refuses the caller",
			path: testAuditPath,
			arrange: func() {
				s.allow(false)
				s.store.On("GetOrganizationUnitByPath", mock.Anything, []string{testAuditPath}).
					Return(existingUnit(), nil)
			},
			ouID:    testAuditOUID,
			outcome: audit.OutcomeDenied("forbidden"),
		},
		{
			name: "the path resolves to no unit",
			path: testAuditPath,
			arrange: func() {
				s.store.On("GetOrganizationUnitByPath", mock.Anything, []string{testAuditPath}).
					Return(OrganizationUnit{}, ErrOrganizationUnitNotFound)
			},
			outcome: audit.OutcomeFailed("ou_1003"),
		},
		{
			name:    "the path is not a path",
			path:    "  ",
			arrange: func() {},
			outcome: audit.OutcomeFailed("ou_1009"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			tc.arrange()

			_, svcErr := s.service(bus).UpdateOrganizationUnitByPath(adminContext(), tc.path, auditedRequest())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventOUUpdate, audit.ActionUpdate, tc.ouID, tc.outcome)
		})
	}
}

// A delete of a unit that does not exist fails rather than succeeding, so it is recorded as failed.
func (s *OUServiceAuditTestSuite) TestDeleteOrganizationUnit_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the unit is deleted",
			arrange: func() { s.allow(true); s.expectDelete() },
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the unit does not exist",
			arrange: func() {
				s.allow(true)
				s.store.On("IsOrganizationUnitExists", mock.Anything, testAuditOUID).Return(false, nil)
			},
			outcome: audit.OutcomeFailed("ou_1003"),
		},
		{
			name:    "the access check refuses the caller",
			arrange: func() { s.allow(false) },
			outcome: audit.OutcomeDenied("forbidden"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			tc.arrange()

			svcErr := s.service(bus).DeleteOrganizationUnit(adminContext(), testAuditOUID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventOUDelete, audit.ActionDelete, testAuditOUID, tc.outcome)
		})
	}
}

func (s *OUServiceAuditTestSuite) TestDeleteOrganizationUnitByPath_NamesTheUnitThePathResolvedTo() {
	cases := []struct {
		name    string
		arrange func()
		blocked bool
		ouID    string
		outcome audit.Outcome
	}{
		{
			name:    "the unit is deleted",
			arrange: func() { s.allow(true); s.expectDelete() },
			ouID:    testAuditOUID,
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "something still depends on the unit",
			blocked: true,
			arrange: func() {
				s.allow(true)
				s.store.On("GetOrganizationUnitByPath", mock.Anything, []string{testAuditPath}).
					Return(existingUnit(), nil)
				s.store.On("IsOrganizationUnitDeclarative", mock.Anything, testAuditOUID).Return(false)
			},
			ouID:    testAuditOUID,
			outcome: audit.OutcomeFailed("ou_1006"),
		},
		{
			name: "the path resolves to no unit",
			arrange: func() {
				s.store.On("GetOrganizationUnitByPath", mock.Anything, []string{testAuditPath}).
					Return(OrganizationUnit{}, ErrOrganizationUnitNotFound)
			},
			outcome: audit.OutcomeFailed("ou_1003"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			tc.arrange()
			service := s.service(bus)
			if tc.blocked {
				service.dependencyRegistry = &stubDependencyRegistry{resp: blockingDeps()}
			}

			svcErr := service.DeleteOrganizationUnitByPath(adminContext(), testAuditPath)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventOUDelete, audit.ActionDelete, tc.ouID, tc.outcome)
		})
	}
}

// A bus that cannot take the event never fails the change it describes.
func (s *OUServiceAuditTestSuite) TestCreateOrganizationUnit_SucceedsWhenTheBusRefusesTheEvent() {
	s.Run("the bus refuses the event", func() {
		failing := eventbusmock.NewEventBusInterfaceMock(s.T())
		failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
		s.allow(true)
		s.expectCreate(nil)

		created, svcErr := s.service(failing).CreateOrganizationUnit(adminContext(), auditedRequest())

		s.Nil(svcErr)
		s.NotEmpty(created.ID)
	})
}

// A create the declarative store refuses is recorded as failed, like any other refusal that is not an
// access check.
func (s *OUServiceAuditTestSuite) TestCreateOrganizationUnit_PublishesARefusalOfADeclarativeStore() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime(s.T().TempDir(), &config.Config{
		DeclarativeResources: config.DeclarativeResources{Enabled: true},
	}))
	s.Run("the store is declarative", func() {
		bus, published := newRecordingBus(s.T())

		_, svcErr := s.service(bus).CreateOrganizationUnit(adminContext(), auditedRequest())

		s.Equal(&ErrorCannotModifyDeclarativeResource, svcErr)
		s.requireOnlyEvent(published(), audit.EventOUCreate, audit.ActionCreate, "",
			audit.OutcomeFailed("ou_1012"))
	})
}

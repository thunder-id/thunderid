// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzdetailmgt

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const testAuditAdminID = "admin-1"

// publishedEvent is one audit event the authorization details type service published.
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
	require.NoError(t, audit.Subscribe(registry, "authorization-detail-type-service-test", sink))
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

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "", "", nil, nil))
}

// ServiceAuditTestSuite checks the audit event each change to an authorization details type publishes.
type ServiceAuditTestSuite struct {
	suite.Suite
	typeStore *typeStoreInterfaceMock
	published func() []publishedEvent
	service   *authorizationDetailMgtService
}

func TestServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAuditTestSuite))
}

func (s *ServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{}))
	s.typeStore = newTypeStoreInterfaceMock(s.T())
	s.service = s.newService(true)
}

func (s *ServiceAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// newService builds the service over the suite's store, for a caller who may write types when allowed is
// set, publishing on a recording bus.
func (s *ServiceAuditTestSuite) newService(allowed bool) *authorizationDetailMgtService {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(allowed, nil).Maybe()
	var bus eventbus.EventBusInterface
	bus, s.published = newRecordingBus(s.T())
	return newAuthorizationDetailMgtService(s.typeStore, nil, authz, bus).(*authorizationDetailMgtService)
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin on
// the type with typeID. A succeeded attempt names it as subject too; a denied or failed one has no subject,
// since access is checked before the type is looked up. Nothing of the definition is recorded.
func (s *ServiceAuditTestSuite) requireOnlyEvent(name string, action audit.Action, typeID string,
	outcome audit.Outcome) {
	s.T().Helper()
	events := s.published()
	s.Require().Len(events, 1)
	event := events[0]
	target := audit.Ref{Type: audit.TargetTypeAuthorizationDetailType, ID: typeID}
	s.Equal(name, event.name)
	s.Equal(target, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultSucceeded {
		s.Equal(target, event.payload.Subject())
	} else {
		s.Equal(audit.Ref{}, event.payload.Subject(), "only a succeeded attempt names a subject")
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testFlightType, "Flight booking", "maxFare"} {
		s.NotContains(recorded, value, "nothing of the definition is recorded")
	}
}

func (s *ServiceAuditTestSuite) TestCreateAuthorizationDetailType_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		allowed bool
		arrange func()
		created bool
		outcome audit.Outcome
	}{
		{
			name: "the type is created", allowed: true,
			arrange: func() {
				s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).
					Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
				s.typeStore.EXPECT().CreateType(mock.Anything, mock.Anything).Return(nil).Once()
			},
			created: true, outcome: audit.OutcomeSucceeded,
		},
		{name: "the caller may not write types", allowed: false, arrange: func() {},
			outcome: audit.OutcomeDenied("forbidden")},
		{
			name: "a type of that value exists", allowed: true,
			arrange: func() {
				s.typeStore.EXPECT().GetTypeByValue(mock.Anything, testFlightType).Return(flightType(), nil).Once()
			},
			outcome: audit.OutcomeFailed("rar_1003"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.service = s.newService(tc.allowed)
			tc.arrange()

			created, svcErr := s.service.CreateAuthorizationDetailType(adminContext(), apiType(flightType()))

			s.Equal(tc.created, svcErr == nil)
			var typeID string
			if svcErr == nil {
				typeID = created.ID
			}
			s.requireOnlyEvent(audit.EventAuthorizationDetailTypeCreate, audit.ActionCreate, typeID, tc.outcome)
		})
	}
}

func (s *ServiceAuditTestSuite) TestUpdateAuthorizationDetailType_PublishesOneEventPerAttempt() {
	readOnly := flightType()
	readOnly.IsReadOnly = true
	changed := flightType()
	changed.Name = "Flight booking with a cap"
	cases := []struct {
		name    string
		typeID  string
		allowed bool
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the type is updated", typeID: "type-1", allowed: true,
			arrange: func() {
				s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil).Once()
				s.typeStore.EXPECT().UpdateType(mock.Anything, mock.Anything, false).Return(nil).Once()
				s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(changed, nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{name: "the caller may not write types", typeID: "made-up", allowed: false, arrange: func() {},
			outcome: audit.OutcomeDenied("forbidden")},
		{
			name: "the type does not exist", typeID: "missing", allowed: true,
			arrange: func() {
				s.typeStore.EXPECT().GetTypeByID(mock.Anything, "missing").
					Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()
			},
			outcome: audit.OutcomeFailed("rar_1002"),
		},
		{
			name: "the type is declared in a file", typeID: "type-1", allowed: true,
			arrange: func() {
				s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(readOnly, nil).Once()
			},
			outcome: audit.OutcomeFailed("rar_1013"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.service = s.newService(tc.allowed)
			tc.arrange()

			_, svcErr := s.service.UpdateAuthorizationDetailType(adminContext(), tc.typeID, changed)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventAuthorizationDetailTypeUpdate, audit.ActionUpdate, tc.typeID, tc.outcome)
		})
	}
}

func (s *ServiceAuditTestSuite) TestDeleteAuthorizationDetailType_PublishesOneEventPerAttempt() {
	readOnly := flightType()
	readOnly.IsReadOnly = true
	cases := []struct {
		name    string
		allowed bool
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the type is deleted", allowed: true,
			arrange: func() {
				s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil).Once()
				s.typeStore.EXPECT().DeleteType(mock.Anything, "type-1").Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{name: "the caller may not write types", allowed: false, arrange: func() {},
			outcome: audit.OutcomeDenied("forbidden")},
		{
			name: "the type is declared in a file", allowed: true,
			arrange: func() {
				s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(readOnly, nil).Once()
			},
			outcome: audit.OutcomeFailed("rar_1013"),
		},
		{
			name: "the store fails", allowed: true,
			arrange: func() {
				s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil).Once()
				s.typeStore.EXPECT().DeleteType(mock.Anything, "type-1").Return(errors.New("database is down")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.service = s.newService(tc.allowed)
			tc.arrange()

			svcErr := s.service.DeleteAuthorizationDetailType(adminContext(), "type-1")

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventAuthorizationDetailTypeDelete, audit.ActionDelete, "type-1", tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no type records nothing, so a caller cannot write a
// succeeded delete for any ID it chooses.
func (s *ServiceAuditTestSuite) TestDeleteAuthorizationDetailType_PublishesNothingForAMissingType() {
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "missing").
		Return(providers.AuthorizationDetailType{}, errTypeNotFound).Once()

	svcErr := s.service.DeleteAuthorizationDetailType(adminContext(), "missing")

	s.Nil(svcErr)
	s.Empty(s.published())
}

// A bus that cannot take the event never fails the change it describes.
func (s *ServiceAuditTestSuite) TestDeleteAuthorizationDetailType_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.service.bus = failing
	s.typeStore.EXPECT().GetTypeByID(mock.Anything, "type-1").Return(flightType(), nil).Once()
	s.typeStore.EXPECT().DeleteType(mock.Anything, "type-1").Return(nil).Once()

	s.Nil(s.service.DeleteAuthorizationDetailType(adminContext(), "type-1"))
}

func (s *ServiceAuditTestSuite) TestInitializeRefusesANilEventBus() {
	service, exporter, err := Initialize(http.NewServeMux(), nil, nil, nil, nil)

	s.Require().ErrorIs(err, errNilEventBus)
	s.Nil(service)
	s.Nil(exporter)
}

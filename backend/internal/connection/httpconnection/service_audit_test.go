// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package httpconnection

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/resourcedependency"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcedependencymock"
)

const (
	testAuditAdminID      = "admin-1"
	testAuditConnectionID = "019a0000-0000-7000-8000-00000000c001"
	testAuditName         = "Audited Risk Engine"
	testAuditURL          = "https://audited-risk.example.com/evaluate"
	testAuditHeaderValue  = "audited-tenant-header-value"
)

// publishedEvent is one audit event the HTTP connection service published.
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
	require.NoError(t, audit.Subscribe(registry, "httpconnection-service-test", sink))
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

// HTTPConnectionServiceAuditTestSuite covers the audit events the HTTP connection service publishes for the
// connections it creates, updates and deletes.
type HTTPConnectionServiceAuditTestSuite struct {
	suite.Suite
	store *httpConnectionStoreInterfaceMock
}

func TestHTTPConnectionServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(HTTPConnectionServiceAuditTestSuite))
}

func (s *HTTPConnectionServiceAuditTestSuite) SetupTest() {
	initTestRuntime(s.T(), false)
	s.store = newHttpConnectionStoreInterfaceMock(s.T())
}

func (s *HTTPConnectionServiceAuditTestSuite) SetupSubTest() {
	s.store = newHttpConnectionStoreInterfaceMock(s.T())
}

// newService returns a connection service over the store mock that publishes on bus, in which no resource
// uses a connection.
func (s *HTTPConnectionServiceAuditTestSuite) newService(bus eventbus.EventBusInterface) *httpConnectionService {
	ouService := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	ouService.On("ResolveOwner", mock.Anything, "ou-1", "", mock.Anything).Return("ou-1", nil).Maybe()
	ouService.EXPECT().ResolveOwnerForUpdate(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).RunAndReturn(func(
		_ context.Context, _, _, currentID string, _ ou.OwnerErrors,
	) (string, *tidcommon.ServiceError) {
		return currentID, nil
	}).Maybe()
	service := newHTTPConnectionService(s.store, ouService, bus)
	total := 0
	registry := resourcedependencymock.NewRegistryMock(s.T())
	registry.EXPECT().GetDependencies(mock.Anything, resourcedependency.ResourceTypeHTTPConnection, mock.Anything).
		Return(&resourcedependency.DependenciesResponse{TotalResults: &total, Summary: map[string]int{}}, nil).
		Maybe()
	service.SetDependencyRegistry(registry)
	return service
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil))
}

// auditedConnection is a connection that authenticates with a password and sends a tenant header.
func (s *HTTPConnectionServiceAuditTestSuite) auditedConnection() HTTPConnection {
	return HTTPConnection{
		OUID: "ou-1", Name: testAuditName, URL: testAuditURL,
		Headers:        []Header{{Name: "X-Tenant", Value: testAuditHeaderValue}},
		AuthProperties: basicAuthProperties(s.T()),
	}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the connection with id, and that it records nothing of the connection but its ID.
func (s *HTTPConnectionServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string,
	action audit.Action, id string, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	connection := audit.Ref{Type: audit.TargetTypeConnection, ID: id}
	s.Equal(name, event.name)
	s.Equal(connection, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details(), "no property of the connection is recorded")
	if outcome.Result == audit.ResultFailed || id == "" {
		s.Equal(audit.Ref{}, event.payload.Subject(), "a failed attempt may name no connection")
	} else {
		s.Equal(connection, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	s.NotContains(recorded, testPassword, "a credential is never recorded")
	s.NotContains(recorded, testAuditHeaderValue, "no header is recorded")
	s.NotContains(recorded, testAuditURL, "the URL is not recorded")
	s.NotContains(recorded, testAuditName, "the name of the connection is not recorded")
}

func (s *HTTPConnectionServiceAuditTestSuite) TestCreateHTTPConnection_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		mutate  func(*HTTPConnection)
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the connection is created",
			mutate:  func(*HTTPConnection) {},
			arrange: func() { s.store.On("CreateHTTPConnection", mock.Anything, mock.Anything).Return(nil).Once() },
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the request has no name",
			mutate:  func(connection *HTTPConnection) { connection.Name = "" },
			arrange: func() {},
			outcome: audit.OutcomeFailed("con_1017"),
		},
		{
			name:   "the name is taken",
			mutate: func(*HTTPConnection) {},
			arrange: func() {
				s.store.On("CreateHTTPConnection", mock.Anything, mock.Anything).Return(errNameConflict).Once()
			},
			outcome: audit.OutcomeFailed("con_1016"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()
			connection := s.auditedConnection()
			tc.mutate(&connection)

			created, svcErr := service.CreateHTTPConnection(adminContext(), connection)

			id := ""
			if tc.outcome == audit.OutcomeSucceeded {
				s.Require().Nil(svcErr)
				id = created.ID
				s.Require().NotEmpty(id)
			} else {
				s.Require().NotNil(svcErr)
			}
			s.requireOnlyEvent(published(), audit.EventConnectionCreate, audit.ActionCreate, id, tc.outcome)
		})
	}
}

func (s *HTTPConnectionServiceAuditTestSuite) TestUpdateHTTPConnection_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the connection is updated",
			arrange: func() {
				stored := s.auditedConnection()
				stored.ID = testAuditConnectionID
				s.store.On("GetHTTPConnection", mock.Anything, testAuditConnectionID).Return(&stored, nil)
				s.store.On("UpdateHTTPConnection", mock.Anything, mock.Anything).Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the connection does not exist",
			arrange: func() {
				s.store.On("GetHTTPConnection", mock.Anything, testAuditConnectionID).
					Return((*HTTPConnection)(nil), nil)
			},
			outcome: audit.OutcomeFailed("con_1015"),
		},
		{
			name: "the store fails",
			arrange: func() {
				stored := s.auditedConnection()
				stored.ID = testAuditConnectionID
				s.store.On("GetHTTPConnection", mock.Anything, testAuditConnectionID).Return(&stored, nil)
				s.store.On("UpdateHTTPConnection", mock.Anything, mock.Anything).
					Return(errors.New("database unavailable")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			_, svcErr := service.UpdateHTTPConnection(adminContext(), testAuditConnectionID, s.auditedConnection())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventConnectionUpdate, audit.ActionUpdate, testAuditConnectionID,
				tc.outcome)
		})
	}
}

func (s *HTTPConnectionServiceAuditTestSuite) TestDeleteHTTPConnection_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		err     error
		outcome audit.Outcome
	}{
		{name: "the connection is deleted", outcome: audit.OutcomeSucceeded},
		{name: "the connection is declared in a file", err: errHTTPConnectionIsImmutable,
			outcome: audit.OutcomeFailed("dcr_1003")},
		{name: "the store fails", err: errors.New("database unavailable"), outcome: audit.OutcomeFailed("sse_5000")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			s.store.On("DeleteHTTPConnection", mock.Anything, testAuditConnectionID).Return(tc.err).Once()

			svcErr := service.DeleteHTTPConnection(adminContext(), testAuditConnectionID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventConnectionDelete, audit.ActionDelete, testAuditConnectionID,
				tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no connection records nothing, so a caller cannot write a
// succeeded delete for any ID it chooses. The store reports a missing connection with
// errConnectionNotFound, as its delete does when no row matched.
func (s *HTTPConnectionServiceAuditTestSuite) TestDeleteHTTPConnection_PublishesNothingForAMissingConnection() {
	bus, published := newRecordingBus(s.T())
	service := s.newService(bus)
	s.store.On("DeleteHTTPConnection", mock.Anything, testAuditConnectionID).Return(errConnectionNotFound).Once()

	svcErr := service.DeleteHTTPConnection(adminContext(), testAuditConnectionID)

	s.Nil(svcErr)
	s.Empty(published())
}

// A bus that cannot take the event never fails the change it describes.
func (s *HTTPConnectionServiceAuditTestSuite) TestCreateHTTPConnection_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	service := s.newService(failing)
	s.store.On("CreateHTTPConnection", mock.Anything, mock.Anything).Return(nil).Once()

	created, svcErr := service.CreateHTTPConnection(adminContext(), s.auditedConnection())

	s.Nil(svcErr)
	s.NotEmpty(created.ID)
}

func (s *HTTPConnectionServiceAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	service, err := Initialize(config.HTTPConnectionConfig{}, nil, nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(service)
}

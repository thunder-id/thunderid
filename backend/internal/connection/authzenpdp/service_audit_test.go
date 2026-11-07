// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authzenpdp

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
	"github.com/thunder-id/thunderid/internal/system/outboundauth"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	testAuditAdminID      = "admin-1"
	testAuditConnectionID = "019a0000-0000-7000-8000-00000000b001"
	testAuditName         = "Audited Policy Decision Point"
	testAuditEndpoint     = "https://audited-pdp.example.com/access/v1/evaluation"
	testAuditBearerToken  = "audit-test-pdp-bearer-token-0123456789"
)

// publishedEvent is one audit event the AuthZEN PDP connection service published.
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
	require.NoError(t, audit.Subscribe(registry, "authzenpdp-service-test", sink))
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

// AuthZENPDPServiceAuditTestSuite covers the audit events the AuthZEN PDP connection service publishes for
// the connections it creates, updates and deletes.
type AuthZENPDPServiceAuditTestSuite struct {
	suite.Suite
	store *serviceStoreStub
}

func TestAuthZENPDPServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(AuthZENPDPServiceAuditTestSuite))
}

func (s *AuthZENPDPServiceAuditTestSuite) SetupTest() {
	s.store = &serviceStoreStub{}
}

func (s *AuthZENPDPServiceAuditTestSuite) SetupSubTest() {
	s.store = &serviceStoreStub{}
}

// newService returns a connection service over the store stub that publishes on bus.
func (s *AuthZENPDPServiceAuditTestSuite) newService(bus eventbus.EventBusInterface) *AuthZENPDPService {
	return newAuthZENPDPService(s.store, config.AuthZENPDPConfig{}, transaction.NewNoOpTransactioner(), nil,
		newTestOUService(s.T()), bus)
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil))
}

// auditedRequest is a connection to a decision point that authenticates with a bearer token.
func auditedRequest() ConnectionRequest {
	return ConnectionRequest{
		OUID:     "ou-1",
		Name:     testAuditName,
		Endpoint: testAuditEndpoint,
		Authentication: &outboundauth.Authentication{
			Type: string(outboundauth.TypeBearer), Properties: map[string]string{"token": testAuditBearerToken},
		},
	}
}

// storedConnection is the connection auditedRequest stores.
func (s *AuthZENPDPServiceAuditTestSuite) storedConnection() AuthZENPDPConnection {
	return AuthZENPDPConnection{
		ID: testAuditConnectionID, OUID: "ou-1", Name: testAuditName, Endpoint: testAuditEndpoint,
		AuthenticationProperties: testAuthProperties(s.T(), *auditedRequest().Authentication),
	}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the connection with id, and that it records nothing of the connection but its ID.
func (s *AuthZENPDPServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string,
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
	s.NotContains(recorded, testAuditBearerToken, "a credential is never recorded")
	s.NotContains(recorded, testAuditEndpoint, "the endpoint is not recorded")
	s.NotContains(recorded, testAuditName, "the name of the connection is not recorded")
}

func (s *AuthZENPDPServiceAuditTestSuite) TestCreateAuthZENPDPConnection_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		request func() ConnectionRequest
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the connection is created",
			request: auditedRequest,
			arrange: func() {},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the request has no name",
			request: func() ConnectionRequest {
				request := auditedRequest()
				request.Name = ""
				return request
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("con_1010"),
		},
		{
			name:    "the store fails",
			request: auditedRequest,
			arrange: func() { s.store.err = errors.New("store failure") },
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			created, svcErr := service.CreateAuthZENPDPConnection(adminContext(), tc.request())

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

func (s *AuthZENPDPServiceAuditTestSuite) TestUpdateAuthZENPDPConnection_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the connection is updated",
			arrange: func() { s.store.connection = s.storedConnection() },
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the connection does not exist",
			arrange: func() { s.store.missing = true },
			outcome: audit.OutcomeFailed("con_1004"),
		},
		{
			name: "the connection is declared in a file",
			arrange: func() {
				s.store.connection = s.storedConnection()
				s.store.updateErr = ErrAuthZENPDPIsImmutable
			},
			outcome: audit.OutcomeFailed("dcr_1002"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			_, svcErr := service.UpdateAuthZENPDPConnection(adminContext(), testAuditConnectionID, auditedRequest())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventConnectionUpdate, audit.ActionUpdate, testAuditConnectionID,
				tc.outcome)
		})
	}
}

func (s *AuthZENPDPServiceAuditTestSuite) TestDeleteAuthZENPDPConnection_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name:    "the connection is deleted",
			arrange: func() { s.store.connection = s.storedConnection() },
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the connection is declared in a file",
			arrange: func() {
				s.store.connection = s.storedConnection()
				s.store.deleteErr = ErrAuthZENPDPIsImmutable
			},
			outcome: audit.OutcomeFailed("dcr_1003"),
		},
		{
			name:    "the store fails",
			arrange: func() { s.store.deleteErr = errors.New("store failure") },
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			svcErr := service.DeleteAuthZENPDPConnection(adminContext(), testAuditConnectionID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventConnectionDelete, audit.ActionDelete, testAuditConnectionID,
				tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no connection records nothing, so a caller cannot write
// a succeeded delete for any ID it chooses. The stub reports the missing row as the real store does.
func (s *AuthZENPDPServiceAuditTestSuite) TestDeleteAuthZENPDPConnection_PublishesNothingForAMissingConnection() {
	bus, published := newRecordingBus(s.T())
	service := s.newService(bus)
	s.store.missing = true

	svcErr := service.DeleteAuthZENPDPConnection(adminContext(), testAuditConnectionID)

	s.Nil(svcErr)
	s.Empty(published())
	s.Equal(1, s.store.deleteCalls)
}

// A bus that cannot take the event never fails the change it describes.
func (s *AuthZENPDPServiceAuditTestSuite) TestCreateAuthZENPDPConnection_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	service := s.newService(failing)

	created, svcErr := service.CreateAuthZENPDPConnection(adminContext(), auditedRequest())

	s.Nil(svcErr)
	s.NotEmpty(created.ID)
}

func (s *AuthZENPDPServiceAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	service, err := Initialize(config.AuthZENPDPConfig{}, nil, nil, nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(service)
}

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package presentation

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
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAuditAdminID = "admin-1"
	// testAuditHandle and testAuditClaim are parts of a definition; no event may carry them.
	testAuditHandle = "employee-badge"
	testAuditClaim  = "employee_number"
)

// publishedEvent is one audit event the presentation definition service published.
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
	require.NoError(t, audit.Subscribe(registry, "presentation-definition-service-test", sink))
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

// auditedDefinition is a presentation definition as an admin would send it.
func auditedDefinition() *PresentationDefinitionDTO {
	return &PresentationDefinitionDTO{Handle: testAuditHandle, VCT: "EmployeeBadge",
		RequestedClaims: []string{testAuditClaim}}
}

// ServiceAuditTestSuite checks the audit event each change to a presentation definition publishes.
type ServiceAuditTestSuite struct {
	suite.Suite
	store     *definitionStoreInterfaceMock
	published func() []publishedEvent
	service   *definitionService
}

func TestServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAuditTestSuite))
}

func (s *ServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		OpenID4VP: config.OpenID4VPConfig{Store: string(serverconst.StoreModeMutable)},
	}))
	s.store = newStatefulDefinitionStore(s.T())
	s.service = s.newService(true)
}

func (s *ServiceAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// newService builds the service over the suite's store, for a caller who may write definitions when
// allowed is set, publishing on a recording bus.
func (s *ServiceAuditTestSuite) newService(allowed bool) *definitionService {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(allowed, nil).Maybe()
	var bus eventbus.EventBusInterface
	bus, s.published = newRecordingBus(s.T())
	return newPresentationDefinitionService(s.store, nil, authz, nil, false, bus)
}

// stored creates the audited definition through a service that publishes nothing, and returns its ID.
func (s *ServiceAuditTestSuite) stored() string {
	created, svcErr := newPresentationDefinitionService(s.store, nil, allowAllAuthz(s.T()), nil, false,
		discardingBus{}).CreatePresentationDefinition(adminContext(), auditedDefinition())
	s.Require().Nil(svcErr)
	return created.ID
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin on
// the definition with id. A succeeded or denied attempt on a known definition names it as subject
// too; a failed one, or a denied create, which has no ID yet, has no subject. Nothing of the
// definition is recorded.
func (s *ServiceAuditTestSuite) requireOnlyEvent(name string, action audit.Action, id string,
	outcome audit.Outcome) {
	s.T().Helper()
	events := s.published()
	s.Require().Len(events, 1)
	event := events[0]
	target := audit.Ref{Type: audit.TargetTypePresentationDefinition, ID: id}
	s.Equal(name, event.name)
	s.Equal(target, event.payload.Target())
	s.Equal(action, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultFailed || id == "" {
		s.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		s.Equal(target, event.payload.Subject())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditHandle, testAuditClaim, "EmployeeBadge"} {
		s.NotContains(recorded, value, "nothing of the definition is recorded")
	}
}

func (s *ServiceAuditTestSuite) TestCreatePresentationDefinition_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		allowed bool
		request func() *PresentationDefinitionDTO
		outcome audit.Outcome
	}{
		{name: "the definition is created", allowed: true, request: auditedDefinition,
			outcome: audit.OutcomeSucceeded},
		{name: "the caller may not create definitions", allowed: false, request: auditedDefinition,
			outcome: audit.OutcomeDenied("forbidden")},
		{name: "the definition has no handle", allowed: true, request: func() *PresentationDefinitionDTO {
			invalid := auditedDefinition()
			invalid.Handle = ""
			return invalid
		}, outcome: audit.OutcomeFailed("vp_2001")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.service = s.newService(tc.allowed)

			created, svcErr := s.service.CreatePresentationDefinition(adminContext(), tc.request())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			var id string
			if svcErr == nil {
				id = created.ID
			}
			s.requireOnlyEvent(audit.EventPresentationDefinitionCreate, audit.ActionCreate, id, tc.outcome)
		})
	}
}

func (s *ServiceAuditTestSuite) TestUpdatePresentationDefinition_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		allowed bool
		missing bool
		outcome audit.Outcome
	}{
		{name: "the definition is updated", allowed: true, outcome: audit.OutcomeSucceeded},
		{name: "the caller may not update definitions", allowed: false,
			outcome: audit.OutcomeDenied("forbidden")},
		{name: "the definition does not exist", allowed: true, missing: true,
			outcome: audit.OutcomeFailed("vp_2002")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			id := s.stored()
			if tc.missing {
				id = "missing"
			}
			s.service = s.newService(tc.allowed)

			_, svcErr := s.service.UpdatePresentationDefinition(adminContext(), id, auditedDefinition())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventPresentationDefinitionUpdate, audit.ActionUpdate, id, tc.outcome)
		})
	}
}

func (s *ServiceAuditTestSuite) TestDeletePresentationDefinition_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		allowed bool
		outcome audit.Outcome
	}{
		{name: "the definition is deleted", allowed: true, outcome: audit.OutcomeSucceeded},
		{name: "the caller may not delete definitions", allowed: false,
			outcome: audit.OutcomeDenied("forbidden")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			id := s.stored()
			s.service = s.newService(tc.allowed)

			svcErr := s.service.DeletePresentationDefinition(adminContext(), id)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventPresentationDefinitionDelete, audit.ActionDelete, id, tc.outcome)
		})
	}
}

// A definition declared in a file cannot be deleted through the API, so the attempt is recorded as
// failed.
func (s *ServiceAuditTestSuite) TestDeletePresentationDefinition_PublishesAFailureForAnImmutableOne() {
	id := "declared-1"
	s.store = newDefinitionStoreInterfaceMock(s.T())
	s.store.EXPECT().GetPresentationDefinitionByID(mock.Anything, id).
		Return(&PresentationDefinitionDTO{ID: id, Handle: testAuditHandle}, nil).Once()
	s.store.EXPECT().DeletePresentationDefinition(mock.Anything, id).Return(ErrDefinitionIsImmutable).Once()
	s.service = s.newService(true)

	svcErr := s.service.DeletePresentationDefinition(adminContext(), id)

	s.Require().NotNil(svcErr)
	s.requireOnlyEvent(audit.EventPresentationDefinitionDelete, audit.ActionDelete, id,
		audit.OutcomeFailed("vp_2005"))
}

// A delete stays idempotent, but one that finds no definition records nothing, so a caller cannot
// write a succeeded delete for any ID it chooses.
func (s *ServiceAuditTestSuite) TestDeletePresentationDefinition_PublishesNothingForAMissingOne() {
	svcErr := s.service.DeletePresentationDefinition(adminContext(), "missing")

	s.Nil(svcErr)
	s.Empty(s.published())
}

// A bus that cannot take the event never fails the change it describes.
func (s *ServiceAuditTestSuite) TestCreatePresentationDefinition_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.service.bus = failing

	created, svcErr := s.service.CreatePresentationDefinition(adminContext(), auditedDefinition())

	s.Nil(svcErr)
	s.NotEmpty(created.ID)
}

func (s *ServiceAuditTestSuite) TestInitializeRefusesANilEventBus() {
	service, exporter, err := Initialize(http.NewServeMux(), nil, nil, nil, nil)

	s.Require().ErrorIs(err, errNilEventBus)
	s.Nil(service)
	s.Nil(exporter)
}

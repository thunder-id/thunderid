// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package credential

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
	// testAuditHandle and testAuditClaim are parts of a configuration; no event may carry them.
	testAuditHandle = "employee-badge"
	testAuditClaim  = "employee_number"
)

// publishedEvent is one audit event the credential configuration service published.
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
	require.NoError(t, audit.Subscribe(registry, "credential-configuration-service-test", sink))
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

// auditedConfiguration is a credential configuration as an admin would send it.
func auditedConfiguration() *CredentialConfigurationDTO {
	return &CredentialConfigurationDTO{Handle: testAuditHandle, VCT: "EmployeeBadge",
		Claims: []ClaimMapping{{Name: testAuditClaim}}}
}

// ServiceAuditTestSuite checks the audit event each change to a credential configuration publishes.
type ServiceAuditTestSuite struct {
	suite.Suite
	store     *credentialStoreInterfaceMock
	published func() []publishedEvent
	service   *configurationService
}

func TestServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceAuditTestSuite))
}

func (s *ServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("", &config.Config{
		OpenID4VCI: config.OpenID4VCIConfig{Store: string(serverconst.StoreModeMutable)},
	}))
	s.store = newStatefulCredentialStore(s.T())
	s.service = s.newService(true)
}

func (s *ServiceAuditTestSuite) SetupSubTest() {
	s.SetupTest()
}

func (s *ServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

// newService builds the service over the suite's store, for a caller who may write configurations when
// allowed is set, publishing on a recording bus.
func (s *ServiceAuditTestSuite) newService(allowed bool) *configurationService {
	authz := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(s.T())
	authz.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(allowed, nil).Maybe()
	var bus eventbus.EventBusInterface
	bus, s.published = newRecordingBus(s.T())
	return newCredentialConfigurationService(s.store, nil, authz, nil, false, bus)
}

// stored creates the audited configuration through a service that publishes nothing, and returns its ID.
func (s *ServiceAuditTestSuite) stored() string {
	created, svcErr := newCredentialConfigurationService(s.store, nil, allowAllAuthz(s.T()), nil, false,
		discardingBus{}).CreateCredentialConfiguration(adminContext(), auditedConfiguration())
	s.Require().Nil(svcErr)
	return created.ID
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin on
// the configuration with id. A succeeded or denied attempt on a known configuration names it as subject
// too; a failed one, or a denied create, which has no ID yet, has no subject. Nothing of the
// configuration is recorded.
func (s *ServiceAuditTestSuite) requireOnlyEvent(name string, action audit.Action, id string,
	outcome audit.Outcome) {
	s.T().Helper()
	events := s.published()
	s.Require().Len(events, 1)
	event := events[0]
	target := audit.Ref{Type: audit.TargetTypeCredentialConfiguration, ID: id}
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
		s.NotContains(recorded, value, "nothing of the configuration is recorded")
	}
}

func (s *ServiceAuditTestSuite) TestCreateCredentialConfiguration_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		allowed bool
		request func() *CredentialConfigurationDTO
		outcome audit.Outcome
	}{
		{name: "the configuration is created", allowed: true, request: auditedConfiguration,
			outcome: audit.OutcomeSucceeded},
		{name: "the caller may not create configurations", allowed: false, request: auditedConfiguration,
			outcome: audit.OutcomeDenied("forbidden")},
		{name: "the configuration has no handle", allowed: true, request: func() *CredentialConfigurationDTO {
			invalid := auditedConfiguration()
			invalid.Handle = ""
			return invalid
		}, outcome: audit.OutcomeFailed("vci_2001")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.service = s.newService(tc.allowed)

			created, svcErr := s.service.CreateCredentialConfiguration(adminContext(), tc.request())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			var id string
			if svcErr == nil {
				id = created.ID
			}
			s.requireOnlyEvent(audit.EventCredentialConfigurationCreate, audit.ActionCreate, id, tc.outcome)
		})
	}
}

func (s *ServiceAuditTestSuite) TestUpdateCredentialConfiguration_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		allowed bool
		missing bool
		outcome audit.Outcome
	}{
		{name: "the configuration is updated", allowed: true, outcome: audit.OutcomeSucceeded},
		{name: "the caller may not update configurations", allowed: false,
			outcome: audit.OutcomeDenied("forbidden")},
		{name: "the configuration does not exist", allowed: true, missing: true,
			outcome: audit.OutcomeFailed("vci_2002")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			id := s.stored()
			if tc.missing {
				id = "missing"
			}
			s.service = s.newService(tc.allowed)

			_, svcErr := s.service.UpdateCredentialConfiguration(adminContext(), id, auditedConfiguration())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventCredentialConfigurationUpdate, audit.ActionUpdate, id, tc.outcome)
		})
	}
}

func (s *ServiceAuditTestSuite) TestDeleteCredentialConfiguration_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		allowed bool
		outcome audit.Outcome
	}{
		{name: "the configuration is deleted", allowed: true, outcome: audit.OutcomeSucceeded},
		{name: "the caller may not delete configurations", allowed: false,
			outcome: audit.OutcomeDenied("forbidden")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			id := s.stored()
			s.service = s.newService(tc.allowed)

			svcErr := s.service.DeleteCredentialConfiguration(adminContext(), id)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(audit.EventCredentialConfigurationDelete, audit.ActionDelete, id, tc.outcome)
		})
	}
}

// A configuration declared in a file cannot be deleted through the API, so the attempt is recorded as
// failed.
func (s *ServiceAuditTestSuite) TestDeleteCredentialConfiguration_PublishesAFailureForAnImmutableOne() {
	id := "declared-1"
	s.store = newCredentialStoreInterfaceMock(s.T())
	s.store.EXPECT().GetCredentialConfigurationByID(mock.Anything, id).
		Return(&CredentialConfigurationDTO{ID: id, Handle: testAuditHandle}, nil).Once()
	s.store.EXPECT().DeleteCredentialConfiguration(mock.Anything, id).Return(ErrConfigurationIsImmutable).Once()
	s.service = s.newService(true)

	svcErr := s.service.DeleteCredentialConfiguration(adminContext(), id)

	s.Require().NotNil(svcErr)
	s.requireOnlyEvent(audit.EventCredentialConfigurationDelete, audit.ActionDelete, id,
		audit.OutcomeFailed("vci_2005"))
}

// A delete stays idempotent, but one that finds no configuration records nothing, so a caller cannot
// write a succeeded delete for any ID it chooses.
func (s *ServiceAuditTestSuite) TestDeleteCredentialConfiguration_PublishesNothingForAMissingOne() {
	svcErr := s.service.DeleteCredentialConfiguration(adminContext(), "missing")

	s.Nil(svcErr)
	s.Empty(s.published())
}

// A bus that cannot take the event never fails the change it describes.
func (s *ServiceAuditTestSuite) TestCreateCredentialConfiguration_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.service.bus = failing

	created, svcErr := s.service.CreateCredentialConfiguration(adminContext(), auditedConfiguration())

	s.Nil(svcErr)
	s.NotEmpty(created.ID)
}

func (s *ServiceAuditTestSuite) TestInitializeRefusesANilEventBus() {
	service, exporter, err := Initialize(http.NewServeMux(), nil, nil, nil, nil)

	s.Require().ErrorIs(err, errNilEventBus)
	s.Nil(service)
	s.Nil(exporter)
}

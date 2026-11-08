// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package idp

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/cache"
	"github.com/thunder-id/thunderid/internal/system/cmodels"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/system/utils"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const (
	testAuditAdminID      = "admin-1"
	testAuditIDPID        = "019a0000-0000-7000-8000-00000000a001"
	testAuditIDPName      = "Audited Corporate OIDC"
	testAuditClientID     = "audited-oidc-client"
	testAuditClientSecret = "audit-test-idp-client-secret-0123456789"
)

// publishedEvent is one audit event the identity provider service published.
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
	require.NoError(t, audit.Subscribe(registry, "idp-service-test", sink))
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

// IDPServiceAuditTestSuite covers the audit events the identity provider service publishes for the
// connections it creates, updates and deletes.
type IDPServiceAuditTestSuite struct {
	suite.Suite
	store *idpStoreInterfaceMock
}

func TestIDPServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(IDPServiceAuditTestSuite))
}

func (s *IDPServiceAuditTestSuite) SetupTest() {
	config.ResetServerRuntime()
	s.Require().NoError(config.InitializeServerRuntime("/tmp/test", &config.Config{}))
}

func (s *IDPServiceAuditTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (s *IDPServiceAuditTestSuite) SetupSubTest() {
	s.store = newIdpStoreInterfaceMock(s.T())
}

// newService returns an identity provider service over the store mock that publishes on bus.
func (s *IDPServiceAuditTestSuite) newService(bus eventbus.EventBusInterface) *idpService {
	entityTypes := entitytypemock.NewEntityTypeServiceInterfaceMock(s.T())
	entityTypes.On("GetEntityTypeList", mock.Anything, entitytype.TypeCategoryUser,
		mock.Anything, mock.Anything, mock.Anything).Return(&entitytype.EntityTypeListResponse{}, nil).Maybe()
	ouService := oumock.NewOrganizationUnitServiceInterfaceMock(s.T())
	ouService.On("ResolveOwner", mock.Anything, "ou-1", "", mock.Anything).Return("ou-1", nil).Maybe()
	ouService.EXPECT().ResolveOwnerForUpdate(mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).RunAndReturn(func(
		_ context.Context, _, _, currentID string, _ ou.OwnerErrors,
	) (string, *tidcommon.ServiceError) {
		return currentID, nil
	}).Maybe()
	return &idpService{
		idpStore:           s.store,
		entityTypeService:  entityTypes,
		ouService:          ouService,
		transactioner:      &mockTransactioner{},
		dependencyRegistry: newNoBlockingDepsRegistry(),
		logger:             log.GetLogger().With(log.String(log.LoggerKeyComponentName, "IdPService")),
		uuidGenerator:      utils.GenerateUUIDv7,
		bus:                bus,
	}
}

// adminContext is the context of a management call made by the admin.
func adminContext() context.Context {
	return security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditAdminID, "ou-1", "", nil, nil))
}

// auditedIDP is an OIDC connection with a client secret, as an administrator saves it.
func auditedIDP() *providers.IDPDTO {
	clientID, _ := cmodels.NewProperty("client_id", testAuditClientID, false)
	clientSecret, _ := cmodels.NewProperty("client_secret", testAuditClientSecret, true)
	redirectURI, _ := cmodels.NewProperty("redirect_uri", "https://audit.example.com/callback", false)
	authorize, _ := cmodels.NewProperty("authorization_endpoint", "https://idp.example.com/authorize", false)
	token, _ := cmodels.NewProperty("token_endpoint", "https://idp.example.com/token", false)
	return &providers.IDPDTO{
		OUID:       "ou-1",
		Name:       testAuditIDPName,
		Type:       providers.IDPTypeOIDC,
		Properties: []cmodels.Property{*clientID, *clientSecret, *redirectURI, *authorize, *token},
	}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the connection with id, and that it records nothing of the connection but its ID.
func (s *IDPServiceAuditTestSuite) requireOnlyEvent(events []publishedEvent, name string, action audit.Action,
	id string, outcome audit.Outcome) {
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
	s.NotContains(recorded, testAuditClientSecret, "a client secret is never recorded")
	s.NotContains(recorded, testAuditClientID, "no property of the connection is recorded")
	s.NotContains(recorded, testAuditIDPName, "the name of the connection is not recorded")
}

func (s *IDPServiceAuditTestSuite) TestCreateIdentityProvider_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		idp     func() *providers.IDPDTO
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the connection is created",
			idp:  auditedIDP,
			arrange: func() {
				s.store.On("GetIdentityProviderByName", mock.Anything, testAuditIDPName).
					Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
				s.store.On("CreateIdentityProvider", mock.Anything, mock.Anything).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the request has no name",
			idp: func() *providers.IDPDTO {
				idp := auditedIDP()
				idp.Name = ""
				return idp
			},
			arrange: func() {},
			outcome: audit.OutcomeFailed("idp_1003"),
		},
		{
			name: "the name is taken",
			idp:  auditedIDP,
			arrange: func() {
				s.store.On("GetIdentityProviderByName", mock.Anything, testAuditIDPName).
					Return(&providers.IDPDTO{ID: "other"}, nil)
			},
			outcome: audit.OutcomeFailed("idp_1005"),
		},
		{
			name: "the store fails",
			idp:  auditedIDP,
			arrange: func() {
				s.store.On("GetIdentityProviderByName", mock.Anything, testAuditIDPName).
					Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
				s.store.On("CreateIdentityProvider", mock.Anything, mock.Anything).Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			created, svcErr := service.CreateIdentityProvider(adminContext(), tc.idp())

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

func (s *IDPServiceAuditTestSuite) TestUpdateIdentityProvider_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the connection is updated",
			arrange: func() {
				existing := auditedIDP()
				existing.ID = testAuditIDPID
				s.store.On("GetIdentityProvider", mock.Anything, testAuditIDPID).Return(existing, nil)
				s.store.On("UpdateIdentityProvider", mock.Anything, mock.Anything).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the connection does not exist",
			arrange: func() {
				s.store.On("GetIdentityProvider", mock.Anything, testAuditIDPID).
					Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
			},
			outcome: audit.OutcomeFailed("idp_1001"),
		},
		{
			name: "the connection is declared in a file",
			arrange: func() {
				existing := auditedIDP()
				existing.ID = testAuditIDPID
				s.store.On("GetIdentityProvider", mock.Anything, testAuditIDPID).Return(existing, nil)
				s.store.On("UpdateIdentityProvider", mock.Anything, mock.Anything).Return(ErrIDPIsImmutable)
			},
			outcome: audit.OutcomeFailed("idp_1010"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			_, svcErr := service.UpdateIdentityProvider(adminContext(), testAuditIDPID, auditedIDP())

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventConnectionUpdate, audit.ActionUpdate, testAuditIDPID,
				tc.outcome)
		})
	}
}

func (s *IDPServiceAuditTestSuite) TestDeleteIdentityProvider_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		arrange func()
		outcome audit.Outcome
	}{
		{
			name: "the connection is deleted",
			arrange: func() {
				s.store.On("GetIdentityProvider", mock.Anything, testAuditIDPID).Return(auditedIDP(), nil)
				s.store.On("DeleteIdentityProvider", mock.Anything, testAuditIDPID).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the connection is declared in a file",
			arrange: func() {
				s.store.On("GetIdentityProvider", mock.Anything, testAuditIDPID).Return(auditedIDP(), nil)
				s.store.On("DeleteIdentityProvider", mock.Anything, testAuditIDPID).Return(ErrIDPIsImmutable)
			},
			outcome: audit.OutcomeFailed("idp_1010"),
		},
		{
			name: "the store fails",
			arrange: func() {
				s.store.On("GetIdentityProvider", mock.Anything, testAuditIDPID).
					Return((*providers.IDPDTO)(nil), errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			service := s.newService(bus)
			tc.arrange()

			svcErr := service.DeleteIdentityProvider(adminContext(), testAuditIDPID)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			s.requireOnlyEvent(published(), audit.EventConnectionDelete, audit.ActionDelete, testAuditIDPID,
				tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no identity provider records nothing, so a caller cannot
// write a succeeded delete for any ID it chooses. The store reports a missing identity provider with
// ErrIDPNotFound, as the database and file stores do.
func (s *IDPServiceAuditTestSuite) TestDeleteIdentityProvider_PublishesNothingForAMissingConnection() {
	s.store = newIdpStoreInterfaceMock(s.T())
	bus, published := newRecordingBus(s.T())
	service := s.newService(bus)
	s.store.On("GetIdentityProvider", mock.Anything, testAuditIDPID).Return((*providers.IDPDTO)(nil), ErrIDPNotFound)

	svcErr := service.DeleteIdentityProvider(adminContext(), testAuditIDPID)

	s.Nil(svcErr)
	s.Empty(published())
	s.store.AssertNotCalled(s.T(), "DeleteIdentityProvider", mock.Anything, mock.Anything)
}

// A bus that cannot take the event never fails the change it describes.
func (s *IDPServiceAuditTestSuite) TestCreateIdentityProvider_SucceedsWhenTheBusRefusesTheEvent() {
	s.store = newIdpStoreInterfaceMock(s.T())
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	service := s.newService(failing)
	s.store.On("GetIdentityProviderByName", mock.Anything, testAuditIDPName).
		Return((*providers.IDPDTO)(nil), ErrIDPNotFound)
	s.store.On("CreateIdentityProvider", mock.Anything, mock.Anything).Return(nil)

	created, svcErr := service.CreateIdentityProvider(adminContext(), auditedIDP())

	s.Nil(svcErr)
	s.NotEmpty(created.ID)
}

func (s *IDPServiceAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	service, err := Initialize(cache.Initialize(config.GetServerRuntime().Config.Cache, "test-deployment"),
		nil, nil, nil, nil, nil, nil)

	s.ErrorIs(err, errNilEventBus)
	s.Nil(service)
}

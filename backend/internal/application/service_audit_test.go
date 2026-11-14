// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/entity"
	"github.com/thunder-id/thunderid/internal/inboundclient"
	"github.com/thunder-id/thunderid/internal/sharing"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/entitymock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/sharingmock"
	"github.com/thunder-id/thunderid/tests/mocks/sysauthzmock"
)

const (
	testAuditAdminID     = "admin-1"
	testAuditSecretValue = "audit-test-client-secret-0123456789"
	testAuditRedirectURI = "https://audit.example.com/callback"
)

// publishedEvent is one audit event the application service published.
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
	require.NoError(t, audit.Subscribe(registry, "application-service-test", sink))
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
		security.NewSecurityContextForTest(testAuditAdminID, testOUID, "", nil, nil))
}

// confidentialApplication is an application whose OAuth client authenticates with a client secret.
func confidentialApplication() *model.ApplicationDTO {
	return &model.ApplicationDTO{
		Type: model.ApplicationTypeFullStack,
		Name: "Audited App",
		OUID: testOUID,
		InboundAuthProfile: providers.InboundAuthProfile{
			AuthFlowID: "auth-flow-id",
		},
		InboundAuthConfig: []providers.InboundAuthConfigWithSecret{{
			Type: providers.OAuthInboundAuthType,
			OAuthConfig: &providers.OAuthConfigWithSecret{
				ClientID:                testClientID,
				ClientSecret:            testAuditSecretValue,
				RedirectURIs:            []string{testAuditRedirectURI},
				GrantTypes:              []providers.GrantType{providers.GrantTypeAuthorizationCode},
				ResponseTypes:           []providers.ResponseType{providers.ResponseTypeCode},
				TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic,
			},
		}},
	}
}

// requireOnlyEvent checks that exactly one event was published, named name, for an attempt by the admin
// on the application with appID, and returns it. An attempt that succeeded or was denied has the
// application as subject too; a failed one has no subject.
func (suite *ServiceTestSuite) requireOnlyEvent(events []publishedEvent, name string, action audit.Action,
	appID string, outcome audit.Outcome) publishedEvent {
	suite.T().Helper()
	suite.Require().Len(events, 1)
	event := events[0]
	application := audit.Ref{Type: audit.TargetTypeApplication, ID: appID}
	suite.Equal(name, event.name)
	suite.Equal(application, event.payload.Target())
	suite.Equal(action, event.payload.Action())
	suite.Equal(outcome, event.payload.Outcome())
	suite.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	suite.Empty(event.payload.Details(), "no attribute of the application is recorded")
	if outcome.Result == audit.ResultFailed || appID == "" {
		suite.Equal(audit.Ref{}, event.payload.Subject(), "a failed attempt may name no application")
	} else {
		suite.Equal(application, event.payload.Subject())
	}
	return event
}

// requireNoSecret checks that no part of the event carries the client secret or the redirect URI.
func (suite *ServiceTestSuite) requireNoSecret(event publishedEvent) {
	suite.T().Helper()
	recorded := fmt.Sprintf("%+v", event.payload)
	suite.NotContains(recorded, testAuditSecretValue, "a client secret is never recorded")
	suite.NotContains(recorded, testAuditRedirectURI, "no OAuth configuration is recorded")
}

func (suite *ServiceTestSuite) TestCreateApplication_PublishesTheCreatedApplication() {
	suite.withMutableRuntime()
	bus, published := newRecordingBus(suite.T())
	service, mockStore := suite.setupTestService()
	service.bus = bus
	mockStore.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	created, svcErr := service.CreateApplication(adminContext(), confidentialApplication())

	suite.Require().Nil(svcErr)
	suite.Require().NotEmpty(created.ID)
	event := suite.requireOnlyEvent(published(), audit.EventApplicationCreate, audit.ActionCreate, created.ID,
		audit.OutcomeSucceeded)
	suite.requireNoSecret(event)
}

func (suite *ServiceTestSuite) TestCreateApplication_PublishesTheSameNameWhenItFails() {
	cases := []struct {
		name    string
		app     func() *model.ApplicationDTO
		arrange func(service *applicationService)
		outcome audit.Outcome
	}{
		{
			name: "the request has no name",
			app: func() *model.ApplicationDTO {
				app := confidentialApplication()
				app.Name = ""
				return app
			},
			outcome: audit.OutcomeFailed("app_1004"),
		},
		{
			name: "the store fails",
			app:  confidentialApplication,
			arrange: func(service *applicationService) {
				resetEntityServiceMethod(service, "CreateEntity").On("CreateEntity",
					mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return((*providers.Entity)(nil), errors.New("store failure")).Once()
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.withMutableRuntime()
			bus, published := newRecordingBus(suite.T())
			service, _ := suite.setupTestService()
			service.bus = bus
			if tc.arrange != nil {
				tc.arrange(service)
			}

			created, svcErr := service.CreateApplication(adminContext(), tc.app())

			suite.Nil(created)
			suite.Require().NotNil(svcErr)
			event := suite.requireOnlyEvent(published(), audit.EventApplicationCreate, audit.ActionCreate, "",
				tc.outcome)
			suite.requireNoSecret(event)
		})
	}
}

// An update that rotates the client secret is recorded as an update, without the secret.
func (suite *ServiceTestSuite) TestUpdateApplication_PublishesTheUpdatedApplication() {
	f := suite.newSecretUpdateFixture(testAuditSecretValue)
	f.storedSecret(false)
	recordSecretRotation(f.entities, nil, nil, &f.order)
	bus, published := newRecordingBus(suite.T())
	f.service.bus = bus

	_, svcErr := f.service.UpdateApplication(adminContext(), testServiceAppID, f.update)

	suite.Require().Nil(svcErr)
	event := suite.requireOnlyEvent(published(), audit.EventApplicationUpdate, audit.ActionUpdate,
		testServiceAppID, audit.OutcomeSucceeded)
	suite.requireNoSecret(event)
}

// The application service has no access check of its own, so an update is denied only when a step it
// runs, here the entity service's secret rotation, refuses the caller. The rotation runs once the
// application is found, so the denied event has it as subject.
func (suite *ServiceTestSuite) TestUpdateApplication_PublishesARefusalAsDenied() {
	f := suite.newSecretUpdateFixture(testAuditSecretValue)
	f.storedSecret(false)
	recordSecretRotation(f.entities, &tidcommon.ErrorUnauthorized, nil, &f.order)
	bus, published := newRecordingBus(suite.T())
	f.service.bus = bus

	_, svcErr := f.service.UpdateApplication(adminContext(), testServiceAppID, f.update)

	suite.Equal(&tidcommon.ErrorUnauthorized, svcErr)
	event := suite.requireOnlyEvent(published(), audit.EventApplicationUpdate, audit.ActionUpdate,
		testServiceAppID, audit.OutcomeDenied("forbidden"))
	suite.requireNoSecret(event)
}

func (suite *ServiceTestSuite) TestUpdateApplication_PublishesAFailureForAMissingApplication() {
	suite.withMutableRuntime()
	bus, published := newRecordingBus(suite.T())
	service, mockStore := suite.setupTestService()
	service.bus = bus
	mockStore.On("GetInboundClientByEntityID", mock.Anything, testServiceAppID).
		Return(nil, inboundclient.ErrInboundClientNotFound)

	_, svcErr := service.UpdateApplication(adminContext(), testServiceAppID,
		confidentialApplication())

	suite.Equal(&ErrorApplicationNotFound, svcErr)
	suite.requireOnlyEvent(published(), audit.EventApplicationUpdate, audit.ActionUpdate, testServiceAppID,
		audit.OutcomeFailed("app_1001"))
}

func (suite *ServiceTestSuite) TestDeleteApplication_PublishesOneEventPerAttempt() {
	existing := func(service *applicationService) {
		resetEntityServiceMethod(service, "GetEntityProfile").On("GetEntityProfile", mock.Anything,
			testServiceAppID).Return(&providers.Entity{ID: testServiceAppID, Category: providers.EntityCategoryApp},
			nil)
	}
	cases := []struct {
		name    string
		arrange func(service *applicationService, store *inboundclientmock.InboundClientServiceInterfaceMock)
		outcome audit.Outcome
	}{
		{
			name: "the application is deleted",
			arrange: func(service *applicationService, store *inboundclientmock.InboundClientServiceInterfaceMock) {
				existing(service)
				store.On("DeleteInboundClient", mock.Anything, testServiceAppID).Return(nil)
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name: "what the client issued cannot be revoked",
			arrange: func(service *applicationService, store *inboundclientmock.InboundClientServiceInterfaceMock) {
				existing(service)
				store.On("DeleteInboundClient", mock.Anything, testServiceAppID).
					Return(inboundclient.ErrRevocationFailed)
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name: "the application cannot be read",
			arrange: func(service *applicationService, _ *inboundclientmock.InboundClientServiceInterfaceMock) {
				resetEntityServiceMethod(service, "GetEntityProfile").On("GetEntityProfile", mock.Anything,
					testServiceAppID).Return((*providers.Entity)(nil), errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
		{
			name: "what is left of a missing application cannot be removed",
			arrange: func(_ *applicationService, store *inboundclientmock.InboundClientServiceInterfaceMock) {
				store.On("DeleteInboundClient", mock.Anything, testServiceAppID).Return(errors.New("store failure"))
			},
			outcome: audit.OutcomeFailed("sse_5000"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.withMutableRuntime()
			bus, published := newRecordingBus(suite.T())
			service, mockStore := suite.setupTestService()
			service.bus = bus
			tc.arrange(service, mockStore)

			svcErr := service.DeleteApplication(adminContext(), testServiceAppID)

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlyEvent(published(), audit.EventApplicationDelete, audit.ActionDelete, testServiceAppID,
				tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no application records nothing, so a caller cannot
// write a succeeded delete for any ID it chooses. The inbound client store deletes without reporting
// whether a row matched, so its delete returns nil here as it does for a real missing ID.
func (suite *ServiceTestSuite) TestDeleteApplication_PublishesNothingForAMissingApplication() {
	suite.withMutableRuntime()
	bus, published := newRecordingBus(suite.T())
	service, mockStore := suite.setupTestService()
	service.bus = bus
	mockStore.On("DeleteInboundClient", mock.Anything, testServiceAppID).Return(nil)

	svcErr := service.DeleteApplication(adminContext(), testServiceAppID)

	suite.Nil(svcErr)
	suite.Empty(published())
	mockStore.AssertCalled(suite.T(), "DeleteInboundClient", mock.Anything, testServiceAppID)
}

// A bus that cannot take the event never fails the change it describes.
func (suite *ServiceTestSuite) TestCreateApplication_SucceedsWhenTheBusRefusesTheEvent() {
	suite.withMutableRuntime()
	failing := eventbusmock.NewEventBusInterfaceMock(suite.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	service, mockStore := suite.setupTestService()
	service.bus = failing
	mockStore.On("CreateInboundClient", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	created, svcErr := service.CreateApplication(adminContext(), confidentialApplication())

	suite.Nil(svcErr)
	suite.NotEmpty(created.ID)
}

// regenerationStage is how far a secret regeneration gets, and so which calls the entity layer and the
// inbound client store expect.
type regenerationStage int

const (
	// regenerationNotLoaded is a regeneration that reads the application and stops.
	regenerationNotLoaded regenerationStage = iota
	// regenerationNotStored is a regeneration that checks the client and stops before it writes.
	regenerationNotStored
	// regenerationStored is a regeneration that writes the secret.
	regenerationStored
)

// regeneratingService returns a service whose entity layer holds existing as testServiceAppID, whose OAuth
// client authenticates with authMethod, and whose secret write ends with persistErr. It expects exactly the
// calls a regeneration makes up to stage, so a write that should not happen fails the test.
func (suite *ServiceTestSuite) regeneratingService(existing *providers.Entity, entityErr error,
	authMethod providers.TokenEndpointAuthMethod, persistErr error, stage regenerationStage) *applicationService {
	mockStore := inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	mockEntityService := entitymock.NewEntityServiceInterfaceMock(suite.T())
	if stage >= regenerationNotLoaded {
		mockEntityService.On("GetEntityProfile", mock.Anything, testServiceAppID).Return(existing, entityErr).Once()
	}
	if stage >= regenerationNotStored {
		mockEntityService.On("IsEntityDeclarative", mock.Anything, testServiceAppID).Return(false, nil).Once()
		mockStore.On("GetOAuthClientByClientID", mock.Anything, "secret-client").
			Return(&providers.OAuthClient{TokenEndpointAuthMethod: authMethod}, nil).Once()
	}
	if stage >= regenerationStored {
		mockEntityService.On("UpdateSystemCredentials", mock.Anything, testServiceAppID, mock.Anything).
			Return(persistErr).Once()
	}
	service := &applicationService{
		logger:               log.GetLogger().With(log.String(log.LoggerKeyComponentName, "ApplicationService")),
		inboundClientService: mockStore,
		entityService:        mockEntityService,
		dependencyRegistry:   noopDepRegistry{},
		authzService:         allowAllAuthz(suite.T()),
		bus:                  discardingBus{},
	}
	regeneratesThroughTheHolder(service, mockEntityService)
	return service
}

// testRegeneratedSecret is the secret the mocked entity layer generates.
const testRegeneratedSecret = "regenerated-client-secret-0123456789"

// regeneratesThroughTheHolder has the mocked entity layer regenerate a secret as the entity service does: it
// asks the application service, the holder of applications, whether the secret may be regenerated, then
// stores a new one. The system flows the entity service runs around the write are tested with it.
func regeneratesThroughTheHolder(service *applicationService, entities *entitymock.EntityServiceInterfaceMock) {
	entities.EXPECT().RegenerateClientSecret(mock.Anything, providers.EntityCategoryApp, mock.Anything).
		RunAndReturn(func(ctx context.Context, _ providers.EntityCategory, appID string) (
			*entity.ClientSecretRegeneration, *tidcommon.ServiceError) {
			if svcErr := service.ValidateClientSecretRegeneration(ctx, appID); svcErr != nil {
				return nil, svcErr
			}
			if err := entities.UpdateSystemCredentials(ctx, appID,
				[]byte(`{"clientSecret":"`+testRegeneratedSecret+`"}`)); err != nil {
				return nil, &tidcommon.InternalServerError
			}
			return &entity.ClientSecretRegeneration{ClientSecret: testRegeneratedSecret}, nil
		}).Maybe()
}

// Regenerating a client secret records an update of the application, never the secret it generated.
func (suite *ServiceTestSuite) TestRegenerateClientSecret_PublishesOneEventPerAttempt() {
	confidential := appEntityWithClientID("secret-client")
	basic := providers.TokenEndpointAuthMethodClientSecretBasic
	cases := []struct {
		name       string
		appID      string
		existing   *providers.Entity
		entityErr  error
		authMethod providers.TokenEndpointAuthMethod
		persistErr error
		stage      regenerationStage
		outcome    audit.Outcome
	}{
		{name: "the secret is regenerated", appID: testServiceAppID, existing: confidential,
			authMethod: basic, stage: regenerationStored, outcome: audit.OutcomeSucceeded},
		{name: "the application does not exist", appID: testServiceAppID, entityErr: entity.ErrEntityNotFound,
			stage: regenerationNotLoaded, outcome: audit.OutcomeFailed("app_1001")},
		{name: "the client authenticates without a secret", appID: testServiceAppID, existing: confidential,
			authMethod: providers.TokenEndpointAuthMethodPrivateKeyJWT, stage: regenerationNotStored,
			outcome: audit.OutcomeFailed("app_1047")},
		{name: "the new secret cannot be stored", appID: testServiceAppID, existing: confidential,
			authMethod: basic, persistErr: errors.New("database is unavailable"), stage: regenerationStored,
			outcome: audit.OutcomeFailed("sse_5000")},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			service := suite.regeneratingService(tc.existing, tc.entityErr, tc.authMethod, tc.persistErr,
				tc.stage)
			bus, published := newRecordingBus(suite.T())
			service.bus = bus

			secret, svcErr := regeneratedSecret(service.RegenerateClientSecret(adminContext(), tc.appID))

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			event := suite.requireOnlyEvent(published(), audit.EventApplicationUpdate, audit.ActionUpdate,
				tc.appID, tc.outcome)
			if secret != "" {
				suite.NotContains(fmt.Sprintf("%+v", event.payload), secret, "the new secret is never recorded")
			}
		})
	}
}

// A bus that cannot take the event never fails the regeneration it describes, nor withholds its secret.
func (suite *ServiceTestSuite) TestRegenerateClientSecret_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(suite.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	service := suite.regeneratingService(appEntityWithClientID("secret-client"), nil,
		providers.TokenEndpointAuthMethodClientSecretPost, nil, regenerationStored)
	service.bus = failing

	secret, svcErr := regeneratedSecret(service.RegenerateClientSecret(adminContext(), testServiceAppID))

	suite.Nil(svcErr)
	suite.NotEmpty(secret)
}

// requireOnlySharingPolicyEvent checks that exactly one event was published, named name, for an attempt
// by the admin on the policy with policyID of the application. A succeeded attempt has the application as
// subject; a failed one has none.
func (suite *ServiceTestSuite) requireOnlySharingPolicyEvent(events []publishedEvent, name string,
	action audit.Action, policyID string, outcome audit.Outcome) {
	suite.T().Helper()
	suite.Require().Len(events, 1)
	event := events[0]
	suite.Equal(name, event.name)
	suite.Equal(audit.Ref{Type: audit.TargetTypeApplicationSharingPolicy, ID: policyID}, event.payload.Target())
	suite.Equal(action, event.payload.Action())
	suite.Equal(outcome, event.payload.Outcome())
	suite.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditAdminID}, event.payload.Actor())
	suite.Empty(event.payload.Details())
	if outcome.Result == audit.ResultFailed {
		suite.Equal(audit.Ref{}, event.payload.Subject())
	} else {
		suite.Equal(audit.Ref{Type: audit.TargetTypeApplication, ID: testServiceAppID}, event.payload.Subject())
	}
	suite.NotContains(fmt.Sprintf("%+v", event.payload), "owner-ou", "no organization unit a policy names is recorded")
}

// ownPolicy is a policy of the application under test.
func ownPolicy() sharing.Policy {
	return sharing.Policy{ID: "p1", ResourceType: ApplicationSharingType, ResourceID: testServiceAppID}
}

func (suite *ServiceTestSuite) TestCreateSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name     string
		arrange  func(*sharingmock.SharingServiceInterfaceMock)
		policyID string
		outcome  audit.Outcome
	}{
		{
			name: "the policy is created",
			arrange: func(sharingMock *sharingmock.SharingServiceInterfaceMock) {
				sharingMock.EXPECT().CreatePolicy(mock.Anything, ApplicationSharingType, testServiceAppID,
					"owner-ou", mock.Anything).Return(ownPolicy(), nil).Once()
			},
			policyID: "p1", outcome: audit.OutcomeSucceeded,
		},
		{
			name: "the framework refuses a second policy for the unit",
			arrange: func(sharingMock *sharingmock.SharingServiceInterfaceMock) {
				sharingMock.EXPECT().CreatePolicy(mock.Anything, ApplicationSharingType, testServiceAppID,
					"owner-ou", mock.Anything).Return(sharing.Policy{}, &sharing.ErrorPolicyExists).Once()
			},
			outcome: audit.OutcomeFailed("app_1053"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			service, sharingMock := suite.sharingTestService()
			bus, published := newRecordingBus(suite.T())
			service.bus = bus
			tc.arrange(sharingMock)

			_, svcErr := service.CreateSharingPolicy(adminContext(), testServiceAppID, sharing.PolicyRequest{})

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlySharingPolicyEvent(published(), audit.EventApplicationSharingPolicyCreate,
				audit.ActionCreate, tc.policyID, tc.outcome)
		})
	}
}

func (suite *ServiceTestSuite) TestUpdateSharingPolicy_PublishesOneEventPerAttempt() {
	cases := []struct {
		name    string
		policy  sharing.Policy
		arrange func(*sharingmock.SharingServiceInterfaceMock)
		outcome audit.Outcome
	}{
		{
			name:   "the policy is updated",
			policy: ownPolicy(),
			arrange: func(sharingMock *sharingmock.SharingServiceInterfaceMock) {
				sharingMock.EXPECT().UpdatePolicy(mock.Anything, "p1", mock.Anything).Return(ownPolicy(), nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "the policy belongs to another application",
			policy:  sharing.Policy{ID: "p1", ResourceType: ApplicationSharingType, ResourceID: "other"},
			arrange: func(*sharingmock.SharingServiceInterfaceMock) {},
			outcome: audit.OutcomeFailed("app_1052"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			service, sharingMock := suite.sharingTestService()
			bus, published := newRecordingBus(suite.T())
			service.bus = bus
			sharingMock.EXPECT().GetPolicy(mock.Anything, "p1").Return(tc.policy, nil).Once()
			tc.arrange(sharingMock)

			_, svcErr := service.UpdateSharingPolicy(adminContext(), testServiceAppID, "p1", sharing.PolicyRequest{})

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlySharingPolicyEvent(published(), audit.EventApplicationSharingPolicyUpdate,
				audit.ActionUpdate, "p1", tc.outcome)
		})
	}
}

func (suite *ServiceTestSuite) TestDeleteSharingPolicy_PublishesOneEventPerAttempt() {
	declared := ownPolicy()
	declared.Declared = true
	cases := []struct {
		name    string
		policy  sharing.Policy
		arrange func(*sharingmock.SharingServiceInterfaceMock)
		outcome audit.Outcome
	}{
		{
			name:   "the policy is deleted",
			policy: ownPolicy(),
			arrange: func(sharingMock *sharingmock.SharingServiceInterfaceMock) {
				sharingMock.EXPECT().DeletePolicy(mock.Anything, "p1").Return(nil).Once()
			},
			outcome: audit.OutcomeSucceeded,
		},
		{
			name:    "a declared policy",
			policy:  declared,
			arrange: func(*sharingmock.SharingServiceInterfaceMock) {},
			outcome: audit.OutcomeFailed("app_1055"),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			service, sharingMock := suite.sharingTestService()
			bus, published := newRecordingBus(suite.T())
			service.bus = bus
			sharingMock.EXPECT().GetPolicy(mock.Anything, "p1").Return(tc.policy, nil).Once()
			tc.arrange(sharingMock)

			svcErr := service.DeleteSharingPolicy(adminContext(), testServiceAppID, "p1")

			suite.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			suite.requireOnlySharingPolicyEvent(published(), audit.EventApplicationSharingPolicyDelete,
				audit.ActionDelete, "p1", tc.outcome)
		})
	}
}

// A delete stays idempotent, but one that finds no policy of the application records nothing, so a caller
// cannot write a succeeded delete for any ID it chooses.
func (suite *ServiceTestSuite) TestDeleteSharingPolicy_PublishesNothingForAMissingPolicy() {
	for _, tc := range []struct {
		name   string
		policy sharing.Policy
		err    *tidcommon.ServiceError
	}{
		{name: "no policy has the ID", err: &sharing.ErrorPolicyNotFound},
		{name: "the policy belongs to another application",
			policy: sharing.Policy{ID: "p1", ResourceType: ApplicationSharingType, ResourceID: "other"}},
	} {
		suite.Run(tc.name, func() {
			service, sharingMock := suite.sharingTestService()
			bus, published := newRecordingBus(suite.T())
			service.bus = bus
			sharingMock.EXPECT().GetPolicy(mock.Anything, "p1").Return(tc.policy, tc.err).Once()

			suite.Nil(service.DeleteSharingPolicy(adminContext(), testServiceAppID, "p1"))
			suite.Empty(published())
		})
	}
}

func (suite *ServiceTestSuite) TestRegenerateClientSecret_RecordsAnAccessDenialWithTheApplicationAsSubject() {
	service := suite.regeneratingService(appEntityWithClientID("secret-client"), nil,
		providers.TokenEndpointAuthMethodClientSecretBasic, nil, regenerationNotLoaded)
	denying := sysauthzmock.NewSystemAuthorizationServiceInterfaceMock(suite.T())
	denying.On("IsActionAllowed", mock.Anything, mock.Anything, mock.Anything).Return(false, nil)
	service.authzService = denying
	bus, published := newRecordingBus(suite.T())
	service.bus = bus

	secret, svcErr := regeneratedSecret(service.RegenerateClientSecret(adminContext(), testServiceAppID))

	suite.Empty(secret)
	suite.NotNil(svcErr)
	suite.requireOnlyEvent(published(), audit.EventApplicationUpdate, audit.ActionUpdate, testServiceAppID,
		audit.OutcomeDenied("forbidden"))
}

// regeneratedSecret reads the secret out of a regeneration's result, empty when it failed.
func regeneratedSecret(regenerated *model.ClientSecretResponse, svcErr *tidcommon.ServiceError) (
	string, *tidcommon.ServiceError) {
	if regenerated == nil {
		return "", svcErr
	}
	return regenerated.ClientSecret, svcErr
}

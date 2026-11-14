// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package manager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/providermock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
)

const (
	testAuditUserID       = "0196a1b2-0000-7000-8000-0000000000a1"
	testAuditCredentialID = "audit-passkey-credential-id-0123"
	testAuditAttestation  = "audit-passkey-attestation-object-4567"
	testAuditSessionToken = "audit-passkey-session-token-89ab"
	testAuditConnectionID = "idp-audit-connection-1"
	testAuditExternalSub  = "external-subject-123"
)

// publishedEvent is one audit event the manager published.
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
	if err := audit.Subscribe(registry, "authn-provider-manager-test", sink); err != nil {
		t.Fatal(err)
	}
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// newDiscardingBus returns an event bus with no subscriber, which drops every event, for a test the audit
// event does not matter to. A test of what the manager publishes uses newRecordingBus.
func newDiscardingBus(t testing.TB) eventbus.EventBusInterface {
	bus, _, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)
	return bus
}

// ManagerAuditTestSuite checks the audit event each attempt to enroll a credential or link an account publishes.
type ManagerAuditTestSuite struct {
	suite.Suite
	provider *providermock.AuthnProviderInterfaceMock
}

func TestManagerAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ManagerAuditTestSuite))
}

func (s *ManagerAuditTestSuite) SetupTest() {
	s.provider = providermock.NewAuthnProviderInterfaceMock(s.T())
}

func (s *ManagerAuditTestSuite) SetupSubTest() {
	s.provider = providermock.NewAuthnProviderInterfaceMock(s.T())
}

// manager returns a manager over the suite's provider that publishes on bus.
func (s *ManagerAuditTestSuite) manager(bus eventbus.EventBusInterface) providers.AuthnProviderManager {
	mgr, err := Initialize(s.provider, nil, bus)
	s.Require().NoError(err)
	return mgr
}

// passkeyCredentials is the credential a passkey registration finish hands the manager.
func passkeyCredentials() map[string]interface{} {
	return map[string]interface{}{authnprovidercm.CredentialTypePasskey: &passkey.PasskeyRegistrationFinishRequest{
		CredentialID:      testAuditCredentialID,
		CredentialType:    "public-key",
		ClientDataJSON:    "eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIn0",
		AttestationObject: testAuditAttestation,
		SessionToken:      testAuditSessionToken,
	}}
}

// enrolledUser is the result the default provider returns for a passkey stored for testAuditUserID.
func enrolledUser() *providers.AuthnResult {
	return &providers.AuthnResult{
		EntityReference: &providers.EntityReference{
			EntityID: testAuditUserID, EntityCategory: string(providers.EntityCategoryUser), EntityType: "customer",
		},
		Attributes:          &providers.AttributesResponse{},
		AuthenticatedClaims: providers.AuthenticatedClaims{authnprovidercm.UserAttributeUserID: testAuditUserID},
	}
}

// linkedUser is an AuthUser whose default provider state names testAuditUserID by reference.
func linkedUser() providers.AuthUser {
	return authUserWithDefaultState(providers.AuthState{
		EntityReference: &providers.EntityReference{EntityID: testAuditUserID,
			EntityCategory: string(providers.EntityCategoryUser), EntityType: "customer"},
		Attributes: &providers.AttributesResponse{},
	})
}

// expectedEvent is what requireOnlyEvent expects of the one event a manager call published.
type expectedEvent struct {
	name    string
	action  audit.Action
	userID  string
	outcome audit.Outcome
	actor   audit.Actor
	details map[string]string
	// absent lists values that must be recorded nowhere in the payload.
	absent []string
}

// enrollEvent is the event an enrollment is expected to publish; no passkey data may be recorded in it.
func enrollEvent(userID string, outcome audit.Outcome, actor audit.Actor) expectedEvent {
	return expectedEvent{name: audit.EventUserCredentialEnroll, action: audit.ActionEnroll, userID: userID,
		outcome: outcome, actor: actor,
		absent: []string{testAuditCredentialID, testAuditAttestation, testAuditSessionToken}}
}

// linkEvent is the event a link is expected to publish; the sub of the external identity may not be
// recorded in it.
func linkEvent(userID string, outcome audit.Outcome, actor audit.Actor,
	details map[string]string) expectedEvent {
	return expectedEvent{name: audit.EventUserLinkedAccountCreate, action: audit.ActionCreate, userID: userID,
		outcome: outcome, actor: actor, details: details, absent: []string{testAuditExternalSub}}
}

// requireOnlyEvent checks that exactly one event was published, about the user with userID, and that
// none of the absent values is recorded. A succeeded attempt that names a user has it as subject.
func (s *ManagerAuditTestSuite) requireOnlyEvent(events []publishedEvent, want expectedEvent) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	user := audit.Ref{Type: audit.TargetTypeUser, ID: want.userID}
	s.Equal(want.name, event.name)
	s.Equal(user, event.payload.Target())
	s.Equal(want.action, event.payload.Action())
	s.Equal(want.outcome, event.payload.Outcome())
	if want.details == nil {
		s.Empty(event.payload.Details())
	} else {
		s.Equal(want.details, event.payload.Details())
	}
	if want.outcome.Result == audit.ResultSucceeded && want.userID != "" {
		s.Equal(user, event.payload.Subject())
	} else {
		s.Equal(audit.Ref{}, event.payload.Subject())
	}
	s.Equal(want.actor, event.payload.Actor())
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range want.absent {
		s.NotContains(recorded, value, "must not be recorded")
	}
}

func (s *ManagerAuditTestSuite) TestEnroll_PublishesOneEventPerAttempt() {
	clientError := func(code string) *tidcommon.ServiceError {
		return &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: code}
	}
	cases := []struct {
		name    string
		result  *providers.AuthnResult
		err     *tidcommon.ServiceError
		userID  string
		outcome audit.Outcome
	}{
		{name: "the passkey is stored", result: enrolledUser(), userID: testAuditUserID,
			outcome: audit.OutcomeSucceeded},
		{name: "the attestation is refused", err: clientError(authnprovidercm.ErrorCodeEnrollmentFailed),
			outcome: audit.OutcomeFailed("authn_mgr_1002")},
		{name: "the enrollment payload is invalid", err: clientError(authnprovidercm.ErrorCodeInvalidRequest),
			outcome: audit.OutcomeFailed("authn_mgr_1008")},
		{name: "the user is suspended", err: clientError(authnprovidercm.ErrorCodeAccountSuspended),
			outcome: audit.OutcomeDenied("account_suspended")},
		{name: "the provider fails", err: &tidcommon.InternalServerError,
			outcome: audit.OutcomeFailed("sse_5000")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			credentials := passkeyCredentials()
			s.provider.On("Enroll", mock.Anything, map[string]interface{}(nil), credentials,
				(*providers.AuthnMetadata)(nil)).Return(tc.result, tc.err).Once()

			_, _, svcErr := s.manager(bus).Enroll(context.Background(), nil, credentials, nil, nil,
				providers.AuthUser{})

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			actor := audit.Actor{Type: audit.ActorTypeAnonymous}
			if tc.userID != "" {
				actor = audit.Actor{Type: audit.ActorTypeUser, ID: tc.userID}
			}
			s.requireOnlyEvent(published(), enrollEvent(tc.userID, tc.outcome, actor))
		})
	}
}

// The credential is already stored when the subject check refuses the user, so the enrollment succeeded.
func (s *ManagerAuditTestSuite) TestEnroll_RecordsTheStoredCredentialWhenTheSubjectIsRefused() {
	bus, published := newRecordingBus(s.T())
	ctx := authnprovidercm.WithSubjectTypeConstraints(context.Background(),
		authnprovidercm.SubjectTypeConstraints{RequiredSubjectID: "0196a1b2-0000-7000-8000-0000000000b2"})
	credentials := passkeyCredentials()
	s.provider.On("Enroll", mock.Anything, map[string]interface{}(nil), credentials,
		(*providers.AuthnMetadata)(nil)).Return(enrolledUser(), (*tidcommon.ServiceError)(nil)).Once()

	_, _, svcErr := s.manager(bus).Enroll(ctx, nil, credentials, nil, nil, providers.AuthUser{})

	s.Require().NotNil(svcErr)
	s.Equal(ErrorSubjectNotAllowed.Code, svcErr.Code)
	s.requireOnlyEvent(published(), enrollEvent(testAuditUserID, audit.OutcomeSucceeded,
		audit.Actor{Type: audit.ActorTypeUser, ID: testAuditUserID}))
}

// The enrollment endpoints are public, so a bearer token sent with the request never decides the actor.
func (s *ManagerAuditTestSuite) TestEnroll_IgnoresTheRequestsSecurityContextForTheActor() {
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("0196a1b2-0000-7000-8000-0000000000c3", "", "", nil, nil))
	cases := []struct {
		name   string
		result *providers.AuthnResult
		err    *tidcommon.ServiceError
		actor  audit.Actor
	}{
		{name: "the passkey is stored", result: enrolledUser(),
			actor: audit.Actor{Type: audit.ActorTypeUser, ID: testAuditUserID}},
		{name: "the attestation is refused", err: &tidcommon.ServiceError{Type: tidcommon.ClientErrorType,
			Code: authnprovidercm.ErrorCodeEnrollmentFailed}, actor: audit.Actor{Type: audit.ActorTypeAnonymous}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			credentials := passkeyCredentials()
			s.provider.On("Enroll", mock.Anything, map[string]interface{}(nil), credentials,
				(*providers.AuthnMetadata)(nil)).Return(tc.result, tc.err).Once()

			s.manager(bus).Enroll(ctx, nil, credentials, nil, nil, providers.AuthUser{})

			events := published()
			s.Require().Len(events, 1)
			s.Equal(tc.actor, events[0].payload.Actor())
			s.Empty(events[0].payload.OnBehalfOf())
		})
	}
}

// A stored credential that names no user is recorded as succeeded with no target ID and no subject.
func (s *ManagerAuditTestSuite) TestEnroll_RecordsAStoredCredentialThatNamesNoUser() {
	bus, published := newRecordingBus(s.T())
	credentials := passkeyCredentials()
	s.provider.On("Enroll", mock.Anything, map[string]interface{}(nil), credentials,
		(*providers.AuthnMetadata)(nil)).Return(&providers.AuthnResult{}, (*tidcommon.ServiceError)(nil)).Once()

	s.manager(bus).Enroll(context.Background(), nil, credentials, nil, nil, providers.AuthUser{})

	events := published()
	s.Require().Len(events, 1)
	payload := events[0].payload
	s.Equal(audit.Ref{Type: audit.TargetTypeUser}, payload.Target())
	s.Equal(audit.OutcomeSucceeded, payload.Outcome())
	s.Equal(audit.Ref{}, payload.Subject())
	s.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, payload.Actor())
}

// A bus that cannot take the event never fails the enrollment it describes.
func (s *ManagerAuditTestSuite) TestEnroll_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	credentials := passkeyCredentials()
	s.provider.On("Enroll", mock.Anything, map[string]interface{}(nil), credentials,
		(*providers.AuthnMetadata)(nil)).Return(enrolledUser(), (*tidcommon.ServiceError)(nil)).Once()

	authUser, _, svcErr := s.manager(failing).Enroll(context.Background(), nil, credentials, nil, nil,
		providers.AuthUser{})

	s.Nil(svcErr)
	s.True(authUser.IsAuthenticated())
}

func (s *ManagerAuditTestSuite) TestLinkAccount_PublishesOneEventPerAttempt() {
	cases := []struct {
		name          string
		authUser      providers.AuthUser
		idpID         string
		sub           string
		storeErr      *tidcommon.ServiceError
		callsProvider bool
		userID        string
		connection    string
		outcome       audit.Outcome
	}{
		{name: "the link is stored", authUser: linkedUser(), idpID: testAuditConnectionID,
			sub: testAuditExternalSub, callsProvider: true, userID: testAuditUserID, connection: testAuditConnectionID,
			outcome: audit.OutcomeSucceeded},
		{name: "the provider rejects the link", authUser: linkedUser(), idpID: testAuditConnectionID,
			sub: testAuditExternalSub, callsProvider: true, storeErr: &tidcommon.ServiceError{
				Type: tidcommon.ClientErrorType, Code: authnprovidercm.ErrorCodeInvalidRequest},
			userID: testAuditUserID, connection: testAuditConnectionID,
			outcome: audit.OutcomeFailed("authn_mgr_1012")},
		{name: "the provider fails", authUser: linkedUser(), idpID: testAuditConnectionID,
			sub: testAuditExternalSub, callsProvider: true, storeErr: &tidcommon.InternalServerError,
			userID: testAuditUserID, connection: testAuditConnectionID,
			outcome: audit.OutcomeFailed("sse_5000")},
		{name: "the connection is missing", authUser: linkedUser(), sub: testAuditExternalSub,
			outcome: audit.OutcomeFailed("authn_mgr_1008")},
		{name: "the subject is missing", authUser: linkedUser(), idpID: testAuditConnectionID,
			connection: testAuditConnectionID, outcome: audit.OutcomeFailed("authn_mgr_1008")},
		{name: "the user carries no provider state", authUser: providers.AuthUser{}, idpID: testAuditConnectionID,
			sub: testAuditExternalSub, connection: testAuditConnectionID,
			outcome: audit.OutcomeFailed("authn_mgr_1001")},
		{name: "the state names a provider that is not registered",
			authUser: authUserWithStates(map[string]providers.AuthState{"unregistered": {}}),
			idpID:    testAuditConnectionID, sub: testAuditExternalSub, connection: testAuditConnectionID,
			outcome: audit.OutcomeFailed("sse_5000")},
		{name: "the state names no entity",
			authUser: authUserWithDefaultState(providers.AuthState{}), idpID: testAuditConnectionID,
			sub: testAuditExternalSub, connection: testAuditConnectionID,
			outcome: audit.OutcomeFailed("authn_mgr_1008")},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			if tc.callsProvider {
				s.provider.On("StoreAccountLink", mock.Anything,
					map[string]interface{}{authnprovidercm.UserAttributeUserID: testAuditUserID},
					tc.idpID, tc.sub).Return(tc.storeErr).Once()
			}

			svcErr := s.manager(bus).LinkAccount(context.Background(), tc.authUser, tc.idpID, tc.sub)

			s.Equal(tc.outcome == audit.OutcomeSucceeded, svcErr == nil)
			actor := audit.Actor{Type: audit.ActorTypeAnonymous}
			if tc.userID != "" {
				actor = audit.Actor{Type: audit.ActorTypeUser, ID: tc.userID}
			}
			var details map[string]string
			if tc.connection != "" {
				details = map[string]string{audit.DetailConnection: tc.connection}
			}
			s.requireOnlyEvent(published(), linkEvent(tc.userID, tc.outcome, actor, details))
		})
	}
}

// A provider that answers not implemented, as the default provider does for a declarative user, records nothing.
func (s *ManagerAuditTestSuite) TestLinkAccount_PublishesNothingWhenTheProviderStoresNoLinks() {
	bus, published := newRecordingBus(s.T())
	s.provider.On("StoreAccountLink", mock.Anything,
		map[string]interface{}{authnprovidercm.UserAttributeUserID: testAuditUserID},
		testAuditConnectionID, testAuditExternalSub).Return(&tidcommon.ServiceError{
		Type: tidcommon.ClientErrorType, Code: authnprovidercm.ErrorCodeNotImplemented}).Once()

	svcErr := s.manager(bus).LinkAccount(context.Background(), linkedUser(), testAuditConnectionID,
		testAuditExternalSub)

	s.Nil(svcErr)
	s.Empty(published())
}

// The link is published on the server's authority, so the bearer token sent with the request never
// decides the actor, whether the link is stored or fails for a known user.
func (s *ManagerAuditTestSuite) TestLinkAccount_IgnoresTheRequestsSecurityContextForTheActor() {
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("0196a1b2-0000-7000-8000-0000000000c3", "", "", nil, nil))
	cases := []struct {
		name     string
		storeErr *tidcommon.ServiceError
		actor    audit.Actor
	}{
		{name: "the link is stored", actor: audit.Actor{Type: audit.ActorTypeUser, ID: testAuditUserID}},
		{name: "the provider rejects the link", storeErr: &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: authnprovidercm.ErrorCodeInvalidRequest},
			actor: audit.Actor{Type: audit.ActorTypeUser, ID: testAuditUserID}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			bus, published := newRecordingBus(s.T())
			s.provider.On("StoreAccountLink", mock.Anything,
				map[string]interface{}{authnprovidercm.UserAttributeUserID: testAuditUserID},
				testAuditConnectionID, testAuditExternalSub).Return(tc.storeErr).Once()

			s.manager(bus).LinkAccount(ctx, linkedUser(), testAuditConnectionID, testAuditExternalSub)

			events := published()
			s.Require().Len(events, 1)
			s.Equal(tc.actor, events[0].payload.Actor())
			s.Empty(events[0].payload.OnBehalfOf())
		})
	}
}

// A stored link whose state holds a token but no resolved reference names no user, so it is recorded
// as succeeded with no target ID, no subject and an anonymous actor.
func (s *ManagerAuditTestSuite) TestLinkAccount_RecordsAStoredLinkThatNamesNoUser() {
	bus, published := newRecordingBus(s.T())
	token := map[string]interface{}{authnprovidercm.UserAttributeUserID: testAuditUserID}
	s.provider.On("StoreAccountLink", mock.Anything, token, testAuditConnectionID, testAuditExternalSub).
		Return((*tidcommon.ServiceError)(nil)).Once()
	authUser := authUserWithDefaultState(providers.AuthState{EntityReferenceToken: token})

	svcErr := s.manager(bus).LinkAccount(context.Background(), authUser, testAuditConnectionID,
		testAuditExternalSub)

	s.Nil(svcErr)
	s.requireOnlyEvent(published(), linkEvent("", audit.OutcomeSucceeded,
		audit.Actor{Type: audit.ActorTypeAnonymous}, map[string]string{audit.DetailConnection: testAuditConnectionID}))
}

// A bus that cannot take the event never fails the link it describes.
func (s *ManagerAuditTestSuite) TestLinkAccount_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.provider.On("StoreAccountLink", mock.Anything,
		map[string]interface{}{authnprovidercm.UserAttributeUserID: testAuditUserID},
		testAuditConnectionID, testAuditExternalSub).Return((*tidcommon.ServiceError)(nil)).Once()

	svcErr := s.manager(failing).LinkAccount(context.Background(), linkedUser(), testAuditConnectionID,
		testAuditExternalSub)

	s.Nil(svcErr)
}

func (s *ManagerAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	mgr, err := Initialize(s.provider, nil, nil)

	s.Nil(mgr)
	s.ErrorIs(err, errNilEventBus)
}

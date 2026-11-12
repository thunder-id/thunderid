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

// ManagerAuditTestSuite checks the audit event each attempt to enroll a credential publishes.
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

// requireOnlyEvent checks that exactly one enrollment event was published, about the user with userID,
// and that nothing of the passkey is recorded. A succeeded attempt has the user as subject.
func (s *ManagerAuditTestSuite) requireOnlyEvent(events []publishedEvent, userID string, outcome audit.Outcome) {
	s.T().Helper()
	s.Require().Len(events, 1)
	event := events[0]
	user := audit.Ref{Type: audit.TargetTypeUser, ID: userID}
	s.Equal(audit.EventUserCredentialEnroll, event.name)
	s.Equal(user, event.payload.Target())
	s.Equal(audit.ActionEnroll, event.payload.Action())
	s.Equal(outcome, event.payload.Outcome())
	s.Empty(event.payload.Details())
	if outcome.Result == audit.ResultSucceeded {
		s.Equal(user, event.payload.Subject())
		s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: userID}, event.payload.Actor())
	} else {
		s.Equal(audit.Ref{}, event.payload.Subject())
		s.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, event.payload.Actor())
	}
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, value := range []string{testAuditCredentialID, testAuditAttestation, testAuditSessionToken} {
		s.NotContains(recorded, value, "no passkey data is recorded")
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
			s.requireOnlyEvent(published(), tc.userID, tc.outcome)
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
	s.requireOnlyEvent(published(), testAuditUserID, audit.OutcomeSucceeded)
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

func (s *ManagerAuditTestSuite) TestInitialize_RefusesANilEventBus() {
	mgr, err := Initialize(s.provider, nil, nil)

	s.Nil(mgr)
	s.ErrorIs(err, errNilEventBus)
}

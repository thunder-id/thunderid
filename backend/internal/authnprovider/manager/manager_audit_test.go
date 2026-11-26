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
	testAuditPassword     = "audit-typed-password-cdef"
	testAuditIdentifier   = "audit-typed-username-0123"
	testAuditOUID         = "0196a1b2-0000-7000-8000-0000000000d4"
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

// ManagerAuditTestSuite checks the audit event each attempt to enroll a credential, and each credential
// check, publishes.
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
	mgr, err := Initialize(s.provider, nil, bus, FederatedSyncDependencies{})
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
	mgr, err := Initialize(s.provider, nil, nil, FederatedSyncDependencies{})

	s.Nil(mgr)
	s.ErrorIs(err, errNilEventBus)
}

// passwordCredentials is what a sign-in with a password hands the manager.
func passwordCredentials() map[string]interface{} {
	return map[string]interface{}{"password": testAuditPassword}
}

// typedIdentifiers is the text the user typed to name the account.
func typedIdentifiers() map[string]interface{} {
	return map[string]interface{}{"username": testAuditIdentifier}
}

// authenticatedEntity is the result the default provider returns for a verified entity of category.
func authenticatedEntity(category providers.EntityCategory) *providers.AuthnResult {
	return &providers.AuthnResult{
		EntityReference: &providers.EntityReference{
			EntityID: testAuditUserID, EntityCategory: string(category), EntityType: "customer",
			OUID: testAuditOUID,
		},
		Attributes:          &providers.AttributesResponse{},
		AuthenticatedClaims: providers.AuthenticatedClaims{authnprovidercm.UserAttributeUserID: testAuditUserID},
	}
}

// clientErrorCode is a client error of a provider with code.
func clientErrorCode(code string) *tidcommon.ServiceError {
	return &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: code}
}

// authenticate runs one credential check through a manager over the suite's provider, which answers it with
// result and err, and returns what the check returned and the audit events it published.
func (s *ManagerAuditTestSuite) authenticate(ctx context.Context, credentials map[string]interface{},
	result *providers.AuthnResult, err *tidcommon.ServiceError) (*tidcommon.ServiceError, []publishedEvent) {
	bus, published := newRecordingBus(s.T())
	s.provider.On("Authenticate", mock.Anything, typedIdentifiers(), credentials,
		(*providers.AuthnMetadata)(nil)).Return(result, err).Once()

	_, _, svcErr := s.manager(bus).AuthenticateUser(ctx, typedIdentifiers(), credentials, nil, nil,
		providers.AuthUser{})

	return svcErr, published()
}

// requireOnlyAuthentication checks that exactly one credential check event was published, with method as its
// only detail, and that neither the typed identifier nor the password is recorded.
func (s *ManagerAuditTestSuite) requireOnlyAuthentication(events []publishedEvent, method string) audit.Payload {
	s.T().Helper()
	s.Require().Len(events, 1)
	s.Equal(audit.EventUserAuthenticate, events[0].name)
	payload := events[0].payload
	s.Equal(audit.ActionAuthenticate, payload.Action())
	s.Equal(map[string]string{audit.DetailAuthenticationMethod: method}, payload.Details())
	recorded := fmt.Sprintf("%+v", payload)
	for _, value := range []string{testAuditPassword, testAuditIdentifier} {
		s.NotContains(recorded, value, "nothing the user typed is recorded")
	}
	return payload
}

func (s *ManagerAuditTestSuite) TestAuthenticateUser_PublishesOneEventPerCheck() {
	cases := []struct {
		name        string
		credentials map[string]interface{}
		ctx         context.Context
		result      *providers.AuthnResult
		err         *tidcommon.ServiceError
		method      string
		outcome     audit.Outcome
		returned    string
	}{
		{name: "the password is right", result: authenticatedEntity(providers.EntityCategoryUser),
			outcome: audit.OutcomeSucceeded},
		{name: "the provider fails", err: &tidcommon.InternalServerError,
			outcome: audit.OutcomeFailed(audit.ReasonServerError), returned: tidcommon.InternalServerError.Code},
		{name: "the user is not found", err: clientErrorCode(authnprovidercm.ErrorCodeUserNotFound),
			outcome: audit.OutcomeFailed("user_not_found"), returned: ErrorUserNotFound.Code},
		{name: "the request is invalid", err: clientErrorCode(authnprovidercm.ErrorCodeInvalidRequest),
			outcome: audit.OutcomeFailed("invalid_request"), returned: ErrorInvalidRequest.Code},
		{name: "the identifier names several users", err: clientErrorCode(authnprovidercm.ErrorCodeAmbiguousUser),
			outcome: audit.OutcomeFailed("ambiguous_user"), returned: ErrorAuthenticationFailed.Code},
		{name: "the link of a federated identity names several users", method: "federated",
			credentials: map[string]interface{}{authnprovidercm.CredentialTypeFederated: "code"},
			err:         clientErrorCode(authnprovidercm.ErrorCodeAmbiguousUser),
			outcome:     audit.OutcomeFailed("ambiguous_user"), returned: ErrorAmbiguousUser.Code},
		{name: "the password is wrong", err: clientErrorCode(authnprovidercm.ErrorCodeAuthenticationFailed),
			outcome: audit.OutcomeFailed("invalid_credentials"), returned: ErrorAuthenticationFailed.Code},
		{name: "the account is locked", err: clientErrorCode(authnprovidercm.ErrorCodeAccountLocked),
			outcome: audit.OutcomeDenied("account_locked"), returned: ErrorAccountLocked.Code},
		{name: "the account is suspended", err: clientErrorCode(authnprovidercm.ErrorCodeAccountSuspended),
			outcome: audit.OutcomeDenied("account_suspended"), returned: ErrorAccountSuspended.Code},
		{name: "the application does not accept the subject",
			ctx: authnprovidercm.WithSubjectTypeConstraints(context.Background(),
				authnprovidercm.SubjectTypeConstraints{RequiredSubjectID: "0196a1b2-0000-7000-8000-0000000000b2"}),
			result:  authenticatedEntity(providers.EntityCategoryUser),
			outcome: audit.OutcomeDenied("subject_not_allowed"), returned: ErrorSubjectNotAllowed.Code},
		{name: "the provider result names no entity", result: &providers.AuthnResult{},
			outcome: audit.OutcomeFailed(audit.ReasonServerError), returned: tidcommon.InternalServerError.Code},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			ctx, credentials, method := tc.ctx, tc.credentials, tc.method
			if ctx == nil {
				ctx = context.Background()
			}
			if credentials == nil {
				credentials, method = passwordCredentials(), "credential"
			}

			svcErr, events := s.authenticate(ctx, credentials, tc.result, tc.err)

			if tc.returned == "" {
				s.Nil(svcErr)
			} else {
				s.Require().NotNil(svcErr)
				s.Equal(tc.returned, svcErr.Code, "the caller's error is unchanged")
			}
			payload := s.requireOnlyAuthentication(events, method)
			s.Equal(tc.outcome, payload.Outcome())
			if tc.outcome.Result == audit.ResultSucceeded {
				user := audit.Ref{Type: audit.TargetTypeUser, ID: testAuditUserID}
				s.Equal(audit.Actor{Type: audit.ActorTypeUser, ID: testAuditUserID}, payload.Actor())
				s.Equal(user, payload.Subject())
				s.Equal(user, payload.Target())
				s.Equal(testAuditOUID, payload.OU())
				return
			}
			s.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, payload.Actor())
			s.Equal(audit.Ref{}, payload.Subject(), "a failed check names no account here")
			s.Equal(audit.Ref{Type: audit.TargetTypeUser}, payload.Target())
			s.Empty(payload.OU())
		})
	}
}

// The credentials are handed to the provider, which may change the map, so the method is classified before the
// call and a sign-in is still recorded when the provider has emptied the map by the time it returns.
func (s *ManagerAuditTestSuite) TestAuthenticateUser_ClassifiesTheMethodBeforeTheProviderChangesTheCredentials() {
	bus, published := newRecordingBus(s.T())
	s.provider.On("Authenticate", mock.Anything, typedIdentifiers(), mock.Anything,
		(*providers.AuthnMetadata)(nil)).Run(func(args mock.Arguments) {
		clear(args.Get(2).(map[string]interface{}))
	}).Return(authenticatedEntity(providers.EntityCategoryUser), (*tidcommon.ServiceError)(nil)).Once()

	_, _, svcErr := s.manager(bus).AuthenticateUser(context.Background(), typedIdentifiers(), passwordCredentials(),
		nil, nil, providers.AuthUser{})

	s.Nil(svcErr)
	s.requireOnlyAuthentication(published(), "credential")
}

func (s *ManagerAuditTestSuite) TestAuthenticateUser_TheActorFollowsTheEntityCategory() {
	cases := []struct {
		category providers.EntityCategory
		actor    audit.ActorType
		target   string
	}{
		{providers.EntityCategoryUser, audit.ActorTypeUser, audit.TargetTypeUser},
		{providers.EntityCategoryAgent, audit.ActorTypeAgent, audit.TargetTypeAgent},
		{providers.EntityCategoryApp, audit.ActorTypeApplication, audit.TargetTypeApplication},
	}
	for _, tc := range cases {
		s.Run(string(tc.category), func() {
			_, events := s.authenticate(context.Background(), passwordCredentials(),
				authenticatedEntity(tc.category), nil)

			payload := s.requireOnlyAuthentication(events, "credential")
			s.Equal(audit.Actor{Type: tc.actor, ID: testAuditUserID}, payload.Actor())
			s.Equal(audit.Ref{Type: tc.target, ID: testAuditUserID}, payload.Subject())
			s.Equal(audit.Ref{Type: tc.target, ID: testAuditUserID}, payload.Target())
		})
	}
}

func (s *ManagerAuditTestSuite) TestAuthenticateUser_PublishesNothingForAResultThatNamesNoEntity() {
	credentials := map[string]interface{}{authnprovidercm.CredentialTypeFederated: "code"}
	result := &providers.AuthnResult{
		EntityReferenceToken: map[string]interface{}{"sub": "external-subject"},
		Attributes:           &providers.AttributesResponse{},
	}

	svcErr, events := s.authenticate(context.Background(), credentials, result, nil)

	s.Nil(svcErr)
	s.Empty(events)
}

func (s *ManagerAuditTestSuite) TestAuthenticateUser_AFaultChoosingTheProviderIsAFailure() {
	bus, published := newRecordingBus(s.T())
	mgr, err := Initialize(s.provider, map[string]providers.CustomAuthnProvider{
		"custom": {Instance: providermock.NewAuthnProviderInterfaceMock(s.T()), Creds: []string{"pin"}},
	}, bus, FederatedSyncDependencies{})
	s.Require().NoError(err)

	_, _, svcErr := mgr.AuthenticateUser(context.Background(), typedIdentifiers(),
		map[string]interface{}{"pin": "1234", "password": testAuditPassword}, nil, nil, providers.AuthUser{})

	s.Require().NotNil(svcErr)
	payload := s.requireOnlyAuthentication(published(), "credential")
	s.Equal(audit.OutcomeFailed(audit.ReasonServerError), payload.Outcome())
}

func (s *ManagerAuditTestSuite) TestAuthenticateUser_PublishesNothingForACheckThatIsNotASignIn() {
	cases := []struct {
		name        string
		credentials map[string]interface{}
	}{
		{"a provisioned entity ID", map[string]interface{}{
			authnprovidercm.CredentialTypeProvisionedEntityID: testAuditUserID}},
		{"a client secret", map[string]interface{}{authnprovidercm.CredentialTypeClientSecret: "secret"}},
		{"a flow secret", map[string]interface{}{authnprovidercm.CredentialTypeFlowSecret: "secret"}},
	}
	for _, tc := range cases {
		for _, outcome := range []struct {
			name   string
			result *providers.AuthnResult
			err    *tidcommon.ServiceError
		}{
			{"succeeds", authenticatedEntity(providers.EntityCategoryUser), nil},
			{"fails", nil, clientErrorCode(authnprovidercm.ErrorCodeAuthenticationFailed)},
		} {
			s.Run(tc.name+" that "+outcome.name, func() {
				_, events := s.authenticate(context.Background(), tc.credentials, outcome.result, outcome.err)

				s.Empty(events)
			})
		}
	}
}

func (s *ManagerAuditTestSuite) TestAuthenticateUser_PublishesNothingWithoutCredentials() {
	for _, credentials := range []map[string]interface{}{nil, {}} {
		bus, published := newRecordingBus(s.T())

		_, _, svcErr := s.manager(bus).AuthenticateUser(context.Background(), typedIdentifiers(), credentials,
			nil, nil, providers.AuthUser{})

		s.Require().NotNil(svcErr)
		s.Equal(ErrorAuthenticationFailed.Code, svcErr.Code)
		s.Empty(published())
	}
}

func (s *ManagerAuditTestSuite) TestAuthenticateUser_IgnoresTheRequestsSecurityContextForTheActor() {
	bearer := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("0196a1b2-0000-7000-8000-0000000000c3", "", "", nil, nil))
	delegated := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest("0196a1b2-0000-7000-8000-0000000000c3", "", "", nil,
			map[string]interface{}{"act": map[string]interface{}{"sub": "0196a1b2-0000-7000-8000-0000000000c4"}}))
	for name, ctx := range map[string]context.Context{"a bearer token": bearer, "an act claim": delegated} {
		for _, tc := range []struct {
			name   string
			result *providers.AuthnResult
			err    *tidcommon.ServiceError
			actor  audit.Actor
		}{
			{"the password is right", authenticatedEntity(providers.EntityCategoryUser), nil,
				audit.Actor{Type: audit.ActorTypeUser, ID: testAuditUserID}},
			{"the password is wrong", nil, clientErrorCode(authnprovidercm.ErrorCodeAuthenticationFailed),
				audit.Actor{Type: audit.ActorTypeAnonymous}},
			{"the account is locked", nil, clientErrorCode(authnprovidercm.ErrorCodeAccountLocked),
				audit.Actor{Type: audit.ActorTypeAnonymous}},
		} {
			s.Run(name+" and "+tc.name, func() {
				_, events := s.authenticate(ctx, passwordCredentials(), tc.result, tc.err)

				s.Require().Len(events, 1)
				s.Equal(tc.actor, events[0].payload.Actor())
				s.Empty(events[0].payload.OnBehalfOf())
			})
		}
	}
}

func (s *ManagerAuditTestSuite) TestAuthenticateUser_SkipsAContactProofAndRecordsARecoveryProof() {
	credentials := map[string]interface{}{authnprovidercm.CredentialTypeOTP: "123456"}
	cases := []struct {
		name      string
		ctx       context.Context
		published bool
	}{
		{"a sign-in", context.Background(), true},
		{"a recovery", authnprovidercm.WithRecoveryProof(context.Background()), true},
		{"a contact verification", authnprovidercm.WithContactProof(context.Background()), false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, events := s.authenticate(tc.ctx, credentials, authenticatedEntity(providers.EntityCategoryUser), nil)

			if !tc.published {
				s.Empty(events)
				return
			}
			s.requireOnlyAuthentication(events, "otp")
		})
	}
}

func (s *ManagerAuditTestSuite) TestAuthenticateUser_SucceedsWhenTheBusRefusesTheEvent() {
	failing := eventbusmock.NewEventBusInterfaceMock(s.T())
	failing.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("audit queue is full")).Once()
	s.provider.On("Authenticate", mock.Anything, typedIdentifiers(), passwordCredentials(),
		(*providers.AuthnMetadata)(nil)).Return(authenticatedEntity(providers.EntityCategoryUser),
		(*tidcommon.ServiceError)(nil)).Once()

	authUser, _, svcErr := s.manager(failing).AuthenticateUser(context.Background(), typedIdentifiers(),
		passwordCredentials(), nil, nil, providers.AuthUser{})

	s.Nil(svcErr)
	s.True(authUser.IsAuthenticated())
}

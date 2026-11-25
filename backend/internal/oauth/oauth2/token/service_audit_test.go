// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/dpopmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/granthandlersmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/scopemock"
)

const (
	testAuditAppID    = "01900000-0000-7000-8000-000000000061"
	testAuditUserID   = "01900000-0000-7000-8000-000000000062"
	testAuditClientID = "audit-token-client"
	testAuditSecret   = "audit-token-secret-0123456789"
	testAuditCode     = "audit-authorization-code-0123456789"
	// testAuditAccessToken and testAuditRefreshToken are shaped as the server issues them.
	testAuditAccessToken  = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJhdWRpdCJ9.c2lnbmF0dXJlLWFjY2Vzcw"
	testAuditRefreshToken = "eyJhbGciOiJSUzI1NiJ9.eyJ0eXAiOiJyZWZyZXNoIn0.c2lnbmF0dXJlLXJlZnJlc2g"
	testAuditIDToken      = "eyJhbGciOiJSUzI1NiJ9.eyJub25jZSI6ImF1ZGl0In0.c2lnbmF0dXJlLWlk"
)

// publishedEvent is one audit event the token endpoint published.
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
// an event the allowlist refuses is never recorded. The returned function stops the bus, which delivers
// every queued event, and returns what the sink received.
func newRecordingBus(t *testing.T) (eventbus.EventBusInterface, func() []publishedEvent) {
	bus, registry, stopper := eventbus.Initialize()
	t.Cleanup(stopper.Stop)

	sink := &recordingSink{}
	require.NoError(t, audit.Subscribe(registry, "token-test", sink))
	return bus, func() []publishedEvent {
		stopper.Stop()
		sink.mu.Lock()
		defer sink.mu.Unlock()
		return sink.events
	}
}

// discardingBus accepts every event and keeps none, for tests the audit event does not matter to.
type discardingBus struct{}

func (discardingBus) Publish(context.Context, *eventbus.Event) error {
	return nil
}

// requireNoCredential checks that no token, code or secret of the request or its response is recorded.
func requireNoCredential(t *testing.T, event publishedEvent) {
	t.Helper()
	recorded := fmt.Sprintf("%+v", event.payload)
	for _, credential := range []string{testAuditAccessToken, testAuditRefreshToken, testAuditIDToken,
		testAuditCode, testAuditSecret, testAuditClientID} {
		require.NotContains(t, recorded, credential)
	}
}

type TokenServiceAuditTestSuite struct {
	suite.Suite
	grantProvider  *granthandlersmock.GrantHandlerProviderInterfaceMock
	grantHandler   *granthandlersmock.GrantHandlerInterfaceMock
	scopeValidator *scopemock.ScopeValidatorInterfaceMock
}

func TestTokenServiceAuditTestSuite(t *testing.T) {
	suite.Run(t, new(TokenServiceAuditTestSuite))
}

func (suite *TokenServiceAuditTestSuite) SetupTest() {
	suite.grantProvider = granthandlersmock.NewGrantHandlerProviderInterfaceMock(suite.T())
	suite.grantHandler = granthandlersmock.NewGrantHandlerInterfaceMock(suite.T())
	suite.scopeValidator = scopemock.NewScopeValidatorInterfaceMock(suite.T())
}

func (suite *TokenServiceAuditTestSuite) SetupSubTest() {
	suite.SetupTest()
}

// newService builds the token service publishing on bus, with observability off.
func (suite *TokenServiceAuditTestSuite) newService(bus eventbus.EventBusInterface) TokenServiceInterface {
	return newTokenService(suite.grantProvider, suite.scopeValidator, nil,
		dpopmock.NewVerifierInterfaceMock(suite.T()), "https://example.test/oauth2/token", false, bus)
}

// client is a registered client of category that may use grantType.
func client(category providers.EntityCategory, grantType providers.GrantType) *providers.OAuthClient {
	return &providers.OAuthClient{
		ID:             testAuditAppID,
		ClientID:       testAuditClientID,
		EntityCategory: category,
		GrantTypes:     []providers.GrantType{grantType},
	}
}

// issuing makes the grant handler for grantType accept the request and issue tokens for subjectID of
// subjectCategory, or answer with grantErr.
func (suite *TokenServiceAuditTestSuite) issuing(grantType providers.GrantType, subjectID, subjectCategory string,
	grantErr *model.ErrorResponse) {
	suite.grantProvider.EXPECT().GetGrantHandler(grantType).Return(suite.grantHandler, nil).Once()
	suite.grantHandler.EXPECT().ValidateGrant(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	suite.scopeValidator.EXPECT().ValidateScopes(mock.Anything, mock.Anything, mock.Anything).
		Return("openid", nil).Once()
	if grantErr != nil {
		suite.grantHandler.EXPECT().HandleGrant(mock.Anything, mock.Anything, mock.Anything).
			Return(nil, grantErr).Once()
		return
	}
	suite.grantHandler.EXPECT().HandleGrant(mock.Anything, mock.Anything, mock.Anything).
		Return(&model.TokenResponseDTO{
			AccessToken: model.TokenDTO{Token: testAuditAccessToken, TokenType: "Bearer", ExpiresIn: 3600,
				Scopes: []string{"openid"}, Subject: subjectID, SubjectID: subjectID,
				SubjectCategory: subjectCategory},
			RefreshToken: model.TokenDTO{Token: testAuditRefreshToken},
			IDToken:      model.TokenDTO{Token: testAuditIDToken},
		}, nil).Once()
}

// An issued token is one token.issue event by the client, named by its application or agent ID, about
// the user it was issued for, with the grant type and no token, code or secret.
func (suite *TokenServiceAuditTestSuite) TestAnIssuedTokenPublishesOneEvent() {
	cases := []struct {
		name      string
		category  providers.EntityCategory
		grantType providers.GrantType
		subjectID string
		subjCat   string
		actor     audit.Actor
		subject   audit.Ref
	}{
		{
			name: "an authorization code for a user", category: providers.EntityCategoryApp,
			grantType: providers.GrantTypeAuthorizationCode, subjectID: testAuditUserID, subjCat: "user",
			actor:   audit.Actor{Type: audit.ActorTypeApplication, ID: testAuditAppID},
			subject: audit.Ref{Type: audit.TargetTypeUser, ID: testAuditUserID},
		},
		{
			name: "client credentials of an application, for itself", category: providers.EntityCategoryApp,
			grantType: providers.GrantTypeClientCredentials, subjectID: testAuditAppID, subjCat: "app",
			actor: audit.Actor{Type: audit.ActorTypeApplication, ID: testAuditAppID},
		},
		{
			name: "client credentials of an agent, for itself", category: providers.EntityCategoryAgent,
			grantType: providers.GrantTypeClientCredentials, subjectID: testAuditAppID, subjCat: "agent",
			actor: audit.Actor{Type: audit.ActorTypeAgent, ID: testAuditAppID},
		},
		{
			name: "a token exchange whose subject resolved to no entity", category: providers.EntityCategoryApp,
			grantType: providers.GrantTypeTokenExchange,
			actor:     audit.Actor{Type: audit.ActorTypeApplication, ID: testAuditAppID},
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.issuing(tc.grantType, tc.subjectID, tc.subjCat, nil)
			bus, published := newRecordingBus(suite.T())

			response, tokenErr := suite.newService(bus).ProcessTokenRequest(context.Background(),
				&model.TokenRequest{GrantType: string(tc.grantType), ClientID: testAuditClientID,
					ClientSecret: testAuditSecret, Code: testAuditCode, Scope: "openid"},
				client(tc.category, tc.grantType))

			suite.Require().Nil(tokenErr)
			suite.Require().Equal(testAuditAccessToken, response.AccessToken)
			events := published()
			suite.Require().Len(events, 1)
			event := events[0]
			suite.Equal(audit.EventTokenIssue, event.name)
			suite.Equal(audit.ActionIssue, event.payload.Action())
			suite.Equal(audit.Ref{Type: audit.TargetTypeToken}, event.payload.Target())
			suite.Equal(tc.actor, event.payload.Actor())
			suite.Equal(tc.subject, event.payload.Subject())
			suite.Equal(audit.OutcomeSucceeded, event.payload.Outcome())
			suite.Equal(map[string]string{audit.DetailGrantType: string(tc.grantType)}, event.payload.Details())
			requireNoCredential(suite.T(), event)
		})
	}
}

// A refused token request is one event: denied when the client or what it asked for is refused, failed
// when the request is malformed or the grant does not hold. A grant type the server does not know is
// not recorded, since it is text the caller chose.
func (suite *TokenServiceAuditTestSuite) TestARefusedTokenRequestPublishesOneEvent() {
	cases := []struct {
		name      string
		grantType string
		setup     func()
		outcome   audit.Outcome
		details   map[string]string
	}{
		{
			name: "no grant type", outcome: audit.OutcomeFailed(constants.ErrorInvalidRequest), setup: func() {},
		},
		{
			name: "a grant type the server does not know", grantType: "urn:example:" + testAuditSecret,
			outcome: audit.OutcomeFailed(constants.ErrorUnsupportedGrantType), setup: func() {},
		},
		{
			name: "a grant type the client may not use", grantType: string(providers.GrantTypeRefreshToken),
			setup: func() {
				suite.grantProvider.EXPECT().GetGrantHandler(providers.GrantTypeRefreshToken).
					Return(suite.grantHandler, nil).Once()
			},
			outcome: audit.OutcomeDenied(constants.ErrorUnauthorizedClient),
			details: map[string]string{audit.DetailGrantType: "refresh_token"},
		},
		{
			name: "a code that does not hold", grantType: string(providers.GrantTypeAuthorizationCode),
			setup: func() {
				suite.issuing(providers.GrantTypeAuthorizationCode, "", "",
					&model.ErrorResponse{Error: constants.ErrorInvalidGrant, ErrorDescription: "Invalid code"})
			},
			outcome: audit.OutcomeFailed(constants.ErrorInvalidGrant),
			details: map[string]string{audit.DetailGrantType: "authorization_code"},
		},
		{
			name: "a grant the user refused", grantType: string(providers.GrantTypeAuthorizationCode),
			setup: func() {
				suite.issuing(providers.GrantTypeAuthorizationCode, "", "",
					&model.ErrorResponse{Error: constants.ErrorAccessDenied})
			},
			outcome: audit.OutcomeDenied(constants.ErrorAccessDenied),
			details: map[string]string{audit.DetailGrantType: "authorization_code"},
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			tc.setup()
			bus, published := newRecordingBus(suite.T())

			_, tokenErr := suite.newService(bus).ProcessTokenRequest(context.Background(),
				&model.TokenRequest{GrantType: tc.grantType, ClientID: testAuditClientID, Code: testAuditCode},
				client(providers.EntityCategoryApp, providers.GrantTypeAuthorizationCode))

			suite.Require().NotNil(tokenErr)
			events := published()
			suite.Require().Len(events, 1)
			event := events[0]
			suite.Equal(audit.EventTokenIssue, event.name)
			suite.Equal(audit.Actor{Type: audit.ActorTypeApplication, ID: testAuditAppID}, event.payload.Actor())
			suite.Equal(audit.Ref{}, event.payload.Subject())
			suite.Equal(tc.outcome, event.payload.Outcome())
			if tc.details == nil {
				suite.Empty(event.payload.Details())
			} else {
				suite.Equal(tc.details, event.payload.Details())
			}
			requireNoCredential(suite.T(), event)
		})
	}
}

// The answers a polling client gets while a CIBA or device grant is pending are not outcomes and would
// flood the log, so they publish nothing. An expired device code is an outcome and is recorded.
func (suite *TokenServiceAuditTestSuite) TestPollingAnswersPublishNothing() {
	cases := []struct {
		name      string
		grantType providers.GrantType
		code      string
		published int
	}{
		{"a pending CIBA request", providers.GrantTypeCIBA, constants.ErrorAuthorizationPending, 0},
		{"a device client polling too fast", providers.GrantTypeDeviceCode, constants.ErrorSlowDown, 0},
		{"an expired device code", providers.GrantTypeDeviceCode, constants.ErrorExpiredToken, 1},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.issuing(tc.grantType, "", "", &model.ErrorResponse{Error: tc.code})
			bus, published := newRecordingBus(suite.T())

			_, tokenErr := suite.newService(bus).ProcessTokenRequest(context.Background(),
				&model.TokenRequest{GrantType: string(tc.grantType), ClientID: testAuditClientID},
				client(providers.EntityCategoryApp, tc.grantType))

			suite.Require().NotNil(tokenErr)
			suite.Require().Equal(tc.code, tokenErr.Error)
			suite.Len(published(), tc.published)
		})
	}
}

// The token endpoint authenticates a client, so a bearer token the request also carries names neither
// the actor nor the principal it acted for.
func (suite *TokenServiceAuditTestSuite) TestTheActorIsTheClientWhateverTokenTheRequestCarries() {
	suite.issuing(providers.GrantTypeClientCredentials, testAuditAppID, "app", nil)
	bus, published := newRecordingBus(suite.T())
	ctx := security.WithSecurityContextTest(context.Background(),
		security.NewSecurityContextForTest(testAuditUserID, "", "", nil, nil))

	_, tokenErr := suite.newService(bus).ProcessTokenRequest(ctx,
		&model.TokenRequest{GrantType: string(providers.GrantTypeClientCredentials), ClientID: testAuditClientID},
		client(providers.EntityCategoryApp, providers.GrantTypeClientCredentials))

	suite.Require().Nil(tokenErr)
	events := published()
	suite.Require().Len(events, 1)
	suite.Equal(audit.Actor{Type: audit.ActorTypeApplication, ID: testAuditAppID}, events[0].payload.Actor())
	suite.Equal(audit.Ref{}, events[0].payload.OnBehalfOf())
}

// A bus that fails to publish does not fail the token request.
func (suite *TokenServiceAuditTestSuite) TestAFailingBusDoesNotFailTheRequest() {
	suite.issuing(providers.GrantTypeClientCredentials, testAuditAppID, "app", nil)
	bus := eventbusmock.NewEventBusInterfaceMock(suite.T())
	bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("bus closed")).Once()

	response, tokenErr := suite.newService(bus).ProcessTokenRequest(context.Background(),
		&model.TokenRequest{GrantType: string(providers.GrantTypeClientCredentials), ClientID: testAuditClientID},
		client(providers.EntityCategoryApp, providers.GrantTypeClientCredentials))

	suite.Nil(tokenErr)
	suite.Equal(testAuditAccessToken, response.AccessToken)
}

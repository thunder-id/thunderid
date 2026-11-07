// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package clientauth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/eventbus"
	"github.com/thunder-id/thunderid/internal/system/eventbus/audit"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/eventbusmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
)

const (
	testAuditEntityID = "01900000-0000-7000-8000-000000000051"
	testAuditSecret   = "audit-client-secret-0123456789"
	testAuditUserID   = "01900000-0000-7000-8000-000000000052"
)

// publishedEvent is one audit event the middleware published.
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
	require.NoError(t, audit.Subscribe(registry, "clientauth-test", sink))
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

type ClientAuthMiddlewareAuditTestSuite struct {
	suite.Suite
	mockInboundClient *inboundclientmock.InboundClientServiceInterfaceMock
	mockAuthnProvider *managermock.AuthnProviderManagerMock
}

func TestClientAuthMiddlewareAuditTestSuite(t *testing.T) {
	suite.Run(t, new(ClientAuthMiddlewareAuditTestSuite))
}

func (suite *ClientAuthMiddlewareAuditTestSuite) SetupTest() {
	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
}

// serve runs request through the middleware publishing on bus and returns the status it answered with.
func (suite *ClientAuthMiddlewareAuditTestSuite) serve(bus eventbus.EventBusInterface, request *http.Request) int {
	actorProvider := actorprovider.Initialize(suite.mockInboundClient,
		entityprovidermock.NewEntityProviderInterfaceMock(suite.T()), noopAuthnMgr(), nil, nil)
	middleware := ClientAuthMiddleware(actorProvider, suite.mockAuthnProvider, nil,
		jwtmock.NewJWTServiceInterfaceMock(suite.T()), nil, testIssuer, testAssertionCfg, testJWTLeeway, bus)
	recorder := httptest.NewRecorder()
	middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(recorder, request)
	return recorder.Code
}

// secretPostClient is a registered client that authenticates with client_secret_post.
func secretPostClient() *providers.OAuthClient {
	return &providers.OAuthClient{
		ID:                      testAuditEntityID,
		ClientID:                testClientID,
		TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
		EntityCategory:          providers.EntityCategoryApp,
		GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
	}
}

// formRequest builds a token request carrying form.
func formRequest(form url.Values) *http.Request {
	request := httptest.NewRequest(http.MethodPost, "/oauth2/token", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

// failingSecret makes every client secret check fail, as a wrong secret does.
func (suite *ClientAuthMiddlewareAuditTestSuite) failingSecret() {
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType, Code: authnprovidermgr.ErrorAuthenticationFailed.Code,
		}).Once()
}

// Each refused authentication is one client.authenticate event: anonymous, with the client the request
// named as target and no subject, denied when the client is not valid and failed when the request is
// malformed. No secret and no assertion is recorded.
func (suite *ClientAuthMiddlewareAuditTestSuite) TestEveryRefusalPublishesOneEvent() {
	assertion := buildFakeJWTWithSub("assertion-client")
	basic := base64.StdEncoding.EncodeToString([]byte(testClientID + ":" + testAuditSecret))
	cases := []struct {
		name     string
		setup    func()
		request  func() *http.Request
		status   int
		clientID string
		outcome  audit.Outcome
	}{
		{
			name: "an unknown client",
			setup: func() {
				suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "unknown-client").
					Return(nil, nil).Once()
			},
			request: func() *http.Request {
				return formRequest(url.Values{"client_id": {"unknown-client"}, "client_secret": {testAuditSecret}})
			},
			status: http.StatusUnauthorized, clientID: "unknown-client",
			outcome: audit.OutcomeDenied(constants.ErrorInvalidClient),
		},
		{
			name: "a wrong secret in Basic credentials",
			setup: func() {
				suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
					Return(&providers.OAuthClient{ID: testAuditEntityID, ClientID: testClientID,
						TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretBasic}, nil).Once()
				suite.failingSecret()
			},
			request: func() *http.Request {
				request := formRequest(url.Values{"grant_type": {"client_credentials"}})
				request.Header.Set("Authorization", "Basic "+basic)
				return request
			},
			status: http.StatusUnauthorized, clientID: testClientID,
			outcome: audit.OutcomeDenied(constants.ErrorInvalidClient),
		},
		{
			name: "an assertion for a client that authenticates with a secret",
			setup: func() {
				suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, "assertion-client").
					Return(secretPostClient(), nil).Once()
			},
			request: func() *http.Request {
				return formRequest(url.Values{
					"client_assertion_type": {constants.SupportedClientAssertionType},
					"client_assertion":      {assertion},
				})
			},
			status: http.StatusBadRequest, clientID: "assertion-client",
			outcome: audit.OutcomeDenied(constants.ErrorUnauthorizedClient),
		},
		{
			name:    "no client",
			setup:   func() {},
			request: func() *http.Request { return formRequest(url.Values{"grant_type": {"client_credentials"}}) },
			status:  http.StatusBadRequest, outcome: audit.OutcomeFailed(constants.ErrorInvalidRequest),
		},
		{
			name:  "a client_id that differs from the Basic credentials",
			setup: func() {},
			request: func() *http.Request {
				request := formRequest(url.Values{"client_id": {"other-client"}})
				request.Header.Set("Authorization", "Basic "+basic)
				return request
			},
			status: http.StatusBadRequest, clientID: "other-client",
			outcome: audit.OutcomeFailed(constants.ErrorInvalidRequest),
		},
		{
			name:  "two authentication methods",
			setup: func() {},
			request: func() *http.Request {
				return formRequest(url.Values{"client_id": {testClientID}, "client_secret": {testAuditSecret},
					"client_assertion": {assertion}})
			},
			status: http.StatusBadRequest, clientID: testClientID,
			outcome: audit.OutcomeFailed(constants.ErrorInvalidRequest),
		},
	}
	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setup()
			bus, published := newRecordingBus(suite.T())

			status := suite.serve(bus, tc.request())

			suite.Require().Equal(tc.status, status)
			events := published()
			suite.Require().Len(events, 1)
			event := events[0]
			suite.Equal(audit.EventClientAuthenticate, event.name)
			suite.Equal(audit.ActionAuthenticate, event.payload.Action())
			suite.Equal(audit.Ref{Type: audit.TargetTypeApplication, ID: tc.clientID}, event.payload.Target())
			suite.Equal(audit.Ref{}, event.payload.Subject())
			suite.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, event.payload.Actor())
			suite.Equal(tc.outcome, event.payload.Outcome())
			suite.Empty(event.payload.Details())
			recorded := fmt.Sprintf("%+v", event.payload)
			suite.NotContains(recorded, testAuditSecret, "a client secret is never recorded")
			suite.NotContains(recorded, basic, "the credentials are never recorded")
			suite.NotContains(recorded, assertion, "an assertion is never recorded")
			suite.NotContains(recorded, testAuditEntityID, "a refusal names only what the request presented")
		})
	}
}

// A refusal is anonymous even when the request carries a verified bearer token, so a token sent to an
// OAuth endpoint never names the actor or the principal it acted for.
func (suite *ClientAuthMiddlewareAuditTestSuite) TestARefusalIsAnonymousWhateverTokenTheRequestCarries() {
	bus, published := newRecordingBus(suite.T())
	request := formRequest(url.Values{"grant_type": {"client_credentials"}})
	request = request.WithContext(security.WithSecurityContextTest(request.Context(),
		security.NewSecurityContextForTest(testAuditUserID, "", "", nil, nil)))

	suite.Require().Equal(http.StatusBadRequest, suite.serve(bus, request))

	events := published()
	suite.Require().Len(events, 1)
	suite.Equal(audit.Actor{Type: audit.ActorTypeAnonymous}, events[0].payload.Actor())
	suite.Equal(audit.Ref{}, events[0].payload.OnBehalfOf())
}

// The client ID a request names is untrusted text, so it is cut to the 128 characters a target ID holds.
func (suite *ClientAuthMiddlewareAuditTestSuite) TestALongClientIDIsCut() {
	long := strings.Repeat("c", 300)
	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, long).Return(nil, nil).Once()
	bus, published := newRecordingBus(suite.T())

	suite.Require().Equal(http.StatusUnauthorized,
		suite.serve(bus, formRequest(url.Values{"client_id": {long}, "client_secret": {testAuditSecret}})))

	events := published()
	suite.Require().Len(events, 1)
	suite.Equal(long[:128], events[0].payload.Target().ID)
}

// A client that authenticates publishes nothing: the request it goes on to make records its own event.
func (suite *ClientAuthMiddlewareAuditTestSuite) TestAnAuthenticatedClientPublishesNothing() {
	suite.mockInboundClient.On("GetOAuthClientByClientID", mock.Anything, testClientID).
		Return(secretPostClient(), nil).Once()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil)).Once()
	bus, published := newRecordingBus(suite.T())

	suite.Require().Equal(http.StatusOK,
		suite.serve(bus, formRequest(url.Values{"client_id": {testClientID}, "client_secret": {testAuditSecret}})))

	suite.Empty(published())
}

// A bus that fails to publish does not change the answer to a refused client.
func (suite *ClientAuthMiddlewareAuditTestSuite) TestAFailingBusDoesNotChangeTheRefusal() {
	bus := eventbusmock.NewEventBusInterfaceMock(suite.T())
	bus.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("bus closed")).Once()

	suite.Equal(http.StatusBadRequest, suite.serve(bus, formRequest(url.Values{"grant_type": {"client_credentials"}})))
}

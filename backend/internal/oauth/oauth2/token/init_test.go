// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/discoverymock"
	"github.com/thunder-id/thunderid/tests/mocks/ouprovidermock"
)

// TokenRouteRegistrationTestSuite covers which token routes a deployment gets.
//
// The organization-unit-scoped route admits a client against the unit it names, and that answer is
// the deployment's to give. A deployment that cannot give it, such as the embedded engine whose
// actor provider comes from the embedder, must not serve the route at all: the middleware only
// establishes that the unit exists, so an unguarded route would hand any client a token for any
// organization unit in the deployment.
type TokenRouteRegistrationTestSuite struct {
	suite.Suite
}

func TestTokenRouteRegistrationTestSuite(t *testing.T) {
	suite.Run(t, new(TokenRouteRegistrationTestSuite))
}

// routes registers the token routes with the endpoint enabled or not, and reports which patterns
// the mux ended up serving.
func (s *TokenRouteRegistrationTestSuite) routes(enabled bool) (bare, scoped string) {
	discoverySvc := discoverymock.NewDiscoveryServiceInterfaceMock(s.T())
	discoverySvc.On("GetOAuth2AuthorizationServerMetadata", mock.Anything).
		Return(&discovery.OAuth2AuthorizationServerMetadata{
			Issuer: "https://localhost:8090",
		})

	mux := http.NewServeMux()
	registerRoutes(mux, newTokenHandler(NewTokenServiceInterfaceMock(s.T()), nil), nil, nil, nil,
		discoverySvc, nil, nil, enabled, engineconfig.ClientAssertionConfig{}, 0)

	pattern := func(target string) string {
		_, p := mux.Handler(httptest.NewRequest(http.MethodPost, target, nil))
		return p
	}
	return pattern("/oauth2/token"), pattern("/ou/customer-a/oauth2/token")
}

// Turned on, both forms are served.
func (s *TokenRouteRegistrationTestSuite) TestBothRoutesAreServedWhenEnabled() {
	bare, scoped := s.routes(true)

	s.NotEmpty(bare)
	s.NotEmpty(scoped, "the organization-unit-scoped route is served")
}

// Left off, which is the default, the qualified route does not exist. This is the embedded
// engine's case, and it is what stops any client obtaining a token for any organization unit
// there when the actor provider does not really answer the question.
func (s *TokenRouteRegistrationTestSuite) TestTheQualifiedRouteIsAbsentWhenDisabled() {
	bare, scoped := s.routes(false)

	s.NotEmpty(bare, "the bare token endpoint is unaffected")
	s.Empty(scoped, "no pattern matches the organization-unit-scoped path")
}

// OUEndpointOrderingTestSuite covers the order the organization-unit-scoped route applies its
// checks in, which is what decides how much an unauthenticated caller can learn.
//
// The rule is: not authenticated is 401, and everything about the organization unit is 400 behind
// it. Answering the unit first would make its existence observable to anyone, since a made-up
// client id would draw a 400 for a unit that does not exist and a 401 for one that does.
type OUEndpointOrderingTestSuite struct {
	suite.Suite
	inbound   *inboundclientmock.InboundClientServiceInterfaceMock
	entities  *entityprovidermock.EntityProviderInterfaceMock
	authn     *managermock.AuthnProviderManagerMock
	ouService *ouprovidermock.OrganizationUnitProviderMock
}

func TestOUEndpointOrderingTestSuite(t *testing.T) {
	suite.Run(t, new(OUEndpointOrderingTestSuite))
}

const (
	orderingClientID     = "m2m-client"
	orderingClientSecret = "m2m-secret" //nolint:gosec // test credential
)

func (s *OUEndpointOrderingTestSuite) SetupTest() {
	s.inbound = inboundclientmock.NewInboundClientServiceInterfaceMock(s.T())
	s.entities = entityprovidermock.NewEntityProviderInterfaceMock(s.T())
	s.authn = managermock.NewAuthnProviderManagerMock(s.T())
	s.ouService = ouprovidermock.NewOrganizationUnitProviderMock(s.T())
	s.authn.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{"userId": orderingClientID},
			(*tidcommon.ServiceError)(nil)).Maybe()
}

// post drives one request through the real route chain and returns the answer.
func (s *OUEndpointOrderingTestSuite) post(ouID string) (*httptest.ResponseRecorder, map[string]any) {
	discoverySvc := discoverymock.NewDiscoveryServiceInterfaceMock(s.T())
	discoverySvc.On("GetOAuth2AuthorizationServerMetadata", mock.Anything).
		Return(&discovery.OAuth2AuthorizationServerMetadata{Issuer: "https://localhost:8090"})

	mux := http.NewServeMux()
	registerRoutes(mux, newTokenHandler(NewTokenServiceInterfaceMock(s.T()), nil),
		actorprovider.Initialize(s.inbound, s.entities, s.authn, nil), s.authn, nil,
		discoverySvc, nil, s.ouService, true, engineconfig.ClientAssertionConfig{}, 0)

	form := url.Values{}
	form.Set("client_id", orderingClientID)
	form.Set("client_secret", orderingClientSecret)
	form.Set("grant_type", string(providers.GrantTypeClientCredentials))
	req := httptest.NewRequest(http.MethodPost, "/ou/"+ouID+"/oauth2/token", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, req)

	var body map[string]any
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	return recorder, body
}

// answers makes the inbound client service, which the actor provider delegates to, give the same
// answer to every organization unit question.
func (s *OUEndpointOrderingTestSuite) answers(accessible bool) {
	s.inbound.EXPECT().IsClientAccessibleFromOU(mock.Anything, mock.Anything, mock.Anything).
		Return(accessible, nil).Maybe()
}

// authenticates makes the client resolve and its secret check out.
func (s *OUEndpointOrderingTestSuite) authenticates() {
	s.inbound.EXPECT().GetOAuthClientByClientID(mock.Anything, orderingClientID).Return(
		&providers.OAuthClient{
			ID: "m2m-app", ClientID: orderingClientID, OUID: "m2m-root",
			TokenEndpointAuthMethod: providers.TokenEndpointAuthMethodClientSecretPost,
			GrantTypes:              []providers.GrantType{providers.GrantTypeClientCredentials},
		}, nil).Once()
}

// A caller who cannot authenticate is told only that, whatever organization unit it named, and the
// unit is never looked up. This is what stops the endpoint being used to enumerate units.
func (s *OUEndpointOrderingTestSuite) TestAnUnauthenticatedCallerLearnsNothingAboutTheUnit() {
	s.inbound.EXPECT().GetOAuthClientByClientID(mock.Anything, orderingClientID).
		Return(nil, assert.AnError).Once()

	recorder, _ := s.post("no-such-ou")

	s.Equal(http.StatusUnauthorized, recorder.Code, "a bad credential is 401, not a 400 about the unit")
	s.ouService.AssertNotCalled(s.T(), "GetOrganizationUnit", mock.Anything, mock.Anything)
}

// Behind authentication, a unit that does not exist and one the client was never shared are
// answered identically, so an authenticated client cannot enumerate units either.
func (s *OUEndpointOrderingTestSuite) TestTheTwoOURefusalsAreIndistinguishable() {
	s.authenticates()
	s.ouService.EXPECT().GetOrganizationUnit(mock.Anything, "no-such-ou").
		Return(providers.OrganizationUnit{}, &tidcommon.ErrorUnauthorized).Once()
	s.answers(true)
	missing, missingBody := s.post("no-such-ou")

	s.SetupTest()
	s.authenticates()
	s.ouService.EXPECT().GetOrganizationUnit(mock.Anything, "customer-a").
		Return(providers.OrganizationUnit{ID: "customer-a"}, nil).Once()
	s.answers(false)
	unshared, unsharedBody := s.post("customer-a")

	s.Equal(http.StatusBadRequest, missing.Code)
	s.Equal(http.StatusBadRequest, unshared.Code)
	s.Equal(missing.Body.String(), unshared.Body.String(),
		"a unit that does not exist and one not shared must read the same")
	s.Equal(constants.OUAccessRefusal, missingBody["error_description"])
	s.Equal(constants.OUAccessRefusal, unsharedBody["error_description"])
}

// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package introspect

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/discoverymock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
)

type InitTestSuite struct {
	suite.Suite
	mockJWTService       *jwtmock.JWTServiceInterfaceMock
	mockDiscoveryService *discoverymock.DiscoveryServiceInterfaceMock
	mockTokenValidator   *tokenservicemock.TokenValidatorInterfaceMock
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockDiscoveryService = discoverymock.NewDiscoveryServiceInterfaceMock(suite.T())
	suite.mockTokenValidator = tokenservicemock.NewTokenValidatorInterfaceMock(suite.T())
	suite.mockDiscoveryService.On("GetOAuth2AuthorizationServerMetadata", mock.Anything).
		Return(&discovery.OAuth2AuthorizationServerMetadata{
			IntrospectionEndpoint: "https://localhost:8090/oauth2/introspect",
		})
}

func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()

	service := Initialize(mux, suite.mockJWTService, nil, nil, suite.mockDiscoveryService,
		suite.mockTokenValidator, nil, clientauth.AssertionValidationConfig{})

	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*TokenIntrospectionServiceInterface)(nil), service)
}

func (suite *InitTestSuite) TestInitialize_RegistersRoutes() {
	mux := http.NewServeMux()

	Initialize(mux, suite.mockJWTService, nil, nil, suite.mockDiscoveryService,
		suite.mockTokenValidator, nil, clientauth.AssertionValidationConfig{})

	// Verify that the routes are registered by attempting to get a handler for them.
	// The pattern includes the method because of CORS middleware wrapping.
	_, pattern := mux.Handler(&http.Request{Method: "POST", URL: &url.URL{Path: "/oauth2/introspect"}})
	assert.Contains(suite.T(), pattern, "/oauth2/introspect")

	_, pattern = mux.Handler(&http.Request{Method: "OPTIONS", URL: &url.URL{Path: "/oauth2/introspect"}})
	assert.Contains(suite.T(), pattern, "/oauth2/introspect")
}

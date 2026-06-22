/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package userinfo

import (
	"net/http"
	"net/url"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/discoverymock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/dpopmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/tokenservicemock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

type InitTestSuite struct {
	suite.Suite
	mockJWTService            *jwtmock.JWTServiceInterfaceMock
	mockTokenValidator        *tokenservicemock.TokenValidatorInterfaceMock
	mockInboundClient         *inboundclientmock.InboundClientServiceInterfaceMock
	mockEntityProvider        *entityprovidermock.EntityProviderInterfaceMock
	mockAttributeCacheService *attributecachemock.AttributeCacheServiceInterfaceMock
	mockDiscoveryService      *discoverymock.DiscoveryServiceInterfaceMock
	mockDPoPVerifier          *dpopmock.VerifierInterfaceMock
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockTokenValidator = tokenservicemock.NewTokenValidatorInterfaceMock(suite.T())
	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockAttributeCacheService = attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
	suite.mockDiscoveryService = discoverymock.NewDiscoveryServiceInterfaceMock(suite.T())
	suite.mockDPoPVerifier = dpopmock.NewVerifierInterfaceMock(suite.T())
	suite.mockDiscoveryService.On("GetOAuth2AuthorizationServerMetadata", mock.Anything).
		Return(&discovery.OAuth2AuthorizationServerMetadata{
			UserInfoEndpoint: "https://localhost:8090/oauth2/userinfo",
		})

	config.ResetServerRuntime()
	_ = config.InitializeServerRuntime(
		"test-home",
		&config.Config{
			OAuth: engineconfig.OAuthConfig{
				DPoP: engineconfig.DPoPConfig{
					AllowedAlgs: []string{"ES256", "PS256"},
				},
			},
		},
	)
}

func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()

	service := Initialize(mux, suite.mockJWTService, nil, nil,
		suite.mockTokenValidator,
		actorprovider.Initialize(suite.mockInboundClient, suite.mockEntityProvider, noopAuthnMgr()),
		suite.mockAttributeCacheService, suite.mockDiscoveryService, suite.mockDPoPVerifier, testhelpers.OAuthConfig())

	assert.NotNil(suite.T(), service)
}

func (suite *InitTestSuite) TestInitialize_RegistersRoutes() {
	mux := http.NewServeMux()

	Initialize(mux, suite.mockJWTService, nil, nil,
		suite.mockTokenValidator,
		actorprovider.Initialize(suite.mockInboundClient, suite.mockEntityProvider, noopAuthnMgr()),
		suite.mockAttributeCacheService, suite.mockDiscoveryService, suite.mockDPoPVerifier, testhelpers.OAuthConfig())

	// Verify that the routes are registered by attempting to get a handler for them.
	// The pattern includes the method because of CORS middleware wrapping.
	_, pattern := mux.Handler(&http.Request{Method: "GET", URL: &url.URL{Path: "/oauth2/userinfo"}})
	assert.Contains(suite.T(), pattern, "/oauth2/userinfo")

	_, pattern = mux.Handler(&http.Request{Method: "POST", URL: &url.URL{Path: "/oauth2/userinfo"}})
	assert.Contains(suite.T(), pattern, "/oauth2/userinfo")

	_, pattern = mux.Handler(&http.Request{Method: "OPTIONS", URL: &url.URL{Path: "/oauth2/userinfo"}})
	assert.Contains(suite.T(), pattern, "/oauth2/userinfo")
}

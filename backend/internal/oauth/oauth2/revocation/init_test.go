// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package revocation

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/clientauth"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/discoverymock"
)

type InitTestSuite struct {
	suite.Suite
	mockJWTService       *jwtmock.JWTServiceInterfaceMock
	mockDiscoveryService *discoverymock.DiscoveryServiceInterfaceMock
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	// Initialize() builds the store, which reads the server runtime config.
	_ = config.InitializeServerRuntime("test", &config.Config{
		Database: config.DatabaseConfig{
			RuntimePersistent: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: ":memory:"},
			},
		},
	})

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockDiscoveryService = discoverymock.NewDiscoveryServiceInterfaceMock(suite.T())
}

func (suite *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize() {
	enforcementService, revocationService := Initialize(
		suite.mockJWTService, nil, time.Hour, true)

	assert.NotNil(suite.T(), enforcementService)
	assert.Implements(suite.T(), (*EnforcementServiceInterface)(nil), enforcementService)
	assert.NotNil(suite.T(), revocationService)
	assert.Implements(suite.T(), (*RevocationServiceInterface)(nil), revocationService)
	assert.Implements(suite.T(), (*RefreshTokenRevokerInterface)(nil), revocationService)
	assert.Implements(suite.T(), (*CriteriaRevokerInterface)(nil), revocationService)
}

func (suite *InitTestSuite) TestInitialize_RegistersRoutes() {
	suite.mockDiscoveryService.On("GetOAuth2AuthorizationServerMetadata", mock.Anything).
		Return(&discovery.OAuth2AuthorizationServerMetadata{
			RevocationEndpoint: "https://localhost:8090/oauth2/revoke",
		})
	mux := http.NewServeMux()
	_, revocationService := Initialize(suite.mockJWTService, nil, time.Hour, true)

	RegisterRoutes(mux, suite.mockJWTService, nil, nil, suite.mockDiscoveryService, revocationService, nil,
		clientauth.AssertionValidationConfig{})

	// The pattern includes the method because of CORS middleware wrapping.
	_, pattern := mux.Handler(&http.Request{Method: "POST", URL: &url.URL{Path: "/oauth2/revoke"}})
	assert.Contains(suite.T(), pattern, "/oauth2/revoke")

	_, pattern = mux.Handler(&http.Request{Method: "OPTIONS", URL: &url.URL{Path: "/oauth2/revoke"}})
	assert.Contains(suite.T(), pattern, "/oauth2/revoke")
}

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

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/discovery"
	"github.com/thunder-id/thunderid/internal/system/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
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
	enforcementService, revocationService, _, err := Initialize(revocationConfig(10), suite.mockJWTService, nil)

	assert.NoError(suite.T(), err)

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
	_, revocationService, _, err := Initialize(revocationConfig(10), suite.mockJWTService, nil)
	assert.NoError(suite.T(), err)

	RegisterRoutes(mux, suite.mockJWTService, nil, nil, suite.mockDiscoveryService, revocationService, nil, nil,
		engineconfig.ClientAssertionConfig{}, 0)

	// The pattern includes the method because of CORS middleware wrapping.
	_, pattern := mux.Handler(&http.Request{Method: "POST", URL: &url.URL{Path: "/oauth2/revoke"}})
	assert.Contains(suite.T(), pattern, "/oauth2/revoke")

	_, pattern = mux.Handler(&http.Request{Method: "OPTIONS", URL: &url.URL{Path: "/oauth2/revoke"}})
	assert.Contains(suite.T(), pattern, "/oauth2/revoke")
}

func (suite *InitTestSuite) TestInitialize_RejectsNegativeMaxEntriesPerChange() {
	enforcementService, revocationService, _, err := Initialize(revocationConfig(-1), suite.mockJWTService, nil)

	assert.ErrorContains(suite.T(), err, "oauth.revocation.criteria.max_entries_per_change")
	assert.Nil(suite.T(), enforcementService)
	assert.Nil(suite.T(), revocationService)
}

func (suite *InitTestSuite) TestInitialize_ReadsTheOAuthConfig() {
	cfg := revocationConfig(7)
	cfg.OAuth.RefreshToken.ValidityPeriod = 7200
	explicit := false
	cfg.OAuth.Revocation.TokenFamily.OnExplicitRevoke = &explicit
	cfg.JWT.Leeway = 30

	_, writeService, _, err := Initialize(cfg, suite.mockJWTService, nil)

	suite.Require().NoError(err)
	svc, ok := writeService.(*revocationService)
	suite.Require().True(ok)
	suite.Equal(7, svc.maxEntriesPerChange)
	suite.Equal(2*time.Hour, svc.tokenFamilyLifetime)
	suite.False(svc.revokeTokenFamily)
	suite.Equal(30*time.Second, svc.validationLeeway)
}

func (suite *InitTestSuite) TestInitialize_ZeroMaxEntriesPerChangeIsKept() {
	_, writeService, _, err := Initialize(revocationConfig(0), suite.mockJWTService, nil)

	suite.Require().NoError(err)
	svc, ok := writeService.(*revocationService)
	suite.Require().True(ok)
	suite.Equal(0, svc.maxEntriesPerChange)
}

func revocationConfig(maxEntriesPerChange int) oauthconfig.Config {
	cfg := oauthconfig.Config{}
	cfg.OAuth.RefreshToken.ValidityPeriod = 3600
	cfg.OAuth.Revocation.Criteria.MaxEntriesPerChange = maxEntriesPerChange
	return cfg
}

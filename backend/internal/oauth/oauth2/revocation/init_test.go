/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package revocation

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

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
	suite.mockDiscoveryService.On("GetOAuth2AuthorizationServerMetadata", mock.Anything).
		Return(&discovery.OAuth2AuthorizationServerMetadata{
			RevocationEndpoint: "https://localhost:8090/oauth2/revoke",
		})
}

func (suite *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()

	enforcementService, revocationService := Initialize(
		mux, suite.mockJWTService, nil, nil, suite.mockDiscoveryService, nil, time.Hour, true)

	assert.NotNil(suite.T(), enforcementService)
	assert.Implements(suite.T(), (*EnforcementServiceInterface)(nil), enforcementService)
	assert.NotNil(suite.T(), revocationService)
	assert.Implements(suite.T(), (*RevocationServiceInterface)(nil), revocationService)
	assert.Implements(suite.T(), (*RefreshTokenRevokerInterface)(nil), revocationService)
	assert.Implements(suite.T(), (*CriteriaRevokerInterface)(nil), revocationService)
}

func (suite *InitTestSuite) TestInitialize_RegistersRoutes() {
	mux := http.NewServeMux()

	Initialize(mux, suite.mockJWTService, nil, nil, suite.mockDiscoveryService, nil, time.Hour, true)

	// The pattern includes the method because of CORS middleware wrapping.
	_, pattern := mux.Handler(&http.Request{Method: "POST", URL: &url.URL{Path: "/oauth2/revoke"}})
	assert.Contains(suite.T(), pattern, "/oauth2/revoke")

	_, pattern = mux.Handler(&http.Request{Method: "OPTIONS", URL: &url.URL{Path: "/oauth2/revoke"}})
	assert.Contains(suite.T(), pattern, "/oauth2/revoke")
}

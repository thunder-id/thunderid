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

package authz

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v3"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/cors"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
)

type InitTestSuite struct {
	suite.Suite
	mockInboundClient   *inboundclientmock.InboundClientServiceInterfaceMock
	mockResourceService *resourcemock.ResourceServiceInterfaceMock
	mockJWTService      *jwtmock.JWTServiceInterfaceMock
	mockFlowExecService *flowexecmock.FlowExecServiceInterfaceMock
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	// Initialize Runtime config with basic test config
	var allowedOrigins cors.OriginEntries
	suite.Require().NoError(yaml.Unmarshal([]byte(`
- https://example.com
`), &allowedOrigins))
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			Runtime: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
		},
		GateClient: config.GateClientConfig{
			Scheme:    "https",
			Hostname:  "localhost",
			Port:      3000,
			LoginPath: "/login",
			ErrorPath: "/error",
		},
		CORS: config.CORSConfig{AllowedOrigins: allowedOrigins},
	}
	suite.Require().NoError(cors.InitializeMatcher(testConfig.CORS.AllowedOrigins))
	_ = config.InitializeServerRuntime("", testConfig)

	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockResourceService = resourcemock.NewResourceServiceInterfaceMock(suite.T())
	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockFlowExecService = flowexecmock.NewFlowExecServiceInterfaceMock(suite.T())
}

func (suite *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()

	service, err := Initialize(
		mux, suite.mockInboundClient, suite.mockResourceService,
		suite.mockJWTService, suite.mockFlowExecService, nil,
	)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*AuthorizeServiceInterface)(nil), service)
}

func (suite *InitTestSuite) TestInitialize_RegistersRoutes() {
	mux := http.NewServeMux()

	_, err := Initialize(
		mux, suite.mockInboundClient, suite.mockResourceService,
		suite.mockJWTService, suite.mockFlowExecService, nil,
	)
	assert.NoError(suite.T(), err)

	// Verify that the routes are registered by attempting to get a handler for them.
	// The pattern includes the method because of CORS middleware wrapping.
	_, pattern := mux.Handler(&http.Request{Method: "GET", URL: &url.URL{Path: "/oauth2/authorize"}})
	assert.Contains(suite.T(), pattern, "/oauth2/authorize")

	_, pattern = mux.Handler(&http.Request{Method: "POST", URL: &url.URL{Path: "/oauth2/auth/callback"}})
	assert.Contains(suite.T(), pattern, "/oauth2/auth/callback")

	_, pattern = mux.Handler(&http.Request{Method: "OPTIONS", URL: &url.URL{Path: "/oauth2/auth/callback"}})
	assert.Contains(suite.T(), pattern, "/oauth2/auth/callback")
}

func (suite *InitTestSuite) TestRegisterRoutes_CORSConfiguration() {
	mux := http.NewServeMux()

	_, err := Initialize(
		mux, suite.mockInboundClient, suite.mockResourceService,
		suite.mockJWTService, suite.mockFlowExecService, nil,
	)
	assert.NoError(suite.T(), err)

	testCases := []struct {
		name          string
		method        string
		path          string
		expectAllowed bool
	}{
		{
			name:          "GET /oauth2/authorize allowed",
			method:        "GET",
			path:          "/oauth2/authorize",
			expectAllowed: true,
		},
		{
			name:          "POST /oauth2/auth/callback allowed",
			method:        "POST",
			path:          "/oauth2/auth/callback",
			expectAllowed: true,
		},
		{
			name:          "OPTIONS /oauth2/auth/callback returns no content",
			method:        "OPTIONS",
			path:          "/oauth2/auth/callback",
			expectAllowed: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			req, err := http.NewRequest(tc.method, tc.path, nil)
			assert.NoError(suite.T(), err)

			handler, pattern := mux.Handler(req)

			if tc.expectAllowed {
				assert.Contains(suite.T(), pattern, tc.path, "Route should be registered")
				assert.NotNil(suite.T(), handler, "Handler should be registered")
			}
		})
	}
}

func (suite *InitTestSuite) TestRegisterRoutes_CORSHeaders() {
	mux := http.NewServeMux()

	_, err := Initialize(
		mux, suite.mockInboundClient, suite.mockResourceService,
		suite.mockJWTService, suite.mockFlowExecService, nil,
	)
	assert.NoError(suite.T(), err)

	testCases := []struct {
		name                 string
		method               string
		path                 string
		origin               string
		expectedStatus       int
		expectedAllowMethods string
		expectedAllowHeaders string
	}{
		{
			name:                 "OPTIONS /oauth2/auth/callback returns POST method",
			method:               "OPTIONS",
			path:                 "/oauth2/auth/callback",
			origin:               "https://example.com",
			expectedStatus:       http.StatusNoContent,
			expectedAllowMethods: "POST",
			expectedAllowHeaders: "Content-Type, Authorization",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Allow-Methods/Allow-Headers are preflight-only response headers
			// per the Fetch spec; the request must carry
			// Access-Control-Request-Method to elicit them.
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", "POST")
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			assert.Equal(suite.T(), tc.expectedStatus, rec.Code)
			assert.Equal(suite.T(), tc.expectedAllowMethods, rec.Header().Get("Access-Control-Allow-Methods"))
			assert.Equal(suite.T(), tc.expectedAllowHeaders, rec.Header().Get("Access-Control-Allow-Headers"))
			assert.Equal(suite.T(), "true", rec.Header().Get("Access-Control-Allow-Credentials"))
		})
	}
}

func (suite *InitTestSuite) TestWithFrameProtection() {
	// RFC 9700 §4.16: Authorization servers MUST prevent clickjacking attacks.
	handler := withFrameProtection(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/oauth2/authorize", nil)
	rec := httptest.NewRecorder()

	handler(rec, req)

	assert.Equal(suite.T(), http.StatusOK, rec.Code)
	assert.Equal(suite.T(), "DENY", rec.Header().Get("X-Frame-Options"))
	assert.Equal(suite.T(), "frame-ancestors 'none'", rec.Header().Get("Content-Security-Policy"))
}

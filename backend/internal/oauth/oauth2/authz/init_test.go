// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/actorprovider"
	flowsession "github.com/thunder-id/thunderid/internal/flow/session"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/jti"
	"github.com/thunder-id/thunderid/internal/runtimestore/inmemory"
	"github.com/thunder-id/thunderid/internal/system/config"
	syscontext "github.com/thunder-id/thunderid/internal/system/context"
	"github.com/thunder-id/thunderid/internal/system/transaction"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/flowexecmock"
	"github.com/thunder-id/thunderid/tests/mocks/inboundclientmock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/resourcemock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

type InitTestSuite struct {
	suite.Suite
	mockInboundClient   *inboundclientmock.InboundClientServiceInterfaceMock
	mockEntityProvider  *entityprovidermock.EntityProviderInterfaceMock
	mockResourceService *resourcemock.ResourceServiceInterfaceMock
	mockJWTService      *jwtmock.JWTServiceInterfaceMock
	mockFlowExecService *flowexecmock.FlowExecServiceInterfaceMock
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	// Initialize Runtime config with basic test config
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
			RuntimeTransient: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
		},
		GateClient: engineconfig.GateClientConfig{
			Scheme:    "https",
			Hostname:  "localhost",
			Port:      3000,
			LoginPath: "/login",
			ErrorPath: "/error",
		},
	}
	_ = config.InitializeServerRuntime("", testConfig)

	suite.mockInboundClient = inboundclientmock.NewInboundClientServiceInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
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
		mux,
		actorprovider.Initialize(suite.mockInboundClient, suite.mockEntityProvider, noopAuthnMgr(), nil, nil),
		suite.mockResourceService,
		suite.mockJWTService, suite.mockFlowExecService, nil, nil, nil,
		flowsession.NewHandleTransport(flowsession.TransportConfig{}), nil, nil,
		testhelpers.OAuthConfig(),
		inmemory.Initialize("test-deployment"), transaction.NewNoOpTransactioner(),
		jti.Initialize(inmemory.Initialize("test-deployment")),
		nil,
	)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*AuthorizeServiceInterface)(nil), service)
}

func (suite *InitTestSuite) TestInitialize_RegistersRoutes() {
	mux := http.NewServeMux()

	_, err := Initialize(
		mux,
		actorprovider.Initialize(suite.mockInboundClient, suite.mockEntityProvider, noopAuthnMgr(), nil, nil),
		suite.mockResourceService,
		suite.mockJWTService, suite.mockFlowExecService, nil, nil, nil,
		flowsession.NewHandleTransport(flowsession.TransportConfig{}), nil, nil,
		testhelpers.OAuthConfig(),
		inmemory.Initialize("test-deployment"), transaction.NewNoOpTransactioner(),
		jti.Initialize(inmemory.Initialize("test-deployment")),
		nil,
	)
	assert.NoError(suite.T(), err)

	// Verify that the GET /oauth2/authorize route is registered.
	// POST /oauth2/auth/callback is now registered by the callback package, not authz.
	_, pattern := mux.Handler(&http.Request{Method: "GET", URL: &url.URL{Path: "/oauth2/authorize"}})
	assert.Contains(suite.T(), pattern, "/oauth2/authorize")
}

func (suite *InitTestSuite) TestRegisterRoutes_CORSConfiguration() {
	mux := http.NewServeMux()

	_, err := Initialize(
		mux,
		actorprovider.Initialize(suite.mockInboundClient, suite.mockEntityProvider, noopAuthnMgr(), nil, nil),
		suite.mockResourceService,
		suite.mockJWTService, suite.mockFlowExecService, nil, nil, nil,
		flowsession.NewHandleTransport(flowsession.TransportConfig{}), nil, nil,
		testhelpers.OAuthConfig(),
		inmemory.Initialize("test-deployment"), transaction.NewNoOpTransactioner(),
		jti.Initialize(inmemory.Initialize("test-deployment")),
		nil,
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

func (suite *InitTestSuite) TestRegisterRoutes_OUQualifiedRouteFollowsTheSwitch() {
	for _, enabled := range []bool{true, false} {
		mux := http.NewServeMux()
		registerRoutes(mux, NewAuthorizeHandlerInterfaceMock(suite.T()), enabled)

		_, bare := mux.Handler(httptest.NewRequest(http.MethodGet, "/oauth2/authorize", nil))
		_, qualified := mux.Handler(httptest.NewRequest(http.MethodGet, "/ou/customer-a/oauth2/authorize", nil))

		suite.Equal("GET /oauth2/authorize", bare, "enabled=%t", enabled)
		if enabled {
			suite.Equal("GET /ou/{ouId}/oauth2/authorize", qualified)
		} else {
			suite.Empty(qualified, "no pattern matches the organization-unit-qualified path")
		}
	}
}

// getQualified sends GET /ou/{ouSegment}/oauth2/authorize through the registered route.
func (suite *InitTestSuite) getQualified(handler AuthorizeHandlerInterface, ouSegment string) (
	*httptest.ResponseRecorder, map[string]string) {
	mux := http.NewServeMux()
	registerRoutes(mux, handler, true)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet,
		"/ou/"+ouSegment+"/oauth2/authorize?client_id=test-client-id", nil))

	var body map[string]string
	_ = json.Unmarshal(recorder.Body.Bytes(), &body)
	return recorder, body
}

// The path's organization unit reaches the handler sanitized and without a lookup, so one that does not
// exist is left to the flow start to refuse like any other.
func (suite *InitTestSuite) TestOUQualifiedRoute_PutsTheOrganizationUnitOnTheContext() {
	cases := []struct {
		name      string
		ouSegment string
		wantOUID  string
	}{
		{"an organization unit", "customer-a", "customer-a"},
		{"one padded with spaces", "%20customer-a%20", "customer-a"},
		{"one that does not exist", "no-such-ou", "no-such-ou"},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			handler := NewAuthorizeHandlerInterfaceMock(suite.T())
			var accessingOUID string
			handler.EXPECT().HandleAuthorizeGetRequest(mock.Anything, mock.Anything).
				Run(func(_ http.ResponseWriter, r *http.Request) {
					accessingOUID = syscontext.GetAccessingOUID(r.Context())
				}).Once()

			suite.getQualified(handler, tc.ouSegment)

			suite.Equal(tc.wantOUID, accessingOUID)
		})
	}
}

// A path naming no usable organization unit never reaches the handler, so it cannot fall back to an
// unbounded sign-in.
func (suite *InitTestSuite) TestOUQualifiedRoute_RefusesABlankOrganizationUnit() {
	for _, ouSegment := range []string{"%20", "%09%07"} {
		recorder, body := suite.getQualified(NewAuthorizeHandlerInterfaceMock(suite.T()), ouSegment)

		suite.Equal(http.StatusBadRequest, recorder.Code, ouSegment)
		suite.Equal(oauth2const.ErrorInvalidRequest, body["error"], ouSegment)
	}

	recorder, _ := suite.getQualified(NewAuthorizeHandlerInterfaceMock(suite.T()), "")
	suite.NotEqual(http.StatusOK, recorder.Code, "an empty segment matches no authorize route")
}

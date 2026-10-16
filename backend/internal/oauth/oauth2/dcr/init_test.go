// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package dcr

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/testhelpers"
)

type InitTestSuite struct {
	suite.Suite
	mockAppService *applicationmock.ApplicationServiceInterfaceMock
	mockOUService  *oumock.OrganizationUnitServiceInterfaceMock
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	config.ResetServerRuntime()
	suite.mockAppService = applicationmock.NewApplicationServiceInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config:           config.DataSource{Type: "sqlite", SQLite: config.SQLiteDataSource{Path: "test.db"}},
			RuntimeTransient: config.DataSource{Type: "sqlite", SQLite: config.SQLiteDataSource{Path: "test.db"}},
			Entity:           config.DataSource{Type: "sqlite", SQLite: config.SQLiteDataSource{Path: "test.db"}},
		},
	}
	_ = config.InitializeServerRuntime("", testConfig)
}

func (suite *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()

	err := Initialize(mux, suite.mockAppService, suite.mockOUService, nil, nil, testhelpers.OAuthConfig())

	assert.NoError(suite.T(), err)
}

func (suite *InitTestSuite) TestInitialize_RegistersRoutes() {
	mux := http.NewServeMux()

	err := Initialize(mux, suite.mockAppService, suite.mockOUService, nil, nil, testhelpers.OAuthConfig())
	assert.NoError(suite.T(), err)

	// Verify that the routes are registered by attempting to get a handler for them.
	// The pattern includes the method because of CORS middleware wrapping.
	_, pattern := mux.Handler(&http.Request{Method: "POST", URL: &url.URL{Path: "/oauth2/dcr/register"}})
	assert.Contains(suite.T(), pattern, "/oauth2/dcr/register")

	_, pattern = mux.Handler(&http.Request{Method: "OPTIONS", URL: &url.URL{Path: "/oauth2/dcr/register"}})
	assert.Contains(suite.T(), pattern, "/oauth2/dcr/register")

	// RFC 7592 client configuration endpoint routes.
	for _, method := range []string{"GET", "PUT", "DELETE", "OPTIONS"} {
		_, pattern = mux.Handler(&http.Request{
			Method: method, URL: &url.URL{Path: "/oauth2/dcr/register/client-123"},
		})
		assert.Contains(suite.T(), pattern, "/oauth2/dcr/register/{client_id}",
			"expected %s route for the client configuration endpoint", method)
	}
}

func (suite *InitTestSuite) TestRegisterRoutes_SetsCorrelationID() {
	mux := http.NewServeMux()
	registerRoutes(mux, newDCRHandler(NewDCRServiceInterfaceMock(suite.T()), testhelpers.OAuthConfig()))

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/oauth2/dcr/register"},
		{http.MethodGet, "/oauth2/dcr/register/client-abcd"},
		{http.MethodPut, "/oauth2/dcr/register/client-abcd"},
		{http.MethodDelete, "/oauth2/dcr/register/client-abcd"},
	}
	for _, route := range routes {
		suite.T().Run(route.method, func(t *testing.T) {
			req := httptest.NewRequest(route.method, route.path, nil)
			req.Header.Set(serverconst.CorrelationIDHeaderName, "trace-abcd")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			assert.Equal(t, "trace-abcd", rec.Header().Get(serverconst.CorrelationIDHeaderName))
		})
	}
}

func (suite *InitTestSuite) TestInitialize_ReturnsError_WhenRuntimeTransactionerUnavailable() {
	config.ResetServerRuntime()
	testConfig := &config.Config{
		Database: config.DatabaseConfig{
			Config:           config.DataSource{Type: "sqlite", SQLite: config.SQLiteDataSource{Path: "test.db"}},
			RuntimeTransient: config.DataSource{},
			Entity:           config.DataSource{Type: "sqlite", SQLite: config.SQLiteDataSource{Path: "test.db"}},
		},
	}
	_ = config.InitializeServerRuntime("", testConfig)

	mux := http.NewServeMux()
	err := Initialize(mux, suite.mockAppService, suite.mockOUService, nil, nil, testhelpers.OAuthConfig())

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get runtime transient DB transactioner for DCR")
}

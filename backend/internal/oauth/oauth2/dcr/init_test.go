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

package dcr

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/applicationmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
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
			Config:  config.DataSource{Type: "sqlite", SQLite: config.SQLiteDataSource{Path: "test.db"}},
			Runtime: config.DataSource{Type: "sqlite", SQLite: config.SQLiteDataSource{Path: "test.db"}},
			User:    config.DataSource{Type: "sqlite", SQLite: config.SQLiteDataSource{Path: "test.db"}},
		},
	}
	_ = config.InitializeServerRuntime("", testConfig)
}

func (suite *InitTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize() {
	mux := http.NewServeMux()

	service := Initialize(mux, suite.mockAppService, suite.mockOUService, nil, &MockTransactioner{})

	assert.NotNil(suite.T(), service)
	assert.Implements(suite.T(), (*DCRServiceInterface)(nil), service)
}

func (suite *InitTestSuite) TestInitialize_RegistersRoutes() {
	mux := http.NewServeMux()

	Initialize(mux, suite.mockAppService, suite.mockOUService, nil, &MockTransactioner{})

	// Verify that the routes are registered by attempting to get a handler for them.
	// The pattern includes the method because of CORS middleware wrapping.
	_, pattern := mux.Handler(&http.Request{Method: "POST", URL: &url.URL{Path: "/oauth2/dcr/register"}})
	assert.Contains(suite.T(), pattern, "/oauth2/dcr/register")

	_, pattern = mux.Handler(&http.Request{Method: "OPTIONS", URL: &url.URL{Path: "/oauth2/dcr/register"}})
	assert.Contains(suite.T(), pattern, "/oauth2/dcr/register")
}

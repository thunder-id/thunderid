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

package ou

import (
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type InitTestSuite struct {
	suite.Suite
}

func TestInitTestSuite(t *testing.T) {
	suite.Run(t, new(InitTestSuite))
}

func (suite *InitTestSuite) SetupTest() {
	// Initialize server runtime for each test
	config.ResetServerRuntime()
	tmpDir := suite.T().TempDir()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
		Database: config.DatabaseConfig{
			User: config.DataSource{
				Type:   "sqlite",
				SQLite: config.SQLiteDataSource{Path: "test.db"},
			},
		},
	}
	err := config.InitializeServerRuntime(tmpDir, testConfig)
	suite.Require().NoError(err)
}

func (suite *InitTestSuite) TearDownTest() {
	// Clean up after each test
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize_WithDeclarativeResourcesDisabled() {
	// Setup: Disable declarative resources
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = false

	mux := http.NewServeMux()

	// Execute
	service, resolver, exporter, err := Initialize(mux, nil, nil, nil)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.NotNil(suite.T(), resolver)
	assert.NotNil(suite.T(), exporter)

	// Verify exporter is properly created
	assert.Equal(suite.T(), "organization_unit", exporter.GetResourceType())
	assert.Equal(suite.T(), "OrganizationUnit", exporter.GetParameterizerType())
}

func (suite *InitTestSuite) TestInitialize_WithDeclarativeResourcesEnabled() {
	// Setup: Enable declarative resources
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = true

	mux := http.NewServeMux()

	// Execute
	service, resolver, exporter, err := Initialize(mux, nil, nil, nil)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.NotNil(suite.T(), resolver)
	assert.NotNil(suite.T(), exporter)

	// Verify exporter is properly created
	assert.Equal(suite.T(), "organization_unit", exporter.GetResourceType())
	assert.Equal(suite.T(), "OrganizationUnit", exporter.GetParameterizerType())
}

func (suite *InitTestSuite) TestInitialize_FileBasedStoreCreation() {
	// Setup: Enable declarative resources
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = true

	mux := http.NewServeMux()

	// Execute
	service, resolver, exporter, err := Initialize(mux, nil, nil, nil)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.NotNil(suite.T(), resolver)
	assert.NotNil(suite.T(), exporter)

	// Test that the service works (would use file-based store)
	// We can't directly verify the store type, but we can check the service is functional
	assert.NotNil(suite.T(), service)
}

func (suite *InitTestSuite) TestInitialize_DatabaseStoreCreation() {
	// Setup: Disable declarative resources (uses database store)
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = false

	mux := http.NewServeMux()

	// Execute
	service, resolver, exporter, err := Initialize(mux, nil, nil, nil)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.NotNil(suite.T(), resolver)
	assert.NotNil(suite.T(), exporter)
}

func (suite *InitTestSuite) TestInitialize_RoutesRegistered() {
	// Setup
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = false

	mux := http.NewServeMux()

	// Execute
	service, resolver, exporter, err := Initialize(mux, nil, nil, nil)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.NotNil(suite.T(), resolver)
	assert.NotNil(suite.T(), exporter)

	// Verify routes are registered by checking if requests can be matched
	// Note: We can't easily verify all routes without making actual HTTP requests,
	// but we can verify the function completed successfully
}

func (suite *InitTestSuite) TestInitialize_ExporterInterfaceCompliance() {
	// Setup
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = false

	mux := http.NewServeMux()

	// Execute
	service, resolver, exporter, err := Initialize(mux, nil, nil, nil)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.NotNil(suite.T(), resolver)

	// Verify exporter implements ResourceExporter interface
	var _ declarativeresource.ResourceExporter = exporter

	// Test exporter methods
	assert.Equal(suite.T(), "organization_unit", exporter.GetResourceType())
	assert.Equal(suite.T(), "OrganizationUnit", exporter.GetParameterizerType())

	rules := exporter.GetResourceRules()
	assert.NotNil(suite.T(), rules)
	assert.Empty(suite.T(), rules.Variables)
	assert.Empty(suite.T(), rules.ArrayVariables)
}

func (suite *InitTestSuite) TestInitialize_ServiceInterfaceCompliance() {
	// Setup
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = false

	mux := http.NewServeMux()

	// Execute
	service, resolver, exporter, err := Initialize(mux, nil, nil, nil)

	// Assert
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resolver)
	assert.NotNil(suite.T(), exporter)

	// Verify service implements OrganizationUnitServiceInterface
	var _ OrganizationUnitServiceInterface = service
}

func (suite *InitTestSuite) TestInitialize_MultipleInitializations() {
	// Test that multiple initializations work (e.g., for testing scenarios)
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = false

	mux1 := http.NewServeMux()
	service1, resolver1, exporter1, err1 := Initialize(mux1, nil, nil, nil)
	assert.NoError(suite.T(), err1)
	assert.NotNil(suite.T(), service1)
	assert.NotNil(suite.T(), resolver1)
	assert.NotNil(suite.T(), exporter1)

	mux2 := http.NewServeMux()
	service2, resolver2, exporter2, err2 := Initialize(mux2, nil, nil, nil)
	assert.NoError(suite.T(), err2)
	assert.NotNil(suite.T(), service2)
	assert.NotNil(suite.T(), resolver2)
	assert.NotNil(suite.T(), exporter2)

	// Services should be different instances
	assert.NotSame(suite.T(), service1, service2)
	assert.NotSame(suite.T(), exporter1, exporter2)
}

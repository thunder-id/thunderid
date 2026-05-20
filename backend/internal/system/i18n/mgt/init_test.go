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

package mgt

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
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
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: false,
		},
	}
	_ = config.InitializeServerRuntime("/tmp/test", testConfig)
}

func (suite *InitTestSuite) TearDownTest() {
	// Clean up after each test
	config.ResetServerRuntime()
}

func (suite *InitTestSuite) TestInitialize_MutableMode() {
	// Setup
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = false

	mux := http.NewServeMux()

	// Execute
	service, exporter, err := Initialize(mux)

	// Verify
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.NotNil(suite.T(), exporter)

	// Verify exporter is properly created
	assert.Equal(suite.T(), "translation", exporter.GetResourceType())
	assert.Equal(suite.T(), "Translation", exporter.GetParameterizerType())
}

func (suite *InitTestSuite) TestInitialize_DeclarativeMode() {
	// Setup
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = true

	mux := http.NewServeMux()

	// Execute
	service, exporter, err := Initialize(mux)

	// Verify
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), service)
	assert.NotNil(suite.T(), exporter)

	// Verify exporter is properly created
	assert.Equal(suite.T(), "translation", exporter.GetResourceType())
	assert.Equal(suite.T(), "Translation", exporter.GetParameterizerType())
}

func (suite *InitTestSuite) TestInitialize_DeclarativeMode_LoadError() {
	// Setup temp dir for resources
	tempDir, err := os.MkdirTemp("", "test_resources")
	suite.NoError(err)
	defer func() {
		_ = os.RemoveAll(tempDir)
	}()

	// Create translations directory
	translationsDir := filepath.Join(tempDir, "repository", "resources", "translations")
	err = os.MkdirAll(translationsDir, 0750)
	suite.NoError(err)

	// Create invalid YAML file
	invalidFile := filepath.Join(translationsDir, "invalid.yaml")
	err = os.WriteFile(invalidFile, []byte("invalid_yaml: ["), 0600)
	suite.NoError(err)

	// Setup Runtime with temp dir
	config.ResetServerRuntime()
	testConfig := &config.Config{
		DeclarativeResources: config.DeclarativeResources{
			Enabled: true,
		},
	}
	_ = config.InitializeServerRuntime(tempDir, testConfig)

	mux := http.NewServeMux()

	// Execute
	service, exporter, err := Initialize(mux)

	// Verify
	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), service)
	assert.Nil(suite.T(), exporter)
}

func (suite *InitTestSuite) TestRegisterRoutes() {
	mux := http.NewServeMux()
	store := newI18nStoreInterfaceMock(suite.T())
	service := newI18nService(store)
	handler := newI18nHandler(service)

	registerRoutes(mux, handler)

	// Helper to check route registration
	checkRoute := func(method, path string) {
		req, _ := http.NewRequest(method, path, nil)
		_, pattern := mux.Handler(req)
		assert.NotEmpty(suite.T(), pattern, "Route %s %s should be registered", method, path)
	}

	checkRoute("GET", "/i18n/languages")
	checkRoute("OPTIONS", "/i18n/languages")
	checkRoute("GET", "/i18n/languages/en/translations/resolve")
	checkRoute("OPTIONS", "/i18n/languages/en/translations/resolve")
	checkRoute("POST", "/i18n/languages/en/translations")
	checkRoute("DELETE", "/i18n/languages/en/translations")
	checkRoute("OPTIONS", "/i18n/languages/en/translations")
	checkRoute("GET", "/i18n/languages/en/translations/ns/ns1/keys/k1/resolve")
	checkRoute("OPTIONS", "/i18n/languages/en/translations/ns/ns1/keys/k1/resolve")
	checkRoute("POST", "/i18n/languages/en/translations/ns/ns1/keys/k1")
	checkRoute("DELETE", "/i18n/languages/en/translations/ns/ns1/keys/k1")
	checkRoute("OPTIONS", "/i18n/languages/en/translations/ns/ns1/keys/k1")
}

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

package application

import (
	"testing"

	"github.com/stretchr/testify/suite"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// ConfigTestSuite tests application configuration functions.
type ConfigTestSuite struct {
	suite.Suite
}

// SetupSuite sets up the test suite once.
func (suite *ConfigTestSuite) SetupSuite() {
	// Initialize runtime once for all tests in the suite
	testConfig := &config.Config{}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		suite.Fail("Failed to initialize runtime", err)
	}
}

// SetupTest sets up the test environment before each test.
func (suite *ConfigTestSuite) SetupTest() {
	// Reset config before each test
	runtime := config.GetServerRuntime()
	runtime.Config.Application.Store = ""
	runtime.Config.DeclarativeResources.Enabled = false
}

// TestGetApplicationStoreMode tests the store mode resolution logic.
func (suite *ConfigTestSuite) TestGetApplicationStoreMode() {
	testCases := []struct {
		name         string
		appStore     string
		declEnabled  bool
		expectedMode serverconst.StoreMode
		description  string
	}{
		{
			name:         "explicit mutable mode",
			appStore:     "mutable",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when application.store is explicitly set to 'mutable'",
		},
		{
			name:         "explicit declarative mode",
			appStore:     "declarative",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when application.store is explicitly set to 'declarative'",
		},
		{
			name:         "explicit composite mode",
			appStore:     "composite",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeComposite,
			description:  "when application.store is explicitly set to 'composite'",
		},
		{
			name:         "case insensitive mutable",
			appStore:     "MUTABLE",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when application.store is case-insensitive",
		},
		{
			name:         "case insensitive declarative",
			appStore:     "DECLARATIVE",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when application.store is case-insensitive",
		},
		{
			name:         "case insensitive composite",
			appStore:     "Composite",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeComposite,
			description:  "when application.store is case-insensitive",
		},
		{
			name:         "trimmed whitespace mutable",
			appStore:     "  mutable  ",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when application.store has leading/trailing whitespace",
		},
		{
			name:         "fallback to declarative when enabled",
			appStore:     "",
			declEnabled:  true,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when application.store is empty and declarative_resources.enabled is true",
		},
		{
			name:         "fallback to mutable when disabled",
			appStore:     "",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when application.store is empty and declarative_resources.enabled is false",
		},
		{
			name:         "explicit config takes precedence over global setting",
			appStore:     "mutable",
			declEnabled:  true,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when application.store is set, it takes precedence over global setting",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Application.Store = tc.appStore
			runtime.Config.DeclarativeResources.Enabled = tc.declEnabled

			// Execute
			mode := getApplicationStoreMode()

			// Assert
			suite.Equal(tc.expectedMode, mode, tc.description)
		})
	}
}

// TestIsDeclarativeModeEnabled tests checking if declarative mode is enabled.
func (suite *ConfigTestSuite) TestIsDeclarativeModeEnabled() {
	testCases := []struct {
		name           string
		appStore       string
		declEnabled    bool
		expectedResult bool
		description    string
	}{
		{
			name:           "returns true when application.store is declarative",
			appStore:       "declarative",
			declEnabled:    false,
			expectedResult: true,
			description:    "when application.store is explicitly set to 'declarative'",
		},
		{
			name:           "returns false when application.store is mutable",
			appStore:       "mutable",
			declEnabled:    true,
			expectedResult: false,
			description:    "when application.store is explicitly set to 'mutable'",
		},
		{
			name:           "returns false when application.store is composite",
			appStore:       "composite",
			declEnabled:    false,
			expectedResult: false,
			description:    "when application.store is explicitly set to 'composite'",
		},
		{
			name:           "returns true when global declarative is enabled and no explicit config",
			appStore:       "",
			declEnabled:    true,
			expectedResult: true,
			description:    "when global declarative_resources.enabled is true",
		},
		{
			name:           "returns false when global declarative is disabled and no explicit config",
			appStore:       "",
			declEnabled:    false,
			expectedResult: false,
			description:    "when global declarative_resources.enabled is false",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Application.Store = tc.appStore
			runtime.Config.DeclarativeResources.Enabled = tc.declEnabled

			// Execute
			result := isDeclarativeModeEnabled()

			// Assert
			suite.Equal(tc.expectedResult, result, tc.description)
		})
	}
}

// TestGetApplicationStoreMode_InvalidConfig tests invalid configuration handling.
func (suite *ConfigTestSuite) TestGetApplicationStoreMode_InvalidConfig() {
	testCases := []struct {
		name         string
		appStore     string
		declEnabled  bool
		expectedMode serverconst.StoreMode
		description  string
	}{
		{
			name:         "invalid value falls back to global setting",
			appStore:     "invalid",
			declEnabled:  true,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when application.store has an invalid value, fall back to global setting",
		},
		{
			name:         "invalid value falls back to mutable when global disabled",
			appStore:     "invalid",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when application.store has an invalid value and global disabled",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Application.Store = tc.appStore
			runtime.Config.DeclarativeResources.Enabled = tc.declEnabled

			// Execute
			mode := getApplicationStoreMode()

			// Assert
			suite.Equal(tc.expectedMode, mode, tc.description)
		})
	}
}

// TestConfigTestSuite runs the configuration test suite.
func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

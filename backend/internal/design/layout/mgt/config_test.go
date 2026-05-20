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

package layoutmgt

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

// ConfigTestSuite tests layout configuration functions.
type ConfigTestSuite struct {
	suite.Suite
}

// SetupSuite sets up the test suite once.
func (suite *ConfigTestSuite) SetupSuite() {
	// Reset server runtime to ensure clean state
	config.ResetServerRuntime()
	// Initialize runtime once for all tests in the suite
	testConfig := &config.Config{}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		suite.Fail("Failed to initialize runtime", err)
	}
}

// TearDownSuite cleans up after the test suite.
func (suite *ConfigTestSuite) TearDownSuite() {
	// Reset server runtime to avoid state leakage to other test suites
	config.ResetServerRuntime()
}

// SetupTest sets up the test environment before each test.
func (suite *ConfigTestSuite) SetupTest() {
	// Reset config before each test
	runtime := config.GetServerRuntime()
	runtime.Config.Layout.Store = ""
	runtime.Config.DeclarativeResources.Enabled = false
}

// TestGetLayoutStoreMode tests the store mode resolution logic.
func (suite *ConfigTestSuite) TestGetLayoutStoreMode() {
	testCases := []struct {
		name         string
		layoutStore  string
		declEnabled  bool
		expectedMode serverconst.StoreMode
		description  string
	}{
		{
			name:         "explicit mutable mode",
			layoutStore:  "mutable",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when layout.store is explicitly set to 'mutable'",
		},
		{
			name:         "explicit declarative mode",
			layoutStore:  "declarative",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when layout.store is explicitly set to 'declarative'",
		},
		{
			name:         "explicit composite mode",
			layoutStore:  "composite",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeComposite,
			description:  "when layout.store is explicitly set to 'composite'",
		},
		{
			name:         "case insensitive mutable",
			layoutStore:  "MUTABLE",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when layout.store is case-insensitive",
		},
		{
			name:         "case insensitive declarative",
			layoutStore:  "DECLARATIVE",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when layout.store is case-insensitive",
		},
		{
			name:         "case insensitive composite",
			layoutStore:  "Composite",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeComposite,
			description:  "when layout.store is case-insensitive",
		},
		{
			name:         "trimmed whitespace mutable",
			layoutStore:  "  mutable  ",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when layout.store has leading/trailing whitespace",
		},
		{
			name:         "fallback to declarative when enabled",
			layoutStore:  "",
			declEnabled:  true,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when layout.store is empty and declarative_resources.enabled is true",
		},
		{
			name:         "fallback to mutable when disabled",
			layoutStore:  "",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when layout.store is empty and declarative_resources.enabled is false",
		},
		{
			name:         "explicit config takes precedence over global setting",
			layoutStore:  "mutable",
			declEnabled:  true,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when layout.store is set, it takes precedence over global setting",
		},
		{
			name:         "invalid mode fallback to mutable",
			layoutStore:  "invalid_mode",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when layout.store has invalid value, fallback to global setting (mutable)",
		},
		{
			name:         "invalid mode fallback to declarative",
			layoutStore:  "unknown_value",
			declEnabled:  true,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when layout.store has invalid value, fallback to global declarative setting",
		},
		{
			name:         "whitespace only fallback",
			layoutStore:  "   ",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when layout.store contains only whitespace, treat as empty",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Layout.Store = tc.layoutStore
			runtime.Config.DeclarativeResources.Enabled = tc.declEnabled

			// Execute
			mode := getLayoutStoreMode()

			// Assert
			suite.Equal(tc.expectedMode, mode, tc.description)
		})
	}
}

// TestIsDeclarativeModeEnabled tests the declarative mode check.
func (suite *ConfigTestSuite) TestIsDeclarativeModeEnabled() {
	testCases := []struct {
		name         string
		layoutStore  string
		declEnabled  bool
		expectedMode bool
		description  string
	}{
		{
			name:         "declarative mode enabled",
			layoutStore:  "declarative",
			declEnabled:  false,
			expectedMode: true,
			description:  "when layout.store is set to 'declarative'",
		},
		{
			name:         "mutable mode",
			layoutStore:  "mutable",
			declEnabled:  false,
			expectedMode: false,
			description:  "when layout.store is set to 'mutable'",
		},
		{
			name:         "composite mode",
			layoutStore:  "composite",
			declEnabled:  false,
			expectedMode: false,
			description:  "when layout.store is set to 'composite'",
		},
		{
			name:         "fallback to global declarative enabled",
			layoutStore:  "",
			declEnabled:  true,
			expectedMode: true,
			description:  "when layout.store is empty and global declarative is enabled",
		},
		{
			name:         "fallback to global mutable disabled",
			layoutStore:  "",
			declEnabled:  false,
			expectedMode: false,
			description:  "when layout.store is empty and global declarative is disabled",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Layout.Store = tc.layoutStore
			runtime.Config.DeclarativeResources.Enabled = tc.declEnabled

			// Execute
			mode := isDeclarativeModeEnabled()

			// Assert
			suite.Equal(tc.expectedMode, mode, tc.description)
		})
	}
}

// TestConfigTestSuite runs the config test suite.
func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

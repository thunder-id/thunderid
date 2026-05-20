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

package role

import (
	"testing"

	"github.com/stretchr/testify/suite"

	serverconst "github.com/thunder-id/thunderid/internal/system/constants"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// ConfigTestSuite tests role configuration functions.
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
	runtime.Config.Role.Store = ""
	runtime.Config.DeclarativeResources.Enabled = false
}

// TestGetRoleStoreMode tests the store mode resolution logic.
func (suite *ConfigTestSuite) TestGetRoleStoreMode() {
	testCases := []struct {
		name         string
		roleStore    string
		declEnabled  bool
		expectedMode serverconst.StoreMode
		description  string
	}{
		{
			name:         "explicit mutable mode",
			roleStore:    "mutable",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when role.store is explicitly set to 'mutable'",
		},
		{
			name:         "explicit declarative mode",
			roleStore:    "declarative",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when role.store is explicitly set to 'declarative'",
		},
		{
			name:         "explicit composite mode",
			roleStore:    "composite",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeComposite,
			description:  "when role.store is explicitly set to 'composite'",
		},
		{
			name:         "case insensitive mutable",
			roleStore:    "MUTABLE",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when role.store is case-insensitive",
		},
		{
			name:         "case insensitive declarative",
			roleStore:    "DECLARATIVE",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when role.store is case-insensitive",
		},
		{
			name:         "case insensitive composite",
			roleStore:    "Composite",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeComposite,
			description:  "when role.store is case-insensitive",
		},
		{
			name:         "trimmed whitespace mutable",
			roleStore:    "  mutable  ",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when role.store has leading/trailing whitespace",
		},
		{
			name:         "fallback to declarative when enabled",
			roleStore:    "",
			declEnabled:  true,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when role.store is empty and declarative_resources.enabled is true",
		},
		{
			name:         "fallback to mutable when disabled",
			roleStore:    "",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when role.store is empty and declarative_resources.enabled is false",
		},
		{
			name:         "explicit config takes precedence over global setting",
			roleStore:    "mutable",
			declEnabled:  true,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when role.store is set, it takes precedence over global setting",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Role.Store = tc.roleStore
			runtime.Config.DeclarativeResources.Enabled = tc.declEnabled

			// Execute
			mode := getRoleStoreMode()

			// Assert
			suite.Equal(tc.expectedMode, mode, tc.description)
		})
	}
}

// TestIsDeclarativeModeEnabled tests the declarative mode check.
func (suite *ConfigTestSuite) TestIsDeclarativeModeEnabled() {
	testCases := []struct {
		name         string
		roleStore    string
		declEnabled  bool
		expectedMode bool
		description  string
	}{
		{
			name:         "declarative mode enabled",
			roleStore:    "declarative",
			declEnabled:  false,
			expectedMode: true,
			description:  "when role.store is set to 'declarative'",
		},
		{
			name:         "mutable mode",
			roleStore:    "mutable",
			declEnabled:  false,
			expectedMode: false,
			description:  "when role.store is set to 'mutable'",
		},
		{
			name:         "composite mode",
			roleStore:    "composite",
			declEnabled:  false,
			expectedMode: false,
			description:  "when role.store is set to 'composite'",
		},
		{
			name:         "fallback to global declarative enabled",
			roleStore:    "",
			declEnabled:  true,
			expectedMode: true,
			description:  "when role.store is empty and global declarative is enabled",
		},
		{
			name:         "fallback to global mutable disabled",
			roleStore:    "",
			declEnabled:  false,
			expectedMode: false,
			description:  "when role.store is empty and global declarative is disabled",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Role.Store = tc.roleStore
			runtime.Config.DeclarativeResources.Enabled = tc.declEnabled

			// Execute
			mode := isDeclarativeModeEnabled()

			// Assert
			suite.Equal(tc.expectedMode, mode, tc.description)
		})
	}
}

// TestInvalidConfigurationFallback tests invalid configuration fallback behavior.
func (suite *ConfigTestSuite) TestInvalidConfigurationFallback() {
	testCases := []struct {
		name         string
		roleStore    string
		declEnabled  bool
		expectedMode serverconst.StoreMode
		description  string
	}{
		{
			name:         "invalid mode fallback to mutable",
			roleStore:    "invalid",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when role.store has invalid value, fallback to global setting",
		},
		{
			name:         "invalid mode fallback to declarative",
			roleStore:    "invalid",
			declEnabled:  true,
			expectedMode: serverconst.StoreModeDeclarative,
			description:  "when role.store has invalid value, fallback to global declarative setting",
		},
		{
			name:         "whitespace only fallback",
			roleStore:    "   ",
			declEnabled:  false,
			expectedMode: serverconst.StoreModeMutable,
			description:  "when role.store contains only whitespace, treat as empty",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Role.Store = tc.roleStore
			runtime.Config.DeclarativeResources.Enabled = tc.declEnabled

			// Execute
			mode := getRoleStoreMode()

			// Assert
			suite.Equal(tc.expectedMode, mode, tc.description)
		})
	}
}

// TestRoleConfigCaseSensitivity tests case sensitivity handling.
func (suite *ConfigTestSuite) TestRoleConfigCaseSensitivity() {
	testCases := []struct {
		name         string
		roleStore    string
		expectedMode serverconst.StoreMode
	}{
		{"lowercase mutable", "mutable", serverconst.StoreModeMutable},
		{"uppercase mutable", "MUTABLE", serverconst.StoreModeMutable},
		{"mixed case mutable", "MuTaBlE", serverconst.StoreModeMutable},
		{"lowercase declarative", "declarative", serverconst.StoreModeDeclarative},
		{"uppercase declarative", "DECLARATIVE", serverconst.StoreModeDeclarative},
		{"mixed case declarative", "DeCLaRaTivE", serverconst.StoreModeDeclarative},
		{"lowercase composite", "composite", serverconst.StoreModeComposite},
		{"uppercase composite", "COMPOSITE", serverconst.StoreModeComposite},
		{"mixed case composite", "CoMpOsItE", serverconst.StoreModeComposite},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Role.Store = tc.roleStore
			runtime.Config.DeclarativeResources.Enabled = false

			// Execute
			mode := getRoleStoreMode()

			// Assert
			suite.Equal(tc.expectedMode, mode)
		})
	}
}

// TestRoleConfigWhitespaceTrimming tests whitespace trimming.
func (suite *ConfigTestSuite) TestRoleConfigWhitespaceTrimming() {
	testCases := []struct {
		name         string
		roleStore    string
		expectedMode serverconst.StoreMode
	}{
		{"leading whitespace", "  mutable", serverconst.StoreModeMutable},
		{"trailing whitespace", "mutable  ", serverconst.StoreModeMutable},
		{"both whitespace", "  mutable  ", serverconst.StoreModeMutable},
		{"tab whitespace", "\tmutable\t", serverconst.StoreModeMutable},
		{"mixed whitespace", " \t  declarative  \t ", serverconst.StoreModeDeclarative},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Role.Store = tc.roleStore
			runtime.Config.DeclarativeResources.Enabled = false

			// Execute
			mode := getRoleStoreMode()

			// Assert
			suite.Equal(tc.expectedMode, mode)
		})
	}
}

// TestRoleConfigPrecedence tests service-level precedence over global setting.
func (suite *ConfigTestSuite) TestRoleConfigPrecedence() {
	testCases := []struct {
		name          string
		roleStore     string
		globalDeclare bool
		expectedMode  serverconst.StoreMode
		description   string
	}{
		{
			name:          "service mutable overrides global declarative",
			roleStore:     "mutable",
			globalDeclare: true,
			expectedMode:  serverconst.StoreModeMutable,
			description:   "service-level config should take precedence",
		},
		{
			name:          "service declarative overrides global mutable",
			roleStore:     "declarative",
			globalDeclare: false,
			expectedMode:  serverconst.StoreModeDeclarative,
			description:   "service-level config should take precedence",
		},
		{
			name:          "service composite overrides global settings",
			roleStore:     "composite",
			globalDeclare: true,
			expectedMode:  serverconst.StoreModeComposite,
			description:   "service-level config should take precedence",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Setup
			runtime := config.GetServerRuntime()
			runtime.Config.Role.Store = tc.roleStore
			runtime.Config.DeclarativeResources.Enabled = tc.globalDeclare

			// Execute
			mode := getRoleStoreMode()

			// Assert
			suite.Equal(tc.expectedMode, mode, tc.description)
		})
	}
}

// TestSuite runs the config test suite.
func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

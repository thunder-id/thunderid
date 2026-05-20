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

package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
	declarativeresource "github.com/thunder-id/thunderid/internal/system/declarative_resource"
)

const testStoreModeComposite = "composite"

// ResourceConfigTestSuite tests resource configuration.
type ResourceConfigTestSuite struct {
	suite.Suite
	// Store original config values for restoration after each test
	originalResourceStore      string
	originalDeclarativeEnabled bool
}

func TestResourceConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ResourceConfigTestSuite))
}

func (s *ResourceConfigTestSuite) SetupSuite() {
	// Initialize runtime once for all tests in the suite
	testConfig := &config.Config{}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		s.Fail("Failed to initialize runtime", err)
	}
}

func (s *ResourceConfigTestSuite) SetupTest() {
	// Capture original config values for restoration
	runtime := config.GetServerRuntime()
	s.originalResourceStore = runtime.Config.Resource.Store
	s.originalDeclarativeEnabled = runtime.Config.DeclarativeResources.Enabled

	// Reset config before each test
	runtime.Config.Resource.Store = ""
	runtime.Config.DeclarativeResources.Enabled = false
}

func (s *ResourceConfigTestSuite) TearDownTest() {
	// Restore original config values after each test to prevent mutation of shared state
	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = s.originalResourceStore
	runtime.Config.DeclarativeResources.Enabled = s.originalDeclarativeEnabled
}

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_DeclarativeModeEnabled() {
	// Set up config with declarative mode enabled
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = true

	mode := getResourceStoreMode()

	assert.Equal(s.T(), serverconst.StoreModeDeclarative, mode)
}

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_DeclarativeModeDisabled() {
	// Set up config with declarative mode disabled
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = false

	mode := getResourceStoreMode()

	assert.Equal(s.T(), serverconst.StoreModeMutable, mode)
}

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_DefaultMutable() {
	// Use default config (should be mutable)
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = false

	mode := getResourceStoreMode()

	// When declarative is not explicitly enabled, should be mutable
	assert.Equal(s.T(), serverconst.StoreModeMutable, mode)
}

// Test integration with declarativeresource package

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_WithDeclarativeResourceCheck() {
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = true

	// Verify IsDeclarativeModeEnabled returns true
	enabled := declarativeresource.IsDeclarativeModeEnabled()
	assert.True(s.T(), enabled)

	mode := getResourceStoreMode()
	assert.Equal(s.T(), serverconst.StoreModeDeclarative, mode)
}

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_WithDeclarativeResourceDisabled() {
	runtime := config.GetServerRuntime()
	runtime.Config.DeclarativeResources.Enabled = false

	// Verify IsDeclarativeModeEnabled returns false
	enabled := declarativeresource.IsDeclarativeModeEnabled()
	assert.False(s.T(), enabled)

	mode := getResourceStoreMode()
	assert.Equal(s.T(), serverconst.StoreModeMutable, mode)
}

// Test future service-level config (when Resource.Store is added)

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_ServiceLevelMutable() {
	// Service-level config should take precedence over global setting
	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = "mutable"
	runtime.Config.DeclarativeResources.Enabled = true // Global is declarative

	mode := getResourceStoreMode()

	// Should use service-level mutable, not global declarative
	assert.Equal(s.T(), serverconst.StoreModeMutable, mode)
}

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_ServiceLevelDeclarative() {
	// Service-level config should take precedence over global setting
	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = "declarative"
	runtime.Config.DeclarativeResources.Enabled = false // Global is mutable

	mode := getResourceStoreMode()

	// Should use service-level declarative, not global mutable
	assert.Equal(s.T(), serverconst.StoreModeDeclarative, mode)
}

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_ServiceLevelComposite() {
	// Composite mode allows both declarative and mutable resources
	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = testStoreModeComposite
	runtime.Config.DeclarativeResources.Enabled = false

	mode := getResourceStoreMode()

	assert.Equal(s.T(), serverconst.StoreModeComposite, mode)
}

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_InvalidServiceLevelFallsBack() {
	// Invalid service-level config should fall back to global setting
	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = "invalid-mode"
	runtime.Config.DeclarativeResources.Enabled = true

	mode := getResourceStoreMode()

	// Should fall back to global declarative
	assert.Equal(s.T(), serverconst.StoreModeDeclarative, mode)
}

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_CaseInsensitive() {
	// Store mode should be case-insensitive
	testCases := []struct {
		name         string
		storeValue   string
		expectedMode serverconst.StoreMode
	}{
		{
			name:         "Uppercase MUTABLE",
			storeValue:   "MUTABLE",
			expectedMode: serverconst.StoreModeMutable,
		},
		{
			name:         "Mixed case Declarative",
			storeValue:   "Declarative",
			expectedMode: serverconst.StoreModeDeclarative,
		},
		{
			name:         "Uppercase COMPOSITE",
			storeValue:   "COMPOSITE",
			expectedMode: serverconst.StoreModeComposite,
		},
		{
			name:         "With whitespace",
			storeValue:   "  mutable  ",
			expectedMode: serverconst.StoreModeMutable,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			runtime := config.GetServerRuntime()
			runtime.Config.Resource.Store = tc.storeValue
			runtime.Config.DeclarativeResources.Enabled = false

			mode := getResourceStoreMode()

			assert.Equal(s.T(), tc.expectedMode, mode)
		})
	}
}

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_EmptyStringFallsBack() {
	// Empty string should fall back to global setting
	runtime := config.GetServerRuntime()
	runtime.Config.Resource.Store = ""
	runtime.Config.DeclarativeResources.Enabled = true

	mode := getResourceStoreMode()

	// Should fall back to global declarative
	assert.Equal(s.T(), serverconst.StoreModeDeclarative, mode)
}

func (s *ResourceConfigTestSuite) TestGetResourceStoreMode_ValidReturnValues() {
	// Verify that getResourceStoreMode only returns valid store modes

	testCases := []struct {
		name               string
		declarativeEnabled bool
		expectedMode       serverconst.StoreMode
	}{
		{
			name:               "Declarative enabled returns declarative mode",
			declarativeEnabled: true,
			expectedMode:       serverconst.StoreModeDeclarative,
		},
		{
			name:               "Declarative disabled returns mutable mode",
			declarativeEnabled: false,
			expectedMode:       serverconst.StoreModeMutable,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			runtime := config.GetServerRuntime()
			runtime.Config.DeclarativeResources.Enabled = tc.declarativeEnabled

			mode := getResourceStoreMode()

			assert.Equal(s.T(), tc.expectedMode, mode)

			// Verify it's one of the valid modes
			validModes := []serverconst.StoreMode{
				serverconst.StoreModeMutable,
				serverconst.StoreModeDeclarative,
				serverconst.StoreModeComposite,
			}
			assert.Contains(s.T(), validModes, mode)
		})
	}
}

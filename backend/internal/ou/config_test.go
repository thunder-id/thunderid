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
	"testing"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"

	"github.com/stretchr/testify/assert"
)

func TestGetOrganizationUnitStoreMode(t *testing.T) {
	// Initialize runtime with test config
	testConfig := &config.Config{}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		t.Fatalf("Failed to initialize runtime: %v", err)
	}

	tests := []struct {
		name                      string
		ouStoreConfig             string
		immutableResourcesEnabled bool
		expectedMode              serverconst.StoreMode
	}{
		{
			name:                      "explicit mutable mode",
			ouStoreConfig:             "mutable",
			immutableResourcesEnabled: true, // Should be ignored
			expectedMode:              serverconst.StoreModeMutable,
		},
		{
			name:                      "explicit declarative mode",
			ouStoreConfig:             "declarative",
			immutableResourcesEnabled: false, // Should be ignored
			expectedMode:              serverconst.StoreModeDeclarative,
		},
		{
			name:                      "explicit composite mode",
			ouStoreConfig:             "composite",
			immutableResourcesEnabled: false, // Should be ignored
			expectedMode:              serverconst.StoreModeComposite,
		},
		{
			name:                      "uppercase explicit mode",
			ouStoreConfig:             "COMPOSITE",
			immutableResourcesEnabled: false,
			expectedMode:              serverconst.StoreModeComposite,
		},
		{
			name:                      "whitespace in explicit mode",
			ouStoreConfig:             "  mutable  ",
			immutableResourcesEnabled: true,
			expectedMode:              serverconst.StoreModeMutable,
		},
		{
			name:                      "invalid mode falls back to global config - immutable",
			ouStoreConfig:             "invalid",
			immutableResourcesEnabled: true,
			expectedMode:              serverconst.StoreModeDeclarative,
		},
		{
			name:                      "invalid mode falls back to global config - mutable",
			ouStoreConfig:             "invalid",
			immutableResourcesEnabled: false,
			expectedMode:              serverconst.StoreModeMutable,
		},
		{
			name:                      "empty config falls back to global - immutable",
			ouStoreConfig:             "",
			immutableResourcesEnabled: true,
			expectedMode:              serverconst.StoreModeDeclarative,
		},
		{
			name:                      "empty config falls back to global - mutable",
			ouStoreConfig:             "",
			immutableResourcesEnabled: false,
			expectedMode:              serverconst.StoreModeMutable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up test configuration
			runtime := config.GetServerRuntime()
			runtime.Config.OrganizationUnit.Store = tt.ouStoreConfig
			runtime.Config.DeclarativeResources.Enabled = tt.immutableResourcesEnabled

			// Test
			actualMode := getOrganizationUnitStoreMode()
			assert.Equal(t, tt.expectedMode, actualMode)
		})
	}
}

func TestIsCompositeModeEnabled(t *testing.T) {
	testConfig := &config.Config{}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		t.Fatalf("Failed to initialize runtime: %v", err)
	}

	tests := []struct {
		name     string
		mode     string
		expected bool
	}{
		{"composite mode enabled", "composite", true},
		{"mutable mode not composite", "mutable", false},
		{"declarative mode not composite", "declarative", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := config.GetServerRuntime()
			runtime.Config.OrganizationUnit.Store = tt.mode
			runtime.Config.DeclarativeResources.Enabled = false

			assert.Equal(t, tt.expected, isCompositeModeEnabled())
		})
	}
}

func TestIsMutableModeEnabled(t *testing.T) {
	testConfig := &config.Config{}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		t.Fatalf("Failed to initialize runtime: %v", err)
	}

	tests := []struct {
		name     string
		mode     string
		expected bool
	}{
		{"mutable mode enabled", "mutable", true},
		{"composite mode not mutable", "composite", false},
		{"declarative mode not mutable", "declarative", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := config.GetServerRuntime()
			runtime.Config.OrganizationUnit.Store = tt.mode
			runtime.Config.DeclarativeResources.Enabled = false

			assert.Equal(t, tt.expected, isMutableModeEnabled())
		})
	}
}

func TestIsDeclarativeModeEnabled(t *testing.T) {
	testConfig := &config.Config{}
	err := config.InitializeServerRuntime("/tmp/test", testConfig)
	if err != nil {
		t.Fatalf("Failed to initialize runtime: %v", err)
	}

	tests := []struct {
		name     string
		mode     string
		expected bool
	}{
		{"declarative mode enabled", "declarative", true},
		{"mutable mode not immutable", "mutable", false},
		{"composite mode not immutable", "composite", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := config.GetServerRuntime()
			runtime.Config.OrganizationUnit.Store = tt.mode
			runtime.Config.DeclarativeResources.Enabled = false

			assert.Equal(t, tt.expected, isDeclarativeModeEnabled())
		})
	}
}

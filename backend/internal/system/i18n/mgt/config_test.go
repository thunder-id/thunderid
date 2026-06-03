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

package mgt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/system/config"
	serverconst "github.com/thunder-id/thunderid/internal/system/constants"
)

type ConfigTestSuite struct {
	suite.Suite
}

func TestConfigSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (suite *ConfigTestSuite) TestGetI18nStoreMode() {
	t := suite.T()

	tests := []struct {
		name                      string
		translationStore          string
		declarativeModeEnabled    bool
		expectedResolvedStoreMode serverconst.StoreMode
	}{
		{
			name:                      "returns mutable when explicitly configured",
			translationStore:          "mutable",
			declarativeModeEnabled:    true,
			expectedResolvedStoreMode: serverconst.StoreModeMutable,
		},
		{
			name:                      "returns declarative when explicitly configured",
			translationStore:          "declarative",
			declarativeModeEnabled:    false,
			expectedResolvedStoreMode: serverconst.StoreModeDeclarative,
		},
		{
			name:                      "returns composite when explicitly configured",
			translationStore:          "composite",
			declarativeModeEnabled:    false,
			expectedResolvedStoreMode: serverconst.StoreModeComposite,
		},
		{
			name:                      "normalizes casing and spaces of explicit store mode",
			translationStore:          "  CoMpOsItE  ",
			declarativeModeEnabled:    false,
			expectedResolvedStoreMode: serverconst.StoreModeComposite,
		},
		{
			name:                      "falls back to declarative mode when explicit store is invalid",
			translationStore:          "invalid",
			declarativeModeEnabled:    true,
			expectedResolvedStoreMode: serverconst.StoreModeDeclarative,
		},
		{
			name:                      "falls back to mutable when store is empty and declarative mode is disabled",
			translationStore:          "",
			declarativeModeEnabled:    false,
			expectedResolvedStoreMode: serverconst.StoreModeMutable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.ResetServerRuntime()
			t.Cleanup(config.ResetServerRuntime)

			testConfig := &config.Config{
				DeclarativeResources: config.DeclarativeResources{
					Enabled: tt.declarativeModeEnabled,
				},
			}
			err := config.InitializeServerRuntime("/tmp/test", testConfig)
			assert.NoError(t, err)

			resolvedStoreMode := getI18nStoreMode(config.TranslationConfig{
				Store: tt.translationStore,
			})

			assert.Equal(t, tt.expectedResolvedStoreMode, resolvedStoreMode)
		})
	}
}

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

package declarativeresource

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

func TestIsDeclarativeModeEnabled(t *testing.T) {
	t.Run("Returns true when declarative resources are enabled", func(t *testing.T) {
		config.ResetServerRuntime()
		defer config.ResetServerRuntime() // Clean up after test
		testConfig := &config.Config{
			DeclarativeResources: config.DeclarativeResources{
				Enabled: true,
			},
		}
		err := config.InitializeServerRuntime("", testConfig)
		assert.NoError(t, err)

		result := IsDeclarativeModeEnabled()
		assert.True(t, result)
	})

	t.Run("Returns false when declarative resources are disabled", func(t *testing.T) {
		config.ResetServerRuntime()
		defer config.ResetServerRuntime() // Clean up after test
		testConfig := &config.Config{
			DeclarativeResources: config.DeclarativeResources{
				Enabled: false,
			},
		}
		err := config.InitializeServerRuntime("", testConfig)
		assert.NoError(t, err)

		result := IsDeclarativeModeEnabled()
		assert.False(t, result)
	})
}

func TestCheckDeclarativeOperations(t *testing.T) {
	testCases := []struct {
		name            string
		operation       string
		checkFunc       func() *serviceerror.ServiceError
		expectedErrText string
	}{
		{
			name:            "CheckDeclarativeCreate",
			operation:       "create",
			checkFunc:       CheckDeclarativeCreate,
			expectedErrText: "Declarative resource create operation",
		},
		{
			name:            "CheckDeclarativeUpdate",
			operation:       "update",
			checkFunc:       CheckDeclarativeUpdate,
			expectedErrText: "Declarative resource update operation",
		},
		{
			name:            "CheckDeclarativeDelete",
			operation:       "delete",
			checkFunc:       CheckDeclarativeDelete,
			expectedErrText: "Declarative resource delete operation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Returns error when declarative mode is enabled", func(t *testing.T) {
				config.ResetServerRuntime()
				defer config.ResetServerRuntime() // Clean up after test
				testConfig := &config.Config{
					DeclarativeResources: config.DeclarativeResources{
						Enabled: true,
					},
				}
				err := config.InitializeServerRuntime("", testConfig)
				assert.NoError(t, err)

				result := tc.checkFunc()
				assert.NotNil(t, result)
				assert.Contains(t, result.Error.DefaultValue, tc.expectedErrText)
			})

			t.Run("Returns nil when declarative mode is disabled", func(t *testing.T) {
				config.ResetServerRuntime()
				defer config.ResetServerRuntime() // Clean up after test
				testConfig := &config.Config{
					DeclarativeResources: config.DeclarativeResources{
						Enabled: false,
					},
				}
				err := config.InitializeServerRuntime("", testConfig)
				assert.NoError(t, err)

				result := tc.checkFunc()
				assert.Nil(t, result)
			})
		})
	}
}

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

package enginebridge

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/pkg/thunderidengine"
)

func TestNewBridgesFromProviders(t *testing.T) {
	require.NotNil(t, newClientBridge(nil))
	require.NotNil(t, newAuthnBridge(nil))
	require.NotNil(t, newAuthzBridge(nil))
	require.NotNil(t, newResourceBridge(nil))
	require.NotNil(t, newOUBridge(nil))
	require.NotNil(t, newIDPBridge(nil))
	require.NotNil(t, newEntityBridge(nil))
	require.NotNil(t, newObservabilityBridge(nil))
	require.NotNil(t, newExecutorRegistryBridge(nil))
	require.NotNil(t, NewHostRuntimeStores(nil))
}

func TestInitializeRequiresConfigPath(t *testing.T) {
	err := Initialize(thunderidengine.EngineConfig{}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "ConfigPath")
}

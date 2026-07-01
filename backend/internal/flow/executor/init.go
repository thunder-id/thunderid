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

package executor

import (
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Initialize creates an executor registry and registers built-in executors.
// When flowConfig.Executors is empty, all built-in executors are registered.
// When non-empty, only the listed executors are registered; flows using other executors
// will fail validation until those executors are included or the list is cleared.
func Initialize(deps ExecutorDependencies, flowConfig engineconfig.FlowConfig) (ExecutorRegistryInterface, error) {
	reg := newExecutorRegistry()
	names := flowConfig.Executors
	if err := registerBuiltInExecutors(reg, deps, names); err != nil {
		return nil, err
	}
	return reg, nil
}

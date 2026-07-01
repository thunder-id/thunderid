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

// Package interceptor provides the interceptor abstraction for cross-cutting flow concerns.
package interceptor

import (
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
)

// Initialize creates the interceptor registry and registers all built-in interceptors.
func Initialize(
	deps InterceptorDependencies,
	flowConfig engineconfig.FlowConfig,
) (InterceptorRegistryInterface, error) {
	reg := newInterceptorRegistry()
	interceptorNames := flowConfig.Interceptors
	if err := registerInterceptors(deps, reg, interceptorNames); err != nil {
		return nil, err
	}
	// Initialize default interceptorUnits for use in interceptor runner.
	initDefaultInterceptorUnits(deps.FlowFactory)
	return reg, nil
}

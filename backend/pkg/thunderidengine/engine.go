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

package thunderidengine

import "net/http"

// bootstrapFunc wires host providers into internal services. Registered by internal/enginebridge.
type bootstrapFunc func(EngineConfig, *http.ServeMux) error

var registeredBootstrap bootstrapFunc

// RegisterBootstrap registers the module-internal bootstrap implementation.
// It is called from internal/enginebridge and must not be used by embedders.
func RegisterBootstrap(fn bootstrapFunc) {
	registeredBootstrap = fn
}

// Engine wires host providers and internal Thunder services for embeddable OAuth and flow runtime.
type Engine struct {
	config EngineConfig
}

// New constructs an Engine from configuration. Call Initialize to bootstrap services and routes.
func New(config EngineConfig) *Engine {
	return &Engine{config: config}
}

// Initialize loads configuration, wires providers, and registers runtime HTTP routes on mux.
func (e *Engine) Initialize(mux *http.ServeMux) error {
	if registeredBootstrap == nil {
		return ErrNotImplemented
	}
	return registeredBootstrap(e.config, mux)
}

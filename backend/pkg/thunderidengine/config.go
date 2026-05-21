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

// Providers holds optional host implementations. Nil entries use internal Thunder defaults
// when Engine.Initialize runs with full-server configuration.
type Providers struct {
	Client         ClientProvider
	Authn          AuthnProvider
	Authz          AuthzProvider
	Resource       ResourceProvider
	OU             OUProvider
	IDP            IDPProvider
	FlowDefinition FlowDefinitionProvider
	Observability  ObservabilityProvider
	RuntimeStore   RuntimeStore
}

// ExecutorConfig controls built-in and custom executor registration.
type ExecutorConfig struct {
	// Names lists built-in executor names to register. Empty registers the full default set.
	Names []string
	// CustomRegistry replaces the default registry when set.
	CustomRegistry ExecutorRegistry
	// InjectCustom registers additional executors on the default registry.
	InjectCustom []ExecutorInterface
}

// EngineConfig configures Engine initialization.
type EngineConfig struct {
	// ConfigPath is the path to the Thunder server runtime configuration file.
	ConfigPath string
	Providers  Providers
	Executors  ExecutorConfig
	// RegisterRoutes when nil or true registers OAuth and flow runtime routes on Initialize.
	RegisterRoutes *bool
}

// RegisterRoutesEnabled reports whether route registration is enabled (default true).
func (c EngineConfig) RegisterRoutesEnabled() bool {
	if c.RegisterRoutes == nil {
		return true
	}
	return *c.RegisterRoutes
}

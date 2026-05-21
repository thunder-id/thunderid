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

import "fmt"

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
	Design         DesignProvider
	I18n           I18nProvider
	Role           RoleProvider
	Observability  ObservabilityProvider
	RuntimeStore   RuntimeStore
}

// ProvidersComplete reports whether all providers required for host-only bootstrap are set.
func ProvidersComplete(p Providers) bool {
	return p.Client != nil &&
		p.Authn != nil &&
		p.Authz != nil &&
		p.Resource != nil &&
		p.OU != nil &&
		p.IDP != nil &&
		p.FlowDefinition != nil &&
		p.Design != nil &&
		p.I18n != nil &&
		p.Role != nil &&
		p.RuntimeStore != nil
}

// ValidateHostOnlyProviders returns an error listing missing required providers.
func ValidateHostOnlyProviders(p Providers) error {
	var missing []string
	if p.Client == nil {
		missing = append(missing, "Client")
	}
	if p.Authn == nil {
		missing = append(missing, "Authn")
	}
	if p.Authz == nil {
		missing = append(missing, "Authz")
	}
	if p.Resource == nil {
		missing = append(missing, "Resource")
	}
	if p.OU == nil {
		missing = append(missing, "OU")
	}
	if p.IDP == nil {
		missing = append(missing, "IDP")
	}
	if p.FlowDefinition == nil {
		missing = append(missing, "FlowDefinition")
	}
	if p.Design == nil {
		missing = append(missing, "Design")
	}
	if p.I18n == nil {
		missing = append(missing, "I18n")
	}
	if p.Role == nil {
		missing = append(missing, "Role")
	}
	if p.RuntimeStore == nil {
		missing = append(missing, "RuntimeStore")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("thunderidengine: host-only mode requires providers: %v", missing)
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
	// HostOnly when true skips internal domain bootstrap and uses only host providers.
	// When nil and ProvidersComplete is true, host-only mode is enabled automatically.
	HostOnly *bool
	// RegisterRoutes when nil or true registers OAuth and flow runtime routes on Initialize.
	RegisterRoutes *bool
}

// HostOnlyEnabled reports whether host-only bootstrap should run.
func (c EngineConfig) HostOnlyEnabled() bool {
	if c.HostOnly != nil {
		return *c.HostOnly
	}
	return ProvidersComplete(c.Providers)
}

// RegisterRoutesEnabled reports whether route registration is enabled (default true).
func (c EngineConfig) RegisterRoutesEnabled() bool {
	if c.RegisterRoutes == nil {
		return true
	}
	return *c.RegisterRoutes
}

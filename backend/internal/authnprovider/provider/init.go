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

package provider

import (
	"fmt"

	"github.com/thunder-id/thunderid/internal/system/config"
)

// InitializeAuthnProviders constructs every authn provider listed in the catalog
// and returns the resulting map keyed by provider name. Per-provider runtime
// config is pulled from config.AuthnProviderConfig.Properties.
func InitializeAuthnProviders(deps AuthnProviderDependencies) (map[string]AuthnProviderInterface, error) {
	catalog := newBuiltInAuthnProviderRegistrars()
	properties := config.GetServerRuntime().Config.AuthnProvider.Properties

	registered := make(map[string]AuthnProviderInterface, len(catalog))
	for name, registrar := range catalog {
		p, err := registrar(properties[name], deps)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize authn provider %q: %w", name, err)
		}
		// nil with no error => registrar opted out (provider not enabled in this deployment).
		if p == nil {
			continue
		}
		registered[name] = p
	}
	return registered, nil
}

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

package idp

import "github.com/thunder-id/thunderid/internal/integration"

// idpIntegrationInfo holds the catalog presentation details for an IDP type.
type idpIntegrationInfo struct {
	displayName string
	category    string
}

// idpIntegrationInfos maps each supported IDP type to its catalog presentation details.
var idpIntegrationInfos = map[IDPType]idpIntegrationInfo{
	IDPTypeGoogle: {displayName: "Google", category: integration.CategorySocialLogin},
	IDPTypeGitHub: {displayName: "GitHub", category: integration.CategorySocialLogin},
	IDPTypeOAuth:  {displayName: "OAuth 2.0", category: integration.CategoryEnterprise},
	IDPTypeOIDC:   {displayName: "OpenID Connect", category: integration.CategoryEnterprise},
}

// Integrations returns the integration catalog descriptors for all supported
// identity provider types, derived from their property configurations.
func Integrations() []integration.Descriptor {
	descriptors := make([]integration.Descriptor, 0, len(supportedIDPTypes))

	for _, idpType := range supportedIDPTypes {
		info := idpIntegrationInfos[idpType]
		propConfig := idpPropertyConfigs[idpType]

		fields := make([]integration.Field, 0,
			len(propConfig.Required)+len(propConfig.Optional))
		fields = appendIntegrationFields(fields, propConfig.Required, true, propConfig.Defaults)
		fields = appendIntegrationFields(fields, propConfig.Optional, false, propConfig.Defaults)

		descriptors = append(descriptors, integration.Descriptor{
			Type:              string(idpType),
			DisplayName:       info.displayName,
			Category:          info.category,
			HostedCredentials: false,
			Fields:            fields,
		})
	}

	return descriptors
}

// appendIntegrationFields converts property names into integration fields, marking
// secrets and applying any default values.
func appendIntegrationFields(fields []integration.Field, names []string,
	required bool, defaults map[string]string) []integration.Field {
	for _, name := range names {
		fields = append(fields, integration.Field{
			Name:     name,
			Required: required,
			Secret:   name == PropClientSecret,
			Default:  defaults[name],
		})
	}
	return fields
}

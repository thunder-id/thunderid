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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package managedresource

// The resource types recorded in the registry. These are the values the import already reports in its
// per-document outcome, so the marking side and the guarding side name a resource the same way.
const (
	TypeOrganizationUnit        = "organization_unit"
	TypeEntityType              = "user_type"
	TypeResourceServer          = "resource_server"
	TypeRole                    = "role"
	TypeGroup                   = "group"
	TypeConnection              = "connection"
	TypeFlow                    = "flow"
	TypeTheme                   = "theme"
	TypeLayout                  = "layout"
	TypeApplication             = "application"
	TypeUser                    = "user"
	TypeTranslation             = "translation"
	TypeAgent                   = "agent"
	TypePresentationDefinition  = "presentation_definition"
	TypeCredentialConfiguration = "credential_configuration" //nolint:gosec // a resource type name, not a credential
	TypeServerConfig            = "server_config"
)

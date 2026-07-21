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

package connection

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// connectionExportModel is the unified declarative/export representation of a connection,
// matching the /connections API's typed, vendor-scoped shape (as opposed to the legacy
// identity-provider/notification-sender properties-bag format). Only the fields relevant to
// a given vendor (selected by Type) are populated; the rest are omitted from the rendered
// YAML via `omitempty`. Every field carries both a `yaml` tag (required for declarative
// load/export — the parameterizer and yaml.v3 both skip fields without one) and a `json` tag
// (used by the runtime import parser, which decodes the same shape from a yaml.Node).
type connectionExportModel struct {
	ID          string `yaml:"id"                    json:"id"`
	Type        string `yaml:"type"                  json:"type"`
	Name        string `yaml:"name"                  json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// IdP-backed vendor fields (google, github, oidc, oauth).
	ClientID              string   `yaml:"clientId,omitempty"              json:"clientId,omitempty"`
	ClientSecret          string   `yaml:"clientSecret,omitempty"          json:"clientSecret,omitempty"`
	RedirectURI           string   `yaml:"redirectUri,omitempty"           json:"redirectUri,omitempty"`
	Scopes                []string `yaml:"scopes,omitempty"                json:"scopes,omitempty"`
	Prompt                string   `yaml:"prompt,omitempty"                json:"prompt,omitempty"`
	AuthorizationEndpoint string   `yaml:"authorizationEndpoint,omitempty" json:"authorizationEndpoint,omitempty"`
	TokenEndpoint         string   `yaml:"tokenEndpoint,omitempty"         json:"tokenEndpoint,omitempty"`
	UserInfoEndpoint      string   `yaml:"userInfoEndpoint,omitempty"      json:"userInfoEndpoint,omitempty"`
	JwksEndpoint          string   `yaml:"jwksEndpoint,omitempty"          json:"jwksEndpoint,omitempty"`
	LogoutEndpoint        string   `yaml:"logoutEndpoint,omitempty"        json:"logoutEndpoint,omitempty"`
	Issuer                string   `yaml:"issuer,omitempty"                json:"issuer,omitempty"`
	TokenExchangeEnabled  *bool    `yaml:"tokenExchangeEnabled,omitempty"  json:"tokenExchangeEnabled,omitempty"`
	TrustedTokenAudience  string   `yaml:"trustedTokenAudience,omitempty"  json:"trustedTokenAudience,omitempty"`

	//nolint:lll // long struct tag: both yaml and json keys needed for declarative load/export and import
	AttributeConfiguration *providers.AttributeConfiguration `yaml:"attributeConfiguration,omitempty" json:"attributeConfiguration,omitempty"`

	// SMS-backed vendor fields (twilio, vonage, sms-gateway).
	AccountSID  string `yaml:"accountSid,omitempty"  json:"accountSid,omitempty"`
	AuthToken   string `yaml:"authToken,omitempty"   json:"authToken,omitempty"`
	APIKey      string `yaml:"apiKey,omitempty"      json:"apiKey,omitempty"`
	APISecret   string `yaml:"apiSecret,omitempty"   json:"apiSecret,omitempty"`
	SenderID    string `yaml:"senderId,omitempty"    json:"senderId,omitempty"`
	URL         string `yaml:"url,omitempty"         json:"url,omitempty"`
	HTTPMethod  string `yaml:"httpMethod,omitempty"  json:"httpMethod,omitempty"`
	HTTPHeaders string `yaml:"httpHeaders,omitempty" json:"httpHeaders,omitempty"`
	ContentType string `yaml:"contentType,omitempty" json:"contentType,omitempty"`
}

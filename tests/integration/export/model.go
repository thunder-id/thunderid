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

package export

// ExportRequest represents the request structure for exporting resources.
type ExportRequest struct {
	Applications      []string `json:"applications,omitempty"`
	IdentityProviders []string `json:"identityProviders,omitempty"`
}

// ExportResponse represents the response structure for exporting resources.
type ExportResponse struct {
	Files []ExportFile `json:"files"`
}

// ExportFile represents a single YAML file in the export response.
type ExportFile struct {
	FileName string `json:"fileName"`
	Content  string `json:"content"`
}

// JSONExportResponse represents the simplified JSON response for export endpoints.
type JSONExportResponse struct {
	Resources            string `json:"resources"`
	EnvironmentVariables string `json:"environment_variables"`
}

// Application represents the structure for application request and response in tests.
type Application struct {
	ID                        string              `json:"id,omitempty"`
	OUID                      string              `json:"ouId,omitempty"`
	Name                      string              `json:"name"`
	Description               string              `json:"description,omitempty"`
	ClientID                  string              `json:"clientId,omitempty"`
	ClientSecret              string              `json:"clientSecret,omitempty"`
	AuthFlowID                string              `json:"authFlowId,omitempty"`
	RegistrationFlowID        string              `json:"registrationFlowId,omitempty"`
	IsRegistrationFlowEnabled bool                `json:"isRegistrationFlowEnabled"`
	URL                       string              `json:"url,omitempty"`
	LogoURL                   string              `json:"logoUrl,omitempty"`
	Certificate               *ApplicationCert    `json:"certificate,omitempty"`
	Assertion                 *AssertionConfig    `json:"assertion,omitempty"`
	TosURI                    string              `json:"tosUri,omitempty"`
	PolicyURI                 string              `json:"policyUri,omitempty"`
	Contacts                  []string            `json:"contacts,omitempty"`
	LoginConsent              *LoginConsentConfig `json:"loginConsent,omitempty"`
	InboundAuthConfig         []InboundAuthConfig `json:"inboundAuthConfig,omitempty"`
}

// ApplicationCert represents the certificate structure in the application.
type ApplicationCert struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// InboundAuthConfig represents the inbound authentication configuration.
type InboundAuthConfig struct {
	Type           string          `json:"type"`
	OAuthAppConfig *OAuthAppConfig `json:"config,omitempty"`
}

// OAuthAppConfig represents the OAuth application configuration.
type OAuthAppConfig struct {
	ClientID                string            `json:"clientId"`
	ClientSecret            string            `json:"clientSecret,omitempty"`
	RedirectURIs            []string          `json:"redirectUris"`
	GrantTypes              []string          `json:"grantTypes"`
	ResponseTypes           []string          `json:"responseTypes"`
	TokenEndpointAuthMethod string            `json:"tokenEndpointAuthMethod"`
	PKCERequired            bool              `json:"pkceRequired"`
	PublicClient            bool              `json:"publicClient"`
	Scopes                  []string          `json:"scopes,omitempty"`
	Token                   *OAuthTokenConfig `json:"token,omitempty"`
}

// OAuthTokenConfig represents the OAuth token configuration.
type OAuthTokenConfig struct {
	AccessToken *AccessTokenConfig `json:"accessToken,omitempty"`
	IDToken     *IDTokenConfig     `json:"idToken,omitempty"`
}

// AssertionConfig represents the assertion configuration (for application-level).
type AssertionConfig struct {
	ValidityPeriod int64    `json:"validityPeriod,omitempty"`
	UserAttributes []string `json:"userAttributes,omitempty"`
}

// AccessTokenConfig represents the access token configuration, split by token subject.
type AccessTokenConfig struct {
	UserConfig   *AccessTokenSubConfig `json:"userConfig,omitempty"`
	ClientConfig *AccessTokenSubConfig `json:"clientConfig,omitempty"`
}

// AccessTokenSubConfig represents the validity period and attribute selection for one
// access token subject type (user or client).
type AccessTokenSubConfig struct {
	ValidityPeriod int64    `json:"validityPeriod,omitempty"`
	Attributes     []string `json:"attributes,omitempty"`
}

// IDTokenConfig represents the ID token configuration.
type IDTokenConfig struct {
	ValidityPeriod int64    `json:"validityPeriod,omitempty"`
	UserAttributes []string `json:"userAttributes,omitempty"`
}

// LoginConsentConfig represents the login consent configuration for an application.
type LoginConsentConfig struct {
	ValidityPeriod int64 `json:"validityPeriod,omitempty"`
}

// IDPProperty represents a property of an identity provider.
type IDPProperty struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	IsSecret bool   `json:"isSecret"`
}

// IDP represents an identity provider.
type IDP struct {
	ID          string        `json:"id,omitempty"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Type        string        `json:"type"`
	Properties  []IDPProperty `json:"properties"`
}

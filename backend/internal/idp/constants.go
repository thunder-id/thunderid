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

package idp

import "github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

// IDP property names.
const (
	PropClientID              = "client_id"
	PropClientSecret          = "client_secret"
	PropRedirectURI           = "redirect_uri"
	PropScopes                = "scopes"
	PropAuthorizationEndpoint = "authorization_endpoint"
	PropTokenEndpoint         = "token_endpoint"
	PropUserInfoEndpoint      = "userinfo_endpoint"
	PropUserEmailEndpoint     = "user_email_endpoint"
	PropLogoutEndpoint        = "logout_endpoint"
	PropJwksEndpoint          = "jwks_endpoint"
	PropPrompt                = "prompt"
	PropIssuer                = "issuer"
	PropTokenExchangeEnabled  = "token_exchange_enabled"
)

// Known endpoints for Google OAuth2/OIDC.
const (
	googleAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint         = "https://oauth2.googleapis.com/token" // #nosec G101
	googleUserInfoEndpoint      = "https://openidconnect.googleapis.com/v1/userinfo"
	googleJwksEndpoint          = "https://www.googleapis.com/oauth2/v3/certs"
)

// Known endpoints for GitHub OAuth.
const (
	gitHubAuthorizationEndpoint = "https://github.com/login/oauth/authorize"
	gitHubTokenEndpoint         = "https://github.com/login/oauth/access_token" // #nosec G101
	gitHubUserInfoEndpoint      = "https://api.github.com/user"
	gitHubUserEmailEndpoint     = "https://api.github.com/user/emails"
)

// idpPropertyConfig defines the required and optional properties for an IDP type,
// along with any default values.
type idpPropertyConfig struct {
	Required []string
	Optional []string
	Defaults map[string]string
}

// idpPropertyConfigs maps each IDP type to its property configuration.
var idpPropertyConfigs = map[providers.IDPType]idpPropertyConfig{
	providers.IDPTypeOAuth: {
		Required: []string{
			PropClientID,
			PropClientSecret,
			PropRedirectURI,
			PropAuthorizationEndpoint,
			PropTokenEndpoint,
			PropUserInfoEndpoint,
		},
		Optional: []string{
			PropScopes,
			PropLogoutEndpoint,
			PropPrompt,
		},
		Defaults: map[string]string{},
	},
	providers.IDPTypeOIDC: {
		Required: []string{
			PropClientID,
			PropClientSecret,
			PropRedirectURI,
			PropAuthorizationEndpoint,
			PropTokenEndpoint,
		},
		Optional: []string{
			PropScopes,
			PropUserInfoEndpoint,
			PropLogoutEndpoint,
			PropJwksEndpoint,
			PropPrompt,
			PropIssuer,
			PropTokenExchangeEnabled,
		},
		Defaults: map[string]string{},
	},
	providers.IDPTypeGoogle: {
		Required: []string{
			PropClientID,
			PropClientSecret,
			PropRedirectURI,
		},
		Optional: []string{
			PropAuthorizationEndpoint,
			PropTokenEndpoint,
			PropScopes,
			PropUserInfoEndpoint,
			PropLogoutEndpoint,
			PropJwksEndpoint,
			PropPrompt,
			PropIssuer,
			PropTokenExchangeEnabled,
		},
		Defaults: map[string]string{
			PropAuthorizationEndpoint: googleAuthorizationEndpoint,
			PropTokenEndpoint:         googleTokenEndpoint,
			PropUserInfoEndpoint:      googleUserInfoEndpoint,
			PropJwksEndpoint:          googleJwksEndpoint,
		},
	},
	providers.IDPTypeGitHub: {
		Required: []string{
			PropClientID,
			PropClientSecret,
			PropRedirectURI,
		},
		Optional: []string{
			PropAuthorizationEndpoint,
			PropTokenEndpoint,
			PropUserInfoEndpoint,
			PropUserEmailEndpoint,
			PropScopes,
			PropLogoutEndpoint,
			PropPrompt,
		},
		Defaults: map[string]string{
			PropAuthorizationEndpoint: gitHubAuthorizationEndpoint,
			PropTokenEndpoint:         gitHubTokenEndpoint,
			PropUserInfoEndpoint:      gitHubUserInfoEndpoint,
			PropUserEmailEndpoint:     gitHubUserEmailEndpoint,
		},
	},
}

// tokenExchangeRequiredProps defines the required properties per IDP type when token exchange is enabled.
var tokenExchangeRequiredProps = map[providers.IDPType][]string{
	providers.IDPTypeOIDC: {
		PropIssuer,
		PropJwksEndpoint,
	},
	providers.IDPTypeGoogle: {
		PropIssuer,
		PropJwksEndpoint,
	},
}

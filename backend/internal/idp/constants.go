// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	PropJwksEndpoint          = "jwks_endpoint"
	PropPrompt                = "prompt"
	PropIssuer                = "issuer"
	PropTokenExchangeEnabled  = "token_exchange_enabled"
	PropTrustedTokenAudience  = "trusted_token_audience"
	PropIDJagEnabled          = "id_jag_enabled"
)

// Claims and scopes shared by the OIDC-style providers. A claim is what the provider emits; a scope
// only asks for it.
const (
	emailClaim        = "email"
	emailScope        = "email"
	defaultOIDCScopes = "openid,email,profile"
)

// Local attribute names, as declared by user type schemas.
// TODO: Drop these once schemas can declare attribute types (EMAIL, MOBILE, ...) as a first class
// concept, so the attribute carrying an email can be found rather than named here.
const (
	defaultAccountLinkingAttribute = "email"
	localUsernameAttribute         = "username"
)

// Known endpoints for Google OAuth2/OIDC.
const (
	googleAuthorizationEndpoint = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenEndpoint         = "https://oauth2.googleapis.com/token" // #nosec G101
	googleUserInfoEndpoint      = "https://openidconnect.googleapis.com/v1/userinfo"
	googleJwksEndpoint          = "https://www.googleapis.com/oauth2/v3/certs"
)

// Known endpoints, claims and scopes for GitHub OAuth.
const (
	gitHubAuthorizationEndpoint = "https://github.com/login/oauth/authorize"
	gitHubTokenEndpoint         = "https://github.com/login/oauth/access_token" // #nosec G101
	gitHubUserInfoEndpoint      = "https://api.github.com/user"
	gitHubUserEmailEndpoint     = "https://api.github.com/user/emails"

	gitHubLoginClaim     = "login"
	gitHubUserScope      = "user"
	gitHubUserEmailScope = "user:email"
	defaultGitHubScopes  = gitHubUserEmailScope
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
		},
		Optional: []string{
			PropScopes,
			PropUserInfoEndpoint,
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
			PropJwksEndpoint,
			PropPrompt,
			PropIssuer,
			PropTokenExchangeEnabled,
			PropTrustedTokenAudience,
			PropIDJagEnabled,
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

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

// Package model defines OAuth-related types for inbound client configuration.
//
//nolint:lll
package model

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/jose/jwe"
	"github.com/thunder-id/thunderid/internal/system/jose/jws"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// InboundAuthType identifies the kind of inbound authentication configured for an entity.
type InboundAuthType string

const (
	// OAuthInboundAuthType is the OAuth 2.0 inbound authentication type.
	OAuthInboundAuthType InboundAuthType = "oauth2"
)

// OAuthTokenConfig wraps access and ID token configs.
type OAuthTokenConfig struct {
	AccessToken *AccessTokenConfig `json:"accessToken,omitempty" yaml:"access_token,omitempty" jsonschema:"Access token configuration."`
	IDToken     *IDTokenConfig     `json:"idToken,omitempty"    yaml:"id_token,omitempty"     jsonschema:"ID token configuration."`
}

// AccessTokenConfig is the access token configuration.
type AccessTokenConfig struct {
	ValidityPeriod int64    `json:"validityPeriod,omitempty" yaml:"validity_period,omitempty" jsonschema:"Access token validity period in seconds."`
	UserAttributes []string `json:"userAttributes,omitempty" yaml:"user_attributes,omitempty" jsonschema:"User attributes to embed in the access token."`
}

// IDTokenConfig is the ID token configuration.
type IDTokenConfig struct {
	ValidityPeriod int64               `json:"validityPeriod,omitempty" yaml:"validity_period,omitempty" jsonschema:"ID token validity period in seconds."`
	UserAttributes []string            `json:"userAttributes,omitempty" yaml:"user_attributes,omitempty" jsonschema:"User attributes to embed in the ID token."`
	ResponseType   IDTokenResponseType `json:"responseType,omitempty"   yaml:"response_type,omitempty"   jsonschema:"ID token response type (JWT, JWE, NESTED_JWT). Defaults to JWT."`
	EncryptionAlg  string              `json:"encryptionAlg,omitempty"  yaml:"encryption_alg,omitempty"  jsonschema:"JWE key-management algorithm. Required when responseType is JWE or NESTED_JWT."`
	EncryptionEnc  string              `json:"encryptionEnc,omitempty"  yaml:"encryption_enc,omitempty"  jsonschema:"JWE content-encryption algorithm. Required when responseType is JWE or NESTED_JWT."`
}

// IDTokenResponseType is the response format of the ID token.
type IDTokenResponseType string

const (
	// IDTokenResponseTypeJWT is the standard signed JWT response type (default).
	IDTokenResponseTypeJWT IDTokenResponseType = "JWT"
	// IDTokenResponseTypeJWE is the encrypted JWT response type.
	IDTokenResponseTypeJWE IDTokenResponseType = "JWE"
	// IDTokenResponseTypeNESTEDJWT is the sign-then-encrypt (Nested JWT) response type.
	IDTokenResponseTypeNESTEDJWT IDTokenResponseType = "NESTED_JWT" //nolint:gosec // not a credential
)

// UserInfoConfig is the user info endpoint configuration.
type UserInfoConfig struct {
	ResponseType   UserInfoResponseType `json:"responseType,omitempty"   yaml:"response_type,omitempty"   jsonschema:"UserInfo response type (JSON, JWS, JWE, NESTED_JWT). Required algorithm fields must match the selected response type."`
	UserAttributes []string             `json:"userAttributes,omitempty" yaml:"user_attributes,omitempty" jsonschema:"User attributes to include in the userinfo response."`
	SigningAlg     string               `json:"signingAlg,omitempty"     yaml:"signing_alg,omitempty"     jsonschema:"JWS algorithm for signed userinfo responses (e.g. RS256)."`
	EncryptionAlg  string               `json:"encryptionAlg,omitempty"  yaml:"encryption_alg,omitempty"  jsonschema:"JWE key-management algorithm for encrypted userinfo responses (e.g. RSA-OAEP-256)."`
	EncryptionEnc  string               `json:"encryptionEnc,omitempty"  yaml:"encryption_enc,omitempty"  jsonschema:"JWE content-encryption algorithm (e.g. A256GCM). Required when encryptionAlg is set."`
}

// UserInfoResponseType is the response format of the UserInfo endpoint.
type UserInfoResponseType string

// Supported response formats for the UserInfo endpoint.
const (
	UserInfoResponseTypeJSON      UserInfoResponseType = "JSON"
	UserInfoResponseTypeJWS       UserInfoResponseType = "JWS"
	UserInfoResponseTypeJWE       UserInfoResponseType = "JWE"
	UserInfoResponseTypeNESTEDJWT UserInfoResponseType = "NESTED_JWT"
)

// Supported JOSE algorithms for userinfo responses.
var (
	SupportedUserInfoSigningAlgs = []string{
		string(jws.RS256), string(jws.RS512), string(jws.PS256),
		string(jws.ES256), string(jws.ES384), string(jws.ES512),
		string(jws.EdDSA),
	}
	SupportedUserInfoEncryptionAlgs = []string{string(jwe.RSAOAEP), string(jwe.RSAOAEP256)}
	SupportedUserInfoEncryptionEncs = []string{string(jwe.A128CBCHS256), string(jwe.A256GCM)}
)

// OAuthProfile is the persistence shape (OAUTH_PROFILE JSONB column).
type OAuthProfile struct {
	RedirectURIs                       []string            `json:"redirectUris"`
	GrantTypes                         []string            `json:"grantTypes"`
	ResponseTypes                      []string            `json:"responseTypes"`
	TokenEndpointAuthMethod            string              `json:"tokenEndpointAuthMethod"`
	PKCERequired                       bool                `json:"pkceRequired"`
	PublicClient                       bool                `json:"publicClient"`
	RequirePushedAuthorizationRequests bool                `json:"requirePushedAuthorizationRequests"`
	Token                              *OAuthTokenConfig   `json:"token,omitempty"`
	Scopes                             []string            `json:"scopes,omitempty"`
	UserInfo                           *UserInfoConfig     `json:"userInfo,omitempty"`
	ScopeClaims                        map[string][]string `json:"scopeClaims,omitempty"`
	Certificate                        *Certificate        `json:"certificate,omitempty"`
	AcrValues                          []string            `json:"acrValues,omitempty"`
}

// OAuthConfigWithSecret is the wire input shape and the create/update echo response shape.
// Carries ClientSecret (omitempty) so it appears only when freshly issued.
type OAuthConfigWithSecret struct {
	ClientID                           string                              `json:"clientId,omitempty"                          yaml:"client_id,omitempty"                          jsonschema:"OAuth client ID (auto-generated if not provided)"`
	ClientSecret                       string                              `json:"clientSecret,omitempty"                      yaml:"client_secret,omitempty"                      jsonschema:"OAuth client secret (auto-generated if not provided)"`
	RedirectURIs                       []string                            `json:"redirectUris,omitempty"                      yaml:"redirect_uris,omitempty"                      jsonschema:"Allowed redirect URIs. Required for Public (SPA/Mobile) and Confidential (Server) clients. Omit for M2M."`
	GrantTypes                         []oauth2const.GrantType             `json:"grantTypes,omitempty"                        yaml:"grant_types,omitempty"                        jsonschema:"OAuth grant types. Common: [authorization_code, refresh_token] for user apps, [client_credentials] for M2M."`
	ResponseTypes                      []oauth2const.ResponseType          `json:"responseTypes,omitempty"                     yaml:"response_types,omitempty"                     jsonschema:"OAuth response types. Common: [code] for user apps. Omit for M2M."`
	TokenEndpointAuthMethod            oauth2const.TokenEndpointAuthMethod `json:"tokenEndpointAuthMethod,omitempty"           yaml:"token_endpoint_auth_method,omitempty"         jsonschema:"Client authentication method. Use 'none' for Public clients, 'client_secret_basic' for Confidential/M2M."`
	PKCERequired                       bool                                `json:"pkceRequired"                                yaml:"pkce_required"                                jsonschema:"Require PKCE for security. Recommended for all user-interactive flows."`
	PublicClient                       bool                                `json:"publicClient"                                yaml:"public_client"                                jsonschema:"Identify if client is public (cannot store secrets). Set true for SPA/Mobile."`
	RequirePushedAuthorizationRequests bool                                `json:"requirePushedAuthorizationRequests"          yaml:"require_pushed_authorization_requests"        jsonschema:"Require Pushed Authorization Requests (PAR) per RFC 9126."`
	Token                              *OAuthTokenConfig                   `json:"token,omitempty"                             yaml:"token,omitempty"                              jsonschema:"Token configuration for access tokens and ID tokens"`
	Scopes                             []string                            `json:"scopes,omitempty"                            yaml:"scopes,omitempty"                             jsonschema:"Allowed OAuth scopes. Add custom scopes as needed for your application."`
	UserInfo                           *UserInfoConfig                     `json:"userInfo,omitempty"                          yaml:"user_info,omitempty"                          jsonschema:"UserInfo endpoint configuration. Configure user attributes returned from the OIDC userinfo endpoint."`
	ScopeClaims                        map[string][]string                 `json:"scopeClaims,omitempty"                       yaml:"scope_claims,omitempty"                       jsonschema:"Scope-to-claims mapping. Maps OAuth scopes to user claims for both ID token and userinfo."`
	Certificate                        *Certificate                        `json:"certificate,omitempty"                       yaml:"certificate,omitempty"                        jsonschema:"Application certificate. Optional. For certificate-based authentication or JWT validation."`
	AcrValues                          []string                            `json:"acrValues,omitempty"                         yaml:"acr_values,omitempty"                         jsonschema:"Default ACR values applied when the request does not specify acr_values."`
}

// OAuthConfig is the wire output shape (GET responses). ClientSecret is structurally absent.
// Empty slice/map fields are omitted; booleans are always serialized for explicit semantics.
type OAuthConfig struct {
	ClientID                           string                              `json:"clientId,omitempty"`
	RedirectURIs                       []string                            `json:"redirectUris,omitempty"`
	GrantTypes                         []oauth2const.GrantType             `json:"grantTypes,omitempty"`
	ResponseTypes                      []oauth2const.ResponseType          `json:"responseTypes,omitempty"`
	TokenEndpointAuthMethod            oauth2const.TokenEndpointAuthMethod `json:"tokenEndpointAuthMethod,omitempty"`
	PKCERequired                       bool                                `json:"pkceRequired"`
	PublicClient                       bool                                `json:"publicClient"`
	RequirePushedAuthorizationRequests bool                                `json:"requirePushedAuthorizationRequests"`
	Token                              *OAuthTokenConfig                   `json:"token,omitempty"`
	Scopes                             []string                            `json:"scopes,omitempty"`
	UserInfo                           *UserInfoConfig                     `json:"userInfo,omitempty"`
	ScopeClaims                        map[string][]string                 `json:"scopeClaims,omitempty"`
	Certificate                        *Certificate                        `json:"certificate,omitempty"`
	AcrValues                          []string                            `json:"acrValues,omitempty"`
}

// SupportedIDTokenEncryptionAlgs lists JWE key-management algorithms supported for ID token encryption.
var SupportedIDTokenEncryptionAlgs = []string{string(jwe.RSAOAEP), string(jwe.RSAOAEP256)}

// SupportedIDTokenEncryptionEncs lists JWE content-encryption algorithms supported for ID token encryption.
var SupportedIDTokenEncryptionEncs = []string{string(jwe.A128CBCHS256), string(jwe.A256GCM)}

// OAuthClient is the resolved runtime view.
type OAuthClient struct {
	ID                                 string                              `yaml:"id,omitempty"`
	OUID                               string                              `yaml:"ou_id,omitempty"`
	ClientID                           string                              `yaml:"client_id,omitempty"`
	RedirectURIs                       []string                            `yaml:"redirect_uris,omitempty"`
	GrantTypes                         []oauth2const.GrantType             `yaml:"grant_types,omitempty"`
	ResponseTypes                      []oauth2const.ResponseType          `yaml:"response_types,omitempty"`
	TokenEndpointAuthMethod            oauth2const.TokenEndpointAuthMethod `yaml:"token_endpoint_auth_method,omitempty"`
	PKCERequired                       bool                                `yaml:"pkce_required,omitempty"`
	PublicClient                       bool                                `yaml:"public_client,omitempty"`
	RequirePushedAuthorizationRequests bool                                `yaml:"require_pushed_authorization_requests,omitempty"`
	Token                              *OAuthTokenConfig                   `yaml:"token,omitempty"`
	Scopes                             []string                            `yaml:"scopes,omitempty"`
	UserInfo                           *UserInfoConfig                     `yaml:"user_info,omitempty"`
	ScopeClaims                        map[string][]string                 `yaml:"scope_claims,omitempty"`
	Certificate                        *Certificate                        `yaml:"certificate,omitempty"`
	AcrValues                          []string                            `yaml:"acr_values,omitempty"`
}

// IsAllowedGrantType reports whether the given grant type is allowed for this client.
func (o *OAuthClient) IsAllowedGrantType(grantType oauth2const.GrantType) bool {
	return IsAllowedGrantType(o.GrantTypes, grantType)
}

// IsAllowedResponseType reports whether the given response type is allowed for this client.
func (o *OAuthClient) IsAllowedResponseType(responseType string) bool {
	return IsAllowedResponseType(o.ResponseTypes, responseType)
}

// IsAllowedTokenEndpointAuthMethod reports whether the given auth method is the one configured for this client.
func (o *OAuthClient) IsAllowedTokenEndpointAuthMethod(method oauth2const.TokenEndpointAuthMethod) bool {
	return o.TokenEndpointAuthMethod == method
}

// ValidateRedirectURI validates the given redirect URI against this client's registered URIs.
func (o *OAuthClient) ValidateRedirectURI(redirectURI string) error {
	return ValidateRedirectURI(o.RedirectURIs, redirectURI)
}

// RequiresPKCE reports whether PKCE is required for this client.
func (o *OAuthClient) RequiresPKCE() bool {
	return o.PKCERequired || o.PublicClient
}

// RequiresPAR reports whether pushed authorization requests are required for this client.
func (o *OAuthClient) RequiresPAR() bool {
	return o.RequirePushedAuthorizationRequests || config.GetServerRuntime().Config.OAuth.PAR.RequirePAR
}

// InboundAuthConfigWithSecret is the wire input wrapper and create/update echo response wrapper.
type InboundAuthConfigWithSecret struct {
	Type        InboundAuthType        `json:"type"             yaml:"type"             jsonschema:"Inbound authentication type. Use 'oauth2' for OAuth/OIDC applications."`
	OAuthConfig *OAuthConfigWithSecret `json:"config,omitempty" yaml:"config,omitempty" jsonschema:"OAuth/OIDC configuration. Required when type is 'oauth2'. Defines OAuth grant types, redirect URIs, client authentication, and PKCE settings."`
}

// InboundAuthConfig is the wire output wrapper (GET responses).
type InboundAuthConfig struct {
	Type        InboundAuthType `json:"type"`
	OAuthConfig *OAuthConfig    `json:"config,omitempty"`
}

// InboundAuthConfigProcessed is the runtime wrapper.
type InboundAuthConfigProcessed struct {
	Type        InboundAuthType `json:"type"             yaml:"type,omitempty"`
	OAuthConfig *OAuthClient    `json:"config,omitempty" yaml:"config,omitempty"`
}

// IsAllowedGrantType reports whether the given grant type is in the allowed list.
func IsAllowedGrantType(grantTypes []oauth2const.GrantType, grantType oauth2const.GrantType) bool {
	if grantType == "" {
		return false
	}
	return slices.Contains(grantTypes, grantType)
}

// IsAllowedResponseType reports whether the given response type is in the allowed list.
func IsAllowedResponseType(responseTypes []oauth2const.ResponseType, responseType string) bool {
	if responseType == "" {
		return false
	}
	return slices.Contains(responseTypes, oauth2const.ResponseType(responseType))
}

// ValidateRedirectURI validates the provided redirect URI against the registered list.
func ValidateRedirectURI(redirectURIs []string, redirectURI string) error {
	logger := log.GetLogger()

	if redirectURI == "" {
		if len(redirectURIs) != 1 {
			return fmt.Errorf("redirect URI is required in the authorization request")
		}
		// AC-12: a wildcard pattern cannot serve as a concrete redirect target.
		if strings.Contains(redirectURIs[0], "*") {
			return fmt.Errorf("redirect URI is required in the authorization request")
		}
		parsed, err := url.Parse(redirectURIs[0])
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("registered redirect URI is not fully qualified")
		}
		return nil
	}

	if !matchAnyRedirectURIPattern(redirectURIs, redirectURI) {
		return fmt.Errorf("your application's redirect URL does not match with the registered redirect URLs")
	}

	parsedRedirectURI, err := utils.ParseURL(redirectURI)
	if err != nil {
		logger.Error("Failed to parse redirect URI", log.Error(err))
		return fmt.Errorf("invalid redirect URI: %s", err.Error())
	}
	if parsedRedirectURI.Fragment != "" {
		return fmt.Errorf("redirect URI must not contain a fragment component")
	}

	return nil
}

// matchAnyRedirectURIPattern compares incoming against each registered URI/pattern. AC-11: first match wins.
func matchAnyRedirectURIPattern(patterns []string, redirectURI string) bool {
	wildcardEnabled := config.GetServerRuntime().Config.OAuth.AllowWildcardRedirectURI
	for _, pattern := range patterns {
		if !wildcardEnabled || !strings.Contains(pattern, "*") {
			if pattern == redirectURI {
				return true
			}
			continue
		}
		matched, err := utils.MatchURIPattern(pattern, redirectURI)
		if err != nil {
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

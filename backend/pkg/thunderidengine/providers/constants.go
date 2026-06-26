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

// Package providers provides constants for the providers module.
package providers

// IDPType represents the type of an identity provider.
type IDPType string

const (
	// IDPTypeOAuth represents an OAuth2 identity provider.
	IDPTypeOAuth IDPType = "OAUTH"
	// IDPTypeOIDC represents an OIDC identity provider.
	IDPTypeOIDC IDPType = "OIDC"
	// IDPTypeGoogle represents a Google identity provider.
	IDPTypeGoogle IDPType = "GOOGLE"
	// IDPTypeGitHub represents a GitHub identity provider.
	IDPTypeGitHub IDPType = "GITHUB"
)

// SupportedIDPTypes lists all the supported identity provider types.
var SupportedIDPTypes = []IDPType{
	IDPTypeOAuth,
	IDPTypeOIDC,
	IDPTypeGoogle,
	IDPTypeGitHub,
}

// FlowType defines the type of flow execution.
type FlowType string

const (
	// FlowTypeAuthentication represents a flow execution for user authentication.
	FlowTypeAuthentication FlowType = "AUTHENTICATION"
	// FlowTypeRegistration represents a flow execution for user registration.
	FlowTypeRegistration FlowType = "REGISTRATION"
	// FlowTypeUserOnboarding represents an admin-initiated user onboarding flow.
	FlowTypeUserOnboarding FlowType = "USER_ONBOARDING"
	// FlowTypeRecovery represents a flow execution for account recovery (e.g., password reset).
	FlowTypeRecovery FlowType = "RECOVERY"
)

// NodeVariant identifies a PROMPT node sub-type that activates a variant-specific code path.
type NodeVariant string

const (
	// NodeVariantLoginOptions identifies a PROMPT node that presents login method choices to the user.
	NodeVariantLoginOptions NodeVariant = "LOGIN_OPTIONS"
)

// String returns the string representation of the node variant.
func (nv NodeVariant) String() string {
	return string(nv)
}

// InterceptorMode represents the lifecycle point at which an interceptor executes.
type InterceptorMode string

// Interceptor mode constants.
const (
	InterceptorModePreRequest  InterceptorMode = "PRE_REQUEST"
	InterceptorModePreNode     InterceptorMode = "PRE_NODE"
	InterceptorModePostNode    InterceptorMode = "POST_NODE"
	InterceptorModePostRequest InterceptorMode = "POST_REQUEST"
)

// InterceptorScope determines which nodes a per-node interceptor applies to.
type InterceptorScope string

// Interceptor scope constants.
const (
	InterceptorScopeAll      InterceptorScope = "ALL"
	InterceptorScopeSelected InterceptorScope = "SELECTED"
)

// ValidInterceptorModes contains the set of valid interceptor modes for validation.
var ValidInterceptorModes = map[InterceptorMode]bool{
	InterceptorModePreRequest:  true,
	InterceptorModePreNode:     true,
	InterceptorModePostNode:    true,
	InterceptorModePostRequest: true,
}

// ValidInterceptorScopes contains the set of valid interceptor scopes for validation.
var ValidInterceptorScopes = map[InterceptorScope]bool{
	InterceptorScopeAll:      true,
	InterceptorScopeSelected: true,
}

// DesignResolveType represents the type of entity for design resolution.
type DesignResolveType string

// Design resolve type constants.
const (
	// DesignResolveTypeAPP represents the application type for design resolution.
	DesignResolveTypeAPP DesignResolveType = "APP"
	// DesignResolveTypeOU represents the organizational unit type for design resolution.
	DesignResolveTypeOU DesignResolveType = "OU"
)

// GrantType defines a type for OAuth2 grant types.
type GrantType string

const (
	// GrantTypeAuthorizationCode represents the authorization code grant type.
	GrantTypeAuthorizationCode GrantType = "authorization_code"
	// GrantTypeClientCredentials represents the client credentials grant type.
	GrantTypeClientCredentials GrantType = "client_credentials"
	// GrantTypeRefreshToken represents the refresh token grant type.
	GrantTypeRefreshToken GrantType = "refresh_token"
	// GrantTypeTokenExchange represents the token exchange grant type.
	GrantTypeTokenExchange GrantType = "urn:ietf:params:oauth:grant-type:token-exchange" //nolint:gosec
	// GrantTypeCIBA represents the OpenID Connect CIBA (Client-Initiated Backchannel Authentication) grant type.
	GrantTypeCIBA GrantType = "urn:openid:params:grant-type:ciba"
)

// ResponseType defines a type for OAuth2 response types.
type ResponseType string

const (
	// ResponseTypeCode represents the authorization code response type.
	ResponseTypeCode ResponseType = "code"
	// ResponseTypeIDToken represents the id token response type.
	ResponseTypeIDToken ResponseType = "id_token"
)

// TokenEndpointAuthMethod defines a type for token endpoint authentication methods.
type TokenEndpointAuthMethod string

const (
	// TokenEndpointAuthMethodClientSecretBasic represents the client secret basic authentication method.
	TokenEndpointAuthMethodClientSecretBasic TokenEndpointAuthMethod = "client_secret_basic"
	// TokenEndpointAuthMethodClientSecretPost represents the client secret post authentication method.
	TokenEndpointAuthMethodClientSecretPost TokenEndpointAuthMethod = "client_secret_post"
	// TokenEndpointAuthMethodPrivateKeyJWT represents the private key JWT authentication method.
	// #nosec G101 - This is not a hardcoded credential, but a constant representing an authentication method.
	TokenEndpointAuthMethodPrivateKeyJWT TokenEndpointAuthMethod = "private_key_jwt"
	// TokenEndpointAuthMethodNone represents no authentication method.
	TokenEndpointAuthMethodNone TokenEndpointAuthMethod = "none"
)

// SupportedGrantTypes lists all the supported grant types.
var SupportedGrantTypes = []GrantType{
	GrantTypeAuthorizationCode,
	GrantTypeClientCredentials,
	GrantTypeRefreshToken,
	GrantTypeTokenExchange,
	GrantTypeCIBA,
}

// IsValid checks if the GrantType is valid.
func (gt GrantType) IsValid() bool {
	for _, valid := range SupportedGrantTypes {
		if gt == valid {
			return true
		}
	}
	return false
}

// SupportedResponseTypes lists all the supported response types.
var SupportedResponseTypes = []ResponseType{
	ResponseTypeCode,
}

// IsValid checks if the ResponseType is valid.
func (rt ResponseType) IsValid() bool {
	for _, valid := range SupportedResponseTypes {
		if rt == valid {
			return true
		}
	}
	return false
}

// SupportedTokenEndpointAuthMethods lists all the supported token endpoint authentication methods.
var SupportedTokenEndpointAuthMethods = []TokenEndpointAuthMethod{
	TokenEndpointAuthMethodClientSecretBasic,
	TokenEndpointAuthMethodClientSecretPost,
	TokenEndpointAuthMethodPrivateKeyJWT,
	TokenEndpointAuthMethodNone,
}

// IsValid checks if the TokenEndpointAuthMethod is valid.
func (tam TokenEndpointAuthMethod) IsValid() bool {
	for _, valid := range SupportedTokenEndpointAuthMethods {
		if tam == valid {
			return true
		}
	}
	return false
}

// EntityCategory represents the category of an entity (e.g., user, application, agent).
type EntityCategory string

const (
	// EntityCategoryUser represents a user entity.
	EntityCategoryUser EntityCategory = "user"
	// EntityCategoryApp represents an application entity.
	EntityCategoryApp EntityCategory = "app"
	// EntityCategoryAgent represents an agent entity.
	EntityCategoryAgent EntityCategory = "agent"
)

// String returns the string representation of the entity category.
func (ec EntityCategory) String() string {
	return string(ec)
}

// FlowInitiationMode classifies how an application is permitted to initiate a new authentication
// flow directly over HTTP. It is derived at runtime from the application's inbound protocol
// configuration and is intentionally protocol-neutral, so the flow-execution layer can decide
// behavior without inspecting protocol-specific data (e.g. OAuth grant types).
type FlowInitiationMode string

const (
	// FlowInitiationModeRedirectOnly indicates the application signs users in through a
	// redirect-based protocol component (currently OAuth 2.0 apps using the authorization_code
	// grant). Such applications must have their flows initiated by that component and may not
	// initiate a new flow via a direct HTTP call.
	FlowInitiationModeRedirectOnly FlowInitiationMode = "REDIRECT_ONLY"
	// FlowInitiationModeAppSecret indicates a backend / server-side application — one that does not
	// sign in by redirect, or an embedded app with no protocol profile at all — that may initiate a
	// flow directly by presenting a valid App Secret.
	FlowInitiationModeAppSecret FlowInitiationMode = "APP_SECRET"
)

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

// UserInfoResponseType is the response format of the UserInfo endpoint.
type UserInfoResponseType string

const (
	// UserInfoResponseTypeJSON is the JSON response type.
	UserInfoResponseTypeJSON UserInfoResponseType = "JSON"
	// UserInfoResponseTypeJWS is the JWS response type.
	UserInfoResponseTypeJWS UserInfoResponseType = "JWS"
	// UserInfoResponseTypeJWE is the JWE response type.
	UserInfoResponseTypeJWE UserInfoResponseType = "JWE"
	// UserInfoResponseTypeNESTEDJWT is the Nested JWT response type.
	UserInfoResponseTypeNESTEDJWT UserInfoResponseType = "NESTED_JWT"
)

// CertificateType represents the type of certificates in the system.
type CertificateType string

const (
	// CertificateTypeJWKS represents a JSON Web Key Set (JWKS) certificate.
	CertificateTypeJWKS CertificateType = "JWKS"
	// CertificateTypeJWKSURI represents a JWKS URI certificate.
	CertificateTypeJWKSURI CertificateType = "JWKS_URI"
)

// EntityState represents the lifecycle state of an entity.
type EntityState string

const (
	// EntityStateActive represents an active entity.
	EntityStateActive EntityState = "ACTIVE"
)

// String returns the string representation of the entity state.
func (es EntityState) String() string {
	return string(es)
}

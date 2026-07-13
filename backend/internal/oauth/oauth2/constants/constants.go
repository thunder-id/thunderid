/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

// Package constants defines constants used across the OAuth2 module.
package constants

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// OAuth2 request parameters.
const (
	RequestParamGrantType           string = "grant_type"
	RequestParamClientID            string = "client_id"
	RequestParamClientSecret        string = "client_secret"
	RequestParamClientAssertion     string = "client_assertion"
	RequestParamClientAssertionType string = "client_assertion_type"
	RequestParamRedirectURI         string = "redirect_uri"
	RequestParamUsername            string = "username"
	RequestParamPassword            string = "password"
	RequestParamScope               string = "scope"
	RequestParamCode                string = "code"
	RequestParamCodeVerifier        string = "code_verifier"
	RequestParamCodeChallenge       string = "code_challenge"
	RequestParamCodeChallengeMethod string = "code_challenge_method"
	RequestParamRefreshToken        string = "refresh_token"
	RequestParamResponseType        string = "response_type"
	RequestParamState               string = "state"
	RequestParamIss                 string = "iss"
	RequestParamResource            string = "resource"
	RequestParamError               string = "error"
	RequestParamErrorDescription    string = "error_description"
	RequestParamToken               string = "token"
	RequestParamTokenTypeHint       string = "token_type_hint"
	RequestParamSubjectToken        string = "subject_token"
	RequestParamSubjectTokenType    string = "subject_token_type"
	RequestParamActorToken          string = "actor_token"
	RequestParamActorTokenType      string = "actor_token_type"
	RequestParamRequestedTokenType  string = "requested_token_type"
	RequestParamAudience            string = "audience"
	RequestParamClaims              string = "claims"
	RequestParamClaimsLocales       string = "claims_locales"
	RequestParamNonce               string = "nonce"
	RequestParamPrompt              string = "prompt"
	RequestParamRequestURI          string = "request_uri"
	RequestParamAcrValues           string = "acr_values"
	RequestParamMaxAge              string = "max_age"
	RequestParamDPoPJkt             string = "dpop_jkt"
	RequestParamLoginHint           string = "login_hint"
	RequestParamIDTokenHint         string = "id_token_hint"
	RequestParamLoginHintToken      string = "login_hint_token" // #nosec G101
	RequestParamBindingMessage      string = "binding_message"
	RequestParamRequestedExpiry     string = "requested_expiry"
	RequestParamAuthReqID           string = "auth_req_id"
)

// OAuth2 HTTP headers.
const (
	HeaderDPoP string = "DPoP"
)

// OIDC prompt parameter values.
const (
	PromptNone          string = "none"
	PromptLogin         string = "login"
	PromptConsent       string = "consent"
	PromptSelectAccount string = "select_account"
)

// ValidPromptValues contains all valid OIDC prompt parameter values.
var ValidPromptValues = []string{
	PromptNone, PromptLogin, PromptConsent, PromptSelectAccount,
}

// OAuth2 request parameter validation limits.
const (
	// MaxNonceLength defines the maximum allowed length of the nonce parameter.
	// Aligned with FAPI 2.0 Security Profile recommendation (64 characters).
	MaxNonceLength = 64
)

// Server OAuth constants.
const (
	AuthID                string = "authId"
	SessionDataKeyConsent string = "sessionDataKeyConsent"
	ShowInsecureWarning   string = "showInsecureWarning"
	AppID                 string = "applicationId"
	ExecutionID           string = "executionId"
	Assertion             string = "assertion"
)

// Oauth message types.
const (
	TypeInitialAuthorizationRequest     string = "initialAuthorizationRequest"
	TypeAuthorizationResponseFromEngine string = "authorizationResponseFromEngine"
	TypeConsentResponseFromUser         string = "consentResponseFromUser"
)

// OAuth2 endpoints.
const (
	OAuth2TokenEndpoint                   string = "/oauth2/token" // #nosec G101
	OAuth2AuthorizationEndpoint           string = "/oauth2/authorize"
	OAuth2IntrospectionEndpoint           string = "/oauth2/introspect"
	OAuth2RevokeEndpoint                  string = "/oauth2/revoke"
	OAuth2UserInfoEndpoint                string = "/oauth2/userinfo"
	OAuth2JWKSEndpoint                    string = "/oauth2/jwks"
	OAuth2LogoutEndpoint                  string = "/oauth2/logout"
	OAuth2DCREndpoint                     string = "/oauth2/dcr/register"
	OAuth2PAREndpoint                     string = "/oauth2/par"
	OAuth2BackchannelAuthEndpoint         string = "/oauth2/bc-authorize"
	OAuth2BackchannelAuthCallbackEndpoint string = "/oauth2/bc-authorize/callback"
)

// OAuth2 token types.
const (
	TokenTypeBearer = "Bearer"
	TokenTypeDPoP   = "DPoP"
)

// TokenTypeIdentifier defines a type for RFC 8693 token type identifiers.
type TokenTypeIdentifier string

// RFC 8693 Token Type Identifiers
const (
	//nolint:gosec // Token type identifier, not a credential
	TokenTypeIdentifierAccessToken TokenTypeIdentifier = "urn:ietf:params:oauth:token-type:access_token"
	//nolint:gosec // Token type identifier, not a credential
	TokenTypeIdentifierRefreshToken TokenTypeIdentifier = "urn:ietf:params:oauth:token-type:refresh_token"
	//nolint:gosec // Token type identifier, not a credential
	TokenTypeIdentifierIDToken TokenTypeIdentifier = "urn:ietf:params:oauth:token-type:id_token"
	//nolint:gosec // Token type identifier, not a credential
	TokenTypeIdentifierJWT TokenTypeIdentifier = "urn:ietf:params:oauth:token-type:jwt"
)

// supportedTokenTypeIdentifiers is the single source of truth for all supported token type identifiers.
var supportedTokenTypeIdentifiers = []TokenTypeIdentifier{
	TokenTypeIdentifierAccessToken,
	TokenTypeIdentifierRefreshToken,
	TokenTypeIdentifierIDToken,
	TokenTypeIdentifierJWT,
}

// IsValid checks if the TokenTypeIdentifier is valid.
func (tti TokenTypeIdentifier) IsValid() bool {
	for _, valid := range supportedTokenTypeIdentifiers {
		if tti == valid {
			return true
		}
	}
	return false
}

// OAuth2 error codes.
const (
	ErrorInvalidRequest           string = "invalid_request"
	ErrorInvalidClient            string = "invalid_client"
	ErrorInvalidGrant             string = "invalid_grant"
	ErrorUnauthorizedClient       string = "unauthorized_client"
	ErrorUnsupportedGrantType     string = "unsupported_grant_type"
	ErrorInvalidScope             string = "invalid_scope"
	ErrorInvalidTarget            string = "invalid_target"
	ErrorServerError              string = "server_error"
	ErrorUnsupportedTokenType     string = "unsupported_token_type" //nolint:gosec // OAuth error code, not a credential
	ErrorUnsupportedResponseType  string = "unsupported_response_type"
	ErrorAccessDenied             string = "access_denied"
	ErrorLoginRequired            string = "login_required"
	ErrorConsentRequired          string = "consent_required"
	ErrorAccountSelectionRequired string = "account_selection_required"
	ErrorInvalidDPoPProof         string = "invalid_dpop_proof"
	ErrorAuthorizationPending     string = "authorization_pending"
	ErrorSlowDown                 string = "slow_down"
	ErrorExpiredToken             string = "expired_token" // #nosec G101
	ErrorUnknownUserID            string = "unknown_user_id"
	ErrorInvalidBindingMessage    string = "invalid_binding_message"
)

// UnSupportedGrantTypeError is returned when an unsupported grant type is requested.
var UnSupportedGrantTypeError = errors.New("unsupported_grant_type")

// StandardOIDCScopes contains all standard OIDC scopes
var StandardOIDCScopes = map[string]model.OIDCScope{
	"openid": {
		Name:        "openid",
		Description: "REQUIRED scope for OpenID Connect authentication",
		Claims:      []string{"sub"},
	},
	"profile": {
		Name:        "profile",
		Description: "Requests access to end-user's default profile claims",
		Claims: []string{
			"name", "family_name", "given_name", "middle_name",
			"nickname", "preferred_username", "profile", "picture",
			"website", "gender", "birthdate", "zoneinfo", "locale", "updated_at",
		},
	},
	"email": {
		Name:        "email",
		Description: "Requests access to email and email_verified claims",
		Claims:      []string{"email", "email_verified"},
	},
	"phone": {
		Name:        "phone",
		Description: "Requests access to phone_number and phone_number_verified claims",
		Claims:      []string{"phone_number", "phone_number_verified"},
	},
	"address": {
		Name:        "address",
		Description: "Requests access to address claim",
		Claims:      []string{"address"},
	},
	"roles": {
		Name:        "roles",
		Description: "Requests access to user's assigned roles",
		Claims:      []string{"roles"},
	},
}

// Standard JWT claim names.
const (
	ClaimSub      string = "sub"
	ClaimIss      string = "iss"
	ClaimAud      string = "aud"
	ClaimExp      string = "exp"
	ClaimIat      string = "iat"
	ClaimJTI      string = "jti"
	ClaimAuthTime string = "auth_time"
)

// Custom JWT claim names.
const (
	ClaimUserType               string = "userType"
	ClaimOUID                   string = "ouId"
	ClaimOUName                 string = "ouName"
	ClaimOUHandle               string = "ouHandle"
	ClaimClaimsRequest          string = "claims_req"
	ClaimClaimsLocales          string = "claims_locales"
	ClaimCompletedAuthClass     string = "completed_auth_class"
	ClaimDPoPJkt                string = "dpop_jkt"
	ClaimAuthorizedPermissions  string = "authorized_permissions"
	ClaimAuthorizationRequestID string = "authorization_request_id"
	ClaimClientID               string = "client_id"
)

// OIDC subject types.
const (
	SubjectTypePublic string = "public"
)

// User attribute constants.
const (
	// UserAttributeGroups is the constant for user's groups attribute.
	UserAttributeGroups = "groups"
	// UserAttributeRoles is the constant for user's roles attribute.
	UserAttributeRoles = "roles"
	// DefaultGroupListLimit is the default limit for group list retrieval.
	DefaultGroupListLimit = 20
)

// Standard OIDC scope names.
const (
	ScopeOpenID = "openid"
)

const (
	// SupportedClientAssertionType is the constant for supported client assertion type.
	SupportedClientAssertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
)

const (
	// AttributeCacheTTLBufferSeconds is a fixed buffer added to attribute cache TTL values to
	// account for the gap between cache entry creation (end of authentication) and token issuance.
	AttributeCacheTTLBufferSeconds = 60
)

const (
	// CIBADefaultExpiresInSeconds is the default lifetime in seconds of a CIBA authentication request.
	CIBADefaultExpiresInSeconds = 120
	// CIBADefaultIntervalSeconds is the default minimum interval in seconds between CIBA token polls.
	CIBADefaultIntervalSeconds = 5
	// CIBAMaxExpiresInSeconds is the maximum lifetime in seconds a client may request via requested_expiry.
	CIBAMaxExpiresInSeconds = 600
)

// GetSupportedResponseTypes returns all supported OAuth2 response types.
func GetSupportedResponseTypes() []string {
	result := make([]string, len(providers.SupportedResponseTypes))
	for i, rt := range providers.SupportedResponseTypes {
		result[i] = string(rt)
	}
	return result
}

// GetSupportedGrantTypes returns all supported OAuth2 grant types.
func GetSupportedGrantTypes() []string {
	result := make([]string, len(providers.SupportedGrantTypes))
	for i, gt := range providers.SupportedGrantTypes {
		result[i] = string(gt)
	}
	return result
}

// GetSupportedTokenEndpointAuthMethods returns all supported token endpoint authentication methods.
func GetSupportedTokenEndpointAuthMethods() []string {
	result := make([]string, len(providers.SupportedTokenEndpointAuthMethods))
	for i, tam := range providers.SupportedTokenEndpointAuthMethods {
		result[i] = string(tam)
	}
	return result
}

// GetSupportedSubjectTypes returns all supported OIDC subject types.
func GetSupportedSubjectTypes() []string {
	return []string{SubjectTypePublic}
}

// GetStandardClaims returns all standard JWT claims that are always included in tokens.
func GetStandardClaims() []string {
	return []string{
		ClaimSub,
		ClaimIss,
		ClaimAud,
		ClaimExp,
		ClaimIat,
		ClaimAuthTime,
	}
}

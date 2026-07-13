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

import "errors"

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

// ValidFlowTypes is the set of supported flow types.
var ValidFlowTypes = []FlowType{
	FlowTypeAuthentication,
	FlowTypeRegistration,
	FlowTypeUserOnboarding,
	FlowTypeRecovery,
}

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

// ConsentStatus defines the possible statuses for a consent record.
type ConsentStatus string

const (
	// ConsentStatusCreated indicates that the consent record has been created, but not yet active.
	ConsentStatusCreated ConsentStatus = "CREATED"
	// ConsentStatusActive indicates that the consent is active and valid.
	ConsentStatusActive ConsentStatus = "ACTIVE"
	// ConsentStatusRejected indicates that the consent has been rejected by the user.
	ConsentStatusRejected ConsentStatus = "REJECTED"
	// ConsentStatusRevoked indicates that the consent has been revoked by the user or admin.
	ConsentStatusRevoked ConsentStatus = "REVOKED"
	// ConsentStatusExpired indicates that the consent has expired after its validity time.
	ConsentStatusExpired ConsentStatus = "EXPIRED"
)

// ConsentType defines the possible types for a consent record.
type ConsentType string

const (
	// ConsentTypeAuthentication represents a consent record related to authentication flows.
	ConsentTypeAuthentication ConsentType = "AUTHENTICATION"
)

// Namespace represents the consent namespace to scope consent elements and purposes.
type Namespace string

const (
	// NamespaceAttribute represents the attribute consent namespace.
	// Used for managing consent over user attributes (e.g. email, mobile).
	NamespaceAttribute Namespace = "attribute"
	// NamespacePermission represents the permission consent namespace.
	// Used for managing consent over resource action permissions (e.g. booking:reservations:read).
	NamespacePermission Namespace = "permission"
)

// ConsentAuthorizationStatus defines the possible statuses for a consent authorization record.
type ConsentAuthorizationStatus string

const (
	// AuthorizationStatusCreated indicates that the authorization record has been created,
	// but not yet approved or rejected.
	AuthorizationStatusCreated ConsentAuthorizationStatus = "CREATED"
	// AuthorizationStatusApproved indicates that the authorization record has been approved by the user.
	AuthorizationStatusApproved ConsentAuthorizationStatus = "APPROVED"
	// AuthorizationStatusRejected indicates that the authorization record has been rejected by the user.
	AuthorizationStatusRejected ConsentAuthorizationStatus = "REJECTED"
)

// ConsentAuthorizationType defines the possible types for a consent authorization record.
type ConsentAuthorizationType string

const (
	// AuthorizationTypeAuthorization represents a standard user authorization action for a consent.
	AuthorizationTypeAuthorization ConsentAuthorizationType = "AUTHORIZATION"
	// AuthorizationTypeReAuthorization represents a re-authorization action for a consent.
	AuthorizationTypeReAuthorization ConsentAuthorizationType = "RE_AUTHORIZATION"
)

// TODO: Define a go type for InputType when formalizing input types

// InputType constants define known input types used in flow definitions.
const (
	// InputTypeText represents a text input type.
	InputTypeText = "TEXT_INPUT"
	// InputTypeEmail represents an email input type.
	InputTypeEmail = "EMAIL_INPUT"
	// InputTypePassword represents a password credential input type.
	InputTypePassword = "PASSWORD_INPUT"
	// InputTypeOTP represents a one-time password input type.
	InputTypeOTP = "OTP_INPUT"
	// InputTypePhone represents a phone number input type.
	InputTypePhone = "PHONE_INPUT"
	// InputTypeConsent represents a consent decisions input type.
	InputTypeConsent = "CONSENT_INPUT"
	// InputTypeHidden represents a hidden input type.
	InputTypeHidden = "HIDDEN"
	// InputTypeSelect represents a select (dropdown) input type.
	InputTypeSelect = "SELECT"
	// InputTypeOUSelect represents an organization unit selection input type.
	InputTypeOUSelect = "OU_SELECT"
	// InputTypeNumber represents a numeric input type.
	InputTypeNumber = "NUMBER_INPUT"
	// InputTypeDate represents a date input type.
	InputTypeDate = "DATE_INPUT"

	// TODO: Add support for other sensitive input types:
	// - Passkey credential fields (credentialId, clientDataJSON, authenticatorData, signature, userHandle)
	// - OAuth/OIDC authorization codes
	// - OIDC nonce
	// - Invite tokens
)

// ValidInputTypes is the set of valid input type strings.
var ValidInputTypes = map[string]bool{
	InputTypeText:     true,
	InputTypeEmail:    true,
	InputTypePassword: true,
	InputTypeOTP:      true,
	InputTypePhone:    true,
	InputTypeConsent:  true,
	InputTypeHidden:   true,
	InputTypeSelect:   true,
	InputTypeOUSelect: true,
	InputTypeNumber:   true,
	InputTypeDate:     true,
}

// ExecutorType defines the type of an executor in the flow execution.
type ExecutorType string

const (
	// ExecutorTypeAuthentication represents an executor that performs authentication.
	ExecutorTypeAuthentication ExecutorType = "AUTHENTICATION"
	// ExecutorTypeRegistration represents an executor that handles user registration/provisioning.
	ExecutorTypeRegistration ExecutorType = "REGISTRATION"
	// ExecutorTypeUtility represents a utility executor for common operations.
	ExecutorTypeUtility ExecutorType = "UTILITY"
)

// ExecutorStatus defines the status of an executor in the flow execution.
type ExecutorStatus string

const (
	// ExecComplete indicates that the executor has completed its execution successfully.
	ExecComplete ExecutorStatus = "COMPLETE"
	// ExecUserInputRequired indicates that the executor requires user input to proceed.
	ExecUserInputRequired ExecutorStatus = "USER_INPUT_REQUIRED"
	// ExecExternalRedirection indicates that the executor is redirecting to an external URL.
	ExecExternalRedirection ExecutorStatus = "EXTERNAL_REDIRECTION"
	// ExecFailure indicates that the executor has failed during its execution.
	ExecFailure ExecutorStatus = "FAILURE"
	// ExecRetry indicates that the executor is retrying its execution.
	ExecRetry ExecutorStatus = "RETRY"
)

// ValidationType identifies the constraint type of a ValidationRule.
type ValidationType string

// Validation rule types.
const (
	// ValidationTypeRegex matches the submitted value against a regex pattern.
	ValidationTypeRegex ValidationType = "regex"
	// ValidationTypeMinLength enforces a minimum string length on the submitted value.
	ValidationTypeMinLength ValidationType = "minLength"
	// ValidationTypeMaxLength enforces a maximum string length on the submitted value.
	ValidationTypeMaxLength ValidationType = "maxLength"
)

// Default i18n fallback message keys returned in fieldErrors when a validation
// rule does not specify a message.
const (
	DefaultValidationMessageRegex     = "validation.pattern.invalid"
	DefaultValidationMessageMinLength = "validation.minLength.invalid"
	DefaultValidationMessageMaxLength = "validation.maxLength.invalid"
)

// InboundAuthType identifies the kind of inbound authentication configured for an entity.
type InboundAuthType string

const (
	// OAuthInboundAuthType is the OAuth 2.0 inbound authentication type.
	OAuthInboundAuthType InboundAuthType = "oauth2"
)

// FlowStatus defines the status of a flow execution.
type FlowStatus string

const (
	// FlowStatusComplete indicates that the flow execution is complete.
	FlowStatusComplete FlowStatus = "COMPLETE"
	// FlowStatusIncomplete indicates that the flow execution is incomplete.
	FlowStatusIncomplete FlowStatus = "INCOMPLETE"
	// FlowStatusError indicates that there was an error during the flow execution.
	FlowStatusError FlowStatus = "ERROR"
)

// ValidValidationRuleTypes is the set of valid validation rule type strings.
var ValidValidationRuleTypes = map[string]bool{
	string(ValidationTypeRegex):     true,
	string(ValidationTypeMinLength): true,
	string(ValidationTypeMaxLength): true,
}

// EventType is a type alias for event type strings.
// This allows for type-safe event type constants while keeping the Event struct generic.
type EventType string

// Common status values (use these for consistency, but not enforced)
const (
	StatusSuccess    = "success"
	StatusFailure    = "failure"
	StatusInProgress = "in_progress"
	StatusPending    = "pending"
)

// RuntimeStoreNamespace identifies the category of data stored in the runtime store.
type RuntimeStoreNamespace string

// Namespace constants for the runtime store. All namespaces follow the <category>:<type> format.
const (
	NamespaceAttributeCache RuntimeStoreNamespace = "attribute:cache"
	NamespaceFlow           RuntimeStoreNamespace = "flow:state"
	NamespaceAuthzCode      RuntimeStoreNamespace = "authz:code"
	NamespaceAuthzReq       RuntimeStoreNamespace = "authz:req"
	NamespacePAR            RuntimeStoreNamespace = "par:req"
	NamespaceCIBA           RuntimeStoreNamespace = "ciba:req"
	NamespaceJTI            RuntimeStoreNamespace = "jti:token"
	NamespaceVCINonce       RuntimeStoreNamespace = "vci:nonce"
	NamespaceVCIOffer       RuntimeStoreNamespace = "vci:offer"
	NamespaceVPState        RuntimeStoreNamespace = "vp:state"
)

// Error constants
var (
	// ErrRuntimeStoreKeyNotFound to identify key not found error in the runtime store providers
	ErrRuntimeStoreKeyNotFound = errors.New("RuntimeStore key not found")
)

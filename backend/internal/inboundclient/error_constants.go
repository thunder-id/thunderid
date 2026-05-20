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

package inboundclient

import (
	"errors"

	"github.com/thunder-id/thunderid/internal/cert"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
)

var (
	// ErrInboundClientNotFound is returned when an inbound client is not found.
	ErrInboundClientNotFound = errors.New("inbound client not found")

	// ErrInboundClientDataCorrupted is returned when a file-based inbound client cannot be
	// read back as the expected Go type.
	ErrInboundClientDataCorrupted = errors.New("inbound client data is corrupted")

	// ErrCompositeResultLimitExceeded is returned when the composite list result exceeds the
	// configured limit across file + DB stores.
	ErrCompositeResultLimitExceeded = errors.New("composite store result limit exceeded")

	// ErrCannotModifyDeclarative is returned by mutating service methods (Create/Update/
	// SyncOAuthProfile/Delete) when the targeted entity is sourced from a declarative file.
	// Callers translate this into their own "declarative resource is read-only" error.
	ErrCannotModifyDeclarative = errors.New("cannot modify declarative inbound client")
)

var (
	// ErrFKInvalidAuthFlow and related errors are returned when a caller-supplied ID does not resolve.
	ErrFKInvalidAuthFlow = errors.New("invalid auth flow ID")
	// ErrFKInvalidRegistrationFlow is returned when the registration flow ID does not exist.
	ErrFKInvalidRegistrationFlow = errors.New("invalid registration flow ID")
	// ErrFKInvalidRecoveryFlow is returned when the recovery flow ID does not exist.
	ErrFKInvalidRecoveryFlow = errors.New("invalid recovery flow ID")
	// ErrFKFlowDefinitionRetrievalFailed is returned when a flow definition cannot be retrieved.
	ErrFKFlowDefinitionRetrievalFailed = errors.New("error retrieving flow definition")
	// ErrFKFlowServerError is returned when a server error occurs while resolving a flow.
	ErrFKFlowServerError = errors.New("server error while resolving flow")
	// ErrFKThemeNotFound is returned when the specified theme does not exist.
	ErrFKThemeNotFound = errors.New("theme not found")
	// ErrFKLayoutNotFound is returned when the specified layout does not exist.
	ErrFKLayoutNotFound = errors.New("layout not found")
	// ErrFKInvalidUserType is returned when the specified user type is invalid.
	ErrFKInvalidUserType = errors.New("invalid user type")
	// ErrUserSchemaLookupFailed is returned when the user-schema service fails (e.g. DB outage)
	// while validating allowed user types. Distinct from ErrFKInvalidUserType so the handler
	// can map it to a server error instead of a client validation error.
	ErrUserSchemaLookupFailed = errors.New("user schema lookup failed")
	// ErrInvalidUserAttribute is returned when a user attribute is not valid for any of the allowed user types.
	ErrInvalidUserAttribute = errors.New("invalid user attribute")

	// ErrOAuthInvalidRedirectURI is returned when the redirect URI is invalid.
	ErrOAuthInvalidRedirectURI = errors.New("invalid redirect URI")
	// ErrOAuthRedirectURIFragmentNotAllowed is returned when a redirect URI contains a fragment.
	ErrOAuthRedirectURIFragmentNotAllowed = errors.New("redirect URI must not contain a fragment")
	// ErrOAuthAuthCodeRequiresRedirectURIs is returned when authorization_code grant has no redirect URIs.
	ErrOAuthAuthCodeRequiresRedirectURIs = errors.New("authorization_code grant requires redirect URIs")
	// ErrOAuthInvalidGrantType is returned when an unsupported grant type is specified.
	ErrOAuthInvalidGrantType = errors.New("invalid grant type")
	// ErrOAuthInvalidResponseType is returned when an unsupported response type is specified.
	ErrOAuthInvalidResponseType = errors.New("invalid response type")
	// ErrOAuthClientCredentialsCannotUseResponseTypes is returned when client_credentials uses response types.
	ErrOAuthClientCredentialsCannotUseResponseTypes = errors.New("client_credentials grant cannot use response types")
	// ErrOAuthAuthCodeRequiresCodeResponseType is returned when authorization_code grant lacks code response type.
	ErrOAuthAuthCodeRequiresCodeResponseType = errors.New("authorization_code grant requires code response type")
	// ErrOAuthRefreshTokenCannotBeSoleGrant is returned when refresh_token is the only grant type.
	ErrOAuthRefreshTokenCannotBeSoleGrant = errors.New("refresh_token cannot be the sole grant type")
	// ErrOAuthPKCERequiresAuthCode is returned when PKCE is enabled without authorization_code grant.
	ErrOAuthPKCERequiresAuthCode = errors.New("PKCE requires authorization_code grant type")
	// ErrOAuthResponseTypesRequireAuthCode is returned when response types are set without authorization_code grant.
	ErrOAuthResponseTypesRequireAuthCode = errors.New("response types require authorization_code grant type")
	// ErrOAuthInvalidTokenEndpointAuthMethod is returned when an unsupported auth method is specified.
	ErrOAuthInvalidTokenEndpointAuthMethod = errors.New("invalid token endpoint auth method")
	// ErrOAuthPrivateKeyJWTRequiresCertificate is returned when private_key_jwt is used without a certificate.
	ErrOAuthPrivateKeyJWTRequiresCertificate = errors.New("private_key_jwt requires a certificate")
	// ErrOAuthPrivateKeyJWTCannotHaveClientSecret is returned when private_key_jwt is used with a client secret.
	ErrOAuthPrivateKeyJWTCannotHaveClientSecret = errors.New("private_key_jwt cannot have a client secret")
	// ErrOAuthClientSecretCannotHaveCertificate is returned when client-secret auth is used with a certificate.
	ErrOAuthClientSecretCannotHaveCertificate = errors.New("client secret auth cannot have a certificate")
	// ErrOAuthNoneAuthRequiresPublicClient is returned when none auth method is used without a public client.
	ErrOAuthNoneAuthRequiresPublicClient = errors.New("none auth method requires a public client")
	// ErrOAuthNoneAuthCannotHaveCertOrSecret is returned when none auth method is used with a certificate or secret.
	ErrOAuthNoneAuthCannotHaveCertOrSecret = errors.New("none auth method cannot have certificate or secret")
	// ErrOAuthClientCredentialsCannotUseNoneAuth is returned when client_credentials uses none auth method.
	ErrOAuthClientCredentialsCannotUseNoneAuth = errors.New("client_credentials cannot use none auth method")
	// ErrOAuthPublicClientMustUseNoneAuth is returned when a public client uses an auth method other than none.
	ErrOAuthPublicClientMustUseNoneAuth = errors.New("public client must use none auth method")
	// ErrOAuthPublicClientMustHavePKCE is returned when a public client does not have PKCE required.
	ErrOAuthPublicClientMustHavePKCE = errors.New("public client must have PKCE required")

	// ErrCertValueRequired is returned when a certificate value is missing.
	ErrCertValueRequired = errors.New("certificate value is required")
	// ErrCertInvalidJWKSURI is returned when the JWKS URI is invalid.
	ErrCertInvalidJWKSURI = errors.New("invalid JWKS URI")
	// ErrCertInvalidType is returned when the certificate type is invalid.
	ErrCertInvalidType = errors.New("invalid certificate type")

	// ErrOAuthUserInfoUnsupportedSigningAlg is returned when the userinfo signing algorithm is not supported.
	ErrOAuthUserInfoUnsupportedSigningAlg = errors.New("unsupported userinfo signing algorithm")
	// ErrOAuthUserInfoUnsupportedEncryptionAlg is returned when the userinfo encryption algorithm is not supported.
	ErrOAuthUserInfoUnsupportedEncryptionAlg = errors.New("unsupported userinfo encryption algorithm")
	// ErrOAuthUserInfoUnsupportedEncryptionEnc is returned when the userinfo content-encryption alg is not supported.
	ErrOAuthUserInfoUnsupportedEncryptionEnc = errors.New("unsupported userinfo content-encryption algorithm")
	// ErrOAuthUserInfoEncryptionAlgRequiresEnc is returned when encryptionAlg is set without encryptionEnc.
	ErrOAuthUserInfoEncryptionAlgRequiresEnc = errors.New(
		"userinfo encryptionEnc is required when encryptionAlg is set")
	// ErrOAuthUserInfoEncryptionEncRequiresAlg is returned when encryptionEnc is set without encryptionAlg.
	ErrOAuthUserInfoEncryptionEncRequiresAlg = errors.New(
		"userinfo encryptionAlg is required when encryptionEnc is set")
	// ErrOAuthUserInfoEncryptionRequiresCertificate is returned when userinfo encryption has no certificate.
	ErrOAuthUserInfoEncryptionRequiresCertificate = errors.New(
		"userinfo encryption requires a certificate (JWKS or JWKS_URI)")
	// ErrOAuthUserInfoJWKSURINotSSRFSafe is returned when the JWKS URI fails SSRF safety checks.
	ErrOAuthUserInfoJWKSURINotSSRFSafe = errors.New("userinfo JWKS URI must be a publicly reachable HTTPS URL")
	// ErrOAuthUserInfoUnsupportedResponseType is returned when an unsupported userinfo response type is specified.
	ErrOAuthUserInfoUnsupportedResponseType = errors.New("unsupported userinfo response type")
	// ErrOAuthUserInfoJWSRequiresSigningAlg is returned when responseType is JWS but signingAlg is not set.
	ErrOAuthUserInfoJWSRequiresSigningAlg = errors.New("signingAlg is required when userinfo responseType is JWS")
	// ErrOAuthUserInfoJWERequiresEncryption is returned when responseType is JWE but encryption fields are missing.
	ErrOAuthUserInfoJWERequiresEncryption = errors.New(
		"encryptionAlg and encryptionEnc are required when userinfo responseType is JWE")
	// ErrOAuthUserInfoNestedJWTRequiresAll is returned when responseType is NESTED_JWT but fields are missing.
	ErrOAuthUserInfoNestedJWTRequiresAll = errors.New(
		"signingAlg, encryptionAlg, and encryptionEnc are required when userinfo responseType is NESTED_JWT")
	// ErrOAuthUserInfoAlgRequiresResponseType is returned when algorithm fields
	// are set without an explicit responseType.
	ErrOAuthUserInfoAlgRequiresResponseType = errors.New(
		"userinfo responseType is required when signingAlg or encryptionAlg is set")

	// ErrOAuthIDTokenUnsupportedEncryptionAlg is returned when the ID token encryption algorithm is not supported.
	ErrOAuthIDTokenUnsupportedEncryptionAlg = errors.New("unsupported ID token encryption algorithm")
	// ErrOAuthIDTokenUnsupportedEncryptionEnc is returned when the ID token content-encryption
	// algorithm is not supported.
	ErrOAuthIDTokenUnsupportedEncryptionEnc = errors.New("unsupported ID token content-encryption algorithm")
	// ErrOAuthIDTokenEncryptionAlgRequiresEnc is returned when encryptionAlg is set without encryptionEnc.
	ErrOAuthIDTokenEncryptionAlgRequiresEnc = errors.New(
		"idToken encryptionEnc is required when encryptionAlg is set")
	// ErrOAuthIDTokenEncryptionEncRequiresAlg is returned when encryptionEnc is set without encryptionAlg.
	ErrOAuthIDTokenEncryptionEncRequiresAlg = errors.New(
		"idToken encryptionAlg is required when encryptionEnc is set")
	// ErrOAuthIDTokenEncryptionRequiresCertificate is returned when ID token encryption has no certificate.
	ErrOAuthIDTokenEncryptionRequiresCertificate = errors.New(
		"idToken encryption requires a certificate (JWKS or JWKS_URI)")
	// ErrOAuthIDTokenJWKSURINotSSRFSafe is returned when the JWKS URI fails SSRF safety checks.
	ErrOAuthIDTokenJWKSURINotSSRFSafe = errors.New("idToken JWKS URI must be a publicly reachable HTTPS URL")
	// ErrOAuthIDTokenUnsupportedResponseType is returned when an unsupported ID token response type is specified.
	ErrOAuthIDTokenUnsupportedResponseType = errors.New("unsupported ID token response type")
	// ErrOAuthIDTokenEncryptionFieldsNotAllowed is returned when encryption fields are set for JWT responseType.
	ErrOAuthIDTokenEncryptionFieldsNotAllowed = errors.New(
		"idToken encryptionAlg and encryptionEnc must not be set when responseType is JWT")
)

// Certificate operation labels used in CertOperationError.
const (
	CertOpCreate   = "create"
	CertOpUpdate   = "update"
	CertOpDelete   = "delete"
	CertOpRetrieve = "retrieve"
)

// CertOperationError wraps an underlying cert.Service error along with the operation that
// produced it and the reference type involved.
type CertOperationError struct {
	Operation  string
	RefType    cert.CertificateReferenceType
	Underlying *serviceerror.ServiceError
}

// Error implements the error interface.
func (e *CertOperationError) Error() string {
	if e.Underlying != nil {
		return e.Underlying.ErrorDescription.DefaultValue
	}
	return "certificate operation failed"
}

// IsClientError reports whether the underlying cert service error is a client error.
func (e *CertOperationError) IsClientError() bool {
	return e.Underlying != nil && e.Underlying.Type == serviceerror.ClientErrorType
}

// ConsentSyncError wraps an underlying ServiceError from the consent service, allowing callers
// to translate it into their own error vocabulary.
type ConsentSyncError struct {
	Underlying *serviceerror.ServiceError
}

// Error implements the error interface. Falls back through (description → code → generic) so
// the returned string is never empty even when the underlying error has no description.
func (e *ConsentSyncError) Error() string {
	if e.Underlying != nil {
		if msg := e.Underlying.ErrorDescription.DefaultValue; msg != "" {
			return msg
		}
		if e.Underlying.Code != "" {
			return "consent sync failed (code " + e.Underlying.Code + ")"
		}
	}
	return "consent sync failed"
}

// IsClientError reports whether the underlying error is a client error.
func (e *ConsentSyncError) IsClientError() bool {
	return e.Underlying != nil && e.Underlying.Type == serviceerror.ClientErrorType
}

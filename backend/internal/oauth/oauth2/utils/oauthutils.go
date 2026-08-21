// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package utils provides utility functions for OAuth2 operations.
package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/system/utils"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
)

// GetURIWithQueryParams constructs a URI with the given query parameters.
// It validates the error code and error description according to the spec.
func GetURIWithQueryParams(uri string, queryParams map[string]string) (string, error) {
	// Validate the error params if present.
	if err := validateErrorParams(queryParams[constants.RequestParamError],
		queryParams[constants.RequestParamErrorDescription]); err != nil {
		return "", err
	}

	return utils.GetURIWithQueryParams(uri, queryParams)
}

// allowedErrorParamChars matches the character set permitted for the error and error_description
// parameters: %x20-21 / %x23-5B / %x5D-7E.
var allowedErrorParamChars = regexp.MustCompile(`^[\x20-\x21\x23-\x5B\x5D-\x7E]*$`)

// maxErrorDescriptionLength bounds the error description carried in a client redirect.
const maxErrorDescriptionLength = 256

// validateErrorParams validates the error code and error description parameters.
func validateErrorParams(err, desc string) error {
	// Validate the error code.
	if err != "" && !allowedErrorParamChars.MatchString(err) {
		return fmt.Errorf("invalid error code: %s", err)
	}

	// Validate the error description.
	if desc != "" && !allowedErrorParamChars.MatchString(desc) {
		return fmt.Errorf("invalid error description: %s", desc)
	}

	return nil
}

// SanitizeErrorDescription drops characters the spec disallows in an error description and truncates
// the result, so a description sourced from a flow error cannot make the client redirect unbuildable.
// It returns "" when nothing usable remains, letting the caller fall back to its own message.
func SanitizeErrorDescription(desc string) string {
	sanitized := strings.Map(func(r rune) rune {
		if allowedErrorParamChars.MatchString(string(r)) {
			return r
		}
		return -1
	}, desc)

	sanitized = strings.TrimSpace(sanitized)
	if len(sanitized) > maxErrorDescriptionLength {
		sanitized = strings.TrimSpace(sanitized[:maxErrorDescriptionLength])
	}

	return sanitized
}

const (
	// OAuth2ClientIDLength specifies the byte length for OAuth client IDs (16 bytes = 128 bits)
	// This provides sufficient entropy while keeping the resulting base64 string reasonably short
	OAuth2ClientIDLength = 16

	// OAuth2ClientSecretLength specifies the byte length for OAuth client secrets (32 bytes = 256 bits)
	// This provides high entropy for cryptographic security as recommended by OAuth security best practices
	OAuth2ClientSecretLength = 32

	// OAuth2AuthorizationCodeLength specifies the byte length for OAuth authorization codes (20 bytes = 160 bits)
	// This requires guessing probability ≤ 2^(-128) and recommends ≤ 2^(-160)
	OAuth2AuthorizationCodeLength = 20
)

// OAuth2CredentialType represents the type of OAuth 2.0 credential to generate
type OAuth2CredentialType string

const (
	// ClientIDCredential represents an OAuth 2.0 client identifier
	ClientIDCredential OAuth2CredentialType = "client ID"

	// ClientSecretCredential represents an OAuth 2.0 client secret
	ClientSecretCredential OAuth2CredentialType = "client secret"

	// AuthorizationCodeCredential represents an OAuth 2.0 authorization code
	AuthorizationCodeCredential OAuth2CredentialType = "authorization code"
)

// generateOAuth2Credential generates a base64url-encoded OAuth 2.0 credential.
// This private method contains the common logic for generating both client IDs and secrets.
// The length is automatically determined based on the credential type to ensure OAuth compliance.
func generateOAuth2Credential(credentialType OAuth2CredentialType) (string, error) {
	var length int

	switch credentialType {
	case ClientIDCredential:
		length = OAuth2ClientIDLength
	case ClientSecretCredential:
		length = OAuth2ClientSecretLength
	case AuthorizationCodeCredential:
		length = OAuth2AuthorizationCodeLength
	default:
		return "", fmt.Errorf("unsupported credential type: %s", credentialType)
	}

	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes for OAuth %s: %w", credentialType, err)
	}

	// Use base64 URL encoding without padding for web-friendly credentials
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

// GenerateOAuth2ClientID generates a URL-safe OAuth 2.0 client identifier.
func GenerateOAuth2ClientID() (string, error) {
	return generateOAuth2Credential(ClientIDCredential)
}

// GenerateOAuth2ClientSecret generates a cryptographically secure OAuth 2.0 client secret.
func GenerateOAuth2ClientSecret() (string, error) {
	return generateOAuth2Credential(ClientSecretCredential)
}

// GenerateAuthorizationCode generates a cryptographically secure OAuth 2.0 authorization code.
func GenerateAuthorizationCode() (string, error) {
	return generateOAuth2Credential(AuthorizationCodeCredential)
}

// SeparateOIDCAndNonOIDCScopes separates the given scopes into OIDC and non-OIDC scopes.
// A scope is OIDC if it is a standard OIDC scope or the app's scope-to-claims mapping defines it.
// Anything else is a permission scope for the resource servers.
func SeparateOIDCAndNonOIDCScopes(scopes string, scopeClaimsMapping map[string][]string) ([]string, []string) {
	scopeSlice := utils.ParseStringArray(scopes, " ")
	var oidcScopes []string
	var nonOidcScopes []string

	for _, scp := range scopeSlice {
		_, isMapped := scopeClaimsMapping[scp]
		_, isStandard := constants.StandardOIDCScopes[scp]
		if isMapped || isStandard {
			oidcScopes = append(oidcScopes, scp)
		} else {
			nonOidcScopes = append(nonOidcScopes, scp)
		}
	}
	return oidcScopes, nonOidcScopes
}

// ResolveScopeClaims returns the claims a scope releases. The app's mapping wins where it defines
// the scope, so mapping it to a shorter list narrows what it releases; a standard OIDC scope the
// app leaves unmapped falls back to its standard claims. Returns nil for an unmapped custom scope,
// which therefore releases nothing.
func ResolveScopeClaims(scope string, scopeClaimsMapping map[string][]string) []string {
	if claims, ok := scopeClaimsMapping[scope]; ok {
		return claims
	}
	if standard, ok := constants.StandardOIDCScopes[scope]; ok {
		return standard.Claims
	}
	return nil
}

// DefaultScopeClaims returns the standard OIDC scope-to-claims mapping a client starts with when
// created without one. openid is left out: it is always granted and carries no user attributes.
func DefaultScopeClaims() map[string][]string {
	defaults := make(map[string][]string, len(constants.StandardOIDCScopes))
	for scope, def := range constants.StandardOIDCScopes {
		if scope == constants.ScopeOpenID {
			continue
		}
		claims := make([]string, len(def.Claims))
		copy(claims, def.Claims)
		defaults[scope] = claims
	}
	return defaults
}

// ParseClaimsRequest parses the claims parameter JSON string into a ClaimsRequest struct.
// Returns nil if the input is empty.
// Returns an error if the JSON is malformed or violates OIDC spec constraints.
func ParseClaimsRequest(claimsParam string) (*model.ClaimsRequest, error) {
	if claimsParam == "" {
		return nil, nil
	}

	var claimsRequest model.ClaimsRequest
	if err := json.Unmarshal([]byte(claimsParam), &claimsRequest); err != nil {
		return nil, fmt.Errorf("invalid claims parameter: %w", err)
	}

	// Validate claims request
	if err := validateClaimsRequest(&claimsRequest); err != nil {
		return nil, err
	}

	return &claimsRequest, nil
}

// validateClaimsRequest validates a ClaimsRequest against OIDC spec constraints. Normal claims
// and verified_claims are already normalized and structurally validated by
// ClaimsRequest.UnmarshalJSON; here only the normal-claim constraint grammar is enforced.
func validateClaimsRequest(cr *model.ClaimsRequest) error {
	if cr == nil {
		return nil
	}

	// Validate normal userinfo claims
	for claimName, claimReq := range cr.UserInfo {
		if err := validateIndividualClaimRequest("userinfo", claimName, claimReq); err != nil {
			return err
		}
	}

	// Validate id_token claims
	for claimName, claimReq := range cr.IDToken {
		if err := validateIndividualClaimRequest("id_token", claimName, claimReq); err != nil {
			return err
		}
	}

	return nil
}

// validateIndividualClaimRequest validates constraints for an individual claim request.
func validateIndividualClaimRequest(location, claimName string, icr *model.IndividualClaimRequest) error {
	if err := icr.Validate(); err != nil {
		return fmt.Errorf("invalid claims parameter: claim '%s' in %s %w", claimName, location, err)
	}
	return nil
}

// SerializeClaimsRequest serializes a ClaimsRequest to JSON string.
// Returns empty string if the claims request is nil or empty.
func SerializeClaimsRequest(cr *model.ClaimsRequest) (string, error) {
	if cr == nil || cr.IsEmpty() {
		return "", nil
	}

	data, err := json.Marshal(cr)
	if err != nil {
		return "", fmt.Errorf("failed to serialize claims request: %w", err)
	}

	return string(data), nil
}

// EnsureClientSubTypeAttribute selects the sub_type claim in a client's own access token attribute
// list, returning the updated token config. Called at client creation only, so a claim removed later
// stays removed. Selection alone enables the claim; the builder supplies its value.
func EnsureClientSubTypeAttribute(token *providers.OAuthTokenConfig) *providers.OAuthTokenConfig {
	if token == nil {
		token = &providers.OAuthTokenConfig{}
	}
	if token.AccessToken == nil {
		token.AccessToken = &providers.AccessTokenConfig{}
	}
	if token.AccessToken.ClientConfig == nil {
		token.AccessToken.ClientConfig = &providers.AccessTokenSubConfig{}
	}

	clientConfig := token.AccessToken.ClientConfig
	if !slices.Contains(clientConfig.Attributes, constants.ClaimSubType) {
		clientConfig.Attributes = append(clientConfig.Attributes, constants.ClaimSubType)
	}
	return token
}

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

// Package utils provides utility functions for OAuth2 operations.
package utils

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/system/utils"
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

// validateErrorParams validates the error code and error description parameters.
func validateErrorParams(err, desc string) error {
	// Define a regex pattern for the allowed character set: %x20-21 / %x23-5B / %x5D-7E
	allowedCharPattern := `^[\x20-\x21\x23-\x5B\x5D-\x7E]*$`
	allowedCharRegex := regexp.MustCompile(allowedCharPattern)

	// Validate the error code.
	if err != "" && !allowedCharRegex.MatchString(err) {
		return fmt.Errorf("invalid error code: %s", err)
	}

	// Validate the error description.
	if desc != "" && !allowedCharRegex.MatchString(desc) {
		return fmt.Errorf("invalid error description: %s", desc)
	}

	return nil
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
// A scope is treated as OIDC if it is a standard OIDC scope or is present in the app's
// custom scope_claims mapping.
func SeparateOIDCAndNonOIDCScopes(scopes string, scopeClaimsMapping map[string][]string) ([]string, []string) {
	scopeSlice := utils.ParseStringArray(scopes, " ")
	var oidcScopes []string
	var nonOidcScopes []string

	for _, scp := range scopeSlice {
		_, isStandard := constants.StandardOIDCScopes[scp]
		_, isCustomOIDC := scopeClaimsMapping[scp]
		if isStandard || isCustomOIDC {
			oidcScopes = append(oidcScopes, scp)
		} else {
			nonOidcScopes = append(nonOidcScopes, scp)
		}
	}
	return oidcScopes, nonOidcScopes
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

// validateClaimsRequest validates a ClaimsRequest against OIDC spec constraints.
func validateClaimsRequest(cr *model.ClaimsRequest) error {
	if cr == nil {
		return nil
	}

	// Validate userinfo claims
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
	if icr == nil {
		return nil
	}

	// value and values are mutually exclusive
	if icr.Value != nil && len(icr.Values) > 0 {
		return fmt.Errorf(
			"invalid claims parameter: claim '%s' in %s has both 'value' and 'values' specified "+
				"(mutually exclusive per OIDC spec)",
			claimName, location)
	}

	// values array must contain at least one value
	if icr.Values != nil && len(icr.Values) == 0 {
		return fmt.Errorf(
			"invalid claims parameter: claim '%s' in %s has empty 'values' array "+
				"(must contain at least one value)",
			claimName, location)
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

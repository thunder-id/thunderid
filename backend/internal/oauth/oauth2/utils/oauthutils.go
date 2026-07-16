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
	"errors"
	"fmt"
	"regexp"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	"github.com/thunder-id/thunderid/internal/system/utils"
)

// errMalformedStatusReference indicates a token that carries a status_list object but whose uri or idx
// is invalid. It is distinct from an absent reference: a present-but-invalid reference cannot be
// resolved, so the caller must fail closed rather than treat the token as having no revocation channel.
var errMalformedStatusReference = errors.New("malformed status list reference")

// ExtractStatusListReference reads the Token Status List reference from a token's claims: the list URI
// and index carried under status.status_list (draft-ietf-oauth-status-list). ok is true only for a
// present, well-formed reference. A token with no status_list object returns ok=false with a nil error
// (the feature is off, or the token predates it). A token that carries a status_list object with an
// invalid uri or idx returns a non-nil error so the caller fails closed instead of skipping revocation.
// The index is decoded from a JSON number (float64) as produced by JWT decoding.
func ExtractStatusListReference(claims map[string]interface{}) (uri string, idx int64, ok bool, err error) {
	status, isMap := claims[constants.ClaimStatus].(map[string]interface{})
	if !isMap {
		return "", 0, false, nil
	}
	statusList, isMap := status[constants.ClaimStatusList].(map[string]interface{})
	if !isMap {
		return "", 0, false, nil
	}
	// A status_list object is present, so the token opts into Token Status List revocation. From here a
	// malformed uri or idx is a present-but-invalid reference, not an absent one.
	uri, isStr := statusList[constants.ClaimStatusListURI].(string)
	if !isStr || uri == "" {
		return "", 0, false, errMalformedStatusReference
	}
	// idx must be a non-negative integer (draft-ietf-oauth-status-list §6.1). Reject fractional or
	// negative values rather than silently truncating.
	switch v := statusList[constants.ClaimStatusListIdx].(type) {
	case float64:
		if v < 0 || v != float64(int64(v)) {
			return "", 0, false, errMalformedStatusReference
		}
		return uri, int64(v), true, nil
	case int64:
		if v < 0 {
			return "", 0, false, errMalformedStatusReference
		}
		return uri, v, true, nil
	default:
		return "", 0, false, errMalformedStatusReference
	}
}

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

// FilterOIDCScopesByAllowedScopes filters requested OIDC scopes against the application's active scopes.
func FilterOIDCScopesByAllowedScopes(oidcScopes []string, allowedScopes []string) []string {
	if allowedScopes == nil {
		return oidcScopes
	}

	allowedScopeSet := make(map[string]struct{}, len(allowedScopes))
	for _, scope := range allowedScopes {
		allowedScopeSet[scope] = struct{}{}
	}

	filteredScopes := make([]string, 0, len(oidcScopes))
	for _, scope := range oidcScopes {
		if _, ok := allowedScopeSet[scope]; ok {
			filteredScopes = append(filteredScopes, scope)
		}
	}
	return filteredScopes
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

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

// Package requestvalidator provides shared validation for OAuth2 authorization
// request parameters used by both the authorize and PAR endpoints.
package requestvalidator

import (
	"slices"
	"strings"

	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/pkce"
)

// ValidateAuthorizationRequestParams validates the common authorization request parameters
// shared by both the standard authorize endpoint and the PAR endpoint.
//
// This validates: prompt, grant_type, response_type, PKCE, and nonce.
// Callers are responsible for validating client_id and redirect_uri before calling this
// function, since those validations have endpoint-specific error handling semantics
// (e.g., the authorize endpoint must not redirect errors when the redirect_uri is invalid).
// Resource indicators (RFC 8707) are multi-valued and must be validated separately by callers
// via resourceindicators.ValidateResourceURIs.
//
// Returns (errorCode, errorDescription). Empty errorCode means validation passed.
func ValidateAuthorizationRequestParams(
	params map[string]string, oauthApp *inboundmodel.OAuthClient,
) (string, string) {
	responseType := params[constants.RequestParamResponseType]

	// Validate the prompt parameter if present.
	prompt, promptExists := params[constants.RequestParamPrompt]
	if promptExists {
		if errCode, errMsg := ValidatePromptParameter(prompt); errCode != "" {
			return errCode, errMsg
		}
	}

	// Validate grant type is allowed.
	if !oauthApp.IsAllowedGrantType(constants.GrantTypeAuthorizationCode) {
		return constants.ErrorUnauthorizedClient,
			"Authorization code grant type is not allowed for the client"
	}

	// Validate response type.
	if responseType == "" {
		return constants.ErrorInvalidRequest, "Missing response_type parameter"
	}
	if !oauthApp.IsAllowedResponseType(responseType) {
		return constants.ErrorUnsupportedResponseType, "Unsupported response type"
	}

	// Validate PKCE parameters.
	if responseType == string(constants.ResponseTypeCode) {
		codeChallenge := params[constants.RequestParamCodeChallenge]
		codeChallengeMethod := params[constants.RequestParamCodeChallengeMethod]

		if oauthApp.RequiresPKCE() && codeChallenge == "" {
			return constants.ErrorInvalidRequest, "code_challenge is required for this application"
		}

		if codeChallenge != "" || codeChallengeMethod != "" {
			if err := pkce.ValidateCodeChallenge(codeChallenge, codeChallengeMethod); err != nil {
				return constants.ErrorInvalidRequest,
					"Invalid code_challenge or code_challenge_method parameter"
			}
		}
	}

	// Validate nonce length.
	nonce := params[constants.RequestParamNonce]
	if nonce != "" && len(nonce) > constants.MaxNonceLength {
		return constants.ErrorInvalidRequest, "nonce exceeds maximum allowed length"
	}

	return "", ""
}

// ValidatePromptParameter validates the OIDC prompt parameter per OIDC Core §3.1.2.1.
// Returns (errorCode, errorDescription). Empty errorCode means validation passed.
func ValidatePromptParameter(prompt string) (string, string) {
	if strings.TrimSpace(prompt) == "" {
		return constants.ErrorInvalidRequest, "The prompt parameter cannot be empty"
	}

	values := strings.Fields(prompt)

	for _, v := range values {
		if !slices.Contains(constants.ValidPromptValues, v) {
			return constants.ErrorInvalidRequest, "Unsupported prompt parameter value"
		}
	}

	if slices.Contains(values, constants.PromptNone) {
		// "none" must not be combined with other values.
		if len(values) > 1 {
			return constants.ErrorInvalidRequest,
				"prompt value 'none' must not be combined with other values"
		}

		// The server does not support server-side sessions as of now.
		return constants.ErrorLoginRequired,
			"User authentication is required"
	}

	// The server does not support consent or account selection prompts as of now.
	if slices.Contains(values, constants.PromptConsent) {
		return constants.ErrorConsentRequired,
			"Consent is not supported"
	}

	if slices.Contains(values, constants.PromptSelectAccount) {
		return constants.ErrorAccountSelectionRequired,
			"Account selection is not supported"
	}

	return "", ""
}

// ResolveACRValues returns the effective acr_values: requested ACRs filtered against the
// app's list, falling back to the app's full list when nothing matches or none were requested.
func ResolveACRValues(requestedAcrValues string, appAcrValues []string) string {
	requested := parseACRValues(requestedAcrValues)
	filtered := make([]string, 0, len(requested))
	for _, acr := range requested {
		if slices.Contains(appAcrValues, acr) {
			filtered = append(filtered, acr)
		}
	}
	if len(filtered) == 0 {
		return strings.Join(appAcrValues, " ")
	}
	return strings.Join(filtered, " ")
}

// parseACRValues splits acr_values into a deduplicated, order-preserving slice.
func parseACRValues(acrValues string) []string {
	parts := strings.Fields(acrValues)
	seen := make(map[string]bool, len(parts))
	result := make([]string, 0, len(parts))
	for _, acr := range parts {
		if !seen[acr] {
			seen[acr] = true
			result = append(result, acr)
		}
	}
	return result
}

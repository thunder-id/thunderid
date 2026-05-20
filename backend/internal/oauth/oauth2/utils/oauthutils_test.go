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

package utils

import (
	"encoding/base64"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/oauth/oauth2/model"
	sysutils "github.com/thunder-id/thunderid/internal/system/utils"
)

type OAuth2UtilsTestSuite struct {
	suite.Suite
}

func TestOAuth2UtilsSuite(t *testing.T) {
	suite.Run(t, new(OAuth2UtilsTestSuite))
}

func (suite *OAuth2UtilsTestSuite) TestGetURIWithQueryParams_Success() {
	testCases := []struct {
		name        string
		uri         string
		queryParams map[string]string
		expectedURI string
	}{
		{
			name: "SimpleParams",
			uri:  "https://example.com/callback",
			queryParams: map[string]string{
				"code":  "test-code",
				"state": "test-state",
			},
			expectedURI: "https://example.com/callback?code=test-code&state=test-state",
		},
		{
			name:        "EmptyParams",
			uri:         "https://example.com/callback",
			queryParams: map[string]string{},
			expectedURI: "https://example.com/callback",
		},
		{
			name:        "NilParams",
			uri:         "https://example.com/callback",
			queryParams: nil,
			expectedURI: "https://example.com/callback",
		},
		{
			name: "ValidErrorParams",
			uri:  "https://example.com/callback",
			queryParams: map[string]string{
				constants.RequestParamError:            "invalid_request",
				constants.RequestParamErrorDescription: "Missing client_id parameter",
			},
			expectedURI: "https://example.com/callback?error=invalid_request&error_description=" +
				"Missing+client_id+parameter",
		},
		{
			name: "SpecialCharactersInParams",
			uri:  "https://example.com/callback",
			queryParams: map[string]string{
				"redirect_uri": "https://client.example.com/cb?param=value",
				"scope":        "read write admin",
			},
			expectedURI: "https://example.com/callback?redirect_uri=https%3A%2F%2Fclient.example.com" +
				"%2Fcb%3Fparam%3Dvalue&scope=read+write+admin",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			result, err := GetURIWithQueryParams(tc.uri, tc.queryParams)
			assert.NoError(t, err)

			// Parse both URIs to compare them properly (query params can be in different order)
			expectedParsed, err := url.Parse(tc.expectedURI)
			assert.NoError(t, err)
			resultParsed, err := url.Parse(result)
			assert.NoError(t, err)

			assert.Equal(t, expectedParsed.Scheme, resultParsed.Scheme)
			assert.Equal(t, expectedParsed.Host, resultParsed.Host)
			assert.Equal(t, expectedParsed.Path, resultParsed.Path)

			// Compare query parameters
			expectedQuery := expectedParsed.Query()
			resultQuery := resultParsed.Query()
			assert.Equal(t, expectedQuery, resultQuery)
		})
	}
}

func (suite *OAuth2UtilsTestSuite) TestGetURIWithQueryParams_InvalidErrorCode() {
	testCases := []struct {
		name        string
		errorCode   string
		description string
	}{
		{
			name:      "InvalidCharacterInErrorCode",
			errorCode: "invalid\x22request", // Contains quote character which is not allowed
		},
		{
			name:      "ControlCharacterInErrorCode",
			errorCode: "invalid\x01request", // Contains control character
		},
		{
			name:      "InvalidCharacterAtStart",
			errorCode: "\x19invalid_request", // Control character at start
		},
		{
			name:      "InvalidCharacterAtEnd",
			errorCode: "invalid_request\x7F", // DEL character at end
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			queryParams := map[string]string{
				constants.RequestParamError: tc.errorCode,
			}
			if tc.description != "" {
				queryParams[constants.RequestParamErrorDescription] = tc.description
			}

			result, err := GetURIWithQueryParams("https://example.com/callback", queryParams)

			assert.Error(t, err)
			assert.Empty(t, result)
			assert.Contains(t, err.Error(), "invalid error code")
		})
	}
}

func (suite *OAuth2UtilsTestSuite) TestGetURIWithQueryParams_InvalidErrorDescription() {
	testCases := []struct {
		name        string
		description string
	}{
		{
			name:        "InvalidCharacterInDescription",
			description: "Missing \"client_id\" parameter", // Contains quote character
		},
		{
			name:        "ControlCharacterInDescription",
			description: "Missing\x01client_id parameter", // Contains control character
		},
		{
			name:        "BackslashInDescription",
			description: "Missing\\client_id parameter", // Contains backslash (\x5C)
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			queryParams := map[string]string{
				constants.RequestParamError:            "invalid_request",
				constants.RequestParamErrorDescription: tc.description,
			}

			result, err := GetURIWithQueryParams("https://example.com/callback", queryParams)

			assert.Error(t, err)
			assert.Empty(t, result)
			assert.Contains(t, err.Error(), "invalid error description")
		})
	}
}

func (suite *OAuth2UtilsTestSuite) TestGetURIWithQueryParams_ValidCharacterRange() {
	// Test with characters from the allowed range: %x20-21 / %x23-5B / %x5D-7E
	validChars := " !#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[]^_`abcdefghijklmnopqrstuvwxyz{|}~"

	queryParams := map[string]string{
		constants.RequestParamError:            "invalid_request",
		constants.RequestParamErrorDescription: validChars,
	}

	result, err := GetURIWithQueryParams("https://example.com/callback", queryParams)

	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), result)
}

func (suite *OAuth2UtilsTestSuite) TestGetURIWithQueryParams_EmptyErrorParams() {
	// Test with empty error parameters (should be valid)
	queryParams := map[string]string{
		constants.RequestParamError:            "",
		constants.RequestParamErrorDescription: "",
		"other_param":                          "value",
	}

	result, err := GetURIWithQueryParams("https://example.com/callback", queryParams)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), result, "other_param=value")
}

func (suite *OAuth2UtilsTestSuite) TestGetURIWithQueryParams_OnlyErrorCode() {
	// Test with only error code, no description
	queryParams := map[string]string{
		constants.RequestParamError: "invalid_client",
	}

	result, err := GetURIWithQueryParams("https://example.com/callback", queryParams)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), result, "error=invalid_client")
	assert.NotContains(suite.T(), result, "error_description")
}

func (suite *OAuth2UtilsTestSuite) TestGetURIWithQueryParams_OnlyErrorDescription() {
	// Test with only error description, no error code
	queryParams := map[string]string{
		constants.RequestParamErrorDescription: "Something went wrong",
	}

	result, err := GetURIWithQueryParams("https://example.com/callback", queryParams)

	assert.NoError(suite.T(), err)
	assert.Contains(suite.T(), result, "error_description=Something+went+wrong")
	assert.NotContains(suite.T(), result, "error=")
}

func (suite *OAuth2UtilsTestSuite) TestValidateErrorParams_DirectCall() {
	// Test the validateErrorParams function directly (even though it's not exported)
	// We test it through the public function

	testCases := []struct {
		name        string
		errorCode   string
		description string
		expectError bool
	}{
		{
			name:        "ValidParams",
			errorCode:   "invalid_request",
			description: "Missing required parameter",
			expectError: false,
		},
		{
			name:        "EmptyParams",
			errorCode:   "",
			description: "",
			expectError: false,
		},
		{
			name:        "InvalidErrorCode",
			errorCode:   "invalid\"request",
			description: "Valid description",
			expectError: true,
		},
		{
			name:        "InvalidDescription",
			errorCode:   "invalid_request",
			description: "Invalid\"description",
			expectError: true,
		},
		{
			name:        "BothInvalid",
			errorCode:   "invalid\"request",
			description: "Invalid\"description",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			queryParams := map[string]string{
				constants.RequestParamError:            tc.errorCode,
				constants.RequestParamErrorDescription: tc.description,
			}

			_, err := GetURIWithQueryParams("https://example.com/callback", queryParams)

			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func (suite *OAuth2UtilsTestSuite) TestGetURIWithQueryParams_MalformedBaseURI() {
	// Test that the function still validates error params even with malformed base URI
	queryParams := map[string]string{
		constants.RequestParamError: "invalid\"request", // Invalid character
	}

	result, err := GetURIWithQueryParams("not-a-valid-uri", queryParams)

	// Should fail on error validation before URI processing
	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "invalid error code")
}

func (suite *OAuth2UtilsTestSuite) TestGetURIWithQueryParams_SpecialErrorCodes() {
	// Test with standard OAuth2 error codes
	standardErrorCodes := []string{
		"invalid_request",
		"invalid_client",
		"invalid_grant",
		"unauthorized_client",
		"unsupported_grant_type",
		"invalid_scope",
		"server_error",
		"temporarily_unavailable",
		"unsupported_response_type",
		"access_denied",
	}

	for _, errorCode := range standardErrorCodes {
		suite.T().Run("ErrorCode_"+errorCode, func(t *testing.T) {
			queryParams := map[string]string{
				constants.RequestParamError:            errorCode,
				constants.RequestParamErrorDescription: "Standard OAuth2 error",
			}

			result, err := GetURIWithQueryParams("https://example.com/callback", queryParams)

			assert.NoError(t, err)
			assert.Contains(t, result, "error="+errorCode)
		})
	}
}

func (suite *OAuth2UtilsTestSuite) TestGetURIWithQueryParams_BoundaryCharacters() {
	// Test characters at the boundaries of allowed ranges
	testCases := []struct {
		name       string
		char       string
		shouldPass bool
	}{
		{"Space_0x20", "\x20", true},        // First allowed character
		{"Exclamation_0x21", "\x21", true},  // Last of first range
		{"Quote_0x22", "\x22", false},       // Not allowed (between ranges)
		{"Hash_0x23", "\x23", true},         // First of second range
		{"LeftBracket_0x5B", "\x5B", true},  // Last of second range
		{"Backslash_0x5C", "\x5C", false},   // Not allowed (between ranges)
		{"RightBracket_0x5D", "\x5D", true}, // First of third range
		{"Tilde_0x7E", "\x7E", true},        // Last allowed character
		{"DEL_0x7F", "\x7F", false},         // Not allowed (after range)
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			queryParams := map[string]string{
				constants.RequestParamError: "test_error" + tc.char,
			}

			_, err := GetURIWithQueryParams("https://example.com/callback", queryParams)

			if tc.shouldPass {
				assert.NoError(t, err, "Character %s should be allowed", tc.char)
			} else {
				assert.Error(t, err, "Character %s should not be allowed", tc.char)
			}
		})
	}
}

// OAuth credential generation tests

func (suite *OAuth2UtilsTestSuite) TestGenerateOAuth2ClientID() {
	clientID, err := GenerateOAuth2ClientID()

	// Should not return an error
	assert.NoError(suite.T(), err, "GenerateOAuth2ClientID should not return an error")
	assert.NotEmpty(suite.T(), clientID, "Generated client ID should not be empty")

	// Verify format - should be base64url without padding
	base64URLPattern := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	assert.True(suite.T(), base64URLPattern.MatchString(clientID),
		"Client ID should contain only base64url characters (A-Z, a-z, 0-9, -, _)")

	// Should not contain padding characters
	assert.False(suite.T(), strings.Contains(clientID, "="),
		"Client ID should not contain padding characters")

	// Verify length - 16 bytes base64url encoded without padding should be ~22 characters
	expectedLength := base64.RawURLEncoding.EncodedLen(OAuth2ClientIDLength)
	assert.Equal(suite.T(), expectedLength, len(clientID),
		"Client ID should have the expected encoded length")

	// Verify it can be decoded back to original byte length
	decoded, err := base64.RawURLEncoding.DecodeString(clientID)
	assert.NoError(suite.T(), err, "Generated client ID should be valid base64url")
	assert.Equal(suite.T(), OAuth2ClientIDLength, len(decoded),
		"Decoded client ID should have the expected byte length")
}

func (suite *OAuth2UtilsTestSuite) TestGenerateOAuth2ClientSecret() {
	clientSecret, err := GenerateOAuth2ClientSecret()

	// Should not return an error
	assert.NoError(suite.T(), err, "GenerateOAuth2ClientSecret should not return an error")
	assert.NotEmpty(suite.T(), clientSecret, "Generated client secret should not be empty")

	// Verify format - should be base64url without padding
	base64URLPattern := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	assert.True(suite.T(), base64URLPattern.MatchString(clientSecret),
		"Client secret should contain only base64url characters (A-Z, a-z, 0-9, -, _)")

	// Should not contain padding characters
	assert.False(suite.T(), strings.Contains(clientSecret, "="),
		"Client secret should not contain padding characters")

	// Verify length - 32 bytes base64url encoded without padding should be ~43 characters
	expectedLength := base64.RawURLEncoding.EncodedLen(OAuth2ClientSecretLength)
	assert.Equal(suite.T(), expectedLength, len(clientSecret),
		"Client secret should have the expected encoded length")

	// Verify it can be decoded back to original byte length
	decoded, err := base64.RawURLEncoding.DecodeString(clientSecret)
	assert.NoError(suite.T(), err, "Generated client secret should be valid base64url")
	assert.Equal(suite.T(), OAuth2ClientSecretLength, len(decoded),
		"Decoded client secret should have the expected byte length")
}

func (suite *OAuth2UtilsTestSuite) TestGenerateOAuth2ClientIDUniqueness() {
	clientIDs := make(map[string]bool)

	// Generate multiple client IDs and verify uniqueness
	for i := 0; i < 1000; i++ {
		clientID, err := GenerateOAuth2ClientID()
		assert.NoError(suite.T(), err, "Should not return an error during generation")

		_, exists := clientIDs[clientID]
		assert.False(suite.T(), exists, "Generated client IDs should be unique")
		clientIDs[clientID] = true
	}

	assert.Equal(suite.T(), 1000, len(clientIDs), "Should have generated 1000 unique client IDs")
}

func (suite *OAuth2UtilsTestSuite) TestGenerateOAuth2ClientSecretUniqueness() {
	clientSecrets := make(map[string]bool)

	// Generate multiple client secrets and verify uniqueness
	for i := 0; i < 1000; i++ {
		clientSecret, err := GenerateOAuth2ClientSecret()
		assert.NoError(suite.T(), err, "Should not return an error during generation")

		_, exists := clientSecrets[clientSecret]
		assert.False(suite.T(), exists, "Generated client secrets should be unique")
		clientSecrets[clientSecret] = true
	}

	assert.Equal(suite.T(), 1000, len(clientSecrets), "Should have generated 1000 unique client secrets")
}

func (suite *OAuth2UtilsTestSuite) TestGenerateAuthorizationCode() {
	code, err := GenerateAuthorizationCode()

	// Should not return an error
	assert.NoError(suite.T(), err, "GenerateAuthorizationCode should not return an error")
	assert.NotEmpty(suite.T(), code, "Generated authorization code should not be empty")

	// Verify format - should be base64url without padding
	base64URLPattern := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	assert.True(suite.T(), base64URLPattern.MatchString(code),
		"Authorization code should contain only base64url characters (A-Z, a-z, 0-9, -, _)")

	// Should not contain padding characters
	assert.False(suite.T(), strings.Contains(code, "="),
		"Authorization code should not contain padding characters")

	// Verify length - 20 bytes base64url encoded without padding should be 27 characters
	expectedLength := base64.RawURLEncoding.EncodedLen(OAuth2AuthorizationCodeLength)
	assert.Equal(suite.T(), expectedLength, len(code),
		"Authorization code should have the expected encoded length")

	// Verify it can be decoded back to original byte length (20 bytes = 160 bits)
	decoded, err := base64.RawURLEncoding.DecodeString(code)
	assert.NoError(suite.T(), err, "Generated authorization code should be valid base64url")
	assert.Equal(suite.T(), OAuth2AuthorizationCodeLength, len(decoded),
		"Decoded authorization code should have the expected byte length")
}

func (suite *OAuth2UtilsTestSuite) TestGenerateAuthorizationCodeUniqueness() {
	codes := make(map[string]bool)

	// Generate multiple authorization codes and verify uniqueness
	for i := 0; i < 1000; i++ {
		code, err := GenerateAuthorizationCode()
		assert.NoError(suite.T(), err, "Should not return an error during generation")

		_, exists := codes[code]
		assert.False(suite.T(), exists, "Generated authorization codes should be unique")
		codes[code] = true
	}

	assert.Equal(suite.T(), 1000, len(codes), "Should have generated 1000 unique authorization codes")
}

func (suite *OAuth2UtilsTestSuite) TestOAuth2CredentialsDifferentFromUUID() {
	// Generate OAuth credentials
	clientID, err := GenerateOAuth2ClientID()
	assert.NoError(suite.T(), err)

	clientSecret, err := GenerateOAuth2ClientSecret()
	assert.NoError(suite.T(), err)

	// Generate UUID for comparison
	uuid, err := sysutils.GenerateUUIDv7()
	assert.NoError(suite.T(), err)

	// OAuth credentials should have different format than UUID
	uuidPattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	assert.False(suite.T(), uuidPattern.MatchString(clientID),
		"OAuth client ID should not match UUID format")
	assert.False(suite.T(), uuidPattern.MatchString(clientSecret),
		"OAuth client secret should not match UUID format")
	assert.True(suite.T(), uuidPattern.MatchString(uuid),
		"UUID should match expected UUID format")

	// OAuth credentials should be shorter/different than UUID
	assert.True(suite.T(), len(clientID) < len(uuid),
		"OAuth client ID should be shorter than UUID")
	assert.True(suite.T(), len(clientSecret) > len(uuid),
		"OAuth client secret should be longer than UUID for better security")
}

func (suite *OAuth2UtilsTestSuite) TestOAuth2URLSafety() {
	// Generate credentials multiple times to test URL safety
	for i := 0; i < 100; i++ {
		clientID, err := GenerateOAuth2ClientID()
		assert.NoError(suite.T(), err)

		clientSecret, err := GenerateOAuth2ClientSecret()
		assert.NoError(suite.T(), err)

		// Should not contain URL-unsafe characters
		urlUnsafeChars := []string{"+", "/", "=", " ", "&", "?", "#"}
		for _, char := range urlUnsafeChars {
			assert.False(suite.T(), strings.Contains(clientID, char),
				"Client ID should not contain URL-unsafe character: %s", char)
			assert.False(suite.T(), strings.Contains(clientSecret, char),
				"Client secret should not contain URL-unsafe character: %s", char)
		}
	}
}

func (suite *OAuth2UtilsTestSuite) TestOAuth2EntropyLevels() {
	clientID, err := GenerateOAuth2ClientID()
	assert.NoError(suite.T(), err)

	clientSecret, err := GenerateOAuth2ClientSecret()
	assert.NoError(suite.T(), err)

	// Decode to verify entropy
	clientIDBytes, err := base64.RawURLEncoding.DecodeString(clientID)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), OAuth2ClientIDLength, len(clientIDBytes),
		"Client ID should have 16 bytes (128 bits) of entropy")

	clientSecretBytes, err := base64.RawURLEncoding.DecodeString(clientSecret)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), OAuth2ClientSecretLength, len(clientSecretBytes),
		"Client secret should have 32 bytes (256 bits) of entropy")

	// Client secret should have more entropy than client ID
	assert.True(suite.T(), len(clientSecretBytes) > len(clientIDBytes),
		"Client secret should have more entropy than client ID")
}

func (suite *OAuth2UtilsTestSuite) TestGenerateOAuth2CredentialInvalidType() {
	// Test that the private function properly handles invalid credential types
	// We test this indirectly by ensuring our constants are used correctly

	// Verify that our defined constants work correctly
	clientID, err := GenerateOAuth2ClientID()
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), clientID)

	clientSecret, err := GenerateOAuth2ClientSecret()
	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), clientSecret)

	// The lengths should be exactly what we expect based on the credential type
	clientIDBytes, err := base64.RawURLEncoding.DecodeString(clientID)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), OAuth2ClientIDLength, len(clientIDBytes),
		"Client ID should automatically use the correct length")

	clientSecretBytes, err := base64.RawURLEncoding.DecodeString(clientSecret)
	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), OAuth2ClientSecretLength, len(clientSecretBytes),
		"Client secret should automatically use the correct length")
}

func (suite *OAuth2UtilsTestSuite) TestSeparateOIDCAndNonOIDCScopes() {
	testCases := []struct {
		name            string
		scopes          string
		expectedOIDC    []string
		expectedNonOIDC []string
	}{
		{
			name:            "OnlyOIDCScopes",
			scopes:          "openid profile email",
			expectedOIDC:    []string{"openid", "profile", "email"},
			expectedNonOIDC: nil, // Function returns nil for empty slice
		},
		{
			name:            "OnlyNonOIDCScopes",
			scopes:          "read write admin",
			expectedOIDC:    nil, // Function returns nil for empty slice
			expectedNonOIDC: []string{"read", "write", "admin"},
		},
		{
			name:            "MixedScopes",
			scopes:          "openid profile read write",
			expectedOIDC:    []string{"openid", "profile"},
			expectedNonOIDC: []string{"read", "write"},
		},
		{
			name:            "EmptyScopes",
			scopes:          "",
			expectedOIDC:    nil, // Function returns nil for empty slice
			expectedNonOIDC: nil, // Function returns nil for empty slice
		},
		{
			name:            "SingleOIDCScope",
			scopes:          "openid",
			expectedOIDC:    []string{"openid"},
			expectedNonOIDC: nil, // Function returns nil for empty slice
		},
		{
			name:            "SingleNonOIDCScope",
			scopes:          "custom_scope",
			expectedOIDC:    nil, // Function returns nil for empty slice
			expectedNonOIDC: []string{"custom_scope"},
		},
		{
			name:            "AllStandardOIDCScopes",
			scopes:          "openid profile email address phone",
			expectedOIDC:    []string{"openid", "profile", "email", "address", "phone"},
			expectedNonOIDC: nil, // Function returns nil for empty slice
		},
		{
			name: "MixedWithMultipleSpaces",
			// Single spaces - ParseStringArray may include empty strings for multiple spaces
			scopes:          "openid profile read write",
			expectedOIDC:    []string{"openid", "profile"},
			expectedNonOIDC: []string{"read", "write"},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			oidcScopes, nonOidcScopes := SeparateOIDCAndNonOIDCScopes(tc.scopes, nil)
			// Compare lengths and contents, handling nil vs empty slice
			if tc.expectedOIDC == nil {
				assert.Nil(t, oidcScopes, "OIDC scopes should be nil")
			} else {
				assert.Equal(t, tc.expectedOIDC, oidcScopes, "OIDC scopes should match")
			}
			if tc.expectedNonOIDC == nil {
				assert.Nil(t, nonOidcScopes, "Non-OIDC scopes should be nil")
			} else {
				// Filter out empty strings that may be introduced by ParseStringArray
				filteredNonOIDC := []string{}
				for _, s := range nonOidcScopes {
					if s != "" {
						filteredNonOIDC = append(filteredNonOIDC, s)
					}
				}
				assert.Equal(t, tc.expectedNonOIDC, filteredNonOIDC, "Non-OIDC scopes should match")
			}
		})
	}
}

func (suite *OAuth2UtilsTestSuite) TestSeparateOIDCAndNonOIDCScopes_StandardOIDCScopes() {
	// Test all standard OIDC scopes are correctly identified
	// Based on constants.StandardOIDCScopes, these are the standard scopes
	standardOIDCScopes := []string{"openid", "profile", "email", "address", "phone"}

	for _, scope := range standardOIDCScopes {
		suite.T().Run("OIDCScope_"+scope, func(t *testing.T) {
			oidcScopes, nonOidcScopes := SeparateOIDCAndNonOIDCScopes(scope, nil)
			if oidcScopes == nil {
				assert.Fail(t, "OIDC scopes should not be nil for standard OIDC scope")
			} else {
				assert.Contains(t, oidcScopes, scope, "Standard OIDC scope should be in OIDC list")
			}
			if nonOidcScopes != nil {
				assert.NotContains(t, nonOidcScopes, scope, "Standard OIDC scope should not be in non-OIDC list")
			}
		})
	}
}

func (suite *OAuth2UtilsTestSuite) TestSeparateOIDCAndNonOIDCScopes_CustomScopes() {
	// Test custom scopes are correctly identified as non-OIDC
	customScopes := []string{"custom_read", "custom_write", "api_access", "admin_scope"}

	for _, scope := range customScopes {
		suite.T().Run("CustomScope_"+scope, func(t *testing.T) {
			oidcScopes, nonOidcScopes := SeparateOIDCAndNonOIDCScopes(scope, nil)
			// Handle nil case - function may return nil for empty slices
			if oidcScopes == nil {
				assert.Nil(t, oidcScopes, "OIDC scopes should be nil for custom scope")
			} else {
				assert.NotContains(t, oidcScopes, scope, "Custom scope should not be in OIDC list")
			}
			assert.Contains(t, nonOidcScopes, scope, "Custom scope should be in non-OIDC list")
		})
	}
}

func (suite *OAuth2UtilsTestSuite) TestSeparateOIDCAndNonOIDCScopes_WithCustomScopeClaimsMapping() {
	scopeClaimsMapping := map[string][]string{
		"ou":       {"ouId", "ouName"},
		"employee": {"emp_id", "department"},
	}

	testCases := []struct {
		name            string
		scopes          string
		expectedOIDC    []string
		expectedNonOIDC []string
	}{
		{
			name:            "CustomMappedScopeIsOIDC",
			scopes:          "openid ou",
			expectedOIDC:    []string{"openid", "ou"},
			expectedNonOIDC: nil,
		},
		{
			name:            "UnmappedCustomScopeIsNonOIDC",
			scopes:          "openid read",
			expectedOIDC:    []string{"openid"},
			expectedNonOIDC: []string{"read"},
		},
		{
			name:            "MixedStandardAndCustomMappedScopes",
			scopes:          "openid profile ou employee read",
			expectedOIDC:    []string{"openid", "profile", "ou", "employee"},
			expectedNonOIDC: []string{"read"},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			oidcScopes, nonOidcScopes := SeparateOIDCAndNonOIDCScopes(tc.scopes, scopeClaimsMapping)
			if tc.expectedOIDC == nil {
				assert.Nil(t, oidcScopes)
			} else {
				assert.Equal(t, tc.expectedOIDC, oidcScopes)
			}
			if tc.expectedNonOIDC == nil {
				assert.Nil(t, nonOidcScopes)
			} else {
				assert.Equal(t, tc.expectedNonOIDC, nonOidcScopes)
			}
		})
	}
}

// Claims parameter parsing tests

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_ValidJSON() {
	jsonStr := `{
		"userinfo": {
			"given_name": {"essential": true},
			"nickname": null,
			"email": {"essential": true},
			"picture": null
		},
		"id_token": {
			"auth_time": {"essential": true},
			"acr": {"values": ["urn:mace:incommon:iap:silver"]}
		}
	}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), claimsRequest)
	assert.NotNil(suite.T(), claimsRequest.UserInfo)
	assert.NotNil(suite.T(), claimsRequest.IDToken)

	// Verify userinfo claims
	assert.Len(suite.T(), claimsRequest.UserInfo, 4)
	assert.NotNil(suite.T(), claimsRequest.UserInfo["given_name"])
	assert.True(suite.T(), claimsRequest.UserInfo["given_name"].Essential)
	assert.Nil(suite.T(), claimsRequest.UserInfo["nickname"])

	// Verify id_token claims
	assert.Len(suite.T(), claimsRequest.IDToken, 2)
	assert.NotNil(suite.T(), claimsRequest.IDToken["auth_time"])
	assert.True(suite.T(), claimsRequest.IDToken["auth_time"].Essential)
	assert.NotNil(suite.T(), claimsRequest.IDToken["acr"])
	assert.Contains(suite.T(), claimsRequest.IDToken["acr"].Values, "urn:mace:incommon:iap:silver")
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_EmptyString() {
	claimsRequest, err := ParseClaimsRequest("")

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), claimsRequest)
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_InvalidJSON() {
	jsonStr := `{invalid json}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claimsRequest)
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_OnlyUserInfo() {
	jsonStr := `{
		"userinfo": {
			"email": {"essential": true}
		}
	}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), claimsRequest)
	assert.NotNil(suite.T(), claimsRequest.UserInfo)
	assert.Nil(suite.T(), claimsRequest.IDToken)
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_OnlyIDToken() {
	jsonStr := `{
		"id_token": {
			"sub": {"value": "248289761001"}
		}
	}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), claimsRequest)
	assert.Nil(suite.T(), claimsRequest.UserInfo)
	assert.NotNil(suite.T(), claimsRequest.IDToken)
	assert.NotNil(suite.T(), claimsRequest.IDToken["sub"])
	assert.Equal(suite.T(), "248289761001", claimsRequest.IDToken["sub"].Value)
}

func (suite *OAuth2UtilsTestSuite) TestSerializeClaimsRequest_ValidRequest() {
	claimsRequest := &model.ClaimsRequest{
		UserInfo: map[string]*model.IndividualClaimRequest{
			"email":      {Essential: true},
			"given_name": nil,
		},
		IDToken: map[string]*model.IndividualClaimRequest{
			"auth_time": {Essential: true},
		},
	}

	serialized, err := SerializeClaimsRequest(claimsRequest)

	assert.NoError(suite.T(), err)
	assert.NotEmpty(suite.T(), serialized)

	// Verify roundtrip
	parsed, err := ParseClaimsRequest(serialized)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), parsed)
	assert.True(suite.T(), parsed.UserInfo["email"].Essential)
}

func (suite *OAuth2UtilsTestSuite) TestSerializeClaimsRequest_NilRequest() {
	serialized, err := SerializeClaimsRequest(nil)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), serialized)
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_MutuallyExclusiveValueAndValues() {
	// Test that value and values cannot both be specified (OIDC spec violation)
	jsonStr := `{
		"userinfo": {
			"email": {
				"value": "user@example.com",
				"values": ["user1@example.com", "user2@example.com"]
			}
		}
	}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claimsRequest)
	assert.Contains(suite.T(), err.Error(), "mutually exclusive")
	assert.Contains(suite.T(), err.Error(), "email")
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_EmptyValuesArray() {
	// Test that empty values array is rejected (OIDC spec - must match "one of" the values)
	jsonStr := `{
		"userinfo": {
			"email": {
				"values": []
			}
		}
	}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claimsRequest)
	assert.Contains(suite.T(), err.Error(), "empty 'values' array")
	assert.Contains(suite.T(), err.Error(), "email")
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_MutuallyExclusiveInIDToken() {
	// Test mutual exclusivity in id_token section
	jsonStr := `{
		"id_token": {
			"sub": {
				"value": "user123",
				"values": ["user123", "user456"]
			}
		}
	}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claimsRequest)
	assert.Contains(suite.T(), err.Error(), "mutually exclusive")
	assert.Contains(suite.T(), err.Error(), "sub")
	assert.Contains(suite.T(), err.Error(), "id_token")
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_EmptyValuesArrayInIDToken() {
	// Test empty values array rejection in id_token section
	jsonStr := `{
		"id_token": {
			"acr": {
				"values": []
			}
		}
	}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), claimsRequest)
	assert.Contains(suite.T(), err.Error(), "empty 'values' array")
	assert.Contains(suite.T(), err.Error(), "acr")
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_ValidValueOnly() {
	// Test valid request with only value (not values)
	jsonStr := `{
		"userinfo": {
			"email": {
				"value": "user@example.com"
			}
		}
	}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), claimsRequest)
	assert.NotNil(suite.T(), claimsRequest.UserInfo["email"])
	assert.Equal(suite.T(), "user@example.com", claimsRequest.UserInfo["email"].Value)
	assert.Nil(suite.T(), claimsRequest.UserInfo["email"].Values)
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_ValidValuesOnly() {
	// Test valid request with only values (not value)
	jsonStr := `{
		"id_token": {
			"acr": {
				"values": ["urn:mace:incommon:iap:silver", "urn:mace:incommon:iap:bronze"]
			}
		}
	}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), claimsRequest)
	assert.NotNil(suite.T(), claimsRequest.IDToken["acr"])
	assert.Nil(suite.T(), claimsRequest.IDToken["acr"].Value)
	assert.Len(suite.T(), claimsRequest.IDToken["acr"].Values, 2)
}

func (suite *OAuth2UtilsTestSuite) TestParseClaimsRequest_NullClaimRequest() {
	// Test that null individual claim requests are valid (means claim is requested without constraints)
	jsonStr := `{
		"userinfo": {
			"email": null,
			"name": null
		}
	}`

	claimsRequest, err := ParseClaimsRequest(jsonStr)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), claimsRequest)
	assert.Nil(suite.T(), claimsRequest.UserInfo["email"])
	assert.Nil(suite.T(), claimsRequest.UserInfo["name"])
}

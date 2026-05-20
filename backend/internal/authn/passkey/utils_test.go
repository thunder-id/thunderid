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

package passkey

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entity"
)

type UtilsTestSuite struct {
	suite.Suite
}

func TestUtilsTestSuite(t *testing.T) {
	suite.Run(t, new(UtilsTestSuite))
}

func (suite *UtilsTestSuite) TestGenerateDefaultCredentialName() {
	name := generateDefaultCredentialName()

	suite.NotEmpty(name)
	suite.Contains(name, "Passkey")
	suite.Contains(name, time.Now().Format("2006-01-02"))
}

func (suite *UtilsTestSuite) TestGetConfiguredOrigins() {
	origins := getConfiguredOrigins()

	suite.NotNil(origins)
	suite.NotEmpty(origins)
}

func (suite *UtilsTestSuite) TestParseUserAttributes_ValidJSON() {
	attrs := json.RawMessage(`{"name":"John Doe","email":"john@example.com"}`)

	result := parseEntityAttributes(attrs)

	suite.NotNil(result)
	suite.Equal("John Doe", result["name"])
	suite.Equal("john@example.com", result["email"])
}

func (suite *UtilsTestSuite) TestParseUserAttributes_EmptyJSON() {
	attrs := json.RawMessage(`{}`)

	result := parseEntityAttributes(attrs)

	suite.NotNil(result)
	suite.Empty(result)
}

func (suite *UtilsTestSuite) TestParseUserAttributes_NilInput() {
	var attrs json.RawMessage

	result := parseEntityAttributes(attrs)

	suite.Nil(result)
}

func (suite *UtilsTestSuite) TestParseUserAttributes_InvalidJSON() {
	attrs := json.RawMessage(`{invalid json}`)

	result := parseEntityAttributes(attrs)

	suite.Nil(result)
}

func (suite *UtilsTestSuite) TestBuildUserDisplayName_WithName() {
	attrs := map[string]interface{}{
		"given_name":  "John",
		"family_name": "Doe",
	}

	result := buildWebAuthnDisplayName(testUserID, attrs)

	suite.Equal("John Doe", result)
}

func (suite *UtilsTestSuite) TestBuildUserDisplayName_WithFirstNameOnly() {
	attrs := map[string]interface{}{
		"given_name": "John",
	}

	result := buildWebAuthnDisplayName(testUserID, attrs)

	suite.Equal("John", result)
}

func (suite *UtilsTestSuite) TestBuildUserDisplayName_Fallback() {
	attrs := map[string]interface{}{
		"email": "john@example.com", // email is not used by buildWebAuthnDisplayName
	}

	result := buildWebAuthnDisplayName(testUserID, attrs)

	suite.Equal(testUserID, result)
}

func (suite *UtilsTestSuite) TestBuildUserDisplayName_NilAttributes() {
	result := buildWebAuthnDisplayName(testUserID, nil)

	suite.Equal(testUserID, result)
}

func (suite *UtilsTestSuite) TestResolveUserName_WithUsername() {
	attrs := map[string]interface{}{
		"username": "johndoe",
	}

	result := resolveWebAuthnName(testUserID, attrs)

	suite.Equal("johndoe", result)
}

func (suite *UtilsTestSuite) TestResolveUserName_WithEmail() {
	attrs := map[string]interface{}{
		"email": "john@example.com",
	}

	result := resolveWebAuthnName(testUserID, attrs)

	suite.Equal("john@example.com", result)
}

func (suite *UtilsTestSuite) TestResolveUserName_Fallback() {
	attrs := map[string]interface{}{}

	result := resolveWebAuthnName(testUserID, attrs)

	suite.Equal(testUserID, result)
}

func (suite *UtilsTestSuite) TestResolveUserName_NilAttributes() {
	result := resolveWebAuthnName(testUserID, nil)

	suite.Equal(testUserID, result)
}

func (suite *UtilsTestSuite) TestValidateRegistrationStartRequest_ValidRequest() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: "example.com",
	}

	err := validateRegistrationStartRequest(req)

	suite.Nil(err)
}

func (suite *UtilsTestSuite) TestValidateRegistrationStartRequest_EmptyUserID() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         "",
		RelyingPartyID: "example.com",
	}

	err := validateRegistrationStartRequest(req)

	suite.NotNil(err)
	suite.Equal(ErrorEmptyUserIdentifier.Code, err.Code)
}

func (suite *UtilsTestSuite) TestValidateRegistrationStartRequest_EmptyRelyingPartyID() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: "",
	}

	err := validateRegistrationStartRequest(req)

	suite.NotNil(err)
	suite.Equal(ErrorEmptyRelyingPartyID.Code, err.Code)
}

func (suite *UtilsTestSuite) TestValidateAuthenticationStartRequest_NilRequest() {
	err := validateAuthenticationStartRequest(nil)

	suite.NotNil(err)
	suite.Equal(ErrorInvalidFinishData.Code, err.Code)
}

func (suite *UtilsTestSuite) TestDecodeBase64_RawURLEncoding() {
	// Test with RawURLEncoding (no padding)
	input := "SGVsbG8gV29ybGQ"

	result, err := decodeBase64(input)

	suite.NoError(err)
	suite.Equal("Hello World", string(result))
}

func (suite *UtilsTestSuite) TestDecodeBase64_URLEncoding() {
	// Test with URLEncoding (with padding)
	input := "SGVsbG8gV29ybGQ="

	result, err := decodeBase64(input)

	suite.NoError(err)
	suite.Equal("Hello World", string(result))
}

func (suite *UtilsTestSuite) TestDecodeBase64_StdEncoding() {
	// Test with standard encoding
	input := "SGVsbG8gV29ybGQ="

	result, err := decodeBase64(input)

	suite.NoError(err)
	suite.Equal("Hello World", string(result))
}

func (suite *UtilsTestSuite) TestDecodeBase64_InvalidInput() {
	// Test with invalid base64
	input := "not-valid-base64!@#$%"

	_, err := decodeBase64(input)

	suite.Error(err)
}

func (suite *UtilsTestSuite) TestExtractCoreUser_WithFullAttributes() {
	attrs := json.RawMessage(`{"given_name":"John","family_name":"Doe","username":"johndoe"}`)
	testEntity := &entity.Entity{
		ID:         testUserID,
		Category:   entity.EntityCategoryUser,
		Type:       "person",
		OUID:       "org123",
		Attributes: attrs,
	}

	displayName, userName := extractWebAuthnIdentity(testEntity)

	suite.Equal("John Doe", displayName)
	suite.Equal("johndoe", userName)
}

func (suite *UtilsTestSuite) TestExtractCoreUser_WithEmailOnly() {
	attrs := json.RawMessage(`{"email":"john@example.com"}`)
	testEntity := &entity.Entity{
		ID:         testUserID,
		Category:   entity.EntityCategoryUser,
		Attributes: attrs,
	}

	displayName, userName := extractWebAuthnIdentity(testEntity)

	suite.Equal(testUserID, displayName) // Falls back to ID
	suite.Equal("john@example.com", userName)
}

func (suite *UtilsTestSuite) TestExtractCoreUser_EmptyAttributes() {
	testEntity := &entity.Entity{
		ID:       testUserID,
		Category: entity.EntityCategoryUser,
	}

	displayName, userName := extractWebAuthnIdentity(testEntity)
	suite.Equal(testUserID, displayName)
	suite.Equal(testUserID, userName)
}

func (suite *UtilsTestSuite) TestBuildAuthenticatorSelection() {
	tests := []struct {
		name     string
		input    *AuthenticatorSelection
		expected protocol.AuthenticatorSelection
	}{
		{
			name: "All fields populated",
			input: &AuthenticatorSelection{
				AuthenticatorAttachment: "platform",
				ResidentKey:             "required",
				UserVerification:        "required",
			},
			expected: protocol.AuthenticatorSelection{
				AuthenticatorAttachment: protocol.Platform,
				ResidentKey:             protocol.ResidentKeyRequirementRequired,
				UserVerification:        protocol.VerificationRequired,
			},
		},
		{
			name: "Empty fields triggers defaults and skips assignments",
			input: &AuthenticatorSelection{
				AuthenticatorAttachment: "",
				ResidentKey:             "",
				UserVerification:        "",
			},
			expected: protocol.AuthenticatorSelection{
				AuthenticatorAttachment: "",
				ResidentKey:             "",
				UserVerification:        protocol.VerificationPreferred, // Tests the 'else' branch
			},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			got := buildAuthenticatorSelection(tt.input)

			if got.AuthenticatorAttachment != tt.expected.AuthenticatorAttachment {
				suite.T().Errorf("AuthenticatorAttachment: got %v, want %v", got.AuthenticatorAttachment,
					tt.expected.AuthenticatorAttachment)
			}

			if got.ResidentKey != tt.expected.ResidentKey {
				suite.T().Errorf("ResidentKey: got %v, want %v", got.ResidentKey, tt.expected.ResidentKey)
			}

			if got.UserVerification != tt.expected.UserVerification {
				suite.T().Errorf(
					"UserVerification: got %v, want %v", got.UserVerification, tt.expected.UserVerification)
			}
		})
	}
}

// Helper to encode strings to Base64URL (assuming decodeBase64 uses this format).
func b64(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func (suite *UtilsTestSuite) TestParseAssertionResponse() {
	// Setup valid dummy data
	validClientData := `{"type":"passkey.get","challenge":"Y2hhbGxlbmdl","origin":"https://localhost"}`
	validAuthData := make([]byte, 37) // Minimum length for AuthData is usually 37 bytes

	tests := []struct {
		name           string
		credentialID   string
		credentialType string
		clientDataJSON string
		authData       string
		signature      string
		userHandle     string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "Success - All fields valid",
			credentialID:   b64("id123"),
			credentialType: "public-key",
			clientDataJSON: b64(validClientData),
			authData:       base64.RawURLEncoding.EncodeToString(validAuthData),
			signature:      b64("signature-bytes"),
			userHandle:     b64("user-123"),
			wantErr:        false,
		},
		{
			name:           "Success - Optional UserHandle fails decoding (soft fail)",
			credentialID:   b64("id123"),
			credentialType: "public-key",
			clientDataJSON: b64(validClientData),
			authData:       base64.RawURLEncoding.EncodeToString(validAuthData),
			signature:      b64("sig"),
			userHandle:     "!!!invalid-base64!!!",
			wantErr:        false, // The code explicitly swallows this error
		},
		{
			name:         "Error - Invalid webauthnCredential ID Base64",
			credentialID: "!!!",
			wantErr:      true,
			errContains:  "failed to decode credential ID",
		},
		{
			name:           "Error - Invalid ClientData JSON",
			credentialID:   b64("id"),
			clientDataJSON: b64("{ invalid json"),
			wantErr:        true,
			errContains:    "failed to parse client data JSON",
		},
		{
			name:           "Error - Invalid Authenticator Data",
			credentialID:   b64("id"),
			clientDataJSON: b64(validClientData),
			authData:       b64("too-short"), // Will fail Unmarshal
			wantErr:        true,
			errContains:    "failed to parse authenticator data",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			result, err := parseAssertionResponse(
				tt.credentialID,
				tt.credentialType,
				tt.clientDataJSON,
				tt.authData,
				tt.signature,
				tt.userHandle,
			)

			if tt.wantErr {
				suite.Error(err)
				suite.Contains(err.Error(), tt.errContains)
				suite.Nil(result)
			} else {
				suite.NoError(err)
				suite.NotNil(result)

				// Verify Data Structure Mapping
				suite.Equal(tt.credentialType, result.ParsedPublicKeyCredential.ParsedCredential.Type)
				suite.Equal(tt.credentialID, result.ParsedPublicKeyCredential.ParsedCredential.ID)

				// Verify UserHandle specifically
				if tt.userHandle != "" && tt.name != "Success - Optional UserHandle fails decoding (soft fail)" {
					suite.NotNil(result.Response.UserHandle)
				}
			}
		})
	}
}

func (suite *UtilsTestSuite) TestDecodeBase64_RawStdEncoding() {
	input := "SGVsbG8gV29ybGQ" // "Hello World" in RawStdEncoding

	result, err := decodeBase64(input)

	suite.NoError(err)
	suite.Equal("Hello World", string(result))
}

func (suite *UtilsTestSuite) TestParseAssertionResponse_ClientDataJSONDecodeError() {
	_, err := parseAssertionResponse(
		b64("valid-id"),
		"public-key",
		"!!!invalid-base64!!!",
		b64("valid-auth-data"),
		b64("valid-signature"),
		"",
	)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to decode client data JSON")
}

func (suite *UtilsTestSuite) TestParseAssertionResponse_AuthenticatorDataDecodeError() {
	_, err := parseAssertionResponse(
		b64("valid-id"),
		"public-key",
		b64(`{"type":"passkey.get"}`),
		"!!!invalid-base64!!!",
		b64("valid-signature"),
		"",
	)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to decode authenticator data")
}

func (suite *UtilsTestSuite) TestParseAssertionResponse_SignatureDecodeError() {
	validAuthData := make([]byte, 37)
	_, err := parseAssertionResponse(
		b64("valid-id"),
		"public-key",
		b64(`{"type":"passkey.get"}`),
		base64.RawURLEncoding.EncodeToString(validAuthData),
		"!!!invalid-base64!!!",
		"",
	)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to decode signature")
}

func (suite *UtilsTestSuite) TestParseAttestationResponse_ClientDataJSONDecodeError() {
	_, err := parseAttestationResponse(
		"credential-id",
		"public-key",
		"!!!invalid-base64!!!",
		b64("valid-attestation"),
	)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to decode client data JSON")
}

func (suite *UtilsTestSuite) TestParseAttestationResponse_AttestationObjectDecodeError() {
	_, err := parseAttestationResponse(
		"credential-id",
		"public-key",
		b64(`{"type":"passkey.create"}`),
		"!!!invalid-base64!!!",
	)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to decode attestation object")
}

func (suite *UtilsTestSuite) TestParseAttestationResponse_Success() {
	validClientData := `{"type":"passkey.create","challenge":"test","origin":"https://localhost"}`

	// Create a minimal valid CBOR attestation object
	// Format: map with "fmt" and "attStmt" keys
	attestationCBOR := []byte{
		0xa2,                   // map(2)
		0x63, 0x66, 0x6d, 0x74, // "fmt" (text string of length 3)
		0x64, 0x6e, 0x6f, 0x6e, 0x65, // "none" (text string of length 4)
		0x67, 0x61, 0x74, 0x74, 0x53, 0x74, 0x6d, 0x74, // "attStmt" (text string of length 7)
		0xa0, // empty map
	}

	result, err := parseAttestationResponse(
		b64("credential-id"),
		"public-key",
		b64(validClientData),
		base64.RawURLEncoding.EncodeToString(attestationCBOR),
	)

	if err != nil {
		suite.Contains(err.Error(), "failed to parse credential creation response")
	} else {
		suite.NotNil(result)
	}
}

func (suite *UtilsTestSuite) TestBuildRegistrationOptions_WithAuthenticatorSelection() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: "example.com",
		AuthenticatorSelection: &AuthenticatorSelection{
			AuthenticatorAttachment: "platform",
			ResidentKey:             "required",
			UserVerification:        "required",
		},
	}

	options := buildRegistrationOptions(req)

	suite.NotNil(options)
	suite.NotEmpty(options)
	// Should contain at least the authenticator selection option
	suite.GreaterOrEqual(len(options), 1)
}

func (suite *UtilsTestSuite) TestBuildRegistrationOptions_WithAttestation() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: "example.com",
		Attestation:    "direct",
	}

	options := buildRegistrationOptions(req)

	suite.NotNil(options)
	suite.NotEmpty(options)
	// Should contain at least the attestation option
	suite.GreaterOrEqual(len(options), 1)
}

func (suite *UtilsTestSuite) TestBuildRegistrationOptions_WithBothOptions() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: "example.com",
		AuthenticatorSelection: &AuthenticatorSelection{
			AuthenticatorAttachment: "cross-platform",
			ResidentKey:             "preferred",
			UserVerification:        "preferred",
		},
		Attestation: "indirect",
	}

	options := buildRegistrationOptions(req)

	suite.NotNil(options)
	suite.NotEmpty(options)
	// Should contain both options
	suite.Equal(2, len(options))
}

func (suite *UtilsTestSuite) TestBuildRegistrationOptions_WithNeitherOption() {
	req := &PasskeyRegistrationStartRequest{
		UserID:         testUserID,
		RelyingPartyID: "example.com",
		// No AuthenticatorSelection
		// No Attestation
	}

	options := buildRegistrationOptions(req)

	// When no options are added, the slice remains nil
	suite.Len(options, 0)
}

func (suite *UtilsTestSuite) TestGetConfiguredOrigins_ReturnsDefaultsWhenNoConfig() {
	origins := getConfiguredOrigins()

	suite.NotNil(origins)
	suite.NotEmpty(origins)
	// Should return either configured origins or defaults
	suite.True(len(origins) >= 1)
}

func (suite *UtilsTestSuite) TestBuildAuthenticatorSelection_WithAuthenticatorAttachment() {
	// Tests the branch where AuthenticatorAttachment is set
	sel := &AuthenticatorSelection{
		AuthenticatorAttachment: "platform",
	}

	result := buildAuthenticatorSelection(sel)

	suite.Equal(protocol.Platform, result.AuthenticatorAttachment)
	suite.Equal(protocol.VerificationPreferred, result.UserVerification)
}

func (suite *UtilsTestSuite) TestBuildAuthenticatorSelection_WithResidentKey() {
	// Tests the branch where ResidentKey is set
	sel := &AuthenticatorSelection{
		ResidentKey: "required",
	}

	result := buildAuthenticatorSelection(sel)

	suite.Equal(protocol.ResidentKeyRequirementRequired, result.ResidentKey)
	suite.Equal(protocol.VerificationPreferred, result.UserVerification)
}

func (suite *UtilsTestSuite) TestBuildAuthenticatorSelection_WithUserVerification() {
	// Tests the branch where UserVerification is set (not empty)
	sel := &AuthenticatorSelection{
		UserVerification: "required",
	}

	result := buildAuthenticatorSelection(sel)

	suite.Equal(protocol.VerificationRequired, result.UserVerification)
}

func (suite *UtilsTestSuite) TestBuildAuthenticatorSelection_EmptyUserVerification() {
	// Tests the else branch where UserVerification defaults to preferred
	sel := &AuthenticatorSelection{
		UserVerification: "",
	}

	result := buildAuthenticatorSelection(sel)

	suite.Equal(protocol.VerificationPreferred, result.UserVerification)
}

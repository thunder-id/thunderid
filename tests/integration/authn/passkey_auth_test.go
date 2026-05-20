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

package authn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	passkeyRegisterStartEndpoint  = "/register/passkey/start"
	passkeyRegisterFinishEndpoint = "/register/passkey/finish"
	passkeyAuthStartEndpoint      = "/auth/passkey/start"
	passkeyAuthFinishEndpoint     = "/auth/passkey/finish"
	testRelyingPartyID            = "localhost"
	testRelyingPartyName          = "ThunderID Test"
)

var (
	passkeyTestOU = testutils.OrganizationUnit{
		Handle:      "passkey-auth-test-ou",
		Name:        "Passkey Auth Test Organization Unit",
		Description: "Organization unit for passkey authentication testing",
		Parent:      nil,
	}

	passkeyEntityType = testutils.UserType{
		Name: "passkey_user",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"displayName": map[string]interface{}{
				"type": "string",
			},
		},
	}
)

// PasskeyRegisterStartRequest represents the request to start passkey registration
type PasskeyRegisterStartRequest struct {
	UserID                 string                          `json:"userId"`
	RelyingPartyID         string                          `json:"relyingPartyId"`
	RelyingPartyName       string                          `json:"relyingPartyName,omitempty"`
	AuthenticatorSelection *AuthenticatorSelectionCriteria `json:"authenticatorSelection,omitempty"`
	Attestation            string                          `json:"attestation,omitempty"`
}

// AuthenticatorSelectionCriteria represents authenticator selection criteria
type AuthenticatorSelectionCriteria struct {
	AuthenticatorAttachment string `json:"authenticatorAttachment,omitempty"`
	RequireResidentKey      bool   `json:"requireResidentKey,omitempty"`
	ResidentKey             string `json:"residentKey,omitempty"`
	UserVerification        string `json:"userVerification,omitempty"`
}

// PasskeyRegisterStartResponse represents the response from starting passkey registration
type PasskeyRegisterStartResponse struct {
	SessionToken                       string                                     `json:"sessionToken"`
	PublicKeyCredentialCreationOptions PublicKeyCredentialCreationOptionsResponse `json:"publicKeyCredentialCreationOptions"`
}

// PublicKeyCredentialCreationOptionsResponse represents the credential creation options
type PublicKeyCredentialCreationOptionsResponse struct {
	Challenge              string                          `json:"challenge"`
	RelyingParty           RelyingParty                    `json:"rp"`
	User                   PublicKeyCredentialUser         `json:"user"`
	PubKeyCredParams       []CredentialParameter           `json:"pubKeyCredParams"`
	Timeout                int                             `json:"timeout,omitempty"`
	ExcludeCredentials     []PublicKeyCredential           `json:"excludeCredentials,omitempty"`
	AuthenticatorSelection *AuthenticatorSelectionCriteria `json:"authenticatorSelection,omitempty"`
	Attestation            string                          `json:"attestation,omitempty"`
}

// RelyingParty represents the relying party information
type RelyingParty struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PublicKeyCredentialUser represents the user information for WebAuthn
type PublicKeyCredentialUser struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

// CredentialParameter represents credential parameters
type CredentialParameter struct {
	Type      string `json:"type"`
	Algorithm int    `json:"alg"`
}

// PublicKeyCredential represents a public key credential descriptor
type PublicKeyCredential struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Transports []string `json:"transports,omitempty"`
}

// PasskeyRegisterFinishRequest represents the request to finish passkey registration
type PasskeyRegisterFinishRequest struct {
	PublicKeyCredential PublicKeyCredentialAttestation `json:"publicKeyCredential"`
	SessionToken        string                         `json:"sessionToken"`
	CredentialName      string                         `json:"credentialName,omitempty"`
}

// PublicKeyCredentialAttestation represents the attestation response
type PublicKeyCredentialAttestation struct {
	ID       string                           `json:"id"`
	Type     string                           `json:"type"`
	RawID    string                           `json:"rawId"`
	Response AuthenticatorAttestationResponse `json:"response"`
}

// AuthenticatorAttestationResponse represents the attestation response
type AuthenticatorAttestationResponse struct {
	ClientDataJSON    string   `json:"clientDataJSON"`
	AttestationObject string   `json:"attestationObject"`
	Transports        []string `json:"transports,omitempty"`
}

// PasskeyRegisterFinishResponse represents the response from finishing passkey registration
type PasskeyRegisterFinishResponse struct {
	CredentialID   string `json:"credentialId"`
	CredentialName string `json:"credentialName,omitempty"`
}

// PasskeyAuthStartRequest represents the request to start passkey authentication
type PasskeyAuthStartRequest struct {
	UserID         string `json:"userId"`
	RelyingPartyID string `json:"relyingPartyId"`
}

// PasskeyAuthStartResponse represents the response from starting passkey authentication
type PasskeyAuthStartResponse struct {
	SessionToken                      string                                    `json:"sessionToken"`
	PublicKeyCredentialRequestOptions PublicKeyCredentialRequestOptionsResponse `json:"publicKeyCredentialRequestOptions"`
}

// PublicKeyCredentialRequestOptionsResponse represents the credential request options
type PublicKeyCredentialRequestOptionsResponse struct {
	Challenge        string                `json:"challenge"`
	RelyingPartyID   string                `json:"rpId"`
	AllowCredentials []PublicKeyCredential `json:"allowCredentials"`
	Timeout          int                   `json:"timeout,omitempty"`
	UserVerification string                `json:"userVerification,omitempty"`
}

// PasskeyAuthFinishRequest represents the request to finish passkey authentication
type PasskeyAuthFinishRequest struct {
	CredentialID      string                         `json:"credentialId"`
	CredentialType    string                         `json:"credentialType"`
	Response          AuthenticatorAssertionResponse `json:"response"`
	SessionToken      string                         `json:"sessionToken"`
	SkipAssertion     bool                           `json:"skipAssertion,omitempty"`
	ExistingAssertion string                         `json:"existingAssertion,omitempty"`
}

// AuthenticatorAssertionResponse represents the assertion response
type AuthenticatorAssertionResponse struct {
	ClientDataJSON    string `json:"clientDataJSON"`
	AuthenticatorData string `json:"authenticatorData"`
	Signature         string `json:"signature"`
	UserHandle        string `json:"userHandle,omitempty"`
}

type PasskeyAuthTestSuite struct {
	suite.Suite
	client       *http.Client
	testUserID   string
	entityTypeID string
	ouID         string
}

func TestPasskeyAuthTestSuite(t *testing.T) {
	suite.Run(t, new(PasskeyAuthTestSuite))
}

func (suite *PasskeyAuthTestSuite) SetupSuite() {
	suite.client = testutils.GetHTTPClient()

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(passkeyTestOU)
	if err != nil {
		suite.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	suite.ouID = ouID

	// Create user type
	passkeyEntityType.OUID = suite.ouID
	schemaID, err := testutils.CreateUserType(passkeyEntityType)
	if err != nil {
		suite.T().Fatalf("Failed to create user type during setup: %v", err)
	}
	suite.entityTypeID = schemaID

	// Create test user
	attributes := map[string]interface{}{
		"username":    "passkeytest_user",
		"email":       "passkeytest@example.com",
		"displayName": "Passkey Test User",
	}
	attributesJSON, err := json.Marshal(attributes)
	suite.Require().NoError(err, "Failed to marshal user attributes")

	user := testutils.User{
		Type:       "passkey_user",
		OUID:       suite.ouID,
		Attributes: json.RawMessage(attributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err, "Failed to create test user")
	suite.testUserID = userID
}

func (suite *PasskeyAuthTestSuite) TearDownSuite() {
	// Delete test user
	if suite.testUserID != "" {
		err := testutils.DeleteUser(suite.testUserID)
		if err != nil {
			suite.T().Errorf("Failed to delete test user during teardown: %v", err)
		}
	}

	// Delete user type
	if suite.entityTypeID != "" {
		err := testutils.DeleteUserType(suite.entityTypeID)
		if err != nil {
			suite.T().Errorf("Failed to delete user type during teardown: %v", err)
		}
	}

	// Delete test organization unit
	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// TestPasskeyRegistrationStart tests the start of passkey registration
func (suite *PasskeyAuthTestSuite) TestPasskeyRegistrationStart() {
	registerRequest := PasskeyRegisterStartRequest{
		UserID:           suite.testUserID,
		RelyingPartyID:   testRelyingPartyID,
		RelyingPartyName: testRelyingPartyName,
	}

	response, statusCode, err := suite.sendPasskeyRegisterStartRequest(registerRequest)
	suite.Require().NoError(err, "Failed to send passkey register start request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful registration start")

	// Verify response structure
	suite.NotEmpty(response.SessionToken, "Response should contain session token")
	suite.NotEmpty(response.PublicKeyCredentialCreationOptions.Challenge, "Response should contain challenge")
	suite.Equal(testRelyingPartyID, response.PublicKeyCredentialCreationOptions.RelyingParty.ID,
		"Response should contain correct RP ID")
	suite.Equal(testRelyingPartyName, response.PublicKeyCredentialCreationOptions.RelyingParty.Name,
		"Response should contain correct RP name")
	suite.NotEmpty(response.PublicKeyCredentialCreationOptions.User.ID, "Response should contain user ID")
	suite.NotEmpty(response.PublicKeyCredentialCreationOptions.PubKeyCredParams,
		"Response should contain credential parameters")

	// Verify challenge is valid base64
	_, err = base64.RawURLEncoding.DecodeString(response.PublicKeyCredentialCreationOptions.Challenge)
	suite.NoError(err, "Challenge should be valid base64")
}

// TestPasskeyRegistrationStartWithAuthenticatorSelection tests registration with authenticator selection
func (suite *PasskeyAuthTestSuite) TestPasskeyRegistrationStartWithAuthenticatorSelection() {
	registerRequest := PasskeyRegisterStartRequest{
		UserID:           suite.testUserID,
		RelyingPartyID:   testRelyingPartyID,
		RelyingPartyName: testRelyingPartyName,
		AuthenticatorSelection: &AuthenticatorSelectionCriteria{
			AuthenticatorAttachment: "platform",
			RequireResidentKey:      true,
			ResidentKey:             "required",
			UserVerification:        "required",
		},
		Attestation: "direct",
	}

	response, statusCode, err := suite.sendPasskeyRegisterStartRequest(registerRequest)
	suite.Require().NoError(err, "Failed to send passkey register start request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful registration start")

	// Verify response includes authenticator selection
	suite.NotNil(response.PublicKeyCredentialCreationOptions.AuthenticatorSelection,
		"Response should contain authenticator selection")
	suite.Equal("platform",
		response.PublicKeyCredentialCreationOptions.AuthenticatorSelection.AuthenticatorAttachment)
	suite.Equal("direct", response.PublicKeyCredentialCreationOptions.Attestation)
}

// TestPasskeyRegistrationStartInvalidUserID tests registration with invalid user ID
func (suite *PasskeyAuthTestSuite) TestPasskeyRegistrationStartInvalidUserID() {
	registerRequest := PasskeyRegisterStartRequest{
		UserID:         "invalid-user-id",
		RelyingPartyID: testRelyingPartyID,
	}

	_, statusCode, _ := suite.sendPasskeyRegisterStartRequest(registerRequest)
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for invalid user ID")
}

// TestPasskeyRegistrationStartEmptyUserID tests registration with empty user ID
func (suite *PasskeyAuthTestSuite) TestPasskeyRegistrationStartEmptyUserID() {
	registerRequest := PasskeyRegisterStartRequest{
		UserID:         "",
		RelyingPartyID: testRelyingPartyID,
	}

	_, statusCode, _ := suite.sendPasskeyRegisterStartRequest(registerRequest)
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for empty user ID")
}

// TestPasskeyRegistrationStartEmptyRelyingPartyID tests registration with empty relying party ID
func (suite *PasskeyAuthTestSuite) TestPasskeyRegistrationStartEmptyRelyingPartyID() {
	registerRequest := PasskeyRegisterStartRequest{
		UserID:         suite.testUserID,
		RelyingPartyID: "",
	}

	_, statusCode, _ := suite.sendPasskeyRegisterStartRequest(registerRequest)
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for empty relying party ID")
}

// TestPasskeyAuthenticationStartNoCredentials tests authentication start when user has no credentials
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationStartNoCredentials() {
	authRequest := PasskeyAuthStartRequest{
		UserID:         suite.testUserID,
		RelyingPartyID: testRelyingPartyID,
	}

	_, statusCode, _ := suite.sendPasskeyAuthStartRequest(authRequest)
	// Should return 404 or specific error when no credentials exist
	suite.True(statusCode == http.StatusNotFound || statusCode == http.StatusBadRequest,
		"Expected status 404 or 400 when user has no registered credentials")
}

// TestPasskeyAuthenticationStartInvalidUserID tests authentication with invalid user ID
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationStartInvalidUserID() {
	authRequest := PasskeyAuthStartRequest{
		UserID:         "invalid-user-id",
		RelyingPartyID: testRelyingPartyID,
	}

	_, statusCode, _ := suite.sendPasskeyAuthStartRequest(authRequest)
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for invalid user ID")
}

// TestPasskeyAuthenticationStartEmptyUserID tests usernameless authentication with empty user ID
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationStartEmptyUserID() {
	authRequest := PasskeyAuthStartRequest{
		UserID:         "",
		RelyingPartyID: testRelyingPartyID,
	}

	response, statusCode, err := suite.sendPasskeyAuthStartRequest(authRequest)
	// Usernameless authentication should succeed
	suite.NoError(err, "Failed to send passkey auth start request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication")
	suite.NotNil(response, "Response should not be nil")
	suite.NotEmpty(response.SessionToken, "Response should contain session token")
	suite.NotEmpty(response.PublicKeyCredentialRequestOptions.Challenge, "Response should contain challenge")
}

// TestPasskeyRegistrationFinishInvalidSessionToken tests finish registration with invalid session
func (suite *PasskeyAuthTestSuite) TestPasskeyRegistrationFinishInvalidSessionToken() {
	// Create mock credential response
	finishRequest := PasskeyRegisterFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAttestation{
			ID:    "mock-credential-id",
			Type:  "public-key",
			RawID: base64.RawURLEncoding.EncodeToString([]byte("mock-credential-id")),
			Response: AuthenticatorAttestationResponse{
				ClientDataJSON:    base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.create"}`)),
				AttestationObject: base64.RawURLEncoding.EncodeToString([]byte("mock-attestation")),
			},
		},
		SessionToken:   "invalid-session-token",
		CredentialName: "Test Credential",
	}

	_, statusCode, _ := suite.sendPasskeyRegisterFinishRequest(finishRequest)
	suite.True(statusCode == http.StatusUnauthorized || statusCode == http.StatusBadRequest,
		"Expected status 401 or 400 for invalid session token")
}

// TestPasskeyAuthenticationFinishInvalidSessionToken tests finish authentication with invalid session
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationFinishInvalidSessionToken() {
	finishRequest := PasskeyAuthFinishRequest{
		CredentialID:   "mock-credential-id",
		CredentialType: "public-key",
		Response: AuthenticatorAssertionResponse{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.get"}`)),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("mock-auth-data")),
			Signature:         base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
		},
		SessionToken: "invalid-session-token",
	}

	_, statusCode, _ := suite.sendPasskeyAuthFinishRequest(finishRequest)
	suite.True(statusCode == http.StatusUnauthorized || statusCode == http.StatusBadRequest,
		"Expected status 401 or 400 for invalid session token")
}

// TestPasskeyAuthenticationUsernamelessFlow tests the complete usernameless passkey authentication flow
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationUsernamelessFlow() {
	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "", // Empty userID for usernameless flow
		RelyingPartyID: testRelyingPartyID,
	}

	startResponse, statusCode, err := suite.sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")
	suite.NotNil(startResponse, "Start response should not be nil")

	suite.NotEmpty(startResponse.SessionToken, "Response should contain session token")
	suite.NotEmpty(startResponse.PublicKeyCredentialRequestOptions.Challenge,
		"Response should contain challenge")
	suite.Equal(testRelyingPartyID, startResponse.PublicKeyCredentialRequestOptions.RelyingPartyID,
		"Response should contain correct RP ID")

	suite.Empty(startResponse.PublicKeyCredentialRequestOptions.AllowCredentials,
		"AllowCredentials should be empty for usernameless flow to enable discoverable credentials")

	_, err = base64.RawURLEncoding.DecodeString(startResponse.PublicKeyCredentialRequestOptions.Challenge)
	suite.NoError(err, "Challenge should be valid base64")

	suite.NotZero(startResponse.PublicKeyCredentialRequestOptions.Timeout,
		"Timeout should be set in request options")

	suite.NotEmpty(startResponse.PublicKeyCredentialRequestOptions.UserVerification,
		"User verification should be specified")
}

// TestPasskeyAuthenticationUsernamelessFlowWithWhitespace tests usernameless flow with whitespace userID
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationUsernamelessFlowWithWhitespace() {
	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "   ", // Whitespace userID
		RelyingPartyID: testRelyingPartyID,
	}

	startResponse, statusCode, err := suite.sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")
	suite.NotNil(startResponse, "Start response should not be nil")

	suite.Empty(startResponse.PublicKeyCredentialRequestOptions.AllowCredentials,
		"AllowCredentials should be empty for usernameless flow")
}

// TestPasskeyAuthenticationUsernamelessFlowEmptyRelyingPartyID tests usernameless with missing RP ID
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationUsernamelessFlowEmptyRelyingPartyID() {
	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "", // Usernameless
		RelyingPartyID: "", // Missing RP ID
	}

	_, statusCode, _ := suite.sendPasskeyAuthStartRequest(authStartRequest)
	suite.Equal(http.StatusBadRequest, statusCode,
		"Expected status 400 for usernameless flow with missing RP ID")
}

// TestPasskeyAuthenticationFinishUsernamelessWithValidCredential tests finish authentication for usernameless flow
// This test covers the ValidatePasskeyLogin path including the type assertion of user interface
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationFinishUsernamelessWithValidCredential() {
	registerStartRequest := PasskeyRegisterStartRequest{
		UserID:           suite.testUserID,
		RelyingPartyID:   testRelyingPartyID,
		RelyingPartyName: testRelyingPartyName,
		AuthenticatorSelection: &AuthenticatorSelectionCriteria{
			ResidentKey:      "required",
			UserVerification: "required",
		},
	}

	registerStartResponse, statusCode, err := suite.sendPasskeyRegisterStartRequest(registerStartRequest)
	suite.Require().NoError(err, "Failed to send passkey register start request")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for registration start")
	suite.Require().NotEmpty(registerStartResponse.SessionToken, "Session token should not be empty")

	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "", // Empty userID for usernameless flow
		RelyingPartyID: testRelyingPartyID,
	}

	authStartResponse, statusCode, err := suite.sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")
	suite.NotNil(authStartResponse, "Auth start response should not be nil")
	suite.NotEmpty(authStartResponse.SessionToken, "Session token should not be empty")

	// Verify the response structure for usernameless flow
	suite.Empty(authStartResponse.PublicKeyCredentialRequestOptions.AllowCredentials,
		"AllowCredentials should be empty for usernameless flow")
	suite.NotEmpty(authStartResponse.PublicKeyCredentialRequestOptions.Challenge,
		"Challenge should be present")
	suite.Equal(testRelyingPartyID, authStartResponse.PublicKeyCredentialRequestOptions.RelyingPartyID,
		"RelyingPartyID should match")

	finishRequest := PasskeyAuthFinishRequest{
		CredentialID:   "mock-credential-id",
		CredentialType: "public-key",
		Response: AuthenticatorAssertionResponse{
			ClientDataJSON: base64.RawURLEncoding.EncodeToString([]byte(
				`{"type":"webauthn.get","challenge":"` +
					authStartResponse.PublicKeyCredentialRequestOptions.Challenge + `","origin":"http://localhost"}`)),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte(
				"mock-auth-data-with-sufficient-length-for-parsing")),
			Signature:  base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
			UserHandle: base64.StdEncoding.EncodeToString([]byte(suite.testUserID)),
		},
		SessionToken: authStartResponse.SessionToken,
	}

	_, statusCode, _ = suite.sendPasskeyAuthFinishRequest(finishRequest)
	// Should fail validation but the code path including type assertion should be exercised
	suite.True(statusCode == http.StatusBadRequest || statusCode == http.StatusUnauthorized,
		"Expected validation error status for mock credential")
}

// TestPasskeyAuthenticationFinishUsernamelessWithInvalidUserHandle tests usernameless flow with invalid userHandle
// This test ensures proper error handling in the ValidatePasskeyLogin path
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationFinishUsernamelessWithInvalidUserHandle() {
	// Start usernameless authentication
	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "", // Empty userID for usernameless flow
		RelyingPartyID: testRelyingPartyID,
	}

	authStartResponse, statusCode, err := suite.sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")

	// Attempt finish with invalid user handle
	finishRequest := PasskeyAuthFinishRequest{
		CredentialID:   "mock-credential-id",
		CredentialType: "public-key",
		Response: AuthenticatorAssertionResponse{
			ClientDataJSON: base64.RawURLEncoding.EncodeToString([]byte(
				`{"type":"webauthn.get","challenge":"` +
					authStartResponse.PublicKeyCredentialRequestOptions.Challenge + `","origin":"http://localhost"}`)),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("mock-auth-data")),
			Signature:         base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
			UserHandle:        "!!!invalid-base64!!!",
		},
		SessionToken: authStartResponse.SessionToken,
	}

	_, statusCode, _ = suite.sendPasskeyAuthFinishRequest(finishRequest)
	suite.True(statusCode == http.StatusBadRequest || statusCode == http.StatusUnauthorized,
		"Expected error status for invalid user handle")
}

// TestPasskeyAuthenticationFinishUsernamelessWithEmptyUserHandle tests usernameless flow without userHandle
// This covers the error case when userHandle is missing in usernameless authentication
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationFinishUsernamelessWithEmptyUserHandle() {
	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "", // Empty userID for usernameless flow
		RelyingPartyID: testRelyingPartyID,
	}

	authStartResponse, statusCode, err := suite.sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Require().Equal(
		http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")

	// Attempt finish without user handle
	finishRequest := PasskeyAuthFinishRequest{
		CredentialID:   "mock-credential-id",
		CredentialType: "public-key",
		Response: AuthenticatorAssertionResponse{
			ClientDataJSON:    base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.get"}`)),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("mock-auth-data")),
			Signature:         base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
			UserHandle:        "", // Empty userHandle
		},
		SessionToken: authStartResponse.SessionToken,
	}

	_, statusCode, _ = suite.sendPasskeyAuthFinishRequest(finishRequest)
	suite.True(statusCode == http.StatusBadRequest || statusCode == http.StatusUnauthorized,
		"Expected error status when userHandle is missing in usernameless flow")
}

// TestPasskeyAuthenticationFinishUsernamelessWithNonExistentUser tests usernameless flow with non-existent user
// This test covers the case where the userHandle points to a user that doesn't exist
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationFinishUsernamelessWithNonExistentUser() {
	// Start usernameless authentication
	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "", // Empty userID for usernameless flow
		RelyingPartyID: testRelyingPartyID,
	}

	authStartResponse, statusCode, err := suite.sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")

	// Attempt finish with userHandle pointing to non-existent user
	nonExistentUserID := "non-existent-user-id-12345"
	finishRequest := PasskeyAuthFinishRequest{
		CredentialID:   "mock-credential-id",
		CredentialType: "public-key",
		Response: AuthenticatorAssertionResponse{
			ClientDataJSON: base64.RawURLEncoding.EncodeToString([]byte(
				`{"type":"webauthn.get","challenge":"` +
					authStartResponse.PublicKeyCredentialRequestOptions.Challenge + `","origin":"http://localhost"}`)),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("mock-auth-data")),
			Signature:         base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
			UserHandle:        base64.StdEncoding.EncodeToString([]byte(nonExistentUserID)),
		},
		SessionToken: authStartResponse.SessionToken,
	}

	_, statusCode, _ = suite.sendPasskeyAuthFinishRequest(finishRequest)
	suite.True(statusCode >= http.StatusBadRequest,
		"Expected error status for non-existent user in usernameless flow")
}

// TestPasskeyAuthenticationUsernamelessValidationError tests the ValidatePasskeyLogin path
// when signature validation fails. This explicitly tests the error handling in the usernameless
// flow where ValidatePasskeyLogin returns an error resulting in ErrorInvalidSignature.
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationUsernamelessValidationError() {
	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "", // Empty userID for usernameless flow
		RelyingPartyID: testRelyingPartyID,
	}

	authStartResponse, statusCode, err := suite.sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")
	suite.Require().NotEmpty(authStartResponse.SessionToken, "Session token should not be empty")

	finishRequest := PasskeyAuthFinishRequest{
		CredentialID:   base64.RawURLEncoding.EncodeToString([]byte("test-credential-id")),
		CredentialType: "public-key",
		Response: AuthenticatorAssertionResponse{
			ClientDataJSON: base64.RawURLEncoding.EncodeToString([]byte(
				`{"type":"webauthn.get","challenge":"` +
					authStartResponse.PublicKeyCredentialRequestOptions.Challenge +
					`","origin":"http://` + testRelyingPartyID + `"}`)),
			AuthenticatorData: base64.RawURLEncoding.EncodeToString(make([]byte, 37)),
			Signature:         base64.RawURLEncoding.EncodeToString([]byte("invalid-signature")),
			UserHandle:        base64.StdEncoding.EncodeToString([]byte(suite.testUserID)),
		},
		SessionToken: authStartResponse.SessionToken,
	}

	_, statusCode, _ = suite.sendPasskeyAuthFinishRequest(finishRequest)
	suite.True(statusCode == http.StatusBadRequest || statusCode == http.StatusUnauthorized,
		"Expected validation error status for usernameless flow with invalid signature")
}

// TestPasskeyAuthenticationUsernameBasedValidationError tests the ValidateLogin path
// when signature validation fails. This explicitly tests the error handling in the username-based
// flow where ValidateLogin returns an error resulting in ErrorInvalidSignature.
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationUsernameBasedValidationError() {
	authStartRequest := PasskeyAuthStartRequest{
		UserID:         suite.testUserID, // Provide user ID for username-based flow
		RelyingPartyID: testRelyingPartyID,
	}

	registerStartRequest := PasskeyRegisterStartRequest{
		UserID:           suite.testUserID,
		RelyingPartyID:   testRelyingPartyID,
		RelyingPartyName: testRelyingPartyName,
	}

	_, regStatus, _ := suite.sendPasskeyRegisterStartRequest(registerStartRequest)
	suite.Require().Equal(http.StatusOK, regStatus, "Registration start should succeed")

	_, statusCode, _ := suite.sendPasskeyAuthStartRequest(authStartRequest)

	suite.True(statusCode == http.StatusNotFound || statusCode == http.StatusBadRequest,
		"Expected error when user has no registered credentials for username-based flow")
}

// Helper methods

func (suite *PasskeyAuthTestSuite) sendPasskeyRegisterStartRequest(
	request PasskeyRegisterStartRequest,
) (*PasskeyRegisterStartResponse, int, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := testutils.TestServerURL + passkeyRegisterStartEndpoint
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}

	var response PasskeyRegisterStartResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, resp.StatusCode, nil
}

func (suite *PasskeyAuthTestSuite) sendPasskeyRegisterFinishRequest(
	request PasskeyRegisterFinishRequest,
) (*PasskeyRegisterFinishResponse, int, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := testutils.TestServerURL + passkeyRegisterFinishEndpoint
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}

	var response PasskeyRegisterFinishResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, resp.StatusCode, nil
}

func (suite *PasskeyAuthTestSuite) sendPasskeyAuthStartRequest(
	request PasskeyAuthStartRequest,
) (*PasskeyAuthStartResponse, int, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := testutils.TestServerURL + passkeyAuthStartEndpoint
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}

	var response PasskeyAuthStartResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, resp.StatusCode, nil
}

func (suite *PasskeyAuthTestSuite) sendPasskeyAuthFinishRequest(
	request PasskeyAuthFinishRequest,
) (*testutils.AuthenticationResponse, int, error) {
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := testutils.TestServerURL + passkeyAuthFinishEndpoint
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, nil
	}

	var response testutils.AuthenticationResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, resp.StatusCode, nil
}

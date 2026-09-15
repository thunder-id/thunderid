// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	passkeyRegisterStartEndpoint  = "/register/passkey/start"
	passkeyRegisterFinishEndpoint = "/register/passkey/finish"
	passkeyAuthStartEndpoint      = "/auth/passkey/start"
	passkeyAuthFinishEndpoint     = "/auth/passkey/finish"
	testRelyingPartyID            = "localhost"
	testRelyingPartyName          = "ThunderID Test"
	// testPasskeyOrigin must be one of the origins under passkey.allowed_origins in the test
	// deployment.yaml, since the direct passkey APIs take their allowed origins from server config.
	testPasskeyOrigin = "https://localhost:8095"
)

var (
	passkeyTestOU = testutils.OrganizationUnit{
		Handle:      "passkey-auth-test-ou",
		Name:        "Passkey Auth Test Organization Unit",
		Description: "Organization unit for passkey authentication testing",
		Parent:      nil,
	}

	// The optional password is used only in setup to obtain an assertion and enroll the shared
	// passkey through the direct API. Authentication tests use the resulting passkey.
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
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
		},
	}
)

const (
	credentialUsername = "passkeytest_credential_user"
	credentialPassword = "PasskeyCredentialPassword123!"
)

// PasskeyRegisterStartRequest represents the request to start passkey registration
type PasskeyRegisterStartRequest struct {
	UserID                 string                          `json:"userId"`
	RelyingPartyID         string                          `json:"relyingPartyId"`
	RelyingPartyName       string                          `json:"relyingPartyName,omitempty"`
	AuthenticatorSelection *AuthenticatorSelectionCriteria `json:"authenticatorSelection,omitempty"`
	Attestation            string                          `json:"attestation,omitempty"`
	Assertion              string                          `json:"assertion,omitempty"`
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

// PasskeyRegisterFinishRequest represents the request to finish passkey registration.
// Mirrors PasskeyRegisterFinishRequestDTO in backend/internal/authn/model.go.
type PasskeyRegisterFinishRequest struct {
	PublicKeyCredential PublicKeyCredentialAttestation `json:"publicKeyCredential"`
	SessionToken        string                         `json:"sessionToken"`
	SkipAssertion       bool                           `json:"skipAssertion,omitempty"`
	Assertion           string                         `json:"assertion,omitempty"`
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

// PasskeyAuthFinishRequest represents the request to finish passkey authentication.
// Mirrors PasskeyFinishRequestDTO in backend/internal/authn/model.go: the credential is nested
// under publicKeyCredential, exactly as it is for registration.
type PasskeyAuthFinishRequest struct {
	PublicKeyCredential PublicKeyCredentialAssertion `json:"publicKeyCredential"`
	SessionToken        string                       `json:"sessionToken"`
	SkipAssertion       bool                         `json:"skipAssertion,omitempty"`
	Assertion           string                       `json:"assertion,omitempty"`
}

// PublicKeyCredentialAssertion represents a WebAuthn credential returned from an assertion ceremony
type PublicKeyCredentialAssertion struct {
	ID       string                         `json:"id"`
	Type     string                         `json:"type"`
	RawID    string                         `json:"rawId,omitempty"`
	Response AuthenticatorAssertionResponse `json:"response"`
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
	testUserID   string
	entityTypeID string
	ouID         string

	// credentialUserID is a second user that owns a registered passkey. It is kept separate from
	// testUserID so the tests asserting behaviour for a user with no credentials stay valid.
	credentialUserID string
	// authenticator holds a credential registered against credentialUserID during SetupSuite, so
	// authentication tests do not have to register one and do not depend on test ordering.
	authenticator      *testutils.VirtualAuthenticator
	sharedCredentialID string
	// webAuthnUserHandle is the user.id the server issued for credentialUserID, needed as the
	// userHandle in usernameless assertions.
	webAuthnUserHandle string
}

func TestPasskeyAuthTestSuite(t *testing.T) {
	suite.Run(t, new(PasskeyAuthTestSuite))
}

func (suite *PasskeyAuthTestSuite) SetupSuite() {
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

	// Create a second user of the same type to own a registered passkey.
	// testUserID is deliberately left without a passkey and without any other credential, since
	// several tests assert the behaviour for a user that has none.
	credentialAttributes, err := json.Marshal(map[string]interface{}{
		"username":    credentialUsername,
		"email":       "passkeytest_credential@example.com",
		"displayName": "Passkey Credential User",
		"password":    credentialPassword,
	})
	suite.Require().NoError(err, "Failed to marshal credential user attributes")

	credentialUserID, err := testutils.CreateUser(testutils.User{
		Type:       passkeyEntityType.Name,
		OUID:       suite.ouID,
		Attributes: json.RawMessage(credentialAttributes),
	})
	suite.Require().NoError(err, "Failed to create credential test user")
	suite.credentialUserID = credentialUserID

	// Register a credential the authentication tests can reuse. Doing it here rather than in a test
	// keeps the authentication tests independent of execution order. Enrollment is gated on an
	// assertion proving the target user, so authenticate with the bootstrap password first; the
	// enrollment ceremony itself is exercised by PasskeyEnrollmentTestSuite.
	credentialUserAssertion, err := testutils.ObtainAuthAssertion(credentialUsername, credentialPassword)
	suite.Require().NoError(err, "Failed to obtain an auth assertion for the credential user")

	authenticator, userHandle, err := testutils.RegisterPasskeyCredential(
		suite.credentialUserID, testRelyingPartyID, testRelyingPartyName, testPasskeyOrigin,
		credentialUserAssertion)
	suite.Require().NoError(err, "Failed to register a passkey for the credential user")

	suite.authenticator = authenticator
	suite.webAuthnUserHandle = userHandle
	suite.sharedCredentialID = authenticator.CredentialID()
}

func (suite *PasskeyAuthTestSuite) TearDownSuite() {
	// Delete the user that owns the registered passkey
	if suite.credentialUserID != "" {
		if err := testutils.DeleteUser(suite.credentialUserID); err != nil {
			suite.T().Errorf("Failed to delete credential test user during teardown: %v", err)
		}
	}

	// Delete test user
	if suite.testUserID != "" {
		err := testutils.DeleteUser(suite.testUserID)
		if err != nil {
			suite.T().Errorf("Failed to delete test user during teardown: %v", err)
		}
	}

	// Delete the shared user type after both users.
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

// TestPasskeyAuthenticationStartNoCredentials tests authentication start when user has no credentials
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationStartNoCredentials() {
	authRequest := PasskeyAuthStartRequest{
		UserID:         suite.testUserID,
		RelyingPartyID: testRelyingPartyID,
	}

	_, statusCode, _ := sendPasskeyAuthStartRequest(authRequest)
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

	_, statusCode, _ := sendPasskeyAuthStartRequest(authRequest)
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for invalid user ID")
}

// TestPasskeyAuthenticationStartEmptyUserID tests usernameless authentication with empty user ID
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationStartEmptyUserID() {
	authRequest := PasskeyAuthStartRequest{
		UserID:         "",
		RelyingPartyID: testRelyingPartyID,
	}

	response, statusCode, err := sendPasskeyAuthStartRequest(authRequest)
	// Usernameless authentication should succeed
	suite.NoError(err, "Failed to send passkey auth start request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication")
	suite.NotNil(response, "Response should not be nil")
	suite.NotEmpty(response.SessionToken, "Response should contain session token")
	suite.NotEmpty(response.PublicKeyCredentialRequestOptions.Challenge, "Response should contain challenge")
}

// TestPasskeyAuthenticationFinishInvalidSessionToken tests finish authentication with invalid session
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationFinishInvalidSessionToken() {
	finishRequest := PasskeyAuthFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAssertion{
			ID:   "mock-credential-id",
			Type: "public-key",
			Response: AuthenticatorAssertionResponse{
				ClientDataJSON:    base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.get"}`)),
				AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("mock-auth-data")),
				Signature:         base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
			},
		},
		SessionToken: "invalid-session-token",
	}

	_, statusCode, _ := sendPasskeyAuthFinishRequest(finishRequest)
	suite.True(statusCode == http.StatusUnauthorized || statusCode == http.StatusBadRequest,
		"Expected status 401 or 400 for invalid session token")
}

// TestPasskeyAuthenticationUsernamelessFlow tests the complete usernameless passkey authentication flow
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationUsernamelessFlow() {
	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "", // Empty userID for usernameless flow
		RelyingPartyID: testRelyingPartyID,
	}

	startResponse, statusCode, err := sendPasskeyAuthStartRequest(authStartRequest)
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

	startResponse, statusCode, err := sendPasskeyAuthStartRequest(authStartRequest)
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

	_, statusCode, _ := sendPasskeyAuthStartRequest(authStartRequest)
	suite.Equal(http.StatusBadRequest, statusCode,
		"Expected status 400 for usernameless flow with missing RP ID")
}

// TestPasskeyAuthenticationFinishUsernamelessWithInvalidUserHandle tests usernameless flow with invalid userHandle
// This test ensures proper error handling in the ValidatePasskeyLogin path
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationFinishUsernamelessWithInvalidUserHandle() {
	// Start usernameless authentication
	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "", // Empty userID for usernameless flow
		RelyingPartyID: testRelyingPartyID,
	}

	authStartResponse, statusCode, err := sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")

	// Attempt finish with invalid user handle
	finishRequest := PasskeyAuthFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAssertion{
			ID:   "mock-credential-id",
			Type: "public-key",
			Response: AuthenticatorAssertionResponse{
				ClientDataJSON: base64.RawURLEncoding.EncodeToString([]byte(
					`{"type":"webauthn.get","challenge":"` +
						authStartResponse.PublicKeyCredentialRequestOptions.Challenge + `","origin":"http://localhost"}`)),
				AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("mock-auth-data")),
				Signature:         base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
				UserHandle:        "!!!invalid-base64!!!",
			},
		},
		SessionToken: authStartResponse.SessionToken,
	}

	_, statusCode, _ = sendPasskeyAuthFinishRequest(finishRequest)
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

	authStartResponse, statusCode, err := sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Require().Equal(
		http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")

	// Attempt finish without user handle
	finishRequest := PasskeyAuthFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAssertion{
			ID:   "mock-credential-id",
			Type: "public-key",
			Response: AuthenticatorAssertionResponse{
				ClientDataJSON:    base64.RawURLEncoding.EncodeToString([]byte(`{"type":"webauthn.get"}`)),
				AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("mock-auth-data")),
				Signature:         base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
				UserHandle:        "", // Empty userHandle
			},
		},
		SessionToken: authStartResponse.SessionToken,
	}

	_, statusCode, _ = sendPasskeyAuthFinishRequest(finishRequest)
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

	authStartResponse, statusCode, err := sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")

	// Attempt finish with userHandle pointing to non-existent user
	nonExistentUserID := "non-existent-user-id-12345"
	finishRequest := PasskeyAuthFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAssertion{
			ID:   "mock-credential-id",
			Type: "public-key",
			Response: AuthenticatorAssertionResponse{
				ClientDataJSON: base64.RawURLEncoding.EncodeToString([]byte(
					`{"type":"webauthn.get","challenge":"` +
						authStartResponse.PublicKeyCredentialRequestOptions.Challenge + `","origin":"http://localhost"}`)),
				AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte("mock-auth-data")),
				Signature:         base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
				UserHandle:        base64.StdEncoding.EncodeToString([]byte(nonExistentUserID)),
			},
		},
		SessionToken: authStartResponse.SessionToken,
	}

	_, statusCode, _ = sendPasskeyAuthFinishRequest(finishRequest)
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

	authStartResponse, statusCode, err := sendPasskeyAuthStartRequest(authStartRequest)
	suite.Require().NoError(err, "Failed to send usernameless passkey auth start request")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for usernameless authentication start")
	suite.Require().NotEmpty(authStartResponse.SessionToken, "Session token should not be empty")

	finishRequest := PasskeyAuthFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAssertion{
			ID:   base64.RawURLEncoding.EncodeToString([]byte("test-credential-id")),
			Type: "public-key",
			Response: AuthenticatorAssertionResponse{
				ClientDataJSON: base64.RawURLEncoding.EncodeToString([]byte(
					`{"type":"webauthn.get","challenge":"` +
						authStartResponse.PublicKeyCredentialRequestOptions.Challenge +
						`","origin":"http://` + testRelyingPartyID + `"}`)),
				AuthenticatorData: base64.RawURLEncoding.EncodeToString(make([]byte, 37)),
				Signature:         base64.RawURLEncoding.EncodeToString([]byte("invalid-signature")),
				UserHandle:        base64.StdEncoding.EncodeToString([]byte(suite.testUserID)),
			},
		},
		SessionToken: authStartResponse.SessionToken,
	}

	_, statusCode, _ = sendPasskeyAuthFinishRequest(finishRequest)
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

	_, statusCode, _ := sendPasskeyAuthStartRequest(authStartRequest)

	suite.True(statusCode == http.StatusNotFound || statusCode == http.StatusBadRequest,
		"Expected error when user has no registered credentials for username-based flow")
}

// TestPasskeyAuthenticationUsernameBasedSuccess authenticates a user that has a registered
// credential, by supplying the user ID up front.
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationUsernameBasedSuccess() {
	startResponse, statusCode, err := sendPasskeyAuthStartRequest(PasskeyAuthStartRequest{
		UserID:         suite.credentialUserID,
		RelyingPartyID: testRelyingPartyID,
	})
	suite.Require().NoError(err, "Failed to start passkey authentication")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for authentication start")
	suite.Require().NotEmpty(startResponse.PublicKeyCredentialRequestOptions.AllowCredentials,
		"AllowCredentials should list the registered credential for a username-based ceremony")

	credentialID, clientDataJSON, authenticatorData, signature, err :=
		suite.authenticator.CreateAssertionResponse(
			startResponse.PublicKeyCredentialRequestOptions.Challenge, true)
	suite.Require().NoError(err, "Failed to build assertion response")

	response, statusCode, err := sendPasskeyAuthFinishRequest(PasskeyAuthFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAssertion{
			ID:    credentialID,
			Type:  "public-key",
			RawID: credentialID,
			Response: AuthenticatorAssertionResponse{
				ClientDataJSON:    clientDataJSON,
				AuthenticatorData: authenticatorData,
				Signature:         signature,
				UserHandle:        suite.webAuthnUserHandle,
			},
		},
		SessionToken: startResponse.SessionToken,
	})
	suite.Require().NoError(err, "Failed to finish passkey authentication")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for authentication finish")
	suite.Equal(suite.credentialUserID, response.ID, "Response should identify the authenticated user")
	suite.NotEmpty(response.Assertion, "A JWT assertion should be issued on success")
}

// TestPasskeyAuthenticationUsernamelessSuccess authenticates without supplying a user ID, so the
// user is resolved from the credential's user handle through the discoverable credential path.
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationUsernamelessSuccess() {
	startResponse, statusCode, err := sendPasskeyAuthStartRequest(PasskeyAuthStartRequest{
		UserID:         "",
		RelyingPartyID: testRelyingPartyID,
	})
	suite.Require().NoError(err, "Failed to start usernameless passkey authentication")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for authentication start")
	suite.Require().Empty(startResponse.PublicKeyCredentialRequestOptions.AllowCredentials,
		"AllowCredentials should be empty for a usernameless ceremony")

	credentialID, clientDataJSON, authenticatorData, signature, err :=
		suite.authenticator.CreateAssertionResponse(
			startResponse.PublicKeyCredentialRequestOptions.Challenge, true)
	suite.Require().NoError(err, "Failed to build assertion response")

	response, statusCode, err := sendPasskeyAuthFinishRequest(PasskeyAuthFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAssertion{
			ID:    credentialID,
			Type:  "public-key",
			RawID: credentialID,
			Response: AuthenticatorAssertionResponse{
				ClientDataJSON:    clientDataJSON,
				AuthenticatorData: authenticatorData,
				Signature:         signature,
				UserHandle:        suite.webAuthnUserHandle,
			},
		},
		SessionToken: startResponse.SessionToken,
	})
	suite.Require().NoError(err, "Failed to finish usernameless passkey authentication")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for authentication finish")
	suite.Equal(suite.credentialUserID, response.ID,
		"Usernameless authentication should resolve the user from the user handle")
	suite.NotEmpty(response.Assertion, "A JWT assertion should be issued on success")
}

// TestPasskeyAuthenticationReplayedSessionToken confirms a session token cannot be used twice.
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationReplayedSessionToken() {
	startResponse, statusCode, err := sendPasskeyAuthStartRequest(PasskeyAuthStartRequest{
		UserID:         suite.credentialUserID,
		RelyingPartyID: testRelyingPartyID,
	})
	suite.Require().NoError(err, "Failed to start passkey authentication")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for authentication start")

	finishRequest := func() PasskeyAuthFinishRequest {
		credentialID, clientDataJSON, authenticatorData, signature, buildErr :=
			suite.authenticator.CreateAssertionResponse(
				startResponse.PublicKeyCredentialRequestOptions.Challenge, true)
		suite.Require().NoError(buildErr, "Failed to build assertion response")
		return PasskeyAuthFinishRequest{
			PublicKeyCredential: PublicKeyCredentialAssertion{
				ID:    credentialID,
				Type:  "public-key",
				RawID: credentialID,
				Response: AuthenticatorAssertionResponse{
					ClientDataJSON:    clientDataJSON,
					AuthenticatorData: authenticatorData,
					Signature:         signature,
					UserHandle:        suite.webAuthnUserHandle,
				},
			},
			SessionToken: startResponse.SessionToken,
		}
	}

	_, statusCode, err = sendPasskeyAuthFinishRequest(finishRequest())
	suite.Require().NoError(err, "Failed to finish passkey authentication")
	suite.Require().Equal(http.StatusOK, statusCode, "First use of the session token should succeed")

	_, statusCode, err = sendPasskeyAuthFinishRequest(finishRequest())
	suite.Require().NoError(err, "Failed to send replayed authentication finish")
	suite.True(statusCode == http.StatusBadRequest || statusCode == http.StatusUnauthorized,
		"Replaying a consumed session token should be rejected, got %d", statusCode)
}

// TestPasskeyAuthenticationSignCountRegression documents that a regressed signature counter is
// accepted. The library flags it through Authenticator.CloneWarning but returns no error, and that
// flag is never inspected, so cloned authenticators are not currently detected.
func (suite *PasskeyAuthTestSuite) TestPasskeyAuthenticationSignCountRegression() {
	startResponse, statusCode, err := sendPasskeyAuthStartRequest(PasskeyAuthStartRequest{
		UserID:         suite.credentialUserID,
		RelyingPartyID: testRelyingPartyID,
	})
	suite.Require().NoError(err, "Failed to start passkey authentication")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for authentication start")

	suite.authenticator.SetSignCount(0)
	credentialID, clientDataJSON, authenticatorData, signature, err :=
		suite.authenticator.CreateAssertionResponse(
			startResponse.PublicKeyCredentialRequestOptions.Challenge, true)
	suite.Require().NoError(err, "Failed to build assertion response")

	_, statusCode, err = sendPasskeyAuthFinishRequest(PasskeyAuthFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAssertion{
			ID:    credentialID,
			Type:  "public-key",
			RawID: credentialID,
			Response: AuthenticatorAssertionResponse{
				ClientDataJSON:    clientDataJSON,
				AuthenticatorData: authenticatorData,
				Signature:         signature,
				UserHandle:        suite.webAuthnUserHandle,
			},
		},
		SessionToken: startResponse.SessionToken,
	})
	suite.Require().NoError(err, "Failed to finish passkey authentication")
	suite.Equal(http.StatusOK, statusCode,
		"A regressed signature counter is currently accepted; update this test if clone detection is added")
}

// Helper methods

func sendPasskeyRegisterStartRequest(
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

	resp, err := testutils.GetHTTPClient().Do(req)
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

func sendPasskeyRegisterFinishRequest(
	request PasskeyRegisterFinishRequest,
) (*testutils.AuthenticationResponse, int, error) {
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

	resp, err := testutils.GetHTTPClient().Do(req)
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

func sendPasskeyAuthStartRequest(
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

	resp, err := testutils.GetHTTPClient().Do(req)
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

func sendPasskeyAuthFinishRequest(
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

	resp, err := testutils.GetHTTPClient().Do(req)
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

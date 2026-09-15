// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authn

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// Enrolling a passkey through the direct API requires an assertion proving the target user, so a
// user needs another credential to authenticate with first. This suite uses a password to obtain
// that assertion, then exercises credential enrollment onto an existing account.
var (
	passkeyEnrollmentTestOU = testutils.OrganizationUnit{
		Handle:      "passkey-enrollment-test-ou",
		Name:        "Passkey Enrollment Test Organization Unit",
		Description: "Organization unit for passkey enrollment testing",
		Parent:      nil,
	}

	passkeyEnrollmentEntityType = testutils.UserType{
		Name: "passkey_enrollment_user",
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
	enrollmentUsername         = "passkeyenrollmentuser"
	enrollmentPassword         = "PasskeyEnrollPassword123!"
	enrollmentAttackerUsername = "passkeyattacker"
	enrollmentAttackerPassword = "AttackerPassword123!"
)

// PasskeyEnrollmentTestSuite covers the direct passkey enrollment API: the registration start and
// finish ceremony, assertion ownership checks, and authentication with an enrolled credential.
type PasskeyEnrollmentTestSuite struct {
	suite.Suite
	ouID         string
	entityTypeID string
	userID       string
	attackerID   string
	// userAssertion proves userID to the registration API. Registration start enrolls only for the
	// subject of the assertion it is given, so the tests need one for the user they register for.
	userAssertion string
}

func TestPasskeyEnrollmentTestSuite(t *testing.T) {
	suite.Run(t, new(PasskeyEnrollmentTestSuite))
}

func (suite *PasskeyEnrollmentTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(passkeyEnrollmentTestOU)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	passkeyEnrollmentEntityType.OUID = suite.ouID
	entityTypeID, err := testutils.CreateUserType(passkeyEnrollmentEntityType)
	suite.Require().NoError(err, "Failed to create test user type")
	suite.entityTypeID = entityTypeID

	attributes, err := json.Marshal(map[string]interface{}{
		"username":    enrollmentUsername,
		"email":       enrollmentUsername + "@example.com",
		"displayName": "Passkey Enrollment User",
		"password":    enrollmentPassword,
	})
	suite.Require().NoError(err, "Failed to marshal user attributes")

	userID, err := testutils.CreateUser(testutils.User{
		Type:       passkeyEnrollmentEntityType.Name,
		OUID:       suite.ouID,
		Attributes: json.RawMessage(attributes),
	})
	suite.Require().NoError(err, "Failed to create test user")
	suite.userID = userID

	suite.userAssertion = suite.assertionForUser(enrollmentUsername, enrollmentPassword, suite.userID)

	attackerAttributes, err := json.Marshal(map[string]interface{}{
		"username": enrollmentAttackerUsername,
		"password": enrollmentAttackerPassword,
	})
	suite.Require().NoError(err, "Failed to marshal attacker attributes")
	attackerID, err := testutils.CreateUser(testutils.User{
		Type:       passkeyEnrollmentEntityType.Name,
		OUID:       suite.ouID,
		Attributes: json.RawMessage(attackerAttributes),
	})
	suite.Require().NoError(err, "Failed to create attacker user")
	suite.attackerID = attackerID
}

func (suite *PasskeyEnrollmentTestSuite) TearDownSuite() {
	for _, userID := range []string{suite.userID, suite.attackerID} {
		if userID == "" {
			continue
		}
		if err := testutils.DeleteUser(userID); err != nil {
			suite.T().Logf("teardown: failed to delete user: %v", err)
		}
	}
	if suite.entityTypeID != "" {
		if err := testutils.DeleteUserType(suite.entityTypeID); err != nil {
			suite.T().Logf("teardown: failed to delete user type: %v", err)
		}
	}
	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("teardown: failed to delete organization unit: %v", err)
		}
	}
}

// assertionForUser authenticates with a password and verifies the assertion belongs to the expected user.
func (suite *PasskeyEnrollmentTestSuite) assertionForUser(username, password, expectedID string) string {
	assertion, err := testutils.ObtainAuthAssertion(username, password)
	suite.Require().NoError(err, "Failed to obtain an auth assertion for %s", username)
	claims, err := testutils.DecodeJWT(assertion)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedID, claims.Sub, "assertion should belong to the expected user")
	return assertion
}

// registerCredential runs a full registration ceremony for the given user using a fresh virtual
// authenticator, and returns the authenticator along with the WebAuthn user handle the server
// issued for that user. The assertion must prove userID. Each authenticator owns a single
// credential ID, so callers that need a distinct credential must call this again rather than
// reusing an existing authenticator.
func (suite *PasskeyEnrollmentTestSuite) registerCredential(
	userID, assertion string,
) (*testutils.VirtualAuthenticator, string) {
	authenticator, err := testutils.NewVirtualAuthenticator(testRelyingPartyID, testPasskeyOrigin)
	suite.Require().NoError(err, "Failed to create virtual authenticator")

	startResponse, statusCode, err := sendPasskeyRegisterStartRequest(PasskeyRegisterStartRequest{
		UserID:           userID,
		RelyingPartyID:   testRelyingPartyID,
		RelyingPartyName: testRelyingPartyName,
		AuthenticatorSelection: &AuthenticatorSelectionCriteria{
			ResidentKey:      "required",
			UserVerification: "required",
		},
		Assertion: assertion,
	})
	suite.Require().NoError(err, "Failed to start passkey registration")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for registration start")

	credentialID, clientDataJSON, attestationObject, err := authenticator.CreateAttestationResponse(
		startResponse.PublicKeyCredentialCreationOptions.Challenge, true)
	suite.Require().NoError(err, "Failed to build attestation response")

	_, statusCode, err = sendPasskeyRegisterFinishRequest(PasskeyRegisterFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAttestation{
			ID:    credentialID,
			Type:  "public-key",
			RawID: credentialID,
			Response: AuthenticatorAttestationResponse{
				ClientDataJSON:    clientDataJSON,
				AttestationObject: attestationObject,
			},
		},
		SessionToken: startResponse.SessionToken,
	})
	suite.Require().NoError(err, "Failed to finish passkey registration")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for registration finish")

	return authenticator, startResponse.PublicKeyCredentialCreationOptions.User.ID
}

// TestPasskeyRegistrationStart tests the start of passkey registration
func (suite *PasskeyEnrollmentTestSuite) TestPasskeyRegistrationStart() {
	registerRequest := PasskeyRegisterStartRequest{
		UserID:           suite.userID,
		RelyingPartyID:   testRelyingPartyID,
		RelyingPartyName: testRelyingPartyName,
		Assertion:        suite.userAssertion,
	}

	response, statusCode, err := sendPasskeyRegisterStartRequest(registerRequest)
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
func (suite *PasskeyEnrollmentTestSuite) TestPasskeyRegistrationStartWithAuthenticatorSelection() {
	registerRequest := PasskeyRegisterStartRequest{
		UserID:           suite.userID,
		RelyingPartyID:   testRelyingPartyID,
		RelyingPartyName: testRelyingPartyName,
		AuthenticatorSelection: &AuthenticatorSelectionCriteria{
			AuthenticatorAttachment: "platform",
			RequireResidentKey:      true,
			ResidentKey:             "required",
			UserVerification:        "required",
		},
		Attestation: "direct",
		Assertion:   suite.userAssertion,
	}

	response, statusCode, err := sendPasskeyRegisterStartRequest(registerRequest)
	suite.Require().NoError(err, "Failed to send passkey register start request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful registration start")

	// Verify response includes authenticator selection
	suite.NotNil(response.PublicKeyCredentialCreationOptions.AuthenticatorSelection,
		"Response should contain authenticator selection")
	suite.Equal("platform",
		response.PublicKeyCredentialCreationOptions.AuthenticatorSelection.AuthenticatorAttachment)
	suite.Equal("direct", response.PublicKeyCredentialCreationOptions.Attestation)
}

// TestPasskeyRegistrationStartInvalidUserID tests registration with invalid user ID. The assertion
// proves a different user, so the mismatch is what rejects it.
func (suite *PasskeyEnrollmentTestSuite) TestPasskeyRegistrationStartInvalidUserID() {
	registerRequest := PasskeyRegisterStartRequest{
		UserID:         "invalid-user-id",
		RelyingPartyID: testRelyingPartyID,
		Assertion:      suite.userAssertion,
	}

	_, statusCode, _ := sendPasskeyRegisterStartRequest(registerRequest)
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for invalid user ID")
}

// TestPasskeyRegistrationStartEmptyUserID tests registration with empty user ID
func (suite *PasskeyEnrollmentTestSuite) TestPasskeyRegistrationStartEmptyUserID() {
	registerRequest := PasskeyRegisterStartRequest{
		UserID:         "",
		RelyingPartyID: testRelyingPartyID,
		Assertion:      suite.userAssertion,
	}

	_, statusCode, _ := sendPasskeyRegisterStartRequest(registerRequest)
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for empty user ID")
}

// TestPasskeyRegistrationStartEmptyRelyingPartyID tests registration with empty relying party ID.
// The relying party ID is a required field, so this is caught by request validation.
func (suite *PasskeyEnrollmentTestSuite) TestPasskeyRegistrationStartEmptyRelyingPartyID() {
	registerRequest := PasskeyRegisterStartRequest{
		UserID:         suite.userID,
		RelyingPartyID: "",
		Assertion:      suite.userAssertion,
	}

	_, statusCode, _ := sendPasskeyRegisterStartRequest(registerRequest)
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for empty relying party ID")
}

// TestPasskeyRegistrationFinishInvalidSessionToken tests finish registration with invalid session
func (suite *PasskeyEnrollmentTestSuite) TestPasskeyRegistrationFinishInvalidSessionToken() {
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
		SessionToken: "invalid-session-token",
	}

	_, statusCode, _ := sendPasskeyRegisterFinishRequest(finishRequest)
	suite.True(statusCode == http.StatusUnauthorized || statusCode == http.StatusBadRequest,
		"Expected status 401 or 400 for invalid session token")
}

// TestPasskeyAuthenticationFinishUsernamelessWithValidCredential tests finish authentication for usernameless flow
// This test covers the ValidatePasskeyLogin path including the type assertion of user interface
func (suite *PasskeyEnrollmentTestSuite) TestPasskeyAuthenticationFinishUsernamelessWithValidCredential() {
	registerStartRequest := PasskeyRegisterStartRequest{
		UserID:           suite.userID,
		RelyingPartyID:   testRelyingPartyID,
		RelyingPartyName: testRelyingPartyName,
		AuthenticatorSelection: &AuthenticatorSelectionCriteria{
			ResidentKey:      "required",
			UserVerification: "required",
		},
		Assertion: suite.userAssertion,
	}

	registerStartResponse, statusCode, err := sendPasskeyRegisterStartRequest(registerStartRequest)
	suite.Require().NoError(err, "Failed to send passkey register start request")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for registration start")
	suite.Require().NotEmpty(registerStartResponse.SessionToken, "Session token should not be empty")

	authStartRequest := PasskeyAuthStartRequest{
		UserID:         "", // Empty userID for usernameless flow
		RelyingPartyID: testRelyingPartyID,
	}

	authStartResponse, statusCode, err := sendPasskeyAuthStartRequest(authStartRequest)
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
		PublicKeyCredential: PublicKeyCredentialAssertion{
			ID:   "mock-credential-id",
			Type: "public-key",
			Response: AuthenticatorAssertionResponse{
				ClientDataJSON: base64.RawURLEncoding.EncodeToString([]byte(
					`{"type":"webauthn.get","challenge":"` +
						authStartResponse.PublicKeyCredentialRequestOptions.Challenge + `","origin":"http://localhost"}`)),
				AuthenticatorData: base64.RawURLEncoding.EncodeToString([]byte(
					"mock-auth-data-with-sufficient-length-for-parsing")),
				Signature:  base64.RawURLEncoding.EncodeToString([]byte("mock-signature")),
				UserHandle: base64.StdEncoding.EncodeToString([]byte(suite.userID)),
			},
		},
		SessionToken: authStartResponse.SessionToken,
	}

	_, statusCode, _ = sendPasskeyAuthFinishRequest(finishRequest)
	// Should fail validation but the code path including type assertion should be exercised
	suite.True(statusCode == http.StatusBadRequest || statusCode == http.StatusUnauthorized,
		"Expected validation error status for mock credential")
}

// TestPasskeyRegisterFinishSuccess completes a registration ceremony with a credential that carries
// a real signature, and confirms the credential is persisted against the user.
func (suite *PasskeyEnrollmentTestSuite) TestPasskeyRegisterFinishSuccess() {
	// Register against a dedicated user so the credential does not affect other tests.
	attributes, err := json.Marshal(map[string]interface{}{
		"username":    "passkeytest_register_user",
		"email":       "passkeytest_register@example.com",
		"displayName": "Passkey Register User",
		"password":    "PasskeyRegisterPassword123!",
	})
	suite.Require().NoError(err, "Failed to marshal user attributes")

	userID, err := testutils.CreateUser(testutils.User{
		Type:       passkeyEnrollmentEntityType.Name,
		OUID:       suite.ouID,
		Attributes: json.RawMessage(attributes),
	})
	suite.Require().NoError(err, "Failed to create user for registration test")
	defer func() {
		if err := testutils.DeleteUser(userID); err != nil {
			suite.T().Logf("Failed to delete registration test user: %v", err)
		}
	}()

	registerUserAssertion := suite.assertionForUser("passkeytest_register_user", "PasskeyRegisterPassword123!", userID)
	authenticator, _ := suite.registerCredential(userID, registerUserAssertion)

	// Stored credentials are not exposed by any API, so persistence is confirmed by starting an
	// authentication ceremony and checking the credential is offered back.
	startResponse, statusCode, err := sendPasskeyAuthStartRequest(PasskeyAuthStartRequest{
		UserID:         userID,
		RelyingPartyID: testRelyingPartyID,
	})
	suite.Require().NoError(err, "Failed to start authentication after registration")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for authentication start")

	credentialIDs := make([]string, 0, len(startResponse.PublicKeyCredentialRequestOptions.AllowCredentials))
	for _, credential := range startResponse.PublicKeyCredentialRequestOptions.AllowCredentials {
		credentialIDs = append(credentialIDs, credential.ID)
	}
	suite.Contains(credentialIDs, authenticator.CredentialID(),
		"Registered credential should be offered back in allowCredentials")
}

// TestPasskeyRegisterFinishWrongOrigin rejects a credential collected from an origin the server
// does not allow.
func (suite *PasskeyEnrollmentTestSuite) TestPasskeyRegisterFinishWrongOrigin() {
	statusCode := suite.attemptRegistrationWithAuthenticator(testRelyingPartyID,
		"https://evil.example.com")
	suite.Equal(http.StatusBadRequest, statusCode,
		"Expected status 400 for a credential collected from a disallowed origin")
}

// TestPasskeyRegisterFinishWrongRPID rejects a credential whose authenticator data was bound to a
// different relying party.
func (suite *PasskeyEnrollmentTestSuite) TestPasskeyRegisterFinishWrongRPID() {
	statusCode := suite.attemptRegistrationWithAuthenticator("example.com", testPasskeyOrigin)
	suite.Equal(http.StatusBadRequest, statusCode,
		"Expected status 400 for a credential bound to a different relying party")
}

// attemptRegistrationWithAuthenticator runs a registration ceremony where the authenticator is
// built with the given relying party ID and origin, and returns the status of the finish call. The
// start call is expected to succeed, since these mismatches are only detectable at finish.
func (suite *PasskeyEnrollmentTestSuite) attemptRegistrationWithAuthenticator(rpID, origin string) int {
	authenticator, err := testutils.NewVirtualAuthenticator(rpID, origin)
	suite.Require().NoError(err, "Failed to create virtual authenticator")

	startResponse, statusCode, err := sendPasskeyRegisterStartRequest(PasskeyRegisterStartRequest{
		UserID:           suite.userID,
		RelyingPartyID:   testRelyingPartyID,
		RelyingPartyName: testRelyingPartyName,
		Assertion:        suite.userAssertion,
	})
	suite.Require().NoError(err, "Failed to start passkey registration")
	suite.Require().Equal(http.StatusOK, statusCode, "Expected status 200 for registration start")

	credentialID, clientDataJSON, attestationObject, err := authenticator.CreateAttestationResponse(
		startResponse.PublicKeyCredentialCreationOptions.Challenge, true)
	suite.Require().NoError(err, "Failed to build attestation response")

	_, statusCode, err = sendPasskeyRegisterFinishRequest(PasskeyRegisterFinishRequest{
		PublicKeyCredential: PublicKeyCredentialAttestation{
			ID:    credentialID,
			Type:  "public-key",
			RawID: credentialID,
			Response: AuthenticatorAttestationResponse{
				ClientDataJSON:    clientDataJSON,
				AttestationObject: attestationObject,
			},
		},
		SessionToken: startResponse.SessionToken,
	})
	suite.Require().NoError(err, "Failed to finish passkey registration")

	return statusCode
}

// TestStartRejectsRequestWithoutAssertion asserts a registration start that proves nothing about
// the caller is rejected, rather than handing out a challenge bound to the named user. An absent
// assertion is the zero value of a required field, so request validation catches it before the
// service runs, which is why the response carries the structural code rather than a service one.
func (suite *PasskeyEnrollmentTestSuite) TestStartRejectsRequestWithoutAssertion() {
	status, body := suite.postRegisterStart(map[string]interface{}{
		"userId":         suite.userID,
		"relyingPartyId": testRelyingPartyID,
	})

	suite.Equal(http.StatusBadRequest, status,
		"registration start must not issue a challenge for a user the caller has not proven; body: %s", body)
	suite.Equal("INVALID_INPUT_METADATA", suite.errorCode(body), "body: %s", body)
}

// TestStartRejectsEmptyAssertion asserts an explicitly empty assertion is rejected the same way an
// absent one is, since both are the field's zero value.
func (suite *PasskeyEnrollmentTestSuite) TestStartRejectsEmptyAssertion() {
	status, body := suite.postRegisterStart(map[string]interface{}{
		"userId":         suite.userID,
		"relyingPartyId": testRelyingPartyID,
		"assertion":      "",
	})

	suite.Equal(http.StatusBadRequest, status, "body: %s", body)
	suite.Equal("INVALID_INPUT_METADATA", suite.errorCode(body), "body: %s", body)
}

// TestStartRejectsBlankAssertion asserts a whitespace only assertion is rejected too. Required
// field validation treats a blank string as absent, so this lands with the other zero values
// rather than reaching the service.
func (suite *PasskeyEnrollmentTestSuite) TestStartRejectsBlankAssertion() {
	status, body := suite.postRegisterStart(map[string]interface{}{
		"userId":         suite.userID,
		"relyingPartyId": testRelyingPartyID,
		"assertion":      "   ",
	})

	suite.Equal(http.StatusBadRequest, status, "body: %s", body)
	suite.Equal("INVALID_INPUT_METADATA", suite.errorCode(body), "body: %s", body)
}

// TestStartRejectsAssertionForDifferentUser asserts an assertion proving one identity cannot be
// used to enroll a passkey onto a different account.
func (suite *PasskeyEnrollmentTestSuite) TestStartRejectsAssertionForDifferentUser() {
	attackerAssertion := suite.assertionForUser(enrollmentAttackerUsername, enrollmentAttackerPassword, suite.attackerID)

	status, body := suite.postRegisterStart(map[string]interface{}{
		"userId":         suite.userID,
		"relyingPartyId": testRelyingPartyID,
		"assertion":      attackerAssertion,
	})

	suite.Equal(http.StatusBadRequest, status,
		"registration start must reject an assertion whose subject is not the target user; body: %s", body)
	suite.Equal("AUTHN-1010", suite.errorCode(body), "body: %s", body)
}

// TestStartRejectsMissingRelyingPartyID asserts the relying party ID is enforced as a required
// field, rather than reaching the passkey service and surfacing as a generic enrollment failure.
func (suite *PasskeyEnrollmentTestSuite) TestStartRejectsMissingRelyingPartyID() {
	victimAssertion := suite.assertionForUser(enrollmentUsername, enrollmentPassword, suite.userID)

	status, body := suite.postRegisterStart(map[string]interface{}{
		"userId":    suite.userID,
		"assertion": victimAssertion,
	})

	suite.Equal(http.StatusBadRequest, status, "body: %s", body)
	suite.Equal("INVALID_INPUT_METADATA", suite.errorCode(body), "body: %s", body)
}

// TestStartRejectsUnverifiableAssertion asserts a forged assertion is rejected on verification.
func (suite *PasskeyEnrollmentTestSuite) TestStartRejectsUnverifiableAssertion() {
	status, body := suite.postRegisterStart(map[string]interface{}{
		"userId":         suite.userID,
		"relyingPartyId": testRelyingPartyID,
		"assertion":      "not.a.valid-jwt",
	})

	suite.Equal(http.StatusBadRequest, status, "body: %s", body)
	suite.Equal("AUTHN-1009", suite.errorCode(body), "body: %s", body)
}

// TestAttackerCannotTakeOverVictimAccount verifies that an assertion for another user is rejected
// at enrollment start with the expected error and without issuing a session token.
func (suite *PasskeyEnrollmentTestSuite) TestAttackerCannotTakeOverVictimAccount() {
	attackerAssertion := suite.assertionForUser(enrollmentAttackerUsername, enrollmentAttackerPassword, suite.attackerID)

	// The ceremony must fail at the first step, and for the right reason: the attacker's assertion
	// does not name the victim. Asserting the code rather than merely "an error" keeps the test from
	// passing on an unrelated failure such as a wrong origin or an expired session.
	status, body := suite.postRegisterStart(map[string]interface{}{
		"userId":         suite.userID,
		"relyingPartyId": testRelyingPartyID,
		"assertion":      attackerAssertion,
	})
	suite.Require().Equal(http.StatusBadRequest, status,
		"ACCOUNT TAKEOVER: registration start issued a challenge for the victim to a caller holding "+
			"only the Direct Auth Secret, its own account, and the victim's user ID; body: %s", body)
	suite.Require().Equal("AUTHN-1010", suite.errorCode(body), "body: %s", body)

	// No session token was issued, so the attacker cannot reach finish and no credential can come
	// into existence from this attempt.
	suite.NotContains(body, "sessionToken", "a rejected start must not return a session token")
}

// TestUserCanEnrollWithOwnAssertion is the positive case: a user that proves its own identity
// enrolls a passkey and can then authenticate with it.
func (suite *PasskeyEnrollmentTestSuite) TestUserCanEnrollWithOwnAssertion() {
	victimAssertion := suite.assertionForUser(enrollmentUsername, enrollmentPassword, suite.userID)

	authenticator, userHandle, err := testutils.RegisterPasskeyCredential(
		suite.userID, testRelyingPartyID, testRelyingPartyName, testPasskeyOrigin, victimAssertion)
	suite.Require().NoError(err, "a user proving its own identity should be able to enroll a passkey")

	authResponse, err := testutils.AuthenticateWithPasskey(
		suite.userID, testRelyingPartyID, userHandle, authenticator)
	suite.Require().NoError(err, "the enrolled passkey should be usable for authentication")
	suite.Equal(suite.userID, authResponse.ID, "authentication should return the enrolling user")
	suite.NotEmpty(authResponse.Assertion, "authentication should return an assertion")
}

// TestStartAcceptsOwnAssertion asserts the happy path at the start endpoint on its own, so a
// failure here is distinguishable from a failure later in the ceremony.
func (suite *PasskeyEnrollmentTestSuite) TestStartAcceptsOwnAssertion() {
	victimAssertion := suite.assertionForUser(enrollmentUsername, enrollmentPassword, suite.userID)

	status, body := suite.postRegisterStart(map[string]interface{}{
		"userId":         suite.userID,
		"relyingPartyId": testRelyingPartyID,
		"assertion":      victimAssertion,
	})

	suite.Equal(http.StatusOK, status,
		"registration start should accept an assertion proving the target user; body: %s", body)
}

// errorCode pulls the top level error code out of a response body, so the tests pin the exact code
// a rejection carries rather than only its status.
func (suite *PasskeyEnrollmentTestSuite) errorCode(body string) string {
	var parsed struct {
		Code string `json:"code"`
	}
	suite.Require().NoError(json.Unmarshal([]byte(body), &parsed), "failed to decode error body: %s", body)
	return parsed.Code
}

// postRegisterStart posts a raw registration start payload and returns the status and body. The
// test client injects the Direct Auth Secret, so this models a caller that holds the secret.
func (suite *PasskeyEnrollmentTestSuite) postRegisterStart(payload map[string]interface{}) (int, string) {
	body, err := json.Marshal(payload)
	suite.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost,
		testutils.TestServerURL+passkeyRegisterStartEndpoint, bytes.NewReader(body))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	return resp.StatusCode, string(responseBody)
}

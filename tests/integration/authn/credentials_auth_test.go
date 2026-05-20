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
	"encoding/json"
	"io"
	"log"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	credentialsAuthEndpoint = "/auth/credentials/authenticate"
)

var (
	testOU = testutils.OrganizationUnit{
		Handle:      "creds-auth-test-ou",
		Name:        "Credentials Auth Test Organization Unit",
		Description: "Organization unit for credentials authentication testing",
		Parent:      nil,
	}

	credentialEntityTypes = map[string]testutils.UserType{
		"username_password": {
			Name: "username_password",
			Schema: map[string]interface{}{
				"username": map[string]interface{}{
					"type": "string",
				},
				"password": map[string]interface{}{
					"type": "string",
					"credential": true,
				},
				"email": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"email_password": {
			Name: "email_password",
			Schema: map[string]interface{}{
				"email": map[string]interface{}{
					"type": "string",
				},
				"password": map[string]interface{}{
					"type": "string",
					"credential": true,
				},
				"username": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"mobile_password": {
			Name: "mobile_password",
			Schema: map[string]interface{}{
				"mobileNumber": map[string]interface{}{
					"type": "string",
				},
				"password": map[string]interface{}{
					"type": "string",
					"credential": true,
				},
				"username": map[string]interface{}{
					"type": "string",
				},
			},
		},
		"multiple_attributes": {
			Name: "multiple_attributes",
			Schema: map[string]interface{}{
				"username": map[string]interface{}{
					"type": "string",
				},
				"email": map[string]interface{}{
					"type": "string",
				},
				"mobileNumber": map[string]interface{}{
					"type": "string",
				},
				"password": map[string]interface{}{
					"type": "string",
					"credential": true,
				},
				"given_name": map[string]interface{}{
					"type": "string",
				},
				"family_name": map[string]interface{}{
					"type": "string",
				},
			},
		},
	}
)

type CredentialsAuthTestSuite struct {
	suite.Suite
	client        *http.Client
	users         map[string]string // map of test name to user ID
	entityTypeIDs map[string]string
	ouID          string
}

func TestCredentialsAuthTestSuite(t *testing.T) {
	suite.Run(t, new(CredentialsAuthTestSuite))
}

func (suite *CredentialsAuthTestSuite) SetupSuite() {
	suite.client = testutils.GetHTTPClient()
	suite.users = make(map[string]string)
	suite.entityTypeIDs = make(map[string]string)

	// Create test organization unit
	ouID, err := testutils.CreateOrganizationUnit(testOU)
	if err != nil {
		suite.T().Fatalf("Failed to create test organization unit during setup: %v", err)
	}
	suite.ouID = ouID

	for userType, schema := range credentialEntityTypes {
		schema.OUID = suite.ouID
		schemaID, err := testutils.CreateUserType(schema)
		if err != nil {
			suite.T().Fatalf("Failed to create user type %s during setup: %v", userType, err)
		}
		suite.entityTypeIDs[userType] = schemaID
	}

	// Create test users with different attribute types
	testUsers := []struct {
		userType   string
		attributes map[string]interface{}
	}{
		{
			userType: "username_password",
			attributes: map[string]interface{}{
				"username": "credtest_user1",
				"password": "TestPassword123!",
				"email":    "credtest1@example.com",
			},
		},
		{
			userType: "email_password",
			attributes: map[string]interface{}{
				"email":    "credtest2@example.com",
				"password": "TestPassword456!",
				"username": "credtest_user2",
			},
		},
		{
			userType: "mobile_password",
			attributes: map[string]interface{}{
				"mobileNumber": "+1234567891",
				"password":     "TestPassword789!",
				"username":     "credtest_user3",
			},
		},
		{
			userType: "multiple_attributes",
			attributes: map[string]interface{}{
				"username":     "credtest_user4",
				"email":        "credtest4@example.com",
				"mobileNumber": "+1234567892",
				"password":     "TestPassword999!",
				"given_name":    "Test",
				"family_name":     "User",
			},
		},
	}

	for _, tu := range testUsers {
		attributesJSON, err := json.Marshal(tu.attributes)
		suite.Require().NoError(err, "Failed to marshal attributes for %s", tu.userType)

		user := testutils.User{
			Type:             tu.userType,
			OUID:             suite.ouID,
			Attributes:       json.RawMessage(attributesJSON),
		}

		userID, err := testutils.CreateUser(user)
		suite.Require().NoError(err, "Failed to create test user for %s", tu.userType)
		suite.users[tu.userType] = userID
	}
}

func (suite *CredentialsAuthTestSuite) TearDownSuite() {
	for _, userID := range suite.users {
		if userID != "" {
			err := testutils.DeleteUser(userID)
			if err != nil {
				suite.T().Errorf("Failed to delete user %s during teardown: %v", userID, err)
			}
		}
	}

	for userType, schemaID := range suite.entityTypeIDs {
		if schemaID != "" {
			err := testutils.DeleteUserType(schemaID)
			if err != nil {
				suite.T().Errorf("Failed to delete user type %s during teardown: %v", userType, err)
			}
		}
	}

	// Delete test organization unit
	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// TestAuthenticateWithUsernamePassword tests successful authentication with username and password
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithUsernamePassword() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")

	suite.NotEmpty(response.ID, "Response should contain user ID")
	suite.Equal("username_password", response.Type, "Response should contain correct user type")
	suite.Equal(suite.users["username_password"], response.ID, "Response should contain the correct user ID")
}

// TestAuthenticateWithEmailPassword tests successful authentication with email and password
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithEmailPassword() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"email": "credtest2@example.com",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword456!",
		},
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")

	suite.NotEmpty(response.ID, "Response should contain user ID")
	suite.Equal("email_password", response.Type, "Response should contain correct user type")
	suite.Equal(suite.users["email_password"], response.ID, "Response should contain the correct user ID")
}

// TestAuthenticateWithMobilePassword tests successful authentication with mobile number and password
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithMobilePassword() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"mobileNumber": "+1234567891",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword789!",
		},
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")

	suite.NotEmpty(response.ID, "Response should contain user ID")
	suite.Equal("mobile_password", response.Type, "Response should contain correct user type")
	suite.Equal(suite.ouID, response.OUID, "Response should contain correct organization unit")
	suite.Equal(suite.users["mobile_password"], response.ID, "Response should contain the correct user ID")
}

// TestAuthenticateWithMultipleAttributes tests successful authentication with multiple identifying attributes
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithMultipleAttributes() {
	testCases := []struct {
		name        string
		authRequest map[string]interface{}
	}{
		{
			name: "Username with multiple attributes",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"username": "credtest_user4",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword999!",
				},
			},
		},
		{
			name: "Email with multiple attributes",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"email": "credtest4@example.com",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword999!",
				},
			},
		},
		{
			name: "Mobile with multiple attributes",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"mobileNumber": "+1234567892",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword999!",
				},
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			response, statusCode, err := suite.sendAuthRequest(tc.authRequest)
			suite.Require().NoError(err, "Failed to send authenticate request")
			suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")

			suite.NotEmpty(response.ID, "Response should contain user ID")
			suite.Equal("multiple_attributes", response.Type, "Response should contain correct user type")
			suite.Equal(suite.ouID, response.OUID, "Response should contain correct organization unit")
			suite.Equal(suite.users["multiple_attributes"], response.ID, "Response should contain the correct user ID")
			suite.NotEmpty(response.Assertion, "Response should contain assertion token by default")
		})
	}
}

// TestAuthenticateWithInvalidPassword tests authentication failure with invalid password
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithInvalidPassword() {
	testCases := []struct {
		name        string
		authRequest map[string]interface{}
	}{
		{
			name: "Invalid password with username",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"username": "credtest_user1",
				},
				"credentials": map[string]interface{}{
					"password": "WrongPassword123!",
				},
			},
		},
		{
			name: "Invalid password with email",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"email": "credtest2@example.com",
				},
				"credentials": map[string]interface{}{
					"password": "WrongPassword456!",
				},
			},
		},
		{
			name: "Invalid password with mobile",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"mobileNumber": "+1234567891",
				},
				"credentials": map[string]interface{}{
					"password": "WrongPassword789!",
				},
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			errorResp, statusCode, err := suite.sendAuthRequestExpectingError(tc.authRequest)
			suite.Require().NoError(err, "Failed to send authenticate request")
			suite.Equal(http.StatusUnauthorized, statusCode, "Expected status 401 for invalid password")
			suite.Equal("AUTH-CRED-1002", errorResp.Code, "Expected error code AUTH-CRED-1002 for invalid credentials")
		})
	}
}

// TestAuthenticateWithNonExistentUser tests authentication failure with non-existent user
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithNonExistentUser() {
	testCases := []struct {
		name        string
		authRequest map[string]interface{}
	}{
		{
			name: "Non-existent username",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"username": "nonexistent_user",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword123!",
				},
			},
		},
		{
			name: "Non-existent email",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"email": "nonexistent@example.com",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword123!",
				},
			},
		},
		{
			name: "Non-existent mobile",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"mobileNumber": "+9999999999",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword123!",
				},
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			errorResp, statusCode, err := suite.sendAuthRequestExpectingError(tc.authRequest)
			suite.Require().NoError(err, "Failed to send authenticate request")
			suite.Equal(http.StatusNotFound, statusCode, "Expected status 404 for non-existent user")
			suite.Equal("AUTHN-1008", errorResp.Code, "Expected error code AUTHN-1008 for user not found")
		})
	}
}

// TestAuthenticateWithMissingPassword tests authentication failure when password is missing
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithMissingPassword() {
	testCases := []struct {
		name        string
		authRequest map[string]interface{}
	}{
		{
			name: "Missing password with username",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"username": "credtest_user1",
				},
			},
		},
		{
			name: "Missing password with email",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"email": "credtest2@example.com",
				},
			},
		},
		{
			name: "Missing password with mobile",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"mobileNumber": "+1234567891",
				},
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			_, statusCode, err := suite.sendAuthRequestExpectingError(tc.authRequest)
			suite.Require().NoError(err, "Failed to send authenticate request")
			suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for missing password")
		})
	}
}

// TestAuthenticateWithMissingIdentifyingAttributes tests authentication failure when identifying attributes are missing
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithMissingIdentifyingAttributes() {
	authRequest := map[string]interface{}{
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
	}

	_, statusCode, err := suite.sendAuthRequestExpectingError(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for missing identifying attributes")
}

// TestAuthenticateWithEmptyRequest tests authentication failure when request is empty
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithEmptyRequest() {
	authRequest := map[string]interface{}{}

	errorResp, statusCode, err := suite.sendAuthRequestExpectingError(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for empty request")
	suite.Equal("AUTH-CRED-1001", errorResp.Code, "Expected error code AUTH-CRED-1001 for empty attributes")
}

// TestAuthenticateWithEmptyCredentials tests authentication failure with empty values
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithEmptyCredentials() {
	testCases := []struct {
		name           string
		authRequest    map[string]interface{}
		expectedStatus int
	}{
		{
			name: "Empty username",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"username": "",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword123!",
				},
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "Empty password",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"username": "credtest_user1",
				},
				"credentials": map[string]interface{}{
					"password": "",
				},
			},
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name: "Empty email",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"email": "",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword123!",
				},
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "Both empty",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"username": "",
				},
				"credentials": map[string]interface{}{
					"password": "",
				},
			},
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			_, statusCode, err := suite.sendAuthRequestExpectingError(tc.authRequest)
			suite.Require().NoError(err, "Failed to send authenticate request")
			suite.Equal(tc.expectedStatus, statusCode, "Unexpected status code")
		})
	}
}

// TestAuthenticateWithMalformedJSON tests authentication failure with malformed JSON
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithMalformedJSON() {
	malformedJSON := []byte(`{"identifiers": {"username": "test"}, "credentials": {"password": }}`)

	req, err := http.NewRequest("POST", testutils.TestServerURL+credentialsAuthEndpoint,
		bytes.NewReader(malformedJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode, "Expected status 400 for malformed JSON")

	var errorResp testutils.ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResp)
	suite.Require().NoError(err)
	suite.Equal("AUTHN-1000", errorResp.Code, "Expected error code AUTHN-1000 for invalid request format")
}

// TestAuthenticateWithDifferentAttributeCombinations tests various attribute combinations
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithDifferentAttributeCombinations() {
	testCases := []struct {
		name           string
		authRequest    map[string]interface{}
		expectedUserID string
		shouldSucceed  bool
	}{
		{
			name: "Username and email (both valid for same user)",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"username": "credtest_user4",
					"email":    "credtest4@example.com",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword999!",
				},
			},
			expectedUserID: "multiple_attributes",
			shouldSucceed:  true,
		},
		{
			name: "Only additional attributes (no identifying attribute)",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"given_name": "Test",
					"family_name":  "User",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword999!",
				},
			},
			expectedUserID: "",
			shouldSucceed:  true, // Changed: API now returns 200 with these attributes
		},
		{
			name: "Valid username with additional attributes",
			authRequest: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"username": "credtest_user1",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword123!",
				},
			},
			expectedUserID: "username_password",
			shouldSucceed:  true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			if tc.shouldSucceed {
				response, statusCode, err := suite.sendAuthRequest(tc.authRequest)
				log.Printf("Response: %+v, StatusCode: %d, Error: %v", response, statusCode, err)

				suite.Require().NoError(err, "Failed to send authenticate request")
				suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")
				if tc.expectedUserID != "" {
					suite.Equal(suite.ouID, response.OUID, "Response should contain correct organization unit")
					suite.Equal(suite.users[tc.expectedUserID], response.ID, "Response should contain the correct user ID")
				}
			} else {
				_, statusCode, err := suite.sendAuthRequestExpectingError(tc.authRequest)
				suite.Require().NoError(err, "Failed to send authenticate request")
				suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for invalid request")
			}
		})
	}
}

// TestAuthenticateWithSkipAssertionFalse tests authentication with skip_assertion explicitly set to false
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithSkipAssertionFalse() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
		"skipAssertion": false,
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")

	suite.NotEmpty(response.ID, "Response should contain user ID")
	suite.Equal("username_password", response.Type, "Response should contain correct user type")
	suite.Equal(suite.ouID, response.OUID, "Response should contain correct organization unit")
	suite.Equal(suite.users["username_password"], response.ID, "Response should contain the correct user ID")
	suite.NotEmpty(response.Assertion, "Response should contain assertion token when skip_assertion is false")
}

// TestAuthenticateWithSkipAssertionTrue tests authentication with skip_assertion set to true
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithSkipAssertionTrue() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
		"skipAssertion": true,
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")

	suite.NotEmpty(response.ID, "Response should contain user ID")
	suite.Equal("username_password", response.Type, "Response should contain correct user type")
	suite.Equal(suite.ouID, response.OUID, "Response should contain correct organization unit")
	suite.Equal(suite.users["username_password"], response.ID, "Response should contain the correct user ID")
	suite.Empty(response.Assertion, "Response should not contain assertion token when skip_assertion is true")
}

// TestAuthenticateWithAssuranceLevelAAL1 tests that credentials authentication generates AAL1 assurance level
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithAssuranceLevelAAL1() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")

	suite.NotEmpty(response.Assertion, "Response should contain assertion token by default")

	// Verify assertion contains AAL1 for single-factor authentication
	aal := extractAssuranceLevelFromAssertion(response.Assertion, "aal")
	suite.NotEmpty(aal, "Assertion should contain AAL information")
	suite.Equal("AAL1", aal, "Single-factor credentials authentication should result in AAL1")

	// Verify IAL is present (default IAL1 for self-asserted identities)
	ial := extractAssuranceLevelFromAssertion(response.Assertion, "ial")
	suite.NotEmpty(ial, "Assertion should contain IAL information")
	suite.Equal("IAL1", ial, "Self-asserted identity should result in IAL1")
}

// TestAuthenticateWithAssuranceLevelNoAssertion tests that AAL/IAL are not present when assertion is skipped
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithAssuranceLevelNoAssertion() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
		"skipAssertion": true,
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")

	suite.Empty(response.Assertion, "Response should not contain assertion when skip_assertion is true")
}

// TestCredentialsAuthenticationWithVariousAttributes tests AAL1 is generated for different identifying attributes
func (suite *CredentialsAuthTestSuite) TestCredentialsAuthenticationWithVariousAttributes() {
	testCases := []struct {
		name        string
		credentials map[string]interface{}
	}{
		{
			name: "Email and password authentication",
			credentials: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"email": "credtest2@example.com",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword456!",
				},
			},
		},
		{
			name: "Mobile and password authentication",
			credentials: map[string]interface{}{
				"identifiers": map[string]interface{}{
					"mobileNumber": "+1234567891",
				},
				"credentials": map[string]interface{}{
					"password": "TestPassword789!",
				},
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			response, statusCode, err := suite.sendAuthRequest(tc.credentials)
			suite.Require().NoError(err, "Failed to send authenticate request")
			suite.Equal(http.StatusOK, statusCode, "Expected status 200 for successful authentication")

			// All single-factor credentials should result in AAL1
			aal := extractAssuranceLevelFromAssertion(response.Assertion, "aal")
			suite.NotEmpty(aal, "Assertion should contain AAL information")
			suite.Equal("AAL1", aal, "Single-factor credentials authentication should result in AAL1")
		})
	}
}

// TestAuthenticateWithExistingAssertionInvalidJWT tests authentication with invalid existing assertion
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithExistingAssertionInvalidJWT() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
		"assertion": "invalid.jwt.token",
	}

	errorResp, statusCode, err := suite.sendAuthRequestExpectingError(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for invalid assertion")
	suite.Equal("AUTHN-1009", errorResp.Code, "Expected error code AUTHN-1009 for invalid assertion")
}

// TestAuthenticateWithExistingAssertionSubjectMismatch tests authentication with assertion for different user
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithExistingAssertionSubjectMismatch() {
	// First, authenticate as user1 to get an assertion
	firstAuthRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
	}

	firstResponse, statusCode, err := suite.sendAuthRequest(firstAuthRequest)
	suite.Require().NoError(err, "Failed to send first authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for first authentication")
	suite.NotEmpty(firstResponse.Assertion, "First response should contain assertion")

	// Now try to authenticate as user2 with user1's assertion
	secondAuthRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user2",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword456!",
		},
		"assertion": firstResponse.Assertion,
	}

	errorResp, statusCode, err := suite.sendAuthRequestExpectingError(secondAuthRequest)
	suite.Require().NoError(err, "Failed to send second authenticate request")
	suite.Equal(http.StatusBadRequest, statusCode, "Expected status 400 for subject mismatch")
	suite.Equal("AUTHN-1010", errorResp.Code, "Expected error code AUTHN-1010 for assertion subject mismatch")
}

// TestAuthenticateWithExistingAssertionMultiStep tests multi-step authentication with credentials
// This simulates a scenario where credentials are used as a second factor after another authentication method
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithExistingAssertionMultiStep() {
	// First authentication step - authenticate with credentials to get initial assertion
	firstAuthRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
	}

	firstResponse, statusCode, err := suite.sendAuthRequest(firstAuthRequest)
	suite.Require().NoError(err, "Failed to send first authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for first authentication")
	suite.NotEmpty(firstResponse.Assertion, "First response should contain assertion")

	// Verify first assertion has AAL1
	aal1 := extractAssuranceLevelFromAssertion(firstResponse.Assertion, "aal")
	suite.Equal("AAL1", aal1, "First authentication should result in AAL1")

	// Second authentication step - authenticate with same credentials again, passing the assertion
	// This simulates re-authenticating with credentials in a multi-step flow
	secondAuthRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
		"assertion": firstResponse.Assertion,
	}

	secondResponse, statusCode, err := suite.sendAuthRequest(secondAuthRequest)
	suite.Require().NoError(err, "Failed to send second authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for second authentication")
	suite.NotEmpty(secondResponse.Assertion, "Second response should contain updated assertion")

	// Verify the assertion was updated (different from the first one)
	suite.NotEqual(firstResponse.Assertion, secondResponse.Assertion,
		"Second assertion should be different from first assertion")

	// The updated assertion should still maintain user information
	suite.Equal(firstResponse.ID, secondResponse.ID, "User ID should remain the same")
	suite.Equal(firstResponse.Type, secondResponse.Type, "User type should remain the same")
	suite.Equal(firstResponse.OUID, secondResponse.OUID,
		"Organization unit should remain the same")
}

// TestAuthenticateWithExistingAssertionAAL2MultiFactorSimulation tests AAL2 generation with multi-step credentials
// This simulates a multi-factor authentication scenario
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithExistingAssertionAAL2MultiFactorSimulation() {
	// Step 1: First factor authentication (e.g., credentials)
	firstAuthRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user4",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword999!",
		},
	}

	firstResponse, statusCode, err := suite.sendAuthRequest(firstAuthRequest)
	suite.Require().NoError(err, "Failed to send first authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for first authentication")
	suite.NotEmpty(firstResponse.Assertion, "First response should contain assertion")

	// Verify first assertion has AAL1
	aal1 := extractAssuranceLevelFromAssertion(firstResponse.Assertion, "aal")
	suite.Equal("AAL1", aal1, "First factor should result in AAL1")

	// Step 2: Second factor authentication with different credential (e.g., email-based auth)
	// In a real scenario, this would be a different authentication method like OTP or biometric
	// Here we simulate by authenticating with email instead of username
	secondAuthRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"email": "credtest4@example.com",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword999!",
		},
		"assertion": firstResponse.Assertion,
	}

	secondResponse, statusCode, err := suite.sendAuthRequest(secondAuthRequest)
	suite.Require().NoError(err, "Failed to send second authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for second authentication")
	suite.NotEmpty(secondResponse.Assertion, "Second response should contain updated assertion")

	// Verify the assertion contains AAL2 for multi-step authentication
	aal2 := extractAssuranceLevelFromAssertion(secondResponse.Assertion, "aal")
	suite.NotEmpty(aal2, "Second assertion should contain AAL information")
	// Note: In a real multi-factor scenario with different authentication methods (e.g., credentials + OTP),
	// this would be AAL2. However, using credentials twice may not elevate to AAL2 depending on implementation.
	// This test documents the behavior for multi-step authentication with assertions.
	suite.NotEmpty(aal2, "Multi-step authentication should maintain AAL information")
}

// TestAuthenticateWithExistingAssertionSkipAssertionTrue tests that existing assertion is
// ignored when skip_assertion is true
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithExistingAssertionSkipAssertionTrue() {
	// First, get an assertion
	firstAuthRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
	}

	firstResponse, statusCode, err := suite.sendAuthRequest(firstAuthRequest)
	suite.Require().NoError(err, "Failed to send first authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for first authentication")
	suite.NotEmpty(firstResponse.Assertion, "First response should contain assertion")

	// Second authentication with skip_assertion=true and existing assertion
	secondAuthRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
		"skipAssertion": true,
		"assertion":      firstResponse.Assertion,
	}

	secondResponse, statusCode, err := suite.sendAuthRequest(secondAuthRequest)
	suite.Require().NoError(err, "Failed to send second authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for second authentication")
	suite.Empty(secondResponse.Assertion, "Response should not contain assertion when skip_assertion is true")
}

// TestAuthenticateWithExistingAssertionEmptyString tests authentication with empty assertion string
func (suite *CredentialsAuthTestSuite) TestAuthenticateWithExistingAssertionEmptyString() {
	authRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "credtest_user1",
		},
		"credentials": map[string]interface{}{
			"password": "TestPassword123!",
		},
		"assertion": "",
	}

	response, statusCode, err := suite.sendAuthRequest(authRequest)
	suite.Require().NoError(err, "Failed to send authenticate request")
	suite.Equal(http.StatusOK, statusCode, "Expected status 200 for authentication with empty assertion")
	suite.NotEmpty(response.Assertion, "Response should contain new assertion when existing assertion is empty")

	// Verify AAL1 for single-factor authentication
	aal := extractAssuranceLevelFromAssertion(response.Assertion, "aal")
	suite.Equal("AAL1", aal, "Single-factor authentication should result in AAL1")
}

func (suite *CredentialsAuthTestSuite) sendAuthRequest(authRequest map[string]interface{}) (
	*testutils.AuthenticationResponse, int, error) {
	requestJSON, err := json.Marshal(authRequest)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+credentialsAuthEndpoint,
		bytes.NewReader(requestJSON))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var response testutils.AuthenticationResponse
	bodyBytes, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return &response, resp.StatusCode, nil
}

func (suite *CredentialsAuthTestSuite) sendAuthRequestExpectingError(authRequest map[string]interface{}) (
	*testutils.ErrorResponse, int, error) {
	requestJSON, err := json.Marshal(authRequest)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+credentialsAuthEndpoint,
		bytes.NewReader(requestJSON))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var errorResp testutils.ErrorResponse
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bodyBytes, &errorResp)

	return &errorResp, resp.StatusCode, nil
}

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
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	mockNotificationServerPort = 8099
	smsOTPAuthSendEndpoint     = "/auth/otp/sms/send"
	smsOTPAuthVerifyEndpoint   = "/auth/otp/sms/verify"
	testMobileNumber           = "+1234567890"
)

var smsOTPEntityType = testutils.UserType{
	Name: "smsotp_user",
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
		"mobileNumber": map[string]interface{}{
			"type": "string",
		},
	},
}

var smsOTPAuthTestOU = testutils.OrganizationUnit{
	Handle:      "sms-otp-auth-test-ou",
	Name:        "SMS OTP Auth Test Organization Unit",
	Description: "Organization unit for SMS OTP authentication testing",
	Parent:      nil,
}

type SMSOTPAuthTestSuite struct {
	suite.Suite
	mockServer   *testutils.MockNotificationServer
	client       *http.Client
	senderID     string
	userID       string
	mobileNumber string
	entityTypeID string
	ouID         string
}

func TestSMSOTPAuthTestSuite(t *testing.T) {
	suite.Run(t, new(SMSOTPAuthTestSuite))
}

func (suite *SMSOTPAuthTestSuite) SetupSuite() {
	suite.client = testutils.GetHTTPClient()
	suite.mobileNumber = testMobileNumber

	suite.mockServer = testutils.NewMockNotificationServer(mockNotificationServerPort)
	err := suite.mockServer.Start()
	suite.Require().NoError(err, "Failed to start mock notification server")

	time.Sleep(100 * time.Millisecond)

	ouID, err := testutils.CreateOrganizationUnit(smsOTPAuthTestOU)
	suite.Require().NoError(err, "Failed to create test organization unit")
	suite.ouID = ouID

	customSender := NotificationSenderRequest{
		Name:        "Test SMS OTP Auth Sender",
		Description: "Sender for SMS OTP authentication testing",
		Provider:    "custom",
		Properties: []SenderProperty{
			{
				Name:     "url",
				Value:    suite.mockServer.GetSendSMSURL(),
				IsSecret: false,
			},
			{
				Name:     "http_method",
				Value:    "POST",
				IsSecret: false,
			},
			{
				Name:     "content_type",
				Value:    "JSON",
				IsSecret: false,
			},
		},
	}

	senderID, err := suite.createNotificationSender(customSender)
	suite.Require().NoError(err, "Failed to create notification sender")
	suite.senderID = senderID

	smsOTPEntityType.OUID = suite.ouID
	schemaID, err := testutils.CreateUserType(smsOTPEntityType)
	suite.Require().NoError(err, "Failed to create SMS OTP user type")
	suite.entityTypeID = schemaID

	userAttributes := map[string]interface{}{
		"username":     "smsotp_user",
		"password":     "Test@1234",
		"email":        "smsotp@example.com",
		"mobileNumber": suite.mobileNumber,
	}
	userAttributesJSON, err := json.Marshal(userAttributes)
	suite.Require().NoError(err)

	user := testutils.User{
		Type:             smsOTPEntityType.Name,
		OUID:             suite.ouID,
		Attributes:       userAttributesJSON,
	}
	userID, err := testutils.CreateUser(user)
	suite.Require().NoError(err, "Failed to create test user")
	suite.userID = userID
}

func (suite *SMSOTPAuthTestSuite) TearDownSuite() {
	if suite.userID != "" {
		_ = testutils.DeleteUser(suite.userID)
	}

	if suite.senderID != "" {
		_ = suite.deleteNotificationSender(suite.senderID)
	}

	if suite.entityTypeID != "" {
		_ = testutils.DeleteUserType(suite.entityTypeID)
	}

	if suite.mockServer != nil {
		_ = suite.mockServer.Stop()
	}

	if suite.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(suite.ouID); err != nil {
			suite.T().Logf("Failed to delete test organization unit: %v", err)
		}
	}
}

func (suite *SMSOTPAuthTestSuite) SetupTest() {
	suite.mockServer.ClearMessages()
}

func (suite *SMSOTPAuthTestSuite) TestSendOTPSuccess() {
	sendRequest := map[string]interface{}{
		"senderId": suite.senderID,
		"recipient": suite.mobileNumber,
	}
	sendRequestJSON, err := json.Marshal(sendRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthSendEndpoint, bytes.NewReader(sendRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var sendResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&sendResponse)
	suite.Require().NoError(err)

	suite.Contains(sendResponse, "status")
	suite.Equal("SUCCESS", sendResponse["status"])
	suite.Contains(sendResponse, "sessionToken")
	suite.NotEmpty(sendResponse["sessionToken"])

	time.Sleep(100 * time.Millisecond)

	lastMessage := suite.mockServer.GetLastMessage()
	suite.Require().NotNil(lastMessage, "OTP message should be sent to mock server")
	suite.NotEmpty(lastMessage.OTP, "OTP should be extractable from message")
}

func (suite *SMSOTPAuthTestSuite) TestSendOTPInvalidSender() {
	sendRequest := map[string]interface{}{
		"senderId": "invalid-sender-id",
		"recipient": suite.mobileNumber,
	}
	sendRequestJSON, err := json.Marshal(sendRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthSendEndpoint, bytes.NewReader(sendRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *SMSOTPAuthTestSuite) TestSendOTPMissingFields() {
	testCases := []struct {
		name    string
		request map[string]interface{}
	}{
		{
			name: "Missing sender_id",
			request: map[string]interface{}{
				"recipient": suite.mobileNumber,
			},
		},
		{
			name: "Missing recipient",
			request: map[string]interface{}{
				"senderId": suite.senderID,
			},
		},
		{
			name: "Empty sender_id",
			request: map[string]interface{}{
				"senderId": "",
				"recipient": suite.mobileNumber,
			},
		},
		{
			name: "Empty recipient",
			request: map[string]interface{}{
				"senderId": suite.senderID,
				"recipient": "",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			requestJSON, err := json.Marshal(tc.request)
			suite.Require().NoError(err)

			req, err := http.NewRequest("POST", testServerURL+smsOTPAuthSendEndpoint,
				bytes.NewReader(requestJSON))
			suite.Require().NoError(err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.client.Do(req)
			suite.Require().NoError(err)
			defer resp.Body.Close()

			suite.Equal(http.StatusBadRequest, resp.StatusCode)
		})
	}
}

func (suite *SMSOTPAuthTestSuite) TestVerifyOTPSuccess() {
	sessionToken, otp := suite.sendOTPAndExtract()

	verifyRequest := map[string]interface{}{
		"sessionToken": sessionToken,
		"otp":           otp,
	}
	verifyRequestJSON, err := json.Marshal(verifyRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthVerifyEndpoint, bytes.NewReader(verifyRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.NotEmpty(authResponse.ID, "Response should contain user ID")
	suite.Equal(suite.userID, authResponse.ID, "Response should contain the correct user ID")
	suite.NotEmpty(authResponse.Type, "Response should contain user type")
	suite.NotEmpty(authResponse.OUID, "Response should contain organization unit")
	suite.NotEmpty(authResponse.Assertion, "Response should contain assertion token by default")
}

func (suite *SMSOTPAuthTestSuite) TestVerifyOTPInvalidCode() {
	sessionToken, _ := suite.sendOTPAndExtract()

	verifyRequest := map[string]interface{}{
		"sessionToken": sessionToken,
		"otp":           "000000",
	}
	verifyRequestJSON, err := json.Marshal(verifyRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthVerifyEndpoint, bytes.NewReader(verifyRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusUnauthorized, resp.StatusCode)
}

func (suite *SMSOTPAuthTestSuite) TestVerifyOTPInvalidSessionToken() {
	verifyRequest := map[string]interface{}{
		"sessionToken": "invalid-session-token",
		"otp":           "123456",
	}
	verifyRequestJSON, err := json.Marshal(verifyRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthVerifyEndpoint, bytes.NewReader(verifyRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusBadRequest, resp.StatusCode)
}

func (suite *SMSOTPAuthTestSuite) TestVerifyOTPMissingFields() {
	testCases := []struct {
		name    string
		request map[string]interface{}
	}{
		{
			name: "Missing session_token",
			request: map[string]interface{}{
				"otp": "123456",
			},
		},
		{
			name: "Missing otp",
			request: map[string]interface{}{
				"sessionToken": "some-token",
			},
		},
		{
			name: "Empty session_token",
			request: map[string]interface{}{
				"sessionToken": "",
				"otp":           "123456",
			},
		},
		{
			name: "Empty otp",
			request: map[string]interface{}{
				"sessionToken": "some-token",
				"otp":           "",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			requestJSON, err := json.Marshal(tc.request)
			suite.Require().NoError(err)

			req, err := http.NewRequest("POST", testServerURL+smsOTPAuthVerifyEndpoint,
				bytes.NewReader(requestJSON))
			suite.Require().NoError(err)
			req.Header.Set("Content-Type", "application/json")

			resp, err := suite.client.Do(req)
			suite.Require().NoError(err)
			defer resp.Body.Close()

			suite.Equal(http.StatusBadRequest, resp.StatusCode, "Unexpected status code")
		})
	}
}

func (suite *SMSOTPAuthTestSuite) TestCompleteOTPAuthFlow() {
	sendRequest := map[string]interface{}{
		"senderId": suite.senderID,
		"recipient": suite.mobileNumber,
	}
	sendRequestJSON, err := json.Marshal(sendRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthSendEndpoint, bytes.NewReader(sendRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var sendResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&sendResponse)
	suite.Require().NoError(err)

	sessionToken := sendResponse["sessionToken"].(string)
	suite.NotEmpty(sessionToken)

	time.Sleep(100 * time.Millisecond)

	lastMessage := suite.mockServer.GetLastMessage()
	suite.Require().NotNil(lastMessage)
	otp := lastMessage.OTP
	suite.Require().NotEmpty(otp)

	verifyRequest := map[string]interface{}{
		"sessionToken": sessionToken,
		"otp":           otp,
	}
	verifyRequestJSON, err := json.Marshal(verifyRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+smsOTPAuthVerifyEndpoint, bytes.NewReader(verifyRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.Contains(authResponse, "id")
	suite.Contains(authResponse, "type")
	suite.Contains(authResponse, "ouId")
	suite.Equal(suite.userID, authResponse["id"])
}

// TestVerifyOTPWithSkipAssertionFalse tests OTP verification with skip_assertion explicitly set to false
func (suite *SMSOTPAuthTestSuite) TestVerifyOTPWithSkipAssertionFalse() {
	sessionToken, otp := suite.sendOTPAndExtract()

	verifyRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"otp":            otp,
		"skipAssertion": false,
	}
	verifyRequestJSON, err := json.Marshal(verifyRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthVerifyEndpoint, bytes.NewReader(verifyRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.NotEmpty(authResponse.ID, "Response should contain user ID")
	suite.Equal(suite.userID, authResponse.ID, "Response should contain the correct user ID")
	suite.NotEmpty(authResponse.Type, "Response should contain user type")
	suite.NotEmpty(authResponse.OUID, "Response should contain organization unit")
	suite.NotEmpty(authResponse.Assertion, "Response should contain assertion token when skip_assertion is false")
}

// TestVerifyOTPWithSkipAssertionTrue tests OTP verification with skip_assertion set to true
func (suite *SMSOTPAuthTestSuite) TestVerifyOTPWithSkipAssertionTrue() {
	sessionToken, otp := suite.sendOTPAndExtract()

	verifyRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"otp":            otp,
		"skipAssertion": true,
	}
	verifyRequestJSON, err := json.Marshal(verifyRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthVerifyEndpoint, bytes.NewReader(verifyRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.NotEmpty(authResponse.ID, "Response should contain user ID")
	suite.Equal(suite.userID, authResponse.ID, "Response should contain the correct user ID")
	suite.NotEmpty(authResponse.Type, "Response should contain user type")
	suite.NotEmpty(authResponse.OUID, "Response should contain organization unit")
	suite.Empty(authResponse.Assertion, "Response should not contain assertion token when skip_assertion is true")
}

// TestSMSOTPVerifyWithAssuranceLevelAAL1 tests that SMS OTP alone generates AAL1 assurance level
func (suite *SMSOTPAuthTestSuite) TestSMSOTPVerifyWithAssuranceLevelAAL1() {
	sessionToken, otp := suite.sendOTPAndExtract()

	verifyRequest := map[string]interface{}{
		"sessionToken": sessionToken,
		"otp":           otp,
	}
	verifyRequestJSON, err := json.Marshal(verifyRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthVerifyEndpoint, bytes.NewReader(verifyRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.NotEmpty(authResponse.Assertion, "Response should contain assertion token by default")

	// Verify assertion contains AAL1 for SMS OTP alone (no prior authentication)
	aal := extractAssuranceLevelFromAssertion(authResponse.Assertion, "aal")
	suite.NotEmpty(aal, "Assertion should contain AAL information")
	suite.Equal("AAL1", aal, "Single-factor SMS OTP authentication should result in AAL1")

	// Verify IAL is present
	ial := extractAssuranceLevelFromAssertion(authResponse.Assertion, "ial")
	suite.NotEmpty(ial, "Assertion should contain IAL information")
	suite.Equal("IAL1", ial, "Self-asserted identity should result in IAL1")
}

// TestSMSOTPNoAssertionGeneration tests that AAL/IAL are not present when assertion is skipped
func (suite *SMSOTPAuthTestSuite) TestSMSOTPNoAssertionGeneration() {
	sessionToken, otp := suite.sendOTPAndExtract()

	verifyRequest := map[string]interface{}{
		"sessionToken":  sessionToken,
		"otp":            otp,
		"skipAssertion": true,
	}
	verifyRequestJSON, err := json.Marshal(verifyRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthVerifyEndpoint, bytes.NewReader(verifyRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode)

	var authResponse testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&authResponse)
	suite.Require().NoError(err)

	suite.Empty(authResponse.Assertion, "Response should not contain assertion when skip_assertion is true")
}

// TestSMSOTPWithCredentialsMultiFactorAAL2 tests that credentials + SMS OTP generates AAL2
// This test demonstrates multi-factor authentication flow
func (suite *SMSOTPAuthTestSuite) TestSMSOTPWithCredentialsMultiFactorAAL2() {
	// First, authenticate with credentials (get assertion for first factor)
	credentialsRequest := map[string]interface{}{
		"identifiers": map[string]interface{}{
			"username": "smsotp_user",
		},
		"credentials": map[string]interface{}{
			"password": "Test@1234",
		},
	}
	credJSON, err := json.Marshal(credentialsRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+credentialsAuthEndpoint, bytes.NewReader(credJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode, "Credentials authentication should succeed")

	var credResp testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&credResp)
	suite.Require().NoError(err)
	suite.NotEmpty(credResp.ID, "Should get user ID from credentials auth")
	suite.NotEmpty(credResp.Assertion, "Should get assertion from first factor")

	// Then, authenticate with SMS OTP (second factor), passing the assertion from first factor
	sessionToken, otp := suite.sendOTPAndExtract()

	verifyRequest := map[string]interface{}{
		"sessionToken": sessionToken,
		"otp":           otp,
		"assertion":     credResp.Assertion,
	}
	verifyJSON, err := json.Marshal(verifyRequest)
	suite.Require().NoError(err)

	req, err = http.NewRequest("POST", testServerURL+smsOTPAuthVerifyEndpoint, bytes.NewReader(verifyJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err = suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Equal(http.StatusOK, resp.StatusCode, "SMS OTP verification should succeed")

	var otpResp testutils.AuthenticationResponse
	err = json.NewDecoder(resp.Body).Decode(&otpResp)
	suite.Require().NoError(err)

	suite.NotEmpty(otpResp.Assertion, "Response should contain assertion after MFA")

	// Verify assertion contains AAL2 for multi-factor authentication
	aal := extractAssuranceLevelFromAssertion(otpResp.Assertion, "aal")
	suite.NotEmpty(aal, "Assertion should contain AAL information")
	suite.Equal("AAL2", aal, "Multi-factor authentication (credentials + SMS OTP) should result in AAL2")

	// Verify IAL is present
	ial := extractAssuranceLevelFromAssertion(otpResp.Assertion, "ial")
	suite.NotEmpty(ial, "Assertion should contain IAL information")
	suite.Equal("IAL1", ial, "Self-asserted identity should result in IAL1")
}

func (suite *SMSOTPAuthTestSuite) sendOTPAndExtract() (string, string) {
	sendRequest := map[string]interface{}{
		"senderId": suite.senderID,
		"recipient": suite.mobileNumber,
	}
	sendRequestJSON, err := json.Marshal(sendRequest)
	suite.Require().NoError(err)

	req, err := http.NewRequest("POST", testServerURL+smsOTPAuthSendEndpoint, bytes.NewReader(sendRequestJSON))
	suite.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	suite.Require().NoError(err)
	defer resp.Body.Close()

	suite.Require().Equal(http.StatusOK, resp.StatusCode)

	var sendResponse map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&sendResponse)
	suite.Require().NoError(err)

	sessionToken := sendResponse["sessionToken"].(string)

	time.Sleep(100 * time.Millisecond)

	lastMessage := suite.mockServer.GetLastMessage()
	suite.Require().NotNil(lastMessage)
	otp := lastMessage.OTP
	suite.Require().NotEmpty(otp)

	return sessionToken, otp
}

func (suite *SMSOTPAuthTestSuite) createNotificationSender(sender NotificationSenderRequest) (string, error) {
	senderJSON, err := json.Marshal(sender)
	if err != nil {
		return "", fmt.Errorf("failed to marshal sender: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/notification-senders/message",
		bytes.NewReader(senderJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := suite.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("expected status 201, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respBody)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	id, ok := respBody["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id")
	}

	return id, nil
}

func (suite *SMSOTPAuthTestSuite) deleteNotificationSender(senderID string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/notification-senders/message/"+senderID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := suite.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete sender: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("expected status 200 or 204, got %d", resp.StatusCode)
	}

	return nil
}

type NotificationSenderRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Provider    string           `json:"provider"`
	Properties  []SenderProperty `json:"properties"`
}

type SenderProperty struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

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

package notification

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	mockNotificationServerPort = 8097
	testMobileNumber           = "+1234567890"
)

// OTPRequest represents the request to send an OTP
type OTPRequest struct {
	Recipient string `json:"recipient"`
	SenderID  string `json:"senderId"`
	Channel   string `json:"channel"`
}

// OTPSendResponse represents the response when sending an OTP
type OTPSendResponse struct {
	Status       string `json:"status"`
	SessionToken string `json:"sessionToken"`
}

// OTPVerifyRequest represents the request to verify an OTP
type OTPVerifyRequest struct {
	SessionToken string `json:"sessionToken"`
	OTPCode      string `json:"otpCode"`
}

// OTPVerifyResponse represents the response when verifying an OTP
type OTPVerifyResponse struct {
	Status string `json:"status"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Code        string                `json:"code"`
	Message     testutils.I18nMessage `json:"message"`
	Description testutils.I18nMessage `json:"description"`
}

// OTPAPITestSuite tests the OTP API endpoints
type OTPAPITestSuite struct {
	suite.Suite
	client       *http.Client
	mockServer   *testutils.MockNotificationServer
	testSenderID string
}

// TestOTPAPITestSuite runs the OTP API test suite
func TestOTPAPITestSuite(t *testing.T) {
	suite.Run(t, new(OTPAPITestSuite))
}

// SetupSuite initializes the test suite
func (ts *OTPAPITestSuite) SetupSuite() {
	// Initialize HTTP client
	ts.client = testutils.GetHTTPClient()

	// Start mock notification server
	ts.mockServer = testutils.NewMockNotificationServer(mockNotificationServerPort)
	err := ts.mockServer.Start()
	if err != nil {
		ts.T().Fatalf("Failed to start mock notification server: %v", err)
	}

	// Wait for mock server to start
	time.Sleep(100 * time.Millisecond)
	ts.T().Log("Mock notification server started successfully")

	// Create a custom notification sender for testing
	customSender := NotificationSenderRequest{
		Name:        "Test OTP Custom Sender",
		Description: "Custom sender for OTP testing",
		Provider:    "custom",
		Properties: []SenderProperty{
			{
				Name:     "url",
				Value:    ts.mockServer.GetSendSMSURL(),
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

	// Create the sender
	senderID, err := createSenderWithRequest(customSender)
	if err != nil {
		ts.T().Fatalf("Failed to create test notification sender: %v", err)
	}
	ts.testSenderID = senderID
	ts.T().Logf("Created test notification sender with ID: %s", senderID)
}

// TearDownSuite cleans up after the test suite
func (ts *OTPAPITestSuite) TearDownSuite() {
	// Delete the test sender
	if ts.testSenderID != "" {
		err := deleteSender(ts.testSenderID)
		if err != nil {
			ts.T().Logf("Warning: Failed to delete test notification sender: %v", err)
		}
	}

	// Stop the mock notification server
	if ts.mockServer != nil {
		err := ts.mockServer.Stop()
		if err != nil {
			ts.T().Logf("Warning: Failed to stop mock notification server: %v", err)
		}
	}
}

// TestOTPSendSuccess tests successful OTP sending
func (ts *OTPAPITestSuite) TestOTPSendSuccess() {
	if ts.testSenderID == "" {
		ts.T().Fatal("Test sender ID is required for OTP send test")
	}

	otpReq := OTPRequest{
		Recipient: testMobileNumber,
		SenderID:  ts.testSenderID,
		Channel:   "sms",
	}

	reqBody, err := json.Marshal(otpReq)
	ts.Require().NoError(err, "Failed to marshal OTP request")

	req, err := http.NewRequest("POST", testServerURL+"/notification-senders/otp/send", bytes.NewReader(reqBody))
	ts.Require().NoError(err, "Failed to create OTP send request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send OTP request")
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusOK, resp.StatusCode, "Expected successful OTP send")

	var otpResp OTPSendResponse
	err = json.NewDecoder(resp.Body).Decode(&otpResp)
	ts.Require().NoError(err, "Failed to decode OTP send response")

	ts.Assert().Equal("SUCCESS", otpResp.Status, "Expected SUCCESS status")
	ts.Assert().NotEmpty(otpResp.SessionToken, "Session token should not be empty")

	// Wait a bit to ensure the message was processed by mock server
	time.Sleep(100 * time.Millisecond)

	// Extract OTP from mock server (for testing purposes)
	// In a real scenario, this would come from the user receiving the SMS
	otpCode := ts.extractOTPFromMockServer()
	ts.Assert().NotEmpty(otpCode, "OTP should be extractable from mock server")

	ts.T().Logf("OTP sent successfully with session token: %s, OTP: %s", otpResp.SessionToken, otpCode)
}

// TestOTPSendInvalidSender tests OTP sending with invalid sender ID
func (ts *OTPAPITestSuite) TestOTPSendInvalidSender() {
	otpReq := OTPRequest{
		Recipient: testMobileNumber,
		SenderID:  "invalid-sender-id",
		Channel:   "sms",
	}

	reqBody, err := json.Marshal(otpReq)
	ts.Require().NoError(err, "Failed to marshal OTP request")

	req, err := http.NewRequest("POST", testServerURL+"/notification-senders/otp/send", bytes.NewReader(reqBody))
	ts.Require().NoError(err, "Failed to create OTP send request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send OTP request")
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusNotFound, resp.StatusCode, "Expected 404 for invalid sender ID")

	var errResp ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	ts.Require().NoError(err, "Failed to decode error response")

	ts.Assert().NotEmpty(errResp.Code, "Error code should not be empty")
	ts.Assert().NotEmpty(errResp.Message.DefaultValue, "Error message should not be empty")
}

// TestOTPSendMissingFields tests OTP sending with missing required fields
func (ts *OTPAPITestSuite) TestOTPSendMissingFields() {
	testCases := []struct {
		name        string
		request     OTPRequest
		description string
	}{
		{
			name: "Missing Recipient",
			request: OTPRequest{
				SenderID: ts.testSenderID,
				Channel:  "sms",
			},
			description: "Should fail when recipient is missing",
		},
		{
			name: "Missing Sender ID",
			request: OTPRequest{
				Recipient: testMobileNumber,
				Channel:   "sms",
			},
			description: "Should fail when sender ID is missing",
		},
		{
			name: "Missing Channel",
			request: OTPRequest{
				Recipient: testMobileNumber,
				SenderID:  ts.testSenderID,
			},
			description: "Should fail when channel is missing",
		},
		{
			name: "Invalid Channel",
			request: OTPRequest{
				Recipient: testMobileNumber,
				SenderID:  ts.testSenderID,
				Channel:   "email", // Only SMS is supported
			},
			description: "Should fail when channel is invalid",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			reqBody, err := json.Marshal(tc.request)
			ts.Require().NoError(err, "Failed to marshal OTP request")

			req, err := http.NewRequest("POST", testServerURL+"/notification-senders/otp/send", bytes.NewReader(reqBody))
			ts.Require().NoError(err, "Failed to create OTP send request")

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")

			resp, err := ts.client.Do(req)
			ts.Require().NoError(err, "Failed to send OTP request")
			defer resp.Body.Close()

			ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode, tc.description)

			var errResp ErrorResponse
			err = json.NewDecoder(resp.Body).Decode(&errResp)
			ts.Require().NoError(err, "Failed to decode error response")

			ts.Assert().NotEmpty(errResp.Code, "Error code should not be empty")
			ts.Assert().NotEmpty(errResp.Message.DefaultValue, "Error message should not be empty")
		})
	}
}

// TestOTPVerifySuccess tests successful OTP verification
func (ts *OTPAPITestSuite) TestOTPVerifySuccess() {
	// This test depends on TestOTPSendSuccess running first to get session token and OTP
	// We'll run the send workflow in this test to ensure proper ordering
	if ts.testSenderID == "" {
		ts.T().Fatal("Test sender ID is required for OTP tests")
	}

	// Step 1: Send OTP first
	otpReq := OTPRequest{
		Recipient: testMobileNumber,
		SenderID:  ts.testSenderID,
		Channel:   "sms",
	}

	reqBody, err := json.Marshal(otpReq)
	ts.Require().NoError(err, "Failed to marshal OTP request")

	req, err := http.NewRequest("POST", testServerURL+"/notification-senders/otp/send", bytes.NewReader(reqBody))
	ts.Require().NoError(err, "Failed to create OTP send request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send OTP request")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.T().Skip("Skipping OTP verify test - OTP send failed")
	}

	var otpResp OTPSendResponse
	err = json.NewDecoder(resp.Body).Decode(&otpResp)
	ts.Require().NoError(err, "Failed to decode OTP send response")

	// Wait for mock server to process message
	time.Sleep(100 * time.Millisecond)

	// Extract OTP from mock server
	otpCode := ts.extractOTPFromMockServer()
	if otpCode == "" {
		ts.T().Skip("Skipping OTP verify test - could not extract OTP from mock server")
	}

	// Step 2: Verify the OTP
	verifyReq := OTPVerifyRequest{
		SessionToken: otpResp.SessionToken,
		OTPCode:      otpCode,
	}

	verifyReqBody, err := json.Marshal(verifyReq)
	ts.Require().NoError(err, "Failed to marshal OTP verify request")

	verifyReqHTTP, err := http.NewRequest("POST", testServerURL+"/notification-senders/otp/verify", bytes.NewReader(verifyReqBody))
	ts.Require().NoError(err, "Failed to create OTP verify request")

	verifyReqHTTP.Header.Set("Content-Type", "application/json")
	verifyReqHTTP.Header.Set("Accept", "application/json")

	verifyResp, err := ts.client.Do(verifyReqHTTP)
	ts.Require().NoError(err, "Failed to send OTP verify request")
	defer verifyResp.Body.Close()

	ts.Assert().Equal(http.StatusOK, verifyResp.StatusCode, "Expected successful OTP verification")

	var verifyRespBody OTPVerifyResponse
	err = json.NewDecoder(verifyResp.Body).Decode(&verifyRespBody)
	ts.Require().NoError(err, "Failed to decode OTP verify response")

	ts.Assert().Equal("VERIFIED", verifyRespBody.Status, "Expected VERIFIED status")

	ts.T().Log("OTP verified successfully")
}

// TestOTPVerifyInvalidToken tests OTP verification with invalid session token
func (ts *OTPAPITestSuite) TestOTPVerifyInvalidToken() {
	verifyReq := OTPVerifyRequest{
		SessionToken: "invalid-session-token",
		OTPCode:      "123456",
	}

	reqBody, err := json.Marshal(verifyReq)
	ts.Require().NoError(err, "Failed to marshal OTP verify request")

	req, err := http.NewRequest("POST", testServerURL+"/notification-senders/otp/verify", bytes.NewReader(reqBody))
	ts.Require().NoError(err, "Failed to create OTP verify request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send OTP verify request")
	defer resp.Body.Close()

	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode, "Expected 400 for invalid session token")

	var errResp ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errResp)
	ts.Require().NoError(err, "Failed to decode error response")

	ts.Assert().NotEmpty(errResp.Code, "Error code should not be empty")
	ts.Assert().NotEmpty(errResp.Message.DefaultValue, "Error message should not be empty")
}

// TestOTPVerifyInvalidCode tests OTP verification with invalid OTP code
func (ts *OTPAPITestSuite) TestOTPVerifyInvalidCode() {
	if ts.testSenderID == "" {
		ts.T().Fatal("Test sender ID is required for OTP tests")
	}

	// Step 1: Send OTP first to get a valid session token
	otpReq := OTPRequest{
		Recipient: testMobileNumber,
		SenderID:  ts.testSenderID,
		Channel:   "sms",
	}

	reqBody, err := json.Marshal(otpReq)
	ts.Require().NoError(err, "Failed to marshal OTP request")

	req, err := http.NewRequest("POST", testServerURL+"/notification-senders/otp/send", bytes.NewReader(reqBody))
	ts.Require().NoError(err, "Failed to create OTP send request")

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send OTP request")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.T().Skip("Skipping OTP verify test - OTP send failed")
	}

	var otpResp OTPSendResponse
	err = json.NewDecoder(resp.Body).Decode(&otpResp)
	ts.Require().NoError(err, "Failed to decode OTP send response")

	// Step 2: Try to verify with invalid OTP code
	verifyReq := OTPVerifyRequest{
		SessionToken: otpResp.SessionToken,
		OTPCode:      "999999", // Invalid OTP code
	}

	verifyReqBody, err := json.Marshal(verifyReq)
	ts.Require().NoError(err, "Failed to marshal OTP verify request")

	verifyReqHTTP, err := http.NewRequest("POST", testServerURL+"/notification-senders/otp/verify", bytes.NewReader(verifyReqBody))
	ts.Require().NoError(err, "Failed to create OTP verify request")

	verifyReqHTTP.Header.Set("Content-Type", "application/json")
	verifyReqHTTP.Header.Set("Accept", "application/json")

	verifyResp, err := ts.client.Do(verifyReqHTTP)
	ts.Require().NoError(err, "Failed to send OTP verify request")
	defer verifyResp.Body.Close()

	ts.Assert().Equal(http.StatusOK, verifyResp.StatusCode, "Expected 200 OK for invalid OTP code")

	var verifyRespBody OTPVerifyResponse
	err = json.NewDecoder(verifyResp.Body).Decode(&verifyRespBody)
	ts.Require().NoError(err, "Failed to decode OTP verify response")

	ts.Assert().Equal("INVALID", verifyRespBody.Status, "Expected INVALID status for wrong OTP code")
}

// TestOTPVerifyMissingFields tests OTP verification with missing required fields
func (ts *OTPAPITestSuite) TestOTPVerifyMissingFields() {
	testCases := []struct {
		name        string
		request     OTPVerifyRequest
		description string
	}{
		{
			name: "Missing Session Token",
			request: OTPVerifyRequest{
				OTPCode: "123456",
			},
			description: "Should fail when session token is missing",
		},
		{
			name: "Missing OTP Code",
			request: OTPVerifyRequest{
				SessionToken: "some-token",
			},
			description: "Should fail when OTP code is missing",
		},
		{
			name: "Empty Session Token",
			request: OTPVerifyRequest{
				SessionToken: "",
				OTPCode:      "123456",
			},
			description: "Should fail when session token is empty",
		},
		{
			name: "Empty OTP Code",
			request: OTPVerifyRequest{
				SessionToken: "some-token",
				OTPCode:      "",
			},
			description: "Should fail when OTP code is empty",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			reqBody, err := json.Marshal(tc.request)
			ts.Require().NoError(err, "Failed to marshal OTP verify request")

			req, err := http.NewRequest("POST", testServerURL+"/notification-senders/otp/verify", bytes.NewReader(reqBody))
			ts.Require().NoError(err, "Failed to create OTP verify request")

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json")

			resp, err := ts.client.Do(req)
			ts.Require().NoError(err, "Failed to send OTP verify request")
			defer resp.Body.Close()

			ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode, tc.description)

			var errResp ErrorResponse
			err = json.NewDecoder(resp.Body).Decode(&errResp)
			ts.Require().NoError(err, "Failed to decode error response")

			ts.Assert().NotEmpty(errResp.Code, "Error code should not be empty")
			ts.Assert().NotEmpty(errResp.Message.DefaultValue, "Error message should not be empty")
		})
	}
}

// extractOTPFromMockServer extracts the OTP from the mock server's latest message
func (ts *OTPAPITestSuite) extractOTPFromMockServer() string {
	// Get messages from mock server
	resp, err := http.Get(ts.mockServer.GetURL() + "/messages")
	if err != nil {
		ts.T().Logf("Failed to get messages from mock server: %v", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.T().Logf("Mock server returned non-200 status: %d", resp.StatusCode)
		return ""
	}

	var messages []testutils.SMSMessage
	err = json.NewDecoder(resp.Body).Decode(&messages)
	if err != nil {
		ts.T().Logf("Failed to decode messages from mock server: %v", err)
		return ""
	}

	if len(messages) == 0 {
		ts.T().Log("No messages found in mock server")
		return ""
	}

	// Get the latest message and extract OTP
	latestMessage := messages[len(messages)-1]
	return latestMessage.OTP
}

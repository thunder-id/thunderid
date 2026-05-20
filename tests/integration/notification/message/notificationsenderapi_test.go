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
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	testServerURL = "https://localhost:8095"
)

// SenderTestData defines a data provider structure for testing different sender types
type SenderTestData struct {
	Name        string
	Description string
	Provider    string
	Properties  []SenderProperty
}

// Sender test data for different providers
var (
	// Vonage test data
	vonageData = SenderTestData{
		Name:        "Test Vonage",
		Description: "Test Vonage notification sender",
		Provider:    "vonage",
		Properties: []SenderProperty{
			{
				Name:     "api_key",
				Value:    "test-api-key",
				IsSecret: true,
			},
			{
				Name:     "api_secret",
				Value:    "test-api-secret",
				IsSecret: true,
			},
			{
				Name:     "sender_id",
				Value:    "TestSender",
				IsSecret: false,
			},
		},
	}

	// Twilio test data
	twilioData = SenderTestData{
		Name:        "Test Twilio",
		Description: "Test Twilio notification sender",
		Provider:    "twilio",
		Properties: []SenderProperty{
			{
				Name:     "account_sid",
				Value:    "AC00112233445566778899aabbccddeeff",
				IsSecret: true,
			},
			{
				Name:     "auth_token",
				Value:    "auth-token-test-value",
				IsSecret: true,
			},
			{
				Name:     "sender_id",
				Value:    "+15551234567",
				IsSecret: false,
			},
		},
	}

	// Custom test data
	customData = SenderTestData{
		Name:        "Test Custom",
		Description: "Test Custom notification sender",
		Provider:    "custom",
		Properties: []SenderProperty{
			{
				Name:     "url",
				Value:    "https://api.example.com/sms",
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
			{
				Name:     "http_headers",
				Value:    "Authorization: Bearer token123, Content-Type: application/json",
				IsSecret: true,
			},
		},
	}

	// We'll use Vonage for the default sender creation in SetupSuite
	senderToCreate = convertToSenderRequest(vonageData)

	// Modified version of Vonage for update testing
	senderToUpdate = NotificationSenderRequest{
		Name:        "Updated Vonage",
		Description: "Updated Vonage notification sender",
		Provider:    "vonage",
		Properties: []SenderProperty{
			{
				Name:     "api_key",
				Value:    "updated-api-key",
				IsSecret: true,
			},
			{
				Name:     "api_secret",
				Value:    "updated-api-secret",
				IsSecret: true,
			},
			{
				Name:     "sender_id",
				Value:    "UpdatedSender",
				IsSecret: false,
			},
		},
	}
)

// Helper function to convert test data to sender request
func convertToSenderRequest(data SenderTestData) NotificationSenderRequest {
	return NotificationSenderRequest{
		Name:        data.Name,
		Description: data.Description,
		Provider:    data.Provider,
		Properties:  data.Properties,
	}
}

// Set IDs for additional senders that will be created in tests
var (
	createdSenderID string
	createdTwilioID string
	createdCustomID string
)

// NotificationSender represents a message notification sender.
type NotificationSender struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Provider    string           `json:"provider"`
	Properties  []SenderProperty `json:"properties"`
}

// NotificationSenderRequest represents the request to create/update a message notification sender.
type NotificationSenderRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Provider    string           `json:"provider"`
	Properties  []SenderProperty `json:"properties"`
}

// SenderProperty represents a key-value property for a message notification sender.
type SenderProperty struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

type NotificationSenderAPITestSuite struct {
	suite.Suite
}

func TestNotificationSenderAPITestSuite(t *testing.T) {
	suite.Run(t, new(NotificationSenderAPITestSuite))
}

// SetupSuite creates a test notification sender
func (ts *NotificationSenderAPITestSuite) SetupSuite() {
	id, err := createSender(ts)
	if err != nil {
		ts.T().Fatalf("Failed to create notification sender during setup: %v", err)
	} else {
		createdSenderID = id
	}
}

// TearDownSuite deletes the test notification sender
func (ts *NotificationSenderAPITestSuite) TearDownSuite() {
	// Delete main test sender
	if createdSenderID != "" {
		err := deleteSender(createdSenderID)
		if err != nil {
			ts.T().Fatalf("Failed to delete notification sender during teardown: %v", err)
		}
	}

	// Delete additional senders if they were created
	if createdTwilioID != "" {
		_ = deleteSender(createdTwilioID)
	}

	if createdCustomID != "" {
		_ = deleteSender(createdCustomID)
	}
}

// TestNotificationSenderListing tests listing all notification senders
func (ts *NotificationSenderAPITestSuite) TestNotificationSenderListing() {
	req, err := http.NewRequest("GET", testServerURL+"/notification-senders/message", nil)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}

	// Configure the HTTP client to skip TLS verification
	client := testutils.GetHTTPClient()

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Validate the response
	if resp.StatusCode != http.StatusOK {
		ts.T().Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Parse the response body
	var senders []NotificationSender
	err = json.NewDecoder(resp.Body).Decode(&senders)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v", err)
	}

	senderListLength := len(senders)
	if senderListLength == 0 {
		ts.T().Fatalf("Response does not contain any notification senders")
	}

	// Check if our created sender exists in the list
	foundCreated := false
	for _, sender := range senders {
		if sender.ID == createdSenderID {
			ts.Assert().Equal(senderToCreate.Name, sender.Name)
			ts.Assert().Equal(senderToCreate.Description, sender.Description)
			ts.Assert().Equal(senderToCreate.Provider, sender.Provider)
			foundCreated = true
			break
		}
	}

	ts.Assert().True(foundCreated, "Created notification sender not found in the list")
}

// TestNotificationSenderGetByID tests getting a notification sender by ID
func (ts *NotificationSenderAPITestSuite) TestNotificationSenderGetByID() {
	if createdSenderID == "" {
		ts.T().Fatal("Notification sender ID is not available for retrieval")
	}

	sender := buildCreatedSender()
	retrieveAndValidateSenderDetails(ts, sender)
}

// TestNotificationSenderUpdate tests updating a notification sender
func (ts *NotificationSenderAPITestSuite) TestNotificationSenderUpdate() {
	if createdSenderID == "" {
		ts.T().Fatal("Notification sender ID is not available for update")
	}

	senderJSON, err := json.Marshal(senderToUpdate)
	if err != nil {
		ts.T().Fatalf("Failed to marshal senderToUpdate: %v", err)
	}

	reqBody := bytes.NewReader(senderJSON)
	req, err := http.NewRequest("PUT", testServerURL+"/notification-senders/message/"+createdSenderID, reqBody)
	if err != nil {
		ts.T().Fatalf("Failed to create update request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send update request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Expected status 200, got %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	// Validate the update by retrieving the sender
	retrieveAndValidateSenderDetails(ts, NotificationSender{
		ID:          createdSenderID,
		Name:        senderToUpdate.Name,
		Description: senderToUpdate.Description,
		Provider:    senderToUpdate.Provider,
		Properties:  senderToUpdate.Properties,
	})
}

// TestNotificationSenderCreateDuplicate tests creating a notification sender with a duplicate name
func (ts *NotificationSenderAPITestSuite) TestNotificationSenderCreateDuplicate() {
	// First, make sure our updated sender exists with the updated name
	if createdSenderID == "" {
		ts.T().Fatal("Notification sender ID is not available for duplicate test")
	}

	// Get the sender to confirm its current name
	req, err := http.NewRequest("GET", testServerURL+"/notification-senders/message/"+createdSenderID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create get request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send get request: %v", err)
	}

	var existingSender NotificationSender
	err = json.NewDecoder(resp.Body).Decode(&existingSender)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v", err)
	}
	resp.Body.Close()

	// Now create a new sender with the exact same name but different properties
	duplicateSender := NotificationSenderRequest{
		Name:        existingSender.Name,
		Description: "Duplicate notification sender test",
		Provider:    "twilio",
		Properties: []SenderProperty{
			{
				Name:     "account_sid",
				Value:    "AC1234567890abcdef1234567890abcdef",
				IsSecret: true,
			},
			{
				Name:     "auth_token",
				Value:    "auth-token-test-value",
				IsSecret: true,
			},
			{
				Name:     "sender_id",
				Value:    "+15551234000",
				IsSecret: false,
			},
		},
	}

	ts.T().Logf("Attempting to create duplicate sender with name: %s", existingSender.Name)

	senderJSON, err := json.Marshal(duplicateSender)
	if err != nil {
		ts.T().Fatalf("Failed to marshal duplicateSender: %v", err)
	}

	reqBody := bytes.NewReader(senderJSON)
	req, err = http.NewRequest("POST", testServerURL+"/notification-senders/message", reqBody)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	resp, err = client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Expect a conflict status code
	ts.Assert().Equal(http.StatusConflict, resp.StatusCode,
		"Expected status 409 (Conflict) for duplicate sender name")

	// Verify the error response
	var errorResponse struct {
		Code        string                `json:"code"`
		Message     testutils.I18nMessage `json:"message"`
		Description testutils.I18nMessage `json:"description"`
	}

	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	if err != nil {
		ts.T().Fatalf("Failed to parse error response: %v", err)
	}

	ts.T().Logf("Error response: %+v", errorResponse)
	ts.Assert().Contains(errorResponse.Description.DefaultValue, "already exists",
		"Error description should mention that the name already exists")
}

// TestNotificationSenderInvalidProvider tests creating a notification sender with an invalid provider
func (ts *NotificationSenderAPITestSuite) TestNotificationSenderInvalidProvider() {
	invalidSender := NotificationSenderRequest{
		Name:        "Invalid Provider Sender",
		Description: "Test sender with invalid provider",
		Provider:    "invalid-provider",
		Properties:  []SenderProperty{},
	}

	senderJSON, err := json.Marshal(invalidSender)
	if err != nil {
		ts.T().Fatalf("Failed to marshal invalidSender: %v", err)
	}

	reqBody := bytes.NewReader(senderJSON)
	req, err := http.NewRequest("POST", testServerURL+"/notification-senders/message", reqBody)
	if err != nil {
		ts.T().Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Expect a bad request status code
	ts.Assert().Equal(http.StatusBadRequest, resp.StatusCode,
		"Expected status 400 (Bad Request) for invalid provider")
}

func retrieveAndValidateSenderDetails(ts *NotificationSenderAPITestSuite, expectedSender NotificationSender) {
	req, err := http.NewRequest("GET", testServerURL+"/notification-senders/message/"+expectedSender.ID, nil)
	if err != nil {
		ts.T().Fatalf("Failed to create get request: %v", err)
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		ts.T().Fatalf("Failed to send get request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ts.T().Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Check if the response Content-Type is application/json
	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		rawBody, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Unexpected Content-Type: %s. Raw body: %s", contentType, string(rawBody))
	}

	var sender NotificationSender
	err = json.NewDecoder(resp.Body).Decode(&sender)
	if err != nil {
		ts.T().Fatalf("Failed to parse response body: %v", err)
	}

	ts.Assert().Equal(expectedSender.ID, sender.ID)
	ts.Assert().Equal(expectedSender.Name, sender.Name)
	ts.Assert().Equal(expectedSender.Description, sender.Description)
	ts.Assert().Equal(expectedSender.Provider, sender.Provider)

	// Check if properties are returned (may be masked for secret values)
	ts.Assert().NotEmpty(sender.Properties, "Expected properties to be returned")

	// For non-secret properties, check exact values
	for _, expectedProp := range expectedSender.Properties {
		if !expectedProp.IsSecret {
			found := false
			for _, actualProp := range sender.Properties {
				if actualProp.Name == expectedProp.Name {
					ts.Assert().Equal(expectedProp.Value, actualProp.Value,
						"Non-secret property value mismatch for %s", expectedProp.Name)
					found = true
					break
				}
			}
			ts.Assert().True(found, "Property %s not found in response", expectedProp.Name)
		}
	}

	// For secret properties, check they are masked
	for _, prop := range sender.Properties {
		if prop.IsSecret {
			ts.Assert().Equal("******", prop.Value, "Secret property should be masked")
		}
	}
}

func createSender(ts *NotificationSenderAPITestSuite) (string, error) {
	id, err := createSenderWithRequest(senderToCreate)
	if err != nil {
		ts.T().Logf("Failed to create sender: %v", err)
	}
	return id, err
}

func deleteSender(senderID string) error {
	req, err := http.NewRequest("DELETE", testServerURL+"/notification-senders/message/"+senderID, nil)
	if err != nil {
		return err
	}

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func buildCreatedSender() NotificationSender {
	return NotificationSender{
		ID:          createdSenderID,
		Name:        senderToCreate.Name,
		Description: senderToCreate.Description,
		Provider:    senderToCreate.Provider,
		Properties:  senderToCreate.Properties,
	}
}

// TestCreateDifferentProviderTypes tests creating senders for different provider types
func (ts *NotificationSenderAPITestSuite) TestCreateDifferentProviderTypes() {
	// Test Twilio sender creation
	twilioReq := convertToSenderRequest(twilioData)
	twilioID, err := createSenderWithRequest(twilioReq)
	ts.Require().NoError(err, "Failed to create Twilio sender")
	createdTwilioID = twilioID

	// Verify Twilio sender
	twilioSender := NotificationSender{
		ID:          twilioID,
		Name:        twilioData.Name,
		Description: twilioData.Description,
		Provider:    twilioData.Provider,
		Properties:  twilioData.Properties,
	}
	retrieveAndValidateSenderDetails(ts, twilioSender)

	// Test Custom sender creation
	customReq := convertToSenderRequest(customData)
	customID, err := createSenderWithRequest(customReq)
	ts.Require().NoError(err, "Failed to create Custom sender")
	createdCustomID = customID

	// Verify Custom sender
	customSender := NotificationSender{
		ID:          customID,
		Name:        customData.Name,
		Description: customData.Description,
		Provider:    customData.Provider,
		Properties:  customData.Properties,
	}
	retrieveAndValidateSenderDetails(ts, customSender)
}

// TestNotificationSenderInvalidProviderProperties tests validation of required properties
func (ts *NotificationSenderAPITestSuite) TestNotificationSenderInvalidProviderProperties() {
	// Test cases for invalid properties
	testCases := []struct {
		name           string
		provider       string
		properties     []SenderProperty
		expectedStatus int
	}{
		{
			name:     "Twilio Missing Properties",
			provider: "twilio",
			properties: []SenderProperty{
				{Name: "account_sid", Value: "test", IsSecret: true},
				// Missing auth_token and from_number
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "Vonage Missing Properties",
			provider: "vonage",
			properties: []SenderProperty{
				{Name: "api_key", Value: "test", IsSecret: true},
				// Missing api_secret and from
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:     "Custom Missing URL",
			provider: "custom",
			properties: []SenderProperty{
				{Name: "http_method", Value: "POST", IsSecret: false},
				{Name: "content_type", Value: "JSON", IsSecret: false},
				// Missing url which is required
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			invalidSender := NotificationSenderRequest{
				Name:        "Invalid " + tc.provider + " Sender",
				Description: "Test with invalid properties",
				Provider:    tc.provider,
				Properties:  tc.properties,
			}

			senderJSON, err := json.Marshal(invalidSender)
			ts.Require().NoError(err, "Failed to marshal sender")

			reqBody := bytes.NewReader(senderJSON)
			req, err := http.NewRequest("POST", testServerURL+"/notification-senders/message", reqBody)
			ts.Require().NoError(err, "Failed to create request")

			req.Header.Set("Content-Type", "application/json")

			client := testutils.GetHTTPClient()

			resp, err := client.Do(req)
			ts.Require().NoError(err, "Failed to send request")
			defer resp.Body.Close()

			ts.Assert().Equal(tc.expectedStatus, resp.StatusCode,
				"Expected status %d for invalid %s properties", tc.expectedStatus, tc.provider)
		})
	}
}

// createSenderWithRequest is a utility function to create a sender from a request
func createSenderWithRequest(sender NotificationSenderRequest) (string, error) {
	senderJSON, err := json.Marshal(sender)
	if err != nil {
		return "", fmt.Errorf("failed to marshal sender: %w", err)
	}

	reqBody := bytes.NewReader(senderJSON)
	req, err := http.NewRequest("POST", testServerURL+"/notification-senders/message", reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
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

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
	"io"
	"net/http"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

type SenderPropertyValidationTestSuite struct {
	suite.Suite
	client *http.Client
}

// TestSenderPropertyValidationTestSuite runs the test suite
func TestSenderPropertyValidationTestSuite(t *testing.T) {
	suite.Run(t, new(SenderPropertyValidationTestSuite))
}

// SetupSuite initializes the HTTP client
func (ts *SenderPropertyValidationTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()
}

// TestVonageMissingRequiredProperties tests validation with missing properties for Vonage provider
func (ts *SenderPropertyValidationTestSuite) TestVonageMissingRequiredProperties() {
	testCases := []struct {
		name            string
		properties      []SenderProperty
		missingPropName string
	}{
		{
			name: "Missing API Key",
			properties: []SenderProperty{
				{Name: "api_secret", Value: "test-secret", IsSecret: true},
				{Name: "sender_id", Value: "TestSender", IsSecret: false},
			},
			missingPropName: "api_key",
		},
		{
			name: "Missing API Secret",
			properties: []SenderProperty{
				{Name: "api_key", Value: "test-key", IsSecret: true},
				{Name: "sender_id", Value: "TestSender", IsSecret: false},
			},
			missingPropName: "api_secret",
		},
		{
			name: "Missing From",
			properties: []SenderProperty{
				{Name: "api_key", Value: "test-key", IsSecret: true},
				{Name: "api_secret", Value: "test-secret", IsSecret: true},
			},
			missingPropName: "sender_id",
		},
		{
			name:            "Empty Properties",
			properties:      []SenderProperty{},
			missingPropName: "api_key",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			sender := NotificationSenderRequest{
				Name:        "Test Vonage " + tc.name,
				Description: "Test Vonage with missing properties",
				Provider:    "vonage",
				Properties:  tc.properties,
			}

			response, responseBody := ts.sendCreateSenderRequest(sender)

			ts.Equal(http.StatusBadRequest, response.StatusCode,
				"Expected 400 Bad Request for missing Vonage properties")

			var errorResp map[string]interface{}
			err := json.Unmarshal(responseBody, &errorResp)
			ts.Require().NoError(err, "Failed to unmarshal error response")

			// Verify error details
			if len(tc.properties) == 0 {
				ts.Assert().Contains(errorResp["description"].(map[string]interface{})["defaultValue"].(string), "cannot be empty",
					"Error should mention that properties cannot be empty")
			} else {
				ts.Assert().Contains(errorResp["description"].(map[string]interface{})["defaultValue"].(string), "missing",
					"Error should mention property is missing")
				ts.Assert().Contains(errorResp["description"].(map[string]interface{})["defaultValue"].(string), tc.missingPropName,
					"Error should mention which property is missing")
			}
		})
	}
}

// TestTwilioMissingRequiredProperties tests validation with missing properties for Twilio provider
func (ts *SenderPropertyValidationTestSuite) TestTwilioMissingRequiredProperties() {
	testCases := []struct {
		name            string
		properties      []SenderProperty
		missingPropName string
	}{
		{
			name: "Missing Account SID",
			properties: []SenderProperty{
				{Name: "auth_token", Value: "test-token", IsSecret: true},
				{Name: "sender_id", Value: "+15551234567", IsSecret: false},
			},
			missingPropName: "account_sid",
		},
		{
			name: "Missing Auth Token",
			properties: []SenderProperty{
				{Name: "account_sid", Value: "AC00112233445566778899aabbccddeeff", IsSecret: true},
				{Name: "sender_id", Value: "+15551234567", IsSecret: false},
			},
			missingPropName: "auth_token",
		},
		{
			name: "Missing From Number",
			properties: []SenderProperty{
				{Name: "account_sid", Value: "AC00112233445566778899aabbccddeeff", IsSecret: true},
				{Name: "auth_token", Value: "test-token", IsSecret: true},
			},
			missingPropName: "sender_id",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			sender := NotificationSenderRequest{
				Name:        "Test Twilio " + tc.name,
				Description: "Test Twilio with missing properties",
				Provider:    "twilio",
				Properties:  tc.properties,
			}

			response, responseBody := ts.sendCreateSenderRequest(sender)

			ts.Equal(http.StatusBadRequest, response.StatusCode,
				"Expected 400 Bad Request for missing Twilio properties")

			var errorResp map[string]interface{}
			err := json.Unmarshal(responseBody, &errorResp)
			ts.Require().NoError(err, "Failed to unmarshal error response")

			// Verify error details
			ts.Assert().Contains(errorResp["description"].(map[string]interface{})["defaultValue"].(string), "missing",
				"Error should mention property is missing")
			ts.Assert().Contains(errorResp["description"].(map[string]interface{})["defaultValue"].(string), tc.missingPropName,
				"Error should mention which property is missing")
		})
	}
}

// TestCustomMissingRequiredProperties tests validation with missing properties for Custom provider
func (ts *SenderPropertyValidationTestSuite) TestCustomMissingRequiredProperties() {
	testCases := []struct {
		name          string
		properties    []SenderProperty
		expectSuccess bool
	}{
		{
			name: "Missing URL",
			properties: []SenderProperty{
				{Name: "http_method", Value: "POST", IsSecret: false},
				{Name: "content_type", Value: "JSON", IsSecret: false},
			},
			expectSuccess: false,
		},
		{
			name: "Has URL Only",
			properties: []SenderProperty{
				{Name: "url", Value: "https://example.com", IsSecret: false},
			},
			expectSuccess: true,
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			sender := NotificationSenderRequest{
				Name:        "Test Custom " + tc.name,
				Description: "Test Custom with various properties",
				Provider:    "custom",
				Properties:  tc.properties,
			}

			response, responseBody := ts.sendCreateSenderRequest(sender)

			if tc.expectSuccess {
				ts.Equal(http.StatusCreated, response.StatusCode,
					"Expected 201 Created for valid Custom provider configuration")

				var respData map[string]interface{}
				err := json.Unmarshal(responseBody, &respData)
				ts.Require().NoError(err, "Failed to unmarshal response")

				// Delete the created sender to clean up
				if id, ok := respData["id"].(string); ok && id != "" {
					ts.deleteTestSender(id)
				}
			} else {
				ts.Equal(http.StatusBadRequest, response.StatusCode,
					"Expected 400 Bad Request for missing Custom properties")

				var errorResp map[string]interface{}
				err := json.Unmarshal(responseBody, &errorResp)
				ts.Require().NoError(err, "Failed to unmarshal error response")

				// Verify error details contain "url"
				ts.Assert().Contains(errorResp["description"].(map[string]interface{})["defaultValue"].(string), "URL",
					"Error should mention URL is required")
			}
		})
	}
}

// TestInvalidTwilioAccountSID tests validation of Twilio account SID format
func (ts *SenderPropertyValidationTestSuite) TestInvalidTwilioAccountSID() {
	testCases := []struct {
		name       string
		accountSID string
	}{
		{
			name:       "Too Short",
			accountSID: "AC1234",
		},
		{
			name:       "Wrong Prefix",
			accountSID: "XX00112233445566778899aabbccddeeff",
		},
		{
			name:       "Invalid Characters",
			accountSID: "AC00112233445566778899aabbccddeef$",
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			sender := NotificationSenderRequest{
				Name:        "Test Twilio Invalid SID " + tc.name,
				Description: "Test Twilio with invalid account SID",
				Provider:    "twilio",
				Properties: []SenderProperty{
					{Name: "account_sid", Value: tc.accountSID, IsSecret: true},
					{Name: "auth_token", Value: "test-token", IsSecret: true},
					{Name: "sender_id", Value: "+15551234567", IsSecret: false},
				},
			}

			response, responseBody := ts.sendCreateSenderRequest(sender)

			ts.Equal(http.StatusBadRequest, response.StatusCode,
				"Expected 400 Bad Request for invalid Twilio account SID format")

			var errorResp map[string]interface{}
			err := json.Unmarshal(responseBody, &errorResp)
			ts.Require().NoError(err, "Failed to unmarshal error response")

			// Verify error details related to account SID format
			ts.Assert().Contains(errorResp["description"].(map[string]interface{})["defaultValue"].(string), "invalid",
				"Error should mention format is invalid")
		})
	}
}

// TestEmptyPropertyName tests validation when property name is empty
func (ts *SenderPropertyValidationTestSuite) TestEmptyPropertyName() {
	testCases := []struct {
		name     string
		provider string
	}{
		{name: "Vonage with Empty Property Name", provider: "vonage"},
		{name: "Twilio with Empty Property Name", provider: "twilio"},
		{name: "Custom with Empty Property Name", provider: "custom"},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			sender := NotificationSenderRequest{
				Name:        "Test " + tc.name,
				Description: "Test with empty property name",
				Provider:    tc.provider,
				Properties: []SenderProperty{
					{Name: "", Value: "some-value", IsSecret: false},
				},
			}

			response, responseBody := ts.sendCreateSenderRequest(sender)

			ts.Equal(http.StatusBadRequest, response.StatusCode,
				"Expected 400 Bad Request for empty property name")

			var errorResp map[string]interface{}
			err := json.Unmarshal(responseBody, &errorResp)
			ts.Require().NoError(err, "Failed to unmarshal error response")

			// Verify error details related to empty property name
			ts.Assert().Contains(errorResp["description"].(map[string]interface{})["defaultValue"].(string), "name",
				"Error should mention issue with property name")
		})
	}
}

// TestContentTypeValidationForCustomProvider tests validation of content_type property for Custom provider
func (ts *SenderPropertyValidationTestSuite) TestContentTypeValidationForCustomProvider() {
	testCases := []struct {
		name        string
		contentType string
		isValid     bool
	}{
		{
			name:        "Valid JSON Content Type",
			contentType: "JSON",
			isValid:     true,
		},
		{
			name:        "Valid FORM Content Type",
			contentType: "FORM",
			isValid:     true,
		},
		{
			name:        "Case Insensitive JSON",
			contentType: "json",
			isValid:     true,
		},
		{
			name:        "Case Insensitive FORM",
			contentType: "form",
			isValid:     true,
		},
		{
			name:        "Invalid Content Type",
			contentType: "XML",
			isValid:     false,
		},
	}

	for _, tc := range testCases {
		ts.Run(tc.name, func() {
			sender := NotificationSenderRequest{
				Name:        "Test Custom " + tc.name,
				Description: "Test with various content types",
				Provider:    "custom",
				Properties: []SenderProperty{
					{Name: "url", Value: "https://example.com", IsSecret: false},
					{Name: "http_method", Value: "POST", IsSecret: false},
					{Name: "content_type", Value: tc.contentType, IsSecret: false},
				},
			}

			response, responseBody := ts.sendCreateSenderRequest(sender)

			if tc.isValid {
				ts.Equal(http.StatusCreated, response.StatusCode,
					"Expected 201 Created for valid content type")

				var respData map[string]interface{}
				err := json.Unmarshal(responseBody, &respData)
				ts.Require().NoError(err, "Failed to unmarshal response")

				// Clean up the created sender
				if id, ok := respData["id"].(string); ok && id != "" {
					ts.deleteTestSender(id)
				}
			} else {
				ts.Equal(http.StatusBadRequest, response.StatusCode,
					"Expected 400 Bad Request for invalid content type")

				var errorResp map[string]interface{}
				err := json.Unmarshal(responseBody, &errorResp)
				ts.Require().NoError(err, "Failed to unmarshal error response")

				ts.Assert().Contains(errorResp["description"].(map[string]interface{})["defaultValue"].(string), "content type",
					"Error should mention content type issue")
			}
		})
	}
}

// Helper method to send a create sender request and return the response and body
func (ts *SenderPropertyValidationTestSuite) sendCreateSenderRequest(
	sender NotificationSenderRequest) (*http.Response, []byte) {
	senderJSON, err := json.Marshal(sender)
	ts.Require().NoError(err, "Failed to marshal sender request")

	reqBody := bytes.NewReader(senderJSON)
	req, err := http.NewRequest("POST", testServerURL+"/notification-senders/message", reqBody)
	ts.Require().NoError(err, "Failed to create HTTP request")

	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send HTTP request")
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err, "Failed to read response body")

	return resp, respBody
}

// Helper method to delete a test sender
func (ts *SenderPropertyValidationTestSuite) deleteTestSender(id string) {
	req, err := http.NewRequest("DELETE", testServerURL+"/notification-senders/message/"+id, nil)
	ts.Require().NoError(err, "Failed to create delete request")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to send delete request")
	resp.Body.Close()
}

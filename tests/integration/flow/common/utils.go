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

package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const testServerURL = "https://localhost:8095"

// InitiateAuthenticationFlow initiates the authentication flow
func InitiateAuthenticationFlow(appID string, verbose bool, inputs map[string]string, action string) (
	*FlowStep, error) {
	return initiateFlow(appID, "AUTHENTICATION", verbose, inputs, action)
}

// InitiateRegistrationFlow initiates the registration flow
func InitiateRegistrationFlow(appID string, verbose bool, inputs map[string]string, action string) (
	*FlowStep, error) {
	return initiateFlow(appID, "REGISTRATION", verbose, inputs, action)
}

// InitiateRecoveryFlow initiates the recovery flow
func InitiateRecoveryFlow(appID string, verbose bool, inputs map[string]string, action string) (
	*FlowStep, error) {
	return initiateFlow(appID, "RECOVERY", verbose, inputs, action)
}

// initiateFlow is a generic helper to initiate a flow of a given type
func initiateFlow(appID, flowType string, verbose bool, inputs map[string]string, action string) (
	*FlowStep, error) {
	flowReqBody := map[string]interface{}{
		"applicationId": appID,
		"flowType":      flowType,
	}
	if verbose {
		flowReqBody["verbose"] = true
	}
	if len(inputs) > 0 {
		flowReqBody["inputs"] = inputs
	}
	if action != "" {
		flowReqBody["action"] = action
	}

	reqBody, err := json.Marshal(flowReqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create flow request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send flow request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	var flowStep FlowStep
	err = json.NewDecoder(resp.Body).Decode(&flowStep)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	return &flowStep, nil
}

// InitiateAuthFlowWithError initiates the authentication flow and expects an error response
func InitiateAuthFlowWithError(appID string, inputs map[string]string) (*ErrorResponse, error) {
	flowReqBody := map[string]interface{}{
		"applicationId": appID,
		"flowType":      "AUTHENTICATION",
	}
	if len(inputs) > 0 {
		flowReqBody["inputs"] = inputs
	}

	reqBody, err := json.Marshal(flowReqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create flow request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send flow request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var errorResponse ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse error response body: %w", err)
	}

	return &errorResponse, nil
}

// ResumeFlow resumes a flow without new inputs or an action selection.
func ResumeFlow(flowID string) (*FlowStep, error) {
	return CompleteFlow(flowID, map[string]string{}, "", "")
}

// CompleteFlow completes the flow with given inputs, action and challenge token
func CompleteFlow(executionId string, inputs map[string]string, action string, challengeToken string) (
	*FlowStep, error) {
	flowReqBody := map[string]interface{}{
		"executionId":    executionId,
		"challengeToken": challengeToken,
	}
	if len(inputs) > 0 {
		flowReqBody["inputs"] = inputs
	}
	if action != "" {
		flowReqBody["action"] = action
	}

	reqBody, err := json.Marshal(flowReqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create flow request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send flow request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var flowStep FlowStep
	err = json.NewDecoder(resp.Body).Decode(&flowStep)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	return &flowStep, nil
}

// CompleteAuthFlowWithError completes the authentication flow and expects an error response
func CompleteAuthFlowWithError(executionId string, inputs map[string]string, challengeToken string) (
	*ErrorResponse, error) {
	flowReqBody := map[string]interface{}{
		"executionId":    executionId,
		"challengeToken": challengeToken,
		"inputs":         inputs,
	}

	reqBody, err := json.Marshal(flowReqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest("POST", testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create flow request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := testutils.GetHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send flow request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var errorResponse ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse error response body: %w", err)
	}

	return &errorResponse, nil
}

// GetAppConfig retrieves the current application configuration
func GetAppConfig(appID string) (map[string]interface{}, error) {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/applications/%s", testServerURL, appID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var appConfig map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&appConfig); err != nil {
		return nil, fmt.Errorf("failed to parse app config: %w", err)
	}

	return appConfig, nil
}

// UpdateAppConfig updates the application configuration with the specified flow IDs
func UpdateAppConfig(appID, authFlowID, registrationFlowID string) error {
	appConfig, err := GetAppConfig(appID)
	if err != nil {
		return fmt.Errorf("failed to get current app config: %w", err)
	}

	if authFlowID != "" {
		appConfig["authFlowId"] = authFlowID
	}
	if registrationFlowID != "" {
		appConfig["registrationFlowId"] = registrationFlowID
	}
	appConfig["clientSecret"] = "secret123"

	client := testutils.GetHTTPClient()

	jsonPayload, err := json.Marshal(appConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/applications/%s", testServerURL, appID),
		bytes.NewBuffer(jsonPayload),
	)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreateNotificationSender creates a custom notification sender with a specified URL and name
func CreateNotificationSender(senderURL, senderName string) (string, error) {
	senderRequest := map[string]interface{}{
		"name":        senderName,
		"description": "Custom SMS sender for integration tests",
		"provider":    "custom",
		"properties": []map[string]interface{}{
			{
				"name":      "url",
				"value":     senderURL,
				"is_secret": false,
			},
			{
				"name":      "http_method",
				"value":     "POST",
				"is_secret": false,
			},
			{
				"name":      "content_type",
				"value":     "JSON",
				"is_secret": false,
			},
		},
	}

	jsonPayload, err := json.Marshal(senderRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal sender request: %w", err)
	}

	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("POST", testServerURL+"/notification-senders/message", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create sender request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("sender request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("sender creation failed with status %d: %s", resp.StatusCode, string(body))
	}

	var sender map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&sender); err != nil {
		return "", fmt.Errorf("failed to parse sender response: %w", err)
	}

	senderID, ok := sender["id"].(string)
	if !ok {
		return "", fmt.Errorf("sender ID not found in response")
	}

	return senderID, nil
}

// DeleteNotificationSender deletes a notification sender
func DeleteNotificationSender(senderID string) error {
	client := testutils.GetHTTPClient()

	req, err := http.NewRequest("DELETE", testServerURL+"/notification-senders/message/"+senderID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// RestoreAppConfig restores the original application configuration
func RestoreAppConfig(appID string, originalConfig map[string]interface{}) error {
	if originalConfig == nil {
		return fmt.Errorf("no original config to restore")
	}

	// Add client secret to original config for restoration
	originalConfig["clientSecret"] = "secret123"

	client := testutils.GetHTTPClient()

	jsonPayload, err := json.Marshal(originalConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal original config: %w", err)
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/applications/%s", testServerURL, appID),
		bytes.NewBuffer(jsonPayload),
	)
	if err != nil {
		return fmt.Errorf("failed to create restore request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("restore request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("restore failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ValidateRequiredInputs validates that the expected input names are present in the flow data
func ValidateRequiredInputs(actualInputs []Inputs, expectedInputNames []string) bool {
	inputMap := make(map[string]bool)
	for _, input := range actualInputs {
		inputMap[input.Identifier] = true
	}

	for _, expectedName := range expectedInputNames {
		if !inputMap[expectedName] {
			return false
		}
	}

	return true
}

// ValidateRequiredActions validates that the expected action refs are present in the flow data
func ValidateRequiredActions(actualActions []Action, expectedActionRefs []string) bool {
	actionMap := make(map[string]bool)
	for _, action := range actualActions {
		actionMap[action.Ref] = true
	}

	for _, expectedRef := range expectedActionRefs {
		if !actionMap[expectedRef] {
			return false
		}
	}

	return true
}

// HasInput checks if a specific input is present in the flow data
func HasInput(inputs []Inputs, inputName string) bool {
	for _, input := range inputs {
		if input.Identifier == inputName {
			return true
		}
	}
	return false
}

// HasAction checks if a specific action is present in the flow data
func HasAction(actions []Action, actionRef string) bool {
	for _, action := range actions {
		if action.Ref == actionRef {
			return true
		}
	}
	return false
}

// WaitAndValidateNotification waits for a notification to be sent and validates it
// This is a generic helper that can be used with different mock server types
func WaitAndValidateNotification(mockServer interface{}, expectedCount int, timeoutSeconds int) error {
	// This would need to be implemented based on the specific mock server interface
	// For now, we'll return a placeholder
	return fmt.Errorf("notification validation not implemented - should be customized per mock server type")
}

// GenerateUniqueUsername generates a unique username using the given prefix
func GenerateUniqueUsername(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

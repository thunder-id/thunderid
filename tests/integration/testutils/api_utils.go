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

package testutils

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	TestServerURL = "https://localhost:8095"
)

// GetHTTPClient returns a configured HTTP client for test requests with automatic auth injection
func GetHTTPClient() *http.Client {
	return NewHTTPClientWithTokenProvider(GetAccessToken)
}

// GetNoRedirectHTTPClient returns an HTTP client that does not follow redirects.
func GetNoRedirectHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 30 * time.Second,
	}
}

// CreateUserType creates a user type via API and returns the schema ID
func CreateUserType(schema UserType) (string, error) {
	if !schema.AllowSelfRegistration {
		schema.AllowSelfRegistration = true
	}

	payload, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("failed to marshal user type: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/user-types", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdSchema map[string]interface{}
	err = json.Unmarshal(bodyBytes, &createdSchema)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	schemaID, ok := createdSchema["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}
	return schemaID, nil
}

// CreateAgentType ensures the single allowed `default` agent type exists with the given schema
// and returns its ID. The server restricts agent types to one `default` schema and rejects
// deletion, so suites share the singleton: this helper creates it on first call and updates
// it (PUT) on subsequent calls so each suite's schema fixture takes effect. The caller's
// `Name` is ignored — it is always coerced to `default`.
func CreateAgentType(schema UserType) (string, error) {
	schema.Name = "default"

	id, err := postAgentType(schema)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, errAgentTypeNameConflict) {
		return "", err
	}

	existingID, lookupErr := findDefaultAgentTypeID()
	if lookupErr != nil {
		return "", lookupErr
	}
	if updateErr := putAgentType(existingID, schema); updateErr != nil {
		return "", updateErr
	}
	return existingID, nil
}

// DeleteAgentType is a no-op. The server rejects agent type deletion (USRS-1015) — see
// CreateAgentType for how suites share the singleton `default` schema.
func DeleteAgentType(_ string) error {
	return nil
}

var errAgentTypeNameConflict = errors.New("agent type name conflict")

func postAgentType(schema UserType) (string, error) {
	payload, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("failed to marshal agent type: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/agent-types", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode == http.StatusConflict {
		return "", errAgentTypeNameConflict
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdSchema map[string]interface{}
	if err = json.Unmarshal(bodyBytes, &createdSchema); err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}
	id, ok := createdSchema["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}
	return id, nil
}

func putAgentType(schemaID string, schema UserType) error {
	payload, err := json.Marshal(schema)
	if err != nil {
		return fmt.Errorf("failed to marshal agent type: %w", err)
	}
	req, err := http.NewRequest("PUT",
		fmt.Sprintf("%s/agent-types/%s", TestServerURL, schemaID), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
	}
	return nil
}

func findDefaultAgentTypeID() (string, error) {
	req, err := http.NewRequest("GET", TestServerURL+"/agent-types?limit=100", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to list agent types: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
	}

	var list struct {
		Types []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"types"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return "", fmt.Errorf("failed to parse list response: %w. Response: %s", err, string(body))
	}
	for _, s := range list.Types {
		if s.Name == "default" {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("default agent type not found in list. Response: %s", string(body))
}

// CreateUser creates a user via API and returns the user ID
func CreateUser(user User) (string, error) {
	userJSON, err := json.Marshal(user)
	if err != nil {
		return "", fmt.Errorf("failed to marshal user: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/users", bytes.NewReader(userJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdUser map[string]interface{}
	err = json.Unmarshal(bodyBytes, &createdUser)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	userID, ok := createdUser["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}
	return userID, nil
}

// DeleteUserType deletes a user type by ID
func DeleteUserType(schemaID string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/user-types/%s", TestServerURL, schemaID), nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete user type: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DeleteUser deletes a user by ID
func DeleteUser(userID string) error {
	req, err := http.NewRequest("DELETE", TestServerURL+"/users/"+userID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(body))
	}
	return nil
}

// CreateMultipleUsers creates multiple users and returns their IDs
func CreateMultipleUsers(users ...User) ([]string, error) {
	var userIDs []string

	for i, user := range users {
		userID, err := CreateUser(user)
		if err != nil {
			// Cleanup already created users on failure
			for _, createdID := range userIDs {
				DeleteUser(createdID)
			}
			return nil, fmt.Errorf("failed to create user %d: %w", i, err)
		}
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

// CleanupUsers deletes multiple users
func CleanupUsers(userIDs []string) error {
	var errs []error

	for _, userID := range userIDs {
		if userID != "" {
			if err := DeleteUser(userID); err != nil {
				errs = append(errs, fmt.Errorf("failed to delete user %s: %w", userID, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	return nil
}

// CreateApplication creates an application via API and returns the application ID
func CreateApplication(app Application) (string, error) {
	redirectURIs := app.RedirectURIs
	if len(redirectURIs) == 0 {
		redirectURIs = []string{"http://localhost:8080/callback"}
	}

	inboundAuthConfig := app.InboundAuthConfig
	if len(inboundAuthConfig) == 0 {
		inboundAuthConfig = []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":     app.ClientID,
					"clientSecret": app.ClientSecret,
					"redirectUris": redirectURIs,
				},
			},
		}
	}

	appData := map[string]interface{}{
		"name":                      app.Name,
		"description":               app.Description,
		"ouId":                      app.OUID,
		"isRegistrationFlowEnabled": app.IsRegistrationFlowEnabled,
		"isRecoveryFlowEnabled":     app.IsRecoveryFlowEnabled,
		"authFlowId":                app.AuthFlowID,
		"registrationFlowId":        app.RegistrationFlowID,
		"recoveryFlowId":            app.RecoveryFlowID,
		"inboundAuthConfig":         inboundAuthConfig,
	}

	// Add allowed_user_types if provided
	if len(app.AllowedUserTypes) > 0 {
		appData["allowedUserTypes"] = app.AllowedUserTypes
	}

	// Add assertion config if provided
	if app.AssertionConfig != nil {
		appData["assertion"] = app.AssertionConfig
	}

	appJSON, err := json.Marshal(appData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal application: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/applications", bytes.NewReader(appJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdApp map[string]interface{}
	err = json.Unmarshal(bodyBytes, &createdApp)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	appID, ok := createdApp["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}
	return appID, nil
}

// DeleteApplication deletes an application by ID
func DeleteApplication(appID string) error {
	req, err := http.NewRequest("DELETE", TestServerURL+"/applications/"+appID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete application: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// CreateOrganizationUnit creates an organization unit via API and returns the OU ID
func CreateOrganizationUnit(ou OrganizationUnit) (string, error) {
	ouJSON, err := json.Marshal(ou)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OU request: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/organization-units", bytes.NewReader(ouJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdOU map[string]interface{}
	err = json.Unmarshal(bodyBytes, &createdOU)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	ouID, ok := createdOU["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}
	return ouID, nil
}

// DeleteOrganizationUnit deletes an organization unit by ID
func DeleteOrganizationUnit(ouID string) error {
	req, err := http.NewRequest("DELETE", TestServerURL+"/organization-units/"+ouID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete organization unit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200 or 204, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// DeleteOrganizationUnitByHandlePath deletes an organization unit by its hierarchical handle path
func DeleteOrganizationUnitByHandlePath(handlePath string) error {
	req, err := http.NewRequest("DELETE", TestServerURL+"/organization-units/tree/"+handlePath, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete organization unit by handle path: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200 or 204, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// GetOrganizationUnit retrieves an organization unit by ID
func GetOrganizationUnit(ouID string) (*OrganizationUnit, error) {
	req, err := http.NewRequest("GET", TestServerURL+"/organization-units/"+ouID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	var ou OrganizationUnit
	err = json.NewDecoder(resp.Body).Decode(&ou)
	if err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	return &ou, nil
}

// CreateIDP creates an identity provider via API and returns the IDP ID
func CreateIDP(idp IDP) (string, error) {
	idpJSON, err := json.Marshal(idp)
	if err != nil {
		return "", fmt.Errorf("failed to marshal IDP: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/identity-providers", bytes.NewReader(idpJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdIDP map[string]interface{}
	err = json.Unmarshal(bodyBytes, &createdIDP)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	idpID, ok := createdIDP["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}
	return idpID, nil
}

// DeleteIDP deletes an identity provider by ID
func DeleteIDP(idpID string) error {
	req, err := http.NewRequest("DELETE", TestServerURL+"/identity-providers/"+idpID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete identity provider: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200 or 204, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// GetIDP retrieves an identity provider by ID
func GetIDP(idpID string) (*IDP, error) {
	req, err := http.NewRequest(
		"GET",
		fmt.Sprintf("%s/identity-providers/%s", TestServerURL, idpID),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create get request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("IDP get request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("IDP get failed with status %d: %s", resp.StatusCode, string(body))
	}

	var idp IDP
	if err := json.NewDecoder(resp.Body).Decode(&idp); err != nil {
		return nil, fmt.Errorf("failed to decode IDP response: %w", err)
	}

	return &idp, nil
}

// UpdateIDP updates an existing identity provider
func UpdateIDP(idpID string, idp IDP) error {
	idpJSON, err := json.Marshal(idp)
	if err != nil {
		return fmt.Errorf("failed to marshal IDP: %w", err)
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/identity-providers/%s", TestServerURL, idpID),
		bytes.NewReader(idpJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to create update request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("IDP update request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("IDP update failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// GetUserAttributes extracts user attributes from JSON into a map
func GetUserAttributes(user User) (map[string]interface{}, error) {
	var userAttrs map[string]interface{}
	err := json.Unmarshal(user.Attributes, &userAttrs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user attributes: %w", err)
	}
	return userAttrs, nil
}

// FindUserByAttribute retrieves all users and returns the user with a matching attribute key and value
func FindUserByAttribute(key, value string) (*User, error) {
	client := GetHTTPClient()

	req, err := http.NewRequest("GET", TestServerURL+"/users", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create user list request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send user list request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user list, status: %d", resp.StatusCode)
	}

	var userListResponse UserListResponse
	err = json.NewDecoder(resp.Body).Decode(&userListResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to parse user list response: %w", err)
	}

	for _, user := range userListResponse.Users {
		attrs, err := GetUserAttributes(user)

		if err != nil {
			continue
		}
		if v, ok := attrs[key]; ok && v == value {
			return &user, nil
		}
	}
	return nil, nil
}

// CreateGroup creates a group via API and returns the group ID
func CreateGroup(group Group) (string, error) {
	groupJSON, err := json.Marshal(group)
	if err != nil {
		return "", fmt.Errorf("failed to marshal group: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/groups", bytes.NewReader(groupJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdGroup map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&createdGroup)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w", err)
	}

	groupID, ok := createdGroup["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id")
	}
	return groupID, nil
}

// GetGroupMembers retrieves all members of a group
func GetGroupMembers(groupID string) ([]GroupMember, error) {
	// Use a large limit to get all members in one request
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/groups/%s/members", TestServerURL, groupID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get members request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var memberListResponse struct {
		TotalResults int           `json:"totalResults"`
		StartIndex   int           `json:"startIndex"`
		Count        int           `json:"count"`
		Members      []GroupMember `json:"members"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&memberListResponse); err != nil {
		return nil, fmt.Errorf("failed to decode members response: %w", err)
	}

	return memberListResponse.Members, nil
}

// DeleteGroup deletes a group by ID
func DeleteGroup(groupID string) error {
	req, err := http.NewRequest("DELETE", TestServerURL+"/groups/"+groupID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 204 or 200, got %d", resp.StatusCode)
	}
	return nil
}

// CreateRole creates a role via API and returns the role ID
func CreateRole(role Role) (string, error) {
	roleJSON, err := json.Marshal(role)
	if err != nil {
		return "", fmt.Errorf("failed to marshal role: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/roles", bytes.NewReader(roleJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create role: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		var errResp ErrorResponse
		_ = json.Unmarshal(respBody, &errResp)
		return "", fmt.Errorf("failed to create role, status %d: %s - %s", resp.StatusCode, errResp.Code, errResp.Message)
	}

	var createdRole Role
	if err := json.Unmarshal(respBody, &createdRole); err != nil {
		return "", fmt.Errorf("failed to unmarshal role response: %w", err)
	}

	return createdRole.ID, nil
}

// DeleteRole deletes a role by ID
func DeleteRole(roleID string) error {
	client := GetHTTPClient()

	// Step 1: Get all assignments for this role
	assignmentsResp, err := getRoleAssignments(roleID, client)
	if err != nil {
		return fmt.Errorf("failed to get role assignments: %w", err)
	}

	// Step 2: Remove all assignments if any exist
	if assignmentsResp != nil && len(assignmentsResp.Assignments) > 0 {
		if err := removeRoleAssignments(roleID, assignmentsResp.Assignments, client); err != nil {
			return fmt.Errorf("failed to remove role assignments: %w", err)
		}
	}

	// Step 3: Delete the role
	req, err := http.NewRequest("DELETE", TestServerURL+"/roles/"+roleID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204 or 200, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// AssignmentListResponse represents the paginated list of assignments
type AssignmentListResponse struct {
	TotalResults int          `json:"totalResults"`
	StartIndex   int          `json:"startIndex"`
	Count        int          `json:"count"`
	Assignments  []Assignment `json:"assignments"`
}

// GetRoleAssignments fetches all assignments for a role
func GetRoleAssignments(roleID string) ([]Assignment, error) {
	client := GetHTTPClient()
	resp, err := getRoleAssignments(roleID, client)
	if err != nil {
		return nil, err
	}
	return resp.Assignments, nil
}

// getRoleAssignments fetches all assignments for a role
func getRoleAssignments(roleID string, client *http.Client) (*AssignmentListResponse, error) {
	url := fmt.Sprintf("%s/roles/%s/assignments?offset=0&limit=100", TestServerURL, roleID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get assignments, status: %d", resp.StatusCode)
	}

	var assignmentsResp AssignmentListResponse
	if err := json.NewDecoder(resp.Body).Decode(&assignmentsResp); err != nil {
		return nil, err
	}

	return &assignmentsResp, nil
}

// removeRoleAssignments removes all assignments from a role
func removeRoleAssignments(roleID string, assignments []Assignment, client *http.Client) error {
	removeRequest := map[string]interface{}{
		"assignments": assignments,
	}

	body, err := json.Marshal(removeRequest)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/roles/%s/assignments/remove", TestServerURL, roleID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to remove assignments, status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// SimulateFederatedOAuthFlow simulates a federated OAuth flow (Google, GitHub, etc.) by
// following the redirect URL and extracting the authorization code and state parameter.
func SimulateFederatedOAuthFlow(redirectURL string) (string, string, error) {
	// Create HTTP client that doesn't follow redirects automatically
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // Don't follow redirects
		},
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// Make request to the authorization endpoint
	resp, err := client.Get(redirectURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to make authorization request: %w", err)
	}
	defer resp.Body.Close()

	// Check if we got a redirect response
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusSeeOther &&
		resp.StatusCode != http.StatusTemporaryRedirect {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("expected redirect response, got status %d: %s",
			resp.StatusCode, string(bodyBytes))
	}

	// Extract the Location header which contains the callback URL with the code
	location := resp.Header.Get("Location")
	if location == "" {
		return "", "", fmt.Errorf("no Location header in redirect response")
	}

	// Parse the location URL to extract the authorization code
	locationURL, err := url.Parse(location)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse location URL: %w", err)
	}

	// Extract the code parameter
	code := locationURL.Query().Get("code")
	if code == "" {
		return "", "", fmt.Errorf("no authorization code found in callback URL")
	}

	// Extract the state parameter
	state := locationURL.Query().Get("state")

	return code, state, nil
}

// ExtractStateFromRedirectURL extracts the OAuth state parameter from a redirect URL.
func ExtractStateFromRedirectURL(redirectURL string) string {
	parsedURL, err := url.Parse(redirectURL)
	if err != nil {
		return ""
	}

	return parsedURL.Query().Get("state")
}

// CreateResourceServerWithActions creates a resource server and multiple actions, returning the resource server ID
func CreateResourceServerWithActions(rs ResourceServer, actions []Action) (string, error) {
	// Create the resource server
	rsID, err := createResourceServer(rs)
	if err != nil {
		return "", fmt.Errorf("failed to create resource server: %w", err)
	}

	for i, action := range actions {
		_, err := createAction(rsID, action)
		if err != nil {
			// Cleanup: delete the resource server on failure
			DeleteResourceServer(rsID)
			return "", fmt.Errorf("failed to create action %d: %w", i, err)
		}
	}

	return rsID, nil
}

// createResourceServer creates a resource server via API and returns the resource server ID
func createResourceServer(rs ResourceServer) (string, error) {
	client := GetHTTPClient()

	rsJSON, err := json.Marshal(rs)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource server: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/resource-servers", bytes.NewReader(rsJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdRS ResourceServer
	if err := json.Unmarshal(bodyBytes, &createdRS); err != nil {
		return "", fmt.Errorf("failed to unmarshal resource server response: %w", err)
	}

	return createdRS.ID, nil
}

// GetResourceServerByIdentifier lists all resource servers and returns the ID of
// the first one whose identifier field matches the given identifier string.
func GetResourceServerByIdentifier(identifier string) (string, error) {
	client := GetHTTPClient()

	req, err := http.NewRequest("GET", TestServerURL+"/resource-servers", nil)
	if err != nil {
		return "", fmt.Errorf("failed to build list-resource-servers request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to list resource servers: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("list resource servers returned status %d: %s", resp.StatusCode, string(body))
	}

	// Minimal struct to extract from the paginated response
	var listResp struct {
		ResourceServers []struct {
			ID         string `json:"id"`
			Identifier string `json:"identifier"`
		} `json:"resourceServers"`
	}
	if err := json.Unmarshal(body, &listResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal resource servers response: %w", err)
	}

	for _, rs := range listResp.ResourceServers {
		if rs.Identifier == identifier {
			return rs.ID, nil
		}
	}

	return "", fmt.Errorf("resource server with identifier %q not found", identifier)
}

// GetResourceServerByName lists all resource servers and returns the ID of
// the first one whose name field matches the given name string.
func GetResourceServerByName(name string) (string, error) {
	client := GetHTTPClient()

	req, err := http.NewRequest("GET", TestServerURL+"/resource-servers", nil)
	if err != nil {
		return "", fmt.Errorf("failed to build list-resource-servers request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to list resource servers: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("list resource servers returned status %d: %s", resp.StatusCode, string(body))
	}

	// Minimal struct to extract from the paginated response
	var listResp struct {
		ResourceServers []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"resourceServers"`
	}
	if err := json.Unmarshal(body, &listResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal resource servers response: %w", err)
	}

	for _, rs := range listResp.ResourceServers {
		if rs.Name == name {
			return rs.ID, nil
		}
	}

	return "", fmt.Errorf("resource server with name %q not found", name)
}

func DeleteResourceServer(rsID string) error {
	client := GetHTTPClient()

	req, err := http.NewRequest("DELETE", TestServerURL+"/resource-servers/"+rsID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete resource server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// createAction creates an action on a resource server via API and returns the action ID
func createAction(resourceServerID string, action Action) (string, error) {
	client := GetHTTPClient()

	actionJSON, err := json.Marshal(action)
	if err != nil {
		return "", fmt.Errorf("failed to marshal action: %w", err)
	}

	url := fmt.Sprintf("%s/resource-servers/%s/actions", TestServerURL, resourceServerID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(actionJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdAction Action
	if err := json.Unmarshal(bodyBytes, &createdAction); err != nil {
		return "", fmt.Errorf("failed to unmarshal action response: %w", err)
	}

	return createdAction.ID, nil
}

// CreateFlow creates a flow via API and returns the flow ID
func CreateFlow(flowDefinition Flow) (string, error) {
	flowJSON, err := json.Marshal(flowDefinition)
	if err != nil {
		return "", fmt.Errorf("failed to marshal flow definition: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/flows", bytes.NewReader(flowJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var createdFlow map[string]interface{}
	err = json.Unmarshal(bodyBytes, &createdFlow)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	flowID, ok := createdFlow["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}
	return flowID, nil
}

// DeleteFlow deletes a flow by ID
func DeleteFlow(flowID string) error {
	req, err := http.NewRequest("DELETE", TestServerURL+"/flows/"+flowID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete flow: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}
	return nil
}

// GetFlowIDByHandle retrieves a flow ID by its handle and type
func GetFlowIDByHandle(handle string, flowType string) (string, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/flows?flowType=%s&limit=200", TestServerURL, flowType), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create flow list request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("flows list request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to list flows, status %d: %s", resp.StatusCode, string(body))
	}

	var flowsResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&flowsResp); err != nil {
		return "", fmt.Errorf("failed to decode flows response: %w", err)
	}

	flows, ok := flowsResp["flows"].([]interface{})
	if !ok {
		return "", fmt.Errorf("flows list format invalid")
	}

	for _, f := range flows {
		flow, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		if h, ok := flow["handle"].(string); ok && h == handle {
			if id, ok := flow["id"].(string); ok {
				return id, nil
			}
		}
	}

	return "", fmt.Errorf("flow with handle '%s' not found", handle)
}

// CreateNotificationSender creates a notification sender via API and returns the sender ID
func CreateNotificationSender(sender NotificationSender) (string, error) {
	senderJSON, err := json.Marshal(sender)
	if err != nil {
		return "", fmt.Errorf("failed to marshal notification sender: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/notification-senders/message",
		bytes.NewReader(senderJSON))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status 201, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}

	var respBody map[string]interface{}
	err = json.Unmarshal(bodyBytes, &respBody)
	if err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	id, ok := respBody["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}

	return id, nil
}

// DeleteNotificationSender deletes a notification sender by ID
func DeleteNotificationSender(senderID string) error {
	req, err := http.NewRequest("DELETE", TestServerURL+"/notification-senders/message/"+senderID, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete notification sender: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		responseBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200 or 204, got %d. Response: %s", resp.StatusCode, string(responseBody))
	}

	return nil
}

// AuthenticateWithCredential authenticates a user via the credentials endpoint.
// Returns (true, nil) on success, (false, nil) on auth failure, (false, err) on request error.
func AuthenticateWithCredential(identifierKey, identifierValue, credentialKey, credentialValue string) (bool, error) {
	reqBody := map[string]interface{}{
		"identifiers": map[string]interface{}{
			identifierKey: identifierValue,
		},
		"credentials": map[string]interface{}{
			credentialKey: credentialValue,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("failed to marshal auth request: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/auth/credentials/authenticate", bytes.NewReader(bodyBytes))
	if err != nil {
		return false, fmt.Errorf("failed to create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusBadRequest {
		return false, nil
	}
	body, _ := io.ReadAll(resp.Body)
	return false, fmt.Errorf("unexpected auth status %d: %s", resp.StatusCode, string(body))
}

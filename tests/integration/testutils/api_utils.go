// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	"reflect"
	"strings"
	"sync"
	"time"
)

// FlowSecretRegistry maps application IDs to their plaintext Flow Secrets as returned at creation
// time. Populated by CreateApplication; consumed by flow initiation helpers that need to present
// the Flow Secret on behalf of non-redirect test applications.
var (
	flowSecretRegistryMu sync.RWMutex
	flowSecretRegistry   = map[string]string{}
)

// GetFlowSecret returns the Flow Secret stored for an application ID, or "" if none.
func GetFlowSecret(appID string) string {
	flowSecretRegistryMu.RLock()
	defer flowSecretRegistryMu.RUnlock()
	return flowSecretRegistry[appID]
}

const (
	TestServerURL = "https://localhost:8095"

	// SystemResourceIdentifier is the identifier of the bootstrapped System resource server
	// (backend/cmd/server/bootstrap/01-default-resources.yaml). The CONSOLE app binds its tokens
	// to this resource server via the RFC 8707 resource parameter, mirroring the console runtime
	// config (frontend/apps/console/public/config.js).
	SystemResourceIdentifier = "https://localhost:8090/mcp"

	// FlowSecretHeaderName is the header used to present a Flow Secret to /flow/execute.
	FlowSecretHeaderName = "Flow-Secret"
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

// AgentTypeSnapshot captures the singleton `default` agent type so a suite that mutates it can put
// it back. It holds every mutable field, not just the schema: an entity-type update replaces the
// whole record, so any field left out of the restore payload is silently reset to its zero value.
// The schema in particular cannot come from the list endpoint, which omits it.
type AgentTypeSnapshot struct {
	ID                    string
	OUID                  string
	AllowSelfRegistration bool
	SystemAttributes      map[string]interface{}
	Schema                map[string]interface{}
}

// SnapshotAgentType reads the current `default` agent type from the detail endpoint. Suites that
// call CreateAgentType mutate a singleton every other suite shares, so they must snapshot before
// and RestoreAgentType after, otherwise the schema and OU they installed leak into later packages.
func SnapshotAgentType() (*AgentTypeSnapshot, error) {
	id, err := findDefaultAgentTypeID()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/agent-types/%s", TestServerURL, id), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent type request: %w", err)
	}

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch agent type: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read agent type response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status 200 fetching agent type, got %d: %s",
			resp.StatusCode, string(body))
	}

	var detail struct {
		ID                    string                 `json:"id"`
		OUID                  string                 `json:"ouId"`
		AllowSelfRegistration bool                   `json:"allowSelfRegistration"`
		SystemAttributes      map[string]interface{} `json:"systemAttributes"`
		Schema                map[string]interface{} `json:"schema"`
	}
	if err := json.Unmarshal(body, &detail); err != nil {
		return nil, fmt.Errorf("failed to parse agent type: %w", err)
	}

	return &AgentTypeSnapshot{
		ID:                    detail.ID,
		OUID:                  detail.OUID,
		AllowSelfRegistration: detail.AllowSelfRegistration,
		SystemAttributes:      detail.SystemAttributes,
		Schema:                detail.Schema,
	}, nil
}

// RestoreAgentType puts a snapshot back, then re-reads the type to confirm its OU resolves. If the
// snapshot's OU no longer exists, because a suite pointed the singleton at an OU it then deleted,
// the type is restored against the bootstrap `default` OU instead so the PUT cannot fail on a
// dangling reference.
func RestoreAgentType(snapshot *AgentTypeSnapshot) error {
	if snapshot == nil {
		return errors.New("agent type snapshot is nil")
	}

	ouID := snapshot.OUID
	if _, err := GetOrganizationUnit(ouID); err != nil {
		bootstrapOUID, lookupErr := findBootstrapOUID()
		if lookupErr != nil {
			return fmt.Errorf("snapshot OU %s is gone and the bootstrap OU is unavailable: %w",
				ouID, lookupErr)
		}
		ouID = bootstrapOUID
	}

	// Build the payload by hand rather than through UserType: its AllowSelfRegistration carries
	// `omitempty`, so a snapshotted `false` would be dropped, and it has no SystemAttributes field at
	// all. Either omission makes the server reset that field instead of restoring it.
	payload := map[string]interface{}{
		"name":                  "default",
		"ouId":                  ouID,
		"allowSelfRegistration": snapshot.AllowSelfRegistration,
		"schema":                snapshot.Schema,
	}
	if snapshot.SystemAttributes != nil {
		payload["systemAttributes"] = snapshot.SystemAttributes
	}

	if err := putAgentTypeRaw(snapshot.ID, payload); err != nil {
		return err
	}

	// Confirm the PUT actually applied. A 200 alone does not prove it: a partial or ignored update
	// would leave the calling suite's state installed and still look successful here.
	restored, err := SnapshotAgentType()
	if err != nil {
		return fmt.Errorf("failed to re-read the agent type after restoring it: %w", err)
	}
	if restored.ID != snapshot.ID {
		return fmt.Errorf("restored agent type has id %s, want %s", restored.ID, snapshot.ID)
	}
	if restored.OUID != ouID {
		return fmt.Errorf("restored agent type has ouId %s, want %s", restored.OUID, ouID)
	}
	if restored.AllowSelfRegistration != snapshot.AllowSelfRegistration {
		return fmt.Errorf("restored agent type has allowSelfRegistration %t, want %t",
			restored.AllowSelfRegistration, snapshot.AllowSelfRegistration)
	}
	if !reflect.DeepEqual(restored.SystemAttributes, snapshot.SystemAttributes) {
		return fmt.Errorf("restored agent type systemAttributes do not match the snapshot: got %v, want %v",
			restored.SystemAttributes, snapshot.SystemAttributes)
	}
	if !reflect.DeepEqual(restored.Schema, snapshot.Schema) {
		return fmt.Errorf("restored agent type schema does not match the snapshot: got %v, want %v",
			restored.Schema, snapshot.Schema)
	}
	if _, err := GetOrganizationUnit(restored.OUID); err != nil {
		return fmt.Errorf("restored agent type points at unresolvable OU %s: %w", restored.OUID, err)
	}
	return nil
}

// findBootstrapOUID resolves the long-lived `default` organization unit seeded at bootstrap.
func findBootstrapOUID() (string, error) {
	req, err := http.NewRequest("GET", TestServerURL+"/organization-units?limit=100", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create OU list request: %w", err)
	}

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to list organization units: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read OU list response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("expected status 200 listing organization units, got %d: %s",
			resp.StatusCode, string(body))
	}

	var list struct {
		OrganizationUnits []struct {
			ID     string `json:"id"`
			Handle string `json:"handle"`
		} `json:"organizationUnits"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return "", fmt.Errorf("failed to parse OU list: %w", err)
	}

	for _, ou := range list.OrganizationUnits {
		if ou.Handle == "default" {
			return ou.ID, nil
		}
	}
	return "", errors.New("bootstrap organization unit with handle 'default' not found")
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
	return putAgentTypeRaw(schemaID, schema)
}

// putAgentTypeRaw updates an agent type from an arbitrary payload, so a caller can send fields that
// the UserType struct drops or does not model.
func putAgentTypeRaw(schemaID string, body interface{}) error {
	payload, err := json.Marshal(body)
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

// ListUserTypes returns every user type visible to the caller, each with its schema.
//
// The listing endpoint omits the schema, so each type is fetched individually. Seeding reads every user
// type in the deployment, so tests asserting seeded defaults use this to check the precondition their
// expectation depends on rather than assuming no other suite left a type behind.
func ListUserTypes() ([]UserType, error) {
	body, err := getJSON(TestServerURL + "/user-types?limit=100")
	if err != nil {
		return nil, err
	}

	var listing struct {
		TotalResults int `json:"totalResults"`
		Types        []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"types"`
	}
	if err := json.Unmarshal(body, &listing); err != nil {
		return nil, fmt.Errorf("failed to parse user type listing: %w. Response: %s", err, string(body))
	}
	// Callers use this as a deployment-wide precondition, so a truncated page must not read as the whole
	// deployment.
	if listing.TotalResults > len(listing.Types) {
		return nil, fmt.Errorf("user type listing is truncated: %d of %d returned; raise the limit",
			len(listing.Types), listing.TotalResults)
	}

	userTypes := make([]UserType, 0, len(listing.Types))
	for _, item := range listing.Types {
		detail, err := getJSON(fmt.Sprintf("%s/user-types/%s", TestServerURL, item.ID))
		if err != nil {
			return nil, fmt.Errorf("failed to read user type %q: %w", item.Name, err)
		}
		var userType UserType
		if err := json.Unmarshal(detail, &userType); err != nil {
			return nil, fmt.Errorf("failed to parse user type %q: %w. Response: %s", item.Name, err, string(detail))
		}
		userTypes = append(userTypes, userType)
	}
	return userTypes, nil
}

// IsAttributeUnique reports whether the named schema attribute is declared unique on this user type.
func (u UserType) IsAttributeUnique(attribute string) bool {
	definition, ok := u.Schema[attribute].(map[string]interface{})
	if !ok {
		return false
	}
	unique, _ := definition["unique"].(bool)
	return unique
}

// getJSON performs an authenticated GET and returns the raw body, failing on any non-200 status.
func getJSON(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
	}
	return body, nil
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

	// The application type is required. Tests that do not care about the type default to full-stack,
	// whose flow behavior is derived from the OAuth config shape (matching what an untyped app used
	// to do). Tests exercising type-specific behavior set Type explicitly.
	appType := app.Type
	if appType == "" {
		appType = "fullstack"
	}

	inboundAuthConfig := app.InboundAuthConfig
	if len(inboundAuthConfig) == 0 && !app.Embedded {
		// Include token-exchange so the default test app is flow-native capable (eligible for a Flow
		// Secret and permitted to initiate flows), not a plain client_credentials-only M2M app.
		inboundAuthConfig = []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":     app.ClientID,
					"clientSecret": app.ClientSecret,
					"redirectUris": redirectURIs,
					"grantTypes":   []string{"client_credentials", "urn:ietf:params:oauth:grant-type:token-exchange"},
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
	}

	// Omit inboundAuthConfig entirely for embedded apps so they have no OAuth profile.
	if len(inboundAuthConfig) > 0 {
		appData["inboundAuthConfig"] = inboundAuthConfig
	}

	// Add allowed_user_types if provided
	if len(app.AllowedUserTypes) > 0 {
		appData["allowedUserTypes"] = app.AllowedUserTypes
	}

	// Add subject attribute mapping if provided
	if len(app.SubjectAttribute) > 0 {
		appData["subjectAttribute"] = app.SubjectAttribute
	}

	// Add assertion config if provided
	if app.AssertionConfig != nil {
		appData["assertion"] = app.AssertionConfig
	}

	// Add login consent config if provided
	if app.LoginConsent != nil {
		appData["loginConsent"] = app.LoginConsent
	}

	// Add the application type (explicit, or defaulted to full-stack above).
	appData["type"] = appType

	// Add client-level attestation config if provided
	if app.Attestation != nil {
		appData["attestation"] = app.Attestation
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

	if flowSecret, ok := createdApp["flowSecret"].(string); ok && flowSecret != "" {
		flowSecretRegistryMu.Lock()
		flowSecretRegistry[appID] = flowSecret
		flowSecretRegistryMu.Unlock()
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

// idpVendorRegistryMu guards idpVendorRegistry.
var idpVendorRegistryMu sync.RWMutex

// idpVendorRegistry maps an IDP ID (as returned by CreateIDP) to the /connections vendor path
// it was created under, so DeleteIDP (which only takes an ID) can address the right vendor
// route. Populated by CreateIDP.
var idpVendorRegistry = map[string]string{}

// idpVendorPath maps a legacy IDP Type value (e.g. "GOOGLE") to its /connections vendor path
// (e.g. "google").
func idpVendorPath(idpType string) (string, error) {
	switch strings.ToUpper(idpType) {
	case "GOOGLE":
		return "google", nil
	case "GITHUB":
		return "github", nil
	case "OIDC":
		return "oidc", nil
	case "OAUTH":
		return "oauth", nil
	default:
		return "", fmt.Errorf("unsupported IDP type for /connections: %s", idpType)
	}
}

// idpPropertyToConnectionField maps a legacy IDP property name to its /connections typed
// camelCase field name. Every IdP-backed vendor shares the same property key set; a vendor's
// request struct simply ignores fields it doesn't declare.
var idpPropertyToConnectionField = map[string]string{
	"client_id":              "clientId",
	"client_secret":          "clientSecret",
	"redirect_uri":           "redirectUri",
	"prompt":                 "prompt",
	"authorization_endpoint": "authorizationEndpoint",
	"token_endpoint":         "tokenEndpoint",
	"userinfo_endpoint":      "userInfoEndpoint",
	"jwks_endpoint":          "jwksEndpoint",
	"issuer":                 "issuer",
	"trusted_token_audience": "trustedTokenAudience",
}

// idpToConnectionBody converts a legacy IDP{Properties: [...]} fixture into the typed
// camelCase body /connections/{vendor} expects. "scopes" and "token_exchange_enabled" get
// special handling (comma-string -> array, string -> bool); every other recognized property
// name is copied through as a plain string. Unrecognized properties (not part of any
// /connections vendor's typed schema) are dropped.
func idpToConnectionBody(idp IDP) map[string]interface{} {
	body := map[string]interface{}{
		"name":        idp.Name,
		"description": idp.Description,
	}
	for _, prop := range idp.Properties {
		switch prop.Name {
		case "scopes":
			body["scopes"] = strings.Split(prop.Value, ",")
		case "token_exchange_enabled":
			body["tokenExchangeEnabled"] = prop.Value == "true"
		case "id_jag_enabled":
			body["idJagEnabled"] = prop.Value == "true"
		default:
			if field, ok := idpPropertyToConnectionField[prop.Name]; ok {
				body[field] = prop.Value
			}
		}
	}
	if idp.AttributeConfiguration != nil {
		body["attributeConfiguration"] = idp.AttributeConfiguration
	}
	return body
}

// connectionBodyToIDP converts a /connections/{vendor} JSON response back into the legacy
// IDP{Properties: [...]} shape.
func connectionBodyToIDP(idpType string, resp map[string]interface{}) *IDP {
	idp := &IDP{Type: strings.ToUpper(idpType)}
	if id, ok := resp["id"].(string); ok {
		idp.ID = id
	}
	if name, ok := resp["name"].(string); ok {
		idp.Name = name
	}
	if desc, ok := resp["description"].(string); ok {
		idp.Description = desc
	}
	for propName, field := range idpPropertyToConnectionField {
		if value, ok := resp[field].(string); ok && value != "" {
			idp.Properties = append(idp.Properties, IDPProperty{Name: propName, Value: value})
		}
	}
	if scopes, ok := resp["scopes"].([]interface{}); ok {
		parts := make([]string, 0, len(scopes))
		for _, s := range scopes {
			if str, ok := s.(string); ok {
				parts = append(parts, str)
			}
		}
		idp.Properties = append(idp.Properties, IDPProperty{Name: "scopes", Value: strings.Join(parts, ",")})
	}
	// Round-tripped through JSON because the response arrives as a generic map. A malformed section is
	// left nil rather than failing the conversion, so callers assert on it instead of on a parse error.
	if raw, ok := resp["attributeConfiguration"]; ok && raw != nil {
		if encoded, err := json.Marshal(raw); err == nil {
			var config AttributeConfiguration
			if err := json.Unmarshal(encoded, &config); err == nil {
				idp.AttributeConfiguration = &config
			}
		}
	}
	return idp
}

// CreateIDP creates an identity provider via /connections/{vendor} and returns its ID.
func CreateIDP(idp IDP) (string, error) {
	vendor, err := idpVendorPath(idp.Type)
	if err != nil {
		return "", err
	}

	bodyJSON, err := json.Marshal(idpToConnectionBody(idp))
	if err != nil {
		return "", fmt.Errorf("failed to marshal connection body: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/connections/"+vendor, bytes.NewReader(bodyJSON))
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

	var created map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &created); err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	idpID, ok := created["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}

	idpVendorRegistryMu.Lock()
	idpVendorRegistry[idpID] = vendor
	idpVendorRegistryMu.Unlock()

	return idpID, nil
}

// DeleteIDP deletes an identity provider (created via CreateIDP) by ID.
func DeleteIDP(idpID string) error {
	idpVendorRegistryMu.RLock()
	vendor, ok := idpVendorRegistry[idpID]
	idpVendorRegistryMu.RUnlock()
	if !ok {
		return fmt.Errorf("no /connections vendor registered for IDP ID %q — "+
			"it was not created via CreateIDP in this process", idpID)
	}

	req, err := http.NewRequest("DELETE", TestServerURL+"/connections/"+vendor+"/"+idpID, nil)
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

	idpVendorRegistryMu.Lock()
	delete(idpVendorRegistry, idpID)
	idpVendorRegistryMu.Unlock()
	return nil
}

// GetIDP retrieves an identity provider by vendor type and ID via /connections/{vendor}/{id}.
func GetIDP(idpType, idpID string) (*IDP, error) {
	vendor, err := idpVendorPath(idpType)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/connections/%s/%s", TestServerURL, vendor, idpID), nil)
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

	var respBody map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return nil, fmt.Errorf("failed to decode IDP response: %w", err)
	}

	return connectionBodyToIDP(idpType, respBody), nil
}

// UpdateIDP updates an existing identity provider via /connections/{vendor}/{id}. The vendor is
// derived from idp.Type.
func UpdateIDP(idpID string, idp IDP) error {
	vendor, err := idpVendorPath(idp.Type)
	if err != nil {
		return err
	}

	bodyJSON, err := json.Marshal(idpToConnectionBody(idp))
	if err != nil {
		return fmt.Errorf("failed to marshal connection body: %w", err)
	}

	req, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s/connections/%s/%s", TestServerURL, vendor, idpID),
		bytes.NewReader(bodyJSON),
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
			// Roll back through the child-aware delete: any action created before this one blocks a
			// plain resource-server delete, so a partially built server would otherwise survive.
			return "", rollbackResourceServer(rsID, fmt.Errorf("failed to create action %d: %w", i, err))
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

// DeleteResourceServer deletes a resource server that owns no resources or actions. The server
// refuses the delete with RES-1006 while any dependency remains, so a server built with resources or
// actions must go through DeleteResourceServerWithChildren instead.
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

// CreateAction creates an action on a resource server and returns the action ID.
func CreateAction(resourceServerID string, action Action) (string, error) {
	return createAction(resourceServerID, action)
}

// GetActionsByResourceServer returns the IDs of the actions defined on a resource server.
func GetActionsByResourceServer(resourceServerID string) ([]string, error) {
	client := GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/actions", TestServerURL, resourceServerID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create list-actions request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list actions: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
	}

	var listResp struct {
		Actions []struct {
			ID string `json:"id"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal actions response: %w", err)
	}

	actionIDs := make([]string, 0, len(listResp.Actions))
	for _, action := range listResp.Actions {
		actionIDs = append(actionIDs, action.ID)
	}
	return actionIDs, nil
}

// DeleteAction deletes an action from a resource server.
func DeleteAction(resourceServerID, actionID string) error {
	client := GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/actions/%s", TestServerURL, resourceServerID, actionID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete action: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

func PutDefaultResourceServer(resourceServerID string) error {
	client := GetHTTPClient()

	payload, err := json.Marshal(map[string]string{
		"resourceServerId": resourceServerID,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal default resource server config: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, TestServerURL+"/server-config/defaultResourceServer", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create default resource server config request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to update default resource server config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 200, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// GetWritableServerConfig returns the writable layer of the named server-config section, so a
// caller can restore exactly what was there rather than guessing at a default. An absent layer is
// reported as an empty object.
func GetWritableServerConfig(section string) (json.RawMessage, error) {
	url := fmt.Sprintf("%s/server-config/%s", TestServerURL, section)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s server config request: %w", section, err)
	}

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s server config: %w", section, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s server config response: %w", section, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expected status 200 reading %s server config, got %d. Response: %s",
			section, resp.StatusCode, string(body))
	}

	var layers struct {
		Writable json.RawMessage `json:"writable"`
	}
	if err := json.Unmarshal(body, &layers); err != nil {
		return nil, fmt.Errorf("failed to parse %s server config: %w", section, err)
	}

	if len(layers.Writable) == 0 {
		return json.RawMessage("{}"), nil
	}
	return layers.Writable, nil
}

// PutWritableServerConfig replaces the writable layer of the named server-config section.
func PutWritableServerConfig(section string, body []byte) error {
	url := fmt.Sprintf("%s/server-config/%s", TestServerURL, section)

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create %s server config request: %w", section, err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to update %s server config: %w", section, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read %s server config response: %w", section, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200 updating %s server config with %s, got %d. Response: %s",
			section, string(body), resp.StatusCode, string(respBody))
	}
	return nil
}

// MergeWritableServerConfig applies the given top-level keys over the current writable layer of the
// section and writes the result back, returning the layer as it was beforehand.
//
// Merging rather than replacing keeps sibling keys that the caller did not set, which a bare PUT
// would drop. The returned value is what the caller restores when it is done.
func MergeWritableServerConfig(section string, patch map[string]interface{}) (json.RawMessage, error) {
	original, err := GetWritableServerConfig(section)
	if err != nil {
		return nil, err
	}

	merged := make(map[string]interface{})
	if err := json.Unmarshal(original, &merged); err != nil {
		return nil, fmt.Errorf("failed to parse writable %s server config: %w", section, err)
	}
	for key, value := range patch {
		merged[key] = value
	}

	body, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s server config: %w", section, err)
	}
	if err := PutWritableServerConfig(section, body); err != nil {
		return nil, err
	}
	return original, nil
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

// CreateResource creates a resource under a resource server via API and returns the created
// resource ID. parentID may be empty to create a top-level resource.
func CreateResource(resourceServerID, name, handle, parentID string) (string, error) {
	client := GetHTTPClient()

	body := map[string]interface{}{"name": name, "handle": handle}
	if parentID != "" {
		body["parent"] = parentID
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal resource: %w", err)
	}

	url := fmt.Sprintf("%s/resource-servers/%s/resources", TestServerURL, resourceServerID)
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
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

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(bodyBytes, &created); err != nil {
		return "", fmt.Errorf("failed to unmarshal resource response: %w", err)
	}
	return created.ID, nil
}

// createActionUnderResource creates an action nested under a resource and returns the action ID.
func createActionUnderResource(resourceServerID, resourceID string, action Action) (string, error) {
	client := GetHTTPClient()

	actionJSON, err := json.Marshal(action)
	if err != nil {
		return "", fmt.Errorf("failed to marshal action: %w", err)
	}

	url := fmt.Sprintf("%s/resource-servers/%s/resources/%s/actions", TestServerURL, resourceServerID, resourceID)
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

	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(bodyBytes, &created); err != nil {
		return "", fmt.Errorf("failed to unmarshal action response: %w", err)
	}
	return created.ID, nil
}

// CreateSystemScopedResourceServer creates a custom resource server that reproduces the
// hierarchical "system:<handle>:view" permission strings used by the built-in system management
// APIs. The product ships only the root "system" scope by default; this helper simulates an
// operator declaring fine-grained scopes, letting the authz suites verify that resource-level
// permissions still enforce when configured. It builds a "system" root resource, then one child
// resource per handle (each with a "view" action), yielding the permissions "system",
// "system:<handle>" and "system:<handle>:view". Returns the resource server ID; delete it with
// DeleteResourceServerWithChildren during teardown — a plain DeleteResourceServer is refused with
// RES-1006 while the resources and actions built here still exist.
func CreateSystemScopedResourceServer(ouID, name, identifier string, childHandles ...string) (string, error) {
	rsID, err := createResourceServer(ResourceServer{
		Name:       name,
		Identifier: identifier,
		OUID:       ouID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create resource server: %w", err)
	}

	systemID, err := CreateResource(rsID, "System", "system", "")
	if err != nil {
		return "", rollbackResourceServer(rsID, fmt.Errorf("failed to create system resource: %w", err))
	}

	for _, handle := range childHandles {
		childID, err := CreateResource(rsID, handle, handle, systemID)
		if err != nil {
			return "", rollbackResourceServer(rsID, fmt.Errorf("failed to create %q resource: %w", handle, err))
		}
		if _, err := createActionUnderResource(rsID, childID, Action{Name: "View", Handle: "view"}); err != nil {
			return "", rollbackResourceServer(rsID, fmt.Errorf("failed to create view action for %q: %w", handle, err))
		}
	}

	return rsID, nil
}

// rollbackResourceServer deletes a partially built resource server after a setup step failed. It
// returns the original cause, wrapping any cleanup failure so neither error is silently discarded.
func rollbackResourceServer(rsID string, cause error) error {
	if delErr := DeleteResourceServerWithChildren(rsID); delErr != nil {
		return fmt.Errorf("%w (resource server cleanup also failed: %v)", cause, delErr)
	}
	return cause
}

// DeleteResourceServerWithChildren removes a resource server together with its resource tree and
// every action it owns. A plain DELETE on a resource server that still has dependencies is refused
// with RES-1006, so any server built by CreateSystemScopedResourceServer or
// CreateResourceServerWithActions must be torn down through here or it survives in the shared
// database.
//
// Actions live at two levels and both block deletion: CreateSystemScopedResourceServer attaches them
// to resources, while CreateResourceServerWithActions attaches them directly to the server.
func DeleteResourceServerWithChildren(rsID string) error {
	// Collect the tree depth-first so children are always deleted before their parents. The list
	// endpoint returns only one level: without a parentId it yields the top-level resources, so the
	// nested ones are invisible unless each level is walked explicitly.
	ordered, err := collectResourceIDsDeepestFirst(rsID, "")
	if err != nil {
		return err
	}

	for _, resourceID := range ordered {
		actions, actionErr := ListActionIDsAtResource(rsID, resourceID)
		if actionErr != nil {
			return actionErr
		}
		for _, actionID := range actions {
			if delErr := deleteActionAtResource(rsID, resourceID, actionID); delErr != nil {
				return delErr
			}
		}
		if delErr := deleteResource(rsID, resourceID); delErr != nil {
			return fmt.Errorf("failed to delete resource %s of resource server %s: %w",
				resourceID, rsID, delErr)
		}
	}

	serverActions, err := ListActionIDsAtResourceServer(rsID)
	if err != nil {
		return err
	}
	for _, actionID := range serverActions {
		if delErr := DeleteAction(rsID, actionID); delErr != nil {
			return fmt.Errorf("failed to delete action %s of resource server %s: %w",
				actionID, rsID, delErr)
		}
	}

	return DeleteResourceServer(rsID)
}

// ListActionIDsAtResourceServer returns the IDs of actions attached directly to a resource server,
// as opposed to those attached to one of its resources.
func ListActionIDsAtResourceServer(rsID string) ([]string, error) {
	body, err := getJSON(fmt.Sprintf("%s/resource-servers/%s/actions?limit=100", TestServerURL, rsID))
	if err != nil {
		return nil, fmt.Errorf("failed to list actions of resource server %s: %w", rsID, err)
	}

	var list struct {
		Actions []struct {
			ID string `json:"id"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("failed to parse action list: %w", err)
	}

	ids := make([]string, 0, len(list.Actions))
	for _, action := range list.Actions {
		ids = append(ids, action.ID)
	}
	return ids, nil
}

// collectResourceIDsDeepestFirst returns the resource subtree under parentID, children ahead of
// their parents.
func collectResourceIDsDeepestFirst(rsID, parentID string) ([]string, error) {
	children, err := ListResourceIDs(rsID, parentID)
	if err != nil {
		return nil, err
	}

	ordered := make([]string, 0, len(children))
	for _, child := range children {
		descendants, descErr := collectResourceIDsDeepestFirst(rsID, child)
		if descErr != nil {
			return nil, descErr
		}
		ordered = append(ordered, descendants...)
		ordered = append(ordered, child)
	}
	return ordered, nil
}

// ListResourceIDs returns the IDs of resources directly under parentID. An empty parentID lists the
// top-level resources, which requires omitting the query parameter entirely: sending `parentId=`
// makes the server look up a resource whose id is the empty string and answer 404.
func ListResourceIDs(rsID, parentID string) ([]string, error) {
	requestURL := fmt.Sprintf("%s/resource-servers/%s/resources?limit=100", TestServerURL, rsID)
	if parentID != "" {
		requestURL += "&parentId=" + url.QueryEscape(parentID)
	}

	body, err := getJSON(requestURL)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources of resource server %s: %w", rsID, err)
	}

	var list struct {
		Resources []struct {
			ID string `json:"id"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("failed to parse resource list: %w", err)
	}

	ids := make([]string, 0, len(list.Resources))
	for _, resource := range list.Resources {
		ids = append(ids, resource.ID)
	}
	return ids, nil
}

// ListActionIDsAtResource returns the IDs of actions defined directly on a resource.
func ListActionIDsAtResource(rsID, resourceID string) ([]string, error) {
	body, err := getJSON(fmt.Sprintf("%s/resource-servers/%s/resources/%s/actions?limit=100",
		TestServerURL, rsID, resourceID))
	if err != nil {
		return nil, fmt.Errorf("failed to list actions of resource %s: %w", resourceID, err)
	}

	var list struct {
		Actions []struct {
			ID string `json:"id"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("failed to parse action list: %w", err)
	}

	ids := make([]string, 0, len(list.Actions))
	for _, action := range list.Actions {
		ids = append(ids, action.ID)
	}
	return ids, nil
}

func deleteActionAtResource(rsID, resourceID, actionID string) error {
	return deleteURL(fmt.Sprintf("%s/resource-servers/%s/resources/%s/actions/%s",
		TestServerURL, rsID, resourceID, actionID))
}

func deleteResource(rsID, resourceID string) error {
	return deleteURL(fmt.Sprintf("%s/resource-servers/%s/resources/%s",
		TestServerURL, rsID, resourceID))
}

// deleteURL issues a DELETE and requires a 204.
func deleteURL(url string) error {
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(body))
	}
	return nil
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

// CreateIsolatedAuthFlow creates a minimal AUTHENTICATION flow with no CALL nodes, suitable for
// tests that attach a custom registration/recovery/signout flow to an application: reusing the
// default auth flow would trigger cross-type reference validation (APP-1039) because it CALLs the
// default registration and recovery flows. The handle is caller-supplied so tests can craft unique
// values per suite and clean up deterministically.
func CreateIsolatedAuthFlow(handle string) (string, error) {
	return CreateFlow(Flow{
		Name:     "Isolated Auth Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	})
}

// CreateIsolatedRegistrationFlow creates a minimal REGISTRATION flow suitable for tests that need
// an app with IsRegistrationFlowEnabled set without triggering cross-type reference validation.
// The handle is caller-supplied so tests can craft unique values per suite and clean up
// deterministically.
func CreateIsolatedRegistrationFlow(handle string) (string, error) {
	return CreateFlow(Flow{
		Name:     "Isolated Registration Flow " + handle,
		FlowType: "REGISTRATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "user_type_resolver",
			},
			{
				"id":   "user_type_resolver",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "UserTypeResolver",
				},
				"onSuccess": "provisioning",
			},
			{
				"id":   "provisioning",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "ProvisioningExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	})
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

// senderVendorRegistryMu guards senderVendorRegistry.
var senderVendorRegistryMu sync.RWMutex

// senderVendorRegistry maps a notification sender ID (as returned by CreateNotificationSender)
// to the /connections vendor path it was created under, so DeleteNotificationSender (which only
// takes an ID) can address the right vendor route. Populated by CreateNotificationSender.
var senderVendorRegistry = map[string]string{}

// senderVendorPath maps a legacy NotificationSender.Provider value (e.g. "custom") to its
// /connections vendor path (e.g. "sms-gateway").
func senderVendorPath(provider string) (string, error) {
	switch provider {
	case "twilio":
		return "twilio", nil
	case "vonage":
		return "vonage", nil
	case "custom":
		return "sms-gateway", nil
	default:
		return "", fmt.Errorf("unsupported notification sender provider for /connections: %s", provider)
	}
}

// senderPropertyToConnectionField maps a legacy NotificationSender property name to its
// /connections typed camelCase field name, across all sender-backed vendors.
var senderPropertyToConnectionField = map[string]string{
	"account_sid":  "accountSid",
	"auth_token":   "authToken",
	"api_key":      "apiKey",
	"api_secret":   "apiSecret",
	"sender_id":    "senderId",
	"url":          "url",
	"http_method":  "httpMethod",
	"http_headers": "httpHeaders",
	"content_type": "contentType",
}

// senderToConnectionBody converts a legacy NotificationSender{Properties: [...]} fixture into
// the typed camelCase body /connections/{vendor} expects for sender-backed vendors.
func senderToConnectionBody(sender NotificationSender) map[string]interface{} {
	body := map[string]interface{}{
		"name":        sender.Name,
		"description": sender.Description,
	}
	for _, prop := range sender.Properties {
		if field, ok := senderPropertyToConnectionField[prop.Name]; ok {
			body[field] = prop.Value
		}
	}
	return body
}

// CreateNotificationSender creates a notification sender via /connections/{vendor} and returns
// its ID.
func CreateNotificationSender(sender NotificationSender) (string, error) {
	vendor, err := senderVendorPath(sender.Provider)
	if err != nil {
		return "", err
	}

	bodyJSON, err := json.Marshal(senderToConnectionBody(sender))
	if err != nil {
		return "", fmt.Errorf("failed to marshal connection body: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/connections/"+vendor, bytes.NewReader(bodyJSON))
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
	if err := json.Unmarshal(bodyBytes, &respBody); err != nil {
		return "", fmt.Errorf("failed to parse response body: %w. Response: %s", err, string(bodyBytes))
	}

	id, ok := respBody["id"].(string)
	if !ok {
		return "", fmt.Errorf("response does not contain id or id is not a string. Response: %s", string(bodyBytes))
	}

	senderVendorRegistryMu.Lock()
	senderVendorRegistry[id] = vendor
	senderVendorRegistryMu.Unlock()

	return id, nil
}

// DeleteNotificationSender deletes a notification sender (created via CreateNotificationSender)
// by ID.
func DeleteNotificationSender(senderID string) error {
	senderVendorRegistryMu.RLock()
	vendor, ok := senderVendorRegistry[senderID]
	senderVendorRegistryMu.RUnlock()
	if !ok {
		return fmt.Errorf("no /connections vendor registered for sender ID %q — "+
			"it was not created via CreateNotificationSender in this process", senderID)
	}

	req, err := http.NewRequest("DELETE", TestServerURL+"/connections/"+vendor+"/"+senderID, nil)
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

	senderVendorRegistryMu.Lock()
	delete(senderVendorRegistry, senderID)
	senderVendorRegistryMu.Unlock()
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

// CreateAgent creates an agent via API and returns its ID.
func CreateAgent(a Agent) (string, error) {
	payload, err := json.Marshal(a)
	if err != nil {
		return "", fmt.Errorf("failed to marshal agent: %w", err)
	}

	req, err := http.NewRequest("POST", TestServerURL+"/agents", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("failed to create agent request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("create agent request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read create agent response body: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("create agent failed with status %d: %s", resp.StatusCode, string(body))
	}

	var created Agent
	if err := json.Unmarshal(body, &created); err != nil {
		return "", fmt.Errorf("failed to decode create agent response: %w", err)
	}
	return created.ID, nil
}

// DeleteAgent deletes an agent by ID via API.
func DeleteAgent(agentID string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/agents/%s", TestServerURL, agentID), nil)
	if err != nil {
		return fmt.Errorf("failed to create delete agent request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("delete agent request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("delete agent failed with status %d (failed to read body: %w)", resp.StatusCode, err)
		}
		return fmt.Errorf("delete agent failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetAgent retrieves an agent by ID via API.
func GetAgent(agentID string) (*Agent, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/agents/%s", TestServerURL, agentID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create get agent request: %w", err)
	}

	client := GetHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get agent request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read get agent response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get agent failed with status %d: %s", resp.StatusCode, string(body))
	}

	var a Agent
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, fmt.Errorf("failed to decode get agent response: %w", err)
	}
	return &a, nil
}

// UpdateRole replaces a role by ID. Used to change a role's permissions or assignments after tokens
// have been issued against them.
func UpdateRole(roleID string, role Role) error {
	roleJSON, err := json.Marshal(role)
	if err != nil {
		return fmt.Errorf("failed to marshal role: %w", err)
	}

	req, err := http.NewRequest("PUT", TestServerURL+"/roles/"+roleID, bytes.NewReader(roleJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to update role: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update role, status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// UpdateUserCredentials sets new credentials (e.g. a password) for a user, the way an administrative
// password reset does.
func UpdateUserCredentials(userID string, credentials map[string]string) error {
	credsJSON, err := json.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}
	payload, err := json.Marshal(map[string]json.RawMessage{"credentials": credsJSON})
	if err != nil {
		return fmt.Errorf("failed to marshal credential update request: %w", err)
	}

	req, err := http.NewRequest("POST",
		TestServerURL+"/users/"+userID+"/update-credentials", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to update user credentials: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update user credentials, status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// UpdateApplication replaces an application by ID. Supplying a new client secret on the OAuth inbound
// auth config rotates it.
func UpdateApplication(appID string, app Application) error {
	appJSON, err := json.Marshal(app)
	if err != nil {
		return fmt.Errorf("failed to marshal application: %w", err)
	}

	req, err := http.NewRequest("PUT", TestServerURL+"/applications/"+appID, bytes.NewReader(appJSON))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := GetHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("failed to update application: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update application, status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// RemoveRoleAssignments unassigns the given subjects from a role, the way an administrator revoking a
// user's role does. Assignments are a sub-resource of the role, so a role update does not carry them.
func RemoveRoleAssignments(roleID string, assignments []Assignment) error {
	return removeRoleAssignments(roleID, assignments, GetHTTPClient())
}

// DeleteResource deletes a resource from a resource server. A resource server cannot be deleted
// while it still has resources, so suites that build a resource tree must remove it leaf first.
func DeleteResource(resourceServerID, resourceID string) error {
	client := GetHTTPClient()

	url := fmt.Sprintf("%s/resource-servers/%s/resources/%s", TestServerURL, resourceServerID, resourceID)
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete resource: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("expected status 204, got %d. Response: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

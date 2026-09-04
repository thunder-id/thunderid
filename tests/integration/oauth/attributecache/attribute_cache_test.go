// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package attributecache verifies the OAuth attribute-cache round-trip: during an authorization_code
// flow the resolved user attributes are cached and referenced from tokens via the aci claim, then
// resolved back into the id token, a refreshed access token, and the userinfo response. It runs the
// round-trip in both attribute-cache modes: plaintext (the default) and encrypted (AES-GCM
// encrypt-on-write / decrypt-on-read, gated by attribute_cache.encryption.enabled). Both modes must
// yield identical claims, so the encrypted case flips the toggle for the duration of its test and
// restores the default before returning, keeping the shared server pristine for other packages.
package attributecache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	testServerURL            = "https://localhost:8095"
	clientID                 = "attr_cache_client"
	clientSecret             = "attr_cache_secret"
	appName                  = "AttributeCacheTestApp"
	redirectURI              = "https://localhost:3000"
	resourceServerIdentifier = "https://attr-cache.example.com"

	testUsername  = "attr_cache_user"
	testPassword  = "SecurePass123!"
	testEmail     = "attr_cache@example.com"
	testGivenName = "AttrCache"
	testFamily    = "RoundTrip"
)

var testUserType = testutils.UserType{
	Name: "attr-cache-person",
	Schema: map[string]interface{}{
		"username":    map[string]interface{}{"type": "string"},
		"password":    map[string]interface{}{"type": "string", "credential": true},
		"email":       map[string]interface{}{"type": "string"},
		"given_name":  map[string]interface{}{"type": "string"},
		"family_name": map[string]interface{}{"type": "string"},
	},
}

// encryptionPatch builds a deployment-config patch that toggles attribute-cache encryption. The
// integration fixture omits the section (default disabled), so restoring to disabled matches the
// pristine config other suites rely on.
func encryptionPatch(enabled bool) map[string]interface{} {
	return map[string]interface{}{
		"attribute_cache": map[string]interface{}{
			"encryption": map[string]interface{}{"enabled": enabled},
		},
	}
}

type AttributeCacheTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	entityTypeID     string
	userID           string
	flowID           string
	applicationID    string
	resourceServerID string
}

func TestAttributeCacheTestSuite(t *testing.T) {
	suite.Run(t, new(AttributeCacheTestSuite))
}

func (ts *AttributeCacheTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "attr-cache-ou",
		Name:        "Attribute Cache OU",
		Description: "Organization unit for attribute cache integration testing",
		Parent:      nil,
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	testUserType.OUID = ts.ouID
	entityTypeID, err := testutils.CreateUserType(testUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = entityTypeID

	ts.userID = ts.createTestUser()

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Attribute Cache Resource Server",
		Description: "Resource server for attribute cache integration tests",
		Identifier:  resourceServerIdentifier,
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "Failed to create resource server")
	ts.resourceServerID = resourceServerID

	ts.flowID = ts.createTestAuthenticationFlow()
	ts.applicationID = ts.createTestApplication(ts.flowID)
}

func (ts *AttributeCacheTestSuite) TearDownSuite() {
	if ts.applicationID != "" {
		ts.deleteApplication(ts.applicationID)
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server: %v", err)
		}
	}
	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("Failed to delete authentication flow during teardown: %v", err)
		}
	}
	if ts.userID != "" {
		testutils.DeleteUser(ts.userID)
	}
	if ts.ouID != "" {
		testutils.DeleteOrganizationUnit(ts.ouID)
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
}

func (ts *AttributeCacheTestSuite) createTestUser() string {
	attributes := map[string]interface{}{
		"username":    testUsername,
		"password":    testPassword,
		"email":       testEmail,
		"given_name":  testGivenName,
		"family_name": testFamily,
	}
	attributesJSON, err := json.Marshal(attributes)
	ts.Require().NoError(err, "Failed to marshal user attributes")

	userID, err := testutils.CreateUser(testutils.User{
		Type:       testUserType.Name,
		OUID:       ts.ouID,
		Attributes: json.RawMessage(attributesJSON),
	})
	ts.Require().NoError(err, "Failed to create test user")
	return userID
}

func (ts *AttributeCacheTestSuite) createTestAuthenticationFlow() string {
	flow := testutils.Flow{
		Name:     "Attribute Cache Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "attr_cache_auth_flow",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
							{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
					},
				},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":        "auth_assert",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}

	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	return flowID
}

func (ts *AttributeCacheTestSuite) createTestApplication(authFlowID string) string {
	app := map[string]interface{}{
		"name":                      appName,
		"description":               "Application for attribute cache integration tests",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{testUserType.Name},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"redirectUris":            []string{redirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_post",
					"scopes":                  []string{"openid", "profile", "email"},
					"token": map[string]interface{}{
						"idToken": map[string]interface{}{
							"userAttributes": []string{"email", "given_name", "family_name"},
						},
						"userInfo": map[string]interface{}{
							"userAttributes": []string{"email", "given_name", "family_name"},
						},
					},
					"scopeClaims": map[string][]string{
						"profile": {"given_name", "family_name", "name"},
						"email":   {"email"},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err, "Failed to marshal application data")

	req, err := http.NewRequest("POST", testServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to create application")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusCreated, resp.StatusCode, "Failed to create application")

	var respData map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&respData), "Failed to parse response")
	return respData["id"].(string)
}

func (ts *AttributeCacheTestSuite) deleteApplication(appID string) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/applications/%s", testServerURL, appID), nil)
	if err != nil {
		ts.T().Errorf("Failed to create delete request: %v", err)
		return
	}
	resp, err := ts.client.Do(req)
	if err != nil {
		ts.T().Errorf("Failed to delete application: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Errorf("Failed to delete application. Status: %d, Response: %s", resp.StatusCode, string(bodyBytes))
	}
}

func (ts *AttributeCacheTestSuite) callUserInfo(accessToken string) map[string]interface{} {
	req, err := http.NewRequest("GET", testServerURL+"/oauth2/userinfo", nil)
	ts.Require().NoError(err, "Failed to create userinfo request")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to call UserInfo endpoint")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusOK, resp.StatusCode, "UserInfo should return 200")

	var userInfo map[string]interface{}
	ts.Require().NoError(json.NewDecoder(resp.Body).Decode(&userInfo), "Failed to parse UserInfo response")
	return userInfo
}

// assertUserClaims asserts the three cached user attributes are present with their expected values
// in the given claims/userinfo map. where labels the source for failure messages.
func (ts *AttributeCacheTestSuite) assertUserClaims(claims map[string]interface{}, where string) {
	ts.Equal(testEmail, claims["email"], where+" email should match")
	ts.Equal(testGivenName, claims["given_name"], where+" given_name should match")
	ts.Equal(testFamily, claims["family_name"], where+" family_name should match")
}

// assertRoundTrip drives the full attribute-cache round-trip and asserts the cached user attributes
// surface identically in (1) the id token, (2) the userinfo response, and (3) the userinfo response
// after a refresh_token grant, while the access token only references the cache via the aci claim.
// It is mode-agnostic: the caller runs it with encryption on or off and expects the same result.
func (ts *AttributeCacheTestSuite) assertRoundTrip() {
	// No resource indicator is requested so the token's audience is the client's own default
	// audience, which the UserInfo endpoint accepts.
	tokens, err := testutils.ObtainAccessTokenWithPassword(clientID, redirectURI, "openid profile email",
		testUsername, testPassword, false, clientSecret)
	ts.Require().NoError(err, "failed to obtain tokens via authorization_code flow")
	ts.Require().NotEmpty(tokens.AccessToken, "access token should not be empty")
	ts.Require().NotEmpty(tokens.IDToken, "id token should not be empty")
	ts.Require().NotEmpty(tokens.RefreshToken, "refresh token should not be empty")

	// (1) id token carries the user attributes resolved from the cache.
	idClaims, err := testutils.DecodeJWT(tokens.IDToken)
	ts.Require().NoError(err, "failed to decode id token")
	ts.assertUserClaims(idClaims.Additional, "id token")

	// The access token references the cache via aci; attributes are not embedded directly.
	atClaims, err := testutils.DecodeJWT(tokens.AccessToken)
	ts.Require().NoError(err, "failed to decode access token")
	ts.NotEmpty(atClaims.Additional["aci"], "access token should carry the aci claim")

	// (2) userinfo resolves the same attributes from the cache.
	ts.assertUserClaims(ts.callUserInfo(tokens.AccessToken), "userinfo")

	// (3) a refreshed access token still resolves the cached attributes.
	refreshed, err := testutils.RefreshAccessTokenWithClientCredentialsInBody(clientID, clientSecret, tokens.RefreshToken)
	ts.Require().NoError(err, "failed to refresh access token")
	ts.Require().NotEmpty(refreshed.AccessToken, "refreshed access token should not be empty")
	ts.assertUserClaims(ts.callUserInfo(refreshed.AccessToken), "userinfo after refresh")
}

// TestAttributeCache_Plaintext runs the round-trip with encryption disabled (the default), where the
// attributes are cached as plaintext.
func (ts *AttributeCacheTestSuite) TestAttributeCache_Plaintext() {
	ts.assertRoundTrip()
}

// TestAttributeCache_Encrypted runs the same round-trip with encryption enabled, proving the AES-GCM
// encrypt-on-write / decrypt-on-read path yields identical attributes. It flips the toggle for the
// duration of the test and restores the default (disabled) config before returning, so the shared
// server stays pristine for the other packages.
func (ts *AttributeCacheTestSuite) TestAttributeCache_Encrypted() {
	ts.Require().NoError(testutils.PatchDeploymentConfig(encryptionPatch(true)),
		"Failed to enable attribute cache encryption")
	ts.Require().NoError(testutils.RestartServer(), "Failed to restart server with encryption enabled")
	ts.Require().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token after restart")
	// Restore steps use non-fatal assertions so a failure in one still lets the rest run, leaving the
	// shared server in the default (disabled) state for the other packages.
	defer func() {
		ts.Assert().NoError(testutils.PatchDeploymentConfig(encryptionPatch(false)),
			"Failed to restore attribute cache encryption config")
		ts.Assert().NoError(testutils.RestartServer(), "Failed to restart server after config restore")
		ts.Assert().NoError(testutils.ObtainAdminAccessToken(), "Failed to re-obtain admin token after restore")
	}()

	ts.assertRoundTrip()
}

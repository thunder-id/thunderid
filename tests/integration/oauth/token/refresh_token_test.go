// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	refreshTokenTestClientID     = "refresh_token_test_client"
	refreshTokenTestClientSecret = "refresh_token_test_secret"
	refreshTokenTestAppName      = "RefreshTokenTestApp"
	refreshTokenTestRedirectURI  = "https://localhost:3000"
	refreshTokenTestUsername     = "refresh_token_test_user"
	refreshTokenTestPassword     = "testpass123"
	refreshTokenTestResource     = "https://refresh-token.example.com"
)

var (
	refreshTokenTestOU = testutils.OrganizationUnit{
		Handle:      "refresh-token-test-ou",
		Name:        "Refresh Token Test OU",
		Description: "Organization unit for refresh token integration testing",
		Parent:      nil,
	}

	refreshTokenTestUserType = testutils.UserType{
		Name: "refresh-token-test-person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
			"family_name": map[string]interface{}{
				"type": "string",
			},
		},
	}

	refreshTokenTestAuthFlow = testutils.Flow{
		Name:     "Refresh Token Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_refresh_token_test",
		Nodes: []map[string]interface{}{
			{
				"id":        "start",
				"type":      "START",
				"onSuccess": "prompt_credentials",
			},
			{
				"id":   "prompt_credentials",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{
								"ref":        "input_001",
								"identifier": "username",
								"type":       "TEXT_INPUT",
								"required":   true,
							},
							{
								"ref":        "input_002",
								"identifier": "password",
								"type":       "PASSWORD_INPUT",
								"required":   true,
							},
						},
						"action": map[string]interface{}{
							"ref":      "action_001",
							"nextNode": "credentials_auth",
						},
					},
				},
			},
			{
				"id":   "credentials_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
				},
				"onSuccess": "authorization_check",
			},
			{
				"id":   "authorization_check",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthorizationExecutor",
				},
				"onSuccess": "auth_assert",
			},
			{
				"id":   "auth_assert",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AuthAssertExecutor",
				},
				"onSuccess": "end",
			},
			{
				"id":   "end",
				"type": "END",
			},
		},
	}
)

// RefreshTokenTestSuite tests the refresh token grant flow,
// specifically verifying ID token behavior.
type RefreshTokenTestSuite struct {
	suite.Suite
	applicationID    string
	entityTypeID     string
	authFlowID       string
	ouID             string
	userID           string
	resourceServerID string
	client           *http.Client
}

func TestRefreshTokenTestSuite(t *testing.T) {
	suite.Run(t, new(RefreshTokenTestSuite))
}

func (ts *RefreshTokenTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	// Create organization unit.
	ouID, err := testutils.CreateOrganizationUnit(refreshTokenTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	// Create user type.
	refreshTokenTestUserType.OUID = ouID
	schemaID, err := testutils.CreateUserType(refreshTokenTestUserType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = schemaID

	// Create authentication flow.
	flowID, err := testutils.CreateFlow(refreshTokenTestAuthFlow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	ts.authFlowID = flowID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Refresh Token Resource Server",
		Description: "Resource server for refresh token integration tests",
		Identifier:  refreshTokenTestResource,
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err, "Failed to create refresh token resource server")
	ts.resourceServerID = resourceServerID

	// Create application with authorization_code and refresh_token grants.
	ts.applicationID = ts.createTestApplication()

	// Create test user.
	user := testutils.User{
		OUID: ouID,
		Type: "refresh-token-test-person",
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "refresh_token_test@example.com",
			"given_name": "Refresh",
			"family_name": "TokenTest"
		}`, refreshTokenTestUsername, refreshTokenTestPassword)),
	}
	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.userID = userID
}

func (ts *RefreshTokenTestSuite) createTestApplication() string {
	app := map[string]interface{}{
		"name":                      refreshTokenTestAppName,
		"description":               "Application for refresh token integration tests",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"refresh-token-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                refreshTokenTestClientID,
					"clientSecret":            refreshTokenTestClientSecret,
					"redirectUris":            []string{refreshTokenTestRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err, "Failed to marshal application data")

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err, "Failed to create request")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err, "Failed to create application")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ts.T().Fatalf("Failed to create application. Status: %d, Response: %s",
			resp.StatusCode, string(bodyBytes))
	}

	var respData map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respData)
	ts.Require().NoError(err, "Failed to parse response")

	appID := respData["id"].(string)
	ts.T().Logf("Created refresh token test application with ID: %s", appID)
	return appID
}

func (ts *RefreshTokenTestSuite) TearDownSuite() {
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("Failed to delete test user: %v", err)
		}
	}

	if ts.applicationID != "" {
		if err := testutils.DeleteApplication(ts.applicationID); err != nil {
			ts.T().Logf("Failed to delete test application: %v", err)
		}
	}

	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Logf("Failed to delete test auth flow: %v", err)
		}
	}

	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete resource server: %v", err)
		}
	}

	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type: %v", err)
		}
	}

	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test OU: %v", err)
		}
	}
}

// obtainTokensViaAuthCodeFlow performs the complete authorization code flow
// and returns the token response.
func (ts *RefreshTokenTestSuite) obtainTokensViaAuthCodeFlow(
	scope string) *testutils.TokenResponse {

	// Step 1: Initiate authorization flow.
	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		refreshTokenTestClientID, refreshTokenTestRedirectURI,
		"code", scope, "test-state", refreshTokenTestResource)
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode,
		"Expected redirect status from authorization endpoint")

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Expected Location header")

	authID, executionId, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract auth data")

	// Step 2: Execute authentication flow.
	initialStep, err := testutils.ExecuteAuthenticationFlow(executionId, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionId,
		map[string]string{
			"username": refreshTokenTestUsername,
			"password": refreshTokenTestPassword,
		}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to execute authentication flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"Authentication flow should complete")
	ts.Require().NotEmpty(flowStep.Assertion, "Assertion should not be empty")

	// Step 3: Complete authorization.
	authzResp, err := testutils.CompleteAuthorization(
		authID, flowStep.Assertion)
	ts.Require().NoError(err, "Failed to complete authorization")

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err, "Failed to extract authorization code")

	// Step 4: Exchange code for tokens using Basic Auth.
	tokenResult, err := testutils.RequestTokenWithResource(
		refreshTokenTestClientID, refreshTokenTestClientSecret,
		code, refreshTokenTestRedirectURI, "authorization_code", refreshTokenTestResource)
	ts.Require().NoError(err, "Failed to request token")
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode,
		"Token request should succeed. Response: %s",
		string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token, "Token should not be nil")
	ts.Require().NotEmpty(tokenResult.Token.AccessToken,
		"Access token should not be empty")
	ts.Require().NotEmpty(tokenResult.Token.RefreshToken,
		"Refresh token should not be empty")

	return tokenResult.Token
}

// TestRefreshTokenGrantReturnsIDToken verifies that when the original
// authorization code flow includes the "openid" scope, the refresh token
// grant also returns a new ID token.
func (ts *RefreshTokenTestSuite) TestRefreshTokenGrantReturnsIDToken() {
	// Step 1: Obtain tokens via auth code flow with openid scope.
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("openid")
	ts.NotEmpty(tokenResponse.IDToken,
		"Auth code response should contain an ID token with openid scope")

	// Step 2: Use the refresh token to get new tokens.
	refreshResponse, err := testutils.RefreshAccessToken(
		refreshTokenTestClientID, refreshTokenTestClientSecret,
		tokenResponse.RefreshToken)
	ts.Require().NoError(err, "Refresh token request should succeed")
	ts.Require().NotNil(refreshResponse,
		"Refresh token response should not be nil")

	// Step 3: Validate the refresh token response contains an ID token.
	ts.NotEmpty(refreshResponse.AccessToken,
		"Refresh response should contain an access token")
	ts.NotEmpty(refreshResponse.IDToken,
		"Refresh response should contain an ID token with openid scope")
}

// TestRefreshTokenGrantWithoutOpenIDScope verifies that when the original
// authorization code flow does not include the "openid" scope, the refresh
// token grant does not return an ID token.
func (ts *RefreshTokenTestSuite) TestRefreshTokenGrantWithoutOpenIDScope() {
	// Step 1: Obtain tokens via auth code flow without openid scope.
	tokenResponse := ts.obtainTokensViaAuthCodeFlow("internal_user_mgt_view")
	ts.Empty(tokenResponse.IDToken,
		"Auth code response should not contain an ID token without openid scope")

	// Step 2: Use the refresh token to get new tokens.
	refreshResponse, err := testutils.RefreshAccessToken(
		refreshTokenTestClientID, refreshTokenTestClientSecret,
		tokenResponse.RefreshToken)
	ts.Require().NoError(err, "Refresh token request should succeed")
	ts.Require().NotNil(refreshResponse,
		"Refresh token response should not be nil")

	// Step 3: Validate the refresh token response has no ID token.
	ts.NotEmpty(refreshResponse.AccessToken,
		"Refresh response should contain an access token")
	ts.Empty(refreshResponse.IDToken,
		"Refresh response should not contain an ID token without openid scope")
}

// This suite covers what happens to an already-issued refresh token when the authorization behind it
// changes: role and permission edits must narrow the scopes it can mint, and a credential change must
// stop it minting at all. Each scenario runs end to end, from an authorization code flow through the
// mutation to a refresh.
//
// Every scenario owns the state it mutates (its own user, role, or application), so the mutations
// cannot leak into another scenario's expectations.

const (
	refreshSecClientID     = "refresh_security_test_client"
	refreshSecClientSecret = "refresh_security_test_secret"
	refreshSecRedirectURI  = "https://localhost:3000"
	refreshSecPassword     = "testpass123"
	refreshSecResource     = "https://refresh-security.example.com"
	refreshSecUserType     = "refresh-security-person"
)

type RefreshSecurityTestSuite struct {
	suite.Suite
	ouID             string
	entityTypeID     string
	authFlowID       string
	resourceServerID string
	applicationID    string
	createdUserIDs   []string
	createdRoleIDs   []string
	createdAppIDs    []string
}

func TestRefreshSecurityTestSuite(t *testing.T) {
	suite.Run(t, new(RefreshSecurityTestSuite))
}

func (ts *RefreshSecurityTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "refresh-security-test-ou",
		Name:        "Refresh Security Test OU",
		Description: "Organization unit for refresh token security integration testing",
	})
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	userType := refreshTokenTestUserType
	userType.Name = refreshSecUserType
	userType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = entityTypeID

	// Reuse the authentication flow shape from the refresh token suite: it already runs the
	// AuthorizationExecutor, which is what resolves permission scopes at the authorization endpoint.
	flow := refreshTokenTestAuthFlow
	flow.Name = "Refresh Security Test Auth Flow"
	flow.Handle = "auth_flow_refresh_security_test"
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	ts.authFlowID = flowID

	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Refresh Security Resource Server",
		Description: "Resource server for refresh token security integration tests",
		Identifier:  refreshSecResource,
		OUID:        ouID,
	}, []testutils.Action{
		{Name: "Read Documents", Handle: "read", Description: "Permission to read documents"},
		{Name: "Write Documents", Handle: "write", Description: "Permission to write documents"},
	})
	ts.Require().NoError(err, "Failed to create resource server")
	ts.resourceServerID = resourceServerID

	ts.applicationID = ts.createApplication("RefreshSecurityTestApp", refreshSecClientID, refreshSecClientSecret)
}

func (ts *RefreshSecurityTestSuite) TearDownSuite() {
	for _, appID := range ts.createdAppIDs {
		_ = testutils.DeleteApplication(appID)
	}
	for _, roleID := range ts.createdRoleIDs {
		_ = testutils.DeleteRole(roleID)
	}
	for _, userID := range ts.createdUserIDs {
		_ = testutils.DeleteUser(userID)
	}
	if ts.resourceServerID != "" {
		_ = testutils.DeleteResourceServer(ts.resourceServerID)
	}
	if ts.authFlowID != "" {
		_ = testutils.DeleteFlow(ts.authFlowID)
	}
	if ts.entityTypeID != "" {
		_ = testutils.DeleteUserType(ts.entityTypeID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
}

// createApplication registers an application wired to the suite's flow, with the grants the refresh
// scenarios need.
func (ts *RefreshSecurityTestSuite) createApplication(name, clientID, clientSecret string) string {
	appID, err := testutils.CreateApplication(testutils.Application{
		Name:                      name,
		Description:               "Application for refresh token security integration tests",
		OUID:                      ts.ouID,
		Type:                      "fullstack",
		AuthFlowID:                ts.authFlowID,
		IsRegistrationFlowEnabled: false,
		AllowedUserTypes:          []string{refreshSecUserType},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                clientID,
					"clientSecret":            clientSecret,
					"redirectUris":            []string{refreshSecRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err, "Failed to create application")
	ts.createdAppIDs = append(ts.createdAppIDs, appID)
	return appID
}

// updateApplication replaces the application, optionally rotating its client secret. An empty secret
// leaves the stored one intact, so the update touches only the system attributes.
func (ts *RefreshSecurityTestSuite) updateApplication(appID, name, clientID, clientSecret string) error {
	oauthConfig := map[string]interface{}{
		"clientId":                clientID,
		"redirectUris":            []string{refreshSecRedirectURI},
		"grantTypes":              []string{"authorization_code", "refresh_token"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
	}
	if clientSecret != "" {
		oauthConfig["clientSecret"] = clientSecret
	}
	return testutils.UpdateApplication(appID, testutils.Application{
		ID:                        appID,
		Name:                      name,
		Description:               "Application for refresh token security integration tests",
		OUID:                      ts.ouID,
		Type:                      "fullstack",
		AuthFlowID:                ts.authFlowID,
		IsRegistrationFlowEnabled: false,
		AllowedUserTypes:          []string{refreshSecUserType},
		InboundAuthConfig: []map[string]interface{}{
			{"type": "oauth2", "config": oauthConfig},
		},
	})
}

// createUser registers a user of the suite's type and tracks it for cleanup.
func (ts *RefreshSecurityTestSuite) createUser(username string) string {
	userID, err := testutils.CreateUser(testutils.User{
		OUID: ts.ouID,
		Type: refreshSecUserType,
		Attributes: json.RawMessage(fmt.Sprintf(`{
			"username": "%s",
			"password": "%s",
			"email": "%s@example.com",
			"given_name": "Refresh",
			"family_name": "Security"
		}`, username, refreshSecPassword, username)),
	})
	ts.Require().NoError(err, "Failed to create test user")
	ts.createdUserIDs = append(ts.createdUserIDs, userID)
	return userID
}

// createRole grants the given permissions on the suite's resource server to the given user.
func (ts *RefreshSecurityTestSuite) createRole(name, userID string, permissions []string) string {
	roleID, err := testutils.CreateRole(testutils.Role{
		Name:        name,
		Description: "Role for refresh token security integration tests",
		OUID:        ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: permissions},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	})
	ts.Require().NoError(err, "Failed to create test role")
	ts.createdRoleIDs = append(ts.createdRoleIDs, roleID)
	return roleID
}

// obtainTokens runs the full authorization code flow and returns the token response.
func (ts *RefreshSecurityTestSuite) obtainTokens(
	clientID, clientSecret, username, scope string,
) *testutils.TokenResponse {
	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		clientID, refreshSecRedirectURI, "code", scope, "test-state", refreshSecResource)
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "Expected redirect from authorization endpoint")

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "Failed to extract auth data")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": username,
		"password": refreshSecPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to execute authentication flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Authentication flow should complete")

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err, "Failed to complete authorization")

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err, "Failed to extract authorization code")

	result, err := testutils.RequestTokenWithResource(
		clientID, clientSecret, code, refreshSecRedirectURI, "authorization_code", refreshSecResource)
	ts.Require().NoError(err, "Failed to exchange code for tokens")
	ts.Require().Equal(http.StatusOK, result.StatusCode,
		"Token request should succeed. Response: %s", string(result.Body))
	ts.Require().NotEmpty(result.Token.RefreshToken, "Refresh token should not be empty")

	return result.Token
}

// accessTokenScopes returns the scopes carried by the given access token.
func (ts *RefreshSecurityTestSuite) accessTokenScopes(accessToken string) []string {
	claims, err := testutils.DecodeJWT(accessToken)
	ts.Require().NoError(err, "Failed to decode access token")

	scopeRaw, ok := claims.Additional["scope"]
	if !ok {
		return nil
	}
	scopeStr, ok := scopeRaw.(string)
	ts.Require().True(ok, "scope claim should be a string")
	if scopeStr == "" {
		return nil
	}
	return strings.Split(scopeStr, " ")
}

// Baseline: with the grant untouched, a refresh keeps minting the permission scopes. Without this the
// negative cases below could pass for the wrong reason.
func (ts *RefreshSecurityTestSuite) TestRefresh_NoChange_KeepsPermissionScopes() {
	userID := ts.createUser("refresh_sec_baseline")
	ts.createRole("RefreshSec_Baseline", userID, []string{"read", "write"})

	tokens := ts.obtainTokens(refreshSecClientID, refreshSecClientSecret, "refresh_sec_baseline", "openid read write")
	ts.Require().Contains(ts.accessTokenScopes(tokens.AccessToken), "read", "initial token should carry read")

	refreshed, err := testutils.RefreshAccessToken(refreshSecClientID, refreshSecClientSecret, tokens.RefreshToken)
	ts.Require().NoError(err, "Refresh should succeed")

	scopes := ts.accessTokenScopes(refreshed.AccessToken)
	ts.Assert().Contains(scopes, "read", "refreshed token should retain read")
	ts.Assert().Contains(scopes, "write", "refreshed token should retain write")
}

// Unassigning the role must stop the refresh token minting the permissions it granted.
func (ts *RefreshSecurityTestSuite) TestRefresh_RoleUnassigned_DropsPermissionScopes() {
	userID := ts.createUser("refresh_sec_unassign")
	roleID := ts.createRole("RefreshSec_Unassign", userID, []string{"read", "write"})

	tokens := ts.obtainTokens(refreshSecClientID, refreshSecClientSecret, "refresh_sec_unassign", "openid read write")
	ts.Require().Contains(ts.accessTokenScopes(tokens.AccessToken), "read", "initial token should carry read")

	ts.Require().NoError(testutils.RemoveRoleAssignments(roleID, []testutils.Assignment{
		{ID: userID, Type: "user"},
	}), "Failed to unassign role")

	refreshed, err := testutils.RefreshAccessToken(refreshSecClientID, refreshSecClientSecret, tokens.RefreshToken)
	ts.Require().NoError(err, "Refresh should still succeed, with narrowed scopes")

	scopes := ts.accessTokenScopes(refreshed.AccessToken)
	ts.Assert().NotContains(scopes, "read", "refreshed token must not carry read after unassignment")
	ts.Assert().NotContains(scopes, "write", "refreshed token must not carry write after unassignment")
	ts.Assert().Contains(scopes, "openid", "OIDC scopes are not permissions and must survive")
}

// Deleting the role entirely has the same effect as unassigning it.
func (ts *RefreshSecurityTestSuite) TestRefresh_RoleDeleted_DropsPermissionScopes() {
	userID := ts.createUser("refresh_sec_roledelete")
	roleID := ts.createRole("RefreshSec_RoleDelete", userID, []string{"read", "write"})

	tokens := ts.obtainTokens(refreshSecClientID, refreshSecClientSecret, "refresh_sec_roledelete", "openid read write")
	ts.Require().Contains(ts.accessTokenScopes(tokens.AccessToken), "read", "initial token should carry read")

	ts.Require().NoError(testutils.DeleteRole(roleID), "Failed to delete role")

	refreshed, err := testutils.RefreshAccessToken(refreshSecClientID, refreshSecClientSecret, tokens.RefreshToken)
	ts.Require().NoError(err, "Refresh should still succeed, with narrowed scopes")

	scopes := ts.accessTokenScopes(refreshed.AccessToken)
	ts.Assert().NotContains(scopes, "read", "refreshed token must not carry read after role deletion")
	ts.Assert().NotContains(scopes, "write", "refreshed token must not carry write after role deletion")
}

// Stripping one permission from the role narrows the refreshed token to the permissions that remain.
func (ts *RefreshSecurityTestSuite) TestRefresh_RolePermissionRemoved_DropsThatScopeOnly() {
	userID := ts.createUser("refresh_sec_permstrip")
	roleID := ts.createRole("RefreshSec_PermStrip", userID, []string{"read", "write"})

	tokens := ts.obtainTokens(refreshSecClientID, refreshSecClientSecret, "refresh_sec_permstrip", "openid read write")
	ts.Require().Contains(ts.accessTokenScopes(tokens.AccessToken), "write", "initial token should carry write")

	ts.Require().NoError(testutils.UpdateRole(roleID, testutils.Role{
		Name:        "RefreshSec_PermStrip",
		Description: "Role for refresh token security integration tests",
		OUID:        ts.ouID,
		Permissions: []testutils.ResourcePermissions{
			{ResourceServerID: ts.resourceServerID, Permissions: []string{"read"}},
		},
		Assignments: []testutils.Assignment{{ID: userID, Type: "user"}},
	}), "Failed to strip permission from role")

	refreshed, err := testutils.RefreshAccessToken(refreshSecClientID, refreshSecClientSecret, tokens.RefreshToken)
	ts.Require().NoError(err, "Refresh should still succeed, with narrowed scopes")

	scopes := ts.accessTokenScopes(refreshed.AccessToken)
	ts.Assert().Contains(scopes, "read", "the retained permission must still be minted")
	ts.Assert().NotContains(scopes, "write", "the removed permission must not be minted")
}

// A password reset must stop the refresh tokens established before it.
func (ts *RefreshSecurityTestSuite) TestRefresh_PasswordReset_RejectsToken() {
	userID := ts.createUser("refresh_sec_password")
	ts.createRole("RefreshSec_Password", userID, []string{"read"})

	tokens := ts.obtainTokens(refreshSecClientID, refreshSecClientSecret, "refresh_sec_password", "openid read")

	ts.Require().NoError(testutils.UpdateUserCredentials(userID, map[string]string{
		"password": "new-testpass456",
	}), "Failed to reset user password")

	_, err := testutils.RefreshAccessToken(refreshSecClientID, refreshSecClientSecret, tokens.RefreshToken)
	ts.Require().Error(err, "Refresh must be rejected after a password reset")
	ts.Assert().Contains(err.Error(), "invalid_grant", "Rejection should be invalid_grant")
}

// A client secret rotation must stop the refresh tokens issued under the old secret.
func (ts *RefreshSecurityTestSuite) TestRefresh_ClientSecretRotated_RejectsToken() {
	const (
		rotateClientID     = "refresh_security_rotate_client"
		rotateClientSecret = "refresh_security_rotate_secret"
	)
	appID := ts.createApplication("RefreshSecurityRotateApp", rotateClientID, rotateClientSecret)
	userID := ts.createUser("refresh_sec_rotate")
	ts.createRole("RefreshSec_Rotate", userID, []string{"read"})

	tokens := ts.obtainTokens(rotateClientID, rotateClientSecret, "refresh_sec_rotate", "openid read")

	ts.Require().NoError(testutils.UpdateApplication(appID, testutils.Application{
		ID:                        appID,
		Name:                      "RefreshSecurityRotateApp",
		Description:               "Application for refresh token security integration tests",
		OUID:                      ts.ouID,
		Type:                      "fullstack",
		AuthFlowID:                ts.authFlowID,
		IsRegistrationFlowEnabled: false,
		AllowedUserTypes:          []string{refreshSecUserType},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                rotateClientID,
					"clientSecret":            "rotated-secret-value",
					"redirectUris":            []string{refreshSecRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	}), "Failed to rotate client secret")

	_, err := testutils.RefreshAccessToken(rotateClientID, "rotated-secret-value", tokens.RefreshToken)
	ts.Require().Error(err, "Refresh must be rejected after a client secret rotation")
	ts.Assert().Contains(err.Error(), "invalid_grant", "Rejection should be invalid_grant")
}

// An unrelated application edit rebuilds the whole system-attribute blob, so the marker written by a
// secret rotation must survive it. Losing it would revive the refresh tokens the rotation invalidated.
func (ts *RefreshSecurityTestSuite) TestRefresh_ClientSecretRotatedThenAppRenamed_StillRejectsToken() {
	const (
		renameClientID = "refresh_security_rename_client"
		renameSecret   = "refresh_security_rename_secret"
		rotatedSecret  = "rotated-secret-after-rename"
	)
	appID := ts.createApplication("RefreshSecurityRenameApp", renameClientID, renameSecret)
	userID := ts.createUser("refresh_sec_rename")
	ts.createRole("RefreshSec_Rename", userID, []string{"read"})

	tokens := ts.obtainTokens(renameClientID, renameSecret, "refresh_sec_rename", "openid read")

	ts.Require().NoError(ts.updateApplication(appID, "RefreshSecurityRenameApp", renameClientID, rotatedSecret),
		"Failed to rotate client secret")
	// The rename supplies no secret, so it only rewrites the system attributes.
	ts.Require().NoError(ts.updateApplication(appID, "RefreshSecurityRenamedApp", renameClientID, ""),
		"Failed to rename application")

	_, err := testutils.RefreshAccessToken(renameClientID, rotatedSecret, tokens.RefreshToken)
	ts.Require().Error(err, "Refresh must stay rejected after an unrelated application edit")
	ts.Assert().Contains(err.Error(), "invalid_grant", "Rejection should be invalid_grant")
}

// A deleted user cannot hold a valid grant. The application maps no subject, so the token can only
// have carried an entity ID, and a missing entity means the subject is gone.
func (ts *RefreshSecurityTestSuite) TestRefresh_UserDeleted_RejectsToken() {
	userID := ts.createUser("refresh_sec_userdelete")
	ts.createRole("RefreshSec_UserDelete", userID, []string{"read"})

	tokens := ts.obtainTokens(refreshSecClientID, refreshSecClientSecret, "refresh_sec_userdelete", "openid read")

	ts.Require().NoError(testutils.DeleteUser(userID), "Failed to delete user")

	_, err := testutils.RefreshAccessToken(refreshSecClientID, refreshSecClientSecret, tokens.RefreshToken)
	ts.Require().Error(err, "Refresh must be rejected after the user is deleted")
	ts.Assert().Contains(err.Error(), "invalid_grant", "Rejection should be invalid_grant")
}

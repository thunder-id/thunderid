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

package authz

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/thunder-id/thunderid/tests/integration/testutils"
	"github.com/stretchr/testify/suite"
)

const (
	scopeTestClientID     = "scope_authz_test_client_456"
	scopeTestClientSecret = "scope_authz_test_secret_456"
	scopeTestAppName      = "ScopeAuthzTestApp"
	scopeTestRedirectURI  = "https://localhost:3000/callback"
)

var (
	scopeTestOUID           string
	scopeTestRoleID         string
	scopeUserWithRole       string
	scopeUserNoRole         string
	scopeUserWithGroup      string
	scopeGroupID            string
	scopeEntityTypeID       string
	scopeTestResourceServer string
	scopeTestEntityType     = testutils.UserType{
		Name: "authz-test-person",
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
			"name": map[string]interface{}{
				"type": "string",
			},
			"phone": map[string]interface{}{
				"type": "string",
			},
			"groups": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
			"roles": map[string]interface{}{
				"type":  "array",
				"items": map[string]interface{}{"type": "string"},
			},
			"customAttr": map[string]interface{}{
				"type": "string",
			},
		},
	}
)

type OAuthAuthzScopeTestSuite struct {
	suite.Suite
	client        *http.Client
	flowID        string
	applicationID string
}

func TestOAuthAuthzScopeTestSuite(t *testing.T) {
	suite.Run(t, new(OAuthAuthzScopeTestSuite))
}

func (ts *OAuthAuthzScopeTestSuite) SetupSuite() {
	var err error
	// Setup HTTP client
	ts.client = testutils.GetHTTPClient()
	// Create test organization unit
	ou := testutils.OrganizationUnit{
		Handle:      "oauth-scope-authz-test-ou",
		Name:        "OAuth Scope Authorization Test OU",
		Description: "Organization unit for OAuth scope authorization testing",
		Parent:      nil,
	}
	scopeTestOUID, err = testutils.CreateOrganizationUnit(ou)
	if err != nil {
		ts.T().Fatalf("Failed to create test organization unit: %v", err)
	}
	scopeTestEntityType.OUID = scopeTestOUID

	// Create user type
	scopeEntityTypeID, err = testutils.CreateUserType(scopeTestEntityType)
	if err != nil {
		ts.T().Fatalf("Failed to create user type: %v", err)
	}

	// Create authentication flow
	authFlowID := ts.createTestAuthenticationFlow()
	ts.NotEmpty(authFlowID, "Authentication flow ID should not be empty")
	ts.flowID = authFlowID

	// We need to use the inbound_auth_config approach for OAuth apps
	appID, err := ts.createOAuthApplication(authFlowID)
	if err != nil {
		ts.T().Fatalf("Failed to create OAuth application: %v", err)
	}
	ts.applicationID = appID

	// Create user with role
	userWithRole := testutils.User{
		OUID: scopeTestOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(`{
			"username": "oauth_authorized_user",
			"password": "SecurePass123!",
			"email": "oauth_authorized@test.com",
			"given_name": "OAuth",
			"family_name": "Authorized"
		}`),
	}
	scopeUserWithRole, err = testutils.CreateUser(userWithRole)
	if err != nil {
		ts.T().Fatalf("Failed to create user with role: %v", err)
	}

	// Create user without role
	userNoRole := testutils.User{
		OUID: scopeTestOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(`{
			"username": "oauth_unauthorized_user",
			"password": "SecurePass123!",
			"email": "oauth_unauthorized@test.com",
			"given_name": "OAuth",
			"family_name": "Unauthorized"
		}`),
	}
	scopeUserNoRole, err = testutils.CreateUser(userNoRole)
	if err != nil {
		ts.T().Fatalf("Failed to create user without role: %v", err)
	}

	// Create user to be assigned to the authz group
	userWithGroup := testutils.User{
		OUID: scopeTestOUID,
		Type: "authz-test-person",
		Attributes: json.RawMessage(`{
			"username": "oauth_authorized_group_user",
			"password": "SecurePass123!",
			"email": "oauth_authorized_group@test.com",
			"given_name": "OAuth",
			"family_name": "Authorized"
		}`),
	}
	scopeUserWithGroup, err = testutils.CreateUser(userWithGroup)
	if err != nil {
		ts.T().Fatalf("Failed to create user with group: %v", err)
	}

	// Create group and assign user to group
	group := testutils.Group{
		Name:        "OAuth_DocumentEditors",
		Description: "Group for document editors (OAuth test)",
		OUID:        scopeTestOUID,
		Members: []testutils.Member{
			{
				Id:   scopeUserWithGroup,
				Type: "user",
			},
		},
	}
	scopeGroupID, err = testutils.CreateGroup(group)
	if err != nil {
		ts.T().Fatalf("Failed to create group: %v", err)
	}

	// Create resource server with actions for permissions
	resourceServer := testutils.ResourceServer{
		Name:        "OAuth Document Management System",
		Description: "System for managing documents via OAuth",
		Identifier:  "oauth-document-mgmt",
		OUID:        scopeTestOUID,
	}
	actions := []testutils.Action{
		{
			Name:        "Read Documents",
			Handle:      "read",
			Description: "Permission to read documents",
		},
		{
			Name:        "Write Documents",
			Handle:      "write",
			Description: "Permission to write documents",
		},
	}
	scopeTestResourceServer, err = testutils.CreateResourceServerWithActions(resourceServer, actions)
	if err != nil {
		ts.T().Fatalf("Failed to create resource server with actions: %v", err)
	}

	// Create role with permissions and assign to first user
	role := testutils.Role{
		Name:        "OAuth_DocumentEditor",
		Description: "Can read and write documents (OAuth test)",
		OUID:        scopeTestOUID,
		Permissions: []testutils.ResourcePermissions{
			{
				ResourceServerID: scopeTestResourceServer,
				Permissions:      []string{"read", "write"},
			},
		},
		Assignments: []testutils.Assignment{
			{ID: scopeUserWithRole, Type: "user"},
			{ID: scopeGroupID, Type: "group"},
		},
	}
	scopeTestRoleID, err = testutils.CreateRole(role)
	if err != nil {
		ts.T().Fatalf("Failed to create test role: %v", err)
	}
}

func (ts *OAuthAuthzScopeTestSuite) TearDownSuite() {
	// Cleanup in reverse order
	if scopeTestRoleID != "" {
		if err := testutils.DeleteRole(scopeTestRoleID); err != nil {
			ts.T().Logf("Failed to delete test role: %v", err)
		}
	}

	if scopeTestResourceServer != "" {
		if err := testutils.DeleteResourceServer(scopeTestResourceServer); err != nil {
			ts.T().Logf("Failed to delete test resource server: %v", err)
		}
	}

	if scopeUserNoRole != "" {
		if err := testutils.DeleteUser(scopeUserNoRole); err != nil {
			ts.T().Logf("Failed to delete user without role: %v", err)
		}
	}

	if scopeUserWithGroup != "" {
		if err := testutils.DeleteUser(scopeUserWithGroup); err != nil {
			ts.T().Logf("Failed to delete user with group: %v", err)
		}
	}

	if scopeGroupID != "" {
		if err := testutils.DeleteGroup(scopeGroupID); err != nil {
			ts.T().Logf("Failed to delete group: %v", err)
		}
	}

	if scopeUserWithRole != "" {
		if err := testutils.DeleteUser(scopeUserWithRole); err != nil {
			ts.T().Logf("Failed to delete user with role: %v", err)
		}
	}

	if ts.applicationID != "" {
		if err := testutils.DeleteApplication(ts.applicationID); err != nil {
			ts.T().Logf("Failed to delete application: %v", err)
		}
	}

	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("Failed to delete authentication flow: %v", err)
		}
	}

	if scopeTestOUID != "" {
		if err := testutils.DeleteOrganizationUnit(scopeTestOUID); err != nil {
			ts.T().Logf("Failed to delete organization unit: %v", err)
		}
	}

	if scopeEntityTypeID != "" {
		if err := testutils.DeleteUserType(scopeEntityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type: %v", err)
		}
	}
}

// TestOAuthAuthzFlow_WithAuthorizedScopesWithRoleUserAssignment tests the complete OAuth flow for a user
// with role assignment and verifies that the access token contains the authorized scopes based
//
//	on the user's role permissions.
func (ts *OAuthAuthzScopeTestSuite) TestOAuthAuthzFlow_WithAuthorizedScopesWithRoleUserAssignment() {
	ts.testOAuthAuthzFlow_WithAuthorizedScopes("oauth_authorized_user")
}

// TestOAuthAuthzFlow_WithAuthorizedScopesWithRoleGroupAssignment tests the complete OAuth flow for a user
// with group assignment and verifies that the access token contains the authorized scopes based
// on the user's group role permissions.
func (ts *OAuthAuthzScopeTestSuite) TestOAuthAuthzFlow_WithAuthorizedScopesWithRoleGroupAssignment() {
	ts.testOAuthAuthzFlow_WithAuthorizedScopes("oauth_authorized_group_user")
}

// testOAuthAuthzFlow_WithAuthorizedScopes tests complete OAuth flow with authorized scopes
func (ts *OAuthAuthzScopeTestSuite) testOAuthAuthzFlow_WithAuthorizedScopes(username string) {
	// Step 1: Execute full OAuth flow and obtain token for authorized user
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		scopeTestClientID,
		scopeTestRedirectURI,
		"openid read write",
		username,
		"SecurePass123!",
		false,
		scopeTestClientSecret,
	)
	ts.Require().NoError(err, "Failed to obtain access token")
	ts.Require().NotNil(tokenResp, "Token response should not be nil")
	ts.Require().NotEmpty(tokenResp.AccessToken, "Access token should not be empty")

	// Step 2: Decode access token and verify scopes
	claims, err := testutils.DecodeJWT(tokenResp.AccessToken)
	ts.Require().NoError(err, "Failed to decode access token")
	ts.Require().NotNil(claims, "Claims should not be nil")

	// Verify scope claim contains authorized scopes
	scopeRaw, ok := claims.Additional["scope"]
	ts.Require().True(ok, "scope claim should be present in access token")

	scopeStr, ok := scopeRaw.(string)
	ts.Require().True(ok, "scope claim should be a string")

	// Parse scopes (space-separated)
	scopes := strings.Split(scopeStr, " ")
	ts.Require().Contains(scopes, "openid", "Token should contain openid scope")
	ts.Require().Contains(scopes, "read", "Token should contain read scope")
	ts.Require().Contains(scopes, "write", "Token should contain write scope")
}

// TestOAuthAuthzFlow_WithNoAuthorizedScopes tests OAuth flow when user has no custom scopes
func (ts *OAuthAuthzScopeTestSuite) TestOAuthAuthzFlow_WithNoAuthorizedScopes() {
	// Step 1: Execute full OAuth flow and obtain token for user without role assignments
	tokenResp, err := testutils.ObtainAccessTokenWithPassword(
		scopeTestClientID,
		scopeTestRedirectURI,
		"openid read write",
		"oauth_unauthorized_user",
		"SecurePass123!",
		false,
		scopeTestClientSecret,
	)
	ts.Require().NoError(err, "Failed to obtain access token")
	ts.Require().NotNil(tokenResp, "Token response should not be nil")
	ts.Require().NotEmpty(tokenResp.AccessToken, "Access token should not be empty")

	// Step 2: Decode access token and verify scopes
	claims, err := testutils.DecodeJWT(tokenResp.AccessToken)
	ts.Require().NoError(err, "Failed to decode access token")
	ts.Require().NotNil(claims, "Claims should not be nil")

	// Verify scope claim contains ONLY OIDC scopes (no custom scopes)
	scopeRaw, ok := claims.Additional["scope"]
	ts.Require().True(ok, "scope claim should be present in access token")

	scopeStr, ok := scopeRaw.(string)
	ts.Require().True(ok, "scope claim should be a string")

	// Parse scopes (space-separated)
	scopes := strings.Split(scopeStr, " ")
	ts.Require().Contains(scopes, "openid", "Token should contain openid scope")
	ts.Require().NotContains(scopes, "read", "Token should NOT contain read scope")
	ts.Require().NotContains(scopes, "write", "Token should NOT contain write scope")

	// Verify only OIDC scopes are present
	for _, scope := range scopes {
		// OIDC scopes: openid, profile, email, address, phone, offline_access
		isOIDCScope := scope == "openid" || scope == "profile" || scope == "email" ||
			scope == "address" || scope == "phone" || scope == "offline_access"
		ts.Require().True(isOIDCScope, "Scope '%s' should be an OIDC scope", scope)
	}
}

// createOAuthApplication creates an OAuth application using the low-level API
func (ts *OAuthAuthzScopeTestSuite) createOAuthApplication(authFlowID string) (string, error) {
	app := map[string]interface{}{
		"name":                      scopeTestAppName,
		"description":               "OAuth application for scope authorization testing",
		"ouId":                      scopeTestOUID,
		"authFlowId":                authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"authz-test-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                scopeTestClientID,
					"clientSecret":            scopeTestClientSecret,
					"redirectUris":            []string{scopeTestRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_post",
				},
			},
		},
	}

	return ts.createApplicationRaw(app)
}

// createApplicationRaw creates an application using raw HTTP request
func (ts *OAuthAuthzScopeTestSuite) createApplicationRaw(app map[string]interface{}) (string, error) {
	jsonData, err := json.Marshal(app)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", strings.NewReader(string(jsonData)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create application, status: %d", resp.StatusCode)
	}

	var respData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return "", err
	}

	return respData["id"].(string), nil
}

func (ts *OAuthAuthzScopeTestSuite) createTestAuthenticationFlow() string {
	flow := testutils.Flow{
		Name:     "Authz Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "authz_test_auth_flow",
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
							"nextNode": "basic_auth",
						},
					},
				},
			},
			{
				"id":   "basic_auth",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "BasicAuthExecutor",
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

	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	ts.T().Logf("Created test authentication flow with ID: %s", flowID)

	return flowID
}

// TestOAuthAuthzFlow_WithRequiredAttributes tests that required_optional_attributes from scopes and
// access token config are correctly filtered in the auth assertion
func (ts *OAuthAuthzScopeTestSuite) TestOAuthAuthzFlow_WithRequiredAttributes() {
	// Create OAuth app with IDToken and AccessToken configs
	appConfig := map[string]interface{}{
		"name":                    "RequiredAttributesTestApp",
		"ouId":                    scopeTestOUID,
		"clientId":                "required_attrs_test_client",
		"redirectUris":            []string{scopeTestRedirectURI},
		"grantTypes":              []string{"authorization_code", "refresh_token"},
		"responseTypes":           []string{"code"},
		"tokenEndpointAuthMethod": "client_secret_basic",
		"authFlowId":              ts.flowID,
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                "required_attrs_test_client",
					"clientSecret":            "required_attrs_test_secret",
					"redirectUris":            []string{scopeTestRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"token": map[string]interface{}{
						"idToken": map[string]interface{}{
							"userAttributes": []string{"sub", "name", "email"}, // Only allow these in ID token
						},
						"accessToken": map[string]interface{}{
							"userAttributes": []string{"groups", "roles"}, // Access token attributes
						},
					},
					"scopeClaims": map[string]interface{}{
						"profile": []string{"name"}, // Custom mapping
					},
				},
			},
		},
	}

	appID, err := ts.createApplicationRaw(appConfig)
	ts.Require().NoError(err, "Failed to create OAuth application")
	defer func() {
		_ = testutils.DeleteApplication(appID)
	}()

	// Create a user with attributes (roles and groups are computed from role/group assignments,
	// not stored as user attributes)
	userAttributesJSON := `{
		"username": "requiredattrsuser",
		"password": "TestPassword123!",
		"email": "requiredattrs@test.com",
		"given_name": "Required",
		"family_name": "Attrs",
		"name": "Required Attrs",
		"phone": "+1234567890",
		"customAttr": "customValue"
	}`

	user := testutils.User{
		OUID:       scopeTestOUID,
		Type:       "authz-test-person",
		Attributes: json.RawMessage(userAttributesJSON),
	}

	userID, err := testutils.CreateUser(user)
	ts.Require().NoError(err, "Failed to create test user")
	defer func() {
		_ = testutils.DeleteUser(userID)
	}()

	// Create a role directly assigned to the user
	directRole := testutils.Role{
		Name:        "developer",
		Description: "Developer role directly assigned to user",
		OUID:        scopeTestOUID,
		Assignments: []testutils.Assignment{
			{ID: userID, Type: "user"},
		},
	}
	directRoleID, err := testutils.CreateRole(directRole)
	ts.Require().NoError(err, "Failed to create direct role")
	defer func() {
		_ = testutils.DeleteRole(directRoleID)
	}()

	// Create a group with the user as a member, then assign a second role to the group.
	// This validates that roles inherited through group membership are included in the token.
	reqAttrsGroup := testutils.Group{
		Name:        "RequiredAttrs_Engineers",
		Description: "Group for required attributes role inheritance test",
		OUID:        scopeTestOUID,
		Members: []testutils.Member{
			{Id: userID, Type: "user"},
		},
	}
	reqAttrsGroupID, err := testutils.CreateGroup(reqAttrsGroup)
	ts.Require().NoError(err, "Failed to create test group")
	defer func() {
		_ = testutils.DeleteGroup(reqAttrsGroupID)
	}()

	groupRole := testutils.Role{
		Name:        "reviewer",
		Description: "Reviewer role assigned via group membership",
		OUID:        scopeTestOUID,
		Assignments: []testutils.Assignment{
			{ID: reqAttrsGroupID, Type: "group"},
		},
	}
	groupRoleID, err := testutils.CreateRole(groupRole)
	ts.Require().NoError(err, "Failed to create group role")
	defer func() {
		_ = testutils.DeleteRole(groupRoleID)
	}()

	// Step 1: Initiate authorization flow with OIDC scopes
	scope := "openid profile email"
	state := "test-state-123"
	resp, err := testutils.InitiateAuthorizationFlow("required_attrs_test_client", scopeTestRedirectURI, "code", scope, state)
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "Should redirect to login page")

	// Extract auth ID and flow ID from redirect
	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Location header should be present")
	authId, flowID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract auth data")
	ts.Require().NotEmpty(authId, "Auth ID should not be empty")
	ts.Require().NotEmpty(flowID, "Flow ID should not be empty")

	// Step 2: Execute authentication flow
	flowStep, err := testutils.ExecuteAuthenticationFlow(flowID, map[string]string{
		"username": "requiredattrsuser",
		"password": "TestPassword123!",
	}, "action_001")
	ts.Require().NoError(err, "Failed to execute authentication flow")
	ts.Require().NotEmpty(flowStep.Assertion, "Assertion should be generated")

	// Step 3: Verify assertion structure for OAuth flow.
	// In OAuth flows, user attributes are stored in the attribute cache and not embedded
	// directly in the assertion JWT. Only the aci is included in the JWT claims.

	// Decode JWT to verify claims
	jwtClaims, err := testutils.DecodeJWT(flowStep.Assertion)
	ts.Require().NoError(err, "Failed to decode JWT assertion")
	claims := jwtClaims.Additional

	// Verify sub is present (standard JWT subject claim)
	ts.Assert().NotNil(claims["sub"], "sub claim should be present")
	ts.Assert().Equal(userID, claims["sub"], "sub should match user ID")

	// Verify aci is present and is a non-empty string (OAuth flow caches user attributes)
	ts.Assert().NotNil(claims["aci"], "aci should be present in OAuth assertion")
	cacheID, ok := claims["aci"].(string)
	ts.Require().True(ok, "aci should be a string")
	ts.Assert().NotEmpty(cacheID, "aci should not be empty")

	// Verify user attributes are NOT directly embedded in the assertion (they are in the cache)
	ts.Assert().Nil(claims["name"], "name should NOT be directly in assertion (stored in attribute cache)")
	ts.Assert().Nil(claims["email"], "email should NOT be directly in assertion (stored in attribute cache)")
	ts.Assert().Nil(claims["roles"], "roles should NOT be directly in assertion (stored in attribute cache)")
	ts.Assert().Nil(claims["groups"], "groups should NOT be directly in assertion (stored in attribute cache)")
	ts.Assert().Nil(claims["phone"], "phone should NOT be present in assertion")
	ts.Assert().Nil(claims["customAttr"], "customAttr should NOT be present in assertion")
	ts.Assert().Nil(claims["email_verified"], "email_verified should NOT be present in assertion")
	ts.Assert().Nil(claims["given_name"], "given_name should NOT be present in assertion")
	ts.Assert().Nil(claims["family_name"], "family_name should NOT be present in assertion")

	// Step 4: Complete the authorization flow and verify the attribute cache content via the access token.
	// The access token is built directly from the attribute cache, so its claims reflect the filtered set of
	// resolved attributes. This downstream check confirms that required-attribute filtering was applied.
	authzResp, err := testutils.CompleteAuthorization(authId, flowStep.Assertion)
	ts.Require().NoError(err, "Failed to complete authorization")

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err, "Failed to extract authorization code")
	ts.Require().NotEmpty(code, "Authorization code should not be empty")

	tokenResult, err := testutils.RequestToken(
		"required_attrs_test_client", "required_attrs_test_secret",
		code, scopeTestRedirectURI, "authorization_code",
	)
	ts.Require().NoError(err, "Failed to request access token")
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, "Token request should succeed")
	ts.Require().NotNil(tokenResult.Token, "Token response should not be nil")

	// Decode the access token JWT to inspect its resolved claims
	accessTokenJWT, err := testutils.DecodeJWT(tokenResult.Token.AccessToken)
	ts.Require().NoError(err, "Failed to decode access token")
	atClaims := accessTokenJWT.Additional

	// The access token must carry the same aci, confirming the cache ID is propagated
	// from the assertion JWT through the authorization code to the issued token
	ts.Assert().Equal(cacheID, atClaims["aci"],
		"Access token aci must match the one from the assertion JWT")

	// Required attribute (roles) must be resolved and present — it was included in access_token.user_attributes.
	// The claim should contain both the directly assigned role ("developer") and the group-inherited role ("reviewer").
	ts.Require().NotNil(atClaims["roles"], "roles should be present in access token (required access_token attribute)")
	rolesRaw, ok := atClaims["roles"].([]interface{})
	ts.Require().True(ok, "roles claim should be an array")
	roleNames := make([]string, len(rolesRaw))
	for i, r := range rolesRaw {
		roleNames[i], ok = r.(string)
		ts.Require().True(ok, "each role should be a string")
	}
	ts.Assert().Contains(roleNames, "developer", "roles should include directly assigned role")
	ts.Assert().Contains(roleNames, "reviewer", "roles should include group-inherited role")

	// Excluded attributes must NOT appear — they were never added to the attribute cache
	ts.Assert().Nil(atClaims["name"], "name should NOT be in access token (not an access_token attribute)")
	ts.Assert().Nil(atClaims["email"], "email should NOT be in access token (not an access_token attribute)")
	ts.Assert().Nil(atClaims["phone"], "phone should NOT be in access token (excluded attribute)")
	ts.Assert().Nil(atClaims["customAttr"], "customAttr should NOT be in access token (excluded attribute)")
	ts.Assert().Nil(atClaims["email_verified"], "email_verified should NOT be in access token (excluded attribute)")
	ts.Assert().Nil(atClaims["given_name"], "given_name should NOT be in access token (excluded attribute)")
	ts.Assert().Nil(atClaims["family_name"], "family_name should NOT be in access token (excluded attribute)")
}

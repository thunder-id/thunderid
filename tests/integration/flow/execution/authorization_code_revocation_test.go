// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	codeRevocationClientID     = "code_revocation_flow_client"
	codeRevocationClientSecret = "code_revocation_flow_secret" // #nosec G101 -- test fixture, not a credential
	codeRevocationRedirectURI  = "https://localhost:3000"
	codeRevocationResourceID   = "https://code-revocation.dmv.test"
	codeRevocationPassword     = "Testpass1"
)

const (
	scopeCodeDropped = "code-dropped"
	scopeCodeKept    = "code-kept"
)

var codeRevocationTestOU = testutils.OrganizationUnit{
	Handle:      "code_revocation_flow_test_ou",
	Name:        "Test OU for Authorization Code Revocation",
	Description: "Organization unit created for authorization code revocation testing",
	Parent:      nil,
}

var codeRevocationUserType = testutils.UserType{
	Handle:      "code_revocation_flow_test_user",
	DisplayName: "Code Revocation Test User",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
	},
}

// The AuthorizationExecutor resolves the user's permission scopes into the authorization code.
var codeRevocationAuthFlow = testutils.Flow{
	Name:     "Code Revocation Test Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_code_revocation_test",
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
			"onSuccess": "authorization_check",
		},
		{
			"id":        "authorization_check",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
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

type AuthorizationCodeRevocationTestSuite struct {
	suite.Suite
	ouID             string
	entityTypeID     string
	authFlowID       string
	resourceServerID string
	resourceIDs      []string
	appID            string
	flowIDs          map[string]string
	createdUserIDs   []string
	createdRoleIDs   []string
}

func TestAuthorizationCodeRevocationTestSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationCodeRevocationTestSuite))
}

func (ts *AuthorizationCodeRevocationTestSuite) SetupSuite() {
	ts.flowIDs = map[string]string{}
	for _, handle := range []string{rolePermissionRemovalFlowHandle, deletionFlowHandle} {
		flowID, err := testutils.GetFlowIDByHandle(handle, administrationFlowType)
		ts.Require().NoError(err, "Failed to resolve the shipped %s", handle)
		ts.Require().NotEmpty(flowID, "The shipped %s must be present", handle)
		ts.flowIDs[handle] = flowID
	}

	ouID, err := testutils.CreateOrganizationUnit(codeRevocationTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	userType := codeRevocationUserType
	userType.OUID = ts.ouID
	entityTypeID, err := testutils.CreateUserType(userType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = entityTypeID

	authFlowID, err := testutils.CreateFlow(codeRevocationAuthFlow)
	ts.Require().NoError(err, "Failed to create test authentication flow")
	ts.authFlowID = authFlowID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Code Revocation DMV API",
		Description: "Resource server created for authorization code revocation testing",
		Identifier:  codeRevocationResourceID,
		OUID:        ts.ouID,
	}, nil)
	ts.Require().NoError(err, "Failed to create test resource server")
	ts.resourceServerID = rsID

	for _, scope := range []string{scopeCodeDropped, scopeCodeKept} {
		resourceID, resErr := testutils.CreateResource(ts.resourceServerID, scope, scope, "")
		ts.Require().NoError(resErr, "Failed to create test resource %s", scope)
		ts.resourceIDs = append(ts.resourceIDs, resourceID)
	}

	appID, err := testutils.CreateApplication(testutils.Application{
		Name:                      "Code Revocation Web Client",
		Description:               "Application created for authorization code revocation testing",
		OUID:                      ts.ouID,
		Type:                      "fullstack",
		AuthFlowID:                ts.authFlowID,
		IsRegistrationFlowEnabled: false,
		AllowedUserTypes:          []string{codeRevocationUserType.Handle},
		InboundAuthConfig: []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                codeRevocationClientID,
					"clientSecret":            codeRevocationClientSecret,
					"redirectUris":            []string{codeRevocationRedirectURI},
					"grantTypes":              []string{"authorization_code"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
				},
			},
		},
	})
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *AuthorizationCodeRevocationTestSuite) TearDownSuite() {
	for _, roleID := range ts.createdRoleIDs {
		if err := testutils.DeleteRole(roleID); err != nil {
			ts.T().Logf("Failed to delete test role %s during teardown: %v", roleID, err)
		}
	}
	// A user removed by the deletion flow is already gone, so its failure here is expected.
	for _, userID := range ts.createdUserIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Failed to delete test user %s during teardown: %v", userID, err)
		}
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	for _, resourceID := range ts.resourceIDs {
		if err := testutils.DeleteResource(ts.resourceServerID, resourceID); err != nil {
			ts.T().Logf("Failed to delete test resource %s during teardown: %v", resourceID, err)
		}
	}
	if ts.resourceServerID != "" {
		if err := testutils.DeleteResourceServer(ts.resourceServerID); err != nil {
			ts.T().Logf("Failed to delete test resource server during teardown: %v", err)
		}
	}
	if ts.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.authFlowID); err != nil {
			ts.T().Logf("Failed to delete test authentication flow during teardown: %v", err)
		}
	}
	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

func (ts *AuthorizationCodeRevocationTestSuite) TestPermissionRemovedAfterCodeIssued_IsNotMinted() {
	username := common.GenerateUniqueUsername("code_revocation_narrowed")
	user := testutils.Assignment{ID: ts.createUser(username), Type: "user"}
	roleID := createRole(&ts.Suite, &ts.createdRoleIDs, ts.ouID, ts.resourceServerID,
		"Code Revocation Narrowed Role", []string{scopeCodeDropped, scopeCodeKept}, user)

	code := ts.obtainCode(username, "openid "+scopeCodeDropped+" "+scopeCodeKept)

	requireCompleted(&ts.Suite, executeAsAdmin(&ts.Suite, ts.flowIDs[rolePermissionRemovalFlowHandle],
		map[string]string{
			roleTargetInput:        roleID,
			permissionsTargetInput: permissionSet(&ts.Suite, ts.resourceServerID, scopeCodeKept),
		}))

	result := ts.redeem(code)

	ts.Require().Equal(http.StatusOK, result.StatusCode, "Token response: %s", string(result.Body))
	granted := strings.Fields(result.Token.Scope)
	ts.NotContains(granted, scopeCodeDropped, "A permission revoked after the code was issued must not be granted")
	ts.Contains(granted, scopeCodeKept, "A permission the role still grants must be granted")
	ts.Contains(granted, "openid", "OIDC scopes are not permissions and must survive")
	minted := ts.accessTokenScopes(result.Token.AccessToken)
	ts.NotContains(minted, scopeCodeDropped, "The access token must not carry the revoked permission")
	ts.Contains(minted, scopeCodeKept, "The access token must carry the permission the role still grants")
}

func (ts *AuthorizationCodeRevocationTestSuite) TestUserDeletedAfterCodeIssued_RefusesRedemption() {
	username := common.GenerateUniqueUsername("code_revocation_deleted")
	userID := ts.createUser(username)
	createRole(&ts.Suite, &ts.createdRoleIDs, ts.ouID, ts.resourceServerID,
		"Code Revocation Deleted User Role", []string{scopeCodeKept}, testutils.Assignment{ID: userID, Type: "user"})

	code := ts.obtainCode(username, "openid "+scopeCodeKept)

	requireCompleted(&ts.Suite, executeAsAdmin(&ts.Suite, ts.flowIDs[deletionFlowHandle],
		map[string]string{deletionSubjectInput: userID}))

	result := ts.redeem(code)

	ts.Require().Equal(http.StatusBadRequest, result.StatusCode, "Token response: %s", string(result.Body))
	var errResp map[string]interface{}
	ts.Require().NoError(json.Unmarshal(result.Body, &errResp), "Token response: %s", string(result.Body))
	ts.Equal("invalid_grant", errResp["error"], "A code whose subject was revoked must be refused")
	ts.Equal("The authorization grant has been revoked", errResp["error_description"],
		"The refusal must come from the revocation check, not from any other code validation")
}

func (ts *AuthorizationCodeRevocationTestSuite) TestCodeIssuedAfterPermissionRemoval_IsNotNarrowed() {
	username := common.GenerateUniqueUsername("code_revocation_regranted")
	user := testutils.Assignment{ID: ts.createUser(username), Type: "user"}
	roleID := createRole(&ts.Suite, &ts.createdRoleIDs, ts.ouID, ts.resourceServerID,
		"Code Revocation Regranted Role", []string{scopeCodeDropped, scopeCodeKept}, user)

	requireCompleted(&ts.Suite, executeAsAdmin(&ts.Suite, ts.flowIDs[rolePermissionRemovalFlowHandle],
		map[string]string{
			roleTargetInput:        roleID,
			permissionsTargetInput: permissionSet(&ts.Suite, ts.resourceServerID, scopeCodeKept),
		}))
	// The user holds the revoked permission again, so only the revocation row could keep it out of the token.
	createRole(&ts.Suite, &ts.createdRoleIDs, ts.ouID, ts.resourceServerID,
		"Code Revocation Restored Role", []string{scopeCodeDropped}, user)

	// The code's issue time is truncated to the second and revoked when at or before the cutoff.
	time.Sleep(1100 * time.Millisecond)

	code := ts.obtainCode(username, "openid "+scopeCodeDropped+" "+scopeCodeKept)
	result := ts.redeem(code)

	ts.Require().Equal(http.StatusOK, result.StatusCode, "Token response: %s", string(result.Body))
	granted := strings.Fields(result.Token.Scope)
	ts.Contains(granted, scopeCodeDropped,
		"A permission held when the code was issued must be granted; the revocation is bounded by its cutoff")
	ts.Contains(granted, scopeCodeKept, "A permission the role still grants must be granted")
	ts.Contains(ts.accessTokenScopes(result.Token.AccessToken), scopeCodeDropped,
		"The access token must carry the permission held when the code was issued")
}

func (ts *AuthorizationCodeRevocationTestSuite) createUser(username string) string {
	ts.T().Helper()

	attributes, err := json.Marshal(map[string]string{"username": username, "password": codeRevocationPassword})
	ts.Require().NoError(err)

	userID, err := testutils.CreateUser(testutils.User{
		Type:       codeRevocationUserType.Handle,
		OUID:       ts.ouID,
		Attributes: attributes,
	})
	ts.Require().NoError(err, "Failed to create test user")
	ts.createdUserIDs = append(ts.createdUserIDs, userID)
	return userID
}

func (ts *AuthorizationCodeRevocationTestSuite) obtainCode(username, scope string) string {
	ts.T().Helper()

	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		codeRevocationClientID, codeRevocationRedirectURI, "code", scope, "test-state", codeRevocationResourceID)
	ts.Require().NoError(err, "Failed to initiate authorization flow")
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode, "Expected redirect from authorization endpoint")

	authID, executionID, err := testutils.ExtractAuthData(resp.Header.Get("Location"))
	ts.Require().NoError(err, "Failed to extract auth data")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": username,
		"password": codeRevocationPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to execute authentication flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "Authentication flow should complete")

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err, "Failed to complete authorization")

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err, "Failed to extract authorization code")
	return code
}

func (ts *AuthorizationCodeRevocationTestSuite) redeem(code string) *testutils.TokenHTTPResult {
	ts.T().Helper()

	result, err := testutils.RequestTokenWithResource(codeRevocationClientID, codeRevocationClientSecret,
		code, codeRevocationRedirectURI, "authorization_code", codeRevocationResourceID)
	ts.Require().NoError(err, "Token request failed")
	return result
}

func (ts *AuthorizationCodeRevocationTestSuite) accessTokenScopes(accessToken string) []string {
	ts.T().Helper()

	claims, err := testutils.DecodeJWT(accessToken)
	ts.Require().NoError(err, "Failed to decode access token")
	scope, ok := claims.Additional["scope"].(string)
	ts.Require().True(ok, "The access token must carry a string scope claim")
	return strings.Fields(scope)
}

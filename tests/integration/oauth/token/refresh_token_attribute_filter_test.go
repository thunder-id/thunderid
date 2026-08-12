// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	rtAttrFilterClientID     = "rt_attr_filter_test_client"
	rtAttrFilterClientSecret = "rt_attr_filter_test_secret"
	rtAttrFilterRedirectURI  = "https://localhost:3000"
	rtAttrFilterUsername     = "rt_attr_filter_test_user"
	rtAttrFilterPassword     = "RtAttrFilterPass1!"
)

var rtAttrFilterUserType = testutils.UserType{
	Name: "rt-attr-filter-person",
	Schema: map[string]interface{}{
		"username":    map[string]interface{}{"type": "string"},
		"password":    map[string]interface{}{"type": "string", "credential": true},
		"given_name":  map[string]interface{}{"type": "string"},
		"family_name": map[string]interface{}{"type": "string"},
	},
}

var rtAttrFilterAuthFlow = testutils.Flow{
	Name:     "Refresh Token Attribute Filter Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_rt_attr_filter_test",
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

// RefreshTokenAttributeFilterTestSuite verifies that a refresh_token-issued access token only
// carries the user attributes selected by the app's token.accessToken.userConfig.attributes
// allow-list, mirroring the allow-list enforcement already proven for the authorization_code grant.
type RefreshTokenAttributeFilterTestSuite struct {
	suite.Suite
	client           *http.Client
	ouID             string
	entityTypeID     string
	authFlowID       string
	userID           string
	applicationID    string
	resourceServerID string
}

// TestRefreshTokenAttributeFilterTestSuite runs the RefreshTokenAttributeFilterTestSuite.
func TestRefreshTokenAttributeFilterTestSuite(t *testing.T) {
	suite.Run(t, new(RefreshTokenAttributeFilterTestSuite))
}

// SetupSuite creates the shared organization unit, user type, auth flow, resource server,
// application, and test user for the suite.
func (ts *RefreshTokenAttributeFilterTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "rt-attr-filter-ou",
		Name:        "Refresh Token Attribute Filter OU",
		Description: "Organization unit for refresh token attribute allow-list integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	rtAttrFilterUserType.OUID = ouID
	schemaID, err := testutils.CreateUserType(rtAttrFilterUserType)
	ts.Require().NoError(err)
	ts.entityTypeID = schemaID

	flowID, err := testutils.CreateFlow(rtAttrFilterAuthFlow)
	ts.Require().NoError(err)
	ts.authFlowID = flowID

	rsID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name:        "Refresh Token Attribute Filter API",
		Description: "Resource server for refresh token attribute allow-list testing",
		Identifier:  "https://rt-attr-filter.example.com",
		OUID:        ts.ouID,
	}, []testutils.Action{})
	ts.Require().NoError(err)
	ts.resourceServerID = rsID

	ts.applicationID = ts.createTestApplication()

	attributesJSON, err := json.Marshal(map[string]interface{}{
		"username":    rtAttrFilterUsername,
		"password":    rtAttrFilterPassword,
		"given_name":  "Ada",
		"family_name": "Lovelace",
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		OUID:       ouID,
		Type:       "rt-attr-filter-person",
		Attributes: json.RawMessage(attributesJSON),
	})
	ts.Require().NoError(err)
	ts.userID = userID
}

// TearDownSuite deletes the resources created in SetupSuite.
func (ts *RefreshTokenAttributeFilterTestSuite) TearDownSuite() {
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.applicationID != "" {
		_ = testutils.DeleteApplication(ts.applicationID)
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

// createTestApplication creates the authorization_code/refresh_token application with a
// given_name-only userConfig.attributes allow-list.
func (ts *RefreshTokenAttributeFilterTestSuite) createTestApplication() string {
	app := map[string]interface{}{
		"name":                      "RefreshTokenAttrFilterTestApp",
		"description":               "Application for refresh token attribute allow-list testing",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"rt-attr-filter-person"},
		"inboundAuthConfig": []map[string]interface{}{
			{
				"type": "oauth2",
				"config": map[string]interface{}{
					"clientId":                rtAttrFilterClientID,
					"clientSecret":            rtAttrFilterClientSecret,
					"redirectUris":            []string{rtAttrFilterRedirectURI},
					"grantTypes":              []string{"authorization_code", "refresh_token"},
					"responseTypes":           []string{"code"},
					"tokenEndpointAuthMethod": "client_secret_basic",
					"token": map[string]interface{}{
						"accessToken": map[string]interface{}{
							"userConfig": map[string]interface{}{
								"attributes": []string{"given_name"},
							},
						},
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(jsonData))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(bodyBytes))

	var respData map[string]interface{}
	ts.Require().NoError(json.Unmarshal(bodyBytes, &respData))
	return respData["id"].(string)
}

// obtainTokensViaAuthCodeFlow performs the full authorization code flow and returns the token response.
func (ts *RefreshTokenAttributeFilterTestSuite) obtainTokensViaAuthCodeFlow() *testutils.TokenResponse {
	resp, err := testutils.InitiateAuthorizationFlowWithResource(
		rtAttrFilterClientID, rtAttrFilterRedirectURI, "code", "openid", "test-state",
		"https://rt-attr-filter.example.com")
	ts.Require().NoError(err)
	defer resp.Body.Close()
	ts.Require().Equal(http.StatusFound, resp.StatusCode)

	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location)

	authID, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": rtAttrFilterUsername,
		"password": rtAttrFilterPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus)
	ts.Require().NotEmpty(flowStep.Assertion)

	authzResp, err := testutils.CompleteAuthorization(authID, flowStep.Assertion)
	ts.Require().NoError(err)

	code, err := testutils.ExtractAuthorizationCode(authzResp.RedirectURI)
	ts.Require().NoError(err)

	tokenResult, err := testutils.RequestTokenWithResource(
		rtAttrFilterClientID, rtAttrFilterClientSecret, code, rtAttrFilterRedirectURI, "authorization_code",
		"https://rt-attr-filter.example.com")
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, tokenResult.StatusCode, string(tokenResult.Body))
	ts.Require().NotNil(tokenResult.Token)
	ts.Require().NotEmpty(tokenResult.Token.RefreshToken)

	return tokenResult.Token
}

// TestRefreshToken_AttributeAllowList verifies that only the allow-listed subject attribute
// (given_name) is present in the refreshed access token, and non-allow-listed attributes
// (family_name) are excluded.
func (ts *RefreshTokenAttributeFilterTestSuite) TestRefreshToken_AttributeAllowList() {
	tokenResponse := ts.obtainTokensViaAuthCodeFlow()

	refreshResponse, err := testutils.RefreshAccessToken(
		rtAttrFilterClientID, rtAttrFilterClientSecret, tokenResponse.RefreshToken)
	ts.Require().NoError(err, "refresh token request should succeed")
	ts.Require().NotNil(refreshResponse)
	ts.Require().NotEmpty(refreshResponse.AccessToken)

	claims, err := testutils.DecodeJWT(refreshResponse.AccessToken)
	ts.Require().NoError(err)
	ts.Require().Equal(ts.userID, claims.Sub)

	ts.Assert().Equal("Ada", claims.Additional["given_name"], "allow-listed attribute should be present")
	ts.Assert().NotContains(claims.Additional, "family_name", "non-allow-listed attribute must be excluded")
}

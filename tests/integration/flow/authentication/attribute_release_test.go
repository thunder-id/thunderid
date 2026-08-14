// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// This suite pins the attribute-release branch of AuthAssertExecutor. The branch key is the
// presence of the user_attributes_cache_ttl_seconds runtime value, which the OAuth authorization
// service injects when it starts the flow and a direct POST /flow/execute never does:
//
//   - App-Native (no runtime key): every resolved user attribute is inlined as a top-level
//     assertion claim and no aci claim is minted.
//   - OAuth-initiated (runtime key present): the resolved attributes go into the attribute cache
//     and the assertion only carries the aci reference to them.
//
// The same application and the same flow drive both cases, so the initiation path is the only
// variable. That is possible because the application is a mobile app with attestation dev mode
// enabled: dev mode lets it initiate a flow directly, while its authorization_code OAuth profile
// lets it go through GET /oauth2/authorize.

const (
	attrReleaseClientID    = "attr_release_branch_client"
	attrReleaseSecret      = "attr_release_branch_secret"
	attrReleaseRedirectURI = "https://localhost:3000/attr-release-callback"
	attrReleaseUsername    = "attr_release_user"
	attrReleasePassword    = "SecurePass123!"
	attrReleaseEmail       = "attr.release@test.com"
	attrReleaseGivenName   = "Attribute"
	attrReleaseFamilyName  = "Release"
)

var attrReleaseOU = testutils.OrganizationUnit{
	Handle:      "attr-release-branch-ou",
	Name:        "Attribute Release Branch OU",
	Description: "Organization unit for the attribute release branch tests",
	Parent:      nil,
}

var attrReleaseUserType = testutils.UserType{
	Name: "attr-release-person",
	Schema: map[string]interface{}{
		"username":    map[string]interface{}{"type": "string"},
		"password":    map[string]interface{}{"type": "string", "credential": true},
		"email":       map[string]interface{}{"type": "string"},
		"given_name":  map[string]interface{}{"type": "string"},
		"family_name": map[string]interface{}{"type": "string"},
	},
}

var attrReleaseFlow = testutils.Flow{
	Name:     "Attribute Release Branch Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_attr_release_branch",
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

// attrReleaseApp is a mobile application with attestation dev mode enabled so the very same
// application can initiate a flow directly (App-Native) and through GET /oauth2/authorize.
var attrReleaseApp = testutils.Application{
	Name:                      "Attribute Release Branch App",
	Description:               "Application for the attribute release branch tests",
	Type:                      "mobile",
	IsRegistrationFlowEnabled: false,
	RedirectURIs:              []string{attrReleaseRedirectURI},
	AllowedUserTypes:          []string{"attr-release-person"},
	Attestation:               map[string]interface{}{"devMode": true},
	AssertionConfig: map[string]interface{}{
		"userAttributes": []string{"email", "given_name", "family_name"},
	},
	InboundAuthConfig: []map[string]interface{}{
		{
			"type": "oauth2",
			"config": map[string]interface{}{
				"clientId":                attrReleaseClientID,
				"clientSecret":            attrReleaseSecret,
				"redirectUris":            []string{attrReleaseRedirectURI},
				"grantTypes":              []string{"authorization_code"},
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
					"profile": {"given_name", "family_name"},
					"email":   {"email"},
				},
			},
		},
	},
}

type AttributeReleaseBranchTestSuite struct {
	suite.Suite
	ouID       string
	userTypeID string
	userID     string
	flowID     string
	appID      string
}

func TestAttributeReleaseBranchTestSuite(t *testing.T) {
	suite.Run(t, new(AttributeReleaseBranchTestSuite))
}

func (ts *AttributeReleaseBranchTestSuite) SetupSuite() {
	ouID, err := testutils.CreateOrganizationUnit(attrReleaseOU)
	ts.Require().NoError(err, "Failed to create organization unit")
	ts.ouID = ouID

	attrReleaseUserType.OUID = ts.ouID
	userTypeID, err := testutils.CreateUserType(attrReleaseUserType)
	ts.Require().NoError(err, "Failed to create user type")
	ts.userTypeID = userTypeID

	flowID, err := testutils.CreateFlow(attrReleaseFlow)
	ts.Require().NoError(err, "Failed to create authentication flow")
	ts.flowID = flowID

	app := attrReleaseApp
	app.OUID = ts.ouID
	app.AuthFlowID = flowID
	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "Failed to create application")
	ts.appID = appID

	attributes, err := json.Marshal(map[string]interface{}{
		"username":    attrReleaseUsername,
		"password":    attrReleasePassword,
		"email":       attrReleaseEmail,
		"given_name":  attrReleaseGivenName,
		"family_name": attrReleaseFamilyName,
	})
	ts.Require().NoError(err, "Failed to marshal user attributes")

	userID, err := testutils.CreateUser(testutils.User{
		Type:       attrReleaseUserType.Name,
		OUID:       ts.ouID,
		Attributes: json.RawMessage(attributes),
	})
	ts.Require().NoError(err, "Failed to create user")
	ts.userID = userID
}

func (ts *AttributeReleaseBranchTestSuite) TearDownSuite() {
	if ts.userID != "" {
		if err := testutils.DeleteUser(ts.userID); err != nil {
			ts.T().Logf("Failed to delete user during teardown: %v", err)
		}
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete application during teardown: %v", err)
		}
	}
	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("Failed to delete flow during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}
}

// authenticateAppNatively drives the flow with a direct POST /flow/execute and returns the
// assertion. No OAuth component is involved, so user_attributes_cache_ttl_seconds is never set.
func (ts *AttributeReleaseBranchTestSuite) authenticateAppNatively() string {
	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false,
		map[string]string{"applicationId": ts.appID}, "")
	ts.Require().NoError(err, "Failed to initiate app-native flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus, "Flow should start incomplete")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID, map[string]string{
		"username": attrReleaseUsername,
		"password": attrReleasePassword,
	}, "action_001", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete app-native flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "App-native flow should complete")
	ts.Require().NotEmpty(flowStep.Assertion, "App-native flow should return an assertion")

	return flowStep.Assertion
}

// authenticateThroughAuthorizeEndpoint drives the same flow through GET /oauth2/authorize and
// returns the assertion produced at the end of the flow. The authorization service seeds
// user_attributes_cache_ttl_seconds into the flow runtime data on this path.
func (ts *AttributeReleaseBranchTestSuite) authenticateThroughAuthorizeEndpoint() string {
	resp, err := testutils.InitiateAuthorizationFlow(attrReleaseClientID, attrReleaseRedirectURI,
		"code", "openid profile email", "attr-release-state")
	ts.Require().NoError(err, "Failed to call the authorize endpoint")
	defer resp.Body.Close()

	ts.Require().Equal(http.StatusFound, resp.StatusCode, "Authorize endpoint should redirect to the flow")
	location := resp.Header.Get("Location")
	ts.Require().NotEmpty(location, "Authorize response should carry a Location header")

	_, executionID, err := testutils.ExtractAuthData(location)
	ts.Require().NoError(err, "Failed to extract the execution ID from the authorize redirect")

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err, "Failed to execute the initial OAuth-initiated flow step")

	flowStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": attrReleaseUsername,
		"password": attrReleasePassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete the OAuth-initiated flow")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus, "OAuth-initiated flow should complete")
	ts.Require().NotEmpty(flowStep.Assertion, "OAuth-initiated flow should return an assertion")

	return flowStep.Assertion
}

// TestAppNativeFlow_InlinesUserAttributesAndOmitsACI covers case 5: with no attribute cache TTL in
// runtime data, every resolved user attribute is a top-level assertion claim and no aci is minted.
func (ts *AttributeReleaseBranchTestSuite) TestAppNativeFlow_InlinesUserAttributesAndOmitsACI() {
	claims, err := testutils.DecodeJWTPayloadMap(ts.authenticateAppNatively())
	ts.Require().NoError(err, "Failed to decode the app-native assertion")

	ts.Equal(attrReleaseEmail, claims["email"], "email should be inlined in the assertion")
	ts.Equal(attrReleaseGivenName, claims["given_name"], "given_name should be inlined in the assertion")
	ts.Equal(attrReleaseFamilyName, claims["family_name"], "family_name should be inlined in the assertion")

	_, hasACI := claims["aci"]
	ts.False(hasACI, fmt.Sprintf("App-native assertion should not carry an aci claim, got claims %v", claims))
}

// TestOAuthInitiatedFlow_ReferencesAttributeCacheViaACI covers case 6: the same application and the
// same flow, started through the authorize endpoint, cache the attributes and carry only the aci
// reference in the assertion.
func (ts *AttributeReleaseBranchTestSuite) TestOAuthInitiatedFlow_ReferencesAttributeCacheViaACI() {
	claims, err := testutils.DecodeJWTPayloadMap(ts.authenticateThroughAuthorizeEndpoint())
	ts.Require().NoError(err, "Failed to decode the OAuth-initiated assertion")

	aci, hasACI := claims["aci"]
	ts.Require().True(hasACI, fmt.Sprintf("OAuth-initiated assertion should carry an aci claim, got %v", claims))
	ts.NotEmpty(aci, "aci claim should not be empty")

	for _, attribute := range []string{"email", "given_name", "family_name"} {
		_, present := claims[attribute]
		ts.False(present, "OAuth-initiated assertion should not inline "+attribute)
	}
}

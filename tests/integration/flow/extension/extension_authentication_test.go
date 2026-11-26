// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	extensionAuthenticationExecutorName = "ExtensionAuthenticationExecutor"
	authenticationEndpointPath          = "/connections/authentication-endpoint"

	authNode = "ext_auth"

	authEndpointFederatedUsername = "ext-auth-federated-user"
	authEndpointLocalUsername     = "extauthuser"
	authEndpointLocalPassword     = "ExtAuthPass123!"

	pinPageURL    = "https://pin.example.com/entry?flowId=1"
	pinReturnCode = "pin-return-code"

	errCodeUserNotAuthenticated = "FET-1004"
)

// redirectAnswer is the service's first answer when it sends the browser to its own page.
var redirectAnswer = okAnswer(`{"actionStatus":"INCOMPLETE","redirectUrl":"` + pinPageURL + `",` +
	`"challengeId":"ch-1"}`)

var (
	authEndpointOU = testutils.OrganizationUnit{
		Handle:      "ext-auth-endpoint-ou",
		Name:        "Extension Authentication Endpoint Test OU",
		Description: "OU for authentication through an authentication endpoint connection",
	}

	authEndpointUserType = testutils.UserType{
		Handle:                "ext_auth_endpoint_person",
		DisplayName:           "Extension Authentication Endpoint Test Person",
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username":   map[string]interface{}{"type": "string", "unique": true},
			"email":      map[string]interface{}{"type": "string"},
			"given_name": map[string]interface{}{"type": "string"},
			"password":   map[string]interface{}{"type": "string", "credential": true},
		},
	}

	authEndpointUser = testutils.User{
		Type: "ext_auth_endpoint_person",
		Attributes: json.RawMessage(`{
			"username": "` + authEndpointLocalUsername + `",
			"password": "` + authEndpointLocalPassword + `",
			"email": "extauthuser@example.com"
		}`),
	}

	authEndpointApp = testutils.Application{
		Name:                      "Extension Authentication Endpoint Test Application",
		Description:               "Application signed in to through an authentication endpoint connection",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "ext_auth_endpoint_client",
		ClientSecret:              "ext_auth_endpoint_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"ext_auth_endpoint_person"},
	}
)

// ExtensionAuthenticationTestSuite covers the extension authentication executor and its authentication

// already known locally, for one who is not yet, and as a second factor after a first one.
type ExtensionAuthenticationTestSuite struct {
	suite.Suite
	service      *mockService
	fixtures     *identityFixtures
	connectionID string
}

func TestExtensionAuthenticationTestSuite(t *testing.T) {
	suite.Run(t, new(ExtensionAuthenticationTestSuite))
}

func (ts *ExtensionAuthenticationTestSuite) SetupSuite() {
	ts.service = newMockService(seedValues{})
	ts.fixtures = createIdentityFixtures(&ts.Suite, authEndpointOU, authEndpointUserType, authEndpointUser)

	status, body := apiRequest(&ts.Suite, http.MethodPost, authenticationEndpointPath, map[string]interface{}{
		"name":      "ext-auth-endpoint-connection",
		"ouId":      ts.fixtures.ouID,
		"url":       ts.service.connectionURL(),
		"timeoutMs": 5000,
		"attributeConfiguration": testutils.AttributeConfiguration{
			UserTypeResolution: &testutils.UserTypeResolution{Default: authEndpointUserType.Handle},
			UserTypeAttributeMappings: []testutils.UserTypeAttributeMapping{{
				UserType: authEndpointUserType.Handle,
				Attributes: []testutils.AttributeMapping{
					{ExternalAttribute: "username", LocalAttribute: "username"},
					{ExternalAttribute: "email", LocalAttribute: "email"},
					{ExternalAttribute: "given_name", LocalAttribute: "given_name"},
				},
			}},
			AccountLinking: &testutils.AccountLinking{Attributes: []string{"username"}},
		},
	})
	ts.Require().Equal(http.StatusCreated, status, "Failed to create the authentication endpoint connection: %s",
		string(body))
	var created map[string]interface{}
	ts.Require().NoError(json.Unmarshal(body, &created))
	ts.connectionID, _ = created["id"].(string)
	ts.Require().NotEmpty(ts.connectionID)

	flowID, err := testutils.CreateFlow(ts.federatedFlow("ext_auth_endpoint_bootstrap_flow"))
	ts.Require().NoError(err, "Failed to create the bootstrap flow")
	ts.fixtures.createApplication(&ts.Suite, authEndpointApp, flowID)
	ts.fixtures.authFlowID = flowID
}

func (ts *ExtensionAuthenticationTestSuite) TearDownSuite() {
	ts.fixtures.deleteApplication(&ts.Suite)
	if ts.fixtures != nil && ts.fixtures.authFlowID != "" {
		if err := testutils.DeleteFlow(ts.fixtures.authFlowID); err != nil {
			ts.T().Logf("Failed to delete the bootstrap flow: %v", err)
		}
	}
	if ts.connectionID != "" {
		if status, body := apiRequest(&ts.Suite, http.MethodDelete,
			authenticationEndpointPath+"/"+ts.connectionID, nil); status != http.StatusNoContent {
			ts.T().Logf("Failed to delete the authentication endpoint connection: %d %s", status, string(body))
		}
	}
	ts.fixtures.deleteIdentities(&ts.Suite)
	if ts.service != nil {
		ts.service.close()
	}
}

func (ts *ExtensionAuthenticationTestSuite) SetupTest() {
	ts.service.reset()
}

func (ts *ExtensionAuthenticationTestSuite) authStep(properties map[string]interface{},
	routes map[string]string) map[string]interface{} {
	props := map[string]interface{}{"connectionId": ts.connectionID}
	for key, value := range properties {
		props[key] = value
	}
	node := extensionNode(authNode, props, routes)
	node["executor"] = map[string]interface{}{"name": extensionAuthenticationExecutorName}
	return node
}

// federatedFlow is for users who may not exist yet:
//
//	start -> ext_auth (allowAuthenticationWithoutLocalUser) -> provision -> authorize -> auth_assert -> end
func (ts *ExtensionAuthenticationTestSuite) federatedFlow(handle string) testutils.Flow {
	return testutils.Flow{
		Name:     "Extension Authentication Federated Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			startNode(authNode),
			ts.authStep(map[string]interface{}{"allowAuthenticationWithoutLocalUser": true},
				map[string]string{"onSuccess": "provision"}),
			{
				"id":        "provision",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "auth_assert",
			},
			authAssertNode(),
			endNode(),
		},
	}
}

// signInOnlyFlow signs in whoever the connection resolves and provisions no one.
func (ts *ExtensionAuthenticationTestSuite) signInOnlyFlow(handle string) testutils.Flow {
	return testutils.Flow{
		Name:     "Extension Authentication Sign In Only Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			startNode(authNode),
			ts.authStep(nil, map[string]string{"onSuccess": "auth_assert"}),
			authAssertNode(),
			endNode(),
		},
	}
}

// secondFactorFlow runs the organization's service after a password sign-in:
//
//	start -> prompt_credentials -> credentials_auth -> ext_auth -> linking -> auth_assert -> end
func (ts *ExtensionAuthenticationTestSuite) secondFactorFlow(handle string) testutils.Flow {
	return testutils.Flow{
		Name:     "Extension Authentication Second Factor Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: append([]map[string]interface{}{
			startNode("prompt_credentials"),
			promptCredentialsNode("credentials_auth"),
			credentialsAuthNode(authNode),
			ts.authStep(nil, map[string]string{"onSuccess": "linking"}),
		}, linkingNodes()...),
	}
}

func linkingNode() map[string]interface{} {
	return map[string]interface{}{
		"id":           "linking",
		"type":         "TASK_EXECUTION",
		"executor":     map[string]interface{}{"name": "AccountLinkingExecutor"},
		"onSuccess":    "auth_assert",
		"onIncomplete": "linking_prompt",
	}
}

// linkingFlow is for a user who exists locally. The first sign-in asks the user to confirm the match and
// prove the local account with its password, which records the link; later sign-ins need neither.
//
//	start -> ext_auth -> linking -> (prompt, verify) -> linking -> auth_assert -> end
func (ts *ExtensionAuthenticationTestSuite) linkingFlow(handle string) testutils.Flow {
	return testutils.Flow{
		Name:     "Extension Authentication Linking Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: append([]map[string]interface{}{
			startNode(authNode),
			ts.authStep(nil, map[string]string{"onSuccess": "linking"}),
		}, linkingNodes()...),
	}
}

// linkingNodes are the nodes from linking to the end: the linking step, its confirmation prompt, the
// password verification it falls back to, and the assertion.
func linkingNodes() []map[string]interface{} {
	return []map[string]interface{}{
		linkingNode(),
		{
			"id":   "linking_prompt",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{"action": map[string]interface{}{
					"ref": common.LinkAccountAction, "type": "CONFIRM", "nextNode": "verify_prompt"}},
				{"action": map[string]interface{}{
					"ref": common.SeparateAccountAction, "type": "REJECT", "nextNode": "linking"}},
			},
		},
		promptCredentialsNodeNamed("verify_prompt", "verify_auth", common.VerifyPasswordAction),
		{
			"id":           "verify_auth",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
			"onSuccess":    "linking",
			"onIncomplete": "verify_prompt",
		},
		authAssertNode(),
		endNode(),
	}
}

func promptCredentialsNodeNamed(id, next, action string) map[string]interface{} {
	return map[string]interface{}{
		"id":   id,
		"type": "PROMPT",
		"prompts": []map[string]interface{}{
			{
				"inputs": credentialInputs,
				"action": map[string]interface{}{"ref": action, "nextNode": next},
			},
		},
	}
}

func (ts *ExtensionAuthenticationTestSuite) useFlow(flow testutils.Flow) {
	ts.T().Helper()
	ts.fixtures.bindFlow(&ts.Suite, createTestFlow(&ts.Suite, flow))
}

func verifiedAnswer(subject, username string) serviceAnswer {
	return okAnswer(`{"actionStatus":"SUCCESS","authenticatedUser":{"subject":"` + subject + `","attributes":{` +
		`"username":"` + username + `","email":"` + username + `@example.com","given_name":"Peter"}}}`)
}

func (ts *ExtensionAuthenticationTestSuite) deleteUserOnCleanup(username string) {
	ts.T().Cleanup(func() {
		if user, err := testutils.FindUserByAttribute("username", username); err == nil && user != nil {
			if err := testutils.CleanupUsers([]string{user.ID}); err != nil {
				ts.T().Logf("Failed to delete user %s: %v", username, err)
			}
		}
	})
}

func (ts *ExtensionAuthenticationTestSuite) signedInSubject(result *common.FlowStep, raw string) string {
	ts.T().Helper()
	ts.Require().Nil(result.Error, "A completed flow should carry no error: %s", raw)
	ts.Require().Equal("COMPLETE", result.FlowStatus, "Flow should complete: %s", raw)
	claims, err := testutils.DecodeJWT(result.Assertion)
	ts.Require().NoError(err, "The assertion should be a decodable JWT")
	return claims.Sub
}

func (ts *ExtensionAuthenticationTestSuite) TestFederatedFirstSignInProvisionsAndTheNextFindsTheUser() {
	ts.useFlow(ts.federatedFlow("ext_auth_endpoint_federated_flow"))
	ts.deleteUserOnCleanup(authEndpointFederatedUsername)
	ts.service.answerInTurn(authNode, verifiedAnswer("ext-sub-1", authEndpointFederatedUsername))

	step, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", step.FlowStatus, "One call should provision and sign the user in: %+v", step)

	user, err := testutils.FindUserByAttribute("username", authEndpointFederatedUsername)
	ts.Require().NoError(err)
	ts.Require().NotNil(user, "The first sign-in should create the user")
	claims, err := testutils.DecodeJWT(step.Assertion)
	ts.Require().NoError(err)
	ts.Equal(user.ID, claims.Sub)
	attrs, err := testutils.GetUserAttributes(*user)
	ts.Require().NoError(err)
	ts.Equal(authEndpointFederatedUsername+"@example.com", attrs["email"], "Mapped attributes are taken")
	ts.Equal("Peter", attrs["given_name"], "Mapped attributes are taken")

	ts.service.reset()
	ts.service.answerInTurn(authNode, verifiedAnswer("ext-sub-1", authEndpointFederatedUsername))
	again, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Equal(user.ID, ts.signedInSubject(again, ""), "The second sign-in finds the same user through the recorded link")
	sameUser, err := testutils.FindUserByAttribute("username", authEndpointFederatedUsername)
	ts.Require().NoError(err)
	ts.Equal(user.ID, sameUser.ID, "No second user is provisioned")
}

func (ts *ExtensionAuthenticationTestSuite) TestRedirectResumesAndThenAuthenticates() {
	ts.useFlow(ts.federatedFlow("ext_auth_endpoint_redirect_flow"))
	ts.deleteUserOnCleanup(authEndpointFederatedUsername)
	ts.service.answerInTurn(authNode, redirectAnswer,
		verifiedAnswer("ext-sub-redirect", authEndpointFederatedUsername))

	step, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("REDIRECTION", step.Type, "The service's redirect should reach the browser: %+v", step)
	ts.Equal(pinPageURL, step.Data.RedirectURL)
	ts.Empty(step.Assertion, "No one is signed in before the service verifies a user")

	result, raw := submit(&ts.Suite, step, map[string]string{codeInput: pinReturnCode}, "")

	ts.Require().Equal("COMPLETE", result.FlowStatus, "The return should provision and sign the user in: %s", raw)
	calls := ts.service.callsFrom(authNode)
	ts.Require().Len(calls, 2)
	ts.Equal(calls[0].Body.ExecutionID, calls[1].Body.ExecutionID, "Both calls carry the same executionId")
}

func (ts *ExtensionAuthenticationTestSuite) TestKnownLocalUserNamedByUsernameSignsInDirectly() {
	ts.useFlow(ts.signInOnlyFlow("ext_auth_endpoint_known_user_flow"))
	ts.service.answerInTurn(authNode, verifiedAnswer("ext-sub-known", authEndpointLocalUsername))

	step, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err)

	ts.Equal(ts.fixtures.userID, ts.signedInSubject(step, ""), "The service is trusted: no link, no password prompt")
	ts.Len(ts.service.callsFrom(authNode), 1)
}

func (ts *ExtensionAuthenticationTestSuite) TestKnownLocalUserNamedByUserIDSignsInDirectly() {
	ts.useFlow(ts.signInOnlyFlow("ext_auth_endpoint_known_user_id_flow"))
	ts.service.answerInTurn(authNode, okAnswer(`{"actionStatus":"SUCCESS","authenticatedUser":{"subject":"ext-sub-id",`+
		`"attributes":{"userID":"`+ts.fixtures.userID+`","username":"ignored-name"}}}`))

	step, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err)

	ts.Equal(ts.fixtures.userID, ts.signedInSubject(step, ""), "A user ID names the user, whatever the username says")
}

func (ts *ExtensionAuthenticationTestSuite) TestKnownLocalUserNeedsNoPromptInAFlowThatHasLinking() {
	ts.useFlow(ts.linkingFlow("ext_auth_endpoint_linking_flow"))
	ts.service.answerInTurn(authNode, verifiedAnswer("ext-sub-link", authEndpointLocalUsername))

	step, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err)

	ts.Equal(ts.fixtures.userID, ts.signedInSubject(step, ""), "A user the service names is already resolved")
}

func (ts *ExtensionAuthenticationTestSuite) TestNoLocalUserSignsInNoOneWithoutProvisioning() {
	ts.useFlow(ts.signInOnlyFlow("ext_auth_endpoint_unknown_user_flow"))
	ts.service.answerInTurn(authNode, verifiedAnswer("ext-sub-unknown", "nobody-by-that-name"))

	step, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err)

	ts.Empty(step.Assertion, "No local user matches and the flow does not provision: %+v", step)
	ts.NotEqual("COMPLETE", step.FlowStatus)
}

func (ts *ExtensionAuthenticationTestSuite) TestSecondFactorKeepsTheSignedInUser() {
	ts.useFlow(ts.secondFactorFlow("ext_auth_endpoint_second_factor_flow"))
	ts.service.answerInTurn(authNode, verifiedAnswer("ext-sub-mfa", authEndpointLocalUsername))

	result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))

	ts.Equal(ts.fixtures.userID, ts.signedInSubject(result, raw), "The service confirms the password user, with no prompt")
	calls := ts.service.callsFrom(authNode)
	ts.Require().Len(calls, 1)
	ts.Contains(calls[0].RawBody, `"entityId":"`+ts.fixtures.userID+`"`, "The service is told who signed in")
}

func (ts *ExtensionAuthenticationTestSuite) TestSecondFactorNamingAnotherExistingUserIsRefused() {
	ts.useFlow(ts.secondFactorFlow("ext_auth_endpoint_other_user_flow"))
	otherID, err := testutils.CreateUser(testutils.User{
		Type: authEndpointUser.Type, OUID: ts.fixtures.ouID,
		Attributes: json.RawMessage(`{"username": "extauthother", "password": "ExtAuthOther123!"}`),
	})
	ts.Require().NoError(err)
	ts.T().Cleanup(func() { _ = testutils.CleanupUsers([]string{otherID}) })
	ts.service.answerInTurn(authNode, verifiedAnswer("ext-sub-other-user", "extauthother"))

	result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))

	ts.Empty(result.Assertion, "The service must not switch the signed-in user: %s", raw)
	ts.NotEqual("COMPLETE", result.FlowStatus, raw)
}

func (ts *ExtensionAuthenticationTestSuite) TestSecondFactorForAnotherIdentityIsRefused() {
	ts.useFlow(ts.secondFactorFlow("ext_auth_endpoint_other_identity_flow"))
	ts.service.answerInTurn(authNode, verifiedAnswer("ext-sub-other", "someone-else"))

	result, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))

	ts.Empty(result.Assertion, "A different identity must not sign anyone in: %s", raw)
	ts.NotEqual("COMPLETE", result.FlowStatus, raw)
}

func (ts *ExtensionAuthenticationTestSuite) TestServiceAnswerWithoutAVerifiedUserSignsInNoOne() {
	ts.useFlow(ts.signInOnlyFlow("ext_auth_endpoint_no_user_flow"))
	ts.service.answerInTurn(authNode, okAnswer(`{"actionStatus":"SUCCESS"}`))

	step, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err)

	ts.Empty(step.Assertion)
	ts.Equal("ERROR", step.FlowStatus, "%+v", step)
	ts.Require().NotNil(step.Error)
	ts.Equal(errCodeUserNotAuthenticated, step.Error.Code)
}

func (ts *ExtensionAuthenticationTestSuite) TestFlowNamingAMissingConnectionIsRefusedAtSave() {
	node := ts.authStep(nil, map[string]string{"onSuccess": "auth_assert"})
	node["properties"] = map[string]interface{}{"connectionId": "00000000-0000-7000-8000-00000000dead"}

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Extension Authentication Missing Connection Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "ext_auth_endpoint_missing_connection_flow",
		Nodes:    []map[string]interface{}{startNode(authNode), node, authAssertNode(), endNode()},
	})
	if err == nil {
		_ = testutils.DeleteFlow(flowID)
	}

	ts.Error(err, "A flow naming a connection that does not exist is refused")
}

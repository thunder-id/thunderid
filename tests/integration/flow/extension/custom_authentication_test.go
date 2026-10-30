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
	customAuthConnectionName = "ext-custom-auth-service"
	customAuthTimeoutMS      = 5000

	extensionAuthResolverName = "ExtensionAuthResolver"

	// pinNode is the extension step that sends the user to the PIN service's page.
	pinNode = "ext_pin"
	// pinPageURL is where the PIN service's first answer sends the browser.
	pinPageURL = "https://pin.example.com/entry?flowId=1"
	// pinReturnCode is the code the PIN page hands back through the gate's callback.
	pinReturnCode = "pin-return-code"

	errCodeEntityNotFound          = "FET-1001"
	errCodeAmbiguousEntity         = "FET-1003"
	errCodeUserNotAuthenticated    = "FET-1004"
	errCodeExtensionReturnRefused  = "FET-1126"
	errCodeInvalidExecutorConfig   = "FLM-1023"
	customAuthPartnerUserTypeName  = "ext_custom_auth_partner"
	customAuthResourceServerHandle = "ext-custom-auth-documents"

	// provisionedUsername is the username the PIN service verifies for a person ThunderID does not know.
	provisionedUsername = "ext-custom-auth-new-user"
)

var (
	customAuthOU = testutils.OrganizationUnit{
		Handle:      "ext-custom-auth-ou",
		Name:        "Extension Custom Authentication Test OU",
		Description: "OU for custom authentication through an extension",
	}

	customAuthUserType = testutils.UserType{
		Handle:                "ext_custom_auth_person",
		DisplayName:           "Extension Custom Authentication Test Person",
		AllowSelfRegistration: true,
		Schema: map[string]interface{}{
			"username":    map[string]interface{}{"type": "string", "unique": true},
			"email":       map[string]interface{}{"type": "string"},
			"given_name":  map[string]interface{}{"type": "string"},
			"family_name": map[string]interface{}{"type": "string"},
			"password":    map[string]interface{}{"type": "string", "credential": true},
		},
	}

	customAuthUser = testutils.User{
		Type: "ext_custom_auth_person",
		Attributes: json.RawMessage(`{
			"username": "extcustomauthuser",
			"password": "ExtCustomAuthPass123!",
			"email": "extcustomauthuser@example.com"
		}`),
	}

	customAuthApp = testutils.Application{
		Name:                      "Extension Custom Authentication Test Application",
		Description:               "Application signed in to through an organization's service",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "ext_custom_auth_client",
		ClientSecret:              "ext_custom_auth_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"ext_custom_auth_person"},
	}
)

// redirectAnswer is the PIN service's first answer: send the browser to its page.
var redirectAnswer = okAnswer(`{"actionStatus":"INCOMPLETE","redirectUrl":"` + pinPageURL + `",` +
	`"challengeId":"ch-1"}`)

// CustomAuthenticationTestSuite covers the redirect to a service's page (AC4.5 to AC4.7) and the three
// custom authentication patterns through the extension auth resolver (R9).
type CustomAuthenticationTestSuite struct {
	suite.Suite
	service      *mockService
	fixtures     *identityFixtures
	connectionID string
	flowIDs      []string
}

func TestCustomAuthenticationTestSuite(t *testing.T) {
	suite.Run(t, new(CustomAuthenticationTestSuite))
}

func (ts *CustomAuthenticationTestSuite) SetupSuite() {
	ts.service = newMockService(seedValues{})
	ts.fixtures = createIdentityFixtures(&ts.Suite, customAuthOU, customAuthUserType, customAuthUser)

	connectionID, err := testutils.CreateHTTPConnection(testutils.HTTPConnection{
		Name: customAuthConnectionName, OUID: ts.fixtures.ouID, URL: ts.service.connectionURL(),
		TimeoutMS: customAuthTimeoutMS,
	})
	ts.Require().NoError(err, "Failed to create the HTTP endpoint connection")
	ts.connectionID = connectionID

	flowID, err := testutils.CreateFlow(ts.existingUserFlow("ext_custom_auth_flow"))
	ts.Require().NoError(err, "Failed to create the custom authentication flow")
	ts.flowIDs = append(ts.flowIDs, flowID)
	ts.fixtures.createApplication(&ts.Suite, customAuthApp, flowID)
}

func (ts *CustomAuthenticationTestSuite) TearDownSuite() {
	ts.fixtures.deleteApplication(&ts.Suite)
	for _, flowID := range ts.flowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow %s during teardown: %v", flowID, err)
		}
	}
	if ts.connectionID != "" {
		if err := testutils.DeleteHTTPConnection(ts.connectionID); err != nil {
			ts.T().Logf("Failed to delete HTTP connection during teardown: %v", err)
		}
	}
	ts.fixtures.deleteIdentities(&ts.Suite)
	if ts.service != nil {
		ts.service.close()
	}
}

func (ts *CustomAuthenticationTestSuite) SetupTest() {
	ts.service.reset()
}

// pinStep is the extension step that calls the PIN service, with routes as needed.
func (ts *CustomAuthenticationTestSuite) pinStep(properties map[string]interface{},
	routes map[string]string) map[string]interface{} {
	props := map[string]interface{}{"connectionId": ts.connectionID}
	for key, value := range properties {
		props[key] = value
	}
	return extensionNode(pinNode, props, routes)
}

func resolverNode(properties map[string]interface{}, onSuccess string) map[string]interface{} {
	return map[string]interface{}{
		"id":         "resolve_user",
		"type":       "TASK_EXECUTION",
		"executor":   map[string]interface{}{"name": extensionAuthResolverName},
		"properties": properties,
		"onSuccess":  onSuccess,
	}
}

func authorizationNode() map[string]interface{} {
	return map[string]interface{}{
		"id":        "authorize",
		"type":      "TASK_EXECUTION",
		"executor":  map[string]interface{}{"name": "AuthorizationExecutor"},
		"onSuccess": "auth_assert",
	}
}

// existingUserFlow is pattern 1, for users who exist in ThunderID:
//
//	start -> ext_pin -> resolve_user (userID) -> authorize -> auth_assert -> end
//
// ext_pin names code in inputKeys and its own stored challengeId and a would-be redirectUrl in
// runtimeDataKeys, so its second call shows what its redirect answer stored. Its onIncomplete and
// onFailure lead to a code prompt, so a redirect is proved not to be taken there.
func (ts *CustomAuthenticationTestSuite) existingUserFlow(handle string) testutils.Flow {
	return testutils.Flow{
		Name:     "Extension Custom Authentication Existing User Flow",
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			startNode(pinNode),
			ts.pinStep(map[string]interface{}{
				"inputKeys":       []string{codeInput},
				"responseKeys":    []string{"verifiedUserId", "challengeId"},
				"runtimeDataKeys": []string{extensionKey(pinNode, "challengeId"), extensionKey(pinNode, "redirectUrl")},
			}, map[string]string{"onSuccess": "resolve_user", "onFailure": "prompt_code", "onIncomplete": "prompt_code"}),
			codePromptNode("prompt_code", pinNode),
			resolverNode(map[string]interface{}{
				"sourceKey":      extensionKey(pinNode, "verifiedUserId"),
				"matchAttribute": "userID",
			}, "authorize"),
			authorizationNode(),
			authAssertNode(),
			endNode(),
		},
	}
}

// useFlow creates flow and binds the application to it until the test ends.
func (ts *CustomAuthenticationTestSuite) useFlow(flow testutils.Flow) {
	ts.T().Helper()
	ts.fixtures.bindFlow(&ts.Suite, createTestFlow(&ts.Suite, flow))
}

// startAtPinPage initiates the flow and checks the PIN service sent the browser to its page.
func (ts *CustomAuthenticationTestSuite) startAtPinPage() *common.FlowStep {
	ts.T().Helper()
	step, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.assertRedirectedToPinPage(step)
	return step
}

func (ts *CustomAuthenticationTestSuite) assertRedirectedToPinPage(step *common.FlowStep) {
	ts.T().Helper()
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "The flow should wait for the browser to return")
	ts.Require().Equal("REDIRECTION", step.Type, "The step should send the browser to the service's page")
	ts.Equal(pinPageURL, step.Data.RedirectURL, "The browser should go to the service's redirectUrl")
	ts.Nil(step.Error, "A redirect shows no error")
	ts.Empty(step.Assertion, "No one is signed in before the service answers")
}

// returnFromPinPage resumes the flow as the gate does when the browser comes back from the service's
// page: the same execution, its latest challenge token and the code the page handed back.
func (ts *CustomAuthenticationTestSuite) returnFromPinPage(step *common.FlowStep) (*common.FlowStep, string) {
	ts.T().Helper()
	return submit(&ts.Suite, step, map[string]string{codeInput: pinReturnCode}, "")
}

// assertSignedInAs checks the flow completed with an assertion for userID.
func (ts *CustomAuthenticationTestSuite) assertSignedInAs(result *common.FlowStep, raw, userID string) {
	ts.T().Helper()
	ts.Require().Nil(result.Error, "A completed flow should carry no error: %s", raw)
	ts.Require().Equal("COMPLETE", result.FlowStatus, "Flow should complete: %s", raw)
	claims, err := testutils.DecodeJWT(result.Assertion)
	ts.Require().NoError(err, "The assertion should be a decodable JWT")
	ts.Equal(userID, claims.Sub, "The assertion subject should be the user the service verified")
}

// assertEndedWith checks the flow ended with code and signed no one in.
func (ts *CustomAuthenticationTestSuite) assertEndedWith(result *common.FlowStep, raw, code string) {
	ts.T().Helper()
	ts.Equal("ERROR", result.FlowStatus, "The flow should end: %s", raw)
	ts.Empty(result.Assertion, "No one may be signed in: %s", raw)
	ts.Require().NotNil(result.Error, "The flow should report why it ended: %s", raw)
	ts.Equal(code, result.Error.Code, "Unexpected error: %s", raw)
}

func (ts *CustomAuthenticationTestSuite) TestRedirectResumesTheSameExecutionAndSignsInTheVerifiedUser() {
	ts.service.answerInTurn(pinNode, redirectAnswer,
		okAnswer(`{"actionStatus":"SUCCESS","verifiedUserId":"`+ts.fixtures.userID+`"}`))

	step := ts.startAtPinPage()
	result, raw := ts.returnFromPinPage(step)

	ts.assertSignedInAs(result, raw, ts.fixtures.userID)
	calls := ts.service.callsFrom(pinNode)
	ts.Require().Len(calls, 2, "The step should call the service again when the browser returns")
	ts.Equal(step.ExecutionID, calls[0].Body.ExecutionID)
	ts.Equal(calls[0].Body.ExecutionID, calls[1].Body.ExecutionID, "Both calls carry the same executionId")
	ts.Equal(map[string]string{}, calls[0].Body.UserInputs, "The first call has no code yet")
	ts.Equal(map[string]string{codeInput: pinReturnCode}, calls[1].Body.UserInputs,
		"The returned code is sent because inputKeys names it")
	ts.Equal(map[string]string{extensionKey(pinNode, "challengeId"): "ch-1"}, calls[1].Body.RuntimeData,
		"The redirect answer's fields are stored, and redirectUrl is never stored")
}

func (ts *CustomAuthenticationTestSuite) TestReturnedCodeIsForwardedEvenWhenNotNamed() {
	ts.useFlow(testutils.Flow{
		Name:     "Extension Custom Authentication Unnamed Code Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "ext_custom_auth_unnamed_code_flow",
		Nodes: []map[string]interface{}{
			startNode(pinNode),
			ts.pinStep(nil, map[string]string{"onSuccess": "resolve_user"}),
			resolverNode(map[string]interface{}{
				"sourceKey": extensionKey(pinNode, "verifiedUserId"), "matchAttribute": "userID",
			}, "auth_assert"),
			authAssertNode(),
			endNode(),
		},
	})
	ts.service.answerInTurn(pinNode, redirectAnswer,
		okAnswer(`{"actionStatus":"SUCCESS","verifiedUserId":"`+ts.fixtures.userID+`"}`))

	result, raw := ts.returnFromPinPage(ts.startAtPinPage())

	ts.assertSignedInAs(result, raw, ts.fixtures.userID)
	calls := ts.service.callsFrom(pinNode)
	ts.Require().Len(calls, 2)
	ts.Equal(map[string]string{codeInput: pinReturnCode}, calls[1].Body.UserInputs,
		"The returned code is always sent, so the service can check it issued it to this browser")
}

func (ts *CustomAuthenticationTestSuite) TestRefusalAfterTheRedirectSignsInNoOne() {
	ts.service.answerInTurn(pinNode, redirectAnswer,
		okAnswer(`{"actionStatus":"FAILED","failureReason":"wrong_pin","failureDescription":"Wrong PIN."}`))

	result, raw := ts.returnFromPinPage(ts.startAtPinPage())

	ts.Require().Equal("INCOMPLETE", result.FlowStatus, "FAILED should follow onFailure to the prompt: %s", raw)
	ts.True(common.ValidateRequiredInputs(result.Data.Inputs, []string{codeInput}), "onFailure leads to the prompt")
	ts.Empty(result.Assertion, "A refused sign-in must not return an assertion")
	ts.Require().NotNil(result.Error, "The prompt should carry the refusal: %s", raw)
	ts.Equal(errCodeExtensionActionFailed, result.Error.Code)
}

func (ts *CustomAuthenticationTestSuite) TestResolverFailuresSignInNoOne() {
	cases := []struct {
		name       string
		verifiedID string
		code       string
	}{
		{name: "UnknownUser", verifiedID: "00000000-0000-7000-8000-00000000dead", code: errCodeEntityNotFound},
		{name: "NoIdentifier", verifiedID: "", code: errCodeUserNotAuthenticated},
	}
	for _, tc := range cases {
		ts.Run(tc.name, func() {
			ts.SetupTest()
			ts.service.answerInTurn(pinNode, redirectAnswer,
				okAnswer(`{"actionStatus":"SUCCESS","verifiedUserId":"`+tc.verifiedID+`"}`))

			result, raw := ts.returnFromPinPage(ts.startAtPinPage())

			ts.assertEndedWith(result, raw, tc.code)
		})
	}
}

// refusedFlow builds a flow from nodes and checks flow save refuses it as a misconfigured step.
func (ts *CustomAuthenticationTestSuite) assertSaveRefused(handle string, nodes []map[string]interface{},
	want ...string) {
	ts.T().Helper()
	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name: "Extension Custom Authentication Refused Flow " + handle, FlowType: "AUTHENTICATION",
		Handle: handle, Nodes: nodes,
	})
	if flowID != "" {
		ts.T().Cleanup(func() { _ = testutils.DeleteFlow(flowID) })
	}
	ts.Require().Error(err, "The flow must not be saved")
	for _, text := range append([]string{errCodeInvalidExecutorConfig}, want...) {
		ts.Contains(err.Error(), text)
	}
}

func (ts *CustomAuthenticationTestSuite) TestResolverNotBoundToItsExtensionIsRefusedAtSave() {
	ts.Run("SourceKeyOutsideTheNamespace", func() {
		ts.assertSaveRefused("ext_custom_auth_bare_source_flow", []map[string]interface{}{
			startNode(pinNode),
			ts.pinStep(nil, map[string]string{"onSuccess": "resolve_user"}),
			resolverNode(map[string]interface{}{"sourceKey": "username"}, "auth_assert"),
			authAssertNode(),
			endNode(),
		}, "sourceKey")
	})
	ts.Run("AnotherStepsNamespace", func() {
		ts.assertSaveRefused("ext_custom_auth_other_source_flow", []map[string]interface{}{
			startNode(pinNode),
			ts.pinStep(nil, map[string]string{"onSuccess": "resolve_user"}),
			resolverNode(map[string]interface{}{"sourceKey": extensionKey("risk_check", "username")}, "auth_assert"),
			authAssertNode(),
			endNode(),
		}, "sourceKey")
	})
	ts.Run("ReachedFromAPromptToo", func() {
		ts.assertSaveRefused("ext_custom_auth_prompt_route_flow", []map[string]interface{}{
			startNode(pinNode),
			ts.pinStep(nil, map[string]string{"onSuccess": "resolve_user", "onFailure": "prompt_code"}),
			codePromptNode("prompt_code", "resolve_user"),
			resolverNode(map[string]interface{}{"sourceKey": extensionKey(pinNode, "username")}, "auth_assert"),
			authAssertNode(),
			endNode(),
		}, "sourceKey")
	})
}

func (ts *CustomAuthenticationTestSuite) TestMatchAttributeUsersCanChangeIsRefusedAtSave() {
	ts.assertSaveRefused("ext_custom_auth_email_match_flow", []map[string]interface{}{
		startNode(pinNode),
		ts.pinStep(nil, map[string]string{"onSuccess": "resolve_user"}),
		resolverNode(map[string]interface{}{
			"sourceKey": extensionKey(pinNode, "email"), "matchAttribute": "email",
		}, "auth_assert"),
		authAssertNode(),
		endNode(),
	}, "matchAttribute")
}

func (ts *CustomAuthenticationTestSuite) TestInternalCredentialInputIsRefusedAtSave() {
	inputs := []map[string]interface{}{
		{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_verified", "identifier": "extensionVerifiedEntityID", "type": "PASSWORD_INPUT",
			"required": true},
	}
	ts.assertSaveRefused("ext_custom_auth_internal_credential_flow", []map[string]interface{}{
		startNode("prompt_verified"),
		{
			"id":   "prompt_verified",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{{
				"inputs": inputs,
				"action": map[string]interface{}{"ref": "action_verified", "nextNode": "credentials_auth"},
			}},
		},
		{
			"id":           "credentials_auth",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": inputs},
			"onSuccess":    "auth_assert",
			"onIncomplete": "prompt_verified",
		},
		authAssertNode(),
		endNode(),
	}, "extensionVerifiedEntityID")
}

func (ts *CustomAuthenticationTestSuite) TestExtensionWithoutResolverAuthenticatesNoOne() {
	ts.useFlow(testutils.Flow{
		Name:     "Extension Custom Authentication No Resolver Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "ext_custom_auth_no_resolver_flow",
		Nodes: []map[string]interface{}{
			startNode(pinNode),
			ts.pinStep(nil, map[string]string{"onSuccess": "authorize"}),
			authorizationNode(),
			authAssertNode(),
			endNode(),
		},
	})
	ts.service.answerInTurn(pinNode, redirectAnswer,
		okAnswer(`{"actionStatus":"SUCCESS","userID":"`+ts.fixtures.userID+`"}`))

	result, raw := ts.returnFromPinPage(ts.startAtPinPage())

	ts.assertEndedWith(result, raw, errCodeUserNotAuthenticated)
}

// newUserFlow is pattern 2, for users who do not exist in ThunderID yet:
//
//	start -> ext_pin -> resolve_user (username, allowAuthenticationWithoutLocalUser) -> provision
//	      -> authorize -> auth_assert -> end
//
// The resolver maps username, email and given_name; family_name is stored but not mapped.
func (ts *CustomAuthenticationTestSuite) newUserFlow(handle string, matchAttribute string) testutils.Flow {
	return testutils.Flow{
		Name:     "Extension Custom Authentication New User Flow " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			startNode(pinNode),
			ts.pinStep(map[string]interface{}{
				"responseKeys": []string{"username", "email", "given_name", "family_name"},
			}, map[string]string{"onSuccess": "resolve_user"}),
			resolverNode(map[string]interface{}{
				"sourceKey":                           extensionKey(pinNode, "username"),
				"matchAttribute":                      matchAttribute,
				"allowAuthenticationWithoutLocalUser": true,
				"attributeMapping": map[string]interface{}{
					"username":   extensionKey(pinNode, "username"),
					"email":      extensionKey(pinNode, "email"),
					"given_name": extensionKey(pinNode, "given_name"),
				},
			}, "provision"),
			{
				"id":        "provision",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "authorize",
			},
			authorizationNode(),
			authAssertNode(),
			endNode(),
		},
	}
}

// pinAnswerFor is the PIN service's SUCCESS answer for username, with its profile.
func pinAnswerFor(username string) serviceAnswer {
	return okAnswer(`{"actionStatus":"SUCCESS","serviceUserId":"svc-42","username":"` + username + `",` +
		`"email":"` + username + `@example.com","given_name":"Peter","family_name":"Parker"}`)
}

func (ts *CustomAuthenticationTestSuite) TestFirstSignInProvisionsTheUserAndTheNextFindsThem() {
	ts.useFlow(ts.newUserFlow("ext_custom_auth_new_user_flow", "username"))
	ts.T().Cleanup(func() {
		if user, err := testutils.FindUserByAttribute("username", provisionedUsername); err == nil && user != nil {
			if err := testutils.CleanupUsers([]string{user.ID}); err != nil {
				ts.T().Logf("Failed to delete the provisioned user: %v", err)
			}
		}
	})

	ts.service.answerInTurn(pinNode, redirectAnswer, pinAnswerFor(provisionedUsername))
	result, raw := ts.returnFromPinPage(ts.startAtPinPage())

	ts.Require().Equal("COMPLETE", result.FlowStatus, "One sign-in should create and sign in the user: %s", raw)
	user, err := testutils.FindUserByAttribute("username", provisionedUsername)
	ts.Require().NoError(err)
	ts.Require().NotNil(user, "The first sign-in should create the user")
	ts.assertSignedInAs(result, raw, user.ID)
	attrs, err := testutils.GetUserAttributes(*user)
	ts.Require().NoError(err)
	ts.Equal(provisionedUsername+"@example.com", attrs["email"], "Mapped attributes are taken")
	ts.Equal("Peter", attrs["given_name"], "Mapped attributes are taken")
	ts.NotContains(attrs, "family_name", "An attribute the resolver does not map is never taken")
	ts.NotContains(attrs, "serviceUserId", "A field the step does not store is never taken")

	ts.service.reset()
	ts.service.answerInTurn(pinNode, redirectAnswer, pinAnswerFor(provisionedUsername))
	again, againRaw := ts.returnFromPinPage(ts.startAtPinPage())

	ts.assertSignedInAs(again, againRaw, user.ID)
	sameUser, err := testutils.FindUserByAttribute("username", provisionedUsername)
	ts.Require().NoError(err)
	ts.Require().NotNil(sameUser)
	ts.Equal(user.ID, sameUser.ID, "The second sign-in finds the user and provisions no one")
}

func (ts *CustomAuthenticationTestSuite) TestExistingUserIsSignedInWithoutProvisioning() {
	ts.useFlow(ts.newUserFlow("ext_custom_auth_existing_user_flow", "username"))

	ts.service.answerInTurn(pinNode, redirectAnswer, pinAnswerFor(ts.fixtures.username))
	result, raw := ts.returnFromPinPage(ts.startAtPinPage())

	ts.assertSignedInAs(result, raw, ts.fixtures.userID)
	attrs := ts.userAttributes(ts.fixtures.username)
	ts.Equal("extcustomauthuser@example.com", attrs["email"], "The existing user's attributes are unchanged")
	ts.NotContains(attrs, "given_name", "The existing user's attributes are unchanged")
}

func (ts *CustomAuthenticationTestSuite) TestLookupByUserIDNeverProvisions() {
	ts.useFlow(ts.newUserFlow("ext_custom_auth_user_id_flow", "userID"))

	ts.service.answerInTurn(pinNode, redirectAnswer, pinAnswerFor(provisionedUsername))
	result, raw := ts.returnFromPinPage(ts.startAtPinPage())

	ts.assertEndedWith(result, raw, errCodeEntityNotFound)
	user, err := testutils.FindUserByAttribute("username", provisionedUsername)
	ts.Require().NoError(err)
	ts.Nil(user, "A lookup by userID must not mark anyone for provisioning")
}

// userAttributes reads the attributes of the user with username.
func (ts *CustomAuthenticationTestSuite) userAttributes(username string) map[string]interface{} {
	ts.T().Helper()
	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err, "Failed to read user %s", username)
	ts.Require().NotNil(user, "User %s should exist", username)
	attrs, err := testutils.GetUserAttributes(*user)
	ts.Require().NoError(err)
	return attrs
}

func (ts *CustomAuthenticationTestSuite) TestSecondFactorKeepsTheSignedInUser() {
	ts.useFlow(testutils.Flow{
		Name:     "Extension Custom Authentication Second Factor Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "ext_custom_auth_second_factor_flow",
		Nodes: []map[string]interface{}{
			startNode("prompt_credentials"),
			promptCredentialsNode("credentials_auth"),
			credentialsAuthNode(pinNode),
			ts.pinStep(nil, map[string]string{"onSuccess": "authorize"}),
			authorizationNode(),
			authAssertNode(),
			endNode(),
		},
	})
	ts.service.answerInTurn(pinNode, redirectAnswer,
		okAnswer(`{"actionStatus":"SUCCESS","userID":"someone-else","verifiedUserId":"someone-else"}`))

	pinPage, raw := ts.fixtures.submitCredentials(&ts.Suite, ts.fixtures.startLogin(&ts.Suite))
	ts.Require().Equal("REDIRECTION", pinPage.Type, "The second factor should send the browser out: %s", raw)
	result, raw := ts.returnFromPinPage(pinPage)

	ts.assertSignedInAs(result, raw, ts.fixtures.userID)
	calls := ts.service.callsFrom(pinNode)
	ts.Require().Len(calls, 2)
	for _, call := range calls {
		ts.Equal(map[string]string{"userID": ts.fixtures.userID}, call.Body.RuntimeData,
			"The service is told who signed in")
		ts.Contains(call.RawBody, `"entityId":"`+ts.fixtures.userID+`"`, "authUser names who signed in")
	}
}

func (ts *CustomAuthenticationTestSuite) TestResumeWithoutTheReturnedCodeIsRefused() {
	ts.service.answerInTurn(pinNode, redirectAnswer,
		okAnswer(`{"actionStatus":"SUCCESS","verifiedUserId":"`+ts.fixtures.userID+`"}`))
	step := ts.startAtPinPage()

	// Another client that copied the execution ID and the challenge token, but not the code only the
	// browser that completed the service's page holds.
	result, raw := submit(&ts.Suite, step, nil, "")

	ts.Empty(result.Assertion, "The flow must not resume without the code: %s", raw)
	ts.Require().NotNil(result.Error, "The refusal should be reported: %s", raw)
	ts.Equal(errCodeExtensionReturnRefused, result.Error.Code)
	ts.Len(ts.service.callsFrom(pinNode), 1, "The service is not asked for the outcome")
}

func (ts *CustomAuthenticationTestSuite) TestOutcomeIsBoundToTheReturnedCode() {
	// The PIN service records its outcome against the code it handed the browser that completed its
	// page, and refuses any other code.
	ts.service.answerInTurn(pinNode, redirectAnswer,
		okAnswer(`{"actionStatus":"FAILED","failureReason":"unknown_code"}`))
	step := ts.startAtPinPage()

	result, raw := submit(&ts.Suite, step, map[string]string{codeInput: "guessed-code"}, "")

	ts.Empty(result.Assertion, "Another person's outcome must not sign anyone in: %s", raw)
	calls := ts.service.callsFrom(pinNode)
	ts.Require().Len(calls, 2)
	ts.Equal("guessed-code", calls[1].Body.UserInputs[codeInput],
		"The service receives the code to check it against the one it issued")
}

// pinAnswerForID is the PIN service's SUCCESS answer for the ThunderID user ID it verified.
func pinAnswerForID(userID string) serviceAnswer {
	return okAnswer(`{"actionStatus":"SUCCESS","verifiedUserId":"` + userID + `"}`)
}

func (ts *CustomAuthenticationTestSuite) TestResolverRefusesUsersItMustNotSignIn() {
	partnerTypeID, err := testutils.CreateUserType(testutils.UserType{
		Handle: customAuthPartnerUserTypeName, DisplayName: "Extension Custom Authentication Partner",
		OUID: ts.fixtures.ouID,
		Schema: map[string]interface{}{
			"username": map[string]interface{}{"type": "string"},
			"password": map[string]interface{}{"type": "string", "credential": true},
		},
	})
	ts.Require().NoError(err, "Failed to create the partner user type")
	ts.T().Cleanup(func() { _ = testutils.DeleteUserType(partnerTypeID) })
	partnerID, err := testutils.CreateUser(testutils.User{
		Type: customAuthPartnerUserTypeName, OUID: ts.fixtures.ouID,
		Attributes: json.RawMessage(`{"username": "extcustomauthuser", "password": "ExtPartnerPass123!"}`),
	})
	ts.Require().NoError(err, "Failed to create the partner user")
	ts.T().Cleanup(func() { _ = testutils.CleanupUsers([]string{partnerID}) })

	ts.Run("UserTypeTheApplicationDoesNotAllow", func() {
		ts.SetupTest()
		ts.service.answerInTurn(pinNode, redirectAnswer, pinAnswerForID(partnerID))

		result, raw := ts.returnFromPinPage(ts.startAtPinPage())

		ts.assertEndedWith(result, raw, errCodeEntityNotFound)
	})

	ts.Run("MoreThanOneMatch", func() {
		ts.SetupTest()
		ts.useFlow(ts.newUserFlow("ext_custom_auth_ambiguous_flow", "username"))
		ts.service.answerInTurn(pinNode, redirectAnswer, pinAnswerFor(ts.fixtures.username))

		result, raw := ts.returnFromPinPage(ts.startAtPinPage())

		ts.assertEndedWith(result, raw, errCodeAmbiguousEntity)
		user, err := testutils.FindUserByAttribute("given_name", "Peter")
		ts.Require().NoError(err)
		ts.Nil(user, "An ambiguous match must not provision anyone")
	})
}

func (ts *CustomAuthenticationTestSuite) TestResolverRefusesANonUserEntity() {
	snapshot, err := testutils.SnapshotAgentType()
	ts.Require().NoError(err, "Failed to snapshot the default agent type")
	ts.T().Cleanup(func() { _ = testutils.RestoreAgentType(snapshot) })
	_, err = testutils.CreateAgentType(testutils.UserType{
		DisplayName: "Default", OUID: ts.fixtures.ouID,
		Schema: map[string]interface{}{
			"name": map[string]interface{}{"type": "string", "required": true, "unique": true},
		},
	})
	ts.Require().NoError(err, "Failed to create the agent type")
	agentID, err := testutils.CreateAgent(testutils.Agent{
		OUID: ts.fixtures.ouID, Type: "default", Owner: ts.fixtures.userID,
		Attributes: map[string]interface{}{"name": "ext-custom-auth-agent"},
	})
	ts.Require().NoError(err, "Failed to create the agent")
	ts.T().Cleanup(func() { _ = testutils.DeleteAgent(agentID) })
	ts.service.answerInTurn(pinNode, redirectAnswer, pinAnswerForID(agentID))

	result, raw := ts.returnFromPinPage(ts.startAtPinPage())

	ts.assertEndedWith(result, raw, errCodeEntityNotFound)
}

// runAdministrationFlow runs the shipped administration flow handle against subject as the administrator.
func (ts *CustomAuthenticationTestSuite) runAdministrationFlow(handle, subject string) {
	ts.T().Helper()
	token, err := testutils.GetAccessToken()
	ts.Require().NoError(err, "The administrator must hold a token")
	flowID, err := testutils.GetFlowIDByHandle(handle, "ADMINISTRATION")
	ts.Require().NoError(err, "The %s flow must ship", handle)
	status, raw, err := testutils.ExecuteFlowRaw(nil, token,
		map[string]interface{}{"flowId": flowID, "inputs": map[string]string{"subject": subject}})
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, status, "%s failed: %s", handle, string(raw))
	ts.Require().Contains(string(raw), `"COMPLETE"`, "%s did not complete: %s", handle, string(raw))
}

func (ts *CustomAuthenticationTestSuite) TestResolverRefusesASuspendedUser() {
	userID, err := testutils.CreateUser(testutils.User{
		Type: customAuthUser.Type, OUID: ts.fixtures.ouID,
		Attributes: json.RawMessage(`{"username": "extcustomauthsuspended", "password": "ExtSuspended123!"}`),
	})
	ts.Require().NoError(err, "Failed to create the user to suspend")
	ts.T().Cleanup(func() { _ = testutils.CleanupUsers([]string{userID}) })
	ts.runAdministrationFlow("default-user-suspend-flow", userID)
	ts.service.answerInTurn(pinNode, redirectAnswer, pinAnswerForID(userID))

	result, raw := ts.returnFromPinPage(ts.startAtPinPage())

	ts.assertEndedWith(result, raw, errCodeEntityNotFound)
}

func (ts *CustomAuthenticationTestSuite) TestAuthorizationEvaluatesTheResolvedUser() {
	resourceServerID, err := testutils.CreateResourceServerWithActions(testutils.ResourceServer{
		Name: "Extension Custom Authentication Documents", Identifier: customAuthResourceServerHandle,
		OUID: ts.fixtures.ouID,
	}, []testutils.Action{
		{Name: "Read Documents", Handle: "read"},
		{Name: "Write Documents", Handle: "write"},
	})
	ts.Require().NoError(err, "Failed to create the resource server")
	ts.T().Cleanup(func() { _ = testutils.DeleteResourceServerWithChildren(resourceServerID) })
	roleID, err := testutils.CreateRole(testutils.Role{
		Name: "ExtCustomAuthReader", OUID: ts.fixtures.ouID,
		Permissions: []testutils.ResourcePermissions{{ResourceServerID: resourceServerID, Permissions: []string{"read"}}},
		Assignments: []testutils.Assignment{{ID: ts.fixtures.userID, Type: "user"}},
	})
	ts.Require().NoError(err, "Failed to create the role")
	ts.T().Cleanup(func() { _ = testutils.DeleteRole(roleID) })
	ts.service.answerInTurn(pinNode, redirectAnswer, pinAnswerForID(ts.fixtures.userID))

	step, err := common.InitiateAuthenticationFlow(ts.fixtures.appID, false, map[string]string{
		"requested_permissions":      "read write",
		"resource_server_identifier": customAuthResourceServerHandle,
	}, "")
	ts.Require().NoError(err)
	ts.assertRedirectedToPinPage(step)
	result, raw := ts.returnFromPinPage(step)

	ts.assertSignedInAs(result, raw, ts.fixtures.userID)
	claims, err := testutils.DecodeJWTPayloadMap(result.Assertion)
	ts.Require().NoError(err)
	ts.Equal("read", claims["authorized_permissions"], "Only the resolved user's own permissions are granted")
}

func (ts *CustomAuthenticationTestSuite) TestResolverSignInClaimsNoAuthenticationItDidNotRecord() {
	ts.service.answerInTurn(pinNode, redirectAnswer, pinAnswerForID(ts.fixtures.userID))

	result, raw := ts.returnFromPinPage(ts.startAtPinPage())

	ts.assertSignedInAs(result, raw, ts.fixtures.userID)
	claims, err := testutils.DecodeJWTPayloadMap(result.Assertion)
	ts.Require().NoError(err)
	assurance, _ := claims["assurance"].(map[string]interface{})
	for _, level := range []string{"AAL1", "AAL2", "AAL3"} {
		ts.NotEqual(level, assurance["aal"], "The resolver records no authenticator: %v", claims)
	}
	ts.NotContains(claims, "amr", "The resolver records no authentication method")
}

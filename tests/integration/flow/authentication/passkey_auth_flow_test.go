// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	passkeyFlowRelyingPartyID   = "localhost"
	passkeyFlowRelyingPartyName = "ThunderID Test"
	// passkeyFlowOrigin is set as the application's only allowed passkey origin, which is what the
	// flow executor validates the assertion against. It must also be listed under
	// passkey.allowed_origins in the test deployment.yaml, since SetupSuite enrols the credential
	// through the direct passkey API, and that path takes its allowed origins from server config.
	passkeyFlowOrigin = "https://localhost:8095"
)

// passkeyChallengeNode builds the challenge node, optionally omitting the relying party ID so the
// misconfiguration case can be exercised.
func passkeyChallengeNode(includeRelyingPartyID bool, nextNode string) map[string]interface{} {
	node := map[string]interface{}{
		"id":   "passkey_challenge",
		"type": "TASK_EXECUTION",
		"executor": map[string]interface{}{
			"name": "PasskeyAuthExecutor",
			"mode": "challenge",
		},
		"onSuccess": nextNode,
	}
	if includeRelyingPartyID {
		node["properties"] = map[string]interface{}{
			"relyingPartyId":   passkeyFlowRelyingPartyID,
			"relyingPartyName": passkeyFlowRelyingPartyName,
		}
	}
	return node
}

// passkeyAssertionPromptNode builds the prompt that collects the assertion. When requireSignature is
// false the signature input is dropped, so the executor sees an incomplete submission.
func passkeyAssertionPromptNode(requireSignature bool) map[string]interface{} {
	inputs := []map[string]interface{}{
		{"ref": "input_credential_id", "identifier": "credentialId", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_client_data", "identifier": "clientDataJSON", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_auth_data", "identifier": "authenticatorData", "type": "TEXT_INPUT", "required": true},
	}
	if requireSignature {
		inputs = append(inputs, map[string]interface{}{
			"ref": "input_signature", "identifier": "signature", "type": "TEXT_INPUT", "required": true,
		})
	}
	inputs = append(inputs, map[string]interface{}{
		"ref": "input_user_handle", "identifier": "userHandle", "type": "TEXT_INPUT", "required": false,
	})

	return map[string]interface{}{
		"id":   "prompt_assertion",
		"type": "PROMPT",
		"prompts": []map[string]interface{}{
			{
				"inputs": inputs,
				"action": map[string]interface{}{
					"ref":      "action_assertion",
					"nextNode": "passkey_verify",
				},
			},
		},
	}
}

// passkeyTailNodes are the nodes shared by every variant once the assertion has been collected.
func passkeyTailNodes() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":   "passkey_verify",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "PasskeyAuthExecutor",
				"mode": "verify",
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
	}
}

// usernamePromptNodes prompt for a username and resolve it to a user before the challenge is
// generated, which is what makes the ceremony username based rather than usernameless.
func usernamePromptNodes() []map[string]interface{} {
	return []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_username"},
		{
			"id":   "prompt_username",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					},
					"action": map[string]interface{}{"ref": "action_username", "nextNode": "identify_user"},
				},
			},
		},
		{
			"id":   "identify_user",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "IdentifyingExecutor",
				"mode": "identify",
				"inputs": []map[string]interface{}{
					{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
				},
			},
			"onSuccess": "passkey_challenge",
		},
	}
}

// buildUsernameFlow assembles a username based passkey authentication flow.
func buildUsernameFlow(name, handle string, includeRelyingPartyID, requireSignature bool) testutils.Flow {
	nodes := usernamePromptNodes()
	nodes = append(nodes, passkeyChallengeNode(includeRelyingPartyID, "prompt_assertion"))
	nodes = append(nodes, passkeyAssertionPromptNode(requireSignature))
	nodes = append(nodes, passkeyTailNodes()...)

	return testutils.Flow{Name: name, FlowType: "AUTHENTICATION", Handle: handle, Nodes: nodes}
}

// buildUsernamelessFlow assembles a flow that issues a challenge with no user in context, so the
// user is resolved from the credential's user handle at verification time.
func buildUsernamelessFlow(name, handle string) testutils.Flow {
	nodes := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "passkey_challenge"},
		passkeyChallengeNode(true, "prompt_assertion"),
		passkeyAssertionPromptNode(true),
	}
	nodes = append(nodes, passkeyTailNodes()...)

	return testutils.Flow{Name: name, FlowType: "AUTHENTICATION", Handle: handle, Nodes: nodes}
}

var (
	passkeyFlowTestOU = testutils.OrganizationUnit{
		Handle:      "passkey-auth-flow-test-ou",
		Name:        "Passkey Auth Flow Test OU",
		Description: "Organization unit for passkey authentication flow tests",
	}

	// The password is only here to bootstrap: enrolling a passkey through the direct API requires an
	// assertion proving the target user, so the user needs another credential to authenticate with
	// first. The flows under test never use it.
	passkeyFlowEntityType = testutils.UserType{
		Name: "passkey_flow_user",
		Schema: map[string]interface{}{
			"username":    map[string]interface{}{"type": "string"},
			"email":       map[string]interface{}{"type": "string"},
			"displayName": map[string]interface{}{"type": "string"},
			"password":    map[string]interface{}{"type": "string", "credential": true},
		},
	}

	passkeyFlowTestUser = testutils.User{
		Type: "passkey_flow_user",
		Attributes: json.RawMessage(`{
			"username": "passkeyflowuser",
			"email": "passkeyflowuser@example.com",
			"displayName": "Passkey Flow User",
			"password": "PasskeyFlowPassword123!"
		}`),
	}

	passkeyFlowTestApp = testutils.Application{
		Name:                      "Passkey Auth Flow Test Application",
		Description:               "Application for testing passkey authentication flows",
		IsRegistrationFlowEnabled: false,
		ClientID:                  "passkey_auth_flow_test_client",
		ClientSecret:              "passkey_auth_flow_test_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{"passkey_flow_user"},
		// The flow executor takes its allowed origins from the application, not from server config.
		PasskeyAllowedOrigins: []string{passkeyFlowOrigin},
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	}
)

type PasskeyAuthFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	appID              string
	entityTypeID       string
	userID             string
	authenticator      *testutils.VirtualAuthenticator
	webAuthnUserHandle string

	baselineFlowID     string
	usernamelessFlowID string
	missingSignatureID string
}

func TestPasskeyAuthFlowTestSuite(t *testing.T) {
	suite.Run(t, new(PasskeyAuthFlowTestSuite))
}

func (ts *PasskeyAuthFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(passkeyFlowTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	passkeyFlowTestOU.ID = ouID

	passkeyFlowEntityType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(passkeyFlowEntityType)
	ts.Require().NoError(err, "Failed to create test user type")
	ts.entityTypeID = entityTypeID

	user := passkeyFlowTestUser
	user.OUID = ouID
	userIDs, err := testutils.CreateMultipleUsers(user)
	ts.Require().NoError(err, "Failed to create test user")
	ts.config.CreatedUserIDs = userIDs
	ts.userID = userIDs[0]

	// Register a credential through the direct API. Flow tests exercise authentication, and
	// registration through a flow is covered by the registration suite. The direct API enrolls only
	// for the subject of a supplied assertion, so authenticate with the bootstrap password first.
	assertion, err := testutils.ObtainAuthAssertion("passkeyflowuser", "PasskeyFlowPassword123!")
	ts.Require().NoError(err, "Failed to obtain an auth assertion for the test user")

	authenticator, userHandle, err := testutils.RegisterPasskeyCredential(
		ts.userID, passkeyFlowRelyingPartyID, passkeyFlowRelyingPartyName, passkeyFlowOrigin, assertion)
	ts.Require().NoError(err, "Failed to register a passkey credential for the test user")
	ts.authenticator = authenticator
	ts.webAuthnUserHandle = userHandle

	ts.baselineFlowID = ts.createFlow(buildUsernameFlow(
		"Passkey Auth Flow Test", "auth_flow_passkey_test", true, true))
	ts.usernamelessFlowID = ts.createFlow(buildUsernamelessFlow(
		"Passkey Usernameless Auth Flow Test", "auth_flow_passkey_usernameless_test"))
	ts.missingSignatureID = ts.createFlow(buildUsernameFlow(
		"Passkey Auth Flow Missing Signature Test", "auth_flow_passkey_no_signature_test", true, false))

	passkeyFlowTestApp.OUID = ouID
	passkeyFlowTestApp.AuthFlowID = ts.baselineFlowID
	appID, err := testutils.CreateApplication(passkeyFlowTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID
}

func (ts *PasskeyAuthFlowTestSuite) createFlow(flow testutils.Flow) string {
	flowID, err := testutils.CreateFlow(flow)
	ts.Require().NoError(err, "Failed to create flow %s", flow.Handle)
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	return flowID
}

func (ts *PasskeyAuthFlowTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to cleanup users during teardown: %v", err)
	}

	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete application during teardown: %v", err)
		}
	}

	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s during teardown: %v", flowID, err)
		}
	}

	if ts.entityTypeID != "" {
		if err := testutils.DeleteUserType(ts.entityTypeID); err != nil {
			ts.T().Logf("Failed to delete user type during teardown: %v", err)
		}
	}

	if passkeyFlowTestOU.ID != "" {
		if err := testutils.DeleteOrganizationUnit(passkeyFlowTestOU.ID); err != nil {
			ts.T().Logf("Failed to delete organization unit during teardown: %v", err)
		}
	}
}

// useFlow points the test application at the given flow, so each test can drive its own variant.
func (ts *PasskeyAuthFlowTestSuite) useFlow(flowID string) {
	ts.Require().NoError(common.UpdateAppConfig(ts.appID, flowID, ""),
		"Failed to point the application at flow %s", flowID)
}

// challengeFromStep decodes the credential request options the challenge node returns, which arrive
// as a JSON string in the step's additional data.
func (ts *PasskeyAuthFlowTestSuite) challengeFromStep(step *common.FlowStep) string {
	raw, ok := step.Data.AdditionalData["passkeyChallenge"]
	ts.Require().True(ok, "Flow step should carry a passkey challenge")

	var options struct {
		Challenge string `json:"challenge"`
	}
	ts.Require().NoError(json.Unmarshal([]byte(raw), &options),
		"Failed to decode passkey challenge options")
	ts.Require().NotEmpty(options.Challenge, "Challenge should not be empty")

	return options.Challenge
}

// assertionInputs builds the flow inputs for an assertion over the given challenge.
func (ts *PasskeyAuthFlowTestSuite) assertionInputs(challenge string) map[string]string {
	credentialID, clientDataJSON, authenticatorData, signature, err :=
		ts.authenticator.CreateAssertionResponse(challenge, true)
	ts.Require().NoError(err, "Failed to build assertion response")

	return map[string]string{
		"credentialId":      credentialID,
		"clientDataJSON":    clientDataJSON,
		"authenticatorData": authenticatorData,
		"signature":         signature,
		"userHandle":        ts.webAuthnUserHandle,
	}
}

// startUsernameFlow drives a username based flow up to the point where the assertion is requested.
func (ts *PasskeyAuthFlowTestSuite) startUsernameFlow() *common.FlowStep {
	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().True(common.HasInput(step.Data.Inputs, "username"), "Username input should be required")

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"username": "passkeyflowuser"}, "action_username", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit username")

	return step
}

// TestPasskeyAuthFlow_Success drives a full username based passkey ceremony through the flow API.
func (ts *PasskeyAuthFlowTestSuite) TestPasskeyAuthFlow_Success() {
	ts.useFlow(ts.baselineFlowID)

	step := ts.startUsernameFlow()
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Expected flow status to be INCOMPLETE")
	ts.Require().True(common.HasInput(step.Data.Inputs, "signature"),
		"Assertion inputs should be requested after the challenge")

	challenge := ts.challengeFromStep(step)

	finalStep, err := common.CompleteFlow(step.ExecutionID, ts.assertionInputs(challenge),
		"action_assertion", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the passkey assertion")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().Nil(finalStep.Error, "Error should be nil for a successful authentication")
	ts.Require().NotEmpty(finalStep.Assertion, "A JWT assertion should be returned")

	claims, err := testutils.ValidateJWTAssertionFields(finalStep.Assertion, ts.appID,
		passkeyFlowEntityType.Name, passkeyFlowTestOU.ID, passkeyFlowTestOU.Name, passkeyFlowTestOU.Handle)
	ts.Require().NoError(err, "Failed to validate JWT assertion fields")
	ts.Require().NotNil(claims, "JWT claims should not be nil")
}

// TestPasskeyAuthFlow_Usernameless issues a challenge with no user in context, so the user is
// resolved from the credential itself.
func (ts *PasskeyAuthFlowTestSuite) TestPasskeyAuthFlow_Usernameless() {
	ts.useFlow(ts.usernamelessFlowID)

	step, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate usernameless authentication flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Expected flow status to be INCOMPLETE")

	challenge := ts.challengeFromStep(step)

	finalStep, err := common.CompleteFlow(step.ExecutionID, ts.assertionInputs(challenge),
		"action_assertion", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the passkey assertion")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus, "Expected flow status to be COMPLETE")
	ts.Require().NotEmpty(finalStep.Assertion, "A JWT assertion should be returned")
}

// TestPasskeyAuthFlow_MissingRelyingPartyIdRejectedAtCreation covers a misconfigured challenge
// node. The executor carries a runtime guard for a missing relying party ID, but that guard is
// unreachable through the management API: relyingPartyId is declared required for the challenge
// mode, so the flow is rejected when it is created rather than when it runs.
func (ts *PasskeyAuthFlowTestSuite) TestPasskeyAuthFlow_MissingRelyingPartyIdRejectedAtCreation() {
	_, err := testutils.CreateFlow(buildUsernameFlow(
		"Passkey Auth Flow Without RP ID Test", "auth_flow_passkey_no_rp_test", false, true))
	ts.Require().Error(err, "A challenge node without a relying party ID should be rejected")
	ts.Contains(err.Error(), "relyingPartyId",
		"The rejection should name the missing executor property")
}

// TestPasskeyAuthFlow_InvalidSignature rejects an assertion whose signature does not verify.
func (ts *PasskeyAuthFlowTestSuite) TestPasskeyAuthFlow_InvalidSignature() {
	ts.useFlow(ts.baselineFlowID)

	step := ts.startUsernameFlow()
	challenge := ts.challengeFromStep(step)

	inputs := ts.assertionInputs(challenge)
	// Corrupt the signature while keeping it valid base64, so the request reaches signature
	// verification rather than failing to parse.
	inputs["signature"] = "AAAA" + inputs["signature"][4:]

	finalStep, err := common.CompleteFlow(step.ExecutionID, inputs, "action_assertion",
		step.ChallengeToken)
	if err == nil {
		ts.Require().NotEqual("COMPLETE", finalStep.FlowStatus,
			"A tampered signature must not complete the flow")
	}
}

// TestPasskeyAuthFlow_MissingRequiredInputs submits an assertion with no signature, which the
// executor should treat as an incomplete submission rather than a verification failure.
func (ts *PasskeyAuthFlowTestSuite) TestPasskeyAuthFlow_MissingRequiredInputs() {
	ts.useFlow(ts.missingSignatureID)

	step := ts.startUsernameFlow()
	challenge := ts.challengeFromStep(step)

	inputs := ts.assertionInputs(challenge)
	delete(inputs, "signature")

	finalStep, err := common.CompleteFlow(step.ExecutionID, inputs, "action_assertion",
		step.ChallengeToken)
	if err == nil {
		ts.Require().NotEqual("COMPLETE", finalStep.FlowStatus,
			"An assertion without a signature must not complete the flow")
		ts.Require().True(common.HasInput(finalStep.Data.Inputs, "signature"),
			"The flow should ask for the missing signature input again")
	}
}

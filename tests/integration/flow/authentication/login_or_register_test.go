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

// loginOrRegisterSchema declares no unique attributes so two users may share an email, which is what
// the ambiguous branch requires.
var loginOrRegisterSchema = map[string]interface{}{
	"username": map[string]interface{}{"type": "string"},
	"email":    map[string]interface{}{"type": "string"},
	"password": map[string]interface{}{"type": "string", "credential": true},
}

type LoginOrRegisterTestSuite struct {
	suite.Suite
	parentOUID     string
	ouAID          string
	ouBID          string
	ouAHandle      string
	ouBHandle      string
	typeAID        string
	typeBID        string
	typeAName      string
	typeBName      string
	soloEmail      string
	soloPassword   string
	sharedEmail    string
	appID          string
	flowID         string
	createdUserIDs []string
}

func TestLoginOrRegisterTestSuite(t *testing.T) {
	suite.Run(t, new(LoginOrRegisterTestSuite))
}

// buildLoginOrRegisterFlow returns a unified authentication flow: identify the caller by email, then
// route a known user to password login, an ambiguous match to disambiguation, and an unknown email
// to registration. AUTHENTICATION requires AuthAssertExecutor, every onIncomplete must target a
// PROMPT, and both IdentifyingExecutor and CredentialsAuthExecutor need explicit email inputs because
// they default to username.
func buildLoginOrRegisterFlow(handle string) testutils.Flow {
	emailInput := []map[string]interface{}{
		{"ref": "input_email", "identifier": "email", "type": "TEXT_INPUT", "required": true},
	}

	return testutils.Flow{
		Name:     "Login Or Register Flow",
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_email"},
			{
				"id":   "prompt_email",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": emailInput,
						"action": map[string]interface{}{"ref": "action_email", "nextNode": "check_state"},
					},
				},
			},
			{
				"id":   "check_state",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name":   "IdentifyingExecutor",
					"mode":   "check_state",
					"inputs": emailInput,
				},
				"onSuccess": "login_existing",
			},
			// Known user: authenticate. Observable by the password prompt.
			{
				"id":   "login_existing",
				"type": "TASK_EXECUTION",
				"condition": map[string]interface{}{
					"key":    "{{ctx(entityState)}}",
					"value":  "exists",
					"onSkip": "route_ambiguous",
				},
				"executor": map[string]interface{}{
					"name": "CredentialsAuthExecutor",
					"inputs": []map[string]interface{}{
						{"ref": "input_email", "identifier": "email", "type": "TEXT_INPUT", "required": true},
						{"ref": "input_pwd", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
					},
				},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_password",
			},
			{
				"id":   "prompt_password",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_pwd", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_pwd", "nextNode": "login_existing"},
					},
				},
			},
			// Ambiguous match: disambiguate. Observable by the forwarded option lists.
			{
				"id":   "route_ambiguous",
				"type": "TASK_EXECUTION",
				"condition": map[string]interface{}{
					"key":    "{{ctx(entityState)}}",
					"value":  "ambiguous",
					"onSkip": "prompt_registration",
				},
				"executor": map[string]interface{}{
					"name":   "IdentifyingExecutor",
					"mode":   "resolve",
					"inputs": emailInput,
				},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_disambiguation",
			},
			{
				"id":   "prompt_disambiguation",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_ou", "identifier": "ouHandle", "type": "TEXT_INPUT", "required": false},
						},
						"action": map[string]interface{}{"ref": "action_disamb", "nextNode": "route_ambiguous"},
					},
				},
			},
			// Unknown email: the not_exists fall-through target. A plain PROMPT, never provisioning,
			// because auth-flow provisioning skips without userEligibleForProvisioning.
			{
				"id":   "prompt_registration",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_signup", "identifier": "signupUsername", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_signup", "nextNode": "auth_assert"},
					},
				},
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
}

func (ts *LoginOrRegisterTestSuite) SetupSuite() {
	parentOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "login-or-register-test-ou",
		Name:        "Login Or Register Test OU",
		Description: "Parent OU for login-or-register branch routing tests",
	})
	ts.Require().NoError(err, "Failed to create parent OU")
	ts.parentOUID = parentOUID

	// Two child OUs give ouHandle two distinct values for a shared email.
	ts.ouAHandle = "login-or-register-ou-a"
	ts.ouAID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.ouAHandle, Name: "Login Or Register OU A", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "Failed to create OU A")

	ts.ouBHandle = "login-or-register-ou-b"
	ts.ouBID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.ouBHandle, Name: "Login Or Register OU B", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "Failed to create OU B")

	ts.typeAName = "login-or-register-type-a"
	ts.typeAID, err = testutils.CreateUserType(testutils.UserType{
		Name: ts.typeAName, OUID: ts.ouAID, AllowSelfRegistration: true, Schema: loginOrRegisterSchema,
	})
	ts.Require().NoError(err, "Failed to create user type A")

	ts.typeBName = "login-or-register-type-b"
	ts.typeBID, err = testutils.CreateUserType(testutils.UserType{
		Name: ts.typeBName, OUID: ts.ouBID, AllowSelfRegistration: true, Schema: loginOrRegisterSchema,
	})
	ts.Require().NoError(err, "Failed to create user type B")

	// One user with a unique email drives the exists branch.
	soloName := common.GenerateUniqueUsername("solo")
	ts.soloEmail = soloName + "@example.com"
	ts.soloPassword = "SoloPass123!"
	soloIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID: ts.ouAID, Type: ts.typeAName,
		Attributes: json.RawMessage(`{"username": "` + soloName + `", "email": "` + ts.soloEmail +
			`", "password": "` + ts.soloPassword + `"}`),
	})
	ts.Require().NoError(err, "Failed to create the single-match user")
	ts.createdUserIDs = append(ts.createdUserIDs, soloIDs...)

	// Two users sharing an email across different OUs and types drive the ambiguous branch.
	ts.sharedEmail = common.GenerateUniqueUsername("shared") + "@example.com"
	sharedIDs, err := testutils.CreateMultipleUsers(
		testutils.User{OUID: ts.ouAID, Type: ts.typeAName, Attributes: json.RawMessage(
			`{"username": "` + common.GenerateUniqueUsername("sharedA") + `", "email": "` +
				ts.sharedEmail + `", "password": "SharedPass123!"}`)},
		testutils.User{OUID: ts.ouBID, Type: ts.typeBName, Attributes: json.RawMessage(
			`{"username": "` + common.GenerateUniqueUsername("sharedB") + `", "email": "` +
				ts.sharedEmail + `", "password": "SharedPass123!"}`)},
	)
	ts.Require().NoError(err, "Failed to create the ambiguous user pair")
	ts.createdUserIDs = append(ts.createdUserIDs, sharedIDs...)

	flowID, err := testutils.CreateFlow(buildLoginOrRegisterFlow("auth_flow_login_or_register"))
	ts.Require().NoError(err, "Failed to create the login-or-register flow")
	ts.flowID = flowID

	appID, err := testutils.CreateApplication(testutils.Application{
		OUID:             ts.parentOUID,
		Name:             "Login Or Register App",
		ClientID:         "login_or_register_client",
		ClientSecret:     "login_or_register_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{ts.typeAName, ts.typeBName},
		AuthFlowID:       flowID,
	})
	ts.Require().NoError(err, "Failed to create the test application")
	ts.appID = appID
}

func (ts *LoginOrRegisterTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.createdUserIDs); err != nil {
		ts.T().Logf("Failed to clean up users: %v", err)
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("Failed to delete app: %v", err)
		}
	}
	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("Failed to delete flow: %v", err)
		}
	}
	for _, typeID := range []string{ts.typeAID, ts.typeBID} {
		if err := testutils.DeleteUserType(typeID); err != nil {
			ts.T().Logf("Failed to delete user type %s: %v", typeID, err)
		}
	}
	for _, ouID := range []string{ts.ouAID, ts.ouBID, ts.parentOUID} {
		if err := testutils.DeleteOrganizationUnit(ouID); err != nil {
			ts.T().Logf("Failed to delete OU %s: %v", ouID, err)
		}
	}
}

// submitEmail drives the flow to the branch point for the given email.
func (ts *LoginOrRegisterTestSuite) submitEmail(email string) *common.FlowStep {
	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate the login-or-register flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "email"),
		"Expected the email prompt at the start of the flow")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"email": email}, "action_email", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the email")
	return flowStep
}

// Scenario 13: a known email routes to password login, not registration
// (entityState "exists", identifying_executor.go:289), and authentication completes for that account.
func (ts *LoginOrRegisterTestSuite) TestKnownEmailRoutesToPasswordLogin() {
	flowStep := ts.submitEmail(ts.soloEmail)

	ts.Require().NotEqual("COMPLETE", flowStep.FlowStatus,
		"A known email must stop for the password, not complete outright")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"),
		"A known email must route to password login")
	ts.False(common.HasInput(flowStep.Data.Inputs, "signupUsername"),
		"A known email must not route to registration")
	ts.Empty(flowStep.Assertion, "No assertion may be issued before the password is verified")

	// Supplying the correct password completes authentication for that account.
	flowStep, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"password": ts.soloPassword}, "action_pwd", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the password")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"The correct password must complete authentication")
	ts.NotEmpty(flowStep.Assertion, "A completed authentication must issue an assertion")
}

// Scenario 12: an unknown email routes to registration, not login
// (entityState "not_exists", identifying_executor.go:286). The branch falls through directly to a
// PROMPT because auth-flow provisioning would skip without userEligibleForProvisioning.
func (ts *LoginOrRegisterTestSuite) TestUnknownEmailRoutesToRegistration() {
	unknownEmail := "unknown-" + common.GenerateUniqueUsername("x") + "@example.com"
	flowStep := ts.submitEmail(unknownEmail)

	ts.Require().NotEqual("COMPLETE", flowStep.FlowStatus,
		"An unknown email must stop to collect registration details")
	ts.True(common.HasInput(flowStep.Data.Inputs, "signupUsername"),
		"An unknown email must route to the registration prompt")
	ts.False(common.HasInput(flowStep.Data.Inputs, "password"),
		"An unknown email must not route to password login")
	ts.Empty(flowStep.Assertion, "No assertion may be issued for an unknown user")
}

// Scenario 14: an email shared by two accounts routes to disambiguation rather than guessing
// (entityState "ambiguous", identifying_executor.go:293). Guessing would be an account-takeover
// vector, so the flow must never proceed to password login for an arbitrary account.
func (ts *LoginOrRegisterTestSuite) TestAmbiguousEmailRoutesToDisambiguation() {
	flowStep := ts.submitEmail(ts.sharedEmail)

	ts.Require().NotEqual("COMPLETE", flowStep.FlowStatus,
		"An ambiguous email must never resolve to an account on its own")
	ts.Empty(flowStep.Assertion,
		"No assertion may be issued while the identity is ambiguous")
	ts.False(common.HasInput(flowStep.Data.Inputs, "password"),
		"An ambiguous email must not proceed to password login for an arbitrary account")

	hasDisambiguator := common.HasInput(flowStep.Data.Inputs, "ouHandle") ||
		common.HasInput(flowStep.Data.Inputs, "userType")
	ts.True(hasDisambiguator,
		"An ambiguous email must offer a disambiguating attribute, got inputs %+v",
		flowStep.Data.Inputs)
}

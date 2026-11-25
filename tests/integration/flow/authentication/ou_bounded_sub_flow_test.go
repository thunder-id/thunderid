// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// errCodeAttributeTaken is returned when a unique attribute value is already taken.
const errCodeAttributeTaken = "FET-1061"

// ouBoundarySignUpInputs are the attributes the sign-up flows collect.
var ouBoundarySignUpInputs = []map[string]interface{}{
	{"ref": "input_username", "identifier": "username", "type": "TEXT_INPUT", "required": true},
	{"ref": "input_email", "identifier": "email", "type": "TEXT_INPUT", "required": true},
	{"ref": "input_password", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
}

// ouBoundaryRegistrationCallerFlow returns a registration flow whose login option CALLs the sign-in flow
// calleeID, asserting the user on the call's success route and on its failure route alike.
func ouBoundaryRegistrationCallerFlow(handle, calleeID string) testutils.Flow {
	return testutils.Flow{
		Name:     "OU Boundary " + handle,
		FlowType: "REGISTRATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_initial"},
			{
				"id":   "prompt_initial",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{"action": map[string]interface{}{"ref": "action_login", "nextNode": "call_sign_in"}},
					{"action": map[string]interface{}{"ref": "action_register", "nextNode": "user_type_resolver"}},
				},
			},
			{
				"id":        "call_sign_in",
				"type":      "CALL",
				"flow":      map[string]interface{}{"ref": calleeID},
				"onSuccess": "assert_signed_in",
				"onFailure": "assert_refused",
			},
			{
				"id":        "assert_signed_in",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{
				"id":        "assert_refused",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
				"onSuccess": "end",
			},
			{
				"id":        "user_type_resolver",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "UserTypeResolver"},
				"onSuccess": "provisioning",
			},
			{
				"id":        "provisioning",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "ProvisioningExecutor"},
				"onSuccess": "end",
			},
			{"id": "end", "type": "END"},
		},
	}
}

// ouBoundarySignUpFlow returns a registration flow that collects the attributes, checks the unique ones when
// checkUniqueness is set, and provisions the user.
func ouBoundarySignUpFlow(handle string, checkUniqueness bool) testutils.Flow {
	afterPrompt := "provision_user"
	if checkUniqueness {
		afterPrompt = "check_uniqueness"
	}
	nodes := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "user_type_resolver"},
		{
			"id":        "user_type_resolver",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "UserTypeResolver"},
			"onSuccess": "prompt_user_data",
		},
		{
			"id":   "prompt_user_data",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": ouBoundarySignUpInputs,
					"action": map[string]interface{}{"ref": "action_submit", "nextNode": afterPrompt},
				},
			},
		},
		{
			"id":        "provision_user",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "ProvisioningExecutor", "inputs": ouBoundarySignUpInputs},
			"onSuccess": "end",
		},
		{"id": "end", "type": "END"},
	}
	if checkUniqueness {
		nodes = append(nodes, map[string]interface{}{
			"id":           "check_uniqueness",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "AttributeUniquenessValidator"},
			"onSuccess":    "provision_user",
			"onIncomplete": "prompt_user_data",
		})
	}
	return testutils.Flow{Name: "OU Boundary " + handle, FlowType: "REGISTRATION", Handle: handle, Nodes: nodes}
}

// ouBoundarySignInCallingSignUpFlow returns a sign-in flow whose sign-up option CALLs the registration flow
// calleeID and, on its success route, asks for a nickname before the assertion.
func ouBoundarySignInCallingSignUpFlow(handle, calleeID string) testutils.Flow {
	return testutils.Flow{
		Name:     "OU Boundary " + handle,
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_options"},
			{
				"id":   "prompt_options",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{"action": map[string]interface{}{"ref": "action_signup", "nextNode": "call_sign_up"}},
				},
			},
			{
				"id":        "call_sign_up",
				"type":      "CALL",
				"flow":      map[string]interface{}{"ref": calleeID},
				"onSuccess": "prompt_after_sign_up",
				"onFailure": "end",
			},
			{
				"id":   "prompt_after_sign_up",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_nickname", "identifier": "nickname", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_after", "nextNode": "auth_assert"},
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

// createRegistrationApp creates an application in ouID with a sign-in and a registration flow, allowing the
// parent's user type only.
func (ts *OUBoundedSignInTestSuite) createRegistrationApp(name, clientID, ouID, authFlowID,
	registrationFlowID string) string {
	ts.T().Helper()
	id, err := testutils.CreateApplication(testutils.Application{
		Name:                      name,
		OUID:                      ouID,
		ClientID:                  clientID,
		ClientSecret:              clientID + "_secret",
		AuthFlowID:                authFlowID,
		RegistrationFlowID:        registrationFlowID,
		IsRegistrationFlowEnabled: true,
		AllowedUserTypes:          []string{ts.parentType},
		AssertionConfig:           map[string]interface{}{"userAttributes": []string{"ouId"}},
	})
	ts.Require().NoError(err, "create application %s", name)
	ts.appIDs = append(ts.appIDs, id)
	return id
}

// usersNamed returns the users with the given username.
func (ts *OUBoundedSignInTestSuite) usersNamed(username string) []testutils.User {
	ts.T().Helper()
	status, raw := ts.request(testutils.GetHTTPClient(), http.MethodGet,
		"/users?filter="+url.QueryEscape(`username eq "`+username+`"`), nil)
	ts.Require().Equal(http.StatusOK, status, string(raw))
	var list testutils.UserListResponse
	ts.Require().NoError(json.Unmarshal(raw, &list))
	return list.Users
}

// signInThroughRegistration starts the application's registration flow, takes its login option and submits
// the shared user's credentials to the called sign-in.
func (ts *OUBoundedSignInTestSuite) signInThroughRegistration(appID string) *common.FlowStep {
	ts.T().Helper()
	options, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", options.FlowStatus, "the registration offers its options: %+v", options.Error)

	callee, err := common.CompleteFlow(options.ExecutionID, map[string]string{}, "action_login",
		options.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().True(common.HasInput(callee.Data.Inputs, "username"), "the called sign-in asks for credentials")

	step, err := common.CompleteFlow(callee.ExecutionID,
		map[string]string{"username": ouBoundarySharedUser, "password": ouBoundaryPassword}, "action_001",
		callee.ChallengeToken)
	ts.Require().NoError(err)
	return step
}

// requireSignedInByTheCaller checks that the caller asserted the shared user the sign-in sub-flow signed in.
func (ts *OUBoundedSignInTestSuite) requireSignedInByTheCaller(step *common.FlowStep) {
	ts.T().Helper()
	ts.Require().Equal("COMPLETE", step.FlowStatus, "the caller asserts the user: %+v", step.Error)
	claims, err := testutils.DecodeJWT(step.Assertion)
	ts.Require().NoError(err)
	ts.Equal(ts.sharedUserID, claims.Sub)
}

// pickOUThroughRegistration signs the shared user in through the application's registration and picks ouID in the
// called sign-in.
func (ts *OUBoundedSignInTestSuite) pickOUThroughRegistration(appID, ouID string) *common.FlowStep {
	ts.T().Helper()
	step := ts.signInThroughRegistration(appID)
	ts.Require().True(common.HasInput(step.Data.Inputs, "ouId"), "the sign-in asks for the OU: %+v", step.Error)
	next, err := common.CompleteFlow(step.ExecutionID, map[string]string{"ouId": ouID}, "action_ou",
		step.ChallengeToken)
	ts.Require().NoError(err)
	return next
}

// ouBoundaryUnassertedSignInFlow returns a sign-in flow whose credentials route ends without its assertion step,
// after the user picks an OU below parentOUID when it is set. Its assertion is reached only by an action the tests
// never take.
func ouBoundaryUnassertedSignInFlow(handle, parentOUID string) testutils.Flow {
	credentialInputs := []map[string]interface{}{
		{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
		{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
	}
	afterCredentials := "end"
	if parentOUID != "" {
		afterCredentials = "resolve_ou"
	}
	nodes := []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{
			"id":   "prompt_credentials",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": credentialInputs,
					"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
				},
				{"action": map[string]interface{}{"ref": "action_assert", "nextNode": "auth_assert"}},
			},
		},
		{
			"id":           "credentials_auth",
			"type":         "TASK_EXECUTION",
			"executor":     map[string]interface{}{"name": "CredentialsAuthExecutor", "inputs": credentialInputs},
			"onSuccess":    afterCredentials,
			"onIncomplete": "prompt_credentials",
		},
		{
			"id":        "auth_assert",
			"type":      "TASK_EXECUTION",
			"executor":  map[string]interface{}{"name": "AuthAssertExecutor"},
			"onSuccess": "end",
		},
		{"id": "end", "type": "END"},
	}
	if parentOUID != "" {
		nodes = append(nodes,
			map[string]interface{}{
				"id":           "resolve_ou",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "OUResolverExecutor"},
				"properties":   map[string]interface{}{"resolveFrom": "prompt", "parentOuId": parentOUID},
				"onSuccess":    "end",
				"onIncomplete": "prompt_ou",
			},
			map[string]interface{}{
				"id":   "prompt_ou",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_ou", "identifier": "ouId", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_ou", "nextNode": "resolve_ou"},
					},
				},
			},
		)
	}
	return testutils.Flow{Name: "OU Boundary " + handle, FlowType: "AUTHENTICATION", Handle: handle, Nodes: nodes}
}

// A sign-in sub-flow is bounded only inside a bounded caller, so a registration flow runs the sign-in it CALLs
// unbounded. Its pick and its assertion step are not limited to an OU, and it returns to the call's success route
// with its user whichever OU it decided, or none, with or without its own assertion step.
func (ts *OUBoundedSignInTestSuite) TestRegistrationCallingSignIn_SignInRunsUnbounded() {
	noOUCaller := ts.createFlow(ouBoundaryRegistrationCallerFlow("ou_boundary_reg_calls_no_ou",
		ts.credentialsFlowID))
	noOUApp := ts.createRegistrationApp("OU Boundary Registration No OU", "ou_boundary_reg_no_ou", ts.parentOUID,
		ts.credentialsFlowID, noOUCaller)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(noOUApp))

	pickCallee := ts.createFlow(ouBoundaryFlow("ou_boundary_callee_picks_ou",
		map[string]interface{}{"resolveFrom": "prompt", "parentOuId": ts.parentOUID}))
	pickCaller := ts.createFlow(ouBoundaryRegistrationCallerFlow("ou_boundary_reg_calls_pick_ou", pickCallee))
	pickApp := ts.createRegistrationApp("OU Boundary Registration Pick OU", "ou_boundary_reg_pick_ou",
		ts.parentOUID, pickCallee, pickCaller)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(pickApp))

	unassertedCallee := ts.createFlow(ouBoundaryUnassertedSignInFlow("ou_boundary_callee_unasserted_pick",
		ts.parentOUID))
	unassertedCaller := ts.createFlow(ouBoundaryRegistrationCallerFlow("ou_boundary_reg_calls_unasserted_pick",
		unassertedCallee))
	unassertedApp := ts.createRegistrationApp("OU Boundary Registration Unasserted Pick",
		"ou_boundary_reg_unasserted_pick", ts.parentOUID, unassertedCallee, unassertedCaller)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(unassertedApp))

	ts.Run("NoOUDecided", func() {
		ts.requireSignedInByTheCaller(ts.signInThroughRegistration(noOUApp))
	})
	for name, ouID := range map[string]string{"OUTheUserIsIn": ts.childOUID, "OUTheUserIsNotIn": ts.childTwoOUID} {
		ts.Run(name, func() {
			ts.requireSignedInByTheCaller(ts.pickOUThroughRegistration(pickApp, ouID))
		})
		ts.Run(name+"WithoutItsOwnAssertion", func() {
			ts.requireSignedInByTheCaller(ts.pickOUThroughRegistration(unassertedApp, ouID))
		})
	}
}

// A sign-up sub-flow called from a sign-in is not bounded: in a shared application's sign-in, which has no OU
// yet, the sign-up returns to the caller's success route and provisions the user into its user type's OU, as
// before. A bounded sub-flow would have been refused for ending without an OU.
func (ts *OUBoundedSignInTestSuite) TestSignInCallingSignUp_SignUpIsNotBounded() {
	signUp := ts.createFlow(ouBoundarySignUpFlow("ou_boundary_sign_up_callee", false))
	caller := ts.createFlow(ouBoundarySignInCallingSignUpFlow("ou_boundary_sign_in_calls_sign_up", signUp))
	appID := ts.createRegistrationApp("OU Boundary Sign In Calls Sign Up", "ou_boundary_sign_in_calls_sign_up",
		ts.parentOUID, caller, signUp)
	ts.Require().NoError(testutils.ShareApplicationWithChildUnits(appID))

	options, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", options.FlowStatus, "the sign-in offers its options: %+v", options.Error)
	form, err := common.CompleteFlow(options.ExecutionID, map[string]string{}, "action_signup", options.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().True(common.HasInput(form.Data.Inputs, "username"), "the sign-up asks for the attributes")

	username := common.GenerateUniqueUsername("ou_boundary_sign_up")
	after, err := common.CompleteFlow(form.ExecutionID, map[string]string{
		"username": username, "email": username + "@ou-boundary.test", "password": ouBoundaryPassword,
	}, "action_submit", form.ChallengeToken)
	created := ts.usersNamed(username)
	for _, user := range created {
		ts.T().Cleanup(func() {
			if err := testutils.DeleteUser(user.ID); err != nil {
				ts.T().Logf("cleanup: delete user %s: %v", user.ID, err)
			}
		})
	}
	ts.Require().NoError(err)

	ts.Equal("INCOMPLETE", after.FlowStatus, "%+v", after.Error)
	ts.Nil(after.Error)
	ts.True(common.HasInput(after.Data.Inputs, "nickname"), "the sign-up returned to the caller's success route")
	ts.Require().Len(created, 1, "the sign-up provisions the user")
	ts.Equal(ts.parentOUID, created[0].OUID, "the user lands in its user type's OU")
}

// A registration flow has no boundary: for an unshared application in the child, the uniqueness check still
// finds a username taken by a user in another OU, whom a sign-in to that application would not find.
func (ts *OUBoundedSignInTestSuite) TestRegistration_LookupsReachOtherOUs() {
	signUp := ts.createFlow(ouBoundarySignUpFlow("ou_boundary_sign_up_unique", true))
	appID := ts.createRegistrationApp("OU Boundary Child Registration", "ou_boundary_child_registration",
		ts.childOUID, ts.credentialsFlowID, signUp)
	ts.requireNotFound(ts.signIn(appID, "", ouBoundaryChildTwoUser, nil))

	form, err := common.InitiateRegistrationFlow(appID, false, nil, "")
	ts.Require().NoError(err)
	ts.Require().True(common.HasInput(form.Data.Inputs, "username"), "the sign-up asks for the attributes: %+v",
		form.Error)
	email := common.GenerateUniqueUsername("ou_boundary_taken") + "@ou-boundary.test"
	step, err := common.CompleteFlow(form.ExecutionID, map[string]string{
		"username": ouBoundaryChildTwoUser, "email": email, "password": ouBoundaryPassword,
	}, "action_submit", form.ChallengeToken)
	ts.Require().NoError(err)

	ts.Equal("INCOMPLETE", step.FlowStatus, "the taken username is asked for again")
	ts.Require().NotNil(step.Error, "the taken username is refused: %+v", step)
	ts.Equal(errCodeAttributeTaken, step.Error.Code)
	ts.Equal("username", step.Error.Message.Params["attribute"])
	ts.Len(ts.usersNamed(ouBoundaryChildTwoUser), 1, "no second account is created")
}

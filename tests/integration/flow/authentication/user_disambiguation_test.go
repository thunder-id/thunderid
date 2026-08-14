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
	// errCodeUserNotFound is returned when filtering leaves no candidate.
	errCodeUserNotFound = "FET-1001"
	// errCodeFailedToIdentifyUser is returned when candidates cannot be told apart.
	errCodeFailedToIdentifyUser = "FET-1002"
)

// disambiguationSchema declares no unique attributes so several users may share every value, which
// scenario 17 requires: extractDisambiguationOptions only offers an attribute with more than one
// distinct value (identifying_executor.go:507).
var disambiguationSchema = map[string]interface{}{
	"username": map[string]interface{}{"type": "string"},
	"email":    map[string]interface{}{"type": "string"},
	"password": map[string]interface{}{"type": "string", "credential": true},
}

type UserDisambiguationTestSuite struct {
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
	sharedEmail    string
	passwordA      string
	passwordB      string
	twinEmail      string
	resolveAppID   string
	hintAppID      string
	createdFlowIDs []string
	createdUserIDs []string
}

func TestUserDisambiguationTestSuite(t *testing.T) {
	suite.Run(t, new(UserDisambiguationTestSuite))
}

// buildResolveFlow returns an authentication flow whose IdentifyingExecutor runs in resolve mode and,
// once a single account is resolved, authenticates it. Both executors need explicit email inputs
// because they default to username. onIncomplete targets a PROMPT in both places.
func buildResolveFlow(handle string) testutils.Flow {
	emailInput := []map[string]interface{}{
		{"ref": "input_email", "identifier": "email", "type": "TEXT_INPUT", "required": true},
	}

	return testutils.Flow{
		Name:     "Resolve Mode Flow",
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
						"action": map[string]interface{}{"ref": "action_email", "nextNode": "resolve_user"},
					},
				},
			},
			{
				"id":   "resolve_user",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name":   "IdentifyingExecutor",
					"mode":   "resolve",
					"inputs": emailInput,
				},
				"onSuccess":    "authenticate_resolved",
				"onIncomplete": "prompt_email",
			},
			{
				"id":   "authenticate_resolved",
				"type": "TASK_EXECUTION",
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
						"action": map[string]interface{}{"ref": "action_pwd", "nextNode": "authenticate_resolved"},
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

// buildLoginHintFlow is a separate graph for scenario 19. loginHintAttribute cannot live on the
// shared resolver node, or scenarios 15-18 would start demanding a login_hint input too.
func buildLoginHintFlow(handle string) testutils.Flow {
	return testutils.Flow{
		Name:     "Login Hint Identify Flow",
		FlowType: "AUTHENTICATION",
		Handle:   handle,
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "identify_by_hint"},
			{
				"id":         "identify_by_hint",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"loginHintAttribute": "email"},
				"executor": map[string]interface{}{
					"name": "IdentifyingExecutor",
					"inputs": []map[string]interface{}{
						{"ref": "input_hint", "identifier": "login_hint", "type": "TEXT_INPUT", "required": true},
					},
				},
				"onSuccess":    "auth_assert",
				"onIncomplete": "prompt_hint",
			},
			{
				"id":   "prompt_hint",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_hint", "identifier": "login_hint", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_hint", "nextNode": "identify_by_hint"},
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

func (ts *UserDisambiguationTestSuite) SetupSuite() {
	parentOUID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "user-disambiguation-test-ou",
		Name:        "User Disambiguation Test OU",
		Description: "Parent OU for identifying executor resolve mode tests",
	})
	ts.Require().NoError(err, "Failed to create parent OU")
	ts.parentOUID = parentOUID

	ts.ouAHandle = "disambiguation-ou-a"
	ts.ouAID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.ouAHandle, Name: "Disambiguation OU A", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "Failed to create OU A")

	ts.ouBHandle = "disambiguation-ou-b"
	ts.ouBID, err = testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle: ts.ouBHandle, Name: "Disambiguation OU B", Parent: &ts.parentOUID,
	})
	ts.Require().NoError(err, "Failed to create OU B")

	ts.typeAName = "disambiguation-type-a"
	ts.typeAID, err = testutils.CreateUserType(testutils.UserType{
		Name: ts.typeAName, OUID: ts.ouAID, AllowSelfRegistration: true, Schema: disambiguationSchema,
	})
	ts.Require().NoError(err, "Failed to create user type A")

	ts.typeBName = "disambiguation-type-b"
	ts.typeBID, err = testutils.CreateUserType(testutils.UserType{
		Name: ts.typeBName, OUID: ts.ouBID, AllowSelfRegistration: true, Schema: disambiguationSchema,
	})
	ts.Require().NoError(err, "Failed to create user type B")

	// Two users sharing an email, differing in OU and type, with DIFFERENT passwords. The distinct
	// passwords are what prove which account a disambiguation actually resolved to.
	ts.sharedEmail = common.GenerateUniqueUsername("shared") + "@example.com"
	ts.passwordA = "PasswordForA123!"
	ts.passwordB = "PasswordForB456!"
	sharedIDs, err := testutils.CreateMultipleUsers(
		testutils.User{OUID: ts.ouAID, Type: ts.typeAName, Attributes: json.RawMessage(
			`{"username": "` + common.GenerateUniqueUsername("sharedA") + `", "email": "` +
				ts.sharedEmail + `", "password": "` + ts.passwordA + `"}`)},
		testutils.User{OUID: ts.ouBID, Type: ts.typeBName, Attributes: json.RawMessage(
			`{"username": "` + common.GenerateUniqueUsername("sharedB") + `", "email": "` +
				ts.sharedEmail + `", "password": "` + ts.passwordB + `"}`)},
	)
	ts.Require().NoError(err, "Failed to create the ambiguous user pair")
	ts.createdUserIDs = append(ts.createdUserIDs, sharedIDs...)

	// Two users identical in OU, type and every attribute value, so no attribute can tell them apart.
	ts.twinEmail = common.GenerateUniqueUsername("twin") + "@example.com"
	twinUsername := common.GenerateUniqueUsername("twin")
	twinAttrs := json.RawMessage(`{"username": "` + twinUsername + `", "email": "` + ts.twinEmail +
		`", "password": "TwinPass123!"}`)
	twinIDs, err := testutils.CreateMultipleUsers(
		testutils.User{OUID: ts.ouAID, Type: ts.typeAName, Attributes: twinAttrs},
		testutils.User{OUID: ts.ouAID, Type: ts.typeAName, Attributes: twinAttrs},
	)
	ts.Require().NoError(err, "Failed to create the indistinguishable user pair")
	ts.createdUserIDs = append(ts.createdUserIDs, twinIDs...)

	resolveFlowID, err := testutils.CreateFlow(buildResolveFlow("auth_flow_resolve_mode"))
	ts.Require().NoError(err, "Failed to create the resolve flow")
	ts.createdFlowIDs = append(ts.createdFlowIDs, resolveFlowID)

	ts.resolveAppID, err = testutils.CreateApplication(testutils.Application{
		OUID:             ts.parentOUID,
		Name:             "Disambiguation Resolve App",
		ClientID:         "disambiguation_resolve_client",
		ClientSecret:     "disambiguation_resolve_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{ts.typeAName, ts.typeBName},
		AuthFlowID:       resolveFlowID,
		// Emit the identity claims so a test can verify WHICH account was authenticated, not merely
		// that some authentication succeeded.
		AssertionConfig: map[string]interface{}{
			"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
		},
	})
	ts.Require().NoError(err, "Failed to create the resolve app")

	hintFlowID, err := testutils.CreateFlow(buildLoginHintFlow("auth_flow_login_hint"))
	ts.Require().NoError(err, "Failed to create the login-hint flow")
	ts.createdFlowIDs = append(ts.createdFlowIDs, hintFlowID)

	ts.hintAppID, err = testutils.CreateApplication(testutils.Application{
		OUID:             ts.parentOUID,
		Name:             "Disambiguation Login Hint App",
		ClientID:         "disambiguation_hint_client",
		ClientSecret:     "disambiguation_hint_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		AllowedUserTypes: []string{ts.typeAName},
		AuthFlowID:       hintFlowID,
	})
	ts.Require().NoError(err, "Failed to create the login-hint app")
}

func (ts *UserDisambiguationTestSuite) TearDownSuite() {
	if err := testutils.CleanupUsers(ts.createdUserIDs); err != nil {
		ts.T().Logf("Failed to clean up users: %v", err)
	}
	for _, appID := range []string{ts.resolveAppID, ts.hintAppID} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete app %s: %v", appID, err)
		}
	}
	for _, flowID := range ts.createdFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete flow %s: %v", flowID, err)
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

// startResolve drives the resolve flow up to and including the first email submission.
func (ts *UserDisambiguationTestSuite) startResolve(email string) *common.FlowStep {
	flowStep, err := common.InitiateAuthenticationFlow(ts.resolveAppID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate the resolve flow")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"email": email}, "action_email", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the email for resolve")
	return flowStep
}

// findDisambiguationInput returns the input with the given identifier, or nil.
func findDisambiguationInput(inputs []common.Inputs, identifier string) *common.Inputs {
	for i := range inputs {
		if inputs[i].Identifier == identifier {
			return &inputs[i]
		}
	}
	return nil
}

// Scenario 15: two candidates in different OUs yield an ouHandle option list, and supplying one
// resolves to that single account, proven by only that account's password authenticating.
func (ts *UserDisambiguationTestSuite) TestResolveDisambiguatesByOUHandle() {
	flowStep := ts.startResolve(ts.sharedEmail)
	ts.Require().NotEqual("COMPLETE", flowStep.FlowStatus,
		"An ambiguous resolve must not complete on the first round")
	ts.Empty(flowStep.Assertion, "No assertion may be issued while the identity is ambiguous")

	ouInput := findDisambiguationInput(flowStep.Data.Inputs, "ouHandle")
	ts.Require().NotNil(ouInput, "Expected an ouHandle disambiguation input, got %+v",
		flowStep.Data.Inputs)
	ts.ElementsMatch([]string{ts.ouAHandle, ts.ouBHandle}, ouInput.Options,
		"Expected both OU handles as disambiguation options")

	// Disambiguate to OU A, then authenticate.
	flowStep, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"email": ts.sharedEmail, "ouHandle": ts.ouAHandle},
		"action_email", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the ouHandle disambiguation value")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"),
		"Resolving to a single account must proceed to the password prompt")

	// The other account's password must be REJECTED. Without this, a defect where credential auth
	// re-resolves by email+password instead of honouring the disambiguated account would still pass.
	rejected, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"password": ts.passwordB}, "action_pwd", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the non-selected account's password")
	ts.Require().Equal("INCOMPLETE", rejected.FlowStatus,
		"The non-selected account's password must be rejected and the flow must stay resumable")
	ts.Empty(rejected.Assertion,
		"No assertion may be issued for the non-selected account's password")

	// Only the selected account's own password completes the flow.
	flowStep, err = common.CompleteFlow(rejected.ExecutionID,
		map[string]string{"password": ts.passwordA}, "action_pwd", rejected.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the selected account's password")
	ts.Require().Equal("COMPLETE", flowStep.FlowStatus,
		"The disambiguated account's own password must complete authentication")
	ts.Require().NotEmpty(flowStep.Assertion,
		"A completed authentication must issue an assertion for the selected account")

	// The assertion must name the account in OU A, not the one sharing its email in OU B.
	_, err = testutils.ValidateJWTAssertionFields(flowStep.Assertion, ts.resolveAppID,
		ts.typeAName, ts.ouAID, "Disambiguation OU A", ts.ouAHandle)
	ts.Require().NoError(err, "The assertion must identify the disambiguated account in OU A")
}

// Scenario 16: the same pair also differs by userType, so userType works as a disambiguator and
// resolves to the other account, proven by only that account's password authenticating.
func (ts *UserDisambiguationTestSuite) TestResolveDisambiguatesByUserType() {
	flowStep := ts.startResolve(ts.sharedEmail)

	typeInput := findDisambiguationInput(flowStep.Data.Inputs, "userType")
	ts.Require().NotNil(typeInput, "Expected a userType disambiguation input, got %+v",
		flowStep.Data.Inputs)
	ts.ElementsMatch([]string{ts.typeAName, ts.typeBName}, typeInput.Options,
		"Expected both user types as disambiguation options")

	// Disambiguate to type B this time.
	flowStep, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"email": ts.sharedEmail, "userType": ts.typeBName},
		"action_email", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the userType disambiguation value")
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "password"),
		"Resolving to a single account must proceed to the password prompt")

	flowStep, err = common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"password": ts.passwordB}, "action_pwd", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the password")
	ts.Equal("COMPLETE", flowStep.FlowStatus,
		"Resolving by userType must authenticate the account in that type, not the other one")
	ts.NotEmpty(flowStep.Assertion, "A completed authentication must issue an assertion")
}

// Scenario 17: candidates identical in every extractable attribute offer no disambiguation options,
// so the resolve fails outright (identifying_executor.go:378-384). Resolving to an arbitrary one of
// two accounts would be an account-takeover vector.
func (ts *UserDisambiguationTestSuite) TestResolveIndistinguishableCandidatesFails() {
	flowStep := ts.startResolve(ts.twinEmail)

	ts.NotEqual("COMPLETE", flowStep.FlowStatus, "Indistinguishable candidates must not resolve")
	ts.Empty(flowStep.Assertion, "No assertion may be issued for indistinguishable candidates")
	ts.Require().NotNil(flowStep.Error,
		"Expected an error when candidates offer no disambiguation options")
	ts.Equal(errCodeFailedToIdentifyUser, flowStep.Error.Code)
	ts.False(common.HasInput(flowStep.Data.Inputs, "password"),
		"Indistinguishable candidates must never reach password login")
	ts.Nil(findDisambiguationInput(flowStep.Data.Inputs, "ouHandle"),
		"Indistinguishable candidates must not offer ouHandle options")
	ts.Nil(findDisambiguationInput(flowStep.Data.Inputs, "userType"),
		"Indistinguishable candidates must not offer userType options")
}

// Scenario 18: a second-round value matching no stored candidate filters to zero, which re-prompts
// with a user-not-found error rather than failing hard (identifying_executor.go:248-253).
func (ts *UserDisambiguationTestSuite) TestResolveSecondRoundNoMatchRePrompts() {
	flowStep := ts.startResolve(ts.sharedEmail)
	ts.Require().NotNil(findDisambiguationInput(flowStep.Data.Inputs, "ouHandle"),
		"Expected disambiguation options on the first round")

	flowStep, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"email": ts.sharedEmail, "ouHandle": "no-such-ou-handle"},
		"action_email", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the non-matching disambiguation value")

	// The contract is a re-prompt, not a termination: assert the exact INCOMPLETE status and that the
	// email prompt came back. Asserting only "not COMPLETE" would also pass on ERROR.
	ts.Equal("INCOMPLETE", flowStep.FlowStatus,
		"Filtering to zero candidates must re-prompt, not terminate the flow")
	ts.True(common.HasInput(flowStep.Data.Inputs, "email"),
		"onIncomplete must route back to the email prompt, got inputs %+v", flowStep.Data.Inputs)
	ts.Empty(flowStep.Assertion, "No assertion may be issued when filtering leaves no candidate")
	ts.Require().NotNil(flowStep.Error, "Expected a user-not-found error after filtering to zero")
	ts.Equal(errCodeUserNotFound, flowStep.Error.Code)
	ts.False(common.HasInput(flowStep.Data.Inputs, "password"),
		"Filtering to zero candidates must not reach password login")
}

// Scenario 19: with loginHintAttribute set the identifier came from outside the interaction, so an
// unresolvable hint stays failed rather than being promoted to a re-prompt
// (identifying_executor.go:184-195). The hint is consumed from the login_hint input, not from email.
func (ts *UserDisambiguationTestSuite) TestIdentifyWithLoginHintDoesNotRePrompt() {
	absentEmail := "login-hint-absent-" + common.GenerateUniqueUsername("x") + "@example.com"

	flowStep, err := common.InitiateAuthenticationFlow(ts.hintAppID, false,
		map[string]string{"login_hint": absentEmail}, "")
	ts.Require().NoError(err, "Failed to initiate the login-hint identify flow")

	ts.NotEqual("COMPLETE", flowStep.FlowStatus,
		"An unresolvable login hint must not complete the flow")
	ts.Empty(flowStep.Assertion, "No assertion may be issued for an unresolvable login hint")
	ts.Require().NotNil(flowStep.Error, "Expected a user-not-found error for the absent hint")
	ts.Equal(errCodeUserNotFound, flowStep.Error.Code)
	ts.False(common.HasInput(flowStep.Data.Inputs, "login_hint"),
		"With loginHintAttribute set, a not-found user must not be re-prompted for the identifier")
}

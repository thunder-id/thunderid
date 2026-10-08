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
	// The two ambiguous users share this email, so identifying by email alone cannot separate them.
	sharedIdentifyEmail = "shared@identify.test"
	uniqueIdentifyEmail = "unique@identify.test"
	absentIdentifyEmail = "absent@identify.test"
)

var (
	identifyTestOU = testutils.OrganizationUnit{
		Handle:      "identify-modes-test-ou",
		Name:        "Identify Modes Test Organization Unit",
		Description: "Organization unit for identifying executor mode testing",
		Parent:      nil,
	}

	identifyTestUserType = testutils.UserType{
		Handle:      "identify-modes-person",
		DisplayName: "Identify Modes Person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
		},
	}
)

// identifyNode builds the identifying executor node for the given mode, searching on email.
func identifyNode(mode, onSuccess, onIncomplete string) map[string]interface{} {
	return map[string]interface{}{
		"id":   "identify_user",
		"type": "TASK_EXECUTION",
		"executor": map[string]interface{}{
			"name": "IdentifyingExecutor",
			"mode": mode,
			"inputs": []map[string]interface{}{
				{
					"ref":        "input_email",
					"identifier": "email",
					"type":       "TEXT_INPUT",
					"required":   true,
				},
			},
		},
		"onSuccess":    onSuccess,
		"onIncomplete": onIncomplete,
	}
}

// markerPrompt builds a prompt whose single input identifies which branch the flow reached, so a
// test can assert the branch from the response alone.
func markerPrompt(id, marker, nextNode string, condition map[string]interface{}) map[string]interface{} {
	node := map[string]interface{}{
		"id":   id,
		"type": "PROMPT",
		"prompts": []map[string]interface{}{
			{
				"inputs": []map[string]interface{}{
					{
						"ref":        marker,
						"identifier": marker,
						"type":       "TEXT_INPUT",
						"required":   true,
					},
				},
				"action": map[string]interface{}{
					"ref":      "action_" + marker,
					"nextNode": nextNode,
				},
			},
		},
	}
	if condition != nil {
		node["condition"] = condition
	}
	return node
}

// identifyResolveFlowNodes builds a flow that resolves a user by email, asking for a distinguishing
// attribute when more than one user matches.
func identifyResolveFlowNodes() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "prompt_email",
		},
		{
			"id":   "prompt_email",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_email",
							"identifier": "email",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_email",
						"nextNode": "identify_user",
					},
				},
			},
		},
		identifyNode("resolve", "prompt_resolved", "prompt_disambiguate"),
		// given_name is declared as a select so the distinct candidate values forwarded by the
		// executor are merged into it as options.
		{
			"id":   "prompt_disambiguate",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_given_name",
							"identifier": "given_name",
							"type":       "SELECT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_disambiguate",
						"nextNode": "identify_user",
					},
				},
			},
		},
		markerPrompt("prompt_resolved", "resolved_marker", "auth_assert", nil),
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
	}
}

// identifyCheckStateFlowNodes builds a flow that records how many users match and then branches on
// that state, so each of the three outcomes lands on its own prompt.
func identifyCheckStateFlowNodes() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "prompt_email",
		},
		{
			"id":   "prompt_email",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "input_email",
							"identifier": "email",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_email",
						"nextNode": "identify_user",
					},
				},
			},
		},
		identifyNode("check_state", "prompt_exists", "prompt_email"),
		markerPrompt("prompt_exists", "state_exists", "auth_assert", map[string]interface{}{
			"key":    "{{ctx(entityState)}}",
			"value":  "exists",
			"onSkip": "prompt_ambiguous",
		}),
		markerPrompt("prompt_ambiguous", "state_ambiguous", "auth_assert", map[string]interface{}{
			"key":    "{{ctx(entityState)}}",
			"value":  "ambiguous",
			"onSkip": "prompt_not_exists",
		}),
		markerPrompt("prompt_not_exists", "state_not_exists", "auth_assert", nil),
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
	}
}

type IdentifyModesTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	ouID            string
	userTypeID      string
	resolveAppID    string
	checkStateAppID string
}

func TestIdentifyModesTestSuite(t *testing.T) {
	suite.Run(t, new(IdentifyModesTestSuite))
}

func (ts *IdentifyModesTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(identifyTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	userType := identifyTestUserType
	userType.OUID = ouID
	ts.userTypeID, err = testutils.CreateUserType(userType)
	ts.Require().NoError(err, "Failed to create test user type")

	// Two users share an email and differ only in given_name, which is what makes the email
	// ambiguous and given_name the attribute that can separate them.
	for _, attrs := range []string{
		`{"username":"identify_ada","email":"` + sharedIdentifyEmail + `","given_name":"Ada"}`,
		`{"username":"identify_bob","email":"` + sharedIdentifyEmail + `","given_name":"Bob"}`,
		`{"username":"identify_cleo","email":"` + uniqueIdentifyEmail + `","given_name":"Cleo"}`,
	} {
		userID, err := testutils.CreateUser(testutils.User{
			Type:       identifyTestUserType.Handle,
			OUID:       ouID,
			Attributes: json.RawMessage(attrs),
		})
		ts.Require().NoError(err, "Failed to create test user")
		ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, userID)
	}

	resolveFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Identify Resolve Test Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_identify_resolve_test",
		Nodes:    identifyResolveFlowNodes(),
	})
	ts.Require().NoError(err, "Failed to create resolve mode flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, resolveFlowID)

	checkStateFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Identify Check State Test Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_identify_check_state_test",
		Nodes:    identifyCheckStateFlowNodes(),
	})
	ts.Require().NoError(err, "Failed to create check state mode flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, checkStateFlowID)

	ts.resolveAppID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Identify Resolve Test Application",
		Description:      "Application for testing identifying executor resolve mode",
		ClientID:         "identify_resolve_test_client",
		ClientSecret:     "identify_resolve_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		OUID:             ouID,
		AllowedUserTypes: []string{identifyTestUserType.Handle},
		AuthFlowID:       resolveFlowID,
	})
	ts.Require().NoError(err, "Failed to create resolve mode application")

	ts.checkStateAppID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Identify Check State Test Application",
		Description:      "Application for testing identifying executor check state mode",
		ClientID:         "identify_check_state_test_client",
		ClientSecret:     "identify_check_state_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		OUID:             ouID,
		AllowedUserTypes: []string{identifyTestUserType.Handle},
		AuthFlowID:       checkStateFlowID,
	})
	ts.Require().NoError(err, "Failed to create check state mode application")
}

func (ts *IdentifyModesTestSuite) TearDownSuite() {
	for _, appID := range []string{ts.resolveAppID, ts.checkStateAppID} {
		if appID == "" {
			continue
		}
		if err := testutils.DeleteApplication(appID); err != nil {
			ts.T().Logf("Failed to delete test application during teardown: %v", err)
		}
	}
	for _, userID := range ts.config.CreatedUserIDs {
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("Failed to delete test user during teardown: %v", err)
		}
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		if err := testutils.DeleteFlow(flowID); err != nil {
			ts.T().Logf("Failed to delete test flow during teardown: %v", err)
		}
	}
	if ts.userTypeID != "" {
		if err := testutils.DeleteUserType(ts.userTypeID); err != nil {
			ts.T().Logf("Failed to delete test user type during teardown: %v", err)
		}
	}
	if ts.ouID != "" {
		if err := testutils.DeleteOrganizationUnit(ts.ouID); err != nil {
			ts.T().Logf("Failed to delete test organization unit during teardown: %v", err)
		}
	}
}

// submitEmail drives a flow to its first prompt and answers it with the given email.
func (ts *IdentifyModesTestSuite) submitEmail(appID, email string) *common.FlowStep {
	ts.T().Helper()

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Flow should pause for the email")

	step, err = common.CompleteFlow(step.ExecutionID,
		map[string]string{"email": email}, "action_email", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the email")
	return step
}

// Resolve mode narrows a set of candidates instead of failing on ambiguity: an email matching two
// users returns the attribute that distinguishes them, with the candidate values as options, and
// answering it resolves the user. The candidates are carried in the execution, so the second pass
// filters the stored set rather than searching again.
func (ts *IdentifyModesTestSuite) TestResolve_AmbiguousCandidatesDisambiguated() {
	step := ts.submitEmail(ts.resolveAppID, sharedIdentifyEmail)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus,
		"Two matching users should pause the flow for disambiguation")

	givenName := findInput(step.Data.Inputs, "given_name")
	ts.Require().NotNil(givenName, "The distinguishing attribute must be requested")
	ts.ElementsMatch([]string{"Ada", "Bob"}, givenName.Options,
		"The options must be the distinct values across the candidates")

	resolved, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"given_name": "Ada"}, "action_disambiguate", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the distinguishing attribute")
	ts.Equal("INCOMPLETE", resolved.FlowStatus, "The flow should continue once one user matches")
	ts.NotNil(findInput(resolved.Data.Inputs, "resolved_marker"),
		"Resolving a single user should carry the flow past the identifying node")
}

// An email matching exactly one user resolves without any disambiguation step.
func (ts *IdentifyModesTestSuite) TestResolve_SingleCandidateResolvesImmediately() {
	step := ts.submitEmail(ts.resolveAppID, uniqueIdentifyEmail)
	ts.Equal("INCOMPLETE", step.FlowStatus, "The flow should continue past the identifying node")
	ts.NotNil(findInput(step.Data.Inputs, "resolved_marker"),
		"A single match should resolve without a disambiguation prompt")
}

// An email matching no user pauses for more input rather than failing the flow, since the value is
// user supplied and correctable. The step is routed to the node's onIncomplete prompt, so what marks
// the outcome is the reported error together with the flow not advancing past identification.
func (ts *IdentifyModesTestSuite) TestResolve_NoCandidatesPausesWithUserNotFound() {
	step := ts.submitEmail(ts.resolveAppID, absentIdentifyEmail)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "No match should pause rather than fail the flow")
	ts.Require().NotNil(step.Error, "A non-matching email must be reported")
	ts.Equal(errCodeUserNotFound, step.Error.Code, "No match must be reported as user not found")
	ts.Nil(findInput(step.Data.Inputs, "resolved_marker"),
		"An unresolved user must not carry the flow past the identifying node")
}

// Check state mode never fails on the match count: it records whether zero, one, or several users
// match and lets the flow branch on it. Each of the three states is asserted from the prompt the
// flow lands on.
func (ts *IdentifyModesTestSuite) TestCheckState_ReportsEachMatchState() {
	for _, tc := range []struct {
		name   string
		email  string
		marker string
	}{
		{name: "single match", email: uniqueIdentifyEmail, marker: "state_exists"},
		{name: "multiple matches", email: sharedIdentifyEmail, marker: "state_ambiguous"},
		{name: "no match", email: absentIdentifyEmail, marker: "state_not_exists"},
	} {
		ts.Run(tc.name, func() {
			step := ts.submitEmail(ts.checkStateAppID, tc.email)
			ts.Require().Equal("INCOMPLETE", step.FlowStatus,
				"Check state mode should always carry the flow forward")
			ts.NotNil(findInput(step.Data.Inputs, tc.marker),
				"The flow should branch to the prompt for the %s state", tc.name)
		})
	}
}

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	consentInputIdentifier = "consent_decisions"
	consentPromptDataKey   = "consentPrompt"
	consentApproveAction   = "consent_action_allow"

	// The consent node of consentTimeoutTestFlow carries this timeout, in seconds. It is the
	// shortest wait that still crosses the expiry boundary once the test sleeps past it.
	consentTimeoutSeconds = 1
)

// consentPurposePrompt mirrors the purposes payload the consent executor forwards to the prompt,
// which is what the Console renders and what the decisions submitted back must line up with.
type consentPurposePrompt struct {
	PurposeName string `json:"purposeName"`
	PurposeID   string `json:"purposeId"`
	Type        string `json:"type"`
	Essential   []struct {
		Name string `json:"name"`
	} `json:"essential"`
	Optional []struct {
		Name string `json:"name"`
	} `json:"optional"`
}

var (
	consentTestOU = testutils.OrganizationUnit{
		Handle:      "consent-flow-test-ou",
		Name:        "Consent Flow Test Organization Unit",
		Description: "Organization unit for consent flow testing",
		Parent:      nil,
	}

	consentTestUserType = testutils.UserType{
		Handle:      "consent-test-person",
		DisplayName: "Consent Test Person",
		Schema: map[string]interface{}{
			"username": map[string]interface{}{
				"type": "string",
			},
			"password": map[string]interface{}{
				"type":       "string",
				"credential": true,
			},
			"email": map[string]interface{}{
				"type": "string",
			},
			"given_name": map[string]interface{}{
				"type": "string",
			},
		},
	}

	// consentTestUser has both attributes the application requests, so neither is dropped by the
	// profile-presence filter and both are prompted.
	consentTestUser = testutils.User{
		Type: "consent-test-person",
		Attributes: json.RawMessage(`{
			"username": "consent_user",
			"password": "SecurePass123!",
			"email": "consent.user@test.com",
			"given_name": "Consent"
		}`),
	}

	// A second user keeps the decision-handling tests off the consent record written by the
	// approve-then-skip test, which would otherwise suppress the prompt they depend on.
	consentDecisionUser = testutils.User{
		Type: "consent-test-person",
		Attributes: json.RawMessage(`{
			"username": "consent_decision_user",
			"password": "SecurePass123!",
			"email": "consent.decision@test.com",
			"given_name": "Decision"
		}`),
	}

	consentTimeoutUser = testutils.User{
		Type: "consent-test-person",
		Attributes: json.RawMessage(`{
			"username": "consent_timeout_user",
			"password": "SecurePass123!",
			"email": "consent.timeout@test.com",
			"given_name": "Timeout"
		}`),
	}
)

// consentFlowNodes builds an authentication flow that authenticates with credentials and then runs
// the consent executor, looping back into it from the consent prompt. nodeProperties is applied to
// the consent node, which is how the timeout variant differs from the default one.
func consentFlowNodes(nodeProperties map[string]interface{}) []map[string]interface{} {
	consentNode := map[string]interface{}{
		"id":   "consent_check",
		"type": "TASK_EXECUTION",
		"executor": map[string]interface{}{
			"name": "ConsentExecutor",
		},
		"onSuccess":    "auth_assert",
		"onIncomplete": "prompt_consent",
	}
	if len(nodeProperties) > 0 {
		consentNode["properties"] = nodeProperties
	}

	return []map[string]interface{}{
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
						{
							"ref":        "input_001",
							"identifier": "username",
							"type":       "TEXT_INPUT",
							"required":   true,
						},
						{
							"ref":        "input_002",
							"identifier": "password",
							"type":       "PASSWORD_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "action_001",
						"nextNode": "credentials_auth",
					},
				},
			},
		},
		{
			"id":   "credentials_auth",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "CredentialsAuthExecutor",
			},
			"onSuccess":    "consent_check",
			"onIncomplete": "prompt_credentials",
		},
		consentNode,
		{
			"id":   "prompt_consent",
			"type": "PROMPT",
			"meta": map[string]interface{}{
				"components": []map[string]interface{}{
					{
						"type": "BLOCK",
						"id":   "consent_block",
						"components": []map[string]interface{}{
							{
								"id":       "consent_input",
								"ref":      consentInputIdentifier,
								"type":     "CONSENT_INPUT",
								"required": true,
							},
							{
								"type":      "ACTION",
								"id":        consentApproveAction,
								"label":     "Approve",
								"eventType": "SUBMIT",
							},
						},
					},
				},
			},
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{
						{
							"ref":        "consent_input",
							"identifier": consentInputIdentifier,
							"type":       "CONSENT_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      consentApproveAction,
						"nextNode": "consent_check",
					},
				},
			},
		},
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

type ConsentFlowTestSuite struct {
	suite.Suite
	config *common.TestSuiteConfig

	ouID           string
	userTypeID     string
	appID          string
	timeoutAppID   string
	consentUserID  string
	decisionUserID string
	timeoutUserID  string
}

func TestConsentFlowTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentFlowTestSuite))
}

func (ts *ConsentFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ouID, err := testutils.CreateOrganizationUnit(consentTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	userType := consentTestUserType
	userType.OUID = ouID
	ts.userTypeID, err = testutils.CreateUserType(userType)
	ts.Require().NoError(err, "Failed to create test user type")

	// The requested user attributes are what the attribute consent purpose is derived from, so the
	// application's assertion config is what makes consent applicable at all.
	assertionConfig := map[string]interface{}{
		"userAttributes": []string{"email", "given_name"},
	}

	flowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Consent Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_consent_test",
		Nodes:    consentFlowNodes(nil),
	})
	ts.Require().NoError(err, "Failed to create consent test flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)

	ts.appID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Consent Flow Test Application",
		Description:      "Application for testing consent collection in flows",
		ClientID:         "consent_flow_test_client",
		ClientSecret:     "consent_flow_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		OUID:             ouID,
		AllowedUserTypes: []string{consentTestUserType.Handle},
		AuthFlowID:       flowID,
		AssertionConfig:  assertionConfig,
	})
	ts.Require().NoError(err, "Failed to create test application")

	timeoutFlowID, err := testutils.CreateFlow(testutils.Flow{
		Name:     "Consent Timeout Test Auth Flow",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_consent_timeout_test",
		Nodes: consentFlowNodes(map[string]interface{}{
			"timeout": "1",
		}),
	})
	ts.Require().NoError(err, "Failed to create consent timeout test flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, timeoutFlowID)

	ts.timeoutAppID, err = testutils.CreateApplication(testutils.Application{
		Name:             "Consent Timeout Flow Test Application",
		Description:      "Application for testing consent prompt expiry in flows",
		ClientID:         "consent_timeout_flow_test_client",
		ClientSecret:     "consent_timeout_flow_test_secret",
		RedirectURIs:     []string{"http://localhost:3000/callback"},
		OUID:             ouID,
		AllowedUserTypes: []string{consentTestUserType.Handle},
		AuthFlowID:       timeoutFlowID,
		AssertionConfig:  assertionConfig,
	})
	ts.Require().NoError(err, "Failed to create timeout test application")

	ts.consentUserID = ts.createUser(consentTestUser)
	ts.decisionUserID = ts.createUser(consentDecisionUser)
	ts.timeoutUserID = ts.createUser(consentTimeoutUser)
}

func (ts *ConsentFlowTestSuite) createUser(user testutils.User) string {
	ts.T().Helper()

	toCreate := user
	toCreate.OUID = ts.ouID
	userID, err := testutils.CreateUser(toCreate)
	ts.Require().NoError(err, "Failed to create test user")
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, userID)
	return userID
}

func (ts *ConsentFlowTestSuite) TearDownSuite() {
	for _, appID := range []string{ts.appID, ts.timeoutAppID} {
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

// authenticateToConsentPrompt drives the flow through credentials authentication and returns the
// step the consent executor paused on.
func (ts *ConsentFlowTestSuite) authenticateToConsentPrompt(appID, username string) *common.FlowStep {
	ts.T().Helper()

	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Flow should pause at the credentials prompt")

	step, err = common.CompleteFlow(step.ExecutionID, map[string]string{
		"username": username,
		"password": "SecurePass123!",
	}, "action_001", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit credentials")
	return step
}

// requireConsentPrompt asserts the step is a consent prompt and returns the prompted purposes.
func (ts *ConsentFlowTestSuite) requireConsentPrompt(step *common.FlowStep) []consentPurposePrompt {
	ts.T().Helper()

	ts.Require().Equal("INCOMPLETE", step.FlowStatus, "Consent should pause the flow for input")
	ts.Require().True(common.HasInput(step.Data.Inputs, consentInputIdentifier),
		"The consent prompt must request the consent decisions input")

	promptJSON, ok := step.Data.AdditionalData[consentPromptDataKey]
	ts.Require().True(ok, "The consent prompt must carry the purposes to render")

	var purposes []consentPurposePrompt
	ts.Require().NoError(json.Unmarshal([]byte(promptJSON), &purposes),
		"Consent prompt data should be a purposes array")
	ts.Require().NotEmpty(purposes, "At least one purpose must be prompted")
	return purposes
}

// decisionsFor builds a decisions payload answering every prompted element with approved.
func decisionsFor(purposes []consentPurposePrompt, approved bool, reason string) string {
	decisions := consentDecisions{Approved: approved, Reason: reason}
	for _, purpose := range purposes {
		decision := consentPurposeDecision{PurposeName: purpose.PurposeName, Approved: approved}
		for _, element := range purpose.Essential {
			decision.Elements = append(decision.Elements,
				consentElementDecision{Name: element.Name, Approved: approved})
		}
		for _, element := range purpose.Optional {
			decision.Elements = append(decision.Elements,
				consentElementDecision{Name: element.Name, Approved: approved})
		}
		decisions.Purposes = append(decisions.Purposes, decision)
	}

	payload, err := json.Marshal(decisions)
	if err != nil {
		return ""
	}
	return string(payload)
}

// The application requests user attributes the user has, so the first authentication must stop for
// consent, and approving every prompted element must record consent and complete the flow. A second
// authentication then finds that consent active and runs the consent node through without a prompt,
// which is the branch that separates "consent needed" from "already consented".
func (ts *ConsentFlowTestSuite) TestConsent_PromptedThenSkippedOnceRecorded() {
	step := ts.authenticateToConsentPrompt(ts.appID, "consent_user")
	purposes := ts.requireConsentPrompt(step)

	prompted := map[string]bool{}
	for _, purpose := range purposes {
		for _, element := range purpose.Optional {
			prompted[element.Name] = true
		}
	}
	ts.Contains(prompted, "email", "A requested attribute present in the profile must be prompted")
	ts.Contains(prompted, "given_name", "A requested attribute present in the profile must be prompted")

	completed, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		consentInputIdentifier: decisionsFor(purposes, true, ""),
	}, consentApproveAction, step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit consent decisions")
	ts.Require().Equal("COMPLETE", completed.FlowStatus,
		"Approving consent should let the flow reach its assertion")
	ts.NotEmpty(completed.Assertion, "A completed authentication must return an assertion")

	// Consent is now active for both attributes, so the second run has nothing left to prompt.
	repeat := ts.authenticateToConsentPrompt(ts.appID, "consent_user")
	ts.Equal("COMPLETE", repeat.FlowStatus,
		"An active consent record should let the consent node complete without prompting")
	ts.NotContains(repeat.Data.AdditionalData, consentPromptDataKey,
		"No consent prompt data should be forwarded when consent is already active")
}

// Denying every prompted element still records the decision and completes, because none of the
// requested attributes are essential. Nothing is consented, which downstream executors read from
// the empty consented-attributes runtime value.
func (ts *ConsentFlowTestSuite) TestConsent_DeniedOptionalElementsCompletesFlow() {
	step := ts.authenticateToConsentPrompt(ts.appID, "consent_decision_user")
	purposes := ts.requireConsentPrompt(step)

	completed, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		consentInputIdentifier: decisionsFor(purposes, false, "user_denied"),
	}, consentApproveAction, step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit consent denial")
	ts.Equal("COMPLETE", completed.FlowStatus,
		"Denying only optional attributes should not fail the flow")
}

// A decisions payload that is not valid JSON is rejected as a client error rather than crashing the
// node, since the value arrives as an opaque string from the client.
func (ts *ConsentFlowTestSuite) TestConsent_MalformedDecisionsRejected() {
	step := ts.authenticateToConsentPrompt(ts.appID, "consent_decision_user")
	ts.requireConsentPrompt(step)

	failed, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		consentInputIdentifier: "not-a-json-payload",
	}, consentApproveAction, step.ChallengeToken)
	ts.Require().NoError(err, "A malformed payload should still return a flow step")
	ts.Equal("ERROR", failed.FlowStatus, "Unparseable consent decisions must fail the step")
}

// Decisions submitted with the timeout reason are not a user decision: nothing is recorded and the
// flow proceeds with nothing consented. The expiry check is deliberately skipped for these, so this
// also pins that a late timeout submission is accepted rather than rejected as expired.
func (ts *ConsentFlowTestSuite) TestConsent_TimeoutReasonCompletesWithoutRecording() {
	step := ts.authenticateToConsentPrompt(ts.timeoutAppID, "consent_timeout_user")
	purposes := ts.requireConsentPrompt(step)

	stepTimeout, ok := step.Data.AdditionalData["stepTimeout"]
	ts.Require().True(ok, "A consent node with a timeout must publish the step expiry")
	ts.NotEmpty(stepTimeout, "The published step expiry must carry a timestamp")

	completed, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		consentInputIdentifier: decisionsFor(purposes, false, "timeout"),
	}, consentApproveAction, step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit timed out consent decisions")
	ts.Equal("COMPLETE", completed.FlowStatus,
		"A timed out consent prompt should complete without recording consent")
}

// A decision submitted after the configured consent timeout has passed is refused, so a stale
// prompt cannot be answered. Unlike the timeout-reason case above, this submission claims to be a
// real decision, which is what makes the expiry check apply.
func (ts *ConsentFlowTestSuite) TestConsent_ExpiredPromptRejected() {
	step := ts.authenticateToConsentPrompt(ts.timeoutAppID, "consent_timeout_user")
	purposes := ts.requireConsentPrompt(step)

	time.Sleep(consentTimeoutSeconds*time.Second + 2*time.Second)

	failed, err := common.CompleteFlow(step.ExecutionID, map[string]string{
		consentInputIdentifier: decisionsFor(purposes, true, ""),
	}, consentApproveAction, step.ChallengeToken)
	ts.Require().NoError(err, "An expired prompt should still return a flow step")
	ts.Equal("ERROR", failed.FlowStatus, "A decision submitted after the timeout must be refused")
}

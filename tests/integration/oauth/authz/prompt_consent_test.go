// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authz

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
	promptConsentClientID    = "prompt_consent_test_client"
	promptConsentRedirectURI = "https://localhost:3000"
	promptConsentUsername    = "prompt_consent_test_user"
	promptConsentPassword    = "PromptConsentPass1!"
)

var promptConsentUserType = testutils.UserType{
	Handle:      "prompt-consent-person",
	DisplayName: "Prompt Consent Person",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{"type": "string"},
		"password": map[string]interface{}{"type": "string", "credential": true},
		"email":    map[string]interface{}{"type": "string"},
	},
}

// promptConsentAuthFlow mirrors the console-generated "Consent Flow" template: credentials, then an
// authorization check, then a consent check that prompts (via prompt_consent) only when required
// consents are missing or a reprompt has been forced.
var promptConsentAuthFlow = testutils.Flow{
	Name:     "Prompt Consent Test Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_prompt_consent_test",
	Nodes: []map[string]interface{}{
		{"id": "start", "type": "START", "onSuccess": "prompt_credentials"},
		{"id": "prompt_credentials", "type": "PROMPT", "prompts": []map[string]interface{}{
			{
				"inputs": []map[string]interface{}{
					{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
				},
				"action": map[string]interface{}{"ref": "action_001", "nextNode": "credentials_auth"},
			},
		}},
		{"id": "credentials_auth", "type": "TASK_EXECUTION", "executor": map[string]interface{}{
			"name": "CredentialsAuthExecutor",
			"inputs": []map[string]interface{}{
				{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
				{"ref": "input_002", "identifier": "password", "type": "PASSWORD_INPUT", "required": true},
			},
		}, "onSuccess": "authorization_check", "onIncomplete": "prompt_credentials"},
		{"id": "authorization_check", "type": "TASK_EXECUTION",
			"executor": map[string]interface{}{"name": "AuthorizationExecutor"}, "onSuccess": "consent_check"},
		{"id": "consent_check", "type": "TASK_EXECUTION", "executor": map[string]interface{}{
			"name": "ConsentExecutor",
			"inputs": []map[string]interface{}{
				{"ref": "input_f3px", "identifier": "consent_decisions", "type": "CONSENT_INPUT", "required": true},
			},
		}, "onSuccess": "auth_assert", "onIncomplete": "prompt_consent"},
		{"id": "prompt_consent", "type": "PROMPT", "prompts": []map[string]interface{}{
			{
				"inputs": []map[string]interface{}{
					{"ref": "input_0ho7", "identifier": "consent_decisions", "type": "CONSENT_INPUT", "required": true},
				},
				"action": map[string]interface{}{"ref": "consent_action_allow", "nextNode": "consent_check"},
			},
			{
				"inputs": []map[string]interface{}{
					{"ref": "input_0ho7", "identifier": "consent_decisions", "type": "CONSENT_INPUT", "required": true},
				},
				"action": map[string]interface{}{"ref": "consent_action_deny", "nextNode": "consent_check"},
			},
		}},
		{"id": "auth_assert", "type": "TASK_EXECUTION",
			"executor": map[string]interface{}{"name": "AuthAssertExecutor"}, "onSuccess": "end"},
		{"id": "end", "type": "END"},
	},
}

// consentPurposePrompt mirrors providers.ConsentPurposePrompt, just enough to build an
// "approve everything" ConsentDecisions payload from what the server forwards to the prompt.
type consentPurposePrompt struct {
	PurposeName string              `json:"purposeName"`
	Essential   []consentElementRef `json:"essential"`
	Optional    []consentElementRef `json:"optional"`
}

type consentElementRef struct {
	Name string `json:"name"`
}

// PromptConsentTestSuite verifies that prompt=consent forces the ConsentExecutor to re-prompt even
// when the user already holds valid consent for the application, distinct from the default behavior
// of skipping the prompt once consent has been granted.
type PromptConsentTestSuite struct {
	suite.Suite
	client       *http.Client
	ouID         string
	entityTypeID string
	authFlowID   string
	appID        string
	userID       string
}

// TestPromptConsentTestSuite runs the PromptConsentTestSuite.
func TestPromptConsentTestSuite(t *testing.T) {
	suite.Run(t, new(PromptConsentTestSuite))
}

// SetupSuite creates the shared organization unit, user type, consent-bearing auth flow, an
// application requiring consent for its optional "email" attribute, and the test user.
func (ts *PromptConsentTestSuite) SetupSuite() {
	ts.client = testutils.GetHTTPClient()

	ouID, err := testutils.CreateOrganizationUnit(testutils.OrganizationUnit{
		Handle:      "prompt-consent-test-ou",
		Name:        "Prompt Consent Test OU",
		Description: "Organization unit for prompt=consent integration tests",
	})
	ts.Require().NoError(err)
	ts.ouID = ouID

	promptConsentUserType.OUID = ouID
	entityTypeID, err := testutils.CreateUserType(promptConsentUserType)
	ts.Require().NoError(err)
	ts.entityTypeID = entityTypeID

	flowID, err := testutils.CreateFlow(promptConsentAuthFlow)
	ts.Require().NoError(err)
	ts.authFlowID = flowID

	app := map[string]interface{}{
		"name":                      "PromptConsentTestApp",
		"description":               "Application for prompt=consent integration testing",
		"ouId":                      ts.ouID,
		"type":                      "fullstack",
		"authFlowId":                ts.authFlowID,
		"isRegistrationFlowEnabled": false,
		"allowedUserTypes":          []string{"prompt-consent-person"},
		"assertion":                 map[string]interface{}{"userAttributes": []string{"email"}},
		"loginConsent":              map[string]interface{}{"validityPeriod": 3600},
		"inboundAuthConfig": []map[string]interface{}{
			{"type": "oauth2", "config": map[string]interface{}{
				"clientId":                promptConsentClientID,
				"clientSecret":            "prompt_consent_test_secret",
				"redirectUris":            []string{promptConsentRedirectURI},
				"grantTypes":              []string{"authorization_code"},
				"responseTypes":           []string{"code"},
				"tokenEndpointAuthMethod": "client_secret_basic",
			}},
		},
	}
	appJSON, err := json.Marshal(app)
	ts.Require().NoError(err)

	req, err := http.NewRequest("POST", testutils.TestServerURL+"/applications", bytes.NewBuffer(appJSON))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.client.Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	ts.Require().Equal(http.StatusCreated, resp.StatusCode, string(bodyBytes))

	var respData map[string]interface{}
	ts.Require().NoError(json.Unmarshal(bodyBytes, &respData))
	ts.appID = respData["id"].(string)

	attributesJSON, err := json.Marshal(map[string]interface{}{
		"username": promptConsentUsername,
		"password": promptConsentPassword,
		"email":    "prompt_consent_test@example.com",
	})
	ts.Require().NoError(err)
	userID, err := testutils.CreateUser(testutils.User{
		OUID:       ouID,
		Type:       "prompt-consent-person",
		Attributes: json.RawMessage(attributesJSON),
	})
	ts.Require().NoError(err)
	ts.userID = userID
}

// TearDownSuite deletes the resources created in SetupSuite.
func (ts *PromptConsentTestSuite) TearDownSuite() {
	if ts.userID != "" {
		_ = testutils.DeleteUser(ts.userID)
	}
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
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

// runLoginThroughCredentials initiates the authorization flow (optionally with a prompt parameter)
// and executes it through the credentials step, returning the resulting step (which is either the
// consent prompt or, if consent was skipped, the completed flow) along with the auth ID.
func (ts *PromptConsentTestSuite) runLoginThroughCredentials(prompt string) (string, *testutils.FlowStep) {
	var httpResp *http.Response
	var err error
	if prompt != "" {
		httpResp, err = testutils.InitiateAuthorizationFlowWithPrompt(
			promptConsentClientID, promptConsentRedirectURI, "code", "openid", "consent-test-state", prompt)
	} else {
		httpResp, err = testutils.InitiateAuthorizationFlow(
			promptConsentClientID, promptConsentRedirectURI, "code", "openid", "consent-test-state")
	}
	ts.Require().NoError(err)
	defer httpResp.Body.Close()
	ts.Require().Equal(http.StatusFound, httpResp.StatusCode)

	authID, executionID, err := testutils.ExtractAuthData(httpResp.Header.Get("Location"))
	ts.Require().NoError(err)

	initialStep, err := testutils.ExecuteAuthenticationFlow(executionID, nil, "")
	ts.Require().NoError(err)

	credsStep, err := testutils.ExecuteAuthenticationFlow(executionID, map[string]string{
		"username": promptConsentUsername,
		"password": promptConsentPassword,
	}, "action_001", initialStep.ChallengeToken)
	ts.Require().NoError(err)

	return authID, credsStep
}

// approveConsent decodes the forwarded consent purposes and submits an "approve everything"
// decision, returning the resulting (now complete) flow step.
func (ts *PromptConsentTestSuite) approveConsent(step *testutils.FlowStep) *testutils.FlowStep {
	ts.Require().NotNil(step.Data)
	raw, ok := step.Data.AdditionalData["consentPrompt"].(string)
	ts.Require().True(ok, "expected a consentPrompt entry in additionalData")

	var purposes []consentPurposePrompt
	ts.Require().NoError(json.Unmarshal([]byte(raw), &purposes))
	ts.Require().NotEmpty(purposes, "expected at least one consent purpose to approve")

	type elementDecision struct {
		Name     string `json:"name"`
		Approved bool   `json:"approved"`
	}
	type purposeDecision struct {
		PurposeName string            `json:"purposeName"`
		Approved    bool              `json:"approved"`
		Elements    []elementDecision `json:"elements"`
	}
	type consentDecisions struct {
		Approved bool              `json:"approved"`
		Purposes []purposeDecision `json:"purposes"`
	}

	decisions := consentDecisions{Approved: true}
	for _, p := range purposes {
		pd := purposeDecision{PurposeName: p.PurposeName, Approved: true}
		for _, e := range append(append([]consentElementRef{}, p.Essential...), p.Optional...) {
			pd.Elements = append(pd.Elements, elementDecision{Name: e.Name, Approved: true})
		}
		decisions.Purposes = append(decisions.Purposes, pd)
	}
	decisionsJSON, err := json.Marshal(decisions)
	ts.Require().NoError(err)

	nextStep, err := testutils.ExecuteAuthenticationFlow(step.ExecutionID, map[string]string{
		"consent_decisions": string(decisionsJSON),
	}, "consent_action_allow", step.ChallengeToken)
	ts.Require().NoError(err)
	return nextStep
}

// TestPromptConsent_ForcesReconsentDespiteExistingConsent verifies the full consent lifecycle: the
// first login requires and records consent, a second login without a prompt parameter skips the
// consent step entirely (the default behavior), and a third login with prompt=consent is forced back
// into the consent prompt even though valid consent already exists.
func (ts *PromptConsentTestSuite) TestPromptConsent_ForcesReconsentDespiteExistingConsent() {
	// Login 1: no prior consent, must be prompted.
	authID1, step1 := ts.runLoginThroughCredentials("")
	ts.Require().NotEqual("COMPLETE", step1.FlowStatus, "first login should require consent")
	completed1 := ts.approveConsent(step1)
	ts.Require().Equal("COMPLETE", completed1.FlowStatus, "flow should complete once consent is approved")
	ts.Require().NotEmpty(completed1.Assertion)
	_, err := testutils.CompleteAuthorization(authID1, completed1.Assertion)
	ts.Require().NoError(err)

	// Login 2: consent already granted and still valid, no prompt requested -> should skip straight
	// through to COMPLETE without ever hitting the consent prompt again.
	authID2, step2 := ts.runLoginThroughCredentials("")
	ts.Require().Equal("COMPLETE", step2.FlowStatus,
		"second login should skip the consent prompt since valid consent already exists")
	ts.Require().NotEmpty(step2.Assertion)
	_, err = testutils.CompleteAuthorization(authID2, step2.Assertion)
	ts.Require().NoError(err)

	// Login 3: prompt=consent forces a re-prompt despite the still-valid existing consent.
	_, step3 := ts.runLoginThroughCredentials("consent")
	ts.Require().NotEqual("COMPLETE", step3.FlowStatus,
		"prompt=consent should force a re-prompt even though consent is already valid")
	ts.Require().NotNil(step3.Data)
	_, hasConsentPrompt := step3.Data.AdditionalData["consentPrompt"]
	ts.Assert().True(hasConsentPrompt, "forced re-consent should forward consent purposes to prompt again")
}

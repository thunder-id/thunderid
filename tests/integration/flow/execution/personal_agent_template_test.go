// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package execution

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

const (
	personalAgentTemplateHandle = "personal-assistant-registration-flow"
	ownerSignInAction           = "action_001"
	defaultFlowHandle           = "default-flow"
)

// translationKeyPattern matches a label's translation key, {{ t(namespace:key) }}.
var translationKeyPattern = regexp.MustCompile(`t\(([a-zA-Z]+):([a-zA-Z0-9_.]+)\)`)

// PersonalAgentTemplateTestSuite registers through a flow created from the Console's "User and Personal
// Assistants" template, as the Console creates it, so the template stays in step with the executors, the
// shipped flows it calls and the translations it uses.
type PersonalAgentTemplateTestSuite struct {
	suite.Suite
	flowID    string
	loginFlow string
	appID     string
	userID    string
	username  string
	agentIDs  []string
	userIDs   []string
}

func TestPersonalAgentTemplateTestSuite(t *testing.T) {
	suite.Run(t, new(PersonalAgentTemplateTestSuite))
}

func (ts *PersonalAgentTemplateTestSuite) SetupSuite() {
	name, nodes := ts.templateConfig()
	var err error
	ts.flowID, err = testutils.CreateFlow(testutils.Flow{
		Name:     name,
		FlowType: registrationFlowType,
		Handle:   personalAgentTemplateHandle,
		Nodes:    nodes,
	})
	ts.Require().NoError(err)

	ts.loginFlow, err = testutils.GetFlowIDByHandle(personalAgentAuthFlowHandle, "AUTHENTICATION")
	ts.Require().NoError(err)

	userTypes, err := testutils.ListUserTypes()
	ts.Require().NoError(err)
	var ouID string
	for _, userType := range userTypes {
		if userType.Handle == seededUserTypeName {
			ouID = userType.OUID
		}
	}
	ts.Require().NotEmpty(ouID, "the seeded %q user type is needed", seededUserTypeName)

	ts.username = common.GenerateUniqueUsername("pa_template_owner")
	attributes, err := json.Marshal(map[string]string{
		"username": ts.username, "email": ts.username + "@example.com", "password": personalAgentPassword,
	})
	ts.Require().NoError(err)
	ts.userID, err = testutils.CreateUser(testutils.User{OUID: ouID, Type: seededUserTypeName, Attributes: attributes})
	ts.Require().NoError(err)

	// The template signs the owner in through the personal agent login flow, so the application signs in with it.
	ts.appID, err = testutils.CreateApplication(testutils.Application{
		Name:                      "pa_template_app",
		Description:               "personal assistant registration template test",
		OUID:                      ouID,
		AuthFlowID:                ts.loginFlow,
		RegistrationFlowID:        ts.flowID,
		IsRegistrationFlowEnabled: true,
		ClientID:                  "pa_template_app_client",
		ClientSecret:              "pa_template_app_secret",
		RedirectURIs:              []string{"http://localhost:3000/callback"},
		AllowedUserTypes:          []string{seededUserTypeName},
	})
	ts.Require().NoError(err)
}

func (ts *PersonalAgentTemplateTestSuite) TearDownSuite() {
	for _, agentID := range ts.agentIDs {
		if err := testutils.DeleteAgent(agentID); err != nil {
			ts.T().Logf("failed to delete agent %s: %v", agentID, err)
		}
	}
	if ts.appID != "" {
		if err := testutils.DeleteApplication(ts.appID); err != nil {
			ts.T().Logf("failed to delete app %s: %v", ts.appID, err)
		}
	}
	if ts.flowID != "" {
		if err := testutils.DeleteFlow(ts.flowID); err != nil {
			ts.T().Logf("failed to delete flow %s: %v", ts.flowID, err)
		}
	}
	for _, userID := range append(ts.userIDs, ts.userID) {
		if userID == "" {
			continue
		}
		if err := testutils.DeleteUser(userID); err != nil {
			ts.T().Logf("failed to delete user %s: %v", userID, err)
		}
	}
}

// The kind, the login flow and consent on every authorization are fixed by the flow, so values sent for them
// are ignored. The JWKS URI selects private_key_jwt, so the agent has no client secret.
func (ts *PersonalAgentTemplateTestSuite) TestRegistersAPersonalAgentWithAJWKSURI() {
	defaultFlowID, err := testutils.GetFlowIDByHandle(defaultFlowHandle, "AUTHENTICATION")
	ts.Require().NoError(err)

	executionID, challenge := ts.signIn()
	step, body := ts.step(executionID, challenge, testutils.PersonalAgentDetailsAction, map[string]string{
		agentNameInput:         common.GenerateUniqueUsername("pa_template_agent"),
		agentRedirectURIsInput: personalAgentCallback,
		"jwksUri":              personalAgentJWKSURI,
		"kind":                 "bogus",
		"authFlowId":           defaultFlowID,
		"alwaysPromptConsent":  "false",
	})
	ts.Require().Equal("COMPLETE", step.FlowStatus, body)
	agentID := step.Data.AdditionalData[agentIDData]
	ts.Require().NotEmpty(agentID, body)
	ts.agentIDs = append(ts.agentIDs, agentID)
	ts.NotEmpty(step.Data.AdditionalData[agentClientIDData])
	ts.Empty(step.Data.AdditionalData[agentClientSecretData])

	agent, err := testutils.GetAgent(agentID)
	ts.Require().NoError(err)
	ts.Equal(personalKind, agent.Kind)
	ts.Equal(ts.userID, agent.Owner, "the agent belongs to the user who signed in")
	ts.Equal(ts.loginFlow, agent.AuthFlowID)
	ts.Require().NotNil(agent.LoginConsent)
	ts.Require().NotNil(agent.LoginConsent.AlwaysPrompt)
	ts.True(*agent.LoginConsent.AlwaysPrompt)
	ts.Require().Len(agent.InboundAuthConfig, 1)
	ts.Require().NotNil(agent.InboundAuthConfig[0].Config)
	ts.Equal("private_key_jwt", agent.InboundAuthConfig[0].Config.TokenEndpointAuthMethod)
}

// The template registers only agents with a JWKS URI, so the details prompt asks for one again without it.
func (ts *PersonalAgentTemplateTestSuite) TestAsksForTheJWKSURI() {
	executionID, challenge := ts.signIn()
	step, body := ts.step(executionID, challenge, testutils.PersonalAgentDetailsAction, map[string]string{
		agentNameInput:         common.GenerateUniqueUsername("pa_template_no_key"),
		agentRedirectURIsInput: personalAgentCallback,
	})

	ts.Equal("INCOMPLETE", step.FlowStatus, body)
	ts.Empty(step.Data.AdditionalData[agentIDData], body)
	ts.Contains(inputIdentifiers(step), "jwksUri", body)
}

// The labels are translation keys the gate resolves, so every key the template uses must be served, and the
// template's own text asks for the JWKS URI and for the client ID to be copied.
func (ts *PersonalAgentTemplateTestSuite) TestTranslatesEveryLabel() {
	_, nodes := ts.templateConfig()
	raw, err := json.Marshal(nodes)
	ts.Require().NoError(err)

	keys := map[string][]string{}
	for _, match := range translationKeyPattern.FindAllStringSubmatch(string(raw), -1) {
		keys[match[1]] = append(keys[match[1]], match[2])
	}
	ts.Require().NotEmpty(keys)

	translations := map[string]map[string]string{}
	for namespace, nsKeys := range keys {
		translations[namespace] = ts.resolveTranslations(namespace)
		for _, key := range nsKeys {
			ts.NotEmpty(translations[namespace][key], "%s:%s has no translation", namespace, key)
		}
	}

	signup := translations["signup"]
	ts.Contains(signup["forms.personal_agent_details.jwks_subtitle"], "the URI that serves its public keys")
	ts.NotContains(signup["forms.personal_agent_details.fields.jwks_uri.required_placeholder"], "Optional")
	ts.Contains(signup["forms.personal_agent_credentials.jwks_message"], "Copy the client ID")
}

func (ts *PersonalAgentTemplateTestSuite) TestUserChoiceSignsAUserUp() {
	step, body := ts.step("", "", "", nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, body)
	step, body = ts.step(step.ExecutionID, step.ChallengeToken, testutils.RegisterUserAction, nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, body)

	username := common.GenerateUniqueUsername("pa_template_user")
	values := map[string]string{
		"userType": seededUserTypeName, "username": username, "password": personalAgentPassword,
		"email": username + "@example.com",
	}
	for i := 0; i < 5 && step.FlowStatus == "INCOMPLETE"; i++ {
		ts.Require().NotEmpty(step.Data.Actions, body)
		inputs := map[string]string{}
		for _, identifier := range inputIdentifiers(step) {
			value, ok := values[identifier]
			ts.Require().True(ok, "unexpected input %q: %s", identifier, body)
			inputs[identifier] = value
		}
		step, body = ts.step(step.ExecutionID, step.ChallengeToken, step.Data.Actions[0].Ref, inputs)
	}
	ts.Require().Equal("COMPLETE", step.FlowStatus, body)

	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err)
	ts.Require().NotNil(user)
	ts.userIDs = append(ts.userIDs, user.ID)
}

// signIn starts the registration, chooses a personal agent and signs the owner in through the called login
// flow. It returns the execution and the challenge token of the details prompt that follows.
func (ts *PersonalAgentTemplateTestSuite) signIn() (string, string) {
	ts.T().Helper()
	step, body := ts.step("", "", "", nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, body)
	step, body = ts.step(step.ExecutionID, step.ChallengeToken, testutils.RegisterPersonalAgentAction, nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, body)

	executionID := step.ExecutionID
	step, body = ts.step(executionID, step.ChallengeToken, ownerSignInAction, map[string]string{
		"username": ts.username, "password": personalAgentPassword,
	})
	ts.Require().Equal("INCOMPLETE", step.FlowStatus, body)
	ts.Require().Subset(inputIdentifiers(step), []string{agentNameInput, agentRedirectURIsInput, "jwksUri"}, body)
	ts.NotContains(inputIdentifiers(step), "publicKey")
	return executionID, step.ChallengeToken
}

// step posts one interaction to the template's registration flow. The first call starts the flow.
func (ts *PersonalAgentTemplateTestSuite) step(executionID, challengeToken, action string,
	inputs map[string]string) (common.FlowStep, string) {
	ts.T().Helper()
	payload := map[string]interface{}{"applicationId": ts.appID, "flowType": registrationFlowType}
	if executionID != "" {
		payload = map[string]interface{}{"executionId": executionID, "challengeToken": challengeToken}
	}
	if len(inputs) > 0 {
		payload["inputs"] = inputs
	}
	if action != "" {
		payload["action"] = action
	}
	reqBody, err := json.Marshal(payload)
	ts.Require().NoError(err)

	req, err := http.NewRequest(http.MethodPost, testServerURL+"/flow/execute", bytes.NewReader(reqBody))
	ts.Require().NoError(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := testutils.GetHTTPClient().Do(req)
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var step common.FlowStep
	ts.Require().NoError(json.Unmarshal(body, &step))
	return step, string(body)
}

// resolveTranslations returns the English translations of a namespace, as the gate receives them.
func (ts *PersonalAgentTemplateTestSuite) resolveTranslations(namespace string) map[string]string {
	ts.T().Helper()
	resp, err := testutils.GetHTTPClient().Get(testServerURL + "/i18n/languages/en-US/translations/resolve?namespace=" +
		url.QueryEscape(namespace))
	ts.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	ts.Require().NoError(err)
	ts.Require().Equal(http.StatusOK, resp.StatusCode, string(body))

	var resolved struct {
		Translations map[string]map[string]string `json:"translations"`
	}
	ts.Require().NoError(json.Unmarshal(body, &resolved))
	return resolved.Translations[namespace]
}

// templateConfig reads the template from the Console's template catalog and returns the flow name and nodes
// the Console sends when it creates a flow from it.
func (ts *PersonalAgentTemplateTestSuite) templateConfig() (string, []interface{}) {
	ts.T().Helper()
	_, file, _, ok := runtime.Caller(0)
	ts.Require().True(ok)
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file),
		"../../../../frontend/packages/configure-flows/src/data/templates.json"))
	ts.Require().NoError(err)

	var templates []struct {
		Config struct {
			Name   string        `json:"name"`
			Handle string        `json:"handle"`
			Nodes  []interface{} `json:"nodes"`
		} `json:"config"`
	}
	ts.Require().NoError(json.Unmarshal(raw, &templates))
	for _, template := range templates {
		if template.Config.Handle == personalAgentTemplateHandle {
			return template.Config.Name, template.Config.Nodes
		}
	}
	ts.FailNow("the template is not in the catalog")
	return "", nil
}

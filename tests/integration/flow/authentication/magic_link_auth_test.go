// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package authentication

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

// emailOTPRegex matches the OTP in the email OTP template.
var emailOTPRegex = regexp.MustCompile(`>\s*(\d{4,10})\s*</td>`)

var magicLinkAuthFlow = testutils.Flow{
	Name:     "Magic Link Auth Flow",
	FlowType: "AUTHENTICATION",
	Handle:   "auth_flow_magic_link_test",
	Nodes: []map[string]interface{}{
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
							"ref":        "input_001",
							"identifier": "email",
							"type":       "EMAIL_INPUT",
							"required":   true,
						},
					},
					"action": map[string]interface{}{
						"ref":      "magic_link_action",
						"nextNode": "send_magic_link",
					},
				},
			},
		},
		{
			"id":   "send_magic_link",
			"type": "TASK_EXECUTION",
			"properties": map[string]interface{}{
				"magicLinkURL": "https://localhost:8095/gate/signin",
			},
			"executor": map[string]interface{}{
				"name": "MagicLinkExecutor",
				"mode": "generate",
			},
			"onSuccess": "email_magic_link",
		},
		{
			"id":   "email_magic_link",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "EmailExecutor",
				"mode": "send",
				"inputs": []map[string]interface{}{
					{
						"ref":        "input_001",
						"identifier": "email",
						"type":       "EMAIL_INPUT",
						"required":   true,
					},
				},
			},
			"properties": map[string]interface{}{
				"emailTemplate": "MAGIC_LINK",
			},
			"onSuccess": "verify_magic_link",
		},
		{
			"id":   "verify_magic_link",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "MagicLinkExecutor",
				"mode": "verify",
			},
			"onSuccess": "auth_assert",
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
	},
}

var magicLinkTestApp = testutils.Application{
	Name:                      "Magic Link Test Application",
	Description:               "Application for testing magic link authentication",
	IsRegistrationFlowEnabled: false,
	ClientID:                  "magic_link_test_client",
	ClientSecret:              "magic_link_test_secret",
	RedirectURIs:              []string{"http://localhost:3000/callback"},
	AllowedUserTypes:          []string{"magic_link_test_user"},
	AssertionConfig: map[string]interface{}{
		"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
	},
}

var magicLinkTestUserSchema = testutils.UserType{
	Name: "magic_link_test_user",
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
		"family_name": map[string]interface{}{
			"type": "string",
		},
		"mobile_number": map[string]interface{}{
			"type": "string",
		},
	},
}

var magicLinkTestUser = testutils.User{
	Type: magicLinkTestUserSchema.Name,
	Attributes: json.RawMessage(`{
		"email": "userA@example.com",
		"given_name": "user",
		"family_name": "A"
	}`),
}

var magicLinkTestOU = testutils.OrganizationUnit{
	Handle:      "magic-link-test-ou",
	Name:        "Magic Link Test OU",
	Description: "Organization unit for magic link authentication tests",
}

type magicLinkAuthFlowTestSuite struct {
	suite.Suite
	config           *common.TestSuiteConfig
	mockSMTP         *testutils.MockSMTPServer
	ouID             string
	appID            string
	shortTTLAppID    string
	reusedTokenAppID string
	userSchemaID     string
	originalPatchSet bool
}

func TestMagicLinkAuthFlowTestSuite(t *testing.T) {
	suite.Run(t, new(magicLinkAuthFlowTestSuite))
}

func (ts *magicLinkAuthFlowTestSuite) SetupSuite() {
	ts.config = &common.TestSuiteConfig{}

	ts.mockSMTP = testutils.NewMockSMTPServer(0)
	ts.Require().NoError(ts.mockSMTP.Start(), "Failed to start mock SMTP server")

	patch := map[string]interface{}{
		"email": map[string]interface{}{
			"smtp": map[string]interface{}{
				"host":                  "localhost",
				"port":                  ts.mockSMTP.GetPort(),
				"username":              "",
				"password":              "",
				"from_address":          "no-reply@example.com",
				"enable_start_tls":      false,
				"enable_authentication": false,
			},
		},
		"jwt": map[string]interface{}{
			"leeway": 1,
		},
	}

	if err := testutils.PatchDeploymentConfig(patch); err != nil {
		ts.T().Fatalf("Failed to patch deployment config: %v", err)
	}
	ts.originalPatchSet = true

	if err := testutils.RestartServer(); err != nil {
		ts.T().Fatalf("Failed to restart server with SMTP configuration: %v", err)
	}
	if err := testutils.ObtainAdminAccessToken(); err != nil {
		ts.T().Fatalf("Failed to re-obtain admin token after restart: %v", err)
	}

	ouID, err := testutils.CreateOrganizationUnit(magicLinkTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	magicLinkTestUserSchema.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(magicLinkTestUserSchema)
	ts.Require().NoError(err, "Failed to create test user schema")
	ts.userSchemaID = schemaID

	magicLinkTestUser.OUID = ts.ouID
	userIDs, err := testutils.CreateMultipleUsers(magicLinkTestUser)
	ts.Require().NoError(err, "Failed to create test user")
	ts.config.CreatedUserIDs = userIDs

	flowID, err := testutils.CreateFlow(magicLinkAuthFlow)
	ts.Require().NoError(err, "Failed to create magic link flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	magicLinkTestApp.AuthFlowID = flowID
	magicLinkTestApp.OUID = ts.ouID

	appID, err := testutils.CreateApplication(magicLinkTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID

	// Build short TTL flow
	b, err := json.Marshal(magicLinkAuthFlow)
	ts.Require().NoError(err, "Failed to marshal magicLinkAuthFlow")
	var shortTTLFlow testutils.Flow
	err = json.Unmarshal(b, &shortTTLFlow)
	ts.Require().NoError(err, "Failed to unmarshal magicLinkAuthFlow")
	shortTTLFlow.Handle = "auth_flow_magic_link_test_short_ttl"
	shortTTLFlow.Name = "Magic Link Auth Flow Short TTL"

	ts.modifyFlowNode(&shortTTLFlow, "send_magic_link", func(node map[string]interface{}) {
		props, ok := node["properties"].(map[string]interface{})
		if !ok {
			props = make(map[string]interface{})
			node["properties"] = props
		}
		props["tokenExpiry"] = "2"
	})

	shortFlowID, err := testutils.CreateFlow(shortTTLFlow)
	ts.Require().NoError(err, "Failed to create short TTL magic link flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, shortFlowID)

	shortTTLApp := magicLinkTestApp
	shortTTLApp.Name = "Magic Link Test App Short TTL"
	shortTTLApp.ClientID = "magic_link_test_client_short_ttl"
	shortTTLApp.AuthFlowID = shortFlowID
	shortTTLApp.OUID = ts.ouID
	shortAppID, err := testutils.CreateApplication(shortTTLApp)
	ts.Require().NoError(err, "Failed to create short TTL test application")
	ts.shortTTLAppID = shortAppID

	// Build reused token flow: inserts a dummy prompt between verify_magic_link and
	// auth_assert that loops back to verify_magic_link, allowing the test to submit
	// the same token twice and assert the jti replay check.
	var reusedTokenFlow testutils.Flow
	err = json.Unmarshal(b, &reusedTokenFlow)
	ts.Require().NoError(err, "Failed to unmarshal reusedTokenFlow")
	reusedTokenFlow.Handle = "auth_flow_magic_link_test_reused"
	reusedTokenFlow.Name = "Magic Link Auth Flow Reused Token"

	ts.modifyFlowNode(&reusedTokenFlow, "verify_magic_link", func(node map[string]interface{}) {
		node["onSuccess"] = "dummy_prompt"
	})
	dummyPromptNode := map[string]interface{}{
		"id":   "dummy_prompt",
		"type": "PROMPT",
		"prompts": []map[string]interface{}{
			{
				"inputs": []map[string]interface{}{
					{
						"ref":        "dummy_input",
						"identifier": "dummy",
						"type":       "TEXT_INPUT",
						"required":   true,
					},
				},
				"action": map[string]interface{}{
					"ref":      "dummy_action",
					"nextNode": "verify_magic_link",
				},
			},
			{
				"inputs": []map[string]interface{}{},
				"action": map[string]interface{}{
					"ref":      "dummy_action2",
					"nextNode": "auth_assert",
				},
			},
		},
	}
	reusedNodesList := reusedTokenFlow.Nodes.([]interface{})
	var newNodesList []interface{}
	for _, n := range reusedNodesList {
		newNodesList = append(newNodesList, n)
		node, ok := n.(map[string]interface{})
		if ok && node["id"] == "verify_magic_link" {
			newNodesList = append(newNodesList, dummyPromptNode)
		}
	}
	reusedTokenFlow.Nodes = newNodesList

	reusedFlowID, err := testutils.CreateFlow(reusedTokenFlow)
	ts.Require().NoError(err, "Failed to create reused token magic link flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, reusedFlowID)

	reusedTokenApp := magicLinkTestApp
	reusedTokenApp.Name = "Magic Link Test App Reused Token"
	reusedTokenApp.ClientID = "magic_link_test_client_reused"
	reusedTokenApp.AuthFlowID = reusedFlowID
	reusedTokenApp.OUID = ts.ouID
	reusedAppID, err := testutils.CreateApplication(reusedTokenApp)
	ts.Require().NoError(err, "Failed to create reused token test application")
	ts.reusedTokenAppID = reusedAppID
}

func (ts *magicLinkAuthFlowTestSuite) TearDownSuite() {
	if ts.appID != "" {
		_ = testutils.DeleteApplication(ts.appID)
	}
	if ts.shortTTLAppID != "" {
		_ = testutils.DeleteApplication(ts.shortTTLAppID)
	}
	if ts.reusedTokenAppID != "" {
		_ = testutils.DeleteApplication(ts.reusedTokenAppID)
	}
	for _, flowID := range ts.config.CreatedFlowIDs {
		_ = testutils.DeleteFlow(flowID)
	}
	if err := testutils.CleanupUsers(ts.config.CreatedUserIDs); err != nil {
		ts.T().Logf("Failed to clean up users: %v", err)
	}
	if ts.userSchemaID != "" {
		_ = testutils.DeleteUserType(ts.userSchemaID)
	}
	if ts.ouID != "" {
		_ = testutils.DeleteOrganizationUnit(ts.ouID)
	}
	if ts.mockSMTP != nil {
		_ = ts.mockSMTP.Stop()
	}
	if ts.originalPatchSet {
		if err := testutils.UpdateDeploymentConfig("../../resources/deployment.yaml"); err != nil {
			ts.T().Logf("teardown: failed to restore deployment config: %v", err)
		}
		if err := testutils.RestartServer(); err != nil {
			ts.T().Logf("teardown: server did not restart cleanly after config restore: %v", err)
		}
		if err := testutils.ObtainAdminAccessToken(); err != nil {
			ts.T().Logf("teardown: failed to re-obtain admin token after restore: %v", err)
		}
	}
}

// modifyFlowNode safely finds a node by ID in a Flow and applies a modifier function to it.
func (ts *magicLinkAuthFlowTestSuite) modifyFlowNode(flow *testutils.Flow, nodeID string, modifier func(node map[string]interface{})) {
	nodesArray, ok := flow.Nodes.([]interface{})
	ts.Require().True(ok, "flow.Nodes is not a slice of interfaces")

	for _, n := range nodesArray {
		node, ok := n.(map[string]interface{})
		ts.Require().True(ok, "flow node is not a map[string]interface{}")
		if node["id"] == nodeID {
			modifier(node)
			return
		}
	}
	ts.Require().FailNow(fmt.Sprintf("Node with ID %s not found in flow", nodeID))
}

// waitForEmail polls the mock SMTP server until an email is received or the timeout is reached.
func (ts *magicLinkAuthFlowTestSuite) waitForEmail() *testutils.EmailMessage {
	var emailMessage *testutils.EmailMessage
	ts.Require().Eventually(func() bool {
		emailMessage = ts.mockSMTP.GetLastEmail()
		return emailMessage != nil
	}, 5*time.Second, 100*time.Millisecond, "Expected magic link email to be captured")
	return emailMessage
}

func (ts *magicLinkAuthFlowTestSuite) TestMagicLinkLoginFlow() {
	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate magic link flow")
	ts.Require().Equal("INCOMPLETE", flowStep.FlowStatus)
	ts.Require().Equal("VIEW", flowStep.Type)
	ts.Require().NotEmpty(flowStep.ExecutionID)
	ts.Require().True(common.HasInput(flowStep.Data.Inputs, "email"), "Email input should be required")
	ts.Require().True(common.HasAction(flowStep.Data.Actions, "magic_link_action"),
		"Magic link action should be available")

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": "userA@example.com"},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit magic link email")
	ts.Require().Equal("INCOMPLETE", step2.FlowStatus)
	ts.Require().Equal("VIEW", step2.Type)
	ts.Require().NotEmpty(step2.ChallengeToken)
	ts.Require().True(common.HasInput(step2.Data.Inputs, "token"), "Magic link token input should be required")
	ts.Require().Equal("true", step2.Data.AdditionalData["emailSent"], "Magic link email should be sent")

	emailMessage := ts.waitForEmail()

	token := common.ExtractMagicLinkToken(emailMessage)
	ts.Require().NotEmpty(token, "Expected magic link token to be present in the email body")

	completeStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"token": token}, "",
		step2.ChallengeToken)
	ts.Require().NoError(err, "Failed to complete magic link login")
	ts.Require().Equal("COMPLETE", completeStep.FlowStatus)
	ts.Require().NotEmpty(completeStep.Assertion)

	claims, err := testutils.ValidateJWTAssertionFields(completeStep.Assertion, ts.appID,
		magicLinkTestUserSchema.Name, ts.ouID, magicLinkTestOU.Name, magicLinkTestOU.Handle)
	ts.Require().NoError(err, "Failed to validate JWT assertion")
	ts.Require().Equal(magicLinkTestUserSchema.Name, claims.UserType)
	ts.Require().Equal(ts.ouID, claims.OUID)
}

func (ts *magicLinkAuthFlowTestSuite) TestMagicLinkLoginFlow_UsernameSendsToStoredEmail() {
	username := common.GenerateUniqueUsername("magiclinkuser")
	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID: ts.ouID,
		Type: magicLinkTestUserSchema.Name,
		Attributes: json.RawMessage(`{
			"username": "` + username + `",
			"email": "` + username + `@example.com"
		}`),
	})
	ts.Require().NoError(err, "Failed to create the username test user")
	defer func() {
		if err := testutils.CleanupUsers(userIDs); err != nil {
			ts.T().Logf("Failed to delete the username test user: %v", err)
		}
	}()

	flowID, err := testutils.CreateFlow(buildUsernameMagicLinkFlow())
	ts.Require().NoError(err, "Failed to create the username magic link flow")
	defer func() {
		_ = testutils.DeleteFlow(flowID)
	}()

	app := magicLinkTestApp
	app.Name = "Magic Link Test App Username"
	app.ClientID = "magic_link_test_client_username"
	app.AuthFlowID = flowID
	app.OUID = ts.ouID
	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "Failed to create the username magic link test application")
	defer func() {
		_ = testutils.DeleteApplication(appID)
	}()

	ts.mockSMTP.ClearEmails()
	flowStep, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate the username magic link flow")

	step2, err := common.CompleteFlow(flowStep.ExecutionID,
		map[string]string{"username": username, "email": "other@example.com"},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the username")
	ts.Require().Equal("INCOMPLETE", step2.FlowStatus)

	emailMessage := ts.waitForEmail()
	ts.Equal([]string{username + "@example.com"}, emailMessage.To,
		"The magic link goes to the email on the user's profile")
}

func (ts *magicLinkAuthFlowTestSuite) TestMagicLinkLoginFlow_AttributeSentWithEmailIsPromptedAfterLink() {
	email := common.GenerateUniqueUsername("magiclinkcollect") + "@example.com"
	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID:       ts.ouID,
		Type:       magicLinkTestUserSchema.Name,
		Attributes: json.RawMessage(`{"email": "` + email + `"}`),
	})
	ts.Require().NoError(err, "Failed to create the test user")
	defer func() {
		if err := testutils.CleanupUsers(userIDs); err != nil {
			ts.T().Logf("Failed to delete the test user: %v", err)
		}
	}()

	b, err := json.Marshal(magicLinkAuthFlow)
	ts.Require().NoError(err, "Failed to marshal magicLinkAuthFlow")
	var collectFlow testutils.Flow
	ts.Require().NoError(json.Unmarshal(b, &collectFlow), "Failed to unmarshal magicLinkAuthFlow")
	collectFlow.Handle = "auth_flow_magic_link_test_collect"
	collectFlow.Name = "Magic Link Auth Flow Attribute Collect"
	collectNode := map[string]interface{}{
		"id":   "attribute_collect",
		"type": "TASK_EXECUTION",
		"executor": map[string]interface{}{
			"name": "AttributeCollector",
			"inputs": []map[string]interface{}{
				{"ref": "input_002", "identifier": "mobile_number", "type": "TEXT_INPUT", "required": true},
			},
		},
		"onSuccess": "auth_assert",
	}
	var nodes []interface{}
	for _, n := range collectFlow.Nodes.([]interface{}) {
		node := n.(map[string]interface{})
		switch node["id"] {
		case "send_magic_link":
			node["executor"].(map[string]interface{})["inputs"] = []map[string]interface{}{
				{"ref": "input_001", "identifier": "email", "type": "EMAIL_INPUT", "required": true},
			}
		case "verify_magic_link":
			node["onSuccess"] = "attribute_collect"
		}
		nodes = append(nodes, node)
		if node["id"] == "verify_magic_link" {
			nodes = append(nodes, collectNode)
		}
	}
	collectFlow.Nodes = nodes

	flowID, err := testutils.CreateFlow(collectFlow)
	ts.Require().NoError(err, "Failed to create the attribute collect magic link flow")
	defer func() {
		_ = testutils.DeleteFlow(flowID)
	}()

	app := magicLinkTestApp
	app.Name = "Magic Link Test App Attribute Collect"
	app.ClientID = "magic_link_test_client_collect"
	app.AuthFlowID = flowID
	app.OUID = ts.ouID
	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "Failed to create the attribute collect test application")
	defer func() {
		_ = testutils.DeleteApplication(appID)
	}()

	// The request that asks for the link also sends a mobile number, either with the email step or as the
	// request that starts the flow.
	for _, onStart := range []bool{false, true} {
		ts.mockSMTP.ClearEmails()
		inputs := map[string]string{"email": email, "mobile_number": "+1999000111"}
		var step2 *common.FlowStep
		if onStart {
			step2, err = common.InitiateAuthenticationFlow(appID, false, inputs, "magic_link_action")
			ts.Require().NoError(err, "Failed to initiate the flow")
		} else {
			flowStep, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
			ts.Require().NoError(err, "Failed to initiate the flow")
			step2, err = common.CompleteFlow(flowStep.ExecutionID, inputs, "magic_link_action",
				flowStep.ChallengeToken)
			ts.Require().NoError(err, "Failed to submit the email")
		}
		ts.Require().Equal("INCOMPLETE", step2.FlowStatus)

		token := common.ExtractMagicLinkToken(ts.waitForEmail())
		ts.Require().NotEmpty(token, "Expected magic link token to be present in the email body")

		// The link is opened with only the token, as the gate sends it.
		step3, err := common.CompleteFlow(step2.ExecutionID, map[string]string{"token": token}, "", "")
		ts.Require().NoError(err, "Failed to submit the magic link token")
		ts.Equal("INCOMPLETE", step3.FlowStatus, "The mobile number should be prompted after the link")
		ts.True(common.HasInput(step3.Data.Inputs, "mobile_number"), "The mobile number input should be requested")

		user, err := testutils.FindUserByAttribute("email", email)
		ts.Require().NoError(err, "Failed to read the user back")
		ts.Require().NotNil(user, "The user should exist")
		attrs, err := testutils.GetUserAttributes(*user)
		ts.Require().NoError(err, "Failed to read the user's attributes")
		ts.Nil(attrs["mobile_number"], "The mobile number sent with the email should not be stored")
	}
}

// buildUsernameMagicLinkFlow builds a magic link authentication flow that identifies the user by username.
func buildUsernameMagicLinkFlow() testutils.Flow {
	return testutils.Flow{
		Name:     "Magic Link Auth Flow Username",
		FlowType: "AUTHENTICATION",
		Handle:   "auth_flow_magic_link_test_username",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "prompt_username"},
			{
				"id":   "prompt_username",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "magic_link_action", "nextNode": "send_magic_link"},
					},
				},
			},
			{
				"id":         "send_magic_link",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"magicLinkURL": "https://localhost:8095/gate/signin"},
				"executor": map[string]interface{}{
					"name": "MagicLinkExecutor",
					"mode": "generate",
					"inputs": []map[string]interface{}{
						{"ref": "input_001", "identifier": "username", "type": "TEXT_INPUT", "required": true},
					},
				},
				"onSuccess": "email_magic_link",
			},
			{
				"id":         "email_magic_link",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"emailTemplate": "MAGIC_LINK"},
				"executor": map[string]interface{}{
					"name": "EmailExecutor",
					"mode": "send",
					"inputs": []map[string]interface{}{
						{"identifier": "email", "type": "EMAIL_INPUT", "required": true},
					},
				},
				"onSuccess": "verify_magic_link",
			},
			{
				"id":        "verify_magic_link",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "MagicLinkExecutor", "mode": "verify"},
				"onSuccess": "auth_assert",
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

func (ts *magicLinkAuthFlowTestSuite) TestMagicLinkLoginFlow_NonExistingUser() {
	ts.mockSMTP.ClearEmails()

	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": "nobody@example.com"},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	// Anti-enumeration: must look like it succeeded
	ts.Require().Equal("INCOMPLETE", step2.FlowStatus)
	ts.Require().Equal("true", step2.Data.AdditionalData["emailSent"])
	ts.Require().True(common.HasInput(step2.Data.Inputs, "token"))

	// But no email was sent
	time.Sleep(500 * time.Millisecond)
	ts.Require().Nil(ts.mockSMTP.GetLastEmail(), "No email should be sent for a non-existent user")
}

func (ts *magicLinkAuthFlowTestSuite) TestMagicLinkLoginFlow_InvalidToken() {
	ts.mockSMTP.ClearEmails()

	flowStep, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": "userA@example.com"},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	time.Sleep(500 * time.Millisecond)

	// Submit an invalid token — server must reject with flowStatus=ERROR
	errStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"token": "invalid-token-123"}, "",
		step2.ChallengeToken)
	ts.Require().NoError(err, "Unexpected transport error submitting invalid magic link token")
	ts.Require().Equal("ERROR", errStep.FlowStatus, "Expected ERROR flow status for an invalid magic link token")
	ts.Require().NotNil(errStep.Error, "Expected an error in the flow response")
	ts.Require().NotEmpty(errStep.Error.Code, "Expected an error code in the error response")
}

func (ts *magicLinkAuthFlowTestSuite) TestMagicLinkLoginFlow_ExpiredToken() {
	ts.mockSMTP.ClearEmails()

	flowStep, err := common.InitiateAuthenticationFlow(ts.shortTTLAppID, false, nil, "")
	ts.Require().NoError(err)

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": "userA@example.com"},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	emailMessage := ts.waitForEmail()
	token := common.ExtractMagicLinkToken(emailMessage)
	ts.Require().NotEmpty(token)

	// Wait for TTL (2 seconds) plus leeway (1 second) to expire
	time.Sleep(5 * time.Second)

	// Submit the expired token — server must reject with flowStatus=ERROR
	errStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"token": token}, "",
		step2.ChallengeToken)
	ts.Require().NoError(err, "Unexpected transport error submitting expired magic link token")
	ts.Require().Equal("ERROR", errStep.FlowStatus, "Expected ERROR flow status for an expired magic link token")
	ts.Require().NotNil(errStep.Error, "Expected an error in the flow response")
	ts.Require().NotEmpty(errStep.Error.Code, "Expected an error code in the error response")
}

func (ts *magicLinkAuthFlowTestSuite) TestMagicLinkLoginFlow_ReusedToken() {
	ts.mockSMTP.ClearEmails()

	flowStep1, err := common.InitiateAuthenticationFlow(ts.reusedTokenAppID, false, nil, "")
	ts.Require().NoError(err)

	step2, err := common.CompleteFlow(flowStep1.ExecutionID, map[string]string{"email": "userA@example.com"},
		"magic_link_action", flowStep1.ChallengeToken)
	ts.Require().NoError(err)

	emailMessage := ts.waitForEmail()
	token := common.ExtractMagicLinkToken(emailMessage)
	ts.Require().NotEmpty(token, "Expected extracted magic-link token to be non-empty")

	step3, err := common.CompleteFlow(flowStep1.ExecutionID, map[string]string{"token": token}, "",
		step2.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", step3.FlowStatus)
	ts.Require().True(common.HasInput(step3.Data.Inputs, "dummy"), "Flow should have paused at dummy prompt")

	// Submit dummy prompt which loops back to verify_magic_link
	step4, err := common.CompleteFlow(flowStep1.ExecutionID, map[string]string{"dummy": "test"}, "dummy_action",
		step3.ChallengeToken)
	ts.Require().NoError(err)
	// Loop back to verify_magic_link replays the same jti — must be rejected with flowStatus=ERROR
	ts.Require().Equal("ERROR", step4.FlowStatus, "Expected ERROR flow status for jti replay")
	ts.Require().NotNil(step4.Error, "Expected an error in the flow response for jti replay")
	ts.Require().NotEmpty(step4.Error.Code, "Expected an error code in the error response")
}

func (ts *magicLinkAuthFlowTestSuite) TestMagicLinkLoginFlow_CrossFlowToken() {
	ts.mockSMTP.ClearEmails()

	// Execution A
	flowStepA, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	_, err = common.CompleteFlow(flowStepA.ExecutionID, map[string]string{"email": "userA@example.com"},
		"magic_link_action", flowStepA.ChallengeToken)
	ts.Require().NoError(err)

	emailMessageA := ts.waitForEmail()
	tokenA := common.ExtractMagicLinkToken(emailMessageA)
	ts.Require().NotEmpty(tokenA, "Expected extracted magic-link token to be non-empty")

	// Execution B
	ts.mockSMTP.ClearEmails()
	flowStepB, err := common.InitiateAuthenticationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	step2B, err := common.CompleteFlow(flowStepB.ExecutionID, map[string]string{"email": "userA@example.com"},
		"magic_link_action", flowStepB.ChallengeToken)
	ts.Require().NoError(err)

	// Submit Token A to Execution B — execution ID mismatch must be rejected with flowStatus=ERROR
	errStep, err := common.CompleteFlow(flowStepB.ExecutionID, map[string]string{"token": tokenA}, "",
		step2B.ChallengeToken)
	ts.Require().NoError(err, "Unexpected transport error submitting cross-flow magic link token")
	ts.Require().Equal("ERROR", errStep.FlowStatus, "Expected ERROR flow status for a cross-flow magic link token")
	ts.Require().NotNil(errStep.Error, "Expected an error in the flow response")
	ts.Require().NotEmpty(errStep.Error.Code, "Expected an error code in the error response")
}

func (ts *magicLinkAuthFlowTestSuite) TestPasswordMagicLink_SendsToStoredEmail() {
	appID := ts.createPasswordMagicLinkApp()
	username := ts.createPasswordUser(`"email": "stored@example.com"`)
	ts.mockSMTP.ClearEmails()

	step := ts.submitPassword(appID, username, map[string]string{"email": "other@example.com"})

	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	email := common.AwaitEmail(ts.Require(), ts.mockSMTP)
	ts.Equal([]string{"stored@example.com"}, email.To, "The magic link goes to the email on the user's profile")
	ts.Equal("stored@example.com", ts.storedAttribute(username, "email"), "The stored email is kept")

	finalStep, err := common.CompleteFlow(step.ExecutionID,
		map[string]string{"token": common.ExtractMagicLinkToken(email)}, "", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the magic link token")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus)

	claims, err := testutils.DecodeJWT(finalStep.Assertion)
	ts.Require().NoError(err, "Failed to decode the assertion")
	ts.Equal("stored@example.com", claims.Additional["email"], "The assertion carries the stored email")
}

func (ts *magicLinkAuthFlowTestSuite) TestPasswordMagicLink_UserWithoutEmailEnrollsOne() {
	appID := ts.createPasswordMagicLinkApp()
	username := ts.createPasswordUser("")
	ts.mockSMTP.ClearEmails()

	step := ts.submitPassword(appID, username, nil)
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	ts.Require().True(common.HasInput(step.Data.Inputs, "email"), "The email should be collected")

	_, err := common.CompleteFlow(step.ExecutionID, map[string]string{"email": "enrolled@example.com"}, "",
		step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the email")

	email := common.AwaitEmail(ts.Require(), ts.mockSMTP)
	ts.Equal([]string{"enrolled@example.com"}, email.To, "The magic link goes to the collected email")
	ts.Equal("enrolled@example.com", ts.storedAttribute(username, "email"), "The collected email is stored")
}

func (ts *magicLinkAuthFlowTestSuite) TestPasswordEmailOTP_EmailIsStoredOnlyAfterVerification() {
	appID := ts.createPasswordEmailOTPApp()
	username := ts.createPasswordUser("")

	step := ts.submitEmail(ts.submitPassword(appID, username, nil), "wrong@example.com")
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "The OTP should be requested")
	ts.Equal([]string{"wrong@example.com"}, common.AwaitEmail(ts.Require(), ts.mockSMTP).To)
	ts.Nil(ts.storedAttribute(username, "email"), "An unverified email isn't stored")

	step = ts.submitEmail(ts.submitPassword(appID, username, nil), "right@example.com")
	ts.Require().True(common.HasInput(step.Data.Inputs, "otp"), "The OTP should be requested")
	email := common.AwaitEmail(ts.Require(), ts.mockSMTP)
	ts.Equal([]string{"right@example.com"}, email.To)
	match := emailOTPRegex.FindStringSubmatch(email.Body)
	ts.Require().Len(match, 2, "The email should carry the OTP")

	finalStep, err := common.CompleteFlow(step.ExecutionID, map[string]string{"otp": match[1]}, "action_otp",
		step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the OTP")
	ts.Require().Equal("COMPLETE", finalStep.FlowStatus)
	ts.Equal("right@example.com", ts.storedAttribute(username, "email"), "The verified email is stored")
}

func (ts *magicLinkAuthFlowTestSuite) TestPasswordEmailOTP_StoredEmailSkipsTheEmailPrompt() {
	appID := ts.createPasswordEmailOTPApp()
	username := ts.createPasswordUser(`"email": "stored@example.com"`)
	ts.mockSMTP.ClearEmails()

	step := ts.submitPassword(appID, username, nil)

	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	ts.True(common.HasInput(step.Data.Inputs, "otp"), "The OTP should be requested")
	ts.Equal([]string{"stored@example.com"}, common.AwaitEmail(ts.Require(), ts.mockSMTP).To)
}

// createPasswordEmailOTPApp creates a password then email OTP flow and an application for it, deleted when the
// test ends.
func (ts *magicLinkAuthFlowTestSuite) createPasswordEmailOTPApp() string {
	flowID, err := testutils.CreateFlow(buildPasswordEmailOTPEnrollFlow())
	ts.Require().NoError(err, "Failed to create the password email OTP flow")
	ts.T().Cleanup(func() {
		_ = testutils.DeleteFlow(flowID)
	})

	app := magicLinkTestApp
	app.Name = "Magic Link Test App Password Email OTP"
	app.ClientID = "magic_link_test_client_password_email_otp"
	app.AuthFlowID = flowID
	app.OUID = ts.ouID
	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "Failed to create the password email OTP application")
	ts.T().Cleanup(func() {
		_ = testutils.DeleteApplication(appID)
	})
	return appID
}

// createPasswordMagicLinkApp creates a password then magic link flow and an application for it, deleted when the
// test ends.
func (ts *magicLinkAuthFlowTestSuite) createPasswordMagicLinkApp() string {
	flowID, err := testutils.CreateFlow(buildPasswordMagicLinkFlow())
	ts.Require().NoError(err, "Failed to create the password magic link flow")
	ts.T().Cleanup(func() {
		_ = testutils.DeleteFlow(flowID)
	})

	app := magicLinkTestApp
	app.Name = "Magic Link Test App Password"
	app.ClientID = "magic_link_test_client_password"
	app.AuthFlowID = flowID
	app.OUID = ts.ouID
	app.AssertionConfig = map[string]interface{}{"userAttributes": []string{"email"}}
	appID, err := testutils.CreateApplication(app)
	ts.Require().NoError(err, "Failed to create the password magic link application")
	ts.T().Cleanup(func() {
		_ = testutils.DeleteApplication(appID)
	})
	return appID
}

// createPasswordUser creates a user with a password and the given extra attributes, deleted when the test ends.
func (ts *magicLinkAuthFlowTestSuite) createPasswordUser(extraAttributes string) string {
	username := common.GenerateUniqueUsername("magiclinkmfa")
	attributes := `{"username": "` + username + `", "password": "testpassword"`
	if extraAttributes != "" {
		attributes += ", " + extraAttributes
	}
	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		OUID:       ts.ouID,
		Type:       magicLinkTestUserSchema.Name,
		Attributes: json.RawMessage(attributes + "}"),
	})
	ts.Require().NoError(err, "Failed to create the test user")
	ts.T().Cleanup(func() {
		if err := testutils.CleanupUsers(userIDs); err != nil {
			ts.T().Logf("Failed to delete the test user: %v", err)
		}
	})
	return username
}

// submitPassword starts a sign-in and submits the user's password along with any extra inputs.
func (ts *magicLinkAuthFlowTestSuite) submitPassword(appID, username string,
	extraInputs map[string]string) *common.FlowStep {
	step, err := common.InitiateAuthenticationFlow(appID, false, nil, "")
	ts.Require().NoError(err, "Failed to initiate authentication flow")

	inputs := map[string]string{"username": username, "password": "testpassword"}
	for key, value := range extraInputs {
		inputs[key] = value
	}
	step, err = common.CompleteFlow(step.ExecutionID, inputs, "", step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the password")
	return step
}

// submitEmail checks that the step asks for an email, submits the given one and returns the next step.
func (ts *magicLinkAuthFlowTestSuite) submitEmail(step *common.FlowStep, email string) *common.FlowStep {
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	ts.Require().True(common.HasInput(step.Data.Inputs, "email"), "The email should be collected")
	ts.mockSMTP.ClearEmails()

	step, err := common.CompleteFlow(step.ExecutionID, map[string]string{"email": email}, "action_email",
		step.ChallengeToken)
	ts.Require().NoError(err, "Failed to submit the email")
	ts.Require().Equal("INCOMPLETE", step.FlowStatus)
	return step
}

// storedAttribute reads the attribute on the user's profile.
func (ts *magicLinkAuthFlowTestSuite) storedAttribute(username, attribute string) interface{} {
	user, err := testutils.FindUserByAttribute("username", username)
	ts.Require().NoError(err, "Failed to read the user back")
	ts.Require().NotNil(user, "The user should exist")
	attrs, err := testutils.GetUserAttributes(*user)
	ts.Require().NoError(err, "Failed to read the user's attributes")
	return attrs[attribute]
}

// buildPasswordMagicLinkFlow builds a password then magic link flow, collecting an email from a user who has none.
func buildPasswordMagicLinkFlow() testutils.Flow {
	return testutils.Flow{
		Name:     "MFA Recipient Password Magic Link",
		FlowType: "AUTHENTICATION",
		Handle:   "mfa_recipient_password_magic_link",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "credentials_auth"},
			{
				"id":        "credentials_auth",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor"},
				"onSuccess": "attribute_collect",
			},
			{
				"id":   "attribute_collect",
				"type": "TASK_EXECUTION",
				"executor": map[string]interface{}{
					"name": "AttributeCollector",
					"inputs": []map[string]interface{}{
						{"ref": "input_email", "identifier": "email", "type": "EMAIL_INPUT", "required": true},
					},
				},
				"onSuccess": "send_magic_link",
			},
			{
				"id":         "send_magic_link",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"magicLinkURL": "https://localhost:8095/gate/signin"},
				"executor": map[string]interface{}{
					"name": "MagicLinkExecutor",
					"mode": "generate",
					"inputs": []map[string]interface{}{
						{"ref": "input_email", "identifier": "email", "type": "EMAIL_INPUT", "required": true},
					},
				},
				"onSuccess": "email_magic_link",
			},
			{
				"id":         "email_magic_link",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"emailTemplate": "MAGIC_LINK"},
				"executor": map[string]interface{}{
					"name": "EmailExecutor",
					"mode": "send",
					"inputs": []map[string]interface{}{
						{"ref": "input_email", "identifier": "email", "type": "EMAIL_INPUT", "required": true},
					},
				},
				"onSuccess": "verify_magic_link",
			},
			{
				"id":        "verify_magic_link",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "MagicLinkExecutor", "mode": "verify"},
				"onSuccess": "auth_assert",
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

// buildPasswordEmailOTPEnrollFlow builds a password then email OTP flow that stores a collected email only after
// the OTP is verified.
func buildPasswordEmailOTPEnrollFlow() testutils.Flow {
	emailInput := []map[string]interface{}{
		{"ref": "input_email", "identifier": "email", "type": "EMAIL_INPUT", "required": true},
	}
	return testutils.Flow{
		Name:     "Magic Link Test Password Email OTP",
		FlowType: "AUTHENTICATION",
		Handle:   "magic_link_test_password_email_otp",
		Nodes: []map[string]interface{}{
			{"id": "start", "type": "START", "onSuccess": "credentials_auth"},
			{
				"id":        "credentials_auth",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "CredentialsAuthExecutor"},
				"onSuccess": "attribute_collect",
			},
			{
				"id":           "attribute_collect",
				"type":         "TASK_EXECUTION",
				"executor":     map[string]interface{}{"name": "AttributeCollector", "inputs": emailInput},
				"onSuccess":    "generate_otp",
				"onIncomplete": "prompt_email",
			},
			{
				"id":   "prompt_email",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": emailInput,
						"action": map[string]interface{}{"ref": "action_email", "nextNode": "generate_otp"},
					},
				},
			},
			{
				"id":        "generate_otp",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "OTPExecutor", "mode": "generate", "inputs": emailInput},
				"onSuccess": "email_send",
			},
			{
				"id":         "email_send",
				"type":       "TASK_EXECUTION",
				"properties": map[string]interface{}{"emailTemplate": "OTP"},
				"executor":   map[string]interface{}{"name": "EmailExecutor", "mode": "send", "inputs": emailInput},
				"onSuccess":  "prompt_otp",
			},
			{
				"id":   "prompt_otp",
				"type": "PROMPT",
				"prompts": []map[string]interface{}{
					{
						"inputs": []map[string]interface{}{
							{"ref": "input_otp", "identifier": "otp", "type": "OTP_INPUT", "required": true},
						},
						"action": map[string]interface{}{"ref": "action_otp", "nextNode": "verify_otp"},
					},
				},
			},
			{
				"id":        "verify_otp",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "OTPExecutor", "mode": "verify"},
				"onSuccess": "save_email",
			},
			{
				"id":        "save_email",
				"type":      "TASK_EXECUTION",
				"executor":  map[string]interface{}{"name": "AttributeCollector", "inputs": emailInput},
				"onSuccess": "auth_assert",
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

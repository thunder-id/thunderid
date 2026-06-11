/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package authentication

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

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
		"email": map[string]interface{}{
			"type": "string",
		},
		"given_name": map[string]interface{}{
			"type": "string",
		},
		"family_name": map[string]interface{}{
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
	if os.Getenv("THUNDER_EXTRACTED_HOME") == "" {
		ts.T().Skip("Skipping magic link integration test - integration harness not initialized")
	}

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

	// Submit an invalid token — server must reject with 400
	errResp, err := common.CompleteAuthFlowWithError(flowStep.ExecutionID, map[string]string{"token": "invalid-token-123"},
		step2.ChallengeToken)
	ts.Require().NoError(err, "Unexpected transport error submitting invalid magic link token")
	ts.Require().NotNil(errResp, "Expected a 400 error response for an invalid magic link token")
	ts.Require().NotEmpty(errResp.Code, "Expected an error code in the error response")
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

	// Wait for TTL (2 seconds) to expire
	time.Sleep(3 * time.Second)

	// Submit the expired token — server must reject with 400
	errResp, err := common.CompleteAuthFlowWithError(flowStep.ExecutionID, map[string]string{"token": token},
		step2.ChallengeToken)
	ts.Require().NoError(err, "Unexpected transport error submitting expired magic link token")
	ts.Require().NotNil(errResp, "Expected a 400 error response for an expired magic link token")
	ts.Require().NotEmpty(errResp.Code, "Expected an error code in the error response")
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
	ts.Require().Equal("INCOMPLETE", step4.FlowStatus)
	ts.Require().True(common.HasInput(step4.Data.Inputs, "token"), "Flow should prompt for token again")

	// Try to submit the exact same token again — jti replay must be rejected with 400
	errResp, err := common.CompleteAuthFlowWithError(flowStep1.ExecutionID, map[string]string{"token": token},
		step4.ChallengeToken)
	ts.Require().NoError(err, "Unexpected transport error submitting reused magic link token")
	ts.Require().NotNil(errResp, "Expected a 400 error response for a reused magic link token")
	ts.Require().NotEmpty(errResp.Code, "Expected an error code in the error response")
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

	// Submit Token A to Execution B — execution ID mismatch must be rejected with 400
	errResp, err := common.CompleteAuthFlowWithError(flowStepB.ExecutionID, map[string]string{"token": tokenA},
		step2B.ChallengeToken)
	ts.Require().NoError(err, "Unexpected transport error submitting cross-flow magic link token")
	ts.Require().NotNil(errResp, "Expected a 400 error response for a cross-flow magic link token")
	ts.Require().NotEmpty(errResp.Code, "Expected an error code in the error response")
}

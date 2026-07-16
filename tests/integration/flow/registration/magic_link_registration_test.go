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

package registration

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/thunder-id/thunderid/tests/integration/flow/common"
	"github.com/thunder-id/thunderid/tests/integration/testutils"
)

var magicLinkRegistrationFlow = testutils.Flow{
	Name:     "Magic Link Registration Flow",
	FlowType: "REGISTRATION",
	Handle:   "reg_flow_magic_link_test",
	Nodes: []map[string]interface{}{
		{
			"id":        "start",
			"type":      "START",
			"onSuccess": "user_type_resolver",
		},
		{
			"id":   "user_type_resolver",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "UserTypeResolver",
			},
			"onSuccess":    "prompt_email",
			"onIncomplete": "prompt_email",
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
				"magicLinkURL": "https://localhost:8095/gate/signup",
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
						"ref":        "input_email",
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
			"onSuccess": "provisioning",
		},
		{
			"id":   "provisioning",
			"type": "TASK_EXECUTION",
			"executor": map[string]interface{}{
				"name": "ProvisioningExecutor",
			},
			"onSuccess":    "auth_assert",
			"onIncomplete": "prompt_schema_attrs",
		},
		{
			"id":   "prompt_schema_attrs",
			"type": "PROMPT",
			"prompts": []map[string]interface{}{
				{
					"inputs": []map[string]interface{}{},
					"action": map[string]interface{}{
						"ref":      "action_schema_attrs",
						"nextNode": "provisioning",
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
	},
}

var magicLinkRegTestApp = testutils.Application{
	Name:                      "Magic Link Registration Test Application",
	Description:               "Application for testing magic link registration",
	IsRegistrationFlowEnabled: true,
	ClientID:                  "magic_link_reg_test_client",
	ClientSecret:              "magic_link_reg_test_secret",
	RedirectURIs:              []string{"http://localhost:3000/callback"},
	AllowedUserTypes:          []string{"magic_link_reg_test_user"},
	AssertionConfig: map[string]interface{}{
		"userAttributes": []string{"userType", "ouId", "ouName", "ouHandle"},
	},
}

var magicLinkRegTestUserSchema = testutils.UserType{
	Name: "magic_link_reg_test_user",
	Schema: map[string]interface{}{
		"username": map[string]interface{}{
			"type": "string",
		},
		"email": map[string]interface{}{
			"type": "string",
		},
		"given_name": map[string]interface{}{
			"type":     "string",
			"required": true,
		},
		"family_name": map[string]interface{}{
			"type":     "string",
			"required": true,
		},
	},
	AllowSelfRegistration: true,
}

var magicLinkRegTestOU = testutils.OrganizationUnit{
	Handle:      "magic-link-reg-test-ou",
	Name:        "Magic Link Reg Test OU",
	Description: "Organization unit for magic link registration tests",
}

type MagicLinkRegistrationTestSuite struct {
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

func TestMagicLinkRegistrationTestSuite(t *testing.T) {
	suite.Run(t, new(MagicLinkRegistrationTestSuite))
}

func (ts *MagicLinkRegistrationTestSuite) SetupSuite() {
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

	ouID, err := testutils.CreateOrganizationUnit(magicLinkRegTestOU)
	ts.Require().NoError(err, "Failed to create test organization unit")
	ts.ouID = ouID

	magicLinkRegTestUserSchema.OUID = ts.ouID
	schemaID, err := testutils.CreateUserType(magicLinkRegTestUserSchema)
	ts.Require().NoError(err, "Failed to create test user schema")
	ts.userSchemaID = schemaID

	flowID, err := testutils.CreateFlow(magicLinkRegistrationFlow)
	ts.Require().NoError(err, "Failed to create magic link registration flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, flowID)
	magicLinkRegTestApp.RegistrationFlowID = flowID
	magicLinkRegTestApp.OUID = ts.ouID

	appID, err := testutils.CreateApplication(magicLinkRegTestApp)
	ts.Require().NoError(err, "Failed to create test application")
	ts.appID = appID

	// Build short TTL flow
	b, err := json.Marshal(magicLinkRegistrationFlow)
	ts.Require().NoError(err, "Failed to marshal magicLinkRegistrationFlow")
	var shortTTLFlow testutils.Flow
	err = json.Unmarshal(b, &shortTTLFlow)
	ts.Require().NoError(err, "Failed to unmarshal magicLinkRegistrationFlow")
	shortTTLFlow.Handle = "reg_flow_magic_link_test_short_ttl"
	shortTTLFlow.Name = "Magic Link Reg Flow Short TTL"

	ts.modifyFlowNode(&shortTTLFlow, "send_magic_link", func(node map[string]interface{}) {
		props, ok := node["properties"].(map[string]interface{})
		if !ok {
			props = make(map[string]interface{})
			node["properties"] = props
		}
		props["tokenExpiry"] = "2"
	})

	shortFlowID, err := testutils.CreateFlow(shortTTLFlow)
	ts.Require().NoError(err, "Failed to create short TTL magic link reg flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, shortFlowID)

	shortTTLApp := magicLinkRegTestApp
	shortTTLApp.Name = "Magic Link Reg Test App Short TTL"
	shortTTLApp.ClientID = "magic_link_reg_test_client_short_ttl"
	shortTTLApp.RegistrationFlowID = shortFlowID
	shortTTLApp.OUID = ts.ouID
	shortAppID, err := testutils.CreateApplication(shortTTLApp)
	ts.Require().NoError(err, "Failed to create short TTL test application")
	ts.shortTTLAppID = shortAppID

	// Build reused token flow (loops back to verify_magic_link)
	var reusedTokenFlow testutils.Flow
	err = json.Unmarshal(b, &reusedTokenFlow)
	ts.Require().NoError(err, "Failed to unmarshal reusedTokenFlow")
	reusedTokenFlow.Handle = "reg_flow_magic_link_test_reused"
	reusedTokenFlow.Name = "Magic Link Reg Flow Reused Token"

	ts.modifyFlowNode(&reusedTokenFlow, "verify_magic_link", func(node map[string]interface{}) {
		node["onSuccess"] = "dummy_prompt"
	})

	// Create a new slice to add the dummy prompt before provisioning
	var newNodes []interface{}
	reusedNodesList := reusedTokenFlow.Nodes.([]interface{})

	for _, n := range reusedNodesList {
		newNodes = append(newNodes, n)
		node, ok := n.(map[string]interface{})
		if ok && node["id"] == "verify_magic_link" {
			newNodes = append(newNodes, map[string]interface{}{
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
							"nextNode": "verify_magic_link", // loops back
						},
					},
					{
						"inputs": []map[string]interface{}{},
						"action": map[string]interface{}{
							"ref":      "dummy_action2",
							"nextNode": "provisioning",
						},
					},
				},
			})
		}
	}
	reusedTokenFlow.Nodes = newNodes

	reusedFlowID, err := testutils.CreateFlow(reusedTokenFlow)
	ts.Require().NoError(err, "Failed to create reused token magic link reg flow")
	ts.config.CreatedFlowIDs = append(ts.config.CreatedFlowIDs, reusedFlowID)

	reusedTokenApp := magicLinkRegTestApp
	reusedTokenApp.Name = "Magic Link Reg Test App Reused Token"
	reusedTokenApp.ClientID = "magic_link_reg_test_client_reused"
	reusedTokenApp.RegistrationFlowID = reusedFlowID
	reusedTokenApp.OUID = ts.ouID
	reusedAppID, err := testutils.CreateApplication(reusedTokenApp)
	ts.Require().NoError(err, "Failed to create reused token test application")
	ts.reusedTokenAppID = reusedAppID
}

func (ts *MagicLinkRegistrationTestSuite) TearDownSuite() {
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
func (ts *MagicLinkRegistrationTestSuite) modifyFlowNode(flow *testutils.Flow, nodeID string, modifier func(node map[string]interface{})) {
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
func (ts *MagicLinkRegistrationTestSuite) waitForEmail() *testutils.EmailMessage {
	var emailMessage *testutils.EmailMessage
	ts.Require().Eventually(func() bool {
		emailMessage = ts.mockSMTP.GetLastEmail()
		return emailMessage != nil
	}, 5*time.Second, 100*time.Millisecond, "Expected magic link email to be captured")
	return emailMessage
}

func (ts *MagicLinkRegistrationTestSuite) TestMagicLinkRegistration_Success() {
	ts.mockSMTP.ClearEmails()
	emailAddr := common.GenerateUniqueUsername("newuser") + "@example.com"

	flowStep, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": emailAddr},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	ts.Require().Equal("INCOMPLETE", step2.FlowStatus)
	ts.Require().Equal("true", step2.Data.AdditionalData["emailSent"])
	ts.Require().True(common.HasInput(step2.Data.Inputs, "token"))

	emailMessage := ts.waitForEmail()
	token := common.ExtractMagicLinkToken(emailMessage)
	ts.Require().NotEmpty(token)

	// Verify token
	step3, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"token": token}, "",
		step2.ChallengeToken)
	ts.Require().NoError(err)

	// Should prompt for given_name and family_name
	ts.Require().Equal("INCOMPLETE", step3.FlowStatus)
	ts.Require().True(common.HasInput(step3.Data.Inputs, "given_name"))
	ts.Require().True(common.HasInput(step3.Data.Inputs, "family_name"))

	inputs := map[string]string{
		"username":    common.GenerateUniqueUsername("username"),
		"given_name":  "Test",
		"family_name": "User",
	}
	step4, err := common.CompleteFlow(flowStep.ExecutionID, inputs, "action_schema_attrs",
		step3.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("COMPLETE", step4.FlowStatus)

	// Verify user was created
	user, err := testutils.FindUserByAttribute("email", emailAddr)
	ts.Require().NoError(err)
	ts.Require().NotNil(user)
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, user.ID)
}

func (ts *MagicLinkRegistrationTestSuite) TestMagicLinkRegistration_ExistingUser() {
	ts.mockSMTP.ClearEmails()

	// Create user
	emailAddr := common.GenerateUniqueUsername("existing") + "@example.com"
	existingUsername := common.GenerateUniqueUsername("existinguser")
	userIDs, err := testutils.CreateMultipleUsers(testutils.User{
		Type: magicLinkRegTestUserSchema.Name,
		OUID: ts.ouID,
		Attributes: json.RawMessage(`{
			"email": "` + emailAddr + `",
			"username": "` + existingUsername + `",
			"given_name": "Existing",
			"family_name": "User"
		}`),
	})
	ts.Require().NoError(err)
	ts.config.CreatedUserIDs = append(ts.config.CreatedUserIDs, userIDs...)

	flowStep, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": emailAddr},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	// Anti-enumeration: must look like it succeeded
	ts.Require().Equal("INCOMPLETE", step2.FlowStatus)
	ts.Require().Equal("true", step2.Data.AdditionalData["emailSent"])
	ts.Require().True(common.HasInput(step2.Data.Inputs, "token"))

	// But no email was sent
	time.Sleep(500 * time.Millisecond)
	ts.Require().Nil(ts.mockSMTP.GetLastEmail())
}

func (ts *MagicLinkRegistrationTestSuite) TestMagicLinkRegistration_InvalidToken() {
	ts.mockSMTP.ClearEmails()

	flowStep, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": "valid@example.com"},
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

func (ts *MagicLinkRegistrationTestSuite) TestMagicLinkRegistration_ExpiredToken() {
	ts.mockSMTP.ClearEmails()

	flowStep, err := common.InitiateRegistrationFlow(ts.shortTTLAppID, false, nil, "")
	ts.Require().NoError(err)

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": "valid@example.com"},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	emailMessage := ts.waitForEmail()
	token := common.ExtractMagicLinkToken(emailMessage)
	ts.Require().NotEmpty(token, "Expected extracted magic-link token to be non-empty")

	// Wait for TTL (2 seconds) plus leeway (1 second) to expire
	time.Sleep(4 * time.Second)

	// Submit the expired token — server must reject with flowStatus=ERROR
	errStep, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"token": token}, "",
		step2.ChallengeToken)
	ts.Require().NoError(err, "Unexpected transport error submitting expired magic link token")
	ts.Require().Equal("ERROR", errStep.FlowStatus, "Expected ERROR flow status for an expired magic link token")
	ts.Require().NotNil(errStep.Error, "Expected an error in the flow response")
	ts.Require().NotEmpty(errStep.Error.Code, "Expected an error code in the error response")
}

func (ts *MagicLinkRegistrationTestSuite) TestMagicLinkRegistration_ReusedToken() {
	ts.mockSMTP.ClearEmails()

	flowStep, err := common.InitiateRegistrationFlow(ts.reusedTokenAppID, false, nil, "")
	ts.Require().NoError(err)

	step2, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"email": "reused@example.com"},
		"magic_link_action", flowStep.ChallengeToken)
	ts.Require().NoError(err)

	emailMessage := ts.waitForEmail()
	token := common.ExtractMagicLinkToken(emailMessage)
	ts.Require().NotEmpty(token, "Expected extracted magic-link token to be non-empty")

	step3, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"token": token}, "",
		step2.ChallengeToken)
	ts.Require().NoError(err)
	ts.Require().Equal("INCOMPLETE", step3.FlowStatus)
	ts.Require().True(common.HasInput(step3.Data.Inputs, "dummy"))

	step4, err := common.CompleteFlow(flowStep.ExecutionID, map[string]string{"dummy": "test"}, "dummy_action",
		step3.ChallengeToken)
	ts.Require().NoError(err)
	// Loop back to verify_magic_link replays the same jti — must be rejected with flowStatus=ERROR
	ts.Require().Equal("ERROR", step4.FlowStatus, "Expected ERROR flow status for a reused magic link token")
	ts.Require().NotNil(step4.Error, "Expected an error in the flow response")
	ts.Require().NotEmpty(step4.Error.Code, "Expected an error code in the error response")
}

func (ts *MagicLinkRegistrationTestSuite) TestMagicLinkRegistration_CrossFlowToken() {
	ts.mockSMTP.ClearEmails()

	// Execution A
	flowStepA, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	_, err = common.CompleteFlow(flowStepA.ExecutionID, map[string]string{"email": "cross@example.com"},
		"magic_link_action", flowStepA.ChallengeToken)
	ts.Require().NoError(err)

	emailMessageA := ts.waitForEmail()
	tokenA := common.ExtractMagicLinkToken(emailMessageA)
	ts.Require().NotEmpty(tokenA, "Expected extracted magic-link token to be non-empty")

	// Execution B
	ts.mockSMTP.ClearEmails()
	flowStepB, err := common.InitiateRegistrationFlow(ts.appID, false, nil, "")
	ts.Require().NoError(err)

	step2B, err := common.CompleteFlow(flowStepB.ExecutionID, map[string]string{"email": "cross2@example.com"},
		"magic_link_action", flowStepB.ChallengeToken)
	ts.Require().NoError(err)

	// Attempt to use flow A's token in flow B — server must reject with flowStatus=ERROR
	errStep, err := common.CompleteFlow(flowStepB.ExecutionID, map[string]string{"token": tokenA}, "",
		step2B.ChallengeToken)
	ts.Require().NoError(err, "Unexpected transport error submitting cross-flow magic link token")
	ts.Require().Equal("ERROR", errStep.FlowStatus, "Expected ERROR flow status for a cross-flow magic link token")
	ts.Require().NotNil(errStep.Error, "Expected an error in the flow response")
	ts.Require().NotEmpty(errStep.Error.Code, "Expected an error code in the error response")
}

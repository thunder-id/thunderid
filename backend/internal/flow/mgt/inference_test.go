/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package flowmgt

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/executor"
)

type FlowInferenceServiceTestSuite struct {
	suite.Suite
	service flowInferenceServiceInterface
}

func TestFlowInferenceServiceTestSuite(t *testing.T) {
	suite.Run(t, new(FlowInferenceServiceTestSuite))
}

func (s *FlowInferenceServiceTestSuite) SetupTest() {
	s.service = newFlowInferenceService()
}

// Test InferRegistrationFlow

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_Success() {
	authFlow := &FlowDefinition{
		Handle:   "basic-auth-handle",
		Name:     "Basic Authentication",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "prompt"},
			{ID: "prompt", Type: "PROMPT", OnSuccess: "auth"},
			{
				ID:   "auth",
				Type: "TASK_EXECUTION",
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameBasicAuth,
				},
				OnSuccess: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.NoError(err)
	s.NotNil(regFlow)
	s.Equal("Basic Registration", regFlow.Name)
	s.Equal(common.FlowTypeRegistration, regFlow.FlowType)
	s.Equal(authFlow.Handle, regFlow.Handle)

	// Verify provisioning node was inserted
	s.True(s.hasNode(regFlow.Nodes, provisioningNodeID))
	provNode := s.getNode(regFlow.Nodes, provisioningNodeID)
	s.Equal(executor.ExecutorNameProvisioning, provNode.Executor.Name)
	s.Equal("end", provNode.OnSuccess)
	// No layout should be added since source flow has no layout
	s.Nil(provNode.Layout)

	// Verify user type resolver was inserted
	s.True(s.hasNode(regFlow.Nodes, userTypeResolverNodeID))
	resolverNode := s.getNode(regFlow.Nodes, userTypeResolverNodeID)
	s.Equal(executor.ExecutorNameUserTypeResolver, resolverNode.Executor.Name)
	// No layout should be added since source flow has no layout
	s.Nil(resolverNode.Layout)

	// Verify start node points to user type resolver
	startNode := s.getNode(regFlow.Nodes, "start")
	s.Equal(userTypeResolverNodeID, startNode.OnSuccess)
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_WithAuthAssert() {
	authFlow := &FlowDefinition{
		Handle:   "sms-auth-handle",
		Name:     "SMS Authentication",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "prompt"},
			{ID: "prompt", Type: "PROMPT", OnSuccess: "auth"},
			{
				ID:   "auth",
				Type: "TASK_EXECUTION",
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameSMSAuth,
					Mode: executor.ExecutorModeSend,
				},
				OnSuccess: "auth_assert",
			},
			{
				ID:   "auth_assert",
				Type: "TASK_EXECUTION",
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameAuthAssert,
				},
				OnSuccess: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.NoError(err)
	s.NotNil(regFlow)
	s.Equal("SMS Registration", regFlow.Name)
	s.Equal(common.FlowTypeRegistration, regFlow.FlowType)

	// Verify provisioning node was inserted BEFORE AuthAssertExecutor
	s.True(s.hasNode(regFlow.Nodes, provisioningNodeID))
	provNode := s.getNode(regFlow.Nodes, provisioningNodeID)
	s.Equal(executor.ExecutorNameProvisioning, provNode.Executor.Name)
	s.Equal("auth_assert", provNode.OnSuccess, "Provisioning should point to AuthAssert")

	// Verify auth node now points to provisioning instead of auth_assert
	authNode := s.getNode(regFlow.Nodes, "auth")
	s.Equal(provisioningNodeID, authNode.OnSuccess, "Auth node should point to provisioning")

	// Verify auth_assert still points to end
	authAssertNode := s.getNode(regFlow.Nodes, "auth_assert")
	s.Equal("end", authAssertNode.OnSuccess)

	// Verify user type resolver was inserted
	s.True(s.hasNode(regFlow.Nodes, userTypeResolverNodeID))
	resolverNode := s.getNode(regFlow.Nodes, userTypeResolverNodeID)
	s.Equal(executor.ExecutorNameUserTypeResolver, resolverNode.Executor.Name)

	// Verify phone input prompt was inserted before SMS send node
	s.True(s.hasNode(regFlow.Nodes, phoneInputPromptNodeID))
	phonePromptNode := s.getNode(regFlow.Nodes, phoneInputPromptNodeID)
	s.Equal(string(common.NodeTypePrompt), phonePromptNode.Type)
	s.Equal("auth", phonePromptNode.Prompts[0].Action.NextNode, "Phone prompt should point to SMS send node")
	s.Len(phonePromptNode.Prompts[0].Inputs, 1)
	s.Equal(common.InputTypePhone, phonePromptNode.Prompts[0].Inputs[0].Type)
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_WithAuthAssertAndMultiplePaths() {
	authFlow := &FlowDefinition{
		Handle:   "multi-path-auth",
		Name:     "Multi-Path Authentication",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "decision"},
			{
				ID:   "decision",
				Type: "DECISION",
				Prompts: []PromptDefinition{
					{Action: &ActionDefinition{Ref: "path1", NextNode: "auth1"}},
					{Action: &ActionDefinition{Ref: "path2", NextNode: "auth2"}},
				},
			},
			{
				ID:   "auth1",
				Type: "TASK_EXECUTION",
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameBasicAuth,
				},
				OnSuccess: "auth_assert",
			},
			{
				ID:   "auth2",
				Type: "TASK_EXECUTION",
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameSMSAuth,
				},
				OnSuccess: "auth_assert",
			},
			{
				ID:   "auth_assert",
				Type: "TASK_EXECUTION",
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameAuthAssert,
				},
				OnSuccess: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.NoError(err)
	s.NotNil(regFlow)

	// Verify provisioning node was inserted before AuthAssert
	s.True(s.hasNode(regFlow.Nodes, provisioningNodeID))
	provNode := s.getNode(regFlow.Nodes, provisioningNodeID)
	s.Equal("auth_assert", provNode.OnSuccess)

	// Verify both auth nodes now point to provisioning
	auth1Node := s.getNode(regFlow.Nodes, "auth1")
	s.Equal(provisioningNodeID, auth1Node.OnSuccess)

	auth2Node := s.getNode(regFlow.Nodes, "auth2")
	s.Equal(provisioningNodeID, auth2Node.OnSuccess)
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_WithExistingProvisioning() {
	authFlow := &FlowDefinition{
		Handle:   "auth-flow-handle",
		Name:     "Auth Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "task"},
			{
				ID:   "task",
				Type: "TASK_EXECUTION",
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameProvisioning,
				},
				OnSuccess: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.NoError(err)
	s.NotNil(regFlow)
	// Should have 5 nodes (start, user-type-resolver, ut_prompt_node, task, end) - no duplicate provisioning
	s.Len(regFlow.Nodes, 5)
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_WithExistingUserTypeResolver() {
	authFlow := &FlowDefinition{
		Handle:   "auth-flow-handle",
		Name:     "Auth Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "resolver"},
			{
				ID:   "resolver",
				Type: "TASK_EXECUTION",
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameUserTypeResolver,
				},
				OnSuccess: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.NoError(err)
	s.NotNil(regFlow)
	// Should have 4 nodes (start, resolver, provisioning, end) - no duplicate user type resolver
	s.Len(regFlow.Nodes, 4)
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_CleansAuthProperties() {
	authFlow := &FlowDefinition{
		Handle:   "auth-flow-handle",
		Name:     "Auth Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "task"},
			{
				ID:   "task",
				Type: "TASK_EXECUTION",
				Properties: map[string]interface{}{
					common.NodePropertyAllowAuthenticationWithoutLocalUser: true,
					"otherProp": "value",
				},
				OnSuccess: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.NoError(err)
	taskNode := s.getNode(regFlow.Nodes, "task")
	s.NotNil(taskNode.Properties)
	_, hasAuthProp := taskNode.Properties[common.NodePropertyAllowAuthenticationWithoutLocalUser]
	s.False(hasAuthProp)
	s.Equal("value", taskNode.Properties["otherProp"])
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_NoStartNode() {
	authFlow := &FlowDefinition{
		Handle:   "invalid-flow-handle",
		Name:     "Invalid Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "task", Type: "TASK_EXECUTION", OnSuccess: "end"},
			{ID: "end", Type: "END"},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.Error(err)
	s.Nil(regFlow)
	s.Contains(err.Error(), "no START node found")
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_NoEndNode() {
	authFlow := &FlowDefinition{
		Handle:   "invalid-flow-handle",
		Name:     "Invalid Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "task"},
			{ID: "task", Type: "TASK_EXECUTION"},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.Error(err)
	s.Nil(regFlow)
	s.Contains(err.Error(), "no END node found")
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_WithActions() {
	authFlow := &FlowDefinition{
		Handle:   "auth-flow-handle",
		Name:     "Auth Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "prompt"},
			{
				ID:   "prompt",
				Type: "PROMPT",
				Prompts: []PromptDefinition{
					{Action: &ActionDefinition{Ref: "login", NextNode: "end"}},
					{Action: &ActionDefinition{Ref: "signup", NextNode: "end"}},
				},
			},
			{ID: "end", Type: "END"},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.NoError(err)
	promptNode := s.getNode(regFlow.Nodes, "prompt")
	s.Len(promptNode.Prompts, 2)
	// Actions should now point to provisioning node instead of end (since no AuthAssert exists)
	s.Equal(provisioningNodeID, promptNode.Prompts[0].Action.NextNode)
	s.Equal(provisioningNodeID, promptNode.Prompts[1].Action.NextNode)
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_WithOnFailure() {
	authFlow := &FlowDefinition{
		Handle:   "auth-flow-handle",
		Name:     "Auth Flow",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "task"},
			{
				ID:        "task",
				Type:      "TASK_EXECUTION",
				OnSuccess: "end",
				OnFailure: "end",
			},
			{ID: "end", Type: "END"},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.NoError(err)
	taskNode := s.getNode(regFlow.Nodes, "task")
	// OnSuccess should point to provisioning (since no AuthAssert exists)
	s.Equal(provisioningNodeID, taskNode.OnSuccess)
	// OnFailure should also point to provisioning (was pointing to end, no AuthAssert exists)
	s.Equal(provisioningNodeID, taskNode.OnFailure)
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_WithLayout() {
	authFlow := &FlowDefinition{
		Name:     "Basic Authentication",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{
				ID:   "start",
				Type: "START",
				Layout: &NodeLayout{
					Size:     &NodeSize{Width: 180, Height: 80},
					Position: &NodePosition{X: 50, Y: 50},
				},
				OnSuccess: "prompt",
			},
			{
				ID:   "prompt",
				Type: "PROMPT",
				Layout: &NodeLayout{
					Size:     &NodeSize{Width: 320, Height: 200},
					Position: &NodePosition{X: 300, Y: 50},
				},
				OnSuccess: "auth",
			},
			{
				ID:   "auth",
				Type: "TASK_EXECUTION",
				Layout: &NodeLayout{
					Size:     &NodeSize{Width: 200, Height: 120},
					Position: &NodePosition{X: 700, Y: 50},
				},
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameBasicAuth,
				},
				OnSuccess: "end",
			},
			{
				ID:   "end",
				Type: "END",
				Layout: &NodeLayout{
					Size:     &NodeSize{Width: 180, Height: 80},
					Position: &NodePosition{X: 1000, Y: 50},
				},
			},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.NoError(err)
	s.NotNil(regFlow)

	// Verify provisioning node has layout since source flow has layout
	s.True(s.hasNode(regFlow.Nodes, provisioningNodeID))
	provNode := s.getNode(regFlow.Nodes, provisioningNodeID)
	s.NotNil(provNode.Layout, "Provisioning node should have layout when source flow has layout")
	s.NotNil(provNode.Layout.Size)
	s.Equal(float64(100), provNode.Layout.Size.Width)
	s.Equal(float64(120), provNode.Layout.Size.Height)
	s.NotNil(provNode.Layout.Position)
	s.Equal(float64(0), provNode.Layout.Position.X)
	s.Equal(float64(0), provNode.Layout.Position.Y)

	// Verify user type resolver has layout since source flow has layout
	s.True(s.hasNode(regFlow.Nodes, userTypeResolverNodeID))
	resolverNode := s.getNode(regFlow.Nodes, userTypeResolverNodeID)
	s.NotNil(resolverNode.Layout, "User type resolver node should have layout when source flow has layout")
	s.NotNil(resolverNode.Layout.Size)
	s.Equal(float64(100), resolverNode.Layout.Size.Width)
	s.Equal(float64(120), resolverNode.Layout.Size.Height)
	s.NotNil(resolverNode.Layout.Position)
	s.Equal(float64(0), resolverNode.Layout.Position.X)
	s.Equal(float64(0), resolverNode.Layout.Position.Y)

	// Verify original nodes still have their layout preserved
	startNode := s.getNode(regFlow.Nodes, "start")
	s.NotNil(startNode.Layout)
	s.Equal(float64(180), startNode.Layout.Size.Width)
	s.Equal(float64(50), startNode.Layout.Position.X)
}

// Test insertPhoneInputPromptIfNeeded

func (s *FlowInferenceServiceTestSuite) TestInsertPhoneInputPromptIfNeeded_NoSMSSendNode() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "task"},
		{
			ID:   "task",
			Type: "TASK_EXECUTION",
			Executor: &ExecutorDefinition{
				Name: executor.ExecutorNameBasicAuth,
			},
			OnSuccess: "end",
		},
		{ID: "end", Type: "END"},
	}
	initialCount := len(nodes)

	service.insertPhoneInputPromptIfNeeded(&nodes, false)

	s.Len(nodes, initialCount, "No node should be inserted when there is no SMS OTP send node")
}

func (s *FlowInferenceServiceTestSuite) TestInsertPhoneInputPromptIfNeeded_SMSNodeNotSendMode() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "sms"},
		{
			ID:   "sms",
			Type: "TASK_EXECUTION",
			Executor: &ExecutorDefinition{
				Name: executor.ExecutorNameSMSAuth,
				Mode: executor.ExecutorModeVerify,
			},
			OnSuccess: "end",
		},
		{ID: "end", Type: "END"},
	}
	initialCount := len(nodes)

	service.insertPhoneInputPromptIfNeeded(&nodes, false)

	s.Len(nodes, initialCount, "No node should be inserted for SMS verify mode")
}

func (s *FlowInferenceServiceTestSuite) TestInsertPhoneInputPromptIfNeeded_PhoneInputAlreadyCollected() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "phone_prompt"},
		{
			ID:   "phone_prompt",
			Type: string(common.NodeTypePrompt),
			Prompts: []PromptDefinition{
				{
					Inputs: []InputDefinition{
						{Identifier: "mobileNumber", Type: common.InputTypePhone, Required: true},
					},
					Action: &ActionDefinition{NextNode: "sms"},
				},
			},
			OnSuccess: "sms",
		},
		{
			ID:   "sms",
			Type: "TASK_EXECUTION",
			Executor: &ExecutorDefinition{
				Name: executor.ExecutorNameSMSAuth,
				Mode: executor.ExecutorModeSend,
			},
			OnSuccess: "end",
		},
		{ID: "end", Type: "END"},
	}
	initialCount := len(nodes)

	service.insertPhoneInputPromptIfNeeded(&nodes, false)

	s.Len(nodes, initialCount, "No node should be inserted when PHONE_INPUT is already collected")
}

func (s *FlowInferenceServiceTestSuite) TestInsertPhoneInputPromptIfNeeded_InsertsPromptBeforeSMSSend() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "sms"},
		{
			ID:   "sms",
			Type: "TASK_EXECUTION",
			Executor: &ExecutorDefinition{
				Name: executor.ExecutorNameSMSAuth,
				Mode: executor.ExecutorModeSend,
			},
			OnSuccess: "end",
		},
		{ID: "end", Type: "END"},
	}

	service.insertPhoneInputPromptIfNeeded(&nodes, false)

	s.Len(nodes, 4, "Phone prompt node should be inserted")

	// Verify the prompt node exists with correct type and input
	s.True(s.hasNode(nodes, phoneInputPromptNodeID))
	phonePrompt := s.getNode(nodes, phoneInputPromptNodeID)
	s.Equal(string(common.NodeTypePrompt), phonePrompt.Type)
	s.Len(phonePrompt.Prompts, 1)
	s.Len(phonePrompt.Prompts[0].Inputs, 1)
	s.Equal(common.InputTypePhone, phonePrompt.Prompts[0].Inputs[0].Type)
	s.Equal("sms", phonePrompt.Prompts[0].Action.NextNode, "Phone prompt should point to SMS send node")
	s.Nil(phonePrompt.Layout, "Layout should not be added when includeLayout is false")

	// Verify START now points to phone prompt instead of SMS node
	startNode := s.getNode(nodes, "start")
	s.Equal(phoneInputPromptNodeID, startNode.OnSuccess)
}

func (s *FlowInferenceServiceTestSuite) TestInsertPhoneInputPromptIfNeeded_InsertsPromptWithLayout() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "sms"},
		{
			ID:   "sms",
			Type: "TASK_EXECUTION",
			Executor: &ExecutorDefinition{
				Name: executor.ExecutorNameSMSAuth,
				Mode: executor.ExecutorModeSend,
			},
			OnSuccess: "end",
		},
		{ID: "end", Type: "END"},
	}

	service.insertPhoneInputPromptIfNeeded(&nodes, true)

	phonePrompt := s.getNode(nodes, phoneInputPromptNodeID)
	s.NotNil(phonePrompt)
	s.NotNil(phonePrompt.Layout, "Layout should be added when includeLayout is true")
	s.NotNil(phonePrompt.Layout.Size)
	s.NotNil(phonePrompt.Layout.Position)
}

func (s *FlowInferenceServiceTestSuite) TestInsertPhoneInputPromptIfNeeded_UsesExecutorInputIdentifier() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "sms"},
		{
			ID:   "sms",
			Type: "TASK_EXECUTION",
			Executor: &ExecutorDefinition{
				Name: executor.ExecutorNameSMSAuth,
				Mode: executor.ExecutorModeSend,
				Inputs: []InputDefinition{
					{Ref: "phone_input_dvq8", Identifier: "mobile", Type: common.InputTypePhone, Required: true},
				},
			},
			OnSuccess: "end",
		},
		{ID: "end", Type: "END"},
	}

	service.insertPhoneInputPromptIfNeeded(&nodes, false)

	s.Len(nodes, 4, "Phone prompt node should be inserted")

	phonePrompt := s.getNode(nodes, phoneInputPromptNodeID)
	s.Len(phonePrompt.Prompts[0].Inputs, 1)
	s.Equal("mobile", phonePrompt.Prompts[0].Inputs[0].Identifier,
		"Inserted prompt should use identifier from executor inputs")
	s.Equal("phone_input_dvq8", phonePrompt.Prompts[0].Inputs[0].Ref,
		"Inserted prompt should use ref from executor inputs")
}

// Test generateRegistrationFlowName

func (s *FlowInferenceServiceTestSuite) TestGenerateRegistrationFlowName_Authentication() {
	service := s.service.(*flowInferenceService)

	result := service.generateRegistrationFlowName("Basic Authentication")
	s.Equal("Basic Registration", result)
}

func (s *FlowInferenceServiceTestSuite) TestGenerateRegistrationFlowName_Authenticate() {
	service := s.service.(*flowInferenceService)

	result := service.generateRegistrationFlowName("Basic Authenticate")
	s.Equal("Basic Registration", result)
}

func (s *FlowInferenceServiceTestSuite) TestGenerateRegistrationFlowName_Login() {
	service := s.service.(*flowInferenceService)

	result := service.generateRegistrationFlowName("Login Flow")
	s.Equal("Registration Flow", result)
}

func (s *FlowInferenceServiceTestSuite) TestGenerateRegistrationFlowName_SignIn() {
	service := s.service.(*flowInferenceService)

	result := service.generateRegistrationFlowName("Sign-in Flow")
	s.Equal("Registration Flow", result)
}

func (s *FlowInferenceServiceTestSuite) TestGenerateRegistrationFlowName_Signin() {
	service := s.service.(*flowInferenceService)

	result := service.generateRegistrationFlowName("Signin Flow")
	s.Equal("Registration Flow", result)
}

func (s *FlowInferenceServiceTestSuite) TestGenerateRegistrationFlowName_SignInWithSpace() {
	service := s.service.(*flowInferenceService)

	result := service.generateRegistrationFlowName("Sign in Flow")
	s.Equal("Registration Flow", result)
}

func (s *FlowInferenceServiceTestSuite) TestGenerateRegistrationFlowName_Auth() {
	service := s.service.(*flowInferenceService)

	result := service.generateRegistrationFlowName("Basic Auth")
	s.Equal("Basic Registration", result)
}

func (s *FlowInferenceServiceTestSuite) TestGenerateRegistrationFlowName_NoAuthTerm() {
	service := s.service.(*flowInferenceService)

	result := service.generateRegistrationFlowName("Custom Flow")
	s.Equal("Custom Flow - Registration", result)
}

func (s *FlowInferenceServiceTestSuite) TestGenerateRegistrationFlowName_CaseInsensitive() {
	service := s.service.(*flowInferenceService)

	result := service.generateRegistrationFlowName("basic authentication")
	s.Equal("basic Registration", result)
}

// Test replaceAuthLabel

func (s *FlowInferenceServiceTestSuite) TestReplaceAuthLabel_ReplacesKnownTerms() {
	cases := []struct {
		input    string
		expected string
	}{
		{"Sign In", "Sign Up"},
		{"Sign in", "Sign Up"},
		{"Sign-in", "Sign-up"},
		{"Login", "Register"},
		{"Log In", "Register"},
		{"Log in", "Register"},
		{"Authenticate", "Register"},
		{"Authentication", "Registration"},
	}

	for _, tc := range cases {
		result, replaced := replaceAuthLabel(tc.input)
		s.True(replaced, "expected replacement for %q", tc.input)
		s.Equal(tc.expected, result)
	}
}

func (s *FlowInferenceServiceTestSuite) TestReplaceAuthLabel_NoMatchReturnsEmpty() {
	result, replaced := replaceAuthLabel("Continue")
	s.False(replaced)
	s.Empty(result)
}

// Test cloneNodes

func (s *FlowInferenceServiceTestSuite) TestCloneNodes_Success() {
	service := s.service.(*flowInferenceService)
	original := []NodeDefinition{
		{ID: "node1", Type: "START", OnSuccess: "node2"},
		{
			ID:   "node2",
			Type: "TASK_EXECUTION",
			Properties: map[string]interface{}{
				"key": "value",
			},
		},
	}

	cloned, err := service.cloneNodes(original)

	s.NoError(err)
	s.Len(cloned, 2)
	s.Equal(original[0].ID, cloned[0].ID)

	// Verify it's a deep copy by modifying clone
	cloned[0].ID = "modified"
	s.NotEqual(original[0].ID, cloned[0].ID)
}

func (s *FlowInferenceServiceTestSuite) TestCloneNodes_EmptyArray() {
	service := s.service.(*flowInferenceService)

	cloned, err := service.cloneNodes([]NodeDefinition{})

	s.NoError(err)
	s.Empty(cloned)
}

// Test cleanAuthenticationProperties

func (s *FlowInferenceServiceTestSuite) TestCleanAuthenticationProperties_RemovesAuthProp() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{
			ID:   "node1",
			Type: "TASK_EXECUTION",
			Properties: map[string]interface{}{
				common.NodePropertyAllowAuthenticationWithoutLocalUser: true,
				"otherProp": "value",
			},
		},
	}

	service.cleanAuthenticationProperties(nodes)

	_, hasAuthProp := nodes[0].Properties[common.NodePropertyAllowAuthenticationWithoutLocalUser]
	s.False(hasAuthProp)
	s.Equal("value", nodes[0].Properties["otherProp"])
}

func (s *FlowInferenceServiceTestSuite) TestCleanAuthenticationProperties_NilProperties() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "node1", Type: "TASK_EXECUTION"},
	}

	// Should not panic
	s.NotPanics(func() {
		service.cleanAuthenticationProperties(nodes)
	})
}

func (s *FlowInferenceServiceTestSuite) TestCleanAuthProperties_PromptNode_UpdatesLabelsAndRemovesSignUpLink() {
	service := s.service.(*flowInferenceService)
	signUpLinkLabel := `<p class="rich-text-paragraph">` +
		`<span class="rich-text-pre-wrap">Don't have an account? </span>` +
		`<a href="{{meta(application.sign_up_url)}}" target="_blank"` +
		` rel="noopener noreferrer" class="rich-text-link">` +
		`<span class="rich-text-pre-wrap">Sign up</span></a></p>`
	submitBtn := map[string]interface{}{
		"type":      "ACTION",
		"id":        "action_1b71",
		"eventType": "SUBMIT",
		"label":     "Sign In",
	}
	signUpComp := map[string]interface{}{
		"category": "DISPLAY",
		"type":     "RICH_TEXT",
		"id":       "rich_text_p6ae",
		"label":    signUpLinkLabel,
	}
	nodes := []NodeDefinition{
		{
			ID:   "prompt1",
			Type: string(common.NodeTypePrompt),
			Meta: map[string]interface{}{
				"components": []interface{}{
					// Use a random-style ID to confirm matching is by content, not ID
					map[string]interface{}{
						"type":    "TEXT",
						"id":      "text_hexl",
						"label":   "Sign In",
						"variant": "HEADING_3",
					},
					map[string]interface{}{
						"type": "BLOCK",
						"id":   "block_ms6e",
						"components": []interface{}{
							map[string]interface{}{"type": "TEXT_INPUT", "id": "input_username"},
							submitBtn,
							signUpComp,
						},
					},
				},
			},
		},
	}

	service.cleanAuthenticationProperties(nodes)

	meta := nodes[0].Meta.(map[string]interface{})
	components := meta["components"].([]interface{})

	// Heading should be updated to "Sign Up"
	heading := components[0].(map[string]interface{})
	s.Equal("Sign Up", heading["label"])

	// RICH_TEXT sign-up link should be removed from block (matched by label content, not ID)
	block := components[1].(map[string]interface{})
	blockComponents := block["components"].([]interface{})
	s.Len(blockComponents, 2)
	for _, bc := range blockComponents {
		bcMap := bc.(map[string]interface{})
		s.NotEqual("RICH_TEXT", bcMap["type"])
	}

	// SUBMIT action button label should be renamed to "Sign Up"
	var actionComp map[string]interface{}
	for _, bc := range blockComponents {
		bcMap := bc.(map[string]interface{})
		if bcMap["type"] == "ACTION" && bcMap["eventType"] == "SUBMIT" {
			actionComp = bcMap
			break
		}
	}
	s.NotNil(actionComp)
	s.Equal("Sign Up", actionComp["label"])
}

func (s *FlowInferenceServiceTestSuite) TestCleanAuthenticationProperties_PromptNode_NoSignUpLink() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{
			ID:   "prompt1",
			Type: string(common.NodeTypePrompt),
			Meta: map[string]interface{}{
				"components": []interface{}{
					map[string]interface{}{
						"type": "BLOCK",
						"id":   "block_basic",
						"components": []interface{}{
							map[string]interface{}{"type": "TEXT_INPUT", "id": "input_username"},
						},
					},
				},
			},
		},
	}

	// Should not panic when there is no self_sign_up_link
	s.NotPanics(func() {
		service.cleanAuthenticationProperties(nodes)
	})

	meta := nodes[0].Meta.(map[string]interface{})
	components := meta["components"].([]interface{})
	block := components[0].(map[string]interface{})
	s.Len(block["components"].([]interface{}), 1)
}

// Test findStartNode

func (s *FlowInferenceServiceTestSuite) TestFindStartNode_Success() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "task", Type: "TASK_EXECUTION"},
		{ID: "start", Type: "START"},
	}

	id, err := service.findStartNode(nodes)

	s.NoError(err)
	s.Equal("start", id)
}

func (s *FlowInferenceServiceTestSuite) TestFindStartNode_NotFound() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "task", Type: "TASK_EXECUTION"},
	}

	id, err := service.findStartNode(nodes)

	s.Error(err)
	s.Empty(id)
	s.Contains(err.Error(), "no START node found")
}

// Test hasLayoutInformation

func (s *FlowInferenceServiceTestSuite) TestHasLayoutInformation_WithLayout() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{
			ID:   "node1",
			Type: "START",
			Layout: &NodeLayout{
				Size:     &NodeSize{Width: 100, Height: 50},
				Position: &NodePosition{X: 0, Y: 0},
			},
		},
		{ID: "node2", Type: "END"},
	}

	result := service.hasLayoutInformation(nodes)

	s.True(result, "Should return true when at least one node has layout")
}

func (s *FlowInferenceServiceTestSuite) TestHasLayoutInformation_WithoutLayout() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "node1", Type: "START"},
		{ID: "node2", Type: "END"},
	}

	result := service.hasLayoutInformation(nodes)

	s.False(result, "Should return false when no nodes have layout")
}

func (s *FlowInferenceServiceTestSuite) TestHasLayoutInformation_EmptyNodes() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{}

	result := service.hasLayoutInformation(nodes)

	s.False(result, "Should return false for empty node list")
}

func (s *FlowInferenceServiceTestSuite) TestHasLayoutInformation_WithEmptyLayoutObject() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{
			ID:     "node1",
			Type:   "START",
			Layout: &NodeLayout{}, // Empty layout object
		},
		{ID: "node2", Type: "END"},
	}

	result := service.hasLayoutInformation(nodes)

	s.False(result, "Should return false when layout is empty object without size or position")
}

// Test addDefaultLayout

func (s *FlowInferenceServiceTestSuite) TestAddDefaultLayout() {
	service := s.service.(*flowInferenceService)
	node := NodeDefinition{
		ID:   "test-node",
		Type: "TASK_EXECUTION",
	}

	// Verify node has no layout initially
	s.Nil(node.Layout)

	// Add default layout
	service.addDefaultLayout(&node)

	// Verify layout is added with default values
	s.NotNil(node.Layout)
	s.NotNil(node.Layout.Size)
	s.Equal(float64(100), node.Layout.Size.Width)
	s.Equal(float64(120), node.Layout.Size.Height)
	s.NotNil(node.Layout.Position)
	s.Equal(float64(0), node.Layout.Position.X)
	s.Equal(float64(0), node.Layout.Position.Y)
}

// Test findEndNode

func (s *FlowInferenceServiceTestSuite) TestFindEndNode_Success() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "task", Type: "TASK_EXECUTION"},
		{ID: "end", Type: "END"},
	}

	id, err := service.findEndNode(nodes)

	s.NoError(err)
	s.Equal("end", id)
}

func (s *FlowInferenceServiceTestSuite) TestFindEndNode_NotFound() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "task", Type: "TASK_EXECUTION"},
	}

	id, err := service.findEndNode(nodes)

	s.Error(err)
	s.Empty(id)
	s.Contains(err.Error(), "no END node found")
}

// Test hasProvisioningNode

func (s *FlowInferenceServiceTestSuite) TestHasProvisioningNode_Exists() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{
			ID:   "prov",
			Type: "TASK_EXECUTION",
			Executor: &ExecutorDefinition{
				Name: executor.ExecutorNameProvisioning,
			},
		},
	}

	result := service.hasProvisioningNode(nodes)

	s.True(result)
}

func (s *FlowInferenceServiceTestSuite) TestHasProvisioningNode_NotExists() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "task", Type: "TASK_EXECUTION"},
	}

	result := service.hasProvisioningNode(nodes)

	s.False(result)
}

// Test createProvisioningNode

func (s *FlowInferenceServiceTestSuite) TestCreateProvisioningNode() {
	service := s.service.(*flowInferenceService)

	// Test with layout
	nodeWithLayout := service.createProvisioningNode("end-node", true)

	s.Equal(provisioningNodeID, nodeWithLayout.ID)
	s.Equal(string(common.NodeTypeTaskExecution), nodeWithLayout.Type)
	s.NotNil(nodeWithLayout.Executor)
	s.Equal(executor.ExecutorNameProvisioning, nodeWithLayout.Executor.Name)
	s.Equal("end-node", nodeWithLayout.OnSuccess)

	// Verify layout is set with default values
	s.NotNil(nodeWithLayout.Layout)
	s.NotNil(nodeWithLayout.Layout.Size)
	s.Equal(float64(100), nodeWithLayout.Layout.Size.Width)
	s.Equal(float64(120), nodeWithLayout.Layout.Size.Height)
	s.NotNil(nodeWithLayout.Layout.Position)
	s.Equal(float64(0), nodeWithLayout.Layout.Position.X)
	s.Equal(float64(0), nodeWithLayout.Layout.Position.Y)

	// Test without layout
	nodeWithoutLayout := service.createProvisioningNode("end-node", false)

	s.Equal(provisioningNodeID, nodeWithoutLayout.ID)
	s.Equal(string(common.NodeTypeTaskExecution), nodeWithoutLayout.Type)
	s.NotNil(nodeWithoutLayout.Executor)
	s.Equal(executor.ExecutorNameProvisioning, nodeWithoutLayout.Executor.Name)
	s.Equal("end-node", nodeWithoutLayout.OnSuccess)

	// Verify layout is not set
	s.Nil(nodeWithoutLayout.Layout)
}

// Test hasUserTypeResolverNode

func (s *FlowInferenceServiceTestSuite) TestHasUserTypeResolverNode_Exists() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{
			ID:   "resolver",
			Type: "TASK_EXECUTION",
			Executor: &ExecutorDefinition{
				Name: executor.ExecutorNameUserTypeResolver,
			},
		},
	}

	result := service.hasUserTypeResolverNode(nodes)

	s.True(result)
}

func (s *FlowInferenceServiceTestSuite) TestHasUserTypeResolverNode_NotExists() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "task", Type: "TASK_EXECUTION"},
	}

	result := service.hasUserTypeResolverNode(nodes)

	s.False(result)
}

// Test createUserTypeResolverNode

func (s *FlowInferenceServiceTestSuite) TestCreateUserTypeResolverNode() {
	service := s.service.(*flowInferenceService)

	// Test with layout
	nodeWithLayout := service.createUserTypeResolverNode(userTypePromptNodeID, true)

	s.Equal(userTypeResolverNodeID, nodeWithLayout.ID)
	s.Equal(string(common.NodeTypeTaskExecution), nodeWithLayout.Type)
	s.NotNil(nodeWithLayout.Executor)
	s.Equal(executor.ExecutorNameUserTypeResolver, nodeWithLayout.Executor.Name)
	s.Equal(userTypePromptNodeID, nodeWithLayout.OnIncomplete, "OnIncomplete should be set to prompt node ID")

	// Verify layout is set with default values
	s.NotNil(nodeWithLayout.Layout)
	s.NotNil(nodeWithLayout.Layout.Size)
	s.Equal(float64(100), nodeWithLayout.Layout.Size.Width)
	s.Equal(float64(120), nodeWithLayout.Layout.Size.Height)
	s.NotNil(nodeWithLayout.Layout.Position)
	s.Equal(float64(0), nodeWithLayout.Layout.Position.X)
	s.Equal(float64(0), nodeWithLayout.Layout.Position.Y)

	// Test without layout
	nodeWithoutLayout := service.createUserTypeResolverNode(userTypePromptNodeID, false)

	s.Equal(userTypeResolverNodeID, nodeWithoutLayout.ID)
	s.Equal(string(common.NodeTypeTaskExecution), nodeWithoutLayout.Type)
	s.NotNil(nodeWithoutLayout.Executor)
	s.Equal(executor.ExecutorNameUserTypeResolver, nodeWithoutLayout.Executor.Name)
	s.Equal(userTypePromptNodeID, nodeWithoutLayout.OnIncomplete, "OnIncomplete should be set to prompt node ID")

	// Verify layout is not set
	s.Nil(nodeWithoutLayout.Layout)
}

// Test insertNodeBefore

func (s *FlowInferenceServiceTestSuite) TestInsertNodeBefore_WithOnSuccess() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "task"},
		{ID: "task", Type: "TASK_EXECUTION", OnSuccess: "end"},
		{ID: "end", Type: "END"},
	}
	newNode := NodeDefinition{ID: "new", Type: "TASK_EXECUTION", OnSuccess: "end"}

	err := service.insertNodeBefore(&nodes, newNode, "end")

	s.NoError(err)
	s.Len(nodes, 4)
	s.Equal("new", nodes[1].OnSuccess) // task now points to new
	s.Equal("end", nodes[3].OnSuccess) // new node points to end
}

func (s *FlowInferenceServiceTestSuite) TestInsertNodeBefore_WithOnFailure() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "task"},
		{ID: "task", Type: "TASK_EXECUTION", OnSuccess: "success", OnFailure: "end"},
		{ID: "success", Type: "END"},
		{ID: "end", Type: "END"},
	}
	newNode := NodeDefinition{ID: "new", Type: "TASK_EXECUTION", OnSuccess: "end"}

	err := service.insertNodeBefore(&nodes, newNode, "end")

	s.NoError(err)
	s.Equal("new", nodes[1].OnFailure)
}

func (s *FlowInferenceServiceTestSuite) TestInsertNodeBefore_WithActions() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "prompt"},
		{
			ID:   "prompt",
			Type: "PROMPT",
			Prompts: []PromptDefinition{
				{Action: &ActionDefinition{Ref: "action1", NextNode: "end"}},
				{Action: &ActionDefinition{Ref: "action2", NextNode: "task"}},
			},
		},
		{ID: "task", Type: "TASK_EXECUTION"},
		{ID: "end", Type: "END"},
	}
	newNode := NodeDefinition{ID: "new", Type: "TASK_EXECUTION", OnSuccess: "end"}

	err := service.insertNodeBefore(&nodes, newNode, "end")

	s.NoError(err)
	s.Equal("new", nodes[1].Prompts[0].Action.NextNode)
	s.Equal("task", nodes[1].Prompts[1].Action.NextNode) // unchanged
}

func (s *FlowInferenceServiceTestSuite) TestInsertNodeBefore_NoNodesPointingToTarget() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "task"},
		{ID: "task", Type: "TASK_EXECUTION"},
		{ID: "end", Type: "END"},
	}
	newNode := NodeDefinition{ID: "new", Type: "TASK_EXECUTION", OnSuccess: "end"}

	err := service.insertNodeBefore(&nodes, newNode, "end")

	s.Error(err)
	s.Contains(err.Error(), "no nodes pointing to target node")
}

// Test insertNodeAfterStart

func (s *FlowInferenceServiceTestSuite) TestInsertNodeAfterStart_Success() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START", OnSuccess: "task"},
		{ID: "task", Type: "TASK_EXECUTION", OnSuccess: "end"},
		{ID: "end", Type: "END"},
	}
	newNode := NodeDefinition{ID: "new", Type: "TASK_EXECUTION"}

	err := service.insertNodeAfterStart(&nodes, newNode, "start")

	s.NoError(err)
	s.Len(nodes, 4)
	s.Equal("new", nodes[0].OnSuccess)  // start now points to new
	s.Equal("task", nodes[3].OnSuccess) // new node points to task
}

func (s *FlowInferenceServiceTestSuite) TestInsertNodeAfterStart_NoOnSuccess() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "start", Type: "START"},
		{ID: "end", Type: "END"},
	}
	newNode := NodeDefinition{ID: "new", Type: "TASK_EXECUTION"}

	err := service.insertNodeAfterStart(&nodes, newNode, "start")

	s.Error(err)
	s.Contains(err.Error(), "START node has no onSuccess defined")
}

func (s *FlowInferenceServiceTestSuite) TestInsertNodeAfterStart_StartNodeNotFound() {
	service := s.service.(*flowInferenceService)
	nodes := []NodeDefinition{
		{ID: "task", Type: "TASK_EXECUTION"},
	}
	newNode := NodeDefinition{ID: "new", Type: "TASK_EXECUTION"}

	err := service.insertNodeAfterStart(&nodes, newNode, "start")

	s.Error(err)
	s.Contains(err.Error(), "START node not found")
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_InsertProvisioningNodeError() {
	authFlow := &FlowDefinition{
		Handle:   "basic-auth",
		Name:     "Basic Authentication",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START", OnSuccess: "prompt"},
			{ID: "prompt", Type: "PROMPT", OnSuccess: "auth"},
			{
				ID:   "auth",
				Type: "TASK_EXECUTION",
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameBasicAuth,
				},
				OnSuccess: "orphan", // Points to non-existent node
			},
			// Missing END node - will cause insertProvisioningNode to fail
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.Error(err)
	s.Nil(regFlow)
	s.Contains(err.Error(), "no END node found")
}

func (s *FlowInferenceServiceTestSuite) TestInferRegistrationFlow_InsertUserTypeResolverError() {
	authFlow := &FlowDefinition{
		Handle:   "basic-auth",
		Name:     "Basic Authentication",
		FlowType: common.FlowTypeAuthentication,
		Nodes: []NodeDefinition{
			{ID: "start", Type: "START"}, // No OnSuccess - will cause error in insertNodeAfterStart
			{ID: "task", Type: "TASK_EXECUTION", OnSuccess: "end"},
			{ID: "end", Type: "END"},
			{
				ID:   provisioningNodeID,
				Type: "TASK_EXECUTION",
				Executor: &ExecutorDefinition{
					Name: executor.ExecutorNameProvisioning,
				},
				OnSuccess: "end",
			},
		},
	}

	regFlow, err := s.service.InferRegistrationFlow(authFlow)

	s.Error(err)
	s.Nil(regFlow)
	s.Contains(err.Error(), "START node has no onSuccess defined")
}

// Helper methods

func (s *FlowInferenceServiceTestSuite) hasNode(nodes []NodeDefinition, nodeID string) bool {
	for _, node := range nodes {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func (s *FlowInferenceServiceTestSuite) getNode(nodes []NodeDefinition, nodeID string) *NodeDefinition {
	for i := range nodes {
		if nodes[i].ID == nodeID {
			return &nodes[i]
		}
	}
	return nil
}

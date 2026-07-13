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

package flowexec

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/executormock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

type EngineTestSuite struct {
	suite.Suite
}

func TestEngineTestSuite(t *testing.T) {
	suite.Run(t, new(EngineTestSuite))
}

func newAuthenticatedAuthUser() providers.AuthUser {
	var authUser providers.AuthUser
	_ = authUser.UnmarshalJSON([]byte(`{"entityReferenceToken":"tok","attributeToken":"tok"}`))
	return authUser
}

func (s *EngineTestSuite) TestGetNodeInputs_ExecutorBackedNode() {
	t := s.T()
	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	expectedInputs := []providers.Input{
		{Identifier: "username", Type: "string", Required: true},
		{Identifier: "password", Type: "string", Required: true},
	}
	mockNode.On("GetInputs").Return(expectedInputs)

	inputs := getNodeInputs(mockNode)

	s.NotNil(inputs)
	s.Len(inputs, 2)
	s.Equal("username", inputs[0].Identifier)
	s.Equal("password", inputs[1].Identifier)
}

func (s *EngineTestSuite) TestGetNodeInputs_PromptNode() {
	t := s.T()
	mockNode := coremock.NewPromptNodeInterfaceMock(t)
	prompts := []common.Prompt{
		{
			Inputs: []providers.Input{
				{Identifier: "email", Type: "string", Required: true},
			},
		},
		{
			Inputs: []providers.Input{
				{Identifier: "code", Type: "string", Required: true},
			},
		},
	}
	mockNode.On("GetPrompts").Return(prompts)

	inputs := getNodeInputs(mockNode)

	s.NotNil(inputs)
	s.Len(inputs, 2)
	s.Equal("email", inputs[0].Identifier)
	s.Equal("code", inputs[1].Identifier)
}

func (s *EngineTestSuite) TestGetNodeInputs_RegularNode() {
	mockNode := coremock.NewNodeInterfaceMock(s.T())

	inputs := getNodeInputs(mockNode)

	s.Nil(inputs)
}

func (s *EngineTestSuite) TestGetNodeInputs_NilNode() {
	inputs := getNodeInputs(nil)

	s.Nil(inputs)
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_AdditionalData() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		RuntimeData: make(map[string]string),
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
		AdditionalData: map[string]string{
			"passkeyChallenge":       `{"challenge": "abc123"}`,
			"passkeyCreationOptions": `{"rpId": "example.com"}`,
		},
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.NotNil(ctx.AdditionalData)
	s.Equal(`{"challenge": "abc123"}`, ctx.AdditionalData["passkeyChallenge"])
	s.Equal(`{"rpId": "example.com"}`, ctx.AdditionalData["passkeyCreationOptions"])
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_MergesAdditionalData() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		RuntimeData: make(map[string]string),
		AdditionalData: map[string]string{
			"existingKey": "existingValue",
		},
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
		AdditionalData: map[string]string{
			"newKey": "newValue",
		},
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.NotNil(ctx.AdditionalData)
	s.Equal("existingValue", ctx.AdditionalData["existingKey"])
	s.Equal("newValue", ctx.AdditionalData["newKey"])
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_ClearsActionOnComplete() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		CurrentAction: "someAction",
		RuntimeData:   make(map[string]string),
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.Empty(ctx.CurrentAction)
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_ClearsActionOnForward() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		CurrentAction: "someAction",
		RuntimeData:   make(map[string]string),
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusForward,
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.Empty(ctx.CurrentAction)
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_PreservesActionOnIncomplete() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		CurrentAction: "passkeyChallenge",
		RuntimeData:   make(map[string]string),
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusIncomplete,
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.Equal("passkeyChallenge", ctx.CurrentAction)
}

func (s *EngineTestSuite) TestTrackPresentedOptionalInputs_MergesOptionalInputIdentifiers() {
	fe := &flowEngine{}
	ctx := &EngineContext{
		RuntimeData: map[string]string{
			common.RuntimeKeyPresentedOptionalInputs: "nickname",
		},
	}
	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusIncomplete,
		Type:   common.NodeResponseTypeView,
		Inputs: []providers.Input{
			{Identifier: "given_name", Required: false},
			{Identifier: "username", Required: true},
		},
	}

	fe.trackPresentedOptionalInputs(ctx, nodeResp)

	presented := core.ParsePresentedOptionalInputIdentifiers(
		nodeResp.RuntimeData[common.RuntimeKeyPresentedOptionalInputs])
	s.Contains(presented, "nickname")
	s.Contains(presented, "given_name")
}

func (s *EngineTestSuite) TestTrackPresentedOptionalInputs_SkipsNonPromptResponses() {
	fe := &flowEngine{}
	ctx := &EngineContext{
		RuntimeData: map[string]string{
			common.RuntimeKeyPresentedOptionalInputs: "nickname",
		},
	}
	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusForward,
		Type:   common.NodeResponseTypeView,
		Inputs: []providers.Input{
			{Identifier: "given_name", Required: false},
		},
	}

	fe.trackPresentedOptionalInputs(ctx, nodeResp)

	s.Nil(nodeResp.RuntimeData)
}

func (s *EngineTestSuite) TestResolveStepForRedirection_WithAdditionalData() {
	fe := &flowEngine{}

	ctx := &EngineContext{
		AdditionalData: map[string]string{
			"passkeyChallenge": `{"challenge": "xyz789"}`,
			"sessionToken":     "abc123",
		},
	}

	nodeResp := &common.NodeResponse{
		RedirectURL: "https://example.com/auth",
	}

	flowStep := &FlowStep{
		Data: FlowData{},
	}

	err := fe.resolveStepForRedirection(ctx, nodeResp, flowStep)

	s.NoError(err)
	s.Equal("https://example.com/auth", flowStep.Data.RedirectURL)
	s.NotNil(flowStep.Data.AdditionalData)
	s.Equal(`{"challenge": "xyz789"}`, flowStep.Data.AdditionalData["passkeyChallenge"])
	s.Equal("abc123", flowStep.Data.AdditionalData["sessionToken"])
}

func (s *EngineTestSuite) TestResolveStepForRedirection_NoAdditionalData() {
	fe := &flowEngine{}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		RedirectURL: "https://example.com/auth",
	}

	flowStep := &FlowStep{
		Data: FlowData{},
	}

	err := fe.resolveStepForRedirection(ctx, nodeResp, flowStep)

	s.NoError(err)
	s.Equal("https://example.com/auth", flowStep.Data.RedirectURL)
	s.Nil(flowStep.Data.AdditionalData)
}

func (s *EngineTestSuite) TestResolveStepForRedirection_NilNodeResponse() {
	fe := &flowEngine{}
	ctx := &EngineContext{}
	flowStep := &FlowStep{}

	err := fe.resolveStepForRedirection(ctx, nil, flowStep)

	s.Error(err)
	s.Contains(err.Error(), "node response is nil")
}

func (s *EngineTestSuite) TestResolveStepForRedirection_EmptyRedirectURL() {
	fe := &flowEngine{}
	ctx := &EngineContext{}
	nodeResp := &common.NodeResponse{
		RedirectURL: "",
	}
	flowStep := &FlowStep{}

	err := fe.resolveStepForRedirection(ctx, nodeResp, flowStep)

	s.Error(err)
	s.Contains(err.Error(), "redirect URL not found")
}

func (s *EngineTestSuite) TestResolveStepDetailsForPrompt_WithAdditionalData() {
	fe := &flowEngine{}

	ctx := &EngineContext{
		AdditionalData: map[string]string{
			"passkeyCreationOptions": `{"rpId": "example.com"}`,
		},
	}

	nodeResp := &common.NodeResponse{
		Inputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
	}

	flowStep := &FlowStep{
		Data: FlowData{},
	}

	err := fe.resolveStepDetailsForPrompt(ctx, nodeResp, flowStep)

	s.NoError(err)
	s.NotNil(flowStep.Data.AdditionalData)
	s.Equal(`{"rpId": "example.com"}`, flowStep.Data.AdditionalData["passkeyCreationOptions"])
}

func (s *EngineTestSuite) TestResolveStepDetailsForPrompt_WithActions() {
	fe := &flowEngine{}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		Actions: []common.Action{
			{Ref: "submit-action", NextNode: "next-node"},
		},
	}

	flowStep := &FlowStep{
		Data: FlowData{},
	}

	err := fe.resolveStepDetailsForPrompt(ctx, nodeResp, flowStep)

	s.NoError(err)
	s.Len(flowStep.Data.Actions, 1)
	s.Equal("submit-action", flowStep.Data.Actions[0].Ref)
}

func (s *EngineTestSuite) TestResolveStepDetailsForPrompt_NilNodeResponse() {
	fe := &flowEngine{}
	ctx := &EngineContext{}
	flowStep := &FlowStep{}

	err := fe.resolveStepDetailsForPrompt(ctx, nil, flowStep)

	s.Error(err)
	s.Contains(err.Error(), "node response is nil")
}

func (s *EngineTestSuite) TestResolveStepDetailsForPrompt_NoInputsOrActions() {
	fe := &flowEngine{}
	ctx := &EngineContext{}
	nodeResp := &common.NodeResponse{}
	flowStep := &FlowStep{}

	err := fe.resolveStepDetailsForPrompt(ctx, nodeResp, flowStep)

	s.Error(err)
	s.Contains(err.Error(), "no required data or actions found")
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_RuntimeData() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		RuntimeData: map[string]string{"existing": "value"},
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
		RuntimeData: map[string]string{
			"newKey": "newValue",
		},
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.Equal("value", ctx.RuntimeData["existing"])
	s.Equal("newValue", ctx.RuntimeData["newKey"])
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_RuntimeDataNilContext() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{} // No RuntimeData initialized

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
		RuntimeData: map[string]string{
			"userID": "user-123",
		},
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.NotNil(ctx.RuntimeData)
	s.Equal("user-123", ctx.RuntimeData["userID"])
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_Assertion() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		Status:    common.NodeStatusComplete,
		Assertion: "test-assertion-token",
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.Equal("test-assertion-token", ctx.Assertion)
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_SetsAuthUserWhenAuthenticated() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{}
	nodeResp := &common.NodeResponse{
		Status:   common.NodeStatusComplete,
		AuthUser: newAuthenticatedAuthUser(),
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.True(ctx.AuthUser.IsAuthenticated())
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_PreservesAuthUserWhenNotAuthenticated() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	existingAuthUser := newAuthenticatedAuthUser()
	ctx := &EngineContext{
		AuthUser: existingAuthUser,
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.True(ctx.AuthUser.IsAuthenticated())
	existingJSON, err := json.Marshal(&existingAuthUser)
	s.NoError(err)
	updatedJSON, err := json.Marshal(&ctx.AuthUser)
	s.NoError(err)
	s.JSONEq(string(existingJSON), string(updatedJSON))
}

func (s *EngineTestSuite) TestUpdateContextWithNodeResponse_ReplacesAuthUserWhenAuthenticated() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		AuthUser: newAuthenticatedAuthUser(),
	}

	var newAuthUser providers.AuthUser
	err := newAuthUser.UnmarshalJSON([]byte(
		`{"entityReference":{"entityId":"user-456"},"attributes":{}}`))
	s.NoError(err)

	nodeResp := &common.NodeResponse{
		Status:   common.NodeStatusComplete,
		AuthUser: newAuthUser,
	}

	fe.updateContextWithNodeResponse(ctx, nodeResp)

	s.True(ctx.AuthUser.IsAuthenticated())
	updatedJSON, err := json.Marshal(&ctx.AuthUser)
	s.NoError(err)
	expectedJSON, err := json.Marshal(&newAuthUser)
	s.NoError(err)
	s.JSONEq(string(expectedJSON), string(updatedJSON))
}

func (s *EngineTestSuite) TestResolveStepForRedirection_WithInputs() {
	fe := &flowEngine{}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		RedirectURL: "https://example.com/auth",
		Inputs: []providers.Input{
			{Identifier: "code", Type: "string", Required: true},
		},
	}

	flowStep := &FlowStep{
		Data: FlowData{},
	}

	err := fe.resolveStepForRedirection(ctx, nodeResp, flowStep)

	s.NoError(err)
	s.Len(flowStep.Data.Inputs, 1)
	s.Equal("code", flowStep.Data.Inputs[0].Identifier)
	s.Equal(providers.FlowStatusIncomplete, flowStep.Status)
	s.Equal(common.StepTypeRedirection, flowStep.Type)
}

func (s *EngineTestSuite) TestResolveStepForRedirection_AppendsInputs() {
	fe := &flowEngine{}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		RedirectURL: "https://example.com/auth",
		Inputs: []providers.Input{
			{Identifier: "code", Type: "string", Required: true},
		},
	}

	flowStep := &FlowStep{
		Data: FlowData{
			Inputs: []providers.Input{
				{Identifier: "state", Type: "string", Required: true},
			},
		},
	}

	err := fe.resolveStepForRedirection(ctx, nodeResp, flowStep)

	s.NoError(err)
	s.Len(flowStep.Data.Inputs, 2)
}

func (s *EngineTestSuite) TestResolveStepDetailsForPrompt_WithMeta() {
	fe := &flowEngine{}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		Inputs: []providers.Input{
			{Identifier: "username", Type: "string", Required: true},
		},
		Meta: map[string]interface{}{
			"title":       "Login",
			"description": "Enter your credentials",
		},
	}

	flowStep := &FlowStep{
		Data: FlowData{},
	}

	err := fe.resolveStepDetailsForPrompt(ctx, nodeResp, flowStep)

	s.NoError(err)
	s.NotNil(flowStep.Data.Meta)
}

func (s *EngineTestSuite) TestResolveStepDetailsForPrompt_WithError() {
	fe := &flowEngine{}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		Inputs: []providers.Input{
			{Identifier: "otp", Type: "string", Required: true},
		},
		Error: &tidcommon.ServiceError{
			Code: "FET-1008",
			Error: tidcommon.I18nMessage{
				Key:          "flows.executor.errors.invalid_otp",
				DefaultValue: "Invalid OTP provided",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key:          "flows.executor.errors.invalid_otp_desc",
				DefaultValue: "The one-time password provided is invalid or has expired",
			},
		},
	}

	flowStep := &FlowStep{
		Data: FlowData{},
	}

	err := fe.resolveStepDetailsForPrompt(ctx, nodeResp, flowStep)

	s.NoError(err)
	s.NotNil(flowStep.Error)
	s.Equal("FET-1008", flowStep.Error.Code)
	s.Equal("Invalid OTP provided", flowStep.Error.Error.DefaultValue)
	s.Equal(providers.FlowStatusIncomplete, flowStep.Status)
	s.Equal(common.StepTypeView, flowStep.Type)
}

func (s *EngineTestSuite) TestResolveStepDetailsForPrompt_AppendsInputs() {
	fe := &flowEngine{}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		Inputs: []providers.Input{
			{Identifier: "password", Type: "string", Required: true},
		},
	}

	flowStep := &FlowStep{
		Data: FlowData{
			Inputs: []providers.Input{
				{Identifier: "username", Type: "string", Required: true},
			},
		},
	}

	err := fe.resolveStepDetailsForPrompt(ctx, nodeResp, flowStep)

	s.NoError(err)
	s.Len(flowStep.Data.Inputs, 2)
}

func (s *EngineTestSuite) TestResolveStepDetailsForPrompt_ExistingActions() {
	fe := &flowEngine{}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		Actions: []common.Action{
			{Ref: "submit-action"},
		},
	}

	flowStep := &FlowStep{
		Data: FlowData{
			Actions: []common.Action{
				{Ref: "existing-action"},
			},
		},
	}

	err := fe.resolveStepDetailsForPrompt(ctx, nodeResp, flowStep)

	s.NoError(err)
	// Actions are replaced, not appended
	s.Len(flowStep.Data.Actions, 1)
	s.Equal("submit-action", flowStep.Data.Actions[0].Ref)
}

func (s *EngineTestSuite) TestGetNodeInputs_PromptNodeEmptyInputs() {
	mockNode := coremock.NewPromptNodeInterfaceMock(s.T())
	prompts := []common.Prompt{
		{
			Inputs: []providers.Input{},
		},
	}
	mockNode.On("GetPrompts").Return(prompts)

	inputs := getNodeInputs(mockNode)

	s.Nil(inputs)
}

func (s *EngineTestSuite) TestClearSensitiveInputs_AuthFlowRemovesPassword() {
	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(s.T())
	mockNode.On("GetInputs").Return([]providers.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
		{Identifier: "password", Type: providers.InputTypePassword, Required: true},
	})
	mockNode.On("GetExecutor").Return(nil).Maybe()

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "testuser",
			"password": "secret123",
		},
	}

	fe.clearSensitiveInputs(ctx, mockNode)

	s.Equal("testuser", ctx.UserInputs["username"])
	_, exists := ctx.UserInputs["password"]
	s.False(exists)
}

func (s *EngineTestSuite) TestClearSensitiveInputs_AuthFlowRemovesOTP() {
	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(s.T())
	mockNode.On("GetInputs").Return([]providers.Input{
		{Identifier: "otp", Type: providers.InputTypeOTP, Required: true},
	})
	mockNode.On("GetExecutor").Return(nil).Maybe()

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"otp": "123456",
		},
	}

	fe.clearSensitiveInputs(ctx, mockNode)

	_, exists := ctx.UserInputs["otp"]
	s.False(exists)
}

func (s *EngineTestSuite) TestClearSensitiveInputs_RegistrationFlowRetainsPassword() {
	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(s.T())
	mockNode.On("GetInputs").Return([]providers.Input{
		{Identifier: "password", Type: providers.InputTypePassword, Required: true},
	}).Maybe()

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"password": "secret123",
		},
	}

	fe.clearSensitiveInputs(ctx, mockNode)

	s.Equal("secret123", ctx.UserInputs["password"])
}

func (s *EngineTestSuite) TestClearSensitiveInputs_NoNodeInputs() {
	mockNode := coremock.NewNodeInterfaceMock(s.T())

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"password": "secret123",
		},
	}

	fe.clearSensitiveInputs(ctx, mockNode)

	// Password should remain since the node has no declared inputs
	s.Equal("secret123", ctx.UserInputs["password"])
}

func (s *EngineTestSuite) TestClearSensitiveInputs_NonSensitiveInputsRetained() {
	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(s.T())
	mockNode.On("GetInputs").Return([]providers.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
	})
	mockNode.On("GetExecutor").Return(nil).Maybe()

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "testuser",
		},
	}

	fe.clearSensitiveInputs(ctx, mockNode)

	s.Equal("testuser", ctx.UserInputs["username"])
}

func (s *EngineTestSuite) TestClearSensitiveInputs_NoNodeInputsUsesExecutorDefaults() {
	// Node has no configured inputs, but executor defaults have PASSWORD_INPUT.
	mockExecutor := coremock.NewExecutorInterfaceMock(s.T())
	mockExecutor.On("GetDefaultInputs").Return([]providers.Input{
		{Identifier: "username", Type: "string", Required: true},
		{Identifier: "password", Type: providers.InputTypePassword, Required: true},
	})

	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(s.T())
	mockNode.On("GetInputs").Return([]providers.Input{})
	mockNode.On("GetExecutor").Return(mockExecutor)

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"username": "testuser",
			"password": "secret123",
		},
	}

	fe.clearSensitiveInputs(ctx, mockNode)

	s.Equal("testuser", ctx.UserInputs["username"])
	_, exists := ctx.UserInputs["password"]
	s.False(exists)
}

func (s *EngineTestSuite) TestClearSensitiveInputs_UserOnboardingFlowRetainsPassword() {
	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(s.T())
	mockNode.On("GetInputs").Return([]providers.Input{
		{Identifier: "password", Type: providers.InputTypePassword, Required: true},
	}).Maybe()

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: providers.FlowTypeUserOnboarding,
		UserInputs: map[string]string{
			"password": "secret123",
		},
	}

	fe.clearSensitiveInputs(ctx, mockNode)

	s.Equal("secret123", ctx.UserInputs["password"])
}

// Tests for one-time-use cleanup

func (s *EngineTestSuite) TestClearNodePackageOneTimeUseInputs_ClearsSignaledNodeInput() {
	t := s.T()

	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	mockNode.On("GetInputs").Return([]providers.Input{
		{Identifier: "challenge", Type: "TEXT_INPUT", OneTimeUse: true},
		{Identifier: "username", Type: "TEXT_INPUT"},
	})

	fe := &flowEngine{}
	ctx := &EngineContext{
		UserInputs:     map[string]string{"challenge": "abc123", "username": "testuser"},
		consumedInputs: []string{"challenge"},
	}

	fe.clearNodePackageOneTimeUseInputs(ctx, mockNode)

	_, challengeExists := ctx.UserInputs["challenge"]
	s.False(challengeExists, "signaled OneTimeUse node input should be removed")
	s.Equal("testuser", ctx.UserInputs["username"], "non-OneTimeUse input should be preserved")
	s.Empty(ctx.consumedInputs, "pending list should be drained")
}

func (s *EngineTestSuite) TestClearNodePackageOneTimeUseInputs_FallsBackToExecutorDefaults() {
	t := s.T()

	mockExecutor := coremock.NewExecutorInterfaceMock(t)
	mockExecutor.On("GetDefaultInputs").Return([]providers.Input{
		{Identifier: "challenge", Type: "TEXT_INPUT", OneTimeUse: true},
	})

	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	mockNode.On("GetInputs").Return([]providers.Input(nil))
	mockNode.On("GetExecutor").Return(mockExecutor)

	fe := &flowEngine{}
	ctx := &EngineContext{
		UserInputs:     map[string]string{"challenge": "abc123"},
		consumedInputs: []string{"challenge"},
	}

	fe.clearNodePackageOneTimeUseInputs(ctx, mockNode)

	_, exists := ctx.UserInputs["challenge"]
	s.False(exists)
}

func (s *EngineTestSuite) TestClearNodePackageOneTimeUseInputs_IgnoresNonOneTimeUseSignals() {
	t := s.T()

	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	mockNode.On("GetInputs").Return([]providers.Input{
		{Identifier: "username", Type: "TEXT_INPUT"},
	})

	fe := &flowEngine{}
	ctx := &EngineContext{
		UserInputs:     map[string]string{"username": "testuser"},
		consumedInputs: []string{"username"},
	}

	fe.clearNodePackageOneTimeUseInputs(ctx, mockNode)

	s.Equal("testuser", ctx.UserInputs["username"],
		"identifier not declared as OneTimeUse should not be cleared even if signaled")
}

func (s *EngineTestSuite) TestClearNodePackageOneTimeUseInputs_NoPendingIsNoop() {
	t := s.T()

	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)

	fe := &flowEngine{}
	ctx := &EngineContext{
		UserInputs: map[string]string{"challenge": "abc123"},
	}

	fe.clearNodePackageOneTimeUseInputs(ctx, mockNode)

	s.Equal("abc123", ctx.UserInputs["challenge"])
}

func (s *EngineTestSuite) TestClearNodePackageOneTimeUseInputs_ClearsInterceptorDeclaredInput() {
	t := s.T()

	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	mockNode.On("GetInputs").Return([]providers.Input{{Identifier: "username", Type: "TEXT_INPUT"}})
	mockNode.On("GetID").Return("node-1")
	mockNode.On("GetProperties").Return(map[string]interface{}(nil))

	interceptorMock := coremock.NewInterceptorInterfaceMock(t)
	interceptorMock.On("GetInputs").Return([]providers.Input{
		{Identifier: "challenge", Type: "TEXT_INPUT", OneTimeUse: true},
	})

	unit := newTestInterceptorUnitMock(t, "captcha", providers.InterceptorModePreNode,
		providers.InterceptorScopeAll, nil)
	unit.SetInterceptor(interceptorMock)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetInterceptors", providers.InterceptorModePreNode).
		Return([]core.InterceptorUnitInterface{unit})
	mockGraph.On("GetInterceptors", providers.InterceptorModePostNode).
		Return([]core.InterceptorUnitInterface{})

	fe := &flowEngine{}
	ctx := &EngineContext{
		Graph:          mockGraph,
		UserInputs:     map[string]string{"challenge": "abc123", "username": "testuser"},
		consumedInputs: []string{"challenge"},
	}

	fe.clearNodePackageOneTimeUseInputs(ctx, mockNode)

	_, challengeExists := ctx.UserInputs["challenge"]
	s.False(challengeExists,
		"OneTimeUse input declared by a scoped interceptor should be cleared when signaled")
	s.Equal("testuser", ctx.UserInputs["username"])
}

func (s *EngineTestSuite) TestClearRequestPackageOneTimeUseInputs_ClearsSignaledInput() {
	t := s.T()

	interceptorMock := coremock.NewInterceptorInterfaceMock(t)
	interceptorMock.On("GetInputs").Return([]providers.Input{
		{Identifier: "captchaToken", Type: "TEXT_INPUT", OneTimeUse: true},
	})

	unit := newTestInterceptorUnitMock(t, "captcha", providers.InterceptorModePreRequest,
		providers.InterceptorScopeAll, nil)
	unit.SetInterceptor(interceptorMock)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetInterceptors", providers.InterceptorModePreRequest).
		Return([]core.InterceptorUnitInterface{unit})
	mockGraph.On("GetInterceptors", providers.InterceptorModePostRequest).
		Return([]core.InterceptorUnitInterface{})

	fe := &flowEngine{}
	ctx := &EngineContext{
		Graph:          mockGraph,
		UserInputs:     map[string]string{"captchaToken": "abc123"},
		consumedInputs: []string{"captchaToken"},
	}

	fe.clearRequestPackageOneTimeUseInputs(ctx)

	_, exists := ctx.UserInputs["captchaToken"]
	s.False(exists)
	s.Empty(ctx.consumedInputs, "pending request list should be drained")
}

func (s *EngineTestSuite) TestClearRequestPackageOneTimeUseInputs_DrainsUnmatchedEntries() {
	t := s.T()

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetInterceptors", providers.InterceptorModePreRequest).
		Return([]core.InterceptorUnitInterface{})
	mockGraph.On("GetInterceptors", providers.InterceptorModePostRequest).
		Return([]core.InterceptorUnitInterface{})

	fe := &flowEngine{}
	ctx := &EngineContext{
		Graph:          mockGraph,
		UserInputs:     map[string]string{"leftover": "value"},
		consumedInputs: []string{"leftover"},
	}

	fe.clearRequestPackageOneTimeUseInputs(ctx)

	s.Equal("value", ctx.UserInputs["leftover"],
		"unmatched identifiers should not be deleted from UserInputs")
	s.Empty(ctx.consumedInputs, "consumed list should be drained after request package cleanup")
}

func (s *EngineTestSuite) TestClearNodePackageOneTimeUseInputs_PreservesUnmatchedForRequestScope() {
	t := s.T()

	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	mockNode.On("GetInputs").Return([]providers.Input{
		{Identifier: "challenge", Type: "TEXT_INPUT", OneTimeUse: true},
	})
	mockNode.On("GetID").Return("node-1")
	mockNode.On("GetProperties").Return(map[string]interface{}(nil))

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetInterceptors", providers.InterceptorModePreNode).
		Return([]core.InterceptorUnitInterface{})
	mockGraph.On("GetInterceptors", providers.InterceptorModePostNode).
		Return([]core.InterceptorUnitInterface{})

	fe := &flowEngine{}
	ctx := &EngineContext{
		Graph:          mockGraph,
		UserInputs:     map[string]string{"challenge": "abc123", "captchaToken": "xyz"},
		consumedInputs: []string{"challenge", "captchaToken"},
	}

	fe.clearNodePackageOneTimeUseInputs(ctx, mockNode)

	_, challengeExists := ctx.UserInputs["challenge"]
	s.False(challengeExists, "node-scope OneTimeUse identifier should be cleared")
	s.Equal("xyz", ctx.UserInputs["captchaToken"],
		"identifier not declared for this node should be preserved for the request-scope cleanup")
	s.Equal([]string{"captchaToken"}, ctx.consumedInputs,
		"unmatched identifiers stay in the list for later scopes")
}

// Tests for display-only prompt node handling

func (s *EngineTestSuite) TestIsDisplayOnlyPromptNode_WithDisplayOnlyPrompt() {
	t := s.T()
	mockPromptNode := coremock.NewPromptNodeInterfaceMock(t)
	mockPromptNode.On("IsDisplayOnly").Return(true)

	fe := &flowEngine{}
	result := fe.isDisplayOnlyPromptNode(mockPromptNode)

	s.True(result)
}

func (s *EngineTestSuite) TestIsDisplayOnlyPromptNode_WithRegularPrompt() {
	t := s.T()
	mockPromptNode := coremock.NewPromptNodeInterfaceMock(t)
	mockPromptNode.On("IsDisplayOnly").Return(false)

	fe := &flowEngine{}
	result := fe.isDisplayOnlyPromptNode(mockPromptNode)

	s.False(result)
}

func (s *EngineTestSuite) TestIsDisplayOnlyPromptNode_WithNonPromptNode() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{}
	result := fe.isDisplayOnlyPromptNode(mockNode)

	s.False(result)
}

func (s *EngineTestSuite) TestHandleDisplayOnlyPromptResponse_ForwardToNextNode() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	mockPromptNode := coremock.NewPromptNodeInterfaceMock(t)
	mockPromptNode.On("GetNextNode").Return("next-node")

	mockNextNode := coremock.NewNodeInterfaceMock(t)
	mockNextNode.On("GetType").Return(common.NodeTypePrompt)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetNode", "next-node").Return(mockNextNode, true)
	mockGraph.On("HasSegments").Return(false)

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		CurrentNode:    mockPromptNode,
		Graph:          mockGraph,
		AdditionalData: map[string]string{"ctx_key": "ctx_value"},
	}

	flowStep := &FlowStep{
		Data: FlowData{},
	}

	nodeResp := &common.NodeResponse{
		Status:         common.NodeStatusComplete,
		Meta:           map[string]interface{}{"components": []interface{}{}},
		AdditionalData: map[string]string{"msg_key": "msg_value"},
	}

	nextNode, complete, err := fe.handleDisplayOnlyPromptResponse(ctx, nodeResp, flowStep, nil)

	s.Nil(err)
	s.False(complete)
	s.Nil(nextNode)
	s.Equal(providers.FlowStatusIncomplete, flowStep.Status)
	s.Equal(common.StepTypeView, flowStep.Type)
	s.Equal(map[string]interface{}{"components": []interface{}{}}, flowStep.Data.Meta)
	s.Contains(flowStep.Data.AdditionalData, "ctx_key")
	s.Contains(flowStep.Data.AdditionalData, "msg_key")
	s.Equal(mockNextNode, ctx.CurrentNode)
}

func (s *EngineTestSuite) TestHandleDisplayOnlyPromptResponse_ForwardToEndNode() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	mockPromptNode := coremock.NewPromptNodeInterfaceMock(t)
	mockPromptNode.On("GetNextNode").Return("end-node")

	mockEndNode := coremock.NewNodeInterfaceMock(t)
	mockEndNode.On("GetType").Return(common.NodeTypeEnd)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetNode", "end-node").Return(mockEndNode, true)

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		CurrentNode:    mockPromptNode,
		Graph:          mockGraph,
		AdditionalData: map[string]string{"key": "value"},
	}

	flowStep := &FlowStep{
		Data: FlowData{},
	}

	nodeResp := &common.NodeResponse{
		Status:         common.NodeStatusComplete,
		Meta:           map[string]interface{}{"meta_key": "meta_value"},
		AdditionalData: map[string]string{"response_key": "response_value"},
	}

	nextNode, complete, err := fe.handleDisplayOnlyPromptResponse(ctx, nodeResp, flowStep, nil)

	s.Nil(err)
	s.True(complete, "Should complete flow when forwarding to END node")
	s.Nil(nextNode)
	s.Equal(providers.FlowStatusComplete, flowStep.Status)
	// Context AdditionalData is copied to flowStep
	s.Contains(flowStep.Data.AdditionalData, "key")
	s.Equal("value", flowStep.Data.AdditionalData["key"])
	s.Equal(map[string]interface{}{"meta_key": "meta_value"}, flowStep.Data.Meta)
	// Node response AdditionalData is also merged
	s.Contains(flowStep.Data.AdditionalData, "response_key")
	s.Equal("response_value", flowStep.Data.AdditionalData["response_key"])
}

func (s *EngineTestSuite) TestHandleDisplayOnlyPromptResponse_UnknownNextNode() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	mockPromptNode := coremock.NewPromptNodeInterfaceMock(t)
	mockPromptNode.On("GetNextNode").Return("unknown-node")

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetNode", "unknown-node").Return(nil, false)

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		CurrentNode: mockPromptNode,
		Graph:       mockGraph,
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
	}

	logger := log.GetLogger()
	nextNode, complete, err := fe.handleDisplayOnlyPromptResponse(ctx, nodeResp, nil, logger)

	s.NotNil(err)
	s.False(complete)
	s.Nil(nextNode)
}

func (s *EngineTestSuite) TestHandleDisplayOnlyPromptResponse_MergesAdditionalData() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	mockPromptNode := coremock.NewPromptNodeInterfaceMock(t)
	mockPromptNode.On("GetNextNode").Return("end-node")

	mockEndNode := coremock.NewNodeInterfaceMock(t)
	mockEndNode.On("GetType").Return(common.NodeTypeEnd)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetNode", "end-node").Return(mockEndNode, true)

	fe := &flowEngine{
		observabilitySvc: mockObservability,
	}

	ctx := &EngineContext{
		CurrentNode:    mockPromptNode,
		Graph:          mockGraph,
		AdditionalData: map[string]string{"existing_key": "existing_value"},
	}

	flowStep := &FlowStep{
		Data: FlowData{
			AdditionalData: map[string]string{"flow_key": "flow_value"},
		},
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
		AdditionalData: map[string]string{
			"response_key": "response_value",
		},
	}

	nextNode, complete, err := fe.handleDisplayOnlyPromptResponse(ctx, nodeResp, flowStep, nil)

	s.Nil(err)
	s.True(complete)
	s.Nil(nextNode)
	// Verify merged data
	s.Equal(providers.FlowStatusComplete, flowStep.Status)
}

func (s *EngineTestSuite) TestIsSegmentRestartAllowed_NoSegments() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: "seg-1"}

	s.False(fe.isSegmentRestartAllowed(ctx, fe.logger))
}

func (s *EngineTestSuite) TestIsSegmentRestartAllowed_EmptySegmentID() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(true)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: ""}

	s.False(fe.isSegmentRestartAllowed(ctx, fe.logger))
}

func (s *EngineTestSuite) TestIsSegmentRestartAllowed_SegmentNotFound() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(true)
	mockGraph.On("GetSegmentByID", "seg-1").Return((*core.Segment)(nil))

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: "seg-1"}

	s.False(fe.isSegmentRestartAllowed(ctx, fe.logger))
}

func (s *EngineTestSuite) TestIsSegmentRestartAllowed_StartNodeNotFound() {
	t := s.T()
	seg := &core.Segment{ID: "seg-1", StartNodeID: "task-node"}

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(true)
	mockGraph.On("GetSegmentByID", "seg-1").Return(seg)
	mockGraph.On("GetNode", "task-node").Return(nil, false)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: "seg-1"}

	s.False(fe.isSegmentRestartAllowed(ctx, fe.logger))
}

func (s *EngineTestSuite) TestIsSegmentRestartAllowed_NilPolicy() {
	t := s.T()
	seg := &core.Segment{ID: "seg-1", StartNodeID: "prompt-node"}

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetType").Return(common.NodeTypePrompt)
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil))

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(true)
	mockGraph.On("GetSegmentByID", "seg-1").Return(seg)
	mockGraph.On("GetNode", "prompt-node").Return(mockNode, true)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: "seg-1"}

	s.False(fe.isSegmentRestartAllowed(ctx, fe.logger))
}

func (s *EngineTestSuite) TestIsSegmentRestartAllowed_PolicyAllowsRestartFlag() {
	tests := []struct {
		name          string
		allowRestart  bool
		expectAllowed bool
	}{
		{"policy disallows restart", false, false},
		{"policy allows restart", true, true},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			t := s.T()
			seg := &core.Segment{ID: "seg-1", StartNodeID: "task-node"}
			policy := &providers.ExecutionPolicy{SkipChallengeValidation: true, AllowSegmentRestart: tt.allowRestart}

			mockNode := coremock.NewNodeInterfaceMock(t)
			mockNode.On("GetType").Return(common.NodeTypePrompt)
			mockNode.On("GetExecutionPolicy").Return(policy)

			mockGraph := coremock.NewGraphInterfaceMock(t)
			mockGraph.On("HasSegments").Return(true)
			mockGraph.On("GetSegmentByID", "seg-1").Return(seg)
			mockGraph.On("GetNode", "task-node").Return(mockNode, true)

			fe := &flowEngine{
				logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
			}
			ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: "seg-1"}

			s.Equal(tt.expectAllowed, fe.isSegmentRestartAllowed(ctx, fe.logger))
		})
	}
}

func (s *EngineTestSuite) TestHandleDisplayOnlyPromptResponse_ForwardToNextNode_SetsSegmentID() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	mockPromptNode := coremock.NewPromptNodeInterfaceMock(t)
	mockPromptNode.On("GetNextNode").Return("next-node")

	mockNextNode := coremock.NewNodeInterfaceMock(t)
	mockNextNode.On("GetType").Return(common.NodeTypePrompt)
	mockNextNode.On("GetID").Return("next-node")

	seg := &core.Segment{ID: "seg-1", StartNodeID: "next-node"}

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetNode", "next-node").Return(mockNextNode, true)
	mockGraph.On("HasSegments").Return(true)
	mockGraph.On("GetSegmentByStartNode", "next-node").Return(seg)

	fe := &flowEngine{observabilitySvc: mockObservability}
	ctx := &EngineContext{
		CurrentNode: mockPromptNode,
		Graph:       mockGraph,
	}

	_, complete, err := fe.handleDisplayOnlyPromptResponse(ctx, &common.NodeResponse{
		Status: common.NodeStatusComplete,
	}, &FlowStep{Data: FlowData{}}, nil)

	s.Nil(err)
	s.False(complete)
	s.Equal("seg-1", ctx.CurrentSegmentID)
}

func (s *EngineTestSuite) TestHandleDisplayOnlyPromptResponse_ForwardToNextNode_SegmentNotFound_KeepsEmptyID() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	mockPromptNode := coremock.NewPromptNodeInterfaceMock(t)
	mockPromptNode.On("GetNextNode").Return("next-node")

	mockNextNode := coremock.NewNodeInterfaceMock(t)
	mockNextNode.On("GetType").Return(common.NodeTypePrompt)
	mockNextNode.On("GetID").Return("next-node")

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetNode", "next-node").Return(mockNextNode, true)
	mockGraph.On("HasSegments").Return(true)
	mockGraph.On("GetSegmentByStartNode", "next-node").Return((*core.Segment)(nil))

	fe := &flowEngine{observabilitySvc: mockObservability}
	ctx := &EngineContext{
		CurrentNode: mockPromptNode,
		Graph:       mockGraph,
	}

	_, complete, err := fe.handleDisplayOnlyPromptResponse(ctx, &common.NodeResponse{
		Status: common.NodeStatusComplete,
	}, &FlowStep{Data: FlowData{}}, nil)

	s.Nil(err)
	s.False(complete)
	s.Equal("", ctx.CurrentSegmentID)
}

func (s *EngineTestSuite) TestPublishNodeExecutionCompletedEvent_NodeRespErrorPublished() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(true)

	var capturedEvent *providers.Event
	mockObservability.On("PublishEvent", mock.Anything, mock.AnythingOfType("*providers.Event")).
		Run(func(args mock.Arguments) {
			capturedEvent = args.Get(1).(*providers.Event)
		}).Return()

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("test-node")
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)

	ctx := &EngineContext{
		ExecutionID: "exec-123",
		FlowType:    providers.FlowTypeAuthentication,
		AppID:       "app-456",
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{
			"test-node": {
				Step: 1,
				Executions: []providers.ExecutionAttempt{
					{Attempt: 1},
				},
			},
		},
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusIncomplete,
		Error: &tidcommon.ServiceError{
			Code: "FET-1008",
			Error: tidcommon.I18nMessage{
				Key:          "flows.executor.errors.invalid_otp",
				DefaultValue: "Invalid OTP provided",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key:          "flows.executor.errors.invalid_otp_desc",
				DefaultValue: "The one-time password provided is invalid or has expired",
			},
		},
	}

	publishNodeExecutionCompletedEvent(ctx, mockNode, nodeResp, nil, 1000, 2000, mockObservability)

	s.NotNil(capturedEvent)
	s.Equal(string(event.EventTypeFlowNodeExecutionCompleted), capturedEvent.Type)
	s.Equal(providers.StatusSuccess, capturedEvent.Status)

	errorData, ok := capturedEvent.Data[event.DataKey.Error].(map[string]interface{})
	s.True(ok)
	s.Equal("FET-1008", errorData["code"])

	message, ok := errorData["message"].(map[string]string)
	s.True(ok)
	s.Equal("flows.executor.errors.invalid_otp", message["key"])
	s.Equal("Invalid OTP provided", message["defaultValue"])

	description, ok := errorData["description"].(map[string]string)
	s.True(ok)
	s.Equal("flows.executor.errors.invalid_otp_desc", description["key"])
	s.Equal("The one-time password provided is invalid or has expired", description["defaultValue"])
}

func (s *EngineTestSuite) TestPublishNodeExecutionCompletedEvent_NodeErrTakesPrecedenceOverNodeRespError() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(true)

	var capturedEvent *providers.Event
	mockObservability.On("PublishEvent", mock.Anything, mock.AnythingOfType("*providers.Event")).
		Run(func(args mock.Arguments) {
			capturedEvent = args.Get(1).(*providers.Event)
		}).Return()

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("test-node")
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)

	ctx := &EngineContext{
		ExecutionID: "exec-123",
		FlowType:    providers.FlowTypeAuthentication,
		AppID:       "app-456",
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{
			"test-node": {
				Step:       1,
				Executions: []providers.ExecutionAttempt{{Attempt: 1}},
			},
		},
	}

	nodeErr := &tidcommon.ServiceError{
		Code: "SVC-50001",
		Error: tidcommon.I18nMessage{
			Key:          "service.errors.internal",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "service.errors.internal_desc",
			DefaultValue: "An unexpected error occurred",
		},
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusFailure,
		Error: &tidcommon.ServiceError{
			Code: "FET-1008",
			Error: tidcommon.I18nMessage{
				Key:          "flows.executor.errors.invalid_otp",
				DefaultValue: "Invalid OTP provided",
			},
		},
	}

	publishNodeExecutionCompletedEvent(ctx, mockNode, nodeResp, nodeErr, 1000, 2000, mockObservability)

	s.NotNil(capturedEvent)
	s.Equal(string(event.EventTypeFlowNodeExecutionFailed), capturedEvent.Type)
	s.Equal(providers.StatusFailure, capturedEvent.Status)

	errorData, ok := capturedEvent.Data[event.DataKey.Error].(map[string]interface{})
	s.True(ok)
	s.Equal("SVC-50001", errorData["code"], "nodeErr should take precedence over nodeResp.Error")
}

func (s *EngineTestSuite) TestPublishNodeExecutionCompletedEvent_NoErrorPublishedWhenBothNil() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(true)

	var capturedEvent *providers.Event
	mockObservability.On("PublishEvent", mock.Anything, mock.AnythingOfType("*providers.Event")).
		Run(func(args mock.Arguments) {
			capturedEvent = args.Get(1).(*providers.Event)
		}).Return()

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("test-node")
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)

	ctx := &EngineContext{
		ExecutionID: "exec-123",
		FlowType:    providers.FlowTypeAuthentication,
		AppID:       "app-456",
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{
			"test-node": {
				Step:       1,
				Executions: []providers.ExecutionAttempt{{Attempt: 1}},
			},
		},
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
	}

	publishNodeExecutionCompletedEvent(ctx, mockNode, nodeResp, nil, 1000, 2000, mockObservability)

	s.NotNil(capturedEvent)
	s.Equal(string(event.EventTypeFlowNodeExecutionCompleted), capturedEvent.Type)
	s.Equal(providers.StatusSuccess, capturedEvent.Status)
	_, hasError := capturedEvent.Data[event.DataKey.Error]
	s.False(hasError)
}

func (s *EngineTestSuite) TestProcessNodeResponseErrorForEventPublish_ReturnsErrorDetails() {
	nodeResp := &common.NodeResponse{
		Error: &tidcommon.ServiceError{
			Code: "FET-1021",
			Error: tidcommon.I18nMessage{
				Key:          "flows.executor.errors.auth_failed",
				DefaultValue: "Authentication failed",
			},
			ErrorDescription: tidcommon.I18nMessage{
				Key:          "flows.executor.errors.auth_failed_desc",
				DefaultValue: "The authentication attempt was unsuccessful",
			},
		},
	}

	result := processNodeResponseErrorForEventPublish(nodeResp)

	s.NotNil(result)
	s.Equal("FET-1021", result["code"])

	message, ok := result["message"].(map[string]string)
	s.True(ok)
	s.Equal("flows.executor.errors.auth_failed", message["key"])
	s.Equal("Authentication failed", message["defaultValue"])

	description, ok := result["description"].(map[string]string)
	s.True(ok)
	s.Equal("flows.executor.errors.auth_failed_desc", description["key"])
	s.Equal("The authentication attempt was unsuccessful", description["defaultValue"])
}

func (s *EngineTestSuite) TestProcessNodeResponseErrorForEventPublish_NilNodeResponse() {
	result := processNodeResponseErrorForEventPublish(nil)
	s.Nil(result)
}

func (s *EngineTestSuite) TestProcessNodeResponseErrorForEventPublish_NilError() {
	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
	}

	result := processNodeResponseErrorForEventPublish(nodeResp)
	s.Nil(result)
}

func (s *EngineTestSuite) TestProcessServiceErrorForEventPublish_ReturnsErrorDetails() {
	svcErr := &tidcommon.ServiceError{
		Code: "SVC-50001",
		Error: tidcommon.I18nMessage{
			Key:          "service.errors.internal",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "service.errors.internal_desc",
			DefaultValue: "An unexpected error occurred",
		},
	}

	result := processServiceErrorForEventPublish(svcErr)

	s.NotNil(result)
	s.Equal("SVC-50001", result["code"])

	message, ok := result["message"].(map[string]string)
	s.True(ok)
	s.Equal("service.errors.internal", message["key"])
	s.Equal("Internal server error", message["defaultValue"])

	description, ok := result["description"].(map[string]string)
	s.True(ok)
	s.Equal("service.errors.internal_desc", description["key"])
	s.Equal("An unexpected error occurred", description["defaultValue"])
}

func (s *EngineTestSuite) TestProcessServiceErrorForEventPublish_NilError() {
	result := processServiceErrorForEventPublish(nil)
	s.Nil(result)
}

// --- newFlowEngine ---

func (s *EngineTestSuite) TestNewFlowEngine() {
	t := s.T()
	mockRegistry := executormock.NewExecutorRegistryInterfaceMock(t)
	mockInterceptorRunner := NewInterceptorRunnerInterfaceMock(t)
	mockObs := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)

	engine := newFlowEngine(mockRegistry, mockInterceptorRunner, mockObs, mockFlowProvider, mockGraphBuilder)
	s.NotNil(engine)
}

// --- setCurrentExecutionNode ---

func (s *EngineTestSuite) TestSetCurrentExecutionNode_NilGraph() {
	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background()}
	err := fe.setCurrentExecutionNode(ctx, log.GetLogger())
	s.NotNil(err)
}

func (s *EngineTestSuite) TestSetCurrentExecutionNode_GetStartNodeError() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetStartNode").Return(nil, errors.New("no start node"))

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context: context.Background(),
		Graph:   mockGraph,
	}
	err := fe.setCurrentExecutionNode(ctx, log.GetLogger())
	s.NotNil(err)
}

func (s *EngineTestSuite) TestSetCurrentExecutionNode_ExistingNode() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context:          context.Background(),
		Graph:            mockGraph,
		CurrentNode:      mockNode,
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}
	err := fe.setCurrentExecutionNode(ctx, log.GetLogger())
	s.Nil(err)
}

// --- getExecutorByName ---

func (s *EngineTestSuite) TestGetExecutorByName_Success() {
	t := s.T()
	mockRegistry := executormock.NewExecutorRegistryInterfaceMock(t)
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockRegistry.EXPECT().GetExecutor("myExecutor").Return(mockExec, nil)

	fe := &flowEngine{executorRegistry: mockRegistry, logger: log.GetLogger()}
	exec, err := fe.getExecutorByName("myExecutor")
	s.Nil(err)
	s.Equal(mockExec, exec)
}

func (s *EngineTestSuite) TestGetExecutorByName_Error() {
	t := s.T()
	mockRegistry := executormock.NewExecutorRegistryInterfaceMock(t)
	mockRegistry.EXPECT().GetExecutor("unknown").Return(nil, errors.New("not found"))

	fe := &flowEngine{executorRegistry: mockRegistry, logger: log.GetLogger()}
	exec, err := fe.getExecutorByName("unknown")
	s.Nil(exec)
	s.NotNil(err)
}

// --- resolveToNextNode ---

func (s *EngineTestSuite) TestResolveToNextNode_NilResponse() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), Graph: mockGraph}

	next, err := fe.resolveToNextNode(ctx, nil)
	s.Nil(err)
	s.Nil(next)
}

func (s *EngineTestSuite) TestResolveToNextNode_EmptyNextNodeID() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), Graph: mockGraph}

	next, err := fe.resolveToNextNode(ctx, &common.NodeResponse{NextNodeID: ""})
	s.Nil(err)
	s.Nil(next)
}

func (s *EngineTestSuite) TestResolveToNextNode_NodeNotFound() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetNode", "missing-node").Return(nil, false)
	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), Graph: mockGraph}

	next, err := fe.resolveToNextNode(ctx, &common.NodeResponse{NextNodeID: "missing-node"})
	s.Nil(next)
	s.NotNil(err)
}

func (s *EngineTestSuite) TestResolveToNextNode_Success() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("next-node")
	mockGraph.On("GetNode", "next-node").Return(mockNode, true)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), Graph: mockGraph}

	next, err := fe.resolveToNextNode(ctx, &common.NodeResponse{NextNodeID: "next-node"})
	s.Nil(err)
	s.Equal(mockNode, next)
}

// --- handleCompletedResponse ---

func (s *EngineTestSuite) TestHandleCompletedResponse_NoNextNode() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetNode", mock.Anything).Return(nil, false)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context: context.Background(),
		Graph:   mockGraph,
	}
	nodeResp := &common.NodeResponse{
		Status:     common.NodeStatusComplete,
		NextNodeID: "end-node",
	}
	next, err := fe.handleCompletedResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(next)
	s.NotNil(err)
}

func (s *EngineTestSuite) TestHandleCompletedResponse_WithNextNode() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("next-node")
	mockGraph.On("GetNode", "next-node").Return(mockNode, true)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context: context.Background(),
		Graph:   mockGraph,
	}
	nodeResp := &common.NodeResponse{
		Status:     common.NodeStatusComplete,
		NextNodeID: "next-node",
	}
	next, err := fe.handleCompletedResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(err)
	s.Equal(mockNode, next)
}

// --- handleIncompleteResponse ---

func (s *EngineTestSuite) TestHandleIncompleteResponse_RedirectionType() {
	fe := &flowEngine{logger: log.GetLogger()}
	flowStep := &FlowStep{}
	ctx := &EngineContext{Context: context.Background()}
	nodeResp := &common.NodeResponse{
		Status:      common.NodeStatusIncomplete,
		Type:        common.NodeResponseTypeRedirection,
		RedirectURL: "https://example.com/redirect",
	}
	err := fe.handleIncompleteResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(err)
	s.Equal(providers.FlowStatusIncomplete, flowStep.Status)
	s.Equal(common.StepTypeRedirection, flowStep.Type)
}

func (s *EngineTestSuite) TestHandleIncompleteResponse_UnsupportedType() {
	fe := &flowEngine{logger: log.GetLogger()}
	flowStep := &FlowStep{}
	ctx := &EngineContext{Context: context.Background()}
	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusIncomplete,
		Type:   "UNSUPPORTED_TYPE",
	}
	err := fe.handleIncompleteResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.NotNil(err)
}

// --- handleForwardResponse ---

func (s *EngineTestSuite) TestHandleForwardResponse_Success() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("forward-node")
	mockGraph.On("GetNode", "forward-node").Return(mockNode, true)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context: context.Background(),
		Graph:   mockGraph,
	}
	nodeResp := &common.NodeResponse{
		Status:     common.NodeStatusForward,
		NextNodeID: "forward-node",
	}
	next, err := fe.handleForwardResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(err)
	s.Equal(mockNode, next)
}

func (s *EngineTestSuite) TestHandleForwardResponse_NodeNotFound() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetNode", "missing").Return(nil, false)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context: context.Background(),
		Graph:   mockGraph,
	}
	nodeResp := &common.NodeResponse{
		Status:     common.NodeStatusForward,
		NextNodeID: "missing",
	}
	next, err := fe.handleForwardResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(next)
	s.NotNil(err)
}

// --- skipToNextNode ---

func (s *EngineTestSuite) TestSkipToNextNode_NoCondition() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetCondition").Return((*core.NodeCondition)(nil))
	mockNode.On("GetID").Return("node-1")

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background()}

	next, err := fe.skipToNextNode(ctx, mockNode, log.GetLogger())
	s.Nil(next)
	s.NotNil(err)
}

func (s *EngineTestSuite) TestSkipToNextNode_EmptyOnSkip() {
	t := s.T()
	cond := &core.NodeCondition{OnSkip: ""}
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetCondition").Return(cond)
	mockNode.On("GetID").Return("node-1")

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background()}

	next, err := fe.skipToNextNode(ctx, mockNode, log.GetLogger())
	s.Nil(next)
	s.NotNil(err)
}

func (s *EngineTestSuite) TestSkipToNextNode_NodeNotFound() {
	t := s.T()
	cond := &core.NodeCondition{OnSkip: "skip-target"}
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetCondition").Return(cond)
	mockNode.On("GetID").Return("node-1")

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("GetNode", "skip-target").Return(nil, false)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), Graph: mockGraph}

	next, err := fe.skipToNextNode(ctx, mockNode, log.GetLogger())
	s.Nil(next)
	s.NotNil(err)
}

// --- processNodeResponse ---

func (s *EngineTestSuite) TestProcessNodeResponse_EmptyStatus() {
	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background()}
	nodeResp := &common.NodeResponse{Status: ""}
	flowStep := &FlowStep{}
	_, _, err := fe.processNodeResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.NotNil(err)
}

func (s *EngineTestSuite) TestProcessNodeResponse_FailureStatus() {
	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background()}
	svcErr := &tidcommon.ServiceError{Code: "err-1"}
	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusFailure,
		Error:  svcErr,
	}
	flowStep := &FlowStep{}
	next, continueExec, err := fe.processNodeResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(err)
	s.False(continueExec)
	s.Nil(next)
	s.Equal(providers.FlowStatusError, flowStep.Status)
}

func (s *EngineTestSuite) TestProcessNodeResponse_CompleteStatus() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockNextNode := coremock.NewNodeInterfaceMock(t)
	mockNextNode.On("GetID").Return("next-node")
	mockGraph.On("GetNode", "next-node").Return(mockNextNode, true)

	// Use a NodeInterface mock — type assertion to PromptNodeInterface will fail (it's not one),
	// so isDisplayOnlyPromptNode returns false without calling GetType.
	mockCurrentNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context:     context.Background(),
		Graph:       mockGraph,
		CurrentNode: mockCurrentNode,
	}
	nodeResp := &common.NodeResponse{
		Status:     common.NodeStatusComplete,
		NextNodeID: "next-node",
	}
	flowStep := &FlowStep{}
	next, continueExec, err := fe.processNodeResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(err)
	s.True(continueExec)
	s.Equal(mockNextNode, next)
}

func (s *EngineTestSuite) TestProcessNodeResponse_IncompleteViewStatus() {
	t := s.T()
	mockCurrentNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context:     context.Background(),
		CurrentNode: mockCurrentNode,
	}
	nodeResp := &common.NodeResponse{
		Status:      common.NodeStatusIncomplete,
		Type:        common.NodeResponseTypeRedirection,
		RedirectURL: "https://example.com",
	}
	flowStep := &FlowStep{}
	next, continueExec, err := fe.processNodeResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(err)
	s.False(continueExec)
	s.Nil(next)
}

func (s *EngineTestSuite) TestProcessNodeResponse_UnsupportedStatus() {
	t := s.T()
	mockCurrentNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context:     context.Background(),
		CurrentNode: mockCurrentNode,
	}
	nodeResp := &common.NodeResponse{
		Status: "UNKNOWN_STATUS",
	}
	flowStep := &FlowStep{}
	_, _, err := fe.processNodeResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.NotNil(err)
}

// --- recordNodeExecution ---

func (s *EngineTestSuite) TestRecordNodeExecution_NewRecord() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1")
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)

	ctx := &EngineContext{
		Context:          context.Background(),
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}
	nodeResp := &common.NodeResponse{Status: common.NodeStatusComplete}
	recordNodeExecution(ctx, mockNode, nodeResp, nil, 100, 200)

	record, exists := ctx.ExecutionHistory["node-1"]
	s.True(exists)
	s.NotNil(record)
	s.Equal(1, len(record.Executions))
}

func (s *EngineTestSuite) TestRecordNodeExecution_ExistingRecord() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1")

	existing := &providers.NodeExecutionRecord{
		NodeID:     "node-1",
		Executions: make([]providers.ExecutionAttempt, 0),
	}
	ctx := &EngineContext{
		Context: context.Background(),
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{
			"node-1": existing,
		},
	}
	nodeResp := &common.NodeResponse{Status: common.NodeStatusIncomplete}
	recordNodeExecution(ctx, mockNode, nodeResp, nil, 100, 200)

	s.Equal(1, len(existing.Executions))
}

// --- createExecutionRecord ---

func (s *EngineTestSuite) TestCreateExecutionRecord_BasicNode() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1")
	// Return a non-task-execution type so we skip executor-related code
	mockNode.On("GetType").Return(common.NodeTypePrompt)

	record := createExecutionRecord(mockNode, 1)
	s.Equal("node-1", record.NodeID)
	s.Equal(1, record.Step)
}

// --- createExecutionAttempt ---

func (s *EngineTestSuite) TestCreateExecutionAttempt_WithError() {
	nodeRecord := &providers.NodeExecutionRecord{Executions: make([]providers.ExecutionAttempt, 0)}
	svcErr := &tidcommon.ServiceError{Code: "err-1"}

	attempt := createExecutionAttempt(nodeRecord, nil, svcErr, 100, 200)
	s.Equal(providers.FlowStatusError, attempt.Status)
	s.Equal(1, attempt.Attempt)
}

func (s *EngineTestSuite) TestCreateExecutionAttempt_CompleteResponse() {
	nodeRecord := &providers.NodeExecutionRecord{Executions: make([]providers.ExecutionAttempt, 0)}
	nodeResp := &common.NodeResponse{Status: common.NodeStatusComplete}

	attempt := createExecutionAttempt(nodeRecord, nodeResp, nil, 100, 200)
	s.Equal(providers.FlowStatusComplete, attempt.Status)
}

func (s *EngineTestSuite) TestCreateExecutionAttempt_IncompleteResponse() {
	nodeRecord := &providers.NodeExecutionRecord{Executions: make([]providers.ExecutionAttempt, 0)}
	nodeResp := &common.NodeResponse{Status: common.NodeStatusIncomplete}

	attempt := createExecutionAttempt(nodeRecord, nodeResp, nil, 100, 200)
	s.Equal(providers.FlowStatusIncomplete, attempt.Status)
}

func (s *EngineTestSuite) TestCreateExecutionAttempt_FailureResponse() {
	nodeRecord := &providers.NodeExecutionRecord{Executions: make([]providers.ExecutionAttempt, 0)}
	nodeResp := &common.NodeResponse{Status: common.NodeStatusFailure}

	attempt := createExecutionAttempt(nodeRecord, nodeResp, nil, 100, 200)
	s.Equal(providers.FlowStatusError, attempt.Status)
}

// --- setNodeExecutor ---

func (s *EngineTestSuite) TestSetNodeExecutor_NonTaskNode() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetType").Return(common.NodeTypePrompt)

	fe := &flowEngine{logger: log.GetLogger()}
	err := fe.setNodeExecutor(context.Background(), mockNode, log.GetLogger())
	s.Nil(err)
}

func (s *EngineTestSuite) TestSetNodeExecutor_NotExecutorBacked() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)
	mockNode.On("GetID").Return("node-1")

	fe := &flowEngine{logger: log.GetLogger()}
	err := fe.setNodeExecutor(context.Background(), mockNode, log.GetLogger())
	s.NotNil(err)
}

func (s *EngineTestSuite) TestSetNodeExecutor_ExecutorAlreadySet() {
	t := s.T()
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)
	mockNode.On("GetExecutor").Return(mockExec)

	fe := &flowEngine{logger: log.GetLogger()}
	err := fe.setNodeExecutor(context.Background(), mockNode, log.GetLogger())
	s.Nil(err)
}

func (s *EngineTestSuite) TestSetNodeExecutor_ExecutorNameEmpty() {
	t := s.T()
	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)
	mockNode.On("GetExecutor").Return(nil)
	mockNode.On("GetExecutorName").Return("")
	mockNode.On("GetID").Return("node-1")

	fe := &flowEngine{logger: log.GetLogger()}
	err := fe.setNodeExecutor(context.Background(), mockNode, log.GetLogger())
	s.NotNil(err)
}

func (s *EngineTestSuite) TestSetNodeExecutor_RegistryError() {
	t := s.T()
	mockRegistry := executormock.NewExecutorRegistryInterfaceMock(t)
	mockRegistry.EXPECT().GetExecutor("myExec").Return(nil, errors.New("not found"))

	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)
	mockNode.On("GetExecutor").Return(nil)
	mockNode.On("GetExecutorName").Return("myExec")
	mockNode.On("GetID").Return("node-1")

	fe := &flowEngine{executorRegistry: mockRegistry, logger: log.GetLogger()}
	err := fe.setNodeExecutor(context.Background(), mockNode, log.GetLogger())
	s.NotNil(err)
}

// --- handleIncompleteResponse (view type) ---

func (s *EngineTestSuite) TestHandleIncompleteResponse_ViewType() {
	t := s.T()
	mockCurrentNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context:     context.Background(),
		CurrentNode: mockCurrentNode,
	}
	flowStep := &FlowStep{}
	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusIncomplete,
		Type:   common.NodeResponseTypeView,
		Inputs: []providers.Input{{Identifier: "username", Required: true}},
	}
	err := fe.handleIncompleteResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(err)
	s.Equal(providers.FlowStatusIncomplete, flowStep.Status)
}

func (s *EngineTestSuite) TestHandleIncompleteResponse_RedirectionError() {
	fe := &flowEngine{logger: log.GetLogger()}
	flowStep := &FlowStep{}
	ctx := &EngineContext{Context: context.Background()}
	// Empty RedirectURL will cause resolveStepForRedirection to return an error.
	nodeResp := &common.NodeResponse{
		Status:      common.NodeStatusIncomplete,
		Type:        common.NodeResponseTypeRedirection,
		RedirectURL: "",
	}
	err := fe.handleIncompleteResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.NotNil(err)
}

func (s *EngineTestSuite) TestHandleIncompleteResponse_ViewTypeError() {
	t := s.T()
	mockCurrentNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context:     context.Background(),
		CurrentNode: mockCurrentNode,
	}
	flowStep := &FlowStep{}
	// Empty Inputs and Actions will cause resolveStepDetailsForPrompt to return an error.
	nodeResp := &common.NodeResponse{
		Status:  common.NodeStatusIncomplete,
		Type:    common.NodeResponseTypeView,
		Inputs:  nil,
		Actions: nil,
	}
	err := fe.handleIncompleteResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.NotNil(err)
}

// --- createExecutionRecord with executor ---

func (s *EngineTestSuite) TestCreateExecutionRecord_TaskNodeWithExecutor() {
	t := s.T()
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return("PasswordExecutor")
	mockExec.On("GetType").Return(providers.ExecutorType("AUTHENTICATOR"))

	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-exec")
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)
	mockNode.On("GetExecutor").Return(mockExec)
	mockNode.On("GetMode").Return("").Maybe()

	record := createExecutionRecord(mockNode, 2)
	s.Equal("node-exec", record.NodeID)
	s.Equal(2, record.Step)
	s.Equal("PasswordExecutor", record.ExecutorName)
}

// --- processNodeResponse (Forward status) ---

func (s *EngineTestSuite) TestProcessNodeResponse_ForwardStatus() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockNextNode := coremock.NewNodeInterfaceMock(t)
	mockNextNode.On("GetID").Return("forward-node")
	mockGraph.On("GetNode", "forward-node").Return(mockNextNode, true)

	mockCurrentNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context:     context.Background(),
		Graph:       mockGraph,
		CurrentNode: mockCurrentNode,
	}
	nodeResp := &common.NodeResponse{
		Status:     common.NodeStatusForward,
		NextNodeID: "forward-node",
	}
	flowStep := &FlowStep{}
	next, continueExec, err := fe.processNodeResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(err)
	s.True(continueExec)
	s.Equal(mockNextNode, next)
}

// --- setCurrentExecutionNode (with nil history) ---

func (s *EngineTestSuite) TestSetCurrentExecutionNode_InitializesHistory() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockGraph.On("GetStartNode").Return(mockNode, nil)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context:          context.Background(),
		Graph:            mockGraph,
		ExecutionHistory: nil,
	}
	err := fe.setCurrentExecutionNode(ctx, log.GetLogger())
	s.Nil(err)
	s.NotNil(ctx.ExecutionHistory)
	s.Equal(mockNode, ctx.CurrentNode)
}

// --- handleForwardResponse with error in nodeResp ---

func (s *EngineTestSuite) TestHandleForwardResponse_WithErrorMsg() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockNextNode := coremock.NewNodeInterfaceMock(t)
	mockNextNode.On("GetID").Return("next")
	mockGraph.On("GetNode", "next").Return(mockNextNode, true)

	svcErr := &tidcommon.ServiceError{Code: "err-1"}
	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context: context.Background(),
		Graph:   mockGraph,
	}
	nodeResp := &common.NodeResponse{
		Status:     common.NodeStatusForward,
		NextNodeID: "next",
		Error:      svcErr,
	}
	next, err := fe.handleForwardResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(err)
	s.Equal(mockNextNode, next)
}

// --- publishNodeExecutionCompletedEvent ---

func (s *EngineTestSuite) TestPublishNodeExecutionCompletedEvent_NilRecord() {
	t := s.T()
	mockObs := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObs.EXPECT().IsEnabled().Return(true)
	// No PublishEvent call expected because record is nil.

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-no-record")

	ctx := &EngineContext{
		Context:          context.Background(),
		ExecutionID:      "exec-1",
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}
	// Calling with nil nodeResp and nil nodeErr, but no history entry for the node.
	publishNodeExecutionCompletedEvent(ctx, mockNode, nil, nil, 1000, 1100, mockObs)
}

func (s *EngineTestSuite) TestPublishNodeExecutionCompletedEvent_DefaultStatus() {
	t := s.T()
	mockObs := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObs.EXPECT().IsEnabled().Return(true)
	mockObs.EXPECT().PublishEvent(mock.Anything, mock.Anything).Maybe()

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-default")
	mockNode.On("GetType").Return(common.NodeTypePrompt)

	ctx := &EngineContext{
		Context:     context.Background(),
		ExecutionID: "exec-1",
		FlowType:    providers.FlowTypeAuthentication,
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{
			"node-default": {NodeID: "node-default", Step: 1, Executions: []providers.ExecutionAttempt{{Attempt: 1}}},
		},
	}
	// Use an unrecognized NodeStatus to hit the default branch.
	nodeResp := &common.NodeResponse{Status: common.NodeStatus("UNKNOWN_STATUS")}
	publishNodeExecutionCompletedEvent(ctx, mockNode, nodeResp, nil, 1000, 1100, mockObs)
}

// --- Execute (flowEngine) ---

// executeNodePackage coverage

// setupNodePackageMockObs returns an observability mock that reports disabled so publish
// helpers short-circuit; the engine still calls IsEnabled.
func setupNodePackageMockObs(t *testing.T) *observabilitymock.ObservabilityServiceInterfaceMock {
	m := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	m.EXPECT().IsEnabled().Return(false).Maybe()
	return m
}

func (s *EngineTestSuite) TestExecuteNodePackage_SkipsNodeWhenShouldExecuteFalse() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("n1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeStart).Maybe()
	mockNode.On("ShouldExecute", mock.Anything).Return(false)
	mockNode.On("GetCondition").Return(&core.NodeCondition{OnSkip: ""}).Maybe()

	fe := &flowEngine{
		logger:           log.GetLogger(),
		observabilitySvc: setupNodePackageMockObs(t),
	}
	ctx := &EngineContext{
		Context:          context.Background(),
		ExecutionID:      "exec-skip",
		CurrentNode:      mockNode,
		UserInputs:       map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}

	nextNode, exit, err := fe.executeNodePackage(ctx, mockNode, &FlowStep{}, 0)

	s.Nil(nextNode)
	s.False(exit)
	s.NotNil(err, "missing OnSkip target should surface an internal server error")
}

func (s *EngineTestSuite) TestExecuteNodePackage_CompletesAndReturnsNilNextNode() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("n1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeStart).Maybe()
	mockNode.On("ShouldExecute", mock.Anything).Return(true)
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("Execute", mock.Anything).Return(&common.NodeResponse{
		Status: common.NodeStatusComplete,
	}, nil)

	fe := &flowEngine{
		logger:           log.GetLogger(),
		observabilitySvc: setupNodePackageMockObs(t),
	}
	ctx := &EngineContext{
		Context:          context.Background(),
		ExecutionID:      "exec-happy",
		CurrentNode:      mockNode,
		UserInputs:       map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}

	nextNode, exit, err := fe.executeNodePackage(ctx, mockNode, &FlowStep{}, 0)

	s.Nil(err)
	s.False(exit)
	s.Nil(nextNode, "empty NextNodeID resolves to nil")
}

func (s *EngineTestSuite) TestExecuteNodePackage_NodeExecuteErrorPropagates() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("n1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeStart).Maybe()
	mockNode.On("ShouldExecute", mock.Anything).Return(true)
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	execErr := &tidcommon.ServiceError{Code: "boom"}
	mockNode.On("Execute", mock.Anything).Return((*common.NodeResponse)(nil), execErr)

	fe := &flowEngine{
		logger:           log.GetLogger(),
		observabilitySvc: setupNodePackageMockObs(t),
	}
	ctx := &EngineContext{
		Context:          context.Background(),
		ExecutionID:      "exec-err",
		CurrentNode:      mockNode,
		UserInputs:       map[string]string{},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}

	nextNode, exit, err := fe.executeNodePackage(ctx, mockNode, &FlowStep{}, 0)

	s.Nil(nextNode)
	s.False(exit)
	s.Same(execErr, err)
}

func (s *EngineTestSuite) TestExecuteNodePackage_IncompleteExitClearsNodeScopeBeforeRequestScope() {
	t := s.T()

	// Interceptor declares "captcha" as OneTimeUse and the node signals it consumed on Execute.
	interceptorMock := coremock.NewInterceptorInterfaceMock(t)
	interceptorMock.On("GetInputs").Return([]providers.Input{
		{Identifier: "captcha", Type: "TEXT_INPUT", OneTimeUse: true},
	}).Maybe()

	unit := newTestInterceptorUnitMock(t, "captcha-intr", providers.InterceptorModePreNode,
		providers.InterceptorScopeAll, nil)
	unit.SetInterceptor(interceptorMock)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false).Maybe()
	mockGraph.On("GetID").Return("flow-incomplete").Maybe()
	mockGraph.On("GetInterceptors", providers.InterceptorModePreNode).
		Return([]core.InterceptorUnitInterface{unit}).Maybe()
	mockGraph.On("GetInterceptors", providers.InterceptorModePostNode).
		Return([]core.InterceptorUnitInterface{}).Maybe()
	mockGraph.On("GetInterceptors", providers.InterceptorModePostRequest).
		Return([]core.InterceptorUnitInterface{}).Maybe()

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("n1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeStart).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()
	mockNode.On("ShouldExecute", mock.Anything).Return(true)
	mockNode.On("Execute", mock.Anything).Run(func(args mock.Arguments) {
		nodeCtx := args.Get(0).(*providers.NodeContext)
		nodeCtx.AppendConsumedInputs([]string{"captcha"})
	}).Return(&common.NodeResponse{
		Status: common.NodeStatusIncomplete,
		Type:   common.NodeResponseTypeView,
		Inputs: []providers.Input{{Identifier: "email", Required: true}},
	}, nil)

	mockRunner := NewInterceptorRunnerInterfaceMock(t)
	mockRunner.On("runInterceptors", mock.Anything, mock.Anything).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	fe := &flowEngine{
		logger:            log.GetLogger(),
		observabilitySvc:  setupNodePackageMockObs(t),
		interceptorRunner: mockRunner,
	}
	ctx := &EngineContext{
		Context:          context.Background(),
		ExecutionID:      "exec-incomplete",
		Graph:            mockGraph,
		CurrentNode:      mockNode,
		UserInputs:       map[string]string{"captcha": "abc", "email": "e@x"},
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}

	nextNode, exit, err := fe.executeNodePackage(ctx, mockNode, &FlowStep{}, 0)

	s.Nil(err)
	s.True(exit, "incomplete node response must signal flow exit")
	s.Nil(nextNode)
	_, captchaStillHere := ctx.UserInputs["captcha"]
	s.False(captchaStillHere,
		"node-scope OneTimeUse input must be cleared before request-scope cleanup wipes the list")
	s.Equal("e@x", ctx.UserInputs["email"], "non-OneTimeUse input should remain")
	s.Empty(ctx.consumedInputs, "consumed list should be fully drained at this exit path")
}

func (s *EngineTestSuite) TestFlowEngineExecute_NilGraph() {
	t := s.T()
	mockObs := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObs.EXPECT().IsEnabled().Return(false).Maybe()

	fe := &flowEngine{
		logger:           log.GetLogger(),
		observabilitySvc: mockObs,
	}
	ctx := &EngineContext{
		Context:          context.Background(),
		ExecutionID:      "exec-1",
		ExecutionHistory: map[string]*providers.NodeExecutionRecord{},
	}

	_, err := fe.Execute(ctx)
	s.NotNil(err)
}

// Tests for runInterceptors

func (s *EngineTestSuite) TestRunInterceptors_NilGraph() {
	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	ctx := &EngineContext{
		Graph: nil,
	}

	continueExec, err := fe.runInterceptors(providers.InterceptorModePreRequest, ctx, nil, &FlowStep{})

	s.True(continueExec)
	s.Nil(err)
}

func (s *EngineTestSuite) TestRunInterceptors_PreRequest_Success() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID:           "exec-001",
		AppID:                 "app-001",
		FlowType:              providers.FlowTypeAuthentication,
		CurrentNode:           mockNode,
		Graph:                 mockGraph,
		RuntimeData:           map[string]string{"existingKey": "existingValue"},
		InterceptorSharedData: map[string]string{"shared": "data"},
	}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePreRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return(&common.InterceptorResponse{
			Status:        common.InterceptorStatusComplete,
			EngineOutputs: map[string]string{"outputKey": "outputValue"},
		}, nil)

	continueExec, err := fe.runInterceptors(providers.InterceptorModePreRequest, ctx, nil, &FlowStep{})

	s.True(continueExec)
	s.Nil(err)
	s.Equal("existingValue", ctx.RuntimeData["existingKey"])
	s.Equal("outputValue", ctx.RuntimeData["outputKey"])
}

func (s *EngineTestSuite) TestRunInterceptors_ReturnsError() {
	tests := []struct {
		name        string
		mode        providers.InterceptorMode
		executionID string
		errCode     string
		errKey      string
		errDefault  string
	}{
		{
			name:        "PreRequest",
			mode:        providers.InterceptorModePreRequest,
			executionID: "exec-002",
			errCode:     "INT-001",
			errKey:      "error.interceptor.failed",
			errDefault:  "Interceptor failed",
		},
		{
			name:        "PostRequest",
			mode:        providers.InterceptorModePostRequest,
			executionID: "exec-006",
			errCode:     "INT-002",
			errKey:      "error.interceptor.post_request_failed",
			errDefault:  "Post-request interceptor failed",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			t := s.T()
			mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

			mockGraph := coremock.NewGraphInterfaceMock(t)
			mockGraph.On("HasSegments").Return(false)

			mockNode := coremock.NewNodeInterfaceMock(t)
			mockNode.On("GetID").Return("node-1").Maybe()
			mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
			mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
			mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

			fe := &flowEngine{
				interceptorRunner: mockInterceptorSvc,
				logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
			}

			svcErr := &tidcommon.ServiceError{
				Code: tc.errCode,
				Error: tidcommon.I18nMessage{
					Key:          tc.errKey,
					DefaultValue: tc.errDefault,
				},
			}

			mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
				newTestInterceptorUnitMock(
					t,
					"stub",
					providers.InterceptorMode(""),
					providers.InterceptorScope(""),
					nil,
				),
			})

			ctx := &EngineContext{
				ExecutionID: tc.executionID,
				CurrentNode: mockNode,
				Graph:       mockGraph,
			}

			mockInterceptorSvc.On("runInterceptors", tc.mode,
				mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
				Return((*common.InterceptorResponse)(nil), svcErr)

			continueExec, err := fe.runInterceptors(tc.mode, ctx, nil, &FlowStep{})

			s.False(continueExec)
			s.NotNil(err)
			s.Equal(tc.errCode, err.Code)
		})
	}
}

func (s *EngineTestSuite) TestRunInterceptors_PreNode_UsesProvidedNode() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockCurrentNode := coremock.NewNodeInterfaceMock(t)
	mockCurrentNode.On("GetID").Return("node-1").Maybe()
	mockCurrentNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockCurrentNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockCurrentNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()
	mockTargetNode := coremock.NewNodeInterfaceMock(t)
	mockTargetNode.On("GetID").Return("target-node-id")
	mockTargetNode.On("GetType").Return(common.NodeTypeTaskExecution)
	mockTargetNode.On("GetProperties").Return(map[string]interface{}(nil))
	mockTargetNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil))

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-003",
		CurrentNode: mockCurrentNode,
		Graph:       mockGraph,
	}

	// Capture the invocation context to verify the target node's fields are used, not CurrentNode's.
	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePreNode,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Run(func(args mock.Arguments) {
			execCtx := args.Get(1).(*InterceptorRunnerContext)
			s.Equal("target-node-id", execCtx.CurrentNodeID, "Should use the provided node, not CurrentNode")
			s.Equal(common.NodeTypeTaskExecution, execCtx.NodeType)
		}).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	continueExec, err := fe.runInterceptors(providers.InterceptorModePreNode, ctx, mockTargetNode, &FlowStep{})

	s.True(continueExec)
	s.Nil(err)
}

func (s *EngineTestSuite) TestRunInterceptors_PostNode_UsesProvidedNode() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockCurrentNode := coremock.NewNodeInterfaceMock(t)
	mockCurrentNode.On("GetID").Return("node-1").Maybe()
	mockCurrentNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockCurrentNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockCurrentNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()
	mockTargetNode := coremock.NewNodeInterfaceMock(t)
	mockTargetNode.On("GetID").Return("target-node-id")
	mockTargetNode.On("GetType").Return(common.NodeTypeTaskExecution)
	mockTargetNode.On("GetProperties").Return(map[string]interface{}(nil))
	mockTargetNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil))

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-004",
		CurrentNode: mockCurrentNode,
		Graph:       mockGraph,
	}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostNode,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Run(func(args mock.Arguments) {
			execCtx := args.Get(1).(*InterceptorRunnerContext)
			s.Equal("target-node-id", execCtx.CurrentNodeID)
			s.Equal(common.NodeTypeTaskExecution, execCtx.NodeType)
		}).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	continueExec, err := fe.runInterceptors(providers.InterceptorModePostNode, ctx, mockTargetNode, &FlowStep{})

	s.True(continueExec)
	s.Nil(err)
}

func (s *EngineTestSuite) TestRunInterceptors_PostRequest_Success() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID:           "exec-005",
		CurrentNode:           mockNode,
		Graph:                 mockGraph,
		RuntimeData:           map[string]string{},
		InterceptorSharedData: map[string]string{},
	}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return(&common.InterceptorResponse{
			Status:        common.InterceptorStatusComplete,
			EngineOutputs: map[string]string{"challengeToken": "rotated-token"},
		}, nil)

	continueExec, err := fe.runInterceptors(providers.InterceptorModePostRequest, ctx, nil, &FlowStep{})

	s.True(continueExec)
	s.Nil(err)
	s.Equal("rotated-token", ctx.RuntimeData["challengeToken"])
}

func (s *EngineTestSuite) TestRunInterceptors_NoOutputs_RuntimeDataUnchanged() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-007",
		CurrentNode: mockNode,
		Graph:       mockGraph,
		RuntimeData: map[string]string{"existing": "value"},
	}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePreRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	continueExec, err := fe.runInterceptors(providers.InterceptorModePreRequest, ctx, nil, &FlowStep{})

	s.True(continueExec)
	s.Nil(err)
	s.Len(ctx.RuntimeData, 1)
	s.Equal("value", ctx.RuntimeData["existing"])
}

func (s *EngineTestSuite) TestRunInterceptors_NilSharedData_InitializesEmptyMap() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID:           "exec-008",
		CurrentNode:           mockNode,
		Graph:                 mockGraph,
		InterceptorSharedData: nil, // Nil shared data
	}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePreNode,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Run(func(args mock.Arguments) {
			execCtx := args.Get(1).(*InterceptorRunnerContext)
			s.NotNil(execCtx.SharedData, "SharedData should be initialized to empty map when nil")
		}).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	continueExec, err := fe.runInterceptors(providers.InterceptorModePreNode, ctx, mockNode, &FlowStep{})

	s.True(continueExec)
	s.Nil(err)
}

func (s *EngineTestSuite) TestRunInterceptors_ClonesContextFields() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID:           "exec-009",
		AppID:                 "app-009",
		FlowType:              providers.FlowTypeAuthentication,
		CurrentNode:           mockNode,
		Graph:                 mockGraph,
		UserInputs:            map[string]string{"username": "testuser"},
		AdditionalData:        map[string]string{"key": "val"},
		InterceptorSharedData: map[string]string{"shared": "data"},
	}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePreRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Run(func(args mock.Arguments) {
			execCtx := args.Get(1).(*InterceptorRunnerContext)
			s.Equal("exec-009", execCtx.ExecutionID)
			s.Equal("app-009", execCtx.AppID)
			s.Equal(providers.FlowTypeAuthentication, execCtx.FlowType)
			s.Equal("testuser", execCtx.UserInputs["username"])
			s.Equal("val", execCtx.AdditionalData["key"])
			s.Equal("data", execCtx.SharedData["shared"])

			// Verify cloned maps are independent of original.
			execCtx.UserInputs["mutated"] = "yes"
			execCtx.AdditionalData["mutated"] = "yes"
		}).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	continueExec, err := fe.runInterceptors(providers.InterceptorModePreRequest, ctx, nil, &FlowStep{})

	s.True(continueExec)
	s.Nil(err)
	_, exists := ctx.UserInputs["mutated"]
	s.False(exists, "Original UserInputs should not be mutated")
	_, exists = ctx.AdditionalData["mutated"]
	s.False(exists, "Original AdditionalData should not be mutated")
}

func (s *EngineTestSuite) TestRunInterceptors_NodeFailure_ReturnsError() {
	tests := []struct {
		name           string
		mode           providers.InterceptorMode
		errCode        string
		errKey         string
		errDefault     string
		executionID    string
		runtimeDataKey string
		runtimeDataVal string
	}{
		{
			name:           "PreNode",
			mode:           providers.InterceptorModePreNode,
			errCode:        "INT-PRE-NODE",
			errKey:         "error.interceptor.pre_node_failed",
			errDefault:     "Pre-node interceptor blocked execution",
			executionID:    "exec-prenode-fail",
			runtimeDataKey: "before",
			runtimeDataVal: "value",
		},
		{
			name:           "PostNode",
			mode:           providers.InterceptorModePostNode,
			errCode:        "INT-POST-NODE",
			errKey:         "error.interceptor.post_node_failed",
			errDefault:     "Post-node interceptor rejected response",
			executionID:    "exec-postnode-fail",
			runtimeDataKey: "existing",
			runtimeDataVal: "data",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			t := s.T()
			mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

			mockGraph := coremock.NewGraphInterfaceMock(t)
			mockGraph.On("HasSegments").Return(false)

			mockNode := coremock.NewNodeInterfaceMock(t)
			mockNode.On("GetID").Return("node-1").Maybe()
			mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
			mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
			mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

			fe := &flowEngine{
				interceptorRunner: mockInterceptorSvc,
				logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
			}

			svcErr := &tidcommon.ServiceError{
				Code: tc.errCode,
				Error: tidcommon.I18nMessage{
					Key:          tc.errKey,
					DefaultValue: tc.errDefault,
				},
			}

			mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
				newTestInterceptorUnitMock(
					t,
					"stub",
					providers.InterceptorMode(""),
					providers.InterceptorScope(""),
					nil,
				),
			})

			ctx := &EngineContext{
				ExecutionID: tc.executionID,
				CurrentNode: mockNode,
				Graph:       mockGraph,
				RuntimeData: map[string]string{tc.runtimeDataKey: tc.runtimeDataVal},
			}

			mockInterceptorSvc.On("runInterceptors", tc.mode,
				mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
				Return((*common.InterceptorResponse)(nil), svcErr)

			continueExec, err := fe.runInterceptors(tc.mode, ctx, mockNode, &FlowStep{})

			s.False(continueExec)
			s.NotNil(err)
			s.Equal(tc.errCode, err.Code)
			// RuntimeData should not be modified on failure.
			s.Len(ctx.RuntimeData, 1)
			s.Equal(tc.runtimeDataVal, ctx.RuntimeData[tc.runtimeDataKey])
		})
	}
}

func (s *EngineTestSuite) TestRunInterceptors_Failure_NilRuntimeData_NoMergeAttempt() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	svcErr := &tidcommon.ServiceError{
		Code: "INT-NIL-RT",
		Error: tidcommon.I18nMessage{
			Key:          "error.interceptor.nil_runtime",
			DefaultValue: "Failed with nil runtime data",
		},
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-nil-rt",
		CurrentNode: mockNode,
		Graph:       mockGraph,
		RuntimeData: nil, // nil RuntimeData
	}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return((*common.InterceptorResponse)(nil), svcErr)

	continueExec, err := fe.runInterceptors(providers.InterceptorModePostRequest, ctx, nil, &FlowStep{})

	s.False(continueExec)
	s.NotNil(err)
	s.Equal("INT-NIL-RT", err.Code)
	s.Nil(ctx.RuntimeData, "RuntimeData should remain nil on failure")
}

func (s *EngineTestSuite) TestRunInterceptors_Failure_PreservesFullErrorDetails() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	svcErr := &tidcommon.ServiceError{
		Code: "INT-CAPTCHA-001",
		Error: tidcommon.I18nMessage{
			Key:          "error.interceptor.captcha_failed",
			DefaultValue: "CAPTCHA verification failed",
		},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.interceptor.captcha_failed_desc",
			DefaultValue: "The CAPTCHA response was invalid or expired",
		},
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-full-err",
		CurrentNode: mockNode,
		Graph:       mockGraph,
	}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePreNode,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return((*common.InterceptorResponse)(nil), svcErr)

	continueExec, err := fe.runInterceptors(providers.InterceptorModePreNode, ctx, mockNode, &FlowStep{})

	s.False(continueExec)
	s.NotNil(err)
	s.Equal("INT-CAPTCHA-001", err.Code)
	s.Equal("error.interceptor.captcha_failed", err.Error.Key)
	s.Equal("CAPTCHA verification failed", err.Error.DefaultValue)
	s.Equal("error.interceptor.captcha_failed_desc", err.ErrorDescription.Key)
	s.Equal("The CAPTCHA response was invalid or expired", err.ErrorDescription.DefaultValue)
}

func (s *EngineTestSuite) TestRunInterceptors_Failure_AllModes() {
	modes := []providers.InterceptorMode{
		providers.InterceptorModePreRequest,
		providers.InterceptorModePreNode,
		providers.InterceptorModePostNode,
		providers.InterceptorModePostRequest,
	}

	for _, mode := range modes {
		s.Run(string(mode), func() {
			t := s.T()
			mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

			mockGraph := coremock.NewGraphInterfaceMock(t)
			mockGraph.On("HasSegments").Return(false)

			mockNode := coremock.NewNodeInterfaceMock(t)
			mockNode.On("GetID").Return("node-1").Maybe()
			mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
			mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
			mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

			fe := &flowEngine{
				interceptorRunner: mockInterceptorSvc,
				logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
			}

			svcErr := &tidcommon.ServiceError{
				Code: "INT-" + string(mode),
				Error: tidcommon.I18nMessage{
					Key:          "error.interceptor." + string(mode),
					DefaultValue: "Interceptor failed for " + string(mode),
				},
			}

			mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
				newTestInterceptorUnitMock(
					t,
					"stub",
					providers.InterceptorMode(""),
					providers.InterceptorScope(""),
					nil,
				),
			})

			ctx := &EngineContext{
				ExecutionID: "exec-" + string(mode),
				CurrentNode: mockNode,
				Graph:       mockGraph,
			}

			mockInterceptorSvc.On("runInterceptors", mode,
				mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
				Return((*common.InterceptorResponse)(nil), svcErr)

			continueExec, err := fe.runInterceptors(mode, ctx, mockNode, &FlowStep{})

			s.False(continueExec)
			s.NotNil(err)
			s.Equal("INT-"+string(mode), err.Code)
		})
	}
}

// Tests for runPostRequestInterceptorsOnExit

func (s *EngineTestSuite) TestRunPostRequestInterceptorsOnExit_Success_ContinuesExecution() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		observabilitySvc:  mockObservability,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-post-exit-ok",
		CurrentNode: mockNode,
		Graph:       mockGraph,
		RuntimeData: map[string]string{"key": "value"},
	}

	flowStep := &FlowStep{Status: providers.FlowStatusComplete}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return(&common.InterceptorResponse{
			Status:        common.InterceptorStatusComplete,
			EngineOutputs: map[string]string{"token": "rotated"},
		}, nil)

	continueExec, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, 1000)

	s.True(continueExec)
	s.Nil(svcErr)
	s.Equal("rotated", ctx.RuntimeData["token"])
}

func (s *EngineTestSuite) TestRunPostRequestInterceptorsOnExit_InterceptorError_PublishesFailure() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(true)
	mockObservability.On("PublishEvent", mock.Anything, mock.AnythingOfType("*providers.Event")).Return()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		observabilitySvc:  mockObservability,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	svcErr := &tidcommon.ServiceError{
		Code: "INT-POST-ERR",
		Error: tidcommon.I18nMessage{
			Key:          "error.interceptor.post_request",
			DefaultValue: "Post-request interceptor error",
		},
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-post-exit-err",
		CurrentNode: mockNode,
		Graph:       mockGraph,
		FlowType:    providers.FlowTypeAuthentication,
		AppID:       "app-001",
	}

	flowStep := &FlowStep{Status: providers.FlowStatusIncomplete}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return((*common.InterceptorResponse)(nil), svcErr)

	continueExec, err := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, 1000)

	s.False(continueExec)
	s.NotNil(err)
	s.Equal("INT-POST-ERR", err.Code)
	mockObservability.AssertCalled(t, "PublishEvent", mock.Anything, mock.AnythingOfType("*providers.Event"))
}

func (s *EngineTestSuite) TestRunPostRequestInterceptorsOnExit_Incomplete_StopsExecution() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		observabilitySvc:  mockObservability,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-post-exit-incomplete",
		CurrentNode: mockNode,
		Graph:       mockGraph,
	}

	flowStep := &FlowStep{Status: providers.FlowStatusComplete}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return(&common.InterceptorResponse{
			Status: common.InterceptorStatusIncomplete,
		}, nil)

	continueExec, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, 1000)

	s.False(continueExec)
	s.Nil(svcErr)
	s.Equal(providers.FlowStatusIncomplete, flowStep.Status)
}

func (s *EngineTestSuite) TestRunPostRequestInterceptorsOnExit_Fail_PublishesFlowFailure() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	var capturedEvent *providers.Event
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(true)
	mockObservability.On("PublishEvent", mock.Anything, mock.AnythingOfType("*providers.Event")).
		Run(func(args mock.Arguments) {
			capturedEvent = args.Get(1).(*providers.Event)
		}).Return()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		observabilitySvc:  mockObservability,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	interceptorErr := &tidcommon.ServiceError{
		Code: "INT-FAIL-001",
		Error: tidcommon.I18nMessage{
			Key:          "error.interceptor.blocked",
			DefaultValue: "Interceptor blocked the flow",
		},
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-post-exit-fail",
		CurrentNode: mockNode,
		Graph:       mockGraph,
		FlowType:    providers.FlowTypeAuthentication,
		AppID:       "app-002",
	}

	flowStep := &FlowStep{}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return(&common.InterceptorResponse{
			Status: common.InterceptorStatusFailure,
			Error:  interceptorErr,
		}, nil)

	continueExec, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, 1000)

	s.False(continueExec)
	s.Nil(svcErr)
	s.Equal(providers.FlowStatusError, flowStep.Status)
	s.NotNil(flowStep.Error)
	s.Equal("INT-FAIL-001", flowStep.Error.Code)
	// Flow error status should trigger event publication
	s.NotNil(capturedEvent)
	s.Equal(string(event.EventTypeFlowFailed), capturedEvent.Type)
}

func (s *EngineTestSuite) TestRunPostRequestInterceptorsOnExit_Incomplete_NoEventWhenNotError() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		observabilitySvc:  mockObservability,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-post-exit-no-event",
		CurrentNode: mockNode,
		Graph:       mockGraph,
	}

	flowStep := &FlowStep{Status: providers.FlowStatusIncomplete}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return(&common.InterceptorResponse{
			Status: common.InterceptorStatusIncomplete,
		}, nil)

	continueExec, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, 1000)

	s.False(continueExec)
	s.Nil(svcErr)
	// No PublishEvent call should be made since status is INCOMPLETE, not ERROR
	mockObservability.AssertNotCalled(t, "PublishEvent", mock.Anything, mock.Anything)
}

func (s *EngineTestSuite) TestRunPostRequestInterceptorsOnExit_UpdatesFlowStepFields() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		observabilitySvc:  mockObservability,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-post-exit-fields",
		CurrentNode: mockNode,
		Graph:       mockGraph,
	}

	flowStep := &FlowStep{Status: providers.FlowStatusComplete}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return(&common.InterceptorResponse{
			Status:         common.InterceptorStatusComplete,
			ChallengeToken: "new-challenge-token",
			FieldErrors: []common.FieldError{
				{Identifier: "email", Message: "invalid format"},
			},
		}, nil)

	continueExec, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, 1000)

	s.True(continueExec)
	s.Nil(svcErr)
	s.Equal("new-challenge-token", flowStep.ChallengeToken)
	s.Len(flowStep.Data.FieldErrors, 1)
	s.Equal("email", flowStep.Data.FieldErrors[0].Identifier)
}

func (s *EngineTestSuite) TestRunPostRequestInterceptorsOnExit_NilGraph_SkipsInterceptors() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		observabilitySvc: mockObservability,
		logger:           log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	ctx := &EngineContext{
		ExecutionID: "exec-post-exit-nil-graph",
		Graph:       nil,
	}

	flowStep := &FlowStep{Status: providers.FlowStatusComplete}

	continueExec, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, 1000)

	s.True(continueExec)
	s.Nil(svcErr)
}

func (s *EngineTestSuite) TestRunPostRequestInterceptorsOnExit_MergesEngineOutputsToRuntimeData() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		observabilitySvc:  mockObservability,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-post-exit-merge",
		CurrentNode: mockNode,
		Graph:       mockGraph,
		RuntimeData: map[string]string{"existing": "data"},
	}

	flowStep := &FlowStep{Status: providers.FlowStatusComplete}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Return(&common.InterceptorResponse{
			Status:        common.InterceptorStatusComplete,
			EngineOutputs: map[string]string{"newKey": "newValue", "anotherKey": "anotherValue"},
		}, nil)

	continueExec, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, 1000)

	s.True(continueExec)
	s.Nil(svcErr)
	s.Equal("data", ctx.RuntimeData["existing"])
	s.Equal("newValue", ctx.RuntimeData["newKey"])
	s.Equal("anotherValue", ctx.RuntimeData["anotherKey"])
}

func (s *EngineTestSuite) TestRunPostRequestInterceptorsOnExit_PassesFlowStatusToInterceptorContext() {
	t := s.T()
	mockInterceptorSvc := NewInterceptorRunnerInterfaceMock(t)

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("node-1").Maybe()
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution).Maybe()
	mockNode.On("GetProperties").Return(map[string]interface{}(nil)).Maybe()
	mockNode.On("GetExecutionPolicy").Return((*providers.ExecutionPolicy)(nil)).Maybe()

	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(false).Maybe()

	fe := &flowEngine{
		interceptorRunner: mockInterceptorSvc,
		observabilitySvc:  mockObservability,
		logger:            log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	mockGraph.On("GetInterceptors", mock.Anything).Return([]core.InterceptorUnitInterface{
		newTestInterceptorUnitMock(t, "stub", providers.InterceptorMode(""), providers.InterceptorScope(""), nil),
	})

	ctx := &EngineContext{
		ExecutionID: "exec-post-exit-status",
		CurrentNode: mockNode,
		Graph:       mockGraph,
	}

	flowStep := &FlowStep{Status: providers.FlowStatusComplete}

	mockInterceptorSvc.On("runInterceptors", providers.InterceptorModePostRequest,
		mock.AnythingOfType("*flowexec.InterceptorRunnerContext")).
		Run(func(args mock.Arguments) {
			execCtx := args.Get(1).(*InterceptorRunnerContext)
			s.Equal(providers.FlowStatusComplete, execCtx.FlowStatus,
				"Interceptor context should receive current flow status")
		}).
		Return(&common.InterceptorResponse{Status: common.InterceptorStatusComplete}, nil)

	continueExec, svcErr := fe.runPostRequestInterceptorsOnExit(ctx, flowStep, 1000)

	s.True(continueExec)
	s.Nil(svcErr)
}

// --- handleCallResponse ---

func (s *EngineTestSuite) TestHandleCallResponse_DepthExceeded() {
	t := s.T()
	mockCurrentNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), CurrentNode: mockCurrentNode}

	// Fill the stack to maxCallDepth.
	for i := 0; i < maxCallDepth; i++ {
		ctx.pushFrame("call-node-1")
	}

	nodeResp := &common.NodeResponse{
		Status:           common.NodeStatusCall,
		CallTargetFlowID: "target-flow",
	}
	next, svcErr := fe.handleCallResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(next)
	s.NotNil(svcErr)
	s.Equal(ErrorMaxCallDepthExceeded.Code, svcErr.Code)
}

func (s *EngineTestSuite) TestHandleCallResponse_FlowProviderError() {
	t := s.T()
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)

	svcErr := &tidcommon.ServiceError{Code: "not-found"}
	mockFlowProvider.On("GetFlow", mock.Anything, "target-flow").Return(nil, svcErr)

	fe := &flowEngine{
		logger:       log.GetLogger(),
		flowProvider: mockFlowProvider,
		graphBuilder: mockGraphBuilder,
	}
	ctx := &EngineContext{Context: context.Background()}

	nodeResp := &common.NodeResponse{
		Status:           common.NodeStatusCall,
		CallTargetFlowID: "target-flow",
	}
	next, err := fe.handleCallResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(next)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *EngineTestSuite) TestHandleCallResponse_GraphBuilderError() {
	t := s.T()
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)

	flow := &providers.CompleteFlowDefinition{ID: "target-flow"}
	mockFlowProvider.On("GetFlow", mock.Anything, "target-flow").Return(flow, nil)
	graphErr := &tidcommon.ServiceError{Code: "graph-err"}
	mockGraphBuilder.On("GetGraph", mock.Anything, flow).Return(nil, graphErr)

	fe := &flowEngine{
		logger:       log.GetLogger(),
		flowProvider: mockFlowProvider,
		graphBuilder: mockGraphBuilder,
	}
	ctx := &EngineContext{Context: context.Background()}

	nodeResp := &common.NodeResponse{
		Status:           common.NodeStatusCall,
		CallTargetFlowID: "target-flow",
	}
	next, err := fe.handleCallResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(next)
	s.NotNil(err)
	s.Equal(tidcommon.InternalServerError.Code, err.Code)
}

func (s *EngineTestSuite) TestHandleCallResponse_NoStartNode() {
	t := s.T()
	mockCurrentNode := coremock.NewNodeInterfaceMock(t)
	mockCurrentNode.On("GetID").Return("call-node-1")
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCalleeGraph := coremock.NewGraphInterfaceMock(t)

	flow := &providers.CompleteFlowDefinition{ID: "target-flow", FlowType: providers.FlowTypeRegistration}
	mockFlowProvider.On("GetFlow", mock.Anything, "target-flow").Return(flow, nil)
	mockGraphBuilder.On("GetGraph", mock.Anything, flow).Return(mockCalleeGraph, nil)
	mockCalleeGraph.On("GetType").Return(providers.FlowTypeRegistration)
	mockCalleeGraph.On("GetStartNode").Return(nil, errors.New("no start node"))

	fe := &flowEngine{
		logger:       log.GetLogger(),
		flowProvider: mockFlowProvider,
		graphBuilder: mockGraphBuilder,
	}
	ctx := &EngineContext{Context: context.Background(), CurrentNode: mockCurrentNode}

	nodeResp := &common.NodeResponse{
		Status:           common.NodeStatusCall,
		CallTargetFlowID: "target-flow",
	}
	next, err := fe.handleCallResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(next)
	s.NotNil(err)
}

func (s *EngineTestSuite) TestHandleCallResponse_Success() {
	t := s.T()
	mockCurrentNode := coremock.NewNodeInterfaceMock(t)
	mockCurrentNode.On("GetID").Return("call-node-1")
	mockFlowProvider := NewFlowProviderMock(t)
	mockGraphBuilder := NewGraphBuilderInterfaceMock(t)
	mockCalleeGraph := coremock.NewGraphInterfaceMock(t)
	mockStartNode := coremock.NewNodeInterfaceMock(t)

	flow := &providers.CompleteFlowDefinition{ID: "target-flow", FlowType: providers.FlowTypeRegistration}
	mockFlowProvider.On("GetFlow", mock.Anything, "target-flow").Return(flow, nil)
	mockGraphBuilder.On("GetGraph", mock.Anything, flow).Return(mockCalleeGraph, nil)
	mockCalleeGraph.On("GetType").Return(providers.FlowTypeRegistration)
	mockCalleeGraph.On("GetStartNode").Return(mockStartNode, nil)

	fe := &flowEngine{
		logger:       log.GetLogger(),
		flowProvider: mockFlowProvider,
		graphBuilder: mockGraphBuilder,
	}
	ctx := &EngineContext{
		Context:     context.Background(),
		CurrentNode: mockCurrentNode,
		FlowType:    providers.FlowTypeAuthentication,
	}

	nodeResp := &common.NodeResponse{
		Status:           common.NodeStatusCall,
		CallTargetFlowID: "target-flow",
	}
	next, err := fe.handleCallResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(err)
	s.Equal(mockStartNode, next)
	s.Equal(1, ctx.frameDepth())
	s.Equal(mockCalleeGraph, ctx.Graph)
	s.Equal(providers.FlowTypeRegistration, ctx.FlowType)
	s.Equal(mockStartNode, ctx.CurrentNode)
}

// --- handleCalleeReturn ---

func (s *EngineTestSuite) TestHandleCalleeReturn_Success() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallNode := coremock.NewCallNodeInterfaceMock(t)
	mockNextNode := coremock.NewNodeInterfaceMock(t)

	mockCallerGraph.On("GetNode", "call-node-1").Return(mockCallNode, true)
	mockCallNode.On("GetOnSuccess").Return("next-node")
	mockCallerGraph.On("GetNode", "next-node").Return(mockNextNode, true)

	fe := &flowEngine{logger: log.GetLogger()}

	// Simulate caller context.
	ctx := &EngineContext{
		Context:  context.Background(),
		Graph:    mockCallerGraph,
		FlowType: providers.FlowTypeAuthentication,
	}
	ctx.pushFrame("call-node-1")

	// Simulate being in callee now.
	ctx.Graph = coremock.NewGraphInterfaceMock(t)
	ctx.AdditionalData = map[string]string{}

	next, svcErr := fe.handleCalleeReturn(ctx, log.GetLogger())
	s.Nil(svcErr)
	s.Equal(mockNextNode, next)
	s.Equal(0, ctx.frameDepth())
	s.Equal(mockCallerGraph, ctx.Graph)
}

func (s *EngineTestSuite) TestHandleCalleeReturn_CallerCallNodeNotFound() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallerGraph.On("GetNode", "call-node-1").Return(nil, false)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), Graph: mockCallerGraph}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)

	next, svcErr := fe.handleCalleeReturn(ctx, log.GetLogger())
	s.Nil(next)
	s.NotNil(svcErr)
}

func (s *EngineTestSuite) TestHandleCalleeReturn_CallerResumeNodeNotCallNode() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockNonCallNode := coremock.NewNodeInterfaceMock(t)
	mockCallerGraph.On("GetNode", "call-node-1").Return(mockNonCallNode, true)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), Graph: mockCallerGraph}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)

	next, svcErr := fe.handleCalleeReturn(ctx, log.GetLogger())
	s.Nil(next)
	s.NotNil(svcErr)
}

func (s *EngineTestSuite) TestHandleCalleeReturn_OnSuccessNodeNotFound() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallNode := coremock.NewCallNodeInterfaceMock(t)

	mockCallerGraph.On("GetNode", "call-node-1").Return(mockCallNode, true)
	mockCallNode.On("GetOnSuccess").Return("next-node")
	mockCallerGraph.On("GetNode", "next-node").Return(nil, false)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), Graph: mockCallerGraph}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)

	next, svcErr := fe.handleCalleeReturn(ctx, log.GetLogger())
	s.Nil(next)
	s.NotNil(svcErr)
}

func (s *EngineTestSuite) TestHandleCalleeReturn_AdditionalDataMerge() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallNode := coremock.NewCallNodeInterfaceMock(t)
	mockNextNode := coremock.NewNodeInterfaceMock(t)

	mockCallerGraph.On("GetNode", "call-node-1").Return(mockCallNode, true)
	mockCallNode.On("GetOnSuccess").Return("next-node")
	mockCallerGraph.On("GetNode", "next-node").Return(mockNextNode, true)

	fe := &flowEngine{logger: log.GetLogger()}

	// Caller had additionalData = {"caller-key": "caller-val", "shared": "caller"}.
	ctx := &EngineContext{
		Context:        context.Background(),
		Graph:          mockCallerGraph,
		AdditionalData: map[string]string{"caller-key": "caller-val", "shared": "caller"},
	}
	ctx.pushFrame("call-node-1")

	// Callee mutates the shared AdditionalData map in-place (adds a key, overrides "shared").
	ctx.Graph = coremock.NewGraphInterfaceMock(t)
	ctx.AdditionalData["callee-key"] = "callee-val"
	ctx.AdditionalData["shared"] = "callee"

	_, svcErr := fe.handleCalleeReturn(ctx, log.GetLogger())
	s.Nil(svcErr)

	// Callee overrides "shared"; caller-key survives; callee-key is added.
	s.Equal("caller-val", ctx.AdditionalData["caller-key"])
	s.Equal("callee-val", ctx.AdditionalData["callee-key"])
	s.Equal("callee", ctx.AdditionalData["shared"])
}

// --- handleCalleeFailure ---

func (s *EngineTestSuite) TestHandleCalleeFailure_NoOnFailure() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallNode := coremock.NewCallNodeInterfaceMock(t)

	mockCallerGraph.On("GetNode", "call-node-1").Return(mockCallNode, true)
	mockCallNode.On("GetOnFailure").Return("")

	fe := &flowEngine{logger: log.GetLogger()}
	flowStep := &FlowStep{}

	callerErr := &tidcommon.ServiceError{Code: "callee-failed"}
	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusFailure,
		Error:  callerErr,
	}

	ctx := &EngineContext{Context: context.Background(), Graph: mockCallerGraph}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)

	next, continueExec, svcErr := fe.handleCalleeFailure(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(next)
	s.False(continueExec)
	s.Nil(svcErr)
	s.Equal(providers.FlowStatusError, flowStep.Status)
	s.Equal(callerErr, flowStep.Error)
}

func (s *EngineTestSuite) TestHandleCalleeFailure_WithOnFailure() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallNode := coremock.NewCallNodeInterfaceMock(t)
	mockFailureNode := coremock.NewNodeInterfaceMock(t)

	mockCallerGraph.On("GetNode", "call-node-1").Return(mockCallNode, true)
	mockCallNode.On("GetOnFailure").Return("failure-node")
	mockCallerGraph.On("GetNode", "failure-node").Return(mockFailureNode, true)

	fe := &flowEngine{logger: log.GetLogger()}
	flowStep := &FlowStep{}

	callerErr := &tidcommon.ServiceError{Code: "callee-failed"}
	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusFailure,
		Error:  callerErr,
	}

	ctx := &EngineContext{Context: context.Background(), Graph: mockCallerGraph}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)

	next, continueExec, svcErr := fe.handleCalleeFailure(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(svcErr)
	s.True(continueExec)
	s.Equal(mockFailureNode, next)
	s.NotNil(ctx.CurrentNodeResponse)
	s.Equal(common.NodeStatusForward, ctx.CurrentNodeResponse.Status)
	s.Equal(callerErr, ctx.CurrentNodeResponse.Error)
}

func (s *EngineTestSuite) TestHandleCalleeFailure_CallNodeNotFound() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallerGraph.On("GetNode", "call-node-1").Return(nil, false)

	fe := &flowEngine{logger: log.GetLogger()}
	flowStep := &FlowStep{}
	nodeResp := &common.NodeResponse{Status: common.NodeStatusFailure}

	ctx := &EngineContext{Context: context.Background(), Graph: mockCallerGraph}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)

	next, continueExec, svcErr := fe.handleCalleeFailure(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(next)
	s.False(continueExec)
	s.NotNil(svcErr)
}

func (s *EngineTestSuite) TestHandleCalleeReturn_FrameUnderflow() {
	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background()}

	next, svcErr := fe.handleCalleeReturn(ctx, log.GetLogger())
	s.Nil(next)
	s.NotNil(svcErr)
}

func (s *EngineTestSuite) TestHandleCalleeReturn_EmptyOnSuccess() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallNode := coremock.NewCallNodeInterfaceMock(t)

	mockCallerGraph.On("GetNode", "call-node-1").Return(mockCallNode, true)
	mockCallNode.On("GetOnSuccess").Return("")

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), Graph: mockCallerGraph}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)
	ctx.AdditionalData = map[string]string{}

	next, svcErr := fe.handleCalleeReturn(ctx, log.GetLogger())
	s.Nil(next)
	s.NotNil(svcErr)
}

func (s *EngineTestSuite) TestHandleCalleeFailure_FrameUnderflow() {
	fe := &flowEngine{logger: log.GetLogger()}
	flowStep := &FlowStep{}
	nodeResp := &common.NodeResponse{Status: common.NodeStatusFailure}
	ctx := &EngineContext{Context: context.Background()}

	next, continueExec, svcErr := fe.handleCalleeFailure(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(next)
	s.False(continueExec)
	s.NotNil(svcErr)
}

func (s *EngineTestSuite) TestHandleCalleeFailure_CallerResumeNodeNotCallNode() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockNonCallNode := coremock.NewNodeInterfaceMock(t)
	mockCallerGraph.On("GetNode", "call-node-1").Return(mockNonCallNode, true)

	fe := &flowEngine{logger: log.GetLogger()}
	flowStep := &FlowStep{}
	nodeResp := &common.NodeResponse{Status: common.NodeStatusFailure}

	ctx := &EngineContext{Context: context.Background(), Graph: mockCallerGraph}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)

	next, continueExec, svcErr := fe.handleCalleeFailure(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(next)
	s.False(continueExec)
	s.NotNil(svcErr)
}

func (s *EngineTestSuite) TestHandleCalleeFailure_OnFailureNodeNotFound() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallNode := coremock.NewCallNodeInterfaceMock(t)

	mockCallerGraph.On("GetNode", "call-node-1").Return(mockCallNode, true)
	mockCallNode.On("GetOnFailure").Return("missing-node")
	mockCallerGraph.On("GetNode", "missing-node").Return(nil, false)

	fe := &flowEngine{logger: log.GetLogger()}
	flowStep := &FlowStep{}
	nodeResp := &common.NodeResponse{Status: common.NodeStatusFailure}

	ctx := &EngineContext{Context: context.Background(), Graph: mockCallerGraph}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)

	next, continueExec, svcErr := fe.handleCalleeFailure(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(next)
	s.False(continueExec)
	s.NotNil(svcErr)
}

// --- processNodeResponse with frame stack ---

func (s *EngineTestSuite) TestProcessNodeResponse_CallStatus_DepthExceeded() {
	t := s.T()
	mockCurrentNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), CurrentNode: mockCurrentNode}

	for i := 0; i < maxCallDepth; i++ {
		ctx.pushFrame("call-node-1")
	}

	nodeResp := &common.NodeResponse{
		Status:           common.NodeStatusCall,
		CallTargetFlowID: "target-flow",
	}
	flowStep := &FlowStep{}
	next, continueExec, svcErr := fe.processNodeResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(next)
	s.False(continueExec)
	s.NotNil(svcErr)
}

func (s *EngineTestSuite) TestProcessNodeResponse_FailureStatus_WithFrameStack() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallNode := coremock.NewCallNodeInterfaceMock(t)
	mockFailureNode := coremock.NewNodeInterfaceMock(t)

	mockCallerGraph.On("GetNode", "call-node-1").Return(mockCallNode, true)
	mockCallNode.On("GetOnFailure").Return("failure-node")
	mockCallerGraph.On("GetNode", "failure-node").Return(mockFailureNode, true)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{Context: context.Background(), Graph: mockCallerGraph}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)

	callerErr := &tidcommon.ServiceError{Code: "callee-failed"}
	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusFailure,
		Error:  callerErr,
	}
	flowStep := &FlowStep{}
	next, continueExec, svcErr := fe.processNodeResponse(ctx, nodeResp, flowStep, log.GetLogger())
	s.Nil(svcErr)
	s.True(continueExec)
	s.Equal(mockFailureNode, next)
}

func (s *EngineTestSuite) TestHandleCompletedResponse_EndNodeWithFrameStack() {
	t := s.T()
	mockCallerGraph := coremock.NewGraphInterfaceMock(t)
	mockCallNode := coremock.NewCallNodeInterfaceMock(t)
	mockNextNode := coremock.NewNodeInterfaceMock(t)
	mockEndNode := coremock.NewNodeInterfaceMock(t)
	mockEndNode.On("GetType").Return(common.NodeTypeEnd)

	mockCallerGraph.On("GetNode", "call-node-1").Return(mockCallNode, true)
	mockCallNode.On("GetOnSuccess").Return("next-node")
	mockCallerGraph.On("GetNode", "next-node").Return(mockNextNode, true)

	fe := &flowEngine{logger: log.GetLogger()}
	ctx := &EngineContext{
		Context:     context.Background(),
		Graph:       mockCallerGraph,
		CurrentNode: mockEndNode,
	}
	ctx.pushFrame("call-node-1")
	ctx.Graph = coremock.NewGraphInterfaceMock(t)
	ctx.CurrentNode = mockEndNode
	ctx.AdditionalData = map[string]string{}

	nodeResp := &common.NodeResponse{Status: common.NodeStatusComplete}
	next, svcErr := fe.handleCompletedResponse(ctx, nodeResp, log.GetLogger())
	s.Nil(svcErr)
	s.Equal(mockNextNode, next)
	s.Equal(0, ctx.frameDepth())
}

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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	managerpkg "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/observability/event"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/observability/observabilitymock"
)

type EngineTestSuite struct {
	suite.Suite
}

func TestEngineTestSuite(t *testing.T) {
	suite.Run(t, new(EngineTestSuite))
}

func newAuthenticatedAuthUser() managerpkg.AuthUser {
	var authUser managerpkg.AuthUser
	_ = authUser.UnmarshalJSON([]byte(`{"entityReferenceToken":"tok","attributeToken":"tok"}`))
	return authUser
}

func (s *EngineTestSuite) TestGetNodeInputs_ExecutorBackedNode() {
	t := s.T()
	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(t)
	expectedInputs := []common.Input{
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
			Inputs: []common.Input{
				{Identifier: "email", Type: "string", Required: true},
			},
		},
		{
			Inputs: []common.Input{
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
		Inputs: []common.Input{
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
		Inputs: []common.Input{
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
		Inputs: []common.Input{
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

	var newAuthUser managerpkg.AuthUser
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
		Inputs: []common.Input{
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
	s.Equal(common.FlowStatusIncomplete, flowStep.Status)
	s.Equal(common.StepTypeRedirection, flowStep.Type)
}

func (s *EngineTestSuite) TestResolveStepForRedirection_AppendsInputs() {
	fe := &flowEngine{}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		RedirectURL: "https://example.com/auth",
		Inputs: []common.Input{
			{Identifier: "code", Type: "string", Required: true},
		},
	}

	flowStep := &FlowStep{
		Data: FlowData{
			Inputs: []common.Input{
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
		Inputs: []common.Input{
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
		Inputs: []common.Input{
			{Identifier: "otp", Type: "string", Required: true},
		},
		Error: &serviceerror.ServiceError{
			Code: "FET-1008",
			Error: i18ncore.I18nMessage{
				Key:          "flows.executor.errors.invalid_otp",
				DefaultValue: "Invalid OTP provided",
			},
			ErrorDescription: i18ncore.I18nMessage{
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
	s.Equal(common.FlowStatusIncomplete, flowStep.Status)
	s.Equal(common.StepTypeView, flowStep.Type)
}

func (s *EngineTestSuite) TestResolveStepDetailsForPrompt_AppendsInputs() {
	fe := &flowEngine{}

	ctx := &EngineContext{}

	nodeResp := &common.NodeResponse{
		Inputs: []common.Input{
			{Identifier: "password", Type: "string", Required: true},
		},
	}

	flowStep := &FlowStep{
		Data: FlowData{
			Inputs: []common.Input{
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
			Inputs: []common.Input{},
		},
	}
	mockNode.On("GetPrompts").Return(prompts)

	inputs := getNodeInputs(mockNode)

	s.Nil(inputs)
}

func (s *EngineTestSuite) TestClearSensitiveInputs_AuthFlowRemovesPassword() {
	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(s.T())
	mockNode.On("GetInputs").Return([]common.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
		{Identifier: "password", Type: common.InputTypePassword, Required: true},
	})
	mockNode.On("GetExecutor").Return(nil).Maybe()

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: common.FlowTypeAuthentication,
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
	mockNode.On("GetInputs").Return([]common.Input{
		{Identifier: "otp", Type: common.InputTypeOTP, Required: true},
	})
	mockNode.On("GetExecutor").Return(nil).Maybe()

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: common.FlowTypeAuthentication,
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
	mockNode.On("GetInputs").Return([]common.Input{
		{Identifier: "password", Type: common.InputTypePassword, Required: true},
	}).Maybe()

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: common.FlowTypeRegistration,
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
		FlowType: common.FlowTypeAuthentication,
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
	mockNode.On("GetInputs").Return([]common.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
	})
	mockNode.On("GetExecutor").Return(nil).Maybe()

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: common.FlowTypeAuthentication,
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
	mockExecutor.On("GetDefaultInputs").Return([]common.Input{
		{Identifier: "username", Type: "string", Required: true},
		{Identifier: "password", Type: common.InputTypePassword, Required: true},
	})

	mockNode := coremock.NewExecutorBackedNodeInterfaceMock(s.T())
	mockNode.On("GetInputs").Return([]common.Input{})
	mockNode.On("GetExecutor").Return(mockExecutor)

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: common.FlowTypeAuthentication,
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
	mockNode.On("GetInputs").Return([]common.Input{
		{Identifier: "password", Type: common.InputTypePassword, Required: true},
	}).Maybe()

	fe := &flowEngine{}
	ctx := &EngineContext{
		FlowType: common.FlowTypeUserOnboarding,
		UserInputs: map[string]string{
			"password": "secret123",
		},
	}

	fe.clearSensitiveInputs(ctx, mockNode)

	s.Equal("secret123", ctx.UserInputs["password"])
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
	s.Equal(common.FlowStatusIncomplete, flowStep.Status)
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
	s.Equal(common.FlowStatusComplete, flowStep.Status)
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
	s.Equal(common.FlowStatusComplete, flowStep.Status)
}

func (s *EngineTestSuite) TestValidateChallengeToken_EmptyTokenHashSkipsValidation() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{
		ExecutionID:        "test-exec-id",
		ChallengeTokenIn:   "some-token",
		ChallengeTokenHash: "",
	}

	svcErr := fe.validateChallengeToken(ctx, mockNode)
	s.Nil(svcErr)
}

func (s *EngineTestSuite) TestValidateChallengeToken_SkipValidationWhenPolicyAllows() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetExecutionPolicy").Return(&core.ExecutionPolicy{
		SkipChallengeValidation: true,
	})

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	// Generate a token and hash it
	tokenStr, err := cryptolib.GenerateSecureToken()
	s.NoError(err)
	tokenHash := cryptolib.HashToken(tokenStr)

	ctx := &EngineContext{
		ExecutionID:        "test-exec-id",
		ChallengeTokenIn:   "wrong-token",
		ChallengeTokenHash: tokenHash,
	}

	svcErr := fe.validateChallengeToken(ctx, mockNode)
	s.Nil(svcErr)
}

func (s *EngineTestSuite) TestValidateChallengeToken_ReturnsErrorWhenTokenEmpty() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetExecutionPolicy").Return(nil)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	// Generate a token and hash it
	tokenStr, err := cryptolib.GenerateSecureToken()
	s.NoError(err)
	tokenHash := cryptolib.HashToken(tokenStr)

	ctx := &EngineContext{
		ExecutionID:        "test-exec-id",
		ChallengeTokenIn:   "", // Empty token
		ChallengeTokenHash: tokenHash,
	}

	svcErr := fe.validateChallengeToken(ctx, mockNode)
	s.NotNil(svcErr)
	s.Equal("FES-1009", svcErr.Code)
}

func (s *EngineTestSuite) TestValidateChallengeToken_ReturnsErrorWhenTokenInvalid() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetExecutionPolicy").Return(nil)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	// Generate a token and hash it
	tokenStr, err := cryptolib.GenerateSecureToken()
	s.NoError(err)
	tokenHash := cryptolib.HashToken(tokenStr)

	ctx := &EngineContext{
		ExecutionID:        "test-exec-id",
		ChallengeTokenIn:   "invalid-token",
		ChallengeTokenHash: tokenHash,
	}

	svcErr := fe.validateChallengeToken(ctx, mockNode)
	s.NotNil(svcErr)
	s.Equal("FES-1009", svcErr.Code)
}

func (s *EngineTestSuite) TestValidateChallengeToken_SucceedsWhenTokenValid() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetExecutionPolicy").Return(nil)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	// Generate a token and hash it
	tokenStr, err := cryptolib.GenerateSecureToken()
	s.NoError(err)
	tokenHash := cryptolib.HashToken(tokenStr)

	ctx := &EngineContext{
		ExecutionID:        "test-exec-id",
		ChallengeTokenIn:   tokenStr,
		ChallengeTokenHash: tokenHash,
	}

	svcErr := fe.validateChallengeToken(ctx, mockNode)
	s.Nil(svcErr)
}

func (s *EngineTestSuite) TestValidateChallengeToken_SkipValidationWhenNodeNil() {
	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	// Generate a token and hash it
	tokenStr, err := cryptolib.GenerateSecureToken()
	s.NoError(err)
	tokenHash := cryptolib.HashToken(tokenStr)

	ctx := &EngineContext{
		ExecutionID:        "test-exec-id",
		ChallengeTokenIn:   "wrong-token",
		ChallengeTokenHash: tokenHash,
	}

	svcErr := fe.validateChallengeToken(ctx, nil)
	s.NotNil(svcErr)
	s.Equal("FES-1009", svcErr.Code)
}

func (s *EngineTestSuite) TestValidateChallengeToken_SkipValidationWhenPolicyNil() {
	t := s.T()
	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetExecutionPolicy").Return(nil)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}

	// Generate a token and hash it
	tokenStr, err := cryptolib.GenerateSecureToken()
	s.NoError(err)
	tokenHash := cryptolib.HashToken(tokenStr)

	ctx := &EngineContext{
		ExecutionID:        "test-exec-id",
		ChallengeTokenIn:   tokenStr,
		ChallengeTokenHash: tokenHash,
	}

	svcErr := fe.validateChallengeToken(ctx, mockNode)
	s.Nil(svcErr)
}

func (s *EngineTestSuite) TestValidateSegmentResumePolicy_NoSegments() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(false)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: "seg-1"}

	s.False(fe.validateSegmentResumePolicy(ctx, fe.logger))
}

func (s *EngineTestSuite) TestValidateSegmentResumePolicy_EmptySegmentID() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(true)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: ""}

	s.False(fe.validateSegmentResumePolicy(ctx, fe.logger))
}

func (s *EngineTestSuite) TestValidateSegmentResumePolicy_SegmentNotFound() {
	t := s.T()
	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(true)
	mockGraph.On("GetSegmentByID", "seg-1").Return((*core.Segment)(nil))

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: "seg-1"}

	s.False(fe.validateSegmentResumePolicy(ctx, fe.logger))
}

func (s *EngineTestSuite) TestValidateSegmentResumePolicy_StartNodeNotFound() {
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

	s.False(fe.validateSegmentResumePolicy(ctx, fe.logger))
}

func (s *EngineTestSuite) TestValidateSegmentResumePolicy_SetNodeExecutorFails() {
	// Node reports TaskExecution type but NodeInterfaceMock doesn't implement
	// ExecutorBackedNodeInterface, so the type assertion in setNodeExecutor fails.
	t := s.T()
	seg := &core.Segment{ID: "seg-1", StartNodeID: "task-node"}

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)
	mockNode.On("GetID").Return("task-node")

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(true)
	mockGraph.On("GetSegmentByID", "seg-1").Return(seg)
	mockGraph.On("GetNode", "task-node").Return(mockNode, true)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: "seg-1"}

	s.False(fe.validateSegmentResumePolicy(ctx, fe.logger))
}

func (s *EngineTestSuite) TestValidateSegmentResumePolicy_NilPolicy() {
	t := s.T()
	seg := &core.Segment{ID: "seg-1", StartNodeID: "prompt-node"}

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetType").Return(common.NodeTypePrompt)
	mockNode.On("GetExecutionPolicy").Return((*core.ExecutionPolicy)(nil))

	mockGraph := coremock.NewGraphInterfaceMock(t)
	mockGraph.On("HasSegments").Return(true)
	mockGraph.On("GetSegmentByID", "seg-1").Return(seg)
	mockGraph.On("GetNode", "prompt-node").Return(mockNode, true)

	fe := &flowEngine{
		logger: log.GetLogger().With(log.String(log.LoggerKeyComponentName, "FlowEngine")),
	}
	ctx := &EngineContext{Graph: mockGraph, CurrentSegmentID: "seg-1"}

	s.False(fe.validateSegmentResumePolicy(ctx, fe.logger))
}

func (s *EngineTestSuite) TestValidateSegmentResumePolicy_PolicyAllowsRestartFlag() {
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
			policy := &core.ExecutionPolicy{SkipChallengeValidation: true, AllowSegmentRestart: tt.allowRestart}

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

			s.Equal(tt.expectAllowed, fe.validateSegmentResumePolicy(ctx, fe.logger))
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

	var capturedEvent *event.Event
	mockObservability.On("PublishEvent", mock.Anything, mock.AnythingOfType("*event.Event")).
		Run(func(args mock.Arguments) {
			capturedEvent = args.Get(1).(*event.Event)
		}).Return()

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("test-node")
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)

	ctx := &EngineContext{
		ExecutionID: "exec-123",
		FlowType:    common.FlowTypeAuthentication,
		AppID:       "app-456",
		ExecutionHistory: map[string]*common.NodeExecutionRecord{
			"test-node": {
				Step: 1,
				Executions: []common.ExecutionAttempt{
					{Attempt: 1},
				},
			},
		},
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusIncomplete,
		Error: &serviceerror.ServiceError{
			Code: "FET-1008",
			Error: i18ncore.I18nMessage{
				Key:          "flows.executor.errors.invalid_otp",
				DefaultValue: "Invalid OTP provided",
			},
			ErrorDescription: i18ncore.I18nMessage{
				Key:          "flows.executor.errors.invalid_otp_desc",
				DefaultValue: "The one-time password provided is invalid or has expired",
			},
		},
	}

	publishNodeExecutionCompletedEvent(ctx, mockNode, nodeResp, nil, 1000, 2000, mockObservability)

	s.NotNil(capturedEvent)
	s.Equal(string(event.EventTypeFlowNodeExecutionCompleted), capturedEvent.Type)
	s.Equal(event.StatusSuccess, capturedEvent.Status)

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

	var capturedEvent *event.Event
	mockObservability.On("PublishEvent", mock.Anything, mock.AnythingOfType("*event.Event")).
		Run(func(args mock.Arguments) {
			capturedEvent = args.Get(1).(*event.Event)
		}).Return()

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("test-node")
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)

	ctx := &EngineContext{
		ExecutionID: "exec-123",
		FlowType:    common.FlowTypeAuthentication,
		AppID:       "app-456",
		ExecutionHistory: map[string]*common.NodeExecutionRecord{
			"test-node": {
				Step:       1,
				Executions: []common.ExecutionAttempt{{Attempt: 1}},
			},
		},
	}

	nodeErr := &serviceerror.ServiceError{
		Code: "SVC-50001",
		Error: i18ncore.I18nMessage{
			Key:          "service.errors.internal",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: i18ncore.I18nMessage{
			Key:          "service.errors.internal_desc",
			DefaultValue: "An unexpected error occurred",
		},
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusFailure,
		Error: &serviceerror.ServiceError{
			Code: "FET-1008",
			Error: i18ncore.I18nMessage{
				Key:          "flows.executor.errors.invalid_otp",
				DefaultValue: "Invalid OTP provided",
			},
		},
	}

	publishNodeExecutionCompletedEvent(ctx, mockNode, nodeResp, nodeErr, 1000, 2000, mockObservability)

	s.NotNil(capturedEvent)
	s.Equal(string(event.EventTypeFlowNodeExecutionFailed), capturedEvent.Type)
	s.Equal(event.StatusFailure, capturedEvent.Status)

	errorData, ok := capturedEvent.Data[event.DataKey.Error].(map[string]interface{})
	s.True(ok)
	s.Equal("SVC-50001", errorData["code"], "nodeErr should take precedence over nodeResp.Error")
}

func (s *EngineTestSuite) TestPublishNodeExecutionCompletedEvent_NoErrorPublishedWhenBothNil() {
	t := s.T()
	mockObservability := observabilitymock.NewObservabilityServiceInterfaceMock(t)
	mockObservability.On("IsEnabled").Return(true)

	var capturedEvent *event.Event
	mockObservability.On("PublishEvent", mock.Anything, mock.AnythingOfType("*event.Event")).
		Run(func(args mock.Arguments) {
			capturedEvent = args.Get(1).(*event.Event)
		}).Return()

	mockNode := coremock.NewNodeInterfaceMock(t)
	mockNode.On("GetID").Return("test-node")
	mockNode.On("GetType").Return(common.NodeTypeTaskExecution)

	ctx := &EngineContext{
		ExecutionID: "exec-123",
		FlowType:    common.FlowTypeAuthentication,
		AppID:       "app-456",
		ExecutionHistory: map[string]*common.NodeExecutionRecord{
			"test-node": {
				Step:       1,
				Executions: []common.ExecutionAttempt{{Attempt: 1}},
			},
		},
	}

	nodeResp := &common.NodeResponse{
		Status: common.NodeStatusComplete,
	}

	publishNodeExecutionCompletedEvent(ctx, mockNode, nodeResp, nil, 1000, 2000, mockObservability)

	s.NotNil(capturedEvent)
	s.Equal(string(event.EventTypeFlowNodeExecutionCompleted), capturedEvent.Type)
	s.Equal(event.StatusSuccess, capturedEvent.Status)
	_, hasError := capturedEvent.Data[event.DataKey.Error]
	s.False(hasError)
}

func (s *EngineTestSuite) TestProcessNodeResponseErrorForEventPublish_ReturnsErrorDetails() {
	nodeResp := &common.NodeResponse{
		Error: &serviceerror.ServiceError{
			Code: "FET-1021",
			Error: i18ncore.I18nMessage{
				Key:          "flows.executor.errors.auth_failed",
				DefaultValue: "Authentication failed",
			},
			ErrorDescription: i18ncore.I18nMessage{
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
	svcErr := &serviceerror.ServiceError{
		Code: "SVC-50001",
		Error: i18ncore.I18nMessage{
			Key:          "service.errors.internal",
			DefaultValue: "Internal server error",
		},
		ErrorDescription: i18ncore.I18nMessage{
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

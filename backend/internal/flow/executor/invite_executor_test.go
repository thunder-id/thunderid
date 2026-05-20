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

package executor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type InviteExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory *coremock.FlowFactoryInterfaceMock
	executor        *inviteExecutor
}

func (suite *InviteExecutorTestSuite) SetupTest() {
	// Initialize runtime config for tests
	err := config.InitializeServerRuntime(".", &config.Config{
		GateClient: config.GateClientConfig{
			Scheme:   "https",
			Hostname: "localhost",
			Port:     5190,
			Path:     "/gate",
		},
	})
	suite.Require().NoError(err)

	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())

	// Set up expectations for CreateExecutor (called in constructor)
	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameInviteExecutor,
		common.ExecutorTypeUtility,
		[]common.Input{
			{
				Identifier: userInputInviteToken,
				Type:       "HIDDEN",
				Required:   true,
			},
		},
		[]common.Input{}).Return(mockBaseExecutor)

	suite.executor = newInviteExecutor(suite.mockFlowFactory)
}

func (suite *InviteExecutorTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		EntityID:     "test-app-id",
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyStoredInviteToken])
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink])
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "inviteToken=")
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "executionId=test-flow-id")
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "applicationId=test-app-id")
	assert.Empty(suite.T(), resp.AdditionalData[common.DataInviteLink])
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_UserOnboarding_ExposesInviteLink() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		EntityID:     "test-app-id",
		FlowType:     common.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink])
	assert.Equal(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], resp.AdditionalData[common.DataInviteLink])
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_Idempotency() {
	existingToken := "existing-token-123"
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData: map[string]string{
			common.RuntimeKeyStoredInviteToken: existingToken,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), existingToken, resp.RuntimeData[common.RuntimeKeyStoredInviteToken])
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], existingToken)
	assert.Empty(suite.T(), resp.AdditionalData[common.DataInviteLink])
}

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_NoTokenProvided() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeVerify,
		UserInputs:   make(map[string]string),
		RuntimeData: map[string]string{
			common.RuntimeKeyStoredInviteToken: "stored-token",
		},
	}

	mockExecutor := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(false)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
}

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_ValidationSuccess() {
	token := "valid-token"
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputInviteToken: token,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyStoredInviteToken: token,
		},
	}

	mockExecutor := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_ValidationFailure_Mismatch() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputInviteToken: "wrong-token",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyStoredInviteToken: "correct-token",
		},
	}

	mockExecutor := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Invalid invite token", resp.FailureReason)
}

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_ValidationFailure_NoStoredToken() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputInviteToken: "some-token",
		},
		RuntimeData: make(map[string]string),
	}

	mockExecutor := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Invalid invite token", resp.FailureReason)
}

func (suite *InviteExecutorTestSuite) TestExecute_InvalidMode() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: "invalid",
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
	assert.Contains(suite.T(), err.Error(), "invalid executor mode for InviteExecutor")
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_PopulatesTemplateData() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeGenerate,
		FlowType:     common.FlowTypeRegistration,
		RuntimeData:  make(map[string]string),
		Application: appmodel.Application{
			Name: "Test App",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)

	// 1. Ensure TemplateName is completely gone
	_, hasTemplateName := resp.ForwardedData["templateName"]
	suite.False(hasTemplateName, "Template name should no longer be set by Invite Executor")

	// 2. Ensure TemplateData IS set with the link
	templateData, ok := resp.ForwardedData[common.ForwardedDataKeyTemplateData].(map[string]interface{})
	suite.True(ok, "Expected template data to be map[string]interface{}")
	suite.NotEmpty(templateData["inviteLink"], "inviteLink must be present")
	suite.Equal("Test App", templateData["appName"])
}

func (suite *InviteExecutorTestSuite) TestGetExecutionPolicy_GenerateMode_ReturnsNil() {
	policy := suite.executor.GetExecutionPolicy(ExecutorModeGenerate)
	assert.Nil(suite.T(), policy)
}

func (suite *InviteExecutorTestSuite) TestGetExecutionPolicy_VerifyMode_SkipsChallengeValidation() {
	policy := suite.executor.GetExecutionPolicy(ExecutorModeVerify)
	assert.NotNil(suite.T(), policy)
	assert.True(suite.T(), policy.SkipChallengeValidation)
}

func (suite *InviteExecutorTestSuite) TestGetExecutionPolicy_VerifyMode_AllowsSegmentRestart() {
	policy := suite.executor.GetExecutionPolicy(ExecutorModeVerify)
	assert.NotNil(suite.T(), policy)
	assert.True(suite.T(), policy.AllowSegmentRestart)
}

func (suite *InviteExecutorTestSuite) TestGetExecutionPolicy_InvalidMode_ReturnsNil() {
	policy := suite.executor.GetExecutionPolicy("invalid-mode")
	assert.Nil(suite.T(), policy)
}

func (suite *InviteExecutorTestSuite) TestGetExecutionPolicy_EmptyMode_ReturnsNil() {
	policy := suite.executor.GetExecutionPolicy("")
	assert.Nil(suite.T(), policy)
}

func TestInviteExecutorSuite(t *testing.T) {
	suite.Run(t, new(InviteExecutorTestSuite))
}

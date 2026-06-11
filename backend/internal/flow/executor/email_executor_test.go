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
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/email"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/tests/mocks/emailmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/templatemock"
)

type EmailExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory     *coremock.FlowFactoryInterfaceMock
	mockEmailClient     *emailmock.EmailClientInterfaceMock
	mockTemplateService *templatemock.TemplateServiceInterfaceMock
	mockEntityProvider  *entityprovidermock.EntityProviderInterfaceMock
	executor            *emailExecutor
}

func (suite *EmailExecutorTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockEmailClient = emailmock.NewEmailClientInterfaceMock(suite.T())
	suite.mockTemplateService = templatemock.NewTemplateServiceInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameEmailExecutor,
		common.ExecutorTypeUtility,
		[]common.Input{
			{Identifier: userAttributeEmail, Type: common.InputTypeEmail, Required: true},
		},
		[]common.Input{},
	).Return(mockBaseExecutor)

	suite.executor = newEmailExecutor(
		suite.mockFlowFactory,
		suite.mockEmailClient,
		suite.mockTemplateService,
		suite.mockEntityProvider,
	)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_UserInviteTemplate_Success() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     common.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
	).Return(&template.RenderedTemplate{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"user@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataEmailSent])
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_SelfRegistration_InviteLinkNotExposed() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "SELF_REGISTRATION",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioSelfRegistration,
		template.TemplateTypeEmail,
		template.TemplateData{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
	).Return(&template.RenderedTemplate{
		Subject: "Complete Your Registration",
		Body:    "<html><body>Click to register</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"user@example.com"},
		Subject: "Complete Your Registration",
		Body:    "<html><body>Click to register</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataEmailSent])
	suite.Empty(resp.AdditionalData[common.DataInviteLink])
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_UsesRuntimeRecipientOverUserInput() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			"email":                     "runtime@example.com",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{
			"email":                     "runtime@example.com",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
	).Return(&template.RenderedTemplate{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"runtime@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_EmailFromRuntimeData() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: make(map[string]string),
		RuntimeData: map[string]string{
			"email":                     "runtime@example.com",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{
			"email":                     "runtime@example.com",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
	).Return(&template.RenderedTemplate{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"runtime@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_MissingRecipient() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: make(map[string]string),
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal("Email recipient is required", resp.Error.Error.DefaultValue)
	suite.mockEmailClient.AssertNumberOfCalls(suite.T(), "Send", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_MissingInviteLink() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{},
	).Return(&template.RenderedTemplate{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"user@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_SelfRegistration_MissingInviteLink() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			"emailTemplate": "SELF_REGISTRATION",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioSelfRegistration,
		template.TemplateTypeEmail,
		template.TemplateData{},
	).Return(&template.RenderedTemplate{
		Subject: "Complete Your Registration",
		Body:    "<html><body>Click to register</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"user@example.com"},
		Subject: "Complete Your Registration",
		Body:    "<html><body>Click to register</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_MissingTemplateProperty_Fails() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Contains(err.Error(), "missing required property: emailTemplate")
	suite.Nil(resp)
	suite.mockTemplateService.AssertNumberOfCalls(suite.T(), "Render", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_EmptyTemplateString_Fails() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Contains(err.Error(), "email template property is empty in node configuration")
	suite.Nil(resp)
	suite.mockTemplateService.AssertNumberOfCalls(suite.T(), "Render", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_InvalidTemplateType_ReturnsError() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": 123,
		},
	}

	resp, err := suite.executor.Execute(ctx)
	if suite.Error(err) {
		suite.Contains(err.Error(), "invalid type for emailTemplate")
	}
	suite.Nil(resp)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_TemplateRenderError() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
	).Return(nil, &serviceerror.ServiceError{Code: "TMP-5000"})

	resp, err := suite.executor.Execute(ctx)
	if suite.Error(err) {
		suite.Contains(err.Error(), "failed to render email template: TMP-5000")
	}
	suite.Nil(resp)
	suite.mockEmailClient.AssertNumberOfCalls(suite.T(), "Send", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_NilTemplateService() {
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFactory.On("CreateExecutor",
		ExecutorNameEmailExecutor,
		common.ExecutorTypeUtility,
		[]common.Input{
			{Identifier: userAttributeEmail, Type: common.InputTypeEmail, Required: true},
		},
		[]common.Input{},
	).Return(mockBaseExecutor)

	noServiceExecutor := newEmailExecutor(mockFactory, suite.mockEmailClient, nil, suite.mockEntityProvider)

	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	resp, err := noServiceExecutor.Execute(ctx)
	if suite.Error(err) {
		suite.Contains(err.Error(), "template service is not configured")
	}
	suite.Nil(resp)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ClientError() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
	).Return(&template.RenderedTemplate{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"user@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(email.ErrorInvalidRecipient)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal(ErrEmailSendFailed.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_KnownSMTPErrors() {
	cases := []struct {
		name    string
		sendErr error
	}{
		{"SMTPConnectionError", email.ErrorSMTPConnection},
		{"SMTPAuthError", email.ErrorSMTPAuth},
		{"EmailSendFailedError", email.ErrorEmailSendFailed},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctx := &core.NodeContext{
				ExecutionID:  "test-execution-id",
				ExecutorMode: ExecutorModeSend,
				NodeInputs: []common.Input{
					{Identifier: "email", Type: common.InputTypeEmail, Required: true},
				},
				UserInputs: map[string]string{
					"email": "user@example.com",
				},
				RuntimeData: map[string]string{
					common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
				},
				NodeProperties: map[string]interface{}{
					"emailTemplate": "USER_INVITE",
				},
			}

			suite.mockTemplateService.On("Render",
				ctx.Context,
				template.ScenarioUserInvite,
				template.TemplateTypeEmail,
				template.TemplateData{
					common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
				},
			).Return(&template.RenderedTemplate{
				Subject: "You're Invited to Register",
				Body:    "<html><body>Complete Registration</body></html>",
				IsHTML:  true,
			}, nil)

			expectedEmail := email.EmailData{
				To:      []string{"user@example.com"},
				Subject: "You're Invited to Register",
				Body:    "<html><body>Complete Registration</body></html>",
				IsHTML:  true,
			}
			suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(tc.sendErr)

			resp, err := suite.executor.Execute(ctx)

			suite.NoError(err)
			suite.Equal(common.ExecFailure, resp.Status)
			suite.Equal(ErrEmailSendFailed.Error.DefaultValue, resp.Error.Error.DefaultValue)
			suite.Empty(resp.AdditionalData[common.DataEmailSent])
		})
	}
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_UnexpectedError() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
	).Return(&template.RenderedTemplate{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"user@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(fmt.Errorf("unexpected internal error"))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal(ErrEmailSendFailed.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_NilEmailClient_ReturnsFailure() {
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFactory.On("CreateExecutor",
		ExecutorNameEmailExecutor,
		common.ExecutorTypeUtility,
		[]common.Input{
			{Identifier: userAttributeEmail, Type: common.InputTypeEmail, Required: true},
		},
		[]common.Input{},
	).Return(mockBaseExecutor)

	noEmailExecutor := newEmailExecutor(mockFactory, nil, suite.mockTemplateService, suite.mockEntityProvider)

	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     common.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	resp, err := noEmailExecutor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal(dataValueFalse, resp.AdditionalData[common.DataEmailSent])
	suite.Equal(ErrEmailServiceNotConfigured.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_CustomEmailIdentifier() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "workemail", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"workemail": "workmail@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:8090/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.assertExecuteSendSuccess(ctx, "workmail@example.com")
}

func (suite *EmailExecutorTestSuite) TestExecute_InvalidMode() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: "invalid",
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)
	if suite.Error(err) {
		suite.Contains(err.Error(), "invalid executor mode for EmailExecutor")
	}
	suite.Nil(resp)
}

func (suite *EmailExecutorTestSuite) assertExecuteSendSuccess(ctx *core.NodeContext, expectedRecipient string) {
	// Dynamically build the strictly expected template data from the provided ctx
	expectedTemplateData := template.TemplateData{}
	if ctx.RuntimeData != nil {
		for k, v := range ctx.RuntimeData {
			expectedTemplateData[k] = fmt.Sprintf("%v", v)
		}
	}

	suite.mockTemplateService.On("Render",
		mock.Anything,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		expectedTemplateData,
	).Return(&template.RenderedTemplate{
		Subject: "You're Invited",
		Body:    "<html><body>Registration</body></html>",
		IsHTML:  true,
	}, nil)

	var sentEmail email.EmailData
	suite.mockEmailClient.On("Send", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		sentEmail = args.Get(1).(email.EmailData)
	}).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
	suite.Equal([]string{expectedRecipient}, sentEmail.To)
}

func TestEmailExecutorSuite(t *testing.T) {
	suite.Run(t, new(EmailExecutorTestSuite))
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ResolvesEmailFromForwardedData() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		ForwardedData: map[string]interface{}{
			userAttributeEmail: "forwarded@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
	).Return(&template.RenderedTemplate{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"forwarded@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_UsesNodePropertiesAndForwardedData() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		ForwardedData: map[string]interface{}{
			userAttributeEmail: "forwarded@example.com",
			common.ForwardedDataKeyTemplateData: map[string]interface{}{
				"magicLink":     "https://localhost:5190/gate/signin?token=abc",
				"expiryMinutes": "5",
			},
		},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{
			"magicLink":     "https://localhost:5190/gate/signin?token=abc",
			"expiryMinutes": "5",
		},
	).Return(&template.RenderedTemplate{
		Subject: "Sign in to your account",
		Body:    "<html><body>Magic Link</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"forwarded@example.com"},
		Subject: "Sign in to your account",
		Body:    "<html><body>Magic Link</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ResolvesEmailUsingConfiguredInputIdentifier() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "workEmail", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"workEmail": "configured@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
	).Return(&template.RenderedTemplate{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"configured@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ResolvesEmailFromEntityProvider() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "workEmail", Type: common.InputTypeEmail, Required: true},
		},
		RuntimeData: map[string]string{
			userAttributeUserID:         "test-db-user-id",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	mockEntity := &entityprovider.Entity{
		ID:         "test-db-user-id",
		Attributes: []byte(`{"workEmail":"database-resolved@example.com"}`),
	}
	suite.mockEntityProvider.On("GetEntity", "test-db-user-id").Return(mockEntity, nil)

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioUserInvite,
		template.TemplateTypeEmail,
		template.TemplateData{
			userAttributeUserID:         "test-db-user-id",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
	).Return(&template.RenderedTemplate{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}, nil)

	expectedEmail := email.EmailData{
		To:      []string{"database-resolved@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockEmailClient.On("Send", mock.Anything, expectedEmail).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ForwardedDataInvalidType() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		ForwardedData: map[string]interface{}{
			userAttributeEmail: 12345, // invalid type
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal(ErrEmailRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockEmailClient.AssertNumberOfCalls(suite.T(), "Send", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_EntityProviderMissingEmailAttribute() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "workEmail", Type: common.InputTypeEmail, Required: true},
		},
		RuntimeData: map[string]string{
			userAttributeUserID:         "test-db-user-id",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	mockEntity := &entityprovider.Entity{
		ID:         "test-db-user-id",
		Attributes: []byte(`{"other":"data"}`),
	}
	suite.mockEntityProvider.On("GetEntity", "test-db-user-id").Return(mockEntity, nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal(ErrEmailRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockEmailClient.AssertNumberOfCalls(suite.T(), "Send", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_SkipDelivery() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		RuntimeData: map[string]string{
			common.RuntimeKeySkipDelivery: dataValueTrue,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecComplete, resp.Status)
	suite.mockEmailClient.AssertNumberOfCalls(suite.T(), "Send", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_EntityProviderError() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		RuntimeData: map[string]string{
			userAttributeUserID: "test-user-id",
		},
	}

	suite.mockEntityProvider.On("GetEntity", "test-user-id").Return(
		nil, entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError, "provider error", "system failure"))

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "failed to fetch user from entity provider")
	suite.mockEmailClient.AssertNumberOfCalls(suite.T(), "Send", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_EntityProviderUserNotFound() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		RuntimeData: map[string]string{
			userAttributeUserID: "non-existent-user-id",
		},
	}

	suite.mockEntityProvider.On("GetEntity", "non-existent-user-id").Return(
		nil, entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeEntityNotFound, "user not found", "entity not found"))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal(ErrEmailRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockEmailClient.AssertNumberOfCalls(suite.T(), "Send", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_NilEntityProvider_ReturnsError() {
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFactory.On("CreateExecutor",
		ExecutorNameEmailExecutor,
		common.ExecutorTypeUtility,
		[]common.Input{
			{Identifier: userAttributeEmail, Type: common.InputTypeEmail, Required: true},
		},
		[]common.Input{},
	).Return(mockBaseExecutor)

	noProviderExecutor := newEmailExecutor(mockFactory, suite.mockEmailClient, suite.mockTemplateService, nil)

	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		RuntimeData: map[string]string{
			userAttributeUserID: "test-user-id",
		},
	}

	resp, err := noProviderExecutor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "entity provider is not configured for email resolution")
	suite.mockEmailClient.AssertNumberOfCalls(suite.T(), "Send", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_InvalidNodePropertyScenario() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		ForwardedData: map[string]interface{}{
			userAttributeEmail: "forwarded@example.com",
		},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "NON_EXISTENT_TEMPLATE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioType("NON_EXISTENT_TEMPLATE"),
		template.TemplateTypeEmail,
		template.TemplateData{},
	).Return(nil, &serviceerror.ServiceError{Code: "TMP-404"})

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to render email template")
	suite.Nil(resp)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_MissingEmailInputConfig_FallsBackToDefault() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     common.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeSend,
		NodeInputs:   []common.Input{},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioType("USER_INVITE"),
		template.TemplateTypeEmail,
		template.TemplateData{},
	).Return(&template.RenderedTemplate{
		Subject: "Invite",
		Body:    "Welcome",
		IsHTML:  false,
	}, nil)

	suite.mockEmailClient.On("Send", mock.Anything, mock.MatchedBy(func(d email.EmailData) bool {
		return len(d.To) == 1 && d.To[0] == "user@example.com"
	})).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(common.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ApplicationNameInTemplateData() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     common.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []common.Input{
			{Identifier: "email", Type: common.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
		},
	}
	ctx.Application.Name = "Test Application"

	expectedTemplateData := template.TemplateData{
		"appName": "Test Application",
	}

	suite.mockTemplateService.On("Render",
		ctx.Context,
		template.ScenarioType("USER_INVITE"),
		template.TemplateTypeEmail,
		expectedTemplateData,
	).Return(&template.RenderedTemplate{
		Subject: "Test App Invite",
		Body:    "Welcome to Test App",
		IsHTML:  false,
	}, nil)

	suite.mockEmailClient.On("Send", mock.Anything, mock.MatchedBy(func(d email.EmailData) bool {
		return len(d.To) == 1 && d.To[0] == "user@example.com"
	})).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(common.ExecComplete, resp.Status)
}

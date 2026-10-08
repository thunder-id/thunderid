// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"fmt"
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/notification"
	notifcm "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

// Flow template property values (used directly as template handles).
const (
	emailUserInviteProperty = "user-invite"
	emailUserInviteHandle   = "user-invite"
	emailSelfRegProperty    = "self-registration"
	emailSelfRegHandle      = "self-registration"
)

type EmailExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory      *coremock.FlowFactoryInterfaceMock
	mockNotifSenderSvc   *notificationmock.NotificationSenderServiceInterfaceMock
	mockTemplateRenderer *notificationTemplateRendererMock
	mockEntityProvider   *entityprovidermock.EntityProviderInterfaceMock
	executor             *emailExecutor
}

func (suite *EmailExecutorTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockNotifSenderSvc = notificationmock.NewNotificationSenderServiceInterfaceMock(suite.T())
	suite.mockTemplateRenderer = newNotificationTemplateRendererMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameEmailExecutor,
		providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: userAttributeEmail, Type: providers.InputTypeEmail, Required: true},
		},
		[]providers.Input{},
		mock.Anything,
	).Return(mockBaseExecutor)

	suite.executor = newEmailExecutor(
		suite.mockFlowFactory,
		suite.mockNotifSenderSvc,
		suite.mockTemplateRenderer,
		suite.mockEntityProvider,
	)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_UserInviteTemplate_Success() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     providers.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"user@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataEmailSent])
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_SelfRegistration_InviteLinkNotExposed() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailSelfRegProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailSelfRegHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "Complete Your Registration",
		Body:    "<html><body>Click to register</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"user@example.com"},
		Subject: "Complete Your Registration",
		Body:    "<html><body>Click to register</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataEmailSent])
	suite.Empty(resp.AdditionalData[common.DataInviteLink])
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_UsesRuntimeRecipientOverUserInput() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			"email":                     "runtime@example.com",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				"email":                     "runtime@example.com",
				common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"runtime@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_EmailFromRuntimeData() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: make(map[string]string),
		RuntimeData: map[string]string{
			"email":                     "runtime@example.com",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				"email":                     "runtime@example.com",
				common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"runtime@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_MissingRecipient() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: make(map[string]string),
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal("Email recipient is required", resp.Error.Error.DefaultValue)
	suite.mockNotifSenderSvc.AssertNumberOfCalls(suite.T(), "SendEmail", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_MissingInviteLink() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{Data: map[string]string{}},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"user@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_SelfRegistration_MissingInviteLink() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailSelfRegProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailSelfRegHandle,
		notificationtemplate.RenderInput{Data: map[string]string{}},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "Complete Your Registration",
		Body:    "<html><body>Click to register</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"user@example.com"},
		Subject: "Complete Your Registration",
		Body:    "<html><body>Click to register</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_MissingTemplateProperty_Fails() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
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
	suite.mockTemplateRenderer.AssertNumberOfCalls(suite.T(), "Resolve", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_EmptyTemplateString_Fails() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
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
	suite.mockTemplateRenderer.AssertNumberOfCalls(suite.T(), "Resolve", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_InvalidTemplateType_ReturnsError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
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
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
			},
		},
	).Return(nil, &tidcommon.ServiceError{Code: "TMP-5000"})

	resp, err := suite.executor.Execute(ctx)
	if suite.Error(err) {
		suite.Contains(err.Error(), "failed to render email template: TMP-5000")
	}
	suite.Nil(resp)
	suite.mockNotifSenderSvc.AssertNumberOfCalls(suite.T(), "SendEmail", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_NilTemplateRenderer() {
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFactory.On("CreateExecutor",
		ExecutorNameEmailExecutor,
		providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: userAttributeEmail, Type: providers.InputTypeEmail, Required: true},
		},
		[]providers.Input{},
		mock.Anything,
	).Return(mockBaseExecutor)

	noServiceExecutor := newEmailExecutor(mockFactory, suite.mockNotifSenderSvc, nil, suite.mockEntityProvider)

	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	resp, err := noServiceExecutor.Execute(ctx)
	if suite.Error(err) {
		suite.Contains(err.Error(), "template renderer is not configured")
	}
	suite.Nil(resp)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ClientError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"user@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return(&notification.ErrorInvalidProvider)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrEmailProviderNotConfigured.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ClientErrorsBecomeProviderNotConfigured() {
	cases := []struct {
		name    string
		sendErr *tidcommon.ServiceError
	}{
		{"UnsupportedChannel", &notification.ErrorUnsupportedChannel},
		{"SenderTypeMismatch", &notification.ErrorRequestedSenderIsNotOfExpectedType},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctx := &providers.NodeContext{
				ExecutionID:  "test-execution-id",
				ExecutorMode: ExecutorModeSend,
				NodeInputs: []providers.Input{
					{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
				},
				UserInputs: map[string]string{
					"email": "user@example.com",
				},
				RuntimeData: map[string]string{
					common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
				},
				NodeProperties: map[string]interface{}{
					"emailTemplate": emailUserInviteProperty,
				},
			}

			suite.mockTemplateRenderer.On("Resolve",
				ctx.Context,
				notificationtemplate.ChannelTypeEmail,
				emailUserInviteHandle,
				notificationtemplate.RenderInput{
					Data: map[string]string{
						common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?" +
							"executionId=test&inviteToken=abc",
					},
				},
			).Return(&notificationtemplate.ResolvedContent{
				Subject: "You're Invited to Register",
				Body:    "<html><body>Complete Registration</body></html>",
			}, nil)

			expectedEmail := notifcm.EmailData{
				To:      []string{"user@example.com"},
				Subject: "You're Invited to Register",
				Body:    "<html><body>Complete Registration</body></html>",
				IsHTML:  true,
			}
			suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).Return(tc.sendErr)

			resp, err := suite.executor.Execute(ctx)

			suite.NoError(err)
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Equal(ErrEmailProviderNotConfigured.Error.DefaultValue, resp.Error.Error.DefaultValue)
			suite.Equal(dataValueFalse, resp.AdditionalData[common.DataEmailSent])
		})
	}
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_UnexpectedError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"user@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return(&tidcommon.InternalServerError)

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_NoProviderConfigured_ReturnsFailure() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     providers.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		mock.Anything,
	).Return(&notificationtemplate.ResolvedContent{Subject: "s", Body: "b"}, nil)

	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", mock.Anything).
		Return(&notification.ErrorSenderNotFound)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(dataValueFalse, resp.AdditionalData[common.DataEmailSent])
	suite.Equal(ErrEmailProviderNotConfigured.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

// A configured senderId is passed through to the sender service verbatim.
func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ConfiguredSenderID() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
			"senderId":      "smtp-sender-001",
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		"USER_INVITE",
		mock.Anything,
	).Return(&notificationtemplate.ResolvedContent{Subject: "s", Body: "b"}, nil)

	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "smtp-sender-001", mock.Anything).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataEmailSent])
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_NonStringSenderID_ReturnsError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "USER_INVITE",
			"senderId":      123,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.mockNotifSenderSvc.AssertNumberOfCalls(suite.T(), "SendEmail", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_NilSenderService_ReturnsError() {
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFactory.On("CreateExecutor",
		ExecutorNameEmailExecutor,
		providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: userAttributeEmail, Type: providers.InputTypeEmail, Required: true},
		},
		[]providers.Input{},
		mock.Anything,
	).Return(mockBaseExecutor)

	noSenderExecutor := newEmailExecutor(mockFactory, nil, suite.mockTemplateRenderer, suite.mockEntityProvider)

	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs:     map[string]string{"email": "user@example.com"},
		NodeProperties: map[string]interface{}{"emailTemplate": "USER_INVITE"},
	}

	resp, err := noSenderExecutor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_CustomEmailIdentifier() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "workemail", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"workemail": "workmail@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:8090/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.assertExecuteSendSuccess(ctx, "workmail@example.com")
}

func (suite *EmailExecutorTestSuite) TestExecute_InvalidMode() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: "invalid",
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
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

func (suite *EmailExecutorTestSuite) assertExecuteSendSuccess(ctx *providers.NodeContext, expectedRecipient string) {
	// Dynamically build the strictly expected template data from the provided ctx
	expectedTemplateData := map[string]string{}
	if ctx.RuntimeData != nil {
		for k, v := range ctx.RuntimeData {
			expectedTemplateData[k] = fmt.Sprintf("%v", v)
		}
	}
	if ctx.Application.Name != "" {
		expectedTemplateData["appName"] = ctx.Application.Name
	}

	suite.mockTemplateRenderer.On("Resolve",
		mock.Anything,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{Data: expectedTemplateData},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "You're Invited",
		Body:    "<html><body>Registration</body></html>",
	}, nil)

	var sentEmail notifcm.EmailData
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", mock.Anything).Run(func(args mock.Arguments) {
		sentEmail = args.Get(2).(notifcm.EmailData)
	}).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal([]string{expectedRecipient}, sentEmail.To)
}

// A claim is only a fallback for the recipient, so an address the flow collected is not redirected to
// one the external party asserted.
func (suite *EmailExecutorTestSuite) TestResolveRecipientEmail_UserInputWinsOverExternalClaim() {
	ctx := &providers.NodeContext{
		UserInputs: map[string]string{"email": "entered@example.com"},
		RuntimeData: map[string]string{
			common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-1", "sub-1",
				map[string]interface{}{"email": "claimed@example.com"}),
		},
	}

	recipient, err := suite.executor.resolveRecipientEmail(ctx, log.GetLogger())

	suite.NoError(err)
	suite.Equal("entered@example.com", recipient)

	delete(ctx.UserInputs, "email")
	recipient, err = suite.executor.resolveRecipientEmail(ctx, log.GetLogger())

	suite.NoError(err)
	suite.Equal("claimed@example.com", recipient)
}

func (suite *EmailExecutorTestSuite) TestResolveTemplateData_ExternalClaims() {
	ctx := &providers.NodeContext{
		Application: providers.Application{Name: "MyApp"},
		RuntimeData: map[string]string{
			common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-1", "sub-1", map[string]interface{}{
				"userID": "victim-id",
				"name":   "Claimed",
				"mobile": float64(94771234567),
			}),
			"userID": "real-id",
			"name":   "",
			"code":   "",
		},
	}

	data := suite.executor.resolveTemplateData(ctx)

	suite.Equal("real-id", data["userID"], "runtime data must win over a claim of the same name")
	suite.Equal("Claimed", data["name"], "an empty runtime value must not hide the claim")
	suite.Contains(data, "code", "an empty runtime value with no claim behind it is still rendered")
	suite.Equal("94771234567", data["mobile"], "numeric claims render as the claim readers see them")
	suite.NotContains(data, common.RuntimeKeyExternalIdentity)
}

func TestEmailExecutorSuite(t *testing.T) {
	suite.Run(t, new(EmailExecutorTestSuite))
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ResolvesEmailFromForwardedData() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		ForwardedData: map[string]interface{}{
			userAttributeEmail: "forwarded@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"forwarded@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_UsesNodePropertiesAndForwardedData() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		ForwardedData: map[string]interface{}{
			userAttributeEmail: "forwarded@example.com",
			common.ForwardedDataKeyTemplateData: map[string]interface{}{
				"magicLink":  "https://localhost:5190/gate/signin?token=abc",
				"expiryTime": "5 minutes",
			},
		},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				"magicLink":  "https://localhost:5190/gate/signin?token=abc",
				"expiryTime": "5 minutes",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "Sign in to your account",
		Body:    "<html><body>Magic Link</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"forwarded@example.com"},
		Subject: "Sign in to your account",
		Body:    "<html><body>Magic Link</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ResolvesEmailUsingConfiguredInputIdentifier() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "workEmail", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"workEmail": "configured@example.com",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"configured@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ResolvesEmailFromEntityProvider() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "workEmail", Type: providers.InputTypeEmail, Required: true},
		},
		RuntimeData: map[string]string{
			userAttributeUserID:         "test-db-user-id",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	mockEntity := &providers.Entity{
		ID:         "test-db-user-id",
		Attributes: []byte(`{"workEmail":"database-resolved@example.com"}`),
	}
	suite.mockEntityProvider.On("GetEntity", "test-db-user-id").Return(mockEntity, nil)

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				userAttributeUserID:         "test-db-user-id",
				common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
	}, nil)

	expectedEmail := notifcm.EmailData{
		To:      []string{"database-resolved@example.com"},
		Subject: "You're Invited to Register",
		Body:    "<html><body>Complete Registration</body></html>",
		IsHTML:  true,
	}
	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", expectedEmail).
		Return((*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ForwardedDataInvalidType() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		ForwardedData: map[string]interface{}{
			userAttributeEmail: 12345, // invalid type
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrEmailRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockNotifSenderSvc.AssertNumberOfCalls(suite.T(), "SendEmail", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_EntityProviderMissingEmailAttribute() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "workEmail", Type: providers.InputTypeEmail, Required: true},
		},
		RuntimeData: map[string]string{
			userAttributeUserID:         "test-db-user-id",
			common.RuntimeKeyInviteLink: "https://localhost:5190/gate/invite?executionId=test&inviteToken=abc",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	mockEntity := &providers.Entity{
		ID:         "test-db-user-id",
		Attributes: []byte(`{"other":"data"}`),
	}
	suite.mockEntityProvider.On("GetEntity", "test-db-user-id").Return(mockEntity, nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrEmailRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockNotifSenderSvc.AssertNumberOfCalls(suite.T(), "SendEmail", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_SkipDelivery() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		RuntimeData: map[string]string{
			common.RuntimeKeySkipDelivery: dataValueTrue,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataEmailSent])
	suite.mockNotifSenderSvc.AssertNumberOfCalls(suite.T(), "SendEmail", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_EntityProviderError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
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
	suite.mockNotifSenderSvc.AssertNumberOfCalls(suite.T(), "SendEmail", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_EntityProviderUserNotFound() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
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
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrEmailRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockNotifSenderSvc.AssertNumberOfCalls(suite.T(), "SendEmail", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_NilEntityProvider_ReturnsError() {
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFactory.On("CreateExecutor",
		ExecutorNameEmailExecutor,
		providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: userAttributeEmail, Type: providers.InputTypeEmail, Required: true},
		},
		[]providers.Input{},
		mock.Anything,
	).Return(mockBaseExecutor)

	noProviderExecutor := newEmailExecutor(mockFactory, suite.mockNotifSenderSvc, suite.mockTemplateRenderer, nil)

	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		RuntimeData: map[string]string{
			userAttributeUserID: "test-user-id",
		},
	}

	resp, err := noProviderExecutor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "entity provider is not configured for email resolution")
	suite.mockNotifSenderSvc.AssertNumberOfCalls(suite.T(), "SendEmail", 0)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_CustomHandleFromNodeProperty() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		ForwardedData: map[string]interface{}{
			userAttributeEmail: "forwarded@example.com",
		},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			"emailTemplate": "non-existent-template",
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		"non-existent-template",
		notificationtemplate.RenderInput{Data: map[string]string{}},
	).Return(nil, &tidcommon.ServiceError{Code: "TMP-404"})

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Contains(err.Error(), "failed to render email template")
	suite.Nil(resp)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_MissingEmailInputConfig_FallsBackToDefault() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     providers.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeSend,
		NodeInputs:   []providers.Input{},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{Data: map[string]string{}},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "Invite",
		Body:    "Welcome",
	}, nil)

	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", mock.MatchedBy(func(d notifcm.EmailData) bool {
		return len(d.To) == 1 && d.To[0] == "user@example.com"
	})).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *EmailExecutorTestSuite) TestExecute_SendMode_ApplicationNameInTemplateData() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		FlowType:     providers.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeSend,
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
		UserInputs: map[string]string{
			"email": "user@example.com",
		},
		NodeProperties: map[string]interface{}{
			"emailTemplate": emailUserInviteProperty,
		},
	}
	ctx.Application.Name = "Test Application"

	expectedTemplateData := map[string]string{
		"appName": "Test Application",
	}

	suite.mockTemplateRenderer.On("Resolve",
		ctx.Context,
		notificationtemplate.ChannelTypeEmail,
		emailUserInviteHandle,
		notificationtemplate.RenderInput{Data: expectedTemplateData},
	).Return(&notificationtemplate.ResolvedContent{
		Subject: "Test App Invite",
		Body:    "Welcome to Test App",
	}, nil)

	suite.mockNotifSenderSvc.On("SendEmail", mock.Anything, "", mock.MatchedBy(func(d notifcm.EmailData) bool {
		return len(d.To) == 1 && d.To[0] == "user@example.com"
	})).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.NotNil(resp)
	suite.Equal(providers.ExecComplete, resp.Status)
}

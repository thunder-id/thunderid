// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	notifcm "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
)

const testRenderedSMSBody = "Your notification from the system."

// Flow template property values (used directly as template handles).
const (
	smsSelfRegistrationProperty = "self-registration"
	smsSelfRegistrationHandle   = "self-registration"
	smsCIBAProperty             = "ciba-notification"
	smsCIBAHandle               = "ciba-notification"
)

type SMSExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory      *coremock.FlowFactoryInterfaceMock
	mockBaseExecutor     *coremock.ExecutorInterfaceMock
	mockSMSSenderSvc     *notificationmock.NotificationSenderServiceInterfaceMock
	mockTemplateRenderer *notificationTemplateRendererMock
	executor             *smsExecutor
}

func (suite *SMSExecutorTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockBaseExecutor = coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockSMSSenderSvc = notificationmock.NewNotificationSenderServiceInterfaceMock(suite.T())
	suite.mockTemplateRenderer = newNotificationTemplateRendererMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameSMSExecutor,
		providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
		},
		[]providers.Input{},
		mock.Anything,
	).Return(suite.mockBaseExecutor)

	suite.executor = newSMSExecutor(suite.mockFlowFactory, suite.mockSMSSenderSvc, suite.mockTemplateRenderer, nil)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_Success() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("SendMessage",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.MessageData{Recipient: "+94714627887", Body: testRenderedSMSBody},
	).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_RecipientFromRuntimeData() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs:   make(map[string]string),
		RuntimeData: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("SendMessage",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.MessageData{Recipient: "+94714627887", Body: testRenderedSMSBody},
	).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_UserInputOverridesRuntimeData() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: map[string]string{
			common.AttributeMobileNumber: "+94771111111",
		},
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("SendMessage",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.MessageData{Recipient: "+94714627887", Body: testRenderedSMSBody},
	).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_CustomPhoneAttribute() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			"phoneNumber": "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeInputs: []providers.Input{
			{Identifier: "phoneNumber", Type: providers.InputTypePhone, Required: true},
		},
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "phoneNumber", Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("SendMessage",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.MessageData{Recipient: "+94714627887", Body: testRenderedSMSBody},
	).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
}

func (suite *SMSExecutorTestSuite) TestExecute_PrerequisiteNotMet_ReturnsFailure() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrSMSRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_MissingRecipient() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrSMSRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_MissingSenderID() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData:    make(map[string]string),
		NodeProperties: map[string]interface{}{},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "senderId is not configured")
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_InvalidSenderIDType() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: 123,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "senderId is not configured")
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_InvalidPhoneNumber() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "not-a-phone",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrSMSInvalidPhone.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_SkipDelivery() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeySkipDelivery: dataValueTrue,
		},
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_NilSMSSenderService_ReturnsError() {
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFactory.On("CreateExecutor",
		ExecutorNameSMSExecutor,
		providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
		},
		[]providers.Input{},
		mock.Anything,
	).Return(mockBaseExecutor)

	noServiceExecutor := newSMSExecutor(mockFactory, nil, suite.mockTemplateRenderer, nil)

	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
		},
	}

	resp, err := noServiceExecutor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.EqualError(err, "notification sender service is not configured")
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_UserOnboarding_ClientError_ReturnsExecFailure() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		FlowType:     providers.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil)
	clientErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "MNS-1001",
		Error: tidcommon.I18nMessage{Key: "error.test.sender_not_found", DefaultValue: "Sender not found"},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.test.the_requested_notification_sender_could_not_be_fou",
			DefaultValue: "The requested notification sender could not be found",
		},
	}
	suite.mockSMSSenderSvc.On("SendMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(clientErr)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrSMSProviderNotConfigured.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_UserOnboarding_ServerError_ReturnsError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		FlowType:     providers.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil)
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "MNS-5000",
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.internal_server_error", DefaultValue: "internal server error",
		},
	}
	suite.mockSMSSenderSvc.On("SendMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(serverErr)

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "SMS send failed")
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_OtherFlow_NotificationError_ReturnsError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil)
	clientErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "MNS-1001",
		Error: tidcommon.I18nMessage{Key: "error.test.sender_not_found", DefaultValue: "Sender not found"},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.test.the_requested_notification_sender_could_not_be_fou",
			DefaultValue: "The requested notification sender could not be found",
		},
	}
	suite.mockSMSSenderSvc.On("SendMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(clientErr)

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "SMS send failed")
}

func (suite *SMSExecutorTestSuite) TestExecute_NoSMSTemplateProperty_ReturnsFlowError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrSMSTemplateMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockTemplateRenderer.AssertNotCalled(suite.T(), "Resolve",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_EmptySMSTemplateProperty_ReturnsFlowError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          "",
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrSMSTemplateMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockTemplateRenderer.AssertNotCalled(suite.T(), "Resolve",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SMSTemplatePropertySet_UsesCustomHandle() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("SendMessage",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.MessageData{Recipient: "+94714627887", Body: testRenderedSMSBody},
	).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.mockTemplateRenderer.AssertCalled(suite.T(), "Resolve",
		mock.Anything, notificationtemplate.ChannelTypeSMS, smsSelfRegistrationHandle, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_TemplateRenderFailure_ReturnsError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	renderErr := &tidcommon.ServiceError{Code: "TPL-5000"}
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(nil, renderErr)

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "failed to render SMS template")
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_TemplateDataIncludesRuntimeData() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		Application:  providers.Application{Name: "MyApp"},
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyBindingMessage: "Approve sign-in",
		},
		ForwardedData: map[string]interface{}{
			common.ForwardedDataKeyTemplateData: map[string]interface{}{
				"inviteLink": "https://localhost:5190/gate/invite?executionId=test",
			},
		},
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsCIBAProperty,
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS, smsCIBAHandle,
		notificationtemplate.RenderInput{
			Data: map[string]string{
				common.RuntimeKeyBindingMessage: "Approve sign-in",
				"appName":                       "MyApp",
				"inviteLink":                    "https://localhost:5190/gate/invite?executionId=test",
			},
		},
	).Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("SendMessage",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.MessageData{Recipient: "+94714627887", Body: testRenderedSMSBody},
	).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *SMSExecutorTestSuite) TestResolveTemplateData_ExternalClaims() {
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

func TestSMSExecutorSuite(t *testing.T) {
	suite.Run(t, new(SMSExecutorTestSuite))
}

// testPinnedMobile is the number a mobile verification test pins.
const testPinnedMobile = "+94714627887"

// A skipped send reports the SMS as sent, unless a contact is pinned.
func (suite *SMSExecutorTestSuite) TestAttributeVerificationSkippedDeliveryDoesNotReportSent() {
	for want, shared := range map[string]map[string]string{
		dataValueTrue:  {},
		dataValueFalse: verificationEvidenceData(suite.T(), &verificationEvidence{Pin: mobilePin()}),
	} {
		response, err := suite.executor.Execute(&providers.NodeContext{
			FlowType:          providers.FlowTypeSelfService,
			ExecutorMode:      ExecutorModeSend,
			RuntimeData:       map[string]string{common.RuntimeKeySkipDelivery: dataValueTrue},
			SharedRuntimeData: shared,
		})
		suite.Require().NoError(err)
		suite.Equal(providers.ExecComplete, response.Status)
		suite.Equal(want, response.AdditionalData[common.DataSMSSent])
	}
}

// verificationSMSContext is a send step with the given runtime number, inputs and evidence, if any.
func verificationSMSContext(t *testing.T, flowType providers.FlowType, runtimeNumber string, inputs map[string]string,
	proof *verificationEvidence) *providers.NodeContext {
	shared := map[string]string{}
	if proof != nil {
		shared = verificationEvidenceData(t, proof)
	}
	return &providers.NodeContext{
		FlowType:          flowType,
		ExecutorMode:      ExecutorModeSend,
		UserInputs:        inputs,
		RuntimeData:       map[string]string{common.AttributeMobileNumber: runtimeNumber},
		SharedRuntimeData: shared,
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}
}

// mobilePin is a pinned mobile contact for user-a.
func mobilePin() *providers.AttributeVerificationPin {
	return &providers.AttributeVerificationPin{Subject: "user-a", Attribute: "mobile_number",
		Value: testPinnedMobile, Channel: providers.VerificationChannelSMS}
}

// expectSMS expects one SMS to the given number.
func (suite *SMSExecutorTestSuite) expectSMS(recipient string) {
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil).Once()
	suite.mockSMSSenderSvc.On("SendMessage", mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.MessageData{Recipient: recipient, Body: testRenderedSMSBody}).Return(nil).Once()
}

// A pin is enforced in any flow type that has one.
func (suite *SMSExecutorTestSuite) TestAttributeVerificationRecordsDeliveryToThePinnedNumber() {
	for _, flowType := range []providers.FlowType{providers.FlowTypeSelfService, providers.FlowTypeAdministration} {
		suite.Run(string(flowType), func() {
			suite.SetupTest()
			suite.expectSMS(testPinnedMobile)

			response, err := suite.executor.Execute(verificationSMSContext(suite.T(), flowType, testPinnedMobile,
				map[string]string{}, &verificationEvidence{Pin: mobilePin(), Challenge: "session-1"}))

			suite.Require().NoError(err)
			suite.Equal(providers.ExecComplete, response.Status)
			suite.Equal([]verificationReceipt{{Channel: "sms", Value: testPinnedMobile}},
				evidenceIn(suite.T(), response.SharedRuntimeData).Receipts)
			suite.Equal("sms", response.AdditionalData[common.DataVerificationChannel])
			suite.Equal("•••"+testPinnedMobile[len(testPinnedMobile)-2:],
				response.AdditionalData[common.DataMaskedDestination])
		})
	}
}

func (suite *SMSExecutorTestSuite) TestAttributeVerificationRefusesAnEmailPin() {
	response, err := suite.executor.Execute(verificationSMSContext(suite.T(),
		providers.FlowTypeSelfService, testPinnedMobile, map[string]string{},
		&verificationEvidence{Pin: emailPin(), Challenge: "session-1"}))

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, response.Status)
	suite.Equal(ErrVerificationChannelUnavailable.Code, response.Error.Code)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestAttributeVerificationIgnoresAnInputNumber() {
	for _, flowType := range []providers.FlowType{providers.FlowTypeSelfService, providers.FlowTypeAuthentication} {
		suite.Run(string(flowType), func() {
			suite.SetupTest()
			suite.expectSMS(testPinnedMobile)

			response, err := suite.executor.Execute(verificationSMSContext(suite.T(), flowType, testPinnedMobile,
				map[string]string{common.AttributeMobileNumber: "+94770000000"},
				&verificationEvidence{Pin: mobilePin(), Challenge: "session-1"}))

			suite.Require().NoError(err)
			suite.Equal(providers.ExecComplete, response.Status)
			suite.Equal([]verificationReceipt{{Channel: "sms", Value: testPinnedMobile}},
				evidenceIn(suite.T(), response.SharedRuntimeData).Receipts)
		})
	}
}

func (suite *SMSExecutorTestSuite) TestAttributeVerificationRefusesAnUnpinnedDelivery() {
	for name, test := range map[string]struct {
		runtimeNumber string
		inputs        map[string]string
		challenge     string
	}{
		"another number": {runtimeNumber: "+94770000000", challenge: "session-1"},
		"an input number with none pinned in runtime data": {
			inputs: map[string]string{common.AttributeMobileNumber: "+94770000000"}, challenge: "session-1"},
		"no code issued": {runtimeNumber: testPinnedMobile},
	} {
		suite.Run(name, func() {
			suite.SetupTest()

			response, err := suite.executor.Execute(verificationSMSContext(suite.T(),
				providers.FlowTypeSelfService, test.runtimeNumber, test.inputs,
				&verificationEvidence{Pin: mobilePin(), Challenge: test.challenge}))

			suite.Require().NoError(err)
			suite.Equal(providers.ExecFailure, response.Status)
			suite.Equal(ErrVerificationChannelUnavailable.Code, response.Error.Code)
			suite.Empty(response.SharedRuntimeData[common.RuntimeKeyAttributeVerificationEvidence])
			suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage", mock.Anything, mock.Anything,
				mock.Anything, mock.Anything)
		})
	}
}

// Without a pin a send behaves as before in every flow type: an input number comes first.
func (suite *SMSExecutorTestSuite) TestSendWithoutAPinTakesTheInputNumber() {
	for _, flowType := range []providers.FlowType{providers.FlowTypeSelfService, providers.FlowTypeAuthentication} {
		suite.Run(string(flowType), func() {
			suite.SetupTest()
			suite.expectSMS("+94770000000")

			response, err := suite.executor.Execute(verificationSMSContext(suite.T(), flowType,
				testPinnedMobile, map[string]string{common.AttributeMobileNumber: "+94770000000"}, nil))

			suite.Require().NoError(err)
			suite.Equal(providers.ExecComplete, response.Status)
			suite.Empty(response.SharedRuntimeData[common.RuntimeKeyAttributeVerificationEvidence])
		})
	}
}

func (suite *SMSExecutorTestSuite) TestAttributeVerificationFailedSendAddsNoReceipt() {
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil).Once()
	suite.mockSMSSenderSvc.On("SendMessage", mock.Anything, mock.Anything, "sender-uuid-001", mock.Anything).
		Return(&tidcommon.InternalServerError).Once()

	response, err := suite.executor.Execute(verificationSMSContext(suite.T(),
		providers.FlowTypeSelfService, testPinnedMobile, map[string]string{},
		&verificationEvidence{Pin: mobilePin(), Challenge: "session-1"}))

	suite.Error(err)
	suite.Nil(response)
}

func (suite *SMSExecutorTestSuite) TestAttributeVerificationCorruptEvidenceIsAnError() {
	_, err := suite.executor.Execute(&providers.NodeContext{
		FlowType:          providers.FlowTypeSelfService,
		ExecutorMode:      ExecutorModeSend,
		RuntimeData:       map[string]string{common.AttributeMobileNumber: testPinnedMobile},
		SharedRuntimeData: map[string]string{common.RuntimeKeyAttributeVerificationEvidence: "{"},
	})

	suite.Error(err)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything)
}

// In a self-service flow, an unresolved or unknown sender fails the node as an unconfigured provider.
func (suite *SMSExecutorTestSuite) TestExecute_SendMode_SelfService_ClientError_ReturnsExecFailure() {
	for _, senderID := range []string{"{{SENDER_ID}}", "unknown-sender"} {
		suite.Run(senderID, func() {
			suite.SetupTest()
			suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
				smsSelfRegistrationHandle, mock.Anything).
				Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil).Once()
			suite.mockSMSSenderSvc.On("SendMessage", mock.Anything, notifcm.ChannelTypeSMS, senderID, mock.Anything).
				Return(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "MNS-1001"}).Once()
			ctx := verificationSMSContext(suite.T(), providers.FlowTypeSelfService, testPinnedMobile,
				map[string]string{}, &verificationEvidence{Pin: mobilePin(), Challenge: "session-1"})
			ctx.NodeProperties[propertyKeyNotificationSenderID] = senderID

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Equal(ErrSMSProviderNotConfigured.Code, resp.Error.Code)
			suite.NotEqual(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
			suite.Empty(resp.SharedRuntimeData[common.RuntimeKeyAttributeVerificationEvidence], "nothing was delivered")
		})
	}
}

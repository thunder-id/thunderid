// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	notifcm "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
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
	suite.Equal(map[string]string{common.RuntimeKeyDeliveredRecipients: deliveredTo(suite.T(), "+94714627887")},
		resp.SharedRuntimeData, "a send must record where it went")
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
	suite.Nil(resp.SharedRuntimeData, "a skipped send must record no send")
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
	suite.Nil(resp.SharedRuntimeData, "a failed send must record no send")
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

// A mobile recipient an earlier node set wins over every other source of the number, and the send records
// it. A cleared recipient, or one set for another recipient type, leaves the number to the other sources.
func (suite *SMSExecutorTestSuite) TestExecute_SendMode_RecipientSetByAnEarlierNodeWins() {
	for name, test := range map[string]struct {
		recipient string
		arrange   func(ctx *providers.NodeContext)
		want      string
	}{
		"over a user input": {testPinnedMobile, func(ctx *providers.NodeContext) {
			ctx.UserInputs[common.AttributeMobileNumber] = "+94770000000"
		}, testPinnedMobile},
		"over runtime data": {testPinnedMobile, func(ctx *providers.NodeContext) {
			ctx.RuntimeData[common.AttributeMobileNumber] = "+94770000000"
		}, testPinnedMobile},
		// The entity provider mock fails the test if the profile is read.
		"over the profile": {testPinnedMobile, func(ctx *providers.NodeContext) {
			ctx.RuntimeData[userAttributeUserID] = "test-user-id"
		}, testPinnedMobile},
		"cleared": {"", func(ctx *providers.NodeContext) {
			ctx.UserInputs[common.AttributeMobileNumber] = "+94770000000"
		}, "+94770000000"},
		"set for an email recipient": {"user@example.com", func(ctx *providers.NodeContext) {
			ctx.RuntimeData[common.RuntimeKeyRecipientType] = entitytypemodel.TypeEmail
			ctx.UserInputs[common.AttributeMobileNumber] = "+94770000000"
		}, "+94770000000"},
		"set without a recipient type": {testPinnedMobile, func(ctx *providers.NodeContext) {
			delete(ctx.RuntimeData, common.RuntimeKeyRecipientType)
			ctx.UserInputs[common.AttributeMobileNumber] = "+94770000000"
		}, "+94770000000"},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.executor.entityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
			ctx := &providers.NodeContext{
				ExecutionID:  "test-flow-id",
				FlowType:     providers.FlowTypeSelf,
				ExecutorMode: ExecutorModeSend,
				UserInputs:   map[string]string{},
				RuntimeData: map[string]string{common.RuntimeKeyRecipientType: entitytypemodel.TypeMobile,
					common.RuntimeKeyExpectedRecipient: test.recipient},
				NodeProperties: map[string]interface{}{
					propertyKeyNotificationSenderID: "sender-uuid-001",
					propertyKeySMSTemplate:          smsSelfRegistrationProperty,
				},
			}
			test.arrange(ctx)
			suite.expectSMS(test.want)

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(providers.ExecComplete, resp.Status)
			suite.Equal(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
			suite.Equal(map[string]string{common.RuntimeKeyDeliveredRecipients: deliveredTo(suite.T(), test.want)},
				resp.SharedRuntimeData, "the send must record where it went")
		})
	}
}

// In a self flow, an unresolved or unknown sender fails the node as an unconfigured provider, and
// records no send.
func (suite *SMSExecutorTestSuite) TestExecute_SendMode_SelfFlow_ClientError_ReturnsExecFailure() {
	for _, senderID := range []string{"{{SENDER_ID}}", "unknown-sender"} {
		suite.Run(senderID, func() {
			suite.SetupTest()
			suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
				smsSelfRegistrationHandle, mock.Anything).
				Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil).Once()
			suite.mockSMSSenderSvc.On("SendMessage", mock.Anything, notifcm.ChannelTypeSMS, senderID, mock.Anything).
				Return(&tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "MNS-1001"}).Once()
			ctx := &providers.NodeContext{
				FlowType:     providers.FlowTypeSelf,
				ExecutorMode: ExecutorModeSend,
				RuntimeData: map[string]string{common.RuntimeKeyRecipientType: entitytypemodel.TypeMobile,
					common.RuntimeKeyExpectedRecipient: testPinnedMobile},
				NodeProperties: map[string]interface{}{
					propertyKeyNotificationSenderID: senderID,
					propertyKeySMSTemplate:          smsSelfRegistrationProperty,
				},
			}

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Equal(ErrSMSProviderNotConfigured.Code, resp.Error.Code)
			suite.NotEqual(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
			suite.Nil(resp.SharedRuntimeData, "a failed send must record no send")
		})
	}
}

// expectSMS expects one SMS to the given number.
func (suite *SMSExecutorTestSuite) expectSMS(recipient string) {
	suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
		smsSelfRegistrationHandle, mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil).Once()
	suite.mockSMSSenderSvc.On("SendMessage", mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.MessageData{Recipient: recipient, Body: testRenderedSMSBody}).Return(nil).Once()
}

// A send adds its recipient to the recipients sent to before it, once, and a list that does not decode
// fails the node before anything is sent.
func (suite *SMSExecutorTestSuite) TestSendAddsToTheDeliveredRecipients() {
	for name, test := range map[string]struct{ earlier, want string }{
		"a new recipient": {deliveredTo(suite.T(), "user@example.com"),
			deliveredTo(suite.T(), "user@example.com", testPinnedMobile)},
		"a recipient already sent to": {deliveredTo(suite.T(), testPinnedMobile),
			deliveredTo(suite.T(), testPinnedMobile)},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
				smsSelfRegistrationHandle, mock.Anything).
				Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil).Once()
			suite.mockSMSSenderSvc.On("SendMessage", mock.Anything, mock.Anything, "sender-uuid-001",
				notifcm.MessageData{Recipient: testPinnedMobile, Body: testRenderedSMSBody}).Return(nil).Once()

			resp, err := suite.executor.Execute(suite.deliveryContext(test.earlier))

			suite.Require().NoError(err)
			suite.Equal(providers.ExecComplete, resp.Status)
			suite.Equal(map[string]string{common.RuntimeKeyDeliveredRecipients: test.want}, resp.SharedRuntimeData)
		})
	}

	suite.Run("a list that does not decode", func() {
		suite.SetupTest()
		suite.mockTemplateRenderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeSMS,
			smsSelfRegistrationHandle, mock.Anything).
			Return(&notificationtemplate.ResolvedContent{Body: testRenderedSMSBody}, nil).Maybe()

		resp, err := suite.executor.Execute(suite.deliveryContext(testPinnedMobile))

		suite.Error(err)
		suite.Nil(resp)
		suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "SendMessage", mock.Anything, mock.Anything,
			mock.Anything, mock.Anything)
	})
}

// deliveryContext is a send to the pinned mobile after earlier sends recorded as the delivered recipients.
func (suite *SMSExecutorTestSuite) deliveryContext(delivered string) *providers.NodeContext {
	return &providers.NodeContext{
		ExecutionID:       "test-flow-id",
		ExecutorMode:      ExecutorModeSend,
		UserInputs:        map[string]string{common.AttributeMobileNumber: testPinnedMobile},
		RuntimeData:       map[string]string{},
		SharedRuntimeData: map[string]string{common.RuntimeKeyDeliveredRecipients: delivered},
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          smsSelfRegistrationProperty,
		},
	}
}

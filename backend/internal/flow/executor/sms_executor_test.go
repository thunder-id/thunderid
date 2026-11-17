// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	notifcm "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/template"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
	"github.com/thunder-id/thunderid/tests/mocks/templatemock"
)

const testRenderedSMSBody = "Your notification from the system."

type SMSExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory     *coremock.FlowFactoryInterfaceMock
	mockBaseExecutor    *coremock.ExecutorInterfaceMock
	mockSMSSenderSvc    *notificationmock.NotificationSenderServiceInterfaceMock
	mockTemplateService *templatemock.TemplateServiceInterfaceMock
	executor            *smsExecutor
}

func (suite *SMSExecutorTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockBaseExecutor = coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockSMSSenderSvc = notificationmock.NewNotificationSenderServiceInterfaceMock(suite.T())
	suite.mockTemplateService = templatemock.NewTemplateServiceInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameSMSExecutor,
		providers.ExecutorTypeUtility,
		[]providers.Input{
			{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
		},
		[]providers.Input{},
		mock.Anything,
	).Return(suite.mockBaseExecutor)

	suite.executor = newSMSExecutor(suite.mockFlowFactory, suite.mockSMSSenderSvc, suite.mockTemplateService, nil, nil)
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
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("Send",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.NotificationData{Recipient: "+94714627887", Body: testRenderedSMSBody},
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
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("Send",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.NotificationData{Recipient: "+94714627887", Body: testRenderedSMSBody},
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
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("Send",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.NotificationData{Recipient: "+94714627887", Body: testRenderedSMSBody},
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
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: "phoneNumber", Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("Send",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.NotificationData{Recipient: "+94714627887", Body: testRenderedSMSBody},
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
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrSMSRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
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
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
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
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
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
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
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

	noServiceExecutor := newSMSExecutor(mockFactory, nil, suite.mockTemplateService, nil, nil)

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
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	clientErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "MNS-1001",
		Error: tidcommon.I18nMessage{Key: "error.test.sender_not_found", DefaultValue: "Sender not found"},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.test.the_requested_notification_sender_could_not_be_fou",
			DefaultValue: "The requested notification sender could not be found",
		},
	}
	suite.mockSMSSenderSvc.On("Send", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	serverErr := &tidcommon.ServiceError{
		Type: tidcommon.ServerErrorType,
		Code: "MNS-5000",
		ErrorDescription: tidcommon.I18nMessage{
			Key: "error.test.internal_server_error", DefaultValue: "internal server error",
		},
	}
	suite.mockSMSSenderSvc.On("Send", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	clientErr := &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "MNS-1001",
		Error: tidcommon.I18nMessage{Key: "error.test.sender_not_found", DefaultValue: "Sender not found"},
		ErrorDescription: tidcommon.I18nMessage{
			Key:          "error.test.the_requested_notification_sender_could_not_be_fou",
			DefaultValue: "The requested notification sender could not be found",
		},
	}
	suite.mockSMSSenderSvc.On("Send", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
	suite.mockTemplateService.AssertNotCalled(suite.T(), "Render", mock.Anything, mock.Anything, mock.Anything)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
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
	suite.mockTemplateService.AssertNotCalled(suite.T(), "Render", mock.Anything, mock.Anything, mock.Anything)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SMSTemplatePropertySet_UsesCustomScenario() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("Send",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.NotificationData{Recipient: "+94714627887", Body: testRenderedSMSBody},
	).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.mockTemplateService.AssertCalled(suite.T(), "Render",
		mock.Anything, template.ScenarioSelfRegistration, template.TemplateTypeSMS, mock.Anything)
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
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	renderErr := &tidcommon.ServiceError{Code: "TPL-5000"}
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(nil, renderErr)

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "failed to render SMS template")
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
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
			propertyKeySMSTemplate:          string(template.ScenarioCIBANotification),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioCIBANotification,
		template.TemplateTypeSMS,
		template.TemplateData{
			common.RuntimeKeyBindingMessage: "Approve sign-in",
			"appName":                       "MyApp",
			"inviteLink":                    "https://localhost:5190/gate/invite?executionId=test",
		},
	).Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	suite.mockSMSSenderSvc.On("Send",
		mock.Anything, mock.Anything, "sender-uuid-001",
		notifcm.NotificationData{Recipient: "+94714627887", Body: testRenderedSMSBody},
	).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func TestSMSExecutorSuite(t *testing.T) {
	suite.Run(t, new(SMSExecutorTestSuite))
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_IdentifiedUserGetsTheStoredMobile() {
	const stored, other = "+94711111111", "+94770000000"
	for name, test := range map[string]struct {
		profile string
		arrange func(ctx *providers.NodeContext)
		want    string
	}{
		"over a user input": {`{"mobile_number":"` + stored + `"}`, func(ctx *providers.NodeContext) {
			ctx.UserInputs[common.AttributeMobileNumber] = other
		}, stored},
		"over runtime data": {`{"mobile_number":"` + stored + `"}`, func(ctx *providers.NodeContext) {
			ctx.RuntimeData[common.AttributeMobileNumber] = other
		}, stored},
		"without a stored mobile": {`{"username":"testuser"}`, func(ctx *providers.NodeContext) {
			ctx.UserInputs[common.AttributeMobileNumber] = other
		}, ""},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			entityProvider := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
			entityProvider.On("GetEntity", "test-user-id").Return(
				&providers.Entity{ID: "test-user-id", Attributes: []byte(test.profile)}, nil)
			suite.executor.entityProvider = entityProvider
			ctx := &providers.NodeContext{
				ExecutionID:  "test-flow-id",
				FlowType:     providers.FlowTypeRecovery,
				ExecutorMode: ExecutorModeSend,
				UserInputs:   map[string]string{},
				RuntimeData:  map[string]string{userAttributeUserID: "test-user-id"},
				NodeProperties: map[string]interface{}{
					propertyKeyNotificationSenderID: "sender-uuid-001",
					propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
				},
			}
			test.arrange(ctx)
			if test.want != "" {
				suite.expectSMS(test.want)
			}

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			if test.want == "" {
				suite.Equal(providers.ExecFailure, resp.Status)
				suite.Equal(ErrSMSRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
				suite.mockSMSSenderSvc.AssertNumberOfCalls(suite.T(), "Send", 0)
				return
			}
			suite.Equal(providers.ExecComplete, resp.Status)
		})
	}
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_IdentifiedUserProfileLookup() {
	for name, test := range map[string]struct {
		providerErr *entityprovider.EntityProviderError
		noProvider  bool
		wantErr     bool
	}{
		"no entity provider": {noProvider: true, wantErr: true},
		"a store error": {providerErr: entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeSystemError, "provider error", "system failure"), wantErr: true},
		"user not found": {providerErr: entityprovider.NewEntityProviderError(
			entityprovider.ErrorCodeEntityNotFound, "user not found", "entity not found")},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			if !test.noProvider {
				entityProvider := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
				entityProvider.On("GetEntity", "test-user-id").Return(nil, test.providerErr)
				suite.executor.entityProvider = entityProvider
			}
			ctx := &providers.NodeContext{
				ExecutionID:  "test-flow-id",
				ExecutorMode: ExecutorModeSend,
				UserInputs:   map[string]string{common.AttributeMobileNumber: "+94770000000"},
				RuntimeData:  map[string]string{userAttributeUserID: "test-user-id"},
				NodeProperties: map[string]interface{}{
					propertyKeyNotificationSenderID: "sender-uuid-001",
					propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
				},
			}

			resp, err := suite.executor.Execute(ctx)

			if test.wantErr {
				suite.Error(err)
				suite.Nil(resp)
			} else {
				suite.Require().NoError(err)
				suite.Equal(providers.ExecFailure, resp.Status)
				suite.Equal(ErrSMSRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
			}
			suite.mockSMSSenderSvc.AssertNumberOfCalls(suite.T(), "Send", 0)
		})
	}
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_AuthenticatedUserLookupFails() {
	authUser := newAuthenticatedAuthUser()
	authnProvider := managermock.NewAuthnProviderManagerMock(suite.T())
	authnProvider.On("GetEntityReference", mock.Anything, authUser).
		Return(authUser, (*providers.EntityReference)(nil), &tidcommon.InternalServerError)
	suite.executor.authnProvider = authnProvider
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		AuthUser:     authUser,
		UserInputs:   map[string]string{common.AttributeMobileNumber: "+94770000000"},
		RuntimeData:  map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.mockSMSSenderSvc.AssertNumberOfCalls(suite.T(), "Send", 0)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_AuthenticatedUserGetsTheStoredMobile() {
	authUser := newAuthenticatedAuthUser()
	authnProvider := managermock.NewAuthnProviderManagerMock(suite.T())
	authnProvider.On("GetEntityReference", mock.Anything, authUser).
		Return(authUser, &providers.EntityReference{EntityID: "test-user-id"}, (*tidcommon.ServiceError)(nil))
	entityProvider := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	entityProvider.On("GetEntity", "test-user-id").Return(
		&providers.Entity{ID: "test-user-id", Attributes: []byte(`{"mobile_number":"+94711111111"}`)}, nil)
	suite.executor.authnProvider = authnProvider
	suite.executor.entityProvider = entityProvider
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		AuthUser:     authUser,
		UserInputs:   map[string]string{common.AttributeMobileNumber: "+94770000000"},
		RuntimeData:  map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}
	suite.expectSMS("+94711111111")

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_UserWithoutAStoredMobile() {
	for name, test := range map[string]struct {
		signedInAs string
		sent       bool
	}{
		"signed in user gets the submitted mobile":    {signedInAs: "test-user-id", sent: true},
		"user signed in as someone else gets nothing": {signedInAs: "other-user-id"},
		"identified user gets nothing":                {},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			entityProvider := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
			entityProvider.On("GetEntity", "test-user-id").Return(
				&providers.Entity{ID: "test-user-id", Attributes: []byte(`{"username":"testuser"}`)}, nil)
			suite.executor.entityProvider = entityProvider
			ctx := &providers.NodeContext{
				ExecutionID:  "test-flow-id",
				ExecutorMode: ExecutorModeSend,
				UserInputs:   map[string]string{common.AttributeMobileNumber: "+94770000000"},
				RuntimeData:  map[string]string{userAttributeUserID: "test-user-id"},
				NodeProperties: map[string]interface{}{
					propertyKeyNotificationSenderID: "sender-uuid-001",
					propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
				},
			}
			if test.signedInAs != "" {
				ctx.AuthUser = newAuthenticatedAuthUser()
				authnProvider := managermock.NewAuthnProviderManagerMock(suite.T())
				authnProvider.On("GetEntityReference", mock.Anything, ctx.AuthUser).Return(ctx.AuthUser,
					&providers.EntityReference{EntityID: test.signedInAs}, (*tidcommon.ServiceError)(nil))
				suite.executor.authnProvider = authnProvider
			}
			if test.sent {
				suite.expectSMS("+94770000000")
			}

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			if test.sent {
				suite.Equal(providers.ExecComplete, resp.Status)
				return
			}
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Equal(ErrSMSRecipientMissing.Error.DefaultValue, resp.Error.Error.DefaultValue)
			suite.mockSMSSenderSvc.AssertNumberOfCalls(suite.T(), "Send", 0)
		})
	}
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_SignedInUserLookupFailsForASubmittedMobile() {
	for name, svcErr := range map[string]*tidcommon.ServiceError{
		"when the lookup fails":     &tidcommon.InternalServerError,
		"when the user isn't found": {Type: tidcommon.ClientErrorType, Code: "AUP-1001"},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			authUser := newAuthenticatedAuthUser()
			authnProvider := managermock.NewAuthnProviderManagerMock(suite.T())
			authnProvider.On("GetEntityReference", mock.Anything, authUser).
				Return(authUser, (*providers.EntityReference)(nil), svcErr)
			entityProvider := entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
			entityProvider.On("GetEntity", "test-user-id").Return(
				&providers.Entity{ID: "test-user-id", Attributes: []byte(`{"username":"testuser"}`)}, nil)
			suite.executor.authnProvider = authnProvider
			suite.executor.entityProvider = entityProvider
			ctx := &providers.NodeContext{
				ExecutionID:  "test-flow-id",
				ExecutorMode: ExecutorModeSend,
				AuthUser:     authUser,
				UserInputs:   map[string]string{common.AttributeMobileNumber: "+94770000000"},
				RuntimeData:  map[string]string{userAttributeUserID: "test-user-id"},
				NodeProperties: map[string]interface{}{
					propertyKeyNotificationSenderID: "sender-uuid-001",
					propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
				},
			}

			resp, err := suite.executor.Execute(ctx)

			if svcErr.Type == tidcommon.ServerErrorType {
				suite.Error(err)
				suite.Nil(resp)
			} else {
				suite.Require().NoError(err)
				suite.Equal(providers.ExecFailure, resp.Status)
			}
			suite.mockSMSSenderSvc.AssertNumberOfCalls(suite.T(), "Send", 0)
		})
	}
}

// expectSMS expects one SMS to the given number.
func (suite *SMSExecutorTestSuite) expectSMS(recipient string) {
	suite.mockTemplateService.On("Render", mock.Anything, mock.Anything, template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil).Once()
	suite.mockSMSSenderSvc.On("Send", mock.Anything, notifcm.ChannelTypeSMS, "sender-uuid-001",
		notifcm.NotificationData{Recipient: recipient, Body: testRenderedSMSBody}).Return(nil).Once()
}

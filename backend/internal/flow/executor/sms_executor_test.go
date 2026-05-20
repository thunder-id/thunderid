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
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"

	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	notifcm "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/internal/system/template"
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
		common.ExecutorTypeUtility,
		[]common.Input{
			{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
		},
		[]common.Input{},
	).Return(suite.mockBaseExecutor)

	suite.executor = newSMSExecutor(suite.mockFlowFactory, suite.mockSMSSenderSvc, suite.mockTemplateService)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_Success() {
	ctx := &core.NodeContext{
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
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
	suite.Equal(common.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_RecipientFromRuntimeData() {
	ctx := &core.NodeContext{
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
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
	suite.Equal(common.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_UserInputOverridesRuntimeData() {
	ctx := &core.NodeContext{
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
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
	suite.Equal(common.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_CustomPhoneAttribute() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			"phoneNumber": "+94714627887",
		},
		RuntimeData: make(map[string]string),
		NodeInputs: []common.Input{
			{Identifier: "phoneNumber", Type: common.InputTypePhone, Required: true},
		},
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
			propertyKeySMSTemplate:          string(template.ScenarioSelfRegistration),
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "phoneNumber", Type: common.InputTypePhone, Required: true},
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
	suite.Equal(common.ExecComplete, resp.Status)
	suite.Equal(dataValueTrue, resp.AdditionalData[common.DataSMSSent])
}

func (suite *SMSExecutorTestSuite) TestExecute_PrerequisiteNotMet_ReturnsFailure() {
	ctx := &core.NodeContext{
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
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal("SMS recipient is required", resp.FailureReason)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_MissingRecipient() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyNotificationSenderID: "sender-uuid-001",
		},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal("SMS recipient is required", resp.FailureReason)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_MissingSenderID() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeSend,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+94714627887",
		},
		RuntimeData:    make(map[string]string),
		NodeProperties: map[string]interface{}{},
	}

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "senderId is not configured")
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_InvalidSenderIDType() {
	ctx := &core.NodeContext{
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
	suite.Contains(err.Error(), "senderId is not configured")
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_InvalidPhoneNumber() {
	ctx := &core.NodeContext{
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal("SMS recipient is not a valid phone number", resp.FailureReason)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_NilSMSSenderService_ReturnsError() {
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())
	mockFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockFactory.On("CreateExecutor",
		ExecutorNameSMSExecutor,
		common.ExecutorTypeUtility,
		[]common.Input{
			{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
		},
		[]common.Input{},
	).Return(mockBaseExecutor)

	noServiceExecutor := newSMSExecutor(mockFactory, nil, suite.mockTemplateService)

	ctx := &core.NodeContext{
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
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		FlowType:     common.FlowTypeUserOnboarding,
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	clientErr := &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "MNS-1001",
		Error: i18ncore.I18nMessage{Key: "error.test.sender_not_found", DefaultValue: "Sender not found"},
		ErrorDescription: i18ncore.I18nMessage{
			Key:          "error.test.the_requested_notification_sender_could_not_be_fou",
			DefaultValue: "The requested notification sender could not be found",
		},
	}
	suite.mockSMSSenderSvc.On("Send", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(clientErr)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal("Notification configuration is wrong or not set.", resp.FailureReason)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_UserOnboarding_ServerError_ReturnsError() {
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		FlowType:     common.FlowTypeUserOnboarding,
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	serverErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "MNS-5000",
		ErrorDescription: i18ncore.I18nMessage{
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
	ctx := &core.NodeContext{
		ExecutionID:  "test-flow-id",
		FlowType:     common.FlowTypeRegistration,
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
	}).Maybe()
	suite.mockTemplateService.On("Render", mock.Anything, template.ScenarioSelfRegistration,
		template.TemplateTypeSMS, mock.Anything).
		Return(&template.RenderedTemplate{Body: testRenderedSMSBody}, nil)
	clientErr := &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "MNS-1001",
		Error: i18ncore.I18nMessage{Key: "error.test.sender_not_found", DefaultValue: "Sender not found"},
		ErrorDescription: i18ncore.I18nMessage{
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
	ctx := &core.NodeContext{
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal("SMS template is required", resp.FailureReason)
	suite.mockTemplateService.AssertNotCalled(suite.T(), "Render", mock.Anything, mock.Anything, mock.Anything)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_EmptySMSTemplateProperty_ReturnsFlowError() {
	ctx := &core.NodeContext{
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
	}).Maybe()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(common.ExecFailure, resp.Status)
	suite.Equal("SMS template is required", resp.FailureReason)
	suite.mockTemplateService.AssertNotCalled(suite.T(), "Render", mock.Anything, mock.Anything, mock.Anything)
	suite.mockSMSSenderSvc.AssertNotCalled(suite.T(), "Send",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SMSTemplatePropertySet_UsesCustomScenario() {
	ctx := &core.NodeContext{
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
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
	suite.Equal(common.ExecComplete, resp.Status)
	suite.mockTemplateService.AssertCalled(suite.T(), "Render",
		mock.Anything, template.ScenarioSelfRegistration, template.TemplateTypeSMS, mock.Anything)
}

func (suite *SMSExecutorTestSuite) TestExecute_SendMode_TemplateRenderFailure_ReturnsError() {
	ctx := &core.NodeContext{
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

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone, Required: true},
	}).Maybe()
	renderErr := &serviceerror.ServiceError{Code: "TPL-5000"}
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

func TestSMSExecutorSuite(t *testing.T) {
	suite.Run(t, new(SMSExecutorTestSuite))
}

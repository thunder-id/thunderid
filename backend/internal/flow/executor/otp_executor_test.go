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

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authn/otpmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const testOTPUserID = "user-abc-123"

type OTPExecutorTestSuite struct {
	suite.Suite
	mockOTPService     *otpmock.OTPAuthnServiceInterfaceMock
	mockAuthnProvider  *managermock.AuthnProviderManagerInterfaceMock
	mockFlowFactory    *coremock.FlowFactoryInterfaceMock
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	mockBaseExec       *coremock.ExecutorInterfaceMock
	executor           *otpExecutor
}

func TestOTPExecutorSuite(t *testing.T) {
	suite.Run(t, new(OTPExecutorTestSuite))
}

func (suite *OTPExecutorTestSuite) SetupTest() {
	suite.mockOTPService = otpmock.NewOTPAuthnServiceInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())

	defaultInputs := []common.Input{
		{
			Ref:        "otp_input",
			Identifier: userInputOTP,
			Type:       common.InputTypeOTP,
			Required:   true,
		},
	}
	prerequisites := []common.Input{
		{
			Identifier: common.RuntimeKeyOTPSessionToken,
			Type:       common.InputTypeHidden,
			Required:   true,
		},
	}

	suite.mockBaseExec = coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockBaseExec.On("GetName").Return(ExecutorNameOTPExecutor).Maybe()
	suite.mockBaseExec.On("GetType").Return(common.ExecutorTypeAuthentication).Maybe()
	suite.mockBaseExec.On("GetDefaultInputs").Return(defaultInputs).Maybe()
	suite.mockBaseExec.On("GetRequiredInputs", mock.Anything).Return(defaultInputs).Maybe()
	suite.mockBaseExec.On("GetPrerequisites").Return(prerequisites).Maybe()
	suite.mockBaseExec.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).Return(true).Maybe()
	suite.mockBaseExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *core.NodeContext, execResp *common.ExecutorResponse) bool {
			otp, exists := ctx.UserInputs[userInputOTP]
			if !exists || otp == "" {
				execResp.Inputs = defaultInputs
				execResp.Status = common.ExecUserInputRequired
				return false
			}
			return true
		}).Maybe()

	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameOTPExecutor, common.ExecutorTypeAuthentication,
		defaultInputs, prerequisites).Return(suite.mockBaseExec)

	suite.executor = newOTPExecutor(suite.mockFlowFactory, suite.mockOTPService,
		suite.mockAuthnProvider, suite.mockEntityProvider)
	suite.executor.ExecutorInterface = suite.mockBaseExec
}

// Generate mode tests

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_UserInputRequired_NoSearchAttrs() {
	ctx := &core.NodeContext{
		ExecutionID:  "exec-1",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []common.Input{
			{Ref: "otp_input", Identifier: userInputOTP, Type: common.InputTypeOTP, Required: true},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_Success_UserIdentifiedAndOTPGenerated() {
	userID := testOTPUserID
	suite.mockEntityProvider.On("IdentifyEntity", mock.MatchedBy(func(attrs map[string]interface{}) bool {
		_, hasMobile := attrs[common.AttributeMobileNumber]
		return hasMobile
	})).Return(&userID, nil)

	suite.mockOTPService.On("GenerateOTP", mock.Anything, userID, authnprovidercm.UserAttributeUserID).
		Return("session-tok-1", "654321", (*serviceerror.ServiceError)(nil))

	ctx := &core.NodeContext{
		ExecutionID:  "exec-2",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []common.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: common.InputTypePhone, Required: true},
			{Ref: "otp_input", Identifier: userInputOTP, Type: common.InputTypeOTP, Required: true},
		},
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "session-tok-1", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
	assert.Equal(suite.T(), "654321", resp.RuntimeData[common.RuntimeKeyOTPValue])
	assert.Equal(suite.T(), "1", resp.RuntimeData[common.RuntimeKeyOTPAttemptCount])
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_MultipleInputs_IdentifiesUserByAllAttrs() {
	userID := testOTPUserID
	suite.mockEntityProvider.On("IdentifyEntity", mock.MatchedBy(func(attrs map[string]interface{}) bool {
		_, hasMobile := attrs[common.AttributeMobileNumber]
		_, hasEmail := attrs["email"]
		return hasMobile && hasEmail
	})).Return(&userID, nil)

	suite.mockOTPService.On("GenerateOTP", mock.Anything, userID, authnprovidercm.UserAttributeUserID).
		Return("session-tok-2", "111222", (*serviceerror.ServiceError)(nil))

	ctx := &core.NodeContext{
		ExecutionID:  "exec-3",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []common.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: common.InputTypePhone, Required: true},
			{Ref: "email_input", Identifier: "email", Type: common.InputTypeEmail, Required: true},
			{Ref: "otp_input", Identifier: userInputOTP, Type: common.InputTypeOTP, Required: true},
		},
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
			"email":                      "user@example.com",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "session-tok-2", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_UserNotFound_ReturnsFailure() {
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).
		Return((*string)(nil), (*entityprovider.EntityProviderError)(nil))

	ctx := &core.NodeContext{
		ExecutionID:  "exec-4",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []common.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: common.InputTypePhone, Required: true},
		},
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+9999999999",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.NotNil(suite.T(), resp.Error)
	assert.Equal(suite.T(), ErrUserNotFound.Code, resp.Error.Code)
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_Registration_UserNotFound_UsesMobileDestValue() {
	suite.mockEntityProvider.On("IdentifyEntity", mock.MatchedBy(func(attrs map[string]interface{}) bool {
		_, hasMobile := attrs[common.AttributeMobileNumber]
		return hasMobile
	})).Return((*string)(nil), (*entityprovider.EntityProviderError)(nil))

	suite.mockOTPService.On("GenerateOTP", mock.Anything, "+1234567890", common.AttributeMobileNumber).
		Return("session-reg-1", "777888", (*serviceerror.ServiceError)(nil))

	ctx := &core.NodeContext{
		ExecutionID:  "exec-reg-1",
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []common.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: common.InputTypePhone, Required: true},
			{Ref: "otp_input", Identifier: userInputOTP, Type: common.InputTypeOTP, Required: true},
		},
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "session-reg-1", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
	assert.Equal(suite.T(), "777888", resp.RuntimeData[common.RuntimeKeyOTPValue])
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_Registration_UserNotFound_NoPhoneValue_ReturnsInputRequired() {
	suite.mockEntityProvider.On("IdentifyEntity", mock.MatchedBy(func(attrs map[string]interface{}) bool {
		_, hasUsername := attrs["username"]
		return hasUsername
	})).Return((*string)(nil), (*entityprovider.EntityProviderError)(nil))

	ctx := &core.NodeContext{
		ExecutionID:  "exec-reg-2",
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []common.Input{
			{Ref: "username_input", Identifier: "username", Type: common.InputTypeText, Required: true},
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: common.InputTypePhone, Required: true},
		},
		UserInputs: map[string]string{
			"username": "john",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_MaxAttemptsReached_ReturnsFailure() {
	ctx := &core.NodeContext{
		ExecutionID:  "exec-5",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyOTPAttemptCount: "3",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.NotNil(suite.T(), resp.Error)
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_MaxAttemptsFromNodeProperties() {
	ctx := &core.NodeContext{
		ExecutionID:  "exec-5b",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeProperties: map[string]interface{}{
			propertyKeyMaxOTPAttempts: "2",
		},
		UserInputs: map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyOTPAttemptCount: "2",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.NotNil(suite.T(), resp.Error)
}

// Verify mode tests

func (suite *OTPExecutorTestSuite) TestExecuteVerify_OTPInputRequired_WhenNoOTPProvided() {
	ctx := &core.NodeContext{
		ExecutionID:  "exec-6",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs:   map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyOTPSessionToken: "session-tok-1",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
}

func (suite *OTPExecutorTestSuite) TestExecuteVerify_Success() {
	userID := testOTPUserID

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(
			authnprovidermgr.AuthUser{},
			authnprovidercm.AuthenticatedClaims{userAttributeUserID: userID},
			(*serviceerror.ServiceError)(nil),
		)

	ctx := &core.NodeContext{
		ExecutionID:  "exec-7",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputOTP: "654321",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyOTPSessionToken: "session-tok-1",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
	assert.Equal(suite.T(), userID, resp.RuntimeData[userAttributeUserID])
}

func (suite *OTPExecutorTestSuite) TestExecuteVerify_InvalidOTP_ReturnsUserInputRequired() {
	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(
			authnprovidermgr.AuthUser{},
			authnprovidercm.AuthenticatedClaims(nil),
			&authnprovidermgr.ErrorAuthenticationFailed,
		)

	ctx := &core.NodeContext{
		ExecutionID:  "exec-8",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputOTP: "000000",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyOTPSessionToken: "session-tok-1",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.NotNil(suite.T(), resp.Error)
	assert.Equal(suite.T(), ErrInvalidOTP.Code, resp.Error.Code)
}

func (suite *OTPExecutorTestSuite) TestExecuteVerify_MissingSessionToken_ReturnsError() {
	ctx := &core.NodeContext{
		ExecutionID:  "exec-9",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputOTP: "654321",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.NotNil(suite.T(), resp)
}

func (suite *OTPExecutorTestSuite) TestExecuteVerify_PrerequisiteNotMet_ReturnsFailure() {
	freshMock := coremock.NewExecutorInterfaceMock(suite.T())
	freshMock.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			execResp, _ := args.Get(1).(*common.ExecutorResponse)
			if execResp != nil {
				execResp.Status = common.ExecFailure
			}
		}).Return(false)

	exec := &otpExecutor{
		ExecutorInterface: freshMock,
		entityProvider:    suite.mockEntityProvider,
		otpService:        suite.mockOTPService,
		authnProvider:     suite.mockAuthnProvider,
		logger:            suite.executor.logger,
	}

	ctx := &core.NodeContext{
		ExecutionID:  "exec-prereq",
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputOTP: "654321",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := exec.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	freshMock.AssertNotCalled(suite.T(), "HasRequiredInputs", mock.Anything, mock.Anything)
}

// Invalid mode

func (suite *OTPExecutorTestSuite) TestExecute_InvalidMode_ReturnsError() {
	ctx := &core.NodeContext{
		ExecutionID:  "exec-10",
		ExecutorMode: "unknown",
		UserInputs:   map[string]string{},
		RuntimeData:  map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.NotNil(suite.T(), resp)
}

// buildSearchAttributes

func (suite *OTPExecutorTestSuite) TestBuildSearchAttributes_SkipsOTPIdentifier() {
	ctx := &core.NodeContext{
		NodeInputs: []common.Input{
			{Identifier: userInputOTP, Type: common.InputTypeOTP},
			{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone},
		},
		UserInputs: map[string]string{
			userInputOTP:                 "123456",
			common.AttributeMobileNumber: "+1234567890",
		},
		RuntimeData:   map[string]string{},
		ForwardedData: map[string]interface{}{},
	}

	attrs := suite.executor.buildSearchAttributes(ctx)

	assert.NotContains(suite.T(), attrs, userInputOTP)
	assert.Contains(suite.T(), attrs, common.AttributeMobileNumber)
	assert.Equal(suite.T(), "+1234567890", attrs[common.AttributeMobileNumber])
}

func (suite *OTPExecutorTestSuite) TestBuildSearchAttributes_FallsBackToRuntimeData() {
	ctx := &core.NodeContext{
		NodeInputs: []common.Input{
			{Identifier: common.AttributeMobileNumber, Type: common.InputTypePhone},
		},
		UserInputs: map[string]string{},
		RuntimeData: map[string]string{
			common.AttributeMobileNumber: "+9876543210",
		},
		ForwardedData: map[string]interface{}{},
	}

	attrs := suite.executor.buildSearchAttributes(ctx)

	assert.Equal(suite.T(), "+9876543210", attrs[common.AttributeMobileNumber])
}

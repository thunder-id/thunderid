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
	"github.com/thunder-id/thunderid/internal/authn/otp"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"
	"github.com/thunder-id/thunderid/tests/mocks/authn/otpmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const testOTPUserID = "user-abc-123"

type OTPExecutorTestSuite struct {
	suite.Suite
	mockOTPService     *otpmock.OTPAuthnServiceInterfaceMock
	mockAuthnProvider  *managermock.AuthnProviderManagerMock
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
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())

	defaultInputs := []providers.Input{
		{
			Ref:        "otp_input",
			Identifier: userInputOTP,
			Type:       providers.InputTypeOTP,
			Required:   true,
		},
	}
	prerequisites := []providers.Input{
		{
			Identifier: common.RuntimeKeyOTPSessionToken,
			Type:       providers.InputTypeHidden,
			Required:   true,
		},
	}

	suite.mockBaseExec = coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockBaseExec.On("GetName").Return(ExecutorNameOTPExecutor).Maybe()
	suite.mockBaseExec.On("GetType").Return(providers.ExecutorTypeAuthentication).Maybe()
	suite.mockBaseExec.On("GetDefaultInputs").Return(defaultInputs).Maybe()
	suite.mockBaseExec.On("GetRequiredInputs", mock.Anything).Return(defaultInputs).Maybe()
	suite.mockBaseExec.On("GetPrerequisites").Return(prerequisites).Maybe()
	suite.mockBaseExec.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).Return(true).Maybe()

	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameOTPExecutor, providers.ExecutorTypeAuthentication,
		defaultInputs, prerequisites, mock.Anything).Return(suite.mockBaseExec)

	suite.executor = newOTPExecutor(suite.mockFlowFactory, suite.mockOTPService,
		suite.mockAuthnProvider, suite.mockEntityProvider)
	suite.executor.Executor = suite.mockBaseExec
}

// Generate mode tests

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_UserInputRequired_NoSearchAttrs() {
	ctx := &providers.NodeContext{
		ExecutionID:  "exec-1",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Len(suite.T(), resp.Inputs, 1)
	assert.Equal(suite.T(), common.AttributeMobileNumber, resp.Inputs[0].Identifier)
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_UserInputRequired_WhenNodeInputsEmpty() {
	ctx := &providers.NodeContext{
		ExecutionID:  "exec-1b",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs:   nil,
		UserInputs:   map[string]string{},
		RuntimeData:  map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Len(suite.T(), resp.Inputs, 1)
	assert.Equal(suite.T(), common.AttributeMobileNumber, resp.Inputs[0].Identifier)
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_Success_UserIdentifiedAndOTPGenerated() {
	userID := testOTPUserID
	suite.mockEntityProvider.On("IdentifyEntity", mock.MatchedBy(func(attrs map[string]interface{}) bool {
		_, hasMobile := attrs[common.AttributeMobileNumber]
		return hasMobile
	})).Return(&userID, nil)

	suite.mockOTPService.On("GenerateOTP", mock.Anything, userID, authnprovidercm.UserAttributeUserID, mock.Anything).
		Return("session-tok-1", "654321", int64(300), (*tidcommon.ServiceError)(nil))

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-2",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
			{Ref: "otp_input", Identifier: userInputOTP, Type: providers.InputTypeOTP, Required: true},
		},
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "session-tok-1", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
	fwdData, ok := resp.ForwardedData[common.ForwardedDataKeyTemplateData].(map[string]interface{})
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "654321", fwdData[common.ForwardedDataKeyOTPCode])
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_MultipleInputs_IdentifiesUserByAllAttrs() {
	userID := testOTPUserID
	suite.mockEntityProvider.On("IdentifyEntity", mock.MatchedBy(func(attrs map[string]interface{}) bool {
		_, hasMobile := attrs[common.AttributeMobileNumber]
		_, hasEmail := attrs["email"]
		return hasMobile && hasEmail
	})).Return(&userID, nil)

	suite.mockOTPService.On("GenerateOTP", mock.Anything, userID, authnprovidercm.UserAttributeUserID, mock.Anything).
		Return("session-tok-2", "111222", int64(300), (*tidcommon.ServiceError)(nil))

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-3",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
			{Ref: "email_input", Identifier: "email", Type: providers.InputTypeEmail, Required: true},
			{Ref: "otp_input", Identifier: userInputOTP, Type: providers.InputTypeOTP, Required: true},
		},
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
			"email":                      "user@example.com",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "session-tok-2", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_UserNotFound_ReturnsFailure() {
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).
		Return((*string)(nil), &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeEntityNotFound})

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-4",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+9999999999",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.NotNil(suite.T(), resp.Error)
	assert.Equal(suite.T(), ErrUserNotFound.Code, resp.Error.Code)
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_Registration_UserNotFound_UsesMobileDestValue() {
	suite.mockEntityProvider.On("IdentifyEntity", mock.MatchedBy(func(attrs map[string]interface{}) bool {
		_, hasMobile := attrs[common.AttributeMobileNumber]
		return hasMobile
	})).Return((*string)(nil), &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeEntityNotFound})

	suite.mockOTPService.On("GenerateOTP", mock.Anything, "+1234567890", common.AttributeMobileNumber, mock.Anything).
		Return("session-reg-1", "777888", int64(300), (*tidcommon.ServiceError)(nil))

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-reg-1",
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
			{Ref: "otp_input", Identifier: userInputOTP, Type: providers.InputTypeOTP, Required: true},
		},
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "session-reg-1", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
	fwdData, ok := resp.ForwardedData[common.ForwardedDataKeyTemplateData].(map[string]interface{})
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "777888", fwdData[common.ForwardedDataKeyOTPCode])
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_Registration_UserNotFound_NoPhoneValue_ReturnsInputRequired() {
	suite.mockEntityProvider.On("IdentifyEntity", mock.MatchedBy(func(attrs map[string]interface{}) bool {
		_, hasUsername := attrs["username"]
		return hasUsername
	})).Return((*string)(nil), &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeEntityNotFound})

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-reg-2",
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "username_input", Identifier: "username", Type: providers.InputTypeText, Required: true},
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs: map[string]string{
			"username": "john",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_MaxAttemptsReached_ReturnsFailure() {
	userID := testOTPUserID
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(&userID, nil)

	suite.mockOTPService.On("GenerateOTP", mock.Anything, userID, authnprovidercm.UserAttributeUserID, "prev-session-tok").
		Return("", "", int64(0), &otp.ErrorMaxOTPAttemptsExceeded)

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-5",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyOTPSessionToken: "prev-session-tok",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.NotNil(suite.T(), resp.Error)
	assert.Equal(suite.T(), ErrMaxOTPAttemptsReached.Code, resp.Error.Code)
}

// Verify mode tests

func (suite *OTPExecutorTestSuite) TestExecuteVerify_OTPInputRequired_WhenNoOTPProvided() {
	ctx := &providers.NodeContext{
		ExecutionID:  "exec-6",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs:   map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyOTPSessionToken: "session-tok-1",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.NotNil(suite.T(), resp.Error)
	assert.Equal(suite.T(), ErrInvalidOTP.Code, resp.Error.Code)
}

func (suite *OTPExecutorTestSuite) TestExecuteVerify_OTPInRuntimeDataButNotUserInputs_ReturnsInputRequired() {
	ctx := &providers.NodeContext{
		ExecutionID:  "exec-otp-collision",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs:   map[string]string{},
		RuntimeData: map[string]string{
			common.RuntimeKeyOTPSessionToken: "session-tok-1",
			common.ForwardedDataKeyOTPCode:   "123456",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.NotNil(suite.T(), resp.Error)
	assert.Equal(suite.T(), ErrInvalidOTP.Code, resp.Error.Code)
}

func (suite *OTPExecutorTestSuite) TestExecuteVerify_Success() {
	userID := testOTPUserID

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(
			providers.AuthUser{},
			providers.AuthenticatedClaims{userAttributeUserID: userID},
			(*tidcommon.ServiceError)(nil),
		)

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-7",
		FlowType:     providers.FlowTypeAuthentication,
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
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
	assert.Equal(suite.T(), userID, resp.RuntimeData[userAttributeUserID])
}

func (suite *OTPExecutorTestSuite) TestExecuteVerify_InvalidOTP_ReturnsUserInputRequired() {
	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(
			providers.AuthUser{},
			providers.AuthenticatedClaims(nil),
			&authnprovidermgr.ErrorAuthenticationFailed,
		)

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-8",
		FlowType:     providers.FlowTypeAuthentication,
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
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.NotNil(suite.T(), resp.Error)
	assert.Equal(suite.T(), ErrInvalidOTP.Code, resp.Error.Code)
}

func (suite *OTPExecutorTestSuite) TestExecuteVerify_MissingSessionToken_ReturnsError() {
	ctx := &providers.NodeContext{
		ExecutionID:  "exec-9",
		FlowType:     providers.FlowTypeAuthentication,
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
			execResp, _ := args.Get(1).(*providers.ExecutorResponse)
			if execResp != nil {
				execResp.Status = providers.ExecFailure
			}
		}).Return(false)

	exec := &otpExecutor{
		Executor:       freshMock,
		entityProvider: suite.mockEntityProvider,
		otpService:     suite.mockOTPService,
		authnProvider:  suite.mockAuthnProvider,
		logger:         suite.executor.logger,
	}

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-prereq",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputOTP: "654321",
		},
		RuntimeData: map[string]string{},
	}

	resp, err := exec.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	freshMock.AssertNotCalled(suite.T(), "HasRequiredInputs", mock.Anything, mock.Anything)
}

// Invalid mode

func (suite *OTPExecutorTestSuite) TestExecute_InvalidMode_ReturnsError() {
	ctx := &providers.NodeContext{
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
	ctx := &providers.NodeContext{
		NodeInputs: []providers.Input{
			{Identifier: userInputOTP, Type: providers.InputTypeOTP},
			{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone},
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
	ctx := &providers.NodeContext{
		NodeInputs: []providers.Input{
			{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone},
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

func (suite *OTPExecutorTestSuite) TestBuildSearchAttributes_FallsBackToMobileNumber_WhenNodeInputsEmpty() {
	ctx := &providers.NodeContext{
		NodeInputs: nil,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
		},
		RuntimeData:   map[string]string{},
		ForwardedData: map[string]interface{}{},
	}

	attrs := suite.executor.buildSearchAttributes(ctx)

	assert.Contains(suite.T(), attrs, common.AttributeMobileNumber)
	assert.Equal(suite.T(), "+1234567890", attrs[common.AttributeMobileNumber])
}

// getGenerateInputs tests

func (suite *OTPExecutorTestSuite) TestGetGenerateInputs_ReturnsNodeInputs_WhenNonEmpty() {
	ctx := &providers.NodeContext{
		NodeInputs: []providers.Input{
			{Identifier: "email", Type: providers.InputTypeEmail, Required: true},
		},
	}

	inputs := suite.executor.getGenerateInputs(ctx)

	assert.Equal(suite.T(), ctx.NodeInputs, inputs)
}

func (suite *OTPExecutorTestSuite) TestGetGenerateInputs_ReturnsMobileNumberFallback_WhenEmpty() {
	ctx := &providers.NodeContext{NodeInputs: nil}

	inputs := suite.executor.getGenerateInputs(ctx)

	assert.Len(suite.T(), inputs, 1)
	assert.Equal(suite.T(), common.AttributeMobileNumber, inputs[0].Identifier)
	assert.Equal(suite.T(), providers.InputTypePhone, inputs[0].Type)
	assert.True(suite.T(), inputs[0].Required)
}

// resolveUserID: authenticated user path

func (suite *OTPExecutorTestSuite) TestResolveUserID_AuthenticatedUser_ReturnsEntityID() {
	entityID := testOTPUserID
	authUser := providers.AuthUser{}
	authUser.SetStateFor("default", providers.AuthState{
		EntityReference: &providers.EntityReference{EntityID: entityID},
		Attributes:      &providers.AttributesResponse{},
	})

	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(authUser, &providers.EntityReference{EntityID: entityID}, (*tidcommon.ServiceError)(nil))

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-auth-user",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		AuthUser:     authUser,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	suite.mockOTPService.On("GenerateOTP", mock.Anything, entityID, authnprovidercm.UserAttributeUserID, mock.Anything).
		Return("session-auth-tok", "112233", int64(300), (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "session-auth-tok", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
}

func (suite *OTPExecutorTestSuite) TestResolveUserID_AuthenticatedUser_EntityRefError_FallsThrough() {
	authUser := providers.AuthUser{}
	authUser.SetStateFor("default", providers.AuthState{
		EntityReference: &providers.EntityReference{EntityID: "some-id"},
		Attributes:      &providers.AttributesResponse{},
	})

	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (*providers.EntityReference)(nil), &tidcommon.InternalServerError)

	suite.mockEntityProvider.On("IdentifyEntity", mock.MatchedBy(func(attrs map[string]interface{}) bool {
		_, hasMobile := attrs[common.AttributeMobileNumber]
		return hasMobile
	})).Return((*string)(nil), &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeEntityNotFound})

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-auth-err",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		AuthUser:     authUser,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs:  map[string]string{common.AttributeMobileNumber: "+1234567890"},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrUserNotFound.Code, resp.Error.Code)
}

// resolveUserID: IdentifyEntity returns non-nil pointer to empty string

func (suite *OTPExecutorTestSuite) TestResolveUserID_IdentifyEntityReturnsEmptyString_ReturnsFailure() {
	emptyID := ""
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(&emptyID, nil)

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-empty-id",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs:  map[string]string{common.AttributeMobileNumber: "+1234567890"},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrUserNotFound.Code, resp.Error.Code)
}

// buildSearchAttributes: ForwardedData fallback path

func (suite *OTPExecutorTestSuite) TestBuildSearchAttributes_FallsBackToForwardedData() {
	ctx := &providers.NodeContext{
		NodeInputs: []providers.Input{
			{Identifier: common.AttributeMobileNumber, Type: providers.InputTypePhone},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
		ForwardedData: map[string]interface{}{
			common.AttributeMobileNumber: "+5551234567",
		},
	}

	attrs := suite.executor.buildSearchAttributes(ctx)

	assert.Equal(suite.T(), "+5551234567", attrs[common.AttributeMobileNumber])
}

// executeGenerate: GenerateOTP service error path

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_GenerateOTPError_ReturnsError() {
	userID := testOTPUserID
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).Return(&userID, nil)

	svcErr := tidcommon.ServiceError{
		Code:             "OTP-ERR",
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "otp generation failed"},
	}
	suite.mockOTPService.On("GenerateOTP", mock.Anything, userID, authnprovidercm.UserAttributeUserID, mock.Anything).
		Return("", "", int64(0), &svcErr)

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-gen-err",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs:  map[string]string{common.AttributeMobileNumber: "+1234567890"},
		RuntimeData: map[string]string{},
	}

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
}

// resolveOTPDestination: RuntimeData and ForwardedData paths

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_Registration_DestinationFromRuntimeData() {
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).
		Return((*string)(nil), &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeEntityNotFound})

	suite.mockOTPService.On("GenerateOTP", mock.Anything, "+9876543210", common.AttributeMobileNumber, mock.Anything).
		Return("session-rt", "445566", int64(300), (*tidcommon.ServiceError)(nil))

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-reg-rt",
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs: map[string]string{},
		RuntimeData: map[string]string{
			common.AttributeMobileNumber: "+9876543210",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "session-rt", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
}

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_Registration_DestinationFromForwardedData() {
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).
		Return((*string)(nil), &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeEntityNotFound})

	suite.mockOTPService.On("GenerateOTP", mock.Anything, "+1112223333", common.AttributeMobileNumber, mock.Anything).
		Return("session-fwd", "334455", int64(300), (*tidcommon.ServiceError)(nil))

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-reg-fwd",
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
		ForwardedData: map[string]interface{}{
			common.AttributeMobileNumber: "+1112223333",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "session-fwd", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
}

// resolveUserID: userID already in RuntimeData (early return path)

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_UserIDAlreadyInRuntimeData() {
	suite.mockOTPService.On("GenerateOTP", mock.Anything, testOTPUserID, authnprovidercm.UserAttributeUserID, mock.Anything).
		Return("session-cached", "221100", int64(300), (*tidcommon.ServiceError)(nil))

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-cached-uid",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs: map[string]string{},
		RuntimeData: map[string]string{
			"userID": testOTPUserID,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "session-cached", resp.RuntimeData[common.RuntimeKeyOTPSessionToken])
}

// resolveUserID: non-EntityNotFound provider error propagates up

func (suite *OTPExecutorTestSuite) TestExecuteGenerate_IdentifyEntitySystemError_ReturnsError() {
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything).
		Return((*string)(nil), &entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeSystemError})

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-sys-err",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []providers.Input{
			{Ref: "mobile_input", Identifier: common.AttributeMobileNumber,
				Type: providers.InputTypePhone, Required: true},
		},
		UserInputs:  map[string]string{common.AttributeMobileNumber: "+1234567890"},
		RuntimeData: map[string]string{},
	}

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
}

// getAuthenticatedUser: AuthenticateUser returns an unexpected (non-auth-failed) error

func (suite *OTPExecutorTestSuite) TestExecuteVerify_AuthenticateUserUnexpectedError_ReturnsError() {
	unexpectedErr := tidcommon.ServiceError{
		Code:             "AUTHN-9999",
		ErrorDescription: tidcommon.I18nMessage{DefaultValue: "unexpected authn error"},
	}
	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(
			providers.AuthUser{},
			providers.AuthenticatedClaims(nil),
			&unexpectedErr,
		)

	ctx := &providers.NodeContext{
		ExecutionID:  "exec-verify-unexpected",
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputOTP: "654321",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyOTPSessionToken: "session-tok-1",
		},
	}

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
}

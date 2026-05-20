/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/tests/mocks/authn/otpmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type SMSAuthExecutorTestSuite struct {
	suite.Suite
	mockOTPService     *otpmock.OTPAuthnServiceInterfaceMock
	mockAuthnProvider  *managermock.AuthnProviderManagerInterfaceMock
	mockFlowFactory    *coremock.FlowFactoryInterfaceMock
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	executor           *smsOTPAuthExecutor
}

func TestSMSAuthExecutorSuite(t *testing.T) {
	suite.Run(t, new(SMSAuthExecutorTestSuite))
}

func (suite *SMSAuthExecutorTestSuite) SetupTest() {
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
	// Mock identifying executor
	identifyingMock := createMockIdentifyingExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, common.ExecutorTypeUtility,
		mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	// Mock base executor
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameSMSAuth).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return(defaultInputs).Maybe()
	mockExec.On("GetRequiredInputs", mock.Anything).Return(defaultInputs).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{}).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything).Return(true).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *core.NodeContext, execResp *common.ExecutorResponse) bool {
			otp, exists := ctx.UserInputs[userInputOTP]
			if !exists || otp == "" {
				execResp.Inputs = defaultInputs
				execResp.Status = common.ExecUserInputRequired
				return false
			}
			return true
		}).Maybe()

	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameSMSAuth, common.ExecutorTypeAuthentication,
		defaultInputs, []common.Input(nil)).Return(mockExec)

	suite.executor = newSMSOTPAuthExecutor(suite.mockFlowFactory,
		suite.mockOTPService, suite.mockAuthnProvider, suite.mockEntityProvider)
	// Inject the mock base executor
	suite.executor.ExecutorInterface = mockExec
}

func (suite *SMSAuthExecutorTestSuite) TestValidatePrerequisites_RegistrationFlow_PromptsMobileNumber() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}
	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	result := suite.executor.ValidatePrerequisites(ctx, execResp)

	// Should return false (prerequisites not met)
	assert.False(suite.T(), result)

	// Should set status to ExecUserInputRequired
	assert.Equal(suite.T(), common.ExecUserInputRequired, execResp.Status)

	// Should return mobile number input
	assert.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), common.AttributeMobileNumber, execResp.Inputs[0].Identifier)
	assert.Equal(suite.T(), common.InputTypePhone, execResp.Inputs[0].Type)
	assert.Equal(suite.T(), "mobile_number_input", execResp.Inputs[0].Ref)
	assert.True(suite.T(), execResp.Inputs[0].Required)
}

func (suite *SMSAuthExecutorTestSuite) TestValidatePrerequisites_RegistrationFlow_CustomPhoneAttr() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow-123",
		FlowType:    common.FlowTypeRegistration,
		NodeInputs: []common.Input{
			{Ref: "phone_input", Identifier: "phoneNumber", Type: common.InputTypePhone, Required: true},
		},
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}
	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	result := suite.executor.ValidatePrerequisites(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, execResp.Status)
	assert.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), "phoneNumber", execResp.Inputs[0].Identifier)
	assert.Equal(suite.T(), common.InputTypePhone, execResp.Inputs[0].Type)
	assert.Equal(suite.T(), "phone_input", execResp.Inputs[0].Ref)
}

func (suite *SMSAuthExecutorTestSuite) TestValidatePrerequisites_RegistrationFlow_PrerequisitesMet() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
		},
		RuntimeData: make(map[string]string),
	}
	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	result := suite.executor.ValidatePrerequisites(ctx, execResp)

	// Should return true (prerequisites met)
	assert.True(suite.T(), result)

	// Status should NOT be set to ExecUserInputRequired
	assert.NotEqual(suite.T(), common.ExecUserInputRequired, execResp.Status)
}

func (suite *SMSAuthExecutorTestSuite) TestValidatePrerequisites_AuthenticationFlow_DoesNotPromptMobile() {
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetPrerequisites").Return([]common.Input{mobileNumberInput}).Maybe()
	mockExec.On("GetUserIDFromContext", mock.Anything).Return("").Maybe()
	suite.executor.ExecutorInterface = mockExec

	ctx := &core.NodeContext{
		ExecutionID: "test-flow-123",
		FlowType:    common.FlowTypeAuthentication, // Authentication flow, NOT registration
		UserInputs:  make(map[string]string),
		RuntimeData: make(map[string]string),
	}
	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	result := suite.executor.ValidatePrerequisites(ctx, execResp)

	assert.False(suite.T(), result, "Should return false when prerequisites not met")
	assert.NotEqual(suite.T(), common.ExecUserInputRequired, execResp.Status,
		"Authentication flows should not prompt for mobile number directly")
}

// TestGetAuthenticatedUser_MFA_AddsMobileNumberToAttributes verifies that when user is already authenticated
func (suite *SMSAuthExecutorTestSuite) TestGetAuthenticatedUser_MFA_AddsMobileNumberToAttributes() {
	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			UserID: "user-123", UserType: "INTERNAL", OUID: "ou-123",
		}, nil)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userInputOTP: "123456",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeySMSOTPMobileNumber: "+1234567890",
			"otpSessionToken":                   "test-session-token",
		},
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			OUID:            "ou-123",
			UserType:        "INTERNAL",
			Attributes: map[string]interface{}{
				"email": "test@example.com",
			},
		},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	result, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.True(suite.T(), result.IsAuthenticated)
	assert.Equal(suite.T(), "user-123", result.UserID)
	// Verify mobile number was added to attributes
	assert.Equal(suite.T(), "+1234567890", result.Attributes[common.AttributeMobileNumber])
	assert.Equal(suite.T(), "test@example.com", result.Attributes["email"]) // Existing attributes preserved
}

// TestGetAuthenticatedUser_FetchFromStore_PreservesExistingMobileNumber verifies that when fetching user
// from store, if mobile number already exists in stored attributes, it is preserved.
func (suite *SMSAuthExecutorTestSuite) TestGetAuthenticatedUser_FetchFromStore_PreservesExistingMobileNumber() {
	attrs := map[string]interface{}{
		"email":                      "test@example.com",
		common.AttributeMobileNumber: "+9876543210", // Mobile already in stored attributes
	}
	attrsJSON, _ := json.Marshal(attrs)

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			UserID: "user-123", UserType: "INTERNAL", OUID: "ou-123",
		}, nil)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userInputOTP: "123456",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeySMSOTPMobileNumber: "+1234567890",
			"otpSessionToken":                   "test-session-token",
		},
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: false,
		},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	userFromStore := &entityprovider.Entity{
		ID:         "user-123",
		OUID:       "ou-123",
		Type:       "INTERNAL",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(userFromStore, nil)

	result, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.True(suite.T(), result.IsAuthenticated)
	// Verify stored mobile number is preserved
	assert.Equal(suite.T(), "+9876543210", result.Attributes[common.AttributeMobileNumber])
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

// TestGetUserMobileNumber_NotFoundInAttributesOrContext verifies that when mobile number
// is not found in user attributes or context, the function sets failure status.
func (suite *SMSAuthExecutorTestSuite) TestGetUserMobileNumber_NotFoundInAttributesOrContext() {
	// User attributes without mobile number
	attrs := map[string]interface{}{
		"email": "test@example.com",
		// No mobile number
	}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		// No mobile number in UserInputs or RuntimeData
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	userFromStore := &entityprovider.Entity{
		ID:         "user-123",
		OUID:       "ou-123",
		Type:       "INTERNAL",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(userFromStore, nil)

	mobileNumber, err := suite.executor.getUserMobileNumber("user-123", ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), mobileNumber)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "Mobile number not found in user attributes or context", execResp.FailureReason)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

// TestValidatePrerequisites_RegistrationFlow_EmptyPhoneInUserInputs verifies that an empty
// phone value in UserInputs is not treated as a met prerequisite.
func (suite *SMSAuthExecutorTestSuite) TestValidatePrerequisites_RegistrationFlow_EmptyPhoneInUserInputs() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "",
		},
		RuntimeData: make(map[string]string),
	}
	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	result := suite.executor.ValidatePrerequisites(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, execResp.Status)
}

// TestValidatePrerequisites_RegistrationFlow_EmptyPhoneInRuntimeData verifies that an empty
// phone value in RuntimeData is not treated as a met prerequisite.
func (suite *SMSAuthExecutorTestSuite) TestValidatePrerequisites_RegistrationFlow_EmptyPhoneInRuntimeData() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  make(map[string]string),
		RuntimeData: map[string]string{
			common.AttributeMobileNumber: "",
		},
	}
	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	result := suite.executor.ValidatePrerequisites(ctx, execResp)

	assert.False(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, execResp.Status)
}

// TestGetUserMobileNumber_NonStringAttributeValue verifies that a non-string phone attribute
// value in user store does not panic and results in a failure response.
func (suite *SMSAuthExecutorTestSuite) TestGetUserMobileNumber_NonStringAttributeValue() {
	attrs := map[string]interface{}{
		common.AttributeMobileNumber: 1234567890, // integer, not string
	}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	userFromStore := &entityprovider.Entity{
		ID:         "user-123",
		OUID:       "ou-123",
		Type:       "INTERNAL",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(userFromStore, nil)

	mobileNumber, err := suite.executor.getUserMobileNumber("user-123", ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Empty(suite.T(), mobileNumber)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

// TestGetAuthenticatedUser_MFA_NilAttributes verifies that when the authenticated user
// has nil Attributes map, it is initialized before returning.
func (suite *SMSAuthExecutorTestSuite) TestGetAuthenticatedUser_MFA_NilAttributes() {
	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			UserID: "user-123", UserType: "INTERNAL", OUID: "ou-123",
		}, nil)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userInputOTP: "123456",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeySMSOTPMobileNumber: "+1234567890",
			"otpSessionToken":                   "test-session-token",
		},
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			OUID:            "ou-123",
			UserType:        "INTERNAL",
			Attributes:      nil, // Explicitly nil
		},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	result, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.True(suite.T(), result.IsAuthenticated)
	assert.NotNil(suite.T(), result.Attributes) // Attributes should be initialized
	assert.Equal(suite.T(), "+1234567890", result.Attributes[common.AttributeMobileNumber])
}

// TestInitiateOTP_RegistrationFlow_UserAlreadyExists_PromptsForDifferentNumber verifies that when
// a user with the provided mobile number already exists in a registration flow, the executor
// returns ExecUserInputRequired with the phone input so the user can provide a different number.
func (suite *SMSAuthExecutorTestSuite) TestInitiateOTP_RegistrationFlow_UserAlreadyExists_PromptsForDifferentNumber() {
	existingUserID := testExistingUser123ID
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		common.AttributeMobileNumber: "+1234567890",
	}).Return(&existingUserID, nil)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
		},
		RuntimeData:       make(map[string]string),
		AuthenticatedUser: authncm.AuthenticatedUser{IsAuthenticated: false},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	err := suite.executor.InitiateOTP(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, execResp.Status)
	assert.Equal(suite.T(), "User already exists with the provided mobile number.", execResp.FailureReason)
	assert.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), common.AttributeMobileNumber, execResp.Inputs[0].Identifier)
	assert.Equal(suite.T(), common.InputTypePhone, execResp.Inputs[0].Type)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

// TestInitiateOTP_RegistrationFlow_AmbiguousUser_ReturnsError verifies that when user
// identification returns an ambiguous result during registration, an error is returned.
func (suite *SMSAuthExecutorTestSuite) TestInitiateOTP_RegistrationFlow_AmbiguousUser_ReturnsError() {
	ambiguousErr := entityprovider.NewEntityProviderError(
		entityprovider.ErrorCodeAmbiguousEntity, "Ambiguous entity", "multiple users found",
	)
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		common.AttributeMobileNumber: "+1234567890",
	}).Return(nil, ambiguousErr)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			common.AttributeMobileNumber: "+1234567890",
		},
		RuntimeData:       make(map[string]string),
		AuthenticatedUser: authncm.AuthenticatedUser{IsAuthenticated: false},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	err := suite.executor.InitiateOTP(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to identify user during registration flow")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

// TestGetAuthenticatedUser_EmptyOTP_ReturnsUserInputRequired verifies that when the user
// submits an empty OTP, the executor returns ExecUserInputRequired with inputs populated
// so the client can prompt the user to re-enter the OTP.
func (suite *SMSAuthExecutorTestSuite) TestGetAuthenticatedUser_EmptyOTP_ReturnsUserInputRequired() {
	defaultInputs := []common.Input{
		{Ref: "otp_input", Identifier: userInputOTP, Type: common.InputTypeOTP, Required: true},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("GetRequiredInputs", mock.Anything).Return(defaultInputs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userInputOTP: "", // empty OTP
		},
		RuntimeData: map[string]string{
			common.RuntimeKeySMSOTPMobileNumber: "+1234567890",
			"otpSessionToken":                   "test-session-token",
		},
		AuthenticatedUser: authncm.AuthenticatedUser{IsAuthenticated: false},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	result, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, execResp.Status,
		"Empty OTP should return ExecUserInputRequired so the user can retry")
	assert.Equal(suite.T(), failureReasonInvalidOTP, execResp.FailureReason)
	assert.NotEmpty(suite.T(), execResp.Inputs, "Inputs must be populated for retry")
	assert.Equal(suite.T(), userInputOTP, execResp.Inputs[0].Identifier)
}

// TestGetAuthenticatedUser_FetchFromStore_NilAttrsAfterUnmarshal verifies that when
// user attributes unmarshal to nil, the result is returned without error.
func (suite *SMSAuthExecutorTestSuite) TestGetAuthenticatedUser_FetchFromStore_NilAttrsAfterUnmarshal() {
	// JSON null unmarshals to nil map
	attrsJSON := []byte("null")

	suite.mockAuthnProvider.On("AuthenticateUser",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			UserID: "user-123", UserType: "INTERNAL", OUID: "ou-123",
		}, nil)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userInputOTP: "123456",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeySMSOTPMobileNumber: "+1234567890",
			"otpSessionToken":                   "test-session-token",
		},
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: false,
		},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	userFromStore := &entityprovider.Entity{
		ID:         "user-123",
		OUID:       "ou-123",
		Type:       "INTERNAL",
		Attributes: attrsJSON, // null JSON
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(userFromStore, nil)

	result, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.True(suite.T(), result.IsAuthenticated)
	assert.Equal(suite.T(), "user-123", result.UserID)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

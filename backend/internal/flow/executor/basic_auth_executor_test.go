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
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"

	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type BasicAuthExecutorTestSuite struct {
	suite.Suite
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	mockAuthnProvider  *managermock.AuthnProviderManagerInterfaceMock
	mockFlowFactory    *coremock.FlowFactoryInterfaceMock
	executor           *basicAuthExecutor
}

func TestBasicAuthExecutorSuite(t *testing.T) {
	suite.Run(t, new(BasicAuthExecutorTestSuite))
}

func (suite *BasicAuthExecutorTestSuite) SetupTest() {
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())

	defaultInputs := []common.Input{
		{Identifier: userAttributeUsername, Type: common.InputTypeText, Required: true},
		{Identifier: userAttributePassword, Type: common.InputTypePassword, Required: true},
	}

	// Mock the embedded identifying executor first
	identifyingMock := createMockIdentifyingExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, common.ExecutorTypeUtility,
		mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	mockExec := createMockBasicAuthExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameBasicAuth, common.ExecutorTypeAuthentication,
		defaultInputs, []common.Input{}).Return(mockExec)

	suite.executor = newBasicAuthExecutor(suite.mockFlowFactory, suite.mockEntityProvider, suite.mockAuthnProvider)
}

func createMockIdentifyingExecutor(t *testing.T) core.ExecutorInterface {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(ExecutorNameIdentifying).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeUtility).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{}).Maybe()
	return mockExec
}

func createMockExecutorWithCustomInputs(t *testing.T, name string,
	inputs []common.Input) core.ExecutorInterface {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(name).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return(inputs).Maybe()
	mockExec.On("GetRequiredInputs", mock.Anything).Return(inputs).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{}).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *core.NodeContext, execResp *common.ExecutorResponse) bool {
			for _, input := range inputs {
				if input.Required {
					value, exists := ctx.UserInputs[input.Identifier]
					if !exists || value == "" {
						execResp.Inputs = inputs
						execResp.Status = common.ExecUserInputRequired
						return false
					}
				}
			}
			return true
		}).Maybe()
	return mockExec
}

func createMockBasicAuthExecutor(t *testing.T) core.ExecutorInterface {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(ExecutorNameBasicAuth).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{
		{Identifier: userAttributeUsername, Type: common.InputTypeText, Required: true},
		{Identifier: userAttributePassword, Type: common.InputTypePassword, Required: true},
	}).Maybe()
	mockExec.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: userAttributeUsername, Type: common.InputTypeText, Required: true},
		{Identifier: userAttributePassword, Type: common.InputTypePassword, Required: true},
	}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{}).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *core.NodeContext, execResp *common.ExecutorResponse) bool {
			username, hasUsername := ctx.UserInputs[userAttributeUsername]
			password, hasPassword := ctx.UserInputs[userAttributePassword]
			if !hasUsername || username == "" || !hasPassword || password == "" {
				execResp.Inputs = []common.Input{
					{Identifier: userAttributeUsername, Type: common.InputTypeText, Required: true},
					{Identifier: userAttributePassword, Type: common.InputTypePassword, Required: true},
				}
				execResp.Status = common.ExecUserInputRequired
				return false
			}
			return true
		}).Maybe()
	return mockExec
}

func (suite *BasicAuthExecutorTestSuite) TestNewBasicAuthExecutor() {
	assert.NotNil(suite.T(), suite.executor)
	assert.NotNil(suite.T(), suite.executor.authnProvider)
	assert.NotNil(suite.T(), suite.executor.entityProvider)
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_Success_AuthenticationFlow() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
		RuntimeData: make(map[string]string),
	}

	authenticateResult := &authnprovidermgr.AuthnBasicResult{
		UserID:   testUserID,
		UserType: "person",
		OUID:     "ou-123",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{}, authenticateResult, nil)

	suite.mockEntityProvider.On("GetEntity", testUserID).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented, "", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), testUserID, resp.AuthenticatedUser.UserID)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_Success_WithEmailAttribute() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"email":    "test@example.com",
			"password": "password123",
		},
		RuntimeData: make(map[string]string),
	}

	// Override GetRequiredInputs to return email and password as required fields
	originalInputs := []common.Input{
		{Identifier: "email", Type: common.InputTypeText, Required: true},
		{Identifier: "password", Type: common.InputTypePassword, Required: true},
	}
	suite.executor.ExecutorInterface = createMockExecutorWithCustomInputs(
		suite.T(), ExecutorNameBasicAuth, originalInputs)

	authenticatedUser := &authnprovidermgr.AuthnBasicResult{
		UserID:   testUserID,
		UserType: "person",
		OUID:     "ou-123",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		"email": "test@example.com",
	}, map[string]interface{}{
		"password": "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{}, authenticatedUser, nil)

	suite.mockEntityProvider.On("GetEntity", testUserID).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented, "", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), testUserID, resp.AuthenticatedUser.UserID)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_Success_RegistrationFlow() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			userAttributeUsername: "newuser",
			userAttributePassword: "password123",
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeUsername: "newuser",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.False(suite.T(), resp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), "newuser", resp.AuthenticatedUser.Attributes[userAttributeUsername])
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_Success_WithMultipleAttributes() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"email":    "test@example.com",
			"phone":    "+1234567890",
			"password": "password123",
		},
		RuntimeData: make(map[string]string),
	}

	// Override GetRequiredInputs to return email, phone, and password as required fields
	customInputs := []common.Input{
		{Identifier: "email", Type: common.InputTypeText, Required: true},
		{Identifier: "phone", Type: common.InputTypeText, Required: true},
		{Identifier: "password", Type: common.InputTypePassword, Required: true},
	}
	suite.executor.ExecutorInterface = createMockExecutorWithCustomInputs(
		suite.T(), ExecutorNameBasicAuth, customInputs)

	authenticatedUser := &authnprovidermgr.AuthnBasicResult{
		UserID:   testUserID,
		UserType: "person",
		OUID:     "ou-123",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		"email": "test@example.com",
		"phone": "+1234567890",
	}, map[string]interface{}{
		"password": "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{}, authenticatedUser, nil)

	suite.mockEntityProvider.On("GetEntity", testUserID).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented, "", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), testUserID, resp.AuthenticatedUser.UserID)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_UserInputRequired() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		RuntimeData: make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.NotEmpty(suite.T(), resp.Inputs)
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_AuthenticationFailed() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "wrongpassword",
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "wrongpassword",
	}, mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{},
		(*authnprovidermgr.AuthnBasicResult)(nil), &serviceerror.ServiceError{
			Type: serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.invalid_credentials", DefaultValue: "Invalid credentials",
			},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "Failed to authenticate user")
	assert.NotEmpty(suite.T(), resp.Inputs, "Inputs should be re-populated for retry")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_UserNotFound_AuthenticationFlow() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "nonexistent",
			userAttributePassword: "password123",
		},
		RuntimeData: make(map[string]string),
	}

	// Authenticate internally calls IdentifyUser and returns user not found error
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "nonexistent",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{},
		(*authnprovidermgr.AuthnBasicResult)(nil), &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{Key: "error.test.user_not_found", DefaultValue: "User not found"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "Failed to authenticate user",
		"Failure reason should contain authentication failure message")
	assert.NotEmpty(suite.T(), resp.Inputs, "Inputs should be re-populated for retry")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_UserAlreadyExists_RegistrationFlow() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			userAttributeUsername: "existinguser",
			userAttributePassword: "password123",
		},
		RuntimeData: make(map[string]string),
	}

	userID := testUserID
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeUsername: "existinguser",
	}).Return(&userID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "User already exists")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_ServiceError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
		RuntimeData: make(map[string]string),
	}

	// Authenticate returns a server error (e.g., database error)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{},
		(*authnprovidermgr.AuthnBasicResult)(nil), &serviceerror.ServiceError{
			Type:  serviceerror.ServerErrorType,
			Error: i18ncore.I18nMessage{Key: "error.test.database_error", DefaultValue: "database error"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_AuthenticationServiceError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, (*authnprovidermgr.AuthnBasicResult)(nil), &serviceerror.ServiceError{
			Type:  serviceerror.ServerErrorType,
			Error: i18ncore.I18nMessage{Key: "error.test.internal_server_error", DefaultValue: "internal server error"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "Failed to authenticate user")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestGetAuthenticatedUser_SuccessfulAuthentication() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	authenticatedUser := &authnprovidermgr.AuthnBasicResult{
		UserID:   testUserID,
		UserType: "person",
		OUID:     "ou-123",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{}, authenticatedUser, nil)

	suite.mockEntityProvider.On("GetEntity", testUserID).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented, "", ""))

	result, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.True(suite.T(), result.IsAuthenticated)
	assert.Equal(suite.T(), testUserID, result.UserID)
	assert.Equal(suite.T(), "ou-123", result.OUID)
	assert.Equal(suite.T(), "person", result.UserType)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestGetAuthenticatedUser_Success_WithFetchedAttributes() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	authenticateResult := &authnprovidermgr.AuthnBasicResult{
		UserID:   testUserID,
		UserType: "person",
		OUID:     "ou-123",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{}, authenticateResult, nil)

	// Mock UserProvider response
	attrs := map[string]interface{}{"username": "testuser", "email": "fetched@example.com", "role": "admin"}
	attrsJSON, _ := json.Marshal(attrs)
	user := &entityprovider.Entity{
		ID:         testUserID,
		Attributes: attrsJSON,
	}
	suite.mockEntityProvider.On("GetEntity", testUserID).Return(user, nil)

	result, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.True(suite.T(), result.IsAuthenticated)
	assert.Equal(suite.T(), testUserID, result.UserID)
	assert.Equal(suite.T(), "testuser", result.Attributes["username"])
	assert.Equal(suite.T(), "fetched@example.com", result.Attributes["email"])
	assert.Equal(suite.T(), "admin", result.Attributes["role"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestGetAuthenticatedUser_AuthenticationFlow_NoRedundantIdentifyUser() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	authenticatedUser := &authnprovidermgr.AuthnBasicResult{
		UserID:   testUserID,
		UserType: "person",
		OUID:     "ou-123",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{}, authenticatedUser, nil)

	suite.mockEntityProvider.On("GetEntity", testUserID).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented, "", ""))

	result, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.True(suite.T(), result.IsAuthenticated)
	assert.Equal(suite.T(), testUserID, result.UserID)
	// Verify Authenticate was called (which handles IdentifyUser + VerifyUser internally)
	// This test verifies the optimization: no explicit IdentifyUser call for auth flows
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *BasicAuthExecutorTestSuite) TestGetAuthenticatedUser_RegistrationFlow_CallsIdentifyUser() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			userAttributeUsername: "newuser",
			userAttributePassword: "password123",
		},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	// For registration flows, IdentifyUser should be called to check if user exists
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeUsername: "newuser",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	result, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.False(suite.T(), result.IsAuthenticated)
	assert.Equal(suite.T(), "newuser", result.Attributes[userAttributeUsername])
	// Verify IdentifyUser was called for registration flow
	suite.mockEntityProvider.AssertExpectations(suite.T())
	// Verify Authenticate was NOT called for registration flow
	suite.mockAuthnProvider.AssertNotCalled(suite.T(), "AuthenticateUser")
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_RetryableAuthenticationErrors() {
	tests := []struct {
		name           string
		username       string
		password       string
		errorCode      string
		expectedReason string
		message        string
	}{
		{
			name:           "Invalid credentials",
			username:       "testuser",
			password:       "wrongpassword",
			errorCode:      authnprovidermgr.ErrorAuthenticationFailed.Code,
			expectedReason: failureReasonInvalidCredentials,
			message:        "Should return specific failure reason for invalid credentials",
		},
		{
			name:           "User not found",
			username:       "nonexistent",
			password:       "password123",
			errorCode:      authnprovidermgr.ErrorUserNotFound.Code,
			expectedReason: failureReasonUserNotFound,
			message:        "Should return specific failure reason for user not found",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			suite.mockAuthnProvider.ExpectedCalls = nil
			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    common.FlowTypeAuthentication,
				UserInputs: map[string]string{
					userAttributeUsername: tt.username,
					userAttributePassword: tt.password,
				},
				RuntimeData: make(map[string]string),
			}

			suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
				userAttributeUsername: tt.username,
			}, map[string]interface{}{
				userAttributePassword: tt.password,
			}, mock.Anything, mock.Anything, mock.Anything).Return(
				authnprovidermgr.AuthUser{}, (*authnprovidermgr.AuthnBasicResult)(nil), &serviceerror.ServiceError{
					Type: serviceerror.ClientErrorType,
					Code: tt.errorCode,
				})

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, common.ExecUserInputRequired, resp.Status)
			assert.Equal(t, tt.expectedReason, resp.FailureReason, tt.message)
			assert.NotEmpty(t, resp.Inputs, "Inputs should be re-populated for retry")
			assert.Len(t, resp.Inputs, 2, "Should include both username and password inputs")
			suite.mockAuthnProvider.AssertExpectations(t)
		})
	}
}

func (suite *BasicAuthExecutorTestSuite) TestGetAuthenticatedUser_ClientError_ReturnsInputsForRetry() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
	}

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(
		authnprovidermgr.AuthUser{}, (*authnprovidermgr.AuthnBasicResult)(nil), &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			Code:             authnprovidermgr.ErrorAuthenticationFailed.Code,
			ErrorDescription: i18ncore.I18nMessage{Key: "error.test.wrong_password", DefaultValue: "wrong password"},
		})

	result, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, execResp.Status,
		"Should return ExecUserInputRequired for invalid credentials")
	assert.Equal(suite.T(), failureReasonInvalidCredentials, execResp.FailureReason)
	assert.NotEmpty(suite.T(), execResp.Inputs, "Inputs should be re-populated for retry")
	assert.Len(suite.T(), execResp.Inputs, 2, "Should include both username and password inputs")
}

func (suite *BasicAuthExecutorTestSuite) TestBuildAuthnMetadata_WithAllFields() {
	ctx := &core.NodeContext{
		Application: appmodel.Application{
			Metadata: map[string]interface{}{
				"tenant_id": "tenant-123",
				"region":    "us-west",
			},
			InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						ClientID: "oauth-client-1",
					},
				},
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						ClientID: "oauth-client-2",
					},
				},
			},
		},
	}

	metadata := suite.executor.buildAuthnMetadata(ctx)

	assert.NotNil(suite.T(), metadata)
	assert.NotNil(suite.T(), metadata.AppMetadata)
	assert.Equal(suite.T(), "tenant-123", metadata.AppMetadata["tenant_id"])
	assert.Equal(suite.T(), "us-west", metadata.AppMetadata["region"])

	clientIDs, ok := metadata.AppMetadata["client_ids"].([]string)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), clientIDs, 2)
	assert.Contains(suite.T(), clientIDs, "oauth-client-1")
	assert.Contains(suite.T(), clientIDs, "oauth-client-2")
}

func (suite *BasicAuthExecutorTestSuite) TestBuildAuthnMetadata_WithNoMetadata() {
	ctx := &core.NodeContext{
		Application: appmodel.Application{},
	}

	metadata := suite.executor.buildAuthnMetadata(ctx)

	assert.NotNil(suite.T(), metadata)
	assert.NotNil(suite.T(), metadata.AppMetadata)
	assert.Empty(suite.T(), metadata.AppMetadata)
}

func (suite *BasicAuthExecutorTestSuite) TestBuildAuthnMetadata_WithOnlyAppMetadata() {
	ctx := &core.NodeContext{
		Application: appmodel.Application{
			Metadata: map[string]interface{}{
				"environment": "production",
				"version":     "1.0.0",
			},
		},
	}

	metadata := suite.executor.buildAuthnMetadata(ctx)

	assert.NotNil(suite.T(), metadata)
	assert.Equal(suite.T(), "production", metadata.AppMetadata["environment"])
	assert.Equal(suite.T(), "1.0.0", metadata.AppMetadata["version"])
	_, hasClientIDs := metadata.AppMetadata["client_ids"]
	assert.False(suite.T(), hasClientIDs)
}

func (suite *BasicAuthExecutorTestSuite) TestBuildAuthnMetadata_WithOnlyClientIDs() {
	ctx := &core.NodeContext{
		Application: appmodel.Application{
			InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						ClientID: "single-oauth-client",
					},
				},
			},
		},
	}

	metadata := suite.executor.buildAuthnMetadata(ctx)

	assert.NotNil(suite.T(), metadata)
	clientIDs, ok := metadata.AppMetadata["client_ids"].([]string)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), clientIDs, 1)
	assert.Equal(suite.T(), "single-oauth-client", clientIDs[0])
}

func (suite *BasicAuthExecutorTestSuite) TestBuildAuthnMetadata_WithNilOAuthConfig() {
	ctx := &core.NodeContext{
		Application: appmodel.Application{
			InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
				{
					Type:        inboundmodel.OAuthInboundAuthType,
					OAuthConfig: nil,
				},
			},
		},
	}

	metadata := suite.executor.buildAuthnMetadata(ctx)

	assert.NotNil(suite.T(), metadata)
	_, hasClientIDs := metadata.AppMetadata["client_ids"]
	assert.False(suite.T(), hasClientIDs)
}

func (suite *BasicAuthExecutorTestSuite) TestBuildAuthnMetadata_WithEmptyClientID() {
	ctx := &core.NodeContext{
		Application: appmodel.Application{
			InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						ClientID: "",
					},
				},
			},
		},
	}

	metadata := suite.executor.buildAuthnMetadata(ctx)

	assert.NotNil(suite.T(), metadata)
	_, hasClientIDs := metadata.AppMetadata["client_ids"]
	assert.False(suite.T(), hasClientIDs)
}

func (suite *BasicAuthExecutorTestSuite) TestBuildAuthnMetadata_WithMixedInboundConfigs() {
	ctx := &core.NodeContext{
		Application: appmodel.Application{
			InboundAuthConfig: []inboundmodel.InboundAuthConfigWithSecret{
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						ClientID: "valid-client",
					},
				},
				{
					Type:        inboundmodel.OAuthInboundAuthType,
					OAuthConfig: nil,
				},
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						ClientID: "",
					},
				},
				{
					Type: inboundmodel.OAuthInboundAuthType,
					OAuthConfig: &inboundmodel.OAuthConfigWithSecret{
						ClientID: "another-valid-client",
					},
				},
			},
		},
	}

	metadata := suite.executor.buildAuthnMetadata(ctx)

	assert.NotNil(suite.T(), metadata)
	clientIDs, ok := metadata.AppMetadata["client_ids"].([]string)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), clientIDs, 2)
	assert.Contains(suite.T(), clientIDs, "valid-client")
	assert.Contains(suite.T(), clientIDs, "another-valid-client")
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_PreResolvedUser_RequestsPassword() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			userAttributeUserID: "pre-resolved-user-123",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Len(suite.T(), resp.Inputs, 1)
	assert.Equal(suite.T(), userAttributePassword, resp.Inputs[0].Identifier)
}

func (suite *BasicAuthExecutorTestSuite) TestExecute_PreResolvedUser_WithPassword() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributePassword: "password123",
		},
		RuntimeData: map[string]string{
			userAttributeUserID: "pre-resolved-user-123",
		},
		Application: appmodel.Application{},
	}

	authenticateResult := &authnprovidermgr.AuthnBasicResult{
		UserID:   "pre-resolved-user-123",
		UserType: "person",
		OUID:     "ou-123",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything,
		map[string]interface{}{userAttributeUserID: "pre-resolved-user-123"},
		map[string]interface{}{userAttributePassword: "password123"},
		mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{}, authenticateResult, nil)

	suite.mockEntityProvider.On("GetEntity", "pre-resolved-user-123").Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeNotImplemented, "", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthenticatedUser.IsAuthenticated)
}

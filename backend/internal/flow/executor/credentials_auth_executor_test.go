// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type CredentialsAuthExecutorTestSuite struct {
	suite.Suite
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	mockAuthnProvider  *managermock.AuthnProviderManagerMock
	mockFlowFactory    *coremock.FlowFactoryInterfaceMock
	executor           *credentialsAuthExecutor
}

func TestCredentialsAuthExecutorSuite(t *testing.T) {
	suite.Run(t, new(CredentialsAuthExecutorTestSuite))
}

func (suite *CredentialsAuthExecutorTestSuite) SetupTest() {
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())

	defaultInputs := []providers.Input{
		{Identifier: userAttributeUsername, Type: providers.InputTypeText, Required: true},
		{Identifier: userAttributePassword, Type: providers.InputTypePassword, Required: true},
	}

	// Mock the embedded identifying executor first
	identifyingMock := createMockIdentifyingExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, providers.ExecutorTypeUtility,
		mock.Anything, mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	mockExec := createMockCredentialsAuthExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameCredentialsAuth, providers.ExecutorTypeAuthentication,
		defaultInputs, []providers.Input{}, mock.Anything).Return(mockExec)

	suite.executor = newCredentialsAuthExecutor(suite.mockFlowFactory, suite.mockEntityProvider,
		suite.mockAuthnProvider)
}

// newCredentialsAuthAuthenticatedUser creates an AuthUser that returns true for IsAuthenticated().
func newCredentialsAuthAuthenticatedUser() providers.AuthUser {
	var authUser providers.AuthUser
	_ = authUser.UnmarshalJSON([]byte(`{"default":{"entityReferenceToken":"tok","attributeToken":"tok"}}`))
	return authUser
}

func createMockIdentifyingExecutor(t *testing.T) providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(ExecutorNameIdentifying).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeUtility).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{}).Maybe()
	return mockExec
}

func createMockExecutorWithCustomInputs(t *testing.T, name string,
	inputs []providers.Input) providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(name).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return(inputs).Maybe()
	mockExec.On("GetRequiredInputs", mock.Anything).Return(inputs).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{}).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) bool {
			for _, input := range inputs {
				if input.Required {
					value, exists := ctx.UserInputs[input.Identifier]
					if !exists || value == "" {
						execResp.Inputs = inputs
						execResp.Status = providers.ExecUserInputRequired
						return false
					}
				}
			}
			return true
		}).Maybe()
	return mockExec
}

func createMockCredentialsAuthExecutor(t *testing.T) providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(ExecutorNameCredentialsAuth).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{
		{Identifier: userAttributeUsername, Type: providers.InputTypeText, Required: true},
		{Identifier: userAttributePassword, Type: providers.InputTypePassword, Required: true},
	}).Maybe()
	mockExec.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{Identifier: userAttributeUsername, Type: providers.InputTypeText, Required: true},
		{Identifier: userAttributePassword, Type: providers.InputTypePassword, Required: true},
	}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{}).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) bool {
			username, hasUsername := ctx.UserInputs[userAttributeUsername]
			password, hasPassword := ctx.UserInputs[userAttributePassword]
			if !hasUsername || username == "" || !hasPassword || password == "" {
				execResp.Inputs = []providers.Input{
					{Identifier: userAttributeUsername, Type: providers.InputTypeText, Required: true},
					{Identifier: userAttributePassword, Type: providers.InputTypePassword, Required: true},
				}
				execResp.Status = providers.ExecUserInputRequired
				return false
			}
			return true
		}).Maybe()
	return mockExec
}

func (suite *CredentialsAuthExecutorTestSuite) TestNewCredentialsAuthExecutor() {
	assert.NotNil(suite.T(), suite.executor)
	assert.NotNil(suite.T(), suite.executor.authnProvider)
	assert.NotNil(suite.T(), suite.executor.entityProvider)
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_Success_AuthenticationFlow() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
		RuntimeData: make(map[string]string),
	}

	authenticatedAuthUser := newCredentialsAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_Success_WithEmailAttribute() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"email":    "test@example.com",
			"password": "password123",
		},
		RuntimeData: make(map[string]string),
	}

	// Override GetRequiredInputs to return email and password as required fields
	originalInputs := []providers.Input{
		{Identifier: "email", Type: providers.InputTypeText, Required: true},
		{Identifier: "password", Type: providers.InputTypePassword, Required: true},
	}
	suite.executor.Executor = createMockExecutorWithCustomInputs(
		suite.T(), ExecutorNameCredentialsAuth, originalInputs)

	authenticatedAuthUser := newCredentialsAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		"email": "test@example.com",
	}, map[string]interface{}{
		"password": "password123",
	}, mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_Success_RegistrationFlow() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
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
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.False(suite.T(), resp.AuthUser.IsAuthenticated())
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_Success_WithMultipleAttributes() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"email":    "test@example.com",
			"phone":    "+1234567890",
			"password": "password123",
		},
		RuntimeData: make(map[string]string),
	}

	// Override GetRequiredInputs to return email, phone, and password as required fields
	customInputs := []providers.Input{
		{Identifier: "email", Type: providers.InputTypeText, Required: true},
		{Identifier: "phone", Type: providers.InputTypeText, Required: true},
		{Identifier: "password", Type: providers.InputTypePassword, Required: true},
	}
	suite.executor.Executor = createMockExecutorWithCustomInputs(
		suite.T(), ExecutorNameCredentialsAuth, customInputs)

	authenticatedAuthUser := newCredentialsAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		"email": "test@example.com",
		"phone": "+1234567890",
	}, map[string]interface{}{
		"password": "password123",
	}, mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_UserInputRequired() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		RuntimeData: make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.NotEmpty(suite.T(), resp.Inputs)
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_AuthenticationFailed() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
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
	}, mock.Anything, mock.Anything, mock.Anything).Return(providers.AuthUser{},
		(providers.AuthenticatedClaims)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.invalid_credentials", DefaultValue: "Invalid credentials",
			},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrUserAuthFailed.Code, resp.Error.Code)
	assert.NotEmpty(suite.T(), resp.Inputs, "Inputs should be re-populated for retry")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_UserNotFound_AuthenticationFlow() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
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
	}, mock.Anything, mock.Anything, mock.Anything).Return(providers.AuthUser{},
		(providers.AuthenticatedClaims)(nil), &tidcommon.ServiceError{
			Type:             tidcommon.ClientErrorType,
			ErrorDescription: tidcommon.I18nMessage{Key: "error.test.user_not_found", DefaultValue: "User not found"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrUserAuthFailed.Code, resp.Error.Code,
		"Failure reason should contain authentication failure message")
	assert.NotEmpty(suite.T(), resp.Inputs, "Inputs should be re-populated for retry")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_UserAlreadyExists_RegistrationFlow() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
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
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrUserAlreadyExists.Code, resp.Error.Code)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_ServiceError() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
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
	}, mock.Anything, mock.Anything, mock.Anything).Return(providers.AuthUser{},
		(providers.AuthenticatedClaims)(nil), &tidcommon.ServiceError{
			Type:  tidcommon.ServerErrorType,
			Error: tidcommon.I18nMessage{Key: "error.test.database_error", DefaultValue: "database error"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_AuthenticationServiceError() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Error: tidcommon.I18nMessage{
				Key: "error.test.internal_server_error", DefaultValue: "internal server error",
			},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrUserAuthFailed.Code, resp.Error.Code)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestAuthenticateUser_SuccessfulAuthentication() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	authenticatedAuthUser := newCredentialsAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{}, nil)

	err := suite.executor.authenticateUser(ctx, execResp, nil)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestAuthenticateUser_Success_WithAuthenticatedClaims() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	authenticatedAuthUser := newCredentialsAuthAuthenticatedUser()
	runtimeAttrs := providers.AuthenticatedClaims{
		"username": "testuser",
		"email":    "fetched@example.com",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(authenticatedAuthUser, runtimeAttrs, nil)

	err := suite.executor.authenticateUser(ctx, execResp, nil)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "testuser", execResp.RuntimeData["username"])
	assert.Equal(suite.T(), "fetched@example.com", execResp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestAuthenticateUser_AuthenticationFlow_NoRedundantIdentifyUser() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	authenticatedAuthUser := newCredentialsAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{}, nil)

	err := suite.executor.authenticateUser(ctx, execResp, nil)

	assert.NoError(suite.T(), err)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestAuthenticateUser_RegistrationFlow_CallsIdentifyUser() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			userAttributeUsername: "newuser",
			userAttributePassword: "password123",
		},
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeUsername: "newuser",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	err := suite.executor.authenticateUser(ctx, execResp, nil)

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockAuthnProvider.AssertNotCalled(suite.T(), "AuthenticateUser")
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_RetryableAuthenticationErrors() {
	tests := []struct {
		name              string
		username          string
		password          string
		errorCode         string
		expectedErrorCode string
		message           string
	}{
		{
			name:              "Invalid credentials",
			username:          "testuser",
			password:          "wrongpassword",
			errorCode:         authnprovidermgr.ErrorAuthenticationFailed.Code,
			expectedErrorCode: ErrInvalidCredentials.Code,
			message:           "Should return specific failure reason for invalid credentials",
		},
		{
			name:              "User not found",
			username:          "nonexistent",
			password:          "password123",
			errorCode:         authnprovidermgr.ErrorUserNotFound.Code,
			expectedErrorCode: ErrUserNotFound.Code,
			message:           "Should return specific failure reason for user not found",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			suite.mockAuthnProvider.ExpectedCalls = nil
			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    providers.FlowTypeAuthentication,
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
				providers.AuthUser{}, (providers.AuthenticatedClaims)(nil), &tidcommon.ServiceError{
					Type: tidcommon.ClientErrorType,
					Code: tt.errorCode,
				})

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, providers.ExecUserInputRequired, resp.Status)
			assert.Equal(t, tt.expectedErrorCode, resp.Error.Code, tt.message)
			assert.NotEmpty(t, resp.Inputs, "Inputs should be re-populated for retry")
			assert.Len(t, resp.Inputs, 2, "Should include both username and password inputs")
			suite.mockAuthnProvider.AssertExpectations(t)
		})
	}
}

func (suite *CredentialsAuthExecutorTestSuite) TestGetAuthenticatedUser_ClientError_ReturnsInputsForRetry() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, map[string]interface{}{
		userAttributeUsername: "testuser",
	}, map[string]interface{}{
		userAttributePassword: "password123",
	}, mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, (providers.AuthenticatedClaims)(nil), &tidcommon.ServiceError{
			Type:             tidcommon.ClientErrorType,
			Code:             authnprovidermgr.ErrorAuthenticationFailed.Code,
			ErrorDescription: tidcommon.I18nMessage{Key: "error.test.wrong_password", DefaultValue: "wrong password"},
		})

	err := suite.executor.authenticateUser(ctx, execResp, nil)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, execResp.Status,
		"Should return ExecUserInputRequired for invalid credentials")
	assert.Equal(suite.T(), ErrInvalidCredentials.Code, execResp.Error.Code)
	assert.NotEmpty(suite.T(), execResp.Inputs, "Inputs should be re-populated for retry")
	assert.Len(suite.T(), execResp.Inputs, 2, "Should include both username and password inputs")
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_PreResolvedUser_RequestsPassword() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			userAttributeUserID: "pre-resolved-user-123",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Len(suite.T(), resp.Inputs, 1)
	assert.Equal(suite.T(), userAttributePassword, resp.Inputs[0].Identifier)
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_PreResolvedUser_WithPassword() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributePassword: "password123",
		},
		RuntimeData: map[string]string{
			userAttributeUserID: "pre-resolved-user-123",
		},
		Application: providers.Application{},
	}

	authenticatedAuthUser := newCredentialsAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything,
		map[string]interface{}{userAttributeUserID: "pre-resolved-user-123"},
		map[string]interface{}{userAttributePassword: "password123"},
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
}

// identifierInputsFixture returns the mutually exclusive identifier inputs used by the
// identifier-input tests, mirroring a node reachable from several identifier prompts.
func identifierInputsFixture() []providers.Input {
	return []providers.Input{
		{Identifier: "uin", Type: providers.InputTypeText, Required: true},
		{Identifier: "phone", Type: providers.InputTypeText, Required: true},
		{Identifier: "email", Type: providers.InputTypeText, Required: true},
	}
}

// newIdentifierInputsContext builds a context for a node declaring identifier inputs, with
// only the given identifiers supplied alongside the password.
func newIdentifierInputsContext(supplied map[string]string) *providers.NodeContext {
	userInputs := map[string]string{userAttributePassword: "password123"}
	for k, v := range supplied {
		userInputs[k] = v
	}
	return &providers.NodeContext{
		ExecutionID:          "flow-123",
		FlowType:             providers.FlowTypeAuthentication,
		UserInputs:           userInputs,
		RuntimeData:          make(map[string]string),
		NodeIdentifierInputs: identifierInputsFixture(),
	}
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_IdentifierInputs_PassesSuppliedIdentifier() {
	ctx := newIdentifierInputsContext(map[string]string{"phone": "+919876543210@phone"})
	suite.executor.Executor = createMockExecutorWithCustomInputs(suite.T(), ExecutorNameCredentialsAuth,
		[]providers.Input{{Identifier: userAttributePassword, Type: providers.InputTypePassword, Required: true}})

	authenticatedAuthUser := newCredentialsAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything,
		map[string]interface{}{"phone": "+919876543210@phone"},
		map[string]interface{}{userAttributePassword: "password123"},
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_IdentifierInputs_MissingIdentifierPrompts() {
	ctx := newIdentifierInputsContext(nil)
	suite.executor.Executor = createMockExecutorWithCustomInputs(suite.T(), ExecutorNameCredentialsAuth,
		[]providers.Input{{Identifier: userAttributePassword, Type: providers.InputTypePassword, Required: true}})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Subset(suite.T(), resp.Inputs, identifierInputsFixture())
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_IdentifierInputs_AmbiguousIdentifierFails() {
	ctx := newIdentifierInputsContext(map[string]string{"uin": "4358192047", "phone": "+919876543210@phone"})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrAmbiguousIdentifier.Code, resp.Error.Code)
}

func (suite *CredentialsAuthExecutorTestSuite) TestExecute_WithoutIdentifierInputs_UsesDeclaredInputs() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributeUsername: "testuser",
			userAttributePassword: "password123",
		},
		RuntimeData: make(map[string]string),
	}

	authenticatedAuthUser := newCredentialsAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything,
		map[string]interface{}{userAttributeUsername: "testuser"},
		map[string]interface{}{userAttributePassword: "password123"},
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

// TestExecute_PreResolvedUser_SkipsIdentifierInputs verifies that when a userID is
// pre-resolved, identifier resolution and its ambiguity validation are skipped even
// though the node declares identifier inputs and multiple are supplied. Authentication
// must proceed using only the pre-resolved userID rather than failing as ambiguous.
func (suite *CredentialsAuthExecutorTestSuite) TestExecute_PreResolvedUser_SkipsIdentifierInputs() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			userAttributePassword: "password123",
			"uin":                 "4358192047",
			"phone":               "+919876543210@phone",
		},
		RuntimeData: map[string]string{
			userAttributeUserID: "pre-resolved-user-123",
		},
		NodeIdentifierInputs: identifierInputsFixture(),
	}

	authenticatedAuthUser := newCredentialsAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything,
		map[string]interface{}{userAttributeUserID: "pre-resolved-user-123"},
		map[string]interface{}{userAttributePassword: "password123"},
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

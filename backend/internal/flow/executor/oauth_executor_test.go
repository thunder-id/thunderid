/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

type OAuthExecutorTestSuite struct {
	suite.Suite
	mockOAuthService  *oauthmock.OAuthAuthnCoreServiceInterfaceMock
	mockIDPService    *idpmock.IDPServiceInterfaceMock
	mockFlowFactory   *coremock.FlowFactoryInterfaceMock
	mockAuthnProvider *managermock.AuthnProviderManagerMock
	executor          oAuthExecutorInterface
}

func TestOAuthExecutorSuite(t *testing.T) {
	suite.Run(t, new(OAuthExecutorTestSuite))
}

func (suite *OAuthExecutorTestSuite) SetupTest() {
	suite.mockOAuthService = oauthmock.NewOAuthAuthnCoreServiceInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())

	defaultInputs := []providers.Input{{Identifier: "code", Type: "string", Required: true}}
	mockExec := createMockAuthExecutor(suite.T(), ExecutorNameOAuth)
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameOAuth, providers.ExecutorTypeAuthentication,
		defaultInputs, []providers.Input{}, mock.Anything).Return(mockExec)

	suite.executor = newOAuthExecutor(ExecutorNameOAuth, defaultInputs, []providers.Input{},
		suite.mockFlowFactory, suite.mockIDPService, suite.mockOAuthService,
		suite.mockAuthnProvider, providers.IDPTypeOAuth)
}

func newOAuthAuthenticatedUser() providers.AuthUser {
	var authUser providers.AuthUser
	_ = authUser.UnmarshalJSON([]byte(`{"entityReferenceToken":"tok","attributeToken":"tok"}`))
	return authUser
}

func (suite *OAuthExecutorTestSuite) TestNewOAuthExecutor() {
	assert.NotNil(suite.T(), suite.executor)
}

func (suite *OAuthExecutorTestSuite) TestExecute_CodeNotProvided_BuildsAuthorizeURL() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		NodeInputs:  []providers.Input{{Identifier: "code", Type: "string", Required: true}},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return("https://oauth.provider.com/authorize?client_id=abc", nil)

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&providers.IDPDTO{ID: "idp-123", Name: "TestIDP"}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecExternalRedirection, resp.Status)
	assert.Contains(suite.T(), resp.RedirectURL, "https://oauth.provider.com/authorize?client_id=abc")
	assert.Contains(suite.T(), resp.RedirectURL, "state=")
	assert.Equal(suite.T(), "TestIDP", resp.AdditionalData[common.DataIDPName])
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyOAuthState])
	suite.mockOAuthService.AssertExpectations(suite.T())
	suite.mockIDPService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestExecute_CodeProvided_AuthenticatesUser() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	authenticatedAuthUser := newOAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub": "user-sub-123", "email": "test@example.com", "name": "Test User",
		}, (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "test@example.com", resp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestBuildAuthorizeFlow_Success() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return("https://oauth.provider.com/authorize", nil)
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&providers.IDPDTO{ID: "idp-123", Name: "GoogleIDP"}, nil)

	err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecExternalRedirection, execResp.Status)
	assert.Contains(suite.T(), execResp.RedirectURL, "https://oauth.provider.com/authorize")
	assert.Contains(suite.T(), execResp.RedirectURL, "state=")
	assert.Equal(suite.T(), "GoogleIDP", execResp.AdditionalData[common.DataIDPName])
	assert.NotEmpty(suite.T(), execResp.RuntimeData[common.RuntimeKeyOAuthState])
	suite.mockOAuthService.AssertExpectations(suite.T())
	suite.mockIDPService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestBuildAuthorizeFlow_IDPNotConfigured() {
	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       providers.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "idpId is not configured")
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailMismatch_Fails() { //nolint:dupl
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code":  "auth_code_123",
			"email": "invited@example.com",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{
			"sub":   "user-sub-123",
			"email": "authenticated@example.com",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), ErrInvalidFederatedUser.Error.DefaultValue, execResp.Error.Error.DefaultValue)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_SubMismatch_Fails() { //nolint:dupl
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		RuntimeData: map[string]string{
			"sub": "stored-sub-123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{
			"sub":   "authenticated-sub-456",
			"email": "user@example.com",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), ErrInvalidFederatedUser.Error.DefaultValue, execResp.Error.Error.DefaultValue)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestBuildAuthorizeFlow_BuildURLClientError() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return("", &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.invalid_idp_configuration", DefaultValue: "Invalid IDP configuration",
			},
		})

	err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "Invalid IDP configuration", execResp.Error.ErrorDescription.DefaultValue)
	suite.mockOAuthService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestBuildAuthorizeFlow_BuildURLServerError() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return("", &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "OAUTH-5000",
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.internal_server_error", DefaultValue: "Internal server error",
			},
		})

	err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to build authorize URL")
	suite.mockOAuthService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestGetIdpID_Success() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	idpID, err := suite.executor.GetIdpID(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "idp-123", idpID)
}

func (suite *OAuthExecutorTestSuite) TestGetIdpID_NotConfigured() {
	ctx := &providers.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       providers.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{},
	}

	idpID, err := suite.executor.GetIdpID(ctx)

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), idpID)
	assert.Contains(suite.T(), err.Error(), "idpId is not configured")
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_RegistrationFlow_UserNotFound() { //nolint:dupl
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{
			"sub": "new-user-sub", "email": "newuser@example.com", "name": "New User",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "new-user-sub", execResp.RuntimeData["sub"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_AuthFlow_UserNotFound() { //nolint:dupl
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_UserAlreadyExists_RegistrationFlow() { //nolint:dupl
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	authenticatedAuthUser := newOAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub": "existing-user-sub",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_NoCodeProvided() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_ProviderClientError() { //nolint:dupl
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil), &tidcommon.ServiceError{
			Type:             tidcommon.ClientErrorType,
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Invalid authorization code"},
		})

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "Invalid authorization code", execResp.Error.ErrorDescription.DefaultValue)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_ProviderServerError() { //nolint:dupl
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, (providers.AuthenticatedClaims)(nil), &tidcommon.ServiceError{
			Type:             tidcommon.ServerErrorType,
			Code:             "AUTH-5000",
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Internal authentication error"},
		})

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "federated authentication failed")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestHasRequiredInputs_CodeProvided() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
	}

	execResp := &providers.ExecutorResponse{
		Inputs: []providers.Input{},
	}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
}

func (suite *OAuthExecutorTestSuite) TestHasRequiredInputs_CodeNotProvided() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		NodeInputs:  []providers.Input{{Identifier: "code", Type: "string", Required: true}},
	}

	execResp := &providers.ExecutorResponse{
		Inputs: []providers.Input{},
	}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.False(suite.T(), result)
	assert.NotEmpty(suite.T(), execResp.Inputs)
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserAttributes_WithEmail() {
	userInfo := map[string]string{
		"sub":      "user-sub-123",
		"email":    "test@example.com",
		"name":     "Test User",
		"username": "testuser",
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	attributes := suite.executor.(*oAuthExecutor).getContextUserAttributes(execResp, userInfo)

	assert.NotNil(suite.T(), attributes)
	assert.Equal(suite.T(), "test@example.com", attributes["email"])
	assert.Equal(suite.T(), "Test User", attributes["name"])
	assert.NotContains(suite.T(), attributes, "sub")
	assert.NotContains(suite.T(), attributes, "username")
	assert.Equal(suite.T(), "test@example.com", execResp.RuntimeData["email"])
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserAttributes_WithoutEmail() {
	userInfo := map[string]string{
		"sub":  "user-sub-123",
		"name": "Test User",
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	attributes := suite.executor.(*oAuthExecutor).getContextUserAttributes(execResp, userInfo)

	assert.NotNil(suite.T(), attributes)
	assert.Equal(suite.T(), "Test User", attributes["name"])
	assert.NotContains(suite.T(), attributes, "email")
	assert.NotContains(suite.T(), execResp.RuntimeData, "email")
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserAttributes_WithEmptyEmail() {
	userInfo := map[string]string{
		"sub":   "user-sub-123",
		"email": "",
		"name":  "Test User",
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	attributes := suite.executor.(*oAuthExecutor).getContextUserAttributes(execResp, userInfo)

	assert.NotNil(suite.T(), attributes)
	assert.Equal(suite.T(), "", attributes["email"])
	assert.NotContains(suite.T(), execResp.RuntimeData, "email")
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserAttributes_FilterSkipAttributes() {
	userInfo := map[string]string{
		"sub":      "user-sub-123",
		"email":    "test@example.com",
		"name":     "Test User",
		"username": "testuser",
		"id":       "some-id",
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	attributes := suite.executor.(*oAuthExecutor).getContextUserAttributes(execResp, userInfo)

	assert.NotNil(suite.T(), attributes)
	assert.Equal(suite.T(), "test@example.com", attributes["email"])
	assert.Equal(suite.T(), "Test User", attributes["name"])
	assert.NotContains(suite.T(), attributes, "sub")
	assert.NotContains(suite.T(), attributes, "username")
	assert.NotContains(suite.T(), attributes, "id")
	assert.Equal(suite.T(), "test@example.com", execResp.RuntimeData["email"])
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_RegistrationFlow_WithEmail() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{
			"sub": "new-user-sub", "email": "newuser@example.com", "name": "New User",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "new-user-sub", execResp.RuntimeData["sub"])
	assert.Equal(suite.T(), "newuser@example.com", execResp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserAttributes_WithEmail_NilRuntimeData() {
	userInfo := map[string]string{
		"sub":   "user-sub-123",
		"email": "test@example.com",
		"name":  "Test User",
	}

	execResp := &providers.ExecutorResponse{
		RuntimeData: nil, // Explicitly nil
	}

	attributes := suite.executor.(*oAuthExecutor).getContextUserAttributes(execResp, userInfo)

	assert.NotNil(suite.T(), attributes)
	assert.Equal(suite.T(), "test@example.com", attributes["email"])
	assert.Equal(suite.T(), "Test User", attributes["name"])
	assert.NotNil(suite.T(), execResp.RuntimeData, "RuntimeData should be initialized")
	assert.Equal(suite.T(), "test@example.com", execResp.RuntimeData["email"])
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_AllowAuthWithoutLocalUser() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId":                               "idp-123",
			"allowAuthenticationWithoutLocalUser": true,
		},
		Application: providers.Application{
			InboundAuthProfile: providers.InboundAuthProfile{
				AllowedUserTypes: []string{"INTERNAL"},
			},
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{
			"sub": "new-user-sub", "email": "newuser@example.com", "name": "New User",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), dataValueTrue, execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
	assert.Equal(suite.T(), "new-user-sub", execResp.RuntimeData["sub"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_PreventAuthWithoutLocalUser() { //nolint:dupl
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId":                               "idp-123",
			"allowAuthenticationWithoutLocalUser": false,
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(providers.AuthUser{}, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_AllowRegistrationWithExistingUser() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId":                             "idp-123",
			"allowRegistrationWithExistingUser": true,
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	authenticatedAuthUser := newOAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub": "existing-user-sub", "email": "existing@example.com", "name": "Existing User",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), dataValueTrue, execResp.RuntimeData[common.RuntimeKeyAllowRegistrationWithExistingUser])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_PreventRegistrationWithExistingUser() { //nolint:dupl
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId":                             "idp-123",
			"allowRegistrationWithExistingUser": false,
		},
	}

	execResp := &providers.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	authenticatedAuthUser := newOAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub": "existing-user-sub",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

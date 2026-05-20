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

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oauthmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

type OAuthExecutorTestSuite struct {
	suite.Suite
	mockOAuthService      *oauthmock.OAuthAuthnCoreServiceInterfaceMock
	mockIDPService        *idpmock.IDPServiceInterfaceMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	mockFlowFactory       *coremock.FlowFactoryInterfaceMock
	mockAuthnProvider     *managermock.AuthnProviderManagerInterfaceMock
	executor              oAuthExecutorInterface
}

func TestOAuthExecutorSuite(t *testing.T) {
	suite.Run(t, new(OAuthExecutorTestSuite))
}

func (suite *OAuthExecutorTestSuite) SetupTest() {
	suite.mockOAuthService = oauthmock.NewOAuthAuthnCoreServiceInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())

	defaultInputs := []common.Input{{Identifier: "code", Type: "string", Required: true}}
	mockExec := createMockAuthExecutor(suite.T(), ExecutorNameOAuth)
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameOAuth, common.ExecutorTypeAuthentication,
		defaultInputs, []common.Input{}).Return(mockExec)

	suite.executor = newOAuthExecutor(ExecutorNameOAuth, defaultInputs, []common.Input{},
		suite.mockFlowFactory, suite.mockIDPService, suite.mockEntityTypeService, suite.mockOAuthService,
		suite.mockAuthnProvider, idp.IDPTypeOAuth)
}

func (suite *OAuthExecutorTestSuite) TestNewOAuthExecutor() {
	assert.NotNil(suite.T(), suite.executor)
}

func (suite *OAuthExecutorTestSuite) TestExecute_CodeNotProvided_BuildsAuthorizeURL() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		NodeInputs:  []common.Input{{Identifier: "code", Type: "string", Required: true}},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return("https://oauth.provider.com/authorize?client_id=abc", nil)

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&idp.IDPDTO{ID: "idp-123", Name: "TestIDP"}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecExternalRedirection, resp.Status)
	assert.Contains(suite.T(), resp.RedirectURL, "https://oauth.provider.com/authorize?client_id=abc")
	assert.Contains(suite.T(), resp.RedirectURL, "state=")
	assert.Equal(suite.T(), "TestIDP", resp.AdditionalData[common.DataIDPName])
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyOAuthState])
	suite.mockOAuthService.AssertExpectations(suite.T())
	suite.mockIDPService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestExecute_CodeProvided_AuthenticatesUser() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub: "user-sub-123",
			ExternalClaims: map[string]interface{}{
				"sub": "user-sub-123", "email": "test@example.com", "name": "Test User"},
			IsExistingUser: true,
			UserID:         "user-123",
			OUID:           "ou-123",
			UserType:       "INTERNAL",
		}, (*serviceerror.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), "user-123", resp.AuthenticatedUser.UserID)
	assert.Equal(suite.T(), "ou-123", resp.AuthenticatedUser.OUID)
	assert.Equal(suite.T(), "test@example.com", resp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestBuildAuthorizeFlow_Success() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return("https://oauth.provider.com/authorize", nil)
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&idp.IDPDTO{ID: "idp-123", Name: "GoogleIDP"}, nil)

	err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecExternalRedirection, execResp.Status)
	assert.Contains(suite.T(), execResp.RedirectURL, "https://oauth.provider.com/authorize")
	assert.Contains(suite.T(), execResp.RedirectURL, "state=")
	assert.Equal(suite.T(), "GoogleIDP", execResp.AdditionalData[common.DataIDPName])
	assert.NotEmpty(suite.T(), execResp.RuntimeData[common.RuntimeKeyOAuthState])
	suite.mockOAuthService.AssertExpectations(suite.T())
	suite.mockIDPService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestBuildAuthorizeFlow_IDPNotConfigured() {
	ctx := &core.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       common.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "idpId is not configured")
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailMismatch_Fails() { //nolint:dupl
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code":  "auth_code_123",
			"email": "invited@example.com",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub: "user-sub-123",
			ExternalClaims: map[string]interface{}{
				"sub":   "user-sub-123",
				"email": "authenticated@example.com",
			},
			IsExistingUser: false,
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "Invalid federated user", execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_SubMismatch_Fails() { //nolint:dupl
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
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

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub: "authenticated-sub-456",
			ExternalClaims: map[string]interface{}{
				"sub":   "authenticated-sub-456",
				"email": "user@example.com",
			},
			IsExistingUser: false,
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "Invalid federated user", execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestBuildAuthorizeFlow_BuildURLClientError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return("", &serviceerror.ServiceError{
			Type: serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.invalid_idp_configuration", DefaultValue: "Invalid IDP configuration",
			},
		})

	err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "Invalid IDP configuration", execResp.FailureReason)
	suite.mockOAuthService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestBuildAuthorizeFlow_BuildURLServerError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return("", &serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "OAUTH-5000",
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.internal_server_error", DefaultValue: "Internal server error",
			},
		})

	err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to build authorize URL")
	suite.mockOAuthService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestGetIdpID_Success() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	idpID, err := suite.executor.GetIdpID(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), "idp-123", idpID)
}

func (suite *OAuthExecutorTestSuite) TestGetIdpID_NotConfigured() {
	ctx := &core.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       common.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{},
	}

	idpID, err := suite.executor.GetIdpID(ctx)

	assert.Error(suite.T(), err)
	assert.Empty(suite.T(), idpID)
	assert.Contains(suite.T(), err.Error(), "idpId is not configured")
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_RegistrationFlow_UserNotFound() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub: "new-user-sub",
			ExternalClaims: map[string]interface{}{
				"sub": "new-user-sub", "email": "newuser@example.com", "name": "New User"},
			IsExistingUser: false,
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), "new-user-sub", execResp.RuntimeData["sub"])
	assert.NotNil(suite.T(), execResp.AuthenticatedUser.Attributes)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_AuthFlow_UserNotFound() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub:    "unknown-user",
			IsExistingUser: false,
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "User not found", execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_UserAlreadyExists_RegistrationFlow() { //nolint:dupl
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub:    "existing-user-sub",
			IsExistingUser: true,
			UserID:         "user-456",
			OUID:           "ou-456",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Contains(suite.T(), execResp.FailureReason, "User already exists")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_NoCodeProvided() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.False(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_ProviderClientError() { //nolint:dupl
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, (*authnprovidermgr.AuthnBasicResult)(nil), &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Invalid authorization code"},
		})

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "Invalid authorization code", execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_ProviderServerError() { //nolint:dupl
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, (*authnprovidermgr.AuthnBasicResult)(nil), &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			Code:             "AUTH-5000",
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Internal authentication error"},
		})

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "federated authentication failed")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestHasRequiredInputs_CodeProvided() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
	}

	execResp := &common.ExecutorResponse{
		Inputs: []common.Input{},
	}

	result := suite.executor.HasRequiredInputs(ctx, execResp)

	assert.True(suite.T(), result)
}

func (suite *OAuthExecutorTestSuite) TestHasRequiredInputs_CodeNotProvided() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		NodeInputs:  []common.Input{{Identifier: "code", Type: "string", Required: true}},
	}

	execResp := &common.ExecutorResponse{
		Inputs: []common.Input{},
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

	execResp := &common.ExecutorResponse{
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

	execResp := &common.ExecutorResponse{
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

	execResp := &common.ExecutorResponse{
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

	execResp := &common.ExecutorResponse{
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
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub: "new-user-sub",
			ExternalClaims: map[string]interface{}{
				"sub": "new-user-sub", "email": "newuser@example.com", "name": "New User"},
			IsExistingUser: false,
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), "new-user-sub", execResp.RuntimeData["sub"])
	assert.Equal(suite.T(), "newuser@example.com", execResp.RuntimeData["email"])
	assert.Equal(suite.T(), "newuser@example.com", execResp.AuthenticatedUser.Attributes["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserAttributes_WithEmail_NilRuntimeData() {
	userInfo := map[string]string{
		"sub":   "user-sub-123",
		"email": "test@example.com",
		"name":  "Test User",
	}

	execResp := &common.ExecutorResponse{
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
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId":                               "idp-123",
			"allowAuthenticationWithoutLocalUser": true,
		},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"INTERNAL"},
			},
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub: "new-user-sub",
			ExternalClaims: map[string]interface{}{
				"sub": "new-user-sub", "email": "newuser@example.com", "name": "New User"},
			IsExistingUser: false,
		}, (*serviceerror.ServiceError)(nil))
	suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, mock.Anything, "INTERNAL").
		Return(&entitytype.EntityType{
			Name:                  "INTERNAL",
			AllowSelfRegistration: true,
			OUID:                  "ou-123",
		}, nil)

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), dataValueTrue, execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
	assert.Equal(suite.T(), "new-user-sub", execResp.RuntimeData["sub"])
	assert.NotNil(suite.T(), execResp.AuthenticatedUser.Attributes)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_PreventAuthWithoutLocalUser() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId":                               "idp-123",
			"allowAuthenticationWithoutLocalUser": false,
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub:    "new-user-sub",
			IsExistingUser: false,
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "User not found", execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_AllowRegistrationWithExistingUser() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId":                             "idp-123",
			"allowRegistrationWithExistingUser": true,
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub: "existing-user-sub",
			ExternalClaims: map[string]interface{}{
				"sub": "existing-user-sub", "email": "existing@example.com", "name": "Existing User"},
			IsExistingUser: true,
			UserID:         "user-123",
			OUID:           "ou-123",
			UserType:       "INTERNAL",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), "user-123", execResp.AuthenticatedUser.UserID)
	assert.Equal(suite.T(), dataValueTrue, execResp.RuntimeData[common.RuntimeKeySkipProvisioning])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_PreventRegistrationWithExistingUser() { //nolint:dupl
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		NodeProperties: map[string]interface{}{
			"idpId":                             "idp-123",
			"allowRegistrationWithExistingUser": false,
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub:    "existing-user-sub",
			IsExistingUser: true,
			UserID:         "user-123",
			OUID:           "ou-123",
			UserType:       "INTERNAL",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "User already exists with the provided sub claim.", execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestResolveUserTypeForAutoProvisioning() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"INTERNAL"},
			},
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, mock.Anything, "INTERNAL").
		Return(&entitytype.EntityType{
			Name:                  "INTERNAL",
			AllowSelfRegistration: true,
			OUID:                  "ou-123",
		}, nil)

	err := suite.executor.(*oAuthExecutor).resolveUserTypeForAutoProvisioning(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotEqual(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "INTERNAL", execResp.RuntimeData[userTypeKey])
	assert.Equal(suite.T(), "ou-123", execResp.RuntimeData[defaultOUIDKey])
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestResolveUserTypeForAutoProvisioning_Failures() {
	tests := []struct {
		name                  string
		allowedUserTypes      []string
		mockSetup             func()
		expectedFailureReason string
	}{
		{
			name:                  "NoAllowedUserTypes",
			allowedUserTypes:      []string{},
			mockSetup:             func() {},
			expectedFailureReason: errCannotProvisionUserAutomatically,
		},
		{
			name:             "NoSelfRegistrationEnabled",
			allowedUserTypes: []string{"INTERNAL"},
			mockSetup: func() {
				suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, mock.Anything, "INTERNAL").
					Return(&entitytype.EntityType{
						Name:                  "INTERNAL",
						AllowSelfRegistration: false,
						OUID:                  "ou-123",
					}, nil).Once()
			},
			expectedFailureReason: errSelfRegistrationDisabled,
		},
		{
			name:             "MultipleSelfRegistrationEnabled",
			allowedUserTypes: []string{"INTERNAL", "CUSTOMER"},
			mockSetup: func() {
				suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, mock.Anything, "INTERNAL").
					Return(&entitytype.EntityType{
						Name:                  "INTERNAL",
						AllowSelfRegistration: true,
						OUID:                  "ou-123",
					}, nil).Once()
				suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, mock.Anything, "CUSTOMER").
					Return(&entitytype.EntityType{
						Name:                  "CUSTOMER",
						AllowSelfRegistration: true,
						OUID:                  "ou-456",
					}, nil).Once()
			},
			expectedFailureReason: errCannotProvisionUserAutomatically,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    common.FlowTypeAuthentication,
				Application: appmodel.Application{
					InboundAuthProfile: inboundmodel.InboundAuthProfile{
						AllowedUserTypes: tt.allowedUserTypes,
					},
				},
			}

			execResp := &common.ExecutorResponse{
				AdditionalData: make(map[string]string),
				RuntimeData:    make(map[string]string),
			}

			tt.mockSetup()

			err := suite.executor.(*oAuthExecutor).resolveUserTypeForAutoProvisioning(ctx, execResp)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
			assert.Equal(suite.T(), tt.expectedFailureReason, execResp.FailureReason)
			suite.mockEntityTypeService.AssertExpectations(suite.T())
		})
	}
}

func (suite *OAuthExecutorTestSuite) TestResolveUserTypeForAutoProvisioning_GetEntityTypeError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"INTERNAL"},
			},
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, mock.Anything, "INTERNAL").
		Return(nil, &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			Code:             "SCHEMA-5000",
			ErrorDescription: i18ncore.I18nMessage{Key: "error.test.internal_error", DefaultValue: "Internal error"},
		})

	err := suite.executor.(*oAuthExecutor).resolveUserTypeForAutoProvisioning(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "error while retrieving user type")
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserForRegistration_WithExistingUser_SkipProvisioningFlag() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		RuntimeData: map[string]string{
			common.RuntimeKeySkipProvisioning: dataValueTrue,
		},
		NodeProperties: map[string]interface{}{
			"allowRegistrationWithExistingUser": true,
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	existingUser := &entityprovider.Entity{
		ID:   "user-456",
		OUID: "ou-456",
		Type: "INTERNAL",
	}

	contextUser, err := suite.executor.(*oAuthExecutor).getContextUserForRegistration(
		ctx, execResp, "test-sub", existingUser, false)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), contextUser)
	assert.True(suite.T(), contextUser.IsAuthenticated)
	assert.Equal(suite.T(), "user-456", contextUser.UserID)
	assert.Equal(suite.T(), dataValueTrue, execResp.RuntimeData[common.RuntimeKeySkipProvisioning])
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserForRegistration_AmbiguousUser_NoCrossOUProvisioning() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	contextUser, err := suite.executor.(*oAuthExecutor).getContextUserForRegistration(
		ctx, execResp, "ambiguous-sub", nil, true)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), contextUser)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "User identity is ambiguous and cannot be registered.", execResp.FailureReason)
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserForRegistration_AmbiguousUser_AllowRegistrationOnly() {
	// allowRegistrationWithExistingUser=true but allowCrossOUProvisioning=false — should still fail.
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		NodeProperties: map[string]interface{}{
			"idpId":                             "idp-123",
			"allowRegistrationWithExistingUser": true,
			"allowCrossOUProvisioning":          false,
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	contextUser, err := suite.executor.(*oAuthExecutor).getContextUserForRegistration(
		ctx, execResp, "ambiguous-sub", nil, true)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), contextUser)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "User identity is ambiguous and cannot be registered.", execResp.FailureReason)
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserForRegistration_AmbiguousUser_CrossOUProvisioningAllowed() {
	// allowRegistrationWithExistingUser=true AND allowCrossOUProvisioning=true — should proceed.
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		NodeProperties: map[string]interface{}{
			"idpId":                             "idp-123",
			"allowRegistrationWithExistingUser": true,
			"allowCrossOUProvisioning":          true,
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	contextUser, err := suite.executor.(*oAuthExecutor).getContextUserForRegistration(
		ctx, execResp, "ambiguous-sub", nil, true)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), contextUser)
	assert.False(suite.T(), contextUser.IsAuthenticated)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Empty(suite.T(), execResp.FailureReason)
	assert.Equal(suite.T(), "ambiguous-sub", execResp.RuntimeData[userAttributeSub])
}

func (suite *OAuthExecutorTestSuite) TestResolveUserTypeForAutoProvisioning_FailureScenarios() {
	tests := []struct {
		name                  string
		allowedUserTypes      []string
		entityTypes           map[string]*entitytype.EntityType
		expectedFailureReason string
	}{
		{
			name:                  "NoAllowedUserTypes",
			allowedUserTypes:      []string{},
			entityTypes:           nil,
			expectedFailureReason: errCannotProvisionUserAutomatically,
		},
		{
			name:             "NoSelfRegistrationEnabled",
			allowedUserTypes: []string{"TYPE1", "TYPE2"},
			entityTypes: map[string]*entitytype.EntityType{
				"TYPE1": {
					Name:                  "TYPE1",
					AllowSelfRegistration: false,
				},
				"TYPE2": {
					Name:                  "TYPE2",
					AllowSelfRegistration: false,
				},
			},
			expectedFailureReason: errSelfRegistrationDisabled,
		},
		{
			name:             "MultipleEligibleTypes",
			allowedUserTypes: []string{"TYPE1", "TYPE2"},
			entityTypes: map[string]*entitytype.EntityType{
				"TYPE1": {
					Name:                  "TYPE1",
					AllowSelfRegistration: true,
					OUID:                  "ou-1",
				},
				"TYPE2": {
					Name:                  "TYPE2",
					AllowSelfRegistration: true,
					OUID:                  "ou-2",
				},
			},
			expectedFailureReason: errCannotProvisionUserAutomatically,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Clear expectations before each test
			suite.mockEntityTypeService.ExpectedCalls = nil

			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				Application: appmodel.Application{
					InboundAuthProfile: inboundmodel.InboundAuthProfile{
						AllowedUserTypes: tt.allowedUserTypes,
					},
				},
			}

			execResp := &common.ExecutorResponse{
				AdditionalData: make(map[string]string),
				RuntimeData:    make(map[string]string),
			}

			if tt.entityTypes != nil {
				for userType, schema := range tt.entityTypes {
					suite.mockEntityTypeService.On(
						"GetEntityTypeByName", mock.Anything, mock.Anything, userType).
						Return(schema, nil)
				}
			}

			err := suite.executor.(*oAuthExecutor).resolveUserTypeForAutoProvisioning(ctx, execResp)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
			assert.Equal(suite.T(), tt.expectedFailureReason, execResp.FailureReason)

			if tt.entityTypes != nil {
				suite.mockEntityTypeService.AssertExpectations(suite.T())
			}
		})
	}
}

func (suite *OAuthExecutorTestSuite) TestGetContextUserForAuthentication_WithoutLocalUser_NotAllowed() {
	ctx := &core.NodeContext{
		ExecutionID:    "flow-123",
		FlowType:       common.FlowTypeAuthentication,
		NodeProperties: map[string]interface{}{
			// allowAuthenticationWithoutLocalUser not set or false
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	contextUser, err := suite.executor.(*oAuthExecutor).getContextUserForAuthentication(
		ctx, execResp, "test-sub", nil, false)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), contextUser)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "User not found", execResp.FailureReason)
}

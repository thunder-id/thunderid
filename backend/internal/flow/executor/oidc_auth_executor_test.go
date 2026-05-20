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
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/idp"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authn/oidcmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

type OIDCAuthExecutorTestSuite struct {
	suite.Suite
	mockOIDCService       *oidcmock.OIDCAuthnCoreServiceInterfaceMock
	mockIDPService        *idpmock.IDPServiceInterfaceMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	mockFlowFactory       *coremock.FlowFactoryInterfaceMock
	mockAuthnProvider     *managermock.AuthnProviderManagerInterfaceMock
	executor              oidcAuthExecutorInterface
}

func TestOIDCAuthExecutorSuite(t *testing.T) {
	suite.Run(t, new(OIDCAuthExecutorTestSuite))
}

func (suite *OIDCAuthExecutorTestSuite) SetupTest() {
	suite.mockOIDCService = oidcmock.NewOIDCAuthnCoreServiceInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())

	defaultInputs := []common.Input{{Identifier: "code", Type: "string", Required: true}}
	mockExec := createMockAuthExecutor(suite.T(), ExecutorNameOIDCAuth)
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameOIDCAuth, common.ExecutorTypeAuthentication,
		defaultInputs, []common.Input{}).Return(mockExec)

	suite.executor = newOIDCAuthExecutor(ExecutorNameOIDCAuth, defaultInputs, []common.Input{},
		suite.mockFlowFactory, suite.mockIDPService, suite.mockEntityTypeService, suite.mockOIDCService,
		suite.mockAuthnProvider, idp.IDPTypeOIDC)
}

func (suite *OIDCAuthExecutorTestSuite) TestNewOIDCAuthExecutor() {
	assert.NotNil(suite.T(), suite.executor)
}

func (suite *OIDCAuthExecutorTestSuite) TestExecute_CodeNotProvided_BuildsAuthorizeURL() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		NodeInputs:  []common.Input{{Identifier: "code", Type: "string", Required: true}},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	suite.mockOIDCService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return("https://oidc.provider.com/authorize?client_id=abc&scope=openid", nil)

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&idp.IDPDTO{ID: "idp-123", Name: "TestOIDCProvider"}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecExternalRedirection, resp.Status)
	assert.Contains(suite.T(), resp.RedirectURL, "https://oidc.provider.com/authorize")
	assert.Equal(suite.T(), "TestOIDCProvider", resp.AdditionalData[common.DataIDPName])
	suite.mockOIDCService.AssertExpectations(suite.T())
	suite.mockIDPService.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestExecute_CodeProvided_ValidIDToken_AuthenticatesUser() {
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
				"sub": "user-sub-123", "email": "test@example.com", "name": "Test User",
				"iss": "https://oidc.provider.com", "aud": "client-id-123",
				"exp": 1234567890, "iat": 1234567800,
			},
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_ValidIDToken_Success() {
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
			ExternalSub: "user-sub-456",
			ExternalClaims: map[string]interface{}{
				"sub": "user-sub-456", "email": "user@example.com",
				"iss": "https://provider.com", "aud": "client-id",
			},
			IsExistingUser: true,
			UserID:         "user-456",
			OUID:           "ou-456",
			UserType:       "INTERNAL",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_InvalidNonce() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code":  "auth_code_123",
			"nonce": "expected_nonce_123",
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
				"nonce": "different_nonce_456",
			},
			IsExistingUser: false,
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Contains(suite.T(), execResp.FailureReason, "Nonce mismatch")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailMismatch_Fails() { //nolint:dupl
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_SubMismatch_Fails() { //nolint:dupl
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_ProviderClientError() { //nolint:dupl
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
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Invalid ID token"},
		})

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "Invalid ID token", execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_RegistrationFlow_UserNotFound() {
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
				"sub": "new-user-sub", "email": "newuser@example.com", "name": "New User",
			},
			IsExistingUser: false,
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), "new-user-sub", execResp.RuntimeData["sub"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_AuthFlow_UserNotFound() {
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
	assert.Equal(suite.T(), failureReasonUserNotFound, execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_UserAlreadyExists_RegistrationFlow() { //nolint:dupl
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
			UserID:         "user-789",
			OUID:           "ou-789",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Contains(suite.T(), execResp.FailureReason, "User already exists")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_NoCodeProvided() {
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_FiltersNonUserClaimsFromIDToken() {
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
			ExternalSub: "user-sub-123",
			ExternalClaims: map[string]interface{}{
				"sub": "user-sub-123", "email": "user@example.com", "name": "User Name",
				"iss": "https://provider.com", "aud": "client-id",
				"exp": 1234567890, "iat": 1234567800,
				"at_hash": "hash_value", "nonce": "nonce_value",
			},
			IsExistingUser: true,
			UserID:         "user-123",
			OUID:           "ou-123",
			UserType:       "INTERNAL",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Contains(suite.T(), execResp.AuthenticatedUser.Attributes, "email")
	assert.Contains(suite.T(), execResp.AuthenticatedUser.Attributes, "name")
	assert.NotContains(suite.T(), execResp.AuthenticatedUser.Attributes, "iss")
	assert.NotContains(suite.T(), execResp.AuthenticatedUser.Attributes, "aud")
	assert.NotContains(suite.T(), execResp.AuthenticatedUser.Attributes, "exp")
	assert.NotContains(suite.T(), execResp.AuthenticatedUser.Attributes, "iat")
	assert.NotContains(suite.T(), execResp.AuthenticatedUser.Attributes, "at_hash")
	assert.NotContains(suite.T(), execResp.AuthenticatedUser.Attributes, "nonce")
	assert.NotContains(suite.T(), execResp.AuthenticatedUser.Attributes, "sub")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailInIDToken() {
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
			ExternalSub: "user-sub-789",
			ExternalClaims: map[string]interface{}{
				"sub": "user-sub-789", "email": "user@test.com",
				"iss": "https://provider.com", "aud": "client-id",
			},
			IsExistingUser: true,
			UserID:         "user-789",
			OUID:           "ou-789",
			UserType:       "INTERNAL",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), "user@test.com", execResp.RuntimeData["email"])
	assert.Equal(suite.T(), "user@test.com", execResp.AuthenticatedUser.Attributes["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_NoEmailInIDToken() {
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
			ExternalSub: "user-sub-789",
			ExternalClaims: map[string]interface{}{
				"sub": "user-sub-789", "name": "Test User",
				"iss": "https://provider.com", "aud": "client-id",
			},
			IsExistingUser: true,
			UserID:         "user-789",
			OUID:           "ou-789",
			UserType:       "INTERNAL",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	assert.NotContains(suite.T(), execResp.RuntimeData, "email")
	assert.NotContains(suite.T(), execResp.AuthenticatedUser.Attributes, "email")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmptyEmailInIDToken() {
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
			ExternalSub: "user-sub-789",
			ExternalClaims: map[string]interface{}{
				"sub":   "user-sub-789",
				"email": "",
				"iss":   "https://provider.com",
				"aud":   "client-id",
			},
			IsExistingUser: true,
			UserID:         "user-789",
			OUID:           "ou-789",
			UserType:       "INTERNAL",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	assert.NotContains(suite.T(), execResp.RuntimeData, "email")
	assert.Equal(suite.T(), "", execResp.AuthenticatedUser.Attributes["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_RegistrationFlow_WithEmail() {
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
				"sub":   "new-user-sub",
				"email": "newuser@example.com",
				"name":  "New User",
				"iss":   "https://provider.com",
				"aud":   "client-id",
			},
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailFromUserInfo() {
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
			ExternalSub: "user-sub-789",
			ExternalClaims: map[string]interface{}{
				"sub":   "user-sub-789",
				"name":  "Test User",
				"email": "fromUserInfo@example.com",
				"iss":   "https://provider.com",
				"aud":   "client-id",
			},
			IsExistingUser: true,
			UserID:         "user-789",
			OUID:           "ou-789",
			UserType:       "INTERNAL",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), "fromUserInfo@example.com", execResp.RuntimeData["email"])
	assert.Equal(suite.T(), "fromUserInfo@example.com", execResp.AuthenticatedUser.Attributes["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailInIDToken_NilRuntimeData() {
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
		RuntimeData:    nil, // Explicitly nil
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidermgr.AuthnBasicResult{
			ExternalSub: "user-sub-999",
			ExternalClaims: map[string]interface{}{
				"sub":   "user-sub-999",
				"email": "niltest@example.com",
				"iss":   "https://provider.com",
				"aud":   "client-id",
			},
			IsExistingUser: true,
			UserID:         "user-999",
			OUID:           "ou-999",
			UserType:       "INTERNAL",
		}, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthenticatedUser.IsAuthenticated)
	assert.NotNil(suite.T(), execResp.RuntimeData, "RuntimeData should be initialized")
	assert.Equal(suite.T(), "niltest@example.com", execResp.RuntimeData["email"])
	assert.Equal(suite.T(), "niltest@example.com", execResp.AuthenticatedUser.Attributes["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_AllowAuthWithoutLocalUser() {
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
				"sub":   "new-user-sub",
				"email": "newuser@example.com",
				"name":  "New User",
				"iss":   "https://provider.com",
				"aud":   "client-123",
			},
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_PreventAuthWithoutLocalUser() {
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
	assert.Equal(suite.T(), failureReasonUserNotFound, execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_AllowRegistrationWithExistingUser() {
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
				"sub":   "existing-user-sub",
				"email": "existing@example.com",
				"name":  "Existing User",
				"iss":   "https://provider.com",
				"aud":   "client-123",
			},
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

//nolint:dupl
func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_PreventRegistrationWithExistingUser() {
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

func (suite *OIDCAuthExecutorTestSuite) TestGetContextUserAttributes_FiltersNonUserClaims() {
	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	claims := map[string]interface{}{
		"sub":        "user-sub",
		"email":      "user@example.com",
		"name":       "Test User",
		"iss":        "https://provider.com",
		"aud":        "client-123",
		"exp":        float64(1234567890),
		"iat":        float64(1234567000),
		"at_hash":    "hash-value",
		"azp":        "azp-value",
		"nonce":      "nonce-value",
		"given_name": "Test",
	}

	attributes := suite.executor.(*oidcAuthExecutor).getContextUserAttributes(execResp, claims)

	assert.NotNil(suite.T(), attributes)
	assert.Equal(suite.T(), "user@example.com", attributes["email"])
	assert.Equal(suite.T(), "Test User", attributes["name"])
	assert.Equal(suite.T(), "Test", attributes["given_name"])
	assert.NotContains(suite.T(), attributes, "sub")
	assert.NotContains(suite.T(), attributes, "iss")
	assert.NotContains(suite.T(), attributes, "aud")
	assert.NotContains(suite.T(), attributes, "exp")
	assert.NotContains(suite.T(), attributes, "iat")
	assert.NotContains(suite.T(), attributes, "at_hash")
	assert.NotContains(suite.T(), attributes, "azp")
	assert.NotContains(suite.T(), attributes, "nonce")
	assert.Equal(suite.T(), "user@example.com", execResp.RuntimeData["email"])
}

func (suite *OIDCAuthExecutorTestSuite) TestGetContextUserAttributes_EmailAddedToRuntimeData() {
	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	idTokenClaims := map[string]interface{}{
		"sub":        "user-sub",
		"email":      "user@example.com",
		"name":       "Test User",
		"iss":        "https://provider.com",
		"aud":        "client-123",
		"exp":        float64(1234567890),
		"iat":        float64(1234567000),
		"given_name": "Test",
	}

	attributes := suite.executor.(*oidcAuthExecutor).getContextUserAttributes(execResp, idTokenClaims)

	assert.NotNil(suite.T(), attributes)
	assert.Equal(suite.T(), "user@example.com", attributes["email"])
	assert.Equal(suite.T(), "Test User", attributes["name"])
	assert.Equal(suite.T(), "Test", attributes["given_name"])
	assert.NotContains(suite.T(), attributes, "sub")
	assert.NotContains(suite.T(), attributes, "iss")
	assert.NotContains(suite.T(), attributes, "aud")
	assert.NotContains(suite.T(), attributes, "exp")
	assert.NotContains(suite.T(), attributes, "iat")
	assert.Equal(suite.T(), "user@example.com", execResp.RuntimeData["email"])
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_ServerError() { //nolint:dupl
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
			Code:             "OIDC-5000",
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Internal OIDC authentication error"},
		})

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "OIDC authentication failed")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

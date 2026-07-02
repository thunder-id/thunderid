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
	"github.com/thunder-id/thunderid/tests/mocks/authn/oidcmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/idp/idpmock"
)

type OIDCAuthExecutorTestSuite struct {
	suite.Suite
	mockOIDCService   *oidcmock.OIDCAuthnCoreServiceInterfaceMock
	mockIDPService    *idpmock.IDPServiceInterfaceMock
	mockFlowFactory   *coremock.FlowFactoryInterfaceMock
	mockAuthnProvider *managermock.AuthnProviderManagerMock
	executor          oidcAuthExecutorInterface
}

func TestOIDCAuthExecutorSuite(t *testing.T) {
	suite.Run(t, new(OIDCAuthExecutorTestSuite))
}

func (suite *OIDCAuthExecutorTestSuite) SetupTest() {
	suite.mockOIDCService = oidcmock.NewOIDCAuthnCoreServiceInterfaceMock(suite.T())
	suite.mockIDPService = idpmock.NewIDPServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())

	defaultInputs := []providers.Input{{Identifier: "code", Type: "string", Required: true}}
	mockExec := createMockAuthExecutor(suite.T(), ExecutorNameOIDCAuth)
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameOIDCAuth, providers.ExecutorTypeAuthentication,
		defaultInputs, []providers.Input{}, mock.Anything).Return(mockExec)

	suite.executor = newOIDCAuthExecutor(ExecutorNameOIDCAuth, defaultInputs, []providers.Input{},
		suite.mockFlowFactory, suite.mockIDPService, suite.mockOIDCService,
		suite.mockAuthnProvider, providers.IDPTypeOIDC)
}

func newOIDCAuthenticatedUser() providers.AuthUser {
	var authUser providers.AuthUser
	_ = authUser.UnmarshalJSON([]byte(`{"entityReferenceToken":"tok","attributeToken":"tok"}`))
	return authUser
}

func (suite *OIDCAuthExecutorTestSuite) TestNewOIDCAuthExecutor() {
	assert.NotNil(suite.T(), suite.executor)
}

func (suite *OIDCAuthExecutorTestSuite) TestExecute_CodeNotProvided_BuildsAuthorizeURL() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs:  map[string]string{},
		NodeInputs:  []providers.Input{{Identifier: "code", Type: "string", Required: true}},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
	}

	suite.mockOIDCService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return("https://oidc.provider.com/authorize?client_id=abc&scope=openid", nil)

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&providers.IDPDTO{ID: "idp-123", Name: "TestOIDCProvider"}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecExternalRedirection, resp.Status)
	assert.Contains(suite.T(), resp.RedirectURL, "https://oidc.provider.com/authorize")
	assert.Equal(suite.T(), "TestOIDCProvider", resp.AdditionalData[common.DataIDPName])
	suite.mockOIDCService.AssertExpectations(suite.T())
	suite.mockIDPService.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestExecute_CodeProvided_ValidIDToken_AuthenticatesUser() {
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

	authenticatedAuthUser := newOIDCAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub": "user-sub-123", "email": "test@example.com", "name": "Test User",
			"iss": "https://oidc.provider.com", "aud": "client-id-123",
			"exp": 1234567890, "iat": 1234567800,
		}, (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "test@example.com", resp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_ValidIDToken_Success() {
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

	authenticatedAuthUser := newOIDCAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub": "user-sub-456", "email": "user@example.com",
			"iss": "https://provider.com", "aud": "client-id",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_InvalidNonce() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code":  "auth_code_123",
			"nonce": "expected_nonce_123",
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
			"nonce": "different_nonce_456",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Contains(suite.T(), execResp.Error.Error.DefaultValue, "Nonce mismatch")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailMismatch_Fails() { //nolint:dupl
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_SubMismatch_Fails() { //nolint:dupl
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_ProviderClientError() { //nolint:dupl
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
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Invalid ID token"},
		})

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "Invalid ID token", execResp.Error.ErrorDescription.DefaultValue)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_RegistrationFlow_UserNotFound() { //nolint:dupl
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_AuthFlow_UserNotFound() { //nolint:dupl
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_UserAlreadyExists_RegistrationFlow() { //nolint:dupl
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

	authenticatedAuthUser := newOIDCAuthenticatedUser()
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_NoCodeProvided() {
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_FiltersNonUserClaimsFromIDToken() {
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

	authenticatedAuthUser := newOIDCAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub": "user-sub-123", "email": "user@example.com", "name": "User Name",
			"iss": "https://provider.com", "aud": "client-id",
			"exp": 1234567890, "iat": 1234567800,
			"at_hash": "hash_value", "nonce": "nonce_value",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	// Federated attributes are now stored in RuntimeData
	assert.Contains(suite.T(), execResp.RuntimeData, "email")
	assert.Contains(suite.T(), execResp.RuntimeData, "name")
	assert.Contains(suite.T(), execResp.RuntimeData, "iss")
	assert.Contains(suite.T(), execResp.RuntimeData, "aud")
	assert.Contains(suite.T(), execResp.RuntimeData, "exp")
	assert.Contains(suite.T(), execResp.RuntimeData, "iat")
	assert.Contains(suite.T(), execResp.RuntimeData, "at_hash")
	assert.Contains(suite.T(), execResp.RuntimeData, "nonce")
	assert.Contains(suite.T(), execResp.RuntimeData, "sub")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailInIDToken() { //nolint:dupl
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

	authenticatedAuthUser := newOIDCAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub": "user-sub-789", "email": "user@test.com",
			"iss": "https://provider.com", "aud": "client-id",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "user@test.com", execResp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_NoEmailInIDToken() {
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

	authenticatedAuthUser := newOIDCAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub": "user-sub-789", "name": "Test User",
			"iss": "https://provider.com", "aud": "client-id",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.NotContains(suite.T(), execResp.RuntimeData, "email")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmptyEmailInIDToken() { //nolint:dupl
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

	authenticatedAuthUser := newOIDCAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub":   "user-sub-789",
			"email": "",
			"iss":   "https://provider.com",
			"aud":   "client-id",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "", execResp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_RegistrationFlow_WithEmail() {
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
			"sub":   "new-user-sub",
			"email": "newuser@example.com",
			"name":  "New User",
			"iss":   "https://provider.com",
			"aud":   "client-id",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "new-user-sub", execResp.RuntimeData["sub"])
	assert.Equal(suite.T(), "newuser@example.com", execResp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailFromUserInfo() {
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

	authenticatedAuthUser := newOIDCAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub":   "user-sub-789",
			"name":  "Test User",
			"email": "fromUserInfo@example.com",
			"iss":   "https://provider.com",
			"aud":   "client-id",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "fromUserInfo@example.com", execResp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailInIDToken_NilRuntimeData() {
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
		RuntimeData:    nil, // Explicitly nil
	}

	authenticatedAuthUser := newOIDCAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub":   "user-sub-999",
			"email": "niltest@example.com",
			"iss":   "https://provider.com",
			"aud":   "client-id",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.NotNil(suite.T(), execResp.RuntimeData, "RuntimeData should be initialized")
	assert.Equal(suite.T(), "niltest@example.com", execResp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_AllowAuthWithoutLocalUser() {
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
			"sub":   "new-user-sub",
			"email": "newuser@example.com",
			"name":  "New User",
			"iss":   "https://provider.com",
			"aud":   "client-123",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), dataValueTrue, execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
	assert.Equal(suite.T(), "new-user-sub", execResp.RuntimeData["sub"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_PreventAuthWithoutLocalUser() { //nolint:dupl
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

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_AllowRegistrationWithExistingUser() {
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

	authenticatedAuthUser := newOIDCAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{
			"sub":   "existing-user-sub",
			"email": "existing@example.com",
			"name":  "Existing User",
			"iss":   "https://provider.com",
			"aud":   "client-123",
		}, (*tidcommon.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), dataValueTrue, execResp.RuntimeData[common.RuntimeKeyAllowRegistrationWithExistingUser])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

//nolint:dupl
func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_PreventRegistrationWithExistingUser() {
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

	authenticatedAuthUser := newOIDCAuthenticatedUser()
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

func (suite *OIDCAuthExecutorTestSuite) TestGetContextUserAttributes_FiltersNonUserClaims() {
	execResp := &providers.ExecutorResponse{
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
	execResp := &providers.ExecutorResponse{
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
			Code:             "OIDC-5000",
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Internal OIDC authentication error"},
		})

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "OIDC authentication failed")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

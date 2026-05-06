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
	i18ncore "github.com/asgardeo/thunder/internal/system/i18n/core"

	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appmodel "github.com/asgardeo/thunder/internal/application/model"
	authnprovidermgr "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/entitytype"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/flow/core"
	"github.com/asgardeo/thunder/internal/idp"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	"github.com/asgardeo/thunder/tests/mocks/authn/oidcmock"
	"github.com/asgardeo/thunder/tests/mocks/authnprovider/managermock"
	"github.com/asgardeo/thunder/tests/mocks/entitytypemock"
	"github.com/asgardeo/thunder/tests/mocks/flow/coremock"
	"github.com/asgardeo/thunder/tests/mocks/idp/idpmock"
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

func (suite *OIDCAuthExecutorTestSuite) TestExecute_CodeProvided_ValidIDToken_AuthenticatesUser() { //nolint:dupl
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

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":true,`+
		`"runtimeAttributes":{"sub":"user-sub-123"}}],`+
		`"userHistory":[{"userId":"user-123","userType":"INTERNAL","ouId":"ou-123","isValuesIncluded":true}],`+
		`"userState":"exists"}`), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "user-123", resp.AuthUser.GetUserID())
	assert.Equal(suite.T(), "ou-123", resp.AuthUser.GetOUID())
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

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":true,`+
		`"runtimeAttributes":{"sub":"user-sub-456"}}],`+
		`"userHistory":[{"userId":"user-456","userType":"INTERNAL","ouId":"ou-456","isValuesIncluded":true}],`+
		`"userState":"exists"}`), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
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

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":false,`+
		`"runtimeAttributes":{"sub":"user-sub-123","nonce":"different_nonce_456"}}],"userHistory":[],`+
		`"userState":"not_exists"}`),
		&mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Contains(suite.T(), execResp.FailureReason, "Nonce mismatch")
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
		Return(authnprovidermgr.AuthUser{}, &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Invalid ID token"},
		})

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "Invalid ID token", execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_RegistrationFlow_UserNotFound() { //nolint:dupl
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

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":false,`+
		`"runtimeAttributes":{"sub":"new-user-sub"}}],"userHistory":[],"userState":"not_exists"}`), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
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

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":false,`+
		`"runtimeAttributes":{"sub":"unknown-user"}}],"userHistory":[],"userState":"not_exists"}`), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))

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

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":true,`+
		`"runtimeAttributes":{"sub":"existing-user-sub"}}],`+
		`"userHistory":[{"userId":"user-789","ouId":"ou-789","isValuesIncluded":true}],"userState":"exists"}`),
		&mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))

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

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "federated authentication failed")
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_AllowAuthWithoutLocalUser() { //nolint:dupl
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
			AllowedUserTypes: []string{"INTERNAL"},
		},
	}

	execResp := &common.ExecutorResponse{
		AdditionalData: make(map[string]string),
		RuntimeData:    make(map[string]string),
	}

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":false,`+
		`"runtimeAttributes":{"sub":"new-user-sub"}}],"userHistory":[],"userState":"not_exists"}`), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))
	suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, mock.Anything, "INTERNAL").
		Return(&entitytype.EntityType{
			Name:                  "INTERNAL",
			AllowSelfRegistration: true,
			OUID:                  "ou-123",
		}, nil)

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), dataValueTrue, execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
	assert.Equal(suite.T(), "new-user-sub", execResp.RuntimeData["sub"])
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

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":false,`+
		`"runtimeAttributes":{"sub":"new-user-sub"}}],"userHistory":[],"userState":"not_exists"}`), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), failureReasonUserNotFound, execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_AllowRegistrationWithExistingUser() { //nolint:dupl
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

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":true,`+
		`"runtimeAttributes":{"sub":"existing-user-sub"}}],`+
		`"userHistory":[{"userId":"user-123","userType":"INTERNAL","ouId":"ou-123","isValuesIncluded":true}],`+
		`"userState":"exists"}`), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.True(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "user-123", execResp.AuthUser.GetUserID())
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

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":true,`+
		`"runtimeAttributes":{"sub":"existing-user-sub"}}],`+
		`"userHistory":[{"userId":"user-123","userType":"INTERNAL","ouId":"ou-123","isValuesIncluded":true}],`+
		`"userState":"exists"}`), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), "User already exists with the provided sub claim.", execResp.FailureReason)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OIDCAuthExecutorTestSuite) TestProcessAuthFlowResponse_EmailAppendedToRuntimeData() {
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

	var mockAuthUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"OIDC","isVerified":true,`+
		`"runtimeAttributes":{"sub":"user-sub-123","email":"user@example.com"}}],`+
		`"userHistory":[{"userId":"user-123","userType":"INTERNAL","ouId":"ou-123","isValuesIncluded":true}],`+
		`"userState":"exists"}`), &mockAuthUser)
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(mockAuthUser, (*serviceerror.ServiceError)(nil))

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), "user@example.com", execResp.RuntimeData["email"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
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
		Return(authnprovidermgr.AuthUser{}, &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			Code:             "OIDC-5000",
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "Internal OIDC authentication error"},
		})

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "OIDC authentication failed")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

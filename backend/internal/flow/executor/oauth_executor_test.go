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

	"github.com/thunder-id/thunderid/internal/flow/common"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
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

	mockExec := createMockAuthExecutor(suite.T(), ExecutorNameOAuth)
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameOAuth, providers.ExecutorTypeAuthentication,
		defaultCodeOnlyInputs, []providers.Input{}, mock.Anything).Return(mockExec)

	suite.executor = newOAuthExecutor(ExecutorNameOAuth, defaultCodeOnlyInputs, []providers.Input{},
		suite.mockFlowFactory, suite.mockIDPService, suite.mockOAuthService,
		suite.mockAuthnProvider, providers.IDPTypeOAuth)
}

// expectIdentityProviderResolved stubs the AuthorizationRuleMapping resolution ProcessAuthFlowResponse
// performs on every successful federated login. Returns an IDPDTO with no AttributeConfiguration, so
// it resolves no mapped targets and leaves runtime data untouched.
func expectIdentityProviderResolved(m *idpmock.IDPServiceInterfaceMock) {
	m.On("GetIdentityProvider", mock.Anything, "idp-123").Return(&providers.IDPDTO{ID: "idp-123"}, nil).Maybe()
	m.On("GetDirectAuthorizationTargets", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, (*tidcommon.ServiceError)(nil)).Maybe()
}

func newOAuthAuthenticatedUser() providers.AuthUser {
	var authUser providers.AuthUser
	_ = authUser.UnmarshalJSON([]byte(`{"default":{"entityReferenceToken":"tok","attributeToken":"tok"}}`))
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

	oauthURL := "https://oauth.provider.com/authorize?client_id=abc&state=test-state"
	suite.mockOAuthService.On("BuildAuthorizeURL", mock.Anything, "idp-123").
		Return(oauthURL, map[string]string{oauth2const.RequestParamState: "test-state"}, nil)

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&providers.IDPDTO{ID: "idp-123", Name: "TestIDP"}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecExternalRedirection, resp.Status)
	assert.Contains(suite.T(), resp.RedirectURL, "https://oauth.provider.com/authorize?client_id=abc&state=test-state")
	assert.Equal(suite.T(), "TestIDP", resp.AdditionalData[common.DataIDPName])
	assert.Equal(suite.T(), "test-state", resp.RuntimeData[common.RuntimeKeyOAuthState])
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
	expectEntityReferenceResolved(suite.mockAuthnProvider, authenticatedAuthUser)

	expectIdentityProviderResolved(suite.mockIDPService)
	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "test@example.com", externalClaim(resp.RuntimeData, "email"))
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
		Return("https://oauth.provider.com/authorize?state=test-state",
			map[string]string{oauth2const.RequestParamState: "test-state"}, nil)
	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&providers.IDPDTO{ID: "idp-123", Name: "GoogleIDP"}, nil)

	_, err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecExternalRedirection, execResp.Status)
	assert.Contains(suite.T(), execResp.RedirectURL, "https://oauth.provider.com/authorize?state=test-state")
	assert.Equal(suite.T(), "GoogleIDP", execResp.AdditionalData[common.DataIDPName])
	assert.Equal(suite.T(), "test-state", execResp.RuntimeData[common.RuntimeKeyOAuthState])
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

	_, err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

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

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").Return(&providers.IDPDTO{
		ID: "idp-123",
		AttributeConfiguration: &providers.AttributeConfiguration{
			AccountLinking: &providers.AccountLinking{Attributes: []string{"email"}},
		},
	}, nil)

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
			common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-123", "stored-sub-123", nil),
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

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return(&providers.IDPDTO{ID: "idp-123"}, nil)

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), ErrInvalidFederatedUser.Error.DefaultValue, execResp.Error.Error.DefaultValue)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_IDPRetrievalFails_FailsClosed() { //nolint:dupl
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			"code": "auth_code_123",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyExternalIdentity: externalIdentityEntry("idp-123", "stored-sub-123", nil),
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

	suite.mockIDPService.On("GetIdentityProvider", mock.Anything, "idp-123").
		Return((*providers.IDPDTO)(nil), &tidcommon.InternalServerError)

	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.EqualError(suite.T(), err,
		"failed to retrieve identity provider idp-123: "+tidcommon.InternalServerError.Code)
	assert.Nil(suite.T(), execResp.Error)
	suite.mockIDPService.AssertExpectations(suite.T())
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
		Return("", (map[string]string)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.invalid_idp_configuration", DefaultValue: "Invalid IDP configuration",
			},
		})

	_, err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

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
		Return("", (map[string]string)(nil), &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
			Code: "OAUTH-5000",
			ErrorDescription: tidcommon.I18nMessage{
				Key: "error.test.internal_server_error", DefaultValue: "Internal server error",
			},
		})

	_, err := suite.executor.BuildAuthorizeFlow(ctx, execResp)

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

	expectIdentityProviderResolved(suite.mockIDPService)
	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "new-user-sub", externalSub(execResp.RuntimeData))
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

	expectIdentityProviderResolved(suite.mockIDPService)
	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_NoLocalUser_EntityStateNotExists() { //nolint:dupl
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
		Return(newOAuthAuthenticatedUser(), providers.AuthenticatedClaims{
			"sub": "user-sub-123", "email": "new@example.com",
		}, (*tidcommon.ServiceError)(nil))
	// Entity reference resolution finds no matching local account, modeling account linking
	// that did not resolve to an existing local user.
	expectEntityReferenceNotFound(suite.mockAuthnProvider, newOAuthAuthenticatedUser())

	expectIdentityProviderResolved(suite.mockIDPService)
	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), entityStateNotExists, execResp.RuntimeData[common.RuntimeKeyEntityState])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *OAuthExecutorTestSuite) TestProcessAuthFlowResponse_LocalUser_EntityStateExists() { //nolint:dupl
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
		Return(newOAuthAuthenticatedUser(), providers.AuthenticatedClaims{
			"sub": "user-sub-123", "email": "existing@example.com",
		}, (*tidcommon.ServiceError)(nil))
	// A resolved EntityReference models account linking matching an existing local user.
	expectEntityReferenceResolved(suite.mockAuthnProvider, newOAuthAuthenticatedUser())

	expectIdentityProviderResolved(suite.mockIDPService)
	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.Equal(suite.T(), entityStateExists, execResp.RuntimeData[common.RuntimeKeyEntityState])
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
	expectEntityReferenceResolved(suite.mockAuthnProvider, authenticatedAuthUser)

	expectIdentityProviderResolved(suite.mockIDPService)
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

	expectIdentityProviderResolved(suite.mockIDPService)
	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "new-user-sub", externalSub(execResp.RuntimeData))
	assert.Equal(suite.T(), "newuser@example.com", externalClaim(execResp.RuntimeData, "email"))
	suite.mockAuthnProvider.AssertExpectations(suite.T())
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

	expectIdentityProviderResolved(suite.mockIDPService)
	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	assert.False(suite.T(), execResp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), dataValueTrue, execResp.RuntimeData[common.RuntimeKeyUserEligibleForProvisioning])
	assert.Equal(suite.T(), "new-user-sub", externalSub(execResp.RuntimeData))
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

	expectIdentityProviderResolved(suite.mockIDPService)
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
	expectEntityReferenceResolved(suite.mockAuthnProvider, authenticatedAuthUser)

	expectIdentityProviderResolved(suite.mockIDPService)
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
	expectEntityReferenceResolved(suite.mockAuthnProvider, authenticatedAuthUser)

	expectIdentityProviderResolved(suite.mockIDPService)
	err := suite.executor.ProcessAuthFlowResponse(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, execResp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

// An authorization code belongs to the node that requested it. UserInputs persist for the whole
// execution, so a code left behind is still there when a second federated node runs, and
// HasRequiredInputs reads it as that node's own: the node skips its redirect and exchanges the first
// connection's code at the second connection's token endpoint. That second node is exactly what a
// linking node that sends a candidate to verification forwards to.
func (suite *OAuthExecutorTestSuite) TestExecute_ClearsCodeAndState_OnceConsumed() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeAuthentication,
		UserInputs: map[string]string{
			"code":  "auth_code_123",
			"state": "state-123",
		},
		NodeProperties: map[string]interface{}{
			"idpId": "idp-123",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyOAuthState: "state-123",
		},
	}

	authenticatedAuthUser := newOAuthAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authenticatedAuthUser, providers.AuthenticatedClaims{"sub": "sub-github"},
			(*tidcommon.ServiceError)(nil))
	expectEntityReferenceResolved(suite.mockAuthnProvider, authenticatedAuthUser)

	expectIdentityProviderResolved(suite.mockIDPService)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.NotContains(suite.T(), ctx.UserInputs, userInputCode,
		"the consumed code must not survive into the next federated node")
	assert.NotContains(suite.T(), ctx.UserInputs, userInputState,
		"the consumed state must not survive into the next federated node")
}

// An AuthUser with no entity-reference side names nobody. Resolving an entity reference from it is
// not something to ask for: the manager rejects the call as a fault and logs it as one.
func (suite *OAuthExecutorTestSuite) TestExecute_NoEntityReference_DoesNotResolveIt() {
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

	var authUser providers.AuthUser
	suite.Require().NoError(authUser.UnmarshalJSON([]byte(`{"default":{"attributeToken":"tok"}}`)))

	// GetEntityReference is deliberately not stubbed: the mock fails the test if it is called.
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).
		Return(authUser, providers.AuthenticatedClaims{"sub": "sub-github"},
			(*tidcommon.ServiceError)(nil))

	expectIdentityProviderResolved(suite.mockIDPService)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), entityStateNotExists, resp.RuntimeData[common.RuntimeKeyEntityState])
	suite.mockAuthnProvider.AssertNotCalled(suite.T(), "GetEntityReference", mock.Anything, mock.Anything)
}

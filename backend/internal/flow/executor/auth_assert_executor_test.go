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
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"

	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/attributecache"
	authnassert "github.com/thunder-id/thunderid/internal/authn/assert"
	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	oauth2const "github.com/thunder-id/thunderid/internal/oauth/oauth2/constants"
	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/attributecachemock"
	"github.com/thunder-id/thunderid/tests/mocks/authn/assertmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/jose/jwtmock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
	"github.com/thunder-id/thunderid/tests/mocks/rolemock"
)

const (
	testEmail      = "test@example.com"
	testNameValue  = "Test"
	testAuthOUID   = "ou-123"
	testAssertOUID = "ou-789"
)

type AuthAssertExecutorTestSuite struct {
	suite.Suite
	mockJWTService        *jwtmock.JWTServiceInterfaceMock
	mockOUService         *oumock.OrganizationUnitServiceInterfaceMock
	mockAssertGenerator   *assertmock.AuthAssertGeneratorInterfaceMock
	mockAuthnProvider     *managermock.AuthnProviderManagerInterfaceMock
	mockEntityProvider    *entityprovidermock.EntityProviderInterfaceMock
	mockFlowFactory       *coremock.FlowFactoryInterfaceMock
	mockAttributeCacheSvc *attributecachemock.AttributeCacheServiceInterfaceMock
	mockRoleService       *rolemock.RoleServiceInterfaceMock
	executor              *authAssertExecutor
}

func TestAuthAssertExecutorSuite(t *testing.T) {
	suite.Run(t, new(AuthAssertExecutorTestSuite))
}

func (suite *AuthAssertExecutorTestSuite) SetupTest() {
	// Initialize runtime for JWT config access
	_ = initializeTestRuntime()

	suite.mockJWTService = jwtmock.NewJWTServiceInterfaceMock(suite.T())
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.mockAssertGenerator = assertmock.NewAuthAssertGeneratorInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockAttributeCacheSvc = attributecachemock.NewAttributeCacheServiceInterfaceMock(suite.T())
	suite.mockRoleService = rolemock.NewRoleServiceInterfaceMock(suite.T())

	mockExec := createMockExecutorSimple(suite.T(), ExecutorNameAuthAssert, common.ExecutorTypeUtility)
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameAuthAssert, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{}).Return(mockExec)

	suite.executor = newAuthAssertExecutor(suite.mockFlowFactory, suite.mockJWTService,
		suite.mockOUService, suite.mockAssertGenerator, suite.mockAuthnProvider, suite.mockEntityProvider,
		suite.mockAttributeCacheSvc, suite.mockRoleService)
}

func createMockExecutorSimple(t *testing.T, name string,
	executorType common.ExecutorType) core.ExecutorInterface {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(name).Maybe()
	mockExec.On("GetType").Return(executorType).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{}).Maybe()
	return mockExec
}

func initializeTestRuntime() error {
	testConfig := &config.Config{
		JWT: config.JWTConfig{
			Issuer:         "https://auth.example.com",
			ValidityPeriod: 3600,
		},
	}
	return config.InitializeServerRuntime("/tmp/test", testConfig)
}

func (suite *AuthAssertExecutorTestSuite) TestNewAuthAssertExecutor() {
	assert.NotNil(suite.T(), suite.executor)
	assert.NotNil(suite.T(), suite.executor.jwtService)
	assert.NotNil(suite.T(), suite.executor.authnProvider)
	assert.NotNil(suite.T(), suite.executor.entityProvider)
	assert.NotNil(suite.T(), suite.executor.authAssertGenerator)
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_UserAuthenticated_Success() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			OUID:            testAuthOUID,
			UserType:        "INTERNAL",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{
			"node1": {
				ExecutorName: ExecutorNameBasicAuth,
				ExecutorType: common.ExecutorTypeAuthentication,
				Status:       common.FlowStatusComplete,
				Step:         1,
				EndTime:      1234567890,
			},
		},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"userType", "ouId"},
				},
			},
		},
	}

	suite.mockAssertGenerator.On(
		"GenerateAssertion",
		mock.Anything,
		mock.MatchedBy(func(refs []authncm.AuthenticatorReference) bool {
			return len(refs) == 1 && refs[0].Authenticator == authncm.AuthenticatorCredentials
		})).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, nil)

	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testAuthOUID).
		Return(ou.OrganizationUnit{ID: testAuthOUID}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "jwt-token", resp.Assertion)
	suite.mockAssertGenerator.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_UserNotAuthenticated() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: false,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrUserNotAuthenticated.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAuthorizedPermissions() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		RuntimeData: map[string]string{
			"authorized_permissions": "read:documents write:documents",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application:      appmodel.Application{},
	}

	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			perms, ok := claims["authorized_permissions"]
			return ok && perms == "read:documents write:documents"
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "jwt-token", resp.Assertion)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithUserAttributes() {
	attrs := map[string]interface{}{"email": testEmail, "phone": "1234567890"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			Attributes:      map[string]interface{}{"email": testEmail},
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"email", "phone"},
				},
			},
		},
	}

	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["email"] == testEmail && claims["phone"] == "1234567890"
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_JWTGenerationFails() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application:      appmodel.Application{},
	}

	suite.mockJWTService.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", int64(0), &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "JWT_GENERATION_FAILED",
		Error: i18ncore.I18nMessage{
			Key: "error.test.jwt_generation_failed", DefaultValue: "JWT generation failed",
		},
		ErrorDescription: i18ncore.I18nMessage{
			Key: "error.test.failed_to_generate_jwt_token", DefaultValue: "Failed to generate JWT token",
		},
	})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to generate JWT token")
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_AssertionGenerationFails_ServerError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{
			"node1": {
				ExecutorName: ExecutorNameBasicAuth,
				ExecutorType: common.ExecutorTypeAuthentication,
				Status:       common.FlowStatusComplete,
				Step:         1,
			},
		},
		Application: appmodel.Application{},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything, mock.Anything).
		Return(nil, &serviceerror.ServiceError{
			Type:  serviceerror.ServerErrorType,
			Error: i18ncore.I18nMessage{Key: "error.test.internal_error", DefaultValue: "internal error"},
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	suite.mockAssertGenerator.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExtractAuthenticatorReferences() {
	history := map[string]*common.NodeExecutionRecord{
		"node1": {
			ExecutorName: ExecutorNameBasicAuth,
			ExecutorType: common.ExecutorTypeAuthentication,
			Status:       common.FlowStatusComplete,
			Step:         3,
			EndTime:      1000,
		},
		"node2": {
			ExecutorName: ExecutorNameSMSAuth,
			ExecutorType: common.ExecutorTypeAuthentication,
			Status:       common.FlowStatusComplete,
			Step:         1,
			EndTime:      2000,
		},
		"node3": {
			ExecutorName: ExecutorNameProvisioning,
			ExecutorType: common.ExecutorTypeRegistration,
			Status:       common.FlowStatusComplete,
			Step:         2,
		},
		"node4": {
			ExecutorName: ExecutorNameOAuth,
			ExecutorType: common.ExecutorTypeAuthentication,
			Status:       common.FlowStatusError,
			Step:         4,
		},
	}

	refs := suite.executor.extractAuthenticatorReferences(history)

	assert.Len(suite.T(), refs, 2)
	assert.Equal(suite.T(), authncm.AuthenticatorSMSOTP, refs[0].Authenticator)
	assert.Equal(suite.T(), 1, refs[0].Step)
	assert.Equal(suite.T(), authncm.AuthenticatorCredentials, refs[1].Authenticator)
	assert.Equal(suite.T(), 2, refs[1].Step)
}

func (suite *AuthAssertExecutorTestSuite) TestExtractAuthenticatorReferences_EmptyHistory() {
	history := map[string]*common.NodeExecutionRecord{}

	refs := suite.executor.extractAuthenticatorReferences(history)

	assert.Empty(suite.T(), refs)
}

func (suite *AuthAssertExecutorTestSuite) TestExtractAuthenticatorReferences_UnknownExecutor() {
	history := map[string]*common.NodeExecutionRecord{
		"node1": {
			ExecutorName: "UnknownExecutor",
			ExecutorType: common.ExecutorTypeAuthentication,
			Status:       common.FlowStatusComplete,
			Step:         1,
		},
	}

	refs := suite.executor.extractAuthenticatorReferences(history)

	assert.Empty(suite.T(), refs)
}

func (suite *AuthAssertExecutorTestSuite) TestExtractAuthenticatorReferences_SMSOTPSendVerifyMode() {
	history := map[string]*common.NodeExecutionRecord{
		"sms_send_node": {
			ExecutorName: ExecutorNameSMSAuth,
			ExecutorType: common.ExecutorTypeAuthentication,
			ExecutorMode: "send",
			Status:       common.FlowStatusComplete,
			Step:         1,
			EndTime:      1000,
		},
		"sms_verify_node": {
			ExecutorName: ExecutorNameSMSAuth,
			ExecutorType: common.ExecutorTypeAuthentication,
			ExecutorMode: "verify",
			Status:       common.FlowStatusComplete,
			Step:         2,
			EndTime:      2000,
		},
	}

	refs := suite.executor.extractAuthenticatorReferences(history)

	// Should only have one SMS OTP authenticator, not two
	assert.Len(suite.T(), refs, 1)
	assert.Equal(suite.T(), authncm.AuthenticatorSMSOTP, refs[0].Authenticator)
	assert.Equal(suite.T(), 1, refs[0].Step)
}

func (suite *AuthAssertExecutorTestSuite) TestGetUserAttributesFromUserProvider_Success() {
	attrs := map[string]interface{}{"email": testEmail, "name": "Test User"}
	attrsJSON, _ := json.Marshal(attrs)

	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)

	resultAttrs, err := suite.executor.getUserAttributesFromUserProvider(context.Background(), "user-123")

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resultAttrs)
	assert.Equal(suite.T(), testEmail, resultAttrs["email"])
	assert.Equal(suite.T(), "Test User", resultAttrs["name"])
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestGetUserAttributesFromUserProvider_ServiceError() {
	suite.mockEntityProvider.On("GetEntity", "user-123").
		Return(nil, &entityprovider.EntityProviderError{Message: "user not found"})

	resultAttrs, err := suite.executor.getUserAttributesFromUserProvider(context.Background(), "user-123")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resultAttrs)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestGetUserAttributesFromUserProvider_InvalidJSON() {
	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: json.RawMessage(`invalid json`),
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)

	resultAttrs, err := suite.executor.getUserAttributesFromUserProvider(context.Background(), "user-123")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resultAttrs)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestGetUserAttributesFromAuthnProvider_Success() {
	reqAttrs := &authnprovidercm.RequestedAttributes{
		Attributes: map[string]*authnprovidercm.AttributeMetadataRequest{
			"email": nil,
			"name":  nil,
		},
		Verifications: nil,
	}

	res := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": {Value: testEmail},
			"name":  {Value: "Test User"},
		},
	}

	authUser := authnprovidermgr.AuthUser{}
	suite.mockAuthnProvider.
		On("GetUserAttributes", mock.Anything, reqAttrs, (*authnprovidercm.GetAttributesMetadata)(nil), authUser).
		Return(authnprovidermgr.AuthUser{}, res, nil)

	resultAttrs, err := suite.executor.getUserAttributesFromAuthnProvider(context.Background(),
		[]string{"email", "name"}, nil, authUser)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resultAttrs)
	assert.Equal(suite.T(), testEmail, resultAttrs["email"])
	assert.Equal(suite.T(), "Test User", resultAttrs["name"])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestGetUserAttributesFromAuthnProvider_ServiceError() {
	reqAttrs := &authnprovidercm.RequestedAttributes{
		Attributes: map[string]*authnprovidercm.AttributeMetadataRequest{
			"email": nil,
			"name":  nil,
		},
		Verifications: nil,
	}

	authUser := authnprovidermgr.AuthUser{}
	suite.mockAuthnProvider.
		On("GetUserAttributes", mock.Anything, reqAttrs, (*authnprovidercm.GetAttributesMetadata)(nil), authUser).
		Return(authnprovidermgr.AuthUser{}, (*authnprovidercm.AttributesResponse)(nil), &serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			Code: "ATTRIBUTES_FETCH_FAILED",
			Error: i18ncore.I18nMessage{
				Key: "error.test.failed_to_fetch_attributes", DefaultValue: "failed to fetch attributes",
			},
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.something_went_wrong", DefaultValue: "something went wrong",
			},
		})

	resultAttrs, err := suite.executor.getUserAttributesFromAuthnProvider(context.Background(),
		[]string{"email", "name"}, nil, authUser)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resultAttrs)
	assert.Contains(suite.T(), err.Error(), "something went wrong while fetching user attributes")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithUserTypeAndOU() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			UserType:        "EXTERNAL",
			OUID:            "ou-456",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"userType", "ouId"},
				},
			},
		},
	}

	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims[oauth2const.ClaimUserType] == "EXTERNAL" && claims[oauth2const.ClaimOUID] == "ou-456"
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou-456").
		Return(ou.OrganizationUnit{ID: "ou-456"}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithCustomTokenConfig() {
	// App-level assertion config (validity period only — issuer always comes from  config)
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					ValidityPeriod: 7200,
				},
			},
		},
	}

	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", "https://auth.example.com", int64(7200),
		mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(7200), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithOUNameAndHandle() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			OUID:            testAssertOUID,
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"ouId", "ouName", "ouHandle"},
				},
			},
		},
	}

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testAssertOUID).Return(ou.OrganizationUnit{
		ID:     testAssertOUID,
		Name:   "Engineering",
		Handle: "eng",
	}, nil)

	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims[oauth2const.ClaimOUID] == testAssertOUID &&
				claims[oauth2const.ClaimOUName] == "Engineering" &&
				claims[oauth2const.ClaimOUHandle] == "eng"
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "jwt-token", resp.Assertion)
	suite.mockOUService.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_AppendUserDetailsToClaimsFails() {
	attrs := map[string]interface{}{"email": testEmail}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"email"},
				},
			},
		},
	}

	// Test case 1: GetUser returns service error
	suite.mockEntityProvider.On("GetEntity", "user-123").
		Return(nil, &entityprovider.EntityProviderError{
			Message:     "user_not_found",
			Description: "user not found",
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "something went wrong while fetching user attributes")
	suite.mockEntityProvider.AssertExpectations(suite.T())

	// Reset mock for test case 2
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.executor.entityProvider = suite.mockEntityProvider

	// Test case 2: Invalid JSON in user attributes
	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: json.RawMessage(`{invalid json}`),
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)

	_, err = suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "something went wrong while unmarshalling user attributes")
	suite.mockEntityProvider.AssertExpectations(suite.T())

	// Test success case for comparison
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.executor.entityProvider = suite.mockEntityProvider

	existingUser.Attributes = attrsJSON
	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_AppendOUDetailsToClaimsFails() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			OUID:            testAuthOUID,
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{oauth2const.ClaimOUID},
				},
			},
		},
	}

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testAuthOUID).
		Return(ou.OrganizationUnit{}, &serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{Key: "error.test.ou_not_found", DefaultValue: "ou_not_found"},
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.organization_unit_not_found", DefaultValue: "organization unit not found",
			},
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "something went wrong while fetching organization unit")
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestAppendUserDetailsToClaims_GetUserAttributesFails() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			Attributes:      map[string]interface{}{"email": testEmail},
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"email", "phone"},
				},
			},
		},
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").
		Return(nil, &entityprovider.EntityProviderError{
			Message:     "database_error",
			Description: "failed to fetch user",
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "something went wrong while fetching user attributes")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestAppendOUDetailsToClaims_GetOrganizationUnitFails() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			OUID:            "ou-invalid",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{oauth2const.ClaimOUID},
				},
			},
		},
	}

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou-invalid").
		Return(ou.OrganizationUnit{}, &serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{Key: "error.test.ou_not_found", DefaultValue: "ou_not_found"},
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.organization_unit_does_not_exist", DefaultValue: "organization unit does not exist",
			},
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "something went wrong while fetching organization unit")
	assert.Contains(suite.T(), err.Error(), "organization unit does not exist")
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithConfiguredUserAttributes() {
	attrs := map[string]interface{}{"email": testEmail, "username": "testuser", "given_name": testNameValue}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			// Token config with user attributes configured
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"email", "username", "given_name"},
				},
			},
		},
	}

	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			// Should contain the configured user attributes from the user store
			hasEmail := claims["email"] == testEmail
			hasUsername := claims["username"] == "testuser"
			hasFirstName := claims["given_name"] == testNameValue
			return hasEmail && hasUsername && hasFirstName
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithGroups() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{oauth2const.UserAttributeGroups},
				},
			},
		},
	}

	userGroups := []entityprovider.EntityGroup{
		{Name: "admin"},
		{Name: "developer"},
		{Name: "viewer"},
	}

	suite.mockEntityProvider.On("GetTransitiveEntityGroups", "user-123").
		Return(userGroups, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			// Should contain groups claim
			groups, ok := claims[oauth2const.UserAttributeGroups].([]string)
			if !ok {
				return false
			}
			return len(groups) == 3 && groups[0] == "admin" && groups[1] == "developer" && groups[2] == "viewer"
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithGroups_EmptyGroups() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{oauth2const.UserAttributeGroups},
				},
			},
		},
	}

	suite.mockEntityProvider.On("GetTransitiveEntityGroups", "user-123").
		Return([]entityprovider.EntityGroup{}, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			// Should NOT contain groups claim when groups list is empty
			_, ok := claims[oauth2const.UserAttributeGroups]
			return !ok
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithGroups_GetUserGroupsFails() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{oauth2const.UserAttributeGroups},
				},
			},
		},
	}

	suite.mockEntityProvider.On("GetTransitiveEntityGroups", "user-123").Return(
		nil, &entityprovider.EntityProviderError{
			Message: "failed to fetch groups", Description: "database error",
		})

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
	assert.Contains(suite.T(), err.Error(), "something went wrong while fetching user groups")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestGetRequiredUserAttributes_ConsentRecordedWithoutConsentedKey() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			common.RuntimeKeyConsentID: "consent-123",
		},
	}

	result := suite.executor.getRequiredUserAttributes(ctx)

	assert.Empty(suite.T(), result)
}

func (suite *AuthAssertExecutorTestSuite) TestGetRequiredUserAttributes_ConsentRecordedWithConsentedKey() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			common.RuntimeKeyConsentID:           "consent-123",
			common.RuntimeKeyConsentedAttributes: "email name",
		},
	}

	result := suite.executor.getRequiredUserAttributes(ctx)

	assert.Equal(suite.T(), []string{"email", "name"}, result)
}

func (suite *AuthAssertExecutorTestSuite) TestGetRequiredUserAttributes_RuntimeEssentialOnly() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			common.RuntimeKeyRequiredEssentialAttributes: "email name",
		},
	}

	result := suite.executor.getRequiredUserAttributes(ctx)

	assert.Equal(suite.T(), []string{"email", "name"}, result)
}

func (suite *AuthAssertExecutorTestSuite) TestGetRequiredUserAttributes_RuntimeOptionalOnly() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			common.RuntimeKeyRequiredOptionalAttributes: "email phone",
		},
	}

	result := suite.executor.getRequiredUserAttributes(ctx)

	assert.Equal(suite.T(), []string{"email", "phone"}, result)
}

func (suite *AuthAssertExecutorTestSuite) TestGetRequiredUserAttributes_RuntimeEssentialAndOptional() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{
			common.RuntimeKeyRequiredEssentialAttributes: "email",
			common.RuntimeKeyRequiredOptionalAttributes:  "phone name",
		},
	}

	result := suite.executor.getRequiredUserAttributes(ctx)

	assert.Equal(suite.T(), []string{"email", "phone", "name"}, result)
}

func (suite *AuthAssertExecutorTestSuite) TestGetRequiredUserAttributes_FallbackToAssertion() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{UserAttributes: []string{"email", "phone"}},
			},
		},
	}

	result := suite.executor.getRequiredUserAttributes(ctx)

	assert.Equal(suite.T(), []string{"email", "phone"}, result)
}

func (suite *AuthAssertExecutorTestSuite) TestGetRequiredUserAttributes_NoRuntimeOrAssertion() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		RuntimeData: map[string]string{},
		Application: appmodel.Application{},
	}

	result := suite.executor.getRequiredUserAttributes(ctx)

	assert.Empty(suite.T(), result)
}

// ----- Execute with Consented Attributes in RuntimeData -----

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithConsentedAttributes_FiltersUserAttrs() {
	attrs := map[string]interface{}{"email": testEmail, "phone": "1234567890", "name": testNameValue}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyConsentID:           "consent-123",
			common.RuntimeKeyConsentedAttributes: "email name",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"email", "phone", "name"},
				},
			},
		},
	}

	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			// Should only have email and name (consented), NOT phone
			_, hasPhone := claims["phone"]
			hasEmail := claims["email"] == testEmail
			hasName := claims["name"] == testNameValue
			return hasEmail && hasName && !hasPhone
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithEmptyConsentedAttributes() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyConsentID:           "consent-456",
			common.RuntimeKeyConsentedAttributes: "", // Consent ran but no attrs approved
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application:      appmodel.Application{},
	}

	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithoutConsentedAttributes() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application:      appmodel.Application{},
	}

	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

// ----- Execute with Attribute Cache TTL in RuntimeData -----

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_AttrsStoredInCacheNotJWT() {
	attrs := map[string]interface{}{"email": testEmail, "phone": "1234567890"}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"email", "phone"},
				},
			},
		},
	}

	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything,
		mock.MatchedBy(func(cache *attributecache.AttributeCache) bool {
			return cache.TTLSeconds == 300 &&
				cache.Attributes["email"] == testEmail &&
				cache.Attributes["phone"] == "1234567890"
		})).Return(&attributecache.AttributeCache{ID: "cache-abc"}, nil)
	// In the OAuth cache path, only aci goes into the JWT; individual attrs go to cache.
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasEmail := claims["email"]
			_, hasPhone := claims["phone"]
			return claims["aci"] == "cache-abc" && !hasEmail && !hasPhone
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_NilUserAttributes_NoAttrsCopied() {
	// Use runtime essential attributes so resolvedAttributes is non-empty and cache is created,
	// but Assertion.UserAttributes is nil so no individual attrs should be copied to JWT.
	attrs := map[string]interface{}{"email": testEmail}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
			common.RuntimeKeyRequiredEssentialAttributes:   "email",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: nil,
				},
			},
		},
	}

	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything, mock.Anything).
		Return(&attributecache.AttributeCache{ID: "cache-xyz"}, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			// aci present, but no individual attribute claims
			_, hasEmail := claims["email"]
			return claims["aci"] == "cache-xyz" && !hasEmail
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_OnlyResolvedAttrsStoredInCache() {
	// resolved attributes only contain "email"; "phone" is configured but not found in user store
	attrs := map[string]interface{}{"email": testEmail}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "600",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"email", "phone"},
				},
			},
		},
	}

	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)
	// Cache should only contain resolved attrs (email, not phone)
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything,
		mock.MatchedBy(func(cache *attributecache.AttributeCache) bool {
			_, hasPhone := cache.Attributes["phone"]
			return cache.Attributes["email"] == testEmail && !hasPhone
		})).Return(&attributecache.AttributeCache{ID: "cache-def"}, nil)
	// JWT should only contain aci, not individual attrs
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasEmail := claims["email"]
			_, hasPhone := claims["phone"]
			return claims["aci"] == "cache-def" && !hasEmail && !hasPhone
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_NilAssertion_NoAttrsCopied() {
	// Use runtime essential attributes so resolvedAttributes is non-empty and cache is created,
	// but Assertion is nil so no individual attrs should be copied to JWT.
	attrs := map[string]interface{}{"email": testEmail}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
			common.RuntimeKeyRequiredEssentialAttributes:   "email",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: nil,
			},
		},
	}

	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything, mock.Anything).
		Return(&attributecache.AttributeCache{ID: "cache-nil"}, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasEmail := claims["email"]
			return claims["aci"] == "cache-nil" && !hasEmail
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ----- resolveUserAttributes: groups, userType, OU handling -----

func (suite *AuthAssertExecutorTestSuite) TestResolveUserAttributes_WithGroups() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "user-123",
		},
		RuntimeData: map[string]string{},
	}

	userGroups := []entityprovider.EntityGroup{
		{Name: "admin"},
		{Name: "developer"},
	}

	suite.mockEntityProvider.On("GetTransitiveEntityGroups", "user-123").
		Return(userGroups, nil)

	attrs, err := suite.executor.resolveUserAttributes(ctx, []string{oauth2const.UserAttributeGroups})

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), attrs)
	groups, ok := attrs[oauth2const.UserAttributeGroups].([]string)
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), []string{"admin", "developer"}, groups)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestResolveUserAttributes_WithGroups_FetchError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "user-123",
		},
		RuntimeData: map[string]string{},
	}

	suite.mockEntityProvider.On("GetTransitiveEntityGroups", "user-123").
		Return(nil, &entityprovider.EntityProviderError{Message: "groups_fetch_failed", Description: "database error"})

	attrs, err := suite.executor.resolveUserAttributes(ctx, []string{oauth2const.UserAttributeGroups})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), attrs)
	assert.Contains(suite.T(), err.Error(), "something went wrong while fetching user groups")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestResolveUserAttributes_WithGroups_EmptyUserID_GroupsSkipped() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "",
		},
		RuntimeData: map[string]string{},
	}

	attrs, err := suite.executor.resolveUserAttributes(ctx, []string{oauth2const.UserAttributeGroups})

	assert.NoError(suite.T(), err)
	// Groups attribute should not be present when UserID is empty
	_, hasGroups := attrs[oauth2const.UserAttributeGroups]
	assert.False(suite.T(), hasGroups)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "GetTransitiveEntityGroups")
}

func (suite *AuthAssertExecutorTestSuite) TestResolveUserAttributes_WithUserType() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID:   "user-123",
			UserType: "INTERNAL",
		},
		RuntimeData: map[string]string{},
	}

	attrs, err := suite.executor.resolveUserAttributes(ctx, []string{oauth2const.ClaimUserType})

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), attrs)
	assert.Equal(suite.T(), "INTERNAL", attrs[oauth2const.ClaimUserType])
}

func (suite *AuthAssertExecutorTestSuite) TestResolveUserAttributes_WithEmptyUserType_NotAdded() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID:   "user-123",
			UserType: "",
		},
		RuntimeData: map[string]string{},
	}

	attrs, err := suite.executor.resolveUserAttributes(ctx, []string{oauth2const.ClaimUserType})

	assert.NoError(suite.T(), err)
	_, hasUserType := attrs[oauth2const.ClaimUserType]
	assert.False(suite.T(), hasUserType)
}

func (suite *AuthAssertExecutorTestSuite) TestResolveUserAttributes_WithOUDetails() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "user-123",
			OUID:   testAuthOUID,
		},
		RuntimeData: map[string]string{},
	}

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testAuthOUID).
		Return(ou.OrganizationUnit{ID: testAuthOUID, Name: "Engineering", Handle: "eng"}, nil)

	attrs, err := suite.executor.resolveUserAttributes(ctx,
		[]string{oauth2const.ClaimOUID, oauth2const.ClaimOUName, oauth2const.ClaimOUHandle})

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), attrs)
	assert.Equal(suite.T(), testAuthOUID, attrs[oauth2const.ClaimOUID])
	assert.Equal(suite.T(), "Engineering", attrs[oauth2const.ClaimOUName])
	assert.Equal(suite.T(), "eng", attrs[oauth2const.ClaimOUHandle])
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestResolveUserAttributes_WithOUDetails_FetchError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "user-123",
			OUID:   "ou-invalid",
		},
		RuntimeData: map[string]string{},
	}

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, "ou-invalid").
		Return(ou.OrganizationUnit{}, &serviceerror.ServiceError{
			Error: i18ncore.I18nMessage{Key: "error.test.ou_not_found", DefaultValue: "ou_not_found"},
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.organization_unit_not_found", DefaultValue: "organization unit not found",
			},
		})

	attrs, err := suite.executor.resolveUserAttributes(ctx, []string{oauth2const.ClaimOUID})

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), attrs)
	assert.Contains(suite.T(), err.Error(), "something went wrong while fetching organization unit")
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestResolveUserAttributes_WithOUDetails_EmptyOUID_OUDetailsSkipped() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			UserID: "user-123",
			OUID:   "",
		},
		RuntimeData: map[string]string{},
	}

	attrs, err := suite.executor.resolveUserAttributes(ctx, []string{oauth2const.ClaimOUID})

	assert.NoError(suite.T(), err)
	_, hasOUID := attrs[oauth2const.ClaimOUID]
	assert.False(suite.T(), hasOUID)
	suite.mockOUService.AssertNotCalled(suite.T(), "GetOrganizationUnit")
}

// ----- Execute with Attribute Cache: groups/userType/OU now go into cache -----

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_GroupsIncludedInCache() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{oauth2const.UserAttributeGroups},
				},
			},
		},
	}

	userGroups := []entityprovider.EntityGroup{
		{Name: "admin"},
		{Name: "developer"},
	}

	suite.mockEntityProvider.On("GetTransitiveEntityGroups", "user-123").
		Return(userGroups, nil)
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything,
		mock.MatchedBy(func(cache *attributecache.AttributeCache) bool {
			groups, ok := cache.Attributes[oauth2const.UserAttributeGroups].([]string)
			return ok && len(groups) == 2
		})).Return(&attributecache.AttributeCache{ID: "cache-groups"}, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasGroups := claims[oauth2const.UserAttributeGroups]
			return claims["aci"] == "cache-groups" && !hasGroups
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_UserTypeIncludedInCache() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			UserType:        "EXTERNAL",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{oauth2const.ClaimUserType},
				},
			},
		},
	}

	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything,
		mock.MatchedBy(func(cache *attributecache.AttributeCache) bool {
			return cache.Attributes[oauth2const.ClaimUserType] == "EXTERNAL"
		})).Return(&attributecache.AttributeCache{ID: "cache-usertype"}, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasUserType := claims[oauth2const.ClaimUserType]
			return claims["aci"] == "cache-usertype" && !hasUserType
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_OUDetailsIncludedInCache() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		Context:     context.Background(),
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
			OUID:            testAuthOUID,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{oauth2const.ClaimOUID, oauth2const.ClaimOUName},
				},
			},
		},
	}

	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testAuthOUID).
		Return(ou.OrganizationUnit{ID: testAuthOUID, Name: "Engineering"}, nil)
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything,
		mock.MatchedBy(func(cache *attributecache.AttributeCache) bool {
			return cache.Attributes[oauth2const.ClaimOUID] == testAuthOUID &&
				cache.Attributes[oauth2const.ClaimOUName] == "Engineering"
		})).Return(&attributecache.AttributeCache{ID: "cache-ou"}, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasOUID := claims[oauth2const.ClaimOUID]
			return claims["aci"] == "cache-ou" && !hasOUID
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockOUService.AssertExpectations(suite.T())
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithRuntimeRequiredEssentialAndOptionalAttributes() {
	attrs := map[string]interface{}{"email": testEmail, "phone": "1234567890", "name": testNameValue}
	attrsJSON, _ := json.Marshal(attrs)

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		EntityID:    "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-123",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyRequiredEssentialAttributes: "email",
			common.RuntimeKeyRequiredOptionalAttributes:  "name",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{UserAttributes: []string{"email", "phone", "name"}},
			},
		},
	}

	existingUser := &entityprovider.Entity{
		ID:         "user-123",
		Attributes: attrsJSON,
	}

	suite.mockEntityProvider.On("GetEntity", "user-123").Return(existingUser, nil)
	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasPhone := claims["phone"]
			return claims["email"] == testEmail && claims["name"] == testNameValue && !hasPhone
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

// ----- resolvePermissionsForClaim -----

func (suite *AuthAssertExecutorTestSuite) TestResolvePermissionsForClaim_PrefersConsented() {
	ctx := &core.NodeContext{RuntimeData: map[string]string{
		common.RuntimeKeyConsentedPermissions: "booking:read",
		"authorized_permissions":              "booking:read booking:write",
		common.RuntimeKeyRequestedPermissions: "booking:read booking:write booking:cancel",
	}}
	assert.Equal(suite.T(), "booking:read", (&authAssertExecutor{}).resolvePermissionsForClaim(ctx))
}

func (suite *AuthAssertExecutorTestSuite) TestResolvePermissionsForClaim_ConsentedEmptyStillPreferredOverAuthorized() {
	// Consent step ran but the user denied every permission. The empty value must be used so
	// the JWT does not leak authorized but not consented permissions.
	ctx := &core.NodeContext{RuntimeData: map[string]string{
		common.RuntimeKeyConsentedPermissions: "",
		"authorized_permissions":              "booking:read",
	}}
	assert.Equal(suite.T(), "", (&authAssertExecutor{}).resolvePermissionsForClaim(ctx))
}

func (suite *AuthAssertExecutorTestSuite) TestResolvePermissionsForClaim_FallsBackToAuthorized() {
	ctx := &core.NodeContext{RuntimeData: map[string]string{
		"authorized_permissions":              "booking:read",
		common.RuntimeKeyRequestedPermissions: "booking:read booking:write",
	}}
	assert.Equal(suite.T(), "booking:read", (&authAssertExecutor{}).resolvePermissionsForClaim(ctx))
}

func (suite *AuthAssertExecutorTestSuite) TestResolvePermissionsForClaim_RequestedAloneNeverLeaksToClaim() {
	// Raw requested permissions must NEVER end up in the JWT claim without going through
	// the authz executor. Regression: when a user has no authorized permissions, an empty
	// claim must be emitted so token endpoint clears PermissionScopes correctly.
	ctx := &core.NodeContext{RuntimeData: map[string]string{
		common.RuntimeKeyRequestedPermissions: "booking:read booking:write",
	}}
	assert.Equal(suite.T(), "", (&authAssertExecutor{}).resolvePermissionsForClaim(ctx))
}

func (suite *AuthAssertExecutorTestSuite) TestResolvePermissionsForClaim_NoKeysReturnsEmpty() {
	ctx := &core.NodeContext{RuntimeData: map[string]string{}}
	assert.Equal(suite.T(), "", (&authAssertExecutor{}).resolvePermissionsForClaim(ctx))
}

func (suite *AuthAssertExecutorTestSuite) TestResolvePermissionsForClaim_IntersectsConsentedWithAuthorized() {
	// Stale-permission scenario: the consent record has a permission ("write") the user no
	// longer holds in this session. The intersection must drop it from the JWT.
	ctx := &core.NodeContext{RuntimeData: map[string]string{
		common.RuntimeKeyConsentedPermissions: "read write cancel",
		"authorized_permissions":              "read cancel",
	}}
	got := (&authAssertExecutor{}).resolvePermissionsForClaim(ctx)
	assert.Equal(suite.T(), "read cancel", got)
}

func (suite *AuthAssertExecutorTestSuite) TestIntersectPermissionSpaceList_EmptyInputs() {
	assert.Equal(suite.T(), "", intersectPermissionSpaceList("", "a b"))
	assert.Equal(suite.T(), "", intersectPermissionSpaceList("a b", ""))
	assert.Equal(suite.T(), "", intersectPermissionSpaceList("", ""))
}

func (suite *AuthAssertExecutorTestSuite) TestIntersectPermissionSpaceList_PreservesOrderOfFirstArg() {
	assert.Equal(suite.T(), "c a", intersectPermissionSpaceList("c a b", "a c"))
}

func (suite *AuthAssertExecutorTestSuite) TestIntersectPermissionSpaceList_DropsDuplicates() {
	// Defensive dedup: if `a` has duplicates, each token appears at most once in the result.
	assert.Equal(suite.T(), "x y", intersectPermissionSpaceList("x y x y", "x y"))
}

func (suite *AuthAssertExecutorTestSuite) TestIntersectPermissionSpaceList_NoOverlap() {
	assert.Equal(suite.T(), "", intersectPermissionSpaceList("a b", "c d"))
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_CallbackType_EmittedWhenSet() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-ciba",
		EntityID:    "app-1",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-1",
		},
		NodeProperties: map[string]interface{}{
			propertyKeyCallbackType: "urn:openid:params:grant-type:ciba",
		},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
	}

	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-1", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "jwt-token", resp.Assertion)
	assert.Equal(suite.T(), "urn:openid:params:grant-type:ciba", resp.AdditionalData[propertyKeyCallbackType])
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_CallbackType_AbsentWhenNotSet() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-authcode",
		EntityID:    "app-1",
		FlowType:    common.FlowTypeAuthentication,
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "user-1",
		},
		NodeProperties:   map[string]interface{}{},
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
	}

	suite.mockJWTService.On("GenerateJWT", mock.Anything, "user-1", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	_, hasCallbackType := resp.AdditionalData[propertyKeyCallbackType]
	assert.False(suite.T(), hasCallbackType, "callbackType must not be present for auth code flows")
}

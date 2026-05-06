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

	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appmodel "github.com/asgardeo/thunder/internal/application/model"
	"github.com/asgardeo/thunder/internal/attributecache"
	authnassert "github.com/asgardeo/thunder/internal/authn/assert"
	authnprovidercm "github.com/asgardeo/thunder/internal/authnprovider/common"
	authnprovidermgr "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/entityprovider"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/flow/core"
	inboundmodel "github.com/asgardeo/thunder/internal/inboundclient/model"
	oauth2const "github.com/asgardeo/thunder/internal/oauth/oauth2/constants"
	"github.com/asgardeo/thunder/internal/ou"
	"github.com/asgardeo/thunder/internal/system/config"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	"github.com/asgardeo/thunder/tests/mocks/attributecachemock"
	"github.com/asgardeo/thunder/tests/mocks/authn/assertmock"
	"github.com/asgardeo/thunder/tests/mocks/authnprovider/managermock"
	"github.com/asgardeo/thunder/tests/mocks/entityprovidermock"
	"github.com/asgardeo/thunder/tests/mocks/flow/coremock"
	"github.com/asgardeo/thunder/tests/mocks/jose/jwtmock"
	"github.com/asgardeo/thunder/tests/mocks/oumock"
	"github.com/asgardeo/thunder/tests/mocks/rolemock"
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

// mustAssertAuthUser builds an AuthUser from the given fields for use in auth assert tests.
func mustAssertAuthUser(userID, userType, ouID string) authnprovidermgr.AuthUser { //nolint:unparam
	userEntry := map[string]interface{}{"isValuesIncluded": true}
	if userID != "" {
		userEntry["userId"] = userID
	}
	if userType != "" {
		userEntry["userType"] = userType
	}
	if ouID != "" {
		userEntry["ouId"] = ouID
	}
	m := map[string]interface{}{
		"authHistory": []map[string]interface{}{
			{"authType": authnprovidercm.AuthenticatorCredentials, "isVerified": true},
		},
		"userHistory": []map[string]interface{}{userEntry},
		"userState":   "exists",
	}
	b, _ := json.Marshal(m)
	var au authnprovidermgr.AuthUser
	_ = json.Unmarshal(b, &au)
	return au
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
		AppID:       "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    mustAssertAuthUser("user-123", "INTERNAL", testAuthOUID),
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
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"userType", "ouId"},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion",
		mock.MatchedBy(func(refs []authnprovidermgr.AuthenticatorReference) bool {
			return len(refs) == 1 && refs[0].Authenticator == authnprovidercm.AuthenticatorCredentials
		})).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, nil)

	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), failureReasonUserNotAuthenticated, resp.FailureReason)
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAuthorizedPermissions() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		AppID:       "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
		RuntimeData: map[string]string{
			"authorized_permissions": "read:documents write:documents",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application:      appmodel.Application{},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
	ctx := &core.NodeContext{
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", ""),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"email", "phone"},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidercm.AttributesResponse{
			Attributes: map[string]*authnprovidercm.AttributeResponse{
				"email": {Value: testEmail},
				"phone": {Value: "1234567890"},
			},
		}, (*serviceerror.ServiceError)(nil)).Once()
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			return claims["email"] == testEmail && claims["phone"] == "1234567890"
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_JWTGenerationFails() {
	ctx := &core.NodeContext{
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", ""),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application:      appmodel.Application{},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockJWTService.On("GenerateJWT", mock.Anything, mock.Anything,
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
		AppID:       "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
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

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).
		Return(nil, &serviceerror.ServiceError{
			Type:  serviceerror.ServerErrorType,
			Error: i18ncore.I18nMessage{Key: "error.test.internal_error", DefaultValue: "internal error"},
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	suite.mockAssertGenerator.AssertExpectations(suite.T())
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
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "EXTERNAL", "ou-456"),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"userType", "ouId"},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", ""),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				ValidityPeriod: 7200,
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockJWTService.On("GenerateJWT", "user-123", "https://auth.example.com", int64(7200),
		mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(7200), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithOUNameAndHandle() {
	ctx := &core.NodeContext{
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", testAssertOUID),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"ouId", "ouName", "ouHandle"},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testAssertOUID).Return(ou.OrganizationUnit{
		ID:     testAssertOUID,
		Name:   "Engineering",
		Handle: "eng",
	}, nil)

	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
	ctx := &core.NodeContext{
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", ""),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"email"},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))

	// Test case 1: GetUserAttributes returns server error
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, (*authnprovidercm.AttributesResponse)(nil), &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "failed to fetch user attributes"},
		}).Once()

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "something went wrong while fetching user attributes")

	// Test case 2: GetUserAttributes returns a non-server error
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, (*authnprovidercm.AttributesResponse)(nil), &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "attribute fetch rejected"},
		}).Once()

	_, err = suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to fetch user attributes")

	// Test success case for comparison
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidercm.AttributesResponse{
			Attributes: map[string]*authnprovidercm.AttributeResponse{
				"email": {Value: testEmail},
			},
		}, (*serviceerror.ServiceError)(nil)).Once()
	suite.mockJWTService.On("GenerateJWT", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_AppendOUDetailsToClaimsFails() {
	ctx := &core.NodeContext{
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", testAuthOUID),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{oauth2const.ClaimOUID},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
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
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", ""),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"email", "phone"},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, (*authnprovidercm.AttributesResponse)(nil), &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "failed to fetch user attributes"},
		}).Once()

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "something went wrong while fetching user attributes")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestAppendOUDetailsToClaims_GetOrganizationUnitFails() {
	ctx := &core.NodeContext{
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", "ou-invalid"),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{oauth2const.ClaimOUID},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
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
	ctx := &core.NodeContext{
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", ""),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			// Token config with user attributes configured
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"email", "username", "given_name"},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidercm.AttributesResponse{
			Attributes: map[string]*authnprovidercm.AttributeResponse{
				"email":      {Value: testEmail},
				"username":   {Value: "testuser"},
				"given_name": {Value: testNameValue},
			},
		}, (*serviceerror.ServiceError)(nil)).Once()
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
	suite.mockAuthnProvider.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithGroups() {
	ctx := &core.NodeContext{
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", ""),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{oauth2const.UserAttributeGroups},
			},
		},
	}

	userGroups := []entityprovider.EntityGroup{
		{Name: "admin"},
		{Name: "developer"},
		{Name: "viewer"},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockEntityProvider.On("GetTransitiveEntityGroups", "user-123").
		Return(userGroups, nil)
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", ""),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{oauth2const.UserAttributeGroups},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockEntityProvider.On("GetTransitiveEntityGroups", "user-123").
		Return([]entityprovider.EntityGroup{}, nil)
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", ""),
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{oauth2const.UserAttributeGroups},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
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
			Assertion: &inboundmodel.AssertionConfig{UserAttributes: []string{"email", "phone"}},
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
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		AppID:       "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
		RuntimeData: map[string]string{
			common.RuntimeKeyConsentID:           "consent-123",
			common.RuntimeKeyConsentedAttributes: "email name",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"email", "phone", "name"},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidercm.AttributesResponse{
			Attributes: map[string]*authnprovidercm.AttributeResponse{
				"email": {Value: testEmail},
				"name":  {Value: testNameValue},
			},
		}, (*serviceerror.ServiceError)(nil)).Once()
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
	suite.mockAuthnProvider.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithEmptyConsentedAttributes() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		AppID:       "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
		RuntimeData: map[string]string{
			common.RuntimeKeyConsentID:           "consent-456",
			common.RuntimeKeyConsentedAttributes: "", // Consent ran but no attrs approved
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application:      appmodel.Application{},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithoutConsentedAttributes() {
	ctx := &core.NodeContext{
		ExecutionID:      "flow-123",
		AppID:            "app-123",
		FlowType:         common.FlowTypeAuthentication,
		AuthUser:         mustAssertAuthUser("user-123", "", ""),
		RuntimeData:      map[string]string{},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application:      appmodel.Application{},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

// ----- Execute with Attribute Cache TTL in RuntimeData -----

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_AttrsStoredInCacheNotJWT() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		AppID:       "app-123",
		Context:     context.Background(),
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"email", "phone"},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidercm.AttributesResponse{
			Attributes: map[string]*authnprovidercm.AttributeResponse{
				"email": {Value: testEmail},
				"phone": {Value: "1234567890"},
			},
		}, (*serviceerror.ServiceError)(nil)).Once()
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything,
		mock.MatchedBy(func(cache *attributecache.AttributeCache) bool {
			return cache.TTLSeconds == 300 &&
				cache.Attributes["email"] == testEmail &&
				cache.Attributes["phone"] == "1234567890"
		})).Return(&attributecache.AttributeCache{ID: "cache-abc"}, nil)
	// In the OAuth cache path, only aci goes into the JWT; individual attrs go to cache.
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasEmail := claims["email"]
			_, hasPhone := claims["phone"]
			return claims["aci"] == "cache-abc" && !hasEmail && !hasPhone
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_NilUserAttributes_NoAttrsCopied() {
	// Use runtime essential attributes so resolvedAttributes is non-empty and cache is created,
	// but Assertion.UserAttributes is nil so no individual attrs should be copied to JWT.
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		AppID:       "app-123",
		Context:     context.Background(),
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
			common.RuntimeKeyRequiredEssentialAttributes:   "email",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: nil,
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidercm.AttributesResponse{
			Attributes: map[string]*authnprovidercm.AttributeResponse{
				"email": {Value: testEmail},
			},
		}, (*serviceerror.ServiceError)(nil)).Once()
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything, mock.Anything).
		Return(&attributecache.AttributeCache{ID: "cache-xyz"}, nil)
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			// aci present, but no individual attribute claims
			_, hasEmail := claims["email"]
			return claims["aci"] == "cache-xyz" && !hasEmail
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_OnlyResolvedAttrsStoredInCache() {
	// resolved attributes only contain "email"; "phone" is configured but not returned by authn provider
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		AppID:       "app-123",
		Context:     context.Background(),
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "600",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"email", "phone"},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidercm.AttributesResponse{
			Attributes: map[string]*authnprovidercm.AttributeResponse{
				"email": {Value: testEmail},
			},
		}, (*serviceerror.ServiceError)(nil)).Once()
	// Cache should only contain resolved attrs (email, not phone)
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything,
		mock.MatchedBy(func(cache *attributecache.AttributeCache) bool {
			_, hasPhone := cache.Attributes["phone"]
			return cache.Attributes["email"] == testEmail && !hasPhone
		})).Return(&attributecache.AttributeCache{ID: "cache-def"}, nil)
	// JWT should only contain aci, not individual attrs
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasEmail := claims["email"]
			_, hasPhone := claims["phone"]
			return claims["aci"] == "cache-def" && !hasEmail && !hasPhone
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

func (suite *AuthAssertExecutorTestSuite) TestExecute_WithAttributeCache_NilAssertion_NoAttrsCopied() {
	// Use runtime essential attributes so resolvedAttributes is non-empty and cache is created,
	// but Assertion is nil so no individual attrs should be copied to JWT.
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		AppID:       "app-123",
		Context:     context.Background(),
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
			common.RuntimeKeyRequiredEssentialAttributes:   "email",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: nil,
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidercm.AttributesResponse{
			Attributes: map[string]*authnprovidercm.AttributeResponse{
				"email": {Value: testEmail},
			},
		}, (*serviceerror.ServiceError)(nil)).Once()
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything, mock.Anything).
		Return(&attributecache.AttributeCache{ID: "cache-nil"}, nil)
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasEmail := claims["email"]
			return claims["aci"] == "cache-nil" && !hasEmail
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
	suite.mockAttributeCacheSvc.AssertExpectations(suite.T())
	suite.mockJWTService.AssertExpectations(suite.T())
}

// ----- resolveUserAttributes: groups, userType, OU handling -----

func (suite *AuthAssertExecutorTestSuite) TestResolveUserAttributes_WithGroups() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		Context:     context.Background(),
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
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
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
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
		AuthUser:    mustAssertAuthUser("user-123", "INTERNAL", ""),
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
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
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
		AuthUser:    mustAssertAuthUser("user-123", "", testAuthOUID),
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
		AuthUser:    mustAssertAuthUser("user-123", "", "ou-invalid"),
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
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
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
		AppID:       "app-123",
		Context:     context.Background(),
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{oauth2const.UserAttributeGroups},
			},
		},
	}

	userGroups := []entityprovider.EntityGroup{
		{Name: "admin"},
		{Name: "developer"},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockEntityProvider.On("GetTransitiveEntityGroups", "user-123").
		Return(userGroups, nil)
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything,
		mock.MatchedBy(func(cache *attributecache.AttributeCache) bool {
			groups, ok := cache.Attributes[oauth2const.UserAttributeGroups].([]string)
			return ok && len(groups) == 2
		})).Return(&attributecache.AttributeCache{ID: "cache-groups"}, nil)
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
		AppID:       "app-123",
		Context:     context.Background(),
		AuthUser:    mustAssertAuthUser("user-123", "EXTERNAL", ""),
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{oauth2const.ClaimUserType},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything,
		mock.MatchedBy(func(cache *attributecache.AttributeCache) bool {
			return cache.Attributes[oauth2const.ClaimUserType] == "EXTERNAL"
		})).Return(&attributecache.AttributeCache{ID: "cache-usertype"}, nil)
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
		AppID:       "app-123",
		Context:     context.Background(),
		AuthUser:    mustAssertAuthUser("user-123", "", testAuthOUID),
		RuntimeData: map[string]string{
			common.RuntimeKeyUserAttributesCacheTTLSeconds: "300",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{oauth2const.ClaimOUID, oauth2const.ClaimOUName},
			},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockOUService.On("GetOrganizationUnit", mock.Anything, testAuthOUID).
		Return(ou.OrganizationUnit{ID: testAuthOUID, Name: "Engineering"}, nil)
	suite.mockAttributeCacheSvc.On("CreateAttributeCache", mock.Anything,
		mock.MatchedBy(func(cache *attributecache.AttributeCache) bool {
			return cache.Attributes[oauth2const.ClaimOUID] == testAuthOUID &&
				cache.Attributes[oauth2const.ClaimOUName] == "Engineering"
		})).Return(&attributecache.AttributeCache{ID: "cache-ou"}, nil)
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
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
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		AppID:       "app-123",
		FlowType:    common.FlowTypeAuthentication,
		AuthUser:    mustAssertAuthUser("user-123", "", ""),
		RuntimeData: map[string]string{
			common.RuntimeKeyRequiredEssentialAttributes: "email",
			common.RuntimeKeyRequiredOptionalAttributes:  "name",
		},
		ExecutionHistory: map[string]*common.NodeExecutionRecord{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{UserAttributes: []string{"email", "phone", "name"}},
		},
	}

	suite.mockAssertGenerator.On("GenerateAssertion", mock.Anything).Return(&authnassert.AssertionResult{
		Context: &authnassert.AssuranceContext{},
	}, (*serviceerror.ServiceError)(nil))
	suite.mockAuthnProvider.On("GetUserAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(authnprovidermgr.AuthUser{}, &authnprovidercm.AttributesResponse{
			Attributes: map[string]*authnprovidercm.AttributeResponse{
				"email": {Value: testEmail},
				"name":  {Value: testNameValue},
			},
		}, (*serviceerror.ServiceError)(nil)).Once()
	suite.mockJWTService.On("GenerateJWT", "user-123", mock.Anything, mock.Anything,
		mock.MatchedBy(func(claims map[string]interface{}) bool {
			_, hasPhone := claims["phone"]
			return claims["email"] == testEmail && claims["name"] == testNameValue && !hasPhone
		}), mock.Anything, mock.Anything).Return("jwt-token", int64(3600), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

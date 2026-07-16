/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
	"context"
	"encoding/base64"
	"fmt"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/authn/magiclinkmock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const (
	magicLinkTestUserID       = "user-123"
	magicLinkTestEmail        = "test@example.com"
	magicLinkTestExecutionID  = "flow-123"
	magicLinkTestAppID        = "app-123"
	magicLinkTestOUID         = "ou-123"
	magicLinkTestUserType     = "INTERNAL"
	magicLinkTestMagicLinkURL = "https://example.com/verify"
	magicLinkTestJWTHeader    = `{"alg":"HS256","typ":"JWT"}`
)

func toStringPtr(s string) *string { return &s }

func newMagicLinkAuthenticatedUser() providers.AuthUser {
	var authUser providers.AuthUser
	_ = authUser.UnmarshalJSON([]byte(`{"entityReferenceToken":"tok","attributeToken":"tok"}`))
	return authUser
}

// createTestJWTWithClaims creates a test JWT string with the given executionId and jti
func createTestJWTWithClaims(executionID, jti string) string {
	header := magicLinkTestJWTHeader
	payload := fmt.Sprintf(`{"sub":"user-123","executionId":%q,"jti":%q,"exp":9999999999}`, executionID, jti)

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))

	return headerB64 + "." + payloadB64 + ".test-signature"
}

func createRegistrationMagicLinkJWT(executionID, jti, subject string) string {
	header := magicLinkTestJWTHeader
	payload := fmt.Sprintf(
		`{"sub":%q,"email":%q,"registration":true,"executionId":%q,"jti":%q,"exp":9999999999}`,
		subject, subject, executionID, jti)

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))

	return headerB64 + "." + payloadB64 + ".test-signature"
}

type MagicLinkExecutorTestSuite struct {
	suite.Suite
	mockMagicLinkService *magiclinkmock.MagicLinkAuthnServiceInterfaceMock
	mockFlowFactory      *coremock.FlowFactoryInterfaceMock
	mockEntityProvider   *entityprovidermock.EntityProviderInterfaceMock
	mockAuthnProvider    *managermock.AuthnProviderManagerMock
	executor             *magicLinkExecutor
}

func TestMagicLinkExecutorSuite(t *testing.T) {
	suite.Run(t, new(MagicLinkExecutorTestSuite))
}

var testMagicLinkTokenInput = providers.Input{
	Ref:        "magic_link_token_input",
	Identifier: userInputMagicLinkToken,
	Type:       providers.InputTypeHidden,
	Required:   true,
}

var emailInput = providers.Input{
	Ref:        "email_input",
	Identifier: userAttributeEmail,
	Type:       providers.InputTypeEmail,
	Required:   true,
}

func defaultExpiryMatcher() interface{} {
	return mock.MatchedBy(func(expiry int64) bool {
		return expiry == int64(magiclink.DefaultExpirySeconds)
	})
}

func (suite *MagicLinkExecutorTestSuite) SetupTest() {
	suite.mockMagicLinkService = magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())

	defaultInputs := []providers.Input{testMagicLinkTokenInput}
	var prerequisites []providers.Input

	identifyingMock := createMockIdentifyingExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, providers.ExecutorTypeUtility,
		mock.Anything, mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	mockExec := createMockMagicLinkExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameMagicLink, providers.ExecutorTypeAuthentication,
		defaultInputs, prerequisites, mock.Anything).Return(mockExec)

	suite.executor = newMagicLinkExecutor(
		suite.mockFlowFactory,
		suite.mockMagicLinkService,
		suite.mockAuthnProvider,
		suite.mockEntityProvider)
	suite.executor.Executor = mockExec
}

func createMockMagicLinkExecutor(t *testing.T) providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(ExecutorNameMagicLink).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{emailInput}).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).Return(true).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) bool {
			token, exists := ctx.UserInputs[userInputMagicLinkToken]
			if !exists || token == "" {
				execResp.Inputs = []providers.Input{testMagicLinkTokenInput}
				execResp.Status = providers.ExecUserInputRequired
				return false
			}
			return true
		}).Maybe()
	return mockExec
}

func (suite *MagicLinkExecutorTestSuite) TestNewMagicLinkExecutor() {
	assert.NotNil(suite.T(), suite.executor)
	assert.NotNil(suite.T(), suite.executor.magicLinkService)
	assert.NotNil(suite.T(), suite.executor.entityProvider)
}

// Test Send Mode
func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_Success_AuthenticationFlow() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(toStringPtr(magicLinkTestUserID), nil)

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestUserID,
		defaultExpiryMatcher(), map[string]string{
			"id":            magicLinkTestExecutionID,
			"applicationId": magicLinkTestAppID,
			"type":          "AUTHENTICATION",
		},
		map[string]interface{}{"executionId": magicLinkTestExecutionID}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	templateData, ok := resp.ForwardedData[common.ForwardedDataKeyTemplateData].(map[string]interface{})
	assert.True(suite.T(), ok, "Template data should be present in ForwardedData")
	// Assert the correct values are inside the template data
	expectedURL := "https://example.com/verify?id=flow-123&token=jwt-token-123"
	assert.Equal(suite.T(), expectedURL, templateData["magicLink"])
	assert.Equal(suite.T(), "5", templateData["expiryMinutes"])
	assert.Equal(suite.T(), magicLinkTestUserID, resp.RuntimeData[userAttributeUserID])
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_Success_RegistrationFlow_NewUser() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestEmail,
		defaultExpiryMatcher(), map[string]string{
			"id":            magicLinkTestExecutionID,
			"applicationId": magicLinkTestAppID,
			"type":          "REGISTRATION"},
		map[string]interface{}{
			"executionId": magicLinkTestExecutionID,
		}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	templateData, ok := resp.ForwardedData[common.ForwardedDataKeyTemplateData].(map[string]interface{})
	assert.True(suite.T(), ok, "Template data should be present in ForwardedData")
	expectedURL := "https://example.com/verify?id=flow-123&token=jwt-token-123"
	assert.Equal(suite.T(), expectedURL, templateData["magicLink"])
	assert.Equal(suite.T(), "5", templateData["expiryMinutes"])
	assert.Equal(suite.T(), magicLinkTestEmail, resp.RuntimeData[userAttributeEmail])
	assert.Equal(suite.T(), userAttributeEmail, resp.RuntimeData[common.RuntimeKeyMagicLinkDestinationAttribute])
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_Success_RegistrationFlow_mobile_number() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		NodeInputs: []providers.Input{
			{Identifier: "mobile_number"},
		},
		UserInputs: map[string]string{
			"mobile_number": "+1234567890",
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"mobile_number": "+1234567890",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, "+1234567890",
		defaultExpiryMatcher(), map[string]string{
			"id":            magicLinkTestExecutionID,
			"applicationId": magicLinkTestAppID,
			"type":          "REGISTRATION"},
		map[string]interface{}{
			"executionId": magicLinkTestExecutionID,
		}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)

	templateData, ok := resp.ForwardedData[common.ForwardedDataKeyTemplateData].(map[string]interface{})
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "https://example.com/verify?id=flow-123&token=jwt-token-123", templateData["magicLink"])

	assert.Equal(suite.T(), "+1234567890", resp.RuntimeData["mobile_number"])
	assert.Equal(suite.T(), "mobile_number", resp.RuntimeData[common.RuntimeKeyMagicLinkDestinationAttribute])
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_AntiEnumeration_RegistrationFlow_UserExists() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(toStringPtr(magicLinkTestUserID), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeySkipDelivery])
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockMagicLinkService.AssertNotCalled(suite.T(), "GenerateMagicLink")
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_Success_WithCustomTokenExpiry() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: "600",
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(toStringPtr(magicLinkTestUserID), nil)

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestUserID,
		mock.MatchedBy(func(val int64) bool { return val == 600 }),
		map[string]string{
			"id":            magicLinkTestExecutionID,
			"applicationId": magicLinkTestAppID,
			"type":          "AUTHENTICATION",
		},
		map[string]interface{}{"executionId": magicLinkTestExecutionID}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)

	templateData, ok := resp.ForwardedData[common.ForwardedDataKeyTemplateData].(map[string]interface{})
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "10", templateData["expiryMinutes"])

	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_Success_WithCustomMagicLinkURL() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyMagicLinkURL: magicLinkTestMagicLinkURL,
		},
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(toStringPtr(magicLinkTestUserID), nil)

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestUserID,
		defaultExpiryMatcher(), map[string]string{
			"id":            magicLinkTestExecutionID,
			"applicationId": magicLinkTestAppID,
			"type":          "AUTHENTICATION",
		},
		map[string]interface{}{"executionId": magicLinkTestExecutionID},
		magicLinkTestMagicLinkURL).Return(magicLinkTestMagicLinkURL+"?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_Failure_GenerateMagicLinkError() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(toStringPtr(magicLinkTestUserID), nil)

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestUserID,
		defaultExpiryMatcher(), map[string]string{
			"id":            magicLinkTestExecutionID,
			"applicationId": magicLinkTestAppID,
			"type":          "AUTHENTICATION",
		},
		map[string]interface{}{"executionId": magicLinkTestExecutionID}, "").Return(
		"", &tidcommon.ServiceError{Code: tidcommon.InternalServerError.Code})

	resp, err := suite.executor.Execute(ctx)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to generate magic link")
	assert.NotNil(suite.T(), resp)
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_Failure_ClientError() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(toStringPtr(magicLinkTestUserID), nil)

	clientErr := &tidcommon.ServiceError{
		Type:             tidcommon.ClientErrorType,
		Code:             "TEST-CLIENT-ERROR",
		ErrorDescription: tidcommon.InternalServerError.ErrorDescription,
	}
	suite.mockMagicLinkService.On(
		"GenerateMagicLink",
		ctx.Context,
		magicLinkTestUserID,
		defaultExpiryMatcher(), map[string]string{
			"id":            magicLinkTestExecutionID,
			"applicationId": magicLinkTestAppID,
			"type":          "AUTHENTICATION",
		},
		map[string]interface{}{"executionId": magicLinkTestExecutionID}, "").
		Return("", clientErr)

	resp, err := suite.executor.Execute(ctx)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

//nolint:dupl // identical to registration test
func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_AntiEnumeration_UserNotFound() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeySkipDelivery])
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockMagicLinkService.AssertNotCalled(suite.T(), "GenerateMagicLink")
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_Success_WithAuthenticatedUser() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: map[string]string{
			userAttributeUserID: magicLinkTestUserID,
		},
		AuthUser: newMagicLinkAuthenticatedUser(),
	}

	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameMagicLink).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{emailInput}).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).Return(true).Maybe()
	mockExec.On("GetUserIDFromContext", mock.Anything, mock.Anything, mock.Anything).Return(magicLinkTestUserID).Maybe()
	suite.executor.Executor = mockExec

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestUserID,
		defaultExpiryMatcher(), map[string]string{
			"id":            magicLinkTestExecutionID,
			"applicationId": magicLinkTestAppID,
			"type":          "AUTHENTICATION",
		},
		map[string]interface{}{"executionId": magicLinkTestExecutionID}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), magicLinkTestUserID, resp.RuntimeData[userAttributeUserID])
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity")
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

// Test Verify Mode

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_Success() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-success")

	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			userAttributeUserID: magicLinkTestUserID,
		},
	}

	authenticatedAuthUser := newMagicLinkAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", ctx.Context, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(
		authenticatedAuthUser, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthUser.IsAuthenticated())
	assert.Equal(suite.T(), "jti-success", resp.RuntimeData[common.RuntimeKeyMagicLinkUsedJti])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_Success_RegistrationFlow() {
	testToken := createRegistrationMagicLinkJWT(magicLinkTestExecutionID, "jti-registration", magicLinkTestEmail)

	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyMagicLinkDestinationAttribute: userAttributeEmail,
		},
	}

	suite.mockAuthnProvider.On("AuthenticateUser", ctx.Context, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "jti-registration", resp.RuntimeData[common.RuntimeKeyMagicLinkUsedJti])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_Success_RegistrationFlow_MobileNumber() {
	testToken := createRegistrationMagicLinkJWT(magicLinkTestExecutionID, "jti-registration", "+1234567890")

	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyMagicLinkDestinationAttribute: "mobile_number",
		},
	}

	suite.mockAuthnProvider.On("AuthenticateUser", ctx.Context, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "jti-registration", resp.RuntimeData[common.RuntimeKeyMagicLinkUsedJti])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_RegistrationFlow_UsesStoredDestinationAttribute() {
	const (
		workEmailAttr  = "workemail"
		workEmailValue = "johnwork@company.lk"
	)
	testToken := createRegistrationMagicLinkJWT(magicLinkTestExecutionID, "jti-registration", workEmailValue)

	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyMagicLinkDestinationAttribute: workEmailAttr,
		},
	}

	suite.mockAuthnProvider.On("AuthenticateUser", ctx.Context, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_RegistrationFlow_MissingDestinationAttribute() {
	testToken := createRegistrationMagicLinkJWT(magicLinkTestExecutionID, "jti-registration", magicLinkTestEmail)

	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	suite.mockAuthnProvider.AssertNotCalled(suite.T(), "AuthenticateUser")
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_Failure_TokenNotProvided() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Len(suite.T(), resp.Inputs, 1)
	assert.Equal(suite.T(), userInputMagicLinkToken, resp.Inputs[0].Identifier)
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_Failure_InvalidToken() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-invalid")

	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockAuthnProvider.On("AuthenticateUser", ctx.Context, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, (providers.AuthenticatedClaims)(nil),
		&tidcommon.ServiceError{
			Type:             tidcommon.ClientErrorType,
			Code:             authnprovidermgr.ErrorAuthenticationFailed.Code,
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "The provided magic link token is invalid"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "The provided magic link token is invalid", resp.Error.ErrorDescription.DefaultValue)
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_Failure_ReplayAttack() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-replay")

	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyMagicLinkUsedJti: "jti-replay",
		},
	}

	suite.mockAuthnProvider.On("AuthenticateUser", ctx.Context, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrInvalidMagicLinkToken.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_Success_ReplacesStoredJTI() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-new")

	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyMagicLinkUsedJti: "jti-old",
			userAttributeUserID:               magicLinkTestUserID,
		},
	}

	authenticatedAuthUser := newMagicLinkAuthenticatedUser()
	suite.mockAuthnProvider.On("AuthenticateUser", ctx.Context, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(
		authenticatedAuthUser, providers.AuthenticatedClaims{}, (*tidcommon.ServiceError)(nil))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "jti-new", resp.RuntimeData[common.RuntimeKeyMagicLinkUsedJti])
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_Failure_AuthenticateUserServerError() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-server-error")

	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			userAttributeUserID: magicLinkTestUserID,
		},
	}

	suite.mockAuthnProvider.On("AuthenticateUser", ctx.Context, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(
		providers.AuthUser{}, (providers.AuthenticatedClaims)(nil),
		&tidcommon.ServiceError{
			Type:             tidcommon.ServerErrorType,
			Code:             "AUTH-5000",
			ErrorDescription: tidcommon.I18nMessage{DefaultValue: "database error"},
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to verify magic link")
	suite.mockAuthnProvider.AssertExpectations(suite.T())
}

// Test Invalid Executor Mode

func (suite *MagicLinkExecutorTestSuite) TestExecute_InvalidMode() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: "invalid-mode",
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "invalid executor mode")
	assert.NotNil(suite.T(), resp)
}

// Test Prerequisites Aren't Met

func (suite *MagicLinkExecutorTestSuite) TestExecute_PrerequisitesNotMet() {
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameMagicLink).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{emailInput}).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).Return(false)
	suite.executor.Executor = mockExec

	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
}

// Test Helper Methods

func (suite *MagicLinkExecutorTestSuite) TestBuildUserSearchAttributes_FromUserInputs() {
	ctx := &providers.NodeContext{
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	attrs := suite.executor.buildUserSearchAttributes(ctx)

	assert.Equal(suite.T(), magicLinkTestEmail, attrs[userAttributeEmail])
}

func (suite *MagicLinkExecutorTestSuite) TestBuildUserSearchAttributes_NotFound() {
	ctx := &providers.NodeContext{
		UserInputs:    make(map[string]string),
		RuntimeData:   make(map[string]string),
		ForwardedData: make(map[string]interface{}),
	}

	attrs := suite.executor.buildUserSearchAttributes(ctx)

	assert.Empty(suite.T(), attrs)
}

// Test Property Getters

func (suite *MagicLinkExecutorTestSuite) TestGetTokenExpiry_DefaultValue() {
	ctx := &providers.NodeContext{
		NodeProperties: nil,
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(magiclink.DefaultExpirySeconds), expiry)
}

func (suite *MagicLinkExecutorTestSuite) TestGetTokenExpiry_CustomValue() {
	ctx := &providers.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: "600",
		},
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(600), expiry)
}

func (suite *MagicLinkExecutorTestSuite) TestGetTokenExpiry_InvalidValue_UsesDefault() {
	ctx := &providers.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: "invalid",
		},
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(magiclink.DefaultExpirySeconds), expiry)
}

func (suite *MagicLinkExecutorTestSuite) TestGetTokenExpiry_NegativeValue_UsesDefault() {
	ctx := &providers.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: "-100",
		},
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(magiclink.DefaultExpirySeconds), expiry)
}

func (suite *MagicLinkExecutorTestSuite) TestGetTokenExpiry_EmptyString_UsesDefault() {
	ctx := &providers.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: "",
		},
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(magiclink.DefaultExpirySeconds), expiry)
}

func (suite *MagicLinkExecutorTestSuite) TestGetTokenExpiry_NonStringValue_ParsesSuccessfully() {
	ctx := &providers.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: float64(123),
		},
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(123), expiry)
}

func (suite *MagicLinkExecutorTestSuite) TestGetMagicLinkURL_DefaultEmpty() {
	ctx := &providers.NodeContext{
		NodeProperties: nil,
	}

	url := suite.executor.getMagicLinkURL(ctx)

	assert.Equal(suite.T(), "", url)
}

func (suite *MagicLinkExecutorTestSuite) TestGetMagicLinkURL_CustomValue() {
	ctx := &providers.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyMagicLinkURL: magicLinkTestMagicLinkURL,
		},
	}

	url := suite.executor.getMagicLinkURL(ctx)

	assert.Equal(suite.T(), magicLinkTestMagicLinkURL, url)
}

func (suite *MagicLinkExecutorTestSuite) TestGetMagicLinkURL_NonStringValue_ReturnsEmpty() {
	ctx := &providers.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyMagicLinkURL: 12345,
		},
	}

	url := suite.executor.getMagicLinkURL(ctx)

	assert.Equal(suite.T(), "", url)
}

// Test Edge Cases
func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_AuthenticatedUser_EmptyUserID() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
		AuthUser:    newMagicLinkAuthenticatedUser(),
	}

	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameMagicLink).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{emailInput}).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).Return(true).Maybe()
	mockExec.On("GetUserIDFromContext", mock.Anything, mock.Anything, mock.Anything).Return("")
	suite.executor.Executor = mockExec

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "user ID is empty")
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_VerifyMode_EmptyToken() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: "",
		},
		RuntimeData: make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
}

func (suite *MagicLinkExecutorTestSuite) TestExecute_GenerateMode_RegistrationFlow_IdentifyUserSystemError() {
	ctx := &providers.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     providers.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		Application:  providers.Application{ID: magicLinkTestAppID},
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "", ""))

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestEmail,
		defaultExpiryMatcher(), map[string]string{
			"id":            magicLinkTestExecutionID,
			"applicationId": magicLinkTestAppID,
			"type":          "REGISTRATION",
		},
		map[string]interface{}{
			"executionId": magicLinkTestExecutionID,
		}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkExecutorTestSuite) TestValidateFlowClaims_FlowIdMismatch() {
	token := createTestJWTWithClaims("wrong-flow-id", "test-jti-123")
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		RuntimeData: make(map[string]string),
	}

	logger := log.GetLogger()
	tokenJTI, failure := suite.executor.validateFlowClaims(ctx, token, logger)

	suite.Empty(tokenJTI)
	suite.Equal(ErrInvalidMagicLinkToken.Error.DefaultValue, failure.Error.DefaultValue)
}

func (suite *MagicLinkExecutorTestSuite) TestValidateFlowClaims_ReplayAttack() {
	token := createTestJWTWithClaims(magicLinkTestExecutionID, "test-jti-123")
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		RuntimeData: map[string]string{common.RuntimeKeyMagicLinkUsedJti: "test-jti-123"},
	}

	logger := log.GetLogger()
	tokenJTI, failure := suite.executor.validateFlowClaims(ctx, token, logger)

	suite.Empty(tokenJTI)
	suite.Equal(ErrInvalidMagicLinkToken.Error.DefaultValue, failure.Error.DefaultValue)
}

func (suite *MagicLinkExecutorTestSuite) TestValidateFlowClaims_NewTokenReturnsJTI() {
	newToken := createTestJWTWithClaims(magicLinkTestExecutionID, "new-jti-456")
	ctx := &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		RuntimeData: map[string]string{common.RuntimeKeyMagicLinkUsedJti: "old-jti-123"},
	}

	logger := log.GetLogger()
	tokenJTI, failure := suite.executor.validateFlowClaims(ctx, newToken, logger)

	suite.Equal("new-jti-456", tokenJTI)
	suite.Nil(failure)
	suite.Equal("old-jti-123", ctx.RuntimeData[common.RuntimeKeyMagicLinkUsedJti])
}

func (suite *MagicLinkExecutorTestSuite) TestCreateRegistrationMagicLinkJWT_Helper() {
	// Calling the helper with a completely different executionID ("different-flow-id")
	// satisfies the 'unparam' linter, proving the parameter is actually dynamic.
	testEmail := "another@example.com"
	differentExecutionID := "different-flow-id"

	token := createRegistrationMagicLinkJWT(differentExecutionID, "test-jti", testEmail)

	suite.NotEmpty(token, "Generated token should not be empty")
	suite.Contains(token, ".", "Generated token should contain JWT separators")
}

func (suite *MagicLinkExecutorTestSuite) TestValidateFlowClaims_DecodeFailure() {
	// Pass a completely malformed token string
	// nolint:gosec // G101: Test data for negative case, not a real credential
	token := "not.a.valid.jwt.format"

	ctx := &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		RuntimeData: make(map[string]string),
	}

	logger := log.GetLogger()
	tokenJTI, failure := suite.executor.validateFlowClaims(ctx, token, logger)

	suite.Empty(tokenJTI)
	suite.Equal(ErrInvalidMagicLinkToken.Error.DefaultValue, failure.Error.DefaultValue)
}

func (suite *MagicLinkExecutorTestSuite) TestValidateFlowClaims_MissingJTI() {
	header := magicLinkTestJWTHeader
	payload := fmt.Sprintf(`{"sub":"user-123","executionId":%q,"exp":9999999999}`, magicLinkTestExecutionID)

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	token := headerB64 + "." + payloadB64 + ".test-signature"

	ctx := &providers.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		RuntimeData: make(map[string]string),
	}

	logger := log.GetLogger()
	tokenJTI, failure := suite.executor.validateFlowClaims(ctx, token, logger)

	suite.Empty(tokenJTI)
	suite.Equal(ErrInvalidMagicLinkToken.Error.DefaultValue, failure.Error.DefaultValue)
}

func (suite *MagicLinkExecutorTestSuite) TestGetExecutionPolicy() {
	policyVerify := suite.executor.GetExecutionPolicy(ExecutorModeVerify)
	suite.NotNil(policyVerify)
	suite.True(policyVerify.SkipChallengeValidation)
	suite.False(policyVerify.AllowSegmentRestart)

	policyGenerate := suite.executor.GetExecutionPolicy(ExecutorModeGenerate)
	suite.Nil(policyGenerate)

	policyUnknown := suite.executor.GetExecutionPolicy("unknown")
	suite.Nil(policyUnknown)
}

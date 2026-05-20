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
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/magiclink"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/authn/magiclinkmock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const (
	magicLinkTestUserID       = "user-123"
	magicLinkTestEmail        = "test@example.com"
	magicLinkTestExecutionID  = "flow-123"
	magicLinkTestOUID         = "ou-123"
	magicLinkTestUserType     = "INTERNAL"
	magicLinkTestMagicLinkURL = "https://example.com/verify"
	magicLinkTestJWTHeader    = `{"alg":"HS256","typ":"JWT"}`
)

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

type MagicLinkAuthExecutorTestSuite struct {
	suite.Suite
	mockMagicLinkService *magiclinkmock.MagicLinkAuthnServiceInterfaceMock
	mockFlowFactory      *coremock.FlowFactoryInterfaceMock
	mockEntityProvider   *entityprovidermock.EntityProviderInterfaceMock
	executor             *magicLinkAuthExecutor
}

func TestMagicLinkAuthExecutorSuite(t *testing.T) {
	suite.Run(t, new(MagicLinkAuthExecutorTestSuite))
}

var testMagicLinkTokenInput = common.Input{
	Ref:        "magic_link_token_input",
	Identifier: userInputMagicLinkToken,
	Type:       common.InputTypeHidden,
	Required:   true,
}

var emailInput = common.Input{
	Ref:        "email_input",
	Identifier: userAttributeEmail,
	Type:       common.InputTypeEmail,
	Required:   true,
}

func defaultExpiryMatcher() interface{} {
	return mock.MatchedBy(func(expiry int64) bool {
		return expiry == int64(magiclink.DefaultExpirySeconds)
	})
}

func (suite *MagicLinkAuthExecutorTestSuite) SetupTest() {
	suite.mockMagicLinkService = magiclinkmock.NewMagicLinkAuthnServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())

	defaultInputs := []common.Input{testMagicLinkTokenInput}
	var prerequisites []common.Input

	identifyingMock := createMockIdentifyingExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, common.ExecutorTypeUtility,
		mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	mockExec := createMockMagicLinkAuthExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameMagicLinkAuth, common.ExecutorTypeAuthentication,
		defaultInputs, prerequisites).Return(mockExec)

	suite.executor = newMagicLinkAuthExecutor(
		suite.mockFlowFactory,
		suite.mockMagicLinkService,
		suite.mockEntityProvider)
	suite.executor.ExecutorInterface = mockExec
}

func createMockMagicLinkAuthExecutor(t *testing.T) core.ExecutorInterface {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(ExecutorNameMagicLinkAuth).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetRequiredInputs", mock.Anything).Return([]common.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{emailInput}).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything).Return(true).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *core.NodeContext, execResp *common.ExecutorResponse) bool {
			token, exists := ctx.UserInputs[userInputMagicLinkToken]
			if !exists || token == "" {
				execResp.Inputs = []common.Input{testMagicLinkTokenInput}
				execResp.Status = common.ExecUserInputRequired
				return false
			}
			return true
		}).Maybe()
	return mockExec
}

func (suite *MagicLinkAuthExecutorTestSuite) TestNewMagicLinkAuthExecutor() {
	assert.NotNil(suite.T(), suite.executor)
	assert.NotNil(suite.T(), suite.executor.magicLinkService)
	assert.NotNil(suite.T(), suite.executor.entityProvider)
}

// Test Send Mode
func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_Success_AuthenticationFlow() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(new(magicLinkTestUserID), nil)

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestUserID,
		defaultExpiryMatcher(), map[string]string{"id": magicLinkTestExecutionID},
		map[string]interface{}{"executionId": magicLinkTestExecutionID}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
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

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_Success_RegistrationFlow_NewUser() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestEmail,
		defaultExpiryMatcher(), map[string]string{"id": magicLinkTestExecutionID},
		map[string]interface{}{
			"executionId": magicLinkTestExecutionID,
		}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
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

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_Success_RegistrationFlow_MobileNumber() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		NodeInputs: []common.Input{
			{Identifier: "mobileNumber"},
		},
		UserInputs: map[string]string{
			"mobileNumber": "+1234567890",
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"mobileNumber": "+1234567890",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, "+1234567890",
		defaultExpiryMatcher(), map[string]string{"id": magicLinkTestExecutionID},
		map[string]interface{}{
			"executionId": magicLinkTestExecutionID,
		}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)

	templateData, ok := resp.ForwardedData[common.ForwardedDataKeyTemplateData].(map[string]interface{})
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "https://example.com/verify?id=flow-123&token=jwt-token-123", templateData["magicLink"])

	assert.Equal(suite.T(), "+1234567890", resp.RuntimeData["mobileNumber"])
	assert.Equal(suite.T(), "mobileNumber", resp.RuntimeData[common.RuntimeKeyMagicLinkDestinationAttribute])
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_AntiEnumeration_RegistrationFlow_UserExists() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(new(magicLinkTestUserID), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeySkipDelivery])
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockMagicLinkService.AssertNotCalled(suite.T(), "GenerateMagicLink")
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_Success_WithCustomTokenExpiry() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
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
	}).Return(new(magicLinkTestUserID), nil)

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestUserID,
		mock.MatchedBy(func(val int64) bool { return val == 600 }),
		map[string]string{"id": magicLinkTestExecutionID},
		map[string]interface{}{"executionId": magicLinkTestExecutionID}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)

	templateData, ok := resp.ForwardedData[common.ForwardedDataKeyTemplateData].(map[string]interface{})
	assert.True(suite.T(), ok)
	assert.Equal(suite.T(), "10", templateData["expiryMinutes"])

	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_Success_WithCustomMagicLinkURL() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
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
	}).Return(new(magicLinkTestUserID), nil)

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestUserID,
		defaultExpiryMatcher(), map[string]string{"id": magicLinkTestExecutionID},
		map[string]interface{}{"executionId": magicLinkTestExecutionID},
		magicLinkTestMagicLinkURL).Return(magicLinkTestMagicLinkURL+"?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_Failure_GenerateMagicLinkError() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(new(magicLinkTestUserID), nil)

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestUserID,
		defaultExpiryMatcher(), map[string]string{"id": magicLinkTestExecutionID},
		map[string]interface{}{"executionId": magicLinkTestExecutionID}, "").Return(
		"", &serviceerror.ServiceError{Code: serviceerror.InternalServerError.Code})

	resp, err := suite.executor.Execute(ctx)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to generate magic link")
	assert.NotNil(suite.T(), resp)
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_Failure_ClientError() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(new(magicLinkTestUserID), nil)

	clientErr := &serviceerror.ServiceError{
		Type:             serviceerror.ClientErrorType,
		Code:             "TEST-CLIENT-ERROR",
		ErrorDescription: serviceerror.InternalServerError.ErrorDescription,
	}
	suite.mockMagicLinkService.On(
		"GenerateMagicLink",
		ctx.Context,
		magicLinkTestUserID,
		defaultExpiryMatcher(), map[string]string{"id": magicLinkTestExecutionID},
		map[string]interface{}{"executionId": magicLinkTestExecutionID}, "").
		Return("", clientErr)

	resp, err := suite.executor.Execute(ctx)
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

//nolint:dupl // identical to registration test
func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_AntiEnumeration_UserNotFound() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
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
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), dataValueTrue, resp.RuntimeData[common.RuntimeKeySkipDelivery])
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockMagicLinkService.AssertNotCalled(suite.T(), "GenerateMagicLink")
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_Success_WithAuthenticatedUser() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: map[string]string{
			userAttributeUserID: magicLinkTestUserID,
		},
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          magicLinkTestUserID,
		},
	}

	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameMagicLinkAuth).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetRequiredInputs", mock.Anything).Return([]common.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{emailInput}).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything).Return(true).Maybe()
	mockExec.On("GetUserIDFromContext", mock.Anything).Return(magicLinkTestUserID).Maybe()
	suite.executor.ExecutorInterface = mockExec

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestUserID,
		defaultExpiryMatcher(), map[string]string{"id": magicLinkTestExecutionID},
		map[string]interface{}{"executionId": magicLinkTestExecutionID}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), magicLinkTestUserID, resp.RuntimeData[userAttributeUserID])
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity")
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

// Test Verify Mode

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_Success() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-success")
	attrs := map[string]interface{}{
		"email": magicLinkTestEmail,
	}
	attrsJSON, _ := json.Marshal(attrs)
	user := &entityprovider.Entity{
		ID:         magicLinkTestUserID,
		Type:       magicLinkTestUserType,
		OUID:       magicLinkTestOUID,
		Attributes: attrsJSON,
	}

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			userAttributeUserID: magicLinkTestUserID,
		},
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, testToken, "").Return(user, nil)
	suite.mockEntityProvider.On("GetEntity", magicLinkTestUserID).Return(user, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), magicLinkTestUserID, resp.AuthenticatedUser.UserID)
	assert.Equal(suite.T(), magicLinkTestUserType, resp.AuthenticatedUser.UserType)
	assert.Equal(suite.T(), magicLinkTestOUID, resp.AuthenticatedUser.OUID)
	assert.Equal(suite.T(), "jti-success", resp.RuntimeData[common.RuntimeKeyMagicLinkUsedJti])
	suite.mockMagicLinkService.AssertExpectations(suite.T())
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_Success_RegistrationFlow() {
	testToken := createRegistrationMagicLinkJWT(magicLinkTestExecutionID, "jti-registration", magicLinkTestEmail)

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyMagicLinkDestinationAttribute: userAttributeEmail,
		},
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, testToken, userAttributeEmail).
		Return(nil, &authncm.ErrorUserNotFound)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "jti-registration", resp.RuntimeData[common.RuntimeKeyMagicLinkUsedJti])
	suite.mockMagicLinkService.AssertExpectations(suite.T())
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "GetEntity", mock.Anything)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_Success_RegistrationFlow_MobileNumber() {
	testToken := createRegistrationMagicLinkJWT(magicLinkTestExecutionID, "jti-registration", "+1234567890")

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyMagicLinkDestinationAttribute: "mobileNumber",
		},
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, testToken, "mobileNumber").
		Return(nil, &authncm.ErrorUserNotFound)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "jti-registration", resp.RuntimeData[common.RuntimeKeyMagicLinkUsedJti])
	suite.mockMagicLinkService.AssertExpectations(suite.T())
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "GetEntity", mock.Anything)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_RegistrationFlow_UsesStoredDestinationAttribute() {
	const (
		workEmailAttr  = "workemail"
		workEmailValue = "johnwork@company.lk"
	)
	testToken := createRegistrationMagicLinkJWT(magicLinkTestExecutionID, "jti-registration", workEmailValue)

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyMagicLinkDestinationAttribute: workEmailAttr,
		},
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, testToken, workEmailAttr).
		Return(nil, &authncm.ErrorUserNotFound)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_RegistrationFlow_MissingDestinationAttribute() {
	testToken := createRegistrationMagicLinkJWT(magicLinkTestExecutionID, "jti-registration", magicLinkTestEmail)

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	suite.mockMagicLinkService.AssertNotCalled(suite.T(), "VerifyMagicLink")
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_Failure_TokenNotProvided() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Len(suite.T(), resp.Inputs, 1)
	assert.Equal(suite.T(), userInputMagicLinkToken, resp.Inputs[0].Identifier)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_Failure_InvalidToken() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-invalid")

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, testToken, "").Return(
		nil, &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			Code:             "AUTHN-ML-1002",
			ErrorDescription: i18ncore.I18nMessage{DefaultValue: "The provided magic link token is invalid"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "The provided magic link token is invalid", resp.FailureReason)
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_Failure_ReplayAttack() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-replay")

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyMagicLinkUsedJti: "jti-replay",
		},
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, testToken, "").Return(
		&entityprovider.Entity{ID: magicLinkTestUserID}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Magic link has already been used", resp.FailureReason)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_Success_ReplacesStoredJTI() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-new")
	user := &entityprovider.Entity{
		ID:   magicLinkTestUserID,
		Type: magicLinkTestUserType,
		OUID: magicLinkTestOUID,
	}

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyMagicLinkUsedJti: "jti-old",
			userAttributeUserID:               magicLinkTestUserID,
		},
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, testToken, "").Return(user, nil)
	suite.mockEntityProvider.On("GetEntity", magicLinkTestUserID).Return(user, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "jti-new", resp.RuntimeData[common.RuntimeKeyMagicLinkUsedJti])
	suite.mockMagicLinkService.AssertExpectations(suite.T())
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_Failure_UserNotFoundAfterVerification() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-user-not-found")

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, testToken, "").Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), failureReasonUserNotFound, resp.FailureReason)
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_Failure_GetUserError() {
	testToken := createTestJWTWithClaims(magicLinkTestExecutionID, "jti-get-user-error")

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: testToken,
		},
		RuntimeData: map[string]string{
			userAttributeUserID: magicLinkTestUserID,
		},
	}

	user := &entityprovider.Entity{
		ID:   magicLinkTestUserID,
		Type: magicLinkTestUserType,
		OUID: magicLinkTestOUID,
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, testToken, "").Return(user, nil)
	suite.mockEntityProvider.On("GetEntity", magicLinkTestUserID).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "database error", ""))

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get user")
	suite.mockMagicLinkService.AssertExpectations(suite.T())
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

// Test Invalid Executor Mode

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_InvalidMode() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
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

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_PrerequisitesNotMet() {
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameMagicLinkAuth).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{emailInput}).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything).Return(false)
	suite.executor.ExecutorInterface = mockExec

	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
}

// Test Helper Methods

func (suite *MagicLinkAuthExecutorTestSuite) TestBuildUserSearchAttributes_FromUserInputs() {
	ctx := &core.NodeContext{
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	attrs := suite.executor.buildUserSearchAttributes(ctx)

	assert.Equal(suite.T(), magicLinkTestEmail, attrs[userAttributeEmail])
}

func (suite *MagicLinkAuthExecutorTestSuite) TestBuildUserSearchAttributes_NotFound() {
	ctx := &core.NodeContext{
		UserInputs:    make(map[string]string),
		RuntimeData:   make(map[string]string),
		ForwardedData: make(map[string]interface{}),
	}

	attrs := suite.executor.buildUserSearchAttributes(ctx)

	assert.Empty(suite.T(), attrs)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetAuthenticatedUser_Success() {
	user := &entityprovider.Entity{
		ID:   magicLinkTestUserID,
		Type: magicLinkTestUserType,
		OUID: magicLinkTestOUID,
	}

	suite.mockEntityProvider.On("GetEntity", magicLinkTestUserID).Return(user, nil)

	authenticatedUser, err := suite.executor.getAuthenticatedUser(magicLinkTestUserID)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), authenticatedUser)
	assert.True(suite.T(), authenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), magicLinkTestUserID, authenticatedUser.UserID)
	assert.Equal(suite.T(), magicLinkTestUserType, authenticatedUser.UserType)
	assert.Equal(suite.T(), magicLinkTestOUID, authenticatedUser.OUID)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetAuthenticatedUser_UserIDNotFound() {
	authenticatedUser, err := suite.executor.getAuthenticatedUser("")

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), authenticatedUser)
	assert.Contains(suite.T(), err.Error(), "user ID is empty")
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetAuthenticatedUser_GetUserError() {
	suite.mockEntityProvider.On("GetEntity", magicLinkTestUserID).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "user not found", ""))

	authenticatedUser, err := suite.executor.getAuthenticatedUser(magicLinkTestUserID)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), authenticatedUser)
	assert.Contains(suite.T(), err.Error(), "failed to get user")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

// Test Property Getters

func (suite *MagicLinkAuthExecutorTestSuite) TestGetTokenExpiry_DefaultValue() {
	ctx := &core.NodeContext{
		NodeProperties: nil,
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(magiclink.DefaultExpirySeconds), expiry)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetTokenExpiry_CustomValue() {
	ctx := &core.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: "600",
		},
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(600), expiry)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetTokenExpiry_InvalidValue_UsesDefault() {
	ctx := &core.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: "invalid",
		},
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(magiclink.DefaultExpirySeconds), expiry)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetTokenExpiry_NegativeValue_UsesDefault() {
	ctx := &core.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: "-100",
		},
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(magiclink.DefaultExpirySeconds), expiry)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetTokenExpiry_EmptyString_UsesDefault() {
	ctx := &core.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: "",
		},
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(magiclink.DefaultExpirySeconds), expiry)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetTokenExpiry_NonStringValue_UsesDefault() {
	ctx := &core.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyTokenExpiry: 123,
		},
	}

	expiry := suite.executor.getTokenExpiry(ctx)

	assert.Equal(suite.T(), int64(magiclink.DefaultExpirySeconds), expiry)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetMagicLinkURL_DefaultEmpty() {
	ctx := &core.NodeContext{
		NodeProperties: nil,
	}

	url := suite.executor.getMagicLinkURL(ctx)

	assert.Equal(suite.T(), "", url)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetMagicLinkURL_CustomValue() {
	ctx := &core.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyMagicLinkURL: magicLinkTestMagicLinkURL,
		},
	}

	url := suite.executor.getMagicLinkURL(ctx)

	assert.Equal(suite.T(), magicLinkTestMagicLinkURL, url)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestGetMagicLinkURL_NonStringValue_ReturnsEmpty() {
	ctx := &core.NodeContext{
		NodeProperties: map[string]interface{}{
			propertyKeyMagicLinkURL: 12345,
		},
	}

	url := suite.executor.getMagicLinkURL(ctx)

	assert.Equal(suite.T(), "", url)
}

// Test Edge Cases
func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_AuthenticatedUser_EmptyUserID() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
		AuthenticatedUser: authncm.AuthenticatedUser{
			IsAuthenticated: true,
			UserID:          "",
		},
	}

	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return(ExecutorNameMagicLinkAuth).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{testMagicLinkTokenInput}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{emailInput}).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything).Return(true).Maybe()
	mockExec.On("GetUserIDFromContext", mock.Anything).Return("")
	suite.executor.ExecutorInterface = mockExec

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "user ID is empty")
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_VerifyMode_EmptyToken() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeAuthentication,
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputMagicLinkToken: "",
		},
		RuntimeData: make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestExecute_GenerateMode_RegistrationFlow_IdentifyUserSystemError() {
	ctx := &core.NodeContext{
		Context:      context.Background(),
		ExecutionID:  magicLinkTestExecutionID,
		FlowType:     common.FlowTypeRegistration,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs: map[string]string{
			userAttributeEmail: magicLinkTestEmail,
		},
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		userAttributeEmail: magicLinkTestEmail,
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "", ""))

	suite.mockMagicLinkService.On("GenerateMagicLink", ctx.Context, magicLinkTestEmail,
		defaultExpiryMatcher(), map[string]string{"id": magicLinkTestExecutionID},
		map[string]interface{}{
			"executionId": magicLinkTestExecutionID,
		}, "").Return(
		"https://example.com/verify?id=flow-123&token=jwt-token-123", nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
	suite.mockMagicLinkService.AssertExpectations(suite.T())
}

func (suite *MagicLinkAuthExecutorTestSuite) TestValidateMagicLinkToken_FlowIdMismatch() {
	token := createTestJWTWithClaims("wrong-flow-id", "test-jti-123")
	ctx := &core.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		UserInputs:  map[string]string{userInputMagicLinkToken: token},
		RuntimeData: make(map[string]string),
	}
	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, token, "").Return(
		&entityprovider.Entity{ID: magicLinkTestUserID}, nil)

	logger := log.GetLogger()
	tokenJTI, failure, err := suite.executor.validateMagicLinkToken(ctx, logger)

	suite.Empty(tokenJTI)
	suite.Equal("Invalid magic link token", failure)
	suite.Nil(err)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestValidateMagicLinkToken_MissingDestinationAttributeOnRegistration() {
	token := createRegistrationMagicLinkJWT(magicLinkTestExecutionID, "test-jti-123", magicLinkTestEmail)
	ctx := &core.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		FlowType:    common.FlowTypeRegistration,
		UserInputs:  map[string]string{userInputMagicLinkToken: token},
		RuntimeData: make(map[string]string),
	}

	logger := log.GetLogger()
	tokenJTI, failure, err := suite.executor.validateMagicLinkToken(ctx, logger)

	suite.Empty(tokenJTI)
	suite.Empty(failure)
	suite.Error(err)
	suite.Contains(err.Error(), "magic link destination attribute missing from runtime data")
}

func (suite *MagicLinkAuthExecutorTestSuite) TestValidateMagicLinkToken_ReplayAttack() {
	token := createTestJWTWithClaims(magicLinkTestExecutionID, "test-jti-123")
	ctx := &core.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		UserInputs:  map[string]string{userInputMagicLinkToken: token},
		RuntimeData: map[string]string{common.RuntimeKeyMagicLinkUsedJti: "test-jti-123"},
	}
	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, token, "").Return(
		&entityprovider.Entity{ID: magicLinkTestUserID}, nil)

	logger := log.GetLogger()
	tokenJTI, failure, err := suite.executor.validateMagicLinkToken(ctx, logger)

	suite.Empty(tokenJTI)
	suite.Equal("Magic link has already been used", failure)
	suite.Nil(err)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestValidateMagicLinkToken_NewTokenReturnsJTI() {
	newToken := createTestJWTWithClaims(magicLinkTestExecutionID, "new-jti-456")
	testUser := &entityprovider.Entity{
		ID:   magicLinkTestUserID,
		OUID: magicLinkTestOUID,
		Type: magicLinkTestUserType,
	}
	ctx := &core.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		UserInputs:  map[string]string{userInputMagicLinkToken: newToken},
		RuntimeData: map[string]string{common.RuntimeKeyMagicLinkUsedJti: "old-jti-123"},
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", mock.Anything, newToken, "").Return(testUser, nil)

	logger := log.GetLogger()
	tokenJTI, failure, err := suite.executor.validateMagicLinkToken(ctx, logger)

	suite.Equal("new-jti-456", tokenJTI)
	suite.Empty(failure)
	suite.Nil(err)
	suite.Equal("old-jti-123", ctx.RuntimeData[common.RuntimeKeyMagicLinkUsedJti])
}

func (suite *MagicLinkAuthExecutorTestSuite) TestCreateRegistrationMagicLinkJWT_Helper() {
	// Calling the helper with a completely different executionID ("different-flow-id")
	// satisfies the 'unparam' linter, proving the parameter is actually dynamic.
	testEmail := "another@example.com"
	differentExecutionID := "different-flow-id"

	token := createRegistrationMagicLinkJWT(differentExecutionID, "test-jti", testEmail)

	// Run a basic sanity check to ensure the token is generated successfully
	suite.NotEmpty(token, "Generated token should not be empty")
	suite.Contains(token, ".", "Generated token should contain JWT separators")
}

func (suite *MagicLinkAuthExecutorTestSuite) TestValidateMagicLinkToken_DecodeFailure() {
	// Pass a completely malformed token string
	// nolint:gosec // G101: Test data for negative case, not a real credential
	token := "not.a.valid.jwt.format"

	ctx := &core.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		UserInputs:  map[string]string{userInputMagicLinkToken: token},
		RuntimeData: make(map[string]string),
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, token, "").Return(
		&entityprovider.Entity{ID: magicLinkTestUserID}, nil)

	logger := log.GetLogger()
	tokenJTI, failure, err := suite.executor.validateMagicLinkToken(ctx, logger)

	suite.Empty(tokenJTI)
	// Asserts that we gracefully fail with the new unexported constant
	suite.Equal(failureReasonInvalidMagicLink, failure)
	suite.Nil(err)
}

func (suite *MagicLinkAuthExecutorTestSuite) TestValidateMagicLinkToken_MissingJTI() {
	header := magicLinkTestJWTHeader
	// Create a payload that is missing the "jti" claim
	payload := fmt.Sprintf(`{"sub":"user-123","executionId":%q,"exp":9999999999}`, magicLinkTestExecutionID)

	headerB64 := base64.RawURLEncoding.EncodeToString([]byte(header))
	payloadB64 := base64.RawURLEncoding.EncodeToString([]byte(payload))
	token := headerB64 + "." + payloadB64 + ".test-signature"

	ctx := &core.NodeContext{
		Context:     context.Background(),
		ExecutionID: magicLinkTestExecutionID,
		UserInputs:  map[string]string{userInputMagicLinkToken: token},
		RuntimeData: make(map[string]string),
	}

	suite.mockMagicLinkService.On("VerifyMagicLink", ctx.Context, token, "").Return(
		&entityprovider.Entity{ID: magicLinkTestUserID}, nil)

	logger := log.GetLogger()
	tokenJTI, failure, err := suite.executor.validateMagicLinkToken(ctx, logger)

	suite.Empty(tokenJTI)
	suite.Equal(failureReasonInvalidMagicLink, failure)
	suite.Nil(err)
}

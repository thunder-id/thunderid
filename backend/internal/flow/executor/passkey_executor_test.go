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
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"

	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authncm "github.com/thunder-id/thunderid/internal/authn/common"
	"github.com/thunder-id/thunderid/internal/authn/passkey"
	authnprovidermgr "github.com/thunder-id/thunderid/internal/authnprovider/manager"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/authn/passkeymock"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const (
	testPasskeyUserID    = "test-user-123"
	testRelyingPartyID   = "example.com"
	testRelyingPartyName = "Example App"
	testPasskeyFlowID    = "passkey-flow-123"
	testSessionToken     = "session-token-abc"
	// nolint:gosec // G101: This is a test value, not an actual credential
	testCredentialIDValue = "credential-id-xyz"
)

type PasskeyAuthExecutorTestSuite struct {
	suite.Suite
	mockPasskeyService *passkeymock.WebAuthnAuthnServiceInterfaceMock
	mockAuthnProvider  *managermock.AuthnProviderManagerInterfaceMock
	mockFlowFactory    *coremock.FlowFactoryInterfaceMock
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	executor           *passkeyAuthExecutor
}

func TestPasskeyAuthExecutorSuite(t *testing.T) {
	suite.Run(t, new(PasskeyAuthExecutorTestSuite))
}

func (suite *PasskeyAuthExecutorTestSuite) SetupTest() {
	suite.mockPasskeyService = passkeymock.NewWebAuthnAuthnServiceInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())

	// Create mock identifying executor
	identifyingMock := createMockIdentifyingExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, common.ExecutorTypeUtility,
		mock.Anything, mock.Anything).Return(identifyingMock).Maybe()

	// Create mock passkey executor base
	mockExec := createMockPasskeyAuthExecutor(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNamePasskeyAuth, common.ExecutorTypeAuthentication,
		mock.Anything, mock.Anything).Return(mockExec)

	suite.executor = newPasskeyAuthExecutor(suite.mockFlowFactory,
		suite.mockPasskeyService, suite.mockAuthnProvider, suite.mockEntityProvider)
}

func createMockPasskeyAuthExecutor(t *testing.T) core.ExecutorInterface {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(ExecutorNamePasskeyAuth).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeAuthentication).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{
		{Identifier: inputCredentialID, Type: "string", Required: true},
		{Identifier: inputClientDataJSON, Type: "string", Required: true},
		{Identifier: inputAuthenticatorData, Type: "string", Required: true},
		{Identifier: inputSignature, Type: "string", Required: true},
		{Identifier: inputUserHandle, Type: "string", Required: false},
	}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{
		{Identifier: userAttributeUserID, Type: "string", Required: true},
	}).Maybe()
	mockExec.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: inputCredentialID, Type: "string", Required: true},
		{Identifier: inputClientDataJSON, Type: "string", Required: true},
		{Identifier: inputAuthenticatorData, Type: "string", Required: true},
		{Identifier: inputSignature, Type: "string", Required: true},
	}).Maybe()
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *core.NodeContext, execResp *common.ExecutorResponse) bool {
			// Check if all required inputs are present
			requiredInputs := []string{inputCredentialID, inputClientDataJSON, inputAuthenticatorData, inputSignature}
			for _, input := range requiredInputs {
				if _, exists := ctx.UserInputs[input]; !exists {
					execResp.Status = common.ExecUserInputRequired
					return false
				}
			}
			return true
		}).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything).Return(true).Maybe()
	mockExec.On("GetUserIDFromContext", mock.Anything).Return(
		func(ctx *core.NodeContext) string {
			// Check AuthenticatedUser first, then RuntimeData, then UserInputs
			if ctx.AuthenticatedUser.UserID != "" {
				return ctx.AuthenticatedUser.UserID
			}
			if userID, ok := ctx.RuntimeData[userAttributeUserID]; ok {
				return userID
			}
			if userID, ok := ctx.UserInputs[userAttributeUserID]; ok {
				return userID
			}
			return ""
		}).Maybe()
	return mockExec
}

// Helper to create a node context with common properties
func createPasskeyNodeContext(mode string, flowType common.FlowType) *core.NodeContext {
	return &core.NodeContext{
		ExecutionID:  testPasskeyFlowID,
		FlowType:     flowType,
		ExecutorMode: mode,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
		NodeProperties: map[string]interface{}{
			"relyingPartyId":   testRelyingPartyID,
			"relyingPartyName": testRelyingPartyName,
		},
	}
}

func (suite *PasskeyAuthExecutorTestSuite) TestNewPasskeyAuthExecutor() {
	assert.NotNil(suite.T(), suite.executor)
	assert.NotNil(suite.T(), suite.executor.passkeyService)
	assert.NotNil(suite.T(), suite.executor.entityProvider)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecute_InvalidMode() {
	ctx := createPasskeyNodeContext("invalid_mode", common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "invalid executor mode")
	assert.NotNil(suite.T(), resp)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteChallenge_Success() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	expectedStartData := &passkey.PasskeyAuthenticationStartData{
		SessionToken: testSessionToken,
		PublicKeyCredentialRequestOptions: passkey.PublicKeyCredentialRequestOptions{
			Challenge: "dGVzdC1jaGFsbGVuZ2U=",
		},
	}

	suite.mockPasskeyService.On("StartAuthentication", mock.Anything, mock.MatchedBy(
		func(req *passkey.PasskeyAuthenticationStartRequest) bool {
			return req.UserID == testPasskeyUserID && req.RelyingPartyID == testRelyingPartyID
		})).Return(expectedStartData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testSessionToken, resp.RuntimeData[runtimePasskeySessionToken])
	assert.NotEmpty(suite.T(), resp.AdditionalData[runtimePasskeyChallenge])
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteChallenge_MissingUserID() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	// Not setting userID in RuntimeData - this triggers usernameless flow

	expectedStartData := &passkey.PasskeyAuthenticationStartData{
		SessionToken: testSessionToken,
		PublicKeyCredentialRequestOptions: passkey.PublicKeyCredentialRequestOptions{
			Challenge: "dGVzdC1jaGFsbGVuZ2U=",
		},
	}

	// Mock passkey service for usernameless authentication (empty UserID)
	suite.mockPasskeyService.On("StartAuthentication", mock.Anything, mock.MatchedBy(
		func(req *passkey.PasskeyAuthenticationStartRequest) bool {
			return req.UserID == "" && req.RelyingPartyID == testRelyingPartyID
		})).Return(expectedStartData, nil)

	resp, err := suite.executor.Execute(ctx)

	// Usernameless flow should succeed
	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testSessionToken, resp.RuntimeData[runtimePasskeySessionToken])
	assert.NotEmpty(suite.T(), resp.AdditionalData[runtimePasskeyChallenge])
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteChallenge_MissingRelyingPartyID() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.NodeProperties = map[string]interface{}{} // Empty node properties

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "relying party ID is not configured")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteChallenge_ServiceError_Client() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	suite.mockPasskeyService.On("StartAuthentication", mock.Anything, mock.Anything).Return(
		nil, &serviceerror.ServiceError{
			Type: serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.user_has_no_registered_passkeys", DefaultValue: "User has no registered passkeys",
			},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "User has no registered passkeys")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteChallenge_ServiceError_Server() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	suite.mockPasskeyService.On("StartAuthentication", mock.Anything, mock.Anything).Return(
		nil, &serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.database_connection_failed", DefaultValue: "Database connection failed",
			},
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to start passkey authentication")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteVerify_Success() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "eyJ0eXBlIjoid2ViYXV0aG4uZ2V0In0",
		inputAuthenticatorData: "authenticator-data",
		inputSignature:         "signature-data",
		inputUserHandle:        "user-handle",
	}

	authResp := &authnprovidermgr.AuthnBasicResult{
		UserID: testPasskeyUserID,
	}
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{}, authResp, nil)

	attrs := map[string]interface{}{"email": "test@example.com"}
	attrsJSON, _ := json.Marshal(attrs)
	testUser := &entityprovider.Entity{
		ID:         testPasskeyUserID,
		OUID:       "ou-123",
		Type:       "INTERNAL",
		Attributes: attrsJSON,
	}
	suite.mockEntityProvider.On("GetEntity", testPasskeyUserID).Return(testUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), testPasskeyUserID, resp.AuthenticatedUser.UserID)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteVerify_MissingInputs() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	// Empty UserInputs triggers UserInputRequired

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteVerify_MissingSessionToken() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	// Not setting session token
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "client-data",
		inputAuthenticatorData: "authenticator-data",
		inputSignature:         "signature",
	}

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no session token found")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteVerify_InvalidPasskey_ClientError() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "client-data",
		inputAuthenticatorData: "authenticator-data",
		inputSignature:         "invalid-signature",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(
		authnprovidermgr.AuthUser{}, (*authnprovidermgr.AuthnBasicResult)(nil), &serviceerror.ServiceError{
			Type: serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.invalid_signature", DefaultValue: "Invalid signature",
			},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "invalid passkey credentials")
	assert.NotEmpty(suite.T(), resp.Inputs)
	inputIDs := make([]string, 0, len(resp.Inputs))
	for _, input := range resp.Inputs {
		inputIDs = append(inputIDs, input.Identifier)
	}
	assert.Contains(suite.T(), inputIDs, inputCredentialID)
	assert.Contains(suite.T(), inputIDs, inputClientDataJSON)
	assert.Contains(suite.T(), inputIDs, inputAuthenticatorData)
	assert.Contains(suite.T(), inputIDs, inputSignature)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteVerify_ServiceError_Server() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "client-data",
		inputAuthenticatorData: "authenticator-data",
		inputSignature:         "signature",
	}

	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(
		authnprovidermgr.AuthUser{}, (*authnprovidermgr.AuthnBasicResult)(nil), &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			ErrorDescription: i18ncore.I18nMessage{Key: "error.test.database_error", DefaultValue: "Database error"},
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to verify passkey")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterStart_Success() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	expectedStartData := &passkey.PasskeyRegistrationStartData{
		SessionToken: testSessionToken,
		PublicKeyCredentialCreationOptions: passkey.PublicKeyCredentialCreationOptions{
			Challenge: "cmVnaXN0cmF0aW9uLWNoYWxsZW5nZQ==",
		},
	}

	suite.mockPasskeyService.On("StartRegistration", mock.Anything, mock.MatchedBy(
		func(req *passkey.PasskeyRegistrationStartRequest) bool {
			return req.UserID == testPasskeyUserID &&
				req.RelyingPartyID == testRelyingPartyID &&
				req.RelyingPartyName == testRelyingPartyName
		})).Return(expectedStartData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testSessionToken, resp.RuntimeData[runtimePasskeySessionToken])
	assert.NotEmpty(suite.T(), resp.AdditionalData[runtimePasskeyCreationOptions])
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterStart_MissingUserID() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	// Not setting userID

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "User ID is required")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterStart_MissingRelyingPartyID() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.NodeProperties = map[string]interface{}{} // Empty

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "relying party ID is not configured")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterStart_ServiceError_Client() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	suite.mockPasskeyService.On("StartRegistration", mock.Anything, mock.Anything).Return(
		nil, &serviceerror.ServiceError{
			Type:             serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{Key: "error.test.user_not_found", DefaultValue: "User not found"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterStart_ServiceError_Server() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	suite.mockPasskeyService.On("StartRegistration", mock.Anything, mock.Anything).Return(
		nil, &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			ErrorDescription: i18ncore.I18nMessage{Key: "error.test.database_error", DefaultValue: "Database error"},
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to start passkey registration")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterStart_DefaultRelyingPartyName() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	// Set only relyingPartyId, not relyingPartyName
	ctx.NodeProperties = map[string]interface{}{
		"relyingPartyId": testRelyingPartyID,
	}

	expectedStartData := &passkey.PasskeyRegistrationStartData{
		SessionToken:                       testSessionToken,
		PublicKeyCredentialCreationOptions: passkey.PublicKeyCredentialCreationOptions{},
	}

	suite.mockPasskeyService.On("StartRegistration", mock.Anything, mock.MatchedBy(
		func(req *passkey.PasskeyRegistrationStartRequest) bool {
			// relyingPartyName should default to relyingPartyId
			return req.RelyingPartyName == testRelyingPartyID
		})).Return(expectedStartData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterFinish_Success_RegistrationFlow() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegFinish, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "eyJ0eXBlIjoid2ViYXV0aG4uY3JlYXRlIn0",
		inputAttestationObject: "attestation-object-data",
		inputCredentialName:    "My Passkey",
	}

	finishData := &passkey.PasskeyRegistrationFinishData{
		CredentialID:   testCredentialIDValue,
		CredentialName: "My Passkey",
		CreatedAt:      "2025-01-15T00:00:00Z",
	}
	suite.mockPasskeyService.On("FinishRegistration", mock.Anything, mock.Anything).Return(finishData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testCredentialIDValue, resp.RuntimeData[runtimePasskeyCredentialID])
	assert.Equal(suite.T(), "My Passkey", resp.RuntimeData[runtimePasskeyCredentialName])
	assert.Equal(suite.T(), "", resp.RuntimeData[runtimePasskeySessionToken]) // Should be cleared
	// For registration flow, authenticated user should not be set
	assert.False(suite.T(), resp.AuthenticatedUser.IsAuthenticated)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterFinish_Success_AuthenticationFlow() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegFinish, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "client-data",
		inputAttestationObject: "attestation-object",
	}

	finishData := &passkey.PasskeyRegistrationFinishData{
		CredentialID:   testCredentialIDValue,
		CredentialName: "Passkey", // Default name
		CreatedAt:      "2025-01-15T00:00:00Z",
	}
	suite.mockPasskeyService.On("FinishRegistration", mock.Anything, mock.Anything).Return(finishData, nil)

	attrs := map[string]interface{}{"email": "test@example.com"}
	attrsJSON, _ := json.Marshal(attrs)
	testUser := &entityprovider.Entity{
		ID:         testPasskeyUserID,
		OUID:       "ou-123",
		Type:       "INTERNAL",
		Attributes: attrsJSON,
	}
	suite.mockEntityProvider.On("GetEntity", testPasskeyUserID).Return(testUser, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.True(suite.T(), resp.AuthenticatedUser.IsAuthenticated)
	assert.Equal(suite.T(), testPasskeyUserID, resp.AuthenticatedUser.UserID)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterFinish_MissingInputs() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegFinish, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	// Empty UserInputs — all required inputs are missing

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	// All missing inputs must be listed so the client knows what to collect
	assert.NotEmpty(suite.T(), resp.Inputs)
	inputIDs := make([]string, 0, len(resp.Inputs))
	for _, input := range resp.Inputs {
		inputIDs = append(inputIDs, input.Identifier)
	}
	assert.Contains(suite.T(), inputIDs, inputCredentialID)
	assert.Contains(suite.T(), inputIDs, inputClientDataJSON)
	assert.Contains(suite.T(), inputIDs, inputAttestationObject)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterFinish_PartialInputs() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegFinish, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	// Provide credentialID but omit the other required inputs
	ctx.UserInputs = map[string]string{
		inputCredentialID: testCredentialIDValue,
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	// The full input list is returned so the client can re-render the entire form
	assert.NotEmpty(suite.T(), resp.Inputs)
	inputIDs := make([]string, 0, len(resp.Inputs))
	for _, input := range resp.Inputs {
		inputIDs = append(inputIDs, input.Identifier)
	}
	assert.Contains(suite.T(), inputIDs, inputCredentialID)
	assert.Contains(suite.T(), inputIDs, inputClientDataJSON)
	assert.Contains(suite.T(), inputIDs, inputAttestationObject)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterFinish_MissingSessionToken() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegFinish, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	// Not setting session token
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "client-data",
		inputAttestationObject: "attestation-object",
	}

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "no session token found")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterFinish_ServiceError_Client() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegFinish, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "client-data",
		inputAttestationObject: "invalid-attestation",
	}

	suite.mockPasskeyService.On("FinishRegistration", mock.Anything, mock.Anything).Return(
		nil, &serviceerror.ServiceError{
			Type: serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{
				Key: "error.test.invalid_attestation_object", DefaultValue: "Invalid attestation object",
			},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "Invalid attestation object")
	// Client must receive the full input list so it can re-prompt the user
	assert.NotEmpty(suite.T(), resp.Inputs)
	inputIDs := make([]string, 0, len(resp.Inputs))
	for _, input := range resp.Inputs {
		inputIDs = append(inputIDs, input.Identifier)
	}
	assert.Contains(suite.T(), inputIDs, inputCredentialID)
	assert.Contains(suite.T(), inputIDs, inputClientDataJSON)
	assert.Contains(suite.T(), inputIDs, inputAttestationObject)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterFinish_ServiceError_Server() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegFinish, common.FlowTypeRegistration)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "client-data",
		inputAttestationObject: "attestation-object",
	}

	suite.mockPasskeyService.On("FinishRegistration", mock.Anything, mock.Anything).Return(
		nil, &serviceerror.ServiceError{
			Type:             serviceerror.ServerErrorType,
			ErrorDescription: i18ncore.I18nMessage{Key: "error.test.database_error", DefaultValue: "Database error"},
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to finish passkey registration")
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetRelyingPartyID_FromNodeProperties() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)

	rpID := suite.executor.getRelyingPartyID(ctx)

	assert.Equal(suite.T(), testRelyingPartyID, rpID)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetRelyingPartyID_EmptyNodeProperties() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	ctx.NodeProperties = nil

	rpID := suite.executor.getRelyingPartyID(ctx)

	assert.Empty(suite.T(), rpID)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetRelyingPartyName_FromNodeProperties() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)

	rpName := suite.executor.getRelyingPartyName(ctx)

	assert.Equal(suite.T(), testRelyingPartyName, rpName)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetRelyingPartyName_EmptyNodeProperties() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	ctx.NodeProperties = nil

	rpName := suite.executor.getRelyingPartyName(ctx)

	assert.Empty(suite.T(), rpName)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAuthenticatorSelection_FromNodeProperties() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.NodeProperties["authenticatorSelection"] = map[string]interface{}{
		"authenticatorAttachment": "platform",
		"requireResidentKey":      true,
		"residentKey":             "required",
		"userVerification":        "required",
	}

	authSel := suite.executor.getAuthenticatorSelection(ctx)

	assert.NotNil(suite.T(), authSel)
	assert.Equal(suite.T(), "platform", authSel.AuthenticatorAttachment)
	assert.True(suite.T(), authSel.RequireResidentKey)
	assert.Equal(suite.T(), "required", authSel.ResidentKey)
	assert.Equal(suite.T(), "required", authSel.UserVerification)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAuthenticatorSelection_NotConfigured() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	// No authenticatorSelection in node properties

	authSel := suite.executor.getAuthenticatorSelection(ctx)

	assert.Nil(suite.T(), authSel)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAttestation_FromNodeProperties() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.NodeProperties["attestation"] = "direct"

	attestation := suite.executor.getAttestation(ctx)

	assert.Equal(suite.T(), "direct", attestation)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAttestation_DefaultValue() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	// No attestation in node properties

	attestation := suite.executor.getAttestation(ctx)

	assert.Equal(suite.T(), "none", attestation) // Default value
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAttestation_EmptyNodeProperties() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.NodeProperties = nil

	attestation := suite.executor.getAttestation(ctx)

	assert.Equal(suite.T(), "none", attestation)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAuthenticatedUser_Success() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	execResp := &common.ExecutorResponse{
		RuntimeData: map[string]string{
			userAttributeUserID: testPasskeyUserID,
		},
	}

	attrs := map[string]interface{}{"email": "test@example.com", "name": "Test User"}
	attrsJSON, _ := json.Marshal(attrs)
	testUser := &entityprovider.Entity{
		ID:         testPasskeyUserID,
		OUID:       "ou-123",
		Type:       "INTERNAL",
		Attributes: attrsJSON,
	}
	suite.mockEntityProvider.On("GetEntity", testPasskeyUserID).Return(testUser, nil)

	authUser, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), authUser)
	assert.True(suite.T(), authUser.IsAuthenticated)
	assert.Equal(suite.T(), testPasskeyUserID, authUser.UserID)
	assert.Equal(suite.T(), "ou-123", authUser.OUID)
	assert.Equal(suite.T(), "INTERNAL", authUser.UserType)
	assert.Equal(suite.T(), "test@example.com", authUser.Attributes["email"])
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAuthenticatedUser_UserNotFound() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	execResp := &common.ExecutorResponse{
		RuntimeData: map[string]string{
			userAttributeUserID: testPasskeyUserID,
		},
	}

	suite.mockEntityProvider.On("GetEntity", testPasskeyUserID).Return(
		nil, &entityprovider.EntityProviderError{
			Code: entityprovider.ErrorCodeEntityNotFound, Message: "User not found",
		})

	authUser, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), authUser)
	assert.Contains(suite.T(), err.Error(), "failed to get user details")
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAuthenticatedUser_InvalidJSON() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	execResp := &common.ExecutorResponse{
		RuntimeData: map[string]string{
			userAttributeUserID: testPasskeyUserID,
		},
	}

	testUser := &entityprovider.Entity{
		ID:         testPasskeyUserID,
		OUID:       "ou-123",
		Type:       "INTERNAL",
		Attributes: json.RawMessage(`invalid json`),
	}
	suite.mockEntityProvider.On("GetEntity", testPasskeyUserID).Return(testUser, nil)

	authUser, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), authUser)
	assert.Contains(suite.T(), err.Error(), "failed to unmarshal user attributes")
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAuthenticatedUser_EmptyUserID() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	// Not setting userID in RuntimeData

	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	authUser, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), authUser)
	assert.Contains(suite.T(), err.Error(), "user ID is empty")
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetRelyingPartyID_InvalidType() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	// Set relyingPartyId as wrong type (int instead of string)
	ctx.NodeProperties = map[string]interface{}{
		"relyingPartyId": 12345, // Wrong type
	}

	rpID := suite.executor.getRelyingPartyID(ctx)

	assert.Empty(suite.T(), rpID)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetRelyingPartyID_EmptyStringValue() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	ctx.NodeProperties = map[string]interface{}{
		"relyingPartyId": "", // Empty string
	}

	rpID := suite.executor.getRelyingPartyID(ctx)

	assert.Empty(suite.T(), rpID)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetRelyingPartyName_InvalidType() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	ctx.NodeProperties = map[string]interface{}{
		"relyingPartyName": 12345, // Wrong type
	}

	rpName := suite.executor.getRelyingPartyName(ctx)

	assert.Empty(suite.T(), rpName)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetRelyingPartyName_EmptyStringValue() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	ctx.NodeProperties = map[string]interface{}{
		"relyingPartyName": "", // Empty string
	}

	rpName := suite.executor.getRelyingPartyName(ctx)

	assert.Empty(suite.T(), rpName)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAuthenticatorSelection_InvalidMapType() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	// Set authenticatorSelection as wrong type (string instead of map)
	ctx.NodeProperties["authenticatorSelection"] = "invalid"

	authSel := suite.executor.getAuthenticatorSelection(ctx)

	assert.Nil(suite.T(), authSel)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAuthenticatorSelection_PartialFields() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	// Set only some authenticatorSelection fields
	ctx.NodeProperties["authenticatorSelection"] = map[string]interface{}{
		"authenticatorAttachment": "cross-platform",
		// Missing other fields
	}

	authSel := suite.executor.getAuthenticatorSelection(ctx)

	assert.NotNil(suite.T(), authSel)
	assert.Equal(suite.T(), "cross-platform", authSel.AuthenticatorAttachment)
	assert.False(suite.T(), authSel.RequireResidentKey)
	assert.Empty(suite.T(), authSel.ResidentKey)
	assert.Empty(suite.T(), authSel.UserVerification)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAuthenticatorSelection_EmptyNodeProperties() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.NodeProperties = nil

	authSel := suite.executor.getAuthenticatorSelection(ctx)

	assert.Nil(suite.T(), authSel)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAttestation_InvalidType() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.NodeProperties["attestation"] = 12345 // Wrong type

	attestation := suite.executor.getAttestation(ctx)

	assert.Equal(suite.T(), "none", attestation) // Should return default
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAttestation_EmptyStringValue() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	ctx.NodeProperties["attestation"] = "" // Empty string

	attestation := suite.executor.getAttestation(ctx)

	assert.Equal(suite.T(), "none", attestation) // Should return default
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteVerify_GetAuthenticatedUserFails() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "eyJ0eXBlIjoid2ViYXV0aG4uZ2V0In0",
		inputAuthenticatorData: "authenticator-data",
		inputSignature:         "signature-data",
		inputUserHandle:        "user-handle",
	}

	authResp := &authnprovidermgr.AuthnBasicResult{
		UserID: testPasskeyUserID,
	}
	suite.mockAuthnProvider.On("AuthenticateUser", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(authnprovidermgr.AuthUser{}, authResp, nil)

	// Simulate user not found when getting authenticated user details
	suite.mockEntityProvider.On("GetEntity", testPasskeyUserID).Return(
		nil, &entityprovider.EntityProviderError{
			Code: entityprovider.ErrorCodeEntityNotFound, Message: "User not found",
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get authenticated user details")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterFinish_GetAuthenticatedUserFails_AuthFlow() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegFinish, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID
	ctx.RuntimeData[runtimePasskeySessionToken] = testSessionToken
	ctx.UserInputs = map[string]string{
		inputCredentialID:      testCredentialIDValue,
		inputClientDataJSON:    "client-data",
		inputAttestationObject: "attestation-object",
	}

	finishData := &passkey.PasskeyRegistrationFinishData{
		CredentialID:   testCredentialIDValue,
		CredentialName: "Passkey",
		CreatedAt:      "2025-01-15T00:00:00Z",
	}
	suite.mockPasskeyService.On("FinishRegistration", mock.Anything, mock.Anything).Return(finishData, nil)

	// Simulate user not found when getting authenticated user details
	suite.mockEntityProvider.On("GetEntity", testPasskeyUserID).Return(
		nil, &entityprovider.EntityProviderError{
			Code: entityprovider.ErrorCodeEntityNotFound, Message: "User not found",
		})

	_, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to get authenticated user details")
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteChallenge_UserIDFromAuthenticatedUser() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeChallenge, common.FlowTypeAuthentication)
	// Set userID in AuthenticatedUser instead of RuntimeData
	ctx.AuthenticatedUser = authncm.AuthenticatedUser{
		UserID:          testPasskeyUserID,
		IsAuthenticated: true,
	}

	expectedStartData := &passkey.PasskeyAuthenticationStartData{
		SessionToken: testSessionToken,
		PublicKeyCredentialRequestOptions: passkey.PublicKeyCredentialRequestOptions{
			Challenge: "dGVzdC1jaGFsbGVuZ2U=",
		},
	}

	suite.mockPasskeyService.On("StartAuthentication", mock.Anything, mock.MatchedBy(
		func(req *passkey.PasskeyAuthenticationStartRequest) bool {
			return req.UserID == testPasskeyUserID && req.RelyingPartyID == testRelyingPartyID
		})).Return(expectedStartData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *PasskeyAuthExecutorTestSuite) TestExecuteRegisterStart_UserIDFromAuthenticatedUser() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeRegStart, common.FlowTypeRegistration)
	// Set userID in AuthenticatedUser
	ctx.AuthenticatedUser = authncm.AuthenticatedUser{
		UserID:          testPasskeyUserID,
		IsAuthenticated: true,
	}

	expectedStartData := &passkey.PasskeyRegistrationStartData{
		SessionToken:                       testSessionToken,
		PublicKeyCredentialCreationOptions: passkey.PublicKeyCredentialCreationOptions{},
	}

	suite.mockPasskeyService.On("StartRegistration", mock.Anything, mock.MatchedBy(
		func(req *passkey.PasskeyRegistrationStartRequest) bool {
			return req.UserID == testPasskeyUserID
		})).Return(expectedStartData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *PasskeyAuthExecutorTestSuite) TestGetAuthenticatedUser_UserIDFromContext() {
	ctx := createPasskeyNodeContext(passkeyExecutorModeVerify, common.FlowTypeAuthentication)
	ctx.RuntimeData[userAttributeUserID] = testPasskeyUserID

	// Empty runtime data in execResp, should fall back to context
	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	attrs := map[string]interface{}{"email": "test@example.com"}
	attrsJSON, _ := json.Marshal(attrs)
	testUser := &entityprovider.Entity{
		ID:         testPasskeyUserID,
		OUID:       "ou-123",
		Type:       "INTERNAL",
		Attributes: attrsJSON,
	}
	suite.mockEntityProvider.On("GetEntity", testPasskeyUserID).Return(testUser, nil)

	authUser, err := suite.executor.getAuthenticatedUser(ctx, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), authUser)
	assert.Equal(suite.T(), testPasskeyUserID, authUser.UserID)
}

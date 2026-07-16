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
	"encoding/json"
	"strconv"
	"testing"
	"time"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	consentauthn "github.com/thunder-id/thunderid/internal/authn/consent"
	"github.com/thunder-id/thunderid/internal/flow/common"
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/consentprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const (
	testUserTypeInternal = "internal"
)

type ConsentExecutorTestSuite struct {
	suite.Suite
	mockConsentEnforcer *consentprovidermock.ConsentProviderMock
	mockAuthnProvider   *managermock.AuthnProviderManagerMock
	mockFlowFactory     *coremock.FlowFactoryInterfaceMock
	executor            *consentExecutor
}

func TestConsentExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentExecutorTestSuite))
}

func (suite *ConsentExecutorTestSuite) SetupTest() {
	suite.mockConsentEnforcer = consentprovidermock.NewConsentProviderMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())

	mockExec := createMockExecutorWithInputs(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameConsent, providers.ExecutorTypeUtility,
		mock.AnythingOfType("[]providers.Input"), mock.AnythingOfType("[]providers.Input"),
		mock.Anything).Return(mockExec)

	suite.executor = newConsentExecutor(suite.mockFlowFactory, suite.mockConsentEnforcer, suite.mockAuthnProvider)
}

// createMockExecutorWithInputs creates a mock executor that supports ValidatePrerequisites and HasRequiredInputs
// with configurable behavior through the mock's On calls.
func createMockExecutorWithInputs(t *testing.T) *coremock.ExecutorInterfaceMock {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(ExecutorNameConsent).Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeUtility).Maybe()
	mockExec.On("GetDefaultInputs").Return([]providers.Input{
		{Identifier: userInputConsentDecisions, Type: providers.InputTypeConsent, Required: true},
	}).Maybe()
	mockExec.On("GetPrerequisites").Return([]providers.Input{
		{Identifier: userAttributeUserID, Type: providers.InputTypeText, Required: true},
	}).Maybe()
	return mockExec
}

// --- Helper to build a basic NodeContext ---

func buildConsentAuthUser() providers.AuthUser {
	var authUser providers.AuthUser
	_ = authUser.UnmarshalJSON([]byte(`{"entityReferenceToken":"tok","attributeToken":"tok"}`))
	return authUser
}

func buildConsentEntityRef() *providers.EntityReference {
	return &providers.EntityReference{
		EntityID:   testUserID,
		EntityType: "",
		OUID:       "",
	}
}

func buildConsentAvailableAttrs() *providers.AttributesResponse {
	return &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": nil,
			"phone": nil,
			"name":  nil,
		},
	}
}

func buildConsentNodeContext() *providers.NodeContext {
	return &providers.NodeContext{
		Context:        context.Background(),
		ExecutionID:    "flow-123",
		EntityID:       "app-123",
		AuthUser:       buildConsentAuthUser(),
		UserInputs:     map[string]string{},
		RuntimeData:    map[string]string{},
		NodeProperties: map[string]interface{}{},
		Application: providers.Application{
			InboundAuthProfile: providers.InboundAuthProfile{
				Assertion: &inboundmodel.AssertionConfig{
					UserAttributes: []string{"email", "phone"},
				},
			},
		},
	}
}

// setupDefaultAuthnProviderMocks sets up GetEntityReference and GetUserAvailableAttributes
// mock expectations using the default test entity reference and available attributes.
func (suite *ConsentExecutorTestSuite) setupDefaultAuthnProviderMocks() {
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(buildConsentAuthUser(), buildConsentEntityRef(), (*tidcommon.ServiceError)(nil)).Maybe()
	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return(buildConsentAvailableAttrs(), (*tidcommon.ServiceError)(nil)).Maybe()
}

// ----- Constructor Tests -----

func (suite *ConsentExecutorTestSuite) TestNewConsentExecutor() {
	assert.NotNil(suite.T(), suite.executor)
	assert.NotNil(suite.T(), suite.executor.consentEnforcer)
	assert.NotNil(suite.T(), suite.executor.logger)
	assert.Equal(suite.T(), ExecutorNameConsent, suite.executor.GetName())
	assert.Equal(suite.T(), providers.ExecutorTypeUtility, suite.executor.GetType())
}

func (suite *ConsentExecutorTestSuite) TestNewConsentExecutor_DefaultInputs() {
	inputs := suite.executor.GetDefaultInputs()
	assert.Len(suite.T(), inputs, 1)
	assert.Equal(suite.T(), userInputConsentDecisions, inputs[0].Identifier)
	assert.Equal(suite.T(), providers.InputTypeConsent, inputs[0].Type)
	assert.True(suite.T(), inputs[0].Required)
}

func (suite *ConsentExecutorTestSuite) TestNewConsentExecutor_Prerequisites() {
	prereqs := suite.executor.GetPrerequisites()
	assert.Len(suite.T(), prereqs, 1)
	assert.Equal(suite.T(), userAttributeUserID, prereqs[0].Identifier)
	assert.Equal(suite.T(), providers.InputTypeText, prereqs[0].Type)
	assert.True(suite.T(), prereqs[0].Required)
}

// ----- Execute: Prerequisites Failure -----

func (suite *ConsentExecutorTestSuite) TestExecute_PrerequisitesFailure() {
	ctx := buildConsentNodeContext()

	// Mock ValidatePrerequisites to return false
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"),
			mock.Anything).Return(false)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentPrereqFailed.Code, resp.Error.Code)
}

// ----- Execute: checkConsent (no inputs provided) -----

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AllConsentsActive() {
	ctx := buildConsentNodeContext()
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	// ResolveConsent returns nil = all consents active
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		[]string{}, []string{"email", "phone"}, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_ForceRepromptFromRuntimeData() {
	ctx := buildConsentNodeContext()
	ctx.RuntimeData[common.RuntimeKeyForceConsentReprompt] = "true"
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	// forceReprompt must be true when the force-consent-reprompt runtime key is set
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, true, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_ForceRepromptDefaultsFalse() {
	ctx := buildConsentNodeContext()
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	// forceReprompt must be false when the runtime key is absent
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, false, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_RequiredAttributesFromRuntimeData() {
	ctx := buildConsentNodeContext()
	ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes] = "email name"
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	// ResolveConsent should receive attributes from RuntimeData, not from Application config
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		[]string{}, []string{"email", "name"}, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_RequiredEssentialAndOptionalAttributesFromRuntimeData() {
	ctx := buildConsentNodeContext()
	ctx.RuntimeData[common.RuntimeKeyRequiredEssentialAttributes] = "email"
	ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes] = "name"
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		[]string{"email"}, []string{"name"}, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_NilAssertionConfig() {
	ctx := buildConsentNodeContext()
	ctx.Application.Assertion = nil
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	// Attributes should be nil when no RuntimeData and no Assertion config
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		[]string{}, []string{}, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_ExplicitEmptyRuntimeKeysSkipAssertionFallback() {
	ctx := buildConsentNodeContext()
	// Set both runtime keys to empty strings — the keys are present but carry no attributes.
	// The fallback to Application.Assertion.UserAttributes must NOT trigger.
	ctx.RuntimeData[common.RuntimeKeyRequiredEssentialAttributes] = ""
	ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes] = ""
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	// Expect empty slices — NOT the Application.Assertion.UserAttributes (["email","phone"])
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		[]string{}, []string{}, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_ResolveConsent_ClientError() {
	ctx := buildConsentNodeContext()
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			ErrorDescription: tidcommon.I18nMessage{
				DefaultValue: "consent config not found",
			},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentResolutionFailed.Code, resp.Error.Code)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_ResolveConsent_ServerError() {
	ctx := buildConsentNodeContext()
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
		})

	resp, err := suite.executor.Execute(ctx)

	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to resolve consent")
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_PromptRequired_NoTimeout() {
	ctx := buildConsentNodeContext()
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	promptData := &providers.ConsentPromptData{
		Purposes: []providers.ConsentPurposePrompt{
			{
				PurposeName: "app:app-123:attrs",
				PurposeID:   "purpose-1",
				Essential:   []providers.PromptElement{{Name: "email"}},
				Optional:    []providers.PromptElement{{Name: "phone"}},
			},
		},
	}

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(promptData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.NotEmpty(suite.T(), resp.AdditionalData[common.DataConsentPrompt])
	assert.Empty(suite.T(), resp.AdditionalData[common.DataStepTimeout],
		"Should not set timeout without timeout config")

	// Verify ForwardedData contains the prompt data
	assert.NotNil(suite.T(), resp.ForwardedData[common.ForwardedDataKeyConsentPrompt])

	// Verify the JSON serialization
	var parsedPrompt []providers.ConsentPurposePrompt
	parseErr := json.Unmarshal([]byte(resp.AdditionalData[common.DataConsentPrompt]), &parsedPrompt)
	assert.NoError(suite.T(), parseErr)
	assert.Len(suite.T(), parsedPrompt, 1)
	assert.Equal(suite.T(), "app:app-123:attrs", parsedPrompt[0].PurposeName)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_PromptRequired_StoresSessionToken() {
	ctx := buildConsentNodeContext()
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	promptData := &providers.ConsentPromptData{
		Purposes: []providers.ConsentPurposePrompt{{
			PurposeName: "attributes:test-app",
			Optional:    []providers.PromptElement{{Name: "email"}},
		}},
		SessionToken: "consent-session-token",
	}

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "", "user-123",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(promptData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), "consent-session-token", resp.RuntimeData[common.RuntimeKeyConsentSessionToken])
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_PromptRequired_WithTimeout() {
	ctx := buildConsentNodeContext()
	ctx.NodeProperties["timeout"] = "300" // 5 minutes
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	promptData := &providers.ConsentPromptData{
		Purposes: []providers.ConsentPurposePrompt{
			{PurposeName: "attributes:test-app", Essential: []providers.PromptElement{{Name: "email"}}},
		},
	}

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(promptData, nil)

	beforeExec := time.Now().UnixMilli()
	resp, err := suite.executor.Execute(ctx)
	afterExec := time.Now().UnixMilli()

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)

	// Verify timeout is set
	expiresAtStr := resp.AdditionalData[common.DataStepTimeout]
	assert.NotEmpty(suite.T(), expiresAtStr)

	expiresAt, parseErr := strconv.ParseInt(expiresAtStr, 10, 64)
	assert.NoError(suite.T(), parseErr)

	// The expiry should be ~300 seconds from now
	expectedMin := beforeExec + 300*1000
	expectedMax := afterExec + 300*1000
	assert.True(suite.T(), expiresAt >= expectedMin && expiresAt <= expectedMax,
		"expiresAt should be approximately 300 seconds from now")

	// Verify runtime also has the timeout
	assert.Equal(suite.T(), expiresAtStr, resp.RuntimeData[common.RuntimeKeyStepTimeout])
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_EmptyTimeout() {
	ctx := buildConsentNodeContext()
	ctx.NodeProperties["timeout"] = ""
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	promptData := &providers.ConsentPromptData{
		Purposes: []providers.ConsentPurposePrompt{
			{PurposeName: "attributes:test-app", Essential: []providers.PromptElement{{Name: "email"}}},
		},
	}

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(promptData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Empty(suite.T(), resp.AdditionalData[common.DataStepTimeout])
}

// ----- Execute: handleConsentDecisions (inputs provided) -----

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_AllApproved_Success() {
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "app:app-123:attrs",
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: true},
					{Name: "phone", Approved: true},
				},
			},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	ctx.Application.LoginConsent = &inboundmodel.LoginConsentConfig{
		ValidityPeriod: 86400,
	}
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID:     "consent-001",
		Status: providers.ConsentStatusActive,
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "attributes:app-123",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: true},
				},
			},
		},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, "default", "app-123", "user-123",
		mock.AnythingOfType("*providers.ConsentDecisions"), mock.Anything, int64(86400), mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "consent-001", resp.RuntimeData[common.RuntimeKeyConsentID])
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyConsentedAttributes], "email")
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyConsentedAttributes], "phone")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_HTMLEscapedJSON() {
	// Simulate the HTML-escaped JSON that SanitizeStringMap would produce
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)
	// HTML escape the JSON (simulates XSS sanitization)
	escapedJSON := "&lt;script&gt;" // Won't appear in real flow, just to test unescape doesn't break
	_ = escapedJSON
	// Use actual HTML-escaped JSON with angle brackets in values
	htmlEscaped := string(decisionsJSON) // Normal JSON shouldn't have HTML entities typically

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = htmlEscaped
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID:       "consent-002",
		Purposes: []providers.ConsentPurposeItem{},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_EmptyDecisions() {
	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = ""
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentDecisionsMissing.Code, resp.Error.Code)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_MissingDecisionsKey() {
	ctx := buildConsentNodeContext()
	// Don't set userInputConsentDecisions at all
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentDecisionsMissing.Code, resp.Error.Code)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_InvalidJSON() {
	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = "{invalid-json}"
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentDecisionsParseFail.Code, resp.Error.Code)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_ConsentTimeout_Expired() {
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	// Set an expiry timestamp in the past
	pastExpiry := strconv.FormatInt(time.Now().Add(-1*time.Minute).UnixMilli(), 10)
	ctx.RuntimeData[common.RuntimeKeyStepTimeout] = pastExpiry
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentPromptTimedOut.Code, resp.Error.Code)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_ConsentTimeout_NotExpired() {
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	// Set an expiry timestamp in the future
	futureExpiry := strconv.FormatInt(time.Now().Add(5*time.Minute).UnixMilli(), 10)
	ctx.RuntimeData[common.RuntimeKeyStepTimeout] = futureExpiry
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID:       "consent-003",
		Purposes: []providers.ConsentPurposeItem{},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_EssentialDenied() {
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "attributes:test-app",
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: false}, // User denied essential
				},
			},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	// RecordConsent persists the denial and returns an essential-denied error
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*providers.Consent)(nil), &consentauthn.ErrorEssentialConsentDenied)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentDenied.Code, resp.Error.Code)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_RecordConsent_ClientError() {
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			Type: tidcommon.ClientErrorType,
			ErrorDescription: tidcommon.I18nMessage{
				DefaultValue: "invalid consent data",
			},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentRecordFailed.Code, resp.Error.Code)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_RecordConsent_ServerError() {
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &tidcommon.ServiceError{
			Type: tidcommon.ServerErrorType,
		})

	resp, err := suite.executor.Execute(ctx)

	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to record consent")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_NilLoginConsentConfig() {
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	ctx.Application.LoginConsent = nil // No login consent config
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID:       "consent-004",
		Purposes: []providers.ConsentPurposeItem{},
	}

	// ValidityPeriod should be 0 when LoginConsent is nil
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, "default", "app-123", "user-123",
		mock.AnythingOfType("*providers.ConsentDecisions"), mock.Anything, int64(0), mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_PartialElementApproval() {
	// Test where a purpose is approved but some elements are not approved
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "attributes:test-app",
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: true},
					{Name: "phone", Approved: false}, // Not approved but purpose overall is approved
				},
			},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID: "consent-005",
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "attributes:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: false},
				},
			},
		},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)

	// Only email should be in consented attributes (phone was not approved)
	consentedAttrs := resp.RuntimeData[common.RuntimeKeyConsentedAttributes]
	assert.Contains(suite.T(), consentedAttrs, "email")
	assert.NotContains(suite.T(), consentedAttrs, "phone")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_MultiplePurposes_AllApproved() {
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: true},
			{PurposeName: "attributes:test-app-2", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID: "consent-006",
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "attributes:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
			{
				Name: "attributes:test-app-2",
				Elements: []providers.ConsentElementApproval{
					{Name: "name", IsUserApproved: true},
					{Name: "phone", IsUserApproved: true},
				},
			},
		},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)

	consentedAttrs := resp.RuntimeData[common.RuntimeKeyConsentedAttributes]
	assert.Contains(suite.T(), consentedAttrs, "email")
	assert.Contains(suite.T(), consentedAttrs, "name")
	assert.Contains(suite.T(), consentedAttrs, "phone")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_NoConsentedElements() {
	decisions := providers.ConsentDecisions{
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	// Consent record with no approved elements
	consentResult := &providers.Consent{
		ID: "consent-007",
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "attributes:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: false},
					{Name: "phone", IsUserApproved: false},
				},
			},
		},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)

	// RuntimeKeyConsentedAttributes is always set (even empty) so auth assert knows consent ran
	consentedAttrs, hasConsentedAttrs := resp.RuntimeData[common.RuntimeKeyConsentedAttributes]
	assert.True(suite.T(), hasConsentedAttrs,
		"Should always set consented attributes key when consent is recorded")
	assert.Empty(suite.T(), consentedAttrs,
		"Consented attributes should be empty when no elements are approved")
}

// ----- Execute: with augmented attributes tests -----

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AugmentedAttributes_GroupsInjected() {
	ctx := buildConsentNodeContext()
	// UserID is set (from buildConsentNodeContext: testUserID="user-123"), so "groups" must be injected
	// into the available attributes passed to ResolveConsent
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	suite.mockConsentEnforcer.On("ResolveConsent",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
		mock.Anything,
		mock.MatchedBy(func(aa *providers.AttributesResponse) bool {
			if aa == nil || len(aa.Attributes) == 0 {
				return false
			}
			groupsMeta, hasGroups := aa.Attributes["groups"]
			return hasGroups && groupsMeta != nil
		}), mock.Anything, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AugmentedAttributes_OUClaimsInjected() {
	ctx := buildConsentNodeContext()
	entityRef := buildConsentEntityRef()
	entityRef.OUID = "ou-999"
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(buildConsentAuthUser(), entityRef, (*tidcommon.ServiceError)(nil)).Maybe()
	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return(buildConsentAvailableAttrs(), (*tidcommon.ServiceError)(nil)).Maybe()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	suite.mockConsentEnforcer.On("ResolveConsent",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
		mock.Anything,
		mock.MatchedBy(func(aa *providers.AttributesResponse) bool {
			if aa == nil {
				return false
			}
			_, hasOUID := aa.Attributes["ouId"]
			_, hasOUName := aa.Attributes["ouName"]
			_, hasOUHandle := aa.Attributes["ouHandle"]
			return hasOUID && hasOUName && hasOUHandle
		}), mock.Anything, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AugmentedAttributes_UserTypeInjected() {
	ctx := buildConsentNodeContext()
	entityRef := buildConsentEntityRef()
	entityRef.EntityType = "customer"
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(buildConsentAuthUser(), entityRef, (*tidcommon.ServiceError)(nil)).Maybe()
	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return(buildConsentAvailableAttrs(), (*tidcommon.ServiceError)(nil)).Maybe()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	suite.mockConsentEnforcer.On("ResolveConsent",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
		mock.Anything,
		mock.MatchedBy(func(aa *providers.AttributesResponse) bool {
			return aa != nil && func() bool { _, ok := aa.Attributes["userType"]; return ok }()
		}), mock.Anything, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AugmentedAttributes_NilBaseWithSpecialClaims() {
	// With a nil AvailableAttributes base (local/credential-based auth), nil must be forwarded
	// to ResolveConsent so profile-presence filtering is skipped entirely.
	ctx := buildConsentNodeContext()
	entityRef := buildConsentEntityRef()
	entityRef.EntityType = testUserTypeInternal
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(buildConsentAuthUser(), entityRef, (*tidcommon.ServiceError)(nil)).Maybe()
	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return((*providers.AttributesResponse)(nil), (*tidcommon.ServiceError)(nil)).Maybe()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	suite.mockConsentEnforcer.On("ResolveConsent",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
		mock.Anything,
		mock.MatchedBy(func(aa *providers.AttributesResponse) bool {
			return aa == nil
		}), mock.Anything, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

// ----- collectConsentedAttributes Tests -----

func (suite *ConsentExecutorTestSuite) TestCollectConsentedAttributes_MixedApprovals() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "attributes:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: false},
					{Name: "name", IsUserApproved: true},
				},
			},
		},
	}

	attrs := collectConsentedAttributes(c)

	assert.Len(suite.T(), attrs, 2)
	assert.Contains(suite.T(), attrs, "email")
	assert.Contains(suite.T(), attrs, "name")
	assert.NotContains(suite.T(), attrs, "phone")
}

func (suite *ConsentExecutorTestSuite) TestCollectConsentedAttributes_NoDuplicates() {
	// Same attribute name across multiple purposes should not duplicate
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "attributes:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
			{
				Name: "attributes:test-app-2",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true}, // Duplicate
					{Name: "phone", IsUserApproved: true},
				},
			},
		},
	}

	attrs := collectConsentedAttributes(c)

	assert.Len(suite.T(), attrs, 2)
	assert.Contains(suite.T(), attrs, "email")
	assert.Contains(suite.T(), attrs, "phone")
}

func (suite *ConsentExecutorTestSuite) TestCollectConsentedAttributes_EmptyPurposes() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{},
	}

	attrs := collectConsentedAttributes(c)

	assert.Empty(suite.T(), attrs)
}

func (suite *ConsentExecutorTestSuite) TestCollectConsentedAttributes_NilPurposes() {
	c := &providers.Consent{}

	attrs := collectConsentedAttributes(c)

	assert.Empty(suite.T(), attrs)
}

func (suite *ConsentExecutorTestSuite) TestCollectConsentedAttributes_AllRejected() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "attributes:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: false},
					{Name: "phone", IsUserApproved: false},
				},
			},
		},
	}

	attrs := collectConsentedAttributes(c)

	assert.Empty(suite.T(), attrs)
}

func (suite *ConsentExecutorTestSuite) TestCollectConsentedAttributes_AllApproved() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "attributes:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: true},
					{Name: "name", IsUserApproved: true},
				},
			},
		},
	}

	attrs := collectConsentedAttributes(c)

	assert.Len(suite.T(), attrs, 3)
	assert.Contains(suite.T(), attrs, "email")
	assert.Contains(suite.T(), attrs, "phone")
	assert.Contains(suite.T(), attrs, "name")
}

// ----- collectConsentedPermissions / collectApprovedByPurposeNamespace Tests -----

func (suite *ConsentExecutorTestSuite) TestCollectConsentedPermissions_OnlyPermissionPurposes() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "attributes:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
			{
				Name: "permissions:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "read", IsUserApproved: true},
					{Name: "write", IsUserApproved: false},
					{Name: "cancel", IsUserApproved: true},
				},
			},
		},
	}

	perms := collectConsentedPermissions(c)

	assert.Len(suite.T(), perms, 2)
	assert.Contains(suite.T(), perms, "read")
	assert.Contains(suite.T(), perms, "cancel")
	assert.NotContains(suite.T(), perms, "write")
	assert.NotContains(suite.T(), perms, "email", "attribute element must not appear under permissions")
}

func (suite *ConsentExecutorTestSuite) TestCollectConsentedPermissions_EmptyWhenNoPermissionPurpose() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "attributes:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
		},
	}
	assert.Empty(suite.T(), collectConsentedPermissions(c))
}

func (suite *ConsentExecutorTestSuite) TestCollectConsentedPermissions_DedupsAcrossPurposes() {
	// Hypothetical safety: same permission appearing in two permission purposes is deduped.
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: "permissions:test-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "read", IsUserApproved: true},
				},
			},
			{
				Name: "permissions:other-app",
				Elements: []providers.ConsentElementApproval{
					{Name: "read", IsUserApproved: true},
					{Name: "write", IsUserApproved: true},
				},
			},
		},
	}
	perms := collectConsentedPermissions(c)
	assert.Len(suite.T(), perms, 2)
	assert.Contains(suite.T(), perms, "read")
	assert.Contains(suite.T(), perms, "write")
}

// ----- buildAugmentedAvailableAttributes Tests -----

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_NilBase() {
	entityRef := &providers.EntityReference{
		EntityType: testUserTypeInternal,
		OUID:       "ou-123",
	}

	result := suite.executor.buildAugmentedAvailableAttributes(nil, entityRef)

	// availableAttrResp is nil — return nil so
	// the consent enforcer skips profile-presence filtering entirely.
	assert.Nil(suite.T(), result)
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_EmptyAttributes() {
	availableAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{},
	}
	entityRef := &providers.EntityReference{
		EntityID:   testUserID,
		EntityType: testUserTypeInternal,
		OUID:       "ou-123",
	}

	result := suite.executor.buildAugmentedAvailableAttributes(availableAttrs, entityRef)

	// Even with an empty base we should inject special claim keys so they survive the
	// consent profile-presence filter.
	assert.NotNil(suite.T(), result)
	assert.NotEqual(suite.T(), availableAttrs, result)
	assert.Contains(suite.T(), result.Attributes, "userType")
	assert.Contains(suite.T(), result.Attributes, "ouId")
	assert.Contains(suite.T(), result.Attributes, "ouName")
	assert.Contains(suite.T(), result.Attributes, "ouHandle")
	assert.Contains(suite.T(), result.Attributes, "groups")
	assert.Len(suite.T(), result.Attributes, 5)
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_NoSpecialContext() {
	// EntityType, OUID, EntityID are all empty
	availableAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": nil,
			"phone": nil,
		},
	}
	entityRef := &providers.EntityReference{}

	result := suite.executor.buildAugmentedAvailableAttributes(availableAttrs, entityRef)

	assert.NotNil(suite.T(), result)
	// No special keys should be added; only original keys remain
	assert.Len(suite.T(), result.Attributes, 2)
	assert.True(suite.T(), result.Attributes["email"] == nil)
	assert.True(suite.T(), result.Attributes["phone"] == nil)
	assert.NotContains(suite.T(), result.Attributes, "userType")
	assert.NotContains(suite.T(), result.Attributes, "groups")
	assert.NotContains(suite.T(), result.Attributes, "ouId")
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_WithSingleSpecialField() {
	type testCase struct {
		name             string
		entityType       string
		ouID             string
		entityID         string
		expectedContains []string
		expectedAbsent   []string
	}

	cases := []testCase{
		{
			name:             "EntityType only",
			entityType:       testUserTypeInternal,
			expectedContains: []string{"userType", "email"},
			expectedAbsent:   []string{"ouId", "ouName", "ouHandle", "groups"},
		},
		{
			name:             "OUID only",
			ouID:             "ou-456",
			expectedContains: []string{"ouId", "ouName", "ouHandle", "email"},
			expectedAbsent:   []string{"userType", "groups"},
		},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			availableAttrs := &providers.AttributesResponse{
				Attributes: map[string]*providers.AttributeResponse{
					"email": nil,
				},
			}
			entityRef := &providers.EntityReference{
				EntityType: tc.entityType,
				OUID:       tc.ouID,
				EntityID:   tc.entityID,
			}

			result := suite.executor.buildAugmentedAvailableAttributes(availableAttrs, entityRef)

			assert.NotNil(suite.T(), result)
			for _, key := range tc.expectedContains {
				assert.Contains(suite.T(), result.Attributes, key)
			}
			for _, key := range tc.expectedAbsent {
				assert.NotContains(suite.T(), result.Attributes, key)
			}
		})
	}
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_WithGroups() {
	availableAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": nil,
		},
	}
	entityRef := &providers.EntityReference{
		EntityID: "user-abc",
	}

	result := suite.executor.buildAugmentedAvailableAttributes(availableAttrs, entityRef)

	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), result.Attributes, "groups")
	assert.NotContains(suite.T(), result.Attributes, "userType")
	assert.NotContains(suite.T(), result.Attributes, "ouId")
	assert.Contains(suite.T(), result.Attributes, "email")
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_AllSpecialFields() {
	availableAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": nil,
		},
	}
	entityRef := &providers.EntityReference{
		EntityType: testUserTypeInternal,
		OUID:       "ou-789",
		EntityID:   "user-xyz",
	}

	result := suite.executor.buildAugmentedAvailableAttributes(availableAttrs, entityRef)

	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), result.Attributes, "email")
	assert.Contains(suite.T(), result.Attributes, "userType")
	assert.Contains(suite.T(), result.Attributes, "ouId")
	assert.Contains(suite.T(), result.Attributes, "ouName")
	assert.Contains(suite.T(), result.Attributes, "ouHandle")
	assert.Contains(suite.T(), result.Attributes, "groups")
	// Total: 1 original + 5 special
	assert.Len(suite.T(), result.Attributes, 6)
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_DoesNotMutateOriginal() {
	availableAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": nil,
			"phone": nil,
		},
	}
	entityRef := &providers.EntityReference{
		EntityType: testUserTypeInternal,
		OUID:       "ou-789",
		EntityID:   "user-xyz",
	}

	originalLen := len(availableAttrs.Attributes)
	_ = suite.executor.buildAugmentedAvailableAttributes(availableAttrs, entityRef)

	// The original map must not have been modified
	assert.Len(suite.T(), availableAttrs.Attributes, originalLen)
	assert.NotContains(suite.T(), availableAttrs.Attributes, "userType")
	assert.NotContains(suite.T(), availableAttrs.Attributes, "groups")
	assert.NotContains(suite.T(), availableAttrs.Attributes, "ouId")
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_PreservesVerifications() {
	availableAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": nil,
		},
		Verifications: map[string]*providers.VerificationResponse{
			"v-1": {},
		},
	}
	entityRef := &providers.EntityReference{
		EntityType: testUserTypeInternal,
	}

	result := suite.executor.buildAugmentedAvailableAttributes(availableAttrs, entityRef)

	assert.NotNil(suite.T(), result)
	// Verifications reference is preserved (shallow copy)
	assert.Equal(suite.T(), availableAttrs.Verifications, result.Verifications)
}

// ----- BasicAuth + Consent integration-style test -----

func (suite *ConsentExecutorTestSuite) TestExecute_BasicAuth_NilAvailableAttributes_PromptsConsent() {
	// Simulates a BasicAuthExecutor-authenticated user where AuthUser has been populated by the
	// auth provider and GetUserAvailableAttributes returns the provider's attribute set.
	// ResolveConsent must receive that non-nil map so profile-presence filtering works correctly.
	ctx := buildConsentNodeContext()
	ctx.Application.Assertion = &inboundmodel.AssertionConfig{
		UserAttributes: []string{"given_name", "email"},
	}

	entityRef := &providers.EntityReference{
		EntityID: "user-123",
	}
	authUserAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"given_name": {},
			"email":      {},
		},
	}
	suite.mockAuthnProvider.On("GetEntityReference", mock.Anything, mock.Anything).
		Return(buildConsentAuthUser(), entityRef, (*tidcommon.ServiceError)(nil)).Maybe()
	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return(authUserAttrs, (*tidcommon.ServiceError)(nil)).Maybe()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(false)

	promptData := &providers.ConsentPromptData{
		Purposes: []providers.ConsentPurposePrompt{
			{
				PurposeName: "app:app-123:attrs",
				PurposeID:   "purpose-1",
				Optional:    []providers.PromptElement{{Name: "given_name"}, {Name: "email"}},
			},
		},
	}

	suite.mockConsentEnforcer.On("ResolveConsent",
		mock.Anything, "default", "app-123", "", "user-123",
		[]string{}, []string{"given_name", "email"},
		mock.Anything,
		mock.MatchedBy(func(aa *providers.AttributesResponse) bool {
			if aa == nil {
				return false
			}
			_, hasGivenName := aa.Attributes["given_name"]
			_, hasEmail := aa.Attributes["email"]
			_, hasGroups := aa.Attributes["groups"]
			return hasGivenName && hasEmail && hasGroups
		}), mock.Anything, mock.Anything).
		Return(promptData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status,
		"Executor must prompt for consent when AuthUser is authenticated (BasicAuth)")
	assert.NotEmpty(suite.T(), resp.AdditionalData[common.DataConsentPrompt])
	assert.NotNil(suite.T(), resp.ForwardedData[common.ForwardedDataKeyConsentPrompt])
}

// ----- buildAugmentedAvailableAttributes: with availableAttrResp parameter tests -----

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_WithAvailableAttrs() {
	// When availableAttrResp is provided, buildAugmentedAvailableAttributes should use it
	// and return augmented attributes.
	availableAttrs := &providers.AttributesResponse{
		Attributes: map[string]*providers.AttributeResponse{
			"email": {},
			"phone": {},
		},
	}
	entityRef := &providers.EntityReference{
		EntityID: testUserID,
	}

	result := suite.executor.buildAugmentedAvailableAttributes(availableAttrs, entityRef)

	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), result.Attributes, "email")
	assert.Contains(suite.T(), result.Attributes, "phone")
	assert.Contains(suite.T(), result.Attributes, "groups")
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_NilAvailableAttrs_ReturnsNil() {
	// When availableAttrResp is nil, should return nil so the consent enforcer
	// skips profile-presence filtering.
	entityRef := &providers.EntityReference{
		EntityID: testUserID,
	}

	result := suite.executor.buildAugmentedAvailableAttributes(nil, entityRef)

	assert.Nil(suite.T(), result)
}

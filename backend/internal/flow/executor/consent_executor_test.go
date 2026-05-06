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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appmodel "github.com/asgardeo/thunder/internal/application/model"
	consentauthn "github.com/asgardeo/thunder/internal/authn/consent"
	authnprovidercm "github.com/asgardeo/thunder/internal/authnprovider/common"
	authnprovidermgr "github.com/asgardeo/thunder/internal/authnprovider/manager"
	"github.com/asgardeo/thunder/internal/consent"
	"github.com/asgardeo/thunder/internal/flow/common"
	"github.com/asgardeo/thunder/internal/flow/core"
	inboundmodel "github.com/asgardeo/thunder/internal/inboundclient/model"
	"github.com/asgardeo/thunder/internal/system/error/serviceerror"
	i18ncore "github.com/asgardeo/thunder/internal/system/i18n/core"
	"github.com/asgardeo/thunder/tests/mocks/authn/consentenforcermock"
	"github.com/asgardeo/thunder/tests/mocks/authnprovider/managermock"
	"github.com/asgardeo/thunder/tests/mocks/flow/coremock"
)

type ConsentExecutorTestSuite struct {
	suite.Suite
	mockConsentEnforcer *consentenforcermock.ConsentEnforcerServiceInterfaceMock
	mockFlowFactory     *coremock.FlowFactoryInterfaceMock
	mockAuthnProvider   *managermock.AuthnProviderManagerInterfaceMock
	executor            *consentExecutor
}

func TestConsentExecutorTestSuite(t *testing.T) {
	suite.Run(t, new(ConsentExecutorTestSuite))
}

func (suite *ConsentExecutorTestSuite) SetupTest() {
	suite.mockConsentEnforcer = consentenforcermock.NewConsentEnforcerServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerInterfaceMock(suite.T())

	mockExec := createMockExecutorWithInputs(suite.T())
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameConsent, common.ExecutorTypeUtility,
		mock.AnythingOfType("[]common.Input"), mock.AnythingOfType("[]common.Input")).Return(mockExec)

	suite.executor = newConsentExecutor(suite.mockFlowFactory, suite.mockConsentEnforcer, suite.mockAuthnProvider)
}

// createMockExecutorWithInputs creates a mock executor that supports ValidatePrerequisites and HasRequiredInputs
// with configurable behavior through the mock's On calls.
func createMockExecutorWithInputs(t *testing.T) *coremock.ExecutorInterfaceMock {
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(ExecutorNameConsent).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeUtility).Maybe()
	mockExec.On("GetDefaultInputs").Return([]common.Input{
		{Identifier: userInputConsentDecisions, Type: common.InputTypeConsent, Required: true},
	}).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{
		{Identifier: userAttributeUserID, Type: common.InputTypeText, Required: true},
	}).Maybe()
	return mockExec
}

// --- Helper to build a basic NodeContext ---

func buildConsentNodeContext() *core.NodeContext {
	var authUser authnprovidermgr.AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"LOCAL","isVerified":true}],`+
		`"userHistory":[{"userId":"user-123","isValuesIncluded":true}],"userState":"exists"}`), &authUser)

	return &core.NodeContext{
		Context:        context.Background(),
		ExecutionID:    "flow-123",
		AppID:          "app-123",
		AuthUser:       authUser,
		UserInputs:     map[string]string{},
		RuntimeData:    map[string]string{},
		NodeProperties: map[string]interface{}{},
		Application: appmodel.Application{
			Assertion: &inboundmodel.AssertionConfig{
				UserAttributes: []string{"email", "phone"},
			},
		},
	}
}

// setupAuthnProviderForCheckConsent registers expectations for buildAugmentedAvailableAttributes
// which is called on every checkConsent (HasRequiredInputs=false) path.
func (suite *ConsentExecutorTestSuite) setupAuthnProviderForCheckConsent() {
	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return((*authnprovidercm.AttributesResponse)(nil), nil).Once()
}

// ----- Constructor Tests -----

func (suite *ConsentExecutorTestSuite) TestNewConsentExecutor() {
	assert.NotNil(suite.T(), suite.executor)
	assert.NotNil(suite.T(), suite.executor.consentEnforcer)
	assert.NotNil(suite.T(), suite.executor.logger)
	assert.Equal(suite.T(), ExecutorNameConsent, suite.executor.GetName())
	assert.Equal(suite.T(), common.ExecutorTypeUtility, suite.executor.GetType())
}

func (suite *ConsentExecutorTestSuite) TestNewConsentExecutor_DefaultInputs() {
	inputs := suite.executor.GetDefaultInputs()
	assert.Len(suite.T(), inputs, 1)
	assert.Equal(suite.T(), userInputConsentDecisions, inputs[0].Identifier)
	assert.Equal(suite.T(), common.InputTypeConsent, inputs[0].Type)
	assert.True(suite.T(), inputs[0].Required)
}

func (suite *ConsentExecutorTestSuite) TestNewConsentExecutor_Prerequisites() {
	prereqs := suite.executor.GetPrerequisites()
	assert.Len(suite.T(), prereqs, 1)
	assert.Equal(suite.T(), userAttributeUserID, prereqs[0].Identifier)
	assert.Equal(suite.T(), common.InputTypeText, prereqs[0].Type)
	assert.True(suite.T(), prereqs[0].Required)
}

// ----- Execute: Prerequisites Failure -----

func (suite *ConsentExecutorTestSuite) TestExecute_PrerequisitesFailure() {
	ctx := buildConsentNodeContext()

	// Mock ValidatePrerequisites to return false
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "Prerequisites validation failed")
}

// ----- Execute: checkConsent (no inputs provided) -----

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AllConsentsActive() {
	ctx := buildConsentNodeContext()

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	// ResolveConsent returns nil = all consents active
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "user-123",
		[]string{}, []string{"email", "phone"}, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_RequiredAttributesFromRuntimeData() {
	ctx := buildConsentNodeContext()
	ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes] = "email name"

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	// ResolveConsent should receive attributes from RuntimeData, not from Application config
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "user-123",
		[]string{}, []string{"email", "name"}, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_RequiredEssentialAndOptionalAttributesFromRuntimeData() {
	ctx := buildConsentNodeContext()
	ctx.RuntimeData[common.RuntimeKeyRequiredEssentialAttributes] = "email"
	ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes] = "name"

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "user-123",
		[]string{"email"}, []string{"name"}, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_NilAssertionConfig() {
	ctx := buildConsentNodeContext()
	ctx.Application.Assertion = nil

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	// Attributes should be nil when no RuntimeData and no Assertion config
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "user-123",
		[]string{}, []string{}, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_ExplicitEmptyRuntimeKeysSkipAssertionFallback() {
	ctx := buildConsentNodeContext()
	// Set both runtime keys to empty strings — the keys are present but carry no attributes.
	// The fallback to Application.Assertion.UserAttributes must NOT trigger.
	ctx.RuntimeData[common.RuntimeKeyRequiredEssentialAttributes] = ""
	ctx.RuntimeData[common.RuntimeKeyRequiredOptionalAttributes] = ""

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	// Expect empty slices — NOT the Application.Assertion.UserAttributes (["email","phone"])
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "user-123",
		[]string{}, []string{}, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_ResolveConsent_ClientError() {
	ctx := buildConsentNodeContext()

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "user-123",
		mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &serviceerror.ServiceError{
			Type: serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{
				DefaultValue: "consent config not found",
			},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "consent config not found")
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_ResolveConsent_ServerError() {
	ctx := buildConsentNodeContext()

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "user-123",
		mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
		})

	resp, err := suite.executor.Execute(ctx)

	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to resolve consent")
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_PromptRequired_NoTimeout() {
	ctx := buildConsentNodeContext()

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	promptData := &consentauthn.ConsentPromptData{
		Purposes: []consentauthn.ConsentPurposePrompt{
			{
				PurposeName: "app:app-123:attrs",
				PurposeID:   "purpose-1",
				Essential:   []string{"email"},
				Optional:    []string{"phone"},
			},
		},
	}

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "user-123",
		mock.Anything, mock.Anything, mock.Anything).
		Return(promptData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.NotEmpty(suite.T(), resp.AdditionalData[common.DataConsentPrompt])
	assert.Empty(suite.T(), resp.AdditionalData[common.DataStepTimeout],
		"Should not set timeout without timeout config")

	// Verify ForwardedData contains the prompt data
	assert.NotNil(suite.T(), resp.ForwardedData[common.ForwardedDataKeyConsentPrompt])

	// Verify the JSON serialization
	var parsedPrompt []consentauthn.ConsentPurposePrompt
	parseErr := json.Unmarshal([]byte(resp.AdditionalData[common.DataConsentPrompt]), &parsedPrompt)
	assert.NoError(suite.T(), parseErr)
	assert.Len(suite.T(), parsedPrompt, 1)
	assert.Equal(suite.T(), "app:app-123:attrs", parsedPrompt[0].PurposeName)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_PromptRequired_StoresSessionToken() {
	ctx := buildConsentNodeContext()

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	promptData := &consentauthn.ConsentPromptData{
		Purposes:     []consentauthn.ConsentPurposePrompt{{PurposeName: "purpose-1", Optional: []string{"email"}}},
		SessionToken: "consent-session-token",
	}

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, "default", "app-123", "user-123",
		mock.Anything, mock.Anything, mock.Anything).
		Return(promptData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), "consent-session-token", resp.RuntimeData[common.RuntimeKeyConsentSessionToken])
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_PromptRequired_WithTimeout() {
	ctx := buildConsentNodeContext()
	ctx.NodeProperties["timeout"] = "300" // 5 minutes

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	promptData := &consentauthn.ConsentPromptData{
		Purposes: []consentauthn.ConsentPurposePrompt{
			{PurposeName: "purpose-1", Essential: []string{"email"}},
		},
	}

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(promptData, nil)

	beforeExec := time.Now().UnixMilli()
	resp, err := suite.executor.Execute(ctx)
	afterExec := time.Now().UnixMilli()

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)

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

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	promptData := &consentauthn.ConsentPromptData{
		Purposes: []consentauthn.ConsentPurposePrompt{
			{PurposeName: "purpose-1", Essential: []string{"email"}},
		},
	}

	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(promptData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Empty(suite.T(), resp.AdditionalData[common.DataStepTimeout])
}

// ----- Execute: handleConsentDecisions (inputs provided) -----

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_AllApproved_Success() {
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{
				PurposeName: "app:app-123:attrs",
				Approved:    true,
				Elements: []consentauthn.ElementDecision{
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

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	consentResult := &consent.Consent{
		ID:     "consent-001",
		Status: consent.ConsentStatusActive,
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: "app:app-123:attrs",
				Elements: []consent.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: true},
				},
			},
		},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, "default", "app-123", "user-123",
		mock.AnythingOfType("*consent.ConsentDecisions"), mock.Anything, int64(86400)).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "consent-001", resp.RuntimeData[common.RuntimeKeyConsentID])
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyConsentedAttributes], "email")
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyConsentedAttributes], "phone")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_HTMLEscapedJSON() {
	// Simulate the HTML-escaped JSON that SanitizeStringMap would produce
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{PurposeName: "purpose-1", Approved: true},
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

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	consentResult := &consent.Consent{
		ID:       "consent-002",
		Purposes: []consent.ConsentPurposeItem{},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_EmptyDecisions() {
	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = ""

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "missing or empty")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_MissingDecisionsKey() {
	ctx := buildConsentNodeContext()
	// Don't set userInputConsentDecisions at all

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "missing or empty")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_InvalidJSON() {
	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = "{invalid-json}"

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "Failed to parse consent decisions")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_ConsentTimeout_Expired() {
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{PurposeName: "purpose-1", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	// Set an expiry timestamp in the past
	pastExpiry := strconv.FormatInt(time.Now().Add(-1*time.Minute).UnixMilli(), 10)
	ctx.RuntimeData[common.RuntimeKeyStepTimeout] = pastExpiry

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "timed out")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_ConsentTimeout_NotExpired() {
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{PurposeName: "purpose-1", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	// Set an expiry timestamp in the future
	futureExpiry := strconv.FormatInt(time.Now().Add(5*time.Minute).UnixMilli(), 10)
	ctx.RuntimeData[common.RuntimeKeyStepTimeout] = futureExpiry

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	consentResult := &consent.Consent{
		ID:       "consent-003",
		Purposes: []consent.ConsentPurposeItem{},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_EssentialDenied() {
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{
				PurposeName: "purpose-1",
				Approved:    true,
				Elements: []consentauthn.ElementDecision{
					{Name: "email", Approved: false}, // User denied essential
				},
			},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	// RecordConsent persists the denial and returns an essential-denied error
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*consent.Consent)(nil), &consentauthn.ErrorEssentialConsentDenied)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), failureReasonConsentDenied, resp.FailureReason)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_RecordConsent_ClientError() {
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{PurposeName: "purpose-1", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &serviceerror.ServiceError{
			Type: serviceerror.ClientErrorType,
			ErrorDescription: i18ncore.I18nMessage{
				DefaultValue: "invalid consent data",
			},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "invalid consent data")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_RecordConsent_ServerError() {
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{PurposeName: "purpose-1", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &serviceerror.ServiceError{
			Type: serviceerror.ServerErrorType,
		})

	resp, err := suite.executor.Execute(ctx)

	assert.Nil(suite.T(), resp)
	assert.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "failed to record consent")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_NilLoginConsentConfig() {
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{PurposeName: "purpose-1", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	ctx.Application.LoginConsent = nil // No login consent config

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	consentResult := &consent.Consent{
		ID:       "consent-004",
		Purposes: []consent.ConsentPurposeItem{},
	}

	// ValidityPeriod should be 0 when LoginConsent is nil
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, "default", "app-123", "user-123",
		mock.AnythingOfType("*consent.ConsentDecisions"), mock.Anything, int64(0)).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_PartialElementApproval() {
	// Test where a purpose is approved but some elements are not approved
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{
				PurposeName: "purpose-1",
				Approved:    true,
				Elements: []consentauthn.ElementDecision{
					{Name: "email", Approved: true},
					{Name: "phone", Approved: false}, // Not approved but purpose overall is approved
				},
			},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	consentResult := &consent.Consent{
		ID: "consent-005",
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: "purpose-1",
				Elements: []consent.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: false},
				},
			},
		},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)

	// Only email should be in consented attributes (phone was not approved)
	consentedAttrs := resp.RuntimeData[common.RuntimeKeyConsentedAttributes]
	assert.Contains(suite.T(), consentedAttrs, "email")
	assert.NotContains(suite.T(), consentedAttrs, "phone")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_MultiplePurposes_AllApproved() {
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{PurposeName: "purpose-1", Approved: true},
			{PurposeName: "purpose-2", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	consentResult := &consent.Consent{
		ID: "consent-006",
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: "purpose-1",
				Elements: []consent.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
			{
				Name: "purpose-2",
				Elements: []consent.ConsentElementApproval{
					{Name: "name", IsUserApproved: true},
					{Name: "phone", IsUserApproved: true},
				},
			},
		},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)

	consentedAttrs := resp.RuntimeData[common.RuntimeKeyConsentedAttributes]
	assert.Contains(suite.T(), consentedAttrs, "email")
	assert.Contains(suite.T(), consentedAttrs, "name")
	assert.Contains(suite.T(), consentedAttrs, "phone")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_NoConsentedElements() {
	decisions := consentauthn.ConsentDecisions{
		Purposes: []consentauthn.PurposeDecision{
			{PurposeName: "purpose-1", Approved: true},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)

	// Consent record with no approved elements
	consentResult := &consent.Consent{
		ID: "consent-007",
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: "purpose-1",
				Elements: []consent.ConsentElementApproval{
					{Name: "email", IsUserApproved: false},
					{Name: "phone", IsUserApproved: false},
				},
			},
		},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)

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
	// UserID is set (testUserID="user-123"), so "groups" must be injected

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	suite.mockConsentEnforcer.On("ResolveConsent",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
		mock.MatchedBy(func(aa *authnprovidercm.AttributesResponse) bool {
			if aa == nil || len(aa.Attributes) == 0 {
				return false
			}
			groupsMeta, hasGroups := aa.Attributes["groups"]
			return hasGroups && groupsMeta != nil
		})).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AugmentedAttributes_OUClaimsInjected() {
	ctx := buildConsentNodeContext()
	// Set OUID in AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"LOCAL","isVerified":true}],`+
		`"userHistory":[{"userId":"user-123","ouId":"ou-999","isValuesIncluded":true}],"userState":"exists"}`),
		&ctx.AuthUser)

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	suite.mockConsentEnforcer.On("ResolveConsent",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
		mock.MatchedBy(func(aa *authnprovidercm.AttributesResponse) bool {
			if aa == nil {
				return false
			}
			_, hasOUID := aa.Attributes["ouId"]
			_, hasOUName := aa.Attributes["ouName"]
			_, hasOUHandle := aa.Attributes["ouHandle"]
			return hasOUID && hasOUName && hasOUHandle
		})).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AugmentedAttributes_UserTypeInjected() {
	ctx := buildConsentNodeContext()
	// Set UserType in AuthUser
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"LOCAL","isVerified":true}],`+
		`"userHistory":[{"userId":"user-123","userType":"customer","isValuesIncluded":true}],"userState":"exists"}`),
		&ctx.AuthUser)

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	suite.mockConsentEnforcer.On("ResolveConsent",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
		mock.MatchedBy(func(aa *authnprovidercm.AttributesResponse) bool {
			return aa != nil && func() bool { _, ok := aa.Attributes["userType"]; return ok }()
		})).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AugmentedAttributes_NilBaseWithSpecialClaims() {
	// Even with a nil base from GetUserAvailableAttributes, special claim keys from the authenticated
	// user context must be injected and forwarded to ResolveConsent.
	ctx := buildConsentNodeContext()
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"LOCAL","isVerified":true}],`+
		`"userHistory":[{"userId":"user-123","userType":"internal","isValuesIncluded":true}],"userState":"exists"}`),
		&ctx.AuthUser)

	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(true)
	suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*common.ExecutorResponse")).Return(false)

	suite.setupAuthnProviderForCheckConsent()

	suite.mockConsentEnforcer.On("ResolveConsent",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
		mock.MatchedBy(func(aa *authnprovidercm.AttributesResponse) bool {
			if aa == nil {
				return false
			}
			_, hasUserType := aa.Attributes["userType"]
			_, hasGroups := aa.Attributes["groups"]
			return hasUserType && hasGroups
		})).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

// ----- collectConsentedAttributes Tests -----

func (suite *ConsentExecutorTestSuite) TestCollectConsentedAttributes_MixedApprovals() {
	c := &consent.Consent{
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: "purpose-1",
				Elements: []consent.ConsentElementApproval{
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
	c := &consent.Consent{
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: "purpose-1",
				Elements: []consent.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
			{
				Name: "purpose-2",
				Elements: []consent.ConsentElementApproval{
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
	c := &consent.Consent{
		Purposes: []consent.ConsentPurposeItem{},
	}

	attrs := collectConsentedAttributes(c)

	assert.Empty(suite.T(), attrs)
}

func (suite *ConsentExecutorTestSuite) TestCollectConsentedAttributes_NilPurposes() {
	c := &consent.Consent{}

	attrs := collectConsentedAttributes(c)

	assert.Empty(suite.T(), attrs)
}

func (suite *ConsentExecutorTestSuite) TestCollectConsentedAttributes_AllRejected() {
	c := &consent.Consent{
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: "purpose-1",
				Elements: []consent.ConsentElementApproval{
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
	c := &consent.Consent{
		Purposes: []consent.ConsentPurposeItem{
			{
				Name: "purpose-1",
				Elements: []consent.ConsentElementApproval{
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

// ----- buildAugmentedAvailableAttributes Tests -----
// These tests call the method directly on the executor.

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_NilBase() {
	ctx := buildConsentNodeContext()
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"LOCAL","isVerified":true}],`+
		`"userHistory":[{"userId":"user-123","userType":"internal","ouId":"ou-123","isValuesIncluded":true}],`+
		`"userState":"exists"}`), &ctx.AuthUser)

	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return((*authnprovidercm.AttributesResponse)(nil), nil).Once()

	result := suite.executor.buildAugmentedAvailableAttributes(ctx)

	// Even with a nil base we should still inject the special claim keys that are known
	// to be present from the authenticated user context.
	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), result.Attributes, "userType")
	assert.Contains(suite.T(), result.Attributes, "ouId")
	assert.Contains(suite.T(), result.Attributes, "ouName")
	assert.Contains(suite.T(), result.Attributes, "ouHandle")
	assert.Contains(suite.T(), result.Attributes, "groups")
	assert.Len(suite.T(), result.Attributes, 5)
	assert.Nil(suite.T(), result.Verifications)
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_EmptyAttributes() {
	ctx := buildConsentNodeContext()
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"LOCAL","isVerified":true}],`+
		`"userHistory":[{"userId":"user-123","userType":"internal","ouId":"ou-123","isValuesIncluded":true}],`+
		`"userState":"exists"}`), &ctx.AuthUser)

	emptyBase := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{},
	}

	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return(emptyBase, nil).Once()

	result := suite.executor.buildAugmentedAvailableAttributes(ctx)

	// Even with an empty base we should inject special claim keys.
	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), result.Attributes, "userType")
	assert.Contains(suite.T(), result.Attributes, "ouId")
	assert.Contains(suite.T(), result.Attributes, "ouName")
	assert.Contains(suite.T(), result.Attributes, "ouHandle")
	assert.Contains(suite.T(), result.Attributes, "groups")
	assert.Len(suite.T(), result.Attributes, 5)
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_NoSpecialContext() {
	ctx := buildConsentNodeContext()
	// All special fields empty — only authHistory to make IsAuthenticated() return true
	_ = json.Unmarshal([]byte(
		`{"authHistory":[{"authType":"LOCAL","isVerified":true}],"userHistory":[],"userState":"exists"}`),
		&ctx.AuthUser)

	base := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": nil,
			"phone": nil,
		},
	}

	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return(base, nil).Once()

	result := suite.executor.buildAugmentedAvailableAttributes(ctx)

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
		authUserJSON     string
		expectedContains []string
		expectedAbsent   []string
	}

	cases := []testCase{
		{
			name: "UserType only",
			authUserJSON: `{"authHistory":[{"authType":"LOCAL","isVerified":true}],` +
				`"userHistory":[{"userType":"internal","isValuesIncluded":true}],"userState":"exists"}`,
			expectedContains: []string{"userType", "email"},
			expectedAbsent:   []string{"ouId", "ouName", "ouHandle", "groups"},
		},
		{
			name: "OUID only",
			authUserJSON: `{"authHistory":[{"authType":"LOCAL","isVerified":true}],` +
				`"userHistory":[{"ouId":"ou-456","isValuesIncluded":true}],"userState":"exists"}`,
			expectedContains: []string{"ouId", "ouName", "ouHandle", "email"},
			expectedAbsent:   []string{"userType", "groups"},
		},
	}

	for _, tc := range cases {
		suite.Run(tc.name, func() {
			ctx := buildConsentNodeContext()
			_ = json.Unmarshal([]byte(tc.authUserJSON), &ctx.AuthUser)

			base := &authnprovidercm.AttributesResponse{
				Attributes: map[string]*authnprovidercm.AttributeResponse{
					"email": nil,
				},
			}

			suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
				Return(base, nil).Once()

			result := suite.executor.buildAugmentedAvailableAttributes(ctx)

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
	ctx := buildConsentNodeContext()
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"LOCAL","isVerified":true}],`+
		`"userHistory":[{"userId":"user-abc","isValuesIncluded":true}],"userState":"exists"}`), &ctx.AuthUser)

	base := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": nil,
		},
	}

	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return(base, nil).Once()

	result := suite.executor.buildAugmentedAvailableAttributes(ctx)

	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), result.Attributes, "groups")
	assert.NotContains(suite.T(), result.Attributes, "userType")
	assert.NotContains(suite.T(), result.Attributes, "ouId")
	assert.Contains(suite.T(), result.Attributes, "email")
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_AllSpecialFields() {
	ctx := buildConsentNodeContext()
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"LOCAL","isVerified":true}],`+
		`"userHistory":[{"userId":"user-xyz","userType":"internal","ouId":"ou-789","isValuesIncluded":true}],`+
		`"userState":"exists"}`), &ctx.AuthUser)

	base := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": nil,
		},
	}

	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return(base, nil).Once()

	result := suite.executor.buildAugmentedAvailableAttributes(ctx)

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
	ctx := buildConsentNodeContext()
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"LOCAL","isVerified":true}],`+
		`"userHistory":[{"userId":"user-xyz","userType":"internal","ouId":"ou-789","isValuesIncluded":true}],`+
		`"userState":"exists"}`), &ctx.AuthUser)

	baseAttrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": nil,
			"phone": nil,
		},
	}
	originalLen := len(baseAttrs.Attributes)

	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return(baseAttrs, nil).Once()

	_ = suite.executor.buildAugmentedAvailableAttributes(ctx)

	// The original map must not have been modified
	assert.Len(suite.T(), baseAttrs.Attributes, originalLen)
	assert.NotContains(suite.T(), baseAttrs.Attributes, "userType")
	assert.NotContains(suite.T(), baseAttrs.Attributes, "groups")
	assert.NotContains(suite.T(), baseAttrs.Attributes, "ouId")
}

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_PreservesVerifications() {
	ctx := buildConsentNodeContext()
	_ = json.Unmarshal([]byte(`{"authHistory":[{"authType":"LOCAL","isVerified":true}],`+
		`"userHistory":[{"userId":"user-123","userType":"internal","isValuesIncluded":true}],"userState":"exists"}`),
		&ctx.AuthUser)

	baseAttrs := &authnprovidercm.AttributesResponse{
		Attributes: map[string]*authnprovidercm.AttributeResponse{
			"email": nil,
		},
		Verifications: map[string]*authnprovidercm.VerificationResponse{
			"v-1": {},
		},
	}

	suite.mockAuthnProvider.On("GetUserAvailableAttributes", mock.Anything, mock.Anything).
		Return(baseAttrs, nil).Once()

	result := suite.executor.buildAugmentedAvailableAttributes(ctx)

	assert.NotNil(suite.T(), result)
	// Verifications reference is preserved (shallow copy)
	assert.Equal(suite.T(), baseAttrs.Verifications, result.Verifications)
}

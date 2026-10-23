// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
	authzdetail "github.com/thunder-id/thunderid/internal/authzdetail/common"
	"github.com/thunder-id/thunderid/internal/consent"
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

	suite.executor = newConsentExecutor(suite.mockFlowFactory, suite.mockConsentEnforcer,
		suite.mockAuthnProvider)
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
	_ = authUser.UnmarshalJSON([]byte(`{"default":{"entityReferenceToken":"tok","attributeToken":"tok"}}`))
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
		[]string{}, []string{"email",
			"phone"}, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, true, mock.Anything).
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, false, mock.Anything).
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
		[]string{}, []string{"email",
			"name"}, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		[]string{"email"}, []string{"name"}, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		[]string{}, []string{}, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		[]string{}, []string{}, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).
		Return(promptData, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Empty(suite.T(), resp.AdditionalData[common.DataStepTimeout])
}

// ----- Execute: handleConsentDecisions (inputs provided) -----

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_AllApproved_Success() {
	decisions := providers.ConsentDecisions{
		Approved: true,
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

// Consent decisions arrive as a JSON string through SanitizeStringMap. HTML entities in a purpose
// name must reach the enforcer verbatim; unescaping here would corrupt a legitimate "&amp;" value.
func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_HTMLEntitiesPreserved() {
	purposeName := `attributes:R&D <team> &amp; "core"`
	decisions := providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: purposeName, Approved: true},
		},
	}
	decisionsJSON, err := json.Marshal(decisions)
	assert.NoError(suite.T(), err)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID:       "consent-002",
		Purposes: []providers.ConsentPurposeItem{},
	}

	var recorded *providers.ConsentDecisions
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.AnythingOfType("*providers.ConsentDecisions"), mock.Anything,
		mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			recorded = args.Get(4).(*providers.ConsentDecisions)
		}).
		Return(consentResult, nil)

	resp, execErr := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), execErr)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.NotNil(suite.T(), recorded)
	assert.Equal(suite.T(), purposeName, recorded.Purposes[0].PurposeName)
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
		Approved: true,
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

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_TimeoutReason_CompletesWithoutRecording() {
	decisions := providers.ConsentDecisions{
		Approved: false,
		Reason:   providers.ConsentDecisionReasonTimeout,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: false},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	// The auto submission is expected to arrive after the deadline, so the expiry must be skipped
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
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Nil(suite.T(), resp.Error)

	// Nothing is consented, and both keys must be present so auth assert knows consent ran
	consentedAttrs, hasAttrs := resp.RuntimeData[common.RuntimeKeyConsentedAttributes]
	assert.True(suite.T(), hasAttrs)
	assert.Empty(suite.T(), consentedAttrs)
	consentedPerms, hasPerms := resp.RuntimeData[common.RuntimeKeyConsentedPermissions]
	assert.True(suite.T(), hasPerms)
	assert.Empty(suite.T(), consentedPerms)

	// A timeout is not a decision, so no consent record is created
	assert.Empty(suite.T(), resp.RuntimeData[common.RuntimeKeyConsentID])
	suite.mockConsentEnforcer.AssertNotCalled(suite.T(), "RecordConsent", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_TimeoutReason_WithoutConfiguredTimeout() {
	decisions := providers.ConsentDecisions{
		Reason:   providers.ConsentDecisionReasonTimeout,
		Purposes: []providers.PurposeDecision{{PurposeName: "attributes:test-app"}},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockConsentEnforcer.AssertNotCalled(suite.T(), "RecordConsent", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_UserDeniedReason_IsRecorded() {
	decisions := providers.ConsentDecisions{
		Approved: false,
		Reason:   providers.ConsentDecisionReasonUserDenied,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: false},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)

	futureExpiry := strconv.FormatInt(time.Now().Add(5*time.Minute).UnixMilli(), 10)
	ctx.RuntimeData[common.RuntimeKeyStepTimeout] = futureExpiry
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID:       "consent-004",
		Purposes: []providers.ConsentPurposeItem{},
	}

	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)

	// Unlike a timeout, an explicit denial is a user decision and must be persisted
	assert.Equal(suite.T(), "consent-004", resp.RuntimeData[common.RuntimeKeyConsentID])
	suite.mockConsentEnforcer.AssertCalled(suite.T(), "RecordConsent", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_FailOnDeny_UserDenied() {
	decisions := providers.ConsentDecisions{
		Approved: false,
		Reason:   providers.ConsentDecisionReasonUserDenied,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: false},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	ctx.NodeProperties["failOnDeny"] = true
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"),
			mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID:       "consent-fod-1",
		Purposes: []providers.ConsentPurposeItem{},
	}
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentDenied.Code, resp.Error.Code)

	// The failure branch must not populate downstream runtime data
	assert.Empty(suite.T(), resp.RuntimeData[common.RuntimeKeyConsentID])
	_, hasAttrs := resp.RuntimeData[common.RuntimeKeyConsentedAttributes]
	assert.False(suite.T(), hasAttrs)
	_, hasPerms := resp.RuntimeData[common.RuntimeKeyConsentedPermissions]
	assert.False(suite.T(), hasPerms)

	// The consent record is still persisted before failing the flow
	suite.mockConsentEnforcer.AssertCalled(suite.T(), "RecordConsent", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_FailOnDeny_UserApprovedWithOptionalToggledOff() {
	decisions := providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "attributes:test-app",
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "phone", Approved: false}, // Optional element toggled off, but user clicked Allow
				},
			},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	ctx.NodeProperties["failOnDeny"] = true
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"),
			mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID:       "consent-fod-2",
		Purposes: []providers.ConsentPurposeItem{},
	}
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "consent-fod-2", resp.RuntimeData[common.RuntimeKeyConsentID])
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_FailOnDeny_Disabled_UserDeniedSucceeds() {
	decisions := providers.ConsentDecisions{
		Approved: false,
		Reason:   providers.ConsentDecisionReasonUserDenied,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: false},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	// failOnDeny not set, mirroring the current default behavior
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"),
			mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	consentResult := &providers.Consent{
		ID:       "consent-fod-3",
		Purposes: []providers.ConsentPurposeItem{},
	}
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(consentResult, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "consent-fod-3", resp.RuntimeData[common.RuntimeKeyConsentID])
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_FailOnDeny_EssentialDeniedStillFails() {
	decisions := providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "attributes:test-app",
				Approved:    true,
				Elements: []providers.ElementDecision{
					{Name: "email", Approved: false},
				},
			},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	ctx.NodeProperties["failOnDeny"] = true
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"),
			mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	// The essential-denied branch runs before the failOnDeny check
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return((*providers.Consent)(nil), &consentauthn.ErrorEssentialConsentDenied)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentDenied.Code, resp.Error.Code)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_FailOnDeny_TimeoutReasonFailsFlow() {
	decisions := providers.ConsentDecisions{
		Approved: false,
		Reason:   providers.ConsentDecisionReasonTimeout,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:test-app", Approved: false},
		},
	}
	decisionsJSON, _ := json.Marshal(decisions)

	ctx := buildConsentNodeContext()
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	ctx.NodeProperties["failOnDeny"] = true

	pastExpiry := strconv.FormatInt(time.Now().Add(-1*time.Minute).UnixMilli(), 10)
	ctx.RuntimeData[common.RuntimeKeyStepTimeout] = pastExpiry
	suite.setupDefaultAuthnProviderMocks()

	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"),
			mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrConsentPromptTimedOut.Code, resp.Error.Code)

	// A timed-out prompt is not a user decision, so nothing must be recorded even when failing
	suite.mockConsentEnforcer.AssertNotCalled(suite.T(), "RecordConsent", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_ConsentTimeout_NotExpired() {
	decisions := providers.ConsentDecisions{
		Approved: true,
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
		Approved: true,
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
		Approved: true,
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
		Approved: true,
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
		Approved: true,
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
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{
				PurposeName: "attributes:app-123",
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
				Name: "attributes:app-123",
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

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_AttributeAndPermissionPurposes_AllApproved() {
	decisions := providers.ConsentDecisions{
		Approved: true,
		Purposes: []providers.PurposeDecision{
			{PurposeName: "attributes:app-123", Approved: true},
			{PurposeName: "permissions:app-123", Approved: true},
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
				Name: "attributes:app-123",
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "name", IsUserApproved: true},
				},
			},
			{
				Name: "permissions:app-123",
				Elements: []providers.ConsentElementApproval{
					{Name: "read", IsUserApproved: true},
					{Name: "write", IsUserApproved: true},
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

	consentedPerms := resp.RuntimeData[common.RuntimeKeyConsentedPermissions]
	assert.Contains(suite.T(), consentedPerms, "read")
	assert.Contains(suite.T(), consentedPerms, "write")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_NoConsentedElements() {
	decisions := providers.ConsentDecisions{
		Approved: true,
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
				Name: "attributes:app-123",
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
		mock.Anything, mock.Anything,
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
		mock.Anything, mock.Anything,
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
		mock.Anything, mock.Anything,
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
		mock.Anything, mock.Anything,
		mock.MatchedBy(func(aa *providers.AttributesResponse) bool {
			return aa == nil
		}), mock.Anything, mock.Anything).
		Return(nil, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

// ----- collectApprovedElementsByPurposeName Tests -----

func (suite *ConsentExecutorTestSuite) TestCollectApprovedElementsByPurposeName_MixedApprovals() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: buildAttributePurposeName("test-app"),
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: false},
					{Name: "name", IsUserApproved: true},
				},
			},
		},
	}

	attrs := collectApprovedElementsByPurposeName(c, buildAttributePurposeName("test-app"))

	assert.Len(suite.T(), attrs, 2)
	assert.Contains(suite.T(), attrs, "email")
	assert.Contains(suite.T(), attrs, "name")
	assert.NotContains(suite.T(), attrs, "phone")
}

func (suite *ConsentExecutorTestSuite) TestCollectApprovedElementsByPurposeName_DedupsWithinPurpose() {
	// The same element name repeated within the matched purpose is returned only once.
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: buildAttributePurposeName("test-app"),
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: true},
				},
			},
		},
	}

	attrs := collectApprovedElementsByPurposeName(c, buildAttributePurposeName("test-app"))

	assert.Len(suite.T(), attrs, 2)
	assert.Contains(suite.T(), attrs, "email")
	assert.Contains(suite.T(), attrs, "phone")
}

func (suite *ConsentExecutorTestSuite) TestCollectApprovedElementsByPurposeName_SelectsMatchingPurposeOnly() {
	// A record carrying both the attribute and permission purposes for an app: each lookup returns
	// only its own purpose's elements.
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: buildAttributePurposeName("test-app"),
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
			{
				Name: buildPermissionsPurposeName("test-app"),
				Elements: []providers.ConsentElementApproval{
					{Name: "read", IsUserApproved: true},
					{Name: "write", IsUserApproved: false},
					{Name: "cancel", IsUserApproved: true},
				},
			},
		},
	}

	attrs := collectApprovedElementsByPurposeName(c, buildAttributePurposeName("test-app"))
	assert.Equal(suite.T(), []string{"email"}, attrs)

	perms := collectApprovedElementsByPurposeName(c, buildPermissionsPurposeName("test-app"))
	assert.Len(suite.T(), perms, 2)
	assert.Contains(suite.T(), perms, "read")
	assert.Contains(suite.T(), perms, "cancel")
	assert.NotContains(suite.T(), perms, "write")
	assert.NotContains(suite.T(), perms, "email", "attribute element must not appear under permissions")
}

func (suite *ConsentExecutorTestSuite) TestCollectApprovedElementsByPurposeName_PurposeNotFound() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: buildAttributePurposeName("test-app"),
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
				},
			},
		},
	}

	assert.Empty(suite.T(), collectApprovedElementsByPurposeName(c, buildPermissionsPurposeName("test-app")))
	assert.Empty(suite.T(), collectApprovedElementsByPurposeName(c, buildAttributePurposeName("other-app")))
}

func (suite *ConsentExecutorTestSuite) TestCollectApprovedElementsByPurposeName_EmptyPurposes() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{},
	}

	assert.Empty(suite.T(), collectApprovedElementsByPurposeName(c, buildAttributePurposeName("test-app")))
}

func (suite *ConsentExecutorTestSuite) TestCollectApprovedElementsByPurposeName_NilPurposes() {
	c := &providers.Consent{}

	assert.Empty(suite.T(), collectApprovedElementsByPurposeName(c, buildAttributePurposeName("test-app")))
}

func (suite *ConsentExecutorTestSuite) TestCollectApprovedElementsByPurposeName_AllRejected() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: buildAttributePurposeName("test-app"),
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: false},
					{Name: "phone", IsUserApproved: false},
				},
			},
		},
	}

	assert.Empty(suite.T(), collectApprovedElementsByPurposeName(c, buildAttributePurposeName("test-app")))
}

func (suite *ConsentExecutorTestSuite) TestCollectApprovedElementsByPurposeName_AllApproved() {
	c := &providers.Consent{
		Purposes: []providers.ConsentPurposeItem{
			{
				Name: buildAttributePurposeName("test-app"),
				Elements: []providers.ConsentElementApproval{
					{Name: "email", IsUserApproved: true},
					{Name: "phone", IsUserApproved: true},
					{Name: "name", IsUserApproved: true},
				},
			},
		},
	}

	attrs := collectApprovedElementsByPurposeName(c, buildAttributePurposeName("test-app"))

	assert.Len(suite.T(), attrs, 3)
	assert.Contains(suite.T(), attrs, "email")
	assert.Contains(suite.T(), attrs, "phone")
	assert.Contains(suite.T(), attrs, "name")
}

// ----- purpose name builder Tests -----

func (suite *ConsentExecutorTestSuite) TestBuildPurposeNames() {
	assert.Equal(suite.T(), "attributes:test-app", buildAttributePurposeName("test-app"))
	assert.Equal(suite.T(), "permissions:test-app", buildPermissionsPurposeName("test-app"))
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
	assert.Contains(suite.T(), result.Attributes, "userTypeHandle")
	assert.Contains(suite.T(), result.Attributes, "userTypeName")
	assert.Contains(suite.T(), result.Attributes, "ouId")
	assert.Contains(suite.T(), result.Attributes, "ouName")
	assert.Contains(suite.T(), result.Attributes, "ouHandle")
	assert.Contains(suite.T(), result.Attributes, "groups")
	assert.Contains(suite.T(), result.Attributes, "roles")
	assert.Len(suite.T(), result.Attributes, 8)
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
	assert.NotContains(suite.T(), result.Attributes, "roles")
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
			expectedContains: []string{"userType", "userTypeHandle", "userTypeName", "email"},
			expectedAbsent:   []string{"ouId", "ouName", "ouHandle", "groups", "roles"},
		},
		{
			name:             "OUID only",
			ouID:             "ou-456",
			expectedContains: []string{"ouId", "ouName", "ouHandle", "email"},
			expectedAbsent:   []string{"userType", "userTypeHandle", "userTypeName", "groups", "roles"},
		},
		{
			name:             "EntityID only",
			entityID:         testUserID,
			expectedContains: []string{"groups", "roles", "email"},
			expectedAbsent:   []string{"userType", "userTypeHandle", "userTypeName", "ouId", "ouName", "ouHandle"},
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

func (suite *ConsentExecutorTestSuite) TestBuildAugmentedAvailableAttributes_WithGroupsAndRoles() {
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
	assert.Contains(suite.T(), result.Attributes, "roles")
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
	assert.Contains(suite.T(), result.Attributes, "userTypeHandle")
	assert.Contains(suite.T(), result.Attributes, "userTypeName")
	assert.Contains(suite.T(), result.Attributes, "ouId")
	assert.Contains(suite.T(), result.Attributes, "ouName")
	assert.Contains(suite.T(), result.Attributes, "ouHandle")
	assert.Contains(suite.T(), result.Attributes, "groups")
	assert.Contains(suite.T(), result.Attributes, "roles")
	// Total: 1 original + 8 special
	assert.Len(suite.T(), result.Attributes, 9)
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
	assert.NotContains(suite.T(), availableAttrs.Attributes, "roles")
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
		mock.Anything, mock.Anything,
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
	assert.Contains(suite.T(), result.Attributes, "roles")
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

// ----- Authorization Details Tests -----

func authorizationDetail(detailID string) providers.AuthorizationDetail {
	value := json.RawMessage(`{"type":"payment_initiation","amount":10}`)
	return providers.AuthorizationDetail{ID: detailID, Type: "payment_initiation", TypeID: "type-1",
		TypeVersion: 1, Requested: value, Value: value, Admitted: true}
}

func (suite *ConsentExecutorTestSuite) withAuthorizationDetails(
	details ...providers.AuthorizationDetail) *providers.NodeContext {
	suite.setupDefaultAuthnProviderMocks()
	ctx := buildConsentNodeContext()
	encoded, err := json.Marshal(details)
	suite.Require().NoError(err)
	ctx.RuntimeData[common.RuntimeKeyRequestedAuthorizationDetails] = string(encoded)
	return ctx
}

func (suite *ConsentExecutorTestSuite) grantedDetailIDs(resp *providers.ExecutorResponse) ([]string, bool) {
	encoded, ok := resp.RuntimeData[common.RuntimeKeyConsentedAuthorizationDetails]
	if !ok {
		return nil, false
	}
	var granted []providers.AuthorizationDetail
	suite.Require().NoError(json.Unmarshal([]byte(encoded), &granted))
	ids := []string{}
	for _, d := range granted {
		ids = append(ids, d.ID)
	}
	return ids, true
}

func (suite *ConsentExecutorTestSuite) expectConsentInputs(ctx *providers.NodeContext, hasInputs bool) {
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("ValidatePrerequisites", ctx, mock.AnythingOfType("*providers.ExecutorResponse"), mock.Anything).Return(true)
	suite.executor.Executor.(*coremock.ExecutorInterfaceMock).
		On("HasRequiredInputs", ctx, mock.AnythingOfType("*providers.ExecutorResponse")).Return(hasInputs)
}

func (suite *ConsentExecutorTestSuite) expectResolve(details any, forceReprompt any,
	prompt *providers.ConsentPromptData, svcErr *tidcommon.ServiceError) {
	suite.mockConsentEnforcer.On("ResolveConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, details, mock.Anything, forceReprompt,
		mock.Anything).Return(prompt, svcErr).Once()
}

func detailPurpose(detailIDs ...string) providers.ConsentPurposePrompt {
	elements := make([]providers.PromptElement, 0, len(detailIDs))
	for _, detailID := range detailIDs {
		elements = append(elements, providers.PromptElement{Name: detailID, DetailType: "payment_initiation",
			Summary: "Pay 10 EUR"})
	}
	return providers.ConsentPurposePrompt{
		PurposeName: consent.AuthorizationDetailPurposeName("app-123"),
		PurposeID:   consent.AuthorizationDetailPurposeName("app-123"),
		Optional:    elements,
	}
}

func detailDecision(detailID string, approved bool) providers.PurposeDecision {
	return providers.PurposeDecision{
		PurposeName: consent.AuthorizationDetailPurposeName("app-123"),
		Approved:    approved,
		Elements:    []providers.ElementDecision{{Name: detailID, Approved: approved}},
	}
}

func (suite *ConsentExecutorTestSuite) decided(decisions providers.ConsentDecisions,
	details ...providers.AuthorizationDetail) *providers.NodeContext {
	ctx := suite.withAuthorizationDetails(details...)
	decisionsJSON, err := json.Marshal(decisions)
	suite.Require().NoError(err)
	ctx.UserInputs[userInputConsentDecisions] = string(decisionsJSON)
	ctx.RuntimeData[common.RuntimeKeyConsentSessionToken] = "session"
	suite.expectConsentInputs(ctx, true)
	return ctx
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AuthorizationDetailsArePassedToTheEnforcer() {
	ctx := suite.withAuthorizationDetails(authorizationDetail("d-1"), authorizationDetail("d-2"))
	suite.expectConsentInputs(ctx, false)
	suite.expectResolve([]providers.AuthorizationDetail{authorizationDetail("d-1"), authorizationDetail("d-2")},
		false, &providers.ConsentPromptData{SessionToken: "session", Purposes: []providers.ConsentPurposePrompt{
			{PurposeName: "attributes:app-123"}, detailPurpose("d-1")}}, nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	suite.Equal("session", resp.RuntimeData[common.RuntimeKeyConsentSessionToken])
	_, granted := suite.grantedDetailIDs(resp)
	suite.False(granted, "nothing is granted before the user decides")

	var prompt []providers.ConsentPurposePrompt
	suite.Require().NoError(json.Unmarshal([]byte(resp.AdditionalData[common.DataConsentPrompt]), &prompt))
	suite.Require().Len(prompt, 2)
	suite.Equal("Pay 10 EUR", prompt[1].Optional[0].Summary)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_ForcedRepromptIsPassedOn() {
	ctx := suite.withAuthorizationDetails(authorizationDetail("d-1"))
	ctx.RuntimeData[common.RuntimeKeyForceConsentReprompt] = dataValueTrue
	suite.expectConsentInputs(ctx, false)
	suite.expectResolve(mock.Anything, true, &providers.ConsentPromptData{
		SessionToken: "session", Purposes: []providers.ConsentPurposePrompt{detailPurpose("d-1")}}, nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AllAuthorizationDetailsCovered() {
	ctx := suite.withAuthorizationDetails(authorizationDetail("d-1"))
	suite.expectConsentInputs(ctx, false)
	suite.expectResolve(mock.Anything, mock.Anything, nil, nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status, "covered details need no prompt")
	granted, _ := suite.grantedDetailIDs(resp)
	suite.Equal([]string{"d-1"}, granted)
}

// Details that skipped their processor are refused before the consent provider is asked, so a provider
// that returns no prompt cannot grant them.
func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_UnadmittedDetailIsRefused() {
	notAdmitted := authorizationDetail("d-2")
	notAdmitted.Admitted = false
	ctx := suite.withAuthorizationDetails(authorizationDetail("d-1"), notAdmitted)
	suite.expectConsentInputs(ctx, false)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(&ErrAuthorizationDetailsNotChecked, resp.Error)
	suite.NotContains(resp.RuntimeData, common.RuntimeKeyConsentedAuthorizationDetails)
	suite.mockConsentEnforcer.AssertNotCalled(suite.T(), "ResolveConsent", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_NoAuthorizationDetailsGrantsNothing() {
	suite.setupDefaultAuthnProviderMocks()
	ctx := buildConsentNodeContext()
	suite.expectConsentInputs(ctx, false)
	suite.expectResolve([]providers.AuthorizationDetail(nil), mock.Anything, nil, nil)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	_, granted := suite.grantedDetailIDs(resp)
	suite.False(granted, "a request without authorization details records no grant")
}

func (suite *ConsentExecutorTestSuite) TestExecute_MalformedAuthorizationDetails() {
	suite.setupDefaultAuthnProviderMocks()
	ctx := buildConsentNodeContext()
	ctx.RuntimeData[common.RuntimeKeyRequestedAuthorizationDetails] = "not json"
	suite.expectConsentInputs(ctx, false)

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_GrantsApprovedAndCoveredDetails() {
	ctx := suite.decided(providers.ConsentDecisions{Approved: true, Purposes: []providers.PurposeDecision{
		{PurposeName: "attributes:app-123", Approved: true,
			Elements: []providers.ElementDecision{{Name: "email", Approved: true}}},
		detailDecision("d-1", true), detailDecision("d-2", false),
	}}, authorizationDetail("d-1"), authorizationDetail("d-2"), authorizationDetail("d-3"))
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, "default", "app-123", testUserID,
		mock.MatchedBy(func(d *providers.ConsentDecisions) bool { return len(d.Purposes) == 3 }),
		"session", mock.Anything, mock.Anything).
		Return(&providers.Consent{ID: "consent-rar", GrantedAuthorizationDetails: []providers.AuthorizationDetail{
			authorizationDetail("d-3"), authorizationDetail("d-1")}}, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal("consent-rar", resp.RuntimeData[common.RuntimeKeyConsentID])
	granted, _ := suite.grantedDetailIDs(resp)
	suite.Equal([]string{"d-1", "d-3"}, granted, "d-2 was declined, d-3 was covered, in the order requested")
}

// A requested detail the record does not grant, declined or not covered, is not granted.
func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_UngrantedDetailIsNotGranted() {
	ctx := suite.decided(providers.ConsentDecisions{Approved: true, Purposes: []providers.PurposeDecision{
		detailDecision("d-1", true), detailDecision("d-2", false)}},
		authorizationDetail("d-1"), authorizationDetail("d-2"))
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&providers.Consent{ID: "consent-rar",
		GrantedAuthorizationDetails: []providers.AuthorizationDetail{authorizationDetail("d-1")}}, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	granted, _ := suite.grantedDetailIDs(resp)
	suite.Equal([]string{"d-1"}, granted)
}

// A detail whose type changed after the prompt was shown fails the step without granting anything.
func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_DetailOfAChangedType() {
	ctx := suite.decided(providers.ConsentDecisions{Approved: true, Purposes: []providers.PurposeDecision{
		detailDecision("d-1", false)}}, authorizationDetail("d-1"), authorizationDetail("d-2"))
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &authzdetail.ErrorAuthorizationDetailTypeChanged).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(&ErrAuthorizationDetailsUnavailable, resp.Error)
	suite.NotContains(resp.RuntimeData, common.RuntimeKeyConsentedAuthorizationDetails)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_DetailsOnlyPromptKeepingNothingSetsNoConsentID() {
	ctx := suite.decided(providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{detailDecision("d-1",
			true)}}, authorizationDetail("d-1"))
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&providers.Consent{
		GrantedAuthorizationDetails: []providers.AuthorizationDetail{authorizationDetail("d-1")}}, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.NotContains(resp.RuntimeData, common.RuntimeKeyConsentID)
	granted, _ := suite.grantedDetailIDs(resp)
	suite.Equal([]string{"d-1"}, granted)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_DeniedPromptGrantsNoDetails() {
	ctx := suite.decided(providers.ConsentDecisions{Approved: false,
		Purposes: []providers.PurposeDecision{detailDecision("d-1", true)}},
		authorizationDetail("d-1"), authorizationDetail("d-2"))
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&providers.Consent{ID: "consent-1"}, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	granted, ok := suite.grantedDetailIDs(resp)
	suite.True(ok, "a denial is a decision")
	suite.Empty(granted, "a denied prompt grants neither prompted nor covered details")
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_AuthorizationDetailsOnly_FailOnDeny() {
	ctx := suite.decided(providers.ConsentDecisions{Approved: false,
		Purposes: []providers.PurposeDecision{detailDecision("d-1",
			false)}}, authorizationDetail("d-1"))
	ctx.NodeProperties[propertyKeyConsentFailOnDeny] = true
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&providers.Consent{}, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(&ErrConsentDenied, resp.Error)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_AuthorizationDetailsTimeout() {
	ctx := suite.decided(providers.ConsentDecisions{Reason: providers.ConsentDecisionReasonTimeout},
		authorizationDetail("d-1"), authorizationDetail("d-2"))

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	granted, ok := suite.grantedDetailIDs(resp)
	suite.True(ok)
	suite.Empty(granted, "a timed out prompt grants nothing")
	suite.mockConsentEnforcer.AssertNotCalled(suite.T(), "RecordConsent", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_GrantsTheDetailsAsTheRecordDecidesThem() {
	ctx := suite.decided(providers.ConsentDecisions{Approved: true, Purposes: []providers.PurposeDecision{
		detailDecision("d-1", true), detailDecision("d-2", true),
	}}, authorizationDetail("d-1"), authorizationDetail("d-2"), authorizationDetail("d-3"))
	modified := authorizationDetail("d-1")
	modified.Value = json.RawMessage(`{"type":"payment_initiation","amount":5}`)
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&providers.Consent{
		GrantedAuthorizationDetails: []providers.AuthorizationDetail{modified, authorizationDetail("d-3")}}, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	var granted []providers.AuthorizationDetail
	suite.Require().NoError(json.Unmarshal(
		[]byte(resp.RuntimeData[common.RuntimeKeyConsentedAuthorizationDetails]), &granted))
	suite.Require().Len(granted, 2, "d-2 is not granted by the record; d-3 was covered")
	suite.Equal("d-1", granted[0].ID)
	suite.JSONEq(`{"type":"payment_initiation","amount":5}`, string(granted[0].Value))
	suite.Equal("d-3", granted[1].ID)
	suite.JSONEq(`{"type":"payment_initiation","amount":10}`, string(granted[1].Value))
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_ChangeRefused() {
	ctx := suite.decided(providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{detailDecision("d-1", true)}},
		authorizationDetail("d-1"))
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, &consentauthn.ErrorAuthorizationDetailModificationRefused).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(&ErrAuthorizationDetailsChangeRefused, resp.Error)
}

// undecidedDetailsCodes maps the errors that end a flow's authorization details without a server error to the
// flow error each fails the step with.
var undecidedDetailsCodes = map[string]*tidcommon.ServiceError{
	consentauthn.ErrorAuthorizationDetailsNotAdmitted.Code: &ErrAuthorizationDetailsNotChecked,
	authzdetail.ErrorAuthorizationDetailTypeChanged.Code:   &ErrAuthorizationDetailsUnavailable,
}

func (suite *ConsentExecutorTestSuite) TestExecute_NoInputs_AuthorizationDetailsCannotBeDecided() {
	for code, want := range undecidedDetailsCodes {
		suite.Run(code, func() {
			suite.SetupTest()
			ctx := suite.withAuthorizationDetails(authorizationDetail("d-1"))
			suite.expectConsentInputs(ctx, false)
			suite.expectResolve(mock.Anything, mock.Anything, nil,
				&tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: code})

			resp, err := suite.executor.Execute(ctx)

			suite.NoError(err)
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Equal(want, resp.Error)
		})
	}
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_AuthorizationDetailsCannotBeDecided() {
	for code, want := range undecidedDetailsCodes {
		suite.Run(code, func() {
			suite.SetupTest()
			ctx := suite.decided(providers.ConsentDecisions{Approved: true,
				Purposes: []providers.PurposeDecision{detailDecision("d-1", true)}},
				authorizationDetail("d-1"))
			suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything,
				mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil, &tidcommon.ServiceError{Type: tidcommon.ServerErrorType, Code: code}).Once()

			resp, err := suite.executor.Execute(ctx)

			suite.NoError(err)
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Equal(want, resp.Error)
			_, granted := suite.grantedDetailIDs(resp)
			suite.False(granted)
		})
	}
}

func (suite *ConsentExecutorTestSuite) TestExecute_HasInputs_RecordConsentFailsWithAuthorizationDetails() {
	ctx := suite.decided(providers.ConsentDecisions{Approved: true,
		Purposes: []providers.PurposeDecision{detailDecision("d-1",
			true)}}, authorizationDetail("d-1"))
	suite.mockConsentEnforcer.On("RecordConsent", mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, &tidcommon.InternalServerError).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
}

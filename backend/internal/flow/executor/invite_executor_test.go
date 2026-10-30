// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/config"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
)

type InviteExecutorTestSuite struct {
	suite.Suite
	mockFlowFactory *coremock.FlowFactoryInterfaceMock
	executor        *inviteExecutor
}

func (suite *InviteExecutorTestSuite) SetupTest() {
	// Initialize runtime config for tests
	err := config.InitializeServerRuntime(".", &config.Config{
		GateClient: engineconfig.GateClientConfig{
			Scheme:   "https",
			Hostname: "localhost",
			Port:     5190,
			Path:     "/gate",
		},
	})
	suite.Require().NoError(err)

	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockBaseExecutor := coremock.NewExecutorInterfaceMock(suite.T())

	// Set up expectations for CreateExecutor (called in constructor)
	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameInviteExecutor,
		providers.ExecutorTypeUtility,
		[]providers.Input{
			{
				Identifier: userInputInviteToken,
				Type:       "HIDDEN",
				Required:   true,
			},
		},
		[]providers.Input{}, mock.Anything).Return(mockBaseExecutor)

	suite.executor = newInviteExecutor(suite.mockFlowFactory)
}

func (suite *InviteExecutorTestSuite) TearDownTest() {
	config.ResetServerRuntime()
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		EntityID:     "test-app-id",
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyStoredInviteToken])
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink])
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "inviteToken=")
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "executionId=test-flow-id")
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "applicationId=test-app-id")
	assert.Empty(suite.T(), resp.AdditionalData[common.DataInviteLink])
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_UserOnboarding_ExposesInviteLink() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		EntityID:     "test-app-id",
		FlowType:     providers.FlowTypeUserOnboarding,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink])
	assert.Equal(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], resp.AdditionalData[common.DataInviteLink])
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_Idempotency() {
	existingToken := "existing-token-123"
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData: map[string]string{
			common.RuntimeKeyStoredInviteToken: existingToken,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), existingToken, resp.RuntimeData[common.RuntimeKeyStoredInviteToken])
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], existingToken)
	assert.Empty(suite.T(), resp.AdditionalData[common.DataInviteLink])
}

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_NoTokenProvided() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeVerify,
		UserInputs:   make(map[string]string),
		RuntimeData: map[string]string{
			common.RuntimeKeyStoredInviteToken: "stored-token",
		},
	}

	mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(false)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
}

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_ValidationSuccess() {
	token := "valid-token"
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputInviteToken: token,
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyStoredInviteToken: token,
		},
	}

	mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_ValidationFailure_Mismatch() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputInviteToken: "wrong-token",
		},
		RuntimeData: map[string]string{
			common.RuntimeKeyStoredInviteToken: "correct-token",
		},
	}

	mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrInvalidInviteToken.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_ValidationFailure_NoStoredToken() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: ExecutorModeVerify,
		UserInputs: map[string]string{
			userInputInviteToken: "some-token",
		},
		RuntimeData: make(map[string]string),
	}

	mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrInvalidInviteToken.Error.DefaultValue, resp.Error.Error.DefaultValue)
}

func (suite *InviteExecutorTestSuite) TestExecute_InvalidMode() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		ExecutorMode: "invalid",
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
	assert.Contains(suite.T(), err.Error(), "invalid executor mode for InviteExecutor")
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_PopulatesTemplateData() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeGenerate,
		FlowType:     providers.FlowTypeRegistration,
		RuntimeData:  make(map[string]string),
		Application: providers.Application{
			Name: "Test App",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)

	// 1. Ensure TemplateName is completely gone
	_, hasTemplateName := resp.ForwardedData["templateName"]
	suite.False(hasTemplateName, "Template name should no longer be set by Invite Executor")

	// 2. Ensure TemplateData IS set with the link
	templateData, ok := resp.ForwardedData[common.ForwardedDataKeyTemplateData].(map[string]interface{})
	suite.True(ok, "Expected template data to be map[string]interface{}")
	suite.NotEmpty(templateData["inviteLink"], "inviteLink must be present")
	suite.Equal("Test App", templateData["appName"])
}

func (suite *InviteExecutorTestSuite) TestGenerateInviteLink_UsesInviteBaseURLProperty() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
		NodeProperties: map[string]interface{}{
			propertyKeyInviteBaseURL: "http://localhost:3000",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "http://localhost:3000?")
}

func (suite *InviteExecutorTestSuite) TestGenerateInviteLink_FallsBackToGateConfigWhenPropertyAbsent() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-execution-id",
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "https://localhost:5190/gate/invite?")
}

func (suite *InviteExecutorTestSuite) TestGetExecutionPolicy_GenerateMode_ReturnsNil() {
	policy := suite.executor.GetExecutionPolicy(ExecutorModeGenerate)
	assert.Nil(suite.T(), policy)
}

func (suite *InviteExecutorTestSuite) TestGetExecutionPolicy_VerifyMode_SkipsChallengeValidation() {
	policy := suite.executor.GetExecutionPolicy(ExecutorModeVerify)
	assert.NotNil(suite.T(), policy)
	assert.True(suite.T(), policy.SkipChallengeValidation)
}

func (suite *InviteExecutorTestSuite) TestGetExecutionPolicy_VerifyMode_AllowsSegmentRestart() {
	policy := suite.executor.GetExecutionPolicy(ExecutorModeVerify)
	assert.NotNil(suite.T(), policy)
	assert.True(suite.T(), policy.AllowSegmentRestart)
}

func (suite *InviteExecutorTestSuite) TestGetExecutionPolicy_InvalidMode_ReturnsNil() {
	policy := suite.executor.GetExecutionPolicy("invalid-mode")
	assert.Nil(suite.T(), policy)
}

func (suite *InviteExecutorTestSuite) TestGetExecutionPolicy_EmptyMode_ReturnsNil() {
	policy := suite.executor.GetExecutionPolicy("")
	assert.Nil(suite.T(), policy)
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_IncludesAuthReqIDWhenPresent() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		EntityID:     "test-app-id",
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData: map[string]string{
			common.RuntimeKeyAuthorizationRequestID: "ciba-req-123",
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "auth_req_id=ciba-req-123")
	assert.Contains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "executionId=test-flow-id")
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_NoAuthReqIDWhenAbsent() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		EntityID:     "test-app-id",
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.NotContains(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink], "auth_req_id")
}

// A pinned contact's invite is always a fresh token, and its proof restarts with it.
func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_PinnedContactIssuesAFreshToken() {
	ctx := &providers.NodeContext{
		ExecutionID:       "test-flow-id",
		FlowType:          providers.FlowTypeAdministration,
		ExecutorMode:      ExecutorModeGenerate,
		UserInputs:        make(map[string]string),
		RuntimeData:       map[string]string{common.RuntimeKeyStoredInviteToken: "existing-token-123"},
		SharedRuntimeData: pinnedEmailEvidence(suite.T(), "existing-token-123", true),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	token := resp.RuntimeData[common.RuntimeKeyStoredInviteToken]
	assert.NotEmpty(suite.T(), token)
	assert.NotEqual(suite.T(), "existing-token-123", token)
	assert.Equal(suite.T(), &verificationEvidence{Pin: emailPin(), Challenge: token},
		evidenceIn(suite.T(), resp.SharedRuntimeData))
	assert.Empty(suite.T(), resp.AdditionalData[common.DataInviteLink])
}

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_PinnedContactMarksPassed() {
	ctx := &providers.NodeContext{
		ExecutionID:       "test-flow-id",
		FlowType:          providers.FlowTypeSelfService,
		ExecutorMode:      ExecutorModeVerify,
		UserInputs:        map[string]string{userInputInviteToken: "invite-1"},
		RuntimeData:       map[string]string{common.RuntimeKeyStoredInviteToken: "invite-1"},
		SharedRuntimeData: pinnedEmailEvidence(suite.T(), "invite-1", false),
	}
	mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	proof := evidenceIn(suite.T(), resp.SharedRuntimeData)
	assert.True(suite.T(), proof.Passed)
	assert.Equal(suite.T(), "invite-1", proof.Challenge)
}

// A token restored from an older CALL frame, or one issued before the pin, is not the active
// challenge and passes nothing.
func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_PinnedContactRefusesAnInactiveToken() {
	for name, challenge := range map[string]string{
		"restored older token":    "new-invite",
		"token issued before pin": "",
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			ctx := &providers.NodeContext{
				ExecutionID:       "test-flow-id",
				FlowType:          providers.FlowTypeSelfService,
				ExecutorMode:      ExecutorModeVerify,
				UserInputs:        map[string]string{userInputInviteToken: "old-invite"},
				RuntimeData:       map[string]string{common.RuntimeKeyStoredInviteToken: "old-invite"},
				SharedRuntimeData: pinnedEmailEvidence(suite.T(), challenge, false),
			}
			mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
			mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			suite.Require().Equal(providers.ExecFailure, resp.Status)
			assert.Equal(suite.T(), ErrInvalidInviteToken.Code, resp.Error.Code)
			assert.Empty(suite.T(), resp.SharedRuntimeData)
		})
	}
}

func (suite *InviteExecutorTestSuite) TestExecute_CorruptEvidenceIsAnError() {
	for _, mode := range []string{ExecutorModeGenerate, ExecutorModeVerify} {
		suite.Run(mode, func() {
			suite.SetupTest()
			ctx := &providers.NodeContext{
				ExecutionID:       "test-flow-id",
				FlowType:          providers.FlowTypeSelfService,
				ExecutorMode:      mode,
				UserInputs:        map[string]string{userInputInviteToken: "invite-1"},
				RuntimeData:       map[string]string{common.RuntimeKeyStoredInviteToken: "invite-1"},
				SharedRuntimeData: map[string]string{common.RuntimeKeyAttributeVerificationEvidence: "{"},
			}
			mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
			mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true).Maybe()

			_, err := suite.executor.Execute(ctx)

			assert.Error(suite.T(), err)
		})
	}
}

// Without a pin, generate and verify behave as before in a self-service flow: the stored token is
// reused and no evidence is written.
func (suite *InviteExecutorTestSuite) TestExecute_WithoutPinBehavesAsBefore() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		FlowType:     providers.FlowTypeSelfService,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   map[string]string{userInputInviteToken: "invite-1"},
		RuntimeData:  map[string]string{common.RuntimeKeyStoredInviteToken: "invite-1"},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal("invite-1", resp.RuntimeData[common.RuntimeKeyStoredInviteToken])
	suite.Empty(resp.SharedRuntimeData)

	ctx.ExecutorMode = ExecutorModeVerify
	mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

	resp, err = suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Empty(resp.SharedRuntimeData)
}

// The email link path: a record before the user confirms the link is refused, and inputs given later
// cannot retarget the pinned user or attribute.
func (suite *InviteExecutorTestSuite) TestExecute_PinnedContactRecordsOnlyAfterVerify() {
	config.ResetServerRuntime()
	factory := newVerificationTestFactory(suite.T())
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(suite.T())
	verification := newAttributeVerificationExecutor(factory, governance, nil)
	invite := newInviteExecutor(factory)
	pin := providers.AttributeVerificationPin{Subject: "user-a", Attribute: "workEmail",
		Value: "User@example.com", Channel: providers.VerificationChannelEmail}
	governance.On("PrepareAttributeVerification", mock.Anything, pin.Subject, pin.Attribute).
		Return(&pin, nil).Once()
	governance.On("RecordAttributeVerification", mock.Anything, pin).Return(nil).Once()
	ctx := selfPrepareContext(pin.Subject, map[string]string{"attribute": pin.Attribute}, map[string]string{})

	prepared, err := verification.Execute(ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(providers.ExecComplete, prepared.Status)
	// The shipped sender reads the channel's default attribute; a custom one may read the attribute.
	suite.Equal(pin.Value, prepared.RuntimeData[userAttributeEmail])
	suite.Equal(pin.Value, prepared.RuntimeData[pin.Attribute])
	suite.ElementsMatch([]string{"attribute"}, ctx.GetConsumedInputs())
	applyVerificationResponse(ctx, prepared)

	ctx.ExecutorMode = ExecutorModeGenerate
	generated, err := invite.Execute(ctx)
	suite.Require().NoError(err)
	applyVerificationResponse(ctx, generated)
	token := ctx.RuntimeData[common.RuntimeKeyStoredInviteToken]
	suite.Require().NotEmpty(token)
	suite.NotContains(generated.AdditionalData, common.DataInviteLink)

	proof, err := pinnedVerificationEvidence(ctx)
	suite.Require().NoError(err)
	suite.Require().True(deliveryAllowed(proof, providers.VerificationChannelEmail, pin.Value))
	delivery := &providers.ExecutorResponse{}
	recordVerificationReceipt(delivery, proof, providers.VerificationChannelEmail, pin.Value)
	applyVerificationResponse(ctx, delivery)

	ctx.ExecutorMode = ExecutorModeRecord
	beforeConfirmation, err := verification.Execute(ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(providers.ExecFailure, beforeConfirmation.Status)

	ctx.ExecutorMode = ExecutorModeVerify
	ctx.UserInputs = map[string]string{userInputInviteToken: token}
	verified, err := invite.Execute(ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(providers.ExecComplete, verified.Status)
	applyVerificationResponse(ctx, verified)

	ctx.ExecutorMode = ExecutorModeRecord
	ctx.UserInputs = map[string]string{"subject": "user-b", "attribute": "email"}
	recorded, err := verification.Execute(ctx)
	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, recorded.Status)
	suite.Empty(recorded.SharedRuntimeData[common.RuntimeKeyAttributeVerificationEvidence])
}

func TestInviteExecutorSuite(t *testing.T) {
	suite.Run(t, new(InviteExecutorTestSuite))
}

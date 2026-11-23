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
	notifcm "github.com/thunder-id/thunderid/internal/notification/common"
	"github.com/thunder-id/thunderid/internal/notificationtemplate"
	"github.com/thunder-id/thunderid/internal/system/config"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/tests/mocks/attributeverificationprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/notification/notificationmock"
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
	assert.Empty(suite.T(), resp.EngineData[common.EngineDataKeyApprovedActions], "approves nothing unasked")
}

// The node approves the writes its approvedActions property names, resolved from the runtime data alone: a unit
// takes the first candidate that resolves, and an entry whose resources resolve to none approves nothing.
func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_ApprovesTheNamedActions() {
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow-id", ExecutorMode: ExecutorModeGenerate, UserInputs: map[string]string{"ouId": "ou-x"},
		RuntimeData: map[string]string{"defaultOUID": "ou-type", "selectedRoles": "role-1 role-2 role-1"},
		NodeProperties: map[string]interface{}{propertyKeyApprovedActions: []interface{}{
			map[string]interface{}{"action": "user:create",
				"ouId": []interface{}{"{{ctx(ouId)}}", "{{ctx(defaultOUID)}}"}},
			map[string]interface{}{"action": "role:update", "ouId": "{{ctx(defaultOUID)}}",
				"resourceIds": "{{ctx(selectedRoles)}}"},
			map[string]interface{}{"action": "group:update", "ouId": "{{ctx(defaultOUID)}}",
				"resourceIds": "{{ctx(selectedGroups)}}"},
		}},
	}

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.JSONEq(`[{"action":"user:create","ouId":"ou-type"},
		{"action":"role:update","ouId":"ou-type","resourceIds":["role-1","role-2"]}]`,
		resp.EngineData[common.EngineDataKeyApprovedActions])
}

func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_RefusesAMalformedApproval() {
	for name, property := range map[string]interface{}{
		"not a list":        "user:create",
		"not an object":     []interface{}{"user:create"},
		"no action":         []interface{}{map[string]interface{}{"ouId": "ou-1"}},
		"unresolved unit":   []interface{}{map[string]interface{}{"action": "user:create", "ouId": "{{ctx(ouId)}}"}},
		"no unit candidate": []interface{}{map[string]interface{}{"action": "user:create"}},
	} {
		suite.Run(name, func() {
			_, err := suite.executor.Execute(&providers.NodeContext{ExecutorMode: ExecutorModeGenerate,
				RuntimeData: map[string]string{}, NodeProperties: map[string]interface{}{
					propertyKeyApprovedActions: property}})

			suite.Error(err)
		})
	}
}

// The admin who sends an onboarding link gets it back, for a screen that shows it when no email can be sent.
func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_OnboardingExposesTheInviteLink() {
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
	assert.Equal(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink],
		resp.AdditionalData[common.DataInviteLink])
}

// The admin who invites does not get the link, with which they could accept for an existing user; the email step
// surfaces it only when no email can be sent.
func (suite *InviteExecutorTestSuite) TestExecute_GenerateMode_InvitationKeepsTheInviteLinkFromTheAdmin() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		EntityID:     "test-app-id",
		FlowType:     providers.FlowTypeInvitation,
		ExecutorMode: ExecutorModeGenerate,
		UserInputs:   make(map[string]string),
		RuntimeData:  make(map[string]string),
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyInviteLink])
	assert.Empty(suite.T(), resp.AdditionalData[common.DataInviteLink])
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

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_PinnedContactMarksPassed() {
	ctx := &providers.NodeContext{
		ExecutionID:       "test-flow-id",
		FlowType:          providers.FlowTypeSelf,
		ExecutorMode:      ExecutorModeVerify,
		UserInputs:        map[string]string{userInputInviteToken: "invite-1"},
		RuntimeData:       map[string]string{common.RuntimeKeyStoredInviteToken: "invite-1"},
		SharedRuntimeData: pinnedEmailEvidence(suite.T(), false),
	}
	mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(&verificationEvidence{Pin: emailPin(), Passed: true}, evidenceIn(suite.T(), resp.SharedRuntimeData))
}

// A token other than the stored one, or none stored, passes nothing for a pinned contact.
func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_PinnedContactWrongTokenDoesNotPass() {
	for name, runtime := range map[string]map[string]string{
		"another token":   {common.RuntimeKeyStoredInviteToken: "invite-1"},
		"no stored token": {},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			ctx := &providers.NodeContext{
				ExecutionID:       "test-flow-id",
				FlowType:          providers.FlowTypeSelf,
				ExecutorMode:      ExecutorModeVerify,
				UserInputs:        map[string]string{userInputInviteToken: "invite-other"},
				RuntimeData:       runtime,
				SharedRuntimeData: pinnedEmailEvidence(suite.T(), false),
			}
			mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
			mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

			resp, err := suite.executor.Execute(ctx)

			suite.NoError(err)
			suite.Require().Equal(providers.ExecFailure, resp.Status)
			suite.Equal(ErrInvalidInviteToken.Code, resp.Error.Code)
			suite.Nil(resp.SharedRuntimeData)
		})
	}
}

// A cleared stored token matches no token, an empty one included.
func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_ClearedTokenDoesNotPass() {
	ctx := &providers.NodeContext{
		ExecutionID:       "test-flow-id",
		FlowType:          providers.FlowTypeSelf,
		ExecutorMode:      ExecutorModeVerify,
		UserInputs:        map[string]string{userInputInviteToken: ""},
		RuntimeData:       map[string]string{common.RuntimeKeyStoredInviteToken: ""},
		SharedRuntimeData: pinnedEmailEvidence(suite.T(), false),
	}
	mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)

	resp, err := suite.executor.Execute(ctx)

	suite.NoError(err)
	suite.Require().Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrInvalidInviteToken.Code, resp.Error.Code)
	suite.Nil(resp.SharedRuntimeData)
}

func (suite *InviteExecutorTestSuite) TestExecute_VerifyMode_CorruptEvidenceIsAnError() {
	ctx := &providers.NodeContext{
		ExecutionID:       "test-flow-id",
		FlowType:          providers.FlowTypeSelf,
		ExecutorMode:      ExecutorModeVerify,
		UserInputs:        map[string]string{userInputInviteToken: "invite-1"},
		RuntimeData:       map[string]string{common.RuntimeKeyStoredInviteToken: "invite-1"},
		SharedRuntimeData: map[string]string{common.RuntimeKeyAttributeVerificationEvidence: "{"},
	}
	mockExecutor := suite.executor.Executor.(*coremock.ExecutorInterfaceMock)
	mockExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true).Maybe()

	_, err := suite.executor.Execute(ctx)

	suite.Error(err)
}

// Without a pin, generate and verify behave as before in a self flow: the stored token is
// reused and no evidence is written.
func (suite *InviteExecutorTestSuite) TestExecute_WithoutPinBehavesAsBefore() {
	ctx := &providers.NodeContext{
		ExecutionID:  "test-flow-id",
		FlowType:     providers.FlowTypeSelf,
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

// The email link reaches only the pinned address, and nothing is recorded before the link is confirmed.
func (suite *InviteExecutorTestSuite) TestExecute_PinnedContactRecordsOnlyAfterVerify() {
	config.ResetServerRuntime()
	factory := newVerificationTestFactory(suite.T())
	provider := attributeverificationprovidermock.NewAttributeVerificationProviderMock(suite.T())
	verification := newAttributeVerificationExecutor(factory, provider, nil, newAccessGuard(nil))
	invite := newInviteExecutor(factory)
	sender := notificationmock.NewNotificationSenderServiceInterfaceMock(suite.T())
	renderer := newNotificationTemplateRendererMock(suite.T())
	email := newEmailExecutor(factory, sender, renderer, nil)
	pin := providers.AttributeVerificationPin{Subject: "user-a", Attribute: "workEmail",
		Value: "User@example.com", Channel: providers.VerificationChannelEmail}
	provider.On("PrepareAttributeVerification", mock.Anything, existingRequest(pin.Subject, pin.Attribute)).
		Return(&pin, nil).Once()
	provider.On("RecordAttributeVerification", mock.Anything, pin).Return(nil).Once()
	renderer.On("Resolve", mock.Anything, notificationtemplate.ChannelTypeEmail, "verify-email", mock.Anything).
		Return(&notificationtemplate.ResolvedContent{Subject: "Verify", Body: "link"}, nil).Once()
	sender.On("SendEmail", mock.Anything, "", mock.MatchedBy(func(sent notifcm.EmailData) bool {
		return len(sent.To) == 1 && sent.To[0] == pin.Value
	})).Return((*tidcommon.ServiceError)(nil)).Once()
	ctx := selfPrepareContext(pin.Subject, map[string]string{"attribute": pin.Attribute}, map[string]string{})
	ctx.UserInputs[userAttributeEmail] = "other@example.com"

	prepared, err := verification.Execute(ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(providers.ExecComplete, prepared.Status)
	suite.NotContains(prepared.RuntimeData, userAttributeEmail)
	suite.NotContains(prepared.RuntimeData, pin.Attribute)
	suite.ElementsMatch([]string{"attribute"}, ctx.GetConsumedInputs())
	applyVerificationResponse(ctx, prepared)

	ctx.ExecutorMode = ExecutorModeGenerate
	generated, err := invite.Execute(ctx)
	suite.Require().NoError(err)
	applyVerificationResponse(ctx, generated)
	token := ctx.RuntimeData[common.RuntimeKeyStoredInviteToken]
	suite.Require().NotEmpty(token)
	suite.NotContains(generated.AdditionalData, common.DataInviteLink)

	ctx.ExecutorMode = ExecutorModeSend
	ctx.NodeProperties = map[string]interface{}{propertyKeyEmailTemplate: "verify-email"}
	delivered, err := email.Execute(ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(providers.ExecComplete, delivered.Status, "the link must go to the pinned address")
	applyVerificationResponse(ctx, delivered)

	ctx.ExecutorMode = ExecutorModeRecord
	beforeConfirmation, err := verification.Execute(ctx)
	suite.Require().NoError(err)
	suite.Require().Equal(providers.ExecFailure, beforeConfirmation.Status)
	provider.AssertNotCalled(suite.T(), "RecordAttributeVerification", mock.Anything, mock.Anything)

	// The flow confirms the link from the state before that refusal.
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
	suite.Equal(clearedShared(), recorded.SharedRuntimeData)
}

func TestInviteExecutorSuite(t *testing.T) {
	suite.Run(t, new(InviteExecutorTestSuite))
}

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"cmp"
	"context"
	"encoding/json"
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	entitytypemodel "github.com/thunder-id/thunderid/internal/entitytype/model"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/system/security"
	"github.com/thunder-id/thunderid/internal/user"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/usermgtprovidermock"
)

type CredentialSetterTestSuite struct {
	suite.Suite
	mockFlowFactory     *coremock.FlowFactoryInterfaceMock
	mockEntityProvider  *entityprovidermock.EntityProviderInterfaceMock
	mockUserMgtProvider *usermgtprovidermock.UserMgtProviderMock
	mockAuthnProvider   *managermock.AuthnProviderManagerMock
	mockBaseExecutor    *coremock.ExecutorInterfaceMock
	mockEntityTypes     *entitytypemock.EntityTypeServiceInterfaceMock
	executor            *credentialSetter
}

func (suite *CredentialSetterTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockUserMgtProvider = usermgtprovidermock.NewUserMgtProviderMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
	suite.mockBaseExecutor = coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockEntityTypes = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameCredentialSetter,
		providers.ExecutorTypeRegistration,
		mock.Anything,
		[]providers.Input{
			{
				Identifier: userAttributeUserID,
				Type:       providers.InputTypeText,
				Required:   true,
			},
		}, mock.Anything).Return(suite.mockBaseExecutor)

	suite.executor = newCredentialSetter(suite.mockFlowFactory, suite.mockEntityProvider, suite.mockUserMgtProvider,
		suite.mockAuthnProvider,
		newAccessGuard(nil), suite.mockEntityTypes)
}

func (suite *CredentialSetterTestSuite) TestExecute_Success() {
	userID := testUserID
	password := "securePass123!"
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: password,
		},
		RuntimeData: map[string]string{
			"userID": userID,
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx, mock.Anything, mock.Anything).Return(userID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]providers.Input{
		{
			Identifier: userAttributePassword,
			Type:       providers.InputTypePassword,
			Required:   true,
		},
	})

	// Use mock.Anything for credentials JSON bytes to avoid strict byte checking
	suite.expectEntity(providers.EntityCategoryUser)
	suite.mockUserMgtProvider.On("UpdateUserCredentials", mock.Anything, userID, mock.Anything).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

// A set outside a reset, even on a recorded administrator's authority, is not marked as completing a reset,
// so its write is audited as the caller's.
func (suite *CredentialSetterTestSuite) TestExecute_SetOutsideAResetIsNotAResetCompletion() {
	ctx := &providers.NodeContext{
		Context:     security.WithRecordedCaller(context.Background(), "admin-1", "ou-1", nil),
		ExecutionID: "test-flow",
		FlowType:    providers.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{userAttributePassword: "securePass123!"},
		RuntimeData: map[string]string{"userID": testUserID},
	}
	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx, mock.Anything, mock.Anything).Return(testUserID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]providers.Input{
		{Identifier: userAttributePassword, Type: providers.InputTypePassword, Required: true},
	})
	suite.expectEntity(providers.EntityCategoryUser)
	suite.mockUserMgtProvider.EXPECT().UpdateUserCredentials(mock.Anything, testUserID, mock.Anything).
		Return(nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
}

func (suite *CredentialSetterTestSuite) TestExecute_MissingInput() {
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  make(map[string]string),
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(false)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
}

func (suite *CredentialSetterTestSuite) TestExecute_MissingUserID() {
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: "password",
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx, mock.Anything, mock.Anything).Return("")

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrUserIDMissingInContext.Code, resp.Error.Code)
}

func (suite *CredentialSetterTestSuite) TestExecute_EmptyPassword() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: "",
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx, mock.Anything, mock.Anything).Return(userID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]providers.Input{
		{
			Identifier: userAttributePassword,
			Type:       providers.InputTypePassword,
			Required:   true,
		},
	})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrCredentialValueEmpty.Code, resp.Error.Code)
}

func (suite *CredentialSetterTestSuite) TestExecute_ServiceError() {
	userID := testUserID
	password := "password"
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: password,
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx, mock.Anything, mock.Anything).Return(userID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]providers.Input{
		{
			Identifier: userAttributePassword,
			Type:       providers.InputTypePassword,
			Required:   true,
		},
	})

	suite.expectEntity(providers.EntityCategoryUser)
	suite.mockUserMgtProvider.On("UpdateUserCredentials", mock.Anything, userID, mock.Anything).
		Return(&tidcommon.InternalServerError)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrCredentialSetFailed.Code, resp.Error.Code)
}

func (suite *CredentialSetterTestSuite) TestExecute_SchemaValidationFailureRequestsInputAgain() {
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: "weak",
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx, mock.Anything, mock.Anything).Return(testUserID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]providers.Input{
		{
			Identifier: userAttributePassword,
			Type:       providers.InputTypePassword,
			Required:   true,
		},
	})

	suite.expectEntity(providers.EntityCategoryUser)
	suite.mockUserMgtProvider.On("UpdateUserCredentials", mock.Anything, testUserID, mock.Anything).
		Return(&user.ErrorInvalidCredentialValue)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), ErrCredentialValueInvalid.Code, resp.Error.Code)
	assert.Equal(suite.T(), userAttributePassword, resp.Error.Error.Params["attribute"])
	assert.Equal(suite.T(), []providers.Input{{
		Identifier: userAttributePassword,
		Type:       providers.InputTypePassword,
		Required:   true,
	}}, resp.Inputs)
	assert.Empty(suite.T(), resp.SharedRuntimeData)
}

func (suite *CredentialSetterTestSuite) TestExecute_CustomAttribute() {
	userID := testUserID
	const customAttr = "pin"
	pinValue := "1234"

	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			customAttr: pinValue,
		},
		RuntimeData: map[string]string{
			"userID": userID,
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", mock.Anything, mock.Anything, mock.Anything).Return(userID)

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]providers.Input{
		{
			Identifier: customAttr,
			Required:   true,
			Type:       providers.InputTypeText,
		},
	})

	// Expect UpdateUserCredentials with custom attribute
	expectedCredentialsJSON := `{"pin":"1234"}` //nolint:gosec // G101: This is test data, not a real credential
	suite.expectEntity(providers.EntityCategoryUser)
	suite.mockUserMgtProvider.On("UpdateUserCredentials", mock.Anything, userID, mock.MatchedBy(func(data []byte) bool {
		return string(data) == expectedCredentialsJSON
	})).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
}

func (suite *CredentialSetterTestSuite) TestExecute_NoRequiredInputs() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: "password",
		},
		RuntimeData: map[string]string{
			"userID": userID,
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx, mock.Anything, mock.Anything).Return(userID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]providers.Input{})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrCredentialInputMissing.Code, resp.Error.Code)
}

func (suite *CredentialSetterTestSuite) TestExecute_EmptyInputIdentifier() {
	userID := testUserID
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: "password",
		},
		RuntimeData: map[string]string{
			"userID": userID,
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx, mock.Anything, mock.Anything).Return(userID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]providers.Input{
		{
			Identifier: "",
			Type:       providers.InputTypePassword,
			Required:   true,
		},
	})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrCredentialInputInvalid.Code, resp.Error.Code)
}

// nodeContext builds a context that reaches the credential write.
func (suite *CredentialSetterTestSuite) nodeContext() *providers.NodeContext {
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		FlowType:    providers.FlowTypeRecovery,
		UserInputs:  map[string]string{userAttributePassword: "Replaced-Secret-9!"},
		RuntimeData: map[string]string{"userID": testUserID},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx, mock.Anything, mock.Anything).Return(testUserID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]providers.Input{
		{Identifier: userAttributePassword, Type: providers.InputTypePassword, Required: true},
	})

	return ctx
}

func (suite *CredentialSetterTestSuite) TestExecute_SuccessfulWriteRecordsTheProvenSubject() {
	ctx := suite.nodeContext()
	suite.expectEntity(providers.EntityCategoryUser)
	suite.mockUserMgtProvider.On("UpdateUserCredentials", mock.Anything, testUserID, mock.Anything).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testUserID, resp.SharedRuntimeData[common.RuntimeKeyProvenSubject])
}

func (suite *CredentialSetterTestSuite) TestExecute_FailedWriteRecordsNothing() {
	ctx := suite.nodeContext()
	suite.expectEntity(providers.EntityCategoryUser)
	suite.mockUserMgtProvider.On("UpdateUserCredentials", mock.Anything, testUserID, mock.Anything).
		Return(&tidcommon.InternalServerError)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Empty(suite.T(), resp.SharedRuntimeData[common.RuntimeKeyProvenSubject])
}

// heldCredentialWrite runs a credential write against the given decision or fault at the recovery plane.
func (suite *CredentialSetterTestSuite) heldCredentialWrite(decision providers.AccessDecision,
	fault *tidcommon.ServiceError) (*providers.ExecutorResponse, error) {
	suite.executor = newCredentialSetter(suite.mockFlowFactory, suite.mockEntityProvider, suite.mockUserMgtProvider,
		suite.mockAuthnProvider,
		newAccessGuard(expectAccess(suite.T(), testUserID, providers.AccessPlaneRecovery, "", decision, fault)),
		suite.mockEntityTypes)
	ctx := &providers.NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  map[string]string{userAttributePassword: "securePass123!"},
	}
	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx, mock.Anything, mock.Anything).Return(testUserID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]providers.Input{
		{Identifier: userAttributePassword, Type: providers.InputTypePassword, Required: true},
	})
	suite.expectEntity(providers.EntityCategoryUser)
	return suite.executor.Execute(ctx)
}

func (suite *CredentialSetterTestSuite) TestExecute_HeldIdentityIsRefused() {
	for name, test := range map[string]struct {
		decision providers.AccessDecision
		want     string
	}{
		"no hold named": {providers.AccessDecision{}, ErrCredentialSetFailed.Code},
		"suspension": {providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended},
			ErrAccountSuspended.Code},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			resp, err := suite.heldCredentialWrite(test.decision, nil)

			suite.Require().NoError(err)
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Equal(test.want, resp.Error.Code)
			suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "UpdateUserCredentials", mock.Anything, mock.Anything,
				mock.Anything)
		})
	}
}

// A fault is answered as a failed write, never as a hold.
func (suite *CredentialSetterTestSuite) TestExecute_GovernanceFaultFailsTheWrite() {
	resp, err := suite.heldCredentialWrite(providers.AccessDecision{}, &tidcommon.InternalServerError)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrCredentialSetFailed.Code, resp.Error.Code)
	suite.Empty(resp.SharedRuntimeData[common.RuntimeKeyProvenSubject])
	suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "UpdateUserCredentials", mock.Anything, mock.Anything,
		mock.Anything)
}

// expectEntity makes the entity provider return the test entity with the given category.
func (suite *CredentialSetterTestSuite) expectEntity(category providers.EntityCategory) {
	suite.mockEntityProvider.On("GetEntity", mock.Anything, testUserID).
		Return(&providers.Entity{ID: testUserID, Category: category}, nil).Once()
}

// An entity other than a user, such as an agent, is written through the entity provider, and a value its
// schema refuses is asked for again.
func (suite *CredentialSetterTestSuite) TestExecute_NonUserEntityIsWrittenThroughTheEntityProvider() {
	for name, test := range map[string]struct {
		providerErr *entityprovider.EntityProviderError
		wantStatus  providers.ExecutorStatus
	}{
		"written": {wantStatus: providers.ExecComplete},
		"refused": {
			providerErr: entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSchemaValidationFailed,
				"Schema validation failed", ""),
			wantStatus: providers.ExecUserInputRequired,
		},
		"failed": {
			providerErr: entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "db error", ""),
			wantStatus:  providers.ExecFailure,
		},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			ctx := suite.nodeContext()
			suite.expectEntity(providers.EntityCategoryAgent)
			suite.mockEntityProvider.On("UpdateCredentials", mock.Anything, testUserID, mock.Anything).
				Return(test.providerErr).Once()

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(test.wantStatus, resp.Status)
			suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "UpdateUserCredentials", mock.Anything,
				mock.Anything, mock.Anything)
		})
	}
}

func (suite *CredentialSetterTestSuite) TestExecute_EntityLookupFailureFailsTheWrite() {
	ctx := suite.nodeContext()
	suite.mockEntityProvider.On("GetEntity", mock.Anything, testUserID).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "db error", "")).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrCredentialSetFailed.Code, resp.Error.Code)
}

// The reset modes are refused outside an administrator-initiated flow, before any check or write.
func (suite *CredentialSetterTestSuite) TestExecute_ResetModesRunOnlyInAdminInitiatedFlows() {
	for _, mode := range []string{ExecutorModeValidate, ExecutorModeInvalidate} {
		for _, flowType := range []providers.FlowType{providers.FlowTypeRecovery, providers.FlowTypeAuthentication,
			providers.FlowTypeAdministration} {
			suite.Run(mode+" in "+string(flowType), func() {
				suite.SetupTest()
				ctx := suite.resetContext(mode)
				ctx.FlowType = flowType

				resp, err := suite.executor.Execute(ctx)

				suite.Error(err)
				suite.Nil(resp)
			})
		}
	}
}

func (suite *CredentialSetterTestSuite) TestExecute_UnknownModeIsAnError() {
	ctx := suite.resetContext("rotate")

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
}

// Validation checks the reset, records the credential and clears what an earlier round of the same execution
// left behind. It pins no recipient: the flow's email step reads the address from the stored profile. The node
// declares no inputs, so the credential is read from credential. Both administrator inputs are then forgotten.
func (suite *CredentialSetterTestSuite) TestValidate_RecordsTheReset() {
	suite.useGovernance(providers.AccessDecision{Admitted: true}, nil)
	ctx := suite.resetContext(ExecutorModeValidate)
	ctx.UserInputs[revocationInputSubject] = testUserID
	ctx.UserInputs[userInputCredential] = userAttributePassword
	ctx.RuntimeData[userAttributeUserID] = "another-user"
	suite.mockUserMgtProvider.EXPECT().
		ValidateRemoveUserCredential(mock.Anything, testUserID, userAttributePassword).
		Return(nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Empty(ctx.UserInputs)
	suite.Equal(testUserID, resp.RuntimeData[userAttributeUserID], "the reset targets the bound user")
	suite.Equal(userAttributePassword, resp.RuntimeData[common.RuntimeKeyResetCredential])
	suite.NotContains(resp.RuntimeData, common.RuntimeKeyRecipientType)
	suite.NotContains(resp.RuntimeData, common.RuntimeKeyExpectedRecipient)
	for _, cleared := range []string{common.RuntimeKeyStoredInviteToken, common.RuntimeKeyInviteLink} {
		value, ok := resp.RuntimeData[cleared]
		suite.True(ok, "%s must be cleared", cleared)
		suite.Empty(value, "%s must be cleared", cleared)
	}
	delivered, ok := resp.SharedRuntimeData[common.RuntimeKeyDeliveredRecipients]
	suite.True(ok, "the delivered recipients must be cleared")
	suite.Empty(delivered)
	suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "RemoveUserCredential", mock.Anything, mock.Anything,
		mock.Anything)
}

// A credential named after a start input is the credential the start names, not a refused extra input. Both
// inputs are forgotten once checked, so the set step reads neither the administrator's choice nor the user ID as
// the credential's new value.
func (suite *CredentialSetterTestSuite) TestValidate_AcceptsACredentialNamedLikeAStartInput() {
	for _, credential := range []string{userInputCredential, revocationInputSubject} {
		suite.Run(credential, func() {
			suite.SetupTest()
			suite.useGovernance(providers.AccessDecision{Admitted: true}, nil)
			ctx := suite.resetContext(ExecutorModeValidate)
			ctx.UserInputs[revocationInputSubject] = testUserID
			ctx.UserInputs[userInputCredential] = credential
			suite.mockUserMgtProvider.EXPECT().
				ValidateRemoveUserCredential(mock.Anything, testUserID, credential).
				Return(nil).Once()

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(providers.ExecComplete, resp.Status)
			suite.Equal(credential, resp.RuntimeData[common.RuntimeKeyResetCredential])
			suite.Empty(ctx.UserInputs)
		})
	}
}

// Validation asks for the credential through the node's first input, or through credential when the node
// declares none or its first input has no identifier, and checks nothing until it is given.
func (suite *CredentialSetterTestSuite) TestValidate_AsksForTheCredentialToReset() {
	declared := providers.Input{Identifier: "credentialType", Type: providers.InputTypeText, Required: true}
	for name, test := range map[string]struct {
		nodeInputs []providers.Input
		want       providers.Input
	}{
		"no node input": {want: providers.Input{Identifier: userInputCredential, Type: providers.InputTypeText,
			Required: true}},
		"a node input with no identifier": {nodeInputs: []providers.Input{{Type: providers.InputTypeText}},
			want: providers.Input{Identifier: userInputCredential, Type: providers.InputTypeText, Required: true}},
		"a node input": {nodeInputs: []providers.Input{declared}, want: declared},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			ctx := suite.resetContext(ExecutorModeValidate)
			ctx.NodeInputs = test.nodeInputs
			ctx.UserInputs[test.want.Identifier] = "  "

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(providers.ExecUserInputRequired, resp.Status)
			suite.Equal([]providers.Input{test.want}, resp.Inputs)
			suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "ValidateRemoveUserCredential", mock.Anything,
				mock.Anything, mock.Anything)
		})
	}
}

// A refused validation fails with its stated reason and records nothing, so nothing is sent. A request that
// carries any attribute of the user's type is refused, since the flow's sender reads a request input before
// the stored profile.
func (suite *CredentialSetterTestSuite) TestValidate_RefusalsRecordNothing() {
	type step int
	const (
		none step = iota
		checked
		held
	)
	for name, test := range map[string]struct {
		noSubject  bool
		marker     string
		credential string
		checkErr   *tidcommon.ServiceError
		decision   providers.AccessDecision
		fault      *tidcommon.ServiceError
		reached    step
		want       string
	}{
		"no bound user":        {noSubject: true, want: ErrUserIDMissingInContext.Code},
		"a recorded reset":     {marker: "2026-11-12T10:00:00.1Z", want: ErrCredentialSetFailed.Code},
		"a refused validation": {checkErr: &user.ErrorUserNotFound, reached: checked, want: "USR-1003"},
		"a suspended user": {decision: providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended},
			reached: held, want: ErrAccountSuspended.Code},
		"a hold naming no reason": {reached: held, want: ErrCredentialSetFailed.Code},
		"a governance fault": {fault: &tidcommon.InternalServerError, reached: held,
			want: ErrCredentialSetFailed.Code},
		"a credential the user service does not remove": {credential: "numericCode",
			checkErr: &user.ErrorInvalidCredential, reached: checked, want: user.ErrorInvalidCredential.Code},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			if test.reached >= held {
				suite.useGovernance(test.decision, test.fault)
			}
			ctx := suite.resetContext(ExecutorModeValidate)
			if test.noSubject {
				ctx.SetBoundSubject("")
			}
			ctx.RuntimeData[common.RuntimeKeyResetCredentialMarker] = test.marker
			credential := cmp.Or(test.credential, userAttributePassword)
			ctx.UserInputs[userInputCredential] = credential
			if test.reached >= checked {
				suite.mockUserMgtProvider.EXPECT().ValidateRemoveUserCredential(mock.Anything, testUserID,
					credential).Return(test.checkErr).Once()
			}

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Require().NotNil(resp.Error)
			suite.Equal(test.want, resp.Error.Code)
			suite.Empty(resp.RuntimeData[common.RuntimeKeyResetCredential])
			if test.reached < checked {
				suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "ValidateRemoveUserCredential",
					mock.Anything, mock.Anything, mock.Anything)
			}
			suite.Equal(credential, ctx.UserInputs[userInputCredential], "a refused reset keeps the input")
			suite.mockEntityProvider.AssertNotCalled(suite.T(), "GetEntityProfile", mock.Anything, mock.Anything)
		})
	}
}

// A server fault while validating is an error of the flow, not a refusal shown to the administrator.
func (suite *CredentialSetterTestSuite) TestValidate_ServerFaultsAreErrors() {
	suite.Run("the user service", func() {
		suite.SetupTest()
		ctx := suite.resetContext(ExecutorModeValidate)
		ctx.UserInputs[userInputCredential] = userAttributePassword
		suite.mockUserMgtProvider.EXPECT().
			ValidateRemoveUserCredential(mock.Anything, testUserID, userAttributePassword).
			Return(&tidcommon.InternalServerError).Once()

		resp, err := suite.executor.Execute(ctx)

		suite.Error(err)
		suite.Nil(resp)
	})
}

// Invalidation removes the credential the validation recorded, once the link was delivered,
// and records the marker the removal wrote.
func (suite *CredentialSetterTestSuite) TestInvalidate_RecordsTheMarker() {
	ctx := suite.deliveredResetContext()
	suite.mockUserMgtProvider.EXPECT().RemoveUserCredential(mock.Anything, testUserID, userAttributePassword).
		Return("2026-11-12T10:00:00.123456789Z", nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal("2026-11-12T10:00:00.123456789Z", resp.RuntimeData[common.RuntimeKeyResetCredentialMarker])
	_, cleared := resp.RuntimeData[common.RuntimeKeyStoredInviteToken]
	suite.False(cleared, "a completed invalidation keeps the link")
}

// Every failed invalidation clears the stored link token, so a link already sent cannot be verified. A
// failure before any send says nothing was delivered. An input that came after the start is refused, since the
// send step may have read it as the recipient.
func (suite *CredentialSetterTestSuite) TestInvalidate_FailuresVoidTheLink() {
	for name, test := range map[string]struct {
		edit       func(ctx *providers.NodeContext)
		called     bool
		marker     string
		serviceErr *tidcommon.ServiceError
		want       string
	}{
		"no bound user": {edit: func(ctx *providers.NodeContext) { ctx.SetBoundSubject("") },
			want: ErrCredentialInvalidationFailed.Code},
		"no recorded credential": {edit: func(ctx *providers.NodeContext) {
			ctx.RuntimeData[common.RuntimeKeyResetCredential] = ""
		}, want: ErrCredentialInvalidationFailed.Code},
		"an invalidation already recorded": {edit: func(ctx *providers.NodeContext) {
			ctx.RuntimeData[common.RuntimeKeyResetCredentialMarker] = "2026-11-12T10:00:00Z"
		}, want: ErrCredentialInvalidationFailed.Code},
		"no generated link": {edit: func(ctx *providers.NodeContext) {
			ctx.RuntimeData[common.RuntimeKeyStoredInviteToken] = ""
		}, want: ErrCredentialResetNotDelivered.Code},
		"nothing delivered": {edit: func(ctx *providers.NodeContext) {
			delete(ctx.SharedRuntimeData, common.RuntimeKeyDeliveredRecipients)
		}, want: ErrCredentialResetNotDelivered.Code},
		"an empty delivery record": {edit: func(ctx *providers.NodeContext) {
			ctx.SharedRuntimeData[common.RuntimeKeyDeliveredRecipients] = `[]`
		}, want: ErrCredentialResetNotDelivered.Code},
		"an unreadable delivery record": {edit: func(ctx *providers.NodeContext) {
			ctx.SharedRuntimeData[common.RuntimeKeyDeliveredRecipients] = `stored@example.com`
		}, want: ErrCredentialResetNotDelivered.Code},
		"a failed removal": {called: true, serviceErr: &tidcommon.InternalServerError,
			want: ErrCredentialInvalidationFailed.Code},
		"a removal writing no marker": {called: true, want: ErrCredentialInvalidationFailed.Code},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			ctx := suite.deliveredResetContext()
			if test.edit != nil {
				test.edit(ctx)
			}
			if test.called {
				suite.mockUserMgtProvider.EXPECT().
					RemoveUserCredential(mock.Anything, testUserID, userAttributePassword).
					Return(test.marker, test.serviceErr).Once()
			}

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Require().NotNil(resp.Error)
			suite.Equal(test.want, resp.Error.Code)
			token, cleared := resp.RuntimeData[common.RuntimeKeyStoredInviteToken]
			suite.True(cleared, "the link must be voided")
			suite.Empty(token, "the link must be voided")
			suite.Empty(resp.RuntimeData[common.RuntimeKeyResetCredentialMarker])
			if !test.called {
				suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "RemoveUserCredential", mock.Anything,
					mock.Anything, mock.Anything)
			}
		})
	}
}

// The user's set asks for the credential only while the marker the invalidation recorded is still the user's
// credential marker and the user is not held, so a voided or held reset is refused before anything is shown.
func (suite *CredentialSetterTestSuite) TestSetInAdminInitiatedFlow_RefusesAVoidedOrHeldResetBeforeAsking() {
	const recorded = "2026-11-12T10:00:00.123456789Z"
	for name, test := range map[string]struct {
		edit     func(ctx *providers.NodeContext)
		current  bool
		checkErr *tidcommon.ServiceError
		looked   bool
		decision *providers.AccessDecision
		want     string
	}{
		"current":        {current: true, looked: true, decision: &providers.AccessDecision{Admitted: true}},
		"a later change": {looked: true, want: ErrInvalidInviteToken.Code},
		"a refused check": {checkErr: &tidcommon.ServiceError{Type: tidcommon.ClientErrorType, Code: "UMP-1003"},
			looked: true, want: ErrInvalidInviteToken.Code},
		"no recorded invalidation": {edit: func(ctx *providers.NodeContext) {
			ctx.RuntimeData[common.RuntimeKeyResetCredentialMarker] = ""
		}, want: ErrInvalidInviteToken.Code},
		"no recorded credential": {edit: func(ctx *providers.NodeContext) {
			ctx.RuntimeData[common.RuntimeKeyResetCredential] = ""
		}, want: ErrInvalidInviteToken.Code},
		"no bound user": {edit: func(ctx *providers.NodeContext) { ctx.SetBoundSubject("") },
			want: ErrInvalidInviteToken.Code},
		"a suspended user": {current: true, looked: true,
			decision: &providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended},
			want:     ErrAccountSuspended.Code},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			if test.decision != nil {
				suite.useGovernance(*test.decision, nil)
			}
			ctx := suite.invalidatedResetContext(ExecutorModeSet, recorded)
			if test.edit != nil {
				test.edit(ctx)
			}
			if test.looked {
				suite.expectResetCurrent(recorded, test.current, test.checkErr)
			}
			if test.want == "" {
				suite.expectAttributes()
			}

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			if test.want == "" {
				suite.Equal(providers.ExecUserInputRequired, resp.Status)
				suite.Equal([]providers.Input{{Identifier: userAttributePassword, Type: providers.InputTypePassword,
					Required: true, DisplayName: userAttributePassword}}, resp.Inputs)
				suite.Equal(resp.Inputs, resp.ForwardedData[common.ForwardedDataKeyInputs],
					"the prompt the set step continues to renders the input it asks for")
				return
			}
			suite.Equal(providers.ExecFailure, resp.Status)
			suite.Require().NotNil(resp.Error)
			suite.Equal(test.want, resp.Error.Code)
			suite.Empty(resp.Inputs, "a refused reset asks for nothing")
			suite.mockEntityProvider.AssertNotCalled(suite.T(), "GetEntityProfile", mock.Anything, mock.Anything)
			if !test.looked {
				suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "IsUserCredentialUnchangedSince", mock.Anything,
					mock.Anything, mock.Anything)
			}
		})
	}
}

func (suite *CredentialSetterTestSuite) TestSetInAdminInitiatedFlow_AFailedCheckIsAnError() {
	ctx := suite.invalidatedResetContext(ExecutorModeSet, "2026-11-12T10:00:00Z")
	suite.expectResetCurrent("2026-11-12T10:00:00Z", false, &tidcommon.InternalServerError)

	resp, err := suite.executor.Execute(ctx)

	suite.Error(err)
	suite.Nil(resp)
}

// A server fault reading the reset credential's input is an error of the flow, not a refusal shown to the user.
func (suite *CredentialSetterTestSuite) TestSetInAdminInitiatedFlow_ServerFaultsAreErrors() {
	const recorded = "2026-11-12T10:00:00Z"
	suite.Run("the profile read", func() {
		suite.SetupTest()
		ctx := suite.invalidatedResetContext(ExecutorModeSet, recorded)
		suite.expectResetCurrent(recorded, true, nil)
		suite.mockEntityProvider.On("GetEntityProfile", mock.Anything, testUserID).Return(nil,
			entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "db error", "")).Once()

		resp, err := suite.executor.Execute(ctx)

		suite.Error(err)
		suite.Nil(resp)
	})
	suite.Run("the schema read", func() {
		suite.SetupTest()
		ctx := suite.invalidatedResetContext(ExecutorModeSet, recorded)
		suite.expectResetCurrent(recorded, true, nil)
		suite.expectUserType()
		suite.mockEntityTypes.EXPECT().GetAttributes(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(nil, &tidcommon.InternalServerError).Once()

		resp, err := suite.executor.Execute(ctx)

		suite.Error(err)
		suite.Nil(resp)
	})
}

// A reset cannot read the user's credentials without the entity type service.
func (suite *CredentialSetterTestSuite) TestResetWithoutTheEntityTypeServiceIsAnError() {
	suite.executor.entityTypeService = nil

	_, attributes, err := suite.executor.entityAttributes(suite.resetContext(ExecutorModeSet), testUserID)

	suite.EqualError(err, "entity type service is not available")
	suite.Nil(attributes)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "GetEntityProfile", mock.Anything, mock.Anything)
}

// The set step first runs in the request that opened the link. It asks for the credential there and records
// that it did, and writes nothing, even when the execution already holds a value of that name.
func (suite *CredentialSetterTestSuite) TestSetInAdminInitiatedFlow_AsksBeforeReadingAValue() {
	const recorded = "2026-11-12T10:00:00Z"
	ctx := suite.invalidatedResetContext(ExecutorModeSet, recorded)
	ctx.UserInputs = map[string]string{userAttributePassword: "Sent-Earlier-1!"}
	suite.expectResetCurrent(recorded, true, nil)
	suite.expectAttributes()

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecUserInputRequired, resp.Status)
	suite.Require().Len(resp.Inputs, 1)
	suite.Equal(userAttributePassword, resp.Inputs[0].Identifier)
	suite.Equal(dataValueTrue, resp.RuntimeData[common.RuntimeKeyResetCredentialRequested])
	suite.NotContains(ctx.UserInputs, userAttributePassword, "the earlier value must be dropped")
	suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "UpdateUserCredentials", mock.Anything, mock.Anything,
		mock.Anything)
}

// The user's set writes only the credential the reset invalidated, for the bound user, from the input of that
// name, and records proof of control.
func (suite *CredentialSetterTestSuite) TestSetInAdminInitiatedFlow_WritesOnlyTheResetCredential() {
	const recorded = "2026-11-12T10:00:00Z"
	ctx := suite.requestedResetContext("", recorded)
	ctx.RuntimeData[userAttributeUserID] = "another-user"
	ctx.UserInputs = map[string]string{userAttributePassword: "New-Secret-1!", "pin": "1234", "subject": "another-user"}
	suite.expectResetCurrent(recorded, true, nil)
	suite.expectAttributes()
	suite.mockUserMgtProvider.EXPECT().UpdateUserCredentials(mock.Anything, testUserID,
		json.RawMessage(`{"password":"New-Secret-1!"}`)).Return(nil).Once()

	resp, err := suite.executor.Execute(ctx)

	suite.Require().NoError(err)
	suite.Equal(providers.ExecComplete, resp.Status)
	suite.Equal(testUserID, resp.SharedRuntimeData[common.RuntimeKeyProvenSubject])
	suite.mockBaseExecutor.AssertNotCalled(suite.T(), "GetUserIDFromContext", mock.Anything, mock.Anything,
		mock.Anything)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "GetEntity", mock.Anything, mock.Anything)
}

func (suite *CredentialSetterTestSuite) TestSetInAdminInitiatedFlow_OutcomesBeforeAndAfterTheWrite() {
	const recorded = "2026-11-12T10:00:00Z"
	pinPrompt := []providers.Input{{Identifier: "pin", Type: providers.InputTypePassword, Required: true,
		DisplayName: "PIN"}}
	for name, test := range map[string]struct {
		voided     bool
		credential string
		value      string
		decision   *providers.AccessDecision
		writeErr   *tidcommon.ServiceError
		wrote      bool
		wantStatus providers.ExecutorStatus
		wantCode   string
		wantInputs []providers.Input
	}{
		"no value yet": {wantStatus: providers.ExecUserInputRequired, wantInputs: pinPrompt},
		"a voided reset": {voided: true, value: "1234",
			wantStatus: providers.ExecFailure, wantCode: ErrInvalidInviteToken.Code},
		"a suspended user": {value: "1234",
			decision:   &providers.AccessDecision{Reason: providers.AccessRefusalReasonSuspended},
			wantStatus: providers.ExecFailure, wantCode: ErrAccountSuspended.Code},
		"a value the schema refuses": {value: "12", writeErr: &user.ErrorInvalidCredentialValue,
			wrote: true, wantStatus: providers.ExecUserInputRequired, wantCode: ErrCredentialValueInvalid.Code,
			wantInputs: pinPrompt},
		"a failed write": {value: "1234", writeErr: &tidcommon.InternalServerError, wrote: true,
			wantStatus: providers.ExecFailure, wantCode: ErrCredentialSetFailed.Code},
		"a credential with no masked input": {credential: "recoveryAnswers", value: "x",
			wantStatus: providers.ExecFailure, wantCode: ErrCredentialSetFailed.Code},
		"a number credential": {credential: "numericCode", value: "1234",
			wantStatus: providers.ExecFailure, wantCode: ErrCredentialSetFailed.Code},
		"a PIN with leading zeros": {value: "001234", wrote: true,
			wantStatus: providers.ExecComplete},
	} {
		suite.Run(name, func() {
			suite.SetupTest()
			if test.decision != nil {
				suite.useGovernance(*test.decision, nil)
			}
			ctx := suite.requestedResetContext(ExecutorModeSet, recorded)
			credential := cmp.Or(test.credential, "pin")
			ctx.RuntimeData[common.RuntimeKeyResetCredential] = credential
			ctx.UserInputs = map[string]string{credential: test.value}
			suite.expectResetCurrent(recorded, !test.voided, nil)
			if test.wantCode != ErrInvalidInviteToken.Code {
				suite.expectAttributes()
			}
			if test.wrote {
				suite.mockUserMgtProvider.EXPECT().UpdateUserCredentials(mock.Anything, testUserID,
					json.RawMessage(`{"pin":"`+test.value+`"}`)).Return(test.writeErr).Once()
			}

			resp, err := suite.executor.Execute(ctx)

			suite.Require().NoError(err)
			suite.Equal(test.wantStatus, resp.Status)
			if test.wantCode != "" {
				suite.Require().NotNil(resp.Error)
				suite.Equal(test.wantCode, resp.Error.Code)
			}
			suite.Equal(test.wantInputs, resp.Inputs)
			if test.wantInputs != nil {
				suite.Equal(test.wantInputs, resp.ForwardedData[common.ForwardedDataKeyInputs])
			}
			if test.wantStatus == providers.ExecComplete {
				suite.Equal(testUserID, resp.SharedRuntimeData[common.RuntimeKeyProvenSubject])
				return
			}
			suite.Empty(resp.SharedRuntimeData[common.RuntimeKeyProvenSubject])
			if !test.wrote {
				suite.mockUserMgtProvider.AssertNotCalled(suite.T(), "UpdateUserCredentials", mock.Anything,
					mock.Anything, mock.Anything)
			}
		})
	}
}

func TestCredentialSetterSuite(t *testing.T) {
	suite.Run(t, new(CredentialSetterTestSuite))
}

// useGovernance makes the executor check holds at the recovery plane against the decision or fault given.
func (suite *CredentialSetterTestSuite) useGovernance(decision providers.AccessDecision,
	fault *tidcommon.ServiceError) {
	suite.executor = newCredentialSetter(suite.mockFlowFactory, suite.mockEntityProvider, suite.mockUserMgtProvider,
		suite.mockAuthnProvider,
		newAccessGuard(expectAccess(suite.T(), testUserID, providers.AccessPlaneRecovery, "", decision, fault)),
		suite.mockEntityTypes)
}

// resetContext builds an administrator-initiated node context bound to the test user, in the mode given.
func (suite *CredentialSetterTestSuite) resetContext(mode string) *providers.NodeContext {
	ctx := &providers.NodeContext{
		Context:           context.Background(),
		ExecutionID:       "test-flow",
		FlowType:          providers.FlowTypeAdminInitiated,
		ExecutorMode:      mode,
		UserInputs:        map[string]string{},
		RuntimeData:       map[string]string{},
		SharedRuntimeData: map[string]string{},
	}
	ctx.SetBoundSubject(testUserID)
	return ctx
}

// deliveredResetContext builds an invalidation context whose link was generated and sent.
func (suite *CredentialSetterTestSuite) deliveredResetContext() *providers.NodeContext {
	ctx := suite.resetContext(ExecutorModeInvalidate)
	ctx.RuntimeData[common.RuntimeKeyResetCredential] = userAttributePassword
	ctx.RuntimeData[common.RuntimeKeyStoredInviteToken] = "stored-token"
	ctx.SharedRuntimeData[common.RuntimeKeyDeliveredRecipients] = `["stored@example.com"]`
	return ctx
}

// invalidatedResetContext builds a context of the user's stage after the invalidation recorded marker.
func (suite *CredentialSetterTestSuite) invalidatedResetContext(mode, marker string) *providers.NodeContext {
	ctx := suite.resetContext(mode)
	ctx.RuntimeData[common.RuntimeKeyResetCredential] = userAttributePassword
	ctx.RuntimeData[common.RuntimeKeyResetCredentialMarker] = marker
	return ctx
}

// requestedResetContext builds a context of a request after the set step asked the user for the credential.
func (suite *CredentialSetterTestSuite) requestedResetContext(mode, marker string) *providers.NodeContext {
	ctx := suite.invalidatedResetContext(mode, marker)
	ctx.RuntimeData[common.RuntimeKeyResetCredentialRequested] = dataValueTrue
	return ctx
}

// expectUserType makes the entity provider return the test user's profile, of the employee type.
func (suite *CredentialSetterTestSuite) expectUserType() {
	suite.mockEntityProvider.On("GetEntityProfile", mock.Anything, testUserID).Return(&providers.Entity{
		ID: testUserID, Category: providers.EntityCategoryUser, Type: "employee",
	}, nil).Once()
}

// expectAttributes makes the test user's type declare a password, a PIN, a number credential, a credential
// with no masked input, and two email attributes.
func (suite *CredentialSetterTestSuite) expectAttributes() {
	suite.expectUserType()
	suite.mockEntityTypes.EXPECT().GetAttributes(mock.Anything, entitytype.TypeCategoryUser, "employee",
		entitytype.AttributeFilter{AllowCredential: true, AllowNonCredential: true}).
		Return([]entitytype.AttributeInfo{
			{Attribute: userAttributePassword, Type: entitytypemodel.TypeString, Credential: true},
			{Attribute: "pin", DisplayName: "PIN", Type: entitytypemodel.TypeString, Credential: true},
			{Attribute: userInputCredential, Type: entitytypemodel.TypeString, Credential: true},
			{Attribute: "numericCode", Type: entitytypemodel.TypeNumber, Credential: true},
			{Attribute: "recoveryAnswers", Type: entitytypemodel.TypeObject, Credential: true},
			{Attribute: "email", Type: entitytypemodel.TypeEmail},
			{Attribute: "workEmail", Type: entitytypemodel.TypeEmail},
		}, nil).Once()
}

// expectResetCurrent makes the user management provider answer whether the test user's credential is unchanged
// since the removal that returned marker.
func (suite *CredentialSetterTestSuite) expectResetCurrent(marker string, current bool,
	svcErr *tidcommon.ServiceError) {
	suite.mockUserMgtProvider.EXPECT().IsUserCredentialUnchangedSince(mock.Anything, testUserID, marker).
		Return(current, svcErr).Once()
}

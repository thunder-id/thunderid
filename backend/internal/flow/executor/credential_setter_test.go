// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/identitygovernanceprovidermock"
)

type CredentialSetterTestSuite struct {
	suite.Suite
	mockFlowFactory    *coremock.FlowFactoryInterfaceMock
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	mockAuthnProvider  *managermock.AuthnProviderManagerMock
	mockBaseExecutor   *coremock.ExecutorInterfaceMock
	executor           *credentialSetter
}

func (suite *CredentialSetterTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
	suite.mockBaseExecutor = coremock.NewExecutorInterfaceMock(suite.T())

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

	suite.executor = newCredentialSetter(suite.mockFlowFactory, suite.mockEntityProvider, suite.mockAuthnProvider, nil)
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
	suite.mockEntityProvider.On("UpdateCredentials", userID, mock.Anything).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
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

	suite.mockEntityProvider.On("UpdateCredentials", userID, mock.Anything).
		Return(entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "db error", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Equal(suite.T(), ErrCredentialSetFailed.Code, resp.Error.Code)
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
	suite.mockEntityProvider.On("UpdateCredentials", userID, mock.MatchedBy(func(data []byte) bool {
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

func TestCredentialSetterSuite(t *testing.T) {
	suite.Run(t, new(CredentialSetterTestSuite))
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

// A completed write records whose control was proven, on the executor-only cross-frame store. The
// value is the identity, not a flag: a later node reads who to act on from here rather than from an
// input, so a flow cannot prove one account and act on another.
func (suite *CredentialSetterTestSuite) TestASuccessfulWriteRecordsTheProvenSubject() {
	ctx := suite.nodeContext()
	suite.mockEntityProvider.On("UpdateCredentials", testUserID, mock.Anything).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testUserID, resp.SharedRuntimeData[common.RuntimeKeyProvenSubject])
}

// A failed write proves nothing, so it records nothing. Without this a node downstream would treat a
// write that did not happen as control that was proven.
func (suite *CredentialSetterTestSuite) TestAFailedWriteRecordsNothing() {
	ctx := suite.nodeContext()
	suite.mockEntityProvider.On("UpdateCredentials", testUserID, mock.Anything).
		Return(&entityprovider.EntityProviderError{Code: entityprovider.ErrorCodeSchemaValidationFailed})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Empty(suite.T(), resp.SharedRuntimeData[common.RuntimeKeyProvenSubject])
}

// heldCredentialWrite runs a credential write for a held identity against the given decision.
func (suite *CredentialSetterTestSuite) heldCredentialWrite(
	decision providers.AccessDecision) (*providers.ExecutorResponse, error) {
	governance := identitygovernanceprovidermock.NewIdentityGovernanceProviderMock(suite.T())
	governance.On("EvaluateAccess", mock.Anything, testUserID, providers.AccessPlaneRecovery,
		providers.AccessScope("")).
		Return(decision, (*tidcommon.ServiceError)(nil)).Once()
	suite.executor = newCredentialSetter(suite.mockFlowFactory, suite.mockEntityProvider, suite.mockAuthnProvider,
		governance)
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
	return suite.executor.Execute(ctx)
}

// A held identity gets no new credential, and by default the refusal is the setter's own.
func (suite *CredentialSetterTestSuite) TestExecute_HeldIdentityIsRefused() {
	resp, err := suite.heldCredentialWrite(providers.AccessDecision{})

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrCredentialSetFailed.Code, resp.Error.Code)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "UpdateCredentials", mock.Anything, mock.Anything)
}

// When the policy permits it, the refusal states the hold.
func (suite *CredentialSetterTestSuite) TestExecute_StatedHoldIsReturned() {
	resp, err := suite.heldCredentialWrite(providers.AccessDecision{
		Reason: providers.AccessRefusalReasonSuspended,
	})

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrAccountSuspended.Code, resp.Error.Code)
}

// An unreadable authority fails the write as a failed write, never as a hold, and writes nothing.
func (suite *CredentialSetterTestSuite) TestExecute_GovernanceFaultFailsTheWrite() {
	suite.executor = newCredentialSetter(suite.mockFlowFactory, suite.mockEntityProvider, suite.mockAuthnProvider,
		decideAccess(suite.T(), providers.AccessDecision{}, &tidcommon.InternalServerError))

	resp, err := suite.executor.Execute(suite.nodeContext())

	suite.Require().NoError(err)
	suite.Equal(providers.ExecFailure, resp.Status)
	suite.Equal(ErrCredentialSetFailed.Code, resp.Error.Code)
	suite.Empty(resp.SharedRuntimeData[common.RuntimeKeyProvenSubject])
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "UpdateCredentials", mock.Anything, mock.Anything)
}

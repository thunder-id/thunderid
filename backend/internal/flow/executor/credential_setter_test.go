// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	"testing"

	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
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

	suite.executor = newCredentialSetter(suite.mockFlowFactory, suite.mockEntityProvider, suite.mockAuthnProvider)
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
	suite.mockEntityProvider.On("UpdateCredentials", mock.Anything, userID, mock.Anything).Return(nil)

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

	suite.mockEntityProvider.On("UpdateCredentials", mock.Anything, userID, mock.Anything).
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
	suite.mockEntityProvider.On("UpdateCredentials", mock.Anything, userID, mock.MatchedBy(func(data []byte) bool {
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

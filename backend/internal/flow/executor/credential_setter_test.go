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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

type CredentialSetterTestSuite struct {
	suite.Suite
	mockFlowFactory    *coremock.FlowFactoryInterfaceMock
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	mockBaseExecutor   *coremock.ExecutorInterfaceMock
	executor           *credentialSetter
}

func (suite *CredentialSetterTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockBaseExecutor = coremock.NewExecutorInterfaceMock(suite.T())

	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameCredentialSetter,
		common.ExecutorTypeRegistration,
		mock.Anything,
		[]common.Input{
			{
				Identifier: userAttributeUserID,
				Type:       common.InputTypeText,
				Required:   true,
			},
		}).Return(suite.mockBaseExecutor)

	suite.executor = newCredentialSetter(suite.mockFlowFactory, suite.mockEntityProvider)
}

func (suite *CredentialSetterTestSuite) TestExecute_Success() {
	userID := testUserID
	password := "securePass123!"
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: password,
		},
		RuntimeData: map[string]string{
			"userID": userID,
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx).Return(userID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]common.Input{
		{
			Identifier: userAttributePassword,
			Type:       common.InputTypePassword,
			Required:   true,
		},
	})

	// Use mock.Anything for credentials JSON bytes to avoid strict byte checking
	suite.mockEntityProvider.On("UpdateCredentials", userID, mock.Anything).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *CredentialSetterTestSuite) TestExecute_MissingInput() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs:  make(map[string]string),
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(false)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
}

func (suite *CredentialSetterTestSuite) TestExecute_MissingUserID() {
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: "password",
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx).Return("")

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "User ID not found in flow context", resp.FailureReason)
}

func (suite *CredentialSetterTestSuite) TestExecute_EmptyPassword() {
	userID := testUserID
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: "",
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx).Return(userID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]common.Input{
		{
			Identifier: userAttributePassword,
			Type:       common.InputTypePassword,
			Required:   true,
		},
	})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Credential value cannot be empty", resp.FailureReason)
}

func (suite *CredentialSetterTestSuite) TestExecute_ServiceError() {
	userID := testUserID
	password := "password"
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: password,
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx).Return(userID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]common.Input{
		{
			Identifier: userAttributePassword,
			Type:       common.InputTypePassword,
			Required:   true,
		},
	})

	suite.mockEntityProvider.On("UpdateCredentials", userID, mock.Anything).
		Return(entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "db error", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), "Failed to set credentials", resp.FailureReason)
}

func (suite *CredentialSetterTestSuite) TestExecute_CustomAttribute() {
	userID := testUserID
	const customAttr = "pin"
	pinValue := "1234"

	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			customAttr: pinValue,
		},
		RuntimeData: map[string]string{
			"userID": userID,
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", mock.Anything, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", mock.Anything).Return(userID)

	suite.mockBaseExecutor.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{
			Identifier: customAttr,
			Required:   true,
			Type:       common.InputTypeText,
		},
	})

	// Expect UpdateUserCredentials with custom attribute
	expectedCredentialsJSON := `{"pin":"1234"}` //nolint:gosec // G101: This is test data, not a real credential
	suite.mockEntityProvider.On("UpdateCredentials", userID, mock.MatchedBy(func(data []byte) bool {
		return string(data) == expectedCredentialsJSON
	})).Return(nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
}

func (suite *CredentialSetterTestSuite) TestExecute_NoRequiredInputs() {
	userID := testUserID
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: "password",
		},
		RuntimeData: map[string]string{
			"userID": userID,
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx).Return(userID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]common.Input{})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "No credential input configured")
}

func (suite *CredentialSetterTestSuite) TestExecute_EmptyInputIdentifier() {
	userID := testUserID
	ctx := &core.NodeContext{
		ExecutionID: "test-flow",
		UserInputs: map[string]string{
			userAttributePassword: "password",
		},
		RuntimeData: map[string]string{
			"userID": userID,
		},
	}

	suite.mockBaseExecutor.On("HasRequiredInputs", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("ValidatePrerequisites", ctx, mock.Anything).Return(true)
	suite.mockBaseExecutor.On("GetUserIDFromContext", ctx).Return(userID)
	suite.mockBaseExecutor.On("GetRequiredInputs", ctx).Return([]common.Input{
		{
			Identifier: "",
			Type:       common.InputTypePassword,
			Required:   true,
		},
	})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.FailureReason, "Invalid credential input configuration")
}

func TestCredentialSetterSuite(t *testing.T) {
	suite.Run(t, new(CredentialSetterTestSuite))
}

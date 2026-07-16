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
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const (
	testUniquenessUserType = "INTERNAL"
	testExistingUserID     = "user-existing"
)

type AttributeUniquenessValidatorTestSuite struct {
	suite.Suite
	mockFlowFactory       *coremock.FlowFactoryInterfaceMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	mockEntityProvider    *entityprovidermock.EntityProviderInterfaceMock
	mockAuthnProvider     *managermock.AuthnProviderManagerMock
	mockBaseExecutor      *coremock.ExecutorInterfaceMock
	executor              *attributeUniquenessValidator
}

func (suite *AttributeUniquenessValidatorTestSuite) SetupTest() {
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())

	suite.mockBaseExecutor = coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockBaseExecutor.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).
		Return(func(
			ctx *providers.NodeContext,
			execResp *providers.ExecutorResponse,
			_ providers.AuthnProviderManager,
		) bool {
			if _, ok := ctx.RuntimeData[userTypeKey]; !ok {
				execResp.Status = providers.ExecFailure
				execResp.Error = &tidcommon.ServiceError{
					Error: tidcommon.I18nMessage{DefaultValue: "Prerequisite not met: " + userTypeKey},
				}
				return false
			}
			return true
		}).Maybe()

	prerequisites := []providers.Input{{Identifier: userTypeKey, Required: true}}
	suite.mockFlowFactory.On("CreateExecutor",
		ExecutorNameAttributeUniquenessValidator,
		providers.ExecutorTypeUtility,
		[]providers.Input{},
		prerequisites, mock.Anything).Return(suite.mockBaseExecutor)

	suite.executor = newAttributeUniquenessValidator(
		suite.mockFlowFactory, suite.mockEntityTypeService, suite.mockEntityProvider, suite.mockAuthnProvider)
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_NoUserType_SkipsCheck() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"email": "test@example.com"},
		RuntimeData: map[string]string{},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	suite.mockEntityTypeService.AssertNotCalled(suite.T(), "GetUniqueAttributes")
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_NoConflict_ReturnsComplete() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"email": "free@example.com", "username": "newuser"},
		RuntimeData: map[string]string{
			userTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{"email", "username"}, nil)

	freeID := (*string)(nil)
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{"email": "free@example.com"}).
		Return(freeID, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "not found", ""))
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{"username": "newuser"}).
		Return(freeID, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "not found", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_AttributeConflict_ReturnsUserInputRequired() {
	tests := []struct {
		name      string
		attribute string
		value     string
	}{
		{name: "email conflict", attribute: "email", value: "taken@example.com"},
		{name: "username conflict", attribute: "username", value: "takenuser"},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			ctx := &providers.NodeContext{
				ExecutionID: "flow-1",
				UserInputs:  map[string]string{tt.attribute: tt.value},
				RuntimeData: map[string]string{
					userTypeKey: testUniquenessUserType,
				},
			}

			suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
				Return([]string{tt.attribute}, nil).Once()

			existingUserID := testExistingUserID
			suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{tt.attribute: tt.value}).
				Return(&existingUserID, nil).Once()

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
			assert.Equal(suite.T(), tt.attribute, resp.Error.ErrorDescription.Params["attribute"])
			assert.Equal(suite.T(), tt.attribute, resp.Error.Error.Params["attribute"])
			assert.Contains(suite.T(), resp.Error.ErrorDescription.String(), "already associated")
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_UniqueAttrNotInInputs_Skipped() {
	// Schema says "email" is unique, but the user hasn't provided it yet — skip the check.
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{
			userTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{"email", "username"}, nil)

	freeID := (*string)(nil)
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{"username": "newuser"}).
		Return(freeID, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "not found", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	// email was NOT in UserInputs so IdentifyUser must not be called for it
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity",
		map[string]interface{}{"email": ""})
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_SchemaServiceError_ReturnsFailure() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"email": "test@example.com"},
		RuntimeData: map[string]string{
			userTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string(nil), &tidcommon.ServiceError{
			Code:  "schema_not_found",
			Error: tidcommon.I18nMessage{Key: "error.test.schema_not_found", DefaultValue: "schema not found"},
		})

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity")
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_IdentifyUserSystemError_ReturnsFailure() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"email": "test@example.com"},
		RuntimeData: map[string]string{
			userTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{"email"}, nil)

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{"email": "test@example.com"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "db error", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_NoUniqueAttributesInSchema_ReturnsComplete() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"given_name": "John"},
		RuntimeData: map[string]string{
			userTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity")
}

func TestAttributeUniquenessValidatorSuite(t *testing.T) {
	suite.Run(t, new(AttributeUniquenessValidatorTestSuite))
}

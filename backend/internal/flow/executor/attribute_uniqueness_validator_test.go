// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package executor

import (
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	authnprovidercm "github.com/thunder-id/thunderid/internal/authnprovider/common"
	"github.com/thunder-id/thunderid/internal/entityprovider"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entityprovidermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
)

const (
	testUniquenessUserType  = "INTERNAL"
	testUniquenessAgentType = "AGENT"
	testExistingUserID      = "user-existing"
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

	suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]entitytype.AttributeInfo{}, (*tidcommon.ServiceError)(nil)).Maybe()

	suite.mockBaseExecutor = coremock.NewExecutorInterfaceMock(suite.T())
	suite.mockBaseExecutor.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).
		Return(func(
			ctx *providers.NodeContext,
			execResp *providers.ExecutorResponse,
			_ providers.AuthnProviderManager,
		) bool {
			for _, key := range []string{categoryNameKey, categoryTypeKey} {
				if _, ok := ctx.RuntimeData[key]; !ok {
					execResp.Status = providers.ExecFailure
					execResp.Error = &tidcommon.ServiceError{
						Error: tidcommon.I18nMessage{DefaultValue: "Prerequisite not met: " + key},
					}
					return false
				}
			}
			return true
		}).Maybe()

	prerequisites := []providers.Input{
		{Identifier: categoryNameKey, Required: true},
		{Identifier: categoryTypeKey, Required: true},
	}
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
			categoryNameKey: string(entitytype.TypeCategoryUser),
			categoryTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{"email", "username"}, nil)

	freeID := (*string)(nil)
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything,
		map[string]interface{}{"email": "free@example.com"}).
		Return(freeID, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "not found", ""))
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything,
		map[string]interface{}{"username": "newuser"}).
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
					categoryNameKey: string(entitytype.TypeCategoryUser),
					categoryTypeKey: testUniquenessUserType,
				},
			}

			suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
				Return([]string{tt.attribute}, nil).Once()

			existingUserID := testExistingUserID
			suite.mockEntityProvider.On("IdentifyEntity", mock.Anything,
				map[string]interface{}{tt.attribute: tt.value}).
				Return(&existingUserID, nil).Once()

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
			assert.Equal(suite.T(), tt.attribute, resp.Error.ErrorDescription.Params["attribute"])
			assert.Equal(suite.T(), tt.attribute, resp.Error.Error.Params["attribute"])
			assert.Equal(suite.T(), string(entitytype.TypeCategoryUser), resp.Error.Error.Params["entity"])
			assert.Equal(suite.T(), string(entitytype.TypeCategoryUser),
				resp.Error.ErrorDescription.Params["entity"])
			assert.Contains(suite.T(), resp.Error.ErrorDescription.String(), "already associated")
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

// A conflicting value is cleared and asked for again as a required input.
func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_AttributeConflict_AsksForTheAttributeAgain() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"email": "taken@example.com", "nickname": "kept"},
		RuntimeData: map[string]string{
			categoryNameKey: string(entitytype.TypeCategoryUser),
			categoryTypeKey: testUniquenessUserType,
			"email":         "taken@example.com",
		},
	}
	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{"email"}, nil).Once()
	existingUserID := testExistingUserID
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything,
		map[string]interface{}{"email": "taken@example.com"}).Return(&existingUserID, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.NotContains(suite.T(), ctx.UserInputs, "email", "the conflicting value is dropped from the inputs")
	assert.Equal(suite.T(), "kept", ctx.UserInputs["nickname"], "the other values are kept")
	assert.Equal(suite.T(), "", resp.RuntimeData["email"], "the conflicting value is cleared from the runtime data")
	assert.Equal(suite.T(), []providers.Input{{Identifier: "email", Type: providers.InputTypeText, Required: true}},
		resp.Inputs)
	assert.Equal(suite.T(), resp.Inputs, resp.ForwardedData[common.ForwardedDataKeyInputs])
}

// The re-asked input takes its display name, type and choices from the schema attribute when the schema has it.
func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_AttributeConflict_ReaskUsesTheSchemaAttribute() {
	cases := map[string]struct {
		info entitytype.AttributeInfo
		want providers.Input
	}{
		"display name": {
			info: entitytype.AttributeInfo{Attribute: "email", DisplayName: "Email address", Type: "string"},
			want: providers.Input{Identifier: "email", DisplayName: "Email address", Type: providers.InputTypeText,
				Required: true},
		},
		"number": {
			info: entitytype.AttributeInfo{Attribute: "email", DisplayName: "Badge", Type: "number"},
			want: providers.Input{Identifier: "email", DisplayName: "Badge", Type: providers.InputTypeNumber,
				Required: true},
		},
		"enum": {
			info: entitytype.AttributeInfo{Attribute: "email", DisplayName: "Plan", Type: "string",
				Enum: []string{"a", "b"}},
			want: providers.Input{Identifier: "email", DisplayName: "Plan", Type: providers.InputTypeSelect,
				Options: []string{"a", "b"}, Ref: "email", Required: true},
		},
	}
	for name, tc := range cases {
		suite.Run(name, func() {
			suite.SetupTest()
			suite.mockEntityTypeService.ExpectedCalls = nil
			ctx := &providers.NodeContext{
				ExecutionID: "flow-1",
				UserInputs:  map[string]string{"email": "taken@example.com"},
				RuntimeData: map[string]string{categoryNameKey: string(entitytype.TypeCategoryUser),
					categoryTypeKey: testUniquenessUserType},
			}
			suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
				Return([]string{"email"}, nil).Once()
			suite.mockEntityTypeService.On("GetAttributes", mock.Anything, mock.Anything, testUniquenessUserType,
				mock.Anything).Return([]entitytype.AttributeInfo{tc.info}, (*tidcommon.ServiceError)(nil)).Once()
			existing := testExistingUserID
			suite.mockEntityProvider.On("IdentifyEntity", mock.Anything,
				map[string]interface{}{"email": "taken@example.com"}).Return(&existing, nil).Once()

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), []providers.Input{tc.want}, resp.Inputs)
		})
	}
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_AttributeConflictForAgent_NamesTheAgent() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"model": "gpt-5"},
		RuntimeData: map[string]string{
			categoryNameKey: string(entitytype.TypeCategoryAgent),
			categoryTypeKey: testUniquenessAgentType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, entitytype.TypeCategoryAgent,
		testUniquenessAgentType).Return([]string{"model"}, nil).Once()

	existingID := testExistingUserID
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything,
		map[string]interface{}{"model": "gpt-5"}).
		Return(&existingID, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), string(entitytype.TypeCategoryAgent), resp.Error.Error.Params["entity"])
	assert.NotContains(suite.T(), resp.Error.Error.String(), "user")
	assert.NotContains(suite.T(), resp.Error.ErrorDescription.String(), "user")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_CategoryFromRuntime_SelectsTheSchema() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"name": "Foo"},
		RuntimeData: map[string]string{
			categoryTypeKey: testUniquenessAgentType,
			categoryNameKey: string(entitytype.TypeCategoryAgent),
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, entitytype.TypeCategoryAgent,
		testUniquenessAgentType).Return([]string{"name"}, nil).Once()

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything,
		map[string]interface{}{"name": "Foo"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "not found", "")).
		Once()

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_NoCategoryInRuntime_PrerequisiteNotMet() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"email": "test@example.com"},
		RuntimeData: map[string]string{categoryTypeKey: testUniquenessUserType},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecFailure, resp.Status)
	assert.Contains(suite.T(), resp.Error.Error.DefaultValue, categoryNameKey)
	suite.mockEntityTypeService.AssertNotCalled(suite.T(), "GetUniqueAttributes")
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity")
}

// The category and the type are read the same way: an empty value leaves nothing to check.
func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_EmptyCategoryOrTypeInRuntime_SkipsCheck() {
	tests := []struct {
		name        string
		runtimeData map[string]string
	}{
		{
			name:        "EmptyCategory",
			runtimeData: map[string]string{categoryNameKey: "", categoryTypeKey: testUniquenessUserType},
		},
		{
			name:        "EmptyType",
			runtimeData: map[string]string{categoryNameKey: string(entitytype.TypeCategoryUser), categoryTypeKey: ""},
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			ctx := &providers.NodeContext{
				ExecutionID: "flow-1",
				UserInputs:  map[string]string{"email": "test@example.com"},
				RuntimeData: tt.runtimeData,
			}

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
			suite.mockEntityTypeService.AssertNotCalled(suite.T(), "GetUniqueAttributes")
		})
	}
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_InvalidCategoryInRuntime_ReturnsError() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"email": "test@example.com"},
		RuntimeData: map[string]string{
			categoryNameKey: "machine",
			categoryTypeKey: testUniquenessUserType,
		},
	}

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
	suite.mockEntityTypeService.AssertNotCalled(suite.T(), "GetUniqueAttributes")
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_UniqueAttrNotInInputs_Skipped() {
	// Schema says "email" is unique, but the user hasn't provided it yet — skip the check.
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"username": "newuser"},
		RuntimeData: map[string]string{
			categoryNameKey: string(entitytype.TypeCategoryUser),
			categoryTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{"email", "username"}, nil)

	freeID := (*string)(nil)
	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything,
		map[string]interface{}{"username": "newuser"}).
		Return(freeID, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "not found", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	// email was NOT in UserInputs so IdentifyEntity must not be called for it
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything,
		map[string]interface{}{"email": ""})
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_SchemaServiceError_ReturnsFailure() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"email": "test@example.com"},
		RuntimeData: map[string]string{
			categoryNameKey: string(entitytype.TypeCategoryUser),
			categoryTypeKey: testUniquenessUserType,
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
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything, mock.Anything)
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_IdentifyEntitySystemError_ReturnsFailure() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"email": "test@example.com"},
		RuntimeData: map[string]string{
			categoryNameKey: string(entitytype.TypeCategoryUser),
			categoryTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{"email"}, nil)

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything,
		map[string]interface{}{"email": "test@example.com"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "db error", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_AmbiguousLookup_ReturnsFailure() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"email": "test@example.com"},
		RuntimeData: map[string]string{
			categoryNameKey: string(entitytype.TypeCategoryUser),
			categoryTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{"email"}, nil)

	suite.mockEntityProvider.On("IdentifyEntity", mock.Anything,
		map[string]interface{}{"email": "test@example.com"}).
		Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAmbiguousEntity, "ambiguous", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), resp)
}

func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_NoUniqueAttributesInSchema_ReturnsComplete() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		UserInputs:  map[string]string{"given_name": "John"},
		RuntimeData: map[string]string{
			categoryNameKey: string(entitytype.TypeCategoryUser),
			categoryTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecComplete, resp.Status)
	suite.mockEntityProvider.AssertNotCalled(suite.T(), "IdentifyEntity", mock.Anything, mock.Anything)
}

// A sign-in boundary on the step must not narrow the lookup: uniqueness stays deployment wide.
func (suite *AttributeUniquenessValidatorTestSuite) TestExecute_SignInBoundary_LookupIsNotBounded() {
	ctx := &providers.NodeContext{
		ExecutionID: "flow-1",
		Context: authnprovidercm.WithSignInBoundary(context.Background(),
			authnprovidercm.SignInBoundary{OUID: "ou-1", ApplicationID: "app-1"}),
		UserInputs: map[string]string{"email": "taken@example.com"},
		RuntimeData: map[string]string{
			categoryNameKey: string(entitytype.TypeCategoryUser),
			categoryTypeKey: testUniquenessUserType,
		},
	}

	suite.mockEntityTypeService.On("GetUniqueAttributes", mock.Anything, mock.Anything, testUniquenessUserType).
		Return([]string{"email"}, nil)
	existingID := testExistingUserID
	suite.mockEntityProvider.On("IdentifyEntity",
		mock.MatchedBy(func(c context.Context) bool {
			_, bounded := authnprovidercm.SignInBoundaryFrom(c)
			return !bounded
		}), map[string]interface{}{"email": "taken@example.com"}).
		Return(&existingID, nil).Once()

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), providers.ExecUserInputRequired, resp.Status)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func TestAttributeUniquenessValidatorSuite(t *testing.T) {
	suite.Run(t, new(AttributeUniquenessValidatorTestSuite))
}

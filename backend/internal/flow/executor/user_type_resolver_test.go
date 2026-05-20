/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
	inboundmodel "github.com/thunder-id/thunderid/internal/inboundclient/model"
	i18ncore "github.com/thunder-id/thunderid/internal/system/i18n/core"

	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	appmodel "github.com/thunder-id/thunderid/internal/application/model"
	"github.com/thunder-id/thunderid/internal/entitytype"
	"github.com/thunder-id/thunderid/internal/flow/common"
	"github.com/thunder-id/thunderid/internal/flow/core"
	"github.com/thunder-id/thunderid/internal/system/error/serviceerror"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

type UserTypeResolverTestSuite struct {
	suite.Suite
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	mockFlowFactory       *coremock.FlowFactoryInterfaceMock
	mockOUService         *oumock.OrganizationUnitServiceInterfaceMock
	executor              *userTypeResolver
}

func TestUserTypeResolverSuite(t *testing.T) {
	suite.Run(t, new(UserTypeResolverTestSuite))
}

func (suite *UserTypeResolverTestSuite) SetupTest() {
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())

	defaultInputs := []common.Input{
		{
			Ref:        "usertype_input",
			Identifier: userTypeKey,
			Type:       "SELECT",
			Required:   true,
		},
	}

	// Mock the CreateExecutor method to return a base executor
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameUserTypeResolver, common.ExecutorTypeRegistration,
		defaultInputs, []common.Input{}).
		Return(createMockUserTypeResolverExecutor(suite.T()))

	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.executor = newUserTypeResolver(suite.mockFlowFactory, suite.mockEntityTypeService, suite.mockOUService)
}

func createMockUserTypeResolverExecutor(t *testing.T) core.ExecutorInterface {
	defaultInputs := []common.Input{
		{
			Ref:        "usertype_input",
			Identifier: userTypeKey,
			Type:       "SELECT",
			Required:   true,
		},
	}
	mockExec := coremock.NewExecutorInterfaceMock(t)
	mockExec.On("GetName").Return(ExecutorNameUserTypeResolver).Maybe()
	mockExec.On("GetType").Return(common.ExecutorTypeRegistration).Maybe()
	mockExec.On("GetDefaultInputs").Return(defaultInputs).Maybe()
	mockExec.On("GetPrerequisites").Return([]common.Input{}).Maybe()

	// HasRequiredInputs returns true if userType input is present
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *core.NodeContext, execResp *common.ExecutorResponse) bool {
			if val, ok := ctx.UserInputs[userTypeKey]; ok && val != "" {
				return true
			}
			return false
		},
	).Maybe()

	return mockExec
}

func (suite *UserTypeResolverTestSuite) TestNewUserTypeResolver() {
	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockEntityTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())

	defaultInputs := []common.Input{
		{
			Ref:        "usertype_input",
			Identifier: userTypeKey,
			Type:       "SELECT",
			Required:   true,
		},
	}

	mockFlowFactory.On("CreateExecutor", ExecutorNameUserTypeResolver, common.ExecutorTypeRegistration,
		defaultInputs, []common.Input{}).
		Return(createMockUserTypeResolverExecutor(suite.T()))

	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	executor := newUserTypeResolver(mockFlowFactory, mockEntityTypeService, mockOUService)

	assert.NotNil(suite.T(), executor)
	assert.Equal(suite.T(), ExecutorNameUserTypeResolver, executor.GetName())
}

func (suite *UserTypeResolverTestSuite) TestExecute_AuthenticationFlow_WithAllowedUserTypes() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer"},
			},
		},
		RuntimeData: map[string]string{},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecComplete, result.Status)
	assert.Empty(suite.T(), result.RuntimeData[userTypeKey])
	suite.mockEntityTypeService.AssertNotCalled(suite.T(), "GetEntityTypeByName")
}

func (suite *UserTypeResolverTestSuite) TestExecute_AuthenticationFlow_NoAllowedUserTypes() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeAuthentication,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{},
			},
		},
		RuntimeData: map[string]string{},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "Authentication not available for this application", result.FailureReason)
	suite.mockEntityTypeService.AssertNotCalled(suite.T(), "GetEntityTypeByName")
}

func (suite *UserTypeResolverTestSuite) TestExecute_UnsupportedFlowType() {
	testCases := []struct {
		name     string
		flowType common.FlowType
	}{
		{
			name:     "UnknownFlowType",
			flowType: common.FlowType("UNKNOWN"),
		},
		{
			name:     "EmptyFlowType",
			flowType: common.FlowType(""),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    tc.flowType,
				Application: appmodel.Application{
					InboundAuthProfile: inboundmodel.InboundAuthProfile{
						AllowedUserTypes: []string{"employee"},
					},
				},
				RuntimeData: map[string]string{},
			}

			result, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), result)
			assert.Equal(suite.T(), common.ExecComplete, result.Status)
			assert.Empty(suite.T(), result.RuntimeData[userTypeKey])
			suite.mockEntityTypeService.AssertNotCalled(suite.T(), "GetEntityTypeByName")
		})
	}
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserTypeProvidedInInput_Success() {
	testCases := []struct {
		name             string
		allowedUserTypes []string
		providedUserType string
		expectedOUID     string
	}{
		{
			name:             "Valid user type with OU",
			allowedUserTypes: []string{"employee", "customer"},
			providedUserType: "employee",
			expectedOUID:     "ou-123",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    common.FlowTypeRegistration,
				Application: appmodel.Application{
					InboundAuthProfile: inboundmodel.InboundAuthProfile{
						AllowedUserTypes: tc.allowedUserTypes,
					},
				},
				UserInputs: map[string]string{
					userTypeKey: tc.providedUserType,
				},
				RuntimeData: map[string]string{},
			}

			entityType := &entitytype.EntityType{
				ID:                    "schema-123",
				Name:                  tc.providedUserType,
				OUID:                  tc.expectedOUID,
				AllowSelfRegistration: true,
			}
			suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, tc.providedUserType).
				Return(entityType, nil)

			result, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), result)
			assert.Equal(suite.T(), common.ExecComplete, result.Status)
			assert.Equal(suite.T(), tc.providedUserType, result.RuntimeData[userTypeKey])
			assert.Equal(suite.T(), tc.expectedOUID, result.RuntimeData[defaultOUIDKey])

			suite.mockEntityTypeService.AssertExpectations(suite.T())
		})
	}
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserTypeProvidedInInput_NoOU() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer"},
			},
		},
		UserInputs: map[string]string{
			userTypeKey: "employee",
		},
		RuntimeData: map[string]string{},
	}

	entityType := &entitytype.EntityType{
		ID:                    "schema-123",
		Name:                  "employee",
		OUID:                  "",
		AllowSelfRegistration: true,
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(entityType, nil)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "no organization unit found for user type")
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserTypeProvidedInInput_NotAllowed() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer"},
			},
		},
		UserInputs: map[string]string{
			userTypeKey: "partner",
		},
		RuntimeData: map[string]string{},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "Application does not allow registration for the user type", result.FailureReason)
	suite.mockEntityTypeService.AssertNotCalled(suite.T(), "GetEntityTypeByName")
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserTypeProvidedInInput_OUResolutionFails() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee"},
			},
		},
		UserInputs: map[string]string{
			userTypeKey: "employee",
		},
		RuntimeData: map[string]string{},
	}

	svcErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "SCHEMA-500",
		Error: i18ncore.I18nMessage{
			Key: "error.test.internal_server_error", DefaultValue: "Internal Server Error",
		},
		ErrorDescription: i18ncore.I18nMessage{
			Key: "error.test.failed_to_retrieve_ou", DefaultValue: "Failed to retrieve OU",
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(nil, svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to resolve user type")
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_NoAllowedUserTypes() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "Self-registration not available for this application", result.FailureReason)
	suite.mockEntityTypeService.AssertNotCalled(suite.T(), "GetEntityTypeByName")
}

func (suite *UserTypeResolverTestSuite) TestExecute_SingleAllowedUserType_Success() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	entityType := &entitytype.EntityType{
		ID:                    "schema-123",
		Name:                  "employee",
		OUID:                  "ou-123",
		AllowSelfRegistration: true,
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(entityType, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecComplete, result.Status)
	assert.Equal(suite.T(), "employee", result.RuntimeData[userTypeKey])
	assert.Equal(suite.T(), "ou-123", result.RuntimeData[defaultOUIDKey])

	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_SingleAllowedUserType_NoOU() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	entityType := &entitytype.EntityType{
		ID:                    "schema-123",
		Name:                  "employee",
		OUID:                  "",
		AllowSelfRegistration: true,
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(entityType, nil)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "no organization unit found for user type")
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_SingleAllowedUserType_OUResolutionFails() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	svcErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "SCHEMA-500",
		Error: i18ncore.I18nMessage{
			Key: "error.test.internal_server_error", DefaultValue: "Internal Server Error",
		},
		ErrorDescription: i18ncore.I18nMessage{
			Key: "error.test.failed_to_retrieve_ou", DefaultValue: "Failed to retrieve OU",
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(nil, svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to resolve user type")
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_MultipleAllowedUserTypes_PromptUser() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer", "partner"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	// Mock all three user types with self registration enabled
	for _, userType := range []string{"employee", "customer", "partner"} {
		entityType := &entitytype.EntityType{
			ID:                    "schema-" + userType,
			Name:                  userType,
			OUID:                  "ou-" + userType,
			AllowSelfRegistration: true,
		}
		suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, userType).
			Return(entityType, nil)
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, result.Status)
	assert.NotEmpty(suite.T(), result.Inputs)
	assert.Len(suite.T(), result.Inputs, 1)

	requiredInput := result.Inputs[0]
	assert.Equal(suite.T(), userTypeKey, requiredInput.Identifier)
	assert.Equal(suite.T(), "SELECT", requiredInput.Type)
	assert.Equal(suite.T(), "usertype_input", requiredInput.Ref)
	assert.True(suite.T(), requiredInput.Required)
	assert.ElementsMatch(suite.T(), []string{"employee", "customer", "partner"}, requiredInput.Options)

	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_EmptyUserTypeInput() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer"},
			},
		},
		UserInputs: map[string]string{
			userTypeKey: "",
		},
		RuntimeData: map[string]string{},
	}

	// Mock both user types with self registration enabled
	for _, userType := range []string{"employee", "customer"} {
		entityType := &entitytype.EntityType{
			ID:                    "schema-" + userType,
			Name:                  userType,
			OUID:                  "ou-" + userType,
			AllowSelfRegistration: true,
		}
		suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, userType).
			Return(entityType, nil)
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, result.Status)
	assert.NotEmpty(suite.T(), result.Inputs)
	assert.Len(suite.T(), result.Inputs, 1)

	requiredInput := result.Inputs[0]
	assert.Equal(suite.T(), userTypeKey, requiredInput.Identifier)
	assert.Equal(suite.T(), "SELECT", requiredInput.Type)

	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserTypeProvidedInInput_SelfRegistrationDisabled() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee"},
			},
		},
		UserInputs: map[string]string{
			userTypeKey: "employee",
		},
		RuntimeData: map[string]string{},
	}

	entityType := &entitytype.EntityType{
		ID:                    "schema-123",
		Name:                  "employee",
		OUID:                  "ou-123",
		AllowSelfRegistration: false,
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, mock.Anything, "employee").
		Return(entityType, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "Self-registration not enabled for the user type", result.FailureReason)
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_SingleAllowedUserType_SelfRegistrationDisabled() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	entityType := &entitytype.EntityType{
		ID:                    "schema-123",
		Name:                  "employee",
		OUID:                  "ou-123",
		AllowSelfRegistration: false,
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", mock.Anything, mock.Anything, "employee").
		Return(entityType, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "Self-registration not enabled for the user type", result.FailureReason)
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_MultipleAllowedUserTypes_OnlyOneSelfRegEnabled() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer", "partner"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	// Only customer has self-registration enabled
	employeeSchema := &entitytype.EntityType{
		ID:                    "schema-employee",
		Name:                  "employee",
		OUID:                  "ou-employee",
		AllowSelfRegistration: false,
	}
	customerSchema := &entitytype.EntityType{
		ID:                    "schema-customer",
		Name:                  "customer",
		OUID:                  "ou-customer",
		AllowSelfRegistration: true,
	}
	partnerSchema := &entitytype.EntityType{
		ID:                    "schema-partner",
		Name:                  "partner",
		OUID:                  "ou-partner",
		AllowSelfRegistration: false,
	}

	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(employeeSchema, nil)
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "customer").
		Return(customerSchema, nil)
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "partner").
		Return(partnerSchema, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecComplete, result.Status)
	assert.Equal(suite.T(), "customer", result.RuntimeData[userTypeKey])
	assert.Equal(suite.T(), "ou-customer", result.RuntimeData[defaultOUIDKey])
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_MultipleAllowedUserTypes_NoSelfRegEnabled() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	// None have self-registration enabled
	employeeSchema := &entitytype.EntityType{
		ID:                    "schema-employee",
		Name:                  "employee",
		OUID:                  "ou-employee",
		AllowSelfRegistration: false,
	}
	customerSchema := &entitytype.EntityType{
		ID:                    "schema-customer",
		Name:                  "customer",
		OUID:                  "ou-customer",
		AllowSelfRegistration: false,
	}

	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(employeeSchema, nil)
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "customer").
		Return(customerSchema, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "Self-registration not available for this application", result.FailureReason)
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_MultipleAllowedUserTypes_SchemaResolutionFails() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	// First schema succeeds, second fails
	employeeSchema := &entitytype.EntityType{
		ID:                    "schema-employee",
		Name:                  "employee",
		OUID:                  "ou-employee",
		AllowSelfRegistration: true,
	}
	svcErr := &serviceerror.ServiceError{
		Type: serviceerror.ServerErrorType,
		Code: "SCHEMA-500",
		Error: i18ncore.I18nMessage{
			Key: "error.test.internal_server_error", DefaultValue: "Internal Server Error",
		},
		ErrorDescription: i18ncore.I18nMessage{
			Key: "error.test.failed_to_retrieve_schema", DefaultValue: "Failed to retrieve schema",
		},
	}

	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(employeeSchema, nil)
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "customer").
		Return(nil, svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to resolve user type")
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_RegistrationFlow_NodeAllowedUserTypes_FiltersAppAllowed() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer", "partner"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyAllowedUserTypes: []interface{}{"employee", "customer"},
		},
	}

	// Mock schemas for the two filtered user types
	employeeSchema := &entitytype.EntityType{Name: "employee", OUID: "ou-123", AllowSelfRegistration: true}
	customerSchema := &entitytype.EntityType{Name: "customer", OUID: "ou-456", AllowSelfRegistration: true}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(employeeSchema, nil)
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "customer").
		Return(customerSchema, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, result.Status)
	assert.Len(suite.T(), result.Inputs, 1)
	assert.ElementsMatch(suite.T(), []string{"employee", "customer"}, result.Inputs[0].Options)
}

func (suite *UserTypeResolverTestSuite) TestExecute_RegistrationFlow_NodeAllowedUserTypes_SingleAutoSelect() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer", "partner"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyAllowedUserTypes: []interface{}{"employee"},
		},
	}

	mockSchema := &entitytype.EntityType{Name: "employee", OUID: "ou-123", AllowSelfRegistration: true}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(mockSchema, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecComplete, result.Status)
	assert.Equal(suite.T(), "employee", result.RuntimeData[userTypeKey])
	assert.Equal(suite.T(), "ou-123", result.RuntimeData[defaultOUIDKey])
}

func (suite *UserTypeResolverTestSuite) TestExecute_RegistrationFlow_NodeAllowedUserTypes_NoneMatchApp() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer"},
			},
		},
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyAllowedUserTypes: []interface{}{"partner"},
		},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "No valid user types available for this flow", result.FailureReason)
}

func (suite *UserTypeResolverTestSuite) TestExecute_RegistrationFlow_NodeAllowedUserTypes_InputValidation() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeRegistration,
		Application: appmodel.Application{
			InboundAuthProfile: inboundmodel.InboundAuthProfile{
				AllowedUserTypes: []string{"employee", "customer", "partner"},
			},
		},
		UserInputs:  map[string]string{userTypeKey: "partner"},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyAllowedUserTypes: []interface{}{"employee", "customer"},
		},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	// "partner" is in application allowed but NOT in node allowed, so resolveUserTypeFromInput
	// won't find it in the filtered allowed list
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "Application does not allow registration for the user type", result.FailureReason)
}

func (suite *UserTypeResolverTestSuite) TestGetEntityTypeAndOU_Success() {
	suite.SetupTest()

	entityType := &entitytype.EntityType{
		ID:                    "schema-123",
		Name:                  "employee",
		OUID:                  "ou-123",
		AllowSelfRegistration: true,
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", context.Background(), mock.Anything, "employee").
		Return(entityType, nil)

	schema, ouID, err := suite.executor.getEntityTypeAndOU(context.Background(), "employee")

	assert.Nil(suite.T(), err)
	assert.NotNil(suite.T(), schema)
	assert.Equal(suite.T(), "ou-123", ouID)
	assert.Equal(suite.T(), "employee", schema.Name)
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestGetEntityTypeAndOU_NoOUFound() {
	suite.SetupTest()

	entityType := &entitytype.EntityType{
		ID:                    "schema-123",
		Name:                  "employee",
		OUID:                  "",
		AllowSelfRegistration: true,
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", context.Background(), mock.Anything, "employee").
		Return(entityType, nil)

	schema, ouID, err := suite.executor.getEntityTypeAndOU(context.Background(), "employee")

	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), schema)
	assert.Equal(suite.T(), "", ouID)
	assert.Contains(suite.T(), err.Error(), "no organization unit found for user type")
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestGetEntityTypeAndOU_SchemaNotFound() {
	suite.SetupTest()

	svcErr := &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "SCHEMA-404",
		Error: i18ncore.I18nMessage{Key: "error.test.not_found", DefaultValue: "Not Found"},
		ErrorDescription: i18ncore.I18nMessage{
			Key: "error.test.user_type_not_found", DefaultValue: "User type not found",
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", context.Background(), mock.Anything, "employee").
		Return(nil, svcErr)

	schema, ouID, err := suite.executor.getEntityTypeAndOU(context.Background(), "employee")

	assert.NotNil(suite.T(), err)
	assert.Nil(suite.T(), schema)
	assert.Equal(suite.T(), "", ouID)
	assert.Contains(suite.T(), err.Error(), "failed to resolve user type")
	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_UserTypeProvided_Success() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding, // User Onboarding Flow
		UserInputs: map[string]string{
			userTypeKey: "employee",
		},
		RuntimeData: map[string]string{},
	}

	entityType := &entitytype.EntityType{
		ID:   "schema-123",
		Name: "employee",
		OUID: "ou-123",
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(entityType, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecComplete, result.Status)
	assert.Equal(suite.T(), "employee", result.RuntimeData[userTypeKey])
	assert.Equal(suite.T(), "ou-123", result.RuntimeData[defaultOUIDKey])

	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_UserTypeProvided_Invalid() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs: map[string]string{
			userTypeKey: "invalid_user",
		},
		RuntimeData: map[string]string{},
	}

	// Mock schema retrieval failing
	svcErr := &serviceerror.ServiceError{
		Type:  serviceerror.ClientErrorType,
		Code:  "SCHEMA-404",
		Error: i18ncore.I18nMessage{Key: "error.test.not_found", DefaultValue: "Not Found"},
		ErrorDescription: i18ncore.I18nMessage{
			Key: "error.test.user_type_not_found", DefaultValue: "User type not found",
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "invalid_user").
		Return(nil, svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err) // Logic returns ExecFailure, not error
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "Invalid user type", result.FailureReason)

	suite.mockEntityTypeService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_NoUserType_SchemaListEmpty() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	// Mock GetEntityTypeList returning empty list
	emptyList := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(emptyList, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "No user types available", result.FailureReason)
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_NoUserType_SchemaListError() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	svcErr := &serviceerror.ServiceError{
		Type:  serviceerror.ServerErrorType,
		Error: i18ncore.I18nMessage{Key: "error.test.simulated_error", DefaultValue: "Simulated Error"},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(nil, svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "Failed to retrieve user types", result.FailureReason)
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_NoUserType_SingleSchema_AutoSelect() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	// Mock GetEntityTypeList returning a single schema
	schemaList := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{Name: "employee", OUID: "ou-123"},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(schemaList, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecComplete, result.Status)
	assert.Equal(suite.T(), "employee", result.RuntimeData[userTypeKey])
	assert.Equal(suite.T(), "ou-123", result.RuntimeData[defaultOUIDKey])
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_NoUserType_PromptUser() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
	}

	// Mock GetEntityTypeList returning schemas
	schemaList := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{Name: "employee"},
			{Name: "customer"},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(schemaList, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, result.Status)
	assert.Len(suite.T(), result.Inputs, 1)

	// Verify options in the prompt
	requiredInput := result.Inputs[0]
	assert.Equal(suite.T(), userTypeKey, requiredInput.Identifier)
	assert.ElementsMatch(suite.T(), []string{"employee", "customer"}, requiredInput.Options)
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_AllowedUserTypes_SingleAutoSelect() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyAllowedUserTypes: []interface{}{"employee"},
		},
	}

	// Mock GetEntityTypeList returning multiple schemas
	schemaList := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{Name: "employee", OUID: "ou-123"},
			{Name: "customer", OUID: "ou-456"},
			{Name: "partner", OUID: "ou-789"},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(schemaList, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecComplete, result.Status)
	assert.Equal(suite.T(), "employee", result.RuntimeData[userTypeKey])
	assert.Equal(suite.T(), "ou-123", result.RuntimeData[defaultOUIDKey])
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_AllowedUserTypes_MultiplePrompt() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyAllowedUserTypes: []interface{}{"employee", "customer"},
		},
	}

	// Mock GetEntityTypeList returning multiple schemas including non-allowed ones
	schemaList := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{Name: "employee", OUID: "ou-123"},
			{Name: "customer", OUID: "ou-456"},
			{Name: "partner", OUID: "ou-789"},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(schemaList, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, result.Status)
	assert.Len(suite.T(), result.Inputs, 1)

	requiredInput := result.Inputs[0]
	assert.ElementsMatch(suite.T(), []string{"employee", "customer"}, requiredInput.Options)
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_AllowedUserTypes_NoneValidInSystem() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyAllowedUserTypes: []interface{}{"nonexistent"},
		},
	}

	// Mock GetEntityTypeList returning schemas that don't match the allowed list
	schemaList := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{Name: "employee", OUID: "ou-123"},
			{Name: "customer", OUID: "ou-456"},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(schemaList, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "No valid user types available for this flow", result.FailureReason)
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_AllowedUserTypes_InputNotInAllowed() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{userTypeKey: "partner"},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyAllowedUserTypes: []interface{}{"employee", "customer"},
		},
	}

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "User type not allowed for this flow", result.FailureReason)
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboardingFlow_AllowedUserTypes_InputInAllowed() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{userTypeKey: "employee"},
		RuntimeData: map[string]string{},
		NodeProperties: map[string]interface{}{
			propertyKeyAllowedUserTypes: []interface{}{"employee", "customer"},
		},
	}

	mockSchema := &entitytype.EntityType{Name: "employee", OUID: "ou-123"}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(mockSchema, nil)

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecComplete, result.Status)
	assert.Equal(suite.T(), "employee", result.RuntimeData[userTypeKey])
	assert.Equal(suite.T(), "ou-123", result.RuntimeData[defaultOUIDKey])
}

func (suite *UserTypeResolverTestSuite) TestPromptUserSelection_ForwardsInputsInForwardedData() {
	suite.SetupTest()

	options := []string{"employee", "customer", "partner"}
	execResp := &common.ExecutorResponse{
		ForwardedData: map[string]interface{}{},
	}

	suite.executor.promptUserSelection(execResp, options)

	// Verify status and inputs are set
	assert.Equal(suite.T(), common.ExecUserInputRequired, execResp.Status)
	assert.Len(suite.T(), execResp.Inputs, 1)
	assert.Equal(suite.T(), userTypeKey, execResp.Inputs[0].Identifier)
	assert.ElementsMatch(suite.T(), options, execResp.Inputs[0].Options)

	// Verify ForwardedData is set with inputs
	assert.NotNil(suite.T(), execResp.ForwardedData)
	forwardedInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs]
	assert.True(suite.T(), ok, "ForwardedData should contain 'inputs' key")

	// Type assert and verify
	inputsSlice, ok := forwardedInputs.([]common.Input)
	assert.True(suite.T(), ok, "ForwardedData['inputs'] should be []common.Input")
	assert.Len(suite.T(), inputsSlice, 1)
	assert.Equal(suite.T(), userTypeKey, inputsSlice[0].Identifier)
	assert.ElementsMatch(suite.T(), options, inputsSlice[0].Options)
}

func (suite *UserTypeResolverTestSuite) TestPromptUserSelection_WithEmptyOptions() {
	suite.SetupTest()

	options := []string{}
	execResp := &common.ExecutorResponse{
		ForwardedData: map[string]interface{}{},
	}

	suite.executor.promptUserSelection(execResp, options)

	// Verify status is set
	assert.Equal(suite.T(), common.ExecUserInputRequired, execResp.Status)
	assert.Len(suite.T(), execResp.Inputs, 1)

	// Verify ForwardedData is still set even with empty options
	assert.NotNil(suite.T(), execResp.ForwardedData)
	forwardedInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs]
	assert.True(suite.T(), ok, "ForwardedData should contain 'inputs' key even with empty options")

	inputsSlice, ok := forwardedInputs.([]common.Input)
	assert.True(suite.T(), ok)
	assert.Len(suite.T(), inputsSlice, 1)
	assert.Empty(suite.T(), inputsSlice[0].Options)
}

func (suite *UserTypeResolverTestSuite) TestPromptUserSelection_PreservesExistingForwardedData() {
	suite.SetupTest()

	options := []string{"employee"}
	execResp := &common.ExecutorResponse{
		ForwardedData: map[string]interface{}{
			"existingKey": "existingValue",
		},
	}

	suite.executor.promptUserSelection(execResp, options)

	// Verify existing ForwardedData is preserved
	assert.NotNil(suite.T(), execResp.ForwardedData)
	assert.Equal(suite.T(), "existingValue", execResp.ForwardedData["existingKey"])

	// Verify new inputs key is added
	forwardedInputs, ok := execResp.ForwardedData[common.ForwardedDataKeyInputs]
	assert.True(suite.T(), ok)
	assert.NotNil(suite.T(), forwardedInputs)
}

// --- OU-first flow tests (UserOnboarding with ouId in RuntimeData) ---

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboarding_OUFirst_UserTypeValidForOU() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs: map[string]string{
			userTypeKey: "employee",
		},
		RuntimeData: map[string]string{
			ouIDKey: "child-ou-456",
		},
	}

	entityType := &entitytype.EntityType{
		ID:   "schema-123",
		Name: "employee",
		OUID: "parent-ou-123",
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(entityType, nil)
	suite.mockOUService.On("IsParent", mock.Anything, "parent-ou-123", "child-ou-456").
		Return(true, (*serviceerror.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecComplete, result.Status)
	assert.Equal(suite.T(), "employee", result.RuntimeData[userTypeKey])
	assert.Equal(suite.T(), "parent-ou-123", result.RuntimeData[defaultOUIDKey])
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboarding_OUFirst_UserTypeNotValidForOU() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs: map[string]string{
			userTypeKey: "employee",
		},
		RuntimeData: map[string]string{
			ouIDKey: "unrelated-ou-789",
		},
	}

	entityType := &entitytype.EntityType{
		ID:   "schema-123",
		Name: "employee",
		OUID: "parent-ou-123",
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(entityType, nil)
	suite.mockOUService.On("IsParent", mock.Anything, "parent-ou-123", "unrelated-ou-789").
		Return(false, (*serviceerror.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "User type is not valid for the selected organization unit", result.FailureReason)
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboarding_OUFirst_IsParentServiceError() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs: map[string]string{
			userTypeKey: "employee",
		},
		RuntimeData: map[string]string{
			ouIDKey: "child-ou-456",
		},
	}

	entityType := &entitytype.EntityType{
		ID:   "schema-123",
		Name: "employee",
		OUID: "parent-ou-123",
	}
	suite.mockEntityTypeService.On("GetEntityTypeByName", ctx.Context, mock.Anything, "employee").
		Return(entityType, nil)
	svcErr := &serviceerror.ServiceError{
		Type:  serviceerror.ServerErrorType,
		Error: i18ncore.I18nMessage{Key: "error.test.internal_error", DefaultValue: "internal error"},
	}
	suite.mockOUService.On("IsParent", mock.Anything, "parent-ou-123", "child-ou-456").
		Return(false, svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to validate user type against selected OU")
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboarding_OUFirst_FiltersSchemasByOU() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			ouIDKey: "child-ou-456",
		},
	}

	schemaList := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{Name: "employee", OUID: "parent-ou-123"},
			{Name: "customer", OUID: "other-ou-789"},
			{Name: "partner", OUID: "parent-ou-123"},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(schemaList, nil)

	// employee's OU is ancestor of selected OU
	suite.mockOUService.On("IsParent", mock.Anything, "parent-ou-123", "child-ou-456").
		Return(true, (*serviceerror.ServiceError)(nil))
	// customer's OU is NOT ancestor of selected OU
	suite.mockOUService.On("IsParent", mock.Anything, "other-ou-789", "child-ou-456").
		Return(false, (*serviceerror.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecUserInputRequired, result.Status)
	assert.Len(suite.T(), result.Inputs, 1)
	// Only employee and partner should remain (both have parent-ou-123)
	assert.ElementsMatch(suite.T(), []string{"employee", "partner"}, result.Inputs[0].Options)
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboarding_OUFirst_FiltersSchemasToSingle_AutoSelect() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			ouIDKey: "child-ou-456",
		},
	}

	schemaList := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{Name: "employee", OUID: "parent-ou-123"},
			{Name: "customer", OUID: "other-ou-789"},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(schemaList, nil)

	suite.mockOUService.On("IsParent", mock.Anything, "parent-ou-123", "child-ou-456").
		Return(true, (*serviceerror.ServiceError)(nil))
	suite.mockOUService.On("IsParent", mock.Anything, "other-ou-789", "child-ou-456").
		Return(false, (*serviceerror.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecComplete, result.Status)
	assert.Equal(suite.T(), "employee", result.RuntimeData[userTypeKey])
	assert.Equal(suite.T(), "parent-ou-123", result.RuntimeData[defaultOUIDKey])
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboarding_OUFirst_AllSchemasFilteredOut() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			ouIDKey: "unrelated-ou-999",
		},
	}

	schemaList := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{Name: "employee", OUID: "ou-123"},
			{Name: "customer", OUID: "ou-456"},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(schemaList, nil)

	suite.mockOUService.On("IsParent", mock.Anything, "ou-123", "unrelated-ou-999").
		Return(false, (*serviceerror.ServiceError)(nil))
	suite.mockOUService.On("IsParent", mock.Anything, "ou-456", "unrelated-ou-999").
		Return(false, (*serviceerror.ServiceError)(nil))

	result, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, result.Status)
	assert.Equal(suite.T(), "No valid user types available for this flow", result.FailureReason)
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *UserTypeResolverTestSuite) TestExecute_UserOnboarding_OUFirst_IsParentErrorAbortsFiltering() {
	suite.SetupTest()

	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    common.FlowTypeUserOnboarding,
		UserInputs:  map[string]string{},
		RuntimeData: map[string]string{
			ouIDKey: "child-ou-456",
		},
	}

	schemaList := &entitytype.EntityTypeListResponse{
		Types: []entitytype.EntityTypeListItem{
			{Name: "employee", OUID: "parent-ou-123"},
			{Name: "customer", OUID: "error-ou"},
		},
	}
	suite.mockEntityTypeService.On("GetEntityTypeList", ctx.Context, mock.Anything, 100, 0, false).
		Return(schemaList, nil)

	suite.mockOUService.On("IsParent", mock.Anything, "parent-ou-123", "child-ou-456").
		Return(true, (*serviceerror.ServiceError)(nil))
	svcErr := &serviceerror.ServiceError{
		Type:  serviceerror.ServerErrorType,
		Error: i18ncore.I18nMessage{Key: "error.test.internal_error", DefaultValue: "internal error"},
	}
	suite.mockOUService.On("IsParent", mock.Anything, "error-ou", "child-ou-456").
		Return(false, svcErr)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Contains(suite.T(), err.Error(), "failed to check OU ancestry for schema customer")
	suite.mockOUService.AssertExpectations(suite.T())
}

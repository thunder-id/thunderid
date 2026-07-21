/*
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
	"github.com/thunder-id/thunderid/pkg/thunderidengine/providers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/thunder-id/thunderid/internal/ou"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/tests/mocks/authnprovider/managermock"
	"github.com/thunder-id/thunderid/tests/mocks/entitytypemock"
	"github.com/thunder-id/thunderid/tests/mocks/flow/coremock"
	"github.com/thunder-id/thunderid/tests/mocks/oumock"
)

const testOUID = "ou-123"

type OUExecutorTestSuite struct {
	suite.Suite
	mockOUService         *oumock.OrganizationUnitServiceInterfaceMock
	mockFlowFactory       *coremock.FlowFactoryInterfaceMock
	mockAuthnProvider     *managermock.AuthnProviderManagerMock
	mockEntityTypeService *entitytypemock.EntityTypeServiceInterfaceMock
	executor              *ouExecutor
}

func TestOUExecutorSuite(t *testing.T) {
	suite.Run(t, new(OUExecutorTestSuite))
}

func (suite *OUExecutorTestSuite) SetupTest() {
	suite.mockOUService = oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())
	suite.mockAuthnProvider = managermock.NewAuthnProviderManagerMock(suite.T())
	suite.mockEntityTypeService = entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())

	defaultInputs := []providers.Input{
		{
			Identifier: userInputOuName,
			Required:   true,
			Type:       "string",
		},
		{
			Identifier: userInputOuHandle,
			Required:   true,
			Type:       "string",
		},
	}

	// Mock the CreateExecutor method to return a base executor
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameOUCreation, providers.ExecutorTypeRegistration,
		defaultInputs, []providers.Input{}, mock.Anything).
		Return(newMockExecutor("TestOUExecutor", providers.ExecutorTypeUtility, defaultInputs, []providers.Input{}))

	suite.executor = newOUExecutor(
		suite.mockFlowFactory, suite.mockOUService, suite.mockAuthnProvider, suite.mockEntityTypeService)
}

// newMockExecutor creates a mock executor for testing purposes
func newMockExecutor(name string, executorType providers.ExecutorType, defaultInputs []providers.Input,
	prerequisites []providers.Input) providers.Executor {
	mockExec := coremock.NewExecutorInterfaceMock(&testing.T{})
	mockExec.On("GetName").Return(name)
	mockExec.On("GetType").Return(executorType)
	mockExec.On("GetDefaultInputs").Return(defaultInputs)
	mockExec.On("GetPrerequisites").Return(prerequisites)
	mockExec.On("GetInputs", mock.Anything).Return(defaultInputs)
	mockExec.On("GetRequiredInputs", mock.Anything).Return(defaultInputs)
	mockExec.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(
		func(ctx *providers.NodeContext, execResp *providers.ExecutorResponse) bool {
			requiredInputs := defaultInputs
			if execResp.Inputs == nil {
				execResp.Inputs = make([]providers.Input, 0)
			}
			if len(ctx.UserInputs) == 0 && len(ctx.RuntimeData) == 0 {
				execResp.Inputs = append(execResp.Inputs, requiredInputs...)
				return false
			}
			requireData := false
			for _, input := range requiredInputs {
				if _, ok := ctx.UserInputs[input.Identifier]; !ok {
					if _, ok := ctx.RuntimeData[input.Identifier]; ok {
						continue
					}
					requireData = true
					execResp.Inputs = append(execResp.Inputs, input)
				}
			}
			return !requireData
		})
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).Return(true)
	mockExec.On("GetUserIDFromContext", mock.Anything, mock.Anything, mock.Anything).Return("")
	return mockExec
}

func (suite *OUExecutorTestSuite) TestNewOUExecutor() {
	mockFlowFactory := coremock.NewFlowFactoryInterfaceMock(suite.T())
	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

	defaultInputs := []providers.Input{
		{
			Identifier: userInputOuName,
			Required:   true,
			Type:       "string",
		},
		{
			Identifier: userInputOuHandle,
			Required:   true,
			Type:       "string",
		},
	}

	// Mock the CreateExecutor method
	mockFlowFactory.On("CreateExecutor", ExecutorNameOUCreation, providers.ExecutorTypeRegistration,
		defaultInputs, []providers.Input{}, mock.Anything).
		Return(newMockExecutor("OUExecutor", providers.ExecutorTypeRegistration, defaultInputs, []providers.Input{}))

	mockAuthnProvider := managermock.NewAuthnProviderManagerMock(suite.T())
	mockEntityTypeService := entitytypemock.NewEntityTypeServiceInterfaceMock(suite.T())
	executor := newOUExecutor(mockFlowFactory, mockOUService, mockAuthnProvider, mockEntityTypeService)

	assert.NotNil(suite.T(), executor)
	assert.Equal(suite.T(), "OUExecutor", executor.GetName())

	defaultInputsResult := executor.GetDefaultInputs()
	assert.Len(suite.T(), defaultInputsResult, 2)
	assert.Equal(suite.T(), userInputOuName, defaultInputsResult[0].Identifier)
	assert.True(suite.T(), defaultInputsResult[0].Required)
	assert.Equal(suite.T(), userInputOuHandle, defaultInputsResult[1].Identifier)
	assert.True(suite.T(), defaultInputsResult[1].Required)
}

func (suite *OUExecutorTestSuite) TestExecutorMetadata() {
	testCases := []struct {
		name     string
		testFunc func()
	}{
		{
			name: "GetName returns correct executor name",
			testFunc: func() {
				assert.Equal(suite.T(), "TestOUExecutor", suite.executor.GetName())
			},
		},
		{
			name: "GetDefaultInputs returns two inputs",
			testFunc: func() {
				inputs := suite.executor.GetDefaultInputs()
				assert.Len(suite.T(), inputs, 2)
			},
		},
		{
			name: "GetPrerequisites returns empty list",
			testFunc: func() {
				prerequisites := suite.executor.GetPrerequisites()
				assert.Empty(suite.T(), prerequisites)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, tc.testFunc)
	}
}

type ExecuteSuccessTestCase struct {
	name             string
	userInputs       map[string]string
	expectedOUID     string
	expectedRequest  providers.OrganizationUnitRequestWithID
	expectedResponse providers.OrganizationUnit
}

func (suite *OUExecutorTestSuite) TestExecute_Success() {
	testCases := []ExecuteSuccessTestCase{
		{
			name: "Create OU with all fields",
			userInputs: map[string]string{
				userInputOuName:   "Engineering",
				userInputOuHandle: "engineering",
			},
			expectedOUID: testOUID,
			expectedRequest: providers.OrganizationUnitRequestWithID{
				Name:   "Engineering",
				Handle: "engineering",
			},
			expectedResponse: providers.OrganizationUnit{
				ID:     testOUID,
				Name:   "Engineering",
				Handle: "engineering",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    providers.FlowTypeRegistration,
				UserInputs:  tc.userInputs,
				RuntimeData: map[string]string{},
			}

			suite.mockOUService.On("CreateOrganizationUnit", mock.Anything, tc.expectedRequest).
				Return(tc.expectedResponse, nil)

			result, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), result)
			assert.Equal(suite.T(), providers.ExecComplete, result.Status)
			assert.Equal(suite.T(), tc.expectedOUID, result.RuntimeData[ouIDKey])
			suite.mockOUService.AssertExpectations(suite.T())
		})
	}
}

type ExecuteNonRegistrationFlowTestCase struct {
	name     string
	flowType providers.FlowType
}

func (suite *OUExecutorTestSuite) TestExecute_NonRegistrationFlow() {
	testCases := []ExecuteNonRegistrationFlowTestCase{
		{
			name:     "Authentication flow",
			flowType: providers.FlowTypeAuthentication,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    tc.flowType,
			}

			result, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), result)
			assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
			assert.Empty(suite.T(), result.RuntimeData[ouIDKey])
		})
	}
}

type ExecutePrerequisitesFailureTestCase struct {
	name        string
	ctx         *providers.NodeContext
	expectedMsg string
}

func (suite *OUExecutorTestSuite) TestExecute_PrerequisitesFailure() {
	mockOUService := oumock.NewOrganizationUnitServiceInterfaceMock(suite.T())

	defaultInputs := []providers.Input{
		{
			Identifier: userInputOuName,
			Required:   true,
			Type:       "string",
		},
		{
			Identifier: userInputOuHandle,
			Required:   true,
			Type:       "string",
		},
	}

	prerequisites := []providers.Input{{Identifier: "requiredField", Required: true, Type: "string"}}

	// Create a mock executor with prerequisites
	mockExec := coremock.NewExecutorInterfaceMock(suite.T())
	mockExec.On("GetName").Return("Test").Maybe()
	mockExec.On("GetType").Return(providers.ExecutorTypeUtility).Maybe()
	mockExec.On("GetDefaultInputs").Return(defaultInputs).Maybe()
	mockExec.On("GetPrerequisites").Return(prerequisites).Maybe()
	mockExec.On("ValidatePrerequisites", mock.Anything, mock.Anything, mock.Anything).Return(
		func(
			ctx *providers.NodeContext,
			execResp *providers.ExecutorResponse,
			_ providers.AuthnProviderManager,
		) bool {
			for _, prerequisite := range prerequisites {
				if _, ok := ctx.UserInputs[prerequisite.Identifier]; !ok {
					if _, ok := ctx.RuntimeData[prerequisite.Identifier]; !ok {
						execResp.Status = providers.ExecFailure
						execResp.Error = &ErrPrerequisitesFailed
						return false
					}
				}
			}
			return true
		}).Maybe()

	// Create a prerequisitesExecutor with the mock interface directly
	prerequisitesExecutor := &ouExecutor{
		Executor:  mockExec,
		ouService: mockOUService,
		logger: log.GetLogger().With(
			log.String(log.LoggerKeyComponentName, ouExecLoggerComponentName)),
	}

	testCases := []ExecutePrerequisitesFailureTestCase{
		{
			name: "Missing prerequisite field",
			ctx: &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    providers.FlowTypeRegistration,
				UserInputs:  map[string]string{},
				RuntimeData: map[string]string{},
			},
			expectedMsg: "Prerequisites validation failed for OU creation",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			result, err := prerequisitesExecutor.Execute(tc.ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), result)
			assert.Equal(suite.T(), providers.ExecFailure, result.Status)
			assert.Equal(suite.T(), tc.expectedMsg, result.Error.Error.DefaultValue)
			mockOUService.AssertNotCalled(suite.T(), "CreateOrganizationUnit", mock.Anything)
		})
	}
}

type ExecuteUserInputRequiredTestCase struct {
	name       string
	userInputs map[string]string
}

func (suite *OUExecutorTestSuite) TestExecute_UserInputRequired() {
	testCases := []ExecuteUserInputRequiredTestCase{
		{
			name:       "No inputs provided",
			userInputs: map[string]string{},
		},
		{
			name: "Missing OU name",
			userInputs: map[string]string{
				userInputOuHandle: "engineering",
			},
		},
		{
			name: "Missing OU handle",
			userInputs: map[string]string{
				userInputOuName: "Engineering",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    providers.FlowTypeRegistration,
				UserInputs:  tc.userInputs,
			}

			result, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), result)
			assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
			assert.NotEmpty(suite.T(), result.Inputs)
			suite.mockOUService.AssertNotCalled(suite.T(), "CreateOrganizationUnit", mock.Anything)
		})
	}
}

func (suite *OUExecutorTestSuite) TestExecute_ErrorScenarios() {
	testCases := []struct {
		name            string
		serviceError    tidcommon.ServiceError
		expectedFailure string
		expectError     bool
		expectNilResult bool
		userInputs      map[string]string
		expectedRequest providers.OrganizationUnitRequestWithID
	}{
		{
			name:            "OU name conflict",
			serviceError:    ou.ErrorOrganizationUnitNameConflict,
			expectedFailure: ErrOUNameConflict.Error.DefaultValue,
			expectError:     false,
			expectNilResult: false,
			userInputs: map[string]string{
				userInputOuName:   "Engineering",
				userInputOuHandle: "engineering",
			},
			expectedRequest: providers.OrganizationUnitRequestWithID{
				Name:   "Engineering",
				Handle: "engineering",
			},
		},
		{
			name:            "OU handle conflict",
			serviceError:    ou.ErrorOrganizationUnitHandleConflict,
			expectedFailure: ErrOUHandleConflict.Error.DefaultValue,
			expectError:     false,
			expectNilResult: false,
			userInputs: map[string]string{
				userInputOuName:   "Engineering",
				userInputOuHandle: "engineering",
			},
			expectedRequest: providers.OrganizationUnitRequestWithID{
				Name:   "Engineering",
				Handle: "engineering",
			},
		},
		{
			name: "Other client error",
			serviceError: tidcommon.ServiceError{
				Type:             tidcommon.ClientErrorType,
				Code:             "OU-9999",
				Error:            tidcommon.I18nMessage{DefaultValue: "Test Error"},
				ErrorDescription: tidcommon.I18nMessage{DefaultValue: "Test error description"},
			},
			expectedFailure: ErrOUCreationFailed.Error.DefaultValue,
			expectError:     false,
			expectNilResult: false,
			userInputs: map[string]string{
				userInputOuName:   "Engineering",
				userInputOuHandle: "engineering",
			},
			expectedRequest: providers.OrganizationUnitRequestWithID{
				Name:   "Engineering",
				Handle: "engineering",
			},
		},
		{
			name:            "Internal server error",
			serviceError:    tidcommon.InternalServerError,
			expectedFailure: "failed to create organization unit",
			expectError:     true,
			expectNilResult: true,
			userInputs: map[string]string{
				userInputOuName:   "Engineering",
				userInputOuHandle: "engineering",
			},
			expectedRequest: providers.OrganizationUnitRequestWithID{
				Name:   "Engineering",
				Handle: "engineering",
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    providers.FlowTypeRegistration,
				UserInputs:  tc.userInputs,
				RuntimeData: map[string]string{},
			}

			suite.mockOUService.On("CreateOrganizationUnit", mock.Anything, tc.expectedRequest).
				Return(providers.OrganizationUnit{}, &tc.serviceError)

			result, err := suite.executor.Execute(ctx)

			if tc.expectError {
				assert.Error(suite.T(), err)
				assert.Equal(suite.T(), tc.expectedFailure, err.Error())
			} else {
				assert.NoError(suite.T(), err)
				assert.Equal(suite.T(), providers.ExecUserInputRequired, result.Status)
				assert.Equal(suite.T(), tc.expectedFailure, result.Error.Error.DefaultValue)
			}

			if tc.expectNilResult {
				assert.Nil(suite.T(), result)
			} else {
				assert.NotNil(suite.T(), result)
			}

			suite.mockOUService.AssertExpectations(suite.T())
		})
	}
}

func (suite *OUExecutorTestSuite) TestExecute_EmptyOUID() {
	suite.SetupTest()

	ctx := &providers.NodeContext{
		ExecutionID: "flow-123",
		FlowType:    providers.FlowTypeRegistration,
		UserInputs: map[string]string{
			userInputOuName:   "Engineering",
			userInputOuHandle: "engineering",
		},
		RuntimeData: map[string]string{},
	}

	expectedRequest := providers.OrganizationUnitRequestWithID{
		Name:   "Engineering",
		Handle: "engineering",
	}

	suite.mockOUService.On("CreateOrganizationUnit", mock.Anything, expectedRequest).
		Return(providers.OrganizationUnit{ID: ""}, nil)

	result, err := suite.executor.Execute(ctx)

	assert.Error(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), "failed to create organization unit", err.Error())
	suite.mockOUService.AssertExpectations(suite.T())
}

func (suite *OUExecutorTestSuite) TestExecute_ParentOuIdProperty() {
	parentOUID := "specific-parent-ou-id"

	testCases := []struct {
		name            string
		nodeProperties  map[string]interface{}
		runtimeData     map[string]string
		expectedRequest providers.OrganizationUnitRequestWithID
	}{
		{
			name:           "parentOuId set to specific UUID",
			nodeProperties: map[string]interface{}{"parentOuId": "specific-parent-ou-id"},
			runtimeData:    map[string]string{},
			expectedRequest: providers.OrganizationUnitRequestWithID{
				Name:   "Engineering",
				Handle: "engineering",
				Parent: &parentOUID,
			},
		},
		{
			name:           "parentOuId set to empty string creates root-level OU",
			nodeProperties: map[string]interface{}{"parentOuId": ""},
			runtimeData:    map[string]string{},
			expectedRequest: providers.OrganizationUnitRequestWithID{
				Name:   "Engineering",
				Handle: "engineering",
			},
		},
		{
			name:           "parentOuId overrides defaultOUID from RuntimeData",
			nodeProperties: map[string]interface{}{"parentOuId": "specific-parent-ou-id"},
			runtimeData:    map[string]string{defaultOUIDKey: "default-ou-from-runtime"},
			expectedRequest: providers.OrganizationUnitRequestWithID{
				Name:   "Engineering",
				Handle: "engineering",
				Parent: &parentOUID,
			},
		},
		{
			name:           "empty parentOuId overrides defaultOUID from RuntimeData",
			nodeProperties: map[string]interface{}{"parentOuId": ""},
			runtimeData:    map[string]string{defaultOUIDKey: "default-ou-from-runtime"},
			expectedRequest: providers.OrganizationUnitRequestWithID{
				Name:   "Engineering",
				Handle: "engineering",
			},
		},
		{
			name:           "parentOuId omitted falls back to defaultOUID",
			nodeProperties: map[string]interface{}{},
			runtimeData:    map[string]string{defaultOUIDKey: "default-ou-from-runtime"},
			expectedRequest: func() providers.OrganizationUnitRequestWithID {
				val := "default-ou-from-runtime"
				return providers.OrganizationUnitRequestWithID{
					Name:   "Engineering",
					Handle: "engineering",
					Parent: &val,
				}
			}(),
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()

			ctx := &providers.NodeContext{
				ExecutionID:    "flow-123",
				FlowType:       providers.FlowTypeRegistration,
				NodeProperties: tc.nodeProperties,
				UserInputs: map[string]string{
					userInputOuName:   "Engineering",
					userInputOuHandle: "engineering",
				},
				RuntimeData: tc.runtimeData,
			}

			suite.mockOUService.On("CreateOrganizationUnit", mock.Anything, tc.expectedRequest).
				Return(providers.OrganizationUnit{ID: testOUID}, nil)

			result, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), result)
			assert.Equal(suite.T(), providers.ExecComplete, result.Status)
			assert.Equal(suite.T(), testOUID, result.RuntimeData[ouIDKey])
			suite.mockOUService.AssertExpectations(suite.T())
		})
	}

	suite.Run("non-string parentOuId returns error", func() {
		suite.SetupTest()

		ctx := &providers.NodeContext{
			ExecutionID:    "flow-123",
			FlowType:       providers.FlowTypeRegistration,
			NodeProperties: map[string]interface{}{"parentOuId": 123},
			UserInputs: map[string]string{
				userInputOuName:   "Engineering",
				userInputOuHandle: "engineering",
			},
			RuntimeData: map[string]string{},
		}

		result, err := suite.executor.Execute(ctx)

		assert.Error(suite.T(), err)
		assert.Nil(suite.T(), result)
		assert.Contains(suite.T(), err.Error(), "parentOuId must be a string")
	})
}

func (suite *OUExecutorTestSuite) TestExecutorHelperMethods() {
	testCases := []struct {
		name     string
		testFunc func()
	}{
		{
			name: "HasRequiredInputs with empty inputs returns false and sets required data",
			testFunc: func() {
				ctx := &providers.NodeContext{
					UserInputs:  map[string]string{},
					RuntimeData: map[string]string{},
				}
				execResp := &providers.ExecutorResponse{
					AdditionalData: make(map[string]string),
					RuntimeData:    make(map[string]string),
				}

				result := suite.executor.HasRequiredInputs(ctx, execResp)

				assert.False(suite.T(), result)
				assert.NotEmpty(suite.T(), execResp.Inputs)
			},
		},
		{
			name: "ValidatePrerequisites with no prerequisites returns true",
			testFunc: func() {
				ctx := &providers.NodeContext{
					UserInputs:  map[string]string{},
					RuntimeData: map[string]string{},
				}
				execResp := &providers.ExecutorResponse{
					AdditionalData: make(map[string]string),
					RuntimeData:    make(map[string]string),
				}

				result := suite.executor.ValidatePrerequisites(ctx, execResp, suite.mockAuthnProvider)

				assert.True(suite.T(), result)
			},
		},
		{
			name: "GetUserIDFromContext with empty context returns empty string",
			testFunc: func() {
				ctx := &providers.NodeContext{
					UserInputs:  map[string]string{},
					RuntimeData: map[string]string{},
				}
				execResp := &providers.ExecutorResponse{
					RuntimeData: make(map[string]string),
				}

				userID := suite.executor.GetUserIDFromContext(ctx, execResp, suite.mockAuthnProvider)
				assert.Empty(suite.T(), userID)
			},
		},
		{
			name: "GetInputs returns three required fields",
			testFunc: func() {
				ctx := &providers.NodeContext{
					UserInputs:  map[string]string{},
					RuntimeData: map[string]string{},
				}

				requiredData := suite.executor.GetRequiredInputs(ctx)

				assert.NotEmpty(suite.T(), requiredData)
				assert.Len(suite.T(), requiredData, 2)
			},
		},
		{
			name: "getOrganizationUnitRequest constructs request correctly",
			testFunc: func() {
				ctx := &providers.NodeContext{
					UserInputs: map[string]string{
						userInputOuName:   "Engineering",
						userInputOuHandle: "engineering",
					},
				}

				request, err := suite.executor.getOrganizationUnitRequest(ctx)

				assert.NoError(suite.T(), err)
				assert.Equal(suite.T(), "Engineering", request.Name)
				assert.Equal(suite.T(), "engineering", request.Handle)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, tc.testFunc)
	}
}

func (suite *OUExecutorTestSuite) TestOUExecutorInterface() {
	var _ providers.Executor = (*ouExecutor)(nil)
}

func (suite *OUExecutorTestSuite) TestExecute_RetryableOUCreationErrors() {
	tests := []struct {
		name           string
		serviceError   tidcommon.ServiceError
		expectedReason string
		message        string
	}{
		{
			name:           "OU name conflict",
			serviceError:   ou.ErrorOrganizationUnitNameConflict,
			expectedReason: ErrOUNameConflict.Error.DefaultValue,
			message:        "Should return inputs for retry when OU name already exists",
		},
		{
			name:           "OU handle conflict",
			serviceError:   ou.ErrorOrganizationUnitHandleConflict,
			expectedReason: ErrOUHandleConflict.Error.DefaultValue,
			message:        "Should return inputs for retry when OU handle already exists",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			suite.SetupTest()

			ctx := &providers.NodeContext{
				ExecutionID: "flow-123",
				FlowType:    providers.FlowTypeRegistration,
				UserInputs: map[string]string{
					userInputOuName:   "Engineering",
					userInputOuHandle: "engineering",
				},
				RuntimeData: map[string]string{},
			}

			suite.mockOUService.On("CreateOrganizationUnit", mock.Anything, providers.OrganizationUnitRequestWithID{
				Name:   "Engineering",
				Handle: "engineering",
			}).Return(providers.OrganizationUnit{}, &tt.serviceError)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, providers.ExecUserInputRequired, resp.Status)
			assert.Equal(t, tt.expectedReason, resp.Error.Error.DefaultValue, tt.message)
			assert.NotEmpty(t, resp.Inputs, "Inputs should be re-populated for retry")
			assert.Len(t, resp.Inputs, 2, "Should include both ouName and ouHandle inputs")
			suite.mockOUService.AssertExpectations(t)
		})
	}
}

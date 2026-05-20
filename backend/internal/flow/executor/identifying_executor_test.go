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
	"encoding/json"
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

type IdentifyingExecutorTestSuite struct {
	suite.Suite
	mockEntityProvider *entityprovidermock.EntityProviderInterfaceMock
	mockFlowFactory    *coremock.FlowFactoryInterfaceMock
	executor           *identifyingExecutor
}

func TestIdentifyingExecutorSuite(t *testing.T) {
	suite.Run(t, new(IdentifyingExecutorTestSuite))
}

func (suite *IdentifyingExecutorTestSuite) SetupTest() {
	suite.mockEntityProvider = entityprovidermock.NewEntityProviderInterfaceMock(suite.T())
	suite.mockFlowFactory = coremock.NewFlowFactoryInterfaceMock(suite.T())

	mockExec := createMockExecutor(suite.T(), ExecutorNameIdentifying, common.ExecutorTypeUtility)
	suite.mockFlowFactory.On("CreateExecutor", ExecutorNameIdentifying, common.ExecutorTypeUtility,
		[]common.Input{}, []common.Input{}).Return(mockExec)

	suite.executor = newIdentifyingExecutor(ExecutorNameIdentifying, []common.Input{},
		[]common.Input{}, suite.mockFlowFactory, suite.mockEntityProvider)
}

func (suite *IdentifyingExecutorTestSuite) TestNewIdentifyingExecutor() {
	assert.NotNil(suite.T(), suite.executor)
	assert.NotNil(suite.T(), suite.executor.entityProvider)

	// Test default name
	exec := newIdentifyingExecutor(
		"",
		[]common.Input{},
		[]common.Input{},
		suite.mockFlowFactory,
		suite.mockEntityProvider,
	)
	assert.NotNil(suite.T(), exec)
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyUser_Success() {
	filters := map[string]interface{}{"username": "testuser"}
	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}
	// Use package-level testUserID constant
	userID := testUserID
	suite.mockEntityProvider.On("IdentifyEntity", filters).Return(&userID, nil)

	result, err := suite.executor.IdentifyUser(filters, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testUserID, *result)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyUser_UserNotFound() {
	filters := map[string]interface{}{"username": "nonexistent"}
	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", filters).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	result, err := suite.executor.IdentifyUser(filters, execResp)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), failureReasonUserNotFound, execResp.FailureReason)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyUser_ServiceError() {
	filters := map[string]interface{}{"username": "testuser"}
	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}

	suite.mockEntityProvider.On("IdentifyEntity", filters).Return(nil,
		entityprovider.NewEntityProviderError(entityprovider.ErrorCodeSystemError, "", ""))

	result, err := suite.executor.IdentifyUser(filters, execResp)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Contains(suite.T(), execResp.FailureReason, "Failed to identify user")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyUser_EmptyUserID() {
	filters := map[string]interface{}{"username": "testuser"}
	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}
	emptyID := ""

	suite.mockEntityProvider.On("IdentifyEntity", filters).Return(&emptyID, nil)

	result, err := suite.executor.IdentifyUser(filters, execResp)

	assert.NoError(suite.T(), err)
	assert.Nil(suite.T(), result)
	assert.Equal(suite.T(), common.ExecFailure, execResp.Status)
	assert.Equal(suite.T(), failureReasonUserNotFound, execResp.FailureReason)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyUser_FilterNonSearchableAttributes() {
	filters := map[string]interface{}{
		"username": "testuser",
		"password": "secret123",
		"code":     "auth-code",
		"nonce":    "nonce-value",
		"otp":      "123456",
	}
	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}
	// Use package-level testUserID constant
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "testuser",
	}).Return(func() *string {
		userID := testUserID
		return &userID
	}(), nil)

	result, err := suite.executor.IdentifyUser(filters, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), testUserID, *result)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyUser_WithEmail() {
	filters := map[string]interface{}{"email": "test@example.com"}
	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}
	emailUserID := "user-456"

	suite.mockEntityProvider.On("IdentifyEntity", filters).Return(&emailUserID, nil)

	result, err := suite.executor.IdentifyUser(filters, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user-456", *result)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestIdentifyUser_WithMobileNumber() {
	filters := map[string]interface{}{"mobileNumber": "+1234567890"}
	execResp := &common.ExecutorResponse{
		RuntimeData: make(map[string]string),
	}
	mobileUserID := "user-789"

	suite.mockEntityProvider.On("IdentifyEntity", filters).Return(&mobileUserID, nil)

	result, err := suite.executor.IdentifyUser(filters, execResp)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), result)
	assert.Equal(suite.T(), "user-789", *result)
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_Success_UserInputs() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"username": "testuser"},
	}
	// Use package-level testUserID constant
	// Configure mock base executor
	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "username", Type: "string", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "testuser",
	}).Return(func() *string {
		userID := testUserID
		return &userID
	}(), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testUserID, resp.RuntimeData[userAttributeUserID])
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_Success_RuntimeData() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  make(map[string]string),
		RuntimeData: map[string]string{"username": "testuser"},
	}
	// Use package-level testUserID constant
	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "username", Type: "string", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "testuser",
	}).Return(func() *string {
		userID := testUserID
		return &userID
	}(), nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), testUserID, resp.RuntimeData[userAttributeUserID])
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_UserInputRequired() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(false)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_Failure_IdentifyUserError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"username": "testuser"},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "username", Type: "string", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "testuser",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	// IdentifyUser method in implementation swallows the error and returns nil, nil.
	// Then Execute checks for nil userID and returns UserNotFound.
	// So we should expect failureReasonUserNotFound
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), failureReasonUserNotFound, resp.FailureReason)
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_Failure_UserNotFound() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"username": "nonexistent"},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "username", Type: "string", Required: true},
	})

	emptyID := ""
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "nonexistent",
	}).Return(&emptyID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), failureReasonUserNotFound, resp.FailureReason)
}

// TestExecute_Success_WithVariousAttributes tests successful user identification with different attributes.
func (suite *IdentifyingExecutorTestSuite) TestExecute_Success_WithVariousAttributes() {
	testCases := []struct {
		name       string
		attribute  string
		value      string
		expectedID string
	}{
		{"email", "email", "test@example.com", "user-email-456"},
		{"mobileNumber", "mobileNumber", "+1234567890", "user-mobile-789"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  map[string]string{tc.attribute: tc.value},
			}

			mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
				{Identifier: tc.attribute, Type: "string", Required: true},
			})

			suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
				tc.attribute: tc.value,
			}).Return(&tc.expectedID, nil)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), common.ExecComplete, resp.Status)
			assert.Equal(suite.T(), tc.expectedID, resp.RuntimeData[userAttributeUserID])
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_Success_WithMultipleAttributes() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs: map[string]string{
			"username": "testuser",
			"email":    "test@example.com",
		},
	}
	multiAttrUserID := "user-multi-123"

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "username", Type: "string", Required: true},
		{Identifier: "email", Type: "string", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "testuser",
		"email":    "test@example.com",
	}).Return(&multiAttrUserID, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), multiAttrUserID, resp.RuntimeData[userAttributeUserID])
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

// TestExecute_Failure_UserNotFoundByAttribute tests failure handling when user is not found by different attributes.
func (suite *IdentifyingExecutorTestSuite) TestExecute_Failure_UserNotFoundByAttribute() {
	testCases := []struct {
		name      string
		attribute string
		value     string
	}{
		{"email", "email", "nonexistent@example.com"},
		{"mobileNumber", "mobileNumber", "+0000000000"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  map[string]string{tc.attribute: tc.value},
			}

			mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
				{Identifier: tc.attribute, Type: "string", Required: true},
			})

			suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
				tc.attribute: tc.value,
			}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""))

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
			assert.Equal(suite.T(), failureReasonUserNotFound, resp.FailureReason)
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

// TestExecute_Success_FromRuntimeData tests successful identification when attributes come from RuntimeData.
func (suite *IdentifyingExecutorTestSuite) TestExecute_Success_FromRuntimeData() {
	testCases := []struct {
		name       string
		attribute  string
		value      string
		expectedID string
	}{
		{"email", "email", "runtime@example.com", "user-runtime-email-456"},
		{"mobileNumber", "mobileNumber", "+9876543210", "user-runtime-mobile-789"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  make(map[string]string),
				RuntimeData: map[string]string{tc.attribute: tc.value},
			}

			mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
				{Identifier: tc.attribute, Type: "string", Required: true},
			})

			suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
				tc.attribute: tc.value,
			}).Return(&tc.expectedID, nil)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), common.ExecComplete, resp.Status)
			assert.Equal(suite.T(), tc.expectedID, resp.RuntimeData[userAttributeUserID])
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

// TestExecute_Failure_EmptyInput tests failure handling when input value is an empty string.
func (suite *IdentifyingExecutorTestSuite) TestExecute_Failure_EmptyInput() {
	testCases := []struct {
		name      string
		attribute string
	}{
		{"username", "username"},
		{"email", "email"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  map[string]string{tc.attribute: ""},
			}

			mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
				{Identifier: tc.attribute, Type: "string", Required: true},
			})

			emptyID := ""
			suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
				tc.attribute: "",
			}).Return(&emptyID, nil)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
			assert.Equal(suite.T(), failureReasonUserNotFound, resp.FailureReason)
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

// TestExecute_UserInputsPriorityOverRuntimeData tests that UserInputs takes priority over RuntimeData.
func (suite *IdentifyingExecutorTestSuite) TestExecute_UserInputsPriorityOverRuntimeData() {
	testCases := []struct {
		name           string
		attribute      string
		userInputValue string
		runtimeValue   string
		expectedID     string
	}{
		{"username", "username", "userinput-user", "runtime-user", "user-from-userinput-123"},
		{"email", "email", "userinput@example.com", "runtime@example.com", "user-from-email-userinput-456"},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			// Both UserInputs and RuntimeData have the same key
			// UserInputs should take priority
			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  map[string]string{tc.attribute: tc.userInputValue},
				RuntimeData: map[string]string{tc.attribute: tc.runtimeValue},
			}

			mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
				{Identifier: tc.attribute, Type: "string", Required: true},
			})

			// The mock should be called with the UserInputs value, not the RuntimeData value
			suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
				tc.attribute: tc.userInputValue,
			}).Return(&tc.expectedID, nil)

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(suite.T(), err)
			assert.NotNil(suite.T(), resp)
			assert.Equal(suite.T(), common.ExecComplete, resp.Status)
			assert.Equal(suite.T(), tc.expectedID, resp.RuntimeData[userAttributeUserID])
			suite.mockEntityProvider.AssertExpectations(suite.T())
		})
	}
}

// --- Resolve mode tests ---

// Test user attribute JSON helpers to keep lines under 120 chars.
var (
	attrsAlexJohnson = json.RawMessage(
		`{"given_name":"Alex","family_name":"Johnson"}`)
	attrsAlexSmith = json.RawMessage(
		`{"given_name":"Alex","family_name":"Smith"}`)
	attrsAlex = json.RawMessage(`{"given_name":"Alex"}`)
)

func (suite *IdentifyingExecutorTestSuite) TestExecuteResolve_UniqueUser() {
	ctx := &core.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeResolve,
		UserInputs:   map[string]string{"given_name": "Alex"},
		RuntimeData:  make(map[string]string),
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("SearchEntities", map[string]interface{}{
		"given_name": "Alex",
	}).Return([]*entityprovider.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlex},
	}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "user-1", resp.RuntimeData[userAttributeUserID])
}

func (suite *IdentifyingExecutorTestSuite) TestExecuteResolve_AmbiguousUser() {
	ctx := &core.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeResolve,
		UserInputs:   map[string]string{"given_name": "Alex"},
		RuntimeData:  make(map[string]string),
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("SearchEntities", map[string]interface{}{
		"given_name": "Alex",
	}).Return([]*entityprovider.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlexJohnson},
		{ID: "user-2", Type: "Engineer", Attributes: attrsAlexSmith},
	}, nil)

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyCandidateUsers])
	assert.NotNil(suite.T(), resp.ForwardedData)
}

func (suite *IdentifyingExecutorTestSuite) TestExecuteResolve_FilteredToOne() {
	candidates := []*entityprovider.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlexJohnson},
		{ID: "user-2", Type: "Person", Attributes: attrsAlexSmith},
	}
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := &core.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeResolve,
		UserInputs:   map[string]string{"given_name": "Alex", "family_name": "Smith"},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
		},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
		{Identifier: "family_name", Type: "TEXT_INPUT", Required: true},
	})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecComplete, resp.Status)
	assert.Equal(suite.T(), "user-2", resp.RuntimeData[userAttributeUserID])
}

func (suite *IdentifyingExecutorTestSuite) TestExecuteResolve_StillAmbiguous() {
	candidates := []*entityprovider.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlexSmith},
		{ID: "user-2", Type: "Engineer", Attributes: attrsAlexSmith},
	}
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := &core.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeResolve,
		UserInputs:   map[string]string{"given_name": "Alex", "family_name": "Smith"},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
		},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
		{Identifier: "family_name", Type: "TEXT_INPUT", Required: true},
	})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.NotEmpty(suite.T(), resp.RuntimeData[common.RuntimeKeyCandidateUsers])
}

func (suite *IdentifyingExecutorTestSuite) TestExecuteResolve_FilteredToNone() {
	candidates := []*entityprovider.Entity{
		{ID: "user-1", Type: "Person", Attributes: attrsAlexJohnson},
	}
	candidatesJSON, _ := json.Marshal(candidates)

	ctx := &core.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeResolve,
		UserInputs:   map[string]string{"given_name": "Alex", "family_name": "Williams"},
		RuntimeData: map[string]string{
			common.RuntimeKeyCandidateUsers: string(candidatesJSON),
		},
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
		{Identifier: "family_name", Type: "TEXT_INPUT", Required: true},
	})

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), failureReasonUserNotFound, resp.FailureReason)
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_IdentifyMode_AmbiguousUser() {
	ctx := &core.NodeContext{
		ExecutionID:  "flow-123",
		ExecutorMode: ExecutorModeIdentify,
		UserInputs:   map[string]string{"given_name": "Alex"},
		RuntimeData:  make(map[string]string),
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "given_name", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"given_name": "Alex",
	}).Return(nil, entityprovider.NewEntityProviderError(entityprovider.ErrorCodeAmbiguousEntity, "Ambiguous user", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), failureReasonAmbiguousUser, resp.FailureReason)
	assert.Empty(suite.T(), resp.Inputs, "Inputs must not be populated for ambiguous user in identify mode")
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_IdentifyMode_UserNotFound_PopulatesInputsForRetry() {
	inputs := []common.Input{{Identifier: "username", Type: "TEXT_INPUT", Required: true}}
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"username": "nonexistent"},
		RuntimeData: make(map[string]string),
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return(inputs)

	// IdentifyUser sets ExecFailure + userNotFound; executeIdentify must promote to UserInputRequired
	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "nonexistent",
	}).Return(nil, entityprovider.NewEntityProviderError(
		entityprovider.ErrorCodeEntityNotFound, "not found", ""))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecUserInputRequired, resp.Status)
	assert.Equal(suite.T(), failureReasonUserNotFound, resp.FailureReason)
	assert.NotEmpty(suite.T(), resp.Inputs, "Inputs must be populated for retry when user is not found")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_IdentifyMode_SystemError() {
	ctx := &core.NodeContext{
		ExecutionID: "flow-123",
		UserInputs:  map[string]string{"username": "testuser"},
		RuntimeData: make(map[string]string),
	}

	mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
	mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
	mockBase.On("GetRequiredInputs", mock.Anything).Return([]common.Input{
		{Identifier: "username", Type: "TEXT_INPUT", Required: true},
	})

	suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
		"username": "testuser",
	}).Return(nil, entityprovider.NewEntityProviderError(
		entityprovider.ErrorCodeSystemError, "System error", "db unavailable"))

	resp, err := suite.executor.Execute(ctx)

	assert.NoError(suite.T(), err)
	assert.NotNil(suite.T(), resp)
	assert.Equal(suite.T(), common.ExecFailure, resp.Status)
	assert.Equal(suite.T(), failureReasonFailedToIdentifyUser, resp.FailureReason)
	assert.Empty(suite.T(), resp.Inputs, "Inputs must not be populated for non-recoverable errors")
	suite.mockEntityProvider.AssertExpectations(suite.T())
}

func TestFilterUsersByAttributes(t *testing.T) {
	users := []*entityprovider.Entity{
		{ID: "u1", Type: "Person", Attributes: attrsAlexJohnson},
		{ID: "u2", Type: "Person", Attributes: attrsAlexSmith},
		{ID: "u3", Type: "Engineer", Attributes: attrsAlexSmith},
	}

	result := filterUsersByAttributes(users, map[string]interface{}{"family_name": "Smith"})
	assert.Len(t, result, 2)

	result = filterUsersByAttributes(users, map[string]interface{}{"userType": "Engineer"})
	assert.Len(t, result, 1)
	assert.Equal(t, "u3", result[0].ID)

	result = filterUsersByAttributes(users, map[string]interface{}{
		"family_name": "Smith",
		"userType":    "Person",
	})
	assert.Len(t, result, 1)
	assert.Equal(t, "u2", result[0].ID)

	result = filterUsersByAttributes(users, map[string]interface{}{"family_name": "Doe"})
	assert.Empty(t, result)
}

func (suite *IdentifyingExecutorTestSuite) TestExecute_RetryableIdentificationErrors() {
	tests := []struct {
		name           string
		attribute      string
		value          string
		entityError    *entityprovider.EntityProviderError
		emptyID        bool
		expectedReason string
		message        string
	}{
		{
			name:           "User not found",
			attribute:      "username",
			value:          "nonexistent",
			entityError:    entityprovider.NewEntityProviderError(entityprovider.ErrorCodeEntityNotFound, "", ""),
			expectedReason: failureReasonUserNotFound,
			message:        "Should return inputs for retry when user is not found",
		},
		{
			name:           "Empty user ID returned",
			attribute:      "username",
			value:          "testuser",
			emptyID:        true,
			expectedReason: failureReasonUserNotFound,
			message:        "Should return inputs for retry when empty user ID is returned",
		},
	}

	for _, tt := range tests {
		suite.T().Run(tt.name, func(t *testing.T) {
			suite.SetupTest()

			inputs := []common.Input{{Identifier: tt.attribute, Type: "string", Required: true}}
			ctx := &core.NodeContext{
				ExecutionID: "flow-123",
				UserInputs:  map[string]string{tt.attribute: tt.value},
			}

			mockBase := suite.executor.ExecutorInterface.(*coremock.ExecutorInterfaceMock)
			mockBase.On("HasRequiredInputs", mock.Anything, mock.Anything).Return(true)
			mockBase.On("GetRequiredInputs", mock.Anything).Return(inputs)

			if tt.emptyID {
				emptyID := ""
				suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
					tt.attribute: tt.value,
				}).Return(&emptyID, nil)
			} else {
				suite.mockEntityProvider.On("IdentifyEntity", map[string]interface{}{
					tt.attribute: tt.value,
				}).Return(nil, tt.entityError)
			}

			resp, err := suite.executor.Execute(ctx)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, common.ExecUserInputRequired, resp.Status)
			assert.Equal(t, tt.expectedReason, resp.FailureReason, tt.message)
			assert.NotEmpty(t, resp.Inputs, "Inputs should be re-populated for retry")
			suite.mockEntityProvider.AssertExpectations(t)
		})
	}
}

func TestExtractDisambiguationOptions(t *testing.T) {
	candidates := []*entityprovider.Entity{
		{ID: "u1", Type: "Person", Attributes: attrsAlexJohnson},
		{ID: "u2", Type: "Person", Attributes: attrsAlexSmith},
		{ID: "u3", Type: "Engineer", Attributes: attrsAlexSmith},
	}

	inputs := extractDisambiguationOptions(candidates)

	inputsByKey := make(map[string]common.Input)
	for _, input := range inputs {
		inputsByKey[input.Identifier] = input
	}

	assert.Contains(t, inputsByKey, "userType")
	assert.ElementsMatch(t, []string{"Person", "Engineer"}, inputsByKey["userType"].Options)
	assert.Equal(t, common.InputTypeSelect, inputsByKey["userType"].Type)

	assert.Contains(t, inputsByKey, "family_name")
	assert.ElementsMatch(t, []string{"Johnson", "Smith"}, inputsByKey["family_name"].Options)
	assert.Equal(t, common.InputTypeSelect, inputsByKey["family_name"].Type)

	assert.NotContains(t, inputsByKey, "given_name")
}
